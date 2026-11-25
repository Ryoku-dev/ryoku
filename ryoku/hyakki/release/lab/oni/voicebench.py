#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-only
"""voicebench.py: the helper behind voicebench.sh.

  corpus --dir D --tarball T --source URL --md5 M
        keep 50 LibriSpeech test-clean utterances of 2-9 s (at most two per
        speaker, speakers in order) as 16 kHz s16 WAV in D/corpus/ls, with
        their transcripts, and write D/corpus/librispeech.json
  espeak --dir D
        render the espeak-ng clips to D/corpus/espeak through --stdout
  run --dir D --whisper BIN --out O
        bench every candidate and write O/voicebench.json and O/voicebench.md

Engines are driven with exactly the command lines and request fields that
voice/stt's adapters use. Every child gets a private, empty
XDG_RUNTIME_DIR and PipeWire/Pulse addresses that lead nowhere, and every
child's open files are checked for sound devices while it runs.
"""

import argparse
import hashlib
import http.client
import json
import math
import os
import re
import secrets
import shutil
import signal
import socket
import statistics
import subprocess
import sys
import tarfile
import threading
import time
import uuid
import wave

RATE = 16000
# spec voice section (internal/oni/spec/oni-spec.json)
SPEECH_RMS, FRAME_MS, PAUSE_MS, MIN_SPEECH_MS = 400, 10, 450, 350
FIRST_PARTIAL_MS, PRE_ROLL_MS = 650, 200
PARTIAL_BUDGET_MS, FINAL_BUDGET_MS = 1200, 3000
STOP_GRACE_S = 2

ESPEAK = [
    ("open-notes", "Open the notes app and start a timer for five minutes."),
    ("weather", "What is the weather like in Lisbon tomorrow morning?"),
    ("remind", "Remind me to call Mara at half past three."),
    ("volume", "Turn the volume down a little, please."),
    ("message", "Read me the last message from Tobias."),
    ("scratch", "Send the report to the team, scratch that, send it to Mara."),
]


def sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def flac_seconds(data):
    # STREAMINFO is the first metadata block after "fLaC"
    if data[:4] != b"fLaC":
        raise ValueError("not FLAC")
    info = data[8:42]
    bits = int.from_bytes(info[10:18], "big")
    rate = bits >> 44
    total = bits & ((1 << 36) - 1)
    return total / rate


def to_wav16k(src_bytes, dest):
    subprocess.run(
        ["ffmpeg", "-nostdin", "-loglevel", "error", "-i", "pipe:0", "-ar", str(RATE), "-ac", "1",
         "-sample_fmt", "s16", "-f", "wav", "-y", dest],
        input=src_bytes, check=True, env=child_env(None))


def cmd_corpus(a):
    out = os.path.join(a.dir, "corpus", "ls")
    os.makedirs(out, exist_ok=True)
    flacs, trans = {}, {}
    with tarfile.open(a.tarball, "r:gz") as tar:
        for m in tar:
            if not m.isfile():
                continue
            if m.name.endswith(".trans.txt"):
                for line in tar.extractfile(m).read().decode().splitlines():
                    uid, text = line.split(" ", 1)
                    trans[uid] = text
            elif m.name.endswith(".flac"):
                data = tar.extractfile(m).read()
                uid = os.path.basename(m.name)[:-5]
                flacs[uid] = (m.name, flac_seconds(data), data)
    speakers = {}
    for uid in sorted(flacs, key=lambda u: tuple(int(x) for x in u.split("-"))):
        name, secs, _ = flacs[uid]
        if 2.0 <= secs <= 9.0:
            speakers.setdefault(uid.split("-")[0], []).append(uid)
    picked = [u[0] for u in speakers.values()]
    for us in speakers.values():
        if len(picked) >= 50:
            break
        if len(us) > 1:
            picked.append(us[1])
    picked = sorted(picked[:50], key=lambda u: tuple(int(x) for x in u.split("-")))
    clips = []
    for uid in picked:
        name, secs, data = flacs[uid]
        dest = os.path.join(out, uid + ".wav")
        to_wav16k(data, dest)
        clips.append({"id": uid, "member": name, "seconds": round(secs, 3), "flacSha256": hashlib.sha256(data).hexdigest(),
                      "wav": dest, "wavSha256": sha256(dest), "reference": trans[uid]})
    meta = {"source": a.source, "license": "CC-BY 4.0 (LibriSpeech ASR corpus, Panayotov et al., OpenSLR SLR12)",
            "tarballSha256": sha256(a.tarball), "tarballMd5": a.md5, "selection": "speakers in numeric order, first utterance of 2-9 s per speaker, then a second for the first speakers until 50",
            "clips": clips}
    with open(os.path.join(a.dir, "corpus", "librispeech.json"), "w") as f:
        json.dump(meta, f, indent=1)
    print(f"kept {len(clips)} utterances")


