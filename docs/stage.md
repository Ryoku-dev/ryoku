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
| The feature and editor | **Stage** | `ryoku/shell/quickshell/shell/modules/stage/`, `ryoku/shell/quickshell/stage/` |
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

- **The sidebar's Stage section** shows the current wallpaper, scene mode,
  enabled widgets, and visualizer state. Its buttons open the matching Stage
  Editor catalogue rather than duplicating controls.
- **The desktop** has one visible way into composition. Right-click bare
  wallpaper and choose **Edit desktop** to open the Stage Editor on that
  monitor. Widget menus remain local to the widget.

The Stage Editor is the shell's one desktop editor. The bar, dock, lockscreen
and menus keep their Hub pages.

## The desktop right-click menu

The quick row contains **Wallpaper** and **Search**. The single accent row,
**Edit desktop**, opens the Stage Editor on the monitor whose desktop was
clicked. Quick controls opens the left sidebar, Settings opens Hub, and Reload
shell reloads the shell.

Plain, Depth, and Parallax are chosen from the Stage Editor's Depth catalogue:

- Plain: `set-effect off`.
- Depth: `set-effect depth`.
- Parallax: `set-effect parallax`.

Depth and Parallax use the same cut-outs. The first enable cuts the current
wallpaper if necessary; progress and Stop stay visible while the engine runs.

## Catalogue routes

The retired Hub routes remain useful as compatibility entry points.
`ryoku-shell hub open desktop-scene` and `ryoku-shell hub open stage` open the
Depth catalogue directly. The `desktop-scene-visualizer` and `visualizer`
routes open Visualizer; `desktop-scene-widgets` and `widgets` open Widgets. The
desktop menu's
**Settings** row opens the Hub normally. The Hub no longer has a scene settings
section or duplicates the editor's depth, visualizer, and widget controls.

Each catalogue owns its complete workflow. Depth includes its picture and PNG
file pickers, Visualizer targets the selected visualizer instance, and Widgets
opens the selected widget's settings inside the editor.

## The Stage Editor

Editing the desktop frames one live monitor at a time. The desktop shrinks into
a rounded card while its live wallpaper is blurred and dimmed around it, with a
soft shadow lifting the card from the surround. `EditModeCard.qml` supplies
that treatment and `Desktop.qml` mounts it around the real wallpaper, stage,
widgets, desktop icons, and editor frames. The island's `Appearance.qml` maps
its paper, ink, spacing, and Space Grotesk, Fraunces, and mono type to Ryoku.Ui
tokens.

The toolbar is **Desktop** | **Widgets** | **Wallpaper** | **Style** |
**Visualizer** | **Depth**, followed by snap, undo, redo, and **Done**. A chip
names the display being edited and cycles displays when there is more than one.
The drawer shows the current catalogue without repeating the toolbar's section
tabs. Visualizer and Depth come from `StageWidgetProvider.extraSections`; the
provider passes them to the island's `Config.extraSections`. There is no
lockscreen, bar, or dock editing in this mode.

Frames appear on hover or selection, and while composing the frame owns the
pointer over its widget: a press anywhere on a widget, buttons included, selects
it and a drag moves it, so the widget's own controls do not react until Done.
Each frame has four corner handles, a size badge, and an action strip with the
widget name, Lock or Unlock, Settings, and Remove. A locked widget shows a lock
beside its name, keeps its corners hidden, and says "Locked" when dragged.
Resizing updates the preview locally, then writes the per-display store once on
release and adds one undo step. Size snaps to 5%; hold Shift for free sizing and
double-click a handle to reset to 100%. Moves snap to the grid, the card centre,
and other widgets' edges and centres; hold Alt to move freely. Visualizer
instances move through the same frame: the drag previews in memory, snaps,
joins group moves and nudges, and writes `visualizer.json` once on release.
Their corner grip still sizes and their top dot still turns.

Shift-click or Ctrl-click builds a selection; dragging a marquee on bare
wallpaper can select widgets and desktop icons. A selected group moves together.
Arrow keys nudge one step, or ten steps while Shift is held. Delete removes the
selection. Ctrl+A selects all, Ctrl+Z undoes, Ctrl+Shift+Z or Ctrl+Y redoes, and
Ctrl+F focuses catalogue search. With a widget selected, + and - scale it by
10%, 0 resets it, L locks its position, and Enter opens its settings. An align
bar appears for two or more widgets.

Double-clicking a widget, choosing Settings, right-clicking it while composing,
or pressing Enter opens the hidden `widget` drawer section. `StageWidgetPage`
uses the widget's name as its title and offers Size with Reset, Lock position,
Depth behind or in front when the wallpaper has a cut, and Remove before the
full `WidgetInspector` in embedded mode. Back returns to the catalogue that was
open, and selecting another widget retargets the page. Provider sections use
the `Config.extraSections` `hidden` and `back` contract for this flow.

