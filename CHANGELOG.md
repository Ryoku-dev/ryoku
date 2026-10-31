# Changelog

Notable changes to the repository as a whole. Each tree keeps its own changelog
for finer detail.

## Unreleased

### Removed
- The mango compositor is gone: its provider, config tree, packages,
  installer choice, deploy steps, tests and docs. Ryoku ships Hyprland and
  niri. A box that still carries the mango variant is moved onto the variant
  for its running (or installed) compositor by `ryoku update` and the doctor,
  which then remove the mango packages, config and provider binary once the
  user is no longer in a mango session.

### Fixed
- Fedora installs keep existing RPM Fusion FFmpeg and Docker CE packages, and
  no longer request the nonexistent `util-linux-libs` package.
- Users on filtered networks can route `ryoku update` and pacman through a
  local HTTP proxy without weakening package signature checks.
- Screenshots no longer pick up the animated pointer when a cursor effect has
  fallen back to software rendering. Capture tools now hide it only while the
  source frame is frozen and restore the exact state they found; explicit
  include-cursor captures are unchanged.
- The Store installs bundles on every edition. Bundle packages keep their Arch
  names in the catalogue, and on Fedora and Void the catalogue's own edition
  tables name them for that edition, so **Install all** brings in the
  edition's packages instead of failing on Arch names. A part an edition does
  not ship stays listed, greyed out with the reason and where to get it
  instead, and the install skips it; the CachyOS layer and DaVinci Resolve,
  which need Arch's repositories or the AUR, say so up front on Fedora and
  Void.
- The 32-bit GPU drivers (`ryoku-gpu-lib32`) install on Void and Fedora too:
  the helper installs only what the edition provides, turns on Void's
  multilib repository first, and no longer fails with
  `line 51: : command not found` when run as root.
- Store items that theme Vesktop and Zen follow the Flathub builds Fedora and
  Void users install: a Vesktop theme lands in the Flathub folder too, and the
  palette bridge sets up Flathub Vesktop and Zen.
- The CachyOS edition now installs what a vanilla CachyOS install carries:
  CachyOS's own builds of pacman, mesa, the firmware and the kernel layer, its
  own pacman.conf with the repositories for the CPU's tier, the CachyOS LTS
  kernel as the fallback, and the Zen browser, yay and the legacy NVIDIA
  drivers from the CachyOS repositories rather than AUR builds. It still
  installs on any x86-64 CPU; as with CachyOS's own offline install, the first
  `pacman -Syu` moves a v3, v4 or Zen 4 CPU onto its optimized builds, with no
  replacement prompts.
- Stable Release now publishes all four edition repositories from the release
  tag and starts the Void ISO alongside the Arch and CachyOS images. Release
  documentation and installer plans identify the experimental Void, Fedora, and
  Wayfire paths and use the canonical `ryoku-dev/ryoku` repository.
- Void installs now use Ryoku's GPU Screen Recorder 6.1.3 package, including the `gsr-cli` recording controls.
- Void installs now get Mesa's VA-API drivers (`mesa-vaapi`), which Arch's
  `mesa` already carries, so AMD GPUs record the screen and decode video in
  hardware instead of on the CPU. Existing Void boxes pick them up on their
  next update.
- `ryoku-shell` now depends on `inotify-tools` on every edition. Without it
  the python bar style respawned its settings watcher thousands of times a
  second.
- Fedora installs now get `pulseaudio-utils` (`pactl`, used by the doctor's
  audio route check) and `NetworkManager-wifi`, which Arch's `libpulse` and
  `networkmanager` already carry, so Fedora Server and minimal spins have wifi
  in the bar. Existing Fedora boxes pick them up on their next update.
- The Fedora repository now publishes the release control manifest, so
  `ryoku update` installs a desktop package an existing Fedora box is missing,
  as it already does on Arch. Doctor reported "no release manifest to converge
  to" and installed nothing. The manifest carries only the packages the Fedora
  installer installs, never the kernel or compilers. A package removed by hand
  before this update may come back once.
- The doctor's reverse-PRIME warning (#270, a black panel after login on a
  hybrid-GPU laptop) now also fires when a forced or drifted GPU render pin
  coexists with a stored hybrid or passthrough choice, which previously
  silenced the check.
- Lock, suspend, lid-close and wake now share one fail-closed lifecycle across
  Hyprland and niri. Lockscreen generations change atomically during updates,
  every sleep waits for compositor-secure qylock, and output recovery retries
  after resume without leaving an unlock gap during daemon restarts.
- The desktop behaves the same on niri as on Hyprland: the night light, window
  borders, the app and brightness keybinds, the colour picker, the recorder and
  the launcher tools, idle management, the lid policy and Super+P all work on a
  packaged niri box, and Ryoku Hub offers niri's own customization (blur,
  frames, animations, layer rules, input tuning) with no Hyprland-only controls
  or wording. The dev deploy now lays only the live compositor's scripts, so a
  checkout no longer hides what a package is missing.
