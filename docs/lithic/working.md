# Working with Lithic

Use this page for a new surface, a redesign or a migration. The same sequence applies in every stack.

## Workflow

1. **Say who uses it, how often and for how long.** A shell control seen every minute needs a different density from a setup flow used once.
2. **Pick the archetype.** Choose the list and detail window, Now board, draft with aside, settings page, dialog flow, player window, conversation window or compact class in [layout.md](layout.md).
3. **List the states.** Give each state a tone, one plain word and a proof. A colour without a word is not a state.
4. **Find the one consequential action.** Give only that action a gold plaque. Keep following actions as line plaques and rare actions in an overflow menu.
5. **Compose from the map.** Use [map.md](map.md) to choose components before writing custom presentation.
6. **Hook the palette.** Use roles from [palette.md](palette.md), never colour values. Let the library resolve named, wallpaper, forced-mode and fallback sources.
7. **Set the motion budget.** Use [motion.md](motion.md), move only in response to a cause, stop when the response ends, and keep nothing moving at rest.
8. **Write the copy.** Use [copy.md](copy.md) for sentence case, claims, proofs, labels and consequences.
9. **Review and render.** Follow [checklist.md](checklist.md), inspect follow, dark and light modes, and compare the render with the relevant sheet.

## Adopt a library

### QML

Add `docs/lithic/library/qml` to the import path and import the module:

```qml
import Lithic

Plaque {
    kind: "gold"
    text: "Connect"
    previewState: "hover"
}
```

Run a gallery page from its directory:

```sh
cd docs/lithic/examples/qml
QT_FORCE_STDERR_LOGGING=1 LITHIC_PAGE=controls LITHIC_MODE=follow qml6 -I ../../library/qml Gallery.qml
```

`previewState` forces an interactive state for galleries. Overlays use `previewOpen` or `previewEmbedded`. A `CutRect` owns its children through its content item, so bind `cut` and `fill` to the `CutRect` by id. Read the [QML library README](library/qml/README.md) for each public property and signal.

### Web

Load the aggregate stylesheet, import the aggregate module and initialize it:

```html
<link rel="stylesheet" href="/library/web/lithic.css">
<script type="module">
  import Lithic from "/library/web/lithic.js";
  Lithic.init();
</script>
```

Fields use the `l-field-control l-cut` wrapper. Search uses `l-search l-cut`. Use `data-l-state` for forced gallery states and static open frames for overlays. Serve `docs/lithic`, then open `examples/web/<page>.html?mode=<mode>&palette=<url>`. Read the [web library README](library/web/README.md) for component markup and attributes.

### Go with Gio

Use the module `github.com/ryoku-dev/ryoku/docs/lithic/library/go`, resolve roles once and build one theme:

```go
roles := palette.FromPalette(palette.Load(palette.Follow))
th := theme.New(roles)
```

Keep `kit` values in the owning view and call their `Layout` methods. Set a component's `Preview` field to `kit.StateHover`, `kit.StatePressed` or `kit.StateFocus` for a forced gallery state. Run a gallery page with `cd docs/lithic/examples/gio && go run . --page controls --mode follow`. From `docs/lithic/library/go`, `go run ./cmd/lithic-roles --mode dark` prints the roles resolved on the current machine. Read the [Go library README](library/go/README.md) for package and component APIs.

### Terminal

Import `github.com/ryoku-dev/ryoku/docs/lithic/library/go/term`, create a theme with `term.NewTheme(mode)`, then call a pure render entry such as `term.RenderPlaque(th, term.Plaque{...})`. Interactive components expose Bubble Tea v2 models such as `term.TextFieldModel` and `term.MenuModel`.

Run a page with `cd docs/lithic/examples/term && go run . --page controls --width 120 --mode follow`. The Unicode glyph set is the default. The console set is selected when `TERM=linux` or `LITHIC_GLYPHS=console`; `LITHIC_GLYPHS=unicode` forces Unicode. Applications may call `term.SetGlyphMode(term.GlyphConsole)` or `term.SetGlyphMode(term.GlyphUnicode)`.

## Upgrade a Ryoku QML surface

The current `ryoku/ui` library is a paper-and-ink system. Its palette singleton, rounded rectangles, inverted bone selection and decorative sheet vocabulary must be replaced as one surface, not mixed with Lithic one control at a time.

### Map the current roles

