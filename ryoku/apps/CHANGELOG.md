# Changelog: ryoku/apps/

## Unreleased

### Added
- `ryostore/`: **Ryostore looks like the Hub and shows products properly.** The
  store moves to the Hub's rail, page head and action bar, with Discover,
  categories, search and Library in the rail. An opened product leads with its
  media: screenshots are decoded at the size they are shown, never stretched
  or cropped, GIFs and animated WebP play only while visible and focused, video
  previews play muted, and the lightbox switches between Fit and 100% with
  panning. Products with a newer version show UPDATE, and a preview that is
  missing or fails falls back to the product's initials instead of an endless
  placeholder (`ryostore/quickshell/ProductDetail.qml`,
  `ryostore/quickshell/ProductMedia.qml`).
- `ryostore/`: **Ryostore moves.** Cards lift and their covers ease in on
  hover and press in on click, the rail's selection plate slides between
  categories, results settle in with a short stagger when you change category,
  search, provider or sort, the hero crossfades, an opened product settles in,
  and Install shows its progress and a brief installed check. Everything rests
  still and sharp, and follows the reduced-motion setting
  (`ryostore/quickshell/ProductCard.qml`, `ryostore/quickshell/InstallAction.qml`).
- `ryostore/`: **The Omarchy plugin market, inside Ryostore.** While Nomarchy is
  the bar style, an Omarchy plugins category lists the plugins.omarchy.org
  catalogue (cached, with Popular, New and Verified orders and search), shows
  each plugin's preview, licence, repository and verification state with an
  unsandboxed-code warning, and installs, updates, enables or removes through
  the `omarchy` CLI. A plugin the running compositor cannot host is refused
  with the reason (`ryostore/backend/provider_omarchy_plugins.go`).
- `ryovm/`: **Ryoport reads as a fleet dashboard.** The Dashboard puts local
  VMs and remotes in one resource list with a fleet summary strip, and the
  selected resource gets CPU, memory, disk and network graphs beside its facts.
  Activity is now scoped to that resource, explains what it records, and shows
  observed machine state and remote health transitions with warning reasons
  alongside connects, tunnels, snapshots and other actions. VM and remote
  details are tabbed (Overview, Access, Networking, Snapshots or Workloads,
  Settings, Activity), the empty Machines and Passthrough plates lead with
  their setup action, and sampling runs only while a plate is on screen.
  `ryovm-mon` reports disk, network and uptime, `ryossh` samples CPU and I/O,
  and `ryovm lg stats` covers passthrough VMs
  (`ryovm/quickshell/Dashboard.qml`, `ryovm/quickshell/Singletons/{Vm,Remotes}.qml`).
- `starship/`, `fastfetch/`: **Prompt layouts, and colours that follow the
  wallpaper.** Starship ships five layouts (Pill, Minimal, Two line, Powerline,
  Lean), picked in Ryoku Hub > Keybinds with rendered previews, and either keeps
  its fixed palette or follows the wallpaper through matugen. Fastfetch can
  follow the wallpaper too: matugen writes `fastfetch/ryoku-colors.json` and
  `ryoku-fastfetch` merges it into a cached copy of your config, so Hub and
  store edits to `config.jsonc` are never overwritten. Both choices survive
  updates (`../hub/backend/starship.go`, `../hub/backend/fastfetch.go`).
- `rashin-app/`: **The window opens on the Ryoku lane.** The first sheet is the
  machine agent; Chat and the new Wiki sheet sit beside it.
- `rashin-app/`: **The window carries Rashin's own icon.** `logo.svg` links to
  the brand seal (`assets/brand/rashin-mark.svg`), which the package and the
  dev deploy both install as `rashin-app` in the hicolor theme; it was
  falling back to the Ryoku 力 mark.
- `rashin-app/`: **Rashin, the AI companion window.** A GTK3 + WebKitGTK
  host for the console `ryoku-rashin` serves: it waits on a paper boot page
  until the daemon answers, loads the chat, opens outside links in the
  browser, grants notifications, remembers its size, and stays single
  instance. The console itself is `ryoku/rashin/web` (Svelte 5 + bits-ui +
  the Libraries.dev effects): the shared agent chat as a workspace, the
  fast-lane Ask, and every dashboard sheet. It opens from Super+Alt+Space or
  the launcher and wears the desktop's live palette. Every fact is the
  daemon's; the app only asks and paints (`apps/rashin-app/`).
- **A Ryostore fastfetch preset can bring its own renderer.** A preset that
  needs to draw something fastfetch cannot (icat PNGs, for one) names an
  executable in its own product folder with a `// ryoku:renderer` comment and
  `ryoku-fastfetch` hands off to it; anything invalid falls through to
  fastfetch (`fastfetch/ryoku-fastfetch`).
- `ryostore/`: **Kairos joins the built-in bar styles in the catalogue.** The
  bar-style provider now lists the shell's island-clock style beside Sumi and QS
  Bar, so Ryoku Settings' Bar Studio shows it as an installed, selectable card
  and `ryostore install` / `remove` refuse it as built-in
  (`backend/provider_bars.go`).
- `ryostore/`: **a Remove button on every installed item.** RyoStore could only
  install; taking something back off meant leaving the app. The product dossier
  and the showroom hero now show a REMOVE action whenever an item is installed
  (any category -- theme, decor, lock, bundle, plugin -- since every provider
  backend already implements remove), wired to `ryostore remove <category> <id>`
  through a new `Store.remove` that reuses the install lifecycle and refreshes
  the catalogue when it is done (`Singletons/Store.qml`, `ProductDetail.qml`,
  `ShowroomStage.qml`, `App.qml`).
- `ryovm/`: **a Looking Glass lane for GPU-passthrough VMs.** Ryoport grows a
  fourth section (rail + `Ctrl+4`) that manages passthrough machines: point it
  at an install ISO and pick the guest, and it defines a tuned `ryoku-<name>`
  libvirt domain (CPU pinning, Hyper-V enlightenments for Windows, virtio disk,
  the dGPU + its audio as vfio hostdevs, the kvmfr Looking Glass shared-memory
  device); one button starts it (the vfio hook binds the dGPU) and opens
  `looking-glass-client`. The lane gates on the GPU page's readiness verdict and
  states the blocker when passthrough is not ready, and a standing checklist
  names the guest-side steps that cannot be automated (OS, virtio drivers, the
  Looking Glass host app, and a virtual-display driver for laptop dGPUs). CLI:
  `ryovm lg <name>` (or the `ryoport` alias) starts a machine and opens Looking
  Glass. Strictly passthrough, always Looking Glass -- quickemu cannot pass a
  GPU through, so this is a separate engine (`Singletons/Lg.qml`,
  `PassthroughPage.qml`, `bin/ryovm` lg verb, `bin/ryoport`).