- Project documentation now uses the Ryoku name and the canonical
  `ryoku-dev/ryoku` repository URL.
- The Now playing widget now respects Power Saver, reduced motion, and the shared
  audio-animation policy instead of keeping its private waveform and decorative
  animations running. Song information and playback controls remain available.
- Ryotunes installs and upgrades now use the official epoch-1 release instead of
  the retired `2.5.1` distro build. Both developer and packaged desktops restore
  a missing installation through the verified release channel.
- The signed pacman repository imports the same official package without
  rebuilding it. An hourly catch-up and a post-publication refresh keep both
  mutable channels current without changing frozen distro snapshots.

### Added
- **Community docks.** RyoStore gains a Docks section: anyone can submit a dock
  as a folder to the ryostore catalogue, users install it there and pick it in
  Edit desktop -> Dock, and it becomes the dock on every monitor and every bar
  style, the same way community launchers and bar styles work. Engawa, a plank
  that lies flush with the screen edge, is the first one.
- **Community app launchers.** RyoStore gains an App launchers section: anyone
  can submit a launcher as a folder to the ryostore catalogue, users install it
  there and pick it in Ryoku Settings -> App Launcher, and Super+Space opens it,
  the same way community bar styles work. Tatami, a full-screen app shelf, is
  the first one.
- **RyoManager joins the default apps.** Fresh installs can keep or drop the
  system monitor; existing boxes get it once through doctor, and a removal
  stays removed.
- **niri gains key sounds.** niri has no plugin system, so it could not have the
  keyboard sound Hyprland gets from its `keysounds` plugin; it has one now, from
  a small Rust engine (`ryoku-keysounds`) that reads the keyboard straight from
  evdev and plays each press through libcanberra, adding no work to niri itself.
  It is not installed by default: Ryoku Hub -> Window Manager -> Behaviour has an
  **Install engine** button that installs and starts it, with on/off, sound
  profile, volume and key-release settings that apply within a second of saving.
  Any pack imported with `ryoku-keysounds-import` joins the profile list. The
  samples and the importer move to their own package, `ryoku-keysounds-profiles`,
  shared with the Hyprland plugin.
- **Commits pass a review gate.** Where `rashin-reviewer` is installed, the
  pre-commit hook refuses secrets, conflict markers, a JSON, YAML, TOML,
  Python, Lua or QML file the commit leaves unparseable, and ShellCheck (CI's
  flags) or gofmt problems on the added lines. Replayed over the last 400
  commits, it stopped seven, each for ShellCheck warnings CI's own flags
  raise or for gofmt slips, and raised no false blocks.
- **Fedora 44 and 45 now carry the complete Fedora-compatible Ryoku desktop.**
  The shell installer supports mutable Workstation, KDE Plasma, Server, and
  spin systems, refuses unpublished releases before sudo, and names the
  published list. Ryoku's signed RPM repository supplies Quickshell, Ryotunes,
  recording, wallpaper, fonts, cursors, controller support, and the Hyprland
  plugins. Both tuned-ppd and power-profiles-daemon work without provider
  replacement. Fedora keeps GRUB, has no snapshot boot menu, stays on
  nouveau, and cannot offer Wayfire until Fedora ships 0.11. Thanks @aethctl for
  the original Fedora port.
- **Void gains GlazePKG, dualsensectl, Oh My Zsh, and Ryotunes.** They install
  and update through Ryoku's signed XBPS repository.
- **CI keeps edition packages in step.** A new Arch package must have a native
  build or an explained translation-table row for both Void and Fedora.
- **`main` is pushed only as the release fast-forward.** The `pre-push` hook now
  refuses every push that moves `main` unless it is named as the release
  (`RYOKU_RELEASE_PUSH=1 git push origin unstable-dev:main`), and even then
  insists on a fast-forward onto a commit `origin/unstable-dev` carries; a
  rewrite or deletion of `main` is refused outright. Work and fixes land on
  `unstable-dev`, which publishes the testing channel on every push. The hook's
  ShellCheck pass uses CI's flags (`-x -s bash --severity=warning`), so a
  rolling box's info-level rules no longer refuse what CI accepts; the release
  push covers every script `main` is missing. `docs/development.md`,
  "Branches and pushing", and `CONTRIBUTING.md` carry the procedure.
- **The installer now asks which window manager, browser, and apps you want.**
  New ISO installs offer a choice of compositor (Hyprland, niri, MangoWM),
  exactly one of the three shipped browsers (Zen, Chromium, Firefox), and a
  keep/remove checklist over every optional app and tool; rows that back a
  desktop feature are marked required and refused. Removed apps are recorded
  in the doctor's provisioning ledger, and `ryoku doctor`/`ryoku verify` now
  treat that ledger as a removal on every lane, so the update manifest leaves
  them deleted. (See `installation/CHANGELOG.md` for the installer detail.)
