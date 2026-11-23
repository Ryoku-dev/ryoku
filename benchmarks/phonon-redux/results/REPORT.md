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

RSS is sampled from each service's main process every 50 ms. It includes loaded libraries and workspace; shared pages may appear in both processes. Lifetime high-water RSS can include activity before this benchmark, so it is reported separately.

## CPU thread tuning

To choose the service's CPU thread count, the same two Phonon test clips ran on a second server at port 8011 with 1, 2, 4, 8 and Fermion's default thread setting (8 on this eight-logical-CPU machine). Each setting was measured three times in a rotating order. Each fresh server first processed the 5-second clip, then the 30-second clip; the first result is the first inference after startup and the second follows that warm request. The installed four-thread server on port 8010 remained loaded but idle. The Redux service was inactive, so these numbers compare Phonon settings only.

| Phonon threads | 5 s first-request median (range) | 30 s median (range) | WER, 5 s / 30 s |
|---:|---:|---:|---:|
| 1 | 1.956 s (1.769 to 2.293) | 11.516 s (10.699 to 11.997) | 0.00% / 10.29% |
| 2 | 1.140 s (0.932 to 1.141) | 6.160 s (5.793 to 6.239) | 0.00% / 10.29% |
| 4 | 0.804 s (0.733 to 0.843) | 4.610 s (4.558 to 4.715) | 0.00% / 10.29% |
| 8 | 1.342 s (0.976 to 1.380) | 4.368 s (3.787 to 4.499) | 0.00% / 10.29% |
| Automatic (8) | 1.052 s (0.990 to 1.912) | 4.399 s (3.894 to 5.294) | 0.00% / 10.29% |

Four threads stayed selected: it was about 24% faster than the automatic setting on the first five-second request, while the automatic setting was about 5% faster on the 30-second clip. Every setting returned the same five-second text. On the 30-second clip, the 1, 2 and 4 thread trials returned “Lynnell's”; the 8 thread and automatic trials returned “Linell's”. Both spellings had the same 10.29% WER here. The short-request advantage better fits ordinary dictation; the automatic setting's small long-clip gain does not justify its first-request latency here. This was a live desktop run, not an isolated fixed-frequency benchmark. The raw trials are in `thread-sweep.json`.

This small English corpus uses LibriSpeech validation samples and contains one speaker. Whole utterances were assembled with 0.2-second gaps and recur across duration cases. The noisy case adds deterministic white noise at 20 dB SNR. This is neither a general accuracy benchmark nor a test of microphone dictation, meetings, accents or multilingual recognition. Do not aggregate overlapping clips as if they were independent test samples.

WER uses the existing Redux benchmark normalization: lowercase, expand Mr./Mrs., then extract words while preserving internal ASCII apostrophes and digits. Raw references, hypotheses, edit counts, WAV hashes, runtime settings and every trial are preserved in `results.json`.

Machine-local home and runtime-directory prefixes in the raw JSON are normalized to `$HOME` and `$XDG_RUNTIME_DIR`.

Corpus source: [LibriSpeech ASR corpus](https://openslr.org/12/). The original corpus builder is `voxtype-redux-adapter/benchmarks/prepare_audio.py`.

Reproduce with both services already running:

```bash
python3 benchmarks/phonon-redux/run.py --run --rounds 3 --output /tmp/phonon-redux-repeat
```