- `fastfetch/`: **the OS line carries the release name** ("Ryoku Onogoro
  v0.56.0-beta.19") via `ryoku version --pretty` (`config.jsonc`).

### Fixed
- `fastfetch/`: **The readout names the distribution it runs on.** The tagline
  said "a hand-built Arch desktop" on every system. It now reads the base
  distribution through `ryoku-base-os` (Arch and CachyOS still say Arch, Void
  says Void), whether it runs through `ryoku-fastfetch`, as a plain
  `fastfetch`, or in the Hub preview (`fastfetch/config.jsonc`,
  `terminal-shell/env.sh`, `fish/config.fish`).
- `ryovm/`: **Ryoport's OS logos stop going blank.** The catalogue prefetch and
  the logos on screen could fetch the same OS at once into one temp file, and
  both tinted it, so the cached SVG carried `fill` twice. That is not valid XML,
  so the Android, Alpine and similar logos drew nothing for up to two weeks.
  Each fetch now has its own temp file, the tint is added once, and a logo
  already broken this way is fetched again (`bin/ryovm`).
- `ryovm/`: **opening a remote in Ghostty no longer raises the terminal's own
  config error window, and the float rule matches again.** The launches built
  `--class <name>` (kitty's form) and Ghostty wants `--class=<name>`, so it read
  the bare flag as valueless, then parsed the name as a second field: every
  connect showed `cli1:class: value required` plus `cli2:ryoport-ssh: invalid
  field`. The flag is now the `=` form, which kitty accepts too. The value moved
  to `dev.ryoku.ryoport_ssh` because Ghostty validates `--class` as a GTK
  application id, which needs a dot: with the old name Ghostty ignored the class
  even in the right form, the window kept `com.mitchellh.ghostty`, and the
  float-and-centre rule could never match it. `ryossh connect` gained the
  missing Ghostty branch in its terminal switch (issue #280).
- `fastfetch/`: **the greeting reports the real shell again.** The wrapper
  bounded fastfetch with `timeout 8`, but fastfetch's shell module walks the
  parent chain and skips known wrappers (`time`, `sudo`, ...) without knowing
  `timeout`, so every greeting read "Shell: timeout". Each branch now `exec`s
  fastfetch, which replaces the wrapper process and leaves the user's shell as
  the direct parent.
- `ryostore/`: **An installed theme now carries the store's preview image, so
  the Color-scheme picker shows it.** The install wrote `scheme.json` and
  `meta.json` and nothing else, while Ryogami's Themes tab looked for
  `preview.jpg` beside them and fell back to palette pills for every store
  theme. The install now fetches the catalogue's preview (through the store's
  asset cache, so the card and the install share one download) and writes it
  as `preview.<ext>`; an unreachable preview still installs the scheme. The
  shell's `theme catalog` reports the art it finds (`preview`) and the picker
  reads that instead of guessing a filename, so `.png` and `.jpg` sources both
  show (`backend/provider_colorschemes.go`, `shell/ipc/usertheme.go`,
  `ryogami/wall-ui/.../WallpaperSelector.qml`).
- `ryostore/`: **A theme's own wallpapers come with it.** Every HANCORE scheme
  ships the backgrounds it was drawn for; the catalogue now lists them
  (`wallpapers`, pinned to a commit) and the install lands them in
  `~/Pictures/Wallpapers` as `<id>-N.<ext>`, where the picker shows them beside
  the rest. Remove takes exactly those files back out.
- `ryogami/`: **The picker refreshes live.** The Themes and Rices strips were
  read once and cached for the picker's lifetime, so a scheme installed while
  Super+W was open never appeared until a reopen. Both are reloaded on every
  open and re-read when their library folders change (a native folder watch,
  no external tool). The wallpaper strip's own watcher execs `inotifywait`,
  which nothing declared: `ryogami` now depends on `inotify-tools`, so a wall
  that lands while the picker is open shows up on user boxes too.

### Removed
- `ryotunes/`: **The Chromium app-window wrapper is gone; Ryotunes is now a real
  app.** YouTube Music ran as a Chromium `--app` window in its own profile
  because the Tauri/Electron clients of the time crashed on this compositor or
  published no MPRIS. The Ryostore-submitted Ryotunes (Tauri + libmpv,
  `ryoku-dev/ryotunes`) does both, so it ships as the `ryotunes` package from the
  `[ryoku]` repo (`release/packages/ryotunes/`) and the wrapper, its `.desktop`
  and icon leave this tree. The desktop's music integration is unchanged: it
  follows `org.mpris.MediaPlayer2.ryotunes`. Super+J now launches Ryotunes
  directly (it is single-instance, so a second press focuses the window)
  instead of toggling the `special:music` scratchpad, and the music widget's
  corner button runs the music app the same way; the `ryoku-music-toggle`
  script that tucked the window into that scratchpad is gone. A dev checkout
  (git channel, which never publishes packages) gets the app too: `ryoku
  deploy` builds `release/packages/ryotunes` with makepkg and lays the binary,
  launcher and icons into `~/.local`, rebuilding only when the pinned commit
  changes.

### Fixed
- `ryowalls/`: **A download that returns an error page or a Git LFS pointer no
  longer poisons the wallpaper folder.** `curl` fetches those with a 200 and
  exit 0, so ryowalls saved a few hundred bytes of ASCII as a `.png`; the
  catalog could not thumbnail it and the picker silently dropped the row, so a
  "saved" wallpaper never appeared. Every download verb now validates the file
  is decodable media (`identify` for images, `ffprobe` for clips) and, on a bad
  file, removes it and fails so the UI reports "Download failed" instead
  (`ryowalls/bin/ryowalls`).

### Added
- `ryostore/`: **Plugins browse as ALL / BAR / DESKTOP, and community plugins
  carry a warning.** The Plugins tab gets the same subtab strip Themes and Decor
  use: BAR is every plugin whose `hosts` includes `topbarGlyph`, DESKTOP the
  rest. A plugin whose registry entry is not `official: true` shows a COMMUNITY
  tag and a warning band on its detail (Ryoku does not review or maintain it; it
  runs in your shell with your permissions), and the backend marks it
  `metadata.community` (`quickshell/App.qml`, `ProductDetail.qml`,
  `lib/store.js`, `backend/provider_plugins.go`).

- `ryostore/backend`: **`ryostore install plugins <id> --from <dir>` installs a
  plugin from a local directory through the same supply-chain transaction as a
  registry install.** It builds a ProductManifest by walking the directory
  (hashing every regular file, skipping symlinks and `.git`), takes the version
  from the plugin's `manifest.json`, and feeds the existing `installProduct`
  transaction with the file bytes read from the directory instead of the cache,
  so the receipt, the content-hashed view, and the journal are written exactly as
  for a store install. This is what lets `ryoku plugin add` produce a plugin the
  shell's `discover.sh` actually loads (`ryostore/backend/provider_plugins_local.go`,
  `product_transaction.go`, `main.go`).
- `fish/`, `bash/`, `zsh/`, `terminal-shell/`: **Fish, Bash, and Zsh now
  carry the same Ryoku terminal tools.** All three initialize Starship, zoxide,
  mise and fzf, share the same eza aliases and environment, load user overrides
  last, and expose Rashin buffer replacement, Alt+R, learning hooks and recipes.
  Bash uses ble.sh; Zsh uses its packaged highlighting, suggestion and history
  plugins. Kitty no longer pins Fish, so it and any other terminal that follows
  the session environment opens the account-wide shell selected in Settings;
  Fastfetch reads that same session value instead of mistaking its timeout
  wrapper for the active shell.

- `wireplumber/`: **Bluetooth earbuds that offer neither LDAC nor aptX stop
  sounding terrible.** `bluez5.codecs` ranked `aac` above `sbc_xq`, so a device
  without a hi-fi codec always landed on AAC, and on Linux that is the weaker
  encoder: PipeWire's AAC typically tops out around 352 kbps while SBC-XQ
  sustains over 500, and Samsung's AAC in particular is poorly regarded. That one
  ordering is why the complaint was brand-shaped rather than universal, because
  only devices without LDAC or aptX ever reached the AAC rung. `sbc_xq` now
  outranks `aac`; LDAC and aptX still win outright when a device offers them, so
  nothing good was given up.

  `bluez5.codecs` is a monitor property, so that order is global and cannot be
  expressed per device. The devices whose AAC really is better (Apple's,
  typically) are served by `ryoku-bt-audio` instead, which remembers a codec per
  device and puts it back on reconnect.
- `ryostore/`: **the Discover page rotates daily instead of showing the same
  thing every day.** The hero and the item order are now seeded by the day
  number, so Discover holds still while you browse but reshuffles each day; it
  reaches for the same eligible pool (real art, not installed) for the hero.
  Search, category and Library views are untouched, and callers that pass no seed
  keep the old deterministic order (`quickshell/lib/store.js` `shuffleSeeded` and
  a seeded `featured`, driven by `quickshell/App.qml` `discoverSeed`).

### Fixed
- `ryostore/`: **the catalogue shows art in its own colour, not dithered.** Every
  product carries two previews, the dithered bake (`art`) and the colour original
  (`artRaw`), and every surface reached for the bake: browsing emblems, decors and
  fastfetch layouts meant a wall of 1-bit bone where the actual artwork is
  colourful. Tiles and the detail plate now lead with the colour original, the
  detail view's DITHER toggle starts off and shows the bake on demand, and an
  install with no explicit choice takes the look you were shown.
- `tools/`: **"Compress video" and "Install app" open again.** Both entries ran
  `hyprctl dispatch global ryoku:<name>`, and this Hyprland takes Lua, so the
  dispatch exited 7 with a parse error and the launcher entry did nothing. They
  dispatch `hl.dsp.global('ryoku:<name>')` now, quoted so the Desktop Entry parser
  hands it over in one piece (verified through `gio launch`).
- `ryostore/`: **opening the Store from the Hub no longer risks a second
  desktop.** `openConfig` runs `flock ... qs -c <config>` and then a `qs -c
  <config> ipc call`, and `ryostore` is launched from surfaces that can carry
  Quickshell's crash-recovery handle (`__QUICKSHELL_CRASH_INFO_FD`, read before
  any argument is parsed), which makes both of those relaunch the desktop config
  instead of the store. `ryostore` clears the crash variables from its own
  environment at startup (`backend/crashenv.go`), covered by a test that proves a
  real child no longer inherits them.
- `ryostore/`: **removing a plugin from Settings no longer strands it in the
  Store.** Add-ons > REMOVE calls `ryostore internal remove-guest plugins <id>`,
  which only deleted the data directory: the receipt, the index row and the
  cached view survived, so the Store kept the card badged INSTALLED with its
  install button disabled and the plugin could never be reinstalled -- while the
  desktop no longer had it. A receipt-owned plugin now leaves through the product
  transaction (files, receipt, index, view), and a dev plugin without a receipt
  keeps the plain symlink-safe unlink (`ryostore/backend/extras_assets.go`).
- `ryostore/`: **a removed or updated plugin stops leaving a second copy of
  itself in state.** Nothing pruned `~/.local/state/ryoku/store/plugin-views/`,
  so an uninstalled plugin's whole source tree stayed behind forever and every
  update added another digest directory beside the live one. The index rebuild
  now drops every view the index does not name
  (`ryostore/backend/provider_plugin_views.go`).
- `pipewire/`: **the audio device you pick now survives a reboot.** The
  `pipewire-pulse.conf.d/10-ryoku-switch-on-connect.conf` drop-in loaded the
  PulseAudio compat `module-switch-on-connect`, which makes the default sink
  follow any device as it appears; every device "appears" fresh at boot, so it
  overrode WirePlumber's saved default on every login and the machine always came
  back on the old sink no matter what was selected (in the shell or pavucontrol).
  Removed the drop-in: WirePlumber persists the chosen sink/source across reboots
  on its own. The Bluetooth mic-profile auto-switch is unaffected (WirePlumber
  native, `wireplumber/wireplumber.conf.d/51-ryoku-bluetooth.conf`). `ryoku
  materialize` prunes the stale drop-in from existing installs on the next update.

