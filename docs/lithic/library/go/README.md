# Lithic for Go

This module is the shared Lithic foundation for Gio applications and terminal applications. It reads the live Ryoku palette, maps it into semantic roles, exposes the generated token scales, and provides motion and drawing primitives that do no work at rest.

## Add the module

From another repository, pin a Ryoku commit:

```sh
go get github.com/ryoku-dev/ryoku/docs/lithic/library/go@<commit>
```

Inside this repository, use a local replacement while developing:

```go
require github.com/ryoku-dev/ryoku/docs/lithic/library/go v0.0.0

replace github.com/ryoku-dev/ryoku/docs/lithic/library/go => ../../library/go
```

The packages are imported separately:

```go
import (
    "github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
    "github.com/ryoku-dev/ryoku/docs/lithic/library/go/fonts"
    "github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
    "github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
    "github.com/ryoku-dev/ryoku/docs/lithic/library/go/symbols"
    "github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
    "github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)
```

A normal Gio setup is:

```go
roles := palette.FromPalette(palette.Load(palette.Follow), palette.Look{})
th := theme.New(roles)
```

`theme.New` loads Lithic's two faces, Geologica and Martian Mono, which package `fonts` embeds, so an application sets the same type on any system without installing anything; system fonts stay on for the scripts those faces do not cover. Pass `theme.WithFontCollection(faces)` to add the application's own faces beside them, and `theme.WithShaper(shaper)` when the application owns a configured shaper, which then needs `fonts.Collection()` in its collection.

## Palette API

`palette` uses only the Go standard library. It reads these files in precedence order:

1. `$XDG_CONFIG_HOME/ryoku/shell.json`, key `themePalette`
2. `$XDG_CACHE_HOME/ryoku/colors.json` when wallpaper following is enabled or the mode is forced
3. the signature fallback, with the edition accent from `/etc/os-release`

`XDG_CONFIG_HOME` defaults to `~/.config`; `XDG_CACHE_HOME` defaults to `~/.cache`.

Exported API:

- `Mode`: `Follow`, `Dark`, `Light`.
- `Palette`: source `Roles`, resolved `Dark`, `Forced`, and `Source` values before Lithic mapping.
- `Load(mode) Palette`: resolves source precedence and mode.
- `Contrast`: `Standard`, `High`. `Tint`: `TintOff`, `TintLow`, `TintMedium`.
- `Look`: the reader's `Contrast` and `Tint`. The zero look (standard, off) is the library default.
- `ParseLook(contrast, tint) Look`: reads `standard` or `high` and `off`, `low` or `medium`; an unknown string gives the zero value of its field.
- `FromPalette(Palette, Look) Roles`: with no palette, returns the signature table for the mode and contrast, with an edition's colour as gold; with one, dyes every role at its step on the tone ladder for the mode and contrast (see [palette.md](../../palette.md#3-mapping-palette-roles-to-lithic-roles)).
- `Watch(ctx, mode, fn)`: watches the three parent directories with inotify, coalesces file replacement, and calls `fn` only when the resolved palette changes. `mode` is a function so an application can change its mode without rebuilding the watcher.
- `Tone`: `Idle`, `Alive`, `Becoming`, `Attention`, `Paused`, `Failed`.
- `Roles`: `Ground`, `Stone1`, `Stone2`, `Stone3`, `Stone4`, `Slab`, `Field`, `Vein`, `Edge`, `Fg`, `Fg2`, `Fg3`, `Sel`, `OnSel`, `OnSel2`, `Gold`, `OnGold`, `GoldInk`, `Jade`, `Lapis`, `Amber`, `Jasper`, `Scrim`, `Dark`, `Source`.
- `Roles.Stone(tone)`: returns the mineral ink for a tone.
- `Roles.MineralField(tone)`: returns the derived mineral field for a tone.
- `Signature(dark)`: returns the signature roles at standard contrast.
- `Mix(a, b, t)`: linearly blends every channel.
- `WithAlpha(color, alpha)`: multiplies the existing alpha.
- `InkOn(background)`: picks black or white by contrast.
- `ContrastRatio(a, b)`: returns the WCAG contrast ratio.

Forced dark and light modes keep the signature neutrals and take only the accents from the palette, the selection included. The tint applies only when following a palette, and it changes chroma, never lightness. Fixed minerals always come from tokens.

## Fonts