| Current `Tokens.qml` role | Lithic destination | Migration note |
|---|---|---|
| `defaultPaper` | signature `ground` | Keep fallback values only in the generated signature. Do not read the default directly. |
| `defaultPaperLift` | signature `stone1` | Overlay fallback belongs to `stone4`, selected by the component rather than this old role. |
| `defaultInk` | signature `fg` | Keep fallback data inside the token output. |
| `defaultInkDim` | signature `fg2` | Keep fallback data inside the token output. |
| `paper` | `ground` | Use `stone1` or `stone2` for panes placed on the ground. |
| `paperLift` | `stone1` for ordinary planes, `stone4` for overlays | Choose by component anatomy. One raised colour no longer serves both jobs. |
| `light` | resolved palette mode | Ask the Lithic theme for its mode. Do not infer local decoration colours. |
| `ink` | `fg` | Primary text and marks. |
| `inkDim` | `fg2` | Secondary text and controls. |
| `inkMuted` | `fg2` | Use the role directly. Do not preserve the old alpha layer. |
| `inkFaint` | `fg3` | Tertiary copy and proof. |
| `bone` | `sel` | Selection is the live secondary container, not fixed inverted stock. |
| `inkOnBone` | `onSel` | Primary ink on selection. |
| `inkOnBoneDim` | `onSel2` | Secondary ink on selection. |
| `lineOnBone` | `onSel2` | Use sparingly for selected-state detail. |
| `keycapDark`, `keycapLight` | no fixed chrome role | Remove the fixed pair and build keycap chrome from the active stone and foreground roles. |
| `keycapOnDark`, `keycapOnLight` | no fixed chrome role | Pair keycap text with the selected live foreground role. |
| `line`, `lineSoft` | `vein` | Lithic has one hairline role. Hierarchy comes from plane and spacing, not extra line alphas. |
| `lineStrong` | `vein` for boundaries, `fg2` for keyboard focus | Use the component recipe instead of a stronger general border. |
| `tint5` | plane hover recipe | Step the plane one tone up. Do not carry the tint token across. |
| `tint10` | transparent or filled hover recipe | Let the component recipe mix the correct ink or fill. |
| `tint16` | press recipe | Deepen the cut and sink content, or use the row press wash. |
| `sun` | `gold` | Use only for the one consequential action. Use minerals for state. |
| `sunDeep` | `goldInk` | Use for line actions that accompany the gold action. |
| `alert` | `jasper` | Resolve it from the palette and tone it for contrast. Do not keep a fixed alert colour. |

The old font, spacing, geometry and duration properties also move to `LithicTokens.type`, `LithicTokens.space`, `LithicTokens.cut`, `LithicTokens.size` and `LithicTokens.motion`. `radius` has no destination. Replace the rounded shape with the component's cut.

### Map the current components

