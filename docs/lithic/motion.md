# Motion

Lithic motion is knapping: one curve, a short duration table, no bounce and
nothing moving at rest. Every animation answers an input or a state change.
Reduced motion makes the same change instant.

Reference: `ui/anim/anim.go`, `ui/theme/theme.go`, `ui/kit/focus.go`,
`popover.go`, `dialog.go`, `disclosure.go`, `placard.go` and `scroll.go` in
Ryoport.

## The curve

**Chisel** is `cubic-bezier(.2, 0, 0, 1)`. It starts decisively and settles
without overshoot. It is the only tween curve. Solve it numerically, as
Ryoport does with 24 bisections, or use the platform's matching bezier.

Continuously changing values are the exception to fixed-duration tweens.
Blade edges use a critically damped spring so new samples ease from the
current edge instead of restarting a tween. Use `omega = 4 / 240 ms` and a
rest threshold of 0.001.

## The durations

| Name | ms | Used by |
|---|---:|---|
| press | 90 | plaque cut and content sink; row and menu-item dip |
| hover in | 160 | a control or plane fading to its hover tone |
| hover out | 100 | hover tone fading back |
| focus in | 120 | keyboard focus ring appearing |
| focus out | 100 | keyboard focus ring leaving |
| select | 240 | row and filter selection slabs, tab blade, rail tile, toggle knob |
| field | 320 | mineral field and toggle tone |
| notch | 200 | a state notch |
| unfold | 200 | Disclosure height and content opacity |
| navigate | 200 | new work-pane content fading in while rising 8 dp |
| menu in | 160 | Popover fading in and sliding the last 6 dp |
| menu out | 100 | Popover fading out and sliding back 4 dp |
| dialog in | 300 | Dialog rising 12 dp over its scrim |
| dialog out | 160 | Dialog dropping 8 dp |
| tooltip in | 300 | Tooltip sliding 8 dp |
| tooltip delay | 600 | delay before Tooltip appears |
| placard in | 300 | Placard rising 8 dp |
| placard life | 5000 | Undo window and its draining blade |
| wheel | 140 | one mouse-wheel notch gliding 120 dp |
| working | 900 | three-flake Working cycle |
| sweep | 1280 | indeterminate BladeProgress period |

## What moves and what it says

| Motion | Meaning |
|---|---|
| Plaque corners deepen 4 dp and content sinks 1 dp | the control was pressed |
| Row and menu item dip by adding `fg` at alpha 0.08 | the item was pressed |
| Hover tone fades in over 160 ms and out over 100 ms | the pointer found an action |
| A row's claim gives way to its one action | this is what Enter will do |
| Row and filter selection slabs glide; the tab blade and rail tile follow | this is the current place |
| New work-pane content fades in while rising 8 dp over 200 ms | navigation completed |
| Disclosure unfolds over 200 ms | more content is available |
| The focus ring fades in over 120 ms | keyboard focus is here |
| Popover closes the last 6 dp to its anchor and retreats 4 dp | it came from that control |
| Dialog rises 12 dp and drops 8 dp over its scrim | a protected choice opened and closed |
| Placard rises 8 dp and its blade drains over 5 s | an outcome can still be undone |
| BecomingName fills and Blade edges follow a spring | work is progressing |
| A wheel notch glides 120 dp over 140 ms | discrete wheel input moved the view |
| Trackpad deltas follow the fingers at the screen's scale | continuous input stays direct |

Outside a media window's playback motion below, nothing else moves. Panes do
not slide in, lists do not stagger, icons do not spin and no idle element
pulses. Removal feedback is a Placard and its draining blade; the resource
simply leaves the layout.

## Playback

A media window answers playback with motion of its own. Each still answers a
cause and stops.

| Motion | Meaning | Duration |
|---|---|---|
| Play and pause swap, the incoming icon growing from .85 | playback started or stopped | notch 200 |
| The liked mark fills from its centre | the track was kept | notch 200 |
| Art crossfades while title and artists rise 8 into place | the track changed | navigate 200 |
| Progress shows its blade handle under the pointer | the pointer can seek here | hover in 160 |
| A card's play plate rises 4 into view | this card plays from here | hover in 160 |
| The source switcher's plate glides | the source changed | select 240 |
| The accent and the window tint retint together across play, progress, marks, fields and stones | a new cover, source or palette | field 320 |
| A new cover is revealed behind a 45 degree cut sweeping from its top-left corner | the track changed | navigate 200 in small art, field 320 in large |
| The side panel's width unfolds | the panel opened | unfold 200 |
| The sidebar's width folds to its rail, labels clipping and fading | the listener wants the room | unfold 200 |
| Every pane re-cuts its corners | the corner setting changed | select 240 |
| The active lyric line grows from its resting size while the last one settles back | the song reached this line | spring, field 320 |
| The word being sung fills behind a 45 degree cut and lifts 2 | this word is being sung | continuous while it fills |
| The played part of the progress is a travelling zigzag that flattens on pause | music is playing | field 320, then continuous |
| The visualizer moves with the music in headers, the now playing view and Home's card, kicking on the beat | this is what you hear | springs, every frame |
| One visualizer style crossfades into the next | the listener changed the style | field 320 |