`fonts` embeds Geologica (Regular, Medium and Bold) and Martian Mono (Regular and Bold) as static instances with one width each, because Gio cannot ask for a width. `fonts.Collection()` parses them once and returns the shared faces. The QML and web libraries load the same files from `fonts/`, so this directory is the only copy. `python3 fonts/build.py` rebuilds them from the Google Fonts builds at pinned commits; both faces are OFL-1.1, and `fonts/Geologica-OFL.txt` and `fonts/MartianMono-OFL.txt` carry the license.

## Symbols

`symbols` holds Material Symbols Sharp as IconVG data, outline at weight 400. Each exported `[]byte` is named after its symbol in CamelCase (`expand_more` is `symbols.ExpandMore`); a symbol listed with `fill` also has a filled `...Fill` variant (`symbols.SettingsFill`) for the current navigation destination. Draw one with `kit.Icon(symbols.Search)`, built once and kept, never per frame.

To add a symbol, put its name in `symbols/list.txt` and run `go run ./cmd/lithic-symbols` here; `-check` reports a stale `symbols.go`. The generator fetches the official SVGs at a pinned upstream commit and caches them under `/dev/shm/lithic-symbols`. Material Symbols are Apache-2.0; `symbols/LICENSE` and `symbols/NOTICE` carry the license and provenance.

## Theme API

`theme.Theme` contains `R palette.Roles`, `T Type`, `Cut`, `Sp Space`, `Motion`, `Density`, and `Shaper`. `New(roles, options...)` fills every scale from `tokens`.

Exported API:

- `TextStyle`: `Face`, `Size`, `Line`.
- `Type`: `Name`, `Sentence`, `Section`, `Title`, `Row`, `Body`, `Proof`, `Label`, `Detail`, `Caption`, `Group`, `Mono`, `MonoSm`.
- `Cut`: `Token`, `Small`, `Mid`, `Large`, `Notch`, `Menu`, `Overlay`, `Placard`, `Tooltip`, `ToggleTrack`, `ToggleKnob`.
- `Space`: `S1` through `S6`.
- `Motion`: `Press`, `HoverIn`, `HoverOut`, `FocusIn`, `FocusOut`, `Select`, `Field`, `Notch`, `Unfold`, `Navigate`, `MenuIn`, `MenuOut`, `DialogIn`, `DialogOut`, `TooltipIn`, `TooltipDelay`, `PlacardIn`, `PlacardLife`, `Wheel`, `Working`, `Sweep`.
- `Density`: `Comfortable`, `Compact`.
- `Theme.RowHeight()`: 56 dp when comfortable and 48 dp when compact.
- `Option`, `WithShaper`, `WithFontCollection`, `New`, `NewShaper`.

## Motion API

All motion defaults to the chisel curve. A settled value requests no frame.

- `SetReduce(bool)` and `Reduced()` control process-wide reduced motion.
- `Spring`: exported tuning fields `Omega` and `Rest`; methods `Set`, `Snap`, `Target`, `Moving`, `Value`.
- `Tween`: exported `Duration` and `Ease`; methods `Set` and `Value`.
- `Phase(gtx, period)` returns a looping zero-to-one phase. Call it only while work is active because it requests another frame.
- `EaseChisel(progress)` evaluates the generated curve `cubic-bezier(.2, 0, 0, 1)`.

Reduced motion snaps springs and tweens to their targets and freezes phases at a legible state.

## Gio kit rules

`kit.State` forces a final interactive state for static previews. Its values are
`StateRest`, `StateHover`, `StatePressed`, and `StateFocus`. Components that
support it expose a `Preview kit.State` field. A forced state renders its final
value on the first frame instead of waiting for its normal transition.

Keep these Gio details in mind:

- `kit.Click.Layout` drains pointer events into a latch. `Update` and `Clicked`
  hand out one latched click and issue `op.InvalidateCmd` so the state caused by
  that click is painted.
- Gio constraints size a widget but do not clip its operations. An unclipped
  `paint.Fill` covers the window. Push a `clip.Rect`, cut path, or other clip
  around every painted surface.
- Live overlays use `op.Defer` through the kit overlay host. A non-rest
  `Preview` draws the overlay in place so a gallery can show it inside its
  specimen bounds.
- `select` is a Go keyword. Use names such as `selected`, `selection`, or
  `choose` in application code.

## Foundation reference

