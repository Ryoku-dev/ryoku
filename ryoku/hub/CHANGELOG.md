# Changelog: ryoku/hub/


### Added
- **The Machine page owns the two switches it used to describe.** The
  hardware display-routing knob (GPU Mode / MUX / Optimus) and the live CPU
  power profile were CLI-only, and both the render card's Hybrid and the
  firmware's Hybrid read as one setting with two names. The render card now
  carries a "Display wired to" segment (a pkexec grant makes the firmware
  write terminal-free; a reboot-pending flag says when it lands) and the CPU
  card a "Live profile" segment that switches through the shell daemon, the
  one owner of the pick. Each row names its layer: a software choice applied
  at next login, versus a hardware switch applied at next reboot
  (`quickshell/pages/GpuPage.qml`, `backend/gpumux.go`, `backend/daemonclient.go`).

- **Keybinds, rebuilt around use.** A search field that fuzzy-matches labels,
  hints, categories and key tokens ("clw" finds Close window, "num 3" the
  number-pad workspaces), a category rail with counts, and one calm column of
  36px rows: a short label, the key caps, a hint only for the row under the
  pointer. A shortcut the running compositor cannot do stays listed, greyed,
  with the reason a hover away. Rebinding clicks the caps and captures the
  chord through a shortcut inhibitor, so the compositor hands the keys to the
  Hub instead of running them, on niri as on Hyprland; the number pad records
  as `Num 1` whatever NumLock says (`pages/KeybindsPage.qml`, `Combos.js`).
- **The legend comes from the provider.** `ryoku-hub keybinds` asks the running
  window-manager provider for the full effective bind list from the neutral
  catalogue, so the page and the cheatsheet show what is actually bound rather
  than a parse of one compositor's config (`backend/keybinds.go`).
- **Night light, on the Displays page.** A switch and a colour temperature that
  read and drive the daemon's `nightlight` topic, shown only where the running
  compositor's provider offers a night light (`pages/DisplaysPage.qml`).
- **Idle timeouts, on the Machine page.** Dim, lock, screen off and suspend, on
  battery and plugged in, plus a master switch and an opt-in for desktops; they
  persist as the idle policy in `power.json` and re-render the idle daemon's
  config on every change (`pages/GpuPage.qml`, `schema/GpuPage.js`,
  `backend/cputune.go`).
- **A touchpad switch, on the Input page.** Reads and flips the pad through the
  `input.touchpad` seam action, shown where the provider reports the
  `touchpadToggle` capability (`pages/InputPage.qml`).
- **The compositor pages follow the running provider.** The Animations page
  renders a provider's own animation rows (niri's per-animation springs and
  curves) beside the shell motion controls and keeps the Hyprland curve editor
  only where that store is live; the Layer Rules page renders a provider's
  layer-rule list through a shared list control built from the row's declared
  fields; the Window Rules action picker takes its vocabulary from the actions
  the provider honours (`pages/AnimationsPage.qml`, `pages/LayerRulesPage.qml`,
  `pages/WindowRulesPage.qml`, `SettingsSheet.qml`, `Singletons/Settings.qml`).

### Fixed
- **No Hyprland wording or dead compositor toggles on niri.** The search
  vocabulary derives from the active provider's rows and name, the import
  wizard names the desktop you run and stands down where it cannot read its
  config, and the Performance page hides the blur, shadow and low-power
  switches where nothing reads them (`Hub.qml`, `pages/ImportPage.qml`,
  `pages/PerformancePage.qml`).

### Changed
- **Ryoku Settings is a full-page window.** It opens at 99% of the screen (the
  Hyprland rule floats it at the same 99% and centres it; niri sizes the column),
  so the settings get the room the layout is designed for instead of a 1200px
  strip (`quickshell/shell.qml`, `hyprland/modules/window_rules.lua`).
- **One measure, and two columns of cards where they fit.** A framed page reads
  on a centred column capped at `Tokens.pageMax`, and the schema sheet lays its
  groups into two columns of cards when the measure holds them, balanced by how
  tall each group renders. The result is a page that fills its width from the top
  rather than one long column beside an empty half, with a row's control still a
  glance from its label (`quickshell/SettingsSheet.qml`, `SchemaPage.qml`).
- **Less text.** Every page description is one sentence now, and the longest row
  descriptions were cut to a line (`quickshell/pages/*`, `quickshell/schema/*`).
- **The rail breathes.** Taller nav rows, a gap between groups, and `Advanced`
  pinned under a hairline as the only rail-foot control (`quickshell/Hub.qml`).

### Removed
- **The rich decor tier, and its code.** The rail's Calm|Rich switch turned on a
  second skin: a register backdrop, a barcode rail foot with the build edition, a
  film-grain plate, an oversized title, and chapter plates filling dead grid
  cells with art. Calm was the default and the right answer, so the switch, the
  tokens behind it (`decorRich`, `showPosters`, `showGrid`, `showGrain`,
  `monoHeads`), the components (`Decor`, `Placard`, `DitherField`, `DecorStore`)
  and every call site are gone; a page has one voice now (`Ryoku.Ui`,
  `docs/ui-ux.md`, "The retired poster layer").

### Added
- **"Bar drifts when silent" on the Performance page.** Opts the bar's gap
  stream into drifting on Balanced and Saver when nothing plays, not only on the
  Performance profile. Off by default (`quickshell/pages/PerformancePage.qml`).
- **A Plugins page manages every Hyprland compositor plugin.** Settings >
  Plugins (DESKTOP, next to Windows) takes over the Windows page's Plugins tab
  and grows into the one place for compositor plugins: a tab per plugin (title
  bars, glass, image borders, cursor motion, focus flash, the new key sounds,
  and anything the user adds) with a status card above its settings: where the
  copy came from (package, built here with commit and date, hyprpm), whether it
  is running, and the verdict when it is not. A plugin is ABI-locked to the
  exact Hyprland build, and an Arch bump of aquamarine or hyprutils between two
  Ryoku releases left a copy the compositor refused with "version mismatch" on
  every reload while the toggle looked on. Every copy now carries an `.abi`
  receipt, the card says "built for aquamarine 0.14, running 0.15", and
  **Rebuild** builds it here from upstream (its hyprpm.toml pin for the
  installed Hyprland, against the installed headers), with **Rebuild stale**
  for all of them and **Docs** for each plugin's own documentation. Cursor
  motion and focus flash keep their rows on the Cursor and Animations pages and
  are borrowed here, so each row is written once (`pages/PluginsPage.qml`,
  `schema/PluginsPage.js`, `backend/hyprplugins.go`).
- **Add a plugin from any git repository.** "+ Add from git" on the Plugins
  page takes a repository with a `hyprpm.toml`, lists the plugins it declares,
  builds the ones picked on this machine and lays them beside the bundled
  ones; Save loads them. Its settings are detected: the `plugin:<ns>:<key>`
  names read out of the `.so`, typed and defaulted by the compositor once it
  loads, rendered as native controls (switch, stepper, slider, field) and
  stored under `plugins.extra.<id>.config`, so a plugin nobody in Ryoku has
  seen still gets a control for every option. **Remove** drops it again.
  hyprpm-installed plugins show on the same page (`backend/hyprplugins_build.go`).
- **`ryoku-hub hypr plugins list|rebuild|add|remove`.** The backend behind the
  page and the doctor: the roster with source, version, ABI verdict and
  detected settings; the builder (bundled plugins from upstream, keysounds from
  the checkout or the shipped source, any git repository), which writes the
  `.so`, its `.abi` and a build receipt under `~/.local/lib/hyprland/plugins`;
  and removal. `settings.lua` now loads only a copy whose receipt matches the
  running compositor and leaves a stale one out with a comment instead of
  having Hyprland refuse it on every reload, and Save unloads a plugin turned
  off (a reload only unloads what it loaded itself). See `docs/hyprland-plugins.md`.

### Fixed
- **Connections lists Bluetooth devices by name, not MAC address.** The
  Bluetooth tab read a device's name from its BlueZ alias, which BlueZ leaves
  equal to the device's own address until it learns a name, so an unnamed
  device showed a MAC even when the device-reported name was available. It now
  resolves through the shared helper, which skips an address-shaped alias
  (`quickshell/pages/ConnectionsPage.qml`).
- **A rice carries its custom reload cover (#146).** The brand layer bundled
  the mark image but left `reloadCover.path` as the author's absolute path, so
  applying a rice on another box (or after the source asset moved) wrote a path
  to a file that was never there and the shell reload fell back to the default
  cover. Capture now copies the reload-cover asset into the rice and rewrites
  the path to `rice://`, apply lands it under `rice-assets/<slug>/` like the
  mark, and a dangling `rice://` or foreign absolute path is dropped so the
  reload degrades to the default cover cleanly (`backend/rice.go`).
- **Store lock skins show in Settings again, and a read failure says so.**
  Settings > Lockscreen listed skins by walking `~/.local/share/qylock/themes`
  two levels deep for a `Main.qml`, but `os.ReadDir` reports a symlinked theme
  dir with `IsDir()` false, so a skin installed as (or under) a symlink was
  dropped and vanished though its files were on disk. The scan now follows a
  symlinked directory, and `ryoku-hub lock list` unions in a second source: the
  RyoStore receipts under `~/.local/state/ryoku/store/lockscreens/*.json`, so a
  receipt-owned product the folder heuristic misses still lists as long as its
  `Main.qml` is present. A themes dir that exists but cannot be read now returns
  an `error`, and the page tells "Couldn't read the lock skins list" (retry)
  apart from "No lock skins installed yet" (browse the Store), instead of one
  message for both (`backend/lock.go`, `pages/LockscreenPage.qml`).
- **Plugin settings apply on Save, and preview live.** `hl.plugin.load()` only
  declares a path; Hyprland loads the declared set after the config pass and
  reloads once more, and the old `if hl.plugin.<name> ~= nil` guard was only
  ever true for a plugin that registers Lua functions, so every other plugin's
  `hl.config` line never ran: key sounds kept its default profile and volume
  whatever the page said, cursor motion its default mode. The config now runs
  behind a lookup in `hl.get_loaded_plugins()` (true on that second pass),
  Save pushes it once more through `hyprctl eval` after the reload, and the
  live preview pushes it on every edit, so a profile change is heard as it is
  picked (`backend/hypr.go`).
- **Key sounds' Docs and Sounds buttons go somewhere real.** Docs opened a
  README on the GitHub `main` branch that does not carry the plugin yet; it now
  opens the README shipped beside the plugin's source on this box (the
  checkout, or `/usr/share/ryoku/hypr-plugins/keysounds`), and a new Sounds
  button opens the Mechvibes packs the profiles are cut from.

### Added
- **`ryoku-hub gpu vm` builds and runs the passthrough VM.** A new subcommand
  generates a performance-tuned libvirt domain named `ryoku-<name>` from an
  install ISO (host-passthrough CPU with vCPU pinning off the caps engine's
  topology, Hyper-V enlightenments + hidden KVM for Windows, virtio disk, the
  dGPU and its sibling functions as `managed='no'` vfio hostdevs so the existing
  hook binds them, the kvmfr 128 MiB Looking Glass shmem via `qemu:commandline`,
  swtpm, UEFI), then defines/starts/stops/removes it via virsh and launches
  `looking-glass-client` on start. Readiness reuses the caps verdict. Ryoport's
  Looking Glass lane and `ryovm lg` drive it (`backend/gpuvm.go`,
  `backend/gpudomain.go`).
- **Add-ons warns on a community plugin.** A plugin whose manifest is not
  `official: true` opens with the same warning the Store and QS Bar Settings
  print, before its placement controls (`pages/AddonsPage.qml`).

- **Desktop > General gains managed shell-reload media.** Preview an image,
  animation, or muted video; use Default or Add Asset with visible format
  guidance, managed import, a persisted CUSTOM ASSET On/Off switch that retains
  metadata, and the existing Save/Revert flow (`quickshell/ReloadCoverControl.qml`,
  `SchemaPage.qml`, `SettingsSheet.qml`, `schema/DesktopPage.js`).

- **The terminal application and account shell are separate choices.** Keybinds
  > Apps keeps the terminal-emulator picker and adds Fish, Bash and Zsh chips
  beneath it. A validated privileged backend changes the account login shell,
  preserves existing startup files, and refreshes the session environment for
  new terminals (`backend/shellpref.go`, `quickshell/pages/KeybindsPage.qml`).

- **Desktop Visualizer exposes exact colours and two-stop gradients.** The
  Visualizer tab adds the shared colour picker, a Gradient switch and a second
  colour picker, keeps extra visualizer state when saving, includes Frame in the
  look gallery, and paints the chosen gradient in the live preview
  (`quickshell/Hub.qml`, `quickshell/VizPreview.qml`,
  `quickshell/schema/DesktopPage.js`).

- **Advanced settings show a worked example.** Free-text advanced fields now
  carry one: the record editors (Window Rules, Environment, Autostart) show it in
  the placeholder, and schema rows (the image-border fields, the desktop name and
  mark) show a faint "e.g. ..." under the description through a new `SettingRow`
  hint (`ui/SettingRow.qml`, `SettingsSheet.qml`, `schema/*.js`, the record pages).
- **Global gets one "System font" and a font size.** The Font group is a single
  font picker plus a size stepper; both apply live to the shell, GTK/Qt apps and
  the terminal. Pick a monospace like Maple Mono NF for a clean terminal
  (`pages/GlobalPage.qml`, `schema/GlobalPage.js`).
- **Enrolling or verifying a fingerprint plays a live scan.** The Sign-in &
  Fingerprint card shows the shared `FingerprintScan` animation while recording
  (the ridges and ring fill with each enrollment stage) and while verifying, and
  a new "Admin prompts" switch wires fingerprint into the pop-up polkit question
  (`/etc/pam.d/polkit-1`) beside the existing Sudo and Sign-in switches
  (`pages/LockscreenPage.qml`).
- **Appearance groups the GTK app-theming controls into one card.** The Theme
  tab, under the palette scheme, gains an APP THEMING card that surfaces the
  Theme apps toggle beside two new controls: a GTK theme chooser (Adw, the
  libadwaita-consistent GTK3 theme that follows the palette; Adwaita, the stock
  GNOME look; or System, which leaves the choice to the user) writing theme.json
  `gtkTheme` through `ryoku-hub hypr gtk-theme`, and a GNOME accent switch writing
  `gnomeAccent` through `ryoku-hub hypr gnome-accent`, which syncs the desktop's
  accent-colour setting to the nearest named accent so Flatpak and GNOME apps
  that read the system setting follow the palette too. Both keys are additive and
  default on absence (`gtkTheme` to `adw`, `gnomeAccent` to on); the hub only
  persists them and asks the daemon to re-apply, so the daemon stays the single
  writer of gtk-theme and accent-color. A new `ryoku doctor` reconciler converges
  GTK session drift, resetting gsettings gtk-theme to the resolved name for the
  current mode and restoring the settings.ini baselines when they go missing
  (`quickshell/pages/AppearancePage.qml`, `backend/schemes.go`, `backend/hypr.go`,
  `cli/internal/doctor/doctor.go`).
- **Displays lets you type a custom resolution.** The Resolution picker gains a
  "Custom…" entry: a small W × H @ Hz form on the page's own surface. The typed
  mode stages into the draft like any other pick, and on Apply it is forced even
  when the panel does not advertise it -- ryoku-monitor turns a non-advertised
  mode into a CVT modeline. Because a bad custom mode can come up wrong, Apply
  arms a 15-second keep-or-revert banner that re-applies the previous layout
  unless you keep it (`quickshell/pages/DisplaysPage.qml`).
- **Bar Studio picks the dock look.** The DOCK card gains a Style control -- the
  five dock looks (Islands, Rail, Ledger, Tanzaku, Seal) as chips -- reading and
  writing the top-level `dock.style` key on the same live channel its neighbours
  write `edge` on, so the running desktop repaints as you pick
  (`quickshell/pages/BarStudioPage.qml`).
- **The Fastfetch emblem gains a 1-bit dither.** A DITHER switch in the EMBLEM
  group bakes the emblem to bone-on-transparent with an ordered Bayer 4x4 dither
  -- the ryodecor look -- as a sibling PNG (`<source>.1bit.png`), never touching
  the original; off restores it. The bake is pure Go (`image`/`image/png`, no
  Pillow and no shell-out), and the on/off state rides the source name so nothing
  new lands in `config.jsonc` (`backend/fastfetch.go`,
  `quickshell/pages/FastfetchPage.qml`).
- **Fastfetch emblems and installed Store layouts can be removed.** The EMBLEM
  group gains REMOVE IMPORTED EMBLEM, which deletes the `ryoku-logo.*` the Hub
  wrote (and its bake) and repoints the readout at the shipped emblem if it was in
  use; a store-installed emblem carries a remove mark on its own tile; and the
  Store row gains REMOVE INSTALLED STYLE beside APPLY. All three call
  `ryostore remove <category> <id>` and refresh the catalogue and config the way
  the apply path does. A failed removal surfaces its stderr instead of failing
  silently (`quickshell/pages/FastfetchPage.qml`, `backend/fastfetch.go`).
- **The Launcher page can set the solar line.** The Hero card gains a Solar line
  control -- Palette (follow the wallpaper), Fixed, or Off -- and, when Fixed, a
  colour field for the line and its sun/moon marker, writing the launcher.json
  `horizonMode` and `horizonColor` keys (`quickshell/pages/LauncherPage.qml`).
- **The wallpaper reveal is pickable from Appearance.** The Theme tab gains a
  WALLPAPER REVEAL card: a Picker over the 22 named reveals plus `random` (the
  default, a fresh reveal per switch), writing the daemon's
  `wallpaper.transition_preset` key -- the same one the Shell Studio's Pickers
  route sets, so the two stay one truth (`quickshell/pages/AppearancePage.qml`).
- **The form kit gains a stagger-entrance wrapper.** `ui/Entrance.qml` reveals its
  child with a small rise and fade, offset by an `index` so a column of cards
  arrives in sequence instead of all at once. Its rise distance, durations and
  curves come from `Tokens` (the house spatial curve for the rise, the effects
  curve for the fade); the cumulative delay is capped so a long column still lands
  well under a second, and under reduce-motion it renders instantly with no timer
  left running. It is a pure decorator -- the rise is a paint translation and the
  fade is opacity -- so it never changes the child's measured height, and a plate
  sized from that measurement stays exact.
- **Weather and Notes tabs in the Widgets page.** The two new desktop widgets get
  the same treatment as the rest: a live preview card and their own controls
  (weather: enable, layout, size, opacity, placement, lock; notes: enable, pad
  width and height, size, opacity, placement, lock).
- **Bar Studio's dock controls move to their own card, with the new knobs.** The
  dock is no longer a qsbar part, so its card is always visible and writes the
  top-level `dock` store in shell.json: Enabled, Edge (auto / top / bottom / left
  / right), Auto-hide, Frost, Depth, Magnify, Hover labels, Media chip, and the
  pinned-apps manager. A rice can carry the dock look too (`dock` joins the shell
  allowlist).
- **Desktop widgets can be placed automatically.** Each widget's placement gains
  "Auto (calm spot)" beside the nine zones: the shell lands it where the wallpaper
  is quietest and re-places it when the wallpaper changes.
- **The music widget's visualiser has a look picker.** Bars (the default) or a
  smoothed wave, in the Widgets page beside the widget's other options.
