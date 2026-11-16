# Checklist

Use this to review a surface that claims to speak the Ryoku UI language: a
new app, a new page, a port of an existing screen. The Hub
(`ryoku/hub/quickshell/`) is the reference; put the surface beside the
`media/hub-*.png` renders and beside the Hub itself. Every "no" is a defect.
The words are defined in [design.md](design.md#2-vocabulary).

Capture the surface in each of the four styles; in Ryoku's colours and the
wallpaper's (on a colourful wallpaper); in dark and in light; at Contrast
standard and high and with Tint off and on; with the Japanese gloss on and
off; with reduced motion on and off; focused and unfocused.
[working.md](working.md#render-the-hub-privately) shows how to render
without touching the reader's screen.

## Styles and colours

The rule is [styles.md](styles.md).

- [ ] The surface draws in Lithic, Material, Adwaita and Fluent from
      `theme.design`, and reshapes at once when it changes.
- [ ] It wears Ryoku's colours by default and the wallpaper's when
      `theme.colours` says so, and changes at once.
- [ ] On Ryoku's colours nothing of the wallpaper shows, even while a
      wallpaper palette exists.
- [ ] Switching the style never changes the colours; switching the colours
      never changes the shapes.
- [ ] Dark and light both work in both colour sources and follow the
      desktop's mode (Dark, Light, Automatic, Sun).
- [ ] It has no style or colour picker of its own; at most it links to
      Settings › Style.
- [ ] It reloads its look when `shell.json`, `theme.json`, `colors.json` or
      `matugen.json` changes, as the Hub does (`ShellBody.qml:52-78`).

## Colour

- [ ] Every colour is a role read from `Lithic`; no colour value appears in
      chrome code. Values live in `tokens.json` only.
- [ ] Planes come from `Lithic.planes()`. In Lithic the sidebar (or its
      equivalent), the page and a slab are one paper, set apart by 1 px
      `vein` hairlines; no grey plane holds a lighter grey one.
- [ ] Text reads `fg2` for working text, `fg3` for faint notes and `fg` for
      titles, values and the selected or hovered.
- [ ] The accent (`sel`) marks place only: the current item's numeral, mark
      and gloss, its branch tick, the scroll marker, the section being read,
      lit counts; and the fills the library gives it (placard, toggle on,
      selected segment, slider handle).
- [ ] Gold fills one consequential action per view. Line plaques carry
      `goldInk`. Gold marks elsewhere are only those the Hub uses: Home's
      motto tick and carved word, card and binder rarity.
- [ ] States use their minerals with a word: jade alive, lapis becoming,
      amber needs you, jasper failed; errors are `jasper` detail text.
- [ ] Hover and press washes are alphas of `fg` from `Tone` or the library;
      disabled controls draw at `Lithic.disabledOpacity`.
- [ ] Text, hairlines and the selection read at both Contrast settings and
      every Tint, in dark and light ([palette.md](palette.md#contrast-targets)).

## Shape

- [ ] Every plane and control is drawn with the library's cut (`CutRect`,
      `Plaque`, `Slab`, ...); no `radius` is set by hand. In Lithic there is
      no rounded corner and no pill.
- [ ] Cuts come from the scale (2, 3, 4, 5, 6, 7, 8, 16; cards keep their
      own) and match the Hub's use: rows and icon buttons 6, plaques, fields
      and slabs 7, popovers and placards 8, the page pane 16 at the top and
      7 at the foot, dialogs 16.
- [ ] A press deepens a plaque's cut by 4 and sinks its content 1 px.
- [ ] Nested cuts are concentric (the segment cuts 5 inside a 7 track).
- [ ] Hairlines are 1 px, inside the outline: pane, slab, between setting
      rows, section rules.
- [ ] A quiet plaque shows no edge or fill at rest.
- [ ] No lit top edge on a plaque or plane; no vertical accent bar on the
      left of a selected option.
- [ ] No shadow on a plane, plaque, slab or dialog (the Hub's only shadows
      are the peek's hard offset and the card seal's); no gradient, glow or
      inset well; no blur in the chrome.
- [ ] Round marks are only where the Hub has them: section nodes and an
      Updates-style ready dot.
- [ ] Shine is restrained: any foil is smooth, never dotted; dither belongs
      to plates and art, not to shine.
- [ ] The logo is the current mark (`ryoku/assets/brand/logo-mark-v2.png`,
      drawn by `components/RyokuMark.qml`), never the old one.

## Type

- [ ] Only Geologica and Martian Mono render, from the library's bundled
      faces; icons are Material Symbols Sharp.
- [ ] Text uses the library's type roles at their sizes; page titles are
      `sentence` at 32 / 40, section titles `section`, row titles `row`,
      details `detail` in `fg2`.
- [ ] One line per view uses `name`, if any.
- [ ] Martian Mono carries data, numbering and running heads; Geologica
      carries everything a person reads.
- [ ] Spaced capitals appear only on running lines the Hub sets that way
      (folder names, sidebar title, masthead, spec-strip labels, summaries,
      card heads) and come from `font.capitalization` with the Hub's
      tracking; the strings themselves are sentence case.
- [ ] Indexes and counts are mono and zero-padded; live figures use tabular
      figures.
- [ ] Selection is marked by DemiBold where the Hub does (the selected leaf,
      the section being read).

## Gloss

- [ ] Every kanji gloss is gated on `UiPreferences.showJapaneseGloss` and
      disappears cleanly when it is off.
- [ ] Each gloss is the real word, taken from or added to
      `registry/registry.js`, set after its Latin name, smaller and fainter
      ([design.md](design.md#kanji-gloss)).
- [ ] Only seals (力, a badge's role kanji) stay when glosses are off.

## The sheet

For a surface with pages of settings or facts:

- [ ] A masthead runs over the page with the `§` number, the place in
      spaced capitals, its gloss and facts, over a tick ruler with a scroll
      marker.
- [ ] Everything listed is numbered: `00` home, `01` onward for groups,
      `01.3` for pages, `01 / 08` for sections.
- [ ] The page head has a title, a lead in `fg2`, and a spec strip of
      zero-padded counts; a live preview or a dithered plate sits at its
      right when there is room.
- [ ] Sections are ruled heads with their ordinal, over slabs of setting
      rows 13 apart with a hairline between.
- [ ] The column is centred and at most 960 wide; there is no section index
      on the page (sections hang in the navigation, as the section outline).
- [ ] The registration sheet lies under the page and the colophon closes it.
- [ ] A changed row shows "Changed" and a quiet reset plaque.

## Space and layout

- [ ] Gaps come from 4, 8, 13, 21, 34, 55; only frame offsets copied from
      the Hub sit outside the run.
- [ ] The window is a desktop window, not phone-shaped.
- [ ] Setting rows are at least 44 tall with the control right-aligned, at
      most 320 wide, 21 from the text; text wraps and never elides.
- [ ] Plaques are small (32) unless the view needs a normal one (40).
- [ ] Dialogs are `stone4`, cut 16, 520 wide by default, 34 padding, on the
      scrim; they close on Escape or a press outside, and any open overlay
      closes when the surface navigates away.
- [ ] An overview presents its live data with engraved charts, as Home's
      readings do.

## Components and behaviour

- [ ] Almost every action is a quiet plaque; one gold per view; `kind` is
      set on every plaque (the library's default is gold).
- [ ] Danger confirms use the `danger` tier with Cancel quiet; a typed name
      or a countdown where the Hub uses one.
- [ ] A change applies when it is made: no Save button, no action bar.
- [ ] Every change raises a placard with Undo that drains over 5 s and
      pauses on hover or focus; at most three stack; a drag or a held key
      updates its own placard. Ctrl+Z undoes.
- [ ] No tooltips: explanations come from a peek on rest or an info button's
      popover.
- [ ] Long work shows nothing for the first second, then `Working` with a
      word, then a `BladeProgress` with Cancel past 10 s.
- [ ] Segmented controls hold 2 to 5 options and also serve as tabs.
- [ ] Search opens on `/`, Ctrl+K or Ctrl+F; results are grouped by page,
      the matched words underlined, with a "Category › Page › Section" crumb;
      no results offer nearby pages.
- [ ] Tab reaches every control; keyboard focus shows the focus ring (or a
      1 px `fg2` stroke on Hub-drawn rows), pointer focus does not.
- [ ] An open dialog takes Escape first; otherwise Escape and Alt+Left step
      back one level at a time: search results, the search field, a
      sub-view, the page, Home, then the window (`Hub.qml:460-461`).
- [ ] Polling and sampling run only for what is on screen and stop when it
      is hidden.

## Motion

The values are in [motion.md](motion.md).

- [ ] Hover, press and selection use the Hub's durations: hover 160 in and
      100 out, press 90, Tone's quick 120, move 240, unfold 260, stagger 18.
- [ ] The Hub's curves: OutCubic for the frame's own motion, chisel inside
      library components; overshoot only where the Hub has it.
- [ ] Pointer follow uses an exponential chase that never overshoots and
      looks the same at 60 Hz and 165 Hz; nothing feels like 30 fps on a
      high-refresh screen.
- [ ] Page changes rise (fade with a 14 px slide); sections reveal in turn,
      the first five only, 40 ms apart.
- [ ] At rest, motion runs only inside the Ambient gate: the window is
      focused and motion is allowed. Unfocused, nothing ticks, scans or
      samples, and between beats no frames are drawn
      ([design.md](design.md#what-may-move-at-rest)).
- [ ] Every loop is finite or bound to live work: a pulse that rests, a
      progress sweep only while the job runs.
- [ ] Reduced motion makes every transition instant and turns Ambient off.

## Copy

The rules are [copy.md](copy.md).

- [ ] Sentence case in every string; capitals only through the font.
- [ ] Plain words addressed to you; no em-dash, no exclamation mark, no
      emoji, no mention of AI or design tools.
- [ ] Leads, notes, dialog text and errors end with a period; row details,
      titles and plaques do not.
- [ ] Plaques say their verb; a confirm's verb repeats its question; no
      "OK" or "Yes".
- [ ] Errors say what happened and what to do next.
- [ ] Every change has a notice ("%1 changed" or its own) with Undo.
- [ ] Separators are the Hub's: › in crumbs, / in the masthead, · in
      summaries, ・ between kanji.
- [ ] Every string goes through `I18n.tr`.

## Review artefacts

Attach to the review: captures of the surface in the four styles, in both
colour sources, dark and light, at Contrast high and with Tint on, with the
gloss off, and beside the matching `media/hub-*.png`; the output of the
palette hookup check ([palette.md](palette.md#8-verifying-the-hookup)); a
note that an unfocused window draws no frames; and a note of any item above
that was deliberately not met and why.
