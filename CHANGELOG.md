# Changelog

Notable changes to the repository as a whole. Each tree keeps its own changelog
for finer detail.

## Unreleased

### Fixed
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
- **Plain-language GitHub release notes, generated from commit notes.** A change
  users notice gets a `Note: New|Fixed|Removed: ...` trailer on its commit;
  `bin/ryoku-release-notes` collects these between releases and the
  `release-notes.yml` bot publishes them as a formatted GitHub Release, with
  optional demo gifs (`| release/media/...`). A stable `v*` tag becomes a full
  release; each unstable-dev bump refreshes one rolling `unstable` pre-release.
  The `commit-msg` and `pre-push` hooks now also hold subjects to 72 characters
  with no trailing period and validate the trailer. A stable release is also
  announced to Discord, reusing the existing `DISCORD_WEBHOOK_URL` secret: the
  published GitHub changelog is posted verbatim in a branded embed (wordmark
  banner, logo mark, brand footer). The rolling `unstable` pre-release is not
  announced, to keep the channel quiet. See `CONTRIBUTING.md`.
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
