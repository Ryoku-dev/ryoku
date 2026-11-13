# UI and UX

The desktop is **Lithic**: cut stone, not molded plastic. Flat stone planes
set a step apart, every corner cut at 45 degrees or left square, bone for where
you are, gold for what you can do, and a small set of minerals that mean a state
only when they sit beside a word. Geologica carries everything a person
reads, Space Mono carries data, and motion answers a cause and then stops.

Lithic has no colours of its own. Every colour on the desktop is a role, and the
roles are filled from the palette the theme daemon already publishes, so the
Hub, the apps, the shell surfaces and Ryoport all retint together from the
wallpaper or a named theme. That rule is older than Lithic and it is the reason
Lithic could replace the old look without a second palette.

The one place the look is defined is `ryoku/ui/Singletons/Tokens.qml`: every
surface reads its colour, type, geometry and motion from there, and nothing
hardcodes a value. This doc is how the language is applied across the QML
desktop: the roles, the type, the cuts, the shared primitives, the pages, the
surfaces and the motion. The UI is Quickshell (QML) in
`ryoku/shell/quickshell/` (plus Ryoku Settings in `ryoku/hub/quickshell/` and the
apps in `ryoku/apps/`), driven by the `ryoku-shell` daemon.

The design language itself is specified in [docs/lithic/](lithic/README.md):
the principles, the palette recipe, the component catalogue, layout, motion,
copy, the review checklist, the design sheets and verified web, QML and Go
starting kits. Ryoport (`ryoku-dev/ryoport`) is its reference implementation.
This page is how that language applies to the Ryoku desktop; when a detail is
not here, `docs/lithic/` is where to look next.

![Lithic on one sheet](lithic/media/sheet-board.png)

*The language on one sheet, rendered from this machine's wallpaper palette:
the principles, the cut grammar, the stones, the type ramp and the motion.*

## Design language

Cut, not molded. Nine rules, and everything below is one of them applied.

1. **Cut, not molded.** Corners are cut at 45 degrees or square, never rounded.
   The cut follows the scale of the thing: 16 on panes and dialogs, 12 for a
   row's state notch, 7 on buttons and fields, 6 on small controls, 4 on a
   toggle track, 3 on tokens. Only content (a logo, a chart, the VRAM ring) may
   curve. The outer window rounding is the compositor's and the user's knob;
   inside our surfaces nothing rounds.
2. **Flat planes.** Every surface is one flat tone. Depth is the order of tones:
   ground, stone 1, stone 2, stone 3, stone 4, then bone. No gradient, glow,
   blur, inner shadow or inset well. An overlay is a solid scrim and a slab one
   tone up with a 1 px vein hairline. The frame's own shadow over the wallpaper
   is a different thing and lives in `docs/frame.md`.
3. **Bone is where you are.** The selected row, the current rail destination,
   the current filter, the on half of a toggle: all are selection slabs in the
   `sel` role with `onSel` ink. Nothing else uses it.
4. **Gold is what you can do.** The `gold` role fills consequential actions
   only (Save, Connect, Start, Install, Sign in). Secondary actions are a gold
   hairline and gold ink. Navigation and view controls are stone. Gold never
   decorates and never shows status. On a Ryoku desktop the gold is the
   palette's `primary`, so it is whatever the wallpaper made it.
5. **Stones are states, beside their word.** Jade is alive, lapis is becoming,
   amber needs you, jasper failed, idle is a vein outline. A state is three
   things at once: its mineral, a shape (a notch, a flake, a mineral field) and
   a plain word. Colour never carries state alone, and a dot is never a state.
6. **The blade marks progress and place.** A bar whose leading edge is cut at
   45 degrees: the active tab's underline, a job's progress, a meter.
7. **Engraved data.** Charts are a fine foreground line over a 45 degree hatch,
   labels in Space Mono, no coloured area fills.
8. **One family, set monumentally.** One line per view gets the `name` size.
   Everything else is at working size, sentence case, never all caps.
9. **Motion answers a cause and stops.** One curve, a short table of durations,
   no frame drawn when nothing changed. Reduced motion makes every transition
   instant.

The words Lithic uses for its parts, so code, comments and reviews mean the
same thing: a **plaque** is a button, a **tablet** is a row, the **notch** is
the cut corner that carries a row's state, a **flake** is the standalone state
token, a **mineral field** is the tinted header of the selected thing, a
**blade** is a cut progress or selection bar, a **slab** is a stone 1 container,
a **claim** is a state word and its **proof** is the fact behind it, and
**chisel** is the one easing curve.

