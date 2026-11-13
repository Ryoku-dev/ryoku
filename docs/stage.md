# Ryostage (舞台)

The desktop is a stage. The wallpaper is the backdrop, cut-outs are layers,
and widgets can sit behind or in front of them. Each wallpaper chooses one of
four modes: **Plain, Depth, Parallax, or Spatial.** Depth is a still cut-out,
Parallax lets the cut layers drift, and Spatial turns the wallpaper into a 3D
scene.

Names, so the parts are findable:

| Part | Name | Where |
|---|---|---|
| The feature and editor | **Stage** | `ryoku/shell/quickshell/shell/modules/stage/`, `ryoku/shell/quickshell/stage/` |
| The cut-out engine helper | **`ryostage`** | `ryoku/shell/scripts/ryostage`, shipped to `/usr/bin` |
| The daemon module | `stage` topic and verbs | `ryoku/shell/ipc/stage.go`, `stage_job.go` |
| The settings | `~/.config/ryoku/stage.json` | user-owned, GUI-managed, never materialized |
| Per-wallpaper scene state | `~/.local/state/ryoku/stage-walls.json` | daemon-owned |
| Artifacts | `~/Pictures/Stage/<stem>/` | the user's files, one folder per wallpaper |
| Engine runtime and models | `~/.local/state/ryoku/ryostage/` | one venv, one model cache |

The two retired engine helpers (the old Depth and Parallax segmentation
scripts), `depth.json`, `parallax.json`, `depth-walls.json`, `~/Pictures/Depth`,
`~/Pictures/Parallax` and the `depth`/`parallax` tabs are retired; the doctor
migrates all of them (below).

## The mental model a user needs

One mode is active for each wallpaper:

- **Plain** draws the wallpaper flat.
- **Depth** cuts out the subject and holds it still over the original
  wallpaper. Widgets can sit behind the cut-out.
- **Parallax** cuts the subject, fills the hole in the wallpaper, and lets the
  cut layers drift. Its backdrop can also use a Spatial depth map.
- **Spatial** turns the original wallpaper into a depth-displaced 3D scene.
  It does not cut out the subject.

Depth and Parallax each choose their own cut model and edge refinement.
Spatial chooses a Depth Anything model. A Parallax wallpaper uses that Spatial
model only when its 3D backdrop switch is on.

Two places, each with one job:

- **The sidebar's Stage section** shows the current wallpaper, scene mode,
  enabled widgets, and visualizer state. Its buttons open the matching Stage
  Editor catalogue rather than duplicating controls.
- **The desktop** has one visible way into composition. Right-click bare
  wallpaper and choose **Edit desktop** to open the Stage Editor on that
  monitor. Widget menus remain local to the widget.

The Stage Editor is the shell's one desktop editor. The bar, lockscreen
and menus keep their Ryoku Settings pages. The card the editor shrinks the desktop
into shows the whole desktop: the bar and the dock draw themselves inside
it, at the same scale, so what you arrange is what you get. The dock stays
clickable there and its Dock chip edits it in place; the bar is shown, not
operated, so its popups never open inside the card.

## The desktop right-click menu

The quick row contains **Wallpaper** and **Search**. The single accent row,
**Edit desktop**, opens the Stage Editor on the monitor whose desktop was
clicked. Quick settings opens the left sidebar, Settings opens Ryoku Settings, and Reload
shell reloads the shell.

`Super+Shift+D` opens the Widgets catalogue directly on the focused monitor.
The shortcut is shared, appears in Kit, and can be changed from the Keybinds
page.

Plain, Depth, Parallax, and Spatial are chosen from the Stage Editor's Depth
catalogue:

- Plain: `set-effect off`.
- Depth: `set-effect depth`.
- Parallax: `set-effect parallax`.
- Spatial: `set-effect spatial`.

Choosing a mode only records the choice. Processing never starts from a mode
change, wallpaper wake, settings change, missing file, or failed job. The scene
card shows what is ready and starts work only when the user presses Generate,
Update, Redo, or a per-step Redo action. Depth needs `subject.png`. Parallax
needs `subject.png` and `background.png`, plus `depth.png` when its 3D backdrop
is on. Spatial needs only `depth.png`.

## Catalogue routes

The retired Settings routes remain useful as compatibility entry points.
`ryoku-shell hub open desktop-scene` and `ryoku-shell hub open stage` open the
Depth catalogue directly. The `desktop-scene-visualizer` and `visualizer`
routes open Visualizer; `desktop-scene-widgets` and `widgets` open Widgets. The
desktop menu's
**Settings** row opens Ryoku Settings normally. Ryoku Settings has no scene
settings section and does not duplicate the editor's depth, visualizer, and
widget controls.

