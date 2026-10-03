# Frame bars

This document describes **Sumi**, Ryoku's built-in painted-frame bar: one
frame-bar system of four independent rails that share the monitor's frame
scene. When Sumi is the active style,
`ryoku/shell/quickshell/shell/modules/bar/Bar.qml` creates a `FrameRail` for each
edge and reads the normalized `frameBars` object from `~/.config/ryoku/shell.json`.

Sumi is no longer the shipped default. The default is **qsbar**, a separate
full-color top bar that loads from a self-contained folder under
`shell/modules/bar/barstyles/`. `Frame.qml` draws Sumi's rails only while
`barStyle` resolves to no folder scene (the id `"sumi"` or an empty value), and
any other id mounts that folder style's `Scene.qml` instead. For style
selection and qsbar, see `docs/barstyles.md`.

A rail is a thin interactive strip, not a second panel process. The monitor
overlay owns its input region and the corresponding exclusive-zone reserve, so
tiled windows clear exactly the enabled edge rails.

## Default profile

The shipped profile enables a compact top rail and a continuous left rail:

- **Top**: centred clock.
- **Left**: quick settings and workspaces at the top, dock in the centre, tray,
  network, and clock at the bottom.
- **Bottom and right**: configured but initially disabled.

Every edge has its own `enabled`, `size`, `reveal`, and three axis-appropriate
zones. Horizontal rails use `start`, `center`, and `end`; vertical rails use
`top`, `center`, and `bottom`. A zone holds its group against its own end of
the rail: a start zone hugs the leading edge, a centre zone sits on the rail's
midpoint, an end zone hugs the trailing edge. The runtime accepts only
catalogued widgets that fit the target axis.

## Bar Studio

Open **Bar Studio** with **Super+Period**. The shortcut records the Bar Studio
section before opening the guarded Ryoku Settings process.

Bar Studio stages a complete immutable `frameBars` object through the normal Hub
draft and Save flow, and its edits apply to the running desktop as you make them.
Pick an edge to work on that rail. It edits only what changes the running frame:

- the frame chrome the shell draws around the desktop: the draw toggle, the
  opacity, the band thickness, and the corner radius;
- each rail's own switches: on or off, and thickness;
- the widgets in each rail's three zones: add a catalogued widget that fits the
  rail's axis and is not already on it, remove one, or reorder within a zone.

The frame's material and colour come from the shell's look (`Theme` and the
palette), not a user knob; picking a different bar style is a separate choice,
covered in `docs/barstyles.md`.

Every change is live on the desktop at once. Save keeps it and rebaselines;
Revert, or closing the window with unsaved edits, walks the desktop back to the
saved state through the same channel. Bar Studio never writes configuration
files directly.

The bounded menus keep whatever
values are persisted: every Bar Studio edit clones the whole `frameBars` object,
so a subtree it does not touch is never dropped. They are configured through
their defaults and the catalogue, not edited on this page.

## Menus and popout cards

A rail widget reports its own rectangle to the monitor-local
`FrameMenuManager`, which owns one open surface per anchor per monitor. It
combines the widget and body mask regions so input lands where it should, and it
closes on Escape, a click outside, focus loss, or fullscreen.

Most status widgets open a popout card. Click the network, Bluetooth, battery,
audio, system-monitor, recording, or music widget and a card grows out of the
rail from the point you clicked, then melts back the same way when it closes.
The cards are not read-outs, they are the controls: audio is a full mixer
(output, input, per-app volume, and the Bluetooth codec), Bluetooth pairs and
connects devices and shows their battery, battery carries the gauge, the power
profiles, and a detail panel, network runs Wi-Fi, and the rest follow suit. They
share one skin from a card kit (`shell/modules/bar/popouts/PopoutCard.qml` and its siblings),
so every card opens, reads, and dismisses the same way.

Super+Escape and Super+S open the two push-aside sidebars. They are no
longer frame menus: their cards, catalog, and settings live in the sidebar
subsystem (`shell/modules/sidebar/`, see `docs/sidebars.md`). Adding a card
is one entry in `SidebarCatalog.js` plus one component under
`sidebar/cards/`; users reorder and hide cards from Ryoku Hub -> Desktop ->
Sidebars.

`ryoku-shell menu <id>` opens a catalogued menu on the active monitor;
`MenuCatalog.js` holds the valid IDs, and anything else is rejected before it
reaches Quickshell. Asking for the menu that already owns an anchor closes it,
so a rail button and its command read as one toggle, and a different menu at
the same anchor replaces it safely. The keyring prompt and the voice toast are
daemon-owned, so they replace rather than toggle.

A card clears the rail it grows from and draws above the rails, so its body
never hides under rail chrome. Voice, keyring, and enabled plugin surfaces share
this same manager scene.

## Extending frame bars

1. Add a catalogued widget or surface with an explicit axis/anchor contract.
2. Add the matching finite IPC route if it needs a command entry point.
3. Keep menu body work gated by its `open` state and release it on close.
4. Preserve owner rectangles, mask regions, monitor locality, and identity-safe
   close behavior.
5. Add the behavior test and Bar Studio label before exposing the new ID.

Do not introduce a parallel renderer, unbounded component loader, or direct
configuration writer.

## QS Bar layout (`qsbar.layout`)

The **qsbar** top bar (the shipped default; style selection lives in
`docs/barstyles.md`) keeps its widget order as data in `~/.config/ryoku/shell.json`
under `qsbar.layout`, not in a cache file:

```json
"qsbar": {
  "layout": {
    "version": 1,
    "left":   ["launcher", "workspaces", "status", "cpu", "volume", "memory", "ai"],
    "center": ["clock"],
    "right":  ["media", "quick", "network", "power", "battery", "brightness",
               "cputemp", "storage", "gpu", "bluetooth", "layout"]
  },
  "widgets": { "status": true, "power": false, "...": true }
}
```

