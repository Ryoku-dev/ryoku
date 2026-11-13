# dictation

Ryoku's voice typing: a microphone stream, a speech model and a keyboard, in one
process.

```
Super + `  →  ryoku-shell voice  →  ryoku-dictate  →  text in the focused window
```

`ryoku-dictate` captures 16 kHz mono from PipeWire, transcribes locally with
[transcribe.cpp](https://github.com/handy-computer/transcribe.cpp) (Parakeet or
Whisper on ggml) and inserts the result through the Wayland virtual-keyboard
protocol. It is the whole stack: no Voxtype, no Python config editor, no cava
process for the meter, no second capture stream.

## Pieces

| Path | What it does |
|---|---|
| `src/main.cpp` | the daemon: socket, state machine, silence stop, idle unload |
| `src/capture.cpp` | one PipeWire stream; the same buffer feeds the model and the meter |
| `src/engine.cpp` | the transcribe.cpp boundary, ~150 lines so the engine can be swapped |
| `src/insert.cpp` | virtual-keyboard typing, clipboard paste, the keymap it uploads |
| `src/config.cpp` | `~/.config/ryoku/dictation.conf` |
| `src/wav.cpp` | a WAV reader for `--selftest` |
| `proto/virtual-keyboard-unstable-v1.xml` | the protocol we implement (MIT, wlroots) |

## The socket

`$XDG_RUNTIME_DIR/ryoku-dictate.sock`, one line per message.

- commands: `start`, `stop`, `cancel`, `toggle`, `status`,
  `set <device|mode|language|model> <value>`, `quit`
- replies: `ok`, `end`, and `state/model/backend/device/mode/language/gpu/enabled`
  lines for `status`
- events: `state <loading|recording|transcribing|inserting|idle|error>`,
  `level <rms> <peak>`, `final <text>` (`\n`, `\r`, `\\` escaped), `took <ms>`,
  `error <text>`, `backend <name>`, `model <path>`, `note <text>`

`ryoku-shell` holds one connection open, mirrors it into the `dictation` bus
topic and opens/closes the voice card from it. The Hub talks to the same socket
through `ryoku-hub dictation`.

## Settings

`~/.config/ryoku/dictation.conf`, flat `key=value`:

| Key | Default | Meaning |
|---|---|---|
| `enabled` | `true` | off means no capture, no model, no process work |
| `source` | empty | microphone node; empty or `default` is the session default |
| `model` | empty | explicit `.gguf`; empty resolves the shipped model, then a user copy, then the newest user model |
| `device` | `auto` | `auto`, `cpu` or `gpu`. `gpu` refuses to fall back silently |
| `language` | empty | hint for Parakeet v3/Ultra and Whisper; empty lets the model decide |
| `mode` | `paste` | `paste` (clipboard + Ctrl+V) or `type` (synthesised keys) |
| `max_duration_secs` | `120` | the card's blade measures against this |
| `silence_ms` | `1500` | stop after this much quiet once something was said; `0` disables |
| `silence_db` | `-45` | the floor that counts as speech |
| `type_delay_ms` | `2` | gap between synthesised keys |
| `idle_unload_secs` | `90` | unmap the model when idle; `0` keeps it resident |

## Models

The package ships `parakeet-tdt_ctc-110m-Q8_0` (135 MB, English, 2.43% WER on
LibriSpeech test-clean). The Hub installs more into
`$XDG_DATA_HOME/ryoku/dictation/models` from a pinned catalogue with SHA-256
checks: Parakeet 0.6B (1.69%, English), Parakeet Ultra (1.80%, 25 European
languages), Whisper large-v3-turbo (2.01%, ~100 languages). Downloads stage in
`<file>.part` and are renamed only after the hash matches.

## Compute: the CPU and GPU switch

The engine library and every ggml backend module live in one directory,
`/usr/lib/ryoku/dictation`, which is also where the daemon looks at startup. The
base package ships `libggml-cpu.so`; the optional `ryoku-dictation-vulkan`
package drops `libggml-vulkan.so` beside it, and `device=gpu` in the settings
file (or Settings > Apps > Voice typing) uses it. Nothing is reinstalled to switch.

| Model (11 s clip) | CPU | Vulkan |
|---|---|---|
| parakeet-tdt_ctc-110m | 117 ms | 42 ms |
| parakeet-tdt-0.6b-v2 | 485 ms | 113 ms |

A GPU backend compiles its shaders on the first real run, which measured 11 s on
a cold box, so `Engine::load` runs a half-second of silence before reporting the
model ready: the cost lands in the card's "Preparing" state instead of the first
dictation.

The daemon finds its own directory first, so a build tree, a package staging
root and an installed system all work with no setup. Two overrides exist for
packaging checks, never for users: `RYOKU_DICTATION_BACKENDS` (a backend
directory) and `RYOKU_DICTATION_PREFIX` (a staging root, so `makepkg`, `nix` and
`xbps-src` output can be proved before it is installed).

## Build and test

```bash
./build.sh                     # builds build/ryoku-dictate and build/dictate-test
./build/dictate-test           # config, UTF-8, keymap (xkbcommon compiles it), WAV
./build/ryoku-dictate --selftest clip.wav --model model.gguf
```

The engine is fetched at a pinned tag and hash by CMake; point `TRANSCRIBE_SRC`
at a checkout to build offline.

A dictation can be proved end to end without a microphone: make a loopback pair,
play a clip into it, record from the other end.

```bash
pw-loopback --capture-props='media.class=Audio/Sink node.name=ryoku-test-sink' \
            --playback-props='media.class=Audio/Source node.name=ryoku-test-mic' &
printf 'source=ryoku-test-mic\nsilence_ms=0\n' > /tmp/dictation.conf   # point XDG_CONFIG_HOME at it
RYOKU_DICTATE_NO_INSERT=1 ryoku-dictate --serve &
# start → pw-play --target ryoku-test-sink clip.wav → stop → read `final`
```

`RYOKU_DICTATE_NO_INSERT=1` keeps the text out of the focused window while
testing; `--print-config` reports the resolved model and built backends.
