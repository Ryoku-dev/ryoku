# Ryostage (舞台)

The desktop as a stage: the wallpaper is the backdrop, the subject and any
extra cut-outs are the layers, and the clock, widgets and the audio
visualizer are the cast, arranged in front of or behind them. Depth and
Parallax used to be two features with two engines, two settings files, two
sidebar tabs and two artifact folders. They are one thing: **Depth is a
stage with a single still layer in front of the widgets; Parallax is the
same stage with motion on.** Ryostage is that one thing.

Names, so the parts are findable:

| Part | Name | Where |
|---|---|---|
| The feature and its sidebar card | **Stage** (`stage` left-sidebar card) | `quickshell/shell/modules/stage/`, `sidebar/cards/StageCard.qml` |
| The cut-out engine helper | **`ryostage`** | `ryoku/shell/scripts/ryostage`, shipped to `/usr/bin` |
| The daemon module | `stage` topic and verbs | `ryoku/shell/ipc/stage.go` |
| The settings | `~/.config/ryoku/stage.json` | user-owned, GUI-managed, never materialized |
| Per-wallpaper scene state | `~/.local/state/ryoku/stage-walls.json` | daemon-owned |
| Artifacts | `~/Pictures/Stage/<stem>/` | the user's files, one folder per wallpaper |
| Engine runtime and models | `~/.local/state/ryoku/ryostage/` | one venv, one model cache |

The two retired engine helpers (the old Depth and Parallax segmentation
scripts), `depth.json`, `parallax.json`, `depth-walls.json`, `~/Pictures/Depth`,
`~/Pictures/Parallax` and the `depth`/`parallax` tabs are retired; the doctor
migrates all of them (below).

## The mental model a user needs

One feature: **Depth**. It lifts the wallpaper's subject in front of your
widgets. **Parallax** is a switch inside Depth: the same cut, the same look,
now drifting with the pointer over an inpainted backdrop. Nothing is configured
twice.

Two places, each with one job:

- **The Stage tab** (Super+Esc, the last rail icon): every setting, one
  scrolling column, ordered by how often it is touched. Nothing here needs a
  Done; every change is live and shows its value.
- **The desktop** (right-click): the two switches, `Edit widgets` (arrange
  widgets in front of or behind the subject), `Customize visualizer`, and
  `Depth settings...`, which opens the Stage tab. Edit widgets is reached
  from the desktop only; the tab is settings, the desktop is arrangement.
  A widget's own menu (built-in, plugin tile) and the visualiser's edit bar
  each carry a `Depth` row that lifts just that widget above the in-front
  cut-outs or drops it behind them; the row shows only while the wallpaper
  has a subject cut.

There is no shell editor. The bar, dock and menus keep their Hub pages.

## The desktop right-click menu

```
Edit widgets
Customize visualizer
Change wallpaper
[Depth      (o)] [Parallax   ( )]     <- two switch cards, side by side
Depth settings...                     <- opens Super+Esc on the Stage tab
Settings
Reload shell
```

The switches are the menu's own choice chips (a bone plate while on, the state
in the label: `Depth, On`), under the rows and above `Depth settings...`.
Tapping either keeps the menu open so the effect is seen at once.

- Depth on: `set-effect depth`. Depth off: `set-effect off` (Parallax's switch
  falls with it).
- Parallax on: `set-effect parallax`, and Depth's switch turns on with it if it
  was off (one tap, no "enable Depth first"). Parallax off: `set-effect depth`.
- While the engine cuts (first enable on a wallpaper), the Depth card reads the
  daemon's percentage; the switch stays on.
- `Depth settings...` asks for the `sidebar-left#stage` surface: the left
  sidebar opens (or switches) to the Stage card on this monitor.

## The Stage card

`modules/sidebar/cards/StageCard.qml`, one card on the left sidebar's Stage
tab, built from the sidebar's own kit (`SidebarCardShell`, `Ryoku.Ui`
controls) plus the stage's preview card and angle dial. One column, one
Flickable, sections in the sidebar's eyebrow rhythm. Top to
bottom:

1. **Title** `Stage`.
2. **Preview**: the current wallpaper with its cut drawn over it; the ring
   while the engine runs.
