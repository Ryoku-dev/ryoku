# Release / Packaging Changelog

## Unreleased

### Added
- **`ryoku-desktop` ships the `ryoku-gpu-trim` initramfs hook.**
  `/usr/lib/initcpio/install/ryoku-gpu-trim`, from
  `system/boot/mkinitcpio/install/`. The HOOKS drop-in names it and mkinitcpio
  aborts on a hook it cannot find, so the package has to own it on every box;
  it keeps the denylisted nouveau driver and its GSP firmware out of each
  kernel image, about 107 MiB of a 2 GiB boot partition per kernel.
- **The release ledger names the ISO of each variant.** `releases/index.json`
  entries gain an `images` map (`plain`, `cachyos`), each with the ISO,
  signature, checksum, per-ISO manifest and public URL, derived from the
  per-ISO manifests already at the bucket root (keyed by their `installer_ref`,
  which is the release tag on a release build, and `variant`). Both variants
  are dispatched and uploaded per release already, but only the mutable
  `latest.json` / `latest-cachyos.json` pointers named them, so an older
  release's image was in the bucket and discoverable by nobody: a CachyOS box
  could roll its packages back and not find the matching installer. The rebuild
  stays derived and idempotent, skips `latest*.json` and anything that is not a
  manifest (with a message, never an abort), and keeps every existing field, so
  a consumer that ignores `images` reads byte-identical entries
  (`bin/ryoku-release-ledger`).

- **`ryoku-keysounds`: the key sounds compositor plugin.** Built from
  `ryoku/hyprland/plugins/keysounds` (depends on `hyprland`, `libcanberra`),
  it installs `keysounds.so` under `/usr/lib/hyprland/plugins/`, eleven
  sample profiles cut at build time from the pinned MIT-licensed Mechvibes
  packs under `/usr/share/ryoku/keysounds/` (the licence beside them), and the
  plugin source under `/usr/share/ryoku/hypr-plugins/keysounds/` so a box with
  no checkout can rebuild it for a newer Hyprland. `ryoku-desktop` pins it like
  the other plugin packages.

### Removed
- **The `awww` package is gone from `[ryoku]`.** The wallpaper backend moved to
  Ryogami, which paints and animates its own transitions from the built-in
  engine, so nothing on the box drives `awww` any more: the shell talks to
  `ryogami.sock` and `ryoku doctor` retires a leftover `awww-daemon`. The recipe
  and its vendored 136K Rust snapshot still built a package on every repo pass
  and kept the daemon installable, so `release/packages/awww/` is deleted and the
  build toolchain drops `lz4` (awww's only pkg-config probe). `ryoku recovery`
  ensures `ryogami` instead of `awww` when it puts the wallpaper daemon back.

### Changed
- **Ryotunes updates on its own release channel, not the `[ryoku]` repo.**
- **Ryotunes also updates on its own GitHub release channel.**
  Ryotunes is released independently as a prebuilt Arch package on
  ryoku-dev/ryotunes' GitHub releases (`ryotunes-<ver>-1-x86_64.pkg.tar.zst` and a
  `.sha256` beside it). `ryoku update` now tracks those directly
  (`ryoku/cli/internal/ryotunesrelease`): it verifies the download by sha256 and
  by its own pacman name/version/arch, installs it with `pacman -U`, and only
  ever moves the version forward, so an external build is never downgraded.
  `ryoku doctor` reports a pending release without installing it, and Ryotunes is
  dropped from the explicit `[ryoku]` update set so the repo's base build cannot
  overwrite a newer one. The `[ryoku]` repo still builds the `ryotunes` package
  (the retained sha256-pinned source tarball) for the initial install only. The
  old auto-bump path is retired with it: `.github/workflows/ryotunes-release.yml`
  and `bin/ryoku-release-ryotunes` are gone, and the Ryotunes release dispatch
  into this repo with them.
- **`ryoku-shell` ships `ryostage`, not the two old engines.** Depth and Parallax
  merged into one engine: the package installs `/usr/bin/ryostage` and no longer
  ships `ryoku-depth` or `ryoku-parallax-engine`. `deploy.sh` removes the two old
  binaries from checkout boxes (pacman drops them from packaged boxes on upgrade).
- **The apps Ryoku ships are optdepends, not depends.** pacman re-satisfies a
  dependency list on every upgrade of the package that carries it, so a hard
  depend meant `ryoku update` reinstalled kitty, Nautilus, Ryotunes or the
  gaming stack for anyone who had deleted them. `ryoku-desktop` now names them
  as optdepends; the ISO pacstraps them and `ryoku doctor` delivers them once to
  an existing box, then honours a removal for good. Feature tools the shell
  calls by name (grim, matugen, cava, mpv, the OCR/capture backends) stay hard
  depends. Present apps are re-marked explicitly installed so leaving `depends`
  cannot turn them into orphan-sweep casualties.
- **Spotify and spicetify are gone.** `spotify-launcher`, `spicetify-cli` and
  `spicetify-marketplace` no longer ship, the two [ryoku] spicetify packages are
  retired, and the Ryoku Canvas extension and its loopback relay are removed
  with them. Ryotunes is the music app Ryoku ships. An already-installed Spotify
  is left alone.
  `ryoku doctor` reports a pending release without installing it. The `[ryoku]`
  repo still builds and ships the `ryotunes` package (the retained sha256-pinned
  source tarball). The old auto-bump path is retired with it:
  `.github/workflows/ryotunes-release.yml` and `bin/ryoku-release-ryotunes` are
  gone, and the Ryotunes release dispatch into this repo with them.
- **`ryotunes` 2.5.1-1 tracks ryoku-dev/ryotunes v2.5.1.** The heart saves without an account. Liking a track when there is no YouTube Music session (or on a SoundCloud/local track) lands it in a device-local Liked...
- **`ryotunes` 2.5.0-1 tracks ryoku-dev/ryotunes v2.5.0.** The package follows
  Ryotunes' GitHub releases (a sha256-pinned source tarball) instead of a
  hand-pinned commit, and enables `ryotunesd.socket` for every user, so
  `ryotunes` opens the native client on a fresh install instead of the old Tauri
  app.
- **Every Hyprland plugin package lays an `.abi` receipt beside its `.so`.**
  `hypr-dynamic-cursors`, `ryoku-hypr-plugins`, `hyprglass`, `imgborders` and
  `ryoku-keysounds` write `<name>.abi` from the build host's `version.h`, the
  plugin ABI string Hyprland checks on load (commit plus the major.minor of
  aquamarine, hyprutils, hyprgraphics, hyprcursor, hyprlang). The Hub's Plugins
  page, the generated `settings.lua` and `ryoku doctor` read it to tell a copy
  an Arch bump left behind from a working one without loading it, and rebuild
  it locally until the next publish ships a fresh package.
- **`ryotunes` 2.4.1-7 tracks ryoku-dev/ryotunes `43d063f`.** SoundCloud as a
  third provider (guest, waveform seek bar, Orange-style artist pages), the
  Discover home, the skin system (ten shipped skins under
  `/usr/share/ryotunes/skins`, the matugen template under
  `/usr/share/ryotunes/matugen`, RyoStore's `ryotunes-skins` category as the
  store source), the static cover wash (the client dropped from ~35 % of a core
  to ~2 % while playing), pause-to-quit (client after a minute parked, daemon a
  minute later), and the Spotify Premium gate that says why a sign-in failed.
- **`ryotunes` 2.4.1-4 ships the native client.** The package now tracks
  ryoku-dev/ryotunes `73e4e96` and carries `ryotunesd` (socket-activated daemon
  owning playback, MPRIS and the tray), `ryotunes-cli`, and the pure-QML
  Quickshell client `ryotunes-qml` at `/usr/share/ryotunes/client`, next to the
  unchanged Tauri app. Measured on a 7940HS laptop the native client idles at
  0.1% CPU paused and 0.7% playing where the WebKit app spent 2% plus three
  helper processes, and scrolls Home at under 1% of a core instead of 76%.
  Only one player runs at a time: `ryotunes` hands off to a live daemon
  (raise its client, or open `ryotunes-qml`) and is the standalone Tauri app
  only when no daemon runs, so Super+J, the dock and the launcher keep
  running plain `ryotunes`, and reopening after Super+Q remaps the client window (2.4.1-4). Hyprland floats its window (`float-ryotunes-qml`, the
  Quickshell class with title `Ryotunes`) like the Tauri one.
