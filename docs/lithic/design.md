# Lithic

Lithic is the UI language Ryoku apps share. Ryoport is its reference
implementation; this folder is the specification. Everything here is written
so a team can rebuild the look in another app and another stack without
reading Ryoport's source.

The thesis in four words: **cut, not molded.**

Material 3 gives Lithic its working structure (an adaptive list and detail, a
navigation rail, real controls, tonal containers, motion that answers a cause).
Lithic replaces Material's molded surface with cut stone: flat planes, 45
degree chamfers, one voice for action, one voice for place, minerals for
state, and type set monumentally.

Read this page first. The companion pages go deeper on one subject each:

| Page | Subject |
|---|---|
| [palette.md](palette.md) | Where colour comes from: applying the live Ryoku matugen palette to Lithic's roles. There are no colour values in Lithic, only roles and a mapping. |
| [components.md](components.md) | The component catalogue: anatomy, geometry, states and motion for every control. |
| [layout.md](layout.md) | Size classes, the rail, panes, pages, overlays, focus and keyboard. |
| [motion.md](motion.md) | The one curve, the duration table, and how to draw nothing at rest. |
| [copy.md](copy.md) | How Lithic talks: claims and proofs, sentence case, the same verb all the way through. |
| [checklist.md](checklist.md) | The review checklist for a surface that claims to be Lithic. |
| [examples/](examples/) | Runnable, verified implementations: web (CSS + JS), QML (Qt 6 / Quickshell), Go (Gio). |

## 1. The nine principles

1. **Cut, not molded.** Corners are cut at 45 degrees or left square, never
   rounded. The cut follows the scale of the thing: 16 dp on panes and
   dialogs, 12 dp for a row's state notch, 7 dp on buttons and fields, 6 dp on
   small controls, 4 dp on a toggle track, 3 dp on tokens. Only content
   (logos, charts) may curve.

2. **Flat planes.** Every surface is one flat tone. Depth is only the order of
   tones: ground, stone 1, stone 2, stone 3, stone 4, then bone. No gradients,
   glow, inner shadows, inset wells or drop shadows. Overlays are a scrim and
   a solid slab one tone up with a hairline. Blur can be used where it fits
   and looks good, as a hint on a plane (see Blur, where it fits).

3. **Bone is where you are.** The selected row, the current rail destination,
   the current filter, the on half of a toggle: all are selection slabs in the
   `sel` role with `onSel` ink. Nothing else uses that role.

4. **Gold is what you can do.** The `gold` role fills consequential actions
   only (connect, start, stop, create, save, sign in). Secondary actions use a
   gold hairline and gold ink. Navigation, view controls and anything you can
   only watch are stone. Gold never decorates and never shows status.

5. **Stones are states, beside their word.** Jade is alive, lapis is becoming,
   amber needs you, jasper failed, and idle is a vein outline. A state is
   always three things at once: its mineral, a shape (notch, flake or mineral
   field) and a plain word. Colour never carries state alone.

6. **The blade marks progress and place.** A bar whose leading edge is cut at
   45 degrees: the current tab's underline, a job's progress, a meter.

7. **Engraved data.** Charts are a fine foreground line over a 45 degree
   hatch, labels in Space Mono, no coloured area fills.

8. **One family, set monumentally.** The selected thing's name is the one big
   line in a view (Space Grotesk Bold at 52 sp). Everything else is at working
   size. Space Mono appears only where the characters are literal data.

9. **Motion answers a cause and stops.** One curve, a short table of
   durations, and no frame drawn when nothing changed. Reduced motion turns
   every transition into an instant state change.

## 2. Vocabulary

Lithic names its parts after stone work. Use these words in code, in
comments and in reviews so everyone means the same thing.

