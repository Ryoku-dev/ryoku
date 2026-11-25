# Oni animation spec (frozen numbers)

This is the distributed copy of the numbers in `internal/oni/spec/oni-spec.json`. The two must agree; `internal/oni/spec` has a test that parses every table below and fails on any difference in either direction.

How to read a table: the first header cell is a JSON path prefix (`root` is the top level). In a two-column table headed `value`, each row is `prefix.row`. In a wider table, each cell is `prefix.row.column`. Arrays are written `[a, b]`. Springs are `[stiffness, damping]` with the integrator mass; ranges are `[min, max]`; eases are CSS `cubic-bezier(x1, y1, x2, y2)`; hover is `[amplitude px, rate, breath]`; notes are MIDI numbers.

Units follow the key name: `S` seconds, `Ms` milliseconds, `Px` CSS pixels, `PxS` px per second, `Hz`, `Fps`, `Db` decibels, `Em`. State `spin` is rad/s; `strand.burst.anglePi` is a multiple of pi. Unsuffixed keys are unitless weights or engine-native units.

Activities for frame tiers: `asleep` (sleeping pose), `idle` (settled, nothing open), `breathing` (an awake state with only hover and breath), `motion` (gestures, strand, flight, drag). Battery saver uses `frames.batteryTier`.

Asleep stops after the commit (`frames.asleepStopsAfterCommit`): this is a deliberate departure from the reference behaviour, which keeps a hover-only loop at the asleep rate. Once the sleeping pose commits (`sleep.commitS` after entering `sleeping`), the renderer paints that pose once and arms no timer until the next state, gesture or drag. The asleep tier therefore only paces the commit window.

Gesture keyframes are original to Hyakki. Each `gestures.keyframes.<id>` table lists `at`, the progress of each key in [0, 1], and one row per offset pose channel (`scale`, `squash`, `leanX`, `leanY`, `unravel`, `straighten`, `tempo`, `bright`) with one delta per key. Every channel starts and ends at 0, so a gesture always returns to the state pose. Segments ease with smoothstep. A replaced gesture hands its residual offset off linearly over `gestures.handoffMs`. `poke` is authored as a poke from the left and mirrors `leanX` when the pointer is on the right. The interior `error.leanX` keys are `gestures.errorShakeLeanX`.

Placement: a pinned character sits `placement.cornerMarginPx` from its corner and is clamped at least that far inside the output; a press becomes a drag after `placement.dragThresholdPx`; `placement.sizes` scale the character.

## root

| root | value |
|---|---|
| version | 1 |

## character

| character | value |
|---|---|
| buttonPx | 72 |
| shellPx | 76 |
| framePx | 180 |
| radiusPx | 32 |
| tubePx | 6.6 |
| samples | 180 |
| supersample | 2 |
| dprCap | 2 |
| wall | 1.06 |
| thickThin | 0.09 |
| outlinePx | 2.8 |
| runArc | 8 |
| overlapInk | 0.4 |
| overlapWhite | 1.2 |
| ink | #0D0F19 |
| white | #F5F5F7 |

## character.trefoil

| character.trefoil | value |
|---|---|
| fov | 6.2 |
| depth | 1.6 |

## character.fallbackS

| character.fallbackS | value |
|---|---|
| flowA | 8.8 |
| flowB | 7.4 |
| glint | 4.6 |
| orbitA | 18 |
| orbitB | 13 |
| core | 3.2 |
| halo | 4.8 |

## poses.flat

| poses.flat | value |
|---|---|
| tilt | 1.15 |
| squash | 0.1 |
| thin | 0.6 |

## poses.loose

| poses.loose | value |
|---|---|
| lobe | 1 |
| toward | [0.8, -0.6] |
| stem | 2.6 |
| width | 0.58 |
| tipWidth | 0.457 |
| edge | 0.12 |
| restPx | 40 |
| reachPx | 56 |
| flightPx | 50 |
| micPulse | 0.05 |
| threadSpeedPxS | 300 |
| threadMinS | 0.2 |

## springs.shape

| springs.shape | value |
|---|---|
| flat | [120, 21] |
| face | [120, 20] |
| loose | [130, 22] |
| tail | [140, 21] |
| mirror | [60, 13] |
| aim | [120, 17] |
| slack | [70, 15] |
| stretch | [220, 24] |
| lagX | [85, 8.5] |
| lagY | [85, 8.5] |
| ripple | [70, 13] |

## springs.pose

| springs.pose | value |
|---|---|
| scale | [260, 28] |
| squash | [220, 25] |
| leanX | [180, 24] |
| leanY | [180, 24] |
| unravel | [200, 27] |
| straighten | [190, 26] |
| tempo | [100, 20] |
| bright | [160, 23] |

## springs.poseRanges

| springs.poseRanges | value |
|---|---|
| scale | [0.7, 1.3] |
| squash | [-0.3, 0.3] |
| lean | [-0.35, 0.35] |
| unravel | [0, 1] |
| tempo | [0.2, 2] |
| bright | [0.3, 1.8] |

## states

| states | flat | face | loose | spin | hover | scale | tempo |
|---|---|---|---|---|---|---|---|
| idle | 1 | 0 | 0 | 0 | [2.6, 1.35, 0.012] | 0.86 | 1 |
| ready | 0 | 0 | 1 | 0 | [1.2, 1.6, 0.006] | 1 | 1.1 |
| listening | 0 | 0 | 1 | 0 | [0.8, 2, 0.004] | 1.02 | 1.2 |
| working | 0 | 1 | 0 | 1.1 | [0.8, 1.8, 0.004] | 0.96 | 1.2 |
| reading | 0 | 1 | 0 | 0 | [0.8, 1.6, 0.004] | 0.96 | 1 |
| thinking | 0 | 1 | 0 | 1.6 | [0.8, 1.8, 0.004] | 0.96 | 1.4 |
| speaking | 0 | 0 | 1 | 0 | [0.8, 1.6, 0.004] | 1 | 1.12 |
| error | 0 | 0 | 1 | 0 | [0.8, 1.6, 0.004] | 1 | 0.9 |
| sleeping | 1 | 0 | 0 | 0 | [2, 0.85, 0.012] | 0.84 | 0.25 |

## motion

| motion | value |
|---|---|
| dragTailAt | 480 |

## motion.spinStop

| motion.spinStop | value |
|---|---|
| spring | [90, 17] |
| lead | 0.2 |
| waitFor | 0.55 |
| waitEase | 9 |
| divisions | 3 |

## motion.stretch

| motion.stretch | value |
|---|---|
| per | 0.00016 |
| max | 0.16 |

## motion.lag

| motion.lag | value |
|---|---|
| per | 0.03 |
| max | 18 |
| reach | 44 |

## motion.ripple

| motion.ripple | value |
|---|---|
| amp | 1.8 |
| waves | 5 |
| waveRate | 13 |
| fullSpeedPxS | 800 |