- **A release dispatches both ISOs.** The publish ran only `build-iso.yml`
  (plain Arch) after a release; `build-iso-cachyos.yml` is dispatched from the
  same frozen release directory now, so the CachyOS ISO carries the release
  too (`publish-repo.yml`).
- **The release ledger is derived, not edited.** `bin/ryoku-release-ledger`
  rebuilds `releases/index.json` from every `releases/<tag>/x86_64/release.json`
  the bucket holds (newest first, `latest` = newest); the publish runs it
  after a release lands and asserts the new tag is `latest`, and the Release
  Ledger workflow rebuilds it on demand. The in-place jq edit it replaces read
  a missing ledger as empty input (rclone cat of a missing key succeeds with
  nothing) and published a zero-byte file on the first release. The publish
  job also installs `github-cli` again, which the release ISO dispatch needs.
- **`build-repo.sh` adopts before it builds.** A package whose output names
  (`makepkg --packagelist`) the mirror already serves is not built: its bytes
  are copied in and re-signed, the immutability rule applied up front instead
  of after a wasted compile. A release is now a promotion of the testing build
  (same commit, same names, same bytes; only ryoku-desktop, which carries the
  channel marker, is rebuilt) and a testing push no longer recompiles a pinned
  external (ryotunes, ryomotion, prowl-agent). The first tag publish spent 40
  minutes hung inside ryotunes' upstream `cargo test` (an mpv integration
  test) and hit the job timeout; that `check()` is gone from the PKGBUILD too:
  the package build proves the binary builds, upstream CI owns its tests.
- **The publish builds once and ships the bytes it tested.** `publish-repo.yml`
  is now build -> gate -> publish: the build job signs the repo with the
  release key and keeps it as a workflow artifact; the container gate installs
  ryoku-desktop from that artifact on Arch and CachyOS with `SigLevel=Required`
  against the release keyring (`container-install.sh RYOKU_PREBUILT_REPO=1`,
  which skips the build toolchain and asserts the real release and channel
  in `/etc/ryoku-release`); the publish job uploads the same artifact. The
  gate used to build its own throwaway-key repo per base and the publish built
  a third time (none of them the shipped bytes), and at 90 minutes the gate
  timed out and the publish was silently skipped, which is how users sat on
  an old stable for days. The rclone remote is written by one script,
  `bin/ryoku-r2-config`, shared with the ISO workflow.

### Fixed
- **gpk tracks its upstream again.** The pinned GlazePKG release binary sat at
  0.5.8 while upstream had moved to 0.6.10, so a box fully up to date against
  the [ryoku] repo still ran a package manager two minor generations old, and
  "up to date" read as a false claim to the person who owns the tool. The pin,
  both architecture checksums and pkgrel move to the current release
  (`release/packages/gpk/PKGBUILD`).

### Added
- **Release lines have names.** `CODENAME` holds the current line's name
  (Onogoro before 1.0; Amaterasu for 1.0) and `release/names.md` tells each
  name's story. `build-repo.sh` writes the name into `release.json`, the
  ryoku-desktop package into `/etc/ryoku-release` (`NAME=`), the publish into
  `releases/index.json`; the Stable Release summary and the Release Notes
  workflow title the release "Ryoku <name> <version>", and a line's first
  release opens its notes with the story (`bin/ryoku-release-notes --intro`).
  The ISO manifests (`<iso>.json`, `latest.json`) carry `name` and `version`
  too, so the site can title the download (`bin/ryoku-iso-manifest`).
- **The `[ryoku]` repo publishes named releases and a testing channel.** Under
  the one bucket mount (`repo.ryoku.dev/stable/`), `x86_64/` is the stable
  pointer every installed box already has, `releases/<tag>/x86_64/` is one
  frozen copy per release tag (never rewritten; the publish refuses an existing
  directory), `releases/index.json` is the ledger, and
  `channels/testing/x86_64/` is rebuilt on every push to `unstable-dev`. A
  push to `main` publishes nothing: a release is the Stable Release workflow
  run on `main` (`bump_type: none` tags the version `main` carries), which
  publishes the frozen directory, moves the stable pointer onto it, records
  the ledger, and dispatches the release ISO from that directory (the ISO
  workflows take a `repo_url`, threaded into `offline-repo.sh`). A stable
  publish mirrors testing first, so a name testing already served is promoted
  rather than rebuilt. `build-repo.sh` writes `release.json` beside the db and
  `ryoku-desktop` ships `/etc/ryoku-release` naming the build. Existing boxes
  need no change: their `Server` line is the stable pointer, and the next
  update lands the client that understands channels.

- `ryotunes` joins the signed repository at 2.4.1: the Ryoku music app (Tauri +
  WebKitGTK + libmpv, built from `ryoku-dev/ryotunes` at a pinned commit, the
  Ryostore submission adopted as the official app). `ryoku-desktop` depends on
  it and no longer installs the Chromium app-window wrapper of the same name;
  the package takes over `/usr/bin/ryotunes`, the `.desktop` entry and the icon
  in the same `pacman -Syu`, so existing boxes switch on `ryoku update`. The
  desktop follows it over `org.mpris.MediaPlayer2.ryotunes` exactly as before
  (now-playing widget, dock pill, media keys), and the Hyprland float rule and
  `ryoku-music-toggle` match its `ryotunes` window class. The build toolchain
  list gains `webkit2gtk-4.1`, `mpv` and `libappindicator-gtk3`. The package
  `conflicts`/`replaces` every side-by-side build the author shipped before
  (`ryotunes-v1.4` through `ryotunes-v2.4`, AUR `ryotunes-bin`), so a box that
  installed one of those gets it removed in the same `pacman -Syu`, leaving one
  launcher entry, one `/usr/bin/ryotunes` and no replacement hook. The old
  Chromium profile at `~/.config/ryotunes` is left in place; delete it by hand
  to reclaim the space.

- `blesh` joins the signed repository at 0.4.0-devel3, and `ryoku-desktop` depends on
  it plus Zsh and the official Zsh editing plugins. The package now materializes
  the managed Bash/Zsh adapters and their shared terminal environment beside the
  existing Fish configuration, so `ryoku update` delivers shell parity to
  existing machines.

- `ryoku-desktop` hard-depends on `adw-gtk-theme` and installs the GTK 3 and
  GTK 4 `settings.ini` (`config/gtk-3.0`, `config/gtk-4.0`) beside the existing
  `qt6ct.conf`, so `ryoku materialize` lays them down and prunes them. The
  Hyprland session now selects `adw-gtk3-dark` instead of `Adwaita-dark`: stock
  Adwaita GTK3 hardcodes its colours, so the palette Ryoku generates barely
  reached GTK3 apps, while adw-gtk3 derives its widget rules from the named
  colours the shell already emits. Without the dependency that theme name has
  nothing on disk and GTK3 apps fall back to stock, so it is a hard depend that
  `ryoku update` carries onto existing boxes.
- `ryoku-desktop` hard-depends on `weston` (official `extra` repo). The SDDM
  greeter moves to Wayland (`DisplayServer=wayland`,
  `CompositorCommand=weston --shell=kiosk`) so it is torn down cleanly at login
  instead of being orphaned on a leftover Xorg when the Wayland session starts;
  weston is the kiosk compositor that hosts it. A hard depend (not optdepend) so
  `pacman -Syu` pulls it onto existing boxes before `ryoku doctor` writes the
  Wayland config -- that config cannot work without it.
- `game-devices-udev` 1.0 and `dualsensectl` 0.7 build into the signed `[ryoku]`
  repository. `game-devices-udev` moves out of `aur.packages` (it is in
  `base.packages`, so it must be reachable by pacman); `dualsensectl` is a
  control tool for a device most machines do not have, so it stays a one
  `pacman -S` away rather than joining the base set.
  `game-devices-udev` deviates from the AUR recipe twice, on purpose: it uses the
  plain release tarball rather than the `?signed` git source, because verifying
  upstream's tag signature needs the maintainer's key in the builder's keyring
  and this build host imports no upstream keys, so a `?signed` source would fail
  the build rather than fail safe; and it drops upstream's `uinput.conf`, because
  `ryoku-desktop` already ships `99-ryoku-uinput.conf` for that one-line job.
  Both source checksums were verified against the upstream artifact, and the
  Codeberg tarball was confirmed byte-stable across refetches before pinning it.
