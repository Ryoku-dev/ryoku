# Lithic for terminals

Package `term` is the terminal subset of Lithic. It renders with Lip Gloss v2 and exposes Bubble Tea v2 models for keyboard interaction. Every component also has a pure `Render<Name>` function for documentation, logging, and static output.

## Setup

```go
import (
    "github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
    "github.com/ryoku-dev/ryoku/docs/lithic/library/go/term"
)

th := term.NewTheme(palette.Dark)
view := term.RenderPlaque(th, term.Plaque{Label: "Connect", Kind: term.PlaqueGold})
```

The parent Go module requires these terminal dependencies:

```go
require (
    charm.land/bubbletea/v2 v2.0.2
    charm.land/lipgloss/v2 v2.0.6
)
```

`Theme` carries the `palette.Roles` returned by `palette.FromPalette(palette.Load(mode), palette.Look{})`. `palette.Dark`, the default, and `palette.Light` are the signature; pass `palette.Follow` only when the reader chooses to follow the wallpaper. An application that offers contrast and tint builds `term.Theme{Roles: palette.FromPalette(palette.Load(mode), look)}` with the reader's `palette.Look`. Terminal applications choose whether to paint the `ground` role.

## Glyph sets

The Unicode set is the default. When `TERM=linux`, the package selects the console set because the kernel console's default font does not contain the diagonal and parallelogram marks. `LITHIC_GLYPHS=console|unicode` overrides detection. An application may call `SetGlyphMode(GlyphConsole|GlyphUnicode)` before rendering. `Glyphs()` reports the selected `GlyphSet`. Lip Gloss handles the `TERM=linux` downsampling of palette roles to 16 ANSI colours.

| Part | Unicode | Console |
|---|---|---|
| Frame | `╱─╲`, `│`, `╲─╱` | `┌─┐`, `│`, `└─┘` |
| Notch | `◤` in the tone colour | `▀` in the tone colour |
| Active flake | `▰` in the tone colour | `▬` in the tone colour |
| Idle flake | `▱` in `fg3` | `▬` in `fg3` |
| Becoming flake | `▰▱` in `lapis` | `▬` in `lapis` |
| Blade | `━` fill, `─` vein track, `╸` leading edge | `─` fill over a `─` vein track |
| Working | Three flakes fill in turn | `░`, `▒`, `▓`, `▒` |
| Arrow and chevron | `›`, `▸` | `>`, `►` |
| Focus cursor | `▸` | `►` |
| Toggle knob | `■` | `■` |
| Secret mask | `•` | `•` |

Components read the selected `GlyphSet`; they do not carry private glyph fallbacks.

The exported glyph API is `GlyphMode`, `GlyphUnicode`, `GlyphConsole`,
`GlyphSet`, `SetGlyphMode`, and `Glyphs`. `CutBorder()` returns the currently
selected glyph set as a Lip Gloss border.

## Translation

- A cut frame is a custom Lip Gloss border. Unicode corners are diagonals. Console corners are square. Rounded and double borders are not used.
- Frames belong to fields, Search, Picker, Menu, Dialog, and Confirm. Empty frames only its mark, while Tablet and MineralField are unframed planes. Becoming Tablet progress occupies the following line.
- Plane renderers pad every inner line to their declared width so background colour never ends at the text.
- Tablets and dialogs use `stone2`, raised controls use `stone3`, menus use `stone4`, and quiet slabs use `stone1`. Only selected and on states use `sel` with `onSel`.
- Focus never uses `sel`. Fields, Search, and Picker change their frame from `vein` to `fg2`. Other focusable components reserve a two-cell gutter for the selected glyph-set cursor in `fg` and make the focused label bold.
- An error TextField uses `jasper` for both its frame and help text, including when nested in a dialog plane.
- Only `PlaqueGold` paints a gold background with `onGold` text. Other plaques use brackets, selection, danger, or a raised `stone3` plane.
- Disabled content uses `fg3`. The terminal has no hover state.
- Names, sentences, sections, and titles are bold. Other levels use `fg`, `fg2`, and `fg3`, never faint text.
- State combines a mineral colour, a notch or flake, and a word. It never relies on colour alone. Toggle uses a track and square knob instead of a flake.
- `WorkingModel` returns a `tea.Tick` command only while `Working.Working` is true. `RenderWorking` always renders one frozen frame.

## Spacing in cells

Terminal spacing maps the generated spacing tokens to cells in one table. Horizontal padding is per side. Vertical padding is per edge.

