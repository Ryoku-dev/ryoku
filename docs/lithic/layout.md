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

## The player window

A media app is not a list and detail window. Its window is built around what
plays, so it has its own archetype. Ryotunes is the reference.

| Part | Expanded | Medium | Compact |
|---|---|---|---|
| top bar | 56 on ground: mark, back and forward, the one search field, source switcher, window controls | same, the mark alone over the rail | mark; search opens across the bar; the switcher becomes one menu |
| navigation | 240 sidebar: destinations, then the user's collections; folds to the 72 rail and back from a toggle beside Back and remembers it | 72 rail with collection covers; the toggle opens the 240 sidebar as a sheet | 56 bottom bar |
| content | `stone2`, top corners cut by the pane cut (16 by default), inset 34 | inset 21 | full width, inset 13 |
| side panel | queue, lyrics and session tabs, 340 wide, docked while the content keeps 520 | docked or a sheet | a sheet, opened only by the user |
| player bar | 88, `stone1`, top corners cut 16, under every pane | same | a 64 player over the bottom bar |

The player window keeps its sidebar longer than the list and detail window:
it is expanded from 1100 dp, medium from 760, and measures in effective dp, the
window's size divided by the screen scale and the app's zoom.

Panes stand 8 apart on the ground. Crossing into compact closes a docked
panel instead of letting it cover the page.

The player bar has three zones. What plays sits on the left: art, title,
artists and the actions that belong to the track. Transport sits in the
centre, its plaques 13 apart, over the progress with a time at each end. Where
it plays and how it sounds sits on the right: panel toggles, volume and sound.
Each zone is a flex with a fixed share, so one never pushes into another.

Play is the consequential action of a media window. The player bar's play and
pause is its standing gold action. A page may spend one more gold on the action
that plays it, such as Play on an album. Radio, download and navigation never
take gold.

Search has one entry point, the top bar field. A page may still narrow its own
list with a filter; that is not search.

The music may carry its own colour. A player can point the gold roles at a
colour sampled from the playing cover, kept to the cover's hue at a fixed
lightness per mode and re-toned to 4.5:1 on `stone2` for text and marks, as
`goldInk` is. Play, progress, the liked mark, the playing track, the sung words
and the field behind page titles then wear one colour, so nothing on screen
disagrees with what plays. A grey cover, or none, falls back to the palette's
own gold. A source, such as a streaming service, keeps its brand colour as
identity in the switcher only, never as state; it never replaces a mineral or
the bone selection.

The cover may tint the whole window, the way the Ryoku palette takes the
wallpaper: ground, stones, veins, inks and bone are re-hued toward the cover's
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
its edges, and one column of title, artists, progress and transport beside it.
It falls back to a portrait card only when the window is made tall.

While music plays, the visualizer rises from the bottom edge of the field: a
lane 32 high inside a page header's 40 bottom padding (18 in 24 when compact),
and an 88 band along the bottom of the now playing view (56 when narrow). Lanes
sit below the text, so no word ever lies on a blade. Its style is the
listener's: blades, a curve over the spectrum, the waveform, or the stereo
field inside a diamond (a half diamond in a lane, and blades in a header lane
too short for it).

Anchor overlays to the window, not to the slot that opened them: a popover
from the player bar opens above it and stays inside the window.

## The conversation window

A conversation app is not a list and detail window either. Its window is built
around one reading column and the field that feeds it, so it has its own
archetype. Rashin is the reference.

| Part | Expanded | Medium | Compact |
|---|---|---|---|
| top bar | 56 on ground: mark and name over the sidebar, the sidebar toggle, back and forward, the one search field, window controls | same, the mark alone over the rail | mark and the list toggle; search opens across the bar |
| navigation | 264 sidebar: the new action, destinations, then the current destination's list, Settings at the foot; folds to the 72 rail from the toggle and remembers it | 72 rail; the toggle opens the 264 sidebar as a sheet | 56 bottom bar for the destinations; the toggle raises the new action and the list as a sheet |
| content | `stone2`, top corners cut by the pane cut: a 52 header, then one reading column, centred and at most 760 wide, with the composer standing at its foot | same, the column inset 21 | full width, the column inset 13 |
| side panel | the work beside the conversation, 440 wide, docked while the content keeps 480 | docked or a sheet | a sheet, opened only by the reader |

It takes its size classes and effective dp from the player window: expanded
from 1100 dp, medium from 760. Panes stand 8 apart on the ground with 8 at the
window's sides and foot.

**The sidebar.** From the top: one line plaque that starts something new in
the current destination (a question, a session, a project to add), the
destinations as rows 40 high with a 20 icon and a `label`, then the list that
belongs to the current destination, and Settings under a vein at the foot. The
bone slab travels between destinations. The list is the destination's own:
questions by day, sessions by state, projects with their worktrees. Rows in it
are not tablets. They lie flat on the pane, 36 high for one line and 48 for a
name over one facts line, 2 apart, and take `stone3` under the pointer and the
bone slab when current. The trailing 44 of a row holds what the row claims
(the Working flakes, a count, a time); under the pointer or the list cursor
the claim gives way to the row's one action, as on a tablet.