Each catalogue owns its complete workflow. Depth includes its picture and PNG
file pickers, Visualizer targets the selected visualizer instance, and Widgets
opens the selected widget's settings inside the editor.

## The Stage Editor

Editing the desktop frames one live monitor at a time. The desktop shrinks into
a rounded card while its live wallpaper is blurred and dimmed around it, with a
soft shadow lifting the card from the surround. `EditModeCard.qml` supplies
that treatment and `Desktop.qml` mounts it around the real wallpaper, stage,
widgets, desktop icons, and editor frames. The island's `Appearance.qml` maps
its paper, ink, spacing, and Geologica, Fraunces, and mono type to Ryoku.Ui
tokens.

The toolbar is **Desktop** | **Widgets** | **Wallpaper** | **Style** |
**Dock** | **Visualizer** | **Depth**, followed by snap, undo, redo, and
**Done**. A chip names the display being edited and cycles displays when
there is more than one. The drawer shows the current catalogue without
repeating the toolbar's section tabs. Visualizer and Depth come from
`StageWidgetProvider.extraSections`; the provider passes them to the
island's `Config.extraSections`.

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

Style reads and writes the same live seams as Ryoku Settings. Light, Dark, Auto, Sun,
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

The page starts with the current mode, then a **This wallpaper** scene card.
The card lists each required step and whether it is ready, missing, stale, or
outdated. It owns Generate, Update, per-step Redo, Redo all, Cancel, and Force
stop. The mode picker offers Plain, Depth, Parallax, and Spatial.

The selected mode decides which tabs are shown:

- **Plain** has no tabs. It explains that the wallpaper and widgets draw flat.
- **Depth** has Look, Layers, Front, and Model.
- **Parallax** has Motion, Look, Layers, Front, and Model.
- **Spatial** has Camera and Model.

Look controls edge softness, shadow strength, and shadow direction. Parallax
also has the 3D backdrop switch. Layers can draw a box around something in the
wallpaper, cut a subject from another picture, add an existing PNG, arrange
layers, and clear cut-outs. Distance is shown only for Parallax. Front chooses
which widgets rise over the cut layers.

Parallax Motion has the Soft, Cinematic, and Beat presets, pointer follow,
idle motion, music response, backdrop drift, and a reset action. Spatial
Camera has pointer follow, sensitivity, range, strength, widget shadow
direction, and a reset action.

