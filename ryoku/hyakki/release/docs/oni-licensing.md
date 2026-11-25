# Oni licensing and asset declarations

Oni is Hyakki's on-screen companion. This document records what the Oni foundation ships, what later phases may add and under which terms, what must never enter the tree, and the decisions taken in the foundation's license review. The combined binaries remain GPL-3.0-only as described in [licensing.md](licensing.md).

## Upstream reference

Oni Vision's guide, voice and consent patterns are adapted from Nūs Companion by Pranav Desu (potlakai), <https://github.com/potlakai/nus-companion>, Apache-2.0. The review checked the upstream repository at commit `a12e9fb04f43d0dc85828522a91f1c839b27f3d5` (2026-11-21):

- `LICENSE` is the unmodified Apache License 2.0 text, byte-identical to `LICENSE.apache-2.0` (SHA-256 `cfc7749b96f63bd31c3c42b5c471bf756814053e847c10f3eb003417bc523d30`). It carries no filled-in copyright line.
- There is no upstream `NOTICE` file, so Apache section 4(d) adds no text to carry. The attribution in the root `NOTICE` is given as credit, together with the grant.
- The README states that "Nūs" and the Knot are trademarks of Pranav Desu. Hyakki does not ship either name or any of their artwork.
- `THIRD-PARTY.md` lists MIT portions (Hussain, 2026; OpenWhispr Team, 2024) and the bundled voice and speech components below. Hyakki copies no upstream source file, so none of those notices applies yet. Any later phase that adapts an upstream file must check whether it carries one of those portions and retain that notice next to the adapted source.

Motion numbers and methods (springs, timings, the bowed path) are facts and are recorded in [oni-animation-spec.md](oni-animation-spec.md) without upstream branding or the upstream status-capsule copy. Oni's screen-ask, guide, select-and-ask and voice prompts and its spoken and card lines are written for Oni; no upstream prompt or interface copy is used.

## Component matrix