- **Displays gains a per-monitor Interface scale.** A stepper on each monitor's
  tuning card (below the compositor scale) sizes the Ryoku shell chrome on that
  display from 50% to 200%, separate from the monitor scale apps render at. It
  writes shell.json `displays.ui_scale.<output>`; the Settings window itself
  honours it too.
- **QMK/VIA keyboards now follow the theme.** A third lighting provider drives a
  VIA-enabled QMK board (Framework Laptop 16, custom mechanical keyboards) through
  `qmk_hid`, alongside OpenRGB and ASUS Aura, so its RGB matrix takes the desktop
  accent as a solid colour. It appears in Appearance > Lighting when `qmk_hid` is
  installed and a board is connected, and stays silent otherwise. No EEPROM write
  per theme change (the colour is re-applied on login and on resume instead).

### Changed
- **Bar Studio drops its QS Bar and Dock sections.** The page keeps the bar-style
  gallery and the built-in style editors (Sumi's frame and rails, Obi, Nacre); QS
  Bar's layout, widgets, form and dock now live in QS Bar Settings, and a QS BAR
  card under the gallery shows the live layout order (watched off shell.json) and
  opens the panel with `ryoku-shell bar settings`
  (`quickshell/pages/BarStudioPage.qml`, `quickshell/schema/BarStudioPage.js`).
- **Picker style moves to the Desktop page.** The theme, wallpaper and media
  picker layout leaves Bar Studio for a PICKERS section on the Desktop page; it
  writes only `qsbar.pickerStyle` through the daemon settings seam, so the bar's
  own layout and widgets under `qsbar` are never overwritten
  (`quickshell/pages/DesktopPage.qml`).

### Fixed
- **The Keybinds page loads again.** Yesterday's Default Apps fix bound
  `onChosen` on the page's `AppPicker`, but the type that name resolves to from
  `pages/` is `Ryoku.Ui.AppPicker`, whose signal is `picked`; Quickshell refused
  the whole file and the page rendered blank. The handler is `onPicked` again
  (`pages/KeybindsPage.qml`), and the publish gate now lints every shipped root
  for this class (`bin/ryoku-dev-lint-qml`).
- **A rice's fastfetch emblem survives updates.** `rice apply` copied the
  emblem over the shipped `fastfetch/fastfetch-emblem.png`, which
  `ryoku materialize` re-lays on every update, so the readout reset to the
  brand mark each time. It now lands on the user-owned `ryoku-logo.<ext>` path
  the Fastfetch page's import uses and repoints `config.jsonc` at it; a new
  `rice emblem` subcommand re-applies the active rice's emblem once a box is on
  the shipped one, and `ryoku doctor` runs it (`backend/rice.go`,
  `backend/fastfetch.go`).
- **The Hub's scheme cards and a rice's colour mode no longer fight the shell's
  theme.** `shell.json` `theme.theme` is the colour master and the daemon shadows
  it into `theme.json` `followWallpaper` on every load, so a MONO/LIGHT/DARK pick
  that only wrote `theme.json` flipped back to the wallpaper palette at the next
  sync, and picking Wallpaper again in the shell was a no-op because
  `theme.theme` already said so: the desktop stuck on the wrong palette. Both
  now select the theme through `ryoku-shell theme` (Wallpaper to follow, Default
  to lock) and write the shadow after (`backend/schemes.go`, `backend/rice.go`).
- **"Unlock with fingerprint" can be switched off on a box with no reader.** The
  switch was gated on a present sensor, so a machine with no fingerprint hardware
  showed it stuck on with no way to turn it off; it is now always operable (a
  stored preference needs no hardware) and reads "no fingerprint sensor is
  connected" when left on without one. Detection is fixed too: `fprintd-list`
  prints "found 0 devices" with no hardware, which used to register as a device,
  and a hung probe now times out instead of sitting on "Checking..." forever
  (`pages/LockscreenPage.qml`).
- **Steam theming no longer breaks Steam's first launch.** Matugen pre-created
  the Steam skin output path under `~/.steam/steam/steamui`, so on a box where
  Steam had never run it created `~/.steam/steam` as a plain directory and stopped
  Steam laying down its own bootstrap there. Steam directory creation and the
  Steam template now wait until `~/.steam/steam/steamui` exists (#64)
  (`backend/matugen.go`, `ipc/matugen.go`).
- **A row gated by a master switch now looks disabled.** `SettingRow` stopped
  taking input when its `enabled` went false but kept full ink, so an inert
  control read as a live one -- the dock's rows under Enabled being the clearest
  case. The row now dims with the state (`ui/SettingRow.qml`), which the shell's
  new studio inherits for free.
- **"Follow focus" on the scrolling layout now scrolls to the window under the
  pointer.** The toggle only flipped the scrolling layout's follow-focus (already
  on by default), so it looked like it did nothing: with the shipped detached
  follow-mouse, hovering a peeked column never scrolled it in until you clicked.
  With Follow focus on, the pointer now drives focus too, so hovering scrolls the
  column into view -- matching how caelestia, dusky and omarchy set up scrolling.
- **GPU temperature and load show on AMD and Intel GPUs, not just NVIDIA.** The
  Machine page's live plate read the GPU only through `nvidia-smi`, so on an AMD
  (or Intel) GPU the temperature stayed blank and the tile fell back to "n/a". It
  now reads utilisation, edge temperature and VRAM from the amdgpu sysfs nodes
  when there is no NVIDIA reading.
- **The graphics-mode switch is hidden on single-GPU machines.** Hybrid /
  Performance / Passthrough only mean something with a second GPU to switch
  between; a machine with one GPU always renders on it, so the switch (which had
  looked "locked to Hybrid" and refused Performance) no longer appears. Nothing
  was throttled -- there was simply nothing to switch.

### Removed
- **Performance > Idle loses "Freeze the bar" and "Freeze the visualiser".** Both
  asked for behaviour that is now unconditional: an audio analyser with no audio
  has nothing to show, so it always stops, and the memory side is already covered
  by "Unload visualiser when silent" and its Go watcher. The only thing the
  switches could still express was "keep analysing silence", which is the waste
  they were meant to prevent, and, left off by default, exactly what shipped: two
  cava processes hours old and a surface repainting hard enough to keep the
  compositor awake.
  No migration needed. This page is `performance.json`'s only writer and
  serialises the whole adapter, so a key it no longer declares is ignored on read
  and pruned on the next save.

### Added
- **Appearance gains a Browsers card.** It explains the Ryoku Theme browser
  extension: Firefox retints live once enabled, Chromium and Brave load it
  unpacked, and web-page recolor toggles in the extension popup.
- **Animations gains a preset gallery: 13 window animation personalities.** On
  top of Ryoku's default, the Animations page now offers Dusky, Bounce, Fade,
  Fast, Mechanical, Minimal, Air, Slow-mo, Exaggerated, Hallucination, Rage and
  Off (ported from dusklinux/dusky). Picking one writes `~/.config/ryoku/anim-preset`,
  which the Hyprland loader reads, and reloads live; the curve editor below still
  fine-tunes the active preset.
- **Animations gains a Shell Motion card: global speed and reduce-motion.** The
  Animations page now carries the shell's own motion controls above the Hyprland
  window editor: a Speed slider (0.25x to 3x) that scales every Ryoku panel,
  menu and transition live, and a Reduce motion switch that snaps them into place
  for comfort. Both write `shell.json` `theme.motion`, which the ui tokens read
  live, so the whole desktop retimes without a restart.
- **Appearance > Colour generation gains a Variant picker.** Follow-wallpaper
  palettes were always built with matugen's tonal-spot; the wallpaper path now
  reads a `schemeType` knob, and Appearance offers all nine matugen variants
  (Tonal Spot, Vibrant, Expressive, Fidelity, Content, Fruit Salad, Rainbow,
  Neutral, Monochrome). Tonal Spot stays the default and the row flags that
  Neutral and Monochrome desaturate toward grey, so the choice is explicit rather
  than hidden. The pick applies live through the same matugen-set seam as Mode
  and Contrast (`matugen.json` schemeType, daemon `matugenArgs`).
- **Washed-out accents no longer light the keyboard near-white.** A bright,
  low-saturation accent (a pale wallpaper palette with all three channels
  bunched near the top) read as plain white on an LED. The lighting backend now
  pulls the non-dominant channels down until the hue shows, keeping the hue and
  the brightness, and leaves already-saturated, grey, and dark accents untouched
  (`backend/lighting.go` `punchColor`). Explicit fixed colours are unchanged.
- **Appearance > Lighting now sees the built-in Aura keyboard on supported ASUS
  laptops instead of only the OpenRGB motherboard alias.** The backend joins
  OpenRGB with `asusd`'s per-controller D-Bus API, prefers that native keyboard
  over OpenRGB's opaque N-KEY laptop controller, and carries old settings onto
  its stable identity. Painted looks move to the closest supported firmware
  effect (Static is the safe fallback), each mode keeps its own firmware values,
  and native-only changes never wake OpenRGB. Failed hand-back writes stay
  managed and retryable rather than silently abandoning the keyboard.

- **The GPU page is now Machine, and you decide what Saver, Balanced and
  Performance actually do.** The page lost its decor -- the 描画 watermark and the
  Ticks-framed read-only specimen, about 210 lines -- and the two-column rail
  collapsed to one full-width column, which is what made room for CPU controls
  without a scroll marathon. A new CPU POWER PROFILES section picks which of ppd's
  three profiles you are *editing* (not switching to, and it says so, naming the
  active one) and exposes governor, EPP, a frequency ceiling and the fan/thermal
  policy for it. That last knob replaces the old standalone "Thermal profile",
  which wrote `platform_profile` live and was silently overwritten by ppd on every
  profile change; as part of a stored definition it now survives. A BATTERY section
  surfaces the charge ceiling and PCIe ASPM policy, which `ryoku-power` has
  implemented since it shipped but had no GUI for. Every row is capability-gated,
  so a machine without a knob simply does not show it, and both sections hide
  cleanly on a backend that has no `cpu` verb.

  A closing note names CPU boost and PPT/TDP as deliberately unexposed and quotes
  the measurements, because telling someone why a knob is missing is part of
  giving them control. `Performance` is renamed **Desktop Effects**, which is all
  that page has ever contained. Nav keys and filenames are unchanged, so deep
  links and the Store's "open in settings" handoff keep working
  (`pages/GpuPage.qml`, `schema/GpuPage.js`, `Hub.qml`, `backend/cputune.go`).
- **The plugin placement stage is a free two-axis drag, and the centre is no
  longer off limits.** Add-ons > a frame popout: drag the popout chip anywhere on
  the screen mock. Land it in the middle and the popout is centred (a modal
  surface, written as `framePopout { edge: "center", align: "center" }`); land it
  near an edge and it docks there, taking `start | center | end` from the thirds
  along that edge. The struck-out "reserved" band is gone, the edge selector
  gained Center, and a centred pick explains itself: it has no hover edge, and it
  shares the middle of the screen with quick settings (Super+Escape) and the
  stash (Super+S), which take turns with it because the shell shows one surface at
  a time (`pages/AddonsPage.qml`).
- **Auto-hide toggle in Bar Studio.** The qsbar bar controls gain an Auto-hide
  switch (`shell.json .qsbar.barAutoHide`): the bar slides off and frees its
  space, revealing on a slow hover along the edge (`pages/BarStudioPage.qml`).
- **World-map time zone picker in Global.** Settings > Global gains a Time zone
  control that opens an Ubuntu-style world map: continents drawn from Natural
  Earth data, a live day/night terminator with the sun's position, a dot per
  zone, click-to-pick the nearest one, a dropped pin, fuzzy search, and the
  selected zone's live local clock. Applying runs `timedatectl set-timezone`
  (no password, via a shipped polkit rule) and restarts the shell so the bar,
  dashboard and lockscreen clocks pick up the new zone live, since a running
  process caches its zone and never re-reads `/etc/localtime`
  (`TimezoneMap.qml`, `world-land.js`, `tzmap.js`, `pages/GlobalPage.qml`,
  `SettingsSheet.qml`, `SchemaPage.qml`).
- **Dock controls in Bar Studio.** The qsbar Dock (the opposite-edge app dock)
  toggles here alongside its Frost, Depth, and Magnify switches, and a "Dock
  apps" manager pins any installed app: an app picker adds them, chips remove
  them, and the list persists in `shell.json .qsbar`
  (`pages/BarStudioPage.qml`, `AppPicker.qml`).
- **Lighting tab in Appearance.** A new Settings > Appearance > Lighting tab
  controls OpenRGB-compatible keyboards and attached lighting devices per device.
  The master switch is off by default; nothing is scanned or written while off.
  Devices are adopted one at a time; an unadopted device is never touched.
  Per-device controls show effect chips in two groups (Ryoku-painted effects and
  the device's own), colour from the wallpaper accent or fixed, per-zone colours,
  brightness, speed, direction, Save to device (where offered), and Hand back
  (which restores the effect the device was on before Ryoku touched it). Settings
  live in `~/.config/ryoku/lighting.json` so an update or a shell restart never
  changes a device (`pages/LightingTab.qml`, `pages/AppearancePage.qml`,
  `schema/AppearancePage.js`).
- **Per-device RGB control over OpenRGB.** The `ryoku-hub lighting` command group
  (state|scan|enable|disable|set|apply|accent|save|release|animate) manages
  devices through the OpenRGB SDK (TCP 127.0.0.1:6742, protocol 5). It starts
  OpenRGB headless as a transient user unit only when asked, bound to loopback,
  and stops it when lighting is switched off; an OpenRGB instance you already run
  is used as-is and never stopped. Ryoku paints seven effects itself through a
  device's per-LED (Direct) mode: Solid, Breathe, Pulse, Spectrum, Rainbow Wave,
  Comet, and Scanner. They exist because many keyboards advertise effects their
  firmware ignores: an ASUS N-KEY laptop board runs Static, Breathing, Flashing
  and Spectrum Cycle and does nothing for Rainbow Wave, Comet or the reactive
  effects, which was confirmed by capturing OpenRGB's own client bytes (identical
  UPDATE_MODE payloads, so it is the firmware, not the protocol). The mode a
  device was on when Ryoku first saw it is recorded and restored on hand-back or
  when lighting is switched off, and the device's own memory is written only by an
  explicit Save to device, never as a side effect (`backend/openrgb.go`,
  `backend/lighting.go`, `backend/lightingfx.go`).

### Changed
- **The theme defaults to following the wallpaper.** `loadThemeState`
  (`backend/schemes.go`) defaults a box with no `theme.json` to follow the
  wallpaper now (was static mono), so the Appearance scheme control and the daemon
  agree with the shell's new default look.
- **The FILES panel points raw Hyprland overrides at the live file.** The "Raw
  overrides" entry now opens `~/.config/hypr/user.lua` (what Hyprland actually
  loads), not the orphaned `user_edits/hypr/user.lua`, and the panel names the
  three real tiers: your GUI config, raw overrides, and the shipped base
  (`quickshell/Hub.qml`).

### Added
- **Import an existing setup (drop-and-go migration).** A new Advanced, Tools
  page, "Import config", brings a config from another Hyprland box or distro onto
  Ryoku: point at a folder, an existing `~/.config`, or a git URL, and it scans
  Hyprland, kitty, fish and fastfetch (plus a generic drop for anything else),
  shows every keybind clash against Ryoku's shipped binds, and resolves each in
  place (keep Ryoku's, use yours with an auto unbind so yours wins, or remap).
  Every touched file is backed up first, so the whole import undoes in one step.
  Hyprland keybinds and window rules become GUI-managed settings; the rest
  (settings via `hl.config`, monitors, env, exec, and non-ingestable binds)
  translate to the `hl` API and layer into `hypr/user.lua`. Engine:
  `ryoku-hub import scan|apply|undo` (`backend/import*.go`, reusing `keybinds.go`
  and `hypr.go`, which gains an `Unbinds` override). UI:
  `quickshell/pages/ImportPage.qml`, sharing the keybind conflict and chord logic
  with the Keybinds page via `quickshell/Combos.js`.
- **The Performance page can follow the power profile.** A new POWER PROFILE
  switch (on by default) lets the active power profile shape the shell: Power
  Saver strips motion, blur and shadows while Balanced and Performance leave the
  page's own switches untouched. It writes the `powerProfileEffects` key the
  shell's `Perf` policy reads (`quickshell/pages/PerformancePage.qml`).
- **A Global page collects the cross-cutting settings.** A new Global section
  (Overview -> Global) is the single home for the interface language, regional
  formats, the machine location and the system font. Language and the system
  font had no editor before (config-only); the location, temperature units and
  regional formats moved here from Desktop, so each setting lives in exactly one
  page -- no duplication. Regional formats stays separate from language, so an
  English UI can pair with Brazilian (or any) date/number formats, and the system
  font is a searchable drawer of the fonts installed on the machine (fc-list)
  (`quickshell/pages/GlobalPage.qml`, `quickshell/schema/GlobalPage.js`,
  `quickshell/Hub.qml`, `quickshell/schema/DesktopPage.js`).
- **Appearance lists downloaded colour schemes.** The Colour scheme picker now
  shows installed RyoStore themes beside the built-in palettes, read live from
  the daemon catalog (`ryoku-shell theme catalog`) and applied through the same
  `theme.theme` seam; the ALL THEMES drawer and the named-palette count include
  them (`quickshell/Singletons/UserSchemes.qml`, `quickshell/pages/AppearancePage.qml`).
- **The desktop visualiser previews live.** The Visualiser subtab now leads with
  a self-contained animated preview that retunes as you change the style
  (bars, dots, line, wave, segments, radial, circle), position, shape, bar
  count, thickness and height, so the look is visible without leaving Settings
  (`quickshell/VizPreview.qml`, `quickshell/pages/DesktopPage.qml`).
- **The Hyprland plugins gather in one place and auto-detect.** The compositor
  plugins (title bars, liquid glass, image borders, plus enables for realistic
  cursor motion and focus animation) were scattered across Look, Cursor and
  Animations; they now gather into a new Windows > Plugins tab that lists only
  the plugins whose .so is actually installed on the box
  (`quickshell/schema/WindowsPage.js`, `quickshell/pages/WindowsPage.qml`).
- **Search understands intent and forgives typos.** A curated synonym map lets a
  typed word reach the vocabulary the index uses ("transparency" finds opacity,
  "hotkey" finds keybinds, "screenshot" finds recording), and a bounded edit
  distance still matches a near-miss like "trasparency". No model, just a
  lexical layer sized to the settings vocabulary (`quickshell/Hub.qml`).
- **Every file and image picker is the same paper-and-ink browser.** The native
  Qt file dialog is gone from the hub: the decor/image chooser, the Fastfetch
  logo and ASCII pickers, the hero-image picker, and profile export/import all
  use the shared `PickFile`, which now carries per-picker filters and a doc tile
  for non-image files (`ui/PickFile.qml`, `ui/Decor.qml`,
  `quickshell/pages/FastfetchPage.qml`, `quickshell/pages/HeroEditor.qml`,
  `quickshell/pages/ProfilePage.qml`, `quickshell/pages/AddonsPage.qml`).
- **Rows no longer collide with their control.** A changed inline row drew its
  struck default and revert glyph in the same corner as the stepper or switch,
  so they overlapped; the value, struck default and revert now sit in one
  cluster to the control's left. Tabs and segmented controls also gained
  breathing room so they read as distinct pills, not one fused strip
  (`ui/SettingRow.qml`, `ui/Tabs.qml`, `ui/Seg.qml`).
- **The hub reads as one instrument, not a wall of cards.** Every setting used
  to be a 104px value-hero card packed into a bento grid, so a page of toggles
  read as scattered tiles shouting ON/OFF. Each setting is now a compact row
  (label and description on the left, the control on the right, or a full-width
  band beneath for a segmented bar, chips, gallery or slider) grouped into
  bordered, collapsible `SettingCard` drawers parted by hairlines. Two shared
  primitives carry it, `SettingRow` and `SettingCard`; the schema pages and
  every settings-grid page were ported to them (Appearance, Animations, Input,
  GPU, Launcher, Displays, Recording, Performance, Bar Studio, Dictation), while
  the bespoke surfaces (Connections lists, Keybinds capture, the displays
  arrange canvas, previews and decor) were kept intact
  (`ui/SettingRow.qml`, `ui/SettingCard.qml`, `quickshell/SettingsSheet.qml`,
  `quickshell/SchemaPage.qml`, `quickshell/pages/*`).
- **Search jumps to the setting.** Typing in the rail search now opens a results
  card under the field, each hit showing its breadcrumb and the matched run
  highlighted, instead of a cramped list crammed into the rail. Choosing a hit
  navigates to its page, switches to the right tab, scrolls the exact control to
  centre and flashes it; Enter takes the top hit and Escape clears. A changed
  row also gains an inline reset on hover (`quickshell/Hub.qml`,
  `quickshell/SettingsSheet.qml`, `ui/SettingRow.qml`).
- **The at-rest write-ledger is gone and the space is reclaimed.** The 360px
  right column drew a decorative torii poster whenever nothing was dirty, which
  is most of the time; it is removed, the page takes the full width, and the
  window narrowed to 1200px to suit. The pending diff moved into a "VIEW DIFF"
  popover the action bar opens on demand, so the write ledger is one click away
  when it matters and out of the way when it does not (`quickshell/Hub.qml`,
  `quickshell/shell.qml`, `ui/ActionBar.qml`).
- **Bar Studio picks the bar style and tunes it.** The Desktop page leads with a
  style picker, Sumi (the left rail) or Obi (the floating top bar), that swaps the
  running bar live. For Sumi it edits the frame and the left rail as before; for
  Obi it shows a per-widget show/hide panel, so the workspaces, media, audio,
  connections, battery, tray and weather chips turn on and off, saved per style in
  shell.json (`quickshell/pages/BarStudioPage.qml`, `quickshell/Hub.qml`).

### Fixed
- **Nothing the Hub opens can bring up a second desktop.** The Hub talks to the
  shell with `qs -c shell ipc call ...` and opens terminals for updates,
  rollbacks, dictation models and the GPU switch. A Quickshell instance that has
  crashed once keeps `__QUICKSHELL_CRASH_INFO_FD` in the environment it hands its
  children, and Quickshell reads that before it parses arguments, so those `qs`
  calls relaunched the desktop config instead of talking to it: two bars, two
  docks. All eighteen launches now go through `Spawn` (`Ryoku.Ui.Singletons`),
  which unsets the handle for the child; the terminals are routed too, so a `qs`
  the user runs inside one is clean as well.
- **The Hub stops waking a sleeping discrete GPU to look at it.** Two polls called
  `nvidia-smi` unconditionally, and that pulls a runtime-suspended card straight
  back out of D3 -- about 10 W on a hybrid laptop. The Profile plate polls every
  1.5s and the GPU page every 2s, so either one, left open, held the dGPU awake and
  silently cancelled the saving the whole hybrid setup exists for; the GPU page was
  the worse of the two, since it woke the card in order to report a number that
  then described the woken state rather than the idle one. Both now read the
  driver's `power/runtime_status` first (one sysfs read, no process spawn, never
  touches the card). The plate rests its GPU fields at zero, exactly as on a
  machine with no NVIDIA card; the GPU page says so outright, reporting
  "suspended, drawing no power" instead of a fabricated reading. This is the guard
  the bar's GPU telemetry already had; these two were the callers missing it
  (`quickshell/Singletons/LiveStats.qml`, `quickshell/pages/GpuPage.qml`).