### Added
- `mimeapps.list`: **videos and audio open in mpv, not HandBrake.** Installing
  HandBrake made it the default handler for `video/*`, so clicking a stashed
  download (or any video) opened the transcoder instead of playing. The default
  map now routes the common video and audio types to `mpv.desktop` (the shipped
  player), so `xdg-open` (and the stash's file-open) plays them
  (`apps/mimeapps.list`).
- `ryostore/`: **the store caches its imagery and flags real updates.** Previews
  are pulled to a local cache (`~/.cache/ryoku/extras/assets`) by a detached
  `ryostore warm` and served from disk, so the store opens instantly, works
  offline, and stops re-downloading every launch. A new `ryostore check` compares
  a content revision of the live registries against the last one the user pulled
  and lights a red dot on Refresh only when ryoku-extras has actually changed
  (`apps/ryostore/backend/{asset_cache.go,catalog_revision.go,main.go}`,
  `apps/ryostore/quickshell/{Singletons/Store.qml,StoreHeader.qml}`).
- `ryotunes/`: **YouTube Music as a first-party Ryoku app, wired to the desktop
  now-playing widget.** A dedicated Chromium app-window on music.youtube.com,
  single-instanced by a flock and isolated in its own profile, so it carries its
  own login and its own MPRIS identity -- which the desktop music widget follows
  and retints to. Chromium is the wrap because Ryoku already ships it and its
  media session speaks MPRIS on Wayland out of the box, where the Electron
  (pear-desktop / glassy) and Tauri (zuno) YTM clients respectively crash on the
  compositor or publish no MPRIS at all. Ships as a launcher on PATH, a `.desktop`
  and an icon; a float window rule gives it the music-player treatment
  (`apps/ryotunes/{bin/ryotunes,ryotunes.desktop,ryotunes.svg}`,
  `hyprland/modules/window_rules.lua`, `shell/deploy.sh`,
  `release/packages/ryoku-desktop/PKGBUILD`).
- `ryostore/`: **A Themes category delivers third-party colour schemes.** It
  serves the `ryoku-extras` colorschemes catalogue through a per-provider subtab
  strip (the HANCORE-linux Omarchy themes and the existing Noctalia set) with a
  My themes tab for the installed library and a per-provider Install all. A
  scheme installs (install-only) into `~/.local/share/ryoku/themes/<id>`, where
  the shell picks it up so it shows in the Color-scheme section (Super+W and the
  Hub); schemes without a preview render an accent/surface swatch tile
  (`backend/provider_colorschemes.go`, `backend/catalog.go`,
  `quickshell/{ProviderTabs,App}.qml`, `quickshell/Singletons/Store.qml`,
  `quickshell/lib/store.js`).
- `ryostore/`: **Bundles preview their contents and install by selection.**
  Opening a bundle lists its components grouped Core / Optional, each with a
  toggle (Core on, Optional off) and a one-line summary. Two actions replace the
  single install: INSTALL SELECTED (only the toggled items) and INSTALL ALL. The
  selection threads to the actuator as `install bundle <id> --only ...`; the
  detail is a scrollable list so a 40-item bundle stays navigable
  (`quickshell/ProductDetail.qml`, `quickshell/Singletons/Store.qml`,
  `backend/{main,provider_bundles}.go`).

### Changed
- `ryostore/`: **The store navigation is a proper two-tier app bar.** The old
  flat row crammed identity, a hidden "SEARCH" overlay trigger, Library,
  Refresh, and every category into one strip, so categories clipped off-screen
  with no affordance. Tier one now carries the 力 wordmark, a persistent search
  field (magnifier, always visible, Ctrl+K focuses it, live result count) and
  the Library / Refresh actions; tier two lays every category out as bone-invert
  Tabs plates so none are hidden. The `SearchLayer` overlay is gone, folded into
  the header field (`quickshell/StoreHeader.qml`, `quickshell/App.qml`).

### Fixed
- `ryostore/`: **closing the store no longer crashes Quickshell.** Quitting
  straight out of the window-close handler raced the QML engine teardown into a
  pure-virtual delete (`std::terminate`) while remote image loads were still in
  flight -- intermittent, but exactly the reported crash. The store now stops its
  child processes and defers the quit past the close event and, with previews
  served from the local cache, tears down with no network image loads
  outstanding (`apps/ryostore/quickshell/{App.qml,Singletons/Store.qml}`).
- `ryostore/`: **Installing a rice no longer 404s on the manifest.** The rice
  provider used the registry's `manifest` value (the published bare `rice.json`)
  as a repo-root path, fetching `.../main/rice.json` (404) instead of the rice's
  own `rices/<id>/rice.json`. A bare filename now resolves under the rice's
  directory -- matching every other extras category and the provider's own
  empty-manifest fallback; a value that already carries a path is still honoured
  (`ryoku/apps/ryostore/backend/provider_rices.go`).
- `ryostore/`: **Installing a rice from a local (`file://`) extras base now works.**
  The catalogue browse read a `file://` base straight off disk, but the rice
  install still HTTP-fetched the manifest and bundled assets, so a local base
  failed with "unsupported protocol scheme file". The install path now reads
  `file://` sources from disk like the catalogue does, so a checkout under test
  (`RYOKU_EXTRAS_BASE` or `~/.config/ryoku/ryostore-base`) installs end to end
  (`ryoku/apps/ryostore/backend/provider_rices.go`).
- `nautilus/`: **The right-click menu no longer shows duplicate "Compress with
  Ryoku" (or "Install with Ryoku") entries.** nautilus-python could register the
  stash extension twice in one nautilus process (an extension reload after a
  deploy, a re-import), doubling every entry; the provider now binds once per
  process (`apps/nautilus/ryoku-stash-menu.py`).
- `ryowalls/`: **The source picker actually switches source now.** Choosing a
  row (MoeWalls, Live, a library, …) dismissed the drawer but left you on
  Wallhaven. The filter field's "Enter picks the top match" was wired to Field's
  `committed`, which rides `editingFinished` and so also fires on focus loss --
  and dismissing the drawer after a row pick moves focus away, re-selecting the
  top row (Wallhaven) and clobbering the choice. It now uses Field's `accepted`
  (Return only), matching the Picker grammar (`quickshell/SourcePicker.qml`).
- `ryowalls/`: **The live preview now shows the exact colours Set will apply.**
  Its palette command copied the picture to an extensionless temp file, which
  matugen cannot decode, so the palette came back empty and the preview -- the
  candidate strip and the cava spectrum -- fell back to stand-in colours; even
  when it did run it used a second matugen invocation that diverged from the
  daemon (no scheme/mode/neutralisation). The daemon now owns generation behind
  one command (`ryoku-shell matugen-preview`) that emits exactly what apply
  writes -- the shell palette, the tonal ramps, and the wallpaper's L* map --
  and the preview's cava paints tones off the image's own primary/secondary
  ramps against that map, mirroring the desktop visualiser
  (`bin/ryowalls`, `quickshell/Singletons/Wallhaven.qml`,
  `quickshell/MockDesktop.qml`, `shell/ipc/matugen.go`).
- `ryowalls/`: **motionbgs page 2 and beyond no longer error out.** The pager
  built the page path without a trailing slash (`/2`), which motionbgs 404s, so
  the whole browse fell over with a curl 22; the real scheme is a trailing-slash
  path (`/2/`, `/3/`, …) -- `?page=` is silently ignored -- so the pager uses it
  now and every page loads distinct results (`bin/ryowalls`).
- `ryowalls/`: **a live wall's Adjust and Colour lanes read cleanly.** A video
  pick has only a few controls (Fit, sampling Frame, Enhance), and the Fit and
  Frame cells were declared at a narrow span: the reserved control slot collapsed
  the cell's text column to nothing and left the Fill/Fit segments stacked in an
  empty box, and it clipped the Frame caption to "The sec…". Both are full-width
  inline cells now, with the shared segment reservation padded a gutter so Fill|Fit
  stays one row (`quickshell/GradeSheet.qml`, `quickshell/PaletteSheet.qml`).

### Changed
- `ryowalls/`: **The store is reworked around the image and its rice preview.**
  The live preview -- your wallpaper wearing the terminal and the cava spectrum
  in its own colours -- is now the hero: it fills the right column down to a slim
  candidate-palette + metadata footer, no longer squeezed above a redundant
  pending-diff card whose state the commit bar already carries. Editing is
  demoted from a co-equal BROWSE / GRADE / PALETTE tab set to a secondary
  "ADJUST" entry that appears on a pick, with a `‹ BROWSE` back and an
  ADJUST / COLOUR switch; browsing is home. Grid thumbnails lean in on hover
  (`quickshell/App.qml`, `quickshell/PreviewStack.qml`, `quickshell/WallCell.qml`;
  `quickshell/PendingCard.qml` retired).
- `ryostore/`: **The store lands on the section it was opened to instead of
  snapping to Discover.** A nav-open (from `ryostore open`, behind every Hub
  "Browse RyoStore" button and "Open in Settings") arrived before the catalogue
  loaded, so the route was dropped; it is now stashed and applied once the
  categories arrive (`quickshell/App.qml`).
- `ryostore/`: **Animated gif previews keep their aspect instead of stretching
  to the plate.** The preview's AnimatedImage forced a sourceSize, which QMovie
  applies as an exact scale that distorts a gif whose aspect differs; dropping it
  lets fillMode fit the frames (`quickshell/ProductMedia.qml`).
- **Softer corners across the desktop.** The shared Ryoku.Ui radius went from
  2 px to 6 px, so buttons, cards, and tiles read less boxy
  (`ui/Singletons/Tokens.qml`).
- `ryostore/`: **"Open in Settings" only shows for products with a real settings
  page.** It appeared on every installed product, but decors, launcher images,
  and lockscreens have no manage or apply page (the lockscreen and app-launcher
  pages are edit-only), so the button did nothing there. The catalogue now marks
  each item's `hasSettings` and the detail hides the action when there is nowhere
  to go (`backend/routing.go`, `backend/{catalog,model}.go`, `quickshell/lib/store.js`).
- `ryostore/`: **Launcher images, a Store category of curated hero art for the
  app launcher's header.** Six wide public-domain works (Hokusai's Great Wave,
  Hiroshige's Shono, Friedrich's Sea of Ice, Van Gogh's Wheatfield with Crows,
  Aivazovsky's Ninth Wave, the Hubble Carina Nebula) ship with raw and
  `ryodither`-baked variants; the detail's DITHER toggle previews and picks
  which installs. An install lands one flat file in `~/Pictures/ryoku-launchers`,
  and the launcher settings hero picker gains a STORE shortcut that browses it,
  so a store image becomes the launcher hero. Decors and launcher images now
  share one `flatImageProvider` (`ryostore/backend/provider_flat_image.go`,
  `backend/{catalog,routing,product_manifest}.go`, `hub/quickshell/pages/LauncherPage.qml`).
- `ryostore/`: **Decors, a new Store category of curated public-domain art for
  the Hub's decor slots.** Seven specimens (Piranesi's Carceri, Dürer's
  Melencolia I and Rhinoceros, Hokusai's Red Fuji, the Hubble Pillars of
  Creation, a Met bronze, a Muybridge motion plate) ship in the external
  catalogue, each with a raw and a `ryodither`-baked 1-bit bone variant. The
  detail's **DITHER** toggle previews both looks and chooses which one installs;
  a decor lands as one flat file in `~/Pictures/ryodecors`, so the `Decor` and
  `Placard` gallery lists it beside the shipped set with no further wiring
  (`ryostore/backend/provider_flat_image.go`, `backend/{catalog,main,routing,product_manifest,model}.go`,
  `ryostore/quickshell/{ProductDetail,ProductCover,App}.qml`,
  `quickshell/Singletons/Store.qml`, `ui/Decor.qml`).
