# Checklist

Use this to review a surface that claims to be Lithic: a new app, a new page,
a port of an existing screen. Take a screenshot at each size class in a dark
and in a light Ryoku palette, at Contrast standard and high and with Tint off
and on, and go down the list. Every "no" is a defect.

## Colour

- [ ] No colour value appears in the UI code except the signature fallback
      and the three fixed minerals (jade, lapis, amber), both clearly marked.
- [ ] Changing the wallpaper changes ground, panes, bone and gold; jade,
      lapis and amber stay.
- [ ] A named theme (`themePalette`) wins over the wallpaper palette.
- [ ] Forcing light on a dark wallpaper gives signature neutrals with the
      wallpaper's gold.
- [ ] `contrast(goldInk, stone2) >= 4.5` and
      `contrast(jasper, stone2) >= 4.5`; danger fill ink is
      `inkOn(jasper)`.
- [ ] Bone (`sel`) appears only on the current place: selected row, current
      destination, selected filter or segment, toggle on, placards. A
      selected row, chip or segment carries its `onSel` place blade.
- [ ] Gold fills only consequential actions; one gold plaque per view (the
      frame's standing action, such as New, is the frame's own); no gold
      status, badge or decoration.
- [ ] Every state shows a mineral, a shape (notch, flake, field) and a word.
      No lone dots, no colour-only state.
- [ ] Disabled controls use alpha 0.5 and do not hover.
- [ ] Rows on `stone4` hover with `fg` at alpha 0.06 and press at 0.08.
      They never step down to `stone3`; lower planes keep the plane-up recipe.
- [ ] Nothing relies on one tone step to be found at rest. Fields carry their
      `edge` hairline, slabs and overlays their `vein` hairline.
- [ ] Slabs, menus and dialogs lift: in dark mode they are lighter than or
      equal to their plane, in light mode their hairline sets them off. No
      container is darker than the plane it stands on.
- [ ] Text, edges and the selection read in both Contrast settings and at
      every Tint, in dark and in light.

## Shape

- [ ] No rounded corner anywhere. No pill.
- [ ] Cuts come from the scale: 3, 4, 5, 6, 7, 12, 16 (8 for menus and
      placards, 4 for tooltips). Nothing in between.
- [ ] Every control with a cut has a fill or a hairline that clears APCA
      Lc 15 against its plane; only a quiet plaque shows its cut on hover
      alone.
- [ ] A cut nested inside another is concentric:
      `inner = outer - inset * (2 - sqrt(2))`, rounded (the selected segment
      cuts 5 inside a 7 track).
- [ ] Panes cut only their top corners; tablets are square apart from the
      notch.
- [ ] Hairlines are 1 dp and sit inside the outline.
- [ ] No shadow, gradient, glow or inset well. Overlays are one tone up with a
      hairline on the scrim.
- [ ] Any blur is a hint where it fits: under a picture or on a scrim, its
      lightness near the plane's, clipped to the cut, still at rest, and never
      on a control, type or a state.
- [ ] Charts are a fine line over a 45 degree hatch; no coloured area fills.

## Type

- [ ] Space Grotesk and Space Mono are embedded; no system fallback renders.
- [ ] Exactly one line per view uses `name`.
- [ ] Space Mono appears only on literal data.
- [ ] Sentence case throughout; no all caps.
- [ ] Sizes and line heights match the scale; nothing invented.
- [ ] Setting details and card secondary lines use `detail` in `fg2`;
      `caption` in `fg3` is kept for timestamps, counts, hints and
      placeholders.

## Space and layout

- [ ] Gaps come from 4, 8, 13, 21, 34, 55.
- [ ] Tablets are 56 (48 compact), 4 apart; controls 40 (32 small); fields 44.
- [ ] The three size classes change structure at 760 and 1280; the compact
      layout has a bottom bar, not a squeezed rail.
- [ ] Panes start 10 below the top edge and sit 8 apart on the ground.
- [ ] Work-pane body and tabs keep 34 side insets; mineral headers and
      Settings keep 36; full pages use the 760 column.
- [ ] With no selection, the work pane shows the Now board: fleet proof,
      pinned tiles, Needs you, Running now, This computer and Recent activity.
- [ ] Host meters use quiet `fg2` and turn amber at 90 percent.
- [ ] Draft forms are no wider than 600; from 820, an Aside previews what
      saving writes. Errors wait until a field was edited and left.
- [ ] The catalogue uses the full work-pane width and opens a system in a 720
      wide dialog.
- [ ] Settings has a scrolling SectionIndex at expanded and a Picker below
      it; never a strip of chips.
- [ ] Dialogs protect decisions; ordinary creation and editing stay in drafts.

## Motion

- [ ] One curve, chisel. No ease-in-out, no bounce.
- [ ] Every duration is from the table: press 90; hover 160 in and 100 out;
      focus 120 in and 100 out; select 240; field 320; navigate and unfold
      200; popover 160 in and 100 out; dialog 300 in and 160 out; wheel 140.
- [ ] Press chips plaque cuts 4 dp deeper and sinks content 1 dp. Rows and
      menu items dip instead.
- [ ] Selection, filter and segment fills glide; a new work pane rises 8 dp;
      Dialog rises 12 dp and drops 8 dp; Placard rises 8 dp.
- [ ] Reduced motion makes everything instant.
- [ ] With the pointer still and no job running, no frames are drawn.

## Behaviour

- [ ] Hovering or focusing a tablet swaps its claim for its action; Enter
      runs it; double click runs it.
- [ ] The selection slab moves between rows; the rail tile travels; the tab
      blade slides.
- [ ] Escape closes the topmost thing first and nothing else.
- [ ] Popover's full-window catcher and Dialog's scrim swallow the outside
      press that closes them; input beneath stays blocked through the exit.
- [ ] In a conversation no line of an answer ends in an ellipsis: text
      wraps, long addresses and commands break, tables wrap their wide
      columns.
- [ ] Text in a conversation can be marked and copied; the marks go when the
      keyboard leaves them.
- [ ] Code inside a sentence takes no room of its own, and a command an
      answer offers to run is shown once.
- [ ] A command run from an answer runs in the conversation: its output
      shows as it comes, a silent one counts its seconds, and Stop ends it.
- [ ] A key answers a waiting request only from an empty composer, only for a
      request on screen, and the composer says which key does what.
- [ ] A select with nine or more options is searched by typing, and an
      option's second line wraps instead of being cut.
- [ ] The choice that stops the asking shows in amber in the composer, not
      only inside its menu.
- [ ] Tab reaches every control; the ring shows only for keyboard focus; icon
      plaques have descriptions.
- [ ] Polling and repainting stop for what is hidden.
- [ ] This computer is polled only while the Now board is on screen: 3 s
      focused, 15 s visible and off otherwise.
- [ ] Poll intervals double on battery.

## Copy

- [ ] Claims come with proofs.
- [ ] The verb on the plaque is the verb on the row is the verb in Activity.
- [ ] Errors say what happened and what to do; empty states offer the next
      step.
- [ ] No exclamation marks, no emoji.

## Review artefacts

Attach to the review: screenshots at expanded, medium and compact; the same
page in a dark and a light palette, at Contrast high and with Tint on; the
output of the palette hookup check and its
[contrast targets](palette.md#contrast-targets) (§8 of
[palette.md](palette.md)); a note of any item above that was deliberately
not met and why.
