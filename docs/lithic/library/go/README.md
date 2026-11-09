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
    "github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
    "github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
    "github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
    "github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)
```

A normal Gio setup is:

```go
roles := palette.FromPalette(palette.Load(palette.Follow))
th := theme.New(roles)
```

Pass `theme.WithFontCollection(faces)` when the application embeds font faces. System fonts remain enabled, and the typefaces fall through the ordered `tokens.FontSans` and `tokens.FontMono` family lists. Pass `theme.WithShaper(shaper)` when the application owns a configured shaper.

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
- `FromPalette(Palette) Roles`: maps source roles into all Lithic roles.
- `Watch(ctx, mode, fn)`: watches the three parent directories with inotify, coalesces file replacement, and calls `fn` only when the resolved palette changes. `mode` is a function so an application can change its mode without rebuilding the watcher.
- `Tone`: `Idle`, `Alive`, `Becoming`, `Attention`, `Paused`, `Failed`.
- `Roles`: `Ground`, `Stone1`, `Stone2`, `Stone3`, `Stone4`, `Vein`, `Fg`, `Fg2`, `Fg3`, `Sel`, `OnSel`, `OnSel2`, `Gold`, `OnGold`, `GoldInk`, `Jade`, `Lapis`, `Amber`, `Jasper`, `Scrim`, `Dark`, `Source`.
- `Roles.Stone(tone)`: returns the mineral ink for a tone.
- `Roles.Field(tone)`: returns the derived mineral field for a tone.
- `Signature(dark)`: returns the generated signature fallback.
- `Mix(a, b, t)`: linearly blends every channel.
- `WithAlpha(color, alpha)`: multiplies the existing alpha.
- `InkOn(background)`: picks black or white by contrast.
- `ContrastRatio(a, b)`: returns the WCAG contrast ratio.

Forced dark and light modes retain signature neutrals and use only the source accents. `GoldInk` and `Jasper` are re-toned to the generated minimum contrast against `Stone2`. Fixed minerals always come from tokens.

## Theme API

`theme.Theme` contains `R palette.Roles`, `T Type`, `Cut`, `Sp Space`, `Motion`, `Density`, and `Shaper`. `New(roles, options...)` fills every scale from `tokens`.

Exported API:

- `TextStyle`: `Face`, `Size`, `Line`.
- `Type`: `Name`, `Sentence`, `Section`, `Title`, `Row`, `Body`, `Proof`, `Label`, `Caption`, `Group`, `Mono`, `MonoSm`.
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
| `Icon` | `Icon(data []byte) *widget.Icon` decodes a shiny Material Design icon and panics for invalid data. |
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
| `Plaque` | A labelled action. Set embedded `Click`, `Kind PlaqueKind`, `Icon *widget.Icon`, `Label string`, `Small bool`, `Busy bool`, and `Preview State`. Kinds are `PlaqueGold`, `PlaqueLine`, `PlaqueQuiet`, `PlaqueOn`, `PlaqueDanger`, and `PlaqueDangerLine`. Call `Layout(gtx, th)` and `Clicked(gtx)`. A busy plaque consumes clicks without reporting them. |
| `SplitPlaque` | A main action plus an independent menu action. Set `Main`, `More`, `Kind`, `Icon`, `Label`, `Open`, and `Preview`. Call `Layout(gtx, th)`, `MainClicked(gtx)`, and `MoreClicked(gtx)`. |
| `IconPlaque` | A square icon action. Set embedded `Click`, `Kind`, `Icon`, `Description`, `Small`, `Busy`, `Notch`, and `Preview`. Call `Layout(gtx, th)` and `Clicked(gtx)`. |
| `TextField` | A labelled single-line editor. Set embedded `widget.Editor`, `Label`, `Placeholder`, `Help`, `Error`, `Secret`, `Mono`, and `Preview`. Call `Layout(gtx, th)`, `Focus(gtx)`, and `Revealed()`. Read and write text through the embedded editor methods such as `Text` and `SetText`. |
| `Search` | A clearable single-line search editor. Set embedded `widget.Editor`, `Placeholder`, `Hint`, and `Preview`. Call `Layout(gtx, th)` and `Focus(gtx)`. |
| `Toggle` | A labelled binary switch. Set embedded `Click`, `Value`, `Label`, and `Preview`. Call `Layout(gtx, th)` and read the clear-on-read `Changed() bool` latch. |
| `Slider` | A bounded floating-point value. Set embedded `Click`, `Value`, `Min`, `Max`, `Step`, `Label`, and `Preview`. Call `Layout(gtx, th)` and `Changed() bool`. |
| `Stepper` | A bounded integer value. Set `Value`, `Min`, `Max`, `Step`, `Unit`, and `Preview`. Call `Layout(gtx, th)` and `Changed() bool`. A nonpositive step becomes one. |
| `Filters` | Mutually exclusive filter chips. Set `Labels`, `Counts`, `Selected`, and `Preview`. Call `Layout(gtx, th)` and `Changed() (int, bool)`. |
| `Tabs` | A horizontal selector of up to the token limit. Set `Labels`, `Selected`, and `Preview`. Call `Layout(gtx, th)` and `Changed() (int, bool)`. |
| `Disclosure` | Animated optional content. Set `Open` and `Preview`, then call `Layout(gtx, th, content)`. Closed content is disabled while it leaves. |

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
| `Tablet` | An interactive resource row. Set embedded `Click`, `Pane color.NRGBA`, and `Preview`. Pass `TabletContent` to `Layout(gtx, th, content, selected, listFocused)`. Call `Clicked(gtx)` or `DoubleClicked(gtx)`. |
| `TabletContent` | Set `Mark layout.Widget`, `Title`, `Facts`, `FactsWidget`, `FactsMono`, `Claim`, `Proof`, `Tone`, `Progress`, and `Action *Plaque`. `FactsWidget` replaces the plain facts string when supplied. |
| `RowList` | A scrolling keyboard list with one selection slab. Set `Preview`. Call `Focus(gtx)`, `Layout(gtx, th, count, selected, row)`, `Changed() (int, bool)`, and `Activated() (int, bool)`. Both result methods report their current ready flag. |
| `GroupHeader` | `GroupHeader(th, title, count)` returns a widget. A nonpositive count renders nothing. |
| `Skeleton` | Static list placeholders. Set `Active` and `Rows`, then call `Layout(gtx, th)`. An inactive skeleton occupies no space and requests no frames. |
| `Empty` | An empty state. Set `Icon`, `Title`, `Body`, and `Actions []*Plaque`, then call `Layout(gtx, th)`. |
| `Pane` | A flat application plane. Set `Kind` to `PaneList` or `PaneWork`, then call `Layout(gtx, th, body)`. |
| `MineralField` | An animated tone-derived plane with no caller fields. Call `Layout(gtx, th, tone, body)`. |
| `Header` | Owns the mineral field behind a selected resource. Pass `HeaderContent` to `Layout(gtx, th, content)`. |
| `HeaderContent` | Set `Tone`, `Mark`, `Name`, `Progress`, `Proof []Span`, `Note`, `Marks []Mark`, `Actions []*Plaque`, and `Below`. |
| `Slab` | `Slab(gtx, ink, cut, body)` is the general painted container. `SlabContent(gtx, th, body)` supplies the standard stone1 plane and inset. |
| `Section` | `Section(th, title, body)` returns a titled widget with canonical gaps. |
| `Fact`, `Facts` | A `Fact` has `Label`, `Value`, and `Mono`. `Facts(th, []Fact)` returns a flat label and value table. |
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
| `Popover` | An anchored overlay. Set `Open`, optional focus-return `Opener`, and `Preview`. Call `Layout(gtx, anchor, content)` and `Close(gtx)`. |
| `MenuItem` | Set `Icon`, `Label`, `Shortcut`, `Danger`, `Disabled`, `Divider`, and `Preview`. |
| `Menu` | A cut command list. Set `Preview`, pass items to `Layout(gtx, th, items)`, and consume `Clicked(gtx) (int, bool)`. |
| `Picker` | A labelled field and popover menu. Set `Preview`. Call `Focus`, `Open`, `Show`, `Close`, `Layout(gtx, th, label, options, selected)`, and `Changed(gtx) (int, bool)`. |
| `Dialog` | A protected deferred surface. Set `Open`, optional `Width`, optional focus-return `Opener`, and `Preview`. Call `Layout(gtx, th, title, body, actions...)`, `Dismiss(gtx)`, and the clear-on-read `Closed() bool`. |
| `Confirm` | A dialog with cancel, action, and optional typed proof. Set `Dialog`, `Danger`, and `Preview`. Call `SetConfirmationText`, `ConfirmationText`, `Layout(gtx, th, title, body, action, typeToConfirm) bool`, and `Closed() bool`. `Layout` returns true for the accepted action. |
| `Tooltip` | Delayed pointer detail. Set `Preview`, then call `Layout(gtx, th, tip, widget)`. |
| `Placard` | A bottom-edge notice stack. Set `Preview`. `Push(now, text, action)` returns an identity, `Dismiss(id)` removes it, `Layout(gtx, th)` draws the stack, and `Action(gtx) (int, bool)` returns an activated identity. |
| `BrandMark` | `BrandMark(th, size)` returns the Home mark widget. |
| `NewPlaque` | The rail's sole gold action. Set embedded `Click` and `Preview`, then call `Layout(gtx, th, compact, open)`. Consume activation through the embedded `Clicked(gtx)`. |
| `RailDestination` | Set `Label`, `Icon`, and `Notch`. |
| `Rail` | Main navigation. Set `Preview` and `NewOpen`. Call `Layout(gtx, th, destinations, selected, class)`, `HomeClicked()`, `NewClicked()`, and `Selected() (int, bool)`. |
| `SectionIndex` | Settings-page navigation. Set `Preview`. Call `Layout(gtx, th, labels, current, class)` and `Selected() (int, bool)`. It renders rows in `FrameExpanded`, horizontal chips in `FrameMedium`, and no index in `FrameCompact`. |
| `EngravedChart` | A polyline over a clipped hatch. Set `Values`, `Min`, `Max`, `Height`, `Ticks`, and `Empty`, then call `Layout(gtx, th)`. Use `math.NaN()` for an automatically derived bound. |
| `AppFrame` | The window composition root. `Class` is the resolved output and `Preview` forces overlay preview. Call `Layout(gtx, th, AppFrameContent)`. |
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
