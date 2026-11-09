# Layout

`AppFrame` is the window composition shared by Lithic apps. It fills the
ground, selects a size class, then arranges a rail of destinations, an
optional resource list pane and a work pane. Full pages such as Activity and
Settings drop the list. Structure changes at a size-class boundary; a wide
layout is never squeezed to fit a narrow window.

Reference: `ui/app/layout.go`, `rail.go`, `window.go`, `overlays.go`,
`ui/env/env.go`, `ui/pages/settings/view.go` in Ryoport.

## Size classes

| Class | Window width | Rail | List pane | Work pane |
|---|---|---|---|---|
| **expanded** | 1280 dp and up | 88 wide, 64 tiles, icons with labels | 384 | the rest |
| **medium** | 760 to 1279 | 72 wide, 56 tiles, icons only | 320 | the rest |
| **compact** | under 760 | none; a 64 high bottom bar | full width | replaces the list, with Back |

Default window 1280 × 800, minimum 640 × 520.

## The ground and the panes

The window is filled with `ground`. Panes sit on it:

- every pane starts **10 dp** below the top edge and runs to the bottom;
- the rail and the list pane are `stone1`, the work pane `stone2`;
- panes are **8 dp** apart (ground shows between them) and the work pane
  keeps 10 dp from the right edge in the web mock, 0 in the native app;
- a pane cuts its **two top corners at 16 dp** and leaves the bottom corners
  square, so the stack reads as slabs standing on the ground;
- in compact, the single pane ends 8 dp above the 64 bottom bar.

When the compositor draws no decorations, the app draws its own window
controls at the top right of the work pane in a 32 dp strip, and the empty
top strip of every pane drags the window.

The work-pane body and tabs keep **34 dp** side insets. Mineral headers keep
**36 dp**, as do Settings pages. A pane body starts 8 dp below its header or
tabs. The list pane's content is inset 10.

## The rail

The rail is `stone1` with no cut of its own (it is a pane). From the top:

| Slot | Expanded | Medium |
|---|---|---|
| Home (brand mark) | 56 high, mark 40 | 48 high, mark 40 |
| first destination starts at | 96 | 88 |
| tile | 64 × 64, 21 icon, cut 7 | 56 × 56, 21 icon, cut 7 |
| tile gap | 8 | 8 |
| New plaque | 62 × 52 gold, at y 168 | 48 × 40 gold, at y 152 |
| last destination (Settings) | 16 above the bottom | 16 above the bottom |

Destinations: Home (the mark, returns to the Now board), Fleet, New,
Activity, then Settings at the bottom. The current destination is a `sel`
tile that **travels** to its new place over 240 ms; hover steps a tile to
`stone3` (160 ms in, 100 ms out). Labels are `caption` in `fg3`, `fg` when
current. An amber notch at the top-left of the Settings tile is the only
badge the rail may show; it means an update is ready.

In compact the same destinations move into the bottom bar: five equal
columns, 48 high tiles cut 6, Home in the first column, New in the middle,
with their state language unchanged. The New menu opens above its tile, 8 dp
away.

## The list pane

Top to bottom, with the list inset 10 from the pane sides:

1. The page title in `section` ("Fleet") over one sentence of state in `body`
   ("Three running. mika-server needs you.") with the attention name in
   amber.
2. The search field (280 wide at rest, growing to the pane) with its
   shortcut hint ("/"); Ctrl K opens the command palette instead.
3. The filters with live counts: All, Machines, Servers, Consoles.
4. The row list, grouped in the fixed order **Needs you, Pinned, Running,
   Becoming, Stopped**; a group appears only when it has rows. A degraded
   server stays in Running with its failure as an amber proof.

Keyboard: Up and Down move the cursor and reveal it; Enter runs the row's
primary action; Ctrl Enter opens its work pane; / focuses the search.

## The work pane

With a resource selected: the mineral field header (mark, name, proof,
marks, act row) uses 36 side insets, then tabs in a 44 strip inset 34 at the
sides, then tab content inset 34 on `stone2`. Tabs are at most five per
resource kind.

With nothing selected, the work pane shows the **Now board** as sections.
Its content is centred up to 1240 wide:

1. The fleet sentence in `sentence`, followed by one proof line.
2. Pinned resource tiles, each with state and one action. A tile is at least
   154 high.
3. From 860 width, two columns with a 60/40 split and 34 gap. Needs you and
   Running now occupy the left; This computer and Recent activity the right.
   Below 860 these sections stack.
4. Needs you and Running now use rows with one action. This computer shows
   CPU, memory and free VM-disk space as quiet `fg2` BladeMeters. A meter
   turns amber at 90 percent.

Drafts occupy the work pane under a header. The form column is at most 600
wide. From 820 available width, an Aside sits 34 to its right, 300 to 400
wide, and previews what saving will make or write. Narrower, the Aside
follows the form and is at most 600 wide. Advanced fields stay in a
Disclosure. A field shows an error only after it was edited and then left.

The catalogue occupies the full work-pane width as a searchable grid. Tiles
are at least 200 and at most 260 wide, 128 high and 4 apart. A system opens
in a configurable Dialog set to 720 wide for release, edition, size and
Create.

Changing the work pane's content fades the new content in over 200 ms with
an 8 dp upward travel.

## Full pages

Activity and Settings omit the list pane and take the remaining width.

**Settings** is the model full page:

| Width | Column | Section index | Insets |
|---|---|---|---|
| expanded | `min(available, 994)`, centred | a sticky 200 wide index to the left of the column, 34 gap; rows 48 high, 4 apart, label inset 13 | 34 top and bottom, 36 sides |
| medium | `min(available, 760)`, centred | a row of filter chips above the column | same |
| compact | full width | none; a plain "Settings" title | same |

Each section is a `section` title, 13 gap, a `stone1` slab cut 7 with a 21
inset, 21 below. Setting rows inside a slab are a title in `row` with a
detail in `caption` 2 below, and the control (toggle, picker, stepper,
slider) right-aligned and no wider than 320; rows are 13 apart.

The index follows the reading position: a section becomes current when less
than a third of it is still visible above the next; at the very end the last
section wins; clicking an index row scrolls to its section and holds the
index on it until the user scrolls again.

## Overlays and the Escape stack

Overlays are drawn after the panes in this order: the rail's New popover,
the command palette, the confirmation dialog, placards. Each overlay that can
take focus does, so the key reaches it first.

Escape closes the **topmost** thing and nothing else:

1. an open menu or popover;
2. the command palette;
3. a draft in the work pane;
4. the catalogue;
5. the open detail, returning to the Now board.

While a confirmation dialog is open, only Escape and Ctrl Q remain live
among the window shortcuts. A popover's own full-window catcher swallows an
outside press before closing it. A dialog's scrim does the same. During
their exit, pointer input underneath stays blocked.

## Keyboard

| Keys | Does |
|---|---|
| Ctrl K | command palette: every resource and action |
| / | search the list |
| Up, Down | move through the list |
| Enter | the row's primary action |
| Ctrl Enter | open the selected row's work pane |
| Ctrl N | the New menu |
| Ctrl 1, 2, 3 | Fleet, Activity, Settings |
| Ctrl R | refresh now |
| Left, Right | move between tabs when a tab has focus |
| Space | toggle a toggle |
| Esc | close the top overlay, then the draft, then return to the Now board |
| Ctrl Q | quit |

Every clickable control is reachable by Tab, shows the focus ring only for
keyboard focus, and has an accessible name (icon plaques carry a description).

## Polling follows what is on screen

Nothing polls or redraws for what nobody is looking at. The contract Ryoport
uses, as a template:

| Source | Focused | Visible, not focused | Hidden |
|---|---|---|---|
| resource list | on file change, 10 s safety | same | 60 s |
| stats of the selected running machine | 2 s | 5 s | off |
| row metrics in the list | 5 s | 10 s | off |
| reachability | 15 s | 30 s | off |
| probes | 30 s, 10 s for the selected one | 60 s | off |
| web checks | 60 s | off | off |
| this computer, only while the Now board is on screen | 3 s | 15 s | off |

On battery every interval doubles. Jobs report their own progress whatever
is on screen, and a job's progress shows on its row, in its work pane and in
Activity at once.