- `publish-repo.yml`: `meson` joins the build toolchain. `ninja` was already
  there but does not drive itself, so both new packages would have failed to
  build on the host while building fine locally.
- `xpadneo-dkms` 0.10.4 and `broadcom-bt-firmware` 12.0.1.1105 build into the
  signed `[ryoku]` repository. Both are in `base.packages`, and an AUR package
  cannot be reached by `ryoku update`, which is pacman: a machine that installed
  before the package set existed, or whose one-shot AUR build failed, had a
  non-working Xbox controller and no route to a fix but a manual `yay -S`. As
  signed repo packages they land on install, on update, and in the offline
  closure. Both are fixed-version recipes tracking upstream releases rather than
  `RYOKU_PKGVER`, and both source checksums were verified against the upstream
  artifact rather than copied from the AUR.
- `asusctl` 6.4.0 is built from its pinned upstream source into the signed
  `[ryoku]` repository, with the upstream daemon, udev rules, and user data. Its
  install hook reloads the rules and starts `asusd` only when upstream's hardware
  match fires. `ryoku-desktop` advertises it as the optional ASUS Aura lighting
  provider.

- `ryoku-desktop` ships `system/hardware/audio/70-ryoku-maono.rules` to
  `/usr/lib/udev/rules.d`, so the vendor HID node on Maono USB mics is reachable
  by the active-session user. See the system/hardware changelog.

### Fixed
- `ryoku-desktop` ships the default-app map to
  `/usr/share/applications/mimeapps.list` instead of the materialized config, so
  an update stops overwriting `~/.config/mimeapps.list`, the file every "Set as
  default" writes. Ryoku's defaults still apply on a fresh install; a user's pick
  now wins for good. `ryoku doctor` clears the copy an older release left behind.
- `ryoku-desktop` no longer ships the PipeWire `switch-on-connect` drop-in
  (`ryoku/apps/pipewire/`): it broke the saved default audio device on every
  reboot. WirePlumber persists the chosen sink on its own, and `ryoku materialize`
  prunes the stale drop-in on the next update. See the ryoku/apps changelog.

### Added
- **A look can lean into depth, not only spin in the plane.** `angle` turns a look
  clockwise on the screen; `tiltX` and `tiltY` pivot its box about its own horizontal
  or vertical axis so one edge goes away from the viewer and the other comes forward,
  with a real perspective divide rather than a squash. Both are bounded to 35 degrees,
  well short of edge-on, since a look flat to the viewer is a look you cannot see, and
  the viewer distance scales with the box so the same degrees read the same at any
  size. It costs one matrix on the item the spin already turns, so a turned and leaned
  look is still one draw with no offscreen buffer. The editing bar gains a LEAN group
  with the two axes and a LEVEL reset, and the Hub gains the matching rows. The leaned
  quad is fitted back onto its box: raw perspective pushed the near edge past the
  outline and left dead space at the far one, so the box stopped meaning what it said
  and the placement guides lied about it. Now the quad touches all four sides at every
  lean and crosses none, which the tests pin as an invariant
  (`ryoku/ui/lib/place.js`, `ryoku/ui/lib/place.test.mjs`, `ryoku/ui/SpectrumField.qml`,
  `ryoku/shell/quickshell/shell/modules/visualizer/EditBar.qml`,
  `ryoku/shell/quickshell/shell/modules/visualizer/Singletons/Config.qml`,
  `ryoku/hub/quickshell/schema/DesktopPage.js`).
- **The spectrum is tuned on the wallpaper, not in the Hub.** Placement showed a
  strip of monospace hints and nothing else, so changing a look meant leaving the
  desktop you were judging it against. It now comes with an editing bar
  (`ryoku/shell/quickshell/shell/modules/visualizer/EditBar.qml`) carrying the look
  itself as a silhouette chip that opens a tray of all eleven (or wheels through
  them), plus bands, mirror, peak caps, gain, smoothing, the live angle with a
  SQUARE reset, the size, FLIP and DONE; `F`, `M`, `P`, `R` and `[`/`]` do the same
  from the keyboard. It is assembled from `Ryoku.Ui`'s own controls (`Btn`, `Step`,
  `Sw`, `Slid`, `Gallery`) at token metrics, so it reads at the shell's size instead
  of the 11px HUD it replaced, and the tray is the Hub's gallery with the Hub's
  painter, so one catalogue draws what every look looks like. Each control gained a
  tracked eyebrow naming it and the gestures moved under a hairline inside the plate,
  since an instruction is not a control and outside the plate it was unreadable over
  a picture. A knob that does not apply to the look in hand dims instead of
  vanishing, because a bar that reflows while you walk the catalogue cannot be aimed
  at, and `Config` now owns which those are so the renderer and the bar cannot
  disagree. Edits settle through the same coalescer as a placement gesture rather
  than writing on the spot, since the config file is watched and a write returns as a
  reload (`ryoku/shell/quickshell/shell/modules/visualizer/Placer.qml`,
  `ryoku/shell/quickshell/shell/modules/visualizer/Singletons/Config.qml`,
  `ryoku/shell/quickshell/shell/modules/visualizer/VisualizerView.qml`).
- **A look turns through a full circle and flips, by hand.** Placement could move
  and size a box but not aim it, so a spectrum could only run along an axis. The
  box grows a dot on a stem above its top edge that turns it freely (`angle`, with a
  live degree readout, `R` to square it) and a FLIP button, on `F` too, that mirrors
  what is on screen: a look growing from an edge swaps to the opposite edge, a
  centred one reverses the band order it is symmetric about, and a polar one reverses
  the way it turns. The turn rotates the drawn pass about the box centre rather than
  the geometry, so it costs one transform instead of re-deriving every band, its
  reflection and its bloom, and the wallpaper tone is read from the turned region the
  look covers rather than from its box. Placement gestures ease toward where the
  pointer asks instead of snapping, and keep easing after the release until they
  arrive, so an unsteady hand still lands a clean size; a turn magnetises to every 15
  degrees and ignores the pointer near the centre, where a pixel is a wild swing. The
  readout is now a bar fixed to the screen edge in the shell's own tokens, since a
  readout of the thing being moved must not move with it
  (`ryoku/ui/SpectrumField.qml`,
  `ryoku/shell/quickshell/shell/modules/visualizer/Placer.qml`,
  `ryoku/shell/quickshell/shell/modules/visualizer/Singletons/Config.qml`,
  `ryoku/hub/quickshell/schema/DesktopPage.js`).
- **Ryostore browses decor, launcher heroes and fastfetch emblems as one Decor
  tab.** They are all pictures you install and point a surface at, but each folder
  had its own top-level plate, so the header collected one per picture kind. Decor
  is now a single plate over three catalogues, switched by the subtab strip Themes
  already used for its providers: `ProviderTabs` takes `{key, label}` entries and
  makes its All and library plates optional, since Decor's plates are whole
  catalogues. The new `fastfetch-emblems` catalogue installs one flat PNG into
  `~/Pictures/ryoemblems`, and the Hub's emblem picker lists that folder, so an
  emblem installed from the store shows up in Settings -> Fastfetch beside the
  brand marks and the decors
  (`ryoku/apps/ryostore/backend/provider_flat_image.go`,
  `ryoku/apps/ryostore/quickshell/App.qml`,
  `ryoku/apps/ryostore/quickshell/ProviderTabs.qml`,
  `ryoku/hub/quickshell/pages/FastfetchPage.qml`).