- **"Open in settings" from the Store lands on the page it promised.** The
  handoff starts the Hub and then IPC-calls `nav open <section>`, but the
  remembered-section restore is a Process: on a cold start its result arrived
  after the deep link and dragged the user back to whatever page they had open
  last, so the button looked like it did nothing. An explicit jump now latches
  and the restore stands down (`quickshell/Hub.qml`, `quickshell/shell.qml`).
- **Fastfetch can be reset to the default.** The page offered "Apply Installed
  Style" from the Store but no way back to the built-in readout; a "Reset to
  Default" action now rewrites config.jsonc from the default model
  (`backend/fastfetch.go`, `quickshell/pages/FastfetchPage.qml`).
- **The App Launcher settings page renders again.** `launcherRoot` fell back to
  `Qt.resolvedUrl`, which resolves to an unusable qrc path in the deployed Hub, so
  the launcher catalogue never loaded and the preview plus every style, palette,
  and motion control drew blank. It now reads the launcher the shell installs
  under `$XDG_CONFIG_HOME/quickshell/launcher` (`quickshell/pages/LauncherPage.qml`).
- **The tiling-layout preview animates again.** Windows -> Layout declares a
  `layoutdemo` row for the looping dwindle/master/scrolling diagram, but
  `SettingsSheet` had no renderer for that control, so it fell through to the
  text field and drew an empty card. It now renders as a full-width band: the
  `art/tiling-<layout>.gif` diagram beside the layout name and blurb, swapped
  live as the picker changes (`quickshell/SettingsSheet.qml`).
- **"Never ask" reaches its fix when the keyring is still encrypted.** `keyring
  init` records never-ask on a box whose keyring is password-protected and leaves
  the file intact, so the mode reads as chosen while every login still prompts.
  Picking the mode that was already selected returned early, which left the
  convert and start-fresh controls unreachable and the option looking dead. The
  Lockscreen page now opens that row as soon as it sees never-ask against an
  encrypted keyring, and picking never-ask again offers it too
  (`quickshell/pages/LockscreenPage.qml`).


### Fixed
- **Adaptive sync stopped switching itself back off.** `hyprctl` reports vrr as a
  bool of whether the link is doing adaptive sync right now, not the mode that
  was set, so On read back as Off whenever nothing was driving it and Fullscreen
  read back as Off outside fullscreen. The Displays page took that reading as the
  current value and saved it over the choice. The mode now comes from the last
  applied layout (`system/hardware/display/ryoku-monitor`).
- **SDR brightness reaches past 2x.** The stepper stopped at 2.0, which is a
  Ryoku limit and not a Hyprland one (it accepts well beyond that), so SDR
  content stayed dim in HDR on a bright panel with the control already at its
  end. The ladder now runs to 6.0, fine to 2.0 and coarser above
  (`quickshell/pages/DisplaysPage.qml`).

### Added
- **Bar Studio lists the Music widget.** The rail's music spectrum widget joins
  the catalogue picker as "Music" (`quickshell/barstudio/CatalogLabels.qml`).

### Fixed
- **Adding a widget to a rail turns that rail on.** The bottom and right rails
  ship off, so dropping a widget onto one in Bar Studio changed the config but
  left nothing on the desktop -- the add read as fully broken. Every add now
  enables the rail it lands on, so "I added a widget, show it" always holds; a
  rail is hidden again with its own Show-this-rail switch, after the fact
  (`quickshell/barstudio/BarStudioModel.js`).
- **Appearance's colour controls apply when you set them, and the live scheme
  regenerates again.** Three faults stacked. The mode and contrast knobs staged
  behind the shared Save, so setting them appeared to do nothing until you found
  the action bar; they now write straight through, as the scheme picker already
  did. `matugen set` unmarshalled the payload onto a zero struct, so every save
  from this page wrote `themeRyokuApps: false` and emptied the roster it does not
  show; it merges onto the stored config now, and the page sends only the two keys
  it owns. And the ALGORITHM picker could select `scheme-monochrome`, which drains
  the colour out of every wallpaper -- indistinguishable from live colours being
  broken -- so it is gone (`backend/matugen.go`, `quickshell/pages/AppearancePage.qml`).

### Added
- **Recording has a Save recordings to setting.** The output directory was the
  one recording knob no UI could reach: the recorder read it from the
  environment, the deck hardcoded it, and this page only described it. It is a
  key in recording.json now like every other setting on the page, and everything
  that writes or lists a recording resolves it. Empty keeps following your
  Videos folder (`quickshell/pages/RecordingPage.qml`).

### Changed
- **The Theme tab leads with colour generation, and the 57 named palettes sit in
  a drawer.** Mode and contrast are what the live scheme is made of, so they come
  first; the scheme section keeps the two dynamic picks and the palette in use
  visible and folds the rest behind ALL THEMES, instead of a six-row wall of
  swatches between the reader and everything below it
  (`quickshell/pages/AppearancePage.qml`).
- **The palette cache path is `~/.cache/ryoku/colors.json`, and the hub palette
  singleton is `Palette`.** `wallustCacheDir()` is renamed `ryokuCacheDir()` and
  points at `~/.cache/ryoku/`, the rotating-border blocks read `hypr-colors.lua`
  from there, and the hub's `Wallust` singleton (Profile plate and clock preview)
  is renamed `Palette`. The clock accent choice `wallust` is renamed `palette` to
  match. The retired wallust engine leaves no path or type name behind
  (`backend/schemes.go`, `backend/matugen.go`, `backend/rice.go`, `backend/hypr.go`,
  `quickshell/Singletons/`, `quickshell/ClockPreview.qml`, `quickshell/schema/WidgetsPage.js`).
- **Appearance > Theme now picks the colour scheme from the daemon's named
  catalog, the same settings seam the shell sidebar theme picker uses.** The old
  Follow / Light / Dark / Mono segmented control -- which drove the hub's own
  `theme.json` scheme through `ryoku-hub hypr scheme` -- is replaced by a
  swatch-card picker over `theme.theme`, written through the settings daemon
  (`Settings.patch`): Follow Wallpaper and the shipped Default as first-class
  cards, then the 57 named palettes each as their own surface, name and
  two-by-three swatch grid, so the Hub and the sidebar read and write one truth.
  A card applies instantly (like the sidebar): the daemon resolves the palette
  into `themePalette` and fans it into the shell and every app, so the scheme
  sits outside the page's staged Save -- the Material You (Matugen) engine and
  knobs and Theme Apps still ride Save. The card catalog is a generated preview
  projection of the daemon's theme table, cross-checked byte-for-byte against the
  sidebar's `MenuTheme.qml` (`pages/AppearancePage.qml`, `schema/ThemeCatalog.js`,
  `schema/AppearancePage.js`).
- **Bar Studio edits only the essentials that provably change the running
  desktop.** Pick an edge from the four rail tabs to put that rail on the bench;
  every edit lands on the running desktop as you make it and rides the Hub's Save
  and Revert. The frame controls are the draw toggle, opacity, band thickness and
  corner radius; each rail has its on/off and thickness; and each rail's three
  zones hold its widgets. Every zone is its own titled section with a numbered,
  reorderable list and a per-zone drawer that offers the widgets that fit the rail
  and are not already on it; the controls use a plain click handler so a tap
  always lands inside the page scroll, which the old compact reorder buttons lost.
  The widget-radius knob and the two-look style switch are gone (they had no live
  consumer, and `ryoku doctor` strips the dead keys); the band thickness and
  corner radius are wired to new keys the shell actually reads (`frameThickness`,
  `frameCorner`), replacing the inert `frameBorder`/`frameRadius`. The per-rail
  auto-hide choice is dropped too: a hover-hidden bar could not be brought back
  because the overlay input mask does not widen on hover, so every rail stays
  pinned. The bounded menus and the stash and system surfaces are still never
  dropped: every edit clones the whole `frameBars` object, so an untouched subtree
  always survives (`pages/BarStudioPage.qml`, `barstudio/ZoneEditor.qml`,
  `barstudio/BarStudioModel.js`, `barstudio/CatalogLabels.qml`,
  `barstudio/qmldir`, `Hub.qml`).
- **Ryoku Settings now describes the Atoll-only shell instead of the retired
  style catalogue. The Bar page keeps the two-look Atoll control, live bar
  geometry and preserved sidebar-content controls, and drops style, island and
  sidebar-opener settings that no longer have a runtime consumer. The Desktop
  Widgets page now edits the wallpaper clock only. The Ryoku theme preset and
  rice look allowlist use Atoll, so an old style cannot be reapplied through a
  preset or imported rice (`schema/BarPage.js`, `schema/WidgetsPage.js`,
  `pages/BarPage.qml`, `pages/WidgetsPage.qml`, `backend/rice.go`,
  `backend/schemes.go`).

### Fixed
- **Backdrop frost now says what it affects.** The launcher editor calls the
  control **Backdrop frost** and explains that it blurs the captured desktop
  seen through the result drawer; zero keeps the drawer solid
  (`quickshell/pages/LauncherPage.qml`).
- **The App Launcher preview now shows the launcher, at its true scale.** The
  old full-width mock only repeated the hero art, so it made the shutter look
  larger and structurally different from the real palette. The specimen now
  keeps the live 720 x 250 hero ratio, search line, scope keys and compact
  centered physical size. The page also exposes **Type settle** (120-700 ms),
  which controls when a completed typed-result deck is released
  (`quickshell/pages/LauncherPage.qml`, `schema/LauncherPage.js`).
- **The App Launcher editor and live launcher now share the same hero crop.**
  The page calls the image a hero instead of a home-card backdrop, keeps the
  image picker and direct drag-to-frame interaction, and renders through the
  same `Ryoku.Ui.HeroCrop` cover/focal-point math as the live shutter. Wide,
  portrait, fallback, and off-centre images therefore land in the launcher
  exactly where the saved preview put them
  (`quickshell/pages/LauncherPage.qml`, `Ryoku.Ui/HeroCrop.qml`).
- **Switching the engine back to Wallust actually reverts now.** Turning the
  Material You toggle off only re-fanned the Matugen colours still sitting in
  `colors.json`, so the desktop stayed on the M3 palette -- the switch looked
  like a no-op. It now re-derives: the change back to Wallust re-applies the
  current scheme (follow re-runs Wallust on the wallpaper, a fixed light/dark
  scheme reloads its palette), replacing the stale Matugen palette instead of
  re-fanning it. A same-engine change (per-app toggle) keeps the fast re-fan
  path (`backend/matugen.go`).
- **Wallust themes every app again (Nautilus, Discord, Steam, ghostty, ...),
  not just the shell.** The Material 3 app templates use M3 role names
  (`primary`, `surface`, `on_surface`, ...) that only exist under the Matugen
  engine; on Wallust the render carrier held base16 only, so the first
  unresolved role aborted the *whole* matugen render and every GTK/Qt app froze
  on its last theme -- most visibly Nautilus keeping a light background after a
  light Matugen theme. The carrier now maps base16 -> the M3 roles, so the
  templates resolve on Wallust too and the app suite re-themes with the
  wallpaper (`backend/matugen.go`).
- **Theme tab leads with Wallust; Matugen is an opt-in "advanced" section, and
  each app has a button to its template.** The Wallust/Matugen picker no longer
  sits at the top as a co-equal choice: Wallust (the default) drives the palette,
  and the Material You (Matugen) engine plus its algorithm, mode and tuning now
  live under a collapsed "MATERIAL YOU (ADVANCED)" toggle, so the Material 3 look
  reads as opt-in. In Theme Apps each app gained a folder button that opens its
  template under `~/.config/matugen/templates`. The folder-icon recolour also
  tints muted wallpapers instead of dropping to grey, so a low-saturation accent
  still reads as a colour (`pages/AppearancePage.qml`,
  `hyprland/scripts/ryoku-cmd-folders`).
- **Theme tab: palette changes go through Save instead of silent auto-apply.**
  The engine (Wallust / Matugen), scheme, Material 3 tuning, Ryoku interface and
  per-app toggles applied the instant you touched them and never lit the action
  bar, so the Save button stayed greyed and the edits felt lost -- unlike every
  other page in the Hub. They now stage into a draft that raises the shared Save
  (and Revert): pick freely, then commit in one click, and Revert or an unsaved
  quit drops the staged edits. Staging also stops the contrast slider re-running
  matugen on every drag frame (`Hub.qml` gains `pageDirty` + `savePage` /
  `revertPage`; `pages/AppearancePage.qml`).
- **Matugen (Material 3) is merge-ready: live wallpapers, real app theming, and
  the panel folded into Theme.** The Material 3 engine now samples a still frame
  from a live/video wallpaper before running matugen (it panicked on a `.webm`,
  silently aborting the whole apply -- shell, apps, and the folder recolour), and
  a scheme or per-app change now re-fans the palette live on the wallust engine
  too, not only matugen. The standalone Matugen tab is gone: its controls live
  under Appearance > Theme, engine-first (Wallust / Matugen), with the wallpaper
  gallery dropped for room and the app list trimmed to the templates that are
  actually toggleable plus a note that terminal, borders and Qt6 always follow and
  Discord, Telegram, OBS, Zed, Steam and Heroic need the theme picked inside the
  app once. The wallpaper->folder-colour post_hook the PR's older `config.toml`
  had dropped is restored (`backend/matugen.go`, `pages/AppearancePage.qml`).
- **Matugen: the visualiser, folder icons and window border follow the wallpaper
  even with Ryoku Interface set to Original.** Those three read
  `~/.cache/wallust/colors.json` directly (no `followWallpaper` gate), so writing
  the monochrome palette to that file when Ryoku Interface was Original dragged
  them mono too, though the toggle only promises to keep the Hub, pill and widgets
  mono. Matugen now always writes the wallpaper's Material 3 palette to
  `colors.json` and keeps the shell mono through `followWallpaper` alone, so on the
  Matugen engine those accents track the wallpaper on both settings while the
  chrome still honours the toggle (`backend/matugen.go`).
- **The theme scheme control shows the mode you are actually on.** The palette
  offers Follow, Light, Dark and now Mono; the fourth was missing, so a desktop
  on the Mono palette (the Ryoku default) lit no segment and the control read as
  if nothing was selected. Mono is now a first-class option, and its blurb no
  longer claims it tracks the wallpaper (`pages/AppearancePage.qml`).
- **Every page's section eyebrow matches the rail again.** Pages hard-code the
  "you are here" group above their title, and the task-oriented regroup had left
  fifteen of them stale: Input, GPU, Displays and Connections claimed SYSTEM
  while they live under Devices; Keybinds, Window Rules, App Overrides and Layer
  Rules pointed at the retired ADVANCED group instead of Apps & Keys; Recording,
  Dictation and Fastfetch were not Tools; and Autostart, Environment and
  Performance were not System. Their section register numbers were corrected too
  (`pages/*.qml`).
- **The Advanced settings switch stays where you left it.** It persisted through
  `ryoku-hub config set advanced`, but the config CLI never knew that key, so the
  write failed silently and every reopen came back with Advanced off, the deep
  knobs re-hidden. Added the key with a round-trip test (`backend/config.go`,
  `backend/config_test.go`).
- **Rices > Browse gains a Refresh button.** The community-store grid fetched its
  catalogue only once per Hub session (`showBrowse` pulled it only while empty),
  so a rice newly added to `ryoku-extras` never appeared without reopening the
  Hub, and the sole re-pull was the empty-state "Try again". A Refresh button now
  re-runs `ryoku-hub rice catalog` (already network-first with CDN-busting),
  matching the Add-ons store and Lockscreen (`pages/AppearancePage.qml`).