def cmd_espeak(a):
    out = os.path.join(a.dir, "corpus", "espeak")
    os.makedirs(out, exist_ok=True)
    version = subprocess.run(["espeak-ng", "--version"], capture_output=True, text=True).stdout.strip()
    clips = []
    for cid, text in ESPEAK:
        raw = subprocess.run(["espeak-ng", "-v", "en-us", "-s", "165", "--stdout", text],
                             check=True, capture_output=True, env=child_env(None)).stdout
        dest = os.path.join(out, cid + ".wav")
        to_wav16k(raw, dest)
        clips.append({"id": "espeak-" + cid, "wav": dest, "wavSha256": sha256(dest), "reference": text,
                      "seconds": round(wav_seconds(dest), 3)})
    with open(os.path.join(a.dir, "corpus", "espeak.json"), "w") as f:
        json.dump({"engine": version, "voice": "en-us", "rate": 165, "clips": clips}, f, indent=1)


# --- engines --------------------------------------------------------------

_RUNTIME = None


def child_env(runtime):
    env = {"PATH": "/usr/bin:/bin", "LANG": "C.UTF-8"}
    if runtime:
        env.update({"XDG_RUNTIME_DIR": runtime, "HOME": runtime,
                    "PIPEWIRE_REMOTE": os.path.join(runtime, "no-pipewire"),
                    "PULSE_SERVER": "unix:" + os.path.join(runtime, "no-pulse")})
    return env


def die_with_parent():
    """Runs in the child before exec: SIGKILL it when this helper dies, so
    an interrupted bench leaves no engine behind."""
    import ctypes
    PR_SET_PDEATHSIG = 1
    ctypes.CDLL("libc.so.6", use_errno=True).prctl(PR_SET_PDEATHSIG, signal.SIGKILL)


def unix_paths():
    paths = {}
    with open("/proc/net/unix") as f:
        next(f)
        for line in f:
            p = line.split()
            if len(p) >= 8:
                paths[p[6]] = p[7]
    return paths


def device_fds(pid):
    """What the process holds that could reach sound: /dev/snd nodes, and
    unix sockets (with their path when bound, e.g. a PipeWire or Pulse
    server's)."""
    found = []
    try:
        names = os.listdir(f"/proc/{pid}/fd")
    except OSError:
        return found
    paths = None
    for n in names:
        try:
            target = os.readlink(f"/proc/{pid}/fd/{n}")
        except OSError:
            continue
        if target.startswith("/dev/snd"):
            found.append(target)
        elif target.startswith("socket:["):
            paths = paths if paths is not None else unix_paths()
            inode = target[8:-1]
            if inode in paths:
                found.append("unix:" + paths[inode])
    return found


def listening(pid, port):
    """Whether pid holds a TCP socket listening on 127.0.0.1:port."""
    want = {f"0100007F:{port:04X}", f"7F000001:{port:04X}"}
    inodes = set()
    with open("/proc/net/tcp") as f:
        for line in f:
            p = line.split()
            if len(p) >= 10 and p[1] in want and p[3] == "0A":
                inodes.add(f"socket:[{p[9]}]")
    if not inodes:
        return False
    try:
        names = os.listdir(f"/proc/{pid}/fd")
    except OSError:
        return False
    for n in names:
        try:
            if os.readlink(f"/proc/{pid}/fd/{n}") in inodes:
                return True
        except OSError:
            pass
    return False