## motion.whip

| motion.whip | value |
|---|---|
| amp | 5 |
| waves | 0.9 |

## motion.hoverFloat

| motion.hoverFloat | value |
|---|---|
| secondary | 0.18 |
| secondaryPhase | 1 |
| quantizePx | 0.25 |

## motion.leans

| motion.leans | value |
|---|---|
| reading | 0.18 |
| pointing | 0.14 |
| listening | 0.035 |
| listeningMinPx | 10 |
| fileHover | 0.15 |
| dragPerSpeed | 1400 |
| dragMax | 0.22 |

## motion.speaking

| motion.speaking | value |
|---|---|
| rateA | 8.2 |
| rateB | 3.1 |
| gain | 0.7 |
| scale | 0.035 |
| squash | -0.025 |
| bright | 0.1 |

## motion.mic

| motion.mic | value |
|---|---|
| bright | 0.1 |
| band | 0.5 |

## colour.palette

| colour.palette | value |
|---|---|
| idle | #F5F5F7 |
| listening | #C58AFF |
| thinking | #FFC76A |
| speaking | #59B9FF |
| done | #72DDA8 |
| error | #FF706B |

## colour.stateColour

| colour.stateColour | value |
|---|---|
| idle | idle |
| ready | idle |
| listening | listening |
| working | thinking |
| reading | speaking |
| thinking | thinking |
| speaking | speaking |
| error | error |
| sleeping | idle |

## colour.lap

| colour.lap | value |
|---|---|
| durationS | 0.5 |
| head | 0.1 |
| headWidth | 0.78 |
| core | 0.46 |
| arriveS | 0.08 |
| leaveS | 0.28 |

## colour.glow

| colour.glow | value |
|---|---|
| spread | 13 |
| strength | 0.85 |
| low | 0.6 |
| high | 1 |
| periodS | 2.6 |

## colour.mic

| colour.mic | value |
|---|---|
| floor | 0.78 |
| width | 0.3 |
| glow | 0.2 |
| ease | 14 |

## colour.beats

| colour.beats | colour | holdS |
|---|---|---|
| done | done | 0.8 |
| error | error | 0.8 |

## colour.focusGuard

| colour.focusGuard | value |
|---|---|
| active | #F2735E |
| done | #5BC48C |

## gestures

| gestures | value |
|---|---|
| handoffMs | 70 |
| errorShakeLeanX | [-0.16, 0.12, -0.05] |

## gestures.durationS

| gestures.durationS | value |
|---|---|
| acknowledge | 0.28 |
| done | 0.56 |
| error | 0.46 |
| poke | 0.38 |
| nudge | 0.68 |
| wake | 0.82 |
| sleep | 1.5 |
| arrive | 0.3 |
| gulp | 0.62 |
| settle | 0.52 |

## gestures.keyframes.acknowledge

| gestures.keyframes.acknowledge | value |
|---|---|
| at | [0, 0.3, 0.65, 1] |
| scale | [0, 0.05, -0.01, 0] |
| squash | [0, -0.06, 0.03, 0] |
| leanY | [0, 0.08, -0.02, 0] |
| bright | [0, 0.15, 0.05, 0] |

## gestures.keyframes.done

| gestures.keyframes.done | value |
|---|---|
| at | [0, 0.2, 0.45, 0.7, 1] |
| scale | [0, -0.03, 0.07, 0.02, 0] |
| squash | [0, 0.08, -0.1, 0.03, 0] |
| straighten | [0, 0, 0.3, 0.1, 0] |
| bright | [0, 0.1, 0.3, 0.12, 0] |

## gestures.keyframes.error

| gestures.keyframes.error | value |
|---|---|
| at | [0, 0.25, 0.55, 0.8, 1] |
| scale | [0, -0.03, -0.02, -0.01, 0] |
| squash | [0, 0.06, 0.04, 0.02, 0] |
| leanX | [0, -0.16, 0.12, -0.05, 0] |
| bright | [0, -0.12, -0.08, -0.03, 0] |

## gestures.keyframes.poke

| gestures.keyframes.poke | value |
|---|---|
| at | [0, 0.18, 0.5, 1] |
| scale | [0, -0.05, 0.02, 0] |
| squash | [0, 0.12, -0.04, 0] |
| leanX | [0, 0.2, -0.06, 0] |

## gestures.keyframes.nudge

| gestures.keyframes.nudge | value |
|---|---|
| at | [0, 0.25, 0.5, 0.75, 1] |
| scale | [0, 0.04, 0, 0.02, 0] |
| leanX | [0, 0.05, -0.05, 0.02, 0] |
| leanY | [0, -0.06, 0.04, -0.02, 0] |
| bright | [0, 0.12, 0.04, 0.08, 0] |

## gestures.keyframes.wake

| gestures.keyframes.wake | value |
|---|---|
| at | [0, 0.3, 0.6, 1] |
| scale | [0, 0.08, 0.03, 0] |
| squash | [0, -0.14, 0.05, 0] |
| straighten | [0, 0.5, 0.2, 0] |
| tempo | [0, 0.4, 0.1, 0] |
| bright | [0, 0.3, 0.1, 0] |

## gestures.keyframes.sleep

| gestures.keyframes.sleep | value |
|---|---|
| at | [0, 0.25, 0.6, 1] |
| scale | [0, 0.04, -0.04, 0] |
| squash | [0, -0.06, 0.1, 0] |
| leanY | [0, -0.04, 0.08, 0] |
| unravel | [0, 0, 0.2, 0] |
| bright | [0, 0.05, -0.2, 0] |

## gestures.keyframes.arrive

| gestures.keyframes.arrive | value |
|---|---|
| at | [0, 0.3, 0.7, 1] |
| scale | [0, -0.03, 0.01, 0] |
| squash | [0, 0.1, -0.03, 0] |

## gestures.keyframes.gulp

| gestures.keyframes.gulp | value |
|---|---|
| at | [0, 0.2, 0.45, 0.7, 1] |
| scale | [0, 0.06, -0.06, 0.03, 0] |
| squash | [0, -0.05, 0.12, -0.02, 0] |
| unravel | [0, 0.3, 0, 0, 0] |
| bright | [0, 0.08, 0.15, 0.05, 0] |

## gestures.keyframes.settle

| gestures.keyframes.settle | value |
|---|---|
| at | [0, 0.3, 0.65, 1] |
| scale | [0, -0.02, 0.01, 0] |
| squash | [0, 0.09, -0.03, 0] |
| leanY | [0, 0.05, -0.015, 0] |

## sleep

| sleep | value |
|---|---|
| idleMs | 90000 |
| commitS | 1.8 |

## placement

| placement | value |
|---|---|
| cornerMarginPx | 40 |
| dragThresholdPx | 4 |

## placement.sizes

