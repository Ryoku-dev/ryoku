#!/usr/bin/env python3
"""Compare the installed, resident Phonon and Redux engines without changing services."""

import argparse
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import shlex
import socket
import statistics
import subprocess
import threading
import time
import urllib.request
import uuid
import wave


ROOT = Path(__file__).resolve().parent
DEFAULT_CORPUS = Path.home() / "Projects/voxtype-redux-adapter/benchmarks/audio/manifest.json"
DEFAULT_MODEL = Path.home() / ".local/share/voxtype/models/redux-packed.gguf"


def sha256_file(path):
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def normalize(text):
    text = re.sub(r"\bmr\.", "mister", text.lower())
    text = re.sub(r"\bmrs\.", "missus", text)
    return re.findall(r"[a-z0-9]+(?:'[a-z0-9]+)?", text)


def score(reference, hypothesis):
    ref, hyp = normalize(reference), normalize(hypothesis)
    if not ref:
        raise ValueError("reference has no words")
    previous = list(range(len(hyp) + 1))
    for i, word in enumerate(ref, 1):
        row = [i]
        for j, candidate in enumerate(hyp, 1):
            row.append(min(row[-1] + 1, previous[j] + 1,
                           previous[j - 1] + (word != candidate)))
        previous = row
    return {"word_errors": previous[-1], "reference_words": len(ref),
            "wer": previous[-1] / len(ref)}


def corpus(manifest_path, selected):
    rows = json.loads(manifest_path.read_text())
    if selected:
        rows = [row for row in rows if row["id"] in selected]
        if set(selected) != {row["id"] for row in rows}:
            raise ValueError("unknown case requested")
    if not rows:
        raise ValueError("empty benchmark corpus")
    for row in rows:
        path = manifest_path.parent / Path(row["file"]).name
        if sha256_file(path) != row["sha256"]:
            raise ValueError(f"corpus hash mismatch: {path}")
        with wave.open(str(path), "rb") as audio:
            if (audio.getframerate(), audio.getnchannels(), audio.getsampwidth()) != (16000, 1, 2):
                raise ValueError(f"expected 16 kHz mono PCM16: {path}")
            duration = audio.getnframes() / audio.getframerate()
        if abs(duration - row["seconds"]) > 0.001:
            raise ValueError(f"corpus duration mismatch: {path}")
        row["file"] = str(path.resolve())
        row["reference_words"] = len(normalize(row["reference"]))
    return rows


def get_json(url):
    with urllib.request.urlopen(url, timeout=10) as response:
        return json.load(response)


def redux_request(path, payload):
    with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as connection:
        connection.settimeout(600)
        connection.connect(str(path))
        connection.sendall(json.dumps(payload).encode() + b"\n")
        with connection.makefile("rb") as stream:
            line = stream.readline(1024 * 1024 + 1)
        if not line.endswith(b"\n") or len(line) > 1024 * 1024:
            raise ValueError("invalid Redux response length")
        result = json.loads(line)
        if not result.get("ok"):
            raise RuntimeError(result.get("error", "Redux request failed"))
        return result


def phonon_request(endpoint, case):
    before = get_json(endpoint + "/health")["stats"]
    start = time.perf_counter()
    boundary = "benchmark-" + uuid.uuid4().hex
    data = bytearray()
    for name, value in (("model", "phonon-2"), ("response_format", "json")):
        data.extend((f"--{boundary}\r\nContent-Disposition: form-data; name=\"{name}\"\r\n\r\n"
                     f"{value}\r\n").encode())
    data.extend((f"--{boundary}\r\nContent-Disposition: form-data; name=\"file\"; filename=\"clip.wav\""
                 "\r\nContent-Type: audio/wav\r\n\r\n").encode())
    data.extend(Path(case["file"]).read_bytes())
    data.extend(f"\r\n--{boundary}--\r\n".encode())
    request = urllib.request.Request(endpoint + "/v1/audio/transcriptions", data=bytes(data),
                                     headers={"Content-Type": f"multipart/form-data; boundary={boundary}"})
    with urllib.request.urlopen(request, timeout=600) as response:
        result = json.load(response)
    wall = time.perf_counter() - start
    after = get_json(endpoint + "/health")["stats"]
    if after["requests"] != before["requests"] + 1 or after["streams"] != before["streams"]:
        raise RuntimeError("another Phonon request overlapped this trial; engine timing is ambiguous")
    if after["errors"] != before["errors"]:
        raise RuntimeError("Phonon reported a request error")
    return {"text": result["text"], "wall_seconds": wall,
            "engine_seconds": after["decode_seconds"] - before["decode_seconds"]}