| API | Purpose and caller input |
|---|---|
| `State` | Preview state. Use `StateRest`, `StateHover`, `StatePressed`, or `StateFocus`. |
| `Click` | Embeds `widget.Clickable`. Call `Layout(gtx, child)`, then consume events with `Update(gtx)` or `Clicked(gtx)`. |
| `Label` | Text component with caller fields `Style theme.TextStyle`, `Color color.NRGBA`, `MaxLines int`, and `Align text.Alignment`. `Layout(gtx, th, value)` renders it. |
| `Text`, `RawText` | Return one-line `layout.Widget` values. `Text` applies disabled ink. `RawText` keeps the supplied ink. |
| `Icon` | `Icon(data []byte) *widget.Icon` decodes IconVG data such as a `symbols` value and panics for invalid data. |
| `IconByName` | Returns the built-in `home`, `search`, `settings`, or `close` icon. It returns `nil` for any other name. |
| `CutPath` | Builds a `clip.PathSpec` from bounds and independent northwest, northeast, southeast, and southwest cuts. |
| `FillCut`, `FillCutCorners`, `FillCutTop` | Fill a cut rectangle, independent cut corners, or cut top corners. |
| `StrokeCut`, `StrokeCutCorners` | Draw an inside stroke for equal or independent cuts. |
| `Slab` | Records a child, paints the supplied cut background, then replays the child. |
| `Notch` | Draws a right-triangle state mark at an `image.Point`. |
| `Blade` | Draws a bar with a 45 degree leading edge. |
| `Flake` | `Flake(gtx, th, tone, fraction, size)` draws idle, filled, or partially becoming state. |
| `Hatch` | Engraves clipped 45 degree lines across a supplied `clip.PathSpec`. |
| `FocusRingAlpha`, `FocusRing` | Draw the shared keyboard-focus ring around the supplied bounds and cut. |

## Controls reference

Every field named below is exported and intended for the caller. Fields not
listed are internal animation or interaction state.