| placement.sizes | value |
|---|---|
| small | 0.7 |
| normal | 1 |
| big | 1.35 |

## frames

| frames | value |
|---|---|
| dtCapMs | 50 |
| averageHz | [75, 144] |
| settledStopMs | 30000 |
| asleepStopsAfterCommit | true |

| frames | asleep | idle | breathing | motion |
|---|---|---|---|---|
| tier | asleep | rest | active | active |
| batteryTier | asleep | rest | rest | active |

## frames.tiersFps

| frames.tiersFps | value |
|---|---|
| active | 60 |
| rest | 30 |
| asleep | 20 |

## integrator

| integrator | value |
|---|---|
| mass | 1 |
| snapX | 0.0001 |
| snapV | 0.001 |

## reducedMotion

| reducedMotion | value |
|---|---|
| loop | false |
| framesPerEvent | 1 |
| threadsJumpToHold | true |

## lightDesktop

| lightDesktop | value |
|---|---|
| sampleMs | 1500 |
| boxPx | 96 |
| onAbove | 0.62 |
| offBelow | 0.5 |
| fadeLuma | [0.45, 0.8] |
| treatment | outline |
| alternatives | [graphite, pool, none] |

## lightDesktop.outline

| lightDesktop.outline | value |
|---|---|
| width | 1.5 |
| feather | 2.4 |
| featherAlpha | 0.16 |
| shadowAlpha | 0.14 |
| shadowRadius | 46 |

## strand

| strand | value |
|---|---|
| holdStopMs | 30000 |
| retargetBlendMs | 300 |
| speedFirst | true |
| preReachDelayMs | 350 |

## strand.stroke

| strand.stroke | value |
|---|---|
| widthPx | 3.2 |
| outlinePx | 2.8 |
| dprCap | 1.5 |
| bboxPadPx | 24 |

## strand.anchor

The thread leaves the character at `strand.anchor`: `x` and `y` are fractions of the art's viewBox, origin top-left. Oni's anchor sits on the chin, just inside the head's outline, so the line comes out from under the face. The renderer maps the anchor through the same pose matrix as the art, weighting each part by `strand.anchor.follow`: `scale` and `squash` about the art's base, `lean` the rotation, `lift` the raise from `leanY`, `hover` the float. A weight of 1 rides with the art and 0 stays at the unposed point. A character profile may move `x` and `y` (`profile.artwork.anchor`); the follow weights stay here. The first `strand.bowedPath.seamBlendPoints` points blend onto this anchor each frame.

| strand.anchor | value |
|---|---|
| x | 0.5 |
| y | 0.9 |

| strand.anchor.follow | value |
|---|---|
| scale | 1 |
| squash | 1 |
| lean | 1 |
| lift | 1 |
| hover | 1 |

## strand.accents

| strand.accents | value |
|---|---|
| guide | #7890FF |
| nudge | #69E2CF |
| ask | #D6DBEA |
| remember | #A78BFA |

## strand.bowedPath

| strand.bowedPath | value |
|---|---|
| segments | 96 |
| points | 97 |
| controls | [0.28, 0.72] |
| bowPerDistance | 0.25 |
| bowMaxPx | 100 |
| bowScale | 0.2 |
| seamBlendPoints | 12 |
| tipGapPx | 9 |
| tipStopPx | 4 |

## strand.tailLoosen

| strand.tailLoosen | value |
|---|---|
| unravelHold | 0.5 |
| depth | 0.55 |
| shrink | 0.04 |

## strand.reach

| strand.reach | value |
|---|---|
| durationS | 0.74 |
| ease | [0.45, 0, 0.2, 1] |
| surfaceSpeedPxS | 300 |
| surfaceClampS | [0.2, 0.74] |
| perchS | 0.8 |

## strand.tap

| strand.tap | value |
|---|---|
| durationS | 0.18 |
| bobPx | 3 |

## strand.retarget

A new target while the thread is out swings the tip from A to B and never goes home first. Progress restarts at `u0 = max(strand.retarget.reachFrom, u)`, where `u` is the outgoing progress (1 while holding), and runs `u = u0 + (1 − u0)·ease(t / strand.retargetBlendMs)` with `strand.retarget.ease`; `u` never drops below `u0` during the swing. Over the same `strand.retargetBlendMs` the old polyline blends point by point into the new one with the same ease. The swing ends in a tap like a reach.

| strand.retarget | value |
|---|---|
| reachFrom | 0.35 |
| ease | [0.45, 0, 0.2, 1] |

## strand.rewind

| strand.rewind | value |
|---|---|
| durationS | 0.52 |
| ease | [0.16, 1, 0.3, 1] |
| lassoS | 0.28 |
| lassoScale | 1.3 |

## strand.perch

Perch picks the top corner of the window frame nearer the anchor (Euclidean distance; a tie takes the corner nearer screen centre) and curls around it on the outside of the frame. When only one corner leaves room for the curl on its outer side (`radiusPx` between the corner and the output's side edge), that corner is taken whatever the distance, so the left half of a tiled pair perches on its top-right corner. A point target perches when it and its frame are at least `minWidthPx` × `minHeightPx`. On a frame flush with the top of the output, the part of the curl over the top edge falls off the output; the part down the outer side stays.

A perch-mode target (`strand.target` mode `perch`, sent while Oni is perched on a window) is the perched window's own frame and always curls, whatever its size. It names its corner (the perch frame's `top-left` or `top-right`, chosen by the same nearer-corner rule from where the character stood before it flew). It is exempt from `session.skipCover`, so a half-screen or maximized window still perches; a frame centred off the character's output is still skipped as `off-output`.

| strand.perch | value |
|---|---|
| minWidthPx | 320 |
| minHeightPx | 200 |
| curlS | 0.4 |
| radiusPx | 18 |
| segments | 24 |
| sweep | 1.5 |
| offsetPx | 18 |
| uncoilS | 0.42 |
| recoilS | 0.52 |

## strand.lasso

| strand.lasso | value |
|---|---|
| points | 96 |
| gapPx | 9 |
| power | 3.4 |
| turns | 1.06 |
| drawS | 0.62 |
| ease | [0.35, 0, 0.15, 1] |
| thrownPx | 7 |
| cinchS | 0.5 |
| leadPoints | 14 |
| startAt | 0.6 |
| landingGapPx | 86 |

## strand.lasso.ripple

| strand.lasso.ripple | value |
|---|---|
| amp | 1.4 |
| waves | 3 |
| decay | 5 |

## strand.region

The pen's hand wobble is deterministic so trajectories replay exactly. At arc length `s` px along the pen path the line moves along its outward normal by `w(s) = wobble·(0.6·sin(2π·wobbleRate·s + φ1) + 0.4·sin(2π·wobbleRate·1.7·s + φ2))`, with `φ1 = 2π·frac(wobbleSeed·0.6180339887)`, `φ2 = 2π·frac(wobbleSeed·0.4142135624)` and `frac(x) = x − floor(x)`. `wobbleSeed` is an unsigned 32-bit integer.

