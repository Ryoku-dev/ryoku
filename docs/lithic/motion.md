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

Nothing else moves. Panes do not slide in, lists do not stagger, icons do not
spin and no idle element pulses. Removal feedback is a Placard and its
draining blade; the resource simply leaves the layout.

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
component.