- **Plain-language GitHub release notes, generated from commit notes.** A change
  users notice gets a `Note: New|Fixed|Removed: ...` trailer on its commit;
  `bin/ryoku-release-notes` collects these between releases and the
  `release-notes.yml` bot publishes them as a formatted GitHub Release, with
  optional demo gifs (`| release/media/...`). A stable `v*` tag becomes a full
  release; each unstable-dev bump refreshes one rolling `unstable` pre-release.
  The `commit-msg` and `pre-push` hooks now also hold subjects to 72 characters
  with no trailing period and validate the trailer. A stable release is
  announced to Discord once the stable channel serves it, and unstable gets a
  roundup about once a day (`release/CHANGELOG.md`). See `CONTRIBUTING.md`.
- **The wallpaper picker's hex layout gains geometric tile families and field
  curves.** The picker can now lay its cards as hexagons, triangles, diamonds or
  rhombi, and bend the column field into a plane, a bow, a ribbon or an S-sweep,
  with adjustable bend strength and wave count. Tile geometry, hover hit-testing
  and grid metrics follow the chosen shape, so cards line up and only their own
  glass area responds to the pointer. Ported from skwd-wall v2. Set it in the
  wallpaper picker's Selector settings (Field curve, Tile family).
- **User edits live in a `user_edits` overlay, separate from Ryoku-owned config.**
  `~/.config/ryoku/user_edits` mirrors `~/.config` and is laid over the base on
  every `ryoku materialize`/deploy, so a file there wins while the base (the
  restore point) still delivers every fix and addition underneath. Overriding by
  overlay (a last-loaded `user.lua`/`settings.lua`/`user.conf`) keeps upstream
  fixes flowing; forking a whole file opts out for that one file. `ryoku reset
  [path]` reverts an override; `ryoku
  recovery` wipes the overlay and the Hub's stores back to shipped defaults.
  Ryoku Settings writes its output into the overlay too, and a seeded `README.md`
  plus clearer file headers explain to hand-editors what edits what. See
  `docs/updates.md`.
- Update-delivery guard: `bin/ryoku-dev-verify-delivery` fails a commit when a
  `ryoku/apps` config reaches no user (shipped by no package, installer, or
  deploy path) and reports how far `main` lags `unstable-dev`. Wired into
  pre-commit, post-commit, and a Delivery check workflow. `docs/updates.md`
  documents the update, materialize, and doctor flow and the delivery contract.
- Fresh repository layout: `installation/`, `system/`, `ryoku/`, each with a README
  and changelog.
- A working installer (Go TUI plus a bash backend) that partitions, optionally
  encrypts with LUKS, installs the base system, configures it, deploys the desktop,
  and sets up Limine.
- A plain Hyprland desktop with kitty, fastfetch, fish, and nautilus, an SDDM
  greeter using the qylock clockwork theme, and Limine with Ryoku branding.
- A `shell/` tree: the full Ryoku shell (a Quickshell bar, panels, launcher, lock,
  and screenshot tool) driven by one Go IPC daemon, `ryoku-shell`, that supervises
  the components and handles every shell control command. Imported and de-branded
  as a base; not yet wired into the installer.
- A shell plugin system: third-party widgets a user places where they like. A
  plugin ships a service plus one adaptive `content/Widget.qml` (glyph / compact
  / full densities); the shell owns the layer, shape, size, and motion of each
  host (frame popout, desktop widget; topbar glyph, window, island to follow),
  so plugins always read as native. Managed in Ryoku Settings -> Plugins
  (enable + pick a host), discovered from `~/.local/share/ryoku/plugins` and the
  user's `plugins.json`, with the signature kit shipped as the `Ryoku.PluginKit`
  QML module. The `ryoku-extras` `plugin` bundle items now install instead of
  being deferred. The legacy `wallhaven` plugin is reworked as the worked
  example.

### Fixed
- The visualizer now accounts for hitbox rotation when determining its placement
  area and allows deliberate edge overhang through the `overhang` property.
- The overview's new-workspace controls now allocate workspace ids globally, so
  clicking `+` or `NEW` on a secondary monitor creates the workspace on that
  monitor instead of jumping to an existing workspace on another output.
- `ryoku doctor` no longer re-writes the SDDM greeter config when it's already
  correct: `readFileSafe` strips the trailing newline on read-back while the
  expected body kept one, so a byte-correct file always looked out of date and
  the sudo rewrite failed silently when no TTY was available. The comparison
  now ignores the trailing newline.
- Limine now shows the generated boot menu: the branded config moved from
  `/boot/limine/limine.conf` (which Limine scans first, shadowing everything
  `limine-entry-tool` generates into `/boot/limine.conf`: the UKI tree and the
  snapper Snapshots submenu) to `/boot/limine.conf` itself, and the EFI binary
  moved onto the path the tool's pacman hook refreshes
  (`EFI/limine/limine_x64.efi`), so the booted bootloader stops aging against
  the installed package. `ryoku doctor` migrates existing installs in place.

### Notes
- The previous Arch tree stays on the `main` branch as reference. The NixOS work
  moved to its own repository.