| strand.region | value |
|---|---|
| minControlAreaRatio | 2 |
| maxWindowAreaRatio | 0.7 |
| padPx | 6 |
| radiusPx | 18 |
| stepPx | 12 |
| overrun | 5 |
| wobble | 1.3 |
| wobbleRate | 0.045 |
| wobbleSeed | 9 |
| penScale | 0.55 |
| aheadPx | 26 |
| outsidePx | 8 |
| speedPxS | 1500 |
| clampS | [1.4, 2.6] |
| cornerSpeed | 0.34 |
| cornerSlowPx | 170 |
| slowEnds | 60 |
| leadPoints | 10 |
| reelS | 0.28 |

## strand.surface

| strand.surface | value |
|---|---|
| offsetXPx | 44 |
| offsetYPx | 56 |
| anchorInsetPx | 16 |
| marginPx | 16 |

## strand.burst

| strand.burst | value |
|---|---|
| lines | 3 |
| anglePi | -0.85 |
| step | 0.35 |
| fromPx | 6 |
| growPx | 4 |
| durationS | 0.16 |
| widthPx | 1 |

## strand.intro

| strand.intro | value |
|---|---|
| traceS | 1.1 |
| outlineRadiusPx | 16 |
| outlineArc | 12 |
| fadeS | 0.32 |

## strand.bubble

| strand.bubble | value |
|---|---|
| textInS | 0.18 |
| textRisePx | 3 |
| textOutS | 0.09 |
| maxWidthPx | 288 |
| borderPx | 2 |
| ringPx | 2.8 |
| kickerPx | 8.5 |
| kickerTrackingEm | 0.16 |
| textPx | 13.5 |
| hintPx | 10.5 |
| solveGutterPx | 360 |

| strand.bubble | durationS | ease | scale |
|---|---|---|---|
| open | 0.32 | [0.22, 1.12, 0.36, 1] | 0.05 |
| close | 0.13 | [0.4, 0, 1, 1] | 0.93 |

## flight

| flight | value |
|---|---|
| bow | 0.18 |
| bowMaxPx | 90 |
| speedPxS | 900 |
| clampS | [0.5, 1.1] |
| ease | [0.45, 0, 0.2, 1] |

## perch

These are the character's side of a perch; `strand.perch` is the thread's.

**Seat.** While perched, the character's frame (a square, `character.buttonPx` × the placement size) sits outside the chosen top corner: its inner edge `seat.gapPx` beyond the window's side edge and its bottom `seat.liftPx` above the window's top edge. When the outside position would cross `seat.marginPx` of the output on an axis, that axis flips to the inside of the window (inner edge `seat.gapPx` inside the side edge, top `seat.liftPx` below the top edge); the frame is then clamped `seat.marginPx` inside the output, the clamp winning over the flip. A half-screen window against the top edge therefore seats below its top edge, and a maximized one seats inside its top corner. `seat.gapPx` must exceed the curl's reach from the corner, `strand.perch.radiusPx + strand.stroke.widthPx/2 + strand.stroke.outlinePx`, so the character never sits on its own thread unless the final clamp forces it; the loader checks this.

**Reattach.** When the perched window moves or resizes, the seat is worked out again. If it moved at most `reattach.snapPx`, the character jumps with the window and the curl is rebuilt round the same corner of the new frame at once: no reach, no flight, no second arrival. A longer move winds the thread home at once (uncoil, then recoil); once the thread is home and the window has held still for `reattach.settleMs`, the character flies to the new seat under the `flight` rules, then the thread reaches (`strand.reach.perchS`) and curls (`strand.perch.curlS`) again, quietly and without a second arrival. A perch onto a different window always takes the long path. A move onto another output, or any move under reduced motion, jumps and curls in one frame.

**Highlight.** While a drag carries the character over a window, that window gets a rounded outline (`highlight.radiusPx`) `highlight.padPx` outside its frame: a white line `highlight.widthPx` wide on an ink rim `highlight.rimPx` each side, in the character's ink and white. It fades in over `highlight.fadeInS` and out over `highlight.fadeOutS`. `highlight.padPx` must be at least `highlight.widthPx/2 + highlight.rimPx`, so the outline never covers the window's own pixels; the loader checks this.

**Chip.** While perched, a chip names the window (app and title, elided to `chip.maxWidthPx` including its padding). It is centred `chip.gapPx` under the character frame, or above it when below would cross `seat.marginPx` of the output: text `chip.textPx`, padding `chip.padXPx` × `chip.padYPx`, corner radius `chip.radiusPx`, a white fill with an ink border `chip.borderPx` and ink text, fading over `chip.fadeS`.

| perch.seat | value |
|---|---|
| gapPx | 28 |
| liftPx | 6 |
| marginPx | 16 |

| perch.reattach | value |
|---|---|
| snapPx | 96 |
| settleMs | 180 |

| perch.highlight | value |
|---|---|
| widthPx | 2 |
| rimPx | 1.4 |
| padPx | 6 |
| radiusPx | 10 |
| fadeInS | 0.12 |
| fadeOutS | 0.1 |

| perch.chip | value |
|---|---|
| textPx | 11 |
| padXPx | 8 |
| padYPx | 4 |
| gapPx | 6 |
| maxWidthPx | 240 |
| radiusPx | 8 |
| borderPx | 1.5 |
| fadeS | 0.16 |

## session

`arrivalTimeoutMs` is the backstop for a target that never lands: the daemon arms it only while a sequence is pending and, when it fires, cancels that sequence without advancing the session. It must exceed the slowest healthy landing, `strand.preReachDelayMs + 1000·(strand.region.clampS[1] + flight.clampS[1] + strand.lasso.drawS + strand.lasso.cinchS + strand.tap.durationS)`, which the loader checks.

`skipCover` skips a target whose box covers at least that fraction of the output on both axes (`covers-screen`); a target centred off the output is skipped as `off-output`. Perch-mode targets are exempt from `skipCover` only (see `strand.perch`).

| session | value |
|---|---|
| arrivalGatesAdvance | true |
| staleSeqIgnored | true |
| skipCover | [0.5, 0.5] |
| skateClearGapPx | 120 |
| planDoneMax | 6 |
| planNextMax | 3 |
| arrivalTimeoutMs | 10000 |

## sound

The motifs and the cue notes are original to Hyakki, written in the yo scale: the pitch classes in `sound.scale` (0 = C). The loader rejects any motif or cue note outside that scale. Which moment plays which motif, the timings, the synthesis and the gating follow the reference behaviour; the melodies are new. Everything is synthesised at run time and no audio file ships.

| sound | value |
|---|---|
| scale | [0, 2, 4, 7, 9] |

## sound.synth