| Component | License | Status | Shipping rule |
| --- | --- | --- | --- |
| Oni profile, silhouette `static.svg`, persona, lines | GPL-3.0-only (Hyakki original) | Bundled now, embedded in the binaries | Declared in `release/oni-assets.json`; `LICENSE.artwork` and `PROVENANCE.md` beside the files |
| Oni motion constants (`oni-spec.json`) | GPL-3.0-only (Hyakki source; numbers are facts) | Bundled now, embedded | Go source, not an asset |
| Speech engines, speech models, voices and the bench corpus | Per component, see [Voice and dictation](#voice-and-dictation) | Phase 11: external programs, downloads after consent, an optional pack or the private bench; none bundled | One row per component, with source, delivery and attribution |
| Text reader (`tesseract`) and its English data | Apache-2.0, see [Guides and the text reader](#guides-and-the-text-reader) | Phase 12: an optional external program and its data from the user's distribution; none bundled | One row per component, with source and delivery |
| Programs quick commands run (`pactl`, `systemd-run`, `grim`, `niri msg`, Ryoku's capture tool and window seam, installed apps and browsers, D-Bus services) and an optional local decision server | Per program, see [Quick commands, reminders and routines](#quick-commands-reminders-and-routines) | Phase 13: external programs and services the user's system already has; none bundled | One row per program, with its use and delivery |
| Programs Hands runs (a Chromium-family browser, `weston`, `niri`, `grim`, `wtype`) | Per program, see [Hands](#hands) | Phases 14 and 15: optional external programs from the user's distribution; none bundled | One row per program, with its use and delivery |
| MiniLM sentence embedding export (Xenova) | Apache-2.0 (model card MIT) | Later phase, only if adopted | License declared with the model |

Nothing in this matrix except the Oni profile files and motion constants is linked or bundled by the foundation. Models must come from their authors, be commit-pinned and SHA-256 verified, and non-commercial or no-derivatives licenses are excluded.

## Voice and dictation

Phase 11's voice loop (`internal/oni/voice/**`) and dictation (`internal/dictate/**`) run every engine as a separate child process found on `PATH`. No speech engine, model, voice or corpus is linked, embedded, vendored or bundled, so `release/oni-assets.json` declares none and the release check still fails on any model or voice file under the module's asset trees. The table lists everything the voice code can run or download. Each license was checked on 2026-11-22 against the project's own license file or model card at the URL given.

| Component | License | Verified at | Hyakki's use | Delivery | Attribution |
| --- | --- | --- | --- | --- | --- |
| whisper.cpp `whisper-server`, with ggml | MIT, © 2023-2026 The ggml authors | <https://github.com/ggml-org/whisper.cpp/blob/v1.9.1/LICENSE>, <https://github.com/ggml-org/ggml/blob/master/LICENSE> | `voice/stt` whisper adapter: one resident server on 127.0.0.1, the fallback engine | External program from the user's distribution, never built or bundled by Hyakki. The bench and the real-engine test ran v1.9.1, commit `f049fff95a089aa9969deb009cdd4892b3e74916`, built for CPU outside the repository | The MIT notice travels with the installed program; credited in `NOTICE` |
| OpenAI Whisper base.en, q5_1 GGML (`ggml-base.en-q5_1.bin`) | MIT, © 2022 OpenAI | <https://github.com/openai/whisper> ("code and model weights ... MIT"), <https://huggingface.co/ggerganov/whisper.cpp> (`license: mit`) | The fallback model (`stt.FallbackModel`) | Downloaded only after consent from `huggingface.co/ggerganov/whisper.cpp`, pinned at 59721011 bytes and SHA-256 `4baf70dd0d7c4247ba2b81fafd9c01005ac77c2f9ef064e00dcf195d0e2fdd2f`; never bundled | Credited in `NOTICE` |
| OpenAI Whisper large-v3-turbo, q5_0 GGML | MIT | <https://huggingface.co/openai/whisper-large-v3-turbo> (`license: mit`), <https://huggingface.co/ggerganov/whisper.cpp> | A catalogue choice only; it missed both budgets in the bench and is never a default | Downloaded only after consent, pinned at SHA-256 `394221709cd5ad1f40c46e6031ca61bce88931e6e088c188294c6d5a55ffa7e2`; never bundled | Credited in `NOTICE` |
| Silero VAD v6.2.0, GGML (`ggml-silero-v6.2.0.bin`) | MIT, © 2020-present Silero Team | <https://github.com/snakers4/silero-vad/blob/master/LICENSE>, <https://huggingface.co/ggml-org/whisper-vad> (`license: mit`) | Passed to `whisper-server --vad --vad-model` with the fallback model (`stt.FallbackVAD`) | Downloaded only after consent from `huggingface.co/ggml-org/whisper-vad`, pinned at SHA-256 `2aa269b785eeb53a82983a20501ddf7c1d9c48e33ab63a41391ac6c9f7fb6987`; never bundled | Credited in `NOTICE` |
| NVIDIA Parakeet TDT 0.6B v2, Q8_0 GGUF by handy-computer | CC-BY-4.0 | <https://huggingface.co/nvidia/parakeet-tdt-0.6b-v2>, <https://huggingface.co/handy-computer/parakeet-tdt-0.6b-v2-gguf> (card and `NOTICE`) | The default model (`stt.DefaultModel`), decoded by `ryoku-dictate` | Downloaded only after consent through Hyakki's pinned fetch (SHA-256 `f0d0e99cebb6d3b83f1f7069b82b5d3c2e39a54545b0da039cb4bafd9c4e5caa`), or used where Ryoku's dictation already downloaded it; never bundled | Required: the text under [Parakeet attribution](#parakeet-attribution), also in `NOTICE` |
| NVIDIA Parakeet TDT_CTC 110M, Q8_0 GGUF by handy-computer | CC-BY-4.0 | <https://huggingface.co/nvidia/parakeet-tdt_ctc-110m>, <https://huggingface.co/handy-computer/parakeet-tdt_ctc-110m-gguf> (card and `NOTICE`) | A smaller model for `ryoku-dictate` | Downloaded only after consent through Hyakki's pinned fetch (SHA-256 `7dd44c74a331d788a4e5f8b16913b3feb29ced22cf5613aad0e0f6cd30516296`), or used where Ryoku's dictation installed it (`/usr/share/ryoku/dictation/models`, `$XDG_DATA_HOME/ryoku/dictation/models`); never bundled | Required: the text under [Parakeet attribution](#parakeet-attribution), also in `NOTICE` |
| `ryoku-dictate` (Ryoku's `ryoku-dictation`), built on transcribe.cpp v0.4.2 and ggml | ryoku-dictation GPL-3.0-or-later; transcribe.cpp MIT, © 2026 The transcribe.cpp authors; ggml MIT | <https://github.com/handy-computer/transcribe.cpp/blob/main/LICENSE>; Ryoku's `ryoku/dictation/NOTICE.md`, retained in `THIRD_PARTY_NOTICES` | `voice/stt` dictate adapter: `--selftest FILE.wav` per decode, used only when it is installed | External program, never required, built or bundled by Hyakki. The bench ran the installed `ryoku-dictation` 1.0.0 | Ryoku's package carries its own notice |
| eSpeak NG (`espeak-ng`) | GPL-3.0-or-later | <https://github.com/espeak-ng/espeak-ng> (README "License Information", `COPYING`) | The default `system` voice (`espeak-ng --stdout --stdin`); the lab's speech fixtures and the bench's synthetic clips | External program; Hyakki ships none of its code or data. Generated fixtures stay in `/dev/shm` or the private evidence cache and are never committed | Credited in `NOTICE` as a separate program |
| Kokoro-82M model and voices | Apache-2.0 | <https://huggingface.co/hexgrad/Kokoro-82M> (`license: apache-2.0`) | The optional `kokoro` voice | The optional removable pack below; Hyakki neither bundles nor downloads it | The pack carries the license |
| espeak-ng-data inside the Kokoro pack | GPL-3.0-or-later | <https://github.com/espeak-ng/espeak-ng> (`COPYING`) | Phoneme data that sherpa-onnx reads for Kokoro | Only inside the optional pack, unmodified, as separate data loaded at run time | The pack carries the license and a source pointer |
| sherpa-onnx `sherpa-onnx-offline-tts` | Apache-2.0 | <https://github.com/k2-fsa/sherpa-onnx/blob/master/LICENSE> | Renders the Kokoro voice, one child per sentence | Optional external program; not built or bundled. Not installed on the build machine, so its run is proven only against a fake | Its own distribution |
| wtype | MIT, © 2019 Josef Gajdusek | <https://github.com/atx/wtype/blob/master/LICENSE> | `dictate/insert`, the only typing tool, at most 40 characters per call; a rewrite in place types its result through the same inserter, and its undo presses one Ctrl+Z at a time through `hands/input` | External program; not bundled | Its own distribution |
| wl-clipboard `wl-paste` | GPL-3.0-or-later, © Sergey Bugaev | <https://github.com/bugaevc/wl-clipboard> (`COPYING`; the Arch package's license field, checked 2026-11-23) | `rewrite/sel`: exactly `wl-paste --primary --no-newline --type text/plain;charset=utf-8`, the PRIMARY selection read for a rewrite in place. The clipboard is never read or written, `wl-copy` is never run, and no copy key is pressed | External program; not bundled or linked | Its own distribution |
| PipeWire `pw-cat` | MIT (Expat), © 2018 Wim Taymans; some PipeWire parts LGPL | <https://gitlab.freedesktop.org/pipewire/pipewire/-/blob/master/COPYING> | Microphone capture at 16 kHz; speech and sound playback | External program; not bundled | Its own distribution |
| PulseAudio `parec`, `paplay`, `pactl` | LGPL-2.1-or-later (utilities GPL when built with its GPL options) | <https://gitlab.freedesktop.org/pulseaudio/pulseaudio/-/blob/master/LICENSE> | Capture and playback fallbacks; the microphone list | External programs; not bundled | Their own distribution |
| LibriSpeech ASR corpus, 50 test-clean utterances | CC-BY-4.0 | <https://www.openslr.org/12/> | The private speech bench (`release/lab/oni/voicebench.sh`) only | Never shipped: staged in the private evidence cache with the tarball deleted after extraction; not in Git, the repository or the source archive | The bench evidence credits "LibriSpeech ASR corpus, Panayotov et al., OpenSLR SLR12" under CC BY 4.0 and records the tarball's digests |

Every bench row names exactly what ran, with the engine and the model's SHA-256. Parakeet TDT 0.6B v3 and Ryoku's "ultra" model are not in Hyakki's catalogue, and neither name is ever used for v2 or 110M. Hugging Face's transformers port of base.en (`openai/whisper-base.en`) is tagged Apache-2.0, but the GGML file comes from OpenAI's original checkpoint, released under MIT. Runtime downloads use only `stt.Catalogue`'s pinned URLs, sizes and digests, and the consent dialog shows each model's license.

### Parakeet attribution

This text accompanies every edition, in `NOTICE` and here:

> NVIDIA Parakeet TDT 0.6B v2 (<https://huggingface.co/nvidia/parakeet-tdt-0.6b-v2>) and Parakeet TDT_CTC 110M (<https://huggingface.co/nvidia/parakeet-tdt_ctc-110m>) are by NVIDIA and licensed under the Creative Commons Attribution 4.0 International license (CC BY 4.0, <https://creativecommons.org/licenses/by/4.0/>). The GGUF files Hyakki can download were converted by handy-computer for transcribe.cpp (<https://huggingface.co/handy-computer/parakeet-tdt-0.6b-v2-gguf>, from NVIDIA's commit 1b149a3; <https://huggingface.co/handy-computer/parakeet-tdt_ctc-110m-gguf>, from commit 431a349). That conversion renames and re-lays-out the tensors and stores the Q8_0 files at reduced numeric precision; the weights were not retrained. Hyakki does not modify the files. No endorsement by NVIDIA or handy-computer is implied.

### Optional Kokoro voice pack

The `kokoro` voice is a pack in `$XDG_DATA_HOME/hyakki/oni/voice/kokoro` holding `model.onnx`, `voices.bin`, `tokens.txt` and an `espeak-ng-data/` folder, with optional `lexicon*.txt`, plus `sherpa-onnx-offline-tts` on `PATH` (`tts.FindKokoro`). It matches the layout of sherpa-onnx's Kokoro model archives. Hyakki never downloads, installs or bundles it, and `release/` defines no package for it. Deleting the folder removes the voice, and Oni falls back to the character's voice, then `system`.

A distributor who packages the pack must ship it as a separate, removable package that Hyakki does not depend on, carrying:

- Kokoro-82M's Apache-2.0 license text beside the model and voices;
- `espeak-ng-data` unmodified, with the GPL version 3 text and a pointer to the corresponding eSpeak NG source;
- sherpa-onnx's Apache-2.0 text, if its program ships in the same package.

## Guides and the text reader

Phase 12's guides, Keep, hints and select-and-ask (`internal/oni/guide/**`, `keep/**`, `ocr/**`, `settle/**` and the guide and select parts of `screenask/**`) bundle no asset, model or data file, so `release/oni-assets.json` declares nothing new. The text reader is run as a separate child process found on `PATH`; nothing of it is linked, embedded, vendored, bundled, installed or downloaded by Hyakki. Each license was checked on 2026-11-23 against the project's own license file.

| Component | License | Verified at | Hyakki's use | Delivery | Attribution |
| --- | --- | --- | --- | --- | --- |
| Tesseract OCR (`tesseract`, or `tesseract-ocr` on Void) | Apache-2.0 | <https://github.com/tesseract-ocr/tesseract/blob/5.5.3/LICENSE>, byte-identical to `LICENSE.apache-2.0`; [Void's template](https://github.com/void-linux/void-packages/blob/master/srcpkgs/tesseract-ocr/template) renames its executable | `oni/ocr`: one child per read, only the selected executable followed by `stdin stdout -l eng --psm 11 tsv`, the picture as PNG on stdin and word boxes as TSV on stdout, nothing on disk; `tesseract-ocr` is selected only when `tesseract` is absent | Optional external program from the user's distribution; without it guides run as best guesses and keeping, replay and select-and-ask are unavailable. Checked against tesseract 5.5.3 as installed on the build machine; Void's alternate executable still needs a real packaged-box OCR run | Its own distribution |
| Tesseract English trained data (`eng.traineddata`, tessdata 4.1.0) | Apache-2.0 | <https://github.com/tesseract-ocr/tessdata/blob/4.1.0/LICENSE>, byte-identical to `LICENSE.apache-2.0` | The `eng` language the reader names; the only language proven | Installed with tesseract by the user's distribution (the build machine's file is byte-identical to tessdata 4.1.0's); never bundled or downloaded by Hyakki | Its own distribution |

The guide, select-and-ask and hint prompts, the guide card's step and closing lines, and the keep and hint questions were written new for Oni. Their marker format (`[POINT]`, `[CONF]`, `[NEXT]`, `[DONE]`, `[NOTE]`, `[ANSWER]`) and the select answer's JSON shape are Hyakki's own. No upstream prompt, interface copy, character name or research text is carried, and this document quotes none.

Kept walkthroughs and the hint record live in the user's state directory and are written by Hyakki at run time; they hold no picture, window title or typed words, and nothing of them ships.

## Quick commands, reminders and routines

Phase 13's quick commands, decisions, reminders, routines and notes (`internal/oni/reflex/**`, `remind/**`, `routine/**`, the executors in `internal/reflex/**`, and `daemon/oni_reflex*.go`, `oni_remind.go`, `oni_routine.go`) bundle no asset, model or data file and add no Go module, so `release/oni-assets.json` declares nothing new. Every program below runs as a separate child process, or is reached as a separate service over the session bus, through an exact allowlist and never a shell; none of it is linked, embedded, vendored, bundled, installed or downloaded by Hyakki. The D-Bus calls use `github.com/godbus/dbus/v5`, already vendored and listed in `THIRD_PARTY_NOTICES`. Each license was checked on 2026-11-23 against the package installed on the build machine.

| Component | License | Hyakki's use | Delivery |
| --- | --- | --- | --- |
| PulseAudio `pactl` (from libpulse) | LGPL-2.1-or-later | `reflex/audio`: seven exact argv, all on `@DEFAULT_SINK@`: read the volume, step it 5 % up or down, set it to 100 %, mute on, off or toggle | External program; not bundled |
| systemd `systemd-run` | LGPL-2.1-or-later | `reflex/launch`: `systemd-run --user --scope --collect -- EXEC`, so an app outlives the daemon's unit; without a systemd user manager, or when the scope is refused, the app starts detached instead (`scope=fallback`) | Part of the user's init system where it is systemd; never required |
| Installed apps and browsers (desktop entries) | Each app's own | `reflex/launch` and `reflex/browser`: an app starts from its own `Exec` line with field codes stripped; a browser is handed one `about:blank` or https address. A web search builds a `https://duckduckgo.com/?q=` address, which is an address and carries no code | The user's own applications; Hyakki ships none |
| MPRIS media players | Each player's own | `reflex/audio`: over the session bus, only the bus's name list, a player's `PlaybackStatus` and its `PlayPause` with no arguments | Separate services; nothing bundled |
| Desktop notification server (`org.freedesktop.Notifications`) | Its own | `reflex/notify`: one fixed `Notify` call while Oni is hidden | Separate service; nothing bundled |
| grim | MIT | `reflex/shot`: one whole-screen PNG named `screenshot-YYYYMMDD-HHMMSS.png` in the screenshots folder, when Ryoku's capture tool is absent | External program; not bundled |
| Ryoku's capture tool (`ryoku-cmd-ryoshot`) and window seam (`ryoku wm`) | Ryoku's own | `reflex/shot` opens the capture tool with no arguments; `reflex/wm` reads `ryoku wm caps` and `state` and runs only `ryoku wm act window.place` for snap left and right | Part of the Ryoku desktop when installed; never required |
| niri `niri msg` | GPL-3.0-or-later | `reflex/wm`: only `niri msg action maximize-column` | The user's compositor; not bundled |
| A local decision server (a `systemone` route on a loopback address) | Its own | `providers.decisions` `local`: one typed question per call to the loopback address and model the user names, only after an on-demand `GET <address>/models` finds it and this computer passes the hardware gate (6 GiB of memory, 4 physical cores); never started by Hyakki | Optional, installed and run by the user; Hyakki bundles, installs, downloads and starts none, and adds no unit for one |

`wtype` is not used by quick commands: nothing under `internal/reflex/`, `internal/oni/reflex/`, `remind/` or `routine/` names an input tool, and `internal/reflex/act`'s boundary test fails if one does. Closing or moving between tabs, which would need key chords into another window, is not offered. `wtype` stays the typing tool of dictation's inserter and `hands/input`; a rewrite in place reaches it only through those two.

The registry, the rule table, the splitter, the sound-alike matcher, the reminder and timer phrasing and every line Oni says or shows for these features were written for Hyakki in its own words, from the behavior the research describes; no upstream rule source was available or used, and the Apache-2.0 attribution in the root `NOTICE` stays. Reminders, routines and the in-memory quick-command ledger are written by Hyakki at run time; they hold no transcript, and nothing of them ships.

## Hands

Hands (`internal/hands/**`, `daemon/oni_hands*.go`) bundles no asset, model or data file and adds no Go module, so `release/oni-assets.json` declares nothing new. Its programs run as separate child processes found on `PATH`, each through one exact argv; none is linked, embedded, vendored, bundled, installed or downloaded by Hyakki. Each license below was read on 2026-11-23 in the project's own license file at the URL given. A program whose terms were not read there is marked unverified, and nothing ships under it.

| Component | License | Verified at | Hyakki's use | Delivery |
| --- | --- | --- | --- | --- |
| Chromium (`chromium`, `chromium-browser`) | BSD-3-Clause, © 2015 The Chromium Authors; the third-party code Chromium includes keeps its own terms | <https://chromium.googlesource.com/chromium/src/+/main/LICENSE> | `hands/chrome`: the real browser binary behind the launcher, on its own `hands-browser` profile, with only `--user-data-dir`, `--remote-debugging-pipe`, `--class`, `--no-first-run`, `--no-default-browser-check`, `--disable-extensions`, `--headless` when asked, and the start address. No network port is opened; Flatpak, snap and shell launchers are refused | External program from the user's distribution; not bundled |
| Google Chrome, Brave, Vivaldi, Microsoft Edge, Thorium | Each vendor's own terms; **unverified**, not read for this review | None | The same argv, when the user has one of them and no Chromium | The user's own installation; Hyakki names none as a dependency |
| weston (`weston`) | MIT (Expat), © 2008-2012 Kristian Høgsberg, © 2010-2012 Intel Corporation and others | <https://gitlab.freedesktop.org/wayland/weston/-/blob/main/COPYING> (SHA-256 `fdb65868f65d0fbdb05c2d3b779e10ce9969fa0c4b9262ba4f260e87086ab860`) | `hands/nest`: the headless parent of the separate desktop, exactly `weston --backend=headless --shell=kiosk --idle-time=0 --no-config --width=W --height=H --socket=NAME` | External program; not bundled |
| niri (`niri`) | GPL-3.0-or-later | <https://github.com/YaLTeR/niri/blob/main/LICENSE> (GPL version 3 text) and `Cargo.toml` (`license = "GPL-3.0-or-later"`, version 26.4.0) | `hands/nest`: `niri -c CONFIG` nested inside that weston, with a config Hyakki writes into the desktop's private folder; also `niri msg action maximize-column` for quick commands | External program; the user's compositor or a separate install; not bundled |
| grim (`grim`) | MIT, © 2018 emersion | <https://git.sr.ht/~emersion/grim/blob/master/LICENSE> | `hands/nest`: `grim -t png -l 3 -` inside the separate desktop, the picture on stdout; also the quick-command screenshot | External program; not bundled |

`wtype` is Hands' only keyboard tool, through `hands/input`; its row is under [Voice and dictation](#voice-and-dictation). [native-distribution.md](native-distribution.md) lists every optional program with its package name per distribution, from `release/optional.json`.

## Other optional programs

Phase 17's optional-program manifest also names these two existing external programs. Their upstream grants were read on 2026-11-23; no new code, model or asset is bundled by that manifest.

| Component | License | Verified at | Delivery |
| --- | --- | --- | --- |
| uv (`uv`) | MIT OR Apache-2.0, at the recipient's option | [Upstream license statement](https://github.com/astral-sh/uv/blob/main/README.md#license) and [MIT text](https://github.com/astral-sh/uv/blob/main/LICENSE-MIT), copyright 2025 Astral Software Inc. | The user's distribution, only used to set up the optional Hindsight engine when asked; not bundled |
| llama.cpp (`llama-server`) | MIT for llama.cpp itself, copyright 2023-2026 The ggml authors; packaged third-party portions keep their own terms | [Upstream LICENSE](https://github.com/ggml-org/llama.cpp/blob/master/LICENSE); [Fedora's package metadata](https://packages.fedoraproject.org/pkgs/llama-cpp/llama-cpp/) additionally declares Apache-2.0 and public-domain portions | The user's distribution or the existing on-request runtime download; not bundled |

The `license` field in `release/optional.json` identifies the named upstream program, not every third-party license in a distribution's binary. In particular, Chromium's BSD-3-Clause row and niri's GPL-3.0-or-later row are not a replacement for their distributors' full notices. External-program rows do not grant permission to redistribute a vendor browser, a speech model or a voice pack. A distributor who adds any of those must supply its own full license and corresponding-source review. Unverified vendor browsers remain unverified and are not nominated as package dependencies.


## Never copied

- Nixi (heynixi) code, artwork, voices, voice clips and their line list, persona text or branding. heynixi is closed source and its license forbids redistribution and reverse engineering; only behavioral observations are used, and raw research stays out of the repository and source archives.
- The Coucou character or its assets.
- The Nūs and Knot names, logos, character artwork, sounds, the Fable voice assets, prompts that name the upstream character, and the upstream status-capsule copy.
- Any artwork from `assessment/` or another product's brand material.

## Recorded decisions

1. **Artwork.** The default Oni artwork is parametric: code draws it from the motion spec (`artwork.kind` "parametric"). The fallback `static.svg` is a minimal geometric silhouette authored for Hyakki. Its `PROVENANCE.md` states that it contains no Nūs, Knot, Nixi, Coucou or third-party art. It is covered by the distribution's GPL-3.0-only combination.
2. **Voice path.** Oni speaks with the `system` voice, an `espeak-ng` child process, until an original clip bank is recorded. eSpeak NG is GPL-3.0-or-later, compatible with this distribution's GPL-3.0-only combination, and Hyakki runs the installed program without shipping its code or data. Original recorded clip packs remain the intended default and carry no GPL data: the loader rejects a clip pack lacking a license or provenance file, and clips must come from an original script and an owned or licensed recording. The clip engine is proven with a pack generated at test time and never committed. A Kokoro voice with its unmodified espeak-ng-data is the optional, removable pack described above and is not bundled. No audio ships.
3. **Parity document.** The motion tables are copied as numbers into `release/docs/oni-animation-spec.md` so the spec parity test works in the extracted source archive, which excludes `research/`.
4. **Text reader.** Guides, Keep, replay and select-and-ask read window words with the user's installed `tesseract` rather than a reader Hyakki ships. Tesseract and its trained data are Apache-2.0, compatible with this distribution's GPL-3.0-only combination as a separate program; Hyakki runs the installed program through one allowlisted argv and ships none of its code or data, and the repository holds no `.traineddata` file. The daemon asks only for `eng`, the one language proven.
5. **Quick commands act only through programs the user has.** Apps, browsers, `pactl`, `systemd-run`, `grim`, `niri msg`, Ryoku's tools, media players and the notification server are run or called as separate programs and services through exact allowlists; no executor lives under `internal/oni/`, none types into another window, and no local decision runtime, service or unit is bundled. Hyakki only detects one the user installed, by looking at its loopback model list on demand behind a hardware gate read from the existing local model scan.
6. **Rules in Hyakki's own words.** The upstream rule source was not available, so the quick-command rules were authored from the described behavior alone, with the Apache-2.0 attribution kept in the root `NOTICE`. The mark scan was extended to `internal/reflex/`.

## Declaring bundled assets

`release/oni-assets.json` lists every file bundled under `internal/oni/characters/`, and any model or voice data anywhere under the module's asset trees. Each entry has exactly these fields:

| Field | Meaning |
| --- | --- |
| `path` | Module-relative file under `internal/`, `vendor/` or `share/` |
| `spdx` | SPDX identifier, limited to GPL-3.0-only, GPL-3.0-or-later, Apache-2.0, MIT, CC-BY-4.0, CC0-1.0, BSD-2-Clause, BSD-3-Clause or OFL-1.1 |
| `license_file` | The license file for the entry, itself declared and named so the notices inventory collects it (`LICENSE*`, `NOTICE*`, `*.license`) |
| `provenance` | A declared provenance file stating author, origin and recording or drawing method |
| `source` | Short origin statement |
| `sha256` | Digest of the file, recorded by `python3 release/native.py oni-assets` |

Go files under `characters/` are embedding code, not assets. A clip pack, model or voice pack declares each file the same way, with its own license and provenance.

## Verification

`python3 release/native.py oni-assets --check` (also run by `check-source`, `notices`, `install` and `source`) fails when:

- a file or symlink under `internal/oni/characters/` is undeclared, or a declared file is missing;
- a declared license or provenance file is missing, undeclared, or not collected into `THIRD_PARTY_NOTICES`;
- a recorded digest is absent or differs from the file;
- a license is outside the allowed set;
- `internal/oni/`, `internal/dictate/`, `internal/reflex/`, `internal/hands/`, `internal/mcpserve/` or `release/docs/oni-animation-spec.md` contains the Nūs, Knot, Nixi, heynixi or Coucou marks, after Unicode normalization and case folding (the capitalized Knot word only);
- the root `NOTICE` lacks the Apache-2.0 attribution to Pranav Desu's Nūs Companion;
- `*.onnx`, `*.bin`, `*.gguf`, `*.wav`, `*.ogg`, `*.opus`, `*.flac`, `*.mp3` or `espeak-ng-data` content appears under the module's asset trees without a declaration.

The mark scan covers `internal/oni/`, `internal/dictate/`, `internal/reflex/`, `internal/hands/` and `internal/mcpserve/`, plus the animation spec. The legal regression test copies every scanned tree and injects forbidden marks into each of those five code trees, including the stdio MCP server; it is not a claim that the runtime or package lifecycle was exercised. For Phase 11 the dictation and Oni trees were also reviewed by hand: neither holds a forbidden mark, the ASCII spelling "Nus", a lowercase "knot", a license header or text copied from a third party or from the research material. For Phase 12 the review was repeated, and every string literal in the non-test Go files of both trees through Phase 12 was compared with the research material word for word: no literal shares a run of five or more words with it except code shapes (character-class patterns, URLs, model file names, a box's coordinate names) and generic validation messages. These are historical Phase 11 and 12 reviews, not a new word-for-word review of the later phases.

For Phase 13 the review covered `oni/reflex/**`, `oni/remind/**`, `oni/routine/**`, `reflex/**`, `daemon/oni_reflex*.go`, `oni_remind.go`, `oni_routine.go`, `spec/reflex.go`, `protocol/oni_reflex.go`, `pages/settings/oni_reflex.go` and `cmd/hyakki/oni_reflex.go`, 62 files counting their tests. None holds a forbidden mark, the ASCII spelling "Nus", a lowercase "knot" or a license header. The 1919 string literals in their non-test Go files were compared with the research material word for word: the only runs of five or more words they share are code shapes (number and weekday alternations, address and clock patterns, `XDG_DATA_DIRS` defaults, a format string), the spoken commands a user says ("set a timer for 5 minutes", "remind me in 20 minutes", "run my routine") and one generic clause. The sensitive-note refusal, which first repeated a sentence of the research material, was reworded in Hyakki's own words before this review closed. The copy audit found no third-party product or model name in any line Oni says or shows for these features: such names appear only in code comments, in the site table's lookup keys and addresses (a spoken line says "Opening the page.", never the site), and in the default search address. `oni/reflex/decide`'s audit test holds its own strings to the same rule, and the Settings tests hold the decision question and switch to it. A launch says the user's own app name back ("Opening NAME."), which comes from the user's installed desktop entries, not from Hyakki's copy.

`python3 release/native.py oni-assets` records digests for files that are already declared; it never declares a new file. After editing an asset, review the change, record its digest, then regenerate `release/SOURCES` and `THIRD_PARTY_NOTICES`. `release/test_oni_legal.py` runs the check against temporary copies with each violation injected, and `release/check.sh` runs it. `release/check.py` confirms each declared license appears in `THIRD_PARTY_NOTICES` and that this document is installed in `/usr/share/doc/hyakki/`.