- `ryostore/`: **Nacre and Obi are installable Store products instead of
  bundled shell payloads.** Their complete QML scenes now live in the external
  catalogue with generated previews and strict manifests. RyoStore owns their
  install receipts, publishes a derived installed-style index, and the shell
  reloads a changed product in place from its versioned URL; removing the active
  product falls back to built-in Sumi without restarting the shell.
- `wireplumber/`: **Bluetooth playback stays in A2DP unless the user selects
  headset mode.** WirePlumber's default microphone autoswitch silently moved
  earbuds into low-bandwidth HFP/HSP whenever an app opened their mic, degrading
  every playing stream. The Ryoku fragment now disables that automatic switch;
  its existing Hi-Fi / Headset control still provides explicit microphone mode.
- `nvim/`: **the editor follows the live wallpaper palette.** `ryoku.lua` pinned
  `tokyonight-night` flat, so the editor ignored the theme while kitty and the
  shell tracked it. It now reads the daemon's `~/.cache/ryoku/colors.json` (the
  same base16 set kitty reads) into tokyonight's `on_colors`, so nvim's
  background and syntax match the terminal; habamax stays the fallback, and a
  `FocusGained` hook re-tints a running editor when the palette changes
  (`nvim/lua/plugins/ryoku.lua`).
- `ryowalls/`: **the palette preview runs through matugen, and the per-image
  scheme tune is gone.** The `palette` verb dropped its wallust invocation and
  now derives the 16-slot preview strip from `matugen image --json hex
  --dry-run`, mapped onto the daemon's base16 order exactly, so the preview
  matches what Set writes. The PALETTE lane's tone / character / colorspace /
  backend / saturation / threshold / contrast rows are removed (colours follow
  Appearance > Wallpaper globally now); the 16-swatch strip and the
  live-wallpaper frame control stay, and the pending diff no longer tracks a
  per-image palette name. The tune state file is `ryoku-ryowalls.json`
  (`bin/ryowalls`, `quickshell/Singletons/Wallhaven.qml`, `quickshell/PaletteSheet.qml`,
  `quickshell/App.qml`, `quickshell/PendingCard.qml`, `quickshell/PreviewStack.qml`).
- `ryovm/`: **a new connection defaults its login to `root`, not you.** A blank
  user on a remote host silently resolves to your local username, which almost
  never matches a VPS and reads as a dead box on first probe. The add form now
  seeds `root` (the common VPS default, still editable); editing a host keeps its
  own user (`AddRemote.qml`).
- `ryovm/`: **a running machine reads red, and the stage claims the freed corner.**
  The RUN drum on a live machine now flips in the sun accent (the yard's one
  earned colour, spent on state), the fleet filter is trimmed to its content, and
  the machine stage rises into the space the departure board left behind, so the
  detail plate starts at the toolbar line instead of below the list
  (`VmCard.qml`, `FleetTile.qml`, `Machines.qml`).
- `ryovm/`: **the page heads are quieter and the machine stage sits higher.** The
  Fraunces page title dropped from display to headline size, and the Machines
  head lost its split-flap `NN MACHINES / NN RUNNING` board, which competed with
  the title and only echoed the fleet vitals already on the Dashboard. The
  reclaimed height lifts the detail stage so a stopped machine's RESOURCES no
  longer clip below the fold (`PageHead.qml`, `Machines.qml`).
- **`ryowalls` and `ryovm` get distinct app icons.** New flat marks on Ryoku's
  dark tile in the brand orange (a framed mountain-and-sun for the wallpaper
  gallery, cascading screens for the virtual machines), replacing the old
  `logo.svg` and matching the Ryo Motion aperture as one cohesive set. They ship
  via `ryoku-desktop` as the scalable hicolor app icon, so `ryoku update`
  delivers them (`ryowalls/quickshell/logo.svg`, `ryovm/quickshell/logo.svg`).
- `ryovm/`: **the dashboard's LAUNCH button is sized for a tile.** The fleet
  tiles used the full control padding, so the primary verb dominated each card.
  A `compact` variant on the shared `Btn` (tighter padding, smaller label) makes
  LAUNCH read as a tile action, not the loudest thing on the plate
  (`ui/Btn.qml`, `FleetTile.qml`).

### Fixed
- `ryowalls/`: **adding a wallpaper library now lives in Settings, and its fields
  actually take typing.** The source drawer and the settings panel never claimed
  keyboard focus (they lacked `focus: open`, so the app root held it and its key
  map ate every keystroke), and their scrim and card caught clicks with a
  `TapHandler` that let a field click fall through and dismiss the overlay -- so
  the drawer's `owner/repo@branch` field could not be typed into at all. Both
  overlays now take focus while open and use `MouseArea`s for click-safety (the
  ryovm connection-sheet pattern), the drawer focuses its filter on open, and the
  app regains focus when an overlay closes. Adding and removing libraries moved
  out of the drawer into ryowalls Settings (the gear -> Wallpaper libraries),
  where the field reliably takes input; the drawer is now a pure source picker
  (`SourcePicker.qml`, `SettingsPanel.qml`, `App.qml`).
- `ryowalls/`: **pressing Enter in the search box no longer sets the wallpaper.**
  The browse-lane key map applied the current pick on Return, and a single-line
  `TextInput` does not consume Return, so hitting Enter to run a search also
  silently set the highlighted wallpaper. Enter now only searches; SET WALLPAPER
  is the one way to apply (`App.qml`).
- `ryowalls/`: **a user-added library grid loads in seconds, not half a minute.**
  A GitHub folder-tree library used each wallpaper's full-resolution file as its
  grid thumbnail, so one page pulled 30-50 MB of originals before anything drew.
  The grid now requests a 480px WebP thumbnail through the wsrv.nl image proxy
  (~15 KB each), while the preview, palette and Set still use the full file, kept
  as a new `large` field (`bin/ryowalls`).
- `ryowalls/`: **Enhance runs on the discrete GPU first.** The last-good-GPU hint
  had settled on the integrated Radeon, which shares memory with the compositor,
  so enhancing dragged the whole desktop. The engine now detects the discrete GPU
  from waifu2x's own device list (cached) and tries it first, and the enhance
  process runs at idle IO and the lowest CPU priority so the extract/encode do not
  fight the desktop for cores (`bin/ryowalls`).
- `ryowalls/`: **a closed or interrupted enhance no longer keeps churning
  invisibly.** The video enhance ran waifu2x and ffmpeg as untracked children, so
  closing the window (Quickshell sends SIGTERM) orphaned them onto the GPU with
  nothing on screen and leaked a multi-GB frame dump in `/tmp`. The heavy steps
  now run backgrounded and a trap reaps the worker and the workdir on
  TERM / INT / HUP / exit (`bin/ryowalls`).
- `ryowalls/`: **Enhance shows progress while it extracts a clip's frames.** The
  bar only moved during the upscale pass, so a long clip sat on a blinking dot
  through the whole extract. Extract now reports frames-done against an estimated
  total, so the bar climbs from the first phase (`bin/ryowalls`).
- `ryowalls/`: **MoeWalls warns that its previews are low-resolution.** MoeWalls
  only serves ~720p preview loops (soft on a large screen) and the exact size
  varies per clip, so the browse view now carries one honest note -- a bone plate,
  black ink, no red -- pointing at Enhance, instead of a misleading per-tile
  number (`WallGrid.qml`).
- `ryovm/`: **a saved password now works for health probes and connect, not just
  in theory.** A keyless host with a saved password read as a dead box: the probe
  ran under `BatchMode`, which blocks password auth, and the CONNECT button went
  through the ssh kitten, which ignores `SSH_ASKPASS` and silently prompted. The
  probe now drops `BatchMode` for a saved-password host and answers from the
  keyring via askpass (one prompt, key first); connect routes a saved-password
  host through plain ssh so askpass fills it, keeping the kitten for keyed hosts.
  Verified live: `probe` returns a full reading and CONNECT lands on a root shell,
  no prompt (`remote/ryossh.go`).