| sound.synth | value |
|---|---|
| harmonic | 0.16 |
| lowpassHz | 2400 |
| attackS | 0.012 |
| decayS | 0.045 |
| sustain | 0.45 |
| releaseS | 0.16 |
| master | 0.12 |
| peak | 0.3 |
| debounceS | 0.09 |

## sound.motifs

| sound.motifs | notes | spacingS | lengthS | delayS |
|---|---|---|---|---|
| acknowledge | [72, 76] | 0.065 | 0.11 | 0 |
| done | [72, 74, 79] | 0.065 | 0.11 | 0 |
| error | [69, 64] | 0.065 | 0.11 | 0 |
| poke | [84, 79] | 0.065 | 0.11 | 0 |
| nudge | [74, 76] | 0.065 | 0.11 | 0 |
| wake | [60, 67, 69] | 0.065 | 0.11 | 0 |
| sleep | [72, 67, 64] | 0.065 | 0.11 | 0 |
| arrive | [81, 79] | 0.065 | 0.11 | 0 |
| gulp | [64, 69, 72] | 0.065 | 0.11 | 0 |
| settle | [64, 60] | 0.065 | 0.11 | 0 |
| reach | [62, 67] | 0.065 | 0.11 | 0.36 |
| rewind | [67, 62] | 0.065 | 0.11 | 0 |
| skate | [64, 67, 69, 76] | 0.12 | 0.17 | 0.36 |
| perch | [69, 76, 74] | 0.065 | 0.11 | 0 |
| bubble | [69, 72] | 0.075 | 0.11 | 0 |
| intro | [67, 72, 69, 76] | 0.11 | 0.11 | 0 |

## sound.events

