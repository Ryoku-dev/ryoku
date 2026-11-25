#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-only
"""Phase 11 helpers for the Oni lab: the speech fixtures, the tone-coded
clip pack, the chord FIFO, the virtual microphone's check and the phase 11
gate. It builds on labkit.py and starts lab processes only through the
lab's shell scripts.

voicekit.py fixtures DIR        speech WAVs (espeak-ng -w, never a device),
                                the sync mark and fixtures.json into DIR
voicekit.py clips DIR           the tone-coded clip pack into DIR
voicekit.py character LAB ID CHARDIR   install character ID (the testmoth
                                profile fixture, voice.engine clips) with a
                                fresh clip pack into CHARDIR
voicekit.py stage-model LAB ID  link speech model ID into the lab
voicekit.py key FIFO EVENT...   write input_event bytes into the chord FIFO
voicekit.py mic-check GRAPH MODULES LAB | --selftest
voicekit.py decode-events FILE  print the chord events a byte file holds
voicekit.py phase11 LAB OUT     the gate (phase11.sh runs it)"""
import errno
import fcntl
import json
import math
import os
import re
import shutil
import struct
import subprocess
import sys
import threading
import time
import wave
from pathlib import Path

sys.dont_write_bytecode = True
HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import labkit  # noqa: E402
from labkit import (Ledger, Phase10, comm_of, descendants, lab_pids, now, read_wav_f32,  # noqa: E402
                    sound_segments, peak_hz)

ROOT = HERE.parents[4]
APP = ROOT / "ryoku/hyakki"

# --- the chord: struct input_event (linux/input.h, 64-bit time) -------------

EVENT = struct.Struct("<qqHHi")  # tv_sec, tv_usec, type, code, value: 24 bytes
EV_SYN, EV_KEY, SYN_REPORT = 0, 1, 0
KEY_LEFTCTRL, KEY_LEFTALT, KEY_RIGHTALT, KEY_A = 29, 56, 100, 30
CHORD_EVENTS = {
    # name: [(code, value), ...], each followed by a SYN_REPORT
    "down": [(KEY_LEFTCTRL, 1), (KEY_LEFTALT, 1)],
    "up": [(KEY_LEFTALT, 0), (KEY_LEFTCTRL, 0)],
    # AltGr arrives as left Ctrl plus right Alt; it never counts as the chord
    "altgr-down": [(KEY_LEFTCTRL, 1), (KEY_RIGHTALT, 1)],
    "altgr-up": [(KEY_RIGHTALT, 0), (KEY_LEFTCTRL, 0)],
    "letter": [(KEY_A, 1), (KEY_A, 0)],
}


def event_bytes(name, wall=None):
    out = b""
    for code, value in CHORD_EVENTS[name]:
        t = time.time() if wall is None else wall
        sec, usec = int(t), int((t - int(t)) * 1e6)
        out += EVENT.pack(sec, usec, EV_KEY, code, value) + EVENT.pack(sec, usec, EV_SYN, SYN_REPORT, 0)
    return out


def decode_events(data):
    out = []
    for i in range(0, len(data) - len(data) % EVENT.size, EVENT.size):
        sec, usec, typ, code, value = EVENT.unpack_from(data, i)
        out.append({"t": sec + usec / 1e6, "type": typ, "code": code, "value": value})
    return out


class KeyWriter:
    """The writing end of the chord FIFO, held open for the run so the
    reader never sees an end of file between presses."""

    def __init__(self, path, timeout=10.0):
        self.path = str(path)
        deadline = time.monotonic() + timeout
        while True:
            try:
                self.fd = os.open(self.path, os.O_WRONLY | os.O_NONBLOCK)
                break
            except OSError as e:
                if e.errno != errno.ENXIO or time.monotonic() > deadline:
                    raise RuntimeError(f"no reader on the chord FIFO {self.path} within {timeout} s: {e}")
                time.sleep(0.1)
        flags = fcntl.fcntl(self.fd, fcntl.F_GETFL)
        fcntl.fcntl(self.fd, fcntl.F_SETFL, flags & ~os.O_NONBLOCK)
        self.log = []

    def send(self, name):
        wall = time.time()
        os.write(self.fd, event_bytes(name, wall))
        self.log.append({"wall": wall, "event": name})
        return wall

    def close(self):
        try:
            os.close(self.fd)
        except OSError:
            pass


def key_cmd(fifo, events):
    w = KeyWriter(fifo)
    try:
        for e in events:
            if e.startswith("wait:"):
                time.sleep(float(e[5:]) / 1000)
            elif e in CHORD_EVENTS:
                w.send(e)
            else:
                raise SystemExit(f"voicekit: unknown key event {e} ({', '.join(CHORD_EVENTS)}, wait:MS)")
    finally:
        w.close()
    return 0


# --- speech fixtures --------------------------------------------------------

# the lab model (model.py) as the local llama.cpp provider up.sh configures
LAB_ASK_MODEL = "llamacpp/oni-fixture-vision"

# name: (text espeak-ng says, words per minute); every one is padded with
# LEAD_S of silence before and TAIL_S after
FIXTURES = {
    "ask": ("How do rivers reach the sea?", 160),
    "time": ("What time is it?", 160),
    "timer": ("Set a timer for five seconds.", 160),
    # espeak-ng's "Hey Oni." reaches Parakeet 0.6B v2 as "Say only." at every
    # voice and rate tried, so the lab arms phrases that espeak-ng speaks
    # legibly; the default phrase's matching is voice/wake's text tests
    "wake": ("Hello robin.", 150),
    "wake2": ("Okay river.", 150),
    "nonwake": ("The purple giraffe sings at midnight.", 160),
    "yes": ("Yes.", 150),
    # 220 wpm: about 17.5 s, under maxHoldS; both engines hear it as one
    # short phrase over and over (Parakeet 0.6B v2 hears "till on")
    "loop": ("go on, " * 43, 220),
    "dictate": ("So um, we should ship the lantern report on Friday.", 160),
    "dictate_long": ("Please note that the lantern festival starts at seven, "
                     "and the boats leave the harbor at nine in the evening.", 160),
}
LEAD_S, TAIL_S = 0.25, 0.35
# words that may appear nowhere but in the target window and OUTDIR
DICTATION_MARKS = ("lantern", "harbor", "festival")
NONWAKE_MARKS = ("giraffe", "purple", "midnight")
WAKE_PHRASE, WAKE_PHRASE2 = "hello robin", "okay river"
FILLERS = ("um", "uh", "er", "erm", "hmm")
SYNC_HZ, SYNC_S, SYNC_RATE = 2000.0, 0.06, 48000


