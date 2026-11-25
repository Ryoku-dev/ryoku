# Oni lab

Development tooling, not part of any package. These scripts run Oni, the
character surface and the Hyakki app inside a private, invisible compositor,
so screen captures and synthetic input never reach the desktop you are using.
Every target compositor is covered: niri and Hyprland.

Run these examples from the `ryoku/hyakki/` module directory. Source checks
use that direct module layout and the canonical shared Ryoku libraries.
Evidence must stay outside the entire repository, not just this module.

```sh
release/lab/oni/up.sh niri --app --surface   # or: up.sh hyprland --app --surface --outputs 2
release/lab/oni/shot.sh /path/to/frame.png
release/lab/oni/shot.sh /path/to/oni.png "1500,800 240x240"
release/lab/oni/input.sh click 126 200
release/lab/oni/ctl.sh hyakki call settings.set '{"patch":{"oni":{"reduceMotion":"on"}}}'
release/lab/oni/measure.sh settled /path/to/settled.json
release/lab/oni/down.sh

release/lab/oni/phase8.sh niri /path/to/evidence   # the whole Phase 8 gate, lab included
release/lab/oni/phase9.sh niri /path/to/evidence   # the Phase 9 strand and sound gate
release/lab/oni/phase10.sh niri /path/to/evidence  # the Phase 10 perch and screen-ask gate
release/lab/oni/phase11.sh niri /path/to/evidence  # the Phase 11 voice and dictation gate
release/lab/oni/phase12.sh niri /path/to/evidence  # the Phase 12 guide, Keep and select-and-ask gate
release/lab/oni/phase13.sh niri /path/to/evidence  # the Phase 13 quick-command, reminder and routine gate
release/lab/oni/phase14.sh niri /path/to/evidence  # the Phase 14 browser Hands gate
```

Without `--app` or `--surface`, `up.sh` starts only the compositor, for any
client you start yourself. Run each script as a command. Never source `env.sh`
or the lab's `lab.env` into an interactive or long-lived shell: that shell
would then point Wayland, D-Bus and Hyakki clients at the lab. Keep evidence
outside the repository and outside `ONI_LAB`.

## What runs

| Compositor | Host | Nested compositor |
|---|---|---|
| `niri` | headless weston, kiosk shell, `--idle-time=0` | niri with a lab `niri.kdl`, output `winit` |
| `hyprland` | headless wlroots `tinywl` on the integrated GPU's render node | Hyprland with a Lua config and headless outputs `HEADLESS-2`, `HEADLESS-3`, ... side by side |

Binaries come from `HYAKKI_BIN`, which defaults to `HYAKKI_BUILD_DIR`, which
defaults to this module's `build/native`, where `release/build.sh` puts them.

For a native Nix payload, keep the app ELF unwrapped and mount its complete
store and signed driver closure privately at `/nix/store`.
`ONI_LAB_SOFTWARE_MESA` names the real Mesa output for software EGL, with
its vendor JSON, DRI and GBM directories checked before launch. It does
not substitute host libraries or change the native executable.

`--app` and `--surface` start a private session bus and the real `oni`
daemon. `--app` then starts `hyakki-app` and waits until the compositor
reports a `dev.ryoku.Hyakki` window. `--surface` seeds Oni on
(`{"enabled": true, "oni": {"master": true}}`, otherwise
`{"enabled": true}`) and starts `oni-surface` with `HYAKKI_ONI_LAB=1` and its
ledger at `ledger/surface.jsonl`, before the app, then waits for its first
committed frame. The surface holds its instance lock in the lab's runtime
directory, so a launch by the app or `hyakki oni show` is a no-op.
Both session- and system-bus clients use that private bus. Host power-profile
changes must not reach the daemon or interrupt a settled resource sample.

Every process runs in its own transient user scope
(`systemd-run --user --scope`, named `oni-lab-<id>-<role>`) with
`MemoryMax`, `MemorySwapMax=0`, `CPUAccounting=yes` and `choom -n 1000`. The
daemon's and surface's roles carry a random suffix (`oni-<hex>`,
`surface-<hex>`) so a restart never collides with a scope still being
collected, and a daemon the surface started itself stays countable.

## Layout

Everything lives under `ONI_LAB` (default `/dev/shm/oni-lab`, at most 20 bytes,
under `/dev/shm` only; `ONI_LAB_ID` picks it, see below):

| Path | Contents |
|---|---|
| `lab.env` | lab id, compositor, display, primary output, output size, pointer extent, the session-environment checksum; later lines override earlier ones |
| `w/` | runtime directory of both compositors: sockets and compositor IPC |
| `rt/` | the app's runtime directory: session bus, Hyakki's socket and the surface lock |
| `home/`, `cfg/`, `state/`, `cache/`, `data/` | the app's HOME and XDG directories; `state/hyakki/oni-placement.json` is the saved placement, `state/hyakki/oni-keeps.json` the kept walkthroughs (mode 0600 in a 0700 folder; no title, typed words or pixels; `hyakki oni keeps --all` empties it) and `state/hyakki/oni-hints.json` the hint record; `state/hyakki/oni-reminders.json` and `oni-reflex-routines.json` are Phase 13's stores (0600, no transcript); `data/hyakki/oni/voice/models` holds the speech models `voice.sh stage-model` links in; `data/applications/` and `cfg/mimeapps.list` hold `launchfix.py`'s catalogue |
| `tmp/` | `TMPDIR` of every process under test; step 13 of Phase 10 and of Phase 12 requires it empty |
| `bin/` | refusal stubs first on every lab process's PATH: `systemctl`, `ryoku`, `ryoku-shell`, `ryoku-host`, each a copy of `refuse.c` built at `up.sh` (a program, so the read-only scan sees the argv the daemon ran, not `/bin/sh` running a script), and `systemd-run`, built from `scoperefuse.c`, which logs and refuses every transient scope as a user manager that says no would |
| `stub/` | Phase 13's accepting wake stub `wakeok`, built by `phase13.sh` from its own source at gate start; `phase13.sh wakestub accept` copies it over `bin/ryoku-host` |
| `out/` | Phase 13's fixture output: the lab browser's `tabs.json` and `tabs-requests.jsonl` |
| `wm/` | compositor configuration |
| `ledger/` | `surface.jsonl` from the running surface, `surface-N.jsonl` from earlier runs |
| `build/` | the pointer client and the fixture client, built on first use; the fixture pictures (`NAME.xrgb`, `NAME.json`) |
| `model/` | the vision fixture's port file (`up.sh --model`) |
| `hands/` | Phase 14's web fixture port file (`up.sh --hands`, one JSON object with both origins) |
| `keys/` | the chord FIFO (`voice.sh keys`) and Phase 14's Esc stop FIFO `stop.fifo` (`phase14.sh stopkeys`), each read by one process only |
| `log/` | every process's output, `refused.log`, `model-requests.jsonl`; for Phase 13 also `model-decisions.jsonl`, `mpris.jsonl`, `notify.jsonl` and `wake.log`; for Phase 14 `hands-requests.jsonl` (every request either origin saw) |

### Two labs at once

`ONI_LAB_ID` picks one of several labs that run side by side, for instance
one gate per phase. Unset or empty is the default lab: `/dev/shm/oni-lab`,
with scopes `oni-lab-<hex>-<role>`, the same as before. `ONI_LAB_ID=13`
(one to three lowercase letters or digits) is `/dev/shm/oni-lab-13`, with
scopes `oni-lab-13-<hex>-<role>`. `env.sh` derives and exports `ONI_LAB`
from it, so every script and kit a lab script starts addresses the same lab.
An explicit `ONI_LAB` that disagrees is refused. Set the variable on every
command for that lab:

```sh
ONI_LAB_ID=13 release/lab/oni/phase13.sh niri /path/to/evidence
ONI_LAB_ID=13 release/lab/oni/ctl.sh state
ONI_LAB_ID=13 release/lab/oni/down.sh
```

Nothing else is shared between labs:

- Every socket lives in that lab's `w/` and `rt/`: weston's `wl-host`,
  tinywl's `wayland-0`, the nested compositor's display and IPC (Hyprland's
  `hypr/<signature>/`), the private bus, PipeWire and pipewire-pulse, the
  keyring the bus activates, and Hyakki's socket and locks.
- Every config, state and cache file the lab writes also stays under its own
  directory.
- The model, decision and Hands fixtures each listen on a free loopback port,
  which they write to that lab's port file.
- Every transient scope carries that lab's random tag, and `labkit.py`
  finds a lab's processes only by that tag.
- `down.sh` matches processes by the lab's own scope tag and by environment
  values equal to `ONI_LAB` or under `ONI_LAB/`, so it never touches the
  other lab.

Only read-only inputs are shared: `HYAKKI_BIN`, the `VOICEBENCH` speech
models (copied into the lab) and the GPU render node.

Two gates running together share CPU time. That can stretch timing checks,
so compare `measure.sh` numbers only between runs that had the machine to
themselves.

## Scripts

- `up.sh niri|hyprland [--app] [--surface] [--model [--decisions]] [--hands] [--outputs N]` refuses an
  existing `ONI_LAB`. If any step fails it runs `down.sh` itself.
  `ONI_LAB_WIDTH`/`ONI_LAB_HEIGHT` set each output's size (1920x1080).
  `--outputs N` (Hyprland only, 1 to 4) places N headless outputs left to
  right; the pointer's absolute extent is their union. Hyprland also reads
  `ONI_LAB_TINYWL` (default `~/.cache/hyprnest/wlroots-build/tinywl/tinywl`)
  and `ONI_LAB_DRM` (default: the first amdgpu, i915 or xe render node).
  `--model` (with `--app` or `--surface`) starts the vision fixture
  (`model.py`) on a free loopback port before the daemon and seeds it as the
  llama.cpp address in `hyakki.json`, with the Ollama address on a path the
  fixture refuses, so a live local server is never asked. `--decisions`
  (with `--model`) also seeds the typed decision route, which only the app
  may change: `providers.decisions` local at the fixture's `/v1`, model
  `oni-fixture-decisions`, and `oni.reflex` local. Both compositors float
  the `notes` and `list` fixtures, Phase 12's guide window and Phase 13's
  lab browser (`dev.ryoku.OniBrowser`) by window rule; Hyprland also floats
  Phase 13's notes window (`dev.ryoku.OniNotes`), which niri leaves tiled
  so its column maximize shows in the geometry. `--hands` starts Phase 14's
  `handsfix.py`: two loopback origins, with the port file at `hands/port`
  and `LAB_HANDS_A` and `LAB_HANDS_B` in `lab.env`. Both compositors float
  the Hands browser (app id `chromium` or `hyakki-hands-<hex>`): niri on
  the right half, Hyprland on the second output when there is one. `up.sh`
  builds `bin/systemd-run` from `scoperefuse.c`.
  The two-output Hyprland rules select `HEADLESS-3` explicitly and use
  monitor-local positions for the Hands browser, app target and sentinel,
  so their boxes stay inside the output the compositor reports.
  Ordinary app targets sit clear of the glow's masked edge strips on both
  compositors. The masks stay enabled during typing and read-back.