Each moment plays one motif. `gesture-<id>` fires when that gesture starts; `strand-reach` when a thread goes out (the motif's delay lets the line leave first) and `strand-lasso` when a lasso is thrown; `strand-arrive` when the tip lands; `strand-rewind` when it winds back; `strand-perch` when it curls on a corner; `bubble-open` when a card opens; `intro-trace` for the first-run drawn hello; `flight-skate` when the character flies and `flight-land` when it lands. Every motif is played by at least one moment. `sound.synth.debounceS` collapses two moments that fall together, such as `strand-arrive` and `gesture-arrive`, into one sound.

| sound.events | value |
|---|---|
| gesture-acknowledge | acknowledge |
| gesture-done | done |
| gesture-error | error |
| gesture-poke | poke |
| gesture-nudge | nudge |
| gesture-wake | wake |
| gesture-sleep | sleep |
| gesture-arrive | arrive |
| gesture-gulp | gulp |
| gesture-settle | settle |
| strand-reach | reach |
| strand-lasso | reach |
| strand-arrive | arrive |
| strand-rewind | rewind |
| strand-perch | perch |
| bubble-open | bubble |
| intro-trace | intro |
| flight-skate | skate |
| flight-land | settle |

## sound.cues

| sound.cues | value |
|---|---|
| partials | [1, 2.76, 5.4] |
| amps | [1, 0.2, 0.04] |
| pitchBendMs | 12 |
| sampleRateHz | 48000 |
| gain | 0.7 |
| handsDoneMinS | 20 |

## sound.cues.items

| sound.cues.items | notes | gainDb |
|---|---|---|
| press | [55] | -20 |
| release | [62] | -16 |
| tick | [84] | -24 |
| done-notes | [67, 76] | -18 |
| hands-start | [57, 64] | -18 |
| hands-stop | [64, 57] | -20 |

## sound.gating

| sound.gating | value |
|---|---|
| quietMs | 20 |
| muteRampMs | 20 |
| ackHoldMs | 2500 |
| speechSampleRateHz | 24000 |

## voice

The voice loop's numbers. The daemon owns capture, the segmenter, speech engines and the turn; the presenter only draws. Nothing here is audio: capture is read from a child process and never written to disk.

## voice.vad

The segmenter reads 16 kHz signed 16-bit mono capture in frames of `frameMs`. A frame is speech when its RMS reaches `speechRms` (in s16 sample units). A segment opens on the first speech frame and starts once it holds `minSpeechMs` of speech frames; a silence of `pauseMs` before that discards it as noise. A started segment asks for its first partial transcript `firstPartialMs` after its first speech frame, then every `partialEveryMs` while new speech arrives. `pauseMs` of silence after the last speech frame ends it with a final transcript and a pause end; a segment is cut into a final at `maxSegmentS` and the next speech opens a new one. Every segment carries the `preRollMs` of audio heard before its first speech frame. The loader checks that every timing is a whole number of frames and that speech starts before the first partial (`minSpeechMs < firstPartialMs`).

| voice.vad | value |
|---|---|
| sampleRateHz | 16000 |
| frameMs | 10 |
| speechRms | 400 |
| pauseMs | 450 |
| minSpeechMs | 350 |
| firstPartialMs | 650 |
| partialEveryMs | 300 |
| maxSegmentS | 9 |
| preRollMs | 200 |

## voice.budgets

A partial decode that has not returned within `partialMs` is abandoned, and no new partial starts while one is in flight; a final decode is given `finalS`. A final whose audio a complete partial already covered reuses that partial.

| voice.budgets | value |
|---|---|
| partialMs | 1200 |
| finalS | 3 |

## voice.press

A hold of the talk chord shorter than `tapDropMs` with nothing heard is dropped without a transcript. A hold is cut at `maxHoldS`. Open mic, opt-in and off by default, ends by itself at `openMicS`. Capture starts on each press; there is no warm mic.

| voice.press | value |
|---|---|
| tapDropMs | 300 |
| maxHoldS | 20 |
| openMicS | 180 |

## voice.wake

The wake phrase is opt-in and off by default. A session it opens listens for at most `sessionS`; wake stays armed for at most `armedMaxS`, then disarms with a notice and must be armed again. Speech that does not start with the wake phrase is discarded. Wake and open-mic input never answers a consent card and never reaches Hands.

| voice.wake | value |
|---|---|
| sessionS | 15 |
| armedMaxS | 3600 |

## voice.turn

One acknowledgement per press, at least `ackGapMs` after the previous one. After Oni speaks a line of n characters the mic ignores what it hears for `echoBaseMs + n * echoPerCharMs`, so Oni's own speech is never heard as a command; a press of the talk chord clears that window and cuts the speech.

| voice.turn | value |
|---|---|
| ackGapMs | 300 |
| echoBaseMs | 900 |
| echoPerCharMs | 70 |

## voice.engines

A resident speech-to-text engine unloads after `sttIdleUnloadS` without a request, a text-to-speech engine after `ttsIdleUnloadS`.

| voice.engines | value |
|---|---|
| sttIdleUnloadS | 600 |
| ttsIdleUnloadS | 1800 |

## voice.levels

Mic and speech level frames go out at most `maxHz` times a second, and only while listening or speaking. The loader checks `maxHz` does not exceed `frames.tiersFps.active`.

| voice.levels | value |
|---|---|
| maxHz | 15 |

## voice.hygiene

Transcript clean-up before a transcript is used. A phrase of up to `loopMaxWords` words said `loopRepeats` or more times in a row is kept once (a decoder stuck in a loop). A sentence of at least `sentenceMinWords` words that repeats an earlier sentence is dropped. A transcript is plausible when its word count is at most `ceil(seconds * wordsPerS) + slackWords`; an implausible transcript is dropped.

| voice.hygiene | value |
|---|---|
| loopRepeats | 3 |
| loopMaxWords | 24 |
| wordsPerS | 5 |
| slackWords | 3 |
| sentenceMinWords | 2 |

## voice.phases

Each voice phase derives the character state in `state` (`none` leaves the state as it is) and whether the presenter shows the persistent mic mark, drawn from the palette roles and not hideable by the user. `armed` means the mic is live for the wake phrase or open mic: it changes no state, so it carries the mic mark. The loader checks that `idle` derives no state and shows no mark, and that every other phase that derives no state shows the mark.

| voice.phases | state | micMark |
|---|---|---|
| idle | none | false |
| armed | none | true |
| listening | listening | false |
| transcribing | thinking | false |
| speaking | speaking | false |

## guide

The guided walkthrough's numbers. A guide reads the window, points at one control, waits for the user to act and reads again; Oni never clicks. A guide ends after `maxSteps` steps with a closing line. One screen read gets `readTimeoutS`, then the guide ends with one clear line; there is no hedged second request and no read before the user asks. A second select-and-ask pick within `pickRepeatGuardMs` of the last is ignored. The checklist limits stay in `session.planDoneMax` and `session.planNextMax`. These values come from the research and await the owner's sign-off.

| guide | value |
|---|---|
| maxSteps | 12 |
| readTimeoutS | 75 |
| pickRepeatGuardMs | 1200 |

## guide.keep

After a guide finishes with every step confirmed, Oni offers to keep it. The offer lapses to Skip after `chipS`, and nothing is written without an explicit yes. The store holds at most `maxEntries` walkthroughs and drops the least recently used first. A new request reuses a kept walkthrough when its goal matches at a similarity of at least `matchMin`. During a replay each step must find its control on screen at a score of at least `findMin` out of 100, scored on whole words so that one label inside a longer one never counts. A kept walkthrough that misses `missesToStale` times in a row is retired. The loader checks that `matchMin` lies in (0, 1] and `findMin` in (0, 100].

| guide.keep | value |
|---|---|
| chipS | 12 |
| maxEntries | 80 |
| matchMin | 0.75 |
| findMin | 80 |
| missesToStale | 2 |

## guide.settle

The settle watcher runs only after a step advances, never while a step waits for the user. Every `frameMs` it reduces the window to a `widthPx` × `heightPx` gray fingerprint, in memory only, over the whole window and over the target's box. A mean change of at least `moveDelta` on either counts as moved; at most `stillDelta` on both counts as still. When no fingerprint can be taken it waits `napMs` instead. It stops after `maxFrames` frames. The loader checks that `stillDelta < moveDelta <= 255`, that `napMs` lasts at least one frame and that `maxFrames · frameMs` outlasts `napMs`.

| guide.settle | value |
|---|---|
| widthPx | 64 |
| heightPx | 40 |
| moveDelta | 4 |
| stillDelta | 1.5 |
| frameMs | 150 |
| napMs | 450 |
| maxFrames | 20 |

## guide.hints

Proactive hints are opt-in and off by default. They come only from kept walkthroughs for the app Oni is perched on, never from a picture of the screen. At most one hint appears every `cooldownS`; a hint nobody touches retracts after `retractS`; with `oncePerKeyPerDay` each kept walkthrough is hinted at most once a day. The loader checks that a hint retracts before its cooldown ends.

| guide.hints | value |
|---|---|
| cooldownS | 1200 |
| retractS | 45 |
| oncePerKeyPerDay | true |

## guide.phases

Each guide phase derives the character state in `state`. A phase that derives no state (`none`) gives its `reason`; a phase with a state gives none. The loader checks that every phase is listed, that each state exists, and that `idle` derives no state.

| guide.phases | state | reason |
|---|---|---|
| idle | none | no guide runs |
| reading | reading |  |
| pointing | ready |  |
| waiting | ready |  |
| explaining | ready |  |
| keeping | thinking |  |
| done | ready |  |
| winding | none | the strand rewinds and the guide hands the state back |
| nudging | ready |  |

## reflex

The quick-command numbers. Reflexes are a fixed set of reversible actions, each announced as it runs; there is no free-form input and no send, buy or delete action. One utterance plans at most `maxActions` actions, and at most `maxActionsPerMinute` actions run in any minute, so a looping transcript cannot become a storm. After an app launch the next launch waits `launchCooldownS`. A verb left dangling at the end of one utterance carries to the next for `danglingVerbS`. Only opening an app or a site fires before the utterance ends, and only at a confidence of at least `earlyFireMin`; an exact app name scores `exactAppMin`. A sounds-like app match needs a score of at least `fuzzyMin` and must lead the runner-up by `fuzzyMargin`. The browser picked for a site stays the pick for `siteStickyS`. The launcher catalog holds at most `catalogMax` apps. The loader checks that `maxActionsPerMinute` admits one full utterance, that `earlyFireMin <= exactAppMin <= 1`, that `fuzzyMin < exactAppMin` and that `fuzzyMin + fuzzyMargin <= 1`. These values come from the research and await the owner's sign-off; `maxActionsPerMinute` is new.

| reflex | value |
|---|---|
| maxActions | 6 |
| maxActionsPerMinute | 12 |
| launchCooldownS | 4 |
| danglingVerbS | 6 |
| earlyFireMin | 0.8 |
| exactAppMin | 0.95 |
| fuzzyMin | 0.6 |
| fuzzyMargin | 0.1 |
| siteStickyS | 120 |
| catalogMax | 200 |

## reflex.routines

A routine is a short run of replayable actions the user asked for, kept only with their yes. It is not a Teams routine: nothing is scheduled and no agent runs. When `offerMinSteps` replayable actions follow each other with at most `gapS` between them, Oni offers once to keep them; the offer lapses to Skip after `chipS`, and nothing is written without an explicit yes. The store holds at most `maxRoutines` routines of at most `maxSteps` steps each and drops the least recently used first. Replay uses no model. The loader checks that `offerMinSteps` lies in [2, `maxSteps`].

| reflex.routines | value |
|---|---|
| offerMinSteps | 3 |
| gapS | 20 |
| maxRoutines | 20 |
| maxSteps | 8 |
| chipS | 12 |

## reflex.reminders

A reminder lies at most `maxDays` days ahead and a timer at most `maxTimerH` hours; both are kept in the reminder store, which holds at most `maxPending` of them. A reminder that comes due while Oni is busy waits up to `busyTries` times `busyWaitS`, then shows anyway; a fire that comes late says so. The wake path starts the daemon only when the next reminder is due within `wakeHorizonMin` minutes, and the daemon stays resident only while the next one is due within `residencyHorizonMin` minutes. The loader checks that a timer is shorter than the longest reminder, that `residencyHorizonMin` exceeds `wakeHorizonMin` and that the whole busy wait ends within the wake horizon. `residencyHorizonMin` is new.

| reflex.reminders | value |
|---|---|
| maxDays | 7 |
| maxTimerH | 24 |
| busyTries | 4 |
| busyWaitS | 15 |
| wakeHorizonMin | 30 |
| residencyHorizonMin | 35 |
| maxPending | 50 |

## reflex.decisions

When the rules miss, an opted-in typed decision may name the action. Its answer counts only at a confidence of at least `minConfidence` with a lead of at least `minMargin` over the runner-up; anything less goes to the slow path. An answer never grants permission. One decision gets `budgetMs`, and at most `hostedPerMin` hosted decisions are asked in any minute. The loader checks that `budgetMs` lies in [100, 5000], the bounds `providers.decisions.budgetMs` accepts, and that `hostedPerMin` does not exceed `maxActionsPerMinute`. `hostedPerMin` is new.

| reflex.decisions | value |
|---|---|
| minConfidence | 0.7 |
| minMargin | 0.15 |
| budgetMs | 1500 |
| hostedPerMin | 6 |

## reflex.ledger

The local ledger records what each reflex did: the action, the route, the time taken, the confidence and the outcome, never the words. It keeps at most `maxRows` rows, oldest out first.

| reflex.ledger | value |
|---|---|
| maxRows | 500 |

## hands

The numbers for Hands. Hands works in one stage at a time, and choosing one never enables another: one browser tab the user authorized, one live window the user picked, or a separate nested desktop that Hyakki owns. It never drives the user's whole live desktop, so no number below describes one. Hands fills fields, types, makes ordinary clicks and scrolls, and announces each step. It never presses a submit, send, post, pay, buy, delete or sign-out control, or anything it cannot classify: it points at that control and waits for the user to press it. It never runs an executable; it only shows its folder. A spoken yes approves nothing, and wake or open-mic speech never starts or resumes Hands. These numbers wait for the owner's ratification.

A task takes at most `budget.steps` steps, plus `budget.extra` more only while no step has been refused or rejected. The same action on the same control `repeatLimit` times in a row halts the task. A finish that the screen or the read-back contradicts is rejected, and `doneRejections` rejections end the task. A paused task can be resumed for `resumeS` seconds, by a press or a button only; after that it ends. After a task finishes, a follow-up within `followUpS` continues it without an anchor phrase. Each model request carries the last `historyCarry` steps and gets `readTimeoutS`, the same time a guide read gets. A Hands consent card lapses after `consentTtlS`, the same lifetime as every other consent card. One read-back of a filled value gets `readbackMs` and one action gets `actMs`. A stop, from any path, aborts the model request and removes the glow and the pill within `stopBudgetMs`. The loader checks that `repeatLimit` lies in [2, `budget.steps`], `doneRejections` in [1, `budget.steps`) and `historyCarry` in [1, `budget.steps`]; that a card lapses within the resume window (`consentTtlS` ≤ `resumeS`); that `stopBudgetMs` < `readbackMs` < `actMs`, so a stop always outruns a step; and that a model request outlasts one action and ends within the resume window (`actMs` < `readTimeoutS`·1000, `readTimeoutS` ≤ `resumeS`). `consentTtlS` is also tested equal to the consent card lifetime in `internal/oni/feature`.

| hands | value |
|---|---|
| repeatLimit | 3 |
| doneRejections | 2 |
| resumeS | 300 |
| followUpS | 120 |
| historyCarry | 6 |
| consentTtlS | 120 |
| readTimeoutS | 75 |
| readbackMs | 1500 |
| actMs | 3000 |
| stopBudgetMs | 250 |

## hands.budget

| hands.budget | value |
|---|---|
| steps | 15 |
| extra | 10 |

## hands.page

What one page read may put in front of the model, however large or hostile the page is. At most `maxControls` controls are listed, each name cut to `maxNameRunes`; the visible page text is cut to `maxTextRunes` and goes inside a fence as material, never as instructions. One fill is at most `maxFillRunes` long. The loader checks that a control name is shorter than the page text bound and that a fill fits within it, so the read-back always sees the whole value. All four are new.

| hands.page | value |
|---|---|
| maxControls | 200 |
| maxNameRunes | 80 |
| maxTextRunes | 6000 |
| maxFillRunes | 2000 |

## hands.glow

While Hands works, four strips `stripPx` wide light the screen edges. Their opacity breathes between `low` and `high` over `periodS`, repainted `fps` times a second, and is scaled by `waitingFactor` while a card waits. On stop the glow is removed in the same commit, with no fade; under reduced motion it holds still. The loader checks that `fps` lies in [1, 30], that 0 < `low` < `high` ≤ 1, that one breath spans at least two frames and that `waitingFactor` lies in (0, 1).

| hands.glow | value |
|---|---|
| fps | 12 |
| low | 0.72 |
| high | 1 |
| periodS | 3.2 |
| stripPx | 130 |
| waitingFactor | 0.5 |

## hands.ledger

The Hands ledger records each step's tool, control role, outcome, reason and time taken, never a typed word, a control name or page text. It keeps at most `maxRows` rows, oldest out first.

| hands.ledger | value |
|---|---|
| maxRows | 500 |

## hands.app

The rules for one live window the user picked for the task. Before and after every action Hands checks the window against the baseline it took when the user picked it: the same window, app and process, on the same output, shown, the only focused window, with the same title and a box that has not moved or grown by more than `geometryTolPx` pixels. One check gets `verifyMs`. Any failed check pauses the task, and so does any key the user presses while an action is live; the key reader reports only that a key went down, never which key. A paused task resumes only from a button, within the resume window. Text goes in chunks of at most `typeChunkRunes` runes, never through the clipboard. Each typing call waits `settleMs` for the keymap before its first key and gets `chunkMs`. One type action carries at most `maxTypeRunes` runes. Typed text is read back by OCR on a masked crop of the window within `readbackMs`; an unreadable result never passes, and a second one ends the task. The loader checks that `geometryTolPx` lies in [0, 8] and `typeChunkRunes` in [1, 40]; that `typeChunkRunes` ≤ `maxTypeRunes` ≤ `hands.page.maxFillRunes`; that `hands.stopBudgetMs` < `verifyMs` < `chunkMs`, so a stop outruns a check and a check fits a typing call; that `settleMs` < `chunkMs`; and that a stop outruns a typed read-back (`hands.stopBudgetMs` < `readbackMs`). All seven are new.

| hands.app | value |
|---|---|
| geometryTolPx | 2 |
| verifyMs | 500 |
| settleMs | 50 |
| chunkMs | 5000 |
| readbackMs | 4000 |
| typeChunkRunes | 40 |
| maxTypeRunes | 200 |

## hands.desktop

The separate nested desktop Hyakki owns: niri on a headless weston, started by Hyakki itself on either host compositor, and offered only when both programs are installed. Nothing on it reaches the user's live session, and nothing ever falls back to the live session. It has one output of `widthPx` × `heightPx` and must report ready within `readyS`, or the start fails and cleans up. A stop asks it to end and kills it after `stopGraceS`. With no task it ends after `idleS`; it also ends with the daemon and never survives a restart. A task on it pauses when the presenter is lost, when the nested output changes, or when the user focuses Hyakki's own surface. The loader checks that `stopGraceS` < `readyS` and that the nested desktop outlives the resume window (`hands.resumeS` ≤ `idleS`). All five are new.

| hands.desktop | value |
|---|---|
| widthPx | 1280 |
| heightPx | 800 |
| readyS | 20 |
| stopGraceS | 3 |
| idleS | 600 |

## hands.task

The task supervisor runs one task at a time and keeps at most `queueMax` waiting behind it, first in, first out; a full queue refuses a new task. A queued task starts only from the app or the presenter, and a task that fails on a Hands error is never retried. One task takes at most `turns` model turns and `ms` milliseconds. `hostedUsd` caps measured usage, and applies only on a hosted route that reports usage; any other route is bounded by `turns` and `ms` alone. Nothing resumes after a restart: a task found running becomes interrupted, and the user restates the goal and presses Start. The loader checks that a task allows one full step budget (`turns` ≥ `hands.budget.steps` + `hands.budget.extra`), that it outlasts the resume window (`hands.resumeS`·1000 < `ms`) and that the journal holds the running task and a full queue (`queueMax` < `hands.retention.rows`).

| hands.task | value |
|---|---|
| turns | 40 |
| ms | 600000 |
| hostedUsd | 2 |
| queueMax | 1 |

## hands.retention

The task journal and projects keep state, stage, app id, step and times, never the goal text, a window title, a URL or a typed word. A row is kept at most `days` days, and at most `rows` rows are kept, oldest out first. Deleting all of them is always one action.

| hands.retention | value |
|---|---|
| days | 30 |
| rows | 200 |

## memory

Voice memory: remembering, forgetting and correcting facts by voice, all through the one memory store, all under the notes switch, and each only from a press. "No, I meant ..." corrects the note saved last, and only within `correctS` seconds of saving it; later it is refused and the note stays. A forget names the facts it found on a card, at most `forgetMax` of them, and forgets them only on a yes from a press; a no or a lapsed card keeps them. The loader checks that `correctS` is at most 600. These numbers wait for the owner's ratification.

| memory | value |
|---|---|
| correctS | 120 |
| forgetMax | 5 |

## recall

Recall answers from the chats the user imported, past conversations, memory facts and, only while `oni.recall.vault` is on, the Ryoku vault, read only. At most `passages` passages go into one request, each cut to `passageTokens` tokens and all of them together to `promptBytes` bytes, inside a fence that marks them as material, never instructions. The answer gets `answerTokens` tokens and `requestS` seconds. A local route sends nothing off the computer; a hosted one first shows the passages' titles and the route on a card. These numbers wait for the owner's ratification.

| recall | value |
|---|---|
| passages | 8 |
| passageTokens | 350 |
| promptBytes | 28672 |
| answerTokens | 500 |
| requestS | 30 |

## rewrite

Rewrite in place reads the selection, never the clipboard, shows it with the route before anything is sent, and types a one-paragraph result back over it; a longer result is offered to copy instead. A selection is at most `maxRunes` runes; one request gets `requestS` seconds. The original stays in memory only, for `undoS` seconds after the result is typed, then it is dropped. Undo presses Ctrl+Z only while each press can be checked by reading the window back; when it cannot, the original is offered to copy. The loader checks that `maxRunes` is at most 20000. These numbers wait for the owner's ratification.

| rewrite | value |
|---|---|
| maxRunes | 8000 |
| requestS | 60 |
| undoS | 60 |

## focus

The focus guard keeps the user off the feeds they name. It stays armed for at most `maxMinutes` minutes, or until the app it was armed for closes, whichever comes first, and the daemon stays up only while it is armed. Before each close a card names the feed and the app; a browser window matched only by its title is announced, never closed. "Let me back" reopens what the guard closed, only within `reopenS` seconds of the close. Its look comes from the renderer's palette roles; no colour is set here. The loader checks that `maxMinutes` is at most 720 and that `reopenS` is no longer than the guard itself. These numbers wait for the owner's ratification.

| focus | value |
|---|---|
| maxMinutes | 480 |
| reopenS | 600 |

## assignment

Assignment mode watches the perched window only after the user starts it, and only on window events, never by polling. A new problem is noticed first on a grey thumbnail `gate.widthPx` × `gate.heightPx`; typing an answer, a cursor or a clock is not a new problem. The last `cacheViews` solved views come back with no request. On a hosted or paid route one local day allows `attemptsPerDay` solves and `tokensPerDay` measured tokens; usage that is not measured is flagged, never counted as free, and a local route is recorded but not capped. The fast lane gets `fastS` seconds and is re-sent once after `hedgeS` seconds, only on a transient error and never on a rate limit; the checker lane gets `checkerS` seconds and gives `steps.min` to `steps.max` steps. The loader checks that the thumbnail is at most 128 pixels a side, that `hedgeS` < `fastS` and that `steps.min` ≤ `steps.max`. These numbers wait for the owner's ratification.

| assignment | value |
|---|---|
| cacheViews | 30 |
| attemptsPerDay | 60 |
| tokensPerDay | 200000 |
| hedgeS | 4 |
| fastS | 30 |
| checkerS | 60 |

## assignment.gate

The new-problem gate ignores OCR differences when the grey thumbnail is identical. Otherwise, a mean absolute grey difference at or below `stillDelta` (0 to 255) suppresses word-loss-only changes; larger changes count as a new problem when at least `goneFraction` of the earlier words are missing. A changed problem number, or more than `criticalTokens` earlier numbers and operators replaced on their own text lines, can identify a new problem even in a small pixel change. Lost OCR alone is not a critical replacement: new critical evidence must appear in the same overlapping vertical text band. An added answer on another line cannot replace a missing repeated heading.

| assignment.gate | value |
|---|---|
| widthPx | 48 |
| heightPx | 27 |
| stillDelta | 3 |
| goneFraction | 0.5 |
| criticalTokens | 1 |

## assignment.steps

| assignment.steps | value |
|---|---|
| min | 3 |
| max | 8 |