Three things move while nothing is touched, and only while music plays: the
wave progress, visible visualizers and the word being sung in visible lyrics.
They are the music, not decoration, so they run at up to 60 frames a second
(30 for the word fill). They stop the moment playback pauses, the window hides
them or motion is reduced, and the window then asks for no frames.

**The wave.** Android's wavy progress, chiselled: a triangle wave with sharp
vertices, never a sine. Draw it as one filled polygon by shifting the zigzag's
points up and down by half the thickness times `√(1 + slope²)`, which keeps the
perpendicular thickness and mitres every vertex. Cut it vertically at the
played position. The unplayed part is a flat bar a small gap after it. Pausing
or dragging eases the amplitude to zero, so the bar is flat and exact.

**The visualizer.** Blades are vertical, square at the bottom with the
top-right corner cut at 45 degrees; each follows its band with a spring that
rises fast and falls slowly, the way a needle does, and a flat peak cap holds
above it for a moment before it falls. A curve is one line through the same
springs with the area under it filled faintly, its ends square. A waveform is
triggered at a rising zero crossing so a steady tone stands still. The stereo
field plots left against right turned 45 degrees, so mono stands upright,
inside a diamond. Levels are shaped so quiet bands sink and loud ones stand
out, loud blades paint brighter, and a beat detector on the bass kicks every
style taller and brighter for a moment. Each drawing is a handful of paths,
one paint each. Visualizers sit in their own lane, never under text, and
flatten or fall to nothing when the music pauses.

**Sung words.** The word being sung fills with the accent from its left edge
behind a 45 degree cut that follows the clock, and lifts a little; sung words
stay lit, unsung ones wait in a quieter ink. Real word timing is used when the
lyrics carry it; otherwise the line's timing is shared across its words by
length, never stretched across an instrumental gap.

## Overlay motion

Popover has an 8 dp anchor gap. Its own full-window outside-press catcher
swallows the press before closing it. While it exits, a blocker prevents the
same press or a later release from reaching content beneath it.

Dialog uses a full-window scrim that also catches and swallows presses. It
accepts a caller width and optional title. The visual and scrim fade together;
pointer input beneath stays blocked until the 160 ms exit ends.

Clicks caught during Layout are latched for the consumer and request one more
frame. This makes a tab, menu item or other control apply the click even when
the consumer asks after layout, without waiting for another pointer event.

## Reduced motion

Read both the desktop setting and the app setting. If either requests reduced
motion, tweens return the target, springs snap, wheel movement applies at
once, and looping indicators freeze at a legible state. Layout, roles and
copy stay identical.

## Drawing nothing at rest

An idle Lithic window requests no frames:

1. A tween stores start, target, start time and duration. It requests another
   frame only while progress is below 1.
2. Setting a tween to its current target is a no-op.
3. A spring requests frames only while position or velocity is outside its
   rest threshold.
4. Working and indeterminate progress request frames only while live and
   never under reduced motion.
5. Hover and focus settle after their one transition.
6. Background data asks for one redraw when a new sample arrives.
7. A palette change under the same tone snaps. The 320 ms field fade belongs
   to state changes.

With the pointer still and no live work, the frame counter does not advance.

## Stack notes

**Web.** Transition only the properties named above with the shared chisel
curve. Register the cut as a length so the chamfer can transition. Respect
the platform reduced-motion preference.

**QML.** Use the shared chisel behaviors and duration tokens. Loop Working
and indeterminate progress only while live.

**Gio.** Use the shared Tween and Spring contracts. Emit an invalidation only
while a value is moving. Use the latched Click wrapper for every clickable
component. Gio does not report whether a scroll came from a wheel or a
trackpad, so apply every delta at once at the screen's scale; never guess a
wheel from the size of a delta, because a trackpad stream can send the same
value and would then trail the fingers.