- `ryovm/`: **the connection sheet fits any window and only closes on purpose.**
  The add/edit form outgrew short windows: its head and the SAVE row fell off
  the top and bottom with no way to scroll, and a click on any field inside
  dismissed the whole thing. The fields now scroll between a pinned head and a
  pinned CANCEL/SAVE row, the card is capped to the window height, and only an
  outside click or Esc closes it. The settings panel gets the same click-safety
  (`AddRemote.qml`, `SettingsPanel.qml`).
- `ryovm/`: **the create sheet no longer overprints the channel switch.** The
  machine stage's rise into the freed corner also lifted the NEW lane's create
  sheet, so a running build's download row landed on top of the
  CATALOG/INSTANT/ISO switch and its refresh. The rise now applies only in the
  LIBRARY lane, where that corner is empty (`Machines.qml`).
- `ryovm/`: **a catalogue build that can't fetch its media fails out loud, not
  into a dead machine.** Microsoft IP-gates the Windows ISO, so `quickget`
  finished with a config but no ISO; worse, its log stream ran through a pipe
  that, under `set -e`, aborted the whole create before any result was reported,
  leaving a machine that only errored at launch with a raw qemu line. The
  streamed log can no longer sink the create, and a build whose ISO never landed
  now clears itself and says why: for Windows, that Microsoft blocks the download
  and to build from a browser-fetched ISO via Load ISO (drivers and TPM already
  wired). Launching a machine with missing media says so plainly too (`bin/ryovm`).
- `ryovm/`: **HEADLESS and SPICE launches honour the mode you pick.** quickemu
  sources a machine's `.conf` after its own `--display` flag, so the conf's saved
  display always won and a machine set to a GTK window would open one even when
  launched headless. The engine now writes the chosen mode into the conf before
  handing off, so `launch <vm> headless` truly runs windowless (`bin/ryovm`).
- `ryovm/`: **cores and memory can go back to AUTO.** Pinning a VM's CPU or RAM
  to a number left no way back to quickemu's auto-tuning short of editing the
  conf; the steppers only counted up and down. Each now grows an `AUTO` button
  once pinned, and the engine's `config` verb clears the key (rather than writing
  a non-numeric value quickemu would choke on) so the guest auto-tunes again
  (`VmDetail.qml`, `bin/ryovm`).
- `ryowalls/`: **the source drawer is legible.** The provider catalogue rendered
  as `paperLift` on a 55% scrim with only a hairline border and no elevation, so
  it dissolved into the grid behind it. It now sits on a deeper scrim with the
  shadow the design system reserves for things that genuinely float. Its opener,
  a faint `N ▾` glyph lost beside the title, is now a bordered `SOURCE ▾` chip
  that reads as a real control (`SourcePicker.qml`, `App.qml`).
- **`Field` never took focus.** The shared field's `focus()` helper was shadowed
  by `Item`'s built-in `focus` property, so `Field.focus()` threw a `TypeError`
  everywhere it was called (the `ryowalls`/`ryovm` search shortcuts, the hub's
  Ctrl+K, the source filter) and no field ever focused. Renamed to `grabFocus()`
  and fixed every call site (`ui/Field.qml`, `apps/{ryowalls,ryovm}`,
  `hub/quickshell/Hub.qml`).
- `ryovm/`: **instant machines now hand over a shell only once the tools are
  actually there.** The connect flow waited for `sshd` to answer, but a cloud
  image installs its toolset in cloud-init's *final* stage, which runs 20s
  (Arch `pacman`) to ~2min (Fedora `dnf`) after SSH is already up. So you'd
  land in a shell with no `git`/`go` and think the toolset never deployed;
  Alpine's ~2s `apk` hid the gap, which is why only it looked fine. The connect
  now waits for `cloud-init status` to reach `done` before the shell, with a
  live timer and Ctrl+C to drop in early. The tools were installing correctly
  all along, the shell was just handed over too soon (`Singletons/Vm.qml`).
- `ryovm/`: **`<tab>` completion works out of the box.** Cloud images ship bash
  but no `bash-completion`, so programmable completion (git subcommands, service
  and package names) was dead in instant machines. Every provisioned machine now
  bakes `bash-completion` (and `bash` itself on Alpine, whose login shell is
  ash); a template spawn inherits it from its base and keeps its refresh seed
  empty so it still boots in seconds (`bin/ryovm`).
- `ryovm/`: **the machine detail's hardware knobs show their labels.** In the
  narrow detail column the CPU-cores, memory, and disk-size cells sized to a
  third of the pane, which left the text column at zero width once the stepper
  took its share, so each showed only a stray word (`How`, `RAM`) with no label
  or value. The paired cells now take half the column and the disk cell its full
  row, so `CPU CORES / AUTO`, `MEMORY / AUTO GB`, and `DISK SIZE` read as
  intended (`VmDetail.qml`).
- `ryovm/`: **a berth's health readout is no longer cramped.** The Remotes list
  held two-thirds of the page, so the detail column fell to a third and elided
  even a short host name (`localh…`) while the middle sat empty. The split now
  matches Machines, giving the host name, the probe grid, and the tunnels room,
  and the list tiles stop clipping their address (`RemotesPage.qml`).

### Added
- `ryovm/`: **a remote's password, saved in the login keyring, never a file.**
  The connection sheet gains a PASSWORD field; the secret goes to the Secret
  Service (`secret-tool`), never the sidecar, the ssh_config, or a command line.
  On connect ssh pulls it through an askpass helper that reads the keyring on
  demand, so a password host opens without a prompt, and COPY KEY can use it to
  deploy a key in one step (`remote/ryossh.go`, `AddRemote.qml`,
  `Singletons/Remotes.qml`, `RemoteDetail.qml`, `ui/Field.qml`).
- `ryovm/`: **kitty connections run through the ssh kitten.** A launched session
  gets real terminfo, shell integration, and OSC-52 clipboard, so copy and paste
  reach the remote instead of a bare, key-mangled `ssh` (`remote/ryossh.go`).
- `ryovm/`: **Windows installs cleanly from an ISO, drivers and all.** Windows
  ships no in-box VirtIO driver, so its installer saw no disk, and the driver CD
  quickget pulls per-VM now comes back as an anti-bot HTML stub from
  fedorapeople, so even that was broken. Building a machine from a Windows ISO
  now enables a TPM (so 11 passes setup), attaches one shared, validated
  virtio-win CD fetched from a mirror that serves the bytes (cached in
  `.images/`, reused by every Windows machine), and leaves Secure Boot off since
  Arch's edk2 has no MS-key firmware to verify the loader. A `virtio` verb keeps
  the driver CD current and repairs a machine whose stub download failed; the
  ISO sheet spells out what's handled and points at Microsoft for the media
  (`bin/ryovm`, `Singletons/Vm.qml`, `IsoPanel.qml`).
- `ryovm/`: **build several machines at once.** The create flow was single-file:
  one download locked out the rest until it finished. Downloads are now a jobs
  model, each build its own streamed process, shown as a compact stack (name,
  live percent and rate, its own cancel) that coexists with the picker, so you
  queue the next OS while the first pulls (up to four; the engine's per-name
  staging keeps them from colliding). The footer counts the fleet in flight and
  offers CANCEL ALL (`Singletons/Vm.qml`, `DownloadStack.qml`, `CreatePanel.qml`,
  `CloudPanel.qml`, `Machines.qml`).
- `ryovm/`: **the VM manager becomes Ryoport, a hub for machines you command.**
  The single-lane quickemu app is reworked into a three-plate hub behind a
  Hub-style nav rail (harbour masthead, kanji seals, foot barcode): a
  **Dashboard** fleet overview (a dossier plate in the Profile register: a
  bone-engraved lighthouse hero (a new `lighthouse` ryodecor, generated with fal
  and baked bone-on-black through `ryoduo`), monumental Fraunces, live fleet vitals, and at-a-glance
  machine and remote tiles you act on in one tap), the **Machines** yard (the
  whole prior quickemu manager, moved intact), and a new **Remotes** fleet. The
  app is rebranded Ryoport (港, the harbour: a network *port* too) in the
  masthead and `.desktop`; the `qs -c ryovm` config id and the `ryovm` VM engine
  keep their names, so nothing in packaging, the keybind, or user data has to
  migrate. Ctrl+1/2/3 switch plates, Ctrl+N opens a new connection
  (`App.qml`, `Rail.qml`, `PageHead.qml`, `Dashboard.qml`, `Machines.qml`,
  `FleetTile.qml`, `shell.qml`, `ryovm.desktop`).
- `ryovm/`: **Remotes: an SSH/VPS console with live health, the PuTTY a hub
  should have.** A new `ryossh` Go engine reads `~/.ssh/config` (and a ryoport
  include it owns, so hosts also work from a bare `ssh <alias>`, no lock-in),
  reports per-host reachability + latency, runs a one-shot agentless health
  probe (uptime, load, memory, disk, failed units, watched services) over a
  reused ControlMaster and fed to a remote `sh -s` on stdin, so a fish/csh login
  shell can't mangle the POSIX script, opens an interactive terminal in a tap,
  and carries the
  key toolkit (`ssh-add`/`keygen`/`copy-id`). The page lists hosts as ink-metered
  tiles (state by word and a dot, never colour), a berth reads the full probe and
  browses the host's files over SFTP in the file manager, and an add/edit sheet
  writes the host back (fields validated so a stray space or newline can't corrupt
  or inject into ssh_config), and a ProxyJump field reaches hosts behind a
  bastion; a per-host watch list adds `systemctl is-active` checks to the probe,
  and the berth reads each watched service's state. Timed probes pause when the page is
  off screen (`remote/ryossh.go`, `Singletons/Remotes.qml`, `RemotesPage.qml`,
  `RemoteTile.qml`, `RemoteDetail.qml`, `AddRemote.qml`).
