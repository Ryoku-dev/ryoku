# Game Bar

The Game Bar is Ryoku's in-game overlay: one passive surface per output that
sits above a fullscreen game, with a strip of widget buttons and up to eight
cards for capture, performance, system load, tasks, media, Discord voice, audio
and recent captures. `Super+Shift+G` toggles it on the focused monitor.

Everything is configured under `gamebar` in `~/.config/ryoku/shell.json` and
through **Ryoku Hub > Desktop > Game Bar**. Changes apply live; the shell never
restarts for them. The bar is meant for a game, but nothing requires one: it
opens over whatever is on that monitor.

The overlay takes the pointer only where the strip or an open card is drawn.
It never takes keyboard focus, so the game keeps input while the bar is visible.
The bar has no close key of its own; press `Super+Shift+G` again, or use the
same shortcut from the keybind catalogue (`ryoku-shell gamebar` toggles it from
a terminal).

## The strip

The strip is centred at the top of the screen. Each button shows one widget;
pressing it again hides that widget. The set of open widgets is remembered, so
the next press of the shortcut brings back the same arrangement.

To the right of the widget buttons, the grid button toggles snapping, then come
the clock and, on a machine with a battery, its charge. The strip is the only
part of the overlay that takes focus while the bar is open, and it hides itself
before a capture so it is never in its own screenshot or recording.

## Widgets

- **Capture** takes a region, window, monitor or all-monitors screenshot, and
  starts or stops the screen recorder with its elapsed time. It uses the same
  capture and recorder paths as the rest of the shell, not a second pipeline.
- **Performance** shows the machine's headroom while a game runs, with the
  resolution and refresh of the display's compositor mode. Ryoku does not read
  an in-game frame counter, so the card never claims one.
- **Resources** has a tab each for CPU, GPU and memory: the current reading in
  large numerals, the total it is out of, and a short graph.
- **Tasks** lists the five busiest processes by average CPU with their memory,
  and ends one through a two-step End. Desktop processes are protected, so the
  list cannot offer to kill the compositor or the shell it floats above.
- **Media** follows the one now-playing pick the whole shell shares, with art,
  title, artist and transport.
- **Voice** shows the Discord call (see below).
- **Audio** carries the same output and application faders as the audio menu.
  A level set here is the level PipeWire holds everywhere.
- **Gallery** shows the newest screenshots and recordings as the files
  themselves, with a switch between the two lists and a button to open the
  current folder.

## Placement

Drag a card by its frame. A card released near a screen edge, a centre line or
another card pulls to that line; when nothing is near it falls back to the
snapping grid. The guide grid appears while a drag can snap. The grid button in
the strip turns snapping on and off, and the choice is saved.

Only the Voice card resizes: drag its bottom-right grip. A card that does not
fit after a monitor change is pulled back onto the screen. Each card keeps its
own position and the Voice card keeps its size, both in the persisted state.

Snap reach, grid spacing, guide spacing and visibility, and the settle spring
are all adjustable; see **Settings**.

## Pinned overlay

The pin button in a card's title bar keeps that card after the bar closes.
Closing the bar leaves the pinned cards on screen and removes everything else.
Pinned cards are pictures while the bar is closed: they never take the pointer
or keyboard focus, so the game keeps both.

A pinned Voice card shrinks to a content-sized panel anchored to the screen
edge its full card sits against, so a card parked in a corner stays flush in
that corner instead of stopping at its full frame. It shows the channel, the
participants on an avatar grid, who is speaking, the mute and deafen state, and
the participant names if you keep them on. Because it is passive, change mute
or deafen from the Voice card in the open bar, not from the pinned panel.

Pinning is saved with the rest of the Game Bar state, so a pinned call survives
a shell restart. Turning the Game Bar off in the Hub closes it and leaves
nothing behind.

## Discord Voice

Voice reads the Discord client you are already in; it does not join a call or
need a bot. Two clients are supported, through two different paths.

### Official Discord

The bridge speaks Discord's local RPC. On the first connection the card asks
you to **Authorize Discord**: confirm the request inside Discord, and Ryoku can
then read the channel, participants and speaking state and can set mute and
deafen. The grant uses the public StreamKit voice RPC client with
`rpc`, `rpc.voice.read` and `rpc.voice.write` only. The resulting token is kept
in `~/.cache/ryoku/discord-voice.json` with mode `0600`; delete that file to
force a fresh authorization.

### Vesktop and Equibop

Vesktop's local RPC proxy answers Rich Presence but not authenticated voice
RPC, so the shell cannot read a call from it directly. The game bar ships a
small Vencord companion that reads the call inside the client and publishes the
same state over a Unix socket in `$XDG_RUNTIME_DIR`. The Voice card detects the
missing companion and offers **Install voice companion**.

The installer closes nothing for you: fully quit the client first, including
its tray icon. It then clones Vencord (or Equicord for Equibop) into
`~/.local/share/ryoku/discord-voice/`, copies the companion into
`src/userplugins/ryokuDiscordVoice`, installs dependencies and builds the
client. It writes the built `dist` path into the client's `state.json`
(`vencordDir` for Vesktop, `equicordDir` for Equibop) after saving a
`state.json.pre-ryoku-voice` backup. Start the client again and the plugin is
enabled by default.

