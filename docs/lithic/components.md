# Components

Each part below is described as Ryoku Settings (the Hub) uses it: what it is
for, where the Hub shows it, its exact geometry, and the Lithic type or Hub
component that provides it. Window composition is [layout.md](layout.md),
type and the pattern names are [design.md](design.md), roles are
[palette.md](palette.md), durations and curves are [motion.md](motion.md),
and [map.md](map.md) indexes every part by stack.

Sizes are logical pixels. Hub sources are cited relative to
`ryoku/hub/quickshell/`, library sources as `Lithic/<File>.qml`, relative to
`docs/lithic/library/qml/`. Every `Library:` line names the part in each
stack; `-` means that stack has none.

## Recipes

Four recipes hold for every part.

| Recipe | Lithic parts | Hub-drawn parts (tree lines, outline lines, IconButtons) |
|---|---|---|
| Hover | a filled plaque mixes its fill toward white by 0.10; a tonal plaque mixes `field` toward `fg` by 0.06; a quiet or line plaque fills its cut with its ink at 0.10; a row plane is `fg` at 0.06. Fades in over 160 ms, out over 100 ms (`Lithic/PlaqueRecipe.qml:22-28`; `Lithic/SettingRow.qml:46`) | `Tone.hover`, `fg` at 0.06 in dark and 0.07 in light, in over 120 ms and out over 240 ms (`Singletons/Tone.qml:26`; `components/nav/NavRow.qml:62-74`) |
| Press | a plaque deepens its cut by 4 and sinks its content 1 px over 90 ms; a row adds `fg` at 0.08 (`Lithic/Plaque.qml:63-93`) | `Tone.press`, `fg` at 0.10 in dark and 0.12 in light; the glyph scales to 0.86 (tree) or 0.85 (IconButton) |
| Focus | a 2 px `fg2` ring 2 outside the cut, the cut grown by 2, keyboard focus only (`Lithic/FocusRing.qml`) | a 1 px `fg2` stroke on the part's own cut, keyboard focus only (`NavRow.qml:75-82`; `components/IconButton.qml:26-27`) |
| Disabled | the whole part at 0.5 alpha, no hover (`LithicTokens.alpha.disabled`) | the sidebar's "close every folder" button at 0.35 (`components/nav/HubSidebar.qml:64`) |

Two shape rules hold for every part:

- **Every plane is a cut.** Each plane is a `CutRect`, so its corners follow
  the style: cut in Lithic, rounded in the others (`Lithic/CutRect.qml`;
  [styles.md](styles.md)).
- **Nested cuts are concentric.** A cut inside another at an inset takes
  `inner = outer - inset * (2 - sqrt(2))`, rounded.

## Foundation

### Icon

Material Symbols Sharp everywhere, in the ink of the part that holds it;
an icon never carries a state alone, and every icon-only action has an
accessible name. The Hub sets 14 to 20 px glyphs: 16 in the peek and tree
chevrons, 17 and 18 for tree marks, 18 in IconButtons
(`HubSidebar.qml:231-236`; `NavRow.qml:104-137`; `IconButton.qml:31-33`).

Library: QML `Glyph`, web `.l-glyph`, Go `kit.Icon`, terminal -

### CutRect

A filled or stroked rectangle with an independent 45° cut on each corner.
The stroke is drawn inside, its inner cut `cut - width * (2 - sqrt(2))`, so
it stays even around the chamfer. In the round styles the same part draws a
rounded rectangle whose radius `Lithic.cornerRadius(cut, width, height)`
derives from the cut; a `DesignScope` makes every CutRect inside it draw
another style, which is how the Style page previews a style not in use
(`components/style/StylePreview.qml`).

The cuts the Hub uses: page pane 16 top and 7 bottom, section slab 7, rows,
search hits and IconButtons 6, plaques 7 (6 when small), placard 8, popover
and menu 8, dialog 16, toggle track 4 and knob 2, segment 5 in a track of 7
(`LithicTokens.cut`; `Singletons/Tone.qml:35-37`).

Library: QML `CutRect`, web `.l-cut`, Go `kit.FillCut` and `kit.StrokeCut`, terminal -

### FocusRing

A 2 px `fg2` stroke 2 outside the part, the cut grown by 2, in over 120 ms
and out over 100 ms, shown for keyboard focus only. Setting rows, plaques,
fields and page rows carry it (`Lithic/SettingRow.qml:148-155`;
`components/PageRow.qml:36-42`).

Library: QML `FocusRing`, web `.l-focus-ring`, Go `kit.FocusRing`, terminal -

### Hatch

1 px `fg` lines at 45°, 6 apart, at 0.30 alpha: an engraved field, not a
background texture. The Hub lays it under engraved charts, behind the
monitor arrangement on Displays and in bar style details
(`components/displays/ArrangeCanvas.qml`; `components/bar/StyleDetailDialog.qml`).

Library: QML `Hatch`, web `.l-hatch`, Go `kit.Hatch`, terminal -

### Flake

A parallelogram 1.4 by 0.7 of its nominal size (9 gives about 13 × 6), 45°
short sides. Idle is a 1.5 px `vein` outline; a state fills it with its
mineral; becoming fills `lapis` to the fraction. The Hub shows flakes inside
`Claim` and `Working`, and alone in the update run view
(`components/updates/RunView.qml`).

Library: QML `Flake`, web `.l-flake`, Go `kit.Flake`, terminal `term.RenderFlake`

### IconButton

The Hub's own bare glyph button (`components/IconButton.qml`): 34 × 34, an 18
glyph in `fg2` (`fg` under the pointer), no plane at rest, a cut-6
`Tone.hover` wash under the pointer and `Tone.press` when held. The `close`
glyph turns 90° under the pointer over 240 ms. It is the window's close
button, the drawer's `menu` button, the sidebar's "close every folder" (at
30 × 30) and the info button beside a row (`Hub.qml:984-1001`;
`HubSidebar.qml:55-66`; `components/about/CardOnlineSwitch.qml:32-38`).