- `shot.sh OUT.png [GEOMETRY]` captures the primary output, or the region
  `"X,Y WxH"` in global logical pixels, with grim. It writes `OUT.json` with
  the outputs, windows, layer surfaces and pixel statistics; `nonblank` is
  true for at least 16 colours and a mean channel standard deviation above 2.
- `input.sh` takes `move X Y`, `click X Y [right]`, `drag X1 Y1 X2 Y2`,
  `scroll X Y N`, `ptr STEP...`, `key ARG...` (one `wtype` call) and
  `type TEXT`, in global logical pixels over every output. The pointer is a
  small wlr virtual-pointer client. On first use it is generated with
  `wayland-scanner` from `wlr-virtual-pointer-unstable-v1.xml` and compiled
  into `build/`. The XML comes from `ONI_LAB_WLR_PROTOCOLS`, then the
  `wlr-protocols` package, then `ONI_LAB_WLROOTS_SRC/protocol` (default
  `~/.cache/hyprnest/wlroots-src`). Nothing third-party is stored here.
  Each keyboard call waits 50 ms after creating its virtual keyboard before
  sending the chord or text; a successful process exit alone is not proof
  that the intended editor received it.
- `ctl.sh` drives the processes under test:
  - `surface start|restart [HYAKKI_ONI_SCRIPT=...] [HYAKKI_ONI_TIMESCALE=...]
    [HYAKKI_ONI_POWER=...] [HYAKKI_ONI_PROFILE=/abs/dir]
    [HYAKKI_ONI_ARRIVALS=/abs/file] [HYAKKI_ONI_SOUND_FILE=/abs/file]`
    (re)starts `oni-surface`; `surface stop|kill`. `HYAKKI_ONI_PROFILE` writes
    `cpu.pprof` and `allocs.pprof` of the run into the directory when the
    surface exits. `HYAKKI_ONI_ARRIVALS` makes the fake daemon append one JSON
    line per `oni.strandArrived` call (`t`, `seq`, `skipped`, `advanced`);
    `HYAKKI_ONI_SOUND_FILE` writes every sound's PCM to a file instead of a
    player.
  - `daemon start|restart [HYAKKI_ONI_CAPTURE_PATHS=LIST]|stop|kill` for
    `oni`. `LIST` (comma separated: `toplevel`, `niri-window`,
    `output-crop`, `portal`, `x11`; an unknown name allows none) is the only
    capture the daemon may use; with it the daemon also gets
    `HYAKKI_ONI_LAB=1`, which is when it reads the hook.
  - `hyakki ARG...` runs the command line as a lab client, for example
    `hyakki call oni.open '{"renderer":"observer"}'` to read the placement.
  - `globals` runs `oni-surface --globals` against the nested compositor.
  - `state` prints outputs (with logical position), windows (with size and
    global position) and layer
    surfaces: `niri msg layers` or `hyprctl layers`, `null` where the
    compositor lists none (the ledger then stands in).
  - `fullscreen on|off` fullscreens the `dev.ryoku.Hyakki` window and confirms
    the compositor reports it: on Hyprland only fullscreen mode 2 counts, not a
    maximize (mode 1), which covers nothing.
- `outputs.sh` changes outputs and records the new pointer extent: `list`,
  `add NAME [X]` and `remove NAME` (Hyprland headless outputs), `scale NAME
  S`, `off NAME`, `on NAME [X]`. The nested niri has one `winit` output, so on
  niri only `scale` applies: `off` is accepted but leaves the output enabled.
- `window.sh` shows the fixture windows: `build [DIR]`, `globals`, `open
  NAME`, `get NAME`, `move NAME X Y`, `focus NAME`, `close NAME` (through the
  compositor, as a user would) and `kill NAME`. The client, `oni-fixture`
  (`fixturewin.c`), is one xdg-toplevel built on first use with
  `wayland-scanner` from the system's `xdg-shell.xml`; it shows a picture
  `fixture.py` draws from code (a 5x7 bitmap font, flat boxes, two marker
  colours) and binds no seat. `notes` (`dev.ryoku.OniFixture`, "Ledger
  notes", 720x460) carries three note lines, an "ignore previous
  instructions" block, a Password field and the Save notes and Delete all
  buttons; `notes-tiled` is the same picture under an app id no rule
  floats; `list` ("Shopping list") is a harmless second window. `globals`
  prints every global the compositor advertises, which Phase 10's census
  needs (the surface's `--globals` lists only what it binds). `build DIR`
  works without a lab, for a dry build under `/dev/shm`.
- `model.py` is the vision fixture: a deterministic loopback server on the
  OpenAI-compatible wire as llama-server answers it (`/v1/models` with one
  image-reading model, `oni-fixture-vision`; `/props`;
  `/v1/chat/completions`, streamed or not). On a pointing prompt it points at
  Save notes (the Password field when the request names it), found again in
  the received JPEG by its marker colour; a question about the window gets
  `[ANSWER]` and then plain words. Mode `hostile` answers as a model that
  obeyed the picture's injection (PWNED, a point at Delete all and a tool
  call), `fail` answers 500, `slow` waits. It keeps every request and picture
  in memory for the lab's control routes (`/_lab/requests`,
  `/_lab/image/N/K`, `/_lab/mode`, `/_lab/reset`) and logs each request
  without picture bytes. `model.py selftest` checks all of it in one process,
  with no lab and no compositor. It is never a model: evidence says so.
  For Phase 13 it also serves the typed decision route (`decidefix.py`):
  `POST /v1/systemone` for model `oni-fixture-decisions` answers the
  `intent` question from a keyword table, a clear phrase above the spec's
  thresholds and send, buy, delete, submit and pay as `none`; `POST
  /_lab/decide` sets mode `keyword`, `slow`, `fail`, `unsure` (valid but too
  weak to act on) or `hostile` (all weight on a label never offered), and
  `/_lab/decisions` lists and resets the decision requests, which are also
  appended to `model-decisions.jsonl` and never reach `/_lab/requests`.
- `measure.sh SCENARIO OUT.json` measures Oni as the lab stands. SCENARIO is
  `motion`, `settled`, `asleep`, `covered`, `hidden`, `off`, `strand-held`,
  `strand-retracted`, `perched` or `detached`; arranging it is
  the caller's job (see below). It waits until the ledger shows no commit for
  `ONI_MEASURE_QUIET` seconds (5; not for `motion`; for `off`, until the
  daemon has not run for as long), giving up after `ONI_MEASURE_TIMEOUT`
  (180), then samples for `ONI_MEASURE_SECONDS` (60).
  Exit 0 when every check passes, 1 when one fails.
- `audio.sh up|graph|clients|record OUT.wav|stop|down` is a private audio
  graph for Oni's sounds: `pipewire`, `wireplumber -p policy` (linking and
  defaults, no ALSA, Bluetooth or camera monitor) and `pipewire-pulse` in lab
  scopes, with the lab's HOME, XDG directories and private bus, sockets in
  `rt/` (so `oni-surface`'s `pw-cat` finds this graph at its default path
  and nothing else), drop-ins that turn off rtkit, the portal, the X11 bell,
  JACK detection and every D-Bus name, and a null sink `oni_lab` that never
  suspends as the default. `up` refuses a graph that holds any device.
  `record` runs `parec` on `oni_lab.monitor` (float32, mono, 48 kHz) into a
  WAV outside the lab; `stop` ends it with SIGINT so the header is written.
  For Phase 11, after `up`: `mic` adds a second never-suspending null sink
  `oni_lab_mic` and a virtual source `oni_lab_mic_in` that remaps its
  monitor, made the default source, refused unless `voicekit.py mic-check`
  finds no device and only lab modules; `inject WAV` plays a WAV into it
  and waits; `play WAV` plays one into `oni_lab` as a sync mark; `echo
  on|off` loops `oni_lab.monitor` into the microphone; `record-mic OUT.wav`
  records the microphone's monitor; `recorders` and `modules` list the
  capture streams and loaded modules.
- `voice.sh VERB ...` holds Phase 11's fixtures and key source. Nothing in
  it opens a microphone, a speaker or an input device. `fixtures DIR`
  renders the speech WAVs with `espeak-ng -w` and `clips DIR` the
  tone-coded clip pack (with its LICENSE and PROVENANCE.md), both under
  `/dev/shm` and without a lab. `stage-model ID` links speech model ID from
  `$VOICEBENCH/models` into the lab after checking its SHA-256 against
  `stt`'s catalogue; `character ID` installs a clip-voiced character;
  `keys` makes the chord FIFO and restarts `oni-surface` with
  `HYAKKI_ONI_KEYS_FIFO` pointing at it (raw `struct input_event` bytes,
  read in place of `/dev/input`, in the lab only); `chord EVENT...` writes
  `down`, `up`, `altgr-down`, `altgr-up`, `letter` or `wait:MS` into it;
  `text open|get|focus|close|kill NAME` drives the dictation targets
  (`textwin.py`, a GTK 4 window with one text view whose every change is
  written to a file outside the lab); `clipboard` prints the nested
  compositor's clipboard and primary selection.
- `phase8.sh niri|hyprland OUTDIR` runs the Phase 8 gate on one compositor:
  its own lab, every step, the measurements, `down.sh`. See below.
- `phase9.sh niri|hyprland OUTDIR` runs the Phase 9 gate likewise.
- `phase10.sh niri|hyprland OUTDIR` runs the Phase 10 gate likewise, with
  `--model`. See below.
- `phase11.sh niri|hyprland OUTDIR` runs the Phase 11 voice gate: its own
  lab with `--model`, the virtual microphone, the chord FIFO, speech
  rendered by `espeak-ng` and the staged speech models. See below.
