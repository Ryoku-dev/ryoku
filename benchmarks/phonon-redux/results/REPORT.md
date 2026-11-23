# Phonon-2 versus Redux Parakeet

2026-10-23T19:01:26.438683+00:00

Machine: Intel(R) Core(TM) i5-8350U CPU @ 1.70GHz; 8 logical CPUs; governor `powersave`. Each engine received one separate five-second warm-up, then 3 measured runs per case. Calls ran serially, alternating which engine ran first.

Phonon: Fermion 0.2.13, native CPU engine with 4 threads. Redux: resident CPP adapter, two native slots with 4 compute threads each and quiet splitting near 30 seconds. Both services stayed loaded; inference calls never overlapped. Model-loading times and full settings are recorded in `results.json`.

This was a live desktop benchmark. The one-minute system load average changed from 4.0 to 14.3. The run was not an isolated, fixed-frequency laboratory test; the ranges below show the observed latency variation.

These are resident-engine request timings after the complete WAV is available. Phonon uses its local HTTP audio endpoint; Redux uses its local Unix-socket adapter. Wall time includes the respective file/transport work and response parsing. It excludes model startup, microphone capture, the Voxtype CLI and typing into apps. Phonon engine time is the server's decode counter delta; Redux engine time is its adapter's inference_seconds (including WAV decode and quiet chunking), so wall time is the primary speed comparison.

| Input | Phonon median wall | Redux median wall | Redux / Phonon | Phonon WER | Redux WER |
|---|---:|---:|---:|---:|---:|
| clean-5s (5.015 s) | 0.923 s | 1.060 s | 1.15× | 0.00% | 0.00% |
| clean-15s (14.470 s) | 2.271 s | 3.198 s | 1.41× | 7.89% | 10.53% |
| clean-30s (29.400 s) | 4.790 s | 6.627 s | 1.38× | 10.29% | 11.76% |
| noise-30s-20db (29.400 s) | 6.340 s | 6.902 s | 1.09× | 10.29% | 11.76% |
| clean-60s (59.565 s) | 11.059 s | 15.022 s | 1.36× | 7.04% | 7.75% |
| clean-120s (119.705 s) | 22.264 s | 29.266 s | 1.31× | 5.92% | 5.23% |

| Engine | Input | Wall time range | Median engine time | Hypotheses |
|---|---|---:|---:|---:|
| phonon | clean-5s | 0.716 to 0.963 s | 0.916 s | 1 |
| redux | clean-5s | 0.952 to 1.157 s | 1.059 s | 1 |
| phonon | clean-15s | 2.200 to 2.301 s | 2.261 s | 1 |
| redux | clean-15s | 3.169 to 3.401 s | 3.197 s | 1 |
| phonon | clean-30s | 4.568 to 6.506 s | 4.776 s | 1 |
| redux | clean-30s | 6.418 to 6.733 s | 6.625 s | 1 |
| phonon | noise-30s-20db | 4.880 to 6.999 s | 6.325 s | 1 |
| redux | noise-30s-20db | 6.791 to 8.573 s | 6.900 s | 1 |
| phonon | clean-60s | 10.349 to 12.669 s | 11.023 s | 1 |
| redux | clean-60s | 14.085 to 17.298 s | 15.020 s | 1 |
| phonon | clean-120s | 19.685 to 24.692 s | 22.213 s | 1 |
| redux | clean-120s | 28.753 to 31.830 s | 29.264 s | 1 |

| Server | Sampled peak RSS | Final lifetime high-water RSS | Final swap |
|---|---:|---:|---:|
| phonon | 1135.6 MiB | 1135.6 MiB | 0.0 MiB |
| redux | 673.6 MiB | 673.0 MiB | 0.0 MiB |

Replay metadata note: Reconstructed from the archived six-case corpus, recorded rounds/output, and installed service defaults; the previous harness did not save CLI overrides.

RSS is sampled from each service's main process every 50 ms. It includes loaded libraries and workspace; shared pages may appear in both processes. Lifetime high-water RSS can include activity before this benchmark, so it is reported separately.

This small English corpus uses LibriSpeech validation samples and contains one speaker. Whole utterances were assembled with 0.2-second gaps and recur across duration cases. The noisy case adds deterministic white noise at 20 dB SNR. This is neither a general accuracy benchmark nor a test of microphone dictation, meetings, accents or multilingual recognition. Do not aggregate overlapping clips as if they were independent test samples.

WER uses the existing Redux benchmark normalization: lowercase, expand Mr./Mrs., then extract words while preserving internal ASCII apostrophes and digits. Raw references, hypotheses, edit counts, WAV hashes, effective input paths, runtime settings and every trial are preserved in `results.json`.

Corpus source: [LibriSpeech ASR corpus](https://openslr.org/12/). The original corpus builder is `voxtype-redux-adapter/benchmarks/prepare_audio.py`.

From the repository root, reproduce with both services already running:

```bash
python3 benchmarks/phonon-redux/run.py --run --manifest "$HOME"/Projects/voxtype-redux-adapter/benchmarks/audio/manifest.json --cases clean-5s clean-15s clean-30s noise-30s-20db clean-60s clean-120s --rounds 3 --endpoint http://127.0.0.1:8010 --socket "$XDG_RUNTIME_DIR"/voxtype-redux/worker.sock --redux-model "$HOME"/.local/share/voxtype/models/redux-packed.gguf --output /tmp/phonon-redux-repeat/reproduction-legacy
```