| Component | Caller fields and methods |
|---|---|
| `Plaque` | A labelled action. Set embedded `Click`, `Kind PlaqueKind`, `Icon *widget.Icon`, `Label string`, `Small bool`, `Busy bool`, and `Preview State`. Kinds are `PlaqueGold`, `PlaqueLine`, `PlaqueQuiet`, `PlaqueOn`, `PlaqueDanger`, and `PlaqueDangerLine`. Call `Layout(gtx, th)` and `Clicked(gtx)`. A busy plaque consumes clicks without reporting them. Like every `kit.Click`, a focused plaque is pressed by Return, keypad Enter or Space. |
| `SplitPlaque` | A main action plus an independent menu action. Set `Main`, `More`, `Kind`, `Icon`, `Label`, `Open`, and `Preview`. Call `Layout(gtx, th)`, `MainClicked(gtx)`, and `MoreClicked(gtx)`. |
| `IconPlaque` | A square icon action. Set embedded `Click`, `Kind`, `Icon`, `Description`, `Small`, `Busy`, `Notch`, and `Preview`. Call `Layout(gtx, th)` and `Clicked(gtx)`. |
| `TextField` | A labelled single-line editor. Set embedded `widget.Editor`, `Label`, `Placeholder`, `Help`, `Error`, `Secret`, `Mono`, and `Preview`. Call `Layout(gtx, th)`, `Focus(gtx)`, and `Revealed()`. Read and write text through the embedded editor methods such as `Text` and `SetText`. |
| `Search` | A clearable single-line search editor. Set embedded `widget.Editor`, `Placeholder`, `Hint`, and `Preview`. Call `Layout(gtx, th)` and `Focus(gtx)`. Escape clears a search with text and keeps focus; an empty search leaves Escape to whatever holds it, so a dialog or popover closes. |
| `Toggle` | A labelled binary switch. Set embedded `Click`, `Value`, `Label`, and `Preview`. Call `Layout(gtx, th)` and read the clear-on-read `Changed() bool` latch. |
| `Slider` | A bounded floating-point value. Set embedded `Click`, `Value`, `Min`, `Max`, `Step`, `Label`, and `Preview`. Call `Layout(gtx, th)` and `Changed() bool`. |
| `Stepper` | A bounded integer value. Set `Value`, `Min`, `Max`, `Step`, `Unit`, and `Preview`. Call `Layout(gtx, th)` and `Changed() bool`. A nonpositive step becomes one. |
| `Filters` | Mutually exclusive filter chips. Set `Labels`, `Counts`, `Selected`, and `Preview`. Call `Layout(gtx, th)` and `Changed() (int, bool)`. |
| `Tabs` | A horizontal selector of up to the token limit. Set `Labels`, `Selected`, and `Preview`. Call `Layout(gtx, th)` and `Changed() (int, bool)`. |
| `Disclosure` | Animated optional content. Set `Open` and `Preview`, then call `Layout(gtx, th, content)`. Closed content is disabled while it leaves. |
| `SettingRow` | One setting in a settings slab. Set `Title`, `Detail`, `Changed`, `Unavailable`, `Highlight float32`, `Activatable`, `First`, and `Preview`, then call `Layout(gtx, th, control)`; the control sits at the right, no wider than `settingsControlMax`. Title and detail wrap and never elide. A row draws the vein line above itself unless `First`. `Changed` adds the "Changed" mark and keeps a slot for the reset icon plaque, shown while the row is hovered or the row or reset holds focus; read `Reset.Clicked(gtx)`. A non-empty `Unavailable` disables the control and replaces the detail. With `Activatable`, the whole row is a pointer and keyboard target: read `Row.Clicked(gtx)` and flip the row's toggle there. The host drives `Highlight` from 1 to 0 over `tokens.MotionHighlight` on the chisel curve, for example with an `anim.Tween`. `Hovered()` reports the pointer. |
| `ChordField` | A keyboard shortcut field. Set embedded `Click`, `Chord`, `Label` (accessible name), `Placeholder` (default "Not set"), `Conflict`, `Preview`, and `PreviewRecording`. Call `Layout(gtx, th)`, `Focus(gtx)`, `Recording() bool`, the clear-on-read `Recorded() (string, bool)` and `Cleared() bool`. The field never changes `Chord` itself and never talks to the compositor; hold a shortcut inhibitor while `Recording` is true. |
| `ColorField` | A colour value field. Set `Color`, `Label` (accessible name), `AllowAlpha`, `Swatches []ColorSwatch` (each a `Label` and a `Color`), and `Preview`; a non-rest preview with `Show()` draws the open popover in place. Call `Layout(gtx, th)`, the clear-on-read `Picked() (color.NRGBA, bool)`, `Open`, `Show`, `Close(gtx)`, `Focus(gtx)`, and `SubmitHex(text)`. A pick also sets `Color`. |
| `PathField`, `PathKind` | A file or folder path in a mono field with a choose action inside it. Set `Path`, `Label`, `Kind` (`PathFolder`, the zero value, or `PathFile`; it only words the default placeholder "Choose a folder" or "Choose a file"), optional `Placeholder`, `Error`, and `Preview` (`StateFocus` for the field, `StateHover` or `StatePressed` for the choose action). Call `Layout(gtx, th)`, `Focus(gtx)`, and the clear-on-read `Edited() (string, bool)`, which reports text committed with Enter or by leaving the field when it differs from `Path`. Read `Choose.Clicked(gtx)`, open the portal dialog and set `Path`; the library never opens one. The host validates and sets `Error`. |
| `ListEditor`, `ListItem` | An editable list inside a settings slab. Set `Label` (accessible name), `AddText` (default "Add"), `DoneText` (Done's label for a new item, default "Add"; an edit says "Done"), `EmptyText`, `Reorderable`, `EditorValid`, `EditorFields []event.Tag`, `Plane` (the surface under the list, the slab when zero), `Preview` (applies to the first row's edit plaque, or its handle when reorderable), and `PreviewDrag` (draws the first row dragged by that distance). Call `Layout(gtx, th, items []ListItem, editor, trailing)`; `editor(gtx, index)` draws the host's fields for item `index` or `ListEditorNew`, `trailing(gtx, index)` an optional control, and either may be nil. The list never changes `items`; read the clear-on-read `Opened() (int, bool)` (load a copy of the item into the editor), `Added() bool`, `Edited() (int, bool)`, `Removed() (int, bool)`, `Moved() (from, to int, ok bool)` and `Cancelled() bool`, then apply them. `Edit(index)`, `Add()`, `Submit() bool`, `Cancel() bool` and `Editing() (int, bool)` drive the editor; one is open at a time. Done is enabled only while `EditorValid`; Enter in a field listed in `EditorFields` means Done and Escape means Cancel. Reorderable rows move with the handle's drag (reported on release) or Alt+Up and Alt+Down on the focused row. Remove is immediate, so offer Undo. |

### ChordField keys

Recording starts on a click, Enter or Space. The next press with Super, Ctrl,
Alt or Shift and a non-modifier key, or a function key alone, records the
canonical chord: modifiers in the order Super, Ctrl, Alt, Shift, then the key,
joined by `+`, with the key named as `Combos.js` `qtKeyName` names it
(`Super+Shift+Return`, `Ctrl+minus`, `Super+Prior`). Escape cancels; Backspace
or Delete alone clears; with a modifier they record. Losing focus cancels.
Keycaps show the faces `ryoku/wm` `DisplayKeys` gives (`Enter`, `Page Up`,
`Num 3`).