Hub: `components/IconButton.qml`

## Plaques

### Plaque tiers

A plaque is a cut button (`Lithic/Plaque.qml`); its tier is a fill recipe
(`Lithic/PlaqueRecipe.qml`). The Hub keeps almost every action quiet and
spends gold on the one action that matters in a view.

| Tier | Fill | Ink | Hairline | The Hub uses it for |
|---|---|---|---|---|
| `quiet` | none | `fg2` | none | almost everything: Reset page, Retry, Undo, Continue in, Binder, Share, Customise, Back, Remove, "Open" on a row (`components/RowControl.qml:353-363`) |
| `line` | none | `goldInk` | `goldInk` | the secondary action beside a gold one: Save picture, Open a card, View certificate, the later items under Needs you |
| `gold` | `gold` | `onGold` | none | the one consequential action: Open Updates on Needs you, Apply on Displays, Copy on Share, Paste a card, Done in the card studio, Activate (`components/Home.qml:818-824, 838-841`) |
| `tonal` | `field` | `fg` | none | Related links at a page's foot, Save and Apply in the Displays lists (`components/SettingsPage.qml:888-895`) |
| `danger` | `jasper` | its own ink on `jasper` | none | the action of a destructive confirm (`Lithic/Confirm.qml`) |
| `dangerLine` | none | `jasper` | `jasper` | an outlined destructive action |
| `on` | `sel` | `onSel` | none | only the placard's Undo (`Lithic/Placard.qml:110-125`) |

A quiet plaque draws no edge and no fill at rest, so its words line up with
the column they stand in (`Home.qml:993-996`).

### Plaque geometry

| Variant | Height | Inset | Icon | Gap | Cut |
|---|---|---|---|---|---|
| normal | 40 | 18 | 19 | 8 | 7 |
| small | 32 | 13 | 17 | 6 | 6 |

The label is the `label` role. Nearly every plaque in the Hub is small. A
press deepens the cut by 4 and sinks the content 1 px over 90 ms. A busy
plaque swallows presses and may show Working. Navigation never takes gold.

Library: QML `Plaque`, web `.l-plaque`, Go `kit.Plaque`, terminal `term.RenderPlaque`

### IconPlaque

A square plaque with no label: 40 with a 20 icon, or 32 with an 18 icon when
small, cut 6, any tier. The Hub uses the small quiet one as a setting row's
reset plaque and in list editors (`Lithic/SettingRow.qml:208-225`).

Library: QML `IconPlaque`, web `.l-icon-plaque`, Go `kit.IconPlaque`, terminal -

## Rows and controls

### Setting row

One setting on a section's slab: the Lithic `SettingRow`, wrapped by
`SettingsPage.RowItem`, with its control picked by `RowControl`
(`Lithic/SettingRow.qml`; `components/SettingsPage.qml`;
`components/RowControl.qml`). Render: [a page](media/hub-page-dark.png).

| Part | Geometry | Source |
|---|---|---|
| title | `row` in `fg`; wraps, never elides | `SettingRow.qml:186-193` |
| detail | `detail` in `fg2`, 2 below the title slot; wraps; a changed stacked row reserves the 32 px reset height in its title slot | `SettingRow.qml:179-215` |
| height | at least 44; the taller of text and control side by side, or their sum plus 13 when stacked | `SettingRow.qml` |
| between rows | 13, with a 1 px `vein` hairline centred in the gap; the first row has none | `SettingRow.qml:24, 131-139` |
| control | short controls stay right-aligned, vertically centred, at most 320 wide (`settingsControlMax`), 21 from the text; taller single controls opt into top alignment; groups and lists sit below the text at full row width | `SettingRow.qml`; `RowControl.qml` |
| Changed tag | "Changed" in `detail` `fg3`, 13 after the title on its baseline, or on its own line when it does not fit | `SettingRow.qml:178-205` |
| reset plaque | a small quiet `restart_alt` IconPlaque 8 before a side-by-side control, or at a stacked title's far end; its slot is kept while changed, and it shows while the row is hovered or holds focus | `SettingRow.qml:42-44, 218-235` |
| hover plane | on stacked groups and whole-row toggle, hand-off, action and link rows: bleeds 8 to each side and 4 above and below, cut 6, `fg` at 0.06 | `SettingRow.qml:141-155` |
| unavailable | the control disabled and the reason in `fg3` in place of the detail | `SettingRow.qml:207-215` |

**Stacked rows** put choice groups, ordered picks, lists and sample editors
below their full-width title and detail, left-aligned, with `s3` (13) between
text and control. The Changed tag stays beside the title and the reset plaque
sits at the title line's far end; its reserved slot never narrows the control.
The hover plane, keyboard ring and search spotlight enclose the entire row.
The host opts in with QML `stacked`, web `data-l-stacked` or Go `Stacked`;
Settings accepts schema `stacked: true` or a custom control's `wide: true`.
A taller single field or picker with helper actions keeps the two columns
but opts into `topAligned`, `data-l-top-aligned` or `TopAligned`, aligning
the title with the control's top. These variants are explicit, never chosen
by measuring their own resulting layout.

Inside a stacked group, single controls keep their own measure rather than
stretching with the host. Pickers cap at `settingsControlMax` (320); entry
fields cap at `settingsFieldMax` (420), with their action beside them when
there is room. Writing samples cap at `settingsTextMeasureMax` (720).
Toggle lists flow across then down in cells between `settingsChoiceMin`
(240) and `settingsControlMax`, separated by `s3` (13); keyboard order
matches that reading order. Word and app lists use compact rows or chips,
not full-width bars.

**A search landing** lights the row at the hover tone and fades it over
1600 ms on the chisel curve, once; with reduced motion it holds for the
placard's life (`SettingRow.qml:77-120`; `SettingsPage.qml:659-663`).