- `phase12.sh niri|hyprland OUTDIR` runs the Phase 12 gate on top of the
  Phase 11 graph, key source and speech model, with the GTK 4 guide window
  `guidewin.py` (app id `dev.ryoku.OniGuide`, floated by `up.sh`'s rule).
  It refuses to start without `tesseract` and its `eng` data, `grim`, GTK 4
  bindings, numpy or PIL. Inside a finished lab it also serves
  `guidekit.py`: `phase12.sh guidewin open VARIANT DIR | get | focus |
  move X Y | output OUT | close | kill` runs `guidewin.py` (variants
  `normal`, `saveas`, `pin`) through `lab_tool` and refuses a window that
  does not float, and `phase12.sh daemon [HYAKKI_ONI_GUIDE_*=VALUE...]`
  restarts the daemon with only those lab hooks. See below.
- `phase13.sh niri|hyprland OUTDIR` runs the Phase 13 gate on top of the
  Phase 11 graph, key source and speech model, with `up.sh --decisions` and
  Phase 13's fixtures, all on the lab's private bus and under `/dev/shm`:
  `launchfix.py` writes the launcher catalogue ("Lindqvist Notes", which
  runs `textwin.py`, and "Oni Lab Browser", the https default) into the
  lab's XDG folders; `tabwin.py` is that browser, a GTK 4 window that opens
  one tab per launch with an address and writes its tab count to
  `out/tabs.json`; `mprisfix.py` is an MPRIS player that logs `PlayPause`;
  `notifyfix.py` owns `org.freedesktop.Notifications` and logs each
  `Notify`; `busfix.py` refuses any bus that is not a socket under
  `/dev/shm`. Each has a `selftest` on a private bus. `reflexkit.py` holds
  the speech fixtures, the pure checks (`selftest`) and the gate (`class
  Phase13` on voicekit's `Phase11`, with guidekit's helpers). Inside a
  finished lab `phase13.sh` also serves `daemon stop|start
  [HYAKKI_ONI_*=NUMBER...]`, `fixtures start|stop`, `app install|remove`
  (remove takes the notes app away), `wakestub accept|refuse`, `scope
  on|off` (the user manager's marker `rt/systemd/private`, refused unless
  `bin/systemd-run` is the lab's stub) and `worker start|stop` (`hyakki
  service routines`, the runit worker). See below.
- `phase14.sh niri|hyprland OUTDIR` runs the Phase 14 gate on top of the
  Phase 11 graph, key source and speech model, with `up.sh --hands`, the
  daemon's own Hands browser (the host's Chromium on the lab's profile
  `state/hyakki/hands-browser`) and `model.py`'s `hands` mode.
  `handsfix.py` serves the pages (`selftest` needs no lab). `handskit.py`
  holds the Esc stop writer (`esc FIFO EVENT...`), the speech fixtures,
  the pure checks (`selftest`) and the gate (`class Phase14` on reflexkit's
  `Phase13`). Inside a finished lab `phase14.sh` also serves:
  - `daemon stop|start [HYAKKI_ONI_*=NUMBER...]`: always with the stop FIFO
    and origin A's `/contact` as the start address;
  - `stopkeys`;
  - `esc down|up|press|letter|wait:MS`;
  - `route ROUTE`: the running app's page;
  - `focus|close ID` and `layout ID`: niri's app column at half the output.

  See below.
- `voicebench.sh stage|run OUTDIR|clean` is the speech-to-text bench behind
  `voice/stt`'s default engine. It needs no lab: everything it downloads,
  builds or renders lives in `VOICEBENCH` (`/dev/shm/hyakki-voicebench`).
  `stage` fetches the pinned models (SHA-256 checked), LibriSpeech
  test-clean (CC-BY 4.0, OpenSLR's md5 checked; 50 utterances of 2-9 s kept,
  the tarball deleted) and renders espeak-ng clips through `--stdout`; it uses
  `/usr/bin/whisper-server` when that starts, else builds whisper.cpp v1.9.1
  for CPU without ffmpeg. `run` drives each engine with the adapters' exact
  command lines and request fields (`voicebench.py`), every child with a
  private empty runtime directory and PipeWire/Pulse pointed nowhere, and
  writes `voicebench.json` (rows, summaries, decision) and `voicebench.md`.
  `clean` deletes `VOICEBENCH`.
- `down.sh` stops every process whose environment points into `ONI_LAB` or that
  runs in one of the lab's scopes. Then it checks that no process and no scope
  is left, removes `ONI_LAB` and compares `systemctl --user show-environment`
  with the checksum `up.sh` recorded. It exits non-zero if anything is left or
  changed. `ONI_LAB_SAVE=DIR` first copies the logs and the surface ledgers
  (`ledger/`) there, including Hyprland's instance log (Hyprland deletes it
  at exit). `ONI_LAB_KEEP=1` keeps
  the directory.

`labkit.py` holds the scripts' shared Python: state normalisation (window
records carry `pos` global, `box` output-local, `output`, `floating`,
`focused`), ledger reading, process counters, crops and the Phase 8, 9 and
10 steps. `voicekit.py` adds Phase 11 (`fixtures`, `clips`, `character`,
`stage-model`, `key`, `mic-check`, `decode-events`, `phase11 LAB OUT`),
`guidekit.py` Phase 12 (`fixtures DIR`, `selftest`, `phase12 LAB OUT`),
`reflexkit.py` Phase 13 (`fixtures DIR`, `selftest`, `phase13 LAB OUT`) and
`handskit.py` Phase 14 (`fixtures DIR`, `esc FIFO EVENT...`, `selftest`,
`phase14 LAB OUT`), each building on the one before without changing it.
`guidekit.py` reads
the guide's closing lines and kickers from `internal/oni/guide` and the
refusal line from `internal/oni/screenask` rather than copying them.
`guidewin.py APP_ID TITLE OUT_DIR [--variant normal|saveas|pin]` is the
guide target: one undecorated GTK 4 window whose document page holds File,
a Dead button that changes nothing and an Account PIN entry, and whose
menu page holds Back, Save, Save As... and Export, with no portal or
dialog. Each control is painted in a flat marker colour; after every
painted change it writes `state.json` (page, size and each visible
control's window-relative box) and each Save writes `saved.json`, so the
lab reads what happened without a screenshot. What is typed into the PIN
entry is never written. `model.py`'s guide mode answers guide prompts
from a script keyed on how many steps the prompt lists and select prompts
with the JSON answer, finding each control by its marker colour or the
`state.json` given to `/_lab/guide`; mode `endless` never finishes.
`fixtures/` holds the frame scripts the surface's fake daemon plays
(`HYAKKI_ONI_SCRIPT`): one entry per line, `{"atMs": N, "frame": {"kind": ...,
"data": ...}}` or `{"atMs": N, "eof": true}`. `atMs` counts wall-clock
milliseconds from `oni.open`; `HYAKKI_ONI_TIMESCALE` slows the surface's
engines, not the script. The `strand-*.jsonl` scripts send strand targets,
retracts, perches, bubbles and motion and voice frames; `phase9.sh` writes the
copy it plays to `OUTDIR/fixtures/`, with perch frames moved onto the
character's output and the `hyakki-app` window.

## Measuring

For each of `oni-surface` and the `oni` daemon, `measure.sh` reads before and
after the sample: the process's scope `CPUUsageNSec`
(`systemctl --user show`), `/proc/PID/stat` utime+stime, every thread's
`schedstat` run time and timeslices, voluntary and involuntary context
switches (per thread and for the leader), and `smaps_rollup` Pss (the daemon
is non-dumpable, so its Pss reads as unavailable). It also counts the
ledger's `commit` lines (one `wl_surface.commit` with new pixels each) and
`arm` lines (one frame callback requested each).

| Scenario | Arrange it with | Checks |
|---|---|---|
| `motion` | Oni shown and awake | the control: measure.sh pokes the character every 2 s; commits > 0 |
| `settled` | Oni shown and awake, untouched | no character event in the window, 0 commits, 0 armed callbacks, surface CPU < 0.05 %, at most 2 wakeups/s |
| `asleep` | 90 s untouched, ledger state `sleeping` | as `settled` |
| `covered` | `ctl.sh fullscreen on` (ledger `hide` reason `courtesy`) | as `settled` |
| `hidden` | `oni.hidden=true` (ledger `hide` reason `hidden`) | as `settled` |
| `off` | `oni.master=false` (ledger `exit` reason `oni-off`) | no `oni-surface` process, its scopes inactive, 0 commits, the daemon (which serves more than Oni and stays up) within the same CPU and wakeup budget |
| `strand-held` | a strand target landed and holding (its 30 s hold stop and the 30 s settled stop apply) | as `settled` |
| `strand-retracted` | the strand rewound and unmapped (ledger `hide` part `strand` reason `home`) | as `settled` |
| `perched` | perched on a window, the curl settled (ledger `perch`, no `detach` after) | as `settled`, and the daemon within the same CPU and wakeup budget |
| `detached` | after a `detach` | as `perched`, and no `niri`, `hyprctl`, `grim` or `ryoku` process under the daemon before or after the sample: the tracker is off |

The daemon's 90 s sleep counts from the last poke, and the settled stop takes
30 s, so `phase8.sh` samples `settled` for 45 s: a 60 s window ran into the
sleep gesture. An exactly zero daemon CPU delta is not reachable for a live Go
process (sysmon and the forced GC wake it a few times a minute).

The thresholds are the scoping report's proposals, recorded in the output as
proposed until the owner ratifies them; the real numbers are always kept.

## Phase 8 gate

`phase8.sh` starts `up.sh WM --app --surface` (Hyprland with `--outputs 2`)
and writes `OUTDIR/phase8.json` after every step:

| Step | What it proves |
|---|---|
| 0 census | `oni-surface --globals` (layer-shell, subcompositor, viewporter, fractional scale, foreign toplevel); the `hyakki-oni` layer surface where the compositor lists layers |
| 1 distinct states | `fixtures/states.jsonl` drives idle, listening, thinking, reading, speaking and error; each crop is background-subtracted against the hidden baseline; the four state accents differ in hue and are absent in idle; every pair of states differs in pixels |
| 2 gestures | `fixtures/gestures.jsonl` at `HYAKKI_ONI_TIMESCALE=0.1`: a mid-gesture crop for each of the ten, the pose moved and back within 0.05 of its base |
| 3 drag | `input.sh drag` from the character's centre to bare desktop (since Phase 10 a drop over a window perches, so the first of a fixed list of drop fractions whose end lies outside every placed window is used, and the drop must perch on nothing); the rect lands on the drop point, the saved fraction equals it within 1/extent, `oni-placement.json` exists, and the rect is identical after a surface restart and after a daemon restart |
| 4 reduced motion | `oni.reduceMotion=on`: a poke and a hover pass commit nothing without an event first |
| 5 display change | Hyprland: a drag toward the second output (a drag stays on its own output, so it drops at the edge, which the first output then remembers), `oni.place` onto the second, remove it, fly back (at least three distinct rects) to the remembered rect on the first; niri: scale 1.5 is an `output` event, keeps the fraction and flies there, back at 1 the rect is identical. niri's `winit` output cannot be turned off, so output removal is Hyprland's alone; `phase8.json` lists that under `limits` |
| 6 covered | fullscreen `hyakki-app` on the character's output: a courtesy `hide`, `measure.sh covered`, back after leaving fullscreen. When the character is on another output (Hyprland), first: it stays shown while the app's output is fullscreen, then `oni.place` puts it on the app's output |
| measurements | `motion`, `settled`, `hidden`, `asleep` (one real 90 s wait), `off`. Before `covered` and `motion` the harness calls `oni.interact` wake, then poke, so the daemon's 90 s sleep restarts and cannot fall inside the window |
| 7 display | every ledger line names the lab's display |

The crops, `globals.json`, every `measure-*.json`, the commands it ran
(`commands.log`) and the lab's logs with every surface ledger
(`logs/ledger/`) are kept in OUTDIR. A failed step
is recorded and the run goes on; the exit status is 0 only when everything
passed and `down.sh` found nothing left.

## Phase 9 gate

`phase9.sh` starts the same lab and plays one `fixtures/strand-*.jsonl` per
step through the surface's fake daemon (the real daemon has no session to
send targets; its handshake is the daemon's own socket test). It writes
`OUTDIR/phase9.json` after every step. The surface ledger logs strand frames
as `commit` lines with `part: "strand"` and
`strand: {phase, u, tip, seq, bbox}` (output-local logical px), the strand's
unmap as `hide` with `part: "strand"`, and `strand-target`, `strand-arrive`,
`strand-skip`, `strand-retract`, `bubble` and `sound` events.

| Step | What it proves |
|---|---|
| 0 census | `--globals`; strand pixels far above the `hyakki-oni` layer surface, the only one listed, which stays at the sprite and while the strand is out reaches the output's bottom-right corner (Hyprland squeezes a subsurface past its layer surface's right or bottom edge; niri lists no layer geometry); at the strand's root the pixel is the character's, not the strand's core (the strand is under the sprite); crossing and clicking the strand puts no pointer, drag, drop or gesture in the ledger |
| 1 reach, tap, hold | `HYAKKI_ONI_TIMESCALE=0.1`; at t = 0, 0.3, 0.74 and 0.92 s after the target the farthest strand pixel, less half the ink width, lies within 2 px of the ledger's tip track during the capture; one arrival at 0.92 s; the ledger replays through the strand engine (`strand.test`, `TestLedgerReplay`) |
| 2 retarget | A then B mid-reach: no home or rewind phase between, `u` never below max(`reachFrom`, its value at the retarget), the swinging tip stays out, the swing takes 300 ms, only B arrives |
| 3 rewind | after the retract, the crop equals the pre-target baseline outside the sprite, the strand unmaps (`home`) after 0.52 s, no strand commit follows, the whole run replays |
| 4 perch | a perch frame with the `hyakki-app` window's frame: curl frames 18 px from the window's top corner nearer the character |
| 5 lasso | ring crops thrown and cinched: 7 px wider thrown, on the two sides away from the lead line |
| 6 region pen | character carried at scale 0.55 with its centre `hypot(26, 8)` px from the tip |
| 7 bubble | four bubble rects (centre, top-right corner, left edge, bottom edge) on the output and clear of their targets; a bubble with no strand is not placed. The surface draws no bubble text yet, so the ledger rect is the evidence |
| 8 arrival | `HYAKKI_ONI_ARRIVALS`: seq 6 once and advanced; the superseded 5 and stale 4 never; a target covered by a fullscreen `hyakki-app` reported once as skipped `covered`; nothing for a target cut by EOF |
| 9 reduced motion | `reduce: on`: one strand commit per target (point, lasso, region), arrival in the same step, at most one per retract and then the unmap |
| 10 settlement | `measure.sh strand-held` after the holds stop, `strand-retracted` after the rewind |
| 11 tier | `HYAKKI_ONI_POWER=saver`: breathing commits at 30 fps, strand commits at 60 fps |
| 12 audio | `audio.sh up` and `record`; the fixture turns sounds on, sends targets, a bubble, retracts, a perch, voice frames and an EOF. From the WAV: every played sound found, the reach starting its 0.36 s delay (at most 80 ms more) after its event against the arrive after it, notes at the motif's spacing within 5 ms and at their MIDI pitch within 50 cents, durations and peak as specified; silence before the setting, while the voice is listening, speaking or transcribing, and after EOF; mic-open brings a playing sound below half within 30 ms and to silence within 50 ms; one `pw-cat` per played sound and none otherwise, each with only the lab's runtime directory and known to the private graph; no device in the graph |
| 13 display | every ledger line names the lab's display |

Crops, the fixtures played, the arrivals files, ledger copies, the WAV and
the graph (`audio/`), `measure-*.json`, `commands.log` and `logs/` are kept in
OUTDIR. `strand.test` is `go test -c` of `internal/oni/strand`, found beside
the binaries in `HYAKKI_BIN` or at `ONI_LAB_STRAND_TEST`; without it the
replay checks fail.

## Phase 10 gate

`phase10.sh` starts `up.sh WM --app --surface --model` (Hyprland with
`--outputs 2`), builds the fixture client, copies the fixture pictures and
manifests to `OUTDIR/fixtures/`, and drives the real daemon, the surface and
the `hyakki oni perch|detach|ask` command line against the fixture windows.
A consent is answered on the asking connection only (the daemon drops a
pending picture when its asker leaves), so every ask goes through
`hyakki oni ask --json`, answering on stdin. A watcher records every
compositor, capture or input program the daemon starts, with its argv, for
step 12. It writes `OUTDIR/phase10.json` after every step:

| Step | What it proves |
|---|---|
| 0 census | every advertised global through `window.sh globals`: `ext_foreign_toplevel_list_v1`, `ext_image_copy_capture_manager_v1`, `ext_foreign_toplevel_image_capture_source_manager_v1`, `ext_output_image_capture_source_manager_v1`, `zwlr_screencopy_manager_v1`; the capture path the compositor offers; the surface binds layer-shell |
| 1 tracker | `notes` and `hyakki-app` (and `notes-tiled` on niri): a perch box within 1 px of the compositor's, with its id, app id and title; a window without a position (niri tiled) refused `no-geometry` with its reason and no `PerchFrame` |
| 2 drag, highlight, drop | a drop on the empty desktop perches nothing and the ledger has the refusal; a drag held over `notes`: the outline `highlight.padPx` outside the frame found at 90 % or more of the positions along each of its four straight runs (a changed pixel within 1.5 px of `padPx`, so the desktop behind does not matter), the window's own pixels untouched; the drop perches; the daemon names the app and title and the chip is drawn |
| 3 perch | the ledger's box is the compositor's; a top corner; the character seated by the perch rules (`perch.seat`) within 2 px; curl tips `strand.perch.radiusPx` from the corner; strand pixels at the corner mid-curl and settled |
| 4 follow | a short move: the new box, the character at the new seat within 50 ms of the perch line, no reach or arrival; a long move: the thread home, a quiet `strand-reperch`, the new seat; onto the second output (Hyprland); a focus change moves nothing until `perchFollowsFocus`, then the perch follows |
| 5 detach | drag-off (`dragged-off`), a click on the chip's detach square (`user`) and the CLI (the daemon's detached `PerchFrame`, which the ledger records as `daemon`): the crop, shot with `notes` focused, back to its baseline, no strand, chip or outline commit after; closing the window detaches and takes strand and chip down; Oni off and on leaves no perch |
| 6 refusals | no perch: `oni.inspect` and an ask refused; `inspect` off: refused with no consent asked; with consent always: the prompt names the path and "stays on this computer", the `ConsentFrame` carries the same destination and route, the preview is the picture's size; a no, the asker killed mid-question and the 120 s expiry: the fixture's request log stays empty |
| 7 ask | one request with one JPEG of exactly the window's size class (Save found where the window has it), no tools, the window's words fenced; the answer text; the route is the fixture on this computer; the point inside Save notes; the ledger's strand tip where `strand.tipAnchor` puts it for the model's box (within 1.5 px) and, like the drawn tip, within `tipGapPx + tipStopPx` of Save notes (a point reach stops short of its box); a question about the window: `[ANSWER]`, then words only and no second prompt under `session`; `always` asks again |
| 8 ladder | the full ladder and each path alone (`HYAKKI_ONI_CAPTURE_PATHS`; the surface is restarted with the daemon): a path runs itself or refuses, never another; none allowed refuses; the disclosure text differs per path; everything answered no |
| 9 masking | on `output-crop`, with the window against the output's top edge and its right edge near the output's, Oni first set down at the bottom right so the top-right perch seats it over the window: the bytes the model got are white in every overlay rect; no overlay line repeats the one before it |
| 10 injection | the fixture in `hostile` mode: one call, no tool offered, the window's words fenced, nothing but capture run under the daemon, no tool output in the answer |
| 11 sensitive | "Where do I type my password?": refused in code with `screenask.RefusalLine`, no point and no strand |
| 12 read-only | no input-executor name (`wtype`, `ydotool`, `uinput`, virtual pointer or keyboard) in any non-test source under `internal/oni`; `feature.test` (`TestPolicyAndProfilesStayApart`) and `window.test` (allowlist tests); every program the daemon ran whose argv the watcher read before it exited is on the allowlist (`niri msg --json windows|outputs|workspaces|event-stream`, `hyprctl -j clients|monitors|cursorpos|layers`, `ryoku wm state|caps|watch`, `grim` to stdout), the exited-unread ones counted by name, and none is an input program |
| 13 disk | no picture file and no inline picture (data URL, base64 JPEG or PNG) written in the lab during the run; `tmp/` empty |
| 14 reduced motion | `reduceMotion` on: at most one commit per part for the perch, the arrival at once, at most one strand commit for the detach |
| 15 settlement | `measure.sh perched` (30 s held) and `detached` |
| 16 display | every ledger line names the lab's display |

`phase10.json` lists the limits it met: niri's tiled windows have no
position, nested niri has one output, the daemon offers capture no toplevel
resolver (so the ladder starts at `output-crop`), a single working capture
path proves the refusal rather than a second rung, and with no OCR or
control list a password field is refused from the model's label after one
call. The model is the loopback fixture, so the run proves routing, consent,
capture, masking and pixels, not model quality; a hosted "goes to" route is
left to the unit tests, since the lab holds no provider key.

## Phase 11 gate

`phase11.sh` starts `up.sh WM --app --surface --model` (Hyprland with
`--outputs 2`), `audio.sh up` and `mic`, the chord FIFO (`voice.sh keys`),
speech rendered by `espeak-ng` into the lab, a tone-coded clip character
and the staged speech models, then drives the real daemon, surface and
`hyakki oni talk|dictate|voice-test` and `oni watch`. It writes
`OUTDIR/phase11.json` after every step:

| Step | What it proves |
|---|---|
| 0 graph | no device sink or source, the default source is the virtual microphone and the guard refuses doctored graphs; the surface reads the chord FIFO; the meter rises on speech, offers no monitor or device and writes nothing |
| 1 push to talk | AltGr and a letter open nothing and a tap is dropped; a held chord gives partials, a final equal to the fixture, one ask, the acknowledgement and the reply spoken and in the recorded PCM, capture only during the press, level frames bounded |
| 2 barge-in | the chord during the reply: listening at once; the barged reply, compared sample by sample with the uncut reply, fades within the mute ramp or is cut inside a silence, and never resumes |
| 3 echo | Oni's speech looped into the microphone on open mic is heard back, not asked |
| 4 wake | the phrase opens a session, a clock question is answered locally, non-wake speech leaves no trace anywhere, the session ceiling, a changed phrase, Hands untouched, disarm |
| 5 open mic | "yes" never answers the consent card; the 180 s ceiling |
| 6 hygiene | the 43-repeat WAV collapses; a hold past `maxHoldS` is cut |
| 7 dictation | exact, clean and polished into a text window; closing it mid-dictation types nothing anywhere (other window, clipboard, primary, files, frames, ledgers) |
| 8 polish | the lab model failing or too slow: clean, and it says so |
| 9 clip voice | the clock and timer lines decoded from recorded PCM |
| 10 sound gate | no motif plays while armed, listening, transcribing or speaking |
| 11 ladder | hotkey to listening, first partial, final, acknowledgement; engine RSS, unload; the first syllable without a warm microphone. Release to final is dated by the frame that ends transcription, since the daemon sends no new transcribing frame when the final equals the last partial, so it is never negative |
| 12 settlement | `measure.sh` over 60 s windows with voice on and idle and with voice off, gated; wake armed sampled for 30 s and recorded, not gated. A 30 s window can catch the Go runtime's forced GC (two minutes after the last one), which wakes most of the daemon's threads at once, so the gated windows are 60 s |
| 13 read-only | no input-executor name outside `internal/dictate/insert`; the boundary, insert and target tests; every `wtype` the daemon ran was `wtype -s 50 -- CHUNK` inside a dictation |

`audio/` (recordings), `dictation/` (what each text window received),
`frames.jsonl`, `measure-*.json`, `commands.log` and `logs/` sit beside
`phase11.json`. `feature.test`, `insert.test` and `target.test` (`go test
-c`) in `HYAKKI_BIN` run step 13. The speech models come from
`voicebench.sh stage` or `ONI_LAB_SPEECH_SOURCE`; `ONI_LAB_STT_MODEL` and
`ONI_LAB_STT_ENGINE` pick one catalogue entry instead of Hyakki's default
and fallback. A run takes about 25 minutes.

`phase11.json` lists the limits it met. Speech is `espeak-ng` rendered at
run time, so the run proves the pipeline, not recognition of human
speech. `espeak-ng`'s "hey oni" reaches Parakeet 0.6B v2 as other words,
so the lab arms two other phrases and leaves the default phrase to
`voice/wake`'s unit tests. The wake ceiling (an hour armed) rests on
`voice/wake`'s injected-clock tests and is not waited out. With wake
armed the daemon wakes about 33 times a second, because `pw-cat` hands it
its capture in 4096-byte blocks (128 ms of 16 kHz mono) and each block
costs about four thread wakeups; that cost is recorded, not gated. A motif
during transcribing is recorded as a limit when the phase ends before the
motif's event reaches the surface.

## Phase 12 gate

`phase12.sh` starts `up.sh WM --app --surface --model` (Hyprland with
`--outputs 2`), the Phase 11 graph, chord FIFO and speech model, renders
the guide's speech (`guidekit.py fixtures`), and drives the real daemon,
surface and `hyakki oni guide|keep|skip|keeps|pick` against `guidewin.py`
and `model.py`'s guide mode. It writes `OUTDIR/phase12.json` after every
step:

| Step | What it proves |
|---|---|
| 0 census | guide settings off by default; the Phase 11 audio graph, chord key source and meter; layer-shell bound; a capture path offered; `tesseract` with `eng` and GTK 4 present; no keeps file; nothing capturing or typing at rest; the guide window's place against its `state.json`; a grim capture's marker colours within 2 px of the recorded boxes; the text reader reads File and Dead |
| 1 consent first | a no, the asker killed and the 120 s expiry each send nothing; with disclosure `always` the next step's picture is asked about again |
| 2 workflow | one JPEG of the window's size per step, no tools, the window's words fenced; the strand lands on File (the ledger within 6 px, and in the pixels with the perch curl, sprite, chip and card masked out); a click, Next, step 2 read from the changed screen; Save writes `saved.json`; the closing line; rising frame seqs; step 2 starts only after step 1's strand arrived |
| 3 unchanged screen | Next on Dead re-points the step with the again kicker or reads it again, then explains, with the strand and card still out; after `circle`, Next waits for the new strand, not the old one |
| 4 keep | the offer lapses after 12 s (the guide leaves keeping for its closing line) and writes nothing, and a late keep is refused; `oni keep` writes one entry, 0600 in a 0700 folder, with no title, the typed PIN, window text or pixels |
| 5 replay | zero model requests; no consent even with disclosure `always`; the replay kicker; lands on File then Save; the replay's closing line; latency recorded, with a limit over 1.5 s a step |
| 6 Save vs Save As | `--variant saveas`: no strand before the fall-through's model call, which asks consent; one miss recorded, the second retires the entry |
| 7 sensitive | new path: a PIN goal refused with the private-field line and no strand; replay path: the control relabelled PIN refused and the entry retired, with no request |
| 8 unwind | Stop mid-read (`slow`), detach, window close, Oni off, the presenter lost, the 12-step ceiling (`endless`: exactly 12 model reads, one a step, so a step read again after a Next that changed nothing fails it), the read timeout through `HYAKKI_ONI_GUIDE_READ_TIMEOUT_MS=5000`, and a stale consent answered yes |
| 9 voice first | a press starts the guide; open-mic "yes" does not answer its consent and a press's does; wake and open-mic "next" do not advance; a press's "next" and "stop" do |
| 10 select-and-ask | one pick surface per output (both on Hyprland); with disclosure `always` each pick's picture is asked about on the waiting `hyakki oni pick`'s own stdin, which answers it; Export answered in the bubble and in the CLI's result; the walk-through offer; a pick on the second output; the PIN refused before any model request; the repeat guard (the second pick ends with nothing sent); a CLI cancel unmaps (the surface's ledger names it `daemon`, its own `cancel` being the right click) and the waiting pick ends with nothing sent |
| 11 hints | off shows nothing; on gives one hint; Skip (Not now) puts it away and, past the cooldown, still holds for the app that day; on a new day a hint again, which retracts untouched at 45 s; past the cooldown the other kept goal's hint; past it again no third, each walkthrough once a day. The lab's hint record stands in for the time: dropping its last-hint time is the cooldown passing, removing it a new day |
| 12 injection | the hostile plan's canary never reaches a later request; no PIN in the plan; the action-tagged select answer is refused; nothing runs outside the allowlist |
| 13 read-only | no input-executor name, guide, keep, ocr and settle included; `feature.test`, `window.test`, `ocr.test`, `keep.test`, `guide.test`; every daemon child on its allowlist, with `tesseract`'s argv exactly `ocr.Allowed`'s; no picture or window text in the lab; `tmp/` empty |
| 14 reduced motion | at most one card commit per guide frame; arrival within 0.05 s |
| 15 settlement | `measure.sh perched` as `guide-waiting` and `guide-ended`, with no capture running, and at most `maxFrames + 1` captures after Next; on Hyprland the floating perch's sampler is measured first with no guide (`perched-floating`, recorded) and the daemon's wakeups in both guide rows count over it |
| 16 display | every ledger line names the lab's display |

The harness reads the guide's state carefully in a few places:

- **Voice fixtures:** "Next." and "Stop." came through the lab's graph as
  "Yeah." and "So", so the guide's voice fixtures say "Okay, next step." and
  "Stop talking.".
- **Step 13's allowlist:** it admits the daemon's two `pactl` reads,
  `pactl -f json list sources` and `pactl get-default-source`, as
  voice/audio's allowlist does.
- **File scans:** they take a WAV (RIFF) for no picture, and they count a
  typed number only standing alone, not inside a longer one such as a float
  in a ledger line.
- **Closing lines:** a closing card holds its line for 10–60 s, so the
  checks wait for the idle frame. "Oni off" reads the snapshot.
- **The done line:** it is the model's own closing sentence
  (`model.GUIDE_DONE`).
- **Step 10's picks:** each one runs `hyakki oni pick --json` in the
  background with its answer on stdin. The command waits after arming,
  answers its click's consent from stdin as `oni ask` does, and its exit
  code, result and consent prompt are read once it ends. "Walk me through
  it" is pressed with `hyakki oni guide next`, which answers the walk's
  own consent: the pick's command has exited by then.
- **Daemon children:** Go starts a child with `CLONE_VM|CLONE_VFORK`, so
  until its `execve` the child's `/proc` cmdline, comm and RSS are the
  daemon's own (`native-build/oni`, no arguments). The child watch reads a
  child once it shows its own image; one that exits first counts as
  exited unread.
  It records the child's executable before reading the parent's metadata,
  so a short-lived helper cannot lose that evidence merely by exiting during
  the parent reads. Missing executable identity still fails the rewritten
  Chromium-title check; a plausible command line is not a substitute.
- **Evidence per run:** every walkthrough, the ceiling run and every pick
  leave their slice of the surface ledger in `ledgers/NAME.jsonl` and each
  model request's picture as the model got it in `model/NAME-I.jpg`, so a
  step's masks, card place and landing can be judged afterwards.

`guidewin/` (each guide window's `state.json` and `saved.json`),
`answers/`, `crops/`, `ledgers/`, `model/`, `audio/`, `frames.jsonl`,
`measure-*.json`, `commands.log` and `logs/` sit beside `phase12.json`.
`feature.test`, `window.test`, `ocr.test`, `keep.test` and `guide.test`
(`go test -c` of `internal/oni/<pkg>`, except `window.test` from the canonical
`ryoku/wm/oniwindow` module) in `HYAKKI_BIN` run step 13. A run
takes about 40 minutes. The model is the loopback fixture: the run proves
routing, consent, capture, text-reader trust, strand landing and
model-free replay, not model quality.

`phase12.json` lists the limits it met. A click is not observed (no
pointer reader, no click guess), so the lab clicks and then presses Next,
as the card says. The card has no Keep pill: keeping is the app's, the
CLI's or a press's. A layer surface without keyboard focus never hears
Esc; the card's Stop, a spoken stop, the CLI and detaching wind a guide
back. Speech is `espeak-ng`, proving the routing, not recognition. On the
new path a step's label comes from the model's reply, so the PIN is
refused after one model request; the refusal before any model call is
select-and-ask's. Nested niri tiles windows and has one output, so the
second-output pick is Hyprland's. The repeat guard is a limit when the lab
cannot land a second pick within 1.2 s, resting on the daemon's tests. The
20-minute hint cooldown and the calendar day are not waited out: the lab
edits its own hint record to stand in for them, and their timing rests on
the daemon's and keep's tests. The pick overlay sets
no cursor shape. A settle capture shorter than the 50 ms watch period can
be missed, so the settle count is a lower bound held to the spec's upper
bound. Replay over 1.5 s a step is recorded, not gated. When the daemon
serves no read-timeout hook, the 75 s timeout rests on the guide and
daemon tests. Hyprland publishes no move event, so while Oni is awake a
floating perch is sampled with backoff to every 2 s (Phase 10's limit,
about 11 daemon wakeups a second); the guide window floats, so step 15
measures that sampler with no guide in the same run and gates the guide's
own daemon wakeups over it, recording the sampler's cost as a limit.

## Phase 13 gate

`phase13.sh` starts `up.sh WM --app --surface --model --decisions`
(Hyprland with `--outputs 2`), the Phase 11 graph, chord FIFO and speech
model, the launcher catalogue, the MPRIS player and the notification
server, and drives the real daemon, surface and `hyakki oni
reminders|routines|reflex log|keep|skip`, `hyakki routines wake`, `hyakki
service routines`, `hyakki memory list` and `hyakki off` by voice against
`model.py` and its decision mode. It writes `OUTDIR/phase13.json` after
every step:

| Step | What it proves |
|---|---|
| 0 census | the tools, globals and stubs (`systemd-run` included), the catalogue and the bus fixtures are present; reflex and its four switches are off and there is no reminders or routines file; the decision route is seeded local on loopback; the spec's reflex numbers; no new unit file |
| 1 off is zero | with reflex off a command is an ordinary Ask: one model request, nothing launched, no ledger row, no file; the five reflex rows are denied |
| 2 launch | a press opens the notes window once, with one acknowledgement of at most five words and one `done` row; the argv is the catalogue's, run bare (`scope=fallback`), and with the user manager's marker the refused `systemd-run` is followed by the same argv bare; the app keeps the lab's display and bus and outlives `oni`'s TERM; on open mic only "Oni, …" acts |
| 3 tabs | a new tab goes through the browser's own launcher (`about:blank`) and the tab count rises by one; "next tab" and "close the tab" say "Can't do that here." and change nothing; no input program runs |
| 4 window verbs | only honest verbs work: on niri `maximize-column` widens the tiled notes column; snap without the Ryoku seam, every Hyprland verb and minimize say "I can't do that here." with an `unsupported` row |
| 5 audio | on the lab sink the volume moves in 5 % steps, at 100 % Oni says it is already full, mute and unmute work; "pause the music" is one MPRIS `PlayPause`; no device joins the graph; a hidden Oni posts a notification |
| 6 timers and reminders | with reflex off a five-second timer rings by voice alone (bubble frame, ledger, pixels and speech in the recorded PCM); a reminder needs its switch, then rings with its words; a bare "remind me" is an Ask; list, cancel and the eight-day refusal; one due while the daemon was down rings once, marked late, then is done; the file is 0600 with no transcript |
| 7 zero at rest | with the wake unit refused the daemon stays up while a reminder is pending and reports `noWake`; with the accepting stub the enable and disable are logged and the daemon idles out with a reminder 40 minutes off; `hyakki routines wake` starts it only within the horizon, never with reflex or Hyakki off; the runit worker uses the same command; `hyakki off` reaches only the stubs |
| 8 storm | the 43-repeat utterance launches at most once; ten clauses run at most six actions; the per-minute cap; a second launch inside 4 s is a `cooldown` row; a dangling "open" completes within 6 s and not after; "I insert a table" opens nothing; "salt and pepper" is one whole search |
| 9 forbidden | send, buy, delete and submit each become an ordinary Ask with no action, launch or input program; `reflex.test`'s registry invariant |
| 10 routines | three commands within 20 s offer once per run; open-mic and wake "yes" and the expiry save nothing; a pressed "yes" saves (0600, at most 20 × 8, no words); "run my routine" replays with zero model and decision requests; a removed app is skipped with its line |
| 11 notes | with memory on the note is in `hyakki memory list`; with memory off Oni says "Memory is off."; a password is refused before memory and is in no file; no notes file exists |
| 12 decisions, local | a phrase the rules miss is classified by the local route and run; `unsure`, `fail` and `slow` fall through to the Ask within the budget; `hostile` runs nothing; requests hold only the utterance and the registry's labels, on loopback |
| 13 decisions, hosted | with no account the Ember route and automatic with `allowHosted` make zero hosted decisions and show no card |
| 14 self-commands | move, bigger, home and sounds off change placement and settings; turning on open mic, wake or hands is refused and the settings file is unchanged; wake and open mic never flip a setting |
| 15 read-only | no input-executor name under `internal/oni` or `internal/reflex`; no third-party name in the quick-command copy; `feature.test`, `reflex.test`, `remind.test`, `routine.test` and `act.test`; every daemon child on its allowlist and every launched app through the lab's launcher; Hands off: `hands.browser` guarded and denied, `hands.app` and `hands.desktop` unavailable, no hands frame |
| 16 disk | no audio, picture, transcript or refused-secret file; the stores are 0600 and hold only their own words; `tmp/` empty |
| 17 reduced | a reminder's bubble commits at most once per part |
| 18 settlement | `measure.sh settled` in three states: reflex on with nothing pending, one reminder pending (the daemon resident), reflex off |
| 19 display | every ledger line names the lab's display, and launched apps got it |

`audio/`, `crops/`, `model/` (the chat and decision requests),
`fixtures/` (`tabs.json`, `tabs-requests.jsonl`, `mpris.jsonl`,
`notify.jsonl`, `wake.log`, `refused.log`, `model-decisions.jsonl`),
`frames.jsonl`, `measure-*.json`, `commands.log` and `logs/` sit beside
`phase13.json`. `feature.test`, `reflex.test`, `remind.test`,
`routine.test` and `act.test` (`go test -c` of
`internal/oni/{feature,reflex,remind,routine}` and `internal/reflex/act`)
in `HYAKKI_BIN` run steps 9 and 15. A run takes about 45 minutes. The
model and the decision fixture are loopback fixtures: the run proves
routing, thresholds, fall-through and refusal, not model quality.

`phase13.json` lists the limits it met. Speech recognition may miss "Oni"
on open mic; the addressed branch then rests on the route tests. Snap
needs the Ryoku seam, which the lab's stub withholds: niri offers maximize
only, Hyprland offers no verb, and minimize works nowhere. The sink, the
tabs and the player are fixtures, not real devices or apps. The wake unit
is exercised only through the Ryoku bridge's systemd form, since the host
has `/run/systemd/system`; the bare `systemctl` form and the runit bridge
rest on the teams unit tests. `decidefix.py` is a keyword table. Hosted
account binding, credits and the per-minute cap are proven by the daemon
wire tests, since the lab has no account. The daemon's count of armed
timers is a daemon test; the lab measures the resident daemon's CPU and
wakeups. A launched app's survival inside its own systemd scope is not
proven in the lab, where every launch falls back.

## Phase 14 gate

`phase14.sh` starts `up.sh WM --app --surface --model --hands` (Hyprland
with `--outputs 2`), the Phase 11 graph, chord FIFO and speech model, and
the Esc stop FIFO (`keys/stop.fifo`). The daemon reads that FIFO through
`HYAKKI_ONI_STOP_FIFO` under `HYAKKI_ONI_LAB=1`. It is its own pipe: two
readers on the chord's FIFO would split its events between them.
`phase14.sh daemon start` also sets `HYAKKI_ONI_HANDS_START_URL` to
origin A's `/contact`, so the Hands browser opens on the fixture page.
`up.sh --hands` starts `handsfix.py`: two loopback origins, A and B, on
random ports. Their addresses go into `lab.env` as `LAB_HANDS_A` and
`LAB_HANDS_B`. Both compositors float the Hands browser by window rule
(app id `chromium` or `hyakki-hands-<hex>`). On niri it sits on the right
half, and `phase14.sh layout` narrows the app's column to the left half.
On Hyprland it sits on the second output. The gate plays the user's part:

- It types addresses into the browser and presses Send, Delete draft and
  the app's buttons with `input.sh`.
- It finds what to press with tesseract in grim pictures of the lab's own
  windows.
- It presses the pill's and card's buttons at the rectangles the surface
  ledger gives.
- It opens Hands once for each new task, then preserves the page and its
  goal draft while finding Start and answering the disclosure.

The native reader walks only the authorized main frame's accessibility
tree, in its child order, without walking inline text boxes. Plain rendered
text leaves use a bounded, compiled function in the isolated world instead
of serializing their whole accessibility layout. A read-only node header
keeps closed shadow hosts on the accessibility path, along with mixed
content, controls, generated content and protected text. The existing
three-second read deadline and control, name and text limits stay unchanged.
From `ryoku/hyakki/`, `go test -tags handsengine ./internal/hands/page` exercises real
headless Chromium, including large text, many controls, Unicode, mixed and
generated content, closed shadow controls and rendered text transforms.

Refusals made before an executor is called still enter the code-only Hands
activity log, without the goal, field names or typed text. The fixture
keeps its strict Origin check: genuine same-origin browser form posts work,
while missing, opaque and cross-origin submissions remain refused.

On GTK-equipped Chromium, its SVG image loader can start through two
Bubblewrap processes. The child scan admits only the observed
`glycin-svg` sandbox shape: private runtime and cache, read-only system
mounts, namespace and seccomp flags, matching D-Bus descriptors, physical
executables and every retained parent link back to the allowed browser.
A missing ancestor, another command or mount, or a foreign runtime is
refused. This is not a general Bubblewrap or image-loader exception.

Hands itself never runs an input program. The gate writes
`OUTDIR/phase14.json` after every step:

| Step | What it proves |
|---|---|
| 0 census | the tools, globals and stubs; both fixtures' selftests; `oni.hands` off, `hands.browser` guarded and denied, `hands.app` and `hands.desktop` unavailable; no profile, no Chromium |
| 1 refusals | hands off or inspect off: `open` refused, no browser; the CLI's settings patch of `oni.hands` and its raw authorize, start and resume are forbidden; the CLI's own authorize, start, resume and forget verbs exit 2; "turn on hands" by chord, wake and open mic changes nothing; the app's switch turns it on |
| 2 launch | the exact allowlisted argv (real binary, the lab profile, `--remote-debugging-pipe`, a fresh `hyakki-hands-<hex>` class, origin A's `/contact`); no `--remote-debugging-port` and no listening socket; the window told apart by PID or app id (recorded); profile 0700 with no extension; a second `open` reuses the browser |
| 3 authorize | before authorizing, a Press with an anchor starts nothing; `tabs` lists origin A and tab B; the app's Authorize binds origin A |
| 4 fill | Start from the app, its `vision.inspect` step disclosure allowed there: Name and Message filled in tab A only (the page's own `/state` reports), fill rows with no words, glow and pill while working, the `hands-start` motif |
| 5 send | Send is a handoff card (`hands.browser`, firm, local, `h-<n>`) and phase `waiting`; a spoken yes by chord, wake or open mic gets "Use the button for that." and nothing acts; the CLI's consent answer and a forged resume are refused; `/submit` stays 0 until the lab presses Send, then lands once with a `user` row and a done line naming the result |
| 6 delete | Delete draft the same way: Not now, an unrelated Press and the card's expiry each end the task with `/delete` at 0; the lab's own press lands once |
| 7 sensitive | the password and card fields are refused before any input (`private-field`), empty in `/state`, the canary nowhere; Pay now is a handoff and `/pay` stays 0 |
| 8 away | a foreign window's focus, a tab switch, another origin, a popup, a dialog and a download each pause with their word and no step after; the CLI and spoken "keep going" never resume, the pill's Resume does; closing the tab stops the task; a pause ends at `hands.resumeS` |
| 9 stop | during a slow model turn, Esc (the stop FIFO), the pill, the CLI, open-mic "stop", inspect off and a killed presenter each give the inactive frame with the glow taken down in the same handler, an aborted model request, a stop row with its reason within `hands.stopBudgetMs` and no step after; stop twice is harmless; the browser stays open |
| 10 protocol | two actions, an invented tool, a plan, a bad permission, bad JSON (retried once) and a filler turn run nothing; the hostile page's order stays inside the fence; repeat halts at the limit; endless ends at the budget |
| 11 bounds | 10,000 controls and a 5 MiB text node: the control list and the prompt stay within `hands.page` |
| 12 boundaries | no input-executor name or `Input.dispatch` under `internal/oni`, no input-tool name in `internal/hands` outside `internal/hands/input`; the unit test binaries; every daemon child is the browser's exact argv, its own helpers or the voice pipeline; tab B saw no input |
| 13 disk | no picture, typed word or goal in any lab file, the browser profile and the fixtures' own logs aside; the ledger holds codes only; `tmp/` empty |
| 14 residency | the daemon stays up past its idle exit while the browser is open and exits once it closes; `hyakki off` and a killed `oni` leave no Chromium; the app's Forget deletes the profile and nothing beside it |
| 15 settlement | `measure.sh settled` authorized with no task, paused (no glow tick), after stop (no glow surface) and with hands off; the glow while a card waits is at most half strength and no faster than its fps |
| 16 reduced | one glow commit per output; stop still takes the glow down in one handler |
| 17 display | every ledger line names the lab's display |

The completion line must actually reach the reader: the gate captures the
app's Last row and matches its rendered text, or observes the spoken or bubble
frame. A CLI status field alone does not satisfy that check.

`crops/` (the pictures the gate read), `model/`, `fixtures/`
(`hands-requests.jsonl`, `hands.log`, `refused.log`, `summary.json`),
`frames.jsonl`, `measure-*.json`, `commands.log` and `logs/` sit beside
`phase14.json`. `handsfix-selftest.json` and `model-selftest.json` retain each
census self-test's exact stdout, stderr and exit code, including failed runs.
The gate runs `go test -c` binaries from `HYAKKI_BIN` in
step 12: `feature.test`, `act.test`, `consent.test`, `proto.test`,
`loop.test`, `cdp.test`, `chrome.test` and `page.test`. They come from:

- `internal/oni/feature` and `internal/reflex/act`;
- `internal/oni/hands/{consent,proto,loop}`;
- `internal/hands/{cdp,chrome,page}`.

A run takes about 75 minutes.

`phase14.json` lists the limits it met:

- The model and the pages are loopback fixtures. The run proves the
  contract, the handoff rule, consent and the stop paths, not model
  quality.
- The browser is the host's Chromium. Firefox, Flatpak and snap are
  refused only in `internal/hands/chrome`'s tests.
- The browser's own profile is outside the scrub claim. So are the
  singleton and shared-memory entries Chromium keeps in the host's `/tmp`
  or `/dev/shm`.
- The glow's strips across two outputs are proven on Hyprland only.

## Phase 15 gate

`phase15.sh niri OUTDIR` and `phase15.sh hyprland OUTDIR` exercise the
picked-window and separate-desktop Hands stages. Hyprland uses two outputs,
with the app on the first and the target and sentinel on the second.
The separate desktop is its own niri-on-weston child, not another view of
the person's session.

Its managed windows open full-size in the floating layout so the compositor
reports their positions. The existing window-box, overlap, focus and scope
checks still decide every click; an unplaced window never gets guessed
coordinates.

The gate uses the real app to switch stages, pick a window, enter a goal
and answer picture disclosure. After returning to the intended foreground,
an already-paused self check is resumed through the actual pill button.
CLI and voice resume remain refusals, and later mid-task theft or self
checks are not resumed by that approval helper.

After a restart, the app stage is selected before opening its window picker.
The key-pause privacy check compares the actual working frame with the paused
frame, preserving existing app and output context while refusing new key data.
The target-close case does not resume a focus-paused task before closing its
window: closure must end it in that state. The daemon's socket regression also
covers closing the target while a handoff waits for the person.
The queue check observes that its waiting task does not start after stop, then
uses the app's real Delete all confirmation to clear only the lab journal
before the independent model-error case.
The following protocol cases use the existing provider-rest readback before
starting another task, so the model-error case cannot hide their real requests
behind the provider's unchanged cooldown.

Shutdown checks that require the runtime directory to disappear wait for both
process exit and directory removal within the original grace period. Process
exit alone can precede the final removal. Forced-kill checks deliberately wait
only for process exit, then verify the next daemon's sweep removes the residue.

Window-theft checks round compositor positions at the fixture's move-command
boundary. Compositors may report fractional logical coordinates; the fixture
accepts whole logical pixels. Its integer parser and the original geometry
pause deadline remain unchanged.

The UI driver reads exact visible placeholder or retained-draft anchors
from the captured field. A rejected start may leave the draft in place,
and a text cursor may spoil an adjacent OCR word; neither is a reason to
invent a field box or change the reader's confidence checks.
An exact interior of the person's previous draft can identify the field when
horizontal scrolling clips its first word; model text is never an anchor.

An interruption waits for an unfinished model request, or changed target
text and a live input child on the exact private display when testing a
typing stop. The fixture delays and the original interruption deadlines
are not stretched.

The model fixture advances only on completed actions or a completed file
search, not on pause notices, refusals or failed read-back. It retains that
observed progress when the daemon's bounded history rolls, without changing
the history or task budgets. Step 5 establishes the app stage and picker
before step 6 uses them. Selected runs must include their prerequisites and
are diagnostics, never full acceptance.

Input provenance resolves the child's runtime and display to the exact
private compositor socket. An absolute and a runtime-relative spelling
may identify the same socket; a foreign socket with the same basename
does not.

For an awake-idle sample, the wake animation settles before a wake
interaction refreshes the existing idle timer. Waking an already-awake
character adds no animation. The original quiet and sampling windows
remain unchanged, and a sleep transition inside the sample still fails.

`phase15.json` records every stage, check and limit beside the actual input
ledger, model requests, task journal, screenshots and process inventory.
Workspace startup, task end, Stop, daemon TERM and KILL/next-boot sweep are
separate checks, and omitted stages remain failed.

## Phase 16 gate

`phase16.sh niri OUTDIR` and `phase16.sh hyprland OUTDIR` exercise memory,
import, recall, selected-text rewrite, focus guard and assignment. Set
`HYAKKI_BIN` to an absolute frozen build directory containing the four
programs and the feature-test binaries. Each run needs a fresh output
directory outside the repository and lab. Two concurrent runs need distinct
`ONI_LAB_ID` values as described above.

The driver writes `phase16.json` after each step. Exit zero requires every
step; a focused diagnostic or a skipped step cannot qualify the phase.

| Step | What it proves |
|---|---|
| 0 census | tools, feature rows, private microphone and GTK target readiness |
| 1 caller boundaries | off stays off; app-only choices cannot be granted by CLI or speech |
| 2 sensitive memory | card and identity numbers are refused before storage |
| 3 spoken memory | real remember, correction and selective forget with their approval rules |
| 4 imports | owned chat import, provenance, removal and index cleanup |
| 5 recall | local retrieval and explicit disclosure before hosted passages could leave |
| 6 rewrite | private PRIMARY, exact review and paste-back; mandatory exact one-word undo and exact paragraph restoration or truthful bounded fallback |
| 7 target refusals | missing selection, private or changed target and stale approval cause no paste |
| 8 voice rewrite | speech prepares disclosure, never supplies approval or paste |
| 9 focus guard | only the authorized browser tab can close and reopen; whole windows remain announcement-only |
| 10 assignment | two observed lanes, bounded ledger, critical replacements on their own text lines, cached view and unrelated-clock suppression |
| 11 boundaries | frozen behavioral packages and strict provenance for every retained child |
| 12 disk | private selection, replacement and rejected data remain outside product stores and frames |
| 13 unarmed | optional work is disabled and the real idle policy exits without a later wake |

The GTK fixture establishes its paragraph selection with three genuine
pointer clicks in its observed text view. Keyboard focus alone, or selection
at focus time, can make GTK's local provider readable without publishing
PRIMARY to the compositor; transient virtual keymaps can also deliver a
different key than the sender requested. No clipboard writer or injected
selection payload substitutes for an actual private `wl-paste` observation.
The fixture has a compositor-reported floating rectangle beside the app,
not under it; native capture still refuses any actual occlusion. The readable
one-word undo case must restore exactly; a failed paragraph undo must return
the complete original honestly, not claim that the widget was restored.

The local picture reader gives small edge text an integer-scaled recognition
image and a background border, still inside the original pixel limit. It
keeps the confidence floor and fixed reader command, returns boxes in the
original window's coordinates and clears its scratch buffers. The picture
shown for consent or sent to a model is unchanged. Assignment compares
numeric literals and operators rather than OCR word groups; added answers
and one clock rollover still do not start another solve.

The normal gate uses the default speech engine on a private null-audio graph.
The deterministic loopback model proves protocol and policy, not model
quality; synthetic speech does not establish human recognition quality.
Containment scopes do not prove native init or package delivery.

## Phase 17 gate

`phase17.sh` starts a fresh private desktop and the real app, without seeding
Oni on. Give it the frozen programs and `protocol.test`, `daemon.test`,
`settings.test`, `shell.test` and `mcpserve.test` built from the same source:

```sh
HYAKKI_BIN=/absolute/frozen ONI_LAB_ID=n17 ONI_LAB=/dev/shm/oni-lab-n17 \
  ./phase17.sh niri /dev/shm/hyakki-settings-niri
HYAKKI_BIN=/absolute/frozen ONI_LAB_ID=h17 ONI_LAB=/dev/shm/oni-lab-h17 \
  ./phase17.sh hyprland /dev/shm/hyakki-settings-hyprland
```

All ten stages must pass: fresh defaults; skip and replay of the nine-card
tour; routes, Recall and persistence; the four styles and independent palette;
real stdio MCP approval and effects; activity and selective clearing; owned
optional parts and removal confirmation; fullscreen courtesy; compiled
consumer contracts; and off residency with the source and binary fence.
Keep the app, its dependencies and lab source unchanged for the whole run.

The tour types into its actual scratch editor and moves focus with Tab
before reading the exact visible word. A template's copy of that word does
not count. Settings switches use the complete visible row and real stored
value, not a guessed coordinate or a successful input command. Row titles
align with the next row; descriptions can repeat their labels. Oni is
dragged onto measured bare desktop when it covers the controls, rather than
being hidden or exempted from the occlusion check.

The stdio client gets no tools before app approval, and approval alone grants
none. The gate enables each tool separately, checks actual reminders and
suggested notes, reads the rendered notification and revokes permissions on
the existing connection. An idle approved pipe must not keep the daemon
resident. Selective clearing must leave unrelated records and suggestions
intact, including after app reopen.

Notification pixels are read inside the surface's committed pill rectangle
on the actual lab output. A message may wrap across lines; its complete
word sequence and sending program must come from one OCR reading, never
pieces of different readings or text elsewhere in the app.

`phase17.json`, the screenshots, compiled-test output, source hashes and
teardown record are evidence, not package or production approval. The final
stage observes natural idle exit and five minutes without reactivation; a
forced stop cannot stand in for it. No model, account, owner microphone,
speaker or live desktop is used. A selected diagnostic that omits stages is
not a complete gate.

## Traps these scripts handle

- **Many wheel notches:** some compositors forward one wheel tick for one
  virtual-pointer packet, even when its discrete count is larger. The input
  helper emits a separate packet for each whole notch, so a top-of-page
  scroll reaches the same place on both compositors. Fractional packets keep
  their original form.

- **A browser fork is not a launch:** a child can briefly inherit the root
  browser's exact argv before assuming its own role. Launch and reuse checks
  count only the observed direct children of Oni. Every retained descendant
  still goes through the full child-boundary audit.

- **Hidden `environ`:** `oni` marks itself non-dumpable, so its
  `/proc/PID/environ` cannot be read, even by its own user. A reaper that only
  reads `environ` leaves the daemon running; `down.sh` also matches the lab's
  scope cgroups, and the lab finds its processes by scope and name.
- **Live `systemctl` and Ryoku:** `oni` runs `systemctl --user` (for example to
  disable `oni-routines.timer`), and the client starts `oni.socket` through
  it; the app opens Ryoku's style page through `ryoku-shell`. Inside the lab,
  the `bin/` stubs refuse `systemctl`, `ryoku`, `ryoku-shell` and `ryoku-host`
  and log them, so nothing reaches the live user manager or desktop. A
  refused `systemctl` makes the client start `oni` itself, inside the lab.
- **Phase 13's frame watch starts the daemon.** `hyakki oni watch` starts
  the daemon when it is down, so the idle-exit and restart checks pause
  the watch (reflexkit's `PausableWatch`), or the daemon never stays down.
- **App-only settings need the daemon stopped.** `providers.*`, `memory`
  and `enabled` are the app's alone. The gate rewrites `hyakki.json` only
  while the daemon is stopped, because a running daemon writes its own
  copy over the file.
- **The refusal stub keeps the daemon up.** With `bin/ryoku-host` refusing,
  the wake unit cannot be armed, so the daemon stays up while a reminder is
  pending (`noWake`). Idle exit needs `phase13.sh wakestub accept`.
- **Without the marker, `systemd-run` never runs.** The lab's runtime
  directory has no `systemd/private`, so the launcher never asks
  `systemd-run` until `phase13.sh scope on` creates the marker, which it
  does only once `bin/systemd-run` is verified as the lab's stub, or a
  launch would reach the live user manager.
- **Launched apps are not in their own scope.** They sit in the daemon's
  lab scope as direct children of `oni`, and the gate ends them by pid once
  their scope shows them as the lab's.
- **Session bus side effects:** a private session bus would activate gvfsd and
  its FUSE mount for GIO clients. Lab apps run with `GIO_USE_VFS=local`.
  `down.sh` still unmounts anything left under the lab.
- **Hyprland nesting:**
  - Hyprland does not render nested under niri or weston, so tinywl hosts it.
  - On hybrid-GPU machines tinywl must use the integrated GPU's render node;
    otherwise the headless output is 0x0. `up.sh` refuses a 0x0 output.
  - `HYPRLAND_NO_SD_VARS=1` keeps Hyprland from rewriting the live systemd user
    environment.
  - `LIBSEAT_BACKEND=seatd` keeps it from taking control of the live logind
    session.
  - The instance is selected by its scope, never by "the newest instance".
  - Hyprland 0.56 takes Lua dispatchers (`hyprctl dispatch 'hl.dsp...'`);
    `ctl.sh fullscreen` tries those, then the classic names, and checks the
    result in `hyprctl clients`.
- **Socket length:** Hyprland's IPC path holds a long instance signature, so
  `ONI_LAB` stays short.
- **Weston idle:** weston blanks an idle headless output after 300 s, and nested
  niri then wedges; `--idle-time=0` prevents this.
- **Hyprland pointer focus:** Hyprland drops pointer focus when a virtual mouse
  disappears and re-picks the surface only on relative motion. Each `input.sh`
  call therefore uses one pointer for its whole step list and nudges 1 px after
  every absolute move.
- **Hyprland keyboard capability:** `--app` requires `wtype` and holds one
  idle virtual keyboard in an owned lab scope. Otherwise removing the last
  short-lived keyboard can lose keyboard focus between commands, even while
  the compositor reports the intended window focused. The holder sends no
  keys and teardown stops it. Its four-hour idle hold uses separate
  30-minute sleeps to avoid `wtype`'s 32-bit microsecond-conversion overflow.
  Readiness checks the actual device name in `hyprctl devices`: an empty
  keymap makes this compositor's JSON form emit bare `none` for the active
  layout index, which is not JSON.
- **Pointer extent:** absolute virtual-pointer motion spans the union of the
  outputs, so `lab.env` carries that union and `outputs.sh` rewrites it.
- **niri hot corner:** under niri, the top-left corner opens the overview. Do
  not park the pointer there.
- **Surface instance lock:** a second `oni-surface` exits at once while one
  holds the lock, so `ctl.sh surface restart` waits for the old process to be
  gone before starting the next.
- **A finalised WAV:** `parec` writes the WAV header's sizes only when it
  exits cleanly, so `audio.sh stop` sends SIGINT; a process backgrounded
  through a shell function is a subshell, so the recorder is found by its
  scope and name, never by `$!`.
- **A recording that keeps time:** a null sink suspends when idle and the
  monitor then delivers nothing, so `oni_lab` is created with
  `session.suspend-timeout-seconds=0` and `node.always-process=true`. Its
  clock still drifts from the ledger's by up to about 1 %, and each `pw-cat`
  starts a few tens of ms late, so step 12 finds each sound near the clock
  the sound before it left and checks delays against the next sound.
- **Sounds that replace each other:** a new sound replaces the one playing
  when its player starts, and its delay is leading silence in that stream.
  Since Phase 10 a perch flies the character to its seat, so the fixture's
  perch plays a skate, a reach under the flight, the land, the curl and the
  arrive inside two seconds. Step 12 lets each segment start belong to one
  sound, finds a sound inside another's run by its first note's rise, holds a
  replaced sound to pitch and spacing only for the notes that began 30 ms
  before the next player started, and reads note envelopes through a 30 ms
  window, which separates notes two semitones apart (reach 62, land 64).
- **The landing burst:** three short inked lines leave the tip when a target
  lands, so the farthest changed pixel is not the tip; step 1 follows the
  thread's white core instead.
- **The fake daemon replays:** after an `eof` the surface redials at once and
  the fake daemon plays the whole script again from its snapshot, which has
  sounds off; a step reads its evidence before the second pass reaches it.
- **niri window position:** `niri msg windows` gives a tiled window's size but
  no place; the lab's app window is the one column, one gap in from the
  output's corner, which step 4 records as its source.
- **Fixture placement:** niri and Hyprland both float the `notes` fixture
  at 1040,120, `list` at 1120,640 and Phase 12's guide window
  (`dev.ryoku.OniGuide`) at 1120,440 by window rule, clear of each other;
  `notes-tiled` matches no rule, so niri tiles it.
- **The app over the guide window:** on Hyprland the floated app window
  overlaps the guide window, so `guidekit.py` focuses the guide window
  before every click.
- **Consent on one connection:** the daemon drops a pending picture when the
  asking connection closes and refuses an answer from another, so a lab ask
  is always `hyakki oni ask` answering on its stdin, never two `hyakki
  call`s.
- **niri relative moves:** `niri msg action move-floating-window` reads a
  signed number as a relative move, so `window.sh move` passes output-local
  absolute values, which stay positive on the lab's outputs.
- **A barged reply that pauses:** `espeak-ng` leaves tens of ms of true
  silence between words, and an envelope check that reaches back across
  such a pause to the last loud sample reads a short fade as a long one.
  Phase 11 step 1 finds where each utterance starts (after at least 0.2 s
  of silence) and keeps the uncut reply; step 2 compares the barged reply
  with it sample by sample and reports an audible fade with its fitted
  length, a cut inside a silence, or a click. The check proves itself on
  synthetic 10 ms and 60 ms fades and a dead stop.
- **A chord source lost on restart:** restarting the daemon drops the
  surface's chord key source, so `voice.sh keys` restarts the surface
  after it.
- **Replay needs Keep:** a kept walkthrough replays only while
  `oni.guide.keep` is on, so Phase 12 turns it on before the replay steps.
- **The model's own log:** `model.py`'s request log holds the guide
  window's words by design, so step 13's scan for window text written in
  the lab excludes it.