Gio reports fewer keys than Qt or a browser, so a Gio host cannot record:

- letters on a non-Latin layout: Gio names a key only for an ASCII keysym, so
  a Cyrillic or Greek letter produces no key event;
- `Print`, `Insert`, `F13` to `F24` and every media key (`XF86Audio*`,
  `XF86MonBrightness*`): Gio has no names for them and sends nothing;
- number-pad digits and `-`, `/`, `.` as `KP_` keys: they arrive as their
  main-row twins (`KP_3` records as `3`); only number-pad Enter (`KP_Enter`),
  `*` (`KP_Multiply`) and `+` (`KP_Add`) without Shift stay distinct.

Gio names the produced symbol rather than the key, so the field folds the US
shifted symbols back (`Shift+!` records as `Shift+1`). The field still shows
any of these chords when the host sets them.

### Control examples

Keep component values in application state. Do not construct them anew on each
frame.

```go
var connect kit.Plaque

func layoutConnect(gtx layout.Context, th *theme.Theme) layout.Dimensions {
    connect.Kind = kit.PlaqueGold
    connect.Label = "Connect"
    if connect.Clicked(gtx) {
        beginConnect()
    }
    return connect.Layout(gtx, th)
}
```

```go
var host kit.TextField

func initHost() {
    host.Label = "Host"
    host.Placeholder = "prowl"
    host.Help = "Fleet name or address"
    host.SetText("ally")
}

func layoutHost(gtx layout.Context, th *theme.Theme) layout.Dimensions {
    return host.Layout(gtx, th)
}
```

## Lists, containers, state, and progress reference

