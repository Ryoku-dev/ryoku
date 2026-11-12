# Applying the Ryoku palette

Lithic has no colours of its own. It has **roles**, and the roles are filled
from the palette the Ryoku desktop is already using, so a Lithic app matches
the wallpaper, the chosen theme, and every other Ryoku surface, and changes
with them. This page is the complete recipe. Implement it once, as one
function from palette to roles, and never write a colour value anywhere else
in the app.

Reference: `ui/theme/palette.go` (`FromPalette`) and
`internal/palette/palette.go` (`Load`), `edition.go`, `watch.go` in Ryoport.
The [examples](examples/) implement the same function in CSS/JS, QML and Go.

## 1. Where the palette lives

The theme daemon of the Ryoku shell is the only author of colour on a Ryoku
desktop. It publishes three files; read all three, in this order of
precedence:

| File | Key | Meaning |
|---|---|---|
| `$XDG_CONFIG_HOME/ryoku/shell.json` | `themePalette` (object of role to `#rrggbb`) | A fixed, named scheme is active. **Wins** over the wallpaper palette. Absent or `null` when the desktop follows the wallpaper. |
| `$XDG_CACHE_HOME/ryoku/colors.json` | the whole object | The live wallpaper palette, written by matugen. Material 3 role names (`surface`, `primary`, `onSecondaryContainer` …) plus terminal `color0` to `color15`, `background`, `foreground`, `cursor`. |
| `$XDG_CONFIG_HOME/ryoku/theme.json` | `followWallpaper` (bool, default `true`) | Whether the wallpaper palette is in use at all. |

`$XDG_CONFIG_HOME` defaults to `~/.config`, `$XDG_CACHE_HOME` to `~/.cache`.
Values are `#rrggbb` or `#rrggbbaa` strings; ignore any key that is not a
parseable colour. Parse the files as plain JSON text; do not bind them to a
typed model, because half the role names start with `on` and a removed key
must read as absent.

## 2. Resolving the source

Inputs: the three files and the app's own colour mode, one of **follow**
(default), **dark**, **light**.

```
roles  := {}
source := "signature"

if followWallpaper || mode != follow:
    roles  += colors.json                       # wallpaper roles
    if colors.json had any role: source = "wallpaper"

roles += shell.json.themePalette                # named scheme overrides
if themePalette had any role: source = "scheme"

if source == "signature":
    roles.primary = editionColour(/etc/os-release)   # see §6

forced := mode != follow
dark   := mode != light
if !forced:
    dark = true
    if roles.surface exists:
        luma = (0.299 R + 0.587 G + 0.114 B) / 255   # of surface, 0..1
        dark = luma <= 0.5
```

So: a named scheme always wins; the wallpaper palette is used when the user
follows the wallpaper or has forced a mode; and only when neither file yields
a role does the app fall back to its signature neutrals (§5) with the host
distribution's colour as the accent.

Dark or light is decided by the palette's `surface` luma unless the user
forced a mode in the app.

## 3. Mapping palette roles to Lithic roles

Inputs: the palette and mode from §2, plus the reader's **look**: a contrast
(**standard** or **high**) and a tint (**off**, **low** or **medium**). The
library default is standard and off; an app picks its own default and offers
both as settings. Every stack runs the same steps with the same rounding.