The Depth and Parallax Model tabs show the cut-out catalogue. Each mode keeps
its own selected model and Refine edges choice. The Spatial Model tab shows
Depth Anything V2 Small, Base, and Large. Model tabs download and remove
models; the scene card owns every processing run. Picture and PNG selection
stays inside the Stage page rather than opening a separate window.

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
ryoku-shell ipc call desktop editSection <section>[/<page>] [monitor]
```

It opens the editor on catalogues such as `visualizer`, `depth`, `wallpaper`,
or `wallpaper/wallpapers`, using the focused monitor when none is supplied.
`ryoku-shell hub open widgets` and its `Super+Shift+D` shortcut use that route
for the Widgets catalogue. `editWidgets` is an alias for `editSection widgets`.
The `visualizer place` command and `Super+Alt+M` use the same session path with
section `visualizer`. When the editor is already open on that monitor, another
request switches its catalogue in place.

Entering the session opens the chrome through `StageEditorHost`; leaving either
side closes the other. `escapeStep()` unwinds the panel, the selection, and the
session in that order.

## Models: one catalogue, visible provenance

`ryostage` owns the curated list, and the UI renders it instead of hardcoding
model cards:

| id | Catalogue label | Use | Download | Licence | Upstream |
|---|---|---|---:|---|---|
| `u2netp` | U2Net Portable | Fast default | 4.6 MB | Apache-2.0 (mirrored weights) | [U-2-Net](https://github.com/xuebinqin/U-2-Net) |
| `silueta` | Silueta · Fast | Fast general cut | 44.2 MB | MIT | [Silueta model](https://github.com/xuebinqin/U-2-Net/issues/295) |
| `birefnet-general-lite` | BiRefNet General Lite | Fine general cut | 224 MB | MIT | [BiRefNet](https://github.com/ZhengPeng7/BiRefNet) |
| `birefnet-general` | BiRefNet General · Best | Best general cut | 973 MB | MIT | [BiRefNet](https://github.com/ZhengPeng7/BiRefNet) |
| `birefnet-portrait` | BiRefNet Portrait · People | Portrait cut | 973 MB | MIT | [BiRefNet](https://github.com/ZhengPeng7/BiRefNet) |

These are rembg session ids. Although rembg's release assets use longer source
names for the BiRefNet files, each session saves its download as `<id>.onnx`;
that is the name `ryostage check <id>` and `remove <id>` look up.

Depth and Parallax each store one model id and one Refine edges switch.
Refinement enables alpha matting and is slower. The picker shows download size,
licence, installed state, and Download or Remove in place. `.index.json`
records the model and refinement that made `subject.png`, so a different pick
is reported as outdated until the user explicitly updates or redoes the cut.

The runtime (`rembg[cpu]`, MIT, on ONNX Runtime, MIT) installs once, on the
first download, into the shared cache. Nothing ML ships in the base image.
The engine header and this table keep the model provenance visible to
packagers.

### Depth estimators

The 3D backdrop's depth map comes from a second, separate opt-in catalogue of
Depth Anything V2 ONNX exports from
[Depth-Anything-ONNX](https://github.com/fabio-sim/Depth-Anything-ONNX). Each
is one file pinned by SHA-256 so a download is verifiable. The upstream
[Depth Anything V2 licence](https://github.com/DepthAnything/Depth-Anything-V2#license)
licenses only the Small model under Apache-2.0; Base and Large are
CC-BY-NC-4.0 and therefore non-commercial:

| id | Label | Download | Licence | Pins |
|---|---|---:|---|---|
| `small` | Depth Anything V2 Small | 99 MB | Apache-2.0 | ViT-S, ~1.2 s at 1080p on CPU |
| `base` | Depth Anything V2 Base | 389 MB | CC-BY-NC-4.0 | ViT-B, finer structure |
| `large` | Depth Anything V2 Large | 1.34 GB | CC-BY-NC-4.0 | ViT-L, best gradients |

`depth-check --json [--model <id>]` reports each model's size, licence,
upstream, selection and installed state for the Stage tab to render.
`depth-install`, `depth-remove` and `depth` all take `--model` and default to
`small`, so the legacy one-model invocations keep working. `stage.json` stores
the pick at `spatial.model` (default `small`). The daemon uses that exact model
for Spatial and Parallax's optional 3D backdrop. A model choice only changes
readiness; downloading it never starts a depth build.

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
| `cut <in> <out.png> [--model id] [--matting] [--region nx,ny,nw,nh]` | cut the full picture, or cut a normalized region and place it on a full-size transparent canvas; never write a partial file |
| `inpaint <image> <mask> <out.png>` | fill the cut-out's hole with surrounding colour at a bounded working size for the Parallax backdrop |

Cache: `~/.local/state/ryoku/ryostage/{venv,models}`. On first run the
helper adopts a pre-split `~/.local/state/ryoku/depth` or `.../parallax` tree by
rename (same filesystem, no re-download); a leftover second tree is reported
by the doctor as reclaimable space.

`inpaint` makes the Parallax backdrop from one cut. It builds the same half-size
base above 1600 px, grows the matte so subject pixels cannot seed the fill, then
runs the iterative fill at a working size whose longest side is at most 1280
px. Only the filled colour is scaled back and composited into the hole. The
existing final upscale, 4 px blur, and 0.90 dim remain unchanged, so pixels away
from the hole keep the established look while 4K and 8K processing stays
bounded.

## Daemon: `ipc/stage.go` and `ipc/stage_job.go`

The coalescing Stage worker is a pure registry and publication pass. It derives
layers, evaluates readiness, updates Ryogami's Depth pixel-lock hand-off, saves
dirty registry state, and publishes. It never probes models or starts the
engine. `stage_job.go` owns the one explicit job that may run at a time.

- **Artifacts** live in `~/Pictures/Stage/<stem>/`. `subject.png` is the cut,
  `background.png` is Parallax's filled backdrop, `depth.png` is the Spatial
  map, `layer-NN.png` files are manual layers, and `.index.json` records cut and
  depth-model provenance.
- **Transactions** write every step to `<wall dir>/.job-<id>/`. Only a fully
  successful job moves its outputs and provenance into place. Failure or
  cancellation removes the transaction and leaves earlier artifacts untouched.
  Startup removes abandoned `.job-*` folders and stray engine process groups.
- **Readiness** is published per wall in `steps`. Missing means no artifact;
  stale means the wallpaper, or for a fill its subject, is newer; outdated means
  the artifact is usable but its recorded model differs from the current pick;
  ready means current. Unknown provenance is usable. Rendering publishes a
  Depth subject only when its cut is usable, a Parallax scene only when cut is
  usable and fill is ready, and a Spatial map when depth is usable. A change to
  `stage.json` republishes at once, so a new model pick shows as outdated
  without waiting for any other event.
- **Topic** `stage` publishes `{ current, busy, job, last, walls }`. `job` names
  the running or stopping job, ordered steps, current step and model, source
  picture or region, and start times. `last` records the most recent `done`,
  `failed`, or `cancelled` result with a stable error code and bounded engine
  detail. The old synthetic `stage`, `percent`, and `notice` fields do not
  exist.
- **Jobs** are `generate`, `refresh [cut|fill|depth]`, `depth-refresh`,
  `cut-layer`, and `cut-region`. Generate runs only steps not ready. Refresh
  reruns the selected step or all mode steps; refreshing a Parallax cut also
  refills its background. `depth-refresh` remains an alias for
  `refresh depth`.
- **Cancellation** uses `cancel` for SIGTERM and a `stopping` frame. `cancel
  --force` sends SIGKILL, records cancellation, removes the transaction, frees
  the runner immediately, prevents a late child exit from committing, and
  kills user-owned orphan engine groups whose command mentions the Stage root.
- **Wall scope** appends ` -- <wall path>` to any wall-owned verb. Without it,
  the current default wallpaper is used. Paths may contain spaces, so the
  daemon splits the raw line at the first ` -- `.
- **Clear** force-cancels a job for that wall, removes `subject.png`,
  `background.png`, and `.index.json`, keeps manual layers and `depth.png`, and
  changes Depth or Parallax to Plain.

The complete intent verbs are:

```text
stage set-effect <off|depth|parallax|spatial> [-- wall]
stage generate [-- wall]
stage refresh [cut|fill|depth] [-- wall]
stage depth-refresh [-- wall]
stage cancel [--force]
stage cut-layer <picture> [-- wall]
stage cut-region <nx> <ny> <nw> <nh> [-- wall]
stage add-layer <png> [-- wall]
stage remove-layer <index> [-- wall]
stage set-layer <index> <json> [-- wall]
stage clear [-- wall]
stage status
stage models
stage depth-sample <wall> <nx> <ny> <width> <height> <fit>
```

Cut and depth steps have 20 minute limits; fill has a 5 minute limit. A job
probes every cut or depth model it needs before its first processing step.

## Settings: `~/.config/ryoku/stage.json`

The v3 file keeps editor choices at the top and gives each mode its own block:

```json
{
  "version": 3,
  "editGridSize": 32,
  "editGridSnap": true,
  "depth": {
    "cut": { "model": "u2netp", "matting": false },
    "edge": 0.15, "shadow": 0, "shadowAngle": 90, "front": []
  },
  "parallax": {
    "cut": { "model": "u2netp", "matting": false },
    "edge": 0.15, "shadow": 0, "shadowAngle": 90, "front": [],
    "depthBackdrop": true,
    "motion": {
      "amount": "normal", "idle": "none", "music": false,
      "musicLevel": 0.6, "speed": 1.0, "mouse": true,
      "sensitivity": 1.0, "range": 1.0, "backdrop": 0.5
    }
  },
  "spatial": {
    "model": "small",
    "shadowAngle": 90,
    "motion": {
      "amount": "normal", "mouse": true, "sensitivity": 1.0, "range": 1.0
    }
  }
}
```

Depth and Parallax can use different cut models and front lists. Their edge and
shadow controls are also independent. Parallax owns drifting, idle motion,
music response, and its optional 3D backdrop. Spatial owns the camera controls
and its `small`, `base`, or `large` depth model.

The daemon first completes the safe v2 fold, then runs the
`ryostage-v3-modes` migration. A v2 quality choice becomes the same initial cut
model and refinement in Depth and Parallax. Look and front settings are copied
to both. The complete old motion object moves to Parallax; amount, pointer
follow, sensitivity, and range are also copied to Spatial. `depthModel` becomes
`spatial.model`. Editor and unknown keys stay in place. Retired v2 keys are
removed. A malformed file is left untouched, and a second successful run does
nothing.

## Registry: per-wallpaper stage

`~/.local/state/ryoku/stage-walls.json`:

```json
{
  "version": 3,
  "current": "<path>",
  "walls": {
    "<path>": {
      "effect": "off|depth|parallax|spatial",
      "layers": [
        {
          "out": "<png>", "label": "Subject", "enabled": true,
          "front": true, "depth": 0.5
        }
      ]
    }
  }
}
```

The layer list belongs to the two cut modes. Its first entry is
`subject.png`; later entries are pictures the user added. `front` places a
layer behind or in front of widgets. `depth` is its Parallax drift distance.

The old registry had no version field, so a missing version means v2. The v3
migration resolves the old `depth` wire by inspecting the wall folder. A wall
with `depth.png` becomes Spatial. With only `subject.png` it becomes
Depth. With neither it becomes Spatial. The older v1 and retired
`depth-walls.json` folds are known static cut-outs and go straight to Depth.
Parallax and Plain stay unchanged. The migration marker is written only after
both the settings and registry have persisted.

## Rendering: `modules/stage/`

The desktop starts with the normal wallpaper. Each mode adds only its own
pieces:

- **Plain** adds nothing.
- **Depth** draws the still cut stack over the original wallpaper. Ryogami also
  receives the finished subject on its stable `depth` wire so the wallpaper and
  subject stay pixel-locked.
- **Parallax** draws `background.png`, then the cut stack. The layers drift from
  their saved distance and Parallax motion settings. With 3D backdrop on and a
  map available, `StageSpatialScene.qml` turns the backdrop into a mesh.
- **Spatial** uses the original wallpaper and `depth.png` in
  `StageSpatialScene.qml`. It has no cut stack or front list.

`WallpaperFramedPlane` keeps cut layers and backdrops aligned with wallpaper
zoom, offset, rotation, and mirror. Cover uses one centre-cropped UV mapping
for the texture, depth map, matte, and depth picker. Fill stretches them across
the complete source. `DepthMeshGeometry` builds its adaptive grid on a worker
thread and publishes only the newest generation.

For Depth and Parallax, layers behind widgets draw at z 2, widgets draw at z 3,
layers in front draw at z 4, and widgets selected in that mode's Front tab draw
at z 5. Only Parallax applies layer motion. Spatial projects each widget through
the same camera as the wallpaper mesh.

An `On desktop` visualizer stays above the backdrop and below cut layers and
widgets. `Above windows` keeps its separate surface outside the editor. While a
cut runs, the subject layer dims and shows progress.

### Widgets in Spatial depth

With the Spatial model installed, desktop widgets can hold their own place in
the 3D scene instead of staying flat on the desk. Each widget carries a "wall
depth" from 0 to 1, with `0.5` as the map's pivot plane and the extremes nearest
and farthest. Its laid-out centre becomes a fixed world point on the wallpaper
plane, pushed toward or away from the camera by that depth. The point is
projected through the same camera the backdrop mesh uses
(`StageSpatialScene.updateCamera`: orbit radius R, yaw 2.5° and pitch 1.8° per
pointer extreme, gain clamped at 1.5).

The editor's 2D slot stays the layout truth; projection is the render.
`StageBackend.widgetSceneAt()` shares the math with built-in and extension
widget slots. Within the Spatial widget band, a nearer widget stacks above a
farther one.

A widget in Spatial also throws a depth-scaled shadow. Its direction comes from
`spatial.shadowAngle`; nearer widgets use a wider, softer shadow. Outside
Spatial the normal flat lift remains.

The depth is set from the widgets page: a slider for the current value and a
Pick that opens `StageDepthPick.qml`, a tinted read-out of `depth.png` with a
crosshair and live sample; a click commits the sampled value. Built-in faces and
the faces store items contribute (Shima's, Python's) keep depth beside their position and size in the
output-keyed `monitors` map in `widgets.json`, so one widget can sit at a
different depth on each display. A legacy value in the top-level
`widgetDepths` map remains the fallback until that output gets its own value.
Plugin tiles' placements carry their own `depth` in the plugin store instead.
Maps are written as a whole because JsonAdapter cannot observe in-place edits.

A hovered widget settles gently to 0.85 opacity and back. The pointer's
parallax never pauses: the camera, layers and widgets follow one smoothed
cursor shared in `StageBackend`, fed by whichever hover target currently owns
the pointer (the backdrop over empty desktop, each widget slot when the pointer
rests on it), so resting on a widget keeps the scene tracking
continuously. The camera ride itself is unfiltered, so widgets register with
the mesh without trailing behind it.

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