| Component | Caller fields and methods |
|---|---|
| `Tablet` | An interactive resource row. Set embedded `Click`, `Pane color.NRGBA`, and `Preview`. Pass `TabletContent` to `Layout(gtx, th, content, selected, listFocused)`. Call `Clicked(gtx)` or `DoubleClicked(gtx)`. When focused, Space selects (`Clicked`) and Return or keypad Enter selects and runs the primary action, reporting both `Clicked` and `DoubleClicked` like a double click. |
| `TabletContent` | Set `Mark layout.Widget`, `Title`, `Facts`, `FactsWidget`, `FactsMono`, `Claim`, `Proof`, `Tone`, `Progress`, and `Action *Plaque`. `FactsWidget` replaces the plain facts string when supplied. |
| `RowList` | A scrolling keyboard list with one selection slab. Set `Preview`. Call `Focus(gtx)`, `Layout(gtx, th, count, selected, row)`, `Changed() (int, bool)`, and `Activated() (int, bool)`. Both result methods report their current ready flag. |
| `ScrollRail` | The RowList thumb for any vertical `layout.List` pane: `edge` at rest, `fg3` on hover or drag, shown only while the content overflows. Set `Preview`. After the list's layout, call `Layout(gtx, th, list, count)` in the same bounds; the rail draws at the right edge and hit-tests a lane `s3` wide there. Dragging the thumb scrolls and a press on the track pages; the wheel and the keys stay with the pane. `Dragging()` reports a held thumb. |
| `DesignTile` | One look in a gallery of looks. Set embedded `Click`, `Title`, `Status`, `Resource Mark` (`Attention` draws Heavy in amber), `Image paint.ImageOp` (the zero value draws a hatch), `Current`, `Selected`, `Busy`, `Unavailable`, `ActionText`, `ActionGold`, `ActionIcon`, and `Preview`. Call `Layout(gtx, th)` at the tile width, `Clicked(gtx)` and `ActionTriggered(gtx)`. The preview is 16:10 (`tokens.SizeDesignPreviewRatio`), cropped to fill; the second line always reserves a small plaque's height so tiles in a row align. The action is a small line plaque, an on plaque while the tile is current or selected, and gold only with `ActionGold`. |
| `DesignGallery` | A grid of design tiles: as many columns as `catalogueTileWidthMin` tiles allow with `s2` between, so the focus ring's outset never touches a neighbour, tiles up to `catalogueTileWidthMax`. Set `Preview` (`StateFocus` rings the selected tile). Call `Layout(gtx, th, tiles, selected)`, `Changed() (int, bool)`, `Activated() (int, bool)`, `Focus(gtx, tiles, selected)`, and `Columns()`. It sets each tile's `Selected` and reads its clicks, so use `Changed` and `Activated` rather than `Clicked`; arrow keys, Home and End move the selection and focus, Enter or Space activates. |
| `GroupHeader` | `GroupHeader(th, title, count)` returns a widget. A nonpositive count renders nothing. |
| `Skeleton` | Static list placeholders. Set `Active` and `Rows`, then call `Layout(gtx, th)`. An inactive skeleton occupies no space and requests no frames. |
| `Empty` | An empty state. Set `Icon`, `Title`, `Body`, and `Actions []*Plaque`, then call `Layout(gtx, th)`. |
| `Pane` | A flat application plane. Set `Kind` to `PaneList` or `PaneWork`, then call `Layout(gtx, th, body)`. |
| `MineralField` | An animated tone-derived plane with no caller fields. Call `Layout(gtx, th, tone, body)`. |
| `Header` | Owns the mineral field behind a selected resource. Pass `HeaderContent` to `Layout(gtx, th, content)`. |
| `HeaderContent` | Set `Tone`, `Mark`, `Name`, `Progress`, `Proof []Span`, `Note`, `Marks []Mark`, `Actions []*Plaque`, and `Below`. |
| `Slab` | `Slab(gtx, ink, cut, body)` is the general painted container. `SlabContent(gtx, th, body)` supplies the standard stone1 plane and inset. |
| `Section` | `Section(th, title, body)` returns a titled widget with canonical gaps. |
| `Fact`, `Facts` | A `Fact` has `Label`, `Value`, and `Mono`. `Facts(th, []Fact)` returns a flat label and value table: labels keep the 140 column, values start `s4` after it, either side wraps, and a row grows past 44 to fit its taller side. |
| `Aside` | `Aside(gtx, th, title, rows...)` lays out a live draft summary. Build rows with `AsideFact`, `AsideNote`, and `AsidePreview`. |
| `DraftColumns` | `DraftColumns(gtx, th, form, aside)` stacks or splits the two widgets at the canonical breakpoint. |
| `Claim` | `Claim(th, tone, word)` returns a tone flake and plain state word. |
| `Mark`, `Marks` | A `Mark` has `Text` and `Attention`. `Marks(th, marks)` returns the trait widget. `ResourceMarks(disposable, instant, sealed)` returns the canonical order. `MarkedFacts(th, facts, factsMono, marks, ink)` composes facts and traits with whole-mark truncation. |
| `Span`, `Sentence` | A `Span` has `Text`, `Mono`, and optional `Color`. `Sentence(gtx, th, style, ink, spans)` composes the runs on one baseline. |
| `BladeMeter` | A stable meter. Set `Fraction` and optional `Ink`, then call `Layout(gtx, th)`. Fractions at or above the token alert threshold use amber. |
| `BladeProgress` | Job progress. Set `Fraction` and `Active`, then call `Layout(gtx, th)`. A negative fraction is indeterminate and requests sweep frames only while active. |
| `Working` | A three-flake work mark. Set `Active` and optional `Size`, then call `Layout(gtx, th)`. |
| `BecomingName` | A name filled by creation progress. Set `Name`, `Fraction`, `Tone`, and `Compact`, then call `Layout(gtx, th)`. |

### Tablet and row list examples

```go
var machine kit.Tablet

func layoutMachine(gtx layout.Context, th *theme.Theme) layout.Dimensions {
    content := kit.TabletContent{
        Title: "ally", Facts: "ops@192.0.2.18", FactsMono: true,
        Claim: "Running", Proof: "prowl", Tone: palette.Alive,
        Progress: -1,
    }
    if machine.DoubleClicked(gtx) {
        openMachine("ally")
    }
    return machine.Layout(gtx, th, content, true, true)
}
```

```go
type fleetList struct {
    rows     []kit.Tablet
    list     kit.RowList
    selected int
    focused  bool
}

func (f *fleetList) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
    dimensions := f.list.Layout(gtx, th, len(f.rows), f.selected,
        func(gtx layout.Context, index int) layout.Dimensions {
            return f.rows[index].Layout(gtx, th, kit.TabletContent{
                Title: "Machine", Tone: palette.Alive, Claim: "Running",
            }, index == f.selected, f.focused)
        })
    if selected, ok := f.list.Changed(); ok {
        f.selected = selected
    }
    return dimensions
}
```

Pass the application's own list-focused state as the final `Tablet.Layout`
argument.

## Overlays, navigation, data, and window reference

