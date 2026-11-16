# Motion

Motion in Ryoku answers the hand or a change of state, briefly, at the
display's own refresh rate, and stops. A window at rest may keep a little
life (a clock, a scan across the plates), but only while it is the focused
window and motion is allowed; otherwise it draws nothing. Reduced motion
makes every change instant.

This page is how Ryoku Settings (the Hub) moves. Paths: the Hub is
`ryoku/hub/quickshell/`, the QML library `docs/lithic/library/qml/Lithic/`.
Durations are in milliseconds. Most go through `Lithic.duration()`, which
returns 0 under reduced motion (`Lithic.qml:191-192`); the rest are skipped
or jump under reduced motion as [Reduced motion](#reduced-motion) lists.

## The curves

| Curve | Where | Source |
|---|---|---|
| `Easing.OutCubic` | the Hub's working curve: the tree, page changes, washes, drawers, shades (46 uses) | e.g. `Hub.qml:1004-1022`, `components/nav/NavRow.qml:66-73` |
| **chisel**, `cubic-bezier(0.2, 0, 0, 1)` | the Lithic components: plaques, setting rows, dialogs, placards, `Reveal`, page scroll jumps (9 uses) | `tokens.json` `motion.curve`; `Lithic.curve`; `ChiselColorBehavior.qml`, `ChiselNumberBehavior.qml` |
| `Easing.OutBack` with overshoot | two marks in the tree: the folder chevron (overshoot 1.6) and the outline node of the section being read (overshoot 2) | `components/nav/NavRow.qml:114-116`; `components/nav/NavOutline.qml:88` |
| `Easing.InOutSine` | card sparkles and the badge emblem's light | `components/about/TradingCard.qml:659-693`; `components/badge/Emblem.qml:55-61` |

Both main curves start fast and settle without overshoot. The only overshoot
in the chrome is the two tree marks above; the window animation preview uses
OutBack too, because it shows the compositor's "pop" style
(`components/previews/AnimationPreview.qml:34`).

**No springs.** A value that follows the pointer uses `Chase`
(`components/Chase.qml:4-35`): each frame it closes a share of the gap that
depends only on the time passed, `1 - exp(-rate × frameTime)`. At its rate of
28 per second it closes 90% of a move in about 80 ms. It never overshoots,
looks the same at 60 Hz and 165 Hz, runs only while a gap remains, and jumps
under reduced motion.

## The durations

The Hub's own vocabulary (`ryoku/hub/quickshell/Singletons/Tone.qml:39-43`):

| Name | ms | Used for |
|---|---:|---|
| `Tone.quick` | 120 | hover washes in, ink changes, crossfades, the scroll marker |
| `Tone.move` | 240 | a move: a row's lean, the chevron, a page's fade, the drawer, shades |
| `Tone.unfold` | 260 | a folder or the section outline opening |
| `Tone.stagger` | 18 | between tree leaves sliding in (0 under reduced motion) |

The Lithic tokens the Hub uses (`tokens.json` `motion`; `LithicTokens.qml:37-63`):

| Token | ms | Used for |
|---|---:|---|
| `press` | 90 | a plaque's cut deepening and its content sinking |
| `hoverIn` / `hoverOut` | 160 / 100 | plaque fills, setting rows, search hits, page rows |
| `navigate` | 200 | a section's rise, page scroll jumps |
| `revealStagger` | 40 | between sections rising |
| `dialogIn` / `dialogOut` | 300 / 160 | a dialog rising 12 and dropping 8 |
| `placardIn` | 300 | a placard rising 8 |
| `placardLife` | 5000 | the Undo window and its draining blade |
| `highlight` | 1600 | a found setting row's highlight fading |
| `countUp` | 640 | Home's readings counting up |
| `working`, `sweep` | 900, 1280 | `Working` and indeterminate `BladeProgress`, while a job runs |

## Hover and press

| What | Motion | Source |
|---|---|---|
| tree row | the hover wash fades in over 120 and out over 240, OutCubic; the row leans 2 px inward over 240; the press wash is `Tone.press`; ink changes take 120 | `components/nav/NavRow.qml:62-131` |
| folder chevron | rotates 90 degrees over 240, OutBack 1.6 | `NavRow.qml:104-117` |
| plaque | hover fill 160 in, 100 out; press deepens the cut by 4 and sinks the content 1 px over 90 | `Plaque.qml:63-93` |
| icon button | wash over 120; the glyph scales to 0.85 on press; the close glyph turns 90 degrees on hover over 240 | `components/IconButton.qml:22-39` |
| setting row, search hit, page row | hover plane 160 in, 100 out | `SettingRow.qml:143-146`; `components/SearchResults.qml:153-160`; `components/PageRow.qml:27-34` |
| Home figure, studio card, binder and viewer cards | tilt toward the pointer through `Chase`: up to 9 / 5, 8 / 6 and 7 / 5 degrees | `components/Home.qml`, `components/about/` |
| binder card | a hovered card lifts 8 px over 240, OutCubic | `components/about/Binder.qml:467-469` |

## Page transitions

- **A page change** fades the stage in over 240 and slides it from 14 px to
  its place over 300, both OutCubic; the slide is 0 under reduced motion
  (`Hub.qml:1004-1022`).
- **Sections rise** as a page opens, the first five only: each fades in and
  settles from 8 px below over 200 on the chisel curve, 40 ms after the one
  before (`components/Reveal.qml:4-45`; `components/SettingsPage.qml:773-783`).
  Only opacity and a translate move, so nothing lays out again. A later
  rebuild of the page moves nothing.
- **Home's entrance**: its groups rise the same way, the carved word stamps
  from 1.06 to 1 over 300 on the chisel curve, and the readings count up over
  640.
- **The tree**: a folder unfolds its height over 260 OutCubic; its leaves
  slide in from 8 px to the side, 18 ms apart, over 240
  (`components/nav/NavFolder.qml:72, 111-157`); the selection mark slides over
  240 within a folder, or lands at a new folder by fading in over 240 while
  it widens from 0.6 over 320 (`components/nav/NavTree.qml:213-232`); the
  section outline unfolds over 260 (`components/nav/NavOutline.qml:20-26`).
- **The sheet**: the folio numeral rolls over 480
  (`components/sheet/Folio.qml:38`), a plate develops over 1100
  (`components/sheet/Plate.qml:32`), the Activity map prints in from the left
  over 900, once (`components/sheet/Activity.qml:74-83`), and the masthead's
  scroll marker follows over 120 (`components/sheet/Masthead.qml:156`).
- **The sidebar**: search and the tree crossfade over 120; the tree peek fades
  over 160 and slides over 240 (`components/nav/HubSidebar.qml:105-139,
  196-200`); in a narrow window the drawer slides over 240 with its scrim
  (`Hub.qml:1024-1048`).
- **Overlays**: a dialog rises 12 over 300 and drops 8 over 160
  (`Dialog.qml:269-303`); a placard rises 8 over 300 (`Placard.qml:191-202`);
  the binder, card viewer and certificate shades fade over 240; the binder's
  detail card grows from 0.94 while fading in from 0.5 over 260.
- **The style preview** fades in from 0.35 over 240 when the style it shows
  changes (`pages/Style.qml:119-130`).

## Finite loops

- **The Updates dot** pulses three times when it appears (scale 1 to 1.5 over
  420, back over 520), then rests (`components/nav/NavRow.qml:237-242`).
- **`Working`** cycles every 900 ms and indeterminate **`BladeProgress`**
  sweeps every 1280 ms, only while a job is live.

## Motion at rest

The exact rule the Hub keeps:

1. **Life at rest runs only while Settings is the focused window and motion
   is allowed.** `Ambient.on` is bound to "the window is active and reduce
   motion is off" (`Hub.qml:405-410`). Everything below waits on it.
2. **While on** (`Singletons/Ambient.qml:7-60`):
   - a 1 s timer ticks the masthead's seconds and Home's clock;
   - every 11 s a scan crosses the dithered plates for 2.2 s, stepped at 30
     frames a second (a 33 ms timer) with a cosine ease
     (`components/sheet/Plate.qml:62-63`). Between scans nothing moves, so
     the window draws no frames;
   - on Home, the dust in the light drifts at about 15 frames a second (a
     66 ms timer), only while Home is shown and its plate is roomy
     (`components/Home.qml:426-433`);
   - the sidebar's live figures sample only while the Activity map is shown
     (`components/sheet/Activity.qml:30-37`);
   - Home's system card animates only while the pointer is on it
     (`components/Home.qml:604`).
3. **While off** (unfocused, or reduced motion): no timers and no scan; the
   masthead clock drops its seconds (`components/sheet/Masthead.qml:34`).
4. **Data is not motion.** Home samples its readings every 3 s while focused
   and every 15 s while not, and never once Home is left
   (`components/Home.qml:183`); each sample redraws once.
5. **Cards are still at rest.** Every card effect needs the pointer and
   motion allowed.
6. **One exception:** while the card studio is open on its Badge tab, the
   badge emblem's rim turns, whether or not the window is focused; reduced
   motion stops it (`components/about/CardEditor.qml:323`;
   `components/badge/Emblem.qml:20, 55-61`).

**Frame rates.** Every answer to the hand runs on Qt's animation clock at the
display's refresh rate (165 Hz on a 165 Hz screen), and pointer-follow runs
every frame through `Chase`; none of it is stepped by a timer. Only the
ambient life above is stepped, on purpose: the scan at 30 frames a second
because "a dither reads as stepped anyway" and the window then redraws a few
times a second on average (`Singletons/Ambient.qml:26-28`), the motes at 15,
the clock at 1.

Rendered headless, the window is never active, so `Ambient` stays off and a
render shows the window at rest.

## Reduced motion

Reduce motion has one home, the desktop's `theme.motion.reduce` in
`shell.json`, bound to `Lithic.reduceMotion` (`ShellBody.qml:23-28`). When
it is on:

- `Lithic.duration()` returns 0 and `Tone.stagger` is 0, so every change
  lands at once.
- `Reveal` never arms; `Chase` jumps; the folio swaps at once; a plate starts
  developed; the Activity map shows at once; the Updates dot does not pulse.
- `Ambient` is off; cards and the emblem are still.
- A placard keeps its Undo window on a timer, with its blade held whole
  (`Placard.qml:44-84`).

Layout, roles and copy stay identical.

## Other window shapes

A media or conversation window may move while nothing is touched, because
the motion is the content (music playing, an answer being made). The same
gate holds: it runs only while the cause lasts and motion is allowed, and
stops when the view is hidden. Ryotunes' wave progress, for example, moves
only while a track plays and motion is not reduced, its amplitude at 60
frames a second and its phase at 30 (`ryotunes-gio`
`internal/client/ui/look/wave.go:20-22, 150, 166`).

## Stack notes

**QML.** Pass every duration through `Lithic.duration()`, or use the
`ChiselColorBehavior` and `ChiselNumberBehavior` behaviours. Gate any timer
that runs at rest on the window being active and `!Lithic.reduceMotion`, the
way `Ambient` is gated. Loop `Working` and progress only while live.

**Gio.** Use `anim.Tween` and `anim.Spring` from the Go library: a settled
value requests no frame, and `anim.Reduced()` turns every animation into an
instant change (`library/go/anim/anim.go`). Emit an invalidation only while a
value is moving.

**Web.** Transition only the properties named here, with the chisel curve,
and respect the platform's reduced-motion preference.
