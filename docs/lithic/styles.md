# Styles and colours

Every Ryoku surface wears the desktop's look. The reader chooses it once, in
**Settings › Appearance › Style**, and everything Ryoku draws follows at
once, without a restart. Ryoku Settings (the Hub, `ryoku/hub/quickshell/`) is
the reference: it is drawn entirely through the Lithic library and follows
every key on this page. This page is the rule, what the Hub does with each
choice, and how to meet it in each stack.

## The rule

An app made for Ryoku must:

1. **Draw in all four styles**: Lithic, Material, Adwaita and Fluent.
2. **Offer both colour sources**: Ryoku's own colours, the default, and the
   wallpaper's Matugen palette (or a fixed scheme set with the wallpaper).
3. **Work in light and in dark**, on both colour sources.
4. **Honour Contrast** (standard or high) **and Tint** (Lithic on the
   wallpaper's colours only).
5. **Follow a change live**, without a restart.
6. **Keep no copy of the choice.** Read the desktop's choice and never offer
   a style or colour picker of your own. To send the reader to it, open
   Settings › Style (`ryoku-shell hub open style`).

The state minerals (jade, lapis, amber) never change with the style or the
colours ([palette.md](palette.md#fixed-minerals)).

## The four styles

A style changes **corners and how planes are layered**. It never changes the
colours (`ryoku/hub/quickshell/Singletons/Tone.qml:7-10`;
`docs/lithic/library/qml/Lithic/Lithic.qml:42-53`).

| Style | How the Style page describes it | Corners | Controls | Planes |
|---|---|---|---|---|
| **Lithic** (default) | "Ryoku's own: cut corners, panes drawn in hairlines, one colour for the action" | cut at 45 degrees | cut plaques, cut toggle | one paper; the page and each group drawn by a 1 px `vein` hairline |
| **Material** | "Material 3: pill buttons and switches, large soft cards on tonal surfaces" | round, cut × 2.6, at most 28 | a pill when 56 or less tall | the page a step up from the ground, its cards a step more, no lines |
| **Adwaita** | "GNOME's look: rounded cards and lists, soft buttons, pill switches" | round, cut × 1.6, at most 14 | a pill when 26 or less tall | the sidebar set apart, the page on the ground, cards on it, no lines |
| **Fluent** | "The Windows 11 look: small corners and layered, outlined surfaces" | round, cut × 0.75, at most 8 | a pill when 22 or less tall | layered planes, each with a thin outline |

Descriptions: `ryoku/hub/quickshell/schema/style.js:12-21`. Corners:
`Lithic.cornerRadius()` (`Lithic.qml:61-69`). Planes: `Lithic.planes()`
(`Lithic.qml:77-88`):

| Style | Sidebar (`side`) | Page (`pane`) | Page stroke | Group of rows (`card`) | Group stroke |
|---|---|---|---|---|---|
| Lithic | `ground` | `stone2` | `vein` | `slab` | `vein` |
| Material | `ground` | `stone1` | none | `stone3` | none |
| Adwaita | `stone3` | `ground` | none | `stone4` | none |
| Fluent | `ground` | `stone3` | `vein` | `stone4` | `vein` at 0.7 |

You never compute a corner or a plane yourself. Give every shape its Lithic
**cut** from the scale ([design.md](design.md#5-shape-the-cut-scale)) and the
library turns it into the style's corner; ask the library for the planes.

## Style and colour are independent

The style is `theme.design`; the colours are `theme.colours`, a separate key
(`Lithic.qml:758-780`). Changing one never touches the other:

- On **Ryoku's colours** every style wears the same signature, the ryoku.dev
  palette with the sun accent (`Lithic.qml:719-735`).
- On the **wallpaper's colours**, Lithic dyes its own lightness ladder with
  the palette's hue, while Material, Adwaita and Fluent take the Matugen
  roles as they are. [palette.md](palette.md#3-mapping-palette-roles-to-lithic-roles)
  has both recipes.

So a reader who picks Material keeps their colours, light or dark, contrast
and tint; switching back to Lithic restores the same look it had.

## Where the choice lives

Apps only read these. Settings writes the `theme.*` keys through the shell
daemon, which validates every value
(`ryoku/shell/ipc/settings.go:78-84, 686-692`), and light or dark through
`ryogami settings set theme.mode` (`ryoku/hub/quickshell/schema/style.js:41-48`).

| File | Key | Meaning |
|---|---|---|
| `~/.config/ryoku/shell.json` | `theme.design` | `lithic`, `material`, `adwaita` or `fluent` |
| `~/.config/ryoku/shell.json` | `theme.colours` | `ryoku` or `wallpaper` |
| `~/.config/ryoku/shell.json` | `theme.contrast` | `standard` or `high` |
| `~/.config/ryoku/shell.json` | `theme.tint` | `off`, `low` or `medium` |
| `~/.config/ryoku/shell.json` | `theme.motion.reduce` | reduce motion ([motion.md](motion.md#reduced-motion)) |
| `~/.config/ryoku/shell.json` | `themePalette` | a fixed scheme's roles, when one is set |
| `~/.config/ryoku/matugen.json` | `mode` | `dark`, `light`, `smart` (Automatic) or `sun` |
| `~/.config/ryoku/theme.json` | `followWallpaper` | whether the wallpaper's palette is used at all |
| `~/.cache/ryoku/colors.json` | the whole file | the wallpaper's palette (Material roles) |

A missing or unknown value is the default: Lithic, Ryoku's colours, standard
contrast, tint off. Watch the files and reload when one changes; the Hub
watches `shell.json`, `theme.json`, `matugen.json` and `colors.json`
(`ryoku/hub/quickshell/ShellBody.qml:55-78`).

For a run that must not follow the desktop (a gallery, a test, a render), set
`LITHIC_MODE=dark|light|follow` and `LITHIC_DESIGN=<style>`; they win over
the files.

## What the Hub does when the style changes

The Style page (`ryoku/hub/quickshell/pages/Style.qml`) draws the four styles
as tiles over a live preview:

- Resting on a tile previews that style; choosing it writes `theme.design`
  through `Changes`, with the notice "Design set to %1" and Undo
  (`Style.qml:24-36`).
- The preview is a real piece of Settings (tree lines, a card of rows with a
  switch, a choice and a slider, the page's buttons) drawn inside a
  `DesignScope` with that style's roles and planes
  (`components/style/StylePreview.qml:7-22`). An "Example" chip and a caption
  say its controls only show the design and change nothing
  (`Style.qml:80-117`). It fades in from 0.35 over 240 ms when the style
  shown changes (`Style.qml:119-130`).
- The Tint row is shown only in Lithic (`Style.qml:45-46`) and only on the
  wallpaper's colours (`schema/style.js:55-60`).
- "Palette in use" shows the colours being worn, with a line saying whether
  they are Ryoku's, a fixed scheme or the wallpaper's
  (`components/style/PaletteStrip.qml:38-44`).

When `shell.json` changes, `Lithic.reload()` runs and the whole window takes
the new style in one pass:

- `Tone` reads the new planes and `Tone.flat` (`Tone.qml:12-22`).
- Every shape is a `CutRect`, so each turns its cut into the new corner.
- In the round styles the open tree line is inked `containerInk` on its wash
  instead of the sun (`components/nav/NavRow.qml:127-128`), the tree peek
  card is `stone4` instead of the paper (`components/nav/HubSidebar.qml:217`),
  and the Activity map's cells and the desktop previews round
  (`components/sheet/Activity.qml:229`; `components/previews/MiniDesk.qml:13`).
- The sheet stays: the registration sheet, masthead, folio, colophon,
  numbering and kanji gloss read no style and are drawn the same in all four
  ([design.md](design.md#2-vocabulary) names them).

## How to do it

### QML (Quickshell), with the Lithic library

The library does all of it. Your part:

1. `import Lithic` and colour everything from roles (`Lithic.ground`,
   `Lithic.fg`, `Lithic.sel`, ...). Never a colour value.
2. Draw **every** shape with `CutRect` and give it a `cut`, never a
   `Rectangle` with a `radius`. Lithic's own controls (`Plaque`, `Toggle`,
   `Segmented`, `Slab`, `FocusRing`, ...) already do.
3. Take the window's planes from `Lithic.planes()`, as the Hub's `Tone` does:

   ```qml
   readonly property var p: Lithic.planes()   // the style in use
   CutRect { cut: LithicTokens.cut.large; fill: p.pane; strokeWidth: 1; strokeColor: p.paneStroke }
   ```

4. Call `Lithic.reload()` when one of the files changes. The snippet to copy
   is in [library/qml/README.md](library/qml/README.md#palette-lifecycle).
5. Bind `Lithic.reduceMotion` to `theme.motion.reduce`, as
   `ryoku/hub/quickshell/ShellBody.qml:23-28` does.
6. To show a style that is not in use (a preview), wrap real controls in a
   `DesignScope { lithicDesign: "material" }` and take their colours from
   `Lithic.rolesFor("material")`, as `StylePreview.qml` does.

Do not set `Lithic.mode`, `contrast` or `tint` yourself: the library reads
them from the desktop.

### Shell surfaces on `Ryoku.Ui` Tokens

`Tokens.radius` follows the style, but only as a rounded radius: it is 6 in
Lithic, 12 in Adwaita and 16 in Material (`ryoku/ui/Singletons/Tokens.qml:138`),
so a Tokens surface cannot cut. Its colours come from its own palette chain,
and it watches `theme.design` but never `theme.colours`, `theme.contrast` or
`theme.tint` (`Tokens.qml:319`). A Tokens surface meets the rule by moving to
the Lithic library ([working.md](working.md#upgrade-a-ryoku-qml-surface)).

### Go with Gio, and the terminal

Use the Lithic Go library ([library/go/README.md](library/go/README.md)). It
reads the choice as the QML library does. Your part:

1. Read the desktop once with `palette.ReadDesktop()` and build the theme with
   `theme.NewFor(desktop)`: it carries the roles, the style
   (`theme.Theme.Design`) and reduced motion.
2. Run `palette.WatchDesktop` for the life of the window, not only while
   following the wallpaper: it calls back only when the look changed, and the
   window's goroutine swaps in `theme.NewFor(next)` and invalidates.
3. Draw every shape through `kit` (`kit.Fill`, `kit.Stroke` and the controls),
   and size your own corners with `th.Corner(gtx, cut, bounds)`, never a cut
   or radius of your own: Lithic cuts, the others round.
4. Take the window's planes from `th.Planes()`, as the Hub's `Tone` does.
5. For a preview of a style not in use, take its roles from
   `desktop.RolesFor(design.Material)`.

Do not offer a mode, contrast or tint of your own. The terminal (`term`)
draws only in Lithic: `term.NewThemeFor(desktop)` gives it the desktop's
colours, light or dark, contrast and tint.

### Web

Use the Lithic web library ([library/web/README.md](library/web/README.md)).
It does not read the desktop's choice yet: the page passes the mode, and
every shape is drawn in the Lithic style.

## Where each stack stands

Be honest about it in a review: a surface is only done when its stack can
meet the rule. The surfaces named are from the system audit of every Ryoku
UI against the Hub (2026-11-16).

| Stack | Surfaces on it | Four styles | Ryoku and wallpaper colours from the desktop | Light, dark, contrast, tint |
|---|---|---|---|---|
| QML Lithic library | the Hub, the Rashin bar, Kit, the dictation card, the game bar, the OSD, notification pop-ups, ryoshot | Yes | Yes | Yes |
| `Ryoku.Ui` Tokens | quick settings, Ask, launcher, welcome, ryopin, keys hint, the Stage Editor pages, RyoStore, the session confirm, Orpheus's kit | Corners only, and always rounded (`Tokens.qml:138`) | The desktop palette only; never reads `theme.colours` (`Tokens.qml:319`) | Light and dark from the palette; no contrast or tint |
| Shell `Theme` singleton | the bar frame, popouts, the notification history, clipboard, the Ryoku dock, Kairos, store bar styles | No: a fixed radius, never reads `theme.design` | No: Matugen roles only | Light and dark from the palette |
| Private theme copies | the launcher, workspace overview, desktop widgets, the Ryogami picker, the plugin kit | No | No: each keeps its own palette chain or fixed values | Their own |
| Go library (Gio, terminal) | Rashin, Ryotunes | Yes (Gio; the terminal stays Lithic) | Yes: `palette.ReadDesktop` and `WatchDesktop`; Rashin follows the desktop without a second style or colour setting | Yes |
| Web library | Ryoku web pages | Not yet: always Lithic | Both exist, but each page picks | Yes |
| Matugen app templates | terminals, GTK, Qt, editors | n/a | The wallpaper's palette whatever `theme.colours` says; the Hub tells the reader so (`components/style/PaletteStrip.qml:41`) | Light and dark from the palette |

A stack marked "No" or "Not yet" is the next work, not an exception: an app
on it does not meet the rule until its library does. When you bring a
library up, move it to "Yes" here in the same commit.

## Check it

Before calling a surface done, look at it in:

- each of the four styles,
- Ryoku's colours and the wallpaper's (on a colourful wallpaper),
- dark and light,
- standard and high contrast, and each tint in Lithic on the wallpaper's
  colours.

Change each one while the surface is open: it must follow without a restart,
and nothing may keep the old shape or colour. Compare it with the Hub in the
same look. The full review is [checklist.md](checklist.md#styles-and-colours).
