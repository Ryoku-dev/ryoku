# Installed Phonon versus Redux benchmark

`run.py` measures the installed resident CPU engines on the existing Redux
LibriSpeech corpus. It never starts, stops or changes a service or selection.
The parent integration must finish and both servers must already be running.

Validate corpus hashes and PCM format without inference:

```bash
python3 benchmarks/phonon-redux/run.py --preflight
python3 -m unittest discover -s benchmarks/phonon-redux -p 'test_*.py'
```

Run three measured rounds of every case, following one model warm-up each:

```bash
python3 benchmarks/phonon-redux/run.py --run --rounds 3
```

Output is `results/results.json` plus `results/REPORT.md`. Interrupted runs keep
`results.partial.json`. Choose a fresh `--output` directory to repeat a run.

Expected settings: Phonon-2 on localhost port 8010, native CPU engine with four
threads; Redux CPP adapter at `$XDG_RUNTIME_DIR/voxtype-redux/worker.sock`, two
native slots with four threads each, quiet splitting near 30 seconds. Requests
are serial and the first engine alternates per case and round. The default
corpus is `~/Projects/voxtype-redux-adapter/benchmarks/audio/manifest.json`;
`--manifest` supports relocating it. WAVs are resolved relative to that file.

The primary timing includes each engine's installed request interface: HTTP
multipart audio for Phonon, Unix JSON RPC with a local WAV path for Redux. It
excludes model load, microphone capture, Voxtype CLI and text injection. Internal
engine timings have different boundaries and are reported separately.

The corpus has one English speaker, overlapping utterances across duration
cases, and one synthetic white-noise variant. Results describe this small corpus,
not general dictation quality. All transcripts and exact references are retained.