**A row says a state only when it has one.** A session that needs the reader
carries an amber notch and stands under a Needs you group header; one at work
shows the Working flakes; one at rest shows no mark and no word. A list never
repeats a state word on every row.

**The reading column.** Everything said sits in one column: the question, the
answer, the work in between and the composer share its two edges. A question
is a `stone3` slab cut 7, as wide as its text up to three quarters of the
column, set against the trailing edge. An answer has no slab: `proof` type
straight on the pane, as wide as the column. Blocks of one turn stand 13
apart, turns 34 apart. The column starts 21 under the header. A short exchange
sits at the top of the pane; one longer than the pane opens at its end. The
view follows the end while an answer is still arriving, until the reader
scrolls away. When they have, one small icon plaque above the composer returns
to the end.

**What is said is never cut.** A line of an answer breaks; it does not end in
an ellipsis. An address or a command longer than the column breaks where it
must, code that fits on a line stays whole on one, and a table gives its
narrow columns their width and wraps the wide ones. Code inside a sentence
stands on a `stone3` plate cut 3 with 6 either side of it. A face that has no
italic leans its emphasis rather than drop it.

**Text can be marked.** Every paragraph, question and block of code can be
selected with the pointer and copied, with `sel` at 60% under the marked
words. A second press takes a word, or all of a piece of code, since that is
what gets copied whole. One stretch is marked at a time, and only while the
transcript holds the keyboard: marks that stay lit after the keyboard has
gone look like something a copy would take. The message menu offers the
marked words first: to copy, or to quote in the reply.

**Work between question and answer** is one line, not a card: a 16 icon, what
was done in `label`, the literal part in `monoSm`, and its claim only while it
runs or when it failed. Steps that ran back to back fold into one line with a
count once the last of them has finished. A line unfolds in place to what went
in and what came out, on a `stone1` slab cut 7. While a turn runs and nothing
in the column shows it, the Working flakes stand at the column's foot; a step
that is running already carries them, so the foot never repeats it.

**A request for permission** is a mineral field in amber, cut 7, in the
column: its claim, what is asked in `title`, the literal command or path in
`mono`, then one gold plaque for the narrowest consent, a line plaque for a
wider one and a quiet plaque to refuse. Once answered it folds to one line
that records the answer.
A request can be answered from the composer too, since that is where the
keyboard is: with nothing typed, Enter gives the narrowest consent and Escape
refuses, and the composer's hint line says so for as long as that holds.
Words in the field are a message, so neither key answers then. A key never
answers a request that is off screen, or one that has been on screen for less
than half a second: it was pressed before anyone could have read it.

**The composer** is one field with its controls inside: `stone3`, cut 7, the
text in `proof` with 16 around it, and under the text a row of small plaques,
what to send with on the left (attach, the model, the mode) and the one gold
icon plaque on the right. It stands straight on the pane, one role away, so it
keeps a 1 dp `vein` hairline at rest and the hairline turns `fg2` with the
keyboard (see [what one step of the ramp can carry](palette.md#what-one-step-of-the-ramp-can-carry)).
It is at least 96 high, grows a line at a time to eight lines and then
scrolls. Send is the standing gold action of a conversation window. While a
turn runs the same plaque is Stop, and with text in the field it offers to
queue the message or steer the turn instead.

The keyboard belongs to the composer. It has it when its page opens, and
gets it back when a menu in its own row, the search field or a dialog closes,
and after an action in the transcript such as copying an answer. Marked
text is the one thing in the transcript that takes it, for as long as it is
marked; Escape, or a press that marked nothing, hands it back. Nobody should
have to press the field before they can type again.

**A new conversation** opens on the composer, not on an empty transcript: one
`sentence` that says what can be asked, one `proof` line under it, the
composer 680 wide at the optical centre of the pane, up to four things to
try as quiet small plaques, and under them one line that says what will
answer: a claim, a `caption`, and the way to change it. Sending the first
message moves the composer to the foot of the column.

**The header** is 52 high and holds the name of what is open in `title`, its
facts in `caption`, and its actions as small icon plaques at the trailing
edge, the rare ones behind one overflow plaque. A vein under it appears only
while the column is scrolled beneath it.

**The work beside it.** When a conversation changes files, the side panel
shows the changes without leaving the conversation: a file per row with its
status letter, path and counts in `mono`, each unfolding to its diff. The
panel never takes gold; committing belongs to the full review.

A destination with nothing open shows its Now board in the content pane: the
`sentence`, one `proof` line, then what needs the reader first. A board with
nothing to report does not stand empty. It says where work can start, on
`stone1` slabs under a `title`: the places to work in with the one action
that picks each, or, when the thing every action needs is missing, the way
to get it. A list in the sidebar that has no rows yet says in one `caption`
line what will stand there.

Search has one entry point, the top bar field. It finds conversations,
projects and commands, and Ctrl K focuses it. Its results open under the
field, as wide as the field, and leave with it on Escape.
