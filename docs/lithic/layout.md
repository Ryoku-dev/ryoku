# Layout

Ryoku Settings (the Hub) is how a Ryoku window is laid out: a desktop window
with a sidebar tree beside one page pane, the page printed on it as a sheet,
and everything that interrupts drawn over both. This page gives every
position and size the Hub uses, with the file that sets it. The words
(sheet, masthead, colophon, folio, plate, sidebar tree and the rest) are
defined in [design.md](design.md#2-vocabulary); the parts themselves are in
[components.md](components.md).

Sizes are logical pixels. Hub sources are cited relative to
`ryoku/hub/quickshell/`, library sources as `Lithic/<File>.qml`, relative to
`docs/lithic/library/qml/`. Renders of the real Hub:
[Home](media/hub-home-dark.png) ([light](media/hub-home-light.png)),
[a page](media/hub-page-dark.png) ([light](media/hub-page-light.png)),
[search](media/hub-search-dark.png), [a dialog](media/hub-dialog-dark.png).

## Size classes

Settings is a `FloatingWindow` of 1280 × 800, and it never gets smaller than
1280 × 800 (`ShellBody.qml:30-37`). It is a desktop window, not a phone
panel: the room beside the page is used, never stacked away.

| Class | Window width | What changes | Source |
|---|---|---|---|
| expanded | 1180 and up | the sidebar tree stands beside the page | `Hub.qml:57` |
| medium | 880 to 1179 (`wide`) | the same frame | `Hub.qml:44, 57` |
| compact | under 880 | the sidebar becomes a drawer over the page | `Hub.qml:44, 984-1048` |

Because the window's minimum is 1280, the drawer appears only when a
compositor sizes the window below its minimum anyway (`ShellBody.qml:35-36`).
Home has its own break at 860 (`roomy`, `Home.qml:25`), see
[the cover](#the-cover). Structure changes at a class boundary; a wide layout
is never squeezed to fit.

## The frame

The frame is drawn in `Hub.qml:901-1048`.

| Part | Geometry | Source |
|---|---|---|
| ground | a full-window plane in `Tone.side` | `Hub.qml:905-908` |
| sidebar tree | 272 wide (`sideWidth`), full height, at x 0 | `Hub.qml:46, 1037-1048` |
| page pane | x 272 (8 when compact), y 8, 8 in from the right and the foot; cut 16 at both top corners (`Tone.paneCut`) and 7 at both bottom corners (`Tone.cardCut`); a 1 px stroke in `Tone.paneStroke` | `Hub.qml:912-924` |
| registration sheet | fills the page pane | `Hub.qml:929-932` |
| masthead | x 28 (52 when compact), y 4, 34 tall, as wide as the pane less x and 10 | `Hub.qml:945-952`; `components/sheet/Masthead.qml:49` |
| stage | from the masthead's foot to the colophon's top, clipped | `Hub.qml:954-974` |
| page in the stage | inset 28 left, 4 right (its scroll rail rides the pane's edge), 8 top, 1 bottom | `Hub.qml:963-973` |
| Home in the stage | flush: 0 left and right, 1 top and bottom | `Hub.qml:965-972` |
| colophon | 26 tall at x 28, 6 above the pane's foot, ending 20 from its right | `Hub.qml:976-982`; `components/sheet/Colophon.qml:13` |
| folio | 22 in from the pane's right, as wide as the pane less 60, 200 tall, standing on the colophon; only on a page that fits | `Hub.qml:933-943` |
| close button | an `IconButton` (34 × 34) 10 in from the pane's right, y 6 | `Hub.qml:994-1001`; `components/IconButton.qml:13-14` |
| drawer button | compact only: a `menu` IconButton at (10, 10) in the pane | `Hub.qml:984-992` |

**One paper, drawn by hairlines.** In the Lithic style the sidebar and the
page pane are the same paper: the pane is set off by its 1 px `vein` cut
hairline, not by a lighter tone (`Hub.qml:910-911`; `Singletons/Tone.qml:7-10`).
The other styles lift the pane in their own planes. The planes come from
`Lithic.planes()` (`Lithic/Lithic.qml:77-88`), read through `Tone`:

| Style | Sidebar (`Tone.side`) | Page pane (`Tone.pane`) | Pane stroke | Section slab (`Tone.card`) |
|---|---|---|---|---|
| Lithic | `ground` | `stone2`, the same paper | `vein` | `slab`, the paper |
| Material | `ground` | `stone1` | none | `stone3` |
| Adwaita | `stone3` | `ground` | none | `stone4` |
| Fluent | `ground` | `stone3` | `vein` | `stone4` |

How a palette fills these roles is [palette.md](palette.md); the styles are
[styles.md](styles.md).

When the width class changes, the pane's x slides over 240 ms, OutCubic
(`Hub.qml:925-927`).

### The registration sheet

Under every page lies the registration sheet (`components/sheet/RegSheet.qml`):
a 1 px dot every 24 px, a 7 px plus on every fourth crossing, and a 13 px
register cross 14 px in from each corner of the pane. Each mark is a fixed
alpha of `fg`, lighter in dark than in light (`RegSheet.qml:10-44`). It is
drawn once per size and ink and never moves.

### The masthead

The masthead is one mono line over the page (`components/sheet/Masthead.qml`):

- at the left, 10 apart: the page number `§ 01.3` in `monoSm`, the accent,
  tracking 0.8; where it lives, `APPEARANCE  /  WINDOW STYLE`, in `monoSm`
  capitals, `fg2`, tracking 1.4; then the category and page gloss
  (`外観・窓飾`) in `caption` 11, tracking 1, `fg3`, while the gloss is on
  (`Masthead.qml:51-84`);
- at the right, 48 short of the pane's edge (`endRoom`, room for the close
  button): `P 02/39 · NIRI · ORPHEUS · 12:40:07`, the page's place among all
  pages, the window manager, the bar style and the clock, in `monoSm`, tracking
  1.2, the clock in `fg2` and the rest in `fg3`, 12 apart with a 3 × 3 square
  between facts (`Masthead.qml:36-47, 93-121`). The clock shows seconds only
  while Settings is focused and motion is not reduced (`Masthead.qml:31-35`);
- underneath, 2 above its foot, the tick ruler: 7 tall, a 1 px `vein` tick
  every 8 px, every fifth 7 tall and the others 4, on a 1 px baseline, ending
  48 short of the right (`Masthead.qml:123-145`). A 24 × 3 accent marker rides
  the ruler to show how far the page is read, moving over 120 ms OutCubic
  (`Masthead.qml:147-158`).

On Home it reads `§ 00  HOME  表紙`.

### The colophon

The colophon closes the sheet (`components/sheet/Colophon.qml`): the motto
"力と美のために" in `caption` 11, tracking 2, `fg3`, while the gloss is on,
then 16 to its right the page's id as a Code 39 barcode (`CardBarcode`, bar
height 9, unit 1, in `fg3`), right-aligned. Home's barcode reads `home`.

### The folio

A page short enough to fit fills its empty foot with its number set huge and
faint (`components/sheet/Folio.qml`): `mono` at 168, Light, tracking −6, in `fg`
at 0.05 alpha in dark and 0.07 in light, set in the sheet's lower right corner
with the page's kanji at 96 (`title`, same ink) 18 to its left while the gloss
is on (`Folio.qml:42-65`). A new number rolls
in over the old, 480 ms OutCubic, half a glyph's travel; reduced motion swaps
it at once. A page that scrolls shows no folio (`Hub.qml:935`).

## The sidebar tree

The sidebar is laid out like an editor's project tree
(`components/nav/HubSidebar.qml`). It is always shown beside the page, and it
holds the whole of Settings: there is no rail, no list pane and no second
level of navigation.

| Part | Geometry | Source |
|---|---|---|
| head | at (12, 8), 40 tall: the RyokuMark (20 × 18, `fg` with an accent part), 10 later "SETTINGS 設定" in `group` capitals, `fg2`, tracking 1.6, and at the end a 30 × 30 "close every folder" IconButton, at 0.35 opacity while there is nothing to close | `HubSidebar.qml:26-67` |
| search | the Lithic `Search` at x 10, 2 below the head, 36 tall (not the token 44), as wide as the sidebar less 20, hint "/" | `HubSidebar.qml:69-90` |
| tree | 10 below the search, inset 4 at each side, down to 4 above the foot; a Flickable that stops at its bounds, with a `ScrollRail` 3 past its right edge | `HubSidebar.qml:92-117`; `components/nav/NavTree.qml:183-186` |
| search results | in the tree's place, inset 6 at each side, while there is a query; the two cross-fade over 120 ms OutCubic | `HubSidebar.qml:101-141` |
| Activity map | at the foot of the tree's room, 6 up, only when the tree leaves room for it plus 28 | `HubSidebar.qml:119-126` |
| Updates foot | 8 above the window's foot, a 34 tall line | `HubSidebar.qml:155-178` |
| foot hairline | 1 px `Tone.line`, 12 in at each side, shown once the tree runs under the foot | `HubSidebar.qml:144-153` |

**Lines.** The tree's lines are `NavRow`s (`components/nav/NavRow.qml`):

| Line | Height | What it shows | Source |
|---|---|---|---|
| Home | 34 | numeral `00`, "HOME", gloss 表紙 | `NavTree.qml:241-253` |
| folder (a category) | 34 | chevron 16, numeral (`monoSm`), name in `group` capitals, tracking 1.4, DemiBold, a leader to its gloss, its page count (`monoSm` `fg3`, hidden while open) | `NavRow.qml:37, 104-217`; `components/nav/NavFolder.qml:47-62` |
| leaf (a page) | 30 | indented 22, no glyph; name in `detail` `fg2`, DemiBold and `Tone.selectInk` when it is the open page; gloss at 11; `open_in_new` for a hand-off page | `NavRow.qml:31, 37, 153-166`; `NavFolder.qml:124-141` |
| section of the open page | 26 | the section outline under the open leaf | `components/nav/NavOutline.qml:17` |
| Updates | 34 | numeral `09`, "UPDATES 更新", the release as trailing text or the Updates dot | `HubSidebar.qml:163-177` |

Numerals come from `registry/numbering.js`: Home `00`, categories `01` to
`08`, Updates `09`. Glosses come from `registry/registry.js:188-210` and show
only while `UiPreferences.showJapaneseGloss` is on: 12 on folders and
numbered lines, 11 on leaves, tracking 1, `fg3` at rest, `fg2` under the
pointer, the accent when selected or open (`NavRow.qml:181-196`).

**Guide and ticks.** An open folder hangs its leaves on a 1 px guide 16 in
from its left, in `Tone.guide`, or `Tone.guideLit` when it holds the open
page (`NavFolder.qml:75-86`). Each leaf draws an 8 × 1 branch tick from the
guide, in the accent on the open page (`NavRow.qml:83-92`). A numbered line
draws a 1 px leader from its name to its gloss or count (`NavRow.qml:168-179`).

**One selection mark.** The open page is marked by one cut-6 `Tone.select`
wash that travels between lines: 26 in from the left under a leaf, 4 under a
shut folder that holds the open page, 4 from the right (`NavTree.qml:188-235`).
Hover and press washes are `Tone.hover` and `Tone.press` inside the same cut,
starting 4 past the line's indent (`NavRow.qml:53-82`).

**The section outline.** The open page's sections hang under its leaf when
it has two or more (`NavOutline.qml:20`): 26-tall lines on a thread at x 36,
each with a section node (a 5 px ring in `Tone.guide`, or a filled 7 px
accent dot for the section being read) and its name in `caption` from x 55,
`fg3`, `fg` and DemiBold when it is being read (`NavOutline.qml:28-104`).
Choosing one scrolls the page to it.

**The Activity map** (`components/sheet/Activity.qml`): live CPU, MEM and TMP
figures in `monoSm` 9, month initials in `monoSm` 8, then a 7-row map of the
last weeks, one 9 px square a day with 2 between, at least 4 weeks, shaded by
the hours the machine was on, then a `monoSm` 9 capitals summary
(`51 DAYS · 21.3 H/DAY · STREAK 17`), tracking 0.6. Squares are square in
Lithic and follow the style's corner elsewhere (`Activity.qml:15-19, 205-277`).

**The peek.** Resting on a tree line for 480 ms shows a peek beside the
sidebar: the line's glyph (16, accent) and title (`label` DemiBold) over a
live one-sentence summary in `detail` `fg2` (`NavTree.qml:22-53`;
`HubSidebar.qml:180-257`). It is at most 300 wide, 4 right of the sidebar,
held 8 inside the window, the paper in Lithic (`stone4` in the other styles)
with a `vein` hairline and cut 7, over a hard offset shadow: the same shape at
(+2, +4) in black at 0.5 alpha in dark and 0.12 in light
(`HubSidebar.qml:203-219`). A 90 ms grace keeps it while the pointer moves
from line to line, and scrolling the tree hides it (`NavTree.qml:41-53`).

## The page sheet

A settings page is printed as a sheet (`components/SettingsPage.qml`). There
is no Save and no action bar: a change applies when it is made, with a
[notice](components.md#notices-with-undo) to undo it.

**The column** is `min(usable, 960)`, centred in the stage, where usable is
the stage width less the 3 px scroll thumb and 13 (`SettingsPage.qml:89-93`).
The page ends with 34 of blank space (`settingsInsetVertical`,
`SettingsPage.qml:700`).

**The page head** (`components/PageHeader.qml`):

| Part | Geometry | Source |
|---|---|---|
| title | `sentence` at 32/40 (the role is 34/40) | `PageHeader.qml:55-63` |
| page actions | the page's own plaques (Reset page) at the title line's end, 6 apart | `PageHeader.qml:64-71`; `SettingsPage.qml:741-749` |
| lead | `body` `fg2`, 4 below, at most 720 wide when nothing stands beside it | `PageHeader.qml:74-83` |
| state line | `detail` `fg`, 6 below | `PageHeader.qml:84-93` |
| note | a 14 `info` glyph and a `caption` `fg3` line, 8 below, 6 apart ("Some options are hidden because this window manager does not support them.") | `PageHeader.qml:94-113`; `SettingsPage.qml:728` |
| spec strip | 16 below, facts 18 apart: a label in `monoSm` capitals, `fg3`, tracking 1.2, then 6 later the value in `monoSm`, `fg` or the accent when lit (`SECTIONS 08 · SETTINGS 16 · CHANGED 03`, CHANGED lit above zero) | `PageHeader.qml:115-139`; `SettingsPage.qml:729-737` |
| beside the words | a live preview, 264 × 128 at y 44, from a head 660 wide; otherwise the category's plate, `min(288, 32%)` wide at y 4, from 720; the words keep 28 clear of it | `PageHeader.qml:32-49, 142-169` |
| Tategaki | while the gloss is on and a preview or plate shows: the page kanji set vertically in the 36 reserved beside it | `PageHeader.qml:37-41, 152-159`; `components/sheet/Tategaki.qml` |
| below the head | 26 | `PageHeader.qml:43-44` |

**The plate** (`components/sheet/Plate.qml`) is the category's sculpture as a
1-bit dither, 1.6 : 1, registered by four 10 px `edge` corner ticks, with its
credit in `monoSm` 9 `fg3` 6 below.

**Sections.** Each section is a Lithic `Section`, ruled
(`SettingsPage.qml:756-783`; `Lithic/Section.qml`): its title in `section`, a
1 px `vein` hairline from 13 after the title to 13 before the section ordinal
(`01 / 08`, `monoSm` `fg3`) at the column's edge, 13 to the content, and 21
below. Inside, a column with 8 between its parts holds an optional body, the
slab (`Tone.card`, cut 7, inset 21; `Lithic/Slab.qml`), an optional footer,
and the `missing` note in `detail` `fg3` when every row is hidden
(`SettingsPage.qml:789-871`). Rows inside a slab are
[setting rows](components.md#setting-row), 13 apart with a centred `vein`
hairline. "More options" is a Lithic `Disclosure` at the slab's foot, 13 below
the last row; its rows are built the first time it opens
(`SettingsPage.qml:820-852`). A closing "Related" section holds `tonal` small
plaques with `arrow_forward`, 8 apart (`SettingsPage.qml:876-898`).

**Hand-drawn pages** (Displays, Shortcuts and the like) build the same head and
the same ruled sections themselves (`pages/Displays.qml`).

## The cover

Home (`components/Home.qml`) is the cover: the reader's figure or system card
beside one column of who they are, what waits, and this computer. It is
designed at 1280 × 800 and lies flush in the stage, so the registration sheet,
masthead and colophon frame it like any page (`Home.qml:20-21`;
`Hub.qml:965-972`). Renders: [dark](media/hub-home-dark.png),
[light](media/hub-home-light.png), [readings](media/hub-overview-dark.png).

| Part | Geometry | Source |
|---|---|---|
| roomy | from 860 wide (`boardColumnsAt`) | `Home.qml:25` |
| side margin | 55 (`s6`) when roomy, 21 (`s4`) otherwise | `Home.qml:26` |
| column | roomy: `min(554, 46% − side)`; otherwise the width less both margins | `Home.qml:29-30` |
| column top | roomy: centred, at least 89 down; otherwise 76 | `Home.qml:705-710` |
| column parts | 34 apart (`s5`) | `Home.qml:712` |
| figure side | right of the column by default; `heroSide` can put it left | `Home.qml:54, 705` |
| top bar | 13 down (`s3`), 44 tall, from the side margin (40 more when compact, clear of the drawer button) to the close button's corner | `Home.qml:611-618` |

**The top bar** holds quiet small plaques: "Continue in …" at the left; at the
right, 8 apart, expand (icon only), Binder, Share and Customise
(`Home.qml:626-698`). In a compact window a SearchDoor stands first, at most
280 wide, since the sidebar's search is in the drawer (`Home.qml:620-625`).

**The column**, top to bottom (`Home.qml:714-1044`):

1. who: the name in `name` (52/56), `user@host` and the clock, the motto in
   `proof`, its gloss when there is no custom motto, and the tick ruler with its
   first segment in `gold` (`components/TickRuler.qml`);
2. what waits: one claim line when nothing does, or the toned "Needs you" slab
   whose first item carries the cover's one gold plaque;
3. compact only: the categories as `PageRow`s, 4 apart;
4. "This computer": the datasheet, the readings and "About this computer";
5. "Recently changed", each with Undo.

**The figure or the card.** Beside the column stands Home's own plate: the
sculpture with its carved word and light, or the system card
scaled to the room (`Home.qml:505-607`). With the stock figure and a roomy
plate, the museum label stands upright in the outer gutter (`Home.qml:34,
559-576`).

## Overlays and the Escape stack

Everything that interrupts is drawn over the frame, in this order from the
bottom (`Hub.qml:1024-1141`):

1. the drawer's scrim and the drawer itself (compact only);
2. the overlay layer: dialogs that pages and Home open (parented to
   `hub.overlayItem`), the confirm dialog (built the first time it is asked
   for), then the notices, which stand over everything.

| Overlay | Plane and size | Source |
|---|---|---|
| dialog | `stone4`, cut 16, `vein` hairline, 520 wide by default, at least 16 from the window, padding 34, over `Lithic.scrim` | `Lithic/Dialog.qml:9-43` |
| confirm | a dialog with Cancel and one action, see [dialogs and confirms](components.md#dialogs-and-confirms) | `Hub.qml:381-399, 1055-1102` |
| card studio | a dialog 1280 wide | `components/about/CardEditor.qml:93-95` |
| binder | its own overlay: a cut-16 `stone1` panel `min(1160, w − 48)` × `(h − 48)` over a `ground` shade | `components/about/Binder.qml:315-333` |
| card viewer | the card at up to 1.4×, over a black shade | `components/about/CardViewer.qml:31-48` |
| certificate | a 1080 × 690 page scaled to fit, over a `ground` shade | `components/badge/Certificate.qml:34-55` |
| popover | anchored 8 from its control, kept 8 inside the window | `Lithic/Popover.qml` |
| notices | up to three placards at the foot, 24 above the bottom | `Hub.qml:1104-1158`; `Lithic/Placard.qml` |
| peek | beside the sidebar, never taking focus | `HubSidebar.qml:180-257` |

A dialog or popover closes on Escape or a press outside it; its scrim
swallows that press (`Lithic/Dialog.qml:29`; `Lithic/Popover.qml`). Any open
dialog or popover closes when Settings navigates elsewhere, so a key never
changes a setting under it; notices stay (`Hub.qml:235-248`).

**Escape closes the topmost thing and nothing else.** An open popover or
dialog takes it first. Then `back()` goes one step (`Hub.qml:462-497`):

1. a search with a query: clear the query;
2. an empty search field: leave it;
3. a drawn-out drawer: put it away and give the keys to the page;
4. a page with a view of its own: step out of that view;
5. a page: go Home, the keys staying in the page area;
6. Home: close Settings.

## The drawer

Below 880 the sidebar is a drawer (`Hub.qml:44, 984-1048`). The page pane
starts 8 from the left and the masthead at 52, clear of the `menu` button at
(10, 10). The button, the search shortcuts and Home's SearchDoor draw the
sidebar out: it slides in from its own width to the left over 240 ms OutCubic,
over a `Lithic.scrim` that fades in over 240 ms. A press on the scrim, Escape
or a choice puts it away. While put away the sidebar takes no keys
(`Hub.qml:1037-1048`).

## Keyboard

| Keys | Does | Source |
|---|---|---|
| / , Ctrl K, Ctrl F | focus the sidebar's search (drawing out the drawer in a compact window) | `Hub.qml:525-528, 792-803` |
| Up, Down in the search | move the cursor through the results | `HubSidebar.qml:78-79` |
| Return, Enter in the search | open the result under the cursor | `HubSidebar.qml:80-81` |
| Up, Down in the tree | move line by line, into a folder's leaves and the outline | `NavRow.qml:43-44`; `NavOutline.qml:59-60` |
| Right on a folder | open it | `NavRow.qml:45` |
| Left | shut a folder, or from a leaf go up to its folder and shut it | `NavRow.qml:47`; `NavFolder.qml:135-140` |
| Return, Enter on a line | open the page or toggle the folder | `NavRow.qml:48-49` |
| Ctrl Tab, Ctrl Shift Tab | the next or previous page in the tree's order, skipping hand-off pages | `Hub.qml:810-825` |
| Ctrl Z | undo the newest change on this page; on Home, the newest of all | `Hub.qml:797-800` |
| Alt Left, Esc | back, see the Escape stack above | `Hub.qml:804-809` |
| Tab | the next control; every control is a Tab stop | |
| Return, Space on a setting row | act on a whole-row toggle, hand-off, action or link | `Lithic/SettingRow.qml:50` |

The focus mark shows only for keyboard focus. Lithic controls draw the 2 px
`fg2` focus ring 2 outside their cut; the Hub's own tree lines, outline lines
and IconButtons draw a 1 px `fg2` stroke on their own cut instead
(`NavRow.qml:75-82`; `components/IconButton.qml:26-27`). Every IconButton has
an accessible name.

## Scrolling

- **A page** scrolls in a `Flickable` that stops at its bounds, with a Lithic
  `ScrollRail` (a 3 px thumb, `edge` at rest) on the stage's right edge; the
  column never runs under it (`SettingsPage.qml:696-713, 914-919`).
- **Jumps** (an outline line, a search landing) animate over 200 ms
  (`navigate`) on the chisel curve (`SettingsPage.qml:665-694`).
- **Keeping a control in view.** A focused control is kept 34 from the
  stage's edges and above the newest notice: the notice's height plus 21
  (`SettingsPage.qml:511-528`; `Hub.qml:401-403`). Something taller than the
  view shows from its top.
- **Where the reader is.** The current section is the last whose top is in
  the top third of the view, unless less than a third of it is still showing
  above the next; at the very end the last section wins. After a jump the
  outline holds the chosen section until the page moves
  (`SettingsPage.qml:597-623`). The masthead's marker shows the same
  position.
- **A search landing** scrolls to the row, then lights it at the hover tone
  and fades it over 1600 ms. A resting pointer shows no hover until it moves
  2 px (`SettingsPage.qml:659-663, 902-912`).
- **Content that shrinks** never leaves the page scrolled past its end
  (`SettingsPage.qml:705-712`).
- **The tree** scrolls the same way; once it runs under the Updates foot a
  hairline appears over the foot (`HubSidebar.qml:144-153`).

## Polling follows what is on screen

Nothing polls or redraws for what nobody is looking at
(`docs/hub.md:107-123`).

| Source | When it runs | Pace | Source |
|---|---|---|---|
| Home's readings (`LiveStats`) | while Home is shown and its readings are on | 3 s while Settings is focused, 15 s while not | `components/Home.qml:168-185` |
| the Activity map's figures (`LiveStats`) | while the map is visible and Settings is focused, motion not reduced | 1.5 s | `components/sheet/Activity.qml:30-37`; `Singletons/LiveStats.qml:28, 113-119` |
| Home's clock | while Home is shown | 5 s | `Home.qml:159-166` |
| the masthead's seconds and the plate scan (`Ambient`) | while Settings is focused and motion is not reduced | 1 s; a scan every 11 s | `Singletons/Ambient.qml`; `Hub.qml:405-410` |
| the Updates page's times | while the Updates page is open | 30 s | `Singletons/Updates.qml:93-98`; `pages/Updates.qml:311-313` |
| the full machine read-out (`SysInfo.load`) | when About opens | once | `Hub.qml:36-39` |

Work is also put off until it is needed: the search index is built when the
search is first reached, the provider schema is read when a page or the
search needs it, the confirm dialog and the card studio are made the first
time they are asked for, and "More options" rows are built when the fold
first opens (`docs/hub.md:115-123`; `HubSidebar.qml:83-86`;
`SettingsPage.qml:838-844`).

## Other window shapes

The Hub is a settings window. Three other Ryoku apps have window shapes the
Hub does not, and keep their own layouts for them. They take the Hub's
language for everything they share with it: the paper, the cut, the type, the
plaques, the motion.

### The list and detail window

Ryoport, the fleet manager, is a list and detail window: a rail of
destinations, a list pane of resources and a work pane for the one selected
(Ryoport `ui/app/layout.go`, `rail.go`, `window.go`). The library carries its
parts as `AppFrame`, `Rail`, `Pane`, `Header`, `MineralField` and the rest,
see [components.md](components.md#the-list-and-detail-frame).

| Class | Window width | Rail | List pane | Work pane |
|---|---|---|---|---|
| expanded | 1280 and up | 88 wide, 64 tiles with labels | 384 | the rest |
| medium | 760 to 1279 | 72 wide, 56 tiles, icons only | 320 | the rest |
| compact | under 760 | none; a 64 high bottom bar | full width | replaces the list, with Back |

Panes start 10 below the top, stand 8 apart on the ground and cut their top
corners at 16. The rail holds Home, Fleet, New (its one gold plaque) and
Activity, with Settings at the foot; the current destination is a `sel` tile
that travels to its new place over 240 ms.

#### The list pane

The page title in `section` over one sentence of state, the search field,
the filters with live counts, then the rows (tablets), grouped in the fixed
order Needs you, Pinned, Running, Becoming, Stopped. The list is inset 10.

#### The work pane

With a resource selected: its mineral field header, tabs in a 44 strip, and
the tab's content inset 34. With nothing selected: the Now board, sections
open on the pane centred up to 1240, with an engraved chart of how the fleet
moved. A draft form is at most 600 wide with an aside 300 to 400 wide beside
it from 820.

#### Full pages

Activity and Settings take the list pane's room as well. A settings page in
such a window is a [page sheet](#the-page-sheet) like the Hub's.

### The player window

A media app is not a list and detail window. Its window is built around what
plays, so it has its own shape. Ryotunes is the reference.

| Part | Expanded | Medium | Compact |
|---|---|---|---|
| top bar | 56 on ground: mark, back and forward, the one search field, source switcher, window controls | same, the mark alone over the rail | mark; search opens across the bar; the switcher becomes one menu |
| navigation | 240 sidebar: destinations, then the user's collections; folds to the 72 rail and back from a toggle beside Back and remembers it | 72 rail with collection covers; the toggle opens the 240 sidebar as a sheet | 56 bottom bar |
| content | `stone2`, top corners cut by the pane cut (16 by default), inset 34 | inset 21 | full width, inset 13 |
| side panel | queue, lyrics and session tabs, 340 wide, docked while the content keeps 520 | docked or a sheet | a sheet, opened only by the user |
| player bar | 88, `stone1`, top corners cut 16, under every pane | same | a 64 player over the bottom bar |

The player window keeps its sidebar longer than the list and detail window:
it is expanded from 1100 dp, medium from 760, and measures in effective dp,
the window's size divided by the screen scale and the app's zoom.

Panes stand 8 apart on the ground. Crossing into compact closes a docked
panel instead of letting it cover the page.

The player bar has three zones. What plays sits on the left: art, title,
artists and the actions that belong to the track. Transport sits in the
centre, its plaques 13 apart, over the progress with a time at each end.
Where it plays and how it sounds sits on the right: panel toggles, volume and
sound. Each zone is a flex with a fixed share, so one never pushes into
another.

Play is the consequential action of a media window. The player bar's play
and pause is its standing gold action. A page may spend one more gold on the
action that plays it, such as Play on an album. Radio, download and
navigation never take gold.

Search has one entry point, the top bar field. A page may still narrow its
own list with a filter; that is not search.

The music may carry its own colour. A player can point the gold roles at a
colour sampled from the playing cover, kept to the cover's hue at a fixed
lightness per mode and re-toned to 4.5:1 on `stone2` for text and marks, as
`goldInk` is. Play, progress, the liked mark, the playing track, the sung
words and the field behind page titles then wear one colour, so nothing on
screen disagrees with what plays. A grey cover, or none, falls back to the
palette's own gold. A source, such as a streaming service, keeps its brand
colour as identity in the switcher only, never as state; it never replaces a
mineral or the selection.

The cover may tint the whole window, the way the Ryoku palette takes the
wallpaper: ground, stones, veins and inks are re-hued toward the cover's
dominant colour in OKLCH, each keeping its own lightness so every contrast
pair holds, with a small chroma that grows with how colourful the cover is.
Minerals never change, so state reads the same on any cover, and a black and
white cover leaves the palette alone. The listener can turn the tint off.

The pane cut may be the listener's: square, the default 16, deeper, or the
desktop's own window corners. Every pane re-cuts in place when it changes.

The now playing view replaces the panes between the two bars: the art in the
left 42 percent, lyrics or the queue on the right, all on the accent field.
The art itself plays and pauses. Home may open with a now playing card inside
its greeting field while something is loaded: the art, the track, progress,
the transport and an instrument slot for the visualizer.

A mini player is a landscape strip: the art as tall as the window, flush with
its edges, and one column of title, artists, progress and transport beside
it. It falls back to a portrait card only when the window is made tall.

While music plays, the visualizer rises from the bottom edge of the field: a
lane 32 high inside a page header's 40 bottom padding (18 in 24 when
compact), and an 88 band along the bottom of the now playing view (56 when
narrow). Lanes sit below the text, so no word ever lies on a blade. Its style
is the listener's: blades, a curve over the spectrum, the waveform, or the
stereo field inside a diamond (a half diamond in a lane, and blades in a
header lane too short for it).

Anchor overlays to the window, not to the slot that opened them: a popover
from the player bar opens above it and stays inside the window.

### The conversation window

A conversation app is not a list and detail window either. Its window is
built around one reading column and the field that feeds it, so it has its
own shape. Rashin is the reference.

| Part | Expanded | Medium | Compact |
|---|---|---|---|
| top bar | 56 on ground: mark and name over the sidebar, the sidebar toggle, back and forward, the one search field, window controls | same, the mark alone over the rail | mark and the list toggle; search opens across the bar |
| navigation | 264 sidebar: the new action, destinations, then the current destination's list, Settings at the foot; folds to the 72 rail from the toggle and remembers it | 72 rail; the toggle opens the 264 sidebar as a sheet | 56 bottom bar for the destinations; the toggle raises the new action and the list as a sheet |
| content | `stone2`, top corners cut by the pane cut: a 52 header, then one reading column, centred and at most 760 wide, with the composer standing at its foot | same, the column inset 21 | full width, the column inset 13 |
| side panel | the work beside the conversation, 440 wide, docked while the content keeps 480 | docked or a sheet | a sheet, opened only by the reader |

It takes its size classes and effective dp from the player window: expanded
from 1100 dp, medium from 760. Panes stand 8 apart on the ground with 8 at
the window's sides and foot.

**The sidebar.** From the top: one line plaque that starts something new in
the current destination (a question, a session, a project to add), the
destinations as rows 40 high with a 20 icon and a `label`, then the list that
belongs to the current destination, and Settings under a vein at the foot.
The current destination is a `sel` fill that travels between destinations.
The list is the destination's own: questions by day, sessions by state,
projects with their worktrees. Rows in it are not tablets. They lie flat on
the pane, 36 high for one line and 48 for a name over one facts line, 2
apart, and take `stone3` under the pointer and the `sel` fill when current.
The trailing 44 of a row holds what the row claims (the Working flakes, a
count, a time); under the pointer or the list cursor the claim gives way to
the row's one action, as on a tablet.

**A row says a state only when it has one.** A session that needs the reader
carries an amber notch and stands under a Needs you group header; one at work
shows the Working flakes; one at rest shows no mark and no word. A list never
repeats a state word on every row.

**The reading column.** Everything said sits in one column: the question, the
answer, the work in between and the composer share its two edges. A question
is a `stone3` slab cut 7, as wide as its text up to three quarters of the
column, set against the trailing edge. An answer has no slab: `proof` type
straight on the pane, as wide as the column. Blocks of one turn stand 13
apart, turns 34 apart. The column starts 21 under the header. A short
exchange sits at the top of the pane; one longer than the pane opens at its
end. The view follows the end while an answer is still arriving, until the
reader scrolls away. When they have, one small icon plaque above the composer
returns to the end.

**What is said is never cut.** A line of an answer breaks; it does not end in
an ellipsis. An address or a command longer than the column breaks where it
must, code that fits on a line stays whole on one, and a table gives its
narrow columns their width and wraps the wide ones. A face that has no italic
leans its emphasis rather than drop it.

**An answer is set for reading.** Its `proof` lines stand on a 24 line, not
the 21 of proof text that is glanced at, paragraphs 13 apart and list items
8. Code inside a sentence is told apart by its face alone: mono in `fg2`, one
step up at 13 so it stands as tall as the words beside it, with no plate and
no room of its own. A bracket or a comma belongs to the name it touches, and
a plate one stone step from the pane does not show on every palette, which
leaves only its gaps. A block of code keeps its slab.

**A command is shown once.** When an answer offers a command to run, one slab
cut 7 carries it: the command in `mono`, why in `detail`, its claim (how far
it reaches) and one line plaque that runs it. The slab stands where the text
shows the command, in place of the block of code, and never again under it.
Several commands share one slab under a `title`, set apart by space.

**Running it stays in the conversation.** The plaque runs the command in
place: the command stands where a question would, in `mono` on the question's
slab, and its output arrives under one work line. A window opened elsewhere
that shows nothing until the command ends reads as a button that did nothing.
A second, small icon plaque opens a terminal for the reader who wants one,
and it is the only action when the command needs a keyboard. A running line
carries the Working flakes and, from three seconds on, the time it has taken
in `caption` with tabular figures; while it runs its unfolded output shows
its end, not its start, since that is where the news is.

**Text can be marked.** Every paragraph, question and block of code can be
selected with the pointer and copied, with `sel` at 60% under the marked
words. A second press takes a word, or all of a piece of code, since that is
what gets copied whole. One stretch is marked at a time, and only while the
transcript holds the keyboard: marks that stay lit after the keyboard has
gone look like something a copy would take. The message menu offers the
marked words first: to copy, or to quote in the reply.

**Work between question and answer** is one line, not a card: a 16 icon,
what was done in `label`, the literal part in `monoSm`, and its claim only
while it runs or when it failed. Steps that ran back to back fold into one
line with a count once the last of them has finished. A line unfolds in place
to what went in and what came out, on a slab cut 7. While a turn runs and
nothing in the column shows it, the Working flakes stand at the column's
foot; a step that is running already carries them, so the foot never repeats
it.

**A request for permission** is a mineral field in amber, cut 7, in the
column: its claim, what is asked in `title`, the literal command or path in
`mono`, then one gold plaque for the narrowest consent, a line plaque for a
wider one and a quiet plaque to refuse. Once answered it folds to one line
that records the answer. A request can be answered from the composer too,
since that is where the keyboard is: with nothing typed, Enter gives the
narrowest consent and Escape refuses, and the composer's hint line says so
for as long as that holds. Words in the field are a message, so neither key
answers then. A key never answers a request that is off screen, or one that
has been on screen for less than half a second: it was pressed before anyone
could have read it.

**The composer** is one field with its controls inside: the `field` fill, cut
7, the text in `proof` with 16 around it, and under the text a row of small
plaques, what to send with on the left (attach, the model, the mode) and the
one gold icon plaque on the right. Like every field it keeps a 1 dp `edge`
hairline at rest, and the hairline turns `fg2` with the keyboard. It is at
least 96 high, grows a line at a time to eight lines and then scrolls. Send
is the standing gold action of a conversation window. While a turn runs the
same plaque is Stop, and with text in the field it offers to queue the
message or steer the turn instead.

**The choices in the composer's row** are quiet selects: the value itself and
a small arrow, 32 high, with an icon only where the value alone would not say
what it chooses. Each stands at its own width. When the row is too narrow for
that, the hint line goes first, then the widest choices give way and cut
their own labels; none is dropped. How far the conversation may go without
asking is one of these choices, beside the model. The value that stops the
asking is written in amber with an open lock, here and in every line that
restates it: a state the reader has to see is never only a word inside a
closed menu.

**A list in a select.** A short list of plain names is a Menu. When an option
needs a second line to be understood, the list shows it under the name in
`detail` and lets it wrap: an explanation cut to an ellipsis explains
nothing. From nine options on, a search field stands at the top and narrows
the list as the reader types, Up, Down and Enter pick, and the list opens on
the current choice with its check. The list is `stone4`, cut 8, 340 wide and
at most 336 high before it scrolls; a row is 36 high, or as tall as its two
lines with 8 above and below.

The keyboard belongs to the composer. It has it when its page opens, and
gets it back when a menu in its own row, the search field or a dialog closes,
and after an action in the transcript such as copying an answer. Marked text
is the one thing in the transcript that takes it, for as long as it is
marked; Escape, or a press that marked nothing, hands it back. Nobody should
have to press the field before they can type again.

**A new conversation** opens on the composer, not on an empty transcript: one
`sentence` that says what can be asked, one `proof` line under it, the
composer 680 wide at the optical centre of the pane, up to four things to try
as quiet small plaques, and under them one line that says what will answer:
a claim, a `caption`, and the way to change it. Sending the first message
moves the composer to the foot of the column.

**The header** is 52 high and holds the name of what is open in `title`, its
facts in `caption`, and its actions as small icon plaques at the trailing
edge, the rare ones behind one overflow plaque. A vein under it appears only
while the column is scrolled beneath it.

**The work beside it.** When a conversation changes files, the side panel
shows the changes without leaving the conversation: a file per row with its
status letter and path in `mono` and its counts in `caption`, each unfolding
to its diff. The panel never takes gold; committing belongs to the full
review.

A destination with nothing open shows its board in the content pane: the
`sentence`, one `proof` line, then what needs the reader first, with a chart
section of how the destination's work moved beside or above it. A board with
nothing to report does not stand empty. It says where work can start, on
slabs under a `title`: the places to work in with the one action that picks
each, or, when the thing every action needs is missing, the way to get it. A
list in the sidebar that has no rows yet says in one `caption` line what will
stand there.

Search has one entry point, the top bar field. It finds conversations,
projects and commands, and Ctrl K focuses it. Its results open under the
field, as wide as the field, and leave with it on Escape.