- **Matugen (fixed-scheme path): pre-create the template output dirs.**
  `backend/schemes.go` (`renderApps`) makes the output dirs before matugen runs so
  its "folder doesn't exist" warnings stay out of the log (matugen already
  self-creates them, and its errors were already surfaced here). GTK/Qt generation
  is intact -- verified live; the reported "no themes" failure is now diagnosable
  via the daemon's newly surfaced matugen errors (see ryoku/shell).
- **Settings nav rail: long translated category labels no longer collide with the
  kanji seal.** The Latin label in `Hub.qml` now elides within the space before the
  right-aligned kanji (Portuguese "Widgets da área de trabalho", "Substituições de
  aplicativos"); short labels are unchanged.
- **Language > Generate with AI now explains the key file.**
  `schema/ShellSettingsPage.js` states that Ryoku auto-creates
  `~/.config/ryoku/i18n-llm.json` on login and the user pastes an Anthropic/OpenAI
  key into it (button and file were wired but undocumented).

### Added
- **Shell settings are flagged in red in the rail.** Every rail section that
  configures the Ryoku shell rather than the Hyprland compositor now carries a
  small red tick at its inner edge, so the shell pages (Bar, Frame, Desktop,
  Widgets, App Launcher, Lockscreen, Appearance, Connections, Recording, and the
  rest) read apart from the compositor pages (Displays, Input, Cursor, Windows,
  Animations, Keybinds, Window Rules, App Overrides, Layer Rules, Autostart,
  Environment) at a glance (`Hub.qml`).
- **Updates moved to a top-right corner button with a live badge.** The Updates
  section left the rail for a compact English UPDATES chip pinned to the Hub's
  top-right corner, riding the empty top strip clear of each page's running-head
  marginalia: it opens the Updates page from anywhere and wears a red dot
  whenever the channel sits behind origin. The dot uses a new fixed `Tokens.alert` red
  that never follows the wallpaper, so an update always reads as an update even
  under a themed palette; the `Updates` singleton self-checks on load and on the
  configured cadence, so the badge is live. The write ledger drops below the
  button on the pages that show it (`Hub.qml`, `Ryoku.Ui` `Tokens.alert`).
- **The Advanced switch curates the rail, and moves to its foot.** It used to
  only reveal the deep knobs buried inside a few pages -- an effect you had to
  already be on the right page to notice -- so the switch read as inert. Now
  Advanced off also trims the sidebar to the everyday sections (Profile,
  Displays, Connections, Input, Windows, Appearance, Bar, Desktop, Lockscreen,
  App Launcher, Keybinds, Store, Installed), folding the power-user
  ones (Cursor, GPU, Frame, Widgets, Animations, the per-window/layer rules and
  App Overrides, every Tool, Performance/Autostart/Environment and Rashin) away
  until it is on. Whole all-advanced groups (Tools) collapse header and all, the
  open section always stays listed so a toggle never strands you, and search
  still reaches everything. The switch itself leaves the top of the rail for its
  foot, seated below the barcode plate as persistent chrome (`Hub.qml`).
- **Displays page: live drag, no cursor-trapping gaps, and a main display.** The
  arrangement canvas now moves each display tile live under the cursor (it only
  jumped to the drop point before). Dropping a display always snaps it flush to a
  neighbour, and a layout opened with a gap is tidied at once -- Hyprland cannot
  move the cursor across a gap, so a separated layout stranded a screen. A new
  "Set as main" control (and a MAIN tile badge) puts a display at the global
  origin, Hyprland's primary / cursor-home corner. The contiguity and main
  geometry live in a unit-tested `pages/lib/arrange.js` (`pages/DisplaysPage.qml`,
  `pages/lib/arrange.test.mjs`, `schema/DisplaysPage.js`).
- **The `dyad` dual-edge bar is selectable from Settings, and Jules3182 is
  credited.** The bar-style gallery gains `dyad` (Jules3182's dual-edge
  floating-island bar) with its own "Dyad look" toggle -- faithful dark capsules
  or Ryoku-native grainy paper chips -- and the Credits page lists Jules3182's
  dotfiles beside the other bar sources (`schema/ShellSettingsPage.js`, `Hub.qml`,
  `pages/CreditsPage.qml`).
- **Multi-language UI with self-maintaining translations.** Ryoku surfaces read
  through one `I18n` singleton (`Ryoku.Ui`): `I18n.tr("English")` returns the
  current language or the English key as a fallback, so a partly-translated UI is
  never broken. Language comes from `~/.config/ryoku/i18n.json` (default `auto`,
  which follows the OS locale) and switching it retranslates every open surface
  live. Wrapping the schema renderer (`SettingsSheet`) translates every settings
  page's labels and descriptions at once; the GPU page is wrapped too. Translation
  files are generated, never hand-edited: `ryoku/ui/i18n-sync.py` extracts English
  and machine-translates only the strings each language is missing (keyless, so it
  runs in CI), with `overrides/<lang>.json` preserving human fixes; a workflow
  regenerates and commits them on every update. First languages: Spanish, French,
  Portuguese, and Brazilian Portuguese.
- **The Lockscreen page gains an "At sign-in" keyring section.** A compact
  hairline card above the skin gallery, in the page's own bespoke language: a
  three-mode chip row (Unlock at sign-in / Never ask / Ask each time) over a live
  status line read from `ryoku keyring status --json` (keyring format, the
  daemon, and the caveats -- an encrypted keyring that only unlocks if its
  password is your login password, the autologin conflict). Picking a mode runs
  `ryoku keyring set <mode>` (pkexec pops polkit for the root PAM half, the same
  UX as applying a skin). When a mode is blocked by an encrypted keyring, an
  inline password field converts it (`--convert --password-stdin`, the secret
  fed through stdin, never argv) or "Start fresh" resets it after a confirm. New
  schema search entries surface it under keyring, secrets, passwords, unlock, and
  sign-in (`pages/LockscreenPage.qml`, `schema/LockscreenPage.js`).
- **The Displays page gains per-monitor colour management, including HDR.** Each
  display gets a COLOUR control (sRGB / Wide / HDR) beside Adaptive sync; picking
  HDR reveals an SDR brightness stepper (1.0x-2.0x) that lifts SDR content into
  the HDR range. The choice maps to Hyprland's `cm` (with 10-bit depth on Wide
  and HDR) and rides Apply, the persisted layout and named profiles like every
  other per-monitor setting. A panel that cannot do HDR has the compositor
  resolve it back to sRGB, so the page reflects that honestly on the next read
  (`pages/DisplaysPage.qml`, `schema/DisplaysPage.js`).
- **A "Theme apps" toggle extends the palette past the shell into GTK / GUI
  apps.** The Appearance page's Theme tab, under the palette scheme, gains a
  switch (`ryoku-hub hypr theme-apps`) that decides whether Files, text editors
  and other GTK / libadwaita apps recolour to the wallpaper or the locked
  scheme, or stay stock. On renders the GTK stylesheets through matugen; off
  blanks them so apps fall back to Adwaita. The shell, terminal, borders and Qt
  always track the palette. A rice now carries the choice (`color.themeApps`),
  so a shared full-system look reaches (or spares) the recipient's apps the same
  way it did the author's, and the Rices tab lists the GTK apps it touches
  (`schemes.go`, `hypr.go`, `rice.go`, `pages/AppearancePage.qml`).
- **Performance ships cheap on RAM by default, and RyoLayer gets an on/off on the
  Shell page.** The Performance page's memory unloads (launcher, overview,
  visualiser, covered desktop widgets) now default ON, so a fresh desktop frees
  every idle surface, and it adds an "Unload the widget board" toggle for
  RyoLayer. The Shell page's DESKTOP tab gains an "Enable widget board" switch
  that turns RyoLayer (the Super+G overlay) on or off entirely
  (`pages/PerformancePage.qml`, `schema/PerformancePage.js`,
  `schema/ShellSettingsPage.js`, `Hub.qml`).
- **The GPU page gains live, per-session tuning, and a full beta-18 rework.** It
  now explains itself (plain-language purpose plus a live status line) and, below
  the render-mode selector, a TUNING section that probes the machine
  (`ryoku-hub gpu tune caps`) and shows only the knobs that hardware actually
  exposes: NVIDIA power limit (where the driver still allows it) and persistence,
  AMD performance level and power cap, and Intel frequency. (The ACPI thermal
  profile started here too, and moved to the CPU power profiles above once it
  became clear ppd rewrites it on every switch.) Overclock, undervolt,
  clock-lock and manual fan sit
  behind an Advanced disclosure with a bone-plate warning. Everything is runtime
  sysfs / nvidia-smi state applied through one pkexec batch and gone on reboot,
  so a reboot (or `gpu tune reset`) is the whole backup. Named presets (Quiet /
  Balanced / Performance plus your own) persist to
  `~/.config/ryoku/gpu-presets.json`; built-ins are symbolic and skip any knob a
  box lacks. The specimen is rebuilt as a Ticks-framed instrument plate under the
  描画 watermark, no bespoke card (`GpuPage.qml`, `gputune.go`, `gpupreset.go`).

### Changed
- **The DESKTOP section is rebuilt around objects, so each thing has one home.**
  Window settings were scattered across Appearance (Windows/Effects/Borders
  tabs), the Shell page, and Animations; now every window setting lives on one
  **Windows** page with clean tabs (Layout, Look, Borders, Motion). The old
  catch-all Shell page split into three object pages that match how you think
  about the desktop: **Bar** (the panel, clusters, island, sidebars), **Frame**
  (shape, surface, shadow, notifications) and **Desktop** (brand, weather, the
  visualiser). The pointer left Appearance for its own **Cursor** page under
  Devices, next to Input, its theme catalogue intact. Appearance is trimmed to
  what it is really about (theme, comfort, rices), and the sheet machinery its
  moved tabs no longer needed is gone (199 lines). The rail is reordered so
  related pages sit together and "Desktop Widgets" reads as "Widgets" beside its
  Desktop sibling (`Hub.qml`, `pages/WindowsPage.qml`, `pages/BarPage.qml`,
  `pages/FramePage.qml`, `pages/DesktopPage.qml`, `pages/CursorPage.qml`,
  `pages/AppearancePage.qml`).
- **Progressive disclosure: one Advanced switch calms every settings page.** A
  global toggle in the rail hides the deep, rarely-touched knobs behind it: blur
  and shadow sub-tuning, dwindle and master internals, the visualiser's spectrum
  and motion, per-style bar variants, frame grain and edge melt. The default view
  shows the settings you actually reach for; search still finds a hidden knob, and
  flipping the switch reveals them all in place (`SchemaPage.qml`,
  `SettingsSheet.qml`, `Hub.qml`, and the page schemas).
- **Ryoku Settings reorganized around how you look for a setting, not how it is
  implemented.** The rail is regrouped into task-oriented sections -- OVERVIEW,
  DEVICES (displays, connections, input, GPU), DESKTOP (appearance, shell,
  animations, lockscreen, launcher, widgets), APPS & KEYS (keybinds + window /
  app / layer rules), TOOLS (recording, dictation, fastfetch), SYSTEM
  (performance, autostart, environment, updates) and ADD-ONS (store, installed,
  rashin) -- so recording/dictation/fastfetch read as tools, performance and the
  agent OS leave the old ADVANCED catch-all, and the group holding the open
  section lifts up the ink ramp as a quiet monochrome "you are here" (`Hub.qml`).
- **Global search reaches every page, the words people actually type, and whole
  phrases.** Each section carries a keyword set, so `wifi` / `bluetooth` /
  `hotspot` find Connections, `agent` finds Rashin, `plugin` finds the Store, and
  synonyms the labels avoid (`transparency`->opacity, `startup`->autostart,
  `screensaver`->lockscreen, `visualizer`) resolve. Multi-word queries no longer
  cliff to nothing when one word is out of vocabulary -- scoring is tolerant,
  summing the words that match and scaling by coverage, so `dark mode`, `second
  monitor` and `turn off blur` land their page instead of blanking. A setting
  also matches its option values (`h264`, `dwindle`, `dark`, `fahrenheit`); tab
  names are searchable; engineering-only surface rows (action buttons,
  dynamic-title notes) no longer pollute results; and a page whose keywords own
  the queried word floats above a stray control that merely mentions it, so a
  bare `blur` / `cursor` / `battery` lands the section, not a decoy (`Hub.qml`).
- **Appearance is re-tabbed by task: the 73-setting Look tab became Windows +
  Effects, and the Wallpaper tab is reframed as Theme.** The old Look wall (window
  shape, tiling, gaps, behaviour, blur, shadows, glow, glass, opacity, motion) is
  now **Windows** (shape, tiling, gaps, behaviour) and **Effects** (the visual
  layers). The former Wallpaper tab -- which already led with the light/dark/follow
  colour scheme and Apply-Ryoku-theme above the wallpaper gallery -- is now named
  **Theme**, since choosing a scheme or wallpaper is how you theme the desktop, so
  `dark mode` / `theme` / `accent` land an obvious tab instead of hiding under
  "Wallpaper". Seven tabs, sized to clear the write-ledger column
  (`schema/AppearancePage.js`, `pages/AppearancePage.qml`).
- **The Installed and Updates pages fill their dead space with a poster.** A
  short plugin list or an up-to-date channel used to leave a large empty column;
  each now closes on a `Decor` specimen (ink-only, no control) that gives the
  section its face, matching the rest of the Hub (`pages/AddonsPage.qml`,
  `pages/UpdatesPage.qml`).
- **Shell settings show only the options the active bar style actually uses.**
  Beyond the Style gallery, each bar style now exposes just the settings its bar
  reads: `washi` shows only its Washi Look, `delos` its island modules / edge /
  radius, the band skins (`noctalia` / `caelestia` / `aegis` / `stele`) the
  module layout, `atoll` only position and thickness, and so on, so a style never
  lists a knob that does nothing for it (e.g. atoll no longer shows the washi or
  island settings). A schema row carries a `styles` list and the sheet hides it
  while the drafted `barStyle` is not in it, live as you switch styles in the
  gallery (`schema/ShellSettingsPage.js`, `SettingsSheet.qml`, `SchemaPage.qml`,
  `pages/ShellPage.qml`).
- **Ryoku Settings writes its generated Hyprland Lua into the `user_edits`
  overlay.** `settings.lua` and `rebinds.lua` are authored under
  `~/.config/ryoku/user_edits/hypr` (the source that survives updates) and
  reflected into the live `~/.config/hypr`, so a Save still hot-reloads at once
  (`backend/hypr.go`).
- **One Keybinds page for every shortcut: Apps, System, Custom.** Default Apps is
  folded in. **Apps** picks what each launcher key opens (browser, terminal,
  editor, files, notes) -- installed apps as chips, a filterable catalogue of
  every installed app (an executable cheatsheet), or any typed command -- and
  rebinds that key right beside it. **System** rebinds the built-in shortcuts
  (Close, Fullscreen, focus, workspaces). **Custom** layers your own, now with the
  same app picker on Run-command rows so you never need to know an executable
  name. App choices launch through the `ryoku-app` resolver (applied on the next
  press, no reload; $BROWSER/$TERMINAL exported for the CLI and xdg-open). A key
  change is a pure remap through the generated `rebinds.lua` that binds.lua's K()
  consults, so shipped and custom binds never double-fire and overlaps are named
  as you go; rebinds reset per row or all at once. Adds a shared filterable
  `AppPicker`; removes the standalone Default Apps page and its nav entry
  (`pages/KeybindsPage.qml`, `ui/AppPicker.qml`, `Hub.qml`, `backend/apps.go`,
  `backend/keybinds.go`, `hyprland/scripts/ryoku-app`, `hyprland/modules/binds.lua`).