**A write that failed** shows its error as the detail in `jasper` and a quiet
Retry plaque under the row (`SettingsPage.qml:968-990`). **A window manager
that cannot keep a value** gives its reason in place of the detail
(`SettingsPage.qml:946-950`).

Library: QML `SettingRow`, web `.l-setting-row`, Go `kit.SettingRow`, terminal -

### Toggle

For a true binary; the whole row toggles. A 40 × 22 track cut 4 with a 14
knob cut 2 at a 4 inset. Off is a `stone4` track with a 1 px `vein` hairline
and an `fg2` knob; on is `sel` with an `onSel` knob. The knob glides over
240 ms and the tone changes over 320 ms. Space toggles it
(`Lithic/Toggle.qml`; `RowControl.qml:153-167`).

Library: QML `Toggle`, web `.l-toggle`, Go `kit.Toggle`, terminal `term.RenderToggle`

### Segmented

For two to five options worth seeing at once, and the Hub's tabs: the card
studio's Picture, Adjust, Words, Layout, Share and Badge, and its preview's
"The card" and "Home" (`components/about/CardEditor.qml:150-167, 291-303`).

- **Track:** `field` with a 1 px `vein` hairline, cut 7, 36 tall.
- **Segments:** equal width, `label` text; the natural width is the widest
  label plus 21 each side, times the count. In a row it is at most 320
  (`RowControl.qml:169-185`).
- **Selected:** inset 4, cut 5, a `sel` fill and `onSel` label; it glides to
  a new segment over 240 ms. Other labels are `fg2`.
- **Keys:** the arrows move the selection, Home and End jump to the ends.

Library: QML `Segmented`, web `.l-segmented`, Go `kit.Segmented`, terminal -

### Slider

For a bounded quantity set by feel. In a row the Hub sets it 200 wide, with
its value 13 to its right in a 72-wide right-aligned `label` `fg2` with
tabular figures (`RowControl.qml:232-262`). The slider is 32 tall, its
`field` track 6, the completed part an `fg2` blade, the `sel` handle 8 × 20.
Arrow keys step it, Home and End jump to its ends.

Library: QML `Slider`, web `.l-slider`, Go `kit.Slider`, terminal -

### Stepper

For an exact number. Two small quiet IconPlaques, `remove` and `add`, 8
either side of the value; the value in a 32-tall field cut 6 with the `field`
fill and `edge` hairline; the unit 4 after in `fg3`. In a row it is editable:
the field fits the widest bound, Return or leaving it commits the typed value,
clamped and snapped, and Escape drops an unsaved edit (`RowControl.qml:264-282`;
`Lithic/Stepper.qml`).

Library: QML `Stepper`, web `.l-stepper`, Go `kit.Stepper`, terminal `term.RenderStepper`

### Picker

For one of many values. A 44-tall field anchor (`field` fill, `edge`
hairline, cut 7, inset 14) with the value in `body` and a 20 `expand_more` in
`fg2`; it opens a Menu in a Popover with the current choice checked and
scrolled into view. The menu never covers its field. In a row it is as wide
as its widest option plus its insets and arrow, at least 160 and at most 320.

Library: QML `Picker`, web `.l-picker`, Go `kit.Picker`, terminal `term.RenderPicker`

### TextField

44 tall, cut 7, inset 14, `body` text on the `field` fill with a 1 px `edge`
hairline at rest; the hairline turns `fg2` with keyboard focus and `jasper`
with an error. Placeholder in `fg3`. In a row it is 320 wide, mono when the
row holds literal text, and writes only text the reader typed
(`RowControl.qml:284-310`).

Library: QML `TextField`, web `.l-text-field`, Go `kit.TextField`, terminal `term.RenderTextField`

### ChordField

One keyboard shortcut, on the text field's surface: 44 tall, cut 7, at least
160 wide, the chord as keycaps (24-tall cut-3 plates, 4 apart). Click, Enter
or Space records; Escape cancels; Backspace clears. A clash shows a `caption`
line in `jasper` 4 below. The Hub uses it in shortcut rows and Default apps
(`components/shortcuts/BindRow.qml`; `components/default-apps/KeyControl.qml`).

Library: QML `ChordField`, web `.l-chord-field`, Go `kit.ChordField`, terminal -

### ColorField

One colour: a 44 swatch over a Hatch, cut 3, beside a mono hex field whose
trailing plaque opens the swatches. The Hub sets it 220 wide and offers the
roles Accent, Jade, Lapis, Amber and Ink as swatches (`RowControl.qml:313-335`).

Library: QML `ColorField`, web `.l-color-field`, Go `kit.ColorField`, terminal -

### PathField

A file or folder: a mono text field, 320 wide in a row, with a small quiet
`folder_open` plaque in the reveal slot that asks the Hub to choose. Folders
use the Hub's own chooser, files the portal dialog
(`RowControl.qml:365-408`; `components/FolderChooser.qml`).

Library: QML `PathField`, web `.l-path-field`, Go `kit.PathField`, terminal -

### Disclosure

Optional content grown in place under its trigger, 8 below, over 200 ms.
The Hub's "More options" at a slab's foot is one; its rows are built the
first time it opens (`SettingsPage.qml:826-852`).

Library: QML `Disclosure`, web `.l-disclosure`, Go `kit.Disclosure`, terminal `term.RenderDisclosure`

### ListEditor

A list edited in place inside a slab: items stacked like setting rows, 13
apart with a centred `vein` hairline, at least 44 tall, Edit and Remove small
quiet IconPlaques 8 apart and 21 from the text, a 20 `drag_indicator` handle
where the list can be reordered, and a quiet Add plaque that names what it
adds. Remove acts at once and offers Undo. The Hub uses it for launcher apps,
graphics presets, key sequences and profile setup
(`components/launcher/AppListEditor.qml`; `components/graphics/YourPresets.qml`).

