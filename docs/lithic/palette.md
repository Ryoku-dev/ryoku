# Applying a palette

Ryoku has no colour rules of its own, only **roles**. A role says what a
colour is for (the paper, the ink, the accent, a failed state); the palette
the reader chose fills it in. The reader chooses once, for the whole desktop,
in Settings › Appearance › Style: Ryoku's own colours (the ryoku.dev palette
with its orange sun accent, the default) or the wallpaper's Matugen palette.
Light or dark, contrast and tint are chosen on the same page. The design style
(Lithic, Material, Adwaita or Fluent) is a separate choice and never changes
the colours ([styles.md](styles.md)).

This page says how a palette becomes roles and which role goes on which part
of the window, as Ryoku Settings (the Hub) does it. Colour values live in one
place, [tokens.json](tokens.json); nothing here or in an app repeats them.

Paths: the Hub is `ryoku/hub/quickshell/`, the QML library is
`docs/lithic/library/qml/Lithic/` (`Lithic.qml`, and `LithicTokens.qml`, which
is generated from `tokens.json`). The Go (`library/go/palette`) and web
(`library/web/palette.js`) resolvers implement the same recipe, and
`library/parity/check.sh` proves the three agree.

## 1. The roles

Every Lithic stack exposes the same roles (`Lithic.qml:15-40`):

| Role | What it is |
|---|---|
| `ground` | the paper: the window, the sidebar |
| `stone1`, `stone2`, `slab` | the same paper under a pane and a group of rows |
| `stone3` | one step up: a row under the pointer |
| `stone4` | what floats: menus, dialogs, popovers, the tree peek in the round styles |
| `field` | what is typed into, slid along or pressed as a tonal plaque |
| `vein` | the hairline that draws a pane, a slab or a rule |
| `edge` | the hairline of a control that must be found at rest (a field, a search) |
| `fg`, `fg2`, `fg3` | ink: what is read, what explains, what is quiet |
| `sel`, `onSel`, `onSel2` | the accent and the ink on it |
| `container`, `containerInk` | the soft wash under the chosen item, and its ink |
| `gold`, `goldInk`, `onGold` | the action colour: the one consequential plaque, gold type on paper, ink on the fill |
| `jade`, `lapis`, `amber`, `jasper` | the minerals: alive, becoming, needs you, failed |
| `scrim` | the shade under a dialog or a drawer |

`Lithic.dark` says which way the paper runs; `Lithic.source` says where the
colours came from (`signature`, `wallpaper`, `scheme` or `host`).

## 2. Where the palette comes from