def service_pid(name):
    result = subprocess.run(["systemctl", "--user", "show", name, "--property=MainPID", "--value"],
                            check=True, capture_output=True, text=True)
    pid = int(result.stdout.strip())
    if pid <= 0:
        raise RuntimeError(f"{name} is not running; benchmark never starts or stops services")
    return pid


def memory(pid):
    result = {}
    for line in Path(f"/proc/{pid}/status").read_text().splitlines():
        key, _, value = line.partition(":")
        if key in ("VmRSS", "VmHWM", "VmSwap"):
            result[key + "_mib"] = int(value.split()[0]) / 1024
    return result


class MemorySampler:
    def __init__(self, pids):
        self.pids = pids
        self.before = {name: memory(pid) for name, pid in pids.items()}
        self.sampled_peak_rss_mib = {name: row["VmRSS_mib"] for name, row in self.before.items()}
        self.stop = threading.Event()
        self.thread = threading.Thread(target=self.sample, daemon=True)

    def sample(self):
        while not self.stop.wait(0.05):
            for name, pid in self.pids.items():
                try:
                    self.sampled_peak_rss_mib[name] = max(self.sampled_peak_rss_mib[name], memory(pid)["VmRSS_mib"])
                except FileNotFoundError:
                    return

    def finish(self):
        self.stop.set()
        self.thread.join()
        return {name: {"pid": pid, "before": self.before[name], "after": memory(pid),
                       "sampled_peak_rss_mib": self.sampled_peak_rss_mib[name]}
                for name, pid in self.pids.items()}


def environment():
    cpu = next((line.partition(":")[2].strip() for line in Path("/proc/cpuinfo").read_text().splitlines()
                if line.startswith("model name")), platform.processor())
    governor = Path("/sys/devices/system/cpu/cpu0/cpufreq/scaling_governor")
    return {"timestamp": dt.datetime.now(dt.timezone.utc).isoformat(), "cpu": cpu,
            "logical_cpus": os.cpu_count(), "kernel": platform.release(), "python": platform.python_version(),
            "governor": governor.read_text().strip() if governor.exists() else None,
            "loadavg": list(os.getloadavg())}


def reproduction_command(args, cases, output):
    """Build a replay command from every effective benchmark input."""
    manifest = args.manifest.resolve()
    socket_path = args.socket.absolute()
    model_path = args.redux_model.absolute()
    argv = ["python3", "benchmarks/phonon-redux/run.py", "--run",
            "--manifest", replay_path_arg(manifest), "--cases",
            *(shlex.quote(case["id"]) for case in cases),
            "--rounds", str(args.rounds), "--endpoint", shlex.quote(args.endpoint.rstrip("/")),
            "--socket", replay_path_arg(socket_path), "--redux-model", replay_path_arg(model_path),
            "--output", replay_path_arg(output)]
    return " ".join(argv)


def replay_path_arg(path):
    """Quote a path, using shell variables for this user's home/runtime roots."""
    path = Path(path).resolve()
    roots = []
    if os.environ.get("HOME"):
        roots.append(("HOME", Path.home().resolve()))
    if os.environ.get("XDG_RUNTIME_DIR"):
        roots.append(("XDG_RUNTIME_DIR", Path(os.environ["XDG_RUNTIME_DIR"]).resolve()))
    for variable, root in roots:
        try:
            relative = path.relative_to(root)
        except ValueError:
            continue
        if str(relative) == ".":
            return f'"${variable}"'
        return f'"${variable}"/{shlex.quote(relative.as_posix())}'
    return shlex.quote(str(path))


def recorded_path(path):
    """Normalize machine-local roots in JSON metadata as the corpus does."""
    path = Path(path).resolve()
    roots = []
    if os.environ.get("HOME"):
        roots.append(("HOME", Path.home().resolve()))
    if os.environ.get("XDG_RUNTIME_DIR"):
        roots.append(("XDG_RUNTIME_DIR", Path(os.environ["XDG_RUNTIME_DIR"]).resolve()))
    for variable, root in roots:
        try:
            relative = path.relative_to(root)
        except ValueError:
            continue
        return f"${variable}" if str(relative) == "." else f"${variable}/{relative.as_posix()}"
    return str(path)