- **Keybinds you can record by pressing them, with named conflicts.** The custom
  editor no longer makes you type "SUPER + J" by hand: every row has a record
  button (and the section's + records a brand-new bind in one click) that
  captures the next combo you press. Even shipped chords like SUPER + Q are
  caught safely, because recording first enters a do-nothing Hyprland submap
  where only its own binds fire, so the combo reaches the Hub instead of closing
  it (Esc, a timeout, or a click outside cancels). A captured bind that shadows a
  shipped one now names it ("Shadows shipped: Close active window") instead of a
  bare "shipped", and Clear all became Restore defaults. Adds the submap module
  `hyprland/modules/record.lua` (`pages/KeybindsPage.qml`, `hyprland/hyprland.lua`).
- **The Shell page's Desktop tab picks images and locations instead of typing
  paths and city guesses.** The brand Logo image was a raw path field; it is now
  an image control: a live thumbnail of the current mark, a Choose button that
  opens a file browser, and Clear (falls back to the text glyph). The weather
  Location was a free-text guess Open-Meteo might or might not geocode; it is now
  a live-autocomplete field: as you type, the same keyless Open-Meteo geocoder
  the weather widgets resolve with suggests real places as "City / Region /
  Country", and picking one disambiguates (Tokyo, Japan vs Tokyo Hill, Texas)
  and records the resolver cache (`~/.local/state/ryoku/weather-loc.json`) so the
  pill, launcher and desktop widget all land on exactly that place instead of
  re-geocoding. Typing freely still works; empty still locates by IP. Two
  reusable schema control types back this (`image` and `location` in
  `SettingsSheet.qml`), usable by any page (`schema/ShellSettingsPage.js`,
  `SchemaPage.qml`).
- **The file/image picker is one shared component now.** The monochrome
  image/folder browser was copied per page; it is promoted to `Ryoku.Ui.PickFile`
  and the Appearance page (border image, rice wallpaper, rice export/import) uses
  the shared one, so the Desktop tab's image picker and every existing picker are
  the same widget (`ryoku/ui/PickFile.qml`, `pages/AppearancePage.qml`).
- **Rices capture the whole desktop now, and their previews tell the truth.**
  A rice snapshots three more look stores -- desktop widgets (clock and
  calendar), the audio visualiser, and the desktop decors, whose pictures are
  bundled into the rice (`rice://` assets) and land under
  `~/.config/ryoku/rice-assets/<slug>/` on apply, so a shared decor never
  points at a file that exists only on the author's disk. The launcher look
  gained its card knobs (`bgBlur`, `radius`, greeting and weather toggles),
  the shell look carries `frameEnabled`, and the brand (name + mark, with the
  mark image bundled) travels as a new opt-in layer routed to `brand.json`.
  Every save now writes `preview.png` from the wallpaper as it stands at that
  moment: a still is scaled down, a live (video) wall contributes the frame
  the palette is tuned to (the wallust offset), so the tile and detail show
  the wall of when it was saved -- an mp4 is never handed to an `<Image>`
  again (a live-wall rice previously drew a broken tile). Live walls are
  captured as the actual clip, badge `LIVE` on the tile and detail, and apply
  routes them into `~/Pictures/livewalls/` so Super+W keeps cycling them.
  `.previous`/`.baseline` snapshots grew to all eight stores, so Restore
  original reverts widgets, visualiser, decors and brand too
  (`backend/rice.go`, `pages/AppearancePage.qml`, `schema/RicesPage.js`).
- **Applying a rice is a choice, not a package deal.** ALSO SETS is now a row
  of toggles (KDE's global-theme partial apply): every behaviour bundle the
  rice carries -- keybinds, input, window rules, layer rules, per-app
  overrides, autostart, environment, brand -- applies by default and taps off
  individually, so a recipient can take the look and leave the keybinds
  (`pages/AppearancePage.qml`).
- **Border colours live with the scheme that governs them, and a colour is a
  swatch now, not a hex string.** The active/inactive window-frame colours moved
  out of Borders and under the Theme tab's colour scheme (Follow / Light /
  Dark), so the one decision -- follow the wallpaper or use fixed colours -- and
  the colours it controls sit in one place instead of split across two tabs. A
  new `ColorField` renders a live swatch with a click-to-open picker beside the
  hex, used wherever a colour was a bare hex field (border, shadow, glow, glass,
  OSD, fastfetch accent), so a colour reads as a colour rather than a code
  (`ColorField.qml`, `SettingsSheet.qml`, `pages/AppearancePage.qml`,
  `schema/AppearancePage.js`).

### Added
- **The capture card shows its coverage before you name the rice.** A
  `ryoku-hub rice preflight` readout lists what the save will carry: the
  wallpaper kind (live video or still), the decor count, widgets and
  visualiser, the colour mode, and the non-empty behaviour layers -- so
  "everything travels" is visible, not asserted (`backend/rice.go`,
  `pages/AppearancePage.qml`).
- **Import closes the sharing loop.** `ryoku-hub rice import <folder>` (and
  the IMPORT button beside Save) installs an exported rice folder as a local
  rice, de-duping the slug and skipping the export's reading matter, so a
  rice folder someone sends you is one pick away from applying
  (`backend/rice.go`, `pages/AppearancePage.qml`).

### Fixed
- **The colour picker is Ryoku's own paper-and-ink surface, not a grey web
  dialog.** Clicking a colour swatch opened the platform `ColorDialog`, which
  ignored the theme. It now opens an in-app picker on the bar's own black surface:
  a saturation/value field, a hue rail, a live swatch and a mono hex field, all
  drawn from `Tokens` (`ColorField.qml`).
- **A colour setting no longer prints its hex on top of its own control.** The
  SURFACE colour cells reserved no footer for the swatch and still showed the raw
  hex as the cell value, so the `ColorField` overlapped the cell content and
  spilled into the gap between cards. Colour cells now reserve the control footer
  and blank the redundant value, like the picker and image cells
  (`SettingsSheet.qml`).
- **Text settings save the value you typed, not the one before it.** A text
  field (the desktop brand name, the mark glyph) committed its edit only on
  focus loss, but clicking Save never blurs the field, so the typed value was
  dropped and the setting "would not save". Fields now commit as you type, so
  Save always writes what is on screen (`SettingsSheet.qml`).
- **The Fastfetch preview shows the emblem at its real size, and an Auto fit
  sizes it undistorted.** The readout preview drew the emblem as a fixed 84x84
  square, so changing Width, Height or Pad moved nothing -- the preview never
  matched the terminal. The emblem is now measured in the readout's own
  character cells (a Width x Height box, shifted right by Pad), and the whole
  readout scales to fit the card, so it is a true specimen: adjusting the size
  resizes it exactly as fastfetch lays it out. A new Auto fit button holds the
  width and sets the height to the art's own aspect against the terminal's cell
  ratio, so a picked image renders square instead of stretched
  (`pages/FastfetchPage.qml`).
- **Bar style previews look like the bars.** The Shell page's bar STYLE
  gallery drew abstract shapes that did not resemble the skins they name. Each
  tile now paints a faithful mini-bar in the skin's own treatment: noctalia's
  dot workspaces and clock pill, caelestia's numbered cell strip in one
  container pill, aegis's flat modules with an accent underline, stele's
  bracket cells, the three rounded islands of triptych and the single floating
  one of delos, nacre's islands under a hairline top edge, and inir, aurora and
  angel as flat solid, glass and heavy-base panels. Drawn once in
  `ui/Singletons/Silhouette.qml`, so the gallery tile and the ryowalls mock
  desktop cannot disagree with the real bar.
- **The Displays scale stepper steps through real scales now.** It stepped the
  percentage by 25 from whatever the compositor last reported, so on most
  panels a single press requested a scale Hyprland cannot hold (1.60 + a step =
  1.85, a clean divisor of nothing common): the compositor refused it, drew the
  "Invalid scale" overlay, substituted a neighbour, and the read-back looked
  like arbitrary numbers -- worst on low-res screens, whose valid scales are
  sparse. The stepper now walks the selected mode's ladder of Hyprland-valid
  scales (`scaleLadders` from `ryoku-monitor list`: the same 1/120
  whole-logical-pixel rule the compositor enforces, floored at a 640x360
  logical desktop, so 720p offers 0.5x through 2x in 13 steps), a resolution
  change re-snaps the staged scale to the new mode's ladder, and Apply reloads
  the live list afterwards so the action bar reports what the compositor
  actually accepted instead of assuming the request stuck
  (`pages/DisplaysPage.qml`).
- **Ryoku Settings fits a 720p screen.** The window rule floats it at 1360x880
  and the window pinned a 1280x820 minimum, so on anything smaller (a 720p
  panel, or a scaled low-res one) the bottom action bar -- Apply itself -- and
  the right-hand controls sat off screen. The window now clamps its maximum
  size (and its minimum with it) to the screen it is on; Hyprland sizes the
  rule into the client's hint and centres the clamped result in the usable
  area, while roomy screens keep the exact previous size (`shell.qml`).
- **Wide segmented controls no longer swallow their own label.** A segmented
  control with three or more options sat inline beside its label, so on a
  narrow card the label, value and description clamped to zero width and
  vanished, leaving the buttons alone in an empty card (worst on small
  screens). Any segmented with three or more options now drops to its own
  full-width band under the text, so the label always has room whatever the
  screen size (`SettingsSheet.qml`, `pages/AppearancePage.qml`).

### Changed
- **Fastfetch emblem is a gallery now, not a file hunt.** Adding your own art
  meant flipping the emblem to Image, finding a small Choose Image button, and
  walking a file dialog, with the art sized in cryptic character columns. The
  emblem is a visual picker now: a wall of ready art you click to set (the
  Ryoku brand marks and the shipped ryodecors, drawn as real thumbnails) plus
  a Your image tile that browses or takes a dropped file. The chosen emblem
  lights up in the wall and the readout preview updates live; ready art sets
  its source directly while your own file is imported and copied as before
  (`pages/FastfetchPage.qml`).
- **Appearance exposes the rest of Hyprland's look, and the layout engines.**
  The Look tab gains the decoration knobs it was missing (fullscreen opacity,
  dim for special and modal windows, dim-around, border-inside-window, the
  full blur set of contrast, brightness, special, popups, ignore-opacity,
  optimizations and vibrancy-darkness, plus sharp and scaled shadows) and the
  general ones (border grab area, resize cursor on border, resize corner,
  no-focus-fallback, workspace gaps). Dwindle and Master each get their own
  group, shown only while that layout is active, so their split, ratio, mfact,
  orientation and new-window knobs are finally reachable. Image-border sizes
  and insets, the glass tint, brightness and theme, and cursor shake-magnify
  are surfaced too. Every knob previews live and persists on Save
  (`backend/hypr.go`, `schema/AppearancePage.js`).
- **Shell exposes the island's dock edge.** The persistent dock already reads
  which screen edge it sits on, but the Shell page never offered it; it now
  sits in the bar's Island group as a four-way pick (top, bottom, left,
  right). The retired pill-era geometry keys stay out (the doctor wipes them),
  so no dead control is added (`schema/ShellSettingsPage.js`).
- **Appearance rebuilt to the register, overlaps gone.** The page opened on
  Rices behind a bare eyebrow with hardcoded font sizes, and three rows
  collided: the wallpaper blurb ran under SHUFFLE, the rice-capture name field
  under SAVE/CANCEL, and long rice names and change-summaries overflowed the
  detail header. It now leads on Look, wears the same register header as the
  schema pages (力 eyebrow with rule, marks and hairline leader, Fraunces
  title, blurb), splits the old SHAPE into SHAPE + TILING, and every font size
  is a `Tokens` value. The colliding rows clamp their width (`Math.max(0, ...)`)
  and long text elides, so nothing overlaps. The inline sheet stays (it carries
  the colour swatches and tiling demo the shared sheet cannot); every setting
  key and its wiring are untouched (`pages/AppearancePage.qml`,
  `schema/AppearancePage.js`).
- **Shell settings regrouped into six coherent tabs.** The global tab crammed
  fifteen unrelated settings, the visualiser spread seventeen across eight thin
  groups, and two sidebar groups carried bullet-char names. The desktop's brand
  and weather now sit in their own Desktop tab; global keeps only the shell-wide
  look (surface, roundness, shadow, text); the visualiser collapses to three
  groups (Style, Spectrum, Motion) led by its master toggle; and the sidebar
  groups read plainly. All 53 settings are preserved -- only tab, group and
  order changed (`schema/ShellSettingsPage.js`).
- **Profile reborn as a gothic system poster -- a live scan of the operator.**
  The Greek Lady Justice marble is retired; the plate is now a post-punk
  brutalist poster in the reference register. A cracked marble profile bust,
  fal-generated then baked to a high-contrast bone xerox (Bayer stipple,
  bone-on-transparent), bleeds across the black; the identity is set monumental
  in Fraunces over a huge 顔 watermark; the machine's vitals read as
  line-and-stat callouts pinned to the face like a body scan -- the CPU and GPU
  cores with temperatures, memory, network, and a fracture reading the beta
  version -- each a big live figure on a thin leader to the point it measures; a
  film-grain tooth and the audio-wave signal gif sit over it. Driven by a new `LiveStats` singleton
  that polls `/proc`, `sensors` and `nvidia-smi` every 1.5s while the plate is on
  screen and writes nothing. The Profile section is also wired into the hub nav
  (it was a nav entry with no page) (`pages/ProfilePage.qml`, `Hub.qml`;
  `Singletons/LiveStats.qml`, `art/profile-hero.png`, `art/grain.png`, new;
  `art/marble-justice.png` retired).
- **Decor picks apply live -- on Save and on "Next image", no restart.** A local
  decor edit (pick from the gallery + Save, "Next image", "Shuffle", reframe)
  persisted at once, but `DecorStore`'s own file-watch then re-broadcast and a
  stale re-read reverted the value just set: the box snapped back to the old art,
  so a pick only "took" after reopening the app and "Next image" needed two
  clicks to advance. `syncFromStore` now holds a short guard after any local write
  so it does not clobber the edit; external changes still land once the window
  passes. The editor's on-close reload also runs deferred (`Qt.callLater`) so the
  picked art repaints once the modal is gone, and shared `Cell.qml` clamps its
  text column against a negative width so a narrow cell no longer clips
  (`ui/Decor.qml`, `ui/Cell.qml`).
- **Decor art caches, so a section with decors stops glitching on select.** The
  decor image loaded with `cache: false`, so every reopen of a page with decors
  (Input has six, several of them large custom stills) re-decoded each one from
  disk -- a visible pop as the art loaded in. It now caches (`cache: true`): a
  decor's art is decoded once and reused across reopens, so the section paints
  from cache instead of re-reading the files (`ui/Decor.qml`).
- **Performance rebuilt as a two-column plate with a hawk specimen.** The ten
  tweaks sat in a 3-per-row grid that orphaned a cell in EYE CANDY and MEMORY
  (two-thirds of each second row dead) and left IDLE short, with no art at all.
  They now flow two-per-row in a left column, even rows, no orphans, beside a
  specimen rail: 疾風 ("swift as the wind"), a hawk graded to the bone duotone,
  the machine at peak performance. fal-generated, graded by hand
  (`pages/PerformancePage.qml`; `Ryoku.Ui` `Placard`;
  `ryoku/assets/ryodecors/hawk.png`, new).
- **Credits sheds the Greek marble for a bone-duotone roots tree.** The kansha
  poster carried a warm Three Graces statue (desaturated to grey at runtime), a
  Greek-noir holdover that clashed with the space-bone-grotesk rewrite. It is now
  an ancient tree, its roots gripping the black, fal-generated and graded by hand
  to the bone duotone (no marble, no colour), under a Fraunces-italic deck ("the
  roots we grow from"), a rotated poster spine, and the editorial gratitude
  ledger. Pure bone on black (`pages/CreditsPage.qml`; `art/roots.png`, new;
  `art/three-graces.png` retired).
- **Rashin wears Hermes's own face.** Advanced > Rashin is a key feature with room
  to grow, so it drops the Hub's chrome and wears the identity of the Rashin/Hermes
  dashboard it fronts (`127.0.0.1:3600`): the warm bone-on-black poster palette and
  Archivo Black + JetBrains Mono type mirrored from the dashboard's `base.css`. A
  full-bleed samurai hero banner opens it, then the live **model Hermes runs on**
  (e.g. `gpt-5.5`, via `openai-codex`, Hermes `v0.18.0`, read from `ryoku-rashin
  status`), a **FUNCTIONS** poster index of what Rashin does (vault, memory, skills,
  agents, chat, code -- with live vault/agent counts), a **TRY** list of example
  commands (`hermes gateway`, `hermes model`, `prowl-agent overview`, ...), and the
  master switch, one-click Hermes setup and dashboard link. Backend: `HermesInfo`
  (`status --json`) now carries the chosen `provider`/`model`. Archivo Black ships
  as a bundled font converted from the dashboard woff2, and the hero rides the
  `ryodecors` art path; the earlier pass's `ryoneedle`/`ryocompass` loops remain as
  `ryodecors` gallery art (`pages/RashinPage.qml`, `fonts/archivo-black.ttf`,
  `rashin/backend/hermes.go`, `ryoku/assets/ryodecors/rashin-hero.png`).
- **Animations, reorganized feel-first with a live motion preview.** The page
  led with a raw cubic-bezier editor and a per-leaf Hyprland override table, so it
  read as advanced. It now opens with named feels (Linear, Gentle, Smooth,
  Snappy, Bouncy) that reshape the curve in one tap, a live preview that slides a
  window along the selected curve so the easing is felt (feedback on change and
  tap, never a perpetual loop), the bezier kept as fine-tune, and the per-leaf
  table moved under an ADVANCED heading with a plain note. A bouncing-ball decor
  (滑らか, smooth) fills the section, drawn by a new deterministic
  `bin/art/ryobounce` and baked by `ryodither`; the loop joins the `ryodecors`
  shuffle gallery (`Ryoku.Ui` `Decor`; `ryoku/assets/ryodecors/bounce.gif`, new;
  `pages/AnimationsPage.qml`).
- **Desktop Widgets' live preview is a pinned side column, not a half-page mock.**
  The preview was a 300px full-width wallpaper mock that sat near-empty and placed
  its widgets by a broken centre-scaled anchor (a "top-left" clock floated
  mid-box), shoving the settings below the fold. It's now a pinned right-hand
  column of three live specimen cards -- clock, calendar, weather -- each
  rendering the real widget scaled to fit, dimmed under a struck header when off,
  with a 3x3 corner map marking where it sits; the settings sheet reflows to the
  left and scrolls full-height (`pages/WidgetsPage.qml`).
- **The Lockscreen section shows the whole qylock catalogue, cached.** It read
  only the installed skins (two), so the 38-design upstream set never appeared; it
  now pulls the live `ryoku-hub lock catalog`. Preview gifs cache locally on first
  open (`ryoku-hub lock cache`, warmed in the background) and refresh over time,
  so the grid stops re-streaming from GitHub on every scroll and survives the
  unauthenticated API rate limit -- the upstream tree is cached too, with a stale
  fallback so a rate-limited fetch never collapses the list to the two vendored
  skins. Picking a catalogue skin downloads and installs it (INSTALL + size hint,
  an Installing state); the full-screen Preview is gated to installed skins. The
  Refresh button pulls a fresh tree behind a spinner with a "+N new designs" /
  "Up to date" cue, finishing fast when nothing is new (`pages/LockscreenPage.qml`,
  `backend/lock.go`, `backend/lockcatalog.go`).
- **The Tiling layout picker shows what each layout does.** Under the dwindle /
  master / scrolling picker (Appearance > Look) a looping diagram plays the
  drafted layout -- windows binary-splitting smaller, a master frame beside a
  stack, or a panning column strip -- and swaps live as you click, before Save
  (it reads the live hypr draft). Bone-on-transparent line art matching the
  sheet, baked by `bin/art/tiling-demos` (`pages/AppearancePage.qml`,
  `schema/AppearancePage.js`; new `quickshell/art/tiling-{dwindle,master,scrolling}.gif`).
- **Decor and Placard art ships as user files in `~/Pictures/ryodecors`.** The
  baked noir set (the statues and moon, the Muybridge/phenakistoscope/earth/
  cradle gifs) and the specimen posters (katana, camera) moved out of the
  `Ryoku.Ui` module into `ryoku/assets/ryodecors`, laid beside `Wallpapers` and
  `livewalls` where a user can see and swap them. The installer seeds them,
  `ryoku-desktop` ships them to `/usr/share/ryoku/ryodecors`, and `ryoku doctor`
  tops up whatever a release adds -- so existing installs receive them on update,
  not just fresh ones. `Decor`/`Placard` resolve through a new `Ryoku.Ui`
  `Ryodecors` singleton; the editor gallery and custom picks are unchanged. Bake
  new art to match with `bin/art/ryodither` (`Ryoku.Ui`
  `Decor.qml`/`Placard.qml`/`Ryodecors.qml`).
- **Connections fills its dead right column with a katana specimen poster.**
  Below the 接続 hero card a slim right-hand bookmark carries a noir-baked
  katana (from the reference pin, mapped onto the ink ramp), a JP title, a
  chapter numeral, a quote, a barcode and a 断 seal -- ink only, no accent,
  shared across every subtab. The lists are held to its left edge so nothing
  overlaps, and it hides when the window is too narrow to spare a slim column
  (`Ryoku.Ui` `Placard`, new; `ryoku/assets/ryodecors/katana.png`, new;
  `pages/ConnectionsPage.qml`).
- **GPU fills its landscape foot with a real-time-render specimen.** Below the
  hardware card and the passthrough dossier, a full-width band carries a shaded
  torus knot -- a stand-in for the desktop's own render -- baked to the house
  dither and cover-filled so it fills the frame edge to edge, beside a 描画
  title, a 三次元 sub, a tate phrase, a caption, an instrument readout
  (SHADING/GEOMETRY/SURFACES/REFRESH), a barcode and a 描 seal. Drawn by a new
  `bin/art/ryorender` -- a deterministic 3D-render loop, like `ryowave` for
  audio, with `--shape` for knot, torus, sphere, cube or spring -- dithered by
  `ryodither`. `Decor` gains a `readout` row, and the five loops join the
  `ryodecors` shuffle gallery (`ryoku/ui/Decor.qml`; `ryoku/assets/ryodecors/`
  `render.gif`, `torus.gif`, `sphere.gif`, `cube.gif`, `spring.gif`, new;
  `pages/GpuPage.qml`).
- **Recording fills its marked right rail with a camera specimen poster.** The
  head and the QUALITY/ENCODER cells reflow into a left column (one cell per row
  while the poster shows, so no label elides), and a wide right rail carries a
  noir-baked camera dimensions blueprint, an editorial epigraph across the
  specimen's head (Fraunces italic), 録画, a chapter numeral, a quote, a barcode
  and a 録 seal. Same `Placard` idiom as Connections; the top-right running-head
  marginalia is subsumed by the poster's header. Hides when the window is too
  narrow to spare the rail (`ryoku/assets/ryodecors/camera.png`, new; `Ryoku.Ui`
  `Placard` gains a `motto` line and a HiDPI `sourceSize`; `pages/RecordingPage.qml`).
- **Dictation fills both its dead zones with voice decor.** A dithered audio-wave
  motion loop runs across the page foot (drawn by `bin/art/ryowave`, baked with
  `ryodither`), and a mic specimen -- the 1938 RCA ribbon-mic patent -- fills the
  right rail. Fine line-art dithers into noise, so the mic (and the katana and
  camera specimens) bake smooth instead: a new `bin/art/ryoduo` maps tone
  straight onto bone with no stipple. The whole decor set is now one bone
  (`#e8d8c9`) on a transparent ground, baked only by `ryodither` (grain) or
  `ryoduo` (smooth), with `bin/art/README.md` on which to reach for (`Ryoku.Ui`
  `Decor` gains `wave.gif` in its gallery; `ryoku/assets/ryodecors/{wave.gif,mic.png}`,
  new; `pages/DictationPage.qml`).
- **The monochrome instrument sheet holds together: a full-hub visual pass on
  every section and subtab.** Driven by screenshots of all 25 sections and
  their tabs on a live session, in one sweep:
  - *No more overlaps.* A long value (a file path) elides in the middle instead
    of running over the neighbouring cell; free-entry fields clip and live in a
    full-width band at the cell foot (like pick bars), so the brand row no
    longer smears `markImage` across the TEXT MARK cell; a gallery cell grows
    to hold its tiles, so the bar-skin gallery stops painting over the cells
    after it; the Widgets mock collapses a disabled widget instead of ghosting
    it at 28% over a live neighbour sharing the anchor (`Ryoku.Ui` `Cell`,
    `SettingsSheet.qml`, `pages/WidgetsPage.qml`).
  - *Cells stopped truncating their own labels.* The side column is opt-in: a
    page without a live preview gets the full content width (the "NO PREVIEW"
    plate is gone), and `Section.span` enforces a 290 px minimum cell, so
    "ENAB…"/"COR…"/"TILI…" read ENABLED, CORNERS, TILING everywhere
    (`Hub.qml`, `Ryoku.Ui` `Section`).
  - *The black is sandpaper, not a void.* Grain doubles to 0.10 so the matte
    speckle actually reads on #000 (`Tokens.grainOpacity`).
  - *The line vocabulary from the reference sheets.* Section marks lead with a
    mono `//` and close their rule with an end tick; framed blocks (the pinned
    preview, the state plate) wear HUD corner ticks (`Ryoku.Ui` `Ticks`, new).
  - *One acid drop of colour.* Following the acid rule (a black base, a single
    saturated accent spent only on state), the active rail item carries a 2 px
    vermillion tick and the unsaved-state dot beats in `Tokens.sun`; everything
    else stays ink (`Hub.qml`, `ActionBar`).
  - *Navigation.* The rail scrolls the active section into view (it used to sit
    clipped to a sliver at the rail edge), typing in Search now also filters the
    rail's sections, Credits is wired to its page instead of a porting plate,
    and `qs -c hub ipc call nav open <key>` jumps sections for scripts and QA
    (`shell.qml`, `Hub.qml`).
  - *Data fixes the sweep surfaced.* Wi-Fi signal shows real percentages and
    bars (Quickshell reports a 0..1 ratio; it was rounded to "1%"), and the
    brand logo row lost its leftover porting-note label
    (`pages/ConnectionsPage.qml`, `schema/ShellSettingsPage.js`).
  - *The reference sheet's full line language, second pass.* A `Reg`
    registration backdrop (faint dot grid, sparse + marks, print-register
    corner crosses) sits under every page; the rail masthead became a framed,
    corner-ticked poster plate (`力 RYOKU ARCH //SETTINGS_`, a `///` mark);
    the rail foot carries a genuine, scannable Code 39 plate (`RYOKU HUB`);
    nav groups wear `01..05` mono indexes with end-ticked rules; the page
    eyebrow runs the full width and closes with `+ ///`; section marks read
    `//TITLE_`; the preview label leads with `//` and the action bar with
    `///` (`Ryoku.Ui` `Reg` new, `Barcode` adopted, `Hub.qml`,
    `SchemaPage.qml`).
  - *Selection is typography, and Japanese carries its weight, third pass.* The
    lone vermillion nav tick (the one generic-UI tell left) is gone: a selected
    tab or nav item is bone with a `//` lead, and every nav row pairs its Latin
    name with its terse kanji seal (外観, 接続, 描画, 規則...), the two scripts
    sitting together as the texture. One shared `Tabs` plate replaces five
    hand-rolled tab bars (schema, Appearance, Connections, Keybinds, Store).
  - *Bento, so no row ends ragged.* `Spans.pack` greedy-fills each group's cells
    into flush 12-column rows and stretches the last to the edge, killing the
    right-half dead space on the schema and Appearance sheets; a 3-option `seg`
    widened so its value stops eliding (`TILING LAYOUT` reads `dwindle`, not
    `d..e`).
  - *Dead zones get meaning, not filler.* Empty states (window/app/layer rules,
    autostart, environment) are a composed plate: 空 in a ring `Motif` over a
    `// EMPTY_` caption. The idle diff panel is a framed specimen the way the
    reference tiles are built, a dithered torii under a solid label bar (so the
    caption always reads), a 空 · AT REST chip, and the brand set vertically as
    リョク; art is generated at dev time (`fal-ai/nano-banana-pro`) and graded to
    pure black. Imagery lands only where a page is about showing something; a
    functional panel gets line and type, never a floating photo (`Ryoku.Ui`
    `Tabs`/`Motif`/`Empty` new, `art/dither-torii.png`, `Hub.qml`, the five
    editor pages).
  - *No flicker on a section change.* The page swap crossfades through two
    loaders, so the content never blanks to bare paper mid-load, and the parked
    page is hidden once faded so a stale hover tooltip cannot leak through the
    overlay; the rail only scrolls the selection into view when it is actually
    off-screen and animates the scroll, so an in-view click never jumps the
    sidebar. `full` and the side ledger are derived from the section, not the
    loading item, so the rail, side column and bar never reflow mid-swap
    (`Hub.qml`).
  - *The at-rest specimen crossfades.* The idle torii plate and the pending diff
    list now fade between each other instead of hard-cutting when the first edit
    lands (`Hub.qml`).
  - *The pinned wireframe preview is gone.* It only duplicated the live desktop
    (edits already show there), so the shell's side column is purely the write
    ledger now (state + pending diff), and the sheet, nav and diff flickables
    reserve a scroll-rail gutter so the bar never overlaps content (`Hub.qml`,
    `SettingsSheet.qml`, `ShellPage.qml`, `AppearancePage.qml`,
    `AnimationsPage.qml`).
  - *The App Launcher page reads dense.* Its lone-cell SHAPE and BACKGROUND
    sections merge into one two-up PALETTE row, Home Card runs three-up, and the
    live preview spans the full width, so no row strands a cell in dead space
    (`LauncherPage.qml`).
- **The Input page, redesigned for personality (section-by-section pass).** The
  KEYBOARD MAP no longer scrolls: a compact live diagram is pinned under the head
  (deep-red `sunDeep` title) beside a decorative poster, so it stays in view
  while you edit and still shows the layout's legends (AZERTY, QWERTZ, Dvorak,
  Colemak, else QWERTY) and lights every remapped key to bone. Each section now
  fills its dead grid space with a `Decor` poster carrying its own kana (配列,
  変換, 操作, 触覚, 連打), so no section reads as a ragged half-row; POINTER's
  controls are reordered so the sliders pair and the rows pack full. The Caps
  Lock chips cell hugs its content (`Cell.neededHeight`) and a faint 入力
  `Watermark` dresses the background (`InputPage.qml`, `Ryoku.Ui`
  `KeyboardMap` compact mode, `Decor`, `Section.titleColor`).