3. **Two tiles**, side by side like Wi-Fi and Bluetooth on Home: `Depth`
   (sub: `Off`, `On`, or the percentage while cutting) and `Parallax` (sub:
   `Off`, `On`). The whole face toggles; the same rules as the menu switches.
Everything below appears only while Depth is on. Off, one quiet line takes its
place: `Turn on Depth to cut the subject out and shape it.` A first enable
cuts in the current tier (Draft by default: the small model, seconds), so the
first result is fast and quality is raised afterwards, with a confirm.

4. **Cut quality** (a section): `Draft | Standard | Fine` (a segment), a
   caption under it naming the tier's model, size and whether it is installed
   (`Fine: 224 MB, installed`). Choosing another tier changes nothing yet: the
   segment shows the choice and a confirm row appears under the caption:
   - model installed: `Re-cut in Fine` with `Re-cut` and `Cancel`;
   - model missing: `Fine needs a 224 MB download` with `Download` and
     `Cancel`; when the download lands the row becomes the Re-cut one;
   - while the engine runs: `Cutting in Fine, 40%` with `Stop`.
   `Re-cut` writes the tier and refreshes; `Cancel` (or leaving the card)
   drops the choice and the segment snaps back to the tier in use.
5. **Layers** (a section): one row per layer, the subject first. A row is
   the layer's name on the left and `Behind | In front` (a segment) on the right;
   an added layer also has a remove cross, and, while Parallax is on, a
   `Drift` slider (near to far) under it. Below the rows, two half-width
   buttons `Cut a picture...` and `Add a PNG...`, and a quiet `Clear cut-outs`
   link. `Cut a picture...` opens the picker, then shows `Cut from
   <name>` with `Cut` and `Cancel`. `Clear cut-outs` shows `Remove every
   cut-out for this wallpaper` with `Clear` and `Cancel`. `Add a PNG...` is
   immediate (nothing runs).
6. **Look** (a section): `Edge` (a slider, 0..1, value shown) and
   `Shadow` (a slider) with the angle dial at the row's end and the degrees
   under it. Both live. A quiet `Reset to defaults` link at the end of the
   section puts edge, shadow, angle and every motion knob back.
7. **Motion** (a section, Parallax only): `Preset` `Soft | Cinematic |
   Beat` (one tap sets amount, idle, speed and music; highlighted only while
   every knob still matches); `Amount` `Subtle | Normal | Strong`; `Idle`
   `Still | Float | Breathe | Sway` with a `Speed` slider while not still;
   `React to music` with an `Intensity` slider while on; `Follow mouse`; a
   `Fine-tune pointer` revealer holding `Sensitivity`, `Range` and `Backdrop
   drift` sliders (shown only while Follow mouse is on).

The confirm rows share one component: a message on the left, one or two text
buttons on the right, in the section's own width; nothing floats and nothing
covers another control. Escape closes the sidebar as it always did.

## Edit widgets

The desktop lifts above open windows, the dock steps back, and every enabled
widget wears a frame:

- a 1 px outline with the widget's name at its top-left;
- drag anywhere on it to move (grid-snapped, live), the bottom-right bracket to
  resize;
- two small buttons on its top-right: **Settings** (opens that widget's own
  menu: design, lock, size, opacity, colour, snap) and **Remove** (hides it).

Every framed widget gets one: the built-ins, the Shima and Python faces, and
the plugin tiles alike.

One toolbar rests bottom-centre, one row:

```
部品 EDIT WIDGETS  [grid] [16]  [Widgets]  [Reset]  [Done]
```

- **Widgets** grows a panel out of the bar's top edge. The roster is too long
  for one honest list, so the panel is a settings page: a category rail on the
  left (Ryoku widgets, Shima widgets, Python widgets, each installed plugin
  set, with a live count) and a two-column grid of widget cards for the chosen
  category. A card carries the glyph, name, hint, an on/off dot, and (once on)
  a tune affordance that opens that widget's editor -- the inspector for a
  slot-hosted face, the Placer for the visualizer, the tile's own menu for a
  plugin. The whole card toggles. Search drops into a flat result grid across
  every category, each card wearing its category as an eyebrow. Keyboard:
  Down from the search enters the grid, arrows move, Space toggles, Esc
  unwinds. Plugin rows read the installed set (enabled or not), so a hidden
  tile keeps its card and the switch brings it back.
