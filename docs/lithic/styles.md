# Styles and colours

Every Ryoku app and every Ryoku surface wears the desktop's look. The reader
chooses it once, in **Settings › Appearance › Style**, and everything Ryoku
draws follows at once: Settings, the shell, the Kit, the store, Rashin and any
app made for Ryoku. This page is the rule, and how to meet it in each stack.

## The rule

An app made for Ryoku must:

1. **Draw in all four styles**: Lithic, Material, Adwaita and Fluent.
2. **Offer both colour sources**: Ryoku's own colours, which are the default,
   and the desktop's colours, from the wallpaper or a fixed scheme.
3. **Work in light and in dark**, in both colour sources.
4. **Honour Contrast** (standard or high) **and Tint** (Lithic on the
   wallpaper's colours only).
5. **Follow a change live**, without a restart.
6. **Keep no copy of the choice.** The app reads the desktop's choice and never
   offers its own style or colour picker. Where it wants one, it links to
   Settings › Style (`ryoku-shell hub open style`).

Two things never change with the style or the colours: the 力 seal, and the
three state minerals (jade, lapis, amber). See [design.md](design.md).

## The four styles

A style changes **shape and surfaces**. It never changes the colours: the same
colours, in four shapes.

| Style | Corners | Controls | Surfaces |
|---|---|---|---|
| **Lithic** (default) | Cut at 45 degrees | Cut plaques, cut toggle | One paper; every pane and group drawn by a 1 dp hairline |
| **Material** | Round, large (cut × 2.6, at most 28) | Pills (anything up to 56 tall) | The page a tone up from the ground, its cards a tone more; no lines |
| **Adwaita** | Round, medium (cut × 1.6, at most 14) | Pills up to 26 tall, soft buttons | The sidebar set apart, the page on the ground, cards on it; no lines |
| **Fluent** | Round, small (cut × 0.75, at most 8) | Pills up to 22 tall, small-cornered buttons | Layered planes, each with a thin outline |

You never compute a corner or a plane yourself. Write the **Lithic cut** a
shape would have (the cut scale in [design.md](design.md#5-shape-the-cut-scale)),
and the library turns it into the style's corner. The same goes for planes:
ask the library for them.

## The colours

| Setting | Choices | What it does |
|---|---|---|
| **Colours** | Ryoku (default), Wallpaper | Ryoku: the signature, black paper, bone ink and the sun as the one action (warm grey paper in light). Wallpaper: the desktop's palette, a fixed scheme if one is set, else the wallpaper's. |
| **Light or dark** | Dark, Light, Automatic, Sun | The whole desktop's. Automatic follows the wallpaper's brightness, Sun the time of day. Ryoku's colours take it too. |
| **Contrast** | Standard, High | High pushes text and lines further from their planes. |
| **Tint** | Off, Low, Medium | How much of the wallpaper's hue Lithic's surfaces take. Only used in Lithic on the wallpaper's colours. |

On the wallpaper's colours, Lithic keeps its own fixed lightness ladder and
only takes the palette's hue, so it never loses contrast; the other styles take
the palette's Material roles as they are. [palette.md](palette.md) holds the
exact recipe for both.

## Where the choice lives

Apps only read these. Settings writes them through the shell daemon, which
validates every value; an app never writes them.

| File | Key | Meaning |
|---|---|---|
| `~/.config/ryoku/shell.json` | `theme.design` | `lithic`, `material`, `adwaita` or `fluent` |
| `~/.config/ryoku/shell.json` | `theme.colours` | `ryoku` or `wallpaper` |
| `~/.config/ryoku/shell.json` | `theme.contrast` | `standard` or `high` |
| `~/.config/ryoku/shell.json` | `theme.tint` | `off`, `low` or `medium` |
| `~/.config/ryoku/matugen.json` | `mode` | `dark`, `light`, `smart` (Automatic) or `sun` |
| `~/.config/ryoku/shell.json` | `themePalette` | A fixed scheme's roles, when one is set |
| `~/.cache/ryoku/colors.json` | the whole file | The wallpaper's palette (Material roles) |
| `~/.config/ryoku/theme.json` | `followWallpaper` | Whether the wallpaper's palette is used at all |

An unknown or missing value means the default: Lithic, Ryoku's colours,
standard contrast, tint off. For `mode`, `smart` and `sun` mean "light when the
palette's `surface` is light". Watch all four files and reload when one
changes.

For a run that must not follow the desktop (a gallery, a test, a screenshot),
set `LITHIC_MODE=dark|light|follow` and `LITHIC_DESIGN=<style>` in the
environment; they win over the files.

## How to do it

### QML (Quickshell), with the Lithic library

The library does all of it. Your part:

1. `import Lithic` and colour everything from roles (`Lithic.ground`,
   `Lithic.fg`, `Lithic.sel`, …). Never a hex.
2. Draw **every** shape with `CutRect` and give it a `cut`, never a
   `Rectangle` with a `radius`. Lithic's own controls (`Plaque`, `Toggle`,
   `Segmented`, `Slab`, `FocusRing`, …) already do.
3. Take the window's planes from `Lithic.planes()`:

   ```qml
   readonly property var p: Lithic.planes()   // the style in use
   CutRect { cut: LithicTokens.cut.large; fill: p.pane; strokeWidth: 1; strokeColor: p.paneStroke }
   ```

4. Call `Lithic.reload()` when one of the four files changes. The snippet to
   copy is in [library/qml/README.md](library/qml/README.md#palette-lifecycle).

5. To show a style that is not in use (a preview), wrap real controls in a
   `DesignScope { lithicDesign: "material" }` and take their colours from
   `Lithic.rolesFor("material")`.

Do not bind `Lithic.mode`, `contrast` or `tint` yourself: the library reads
them from the desktop. Ryoku Settings (`ryoku/hub/quickshell`) is the reference.

Lithic's marks (the notch, the blade, the flake) keep their cut shape in every
style: they are signs, not corners.

### Shell surfaces on `Ryoku.Ui` Tokens

`Tokens.radius` follows the style. Use it for every corner, and colours from
the `Tokens` roles. See [docs/ui-ux.md](../ui-ux.md#roles-never-hardcode-a-value).

### Go with Gio, and the terminal

Use the Lithic Go library ([library/go/README.md](library/go/README.md)):
`palette.Load` for the colours, `kit` for every shape. Read the keys above,
pass Ryoku's colours or the wallpaper's to `palette.Load`, and redraw on a
change.

### Web

Use the Lithic web library ([library/web/README.md](library/web/README.md)) and
pass the desktop's choice to `Lithic.init()`.

## Where each stack stands

Be honest about it in a review: a surface is only done when its stack can meet
the rule.

| Stack | Four styles | Ryoku and wallpaper colours from the desktop | Light, dark, contrast, tint |
|---|---|---|---|
| QML Lithic library | Yes | Yes | Yes |
| `Ryoku.Ui` Tokens (shell surfaces) | Corners only (`Tokens.radius`) | The desktop palette only; does not read `theme.colours` yet | Light and dark from the palette |
| Go library (Gio, terminal) | Not yet: always Lithic | Both exist, but each app picks; does not read `theme.colours` yet | Yes |
| Web library | Not yet: always Lithic | Both exist, but each page picks; does not read `theme.colours` yet | Yes |

A stack marked "not yet" is the next work, not an exception: an app on it does
not meet the rule until its library does. When you bring a library up, move it
to "Yes" here in the same commit.

## Check it

Before calling a surface done, look at it in:

- each of the four styles,
- Ryoku's colours and the wallpaper's (on a colourful wallpaper),
- dark and light,
- standard and high contrast.

Change each one while the surface is open: it must follow without a restart,
and nothing may keep the old shape or colour. The full review is
[checklist.md](checklist.md#styles-and-colours).