- **Connections wears the same personality.** A faint 接続 `Watermark` sits
  behind the page and an editable `Decor` hero (接続 / ネットワーク, a rotating
  earth) fills the head's dead right, shared across the Wi-Fi / Bluetooth /
  Hotspot subtabs; the head text is bounded so it never crowds the hero and the
  network list is untouched (`ConnectionsPage.qml`).

### Added
- **Marginalia: brand ornament for the chrome dead zones.** Two new `Ryoku.Ui`
  primitives dress the empty margins the way the reference posters do, without
  ever crowding a control. `Pixel` draws a 1-bit dingbat from the brand
  vocabulary, Greek and Japanese, never arcade-alien: a Greek-key meander, a
  torii, seigaiha waves, a fluted column, an asanoha star. `Marginalia` sets one
  in a thin strip beside a katakana gloss, a numbered index plate and a chevron
  run, ink only so the acid accent stays on state. It runs app-wide in the dead
  chrome only: the rail foot's edition register (every page), the framed action
  bar's centre, and each full-bleed page's head margin and bar centre where those
  are genuinely empty (occupied heads and read-only pages are left alone)
  (`Ryoku.Ui` `Marginalia`/`Pixel` new, `Hub.qml`, `ActionBar.qml`, and the
  full-bleed pages: launcher, displays, keybinds, gpu, dictation, recording,
  fastfetch, widgets, lockscreen, addons, updates, performance, rashin).
- **Watermark: a page wears its section kanji.** A new `Ryoku.Ui` `Watermark`
  sets a page's section kanji huge and faint behind the content, bled off the
  lower-right and softened by a light blur, so a settings page has a face without
  a photo. It is the one blur allowed on an app surface, because it is background
  art, not panel depth: ~0.05 opacity, behind the head and the scroll, it never
  touches legibility (`Ryoku.Ui` `Watermark` new, `InputPage.qml`).
- **Decor: a decorative poster that fills a section's dead space.** A new
  `Ryoku.Ui` `Decor` turns an empty grid slot into a noir mini-poster in the
  reference style: a real image or gif (a Muybridge/phenakistoscope motion loop,
  a marble statue, the moon, Newton's cradle, a rotating earth), all baked to
  1-bit bone-on-black through an ordered dither, under a big Japanese title, a
  vertical tategaki line, a fine-print caption, a scannable barcode and a kanji
  seal. Right-click the art to open its editor: frame it like the App Launcher frames its hero -- the image covers the panel and
  is placed by a 0..1 focal point you drag, with a zoom (scroll, pinch, or the
  -/+ buttons; below 100% reveals more, above crops in) -- then pick from the
  gallery underneath (the baked set or a custom file of your own, desaturated to
  noir) and Save (or Enter; Cancel/Esc discards). The focal point is a fraction so
  the small preview crops identically to the box (WYSIWYG); choice and framing
  persist per box through `DecorStore`, and gifs autoplay; with no image it falls back to `DitherField`, a
  procedural fractal-noise field dithered in a Canvas. The art set is public
  domain, baked at build time and shipped with the module (`Ryoku.Ui`
  `Decor`/`DitherField`/`DecorStore` new, `ryoku/assets/ryodecors/*`, `InputPage.qml`,
  `ConnectionsPage.qml`).
- **Updates page stays useful when you are up to date.** A packaged box on the
  latest release showed an empty Updates page (nothing incoming). It now lists
  the recent changes the installed version carries, fetched from the GitHub
  commit history (best-effort, cached), under a "Recent changes" heading, while
  the "Update now" action stays hidden until an update is actually available
  (`internal/updater/{update,commits}.go`, `UpdatesPage.qml`, `Singletons/Updates.qml`).
- **App Launcher: a Background blur slider.** A new slider on the App Launcher
  page sets how much the desktop behind the command palette blurs while it is
  open (0 to 30 px; 0 keeps it sharp). It saves to `launcher.json` as `bgBlur`;
  the launcher then drives the compositor blur to that strength on open, on top
  of the global setting so it frosts even when window blur is off, and restores
  the prior blur on hide (`LauncherPage.qml`, `Hub.qml`, launcher `shell.qml`).
- **Fastfetch: a section to edit the branded terminal readout.** A new Desktop
  section (`ryoku-hub fastfetch` + `FastfetchPage.qml`) reads
  `~/.config/ryoku/fastfetch/config.jsonc` into a friendly model and writes it
  back, so the GUI and hand-edits share one file. Pick the emblem (an image, an
  SVG rasterized on import, an ASCII art file, a built-in, or none) and its size;
  set the accent; toggle, reorder and rename the readout rows; and edit the
  tagline, all beside a live preview that renders the real fastfetch output with
  its colours parsed into the panel, plus a Preview in terminal button for the
  exact thing. Edit config.jsonc opens the raw file; the shipped readout is
  untouched until you change something (`backend/fastfetch.go`, `FastfetchPage.qml`,
  `Hub.qml`).
- **Shell, Global gains a Brand section to change the logo globally.** Set the
  desktop's mark (a short text/glyph, or your own SVG/PNG logo) and name, written
  to `~/.config/ryoku/brand.json` and previewed live on your desktop like the
  other shell knobs, with a Save/Revert that follows the same draft model. A
  monochrome mark tints to your accent; a full-colour logo can show as-is. The
  section states the dimensions and format up front and leaves Ryoku's own apps
  on their own brand (`ShellSettingsPage.qml`).
- **Rices: the reworked, full-desktop theming system.** The Appearance
  **Themes** tab is now **Rices**: browse whole-desktop looks as big-preview
  cards, apply one in a tap, and restore your original setup in one click. A
  rice captures the whole look (the four `~/.config/ryoku` stores, wallpaper,
  launcher hero, cursor) as a named, versioned document; applying it merges only
  look keys so a recipient's personal settings stay put, flips the colour master
  and writes a fixed palette when the rice pins one, and every apply first
  snapshots the live setup so **Restore original** is a byte-for-byte revert.
  Save the current desktop as a rice, duplicate or delete your own, and see
  which is active. A **Browse** tab installs rices from the community store
  (`ryoku-extras`), and `ryoku-hub rice publish` extracts a local rice into the
  store ready to commit. Two showcase rices, **Lofi** (pixel-art warmth) and
  **Pastel** (soft, round, glassy), ship there as fixed-palette looks; their
  bespoke wallpapers arrive with the art (`RicesPage.qml`, `RiceTile.qml`,
  `RiceDetail.qml`, `AppearancePage.qml`, `backend/rice.go`,
  `backend/ricestore.go`).
- **Rices show a preview and take a wallpaper.** A rice with no image renders a
  live mockup from its own surface, accent and rounding, so both the tiles and
  the detail read at a glance; a captured rice shows its bundled wallpaper. The
  detail gains **Set wallpaper**, which bundles a chosen image into the rice
  (`ryoku-hub rice setwall`) as both its applied wallpaper and its preview. The
  store registry carries `accent`/`surface`/`rounding` hints so browse tiles
  preview before install (`RiceTile.qml`, `RiceDetail.qml`, `RicesPage.qml`,
  `backend/rice.go`, `backend/ricestore.go`).
- **See what a rice touches and export it.** The rice detail lists every file
  applying it writes (the four config stores, the regenerated Hyprland settings
  and colour caches) plus the assets it carries, each flagged as provided or
  left unchanged, and **View config** shows the manifest itself. **Export**
  extracts the whole setup into a folder you pick: the manifest, its assets, a
  readable `configs/` breakout, and a README, then offers to open it in your
  files (`ryoku-hub rice files`, `ryoku-hub rice export`; `RiceDetail.qml`,
  `RicesPage.qml`, `ConfigViewer.qml`, `ImagePicker.qml`, `backend/rice.go`).
- **A rice now carries every config, and its shape reaches the whole desktop.**
  **Save current setup** captures the full setup, not just the look: your
  keybinds, input, window and layer rules, per-app overrides, autostart and
  environment travel too (empty sections are skipped), each listed in the rice's
  file view, installed on apply and removed on **Restore original**. A rice's
  rounding now applies to the shell as well as the windows: the frame edge, the
  bar and island, the OSD and the inner chrome all follow it, so a square rice is
  square everywhere and a round one round everywhere. The shipped **Lofi** is
  fully square and **Pastel** fully round (`backend/rice.go`, `RicesPage.qml`,
  `ryoku-extras` rices).
- **The Visualizer tab gains four new looks and deeper control.** Style now
  offers **Line** (a stiff angular readout), **Segments** (a lit LED stack per
  band), **Radial** (a ring of bars around a pulsing centre) and **Circle** (a
  blob whose radius breathes with the music), beside Bars, Dots and Wave, with
  the picker promoted to a
  full-width row. New knobs: a **Frame rate** choice (30/45/60), **Adaptive
  quality** (auto-throttle under load), **Smoothing** and **Sensitivity** feel
  sliders, **Peak caps** for bars and segments, and a **Segments** count. The
  live preview renders every new look, cap and segment so you see the change
  before it reaches the desktop (`ShellSettingsPage.qml`, `VizPreview.qml`).
- **Recording page (System).** Screen recording finally has controls: framerate
  (30/60/120), constant vs variable framerate, quality (up to ultra), codec
  (H.264 / HEVC / AV1), GPU-or-CPU encoder, and cursor visibility, saved to
  `recording.json`. An "under the hood" card asks the recorder which backend and
  hardware encoder actually run on your machine (`RecordingPage.qml`, `Hub.qml`).
- **The Advanced pages explain themselves now.** Every config button carries a
  hover tooltip spelling out what it opens and whether edits survive an update.
  App Overrides gained a plain-language intro, a *pick from an open window* class
  picker (no more hunting for a window class in a terminal), and LOOK / EFFECTS
  groupings that say in words what Inherit, Off, and Force opaque do. Window
  Rules, Layer Rules, Autostart, and Environment each got a concrete one-line
  example of what to enter and when it takes effect. Sections can now carry a
  `description` line under their header (`HubButton.qml`, `SettingSection.qml`,
  `PageHeader.qml`, `AppOverridesPage.qml`, `WindowRulesPage.qml`,
  `LayerRulesPage.qml`, `AutostartPage.qml`, `EnvironmentPage.qml`).
- **Config buttons name the file and stop mislabeling your settings as
  defaults.** The two header buttons now match Ryoku's real config layering.
  **Edit user.lua** opens `hypr/user.lua`, the hand-written layer loaded last
  that wins over everything, now seeded with a header explaining that your GUI
  changes live elsewhere (`hypr.json`, generated into `settings.lua`) and
  survive updates, so it opens self-explanatory instead of blank. **View
  defaults** opens only Ryoku's shipped base modules, read-only; your generated
  `settings.lua` is no longer shown there as if it were a default. Displays
  edits `monitors_user.lua`, the Hub-owned JSON sections keep **Edit config**,
  GPU stays view-only, and every button's tooltip says what it opens and
  whether it survives an update (`Hub.qml`, `PageHeader.qml`,
  `hyprland/user.lua`).
- **Keybinds steer you to the Hub editor and flag conflicts.** The Custom editor
  checks every combo against the shipped legend and your other custom binds,
  amber-flagging one that shadows a Ryoku shortcut or duplicates another, and
  both the editor and the legend note that binds added by hand in
  `~/.config/hypr/user.lua` never appear here and are never conflict-checked, so
  the Hub is the place to add them (`KeybindsEditor.qml`, `KeybindLegend.qml`,
  `KeybindsPage.qml`).
- **Per-app appearance overrides (Advanced > App Overrides).** Give one app its
  own look on top of the global Appearance: match it by class (run `hyprctl
  clients` to find it) and an optional title, then set opacity, corners, border
  size, blur, shadow, dim, animations, or force it fully opaque. Anything left
  on Inherit follows the global setting, so it is additive, never a fork of the
  whole look. Each app becomes one Hyprland window rule on Save that beats the
  global decoration for its windows, so Chromium can be opaque, a terminal
  square, or a video player shadowless without the raw rule-per-property the
  Window Rules page beside it needs. New `AppOverridesPage.qml` on the shared
  `HyprStore` (a new `appOverrides` list), backed by an `AppOverride` type and
  `genAppOverride` in the Go backend (`hypr.go`), which only emits proven
  `hl.window_rule` fields so a bad name can never break `settings.lua`.
- **The Updates page shows a real, staged update.** Instead of a single progress
  "wave", the running view renders the update's ordered stages as a determinate
  multi-segment bar with the current step's label and a live log tail, streamed
  from the run-state file `ryoku update` publishes. A failed update gets its own
  view naming the step that broke, with a one-click roll back to the pre-update
  snapshot; a completed one flashes a check before folding away. The consent
  prompt's answer is now passed as a positional argument (no shell interpolation
  of the option label).
