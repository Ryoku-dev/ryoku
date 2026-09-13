# Changelog: ryoku/shell/

## Unreleased

### Added

- **Palette Bridge settings now live in Win+W > Advanced > Matugen.** The
  existing Matugen controls stay on their own page, while a second page handles
  building the bridge, managing its service, checking its health, and setting up
  or removing the Spotify, Vesktop, and Zen integrations.

- **Kairos, a third built-in bar style: one dynamic island carrying the clock.**
  A single near-black pill floats at the top centre showing the time, and opens
  on hover into that clock over a rolling date wheel: the centred day is the
  selected one and the days either side turn away in perspective, scrollable by
  drag, wheel and the arrow keys. Each morph is one animated progress value, the
  clock's size steps to whole pixels (measured at a fixed size, so the resting
  width can never fight the growth) and the wheel's visuals are interpolated from
  each day's distance to the centre, so nothing switches at a threshold. The
  island owns its surface (near-black over the wallpaper, with a cached drop
  shadow) and reserves the resting band, so tiled windows clear it; the strip
  around it stays click-through. Pick it with `ryoku-shell barstyle kairos` or
  from Ryoku Settings' Bar Studio
  (`modules/bar/barstyles/kairos`, `services/BarProducts.qml`).

- **Kairos grows a music pill when a track plays.** A cover bubble appears just
  left of the clock -- a track only ever brings up the bubble, never a panel of
  its own. Hovering it peeks the island open far enough for the transport, and a
  click opens the full now-playing panel, which closes again as soon as the
  pointer leaves it -- no click needed. The panel's backdrop is the cover blurred
  into an even ambient wash,
  inset so the pill's near-black reads as a bezel around it, behind a low tint of
  the shell's accent, so it recolours with the wallpaper palette (matugen) and
  with a named scheme; over it sit the title, artist, album, player, a progress
  bar with elapsed and total time, and prev/play/next wired to the player. The
  clock stays the centre island: it rests on the screen's centre line and expands
  there, symmetric, drawing over the music bubble, while the music grows leftward
  from just beside it. Each island keeps its own hover target and hover-intent
  timer, the shadows sit a little deeper under both, and each pill carries a
  hairline rim so its rounded frame reads on a dark wallpaper where the
  pure-black fill would otherwise vanish into the desktop
  (`barstyles/kairos/components/{Island,MusicSurface}.qml`).

- **Kairos also ships an app launcher, and Super+Space grows the island into it.**
  The pill morphs from the RESTING island -- never from its hover size -- into the
  clock, the app search and the app list, and its window is mapped at full size so
  the surface never resizes mid-morph; the morph is armed only once that surface is
  on screen, so the growth is always visible. While it is open the bar island rests
  underneath it, and the input mask is the pill rather than the window, so the
  desktop around the island stays usable. Select it as **Kairos** in Ryoku
  Settings' App Launcher (`modules/launcher/variants/kairos`, `catalog.json`).

- **Rashin works with any coding agent now, not just Hermes.** The Hub's Rashin
  page and the dashboard both list your detected agents with a one-click Wire
  (it drops a pointer, the ryoku skill, and prowl-agent's code-intelligence
  skill into that agent), show every path Rashin exposes -- the skill, each
  vault map, prowl-agent -- and offer a Copy snippet to point an agent Rashin
  doesn't wire directly. The Super+S chat can run a coding agent other than
  Hermes when its ACP adapter is installed; Hermes stays the recommended
  default. `ryoku-rashin paths` and `ryoku-rashin agent` do the same in a
  terminal.

- **The Super+S chat header picks the agent and its model in one place.** The
  chip shows what is answering -- the agent, plus its model when it has one
  ("Hermes · gpt-5.6-luna", or just "Oh My Pi" for an agent that carries its own
  model). Tapping it opens a two-level picker: choose the agent (Hermes, Oh My
  Pi, and any other whose adapter is installed; the rest show "needs adapter"),
  and, for agents that expose a model list, the model. Switching the agent takes
  effect on your next message, and the chip no longer shows a stale model after
  a switch.

- **The Super+S needle opens with a clearer start.** Instead of a wall of text,
  the empty chat now leads with a heading, a one-line explainer, and three
  tap-to-fill example prompts, so it is obvious what to do first.

- **The needle guides first-time setup instead of failing.** On a box with no
  AI configured yet, the Super+S chat now shows a "Connect an AI" prompt with an
  Open setup button (straight to Ryoku Settings' Rashin page) rather than the
  example prompts, so a first ask never dead-ends on an error.

- **Quick asks are no longer locked to Hermes.** The launcher fast lane can run
  against any of ten built-in providers (OpenRouter, OpenAI, Groq, DeepSeek,
  Mistral, Together, xAI, Cerebras, Ollama, local). `ryoku-rashin backend
  <provider>[:model]` picks one (`backend auto` follows Hermes), and keys can
  live in `~/.config/ryoku/rashin.env` instead of Hermes's own `.env`, so quick
  asks work without a Hermes provider configured.

- **The qsbar music widget opens a now-playing card, with a 10-band equalizer.**
  Clicking the widget (its title, its spectrum glyph, or anywhere on it in the
  `full` style) opens the record and the track: artwork with the playback spectrum
  ringing it, title/artist/album, the output device and the source player, a
  seekable progress bar, and the transport. The old compact card is what grew into
  it, so it keeps the same anchor, reveal and dismissal, and it now draws the
  spectrum from the shell's one `AudioBars` analyser instead of spawning a private
  cava that ignored the Power Saver policy.

  Under the track is the equalizer: ISO octave bands from 31 Hz to 16 kHz, +/-12 dB
  each, the eight presets (Flat, Bass, Treble, Vocal, Pop, Rock, Jazz, Classic),
  and an ON/OFF bypass. `scripts/ryoku-eq` renders ten `bq_peaking` biquads into a
  `libpipewire-module-filter-chain` graph and runs it as a WirePlumber **smart
  filter**, which is spliced between every playback stream and the real device: no
  second output device to pick, already-playing streams included, and players that
  name the default sink themselves (mpv, so also Ryotunes) caught too. Bands are
  live node params, so a slider changes the sound mid-note; `Flat` or OFF releases
  the filter entirely, so a desktop that never opens the card carries no extra
  node. Releasing it is done the way WirePlumber intends, and not by killing the
  process: a player that loses its sink mid-track does not wait for it to come
  back (mpv, so also Ryotunes, treats it as the end of the file and skips on), so
  the filter is marked disabled, which relinks every stream straight to the
  device, and the process is stopped only once the graph says nothing is feeding
  it. The filter runs in the `ryoku-eq.service` user unit, state lives in
  `~/.config/ryoku/equalizer.json` and is watched, so a curve set from a shell or
  a second monitor's card shows up in the one in front of you. The `Equalizer`
  service is what the card binds to, and `Audio.qml` hides the filter pair from
  the mixer's device and app lists
  (`services/Equalizer.qml`, `panels/MprisPanel.qml`, `docs/bar.md`).

- **`deploy.sh` lays the `ryoku-gpu-trim` initramfs hook.** The shipped HOOKS
  drop-in names it, and mkinitcpio aborts on a hook it cannot find, so a dev
  checkout needs the file before `ryoku-boot-apply` rebuilds the images. It also
  keeps the denylisted nouveau driver, and the ~107 MiB of GSP firmware it
  pulls, out of every kernel image. `--overwrite` in the `ryotunes` step grew
  the matching `/usr/lib/initcpio/install/ryoku-*` glob.

- **Depth and Parallax are one feature now: Stage.** The old Depth (a still
  subject cut in front of the widgets) and the unreleased Parallax (the subject
  and extra layers drifting with the cursor over a recoloured backdrop) are the
  two effects of one **Stage** tab, backed by one engine (`scripts/ryostage`),
  one settings file (`~/.config/ryoku/stage.json`), one per-wallpaper registry
  (`stage-walls.json`) and one artifact folder (`~/Pictures/Stage/<stem>/`). The
  doctor migrates every old settings file, state cache and quick-settings rail in
  place (`ipc/stage.go`, `modules/stage/`, `docs/stage.md`).

### Removed
- **The Spotify Canvas relay is gone.** The music daemon no longer runs a
  loopback HTTP listener for the retired spicetify extension. The per-song
  backdrop still plays a clip you keep in `~/.config/ryoku/canvas/<id>.<ext>`
  (`ipc/music.go`).

- **Ryogami's dead `awww` settings are gone.** `paper.engine` and the thirteen
  `paper.awww.*` transition keys (type, duration, fps, step, angle, wave size,
  position, bezier, invertY, filter, fill colour) were read into `Config.qml`
  properties nothing had bound since the cutover: transitions come from
  `transition.shader` and the built-in engine. Nothing wrote the keys, so no
  config migrates (`ryogami/wall-ui/qml/Config.qml`).

### Fixed
- **Turning on the Cobalt download engine no longer adds you to the `docker`
  group.** Docker group membership is passwordless root for every process in
  your session, so a GUI toggle should never grant it. The engine already does
  all its container work as root through a tightly-scoped polkit helper, so the
  membership was pure convenience and is gone: enabling Cobalt now grants your
  session no docker access of its own. If you want plain `docker` on the command
  line you can still add yourself by hand. Fresh installs were never in the
  group; this only affects boxes that had switched Cobalt on.
- **The bar AI usage pill works with OpenCode again, and refreshes faster.** The
  `opencode-usage` collector read a long-gone `opencode.db`; current OpenCode
  keeps per-message JSON under `storage/message/<session>/*.json`. It now walks
  that (and the legacy `session/message` path), falling back to the old sqlite
  only if present, so the OpenCode chip fills from real sessions. The collector
  timer also runs 45s after boot and every 5 min instead of 2 min/10 min, so the
  pill is fresher (`bin/opencode-usage`, `systemd/user/ryoku-ai-usage.timer`).
- **A dev/checkout build now installs the translation catalog, so the Hub's
  language and regional-format pickers list every shipped language instead of
  just Auto and the two English locales.** `deploy.sh` and `dev-run.sh` copied
  the UI module but never the i18n catalog, so on a source build `I18n` found no
  `langs.json` and fell back to an empty language table. Both now run
  `ryoku/i18n/tools/install.sh`, landing `langs.json` and the catalogs at
  `~/.local/share/ryoku/i18n` (a packaged system already ships them to
  `/usr/share/ryoku/i18n`).
- **The now-playing spectrum follows the wallpaper.** With Follow System on, the
  bar retinted on a wallpaper change but the qsbar spectrum kept its old
  gradient: `cavaPalette` was an imperative snapshot taken when the panel
  loaded, and its canvas only repainted when that snapshot changed. It is a
  binding on the live wallpaper slots now, with a named theme's own `cava_theme`
  gradient still winning when one is set
  (`quickshell/shell/modules/bar/barstyles/qsbar/panels/MprisPanel.qml`).
- **A retint no longer wipes your ghostty config.** `matugen/apps.toml` rendered
  the palette straight onto `~/.config/ghostty/config`, ghostty's own config
  file, so every wallpaper change, theme switch and update overwrote whatever
  the user had put there (and overwrote a `user_edits/ghostty/config` fork right
  after materialize laid it down, which is why the documented workaround did not
  work either). matugen now writes only the palette, to
  `~/.config/ghostty/ryoku-colors`, and the shipped `config` pulls it in with
  `config-file = ryoku-colors` plus an optional `user.conf` for overrides: the
  same split kitty has always used (`matugen/apps.toml`,
  `matugen/templates/ghostty.conf`, `apps/ghostty/`, `deploy.sh`).
- **The power profile you pick is remembered again.** Two defects sent every
  session back to performance. Game mode's own `powerprofilesctl set
  performance` looked exactly like a user pick, so it was written to
  `power-profile.json`, and a game-mode session ended by a relogin never wrote
  the old profile back, leaving the store corrupted for good. And restore read
  ppd's active profile once at daemon start: when ppd published its platform
  default a beat later, the desktop landed on that default and banked it. The
  daemon now ignores profile changes while game mode holds the profile,
  re-asserts the saved pick over a late ppd default for the first seconds of a
  session, and banks a pick made through the shell's own call the moment it
  succeeds, so the menu, the bar widget and the battery popout all persist
  (`ipc/powerprofiles.go`, `ipc/autoprofile.go`).
- **The reload cover renders a `~`-based custom asset (#146).** The reload
  cover built its media URL as a bare `"file://" + path`, so a `reloadCover`
  path carrying a leading `~` (a hand-edited or ported `brand.json`) became
  `file://~/...`, which never resolves, and the reload silently showed the
  default wordmark instead of the chosen asset. The renderer now expands a
  leading `~` to `$HOME` before building the URL, the way the shell already
  resolves the brand mark, for both the image and the video path
  (`quickshell/reload-cover/ReloadMedia.qml`).
- **The Super+S chat can approve a tool.** When hermes paused on an edit or a
  command, the sidebar only showed "waiting for approval" with no way to
  answer, so the turn dead-ended unless the dashboard was open. The permission
  frame now carries hermes's options through `ryoku-rashin chat`, the bubble
  shows them as buttons, and a pick answers over the new `--perm <id>
  <option>` flag (`services/Needle.qml`, `modules/bar/panel/PanelChat.qml`,
  `rashin/backend/chatcli.go`).
- **In-shell video wallpapers can play sound again (#139).** The in-shell
  engine's QtMultimedia player was hardwired to `AudioOutput { muted: true }`,
  so a live wallpaper stayed silent even with audio turned on in the picker.
  The daemon now carries the picker's mute and volume on the `wallpaper` frame
  (new `mute`/`volume` keys, taken from the per-output apply maps or the
  wall-ui `wallpaperMute`/`wallpaperVolume` defaults), persists both in
  `outputs.json`, and republishes the live frame on `wall.set_audio` so a
  running clip changes at once; the backdrop binds them to its player. The
  in-shell transcode cache keeps its audio track now (the `-an` that stripped
  it is gone, and the cache key changed so stale silent re-encodes are
  ignored). The ryogami C player stays intentionally silent
  (`ryogami/daemon/`, `modules/wallpaper/`).
- **A silent bar sits still again, and the GPU drift is paced (#60, third
  round).** The GPU gap animation (`StreamShader.qml`) drove its shader clock
  with a `FrameAnimation`, which makes Quickshell render and commit a frame
  every vsync whether or not the picture changed, on the full-screen bar
  layer, on every profile. The shader itself costs nothing; the frames do:
  each one is a compositor frame, and on a 165 Hz hybrid laptop rendering on
  the discrete GPU (reverse PRIME, ~7 ms a frame) that held Hyprland at
  ~38% of a core and the shell at ~26%, silent, all day. The clock is now a
  Timer at the paces the Canvas stream already used: 30 fps (60 for the fast
  modes 5 and 6) while audio drives it, 20 fps for the silent drift, and the
  silent drift only on Performance (`Perf.ambientMotion`), so Balanced and
  Saver idle still like caelestia and end-4. The battery's charging shimmer
  follows the same rule: it was an infinite sweep at vsync whenever the
  laptop was plugged in below full, now Performance-only and one sweep every
  few seconds; the indigo body, wash and bolt still say "charging". Measured
  on the dev box, silent: shell 26% -> 5% (Balanced) / 11% (Performance),
  the shell's share of Hyprland ~56% -> ~17% / ~39%
  (`modules/bar/barstyles/qsbar/modules/StreamShader.qml`,
  `BatteryWidget.qml`, `ReactorLayer.qml`).
- **Quickshell's runtime logs can no longer eat the RAM.** Quickshell keeps
  a per-instance directory under `$XDG_RUNTIME_DIR` (a tmpfs, so memory) with
  two unbounded logs and never removes it; one warning storm wrote 4.3 GB
  there and a hundred dead instances kept their logs after it, which reads as
  "the shell uses gigabytes" and starves every socket in the runtime dir. The
  daemon now prunes dead instance directories at start and every five
  minutes and truncates a live log past 32 MB (`ipc/qsruntime.go`).
- **The wallpaper picker no longer holds 400 MB while hidden.** Its QML tree
  (thumbnails, browsers, previews) was built at daemon boot and kept for the
  session. It is now built on the first Super+W and torn down a second after
  the picker closes; the process stays warm, so a reopen is instant (36 ms
  measured) and an idle picker sits at about 160 MB instead of 410
  (`ryogami/wall-ui/shell.qml`).
- **The desktop lands on a wallpaper when none was ever recorded (#149).**
  Ryogami painted nothing on startup when no choice was stored, so a fresh
  install -- or a box cut over from awww without ever setting one through
  Ryoku -- sat on the empty grey frame, and went black the moment the doctor
  retired a hand-started awww-daemon. The startup restore now paints the first
  static image in the wallpaper directory when nothing is recorded and persists
  it, so the next login restores that choice; a clip is never picked, so the
  fallback stays off the live player on every GPU (`ryogami/daemon/apply.go`,
  `ryogami/daemon/daemon.go`).

### Changed
- **The Ryogami picker closes once something is applied.** Wallpaper, video,
  Wallpaper Engine scene, theme or rice: the picker leaves as soon as the
  apply lands. "Close on apply" in the picker's settings turns it back into
  a stay-open browser (`ryogami/wall-ui/`).

### Fixed
- **The wallpaper reappears after a reboot even when its file or the monitors
  arrive late.** The daemon restored the saved wallpaper (static or live) in a
  single pass at startup and silently gave up if the file the choice named was
  not yet readable or the compositor's outputs were not yet enumerable -- a
  login-time race that left the desktop on the grey Hyprland default until the
  next manual set. The startup restore now retries briefly while the desktop is
  bare, and a Hyprland event-socket watcher re-spans a live wall onto an output
  that comes up after the first pass (a login race, a hotplug, a panel that
  enumerates late). A box upgraded across the Ryogami split also carries its
  pre-split wallpaper choice into the daemon's store on first start, so the
  wallpaper survives the update instead of coming up grey (`ryogami/daemon/apply.go`,
  `ryogami/daemon/restore_watch.go`, `ryogami/daemon/daemon.go`).
- **The launcher returns to its exact idle size after a query is cleared.** On a
  monitor with a non-default interface scale (`displays.ui_scale`), the hero
  launcher opened at one height but settled a few pixels shorter once you typed
  and erased back to empty, so the idle card visibly jumped. The initial open
  path sized the card without the per-monitor UI scale the surface actually
  renders at; it now folds in the same `uiScaleFor` factor, so a fresh open and a
  cleared query land at the identical height
  (`modules/launcher/variants/hero/Main.qml`).
- **The Bluetooth widget no longer leaks `bluetoothctl` processes.** On a box
  with no adapter the widget polled `bluetoothctl show` on a timer even while
  the widget was disabled; without BlueZ the call never returned, so each tick
  orphaned the previous one and dead processes piled into the hundreds, leaking
  RAM. The widget now reads state straight off `Quickshell.Bluetooth` (BlueZ over
  D-Bus) with no polling and no process at all, so an adapterless machine spawns
  nothing (`modules/bar/barstyles/qsbar/modules/BluetoothWidget.qml`).
- **Bluetooth device names load, and pairing a fresh device works.** The qsbar
  panel parsed `bluetoothctl devices` output, which left names blank, and its
  pair step ran `trust`/`pair`/`connect` with no pairing agent, so a new mouse
  never bonded. The panel now lists devices from `Quickshell.Bluetooth` so names
  come from BlueZ, and pairing shells one `bluetoothctl` that brings its own
  `NoInputNoOutput` agent, then trusts, connects and reports the failure text to
  the panel instead of failing silently. The frame-bar popout pairs through the
  same path (`modules/bar/barstyles/qsbar/panels/BluetoothPanel.qml`,
  `modules/bar/popouts/BluetoothPopout.qml`, `services/BtLink.qml`).
- **A pinned dock icon no longer resets to a generic gear.** The dock resolved
  each icon once through a plain function call, so a pin whose icon was not yet
  findable at first paint -- a fresh boot before the icon-theme cache warms, or
  right after an app (e.g. Zen Browser) updates its `.desktop`/icon -- stuck on the
  generic `application-x-executable` fallback until a shell reload. Icon
  resolution now re-runs reactively: on any desktop-database change and via a
  bounded warm-up poll after load, so a pin recovers its real icon on its own. Both
  the first-class dock and the qsbar rail dock go through the one resolver
  (`services/Dock.qml`, `modules/bar/framebars/widgets/RailDock.qml`).
- **The overview stops trying to decode a live wallpaper as an image.** Each
  workspace cell drew the current wallpaper as a still `Image` backdrop, but a
  live (video) wallpaper -- `.mp4`/`.webm`/`.mkv`/`.mov` -- has no still frame, so
  every cell logged "Unsupported image format" and drew nothing. A video
  wallpaper is now treated as no backdrop (the cell keeps its flat fill), matching
  the `Session.wallIsVideo` check (`modules/overview/Singletons/Config.qml`).
- **The bar no longer spawns `makoctl` forever on a box without mako.** The qsbar
  do-not-disturb probe shelled out to `makoctl mode` on every status refresh;
  Ryoku runs its own notification server and never ships mako, so the process
  failed to start on repeat, spamming the log. The probe is now gated on a
  one-shot `makoctl` PATH check, so it runs only where mako is actually installed
  (`modules/bar/barstyles/qsbar/Theme.qml`).
- **A dismissed notification no longer throws while its card animates out.**
  `Notifs.timeLabel()` dereferenced the notification (`n.id`) without a null
  guard, and the card passes `card.notif`, which is null once the service has
  dropped a toast still easing off screen; it now returns "" for a null
  notification (`services/Notifs.qml`).
- **A dev deploy no longer resets the fastfetch emblem (or the kitty
  palette).** `deploy.sh` re-copied `fastfetch/config.jsonc` and
  `kitty/current-theme.conf` on every run, so each `ryoku update` on a
  checkout box put the readout back on the shipped emblem and dropped an
  imported logo or a decor picked in the Hub. Both are now seeded once and
  never re-laid, the same generatedSeed set `ryoku materialize` honours on a
  packaged box (`deploy.sh`).
- **KDE apps can follow the wallpaper palette through kdeglobals.** Dolphin,
  Ark, Gwenview and Kate resolve their colours through KColorScheme and
  `~/.config/kdeglobals`, which the qt6ct palette never reached, so under the
  KDE platform theme they painted at Qt's defaults: white file names on a white
  view in icon and compact mode, and rows striping light and dark in details.
  The palette now renders KDE's colour groups and the daemon merges them into
  kdeglobals, claiming only the colour groups so the fonts, icon theme and
  widget style a user set there survive. Ryoku keeps `qt6ct` as the platform
  theme, so a user gets these colours by installing `plasma-integration` and
  setting `QT_QPA_PLATFORMTHEME=kde`. The qt5ct and Kvantum outputs nothing
  consumed are gone (`matugen/templates/kdeglobals`, `ipc/matugen.go`).

### Added
- **The update island and the Hub's Updates page name the release line.**
  The Hub shows "Ryoku Onogoro" over the version pair, and both say
  "Onogoro v0.56.x -> Amaterasu v1.0.0" when the channel serves the next line
  (`services/Updates.qml`, `hub/quickshell/Singletons/Updates.qml`,
  `hub/quickshell/pages/UpdatesPage.qml`, `UpdateWidget.qml`).
- **The shell daemon records a good boot for the boot guard.** Once the shell
  surface has stayed up 45 s (past the supervisor's crash window) the daemon
  writes the boot id to `/var/lib/ryoku/boot/ok-<uid>`; `ryoku boot-guard`
  reads it on the next boot to tell an update that came back from one that
  did not. Best effort: a box whose `ryoku` package predates the directory
  simply records nothing (`ipc/bootok.go`).
- **A dev checkout takes its packaged externals from the `[ryoku]` channel its
  branch publishes to.** `deploy.sh` trusts the release key from the checkout,
  adds or repoints the `[ryoku]` stanza (`unstable-dev` -> testing, `main` ->
  stable, a foreign server left alone) and installs `ryotunes` with
  `pacman -Syu --needed`, retiring the locally built copy, so a dev box runs
  the same signed package a user gets instead of a minutes-long local build
  that could differ from it (`deploy.sh`).

- **QS Bar Settings has an Identity tab again.** The launcher mark (wordmark or
  glyph, with the full word and glyph grids drawn as the bar draws them) and
  the workspaces (count, marker style, live preview) were folded into two rows
  of the Widgets list, which only had room for a handful of the options. They
  are the bar's identity, so they sit in one route between Bar and Layout, and
  the old `logo` / `spaces` route ids land there. The Launcher and Workspaces
  rows in Widgets now point at it instead of carrying a cut-down copy
  (`qsbar/controlcenter/routes/IdentityRoute.qml`, `kit/Routes.js`,
  `core/widgets.json`).

### Fixed
- **The audio graph no longer touches Quickshell's Pipewire structures mid-teardown.**
  The shell already fed every audio Repeater from a debounced snapshot so a view
  never rebuilds inside a node-removal dispatch, but the singleton's
  `PwObjectTracker` still bound its object set to the live lists, so a mass audio
  reset (a device flap, "Device or resource busy") that destroys every node rewrote
  the tracked set on every single removal, inside the same dispatch. That is a
  reported Quickshell segfault: a binding write into the Pipewire object tracker
  while a node is being freed. The tracker now follows the settled snapshots (the
  two default devices stay live so the bar volume reads instantly), so it never
  re-tracks across a dying node and costs no immediacy, since no view shows a node
  before it reaches the settled list (`services/Audio.qml`).
- **The Rashin chat's skill list now shows the `ryoku` skill.** The sidebar
  listed slash-able skills by walking `~/.hermes/skills`, which does not follow
  the symlink `wire` lays for the shipped skill, so Hermes had the skill but the
  chat never offered it. The walk reads through symlinked skill dirs and their
  bundles; `ryoku update` also re-runs `ryoku-rashin wire` after every reindex,
  so an existing box picks up Prowl's skills for Hermes (`rashin/backend/chatcli.go`,
  `cli/internal/updater/update.go`).

### Added
- **A bar plugin can open a panel under its glyph.** A plugin that ships
  `entryPoints.panel` (a `content/Panel.qml`) opens it in a shared
  `PluginPanel` window on the same connected surface the built-in panels use:
  one panel at a time, Escape or a click outside closes it, and opening
  Network or Battery closes it the way they close each other. `pluginApi` gains
  `stateDir` (`$XDG_STATE_HOME/ryoku/plugins/<id>`, created on load),
  `saveSetting(key, value)` (through `ryoku-plugins-place`, so `plugins.json`
  keeps its one writer), and on the bar `panelOpen`, `openPanel()`,
  `closePanel()`, `togglePanel()`. A bar row now rebuilds only when its widget
  order really changed, so a settings write no longer restarts every plugin on
  the bar (`qsbar/panels/PluginPanel.qml`, `BarSlot.qml`, `Theme.qml`).

### Fixed
- **QS Bar Settings: the head no longer overlaps, the last row is no longer
  buried, Depth lifts the bar, and every switch is its own size.** The route
  name was baseline-anchored inside a Row and rode up into the QS BAR //
  SETTINGS eyebrow on every page; the bottom fade covered the last 24px of
  every page whether or not it scrolled (Depth, Storage and the RIGHT lane sat
  under it) and now only shows while there is more below, with a tail so the
  last row scrolls clear; the Depth switch only shadowed the popouts, so it now
  deepens the bar shell's own shadow too; a switch, segment or stepper inside a
  widget's expansion was stretched to the column by its Loader. Gap animation
  moved from the last card to the second, the Layout route is three lanes side
  by side (a list per lane, each row with its own switch, a fixed strip beneath
  to move the pick up, down or across), and every summary and description is
  a few words so nothing truncates (`qsbar/controlcenter/`, `Theme.qml`,
  `BarSlot.qml`).

### Added
- **QS Bar Settings > Community warns, and runs the plugin CLI in place.** The
  route opens with the community warning (Ryoku does not review or maintain
  these; they run in your shell with your permissions), every community row
  carries EXPORT, SHARE TO RYOSTORE and REMOVE, and the ADD field takes a git URL
  or a local folder. Each action runs `ryoku plugin ...` here and prints its
  result in a console strip (with OPEN FOLDER / OPEN PULL REQUEST when there is
  one), so a bad URL or a missing preview is never silent. A plugin's `int`
  setting now shows its number beside the stepper
  (`controlcenter/routes/CommunityRoute.qml`, `kit/CcWidgetList.qml`).

- **A Community section in QS Bar Settings' rail.** Every installed bar plugin
  that is not Ryoku's own (a manifest without `"official": true`) lists there
  instead of among the shipped widgets, with the same switch, density, colour
  and settings rows, plus its author, version and a REMOVE action; the route
  also takes a git URL (`ryoku plugin add <url> --bar --yes`) or opens
  Ryostore's plugin shelf. `ryoku-shell bar catalog --json` now carries
  `official`, `author` and `version` (`controlcenter/routes/CommunityRoute.qml`,
  `kit/CcWidgetList.qml`, `ipc/bar.go`).

- **Rashin's vault now routes an agent to the `ryoku` skill and the plugin
  path.** Asked for a new bar widget, Hermes read the shell's QML source (on a
  dev box, the checkout) because nothing in the vault said the skill existed or
  that a widget is a plugin. `desktop.md` now opens with "Acting on this
  desktop" (commands, never shipped-file edits; the skill's three files and
  their resolved path) and its bar section ends with "Adding a widget"
  (`plugins.md`, `ryoku plugin validate`, `ryoku plugin add <dir|url> --bar
  --yes`, where it lands); a fresh vault's `AGENTS.md` says the same in two
  lines (`ryoku/rashin/backend/index.go`, `vault.go`).


### Added
- **Rashin gives every agent the desktop map: a shipped `ryoku` skill and a
  config source mirror.** `ryoku/rashin/skills/ryoku/` (`SKILL.md`, `bar.md`,
  `plugins.md`) is the agent skill (safety rules, the command catalogue, the QS
  Bar and dock guide, the plugin contract); `ryoku-rashin wire` symlinks it into
  every agent's skills dir (`~/.agents`, `~/.claude`, `~/.codex`, `~/.omp/agent`,
  `~/.hermes`, each `~/.hermes/profiles/*`), `unwire` removes only those links,
  and `status --json` gained a `skillWired` flag per agent. The vault's
  `desktop.md` now carries a generated "Bar and dock" section (the widget ids,
  visibility keys, and the `ryoku-shell bar`/`dock` crib) built from the qsbar
  widget catalogue and, when the daemon answers, the installed bar plugins. On
  every reindex, when prowl-agent is present, Rashin mirrors `~/.config/quickshell`,
  `~/.config/hypr`, and `~/.config/ryoku/*.json` into
  `~/.local/share/ryoku/rashin/source/` and indexes it, so `prowlRepo()` and
  `search_code` answer on a packaged box with no checkout, not only on a dev
  machine (`rashin/backend/agents.go`, `index.go`, `vault.go`, `prowl.go`,
  `sourcemirror.go`, `skills/ryoku/`).
- **Prowl ships with the desktop, and Rashin sets it up.** `prowl-agent` (the
  code-intelligence indexer and MCP server the agent brain reads source with) is
  now a signed `[ryoku]` package that `ryoku-rashin` depends on, so every rashin
  box has it instead of a hand-install. `ryoku-rashin index` now runs
  `prowl-agent init --integrations agents,agent-skills,claude,omp` in the config
  mirror (was `none`, under a 120s budget), so the mirror carries Prowl's
  `AGENTS.md` block, MCP config and skills next to its code index, and
  `ryoku-rashin wire` also runs `prowl-agent skills --yes --clients <detected>`
  for the claude, omp and hermes clients it detects (skipped on a Prowl without
  the non-interactive `--yes`), installing Prowl's own skill beside the `ryoku`
  one (`rashin/backend/sourcemirror.go`, `agents.go`).
- **A wallpaper video engine toggle (`wallpaper.video_engine`).** Video
  wallpapers now play through one of two engines: `ryogami` (the default, the
  lightweight C player that decodes a cached transcode into `wl_shm` on its own
  background layer while the shell yields to it) or `in_shell` (the shell's own
  QtMultimedia player, which decodes the clip inside the wallpaper surface).
  The switch is in the wall-ui picker's Live wallpapers card and in shell.json.
  Behind the `in_shell` engine there are two resource knobs: `video_enabled`
  (off keeps only the still, the cheapest option) and `video_transcode` with
  its `video_transcode_fps`/`video_transcode_width` caps, which re-encodes the
  clip once to a bite-sized cached mp4 instead of decoding the full-res source.
  Animated image formats (gif, animated webp/apng/avif) are transcoded to mp4
  at scan time so the in-shell player advances their frames
  (`modules/wallpaper/`, `ryogami/daemon/`, `ryogami/wall-ui/`,
  `ipc/settings.go`).
- **A bar and dock CLI on the daemon (`ryoku-shell bar ...` and
  `ryoku-shell dock ...`).** The bar layout is now data a script or an agent can
  read and change without touching JSON. `bar list [--json]` reports every
  widget with its section, index and shown flag; `bar catalog [--json]` merges
  the built-in widget catalogue with installed bar-capable plugins and their
  settings schema; `bar move <id> --section <s> [--index N|--before <id>|--after
  <id>]`, `bar show|hide <id>`, `bar set <id> <key> <value>`, `bar position
  top|bottom`, `bar form full|fit|dock|notch|islands`, `bar defaults` and `bar
  settings [route]` mutate placement, visibility and presentation. `dock
  show|hide`, `dock edge`, `dock autohide on|off`, `dock pin|unpin <app>` and
  `dock list [--json]` drive the dock. Every mutation is validated against the
  catalogue (unknown id, unknown key, option out of list are errors, never a
  silent write) and written through the settings store, the sole writer of
  shell.json; a plugin's visibility and settings go through `ryoku-plugins-place`
  (`ipc/bar.go`, `ipc/daemon.go`, `ipc/main.go`).

### Changed
- **The QS Bar's widget order is data in shell.json, and store plugins ride the
  bar as first-class widgets.** The order and membership of the three lanes now
  lives in `shell.json` under `qsbar.layout` (`{version, left, center, right}` of
  widget ids), replacing the opaque `~/.cache/quickshell_barorder_v2` cache,
  which is migrated once on first load (its `B:/E:` gid string is converted to
  ids through the shipped widget catalogue, then the cache is deleted); a fresh
  box gets the shipped default. A new `qsbar/core/widgets.json` catalogue names
  every built-in widget once (id, gid, label, visibility key, its own settings)
  and is the single source the shell, the daemon CLI and Rashin read. An
  installed plugin enabled with host `topbarGlyph` is now a layout entry rendered
  through the same drag-reorderable slot as a built-in (the fixed plugin glyph
  strip is gone), so it moves, hides and takes width budget like any widget.
  Visibility stays separate from placement: a hidden built-in keeps its place,
  and every write goes through the daemon's settings store, the sole writer of
  shell.json (`modules/bar/barstyles/qsbar/core/`, `.../BarSlot.qml`,
  `.../BarPlugins.qml`, `.../Theme.qml`, `.../VariantRoot.qml`).
- **The QS Bar's in-shell panel is rebuilt as QS Bar Settings: four routes about
  the bar, not nine about the whole shell.** The panel the bar's 力 logo opens is
  now Bar (帯: position, form, surface, gaps, scale, accent, gap animation,
  auto-hide), Layout (配置: three lanes of widget chips you move, hide, add and
  reset, with an add-widget picker of hidden built-ins and installed plugins and
  a way into Ryostore), Widgets (部品: every catalogue widget as a row you show,
  size for density, colour, and tune through its own settings) and Dock (台). The
  Layout route reads and writes the new `qsbar.layout` through the root's
  barLayout API, so a drag in the panel and a `ryoku-shell bar move` change the
  same document. The routes it used to carry left for the homes they already had:
  the launcher mark and the workspaces count/marker are folded into Widgets as
  those two widgets' settings; picker style and desktop widgets moved to the
  Hub's Desktop and Widgets pages; the mid-work switches and the session live in
  the Super+Escape quick settings. The name "Shell Studio" is retired
  (`modules/bar/barstyles/qsbar/controlcenter/`).
- **ryogami: the light/dark toggle now retints the whole desktop, and the
  ollama AI tagging subsystem is gone.** The picker's light/dark (and
  scheme/source-colour) controls called a `wall.retheme` RPC the ryogami
  daemon never implemented and which never ran matugen, so the choice never
  left the picker; they now hand the knobs to the shell's one matugen store
  (`ryoku-hub hypr matugen set`, the same merge-write the Hub uses), which
  ryoku-shell watches and retints from, so mode/scheme/index apply
  system-wide. Separately, the ollama vision auto-tagger and everything that
  only served it are removed for a clean cutover: the ollama settings page,
  status indicator, retag button, scan controls, the tag cloud and tag
  filter/sort, and the weather filter, plus the daemon's analyze plumbing and
  the `Tags`/`Colours`/`Weather`/`AnalyzedBy` catalog fields. The
  hue/saturation colour sort (magick-derived) stays
  (`ryogami/wall-ui/`, `ryogami/daemon/`, `services/DaemonClient.qml`).

### Fixed
- **The visualiser's edit bar fits scaled displays.** Its controls sit in one
  row of about 1900 logical px and were only centred, so on a display scaled
  past 1x (1600 logical px at 1.6x on a 2560 panel) the row ran off both edges
  and the right-hand controls were unreachable. The bar and the look tray now
  scale down to the surface width, anchored to the edge they hang from
  (`modules/visualizer/EditBar.qml`).
- **The visualiser stays inside the screen.** Its box is a fraction of the
  screen, but the placer let a drag or wheel resize leave up to a quarter of it
  past an edge, where the spectrum is simply cut off, and that box was saved
  and restored on every login. Every write now clamps size to the screen and
  position to what the size leaves, and a box stored off-screen is folded back
  once on load (`modules/visualizer/Singletons/Config.qml`).
- **Login always brings the shell up.** The user manager can outlive a session
  (linger, a relogin after a compositor crash) and still hold a `ryoku-shell`
  bound to the dead compositor; `systemctl --user start` then reported it
  active and the login landed on bare Hyprland with no shell. The autostart
  now reloads units, clears a start limit, and `restart`s the shell and
  wallpaper daemons every session (`hyprland/modules/autostart.lua`).
- **Placing the audio visualiser keeps it visible on Power Saver.** Super+Alt+M
  forced the visualiser on, but the spectrum stayed frozen under Power Saver,
  low-power mode, Game Mode or silence, so it was an invisible flat line you
  could not aim. Placement now holds the spectrum live for as long as it is
  being positioned (`modules/visualizer/`).
- **Super+W opens the wallpaper picker on a fresh install.** The picker only
  builds once its config loads, but nothing created `~/.config/ryogami-wall` on
  first run, so `configLoaded` never turned true: Super+W did nothing and the
  daemon had no stored wallpaper. The picker now seeds the config from the
  shipped example on first run, and re-seeds if `config.json` is deleted
  (`ryogami/wall-ui/qml/services/BootstrapService.qml`).
- **The chosen power profile survives reboots and AC changes.** An automatic
  switch (ppd's boot default, or a firmware/platform switch to performance on
  AC) was saved as the user's pick, so the choice drifted to performance.
  Automatic switches near boot or an AC plug/unplug are no longer persisted,
  and the saved profile is re-asserted on the AC-plug edge (`ipc/powerprofiles.go`,
  `ipc/autoprofile.go`).
- **Live video wallpapers decode on the GPU when it can.** The ryogami C player
  opened a CPU-only decoder, so a clip cost about 15% of a core per monitor. It
  now tries VA-API, NVDEC, then VDPAU first and only falls back to software,
  mapping just the decode driver and never a GL context, so the light-RSS
  behaviour holds where no accelerator is present (`livewall/livewall.c`,
  `livewall/build.sh`).
- **A live video wallpaper no longer stays black after a fullscreen app
  closes.** The player free-ran on its own timer with no frame callback, so once
  its background surface was occluded the loop kept decoding into an unseen
  surface and could leave a stale or black frame on reveal. It now gates each
  frame on a `wl_surface.frame` callback: it idles while hidden and repaints the
  moment the wallpaper is visible again (`livewall/livewall.c`).
- **Launcher actions now invoke live shell routes and shipped helpers.** The
  consolidated shell removed the old `toolkit`, `sysinfo`, and `clipboard`
  daemon verbs, but the action catalog still launched them and silently
  discarded their failures. Control Deck and System Info now open Quick
  Settings, Clipboard History opens its supported deep route, and
  `ryoku-cmd-*` helpers resolve through the package-managed `PATH` rather than
  a stale `~/.config/hypr/scripts` copy (`modules/launcher/shared/providers/
  actions/`).

### Added
- **QS Bar size is independently adjustable.** A 100–200% Size control in
  Bar Studio and Shell Studio scales the bar's height live without changing
  monitor display scaling (`quickshell/shell/modules/bar/barstyles/qsbar/
  {Theme.qml,BarSlot.qml,controlcenter/routes/BarsRoute.qml}`,
  `hub/quickshell/pages/BarStudioPage.qml`).

- **The wallpaper's subject can be cut out and drawn in front of the desktop
  widgets, composed from the Super+Esc Desktop route.** A DEPTH card turns the
  effect on, picks the model, and enters a COMPOSE mode where the clock is
  dragged into the subject's negative space while feather, foreground strength,
  and clock-in-front are tuned live. The daemon generates the cutout off the
  wallpaper hot path and carries it on the wallpaper topic; the opt-in engine
  installs on first enable, so nothing ML ships in the base image
  (`modules/depth/`, `ipc/depth.go`, `scripts/ryoku-depth`,
  `controlcenter/routes/DesktopRoute.qml`, `docs/depth.md`).

- **Depth generation stays in the shell daemon and now drives ryogami's
  wallpaper frames.** With the wallpaper surface moved to ryogami, the Go depth
  worker (per-wall registry, engine runs, `depth refresh|status|set-enabled|clear`)
  mirrors ryogami's `wallpaper` topic to see what is on screen and hands finished
  cutouts back over `depth set` / `depth clear` on ryogami.sock; ryogami folds
  them into the published frame and drops a cutout whose wallpaper already left
  the slot (`ipc/ryogami.go`, `ipc/depth.go`, `ryogami/src/server/routing.rs`).

- **The skwd transition catalog, the wallpaper palette, live-wall detection,
  and picker scrolling are all in.** Every switch now animates through the 38
  GLSL transitions ported verbatim from skwd-paper (crosswarp, voronoi-shatter,
  heat-melt, perlin and the rest), driven by the picker's own
  `transition.enabled/shader/durationMs` keys with skwd's random default and
  600 ms duration; the shell's 22 reveal presets stay reachable through
  `transition.shader: "ryoku"`. The dynamic matugen pipeline is rewired: the
  shell's ryogami bridge schedules the palette pass on every switch, so
  colors.json and the app templates render through the shell's enriched
  context again (the daemon no longer execs matugen with a context the
  templates cannot resolve). Videos are catalogued from ~/Pictures/livewalls
  by default, and the picker belt caches its thumbnails instead of re-decoding
  them on every drag. Video hover/selection previews decode a small scan-time
  clip (640w/24fps, built beside the thumbnails, VAAPI when available) instead
  of the full source: a 4K60 HEVC-10 decode per hovered card is what froze the
  selector. The picker itself now runs resident like the shell's overview: one
  quickshell instance preloads hidden at daemon boot and Super+W flips its
  surface over the event hub (~190 ms, livewall playing or not), where it used
  to cold-boot and kill a whole process per press, so rapid presses raced the
  spawn cycle and broke the selection. The Instant playback toggle is no
  longer disabled while previews are off, which silently swallowed the click
  that tried to turn it off (`modules/wallpaper/skwd/`, `ryogami/daemon/`,
  `ipc/ryogami.go`, `ipc/theming.go`, `wall-ui/`).

- **`ryoku update` on a checkout now restarts the ryogami daemon, so its
  fixes actually take effect.** deploy.sh rebuilt and installed the ryogami
  binary but only restarted ryoku-shell, never ryogami.service; systemd kept
  the old daemon running, so every ryogami change (the folder watcher, the
  video player, the picker) sat on disk unused until the next logout, and
  `ryoku update` looked like it did nothing. deploy.sh now `try-restart`s
  ryogami.service after installing the binary and pointing the unit at it
  (`shell/deploy.sh`).
- **Ryogami now notices wallpapers and videos dropped into the folders while
  it runs.** The Go rewrite kept a config-file poller but no library watcher,
  so a file added by hand to ~/Pictures/Wallpapers (or the livewalls dir)
  never entered the catalog until a daemon restart, and the resident picker
  showed a stale grid (the vendored QML inotify watcher was never instantiated,
  needed an uninstalled `inotifywait`, and its handler was a stub). A stdlib
  poll-watcher fingerprints the media dirs (path, mtime, size) every few
  seconds and reuses the mtime-gated rescan on any add, remove or replace; its
  `cache ready` broadcast reloads the open picker, so new art appears within
  seconds with no restart (`ryogami/daemon/watch.go`).
- **Video wallpapers play through ryogami-live (the restored in-repo C player,
  renamed from ryoku-livewall), not mpvpaper.** The software-decode daemon
  (`livewall/livewall.c`) paints wl_shm frames on its own background surface:
  ~85 MB RSS and a quarter core, where mpvpaper's GL pipeline held ~1 GB,
  pinned the CPU on hybrid GPU machines and flickered the screen. Definition
  is fixed at the root: clips are transcoded once to the widest monitor's
  PHYSICAL pixel width (the old logical-width cache came out soft on any
  fractional-scale panel: 1920 @ 1.25 encoded at 1536 and stretched back up),
  bounded by resource tier (low 1920, medium 2560, high 3840), on the AMD
  video engine when present (~2 s for a 4K clip, libx264 crf 18 bicubic
  otherwise). The source frame rate is kept and only capped by tier (24/30/60):
  a 24fps clip is never padded to 30, which duplicated frames into judder. An
  already-fitting H.264 clip plays untouched. The shell paints the clip's own
  still under the player and yields only on the player's READY handshake, so
  the reveal, the depth cutout and the palette work from a real frame, the
  screen never blanks through the first transcode, and the still returns the
  instant every player dies (`livewall/`, `ryogami/daemon/video.go`,
  `ryogami/daemon/livewall.go`, `release/packages/ryogami/PKGBUILD`).

- **The full skwd wallpaper stack now runs in the Go daemon: transitions,
  effects, pipelines, and a video engine.** Every static switch reveals through
  the 22-preset shader engine, rendered in-shell by the restored reveal
  backdrop (`modules/wallpaper/Backdrop.qml`, `reveal.frag`), with the preset
  attached to each published frame and `wallpaper.transition_preset` in
  shell.json pinning one (default rotates with no repeats). The picker's
  effects panel is served natively (theme palettes, invert, flip, mirror,
  grayscale, brightness, contrast, saturation, gamma, pixelate, border,
  round), image optimize and video convert run their preset pipelines with
  progress events, and videos and live walls play through mpvpaper on the
  background layer while the shell painter yields (frame `live` flag). The
  engine settings page now shows the one real engine instead of the skwd
  lineage's external painters (`ryogami/daemon/`, `wall-ui/qml/wallpaper/
  settings/PaperSettings.qml`).
- **The ryogami daemon is now Go, not Rust.** A drop-in rewrite at
  `ryogami/daemon/` keeps the whole wire contract (ryogami.sock verbs, JSON-RPC
  with result payloads, `subscribe` event streaming, the wallpaper topic, the
  depth surface, the wall-ui spawn) and the cache layout, so the picker, the
  shell QML, the keybinds, and the depth bridge run unchanged while applies go
  straight to the in-shell surface. The catalog persists as a JSON index that
  reuses already-generated thumbnails; effects, optimize, convert, steam and
  analysis answer unknown-method until ported (the default feature set never
  calls them). Builds with the desktop's Go toolchain: no cargo, no Rust
  dependency chain (`ryogami/daemon/`, `release/packages/ryogami/PKGBUILD`,
  `deploy.sh`).

- **Super+W opens the ryogami wallpaper picker, the vendored skwd-wall
  full-screen browser.** The picker QML (by liixini, MIT) is vendored under
  `ryogami/wall-ui/`, renamed to speak ryogami.sock (`ryogami.wall.*` events,
  `RYOGAMI_*` env, `~/.config/ryogami-wall/`), and shipped by the ryogami
  package to `/usr/share/ryogami` (dev deploys stage it under XDG data and point
  the unit at it). The daemon now serves its wire contract: connections stay
  open across requests, JSON requests get full result payloads, a JSON
  `subscribe` streams `ryogami.*` events on the same socket, and the new
  `ryogami wallpaper ui` verb toggles the picker for the keybind. The
  frame-blob wallpaper menu stays reachable from the bar logo
  (`ryogami/wall-ui/`, `ryogami/src/server/{connection,routing}.rs`,
  `hyprland/modules/binds.lua`).

- **Hard reload accepts still, animated, and muted video media while retaining the bundled wordmark fallback and unchanged iris/readiness lifecycle.** Reload covers read the selected media descriptor from `brand.json`; a persisted custom-media On/Off gate releases all custom decoders when Off and restores the saved asset when On (`quickshell/reload-cover/`).

- **Hard reload accepts still, animated, and muted video media while retaining the bundled wordmark fallback and unchanged iris/readiness lifecycle.** Reload covers read the selected media descriptor from `brand.json`; a persisted custom-media On/Off gate releases all custom decoders when Off and restores the saved asset when On (`quickshell/reload-cover/`).
- **The desktop visualizer can stack several looks, use exact gradients, and wrap
  the display in a reactive frame.** The placement bar switches, adds and removes
  up to four visualizers, shows a light RAM estimate for each full-screen pass,
  and opens a two-stop colour picker with draggable selectors and hex entry. A
  shared spectrum feed drives every instance, while the new Frame look fills the
  display edge with dense inward-growing bars (`modules/visualizer/`,
  `ui/SpectrumField.qml`, `ui/shaders/spectrum.frag`).

- **The bar's brand logo can open the quick settings instead of the Shell Studio.**
  A STUDIO / QUICK SETTINGS segmented switch in both the Studio foot and the
  Super+Esc quick-settings sidebar sets which surface the logo click opens; the
  choice persists in `shell.json` (`launcherTarget`) and the lit segment shows the
  current target. The Studio foot drops its dead Super+Esc caption for the switch
  (`services/Config.qml`, `controlcenter/CcRail.qml`, `modules/LauncherWidget.qml`,
  `framebars/menus/quicksettings/QuickSettingsHome.qml`).
- **The Session route gains Log out, and the studio shows its Super+Esc key.**
  Session now offers Log out (arm-to-confirm, `hyprctl dispatch exit`) beside
  lock, sleep, restart and power off, and the rail foot carries a SUPER ESC
  keycap beside the search's CTRL K, so the shortcut that toggles the panel is
  learnable from the panel itself (`controlcenter/routes/SessionRoute.qml`,
  `controlcenter/CcRail.qml`).
- **The control center names itself the Shell Studio and points to the Hub.** It
  reads SHELL STUDIO in the rail masthead and carries a persistent "OPEN THE HUB"
  link in the rail foot, so it is clear this is the quick studio and the full
  settings live in the Hub; the search now reads "Search the studio" instead of
  the generic "settings, options, or routes" (`controlcenter/CcRail.qml`,
  `controlcenter/ControlCenter.qml`, `controlcenter/CcSearch.qml`).
- **Recording can target a monitor or a window, not only the whole screen or a
  drawn box.** The capture card's record row now offers Screen, Monitor, Window
  and Region, the same four targets the screenshot row has, raised through the
  same selection overlay. A picked monitor becomes gpu-screen-recorder's own
  `-w <output>` on the KMS backend and degrades to that monitor's rect on the
  portal backend, so it works on the hybrid machines that cannot enumerate a
  monitor at all. On Wayland a window is captured by the area it covers, and the
  control says so rather than implying it follows the window.
- **A delay before recording, not only before a screenshot.** The capture card's
  delay (0, 1, 3, 5 or 10 seconds) now arms recording too: the record island
  counts the seconds down in place of its clock, and its stop control cancels.
  With no delay set nothing changes.
- **A recording announces itself properly when it is finished.** Stopping used
  to fire "Saving recording to Videos/Recordings" before the file existed, with
  no name, no length and nothing to click. gpu-screen-recorder's own completion
  hook now runs `ryoku-cmd-recording-saved` once the file is really written, and
  the notification carries the clip's length and size with Open, Show in folder
  and Copy path on it. Ryoku's screenshots have said "saved to <path>" for a
  long time; recordings now match.
- **Controls answer under the finger.** The design system has always defined a
  pressed ink level (`Tokens.tint16`) and documented it as such, but only three
  surfaces used it: every button, chip, segment, tab, tile and switch in the
  shared kit, the studio's own kit and the popout cards hovered and then went
  dead on press. They now step to the pressed level and settle back on release,
  colour only, so nothing moves and no layout reflows.
- **Font changes now reach the terminal and carry a size, not just GTK.** The
  system font gained a monospace face and a base point size: the daemon writes
  `font-name`, `document-font-name` and `monospace-font-name` (with the size) to
  gsettings, rewrites the qt6ct general font, and lands the terminal font in a
  kitty include it reloads over SIGUSR1, so the terminal retypes live alongside
  GTK apps. The kit monospace (`Tokens.mono`) follows the choice; the Fraunces
  and Space Grotesk brand faces stay fixed (`ipc/matugen.go`,
  `services/Config.qml`, `ui/Singletons/Tokens.qml`, `apps/kitty/kitty.conf`).
- **Admin (polkit) prompts can answer to a fingerprint, like macOS sudo.** When
  the polkit stack races the reader (enabled from Hub > Sign-in & Fingerprint >
  Admin prompts), the shell's admin island shows a live scan the moment PAM asks
  for a touch, and authorizes on a match or the typed password. The daemon
  derives the state from pam_fprintd's own PAM narration and publishes it; a
  shared `FingerprintScan` component (Ryoku.Ui) draws the ridges filling and the
  ring completing, reused unchanged by the lock screen and the Hub
  (`ipc/polkit.go`, `services/Polkit.qml`, `modules/bar/PolkitSurface.qml`,
  `ryoku/ui/FingerprintScan.qml`).
- **The dock has five looks, picked from the Shell Studio.** The app dock keeps
  its Islands baseline (split pills) and gains four more: Rail (one continuous
  plate), Ledger (numbered cells), Tanzaku (hanging strips) and Seal (colour
  means running). The Shell Studio's Dock route grows a Style row and its live
  preview redraws to match, so the choice reads before you commit it. The look
  persists in `shell.json` as `dock.style` (default `islands`, so an existing
  desktop is unchanged) and applies with no reload
  (`controlcenter/routes/DockRoute.qml`, `services/Dock.qml`, `modules/dock/`).
- **The launcher's solar line follows the palette, or a colour you choose.** The
  warm line under the clock, in both the Hero and Main variants, was a hardcoded
  gold; it now tracks the wallpaper's primary by day and the secondary accent by
  night when Match wallpaper is on (Match off keeps today's fixed gold and cool
  blue). Two `launcher.json` keys drive it: `horizonMode` (`auto`, the default, so
  nothing changes; `fixed`; or `off`) and `horizonColor`. Fixed pins the line and
  its sun/moon marker to one colour, keeping the elapsed and remaining segments
  distinct; off hides the line and marker with no gap left behind.
- **Print filters: the compositor can resolve the whole screen to the desktop's
  ink.** Five shaders ship in `ryoku/hyprland/shaders/` and apply to everything
  Hyprland composites, apps included: **Bone** (tone kept, hue dropped, remapped
  to bone rather than white), **Halftone** (newsprint dots, area-proportional and
  resolution-independent), **1-bit** (the shell's own image dither, screen-wide),
  **Vignette** (a page lit from the middle, the one filter that leaves colour
  alone) and **Grain** (film tooth). Pick one on the Shell Studio's Desktop
  route; the choice persists in `shell.json` (`screenShader`) so it survives a
  reload, and low power forces it off because it is a full-screen fragment pass.
- **The palette cross-fades with the wallpaper instead of snapping ahead of it.**
  A new wallpaper's colours used to land in a single frame while the picture it
  was derived from was still wiping in. Every Material role now walks from the
  outgoing palette to the incoming one over the same beat (`Tokens.blend`), so the
  desktop's ink migrates with the reveal. Reduce-motion still lands instantly.
- **You can choose the wallpaper reveal, not just get a random one.** The shell
  ships 22 named reveals (silk fade, iris open, page turn, and the rest) plus a
  `random` sentinel that plays a fresh no-repeat one per switch; until now every
  Super+W / Super+Shift+W switch was random with no way to pin one. A new
  `wallpaper.transition_preset` key (default `random`, so nothing changes for an
  existing desktop) is honoured on every user-driven switch: a named preset
  reveals the same way each time, an unknown or absent value falls back to random,
  and the two re-apply paths (login, live-reload) still never animate. The Shell
  Studio's Pickers route grows a REVEAL card to set it (`ipc/transitions.go`,
  `ipc/settings.go`, `controlcenter/routes/PickersRoute.qml`).
- **The Shell Studio and the launcher results stagger in.** A studio route's cards
  and the launcher's ranked results used to appear all at once; each now rises and
  fades one after the next through the shared `Entrance` wrapper (`ui/Entrance.qml`),
  a short ripple that reads as the surface settling rather than snapping. The delay
  is bounded so even a long route lands promptly, and reduce-motion draws everything
  at once with no motion.
- **The qsbar logo opens a Shell Studio, not a control panel.** The old panel was
  a QUICK deck plus a CONFIGURE landing page with its own bespoke widget
  vocabulary; it is now one plate with a rail of nine routes (Bar, Widgets, Logo,
  Spaces, Pickers, Dock, Desktop, System, Session), each a page of the house form
  kit. Latin names the route, kanji seals it, the active route is the only bone
  plate, and `Ctrl+K` searches every control by name, keyword and route. The dock,
  the desktop widgets, the shell switches and the session actions are reachable
  from it for the first time; nothing that was reachable before was dropped.
- **A live dock preview.** Changing the dock's edge used to change something you
  could not see, because the plate covers the dock (and on the left edge it sits
  directly underneath the panel). The Dock route now carries a schematic of the
  screen with the bar and the dock on it, and the island travels to the edge you
  pick.
- **The bar accent can follow the wallpaper.** `Follow wallpaper` stores the real
  `accent` value rather than pinning a palette slot that pretends to track the
  image, and switching it off puts back the slot you had. The cold-start cache
  used to flatten `accent` to `color01`; it no longer does.
- **Two desktop widgets join the set: weather and notes.** Both ported from
  end-4's ii widget canvas, both in Ryoku's own language (vector glyphs, ink
  picked against the wallpaper tone under the slot, every dimension multiplied by
  the widget's scale so nothing is a stretched texture). **Weather**
  (`modules/desktop/weather/`) reads the shell's existing forecast daemon and has
  two looks: `compact` is a glance (condition glyph, temperature, city) and
  `full` adds the condition, a humidity / wind / feels row and a three-day strip;
  with no reading yet it says so honestly and a tap re-kicks the poll. **Notes**
  (`modules/desktop/notes/`) is a scratch pad: its text is content, not
  configuration, so it lives in `~/.local/state/ryoku/desktop-notes.txt` (atomic,
  debounced, so a burst of typing writes the file a few times and never once per
  keystroke), and it holds the layer's keyboard only while it actually has focus
  -- Escape or a click anywhere else hands it straight back.
- **The dock is its own shell surface, and it finally behaves like a dock.** It
  left the qsbar bar style (`barstyles/qsbar/DockSlot.qml` and `DockRow.qml` are
  gone) and lives at `modules/dock/`, one `PanelWindow` per monitor on the edge
  opposite the bar, so every bar style gets the same dock. What it gained, ported
  from end-4's ii dock: **auto-hide** with a 3 px peek strip (it reveals on
  pointer proximity, when nothing holds focus, and while a pin is being dragged,
  and gets out of the way of a fullscreen window), **drag-to-reorder** pinned
  apps with a ghost that follows the cursor and a live swap as it passes a
  neighbour, a **hover label** with the app's real name, **middle click** to
  launch a fresh instance, and a **launch bounce** on the click that starts an
  app. What it fixes: a pinned app that started running used to jump across the
  separator into the running group -- the order is now pinned-in-user-order
  first, always. Magnify, the frosted islands, the running-window dots and the
  live window-preview strip carry over. Nine keys in a new top-level `dock`
  object in shell.json drive it (`enabled`, `edge`, `autohide`, `pinned`,
  `magnify`, `frost`, `shadow`, `labels`, `media`), written through the daemon's
  settings store like every other shell-owned key; `ryoku doctor` migrates the
  retired `qsbar.dock*` keys into it. Sumi's in-rail `RailDock` widget stays as
  the in-band alternative.
- **Nine expressive wallpaper transitions.** `reveal.frag` grows past masks into
  coordinate-warping effects, ported from end-4's ii transition shaders:
  `pixelate` (a mosaic that swells and resolves), `dissolve` (noise burning
  through behind a warm edge), `ripple`, `shatter` (the old frame diced into
  spinning shards), `glitch` (scanline tears plus RGB split), `crt` (the frame
  collapsing into a bright line and reopening), `stripes`, `melt` (Doom's screen
  melt) and `peel`. The preset table goes from 13 to 22, and a fresh per-switch
  `seed` uniform means the noise-driven ones never replay the same pattern.
- **Desktop widgets can place themselves on the wallpaper's calm regions.** A
  widget's anchor gains `auto` beside the nine compass zones: it lands where the
  picture is quietest and most even in tone, and glides to a new spot when the
  wallpaper changes. The daemon now publishes a per-cell **detail** map (the
  local contrast of each cell of the 8x8 wallpaper tone grid, from the same
  single ffmpeg decode) and `Ink.calmSpot()` picks the position that minimises
  busyness and tonal drift together. This is end-4's least-busy-region placement
  without its python/opencv sidecar.
- **Real drag feedback for desktop widgets.** Dragging a widget now shows the
  snap grid and a pair of centre guides that light up as the widget's centre
  meets the screen's, and releasing flashes the lines it landed on
  (`modules/desktop/DesktopGuides.qml`, which replaces the Canvas-drawn
  `WidgetGrid.qml`). Ported from end-4's widget canvas.
- **The music widget's spectrum can draw as a smoothed wave.** A new `musicViz`
  key picks `bars` (unchanged, the default) or `wave`: the 40-band cava feed,
  moving-averaged and resampled onto a continuous mirrored band with a glow
  behind it, tinted from the album art rather than the scheme accent.
- **Per-monitor interface scale.** Each display can shrink or enlarge the whole
  Ryoku shell chrome -- frame bar, launcher, OSDs, notifications, capture
  overlays -- independently of the Hyprland monitor scale that apps render at, so
  a low-DPI external no longer forces the UI oversized while apps stay crisp.
  Surfaces multiply their own logical sizes by `Tokens.uiScaleFor(output)`, read
  live from shell.json `displays.ui_scale`; set it per monitor in Settings ->
  Displays -> Interface scale. Ported from caelestia (per-monitor token
  multipliers) and omarchy (a shell-wide size scale).

### Changed
- **Control center routes slide instead of cross-fading.** Switching a route in
  the studio now slides the outgoing page off to the left and brings the incoming
  one in from the right on the house spatial curve, swapping off the plate so the
  change reads as one continuous lateral move with no fade or seam
  (`controlcenter/PageMotionStage.qml`).
- **The control center wears a frame.** The studio plate gains HUD corner ticks,
  the same L-bracket vocabulary the poster art and reference sheet use, so it
  reads as a registered instrument surface like the Super+S sidebar rather than a
  bare card. The ticks are anchored to the plate, so the frame reframes as the
  plate resizes on a route change, on the same spatial curve as the page slide
  (`controlcenter/ControlCenter.qml`).
- **Notification popups are flicked away, not faded out.** Dismissing a toast
  (its close mark, its timeout, or the app retracting it) now slides it off the
  edge it lives on while it shrinks and fades in one parallel move, so it reads
  as thrown aside rather than dissolving in place; the cards below hold their
  slot until it has cleared, then rise to close the gap instead of teleporting.
  Arriving is the mirror and deliberately slower: the card comes in from the same
  edge with a small scale settle so it lands rather than appears. The surface
  keeps its full height for the whole dismiss, so a dismissed popup can never
  leave a jump or a ghost row in the stack. Ported from Ambxst's notification
  dismiss (slide-out overshoot plus shrink and fade). Reduce-motion still cuts
  every step to an instant add and remove with no leftover transform.

### Fixed
- **Shell Studio's Session logout and lock work now.** The Session route fired
  `hyprctl dispatch exit` for Log out and launched `hyprlock` for Lock, but the
  desktop's Lua-configured Hyprland fork makes the first a no-op (`exit` is not a
  Lua dispatcher; the working form is `hl.dsp.exit()`) and never ships hyprlock.
  Both now use the shell's own wiring, the same `Hyprland.dispatch("hl.dsp.exit()")`
  and `ryoku-shell lock` every other surface uses (`controlcenter/routes/SessionRoute.qml`).
- **Recording the screen records the screen you are on.** Fullscreen capture
  passed gpu-screen-recorder `-w screen`, which its manual defines as the first
  monitor it enumerates, not the focused one, while the wf-recorder path had
  always honoured the focused output. On a two-monitor desk the two backends
  therefore recorded different screens. Both now record the output you are
  looking at.
- **The microphone is no longer on by default.** A first recording captured the
  user's voice and not the application being demonstrated, which is both the
  wrong way round and a privacy surprise. Desktop audio now defaults on and the
  microphone off, matching what a purpose-built recorder does. An existing
  `record.json` keeps whatever you chose.
- **A remembered region is dropped when the desktop layout changes.** The last
  picked box was reused with no record of the monitor arrangement it was drawn
  in, so after a hotplug or a resolution change it could sit off-screen and the
  recorder would crop to nothing. The region now carries a layout signature and
  is discarded when that no longer matches.
- **The audio visualiser and pill bars no longer freeze while sound is
  playing.** The analysers drop cava when nothing plays, but idle was keyed off
  the MPRIS media player (`Media.playing`), so audio with no MPRIS interface --
  games, browser tabs, system sounds -- read as silence and froze the feed
  mid-sound; a player under-reporting between tracks did the same. Idle now keys
  off real audio: an active PipeWire playback stream (`Audio.streams`), with
  MPRIS kept only as an instant fast-path. The idle gate is also genuinely
  debounced now (the old `audioIdle: !Media.playing` was a live binding that
  defeated its own 4s grace -- the same binding-vs-imperative trap as the
  analysers' `running`), so a stream blip between tracks no longer cuts cava
  (#61) (`services/Perf.qml`). An earlier `stdbuf -oL` attempt is reverted: cava
  writes its raw output unbuffered, so it did nothing.
- **The sign-out button works again.** Ryoku's SDDM session runs `Exec=Hyprland`
  directly, so the old logout command (`systemctl --user exit`) stopped the user
  manager without ever ending the compositor -- the button did nothing. Logout
  now runs `hyprctl dispatch exit`, which exits Hyprland and returns SDDM to the
  greeter (#62) (`ipc/session.go`,
  `modules/bar/framebars/menus/MenuQuickActions.qml`).
- **A dismissed toast stops throwing on its way out.** The delegate outlives its
  model entry, so a card animating away read `appName` and `body` off a null and
  logged a TypeError per frame. Reads go through one guarded accessor now. The
  hazard predates the flick-out dismiss; the longer exit is what made it visible.
- **The dock comes back down when the pointer leaves it.** An icon grown by the
  cursor-tracked magnify stayed grown after the pointer left the band: the tracker
  only ever takes a position from a point inside the band, so on exit it kept the
  last one and the icon under it went on reading a full-scale falloff. The scale
  now consults the band's hover state as well as the position, so leaving the dock
  releases it. Measured on a 46px island: 55px at rest, 77px hovered, and 77px
  again long after the pointer had gone; now back to 55px.
- **The studio's plate corners read as rounded.** A 900px plate carried a control's
  radius, which at that size reads sharp, and the bottom fade squared the corner
  outright because `clip` is rectangular and ignores `radius`: the fade now carries
  the corner itself and the plate's own radius is scaled to its size.
- **The dock's pinned-app rows no longer collide with their own hairline.** The
  rows sat flush against the card border with an icon taller than the gap to the
  divider, so each icon was crossed by a line. They are inset to the card's text
  column and tall enough to clear it.
- **Each widget card shows the mark its widget actually draws in the bar.** The
  inventory was a list of names; the ones whose bar mark is a single glyph now
  carry it (volume's `graphic_eq`, the CPU sparkline, the wifi bars, the media
  note). The ones that draw a ring, a number or a per-state logo stay glyph-less
  rather than showing an invented icon.
- **A click off the studio dismisses it.** The panel ate every click inside the
  plate and ignored the rest of the screen, so the only way out was Escape or the
  close mark, unlike every other popout on the bar.
- **The studio no longer slices a row at its bottom edge.** The plate takes its
  height from the page, but a page longer than the cap was cut mid-row against the
  border, which reads as a broken layout rather than "there is more below": the
  cut now dissolves into the paper, the rail's printed foot is counted in the
  rail's own height instead of overflowing it, and the cap grew so the long pages
  fit.
- **The studio rail's barcode no longer prints across the divider.** Its module
  width was a constant, so the Code 39 plate came out wider than the rail and its
  bars ran over the hairline into the page column. The width is solved from the
  rail's own inner width now, because a clipped barcode loses its stop bars and
  quietly stops being a barcode.
- **The studio's plate no longer snaps between routes.** Routes have different
  natural heights, and the resize was instant while the page faded, so a switch
  read as two unrelated events. The plate now resizes on the house spatial curve
  and the incoming page rises into it as one gesture.
- **A disabled setting row now looks disabled.** `SettingRow` stopped accepting
  input when gated by a master switch but kept full ink, so an inert control lied
  about its state -- in the Hub as well as the studio.
- **A background gpu-screen-recorder no longer strands the record toolbar.** The
  recorder keyed "we are recording" off any gpu-screen-recorder process, so a
  manually-started replay buffer flipped the floating toolbar on and left it stuck
  until the process was killed. The shell now tells a replay buffer (gsr's `-r`
  flag) from a recording and scopes its stop/pause to real recordings, so a
  background buffer is left untouched.
- **Live wallpapers switch with a transition, like every other wallpaper.** A clip
  used to arrive as a hard cut: its still frame was published with no preset and
  the video surface covered the backdrop immediately, so the whole transition set
  only ever applied to stills. The reveal now runs on the clip's own first frame
  and the player is held back until it finishes, so the frame the video starts on
  is the frame the reveal landed on and the handoff is invisible. The hold is
  shared by every launch path, so the fullscreen gate's resume cannot cut in
  mid-reveal either.
- **A live wallpaper no longer freezes on its first frame after a shell reload.**
  The video player and the shell's backdrop share the Wayland background layer,
  where the newest surface draws on top -- so a reloaded shell covered a
  still-running clip with its frozen still, and only the next wallpaper switch
  (or a manual `wallpaper live-reload`) brought the motion back. The daemon now
  publishes who owns the pixels (`live` on the wallpaper topic) and the backdrop
  stands down while the player paints, so stacking order stops mattering.
- **A live wallpaper's reveal is framed like the video that replaces it.** The
  still was painted with the user's image content-fit while livewall maps the clip
  with the ryowalls live fit, so a letterboxed clip visibly jumped scale the
  moment it took over. The still now carries the video's geometry.
- **The dock's active-app tint works, and its auto-hide can tell whether anything
  is focused.** `Hyprland.activeToplevel` never populates on Ryoku's Hyprland
  fork (the same ipc parse gap `Fullscreen.qml` documents), so the dock service
  read focus from a property that was always null. It now reads the focused
  window out of the toplevel list itself (hyprctl marks it `focusHistoryID 0`),
  refreshed on the focus events.
- **The desktop visualiser no longer freezes on a stale frame during playback
  (#61).** Two regressions from the idle-freeze work: the visualiser's cava never
  came back after exiting on its own (a pipewire hiccup) -- its `running` binding
  ignored the restart `backoff` the pill's analyser already honoured, so it stuck
  on the last frame until the next track -- and the shared idle gate dropped cava
  on the brief not-playing a player reports between tracks, blipping the feed
  every song. The binding now matches the pill, and the idle gate holds a few
  seconds before freezing so track gaps ride through.
- **The bar gap animation runs on the GPU, so a default desktop no longer burns
  45-60% GPU (and ~25% of a CPU core) drawing it (#60).** All six drift modes
  (stream, surge, bolt, spark gap, transfer, collider) were rasterised by a
  threaded Canvas -- dozens of fills every frame, running even at a silent idle.
  They are now one fragment shader (`stream.frag`) evaluated per pixel over just
  the bar gaps: the same motion at ~0 GPU and roughly a third of the CPU (bolt:
  25.7% -> 9.0% of a core on the dev box). Because they are cheap now they animate
  on Balanced as well as Performance; reduce-motion / Power Saver still unload
  them. The two stateful modes -- reactor and quotes -- keep their Canvas.
- **A checkout deploy fails with a clear message when the Go toolchain is
  missing.** `deploy.sh` builds the Go programs from source, so a packaged box
  switched to a checkout update channel without `go` died mid-build with a bare
  "go: command not found". It now checks for the toolchain up front and says how
  to fix it (install go, or that a packaged install updates through pacman).
- **The bar's system monitor reads GPU load and temperature on AMD and Intel
  GPUs.** `StatsFeed`'s GPU telemetry came only from `nvidia-smi`, so a non-NVIDIA
  GPU reported nothing. It now falls back to the amdgpu sysfs nodes (utilisation,
  edge temperature, power) when there is no NVIDIA reading.
- **Light themes are legible again.** Muted and faint text used the Material
  `outline`/`outlineVariant` roles (light greys that wash out on a light surface),
  so a white wallpaper left the launcher, Hub and terminal barely readable. The
  ink ramp now derives muted and faint from on-surface-variant at reduced opacity
  so text holds contrast in both modes; fish's comment and autosuggestion colours
  move off outline too. Confirmed against caelestia and dusky (never outline for text).
- **The hero launcher reads in a light palette.** The hero search overlay was
  authored light-on-dark: its "TYPE TO SEARCH" placeholder and idle underline
  were hard white, and the header emboss and edge vignette were hard black, so a
  light wallpaper left the search prompt invisible and the art muddy. A new
  `Tokens.light` flag (surface luma) flips the emboss and vignette to a light
  scrim, and the placeholder/underline move to the on-surface faint token, so the
  hero separates cleanly in either mode. The mode chips (ALL/IMG/FILE/REC) shared
  the same fault: an idle chip fill hard-coded to black with dark on-surface text
  rendered dark-on-dark on a light wallpaper. Their idle fill, hover and border
  now flip with `Tokens.light`, so the chips read in either mode.
- **GTK 4 and libadwaita accents follow the palette instead of stock blue.** The
  palette only ever shipped `@define-color` names, but libadwaita 1.6+ recolours
  its own widgets from CSS custom properties and no longer reads named colours for
  them, so every accent stayed Adwaita blue while the surfaces went warm with the
  wallpaper. The GTK 4 stylesheet now leads with the `:root { --accent-bg-color, ... }`
  custom properties libadwaita actually honours and keeps the `@define-color` set
  below it for libadwaita before 1.6 and for GTK 4 apps that are not libadwaita,
  while GTK 3, which cannot parse custom properties, keeps the named-colour form it
  can read. The single `matugen/templates/gtk-colors.css` splits into
  `gtk4-colors.css` and `gtk3-colors.css` to carry the two shapes.
- **A light scheme no longer leaves GTK apps on the dark theme.** The curated
  light and dark schemes are painted by the Hub, which idles the daemon's paint
  worker, so the mode flip set the colour-scheme preference but nothing ever
  resolved the matching GTK theme name: picking Light kept every GTK 3 app on
  `adw-gtk3-dark` over a light stylesheet. The daemon gains a `gtk apply
  <light|dark>` verb and the Hub calls it instead of writing the preference
  itself, so one place still owns `gtk-theme`, `color-scheme` and `accent-color`
  and the two can no longer disagree.
- **Theme apps off stays off.** The master switch is written by the Hub, which
  blanks the GTK stylesheets, but the daemon gated its own render on the
  appearance page's per-group roster alone. Nothing re-rendered right after the
  toggle, so the blank used to survive by luck; once the toggle asks for a
  repaint the daemon rebuilt the stylesheets and silently undid it. Both
  switches are now read where the rendering happens.
- **Where the palette lands, stated honestly.** An already-open GTK app keeps
  the colours it started with. Measured on this session: a running GTK 3 or
  GTK 4 app picks up neither a rewritten `~/.config/gtk-*/gtk.css` nor a changed
  `gtk-theme`, so the long-standing comment claiming the theme-name flip forces
  a re-read was wrong. Every app opened after a wallpaper change is correct, and
  the code now says so rather than promising a repaint it cannot deliver.

### Added
- **A browser palette host lands the wallpaper scheme in Firefox and Chromium.**
  `ryoku-shell browser-host` is a WebExtension native-messaging host that streams
  the live palette (colors.json) to the Ryoku Theme extension, so a browser
  retints on every scheme change. `ryoku doctor` installs the per-user host
  manifests for each detected browser; the extension ships in `ryoku-desktop`.
  Mapping and native-messaging model ported from caelestia-dots.
- **Shell animations gain a Material 3 motion vocabulary, a global speed, and
  reduce-motion.** The ui token sheet grew a full Material Motion set (the
  Standard and Emphasized durations plus the expressive spatial and effects
  curves as `easing.bezierCurve` control points), an `Anim` component that
  carries a role's duration and curve, and a `Tokens.dur(ms)` scaler. Every
  duration run through `dur()` (so all `Tokens.snap/move/swap/flap`, and any raw
  literal migrated to `dur(N)`) now tracks a global speed and a reduce-motion
  switch read live from `shell.json` `theme.motion` (`scale` 0.25 to 3, `reduce`).
  Ported from caelestia-dots/shell (`plugin/.../Config/tokens.hpp`, `Anim.qml`).
- **The recolor engine themes five more apps: Obsidian, Kvantum, Zathura,
  Alacritty, and tmux.** New matugen templates fan the wallpaper palette into
  each (ported from dusklinux/dusky and mapped onto Ryoku's Material 3 roles),
  gated by the existing per-app roster and the Theme apps toggle. Obsidian is
  wired live by a `ryoku doctor` reconciler that links the generated snippet into
  every vault and enables it; Kvantum installs a dedicated `ryoku` theme so Qt
  apps follow too.
- **Game Mode keeps a fullscreen game rendering off-screen.** Switching away from
  a game's workspace used to stop it dead: still loading and it never finished,
  mid-match and it locked up, audio still playing, input ignored, multiplayer
  timing out. Not a crash and not the game's fault. A Wayland client draws in
  response to `wl_surface.frame` callbacks and an invisible surface receives none,
  so a render loop waiting on them is never scheduled again; the netcode is
  usually pumped from that same loop, which is why the connection goes with it.
  Game Mode now adds Hyprland's `render_unfocused` rule.
  Scoped to fullscreen, deliberately twice over. Fullscreen rather than a class
  list so it covers any launcher (Steam, Lutris, Heroic, a bare binary) instead of
  only the ones we thought to name; and fullscreen rather than every window so the
  desktop behind the game is not also rendered off-screen, which would spend the
  GPU the game needs. Only while the toggle is on, because rendering a window
  nobody is looking at costs real power and heat.
  Verified that Ryoku's Lua window-rule API accepts the field, against a control:
  an unknown field errors, `render_unfocused` does not.
- **Game Mode is reachable.** The whole chain already existed and nothing called
  it: `Flags.gameMode` persisted the state, `shell.qml` drove
  `ryoku-cmd-game-mode` on change, and DND pulled on, but no tile, pill or
  keybind ever invoked `Toggles.toggleGame()`. Quick settings now carries a
  `Gaming` tile next to Keep awake and Do not disturb.
  It is AC-only, because every lever it pulls trades power and heat for latency
  and on battery that is a bad deal the user cannot see. Rather than let the tap
  fail into a notification, the tile reads `Needs AC` and goes inert while
  discharging, so the constraint is visible before the click. A machine with no
  battery is always eligible.
- `Perf`: Game Mode is a fourth input to the performance policy, and the only
  tier that overrides the user's own toggles. The compositor is already stripped
  and the CPU already pinned to performance by then, so leaving the shell's own
  eye-candy running would spend the headroom that was just bought. Motion, blur
  and shadows go off, MSAA drops to its floor, background polling backs off
  further than on battery (each poll is a process spawn competing with the game
  for the same cores), and the audio analyser is dropped outright rather than
  merely frozen when idle: `ParticleStream` releases its cava claim, so the
  process exits instead of sampling for a visualiser nobody is looking at.
  Nothing is persisted, so every switch returns to the user's settings on exit.
- `QsTile`: an `available` property for a tile whose action cannot be taken right
  now. It stays visible and keeps explaining itself through its sub-label, but
  reads inert and swallows the tap. Defaults available, so the six existing tiles
  are unchanged.
- **The Cobalt engine switch sets itself up.** Turning it on used to require the
  user to already have Docker installed, its service running, and their account
  in the `docker` group. When any of that was missing the switch said so and did
  nothing about it: "Start docker.service or add yourself to the docker group",
  which names two chores and performs neither. It now opens a modal wizard that
  performs them, one step at a time, each reporting for itself: runtime present,
  start the service, grant container access, pull the image (the step that
  explains the one-time wait), start cobalt. It ends by flipping the switch.

  There is no reboot step, because the privileged work goes through
  `ryoku-docker` over polkit rather than through this session's groups, so the
  engine works in the session the user is already in. The access step says plainly
  that plain `docker` on the command line arrives at the next login, so the
  difference between "the feature works now" and "the CLI works later" is stated
  instead of discovered.

  A failure names the step that failed and carries the helper's own message, with
  a Retry that re-runs the flow; every step converges, so a retry over finished
  work is a no-op. The modal cannot be dismissed while a privileged step is in
  flight, which would otherwise leave a half-provisioned host with nothing
  watching it. All the state lives in `Stash`, so the flow is testable without a
  window: `tests/ui/cobalt-wizard-probe.sh` drives it through idle, in-flight,
  failure, reset and done and asserts what the view actually renders
  (`modules/bar/panel/CobaltSetupWizard.qml`, `services/Stash.qml`).
- **The daemon re-applies your CPU profile definition after ppd switches.** The
  goroutine already listening to power-profiles-daemon's `PropertiesChanged` now
  schedules a debounced `ryoku-power apply-profile` for whatever profile is active
  once the dust settles, so the definitions edited on the Hub's Machine page win
  over ppd's own governor/EPP/platform_profile writes. No new daemon and no
  polling: it hangs off a signal subscription that already existed.

  The profile is resolved when the timer fires rather than when it is scheduled,
  because at signal time ppd may not have published the new `ActiveProfile` yet
  and reading it early applies the outgoing profile over the incoming one. The
  exec runs on the timer goroutine, never the signal goroutine, so a slow helper
  cannot stall signal handling. If `power.json` has no `profiles` block, or none
  for the active profile, nothing is executed at all -- a user who never defined a
  profile never triggers the helper and never sees an authentication prompt
  (`ipc/powerprofiles.go`).
- **Animated key presses for screen recordings.** Recording settings now include
  a built-in keycap overlay with fixed dark and light palettes, an optional
  shortcuts-only privacy mode, a draggable pre-recording sample, repeat counts,
  and a smooth horizontal history of the latest three chords. Modifier chords use
  Ryoku's Super-first order, shifted keys show their symbols, and each entry fades
  cleanly in and out. Input is read from ordered evdev events only while preview
  or recording is active. The overlay becomes click-through once recording starts
  and disables itself when recording or the shell ends
  (`modules/capture/KeypressOverlay.qml`, `services/Keypresses.qml`,
  `ipc/keypress.go`, `../hub/quickshell/pages/RecordingPage.qml`).
- **A plugin popout can sit in the middle of the screen.** `framePopout` accepts
  `edge: "center"`: the body floats at the exact centre of its monitor with all
  four corners rounded, no edge weld, and no hover band, so it opens only from
  `ryoku-shell plugin <id>`, a keybind or a pin, which is what a modal plugin
  view wants. The frame carries a centred body's rect in its own input mask (it
  has no edge anchor to ride), so it catches clicks in every bar style
  (`modules/bar/popouts/Popout.qml`, `modules/bar/PluginPopouts.qml`,
  `modules/bar/FrameMenuManager.qml`, `modules/bar/Frame.qml`).
- **The bar's weather glyph shows your city, not your IP's.** The qsbar weather
  widget curled `wttr.in` with no location, so it read the IP geolocation of the
  exit node while the dashboard it opens (and every other readout) showed the
  configured `weatherLocation`: two different cities, side by side. It now binds
  the shared daemon-fed `Weather` singleton like the dashboard does, so it follows
  the set location and unit, updates the moment either changes, and makes no HTTP
  call of its own. The bar's own imperial toggle no longer double-converts a shell
  already set to Fahrenheit, and the WMO-code to Nerd Font map lives beside the
  other glyph maps in `services/lib/weather.js` (`nerdFor`, unit-tested)
  (`modules/bar/barstyles/qsbar/modules/WeatherWidget.qml`).
- **Apple Music is a first-class music source.** The now-playing daemon detects
  Sidra and Cider (and any `music.apple.com` web player) as `applemusic` rather
  than lumping them in with "other"; their synced lyrics and cover art already
  flow through the same LRCLIB and iTunes path as every other player, so an Apple
  Music client now works end to end (`ipc/music.go`).
- **Fiction and gaming launcher logos.** 36 more picker marks from the shipped
  Nerd Font: Star Wars factions (Empire, Jedi, Sith, Mandalorian, First Order,
  Death Star), consoles (PlayStation, Xbox, Switch, Minecraft), arcade (Space
  Invaders, ghost, Pokeball), and tabletop or fantasy marks (d20, dungeon,
  wizard, sword, shield, crown, skull, ninja, robot, alien, chess knight), plus
  film and theatre-mask marks. All real font glyphs, none hand-drawn; a Berserk
  or specific anime mark is not in any shipped font, so it is intentionally not
  included (`modules/bar/barstyles/qsbar/Theme.qml`).
- **A big launcher-logo collection.** The bar's logo picker gains 66 brand, OS
  and dev marks (Ubuntu, Debian, Fedora, Gentoo, Void, NixOS, Tux, Apple,
  Android, GNOME, Plasma, Python, Rust, Go, Docker, GitHub, Firefox, Neovim,
  Spotify, Discord, and more) plus ten wordmarks. They are real SpaceMono Nerd
  Font glyphs (font-logos, devicons, font-awesome), so none is hand-drawn and
  each is guaranteed to render; the glyph map is now a lookup keyed by option id.
  The `nix` mark, which pointed at the FreeBSD glyph, now shows the real NixOS
  snowflake (`modules/bar/barstyles/qsbar/Theme.qml`).
- **Auto-hide for the qsbar.** A new toggle (the Bars center menu and Bar
  Studio) slides the bar off its edge and reserves no space, so windows reclaim
  the strip; a slow, smooth hover anywhere along the edge reveals it (over the
  gaps between islands too, where no pill sits) and it slides back out on leave.
  Works across all five forms (islands, full, fit, dock, notch) and with the gap
  animation on or off (`modules/bar/barstyles/qsbar/BarSlot.qml`, `Theme.qml`,
  `modules/bar/barstyles/qsbar/controlcenter/routes/BarsRoute.qml`).
- **A unified time + weather dashboard.** Clicking the bar's clock/weather/date
  group opens one connected panel: a bento of frosted islands with a month
  calendar, a live clock, current conditions, a wind/humidity/rain/feels island,
  and a temperature area-graph for the coming hours. A weather-condition
  animation plays behind the clock (rain/snow fall, storm lightning, clouds
  drift, sun rays by day, stars twinkle by night, haze for fog); tiles lift on
  hover and the graph draws in on open, all off under reduce-motion. Binds to the
  daemon-fed `Weather` singleton and replaces the standalone clock, weather and
  calendar popups (`modules/bar/barstyles/qsbar/panels/DashboardPopup.qml`,
  `modules/bar/barstyles/qsbar/modules/WeatherFX.qml`).

- **A qsbar-style dock on the opposite edge.** An app dock on the screen edge
  opposite the bar (so the two never overlap), rendered as the same frosted
  islands the bar uses with the reactor wave flowing in the gaps between them.
  Apps group into pinned launchers and running apps (the ones with window
  previews), split by a separator; magnify tracks the cursor across the whole
  band and honours reduce-motion. The dock updates live -- an app that opens or
  closes appears or leaves without a shell reload -- and islands animate: a new
  app pops in and the rest slide to their new spot (both off under reduce-motion).
  Hovering an icon grows a window-preview strip that shows EVERY open window of
  that app across all workspaces, stays open while you move onto it, and gives
  each tile an X to close that window; it works in every bar style and clears the
  dock instead of overlapping it. Enable the dock plus Frost/Depth/Magnify from
  the Control Center Bars route; pins persist in `shell.json .qsbar`. The dock
  model is shared with the framebars dock via `services/Dock.qml` and the tested
  `services/lib/dock.js` (`modules/bar/barstyles/qsbar/DockSlot.qml`,
  `modules/bar/barstyles/qsbar/modules/DockRow.qml`,
  `modules/bar/popouts/DockPreviewPopout.qml`, `modules/bar/Frame.qml`).

### Changed
- **The LED worker now only writes to adopted devices, and stops when lighting
  is off.** `ipc/wallpaper.go`'s LED worker calls `ryoku-hub lighting accent`
  instead of `ryoku-leds apply`, so a palette change only reaches the devices the
  user handed to Ryoku and does nothing at all while lighting is off.
- **A redeploy no longer resets your default apps.** `deploy.sh` copied Ryoku's
  map over `~/.config/mimeapps.list` on every run, wiping the picks a dev box had
  made; it installs the map to `/usr/share/applications/mimeapps.list` now (the
  same place the package puts it, skipped cleanly without sudo) and leaves the
  user's file alone.

### Fixed
- **Setting a live wallpaper no longer shows another clip first, or paints the
  desktop white.** Every clip's sampled still was written to one shared file,
  `ryoku-live-frame.png`, by three different callers: the backdrop's still, the
  palette pass, and ryowalls' preview of whatever tile the pointer was on. So the
  frame the desktop showed and the frame matugen read could belong to a clip the
  user did not pick. Caught on this box: the stored still measured 0.612 luma
  while the clip it claimed to represent measures 0.080 across its whole run, and
  0.612 is what flipped smart mode to light. That is the white launcher, the white
  terminal and the grey fonts, over a dark wallpaper.
  Stills are cached per clip, source mtime and frame offset now
  (`ryoku-live-frames/<clip>-<mtime>-<offset>.png`), written to a temp file and
  renamed in, so no caller can hand another its frame and no reader sees a
  half-written PNG. Repeated sets and previews of one clip reuse the still instead
  of re-running ffmpeg, the cache is capped at 24 stills, and the old shared file
  is removed when found.
  Smart light/dark for a clip now reads the whole clip (a frame a second over its
  first minute) instead of the one frame the palette was sampled at, so a dark
  wallpaper with a bright second in it stays dark. `ffmpeg -update 1` for the
  still extraction, which ffmpeg 8 warns about and a later release will refuse.
- **Super+Escape no longer stalls the machine while it opens.** The sidebar's
  brightness fader asks `ddcutil detect` for the external monitor list, and that
  walks every i2c bus on the box: 28 of them on a hybrid laptop, 11.5s on a cold
  ddcutil cache, all of it i2c traffic on the display controller, which reads as
  the cursor stuttering and the whole session freezing. The startup prewarm was
  meant to keep it off the open path, but `detect()` restarted a walk that was
  already running, closing the sidebar killed the walk, and the "already
  enumerated" flag was only set when the output stream ended. So an open during
  the prewarm restarted the whole walk mid-animation, and one close left the
  answer unknown for every open after it.
  A DRM connector read gates the walk now: `status` per connector is a file read,
  so a machine with nothing but its own panel plugged in never touches i2c (0
  ddcutil spawns at login and across four open/close cycles, measured). A box with
  an external display walks the bus once per connector set, never restarted and
  never killed, and the cached set follows Hyprland's monitor events so a display
  plugged in after login gets its fader without a restart.
  The parse was wrong too: `ddcutil detect` lists non-DDC displays in a preamble
  before its first `Display N` marker, and that block counted, so the internal
  panel came back as a second fader writing to a bus nothing answers.
- **Opening an app from the desktop no longer draws a second desktop.** Starting
  ryowalls or ryoport from the launcher brought up a whole second shell over the
  first: two bars, two docks, two wallpapers, and one GPU paying for both.
  Quickshell's crash handler leaves `__QUICKSHELL_CRASH_INFO_FD` in the
  environment of an instance that has crashed once, naming an inherited memfd
  (dup'd without CLOEXEC upstream) that records the config it came back from, and
  `checkCrashRelaunch` reads that variable before it parses a single argument. So
  `qs -c ryowalls` launched from the desktop ignored `-c ryowalls` and relaunched
  `shell/shell.qml`: `qs list --all` showed the process started as `qs -c
  ryowalls` running `Config path: .../shell/shell.qml`, while the same command
  from a clean environment loaded ryowalls correctly.
  Every launch that can end up running Quickshell now goes through the new `Spawn`
  singleton, which unsets the handle for the child, and `ryoku-shell` clears it
  from its own environment at startup so the `qs` calls it makes (the Hub, the
  surfaces, the ipc clients) are clean without any call site remembering.
  `services/AppLaunch.qml` stops using `DesktopEntry.execute()` for this reason:
  it hands the child the desktop's whole environment and takes no argument that
  could trim it.
- **The shell stops analysing silence.** Idle, with nothing playing, the shell sat
  at 16.2% of a core and Hyprland at 6.8%, with two cava processes seven and four
  hours old. A surface that is never done being damaged keeps the compositor awake
  with it, and at 240 Hz the entire frame budget is 4.1 ms, so dragging a window
  is what visibly loses: a 240 Hz panel that felt like 30-50 Hz. Three separate
  faults, each sufficient alone:
  - **Bindings destroyed by assignment.** `cavaProc.running = true` from a restart
    timer (once in `AudioBars`, four times in `Spectrum`) replaces the declarative
    binding to the gate. From the first restart onward `running` followed nothing:
    not playback, not the pill policy, not Power Saver, not Game Mode. Restarts
    are a backoff window now, so the binding is never taken away.
  - **"WhenIdle" that never meant it.** `freezeVisualizerWhenIdle` and
    `freezePillWhenIdle` were folded in as plain always-off switches, so the only
    choices were analyse silence forever (the default, and what shipped) or never
    analyse at all. Silence is the gate now, unconditionally; the hard tiers
    (Saver, lowPowerMode, Game Mode) still win mid-track.
  - **A second analyser with no policy.** `Spectrum` drove its own cava off
    surface visibility alone, ignoring the visualiser freeze switch, Saver,
    lowPowerMode and Game Mode, and ran beside the pill's whenever the surface
    existed. It mirrors `AudioBars` now, including flattening levels when analysis
    stops: the settle timer only runs while analysing, so stale peaks kept
    `Motion` reporting "sounding" and the picture animating after the music ended.
  Measured from a fresh session with nothing playing: shell 16.2% to 1.0%,
  Hyprland 6.8% to 5.4%, cava 2 to 0. Verified against a real MPRIS source that
  both analysers still spawn, the spectrum flows, and both exit when it stops.
- **The ambient particle stream is the power profile's call again.** It was ambient
  decor by design, and the design predates anyone measuring what continuous canvas
  repaint costs the compositor. Freezing it on any silence stopped it on
  Performance and Balanced too, which read as the bar being broken. Silence now
  freezes it only where the profile says so: Saver, lowPowerMode and Game Mode
  refuse it, Balanced and Performance let it drift, and the module's own pacing
  decides how fast. The analyser is a separate question with a separate answer:
  there is nothing to spectrum-analyse in silence, so cava stops on every profile.
  A coarse floor on the idle repaint was tried in between and reverted. About 7fps
  reads as steppy rather than as ambient drift, which is a worse failure than the
  cost it was meant to save, and it bought no saving that survived measurement.
  Numbers here are deliberately loose, because the obvious method does not measure
  well: comparing by switching power profiles has a noise floor around 5 points on
  this hardware, since the switch itself churns blur, shadows and a Hyprland
  reload. Performance and Balanced take the identical path and still read 8.9%
  against 14.3%. What holds across every run is the direction and rough size,
  silent desktop: drift refused 1-3%, drift allowed 8-14%. A single repaint costs
  ~11 ms by itself, a full canvas clear plus a full-width texture upload, so the
  fix that would actually pay is making the frame cheap rather than rare.
- **The system-stats panel stops waking a sleeping discrete GPU.** Its 1.5s poll
  ran `nvidia-smi` unconditionally, and that drags a runtime-suspended card back
  out of D3 (about 10 W on a hybrid laptop), so an open panel pinned the dGPU
  awake and quietly cancelled the saving. It now reads the driver's
  `power/runtime_status` first and reports no GPU while the card sleeps, landing on
  the same rest-at-zero path as a machine with no NVIDIA card. The bar's GPU
  telemetry already did this; `StatsFeed` was missing it
  (`services/StatsFeed.qml`).
- **The bar-gap animation stops burning a CPU core when nothing is moving.** Only
  mode 7 (reactor) self-paced; every other `barAnim` mode drove its `Canvas` at a
  fixed 30fps (modes 1-4) or 60fps (modes 5-6) forever, even while its picture sat
  nearly still. The per-frame full-width `clearRect` plus canvas texture upload
  dominate the cost, so an idle field cost as much as an active one - mode 3 (Bolt)
  alone ran ~40% of a qs core (~half a core with the compositor) and lifted
  package temperature ~17 C. The pacing mode 7 already used is now shared by every
  mode through one `canvas.paintTick` the paint sets from what the frame actually
  drew: full rate while something moves fast, a slow ambient rate while only a
  faint crawl, swell or fade remains, and a coarse watch rate while a gap is dark -
  re-armed to full a lead ahead of the next burst so no onset is clipped. Bolt,
  whose charged field only drifts and fades (no positional motion), drops from
  ~40% to ~9.5% qs CPU; Surge halves (~18% to ~8%); off, reactor and quotes are
  unchanged, and the always-moving Stream marquee only sheds frames when the bar
  has no gaps (`modules/bar/barstyles/qsbar/modules/ParticleStream.qml`).
- **Power Saver now actually silences the pill's spectrum analyser.** `Perf`
  exported a `pillFrozen` policy (set by `lowPowerMode` or the Power Saver
  profile) and its header claimed the shell obeyed it, but nothing read the
  switch: `AudioBars` gated the cava analyser on the owner refcount alone, so any
  visible music widget or card held a 40-band/30fps cava process open and burning
  ~1.5% CPU even in Power Saver. The analyser now runs only while a surface claims
  it and the policy permits a live feed; when frozen it drops cava and its restart
  and settle timers, and the bars fall to their rest slivers through the same
  reset the refcount path uses, rather than freezing on the last peak
  (`services/AudioBars.qml`).
- **The bar stopped making Hyprland look like it had a broken config.** qsbar
  worked out whether the live Hyprland config is Lua or classic hyprlang by
  dispatching a deliberately malformed token (`hl.dsp`) and reading the error
  back. Hyprland files that Lua error in the very buffer `hyprctl configerrors`
  reports, so every bar start left the session looking like it was rejecting its
  config until the next reload, and `ryoku doctor` dutifully warned about it. The
  probe now sends the Lua form it actually wants to use, focusing the workspace
  already focused (`hl.dsp.focus({ workspace = "e+0" })`, so nothing moves): Lua
  answers ok and files nothing, classic rejects an unknown dispatcher and the bar
  falls back to `workspace N` as before
  (`modules/bar/barstyles/qsbar/Theme.qml`).
- **A surface whose output was rebuilt no longer comes back inert.** Per-monitor
  state was looked up by ShellScreen object identity while the sibling lookup used
  the output name, and an output that is disabled and re-enabled (a lid close with
  an external attached, a modeset) returns as a new object under the same name. The
  identity lookup then resolved to nothing exactly while a surface was being rebuilt
  for it, and every binding reading that slice kept its stale value. Both lookups go
  through one name-keyed rule now (`services/lib/screens.js`, `services/ShellState.qml`).
- **The bar stops giving up on its own window.** A closed layer window was retried
  three times and then abandoned, leaving the bar dead for the rest of the session
  with only a hand-typed `ryoku reload` to bring it back; the same path also bailed
  outright when the screen had no geometry yet. It now waits for the screen and
  keeps retrying on a backoff that caps, so a window needing a moment returns on its
  own (`modules/bar/barstyles/qsbar/VariantRoot.qml`).
- **The wallpaper switcher stops throwing while its output goes away.** `isFocused`
  read `screen.name` unguarded on a screen that is null mid-teardown, and it feeds
  the surface's keyboard focus, so the failed binding left the previous value
  standing on an overlay that spans the display
  (`modules/wallpaper/switcher/Switcher.qml`).
- **The low-battery chime no longer fires while the battery is fine.** `watchPowerSounds`
  scanned every `/sys/class/power_supply` entry and let any `type=Battery` device
  overwrite the reading, so a depleted, discharging peripheral (a `scope=Device`
  mouse, keyboard or headset) stood in for the laptop's own percent and the critical
  chime played on a loop while the real battery sat at ~70%. The reader skips
  `scope=Device` supplies now and keys off the system battery alone; the threshold
  and its repeat are untouched (`ipc/powersounds.go`).
- **Terminal apps start from the launcher again.** Quickshell parses `Terminal=true`
  into `runInTerminal` but spawns no terminal for it, and every launch path called
  `execute()` regardless, so a TUI got no controlling tty and exited on the spot
  with no window and no error: btop, yazi, nvim, vim and micro all ship in the base
  set and none of them could be started. A new `services/AppLaunch.qml` is the one
  place an XDG entry is launched, handing the argv to `ryoku-app terminal --` when
  the entry asks for a terminal, and the launcher, its desktop actions, the dock
  and the okshell variant all go through it
  (`modules/launcher/shared/providers/apps/Apps.qml`, `services/Dock.qml`,
  `modules/launcher/variants/okshell/Main.qml`).
- **Ryoku Settings stops opening for the rest of the session.** The Hub is spawned
  under `flock /tmp/ryoku-hub.lock`, but without `-o` the Hub inherited the locked
  descriptor and passed it to everything it spawned, so one detached grandchild
  (Ryostore from Browse rices, a kitty, an `xdg-open`) held the lock after the Hub
  itself was gone. Every later open then failed the lock and threw the failure
  away, which is why no keybind, menu entry or gear button did anything until a
  reboot. `flock -n -o` keeps the lock with flock, where it dies with the Hub
  (`ipc/control.go`).
- **`deploy.sh` no longer restarts into a black screen.** A renderer that cannot
  load (an AUR quickshell built against another Qt) turned a deploy into a dead
  desktop with no way back, which is what a plugin developer hit. Deploy runs
  `qs --version` first: if the renderer will not start it prints the loader error
  and the command that fixes it, and leaves the running desktop alone.
- **A surface that dies on start says why.** The supervised Quickshell's stderr
  went nowhere, so a failure to load was invisible and `ryoku reload` answered ok
  while the surface died on the same error a moment later. Its stderr is kept at
  `~/.local/state/ryoku/surfaces/<name>.log`, a death within three seconds is
  logged with its last line, and `reload` waits for the surface to come back and
  reports that line instead of ok (`ipc/daemon.go`).
- **`Ryoku.Blobs` is rebuilt when Qt changes.** The QML module was built once and
  left, so a Qt update turned it into a module that cannot load, taking the
  surface down with it. `deploy.sh` stamps it with the Qt it built against and
  rebuilds when that changes, the way the Hyprland plugins are stamped against
  the compositor.
- **The now-playing sheet no longer strobes between lyrics and the visualizer.**
  Two causes: the player pick had no stickiness, so a second player (a browser
  tab, a game bleep) could steal it and reload the track mid-song, and the side
  area flipped to the visualizer on any transient gap while lyrics were still
  loading. The pick is now sticky (it keeps the sounding player), and the
  lyrics/visualizer switch has hysteresis, so the visualizer only takes over once
  a track has settled with no lyrics (`services/Media.qml`,
  `modules/desktop/music/MusicWidget.qml`).
- **The desktop no longer ends up drawn twice.** A shell surface left over from a
  daemon that was killed rather than asked to quit kept rendering, and the
  replacement daemon drew a whole second desktop on top of it: two bars, two
  wallpapers, double the memory, for the rest of the session. Quickshell allows
  two instances of one config, so nothing else refused it, and the old reap could
  not clear it: it matched only the argv form the running daemon happened to use
  (a packaged `-c shell` never recognised a checkout's `-p .../quickshell/shell`),
  it signalled without waiting, and it ran once per daemon rather than before
  each start, so a leftover that ignored SIGTERM (a wedged Qt process, which is
  what a busy machine launching an app produces) survived forever. The reap now
  matches either selector form and either binary name, waits for the process to
  actually go and SIGKILLs what will not, and runs before every surface start;
  `shutdown` waits for its children instead of signalling and exiting, so a
  restart cannot hand a live surface to the next daemon; and the daemon holds an
  exclusive lock beside its socket, so a cleaned runtime dir can no longer let two
  daemons supervise two desktops (`ipc/daemon.go`).
- **The QS Bar gap animation no longer strands a frozen frame in Power Saver.**
  Reduce-motion (Power Saver) froze the reactor/stream canvas to its last frame;
  a shell reload in that state re-created the canvas, painted one transitional
  frame, then froze it, stranding a half-drawn wave over the bar. The gap
  animation now turns off entirely under reduce-motion, so there is no frozen
  frame to strand, and it turns back on when Balanced or Performance is active
  (`modules/bar/barstyles/qsbar/modules/ReactorLayer.qml`).
- **The QS Bar pacman workspaces no longer smear in Power Saver.** The pacman
  `travel` animation (the runner that chomps between cells) ran on every
  workspace switch regardless of the reduce-motion policy Power Saver sets,
  unlike every other shell animation. With the compositor throttled under Saver,
  a switch could strand the runner and its eat state until an unrelated relayout
  (toggling the power profile cleared it by hand). It now snaps instantly under
  reduce-motion and resets any in-flight travel the moment Saver engages, so the
  runner can never be stranded
  (`modules/bar/barstyles/qsbar/modules/WorkspaceWidget.qml`).

### Added
- **ryoshot pins a shot to the desktop.** Ctrl+P (or the toolbar pin) renders the
  capture, drops it in `$XDG_RUNTIME_DIR/ryoku/pins/` and hands it to `ryopin`, a
  new on-demand Quickshell surface that shows it as a floating always-on-top card
  on every workspace. ryoshot then exits, so the pin outlives it. Drag the card to
  move it, scroll to resize it within a third of the screen width and half its
  height, and hover it for edit (reopens the PNG in ryoshot), copy, copy path and
  close. One host process holds every pin: a later pin wakes the running host
  through a poke file rather than starting a second one, and the host quits when
  its last pin closes, so pins cost nothing while none exist
  (`quickshell/ryopin/`, `quickshell/ryoshot/shell.qml`).
- **ryoshot redacts instead of pixelating.** The pixelate tool became **Redact**
  (`d`). A downscale keeps the source's spatial structure and can be read back;
  the mosaic now samples the region's six dominant colours and paints fixed
  blocks in a seeded pseudo-random order, so the output carries no information
  about what was under it. Press `d` again for a solid block instead. The blocks
  stay solid until the palette is sampled, so the source is never briefly legible
  (`quickshell/ryoshot/EffectLayer.qml`, `lib/redact.js`).
- **ryoshot gains a spotlight, an OCR grab and shift constraints.** Spotlight
  (`s`) dims the rest of the shot and magnifies its lens, cycling ellipse,
  rectangle and rounded rectangle on repeat presses; the wheel zooms a selected
  lens between 1x and 4x. Copy text (`g`) drags a region and runs it through
  `ryoku-cmd-ocr`, the same OCR path as Super+D, so a blur or redaction placed
  over text is honoured and the text under it cannot be recovered. Holding Shift
  while drawing makes rectangles, ellipses and spotlights square and snaps lines
  and arrows to 45 degrees (`lib/constrain.js`).
- **The captured region can be recropped after the fact.** Eight grips (four
  corners, four edges) sit on the region through the whole editing phase, each
  with the matching resize cursor, anchoring the opposite side and refusing to
  invert. On a selection spanning two monitors each edge grip is drawn once, by
  the head that actually contains it (`quickshell/ryoshot/ResizeHandles.qml`,
  `lib/hittest.js`).
- **Every ryoshot tool has a key, and remembers how it was left.** One tool list
  now drives the toolbar, the tooltips and the key handler, so they cannot drift:
  `v r o l a p h n t b d s z g`. Colour, width and fill are stored per tool in
  `~/.config/ryoku/ryoshot.json` and restored on the next launch, the wheel
  resizes the live stroke or text while drawing, `[` and `]` step the width,
  `1` to `8` pick a swatch, `f` toggles fill, and `?` opens a card listing the
  lot. A colour popover adds an HSV square, a hex field and an eyedropper that
  samples the shot itself; Ctrl+P pins, Enter copies and saves
  (`quickshell/ryoshot/`).
- **ryoshot settings persist.** Blur radius, mosaic block, zoom factor, copy on
  save and a save folder join the existing key rebind in the settings panel, all
  written to `~/.config/ryoku/ryoshot.json` (`quickshell/ryoshot/SettingsPanel.qml`,
  `Singletons/Config.qml`).
- **The volume panel can boost past 100% for quiet hardware.** The bar's audio
  controls capped at 100% (0 dB), so a machine that is simply quiet at unity had
  no way to get louder from the shell. A **BOOST ABOVE 100%** toggle in the
  volume panel lifts the ceiling to 150% (about +10.5 dB); off by default, so
  nothing over-drives unless asked. The pill wheel and the Volume-Up key both
  honour it (`Config.qsbar.audioBoost` in `shell.json`), so raising volume by
  key also boosts when it is on and stays capped at 100% when it is off
  (`modules/bar/barstyles/qsbar/panels/VolumePanel.qml`,
  `modules/bar/barstyles/qsbar/modules/AudioWidget.qml`,
  `hyprland/modules/binds.lua`).
- **qsbar's two bar variants merge into one.** The V1 (split-pill) and V2
  (continuous-surface) bars were separate engines swapped at runtime by a
  variant host, so the shell carried both trees plus a state file recording the
  live choice. They are now a single bar: `variants/V2/` was promoted up, the
  runtime variant switch (`core/VariantHost.qml` and `core/StateService.qml`)
  and the active-variant state file were removed, and there is one
  `VariantRoot`, one `Theme`, and one `BarSlot`. The bar form is now a single
  `barShellStyle` setting: `"islands"` (the split-pill form that replaced the
  V1 engine), `"full"`, `"fit"`, `"dock"`, or `"notch"`, set on the qsbar
  `Theme` and persisted to `shell.json`; the IPC `variant` (activate) and
  `lifecycle.version` targets are gone (`lifecycle.ready()` stays)
  (`modules/bar/barstyles/qsbar/`).
- **The stash download section gets a cobalt engine switch.** The Super+S Tools
  "Download" section silently probed a non-existent cobalt at `localhost:9000`
  and fell back to yt-dlp, whose TikTok extractor is broken upstream, so TikTok
  links failed. A new "Cobalt engine (Docker)" switch runs a local cobalt
  container (`ghcr.io/imputnet/cobalt:11`, loopback-only) on demand: off = yt-dlp
  only, on = downloads route through cobalt with yt-dlp still catching whatever
  it declines. Docker is detected (not force-installed), so the switch is disabled
  with an install hint when it is missing, and shows a starting/resource loader
  on first launch (image pull). State persists in `~/.config/ryoku/stash.json`
  and reconciles to the real container state on shell start; the "works with"
  bubble now reflects the active engine. New `stash-cobalt-server.sh` manages the
  container; `stash-cobalt.sh` skips the cobalt probe when the engine is off
  (`services/Stash.qml`, `modules/bar/panel/PanelTools.qml`,
  `hyprland/scripts/stash-cobalt{,-server}.sh`).
- **The bar clock stops waking the CPU every second.** `ClockWidget` (both bar
  variants) shows only `HH:mm` but drove a 1s `Timer`, waking the CPU 60x a minute
  on an always-visible surface and blocking deep CPU sleep states. It now reads
  Quickshell's `SystemClock` at minute precision (like `RailClock` already does),
  so it ticks once a minute. Identical display, far fewer idle wakeups (a battery
  win). The desktop analog face keeps its per-second tick for the sweeping second
  hand (`qsbar/modules/ClockWidget.qml` and its V2 variant).
- **Power Saver drops the desktop glass blur.** The frosted-glass backdrop behind
  desktop widgets ran a `blurMax: 64` `MultiEffect` pass unconditionally, unlike the
  launcher and visualiser blurs that already gate on the derived switches. The
  desktop `Performance` forwarder now exposes `blurDisabled`, and `WidgetGlass`
  gates its blur on it, so Power Saver (or lowPowerMode) drops the pass and its blur
  buffers, leaving a flat dimmed plate. Completes the saver blur policy across the
  compositor, launcher, visualiser, and desktop layers
  (`modules/desktop/{WidgetGlass,Singletons/Performance}.qml`).
- **Power Saver reclaims resident-surface RAM.** The daemon's idle-unload workers
  (the launcher/overview palettes, the audio visualiser, and the covered-desktop
  widget layer) now unload under the Power Saver profile even when the user turned
  the per-surface `unload*` toggles off, matching how the QML side already forces
  its idle freezes under Saver. Someone who keeps surfaces warm for speed still
  gets a one-switch RAM reclaim by choosing Saver, and Balanced/Performance leave
  the toggles untouched (`ipc/{audiowatch,idlewatch,widgetwatch}.go` via the shared
  `saverActive`).
- **Resident panels drop their off-screen list delegates.** The Arch updater and
  notification panels are always-resident `PanelWindow`s whose lists are fed by
  background polls (the update badge, the notification badge), so their `Repeater`s
  held a QML delegate for every package and notification even while the panel was
  closed. Each list's model now yields its rows only while the panel is open
  (gated on `panel.visible`, which spans the close animation, so the badges and the
  close transition are unaffected), freeing the row delegates the rest of the time
  (`qsbar/panels/{ArchUpdaterPanel,NotificationPanel}.qml` and their V2 variants).
- **Power Saver freezes the bar stream animation.** The bar-gap particle stream
  (`barAnim` 1-6, and 1/stream is the default) drove a threaded `Canvas` at
  30-60fps whenever the gap was on screen, with a continuous cava (PipeWire FFT)
  claim, and never backed off, its own code noting the tick machinery costs
  ~23% CPU. It now reads `Perf`: reduce-motion (its toggle, lowPowerMode, or the
  Power Saver profile) freezes the canvas to its last frame and releases cava, and
  on battery the repaint rate halves via `Perf.pollFactor`. The default look on AC
  is unchanged (`modules/bar/barstyles/qsbar/modules/ParticleStream.qml`).
- The V2 bar's AI-usage poll now backs off on battery and Power Saver like V1: its
  interval reads `Perf.pollFactor`, so the 30s subprocess sampler (claude/codex/
  opencode usage) halves while discharging. Completes poll-backoff parity across
  both bar variants (`variants/V2/Theme.qml`).
- **Power Saver pauses the video wallpaper.** The live-wallpaper gate stops the
  video decode while the Power Saver profile is active (with Follow the power
  profile on), not just under a fullscreen window. The still frame stays on
  screen, so the desktop looks the same while the decode drain is gone; the
  fullscreen/saver decision is a pure, unit-tested `liveShouldStop`
  (`ipc/livewatch.go` + test).
- **Auto power saver on battery.** A new opt-in (Hub -> System -> Performance)
  has the ryoku-shell daemon switch to the Power Saver profile when you unplug and
  restore your profile when you plug back in, through power-profiles-daemon. It
  acts once per battery episode, so a manual profile change while unplugged is left
  alone, and a machine with no power-profiles-daemon or no battery is unaffected.
  The switch rides the normal PowerProfiles stream, so Follow the power profile
  then lightens the shell too. The AC/battery transition logic is a pure,
  unit-tested step (`ipc/autoprofile.go` + test, `ipc/daemon.go`,
  `ipc/powerprofiles.go`, Hub `pages/PerformancePage.qml`).
- **Power Saver reduces motion in the launcher, overview and wallpaper too.**
  Their `Motion` singletons take `reduce` from `shell.services.Perf` now (like the
  shell's shared Motion), so the power profile collapses their transitions to an
  instant cut and each drops its own performance.json watcher
  (`modules/{launcher/shared,overview,wallpaper}/Singletons/Motion.qml`).
- **Power Saver strips compositor blur and shadow too.** The power-profile policy
  now reaches the heaviest present-time GPU cost. When Power Saver is active (and
  "Follow the power profile" is on) the shell writes a flag to ~/.cache/ryoku and
  reloads Hyprland, and a new `hyprland/modules/perf_saver.lua`, loaded after the
  Hub's settings.lua so the profile wins over its decoration tweaks (and before
  user.lua so a hand-written file still wins), drops `decoration.blur` and
  `decoration.shadow`. Balanced and Performance restore them
  (`shell/quickshell/shell/shell.qml`, `hyprland/modules/perf_saver.lua`,
  `hyprland/hyprland.lua`).
- **Power profiles drive the shell, and it eases off on battery.** A new
  `shell.services.Perf` singleton is the one reader of `performance.json`; it
  folds the file with the active power profile and the battery state, so every
  surface reads one set of effective switches instead of watching the file itself.
  With the new **Follow the power profile** switch (Hub -> System -> Performance,
  on by default) Power Saver reduces motion, drops blur and shadows and halves
  vector-layer antialiasing (`layer.samples` 8 -> 2), the same trade the Low power
  mode master makes; Balanced and Performance leave your explicit switches
  untouched, so the default profile never overrides a choice. Independently, while
  the laptop is discharging every background stat poller (CPU, memory, GPU,
  thermals, storage, AI usage, voice, the 45 ms mic meter) samples at half rate,
  and the qsbar GPU poll now reads the NVIDIA dGPU `runtime_status` first and skips
  `nvidia-smi` entirely while the card is runtime-suspended, so a bar GPU widget no
  longer wakes the discrete GPU and burns ~10 W at idle. `Motion` and the desktop,
  launcher and visualiser `Performance` singletons now forward to `Perf`
  (`services/Perf.qml`, `services/Motion.qml`, `services/qmldir`,
  `modules/{desktop,launcher,visualizer}/Singletons/Performance.qml`,
  `modules/bar/barstyles/qsbar/{Theme,panels/CpuPanel}.qml` and its
  `variants/V2/{Theme,panels/VolumePanel,modules/AiBarInset,modules/AiPanelSurface,modules/WorkspaceWidget}.qml`,
  Hub `pages/PerformancePage.qml`).
- **A system font you can change without a logout.** The interface font is a
  live setting now (Hub -> Global -> System font). The shell reads it through
  `Theme.fontPrimary`, and the daemon mirrors it to the toolkits: GTK via
  `gsettings font-name` (running apps re-read at once) and Qt via the qt6ct
  general font (picked up on next launch), applied on the change and again on
  startup. Empty keeps the shipped Space Grotesk; the old hardcoded autostart
  font line is gone, so the daemon owns it (`services/Config.qml`,
  `services/Theme.qml`, `ipc/matugen.go`, `ipc/settings.go`,
  `hyprland/modules/autostart.lua`).
- **Regional formats, separate from the UI language.** A new Region control in
  the Hub (Global) sets a formats locale that the shell uses for
  dates, weekday/month names and the calendar, while the interface language is
  left untouched -- so a Brazilian (or any) desktop can run an English UI and
  still read `dom. 9`, `dd/mm/yyyy` and localized month names. It is a plain
  `formatLocale` passthrough key in `shell.json`; empty follows the system
  locale. The bar clock and its date, the desktop clock faces, the calendar, and
  the quick-settings clock all honour it live (they previously hardcoded English
  names / US date order). Shared date formatting moved into a locale-aware
  `dateParts(date, loc)` with a unit test (`services/Config.qml`,
  `modules/desktop/clock/lib/clock.js` + `clock.test.mjs`,
  `modules/desktop/clock/Date*.qml`, `modules/desktop/calendar/Calendar*.qml`,
  `modules/bar/barstyles/qsbar/**/{BarSlot,modules/ClockWidget}.qml`,
  `modules/bar/framebars/menus/MenuClockCard.qml`; Hub `Hub.qml` +
  `schema/DesktopPage.js`).
- **A chat with the Rashin agent lives in the Super+S sidebar.** A new Chat tab
  (the default) in the feature sidebar holds a multi-turn conversation with the
  `ryoku-rashin` agent: the answer streams in as it is written and renders as
  Markdown, selectable and copyable, with fenced code in a wrapped,
  syntax-highlighted box that has a one-click copy. Attach images with the paperclip, by pasting
  (Ctrl+V), or by dropping them; attached and produced images preview inline.
  The input grows with the text and sends on Enter, Shift+Enter for a newline. A new
  `ryoku-rashin chat` command bridges the daemon's shared hermes session over a
  line protocol, so follow-ups keep context and New Chat (`chat --new`) forgets
  it. The conversation and any in-flight answer live in a `Needle` singleton, so
  closing and reopening the sidebar keeps the thread; a fresh chat starts only
  after the sidebar has been away ten minutes (`modules/bar/panel/PanelChat.qml`,
  `services/Needle.qml`, `rashin/backend/chatcli.go`).
- **The chat survives a shell reload.** On load the sidebar restores the
  conversation the persistent daemon session still holds (text), so a
  `ryoku reload` or relogin lands back on the thread instead of an empty chat.
  A new `ryoku-rashin chat --history` replays the session transcript
  (`rashin/backend/chatcli.go`, `services/Needle.qml`).
- **Browse and resume past conversations from the sidebar.** The chat header
  gained a history button: it opens a drawer listing every stored session
  (newest first, by title) plus New Chat. Picking one loads it into the
  thread, matching the dashboard's session drawer. New
  `ryoku-rashin chat --sessions` and `chat --load <id>` back it, and stored
  titles that captured the identity preamble are cleaned
  (`rashin/backend/chatcli.go`, `rashin/backend/acp.go`, `services/Needle.qml`,
  `modules/bar/panel/PanelChat.qml`).
- **The chat answers as the Needle, not generic hermes.** The daemon rides a
  one-time identity preamble in front of a session's first turn, so the sidebar
  assistant introduces itself as the Needle, Ryoku's resident assistant, and
  stays Ryoku-aware. It is chat-scoped and invisible to the transcript, so
  Claude Code and other agents reading the shared vault are unaffected
  (`rashin/backend/ws.go`).
- **The chat shows the agent working.** While the needle answers, a live
  activity log streams above the text: every tool the agent runs (read, search,
  edit, run a command, ...) as an icon and title with a status that flips to a
  check when it finishes, and the agent's reasoning as it thinks. Each finished
  answer also carries a RETRY (re-ask in place) and COPY row
  (`rashin/backend/chatcli.go`, `services/Needle.qml`,
  `modules/bar/panel/PanelChat.qml`).
- **A slash-command and skills palette in the chat.** Type `/` and the
  session's commands float above the input (`/tools`, `/context`, `/compress`,
  `/steer`, `/model`, `/reset`, ...); type more and every installed hermes
  skill (`/humanizer`, `/architecture-diagram`, `/systematic-debugging`, ...)
  joins the list, each invokable as a slash command. Arrows move (the list
  scrolls), Tab or Enter
  completes, Esc dismisses, and Enter runs it in the session. New
  `ryoku-rashin chat --commands` and `--skills` fetch the lists
  (`rashin/backend/chatcli.go`, `modules/bar/panel/PanelChat.qml`).
- **The needle can query the Ryoku code index mid-chat.** When prowl-agent and
  an indexed checkout are both present, the hermes session starts with
  prowl-agent wired in as an MCP server, so the agent answers "where is X
  defined", "what calls Y", or "what is the blast radius of Z" with cited
  results from the index instead of guessing. Absent prowl, the session opens
  unchanged (`rashin/backend/acp.go`, `rashin/backend/prowl.go`).
- **Pick the model from the sidebar.** A chip in the chat header shows the
  current model; opening it lists every model the hermes session offers (an
  OpenAI model, a local endpoint, or whatever the provider config exposes) and
  switching is one click. So the chat can talk to rashin's hermes, OpenAI, or a
  local model without leaving the sidebar. The pick is remembered: the daemon
  saves it and re-applies it to every new session, so a restart keeps your
  model and the dashboard, sidebar, terminal fast lane, and `status` all use
  the model the session is actually on (`chat --models`, `chat --set-model`;
  `rashin/backend/acp.go`, `rashin/backend/ws.go`, `rashin/backend/hermes.go`,
  `rashin/backend/quick.go`,
  `modules/bar/panel/PanelChat.qml`, `services/Needle.qml`).

### Fixed
- **The all-in-one desktop widget follows the theme and trims its wide divider.**
  The tall AIO face drew its day number, month, clock, weekday, spectrum and
  stars in fixed blue-greys, so it never tracked the active scheme or the
  wallpaper. Its colours now resolve through the desktop palette
  (`Theme.accentOn` for the day number and spectrum, `Theme.inkOn` and
  `inkDimOn` for the text), so the face retints live with the wallpaper and with
  every named scheme. The wide face's diagonal divider ran from y20 down to y330
  and cut through the big weekday; it now ends at the weekday's top edge
  (`modules/desktop/aio/AioWidget.qml`).
- **A ryoshot launch that cannot draw no longer eats the screenshot key.** If the
  compositor or driver refuses a graphics context for the layer surface, Qt logs
  it but QML never hears, so the process sat in its event loop forever holding
  `/tmp/ryoshot.lock` and every later Print press was a silent no-op. A frame
  watchdog now quits the launch when nothing has painted 15s in, releasing the
  lock, and says so in a notification. `ryopin` carries the same watchdog
  (`quickshell/ryoshot/shell.qml`, `quickshell/ryopin/shell.qml`).
- **A monitor that never yields a screencopy frame falls back to grim.** The
  frozen-frame poll gave up in silence after three seconds and left a surface
  that was alive but blank. It now captures that output with `grim` and carries
  on with the rest of the flow. `RYOSHOT_FORCE_FALLBACK=1` exercises the path
  (`quickshell/ryoshot/Overlay.qml`).
- **Blur, pixelate and zoom no longer freeze on the pixels they were drawn over.**
  Their samplers grab once, and moving or resizing the annotation never asked for
  a fresh grab, so the effect kept showing the region it was first placed on. The
  sampled rect now reschedules its grab when it moves
  (`quickshell/ryoshot/EffectLayer.qml`).
- **ryoshot follows the desktop palette.** Every colour, font and duration in the
  surface, the annotation chrome and the Beautify editor came from literals, so
  the one surface on the desktop that did not retint with the wallpaper was the
  one users look at most closely. They all resolve from
  `Ryoku.Ui.Singletons.Tokens` now through a local `Theme` singleton; the only
  literals left are the fixed annotation inks, which must stay recognisable
  whatever the palette is, and the solid redaction block, which must not shift
  after the user has checked it (`quickshell/ryoshot/Singletons/Theme.qml`).
- **The islands bar honours the corner and gap-animation settings.** Merging the
  two variants left the split-pill `"islands"` form reading a fixed
  `islandRadius`, so the control center and Bar Studio "Corners" control (which
  writes `barCornerRadius`) never reshaped the pills, and the reactor gap-stream
  measured its channels from the widget-row edges instead of the pills, so the
  stream drew over each pill's rounded corners. The island pill radius now
  follows `barCornerRadius` (square, soft or round, applied live from both
  surfaces), and the reactor insets its gap channels by the pill padding so the
  stream stays strictly in the desktop gaps between pills for every animation
  mode (`modules/bar/barstyles/qsbar/BarSlot.qml`, `Theme.qml`,
  `modules/bar/barstyles/qsbar/modules/ReactorLayer.qml`).
- **The sidebar chat no longer goes dark when KSyntaxHighlighting is absent.**
  The code-block highlighter imported `org.kde.syntaxhighlighting` at the top
  of the chat panel, so a machine without the `syntax-highlighting` module
  failed the whole panel and the chat showed nothing. The highlighter now
  loads through a `Loader` (`components/CodeHighlight.qml`); a missing module
  degrades to plain, readable code blocks instead of a dead chat. The fix
  reaches existing installs the moment `materialize` lays the updated QML, with
  no package dependency required (`modules/bar/panel/PanelChat.qml`).
- **Chat answers keep their line breaks.** Markdown collapses single newlines,
  so a line-structured reply (a `/tools` list, terse notes) rendered as one
  run-on paragraph. Both chat surfaces now preserve non-blank newlines as hard
  breaks, so structured output reads line by line; blank-line paragraphs and
  fenced code are untouched, and vault docs stay soft-wrapped
  (`modules/bar/panel/PanelChat.qml`, `rashin/backend/web/js/markdown.js`,
  `rashin/backend/web/js/chat.js`).
- **Sidebar downloads that share a title no longer vanish after "done".** With
  no cobalt instance reachable, the yt-dlp fallback downloaded straight into the
  stash under `%(title)s.%(ext)s`; distinct videos that generate the same title
  (for example several captionless posts by one author, all named
  `Video by <author>`) collided, so yt-dlp skipped every one after the first as
  "already downloaded" and exited 0. The queue showed "done" while nothing new
  landed. The fallback now downloads into a private temp dir and moves each
  result into the stash through the same `dest_for` disambiguator the cobalt path
  uses, so a collision lands as "name (1)" instead of being dropped; its stdin is
  tied to `/dev/null` so an ffmpeg merge can never stall on the inherited pipe
  (`hyprland/scripts/stash-cobalt.sh`).
- **The Tools download queue detects a no-op and offers a retry.** A row only
  reads "done" when the worker actually emits a saved file; a run that exits
  cleanly without saving anything (a skipped duplicate, a picker that fetched
  nothing) now shows a red failure with the reason instead of a false success.
  Finished rows carry a dismiss (x); failed rows also carry a retry that re-runs
  the same link in place, and the progress bar hides once a row settles
  (`services/Stash.qml`, `modules/bar/panel/PanelTools.qml`).
- **The dashboard chat recovers from a dead session instead of locking up.**
  If the hermes session dies, the composer stays usable and the banner reads
  "offline, send to reconnect"; the next message respawns the session. The
  daemon now waits for a respawned session to finish initializing before it
  sends the prompt, so the first message after a respawn is not lost (a lost
  prompt failed on the empty session and killed it, dead-ending the chat)
  (`rashin/backend/acp.go`, `rashin/backend/web/js/chat.js`).
- **Chat image upload no longer fails on all but the tiniest images.** The chat
  WebSocket kept coder/websocket's 32 KiB default read limit, so a base64 image
  overran it and the send was dropped. The daemon now raises the limit, and the
  `chat` client downscales through ImageMagick before sending
  (`rashin/backend/server.go`, `rashin/backend/chatcli.go`).
- **A corrupt shell.json no longer wipes qsbar and the other look knobs.** The
  daemon reduced an existing-but-unparseable shell.json to defaults with no
  passthrough keys, then persisted that over the file on the next patch, so a
  transient or torn read (e.g. around an update) could permanently drop the bar
  style the user had. It now retries the read and backs the file up to
  `shell.json.corrupt` before falling back to defaults (`ipc/settings.go`).
- **qsbar Bar Studio settings survive a shell reload.** A control-center change
  wrote the local widget cache, but `applyStudioSettings` re-applies `Config.qsbar`
  over the live state on every reload and every qsbar change, while the persist
  step mirrored only `widgets` back to shell.json. So any other studio setting
  (bar accent, position, workspace and picker mode, launcher logo, AI tool, shell
  style, borders, gap animation) reset to its stale stored value on the next
  refresh. The persist now writes every key `applyStudioSettings` applies, from
  the live properties, so shell.json always matches the live bar and re-applying
  it is a no-op (`modules/bar/barstyles/qsbar/**/Theme.qml`).
- **The music widget shows the Spotify Canvas by default.** The desktop music
  widget's backdrop now defaults to the track's Spotify Canvas (was off), so the
  looping art plays automatically when Spotify is up and a Canvas exists and falls
  back to the album cover otherwise (`modules/desktop/Singletons/Config.qml`,
  `modules/desktop/music/MusicWidget.qml`).
- **The Bluetooth panel shows device details.** A connected device now has an
  info toggle that expands its battery, type, and address, read live from
  `Quickshell.Bluetooth` (`modules/bar/barstyles/qsbar/**/BluetoothPanel.qml`).
- **Changing the DNS provider works from a dev checkout.** The daemon prefers the
  installed `/usr/bin/ryoku-dns`, the polkit rule matches any `ryoku-dns` path,
  and `deploy.sh` installs the helper + rule so escalation works without a
  package build (`ipc/network.go`, `deploy.sh`).
- **The volume panel can select an input (microphone) device.** The qsbar volume
  panel listed output devices but the input section was a mute toggle only, so a
  mic or capture device could not be picked; it now shows an input-device switcher
  off the native `Audio.inputs`, mirroring the output one
  (`modules/bar/barstyles/qsbar/panels/VolumePanel.qml` and the V2 counterpart).
- **The "Open audio" button says when pavucontrol is missing.** It launched
  `pavucontrol` blind, so on a box without it the button did nothing; it now
  notifies instead (`.../qsbar/panels/VolumePanel.qml` and the V2 counterpart).
- **The Control Center's Quick page no longer leaves a tall empty area.** The card
  was locked to the taller Configure page's height; it now fits the active page's
  content (`.../qsbar/controlcenter/ControlCenter.qml`,
  `.../controlcenter/routes/QuickPage.qml`).
- **The system tray no longer shows closed or mislabelled apps.** The qsbar tray
  strip, panel and context menu read Quickshell's own StatusNotifier host, which
  kept dead items and wrong titles on this machine; they now read the ryoku-shell
  daemon's `Tray` service (the documented single host, already used by the frame
  bar), so closed apps drop promptly and icons and titles are correct. The
  context menu renders the daemon's dbusmenu tree in-shell with qsbar theming
  (`modules/bar/barstyles/qsbar/{modules/TrayWidget,panels/TrayPanel,panels/TrayMenu}.qml`,
  `.../qsbar/Theme.qml`, and the V2 counterparts).
- **The bar accent follows the wallpaper without a reload.** `colors.json` is
  rewritten on every palette change, but the qsbar palette was only re-read on a
  theme-name change or an explicit reload, so a follow-wallpaper swap left the
  accent stale until the whole shell reloaded. A watch on `colors.json` retints
  live (`modules/bar/barstyles/qsbar/Theme.qml` and the V2 counterpart).
- **Frame menus open against a bottom bar instead of clipping into it.** The
  shared frame menu host (Super+Escape quick settings and the rest) always
  dropped from the top edge, so on a bottom qsbar the menu ran down into the bar.
  It now folds to the bar's actual edge (`modules/bar/Frame.qml`,
  `modules/bar/FrameMenuManager.qml`).
- **The launcher pill's hover wave no longer hard-clips.** The wave was stroked
  edge to edge inside a rectangular clip, leaving a hard vertical seam; it now
  fades at both ends (`modules/bar/barstyles/qsbar/modules/LauncherWidget.qml`
  and the V2 counterpart).
- **The bar's audio controls are one native PipeWire path, not three racing
  tools.** The volume pill's mute and wheel, and the whole volume panel (output
  device switch, per-app mixer, microphone), now write straight to PipeWire
  through the shell's `Audio` singleton (`Quickshell.Services.Pipewire`) instead
  of a mix of `wpctl`, `pactl` and `pamixer`. The old split read the volume
  natively but wrote through those tools, and switched the default sink with a
  `wpctl set-default` plus `pactl set-default-sink` plus a manual `move-sink-input`
  loop that fought WirePlumber, so the readout and the hardware could disagree and
  switching outputs was flaky. One event-driven source of truth now
  (`modules/bar/barstyles/qsbar/modules/AudioWidget.qml`,
  `.../qsbar/panels/VolumePanel.qml`, and the V2 counterparts, on the shipped
  `services/Audio.qml`).
- **The volume panel's "Open audio" button works.** It launches `pavucontrol`,
  which Ryoku never shipped, so it silently did nothing; `pavucontrol` is a hard
  dependency now (see the release changelog).
- **The follow-the-wallpaper default now reaches every surface, not just the
  frame and bar.** The shipped colour default is the wallpaper, but five
  per-surface colour singletons still defaulted to the static brand palette when
  `theme.json` was absent, so a fresh box's desktop widgets, plugin tiles, audio
  visualiser and wallpaper switcher stayed monochrome while the frame and bar
  retinted. They now default on when the key is absent, matching
  `services/Config.qml` and the daemon, so an uncustomised box follows the
  wallpaper everywhere while an explicit Mono/Light/Dark pick still locks it off
  (`ui/Singletons/Tokens.qml`, `modules/desktop/Singletons/Scheme.qml`,
  `plugins/kit/Singletons/Scheme.qml`,
  `modules/visualizer/Singletons/Scheme.qml`, and
  `modules/wallpaper/switcher/Singletons/Config.qml`).
- **Bluetooth is reachable from the default bar.** The Bluetooth pill defaulted
  hidden (`modBluetooth: false`) and only ever rendered when that toggle was on,
  so a fresh install -- or any user whose widget cache had persisted it off -- had
  no entry point to the Bluetooth panel from the default (V1) bar and couldn't
  turn Bluetooth on, scan, or connect. The pill now auto-shows whenever a
  controller is present (mirroring how the network pill stays up on Wi-Fi),
  detected by extending the widget's `bluetoothctl show` poll to report adapter
  presence (NONE/OFF/ON); machines with no adapter keep a clean bar, and the
  poller keeps running while hidden so a dongle added later still surfaces the
  pill. The panel it opens (power toggle, timed scan, connect/disconnect) already
  works and is identical across V1/V2
  (`modules/bar/barstyles/qsbar/modules/BluetoothWidget.qml` and the V2
  counterpart under `.../qsbar/variants/V2/`).
- **Power-mode switching offers only the profiles the hardware supports.** The
  qsbar power panels hardcoded `power-saver`/`balanced`/`performance` and ran
  `powerprofilesctl set` blindly, so on hardware missing a profile the button did
  nothing and the widget snapped back to the active one (users saw "Balanced works,
  the others don't"). Both Theme roots now parse `powerprofilesctl list` into
  `powerProfileAvailable`; the panel renders, and the widget's right-click cycle
  steps through, only that set -- falling back to the standard three if the list
  can't be read, so nothing regresses (`modules/bar/barstyles/qsbar/Theme.qml`,
  `.../qsbar/panels/PowerProfilePanel.qml`,
  `.../qsbar/modules/PowerProfileWidget.qml`, and the V2 counterparts under
  `.../qsbar/variants/V2/`).

### Changed
- **Colours follow the wallpaper by default.** The shipped default colour source
  is the wallpaper now, not the static Mono palette: `services/Config.qml`
  (`followWallpaper`) and the daemon's `matchWallpaperOn` (`ipc/matugen.go`)
  default on when no `theme.json` is present, so a fresh install and any user who
  never locked a scheme get wallpaper-driven colours (Mono, Light and Dark stay a
  one-click pick). This matches Ryoku's wallpaper-driven identity.
- **The first-run welcome sets you up in a few taps.** The tour's "make it
  yours" step now wires the essentials it used to only point at: an interface
  scale slider (the fix for a first login that reads oversized), a bar picker
  (QS Bar or the Sumi rail), and per-widget toggles for the desktop clock,
  calendar, and music player, each writing the same `shell.json`/`widgets.json`
  the shell already reads live. The keybind cheat-sheet is corrected to the
  current chords, and the tour shows once more for anyone updating into beta 18
  (`quickshell/welcome/StepCustomize.qml`, `StepBasics.qml`, `Welcome.qml`, and
  the version gate in `hyprland/modules/autostart.lua`).
- **QS Bar gains a full Control Center.** The bar's compact 240px control card is
  replaced by a route-based Control Center, opened from the 力/RYOKU launcher: a
  QUICK page (bar variant/form cards, reload, and session actions) and a
  CONFIGURE page whose bezier connection-graph landing opens live editors for
  Bars (position, form Full/Fit/Dock/Notch, surface, accent, edit/restore
  layout), Appearance (per-widget visibility, colour, density), Logo,
  Workspaces, and Pickers, with CTRL-K predictive search and staged page motion.
  The V2 shell now runs the reactor/gap event layer that used to be V1-only, and
  every control mirrors into the Hub's Bar Studio through the `qsbar` shell.json
  map. Ported from Shibumi-Shell's control-center and interaction model; it wears
  QS Bar's own dark theme, not the paper-ink shell tokens
  (`modules/bar/barstyles/qsbar/controlcenter/`,
  `.../qsbar/variants/V2/modules/ReactorLayer.qml`,
  `hub/quickshell/pages/BarStudioPage.qml`).
- **Super+S opens a feature sidebar: screen time and downloads.** The old file
  stash (the board, LocalSend send and receive, the drag-to-edge stash) is
  retired, and the sidebar moved off Super+T (now free) onto Super+S. Usage is a
  local screen-time overview built from a tracker that samples the focused app
  every few seconds and keeps the day history under ~/.local/state/ryoku
  (nothing leaves the box): today's active total, a seven-day trend, and the
  apps used most. Tools is a downloader: paste a link and cobalt fetches it (now
  handling cobalt's client-side local-processing and showing the instance's live
  list of supported sites), recent downloads sit below newest-first, and
  Convert/Install compress media or install a package
  (`quickshell/shell/modules/bar/panel/`, `services/ScreenTime.qml`,
  `services/Stash.qml`, `hyprland/scripts/stash-cobalt.sh`).
- **Compress video and Install app open an in-shell file picker.** Picking files
  for the Tools tab's Compress and Install (and their searchable launcher
  entries) no longer spawns an external zenity dialog that closed the sidebar.
  A paper-and-ink file browser opens inside the sidebar itself: navigate folders,
  multi-select, and run, with the sidebar never closing. The launcher entries
  deep-link straight to it (`stash#compress`, `stash#install`), the picker hands
  the chosen paths to the ffmpeg compressor and the package installer, and the
  zenity dependency plus the scripts' `--pick` mode are retired
  (`quickshell/shell/modules/bar/panel/PanelPicker.qml`, `panel/Panel.qml`,
  `panel/PanelTools.qml`, `services/Stash.qml`,
  `apps/tools/compress-video.desktop`, `apps/tools/install-app.desktop`).
- **The desktop shell now runs as a single Quickshell instance.** The frame bar
  and its menus, the launcher, overview, board, wallpaper, visualiser, on-screen
  displays, notifications, capture, and the desktop widgets were seven separate
  `qs -c <config>` processes, each spawned on its keybind (~130 ms to launch and
  a +130-470 MB spike per open, climbing toward 1-2 GB as surfaces piled up).
  They now live in one `qs -c shell` instance under `quickshell/shell/`
  (`modules/` one directory per surface, `services/` the shared singletons,
  `components/` the shared primitives), driven by the `ryoku-shell` daemon
  through in-process Hyprland global shortcuts. Opening a surface is now a
  property flip (~2 ms, no spawn) and resident RAM holds a flat ~705 MB ceiling
  instead of a 614 MB rest that spiked toward 1-2 GB. The old per-surface
  configs (`pill` and the standalone surface trees) are retired.
- **Barstyle products load through a stable SDK API.** The consolidation retired
  the `pill.*` module namespace that third-party bar styles imported, so nacre
  and obi failed to load and fell back to the built-in sumi. Products now import
  `shell.services` (the shell's live singletons -- one shared notification
  server, never a second) and `shell.barkit` (the non-singleton primitives: icon
  and brand types, MusicBars, TrayMenu, NotificationCard, the Popout bases, and
  the audio and notification menus), both resolved through the Quickshell import
  path. See `docs/barstyles.md`; the ryoku-extras catalogue is migrated to it.
- **Super+S opens capture as a quick-settings tab.** The screenshot and screen
  record card that Super+S raised as a floating popup now lives as a Capture tab
  in the Super+Esc quick settings rail, right after Weather, so it reads as part
  of the same instrument rather than a separate surface. Super+S deep-links to
  that tab (`quick-settings#capture`) and the card content is reused in place. The
  tab carries a Recent section: the latest screenshots and recordings in two
  labelled groups (recordings as poster frames with a play badge and duration),
  each opening its folder in the file manager on click; the section fills in just
  after the open so the sidebar never hitches. `ryoku doctor` adds the tab to a
  rail still carrying the pre-capture default
  (`quickshell/shell/modules/bar/framebars/menus/quicksettings/QuickSettingsCapture.qml`).

### Fixed
- **The bar no longer resets to the sumi rail after an update.** `Frame` loads
  the active bar style through a `Loader`, and a single `Loader.Error` called
  `BarProducts.fail(id)`, which recorded the style as failed for the rest of the
  session -- so any transient load hiccup on a fresh post-update start (a plugin
  `.so` swap, a cold-start import race) permanently dropped the configured qsbar
  style to the built-in sumi rail until a full clean restart. A built-in style
  ships with the shell and cannot be legitimately broken, so `fail()` now ignores
  built-ins and `Frame` retries the loader (up to ~8s) before degrading; a
  genuinely broken store-installed style still falls back as before
  (`quickshell/shell/services/BarProducts.qml`, `quickshell/shell/modules/bar/Frame.qml`).
- **The updates widget detects updates again.** The ported qsbar updater shelled
  out to upstream quickshell-dots scripts (`~/.local/bin/qs-arch-{update-check,
  security-gate,apply-update}.sh`) plus bare `checkupdates`, none of which Ryoku
  ships -- so it always read zero. Rewired the bar's `ArchUpdaterWidget` and the
  `UpdateWidget` dot to Ryoku's own seam, `ryoku status --json` (the same source
  the Hub and update island use: channel commits behind plus, when
  `checkupdates` is present, pending pacman packages). The per-package security
  gate (an upstream concept Ryoku has no script for) is retired -- applying is
  owned by the panel's Ryoku-updater backend (`ryoku update`), which the panel
  already prefers -- so the advisory gate is held benignly clean instead of
  erroring on a missing script
  (`quickshell/shell/modules/bar/barstyles/qsbar/modules/{ArchUpdaterWidget,UpdateWidget}.qml`,
  the V2 copies, and the `archGate` in both `Theme.qml`).
- **The updater panel's "Open Ryoku update" button works.** Two faults left it
  inert: the panel only appended the `update` subcommand for the `cli` backend
  mode, so the Ryoku-updater (`update` mode) launched the bare `ryoku` binary
  (usage, no-op); and the panel's terminal adapter (`qs-system-update.sh`)
  `%q`-joined the command into a single string, so kitty tried to exec a program
  literally named `ryoku update` and failed. The panel now appends `update` for
  either Ryoku-updater mode, and the adapter passes the command as separate args
  with `--hold`, so the button opens a terminal running `ryoku update`
  (`quickshell/shell/modules/bar/barstyles/qsbar/panels/ArchUpdaterPanel.qml` +
  the V2 copy, `.../qsbar/core/qs-system-update.sh`).
- **The AI-usage pill can show data again.** The pill reads
  `~/.cache/{claude,codex,opencode}-usage.json`, but the collectors that write
  them were never ported, so the caches never existed and the pill (which hides
  without a signal) stayed invisible. Ported the three collectors to
  `ryoku/shell/bin/` and added a `ryoku-ai-usage` systemd user timer (10-minute
  cadence) that refreshes the caches; each collector no-ops cleanly when its
  tool is absent. Shipped by both the dev deploy and the `ryoku-desktop` package
  (`ryoku/shell/bin/{claude,codex,opencode}-usage`,
  `ryoku/shell/systemd/user/ryoku-ai-usage.{service,timer}`). The pill itself
  stays off by default; enable it in Appearance / Bar Studio.
- **QS Bar settings no longer leak across variants.** The Control Center's Bars
  route and the Hub's Bar Studio showed V2-only controls (bar form
  full/fit/dock/notch, the separate bar and panel/tooltip borders) while V1 was
  active and never surfaced V1's own island style (border, frost, shadow), and
  the workspace-marker picker offered only V1's three styles under both. Each
  surface now gates on the active variant: V1 shows its STYLE section and the
  full gap-animation set (including the `·2` presets), V2 shows the shell forms
  and two-border surface, and the marker picker is driven by a per-variant
  `workspaceStyleOptions` list (V2 adds Kanji, Frame, Aurora). Bar Studio hides
  the V2-only rows and matches the marker set under V1
  (`quickshell/shell/modules/bar/barstyles/qsbar/controlcenter/routes/BarsRoute.qml`,
  `.../routes/WorkspacesRoute.qml`, `.../qsbar/Theme.qml`,
  `.../qsbar/variants/V2/Theme.qml`, `hub/quickshell/pages/BarStudioPage.qml`).
- **The AI-usage tool is now a first-class setting.** Choosing which coding-agent
  meter the bar pill shows (Claude, Codex, or OpenCode) was only reachable from
  inside the AI-usage popout, so the option never appeared in either settings
  surface. The Control Center's Appearance route grows an AI USAGE selector
  (shown while the widget is enabled) and the Hub's Bar Studio grows an AI tool
  row; both drive `aiTool` on either variant, and the Bar Studio bridge
  (`applyStudioSettings`) now applies the `qsbar.aiTool` key it writes
  (`quickshell/shell/modules/bar/barstyles/qsbar/controlcenter/routes/AppearanceRoute.qml`,
  `.../qsbar/Theme.qml`, `.../qsbar/variants/V2/Theme.qml`,
  `hub/quickshell/pages/BarStudioPage.qml`).
- **V2's GPU, CPU-temperature and storage widgets are now configurable.** These
  three continuous-bar telemetry widgets (and the temperature widget's sensor
  source) existed on the bar but appeared in no settings surface -- neither the
  Control Center's Appearance list nor the Hub's Bar Studio widget list, and the
  Bar Studio bridge dropped them on the floor. The Appearance route gains GPU,
  CPU Temp and Storage rows (visibility, accent, icon-only density) plus a
  TEMPERATURE selector, Bar Studio gains the three widget toggles and a
  temperature-source row (all gated to V2, which alone has these widgets), and
  `applyStudioSettings` now applies `qsbar.barTemperatureSource` and the three
  `qsbar.widgets` keys
  (`quickshell/shell/modules/bar/barstyles/qsbar/controlcenter/routes/AppearanceRoute.qml`,
  `.../qsbar/variants/V2/Theme.qml`, `hub/quickshell/pages/BarStudioPage.qml`).
- **The desktop no longer boots into a keyboard-grabbed, greyed-out state.** The
  photo-viewer overlay's `shown` guard tested `src !== ""`, but `src` is a `url`
  property and strict inequality against a string is always true for an empty
  url, so the viewer counted as open from the first frame: it dimmed the desktop
  with its scrim and bumped `kbWanted`, handing the wallpaper layer an exclusive
  keyboard grab that left every window unfocusable (no typing, no clicks, only
  spawn and global-shortcut keybinds firing). It now tests the url's length,
  matching `open()`, so an empty source reads as closed
  (`quickshell/shell/modules/desktop/Desktop.qml`).
- **Desktop-widget plugins render again.** The desktop host exposed each
  plugin's service as `pluginApi.mainInstance = svc.item`, but the service
  loader carried no `id: svc`, so that reference was undefined and every desktop
  widget saw a null service. Photo Frame fell back to its defaults (a
  transparent "rounded" mat and no image) and drew as a blank grey box; other
  widgets lost their settings the same way. The loader now carries `id: svc`,
  matching the popout host, so the service reaches the content and each widget
  paints from its own settings
  (`quickshell/shell/modules/desktop/Desktop.qml`).
- **The Mono colour scheme no longer follows the wallpaper.** Picking Mono
  (`theme.theme` "Default", the shipped default) wrote `followWallpaper: true`
  into `theme.json`: the shadow key tracked "no fixed palette selected" and so
  lumped Default in with the live Wallpaper variant. Mono then rendered the
  wallpaper's colours, indistinguishable from Wallpaper, so selecting it in
  Appearance did nothing visible. Only the Wallpaper variant follows now;
  Default (and every named theme) turns the key off, so Mono renders the shell's
  compiled monochrome base. Existing installs self-heal on the next daemon
  restart (`ipc/matugen.go`).
- **The quick-settings sidebar no longer ghosts when a page opens.** The device
  pages (clipboard, Wi-Fi, Bluetooth, ...) slid in over the module band with no
  opaque backing, so the modules behind bled through and looked like they cut
  the page off; opening a page directly (Super+V for the clipboard) also
  animated the bands from the home layout, flashing the home dashboard. The page
  band now carries the same surface backing the home and media modules have, and
  the slide only animates once the user navigates in-panel, so a direct open
  snaps straight to its page (`quickshell/shell/modules/bar/framebars/menus/MenuQuickSettings.qml`).
- **Stray clipboard watchers no longer accumulate across logins.** wl-paste's
  Pdeathsig ties a watcher to the daemon that started it, but a force-killed
  daemon or a logout could strand watchers that reparented to init and held a
  data-control slot for the rest of the session; they built up run over run
  (seen: ten watchers plus ten ingest helpers, ~157 MB). The daemon now reaps
  any stray watcher carrying its own `__clip-ingest` marker on start before
  attaching a fresh one, and each ingest helper caps its own lifetime
  (`ipc/clipboard.go`).
- **Ryoport and Ryowalls now have distinct application icons.** Ryoport uses a
  compass for navigating local and remote machines, while Ryowalls uses its
  torii mark and fixed red sun. Both remain legible in launcher and dock tiles
  instead of falling back to the generic Ryoku seal.
- **A grayscale or monochrome wallpaper no longer paints the desktop blue.**
  matugen has no source hue to extract from a colourless picture, so it fell
  back to its built-in blue seed (`#4285f4`) and derived a blue / purple / pink
  palette from a black-and-white wall -- the visualiser bars, desktop widgets,
  and every `colors.json` reader then showed colours the wallpaper never had.
  The daemon now measures the wallpaper's chroma (the sampled still for a live
  wall) and, when it is essentially colourless, neutralizes the generated
  palette and tonal ramps to grays of the same luminance, so the surfaces track
  the picture instead of an invented hue (`ipc/matugen.go`).
- **The visualiser and desktop widgets no longer sweep colours the wallpaper
  never had.** Both floating surfaces built their spectrum from the full Material
  accent set -- `primary`, `tertiary`, and `error` -- so a single-hue wallpaper
  (a blue seascape, say) still drew a band of `tertiary` (a +60 hue rotation)
  and a band of `error` (Material's fixed red), painting pink and red bars over
  a picture that contains neither. They now sweep only the source-hue roles
  (`primary`/`secondary`, both derived from the wallpaper's own colour), so the
  spectrum reads as the wallpaper's hue for any wallpaper, while the chrome keeps
  the full palette for real error states
  (`quickshell/visualizer/Singletons/Scheme.qml`,
  `quickshell/widgets/Singletons/Scheme.qml`).
- **Hero App Launcher no longer has a diffuse rounded halo.** Its floating card
  keeps the crisp offset contact shadow and hairline frame while removing the
  broad soft falloff that made the launcher look like a larger rounded blur.
- **Quick settings now grows directly out of the Ryoku frame without freezing.**
  Its retained full-height body stays at fixed geometry while the frame chrome
  widens around it, then the content crossfades into the opened panel. Opening
  uses the 400 ms sidebar settle and closing uses the 200 ms acceleration, so
  interrupted toggles reverse cleanly without rebuilding a full-height blob.
  Long device and history lists are virtualized, media artwork is bounded and
  lazy, and monitor probes stop when the last sidebar closes. Cross-edge menu
  switches retract before transferring the frame, and Do Not Disturb uses the
  standard minus-circle glyph.
- **Dev deploys now receive and activate the Bluetooth audio policy.** The
  checkout deploy path previously omitted `apps/wireplumber`, so it diverged
  from packaged systems. It now lays the fragment under the user config and
  restarts WirePlumber only when the effective file changed (`deploy.sh`).
- **Nacre media now follows player presence without getting stuck hidden.**
  Pausing keeps the transport available, while a playerless widget collapses
  without disabling its host, so later MPRIS and tray changes reappear live.
  The elapsed timeline accepts click and drag seeking, and its popup closes
  when the last titled player disappears.
- **Clicking outside a pinned Nacre popup now closes it.** The first outside
  press is consumed by a temporary backdrop, while popup controls, bar widgets,
  and hover-only popouts retain their existing input behavior.
- **Nacre popouts now melt cleanly back into their island.** Closing content
  fades while the blob narrows toward its trigger, avoiding clipped text and
  full-width edge snaps. Closing also releases keyboard focus instead of
  blocking other windows. Bar Studio exposes frame size, frame roundness, and
  edge melt, and the media face disappears completely while idle.
- **Fresh installs no longer land on a black desktop.** Hyprland's autostart
  spawned the session env import and `systemctl --user start ryoku-shell` as
  separate fire-and-forget commands, so on a cold first boot the shell start
  could run before `WAYLAND_DISPLAY` reached the systemd user manager; the
  unit's ConditionEnvironment then skipped it silently, leaving keybinds alive
  but no bar, no wallpaper, no shell. The import, session target and shell
  start are now one chained command (`hyprland/modules/autostart.lua`), and
  doctor pushes the session env into the user manager before restarting the
  unit so it can heal a bitten session too.
- **The dock no longer disappears on fresh boots or short rails.** A dock with
  nothing pinned and nothing running rendered zero items, so on a freshly booted
  desktop it read as "the dock is gone" even with the widget enabled in Bar
  Studio; it now falls back to the stock role apps (terminal, browser, files)
  that resolve to an installed desktop entry, shown as launchers until real pins
  or clients replace them. On screens too short for the left rail's three zones,
  the dock was painted underneath the status/tray group; the centre zone now
  sits above its neighbours and the dock shrinks its tiles (macOS style, down
  to a floor) to fit the space the other zones leave it
  (`quickshell/pill/RailZone.qml`, `quickshell/pill/FrameRail.qml`,
  `quickshell/pill/BarWidgetHost.qml`, `framebars/widgets/RailDock.qml`).

### Added
- **A now-playing music sheet joins the wallpaper widgets.** Enable it in Ryoku
  Settings -> Desktop Widgets (or the desktop right-click): the sleeve leads, the
  song's synced lyrics run beside it under the line being sung, and the title, a
  wavy seek rail and prev/play-pause/next close it out. The card wears the
  album's own colour, not the theme's -- the accent, the lit lyric, the seek fill
  and the play tile all retune per song from a `ColorQuantizer` of the cover --
  with a now-playing equaliser on the sleeve and a soft accent glow on the play
  tile while a track moves. It follows any MPRIS player (`services/Media`), so
  Spotify, a browser tab or ryotunes all drive it. A corner button opens your
  music app (ryotunes by default; pick any installed app in the Hub, the same
  way the Keybinds page binds one), then minimises and restores it on later taps
  -- a scratchpad hide/show, since the compositor has no real minimise. Whenever
  no lyric sheet is up (lyrics off, or none found) a live cava spectrum in the
  sleeve's colour takes the space instead of a dead panel. A `cover` and a
  frosted `glass` style, an optional lyric sheet, drag/resize/anchor placement,
  and a live preview + controls in the Hub's Desktop Widgets page. The daemon
  (`ipc/music.go`) owns the enrichment off the QML thread: the full-size cover
  from the player, else Deezer then iTunes, and synced (or plain) lyrics from
  LRCLIB, cached under `~/.cache/ryoku/music`; the `Music` service holds the
  interpolated 60fps playback clock every lyric surface follows
  (`quickshell/shell/modules/desktop/music/` (incl. `MusicViz`, `MusicPulse`),
  `services/{Media,Music,AudioBars}.qml`, `utils/artcolor.js`,
  `modules/desktop/{Desktop,WidgetMenu,WidgetGlass}.qml`, `ipc/music.go`, `hyprland/scripts/ryoku-music-toggle`,
  `hub/quickshell/{MusicPreview,AppPicker,SettingsSheet}.qml`,
  `hub/quickshell/schema/WidgetsPage.js`).
- **The music sheet can go 9:16 with a living video backdrop.** A new Canvas
  toggle on the widget's right-click menu flips the sheet between the
  wide card and a portrait 9:16 "canvas" that full-bleeds a looping backdrop with
  the sung line and transport floating over it. The backdrop, in both shapes, is
  either a custom video or GIF you pick (a themed in-shell chooser with a live
  preview, not the system file dialog), or the track's **Spotify Canvas** pulled
  automatically: a bundled spicetify extension (`apps/spicetify/ryoku-canvas.js`)
  reads the Canvas from Spotify's own session and relays it to the daemon's
  loopback endpoint (Spotify's token is bot-gated, so the daemon cannot fetch it
  directly), which also matches a local clip in `~/.config/ryoku/canvas/<id>.<ext>`.
  Video software-decodes so it composites on hybrid GPUs. `Super+J` toggles the
  music app's scratchpad, the same dropdown idiom as `Super+H`
  (`modules/desktop/music/{MusicBackdrop,MusicTall}.qml`,
  `modules/desktop/MusicVideoPicker.qml`, `apps/spicetify/ryoku-canvas.js`,
  `ipc/music.go`, `hyprland/modules/binds.lua`, `systemd/user/ryoku-shell.service`).
- **QSBar's network popup can switch DNS providers in one click.** A four-way
  DHCP / Cloudflare / Google / Custom row mirrors Omarchy Quattro's compact
  provider control, with inline custom IPv4/IPv6 entry, pending state, and
  surfaced errors. The shared network topic reports NetworkManager's live global
  DNS choice, while `network.dnsSet` validates requests and delegates the
  privileged change to `ryoku-dns`; both QSBar variants use the same component
  (`quickshell/shell/modules/bar/barstyles/qsbar/components/DnsProviderSection.qml`,
  `quickshell/shell/services/Network.qml`, `ipc/network.go`).
- **Downloaded colour schemes appear in the Color-scheme picker.** RyoStore's
  Themes category installs a scheme into `~/.local/share/ryoku/themes/<id>` as a
  Noctalia-format `scheme.json`; the daemon converts its dark/light block into the
  34-role catalog palette and merges it into the theme world, so `ryoku-shell
  theme catalog` lists it (provider-tagged) beside the built-ins, `ryoku-shell
  theme <id>` applies it live through the existing static-theme render, and the
  settings seam validates the id. Installing never activates
  (`ipc/usertheme.go`, `ipc/{themecatalog,matugen,settings}.go`).
- **The desktop now has a location-aware calendar widget.** Wallpaper Glass and
  Ryoku Paper styles share a configurable four-to-eight-week grid with ISO week
  numbers, cached country and subdivision holidays, and indicators for personal
  events. It follows the wallpaper palette, supports hover and bounded passive
  motion, and uses the clock's drag, scale, lock, opacity and placement controls.
- **App Launcher now offers Main, Hero, and OkShell styles in Ryoku Settings.**
  Hero is restored as the default, and the selected style persists across shell
  restarts.
- **Nacre workspaces can be dots, numbers, or Obi's Japanese numerals.**
  Bar Studio switches the face live while preserving occupied-only filtering.
  Its island editor now marks the exact insertion point, labels empty drop
  targets, and makes the unused palette's removal state explicit.
- **Nacre is a configurable folder bar style.** Its three frosted top islands
  keep the hollow workspace rings and icon-led resource readout. Populated
  islands retain a visible capsule under width pressure. Bar Studio can drag
  widgets within and between auto-growing lanes or back to the unused palette
  without corrupting layout positions, and edits height, opacity, padding,
  spacing, gap, frame size and roundness, edge melt, island/OSD scale, workspace
  filter and the unified desktop frame live. Nacre owns the popup contents; Obi
  reuses them.
- **Stash is the floating "Features" sidebar: right edge, tall card, Super+T.**
  The Stash board moved off the full-span left sidebar into a floating card that
  slides open from the right edge (the music/Bluetooth popout envelope), centred
  and tall, with room for more feature panes. Super+T toggles it; dragging a file
  onto the right edge opens it too, and a drop there stashes straight into
  ~/Downloads/Stash. The `stash` surface is non-full-span, default anchor `right`;
  the system sidebar stays full-span (`quickshell/pill/FrameSurface.qml`,
  `quickshell/pill/FrameMenuManager.qml`, `quickshell/pill/popouts/SidebarFeatures.qml`,
  `quickshell/pill/shell.qml`, `framebars/FrameBars.js`).
- **The bar is a pluggable style you pick, and a first alternate ships: Obi.**
  The four-rail frame bar is now one of several drop-in bar styles. A `barStyle`
  key in shell.json selects the active one; each style is a self-contained folder
  under `quickshell/pill/barstyles/<id>` holding its own Scene, widgets, popouts
  and settings, listed in `barstyles/registry.js`. shell.qml draws the built-in
  frame scene for "sumi" (the default, now the left rail only: top, bottom and
  right are retired) and loads any other style's Scene per monitor. **Sumi** is
  that left rail; **Obi** is a new floating top bar mirroring the iNiR shell:
  kanji workspaces centred, CPU/RAM rings, a media chip with a live cava music
  visualizer, a clock, audio output and input controls (scroll to set, click to
  mute, a full mixer on hover), a battery, a Wi-Fi and Bluetooth connections chip
  (network list and device list on hover), a tray and weather, each with its own
  hover popout. Bar Studio becomes a style picker; for Sumi it edits the left
  rail, and for Obi it shows a per-widget show/hide panel, stored per style in an
  `obi` map in shell.json so a folder style owns its own settings. The shell's
  global menus (wallpaper on Super+W, quick settings on Super+Esc, the capture
  card on Super+S) adapt to a top-bar style: they drop from the top edge with
  their own card, and the capture card lands top-left. `ryoku doctor` retires
  top/bottom/right on existing installs. A guide to building your own style (with
  the shell's IPC, cava, MPRIS and service singletons) is `docs/barstyles.md`
  (`quickshell/pill/barstyles/`, `quickshell/pill/shell.qml`, `FrameMenuManager.qml`,
  `FrameMenu.qml`, `FrameSurface.qml`, `Singletons/Config.qml`,
  `hub/quickshell/pages/BarStudioPage.qml`, `hub/quickshell/Hub.qml`,
  `cli/internal/doctor/doctor.go`, `docs/barstyles.md`).
- **A video wallpaper stops while a window is fullscreen.** The player kept
  decoding behind a window covering every pixel of it, holding its buffers and
  burning CPU for nothing. The new Performance switch stops it on fullscreen and
  restarts it on the way out; the backdrop keeps painting the clip's still frame
  underneath, so nothing changes on screen. Fullscreen, not the widget layer's
  "covered", which reports covered when a workspace merely holds a window
  (`ipc/livewatch.go`, `hub/quickshell/pages/PerformancePage.qml`).

- **Keep Awake and Do Not Disturb live in quick settings.** Both toggles existed
  only on the retired deck, so Super+Esc had no way to hold the machine awake or
  silence toasts. Keep Awake also shows how long it has been on ("On for 3d"):
  a forgotten inhibitor blocks every suspend, and the age is the tell
  (`quickshell/pill/framebars/menus/MenuQuickSettings.qml`).

### Fixed
- **A shell reload no longer kills the apps you launched, or your clipboard.**
  The daemon's unit tore down its whole control group on stop, so every reload
  took down apps started from the shell (Discord and the like) and the wl-copy
  holding the last copy, leaving Discord gone and a just-copied screenshot
  un-pasteable. The unit kills only the daemon now (KillMode=process); it already
  reaps its own surfaces, so launched apps and the live selection survive. The
  start-rate keys also move to [Unit], where systemd actually reads them
  (`systemd/user/ryoku-shell.service`).
- **Recordings capture a live (video) wallpaper again.** gpu-screen-recorder's
  KMS capture drops ryoku-livewall's background layer, so a live wall recorded as
  a frozen still. A full-screen capture over a live wallpaper now goes through the
  ScreenCast portal (Ryoku's own in-frame picker, kept by a restore token), which
  records the full composite. Where the portal yields no stream (a hybrid-GPU
  DMA-BUF quirk in xdg-desktop-portal-hyprland) it falls back to wf-recorder and
  remembers that per GPU topology, so the dead-end picker is skipped next time.
  Region captures stay on the KMS path (`hyprland/scripts/ryoku-cmd-screenrecord`).
- **The recording island is clickable again under a folder bar style.** With a
  non-sumi style active (e.g. Obi) the frame overlay's input region collapsed to
  click-through whenever no menu was open, so the floating island rendered but ate
  no clicks mid-capture: pause, stop, the mutes and dragging all fell through, and
  only opening a menu (Super+S) revived it. The overlay now exposes the island's
  own rect while a capture runs or its chooser is open, whatever the bar style
  (`quickshell/pill/shell.qml`).
- **The Super+S capture card can arm the webcam mirror before recording.** The
  mirror (webcam self-view) toggled only from the record island's pre-record
  chooser, which is gone once a capture runs, so there was no way to set it up
  from the Super+S menu. The card's Record row now carries a webcam toggle beside
  the audio ones, so you activate and place the bubble before you start
  (`quickshell/pill/popouts/CapturePopout.qml`).
- **The overview's workspace and desktop switches no longer bounce you straight
  back.** Clicking a workspace cell, the "+", or a window dispatched the Hyprland
  focus and then closed the expo in the same breath; releasing the overlay's
  exclusive keyboard grab made Hyprland refocus the previously active window, so
  the switch was immediately undone and you landed back on the workspace you
  started from. Switches now defer their dispatch until after the expo has closed
  and released the grab, so they stick. Alongside it: the "+" add-workspace tile
  is a full-size target instead of a thin slat that was easy to miss; the "add
  desktop" card creates and moves you to a brand-new desktop instead of only
  previewing an empty one; and the grid and strip no longer render empty
  workspace slats or blank desktop cards, so nothing shows a slot that holds
  nothing (`quickshell/overview/Overview.qml`,
  `quickshell/overview/DesktopStrip.qml`).

### Changed
- **Super+Escape is now Ryoku's only full-height control sidebar.** Its existing
  home controls, notification history, and weather view are independent,
  catalogued modules selected from the fixed icon rail; media is registered as
  an optional module for future configuration. The retired system sidebar,
  illustrated power card, and recording deck are removed, while logout, lock,
  restart, shutdown, and the performance profiles remain in Super+Escape.
  Discord compaction now lives beside the Quick recording controls in the
  Super+S capture card. Doctor removes the retired system surface without
  touching sibling frame settings and preserves configured module ids across
  schema normalization (`framebars/MenuCatalog.js`,
  `quickshell/pill/framebars/menus/MenuQuickSettings.qml`,
  `quickshell/pill/popouts/CapturePopout.qml`, `cli/internal/doctor`).
- **The notification toasts slide in from their edge.** A toast used to open by
  the surface growing from nothing underneath it, which wiped the card into view
  top-down instead of animating it, and the compositor animated the layer on top
  of that so the column flickered on a workspace switch. Each card now slides in
  from the edge it is anchored to and fades with it, the surface only eases its
  height once cards are already up, and the layer is marked no_anim like the
  launcher (`quickshell/pill/NotificationPopups.qml`,
  `hyprland/modules/decoration.lua`).
- **The notification toasts grow from the corner, expand, and open.** The popups
  faded straight in, flat-spaced and flush to the corner; they now float 16 px off
  it, sit 14 px apart, and each card grows out of the anchored top corner on
  arrival and recedes into it on close (Motion.notifIn / notifOut, scaled about
  that corner so the grow never clips). Each toast is compact: the body clamps to
  two lines and the actions tuck away behind an expand chevron that slides the
  card open to the full body and its buttons; an open button fires the
  notification's default action, the click-to-open a toast could not reach
  before. The history panel is unchanged bar the open button appearing when an
  app sent a default action (`quickshell/pill/NotificationPopups.qml`,
  `quickshell/pill/NotificationCard.qml`, `Singletons/Motion.qml`).
- **The volume, mic and brightness OSDs read out their percentage.** The
  bottom-centre OSD showed only an icon and a value bar; it now carries the exact
  value beside the bar as a right-aligned mono percentage, reusing the same
  clamped 0..1 the bar fills to so the number and the bar never disagree. Output
  volume is capped at 100% upstream by `wpctl -l 1`, so the readout tops out with
  the bar (`quickshell/pill/Osd.qml`).
- **The wallpaper menu opens with its images already in hand, and stays light on
  memory.** Super+W re-ran `index.sh` (a ~0.4s thumbnail and dominant-hue pass) on
  every open, so the colour-scheme cards painted a beat before the wallpaper tiles
  and the belt glitched as thumbnails popped in; it also built a tile for every
  wallpaper in the folder though only a handful are ever on screen. A resident
  `WallIndex` singleton now owns the index pass (prewarmed once at pill start,
  refreshed only in the background on open, and a no-op when the set is unchanged
  so a re-open never churns the belts), and each belt builds only the on-screen
  tiles plus a one-cell buffer, incubated off the main thread and decoded just
  before they drift into view. Opening no longer freezes, and the selector holds
  ~a dozen thumbnails at a time instead of the whole folder
  (`quickshell/pill/Singletons/WallIndex.qml`, `Singletons/qmldir`,
  `framebars/menus/MenuWallpaper.qml`, `framebars/menus/WallBelt.qml`,
  `quickshell/pill/shell.qml`).
- **The wallpaper menu filters by images or live.** A type row (All / Images /
  Live) sits above the colour swatches, shown when the folder holds any live
  wallpapers; picking a type resets the colour pick and re-reads the swatch strip
  for that set (`framebars/menus/MenuWallpaper.qml`).
- **The quick-settings sidebar opens without a hitch.** Super+Esc built its whole
  control-centre body (connectivity toggles, sliders, month calendar, system
  gauges, power profiles) synchronously on the first frame of the reveal, stalling
  the slide. A band-filling menu is fixed-size, so its body now incubates off the
  main thread and the reveal stays smooth (`quickshell/pill/FrameMenu.qml`).
- **The wallpaper menu's grid became a scrolling belt.** Super+W's frame menu now
  shows its wallpapers as two endless belts that drift in opposite directions (the
  top rightwards, the bottom leftwards) and speed up on a scroll, with a colour
  filter above. The belt reads cached image and live-video thumbnails from the
  shared `index.sh` (the standalone switcher's engine), so a click applies live
  through the daemon and never dismisses the menu; the menu is anchored
  bottom-centre at 1400 wide (`quickshell/pill/framebars/menus/MenuWallpaper.qml`,
  `WallBelt.qml`, `WallTile.qml`, `WallVideo.qml`, `WallColors.js`,
  `framebars/MenuCatalog.js`, `framebars/FrameBars.js`).
- **The bar's status widgets open popout cards.** Clicking the network,
  Bluetooth, battery, audio, or system-monitor widget grows a frame-edge card
  out of the rail at the point you clicked and melts it back on close, all from a
  shared card kit (`quickshell/pill/popouts/PopoutCard.qml` and its siblings) so
  every card opens and dismisses the same way. The cards carry the controls, not
  just read-outs: audio is a full mixer (output, input, per-app volume, and the
  Bluetooth codec and profile), Bluetooth pairs and connects devices and shows
  their battery and codec, battery carries the gauge, the power profiles, and a
  detail panel, network runs Wi-Fi, and system monitor charts CPU, memory, and
  temperature. The speaker opens the mixer instead of muting, the mic still mutes
  on click, and both keep the scroll wheel for quick volume
  (`quickshell/pill/popouts/*`, `quickshell/pill/SysMonitor.qml`,
  `quickshell/pill/framebars/widgets/RailStatus.qml`, `Singletons/BtLink.qml`,
  `Singletons/Sysinfo.qml`, `Singletons/Audio.qml`).
- **Super+W is the unified wallpaper and colour-scheme switcher now.** The basic
  bottom-left wallpaper menu is replaced on Super+W by the animated switcher
  (formerly Super+C, now freed), reskinned into the shell's Material style and
  anchored bottom-centre: two belts of cached image and live-video tiles drift in
  opposite directions with a colour filter, and a Color-scheme mode flows the 57
  themes through the same belts, with a Follow-wallpaper toggle and a Default
  button in the corner. The theme catalog is served by the daemon
  (`ryoku-shell theme catalog`, derived from `themePalettes`) and applied through
  a new `ryoku-shell theme <scheme>` verb; the old brutalist chrome (chamfers,
  vermillion, Fraunces) gives way to the shell's rounded Material tokens
  (`quickshell/wallpaper/*`, `ipc/themecatalog.go`, `ipc/daemon.go`,
  `ipc/main.go`, `ipc/settings.go`, `dev-binds.sh`).
- **The recording island matches the frame now.** The floating record island was
  rebuilt from the retired liquid blob into a crisp card (the frame surface, a 2px
  outline and rounded corners, like `FrameChrome` and the frame-edge cards): it
  slides in from its docked edge and melts back on stop, still draggable, still
  docking to the nearest edge, flipping vertical on a side edge, and tucking to a
  pulsing nub when hidden. The old drop-merge physics and blob body are gone, and
  it clears any bar on the edge it lands on (`quickshell/pill/RecordHud.qml`,
  `quickshell/pill/shell.qml`).
- **The screenshot menu became the capture card (clean cutover).** The in-band
  `screenshot_menu` and its `Screenshots` / `ScreenRecording` menu-widgets are
  retired for the surface card; `MenuCapture.qml` is deleted, a doctor reconciler
  strips a lingering `menus.screenshot_menu` from a saved store, and
  `FrameMenuManager` ignores any leftover so `menu screenshot` always answers the
  card (`ipc/settings.go`, `quickshell/pill/MenuWidgetHost.qml`,
  `framebars/MenuCatalog.js`, `framebars/FrameBars.js`,
  `cli/internal/doctor/reconcile_screenshot_menu.go`).
- **Bar Studio no longer offers eight widgets that do not belong on a rail.**
  App Launcher, Clipboard, Layout Switcher, Color Picker, Power Profile, Reboot,
  Screenshot and Wallpaper leave the frame-bar catalogue, so the add-to-rail
  drawer only lists the seventeen widgets that render as a rail button;
  `normalize` strips any of the eight from a saved rail and the default left
  stack drops its `reboot`. Their menu versions are untouched (they are still
  frame-menu content). The two now-orphaned rail components (`RailPowerProfile`,
  `RailLayoutSwitcher`) are deleted and `RailAction` narrows to lock, log out and
  shut down (`framebars/BarCatalog.js`, `framebars/FrameBars.js`,
  `quickshell/pill/BarWidgetHost.qml`,
  `quickshell/pill/framebars/widgets/RailAction.qml`).

### Added
- **Super+S opens a capture card, not a full-width menu.** The screenshot keybind
  now grows a compact frame-edge card (the shared `PopoutCard` skin, like the
  music and bluetooth cards) instead of the old in-band menu with its oversized
  buttons. Screenshot is the quick path: a delay chip (0/1/3/5/10s), a save-target
  chip (Screenshots folder, clipboard, or both), and four one-tap modes (All,
  Screen, Window, Region). A "Beautify after" switch hands the saved shot to
  Ryoshot; Record carries desktop/mic toggles and Screen/Region starts (the
  floating island takes over the live controls) plus an "Edit in Ryomotion when
  done" switch; a footer notes that Super+Shift+S opens Ryoshot. The delay, save,
  beautify, audio and edit choices persist (`capture.json`, `record.json`) and the
  terse chips carry hover bubbles. New `quickshell/pill/popouts/CapturePopout.qml`,
  registered as a left surface (`quickshell/pill/FrameMenuManager.qml`,
  `quickshell/pill/FrameSurface.qml`); `Singletons/Capture.qml` and
  `Singletons/Recorder.qml` gain the persisted options and hand-offs.
- **Beautify a shot and edit a recording straight from the card.** With "Beautify
  after" on, a finished screenshot opens in Ryoshot's beautify editor
  (`RYOSHOT_OPEN` loads the file into the compose phase). With "Edit in Ryomotion
  when done" on, a finished Quick recording opens in Ryoku Motion once the clip
  finalises (`hyprland/scripts/ryoku-cmd-edit-recording`).
- **The dock previews an app's windows on hover.** Hovering a dock icon that has
  open windows grows a strip off the rail, welded to the icon, with a live
  thumbnail of each window (a `ScreencopyView` of the toplevel), its title, and a
  close cross; a click focuses that window, the cross closes it. The strip glides
  along the rail as you move between icons and melts shut when you leave it. It
  rides the shared frame-bar popout blob and a small hover-latched singleton, so
  it never enters the menu state and its tiles stay clickable over the desktop
  (`quickshell/pill/popouts/DockPreviewPopout.qml`,
  `quickshell/pill/Singletons/DockPreview.qml`,
  `quickshell/pill/framebars/widgets/RailDock.qml`, `quickshell/pill/FrameMenuManager.qml`).
- **A music widget on the rail opens a now-playing card.** A vertical spectrum
  strip, fed by a shared cava playback feed, rides a rail zone and self-hides
  until a player reports a track; a left click grows a now-playing card off the
  frame edge, and the close melt retracts it fully into that edge. The card
  carries the sleeve (its dominant tone lifted to a vivid accent that tints the
  transport, the progress line and the card's wide spectrum sweep), the title,
  the artist and previous/play/next. The spectrum spreads its bars evenly across
  whatever width or height it is given, and cava is claimed only while a music
  surface is shown, so a playerless desktop never runs the analyser
  (`quickshell/pill/framebars/widgets/RailMusic.qml`,
  `quickshell/pill/popouts/MusicPopout.qml`, `quickshell/pill/MusicBars.qml`,
  `quickshell/pill/Singletons/AudioBars.qml`).

### Fixed
- **Bar menus and popout cards now open on the bar you clicked.** A rail widget
  welded its card to whichever screen edge the click landed nearest, and the side
  edges were tested first, so any widget on the top or bottom bar sitting within a
  side rail's depth opened its card on the left instead. Horizontal rails own the
  shared corners, so the edge test follows that order now and every popout (audio,
  battery, Bluetooth, network, system monitor, music, quick settings) grows out of
  its own bar (`quickshell/pill/FrameMenuManager.qml`).
- **Quick settings keeps its whole panel on the top, bottom, and right bars.** A
  band-filling menu reported its own height back as its content height, so the
  height it settled on depended on which binding resolved first: the same menu drew
  full height on one edge and a stunted panel on another, with the calendar and
  system sections scrolled out of reach. A menu whose sole widget fills the band
  takes the band height outright on every anchor now, and a top or bottom menu
  opens over the widget that summoned it instead of jumping to screen centre
  (`quickshell/pill/FrameMenu.qml`).
- **Closing a menu no longer strands an empty panel on the frame.** On close the
  menu snapped back to its configured anchor, which moved its resting rect and
  drove the reveal open again on that edge, leaving a blank sidebar behind. The
  anchor is held through the close now (`quickshell/pill/FrameMenuManager.qml`).
- **Bar icons hold one size across bars of different thickness.** Icons scaled with
  the band, so a 32px bar drew them at two thirds the size of a 48px one. The
  button still fits its band, but the glyph, its label, the sysmon gauge, the
  notification badge, and the dock icons only shrink once the band is too thin to
  hold them (`quickshell/pill/framebars/widgets/*`).
- **The app launcher follows a fixed named theme, not just the wallpaper.** The
  launcher read only `~/.cache/ryoku/colors.json` (the live wallpaper palette),
  so selecting a coded preset -- whose palette the daemon publishes as
  `shell.json`'s `themePalette`, not into `colors.json` -- left the launcher on
  the last wallpaper's colours. It now resolves colour through the same
  three-layer chain the pill uses (named preset -> wallpaper -> compiled default),
  so both a preset and a wallpaper retint it live (`quickshell/launcher/shell.qml`).
  The daemon also authors `colors.json` on a static-theme apply now, so every
  reader of that file tracks a preset, not only the wallpaper path
  (`ipc/matugen.go`).
- **yazi's file list carries the palette, not just its chrome.** The theme tinted
  tabs, mode and borders but had no `[filetype]` block, so the list itself --
  directories and file kinds -- fell back to yazi's built-in flavour and never
  tracked the wallpaper. A `[filetype]` block now colours directories,
  executables, media, archives and orphans from the base16 ramp (`url`-matched,
  yazi 26's key), so the list matches kitty's ANSI colours
  (`matugen/templates/yazi.toml`).
- **Rail widgets share the whole bar instead of clamping into thirds.** Each
  rail's three zones now pack across its full length -- start from the leading
  edge, centre on the midpoint, end from the trailing edge -- so a heavy zone
  (the default left rail's status stack) uses the whole bar and never clips off
  the far end, and each widget carries a band-scaled gap so buttons read as
  separate instead of butting together. The hidden orientation no longer
  instantiates the centre zone's widgets a second time
  (`quickshell/pill/FrameRail.qml`).
- **A rail widget's popup opens on that rail's edge.** A menu or card opened from
  a bar widget now grows out of the edge its trigger sits on rather than the
  record's configured edge, so a widget on the top rail drops its popup straight
  down from the top instead of stranding it on the left; a command or IPC open
  with no widget rect still uses the configured anchor
  (`quickshell/pill/FrameMenuManager.qml`).
- **Adjacent rails no longer overlap at the corners.** A top or bottom rail spans
  the full width and a side rail the full height, so their corner widgets landed
  on top of each other -- the top rail's first icon sat over the left rail's
  brand seal. Horizontal rails now own the corners and each side rail insets
  between the top and bottom bands, fitting between them instead of colliding
  (`quickshell/pill/FrameRail.qml`, `quickshell/pill/Bar.qml`).
- **The active workspace pill sizes to its own apps, not a fixed three-slot
  width.** It reserved room for three icons even while showing one, so it read as
  a wide near-empty oval beside the compact inactive pills; it now sizes to its
  icons like they do, set apart by its highlight
  (`quickshell/pill/framebars/widgets/RailWorkspaces.qml`).
- **The active workspace pill no longer bulges over its neighbours on a top
  rail.** Its highlight anchored to both the top (as the pill's outer edge) and
  the bottom (as any horizontal rail), so on a top rail the two anchors stretched
  the thin line into a tall bright capsule that overran the pill and reached into
  the pills beside it. The bottom-edge anchor now belongs to the bottom rail
  alone, so the highlight stays the thin edge line it is meant to be on all four
  edges (`quickshell/pill/framebars/widgets/RailWorkspaces.qml`).
- **A notification fired twice no longer stacks two identical popups.** With one
  server and id-based dedup, a duplicate could only come from an app posting the
  same alert twice (or two apps posting the same one), each a distinct id; a new
  popup whose app, summary and body match a live one now replaces it with a fresh
  timer instead of adding a twin (`quickshell/pill/Singletons/Notifs.qml`).
- **A notification popup shows its time left as a draining frame, and no longer
  flashes as it leaves.** Each popup traces a warm accent border that starts
  around the whole card and recedes from the top as its display timer runs down
  (the frame is off for the persistent popups and the history panel). The popup
  surface now resizes with the card slide instead of snapping, so a leaving card
  is not clipped as the surface shrinks under it (`quickshell/pill/NotificationCard.qml`,
  `quickshell/pill/NotificationPopups.qml`).
- **Shell text uses its own sans face instead of a monospace fallback.**
  `Theme.fontPrimary` was empty, so every pill surface (the bar, menus, OSD and
  notifications) fell back to the platform default, which renders monospace here;
  it is now Space Grotesk, the shell's design UI font that the Hub already uses
  (`quickshell/pill/Singletons/Theme.qml`).
- **Notification cards drop the empty action button and lead with the message.**
  A bare freedesktop "default" action (invoked by clicking the notification, not
  a button) drew an empty pill; the card now shows a button only for an action
  with a label, and the app name is a quiet label so the summary reads first
  (`quickshell/pill/NotificationCard.qml`).
- **Turning a rail off reclaims its edge on the running desktop at once, and a
  rail's reveal state applies live.** `edgeReserve` now collapses a disabled rail
  to the frame lip instead of holding its full band, so switching a rail off in
  Bar Studio frees its edge immediately instead of leaving an empty margin until
  the next restart. The per-edge reveal baseline (`edgeRevealed`) also re-derives
  from the live config when a rail's `reveal` changes, not only at startup, while
  a CLI or hover reveal on an untouched edge is preserved
  (`quickshell/pill/shell.qml`).
- **The frame's band thickness and corner radius are adjustable again.** The old
  `frameBorder`/`frameRadius` keys had no runtime consumer (the shell drew from
  compiled tokens); the shell now reads `frameThickness` (the band width, and the
  edge reserve) and `frameCorner` (the desktop hole radius) live, so Bar Studio's
  frame controls change the running frame (`quickshell/pill/shell.qml`,
  `quickshell/pill/Singletons/Config.qml`).
- **A rail thinner than its widgets no longer overflows.** A bar widget kept its
  48px parity size whatever the rail's thickness, so shrinking a rail left the
  buttons and their hover highlight spilling past the band; each widget now scales
  to the rail's thickness, so a small bar stays clean (`quickshell/pill/Bar.qml`).
- **Recordings land in one directory, and the deck lists the one they land in.**
  The recorder resolved the directory properly (env override, then the Videos
  dir), the deck's list hardcoded `$HOME/Videos/Recordings`, and the Hub's page
  claimed a third answer in prose. On a box with a custom `XDG_VIDEOS_DIR` the
  writer and the list parted company and the list showed nothing. All three now
  resolve `Ryoku.Ui.Singletons.Paths.recordingsDir`, which reads the Hub's new
  `directory` setting and falls back the way the recorder always did. The list
  also stops filtering for our own filename shape, so a clip Ryoku Motion
  recorded shows up beside the rest instead of being invisible
  (`ui/Singletons/Paths.qml`, `quickshell/pill/DeckRecord.qml`,
  `hyprland/scripts/ryoku-cmd-screenrecord`).
- **Picking the live scheme turns wallpaper-following back on.** The colour master
  was split in two: the Appearance page and the sidebar write `theme.theme`
  (shell.json), while the dynamic pipeline is gated on `followWallpaper`
  (theme.json) -- and nothing wrote that key except the rice system, which turns it
  off. So once anything had turned it off, selecting the live scheme selected a
  scheme that never regenerated. `theme.theme` is the master now and
  `followWallpaper` is its shadow, synced wherever the theme selection is resolved
  (`ipc/settings.go`, `ipc/matugen.go`).
- **The wallpaper scheme-type knob is gone.** It could be set to
  `scheme-monochrome` or `scheme-neutral`, which drain the colour out of any
  wallpaper; that reads as broken colour generation rather than as a choice. The
  pipeline always generates with `scheme-tonal-spot`, and a `schemeType` left in
  matugen.json is inert and dropped on the next write (`ipc/matugen.go`).
- **Desktop widget and visualiser colour is no longer stuck on black.** The
  per-shell colour singleton was briefly named `Palette`, which collides with
  QtQuick's own `Palette` value type; the module type wins over the directory
  import, so every `Palette.<role>` read in eight configs resolved to `undefined`
  and each ink and accent fell back to opaque black. That is why the clock read
  as flat black text and the spectrum drew black bars whatever the wallpaper.
  The singleton is `Scheme` now, a name QtQuick does not own
  (`quickshell/*/Singletons/`, `hub/quickshell/Singletons/`).

### Added
- **The bar has a music spectrum, and it opens a music card.** A new `music`
  rail widget draws a mini vertical spectrum fed by a shared cava playback feed
  (`AudioBars`, owner-refcounted so the analyser runs only while a music surface
  is showing); it appears once a player reports a track, dances while playing
  and falls to dim slivers on pause. Clicking it grows the `music` card out of
  the frame edge: the sleeve art with a soft black plate-shadow and hairline
  rim, title and artist, a wide spectrum strip, a progress hairline with mono
  timestamps, and a previous / play-pause / next transport. The card is
  pointer-only like the voice toast (no keyboard grab) and dismisses on an
  outside click (`quickshell/pill/Singletons/AudioBars.qml`,
  `quickshell/pill/MusicBars.qml`,
  `quickshell/pill/framebars/widgets/RailMusic.qml`,
  `quickshell/pill/popouts/MusicPopout.qml`,
  `quickshell/pill/FrameSurface.qml`,
  `quickshell/pill/FrameMenuManager.qml`, `framebars/BarCatalog.js`).
- **fish, fzf and yazi follow the palette.** Three surfaces that stayed stock
  while everything around them retinted: the shell you type in, the finder behind
  Ctrl-R and Ctrl-T, and the file manager. fish needs no include line -- the
  palette lands in `conf.d`, which fish sources itself -- and fzf rides the same
  file, appending to `FZF_DEFAULT_OPTS` rather than replacing it. yazi gets a
  `theme.toml`, a file Ryoku otherwise does not ship, so nothing of yours is
  overwritten. A roster group ABSENT from a saved matugen.json now defaults ON;
  absent-means-off would have kept every app added from here on stock for anyone
  who had opened the appearance page once (`matugen/templates/colors.fish`,
  `matugen/templates/yazi.toml`, `matugen/apps.toml`, `ipc/matugen.go`).
- **Surfaces that float on the wallpaper pick their colour against the
  wallpaper.** A Material role is a tone chosen to read on a *panel*; the
  visualiser and a bare desktop widget have none, so consuming roles verbatim
  put near-black accents on a light scheme. matugen already computes the tonal
  ramps behind those roles and the daemon discarded them, so it now publishes
  them to `~/.cache/ryoku/tones.json` alongside a coarse CIE L* map of the
  wallpaper (`wallpaper-tone.json`, written on every wallpaper change whichever
  engine owns the palette). The new `Ryoku.Ui` `Ink` singleton samples the map
  under a surface's own rect and picks a ramp tone far enough from it to read:
  spectrum bands light against the picture behind them, and clock ink that goes
  white over a dark corner and black over a bright one. Accents are held to the
  tones where the ramps still carry chroma, and the light/dark direction is
  fixed once per field so neighbouring bars cannot flip (`ipc/walltone.go`,
  `ipc/matugen.go`, `ui/Singletons/Ink.qml`, `quickshell/visualizer/`,
  `quickshell/widgets/`).

### Changed
- **The launcher is an OkShell-style app launcher for now.** The Raycast-class
  card grew into something that did not fit the shell: a 1109x630 slab leading
  with a clock, weather and a photograph, and rendering its top hit twice. It is
  replaced by a stand-in modelled on OkShell's app launcher, dressed as the
  quick-settings sidebar -- radiusWidget corners, a 1px outline, a sumi edge, the
  active-tile plate for the selection -- and it slides in from the top edge on the
  sidebar's own push (420ms OutQuint), surface and all, so nothing is left behind
  to frost. The selected row slides its content right by 30px, OkShell's cue. The
  eye reveals the entries marked NoDisplay. It searches applications ONLY: the
  dispatcher, the twelve providers, the action panel and the AI ask are still in
  the directory, unwired, and the launcher doc says so
  (`quickshell/launcher/shell.qml`, `hyprland/modules/decoration.lua`).

- **The live palette moved to `~/.cache/ryoku/colors.json`, and the per-shell
  palette singleton is now `Scheme`.** The daemon authored the wallpaper palette
  (and `hypr-colors.lua`) under the retired wallust engine's cache dir; it now
  writes both under `~/.cache/ryoku/`, and every thin reader (pill, widgets,
  visualiser, plugin kit, launcher, overview, wallpaper, and `Ryoku.Ui`'s Tokens)
  plus `decoration.lua`'s border source follow the new path. The `Wallust`
  singleton is renamed `Scheme` (file and qmldir) across every shell, the
  daemon's dead engine knob is dropped, and the ryowalls tune state file is
  `ryoku-ryowalls.json` (`ipc/matugen.go`, `ipc/wallpaper.go`,
  `quickshell/*/Singletons/`, `ui/Singletons/Tokens.qml`, `matugen/config.toml`,
  `hyprland/modules/decoration.lua`).
- **Every desktop surface follows the daemon palette, and the window border
  follows named themes too.** The audio visualiser, the desktop widgets and
  their right-click menu, the plugin kit, and the `Ryoku.Ui` Tokens the Hub and
  apps draw from each carried their own colour singletons that re-derived tints
  from the raw terminal palette (vivid/shade/tone) or pinned a fixed vermillion
  accent, so they drifted from the theme and, under a fixed named scheme, showed
  a stale wallpaper palette (the desktop widgets reading orange instead of the
  system colours, the Hub never retinting at all). They are now thin readers
  that consume the daemon palette verbatim, resolved the pill's way (a named
  scheme's themePalette wins, then the live wallpaper colors.json, then the
  compiled default), so every surface retints on any scheme change, a named
  theme or a wallpaper switch, and the widget menu paints its accent from the
  palette. The compositor border follows the palette in both modes now:
  `genConfig` omits the fixed `col.active_border` whenever a named theme is
  active, not only when following the wallpaper, so `decoration.lua`'s
  hypr-colors.lua border wins (`quickshell/visualizer/Singletons/`,
  `quickshell/widgets/Singletons/`, `quickshell/plugins/kit/Singletons/`,
  `ui/Singletons/Tokens.qml`, `hub/backend/hypr.go`).
- **Follow-the-wallpaper theming runs matugen natively in the daemon.** Match
  wallpaper on + the dynamic Wallpaper scheme -> setting a wallpaper (or a
  scheme knob patch) generates the Material 3 scheme with matugen using the
  configured mode/scheme type/contrast/preference; the daemon writes the shell
  colors.json (base16 + 30 Material roles) and fans it into GTK 3/4 and the
  app suite via the matugen templates, and sets the desktop color-scheme. A
  fixed named theme or Match wallpaper off leaves it idle (`ipc/matugen.go`,
  `ipc/wallpaper.go`, `ipc/settings.go`, `matugen/templates/`).
- **The shell now uses configurable frame bars instead of an Atoll bar.**
  Each monitor owns one shared frame scene with independent top, bottom, left,
  and right rails. The stock profile keeps a compact clock at the top and the
  familiar quick settings, workspace, dock, tray, network, and clock flow on
  the left; the other rails are ready to enable in Bar Studio, a section of
  Ryoku Hub (Super+comma), where bounded widgets, menus, and the
  preserved Stash and System surfaces can be arranged and saved through the
  normal configuration pipeline. `ok-frame` and `ryoku-frame` share this
  topology while changing only chrome and metrics. All frame menus, power,
  voice, keyring, and plugin surfaces use one monitor-local manager, which
  handles replacement, input masks, Escape, backdrop, focus-loss, and
  fullscreen closure safely. The retired Atoll renderer and configuration
  migrate to this contract through `ryoku doctor`
  (`quickshell/pill/framebars/`, `hub/quickshell/barstudio/`, `ipc/`,
  `cli/internal/doctor/`).
- **The notification, clipboard, and media panels moved into the quick settings
  sidebar as slide-in pages.** Their standalone frame menus were mostly empty
  surfaces; now the bar's bell and clipboard buttons open the sidebar straight
  onto that page, media rides a Shelf row beside them, and each page loads on
  first visit and stays cached so the first open stays light. The retired menu
  records leave the catalog, the way the clock menu did
  (`quickshell/pill/framebars/menus/`, `quickshell/pill/framebars/widgets/`,
  `framebars/FrameBars.js`, `framebars/MenuCatalog.js`).
- **Sidebar motion speaks one language.** A `push` token (420 ms on an OutQuint
  settle) drives every page transition, and a global `speed` multiplier read
  from performance.json scales the shell's animation in one place; the hover
  bubbles and press dips draw from the same vocabulary
  (`quickshell/pill/Singletons/Motion.qml`).
- **Sidebar cards carry a sumi edge.** A single 1 px light line along the top of
  the tiles, the media and calendar cards, and the docked footer band, like the
  lit edge of layered paper (`quickshell/pill/SumiEdge.qml`).
- **Today's calendar cell wears the 力 seal.** The brand mark sits faint behind
  the day number, inside the primary-tinted ring
  (`quickshell/pill/Calendar.qml`).

### Fixed
- **The voice surface no longer records when dictation is off.** The Super+grave
  tap with Voxtype absent opened a wave-mode surface that ran cava on the mic
  and never closed, because the off flag set on the record never reached the
  surface delegate, which read the static config record rather than the live
  one. The live record's open-time fields now reach the delegate, so the surface
  shows a quiet "Dictation off" note, spawns no capture, and auto-dismisses
  (`quickshell/pill/FrameMenuManager.qml`, `quickshell/pill/FrameSurface.qml`,
  `quickshell/pill/VoiceSurface.qml`).
- **Frame surfaces open again, and stay open.** On a real session every menu,
  the power card and both sidebars vanished a few milliseconds after opening.
  A modal surface takes the overlay's exclusive keyboard focus, and the shell
  also ran a `HyprlandFocusGrab` over the same window; Hyprland cleared the
  grab the instant focus moved to the grabbing layer and the clear handler shut
  everything. The grab is gone: the full-screen mask, the backdrop press, and
  Escape already covered dismissal (`quickshell/pill/shell.qml`).
- **The bar accepts clicks.** The overlay declared a `frameBars` property and
  gave the rail host the id `frameBars`; the id shadowed the property, so every
  rail rectangle resolved to zero, the input mask claimed no bar strip, and no
  widget ever saw a pointer. The host is now `frameRails` and the geometry
  reads through `overlay.` (`quickshell/pill/shell.qml`).
- **Rail zones sit where their names say.** Every zone left-aligned its group,
  so a centred clock drifted a third of the screen off-centre and an end group
  never reached the far edge. Zones now hold start, centre, and end, centre
  their widgets on the rail's short axis, and space them evenly
  (`quickshell/pill/RailZone.qml`, `quickshell/pill/FrameRail.qml`).
- **A surface clears its own rail.** Every body was inset by the *top* rail's
  thickness whatever edge it grew from, so left and right surfaces slid under
  the side rails and lost their first characters. The manager now takes a
  clearance per edge and menus draw above the rails, with the body padded off
  its own border (`quickshell/pill/FrameMenuManager.qml`,
  `quickshell/pill/framebars/menus/MenuColumn.qml`).
- **The dock shows applications, not initials.** It rendered the first letter
  of each window class. It now resolves the desktop entry's icon, falls back to
  the window class, and keeps the initial only when neither resolves; a dock
  click no longer fires a menu request for a menu that never existed
  (`quickshell/pill/framebars/widgets/RailDock.qml`).
- **Asking twice closes.** Only power and plugins toggled; every other surface
  re-opened on a second press. A rail button and its `ryoku-shell bar <id>`
  command now read as one toggle, while the daemon-owned keyring and voice
  surfaces still replace their record (`quickshell/pill/FrameMenuManager.qml`).
- **`ryoku doctor` sheds the whole Atoll-era config, not four keys of it.**
  A migrated `shell.json` kept `barStyle`, the bar layout/toggle lists, the
  island knobs, the dyad and washi variants and the sidebar openers: twenty-two
  settings no shipped surface reads. They are all retired now, and the settings
  the shell still reads are left alone (`cli/internal/doctor/doctor.go`).
- **Workspaces, Super+Esc power, and every shell surface work again after a
  Hyprland restart mid-session.** `ipc/daemon.go` read `HYPRLAND_INSTANCE_SIGNATURE`
  once at launch and never refreshed it, and the daemon can outlive its compositor
  (a checkout `deploy.sh` starts it detached with `setsid`; a relogin or crash
  brings up a new Hyprland under a new signature). The stale daemon kept the dead
  instance's signature and the new login's daemon exited on "a daemon is already
  running", so the incumbent stayed in charge and supervised every Quickshell
  child (`qsEnv` inherits its env) against the dead Hyprland IPC socket: the
  workspace indicator froze (no events; its `hyprctl activeworkspace` fallback hit
  the dead socket too) and every monitor-aware command (power, launcher, mixer,
  ...) resolved no active monitor. The daemon now reports its launch-time instance
  over a `signature` command, and a starting daemon takes over an incumbent bound
  to a different instance -- quit it, wait for the socket to free, rebind -- so the
  login-time daemon always wins and reconnects the shell to the live compositor. A
  same-session double-start still refuses (`shouldTakeOver`, `daemonSignature`,
  `quitStaleDaemon`; covered by `TestShouldTakeOver`, `TestSignatureCommand`).
- **The screen recorder captures the microphone out of the box.**
  `pill/RecordHud.qml` defaulted `optMic` to false, so a Quick capture recorded no
  voice; it now defaults on (desktop audio stays opt-in) and the pre-record chooser
  still offers a one-tap disable. The backend already selects the default PipeWire
  source.
- **Matugen theme failures are no longer silent.** `ipc/wallpaper.go`
  (`renderApps`) swallowed matugen's exit with `_ = …Run()`, so a failed GTK/Qt
  fan-out (missing palette cache, unreadable template) produced nothing with no
  indication -- the "matugen was enabled but generated no themes, and I couldn't
  tell why" report. It now logs matugen's output on failure and logs a missing
  palette cache, and pre-creates the output dirs to keep matugen's own
  "folder doesn't exist" warnings out of the log. Generation itself is intact:
  matugen 4.x creates missing dirs and the shipped templates render GTK+Qt
  correctly (verified live).
- **Weather location now reaches the launcher and desktop widget.** Changing the
  location in Settings updated the pill, but the launcher's and widget's Weather
  singletons never read the explicit `weatherLocation`: they read the shared
  `~/.local/state/ryoku/weather-loc.json` once at startup (no file watch), and
  their IP-locate wrote a query-less entry that clobbered the pill's query-keyed
  cache, so the launcher kept showing the previously-located city even after a
  restart. Both are now live consumers of the pill's authoritative cache: they
  `watchChanges` and re-read on every update, and no longer write it (the pill is
  the sole resolver/writer), so a location change reaches all three surfaces at
  once (`launcher/Singletons/Weather.qml`, `widgets/Singletons/WeatherData.qml`).

### Added
- **Folder icons follow the wallpaper.** Every palette change now retints the
  file-manager folders to the same accent the shell and GTK use. A helper
  (`hyprland/scripts/ryoku-cmd-folders`) maps the accent to the nearest Papirus
  folder colour and builds a ~300K icon theme under `~/.local/share/icons` that
  inherits Papirus-Dark and overrides only the folder icons with Papirus's
  matching colour set, so it needs no root and never `.pacnew`s the packaged
  theme. A matugen `post_hook` (`shell/matugen/config.toml`) reruns it on every
  palette change, so both follow-wallpaper and fixed-scheme paths retint folders
  with no daemon wiring. Each run writes a freshly-named copy and selects it, so
  running GTK apps (Nautilus, Thunar) load the new colours in one step and
  recolor live -- no reopen, no wrong-colour flash, no stale icon cache.

- **The recorder sidebar gains a Discord toggle: a Quick capture auto-shrinks to
  fit a chat.** With it on, a finished Quick recording is re-encoded to a
  best-effort 10MB or under by a two-pass x264 pass that keeps the native
  resolution and the audio, so the clip drops straight into Discord. It only
  touches Quick captures (Studio stays full quality for the editor), leaves a
  clip already under the limit untouched, retries with a smaller budget if a
  pass overshoots, and replaces the file in place once the smaller result is
  verified. The toggle lives in the sidebar Recording deck with a one-line
  summary; the bitrate split (video budget from duration and target size, audio
  scaled down only when the budget is tight) follows iNiR's compress-discord
  approach (`hyprland/scripts/ryoku-cmd-discord-compress`, `pill/DeckRecord.qml`,
  `pill/Singletons/Recorder.qml`, `pill/RecordHud.qml`, `pill/GlyphIcon.qml`).

- **A dual-edge floating-island bar (`dyad`), ported from Jules3182's dotfiles.**
  Islands ride the top AND bottom screen edges at once: the top carries the 力
  logo (opens the app launcher), the focused window title, a centred day / time
  / date, and
  calendar / clipboard / weather / network / bluetooth / volume / power; the
  bottom carries live CPU / RAM / GPU / net throughput, the occupied-workspace
  grid, and now-playing. Two surfaces via `dyadVariant`: `faithful` (dark
  translucent capsules, the reference) and `ryoku` (grainy paper-black square
  chips, bone ink, Space Grotesk). The frame stays a thin hairline on both edges
  (frame on or off) and both strips reserve their band, so windows tuck under the
  islands instead of behind them; every module taps our own popouts (grown from
  the module's own edge) and reads our own singletons, with SysStats gaining
  kernel-native GPU-busy and network-throughput readings. Selectable from
  Settings' bar-style gallery with its own look toggle (`pill/DyadBar.qml`,
  `pill/shell.qml`, `pill/Singletons/Config.qml`, `pill/Singletons/SysStats.qml`).
- **The Super+Tab overview reads the scrolling layout and takes a right-click to
  enter a workspace.** Each workspace cell now maps its windows against their
  bounding box unioned with the monitor viewport, not the viewport alone: a normal
  (dwindle/master) desktop is unchanged, but a scrolling-layout workspace -- an
  infinite horizontal tape whose off-screen columns sit far past the monitor edge
  -- is shown whole, uniformly scaled into a letterboxed band with the on-screen
  slice framed and a scroll marker, instead of every off-view window crushed onto
  the edge. Right-clicking anywhere on a cell (a window, a gap, or the "+") now
  enters that workspace, so a packed cell is still one click to switch to, while
  left-click keeps focusing the exact window it hits
  (`quickshell/overview/WorkspaceCell.qml`, `quickshell/overview/Overview.qml`).
- **Every bar skin caps the now-playing width, aurora goes niri-clean, and the
  band faces are rebalanced around a real music section.** The music module could grow its title until it
  crossed the centred clock or a neighbouring cluster on the flat (inir / aurora /
  angel), triptych, atoll and delos skins; each now caps its `maxW` to the room
  beside its neighbours (the band skins and washi already did), so a long track
  name elides instead of overlapping. `aurora` drops its layered translucent-glass
  gradient and cream sheen for one flat niri-style tone with a crisp hairline and
  flat borderless modules -- no more weird glassy layering. The reference / native
  band faces (noctalia, caelestia, aegis, stele, triptych) were rebalanced so the
  right cluster no longer overloads and hides the centred clock: the now-playing
  chip and the CPU / RAM / temp stats move to the left beside the workspaces (a
  real music section, matching the flat and nacre skins), the window title elides
  to the room left before the clock, and the right keeps status / weather /
  toggles / tray / power. The delos island re-centres on its docked edge instead
  of cramming into the corner (`pill/Bar.qml`, `pill/BarModule.qml`,
  `pill/AtollBar.qml`, `pill/DelosIsland.qml`, `docs/bar.md`).
- **A power/session popup that warps out of the top-right frame corner.** Super+
  Escape and every bar power button now open one shared card (`qs -c power`)
  instead of a per-skin popout. It is a resident surface, kept warm and hidden,
  so it opens instantly -- like a sidebar -- rather than cold-starting a process
  on every press. It shows the current wallpaper as a live hero (the clip plays
  when the desktop wears one), the logged-in user with uptime, and the session
  actions: Lock and Sleep tap, Logout / Restart / Shutdown are hold-to-confirm, a
  bone plate ramping up under the glyph so one stray click never reboots the box.
  Beta-18 paper and ink with a 力 seal, Space Mono vitals and a scannable
  barcode; the only colour is the wallpaper itself. It scales up from the
  top-right corner its button lives in and collapses back into it. The card and
  its video decoder build only while open, and the still poster shows instantly
  while the clip fades in behind it, so a hidden popup holds no decoder
  (`quickshell/power/`, `ipc/daemon.go` keeps it warm and routes to it,
  `quickshell/pill/shell.qml` points every skin's power trigger at it).

### Changed
- **The GTK / GUI-app palette fan-out is now a toggle, and reaches GTK 3 too.**
  matugen's app templates split into `matugen/config.toml` (the always-on core:
  kitty, Hyprland borders, btop, Qt) and `matugen/apps.toml` (GTK 3 / GTK 4),
  the latter rendered only when "Theme apps" is on (theme.json `themeApps`,
  default on so existing installs keep themed apps); off, the daemon blanks the
  generated `gtk.css` so GTK / libadwaita apps revert to stock Adwaita. The
  shared GTK template now carries the classic GTK 3 `@theme_*` names beside the
  libadwaita ones, so GTK 3 apps follow the palette and not just GTK 4
  (`matugen/config.toml`, `matugen/apps.toml`,
  `matugen/templates/gtk-colors.css`, `ipc/wallpaper.go`).
- **The shell ships cheap on RAM by default: idle surfaces free their whole
  process.** The launcher, workspace overview and the RyoLayer widget board are
  on-demand now (they start on their keybind, not at login), and they unload
  after an idle grace like the visualiser (when silent) and the desktop widgets
  (when covered); all these unloads now default ON. A desktop at rest holds only
  the bar and the daemon, about 330 MB instead of ~900 MB across six resident
  processes. Turn an unload off per surface in Ryoku Settings > Performance to
  keep it warm. RyoLayer with a pinned widget still starts at login to show it
  (`ipc/daemon.go`, `ipc/idlewatch.go`, `ipc/audiowatch.go`, `ipc/widgetwatch.go`).
- **RyoLayer builds its editing slots only while the board is open.** A ryolayer
  kept resident for a pinned widget no longer holds a hidden slot per widget;
  slots read their geometry live off `ryolayer.json`, so a reopen rebuilds them
  in place (`quickshell/ryolayer/Board.qml`, `quickshell/ryolayer/shell.qml`).

### Fixed
- **Screenshots work across two monitors again.** ryoshot stitched a spanning
  capture from per-monitor grabs taken at each monitor's device pixels, so on a
  mixed-scale multi-head setup the HiDPI slice came out oversized and the seam
  landed wrong. Each slice is now grabbed at its logical size onto one logical
  canvas (`lib/coords.js` `stitchPlan`, unit-tested in `lib/coords.test.mjs`),
  and the grab waits for each overlay's screencopy to freeze and retries a
  transient miss instead of aborting the whole shot -- a second monitor that
  froze a few frames late no longer breaks the capture (`ryoshot/shell.qml`,
  `ryoshot/Overlay.qml`).
- **The atoll Thickness control resizes the islands across its whole range, not
  just the top third.** atoll floored its band at the islands' minimum height, so
  the lower half of the Thickness slider (18 to ~34) all mapped to that one floor:
  lowering did nothing, and raising did nothing until the value crossed it. The
  band now maps linearly from the control's minimum, so every step from 18 to 48
  visibly resizes the islands (`pill/Singletons/Config.qml` adds `barBandBase`;
  `pill/shell.qml` uses it for the drawn band and the window reserve).

- **Frame-off bar popouts melt shut cleanly instead of leaving a shrinking bump,
  and open from their module without clipping off the screen.** On a frame-off skin
  (atoll, and the flat iNiR skins) a popout is a standalone floating blob with no
  frame band to melt into. On close it shrank to an opaque nub above the bar -- a
  bump -- because a blob in the shared field cannot fade. The welded skins already
  avoid this with a burial that retracts the blob's inner face one smoothing-depth
  in, so the shape hits zero size before it can strand a metaball fillet; frame-off
  popouts now reuse that same burial (the weld neck stays off, so no bridge across
  the empty band), so they melt shut with no nub while staying a blob.
  Separately, a popout opening from an edge module (battery or network on the
  right) fused flush to the screen edge like a framed skin and clipped its own
  controls off; with no frame wall to fuse into it now stays inset and fully
  on-screen (`quickshell/pill/popouts/Popout.qml`).

- **Wi-Fi login works from the bar on every skin, not just from Ryoku Settings.**
  The bar's network popout was never in the keyboard-focus list, so its inline
  password field could not receive typing -- only washi (Wi-Fi via the link
  surface) and the Hub worked. `network` now takes keyboard focus like the other
  input popouts, so a secured network's password can be typed and the connect
  (`nmcli --ask`, the same backend everywhere) goes through
  (`quickshell/pill/shell.qml`).

- **The atoll volume popout shows how to switch audio devices.** Output and input
  rows already promoted to default on tap, but only a hover cursor hinted it;
  non-default rows now show a bone-outline USE chip on hover (mirroring the
  DEFAULT chip on the active device), so changing the output or the microphone is
  discoverable (`quickshell/pill/popouts/AtollVolumePopout.qml`).

- **Frame-off is flat now: the frame corners square off.** With the frame
  disabled the border already collapsed flush, but the inverted frame rect kept
  its `frameRadius`, leaving four curved frame-surface cut-outs at the screen
  corners. The radius drops to 0 when the frame is off. (Window corners round via
  Hyprland `decoration:rounding`, a separate knob on the Hub's Look page; set it
  to 0 for fully square windows to match.)

- **atoll gained input-device switching, a bar power icon, and a look that
  saves.** The atoll volume popout listed only outputs; it now has an Input
  section (tap a microphone to make it default, its fader sets the level),
  mirroring Output. The atoll bar's left island gained a power icon that opens
  the session popout (lock / sleep / restart / power), which atoll previously
  reached only through the battery popout. And the atoll look picker
  (`atollVariant`) now persists: it was in the Settings schema but missing from
  the Hub's store, so the choice was dropped on save.

- **Sidebar hover-corners are reachable on a multi-monitor shared edge.** A
  left/right sidebar arms only when the pointer reaches the corner where it
  clamps, but on a shared edge between two screens the pointer crosses to the
  neighbour before it can clamp, so that corner never armed. Each overlay now
  detects a flush-adjacent screen and, on that shared axis only, opens the arming
  reach to the whole corner while any axis that still clamps keeps its tight 6px
  reach; the 150ms dwell is unchanged, so a corner is never armed before it is
  reached. Single-monitor behaviour is identical.

- `ryolayer/Singletons/Eq.qml`: toggling the equalizer switch now takes effect.
  `setEnabled` wrote `eq.json` and immediately ran `ryoku-eq apply`, but the
  FileView write was async, so `apply` re-read the file before the new flag
  landed and started/stopped the wrong way; the switch appeared to do nothing.
  The eq.json FileView now sets `blockWrites: true`, so the write completes
  before `apply` runs.
- **Bars no longer show a wrong workspace number (usually "9") on a fresh shell.**
  This Hyprland fork answers the workspace resync socket in a shape quickshell
  0.3.0 cannot parse (see `Singletons/Fullscreen.qml`), so
  `Hyprland.focusedWorkspace` is null on a fresh instance until the first live
  focus event. The delos island fed that null straight to the workspace strip as
  `-1`, and the strip's desktop-group math (`base = floor((id-1)/10)*10`)
  rendered `-1` as "9" (and `0` as "10"), so a bar shown before you first switched
  workspaces displayed a bogus active number until the next switch. The bar skins
  hid it behind their own `hyprctl activeworkspace` seed; the island had none. The
  current workspace id now comes from one `Workspaces` singleton that seeds the
  truth from `hyprctl`, re-seeds while the live focus is still missing, and prefers
  `focusedWorkspace` once it exists, so the seed no longer lives in two places and
  every skin reads the same id. Verified live: on a fresh restart parked on ws 3
  the island and the stele strip both show 3, and both track switches
  (`quickshell/pill/Singletons/Workspaces.qml`, `quickshell/pill/Bar.qml`,
  `quickshell/pill/DelosIsland.qml`).
- **The App Launcher's backdrop blur no longer flashes a frame of frost on open
  at the lowest setting.** With blur set to 0, opening the palette still flashed
  frost for ~one frame: the launcher's layer is frosted by a compositor layer
  rule the instant it maps (using the global blur), and the launcher only forced
  that off a beat later (an async probe, then an eval), so the desktop behind the
  palette blurred briefly before clearing. At 0 the launcher now takes a
  `launcher-noblur` layer namespace the blur rule does not match, so its backdrop
  is never frosted (nothing to flash) and the global blur is left untouched (the
  workspace overview still needs it); blur > 0 keeps the `launcher` namespace and
  the forced strength. The blur layer rule is scoped to the exact `launcher`
  namespace and a separate no-anim rule covers both. Also reset the global blur
  that the old force/restore had drifted to disabled, which had quietly dropped
  the overview's frost. Verified live by burst-capturing the open at blur 0 with
  global blur forced on: the first frame is sharp, where it was fully blurred
  before (`quickshell/launcher/shell.qml`, `hyprland/modules/decoration.lua`).
- **The App Launcher's backdrop blur no longer flickers on close.** The
  write-ordering fix stopped the blur stranding on, but the restore still fired
  the instant the palette began closing, while its window stays mapped and fading
  over the desktop for the whole close morph (Motion.window): the global blur was
  torn down at the first frame of the close, snapping the wallpaper sharp behind
  the still-visible palette. The restore now waits out the morph, so the frost
  holds for the palette's entire exit and drops only once it has unmapped; a
  re-open cancels the pending restore, so a rapid toggle keeps the frost without a
  blink. Verified live by tracing `decoration:blur` against the launcher layer's
  mapped state across a close (`quickshell/launcher/shell.qml`).
- **The first-run welcome fits a 720p screen.** The window rule floats it at
  1180x760 with a 980x640 minimum pinned in QML, so a small (or scaled-down
  low-res) screen cut off the tour's controls. Like the Settings window, it now
  clamps its maximum size to the screen it is on and lets Hyprland centre the
  clamped result in the usable area; roomy screens are unchanged
  (`quickshell/welcome/shell.qml`).

### Added
- **A brightness control in the System (right) sidebar, multi-monitor aware.**
  Below the volume fader, a brightness fader drives the internal backlight
  (`brightnessctl`) plus one fader per external DDC monitor (`ddcutil detect`), so
  a laptop shows one clean fader and a multi-head rig gets a labelled fader each
  (`BrightnessControl.qml`). `Singletons/Devices.qml` now owns the internal
  backlight alongside the external monitors it already tracked, and runs `ddcutil
  detect` when the sidebar opens (nothing called it before, so external brightness
  never populated). Writes debounce so a drag never floods i2c or brightnessctl.

- **A new `washi` bar skin: a floating warping pill, ported from Ricelin.**
  Ryoku forked from Gakuseei's Ricelin long ago, then diverged to the bar + frame
  + delos model; `washi` brings back Ricelin's signature. A small pill rides
  top-centre and warps in place into full surfaces (media, calendar, clipboard,
  mixer, network + bluetooth, power, resources, notifications, workspaces, and a
  wallpaper strip on Ryoku's own switcher), each surface growing out of the body
  on the liquid morph curve as its content cross-fades in, the Ame flame docking
  to each. At rest it shows a glyph, the clock and a breathing flame bead; hover
  expands to workspaces, date and quick-surface icons. It reuses Ryoku's existing
  PillSurfaces, Ame flame and Motion/Theme tokens, routes the existing surface
  keybinds and the `pill` IPC to warp instead of opening an edge popout, and
  reserves a top strip so windows tuck below it. `washiVariant` picks the look:
  `ryoku` (the 力 mark, Space Grotesk, paper-ink) or `ricelin` (faithful: the 時
  kanji and JetBrains Mono). Both selectable in Settings -> Shell -> Bar
  (`quickshell/pill/WashiPill.qml` + the `*Surface.qml` wrappers,
  `Singletons/Config.qml`, `quickshell/pill/shell.qml`).
- **A new `atoll` bar skin: a floating multi-island bar with ilyamiro's popouts,
  ported from ilyamiro's nixos-configuration.** Frame-off, a row of dark rounded
  islands rides the wallpaper and cascades up on startup: search + settings,
  numbered workspace pills with a bone chip sliding behind the active one, a
  now-playing media island, the clock + typewriter date + weather centred, and
  bright status chips (wifi / bluetooth / volume / battery) that invert to bone
  plates when on, plus the tray. Its popouts are faithful ports of ilyamiro's own,
  re-homed on Ryoku's frame-blob surfaces and Motion/Theme tokens: a radial
  network/bluetooth orbit with a wifi|bt toggle, a month grid with an hourly
  weather sun-arc and condition rings, a 10-band EQ music player (wired to
  `ryoku-eq`) with a spinning vinyl and presets, liquid-fill CPU/RAM/temp/disk/net
  cards, a battery ring with brightness/volume faders and session controls, and a
  hero volume orb. Each popout is a transparent content Item swapped in per skin
  by a Loader in the shared popout hosts, so the existing hover/click triggers,
  keybinds and `pill` IPC open them unchanged; settings route to the Hub.
  Selectable in Settings -> Shell -> Bar -> Style, with Ricelin (Gakuseei) and
  ilyamiro credited on the Hub credits page. Verified live: the bar and its
  popouts render on the running compositor (`quickshell/pill/AtollBar.qml`,
  `quickshell/pill/AtollWorkspaces.qml`, `quickshell/pill/AtollStatus.qml`,
  `quickshell/pill/popouts/Atoll*Popout.qml`, `Singletons/Config.qml`,
  `Singletons/Silhouette.qml`, `quickshell/pill/shell.qml`, `quickshell/pill/Bar.qml`,
  `../hub/quickshell/schema/ShellSettingsPage.js`, `../hub/quickshell/pages/CreditsPage.qml`).
- **The band skins get a reorderable modular layout (iNiR's modular bar,
  ported).** On noctalia / caelestia / aegis / stele the left, centre and right
  clusters are data-driven from `barLayoutLeft` / `barLayoutCentre` /
  `barLayoutRight` (ordered module-id lists), edited from Settings -> Shell ->
  Bar -> Layout, so a module reorders, drops, or moves between clusters. It is
  opt-in and zero-regression: an empty zone keeps the classic hardcoded
  arrangement untouched; customise a zone and `BarModularFace` renders that
  skin's own module treatment from the data. The bespoke skins (triptych, nacre,
  the flat iNiR set, delos) keep their designed layouts. Verified live reordering
  on stele and caelestia and confirming the empty default is byte-identical to
  before (`quickshell/pill/BarModularFace.qml`, `quickshell/pill/Bar.qml`,
  `quickshell/pill/Singletons/Config.qml`, `../hub/quickshell/schema/ShellSettingsPage.js`).
- **The bar carries weather, quick-toggles and a special-workspace cue, and the
  user adds or removes what it shows.** A weather module (`BarWeather`, condition
  glyph + temperature off the `Weather` singleton) opens a compact `WeatherPopout`
  (current reading, hourly strip, daily forecast) from the bar edge, with a
  `weather` IPC for keybinds. A placeable quick-toggle module (`BarToggle` /
  `BarToggles`) carries wifi / bluetooth / mic / do-not-disturb / caffeine /
  night-light switches, accent-lit while on; their state and actions live in a new
  shared `Toggles` singleton that also backs the System deck's control tiles, so
  the wifi/mic/night probes and toggle logic exist once, not twice (`DeckControls`
  was refactored onto it). A special-workspace cue (`BarSpecialWs`) names an active
  Hyprland scratchpad and clears when it closes, tracking the `activespecial`
  event. All three, plus the existing title/media/status modules, toggle from
  Ryoku Settings -> Shell -> Bar -> Content (`barShowWeather`, `barShowSpecialWs`,
  a `barToggles` multi-select). Present on every skin, verified live
  (`quickshell/pill/Bar*.qml`, `quickshell/pill/Singletons/Toggles.qml`,
  `quickshell/pill/popouts/WeatherPopout.qml`, `../hub/quickshell/schema/ShellSettingsPage.js`).
- **The flat iNiR skins carry iNiR's per-module character, recoloured to
  bone-and-ink.** `angel` finally has its signature: modules are raised keys with
  a hard accent offset shadow (the iNiR "escalonado" -- no blur, deepening on
  hover) the shell's `shadowHard`/`shadowOffset` tokens described but the bar never
  used; `inir` is a clean flush TUI status-line, `aurora` refined glass. The
  treatments live on `BarModule` so no skin re-rolls them (`quickshell/pill/BarModule.qml`).
- **The delos island is a contextual dynamic island (ActivSpot port).** Its face
  follows the live context -- now-playing (art + title + transport), a
  screen-recording tally (pulsing dot + timer), or a Discord voice call (glyph +
  timer) -- and spring-morphs to fit, falling back to the clock/modules when idle.
  A second active context rides beside it as a minibubble (a satellite blob melding
  into the same field), and a timer rotates which holds the island's face. Verified
  live morphing music <-> idle and the music+recording dual-bubble
  (`quickshell/pill/DelosIsland.qml`).
- **matugen fans the palette across the whole app suite.** A scheme apply (or a
  wallpaper change in follow mode) now renders GTK (libadwaita), Qt (qt6ct), and
  btop from the same colours as kitty and the Hyprland borders, through matugen
  as the shared templating engine. The control plane authors `colors.json` for
  the fixed schemes and wallust still extracts it when following the wallpaper;
  matugen then templates every external app from that one palette, so the
  grainy-mono default and any rice reach past the shell into the apps. GTK apps
  get a colour-scheme nudge to re-read. New `matugen/config.toml` and
  `matugen/templates/` (kitty, hypr, GTK, Qt, btop), shipped as a hard `matugen`
  depend beside wallust; btop gains a shipped `apps/btop/btop.conf` and
  `qt6ct.conf` points at the generated scheme (`ipc/wallpaper.go`,
  `../hub/backend/schemes.go`).
- **Launcher: an "@" prefix tunes a live lofi radio.** `@` lists the stations
  -- Lofi Girl and Chillhop Radio (YouTube 24/7 streams), SomaFM Groove Salad
  and Fluid (plain Icecast) -- `@lofi` puts Lofi Girl on air, `@stop` (or the
  row's Stop) tunes out. The engine (`ryoku-cmd-radio`) resolves a channel's
  /live page with yt-dlp at play time -- pinned video ids rot whenever the
  stream restarts -- pulling the stream's own thumbnail in the same call, so
  the card wears the broadcast's real artwork (direct stations carry their
  published covers; no iTunes guessing for a radio). When YouTube or yt-dlp is
  having a day, each YouTube station falls back to its paired Icecast one,
  saying so in the row and on the card.
  A detached supervisor re-resolves and reconnects when the stream drops (the
  resolved URLs expire after a few hours), so the radio survives everything
  except an explicit stop. On air, the now-playing surfaces wear a broadcast
  coat instead of the track dress: a pulsing ON AIR tally and station plate on
  the launcher card, "● LIVE / 24/7" where elapsed/total would be, and no
  seekbar anywhere -- a broadcast has no position, so the pill popout and
  sidebar retire their scrubbers too (the cava wave carries the motion). When
  other music starts, the radio doesn't fight it: a collision watcher sets it
  aside (live wallpapers, which are mpv on the players bus too, never count as
  music) and a slim parked-radio chip keeps it one tap from returning --
  RESUME restarts the stream, × lets it go. Playback is mpv + mpv-mpris (new
  `ryoku-desktop` deps with yt-dlp), so the radio is an ordinary MPRIS player
  everywhere else. New `lib/radio.js` (+ tests), `Singletons/Radio.qml`,
  `providers/radio/RadioTuner.qml`, `RadioAside.qml`; engine contract pinned
  by `tests/radio-engine.sh`.
- **Low Power mode: one Ryoku Settings toggle strips every heavy effect so a weak
  GPU runs the shell without lag.** New Performance page switches, all off by
  default and written to `~/.config/ryoku/performance.json`: **Low power mode**
  (the master -- implies all four below), **Reduce motion** (collapses every
  `Motion` duration to 0 so transitions cut instantly and stop forcing per-frame
  repaints), **Disable blur**, and **Disable shadows**. The blur/shadow switches
  cut both the shell's own passes AND the compositor's: `hyprland/modules/
  decoration.lua` now reads the same flags and gates `blur`/`shadow` (plus the
  launcher/overview backdrop-blur layer rules), and the toggle fires `hyprctl
  reload` so it applies at once; on login the Lua reads it on first parse. The
  master and each switch are OR'd in a derived `Performance` singleton, so
  consumers read `blurDisabled` / `shadowsDisabled` / `motionReduced` /
  `pillFrozen`, never the raw flag. Wired through: pill bead
  (`pill/Ame.qml`, `pill/Singletons/Performance.qml` + `Motion.qml`), the
  now-playing art blur (`launcher/NowPlaying.qml` + new
  `launcher/Singletons/Performance.qml`, `launcher/Singletons/Motion.qml`), the
  visualiser bloom + freeze (`visualizer/Visualizer.qml`,
  `visualizer/Singletons/Performance.qml`), the desktop-widget drop shadows (new
  `widgets/Singletons/Performance.qml`, `widgets/WidgetSlot.qml`,
  `widgets/WidgetMenu.qml`), the overview motion (`overview/Singletons/Motion.qml`),
  the compositor (`hyprland/modules/decoration.lua`), and the Hub Performance page
  (`hub/quickshell/PerformancePage.qml`).
- **Optional shell components can be turned off entirely.** A `"disabledComponents"`
  array in `~/.config/ryoku/performance.json` (any of `launcher`, `visualizer`,
  `widgets`, `overview`; `pill` is always on) stops the daemon from ever starting
  those processes, at boot or on a keybind -- the Ryoku answer to "disable what
  you don't use" for someone who never opens a surface, so it costs zero rather
  than parking and respawning. A missing key disables nothing (`ipc/daemon.go`
  `componentDisabled`/`parseDisabledComponents`).
- **The launcher and workspace overview can free their memory when idle, opt-in.**
  Both stay resident-and-hidden for an instant open -- a full Qt/jemalloc process
  (~300 MB RSS, ~110-160 MB PSS) each, drawing nothing. New Performance toggles
  let the daemon SIGTERM a palette after 60 s hidden and respawn it on the next
  open; the palettes report their open state over a new `state` IPC and the
  existing `ipcCall` retry covers the cold start, so the open still lands. Off by
  default, mirroring the visualiser and widget unload gates (`ipc/idlewatch.go`,
  `ipc/daemon.go`, `quickshell/launcher/shell.qml`, `quickshell/overview/shell.qml`,
  Hub `PerformancePage.qml`; keys `unloadLauncherWhenIdle`, `unloadOverviewWhenIdle`).
- **Supervised Quickshell processes launch with jemalloc tuned to hoard less idle
  memory.** Quickshell links jemalloc, whose default `narenas` is 4x the CPU count
  (64 on a 16-thread box) and which only returns freed pages to the OS on later
  allocation activity, so an idle shell keeps every dirty page mapped. The daemon
  now hands each `qs` it starts
  `MALLOC_CONF=narenas:2,background_thread:true,dirty_decay_ms:5000,muzzy_decay_ms:5000`
  (a user-set `MALLOC_CONF` still wins), capping arenas and running a background
  thread that purges freed pages on a short decay even while idle. Bounds RSS
  growth over a long session more than it cuts the fresh-idle figure
  (`ipc/daemon.go` `qsEnv`/`jemallocConf`).
- **Three flat frame-off bar styles ported from snowarch's iNiR: `inir`,
  `aurora`, `angel`.** Each is a flush, full-width bar at the screen edge with
  borderless modules (seal, workspaces, stats and now-playing left; the clock
  centred; status and tray right), painting its own surface instead of riding
  the frame band, and no lobes. `inir` is a solid TUI panel with hairline cell
  separators; `aurora` is translucent glass the wallpaper shows through with a
  soft top sheen (translucency, not a gaussian frost: a Wayland layer cannot
  blur its live backdrop); `angel` is an opaque brutalist panel with a heavy
  base border and a bright inset top edge. Meant for the frame off; pick them in
  Ryoku Settings, Shell (`Bar.qml`, `shell.qml`, `Singletons/Config.qml`,
  `popouts/Popout.qml`, and the Hub `ShellSettingsPage.qml`).
- **The desktop mark and name are now user-overridable across the shell.** Every
  力 seal in the chrome (the bar, launcher, overview, the pill deck and popouts,
  desktop widgets and the calendar, the wallpaper switcher, ryoshot's watermark,
  and the welcome tour) renders through a shared `BrandMark` that reads a new
  `~/.config/ryoku/brand.json`: a short text/glyph mark (default 力), or a custom
  SVG/PNG logo (tinted to the accent via `ColorOverlay`, or shown as-is), plus the
  desktop name ("Ryoku" by default) in the welcome copy. Ryoku's own apps (the
  Hub, ryo* apps) keep their brand and ignore it. Edit it in Ryoku Settings,
  Shell, Global (`BrandMark.qml` per app, each `Singletons/Config.qml` and
  `Theme.qml`, and the rewired seals across
  pill/launcher/overview/widgets/wallpaper/welcome/ryoshot).
- **The recorder island is now the single entry point for capture.** The 力
  deck's Record button no longer drops a mode menu; it opens the floating
  island in a pre-record chooser. The capture toggles (screen or region,
  desktop audio, microphone) moved into that chooser, next to three actions.
  **Quick** and **Studio** both capture with gpu-screen-recorder (clean on
  Wayland, no screen-cast portal double-prompt); **Edit** opens **ryomotion**
  (the OpenScreen-based editor, its own package) to import a clip. Quick morphs
  the island into the live control bar and saves a plain recording; Studio also
  samples the cursor, writes ryomotion's `<clip>.cursor.json` sidecar, and opens
  the clip in the editor on stop, so its automatic cursor-follow zoom still works
  (`RecordHud.qml`, `DeckRecord.qml`, `Singletons/Recorder.qml`,
  `ryoku-cmd-screenrecord`, `ryoku-cmd-studiorecord`, `shell.qml`).
- **Region recording shows a live boundary and works for Studio too.** Picking a
  region now dims everything outside it while you record -- a click-through
  overlay (`RegionOverlay.qml`) that stays up for the whole capture, so you can
  keep moving windows around the box while only the region records. Both Quick
  and Studio honour it (Studio used to always catch the full screen), and Studio
  re-bases its cursor sidecar to the region so auto-zoom maps inside it. The box
  the picker drew lives on `Recorder.regionGeom`, shared by the overlay and the
  recorder, and gsr crops to exactly that box (`RegionOverlay.qml`,
  `Singletons/Recorder.qml`, `RecordHud.qml`, `shell.qml`, `ryoku-cmd-studiorecord`).
- **`ryoku-shell system` toggles the right (System) sidebar**, so it can be
  bound like the left `toolkit`/`sidebarLeft`. `Super+Alt+D` uses it; the daemon
  maps the verb to the pill's existing `sidebarRight` handler (`ipc/daemon.go`).
- **The desktop visualiser gains four new looks: a stiff line, LED segments, a
  radial ring and a morphing circle, plus peak caps and a frame budget you
  control.** Beside bars, dots and the filled wave the spectrum now draws as a
  **line** (a stiff angular readout with its own glow that snaps where the wave
  flows), **segments** (a lit LED stack per band), a **radial** ring of bars
  around a pulsing centre, and a **circle** (a closed blob whose radius breathes
  with the music). New per-look controls: render **fps** (30 by default, up to
  60, with cava sampled at the same rate so nothing is computed twice),
  **smoothing** and **sensitivity**, falling **peak caps**, and the **segments**
  count, all live from `visualizer.json` (`Visualizer.qml`,
  `Singletons/Config.qml`, `Singletons/Spectrum.qml`, `shell.qml`).
- **An adaptive governor keeps the visualiser cheap under load.** When the render
  timer slips behind for a sustained stretch (a busy machine or a heavy pick),
  the engine steps down in tiers: it lowers the frame rate, drops the bloom
  buffer and reflection, sheds the peak cache, and folds segments back to bars,
  then climbs back when headroom returns. It throttles the cost, never the
  spectrum, so the desktop keeps drawing (`Visualizer.qml`).
- **Ryoshot gains a Beautify editor (the 力 button), sharing-first.** After a
  capture, the toolbar's 力 button opens a full editor with a frosted, textured
  chrome (the launcher's grainy look): a live canvas beside a grouped panel that
  uses the right control for each job. Background is a thumbnail grid of gradient
  presets plus Solid / custom Gradient (two colours + angle) / Image / None
  (transparent); Frame has padding, roundness and a coloured border; Shadow has
  strength, blur, distance and a direction dial; plus Adjust (brightness /
  contrast / saturation) and a Ratio row with social formats (X, Instagram,
  Story, LinkedIn, YouTube, Pinterest) and an optional 力 + `user@RyokuArch`
  watermark. Copy or Save
  through the same path as a plain shot, so it matches the desktop and needs no
  new dependency (`Beautify.qml`, `Slider.qml`, `Dial.qml`, `Toolbar.qml`,
  `shell.qml`).
- **Beautify presets with a savable default.** A PRESETS row offers one-tap
  looks (Ember, Ocean, Sunset, Mono, Paper, Bare). *★ Set as default* writes the
  current look to `~/.config/ryoku/ryoshot-beautify.json`, so every later capture
  opens already styled without re-tuning; with a default set, the annotate toolbar's
  Copy and Save bake it in and export directly, no editor needed. *Reset* strips to a
  plain image, and the shadow casts the way the dial points (`Beautify.qml`, `shell.qml`).
- **Ryoshot annotation gains a magnifier, counters, pixelate redaction and a
  hand-drawn style.** The toolbar adds a **magnifier** loupe (drag a circle that
  zooms the shot beneath it, ringed), a numbered **counter** stamp (click to drop
  1, 2, 3… badges that scale with the pen width), a **pixelate** region for
  redaction (a mosaic over the frozen shot, beside the existing blur), and a
  **sketch** toggle that renders rectangles, ellipses, lines and arrows with a
  smooth hand-drawn stroke (`Toolbar.qml`, `Icon.qml`, `AnnLayer.qml`,
  `Overlay.qml`, `shell.qml`).
- **Beautify HD ×2 export (opt-in AI upscale).** A SHARE toggle runs the export
  through the GPU (`waifu2x`, no denoise so text stays crisp) to double the
  resolution before Copy/Save. Off by default and saved with your default, so it
  can bake in automatically; it skips already-large shots and falls back to the
  plain grab when the tool is absent (`Beautify.qml`).

### Fixed
- **The App Launcher's backdrop blur no longer strands the compositor blur
  forced-on after the palette closes, and stops flickering on the way out.** The
  launcher drives the global Hyprland blur (a single knob, no per-layer size) to
  the App Launcher page's strength while open and restores the prior blur on
  hide. The force (open) and the restore (close) were two independent
  fire-and-forget `hyprctl eval` calls with no ordering guarantee, so on a slower
  or loaded compositor the restore could reach it before the force and leave blur
  stranded on after close (the reorder itself reads as a flicker); a fast desktop
  always applied them in order, which hid it here. The async baseline probe could
  also read a mid-transition value and make a wrong blur sticky. Every write now
  serializes through one channel, newest request winning in issue order, so a
  rapid open/close settles to its final state cleanly; the baseline is read only
  from a drained compositor and only when well-formed, so a mid-flight or failed
  read can never become the restored value (`quickshell/launcher/shell.qml`).
- **Upgrading with a live wallpaper active no longer strands the old video
  player over every static set.** Releases through beta 16 played live
  wallpapers with mpvpaper (phonto in the interim GPU-picked era), spawned
  detached so it survives its daemon; the livewall-era `stopLive` only pkills
  `ryoku-livewall`, so an update left the old player's background surface
  stacked above awww's forever: every static apply succeeded in awww but
  painted invisibly under the looping clip ("the wallpaper won't change",
  Super+W included), while live picks kept working because livewall's newer
  surface maps above. The daemon now reaps the legacy backends by name where
  it takes ownership of the wallpaper stack -- once at bootstrap, before the
  init apply whose early returns would otherwise skip every kill path -- and
  `ryoku update`'s quiesce does the same. Deliberately NOT on every wallpaper
  change: livewall is single-output today, so a user may run mpvpaper on a
  second monitor on purpose (`ipc/wallpaper.go`, `ipc/daemon.go`, covered by
  `TestWallInitReapsLegacyBackends`).
- **Live wallpapers crossfade in and out like the rest of the wallpaper
  stack.** The `wallpaper-crossfade` layer rule never matched livewall's
  namespace, so a live wallpaper scale-popped in (the global `popin` layers
  animation) instead of fading over the still
  (`hyprland/modules/decoration.lua`).
- **`ryoku-shell lock` blocks until the compositor confirms the lock, so
  suspend can no longer race the lockscreen.** hypridle's `before_sleep_cmd`
  holds logind's sleep delay-inhibitor only while the command runs, but
  `lockSession` fire-and-forgot `lock.sh` and returned in milliseconds:
  Quickshell was still loading QML when the machine suspended, and opening the
  lid showed the desktop for a beat before the lock painted. The daemon now
  waits (bounded at 3s, under logind's 5s `InhibitDelayMaxSec`) for the
  `$XDG_RUNTIME_DIR/qylock.locked` marker qylock touches once
  `WlSessionLock.secure` flips true, clearing a stale marker from a killed
  locker first; a qylock predating the marker just rides out the wait, so
  suspend is delayed, never blocked (`ipc/actions.go`, covered by
  `TestLockSessionWaitsForMarker` and `TestLockSessionClearsStaleMarker`).
- `ipc/actions.go`: the lock no longer leaks one zombie per lock/unlock cycle.
  `lockSession` released the `lock.sh` process handle instead of reaping it, so
  every unlock left a defunct entry in the daemon's process table for the rest
  of the session.
- `ipc/wallpaper.go`: two rapid sets of the same clip no longer corrupt its
  transcode cache. Both encodes shared one `<out>.tmp.mp4` (the generation
  guard drops the livewall launch, not the encode), interleaving two ffmpeg
  writers into a garbage file that then got renamed into the cache and played
  black. The tmp name is now pid+time unique.
- **The now-playing elapsed line no longer pixelates near the end of a track.**
  The wavy seekbar sampled the filled portion with a fixed 48-point budget while
  the number of waves grew with the fill, so past the midpoint each crest got
  fewer than ~5 points and near the end only ~3: the sine aliased into a jagged,
  pixelated polyline (~113 degrees between samples). Resolution now scales with
  the wave count (~16 points per crest, a constant ~22.5 degrees per step at any
  fill), so the line stays smooth from 0 to 100% (`launcher/NowPlaying.qml`).
- **The frame and bar now retract over a fullscreen window on every monitor, not
  just the focused one.** quickshell 0.3.0 learns a workspace went fullscreen two
  ways: the raw `fullscreen` event, which only marks the focused workspace, and a
  `j/workspaces` resync fired right after to catch the rest (a second monitor, a
  fullscreen window dragged between workspaces). Ryoku's Hyprland fork answers that
  request socket in a shape quickshell cannot parse, so the resync did nothing and
  only the focused monitor's shell ever hid. A single panel never noticed (focused
  and active workspace coincide); on a second monitor the frame and top bar stayed
  drawn over the fullscreen content. A new `Singletons/Fullscreen.qml` probes
  `hyprctl -j workspaces` itself on the fullscreen and workspace events and keys
  the result by workspace id; the pill and the OSD read their monitor's active
  workspace from it instead of the resync-derived property (`shell.qml`,
  `OsdWindow.qml`, `Singletons/Fullscreen.qml`, `Singletons/qmldir`).
- **Live (video) wallpapers now play through `ryoku-livewall`, a tiny
  software-decode daemon that holds around 40 MB on any GPU instead of the old
  300-700 MB.** The previous backends were client GL pipelines (`mpvpaper` on
  NVIDIA, `phonto` on AMD/Intel) that cost 300-700 MB RSS; mpvpaper leaked per
  loop (needing a restart watcher), and neither could reach the hard <100 MB
  target on NVIDIA, where the CUDA/GL userspace floor alone exceeds it. Both are
  gone. `ryoku-livewall` (a small C daemon) maps no GPU or EGL driver: it
  software-decodes a downscaled clip on the CPU into `wl_shm` buffers on a
  `wlr-layer-shell` background surface and lets `wp_viewport` upscale to the
  output. Its PSS tracks the panel (~40 MB at 720p, ~57 MB at 2048, ~78 MB at
  2560px) and stays flat across loops with no leak, at ~8% of one core. The
  shell transcodes each clip once to a cached H.264 at the monitor's logical
  width (physical / fractional scale, capped at 2560 for RAM) so the moving
  video renders near 1:1 with the panel instead of upscaled from a fixed 1280
  to a blur, then launches livewall; `awww` (the
  image daemon, unchanged) paints the clip's still frame during that one-time
  transcode and stays under the video. Both wallpaper classes are now under the
  100 MB requirement (static via awww ~31 MB, live via livewall ~40-78 MB by
  panel size). Hardware
  video decode was investigated and rejected: it cannot beat 100 MB on NVIDIA.
  The ryowalls Fit knob now drives the mapping: fill crops the clip to the output
  aspect through the `wp_viewport` source rect (cover), fit letterboxes it whole
  against the shm's opaque-black fill, so a clip never stretches to the panel
  aspect. Single-output for now (multi-monitor is the remaining follow-up)
  (`ipc/wallpaper.go`, `ipc/daemon.go`, `livewall/livewall.c`,
  `livewall/build.sh`, `deploy.sh`).
- **Live (video) wallpapers stop eating RAM and no longer bleed through onto the
  image beneath, with a GPU-picked hardware decoder.** The old path ran a full
  mpv (mpvpaper) that mapped its surface *over* the still `awww` image without
  stopping it, so any letterbox, gap, or launch delay showed the old image
  through, and its documented per-loop leak grew to hundreds of MB. Now `awww`
  paints the clip's own first frame and stays under the video, so the desktop
  always shows the clip's content (the opaque video covers it; anything it does
  not is the clip's own still, never a stale image), and a later switch to an
  image transitions from that real frame, not the pre-video image awww's cache
  would otherwise restore. The decoder is GPU-picked: `phonto` (lean
  GStreamer/VAAPI) on AMD/Intel, `mpvpaper` (mpv `hwdec`=NVDEC) on NVIDIA, where
  VAAPI is unavailable and phonto would fall back to a heavy software decode. mpv
  runs `no-config load-scripts=no` (no mpris hijack), and a watcher relaunches it
  if its RSS crosses a ceiling, to bound the leak. The fit knob maps to `phonto
  --scale` / mpv `panscan`; a missing backend degrades to a still frame through
  `awww` (`ipc/wallpaper.go`).
- **Switching between an image and a live wallpaper now crossfades instead of
  flashing.** The wallpaper rides Hyprland's background layer, where the image
  (`awww`) and video (`mpvpaper`/`phonto`) daemons each map a surface, so a switch
  maps one over the other. The global `layers` animation is `popin 90%`, which
  scale-pops a fullscreen wallpaper surface in and reads as a flicker on every
  image<->live change; a per-namespace `fade` layer rule crossfades the wallpaper
  surfaces instead (the video fades in over the image, and out to reveal it).
  `awww` also fades onto the clip's first frame rather than hard-cutting to it, so
  image->live is one crossfade into the video, matching the Super+C switcher's own
  fade (`hyprland/modules/decoration.lua`, `ipc/wallpaper.go`).
- **The wallpaper switcher no longer sits resident, and a live preview replaces
  the thumbnail instead of playing on top of it.** The Super+C picker was a
  supervised Quickshell surface kept running hidden (~100 MB of scene graph and GL
  context for a picker opened occasionally); it is a one-shot modal like ryoshot
  now, spawned on demand under `flock` and quitting on close, so it holds no
  memory while idle. In the grid, a picked live tile fades its still thumbnail out
  once the clip is actually presenting, so the preview no longer reads as a video
  pasted over a photo (`ipc/daemon.go`, `quickshell/wallpaper/shell.qml`,
  `WallCell.qml`, `VideoPreview.qml`).
- **Setting a wallpaper with no wallpaper daemon installed says so instead of
  hanging and going silent.** With neither `awww` nor `swww` on the box (an
  AUR build that never landed), every image apply ground through ~15s of
  retrying a daemon that does not exist and then returned as if it had worked.
  The daemon start now fails fast when the binary is absent and the apply
  returns a real error naming the fix (`install awww`, or `ryoku doctor` heals
  it), which the shell surfaces like any other wallpaper failure
  (`ipc/wallpaper.go`).
- **The sidebars now close as fast as they open.** A keybind or click dismiss
  sat through the 300ms hover-intent grace before it began melting, so `Super+D`
  (and the right sidebar) snapped open but felt laggy to shut. The grace now
  applies only to a hover-leave -- its real job, debouncing a graze across the
  blob rim -- while a deliberate unpin melts at once, even under the pointer
  (`quickshell/pill/popouts/Popout.qml`).
- **Live (video) wallpapers stop freezing and play smooth and native.** Three
  bugs made a live wallpaper set, then freeze on the first frame, then stutter
  when it did move. (1) The daemon paused mpvpaper whenever the active workspace
  held any window (`pauseWhenCovered` plus the `desktopVisible` "a window covers
  it" heuristic borrowed from the widget layer), so on a single-monitor desktop
  opening one window froze the wallpaper even though it shows through gaps and
  around windows. A live wallpaper is meant to move, so it plays continuously
  now: the covered-pause path (`livePauseReconcile`, the `pause-sync` mode, the
  hyprwatch hook, and the ryowalls "Pause when covered" toggle) is gone. (2)
  Playback ran with `video-sync=display-resample`, which needs the panel refresh
  to pace frames, but mpvpaper's libmpv render path never reports one, so the
  resampler ran blind and juddered; it paces to the clip's own rate now. (3) 4K
  clips dropped frames downscaling with the default scalers, so `profile=fast`
  uses cheap scalers (invisible on a background) while hwdec keeps decode on the
  GPU, so playback stays native and smooth (`ipc/wallpaper.go`,
  `ipc/hyprwatch.go`, ryowalls `SettingsPanel.qml`, `Singletons/Wallhaven.qml`).
- **Styles with a fixed baseline no longer freeze on screen when the music
  stops.** With the idle wave off, the circle and the radial centre ring kept a
  constant radius that never shrank to nothing, so the render ticker halted and
  left a static ring behind (bars, which collapse, cleared fine). The whole
  field now fades out with the "playing" signal, its floor meeting the ticker's
  stop threshold, so every style vanishes on silence (`Visualizer.qml`).
- **The visualiser's analyser no longer runs faster than it draws.** cava was
  pinned at 60fps while the render capped near 30, sampling twice for every frame
  shown; it now follows the configured frame rate (default 30), roughly halving
  its steady-state cost (`Singletons/Spectrum.qml`).
- **The left screen edge is clickable again (no dead strip while browsing).** The
  left sidebar's drag-to-stash trigger masked a band the full hover-corner width
  (`sidebarCornerSize`, ~54px scaled) down the entire left edge, but windows only
  inset `gaps_out` (18px), so roughly 36px of every window's left edge silently
  swallowed clicks for its whole height (a browser's back button, scrollbar, and
  first tab sit under it). The band is now a thin sliver kept inside the frame gap:
  it still opens the stash when a file is flung at the left edge, but never covers
  window content (`pill/shell.qml`).
- **`ryoku deploy` preserves every user file now, matching a packaged update.**
  The dev deploy rebuilt `~/.config/hypr` from the repo and carried across only
  seven named files, so any other user-owned file (an extra `.lua`, a custom
  `modules/*.lua`) was silently dropped, while a packaged `ryoku materialize`
  keeps every file the release does not ship. `deploy.sh` now mirrors it: it
  carries across anything the freshly-staged repo tree does not contain
  (`user.lua`, `monitors_user.lua`, `settings.lua`, `theme.lua`, and anything
  else the user added) and keeps the live copy of the per-machine seeds
  (`monitors.lua`, `gpu.lua`, `keyboard.lua`) over the shipped default, so a dev
  box and an installed box preserve the same set.

### Changed
- **The desktop calendar grew up: edit, time ranges, and navigation that comes
  home.** Clicking an event now reopens it in the add field for editing (Enter
  replaces it, Esc cancels), a leading `9:30-10:30` range is parsed into start
  and end times and shown on the row (the pill's calendar displays it too), and
  every face navigates: month and heat page by month, week by week, agenda by
  seven-day page, each with an accent TODAY chip that appears once you leave
  today and snaps the view (and selection) back. The viewed month derives from
  today plus an offset, so the old one-way navigation that froze the widget in a
  past month is gone and the view self-heals at rollover. Week pre-selects today
  and no longer deselects on a re-click; empty days say "Nothing on this day"
  over the add field; the delete tick arms on the first tap and removes on a
  second within two seconds, on a 24px hit area (chevrons grew to 26px). Under
  the hood: heat cells count events without sorting, the today marks derive from
  a once-a-day key instead of the 1s tick, the calendar slot releases its
  keyboard grab on teardown, and the right-click menu gained an Accent row.

### Removed
- **RyoTunes / YouTube Music is gone from the launcher.** The `@` YouTube Music
  search provider, the mpv radio engine (`Singletons/Radio.qml`), saved playlists
  (`Singletons/Playlists.qml` + the `SavedPlaylists` strip), the pasted
  YouTube-link "play" row, and the MPRIS "YT Radio" seed verb are all removed. The
  now-playing card and the other-sources strip stay, now driven by a slim
  `Players` singleton (the proxy-drop/dedupe helper extracted from the old
  engine), so controlling Spotify, a browser tab or any MPRIS player still works.
  Drops the `mpv-mpris` package, which existed only to expose the YT Music stream.

### Added
- **A first-run welcome walkthrough greets new users once.** On the first login,
  a floating `welcome` surface (`quickshell/welcome`, run as `qs -c welcome`) opens
  over fal.ai-generated Greek-noir threshold art: a five-step guided tour that
  introduces the essential keybinds, names each desktop surface and how to summon
  it, and offers a few genuinely-wired quick settings: shuffle the wallpaper via
  `ryoku-shell`; set the bar position, the bar skin, and the shell-frame corner,
  merged into `shell.json` with a key-preserving write so the running shell retunes
  with no reload; and the window-corner rounding, round-tripped through the Hub's
  own `ryoku-hub hypr` appearance path. The Hyprland
  autostart launches it once, guarded by an flock and a
  `~/.local/state/ryoku/welcome-seen` flag written after the window closes, so the
  tour shows exactly once; a `float-ryoku-welcome` window rule floats and centres
  it. The backdrop is generated at dev time and committed
  (`welcome/art/welcome-bg.png`), so the running target keeps no generation
  dependency.
- **Live wallpapers take motion settings and play smoother.** When it launches
  mpvpaper, the wallpaper daemon reads a max-fps cap and a fill/fit choice from
  `ryowalls.json`, and a new `wallpaper live-reload` mode relaunches the current
  clip with fresh options when ryowalls changes them (no state write, no
  retheme). mpvpaper now also runs with `video-sync=display-resample`, so frames
  are paced to the panel refresh instead of juddering; a sub-60 fps target caps
  decode and paint for battery while 60 (the default) plays the clip at its own
  native rate (`ipc/wallpaper.go`).
- Active quick-toggle icons stay legible on a light wallpaper accent: the icon on
  a lit tile flips to dark ink on the accent fill instead of washing out in
  warm-white.
- **Weather takes a set location and unit.** Beyond auto-locating by IP, the
  shell reads a `weatherLocation` (a city name, geocoded via Open-Meteo; blank
  keeps the IP auto-locate) and a `weatherUnit` ("auto" follows the locale, else
  celsius / fahrenheit), both set from Ryoku Settings' Global tab. A unit change
  re-fetches, so the reading, not just the degree symbol, is right.
- **Two corner-hover sidebars, replacing the control deck.** Full-height panels
  that melt out of the left and right frame edges and fuse into the top and bottom
  frame, so a whole side swells open with no gap. Push the cursor into a top corner to open (a
  short intent, a grace on leave), or toggle via IPC. **Left is Features** (the
  Stash file board, room for more); **right is System**, the control centre folded
  in from the old deck: 力 clock and weather, the session and quick toggles, the
  screen-capture tools and a clipboard button, a volume fader, and a tab rail over
  the notification digest, the month calendar, now-playing, the weather forecast,
  and screen recording. Ryoku Settings' Shell section gains a **Sidebar** tab
  (enable each side, pick and order each side's panes, open-on-hover vs click,
  width, corner-hotspot size). Built on a new `Popout` `fullSpan` mode; the old
  `DeckSurface` / `DeckPopout` control deck is gone.
- **Drag a file to the left edge to stash it.** The left (Features) sidebar now
  springs open when a file is dragged onto the left frame edge or corner -- a
  masked `DropArea` band `sidebarCornerSize` deep, since a HoverHandler can't
  fire while a drag holds the pointer -- so its Stash board is right there to
  drop onto (the board copies the file in and the sidebar tucks away when the
  drag ends; a drop on the edge itself stashes too).
- **A global `roundness` knob and a Global settings tab.** One shell-wide inner
  corner radius (Ryoku Settings' new **Global** tab, alongside the frame melt,
  surface, shadow, and typography controls moved there) rounds every tile, card,
  row and chip so the shell shares one shape with the rounded frame. 0 restores
  the old brutalist sharp corners.
- **A single floating-island bar, `delos`.** `barStyle` takes `delos`: the whole
  bar becomes one draggable island in the frame's blob field, the recorder
  island generalised. Grab its 6-dot grip and it pulls off the edge (the rest
  of the island stays interactive, so the modules keep their taps and wheels);
  near another edge it and a frame bump reach for each other and merge like two
  drops; let go and it drifts to the nearest edge; on a side edge it turns
  vertical (its modules restacking into a narrow strip); tap the grip to
  tuck it to a nub a hover pops back. It carries the modules you pick
  (`islandModules`: workspaces, clock, date, now-playing, and optionally the
  window title, status glyphs, tray), opens the frame-aware popouts from its
  docked edge, and remembers where it sits across a restart. The window reserve
  follows it live: dock it to any edge and tiles tuck against it there, hide it
  and the reserve shrinks to the nub. Power is not a module here; Super+Esc
  opens it as a vertical strip in the top-right corner.
- **Two Ryoku-native bar skins beside the two carried ones.** `barStyle` takes
  `aegis` and `stele` alongside `noctalia` and `caelestia`. Aegis drops the
  module pills for flat modules on the band, a mono editorial clock led by an
  accent tick, an accent underline beneath the active workspace and the
  interactive modules, and tabular mono status: the instrument-panel read.
  Stele engraves every module as a sharp bracket-cornered cell, frames the
  active workspace in accent, and splits the clock with a hairline divider.
  Both ride the same swollen frame edge and grow the same bar-edge popouts as
  the reference skins.
- **A third native skin, triptych.** `barStyle` also takes `triptych`, in the
  Brain_Shell mould: the top edge stays a hairline and the frame grows three
  lobes fused under the module clusters (left: seal, workspaces, title; centre:
  clock, plus now-playing while it sounds; right: status, tray, power), so the
  bar dips between the three instead of swelling into one straight band. The
  lobes are `BlobRect`s in the frame's own field. A popout on triptych is
  frame-aware: its own section swells to a full band to host it while the other
  clusters keep their dips, and on close it narrows back toward the module it
  grew from and melts into that lobe, so the dips return around it rather than a
  wide band deflating in place and snapping shut. The other skins keep their
  straight band.

### Changed
- **The workspace overview and wallpaper switcher open instantly.** Both were
  cold-spawned as a fresh `qs -c` process on every keypress, so Super+Tab paid a
  whole Quickshell start (Qt, Wayland, scene graph, first frame) and the overview
  then polled Hyprland every 140 ms for its still-empty window model before it
  could draw. They now run resident under the shell daemon and toggle over IPC,
  the way the launcher and pill already do: Super+Tab and Super+C flip a hidden
  surface visible against warm compositor models instead of launching a process.
  The overview's live `ScreencopyView` captures are gated on open, and both use
  the render-on-demand loop, so a hidden resident instance draws nothing and
  costs about its memory. The daemon also staggers the persistent components'
  startup so a handful of them no longer cold-start in the same login frame.
- **The orphaned standalone Alt-Tab switcher config is gone.** `quickshell/switcher`
  was a `qs -c switcher` overlay no keybind launched; the workspace overview
  covers window switching, so the dead surface was removed.
- **Bar-island content eases in and out instead of snapping.** The now-playing
  module grows along the band and fades when it appears and shrinks back when it
  leaves (music starting or stopping), instead of popping into place, and the
  island, and its triptych lobe, resizes with it. The focused-window title
  cross-fades old to new on a focus change and eases its width to the new
  length. Both ride the shared motion curves; `BarReveal` and `BarTitle` carry
  it, so every skin gets it.

### Fixed
- **`pip.conf`, the default-app map, and the nvim editor handler now reach every
  install.** `pip.conf` was shipped by no package, installer, or deploy path, so
  `pip install --user` hit the PEP 668 wall everywhere; `mimeapps.list` and the
  nvim `.desktop` were seeded once at install and never updated after. The
  package now ships all three, so `ryoku update` materializes them with the rest
  of the config, and `ryoku/shell/deploy.sh` lays them on a dev box too.
- **Tray menus open under their icon.** A right-click on a system-tray icon
  anchored the menu at the icon's local x inside the tray row, not its position
  in the window, so the menu appeared at the bar's left edge instead of under
  the icon. It maps to the icon's window position now, on every bar skin.
- **A runtime frame-border change repaints.** `BlobInvertedRect`'s border
  setters marked the blob group dirty but never scheduled the item's own
  repaint, so a border-thickness change at runtime, switching bar skins or
  moving the bar between edges, left the old band on screen until the next
  geometry change happened to nudge it. The setters now schedule the repaint.
- **The stele clock divider is visible again.** Its hairline used `Theme.line`,
  a token the bar theme does not define, so it resolved to transparent; it now
  uses `Theme.hair` like the other bar hairlines.
- **A popout close is one monotonic melt again.** The visible dip-then-pop
  was the border sink: the melt buries the body rect fully inside the frame
  band, which is exactly the state that makes the inverted border's inner
  wall recede to pocket a rect, so every close dug a body-wide notch past the
  flush line and released it in one frame at the zero-size drop-out. Blob
  shapes now carry a `sinks` flag and the popout body opts out, so it slides
  in flush while a docked recorder island still gets its pocket. Three
  supporting causes went with it: content teardown at half melt shrank
  `openW`/`openH` through the overshooting spatial Behavior mid-retract (the
  size is now latched at close start, the way `heldAlong` latches the
  centre; Caelestia freezes its launcher height on close with the same
  trick); the input-mask regions re-read the animated body rect and
  recommitted the wayland input region every melt tick (they now bind the
  resting open geometry and drop to zero when the close starts, so a
  dismissed body stops eating clicks; hover bands keep tracking live content
  size so a never-opened popout stays hoverable).
- **The blob deform spring survives hitchy frames and coordinate jumps.** The
  integrator now runs in 8 ms substeps, stable at any configured stiffness
  through a 100 ms frame (at stiffness 800 and up one such frame used to flip
  it divergent, which is what the giant-blob guard was catching), and a
  centre jump past 8000 px/s is treated as a coordinate change rather than
  motion, so a terminal melt collapse or a group rejoin cannot yank a squash
  into the whole field and ring for half a second.
- **Popouts now melt fully flush instead of stalling and snapping shut.** The
  blob body kept its full neck buried in the frame band until the very last
  frame of a close, and the smooth-min fillet held the fused edge a couple of
  pixels proud of the band; when the shape hit zero size it was deleted in one
  frame, ledge, wings and outline all vanishing at once. The body now retracts
  its inner face one smoothing-depth into the border as the close progresses,
  so the silhouette is already flush when the shape drops out. The mirrored
  pop-in on the first open frame is gone too.
- **A closing popout no longer teleports along the bar.** Opening popout B
  while A was up wrote the new icon centre while A was still pinned, so A
  jumped under B's icon (or to bar centre on a keybind) and melted there. The
  toggle now unpins the old popout before moving the anchor, so it melts where
  it opened.
- **Click-opened popouts close the moment you ask.** The body-hover latch that
  keeps the media hover panel alive while the pointer crosses onto it also
  applied to pinned popouts, so a close button, Escape or a keybind re-toggle
  appeared dead until the pointer left the panel. The latch is now scoped to
  hover-driven popouts.
- **Dragging the recording island no longer snaps it home mid-drag.** The
  overlay's input region only covered the island's current rectangle, and the
  island clamps at the frame lips, so a drag toward an edge let the pointer
  slide off the rect: the compositor dropped the grab and the return-to-frame
  spring yanked the island back while the button was still held. The mask now
  widens to the whole surface for the duration of the drag.
- **Wallpaper matching keeps the shell readable on loud wallpapers.** A
  saturated red wallpaper made wallust emit a bright crimson background, and
  the shell painted the frame, bar, popouts, launcher and widgets with it raw:
  red icons on red surface at 1.2:1. The Wallust singletons now tone-map the
  background into the shell's dark band (hue kept, value and saturation
  clamped) and walk accents toward white until they clear 3:1 against the
  surface. Dark palettes pass through byte-identical, so nothing changes on a
  well-behaved wallpaper. The media popout's time labels also move from the
  decorative `faint` token to `dim` so position and duration stay legible.
- **A home-deployed Ryoku.Blobs plugin can no longer shadow the packaged one.**
  The daemon prepended `~/.local/lib/qt6/qml` to the QML import path
  unconditionally, so one old deploy or recovery run pinned the compiled blob
  plugin at that vintage forever, ignoring every later `ryoku-blobs` update.
  Now only a home-deployed daemon (a dev checkout or recovery) prefers the
  home modules; a packaged `/usr/bin/ryoku-shell` loads the packaged plugin.
- **The blob deform spring can never draw diverged geometry.** If the
  integration state ever goes non-finite or lands a whole unit off identity
  (the target stretch caps at 1.35), the plugin snaps that shape to rest
  instead of rendering the garbage on every following frame.

### Changed
- **Voice dictation now runs on Voxtype, not Handy.** Super+` drives Voxtype
  (from `voxtype-bin`) with `voxtype record start`/`stop` on the tap, and its
  engine and model are chosen in Ryoku Settings' Dictation page. The pill's mic
  wave is unchanged; only the transcription engine behind it moved.

### Fixed
- **Live wallpapers no longer hijack the media controls.** mpvpaper is an mpv
  instance, and with `mpv-mpris` installed it registered the wallpaper clip as an
  MPRIS player: the launcher's NowPlaying listed the silent wallpaper as video,
  pausing your music handed the now-playing slot to it, and the MPRIS play/pause
  crosstalk froze the clip when music resumed. The wallpaper mpv now launches with
  `no-config load-scripts=no`, so the plugin never attaches.
- **Live wallpapers set without mpvpaper now, instead of erroring.** Setting a
  video called `mpvpaper` unguarded, so on a box without that optional AUR daemon
  ryowalls reported "Could not set wallpaper" even though the clip downloaded. The
  shell now falls back to a still frame of the clip (one ffmpeg grab shown through
  the image daemon) when mpvpaper is absent, so the pick applies and themes: with
  mpvpaper the wallpaper moves, without it it is the frame. mpvpaper stays a true
  optional enhancement.
- **Wallpapers set again on machines that predate the `swww` -> `awww` rename.**
  The shell drove the wallpaper daemon by the hard-coded name `awww`; upstream
  renamed `swww` to `awww`, and a `ryoku update` never pulls the AUR rename, so a
  box still carrying `swww` silently no-op'd every wallpaper set (static images,
  and with the daemon absent the palette retheme too). The shell now resolves
  whichever of `awww`/`swww` is actually installed (awww preferred, identical
  CLI), so ryowalls and Super+W work on either.
- **The voice dictation wave tracks the mic instead of stalling.** The mic
  spectrum used cava's PipeWire input, the same backend that quits within
  seconds here, so the wave came up late and dropped out mid-sentence while the
  mic itself was fine. It now reads the mic through cava's Pulse backend and
  execs cava so the analyser is reaped cleanly, matching the desktop visualiser.
- **Super+` voice dictation opens centred, and Handy stops flickering.** The
  keybind's socket fast path set the popout without clearing the previous
  icon's centre, so the mic wave grew from wherever the last popout had opened;
  it now recentres like every other keybind popout. The tap also toggled Handy
  via `handy --toggle-transcription`, which launches a second instance to relay
  the flag; on Wayland, where Handy cannot claim a global shortcut of its own,
  that popped the app in and out, so the shell now signals the running instance
  with SIGUSR2.
- **The desktop and launcher spectrum visualisers work again.** cava's PipeWire
  input backend quits within seconds on current PipeWire, so the bars blanked
  and restarted until the surface showed nothing at all. Both visualisers now
  read the sink monitor through cava's Pulse backend (pipewire-pulse), which is
  stable here, and exec cava from the launch shell so the surface's exit reaps
  the analyser instead of leaking a client every time it unloads.

### Added
- **A recording control that lives in the frame's blob field.** While a screen
  recording runs, a small island melts out of the nearest frame edge: a 6-dot
  drag handle, the elapsed time beside a pulsing dot, pause/stop, and mic +
  desktop-audio mutes. Grab it anywhere on the body to pull it off into a
  floating island; as it nears any edge the frame and the island reach for each
  other and merge (both surfaces bulge, like two drops). Let go anywhere, even
  at a corner, and it settles back onto the nearest edge. On a side edge it
  turns vertical while you hold it. A hide button tucks it to a nub with a
  record dot still pulsing on it, so you can see it is hidden, not gone;
  hovering that edge pops it back out. It melts into the frame when recording
  ends, leaving no mark.
- **The app launcher reads its look from a config the Hub edits.** A new
  `~/.config/ryoku/launcher.json` (watched live) sets the palette's corner
  roundness, the home card's weather units (Auto follows the locale, or force
  C/F), the backdrop image with its strength and focal spot, and whether the
  greeting and weather glance show. Changing the unit refetches, so the reading
  is always in the unit on screen, never a value wearing the other's symbol.
- **The now-playing module opens a transport on hover.** Hovering the bar's
  media module grows a compact popout from the frame edge at the module: an
  elapsed / total line you can drag to seek, over a prev / play-pause / next
  cluster. It melts open through the shared blob field like every other popout
  and stays open while the pointer is on the module or the panel.
- **The bar shows only the workspaces you're using.** The strip lists the
  workspaces that own a window plus the active one, so empty numbers vanish
  and it stays as short as your session; occupancy comes straight from hyprctl
  so it's right the moment the shell starts. A toggle in Ryoku Settings' Shell
  section brings back the classic 1..5 run with empties dimmed.

### Changed
- **Notification toasts grow from the bell.** A toast now melts out of the bar
  edge at the notifications bell like the inbox does, and dismisses on its own
  timer, instead of appearing in a separate top-right window. Clicking it still
  opens the full inbox.
- **A popout in a corner fuses into the wall.** A popout clamped against a
  screen edge (the power menu, a toast) now reaches that wall and melts into the
  frame border through the shared blob field, squaring off the corner, instead
  of floating a bar's-width inset off it and leaving a gap.
- **Ryoku Settings' Shell section matches the bar-and-popouts shell.** The
  Island tab is gone with the island; the live knobs it still drove (the
  volume/brightness OSD and notification toasts) moved to a Notifications
  group under Frame, the bar position is top or bottom, and the now-playing
  cover no longer crowds its title.
- **The wallpaper picker is a full-screen switcher now.** Super+C opens a
  Super+Tab-style overlay instead of the pill filmstrip: images and live video
  wallpapers ride two endless belts, the top drifting right and the bottom
  drifting left, sorted into colour groups. The belts idle-drift on their own,
  settle to a stop while the pointer is over them, and a scroll (or the arrows)
  pushes them faster before they ease back; a
  swatch strip filters by colour and an All/Images/Live row by kind. Tiles are
  cut-cornered with a colour chip, a LIVE tag on video and a dot on the
  wallpaper already set; hover or the centre pick lights one, a click or Enter
  sets it, Esc closes. Live tiles preview muted on the pick. The pill's
  wallpaper surface and the old thumbnail script are gone; the switcher keeps
  its own thumbnail and dominant-colour index under `~/.cache/ryoku-wp-thumbs`.
- **The floating pill and centre island are gone.** Everything the pill used to
  host now opens as a bar-edge popout that grows from the frame at its trigger
  with the bar painted on top (caelestia's model), so a panel never hides the
  bar and the bar stays clickable while one is open. `Super+V` clipboard,
  `Super+D` control deck, `Super+Tab` workspaces, the wifi/bluetooth link
  surface, the keyring prompt, voice dictation and the notification inbox all
  moved off the pill; keyboard panels hold the keyboard while open and release
  it on close. The volume/brightness OSD became a small edge window that floats
  above the bar.
  Left/right bar positions and the sysinfo panel were dropped. `Pill.qml`, the
  island blob fields and the island-reserve machinery are deleted.
- **The bar skins are the references now, not our riff on them.** After fair
  pushback that the plate slabs looked bad on round shells, the two bar
  styles are carried one-to-one from the credited shells: `noctalia` (fully
  rounded capsule modules on the band, dot workspaces whose active pill
  widens into an accent lozenge with its number, the stacked clock that
  folds to one line on thin bands) and `caelestia` (the numbered workspace
  cell strip inside one container pill with the sliding indicator and its
  stretchy leading/trailing edges, the calendar-glyph clock, the column
  layout on side bars). Iconography moved to Material Symbols Rounded
  (`ttf-material-symbols-variable`, now in the base set) with the caelestia
  hover/press feel on every module: an 8% overlay and a soft ripple from the
  press point, and the Material 3 expressive curve family drives the module,
  island and reveal motion. Content centres across the full band, so modules
  no longer crowded the bar's bottom edge.
- The battery readout works on AC again everywhere (the bar and the battery
  popout): UPower's synthetic display device drops off some
  versions once the cell sits full, so the Battery singleton now reads the
  physical battery. Wifi signal in the bar's status cluster reads the active
  connection's strength instead of an in-use marker nmcli omits without a
  rescan.

### Added
- Four wallpaper switch transitions ported from **caelestia shell v2**'s
  Material 3 Expressive motion (the animation curves in its
  `plugin/src/Caelestia/Config/tokens.hpp`) join the Super+W rotation.
  `celeste_veil` reproduces caelestia's own wallpaper crossfade (the
  `expressiveSlowEffects` curve) exactly; `comet_streak` (emphasized-decelerate
  wipe), `aurora_ripple` (expressive-fast wave) and `starfall_bloom` (standard
  iris from the top) carry its other signature curves onto our geometric
  sweeps. All ride the shared transition speed. caelestia's springy spatial
  curves overshoot and its emphasized curve is a two-segment spline, so they
  fall outside awww's single monotonic bezier and are left out.
- Status-cluster click popouts, matching the reference catalog. Clicking a
  status icon on the bar reveals its own compact control panel growing from the
  bar edge at that icon (and melting back into it), on a top or bottom bar
  alike. A volume icon opens the mixer (moved off the now-playing module); the
  wifi icon a network panel (enable toggle, rescan, signal-sorted list, inline
  password connect); a bluetooth icon a device panel (adapter toggle, scan,
  pair/connect); the battery icon a readout (charge, state, time, draw,
  capacity, health); and the clock opens the month calendar. The network and
  bluetooth panels reuse the Link surface's own wifi/bluetooth engines, so
  connect and pair behave identically.
- **The bar moves and wears two skins.** `barPosition` places the band on the
  top or bottom frame edge. The chosen edge swells and claims its own strip.
  `barStyle` picks the skin: `plates` keeps the sharp washi slabs, `capsule`
  renders every module as a fully rounded tonal pill (the caelestia idiom) for
  shells riced round, and the workspace block, media art and hover fills all
  follow the choice. With a bar present the resting clock island is gone -- the
  bar carries the clock, workspaces, media and status, and summoned panels grow
  from the bar edge instead of a floating centre pill.
- **The top bar is a real bar now, and the default face.** The band used to be
  naked text floating on the frame's thickened edge; it is now composed of
  module plates: sharp slabs with a faint warm fill and a hairline edge that
  lift on hover, so every module reads as touchable (`pill/BarPlate.qml`).
  What the plates carry:
  - the 力 seal opens the launcher;
  - a workspace strip (`pill/BarWorkspaces.qml`) with mono numerals and an
    accent block that slides behind the active one, leading edge fast and
    trailing edge slow, so a switch stretches across and contracts (the
    caelestia trail); occupied numerals read brighter, click jumps, wheel
    walks neighbours, and cells past five only appear once used;
  - the clock plate (the anchor the calendar drops from) with the vermilion
    colon and a tracked mono date;
  - now-playing (`pill/BarMedia.qml`): art thumb, ping-pong title, play
    wedge; click toggles, wheel nudges the sink volume, and the live
    wallpaper's mpv is filtered out so scenery never poses as music;
  - a status cluster (`pill/BarStatus.qml`): wifi arcs or an ethernet tick
    (fed by the new gentle `Network` singleton, nmcli without rescans),
    battery cell + percentage, the inbox bell with an ember dot while
    something waits, and the DND mark; each glyph routes to its surface
    (link, battery, inbox);
  - the tray on a quiet plate with per-icon hover lift, and the power glyph.
  A wheel over bare band nudges the volume, narrated by the OSD. New shells
  start with the bar on (`barEnabled` default true).

### Fixed
- `ipc/wallpaper.go`: setting a live (video) wallpaper could silently do
  nothing. Every `wallpaper set` was gated behind `ensureWallDaemon()` (the
  awww image daemon), yet a video plays through mpvpaper and never needs awww;
  awww is not autostarted either, so a session that boots on a live wallpaper
  never starts it. If awww then failed to come up, the set returned success
  with nothing painted (ryowalls reported "Wallpaper set" while the wallpaper
  stayed put). The daemon now chooses the backend by file type: a video goes
  straight to mpvpaper with no awww dependency, only image sets ensure awww,
  and a failed mpvpaper launch surfaces as a real error instead of a no-op.
- Blob motion matches the reference shell now. The blobs already render
  identically, but five things made the melt feel less smooth: the deform
  spring used explicit Euler (the energy-injecting form our own ported comment
  warns against) -> switched to the semi-implicit closed form so it settles
  instead of wobbling on frame hitches; the render loop was basic (on-demand,
  GUI-thread) -> threaded (vsync-locked) so the per-frame spring gets regular
  deltas; popouts opened on a no-overshoot curve -> the spatial spring (500ms);
  deformScale was ~40x too large so any motion slammed the stretch cap -> cut
  to the reference's subtle value; and popout content was rigid over a
  deforming blob -> it now transforms by the blob's deform matrix and fades on
  the effects curve, so content and blob move as one body.
- Bar mode no longer swallows notifications and the volume OSD. The island
  logic only summoned the drop panel for open surfaces, so a toast or a
  volume change rendered nothing while the bar was on; both now melt out of
  the band like any summoned surface, and the input mask follows the panel
  so toast actions stay clickable.
- The resting island shows a small ember tick while notifications wait (and
  DND is off), so a quiet desktop still answers "did anything ping me".
- Mixer and power popouts fuse with a side bar as one body. On a left or
  right bar they now grow from the bar's inner edge (power right at its
  button, the mixer from the bare-band centre) and melt back into the band
  on close. Before, they grew from a fixed frame inset that landed inside
  the swelled band, so opening lumped the popout and band into one stuck
  slab, and the power menu even opened on the edge opposite its button.
- Hovering the clock (or anything) on a side bar no longer opens the power
  menu. The power popout carried a tall invisible hover band that, sitting
  by the power button at the bottom, overlapped the clock and status
  modules above it. The side-bar power menu is now click-only (open it by
  tapping the power button, like the reference), with no edge band behind
  its neighbours; the island/top-bar power keeps its edge hover.
- Side-bar popouts open from their own module, not an invisible edge band, so
  hovering one module can never open another's popout (the reference's
  per-module ownership). The now-playing module owns the mixer (hover opens it
  there; a tap still plays/pauses, the wheel still nudges volume); the power
  button owns the power menu (click). This removes the last stray edge band
  from a side bar; top/bottom/island popouts keep their thin-lip band.

### Security
- `ipc`: the `ryoku-shell` control socket is now created owner-only (0700). It
  drives session-scoped actions (surface toggles, wallpaper, dictation), and
  `net.Listen` left it at the ambient umask (0755 at the usual 022), so the
  socket node was world-traversable; when `XDG_RUNTIME_DIR` was unset it fell
  back to `/tmp`, where another local user could reach it. Forcing the umask
  around `Listen` makes it 0700 atomically, with no world-visible window.

### Added
- Ryoku now ships the reference desktop feel as the default look. Shell
  (`quickshell/*/Singletons/Config.qml` + the hub's reset baseline): a thinner,
  softer frame (radius 9, border 59, smoothing 8, shadow 0.63/12), a **floating**
  island that reveals on hover, and JetBrains Mono Nerd Font at 1.3x. Windows
  (`hyprland/modules/decoration.lua` + `hub` `defaultOverrides`): near-square
  rounding 2, gaps 12/18, border 2, blur 4/1, opacity 1/0.94. Seeded on first
  run, so fresh installs get it; existing configs are untouched, and Reset to
  defaults adopts it. Hardware and private bits (monitors, cursor, widgets,
  wallpapers, input) stay at the shipped baseline.
- `plugin/` (`Ryoku.Blobs`) and `quickshell/pill`: the frame, pill and popouts
  gain a 1-2px outline along their shared silhouette, in the wallust hue Hyprland
  uses for window borders (raw `color4`, exposed as `Wallust.border`). `BlobGroup`
  gained `borderColor`/`borderWidth`; the SDF shader paints a band just inside the
  rim, so the melted shapes read as one lined body, not per-shape strokes.
- `quickshell/launcher` **Bluetooth bubbles**: connected devices float as their
  own compact square-cornered cards under the palette window, one per device,
  the Android quick-pair tile in Ryoku grammar -- name up top, a big
  Material-style class pictogram on the left (BlueZ classifies the device as
  audio-headset, input-mouse, phone, ...; the glyphs come from the Material
  Design Icons set already embedded in the shipped Nerd Font), and the battery
  reading large in the corner when the device reports one ("connected" when it
  doesn't). Live off Quickshell.Bluetooth: cards appear on connect, drop on
  disconnect, battery updates in place. Nothing connected renders nothing at
  all. The launcher socket's `state` dump gains `btConnected`
  (`BtConnections.qml`, instantiated in `shell.qml` under the Launcher card).
- `ipc`: a new `ryoku-shell stash-send <file>` command opens the control deck's
  LocalSend picker on that file (a new pill `stashSend` IpcHandler that shows the
  stash and calls `openSendPicker`), so the Nautilus stash menu can hand a file to
  the deck's send flow instead of reinventing device discovery. The path is the
  raw remainder of the command line and goes through the qs client, so a path with
  spaces survives intact; `stashSendPath` is unit-tested. `deploy.sh` also drops
  the `ryoku-stash-menu.py` Nautilus extension into the user extensions dir for the
  dev loop.
- `quickshell/launcher` RyoTunes gains **shuffle** and **gapless prefetch**. A
  shuffle toggle in the now-playing transport (lit when on) reorders the queue via
  mpv's own `playlist-shuffle`/`playlist-unshuffle` (history and prev/next stay
  intact); the engine re-syncs its queue from mpv's new order by videoId so the
  card's cover/title stay correct. mpv now runs with `--prefetch-playlist=yes`, so
  it opens the next queue entry only as the current one nears its end (initial
  connect + `yt-dlp` resolve, not an early full download) - the next track starts
  gaplessly without stealing bandwidth mid-song, safe on slow connections.
- `quickshell/launcher` RyoTunes plays **pasted YouTube / YouTube Music links**,
  including playlists and mixes. Pasting a link (with or without the `@` prefix)
  offers a one-tap "Play": a bare track link seeds its radio, a playlist or mix
  link (`?list=...`) queues the whole playlist through the same InnerTube `/next`
  path. Played playlists are **cached** (`Singletons/Playlists.qml`, an LRU under
  the cache dir) and shown as a **力 SAVED PLAYLISTS** chip row under the
  now-playing stack (`SavedPlaylists.qml`), so the full playlist replays instantly
  with one tap and no network round-trip. Link parsing and the playlist-aware
  radio body are in `providers/media/ytmusic/ytmusic.js` (node-tested); the
  provider surfaces unprefixed links via a `urlFallback` gate
  (`lib/dispatch.js` `looksYtUrl`). Documented in `docs/ryotunes.md`.
- `quickshell/launcher` **RyoTunes**, YouTube Music as the built-in free-music
  source. The `@` provider now searches YouTube Music's keyless InnerTube API
  (`curl`) instead of `yt-dlp`, returning proper songs with clean title/artist/
  album and **square album art inline** (shown in the row and on the now-playing
  card, no second cover lookup), markedly faster than the old search; a prefix
  cache makes refining a query feel instant, and `yt-dlp` flat search stays as a
  fallback when InnerTube is unreachable. Playing a track no longer stops at its
  end: a new engine (`Singletons/Radio.qml`) streams it with a persistent `mpv`
  driven over its JSON IPC socket (Quickshell native `Socket`, no `socat`) and
  auto-extends an **endless YouTube Music radio** (the `/next` continuation),
  which `mpv-mpris` exposes so the card's Next/Prev, the media keys, an up-next
  peek, and **scrub-to-seek** (drag the wavy bar) drive the queue. Playing a track **takes over**: other players (a
  browser tab, Spotify) pause so audio never stacks, and the now-playing card
  **extends into a slim strip per other player** (`MediaSources.qml`) so both
  sources stay visible and one tap switches between them. The card is sticky
  (pausing it never makes it jump to another source) and shows a **buffering**
  state with a frozen seekbar so a slow load never ticks in silence. The MPRIS
  now-playing row gains a **YT Radio** verb that seeds an endless station from
  whatever is already playing; our stream yields to other audio by fading out and
  pausing (not a hard kill). Search + radio parsing live in one node-tested file
  (`providers/media/ytmusic/ytmusic.js`, so QML resolves the shared helpers with
  no `require`); the iTunes cover fallback (`albumart.js`) strips video noise
  (`(Official Video)`, `[HD]`, `feat.`) for a better match. Documented in
  `docs/ryotunes.md`. No new packages.
- `quickshell/launcher` removed the built-in Spotify catalog provider (`s:`) and
  its `ryoku-shell spotify` Web API backend (never released). Spotify stays a
  fully detected MPRIS player: the now-playing card controls it and the YT Radio
  verb can seed free music from it.
- `quickshell/launcher` a standalone command palette (`Super + Space`), a full
  rebuild of the old pill app-list, dropped from the pill so it has room for a
  Raycast/Alfred-class feature set. A daemon-supervised, kept-warm Quickshell
  component (`ryoku-shell launcher`) with provider folders under `providers/`:
  apps, calculator (qalc), system actions (`/`), clipboard (`;`), windows, web
  (`?` + bangs), files (fd), snippets + quicklinks, packages (GPK), MPRIS
  now-playing, YouTube Music (`@`), and a rofi-script/dmenu protocol provider for
  third-party scripts. Two-tier UX (root search + `Ctrl+K` action panel), an
  all-apps grid (`Ctrl+A`), and a now-playing detail with the wavy seekbar.
  Ranking and protocol logic are `lib/*.js` with `node` tests. Documented in
  `docs/launcher.md`. Blur via the `launcher` layer rule in
  `hyprland/modules/decoration.lua`; the `launcher` verb moves from the pill to
  the standalone component in `ipc/daemon.go`. Adds `ryoku-cmd-songrec`.
- `quickshell/launcher` web provider: a `?` query now shows an inline DuckDuckGo
  instant answer above the search row (heading, wrapped abstract, source), so
  `?what is nmap` answers in place instead of only offering a Google search. The
  answer is fetched keyless and async (`providers/web/ddg.js`, node-tested;
  rendered by `AnswerPanel.qml`); a `!bang` skips the fetch and goes straight to
  the chosen site, and the search row stays as the always-present fallback.
- `quickshell/launcher` now-playing: when the media player exposes no cover art
  (an mpv or yt-dlp stream, some browsers), the card fetches one from the keyless
  iTunes Search API by artist and title, once per track
  (`providers/media/albumart.js`, node-tested), so the music-note placeholder is
  a last resort rather than the norm.
- `quickshell/launcher` now-playing: a live cava spectrum wave now sweeps behind
  the card, the same filled smoothed curve the desktop visualiser draws, tinted
  vermilion and eased per frame so it flows with the music. A launcher-local
  `Singletons/Spectrum.qml` reads the PipeWire monitor, gated in `shell.qml` to
  run only while the launcher is open and a player is actually playing (never on
  a hidden or silent palette); the path geometry is `lib/spectrum.js`, node-tested.
- `deploy.sh` installs the Ryoku VM launcher icon (the brand mark) into the user
  icon theme, so the **Ryoku VM** app entry shows the logo instead of a blank tile.
- `quickshell/pill` Control Deck: a **Game Mode** control in the deck, a session
  tile that flips `Flags.gameMode` (the one-click
  competitive toggle). The shell bridges the flag to `ryoku-cmd-game-mode` the way
  Keep-Awake bridges the caffeine helper, and pulls Do-Not-Disturb on while it is
  active (`Flags` saves and restores the prior DND so it never clobbers a user who
  keeps DND on independently). Covered by
  `tests/game-mode.sh`.
- A desktop plugin's right-click menu renders the plugin's own settings inline from
  its `metadata.settings` schema - choice chips, a toggle, a slider, and an image
  thumbnail strip scanned from `~/Pictures` - so a widget (e.g. the photo frame) is
  restyled and its picture changed in place, no Settings trip. Mouse-only (the
  wallpaper layer has no keyboard, so text fields stay hub-side); changes persist
  through `ryoku-plugins-place settings`.
- `quickshell/plugins/ryoku-plugins-place` gains `seed`, `settings`, and `forget`
  verbs: `seed` injects a plugin's manifest preset block (default host + default
  settings) into `plugins.json`, filling only what is missing so user edits
  survive; `settings <json>` merges an edit; `forget` drops the entry. Enabling a
  plugin now also seeds, so its settings exist in the right place the moment it
  goes live.
- `quickshell/pill` stash install: drop a file, get a launcher entry. Beyond
  AppImages and self-contained tarballs, the installer now handles native and
  portable package formats so they open afterwards:
  - Arch packages (`.pkg.tar.zst`, or any tar carrying a `.PKGINFO`) install with
    `pacman -U` through pkexec (the polkit agent prompts); the package's own
    desktop entry is read natively. Previously these were misread as plain
    tarballs and extracted into `~/.local`, producing a broken entry (the binary
    lives under `/opt`), which is why a Warp terminal install never opened.
  - Flatpak bundles (`.flatpak`) install into the user installation with
    `flatpak install --user` (the flathub remote is ensured first so the runtime
    resolves), and flatpak exports the desktop entry. Adds `flatpak` to the base set.
  - `.deb` and `.rpm` payloads are extracted with bsdtar and run through the same
    app-discovery as tarballs (best-effort; an app that hardcodes system paths is
    better served by its native package or flatpak).
  `Stash.qml` `hasInstallable` offers exactly these; self-extracting `.bin`/`.run`
  stay out (running an arbitrary installer blind is unsafe). Covered by
  `tests/stash-install.sh`.
- `quickshell/pill` stash install now clears the source from the stash after a
  successful install, so an installed app is not left duplicated as a leftover
  drop (it already lives in the app store, or a system/flatpak install). It only
  removes on success, never on failure, and `RYOKU_STASH_KEEP=1` opts out.
  Covered by `tests/stash-install.sh`.
- `quickshell/widgets`: desktop widgets on the wallpaper. A new `WlrLayer.Bottom`
  host (per monitor, below windows, namespace
  `ryoku-widgets`), supervised like the pill/visualiser via a `{"widgets", true}`
  entry in `ipc/daemon.go`, carries a clock and a weather widget. The clock ships
  five faces (digital, minimal, analog, flip-card, and a wallust ring clock) with
  12/24h and an optional seconds cluster, plus three toggleable date designs
  (inline, badge, stacked); its accent follows wallust, the brand, or stays mono.
  The weather widget ships three designs (card, minimal, strip) over a live
  animated sky (sun/moon-and-stars, drifting clouds, rain, snow, lightning storm,
  fog) chosen by the WMO condition, with a C/F unit toggle and a today/week scope.
  Every widget is fully customisable, design, size, background (none/card/glass)
  and radius, placement (nine snap zones or a free dragged position), and
  opacity. On the desktop the widgets are interactive: left-drag moves a widget
  (snapping to a grid that fades in, with a press bump and an open/closed-hand
  cursor), the bottom-right bracket resizes it (scrubbing the scale, with a live
  readout), and right-click opens a menu in the carbon-dossier idiom (a 力
  masthead, corner registration ticks, hairline rules, mono spec rows with a
  vermilion hover tick). Right-clicking the bare desktop opens the desktop menu
  (show or hide each widget, settings, reload); right-clicking a widget opens its
  own (cycle the design, toggle date/motion/units, lock against accidental drags,
  snap to a zone, hide it), both ending in the global settings and reload-shell
  actions. The layer is interactive across the wallpaper so the right-click lands
  anywhere, but it sits below windows and a left click on bare wallpaper falls
  through to nothing. State lives in
  `~/.config/ryoku/widgets.json` (a watched `Config` singleton, defaults seeded on
  first run), so a save in Ryoku Settings, a drag, or a menu action retunes the
  running widgets with no reload. Weather comes from Open-Meteo (no key), reusing the pill's cached
  location at `~/.local/state/ryoku/weather-loc.json`; its WMO-to-animation mapping
  and parsing live in `weather/lib/weather.js`, unit-tested by
  `weather/lib/weather.test.mjs`. A `Wallust` singleton watches
  `~/.cache/wallust/colors.json` so tinted widgets retune to the wallpaper.
- `quickshell/pill` stash: a cobalt download window. The Download action now opens
  a paste-the-link panel modelled on cobalt (https://github.com/imputnet/cobalt):
  auto/audio/mute modes, a Paste button, and a processing queue that runs links in
  order with per-item progress. A Remux tab rebuilds a media file's container
  losslessly (no re-encode), and a drop zone takes files straight in. The engine is
  cobalt: `stash-cobalt.sh` POSTs to a cobalt API instance (set `COBALT_API_URL`,
  default `http://localhost:9000`; run one per cobalt's docs) and falls back to
  yt-dlp when none is reachable, so a fresh install still downloads. Remux is a
  local ffmpeg stream copy, the same on-device operation cobalt's own remux does.
  The cobalt credit stays visible in the window since the engine is theirs.
- `quickshell/pill` stash: LocalSend receive and send-a-note. The header's Receive
  switch runs a LocalSend v2 server (`localsend.sh receive`, a self-signed HTTPS
  endpoint that announces the machine on the LAN over multicast) which drops any
  pushed file straight into the stash and shows a live tally; the Text action
  sends a typed or pasted note (written to a temp file) to a picked device. The
  `Stash` singleton drives both: the receiver streams `READY`/`INCOMING`/`SAVED`
  lines parsed by a `SplitParser`, the rest reuses the existing send helpers.
- `quickshell/pill`: weather now comes from Open-Meteo (no API key) instead of the
  rate-limited wttr.in scrape, with the resolved location cached at
  `~/.local/state/ryoku/weather-loc.json` so a restart skips the lookup. The
  temperature unit follows the locale (Fahrenheit for US/LR/MM, Celsius
  elsewhere). The pill and calendar readout (`temp`, `condition`, `glyph`) is
  unchanged; `hourly`, `daily`, `humidity` and `city` are now exposed for a future
  hourly/5-day pane. The WMO-code mapping, unit logic and parsing live in
  `lib/weather.js`, unit-tested by `lib/weather.test.mjs` against a captured
  response.
- `quickshell/pill`: the Calendar surface gains local events. A new `Events`
  singleton persists a JSON array at `~/.local/state/ryoku/events.json` (its date
  logic lives in `lib/events.js`, unit-tested by `lib/events.test.mjs`: coverage,
  multi-day spans, time ordering, and the `HH:MM` entry parser). Days with events
  show a dot, clicking a day selects it, and a compact editor under the grid lists
  that day's events (delete on hover) with a single add field that reads an
  optional leading `HH:MM` start time. The surface grows downward only, so the
  pill geometry stays unchanged.
- `quickshell/pill/lib` + `tests/`: unit-test coverage for the launcher fuzzy
  ranker (`fuzzy.test.mjs`, 15 assertions over prefix, substring, and subsequence
  ranking, the usage tiebreak, and `noDisplay` exclusion), plus a
  `tests/shell-unit-tests.sh` runner and a `Shell unit tests` CI workflow that run
  every `ryoku/shell/**/*.test.mjs` (the ranker and the ryoshot
  coords/keymap/annotation libs) on each change. These pure-JS helpers have no
  Quickshell or display dependency, so unlike the advisory qmllint job this is a
  real gate.
- `ipc/ryoku-shell` + `quickshell/pill`: the GNOME keyring password prompt is now
  a pill island instead of gcr's centred GTK dialog. The daemon registers as the
  keyring system prompter (`org.gnome.keyring.SystemPrompter`, interface
  `org.gnome.keyring.internal.Prompter`) on the session bus and reimplements the
  `sx-aes-1` secret exchange (Diffie-Hellman over the 1536-bit IKE group,
  HKDF-SHA256, AES-128-CBC) so gnome-keyring is unaware of the swap. The typed
  secret returns to the daemon over the control socket on its own line, never as
  a process argument (which would leak through world-readable /proc cmdline). A
  new `KeyringSurface` grows the prompt out of the pill centre (the unlock ask,
  the choose-a-new-password ask with a confirm field, and plain confirms), takes
  exclusive keyboard focus, and treats Escape, the backdrop, and Cancel as a
  cancellation. `ipc/prompter.go` and `ipc/secretexchange.go` hold the daemon
  side; `quickshell/pill/Singletons/Keyring.qml` holds the live state. The daemon
  owns the name from startup, so gcr's own prompter stays as the fallback when the
  shell is not running.
- `quickshell/pill`: island appearance styles. The top island now has three
  looks, read from `shell.json` (`islandStyle`) and chosen in Ryoku Settings'
  Shell section: `island` (the classic pill melted into the top frame, the default
  and unchanged), `floating` (a detached pill that hangs just under the frame and
  floats over the content), and `none` (no resting island). An `islandAutohide`
  flag hides the island at rest and reveals it on a hover of the top centre, for
  the `island` and `floating` styles. In every style but the always-shown classic
  island, the reserved top strip collapses so tiled windows rise to the same gap
  as the other three frame edges. A hidden island stays out of the way: only an
  open surface (a keybind), a peek, or (auto-hidden) a top-centre hover summons it,
  so keybinds stay fully functional while a passing toast or OSD never pops it. The
  frame is identical across styles.
- `ipc/wallpaper`: a `refresh` mode (`ryoku-shell wallpaper refresh`) repaints the
  current wallpaper on every output with no transition, so a monitor connected
  mid-session shows the same image without re-animating the displays that already
  have it. The Hyprland hotplug handler calls it after autoscale.
- `quickshell/pill`: the shell's look is now config-driven and live-editable. A
  new `Config` singleton reads `~/.config/ryoku/shell.json` (watched, atomic
  writes, defaults seeded on first run), and the frame and island read every
  appearance value from it: the screen border's corner radius, thickness, surface
  colour, opacity, edge-melt smoothing, and contact shadow (strength and size),
  and the top island's width, height, rest/open corner radius, top gap, bud-melt
  smoothing, and opacity. The mixer/power popouts follow the frame's radius and
  smoothing. Ryoku Hub's Shell Settings edits this file, so a save retunes the
  running shell with no reload; the hand-tuned defaults preserve the shipped look.
- `quickshell/visualizer`: a desktop audio visualiser. A full-width cava spectrum
  rises from the bottom of the wallpaper on a click-through `WlrLayer.Bottom`
  surface, behind every window and per monitor, with vertical-beam bars, a soft
  bloom, and a fading reflection. It blooms while audio plays and settles to a
  calm breathing line when the system is silent. On by default and supervised like
  the pill; `ryoku-shell visualizer` (`Super+M`) toggles it, `ryoku-shell
  visualizer-overlay` (`Super+Shift+M`) raises it over the windows on demand, and
  cava only runs while it is on. Adds the `Spectrum` (64-band cava on the PipeWire playback
  monitor) and `Wallust` singletons under `quickshell/visualizer/Singletons`, and
  the `visualizer` route plus persistent component in `ipc/daemon.go`.
- `quickshell/visualizer`: the visualiser is now config-driven and live-editable. A
  new `Config` singleton reads `~/.config/ryoku/visualizer.json` (watched, atomic,
  defaults seeded), and the spectrum reads its look from it: on/off (also `Super+M`,
  which now persists), style (bars, a filled wave, or floating dots), position
  (bottom, top, or centre), bar/dot shape (rounded or flat), left-right mirroring,
  band count, bar height and width, bloom, reflection, and the idle breathing wave.
  Changing the band count restarts cava with the new bars. Ryoku Hub's Shell
  Settings has a Visualizer tab, with a live preview, that edits this file.
- `quickshell/visualizer`: every style now animates off one per-frame `FrameAnimation`
  ticker locked to the display refresh, easing each band toward its target (fast
  attack, slow decay) so motion is smooth at 60fps+ rather than stepping between
  cava frames. A smoothed `activity` signal (fast rise, ~1s release) crossfades
  between the live spectrum and the idle wave, so a quiet gap fades down and back
  up gracefully instead of snapping off. With the idle wave disabled the spectrum
  fades to nothing on silence (the minimum sliver and the wave canvas clear fully)
  rather than leaving a thin line.
- `wallust`: a new `shell` template writes the live palette to
  `~/.cache/wallust/colors.json` on every wallpaper change. The visualiser's
  `Wallust` singleton watches it, so the spectrum's colours retune to the
  wallpaper. This is the first QML surface to follow wallust at runtime; the Theme
  palette stays static.

### Changed
- `quickshell/launcher`: the rest card's clock/date strip is reworked from a flat
  slab into a solar-arc scene. The clock and greeting read over a filled wave
  horizon that traces the real day: the stretch behind the marker glows the phase
  colour (how far through this phase we are), the stretch ahead stays faint (what
  is left of it), and a sun by day or a carved crescent moon by night rides the
  ridge at its true position. Day runs from the IP-located sunrise to sunset, night
  wraps midnight to the next sunrise, both fetched through the same Open-Meteo call
  as the weather (`daily=sunrise,sunset`, parsed by `sunFrac`); until that resolves
  the marker falls back to a plain clock. It is the same fill-is-elapsed grammar as
  the NowPlaying seekbar below it, so the resting card and the playing card read as
  one family. The sky colours are fixed (golden day, cool night) and deliberately
  independent of the wallust accent, so the sun stays a sun on any wallpaper. The
  old 力 seal is dropped, a recessed `cardBot` surface with a top sheen replaces the
  lighter floating fill, the colon breathes in the phase colour like the other clock
  faces, and the wave drifts only while the palette is shown so an idle launcher
  costs nothing.
- `ipc/`: the daemon is the single owner of `wallust`. A palette-only `wallpaper
  repaint` re-derives colours with no image transition, and the hub calls it
  instead of running wallust itself. Shell chrome (pill, island, widgets,
  plugins, switcher) reads the one colour master, `theme.json` `followWallpaper`,
  instead of `shell.json` `matchWallpaper`, so borders and chrome follow the
  wallpaper together.
- `quickshell/pill`: the `力 CONTROL DECK` is restructured into a tighter control
  centre (~40% smaller footprint) so it reads less like a generic settings list.
  The right column's six stacked sections collapse to three whitespace-grouped
  zones: **Tools** (the capture-launcher strip, now spread edge-to-edge), a new
  **Controls** zone, and **Record**. Controls unifies what were three separate
  sections: Keep-Awake and Game-Mode become two wide session stat-tiles (glyph,
  label, live value; the whole tile taps to toggle and tints when on) over the
  wifi/bluetooth/mic/DND/night quick-toggle row. Record folds the recordings list
  in under one eyebrow with the count. Section eyebrows drop from eight to four,
  the interior hairline rules give way to spacing, the decorative Tools WaveMeter
  is gone, the masthead is slimmer, and the surface narrows from 660 to 590
  scale-units. This also removes the void where Stash stretched to match the
  taller right column. No functionality is dropped.
- `quickshell/pill`: the mixer popout is reworked from a row of vertical faders
  into an audio control center, while keeping the frame-edge melt and the
  `ryoku-shell mixer` pin. OUTPUT and INPUT each show the active device with an
  inline selector that detects and switches the PipeWire default sink/source
  (`preferredDefaultAudioSink`/`Source`), a horizontal ink fader with a live peak
  meter, and mute. A Bluetooth output adds a chip with battery (native BlueZ
  device), codec, and an A2DP/Headset profile toggle, read from the bluez card via
  `pactl`. An APPS section gives each playback stream its own volume, mute, and
  meter with app icon and name. DISPLAY keeps per-monitor brightness (ddcutil) and
  vibrance (nvibrant), restyled to match. The popout melts to fit and grows as the
  picker expands. New `Singletons/Audio.qml` owns the graph; new `HFader`,
  `MixerDeviceRow`, `MixerAppRow`, and `MixerDisplay` components; `VFader` retired.
- `quickshell/widgets`: a desktop widget's right-click image picker gains a
  **Browse** tile that opens the system file chooser (portal), and its thumbnail
  scan recurses one level into `~/Pictures` (so Wallpapers / Screenshots appear),
  not just the top level.
- `quickshell/widgets`: a desktop-widget tile now honours its plugin's manifest
  default card style (`defaults.desktopWidget.bg`) when the placement pins none,
  so a plugin like photo-frame can opt out of the host card (`bg: "none"`) and
  draw its own frame.
- `quickshell/pill`: Stash, Tools, and Utilities are unified into one wide
  `力 CONTROL DECK` surface instead of three separate pill popouts, opened by
  `Super+D` (the old `Super+Z` and `Super+U` binds are removed). Single view, no
  sub-tabs: a 力 masthead over two hairline-split columns (Stash left; Tools,
  Controls and Record right), corner registration ticks, mono micro-labels and tabular
  figures in the hub Profile dossier idiom. Stash drops onto a filling tray with
  the Profile's square spec grid; its action bar is evenly spaced; the Send,
  Receive, Download and Task sub-screens are dismissed by a single Back control
  in the stash header beside the file count. New `DeckSurface`, `DeckStash`,
  `DeckTools`, `DeckControls`, `DeckRecord`, `DeckSegmented`, `MicroLabel`, `SpecRow`,
  `CornerTicks`; the standalone `StashSurface`, `ToolkitSurface`, and
  `UtilitiesSurface` are retired, and the old Caffeine tile drops out (Keep-Awake
  covers it).
- `quickshell/pill`: the battery surface shows real Health now, read from the
  physical battery device (the synthetic UPower display device reports no
  capacity), and Rate/Time read `0 W`/`Full` on AC instead of bare dashes. The
  soul bead no longer docks on the percentage digits.
- `quickshell/pill`: the island is reworked into the Ryoku carbon-dossier
  language (matching the hub Profile). A 力 foundry stamp leads, the clock is
  tabular over a mono date/weather line, corner registration ticks frame the
  readout, and hairline rules separate a running-app dock from the status cluster
  on flat carbon. Wi-Fi is dropped from the island (it lives in the hub
  Connections section now); battery and notifications stay. New `AppDock` and
  `BatteryGlyph` components. The battery surface is redesigned: a hero percentage,
  the Ryoku wave as the charge gauge, and a rate/time/capacity/health stat grid.
  The pill calendar's today cell is a warm brand marker (vermilion fill, ring,
  flame lap) instead of the cool frame.
- `quickshell/pill`: the island's rest state (idle, collapsed) drops the 力 stamp
  for a cleaner read. A tabular `HH:MM` clock with a vermilion colon leads a
  stacked mono weekday/date, above the workspace wave; the Ame bead's rest anchor
  moves from the stamp to the clock.
- `quickshell/pill`: the now-playing card is reworked into the carbon-dossier
  idiom. A 力 MEDIA eyebrow leads the title and artist, the source/time line is
  mono uppercase, the play seal is flat vermilion (no gloss), and corner
  registration ticks frame it. The album art and the Ryoku wave seek line stay.
- `quickshell/pill` stash: the action bar lights only what applies. It reads the
  live file types (`Stash.hasMedia` / `hasInstallable`), so Compress dims unless
  there is a video/image/audio file and Install dims unless there is an AppImage or
  tarball; a lone note no longer offers to compress or install it. The LocalSend
  send sheet gained a Scan again button (and a header rescan icon) so an empty
  device list can be refreshed without reopening it.
- `quickshell/pill` stash: the surface is rebuilt around a full-width file grid
  with a toolkit-style action bar (Send all, Text, Download, Compress, Install) in
  place of the cramped left rail, plus a header file count and Receive switch.
  Sends raise a focused sheet (LAN scan, pick a device, then a confirmation naming
  exactly what goes where), and every rail job now opens on a confirm step
  (download shows the clipboard link it found) before it runs; removing a file
  confirms inline on its tile rather than vanishing on a stray click. The look
  follows the Hub's flat tiles, hairline rules, and type tags in the pill palette,
  with a brand drop ring while a drag is over the surface. `StashRail.qml` is
  retired; `StashActions`, `StashSendSheet`, and `StashReceive` are new.
- `quickshell/pill` update island: surfaces the git update channel it was built
  for. Its count and target version read the commits the checkout is behind
  `origin/main` (from `ryoku status --json`) rather than pacman package counts.
- `ipc/wallpaper.go`: a wallpaper change keeps the palette of a fixed Ryoku
  Settings theme instead of re-deriving colours from the image (the theme lock at
  `~/.config/ryoku/theme.json`). Wallpaper-driven themes are unaffected.

### Fixed
- `ipc/wallpaper.go`: a live wallpaper could vanish a while after being set and
  revert to the previous one. `stopLive` fired an async `pkill` and relaunched at
  once, and `mpvpaper -f` forks, so a just-launched instance was invisible to the
  next kill: instances leaked, and a leftover one still playing an earlier
  wallpaper reclaimed the background layer. `stopLive` now waits for every
  mpvpaper to actually exit (escalating to SIGKILL), and a launch waits until the
  new instance is really up (its ipc socket appears) before returning, so exactly
  one ever plays and a stale one cannot win. Setting an image over a video clears
  the video reliably now, too.
- `ipc/wallpaper.go`: the "pause live wallpaper when covered" toggle only paused
  for a fullscreen window, so a wallpaper hidden behind ordinary tiled windows
  kept playing at full tilt. It now pauses whenever the desktop is covered on
  every monitor, reusing the same `desktopVisible` coverage test the widget layer
  parks itself on, and reconciles on the window and workspace events that change
  coverage (`affectsCoverage`, shared with the widget gate) rather than only on
  fullscreen toggles.
- `quickshell/launcher` RyoTunes: a batch of fixes so the built-in music finally
  feels native and fast, all in the engine (`Singletons/Radio.qml`) and its `@`
  provider (`providers/media/ytmusic/YtMusic.qml`).
  - **Playback no longer stops a few seconds in.** The "graceful yield" paused our
    own stream whenever *any* other MPRIS player was playing, so a background
    browser tab silenced the music ~4s after it started. The yield is now
    audio-focus: it fires only when another player *transitions* into playing while
    ours is (a deliberate hand-off), never for audio already playing when the engine
    came up (a primed baseline). A player we take over on an explicit play stays a
    baseline, so it never bounces the music straight back.
  - **Play is near-instant, not a ~2s stall.** mpv resolved each track with yt-dlp
    (~1.9s) before the first sound. The provider now pre-resolves the top hit's
    direct audio URL in the background (`prewarm`) as results land, and `play()`
    hands mpv that URL, so the resolve overlaps read time (first sound ~0.16s warm
    vs ~2.1s cold). The videoId rides a `#ryt=` URL fragment (never sent to the
    server) so shuffle and reload-adoption still recover it from the opaque stream
    URL. Raw InnerTube `/player` is not a faster path: YouTube gates playback behind
    PoToken/attestation, which yt-dlp maintains.
  - **Adopted and radio covers are square, not a cropped 16:9 thumbnail.** A
    reloaded stream was adopted as skeletons carrying only a 16:9 `ytimg` frame, and
    radio `/next` dedup'd away the square version of an already-queued track. The
    extend handler now upgrades a skeleton in place with the square cover and clean
    title, and adoption enriches from the current track so the cover lifts within ~1s.
- `quickshell/pill` link: the Bluetooth row and drill-in no longer present a
  dead toggle when bluetoothd is gone. The toggle hides without an adapter, the
  device list line becomes "Service off -- tap to start" (`pkexec systemctl
  enable --now bluetooth.service`, with a transient failure line), the row
  subtext says "Service off" instead of the stray German "Aus", and enabling
  from an rfkill-blocked state unblocks first (`rfkill unblock bluetooth`,
  seat-writable via systemd uaccess), powering the adapter when the unblock
  lands (`setAdapterEnabled` in `LinkBt.qml`, reused by the Link row toggle).
- Wallpaper colours no longer inherit a previous image's tune. `ipc/wallpaper.go`
  `tuneArgs` applies the ryowalls palette tune only when it is keyed to the
  current wallpaper (an `image` match), so a Super+W cycle or a different image
  falls back to default extraction. A green wallpaper is no longer themed with a
  stale complementary (magenta) palette left from an earlier tuning session.
- `quickshell/launcher`: three launcher features shipped wired to tools that no
  package set installed, so they silently did nothing on a real machine.
  `system/packages/base.packages` now ships them and `tests/shell-tool-availability.sh`
  gates all three so the gap cannot reopen.
  - Calculator: `libqalculate` was never packaged, so `ryoku-cmd-calc` fell back
    to a Python evaluator that only did `+ - * / // % **`. `2^10`, `sqrt(16)`,
    `sin(0)`, `15% of 200`, `pi`, `4+3x43`, and units/currency returned nothing.
    Ships `libqalculate` (qalc is the primary path, now gated on its exit code so
    input it cannot parse falls through instead of printing a garbage unit
    string) and hardens the AST-safe fallback to also handle `^` as power,
    `X%`/`X% of Y`/`A +/- B%` percentages, the constants `pi`/`e`/`tau`, and an
    allowlist of `math` functions, so the calculator works even without qalc. The
    script also normalizes hand-typed multiply and divide (`x`/`X` between
    operands, `×`, `÷`, ` of `) so `4+3x43` reads as `4+3*43`. `lib/dispatch.js
    looksNumeric` now routes an unprefixed `sqrt(16)`, `(1+2)*3`, `.5`, or `-3` to
    the calculator (it stays false for app names like `route66`).
    `providers/calc/Calc.qml` no longer starves its own debounce when another
    provider re-runs the results binding. Covered by `tests/calc-eval.sh` (which
    stubs qalc to force the fallback path) and expanded `dispatch.test.mjs` cases.
  - Music: `mpv-mpris` was never packaged, so the YouTube Music mpv stream never
    appeared as an MPRIS player and neither the now-playing card nor the transport
    verbs could see or control it. Ships `mpv-mpris` (autoloaded from
    `/etc/mpv/scripts`). `providers/media/mpris/Mpris.qml matches()` no longer
    leaks the now-playing row into unrelated searches (a substring test against the
    joined keyword list matched any query containing a common letter). `YtMusic.qml`
    stops racing the previous mpv when starting a new stream, and `NowPlaying.qml`
    gains working prev/play-pause/next transport controls. Ships `songrec` for the
    Recognize Music action. `docs/launcher.md` drops the stale socat claim.
  - Rest card: the clock and weather glance rendered weather as text with no icon
    and an unbalanced right column. `RestDashboard.qml` is redesigned with a vector
    weather glyph (new `WeatherGlyph.qml`, sharing the pill's glyph paths), the
    temperature, condition and city, and today hi/lo, balanced against the clock,
    with a clean date-only fallback while weather is still fetching.
- `quickshell/launcher` now-playing: the YouTube Music stream now yields when
  another player starts. `YtMusic.qml` watches MPRIS while its mpv is streaming
  and stops the moment a different player (identity not `mpv`, so Spotify, a
  browser tab, any app) begins playing, so two streams never overlap and the
  card follows whatever is actually playing. The watcher runs only during
  playback, so it costs nothing at rest.
- `quickshell/launcher` now-playing: the cava wave backdrop no longer flickers
  or leaves the analyser running when nothing plays. Its visibility follows the
  fade rather than the per-frame path (an empty path just draws nothing), each
  band keeps a small floor so the curve does not collapse between beats, and the
  seekbar now advances: MPRIS never pushes `position`, so `NowPlaying.qml` polls
  it every 500ms while playing, with elapsed and total time labels flanking the
  transport and the fill gliding between polls.
- `quickshell/launcher` Audio Visualizer action: fired the plain `visualizer`
  verb, which only flips the desktop visualiser's enabled flag while it sits on
  the bottom layer behind every window, so a maximised window hid any change. It
  now fires `visualizer-overlay`, which raises the visualiser over the windows
  (and enables it), so the action actually shows it.
- `quickshell/ryoshot`: the screenshot key (`Super + S`) silently stopped working
  after a **Save** from the toolbar. Save grabbed the shot to the auto-path, then
  ran `kdialog` to pick a destination - but `kdialog` is a KDE tool that Ryoku
  (Hyprland) does not ship, so the `Process` failed to *start*, and a process that
  never starts never fires `onExited`. `dialogMode` stayed `true` forever; the
  per-monitor overlays are `visible: !dialogMode`, so the surface went invisible
  while the instance stayed alive holding `/tmp/ryoshot.lock`. The keybind's
  `flock -n` then turned every later `Super + S` into a silent no-op. The save
  picker now runs through `sh -c "zenity ... || kdialog ..."` (zenity is already a
  dep; kdialog kept as fallback), matching `ryovm`'s `ImportDialog`. Because the
  `sh` wrapper always starts, `onExited` always fires, so a missing or cancelled
  picker drops `dialogMode` and returns to the editor instead of wedging.
- `quickshell/pill`: an app launched from the pill launcher (notably
  Discord/Electron and Vivaldi/Chromium) sometimes came up un-typeable until you
  moved it to another monitor or reopened it. The pill is a full-screen
  `WlrLayer.Overlay` that is always mapped, and its idle keyboard focus was
  `OnDemand`, so when a typing surface closed `Exclusive` -> `OnDemand` the pill
  held the keyboard instead of releasing it to the new window. It now uses
  `keyboardFocus: None` whenever no typing surface is open (the voice surface
  included), `Exclusive` only while a search field is up. Same fix as the
  desktop-widgets layer below.
- `deploy.sh` now carries `keyboard.lua` across a redeploy (added to the preserve
  list beside `user.lua`, `settings.lua`, and the generated drop-ins), matching
  `ryoku materialize`'s seed handling. A `ryoku deploy` was overwriting a
  hand-edited keyboard layout back to the `us` default, the same regression
  `materialize` had.
- `quickshell/plugins`: a desktop plugin's `plugins.json` could be blanked to an
  empty file when two writers landed at once (a drag committing while Settings
  toggled), because every write shared one `$f.tmp`. Each write now uses a unique
  temp and an atomic rename, and both `ryoku-plugins-place` and `discover.sh`
  treat a missing, empty, or corrupt file as `{}` and self-heal it - so one bad
  write no longer blanks the installed list and breaks the plugin store.
- `hyprland/scripts/stash-install.sh`: a `.deb`/`.rpm` whose desktop `Exec` is an
  absolute path (every native package ships one, e.g. `/opt/Termius/termius-app`)
  now resolves onto the extracted tree at that exact path. It used to rewrite the
  `Exec` by searching the whole payload for the basename and taking the first hit,
  which matched an unrelated same-named file when one sorted first: Termius ships
  an `etc/cron.daily/termius-app` cron script, so the launcher ran the cron job and
  the app never opened. Covered by `tests/stash-install.sh`.
- `shell/deploy.sh`: the Hyprland config swap is now near-atomic, so a reload can
  never catch `hyprland.lua` missing. It built `~/.config/hypr` with `rm -rf` then
  `cp -a`, leaving a long window with no `hyprland.lua`; a manual reload or a fresh
  login in that window (both bypass the autoreload pause) tripped emergency mode
  and a stale "cannot open hyprland.lua". It now stages the config in a sibling
  dir, carries the preserved user files and generated drop-ins across, then renames
  staging into place; it also touches the entry, since `cp -a` carried the repo's
  older mtimes and an mtime-watching autoreload could otherwise miss the swap.
- `quickshell/widgets`: opening an app on an empty workspace now focuses it. The
  desktop-widgets layer (full-screen `WlrLayer.Bottom`) requested
  `keyboardFocus: OnDemand`, so on a workspace with no window above it that layer
  held the keyboard and a freshly launched window (terminal keybind or app
  launcher) stayed unfocused until you moved the mouse or hit a focus bind. It now
  uses `keyboardFocus: None`; pointer input (widget drag, right-click desktop
  menu) is unaffected, since layer-shell gates clicks by the input region, not
  keyboard interactivity.
- `quickshell/pill` LocalSend receive: incoming transfers now require consent.
  The receiver auto-accepted any device's `prepare-upload` and dropped the bytes
  straight into the stash. It now holds each offer at `prepare-upload`, shows
  "‹device› wants to send ‹N› file(s)" with Accept/Decline in the receive sheet,
  and issues an upload token only once you Accept (Decline or 60s of silence
  returns 403, so declined bytes never cross the wire). The shell answers over
  the receiver's stdin; the Python program now loads from fd 3 to leave stdin
  free for that channel. Covered by `tests/localsend-receive.sh`.
- `quickshell/pill` stash install: the control deck now steps aside for the sudo
  prompt. Installing a pacman package shells out to `pkexec`, whose polkit window
  (`hyprpolkitagent`) landed behind the deck's overlay layer and could not take
  the password (the deck holds an exclusive keyboard grab), forcing a close-deck,
  type-password, reopen dance. `stash-install.sh` emits an `@AUTH` marker before
  the privileged step, the stash reads it live and the pill dismisses the deck so
  the prompt takes focus, and a window rule floats and centres the
  `hyprpolkitagent` prompt. Covered by `tests/stash-install.sh`.
- `quickshell/visualizer` no longer pins a CPU core and overheats the machine on a
  high-refresh panel. It ran a `FrameAnimation` once per vsync (re-rendering 96
  bands plus the bloom at 165Hz though cava only feeds 60fps), and the `wave` style
  rasterised a full-width filled curve on a software `Canvas` on the main thread.
  Now a Timer caps updates to ~60fps while sound plays and ~30fps for the idle wave
  (and stops entirely when silent), the wave renders as a GPU `Shape` instead of a
  `Canvas`, and the bloom skips its pass when the spectrum is flat. Wave-style CPU
  fell from ~85% of a core to ~10% and the package temperature from ~95°C to ~67°C
  on a 165Hz panel.
- `deploy.sh` no longer trips a live Hyprland session into emergency mode when run
  from outside the session (ssh, an agent, or the curl recovery, which also calls
  it). The config swap pauses Hyprland autoreload first, but that pause was gated
  on hyprctl being reachable, which it is not when `HYPRLAND_INSTANCE_SIGNATURE` is
  absent from the environment, so the brief `rm`+`cp` window where `hyprland.lua`
  is missing got caught by autoreload. deploy now recovers the running instance
  signature from the runtime dir, so the pause happens whenever a session is up.
- `quickshell/pill`: an auto-hidden island no longer collapses while the cursor
  travels onto its buds (the music/update island and the activity strip). The
  reveal expands the pill, and a bud's x tracks the pill width, so hovering a bud
  collapsed the pill (the music island actively suppresses the pill hover), which
  slid the bud out from under the cursor and hid the island before it could be
  used. A hovered bud (`satelliteHover`) now freezes the pill's expand state, so
  it neither collapses nor slides the bud away and the reveal stays put.
- `quickshell/pill` bar: the bar-mode surface now closes by fading its content
  out against the still-opaque panel, then fading the empty panel, so the busy
  desktop behind never shows through a half-faded surface as a doubled overlay.
  The open and resting looks stay the fused melt; only the bar close changed.
- `quickshell/pill`: hover works again across the whole island. The neck/reveal
  hover zone sitting in front of the pill (added so crossing the tray icons would
  not collapse the island) was a covering sibling holding a `HoverHandler`, which
  swallowed hover from every surface beneath it, so stash tiles, action buttons,
  device rows and the like never lit or revealed their hover actions. Island hover
  is now read by a `HoverHandler` on the pill itself (an ancestor of the surfaces,
  so it never blocks their own hover) OR'd with a neck-only zone that no longer
  overlaps the body.
- `deploy.sh` preserves the user's own and per-machine generated Hyprland files
  across a redeploy. The config swap still replaces the shipped base, but now
  restores `user.lua`, `monitors_user.lua`, `settings.lua`, `theme.lua`,
  `monitors.lua`, and `gpu.lua` from the backup afterwards, so a dev redeploy (or
  `ryoku update` on a checkout) never resets your settings, theme, display layout,
  or GPU pin, matching `ryoku materialize` on a packaged install.
- `quickshell/pill` update island: re-check `ryoku status` on a steady cadence
  (every 5 min) instead of only once at startup, so the island reliably surfaces
  updates that appear during a session and recovers if the first check came back
  empty.
- `deploy.sh` now also installs `ryoku-monitor` alongside the other hardware
  helpers, so the dev loop gets the current version (with the `list`/`apply`/
  profile commands the Displays settings need) instead of a stale package copy.
- `deploy.sh` now builds and installs the real Go `ryoku` CLI (`ryoku/cli`) as the
  `ryoku` command, replacing the old `ryoku/shell/ryoku` dev script (removed). The
  dev mirror now runs the production update CLI (`ryoku status`/`update`), so the
  Hub and island read real data; the dev redeploy is now `ryoku deploy` (the old
  `ryoku update` meant "deploy the mirror"; `ryoku update` is now the real pacman
  system update).
- `deploy.sh` clears any orphaned shell surfaces (`qs -c pill`/`sidebar`/
  `visualizer`) before it restarts the daemon. A crashed or quit daemon left them
  running holding their single-instance lock, so the freshly restarted pill could
  not start and the new daemon died with it, leaving a dead shell after a
  `ryoku update` or `ryoku deploy`. The restart now always comes up clean.

### Added
- `plugin/` (`Ryoku.Blobs`) and `quickshell/pill`: the frame border casts a soft
  contact shadow inward for depth. `BlobGroup` gained `shadowStrength`/`shadowSize`;
  the SDF shader draws a falloff just inside the border (0.5 / 26px), gated to the
  border so the pill and popouts, being the frame swelling open rather than panels
  on top, cast no shadow of their own.
- `quickshell/pill`: a workspace switcher overview grown from the pill centre
  (`Super + Tab`, `ryoku-shell workspaces`). A filmstrip of this monitor's
  workspaces, each a scaled mini-map that draws its windows where they actually
  sit as icon cards (off-workspace windows are unmapped in Hyprland and cannot be
  live thumbnails, so a faithful card layout stands in). Click a window to focus
  it, click a tile to switch workspaces, drag a window onto another tile to move
  it there, or drop it on the trailing `+` tile to send it to a fresh workspace;
  the active workspace and the current drop target carry the brand accent. Window
  icon resolution moved to a shared `Singletons/Apps`, so the minimized tray and
  the switcher resolve icons through one place instead of two copies.
- `quickshell/pill`: an update island on the top-right of the frame. When a newer
  build is available it shows a compact chip (a brand download glyph, the target
  version, and the count of pending commits) that opens the Hub's Updates section.
  While an update runs it mirrors the Hub's progress as a Ryoku wave, and on
  success becomes a Refresh shell button (`ryoku-shell reload`) so the update can
  be applied from here too. The run state is read from a small runtime file the
  Hub publishes; availability is still mock in `Singletons/Updates.qml`.
- `quickshell/pill`: a voice dictation surface, toggled with ``Super+` `` (tap to
  start, tap to stop). `ryoku-shell voice` flips Handy's transcription and
  grows a centre-island Ryoku wave driven by the live microphone (`VoiceBars`
  runs cava on the default input): flat while silent, swelling into highs and
  lows as you speak. The surface is non-focus-grabbing, so Handy types the
  transcription into the focused app rather than the pill. The pill's tray hides
  Passive StatusNotifier items (per spec), so Handy, run `--no-tray`, stays out of
  the island instead of churning the hover row and flickering it.
- `quickshell/pill`: a dedicated 力 INBOX surface for notifications, opened by the
  pill's bell icon. Notifications group per app with expandable stacks, critical
  entries flagged, an empty IDLE state, and clear-all. The bell used to open the
  LINK surface with the notification inbox buried under the connectivity rows.
- The Ryoku desktop shell, imported and reorganized into this tree: the Quickshell
  UI (`pill` bar, `sidebar`, `ryoshot`), the Hyprland
  config in Lua, wallust palette generation, and the per-app configs.
- `plugin/` and the pill-shell frame: the screen frame, brought from the legacy
  shell and reorganized. `plugin/` is `Ryoku.Blobs`, a C++/QML SDF metaball
  module: a rounded border (`BlobInvertedRect`) and bodies (`BlobRect`) that melt
  into one shared smooth-min field, with a velocity spring that squashes them as
  they move. The pill shell hosts the field per monitor: a click-through rounded
  border in Hyprland's outer gap (retracting to nothing on fullscreen), the pill
  itself as a top blob necked into the border (the island reads as the frame
  swelling open at top-centre), and edge popouts under `pill/popouts/` (the mixer
  left, power right) that grow out of the centre-left/right border on hover and
  melt back into it; the music and activity islands stay on a separate field. See
  docs/frame.md. `deploy.sh` builds the module (cmake + ninja + qt6-shadertools)
  onto `~/.local/lib/qt6/qml` when that toolchain is present (skipping cleanly
  otherwise), and `ryoku-shell` points the quickshell processes'
  `QML2_IMPORT_PATH` there. The module ships prebuilt, like the Go binaries.
- `ipc/`: `ryoku-shell`, a single Go program that is the shell's control plane.
  `ryoku-shell daemon` supervises the Quickshell components (restarting them if
  they exit), brings up the clipboard-history watchers and the wallpaper, and
  serves one Unix socket. `ryoku-shell <command>` is the client the Hyprland
  keybinds call. It resolves the active monitor itself and fans out to the
  Quickshell IPC, the wallpaper daemon and wallust, and qylock for the lock.
- `dev-run.sh`, `dev-stop.sh`, `dev-binds.sh`: run the shell from this checkout on
  a live Hyprland session via `RYOKU_SHELL_DIR` (`qs -p`), with quickshell
  hot-reload, so it can be developed without installing anything.
- `README.md`: documented the shell's runtime dependencies and how to run it live.
- `deploy.sh`: installs this tree into `~/.config` (one way; the repo is the
  source) and `ryoku-shell` onto `PATH`. Pauses Hyprland auto-reload across the
  `~/.config/hypr` swap so the missing-file window cannot trip emergency mode;
  `--no-reload` stages the files for the next login.
- Hyprland autostart now launches `ryoku-idle start`; that helper starts
  `hypridle` on laptops only.
- The Super+D screen toolkit gained a Caffeine tile: a coffee-glyph toggle that
  holds `Flags.keepAwake` (and thus the pill and sidebar `IdleInhibitor`) on until
  it is turned back off, so the screen never dims or locks. Unlike the launcher
  tiles it flips in place and stays lit warm while active.
- Keep-Awake now survives a shell reload/restart. The durable idle inhibitor runs
  as an external `systemd-inhibit --what=idle:sleep` process outside the shell's
  process tree (`hyprland/scripts/ryoku-cmd-caffeine`, launched via `systemd-run
  --user` with a `setsid` fallback), so respawning the pill no longer drops it and
  the screen can't sleep during the swap. The in-shell Wayland `IdleInhibitor`
  still provides immediate effect; the pill bridges any `Flags.keepAwake` change to
  the helper (`start`/`stop`) and reconciles on startup. The helper persists the
  request to `~/.local/state/ryoku/caffeine.enabled` and exposes
  `start/stop/restore/hold/release/toggle/status`.
- `quickshell/pill`: a Utilities surface grown from the pill centre (Super+U), the
  legacy bottom-right panel reworked as a centre island. Keep-Awake with a live
  elapsed counter (shared `Flags.keepAwakeSince`), a Screen Recorder card with a
  record-mode dropdown (display / region / +sound) and running controls
  (pause/stop, elapsed, REC pulse), quick toggles (wifi / bluetooth / mic / DND /
  night light via hyprsunset),
  and a recordings list (with file sizes) and play / open-folder / trash. Recording is driven by
  the `Recorder` singleton (`ryoku-cmd-screenrecord`: gpu-screen-recorder with a
  wf-recorder fallback).
- `ipc/wallpaper.go`: after wallust regenerates the palette, call `ryoku-leds`
  so OpenRGB-compatible keyboards and lighting devices follow wallpaper changes.
- `ryoku`: lightweight live-mirror CLI. `ryoku update` refuses dirty repo state,
  pulls with `--ff-only`, deploys the shell and Hyprland config, reloads Hyprland,
  and restarts `ryoku-shell`.
- `quickshell/pill`: ported ActivSpot's live-activity functions as Ryoku-native
  islands. A left strip of automatic status chips (screen recording, Discord
  voice call, WireGuard tunnel) folds open beside the pill while each state is
  live, and a separated album-art/CAVA music island sits to the right that
  expands its own transport controls on hover without resizing the main pill.
- `quickshell/pill`: the REC activity chip stops the recording on click. Its dot
  squares into a stop icon on hover, so the chip reads as a control rather than a
  readout, and the recording can be ended without opening the Utilities surface.
- `quickshell/pill`: three more native surfaces grown from the pill. A SYSTEM
  card (`SysInfoSurface` + `Singletons/SysInfo`) reads user@host, distro, kernel,
  CPU/GPU, memory and disk meters, uptime and packages; a file STASH
  (`StashSurface` + `Singletons/Stash` + `hyprland/scripts/localsend.sh`) is a
  drop-target grid over `~/Downloads/Stash` that sends any file to a LAN peer over
  LocalSend; and current weather (`Singletons/Weather`, from wttr.in) shows in the
  calendar surface and the hover clock. The card and stash open from new hover-row
  glyphs, and the stash also rides the activity strip as a live chip.
- `quickshell/pill`: the STASH gains an action rail down its left edge: send the
  whole stash over LocalSend, install dropped AppImages and tarballs into the app
  launcher (Super+Space), compress videos and images through ffmpeg, and pull
  media in from the clipboard with yt-dlp. Adds `StashRail`, `StashTaskOverlay`,
  the `send`/`install`/`compress`/`download` `GlyphIcon` glyphs, `Singletons/Stash`
  actions, and the `hyprland/scripts/stash-install.sh`, `stash-compress.sh`,
  `stash-download.sh` helpers (plus a `send-all` mode on `localsend.sh`) behind them.
- `quickshell/pill`: a TOOLKIT centre island (Super+D) of screen tools that grow
  from the pill and run self-contained `hypr/scripts` helpers: Google Lens (upload
  a region and open the search), a color picker (hyprpicker to the clipboard), OCR
  (tesseract on a region to the clipboard), a webcam Mirror (a flipped mpv
  picture-in-picture, floated and toggled), and a QR scanner (zbar on a region,
  copying the result and opening URLs).

### Changed
- `quickshell/pill`: the media player's seek line is the Ryoku wave now, a uniform
  sine ripple with a dim track and a bright played crest, matching the WaveMeter
  signature instead of the damped brush stroke it used to draw.
- `quickshell/pill`: the mixer is audio and display faders only. The DND and
  Keep-Awake chips moved out (they already live on the Utilities centre island),
  and each fader now shows its level at rest instead of only on hover.
- `quickshell/pill`: the LINK surface is connectivity only (Network, Bluetooth).
  Its notification inbox moved to the new INBOX surface: the bell icon opens that,
  the wifi icon opens LINK.
- `quickshell`: Keep-Awake shows one icon everywhere (the coffee glyph). The
  Utilities toggle and the sidebar quick toggle dropped their eye and clock glyphs
  to match the Toolkit Caffeine tile.
- `ryoku-cmd-screenrecord`: starting a recording no longer raises a "recording
  started" toast. The REC chip on the pill's activity strip is the live indicator,
  so the toast was redundant noise; the stop and failure notifications stay.
- `quickshell/pill`: the Stash tiles show a file-type glyph (archive, image, film,
  music, code, document) instead of a large extension label, and the empty state
  is a faint 力 watermark over a minimal prompt, for a less templated look.
- Relocated from the top-level `shell/` to `ryoku/shell/` as part of folding the
  whole desktop into one `ryoku/` tree. The Hyprland config moved to
  `ryoku/hyprland` (the single Hyprland config); the duplicate `fish` was dropped
  for the base `ryoku/apps/fish`.
- Replaced the per-component daemon and toggle shell scripts with the Go IPC: the
  `*-daemon.sh` watchdogs, `cliphist-watch.sh`, and the `launcher`/`sidebar`/
  `clipboard`/`link`/`lock`/`wallpaper`/`wallpaper-picker` scripts are gone. The
  keybinds (`binds.lua`), autostart (`autostart.lua`), and the QML that ran those
  scripts (the power menus, the wallpaper picker) now call `ryoku-shell`. Only the
  two leaf thumbnailers the UI invokes directly remain under `hypr/scripts/`.
- De-branded the import: no upstream name, attribution, or credits; `torii` ->
  `ryoku`, `rishot` -> `ryoshot`, and the matching file and directory renames.
  Removed the em-dashes from the QML display strings (the regex keeps splitting on
  one via an escape).
- Dropped the shell's own lock component; qylock (shipped by `ryoku/`) stays the
  lock, and `ryoku-shell lock` launches it.
- Standardized the terminal and file manager on `kitty` and `nautilus` (what
  `ryoku/` ships): `binds.lua`, the `window_rules.lua` float rule, the wallust
  template (a `kitty` palette now), and the README; removed the `ghostty` config.
- Replaced the import's machine-specific values with portable, hardware-managed
  ones: dropped the hardcoded dual-monitor layout, the German keyboard, and the
  `DP-1`/`HDMI-A-1` -> workspace mapping in the pill and topbar `Workspaces.qml`
  (a monitor-agnostic fixed range now). `hyprland.lua` requires the managed
  `gpu`/`keyboard`/`monitors` and runs `ryoku-gpu`/`ryoku-monitor` from autostart,
  as `ryoku/` does. Fixed the leftover `/home/erik/...` paths, and made `fish`
  match the base (greeting off, `~/.local/bin` on `PATH`).
- Reworked the keybinds: `SUPER+Q` closes, `W` cycles the wallpaper, `B` opens
  chromium, `A`/`SHIFT+A` float (compact) / tile (restore) the window, and `S`
  takes a ryoshot screenshot; dropped the SUPER-tap launcher and `SUPER+T` float.
  `SUPER+[1..0]` focus workspaces, `SUPER+SHIFT+[1..0]` move the window there.
  `SUPER+N` opens Neovim, `SUPER+ALT+E` opens yazi; `EDITOR`/`VISUAL` are nvim.
- `binds.lua`: Super+Z opens the file stash.
- `input.lua`: matched the upstream Ryoku input, `sensitivity` 0, no explicit
  `accel_profile` (libinput's adaptive default), `touchpad.natural_scroll` false,
  and hardware cursors. The shell's reversed scroll and a positive sensitivity
  were what felt non-native.
- `monitors.lua` seed uses `highrr`, so a panel comes up at its top refresh
  (165Hz here) instead of the EDID-preferred 60Hz.
- Kept `ryoku/`'s branded `ryoku-fastfetch` as the terminal readout: dropped the
  shell's wallust fastfetch template and `fastfetch/` dir, so wallust no longer
  overwrites `~/.config/fastfetch/config.jsonc`. wallust themes the kitty palette
  and Hyprland colors only.
- The shell reads wallpapers from `~/Pictures/Wallpapers` (the XDG Pictures home,
  was `~/Ryoku/wallpapers`); the random picker, the picker strip, and the
  thumbnailer accept `.webp` alongside `.jpg`/`.jpeg`/`.png`.
- Removed the orphaned `brave-theme/`: the shipped browser is chromium (`Super+B`)
  and the theme was deployed by neither the dev nor the install path.
- `ipc/wallpaper.go`: Super+W now cycles nine hand-tuned awww transitions
  (`fade`, `wipe`, `wave`, and the `grow`/`center`/`outer`/`any` circle reveals)
  picked at random and never repeated back-to-back, instead of the one fixed wave.
  All share one slow, smooth speed (a single `--transition-duration`/`--transition-fps`
  appended to every preset), so only the shape varies. Border colors follow the
  wallpaper via `hyprctl reload config-only` (Hyprland's new parser rejects runtime
  `keyword`, and `config-only` leaves the monitors untouched).
- `quickshell/pill`: the STASH signs itself with a `WaveMeter` house mark under
  the header that fills on open, dropping the Ame bead that used to dock in its
  centre; the surface is wider and taller so the rail and a two-row grid have room.

### Fixed
- `quickshell/pill`: the workspace wave under the clock no longer leaks memory. It
  animated a software `Canvas` at 30fps forever, even while the pill was auto-hidden
  (the pill hides by opacity, not visibility, so the wave stayed "visible" and kept
  ticking), accruing the same idle-`Canvas` leak already fixed in `WaveMeter` and the
  visualiser (~1.2 MB/min here, GBs over a day's uptime). The wave is now a static
  Canvas: it repaints only on a workspace/focus/size change and only while the pill
  is shown. The focus crest still glides on switch, it just no longer shimmers at rest.
- `quickshell/pill`: the island stays open while hovering the tray/minimized-app
  icons. The hover zone that drives `pill.hovered` sat behind the pill, so the
  icons' own `hoverEnabled` MouseAreas swallowed the hover and the island
  collapsed the moment the pointer reached one; the zone now sits in front of the
  pill. It is handler-only (a passive `HoverHandler`), so it reports the hover
  without blocking the icons' clicks or their own hover highlight.
- `quickshell/pill`: surface content rides the blob morph instead of fading on its
  own. `PillSurface` faded content over a separate, shorter timeline than the
  pill's size morph, so on close the content ghosted out while the shape closed;
  its opacity now animates on the morph's exact duration and curve, growing and
  shrinking with the island.
- `quickshell/pill`: the shell fully hides while a window is fullscreen. The
  frame, pill, music island, and edge popouts stayed drawn over fullscreen
  content; the whole shell layer now gates on the active workspace's fullscreen
  state (read from the typed `HyprlandWorkspace.hasFullscreen` rather than the
  workspace's last IPC object, which the monitor refresh could clobber).
- `quickshell/pill`: the music island's spectrum bars no longer animate without
  audio behind them. When cava sends no frames they settle to a flat resting line
  instead of a synthetic wave, so the bars read as real playback levels or rest
  flat, the way VoiceBars already treats the mic.
- `quickshell/pill`: the media player surface is reachable from the UI. Tapping
  the now-playing music island opens it; the tap was wired to the file stash, so
  the media surface had no entry point other than the IPC command.
- `quickshell/pill`: Keep-Awake now holds across a shell reload. The in-process
  Wayland `IdleInhibitor` dies with the pill on every respawn, so a durable
  `ryoku-cmd-caffeine` systemd-inhibit (launched outside the shell) bridges the
  swap; toggling any surface still just flips `Flags.keepAwake`.
- `quickshell/pill`: the Recorder detects gpu-screen-recorder by full command line.
  Linux truncates the process comm name to 15 chars, so `pgrep -x
  gpu-screen-recorder` never matched and a live recording read as stopped.
- `quickshell/pill`: the Recorder runs `ryoku-cmd-screenrecord` by its full path.
  `~/.config/hypr/scripts` is not on the shell's PATH, so the bare name never
  resolved and the Record button silently did nothing.
- `quickshell/pill`: the mixer and power edge popouts close on hover-leave. The
  close spring was underdamped enough to spring the body back open past flush,
  re-triggering the hover and sticking the popout (power especially) open; the
  close now eases out with no overshoot.
- `quickshell/pill`: the activity-strip chips (REC stop, stash) now receive hover
  and clicks. The strip rides left of the pill, outside the pill's input mask, so
  its region was never grabbed and clicks fell through to the window behind; the
  mask now covers the strip's bounds, like the music island and edge popouts.
- `ipc/wallpaper.go`: resolve a symlinked wallpaper directory (`EvalSymlinks`)
  before scanning, so `wallpaper next` and the picker work when
  `~/Pictures/Wallpapers` links to a collection elsewhere.
- `quickshell/ryoshot`: create `~/Pictures/Screenshots` on launch; it did not
  exist, so the screenshot grab failed and copy/save silently did nothing.
- `quickshell/ryoshot`: de-branded the selection label (dropped the leftover
  torii glyph; it now reads `ryoshot · WxH`).
- `quickshell/pill`: music track changes no longer open the main pill as a media
  OSD; main and music islands use their own rounded-shape hover masks, so workspace
  dots stay reachable and the separated music island owns its compact controls.
- `quickshell/pill`: the separated music island's close animation no longer pops.
  `scale` was derived from the animated `reveal` but carried its own `Behavior`
  (a ~220ms lag), and opacity was a constant, so the bubble vanished mid-shrink;
  it now fades with `reveal` and scales directly, retracting cleanly behind the pill.
- `ipc/wallpaper.go`: Super+W no longer lags. The retheme (wallust palette, the
  Hyprland reload, and the OpenRGB LED pass) ran synchronously under the wallpaper
  lock, so every press blocked on the multi-second `ryoku-leds`/OpenRGB device scan
  and presses serialized behind it. The keybind now only fires the transition and
  returns (~150ms); the palette+border reload and the slow LED pass run on
  coalescing background workers, so rapid presses stay smooth and the settled
  wallpaper still themes once.
- `quickshell/pill`: removed the island's on-hover wave animations. Both the
  per-icon hover underline (a crossfading trail of orange marks across the status
  row) and the island-open wake-wave streak read as a glitchy wave line on hover;
  both are gone (WakeWave.qml and its latch deleted), and the icons just brighten
  on hover. The hover content clips to the pill and fades in as the island opens,
  so it loads immediately instead of staying blank until the morph settles.

### Not included
- The GRUB theme (the system boots with Limine) and the SDDM theme (a 38 MB
  third-party video, and the login screen is qylock). Bring either in later if
  wanted.