| Current `ryoku/ui` component | Lithic replacement |
|---|---|
| `Btn` | `Plaque`; map `primary` to `kind: "gold"` only when it is the task's consequential action. |
| `IconBtn` | `IconPlaque` with `glyph` and an accessible description. |
| `Field` | `TextField`, or `Search` when the value narrows a collection. |
| `Sw` | `Toggle`. |
| `Step` | `Stepper`. |
| `Slid` | `Slider`. |
| `Chips` | `Filters` when narrowing a list. Use `Tabs` when switching views. |
| `Tabs` | `Tabs`; remove inverted plates and use the moving blade. |
| `Seg` | `Tabs` for views, `Filters` for narrowing, or `Picker` for one setting value. Choose by intent rather than appearance. |
| `Multi` | A `Slab` of labelled `Toggle` rows for independent membership. |
| `PickBar` and `Picker` | One `Picker` with its `Popover` and `Menu`. |
| `Gallery` | `Picker` for one choice, or `RowList` when each option needs proof or secondary facts. |
| `Empty` | `Empty`; replace the ornament and coded label with a title, explanation and next action. |
| `Cell` | A setting row inside `Section` and `Slab`, composed with the appropriate Lithic control. |
| `SettingRow` | A setting row inside `Slab`; keep label and detail before the right-aligned control. |
| `SettingCard` | `Section` plus `Slab`; add `Disclosure` only when the content is optional. |
| `Section` | `Section` plus `Slab`; remove register marks and tracked suffixes. |
| `CardColumns` | The chosen layout archetype, usually `AppFrame` for a page or `DraftColumns` for a form and aside. |
| `ActionBar` | A plain action row made from `Plaque` components, with a `Claim` for saved state and a `Placard` for result or undo. Nothing pulses at rest. |
| `Preview` | App-owned content inside a `Pane` or `Slab`. Keep its label in normal sentence case. |
| `AvatarImage` | The mark slot in `Header`, when identity needs an image. |
| `Barcode` | App-owned content inside a `Slab`; use roles and token dimensions. |
| `FingerprintScan` | App-owned content composed with `Claim`, `BladeProgress` and proof. |
| `ScrollRail` and `WheelScroll` | Scrolling owned by `RowList` or the containing pane, using the Lithic thumb and wheel motion. |
| `Anim` and `Entrance` | Lithic motion helpers and the single motion curve. Remove idle loops. |
| `PickFile` and `AppPicker` | A `Dialog` containing `Search`, `RowList` or `Picker`, with explicit cancel and choose plaques. |
| `FileDialogHost` | Keep the native dialog lifetime wrapper behind the surface. It has no Lithic visual replacement. |
| `BusTopic`, `BusIpcHandler`, `ShellSetting` and `DeferredFileView` | Keep these data and runtime seams behind the surface. They do not draw and have no Lithic visual replacement. |
| `KeyboardMap` | `Section` and `Facts`, with shortcuts as proof text. |
| `Keycap`, `KeyChord` and `KeypressStack` | Compose `CutRect`, `Sentence` and `Marks` from live roles. Keep the shortcut meaning, not fixed keycap colours. |
| `AuraField` and `SpectrumField` | App-owned visualizer content inside a `Pane`; use `EngravedChart` only for a real series. Nothing animates while inactive. |
| `DitherImage` and `HeroCrop` | App-owned media content inside a `Pane` or `Slab`; the frame still uses Lithic roles and cuts. |
| `Grain`, `Motif`, `Marginalia`, `Pixel`, `Reg`, `Ticks` and `Watermark` | Remove decorative sheet chrome. Preserve a part only when it communicates app-owned data, then rebuild it from a named component. |

### Order of work for one QML surface

1. Inventory every state, action, old token read and reusable `Ryoku.Ui` component on the surface.
2. Pick one archetype from [layout.md](layout.md) and sketch its panes, sections and overlay order.
3. Add the Lithic import and palette host at the surface boundary.
4. Replace the outer ground, panes and size-class layout first.
5. Replace controls and rows from the outside in, using the mapping above.
6. Replace local colour, size, type and duration expressions with roles and generated tokens.
7. Rebuild state as a tone, word and proof. Move rare and destructive actions into the overflow path.
8. Remove the old imports, old token reads, rounded shapes, ornament and idle animation from that surface.
9. Check keyboard focus, Escape order, reduced motion and all three palette modes, then render and compare.

Do not ship a half-migrated surface that mixes rounded paper-and-ink controls with cut Lithic controls. Do not keep hardcoded colours, old alpha tints, copied dimensions or compatibility aliases. Migrate the whole surface and remove the old path from its call sites.

## Upgrade other stacks

Use the same cutover in each stack. Inventory the surface, choose an archetype, attach the library at the boundary, replace the outside structure before controls, remove the old styling source, then inspect all modes.