- `ryovm/`: **per-host app shortcuts with a live HTTP monitor.** A berth carries
  a list of web services (Grafana, a Proxmox UI, anything with a URL); each is a
  tile that opens in the browser and, on a timed `ryossh appcheck`, shows a live
  up/warn/down dot and its round-trip in milliseconds. Glance's monitor and
  bookmarks fused and scoped to one host: a GET (self-signed TLS tolerated, body
  never read) reads up on a sub-400 answer, warn on a 4xx/5xx, down when nothing
  answers. Entered in the connection sheet as `name=url` pairs
  (`remote/ryossh.go` appcheck verbs, `Singletons/Remotes.qml`,
  `RemoteDetail.qml`, `AddRemote.qml`).
- `ryovm/`: **Proxmox clusters, controlled from the hub.** Give a host a Proxmox
  API URL and token and its berth grows a GUESTS section: every VM and container
  across the cluster from one `/cluster/resources` call (the token in the header,
  self-signed certs accepted), each with a live state, memory, owning node, and a
  one-tap start/stop that Proxmox routes to the right node. A starting or
  stopping guest holds a pending state until the cluster confirms the new status,
  so a slow graceful shutdown still reads as working, not as a dead button
  (`remote/ryossh.go` pve verbs + `ryossh_test.go`, `Singletons/Remotes.qml`,
  `RemoteDetail.qml`, `AddRemote.qml`).