There is no Save and no global Reset. The live desktop is the document, and
each catalogue edit, group move, nudge, alignment, wallpaper framing change,
style change, add, remove, resize, and scale joins the same undo stack. Done
leaves the editor. Escape closes the open panel, clears the selection, then
leaves the session one level at a time. A bare-wallpaper gesture starts a
selection marquee rather than closing the editor.

### Wallpaper

The Wallpaper catalogue shows Ryogami's library and thumbnails. A per-screen
pick goes through Ryogami's output target, so Ryogami remains the wallpaper
owner. **Use for this workspace** pins the wallpaper currently shown on the
edited display to its active workspace. **Clear workspace wallpaper** removes
that pin and reveals the display, all-display, or shipped-default fallback
again. The workspace changes the selected wallpaper only: Stage scenes and
their generated cut-outs stay keyed by wallpaper path, so two workspaces using
the same image share one scene.

On the card, drag to move the real wallpaper, use the wheel or a pinch to zoom,
and use the restyled framing dock to rotate, mirror, centre, or reset it. A live
drag remains authoritative until its committed record can be read, so releasing
after zoom no longer bounces the picture. Wheel, pinch, and touchpad steps are
written once they settle, so the dock's buttons build on the zoom you see.
Mirror and rotation also re-render the visible picture after a reveal
transition, and a click on the dock between its buttons never reaches the
picture's drag or double-click reset. While framing, widgets, desktop icons, and
the visualizer dim to one quarter opacity and stop taking input. Framing is
stored per monitor and wallpaper path in
`~/.config/ryoku/stage/stage-editor.json`, so returning to a picture restores
its framing.

### Style

Style reads and writes the same live seams as the Hub. Light, Dark, Auto, Sun,
the wallpaper scheme variant, source colour, and named themes update
`matugen.json` through `ryoku-hub desktop matugen set {...}` or
`shell.json` through `ryoku-shell theme <name>`, so the palette changes
everywhere and the page reflects the real current state. Every change is
undoable. Saved looks capture and apply the wallpaper, theme, mode, scheme type,
and source colour. They live in `~/.config/ryoku/stage/style-presets.json`.

### Desktop icons

The Widgets catalogue includes **Add apps to desktop** and **Desktop icons**.
Shortcuts load as soon as the persistent store is ready. Adding or removing an
app updates the desktop immediately, and adding an app unhides icons if needed.
**Add apps to desktop** has a destination row (Desktop, each folder, or a new
folder). Every row says where the app already is, on the desktop, in a folder,
or in the Apps stack, and a placed app is highlighted wherever it sits; tapping
it again removes it. An app is never placed twice: adding it somewhere else
moves it.

**Desktop icons** groups its settings into Desktop, Arrangement, Folders,
Labels and marks, and Other displays. Start from, Fill direction, Spacing, Edge
margin, and Keep clear of panels re-lay the icons already on the desktop as one
undo step; sorting also orders what is inside folders and stacks. The Auto label
style draws a plate on light wallpapers and a shadow on dark ones.

Folders are made from **New folder** on the Desktop icons page or the desktop
menu, or by dropping one icon on another. A folder card renames in place, adds
apps through the same picker, and dissolves back onto the desktop. Each app in
it has Open, Take out, Move to folder, and Remove. The automatic Apps stack
shows its apps the same way and offers **Turn off stacks** to take them out.

Right-clicking an icon opens its own Open, Rename, Details, Copy, and Remove
dialog above widgets without lifting the icon layer. Files, folders, and
`http`, `https`, or `mailto` links can be dropped onto the desktop. The store is
`$XDG_STATE_HOME/quickshell/states.json`, key `desktopShortcutsJson`.

### Per-display widgets

`widgets.json` keeps its existing top-level widget keys as the fallback and
adds output-keyed layout overrides under `monitors`:

```json
{
  "clockEnabled": true,
  "clockX": 72,
  "monitors": {
    "eDP-1": {
      "clockEnabled": true,
      "clockAnchor": "free",
      "clockX": 40,
      "clockY": 64,
      "clockScale": 1.15,
      "clockLocked": false
    }
  }
}
```

An output with no override reads the top-level keys exactly as before. Its
first Stage Editor layout change forks the effective enabled, anchor,
position, size, scale, and lock values into that output's map. Later layout
edits and undo or redo write only that fork. Designs, colours, time format,
face options, and other appearance settings stay global.

### Depth catalogue

The Depth catalogue has five tabs:

- **Scene:** a wallpaper preview and Plain, Depth, or Parallax. A running cut
  shows progress and Stop; a blocked cut shows its notice and Retry. Draft,
  Standard, or Fine quality offers Download or Remove for its model and Re-cut.