- **Every look lives in a box you drag and size anywhere on the desktop.** The
  spectrum was pinned to a screen edge by `position`, `span`, `align` and
  `height`, or to a centre by `originX`, `originY` and `size`, so the ring and orb
  sat mid-screen and nothing could simply be put where its owner wanted it. Those
  seven keys collapse into `x`, `y`, `w`, `h` and `grow` (which edge of the box the
  bands rise from), and a config written before the box folds into one on first
  read, so nothing moves on update. `Super+Alt+M`, the Move visualiser row in the
  desktop's right-click menu, or the Hub's Place on the desktop button (through the
  shell's new `visualizer` IPC target) starts placement mode: the box takes an
  outline and a corner grip, a drag moves it, the grip or the wheel sizes it, and
  each step writes to `visualizer.json` as it happens. Right click, Escape or the
  keybind ends it. Both gestures apply the pointer's delta from the press rather
  than its absolute position, so nothing jumps out from under the cursor; the grip
  rode a padded corner before, which sits `radius * sqrt(2)` out, so reading that
  distance as the radius grew the shape away from the hand in every direction.
  Placement runs on its own overlay surface, since the spectrum's own is
  click-through for life and a masked surface does not take a pointer again when
  its region is swapped. While a box is aimed the spectrum rides the top layer so
  no window hides it, aiming one that is off turns it on first, and ending
  placement hands the layer back to the mode, so it drops behind windows again
  unless the overlay mode is what the user chose
  (`ryoku/shell/quickshell/shell/modules/visualizer/Placer.qml`,
  `ryoku/shell/quickshell/shell/modules/visualizer/Singletons/Config.qml`,
  `ryoku/shell/quickshell/shell/modules/desktop/WidgetMenu.qml`,
  `ryoku/shell/quickshell/shell/services/ShellState.qml`,
  `ryoku/ui/SpectrumField.qml`, `ryoku/hyprland/modules/binds.lua`).
- **A ring folds the spectrum down to the bars its circumference can show.** Sixty
  four bands around a legible radius left 4px slivers that read as fur; a polar
  look now averages its bands into what fits at about 14px each
  (`ryoku/ui/SpectrumField.qml`).
- **The desktop visualiser is rebuilt as one GPU pass with eleven placeable looks,
  a Hub gallery, and a preview you can aim.** Every look draws from one analytic
  fragment shader (`ryoku/ui/shaders/spectrum.frag.qsb`) hosted by `Ryoku.Ui`'s
  `SpectrumField.qml`, so the wallpaper and the Hub preview render the same
  geometry and cannot drift. The set grows to eleven: eight looks that grow from an
  edge of their box (`bars`, `split`, `dots`, `segments`, `wave`, `ribbon`,
  `curtain`, `line`) and three polar ones (`radial`, `orb`, `spiral`), all placed
  by the same `x`, `y`, `w`, `h` and `grow`; a stored `circle` aliases to `orb`, so
  an old config keeps rendering. `curtain` is bar-aware: its surface honours
  exclusive zones, so the wave hangs from wherever the bar ends, at whatever
  height and edge, with no bar geometry plumbed through. The Hub's Desktop page
  gains a silhouette gallery of the eleven looks from
  `ryoku/ui/Singletons/VizStyles.qml`, rows gated by a schema `when` so a knob
  only shows where it applies, and a `VizPreview` that is the real geometry: a
  drag on it moves the look's box
  (`ryoku/hub/quickshell/VizPreview.qml`,
  `ryoku/hub/quickshell/schema/DesktopPage.js`,
  `ryoku/hub/quickshell/SettingsSheet.qml`).
- **The spectrum wears the palette accent instead of a second one.** It stepped
  every band a fixed tone off the wallpaper through two roles, so on a gruvbox
  scheme the sweep went yellow while every other surface stayed teal, and over a
  dark picture it washed out to near-white. `primary` is now used as published,
  re-lit only when the wallpaper behind sits too close in tone to separate, and
  the eight ramp stops walk a narrow band either side of it, bass deeper than
  treble (`ryoku/shell/quickshell/shell/modules/visualizer/Singletons/Scheme.qml`).
- **The Limine boot stack now ships from `[ryoku]`, so offline installs detect
  other OSes and get the Snapshots submenu.** `limine-mkinitcpio-hook` (bundling
  `limine-entry-tool`) and `limine-snapper-sync` were AUR-only, so a fully
  offline install skipped them and booted a frozen flat menu: no
  `FIND_BOOTLOADERS` other-OS detection, no UKI tree, no rollback snapshots, and
  `ryoku doctor` failed its limine reconciler every update. Vendored as GraalVM
  `nativeCompile` PKGBUILDs (`packages/limine-mkinitcpio-hook`,
  `packages/limine-snapper-sync`) and pacstrapped via `base.packages`, so
  `bootloader.sh` runs `limine-update` on every install and both ISO variants
  converge with online. `gradle` joins the build toolchain (`repo/build-repo.sh`,
  `.github/workflows/publish-repo.yml`, `installation/tests/container-install.sh`).

### Fixed
- **The desktop right-click menu works again.** Adding the Move visualiser row
  brought an unqualified `import shell.services` into `WidgetMenu.qml`, and that
  module has a `Config` singleton of its own. QML resolves the last import that
  provides a name, so it shadowed the desktop's `Config` and every row read
  `undefined`: no widget could be switched on, no design cycled, no zone snapped, and
  the only row that still worked was the one reading `ShellState` instead. The import
  is namespaced now, which is also why the shadowing cannot come back
  (`ryoku/shell/quickshell/shell/modules/desktop/WidgetMenu.qml`).
- **Restarting the shell no longer turns the visualiser off.** Whether it ran was
  per-monitor in-memory state that started at off, while `enabled` in
  `visualizer.json` persisted what the user chose and nothing reconciled the two: a
  restart, a crash or a supervisor respawn dropped the spectrum, and the Hub's own
  switch could not turn it on in a running shell. The persisted key is the single
  answer now and only the layer, desktop or overlay, stays per-monitor
  (`ryoku/shell/quickshell/shell/shell.qml`,
  `ryoku/shell/quickshell/shell/services/ShellState.qml`,
  `ryoku/shell/quickshell/shell/modules/desktop/WidgetMenu.qml`).
- **A leaned look now leans about its own axes, not the screen's.** With any spin on
  the box, the lean pivoted about the screen's axes and sheared the bands instead of
  turning a trapezoid: Qt composes an item's `transform` list outside its `rotation`,
  so setting both gave the reverse of the order the lean needs. The spin moved into
  the same matrix, applied after the lean, and the tests pin the composition by
  checking the result equals the box-frame lean turned by the angle
  (`ryoku/ui/lib/place.js`, `ryoku/ui/lib/place.test.mjs`,
  `ryoku/ui/SpectrumField.qml`).
- **A notification toast slides away instead of being cut off.** The exit ran for
  180 ms on an accelerating curve, and the surface it lives in dropped to 1 px the
  moment the card left the model, so the window clipped the slide it was in the
  middle of and the last toast simply vanished. The entrance and exit are now 420
  and 340 ms on the emphasized settle the sidebar uses, the surface holds its
  height until the exit has finished, and the unmap delay outlasts it, so a toast
  glides in from the anchored edge and glides back out the same way. On the Power
  Saver profile, or with Reduce motion on, this stays an instant cut by design
  (`ryoku/shell/quickshell/shell/services/Motion.qml`,
  `ryoku/shell/quickshell/shell/modules/notifications/NotificationPopups.qml`).
- **A turned box resized as though it were square on.** The box turns about
  its centre, so growing it moves that centre: growing width by the drag walked the
  grabbed corner away from the pointer along an axis unrelated to the angle. Square
  on it tracked, at a quarter turn it moved half as far and diagonally, and at a half
  turn it did not move at all. The delta is now read in the box's own axes, the
  corner opposite the grip is held still and the centre re-derived from the new size,
  so the grip lands exactly under the pointer at every angle. The maths moved out of
  the placer into `ryoku/ui/lib/place.js` with `place.test.mjs` beside it, which pins
  that as an invariant across eleven angles and five drags
  (`ryoku/ui/lib/place.js`, `ryoku/ui/qmldir`,
  `ryoku/shell/quickshell/shell/modules/visualizer/Placer.qml`).
- **The Hub's fastfetch preview draws the emblem where the terminal draws it.**
  The writer hardcoded `padding {top: 5, right: 5}` into every saved config while
  the model carried only `left`, so the terminal dropped the emblem five rows and
  left a five-cell gap that the preview knew nothing about: it drew the art at the
  top of the card with a two-cell gap. The model now carries all three paddings,
  the writer emits what it holds, and the preview draws from the same numbers
  (left and right shift the emblem and the readout sideways, top drops the emblem
  without moving the text), with a test pinning the round trip
  (`ryoku/hub/backend/fastfetch.go`,
  `ryoku/hub/quickshell/pages/FastfetchPage.qml`).
- **Every visualiser look is now one shader pass, retiring the old per-look CPU
  cost.** Measured at 64 bands and 60 fps (`qs -c shell` process CPU over the
  visualiser-off baseline), the previous renderer cost `+0.5%` for `bars`, but
  `+2.8%` for `line`, `+6.0%` for `dots`, `+6.5%` for `circle`, `+6.8%` for
  `radial`, `+7.3%` for `wave`, and `+13.8%` for `segments`. `segments` alone
  built 640 `Rectangle` items (bands x cells) whose `visible` churned every
  frame, and `wave`, `line` and `circle` rebuilt an SVG path string every frame
  that Qt then re-parsed and re-triangulated, while bloom was a full-screen
  `MultiEffect` blur whose skirt also flooded the inside of every shape. The whole
  spectrum is an analytic shape, so `VisualizerView.qml` drops 701 lines of
  drawing to a thin geometry and palette layer over `ryoku/ui/SpectrumField.qml`,
  with glow, reflection and peak caps inside the shader. Re-measured the same way
  (interleaved medians, five second samples): `segments` `+1.0%`, `wave` `+2.8%`,
  `dots` `+3.4%`, and every other look inside the baseline's own noise
  (`ryoku/shell/quickshell/shell/modules/visualizer/VisualizerView.qml`,
  `ryoku/shell/quickshell/shell/modules/visualizer/Motion.qml`,
  `ryoku/ui/shaders/spectrum.frag`).
- **The desktop wave no longer starts with a diagonal cut across its corner.** The
  old filled path opened at the baseline corner and ran straight to the first
  band's centre, leaving a slanted edge at each end of the spectrum. The shader
  evaluates the curve at the field edge instead, and every edge look now fades
  out over its last few percent rather than stopping dead
  (`ryoku/ui/shaders/spectrum.frag`).
- **The oscilloscope traces at ordinary listening volume.** It drew the monitor's
  raw samples, which are a few percent of full scale, so the `line` look was a
  flat filament unless the volume was near maximum. Motion now normalises against
  a decaying peak the way cava does internally
  (`ryoku/shell/quickshell/shell/modules/visualizer/Motion.qml`).
- **`limine-mkinitcpio-hook` builds against a vendored Gradle, so a distro bump
  cannot block every user update again.** Arch shipped `gradle 9.7.0-1` on
  2026-08-14, and it no longer resolves the `gradle-public-api-legacy` module the
  graalvm buildtools plugin (1.1.3, pinned by upstream's `build.gradle.kts`) asks
  for. `nativeCompile` died during configuration, `makepkg` failed after all three
  retries, and the container-install gate correctly held the publish: no broken
  update reached anyone, but no good one did either. The PKGBUILD now carries
  `gradle-9.6.1-bin.zip` as a checksummed source beside the GraalVM tarball and
  builds with that, the version the last green publish used, so the toolchain is
  hermetic and `gradle` is no longer a makedepend.
- **The desktop shell now loads on a packaged install (no more grey screen).**
  Every shell config root and `shell/services/Config.qml` import `Ryoku.FrameBars`
  (the shared frame-bar schema and menu/bar catalogs), so it is load-bearing.
  `deploy.sh` installed it on the QML import path for dev boxes, but no package
  shipped it, so on every ISO and shell-installer install `qs -c shell` failed the
  import at login and painted a bare grey Hyprland desktop with no bar or
  wallpaper; `ryoku recovery` only masked it by redeploying from source.
  `ryoku-desktop` now installs `Ryoku.FrameBars` to
  `/usr/lib/qt6/qml/Ryoku/FrameBars` beside `Ryoku.Ui`/`PluginKit`, and
  `container-install.sh` now asserts every imported `Ryoku.*` module is packaged,
  so a future module cannot ship unpackaged (`packages/ryoku-desktop/PKGBUILD`,
  `installation/tests/container-install.sh`).
- **Bluetooth, brightness, and controller hardware fixes reach every box.** The
  `.install` now tunes `/etc/bluetooth/main.conf` in place (bluez owns it, so no
  file conflict) and applies the ASUS AMD+NVIDIA backlight kernel param on
  install + upgrade; the PKGBUILD ships the backlight udev rule and the `uinput`
  module-load, and the AUR set adds the game-controller drivers
  (`packages/ryoku-desktop/PKGBUILD`, `ryoku-desktop.install`).
- **The network panel's DNS switch applies without a password prompt.** `ryoku-dns`
  runs through pkexec but no polkit rule shipped, so the DNS buttons silently did
  nothing. `ryoku-desktop` now installs `50-ryoku-dns.rules` to
  `/usr/share/polkit-1/rules.d` (`packages/ryoku-desktop/PKGBUILD`).
- **The bar's "Open audio" button opens a mixer.** It launches `pavucontrol` for
  advanced routing (card profiles, per-app device moves) beyond the bar's own
  native mixer, but `pavucontrol` was never a dependency, so the button ran a
  missing binary. It is a hard `ryoku-desktop` depend now, from the official extra
  repo, so it works on every install path (`packages/ryoku-desktop/PKGBUILD`).
- **`power-profiles-daemon` is enabled, so power-mode switching persists.** The
  daemon shipped only D-Bus activated (`systemctl is-enabled` = disabled), so the
  shell's power-mode switching could not reliably stick. A one-shot `_powerprofiles`
  hook (guarded by `/var/lib/ryoku/power-profiles-enabled`, mirroring
  `_bluetooth`/`_rtkit`) now enables it on install and upgrade
  (`packages/ryoku-desktop/ryoku-desktop.install`).
- **`ryoku-desktop` delivers the WirePlumber policy through materialize.** The
  Bluetooth fragment moved from the package-only `/etc` drop-in into
  `/usr/share/ryoku/config/wireplumber`, so package updates and checkout deploys
  produce the same user configuration and user overlays remain authoritative.
- **Desktop feature tools now reach every box, not just the ISO.** `ddcutil`,
  `gpu-screen-recorder`, `wf-recorder`, `hyprsunset`, `wtype`, `tesseract`,
  `tesseract-data-eng`, `zbar`, `songrec`, `libqalculate`, `openrgb` and `upower`
  lived only in `system/packages/base.packages` (ISO pacstrap) or `optdepends`, so
  a packaged box on `ryoku update` and a shell-installer box never got them: the
  recorder, night light, dictation, OCR/QR, calculator, LED sync, battery readout
  and external-monitor (DDC/CI) brightness were silently dead. They are now hard
  `ryoku-desktop` depends, so the ISO, `ryoku update` and the shell installer
  converge. `tests/shell-tool-availability.sh` gained a reach check (every
  official-repo feature tool must be a hard depend) so the drift cannot recur.

### Added
- RyoStore now opens as an artwork-led living showroom with filmstrip browsing, reversible product details, Library state, and accessible reduced-motion navigation.
- **`ryoku-desktop` ships DDC/CI i2c access and the `ryoku-i18n` tool.** The
  `system/hardware/ddc/` module-load (`/etc/modules-load.d/ryoku-i2c.conf`, loads
  `i2c-dev`) and udev rule (`/usr/lib/udev/rules.d/60-ryoku-i2c.rules`, `uaccess`)
  let `ddcutil` drive external-monitor brightness with no group setup; and
  `ryoku/ui/i18n-sync.py` installs as `/usr/bin/ryoku-i18n` for the Hub's
  Language > Generate with AI button and the autostart key-file seed.
- **`ryoku-desktop` ships the complete lock, lid and session-lifecycle policy.**
  `ryoku-clamshell`, `ryoku-idle`, their user units and the login1 fallback
  drop-in land together. A session watcher selects the confirmed active
  Hyprland/niri login, binds `ryoku-session.target` to activity and logout, and
  has unlimited restart recovery across login1 or D-Bus outages. Paired libalpm
  hooks preserve the old executor and acquire a verified durable sleep block
  before package extraction or removal, then keep it while every real
  graphical user reloads compositor power bindings and replaces qylock, shell,
  idle, clamshell and Ryogami owners; greeter, lock and stale lingering
  sessions are excluded. The first release runs this all-session cutover
  synchronously from managed updater stage two, or schedules an unlimited-retry
  transient after pacman's lock for a direct install. Every new shell must
  report its sleep guard and service owners ready before protection is released.
  Removal keeps sleep blocked for remaining live sessions and restores logind's
  fallback. Closing a lid on AC with an external display remains awake.
- **`ryoku-desktop` ships the decor art set to `/usr/share/ryoku/ryodecors`.** The
  `Decor` and `Placard` components render from `~/Pictures/ryodecors`; the package
  carries the shipped set so `ryoku doctor` can lay it there on update (the
  installer seeds a fresh box straight from the repo). Moved out of the `Ryoku.Ui`
  QML module, which no longer bakes the art (`ryoku/assets/ryodecors`).
- **`awww` now ships from the `[ryoku]` repo** as a hard `ryoku-desktop`
  dependency, not the AUR. awww (swww renamed upstream) is the animated wallpaper
  daemon the shell drives: `ryoku/shell/ipc/wallpaper.go` runs `awww img` on every
  wallpaper set and starts `awww-daemon` on demand, so a fresh desktop with no
  daemon shows no wallpaper at all and ryowalls can list images but set none. As
  an AUR-only optdepend it was skipped on offline installs and best-effort on a
  failed build, leaving boxes to heal it by hand with `ryoku doctor`. The new
  PKGBUILD builds both binaries from a pinned upstream git commit (default
  features, so no dav1d), and `ryoku-desktop` hard-depends on it, so `pacman -Syu`
  pulls it onto every install and existing box. The publish workflow gains `lz4`
  (awww's pkg-config build probe) and skips `awww` in its official-repo dependency
  check.
- **`ryoku-cursors` now ships from the `[ryoku]` repo** as a hard `ryoku-desktop`
  dependency, not the AUR. It packages the Bibata XCursor family (the theme
  `env.lua`/`autostart.lua` set as `XCURSOR_THEME`/`HYPRCURSOR_THEME` and the Hub
  cursor picker defaults to) into `/usr/share/icons`, built from the pinned
  upstream release tarball (GitHub assets are immutable, so the sha256 is pinned
  for real, GPL-3.0-or-later). As an AUR package (`bibata-cursor-theme-bin`) it
  installed only in the post-install AUR step -- skipped offline, best-effort on
  failure, and never revisited by `ryoku update` -- so a box could come up with no
  configured cursor and a lone fallback bitmap. It is removed from
  `system/packages/aur.packages` (single source of truth) and added, unpinned
  like `wallust`/`awww` (a fixed upstream version, not the monorepo `RYOKU_PKGVER`),
  to `ryoku-desktop`'s depends, so `pacman -Syu` pulls it onto every install and
  existing box.
- Four new `[ryoku]` repo packages build the optional Hyprland compositor plugins
  the Hub can toggle, installed to `/usr/lib/hyprland/plugins/`:
  `hypr-dynamic-cursors`, `ryoku-hypr-plugins` (hyprbars + hyprfocus),
  `hyprglass`, and `imgborders`. Hyprland plugins are ABI-locked to the
  compositor, so each PKGBUILD's `prepare()` reads the build host's Hyprland
  version and checks out the matching plugin commit from upstream's `hyprpm.toml`
  (the same version map `hyprpm` uses); a rebuild always tracks whatever Hyprland
  the repo ships, with no manual commit bumps. If upstream's map has no pin for
  the shipped Hyprland yet (its pins can lag the distro), `prepare()` falls back
  to the plugin's default-branch HEAD (as `hyprpm` does) instead of failing, so a
  Hyprland release that outpaces a plugin's pin table can't abort the `[ryoku]`
  publish over one optional plugin.
  `ryoku-desktop` depends on all four (pinned to its own version), so they reach
  installed machines through `ryoku update` and a toggle never faces a missing
  `.so`. The publish workflow installs the plugin build deps (hyprland,
  hyprcursor, pango, cairo, pkgconf) and skips the new self-repo packages in its
  official-repo dependency check.
- **`wallust` now ships from the `[ryoku]` repo** as a hard `ryoku-desktop`
  dependency, not the AUR. wallust is the wallpaper -> color palette generator the
  shell runs on every wallpaper change (`wallust run <image>` retints kitty,
  Hyprland, and the shell), so "match wallpaper" colors are load-bearing, not a
  best-effort extra. The AUR package pins a checksum against Codeberg's
  auto-generated source archive, which Codeberg regenerates non-reproducibly, so
  the pin drifts and `makepkg` fails the validity check for everyone: the break
  users hit, where colors stopped following the wallpaper because wallust would
  not install. The new PKGBUILD builds from a pinned upstream git commit, which
  sidesteps the archive, and `pacman -Syu` pulls it onto existing boxes on `ryoku
  update`. The publish workflow installs `rust` (cargo builds wallust) and skips
  wallust in its official-repo dependency check.
- **`ryomotion` ships from the `[ryoku]` repo**: Ryoku Motion, the screen-demo
  recorder and editor, built from the OpenScreen fork
  (github.com/ryoku-dev/ryomotion) and rebranded to Ryo Motion. The PKGBUILD
  builds the Electron app from a pinned commit, fetching the fork's pinned node
  22 at build time (its npm 10 runs the electron/esbuild/sharp install scripts a
  newer npm blocks by default) and rebranding name, binary, and appId with
  electron-builder `--config` overrides. `ryomotion <file>` opens a clip straight
  in the editor, so the shell captures with gpu-screen-recorder + a synthesised
  cursor track and hands the clip here with auto-zoom intact.
  `ryomotion --edit` opens the editor straight to its import screen (the island's
  Edit action). Recording from inside the editor is native too: on Linux it
  captures with gpu-screen-recorder (a hard dependency) instead of OpenScreen's
  browser pipeline, falling back to the browser recorder only when gsr is absent.
  It holds a single-instance lock, so a repeat launch (another Studio recording,
  Edit, or clip open) reuses the running app instead of stacking another ~600 MB
  Electron process tree.
  Installs the unpacked app to `/opt/ryomotion` with a `/usr/bin/ryomotion`
  launcher, a `.desktop`, and hicolor icons; `build-repo.sh` picks it up by
  glob. `ryoku-desktop` depends on
  it, so it ships in the ISO and reaches boxes on `ryoku update`.
- **Webcam self-view overlay** replaces the mpv `ryoku-cmd-mirror` PiP. The 力
  deck's Mirror tile and the record island now toggle a shaped, draggable camera
  bubble on a Wayland layer surface, reshaped in place by Figma-style on-canvas
  handles: drag the bottom-right grip to any size/shape, the top-left dot for
  corner roundness, plus a flip toggle. Size, shape, flip and position persist to
  `~/.config/ryoku/camera.json`; it stays across workspace
  switches and gpu-screen-recorder captures it into recordings. The feed is a
  native `CameraFeed` item in the `ryoku-blobs` QML plugin, which now hard-depends
  on `qt6-multimedia` (linked at runtime, required to build) so the `.so` loads on
  `ryoku update`. The retired mpv script and its `float-webcam-mirror` Hyprland
  window rule are removed.

### Changed
- **`ryomotion` now ships a distinct Ryo Motion icon**, not the upstream
  OpenScreen artwork (`pkgrel` 11, so `pacman -Syu` upgrades existing boxes).
  The icon is a purpose-generated flat camera-aperture mark on Ryoku's dark tile
  in the brand orange, matching the ryowalls/ryovm set (not the generic Ryoku
  seal). The PKGBUILD bundles it (`ryomotion-logo.png`) and overrides the fork's
  assets at build time: it replaces the in-app empty-state and tray logo
  (`public/openscreen.png`, before `build-vite`, which vite copies into the
  bundle) and installs it as the app icon for every hicolor size, in place of the
  fork's green-aperture icons. It is a build-safe asset swap (a sha256-validated
  local PNG copied over an existing file), so it cannot fail the Electron build.
  The remaining "OpenScreen" i18n text (About/Project strings) and the recording
  HUD/launch island stay fork-source concerns.
- **The live (video) wallpaper backend now ships hard via `ryoku-shell`, not as
  GPU-picked `ryoku-desktop` optdepends.** `ryoku-shell` also builds and installs
  `ryoku-livewall`, the tiny C video-wallpaper daemon (it software-decodes a
  downscaled clip into `wl_shm` on a `wlr-layer-shell` surface, so ~40 MB RSS on
  any GPU vendor instead of mpv/mpvpaper's 300-700 MB), gaining `wayland`,
  `wayland-protocols`, and `ffmpeg` makedepends plus `ffmpeg` and `wayland`
  depends. `ryoku-desktop` drops its now-obsolete `phonto` (AMD/Intel VAAPI) and
  `mpvpaper` (NVIDIA NVDEC) optdepends, since the one backend reaches every box
  through the hard `ryoku-shell` dependency
  (`release/packages/ryoku-shell/PKGBUILD`,
  `release/packages/ryoku-desktop/PKGBUILD`).
- **`waifu2x-ncnn-vulkan` is now a hard dependency of `ryoku-desktop`** (moved out
  of optdepends), so ryoshot's Beautify HD ×2 export and ryowalls Enhance reach
  every user: `pacman -Syu` pulls it onto existing boxes on `ryoku update`, and it
  is in `system/packages/base.packages` for fresh ISO installs. An optdepend never
  installs on `-Syu`, so existing boxes would never have received it.
- **The publish is now gated on the container-install smoke test.** On every push
  to `main` (and on a release tag), `publish-repo.yml` first builds the packages,
  installs `ryoku-desktop`, and materializes a full config on Arch and CachyOS;
  the sign-and-upload job `needs` that gate, so an unresolved dependency or a
  config no package ships fails the publish instead of reaching users. The Hub
  surfaces an update the moment the repo db lands, so the test now has to pass
  before the db is published, not after (`.github/workflows/publish-repo.yml`,
  `installation/tests/container-install.sh`).

### Fixed
- **Every build toolchain now installs the full makedepends union of the
  `[ryoku]` packages.** The repo builders (`publish-repo.yml`, the
  `container-install.sh` smoke test, and `install-test.yml`) build with `makepkg
  --nodeps`, so each package's makedepends must be pre-installed; the ISO build
  (`build-iso.yml`) prebuilds the Ryoku.Blobs plugin. `qt6-multimedia` + `ffmpeg`
  (Ryoku.Blobs plays video) and the Hyprland plugin libs were missing across
  them, so `ryoku-blobs` failed `build()` (Qt6 Multimedia not found), the
  compositor plugins failed `prepare()`, and the ISO stage-check failed. All four
  toolchains now install what they build.
- **The publish dependency gate no longer rejects `ryomotion`.** The gate
  `pacman -Si`s every hard dep and skips sibling `[ryoku]` packages via an
  allowlist; `ryomotion` (now a hard dep of `ryoku-desktop`) is a `[ryoku]`
  package but doesn't match the `ryoku*` glob, so the gate searched the official
  repos for it and failed the publish. It's allowlisted now.
- **The camera self-view now hides when recording stops.** The webcam bubble is
  a recording companion, so the shell clears it when the last capture ends (a
  plain mirror toggled on with no recording stays until toggled off).
- **A new Studio recording no longer wipes an open Ryo Motion edit.** `ryomotion`
  (repinned, `pkgrel` 9) reuses the running app for a new clip; it now runs the
  save / discard / cancel dialog before replacing the editor instead of
  force-closing it, so an in-progress edit is never silently lost.
- **The portal file chooser renders dark in a dark session.** `gnome-themes-extra`
  is now a hard dependency of `ryoku-desktop`, so the `Adwaita-dark` GTK theme the
  Hyprland autostart selects (`gsettings gtk-theme`) actually exists on disk.
  Nothing pulled it in before, so the name resolved to nothing and every GTK3 app
  -- the `xdg-desktop-portal-gtk` file/upload dialog most visibly -- fell back to
  light. `pacman -Syu` pulls it onto existing boxes on `ryoku update`; it is also
  in `system/packages/base.packages` for fresh ISO installs.
- **A published package filename never changes bytes again.** makepkg is not
  reproducible (BUILDDATE alone reshuffles the compressed bytes), and every
  publish rebuilt the fixed-version packages (`gpk`, `ryoku-keyring`) and
  overwrote their live files in place, packages-first-db-last. Any client
  whose db, HTTP cache, or `.part` resume predated the newest overwrite hit
  pacman's size cap: "Maximum file size exceeded", the 2026-07-08 curl-install
  failures on `gpk-0.5.8-1` (and issue #21's second act). Three changes close
  it: `build-repo.sh` now adopts the mirror's bytes for any name it already
  serves and re-signs them (shipping a real change means a pkgrel bump, which
  changes the name); the publish workflow runs one at a time (concurrency
  group, never cancelled mid-upload); and a post-upload step verifies every
  file the served db lists exists at the recorded size with its `.sig`,
  failing the publish instead of user installs. `gpk` and `ryoku-keyring` got
  a one-time pkgrel bump so every poisoned cache and stale db converges on
  virgin filenames.
- **The desktop set moves in lockstep or not at all.** `ryoku-desktop` now pins
  its monorepo components (`ryoku-shell`, `ryoku-hub`, `ryoku-rashin`,
  `ryoku-blobs`, `ryoku`) to its own version: every publish rebuilds them all
  with one shared version, and the shell QML this package ships must never run
  against another release's compiled plugin or daemon. A partial upgrade now
  fails loudly instead of skewing silently. The package is also `x86_64` now,
  not `any`: it compiles Go helpers into the payload.
- **Publish CI verifies every hard dependency exists in the official repos.**
  Packages build with `--nodeps`, so a typo'd or AUR-only depends entry used to
  publish cleanly and then brick every user's next `pacman -Syu` with an
  unresolvable target (the Material Symbols font dep was one review away from
  exactly that). The publish now fails instead.
- Audio no longer crackles and pops under load. `ryoku-desktop` now depends on
  `rtkit` and enables `rtkit-daemon` (once, on install and on upgrade, the same
  one-shot pattern as `bluetooth.service`), and the installer enables it too. Without
  it PipeWire could not get realtime scheduling (no realtime group, no PAM limits),
  so its audio thread ran `SCHED_OTHER` and got preempted during a Discord video
  call or over Bluetooth, underrunning the buffer into crackling. rtkit hands that
  thread realtime priority over D-Bus with no per-user setup. This is the real fix
  behind "EasyEffects made it stop": raising the buffer only masked the missing
  realtime scheduling.
- Bluetooth calls are less telephone-muffled and music is higher quality.
  `ryoku-desktop` ships a system WirePlumber drop-in
  (`/etc/wireplumber/wireplumber.conf.d/51-ryoku-bluetooth.conf`) that enables mSBC
  wideband speech for the headset (HFP) profile and prefers hi-fi A2DP codecs
  (LDAC, AptX, AAC) over plain SBC. Classic Bluetooth still cannot carry hi-fi
  output and a mic at once, so a call is not A2DP quality, but this is the best the
  profile allows. Users can override it in `~/.config/wireplumber/wireplumber.conf.d/`.

### Added
- `ryoku-desktop` hard-depends `ttf-material-symbols-variable`. The shell's whole
  pill/bar iconography is the "Material Symbols Rounded" font (`MaterialIcon.qml`);
  it was only in the pacstrapped base set, so a box that predates that addition
  never received it on `ryoku update` and every icon rendered as its ligature name
  ("wifi", "power_settings_new"). Official repo, so a hard depend pulls it onto
  existing boxes on the next update.
- `ryoku` and `ryoku-rashin` declare their optional tools: `lua` (the `luac`
  config-syntax pre-check the Hyprland doctor reconciler prefers) and `sqlite`
  (the `sqlite3` introspection the rashin agent uses). Both are guarded
  fallbacks, so absent them the feature degrades rather than breaks; they ride
  as optdepends, surfaced for the curious.
- unstable-dev climbs its own version now. A new `unstable-version-bump`
  workflow bumps `VERSION` one patch on every push, rolling to the next minor
  once the patch passes 9 (`0.1.9` -> `0.2.0`), and keeps the hand-set beta line
  (`0.1.2-beta.16` -> `0.1.3-beta.16` -> ...), so the base grows with the work
  instead of sitting still until a release. `main` is never touched here; it
  holds its version until unstable-dev merges in and adopts whatever the base
  has reached. A push that hand-edits `VERSION` (a new beta, minor, major, or
  base) is taken as deliberate and left alone. `ryoku-release-bump` gained a
  `roll` bump (patch, carrying to minor past 9) and a `keep` stage (bump the
  number, keep the pre-release suffix).
- `ryoku-rashin` ships the `rashin` terminal command as a `/usr/bin/rashin`
  symlink to the daemon binary (argv0 dispatch, the busybox pattern), and
  `ryoku-desktop` lays `ryoku/apps/fish/conf.d/rashin.fish` into the base
  config tree so materialized desktops get the interactive wrapper, the Alt+R
  binding, the learning hook, and the recipes loader. Both stay inert until
  Rashin is enabled. See the ryoku/apps and docs changelogs.
- `ryoku-desktop` ships the PipeWire drop-in (`ryoku/apps/pipewire/`) in the
  base config tree, so materialized desktops get audio that follows a newly
  connected device (see the ryoku/apps changelog).
- `release/packages/ryoku-rashin/`: a PKGBUILD for the optional Ryoku Rashin
  daemon (`ryoku-rashin`), built from the in-repo `ryoku/rashin/backend` with
  `CGO_ENABLED=0 go build -trimpath` like `ryoku-hub`; the build needs network
  for its one module dependency (`github.com/coder/websocket`). `ryoku-desktop`
  now depends on it, so the binary ships with the desktop but stays inert until
  the user enables it (optional means not running, not absent). It carries no
  runtime depends: Hermes is per-user opt-in, and kitty and xdg-open come with
  the desktop.
- `ryoku-rashin` pre-indexes the monorepo at package build: `build()` runs the
  freshly built binary's `repo-index` over the release tree and `package()`
  installs the snapshot to `/usr/share/ryoku/rashin/ryoku-repo.md`, so the
  installed target (which has no checkout) ships with the source map its agent
  vault folds in on every reindex.
- `ryoku-desktop` ships the Nautilus stash menu extension
  (`ryoku/apps/nautilus/ryoku-stash-menu.py`) to
  `/usr/share/nautilus-python/extensions/`, so the file-manager Install / Compress
  / LocalSend actions load for every user with no per-user materialize step.
- `ryoku-desktop` ships the first-party GUI apps (`ryovm`, `ryowalls`) via a
  generic apps loop: each `apps/<name>/quickshell` config, its `bin/` and Go
  helpers (e.g. `ryovm-fetch`), its `.desktop`, and its `logo.svg` as the launcher
  icon under `/usr/share/icons/hicolor/scalable/apps/<name>.svg`. App marks carry
  an intrinsic `width`/`height` so Qt's icon engine renders them (a `viewBox`-only
  SVG resolves but draws blank). `go` is a make-dependency for the helpers. The
  `.install` refreshes the hicolor icon cache and desktop database on
  install/upgrade. The old single-purpose `ryoku-vm` launcher is removed (its
  passthrough VM is configured from Ryoku Settings > GPU; general VMs run in ryovm).
- `ryoku-desktop` now ships the `Ryoku.PluginKit` QML module (to
  `/usr/lib/qt6/qml/Ryoku/PluginKit`, beside `Ryoku.Blobs`) and the
  `ryoku-plugins-place` helper on PATH, so shell plugins find the signature kit
  and persist their placement on an installed system. The `plugins` Quickshell
  config rides along in the packaged `quickshell/` tree.

### Changed
- The desktop packages built from the monorepo (`ryoku-desktop`, `ryoku`,
  `ryoku-shell`, `ryoku-hub`, `ryoku-blobs`) are now versioned per build as
  `<core>.r<commit-count>.g<short-sha>`: `bin/ryoku-release-version --pkgver`
  computes it and `build-repo.sh` injects it as `RYOKU_PKGVER`, which each PKGBUILD
  reads. Every published build is then a strictly newer, commit-identifiable pacman
  version, so `ryoku update` (pacman -Syu) delivers commits pushed after a user's
  ISO instead of seeing a static `0.1.0-3` forever, and `ryoku status` and the Hub
  show the exact commit. `gpk` and `ryoku-keyring` keep their own versions (pinned
  upstream release, key-rotation date).
- Rebuilt the `[ryoku]` repo for the new desktop shell work: `ryoku` to
  `pkgrel=3` (the CLI gains `ryoku recovery`, a last-resort restore) and
  `ryoku-desktop` to `pkgrel=2` (ships the reworked Hub shell-settings editor
  and the live, config-driven desktop visualiser). Republished so a fresh
  install and `ryoku update` both deliver the new shell.

### Fixed
- `ryoku-desktop`: make `nautilus` + `nautilus-python` hard depends instead of
  optional. The Ryoku stash actions (Install/Compress/Send with Ryoku) ship as a
  nautilus-python extension, but nautilus-python was only an optdepend and
  `pacman -Syu` never pulls optdepends, so existing boxes updated without it and
  the right-click menu never loaded. As a depend it reaches every path: pacstrap
  via `base.packages`, the standalone installer transitively, and existing boxes
  on `ryoku update` since the auto-generated `pkgver` bumps.
- Installs no longer fail with "Maximum file size exceeded" from repo.ryoku.dev
  (issue #21). The publish workflow uploaded the repo in one `rclone sync` with no
  ordering guarantee, so a partial or mid-flight run could leave the live `ryoku.db`
  naming a package whose `.sig` was not up yet; pacman fetches that missing `.sig`
  as an HTML 404 that overruns its signature size cap and aborts under
  `SigLevel=Required`. Publish now goes packages + sigs first, db last, prune last,
  so the db is never ahead of what it references.
- `ryoku-desktop`: depend on `bluez` + `bluez-utils`, and one-shot enable + start
  `bluetooth.service` from the `.install` (guarded by a marker under
  `/var/lib/ryoku` so a user who later disables the service stays disabled
  across upgrades; inside the installer chroot only the enable symlink lands).
  Heals installs that predate the bluez dependency, where the Hub/pill
  Bluetooth UI sat dead against a missing daemon.
- `ryoku-desktop`: depend on `papirus-icon-theme` and install `qt6ct/qt6ct.conf`
  in place of the removed `kdeglobals`, matching the shell's switch back to the
  `qt6ct` Qt platform theme so packaged desktops resolve app icons (the
  launcher's all-apps grid showed broken-image placeholders under the
  never-functional `kde` theme). The dependency is the single source that reaches
  every path: pacstrap already pulls it via `base.packages`, the standalone shell
  installer gets it transitively (it installs `ryoku-desktop`), and existing
  boxes pick it up on `ryoku update` since the auto-generated `pkgver` bumps.
- `ryoku` package now depends on `pacman-contrib`: `ryoku status` (the data the
  Hub and update island read for "check for updates") uses `checkupdates` to
  detect pending updates, but it was not installed on Ryoku systems, so the
  update UI silently reported no updates. Bumped `ryoku` to `pkgrel=2`. Surfaced
  by a full end-to-end qemu desktop update test.
- `ryoku-desktop`: ship the fastfetch emblem
  (`ryoku/assets/brand/fastfetch-emblem.png`) into the base config tree at
  `fastfetch/fastfetch-emblem.png`, so `ryoku materialize` lays it beside
  `config.jsonc` on every update. The readout's logo is a fixed asset the config
  references, but it only reached machines as an installer-time brand-asset seed
  under `~/.local/share`, never through an update; when the emblem was redrawn and
  renamed, updated desktops pointed at a file they never got and fastfetch
  silently fell back to the Arch logo. It now rides the same delivery as the
  config it serves.

### Added
- `release/packages/` PKGBUILDs for the Ryoku desktop, built from the in-repo
  checkout: `ryoku-keyring` (ships the release signing key + trust), `ryoku-shell`,
  `ryoku-hub`, `ryoku`, `ryoku-blobs`, and the `ryoku-desktop` umbrella (configs to
  `/usr/share/ryoku/config`, helper scripts to `/usr/bin`, runtime depends).
- `release/packages/gpk/`: ship GlazePKG (`gpk`), the RyokuArch package manager,
  as a first-class signed `[ryoku]` package (repackaged from the pinned upstream
  release binary, `provides`/`replaces` the AUR `gpk-bin`). `ryoku-desktop` now
  depends on it (`pkgrel=3`), so a fresh install and `ryoku update` always deliver
  `gpk` through pacman instead of the best-effort post-install AUR build.
- `release/repo/build-repo.sh`: builds + signs every package and assembles the
  signed `[ryoku]` pacman database, laid out for `https://repo.ryoku.dev/stable/$arch`
  (real db files, not symlinks, for R2).
- `.github/workflows/publish-repo.yml`: builds, signs, and publishes the `[ryoku]`
  repo to Cloudflare R2 on a push to `main` (the user update channel) and on
  release tags (`v*`), plus manual dispatch; `unstable-dev` never publishes. It
  checks out full history (fetch-depth: 0) so the package version can carry the
  commit count.
- `keys/ryoku-release-key.pub.asc`: the Ryoku release public key
  (`releases@ryoku.dev`, ed25519, fpr `EB6D 3C0F 55A7 B3CA BA6B 2838 847B 274F
  025D D6E3`).

### Verified
- `build-repo.sh` builds + signs all 10 package artifacts and a signed `ryoku.db`
  locally; the db and packages verify as "Good signature" against the release key.