Settings writes the reader's choice through the shell daemon and ryogami
([styles.md](styles.md#where-the-choice-lives)); Lithic reads it
(`Lithic.qml:742-794`). An app never writes these.

| File | Key | Meaning |
|---|---|---|
| `$XDG_CONFIG_HOME/ryoku/shell.json` | `theme.colours` | `ryoku` (default) or `wallpaper` |
| `$XDG_CONFIG_HOME/ryoku/shell.json` | `theme.contrast` | `standard` (default) or `high` |
| `$XDG_CONFIG_HOME/ryoku/shell.json` | `theme.tint` | `off` (default), `low` or `medium` |
| `$XDG_CONFIG_HOME/ryoku/shell.json` | `themePalette` | a fixed, named scheme's Material roles, when one is set |
| `$XDG_CONFIG_HOME/ryoku/matugen.json` | `mode` | `dark`, `light`, `smart` (Automatic) or `sun` |
| `$XDG_CONFIG_HOME/ryoku/theme.json` | `followWallpaper` | whether the wallpaper's palette is used at all (default true) |
| `$XDG_CACHE_HOME/ryoku/colors.json` | the whole file | the wallpaper's Matugen palette (Material role names) |
| `/etc/os-release` | `ID`, `ID_LIKE` | the edition, for its action colour (§6) |

`$XDG_CONFIG_HOME` defaults to `~/.config`, `$XDG_CACHE_HOME` to `~/.cache`.
Parse the files as plain JSON and ignore any key that is not a colour.
The daemon validates every value and loads an unknown one as the default
(`ryoku/shell/ipc/settings.go:686-692`).

Three environment variables override the files for one run (a gallery, a
test, a render): `LITHIC_MODE` (`dark`, `light` or `follow`),
`LITHIC_DESIGN` (a style) and `LITHIC_PALETTE_JSON` (a palette file)
(`Lithic.qml:746-749, 760-761, 783-790`).

### 2a. Which source the desktop chose

Lithic turns the files into a **mode** (`Lithic.qml:773-781`):

- `theme.colours` `wallpaper` is **follow**: the palette is read.
- `theme.colours` `ryoku` is **dark** or **light**: Ryoku's signature, and
  no palette is read at all. Light when `matugen.json` `mode` is `light`;
  dark when it is `dark`; with Automatic or Sun, light when the wallpaper
  palette's `surface` is light (luma above 0.5), else dark. The Style page
  describes Sun as following the time of day
  (`ryoku/hub/quickshell/schema/style.js:41-48`); Lithic itself reads only
  the palette's `surface`.
- Following, dark or light is the palette's own: dark when `surface` luma is
  0.5 or less (`Lithic.qml:732-734`).
- Following, the palette is the named scheme (`themePalette`) over the
  wallpaper's (`colors.json`, skipped when `followWallpaper` is false); a
  host palette from `LITHIC_PALETTE_JSON` or `setPalette()` wins over both
  (`Lithic.qml:643-659`). With no palette at all, following paints the
  signature.

So choosing Ryoku's colours **replaces the wallpaper scheme completely**:
dark and light paint the signature whatever `colors.json` and
`themePalette` hold (`Lithic.qml:662-666`). Only the edition accent (§6)
and, under Automatic or Sun, the paper's direction come from outside.

## 3. Mapping palette roles to Lithic roles

One function turns the source into roles. It runs the same in every stack
(`Lithic.qml:550-640`; `library/go/palette`; `library/web/palette.js`).

### Ryoku's colours (the signature)

Dark and light take the signature table for the mode and contrast as it
stands: `signature.dark`, `darkHigh`, `light` or `lightHigh` in
`tokens.json` (`Lithic.qml:562-571`). It is the ryoku.dev palette: black
paper with bone ink in dark, warm grey paper in light, and the orange sun as
both the accent (`sel`) and the action (`gold`). Every style wears it the
same way (`Lithic.qml:719-735`). The paper is the whole window: `ground`,
`stone1`, `stone2` and `slab` are one tone, and only `stone3`, `stone4` and
`field` step off it.

### The wallpaper's colours in Lithic

In the Lithic style a palette **only dyes**. Lithic keeps every lightness
from its tone ladder (`tone.dark`, `darkHigh`, `light`, `lightHigh` in
`tokens.json`) and takes only the OKLCH hue and chroma of a Matugen role,
lowering the chroma until the colour fits sRGB (`Lithic.qml:574-623`). A
wallpaper changes what colour every plane is, never how far apart the planes
sit, so contrast holds on any wallpaper.

| Lithic role | Dyed by | Chroma cap (`tokens.json`) |
|---|---|---|
| `ground` | `surface` | `dye.chroma.ground`, `dye.boost.ground` |
| `stone1` to `stone4`, `slab`, `field` | `surface` | `dye.chroma.stone` |
| `vein`, `edge` | the stone dye | `dye.boost.vein` |
| `fg`, `fg2`, `fg3` | `onSurface` | `dye.chroma.ink` |
| `gold`, `goldInk` | `primary` | `dye.chroma.gold` |
| `onGold` | the gold hue | `dye.chroma.onGold` |
| `sel` | `secondaryContainer` mixed toward `primaryContainer` by `alpha.selMix` | `dye.chroma.sel` |
| `onSel` | the selection hue | `dye.chroma.onSel` |
| `onSel2` | `onSel` mixed toward `sel` by `alpha.onSel2` | |
| `jasper` | the hue of `error` | `dye.chroma.jasper` |
| `scrim` | the palette's `scrim` at `alpha.scrimDark` or `alpha.scrimLight` | |

A role the palette lacks takes its dye from `dye.fallback`.

### The wallpaper's colours in Material, Adwaita and Fluent

The round styles take the Matugen roles as they are (`Lithic.qml:671-700`):

| Lithic role | Matugen role |
|---|---|
| `ground`, `stone2`, `slab` | `surface` |
| `stone1` | `surfaceContainerLow` |
| `stone3` | `surfaceContainer` |
| `stone4` | `surfaceContainerHigh` |
| `field` | `surfaceContainerHighest` |
| `vein`, `edge` | `outlineVariant`, `outline` |
| `fg`, `fg2` | `onSurface`, `onSurfaceVariant` |
| `fg3` | `onSurfaceVariant` mixed toward `surface` |
| `sel`, `onSel` | `primary`, `onPrimary` |
| `gold`, `goldInk`, `onGold` | `tertiary`, `tertiary`, `onTertiary` |
| `jasper` | `error` |
| `container`, `containerInk` | `secondaryContainer`, `onSecondaryContainer` |

A role the palette lacks keeps its Lithic value. On Ryoku's colours, and in
Lithic on any source, `container` is `sel` at 0.16 alpha in dark and 0.14 in
light, and `containerInk` is `fg` (`Lithic.qml:637-638`).

### Contrast and tint

**Contrast high** reads the high ladder and the high signature tables
(`Lithic.qml:551`): what steps up from the paper, the hairlines and the inks
move further from it, while the paper stays. It applies to every source and
every style.

**Tint** low or medium multiplies the neutral dye's chroma by `dye.tint`
(`Lithic.qml:579-581`), so the paper and stones take more of the wallpaper's
hue. It changes no lightness, so it never moves a contrast. It applies only
in the Lithic style on the wallpaper's colours: the Hub's Style page shows
the Tint row only when Colours is Wallpaper
(`ryoku/hub/quickshell/schema/style.js:55-60`) and drops it in the other
styles (`ryoku/hub/quickshell/pages/Style.qml:45-46`).

### What a step and a hairline carry

The Hub spends tone only where something answers the hand and draws
everything else with a line:

- **A pane or a group** lies on the paper and is drawn by a 1 px `vein`
  hairline along its cut. In the Hub the page pane is the same paper as the
  sidebar, set off by its hairline (`ryoku/hub/quickshell/Hub.qml:910-924`),
  and a plain `Slab` draws its `vein` stroke with no fill
  (`Slab.qml:31-37`). The toned slab of something that needs the reader
  (`Slab { tone: "attention" }`) is set off by its mineral field instead and
  carries no line.
- **Rows** are separated by a `vein` hairline (`SettingRow.qml:122-130`).
- **A change** is seen because it happens: a setting row under the pointer
  lights its cut plane with `fg` at `alpha.hoverPlane`
  (`SettingRow.qml:46, 132-146`), and a tree row with the `Tone.hover` wash
  (§4). Neither adds a line.
- **A field** is filled with `field` and carries an `edge` hairline at rest,
  `fg2` with keyboard focus and `jasper` when invalid
  (`FieldSurface.qml:12-13`), because the hairline marks where to type.
- **What floats** (a dialog, a popover, a menu) is `stone4` with a `vein`
  hairline over its scrim.

### Fixed minerals

`jade` (alive), `lapis` (becoming) and `amber` (needs you) never come from a
palette, so "alive" reads the same on every wallpaper. Their dark and light
values are in `tokens.json` `minerals` (`Lithic.qml:626-631`). `jasper`
(failed) takes the palette's `error` hue at its ladder step. A mineral field
(`Lithic.mineralField(tone)`) is the mineral's hue at the `mineralField`
step with chroma `dye.mineral` (`Lithic.qml:632-636`).

### Derived tones

Compute these from roles; never pick them. Alphas are in `tokens.json`
`alpha`.

| Derived | Recipe | Source |
|---|---|---|
| hover on a filled plaque | `mix(fill, white, alpha.hoverFill)` | `Lithic.qml:242-245` |
| hover on a plane | one step toward `stone3`; on `stone4`, `mix(stone4, fg, alpha.hoverPlane)` | `Lithic.qml:252-257` |
| press on a row | `mix(fill, fg, alpha.pressRow)` | `Lithic.qml:259-262` |
| hover on a field | `mix(field, fg, alpha.hoverPlane)` | `FieldSurface.qml:12` |
| disabled | the whole control at `alpha.disabled` | `Lithic.qml:154` |
| ink on a danger fill | `inkOn(jasper)`: black or white, whichever reads better | `Lithic.qml:227-234` |

## 4. Which role goes where

The Hub reads roles only; its chrome holds no colour value
([design.md](design.md#2-vocabulary) names the parts). The planes come from
`Lithic.planes()` for the style in use, through `Tone`
(`ryoku/hub/quickshell/Singletons/Tone.qml:16-22`):

| Surface | Role | Source |
|---|---|---|
| window, sidebar | `Tone.side` (Lithic: `ground`) | `Hub.qml:905-908`, `components/nav/HubSidebar.qml:23` |
| page pane and its stroke | `Tone.pane`, `Tone.paneStroke` (Lithic: the paper, `vein`) | `Hub.qml:912-924` |
| a page section's slab | `Tone.card` (Lithic: `slab`, drawn by `vein`) | `components/SettingsPage.qml:803` |
| registration sheet ornaments | fixed alphas of `fg`, separate for dark and light | `components/sheet/RegSheet.qml:24-44` |
| ink | `fg` for what is read; `detail` type defaults to `fg2`; a row's detail is `fg2`, or `fg3` when the row is unavailable | `Label.qml:7`, `SettingRow.qml:202` |
| the spec strip's counts | `fg3`, a lit fact in the accent | `components/PageHeader.qml:128-135` |
| the tree's hairline, guide, lit guide | `Tone.line`, `Tone.guide`, `Tone.guideLit` | `Tone.qml:23-25` |
| a tree row under the pointer, pressed | `Tone.hover`, `Tone.press` | `Tone.qml:26-27` |
| the open page in the tree | `Tone.select` = `container`, ink `containerInk` | `Tone.qml:31-32` |
| a mark of place | `Tone.accent` = `sel` | `Tone.qml:28` |
| a control that is on (toggle, chosen segment, slider handle), a placard | `sel` with `onSel` | the Lithic components |
| actions | plaques by tier, below | `PlaqueRecipe.qml:6-28` |
| a claim: alive, becoming, failed | `jade`, `lapis`, `jasper` | `components/Home.qml:772-777` |
| what needs the reader | `Slab { tone: "attention" }` on the amber field | `components/Home.qml:798-815` |
| an error | `jasper` detail text | `components/SettingsPage.qml:973-980` |
| a dialog, the narrow drawer | `stone4` on `scrim` | `Dialog.qml:43`, `Hub.qml:1024-1035` |

Each tree wash is an alpha of `fg`:

| Wash | Dark | Light | Used for |
|---|---|---|---|
| `Tone.line` | 0.08 | 0.10 | the hairline under the tree |
| `Tone.guide` | 0.16 | 0.20 | the tree's guide, branch ticks and leaders |
| `Tone.guideLit` | 0.36 | 0.42 | the guide of the open page's folder |
| `Tone.hover` | 0.06 | 0.07 | a row under the pointer |
| `Tone.press` | 0.10 | 0.12 | a pressed row |

**The accent marks place and nothing else.** `sel` is the `§ NN` numeral in
the masthead, the numeral, mark and kanji of the open line and folder, the
branch tick of the open leaf, the masthead's scroll marker, the outline node
of the section being read, the tategaki rule, the peek card's glyph, a lit
fact in the spec strip (for example `CHANGED` above zero) and the Updates dot
(`components/sheet/Masthead.qml:57-62, 154`;
`components/nav/NavRow.qml:91, 128, 147, 192, 228-236`;
`components/nav/NavOutline.qml:85`; `components/PageHeader.qml:133-136`).
In the round styles the open line is inked `containerInk` on its wash
instead (`NavRow.qml:128`).

**Gold is the action.** It fills the `gold` plaque, the one consequential
action of a view (Home keeps "one gold on the plate: the first thing that
waits", `components/Home.qml:838-841`). Beyond actions the Hub also uses it
on Home's motto tick lead and carved word (`Home.qml:476, 757-760`), on card
rarity rims and stamps, and on the binder's header glyph and compare winners
(`components/about/Binder.qml:334-344`; `BinderCompare.qml:45-64`).

**Plaques** take their roles from their tier (`PlaqueRecipe.qml:6-28`):

| Tier | Fill | Ink | Hairline | Uses in the Hub |
|---|---|---|---|---|
| `quiet` | none | `fg2` | none | 206: almost every action |
| `line` | none | `goldInk` | `goldInk` | 40: a secondary action |
| `gold` | `gold` | `onGold` | none | 39: the one consequential action |
| `tonal` | `field` | `fg` | none | 18 |
| `dangerLine` | none | `jasper` | `jasper` | 14 |
| `danger` | `jasper` | `inkOn(jasper)` | none | 3: the action of a destructive confirm |
| `on` | `sel` | `onSel` | none | 0: Lithic uses it inside placards |

Counts are every `kind:` literal in Hub QML, including dialog actions.

On hover a filled tier mixes toward white, a transparent one takes its ink
at `alpha.hoverInk`, and `tonal` mixes toward `fg`.

**Shades** are alphas of a role: the binder `ground` at 0.96 dark and 0.90
light (`Binder.qml:315-322`), the certificate `ground` at 0.97
(`components/badge/Certificate.qml:34-48`), the card viewer black at 0.82
dark and 0.60 light (`CardViewer.qml:31-32`).

**Not themed:** the badge's role inks are fixed art, the same on every
palette (`components/badge/Emblem.qml:21-27`).

## 5. Light and dark

Both modes use the same roles; only the tables differ. Where the Hub draws
something that must read differently on light paper, it says so in the
code:

- Ornament inks carry separate dark and light alphas (`RegSheet.qml`,
  `Folio.qml`, the `Tone` washes).
- Home's carved word is the paper mixed toward `gold` in dark and toward `fg`
  in light, "cut into the stone" (`components/Home.qml:461-476`).
- Home's figure is inked `fg` in dark and `stone4` in light, "never a
  negative" (`Home.qml:524-526`).
- Home's light is `sel` at 0.55 in dark and `fg` at 0.32 in light
  (`Home.qml:496`).

Compare `media/hub-home-dark.png` with `media/hub-home-light.png`: the paper
turns, the Needs you slab is a pale amber field, and the plaques keep the sun.

## 6. The edition colour

On Ryoku's colours, `gold` is the host edition's signal where it has one.
Lithic reads `ID`, then each word of `ID_LIKE`, from `/etc/os-release`.
Fedora, NixOS and Void have an entry in `tokens.json` `edition`; Arch,
CachyOS and other hosts keep the sun. The edition replaces `gold` and
`goldInk`, `onGold` becomes `inkOn(gold)`, and `sel` stays the sun
(`Lithic.qml:566-570`).

## 7. Following changes live

The Hub watches the four files and calls `Lithic.reload()` when one changes
(`ryoku/hub/quickshell/ShellBody.qml:55-78`). A new palette snaps: the roles
are recomputed and every binding takes the new value at once. Do the same in
any QML app ([library/qml/README.md](library/qml/README.md#palette-lifecycle)).
In Go, `palette.Watch` watches the parent directories and calls back only
when the resolved roles change. A browser page cannot read `~/.cache`; the
host must hand the palette in.

## 8. Verifying the hookup

A surface is wired correctly when, on a Ryoku desktop:

1. On Ryoku's colours, changing the wallpaper repaints nothing.
2. On the wallpaper's colours, a new wallpaper changes the paper, panes,
   fields, accent and gold; jade, lapis and amber stay.
3. A named scheme, while following, wins over the wallpaper.
4. Contrast high moves hairlines, steps and inks away from the paper. Tint
   low and medium give the Lithic paper more of the wallpaper's hue; on
   Ryoku's colours and in the round styles it changes nothing.
5. Changing the style changes no colour ([styles.md](styles.md)).
6. A source search finds no colour value outside `tokens.json` and its
   generated files.

`library/parity/check.sh` proves the QML, Go and web resolvers paint the same
roles for every case ([parity guide](library/parity/README.md)).

### Contrast targets

These floors hold for every hue at the capped chroma and every tint, and for
the signature tables; the Go palette tests enforce them around the whole hue
circle (`library/go/palette/palette_test.go:204-243`).

| What | On | WCAG ratio |
|---|---|---|
| `goldInk`, `jasper`, `fg2`, `fg3` | the paper (`stone1`, `stone2`, `slab`) | 4.6:1, 5:1 at high contrast |
| `fg2` | `field` | 4.6:1, 5:1 at high contrast |
| `jade`, `lapis`, `amber` | the paper | 4.6:1 |
| `sel` | every plane | 3:1 |
| `onSel`, `onSel2` | `sel` | 4.5:1 |
