# Components

Lithic components share four recipes. Units are dp unless noted. Type styles
come from [design.md](design.md#4-type), roles from
[palette.md](palette.md), and window composition from
[layout.md](layout.md). Ryoport file names are the reference.

| Recipe | Rule |
|---|---|
| Hover | Filled controls mix the fill toward white by 0.10. Transparent controls show ink at alpha 0.10. Planes step one tone up. Fade in over 160 ms and out over 100 ms. |
| Press | Plaques deepen the cut by 4 and sink content by 1 over 90 ms. Rows and menu items dip by adding `fg` at alpha 0.08. |
| Focus | Draw a 2 dp `fg2` ring 2 dp outside the control, with the cut grown by 2. Fade in over 120 ms and out over 100 ms. Show it only for keyboard focus. |
| Disabled | Draw the whole control at alpha 0.5 and suppress hover. |

## Foundation

### Icon

Use Material Symbols Sharp ligatures on QML and the web. Gio uses Ryoport's
shiny material icons. Icons inherit the component ink, never carry state by
themselves, and every icon-only action has an accessible description.
Reference: `ui/icons/icons.go` and `ui/kit/osicon.go`.

Library: QML Lithic.Glyph, web .l-glyph, Go kit.Icon, terminal -

### CutRect

Fill or stroke a rectangle with independent 45 degree cuts on its four
corners. A 1 dp stroke is drawn inside. Its inner cut is
`cut - width * (2 - sqrt(2))`, which keeps the stroke even around the
chamfer. Reference: `ui/kit/draw.go`.

Library: QML Lithic.CutRect, web .l-cut, Go kit.FillCut and kit.StrokeCut, terminal -

### Notch

Draw the top-left triangle `(x, y) (x+12, y) (x, y+12)`. A tablet uses it for
state. Settings uses one amber notch for an available update. An icon plaque
may use one for a pending change. Reference: `ui/kit/draw.go`.

Library: QML Lithic.Notch, web .l-notch, Go kit.Notch, terminal term.RenderNotch

### Blade

Draw a bar with a 45 degree leading edge. The trailing top corner is cut by
the bar height. Blades mark progress, meters, tabs and other moving places.
Reference: `ui/kit/draw.go`.

Library: QML Lithic.Blade, web .l-blade, Go kit.Blade, terminal term.RenderBlade

### Flake

A flake is a parallelogram 1.4 by 0.7 of its nominal size, with 45 degree
short sides. Idle has a 1.5 dp `vein` outline. Active fills with
`stone(tone)`. Becoming starts on `vein` and fills with `lapis` to the
progress fraction. A nominal size of 9 produces a flake about 13 wide and 6
high. Reference: `ui/kit/token.go`.

Library: QML Lithic.Flake, web .l-flake, Go kit.Flake, terminal term.RenderFlake

### Hatch

Draw 1 dp `fg` lines at 45 degrees, 6 dp apart and alpha 0.30. It is an
engraved chart field, not a general background texture. Reference:
`ui/kit/chart.go`.

Library: QML Lithic.Hatch, web .l-hatch, Go kit.Hatch, terminal -

### FocusRing

The keyboard focus ring is a 2 dp `fg2` stroke 2 dp outside the target. Grow
the target cut by 2 and fade the ring in over 120 ms, out over 100 ms.
Pointer focus does not show it. Reference: `ui/kit/focus.go`.

Library: QML Lithic.FocusRing, web .l-focus-ring, Go kit.FocusRing, terminal -

## Controls

### Plaque

A plaque is a cut button. Normal height is 40 with 18 horizontal inset, cut
7, icon 19 and an 8 gap. Small height is 32 with 13 inset, cut 6, icon 17 and
a 6 gap. Both use `label` type.

| Kind | Fill | Ink | 1 dp hairline | Use |
|---|---|---|---|---|
| gold | `gold` | `onGold` | none | the one consequential action |
| line | none | `goldInk` | `goldInk` | actions beside the gold one |
| quiet | none | `fg2` | `vein` | view controls, cancel and secondary verbs |
| on | `sel` | `onSel` | none | a selected choice |
| danger | `jasper` | `inkOn(jasper)` | none | the direct destructive action |
| danger line | none | `jasper` | `jasper` | an outlined destructive action |

A busy plaque swallows presses and may show Working. Navigation never uses
gold. Reference: `ui/kit/button.go`.

Library: QML Lithic.Plaque, web .l-plaque, Go kit.Plaque, terminal term.RenderPlaque

### SplitPlaque

The main plaque and a 40 wide more segment sit 2 dp apart and share one
outline. The main segment cuts only its leading corners. The more segment
cuts only its trailing corners. Each has independent hover, press and focus.
The `expand_more` arrow becomes `expand_less` while its menu is open.
Reference: `ui/kit/splitbutton.go`.

Library: QML Lithic.SplitPlaque, web .l-split-plaque, Go kit.SplitPlaque, terminal term.RenderSplitPlaque

### IconPlaque

An icon plaque is square with no label: 40 with a 20 icon, or 32 with an 18
icon when small, cut 6. It accepts every plaque kind and may carry one mineral
notch. Reference: `ui/kit/iconbutton.go`.

Library: QML Lithic.IconPlaque, web .l-icon-plaque, Go kit.IconPlaque, terminal -

### TextField

The field is 44 high, `stone3`, cut 7, with 14 horizontal inset and `body`
text. Placeholder text is `fg3`. An optional label and help line use
`caption`, 4 away. Keyboard focus adds a 1 dp `fg2` stroke inside the cut.
An error turns that stroke and help text `jasper`. Secret fields mask with
`•` and place a 32 quiet reveal plaque 6 from the trailing edge. Reference:
`ui/kit/textfield.go`.

Library: QML Lithic.TextField, web .l-text-field, Go kit.TextField, terminal term.RenderTextField

### Search

Search reserves 42 on the leading side for a 20 `search` icon at 14. Its
default width is 280. A trailing shortcut hint uses `fg3`. Escape clears the
text. When text exists, a 32 quiet clear plaque appears and returns focus
after clearing. Reference: `ui/kit/search.go`.

Library: QML Lithic.Search, web .l-search, Go kit.Search, terminal term.RenderSearch

### Toggle

The track is 40 by 22, cut 4, with a 14 knob cut 2 at a 4 inset. Off uses a
`stone4` track with a 1 dp `vein` hairline and `fg2` knob. On uses `sel` and
`onSel`. Hover adds `fg` at alpha 0.06. The knob glides for 240 ms and the
tone changes over 320 ms. The label is 8 away in `label` type. Space toggles
it. Reference: `ui/kit/toggle.go`.

Library: QML Lithic.Toggle, web .l-toggle, Go kit.Toggle, terminal term.RenderToggle

### Slider

The slider is 32 high and 160 wide by default. Its `stone3` track is 6 high,
the completed part is an `fg2` blade, and the `sel` handle is 8 by 20 with
cut `min(width/2, height/4)`. Reference: `ui/kit/slider.go`.

Library: QML Lithic.Slider, web .l-slider, Go kit.Slider, terminal -

### Stepper

Two small quiet icon plaques, `remove` and `add`, sit 8 either side of the
value. The unit follows 4 later in `fg3`. Disable an end plaque at its bound.
Reference: `ui/kit/stepper.go`.

Library: QML Lithic.Stepper, web .l-stepper, Go kit.Stepper, terminal term.RenderStepper

### Filters

Filter chips are 32 high, 13 inset, cut 6 and 4 apart in `label` type. A
resting chip is transparent with `fg2` ink and a `vein` hairline. One `sel`
slab glides between selected chips over 240 ms. A count follows the label by
6 in `monoSm`. Reference: `ui/kit/filters.go`.

Library: QML Lithic.Filters, web .l-filters, Go kit.Filters, terminal term.RenderFilters

### Tabs

Tabs use a 44 high strip above a 1 dp `vein` baseline. Each label is its text
width plus 24. The current label is `fg`; other labels are `fg3`, fading to
`fg2` on hover. A 3 dp `fg` blade moves and resizes over 240 ms. Left and
Right move focus and selection. Keep at most five tabs per resource kind.
Reference: `ui/kit/tabs.go`.

Library: QML Lithic.Tabs, web .l-tabs, Go kit.Tabs, terminal term.RenderTabs

### Disclosure

Disclosure grows optional content to its measured height and fades it in over
200 ms. Closing content is inert. Open it with a quiet plaque or a toggle,
never a bare chevron. Reference: `ui/kit/disclosure.go`.

Library: QML Lithic.Disclosure, web .l-disclosure, Go kit.Disclosure, terminal term.RenderDisclosure

## Lists, containers and state

### Tablet

A resource row is a `stone2` tablet, 56 high or 48 at compact density,
square apart from its 12 notch. Content starts at 16: a 32 mark, 13 gap,
title in `row` over one facts line, then a trailing area at least 72 wide
with claim over proof and 12 trailing inset.

Hover or list focus fades to `stone3` and replaces claim and proof with the
row's one action. Selected ink is `onSel` and quiet selected ink `onSel2`.
Becoming adds a 2 dp `lapis` blade along the bottom. Click selects and double
click runs the primary action. Reference: `ui/kit/row.go`.

Library: QML Lithic.Tablet, web .l-tablet, Go kit.Tablet, terminal term.RenderTablet

### RowList

Rows sit 4 apart. One `sel` slab, drawn behind the selected row, glides to its
new row over 240 ms and follows it while scrolling. The scroll thumb is a 3
wide `vein` blade with a 12 minimum. A wheel notch glides 120 dp over 140 ms.
Trackpad deltas follow the fingers at the screen scale. Reference:
`ui/kit/rowlist.go` and `scroll.go`.

Library: QML Lithic.RowList, web .l-row-list, Go kit.RowList, terminal term.RenderRowList

### GroupHeader

The title uses `group` and the count `monoSm`, both `fg3`. Insets are 14 top,
6 bottom, 6 left and 4 right. Fleet groups appear only when populated and
run Needs you, Pinned, Running, Becoming, Stopped. Reference:
`ui/kit/groupheader.go`.

Library: QML Lithic.GroupHeader, web .l-group-header, Go kit.GroupHeader, terminal term.RenderGroupHeader

### Skeleton

Skeletons are static `stone2` tablets 4 apart with two `stone3` bars, each 8
high and 8 apart, at 60 and 35 percent of the width. Reference:
`ui/kit/skeleton.go`.

Library: QML Lithic.Skeleton, web .l-skeleton, Go kit.Skeleton, terminal -

### Empty

Centre a 72 octagonal `vein` frame with a 30 `fg3` icon, then a `title`, a
`body` no wider than 420, and plaques 8 apart. The copy names the next step.
Reference: `ui/kit/empty.go`.

Library: QML Lithic.Empty, web .l-empty, Go kit.Empty, terminal term.RenderEmpty

### Pane

A pane is one flat plane with its two top corners cut 16 and bottom corners
square. Rail and list panes use `stone1`; the work pane uses `stone2`. Panes
start 10 below the window edge and sit 8 apart. Reference:
`ui/app/layout.go`.

Library: QML Lithic.Pane, web .l-pane, Go kit.Pane, terminal -

### MineralField

The selected resource header sits on `field(tone)`: `stone2` mixed 0.17
toward jade, 0.15 toward lapis, amber or jasper, and `stone3` for idle. State
changes fade over 320 ms. Palette changes snap. Reference: `ui/kit/field.go`.

Library: QML Lithic.MineralField, web .l-mineral-field, Go kit.MineralField, terminal term.RenderMineralField

### Header

Header insets are 30 top, 36 sides and 26 bottom. A 44 mark and the name sit
13 apart. The name uses `name`, or `sentence` below 640 width. Proof follows
8 below, note 4 below, marks 4 below, acts 21 below, and optional content 21
below that. Acts are one gold split plaque, line plaques and a quiet overflow
plaque. Reference: `ui/pages/common/header.go`.

Library: QML Lithic.Header, web .l-header, Go kit.Header, terminal -

### Slab

A slab is a `stone1` container cut 7 with a 21 inset. It groups settings,
forms and quiet board sections without adding depth effects. Reference:
`ui/kit/draw.go`.

Library: QML Lithic.Slab, web .l-slab, Go kit.Slab, terminal term.RenderSlab

### Section

A section places a `section` title 13 above its body and leaves 21 below the
slab. Major full-page regions use 55 between them. Reference:
`ui/pages/settings/view.go`.

Library: QML Lithic.Section, web .l-section, Go kit.Section, terminal term.RenderSection

### Facts

Facts are 44 high label and value rows separated by 1 dp `vein` lines. Labels
use `fg3` in a 140 column. Values use `fg`, switching to mono only for literal
data. An absent value is `-`. Reference: `ui/kit/facts.go`.

Library: QML Lithic.Facts, web .l-facts, Go kit.Facts, terminal term.RenderFacts

### Aside

The draft aside is a quiet `stone1` slab cut 7 with 21 inset. Its `title`
names what saving will make or write. Facts and literal previews update as
the form changes; preview text uses `monoSm` on an inner `stone2` slab cut 6
with 13 inset. Reference: `ui/pages/common/draft.go`.

Library: QML Lithic.Aside, web .l-aside, Go kit.Aside, terminal term.RenderAside

### DraftColumns

Keep a draft form at no more than 600 wide. Below 820 available width, its
aside follows the form and is no wider than 600. From 820, the aside sits 34
to the right, takes 36 percent of the available width and clamps from 300 to
400. A missing aside leaves the form alone at reading width. Reference:
`ui/pages/common/draft.go`.

Library: QML Lithic.DraftColumns, web .l-draft-columns, Go kit.DraftColumns, terminal term.RenderDraftColumns

### Claim

A claim is a Flake beside a plain state word in `label`. The word uses the
tone's mineral, or `fg2` for idle. Pair it with the proof that supports the
claim. Reference: `ui/kit/token.go`.

Library: QML Lithic.Claim, web .l-claim, Go kit.Claim, terminal term.RenderClaim

### Marks

Marks are resource traits in `mono`, separated by ` · `. Disposable is amber
and always first, followed by Instant and Sealed. Drop whole trailing marks
before ellipsising facts. Reference: `ui/pages/common/marks.go`.

Library: QML Lithic.Marks, web .l-marks, Go kit.Marks, terminal term.RenderMarks

### Sentence

Sentence composes spans without turning the whole line into data type. A span
may switch to mono for literal data or use amber for an attention name.
Reference: `ui/pages/common/sentence.go`.

Library: QML Lithic.Sentence, web .l-sentence, Go kit.Sentence, terminal term.RenderSentence

## Progress

### BladeMeter

Use a 6 high `stone3` track with a blade fill in the caller's role. Host CPU,
memory and disk meters use quiet `fg2`, turning amber at 90 percent.
Reference: `ui/kit/blade.go`.

Library: QML Lithic.BladeMeter, web .l-blade-meter, Go kit.BladeMeter, terminal term.RenderBladeMeter

### BladeProgress

Job progress uses a `lapis` blade and adds a 2 wide `gold` marker at the
leading edge while active. Indeterminate progress sweeps a quarter-width
segment over 1280 ms and becomes static under reduced motion. Reference:
`ui/kit/blade.go`.

Library: QML Lithic.BladeProgress, web .l-blade-progress, Go kit.BladeProgress, terminal term.RenderBladeProgress

### Working

Three flakes, each `size/5` wide and `size/2` high with `size/10` gaps, fill
with `lapis` in turn over 900 ms on a `vein` base at alpha 0.55. Reduced
motion freezes them full. Reference: `ui/kit/working.go`.

Library: QML Lithic.Working, web .l-working, Go kit.Working, terminal term.RenderWorking

### BecomingName

A machine being made shows its name as a 1 dp `fg` outline filled from the
left to the job fraction. Its stage stays on a BladeProgress and its latest
lines use mono. Reference: `ui/kit/text.go` and
`ui/pages/common/header.go`.

Library: QML Lithic.BecomingName, web .l-becoming-name, Go kit.BecomingName, terminal -

## Overlays and navigation

### Popover

Popover anchors content 8 from its control, below when space permits, above
otherwise, and clamps it to the window. Its own full-window catcher swallows
an outside press before closing it. It enters over 160 ms while sliding the
last 6, and exits over 100 ms while sliding 4 back. Closing content is inert
and blocks underlying pointer input. Only one popover is open. Reference:
`ui/kit/popover.go`.

Library: QML Lithic.Popover, web .l-popover, Go kit.Popover, terminal -

### Menu

A menu is `stone4`, cut 8, 220 to 360 wide, with 6 vertical padding. Items
are 40 high with 16 inset, a 20 icon, 12 gap, `label` text, and shortcut 13
after in `fg3`. Hover mixes `fg` into `stone4` at alpha 0.06 because nothing
is above the top plane; it never steps down to `stone3`. Press mixes `fg` into
`stone4` at alpha 0.08. Danger ink is `jasper`; disabled ink `fg3`. Dividers
are 1 dp `vein` with 8 inset. Reference: `ui/kit/menu.go`.

Library: QML Lithic.Menu, web .l-menu, Go kit.Menu, terminal term.RenderMenu

### Picker

The 44 high `stone3` anchor is cut 7, at least 160 wide and inset 14. Its
field label uses `caption`, its value uses `body`, and its 20
`expand_more` arrow uses `fg2`. Hover adds `fg` at alpha 0.06. It opens Menu
inside Popover. Reference: `ui/kit/picker.go`.

Library: QML Lithic.Picker, web .l-picker, Go kit.Picker, terminal term.RenderPicker

### Dialog

Use Dialog for a protected decision or a focused catalogue choice, not a
draft. The `stone4` slab is cut 16 with a 1 dp `vein` hairline, at least 16
from the window, with 34 padding. Width defaults to 520 and is configurable;
the catalogue system dialog uses 720. An optional title uses `sentence` with
a 21 gap. Actions follow after 34, flow from the start edge and sit 8 apart.

The full-window scrim catches and swallows outside presses. The dialog enters
over 300 ms while rising 12 and exits over 160 ms while dropping 8. It blocks
pointer input beneath until the exit ends. Reference: `ui/kit/dialog.go`.

Library: QML Lithic.Dialog, web .l-dialog, Go kit.Dialog, terminal term.RenderDialog

### Confirm

Confirm composes Dialog with body, Cancel and a danger or gold action. It may
require the resource name in a TextField 21 below the body before enabling
the action. The action names the verb, never Yes. Reference:
`ui/kit/confirm.go`.

Library: QML Lithic.Confirm, web .l-confirm, Go kit.Confirm, terminal term.RenderConfirm

### Tooltip

After 600 ms of pointer hover, show a `stone4` slab cut 4 with a 1 dp `vein`
hairline, 6 by 8 padding and `caption` text, no wider than 320. It sits 8 from
the anchor and enters over 300 ms with an 8 slide. Reference:
`ui/kit/tooltip.go`.

Library: QML Lithic.Tooltip, web .l-tooltip, Go kit.Tooltip, terminal -

### Placard

A placard is a `sel` slab 48 high, cut 8, with a 1 dp `vein` hairline. It is
no wider than 560 and sits 24 above the bottom edge. Up to three stack 8
apart. Text uses `onSel` with 16 inset. An optional action such as Undo is a
32 high `onSel` plaque. A 2 dp `onSel` blade drains across its 5 s life. It
enters over 300 ms rising 8.
Reference: `ui/kit/placard.go`.

Library: QML Lithic.Placard, web .l-placard, Go kit.Placard, terminal term.RenderPlacard

### Rail

Rail owns Home, Fleet, New and Activity, with Settings at the bottom.
Destination icons are 21. Expanded tiles are 64 in an 88 rail; medium tiles
are 56 in a 72 rail. The current `sel` slab glides over 240 ms. Compact moves
the same destinations into five equal columns in a 64 high bottom bar with
48 high tiles. Reference: `ui/app/rail.go`.

Library: QML Lithic.Rail, web .l-rail, Go kit.Rail, terminal -

### BrandMark

The mark is Home. It is a `sel` square cut at 22 percent of its size with an
`onSel` octagon at 28 percent apothem, a 1.5 dp outline and centre point, plus
a small jasper pointer. It is not page decoration. Reference:
`ui/kit/brand.go`.

Library: QML Lithic.BrandMark, web .l-brand-mark, Go kit.BrandMark, terminal -

### NewPlaque

New is the rail's only gold plaque. It is 62 by 52 expanded and 48 by 40
compact, with icon 21 or 20 and cut 7 or 6. The `add` icon becomes `close`
while its menu is open. Reference: `ui/kit/newplaque.go`.

Library: QML Lithic.NewPlaque, web .l-new-plaque, Go kit.NewPlaque, terminal -

### SectionIndex

On an expanded Settings page, the index is 200 wide and 34 from the 760
content column. Rows are 48 high, 4 apart, with labels inset 13. Its `sel`
row follows the section holding the top third of the view. At medium width it
becomes filter chips above the column; compact omits it. Reference:
`ui/pages/settings/view.go`.

Library: QML Lithic.SectionIndex, web .l-section-index, Go kit.SectionIndex, terminal -

## Data and frame

### EngravedChart

Draw a 1.5 dp `fg` polyline over Hatch, with a 1 dp `vein` baseline, 4 dp
ticks and `monoSm` labels. Do not use an area fill, extra colour or gradient.
Short or absent data shows an `fg3` sentence instead. Reference:
`ui/kit/chart.go`.

Library: QML Lithic.EngravedChart, web .l-engraved-chart, Go kit.EngravedChart, terminal -

### AppFrame

AppFrame fills the window with `ground`, selects expanded, medium or compact
at 1280 and 760, and composes Rail, optional list Pane, work Pane, overlays
and window controls. It owns the compact bottom bar and keeps overlay order
and Escape behavior consistent. Reference: `ui/app/layout.go`, `rail.go`,
`overlays.go` and `window.go`.

Library: QML Lithic.AppFrame, web .l-app-frame, Go kit.AppFrame, terminal -