- `ryovm/`: **live control of a running machine.** A new `ryovm-mon` helper talks
  to the HMP monitor and guest-agent sockets quickemu already opens (no QEMU flag
  injection, no engine change), so the machine stage gains a POWER section:
  pause/resume, a two-tap hard reset, a live memory balloon, vCPU pinning to host cores, and a
  live readout of host CPU/RAM cost, the guest's real IP, and topology. Verified
  live against a running Arch guest (`mon/ryovm-mon.go`, `bin/ryovm` `mon` verb,
  `Singletons/Vm.qml`, `VmDetail.qml`). The machine's PORTS section forwards a
  host port to a guest port (quickemu's native `port_forwards`), reachable at
  `localhost` (`bin/ryovm` `portfwd` verb). Static disk `cache`/`aio` and virtiofs
  shared-folder tuning are noted as a follow-up.
- `ryovm/`: **harbour conveniences.** The Dashboard grows an `// ACTIVITY_` feed
  merging machine and remote events newest-first; Remotes is keyboard-first
  (arrows walk the tiles with a selected-plate highlight, Enter drops into a
  session, `/` jumps to the filter) like a real terminal client. A berth also
  raises **SSH tunnels**: local (-L), remote (-R) and dynamic SOCKS (-D)
  forwards run as tracked `ssh -N` processes (strict spec validation, self-healing
  state, no orphans) shown live with one-tap close, and **one-tap ops** that
  open htop, a live journal, disk usage or listening ports on the host in a held
  terminal. The Dashboard reads at a glance at any fleet size and stays live: each
  section caps to a preview (active machines and ailing remotes first) with a
  `+N more` plate to its full page, and the health probes run whenever the
  dashboard or the fleet page is on screen (`Dashboard.qml`, `MoreTile.qml`,
  `RemotesPage.qml`, `RemoteTile.qml`, `RemoteDetail.qml`, `remote/ryossh.go`,
  `Singletons/{Vm,Remotes}.qml`).
- `ryowalls/`: **the empty spaces wear the house decor.** The head's dead right
  band is now a masthead specimen (a live bone-dithered `wave` under 壁紙 / 画廊,
  a 壁を選ぶ tategaki, a barcode and 壁 seal, right-click to reframe); the empty
  preview shows a 壁紙 / プレビュー earth plate instead of a bare Torii; and the
  empty, loading, and error browse grid becomes a 無 statue `Placard`, and the
  GRADE lane's no-pick column a 調色 statue `Placard`, each with the state woven
  into its caption. The same `Decor`/`Placard` grammar the hub uses for its own
  dead slots (`ryowalls/quickshell/{App,PreviewStack,WallGrid,GradeSheet}.qml`).
- `ryovm/`: **SSH sessions no longer break on the terminal type.** Opening SSH
  from kitty (or foot, WezTerm, …) advertised a `TERM` a minimal guest has no
  terminfo for, `clear`, `less`, `vim` died with `'xterm-kitty': unknown
  terminal type`. The command now prefixes `env TERM=xterm-256color` (a real
  binary, so it survives the app's unquoted `$cmd` run, a bare `TERM=` prefix
  was parsed as a command name and failed with exit 127), advertising a
  terminal type every guest ships; the fix is in both the app-opened terminal
  and the copyable command. The wait-for-boot narration also stops crying wolf
  at 60s, a fresh cloud image legitimately takes about a minute to provision
  on first boot (Arch, Fedora), so it reassures instead of warning, and only
  calls it dead after three minutes. Two instant-catalogue fixes rode along:
  Alpine now uses its UEFI cloud image (quickemu boots OVMF; the BIOS variant
  never booted), and the Fedora resolver picks the plain Generic Base qcow2,
  not the UEFI-UKI secure-boot variant. Verified live: Debian, Ubuntu, Arch,
  and Alpine instant machines all ssh in as ryoku with a working terminal
  (`bin/ryovm`, `Singletons/Vm.qml`).
- `ryovm/`: **the instant-machine seed builds with any ISO tool.** genisoimage
  is AUR-only (cdrtools), so a fresh box could not build the cloud-init seed;
  it now uses whatever is present, xorriso (in the Arch repos via
  `libisoburn`), genisoimage, or mkisofs, and `ryovm setup` pulls xorriso
  alongside quickemu so installing the engine also enables instant machines
  (`bin/ryovm`, `ryoku-desktop` optdepend).
- `ryovm/`: **toolsets, clipboard, and golden templates for instant machines.**
  An instant machine can now boot with a dev toolset already installed, a
  "Tools" panel in the create sheet offers curated chips (git, build tools,
  python, node, go, rust, docker, podman, jq, curl/wget, cli utils, SPICE
  clipboard) plus a free-text field for any other packages, remembered between
  sessions. The seed maps each tool to the target distro's own package names
  (docker is `docker.io`/`moby-engine`/`docker` per distro, with the service
  enabled and `ryoku` added to the docker group) and installs them via
  cloud-init on first boot. **Clipboard**: a `clip` helper is baked into every
  machine, `some-command | clip` copies to the host clipboard over SSH via
  OSC 52 (kitty), host→guest is native terminal paste, and the SPICE-clipboard
  tool adds bidirectional sync in the Console. **Golden templates**: because a
  disposable re-runs cloud-init (and re-installs tools) every boot, a keeper
  machine can be frozen with `Save as template` (or `ryovm template`) into an
  immutable base, and `ryovm spawn` makes thin clones that boot in seconds with
  the tools already baked. Verified live: a Debian instant installed
  git/go/docker in ~40s, was templated, and a disposable spawn came up in 11s
  with docker and go present, no reinstall (`bin/ryovm`, `CloudPanel.qml`,
  `VmDetail.qml`, `Singletons/Vm.qml`).
- `ryovm/`: **instant machines, a prebuilt VM with a known login, no installer.**
  `ryovm instant <os>` is the Kali/Vagrant model: it fetches a distro's official
  pre-installed cloud qcow2 (Ubuntu, Debian, Fedora, Arch, Alpine, openSUSE,
  Rocky, Alma, the curated catalogue quickget refuses to carry), makes a thin
  copy-on-write overlay so every machine costs ~200 KB until written, and
  attaches a cloud-init `cidata` seed that bakes in the standard **Ryoku burn
  account**, `ryoku`/`ryoku`, the ryovm burn SSH key, passwordless sudo, on
  first boot. No 14-step wizard; ssh-able in under a minute, and every later
  instant of that distro is seconds (overlay + seed + boot, zero download). It
  composes with disposable: a `--disposable` instant discards all writes at
  power-off *including* `/var/lib/cloud`, so every boot is a factory-fresh
  re-provision from the same read-only seed, born configured, dies clean.
  Because a burn machine regenerates its SSH host key each boot, its ssh
  command skips host-key pinning (a throwaway has no identity to verify) while
  installed machines keep their per-VM `known_hosts`. Verified end to end on a
  real Debian 13 cloud image: instant → ssh as `ryoku` (key, no password) →
  passwordless root → disposable re-burn wipes writes and re-creates the
  account (`bin/ryovm`).
- `ryovm/`: **the dispatch board, a full rework of the VM manager.** The window
  reads as a rail-dispatch wall crossed with an instrument panel: split-flap
  cells spell the live state (the header board counts `NN MACHINES · NN
  RUNNING`, every card and the machine stage carry their own drums), each
  machine is a boarding-pass ticket (hanko seal over the real brand mark,
  punched perforation, Fraunces display name, mono manifest grid), subsystems
  report on an annunciator row (KVM/UEFI/TPM/DISK/NET/SSH/SPICE/SEALED/BURN,
  lit means engaged, dark means honestly off), and the destructive verbs live
  under caution-striped guard covers that arm on one click and fire on the
  second. Brand marks come from simple-icons tinted to the board's cream ink
  (one visual system across all ~50 that resolve, Fedora included) with the
  quickemu-icons colour badges as fallback and stamped-initial plates for the
  rest.
- `ryovm/`: **disposable machines.** Set a machine up, hit Seal (one reserved
  qcow2 snapshot + a conf stamp), and every launch with the DISPOSABLE switch
  runs on quickemu's `--status-quo`: all disk writes burn up at power-off and
  the machine boots identical next time, the flaps spell BURNING while it
  runs. A dirtied normal run rolls back under the RESTORE SEAL guard. Proven
  end to end on an installed guest (created files evaporate from disposable
  sessions, survive normal ones, and the seal restore reverts everything).
- `ryovm/`: **USB passthrough per machine.** The detail pane lists the host's
  USB devices with hardware slide-switches; engaged devices write quickemu's
  `usb_devices` array and are handed to the guest at the next boot (engine
  verbs `usb list|set`).
- `ryovm/`: **the library works without the engine.** A missing quickemu is a
  blinking ENGINE OFFLINE banner (with the install action) instead of a locked
  app, importing, configuring and deleting machines never needed it. The
  engine's readiness re-polls every 5s, so the board lights up the moment an
  install finishes. Launch failures, dead-end empty states and every error now
  land on a sticky FAULT row with the full engine output behind a DETAIL
  toggle; commands issued mid-operation queue instead of vanishing; create
  defaults skip dev channels (no more `daily-live` Ubuntu) and prefer vanilla
  editions; Esc dismisses instead of quitting (Ctrl+Q quits, with a handshake
  while a download runs); arrows/Enter drive the library and `/` jumps to
  search; SSH gets a copyable command line, `$TERMINAL` respect, and boot
  honesty end to end: QEMU forwards the guest's port the instant the machine
  starts, long before anything answers, so the board probes for the real
  `SSH-` banner and only lights the SSH lamp and endpoint once the guest can
  actually be reached (until then the field reads "no answer"); the
  click-to-connect window narrates the wait instead of sitting pitch dark on a
  booting guest, where it's going, which account it signs in as, elapsed
  seconds, a live-ISO hint after a quiet minute, then hands over to plain ssh
  the moment the banner lands, holding open on failure with the fix spelled
  out; and the login account rides the detail JSON (`sshUser`), shows in the
  Reach-it command line and is editable in place (`ryovm_ssh_user`), because a
  password prompt for an account that doesn't exist in the guest reads as a
  haunted machine.
- `ryowalls/`: a wallpaper **studio**, not just a browser. A new **Adjust** mode
  (a third tab beside Browse and Tune) shapes the picked wallpaper live in the
  rice preview. For an image: a colour **grade** (brightness, contrast,
  saturation, warmth, vignette) and one-tap **Look** presets (Vivid, Faded,
  Cinematic, Noir, Warm, Cool), baked into a sibling file on Set so the desktop
  matches the preview exactly and the extracted palette follows the edit. For a
  live clip: a **Fill / Fit** control that maps the clip onto the screen through
  `ryoku-livewall` (fill covers, fit letterboxes). Both offer an on-demand
  **Enhance** (AI upscale on the GPU) with a real progress bar and honest phases,
  replacing the buried "Enhance on save" toggle. Enhance leaves a source already
  sharp enough as-is and says so ("Already sharp") instead of faking a pass: an
  image past 4K, or a clip past livewall's decode width where the compositor
  would only downscale the extra detail away. A GPU is chosen by validating its
  real output, not a quick probe (a flaky hybrid dGPU can pass a probe then emit
  black); the H.264 result lands in a new .mp4 beside the source instead of
  overwriting it, and any black run is discarded, so a bad enhance never destroys
  or garbles the original. New `AdjustPanel.qml`,
  an `adjust` verb and a reworked on-demand `enhance` verb in the `ryowalls`
  engine, and a `liveFit` setting (`App.qml`, `Singletons/Wallhaven.qml`).
- `ryowalls/`: a **Local** source browses the wallpapers already on the machine,
  images from `~/Pictures/Wallpapers` and live clips from `~/Pictures/livewalls`,
  in one grid with the same All/Images/Live filter the library source uses.
  Setting one is instant (no re-download), and each tile carries a selection
  checkbox so saved wallpapers can be pruned one at a time or in bulk: Select all,
  then Delete behind a confirm. The engine gains `local-list` and a `local-remove`
  that only ever unlinks files under those two folders (`App.qml`, `WallCell.qml`,
  `WallGrid.qml`, `Singletons/Wallhaven.qml`, and the `ryowalls` engine).
- `fish/conf.d/rashin.fish` the terminal weave for Ryoku Rashin's `rashin`
  command: an interactive wrapper that drops a proposed command on the prompt,
  an **Alt+R** binding that transmutes the current command line into a
  command, a `fish_postexec` hook that reports proposed-vs-ran corrections to
  the daemon, and a loader for the generated `rr-<name>` recipe abbreviations.
  Inert when the `ryoku-rashin` daemon is off or absent. Deployed by
  `deploy.sh`, shipped by `ryoku-desktop`. See `docs/rashin-terminal.md`.
- `pipewire/` audio follows the device you just connected. A pipewire-pulse
  drop-in (`pipewire-pulse.conf.d/10-ryoku-switch-on-connect.conf`) loads the
  PulseAudio compat `module-switch-on-connect`, so a Bluetooth headset finishing
  its connect (or a plugged-in USB DAC) becomes the default sink and running
  streams migrate to it. Before, sound kept playing from the old device until
  the sink was re-picked by hand in the mixer. Deployed by `deploy.sh`, shipped
  by `ryoku-desktop`.
- `nautilus/` a Ryoku stash menu in the file-manager right-click: a
  `nautilus-python` extension (`ryoku-stash-menu.py`) that adds **Install with
  Ryoku** (installable files), **Compress with Ryoku** (media), and **Send with
  LocalSend** (a single file), handing the picked file to the control deck's own
  `stash-install.sh` / `stash-compress.sh` and the deck's LocalSend picker so it
  behaves exactly like a stash drop. Install passes `RYOKU_STASH_KEEP=1`, since a
  file you right-clicked is yours to keep, not a redundant stash copy.
- `kitty/` terminal config (`kitty.conf`) plus a default `current-theme.conf` in
  the Ryoku dark palette.
- `fastfetch/` branded readout (`config.jsonc`) and the `ryoku-fastfetch` launcher
  (kitty graphics with a chafa fallback).
- `fish/` shell config with the greeting suppressed and starship, zoxide, fzf,
  and eza wired up.
- `starship/` prompt (directory, git branch, command duration) on a fixed
  Ryoku palette.
- `nautilus/` notes on xdg-user-dirs home folders and optional GSettings defaults.
- `nvim/` LazyVim-based Neovim config with the custom Ryoku startup dashboard
  logo (snacks.nvim header), tokyonight default, plus `ryoku-nvim.desktop` that
  registers it for text files.
- `yazi/` file manager config; its editor opener is Neovim (blocking).
- `mimeapps.list` makes Neovim the default application for text and code files.
- `npm/` ships `~/.npmrc` (global prefix `~/.local`) and `pip/` ships
  `~/.config/pip/pip.conf` (`break-system-packages`), so `npm i -g` and
  `pip install --user` work without root.
- `ryovm/` a virtual-machine manager (`qs -c ryovm`, Super+Shift+V), built on
  quickemu/quickget. A **Library** of your machines and a **Catalog** of ~90
  operating systems (~770 release/edition combos: Windows, macOS, every major
  Linux, the BSDs, Android x86). Brand logos are prefetched in parallel and
  cached to `~/.cache/ryoku/ryovm-icons` (a negative cache skips the ~56 OSes
  with no upstream art, which fall back to a coloured monogram); systems that
  have a real logo sort into a **Popular** section above the rest. Builds a VM in
  app with a live progress bar and Cancel (a `ryovm-fetch` Go helper does the
  parallel download; cancelling wipes the half-image), or from any local ISO via
  **Load ISO**. Manage a machine fully: rename it, pin or leave-automatic its
  cores/memory, **grow** its disk, take and restore **snapshots**, **reclaim**
  the disk (frees the image, keeps the machine) or delete it; every card and the
  detail dossier show the machine's real **disk footprint**, so you can see what
  is eating space. Three display modes: a **Window**, a **SPICE** console, or
  **Headless** (terminal-only, SSH in); the running view shows the mode's
  cursor-release shortcut and the live SPICE/SSH endpoints. The interface wears
  Ryoku's Greek-noir brutalism (flat carbon surfaces, hairlines and hard offset
  shadows, the 力 eyebrow, a Fraunces masthead and registration-mark chrome)
  and tells the truth: automatic resources read **Automatic**, never a
  fabricated number. The `ryovm` engine is the data plane; the GPU-passthrough
  gaming VM in Ryoku Settings > GPU is a separate, single-VM path.

### Changed
- `ryowalls/`: the Live tab plays video wallpapers through **`ryoku-livewall`**
  now, a lightweight software-decode daemon that holds ~40 MB of RAM on any GPU
  vendor, in place of `mpvpaper`/`phonto`, so setting a live wallpaper stays well
  under 100 MB. Browse and select are unchanged; the clip is transcoded to a
  cached <=720p30 clip once, then played. ryowalls still calls `ryoku-shell
  wallpaper set`; the shell's `ipc/wallpaper.go` picks the backend.
- `ryowalls/`: the Adjust tab's live **Max FPS** slider is gone. The wallpaper
  daemon decodes video on the GPU now (phonto/VAAPI or mpv/NVDEC, by GPU) at the
  clip's own rate, so there is no fps cap to tune; the **Fit** control stays and
  maps to the backend's scale/panscan (fill/fit).
