# Working with the Ryoku language

Use this page for a new surface, a redesign or a migration. Ryoku Settings
(the Hub, `ryoku/hub/quickshell/`) is the reference: when a rule here and the
Hub seem to disagree, open the Hub file named and do what it does. The same
sequence applies in every stack.

## Workflow

1. **Say who uses it, how often and for how long.** A shell control seen
   every minute needs a different density from a setup flow used once.
2. **Pick the window shape** in [layout.md](layout.md). A settings window is
   the Hub's: the sidebar tree beside a sheet of ruled sections.
3. **List the states.** Give each state a mineral, one plain word and a
   proof. A colour without a word is not a state.
4. **Find the one consequential action.** Only that action is a `gold`
   plaque; secondary actions are `line`, everything else `quiet`
   ([palette.md](palette.md#4-which-role-goes-where)).
5. **Compose from the map.** Use [map.md](map.md) and the Hub's own parts
   before writing custom presentation.
6. **Hook the style and the palette.** Follow [styles.md](styles.md): read
   the desktop's style and colours, draw every shape as a cut through the
   library, take planes from the library, and use roles
   ([palette.md](palette.md)), never colour values.
7. **Set the motion.** Follow [motion.md](motion.md): move in answer to a
   cause, stop when the answer ends, and keep life at rest behind the
   focused-window and reduce-motion gate.
8. **Write the copy.** Follow [copy.md](copy.md).
9. **Review and render.** Follow [checklist.md](checklist.md), render the
   surface in each style, on Ryoku's colours and the wallpaper's, dark and
   light, and compare it with the Hub renders in [media/](media/).

## Read the Hub first

| To build | Read |
|---|---|
| the window frame: sidebar, page pane, sheet | `Hub.qml:901-1002`, `components/sheet/` |
| the style adapter: planes, washes, cuts, durations | `Singletons/Tone.qml` |
| a settings page from a schema | `components/SettingsPage.qml`, `components/RowControl.qml`, `schema/` |
| the sidebar tree, outline and search | `components/nav/` |
| a change with a notice and Undo | `Hub.qml:838-845, 1104-1158`, `Singletons/Changes.qml` |
| an overview with graphs | `components/Home.qml` readings, `EngravedChart` |
| a confirm | `hub.confirm()` in `Hub.qml:381-399` |
| life at rest | `Singletons/Ambient.qml`, `Hub.qml:405-410` |
| pointer follow, section entrance | `components/Chase.qml`, `components/Reveal.qml` |

The pattern names (sheet, masthead, colophon, folio, plate, registration
marks, tick ruler, plaque tiers, sidebar tree, section nodes) are defined in
[design.md](design.md#2-vocabulary).

## Adopt a library

### QML

Add `docs/lithic/library/qml` to the import path and import the module, as
the Hub does in 164 of its files:

```qml
import Lithic

Plaque {
    kind: "gold"
    text: "Connect"
    previewState: "hover"
}
```

At the surface's root, bind `Lithic.reduceMotion` to the desktop's
`theme.motion.reduce` and call `Lithic.reload()` when the palette files
change, as `ryoku/hub/quickshell/ShellBody.qml:23-78` does. Run a gallery
page from its directory:

```sh
cd docs/lithic/examples/qml
QT_FORCE_STDERR_LOGGING=1 LITHIC_PAGE=controls LITHIC_MODE=dark qml6 -I ../../library/qml Gallery.qml
```

`previewState` forces an interactive state for galleries. Overlays use
`previewOpen` or `previewEmbedded`. A `CutRect` owns its children through
its content item, so bind `cut` and `fill` to the `CutRect` by id. Read the
[QML library README](library/qml/README.md) for each public property and
signal.

### Web

Load the aggregate stylesheet, import the aggregate module and initialize it:

```html
<link rel="stylesheet" href="/library/web/lithic.css">
<script type="module">
  import Lithic from "/library/web/lithic.js";
  Lithic.init();
</script>
```

Fields use the `l-field-control l-cut` wrapper. Search uses
`l-search l-cut`. Use `data-l-state` for forced gallery states and static
open frames for overlays. Serve `docs/lithic`, then open
`examples/web/<page>.html?mode=<mode>&palette=<url>`. Read the
[web library README](library/web/README.md) for component markup and
attributes.

### Go with Gio

Use the module `github.com/ryoku-dev/ryoku/docs/lithic/library/go`, resolve
roles once and build one theme:

```go
roles := palette.FromPalette(palette.Load(palette.Dark), palette.Look{})
th := theme.New(roles)
```

`palette.Dark`, the zero `palette.Mode`, is Ryoku's signature;
`palette.Light` is its light table. Pass `palette.Follow` when the desktop's
`theme.colours` is `wallpaper`, and run `palette.Watch` only while following.
`palette.ParseMode` reads `dark`, `light` or `follow`;
`palette.ParseLook(contrast, tint)` reads the reader's contrast and tint.
Keep `kit` values in the owning view and call their `Layout` methods. Set a
component's `Preview` field to `kit.StateHover`, `kit.StatePressed` or
`kit.StateFocus` for a forced gallery state. From `docs/lithic/library/go`,
`go run ./cmd/lithic-roles --mode follow` prints the roles resolved from
this machine's palette. Gio icons come from package `symbols`; add a name to
`symbols/list.txt` and run `go run ./cmd/lithic-symbols`. Read the
[Go library README](library/go/README.md) for package and component APIs.

### Terminal

Import `github.com/ryoku-dev/ryoku/docs/lithic/library/go/term`, create a
theme with `term.NewTheme(mode)`, then call a pure render entry such as
`term.RenderPlaque(th, term.Plaque{...})`. Interactive components expose
Bubble Tea v2 models such as `term.TextFieldModel` and `term.MenuModel`.

The Unicode glyph set is the default. The console set is selected when
`TERM=linux` or `LITHIC_GLYPHS=console`; `LITHIC_GLYPHS=unicode` forces
Unicode. Applications may call `term.SetGlyphMode(term.GlyphConsole)` or
`term.SetGlyphMode(term.GlyphUnicode)`.

## Upgrade a Ryoku QML surface

Many shell surfaces still draw with `Ryoku.Ui` Tokens, which rounds where
the Hub cuts and does not read the desktop's colour choice
([styles.md](styles.md#where-each-stack-stands)). Replace a surface as one
piece, the way the Hub was built: never mix rounded Tokens controls with cut
Lithic ones on one surface.

### Map the current roles

| `Tokens.qml` role | Ryoku role | Note |
|---|---|---|
| `paper`, `defaultPaper` | `ground` | the window and sidebar; take panes from `Lithic.planes()` |
| `paperLift`, `defaultPaperLift` | `slab` for a group in a page, `stone4` for what floats | choose by what the plane is |
| `light` | `Lithic.dark` | ask the library; never infer it |
| `ink`, `defaultInk` | `fg` | |
| `inkDim`, `inkMuted`, `defaultInkDim` | `fg2` | use the role, not an alpha layer |
| `inkFaint` | `fg3` | |
| `bone` | `container` under the chosen item, `sel` for its marks; `sel` fill for a control that is on | the Hub marks the open page with a wash and the sun, not an inverted plate (`components/nav/NavRow.qml:55-61, 128`) |
| `inkOnBone`, `inkOnBoneDim`, `lineOnBone` | `containerInk` on the wash; `onSel`, `onSel2` on a `sel` fill | |
| `line`, `lineSoft` | `vein` | one hairline role for planes and rules |
| `lineStrong` | `edge` for a field, `fg2` for keyboard focus | |
| `tint5`, `tint10` | a hover wash: `fg` at a small alpha, as `Tone.hover` | `Singletons/Tone.qml:26` |
| `tint16` | a press wash, as `Tone.press`, or the plaque's press | `Tone.qml:27` |
| `sun` | `gold` | the one consequential action |
| `sunDeep` | `goldInk` | `line` plaques |
| `alert` | `jasper` | |
| `keycapDark`, `keycapLight`, `keycapOnDark`, `keycapOnLight` | the live stone and ink roles | no fixed keycap pair |

Fonts, spacing, geometry and durations move to `LithicTokens.font`,
`type`, `space`, `cut`, `size` and `motion`. `radius` has no destination:
give the shape its cut.

### Map the current components

| `ryoku/ui` component | Replacement |
|---|---|
| `Btn` | `Plaque`, `quiet` by default; `gold` only for the one consequential action |
| `IconBtn` | `IconPlaque`, or the Hub's `IconButton` pattern (`components/IconButton.qml`) |
| `Field` | `TextField`, or `Search` when the value narrows a list |
| `Sw`, `Step`, `Slid` | `Toggle`, `Stepper`, `Slider` |
| `Seg`, `Chips` | `Segmented` for two to five options of one setting, `Picker` for a longer set |
| `Tabs` | `Segmented`, as the card studio switches its tabs (`media/hub-studio-dark.png`) |
| `Multi` | a `Slab` of labelled `Toggle` rows |
| `PickBar`, `Picker`, `Gallery` | one `Picker` with its `Popover` and `Menu`, or `RowList` when each option needs facts |
| `Empty` | `Empty`: a title, an explanation and the next action |
| `Cell`, `SettingRow`, `SettingCard`, `Section` | `SettingRow` rows in a `Slab` under a ruled `Section`; `Disclosure` only for optional content |
| `CardColumns` | the Hub's frame (`Hub.qml:901-1002`) or the shape in [layout.md](layout.md) |
| `ActionBar` | plaques on the page, a `Claim` for saved state, and a placard with Undo for each change |
| `ScrollRail`, `WheelScroll` | Lithic `ScrollRail` |
| `Anim`, `Entrance` | `Lithic.duration()`, the Hub's `Reveal` and `Chase` ([motion.md](motion.md)) |
| `PickFile`, `AppPicker` | a `Dialog` with `Search`, `RowList` or `Picker`; `PathField` for a path |
| `KeyboardMap`, `Keycap`, `KeyChord`, `KeypressStack` | `Section` and `Facts` to show shortcuts, `ChordField` to record one; keys from `CutRect`, `Sentence` and `Marks` |
| `Barcode` | the Hub's colophon barcode (`components/about/CardBarcode.qml`) |
| `DitherImage`, `HeroCrop` | the Hub's dithered plate (`components/sheet/Plate.qml`, `shaders/plate.frag`) |
| `Reg`, `Ticks`, `Marginalia`, `Watermark`, `Grain`, `Motif`, `Pixel` | the Hub's sheet parts: `RegSheet`, `Masthead`, `TickRuler`, `Folio`, `Colophon`; drop the rest |
| `AuraField`, `SpectrumField` | app content; `EngravedChart` for a real series |
| `FileDialogHost`, `BusTopic`, `BusIpcHandler`, `ShellSetting`, `DeferredFileView` | keep them: they do not draw, and the Hub uses them too |

### Order of work for one QML surface

1. Inventory every state, action, old token read and `Ryoku.Ui` component on
   the surface.
2. Pick the shape from [layout.md](layout.md) and sketch its panes, sections
   and overlays.
3. Add the Lithic import, the reload and the reduce-motion binding at the
   surface's root.
4. Replace the ground, panes and size classes first.
5. Replace controls and rows from the outside in, using the maps above.
6. Replace local colour, size, type and duration expressions with roles and
   tokens.
7. Rebuild each state as a mineral, a word and a proof.
8. Remove the old imports, token reads, rounded shapes and idle animation
   from that surface.
9. Check keyboard focus, Escape order, reduced motion, each style, both
   colour sources, dark and light; then render it and compare it with the
   Hub.

Do not ship a half-migrated surface. Do not keep hardcoded colours, old
alpha tints, copied dimensions or compatibility aliases.

## Upgrade other stacks

Use the same cutover in each stack: inventory the surface, choose its shape,
attach the library at the boundary, replace the outside structure before
controls, remove the old styling source, then inspect every look.

| Stack | Inventory | Attach roles and tokens | Replace | Remove | Check |
|---|---|---|---|---|---|
| Web | classes, inline styles, ARIA state, overlays, breakpoints | load `lithic.css`, call `Lithic.init()` and let it set `--lithic-*` roles | move markup to `l-*` components and native ARIA state | inline colour and geometry values, rounded cards, duplicated interaction scripts | keyboard, focus, Escape, pointer dismissal, follow, dark, light and reduced motion in Firefox and Chromium |
| Gio | theme colours, widget state, click ownership, dimensions, frame invalidation | resolve `palette`, create one `theme.Theme`, pass it through every `kit` layout | compose the shape from `kit` values kept in the owning view | local paint recipes, copied cuts, duplicated click handling, idle invalidation | keyboard and pointer, each size class, every palette mode, reduced motion, no frames at rest |
| Terminal | Lip Gloss styles, width rules, focus order, status strings | one `term.Theme` from `term.NewTheme(mode)` | the terminal subset in [map.md](map.md), keeping claim, proof and action order | literal terminal colours, decorative borders that replace meaning, colour-only state | a narrow and a 120-column width, keyboard order, both glyph sets, dark and light terminals, state readable without colour |

## Add a component across the stacks

1. Write the component contract in [components.md](components.md): anatomy,
   states, interaction, accessibility and the `Library:` line.
2. Add every shared dimension, cut, type, motion or alpha to
   [tokens.json](tokens.json). From `docs/lithic/library/go`, run
   `go run ./cmd/lithic-tokens` to regenerate the QML, CSS, JavaScript and Go
   outputs; add `-check` to only check them.
3. Add the QML type under `library/qml/Lithic` and register it in `qmldir`.
4. Add the web class and behaviour under `library/web`, registering behaviour
   through `core.js` and the aggregate `lithic.js` entry.
5. Add the Gio API under `library/go/kit`, using roles, tokens and
   `kit.Click` for every clickable.
6. Add a pure `term.Render<Name>` under `library/go/term` when the component
   reads well in a terminal.
7. Add every supported state to the gallery pages.
8. Add or update its row in [map.md](map.md).

## Verify

Never verify on the live desktop. A screen capture grabs whatever the reader
is looking at, and opening Settings the normal way
(`ryoku-shell hub open`) puts it on their screen. Render in a private
headless compositor instead.

### Render the Hub privately

This is how the reference renders in [media/](media/) were made. It runs
Settings in its own headless weston, with the real settings read through the
daemon, and touches nothing on the reader's screen.

1. A private runtime directory, holding a link to the shell daemon's socket
   so pages show the real settings:

   ```sh
   rt=/run/user/$(id -u)/hubprobe
   mkdir -m700 -p "$rt"
   ln -sf "/run/user/$(id -u)/ryoku-shell.sock" "$rt/ryoku-shell.sock"
   ```

2. Weston, headless. `--debug` is required, or the screenshooter does
   nothing:

   ```sh
   env -i PATH="$PATH" HOME="$HOME" XDG_RUNTIME_DIR="$rt" \
       weston --debug --backend=headless --renderer=gl --socket=hubprobe-0 \
       --width=1440 --height=900 --idle-time=0 &
   ```

3. Settings in it, run from the repository root:

   ```sh
   env -i PATH="$PATH" HOME="$HOME" USER="$USER" LANG="${LANG:-en_US.UTF-8}" \
       QML_IMPORT_PATH="$PWD/docs/lithic/library/qml:$HOME/.local/lib/qt6/qml" \
       XDG_RUNTIME_DIR="$rt" WAYLAND_DISPLAY="$rt/hubprobe-0" \
       QT_QPA_PLATFORM=wayland QT_WAYLAND_DISABLE_WINDOWDECORATION=1 \
       RYOKU_BUS_CLIENT=hub-probe RYOKU_TEAM_URL=http://127.0.0.1:1 \
       RYOKU_HUB_START=window-style LITHIC_MODE=dark \
       qs -p ryoku/hub/quickshell &
   ```

   - `RYOKU_HUB_START` opens a page, or `page/row` (`Hub.qml:849`).
   - `RYOKU_BUS_CLIENT` keeps the probe from registering as Settings, so the
     daemon still sends the reader's `hub open` to their own window
     (`ryoku/shell/plugin/busclient.cpp:63-67`).
   - `RYOKU_TEAM_URL` points card updates at a dead port, so the probe makes
     no network calls.
   - `LITHIC_MODE=dark|light` paints Ryoku's colours; `follow` paints the
     wallpaper's. `LITHIC_DESIGN=material|adwaita|fluent` renders another
     style.

4. After about ten seconds, capture. The file lands in `~/Pictures` upside
   down:

   ```sh
   WAYLAND_DISPLAY=hubprobe-0 XDG_RUNTIME_DIR="$rt" weston-screenshooter
   magick ~/Pictures/wayland-screenshot-<time>.png -flip hub-page-dark.png
   rm ~/Pictures/wayland-screenshot-<time>.png
   ```

5. Tear down: kill the probe (`qs list --all` names it by its `hubprobe`
   display), kill weston, remove `"$rt"`.

Things to know:

- Opening a page with `RYOKU_HUB_START` records it as the reader's last page
  (`Hub.qml:268-277`). Set it back afterwards with
  `ryoku-hub config set section <page>`.
- The headless window is never focused, so `Ambient` stays off and the
  render shows the window at rest ([motion.md](motion.md#motion-at-rest)).
- There is no pointer. To open an overlay (search, a confirm, the card
  studio, the binder), add a temporary `Timer` that calls `hub.focusSearch()`,
  `hub.logOut()`, `hub.customiseHome("picture")` or `hub.openBinder()` in a
  copy of the tree, never in the repository, and render the copy.
- Compare the result with the reference renders:
  [home](media/hub-home-dark.png), [page](media/hub-page-dark.png),
  [overview](media/hub-overview-dark.png),
  [search](media/hub-search-dark.png),
  [dialog](media/hub-dialog-dark.png),
  [studio](media/hub-studio-dark.png) and
  [binder](media/hub-binder-dark.png), each with a `-light` twin.

### QML galleries and behaviour tests

Keep `QT_FORCE_STDERR_LOGGING=1` so QML failures stay with the command. This
writes a headless capture of a gallery page:

```sh
cd docs/lithic/examples/qml
QT_FORCE_STDERR_LOGGING=1 QT_QPA_PLATFORM=offscreen QML_XHR_ALLOW_FILE_READ=1 LITHIC_PAGE=controls LITHIC_MODE=dark LITHIC_CAPTURE_PATH=/tmp/lithic-qml.png qml6 -I ../../library/qml Gallery.qml
```

The behaviour tests drive the QML components with real key and pointer
events. Run them after any change under `library/qml/Lithic`, and add a
failing case first when you fix a behaviour bug:

```sh
QT_QPA_PLATFORM=offscreen "$(qmake6 -query QT_HOST_BINS)/qmltestrunner" -import docs/lithic/library/qml -input docs/lithic/library/qml/tests
```

Use `foundation`, `controls`, `lists` or `overlays` for a gallery page, and
`dark`, `light` or `follow` for its mode.

### Web

Serve the Lithic root, then take a Chromium screenshot from another
terminal:

```sh
cd docs/lithic
python -m http.server 8000
```

```sh
chromium --headless --disable-gpu --window-size=1440,900 --screenshot=/tmp/lithic-web.png 'http://127.0.0.1:8000/examples/web/controls.html?mode=dark'
```

### Go with Gio

```sh
cd docs/lithic/examples/gio
go run . --page controls --mode dark --png /tmp/lithic-gio.png
```

### Terminal

Allocate a PTY so width, colour and glyph detection match a real terminal:

```sh
cd docs/lithic/examples/term
script --quiet --return --command 'go run . --page controls --width 120 --mode dark' /dev/null
```

Repeat with `LITHIC_GLYPHS=console` and `LITHIC_GLYPHS=unicode` when
changing terminal geometry or glyph use.

### Palette parity

After changing the palette groups in `tokens.json`, `library/go/palette`,
`library/web/palette.js` or `library/qml/Lithic/Lithic.qml`, run:

```sh
docs/lithic/library/parity/check.sh
```

The [parity guide](library/parity/README.md) describes the cases.

### Check by eye

- The surface has one shape from [layout.md](layout.md), drawn like the Hub.
- Every plane uses a role and every dimension a token; no colour value.
- In Lithic every corner is cut; nothing sets a radius. No lit top edge on a
  button, no vertical accent bar on the left of an option, no glossy or
  heavily dithered shine.
- Text is Geologica; data, numbering and running heads are Martian Mono.
- Kanji gloss shows only behind `UiPreferences.showJapaneseGloss`.
- One action per view is gold. The accent marks place only.
- Every state has a mineral, a word and a proof nearby.
- An overview presents its data with graphs.
- Hover, press and keyboard focus follow the shared recipes; pointer focus
  leaves no keyboard ring.
- Motion stops after it answers. Unfocused, the window draws nothing.
- Each style, both colour sources, dark and light keep text, controls and
  state readable. Reduced motion changes everything at once without losing
  state.