| Word | What it is |
|---|---|
| **ground** | The exposed plane behind the panes. The darkest tone in dark mode, the darkest stone in light mode. |
| **stone 1 to 4** | The ordered stack of flat planes: rail and list pane (1), work pane and row tablets (2), hover, fields and tracks (3), menus and dialogs (4). |
| **vein** | The hairline tone: quiet outlines, separators, idle flakes, chart baselines. |
| **bone** | The selection slab (`sel`) and its ink (`onSel`, quiet ink `onSel2`). Where you are. |
| **gold** | The action fill (`gold`, `onGold`) and the action ink on stone (`goldInk`). What you can do. |
| **mineral, stone (of a state)** | Jade, lapis, amber, jasper. The colour half of a state. |
| **plaque** | A button: a cut slab you can press. Pressing chips the corners deeper. |
| **tablet** | A resource row: a flat stone 2 slab, square apart from its notch. |
| **notch** | The 45 degree cut at a tablet's top-left corner, filled with the state's mineral. Also the amber update marker on the Settings rail tile. |
| **flake** | The standalone state token: a short slanted quadrilateral. Outlined when idle, filled by its mineral when active, part filled while becoming. |
| **mineral field** | The header slab of the selected thing, tinted by mixing its state's mineral into stone 2. |
| **blade** | A progress or selection bar with a cut leading edge. |
| **slab** | A stone 1 container inside a page (a settings section). Also any bone selection surface. |
| **claim and proof** | A state word and the fact that backs it: "Online" over "23 ms". |
| **chisel** | The one easing curve, `cubic-bezier(.2, 0, 0, 1)`. |
| **knapping** | Lithic's motion as a whole: short, decisive, no bounce. |

## 3. Materials: roles, not colours

Lithic has no palette of its own. Every colour in a Lithic surface is a
**role**, and the roles are filled from the live Ryoku palette (matugen) on
the user's desktop, so an app matches the wallpaper and changes with it.
[palette.md](palette.md) is the complete recipe. The short version:

| Role | Dark palette source | Used for |
|---|---|---|
| `ground` | `surfaceContainerLowest` | behind the panes |
| `stone1` | `surface` | rail, list pane |
| `stone2` | `surfaceContainerLow` | work pane, tablets |
| `stone3` | `surfaceContainer` | hover, fields, tracks |
| `stone4` | `surfaceContainerHigh` | menus, dialogs, tooltips |
| `vein` | `surfaceContainerHighest` | hairlines, quiet outlines |
| `fg`, `fg2`, `fg3` | `onSurface`, `onSurfaceVariant`, `outline` | type, strong to faint |
| `sel`, `onSel` | `secondaryContainer`, `onSecondaryContainer` | where you are |
| `onSel2` | `onSel` mixed 45 percent toward `sel` | quiet text on a selection |
| `gold`, `onGold` | `primary`, `onPrimary` | what you can do |
| `goldInk` | `primary`, re-toned to 4.5:1 on `stone2` | gold type and hairlines |
| `jasper` | `error`, re-toned to 4.5:1 on `stone2` | failed, unreachable, destructive |
| `jade`, `lapis`, `amber` | fixed by Lithic | alive, becoming, needs you |
| `scrim` | `scrim` at 0.62 (dark) or 0.45 (light) | behind dialogs |

Light palettes map from the other end of the surface ramp, so the ground is
the darkest stone and the work pane the lightest. Jade, lapis and amber do
not follow the palette because they carry meaning; a Ryoku user must read
"alive" the same way on every wallpaper.

Two named rules govern the roles:

**The bone and gold rule.** Bone means where you are. Gold means what you can
do. Never exchange those jobs, and never use either for decoration.

**The state has three parts rule.** A state needs its mineral, a notch or
flake, and a plain word. Colour never carries state alone.

Derived tones are computed, never picked:

| Derived | Recipe |
|---|---|
| mineral field | `mix(stone2, mineral, 0.17)` for jade, `0.15` for lapis, amber, jasper; idle field is `stone3` |
| hover on a filled plaque | `mix(fill, white, 0.10)` |
| hover on a transparent plaque | ink at alpha `0.10` |
| hover on a tablet or rail tile below `stone4` | step one plane up (`stone2` to `stone3`, `stone1` to `stone3`) |
| hover on a row on `stone4` | `mix(stone4, fg, 0.06)`; nothing is above `stone4`, so it must not step down |
| press on a tablet or menu row | add `fg` at alpha `0.08` |
| disabled | the whole control at opacity `0.5` |
| focus ring | a 2 dp `fg2` stroke, 2 dp outside the control's cut, cut enlarged by 2 dp |

### Blur, where it fits

Blur can be used where it fits and looks good. It fits on a plane whose
subject is a picture, such as the art of a now playing view, an album or a
profile, where the plane takes a soft echo of what it shows, and on a scrim,
which may blur what it covers. It is a hint laid on a role, never a surface of
its own:

| Derived | Recipe |
|---|---|
| picture backdrop | the picture blurred until no shape survives; each colour keeps its hue, with its OKLCH lightness held within 0.05 of the plane's and its chroma at most 0.08; drawn at 0.35 over the plane's role, inside its cut |
| frosted scrim | what the scrim covers, captured once when the overlay opens and blurred, under the usual scrim |

- Type, hairlines and states over a backdrop keep the contrast they have on
  the bare role. That is why its lightness stays near the plane's.
- Nothing about it moves at rest. A backdrop changes only with its picture,
  crossfading over the field duration on the one curve.
- Never blur a control, type or a state, and never use blur in place of a tone
  step. Slabs, menus, dialogs and tooltips stay solid. Shadows, glows and
  gradients stay out.

Each stack makes it the cheap way. QML uses `MultiEffect` (`blurEnabled`,
`blurMax` 64) over a downscaled `ShaderEffectSource`, set `live: false` once it
settles. The web uses a small image scaled up, or `filter: blur()` on a layer
behind the content, and `backdrop-filter` only for a scrim over still content.
Gio has no blur op: blur a small downsample (24 px is plenty) once, off the
frame goroutine, and draw it scaled with linear filtering, as Ryotunes 2 does
in `internal/client/ui/look/backdrop.go`. The terminal has none.

## 4. Type

Two families, embedded with no system fallback:

- **Space Grotesk** for everything a person reads: names, sentences, labels,
  prose, buttons.
- **Space Mono** only for literal data: addresses, ports, paths, counts,
  proofs, chart ticks. Never as a technical mood.

The scale, in sp (size / line height):

| Style | Size | Weight | Used for |
|---|---|---|---|
| `name` | 52 / 56 | Bold | the selected thing's name, the one monument in a view |
| `sentence` | 34 / 40 | Bold | fleet-wide summaries; the name when the pane is narrower than 640 dp |
| `section` | 21 / 28 | Bold | page titles and large section heads |
| `title` | 15 / 20 | Medium | section titles inside a pane |
| `row` | 15 / 20 | Medium | row names |
| `body` | 14 / 20 | Regular | prose, form content |
| `proof` | 15 / 21 | Regular | the sentence under a name |
| `label` | 13 / 16 | Medium | buttons, tabs, filters, controls |
| `caption` | 12 / 16 | Regular | facts, secondary lines |
| `group` | 12 / 16 | Medium | list group headers |
| `mono` | 12 / 16 | Regular (Space Mono) | data |
| `monoSm` | 11 / 15 | Regular (Space Mono) | proofs, counts, chart ticks |

Rules:

**The one monument rule.** One line per view gets `name`. Everything else
stays at working size.

**The literal data rule.** Space Mono only where the characters are data.

Sentence case everywhere. Never all caps, not even for group headers.

## 5. Shape: the cut scale

A cut is a 45 degree chamfer measured along one edge. The eight-point outline
of a rectangle `(x0, y0, x1, y1)` with cut `c` on every corner is

```
(x0+c, y0) (x1-c, y0) (x1, y0+c) (x1, y1-c) (x1-c, y1) (x0+c, y1) (x0, y1-c) (x0, y0+c)
```

Each corner may take its own cut; a pane cuts only its two top corners.

| Cut | dp | Applied to |
|---|---|---|
| `token` | 3 | flakes, chart handles, tooltip and small tokens |
| `toggle` | 4 | the toggle track (its knob cuts 2) |
| `small` | 6 | small plaques (32 dp), filter chips, icon buttons, compact rail tiles |
| `mid` (control) | 7 | plaques (40 dp), fields, pickers, settings slabs, rail tiles |
| `notch` | 12 | a tablet's state corner, the rail's update marker |
| `large` | 16 | panes (top corners only), dialogs |
| menus, placards, tooltips | 8, 8, 4 | overlays one tone up, with a 1 dp vein hairline |

A press deepens a control's cut by **4 dp** and sinks its content by **1 dp**.

Hairlines are **1 dp**, drawn wholly inside the outline. When a cut outline is
stroked, the inner path's cut shrinks by `width × (2 − √2)` so the stroke
stays an even width around the chamfer.

Other cut shapes:

- **Notch.** A right triangle filling a corner: `(x, y) (x+12, y) (x, y+12)`.
- **Blade.** A bar whose trailing top corner is cut by its own height, so the
  leading edge reads as a 45 degree edge: `(x0, y0) (x1−h, y0) (x1, y1) (x0, y1)`.
- **Flake.** A parallelogram `1.4 × 0.7` of its nominal size with both short
  sides at 45 degrees. Idle: a 1.5 dp vein outline. Active: filled with its
  mineral. Becoming: vein base with the mineral filled to the progress fraction.
- **Octagon.** The only other polygon: the brand mark's inner shape and the
  empty state's icon frame.

## 6. Space

The spacing run is `4, 8, 13, 21, 34, 55` dp (`s1` to `s6`). Use it without
interpolating. Common applications:

| Gap | Where |
|---|---|
| 4 | between tablets in a list; toggle knob inset; chip gap |
| 8 | between plaques in an act row; rail tile gap; pane gap; popover gap from its anchor |
| 13 | mark to copy inside a tablet; title to its first row; chip inset |
| 21 | section inset inside a slab; header to acts; settings row gap |
| 34 | dialog padding; tab strip and work-pane body side inset |
| 55 | between major regions of a full page |

Mineral headers and Settings pages use their explicit 36 dp side inset.

Density: tablets are 56 dp in comfortable density and 48 dp in compact
density. Nothing else changes with density.

## 7. Layout, briefly

Lithic surfaces are list-detail workspaces at three widths:

| Class | Width | Rail | List | Work pane |
|---|---|---|---|---|
| expanded | 1280 dp and up | 88 dp, icons with labels, 64 dp tiles | 384 dp | the rest |
| medium | 760 to 1279 dp | 72 dp, icons only, 56 dp tiles | 320 dp | the rest |
| compact | under 760 dp | none; a 64 dp bottom bar | full width | replaces the list, with Back |

Panes start 10 dp below the window edge, sit 8 dp apart, cut their top
corners at 16 dp and stay square at the bottom. The rail and list pane are
stone 1; the work pane is stone 2; the ground shows between them. Structure
changes at a class boundary; a narrow desktop layout is never squeezed.

With no resource selected, the work pane is the Now board. It places the
fleet sentence and proof above pinned tiles, then from 860 dp splits Needs
you and Running now from This computer and Recent activity. Pinned tiles and
rows expose one action. Host CPU, memory and VM disk use quiet `fg2` meters
that turn amber at 90 percent.

Full pages such as Settings use a 760 dp content column. At expanded width a
200 dp SectionIndex sits 34 dp to its left inside a centred 994 dp region.

[layout.md](layout.md) has every number, the rail, overlays and the keyboard.

## 8. Components, briefly

Every control is a cut slab with one tone, one ink, and the shared hover,
press, focus and disabled recipes from section 3. The catalogue in
[components.md](components.md) specifies each one; this is the shape of the
family:

| Component | Tone | Cut | Height |
|---|---|---|---|
| plaque, gold | `gold` fill, `onGold` ink | 7 | 40 (small: 6, 32) |
| plaque, line | transparent, `goldInk` ink and 1 dp hairline | 7 | 40 |
| plaque, quiet | transparent, `fg2` ink, `vein` hairline | 7 | 40 |
| plaque, on | `sel` fill, `onSel` ink | 7 | 40 |
| plaque, danger / danger line | `jasper` fill with `inkOn(jasper)` / `jasper` ink and hairline | 7 | 40 |
| split plaque | two plaques 2 dp apart sharing one outline; the trailing one is 40 wide | 7 | 40 |
| icon plaque | square 40 (small 32), icon 20 (18) | 6 | 40 / 32 |
| field, search | `stone3`, `fg` ink, focus 1 dp `fg2` inner stroke, error `jasper` | 7 | 44 |
| picker | `stone3` anchor opening a menu | 7 | 44 |
| toggle | track `stone4` + vein hairline off, `sel` on; knob 14 | 4 (knob 2) | 22 × 40 |
| slider | `stone3` track 6, `fg2` blade, `sel` handle 8 × 20 | handle cut min(w/2, h/4) | 32 |
| stepper | two small quiet icon plaques around a value | 6 | 32 |
| filter chip | quiet at rest, `sel` when on, count in `monoSm` | 6 | 32 |
| tabs | labels in a 44 dp strip over a 1 dp vein baseline; 3 dp `fg` blade moves | blade | 44 |
| tablet | `stone2`, square with a 12 dp notch; `sel` slab when selected | notch | 56 / 48 |
| group header | `group` title and `monoSm` count in `fg3` | none | 36 |
| slab | `stone1`, 21 dp inset | 7 | content |
| mineral field | `field(tone)` behind a header | none | content |
| blade meter / progress | `stone3` track 6, fill by role; progress is `lapis` | blade | 6 |
| flake and claim | mineral token beside its word | 45° sides | about 13 × 6 at nominal 9 dp |
| menu | `stone4`, vein hairline, 40 dp rows, 220 to 360 wide | 8 | content |
| dialog | `stone4`, vein hairline, 520 default width or a caller width such as 720 for catalogue, 34 dp padding, scrim behind | 16 | content |
| placard | `sel` slab 48 high, `onSel` text and action, drains over 5 s | 8 | 48 |
| tooltip | `stone4`, vein hairline, after 600 ms | 4 | content |
| engraved chart | 1.5 dp `fg` line over a 6 dp hatch of 1 dp lines at 30 percent `fg` | none | caller |
| empty state | 72 dp octagonal vein frame, `fg3` icon, title, body, plaques | octagon | content |
| skeleton | `stone2` tablets with `stone3` bars | none | row height |
| working | three flakes filling in turn over 900 ms | flake | caller |