- `fastfetch/` new emblem (`assets/brand/fastfetch-emblem.png`), redrawn to say
  what Ryoku is in one mark: a torii gate (the arch, and unmistakably Japanese)
  framing a robed Greek marble philosopher inside a Greek-key meander ring (the
  Greek half, stated twice), a vermillion rising sun, and a 力 hanko seal. The
  old bust read as only-Greek with an ambiguous red circle. Bone line-art on a
  transparent background (no baked backdrop), so it floats on the terminal's
  paper instead of sitting in a box; the sun and seal carry the brand red.
  Generated via fal.ai (recraft vector), recoloured to the brand palette and
  seal-stamped locally.
- `ryowalls/`: the palette tune is now per-image and one-time. Setting a
  wallpaper writes the tune keyed to that image (`ryoku-wallust.json` gains an
  `image` field); the daemon applies it only while that image is the wallpaper,
  and a Super+W cycle takes over with default wallust. Tuning no longer writes a
  global mirror on every change, so a tune can never bleed onto a later
  wallpaper.
- `fish/`: put `~/.local/bin` on `PATH` for every shell (not only interactive),
  so user-installed tools and the `ryoku-fastfetch` wrapper resolve.
- `fastfetch/`: align the readout to the upstream Ryoku config (host/cpu/gpu
  layout, no `title`); the `力` brand logo uses a wider left pad to clear the edge.
- `fastfetch/`: color the keys and percentages with fixed brand truecolor
  instead of palette slots, so the readout stays legible under any wallust theme
  (themed `red`/`green` could fall to near-background contrast and vanish).
- `fish/`: ship a fixed, legible syntax-highlight color scheme set
  unconditionally in `config.fish`. fish applied a palette-tied default theme
  before `config.fish` that rendered typed input in a near-background color
  (invisible as you type); pinning command, param, error, comment, and
  autosuggestion colors keeps the command line readable under any wallust theme.
- `fish/`: hook `cd` into zoxide (`zoxide init fish --cmd cd`), so plain `cd`
  learns and jumps to frecent directories (`cdi` for an interactive pick).
- `yazi/`: show hidden files by default (`[mgr] show_hidden = true`), so dotfile
  trees like `~/.config` are visible.
- `fish/`: route `go install` (`GOBIN`) and `cargo install` (`CARGO_INSTALL_ROOT`)
  to `~/.local/bin` and activate `mise`, so every language tool installs onto
  `PATH` and works from day one.

### Fixed
- `ryovm/`: **the app knows whether SSH will answer before you click.** A
  forwarded port is no promise: slirp accepts the TCP connect even while the
  guest is still booting, or is a live ISO that will never run sshd, so
  clicking SSH hung a silent terminal for minutes with no sign of life. The
  engine now probes the actual SSH banner (1s, real signal) and `get` reports
  `sshReady`; the REACH IT panel says "Guest is answering, connect away" in
  green or exactly why not ("still booting, or no SSH server inside, live
  ISOs never have one"), the manifest marks the port "· no answer", the SSH
  annunciator lights only on real readiness, and the interactive attempt is
  bounded at 20s with the diagnosis held on screen instead of an endless
  blank hang (`bin/ryovm`, `VmStage.qml`, `VmDetail.qml`,
  `Singletons/Vm.qml`).
- `ryovm/`: **Stop is a power button, not a hatchet.** `stop` sent quickemu's
  --kill (SIGKILL) straight away: the guest's unflushed writes died with it,
  a machine provisioned seconds before a stop came back missing files, and
  the qcow2 took leaked-cluster damage that had to be repaired on the next
  launch. The engine now presses the ACPI power button over the per-VM QEMU
  monitor socket and gives the guest 20s to shut down clean (a cooperative
  guest takes ~3s); the kill remains as the fallback and as an explicit
  `stop <name> --force`. After a kill it also waits for the dying qemu to
  release the image before returning, and `seal` retries briefly through
  that same lock race instead of failing when asked a breath after a stop
  (`bin/ryovm`).
- `ryovm/`: **the SSH login user is per-machine.** The ssh verb guessed the
  host username for the guest, which is usually wrong; `ryovm_ssh_user` in
  the conf (set with `ryovm config <vm> ryovm_ssh_user <name>`) pins the
  account, with the host name only as the fallback guess (`bin/ryovm`).
- `ryovm/`: **SSH from the app survives more than one machine.** Every VM
  forwards its guest onto the same small host-port range, so the global
  `known_hosts` collided the moment a second machine answered on a port a
  first one once used, ssh failed with "REMOTE HOST IDENTIFICATION HAS
  CHANGED" against your own VM, and the old packaged app's terminal closed
  before the message could be read. The engine's `ssh` verb now pins each
  machine to its own known-hosts file beside its disk with
  `StrictHostKeyChecking=accept-new`: first connect just works, a genuinely
  changed key still fails loudly, and machines can never poison each other
  (`bin/ryovm`). The reworked app already holds the terminal open with the
  diagnosis when the guest refuses.
- `ryovm/`: **the launch and scroll flicker is gone.** Three compounding
  causes: the 5-second poll rebuilt the whole library from a fresh array even
  when nothing changed (every card torn down and re-created, replaying its
  entrance), the entrance animation itself ran from `Component.onCompleted`,
  which also fires for delegates the view creates while scrolling back, and
  the catalogue re-filtered (and so rebuilt all ~92 tiles) on every single
  logo resolution during launch. The engine payloads are now compared before
  they touch a model (identical poll = untouched model, stale detail stays on
  screen until the fresh one lands instead of blinking every det-gated
  section), the roll-call moved to the ListView's populate transition (first
  population only, by design), and the catalogue split no longer depends on
  the icon cache at all (`Singletons/Vm.qml`, `VmGrid.qml`, `OsGrid.qml`).
- `ryowalls/`: **a skipped enhance now explains itself instead of looking dead.**
  The engine's `enhance` prints a one-line JSON verdict on exit (`result`, `kind`,
  the pixels it measured and the cap they met, a `why` on failure), and the panel
  keeps an "Already sharp" or failure note on screen until the pick changes,
  spelling out the numbers ("already 5120px wide, the desktop plays live
  wallpapers at 2560px") instead of flashing a generic label for 3.5 seconds; the
  Enhance button stays through a skip, since the cap moves with the monitor and a
  retry must stay one click away. Failures name their cause (bad GPU output vs. a
  truncated file vs. a missing tool) instead of blaming the GPU for everything.
  The moewalls grid also stopped promising `1280x720` for every clip, the site
  serves previews from 720p to 1440p and dual-wide with no per-item resolution
  anywhere in its API, so a user could pick a tile labelled 720p, hit Enhance on
  the 1440p file it actually downloaded, and read the correct "Already sharp"
  skip as the feature being broken (motionbgs labels were checked against the
  downloaded masters and are honest; wallhaven's come from its API). Local grids
  (Live and Local sources) now badge each clip with its real `ffprobe`d
  resolution, probed 8 at a time so a big pool can't hold the grid hostage,
  and the amber low-res hint covers images too. Also fixed while pinning the
  verdict contract down: an animated webp/gif no longer misreads as past-4K
  (a bare `identify %h` concatenates every frame's height into nonsense like
  "12001200"; the probe now reads the first frame), an unreadable clip no longer
  errexits the verb with no verdict and a state file stuck at "probe" (the user
  saw a GPU blamed for a truncated download), the all-GPUs-failed path no longer
  dies on an unset `ok` under `set -u` before its error verdict prints, a verdict
  from a run that outlived its pick (a download-then-enhance, or minutes of
  frame-by-frame work) reports through the status toast instead of pinning the
  wrong numbers under a wallpaper it never touched, a failed video enhance no
  longer leaves a frozen progress bar under the failure note, and re-enhancing
  within the fade window no longer lets the stale clear-timer blank the status of
  the new run (`bin/ryowalls`, `Singletons/Wallhaven.qml`, `AdjustPanel.qml`,
  `WallCell.qml`; contract pinned by `tests/ryowalls-enhance-verdict.sh`).
- `ryowalls/`: **a live wallpaper set from the app no longer reverts on its own.**
  Enhancing a downloaded video ran a detached background job that, minutes later,
  re-issued `wallpaper set` for whatever file it had upscaled, so a clip enhanced
  earlier reclaimed the desktop over a wallpaper you had since chosen. Enhance is
  now an explicit, foreground action that only swaps its result onto the desktop
  when that file is still the live wallpaper (it reads `~/.local/state/ryoku-wallpaper`
  first), so a late finish can never yank an old wallpaper back. Downloads no
  longer auto-enhance, so a plain Set is never followed by a surprise swap.
- `ryowalls/`: "Enhance on save" leaned on the AUR `video2x` for video, which
  builds against system `ncnn` yet is never rebuilt when it changes, so it broke
  the moment Arch's `ncnn` dropped an API it used. Video now upscales frame by
  frame with `ffmpeg` + `waifu2x-ncnn-vulkan`, the same official-repo tool already
  used for images (Arch rebuilds it against `ncnn`), so one reliable tool sharpens
  both. The Install button also did nothing: it ran a bare `gpk <pkg>` (which only
  searches) and lingered until Settings was reopened. It now runs `gpk install` for
  that one tool and re-checks when the window regains focus (`App.qml`,
  `Window.active`), so Install flips to the live toggle on its own.
- `fastfetch/`: the readout fell back to the Arch logo on machines that updated
  (fresh installs were fine). `config.jsonc` now points its emblem at
  `~/.config/fastfetch/fastfetch-emblem.png`, laid beside the config by
  `ryoku materialize`, instead of `~/.local/share/ryoku/assets/brand/`, which is
  seeded only at install time. The emblem redraw above renamed the file, so
  updated machines referenced an emblem they never received and fastfetch
  silently used its built-in Arch logo. See the release changelog for the
  packaging side.