- **Layers:** **Cut a layer from a picture** and **Add a PNG** file pickers,
  per-layer cards, and a confirmed Clear cut-outs action. Each layer can be
  shown, placed behind or in front, given a Parallax distance, or removed. The
  subject cannot be removed.
- **Look:** edge softness, shadow strength and direction, and reset.
- **Motion:** Soft, Cinematic, or Beat presets, amount, idle motion, music
  response, and pointer follow.
- **Front:** choose which enabled widgets and plugins rise over the front
  cut-outs.

The picture and PNG pickers live in this catalogue. They do not hand off to the
Hub.

## The visualizer in the Stage Editor

The Visualizer catalogue opens with a gallery of every look, followed by
**Look**, **Place**, **Colour**, **Motion**, **Shape**, and, where applicable,
**Field** chips. Place exposes width, height, across, down, and turn, plus
Centre, Square, Full width, Top, Middle, and Bottom actions. These controls are
undoable like direct gestures.

Every visualizer instance has its own frame and grip with an id such as
`visualizer:0`. Selecting a frame makes that instance the catalogue's target.
Remove deletes only that instance and turns the visualizer off only after the
last instance is gone; undo restores the removed instance at its original index
with all of its settings. Grips appear only while the Widgets or Visualizer
catalogue is open. Drag a frame to move it like any widget, use a corner to size
it, and use the top handle to turn it. The Field look fills the screen and is
tuned from the same catalogue. `visualizer place` and `Super+Alt+M` turn the
visualizer on when necessary and open this catalogue with its instance
selected.

## Session model

`modules/stage/Singletons/StageSession.qml` owns one edit session. `mode` is
`""` or `"widgets"`; `monitor` names the framed output; `selection` is the
ordered set of selected widget ids and `selected` is its primary, final member;
`panel` names the open toolbar panel; and `section` is the catalogue path.
`StageEditorHost.showSection()` splits paths such as `wallpaper/wallpapers`
into the drawer section and page.

Extra provider sections may be hidden from the toolbar and declare where Back
returns. The widget settings section uses both properties, so it behaves as a
temporary detail page rather than another top-level catalogue.

The IPC entry point is:

```
qs -c shell ipc call desktop editSection <section>[/<page>] [monitor]
```

It opens the editor on catalogues such as `visualizer`, `depth`, `wallpaper`,
or `wallpaper/wallpapers`, using the focused monitor when none is supplied.
`editWidgets` is an alias for `editSection widgets`. The `visualizer place`
command and `Super+Alt+M` use the same session path with section
`visualizer`. When the editor is already open on that monitor, another request
switches its catalogue in place.

Entering the session opens the chrome through `StageEditorHost`; leaving either
side closes the other. `escapeStep()` unwinds the panel, the selection, and the
session in that order.

## Models: one catalogue, visible provenance

`ryostage` owns the curated list, and the UI renders it instead of hardcoding
model cards:

| id | Catalogue label | Suggested tier | Download | Licence | Upstream |
|---|---|---|---:|---|---|
| `u2netp` | U2Net Portable | Draft | 4.6 MB | Apache-2.0 (mirrored weights) | [U-2-Net](https://github.com/xuebinqin/U-2-Net) |
| `silueta` | Silueta · Fast | Draft | 44.2 MB | MIT | [Silueta model](https://github.com/xuebinqin/U-2-Net/issues/295) |
| `birefnet-general-lite` | BiRefNet General Lite | Fine | 224 MB | MIT | [BiRefNet](https://github.com/ZhengPeng7/BiRefNet) |
| `birefnet-general` | BiRefNet General · Best | Fine | 973 MB | MIT | [BiRefNet](https://github.com/ZhengPeng7/BiRefNet) |
| `birefnet-portrait` | BiRefNet Portrait · People | Fine | 973 MB | MIT | [BiRefNet](https://github.com/ZhengPeng7/BiRefNet) |

These are rembg session ids. Although rembg's release assets use longer source
names for the BiRefNet files, each session saves its download as `<id>.onnx`;
that is the name `ryostage check <id>` and `remove <id>` look up.

Draft, Standard and Fine remain stable presets. Their defaults are `u2netp`,
`u2netp` with alpha matting, and `birefnet-general-lite` with alpha matting.
The model picker below the presets can replace the model for any one tier
without changing its matting behaviour. It shows each model's download size,
licence and installed state, and offers Download or Remove in place. A model
choice is stored in `stage.json`; changing it re-cuts an active scene, and the
model recorded in `.index.json` prevents an old cut from being reused.

The runtime (`rembg[cpu]`, MIT, on ONNX Runtime, MIT) installs once, on the
first download, into the shared cache. Nothing ML ships in the base image.
The engine header and this table keep the model provenance visible to
packagers.

## Engine: `ryostage`

One bash helper, the only place model logic lives. Backend resolution is
unchanged from the old Depth engine (the managed venv first, then a system Python in
rembg's range, `uv` provisioning a managed 3.13 otherwise).

| Subcommand | Contract |
|---|---|
| `check [model]` | exit 0 and print `available` when the runtime and (with a model named) that model, else (without) at least one curated model, are present; otherwise a one-line reason (`runtime missing`, `model <id> missing`, `missing`) and non-zero |
| `models [--json]` | the curated ids one per line, or full catalogue metadata as JSON |
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
  `.index.json` (source + model/matting provenance).
- **Topic** `stage`: `{ current, busy, stage: "cut"|"inpaint"|"", percent,
  notice, walls: { <path>: { effect, subject, background, rev, layers: [...] } } }`,
  published on every change and on each generation phase. QML renders from it
  and nothing else. `subject`/`background` are absolute paths ("" until fresh);
  `rev` is the max mtime across the wall's `subject.png`/`background.png`/
  `layer-NN.png`, so the shell busts every url with the one revision. The frame
  layers carry `{out, label, enabled, front, depth}` and no per-layer rev, and
  `layers[0]` is always the subject slot. `notice` names why the last reconcile
  could not produce a cut (the engine's reason, e.g. `model u2netp missing`);
  it is "" whenever the pipeline is fine, and the UI shows it instead of a
  silently dead toggle.
- **Verbs** (`ryoku-shell stage ...`): `set-effect <off|depth|parallax>`,
  `set-layer <index> <json>` (enabled/front/depth), `add-layer <png>`,
  `cut-layer <picture>` (runs the engine on another picture and adds the
  result), `remove-layer <index>`, `refresh` (re-cut), `cancel`, `clear`
  (delete the current wall's cut-outs and take the wall back to Plain),
  `status`, `models`.
- **Rules**: a wallpaper switch reconciles and never generates; a stage is
  per wallpaper; videos are skipped; an effect switch never re-cuts (only
  Parallax's first use on a wallpaper adds the inpaint); a wall left on whose
  artifacts vanished is re-cut by the next wake (the registry is the intent);
  a blocked or failed cut keeps the effect recorded, logs the reason, and
  publishes it as the frame's `notice`. The subject is still handed to ryogami
  as `depth` for the Depth effect only, unchanged on the wire.

## Settings: `~/.config/ryoku/stage.json`

Global only; anything per-wallpaper is in the registry.

| Key | Default | What it is |
|---|---|---|
| `quality` | `draft` | `draft` / `standard` / `fine`, which sets the matting behaviour and selects one `models` slot |
| `models` | `{"draft":"u2netp","standard":"u2netp","fine":"birefnet-general-lite"}` | selected rembg model id for each quality tier |
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
| `front` | `[]` | widget and plugin ids drawn above the layers marked "in front"; written by the Depth catalogue's **Front** tab |

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
`StageBackdrop.qml` (Parallax only: the inpainted `background.png`, framed by
`WallpaperFramedPlane` with the wallpaper's zoom, offset, rotation, and mirror),
then the layers marked behind the widgets (z 2), then the widgets (z 3), then
the layers marked in front (`StageLayer.qml`, also framed by
`WallpaperFramedPlane`: edge, shadow and angle from the global look, drift by
the layer's `depth` x the shared motion Amount x Sensitivity x Range while
Follow mouse is on, idle and music; z 4), then any widget the user lifted into
`front` (z 5). Taking the same framing keeps the subject and added cut-outs
registered with the shown picture.

Depth is the same stack with `motionEnabled: false` and no backdrop, so the
still cut is pixel-locked over the wallpaper's own subject. While the stage is
on and a visualizer is `On desktop`, the desktop hosts it inside this stack
(`InlineVisualizer` at z 1.5: above the backdrop, below every cut-out and
widget) and its own surface is suppressed (cava keeps running); `Above windows`
keeps that surface outside the editor. There is no second subject renderer and
no path that can draw the subject twice. While the engine cuts, the subject
layer dims and draws its own progress ring.

The Stage Editor is the only UI writer for composition. Typed stage updates go
to the shell's canonical `modules/stage/Singletons/Config.qml`; per-wallpaper
changes go through the daemon's `StageBackend` contract. The desktop mounts the
widget outlines and lifts to the Top layer for the edit session; the chrome
itself lives on its own surfaces under `shell/modules/stage/`. Resize previews
stay local until release, and every committed undo step writes a store as one
update (`setMany`, or one place-tool command), avoiding reloads of half-written
gestures.

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
