# Notices: ryoku-dictation

Ryoku dictation is GPL-3.0-or-later. It builds against, and ships, the
following third-party work.

## transcribe.cpp

- Upstream: https://github.com/handy-computer/transcribe.cpp
- Revision: v0.4.2 (pinned in `ryoku/dictation/CMakeLists.txt` with a SHA-256 of
  the release tarball)
- Licence: MIT
- Used as: the speech inference engine, built from source and linked into
  `ryoku-dictate`. It is fetched at build time, never vendored.

## ggml

- Upstream: https://github.com/ggml-org/ggml (as vendored inside transcribe.cpp)
- Licence: MIT
- Used as: the tensor runtime under transcribe.cpp. The engine package includes
  the CPU and Vulkan backend modules.

## Silero VAD

- Upstream: https://github.com/snakers4/silero-vad
- Licence: MIT
- Used as: not shipped in this version. The daemon detects silence from the
  capture stream's own level, which needs no model.

## Model: Parakeet TDT-CTC 110M

- Weights: NVIDIA Parakeet, converted and quantised by handy-computer
  (https://huggingface.co/handy-computer/parakeet-tdt_ctc-110m-gguf)
- Licence: CC-BY-4.0
- Available as: a separate 135 MB download in Settings, pinned by SHA-256 in
  the model catalogue; weights are not part of the engine package.
- Attribution: NVIDIA Parakeet, via handy-computer's GGUF conversion.

## Optional models

Downloaded by the user from the pinned catalogue in
`ryoku/hub/backend/dictation.go`: Parakeet 0.6B and Parakeet Ultra (NVIDIA,
CC-BY-4.0, via handy-computer) and Whisper large-v3-turbo (OpenAI, MIT, via
handy-computer). Each catalogue entry carries its licence and SHA-256.

## Protocol

`proto/virtual-keyboard-unstable-v1.xml` is the Wayland virtual-keyboard
protocol, © its authors, MIT.