The installer needs `git`, `node` and `npm`; when `pnpm` is absent it enables
it through corepack or installs it into the user prefix. The first build takes a
few minutes. Running the installer again pulls the source and rebuilds.

The bridge process runs only while the Voice widget is on screen, including
when it is pinned and the bar is closed. Hiding Voice stops the bridge; no
Discord socket is held when the widget is not in use.

## Settings

**Ryoku Hub > Desktop > Game Bar** has four tabs. Every control writes one key
under `gamebar` in `shell.json`, so the daemon persists it and the live shell
retunes without a restart.

| Key | Default | Effect |
|---|---|---|
| `enabled` | `true` | Show the Game Bar and bind the shortcut. |
| `dim` | `0.62` | How far the game darkens behind the open bar. |
| `scale` | `1` | Scale the strip, cards and text together. |
| `radius` | `14` | Rounding of the strip and card frames. |
| `fade` | `200` | Opening and closing duration in milliseconds. |
| `snap` | `true` | Pull a released card to edges, centre lines and other cards. |
| `snapReach` | `9` | How close a card must come before it snaps. |
| `snapGrid` | `24` | Fallback spacing when no line is near. |
| `guideGrid` | `true` | Draw the guide grid while a drag can snap. |
| `gridStep` | `48` | Spacing of the guide grid. |
| `gridAlpha` | `0.28` | Visibility of the guide grid. |
| `spring` | `3.2` | Settle stiffness after a drop. |
| `damping` | `0.58` | Settle damping. |
| `mass` | `1` | Settle weight. |
| `voiceAvatars` | `8` | Participant avatars shown before the list stops. |
| `voiceDim` | `true` | Fade participants who are not speaking. |
| `voiceDimOpacity` | `0.5` | How far a quiet participant fades. |
| `voiceNames` | `true` | Names under avatars in the pinned panel. |
| `voiceAvatarSize` | `52` | Avatar size in the pinned panel, in pixels. |
| `voiceBgOpacity` | `0.72` | Background opacity of the pinned panel. |
| `voiceControls` | `true` | Show mute and deafen state in the pinned panel. |

The compositor is untouched: the bar is a normal layer surface at overlay
level, with an exclusive zone of `-1` so it spans the bar and the dock.

## Troubleshooting

- **The shortcut does nothing.** Check `enabled` in the Hub, and that the Game
  Bar binding is still in **Keybinds**. `ryoku-shell gamebar` always toggles it
  when the shell is running.
- **The Voice card says Discord's voice socket is not reachable.** Start the
  client, wait for it to finish loading, then press Retry. The official client
  and Vesktop cannot both own the same RPC socket at once.
- **The card asks for a companion after Authorize.** The client is Vesktop or
  Equibop. Quit it fully, run **Install voice companion**, start it again.
- **The companion build fails.** It needs `git`, `node` and `npm`, and the
  client must be closed for the state file to be written. The last installer
  line stays in the card; rerun after fixing the tool or closing the client.
- **Authorization never completes.** Confirm the request inside Discord. If
  the prompt has gone stale, delete `~/.cache/ryoku/discord-voice.json` and
  authorize again.
- **The bridge stopped.** Press Retry; the shell restarts it with a backoff.
  If it keeps stopping, check that `python3` is on `PATH` and restart the shell.
- **A card is off screen.** Saved positions are clamped to the current output,
  but a large layout change can leave a card under the strip or at an edge;
  drag it back. Pinned cards reappear on the monitor saved in the state file.
- **A pinned card cannot be clicked.** That is deliberate. Pinned cards are
  passive while the bar is closed; open the bar to interact with them.

## Files

- `ryoku/shell/quickshell/shell/modules/gamebar/` the surface, strip, cards,
  look tokens, Discord bridge and companion, and their layout tests.
- `ryoku/shell/quickshell/shell/services/GameBarStore.qml` persisted state,
  card order, pins, positions and sizes.
- `ryoku/shell/quickshell/shell/services/DiscordVoice.qml` the bridge process,
  participant model, reconnect and installer control.
- `ryoku/shell/quickshell/shell/services/Config.qml` the `gamebar` defaults and
  `setGameBar`, which patches one key through the settings daemon.
- `ryoku/hub/quickshell/pages/GameBarPage.qml` and
  `ryoku/hub/quickshell/schema/GameBarPage.js` the Hub page and its rows.
- `ryoku/wm/binds.go` and each compositor's bind table the `shell.gamebar`
  shortcut; `ryoku/shell/quickshell/shell/shell.qml` maps it to the surface.
- `~/.local/state/ryoku/gamebar.json` the open flag, pinned set, card set,
  positions, sizes and remembered resource tab.
- `~/.local/share/ryoku/discord-voice/` the companion source build.
- `~/.cache/ryoku/discord-voice.json` the cached Discord token, mode `0600`.