- **Bundles can ship their own code.** The bundle model gains item tiers
  (`core`, installed by "Install all"; `optional`, opt-in per item), an
  `interactive` flag for user-driven fetches (an aborted one reports *deferred*,
  not *failed*), a `nautilus-pack` item type that installs right-click
  file-manager scripts and removes them cleanly, and bundle store imagery
  (`icon`/`accent`/`preview`/`screenshots`, resolved to absolute URLs like
  plugin assets already are).
- **The bundle store looks like a real store.** Bundle cards and the detail
  view now show a hero image with a warm scrim, an install-progress badge, and
  source / tool-count chips (mirroring the plugin store); the text masonry
  became an image grid and the detail opens on a full-width hero banner.
- The Appearance and Animations pages can now enable and configure optional
  Hyprland compositor plugins, each folded into its natural home as a toggle that
  reveals its settings: **Realistic cursor motion** (rotate / tilt / stretch plus
  shake-to-find) on the **Cursor** tab; **Title bars** (height, text size, blur,
  window buttons) and **Liquid glass** (preset, blur, opacity) on the **Look**
  tab; an **Image border** (pick an image, scale, smoothing) on the **Borders**
  tab; and **Focus flash** (flash / shrink / slide) on the Animations page. The
  **Scrolling** tiling layout also gains column-width and follow-focus knobs (the
  scrolling layout is Hyprland core, not a plugin). Plugin settings apply on Save
  (a plugin loads on reload), written to `settings.lua` as an `hl.plugin.load`
  plus a loaded-guarded `hl.config`; a missing or version-mismatched plugin
  degrades to off instead of erroring the config.
- The Shell page gains a **Global** tab for the shell-wide look: inner
  **Roundness**, plus the frame melt, surface, shadow, and typography controls
  moved out of the Frame tab. Its **Font** picker now lists the fonts people rice
  with (JetBrains Mono, Fira Code, Hack, Cascadia Code, Iosevka, Meslo, and more)
  and only offers the ones actually installed, growing as you add your own.
- The Global tab also carries **weather**: a **Location** field (a city name, or
  blank to auto-locate by IP) and a **Units** switch (Auto / °C / °F).
- The Shell page gains a **Sidebar** tab for the two corner-summoned sidebars:
  enable the left (Features) and right (System) sides on their own, pick and order
  the panes each one shows, toggle open-on-hover vs click, and set the panel width
  and corner-hotspot size. Edits apply live to the running shell via `shell.json`.
- The bar **Style** picker offers **Delos** (one island). When it is picked, an
  **Island** section appears: set its corner **Roundness**, then check which modules the island carries
  (Workspaces, Clock, Date, Now-playing, Window title, Status, Tray). The
  island's dock (edge, position, hidden) round-trips through the settings file,
  so editing other options never disturbs where it sits.
- The Appearance page's **Look** tab gains window-motion toggles. **Wobbly
  windows** gives a dragged floating window a spring: it trails the cursor and
  settles with a little overshoot (a native `windowsMove` bezier, no plugin, so
  it works on a stock Hyprland). **Open / close** picks the window open and close
  style (Pop, Slide, Gnome). The **Borders** tab gains a **Rotating gradient
  border**: the active window's border sweeps a gradient of your accent colours
  (wallpaper-derived while colours follow the wallpaper, the fixed pair
  otherwise) at an adjustable speed. All three preview live and persist to
  `settings.lua`.
- An **App Launcher** page (Desktop): tune the command palette (Super+Space)
  from the Hub. Set its corner roundness, the home card's weather units
  (Auto/C/F), whether the greeting and weather show, and the backdrop. The
  backdrop is chosen from a thumbnail grid (no guessing by filename), dragged
  in a live preview to place the crop, and dimmed with a strength slider. It
  writes `~/.config/ryoku/launcher.json`, which the launcher watches, so a save
  shows on the next open.
