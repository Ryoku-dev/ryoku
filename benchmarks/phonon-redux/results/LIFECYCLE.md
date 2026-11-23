# Live installation validation

Validated on 2026-10-23 using the running Ryoku Hub and its actual backend.

- Phonon-2 appears directly below Base English. The existing card supplies
  install, cancellation, selection and uninstall controls.
- Clicked the installed Phonon card's remove control. The backend adopted the
  session-created installation, validated ownership, and removed the complete
  dedicated venv, model repository directory, unit and receipt. Redux remained
  selected, enabled and active throughout.
- The first fresh installation exposed an incorrect engine cache environment
  path. It rolled back all staged files, preserved Redux and left no partial
  installation. Corrected the path and added a regression assertion to the
  installer tests; the fresh retry completed successfully.
- Fresh runtime: Python 3.13.17, Fermion Research 0.2.13, PyTorch 2.14.1+cpu,
  no CUDA distributions. The service unit passed systemd verification.
- Selected Phonon using its actual Hub card. The server became healthy before
  the preset changed, Voxtype restarted with model `phonon-2`, and the Redux
  worker stopped.
- Normal `voxtype transcribe` on clean-5s returned every reference word correctly
  through the localhost Phonon endpoint. Health reported the native AVX2 CPU
  encoder and C TDT loop, four threads, and no request errors.
- Dictation off stopped Voxtype and Phonon. Selecting Redux started its worker
  and kept Phonon stopped. Selecting Phonon again restored its server and stopped
  Redux. Phonon is the final selected/enabled model.
- Installer `check-remove` passed after service use and transcription, proving
  runtime activity did not invalidate the ownership receipt.

Validation: 293 Go tests, 22 installer tests, four benchmark normalization tests,
Go vet, shell syntax and diff checks; focused Go checks repeated after the final
status-label adjustment. The screenshot `hub-phonon-selected.png` shows the live
model ordering and selection. Benchmark results are in `REPORT.md` and
`results.json` beside this file.