A palette only dyes. Lithic owns every lightness: each role sits at a fixed
step of the **tone ladder** for the mode and contrast (`tone.dark`,
`tone.darkHigh`, `tone.light` and `tone.lightHigh` in `tokens.json`), and the
palette lends that step a hue and a chroma. A wallpaper changes the colour of
every plane, never how far apart the planes sit, so the contrast floors in
[contrast targets](#contrast-targets) hold on any wallpaper.

```
dye(colour)      = its OKLCH chroma and hue
toned(step, dye) = OKLCH(step, dye.chroma, dye.hue), the chroma lowered until
                   the colour is inside sRGB, each channel rounded half up
```

### The signature source

With no palette (§2 found no role), the roles are the signature table for the
mode and contrast (§5), as they stand. An edition colour (§6) replaces `gold`
and `goldInk`, and `onGold` becomes `inkOn(gold)`. Nothing else is computed.

### Neutrals

| Lithic role | Dye when following |
|---|---|
| `ground` | `surface`, chroma `min(0.02, 1.4 × C)` |
| `stone1` to `stone4`, `slab`, `field` | `surface`, chroma up to 0.014 |
| `vein`, `edge` | the stone dye, chroma × 1.2 |
| `fg`, `fg2`, `fg3` | `onSurface`, chroma up to 0.016 |
| `scrim` | the palette's `scrim` at alpha 0.62 (dark) or 0.45 (light) |

Following the palette, each neutral is `toned(its step, its dye)`. In light
mode the ladder runs the other way: the ground is the darkest plane and the
work pane among the lightest, so the panes still sit up from the ground.
`field` is the top of the ramp in dark mode and sits below the work pane in
light mode, so a field stands clear of the pane, a slab and a dialog alike.

When the user **forces** dark or light, the neutrals stay the signature set
for that mode (§5); only the accents below come from the palette. This keeps a
forced light app readable on a dark wallpaper.

### What one step of the ramp can carry

The ladder spaces the planes one small step apart (in dark mode `stone1`
0.188, `stone2` 0.216, `stone3` and `slab` 0.246, `stone4` 0.278, `field`
0.31). One step is enough for a change and not for an edge:

- **A change** is seen because it happens. A row that goes from `stone2` to
  `stone3` under the pointer is read as hovered. Hover, press and the tracks
  inside a control need one step and nothing else.
- **An edge that has to be seen at rest** needs more. A thing that must read
  as its own object either stands two roles from the plane under it (a tonal
  plaque filled with `field` on the `stone2` work pane or on a slab), or,
  when it is one role away or less, carries a 1 dp hairline.
  Menus, dialogs and light-mode slabs carry a `vein` hairline, and so does
  the off track of a toggle. A field, search or picker anchor carries a 1 dp
  `edge` hairline wherever it stands, because the hairline marks where to
  type, not only where the plane ends; keyboard focus turns it `fg2`.

A page that leaves a field with no hairline on the work pane has drawn a
field nobody can find until they press it.

### Accents (always from the palette)

| Lithic role | Dye | Note |
|---|---|---|
| `gold`, `goldInk` | `primary`, chroma `min(0.15, 1.4 × C)` | the action fill; gold type and hairlines on stone |
| `onGold` | the gold hue, chroma 0.03 | ink on the action fill |
| `jasper` | the hue of `error`, chroma 0.17 | failed, unreachable, destructive |
| `sel` | `secondaryContainer` mixed halfway toward `primaryContainer`, chroma up to 0.03 | the selection; see [the selection](#the-selection) |
| `onSel` | the selection hue, chroma 0.03 | ink on a selection |
| `onSel2` | `mix(onSel → sel, 0.2)` | quiet ink on a selection (facts, proofs, counts) |

A role the palette lacks takes its dye from `dye.fallback`: hue 41 (the sun)
for gold, 16 for jasper, and the warm neutral hue 72 at a low chroma for the
rest.

### The selection

The selection is a strong fill. In the signature it is bone, the ink tone
itself, with black ink on it in dark mode, as on ryoku.dev. On a palette it
is the same idea dyed by the wallpaper: a light bone near `fg` in dark mode
and a dark one in light mode, carrying only a hint of the wallpaper (chroma
0.03 at most). Material derives the containers from the same seed as the
primary, so a stronger tint would turn the selection into a paler copy of
gold; bone and gold must never be confused. The fill alone marks the place,
so nothing is drawn on it, no bar and no outline. In every case the parity
suite runs, `sel` stands 6.7:1 or more from every plane, `onSel` 11.8:1 or
more on it, and `onSel2` 8.2:1 or more.

A forced dark or light mode still takes its accents from the palette, so it
gets the wallpaper's selection too.

### Contrast and tint

**Contrast high** reads the high ladder and the high signature tables: the
stones, slabs, fields and veins spread away from the ground and the inks
move out, while the ground stays. It applies to every source.

**Tint low or medium** multiplies the neutral dye's chroma by 1.8 or 2.6
(`dye.tint`), so the ground and stones take more of the palette's colour.
It applies only when following a palette; the signature and the forced modes
keep their neutrals. A tint changes no lightness, so it never moves a
contrast.

A change of contrast or tint re-runs §3 and snaps, like a palette change.

### The edge

`edge` is a ladder step with the vein dye. It is the hairline of a control
that has to be found at rest on its own: a field, a search, a picker anchor or
a stepper's field. `vein` stays the hairline of planes that are set off anyway
(menus, dialogs, light-mode slabs, the tab baseline).

### Fixed minerals

`jade` (alive), `lapis` (becoming) and `amber` (needs you) do **not** come
from the palette. They carry meaning, and a Ryoku user must read "alive" the
same way on every wallpaper. Their dark and light values are canonical in
`tokens.json`, are consumed through each stack library, and stand 4.6:1 or
more on `stone1`, `stone2` and `slab` in their mode on any wallpaper. Do not
invent replacements or allow a scheme to override them. Jasper takes the
palette's `error` hue at its ladder step.

### Derived tones

Compute these from the roles above; never pick them.

| Derived | Recipe |
|---|---|
| `mineralField(tone)` mineral field | the mineral's hue at the `mineralField` step, chroma `dye.mineral` (0.045 to 0.055); idle is `stone3` |
| `stone(tone)` the state's ink | jade, lapis, amber (also for paused), jasper; idle is `vein` |
| hover on a filled plaque | `mix(fill, white, 0.10)` |
| hover on a transparent plaque | ink at alpha 0.10 |
| hover on a tablet or rail tile below `stone4` | one plane up: `stone2 → stone3`, `stone1 → stone3` |
| hover on a row on `stone4` | `mix(stone4, fg, 0.06)`; nothing is above `stone4`, so it must not step down |
| hover on a tonal plaque | `mix(field, fg, 0.06)` |
| hover on field controls | add `fg` at alpha 0.06 |
| press on a tablet / menu row | add `fg` at alpha 0.08 |
| selected tablet pressed | `onSel` at alpha 0.08 |
| disabled | the whole control at opacity 0.5 |
| focus ring | `fg2`, 2 dp stroke, 2 dp outside, cut grown by 2 dp |
| scroll thumb | `vein` |

`mix(a, b, t)` is a per-channel linear blend including alpha:
`a + (b − a) × t`, rounded half up to a whole byte.

## 4. Accent ink

There is no re-toning pass. `goldInk`, `jasper`, `fg2` and `fg3` sit at
ladder steps chosen so that every hue, at its capped chroma and at every
tint, holds the floors in [contrast targets](#contrast-targets) on the planes
it is set on. A change to the ladder must keep that, measured around the whole
hue circle, not on one wallpaper.

Keep the gold fill as `gold`; use `goldInk` for gold text and hairlines on
stone. The danger fill is `jasper`, and the ink on it is `inkOn(jasper)`:
black or white, whichever has the better contrast. An edition's `onGold` is
chosen the same way.

## 5. The signature

The signature is the ryoku.dev stock: black paper, bone ink and the dark
orange sun as the one action, in dark mode; a warm grey paper with
the same sun in light mode, never cream. It is what a Lithic app paints with no
Ryoku palette (another distribution, a CI render, a fresh box), and its
neutrals are what a forced dark or light mode keeps. The four tables
(`signature.dark`, `darkHigh`, `light`, `lightHigh`) are canonical in
`tokens.json`; a stylesheet or theme file with Lithic hex values anywhere
else is a defect.

## 6. The edition colour

Without a Ryoku palette, `gold` is the host edition's signal where it has
one. Resolve it from `/etc/os-release`: try `ID`, then each lower-cased word
of `ID_LIKE`. The table in `tokens.json` follows ryoku.dev's edition stocks:
Fedora, NixOS and Void carry their own signal; Arch, CachyOS and unknown
hosts keep the sun. The selection stays bone; an edition colour never becomes
`sel`.

## 7. Following changes live

The palette changes when the wallpaper changes or the user picks a theme. Watch
the **directories** of the three files with inotify (the daemon writes by
rename, so a watch on the file itself is lost on the first change), coalesce
events for ~150 ms, re-run §2 and §3, and only repaint when the resulting
roles differ. A `kit.MineralField` style component should snap, not animate,
when the palette changes under the same tone; the 320 ms mineral-field
transition is for a state change, not a wallpaper change. A change of
contrast or tint snaps the same way.

In QML, Ryoku's own singletons read the files with `FileView` and parse the
text (see `ryoku/shell/quickshell/plugins/kit/Singletons/Scheme.qml` in the
Ryoku repository). In a browser page the host must hand the JSON in; a page
cannot read `~/.cache`.

## 8. Verifying the hookup

A Lithic surface is correctly wired when all of these hold on a Ryoku desktop:

1. Change the wallpaper: the ground, panes, slabs, fields, selection and gold
   change; jade, lapis and amber do not.
2. Pick a named theme in the Hub: the app follows `themePalette`, not the
   wallpaper.
3. Force light in the app on a dark wallpaper: neutrals become the signature
   light stock, while the accents (the selection included) still come from
   the palette.
4. Switch contrast to high: the stones, slabs, fields and veins move away
   from the ground and the inks move out, while the ground stays. Switch tint
   to low and medium under a wallpaper: the ground, stones, slabs, fields,
   veins and edges take more of the palette's colour, while the ink keeps
   its own. Without a palette the tint changes nothing.
5. Every pair in [contrast targets](#contrast-targets) holds in both modes,
   at both contrasts and every tint.
6. A danger fill uses `inkOn(jasper)`.
7. A source search finds colour values only in the signature tables, the
   three fixed minerals and the edition table.

Each library carries an equivalent palette check and consumes the same
`tokens.json` roles and thresholds; `library/parity/check.sh` proves the
three resolvers agree, looks included.

### Contrast targets

The WCAG ratios are floors: each holds for every hue around the circle at the
capped chroma and at every tint, and for the signature tables. APCA (the
absolute Lc value of the WCAG 3 draft method) is measured beside them,
because it judges thin type and lone hairlines on dark planes better than a
ratio does.

| What | On | WCAG | APCA |
|---|---|---|---|
| `goldInk`, `jasper`, `fg3` | `stone1`, `stone2` and `slab` | 4.6:1 or more, 5:1 at high contrast | |
| `jade`, `lapis`, `amber` | `stone1`, `stone2` and `slab` | 4.6:1 or more at both contrasts | |
| `fg2` | the same and `field` | 4.6:1 or more, 5:1 at high contrast | |
| `fg`, and `fg2` set as `detail` type | `stone2` and `slab` | | measured, see below |
| any lone edge (a hairline that is the only thing marking a control, such as a field's `edge`) | its plane | | Lc 15 or more |
| a control with a cut: its fill or its hairline | its plane | | Lc 15 or more |
| `sel` | every plane | 3:1 or more | |
| `onSel` | `sel` | 4.5:1 or more | |

On the dark work pane the signature's bone, ryoku.dev's own ink, reads at
Lc 69 for `fg` and Lc 54 for `fg2`; the site sets it on black, where it
reads higher. High contrast lifts them to Lc 83 and Lc 68, and the light
signature reads at Lc 96 and Lc 79.