Two things stay Ryoku's whatever the palette does. The **力 seal** is the brand
mark, drawn in its own fixed vermillion, and it is used as a mark (the Home
mark atop Ryoku Settings' rail, the welcome window, the lockscreen family), never as decoration or
state. And the three meaning minerals, jade, lapis and amber, never follow the
wallpaper, so "alive" reads the same on every rice; jasper does follow the
palette's `error`, so destructive controls agree with the rest of the desktop.

## Roles: never hardcode a value

Every surface reads its look from `Ryoku.Ui`. One module, imported by the
shell's configs, the Hub and the apps. `import Ryoku.Ui.Singletons` and read
`Tokens`; never write a hex, a font name, a cut or a duration in a component.

    import Ryoku.Ui
    import Ryoku.Ui.Singletons

### Colour resolves through a chain, it is not a constant

`Tokens` does not hold colours. It holds a *resolution*: every colour token is
`role(materialRole, signatureDefault)`, and `role()` walks three layers in order.

1. A fixed named scheme (`shell.json` `themePalette`) wins, if one is set.
2. Otherwise the live wallpaper palette (`~/.cache/ryoku/colors.json`, Material
   roles written by the daemon), while *Match wallpaper* is on.
3. Otherwise the compiled signature default.

So every Ryoku app retints on any scheme change, a named theme or a wallpaper
switch, with no colour maths of its own. A half-written role (a `colors.json`
caught mid-write, a scheme missing a key) falls through to the next layer rather
than painting black. The three files are read as raw text and watched, because
the `on*` Material role names defeat `JsonAdapter`'s signal-handler grammar and a
removed `themePalette` key only reads as absent from raw text.

Dark or light is the palette's `surface` luma (`(0.299 R + 0.587 G + 0.114 B)
/ 255 <= 0.5` is dark) unless the user forced a mode in the app. The surface
ramp maps one way in dark and the other way in light, so the panes always stand
up from the ground:

|Token|Dark palette role|Light palette role|What it is|
|---|---|---|---|
|`ground`|`surfaceContainerLowest`|`surfaceContainerHighest`|behind the panes|
|`stone1`|`surface`|`surfaceContainerHigh`|the rail, the list pane, slabs|
|`stone2`|`surfaceContainerLow`|`surfaceContainerLowest`|the work pane, row tablets|
|`stone3`|`surfaceContainer`|`surfaceContainer`|hover, fields, tracks|
|`stone4`|`surfaceContainerHigh`|`surfaceContainerLowest`|menus, dialogs, tooltips|
|`vein`|`surfaceContainerHighest`|`outlineVariant`|hairlines, quiet outlines, idle flakes|
|`fg`|`onSurface`|`onSurface`|strong type|
|`fg2`|`onSurfaceVariant`|`onSurfaceVariant`|body and quiet type|
|`fg3`|`outline`|`outline`|faint type, facts, captions|
|`sel`|`secondaryContainer`|`secondaryContainer`|bone, the selection slab|
|`onSel`|`onSecondaryContainer`|`onSecondaryContainer`|ink on a selection|
|`onSel2`|derived|derived|`onSel` mixed 45 percent toward `sel`, quiet ink on a selection|
|`gold`|`primary`|`primary`|the action fill|
|`onGold`|`onPrimary`|`onPrimary`|ink on the action fill|
|`goldInk`|derived|derived|`primary` pushed along HSL lightness until it reads 4.5:1 on `stone2`|
|`jasper`|`error`|`error`|failed, unreachable, destructive|
|`jade`, `lapis`, `amber`|fixed|fixed|alive, becoming, needs you|
|`scrim`|`scrim` at 0.62|`scrim` at 0.45|behind a dialog|

When the user forces dark or light in an app, the neutrals stay the signature
set for that mode and only the accents (`gold`, `sel`, `jasper`) come from the
palette, so a forced light app stays readable on a dark wallpaper. The
signature neutrals are a graphite in dark and a warm marble in light; they are
the fallback for a box with no palette and for forced modes, and the only hex
values allowed in a surface besides the three minerals and the seal.

Derived tones are computed, never picked: a mineral field is `stone2` with 17
percent jade or 15 percent of any other mineral mixed in (idle is `stone3`);
hover on a filled plaque mixes 10 percent white in, on a transparent plaque
lays the ink at 10 percent, and on a tablet, menu row or rail tile steps one
plane up; a press adds `fg` at 6 to 8 percent; disabled is the whole control at
38 percent; the focus ring is `fg2`, 2 px, 2 px outside the cut. The hairline
and tint tokens that did this for the old look (`line`, `lineSoft`,
`lineStrong`, `tint5`, `tint10`, `tint16`) map onto `vein` and those recipes.

### The shell resolves the same palette through `Scheme`

The Hub and the apps read `Tokens`. The shell surfaces read
`shell/services/Scheme.qml`, which resolves the same daemon Material roles for
the shell process, and each surface keeps a thin local `Theme.qml` that names the
roles *that surface* uses. Those adapters are 14 to 133 lines and hold no
palette of their own:

|Adapter|Reads|
|---|---|
|`shell/services/Theme.qml`, `desktop/`, `overview/`, `plugins/kit/`|`Scheme`|
|`launcher/shared/`|`Scheme` and `Tokens`|
|`ryoshot/`, `ryopin/`, `welcome/`|`Tokens`|
|`wallpaper/`, `wallpaper/switcher/`|the default ground only, for letterbox margins|

Two files are genuinely their own thing and should stay that way: an item's own
`Theme.qml` (QS Bar's, in the catalogue repo, is that bar style's entire
configuration surface and owns its own look by contract, see
`docs/subshells.md`) and
`hub/quickshell/Singletons/Theme.qml`, a leftover of the old website palette that
survives in exactly one preview file and should go the next time that preview is
touched.

The rule this replaced is worth remembering. There used to be eleven
`Singletons/Theme.qml` copies kept in step by hand. They were not in step. They
had drifted into three families with the same token names carrying different
values: `Theme.border` was a width in the Hub and a colour in the pill, so a file
moved between them broke silently, and `border2` exists because the width token
was evicted to make room. A local `Theme.qml` today is allowed to *name* roles; it
is not allowed to *define* values. If a value is missing from Tokens, add it to
Tokens.

### Where the module lives

The module lives at `ryoku/ui/`. An installed system reads it from
`/usr/lib/qt6/qml/Ryoku/Ui`, which Qt resolves unaided. A deploy.sh checkout puts
it under `~/.local/lib/qt6/qml`, and only the daemon injects that path
(`ipc/daemon.go`), so `qs -c hub` from a keybind cannot see it without
`QML_IMPORT_PATH`. `hyprland/modules/env.lua` sets it for the session. If an
import fails in dev and works on an installed box, that is why.

Packages ship it compiled. `ryoku/ui/CMakeLists.txt` reads the two `qmldir`
files and builds `Ryoku.Ui` and `Ryoku.Ui.Singletons` as plugin-only QML modules,
so qmlcachegen turns every file into bytecode inside one `.so` per module (bytecode
only, no generated C++; the CMake file has the measurements). The ryoku-desktop
build runs `ryoku/ui/build.sh`, which installs only `qmldir`, the qmltypes and
the `.so`. A dev box keeps copying the
sources (`ryoku/ui/install.sh`, from deploy.sh), which Qt's disk cache compiles on
first load, so an edit shows up on the next reload with no build. Both scripts
empty `Ryoku/Ui` first, so one import root never holds both forms, and
`Ryoku.Ui.Singletons` always resolves from the same root as `Ryoku.Ui`.

Two rules follow from compiling. The qmldir files stay the one list of types, so
a new type is a qmldir line, as before; an entry the CMake file does not know
stops the build. And a file in the module imports the singletons as
`import Ryoku.Ui.Singletons`, never `import "Singletons"`, and a singleton never
reaches outside `Singletons/` by relative path: in the compiled form each module
is its own resource tree, and a directory import into the other one finds
nothing until that module's plugin happens to be loaded.

### Gio apps are Ryoku surfaces too

Rashin's app (`ryoku/rashin/cmd/rashin-app`) draws with Gio and the Lithic Go
library (`docs/lithic/library/go`), the same stack as Ryotunes, so it cannot
import `Tokens` either, but it follows them. The library's `palette` package
resolves the same Material roles through the same chain (a named scheme in
`shell.json`, then `~/.cache/ryoku/colors.json` while Match wallpaper is on,
then the signature default) and its `kit` draws the cuts, the type and the
motion from the Lithic rules. The Rashin page in Ryoku Settings (Apps > Rashin) reads `Tokens` like any
other page. If the app needs a value, it takes it from a role, never a hex of
its own.

### What follows the wallpaper, and what does not

The daemon runs matugen over the wallpaper and publishes a full Material role set
to `~/.cache/ryoku/colors.json`, along with the wallpaper's own luminance.
Material's tone system already answers "what reads on this panel", so the shell
does not do that arithmetic itself.

- **The whole role set follows the wallpaper, not just the accent.** Ground,
  stones, vein, type, bone and gold all resolve through the chain above. A
  named theme in `shell.json` pins them instead.
- **`Ink` answers the question a role cannot.** A Material role knows what reads
  on a *panel*; it knows nothing about a surface floating on the wallpaper (the
  desktop clock, the spectrum, a widget with its backing set to none). Handed
  `onSurface`, those paint the tone Material chose for a panel that is not there,
  which on a light scheme is near-black. So `Ink.legible(bg, role, minRatio)`
  corrects a role against a colour you actually have, and `Ink.inkOver()` /
  `Ink.accentOver()` pick a tone off matugen's own ramps against the published
  wallpaper luminance. Both work in CIE L*, where a tone distance is a contrast
  budget: 40 apart clears 3:1, 50 apart clears 4.5:1. Text gets the full 0..100
  range; an accent is held between 30 and 88, because outside that every hue
  reads as black or white and a mid-tone wallpaper would otherwise turn the
  spectrum into bars of soot. `goldInk` is the same idea applied once, in HSL,
  for gold type on the work pane.
- **The minerals are never derived.** Jade, lapis and amber are fixed in
  `Tokens` for dark and for light stone, and a theme cannot override them.
- **The 力 seal is never derived.** `Theme.brand` is a fixed vermillion. A seal
  is a seal on any wallpaper. Rashin wears a sibling seal, not the 力: its mark
  (`assets/brand/rashin-mark.svg`) is the same tile carrying 十, the cross that
  ends 針 (needle) and is also the compass card at its four cardinals; the
  north-south stroke is the needle, with north painted in the brand vermillion
  and the pivot left as tile. One geometry, three cuts: the colour master (app
  icon, lockup), a one-colour evenodd master, and a small cut without the pivot
  for 16 to 32 px (the console favicon and the Ask bar's header). The lockup
  (`rashin-lockup.svg`) sets Rashin in Space Grotesk Medium with the 羅針 gloss
  at a shared baseline; below 48 px tall use the mark alone.
- **QS Bar is the one surface that wears the whole palette.** It retints every
  slot from the wallpaper's seven colour slots (`color01..color07`, mapped onto
  the ANSI roles), not just the accent, so it reads in full colour like a
  terminal theme. It is a deliberate exception, and it is a store bar style, not
  part of Ryoku; the shipped subshells, the Hub, the apps, the frame and the
  monochrome Sumi bar hold the line above. See `docs/subshells.md`.

So the rice wins inside an envelope the design enforces. Write that down rather
than the reverse: the envelope is the design.

## Type

Self-hosted, no CDN. Two families and one seal face, one role each:

|Role|Family|Token|
|---|---|---|
|Everything a person reads: names, sentences, labels, prose, buttons|**Geologica**|`Tokens.ui` (a user's configured UI font overrides it)|
|Literal data only: addresses, ports, paths, ids, counts, chart ticks|**Space Mono**|`Tokens.mono`|
|The 力 and 十 seals, and nothing else|**Noto Sans CJK JP**|`Tokens.jp`|

One scale, twelve steps, and a step is a role rather than a number to pick (sp,
size / line):

|Token|Size|Weight|For|
|---|---|---|---|
|`fName`|52 / 56|Bold|the selected thing's name, the one monument in a view|
|`fSentence`|34 / 40|Bold|a summary sentence; the name when the pane is under 640 wide|
|`fSection`|21 / 28|Bold|page titles, large section heads|
|`fTitle`|15 / 20|Medium|section titles inside a pane|
|`fRow`|15 / 20|Medium|row names|
|`fBody`|14 / 20|Regular|prose, form content|
|`fProof`|15 / 21|Regular|the sentence under a name|
|`fLabel`|13 / 16|Medium|plaques, tabs, filters, controls|
|`fCaption`|12 / 16|Regular|facts, descriptions, secondary lines|
|`fGroup`|12 / 16|Medium|list group headers|
|`fMono`|12 / 16|Regular, Space Mono|data|
|`fMonoSm`|11 / 15|Regular, Space Mono|proofs, counts, chart ticks|

Sentence case everywhere, no tracking, no all caps, not even for group headers
or eyebrows. Mono is not the UI face: it carries what is literally valid in a
config file (keys, ranges, defaults, paths, ids) and nothing a human reads as
language, numerals in prose included. Setting a surface in mono makes it a
terminal, and setting a label in caps makes it a poster; Lithic is neither.

## Shape: the cut scale

A cut is a 45 degree chamfer measured along one edge, and the eight-point
outline of a rectangle with cut `c` on every corner is
`(x0+c, y0) (x1-c, y0) (x1, y0+c) (x1, y1-c) (x1-c, y1) (x0+c, y1) (x0, y1-c)
(x0, y0+c)`. Each corner may take its own cut; a pane cuts only its two top
corners.

|Token|dp|Applied to|
|---|---|---|
|`cutToken`|3|flakes, small tokens, tooltips|
|`cutToggle`|4|the toggle track (its knob cuts 2)|
|`cutSmall`|6|small plaques (32 high), filter chips, icon plaques, compact rail tiles|
|`cutMid`|7|plaques (40 high), fields, pickers, slabs, rail tiles|
|`cutNotch`|12|a tablet's state corner, the rail's update marker|
|`cutLarge`|16|panes (top corners only), dialogs|
|menus and placards|8|overlays one tone up, with a hairline|

A press deepens a control's cut by 4 and sinks its content by 1. Hairlines are
`Tokens.border` 1 px in `vein`, drawn wholly inside the outline; a stroked cut
shrinks its inner path's cut by `width × (2 − √2)` so the stroke stays even
around the chamfer. The other cut shapes are the **notch** (a right triangle
filling a corner), the **blade** (a bar whose trailing top corner is cut by its
own height), the **flake** (a parallelogram 1.4 × 0.7 of its size with 45
degree short sides: vein outline idle, mineral fill active, part filled while
becoming) and the **octagon**, the only other polygon, for the empty state's
icon frame and the latency rings of a map. Status dots, pills and rounded
toggle knobs are the old look and do not come back.

Fixed furniture: a tablet is `rowH` 56 (48 in compact density), a plaque
`ctlH` 40 (32 small), a field `fieldH` 44, a rail `railW` 88 at expanded width
and 72 at medium. A page never invents these.

## Space

The spacing run is `s1` 4, `s2` 8, `s3` 13, `s4` 21, `s5` 34, `s6` 55. Nothing
between them. Tablets sit 4 apart; plaques in an act row 8; a mark and its copy
13; a slab's inset and the gap under a header's proof 21; a dialog's padding
34; major regions of a full page 55. The work pane keeps 36 at its sides
(`contentInset`), which is the one value outside the run, because it is the
pane's margin and not a gap between things.

## Layout

Panes stand on the ground: every pane starts 10 below the window's top edge,
sits 8 from its neighbours with ground showing between, cuts its two top
corners at `cutLarge` and stays square at the bottom. The rail and the list
pane are `stone1`; the work pane is `stone2`. Structure changes at a size-class
boundary and a wide layout is never squeezed:

|Class|Window width|Rail|List pane|Work pane|
|---|---|---|---|---|
|expanded|1280 and up|88, 64 tiles with labels|384|the rest|
|medium|760 to 1279|72, 56 tiles, icons only|320|the rest|
|compact|under 760|none, a 64 bottom bar|full width|replaces the list, with Back|

A full page (a settings page) drops the list and takes the remaining width: at
expanded width a `min(available, 994)` column with a sticky 200 section index
34 to its left, at medium a `min(available, 760)` column with a row of filter
chips above it, at compact the full width under a plain title; 34 above and
below, 36 at the sides.

### A page uses the window it was given

The Hub opens page-wide, and a page that refuses the width it was given is the
bug this section exists to prevent.

- **The body takes the width it is given.** A page's scroll or body region never
  caps itself (no `Math.min(parent.width, Tokens.contentMax)` on the body). The
  column above caps itself as a column; only *prose* keeps a reading cap: a
  paragraph, a help line, a release note, a blurb.
- **The head sits on the body's grid.** A title and its proof start at the
  body's left inset and span its width, so a page's title aligns with its first
  slab rather than floating in the middle of the window.
- **Content anchors to the top, always.** No block is centred by height, and no
  page spreads spare room into its rows: a block whose place depends on its height
  visibly hops as slabs measure in ("flickers when I switch to it"), and the same
  first slab then sits at two different heights on two pages. One place, always
  the top, is what lets a reader build a map of the page. A sparse page is honest
  empty stone below the slabs; the answer to thinness is a merge, not padding.
- **A page that fits does not scroll, and a page that overflows does.** Every
  scroller carries a `ScrollRail` and declares `WheelScroll { }` inside it, because
  a plain `Flickable` does not answer the wheel on this stack (see Motion below).
  A scroller with nothing to scroll clamps and stays put. The rail's thumb is a
  3 wide `vein` blade.

### Motion and input

- **Every scroller answers the wheel.** `WheelScroll` (declared inside the
  `Flickable`/`ListView`, beside its `ScrollRail`) scrolls it one notch per 120
  units of `angleDelta`, clamped to its bounds, and a notch glides over 140 ms. A
  handler only receives events over its own parent, so it cannot live in the
  rail: a handler inside a `Flickable` is reparented to that flickable's
  `contentItem`, which is why the component walks up to the item that owns
  `contentHeight`. A control that wants the wheel for itself keeps it, since an
  inner handler accepts the event first.
- **A page swap does not reflow.** The incoming page fades in with an 8 px rise
  over `Tokens.notch`, and the page's own first layout is synchronous, so nothing
  moves after the fade starts. Length-dependent placement and post-load
  measurement timers are what make a page jump; there are none.
- **Escape closes the topmost thing and nothing else:** an open menu or
  popover, then the palette, then a draft, then the open detail. A click outside
  a popover or dialog closes it and is swallowed, and while it animates shut it
  blocks pointer input underneath so a dismissing click cannot land on a
  control. Only one popover is open at a time.

### Disclosure, not walls

- **A page leads with what a user came for.** A rarely-touched cluster folds
  (`SettingCard.expanded: false`) and its header says what it hides
  (`SettingCard.summary`: "4 switches"), because a folded slab with no trace of
  its contents reads as an empty one.
- **A deep knob goes under its section's More options**, a fold at the foot of
  the section's slab, and stays reachable from search, which opens the fold when
  it lands on the row. Nothing is ever buried beyond a search.
- **A toggle is only for a true binary.** A choice among modes is a `Seg`, a
  family of related on/offs is a `Multi`, a number is a `Step` or a `Slid`, and
  `Spans.controlFor` already decides this from the value's kind. Reaching for a
  toggle because the key is a `bool` is how a page becomes a wall of toggles.
- **A control that cannot work is never offered.** A plaque beside an empty
  list is armed only when the list has something to act on (`Btn.armed`), and a
  row the active compositor cannot back is dropped, not shown dead.
- **Forms are drafts in the work pane, dialogs are for what cannot be undone.**
  A dialog is a `stone4` slab cut 16 on a scrim with a line plaque to cancel and
  a danger plaque to confirm; it may ask for the thing's name to be typed.
- **Copy is measured, not guessed.** A row's description is one line and 60
  characters at a three-column sheet; a page blurb is 70. A description that only
  restates its label is deleted. These caps are enforced by reading the live
  page, not by counting characters in a file.
- **Nothing casts.** The Hub and the apps are cut stone: an overlay separates
  with `stone4` and a `vein` hairline, never a shadow. The frame's own drop
  shadow over the wallpaper is a different thing and is outside this doc.

## The idioms: shared primitives

They live in `Ryoku.Ui`, and `ryoku/ui/qmldir` is the one list of them. Reuse
them; do not re-roll a bespoke header, control or divider in each surface. That
is how eleven Themes happened.

|Group|Idiom|
|---|---|
|Cut primitives|`CutRect` (a fill or inside stroke with independent corner cuts), `Notch`, `Blade`, `Flake` (the state token, with its `Claim` word)|
|Foundation|`Btn` (a plaque: gold, line, quiet, on, danger, danger line; small; the press chip), `IconBtn` (a square plaque, may carry a notch), `Field` (a 44 `stone3` input with the focus stroke and error state), `Search` (a field with the leading icon and a clear plaque), `ScrollRail` (a flickable's blade thumb)|
|A setting|`Cell` (label, value, unit, struck default, description, control) or `SettingRow` (the compact row), grouped by `SettingCard` (a `stone1` slab) or `Section` (spans come from `Spans`, never by hand)|
|A page body|`CardColumns` (a page's blocks laid into balanced columns, a `fullWidth: true` child taking a band across them)|
|The eight controls|`Sw` (the 40 × 22 cut toggle, bone when on) `Step` `Slid` (a `stone3` track with an `fg2` blade and a cut `sel` handle) `Seg` `Chips` (filter chips, bone when selected) `Multi` `PickBar`+`Picker` `Gallery`. A numeric readout (a stepper's number, a slider's percentage) is typed into on click or Enter: a knob is right for a nudge, typing is right for an exact value, and the typed number is clamped into the row's own range.|
|Lists|`Tablet` (the resource row with its notch, claim and proof, hover action and progress blade), `RowList` (rows 4 apart with the moving bone slab), `GroupHeader`|
|Save state|`ActionBar` (Save in gold, Revert and Reset as line and quiet plaques, and the dirty readout)|
|Live preview|`Preview` (the block a live preview sits in), `SpectrumField` (the audio field, shared with the desktop)|
|Modals and overlays|`Dialog` and `Confirm`, `Menu` in a `Popover`, `Placard` (the bottom notice with Undo that drains over five seconds), `Tooltip`, `AppPicker` (a filterable app or command list), `PickFile` (a file or folder chooser)|
|Navigation|`Tabs` (labels over a vein baseline with the moving blade), `Rail` (the destinations with the travelling bone tile)|
|Data|`EngravedChart` (a line over a hatch), `Facts` (a label and value table), `Empty` (the octagon-framed empty state), `Skeleton`, `Working` (three flakes filling in turn)|
|Image tools|`HeroCrop` (cover plus a draggable 0..1 focal point)|
|Keyboard|`KeyboardMap` (a live diagram lighting the layout legends and remapped keys), `KeypressStack` -> `KeyChord` -> `Keycap` (the recording's keypress overlay)|

Two of those look unused and are not: `Claim` is drawn only by `Flake`'s
callers, and `KeyChord`/`Keycap` only by `KeypressStack`. They are composition,
not museum pieces. The old table here listed `Eyebrow`, `SunDisc`, `RegMark`
and `BrutalPanel`, which really were used zero times, and the poster ornaments
(`Reg`, `Ticks`, `Barcode`, `Marginalia`, `Pixel`, `Watermark`, `Motif`,
`Grain`, `DitherImage`) go with the paper-and-ink look they dressed. A
documented primitive nobody reaches for is not a design system, it is a museum.
If a new idiom is worth having, put it in `Ryoku.Ui` and use it somewhere in the
same change.

Under `ryoku/ui/lib/` sit the maths files, each with a `.test.mjs` beside it,
because they are the arithmetic a component cannot eyeball: `spectrum.js`
(band resampling), `place.js` (the box's tilt and rotation maths, used by
`SpectrumField` so previews and the live look cannot drift), `keypress.js` (the
keypress stack), `hero.js` (image framing, imported by relative path rather
than exported, since only `HeroCrop` needs it) and `cut.js` (the chamfer points
and the inside-stroke inset, shared by `CutRect` and the shaders).

**Choosing a control is not a taste decision.** `Spans.controlFor(kind, options)`
picks it from the value's kind and its option count: `bool` is a `Sw`, `int` a
`Step`, `ratio` a `Slid`, `set` a `Multi`, `visual` a `Gallery`, and an `enum` or
`catalogue` is a `Seg` at four options or fewer, `Chips` at five to eight, and a
`Pick` above that. The bands come from counting the old Settings window: 14 controls have 2
options, 21 have 3, 9 have 4 to 6, one has 7, the font catalogue has 25, and
`islandModules` is a true set. Five or more options is never a segmented; nine or
more is never inline. `Spans.of()` then returns the column span and `Spans.pack()`
bento-fills rows of twelve columns, so a sheet never ends on a ragged right edge.
Subshells and bar styles are a `Gallery`, because no label distinguishes "engraved
bracket cells" from "three islands with concave dips".

## A page is its surfaces

A settings page is not a list of settings. The old Settings window carried 540
settings across 33 pages, and
every page also carries surfaces that are not settings at all: the previews, the
update console, the monitor drag-arrange, keybind capture, the bezier editor,
store cards, scan lists, file pickers, the empty and loading states. The control
histogram over that schema is 150 `sw`, 116 `step`, 73 `seg`, 48 `slid`, 45
`action`, 28 `text`, 22 `pick`, 15 `list`, 11 `readout`, 10 `chips`, 8 `multi`, 8
`color`, and a tail of one-offs (a timezone map, a layout demo, a gallery). Those
45 actions and 15 lists are the tell: a fifth of the old Settings window was not
a value being edited.

So a schema is half a page. Build one in this order:

1. List the page's surfaces before writing anything.
2. Draw them first, through `SettingsPage`'s `bodies`, `slabBodies`,
   `moreBodies` and `footers`, or as the `custom` control of the row they
   belong to. A preview or a console is not a setting and does not become a row.
3. Let the rows flow around them, and give each drawn surface a
   `template: true` row so search still lands on it.
4. Check all four: every surface present, every key present, every write going
   through `Changes` and reaching the desktop, nothing below 4.5:1.

Every edit goes through `Changes` (`ryoku/hub/quickshell/Singletons/Changes.qml`),
the one write path with undo, so a page has no Save step to forget: a row that
writes anywhere else looks fine and then never reaches the desktop.

### The chrome every page inherits

`Hub.qml` owns the frame, so a page only writes its content. The categories
and pages come from `ryoku/hub/quickshell/registry/registry.js`; each page is
`pages/<Name>.qml`, which draws its schema (`schema/<page-id>.js`) through
`components/SettingsPage.qml`, or draws the same head and sections itself when
it is one large surface (Displays, Keyboard shortcuts), and adds its own
surfaces. `docs/hub.md` is the guide to building and extending it.

- **The rail.** The 力 seal as the Home mark at the top, then the categories as
  destinations: Appearance, Desktop, Windows, Devices, Connections, Apps,
  Accounts and System, with Updates at the rail foot. The current category is a
  bone tile that travels to its new place. The only marker a tile may carry is
  an amber notch, and only Updates carries it, when an update is available.
- **The list pane.** On Home it lists every category with a one-line summary
  under the title Settings; in a category it shows the category's name, a
  sentence about it, and its pages. The search field ("Search settings", `/`)
  sits above the list and replaces it with results while there is a query. An
  old page id still lands right, because the registry's `aliases` table maps
  the retired id to its new page.
- **The work pane.** Home (what needs you, your desktop, this computer, and
  recently changed settings with Undo), or the open page.
- **The page head.** The page title and its one-sentence lead in `fg2`, then a
  state line when the page has one, and a note when the window manager hides
  some rows. A page with four or more sections gets a section index, beside the
  column when the page is wide enough and above it when it is not.
- **Sections.** Each section is a titled slab of rows; deep rows fold under
  **More options** at its foot. A page whose schema names related pages ends on
  a Related section linking to them.
- **No action bar.** A change applies when it is made and is kept. The placard
  at the foot of the window names it and offers Undo while it drains, and Home
  keeps the session's recent changes with Undo too.
- **The page measure.** A page is one reading column, capped at
  `settingsExpandedMax` (`settingsMediumMax` at the medium size) and centred in
  the work pane: a wider window buys room for the side index, never a longer
  row and never a wider void.
- **A list of values is tablets, wherever it lives.** The same rhythm as the
  settings slabs: `s4` insets, one row height, 4 between rows, label in `fg`
  over its note in `fg3`, the state as a notch and a claim. A hand-built page
  that draws its own plate with tighter spacing is the drift this rule exists to
  stop (Backup & import's bring-over list, Updates' package and commit rows,
  Displays' saved profiles).
- **Nothing floats.** There is no registration backdrop, no poster layer, no
  ornament. The sheet is stone with slabs on it.

## The surfaces

Each surface is its own directory under `quickshell/`, each component its own
`.qml`. The frame is the chrome the others sit in. Nothing here is
per-application state: a surface reads the shared services in `shell/services/`
and per-monitor visibility from `ShellState`.

### Always on screen

- **frame** the screen border and the popouts that grow out of it; the
  desktop's signature surface. See `docs/frame.md`.
- **bar** the desktop's edge instrument, drawn by the active subshell or bar
  style that `barStyle` names. Ryoku ships two subshells: **Orpheus**, the
  top bar and the fallback, and **Kairos**, the clock island the default rice
  wears. Everything else installs from RyoStore: **QS Bar** is a full-colour top
  bar retinted from the wallpaper's seven palette slots, **Sumi** is the
  monochrome frame-bar system built on Ryoku's frame-bar kit (`docs/bar.md`),
  and **Obi** and **Nacre** have Ryoku Settings editors (Obi's widgets, Nacre's
  layout) on Desktop > Bar that show while they are in use. The `Shells` singleton resolves the id from
  `~/.local/state/ryoku/store/shells.json`; see `docs/subshells.md`. Items built
  on the kit grow the same popout cards from `shell/modules/bar/popouts/`:
  clicking a status widget (network, Bluetooth, battery, audio, system monitor,
  recording, music, voice) grows its live controls out of the bar, and the
  monitor-local menu manager owns those cards and the bounded frame menus. The
  global Quick settings panel lives in
  `ryoku/shell/quickshell/shell/modules/sidebar/`; see `docs/sidebars.md`.
- **dock** a per-monitor shell surface that is independent of the selected
  subshell or bar style. `UniversalDockHost.qml` loads exactly one design:
  Ryoku's stone islands, a dock an installed item provides (Python's flowing
  dock, Shima's notched capsule), a store dock, or none. They all consume
  the same running-window model and canonical desktop-entry pins from the
  top-level `dock` object in `shell.json`, so changing designs keeps app order
  and aliases cannot create duplicate pins. The shared controls are enabled
  state, edge, autohide, icon size, magnification and pin order; each design's
  own look lives below its own key (`dock.python`, `dock.shima`) or the Ryoku
  look keys. An item that owns `dock` turns all of this off while it is active.
  Stage Editor's Dock section is the only settings surface. It carries design
  previews, shared placement and behaviour, each design's details, and pin
  ordering. Older dock pages only open that section. The selected design is
  loaded lazily; unselected designs do not create windows or models.
  Ryoku's design places pinned apps first, then a separator and running apps.
  It can magnify, label and preview windows, and optionally show media. The
  frame-bar kit's `RailDock` remains an in-bar alternative, but reads and writes
  the same shared pins and app identities.

  Docks from RyoStore (the `docks/` catalogue of the ryostore repository, whose
  `docks/AUTHORING.md` is the author's guide) join the same picker. Installing
  one publishes `~/.local/state/ryoku/store/docks.json` with a content-addressed
  view under `dock-views/<id>/<digest>/`; `services/DockProducts.qml` reads that
  index (through the shared `StoreIndex`) and `UniversalDockHost` runs the
  view's `Dock.qml` as the design of that id, waiting for the index so a saved
  store dock does not flash the Ryoku design first. A store dock gets the same
  `screen`, `surfaceVisible` and `stageEditing` the shipped designs get, reads
  the shared model and settings from `Dock`, and opens Ryoku's context menu with
  `Dock.openMenu`. One that fails to load draws the Ryoku design and logs why;
  an update swaps the new view in live. The picker lists every store dock
  beside the shipped ones (an uninstalled one opens RyoStore), offers Update, and
  links to RyoStore. Removing the dock in use moves `dock.design` back to
  `ryoku` in the Store's own transaction, and any dock edit drops that
  removal's recovery marker. The shipped design ids (`Dock.builtinDesigns`) are
  reserved.
- **wallpaper** the background itself, drawn on the bottom layer with its own
  reveal shader for transitions: 22 presets the daemon draws from at random on
  each switch, from the crossfades, directional sweeps and circle irises to a
  coordinate-warping family ported from ii that distorts the two frames rather
  than sweep a mask: block, noise, wave, shatter, glitch, scanline, stripe,
  melt and peel. Its Theme holds exactly one token, the ground colour shown in
  the letterbox margins of a Contain fit.
- **desktop widgets** the clock, calendar, music, all-in-one, system stats,
  weather, notes and any enabled third-party widgets, hosted by one bottom-layer
  surface. Weather reads the shell's own forecast daemon and shows either a
  glance (glyph, temperature, city) or the full card (condition, humidity / wind
  / feels, three days); notes is a scratch pad whose text lives in
  `~/.local/state/ryoku/desktop-notes.txt`, not in a config key, and which holds
  keyboard only while it has focus. Each sits on auto (the wallpaper's calmest,
  most tonally even region, re-followed on every wallpaper change), a compass
  zone, or free pixels. In the Stage Editor, a widget's frame appears on hover
  or selection with four corner grips, a size badge, and Settings and Remove.
  Moves snap to the grid, the card centre, and other widgets' edges and centres;
  Alt keeps a move free. Resize previews stay local, snap to 5% unless Shift is
  held, and commit once on release. Double-clicking a grip resets to 100%.
  Double-clicking the widget or choosing Settings opens its named drawer page
  with quick size, lock, depth, and remove controls above its full inspector.
  The desktop's own right-click menu has Wallpaper and Search in its quick row.
  Its one gold row, Edit desktop, opens the Stage Editor on that monitor;
  Settings and Reload shell remain below it.
- **the desktop spectrum** the audio visualiser, described in full below.

### Summoned

|Surface|Bind|What it is|
|---|---|---|
|**launcher**|`Super+Space`|the app launcher and command palette|
|**overview**|`Super+Tab`|the full-screen workspace expo|
|**Quick settings**|`Super+Escape`|live system activity, connections, levels, and session actions|
|**Kit**|`Super+K`|shortcuts, command references, phone controls, quick tools, and a typing test|
|**ryoshot capture bar**|`Super+Shift+S`|Shot, Edit, OCR, Search, Record, and capture options in one floating bar|
|**Ask**|`Alt+Space`|quick Rashin answers, chat, tools, web search, and the optional desktop bubble|
|**clipboard**|`Super+V`|clipboard history at the bottom edge, with fuzzy search and a starred pane|
|**wallpaper and theme menu**|`Super+W`|the wallpaper carousel and theme picker|
|**Stage Editor**|`Super+Shift+D`|edit the desktop on the focused screen|
|**visualiser placement**|`Super+Alt+M`|open the Stage Editor's Visualizer catalogue|
|**voice**|`Super+grave`|speech to text with a live mic wave|
|**Ryoku Settings**|`Super+,`|the settings app|

A summoned surface is a `stone4` slab with a vein hairline over a scrim where
it is modal, and a cut pane where it is not. It takes focus so Escape reaches
it first, and it leaves over `Tokens.hover`.

### Kit

Kit keeps five tools behind one search field:

- **Keys** lists shared and window-manager shortcuts.
- **Commands** keeps Ryoku, Linux, and Saved libraries separate.
- **Phone** is set up and used in Kit. Its settings live in Ryoku Settings >
  Connections > Phone, and `ryoku-kit phone` opens it directly.
- **Dev tools** holds the quick tools useful during a session.
- **Typing test** provides focused practice and history.

- **launcher** three variants ship, chosen by `LauncherConfig.variant` against
  `launcher/catalog.json`: **Hero** (the default, a full image header with mode
  plaques), **Main** (a compact palette with a centred rest card) and
  **OkShell** (an applications-only list, the fallback). At zero query the rest
  card is the clock, the greeting, the weather, and the hero art; typing
  switches it to results, tablets with the moving bone slab. The mode plaques
  narrow the field to All, Images, Files or Recent, and the current one is
  bone. Twelve providers sit behind one search box, most with a prefix: apps,
  open windows and snippets by default, then `/` actions, `=` the calculator,
  `?` web search with bangs, `>` packages, `@` radio, `/file` `/folder`
  `/image` `/video` the file finder, MPRIS transport, scripts, recent files,
  and `\` which streams an answer from the Rashin agent. See `docs/launcher.md`.
- **overview** the compositor blurs the desktop and a filmstrip shows the
  current desktop's workspaces as scaled mini-desktops with live window
  previews. Drag windows between workspaces or up onto the top desktop strip,
  cycle spaces (scroll or Tab) and desktops (Alt+Tab). A "desktop" is a block of
  ten workspace ids, so each desktop keeps its own 01..10; the same grouping
  drives the desktop-relative `Super+N` binds (`hyprland/modules/workspaces.lua`).
  The gesture legend sits along the bottom margin in `fCaption`, not as buttons.
- **Quick settings** is a compact top-left pane. Vitals is a native mini system
  monitor with per-core load, temperatures, memory, GPU, network and disk as
  blade meters and engraved charts. Connections summarizes Wi-Fi, Bluetooth,
  Ethernet and VPN as tablets with their claim and proof. Levels keeps fine
  volume and brightness sliders with inline per-application audio and
  per-display brightness drawers. Its bottom bar combines hold-to-activate
  session actions, quick toggles, Extensions, and a gear popover.
- **ryoshot capture bar** is the single entry point for screenshots and screen
  recording. Its settings popover owns the 0/1/3/5/10-second delay,
  clipboard/file/both saving, editor handoff, desktop and microphone audio,
  key-press and webcam overlays, the screenshots folder, and the link to Ryoku
  Settings > Apps > Screen capture. Quick settings does not duplicate any of these capture options.
- **Ask** is a Rashin bar for quick streaming answers, full chat, tools and web
  search. `\`, `?` and `/` select Ask, Web and Tools; Tab cycles modes. A pinned
  chat bubble can be dragged, edge-snapped and dismissed, with its position
  stored under `ask.bubble`. See `docs/sidebars.md`.
- **wallpaper and theme menu** a carousel of the wallpaper library (four layouts:
  strips, grid, drift, hearthstone) with the current wall large and named, a
  colour-filter strip, a live tab for animated walls, and a bottom-centre pane
  holding the mode (Walls or Themes), the layout, the filter and the position
  count.
- **ryoshot** screenshot capture, annotation and pinning. Drag a region, click a
  window, or press Space to switch the hover target to a whole monitor; the
  captured region keeps eight grips afterwards, so it can be recropped without
  starting over. Fourteen tools (select, rectangle, ellipse, line, arrow, pen,
  highlighter, step, text, blur, redact, spotlight, zoom, copy text) each take a
  single key, remember their own colour, width and fill across launches, and
  resize live under the scroll wheel. Redact paints a seeded mosaic drawn from
  the region's own dominant colours rather than a downscale, so nothing under it
  can be reconstructed; press its key again for a solid block. Spotlight dims the
  rest of the shot and magnifies its lens. Copy text runs the region through
  `ryoku-cmd-ocr`. `?` lists every key.
- **ryopin** the pinned shot: `Ctrl+P` in ryoshot lifts the finished image onto
  the desktop as a floating always-on-top card that outlives ryoshot. Drag it,
  scroll to resize, hover it for edit, copy, path and close.

### Grown, not summoned

- **notifications** placards in a top corner, left or right by config, 16px off
  it, driven over D-Bus. Each card glides in from off the edge it is anchored to
  and leaves the same way, and its siblings slide down to close the gap. This
  card is the shipped `ryoku` notification design; other installed designs can
  be worn from Ryoku Settings > Desktop > Notifications & pop-ups.
- **OSD** one cut slab, bottom centre, for volume, mic or brightness, raised by
  the media and brightness keys and hidden on a timer; the level is a blade. It
  spans the desktop hole so the compositor clamps it inside the frame, which is
  why it needs no reserve arithmetic of its own. It is the shipped `ryoku` OSD
  design; other installed designs can be worn from Ryoku Settings > Desktop >
  Notifications & pop-ups.
- **capture overlays** the region selector, the camera permission prompt, and the
  keypress overlay a recording draws so a demo shows its own keys.
- **confirm** the session dialog for shutdown, reboot and logout, drawn on the
  monitor that raised it: a `stone4` slab on a scrim, the act in a danger plaque.
- **the keyring prompt** the GNOME keyring password prompt, grown from the bar
  edge as a popout rather than gcr's centred dialog. The `ryoku-shell` daemon acts
  as the keyring system prompter and drives it; `KeyringSurface.qml` renders it.
  The polkit prompt works the same way.

### Its own window

- **Ryoku Settings** the settings app (`ryoku/hub/quickshell/`, run as
  `qs -c hub`). Its rail, list pane and page head set the pattern every page
  follows; see "The chrome every page inherits" above.
- **the apps** standalone Ryoku apps read the same tokens and carry no theme of
  their own: **ryostore** is the catalogue for rices, lockscreens, plugins, bar
  styles, fastfetch presets and bundles; **Ryoport** is the optional fleet
  dashboard package for virtual machines, SSH servers and remote consoles, and
  Lithic's reference implementation. See `docs/store.md`.
- **welcome** the first-run guided tour, shown once on the first login: a
  floating window (`qs -c welcome`) that walks a new user through the core
  keybinds, names each surface and how to summon it, and offers a few live quick
  settings (wallpaper, bar style, frame and window roundness). The Hyprland
  autostart launches it once, gated on a `~/.local/state/ryoku/welcome-seen`
  flag.
- **the lockscreen** `Super+L` runs `ryoku-shell lock`, which renders the active
  qylock theme (`ryoku/lockscreen/qylock/`, vendored, and exempt from our hooks
  for that reason) over a Wayland session-lock surface. The SDDM greeter uses the
  same theme family before the shell exists. Skins are a card gallery on
  Ryoku Settings > Appearance > Lock screen and install from Ryostore; they are the one part of the
  desktop that deliberately does not read `Tokens`, because a lock skin is a
  whole look, not a surface inside ours.
- **third-party widgets and cards** a plugin ships `manifest.json`, a
  `service/Main.qml` and a `content/Widget.qml`, then picks one of four hosts: a
  desktop widget, a frame popout, a bar glyph, or a sidebar card.
  `Ryoku.PluginKit` gives it the shared primitives, and the kit's Theme reads
  `Scheme`, so a plugin follows the active theme without knowing anything about
  it. See `docs/plugins.md`.

## The desktop spectrum

The spectrum is a wallpaper surface, not a widget. It draws on a click-through
layer-shell field, one per monitor, that sits on `WlrLayer.Bottom` under everything
(or `WlrLayer.Top` for the overlay mode that floats it over windows), and it never
takes input, so the desktop behind it stays live. Placement mode is the exception,
and it runs on a surface of its own. It reads the audio level bands the shell
already computes and paints them; nothing about it competes with app content.

Every look is one analytic GPU pass. The bands, the palette and the geometry go
in as uniforms and `ryoku/ui/shaders/spectrum.frag` returns the whole shape in a
single draw call, with glow, reflection and peak caps done inside that pass
rather than as offscreen effects. Both the desktop and the Hub preview render
through the same `Ryoku.Ui.SpectrumField`, so the preview is the exact geometry
the wallpaper draws and the two cannot drift. The spectrum is art on the
wallpaper, not chrome, which is why it may glow and curve where a surface may
not.

There are twelve looks: eight that grow from an edge of their box, three that
are polar, and one that is not a box at all. `aura` is an edge field, drawn by
`ryoku/ui/AuraField` and `shaders/aura.frag` instead of the spectrum shader:
twelve eased sectors (folded and followed in `lib/aura.js`) flow along chosen
screen edges as one organic current, a rail, an open frame or a joined
perimeter. It owns four small vocabularies, material (silk, aurora, contour,
liquid), movement (flow, ribbon, cells, filament), effect (clean, shimmer,
echo, prism, bloom, caustic, afterglow) and colour mode (flow, spectrum, pulse,
static), plus reach, span, taper and corner blending, and it paints a
wallpaper-lit triad unless all three stops are pinned. Its motion twin is
`AuraMotion`, which runs the same adaptive idle/halve rules as `Motion`.

|Look|Kind|What it is|
|---|---|---|
|`bars`|edge|upright bands with rounded caps and a ramp gradient along their length|
|`split`|edge|bars mirrored about the axis, the classic centre-out look|
|`dots`|edge|discs sized by level with a soft edge and a faint trail to the baseline|
|`segments`|edge|quantised cells with a per-cell brightness ramp|
|`wave`|edge|a Catmull-Rom filled area under a lit top edge|
|`ribbon`|edge|three phase-offset translucent waves, an aurora|
|`curtain`|edge|a short wave hanging off the bar's edge, sealed to it by a lit hairline|
|`line`|edge|an oscilloscope trace with a bright core, a wide halo and windowed edges|
|`radial`|polar|rounded polar bars and a per-angle ramp around a bass-pulsed inner ring|
|`orb`|polar|a glass sphere: a barely-there body, a wobbling lit rim and ripples inside|
|`spiral`|polar|bands laid along an Archimedean spiral over one and a half turns|
|`aura`|field|a living current of light flowing along the screen's edges, twelve sectors deep|

Every look but `aura` lives in a box, and the box goes anywhere. `x` and `y` place its top
left corner as fractions of the screen, `w` and `h` size it, `grow` says which of
its edges the bands rise from (up, down, center, left or right), and `angle` turns
the whole thing about its centre. A polar look centres in the box and takes its
radius from the shorter side, so a square box shows the whole ring; `spin` turns it
slowly. That one rectangle replaced an
anchored set of `position`, `span`, `align`, `height`, `originX`, `originY` and
`size`: a look was pinned to a screen edge and could not simply sit where its
owner wanted it. A config written before the box folds into one on first read, and
a stored `circle` style aliases to `orb`, so nothing moves or blanks on update.

`angle` spins the look in the plane of the screen. `tiltX` and `tiltY` are the other
kind of turn: the box pivots about its own horizontal or vertical axis so one edge
goes away from the viewer and the other comes forward, which needs a perspective
divide or the lean is only a squash and reads as nothing. Both are bounded to 35
degrees, well short of edge-on, because a look flat to the viewer is a look you
cannot see; the viewer distance scales with the box, so the same degrees read the
same at any size. The lean is taken in the box's own frame and the spin applied after it, as one matrix
(`ryoku/ui/lib/place.js`, tested in `place.test.mjs`): Qt composes an item's
`transform` list outside its `rotation`, so setting both gives the reverse and the
lean pivots about the screen's axes, which shears the bands instead of turning a
trapezoid. One draw either way, and the shader keeps working in the item's own pixels
while the scene graph interpolates its coordinates with perspective, which is what
makes the near bands come out wider.

A lean is a shape change *inside* the box, not an escape from it. Left raw, the
perspective pushed the near edge past the outline and pulled the far edge short of
it, so the look both spilled over one side and left dead space at the other, and the
box stopped meaning what it said. The leaned quad is therefore fitted back onto the
box: its corners are projected, and an affine composed after the projective matrix
(which acts on the divided point, so it is exactly "scale the picture just computed")
maps that bound onto the box. The quad then touches all four sides at every lean and
crosses none of them, which `place.test.mjs` pins as an invariant, and the placement
guides stay honest without the gestures having to invert a projection.

Each visualizer instance is placed by hand rather than by numbers.
`Super+Alt+M` or
`ryoku-shell ipc call desktop editSection visualizer` opens the Stage Editor's
Visualizer catalogue with an instance selected. Every instance has its own
frame and grip (`visualizer:<index>`); selecting one retargets the catalogue.
Dragging moves it, a corner sizes it, and the top handle turns it. The preview
stays local during a resize, then one write and one undo step land on release.
Remove deletes only that instance. The visualizer turns off when the last
instance is removed, and undo restores a removed instance at its index with all
of its settings.

The catalogue leads with a gallery of every look, followed by Look, Place,
Colour, Motion, Shape, and, where useful, Field chips. Place offers width,
height, across, down, and turn, with Centre, Square, Full width, Top, Middle,
and Bottom actions. Done leaves the editor; Escape first closes the open panel
and clears the selection.

The Stage island maps its stones, type, spacing and cuts through
`modules/common/Appearance.qml` to Ryoku.Ui tokens. A look is tuned where it is
visible, on the live wallpaper, rather than in a detached placement surface.

The catalogue uses the desktop menu's rows (`MenuRow`, `MenuSlider`,
`MenuChip`, `MenuInkPicker`) and the shared `Gallery` painter (`VizStyles`), so
what a look looks like is drawn from one catalogue. A knob that means nothing
for the look in hand hides rather than dims; `Config` owns those rules
(`peaksApply`, `mirrorApplies`) and the renderer reads the same answer. Every
committed edit settles through the same coalescer: the file is watched, so a
write returns as a reload, and the render reads the adapter rather than the
file.

Every gesture applies the pointer's delta from where it was pressed rather than its
absolute position, so nothing jumps out from under the cursor, and the box eases
toward where the pointer asks instead of snapping to it, so an unsteady hand still
lands a clean size. Easing keeps running after the release until it arrives, or
letting go mid-drag would strand the box short. A turn magnetises to every 15
degrees within a couple of degrees of one, which lands square and diagonal without
care and leaves everything between free, and it ignores the pointer within a handle
and a half of the centre, where a pixel of travel is a wild swing.

Sizing a turned box is the subtle one, and `ryoku/ui/lib/place.js` owns the maths
with `place.test.mjs` beside it. The box turns about its centre, so growing it moves
that centre, and simply growing width by the drag walks the grabbed corner away from
the pointer along an axis that has nothing to do with the angle: square on it
tracked, at a quarter turn it moved half as far diagonally, and at a half turn it
did not move at all. Instead the delta is read in the box's own axes, the corner
opposite the grip is held still on screen, and the centre is re-derived from the new
size, which makes the grip land exactly under the pointer at every angle. The test
pins that as an invariant. The guides ride the same turn as the look, so a handle is
always on the corner it appears to be on.

Flip mirrors what is on screen, which is a different thing per family: a look that
grows from an edge swaps to the opposite edge, a centred one reverses the band order
it is symmetric about, and a polar one reverses the way it turns. `angle` turns the
drawn pass about the box centre rather than rotating the geometry, so a turn costs
one transform instead of re-deriving every band, its reflection and its bloom; the
box itself stays axis-aligned, and the wallpaper tone is read from the turned
region the look actually covers rather than from the box.

Placement runs inside the lifted desktop surface while the Stage Editor frames
the monitor: the spectrum's own surface is click-through for life and a masked
surface does not start taking a pointer again just because the region is
swapped, so the grip rides the desktop instead, which already takes presses
while composing. Aiming a look that is off turns it on first, since aiming
nothing places nothing; outside the editor the overlay mode still raises the
spectrum's own surface over windows.

Whether the spectrum runs at all is the persisted `enabled` key, so the
keybind, the catalogue, and the next restart all read one answer; only the
layer, desktop, or overlay is per-monitor memory. It was in-memory state before,
which meant a restart started at off with `enabled` still true.

The `curtain` is the one look that reads the rest of the shell: its surface
honours exclusive zones instead of ignoring them, so it starts where the bar ends
and hangs from that edge whatever the bar's height, position or reveal state. No
bar geometry is plumbed through for it.

Colour is the palette, not a second one. The accent (`primary`) is used exactly
as the daemon published it, so the spectrum wears the colour the bar and the Hub
wear; it is only re-lit through Ink when the wallpaper behind sits too close in
tone to separate. Eight stops walk a narrow band either side of that colour, bass
deeper than treble, and the shader interpolates between them. A loud band's tip
takes a gentle highlight, which is what reads as lit.

Motion is budgeted. A single `Timer` at the configured `fps` drives the field,
halves its rate while the audio only idles, and stops on silence unless
`idleWave` keeps a slow resting motion; an adaptive tier steps the rate down
further when it sees sustained frame overrun, so a loaded machine sheds work
rather than dropping frames. The bloom is computed inside the shader pass and
sized to the band width, which retired the old full-screen blur and its
offscreen buffer.

## Motion

Knapping: one curve, a short table of durations, no bounce, nothing moving at
rest. The curve is **chisel**, `cubic-bezier(.2, 0, 0, 1)`, handed to
`easing.bezierCurve` as `[0.2, 0, 0, 1, 1, 1]` beside `Easing.BezierSpline`. It
is the only curve for chrome; the frame's physical give and the spectrum keep
their own (below).

- **`Tokens` carries the app set**, and a name says what the motion means:
  `press` (90 ms, the cut chips 4 deeper and the plaque sinks 1), `hover`
  (160 ms in, 100 out: a plane steps up a tone, a tablet's claim gives way to
  its action), `select` (240 ms: the bone slab moves, the tab blade slides, the
  toggle knob travels), `field` (320 ms: a mineral field changes stone), `notch`
  (200 ms: a state is cut into a corner, a disclosure opens, a page fades in),
  `overlay` (300 ms in, 160 out: a dialog, placard or tooltip), `break` (420 ms:
  a deleted slab breaks along a diagonal), with `popover` 160 in and 100 out and
  the `placard` life of 5 s. The Hub and the apps use nothing else, and a
  motion not in that list does not happen: icons do not spin, panes do not slide
  in from the side, lists do not stagger, nothing pulses.
- **`Motion` carries the shell set** (`shell/services/Motion.qml`), because a
  surface that grows out of an edge is a different problem from a row changing
  value. Its named tokens, `barReveal`, `menuSlide`, `sidebarEnter` and
  `sidebarExit`, `push`, `notifIn` and `notifOut`, `crossfade` and
  `wallpaperFade`, `rowReveal`, `thumbHover`, and the general `fast`, `hover`,
  `standard`, `morph` and `effects`, ride the chisel curve; `spatial` keeps its
  spring for the frame's popouts, which are the one place overshoot is physical
  rather than scripted. Reach for a named token rather than inventing a value.
- **The Quick settings surface moves in small, local steps.** `SidebarState` scales
  `Tokens.overlay` for opening and `Tokens.hover` for closing; `quick`,
  `standard`, and `calm` apply multipliers of 0.6, 1, and 1.5. The pane fades in
  and settles a short distance from its screen edge. Detail pages crossfade;
  selected rows use the bone slab. Both `Motion.reduce` and
  `Tokens.reduceMotion` remove every surface and chrome animation.
- **Every shell token is already scaled.** Each one is defined as `dur(ms)`,
  which multiplies by `Tokens.motionScale` (the user's Speed, `theme.motion.scale`
  in `shell.json`) and collapses to zero under reduce-motion, so the whole
  shell speeds up or stops in one place. Read the token; never write a literal,
  and never wrap a token in `dur()` again or you scale it twice. `Motion.dur(ms)`
  is for a duration that has no token yet, and the honest fix is usually to add
  the token.
- **The frame's give is physical, not scripted.** `BlobRect` is a C++ item
  (`shell/plugin/blobrect.cpp`): it reads its own velocity, targets a symmetric
  2x2 deformation of `min(speed * deformScale, 0.35)`, and solves an underdamped
  spring toward it in sub-8ms steps. The defaults are `stiffness` 200, `damping`
  16, `deformScale` 0.0005, and it snaps to rest below 0.004 deviation. That is
  the feel when a popout grows. See `docs/frame.md`.
- **Drive transitions from state where the shape of the change is a state**
  (`states` + `transitions`, as the popout reveal and the notification list do).
  Most motion is a single property settling, and a `Behavior on` is the honest
  expression of that; the codebase is overwhelmingly `Behavior on` and that is
  correct. What is never correct is an imperative timer stepping a value.
- **Nothing is drawn at rest.** A `Behavior` costs nothing once it settles, and
  that is the point; the two loops Lithic allows (the indeterminate blade and
  the three-flake `Working` indicator) run only while their job is live and
  freeze under reduce-motion. Gate live work (a `MultiEffect`, a poll, a
  scanner) on the surface being open or visible; a hidden or resting surface
  costs nothing, and idle blobs snap to rest. With the pointer still and no job
  running, the process's frame counter must not advance.

### Building or replicating an animation

1. Read the closest existing component first; `Btn`, `Tablet`, `Tabs` and the
   service surfaces under `quickshell/shell/` show the project's durations,
   curve, and structure. Reuse the tokens.
2. Break the target motion into property transitions (size, position, opacity,
   the cut) and reproduce each with a `Behavior` on a token. If the frame itself
   should give, let a `BlobRect` carry it rather than animating geometry by hand.
3. Prototype live: run the shell from the checkout with `ryoku/shell/dev-run.sh`
   (it launches via `qs -p` with hot-reload), so QML edits show as you save. Tune
   timing against the running surface.
4. Keep it in its own component file. Wire any state it needs through
   `ryoku-shell`, not ad hoc logic in the view.

## Copy

Sentence case, plain verbs, the same word through a whole flow: a plaque that
says Start produces "Starting" and then "Running". A state always comes with
its proof ("Online, 23 ms"; "Needs you, sign-in failed 2 min ago"). Errors say
what happened and what to do. Empty states offer the next step ("No machines
yet. Make one from the catalogue or import an ISO"). Numbers under ten are
words in sentences and digits in proofs. No exclamation marks, no emoji in
chrome, no all caps. The claim words by mineral: jade is Online, Running,
Ready; lapis is Starting, Creating 62%, Downloading; amber is Needs you,
Paused; jasper is Unreachable, Failed; idle is Stopped, Off.

## Art

Every decorative image in the desktop is baked at dev time and committed, because
the running target has no generation dependency. The full contract, the tools and
how each shipped specimen was made is `bin/art/README.md`. In short:

- **One ink.** Every decor is reduced to one ink on a transparent ground, so it
  composites on any stone and the whole set reads as one set; the ink is tinted
  to `fg` at draw time, so art follows the palette like everything else.
- **Two treatments, chosen by what the source is, not by taste.** `ryodither`
  lays an ordered Bayer stipple, which is right for photographs, sculpture and
  motion loops, where the eye reads tone through grain; it is the set's default.
  `ryoduo` maps tone straight onto the ink through the alpha with no stipple,
  which is right for fine line art (a patent drawing, an engraving, a blueprint)
  that a dither would break into noise. If a dithered bake looks noisy, bake it
  smooth; if a smooth bake looks flat, dither it. Engraving is the Lithic
  treatment; where both would do, prefer it.
- **The loops are drawn, not filmed.** `ryowave`, `ryorender`, `ryobounce`,
  `ryocompass` and `ryoneedle` each draw a deterministic grayscale source (the
  dictation wave, a turning 3D object, a bouncing ball on gravity easing, the
  Rashin compass, the Needle poster) which is then baked by one of the two
  treatments.
- **One home.** Both bakers write into `ryoku/assets/ryodecors`, so a new decor
  ships everywhere at once: the installer seeds it, the `ryoku-desktop` package
  carries it to `/usr/share/ryoku/ryodecors`, and `ryoku doctor` lays it into
  `ryodecors/` under each user's configured Pictures folder. The profile hero
  and its editor read it by bare filename.

Figurative art that is not decor (the launcher hero, the welcome backdrop, the
profile portrait) follows the same rule: generated at dev time, background
flood-filled to the canvas colour so it blends, and committed as a static
asset. Art is the one place a curve, a glow or a gradient may appear; it is
content, and the chrome around it stays cut. One desktop constraint:
Quickshell's Qt build has no webp plugin, so shell and Hub art ships as **PNG**,
not webp.

## From paper and ink to Lithic

Ryoku v1 was paper and ink: bone type on pure black, Fraunces display, kanji
glosses beside every label, tracked uppercase mono labels, a 6 px radius,
inversion as the only emphasis, a fixed red for alerts, and a poster ornament
layer. Lithic keeps what was right about it (flat planes, hairline depth, the
palette chain, measured copy, disclosure over walls) and changes the rest:

|Was|Is|
|---|---|
|`paper`, `paperLift`, `ink`, `inkDim`, `inkMuted`, `inkFaint`, `bone`, `inkOnBone`, `sun`, `alert`|`ground`, `stone1..4`, `vein`, `fg`, `fg2`, `fg3`, `sel`, `onSel`, `onSel2`, `gold`, `onGold`, `goldInk`, `jasper`, `jade`, `lapis`, `amber`, `scrim`|
|emphasis by inversion only, no accent in content|bone for place, gold for the one consequential action, minerals with words for state|
|`alert` fixed red|`jasper` from the palette's `error`, `amber` fixed for needs-you, the amber notch as the one badge|
|Fraunces display, `fTitle` 32, `fHero` 34, `fValue` 26|Geologica Bold at `fName` 52, `fSentence` 34, `fSection` 21|
|tracked uppercase mono labels, kanji glosses on nav and eyebrows|sentence case Geologica; the 力 seal stays as the brand mark only|
|`radius` 6, round dots, round knobs|the cut scale 3 / 4 / 6 / 7 / 12 / 16, flakes, a cut knob|
|spacing 4 8 12 16 24 32 48|4 8 13 21 34 55|
|`rowH` 48, `ctlH` 26, `railW` 268|`rowH` 56 (48 compact), `ctlH` 40 (32 small), `fieldH` 44, `railW` 88 / 72|
|`snap` 90, `move` 170, `swap` 210, `flap` 110, `OutCubic`|`press` 90, `hover` 160, `select` 240, `field` 320, `notch` 200, `overlay` 300, `break` 420, one chisel curve|
|`Reg`, `Ticks`, `Barcode`, `Marginalia`, `Watermark`, `Grain` over chrome|gone; stone with slabs on it|
|a `//` lead on the active tab, bone plate tabs|a blade under the active tab|
|status dots|a notch on a tablet, a flake beside a word|

Two things are deliberately not decided here. The frame keeps its rounded
liquid border and `docs/frame.md` keeps describing it; whether a v2 frame cuts
its corners is a frame decision, not a chrome one. And the spectrum stays art
on the wallpaper with its glows and curves, because it is content, not chrome.

Until a surface has moved, it still wears the old look and `Tokens` still
exposes the old names beside the new ones; a surface is Lithic when it reads
only the roles in this doc and passes the checklist in
`docs/lithic/checklist.md` in a dark and a light palette. Move a surface whole,
in one change, and remove the old names from `Tokens` when the last reader is
gone.

## Research

When a control, protocol, or animation is unfamiliar, look it up against primary
sources and confirm on the running system:

- The Quickshell documentation and example configurations for QML widgets, the
  IPC surface, and layer-shell behavior.
- The Hyprland wiki for compositor behavior, dispatchers, and protocols
  (idle-inhibit, layer rules, window rules).
- The Qt/QML documentation for animation, layouts, and bindings.
- The Arch Wiki and each tool's own docs for system-level pieces.

Prefer official sources, cross-check a second one for anything load-bearing, and
verify the result live with the dev loop rather than assuming it renders. Then
look at it: `grim -o <output> shot.png` against the running shell is the only
proof that a surface renders the way the tokens say it does.