## 9. Motion, briefly

One curve: **chisel**, `cubic-bezier(.2, 0, 0, 1)`. No bounce, no overshoot.

| Motion | Duration | What it says |
|---|---|---|
| press: plaque cuts deepen 4 dp and content sinks 1 dp; rows and menu items dip | 90 ms | you pressed it |
| hover: a plane fades up one tone; a tablet's claim gives way to its action | 160 ms in, 100 ms out | this is what Enter will do |
| focus ring fade | 120 ms in, 100 ms out | the keyboard is here |
| selection and filter slabs, tab blade and rail tile glide | 240 ms | you are here now |
| mineral field changes stone | 320 ms | its state changed |
| notch and disclosure unfold | 200 ms | state or detail changed |
| new work pane content fades in, rising 8 dp | 200 ms | you went somewhere |
| popover slides 6 dp in and 4 dp out | 160 ms in, 100 ms out | the menu is open |
| dialog rises 12 dp and drops 8 dp | 300 ms in, 160 ms out | a protected choice opened |
| placard rises 8 dp | 300 ms | something happened |
| becoming name fills and blade edges ease with a spring | with the job | it is being made |
| wheel notch glides 120 dp; trackpad follows the fingers at screen scale | 140 ms for a wheel notch | the page followed the input |
| Undo placard blade drains | 5 s | you can still take it back |

Reduced motion (the desktop's setting or the app's) makes every transition
instant. An idle Lithic window draws no frames. [motion.md](motion.md) has
the implementation recipe.

## 10. Copy, briefly

Sentence case, plain verbs, the same word through a whole flow: a plaque that
says Start produces "Starting" and then "Running". A state always comes with
its proof ("Online, 23 ms"; "Needs you, sign-in failed 2 min ago"). Errors
say what happened and what to do. Empty states offer the next step. Space
Mono wraps only the literal parts. [copy.md](copy.md) has the word lists.

## 11. Do and don't

Do:

- Use roles, never colour values, so the live palette can fill them.
- Keep one consequential gold action visible in the current task.
- Pair every state colour with a word and a notch or flake.
- Use the cut scale and the spacing run as given.
- Stop animating and stop requesting frames as soon as a transition settles.
- Keep any blur a hint: under a picture or on a scrim, near the plane's
  lightness, still at rest.
- Keep drafts in the work pane. Use dialogs for protected decisions and for
  the selected catalogue system.
- Let Escape close the topmost thing first.

Don't:

- Round a corner or draw a pill.
- Add a shadow, gradient, glow or inset well.
- Blur a control, type or a state, or use blur in place of a tone step.
- Use gold for status, decoration or navigation; use bone for anything but the current place.
- Use Space Mono for prose or labels.
- Show state with a dot or with colour alone.
- Set anything in all caps.
- Invent a new cut, a new spacing value or a new duration.

## 12. Adopting Lithic in an app

Use [working.md](working.md) for the adoption sequence, stack entry points
and review path.