| Token | Dp | Horizontal cells | Vertical rows |
|---|---:|---:|---:|
| `tokens.SpaceS1` | 4 | 1 | 0 |
| `tokens.SpaceS2` | 8 | 1 | 0 |
| `tokens.SpaceS3` | 13 | 1 | 1 |
| `tokens.SpaceS4` | 21 | 2 | 1 |
| `tokens.SpaceS5` | 34 | 2 | 1 |
| `tokens.SpaceS6` | 55 | 2 | 1 |

## Pure render API

Foundation:

- `RenderNotch(Theme, palette.Tone) string`
- `RenderBlade(Theme, width, fraction, palette.Tone) string`
- `RenderFlake(Theme, palette.Tone, fraction) string`

Controls:

- `RenderPlaque(Theme, Plaque) string`. `PlaqueKind` is `PlaqueQuiet`, `PlaqueGold`, `PlaqueLine`, `PlaqueOn`, `PlaqueDanger`, or `PlaqueDangerLine`.
- `RenderSplitPlaque(Theme, SplitPlaque) string`
- `RenderTextField(Theme, TextField) string`
- `RenderSearch(Theme, Search) string`
- `RenderToggle(Theme, Toggle) string`
- `RenderStepper(Theme, Stepper) string`
- `RenderFilters(Theme, Filters) string`, with `[]Filter`
- `RenderTabs(Theme, Tabs) string`
- `RenderDisclosure(Theme, Disclosure) string`

Lists, containers, state, and progress:

- `RenderTablet(Theme, Tablet) string`
- `RenderRowList(Theme, RowList) string`
- `RenderGroupHeader(Theme, GroupHeader) string`
- `RenderEmpty(Theme, Empty) string`
- `RenderMineralField(Theme, MineralField) string`
- `RenderSlab(Theme, Slab) string`
- `RenderSection(Theme, Section) string`
- `RenderFacts(Theme, Facts) string`, with `[]Fact`; `Facts.Width` bounds the table so values wrap too
- `RenderAside(Theme, Aside) string`
- `RenderDraftColumns(Theme, DraftColumns) string`
- `RenderClaim(Theme, Claim) string`
- `RenderMarks(Theme, Marks) string`
- `RenderSentence(Theme, Sentence) string`, with `[]Span` and `SpanText|SpanLiteral`
- `RenderBladeMeter(Theme, BladeMeter) string`
- `RenderBladeProgress(Theme, BladeProgress) string`
- `RenderWorking(Theme, Working) string`

Overlays:

- `RenderMenu(Theme, Menu) string`, with `[]MenuItem` and `MenuItemNormal|MenuItemDanger|MenuItemDivider`
- `RenderPicker(Theme, Picker) string`
- `RenderDialog(Theme, Dialog) string`
- `RenderConfirm(Theme, Confirm) string`
- `RenderPlacard(Theme, Placard) string`

## Bubble Tea models

`TextFieldModel`, `SearchModel`, `ToggleModel`, `StepperModel`, `FiltersModel`, `TabsModel`, `DisclosureModel`, `RowListModel`, `MenuModel`, `PickerModel`, `DialogModel`, and `ConfirmModel` implement `tea.Model`. Each owns the corresponding exported component value plus the result flags needed by its caller. `WorkingModel` also implements `tea.Model` for working-only ticks.

`NewTextFieldModel(Theme, TextField)` and `NewSearchModel(Theme, Search)`
create focused editing models. The other model structs are ready to use as
values once their `Theme` and component fields are set.

- Text editing accepts printable keys, Backspace, Ctrl+H, Ctrl+U, and Space. Ctrl+R reveals or masks a secret TextField. Escape clears Search and releases TextField focus. Enter sets `TextFieldModel.Submitted`.
- Toggle and Disclosure use Space or Enter.
- Stepper uses Left, Right, Up, Down, `-`, and `+`.
- Filters and Tabs use Left and Right, with Home and End.
- RowList and Menu use Up and Down, Home and End, and Enter. Escape cancels Menu.
- Picker opens with Enter or Space, moves with Up and Down, chooses with Enter, and closes with Escape.
- Dialog keeps action focus on Left, Right, Tab, and Shift+Tab. Enter chooses an enabled action and Escape cancels.
- Confirm edits the required name while its field has focus, moves focus with Tab, accepts only an exact name, and cancels with Escape.

Models expose result fields such as `Changed`, `Activated`, `Chosen`, `Submitted`, `Accepted`, and `Cancelled`. A caller reads the concrete model returned from `Update` and handles the result in its application update loop.