| Stack | Inventory | Attach roles and tokens | Replace | Remove | Check |
|---|---|---|---|---|---|
| Web | Existing classes, inline styles, ARIA state, overlays and layout breakpoints. | Load `lithic.css`, call `Lithic.init()` and let it set `--lithic-*` roles. | Move markup to `l-*` components and native ARIA state, then adopt the matching archetype. | Inline colour and geometry values, rounded cards, shadow layers, blur on controls or type, duplicated interaction scripts. | Keyboard, focus, Escape, pointer dismissal, follow, dark, light and reduced motion in Firefox and Chromium. |
| Gio | Theme colours, widget state, click ownership, layout dimensions and frame invalidation. | Resolve `palette`, create one `theme.Theme`, and pass it through every `kit` layout. | Keep `kit` values in the owning view and compose the archetype from their `Layout` methods. | Local paint recipes, copied cuts, duplicated click handling and idle invalidation. | Keyboard and pointer paths, compact and expanded layouts, palette modes, reduced motion and zero frames at rest. |
| Terminal | Local Lip Gloss styles, width rules, focus order, commands and status strings. | Create one `term.Theme` with `term.NewTheme(mode)` and pass it to each render function or model. | Use the terminal subset in [map.md](map.md#component-index), preserving claim, proof and action order. | Literal terminal colours, decorative borders that replace meaning, independent spacing scales and colour-only state. | Narrow and 120-column widths, keyboard order, both glyph sets, ANSI output in dark and light terminals, and readable state without colour. |

## Add a component across the stacks

1. Write the component contract in [components.md](components.md), including anatomy, states, interaction, accessibility and the `Library:` line.
2. Add every shared dimension, cut, type, motion or alpha to [tokens.json](tokens.json). From `docs/lithic/library/go`, run `go run ./cmd/lithic-tokens` to update generated QML, CSS, JavaScript and Go outputs. Use `-check` when only checking generated output.
3. Add the public QML type under `library/qml/Lithic` and register it in `qmldir`.
4. Add the web class and behaviour under `library/web`, registering behaviour through `core.js` and the aggregate `lithic.js` entry.
5. Add the Gio API under `library/go/kit`, using roles, generated tokens and `kit.Click` for every clickable.
6. Add a pure `term.Render<Name>` function under `library/go/term` when the component reads well in a terminal. Add a data type when it carries data and a Bubble Tea model when it owns interaction.
7. Add every supported state to the QML, web and Gio gallery pages, and to the terminal gallery when it has a terminal form.
8. Add or update the component row in [map.md](map.md#component-index) with the exact public names in each stack.

## Verify

Use `foundation`, `controls`, `lists` or `overlays` for `<page>`, and `follow`, `dark` or `light` for `<mode>`.

### QML

Keep `QT_FORCE_STDERR_LOGGING=1` so QML failures stay with the command instead of going only to the journal. This recipe writes a headless capture:

```sh
cd docs/lithic/examples/qml
QT_FORCE_STDERR_LOGGING=1 QT_QPA_PLATFORM=offscreen QML_XHR_ALLOW_FILE_READ=1 LITHIC_PAGE=controls LITHIC_MODE=follow LITHIC_CAPTURE_PATH=/tmp/lithic-qml.png qml6 -I ../../library/qml Gallery.qml
```

### Web

Serve the Lithic root in one terminal:

```sh
cd docs/lithic
python -m http.server 8000
```

Then take a Chromium screenshot at the gallery's reference width from another terminal:

```sh
chromium --headless --disable-gpu --window-size=1440,900 --screenshot=/tmp/lithic-web.png 'http://127.0.0.1:8000/examples/web/controls.html?mode=follow'
```

### Go with Gio

Use `--png` for a headless render:

```sh
cd docs/lithic/examples/gio
go run . --page controls --mode follow --png /tmp/lithic-gio.png
```

### Terminal

Allocate a PTY so terminal width, colour and glyph detection match a real terminal:

```sh
cd docs/lithic/examples/term
script --quiet --return --command 'go run . --page controls --width 120 --mode follow' /dev/null
```

Repeat with `LITHIC_GLYPHS=console` and `LITHIC_GLYPHS=unicode` when changing terminal geometry or glyph use.

### Palette parity

After changing the palette groups in `tokens.json`, `library/go/palette`, `library/web/palette.js` or `library/qml/Lithic/Lithic.qml`, run:

```sh
docs/lithic/library/parity/check.sh
```

The [parity guide](library/parity/README.md) describes the compared cases and tool overrides.

### Sheets

Serve `docs/lithic` with the web command above, then open a source from the [sheet index](examples/sheets/README.md). Add `?mode=dark` or `?mode=light` to force a mode and compare it with the matching PNG in [media](media/).

### Check by eye

- The surface matches one archetype and changes structure at its documented size class.
- Every plane uses a role and every dimension comes from a token.
- Corners are square or cut. There are no rounded cards, shadows, gradients, glows, inset wells or status dots. Any blur is a faint hint under a picture or on a scrim, and the type over it reads as it does on the bare plane ([design.md](design.md#blur-where-it-fits)).
- Only one task action is gold. Navigation and state do not spend gold.
- Every state has a mineral shape, a word and nearby proof.
- Hover, press and keyboard focus follow the shared recipes. Pointer focus does not leave a keyboard ring.
- Overlays sit one tone up, close in the documented Escape order and swallow the dismissing click.
- Motion stops after the response. An idle capture has no moving element.
- Copy is sentence case and names consequences before irreversible actions.
- Follow, dark and light keep text, controls and state readable. Reduced motion changes instantly without losing state.