- **Reset** restores widgets.json as it was when the session opened (every
  widget's enabled set, placement, size, style, colour and face options, plus
  the visualizer's flag and the placed plugin set); its slot is kept while
  clean so Done never moves.
- **Done** (or Escape, or a click on bare wallpaper when nothing is selected)
  leaves. There is no Save; the desktop is the document.

## Customize visualizer

The visualizer's own editor, unchanged: the Placer (drag to move, corner to
size, dot to turn, scroll to resize) with its EditBar fixed to a screen edge.
The menu row (and the picker's tune on the visualizer card) turns the
visualizer on if it is off and opens it. Its Done closes it.

## Session model

`modules/stage/Singletons/StageSession.qml` is the Edit widgets session only:
`mode` is `""` or `"widgets"`; `monitor` names the screen that opened it;
`selected` is a widget id; `panel` is the drop-down that is open (`"add"`);
`dirty` shows Reset. `escapeStep()` unwinds one level per press: the
drop-down, then the selection, then the session. The Stage tab keeps its own
pending confirm locally; it is a panel, not a session.

## Models: one catalogue, visible provenance

`ryostage` owns the curated list, and the UI renders it instead of hardcoding
tiers:

```
ryostage models --json
[
  {"id":"u2netp","label":"Draft","tier":"draft","size":"4.6 MB","installed":true,
   "licence":"Apache-2.0 (mirrored weights)","upstream":"https://github.com/xuebinqin/U-2-Net"},
  {"id":"birefnet-general-lite","label":"Fine","tier":"fine","size":"224 MB","installed":false,
   "licence":"MIT","upstream":"https://github.com/ZhengPeng7/BiRefNet"}
]
```

The Quality control maps Draft -> `u2netp`, Standard -> `u2netp` with alpha
matting, Fine -> `birefnet-general-lite` with matting. Picking a tier whose
model is not installed shows the size and a **Download** button in place;
`Remove` frees it again. The runtime (`rembg[cpu]`, MIT, on ONNX Runtime,
MIT) installs once, on the first enable, into the shared cache. Nothing ML
ships in the base image. Both scripts' licence notes live in the engine's
header and here, so a packager can check them.

## Engine: `ryostage`

One bash helper, the only place model logic lives. Backend resolution is
unchanged from the old Depth engine (the managed venv first, then a system Python in
rembg's range, `uv` provisioning a managed 3.13 otherwise).

| Subcommand | Contract |
|---|---|
| `check` | exit 0 and print `available` when the runtime and at least one model are present, else `missing` and non-zero |
| `models [--json]` | the curated catalogue: ids one per line, or the JSON above |
| `install [model...]` | provision the runtime and fetch the named models (default `u2netp`); opt-in, streams progress |
| `remove <model>` | drop a cached model |
| `cut <in> <out.png> [--model id] [--matting]` | the subject as an alpha-matted PNG; never writes a partial file |
| `inpaint <image> <mask> <out.png>` | fill the cut-out's hole with the surrounding colour (the parallax backdrop) |

Cache: `~/.local/state/ryoku/ryostage/{venv,models}`. On first run the
helper adopts a pre-split `~/.local/state/ryoku/depth` or `.../parallax` tree by
rename (same filesystem, no re-download); a leftover second tree is reported
by the doctor as reclaimable space.

`inpaint` makes the Parallax backdrop from the one cut: the subject's hole
(the matte grown outward so no subject pixel seeds the fill) is filled from its
surroundings by normalized convolution, growing inward until covered, then the
whole image is softened as the far plane (a 4 px blur, a touch darker). A
drifting subject therefore reveals the colours around it, never a flat plate
or its own silhouette. Half resolution above 1600 px keeps a 4K wallpaper at a
few seconds. Backdrops made before this land are regenerated by a re-cut or
Refresh.

## Daemon: `ipc/stage.go`

One worker, one registry (below), one topic.

- **Artifacts** `~/Pictures/Stage/<stem>/`: `subject.png` (the cut),
  `background.png` (the inpainted backdrop, made once the first time
  Parallax is chosen for that wallpaper), `layer-NN.png` (added layers),
  `.index.json` (mtime + quality reuse).
- **Topic** `stage`: `{ current, busy, stage: "cut"|"inpaint"|"", percent,
  walls: { <path>: { effect, subject, background, rev, layers: [...] } } }`,
  published on every change and on each generation phase. QML renders from it
  and nothing else. `subject`/`background` are absolute paths ("" until fresh);
  `rev` is the max mtime across the wall's `subject.png`/`background.png`/
  `layer-NN.png`, so the shell busts every url with the one revision. The frame
  layers carry `{out, label, enabled, front, depth}` and no per-layer rev, and
  `layers[0]` is always the subject slot.
- **Verbs** (`ryoku-shell stage ...`): `set-effect <off|depth|parallax>`,
  `set-layer <index> <json>` (enabled/front/depth), `add-layer <png>`,
  `cut-layer <picture>` (runs the engine on another picture and adds the
  result), `remove-layer <index>`, `refresh` (re-cut), `cancel`, `clear`,
  `status`, `models`.
- **Rules**: a wallpaper switch reconciles and never generates; a stage is
  per wallpaper; videos are skipped; an effect switch never re-cuts (only
  Parallax's first use on a wallpaper adds the inpaint); a failure leaves the
  effect off with a logged reason. The subject is still handed to ryogami as
  `depth` for the Depth effect only, unchanged on the wire.

## Settings: `~/.config/ryoku/stage.json`

Global only; anything per-wallpaper is in the registry.

| Key | Default | What it is |
|---|---|---|
| `quality` | `draft` | `draft` / `standard` / `fine`, the model + matting pair |
| `edge` | `0.15` | edge softness of every cut-out (0..1) |
| `shadow` | `0` | drop shadow behind every layer (0..1) |
| `shadowAngle` | `90` | shadow direction in degrees, 0 = right, 90 = down |
| `motion.amount` | `normal` | `subtle` / `normal` / `strong`: cursor drift, and the idle amplitude |
| `motion.idle` | `none` | `none` / `float` / `breathe` / `sway` |
| `motion.music` | `false` | layers react to the shared spectrum |
| `motion.musicLevel` | `0.6` | how hard the music pushes (0..1) |
| `motion.speed` | `1.0` | idle motion speed (0.25..2) |
| `motion.mouse` | `true` | Parallax follows the pointer at all |
| `motion.sensitivity` | `1.0` | the pointer's pull (0..2) |
| `motion.range` | `1.0` | how far a layer may travel (0..2) |
| `motion.backdrop` | `0` | the inpainted backdrop's own drift (0..1); above 0 a sliver of the base wallpaper shows at the trailing edge |
| `front` | `[]` | widget ids (built-in, plugin tile, or `visualizer`) drawn above the layers marked "in front"; written by the `Depth` row in a widget's right-click menu or the visualiser's edit bar |

The daemon reads `quality`; the shell reads the rest. On the first start after
v2 a v1 `stage.json` (one still carrying `feather`, `lift`, `preset` or the
`motion.{mouse,sensitivity,range,wallpaper}` sub-knobs) is folded once and
rewritten atomically: `feather` -> `edge`, `lift` and `preset` dropped, and the
motion sub-knobs reduce to `motion.amount` (`mouse: false` -> `subtle`,
else `sensitivity >= 1.5` -> `strong`, else `normal`) with `motion.idle`/
`motion.music` defaulted. An already-v2 file is left alone; the daemon never
creates the GUI-owned file. A stable box that skipped v1 has no `stage.json` but
still carries the retired `depth.json`/`parallax.json`; those are folded instead
(model+matting -> `quality`, higher tier winning; `feather` -> `edge`;
`shadow`/`shadowAngle` scalars kept, per-layer arrays skipped).

## Registry: per-wallpaper stage

`~/.local/state/ryoku/stage-walls.json`:

```
{ "current": "<path>",
  "walls": { "<path>": {
      "effect": "off|depth|parallax",
      "layers": [ { "out": "<png>", "label": "Subject", "enabled": true,
                    "front": true, "depth": 0.5 }, ... ] } } }
```

`layers[0]` is always the subject the engine cut (`subject.png`); every
later entry is a picture the user added (`layer-NN.png`, cut from a picture
or dropped in as a PNG). `front` is behind/in front of the widgets; `depth`
0..1 is near..far for Parallax drift. The v1 registry is folded once, gated by
`~/.local/state/ryoku/migrations/ryostage-v2`: `effect: subject` becomes
`depth`, a `scene` order reduces to each layer's `front` (a layer listed after
any `widget:*` token is `front: true`), a v1 `depthFactor` becomes `depth`, and
`mode`, `scene` and the other per-layer knobs are dropped. A v1 manual wall's
`layer-NN.png` entries are kept after a prepended subject slot. Under the same
marker and before the v1 fold, the retired Depth (`depth-walls.json` +
`~/Pictures/Depth`) and Parallax (`layers.pz` + `~/Pictures/Parallax`) state a
stable box still carries is folded in for walls v1 has not claimed, its
artifacts moved by rename into `~/Pictures/Stage/<stem>/`, so both upgrade paths
converge on one registry.

## Rendering: `modules/stage/`

One surface, one stack. The desktop surface draws, back to front:
`StageBackdrop.qml` (Parallax only: the inpainted `background.png`, sized with
the wallpaper's own fit and drifting with the cursor, so it covers the
wallpaper's baked subject and can never misalign with ryogami's surface), then
the layers marked behind the widgets (z 2), then the widgets (z 3), then the
layers marked in front (`StageLayer.qml`: edge, shadow and angle from the global
look, drift by the layer's `depth` x the shared motion Amount x Sensitivity x
Range while Follow mouse is on, idle and music; z 4), then any widget the user
lifted into `front` (z 5). Depth is the same
stack with `motionEnabled: false` and no backdrop, so the still cut is
pixel-locked over the wallpaper's own subject. While the stage is on and the
visualizer is `On desktop`, the desktop hosts the visualizer inside this stack
(`InlineVisualizer` at z 1.5: above the backdrop, below every cut-out and
widget) and the visualizer's own surface is suppressed (cava keeps running);
`Above windows` and the Placer use that surface as before. There is no second
subject renderer, and no path that can draw the subject twice.
While the engine cuts, the subject layer dims and draws its own progress ring.

The Stage tab sits beside the stack, not in it: `QuickSettingsStage.qml`
writes stage.json through `modules/stage/Singletons/Config.qml` (drag-y
setters coalesce through one settle timer) and the daemon through
`StageBackend`. Edit widgets is `modules/stage/StageWidgetsEditor.qml` (the
toolbar), `StageOutline.qml` (the frame on every widget) and
`StageAddPanel.qml` (the Add widget drop-down), mounted by the desktop surface,
which lifts to the Top layer for the session. Config writes from its Reset go
out as one write per file (`Config.setMany`), because a burst of single-key
writes interleaves with the watcher's reloads of older versions and can put an
old value back.

## Delivery

`ryostage` ships in `ryoku-shell` (`/usr/bin/ryostage`) and via `deploy.sh`;
the QML in `ryoku-desktop`. `tests/shell-tool-availability.sh` gates
`[stage-engine]=ryostage` is not needed (the runtime is opt-in), but the helper
must be on both install paths, which the delivery check enforces.

## Verification

- Daemon: `go build ./...` and its unit tests: the `stage` topic carries the
  frame fields, the worker coalesces off the wallpaper hot path, and the registry
  parses.
- Doctor: hermetic Go tests for the rail migration (retired `depth`/`parallax`
  fold to one `stage` tab, idempotent), the settings migration, and the
  `ryostage cache` reclaim.
- QML: `qmllint` on the new and edited `modules/stage/` files.
- Engine: `bash -n` + shellcheck on `ryostage`; `check`, `models --json` and a
  `cut` against a provisioned cache.
- Delivery: `ryostage` is on both install paths (`deploy.sh` and the
  `ryoku-shell` PKGBUILD), enforced by the delivery check.
- The live visual result and real cut quality need a running session with the
  engine provisioned, exercised on the dev box via `dev-run.sh`.