def report(result, destination):
    rows = result["trials"]
    lines = ["# Phonon-2 versus Redux Parakeet", "", result["environment"]["timestamp"], "",
             f"Machine: {result['environment']['cpu']}; {result['environment']['logical_cpus']} logical CPUs; "
             f"governor `{result['environment']['governor']}`. Each engine received one separate five-second warm-up, "
             f"then {result['rounds']} measured runs per case. Calls ran serially, alternating which engine ran first.", "",
             f"Phonon: Fermion {result['settings']['phonon'].get('version', 'unknown')}, native CPU engine with "
             f"{result['settings']['phonon']['decode']['threads']} threads. Redux: resident CPP adapter, two native "
             f"slots with {result['settings']['redux']['pool_threads']} compute threads each and quiet splitting near "
             f"{result['settings']['redux']['chunk_seconds']:g} seconds. Both services stayed loaded; inference calls "
             "never overlapped. Model-loading times and full settings are recorded in `results.json`.", "",
             f"This was a live desktop benchmark. The one-minute system load average changed from "
             f"{result['environment']['loadavg'][0]:.1f} to {result['environment_after']['loadavg'][0]:.1f}. "
             "The run was not an isolated, fixed-frequency laboratory test; the ranges below show the observed "
             "latency variation.", "",
             "These are resident-engine request timings after the complete WAV is available. Phonon uses its local HTTP "
             "audio endpoint; Redux uses its local Unix-socket adapter. Wall time includes the respective file/transport "
             "work and response parsing. It excludes model startup, microphone capture, the Voxtype CLI and typing into apps. "
             "Phonon engine time is the server's decode counter delta; Redux engine time is its adapter's inference_seconds "
             "(including WAV decode and quiet chunking), so wall time is the primary speed comparison.", "",
             "| Input | Phonon median wall | Redux median wall | Redux / Phonon | Phonon WER | Redux WER |",
             "|---|---:|---:|---:|---:|---:|"]
    for case in result["corpus"]:
        by_engine = {engine: [row for row in rows if row["engine"] == engine and row["case"] == case["id"]]
                     for engine in ("phonon", "redux")}
        medians = {engine: statistics.median(row["wall_seconds"] for row in selected)
                   for engine, selected in by_engine.items()}
        wers = {engine: statistics.median(row["wer"] for row in selected)
                for engine, selected in by_engine.items()}
        lines.append(f"| {case['id']} ({case['seconds']:.3f} s) | {medians['phonon']:.3f} s | "
                     f"{medians['redux']:.3f} s | {medians['redux']/medians['phonon']:.2f}× | "
                     f"{wers['phonon']*100:.2f}% | {wers['redux']*100:.2f}% |")
    lines += ["", "| Engine | Input | Wall time range | Median engine time | Hypotheses |",
              "|---|---|---:|---:|---:|"]
    for case in result["corpus"]:
        for engine in ("phonon", "redux"):
            selected = [row for row in rows if row["engine"] == engine and row["case"] == case["id"]]
            times = [row["wall_seconds"] for row in selected]
            lines.append(f"| {engine} | {case['id']} | {min(times):.3f} to {max(times):.3f} s | "
                         f"{statistics.median(row['engine_seconds'] for row in selected):.3f} s | "
                         f"{len({row['text'] for row in selected})} |")
    lines += ["", "| Server | Sampled peak RSS | Final lifetime high-water RSS | Final swap |",
              "|---|---:|---:|---:|"]
    for engine, row in result["memory"].items():
        lines.append(f"| {engine} | {row['sampled_peak_rss_mib']:.1f} MiB | {row['after']['VmHWM_mib']:.1f} MiB | "
                     f"{row['after']['VmSwap_mib']:.1f} MiB |")
    provenance = result.get("configuration", {}).get("provenance")
    if provenance:
        lines += ["", "Replay metadata note: " + provenance]
    lines += ["", "RSS is sampled from each service's main process every 50 ms. It includes loaded libraries and "
              "workspace; shared pages may appear in both processes. Lifetime high-water RSS can include activity "
              "before this benchmark, so it is reported separately.", "",
              "This small English corpus uses LibriSpeech validation samples and contains one speaker. "
              "Whole utterances were assembled with 0.2-second gaps and recur across duration cases. The noisy case "
              "adds deterministic white noise at 20 dB SNR. This is neither a general accuracy benchmark nor a test of "
              "microphone dictation, meetings, accents or multilingual recognition. Do not aggregate overlapping clips "
              "as if they were independent test samples.", "",
              "WER uses the existing Redux benchmark normalization: lowercase, expand Mr./Mrs., then extract words "
              "while preserving internal ASCII apostrophes and digits. Raw references, hypotheses, edit counts, "
              "WAV hashes, effective input paths, runtime settings and every trial are preserved in `results.json`.", "",
              "Corpus source: [LibriSpeech ASR corpus](https://openslr.org/12/). "
              "The original corpus builder is `voxtype-redux-adapter/benchmarks/prepare_audio.py`.", "",
              "From the repository root, reproduce with both services already running:", "", "```bash",
              result["command"], "```", ""]
    (destination / "REPORT.md").write_text("\n".join(lines))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--preflight", action="store_true", help="validate corpus only; no service calls or inference")
    mode.add_argument("--run", action="store_true", help="run against already-running resident engines")
    parser.add_argument("--manifest", type=Path, default=DEFAULT_CORPUS)
    parser.add_argument("--cases", nargs="+")
    parser.add_argument("--rounds", type=int, default=3)
    parser.add_argument("--endpoint", default="http://127.0.0.1:8010")
    parser.add_argument("--socket", type=Path, default=Path(os.environ.get("XDG_RUNTIME_DIR", f"/run/user/{os.getuid()}")) / "voxtype-redux/worker.sock")
    parser.add_argument("--redux-model", type=Path, default=DEFAULT_MODEL)
    parser.add_argument("--output", type=Path, default=ROOT / "results")
    args = parser.parse_args()
    if args.rounds < 3:
        parser.error("at least three measured rounds are required")
    cases = corpus(args.manifest.resolve(), args.cases)
    if args.preflight:
        print(json.dumps({"cases": [{k: row[k] for k in ("id", "seconds", "reference_words", "sha256")} for row in cases]}, indent=2))
        return
    if args.output.exists() and any(args.output.iterdir()):
        parser.error("output directory is not empty; choose a new --output directory")
    args.output.mkdir(parents=True, exist_ok=True)
    phonon = get_json(args.endpoint.rstrip("/") + "/health")
    redux = redux_request(args.socket, {"command": "status"})
    if phonon.get("model") != "phonon-2" or phonon.get("decode", {}).get("engine") != "cpu" or phonon["decode"].get("threads") != 4:
        raise RuntimeError("expected Phonon-2 CPU engine with four threads")
    if redux.get("backend") != "cpp" or redux.get("pool_threads") != 4 or redux.get("chunk_seconds") != 30:
        raise RuntimeError("expected Redux CPP pool4/chunk30 runtime")
    pids = {engine: service_pid(unit) for engine, unit in (("phonon", "phonon-2.service"), ("redux", "voxtype-redux.service"))}
    replay_output = args.output.resolve() / ("reproduction-" + uuid.uuid4().hex[:8])
    configuration = {"manifest": recorded_path(args.manifest.resolve()),
                     "cases": [case["id"] for case in cases],
                     "endpoint": args.endpoint.rstrip("/"),
                     "socket": recorded_path(args.socket.absolute()),
                     "redux_model": recorded_path(args.redux_model.absolute()),
                     "output": recorded_path(args.output.resolve())}
    result = {"environment": environment(), "rounds": args.rounds, "corpus": cases,
              "configuration": configuration,
              "settings": {"phonon": phonon, "redux": redux}, "warmups": [], "trials": [],
              "artifacts": {"redux_model_sha256": sha256_file(args.redux_model),
                            "phonon_model_sha256": phonon.get("sha256")},
              "command": reproduction_command(args, cases, replay_output)}

    def infer(engine, case):
        if engine == "phonon":
            return phonon_request(args.endpoint.rstrip("/"), case)
        start = time.perf_counter()
        response = redux_request(args.socket, {"command": "transcribe", "audio": case["file"], "model": str(args.redux_model)})
        return {"text": response["text"], "engine_seconds": response["inference_seconds"], "wall_seconds": time.perf_counter() - start}

    sampler = MemorySampler(pids)
    sampler.thread.start()
    try:
        warmup = min(cases, key=lambda row: row["seconds"])
        for engine in ("phonon", "redux"):
            row = infer(engine, warmup)
            row.update(engine=engine, case=warmup["id"])
            result["warmups"].append(row)
            print(f"warmup {engine}: {row['wall_seconds']:.3f}s", flush=True)
        for round_index in range(args.rounds):
            for case_index, case in enumerate(cases):
                order = ("phonon", "redux") if (round_index + case_index) % 2 == 0 else ("redux", "phonon")
                for engine in order:
                    row = infer(engine, case)
                    row.update(engine=engine, case=case["id"], round=round_index + 1, audio_seconds=case["seconds"])
                    row.update(score(case["reference"], row["text"]))
                    result["trials"].append(row)
                    (args.output / "results.partial.json").write_text(json.dumps(result, indent=2) + "\n")
                    print(f"round {round_index+1} {engine:6s} {case['id']:15s} wall={row['wall_seconds']:.3f}s "
                          f"engine={row['engine_seconds']:.3f}s WER={row['wer']*100:.2f}%", flush=True)
        result["memory"] = sampler.finish()
        result["environment_after"] = environment()
        if {engine: service_pid(unit) for engine, unit in (("phonon", "phonon-2.service"), ("redux", "voxtype-redux.service"))} != pids:
            raise RuntimeError("a model service restarted during the benchmark")
        (args.output / "results.json").write_text(json.dumps(result, indent=2) + "\n")
        report(result, args.output)
        (args.output / "results.partial.json").unlink()
        print(f"report: {args.output / 'REPORT.md'}", flush=True)
    finally:
        sampler.stop.set()
        sampler.thread.join()


if __name__ == "__main__":
    main()
