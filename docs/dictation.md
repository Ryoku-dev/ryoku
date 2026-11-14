# Dictation

Voice typing is Ryoku's own. `ryoku-dictate` records the microphone, transcribes
on the machine and types the words into the window that has focus.
`Super + grave` starts and stops it; Ryoku Settings > Apps > Voice typing
(`ryoku/hub/quickshell/pages/VoiceTyping.qml`, backed by `ryoku-hub dictation`)
switches it on, chooses the model, and picks the computer that runs it.

The source is `ryoku/dictation/` and the package is `ryoku-dictation`, on every
edition. Nothing about it comes from an AUR helper. Voice typing is off until
you enable it in Settings. The shortcut starts the user unit on demand, and the
model is unmapped after 90 seconds of idleness.

## How it works

| Piece | What it does |
|---|---|
| `ryoku-dictate` | one process: PipeWire capture, inference, text insertion, its own settings file |
| `ryoku-shell` (`ipc/dictate.go`) | holds the connection, mirrors the state into the `dictation` bus topic |
| the voice card | `shell/modules/dictation/DictationSurface.qml`: a transparent, card-sized window at the focused screen's top-left, shared by every bar style |
| `ryoku-hub dictation` | the Settings page's backend: settings file, model catalogue, downloads |

Capture is a single 16 kHz mono PipeWire stream, and the same buffer feeds the
model and the meter, so the wave on screen is exactly what is being transcribed.
There is no second capture stream, no cava process and no bridge binary.

The card fades and lifts into place, then closes after transcription finishes.
Its Lithic engraved trace shows activity above the recent microphone baseline,
not absolute input gain. A short startup calibration keeps hot microphones at
rest until the sound changes; speech gets a quick rise and a softer release.
The separate dB readout remains the absolute captured level. Samples ease
between updates, and the chart range stays fixed so silence cannot expand into
a high plateau. While transcribing, Lithic's Working mark indicates progress
without pretending to be microphone activity. The card takes no pointer or
keyboard input.

Text arrives either by paste (the clipboard plus Ctrl+V, the default, because a
busy application drops synthesised keystrokes) or by typing, which the daemon
does itself over the Wayland virtual-keyboard protocol: one keymap carrying the
characters of the batch, one press per character. Each upload has a distinct
map identity so repeated dictations still type correctly after focus returns
from another app or the physical keyboard. `wtype` is no longer part of dictation.

## Models

The engine is [transcribe.cpp](https://github.com/handy-computer/transcribe.cpp)
(MIT), pinned to a release and built from source into the package, running ggml.
Word error rate is LibriSpeech test-clean at Q8_0, from the engine's own
pipeline, so the numbers compare.

| Model | Size | WER | Languages |
|---|---|---|---|
| parakeet-tdt_ctc-110m | 135 MB | 2.43% | English |
| parakeet-tdt-0.6b-v2 | 730 MB | 1.69% | English |
| parakeet-ultra | 740 MB | 1.80% | 25 European |
| whisper-large-v3-turbo | 886 MB | 2.01% | about 100 |

Updates deliver the engine, not speech models. Every model has an explicit
**Download** control in Settings > Apps > Voice typing, including Parakeet 110M.
Installed models have **Use** controls, and the selected model is marked
**In use** under **Engine and model**. A completed, verified download selects
that model, while an unfinished download leaves the current choice alone.

Each model is pinned by SHA-256 in `ryoku/hub/backend/dictation.go`, downloaded
to a `.part` file and renamed only after the hash matches, so a cancelled or
corrupt transfer can never become the model the daemon loads. Every installing
row shows **Cancel**.

## The CPU and GPU switch

The engine package includes CPU and Vulkan modules together under
`/usr/lib/ryoku/dictation`; GPU support no longer needs a separate package.
Settings probes the engine on the machine using it. A working Vulkan driver
and supported GPU are still required; CPU remains available when they are
absent. Asking explicitly for an unavailable GPU reports that failure rather
than silently switching to CPU.

Measured on an 11-second clip (Ryzen 9 7940HS, RTX 4060 / Radeon 780M): the
110M model takes 117 ms on the CPU and 42 ms on Vulkan; the 0.6B model
takes 485 ms and 113 ms. A GPU backend compiles its shaders on the first real
run, which measured 11 s on a cold box, so the daemon warms the model while the
card still says Preparing.

## Settings

`~/.config/ryoku/dictation.conf`, flat `key=value`, written only by Settings and
re-read by the daemon over its socket, so a change takes effect without a
restart: `enabled`, `source`, `model`, `device`, `language`, `mode`,
`max_duration_secs`, `silence_ms`, `silence_db`, `type_delay_ms`,
`idle_unload_secs`. The tool's `README.md` documents each one, the socket
protocol and the model catalogue.

A missing `enabled` key means off; an explicit saved choice survives updates.
The toggle updates the running engine and the shell's status together, and
turning it off cancels the current recording or transcription. Checking status
does not start the engine, and the shortcut checks the saved choice rather than
treating a stopped process as a disabled feature.

Source updates install the voice engine even when it was absent, through the
same model-free payload as the package. The executable, private CPU/Vulkan
libraries and on-demand service arrive together. The user manager reloads the
new service definition without enabling it; an active engine is restarted
unless `deploy.sh --no-reload` was requested. Enabling voice typing remains a
separate user choice, and pressing its shortcut starts the engine on demand.

Doctor uses the source checkout to identify this update lane, even when an older
desktop package is still installed, so it keeps the deployed engine instead of
removing it as an unowned package conflict.

## Tests

```bash
ryoku/dictation/build.sh              # CMake + Ninja, fetches the pinned engine
ryoku/dictation/build/dictate-test    # settings, UTF-8, keymap, WAV
ryoku/dictation/build/ryoku-dictate --selftest clip.wav --model model.gguf
RYOKU_TEST_HUB_BIN=/path/to/ryoku-hub python3 tests/voice-typing-page.py
RYOKU_TEST_HUB_BIN=/path/to/ryoku-hub python3 tests/dictation-store.py
python3 tests/dictation-levels.py
```

A dictation can also be proved end to end without a microphone: a PipeWire
loopback pair gives a virtual microphone, `pw-play` sends a clip into it, and the
daemon's `final` event carries the text. `RYOKU_DICTATE_NO_INSERT=1` keeps the
words out of the focused window while testing. The recipe is in
`ryoku/dictation/README.md`.