- A **Dictation** page (System): switch on voice typing and pick the Voxtype
  speech-to-text engine, from local Whisper (Fast or Accurate) to the OpenAI
  API (with a key field). Models download and remove in the Hub with one click,
  no terminal; it writes Voxtype's config and drives its user service, while the
  shell keeps Super+` and the mic wave.
- **Shell -> Bar** offers two Ryoku-native bar skins next to the two carried
  ones. The Style control is now a dropdown of Noctalia, Caelestia, **Aegis**
  (a flat instrument panel: mono editorial clock, accent underlines, no module
  pills) and **Stele** (engraved bracket-cornered cells with an accent-framed
  active workspace). The pick writes `barStyle` and shows on the desktop at once.
- **Shell -> Bar** adds a third native skin, **Triptych**, to the Style
  dropdown: it groups the modules into three rounded islands on the band (left,
  a centre carrying the clock and now-playing, and right).

### Changed
- The sidebar sections are reordered for a cleaner scan: System leads with the
  everyday (Displays, Input, Keybinds, Connections) and drops the niche ones
  (GPU, Recording, Dictation) below; Desktop groups the shell chrome (Shell, App
  Launcher, Fastfetch) right after Appearance, then Widgets, Lockscreen and
  Animations (`Hub.qml`).
- Shell -> Bar now places the bar on any frame edge (Top / Bottom / Left /
  Right) and picks its skin (Noctalia / Caelestia, both carried one-to-one
  from the credited shells), next to the thickness and
  content toggles; the **Status glyphs** toggle covers the bar's
  network/battery/inbox cluster. The bar defaults flipped on to match the
  shell's new default face (the reset baseline follows), and the island's
  dead Width/Height knobs gave way to a note: the island sizes itself. A
  **Reserve space below the island** toggle closes the top gap when off
  (windows rise to the frame, the island floats over them).

### Removed
- **Appearance Themes is under construction.** The full-system theme "rices" (a
  bento grid that swapped the look and real Hyprland motion Lua) are retired: the
  tab now shows an "under construction" placeholder. Gone with them are the
  shipped `hyprland/themes/` folder, the `ryoku-hub hypr theme|themes|colorsource`
  backend, and the `theme.lua` layer that `hyprland.lua` loaded. Colours still
  follow the wallpaper or a fixed Light or Dark palette (Appearance -> Wallpaper).
  A leftover `~/.config/hypr/theme.lua` is pruned by `ryoku doctor`
  (`AppearancePage.qml`, `ThemesPage.qml`, `ThemeTile.qml`, `backend/theme.go`,
  `backend/schemes.go`, `backend/hypr.go`).

### Fixed
- **The Hub reopens on the section you left, not always Shell.** The current
  section is written to the Hub's config on every navigation (`ryoku-hub config
  set section`) -- the value it already read back at startup -- so closing on
  Input and reopening lands on Input (`Hub.qml`).
- **A Decor's art updates the moment you save, no restart.** While the editor
  is open the box sits behind the modal, so a new image or GIF picked in there
  changed the box's source while it was occluded and the async load never
  painted -- it looked stale until the next launch. Closing the editor now
  forces the box to reload its art, so the saved picture (or a fresh-started
  GIF) shows at once (`Decor.qml`).
- **A Wi-Fi network's password row can be dismissed again.** Tapping a secured,
  unknown network opened an inline password row that only closed on a successful
  connect, so deciding not to connect left it stuck open. Tapping the network
  again now toggles the row closed, and it gains an explicit close control
  (`WifiTab.qml`).
- **Applying a rice reports a failed store write instead of claiming success
  over mixed state.** The three look overlays (hypr, shell, launcher) discarded
  their write errors, so a disk-full or bad-permission failure mid-apply left
  some stores restyled and others not, with the UI saying done; the first
  failure now surfaces, and the pre-apply `.previous` snapshot stays the
  one-click way back (`backend/rice.go`).
- **Plugin and Nautilus-pack installs are all-or-nothing.** A failed download
  of a file the manifest names used to be skipped silently, landing a
  half-plugin (or a right-click pack missing actions) that still reported
  installed; a miss now aborts the install, `manifest.json` is written last so
  an aborted plugin never looks installed, and the payload files land through
  the atomic temp-and-rename writer (`backend/extras.go`).
- **Hub drawers and dropdowns animate open/close instead of snapping.** The
  Dropdown popup had no enter/exit transition (it blinked in and out), and the
  Wi-Fi password row, its "Connection failed" line, and the GPU readiness-checks
  disclosure toggled `visible` with no motion. All now fade and slide with the
  Hub's timing tokens (`Theme.quick`/`Theme.medium` + `Theme.ease`), matching
  the nav-rail drawers (`Dropdown.qml`, `WifiTab.qml`, `GpuPage.qml`).
- **Live preview and touchpad toggles work again.** Every appearance/input
  preview (and any saved tap-to-click divergence) generated
  `["tap-to-click"]`, a key the Hyprland Lua config rejects; because the
  whole `hl.config({...})` call is one statement, the error killed the entire
  block, so no slider previewed and a saved tap-to-click off silently
  disabled every other override in `settings.lua` on the next login. The
  generator now emits the hl API's `tap_to_click`.
- **Window rule actions No blur / No border / No shadow now apply.** They
  generated `noblur`/`noborder`/`noshadow`, all unknown to `hl.window_rule`,
  and one bad rule stopped `settings.lua` at that line (everything after it,
  binds, env, autostart, silently dropped). They map to the real fields now
  (`no_blur`, `border_size = 0`, `no_shadow`), verified against the live
  runtime.
- **Layer rule "Dim around" now applies** (`dim_around` is a bool in the hl
  API; the old `dim_around = 0.4` errored out the file). The layer "No
  shadow" action is gone from the page and a stored legacy rule is dropped at
  generation: layer surfaces lost that effect in Hyprland's rule rewrite, and
  emitting it would break the whole generated file.
- **The cursor picked in Appearance now reaches the apps you launch.** The
  override only ran `hyprctl setcursor` (compositor + XWayland); spawned apps
  kept reading the base `XCURSOR_*` env from `env.lua`. A diverged cursor now
  also exports `XCURSOR_THEME/SIZE` and `HYPRCURSOR_THEME/SIZE` from
  `settings.lua` (a later `hl.env` wins over the base), and `env.lua` gained
  the missing `HYPRCURSOR_THEME`. Revert/leaving a page also re-asserts the
  saved cursor, so an unsaved cursor preview no longer sticks until logout.
- **Reset to defaults on Input resets the workspace-swipe settings too** (the
  swipe toggle and finger count were skipped before; the reset now walks the
  full input domain).
- The Animations tree no longer lists Hyprland's internal leaves
  (`__internal_fadeCTM`).
- Desktop Widgets: the page now watches `widgets.json` and folds external
  edits into unedited fields, so dragging a widget on the desktop while the
  page is open no longer gets clobbered by the next Save (same treatment for
  `visualizer.json` on the Shell page, which the deck's quick toggle writes).
- "Ryoku Default" on the Themes page is the shipped baseline again: its look
  had drifted to an older rice (rounding 16, gaps 8/26, border 3), so applying
  it pinned non-default values; its look is now empty and folds to the exact
  `decoration.lua` defaults, and `HyprStore`'s initial draft matches them too.
- Connections > Bluetooth no longer shows a dead switch when the service is
  gone. With no org.bluez on the bus (bluez missing or bluetoothd stopped),
  `Bluetooth.defaultAdapter` is null and the old page still rendered "Bluetooth
  is off." with a live-looking toggle that silently did nothing -- exactly what
  a bluez-less install showed. The adapter toggle now hides without an adapter,
  the placeholder says the service isn't running, and a **Start service** pill
  revives it (`pkexec systemctl enable --now bluetooth.service`, the polkit
  prompt via hyprpolkitagent) with a transient failure line. An rfkill-blocked
  radio (airplane mode, a laptop radio key) is surfaced as "Bluetooth is
  blocked." and the toggle unblocks first (`rfkill unblock bluetooth`,
  seat-writable via systemd uaccess), powering the adapter when the unblock
  lands, instead of asking BlueZ for a Powered=true it refuses.
- GPU page no longer offers to "fix" a disabled IOMMU it cannot fix. On an
  Intel host with IOMMU off, the capability engine used to show a warn ("Ryoku
  can add intel_iommu=on") and a `needs-setup` verdict with an Enable button,
  but the enable path never touched the kernel cmdline, and the one function
  that would (`addCmdlineTokens`) edited the `cmdline:` line that
  `limine-mkinitcpio-hook` regenerates on every kernel update, so any such
  edit silently reverted. Modern kernels already default `intel_iommu=on` when
  VT-d is present, so empty IOMMU groups mean VT-d is off in firmware, which
  only the BIOS/UEFI can change. IOMMU-off is now an honest hard fail for
  every vendor ("Enable IOMMU / VT-d / AMD-Vi in firmware."), matching the AMD
  path. Removed the unreachable `addCmdlineTokens`/`iommuFixable`/`pkgInRepo`
  dead code and their tests; `hwcaps_test.go` asserts the corrected verdict.

### Added
- **Security Key/Passkey Support**,
- Added security key/passkey support for authentication with `pam-u2f`.
- Added `ryoku security-key` commands for status, enrollment, removal, policy, and PAM wiring.
- Added support for FIDO2 PIN + touch authentication, including YubiKey FIPS devices that require user verification.
- Added passkey controls to Lockscreen Settings for sudo, admin prompts, and the SDDM sign-in screen.
- Added passkey enrollment and removal UI in Lockscreen Settings.
- Added SDDM greeter support for passkey PIN + touch login prompts.
- **Appearance / Look grew the rest of the decoration surface**, all live
  previewed: corner softness (`rounding_power`), dim-inactive with strength,
  blur X-ray, vibrancy and noise, shadow sharpness (`render_power`), the new
  glow (enabled, range, colour), drag-to-resize at edges (`resize_on_border`),
  floating-window snapping (`general:snap`), and the scrolling tiling layout
  next to dwindle/master.
- **Input covers the pointer and touchpad properly**: left-handed buttons,
  mouse natural scroll and scroll speed, middle-click paste
  (`misc:middle_click_paste`), numlock at login, touchpad tap-and-drag,
  click-by-finger-count, middle-click emulation and scroll speed, and the
  workspace swipe gained direction, create-new, and distance tuning
  (`gestures:*`).
- **Cursor comfort**: hide the cursor after an idle timeout or while typing
  (`cursor:inactive_timeout`, `cursor:hide_on_key_press`).
- **Animations rows gained a style picker** for the families that support one
  (windows pop-in/slide/gnomed, workspace slide/fade variants, layer styles);
  a base style like `popin 78%` shows as-is until you pick another.
- **Window rules: the full useful action set.** New actions: maximize, square
  corners, never dim, no animations, force opaque, blur X-ray, never take
  focus, hold focus (dialogs), keep aspect ratio, pseudo-tile, block
  idle/sleep (always/focus/fullscreen), and ignore app request
  (maximize/fullscreen/activate/activatefocus), the last two with inline
  choice values. Layer rules gained blur X-ray and show-above-lockscreen.
- Comfort tab surfaces a failure from `brightnessctl` or the night-light
  helper instead of pretending the slider applied.
- Themes now declare their decoration nuances (`roundingPower`,
  `blurVibrancy`, `blurNoise`) in `theme.json`'s look instead of raw Lua in
  `init.lua`, so the Appearance sliders reflect the live values after a theme
  applies; `init.lua` is motion-only. An already-applied theme's stale
  `theme.lua` self-heals on the next hub open (`hypr get`): its nuance values
  fold into still-default store fields (the live look does not move) and the
  copy is rewritten from the migrated, motion-only `init.lua`, so the sliders
  stop snapping back on Save.
- **Credits** section (pinned in the nav under a heart): a showcase "thank you"
  screen, Profile's twin in build and mood. kansha (感謝) meets the Three Graces
  of Greek myth: the marble trio dissolves off the right the way Lady Justice
  anchors the Profile (dissolve baked into the asset's alpha, so it melts into
  the canvas with no seam). The projects Ryoku stands on read as editorial type
  lines (name, then role · author in mono, hairline rules, no boxes), each
  opening its home with a click where one is known; a separate band credits the
  alpha and beta testers who keep finding the bugs. `CreditsPage.qml`, a `heart`
  glyph in `Icon.qml`, wired into `Hub.qml`; art (`art/three-graces.png`)
  generated with fal.ai and alpha-ramped locally.
- Every section whose GUI maps to a real config file gains a `CONFIG` chip next to
  its title (in the shared `PageHeader`): it opens that file in `nvim` (side by
  side, in a kitty window). The Hyprland sections (Input, Appearance, Animations,
  Keybinds, Window/Layer Rules, Autostart, Environment) open their base module
  beside `user.lua`, the file edits persist in (the Hub regenerates `settings.lua`
  from its JSON, so base-module and `settings.lua` edits do not survive, but
  `user.lua` loads last and always wins); Displays opens `monitors.lua` beside
  `monitors_user.lua`; GPU opens `gpu.lua`; Shell and Widgets open their `~/.config/ryoku/*.json`. The
  chip is hidden for sections with no editable file (Profile, Connections, Updates,
  Store, Add-ons, Lockscreen).
- The **Store**: the plugin Discover catalogue and the Extras bundles are unified
  into one section with a Plugins / Bundles switch. The separate Plugins and Extras
  nav entries are gone, and installed-plugin management moved to the Add-ons
  section, so the Store only browses and installs.
- An **Add-ons** section: installed plugins as a bento grid, each card opening that
  plugin's own settings - rendered from its `metadata.settings` schema by
  `PluginSettingsForm` (dropdown / toggle / slider / text controls grouped under
  mono headers), plus its enable toggle, host, and a remove action. Changes persist
  to `plugins.json` through `ryoku-plugins-place`, so the desktop retunes live; a
  plugin ships no settings QML, only the schema.
- Updates: `ryoku update` can pause on a question (enabling the snapshot helpers)
  and the Updates page renders it inline, in the `UpdateStatus` idiom: an ember
  rule, a headline and detail, and dossier-stamp Install / Skip actions (no
  centred pill modal). The tap writes the choice to the run-state back-channel the
  update is waiting on.
- Plugins: a plugin's settings now follow its lifecycle. `ryoku-hub extras` seeds
  a plugin's manifest presets into `plugins.json` (`<id>.settings`) on install and
  forgets the whole entry on uninstall, so a widget's settings appear with it and
  disappear when it is removed (through `ryoku-plugins-place seed` / `forget`).
- A **Desktop Widgets** section: a live editor for the clock and weather widgets
  on the wallpaper (`WidgetsPage`, with Clock and Weather tabs). Each tab pairs a
  live preview that mirrors the running design (`ClockPreview`/`WeatherPreview`,
  driven by the real wallust palette via a new hub `Wallust` singleton) with full
  controls: face/design, 12/24h and seconds, the date design, the C/F unit and
  today/week scope, the accent source, size, background (none/card/glass) and
  radius, placement (a snap zone or a free X/Y) with a desktop lock, and opacity. Edits write
  `~/.config/ryoku/widgets.json` throttled and atomically (the widgets host
  watches it, so the desktop retunes live), with Save/Revert/Reset and a
  leave-without-save restore, matching Shell Settings. Adds a `widgets` nav icon.
- A **Connections** section: Wi-Fi, Bluetooth, and Hotspot, each a subtab
  (`ConnectionsPage` + `WifiTab`/`BluetoothTab`/`HotspotTab`), ported from the shell's
  Link surfaces into the hub palette. Wi-Fi lists live networks (Quickshell Networking)
  with connect/disconnect, an inline password for secured unknowns (`nmcli --ask`, the
  secret fed via stdin), and rescan; Bluetooth toggles the adapter, scans, and
  connects/pairs (`bluetoothctl`); Hotspot toggles and edits the SSID/password for the
  `RyokuHotspot` AP (`nmcli`). The nav rail now scrolls when its sections overflow,
  keeping the 力 footer pinned.
- A **Profile** section: a showcase screen built for sharable rice screenshots. The
  shell pill's SYSTEM specimen card, reproduced as a hub-palette twin (`ProfileCard`,
  fed by a hub `SysInfo` reading the shared `ryoku-sysinfo`), sits on the left as the
  hero; a dossier (`ProfileStats`) sits on the right with a live clock, a vitals
  strip (load, CPU temp, processes, battery, displays), runtime spec lines
  (compositor, architecture, swap), a package wave with explicit/AUR/total counts,
  the look (cursor, fonts), and the wallust palette as a spectrum. Extended values
  come from a new `ryoku-profile-stats` helper; no addresses are shown, so the shot
  is safe to post. Drawn entirely in the card's carbon vocabulary (mono labels,
  hairline type-lines, tabular figures) over an ambient, vignetted backdrop.
- A **Lockscreen** section: the full qylock theme catalogue as a bento grid in the
  Appearance/Extras style, fetched live from the upstream repo (`ryoku-hub lock
  catalog`) so new and fixed skins appear without a Ryoku release. Each tile loops
  a preview of the real lockscreen (a local gif for the two vendored clockwork
  skins, the upstream `Assets` gif streamed for the rest, loaded only near the
  viewport). Selecting a skin makes it both the in-session lock (`ryoku-hub lock
  set`, writing `~/.config/qylock/theme`, the preference `lock.sh` reads) and the
  SDDM greeter (`ryoku-hub lock apply-greeter`, reinstalled under the fixed
  `/usr/share/sddm/themes/ryoku`); the greeter half lives on a system path, so it
  escalates with pkexec, leaving only the login/auth flow untouched. Skins not yet
  installed download first (`ryoku-hub lock install`, with the size shown up front).
  **Preview** launches an installed skin live without changing the selection;
  **Refresh** re-syncs; offline falls back to the installed skins.
- Shell Settings, Island tab: an **Appearance** group to choose the island style
  (Island, Floating, None) plus a **Reveal on hover** toggle that hides the island
  at rest and shows it on a top-centre hover. Writes `islandStyle` / `islandAutohide`
  to `shell.json` live, like the other shell knobs, with the frame left untouched.
- A **Themes** tab in Appearance: full-system "rices" as a bento grid in the
  Extras style. Each theme is a folder under `hyprland/themes/<slug>/` with a look
  (`theme.json`) and real Hyprland Lua (`init.lua`: motion design and decoration
  finish, the actual system change, not just colours). Applying one
  (`ryoku-hub hypr theme <slug>`) folds the look onto the appearance store (so
  Look/Borders reflect it), installs its `init.lua` (loaded before settings.lua,
  so a knob still wins), and reloads. Ships **Ryoku Default** (the shipped look),
  Tokyo Night, Aqua (glass), Catppuccin, Gruvbox, Nord, and Rosé Pine. Colours are
  a **separate toggle**, independent of the theme: they either track the wallpaper
  (wallust) or use the theme's own palette (locked so a wallpaper change keeps it)
  via `ryoku-hub hypr colorsource follow|fixed`. The frame and island stay Ryoku.
- The settings brand mark uses the real Ryoku icon asset instead of a procedural
  gradient square.
- **Ryoku Settings**: the hub is now a full settings app for everything the
  Hyprland (Lua) config drives, plus the shell. New sections, each a live editor:
  **Displays** (detect and arrange monitors on a drag canvas with snapping;
  per-monitor resolution, refresh, scale, rotation, adaptive sync, mirror, and
  enable/disable; Apply live or save hardware-keyed layout profiles, via
  `ryoku-monitor`), **Appearance** (gaps, rounding, borders, opacity, blur,
  shadows, layout, animations, border colours, cursor theme/size), **Input**
  (keyboard layout/variant/options, pointer feel, touchpad, key repeat), an
  editable **Keybinds** Custom tab beside the live legend, and **Window Rules**,
  **Autostart**, and **Environment** list editors.
- The override engine: `ryoku-hub hypr get|defaults|save|preview|restore` keeps a
  single JSON document at `~/.config/ryoku/hypr.json` and generates
  `~/.config/hypr/settings.lua` (only the values that diverge from the shipped
  defaults), which `hyprland.lua` loads after the base modules and before
  `user.lua`. Scalar edits preview at once via `hyprctl eval` (flash-free); Save
  persists and reloads. `cursors` and `layouts` enumerate installed cursor themes
  and X11 keyboard layouts. New `HyprStore` engine and `Dropdown`/`MonitorTile`
  components; new `DisplaysPage`, `AppearancePage`, `InputPage`, `WindowRulesPage`,
  `AutostartPage`, `EnvironmentPage`, `KeybindLegend`, and `KeybindsEditor`.
- Appearance also carries a **Wallpaper** tab (a grid that retheme the desktop by
  routing the pick through `ryoku-shell wallpaper`, so the wallust palette follows
  it) and a **Comfort** tab (backlight via `brightnessctl`, night light via
  `ryoku-cmd-nightlight`), both applied at once.
- An **Animations** section: the live Hyprland animation tree (read via `hyprctl
  animations -j`, never a hardcoded copy) with per-leaf enable/speed/bezier and a
  visual bezier-curve editor (drag the two control points, live preview). Curves
  and per-leaf overrides persist to `settings.lua` (`hl.curve`/`hl.animation`) via
  a new `anim` domain in the override store. New `AnimationsPage`, `BezierEditor`,
  and `AnimRow` components.
- Input gains a touchpad **workspace-swipe** gesture (3 or 4 fingers), emitted as
  `hl.gesture` and previewed live.
- A **Layer Rules** editor: blur/dim/no-animation rules on layer-shell surfaces by
  namespace, emitted as `hl.layer_rule` (new `layerRules` override domain,
  `LayerRulesPage`).
- A **Shell Settings** section: a live editor for the shell's look. **Frame** and
  **Island** tabs expose the knobs with the control each one wants, steppers with
  manual entry for exact pixels (radius, border, sizes, corners, gap), sliders for
  values tuned by eye (opacity, shadow strength, edge/bud melt), and a swatch with
  hex and dark presets for the surface colour, grouped under section headers. Every
  edit is applied to the running shell at once by writing `~/.config/ryoku/shell.json`
  (throttled, atomic), which the shell watches, so the preview is the actual desktop,
  the frame around the window and the island above it, not a mock pane. The action
  bar tracks the dirty state; Save keeps the look, Revert and leaving the section put
  the saved one back, and Reset to defaults restores the shipped values. A
  **Visualizer** tab tunes the desktop audio spectrum beside a live animated
  preview window: style (bars, wave, or dots), position (bottom, top, centre),
  shape (rounded or flat), mirror, on/off, band count, height, bar width, bloom,
  reflection, and the idle wave, writing `~/.config/ryoku/visualizer.json` the same
  live way. New `NumberField`, `SliderRow`, `Slider`, `ColorField`, `ToggleRow`,
  `ChoiceRow`, `SettingSection`, `VizPreview`, and `ShellSettingsPage` components.
- An Updates section wired to the `ryoku` CLI (`ryoku status --json`): a
  typographic status header (installed version, the pending-update count, and a
  live "checked Xm ago") over the real list of pending package updates (each
  `name old -> new`), with an automatic-check schedule in the top right (Off /
  Hourly / Daily / Weekly, persisted to the hub's TOML via an `update_interval`
  key) that re-runs the check on its cadence. "Update now" runs the real
  `ryoku update` in a terminal; the page mirrors its progress from the run-state
  file the CLI publishes (a spinner and a Ryoku wave while it applies), and the
  shell's update island reads the same file. A live count badge rides the nav
  rail. When the system is current there are no rows ("Everything is up to
  date") and the island stays hidden.
- Ryoku Hub: the desktop's central control center, a native Qt6/QML app
  (Quickshell, not a webview) with Kirigami-style sidebar navigation. Opened with
  `Super + ,`; it floats and centres on top of the current windows via a Hyprland
  window rule.
- `backend/` `ryoku-hub`, a Go data plane the QML shells out to: `ryoku-hub
  keybinds` parses the live Hyprland binds (`~/.config/hypr/modules/binds.lua`,
  the single source of truth) into categorised JSON, prettifying key tokens and
  deriving descriptions from each bind's comment or dispatcher; `ryoku-hub config
  get|set` persists hub state as TOML at `~/.config/ryoku/hub.toml` (atomic
  write), starting with the last open section.
- `quickshell/` the UI: a `FloatingWindow` with a navigation rail and a content
  area. The **Keybinds** section is functional, rendering the full shortcut
  legend as a flat list (ember section headers with a hairline rule, mechanical
  keycaps). **Extras** is under construction (its controls will likely use GTK4 +
  libadwaita through the Kirigami addons).
- A global fuzzy finder in the sidebar, focused with `Ctrl + K`. It searches
  content across every section: fuzzy-ranked keybinds (tagged with their
  category) and matching section names you can jump to. The matcher is a small
  subsequence scorer in `quickshell/fuzzy.js`.
- Visual language follows the shell: a deep warm canvas with the brand orange as
  the single deliberate accent, the 力 mark, JetBrains Mono keycaps, and the
  shell's morph motion (a single sliding selection indicator in the rail).

### Changed
- Colour sourcing is one master, not three toggles. "Colours follow wallpaper"
  in Appearance -> Themes is the single switch; the Borders "Follow wallpaper
  palette" and Shell "Match wallpaper" toggles are gone, and window borders and
  shell chrome now follow that one master (read from `theme.json`
  `followWallpaper`). The Borders tab shows its fixed-colour fields only when the
  master is off. The backend no longer runs `wallust` directly: `applyTheme`,
  `setFollowWallpaper`, and `applyScheme("follow")` write their intent and call
  `ryoku-shell wallpaper repaint`, so a theme apply or a toggle derives the same
  palette as a normal wallpaper change (honouring the ryowalls per-image tune).
- Single-select controls read as one family. The active choice in `ChoiceRow`
  (Input, Displays, GPU, Shell, Widgets, Appearance) and the pick-one pills for
  plugin placement and host now wear the same dark raised pill (`keyTop` +
  hairline, bright label) as the `Segmented` tab bar and the nav rail, instead of
  a solid ember block. The brand orange stays an accent, never a fill.
- **GPU section is graphics-only.** It chooses which GPU Ryoku renders on (Hybrid
  / Performance / Passthrough) and sets up the optional GPU-passthrough stack
  (binds the discrete GPU to vfio so a VM can own it), with the readiness dossier
  behind a disclosure instead of filling the page. The specimen card is a sibling
  of the Profile card (carbon, holographic wash, cursor foil, parallax tilt, the
  Ryoku wave): a VRAM badge, the render GPU as the hero, and both GPUs in a dossier
  box with a DISPLAY/FREE marker. Running virtual machines moved to the **ryovm**
  app: the Machine tab, the windowed-VM launcher, and the `ryoku-hub vm`
  subcommand (with `qemu.go`/`vmrun.go`/`vmxml.go`/`vmsnapshot.go`) are removed;
  the hub no longer launches or manages VMs.
- Add-ons: a plugin's `image` setting opens the system file chooser (via the
  desktop portal) on click, instead of a raw text field.
- Store: the catalogue refresh moved to a single control to the left of the
  Plugins / Bundles switch, refreshing whichever catalogue is shown; the embedded
  pages no longer show their own refresh inside the store.
- Add-ons: the per-plugin settings detail drops the showcase backdrop (the warm
  glow read muddy behind a form); the bento grid keeps it.
- Plugins store: the Install / Installed / Remove actions match the update consent
  prompt - outlined ember "stamp" buttons (a thin ember border + mono uppercase
  label, no fill) for the live action and hairline ghost stamps for the rest,
  replacing the generic bright full-pill.
- The sidebar brand is a centred masthead: the wide-tracked **RYOKU ARCH**
  wordmark and a "system and shell settings" subtitle sit over a dimmed 力
  backdrop (a soft warm glow and a faint grid, echoing the Profile portrait
  window). The old flat logo image is gone (`brand-icon.png` deleted) and the
  expand/collapse-all control moves beside the search so the masthead owns its row.
- The Profile showcase frame (soft glow, faint grid, vignette, corner ticks) is
  extracted to a shared `ShowcaseBackdrop` and now sits behind the **Store** and
  **Add-ons** pages too, so the three screens read as one family; the Add-ons
  bento cards gain a lifted drop shadow.
- `HubButton` (the Save/Revert/Reset bar on every settings page) and every primary
  action across the hub (Store Install, the update consent Install) are outlined
  ember stamps in the Profile dossier idiom: a small-radius carbon chip with a mono
  uppercase label and ember as a thin accent (border + label), never a fill.
- The sidebar is rebuilt as three bands: **Profile** pinned at the top on its own
  (no more single-item "You" drawer), the functional groups scrolling in the
  middle, and **Updates** pinned at the foot so its pending-update badge stays in
  view. The middle groups are **System** (Displays, Input, Keybinds, Connections),
  **Desktop** (the visual sections), **Add-ons** (Plugins, Extras), and
  **Advanced** (Window/Layer Rules, Autostart, Environment) at the bottom. Each
  group is a drawer that animates open and closed; only the group holding the
  current section starts open, and an expand/collapse-all control in the header
  reveals or tucks the rest. Group headers adopt the Profile dossier idiom (a
  brand accent dot, a larger mono label, hairline rules between bands and groups),
  and nav rows gain a hover highlight that previews the selection.
- Renamed the product to **Ryoku Settings** (window title, sidebar, the `Super +
  ,` legend, and the float/centre window rule), keeping the internal `ryoku-hub`
  binary, `quickshell/hub` config, and `qs -c hub` invocation. The sidebar is now
  grouped (Displays & look, Input & shortcuts, Session, Desktop) and the hub opens
  on Displays by default.
- `backend/`: the keybind legend parser keeps lambda binds (multi-dispatch
  actions like `Super + A`, which floats and centres) in the legend, taking the
  description from the trailing comment, instead of dropping every bind whose
  action is not a bare `hl.dsp` expression.
- The **Updates** section tracks the git update channel (`main`) instead of pacman
  packages: the status header, the count badge, and the list show the commits the
  checkout is behind `origin/main` (subject + short hash), driven by the `channel`
  field `ryoku status --json` now publishes. "Up to date" shows when current.

### Fixed
- **Passkey PAM keeps PIN verification.** Applying sudo, admin-prompt, or SDDM
  targets now preserves the configured FIDO2 PIN policy instead of falling back to
  touch-only PAM lines.
- **The SDDM greeter shows passkey prompts.** Passkey login now surfaces PAM touch
  prompts and accepts the FIDO PIN through the sign-in field instead of leaving
  the greeter stuck on a static waiting message.
- **Passkey setup opens an interactive terminal.** Settings now launches
  enrollment in a terminal so PIN-based keys can prompt correctly, then refreshes
  the passkey list after setup.
- **Passkey management has its own home.** Lockscreen Settings separates passkey
  policy from fingerprint management and lists enrolled passkeys with remove
  actions in the right-hand management column.
- **The login screen keeps the lock skin you pick.** Choosing a lockscreen skin
  applies it as the SDDM greeter too, but a skin pulled from the catalogue
  downloads into a 0700 user-owned dir (`os.MkdirTemp`), and the `cp -a` into
  `/usr/share/sddm/themes/ryoku` preserved that owner and mode. The greeter runs
  as the unprivileged `sddm` user, which then could not read the theme, so SDDM
  silently fell back to its embedded theme on every boot (it bit only users who
  had switched skins). `installGreeter` now normalizes the installed greeter to
  root-owned and world-readable, so the picked skin survives a reboot. Covered by
  `lock_test.go`; `ryoku doctor` heals boxes already broken this way.
- **Input: "Follow mouse" now takes effect.** The Hub's input default was out of
  sync with the shipped Hyprland config (`input.lua` ships `follow_mouse = 2`, the
  backend and the QML store both assumed `1`), so the diff-based generator wrote no
  override when "Normal" was picked and the base config's click-to-focus stayed in
  force. Aligned the default to `2`, so choosing "Normal" now writes
  `follow_mouse = 1` and the window under the cursor becomes active on hover.
- **Enable passthrough now installs the AUR stack.** Looking Glass and the kvmfr
  module are AUR-only, but the privileged `gpu apply enable` ran under pkexec and
  only `pacman`-installed the official core, so it printed a `yay -S` hint and left
  kvmfr absent: passthrough never turned on, even after a relogin. Enable now
  builds the AUR pieces as the invoking user (the Hub launches it in a floating
  terminal), then escalates for the system setup, and the plan reports honest
  state instead of claiming an install it never performed.
- Plugins: installing a plugin now fetches every file it declares, not just its
  entry points. `ensurePlugin` also pulls a `files` manifest array (helper QML and
  assets), so a multi-file plugin (one whose view imports a sibling component, or
  ships a sample image) installs complete instead of rendering nothing.
- Plugins / Extras: a catalogue refresh now reflects a just-pushed change at
  once. `ryoku-hub extras` fetches bypass the GitHub raw CDN cache (a unique
  query param plus a `no-cache` header), which had served a stale `registry.json`
  for minutes, so a newly published addon stayed invisible until the CDN expired.
- Ryoku Hub: `Super + ,` no longer goes dead after the hub is dismissed with the
  compositor's close (`Super + Q`). The keybind guards against a second instance
  with `flock` held for the life of the `qs -c hub` process; an external close
  only hid the window while the process kept running, pinning the lock so further
  presses silently no-opped. The `FloatingWindow` now quits on its `closed`
  signal, so every dismissal releases the lock.
- Animations: the bezier editor clamps control points to Hyprland's valid range
  (X in [0,1], Y in [-1,2]) and the canvas spans exactly that, so a handle dragged
  off-canvas can no longer write an out-of-range curve that broke `settings.lua`
  on reload (and silently stopped every other override from applying).
- Displays: Mirror and Extend are disabled with a single monitor, and a failed or
  empty detection shows a clear message with a Retry button instead of a permanent
  "Detecting displays" (most often a stale `ryoku-monitor` without the `list`
  subcommand; see the shell deploy fix).
- Appearance: the tab content is top-aligned instead of vertically centred, so the
  shorter tabs (Cursor, Wallpaper, Comfort) no longer open with a large gap.