def write_wav16(path, samples, rate):
    with wave.open(str(path), "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(rate)
        w.writeframes(b"".join(struct.pack("<h", max(-32767, min(32767, int(round(s * 32767))))) for s in samples))


def read_wav16(path):
    with wave.open(str(path), "rb") as w:
        if w.getnchannels() != 1 or w.getsampwidth() != 2:
            raise ValueError(f"{path}: not 16-bit mono")
        rate, raw = w.getframerate(), w.readframes(w.getnframes())
    return [v / 32768 for v in struct.unpack(f"<{len(raw) // 2}h", raw)], rate


def render_fixtures(dst):
    dst = Path(dst)
    dst.mkdir(parents=True, exist_ok=True)
    meta = {"engine": None, "lead_s": LEAD_S, "tail_s": TAIL_S, "fixtures": {}}
    ver = subprocess.run(["espeak-ng", "--version"], capture_output=True, text=True, check=True).stdout.strip()
    meta["engine"] = ver
    for name, (text, wpm) in FIXTURES.items():
        raw = dst / f"{name}.raw.wav"
        # -w writes a file: espeak-ng never opens an audio device here
        subprocess.run(["espeak-ng", "-v", "en-us", "-s", str(wpm), "-w", str(raw), text.strip()],
                       check=True, capture_output=True, timeout=120,
                       env={"PATH": "/usr/bin:/bin", "HOME": str(dst)})
        pcm, rate = read_wav16(raw)
        raw.unlink()
        loud = [i for i, v in enumerate(pcm) if abs(v) > 0.015]
        if not loud:
            raise RuntimeError(f"espeak-ng rendered silence for {name}")
        body = pcm[max(0, loud[0] - int(0.02 * rate)):loud[-1] + int(0.05 * rate)]
        out = [0.0] * int(LEAD_S * rate) + body + [0.0] * int(TAIL_S * rate)
        write_wav16(dst / f"{name}.wav", out, rate)
        meta["fixtures"][name] = {"file": str(dst / f"{name}.wav"), "text": text.strip(), "rate": rate,
                                  "seconds": round(len(out) / rate, 3), "speech_start_s": LEAD_S,
                                  "speech_s": round(len(body) / rate, 3), "wpm": wpm}
    n = int(SYNC_S * SYNC_RATE)
    ramp = int(0.005 * SYNC_RATE)
    tone = [0.25 * math.sin(2 * math.pi * SYNC_HZ * i / SYNC_RATE) * min(1.0, i / ramp, (n - 1 - i) / ramp)
            for i in range(n)]
    write_wav16(dst / "sync.wav", tone, SYNC_RATE)
    meta["sync"] = {"file": str(dst / "sync.wav"), "hz": SYNC_HZ, "seconds": SYNC_S}
    (dst / "fixtures.json").write_text(json.dumps(meta, indent=2) + "\n")
    print(f"rendered {len(FIXTURES)} speech fixtures and the sync mark into {dst} ({ver})")
    return meta


# --- the tone-coded clip pack -----------------------------------------------

def clip_vocabulary():
    """Every clip id and its text: what voice/clips composes times, dates and
    counts from (compose.go), the route's local lines ("It's 3:45.",
    "Timer set, 5 seconds.", "Time's up.", "Press the key to answer.") and
    the acknowledgements."""
    words = ["zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten", "eleven",
             "twelve", "thirteen", "fourteen", "fifteen", "sixteen", "seventeen", "eighteen", "nineteen",
             "twenty", "thirty", "forty", "fifty", "sixty", "seventy", "eighty", "ninety", "hundred",
             "first", "second", "third", "fourth", "fifth", "sixth", "seventh", "eighth", "ninth", "tenth",
             "eleventh", "twelfth", "thirteenth", "fourteenth", "fifteenth", "sixteenth", "seventeenth",
             "eighteenth", "nineteenth", "twentieth", "thirtieth",
             "sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday",
             "january", "february", "march", "april", "may", "june", "july", "august", "september",
             "october", "november", "december",
             "past", "timer", "set", "seconds", "minute", "minutes", "hour", "hours", "up",
             "press", "the", "key", "to", "answer", "on", "it", "got", "looking", "now", "moment", "right",
             "away", "is", "time", "a", "and"]
    vocab = [(w, w) for w in words]
    vocab += [("oclock", "o'clock"), ("its", "it's"), ("times", "time's")]
    return vocab


def clip_codes():
    """id -> tone frequency: 25 Hz apart from 400 Hz, well inside the
    recording's 48 kHz and the speech sink's 24 kHz."""
    return {cid: 400.0 + 25.0 * k for k, (cid, _) in enumerate(clip_vocabulary())}


CLIP_RATE, CLIP_TONE_S, CLIP_GAP_S = 24000, 0.12, 0.03


def render_clips(dst, codes_file=None):
    dst = Path(dst)
    dst.mkdir(parents=True, exist_ok=True)
    codes = clip_codes()
    n, ramp, gap = int(CLIP_TONE_S * CLIP_RATE), int(0.005 * CLIP_RATE), int(CLIP_GAP_S * CLIP_RATE)
    manifest = {"version": 1, "clips": {}}
    for cid, text in clip_vocabulary():
        hz = codes[cid]
        tone = [0.3 * math.sin(2 * math.pi * hz * i / CLIP_RATE) * min(1.0, i / ramp, (n - 1 - i) / ramp)
                for i in range(n)]
        write_wav16(dst / f"{cid}.wav", [0.0] * gap + tone + [0.0] * gap, CLIP_RATE)
        manifest["clips"][cid] = {"file": f"{cid}.wav", "text": text}
    (dst / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")
    (dst / "LICENSE").write_text(
        "GPL-3.0-only\n\nThese files are pure tones generated by release/lab/oni/voicekit.py at test\n"
        "time for the Oni lab. They record no voice and are never committed.\n")
    (dst / "PROVENANCE.md").write_text(
        "# Lab clip pack\n\nGenerated by `voicekit.py clips` when the Phase 11 lab runs: each clip is a\n"
        f"{int(CLIP_TONE_S * 1000)} ms sine tone whose frequency names its id (400 Hz plus 25 Hz per id in\n"
        "vocabulary order), so the lab can read back from recorded PCM which clips\n"
        "were played and in what order. No recording, no person's voice.\n")
    codes_file = Path(codes_file) if codes_file else dst.parent / f"{dst.name}-codes.json"
    codes_file.parent.mkdir(parents=True, exist_ok=True)
    codes_file.write_text(json.dumps(codes, indent=1) + "\n")
    print(f"generated {len(codes)} tone-coded clips into {dst}")
    return codes


def install_character(lab, cid, chardir):
    src = APP / "internal/oni/profile/testdata/testmoth"
    if not re.fullmatch(r"[a-z0-9]+(-[a-z0-9]+)*", cid):
        raise SystemExit(f"voicekit: character id {cid!r} is not a lower-case id")
    chardir = Path(chardir)
    if not str(chardir.resolve()).startswith(str(Path(lab).resolve()) + "/"):
        raise SystemExit("voicekit: the characters folder must be inside the lab")
    dst = chardir / cid
    if dst.exists():
        shutil.rmtree(dst)
    shutil.copytree(src, dst, symlinks=False)
    profile = json.loads((dst / "profile.json").read_text())
    profile.update(id=cid, name="Lab clips")
    profile["voice"] = {"engine": "clips", "clipPack": "voice", "speed": 1.0, "honorific": "none"}
    (dst / "profile.json").write_text(json.dumps(profile, indent=2) + "\n")
    # the frequency table goes to the lab's voice folder, never into the pack
    render_clips(dst / "voice", Path(lab) / "voice" / f"{cid}-codes.json")
    print(json.dumps({"character": cid, "path": str(dst)}))
    return 0


# --- speech models ----------------------------------------------------------

# voice/stt's ModelDir under the lab's XDG_DATA_HOME
SPEECH_SUBDIR = os.environ.get("ONI_LAB_SPEECH_SUBDIR", "hyakki/oni/voice/models")


def stt_catalogue():
    src = (APP / "internal/oni/voice/stt/models.go").read_text()
    out = {}
    for m in re.finditer(r'\{ID:\s*"([^"]+)",\s*Engine:\s*(\w+),\s*File:\s*"([^"]+)".*?Size:\s*(\d+),\s*SHA256:\s*"([0-9a-f]{64})"',
                         src, re.S):
        out[m.group(1)] = {"engine": m.group(2), "file": m.group(3), "size": int(m.group(4)), "sha256": m.group(5)}
    return out


def stt_defaults():
    """voice/stt's DefaultModel, FallbackModel and FallbackVAD ids."""
    src = (APP / "internal/oni/voice/stt/models.go").read_text()
    out = []
    for name in ("DefaultModel", "FallbackModel", "FallbackVAD"):
        m = re.search(name + r'\s*=\s*"([^"]+)"', src)
        if not m:
            raise SystemExit(f"voicekit: stt's {name} is not in models.go")
        out.append(m.group(1))
    return out


def stage_model(lab, mid):
    import hashlib
    cat = stt_catalogue()
    if mid not in cat:
        raise SystemExit(f"voicekit: {mid} is not in voice/stt's catalogue ({', '.join(cat)})")
    m = cat[mid]
    dirs = [os.environ.get("ONI_LAB_SPEECH_SOURCE"),
            os.path.join(os.environ.get("VOICEBENCH", "/dev/shm/hyakki-voicebench"), "models"),
            "/usr/share/ryoku/dictation/models"]
    src = next((Path(d) / m["file"] for d in dirs if d and (Path(d) / m["file"]).is_file()), None)
    if src is None:
        raise SystemExit(f"voicekit: {m['file']} is in none of {[d for d in dirs if d]} "
                         "(voicebench.sh stage fetches it, or set ONI_LAB_SPEECH_SOURCE)")
    if src.stat().st_size != m["size"]:
        raise SystemExit(f"voicekit: {src} is {src.stat().st_size} bytes, the pin says {m['size']}")
    h = hashlib.sha256()
    with src.open("rb") as fh:
        for block in iter(lambda: fh.read(1 << 20), b""):
            h.update(block)
    if h.hexdigest() != m["sha256"]:
        raise SystemExit(f"voicekit: {src} does not hash to its pin")
    dst_dir = Path(lab) / "data" / SPEECH_SUBDIR
    dst_dir.mkdir(parents=True, exist_ok=True)
    dst = dst_dir / m["file"]
    if dst.exists():
        dst.unlink()
    try:
        os.link(src, dst)
        how = "linked"
    except OSError:
        shutil.copyfile(src, dst)
        how = "copied"
    print(json.dumps({"id": mid, "engine": m["engine"], "path": str(dst), "source": str(src), "how": how}))
    return 0


# --- the virtual microphone's check -----------------------------------------

LAB_SINKS = {"oni_lab", "oni_lab_mic"}
LAB_SOURCES = {"oni_lab.monitor", "oni_lab_mic.monitor", "oni_lab_mic_in"}
LAB_MODULES = {"module-null-sink", "module-remap-source", "module-loopback", "module-always-sink"}
# what pactl lists for the lab's own PipeWire daemon and pipewire-pulse
# before any lab module loads: the core's protocol, factory and session
# modules and pulse's restore modules. None of them opens a device; a
# device would show as a node with device.api, which is refused above.
LAB_CORE_MODULES = {"libpipewire-module-protocol-native", "libpipewire-module-profiler",
                    "libpipewire-module-metadata", "libpipewire-module-spa-device-factory",
                    "libpipewire-module-spa-node-factory", "libpipewire-module-client-node",
                    "libpipewire-module-client-device", "libpipewire-module-access",
                    "libpipewire-module-adapter", "libpipewire-module-link-factory",
                    "libpipewire-module-session-manager", "module-device-manager", "module-device-restore",
                    "module-stream-restore"}


def mic_check(graph, modules, lab):
    problems = []
    nodes = graph["sinks"] + graph["sources"]
    devices = [n.get("name") for n in nodes if (n.get("properties") or {}).get("device.api")]
    if devices:
        problems.append(f"device nodes {devices}")
    sinks = {n.get("name") for n in graph["sinks"]}
    sources = {n.get("name") for n in graph["sources"]}
    if sinks != LAB_SINKS:
        problems.append(f"sinks {sorted(sinks)} are not {sorted(LAB_SINKS)}")
    if sources != LAB_SOURCES:
        problems.append(f"sources {sorted(sources)} are not {sorted(LAB_SOURCES)}")
    info = graph["info"]
    if info.get("default_sink_name") != "oni_lab":
        problems.append(f"default sink {info.get('default_sink_name')}")
    if info.get("default_source_name") != "oni_lab_mic_in":
        problems.append(f"default source {info.get('default_source_name')}")
    server = info.get("server_string", "")
    if not server.startswith(lab + "/"):
        problems.append(f"server {server} is not inside {lab}")
    remap = []
    for m in modules:
        name, arg = m.get("name"), m.get("argument") or ""
        if name in LAB_CORE_MODULES:
            continue
        if name not in LAB_MODULES:
            problems.append(f"module {name} ({arg}) is not a lab module")
        elif name == "module-remap-source":
            remap.append(arg)
            if "master=oni_lab_mic.monitor" not in arg or "source_name=oni_lab_mic_in" not in arg:
                problems.append(f"remap source {arg} is not oni_lab_mic_in over oni_lab_mic.monitor")
        elif name == "module-loopback":
            if not (re.search(r"(^|\s)source=oni_lab\.monitor(\s|$)", arg) and re.search(r"(^|\s)sink=oni_lab_mic(\s|$)", arg)):
                problems.append(f"loopback {arg} is not oni_lab.monitor into oni_lab_mic")
        elif name == "module-null-sink":
            if not re.search(r"sink_name=(oni_lab|oni_lab_mic)(\s|$)", arg):
                problems.append(f"null sink {arg} is not a lab sink")
    if len(remap) != 1:
        problems.append(f"{len(remap)} remap sources")
    return {"server": server, "sinks": sorted(sinks), "sources": sorted(sources),
            "default_source": info.get("default_source_name"), "modules": [m.get("name") for m in modules],
            "problems": problems}


def mic_check_selftest():
    """The guard against doctored graphs: a clean lab graph passes, every
    graph with a device, a foreign default or a foreign module fails."""
    lab = "/dev/shm/oni-lab"

    def node(name, **props):
        return {"name": name, "properties": props}

    good_graph = {"info": {"server_string": lab + "/rt/pulse/native", "default_sink_name": "oni_lab",
                           "default_source_name": "oni_lab_mic_in"},
                  "sinks": [node("oni_lab"), node("oni_lab_mic")],
                  "sources": [node("oni_lab.monitor"), node("oni_lab_mic.monitor"), node("oni_lab_mic_in")]}
    good_mods = [{"name": "module-null-sink", "argument": "sink_name=oni_lab sink_properties=x"},
                 {"name": "module-null-sink", "argument": "sink_name=oni_lab_mic sink_properties=x"},
                 {"name": "module-remap-source", "argument": "source_name=oni_lab_mic_in master=oni_lab_mic.monitor channels=1"},
                 {"name": "module-loopback", "argument": "source=oni_lab.monitor sink=oni_lab_mic latency_msec=20"}] + [
        {"name": name, "argument": ""} for name in sorted(LAB_CORE_MODULES)]
    cases = {}

    def bad(name, edit_graph=None, edit_mods=None):
        g, m = json.loads(json.dumps(good_graph)), json.loads(json.dumps(good_mods))
        if edit_graph:
            edit_graph(g)
        if edit_mods:
            edit_mods(m)
        cases[name] = mic_check(g, m, lab)["problems"]

    bad("a hardware microphone", lambda g: g["sources"].append(node("alsa_input.pci-0000_00_1f.3.analog-stereo",
                                                                   **{"device.api": "alsa"})))
    bad("a hardware speaker", lambda g: g["sinks"].append(node("alsa_output.usb", **{"device.api": "alsa"})))
    bad("a bluetooth headset source", lambda g: g["sources"].append(node("bluez_input.00", **{"device.api": "bluez5"})))
    bad("the default source a monitor", lambda g: g["info"].update(default_source_name="oni_lab.monitor"))
    bad("the default source foreign", lambda g: g["info"].update(default_source_name="alsa_input.pci"))
    bad("the server outside the lab", lambda g: g["info"].update(server_string="/run/user/1000/pulse/native"))
    bad("a loopback from a device", edit_mods=lambda m: m.append(
        {"name": "module-loopback", "argument": "source=alsa_input.pci sink=oni_lab_mic"}))
    bad("a loopback into a speaker", edit_mods=lambda m: m.append(
        {"name": "module-loopback", "argument": "source=oni_lab.monitor sink=alsa_output.pci"}))
    bad("an alsa module", edit_mods=lambda m: m.append({"name": "module-alsa-source", "argument": "device=hw:0"}))
    bad("a tunnel to another server", edit_mods=lambda m: m.append({"name": "module-tunnel-sink", "argument": "server=x"}))
    bad("a network audio module", edit_mods=lambda m: m.append({"name": "libpipewire-module-roc-source", "argument": ""}))
    bad("a remap over a device", edit_mods=lambda m: m.__setitem__(2, {
        "name": "module-remap-source", "argument": "source_name=oni_lab_mic_in master=alsa_input.pci"}))
    clean = mic_check(good_graph, good_mods, lab)["problems"]
    report = {"clean_lab_graph_problems": clean, "refused": {k: bool(v) for k, v in cases.items()}}
    ok = not clean and all(cases.values())
    print(json.dumps(report, indent=1))
    return 0 if ok else 1


# --- text helpers -----------------------------------------------------------

def norm_words(text):
    text = (text or "").lower().replace("’", "'")
    text = re.sub(r"[^a-z0-9' ]+", " ", text)
    return [w.strip("'") for w in text.split() if w.strip("'")]


def wer(ref, hyp):
    r, h = norm_words(ref), norm_words(hyp)
    if not r:
        return 0.0 if not h else 1.0
    d = list(range(len(h) + 1))
    for i in range(1, len(r) + 1):
        prev, d[0] = d[0], i
        for j in range(1, len(h) + 1):
            cur = d[j]
            d[j] = min(d[j] + 1, d[j - 1] + 1, prev + (r[i - 1] != h[j - 1]))
            prev = cur
    return round(d[len(h)] / len(r), 3)


def first_sentence(text):
    m = re.match(r"(.+?[.!?])(\s|$)", text.strip())
    return m.group(1) if m else text.strip()


def asked_text(q):
    """The question in one model request: an Ask puts it last, after its
    reminder and the voice block, so it is the user turn's tail after its
    last blank line (model.py keeps that tail)."""
    return (q.get("tail") or q.get("request") or "").rsplit("\n\n", 1)[-1]


def max_repeat(words, longest=4):
    """The most times any phrase of 1..LONGEST words follows itself in a
    row in WORDS (1 when nothing repeats, 0 for no words)."""
    best = 1 if words else 0
    for n in range(1, longest + 1):
        for i in range(len(words) - n + 1):
            k = 1
            while words[i + k * n:i + (k + 1) * n] == words[i:i + n]:
                k += 1
            best = max(best, k)
    return best


def clock_ids(hour, minute):
    """compose.go's Time: 15:45 is forty five past three, 9:00 nine oclock."""
    ones = ["zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten", "eleven",
            "twelve", "thirteen", "fourteen", "fifteen", "sixteen", "seventeen", "eighteen", "nineteen"]
    tens = {2: "twenty", 3: "thirty", 4: "forty", 5: "fifty"}

    def number(n):
        if n < 20:
            return [ones[n]]
        return [tens[n // 10]] if n % 10 == 0 else [tens[n // 10], ones[n % 10]]

    h = hour % 12 or 12
    return number(h) + ["oclock"] if minute == 0 else number(minute) + ["past"] + number(h)


def contains_run(seq, run):
    return any(seq[i:i + len(run)] == run for i in range(len(seq) - len(run) + 1)) if run else True


# --- watchers ---------------------------------------------------------------

class FrameWatch(threading.Thread):
    """`hyakki oni watch` in the lab, restarted whenever it ends (the daemon
    may exit idle between steps and start again on the next call), keeping
    every frame line with the wall time it arrived."""

    def __init__(self, lab_id):
        super().__init__(daemon=True)
        self.lab_id, self.lines, self.done, self.restarts = lab_id, [], threading.Event(), 0
        self.proc = None

    def run(self):
        while not self.done.is_set():
            self.proc = subprocess.Popen([str(HERE / "ctl.sh"), "hyakki", "oni", "watch"], stdout=subprocess.PIPE,
                                         stderr=subprocess.DEVNULL, stdin=subprocess.DEVNULL, text=True)
            for raw in self.proc.stdout:
                try:
                    self.lines.append({"wall": time.time(), "frame": json.loads(raw)})
                except ValueError:
                    continue
            self.proc.wait()
            if not self.done.is_set():
                self.restarts += 1
                self.done.wait(0.5)

    def mark(self):
        return len(self.lines)

    def frames(self, kind, since=0, until=None):
        return [(l["wall"], l["frame"].get("data") or {}) for l in self.lines[since:until]
                if (l["frame"] or {}).get("kind") == kind]

    def wait(self, kind, pred, since, timeout, what):
        deadline = time.monotonic() + timeout
        i = since
        while True:
            while i < len(self.lines):
                l = self.lines[i]
                f = l["frame"] or {}
                if f.get("kind") == kind and pred(f.get("data") or {}):
                    return l["wall"], f.get("data") or {}, i + 1
                i += 1
            if time.monotonic() >= deadline:
                if what is None:
                    return None, None, i
                raise RuntimeError(f"no {kind} frame {what} within {timeout} s")
            time.sleep(0.05)

    def stop(self):
        self.done.set()
        for pid in lab_pids(self.lab_id, "hyakki"):
            try:
                if b"watch" in open(f"/proc/{pid}/cmdline", "rb").read():
                    os.kill(pid, 15)
            except OSError:
                pass
        if self.proc:
            try:
                self.proc.wait(timeout=10)
            except subprocess.TimeoutExpired:
                self.proc.kill()
        if self.is_alive():
            self.join(timeout=10)


class ClockWatch(threading.Thread):
    """wall - ledger t, sampled: the newest ledger line is never ahead of
    now, so the smallest difference since the surface started is the clock
    offset. Reset when a new ledger file appears."""

    def __init__(self, path, period=0.1):
        super().__init__(daemon=True)
        self.ledger, self.period, self.done = Ledger(path), period, threading.Event()
        self.best, self.ino = None, None

    def run(self):
        while not self.done.is_set():
            lines = self.ledger.since()
            mark = self.ledger.mark()
            if mark[0] != self.ino:
                self.ino, self.best = mark[0], None
            if lines and "t" in lines[-1]:
                d = time.time() - lines[-1]["t"]
                self.best = d if self.best is None else min(self.best, d)
            self.done.wait(self.period)

    def to_wall(self, t):
        return None if self.best is None else t + self.best


# programs the voice pipeline may run under the daemon, by comm (15 chars)
CAPTURE_PROGRAMS = ("pw-cat", "pw-record", "parec")
PLAYER_PROGRAMS = ("pw-cat", "pw-play", "paplay")
ENGINE_PROGRAMS = ("whisper-server", "whisper-cli", "espeak-ng", "speech-dispatch", "spd-say",
                   "sherpa-onnx-off", "sherpa-onnx", "ryoku-dictate", "transcribe")
INPUT_PROGRAMS = labkit.INPUT_PROGRAMS + ("wl-copy", "wl-paste")


def proc_env(pid, keys=("XDG_RUNTIME_DIR", "PIPEWIRE_REMOTE", "PIPEWIRE_RUNTIME_DIR", "PULSE_SERVER",
                        "PULSE_RUNTIME_PATH", "WAYLAND_DISPLAY")):
    env = {}
    try:
        for entry in open(f"/proc/{pid}/environ", "rb").read().split(b"\0"):
            k, _, v = entry.decode(errors="replace").partition("=")
            if k in keys:
                env[k] = v
    except OSError:
        pass
    return env


def rss_kb(pid):
    try:
        for line in open(f"/proc/{pid}/status"):
            if line.startswith("VmRSS:"):
                return int(line.split()[1])
    except (OSError, ValueError):
        pass
    return None


class VoiceChildWatch(threading.Thread):
    """Every process below the lab's oni daemons: argv, the environment that
    decides which audio server and display it reaches, when it was first and
    last seen and its largest RSS. A child caught between its clone and its
    exec still shows its daemon's own image (Go starts children with
    CLONE_VM|CLONE_VFORK, so the child's /proc cmdline, comm and RSS are the
    daemon's until execve); it is read once it shows its own, and one that
    exits first counts as exited unread."""

    def __init__(self, lab_id, period=0.05):
        super().__init__(daemon=True)
        self.lab_id, self.period, self.done = lab_id, period, threading.Event()
        self.seen, self.passes, self.unread = {}, 0, {}
        # pid -> (comm, daemon) for children still showing their daemon's image
        self.pre_exec = {}

    @staticmethod
    def cmdline(pid):
        try:
            return [a.decode("utf-8", "replace")
                    for a in open(f"/proc/{pid}/cmdline", "rb").read().split(b"\0")[:-1]]
        except OSError:
            return None

    @staticmethod
    def executable(pid):
        try:
            return os.readlink(f"/proc/{pid}/exe")
        except OSError:
            return None

    def run(self):
        daemons, found_at, images = [], 0.0, {}
        while not self.done.is_set():
            self.passes += 1
            if time.monotonic() - found_at >= 1.0:
                daemons, found_at = lab_pids(self.lab_id, "oni"), time.monotonic()
                images = {d: (comm_of(d), self.cmdline(d)) for d in daemons}
            wall = time.time()
            live = set()
            for d in daemons:
                for pid in descendants(d):
                    live.add(pid)
                    rec = self.seen.get(pid)
                    if rec is not None:
                        if rec.get("exe") is None:
                            rec["exe"] = self.executable(pid)
                        rec["last"] = wall
                        r = rss_kb(pid)
                        if r is not None and (rec["rss_kb_max"] or 0) < r:
                            rec["rss_kb_max"] = r
                        continue
                    comm = comm_of(pid)
                    argv = self.cmdline(pid)
                    if argv is None:
                        continue
                    if not argv:
                        self.unread[pid] = comm
                        continue
                    if (comm, argv) == images.get(d):
                        self.pre_exec[pid] = (comm, d)
                        continue
                    self.pre_exec.pop(pid, None)
                    # A short-lived child can exit while its parent is sampled.
                    executable = self.executable(pid)
                    parent_pid = labkit.ppid_of(pid)
                    parent = {"pid": parent_pid, "comm": comm_of(parent_pid),
                              "argv": self.cmdline(parent_pid), "exe": self.executable(parent_pid)}
                    self.seen[pid] = {"pid": pid, "daemon": d, "comm": comm, "argv": argv, "exe": executable,
                                      "parent": parent, "env": proc_env(pid), "first": wall, "last": wall, "rss_kb_max": rss_kb(pid)}
            for pid in [p for p in self.pre_exec if p not in live]:
                self.unread[pid] = self.pre_exec.pop(pid)[0]
            self.done.wait(self.period)

    def procs(self, comms=None, since=None, until=None):
        out = []
        for rec in self.seen.values():
            if comms is not None and rec["comm"] not in comms:
                continue
            if since is not None and rec["last"] < since:
                continue
            if until is not None and rec["first"] > until:
                continue
            out.append(rec)
        return out

    def alive(self, comms, slack=0.3):
        t = time.time()
        return [r for r in self.procs(comms) if r["last"] >= t - slack and os.path.exists(f"/proc/{r['pid']}")]


def is_capture(rec):
    prog, argv = rec["comm"], rec["argv"]
    return prog in ("parec", "pw-record") or (prog == "pw-cat" and any(a in ("--record", "-r") for a in argv))


# --- recordings -------------------------------------------------------------

class Rec:
    """A recording of oni_lab (what Oni plays), optionally of oni_lab_mic
    (what the microphone hears), with a sync tone played into oni_lab right
    after it starts: wall = recording time + offset."""

    def __init__(self, kit, name, mic=False):
        self.kit, self.name = kit, name
        self.wav = kit.out / "audio" / f"{name}.wav"
        self.mic_wav = kit.out / "audio" / f"{name}-mic.wav" if mic else None
        kit.sh("audio.sh", "record", self.wav)
        deadline = time.monotonic() + 10
        while "oni-lab-recorder" not in kit.sh("audio.sh", "recorders").stdout.split("\n"):
            if time.monotonic() > deadline:
                raise RuntimeError("the recorder never joined the graph")
            time.sleep(0.1)
        if mic:
            kit.sh("audio.sh", "record-mic", self.mic_wav)
        time.sleep(0.3)
        # paplay starts through audio.sh and systemd-run about 200 ms after
        # the call, and returns once the tone has drained: the tone began its
        # length before the return, a far closer wall time than the call's
        self.sync_call_wall = time.time()
        kit.sh("audio.sh", "play", kit.fx["sync"]["file"])
        self.sync_wall = time.time() - kit.fx["sync"]["seconds"]
        self.pcm = self.rate = self.mic_pcm = self.offset = None

    def stop(self):
        time.sleep(0.3)
        self.kit.sh("audio.sh", "stop")
        time.sleep(0.3)
        self.pcm, self.rate = read_wav_f32(self.wav)
        segs = sound_segments(self.pcm, self.rate, floor=0.02, gap_s=0.01)
        sync = next((s for s in segs if 0.03 <= s[1] - s[0] <= 0.12), None)
        self.offset = None if sync is None else self.sync_wall - sync[0]
        self.sync_rec = None if sync is None else sync[0]
        if self.mic_wav:
            self.mic_pcm, _ = read_wav_f32(self.mic_wav)
        return self

    def to_rec(self, wall):
        return None if self.offset is None or wall is None else wall - self.offset

    def segments(self, floor=1e-3, gap_s=0.03, after=None):
        segs = sound_segments(self.pcm, self.rate, floor=floor, gap_s=gap_s)
        if self.sync_rec is not None:
            segs = [s for s in segs if s[0] > self.sync_rec + SYNC_S + 0.02]
        return [s for s in segs if after is None or s[1] > after]

    def peak(self, a, b, pcm=None):
        pcm = self.pcm if pcm is None else pcm
        i, j = max(0, int(a * self.rate)), max(0, int(b * self.rate))
        return float(max(abs(v) for v in pcm[i:j])) if j > i else 0.0

    def utterance(self, lo, hi, quiet_s=0.2):
        """The first segment starting within [LO, HI] recording seconds
        that follows at least QUIET_S of silence: where an utterance
        begins, not a word after one of its own gaps (espeak leaves tens
        of ms of digital silence between words, and the ack and the reply
        follow each other)."""
        segs = self.segments()
        for k, s in enumerate(segs):
            prev = segs[k - 1][1] if k else (self.sync_rec + SYNC_S if self.sync_rec is not None else 0.0)
            if lo <= s[0] <= hi and s[0] - prev >= quiet_s:
                return s
        return None

    def speech_end(self, start, floor=1e-4, hold_s=0.2):
        """When speech that starts at START (recording seconds) ends: the
        first moment its 5 ms envelope stays under FLOOR for HOLD_S, the
        measure step 2 uses for a cut reply. Gaps between words are
        shorter than HOLD_S."""
        env, win = envelope(self.pcm, self.rate)
        k, hold = int(max(0.0, start) / win), int(hold_s / win)
        while k + hold < len(env):
            if (env[k:k + hold] < floor).all():
                return k * win
            k += 1
        return None


def envelope(pcm, rate, win_s=0.005):
    import numpy as np
    n = max(1, int(win_s * rate))
    m = len(pcm) // n
    x = np.asarray(pcm[:m * n], dtype=np.float64).reshape(m, n)
    return np.sqrt((x * x).mean(axis=1)), win_s


def cut_against_reference(pcm, rate, onset_s, ref, chord_s, ramp_ms, audible=2e-3):
    """Step 2's cut, sample by sample: the barged reply (PCM from ONSET_S,
    recording seconds) against the same sentence played uncut in step 1
    (REF, from its onset). The TTS renders the sentence identically, so
    the two agree exactly until the cut; from the first sample that
    differs, the gain barged/reference shows the fade itself. espeak leaves
    tens of ms of digital silence between words: a cut there fades nothing
    audible, and the time from the last sound to the end of such a gap is
    no fade. None when nothing differs (the reply played out)."""
    import numpy as np
    i0 = int(round(onset_s * rate))
    n0, span = int(0.2 * rate), int(0.002 * rate)
    r0 = np.asarray(ref[:n0], dtype=np.float64)

    def score(k):
        seg = pcm[i0 + k:i0 + k + n0]
        return float(np.dot(r0[:len(seg)], seg)) if i0 + k >= 0 else float("-inf")
    i0 += max(range(-span, span + 1), key=score)
    n = min(len(ref), len(pcm) - i0)
    b = np.asarray(pcm[i0:i0 + n], dtype=np.float64)
    r = np.asarray(ref[:n], dtype=np.float64)
    loud = np.abs(r) > audible
    same = np.abs(b - r) <= 0.02 * np.abs(r) + 1e-5
    altered = np.flatnonzero(loud & ~same)
    if not len(altered):
        return None
    i_alt = int(altered[0])
    kept = np.flatnonzero(loud[:i_alt] & same[:i_alt])
    i_same = int(kept[-1]) if len(kept) else 0
    sound = np.flatnonzero(np.abs(b[:i_alt + int(0.5 * rate)]) > 1e-5)
    i_last = int(sound[-1]) if len(sound) else i_same
    k = np.arange(i_alt, max(i_alt, i_last) + 1)
    k = k[loud[k]]
    g = b[k] / r[k]
    mid = (g > 0.02) & (g < 0.98)
    out = {"onset_rec_s": round(i0 / rate, 4), "last_unchanged_s": round(i_same / rate, 4),
           "first_changed_s": round(i_alt / rate, 4), "last_sound_s": round(i_last / rate, 4),
           "chord_to_cut_ms": round((i0 + i_alt) / rate * 1000 - chord_s * 1000, 1),
           "chord_to_silence_ms": round((i0 + i_last + 1) / rate * 1000 - chord_s * 1000, 1),
           "spoken_s": round((i_last + 1) / rate, 3), "faded_samples": int(mid.sum())}
    if mid.sum() >= 8:
        # the fade's gain is linear in time: its slope gives its length
        slope, _ = np.polyfit(k[mid], g[mid], 1)
        out["gain_samples"] = [[round(float(t) / rate, 5), round(float(v), 3)] for t, v in zip(k[mid][::8], g[mid][::8])]
        out["fade_ms"] = round(float(-1000.0 / slope / rate), 1) if slope < 0 else None
        out["fade"] = "audible"
        out["pass"] = bool(out["fade_ms"] is not None and out["fade_ms"] <= ramp_ms)
    else:
        # no partial gain on audible samples: either the whole fade fell in
        # a silence of the sentence (nothing audible was faded), or the
        # sound stopped dead on audible speech, a click
        quiet = int(np.sum(~loud[i_same + 1:i_alt]))
        out["silence_at_cut_ms"] = round(quiet / rate * 1000, 1)
        out["fade_ms"] = 0.0
        out["fade"] = "inside a silence" if quiet / rate * 1000 >= ramp_ms else "click"
        out["pass"] = out["fade"] == "inside a silence"
    return out


# --- the gate ---------------------------------------------------------------

class Phase11(Phase10):
    REPORT = "phase11.json"

    def __init__(self, lab, out):
        super().__init__(lab, out)
        for d in ("audio", "dictation"):
            (self.out / d).mkdir(parents=True, exist_ok=True)
        self.vspec = self.spec["voice"]
        self.gating = self.spec["sound"]["gating"]
        self.fx = json.loads((self.lab / "voice/fixtures/fixtures.json").read_text())
        self.clip_char = os.environ.get("ONI_LAB_CLIP_CHARACTER", "labclips")
        codes = self.lab / "voice" / f"{self.clip_char}-codes.json"
        self.codes = json.loads(codes.read_text()) if codes.exists() else clip_codes()
        self.acks = set(json.loads((APP / "internal/oni/characters/oni/lines.json").read_text())["acknowledge"])
        from model import ANSWER
        self.answer = ANSWER
        # with neither variable set the settings keep Hyakki's choice: the
        # daemon picks stt's DefaultModel, else FallbackModel (with its VAD)
        self.stt = {"engine": os.environ.get("ONI_LAB_STT_ENGINE", ""),
                    "model": os.environ.get("ONI_LAB_STT_MODEL", "")}
        self.vchildren = VoiceChildWatch(self.env["LAB_ID"])
        self.watch = FrameWatch(self.env["LAB_ID"])
        self.clock = None
        self.keys = None
        self.dictations = []  # (start wall, end wall) of every dictation
        self.ladder = {"runs": []}
        self.unbarged = self.unbarged_pcm = None
        self.results["voice"] = {
            "stt": self.stt, "fixtures": {k: {"text": v["text"], "seconds": v["seconds"]}
                                          for k, v in self.fx["fixtures"].items()},
            "fixture_engine": self.fx["engine"],
            "fixture_note": "speech is espeak-ng rendered at run time into the lab, played into the virtual "
                            "microphone oni_lab_mic; it proves the pipeline, not recognition of human speech",
            "key_source": "the chord FIFO (HYAKKI_ONI_KEYS_FIFO, lab only) carrying struct input_event bytes; the "
                          "real /dev/input open is the owner's manual check (decision 9)"}

    # --- plumbing ---

    def step(self, number, name, fn):
        try:
            return super().step(number, name, fn)
        finally:
            # a step that failed midway leaves its chord held and its
            # recorders running; let go and stop them so the next step
            # starts from a released chord and a clean graph
            if self.keys and self.keys.log and self.keys.log[-1]["event"] == "down":
                self.keys.send("up")
            self.sh("audio.sh", "stop", check=False)

    def vset(self, **voice_settings):
        return self.settings(voiceSettings=voice_settings)

    def inject(self, name):
        t0 = time.time()
        self.sh("audio.sh", "inject", self.fx["fixtures"][name]["file"], timeout=120)
        return t0, time.time()

    def inject_async(self, name):
        out = {}

        def go():
            out["t0"], out["t1"] = self.inject(name)
        th = threading.Thread(target=go, daemon=True)
        th.start()
        return th, out

    def voice_since(self, mark, until=None):
        return self.watch.frames("voice", mark, until)

    def wait_voice(self, mark, pred, timeout, what):
        return self.watch.wait("voice", pred, mark, timeout, what)

    def is_ack(self, text):
        return (text or "").strip() in self.acks

    def hold(self, name, lead=0.3, tail=0.4, chord_wait=3.0):
        """Hold the chord, play fixture NAME into the microphone, let go."""
        r = {"fixture": name, "mark": self.watch.mark()}
        r["down"] = self.keys.send("down")
        w, d, _ = self.wait_voice(r["mark"], lambda d: d.get("phase") == "listening", chord_wait, "listening")
        r["listening"] = w
        r["listening_source"] = d.get("source")
        time.sleep(lead)
        r["inject"] = self.inject(name)
        time.sleep(tail)
        r["up"] = self.keys.send("up")
        return r

    def final_of(self, r, timeout=None):
        """Partials while held, and the final transcript: the last
        transcribing frame before the first speaking, idle or armed frame
        after the release."""
        timeout = timeout or self.vspec["budgets"]["finalS"] + 12
        deadline = time.monotonic() + timeout
        partials, final, end = [], None, None
        while time.monotonic() < deadline:
            frames = self.voice_since(r["mark"])
            partials = [(w, d.get("text", "")) for w, d in frames if d.get("phase") == "transcribing" and w <= r["up"]]
            after = [(w, d) for w, d in frames if w > r["up"]]
            end = next(((w, d) for w, d in after if d.get("phase") in ("speaking", "idle", "armed")), None)
            if end is not None:
                before = [(w, d.get("text", "")) for w, d in frames if d.get("phase") == "transcribing" and w <= end[0]]
                final = before[-1] if before else None
                break
            time.sleep(0.1)
        r.update(partials=partials, final=final, end=end)
        return r

    def speaking_after(self, mark, timeout, ack=None):
        return self.wait_voice(mark, lambda d: d.get("phase") == "speaking" and (ack is None or self.is_ack(d.get("text")) == ack),
                               timeout, "speaking" + ("" if ack is None else (" an ack" if ack else " a reply")))

    def wait_idle(self, mark, timeout, phases=("idle",)):
        return self.wait_voice(mark, lambda d: d.get("phase") in phases, timeout, f"{'/'.join(phases)}")

    def model_requests(self):
        try:
            return self.model.requests()
        except OSError:
            return []

    def ledger_lines(self, mark=(None, 0)):
        return self.ledger.since(mark)

    def word_counts(self, words, since_wall=0.0, exclude=()):
        """How often each word occurs in each lab file written since
        SINCE_WALL (every ledger and log whatever its age), keyed by the path
        inside the lab. The fixtures folder is skipped: it holds the texts the
        lab plays. EXCLUDE names further folders or files."""
        skip = [str(self.lab / "voice")] + [str(p) for p in exclude]
        counts, scanned = {}, 0
        for dirpath, _, filenames in os.walk(self.lab):
            if any(dirpath == s or dirpath.startswith(s + os.sep) for s in skip):
                continue
            for name in filenames:
                path = os.path.join(dirpath, name)
                if path in skip:
                    continue
                try:
                    st = os.lstat(path)
                    if not os.path.isfile(path) or os.path.islink(path) or st.st_size == 0 or st.st_size > 64 << 20:
                        continue
                    if st.st_mtime < since_wall and not path.endswith((".jsonl", ".log")):
                        continue
                    scanned += 1
                    low = open(path, "rb").read().lower()
                except OSError:
                    continue
                found = {w: low.count(w.lower().encode()) for w in words}
                found = {w: n for w, n in found.items() if n}
                if found:
                    counts[str(Path(path).relative_to(self.lab))] = found
        return counts, scanned

    def scan_for(self, words, since_wall, exclude=(), baseline=None):
        """Every place a word may hide in the lab: frames seen, every ledger
        and log, and each lab file written since SINCE_WALL. BASELINE, from
        word_counts before the step spoke, holds what files such as the
        provider catalogue cache already carried (a hosted model may be named
        after a common word): only occurrences beyond it count."""
        hits = []
        pats = [re.compile(re.escape(w), re.I) for w in words]
        for l in self.watch.lines:
            s = json.dumps(l["frame"])
            for p in pats:
                if p.search(s):
                    hits.append({"where": "frame", "wall": l["wall"], "word": p.pattern})
        counts, scanned = self.word_counts(words, since_wall, exclude)
        base = baseline or {}
        for where, found in sorted(counts.items()):
            for w, n in found.items():
                had = (base.get(where) or {}).get(w, 0)
                if n > had:
                    hits.append({"where": where, "word": w, "count": n, "before": had})
        return hits, scanned

    def hands_frames_off(self, mark):
        hands = self.watch.frames("hands", mark)
        return [d for _, d in hands if d.get("stage") != "off" or d.get("active")]

    def input_programs(self, since):
        return [r for r in self.vchildren.procs(INPUT_PROGRAMS, since=since)]

    def clipboard(self):
        r = self.sh("voice.sh", "clipboard", check=False)
        try:
            return json.loads(r.stdout)
        except ValueError:
            return {"error": (r.stderr or r.stdout).strip()}

    def clipboard_empty(self, c):
        return all((c.get(k) or {}).get("empty") for k in ("clipboard", "primary"))

    def text_of(self, path):
        try:
            return Path(path).read_text()
        except OSError:
            return None

    def wait_text(self, path, n, timeout=10):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            t = self.text_of(path) or ""
            if len(t) >= n:
                return t
            time.sleep(0.1)
        return self.text_of(path) or ""

    def dictate(self, action, mode=None, timeout=120):
        args = ["oni", "dictate", action, "--json"] + (["--mode", mode] if mode else [])
        r = self.cli(*args, timeout=timeout)
        try:
            value = json.loads(r.stdout) if r.stdout.strip() else {}
        except ValueError:
            value = {"raw": r.stdout.strip()}
        return r.returncode, value, r.stderr.strip()

    def talk_cli(self, action, verb="talk"):
        """hyakki oni talk|speak ACTION --json: exit code, result, stderr."""
        r = self.cli("oni", verb, action, "--json")
        try:
            value = json.loads(r.stdout) if r.stdout.strip() else {}
        except ValueError:
            value = {"raw": r.stdout.strip()}
        return r.returncode, value, r.stderr.strip()

    # --- step 0: graph, key source, meter ---

    def census11(self):
        checks = []
        self.sh("audio.sh", "up", timeout=120)
        self.sh("audio.sh", "mic", timeout=60)
        graph = json.loads((self.lab / "log/audio-graph.json").read_text())
        mic = json.loads((self.lab / "log/audio-mic-check.json").read_text())
        (self.out / "audio" / "graph.json").write_text(json.dumps(graph, indent=2))
        st = subprocess.run([sys.executable, str(HERE / "voicekit.py"), "mic-check", "--selftest"],
                            capture_output=True, text=True, timeout=60)
        checks += [
            {"name": "private graph: no device sink or source, the lab's sinks and sources only", "value": mic,
             "pass": not mic["problems"]},
            {"name": "the default source is oni_lab_mic_in, oni_lab_mic's monitor remapped",
             "value": mic["default_source"], "pass": mic["default_source"] == "oni_lab_mic_in"},
            {"name": "the guard refuses a doctored graph (hardware, Bluetooth, foreign default or module)",
             "value": json.loads(st.stdout) if st.stdout.strip().startswith("{") else st.stdout, "pass": st.returncode == 0},
        ]
        fifo = self.sh("voice.sh", "keys").stdout.strip().splitlines()[-1]
        self.clock = ClockWatch(self.ledger.path)
        self.clock.start()
        self.watch.start()
        time.sleep(1.0)
        # the surface starts its chord reader only once a motion frame carries
        # the chord, which the daemon sends only while oni.voice is on
        self.settings(voice=True)
        self.vset(stt=self.stt, tts={"engine": "system"}, speakReplies=True, openMic=False,
                  wake={"enabled": False, "phrase": WAKE_PHRASE},
                  dictation={"enabled": False, "mode": "clean", "polish": "local"})
        stored = self.oni_settings()
        checks.append({"name": "voice on with the lab's speech model", "value": {"voice": stored.get("voice"),
                                                                                 "voiceSettings": stored.get("voiceSettings")},
                       "pass": stored.get("voice") is True and all(
                           (((stored.get("voiceSettings") or {}).get("stt") or {}).get(k) or "") == v for k, v in self.stt.items())})
        # a voice ask runs on the typed Ask's route (decision 4); the lab pins
        # that route to its loopback model so no ask leaves the lab
        ask = self.call("settings.set", {"patch": {"ask": {"model": LAB_ASK_MODEL}}}).get("ask") or {}
        checks.append({"name": "the typed Ask route, which voice asks take, is the lab's loopback model",
                       "value": ask.get("model"), "pass": ask.get("model") == LAB_ASK_MODEL})
        keys_line = self.ledger.wait(lambda l: l.get("ev") == "keys", (None, 0), 15)
        checks.append({"name": "oni-surface reads the chord from the lab FIFO (ledger keys line, reason fifo)",
                       "value": keys_line, "pass": bool(keys_line) and keys_line.get("reason") == "fifo"})
        self.keys = KeyWriter(fifo)
        before = set(self.lab_files())
        quiet = self.cli("oni", "voice-test", "--json")
        th, io = self.inject_async("ask")
        time.sleep(0.4)
        loud = self.cli("oni", "voice-test", "--ms", "2500", "--json")
        th.join(timeout=60)
        q = json.loads(quiet.stdout) if quiet.returncode == 0 and quiet.stdout.strip() else {}
        lo = json.loads(loud.stdout) if loud.returncode == 0 and loud.stdout.strip() else {}
        names = [s.get("name") for s in lo.get("sources") or []]
        graph_devices = {n.get("name") for n in graph["sources"] if (n.get("properties") or {}).get("device.api")}
        written = sorted(set(self.lab_files()) - before)
        checks += [
            {"name": "meter on silence stays low", "value": q.get("levels"),
             "pass": quiet.returncode == 0 and bool(q.get("levels")) and max(q["levels"]) < 0.05},
            {"name": "meter while speech plays into the microphone rises", "value": lo.get("levels"),
             "pass": loud.returncode == 0 and bool(lo.get("levels")) and max(lo["levels"]) > 0.05},
            {"name": "the chooser offers oni_lab_mic_in and no monitor or device", "value": names,
             "pass": "oni_lab_mic_in" in names and not [n for n in names if n and (n.endswith(".monitor") or n in graph_devices)]},
            {"name": "the meter wrote no file in the lab", "value": written, "pass": not written},
        ]
        return {"fifo": fifo, "graph_sources": [n.get("name") for n in graph["sources"]], "meter": {"quiet": q, "loud": lo},
                "checks": checks}, self.checks_pass(checks)

    def lab_files(self):
        out = []
        for dirpath, _, filenames in os.walk(self.lab):
            if dirpath.startswith(str(self.lab / "voice")):
                continue
            for n in filenames:
                p = os.path.join(dirpath, n)
                if p.endswith((".jsonl", ".log", ".json", "lab.env")) or "/log/" in p:
                    continue
                out.append(p)
        return out

    # --- step 1: push to talk ---

    def ptt(self):
        checks = []
        lmark = self.ledger.mark()
        # AltGr (left Ctrl + right Alt) never counts; a letter is no chord
        m = self.watch.mark()
        self.keys.send("altgr-down")
        time.sleep(1.0)
        self.keys.send("altgr-up")
        self.keys.send("letter")
        time.sleep(1.0)
        altgr = [d for _, d in self.voice_since(m) if d.get("phase") != "idle"]
        chord_lines = [l for l in self.ledger_lines(lmark) if l.get("ev") == "chord"]
        checks.append({"name": "AltGr and a letter open nothing (no voice frame, no chord line)",
                       "value": {"frames": altgr, "chord_lines": chord_lines}, "pass": not altgr and not chord_lines})
        # a tap under tapDropMs is dropped
        self.model.reset()
        m = self.watch.mark()
        tap_ms = self.vspec["press"]["tapDropMs"] / 2
        self.keys.send("down")
        time.sleep(tap_ms / 1000)
        self.keys.send("up")
        time.sleep(2.0)
        tap = self.voice_since(m)
        checks.append({"name": f"a {tap_ms:.0f} ms tap is dropped: no transcript, no ask",
                       "value": tap, "pass": not [d for _, d in tap if d.get("text")] and not self.model_requests()})
        # the press
        self.model.reset()
        pmark = self.ledger.mark()
        rec = Rec(self, "ptt", mic=True)
        r = self.hold("ask")
        self.final_of(r)
        ack = self.speaking_after(r["mark"], 15, ack=True)
        reply = self.speaking_after(r["mark"], 60, ack=False)
        done = self.wait_idle(reply[2], 60)
        time.sleep(1.5)
        rec.stop()
        frames = self.voice_since(r["mark"])
        reqs = self.model_requests()
        want = self.fx["fixtures"]["ask"]["text"]
        final_text = r["final"][1] if r["final"] else ""
        caps = [p for p in self.vchildren.procs(CAPTURE_PROGRAMS, since=r["down"] - 0.5) if is_capture(p)]
        cap_alive = [p for p in self.vchildren.alive(CAPTURE_PROGRAMS) if is_capture(p)]
        chord_lines = [l for l in self.ledger_lines(pmark) if l.get("ev") == "chord"]
        keyish = [l for l in self.ledger_lines(lmark) if {"code", "key", "keycode", "keys"} & set(l)]
        # the ack and the reply in the recording: each where an utterance
        # starts (after 0.2 s of silence), never a word after one of the
        # utterance's own gaps; the reply follows the ack by a few hundred ms
        ack_rec = rec.to_rec(ack[0])
        ack_seg = None if ack_rec is None else rec.utterance(ack_rec - 0.3, ack_rec + 1.0)
        reply_rec = rec.to_rec(reply[0])
        reply_seg = None if reply_rec is None else rec.utterance(max(reply_rec - 0.3, ack_seg[1] if ack_seg else 0.0),
                                                                  reply_rec + 1.0)
        if reply_seg:
            # to its first 0.2 s of silence, as step 2 measures the cut
            # reply; its PCM is step 2's reference for the cut
            end = rec.speech_end(reply_seg[0])
            if end is not None:
                self.unbarged = {"text": reply[1].get("text"), "seconds": round(end - reply_seg[0], 3),
                                 "onset_rec_s": round(reply_seg[0], 4)}
                i0 = int(round(reply_seg[0] * rec.rate))
                self.unbarged_pcm = (rec.pcm[i0:int(round((end + 0.05) * rec.rate))].copy(), rec.rate)
        levels = self.watch.frames("level", r["mark"])
        live = self.live_intervals(frames)
        stray = [w for w, _ in levels if not any(a - 0.2 <= w <= b + 0.2 for a, b in live)]
        rate = self.max_rate([w for w, _ in levels])
        checks += [
            {"name": "the chord opens the microphone: listening, source chord",
             "value": {"down_to_listening_ms": round((r["listening"] - r["down"]) * 1000, 1), "source": r["listening_source"]},
             "pass": r["listening_source"] == "chord"},
            {"name": "chord down and up in the ledger, no key identity anywhere",
             "value": {"chord": chord_lines, "keyish": keyish[:5]},
             "pass": [l.get("event") for l in chord_lines] == ["down", "up"] and not keyish},
            {"name": "partial transcripts while held", "value": r["partials"], "pass": len(r["partials"]) >= 1},
            {"name": "final transcript equals the fixture (words, case and punctuation aside)",
             "value": {"final": final_text, "want": want, "wer": wer(want, final_text)},
             "pass": norm_words(final_text) == norm_words(want)},
            {"name": "one ask reached the model, carrying the transcript", "value": [asked_text(q) for q in reqs],
             "pass": len(reqs) == 1 and " ".join(norm_words(want)) in " ".join(norm_words(asked_text(reqs[0])))},
            {"name": "an acknowledgement spoken, then the reply's first sentence",
             "value": {"ack": ack[1].get("text"), "reply": reply[1].get("text")},
             "pass": self.is_ack(ack[1].get("text")) and ack[0] < reply[0]
             and norm_words(reply[1].get("text")) == norm_words(first_sentence(self.answer))},
            {"name": "the ack's PCM is in the recording", "value": {"ack_rec_s": ack_rec, "segment": ack_seg,
                                                                    "sync_offset": rec.offset},
             "pass": ack_seg is not None},
            {"name": "the reply's PCM is in the recording", "value": reply_seg, "pass": reply_seg is not None},
            {"name": "capture ran only for the press, on the lab's runtime dir",
             "value": {"capture": [{k: p[k] for k in ("comm", "argv", "env", "first", "last")} for p in caps],
                       "alive_after": [p["pid"] for p in cap_alive]},
             "pass": bool(caps) and not cap_alive
             and all(p["env"].get("XDG_RUNTIME_DIR") == str(self.lab / "rt") for p in caps)},
            {"name": f"level frames only while listening or speaking, at most {self.vspec['levels']['maxHz']} Hz",
             "value": {"count": len(levels), "outside": len(stray), "max_per_s": rate},
             "pass": not stray and rate <= self.vspec["levels"]["maxHz"] + 1},
            {"name": "back to idle", "value": done[1], "pass": done[1].get("phase") == "idle"},
        ]
        self.ladder["runs"].append(self.ladder_row(r, ack, rec))
        # the same press from the CLI toggle: hyakki oni talk start|stop
        mt = self.watch.mark()
        rc_s, v_s, e_s = self.talk_cli("start")
        tl = self.wait_voice(mt, lambda d: d.get("phase") == "listening", 5, None)
        time.sleep(0.3)
        self.inject("time")
        time.sleep(0.4)
        rc_e, v_e, e_e = self.talk_cli("stop")
        said = self.wait_voice(mt, lambda d: d.get("phase") == "speaking", 15, None)
        self.wait_voice(mt, lambda d: d.get("phase") == "idle", 30, None)
        checks.append({"name": "hyakki oni talk start|stop: listening (source toggle), the clock answered",
                       "value": {"start": [rc_s, v_s, e_s], "stop": [rc_e, v_e, e_e], "listening": tl[1], "said": said[1]},
                       "pass": rc_s == 0 and rc_e == 0 and (tl[1] or {}).get("source") == "toggle"
                       and bool(re.match(r"It's \d{1,2}:\d{2}\.$", (said[1] or {}).get("text") or ""))})
        return {"press": self.press_record(r), "frames": frames, "model_requests": reqs, "recording": str(rec.wav),
                "checks": checks}, self.checks_pass(checks)

    def press_record(self, r):
        return {k: r.get(k) for k in ("fixture", "down", "listening", "listening_source", "inject", "up",
                                      "partials", "final", "end")}

    def live_intervals(self, frames):
        out, start = [], None
        for w, d in frames:
            live = d.get("phase") in ("listening", "speaking", "transcribing")
            if live and start is None:
                start = w
            elif not live and start is not None:
                out.append((start, w))
                start = None
        if start is not None:
            out.append((start, time.time()))
        return out

    def max_rate(self, walls):
        walls = sorted(walls)
        best, j = 0, 0
        for i, w in enumerate(walls):
            while walls[j] < w - 1.0:
                j += 1
            best = max(best, i - j + 1)
        return best

    def ladder_row(self, r, ack, rec):
        fx = self.fx["fixtures"][r["fixture"]]
        # paplay starts through audio.sh and systemd-run, hundreds of ms
        # after the call (r["inject"][0]), and returns tens of ms after the
        # sound drained (r["inject"][1]): the start bounds the onset from
        # above, the end minus the fixture's length from below
        late_onset = r["inject"][0] + fx["speech_start_s"]
        speech_wall = r["inject"][1] - fx["seconds"] + fx["speech_start_s"]
        first_partial = r["partials"][0][0] if r.get("partials") else None
        # the daemon sends no new transcribing frame when the final text
        # equals the last partial, so the last transcribing frame can predate
        # the release; the final is known by the frame that ends transcription
        # (the ack's speaking frame, or idle/armed), sent with it when it changes
        final_w = None
        if r.get("final"):
            final_w = max(r["final"][0], r["end"][0]) if r.get("end") else r["final"][0]
        row = {"hotkey_to_listening_ms": round((r["listening"] - r["down"]) * 1000, 1),
               "speech_to_first_partial_ms": None if first_partial is None else round((first_partial - speech_wall) * 1000, 1),
               "speech_to_first_partial_from_call_ms": None if first_partial is None
               else round((first_partial - late_onset) * 1000, 1),
               "paplay_start_after_call_ms": round((speech_wall - late_onset) * 1000, 1),
               "release_to_final_ms": None if final_w is None else round((final_w - r["up"]) * 1000, 1),
               "release_to_ack_frame_ms": None if ack is None else round((ack[0] - r["up"]) * 1000, 1),
               "note": "speech onset is when paplay returned less the fixture's length plus its lead; paplay "
                       "returns once the sound drained, so speech-relative numbers are short by that drain. "
                       "_from_call_ms counts from the inject call instead, an upper bound that includes "
                       "audio.sh's and systemd-run's start"}
        if rec is not None and ack is not None and rec.offset is not None:
            segs = rec.segments(after=rec.to_rec(r["up"]))
            a = rec.to_rec(ack[0])
            s = next((s for s in segs if a - 0.3 <= s[0] <= a + 1.0), None)
            row["ack_frame_to_pcm_ms"] = None if s is None else round((s[0] - a) * 1000, 1)
        return row

    # --- step 2: barge-in ---

    def barge(self):
        import numpy as np
        checks = []
        self.model.reset()
        rec = Rec(self, "barge")
        r = self.hold("ask")
        self.final_of(r)
        w_reply, d_reply, i_reply = self.speaking_after(r["mark"], 60, ack=False)
        time.sleep(0.35)
        t_barge = self.keys.send("down")
        w_listen, _, _ = self.wait_voice(i_reply, lambda d: d.get("phase") == "listening", 3, "listening after the barge")
        time.sleep(0.1)
        self.keys.send("up")
        time.sleep(2.5)
        rec.stop()
        frames = self.voice_since(r["mark"])
        after = [d for w, d in frames if w > t_barge]
        resumed = [d for d in after if d.get("phase") == "speaking" and d.get("text") == d_reply.get("text")]
        cr = rec.to_rec(t_barge)
        ramp = self.gating["muteRampMs"]
        ref = self.unbarged_pcm if self.unbarged and self.unbarged.get("text") == d_reply.get("text") else None
        onset = None if rec.offset is None else rec.utterance(rec.to_rec(w_reply) - 0.3, rec.to_rec(w_reply) + 1.0)
        cut = None
        if cr is not None and onset is not None and ref is not None and ref[1] == rec.rate:
            cut = cut_against_reference(rec.pcm, rec.rate, onset[0], ref[0], cr, ramp)
            if cut is not None:
                cut["chord_rec_s"] = round(cr, 4)
        # the measure itself, on the reference: a fade within the ramp
        # passes, a slower fade and a dead stop on speech fail
        guard = {}
        if ref is not None:
            x = np.concatenate([ref[0], np.zeros(ref[1], dtype=np.float32)])
            loud = np.flatnonzero(np.abs(x[int(0.25 * ref[1]):]) > 0.05)
            i = int(0.25 * ref[1]) + (int(loud[0]) if len(loud) else 0)
            for name, fade_s in (("ramp", ramp / 2000), ("slow", ramp * 3 / 1000), ("click", 0.0)):
                g = np.linspace(1.0, 0.0, max(1, int(fade_s * ref[1])), endpoint=False) if fade_s else np.zeros(1)
                y = x.copy()
                y[i:i + len(g)] *= g
                y[i + len(g):] = 0
                o = cut_against_reference(y, ref[1], 0.0, ref[0], 0.0, ramp) or {}
                guard[name] = {"fade": o.get("fade"), "fade_ms": o.get("fade_ms"), "pass": bool(o.get("pass"))}
        checks += [
            {"name": "the chord during speech opens the microphone at once",
             "value": round((w_listen - t_barge) * 1000, 1), "pass": w_listen - t_barge <= 0.3},
            {"name": "the cut measure (barged reply against step 1's uncut reply, sample by sample) passes a "
                     "fade within the ramp and fails a slower fade and a dead stop",
             "value": guard, "pass": bool(guard) and guard["ramp"]["pass"] and not guard["slow"]["pass"]
             and not guard["click"]["pass"]},
            {"name": f"the recorded speech fades out within the mute ramp ({ramp} ms), or the cut falls in a "
                     "silence of the sentence and fades nothing audible",
             "value": {"cut": cut, "reference": self.unbarged, "barged_onset": onset},
             "pass": bool(cut) and bool(cut["pass"])},
            {"name": "the cut follows the chord (recording clock mapped through the sync tone)",
             "value": None if not cut else {"chord_to_cut_ms": cut["chord_to_cut_ms"],
                                            "chord_to_silence_ms": cut["chord_to_silence_ms"], "sync_offset": rec.offset},
             "pass": bool(cut) and -300 <= cut["chord_to_cut_ms"] <= 500},
            {"name": "the reply was cut short of its unbarged length (step 1)",
             "value": {"unbarged": self.unbarged, "barged_s": None if not cut else cut.get("spoken_s")},
             "pass": bool(cut and cut.get("spoken_s") and self.unbarged) and cut["spoken_s"] < self.unbarged["seconds"] - 0.2},
            {"name": "the cut reply never resumes, and no second ask", "value": {"resumed": resumed, "asks": len(self.model_requests())},
             "pass": not resumed and len(self.model_requests()) == 1},
        ]
        # hyakki oni speak stop cuts the reply as well
        self.model.reset()
        r2 = self.hold("ask")
        w2, d2, i2 = self.speaking_after(r2["mark"], 60, ack=False)
        time.sleep(0.3)
        t_stop = time.time()
        rc_q, v_q, e_q = self.talk_cli("stop", verb="speak")
        quiet = self.wait_voice(i2, lambda d: d.get("phase") != "speaking", 3, None)
        time.sleep(1.5)
        again = [d for w, d in self.voice_since(i2) if d.get("phase") == "speaking" and d.get("text") == d2.get("text")]
        checks.append({"name": "hyakki oni speak stop: speaking ends at once and does not resume",
                       "value": {"rc": rc_q, "result": v_q, "error": e_q,
                                 "stop_to_frame_ms": None if quiet[0] is None else round((quiet[0] - t_stop) * 1000, 1),
                                 "resumed": again},
                       "pass": rc_q == 0 and quiet[0] is not None and quiet[0] - t_stop <= 0.5 and not again})
        return {"frames": frames, "recording": str(rec.wav), "checks": checks}, self.checks_pass(checks)

    # --- step 3: echo ---

    def echo(self):
        checks = []
        self.sh("audio.sh", "echo", "on")
        self.model.reset()
        rec = Rec(self, "echo", mic=True)
        m = self.watch.mark()
        # oni.voiceSettings.openMic on starts open mic at once
        self.vset(openMic=True)
        opened = self.wait_voice(m, lambda d: d.get("source") == "open" and d.get("phase") in ("listening", "armed"), 5, None)
        time.sleep(0.4)
        t0, t1 = self.inject("ask")
        w_reply, d_reply, i_reply = self.speaking_after(m, 60, ack=False)
        w_end, _, _ = self.wait_voice(i_reply, lambda d: d.get("phase") != "speaking", 60, "the reply's end")
        echo_ms = self.vspec["turn"]["echoBaseMs"] + self.vspec["turn"]["echoPerCharMs"] * len(d_reply.get("text") or "")
        time.sleep(echo_ms / 1000 + 3.0)
        self.vset(openMic=False)
        time.sleep(1.0)
        rec.stop()
        self.sh("audio.sh", "echo", "off")
        closed = self.wait_voice(i_reply, lambda d: d.get("phase") == "idle", 10, None)
        frames = self.voice_since(m)
        reqs = self.model_requests()
        spoken = [d.get("text") for _, d in frames if d.get("phase") == "speaking"]
        heard_after = [d.get("text") for w, d in frames if w > w_reply and d.get("phase") == "transcribing"]
        mic_segs = sound_segments(rec.mic_pcm, rec.rate, floor=1e-3, gap_s=0.05) if rec.mic_pcm is not None else []
        mic_total = round(sum(b - a for a, b in mic_segs), 3)
        speech_s = self.fx["fixtures"]["ask"]["speech_s"]
        checks += [
            {"name": "open mic on from the setting (source open)", "value": opened[1],
             "pass": opened[1] is not None},
            {"name": "open mic off from the setting: idle", "value": closed[1], "pass": closed[1] is not None},
            {"name": "Oni's speech reached the microphone (loopback on): more sound on the mic than the injected ask",
             "value": {"mic_sound_s": mic_total, "injected_speech_s": speech_s}, "pass": mic_total > speech_s + 0.5},
            {"name": "heard back, not a command: one ask only", "value": [asked_text(q) for q in reqs], "pass": len(reqs) == 1},
            {"name": "nothing spoken but the ack and the reply", "value": spoken,
             "pass": all(self.is_ack(t) or t == d_reply.get("text") for t in spoken) and spoken.count(d_reply.get("text")) == 1},
        ]
        return {"echo_window_ms": echo_ms, "heard_after_reply": heard_after, "frames": frames,
                "recording": str(rec.wav), "mic_recording": str(rec.mic_wav), "checks": checks}, self.checks_pass(checks)

    # --- step 4: wake ---

    def wake_phrase_run(self, phrase, fixture):
        m = self.watch.mark()
        self.vset(wake={"enabled": True, "phrase": phrase})
        armed = self.wait_voice(m, lambda d: d.get("phase") == "armed", 8, None)
        time.sleep(1.0)
        m2 = self.watch.mark()
        self.inject(fixture)
        opened = self.wait_voice(m2, lambda d: d.get("phase") == "listening", 10, None)
        return armed, opened

    def wake11(self):
        checks = []
        start_wall = time.time()
        baseline, _ = self.word_counts(NONWAKE_MARKS)
        wmark = self.ledger.mark()
        self.model.reset()
        rec = Rec(self, "wake")
        m = self.watch.mark()
        armed, opened = self.wake_phrase_run(WAKE_PHRASE, "wake")
        caps_armed = [p for p in self.vchildren.alive(CAPTURE_PROGRAMS) if is_capture(p)]
        # the question inside the wake session: a local clock answer
        clock = None
        if opened[0]:
            self.inject("time")
            clock = self.speaking_after(opened[2], 15, None)
        back = self.wait_voice(opened[2] if opened[0] else m, lambda d: d.get("phase") == "armed", 30, None)
        # non-wake speech while armed: nothing at all
        mn = self.watch.mark()
        self.inject("nonwake")
        time.sleep(3.0)
        nonwake = [d for _, d in self.voice_since(mn) if d.get("phase") != "armed" or d.get("text")]
        reqs_nonwake = self.model_requests()
        # the session ceiling: the phrase alone, then silence
        mc = self.watch.mark()
        self.inject("wake")
        w0 = self.wait_voice(mc, lambda d: d.get("phase") == "listening", 10, None)
        w1 = self.wait_voice(w0[2] if w0[0] else mc, lambda d: d.get("phase") in ("armed", "idle"),
                             self.vspec["wake"]["sessionS"] + 15, None)
        ceiling = None if not (w0[0] and w1[0]) else round(w1[0] - w0[0], 2)
        # a changed phrase
        armed2, opened2 = self.wake_phrase_run(WAKE_PHRASE2, "wake2")
        self.wait_voice(opened2[2] if opened2[0] else self.watch.mark(), lambda d: d.get("phase") == "armed",
                        self.vspec["wake"]["sessionS"] + 15, None)
        mo = self.watch.mark()
        self.inject("wake")  # the old phrase no longer opens
        time.sleep(4.0)
        old_opens = [d for _, d in self.voice_since(mo) if d.get("phase") == "listening"]
        # disarm: no capture afterwards
        md = self.watch.mark()
        self.vset(wake={"enabled": False, "phrase": WAKE_PHRASE})
        idle = self.wait_voice(md, lambda d: d.get("phase") == "idle", 8, None)
        time.sleep(2.0)
        caps_after = [p for p in self.vchildren.alive(CAPTURE_PROGRAMS) if is_capture(p)]
        rec.stop()
        hits, scanned = self.scan_for(NONWAKE_MARKS, start_wall, baseline=baseline)
        hands = self.hands_frames_off(m)
        inputs = self.input_programs(start_wall)
        marks = [l.get("reason") for l in self.ledger_lines(wmark) if l.get("ev") == "micmark"]
        marked = [l for l in self.ledger_lines(wmark) if l.get("ev") == "commit" and l.get("mark")]
        clock_text = (clock[1] or {}).get("text") if clock and clock[1] else None
        clock_ok = False
        if clock_text:
            mm = re.match(r"It's (\d{1,2}):(\d{2})\.$", clock_text)
            if mm:
                t = time.localtime(clock[0])
                said = int(mm.group(1)) % 12 * 60 + int(mm.group(2))
                real = t.tm_hour % 12 * 60 + t.tm_min
                clock_ok = min((said - real) % 720, (real - said) % 720) <= 1
        session = self.vspec["wake"]["sessionS"]
        checks += [
            {"name": "wake on: the armed phase, the microphone live", "value": {"armed": armed[1], "capture": len(caps_armed)},
             "pass": armed[1] is not None and armed[1].get("source") == "wake" and bool(caps_armed)},
            {"name": f"'{WAKE_PHRASE}' opens a session (source wake)", "value": opened[1],
             "pass": opened[1] is not None and opened[1].get("source") == "wake"},
            {"name": "the question in the session is answered from the clock, no model",
             "value": {"said": clock_text}, "pass": clock_ok},
            {"name": "the session returns to armed", "value": back[1], "pass": back[1] is not None},
            {"name": "non-wake speech is discarded: no frame, no ask", "value": {"frames": nonwake, "asks": len(reqs_nonwake)},
             "pass": not nonwake and not reqs_nonwake},
            {"name": "non-wake words appear in no frame, ledger, log or lab file",
             "value": {"hits": hits, "files_scanned": scanned}, "pass": not hits},
            {"name": f"a wake session with nothing said ends at the {session} s ceiling",
             "value": ceiling, "pass": ceiling is not None and session - 1 <= ceiling <= session + 2},
            {"name": f"a changed phrase ('{WAKE_PHRASE2}') opens; the old one no longer does",
             "value": {"opened": opened2[1], "old_opens": old_opens},
             "pass": opened2[1] is not None and not old_opens},
            {"name": "Hands untouched: every hands frame off, no input program", "value": {"hands": hands, "inputs": inputs},
             "pass": not hands and not inputs},
            {"name": "disarmed: idle, no capture process", "value": {"idle": idle[1], "capture": [p["pid"] for p in caps_after]},
             "pass": idle[1] is not None and not caps_after},
            {"name": "the mic mark shows while armed and goes with the disarm (ledger micmark on, then off last)",
             "value": {"micmark": marks, "commits_with_mark": len(marked)},
             "pass": "on" in marks and bool(marks) and marks[-1] == "off" and bool(marked)},
        ]
        self.limit(4, f"wake armed ceiling ({self.vspec['wake']['armedMaxS']} s)",
                   "an hour-long arming is proven by voice/wake's injected-clock tests, not waited out in the lab")
        self.limit(4, "the default wake phrase ('hey oni')",
                   "espeak-ng's rendering of it reaches Parakeet 0.6B v2 as 'Say only.', so the lab arms "
                   f"'{WAKE_PHRASE}' and '{WAKE_PHRASE2}'; the default phrase in a transcript is voice/wake's "
                   "TestWakeAcceptsThePhraseAtTheStart and TestWakeRejectsNearMisses")
        return {"frames": self.voice_since(m), "recording": str(rec.wav), "checks": checks}, self.checks_pass(checks)

    # --- step 5: open mic ---

    def openmic(self):
        checks = []
        if self.fixture("notes") is None:
            self.fixture_open("notes")
        self.reperch_notes()
        before = self.oni_settings()
        self.settings(inspect=True, disclosure="always")
        self.model.reset()
        self.park_pointer()
        m = self.watch.mark()
        ask = subprocess.Popen([str(HERE / "ctl.sh"), "hyakki", "oni", "ask", "--json", labkit.QUESTION_POINT],
                               stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        card = self.watch.wait("consent", lambda d: True, m, 60, None)
        mo = self.watch.mark()
        t_open = time.time()
        # oni.voiceSettings.openMic on starts open mic at once; the daemon
        # writes it back to false when the ceiling ends it
        self.vset(openMic=True)
        opened = self.wait_voice(mo, lambda d: d.get("source") == "open" and d.get("phase") in ("listening", "armed"), 5, None)
        time.sleep(0.4)
        self.inject("yes")
        press = self.wait_voice(mo, lambda d: d.get("phase") == "speaking", 15, None)
        time.sleep(1.0)
        pending = ask.poll() is None
        answers = self.watch.frames("consent.answer", m)
        reqs = self.model_requests()
        out, errs = ask.communicate("n\n", timeout=120)
        hands = self.hands_frames_off(m)
        # the ceiling: open mic ends by itself
        limit = self.vspec["press"]["openMicS"]
        end = self.wait_voice(opened[2] if opened[0] else mo, lambda d: d.get("phase") == "idle", limit + 20, None)
        dur = None if end[0] is None else round(end[0] - t_open, 2)
        time.sleep(2.0)
        caps_after = [p for p in self.vchildren.alive(CAPTURE_PROGRAMS) if is_capture(p)]
        written_back = (self.oni_settings().get("voiceSettings") or {}).get("openMic")
        self.vset(openMic=False)
        self.settings(inspect=before.get("inspect", True), disclosure=before.get("disclosure", "session"))
        self.ensure_detached()
        checks += [
            {"name": "a firm consent card is waiting", "value": card[1], "pass": card[1] is not None},
            {"name": "open mic on from the setting (source open)", "value": opened[1], "pass": opened[1] is not None},
            {"name": "'yes' on open mic asks for the key instead", "value": press[1],
             "pass": press[1] is not None and press[1].get("text") == "Press the key to answer."},
            {"name": "the card stays unanswered and nothing left the computer",
             "value": {"cli_waiting": pending, "answers": answers, "model_requests": len(reqs)},
             "pass": pending and not answers and not reqs},
            {"name": "Hands untouched", "value": hands, "pass": not hands},
            {"name": f"open mic ends by itself at {limit} s and the setting reads off again",
             "value": {"seconds": dur, "openMic_after": written_back},
             "pass": dur is not None and limit - 1 <= dur <= limit + 3 and written_back is False},
            {"name": "no capture process after open mic ends", "value": [p["pid"] for p in caps_after], "pass": not caps_after},
        ]
        return {"cli": {"stdout": out[-2000:], "stderr": errs[-2000:]}, "frames": self.voice_since(m),
                "checks": checks}, self.checks_pass(checks)

    # --- step 6: hygiene ---

    def hygiene(self):
        checks = []
        self.model.reset()
        r = self.hold("loop", lead=0.2, tail=0.4)
        self.final_of(r, timeout=30)
        done = self.wait_voice(r["mark"], lambda d: d.get("phase") == "idle", 90, None)
        final_text = r["final"][1] if r["final"] else ""
        secs = self.fx["fixtures"]["loop"]["speech_s"]
        words = norm_words(final_text)
        # whatever phrase the engine heard the loop as, it stays at most once
        # per segment in a row (hygiene's collapse keeps a phrase said three
        # or more times once)
        repeats = max_repeat(words)
        segments = math.ceil(secs / self.vspec["vad"]["maxSegmentS"])
        h = self.vspec["hygiene"]
        plausible = len(words) <= math.ceil(secs * h["wordsPerS"]) + h["slackWords"]
        reqs = self.model_requests()
        checks += [
            {"name": f"43 repeats collapse: something heard, and no phrase of 1-4 words more than once in a row "
                     f"per {self.vspec['vad']['maxSegmentS']} s segment",
             "value": {"final": final_text, "max_repeat": repeats, "segments": segments},
             "pass": bool(words) and repeats <= segments},
            {"name": "the transcript is plausible for its length", "value": {"words": len(words), "seconds": secs},
             "pass": plausible},
            {"name": "what reached the model is the collapsed text", "value": [asked_text(q) for q in reqs],
             "pass": all(max_repeat(norm_words(asked_text(q))) <= segments for q in reqs)},
        ]
        # a silent hold past maxHoldS is cut while the chord stays down
        max_hold = self.vspec["press"]["maxHoldS"]
        m = self.watch.mark()
        t_down = self.keys.send("down")
        w_l, _, i = self.wait_voice(m, lambda d: d.get("phase") == "listening", 3, "listening")
        cut = self.wait_voice(i, lambda d: d.get("phase") != "listening", max_hold + 10, None)
        held_to = time.time()
        self.keys.send("up")
        self.wait_voice(m, lambda d: d.get("phase") == "idle", 60, None)
        dur = None if cut[0] is None else round(cut[0] - t_down, 2)
        checks.append({"name": f"a hold past {max_hold} s is cut while the chord is still down",
                       "value": {"listening_s": dur, "chord_released_s": round(held_to - t_down, 2)},
                       "pass": dur is not None and dur <= max_hold + 1.5})
        return {"press": self.press_record(r), "done": done[1], "checks": checks}, self.checks_pass(checks)

    # --- step 7: dictation ---

    def dictate_into(self, name, mode, fixture="dictate", close_at=None):
        out = self.out / "dictation" / f"a-{name}.txt"
        self.sh("voice.sh", "text", "open", "a", out)
        self.sh("voice.sh", "text", "focus", "a")
        m = self.watch.mark()
        t_start = time.time()
        rc0, v0, e0 = self.dictate("start", mode)
        opened = self.wait_voice(m, lambda d: d.get("phase") == "listening", 5, None)
        time.sleep(0.3)
        closed_at = None
        if close_at is None:
            self.inject(fixture)
            time.sleep(0.5)
        else:
            th, io = self.inject_async(fixture)
            time.sleep(close_at)
            self.sh("voice.sh", "text", "close", "a")
            closed_at = time.time()
            th.join(timeout=60)
            time.sleep(0.5)
        rc, val, err = self.dictate("stop")
        t_stop = time.time()
        self.dictations.append((t_start - 0.5, t_stop + 1.0))
        text = self.wait_text(out, int(val.get("landed") or 0)) if closed_at is None else self.text_of(out)
        if closed_at is None:
            self.sh("voice.sh", "text", "close", "a")
        frames = self.voice_since(m)
        return {"mode": mode, "start": {"rc": rc0, "result": v0, "error": e0}, "opened": opened[1],
                "stop": {"rc": rc, "result": val, "error": err}, "text": text, "file": str(out),
                "closed_at": closed_at, "t_stop": t_stop, "frames": frames}

    def dictation(self):
        checks = []
        start_wall = time.time()
        baseline, _ = self.word_counts(DICTATION_MARKS)
        self.vset(dictation={"enabled": True, "mode": "clean", "polish": "local"})
        self.model.reset()
        self.model.mode("point")
        b = self.out / "dictation" / "b.txt"
        self.sh("voice.sh", "text", "open", "b", b)
        want = self.fx["fixtures"]["dictate"]["text"]
        want_plain = " ".join(w for w in norm_words(want) if w not in FILLERS)
        runs = {}
        for mode in ("exact", "clean", "polished"):
            runs[mode] = self.dictate_into(mode, mode)
            res = runs[mode]["stop"]["result"]
            txt = runs[mode]["text"] or ""
            used = res.get("mode")
            checks += [
                {"name": f"{mode}: started (source dictation), stopped, nothing refused",
                 "value": {"start": runs[mode]["start"], "opened": runs[mode]["opened"], "stop": runs[mode]["stop"]},
                 "pass": runs[mode]["start"]["rc"] == 0 and runs[mode]["stop"]["rc"] == 0 and not res.get("refused")
                 and (runs[mode]["opened"] or {}).get("source") == "dictation"},
                {"name": f"{mode}: the landed count is what the window holds", "value": {"landed": res.get("landed"), "text": txt},
                 "pass": bool(txt) and len(txt) == res.get("landed")},
                {"name": f"{mode}: the words match the fixture", "value": {"wer": min(wer(want, txt), wer(want_plain, txt))},
                 "pass": min(wer(want, txt), wer(want_plain, txt)) <= 0.35},
            ]
            if mode == "exact":
                checks.append({"name": "exact ran as exact", "value": used, "pass": used == "exact"})
            if mode == "clean":
                checks.append({"name": "clean: no filler words", "value": txt,
                               "pass": used == "clean" and not set(norm_words(txt)) & set(FILLERS)})
            if mode == "polished":
                clean_txt = runs["clean"]["text"] or ""
                fell = bool(res.get("fallback"))
                checks.append({"name": "polished: polished, or an honest fallback to clean that says why",
                               "value": {"mode": used, "fallback": res.get("fallback"), "text": txt, "clean": clean_txt},
                               "pass": (used == "polished" and not fell)
                               or (used == "clean" and fell and norm_words(txt) == norm_words(clean_txt))})
        # the target closed mid-dictation
        polish_asks = len(self.model_requests())
        close = self.dictate_into("closed", "clean", fixture="dictate_long", close_at=1.5)
        closed_asks = len(self.model_requests()) - polish_asks
        res = close["stop"]["result"]
        late_inputs = [p for p in self.input_programs(close["closed_at"] or time.time()) if p["first"] >= (close["closed_at"] or 0)]
        b_text = self.text_of(b)
        clip = self.clipboard()
        self.sh("voice.sh", "text", "close", "b")
        time.sleep(1.0)
        # the lab model's own request log records what the Polished run asked
        # it, by design; the closed run's share of it is checked on its own
        model_log = self.lab / "log" / "model-requests.jsonl"
        hits, scanned = self.scan_for(DICTATION_MARKS, start_wall, exclude=[model_log], baseline=baseline)
        dict_frames = [d for r in list(runs.values()) + [close] for _, d in r["frames"] if d.get("source") == "dictation" and d.get("text")]
        checks += [
            {"name": "target closed mid-dictation: refused, nothing landed, the text kept for Copy",
             "value": {"refused": res.get("refused"), "landed": res.get("landed"), "rest_chars": len(res.get("rest") or "")},
             "pass": bool(res.get("refused")) and not res.get("landed") and bool(res.get("rest"))},
            {"name": "no typing program started after the target closed", "value": late_inputs, "pass": not late_inputs},
            {"name": "the closed run asked no model", "value": {"polished_run_asks": polish_asks, "closed_run_asks": closed_asks},
             "pass": closed_asks == 0},
            {"name": "the other window got nothing", "value": b_text, "pass": b_text == ""},
            {"name": "the clipboard and primary selection stay empty", "value": clip, "pass": self.clipboard_empty(clip)},
            {"name": "no dictated text on any voice frame", "value": dict_frames, "pass": not dict_frames},
            {"name": "dictated words in no frame, ledger, log or lab file", "value": {"hits": hits, "files_scanned": scanned},
             "pass": not hits},
        ]
        return {"runs": runs, "closed": close, "checks": checks}, self.checks_pass(checks)

    # --- step 8: polish failure ---

    def lab_provider_rest(self):
        """When the lab model's provider rests until, from providers.list:
        the gateway rests a provider that answered 500 or timed out (two
        minutes) and sends it nothing until then."""
        pid = LAB_ASK_MODEL.split("/", 1)[0]
        for p in self.call("providers.list").get("providers") or []:
            if p.get("id") == pid:
                return p.get("cooldownUntil")
        return None

    def wait_provider_rested(self, timeout=180):
        """Wait out the lab provider's rest, so the next polish or ask
        reaches the lab model; returns the rest seen and how long it took."""
        t0 = time.time()
        seen = self.lab_provider_rest()
        deadline = time.monotonic() + timeout
        while self.lab_provider_rest() and time.monotonic() < deadline:
            time.sleep(2.0)
        return {"rest_until": seen, "waited_s": round(time.time() - t0, 1), "rested": not self.lab_provider_rest()}

    def polish_fallback(self):
        checks = []
        rows = {}
        clean_ref = None
        # dictate's Fallback codes: the model's 500 is failed, no answer
        # within the polish timeout is timeout
        why = {"fail": "failed", "slow": "timeout"}
        for mode, delay in (("fail", 0), ("slow", 120)):
            # the fail run rests the provider; a polish sent while it rests
            # never reaches the model, so the slow run would prove nothing
            rested = self.wait_provider_rested()
            self.model.reset()
            self.model.mode(mode, delay)
            rows[mode] = self.dictate_into(f"polish-{mode}", "polished")
            rows[mode]["provider_rest_before"] = rested
            reqs = self.model_requests()
            res = rows[mode]["stop"]["result"]
            txt = rows[mode]["text"] or ""
            clean_ref = clean_ref or txt
            rows[mode]["model_requests"] = [{k: q.get(k) for k in ("n", "kind", "mode", "stream")} for q in reqs]
            checks += [
                {"name": f"model {mode}: polished falls back to clean and says why ({why[mode]})",
                 "value": {"mode": res.get("mode"), "fallback": res.get("fallback")},
                 "pass": res.get("mode") == "clean" and res.get("fallback") == why[mode]},
                {"name": f"model {mode}: the polish was attempted on the lab model", "value": rows[mode]["model_requests"],
                 "pass": len(reqs) >= 1},
                {"name": f"model {mode}: the clean text landed", "value": {"landed": res.get("landed"), "text": txt},
                 "pass": bool(txt) and len(txt) == res.get("landed") and not set(norm_words(txt)) & set(FILLERS)},
            ]
        self.model.mode("point")
        # later steps ask the lab model; let the provider's rest run out
        rows["after"] = {"provider_rest": self.wait_provider_rested()}
        self.results["model"]["polish_note"] = ("step 8 polishes on the lab model in fail and slow modes; a polish that "
                                                "succeeds is not claimed: the lab model answers its fixed reply")
        return {"runs": rows, "checks": checks}, self.checks_pass(checks)

    # --- step 9: the clip voice ---

    def decode_clips(self, rec, after):
        import numpy as np
        inv = sorted((hz, cid) for cid, hz in self.codes.items())
        out = []
        for a, b in rec.segments(floor=2e-3, gap_s=0.012, after=after):
            if not 0.07 <= b - a <= 0.2:
                continue
            i, j = int((a + 0.01) * rec.rate), int((b - 0.01) * rec.rate)
            hz = peak_hz(np.asarray(rec.pcm[i:j], dtype=np.float64), rec.rate)
            near = min(inv, key=lambda x: abs(x[0] - hz))
            out.append({"t": round(a, 3), "hz": round(hz, 1), "id": near[1] if abs(near[0] - hz) <= 10 else None})
        return out

    def clips11(self):
        checks = []
        before = self.oni_settings()
        # oni.character.select is the app's alone; the stored setting is the CLI's way
        r = self.settings(character=self.clip_char)
        selected = self.oni_settings().get("character")
        self.vset(tts={"engine": "clips"})
        time.sleep(2.0)
        rec = Rec(self, "clips")
        p1 = self.hold("time")
        clock = self.speaking_after(p1["mark"], 15, None)
        self.wait_idle(clock[2], 30)
        p2 = self.hold("timer")
        setl = self.speaking_after(p2["mark"], 15, None)
        up = self.wait_voice(setl[2], lambda d: d.get("phase") == "speaking", 20, None)
        if up[0]:
            self.wait_idle(up[2], 20)
        time.sleep(1.0)
        rec.stop()
        self.vset(tts={"engine": "system"})
        self.settings(character=before.get("character") or "oni")
        decoded = self.decode_clips(rec, rec.to_rec(p1["up"]))
        ids = [d["id"] for d in decoded]
        said = (clock[1] or {}).get("text") or ""
        mm = re.match(r"It's (\d{1,2}):(\d{2})\.$", said)
        want_clock = ["its"] + clock_ids(int(mm.group(1)), int(mm.group(2))) if mm else None
        checks += [
            {"name": "the clip character is selected", "value": {"call": r, "character": selected},
             "pass": selected == self.clip_char},
            {"name": "the clock line said from clips, in order (PCM)", "value": {"line": said, "want": want_clock, "decoded": ids},
             "pass": bool(want_clock) and contains_run(ids, want_clock)},
            {"name": "the timer line said from clips", "value": (setl[1] or {}).get("text"),
             "pass": contains_run(ids, ["timer", "set", "five", "seconds"])},
            {"name": "the timer's end said from clips after five seconds", "value": (up[1] or {}).get("text"),
             "pass": up[1] is not None and up[0] - setl[0] >= 5 and contains_run(ids, ["times", "up"])},
            {"name": "every recorded clip decodes to a known id", "value": [d for d in decoded if d["id"] is None],
             "pass": bool(decoded) and all(d["id"] for d in decoded)},
        ]
        return {"decoded": decoded, "recording": str(rec.wav), "checks": checks}, self.checks_pass(checks)

    # --- step 10: the sound gate ---

    def gate11(self):
        checks = []
        if self.fixture("notes") is None:
            self.fixture_open("notes")
        self.ensure_detached()
        before = self.oni_settings()
        self.settings(sounds=True)
        time.sleep(1.0)
        lmark = self.ledger.mark()
        m = self.watch.mark()
        rec = Rec(self, "gate")
        tried = {}

        def motif():
            notes = self.fixture("notes")
            if summary_perch(self.ledger_lines()):
                self.detach_cli()
            else:
                self.perch_cli(notes["id"])
            return time.time()

        self.vset(wake={"enabled": True, "phrase": WAKE_PHRASE})
        if self.wait_voice(m, lambda d: d.get("phase") == "armed", 8, None)[0]:
            tried["armed"] = motif()
            time.sleep(2.0)
        self.vset(wake={"enabled": False, "phrase": WAKE_PHRASE})
        self.wait_voice(m, lambda d: d.get("phase") == "idle", 8, None)
        ml = self.watch.mark()
        self.keys.send("down")
        if self.wait_voice(ml, lambda d: d.get("phase") == "listening", 3, None)[0]:
            tried["listening"] = motif()
            time.sleep(1.5)
        self.keys.send("up")
        self.wait_voice(ml, lambda d: d.get("phase") == "idle", 20, None)
        r = self.hold("ask")
        tried["transcribing"] = motif()
        sp = self.speaking_after(r["mark"], 60, ack=False)
        if sp[0]:
            tried["speaking"] = motif()
        self.wait_voice(r["mark"], lambda d: d.get("phase") == "idle", 60, None)
        time.sleep(1.0)
        rec.stop()
        self.ensure_detached()
        self.settings(sounds=before.get("sounds", False))
        lines = self.ledger_lines(lmark)
        sounds = [l for l in lines if l.get("ev") == "sound"]
        phase_lines = [l for l in lines if l.get("ev") == "frame" and l.get("kind") == "voice" and l.get("phase")]
        if phase_lines:
            spans, clock = self.spans([(l["t"], l["phase"]) for l in phase_lines]), "ledger"
            at = {id(s): s["t"] for s in sounds}
        else:
            spans = self.spans([(w, d.get("phase")) for w, d in self.voice_since(m)])
            clock = "wall (ledger t mapped through the sampled offset)"
            at = {id(s): self.clock.to_wall(s["t"]) for s in sounds}
        rows = []
        for s in sounds:
            t = at[id(s)]
            ph = next((p for a, b, p in spans if t is not None and a <= t < b), "idle")
            rows.append({"t": s.get("t"), "motif": s.get("motif"), "reason": s.get("reason"), "phase": ph})
        played_live = [r_ for r_ in rows if r_["phase"] != "idle" and r_["reason"] == "played"]
        gated = {r_["phase"] for r_ in rows if r_["phase"] != "idle" and r_["reason"] != "played"}
        checks += [
            {"name": "no motif plays while the microphone is live (armed, listening, transcribing, speaking)",
             "value": played_live, "pass": not played_live},
            {"name": "motifs were gated in armed, listening and speaking", "value": sorted(gated),
             "pass": {"armed", "listening", "speaking"} <= gated},
        ]
        if "transcribing" not in gated:
            self.limit(10, "a motif during transcribing", "the transcribing phase ended before the motif's event "
                       f"reached the surface: {tried.get('transcribing')}")
        return {"clock": clock, "spans": spans, "sounds": rows, "tried": tried, "recording": str(rec.wav),
                "checks": checks}, self.checks_pass(checks)

    def spans(self, pts):
        out = []
        for (t, p), nxt in zip(pts, pts[1:] + [(math.inf, None)]):
            out.append((t, nxt[0], p))
        return out

    # --- step 11: the latency and resource ladder ---

    def ladder11(self):
        checks = []
        for k in range(3):
            self.model.reset()
            rec = Rec(self, f"ladder-{k}")
            r = self.hold("ask")
            self.final_of(r)
            ack = self.wait_voice(r["mark"], lambda d: d.get("phase") == "speaking" and self.is_ack(d.get("text")), 15, None)
            self.wait_voice(r["mark"], lambda d: d.get("phase") == "idle", 60, None)
            time.sleep(1.0)
            rec.stop()
            self.ladder["runs"].append(self.ladder_row(r, ack if ack[0] else None, rec))
        # the first syllable: speech starts the moment the chord goes down
        m = self.watch.mark()
        t_down = self.keys.send("down")
        t0, _ = self.inject("ask")
        time.sleep(0.4)
        t_up = self.keys.send("up")
        r = {"mark": m, "up": t_up}
        self.final_of(r)
        self.wait_voice(m, lambda d: d.get("phase") == "idle", 60, None)
        cap = next((p for p in sorted(self.vchildren.procs(CAPTURE_PROGRAMS, since=t_down - 0.1), key=lambda p: p["first"])
                    if is_capture(p)), None)
        heard = norm_words(r["final"][1] if r.get("final") else "")
        want = norm_words(self.fx["fixtures"]["ask"]["text"])
        self.ladder["first_syllable"] = {
            "capture_started_after_chord_ms": None if cap is None else round((cap["first"] - t_down) * 1000, 1),
            "speech_started_after_chord_ms": round((t0 + self.fx["fixtures"]["ask"]["speech_start_s"] - t_down) * 1000, 1),
            "heard": heard, "first_word_kept": bool(heard) and heard[0] == want[0],
            "note": "no warm microphone: capture starts per press; the capture process is seen by a 50 ms poll"}
        # engines: resident size, and unload when voice goes off
        engines = self.vchildren.procs(ENGINE_PROGRAMS)
        rss = {}
        for e in engines:
            rss[e["comm"]] = max(rss.get(e["comm"]) or 0, e["rss_kb_max"] or 0)
        t_off = time.time()
        self.settings(voice=False)
        gone = None
        deadline = time.monotonic() + 30
        while time.monotonic() < deadline:
            if not self.vchildren.alive(ENGINE_PROGRAMS + CAPTURE_PROGRAMS):
                gone = round(time.time() - t_off, 2)
                break
            time.sleep(0.1)
        self.settings(voice=True)
        self.ladder.update(engine_rss_kb=rss, unload_after_voice_off_s=gone)
        runs = self.ladder["runs"]
        # the segmenter asks for the first partial firstPartialMs into speech,
        # and a partial decode is abandoned after partialMs (spec/voice.go
        # VoiceBudgets bounds one decode): the first partial is due by their sum
        first_due = self.vspec["vad"]["firstPartialMs"] + self.vspec["budgets"]["partialMs"]
        final_budget = self.vspec["budgets"]["finalS"] * 1000
        firsts = [r_["speech_to_first_partial_ms"] for r_ in runs]
        finals = [r_["release_to_final_ms"] for r_ in runs]
        checks += [
            {"name": f"first partial within {first_due:.0f} ms of speech in every press (asked "
                     f"{self.vspec['vad']['firstPartialMs']:.0f} ms in, one decode budget of "
                     f"{self.vspec['budgets']['partialMs']:.0f} ms)", "value": firsts,
             "pass": bool(firsts) and all(v is not None and v <= first_due for v in firsts)},
            {"name": f"final within {final_budget:.0f} ms of the release in every press", "value": finals,
             "pass": bool(finals) and all(v is not None and v <= final_budget for v in finals)},
            {"name": "an engine ran and its size is recorded", "value": rss, "pass": bool(rss)},
            {"name": "voice off unloads every engine and capture within 30 s", "value": gone, "pass": gone is not None},
        ]
        return {"ladder": self.ladder, "checks": checks}, self.checks_pass(checks)

    # --- step 12: settlement ---

    def settle11(self):
        checks = []
        self.vset(wake={"enabled": False, "phrase": WAKE_PHRASE}, openMic=False)
        self.settings(voice=True)
        self.wake()
        time.sleep(5.0)
        caps = [p for p in self.vchildren.alive(CAPTURE_PROGRAMS) if is_capture(p)]
        # The gated windows are measure.sh's 60 s, not 30 s: an idle Go
        # process gets a forced GC 2 minutes after its last GC, which wakes
        # most of this daemon's 16 Ps at once. GODEBUG=gctrace=1 shows it
        # landing in whichever step 12 window step 11's last GC timed it
        # into; a voice-off window that held nothing else counted 62
        # wakeups, 2.07/s over 30 s. A window must also end before the 90 s
        # sleep a poke restarts: a woken character breathes for 30 s first,
        # so wait that out, then poke again; the window starts about 5 s on.
        self.ledger.quiet(5.0, 120)
        self.wake()
        idle = self.measure("settled", timeout=120, seconds=60, name="voice-idle")
        daemon_idle = self.daemon_rates(idle)
        self.settings(voice=False)
        time.sleep(5.0)
        left = self.vchildren.alive(ENGINE_PROGRAMS + CAPTURE_PROGRAMS + PLAYER_PROGRAMS)
        self.wake()
        off = self.measure("settled", timeout=120, seconds=60, name="voice-off")
        daemon_off = self.daemon_rates(off)
        # wake armed is the resident exception; its cost is recorded, not gated
        self.settings(voice=True)
        self.vset(wake={"enabled": True, "phrase": WAKE_PHRASE})
        time.sleep(3.0)
        self.wake()
        armed = self.measure("settled", timeout=120, seconds=30, name="wake-armed", gate=False)
        self.vset(wake={"enabled": False, "phrase": WAKE_PHRASE})
        self.limit(12, "wake armed", "the continuous microphone's cost is recorded, not gated: "
                   f"{self.daemon_rates(armed)}. pw-cat hands the daemon its capture in 4096-byte blocks "
                   "(128 ms of 16 kHz mono, about 7.8 a second); each block runs the reader, the pump and "
                   "the segmenter, about 4 thread wakeups, so about 33/s while the microphone listens")
        th = labkit.THRESHOLDS
        checks += [
            {"name": "voice on and idle: no capture process", "value": [p["pid"] for p in caps], "pass": not caps},
            {"name": "voice on and idle: 0 commits, 0 armed callbacks, surface within budget",
             "value": idle.get("ledger_window"), "pass": bool(idle.get("pass"))},
            {"name": "voice on and idle: the daemon within budget", "value": daemon_idle,
             "pass": all(v["cpu_percent"] is not None and v["cpu_percent"] < th["cpu_percent_max"]
                         and v["wakeups_per_s"] is not None and v["wakeups_per_s"] <= th["wakeups_per_s_max"]
                         for v in daemon_idle)},
            {"name": "voice off: no engine, capture or player left", "value": [(p["comm"], p["pid"]) for p in left],
             "pass": not left},
            {"name": "voice off: 0 commits, 0 armed callbacks", "value": off.get("ledger_window"), "pass": bool(off.get("pass"))},
            {"name": "voice off: the daemon within budget (or gone)", "value": daemon_off,
             "pass": all(v["cpu_percent"] is not None and v["cpu_percent"] < th["cpu_percent_max"]
                         and v["wakeups_per_s"] is not None and v["wakeups_per_s"] <= th["wakeups_per_s_max"]
                         for v in daemon_off)},
        ]
        return {"checks": checks}, self.checks_pass(checks)

    def daemon_rates(self, record):
        return [{"pid": p["pid"], **{k: (p.get("rates") or {}).get(k) for k in ("cpu_percent", "wakeups_per_s")}}
                for p in record.get("processes", []) if p.get("role") == "daemon"]

    # --- step 13: read-only scan ---

    def readonly11(self):
        checks = []
        hits, insert_hits = [], []
        for root in (APP / "internal/oni", APP / "internal/dictate"):
            for path in sorted(root.rglob("*.go")):
                if path.name.endswith("_test.go"):
                    continue
                rel = str(path.relative_to(ROOT))
                inside_insert = (APP / "internal/dictate/insert") in path.parents
                for n, line in enumerate(path.read_text(errors="replace").splitlines(), 1):
                    low = line.lower()
                    for mark in labkit.INPUT_MARKS:
                        if mark in low:
                            (insert_hits if inside_insert else hits).append({"file": rel, "line": n, "mark": mark})
        tests = {}
        for pkg, rel, run in (
                ("feature", "internal/oni/feature", os.environ.get("ONI_LAB_BOUNDARY_RUN", "(?i)(boundary|apart|import|insert)")),
                ("insert", "internal/dictate/insert", "."),
                ("target", "internal/dictate/target", ".")):
            binary = os.environ.get(f"ONI_LAB_{pkg.upper()}_TEST") or str(Path(self.env.get("HYAKKI_BIN", "/")) / f"{pkg}.test")
            if not os.access(binary, os.X_OK):
                tests[pkg] = {"ran": False, "pass": False, "error": f"no {pkg}.test (go test -c of {rel}) in HYAKKI_BIN"}
                continue
            r = subprocess.run(["systemd-run", "--user", "--scope", "--quiet", "--collect",
                                f"--unit=oni-lab-{self.env['LAB_ID']}-{pkg}test-{os.urandom(4).hex()}",
                                "-p", "MemoryMax=1G", "-p", "MemorySwapMax=0", "--",
                                "env", "-i", "PATH=/usr/bin:/bin", binary, "-test.run", run, "-test.v", "-test.count=1"],
                               capture_output=True, text=True, timeout=300, cwd=APP / rel)
            outp = (r.stdout + r.stderr).strip()
            tests[pkg] = {"ran": True, "rc": r.returncode, "output": outp[-1500:],
                          "pass": r.returncode == 0 and "--- PASS" in outp and "--- FAIL" not in outp}
        typed = self.vchildren.procs(INPUT_PROGRAMS)
        bad_argv = [p for p in typed if not wtype_allowed(p["argv"])]
        outside = [p for p in typed if not any(a <= p["first"] <= b for a, b in self.dictations)]
        players = self.vchildren.procs(("aplay",))
        checks += [
            {"name": "no input-executor name in internal/oni or internal/dictate outside internal/dictate/insert",
             "value": hits, "pass": not hits},
            {"name": "the scan sees the insert package (its wtype) and allows only it", "value": len(insert_hits),
             "pass": bool(insert_hits)},
            {"name": "boundary test: feature, presence and internal/oni cannot import the insert package",
             "value": tests.get("feature"), "pass": tests.get("feature", {}).get("pass")},
            {"name": "insert allowlist and target tests", "value": {k: tests.get(k) for k in ("insert", "target")},
             "pass": tests.get("insert", {}).get("pass") and tests.get("target", {}).get("pass")},
            {"name": "every input program the daemon ran was `wtype -s 50 -- CHUNK` (at most 40 characters, no control key)",
             "value": [p["argv"][:2] + [len(p["argv"][2]) if len(p["argv"]) > 2 else None] for p in bad_argv],
             "pass": not bad_argv},
            {"name": "and only inside a dictation", "value": [(p["comm"], p["first"]) for p in outside], "pass": not outside},
            {"name": "aplay never ran", "value": players, "pass": not players},
        ]
        return {"wtype_runs": len(typed), "tests": tests, "checks": checks}, self.checks_pass(checks)

    def run(self):
        if self.model is None:
            self.results["steps"].append({"step": 0, "name": "lab model", "pass": False,
                                          "error": "no LAB_MODEL_URL: phase11 needs up.sh --model"})
            self.save()
            return 1
        self.vchildren.start()
        try:
            self.step(0, "graph, key source, meter", self.census11)
            if self.keys is None:
                raise RuntimeError("no chord FIFO: the rest needs step 0")
            self.step(1, "push to talk", self.ptt)
            self.step(2, "barge-in", self.barge)
            self.step(3, "echo", self.echo)
            self.step(4, "wake", self.wake11)
            self.step(5, "open mic", self.openmic)
            self.step(6, "hygiene", self.hygiene)
            self.step(7, "dictation", self.dictation)
            self.step(8, "polish fallback", self.polish_fallback)
            self.step(9, "clip voice", self.clips11)
            self.step(10, "sound gate", self.gate11)
            self.step(11, "latency and resources", self.ladder11)
            self.step(12, "settlement", self.settle11)
            self.step(13, "read-only scan", self.readonly11)
        except RuntimeError as e:
            print(f"phase11: stopped: {e}")
        finally:
            self.vchildren.done.set()
            self.vchildren.join(timeout=5)
            self.watch.stop()
            if self.clock:
                self.clock.done.set()
            if self.keys:
                self.keys.close()
                self.results["keys_sent"] = self.keys.log
            self.results["daemon_programs"] = list(self.vchildren.seen.values())
            with (self.out / "frames.jsonl").open("w") as fh:
                for l in self.watch.lines:
                    fh.write(json.dumps(l) + "\n")
            self.results["frame_watch_restarts"] = self.watch.restarts
            self.results["ladder"] = self.ladder
        self.results["finished_at"] = now()
        self.save()
        for limit in self.results.get("limits", []):
            print(f"limit (step {limit['step']}): {limit['check']}: {limit['reason']}")
        return 0 if self.results["pass"] else 1


def summary_perch(lines):
    return labkit.summary(lines)["perch"] is not None


# dictate/insert's Settle: the ms each wtype call waits after its keymap
WTYPE_SETTLE = "50"


def wtype_allowed(argv):
    """dictate/insert's Allowed: wtype -s Settle -- CHUNK, CHUNK one argument
    of 1 to 40 characters with no control character but newline and tab."""
    if len(argv) != 5 or os.path.basename(argv[0]) != "wtype" or argv[1:4] != ["-s", WTYPE_SETTLE, "--"]:
        return False
    c = argv[4]
    return 0 < len(c) <= 40 and all(ch in "\n\t" or (ord(ch) >= 32 and ord(ch) != 127) for ch in c)


# --- command line -----------------------------------------------------------

def main(argv):
    if not argv:
        raise SystemExit(__doc__)
    cmd, args = argv[0], argv[1:]
    if cmd == "fixtures" and len(args) == 1:
        render_fixtures(args[0])
    elif cmd == "clips" and len(args) == 1:
        render_clips(args[0])
    elif cmd == "character" and len(args) == 3:
        return install_character(*args)
    elif cmd == "stage-model" and len(args) == 2:
        return stage_model(*args)
    elif cmd == "stt-defaults" and not args:
        print(" ".join(stt_defaults()))
    elif cmd == "key" and len(args) >= 2:
        return key_cmd(args[0], args[1:])
    elif cmd == "decode-events" and len(args) == 1:
        print(json.dumps(decode_events(Path(args[0]).read_bytes())))
    elif cmd == "mic-check" and args == ["--selftest"]:
        return mic_check_selftest()
    elif cmd == "mic-check" and len(args) == 3:
        report = mic_check(json.loads(Path(args[0]).read_text()), json.loads(Path(args[1]).read_text()), args[2])
        print(json.dumps(report))
        return 1 if report["problems"] else 0
    elif cmd == "phase11" and len(args) == 2:
        return Phase11(*args).run()
    else:
        raise SystemExit(__doc__)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