Library: QML `ListEditor`, web `.l-list-editor`, Go `kit.ListEditor`, terminal -

### Design tiles

A `DesignTile` is one look in a gallery: a cut-6 `slab` tile with a `vein`
hairline, a 16:10 preview in a 4 inset, the title in `row`, and a second line
that always reserves 32 for the status. The look in use takes a `sel` plane
under its text. A `DesignGallery` lays them in as many columns as fit tiles
of at least 200, 8 apart, each up to 260. The Hub's lock screen designs, bar
styles and the launcher, notification and quick settings designs use them
(`pages/LockScreen.qml`; `components/bar/StyleGallery.qml`;
`components/DesignChooser.qml`).

Library: QML `DesignTile` and `DesignGallery`, web `.l-design-tile` and `.l-design-gallery`, Go `kit.DesignTile` and `kit.DesignGallery`, terminal -

## Containers

### Slab

A section's slab: the `Tone.card` plane (the paper in Lithic) cut 7, inset 21,
set off by a 1 px `vein` hairline, never a lighter plate
(`Lithic/Slab.qml:10, 30-39`; `SettingsPage.qml:800-804`). The one thing that
needs the reader may stand on a **toned slab**: its mineral field in place of
the paper and no hairline. Home's "Needs you" is the Hub's only one, on the
amber field (`Home.qml:798-815`).

Library: QML `Slab`, web `.l-slab`, Go `kit.Slab`, terminal `term.RenderSlab`

### Section