| Component | Caller fields and methods |
|---|---|
| `Popover` | An anchored overlay. Set `Open`, optional focus-return `Opener`, and `Preview`. Call `Layout(gtx, anchor, content)` and `Close(gtx)`. It takes keyboard focus when it opens and brings focus back only when it leaves both the popover and its content; kit clickables and fields inside the content keep it, and so does a popover opened inside its content, which leaves the outer one open. Escape closes it after its content had the chance to use the key (an inner open menu, a Search with text). |
| `MenuItem` | Set `Icon`, `Label`, `Shortcut`, `Danger`, `Disabled`, `Divider`, and `Preview`. |
| `Menu` | A cut command list. Set `Preview`, pass items to `Layout(gtx, th, items)`, and consume `Clicked(gtx) (int, bool)`. Inside a Popover it starts keyboard focus on its first enabled item; Up and Down move it, wrapping and skipping dividers and disabled items, Home and End go to the ends, Return, Enter or Space chooses, and Escape closes the popover without choosing, returning focus to its opener. A menu opened with the pointer shows no keyboard ring on that start item until a key moves focus; every key move draws it. `FocusItem(gtx, index)` moves focus with the ring, for a host that opens a menu from the keys. |
| `Picker` | A labelled field and popover menu. Set `Preview` and `PreviewKeys` (an open preview drawn as opened from the keys, with the ring on the current choice). Call `Focus`, `Open`, `Show`, `Close`, `Layout(gtx, th, label, options, selected)`, and `Changed(gtx) (int, bool)`. A click, or Space, Return or Down on the anchor, opens it with focus on the current choice; only a key open shows the keyboard ring there. Choosing or Escape returns focus to the anchor. |
| `Dialog` | A protected deferred surface. Set `Open`, optional `Width`, optional focus-return `Opener`, and `Preview`. Call `Layout(gtx, th, title, body, actions...)`, `Dismiss(gtx)`, and the clear-on-read `Closed() bool`. A dialog that opens with nothing inside it focused moves focus to its first enabled kit control on the next frame; one whose content focuses its own field on open keeps that. For a `Confirm` that is the typed-name field when one is required, otherwise Cancel, so Return on a fresh Confirm cancels and Tab then Return confirms. Escape closes it after its content had the chance to use the key, and focus returns to `Opener`. |
| `Confirm` | A dialog with cancel, action, and optional typed proof. Set `Dialog`, `Danger`, `CancelText` (the cancel plaque's label; empty means "Cancel", so a keep-or-revert dialog can say "Keep"), and `Preview`. Call `SetConfirmationText`, `ConfirmationText`, `Layout(gtx, th, title, body, action, typeToConfirm) bool`, and `Closed() bool`. `Layout` returns true for the accepted action. |
| `Tooltip` | Delayed pointer detail. Set `Preview`, then call `Layout(gtx, th, tip, widget)`. |
| `Placard` | A bottom-edge notice stack. Set `Preview`. `Push(now, text, action)` returns an identity, `Dismiss(id)` removes it, `Layout(gtx, th)` draws the stack, and `Action(gtx) (int, bool)` returns an activated identity. |
| `BrandMark`, `BrandMarkWith`, `BrandMarkFunc` | `BrandMark(th, size)` returns the Home mark widget. `BrandMarkWith(th, size, mark)` draws a host's own `BrandMarkFunc func(gtx, size unit.Dp) layout.Dimensions` in place of the octagon and pointer: the library keeps the `sel` square, size and cut, and centres the mark in the square inset by its cut, clipped there; the host draws in `onSel` with at most one `jasper` accent. A nil mark draws `BrandMark`. |
| `NewPlaque` | The rail's sole gold action. Set embedded `Click` and `Preview`, then call `Layout(gtx, th, compact, open)`. Consume activation through the embedded `Clicked(gtx)`. |
| `RailDestination` | Set `Label`, `Icon`, and `Notch`. |
| `Rail` | Main navigation. Set `Preview`, `NewOpen`, `HideNew`, `FootIndex`, and `Brand BrandMarkFunc`. Destinations before `FootIndex` stack from the top with New after the first one; destinations from `FootIndex` on keep reading order as one group anchored to the foot. Zero means the last destination alone, today's layout; a value equal to the count puts none at the foot. `HideNew` drops New from the rail and from the compact bar, whose columns are Home, the destinations and New. Mark a destination with `RailDestination.Notch`. `Brand` draws the host's mark in Home, expanded and compact; nil draws the octagon. In the expanded rail every tile takes the widest label plus `s1` each side, at least the 64 tile and at most the rail less `s1` each side (80); a label wider than that wraps to two centred lines at word boundaries, breaking inside a word only when one word is too long, and its tile grows by one caption line. Medium and compact are unchanged. Call `Layout(gtx, th, destinations, selected, class)`, `HomeClicked()`, `NewClicked()`, and `Selected() (int, bool)`. |
| `SectionIndex` | Settings-page navigation. Set `Preview`. Call `Layout(gtx, th, labels, current, class)` and `Selected() (int, bool)`. It renders rows in `FrameExpanded`, horizontal chips in `FrameMedium`, and no index in `FrameCompact`. |
| `EngravedChart` | A polyline over a clipped hatch. Set `Values`, `Min`, `Max`, `Height`, `Ticks`, and `Empty`, then call `Layout(gtx, th)`. Use `math.NaN()` for an automatically derived bound. |
| `AppFrame` | The window composition root. `Class` is the resolved output and `Preview` forces overlay preview. `HideCompactBar` leaves the compact class without the bottom rail so the host navigates on its own. The host owns its `Rail` and passes its layout as `AppFrameContent.Rail`, so a host mark goes on `Rail.Brand`; the frame draws no mark of its own. Call `Layout(gtx, th, AppFrameContent)`. `DismissOverlay` hears Escape only after the frame's content (an open menu, a dialog, a Search with text, a ListEditor field) had the chance to use it. |
| `AppFrameContent` | Set `Rail`, `List`, `Work`, `Overlays`, `WindowControls`, and `DismissOverlay func() bool`. |
| `FrameClass` | `FrameCompact`, `FrameMedium`, or `FrameExpanded`. `ClassForWidth(width)` resolves the token breakpoints. |

### Overlay examples

```go
var help kit.Popover
var helpButton kit.Plaque

func layoutHelp(gtx layout.Context, th *theme.Theme) layout.Dimensions {
    helpButton.Kind = kit.PlaqueLine
    helpButton.Label = "Details"
    if helpButton.Clicked(gtx) {
        help.Open = !help.Open
        help.Opener = &helpButton.Clickable
    }
    return help.Layout(gtx,
        func(gtx layout.Context) layout.Dimensions {
            return helpButton.Layout(gtx, th)
        },
        kit.Text(th, th.T.Body, th.R.Fg, "Connection details"),
    )
}
```

```go
var decision kit.Dialog
var cancel kit.Plaque
var save kit.Plaque

func layoutDecision(gtx layout.Context, th *theme.Theme) layout.Dimensions {
    cancel.Kind, cancel.Label = kit.PlaqueLine, "Cancel"
    save.Kind, save.Label = kit.PlaqueGold, "Save"
    if cancel.Clicked(gtx) || save.Clicked(gtx) {
        decision.Dismiss(gtx)
    }
    return decision.Layout(gtx, th, "Save changes",
        kit.Text(th, th.T.Body, th.R.Fg2, "Apply these settings?"),
        func(gtx layout.Context) layout.Dimensions { return cancel.Layout(gtx, th) },
        func(gtx layout.Context) layout.Dimensions { return save.Layout(gtx, th) },
    )
}
```

```go
var frame kit.AppFrame

func layoutWindow(gtx layout.Context, th *theme.Theme) layout.Dimensions {
    return frame.Layout(gtx, th, kit.AppFrameContent{
        Rail: layoutRail,
        List: layoutFleet,
        Work: layoutMachine,
        Overlays: layoutOverlays,
        DismissOverlay: dismissTopOverlay,
    })
}
```

## Writing a component

A component is a value type that owns its interaction and motion state. Every
clickable uses `kit.Click`, not a bare `widget.Clickable`. Use palette roles and
token-backed theme values rather than local colours, spacing, cuts, type,
alpha, or durations. Set a tween only when its target changes. Call
`anim.Phase` only while visible work is active. Add semantics and an accessible
label, and keep the focus ring keyboard-only.

## Tokens and commands

`tokens.json` at the Lithic root is the only source for generated values. From
the Go module directory:

```sh
go run ./cmd/lithic-tokens
go run ./cmd/lithic-tokens -check
go run ./cmd/lithic-roles --mode dark
```

The first command regenerates stack outputs. The second checks without writing.
The roles command prints the roles resolved on the current machine. Do not
hand-edit generated token files.

## Gallery

The runnable Gio reference is in `../../examples/gio` and has the four pages
`foundation`, `controls`, `lists`, and `overlays`. Its README lists the window
and PNG commands.
