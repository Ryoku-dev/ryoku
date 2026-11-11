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
both as settings. The mapping runs in five steps, in this order, and every
stack runs the same steps with the same rounding:

1. Start from the signature roles for the resolved mode (§5) and map the
   neutrals, the scrim and the accents below.
2. Mix the selection and derive `onSel2` ([the selection](#the-selection)).
3. Apply the contrast, then the tint ([contrast and tint](#contrast-and-tint)).
4. Derive `edge` ([the edge](#the-edge)).
5. Re-tone `goldInk` and `jasper` on the final `stone2` (§4).

### Neutrals (only when the mode is not forced)

| Lithic role | Dark palette | Light palette |
|---|---|---|
| `ground` | `surfaceContainerLowest` | `surfaceContainerHighest` |
| `stone1` | `surface` | `surfaceContainerHigh` |
| `stone2` | `surfaceContainerLow` | `surfaceContainerLowest` |
| `stone3` | `surfaceContainer` | `surfaceContainer` |
| `stone4` | `surfaceContainerHigh` | `surfaceContainerLowest` |
| `slab` | `surfaceContainer` | `surfaceContainerLowest` |
| `field` | `surfaceContainerHighest` | `surfaceContainerHighest` |
| `vein` | `outlineVariant` | `outlineVariant` |
| `fg` | `onSurface` | `onSurface` |
| `fg2` | `onSurfaceVariant` | `onSurfaceVariant` |
| `fg3` | `outline` | `outline` |
| `scrim` | `scrim` at alpha 0.62 | `scrim` at alpha 0.45 |

Light palettes run the surface ramp the other way: the darkest container
becomes the ground and the lightest becomes the work pane, so the panes still
sit up from the ground. In light mode `stone4` and `slab` equal `stone2`; a
menu or a slab on the work pane is set off by its vein hairline, not by tone.

A slab lifts from the plane it stands on and never sinks into it: it takes
its own role, one step up the ramp from the work pane in dark mode, instead
of a lower stone. `field` is the top of the container ramp, so a field stands
clear of the work pane, a slab and a dialog alike.

When the user **forces** dark or light in the app, the neutrals stay the
signature set for that mode; only the accents below are taken from the
palette. This keeps a forced light app readable on a dark wallpaper.

### What one step of the ramp can carry

The neutrals are not evenly spaced. A wallpaper palette writes the three
containers in the middle of the ramp close together, so `stone2` and `stone3`,
taken from neighbouring containers, read as one plane on most screens. The
signature fallback is spaced wider and hides this: judge a surface under a
real wallpaper palette, in follow mode, at standard contrast.

So one step is enough for a change and not for an edge:

- **A change** is seen because it happens. A row that goes from `stone2` to
  `stone3` under the pointer is read as hovered. Hover, press and the tracks
  inside a control need one step and nothing else.
- **An edge that has to be seen at rest** needs more. A thing that must read
  as its own object either stands two roles from the plane under it (a tonal
  plaque or a resting chip filled with `field` on the `stone2` work pane or
  on a slab), or, when it is one role away or less, carries a 1 dp hairline.
  Slabs, menus and dialogs carry a `vein` hairline, and so does the off track
  of a toggle. A field, search or picker anchor carries a 1 dp `edge`
  hairline wherever it stands, because the hairline marks where to type, not
  only where the plane ends; keyboard focus turns it `fg2`.

A page that leaves a field with no hairline on the work pane has drawn a
field nobody can find until they press it.

### Accents (always)

| Lithic role | Palette | Note |
|---|---|---|
| `gold` | `primary` | the action fill |
| `onGold` | `onPrimary` | ink on the action fill |
| `goldInk` | `primary`, re-toned | gold type and hairlines on stone; see §4 |
| `sel` | `secondaryContainer` mixed toward `primaryContainer` | the selection; see [the selection](#the-selection) |
| `onSel` | `onSecondaryContainer` | ink on a selection |
| `onSel2` | `mix(onSel → sel, 0.45)` | quiet ink on a selection (facts, proofs) |
| `jasper` | `error`, re-toned | failed, unreachable, destructive; see §4 |
| tint | `primaryContainer` | not a role of its own: what the stones take under a tint; see [contrast and tint](#contrast-and-tint) |

### The selection

The selection takes the wallpaper's colour. When the palette has
`secondaryContainer`:

```
sel := secondaryContainer
if palette has primaryContainer:
    sel = mix(secondaryContainer → primaryContainer, alpha.selMix)   # 0.5
onSel  := onSecondaryContainer
onSel2 := mix(onSel → sel, alpha.onSel2)                             # 0.45
```

Without a palette the signature `sel` and `onSel` stay, which are bone. A
forced dark or light mode still takes its accents from the palette, so it
gets the mixed selection too. A selected row, chip or segment is the `sel`
fill plus a place blade in `onSel`; the fill alone never marks the place
(see [components.md](components.md)).

### Contrast and tint

Both run after the mapping and the selection, contrast first.

**Contrast high** spreads the stones away from the ground. For each of
`stone1`, `stone2`, `stone3`, `stone4`, `slab`, `field` and `vein`, per
channel:

```
X := ground + (1 + alpha.contrastHigh) × (X − ground)   # alpha.contrastHigh 0.6
     rounded half up, clamped to 0..255
fg3 := mix(fg3 → fg, alpha.contrastHighFg3)             # 0.3
```

`ground`, `fg` and `fg2` do not change. It applies to every source: a
wallpaper, a scheme, the signature and the forced modes. Standard changes
nothing.

**Tint low or medium** gives the stones a touch of the palette's colour. When
the palette has `primaryContainer`, each of `ground`, `stone1`, `stone2`,
`stone3`, `stone4`, `slab` and `field` becomes
`mix(X → primaryContainer, alpha.tintLow)` (0.06) for low or
`mix(X → primaryContainer, alpha.tintMedium)` (0.10) for medium. Without that
role (the signature, an edition accent) the tint does nothing. `vein`, `edge`
and every ink keep their colour.

A change of contrast or tint re-runs §3 and snaps, like a palette change.

### The edge

```
edge := mix(vein → fg3, alpha.edge)   # 0.5
```

`edge` is derived after the contrast and tint steps, from the final `vein`
and `fg3`. It is the hairline of a control that has to be found at rest on
its own: a field, a search, a picker anchor or a stepper's field. `vein`
stays the hairline of planes that are set off anyway (slabs, menus, dialogs,
the tab baseline, row dividers).

### Fixed minerals

`jade` (alive), `lapis` (becoming) and `amber` (needs you) do **not** come
from the palette. They carry meaning, and a Ryoku user must read "alive" the
same way on every wallpaper. Their dark and light values are canonical in
`tokens.json` and are consumed through each stack library. They match
Ryoport's `ui/theme/theme.go` `SetStones` values. Do not invent replacements
or allow a scheme to override them. Jasper follows the palette's `error`, but
is re-toned so it remains readable on `stone2`.

With no palette, the signature fallback supplies all accents. On a known
non-Ryoku host, the edition colour replaces `gold`; derive `onGold` with
`inkOn(gold)` and re-tone `goldInk` against `stone2`.

### Derived tones

Compute these from the roles above; never pick them.

| Derived | Recipe |
|---|---|
| `mineralField(tone)` mineral field | `mix(stone2, jade, 0.17)`; `mix(stone2, mineral, 0.15)` for lapis, amber, jasper; idle is `stone3` |
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
`a + (b − a) × t`, rounded half up to a whole byte. The resolver rounds after
every step of this section, so the stacks agree.

## 4. Re-toning accent ink

Palette accents are tuned for fills, not necessarily for text on stone.
Apply the same contrast function to `primary` for `goldInk` and to `error`
for `jasper`, and to the signature's own `goldInk` and `jasper` when there is
no palette. Push HSL lightness until each reaches **4.5:1** against the final
`stone2`, after the contrast and tint of §3 (the signature values already pass
at standard contrast; high contrast can move them):

```
contrastTone(accent, stone2, dark):
    if contrast(accent, stone2) >= 4.5: return accent
    h, s, l := toHSL(accent)
    for step in 1..255:
        t  := step / 255
        l' := dark ? l + (1 - l) * t : l * (1 - t)
        c  := fromHSL(h, s, l')
        if contrast(c, stone2) >= 4.5: return c
    return dark ? white : black
```

`contrast(a, b) = (L1 + 0.05) / (L2 + 0.05)` with WCAG relative luminance
and linearised sRGB channels.

Keep the gold fill as `primary`. Use the re-toned `goldInk` for gold text and
hairlines on stone. Keep the danger fill as the re-toned `jasper`. Ink on
that fill is `inkOn(jasper)`: black or white, whichever has the better
contrast.

For any fallback action accent, `onGold` is likewise black or white,
whichever reads better.

## 5. The signature fallback

The signature neutrals exist for two cases only: a box with no Ryoku palette
(another distribution, a CI render), and a forced dark or light mode where
the neutrals must not follow the wallpaper. They are a warm marble in light
mode and a graphite in dark mode. Take them from `ui/theme/theme.go`
(`Signature`) and mark them in code as the fallback; a stylesheet or theme
file with Lithic hex values anywhere else is a defect.

## 6. The edition colour

Without a Ryoku palette, including on a non-Ryoku host, `primary` is the host
distribution's own colour. Resolve it from `/etc/os-release`: try `ID`, then
each lower-cased word of `ID_LIKE`. The canonical table in `tokens.json`
comes from `internal/palette/edition.go` and covers Arch, CachyOS, Fedora,
Void, NixOS, Ubuntu, Debian, openSUSE, Manjaro, EndeavourOS, Linux Mint,
Pop!_OS, Gentoo and Alpine. Unknown hosts keep the signature gold. Selection
stays bone; an edition colour never becomes `sel`.

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
   marble, while accents (the mixed selection included) still come from the
   palette and are re-toned.
4. Switch contrast to high: the stones, slabs, fields and veins move away
   from the ground while the ground, `fg` and `fg2` stay. Switch tint to low
   and medium under a wallpaper: the ground and stones take the palette's
   colour step by step, while veins, edges and ink keep theirs. Without a
   palette the tint changes nothing.
5. Every pair in [contrast targets](#contrast-targets) holds in both modes,
   at both contrasts and every tint.
6. A danger fill uses `inkOn(jasper)`.
7. A source search finds colour values only in the marked signature fallback,
   the three fixed minerals and the edition table.

Each library carries an equivalent palette check and consumes the same
`tokens.json` roles and thresholds; `library/parity/check.sh` proves the
three resolvers agree, looks included.

### Contrast targets

WCAG ratios keep the floor the toning in §4 guarantees. APCA (the absolute
Lc value of the WCAG 3 draft method) is measured beside them, because it
judges thin type and lone hairlines on dark planes better than a ratio does.

| What | On | WCAG | APCA |
|---|---|---|---|
| `goldInk`, `jasper` | `stone2` | 4.5:1 or more | |
| `fg`, and `fg2` set as `detail` type | `stone2` and `slab` | | Lc 75 or more |
| any lone edge (a hairline that is the only thing marking a control, such as a field's `edge`) | its plane | | Lc 15 or more |
| a control with a cut: its fill or its hairline | its plane | | Lc 15 or more |
| `sel`, or its place blade | its plane | 3:1 or more | |