A `section` title, 13 to its content, 21 below. A ruled section draws a
1 px `vein` hairline from 13 after its title to 13 before its mark, and the
mark (the section ordinal, `01 / 08`) in `monoSm` `fg3` at its far edge
(`Lithic/Section.qml:27-63`). Every section of a page sheet is ruled; a
section in a dialog is not ([layout.md](layout.md#the-page-sheet)).

Library: QML `Section`, web `.l-section`, Go `kit.Section`, terminal `term.RenderSection`

### PageRow

One page in a list: 48 tall with its summary (36 without), its title in `row`
over the summary in `detail` `fg2`, 2 apart, inset 13, cut 6, the hover plane
on `stone1`. The Hub lists nearby pages under "No settings match" and the
categories on a compact Home with it (`components/PageRow.qml`;
`components/SearchResults.qml:227-236`; `Home.qml:881-897`).

Hub: `components/PageRow.qml`

### Facts

Label and value rows at least 44 tall, open with no line between: labels in
`fg3` in a 140 column, values 21 after in `fg`, mono only for literal data;
both wrap. An absent value is `-`. The Hub uses it in Keyboard and the import
steps (`pages/Keyboard.qml`; `components/backup-import/ImportSteps.qml`).
Home's datasheet is drawn by Home itself: keys in `monoSm` `fg3` lowercase, one
`vein` column rule, values in `detail` `fg` (`Home.qml:909-946`).

Library: QML `Facts`, web `.l-facts`, Go `kit.Facts`, terminal `term.RenderFacts`

### RowList

A navigable list with one `sel` selection that glides between rows over
240 ms; rows 4 apart; a 3-wide `vein` scroll thumb. The Hub's choosers use it:
folders, fonts and icons, keyboard layouts, launcher apps
(`components/FolderChooser.qml`; `components/keyboard/LayoutChooser.qml`).

Library: QML `RowList`, web `.l-row-list`, Go `kit.RowList`, terminal `term.RenderRowList`

### Tablet

A resource row: a `stone2` tablet 56 tall (48 compact), square apart from
its 12 notch, a 32 mark, the title in `row` over one facts line, and a
trailing claim and proof that give way to the row's one action under the
pointer. The Hub's launcher app list uses it
(`components/launcher/AppListEditor.qml`); Rashin and Ryotunes list their
resources with it.

Library: QML `Tablet`, web `.l-tablet`, Go `kit.Tablet`, terminal `term.RenderTablet`

### Claim, Marks and Sentence

A **claim** is a Flake beside a plain state word in `label`, in the state's
mineral (`fg2` for idle); its proof stands beside it. Home's claim line and
"Needs you" head are claims (`Home.qml:769-807`). **Marks** are short traits
in `detail`, 13 apart with no separator (`components/users/PersonSlab.qml`).
A **sentence** composes spans in one ink, a span switching to mono for literal
data (`pages/Updates.qml`).

Library: QML `Claim`, `Marks`, `Sentence`; web `.l-claim`, `.l-marks`, `.l-sentence`; Go `kit.Claim`, `kit.Marks`, `kit.Sentence`; terminal `term.RenderClaim`, `term.RenderMarks`, `term.RenderSentence`

### ScrollRail

The scroll thumb of every scrolling page, the tree and the search results: a
3-wide blade with both ends cut, at least 12 long, in `edge` at rest and `fg3`
while hovered or dragged, shown only while the content overflows. A press on
the track pages by one view over 140 ms (`SettingsPage.qml:914-919`;
`HubSidebar.qml:110-117`).

Library: QML `ScrollRail`, web `.l-scroll`, Go `kit.ScrollRail`, terminal -

## Search

Search is the sidebar's (`components/nav/HubSidebar.qml:69-141`;
`components/SearchResults.qml`). Render: [search](media/hub-search-dark.png),
[light](media/hub-search-light.png).

- **The field** is the Lithic `Search`, set 36 tall in the sidebar: a 20
  `search` icon at 14, 42 reserved at the leading side, "Search settings" as
  its placeholder and "/" as its hint in `fg3`. Escape clears it.
- **Entry:** `/`, Ctrl K or Ctrl F, or on a compact Home the **SearchDoor**,
  a button drawn as the field at rest (44 tall, up to 280 wide) that draws the
  sidebar out (`Hub.qml:792-803`; `components/SearchDoor.qml`).
- **Results** replace the tree while there is a query, cross-fading over
  120 ms. They are grouped by page, each group head in `label` `fg2` inset 13.
  Each hit is 48 tall: its name in `row` with the matched words underlined,
  over its crumb ("Category › Page › Section") in `detail`, 2 apart, inset 13,
  cut 6. The cursor hit and the hovered hit take the hover plane; a hand-off
  hit shows a 17 `open_in_new` in `fg2` (`SearchResults.qml:129-195`).
- **Keys:** Up and Down move the cursor, Return opens the hit.
- **Nothing found:** "No settings match “blur”" in `body` `fg2`, then
  "Nearby pages" in `label` `fg2` over PageRows, 13 apart
  (`SearchResults.qml:205-237`).
- **What it finds:** pages, sections, rows with their options and keywords,
  window manager rows, hand-offs and Home's studio entries; everyday words map
  to the Hub's own ("transparency" finds opacity, "hotkey" finds shortcut)
  (`Hub.qml:538-549, 608-704`).

Library: QML `Search`, web `.l-search`, Go `kit.Search`, terminal `term.RenderSearch`; Hub: `components/SearchResults.qml`, `components/SearchDoor.qml`

## Notices with Undo

Every change the Hub makes is announced by a **placard** with Undo
(`Hub.qml:838-845, 1104-1158`; `Lithic/Placard.qml`):

- the words: the change's own notice, or "%1 changed" with the setting's
  name; the action: Undo, an `on` small plaque;
- a `sel` plane cut 8 with a `vein` hairline, 48 tall, at most 560 wide, 24
  above the bottom; text in `onSel` inset 16;
- at most three stacked, 8 apart, the newest at the foot;
- a 2 px `onSel` blade drains across its 5 s life; the life stands still
  while the pointer rests on it or it holds focus; under reduced motion a
  timer counts the life with the blade held whole;
- it enters over 300 ms on the chisel curve, rising 8.

A drag or a held key on one setting updates its own placard; changes with a
notice of their own each get one (`Hub.qml:1109-1129`). Ctrl Z undoes the
newest change on the page, or the newest anywhere from Home; Home also keeps
"Recently changed", each with a quiet Undo (`Hub.qml:797-800`;
`Home.qml:1005-1044`). A page keeps the focused control clear of the newest
placard ([layout.md](layout.md#scrolling)).

Library: QML `Placard`, web `.l-placard`, Go `kit.Placard`, terminal `term.RenderPlacard`

## Dialogs and confirms

**Dialog** (`Lithic/Dialog.qml`): a `stone4` plane cut 16 with a `vein`
hairline, 520 wide by default and at least 16 from the window, padding 34; the
title in `sentence` with 21 below it; the actions 34 below the body, 8 apart,
from the start edge. It stands over `Lithic.scrim`, which swallows a press
outside; Escape or that press closes it. It enters over 300 ms rising 12 and
leaves over 160 ms dropping 8. The Hub opens dialogs for choosers and editors
as well as decisions (23 files); the card studio is one 1280 wide
(`CardEditor.qml:93-95`).

**Confirm** is the Hub's one way to ask (`Hub.qml:381-399`;
`Lithic/Confirm.qml`): `hub.confirm({title, text, actionText, cancelText,
destructive, requiredName, timeout})`.

- Cancel is a quiet plaque; the action is `danger`, or `gold` when the
  question is not destructive. A confirm is destructive unless it says
  otherwise.
- The action names the verb ("Log out"), never Yes.
- `requiredName` asks the reader to type a name ("Type %1 to confirm") in a
  TextField 21 below the text before the action enables.
- With a `timeout` it counts down, "Reverting in %1 s.", and answers no by
  itself (`Hub.qml:1067-1102`).
- Escape or a press outside answers no.

Example: "Log out now?", "Every open app closes, and work you have not saved
is lost.", Log out (`Hub.qml:367-374`). Render: [dialog](media/hub-dialog-dark.png),
[light](media/hub-dialog-light.png).

Library: QML `Dialog` and `Confirm`, web `.l-dialog` and `.l-confirm`, Go `kit.Dialog` and `kit.Confirm`, terminal `term.RenderDialog` and `term.RenderConfirm`

## Popovers, menus and info buttons

**Popover** (`Lithic/Popover.qml`): anchored 8 from its control and kept 8
inside the window. It opens below when its content fits, otherwise above
when that fits. When neither fits, it uses the side with more room and
shrinks to that room, never across the control that opened it. A press
outside closes it and is swallowed. It enters over 160 ms sliding the last
6, and leaves over 100 ms sliding 4. Closing hands focus back to the control
that opened it.

Go hosts drawing an anchor away from the window origin pass the window
rectangle in anchor-local coordinates through `Popover.Viewport` or
`Picker.Viewport`. An empty rectangle uses the local layout constraints;
it never invents space above the anchor.

**Menu** (`Lithic/Menu.qml`): `stone4`, cut 8, 220 to 360 wide, 6 above and
below; items 40 tall, inset 16, a 20 icon, 12 gap, `label` text, a shortcut 13
after in `fg3`; a checked item shows `check` in `fg2` at its end, never a `sel`
fill; dividers are 1 px `vein` inset 8. A menu shows at most ten and a half
items (`menuVisibleItemsCount`): the part of the next item makes the overflow
readable. Available room inside the window can make it shorter. Overflow
scrolls inside the cut shape with wheel, touchpad, drag, and a ScrollRail
thumb. Wheel events stop here, including at either end, never on the page
behind it. Items are created as they enter view, not all at open.

Opening brings the current choice into view. Up, Down, Home, End, PageUp,
and PageDown keep the focused item visible; pointer-opened menus keep the
keyboard mark quiet until a key moves focus. Labels share the primary font's
reference-cap baseline, so a fallback script never lowers a row's text.
The Hub's choosers and time zone search use these same menus
(`components/app-rules/ChooserPopover.qml`;
`components/time-language/LocationField.qml`), including embedded previews.

**An info button** is an `info` IconButton beside a row's control that opens
a popover of explanation: `stone4`, a `vein` hairline, cut 8, 360 wide (the
menu's widest), its words in `detail` `fg` with 21 around them. The live-card
switch carries one (`components/about/CardOnlineSwitch.qml:32-82`).

**No tooltips.** The Hub draws none. Its hover explanation is the sidebar's
[peek](layout.md#the-sidebar-tree).

Library: QML `Popover` and `Menu`, web `.l-popover` and `.l-menu`, Go `kit.Popover` and `kit.Menu`, terminal `term.RenderMenu`

## Empty, waiting and error states

| State | What the Hub shows | Source |
|---|---|---|
| nothing waits | one claim line: "Up to date" (jade), "Checking for updates" (lapis) or "Updates can't be checked" (jasper), with "Ryoku %1, %2" | `Home.qml:769-786` |
| something waits | the toned "Needs you" slab: its claim, then setting rows; the first item carries the one gold plaque, the rest are `line` | `Home.qml:798-877` |
| the desktop service is down | one `jasper` detail sentence that says what still works and that Settings will pick it up again | `Home.qml:788-796` |
| a write failed | the row's detail in `jasper` and a quiet Retry plaque | `SettingsPage.qml:968-990` |
| a section has nothing to show | its `missing` note in `detail` `fg3` | `SettingsPage.qml:863-871` |
| an empty binder | three blank cards fanned at −14°, 0° and 14°, then "A place for your friends' cards" and one explanation | `components/about/Binder.qml:397-444` |
| an empty list with a next step | Lithic `Empty`: a 72 octagonal `vein` frame with a 30 `fg3` icon, a `title`, a `body` up to 420 wide and its plaques 8 apart | `Lithic/Empty.qml`; `pages/LockScreen.qml:338` |

**Long work** follows one pattern, copied in each place that needs it
(`components/backup-import/WorkLine.qml`; `components/add-ons/TaskLine.qml`;
`docs/hub.md:724-734`):

1. nothing for the first second;
2. then `Working` with a word for what is happening;
3. past 10 s, a `BladeProgress` with Cancel.

**Working** is three flakes, each a fifth of its size wide and half high, a
tenth apart, filling with `lapis` in turn over 900 ms on a `vein` base at 0.55;
reduced motion holds them full. **BladeProgress** is a `lapis` blade on a 6
`field` track with a 2-wide `gold` marker at its leading edge while active;
indeterminate, a quarter-width segment sweeps over 1280 ms, still under
reduced motion. Both run only while a job is live.

Library: QML `Empty`, `Working`, `BladeProgress`; web `.l-empty`, `.l-working`, `.l-blade-progress`; Go `kit.Empty`, `kit.Working`, `kit.BladeProgress`; terminal `term.RenderEmpty`, `term.RenderWorking`, `term.RenderBladeProgress`

## Charts on overview screens

An overview screen shows how what it oversees moves, as a graph. Render:
[readings](media/hub-overview-dark.png), [light](media/hub-overview-light.png).

**Readings.** Home's "This computer" draws up to four readings, Processor,
Graphics, Memory and Network (`Home.qml:947-992`). Each cell has its name in
`caption` `fg2`, its value in `section` with tabular figures, an engraved
chart, and a proof line in `caption` `fg3`, 4 apart. The cells are 21 apart,
four across from `4 × 112 + 3 × 21` of width, two otherwise. The values count
up over 640 ms once.

**EngravedChart** (`Lithic/EngravedChart.qml`): a 1.5 px `fg` line over a
Hatch, a 1 px `vein` baseline, 4 px ticks and `monoSm` labels; no area fill,
no extra colour, no gradient. Home's are 44 tall over the last 40 samples,
0 to 100 except the network's (`Home.qml:974-982`). Short or absent data shows
an `fg3` sentence instead.

**The Activity map** in the sidebar is the Hub's other graph: a contribution
map of the hours the machine was on, one 9 px square a day, shaded in four
steps of `sel` from 0.3 to 1 by under 3, 6, 12 and 12 or more hours, empty
days `fg` at 0.07 in dark and 0.09 in light, today outlined in `fg2` and the
hovered day in `fg`; resting on a day reads it out in the summary line
(`components/sheet/Activity.qml:85-92, 205-277`).

**BladeMeter**: a measured fraction on a 6 `field` track, `fg2`, turning
amber at 90 percent. The Hub shows a person's storage with one
(`components/users/PersonSlab.qml:218`).

Performance, Power, Graphics and About are pages of rows today and draw no
graph; Home and the Activity map are the Hub's graphs.

Library: QML `EngravedChart`, `BladeMeter`; web `.l-engraved-chart`, `.l-blade-meter`; Go `kit.EngravedChart`, `kit.BladeMeter`; terminal `term.RenderBladeMeter`; Hub: `components/sheet/Activity.qml`

## Cards

### System card

The reader's machine as a trading card (`components/about/TradingCard.qml`),
shown on Home in card view, in the card studio, in the viewer and in friends'
binders. Render: [Home](media/hub-home-dark.png).

| Part | Geometry |
|---|---|
| card | 630 × 880, a playing card's proportion, scaled by its host |
| frame | outer cut 22 in the rim colour; face inset 14 (20 for Secret rare), cut 14; the face stroke 2 px `gold` from Holo rare, 1 px otherwise |
| head | "RYOKU MACHINE · <tier>" in `monoSm` 12 capitals, tracking 1.6, with "· 力の機械" while the gloss is on; the name in `name` at 42; the power at 44 Black with a 力 before it |
| window | 570 × 400 at (30, 104), cut 8, a 3 px stroke, the figure inside, the emblem seal overlapping its corner |
| type line | a cut-6 plaque 30 tall: "No. <serial> distro · wm · gpu" in `monoSm` 11 |
| moves | Processor, Display and Memory: a cut-6 30 × 30 glyph chip, the name in `label` 19 Bold, the value in `name` 30 Black |
| card code | three tick rulers on 14-tall strips: ticks 2 wide at a pitch of 4, their heights the card's code |
| flavour | a cut-6 box 46 tall, the motto in `proof` 14 italic |
| foot | the RyokuMark and release · serial at the left, "<TIER> · N pts" at the right, `monoSm` 10 |

All card text is drawn as outlines (`Text.QtRendering`) so it stays sharp at
any scale. At rest a card is still: its effects run only under the pointer
with motion not reduced, and on Home only while Settings is focused
(`Home.qml:604`).

**Rarity** (`registry/rarity.js:7-14`): six tiers by points, Common ● (0),
Uncommon ◆ (6), Rare ★ (9), Holo rare ★ (12), Ultra rare ★★ (15), Secret rare
★★★ (18). The rim goes from bone to sun to gold; from Ultra rare the art runs
full and six sparkles show. **Secret rare** stands apart: a wider,
diamond-faceted border (`shaders/diamond.frag`), an etched mineral foil, a
gilded name, twelve sparkles in four colours, a beating ★★★ and a gold sweep
every few seconds. The foil is smooth, never dotted, and capped at 0.6 alpha
(`shaders/foil.frag:2-4`).

**Sharing.** Copy and Save draw the card off-screen at twice its size, 1260 ×
1760 (`components/about/CardShare.qml`). The card restores from the picture
alone: its code is in the tick rulers, not in metadata and not in a visible
code.

Hub: `components/about/TradingCard.qml`, `CardShare.qml`, `CardFigure.qml`, `registry/rarity.js`

### Binder

Friends' cards (`components/about/Binder.qml`, `BinderCard.qml`,
`BinderCompare.qml`). Render: [binder](media/hub-binder-dark.png),
[light](media/hub-binder-light.png).

- **Panel:** `stone1`, cut 16, `min(1160, w − 48)` × `(h − 48)`, over a
  `ground` shade at 0.96 in dark and 0.90 in light.
- **Header:** a 28 gold `collections_bookmark`, "Binder", the count in mono
  `fg2` and 収集 while the gloss is on; then Paste a card (gold), Open a card
  (line), Refresh (quiet) and the close button.
- **Grid:** cards 248 wide in 296 cells, rarest first, then newest; a hovered
  card lifts 8 out of its pocket over 240 ms; its name in `label` DemiBold
  and "<mark> <tier> · N pts" in `caption` under it, gold from Rare. Keyboard
  focus is a 2 px gold ring, cut 10, 4 outside.
- **Detail:** the card at up to 430 wide beside a compare table (Threads,
  Memory, Refresh rate, Display width, Graphics class, Rarity points, Power)
  with values in mono; the winner of a line is "▲" Bold in gold.

**The card viewer** shows one card at up to 1.4× over a black shade at 0.82
in dark and 0.60 in light; Escape, a press on the shade or its close button
closes it (`components/about/CardViewer.qml`).

Hub: `components/about/Binder.qml`, `BinderCard.qml`, `BinderCompare.qml`, `CardViewer.qml`

### Card studio

A Dialog 1280 wide titled "Card studio" with one gold Done
(`components/about/CardEditor.qml`). Every change applies at once with Undo.
Render: [studio](media/hub-studio-dark.png), [light](media/hub-studio-light.png).

- **Left half:** the preview, a Segmented "The card" / "Home"; drag to move
  the picture, wheel to zoom. Home's preview is the real Home, inert, in a
  16:10 cut-6 frame.
- **Right:** Segmented tabs, inset 55:

| Tab | Holds |
|---|---|
| Picture | 76 × 76 cut-6 tiles: the stock figure, your picture, ten gallery figures; the chosen one is `sel` |
| Adjust | a slab of rows: the look (Photo, Black & white, Dots), fill, zoom, position, brightness, contrast, colour or dot size, "Ink the dark parts" |
| Words | the name, the line under it, the big words |
| Layout | plate or card on Home, how much shows, the figure's side, which parts show |
| Share | "Your machine draws a %1 card", the points per part, the rarity ladder (28-tall cut-6 lines; resting on a tier previews it on the card), the live-card switch, then Copy (gold), Save (line), Binder (quiet), Back to stock (quiet) |
| Badge | the emblem, the request code, activation, the team design |

Hub: `components/about/CardEditor.qml`, `BadgeTab.qml`

## Badge seal and certificate

### Emblem

The badge seal (`components/badge/Emblem.qml`): a flat two-ink seal in the
role's ink. Role inks are fixed per role and not themed. Its rim carries 96
ticks, every eighth major, with the role, "RYOKU" and the kanji (while the
gloss is on) set round it and the RyokuMark at the top; inside, the badge code
is written four times round a 45° square, with a light passing letter by
letter; at the centre a hanko, the role kanji knocked out of a cut plate (the
founder's carries the RyokuMark). It moves only while it is live, visible and
motion is not reduced: the rim turns once every 90 s and the light passes over
2600 ms, then rests 5200 ms. On the card studio's Badge tab it is live while
the tab shows (`components/about/BadgeTab.qml:63`).

Hub: `components/badge/Emblem.qml`

### Certificate

The badge's certificate (`components/badge/Certificate.qml`): a 1080 × 690
page scaled to fit, cut 32, `stone1` with an `edge` stroke, over a `ground`
shade at 0.97, with the faint hexagon lattice behind it (28 px, strength 0.5,
opacity 0.35; `shaders/design-atmosphere.frag`). It holds a 400 emblem;
"Ryoku <Role>" in `name` 45 Black; "@login" at 28 in the accent; the role's
sentence in `body` 17 `fg2`; "Issued by / neur0map" and "Issued on"; a
verification plaque 530 × 48, cut 10, on a jade field when verified and gold
while pending; and at the foot the certificate number in mono 14, tracking 2,
with "ED25519 / SIGNED BY NEUR0MAP".

Hub: `components/badge/Certificate.qml`

## Parts for other app shapes

The Hub does not use these parts. Other Ryoku apps do, for window shapes the
Hub does not have ([layout.md](layout.md#other-window-shapes)); each keeps the
Hub's language for everything else.

### Notch

The top-left triangle `(x, y) (x+12, y) (x, y+12)` that marks one state on a
tablet or one pending change on an icon plaque. Rashin marks a session that
needs the reader with an amber notch (`ryoku/rashin/internal/app/`).

Library: QML `Notch`, web `.l-notch`, Go `kit.Notch`, terminal `term.RenderNotch`

### Blade

A bar with a 45° leading edge, its trailing top corner cut by its height:
the shape under progress, meters and the current tab. Ryotunes draws it
directly (`ryotunes-gio/internal/client/ui/`).

Library: QML `Blade`, web `.l-blade`, Go `kit.Blade`, terminal `term.RenderBlade`

### Tabs

Views of one resource: a 44 strip over a 1 px `vein` baseline, each label its
text plus 24, the current one `fg` with a 3 px `fg` blade that moves and
resizes over 240 ms, others `fg3`. At most five. Rashin's and Ryotunes's pages
use them; the Hub uses a Segmented instead.

Library: QML `Tabs`, web `.l-tabs`, Go `kit.Tabs`, terminal `term.RenderTabs`

### Filters

Chips that narrow a list, with live counts: 32 tall, inset 13, cut 6, 4
apart, `label` type; the selected chip is a `sel` fill that glides over
240 ms; the count follows by 6 in `caption`. Rashin, Ryotunes and Ryoport
narrow their lists with them.

Library: QML `Filters`, web `.l-filters`, Go `kit.Filters`, terminal `term.RenderFilters`

### GroupHeader

A group's name in `group` and its count in `caption`, both `fg3`; insets 14
top, 6 bottom, 6 left, 4 right. Groups appear only when they have rows. Used
in Rashin's and Ryotunes's lists.

Library: QML `GroupHeader`, web `.l-group-header`, Go `kit.GroupHeader`, terminal `term.RenderGroupHeader`

### Skeleton

Static `stone2` tablets, 4 apart, each with two `stone3` bars 8 tall and 8
apart at 60 and 35 percent of the width, holding a list's shape while it
loads. Rashin and Ryotunes use it; the Hub shows Working instead.

Library: QML `Skeleton`, web `.l-skeleton`, Go `kit.Skeleton`, terminal -

### Tooltip

After 600 ms of hover, a `stone4` slab cut 4 with a `vein` hairline, 6 by 8
padding, `caption` text up to 320 wide, 8 from its anchor, entering over
300 ms with an 8 slide. Ryoport names its unlabelled rail tiles with it
(Ryoport `ui/kit/tooltip.go`). The Hub uses the peek instead.

Library: QML `Tooltip`, web `.l-tooltip`, Go `kit.Tooltip`, terminal -

### SectionIndex

A settings page's sections as a side list: 200 wide, 34 from the column,
rows 48 tall and 4 apart, the current one a `sel` fill; below expanded it
becomes a Picker. Rashin's and Ryotunes's settings pages use it. The Hub hangs
its sections in the sidebar as the section outline instead
([layout.md](layout.md#the-sidebar-tree)).

Library: QML `SectionIndex`, web `.l-section-index`, Go `kit.SectionIndex`, terminal -

### Aside and DraftColumns

A draft form at most 600 wide with an aside that previews what saving will
make or write: from 820 of room the aside sits 34 to the right, 36 percent of
the width within 300 to 400; narrower, it follows the form. The aside is a
standard slab with a `title`, a `detail` `fg2` note and literal previews in
`monoSm` on a cut-6 `field` block. Rashin's forms use them.

Library: QML `Aside` and `DraftColumns`, web `.l-aside` and `.l-draft-columns`, Go `kit.Aside` and `kit.DraftColumns`, terminal `term.RenderAside` and `term.RenderDraftColumns`

### The list and detail frame

Ryoport's fleet window ([layout.md](layout.md#the-list-and-detail-window))
is built from parts the library carries for that shape. Ryoport draws its own
copies of them (Ryoport `ui/app/`, `ui/kit/`, `ui/pages/common/`):

| Part | What it is |
|---|---|
| `AppFrame` | fills the window, picks the size class at 1280 and 760, and composes the rail, the list pane, the work pane, the overlays and the window controls |
| `Rail` | the destinations: 88 wide with 64 tiles expanded, 72 with 56 tiles medium, a 64 bottom bar compact; the current tile a travelling `sel` fill |
| `NewPlaque` | the rail's one gold plaque, 62 × 52 expanded, 48 × 40 compact |
| `Pane` | a plane with its top corners cut 16 and bottom corners square, 10 below the top, 8 apart |
| `Header` | a selected resource's mark, name, proof, marks and actions, inset 30 top, 36 sides, 26 bottom |
| `MineralField` | the header's field, `stone2` mixed toward the resource's mineral |
| `SplitPlaque` | a main action and a 40-wide menu segment 2 apart |
| `BecomingName` | a resource's name filled from the left as it is made |

Library: QML `AppFrame`, `Rail`, `NewPlaque`, `Pane`, `Header`, `MineralField`, `SplitPlaque`, `BecomingName`; web `.l-app-frame`, `.l-rail`, `.l-new-plaque`, `.l-pane`, `.l-header`, `.l-mineral-field`, `.l-split-plaque`, `.l-becoming-name`; Go `kit.` with the same names; terminal `term.RenderMineralField` and `term.RenderSplitPlaque`