- An **entry is a widget id**: a built-in id, or the manifest id of an installed
  plugin enabled on the bar. The order within a section is the order on the bar.
- The **built-in ids** (and their internal `G1..G19` slot gids, labels,
  visibility keys and own settings) are catalogued once in
  `shell/modules/bar/barstyles/qsbar/core/widgets.json`. That file is the single
  source the shell, the daemon (`ryoku-shell bar ...`) and Rashin read; nothing
  hand-lists widgets elsewhere. Built-in ids are reserved: `ryoku plugin add`
  refuses a manifest that claims one.
- **Visibility is separate from placement.** A built-in listed in the layout but
  `false` in `qsbar.widgets` (keyed by the catalogue's `visKey`) keeps its place
  and does not render. A plugin is shown when `~/.config/ryoku/plugins.json` has
  it `enabled` with `host: "topbarGlyph"`. Widgets with no `visKey` (launcher,
  workspaces, clock) are always shown.
- Every id occurs **at most once**; duplicates are dropped, and a known widget
  the layout omits is appended to `right` (a plugin honours its manifest
  `defaults.bar.section` when it names one). A plugin enabled after the layout was
  written lands at the end of its section.
- Separators (`qsbar.barSeps`), density (`qsbar.iconOnlyGids`) and per-widget
  colour (`qsbar.widgetColorStyles`) are presentation, keyed by gid, and are
  untouched by a layout move or reset.
- **Migration, once.** On first load with no `qsbar.layout`, the retired
  `~/.cache/quickshell_barorder_v2` string (`B:G1,B:G16,...|B:G8|B:G9,...`) is
  converted to ids through the catalogue's gid map, written to `qsbar.layout`,
  and the cache file is deleted; empty cells are dropped. With no cache file, the
  shipped default above is written instead. The `G1..G19` slot ids stay internal
  to `BarSlot.qml`.
- The layout is one document. A move rewrites the whole `qsbar.layout` through the
  daemon's settings store (the sole writer of `shell.json`); the bar re-reads on
  change and every monitor's bar follows.

## Now playing and the equalizer (qsbar)

Clicking the **qsbar** music widget (its title, its spectrum glyph, or anywhere on
it in the `full` style) opens the now-playing card,
`shell/modules/bar/barstyles/qsbar/panels/MprisPanel.qml`. The transport glyphs
beside the title keep their own clicks, so the widget stays a remote control and
the card is what a click on the widget itself asks for. Escape, the close mark, or
a click outside dismisses it, and Space toggles playback while it is focused.

The card is the record on the left (the artwork, the grooves, and the playback
spectrum ringing it) and the track on the right: title, artist, album, the output
device and the player it came from, a seekable progress bar, and the transport.
The spectrum is the shell's one analyser, the `AudioBars` service, so it obeys the
same policy as every other music surface and stays still under Power Saver, Game
Mode, and low-power mode rather than holding a 30fps feed open.

### The 10-band equalizer

Under the track sits a 10-band graphic equalizer: ISO octave centres from 31 Hz to
16 kHz, +/-12 dB each, with the eight presets (Flat, Bass, Treble, Vocal, Pop,
Rock, Jazz, Classic). Bands are live: a slider writes one PipeWire node param, so
the curve changes mid-note. `Flat` releases the equalizer entirely; the `ON`/`OFF`
chip beside the preset name is the bypass, for an A/B against the untouched
signal.

It is deliberately **not** a virtual sink. `ryoku/shell/scripts/ryoku-eq` renders a
`libpipewire-module-filter-chain` graph of ten `bq_peaking` biquads and runs it as
a WirePlumber **smart filter** (`filter.smart`), which WirePlumber splices between
every playback stream and the real device:

- Nothing has to be re-targeted. Applications keep playing to the default sink and
  the filter attaches itself, including streams that are already playing and
  players that name the default sink explicitly (mpv, so also Ryotunes).
- No second output device competes with the speakers. A smart filter never
  qualifies as a default node, and the shell's `Audio` service hides the filter
  pair from the mixer, so the output list and the per-app list read exactly as they
  did before.
- Off means gone. The filter lives in the `ryoku-eq.service` user unit and only
  runs while the equalizer is on, so a desktop that never opens the card carries no
  extra node, and `PartOf=pipewire.service` carries a PipeWire restart through to
  it. Releasing it is not a kill: a player that loses its sink mid-track does not
  wait for it to come back (mpv, and so Ryotunes, treats it as the end of the file
  and skips on), so `ryoku-eq` marks the filter disabled, which is what makes
  WirePlumber relink every stream straight to the device, and stops the process
  only once the graph says nothing is feeding it.

The state is one file, `~/.config/ryoku/equalizer.json`, owned by the script; the
`Equalizer` service watches it and re-arms the graph on login, so a curve set from
a shell, a keybind, or a second monitor's card shows up in the card in front of
you. The script is the whole interface, if you want it from a shell or a keybind:

```
ryoku-eq get                    state as JSON (bands, preset, on, running)
ryoku-eq set-band <1-10> <dB>   one band, -12..12, live
ryoku-eq preset <name>          Flat|Bass|Treble|Vocal|Pop|Rock|Jazz|Classic
ryoku-eq on | off               start or stop the filter
ryoku-eq apply                  make the graph match the saved state
ryoku-eq retarget               follow a new default sink
```

A new default output (headphones in, a Bluetooth speaker connecting) is handed to
the running filter as node metadata rather than a restart, so switching devices
never cuts the audio the filter is carrying.
