# Dictation

Voice typing runs on [Voxtype](https://github.com/peteonrails/voxtype). Ryoku
Settings > Apps > Voice typing (`ryoku/hub/quickshell/pages/VoiceTyping.qml`, backed by
`ryoku-hub voxtype`) switches it on, picks the engine and model, downloads or
removes models and manages the user service. `Super + grave` starts and stops
dictation.

## Engines

| Option | What runs |
|---|---|
| Whisper models (Tiny to Large Turbo) | Stock Voxtype with a local Whisper model in `~/.local/share/voxtype/models`. |
| Redux Multilingual + Adapter | NVIDIA Parakeet through a resident worker, 25 European languages. systemd desktops only. |
| OpenAI API | Cloud Whisper with your own key; audio leaves the machine. |

Every option uses the stock `voxtype` binary, so selecting one never needs sudo.
Each model installs into a staging directory first and is published in one
atomic step, so a cancelled or failed install never replaces the selected model.
Every installing card shows **Cancel**, which stops the installer and all of its
children.

## Redux

Redux is built on the machine when it is picked, which keeps the native runtime
out of the packages and matched to the CPU it runs on. The installer
(`ryoku/hub/backend/assets/redux/setup.py`, embedded in `ryoku-hub`):

1. checks for Python 3.11, git, cmake, a C++ compiler and make, and names the
   install command for the edition (`ryoku-host pkg advice`) when one is missing;
2. builds the CPU parakeet.cpp runtime from a pinned commit, at low priority on
   half the cores at most, so the desktop stays responsive;
3. downloads the Redux model at a pinned revision and checks its SHA-256;
4. publishes the model, the runtime, the Voxtype Redux adapter, its worker unit,
   a licence notice and an ownership receipt.

The card shows the step it is on (fetching the source, preparing the build,
building the runtime, downloading the model, installing) and one progress bar
across all of them. The installer prints only those steps and percentages; the
build's own output goes to `~/.local/state/ryoku/redux-install.log`, which a
failed build names.

When Redux is selected, Voxtype calls the adapter as its Whisper CLI, and the
adapter hands each recording to the resident `voxtype-redux.service` over a
socket in `$XDG_RUNTIME_DIR/voxtype-redux`. A drop-in on `voxtype.service` makes
it start the worker. Picking another model stops and disables the worker and
keeps the files for a quick switch back. The minus button removes every file the
receipt lists; files changed since the install are reported and kept.

| File | What it is |
|---|---|
| `~/.local/share/voxtype/models/redux-packed.gguf` | the model |
| `~/.local/bin/voxtype-redux` | the adapter in client mode, Voxtype's CLI |
| `~/.local/share/voxtype-redux/` | the worker, `runtime/libparakeet.so`, `NOTICE.md` and the receipt |
| `~/.config/systemd/user/voxtype-redux.service` | the worker unit |

## Credits and licences

The adapter is [voxtype-redux-adapter](https://github.com/Sipper1236/voxtype-redux-adapter)
by Sipper1236 (MIT), shipped unmodified. The runtime is
[parakeet.cpp](https://github.com/mudler/parakeet.cpp) (MIT) with its ggml
submodule. The weights are Moondream's
[Parakeet Redux](https://huggingface.co/moondream/parakeet-redux), derived from
NVIDIA Parakeet TDT 0.6B v3 (CC-BY-4.0). The pinned revisions and checksum are in
`ryoku/hub/backend/assets/redux/UPSTREAM.md`.

## Tests

From `ryoku/hub/backend`: `go test ./...` and
`python -m unittest discover -s assets/redux -p '*_test.py'`. The offscreen
cancellation test drives the real page: build `ryoku-hub`, then run
`RYOKU_TEST_HUB_BIN=/absolute/path/to/ryoku-hub python tests/dictation-cancel.py`
from the repository root.