def proc_cpu_s(pid):
    with open(f"/proc/{pid}/stat") as f:
        fields = f.read().rsplit(")", 1)[1].split()
    return (int(fields[11]) + int(fields[12])) / os.sysconf("SC_CLK_TCK")


def proc_hwm_kib(pid):
    with open(f"/proc/{pid}/status") as f:
        for line in f:
            if line.startswith("VmHWM:"):
                return int(line.split()[1])
    return 0


def wav_pcm(path):
    with wave.open(path, "rb") as w:
        assert w.getframerate() == RATE and w.getnchannels() == 1 and w.getsampwidth() == 2
        return w.readframes(w.getnframes())


def wav_seconds(path):
    with wave.open(path, "rb") as w:
        return w.getnframes() / w.getframerate()


def wav_bytes(pcm):
    import io
    buf = io.BytesIO()
    with wave.open(buf, "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(RATE)
        w.writeframes(pcm)
    return buf.getvalue()


def first_partial_window(pcm):
    """The audio the first partial decodes, by the segmenter's rules: speech
    starts once 350 ms of frames reach RMS 400 without a 450 ms pause; the
    first partial comes 650 ms after that onset and carries 200 ms of
    pre-roll."""
    n = RATE * FRAME_MS // 1000
    frames = len(pcm) // 2 // n
    import array
    s = array.array("h")
    s.frombytes(pcm[: frames * n * 2])
    start, speech, silence = None, 0, 0
    onset = 0
    for i in range(frames):
        chunk = s[i * n:(i + 1) * n]
        rms = math.sqrt(sum(x * x for x in chunk) / n)
        if rms >= SPEECH_RMS:
            if start is None:
                start = i
            speech += FRAME_MS
            silence = 0
            if speech >= MIN_SPEECH_MS:
                onset = start
                break
        elif start is not None:
            silence += FRAME_MS
            if silence >= PAUSE_MS:
                start, speech, silence = None, 0, 0
    begin = max(0, onset * n - RATE * PRE_ROLL_MS // 1000)
    end = min(frames * n, onset * n + RATE * FIRST_PARTIAL_MS // 1000)
    return pcm[begin * 2:end * 2]


def audio_ctx(seconds):
    return max(256, min(1500, math.ceil((seconds * 50 + 48) / 64) * 64))


class WhisperServer:
    """whisper-server with voice/stt Whisper's argv and request fields."""

    def __init__(self, binary, model, vad_model, runtime):
        self.port = self._free_port()
        self.request = "/p" + secrets.token_hex(12)
        self.inference = "/i" + secrets.token_hex(12)
        threads = max(2, min(8, os.cpu_count()))
        self.argv = [binary, "--model", model, "--threads", str(threads), "--language", "en", "--no-fallback",
                     "--best-of", "1", "--beam-size", "1", "--no-timestamps", "--host", "127.0.0.1",
                     "--port", str(self.port), "--request-path", self.request, "--inference-path", self.inference,
                     "--no-gpu"]
        if vad_model:
            self.argv += ["--vad", "--vad-model", vad_model]
        began = time.monotonic()
        self.proc = subprocess.Popen(self.argv, cwd="/", env=child_env(runtime), stdout=subprocess.DEVNULL,
                                     stderr=subprocess.PIPE, start_new_session=True, preexec_fn=die_with_parent)
        self.stderr = []

        def drain_err():
            for line in self.proc.stderr:
                self.stderr.append(line.decode(errors="replace"))
                del self.stderr[:-50]

        threading.Thread(target=drain_err, daemon=True).start()
        # ready once the child itself holds the listening socket (its
        # "listening" line sits in a block-buffered pipe), as stt does
        while not listening(self.proc.pid, self.port):
            if self.proc.poll() is not None:
                raise RuntimeError("whisper-server exited: " + "".join(self.stderr[-5:]))
            time.sleep(0.01)
        self.load_ms = (time.monotonic() - began) * 1000
        self.devices = device_fds(self.proc.pid)

    @staticmethod
    def _free_port():
        s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        s.bind(("127.0.0.1", 0))
        port = s.getsockname()[1]
        s.close()
        return port

    def decode(self, pcm, budget_ms, ctx=None):
        seconds = len(pcm) / 2 / RATE
        boundary = uuid.uuid4().hex
        parts = []
        fields = {"response_format": "json", "temperature": "0", "language": "en", "no_timestamps": "true",
                  "audio_ctx": str(ctx or audio_ctx(seconds))}
        for k, v in fields.items():
            parts.append(f'--{boundary}\r\nContent-Disposition: form-data; name="{k}"\r\n\r\n{v}\r\n'.encode())
        parts.append(f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="speech.wav"\r\n'
                     f'Content-Type: application/octet-stream\r\n\r\n'.encode() + wav_bytes(pcm) + b"\r\n")
        parts.append(f"--{boundary}--\r\n".encode())
        body = b"".join(parts)
        cpu0 = proc_cpu_s(self.proc.pid)
        began = time.monotonic()
        conn = http.client.HTTPConnection("127.0.0.1", self.port, timeout=120)
        conn.request("POST", self.request + self.inference, body,
                     {"Content-Type": f"multipart/form-data; boundary={boundary}"})
        resp = conn.getresponse()
        data = resp.read()
        conn.close()
        wall = time.monotonic() - began
        cpu = proc_cpu_s(self.proc.pid) - cpu0
        if resp.status != 200:
            raise RuntimeError(f"whisper-server {resp.status}: {data[:200]!r}")
        self.devices += device_fds(self.proc.pid)
        return json.loads(data)["text"].strip(), wall * 1000, 100 * cpu / wall if wall else 0, None

    def close(self):
        hwm = proc_hwm_kib(self.proc.pid)
        began = time.monotonic()
        self.proc.send_signal(signal.SIGTERM)
        try:
            self.proc.wait(STOP_GRACE_S)
            killed = False
        except subprocess.TimeoutExpired:
            self.proc.kill()
            self.proc.wait()
            killed = True
        return hwm, (time.monotonic() - began) * 1000, killed


class Dictate:
    """ryoku-dictate --selftest with voice/stt Dictate's argv: one child per
    decode, nothing resident."""

    def __init__(self, binary, model, runtime, scratch):
        self.binary, self.model, self.runtime, self.scratch = binary, model, runtime, scratch
        self.hwm = 0
        self.devices = []
        self.load_ms = None

    def decode(self, pcm, budget_ms):
        wav = os.path.join(self.scratch, "speech.wav")
        with open(wav, "wb") as f:
            f.write(wav_bytes(pcm))
        os.chmod(wav, 0o600)
        argv = [self.binary, "--selftest", wav, "--model", self.model, "--device", "cpu", "--language", "en"]
        began = time.monotonic()
        proc = subprocess.Popen(argv, cwd="/", env=child_env(self.runtime), stdout=subprocess.PIPE,
                                stderr=subprocess.PIPE, start_new_session=True, preexec_fn=die_with_parent)
        out, err = [], []
        t1 = threading.Thread(target=lambda: out.append(proc.stdout.read()), daemon=True)
        t2 = threading.Thread(target=lambda: err.append(proc.stderr.read()), daemon=True)
        t1.start()
        t2.start()
        while True:
            self.devices += device_fds(proc.pid)
            pid, status, ru = os.wait4(proc.pid, os.WNOHANG)
            if pid:
                break
            time.sleep(0.01)
        wall = time.monotonic() - began
        proc.returncode = os.waitstatus_to_exitcode(status)
        t1.join()
        t2.join()
        os.remove(wav)
        stderr = err[0].decode(errors="replace")
        if proc.returncode != 0:
            raise RuntimeError(f"ryoku-dictate exited {proc.returncode}: {stderr[-300:]}")
        self.hwm = max(self.hwm, ru.ru_maxrss)
        took = re.search(r"took=(\d+)ms", stderr)
        text = out[0].decode(errors="replace").strip().splitlines()
        cpu = ru.ru_utime + ru.ru_stime
        return (text[-1].strip() if text else ""), wall * 1000, 100 * cpu / wall, int(took.group(1)) if took else None

    def close(self):
        return self.hwm, None, False


# --- scoring --------------------------------------------------------------

ONES = "zero one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen sixteen seventeen eighteen nineteen".split()
TENS = "_ _ twenty thirty forty fifty sixty seventy eighty ninety".split()
ABBREV = {"mr": "mister", "mrs": "missus", "dr": "doctor", "st": "saint"}


def say(n):
    if n < 20:
        return ONES[n]
    if n < 100:
        return TENS[n // 10] + ("" if n % 10 == 0 else " " + ONES[n % 10])
    if n < 1000:
        return ONES[n // 100] + " hundred" + ("" if n % 100 == 0 else " " + say(n % 100))
    if n < 1000000:
        return say(n // 1000) + " thousand" + ("" if n % 1000 == 0 else " " + say(n % 1000))
    return str(n)


def normalize(text):
    """Lower case, digits spelt out, punctuation dropped (apostrophes kept),
    a few abbreviations expanded. Deliberately small: it is the same for
    every engine."""
    text = text.lower().replace("-", " ")
    text = re.sub(r"\d+", lambda m: " " + say(int(m.group())) + " ", text)
    words = re.sub(r"[^a-z' ]+", " ", text).split()
    return [ABBREV.get(w, w) for w in (w.strip("'") for w in words) if w]


def edits(ref, hyp):
    prev = list(range(len(hyp) + 1))
    for i, r in enumerate(ref, 1):
        cur = [i] + [0] * len(hyp)
        for j, h in enumerate(hyp, 1):
            cur[j] = min(prev[j] + 1, cur[j - 1] + 1, prev[j - 1] + (r != h))
        prev = cur
    return prev[-1]


def pct(values, q):
    values = sorted(values)
    return values[min(len(values) - 1, math.ceil(q * len(values)) - 1)]


# --- run ------------------------------------------------------------------

def bench(cand, engine, clips):
    rows = []
    for clip in clips:
        pcm = wav_pcm(clip["wav"])
        window = first_partial_window(pcm)
        p_text, p_ms, _, _ = engine.decode(window, PARTIAL_BUDGET_MS)
        text, f_ms, cpu, took = engine.decode(pcm, FINAL_BUDGET_MS)
        ref, hyp = normalize(clip["reference"]), normalize(text)
        err = edits(ref, hyp)
        rows.append({"candidate": cand["id"], "clip": clip["id"], "set": clip["set"], "seconds": clip["seconds"],
                     "refWords": len(ref), "errors": err, "wer": round(err / len(ref), 4),
                     "firstPartialAudioS": round(len(window) / 2 / RATE, 3), "firstPartialMs": round(p_ms, 1),
                     "firstPartialInBudget": p_ms <= PARTIAL_BUDGET_MS, "finalMs": round(f_ms, 1),
                     "finalInBudget": f_ms <= FINAL_BUDGET_MS, "engineDecodeMs": took, "cpuPct": round(cpu, 1),
                     "hypothesis": text})
        print(f"{cand['id']:28} {clip['id']:24} wer={rows[-1]['wer']:.3f} partial={p_ms:7.1f} final={f_ms:7.1f}", flush=True)
    return rows


def summarize(cand, rows, load_ms, hwm_kib, unload_ms, killed, devices):
    def wer(subset):
        r = [x for x in rows if x["set"] == subset]
        words = sum(x["refWords"] for x in r)
        return round(sum(x["errors"] for x in r) / words, 4) if words else None
    fp = [r["firstPartialMs"] for r in rows]
    fm = [r["finalMs"] for r in rows]
    return {
        "candidate": cand["id"], "adapter": cand["adapter"], "engine": cand["engine"], "engineSha256": cand["engineSha256"],
        "model": os.path.basename(cand["model"]), "modelSha256": cand["modelSha256"], "license": cand["license"],
        "vadModel": os.path.basename(cand["vad"]) if cand.get("vad") else None,
        "vadSha256": cand.get("vadSha256"), "argv": cand.get("argvShape"),
        "werLibriSpeech": wer("librispeech"), "werEspeak": wer("espeak"),
        "firstPartialMsMedian": round(statistics.median(fp), 1), "firstPartialMsP90": round(pct(fp, 0.9), 1),
        "firstPartialInBudget": f"{sum(r['firstPartialInBudget'] for r in rows)}/{len(rows)}",
        "finalMsMedian": round(statistics.median(fm), 1), "finalMsP90": round(pct(fm, 0.9), 1),
        "finalInBudget": f"{sum(r['finalInBudget'] for r in rows)}/{len(rows)}",
        "cpuPctMean": round(statistics.mean(r["cpuPct"] for r in rows), 1),
        "peakRssMiB": round(hwm_kib / 1024, 1), "loadMs": round(load_ms, 1) if load_ms is not None else None,
        "unloadMs": round(unload_ms, 1) if unload_ms is not None else None, "unloadKilled": killed,
        "resident": cand["adapter"] == "whispercpp", "soundDeviceFds": sorted(set(devices)),
    }


def cmd_run(a):
    runtime = os.path.join(a.dir, "rt")
    shutil.rmtree(runtime, ignore_errors=True)
    os.makedirs(runtime, mode=0o700)
    scratch = os.path.join(runtime, "scratch")
    os.makedirs(scratch, mode=0o700)
    models = os.path.join(a.dir, "models")
    corpus = json.load(open(os.path.join(a.dir, "corpus", "librispeech.json")))
    espeak = json.load(open(os.path.join(a.dir, "corpus", "espeak.json")))
    clips = [dict(c, set="librispeech") for c in corpus["clips"]] + [dict(c, set="espeak") for c in espeak["clips"]]
    for c in clips:
        if sha256(c["wav"]) != c["wavSha256"]:
            sys.exit(f"{c['wav']} changed since staging")

    dictate = shutil.which("ryoku-dictate")
    p110 = "/usr/share/ryoku/dictation/models/parakeet-tdt_ctc-110m-Q8_0.gguf"
    vad = os.path.join(models, "ggml-silero-v6.2.0.bin")
    cands = [
        {"id": "whisper-base.en-q5_1", "adapter": "whispercpp", "model": os.path.join(models, "ggml-base.en-q5_1.bin"), "license": "MIT"},
        {"id": "whisper-base.en-q5_1+silero", "adapter": "whispercpp", "model": os.path.join(models, "ggml-base.en-q5_1.bin"), "vad": vad, "license": "MIT"},
        {"id": "whisper-large-v3-turbo-q5_0", "adapter": "whispercpp", "model": os.path.join(models, "ggml-large-v3-turbo-q5_0.bin"), "license": "MIT"},
    ]
    if dictate:
        cands += [
            {"id": "parakeet-tdt_ctc-110m-q8_0", "adapter": "transcribecpp", "model": p110, "license": "CC-BY-4.0"},
            {"id": "parakeet-tdt-0.6b-v2-q8_0", "adapter": "transcribecpp", "model": os.path.join(models, "parakeet-tdt-0.6b-v2-Q8_0.gguf"), "license": "CC-BY-4.0"},
        ]
    dictate_version = subprocess.run([dictate, "--version"], capture_output=True, text=True, env=child_env(runtime)).stdout.strip() if dictate else None
    summaries, rows = [], []
    probes = []
    for cand in cands:
        cand["modelSha256"] = sha256(cand["model"])
        if cand.get("vad"):
            cand["vadSha256"] = sha256(cand["vad"])
        if cand["adapter"] == "whispercpp":
            cand["engine"] = a.whisper
            cand["engineSha256"] = sha256(a.whisper)
            engine = WhisperServer(a.whisper, cand["model"], cand.get("vad"), runtime)
            cand["argvShape"] = [x if not re.fullmatch(r"/[pi][0-9a-f]{24}", x) else x[:2] + "<24hex>" for x in engine.argv]
        else:
            cand["engine"] = f"{dictate} ({dictate_version})"
            cand["engineSha256"] = sha256(dictate)
            cand["argvShape"] = [dictate, "--selftest", "<wav>", "--model", cand["model"], "--device", "cpu", "--language", "en"]
            engine = Dictate(dictate, cand["model"], runtime, scratch)
        r = bench(cand, engine, clips)
        if cand["id"] == "whisper-large-v3-turbo-q5_0":
            # the adapter's context sizing (floor 256) is what the rows use;
            # this checks whether the full 1500 context would rescue turbo
            probe = []
            for clip in clips[:3]:
                text, ms, _, _ = engine.decode(wav_pcm(clip["wav"]), FINAL_BUDGET_MS, ctx=1500)
                ref, hyp = normalize(clip["reference"]), normalize(text)
                probe.append({"clip": clip["id"], "seconds": clip["seconds"], "finalMs": round(ms, 1),
                              "wer": round(edits(ref, hyp) / len(ref), 4)})
            probes.append({"candidate": cand["id"], "audioCtx": 1500, "rows": probe})
        hwm, unload_ms, killed = engine.close()
        summaries.append(summarize(cand, r, engine.load_ms, hwm, unload_ms, killed, engine.devices))
        rows += r
        print(json.dumps(summaries[-1]), flush=True)

    # Decision, fixed before the run: candidates whose p90 first partial and
    # p90 final fit the spec budgets, lowest LibriSpeech WER (a difference
    # under half a point is a tie, broken by the median final latency).
    # The fallback is the best candidate of the other adapter.
    def fits(s):
        return s["firstPartialMsP90"] <= PARTIAL_BUDGET_MS and s["finalMsP90"] <= FINAL_BUDGET_MS

    def key(s):
        return (round(s["werLibriSpeech"] / 0.005), s["finalMsMedian"])
    eligible = sorted([s for s in summaries if fits(s)], key=key)
    decision = {"rule": "p90 first partial <= 1200 ms and p90 final <= 3000 ms; then lowest LibriSpeech WER (0.5-point buckets); then lowest median final; fallback = best of the other adapter, budgets first"}
    if eligible:
        best = eligible[0]
        others = sorted([s for s in summaries if s["adapter"] != best["adapter"]], key=lambda s: (not fits(s),) + key(s))
        decision["default"] = {"candidate": best["candidate"], "adapter": best["adapter"]}
        decision["fallback"] = {"candidate": others[0]["candidate"], "adapter": others[0]["adapter"], "fitsBudgets": fits(others[0])} if others else None
    else:
        decision["default"] = None
    shutil.rmtree(runtime, ignore_errors=True)

    st = os.statvfs("/home")
    evidence = {
        "bench": "voice/stt V3b", "date": time.strftime("%Y-%m-%dT%H:%M:%S%z"), "host": {"cpus": os.cpu_count(), "kernel": os.uname().release},
        "homeFreeGiB": round(st.f_bavail * st.f_frsize / 2**30, 2),
        "budgets": {"firstPartialMs": PARTIAL_BUDGET_MS, "finalMs": FINAL_BUDGET_MS},
        "method": ("Each clip: the first partial decodes the audio the segmenter hands over 650 ms after speech onset "
                   "(with 200 ms pre-roll), then the final decodes the whole clip; whisper candidates run one resident "
                   "server per candidate (warm; load time reported separately), ryoku-dictate candidates run one child "
                   "per decode (load included in every latency). CPU % is the engine's CPU time over the decode's wall "
                   "time (100 = one core). Peak RSS is VmHWM (whisper-server) or ru_maxrss (largest ryoku-dictate child). "
                   "Unload is SIGTERM to exit. WER uses the same small normalizer for every engine. No vocabulary hint. "
                   "All engines on CPU; the live desktop kept running, so latencies include its load."),
        "corpus": {"librispeech": {k: v for k, v in corpus.items() if k != "clips"} | {"clips": [{k: c[k] for k in ("id", "member", "seconds", "flacSha256", "wavSha256")} for c in corpus["clips"]]},
                   "espeak": {k: v for k, v in espeak.items() if k != "clips"} | {"clips": [{k: c[k] for k in ("id", "seconds", "wavSha256", "reference")} for c in espeak["clips"]]}},
        "parakeetNote": "Parakeet rows are parakeet-tdt_ctc-110m and parakeet-tdt-0.6b-v2 (English); neither is Parakeet v3 and neither is 'ultra'.",
        "probes": probes, "summaries": summaries, "decision": decision, "rows": rows,
    }
    os.makedirs(a.out, exist_ok=True)
    with open(os.path.join(a.out, "voicebench.json"), "w") as f:
        json.dump(evidence, f, indent=1)
    with open(os.path.join(a.out, "voicebench.md"), "w") as f:
        f.write(markdown(evidence))
    print(json.dumps(decision))


def markdown(e):
    lines = ["# Speech-to-text bench (voice/stt)", "",
             f"Run {e['date']} on {e['host']['cpus']} CPUs, all engines on CPU. {len(e['rows']) // max(1, len(e['summaries']))} clips per candidate: "
             f"{len(e['corpus']['librispeech']['clips'])} LibriSpeech test-clean utterances (CC-BY 4.0, {e['corpus']['librispeech']['source']}, "
             f"tarball sha256 {e['corpus']['librispeech']['tarballSha256']}) and {len(e['corpus']['espeak']['clips'])} espeak-ng clips.", "",
             "| candidate | WER LibriSpeech | WER espeak | first partial median / p90 ms | in budget | final median / p90 ms | in budget | CPU % | peak RSS MiB | load ms | unload ms |",
             "|---|---|---|---|---|---|---|---|---|---|---|"]
    for s in e["summaries"]:
        lines.append(f"| {s['candidate']} | {s['werLibriSpeech']:.2%} | {s['werEspeak']:.2%} | {s['firstPartialMsMedian']} / {s['firstPartialMsP90']} | "
                     f"{s['firstPartialInBudget']} | {s['finalMsMedian']} / {s['finalMsP90']} | {s['finalInBudget']} | {s['cpuPctMean']} | "
                     f"{s['peakRssMiB']} | {s['loadMs'] if s['loadMs'] is not None else 'per decode'} | {s['unloadMs'] if s['unloadMs'] is not None else 'not resident'} |")
    lines += ["", "What ran:", ""]
    for s in e["summaries"]:
        vad = f", VAD {s['vadModel']} sha256 {s['vadSha256']}" if s["vadModel"] else ""
        lines.append(f"- {s['candidate']}: {s['engine']} (sha256 {s['engineSha256']}), model {s['model']} sha256 {s['modelSha256']} ({s['license']}){vad}")
    d = e["decision"]
    lines += ["", f"Decision rule: {d['rule']}.", ""]
    if d.get("default"):
        lines.append(f"Default: **{d['default']['candidate']}** ({d['default']['adapter']}).")
    if d.get("fallback"):
        lines.append(f"Fallback: **{d['fallback']['candidate']}** ({d['fallback']['adapter']}), fits budgets: {d['fallback']['fitsBudgets']}.")
    for p in e["probes"]:
        rows = ", ".join(f"{r['clip']} ({r['seconds']} s): {r['finalMs']} ms, WER {r['wer']:.2%}" for r in p["rows"])
        lines += ["", f"Probe, {p['candidate']} with audio_ctx {p['audioCtx']} instead of the adapter's sizing: {rows}."]
    held = {s["candidate"]: s["soundDeviceFds"] for s in e["summaries"] if s["soundDeviceFds"]}
    lines += ["", "Sound devices or bound unix sockets held by any engine child: " + (json.dumps(held) if held else "none") + "."]
    lines += ["", e["parakeetNote"], "", e["method"], ""]
    return "\n".join(lines)


def main():
    ap = argparse.ArgumentParser()
    sub = ap.add_subparsers(dest="cmd", required=True)
    c = sub.add_parser("corpus")
    c.add_argument("--dir", required=True)
    c.add_argument("--tarball", required=True)
    c.add_argument("--source", required=True)
    c.add_argument("--md5", required=True)
    e = sub.add_parser("espeak")
    e.add_argument("--dir", required=True)
    r = sub.add_parser("run")
    r.add_argument("--dir", required=True)
    r.add_argument("--whisper", required=True)
    r.add_argument("--out", required=True)
    a = ap.parse_args()
    {"corpus": cmd_corpus, "espeak": cmd_espeak, "run": cmd_run}[a.cmd](a)


if __name__ == "__main__":
    main()
