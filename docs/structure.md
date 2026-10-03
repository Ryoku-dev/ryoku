# Repository structure

Three pillars, one job each. Everything else is documentation or tooling.

- `ryoku/` the desktop that a user runs.
- `system/` the machine the desktop runs on.
- `installation/` how that machine is built.

The golden rule: **every path has one purpose and appears once.** If you need
something that already exists, reference it; do not copy it.

## `ryoku/` the desktop

Deploys into the user's home (`~/.config`, `~/.local/...`) one way. Source of
truth for the live desktop.

- `apps/` one directory per application, holding that app's native config only:
  `kitty/`, `fish/`, `fastfetch/` (plus the `ryoku-fastfetch` launcher), `nvim/`
  (LazyVim), `yazi/`, `starship/`, `nautilus/`, `npm/` (`npmrc`), `pip/`
  (`pip.conf`), `hyprland-preview-share-picker/` (the screen-share source
  chooser xdph launches). `mimeapps.list` sets the default apps and ships to
  `/usr/local/share/applications/mimeapps.list`, the site layer: below a user's
  own `~/.config/mimeapps.list` (what "Set as default" writes), which always
  wins, and above the distro's, which no Ryoku package should fight over.
  `chromium-flags.conf` pins Chromium's keyring and Wayland backend.
- `hyprland/` the Hyprland config, authored in **Lua**. `hyprland.lua` is the
  entry point and `require`s each module. `keyboard.lua`, `gpu.lua`,
  `monitors.lua` are hardware-managed seeds, and `monitors_user.lua.example` shows
  how to hand-pin a display that autoscale must leave alone. `modules/` is one concern per file
  (`env`, `input`, `displays`, `decoration`, `animations`, `binds`, `ryoshot`,
  `window_rules`, `fullscreen`, `autostart`). `scripts/` holds Hyprland's own
  leaf helpers, the ones bound to the compositor: `ryoku-monitor` (output
  arrangement), `ryoku-workspace`, and the `ryoku-keysounds-import` sample
  importer. The neutral screen and UI helpers the shell drives by bare name (the
  `ryoku-cmd-*` tools, `ryoku-sysinfo`, the recorder chain) moved to the shell
  tree, so every compositor ships them. `plugins/` holds the one compositor
  plugin Ryoku authors, `keysounds` (C++ against the Hyprland plugin API, with
  its `hyprpm.toml` recipe and the sample generator; see
  `docs/hyprland-plugins.md`); it ships as a package, not with the config. The
  rest of the directory deploys to `~/.config/hypr/`.
- `niri/` the niri config, authored in **KDL**. `config.kdl` is the entry point
  and `include`s the rest, in override order: the `keyboard.kdl`, `gpu.kdl`,
  `monitors.kdl` and `monitors_user.kdl` seeds, then the generated
  `settings.kdl` and `rebinds.kdl`, then `user.kdl` as the last word. Every
  seed ships because a missing include is a hard config error in niri, and each
  is comments only: the generated files carry every real setting. There is no
  `modules/` and no `scripts/`, since the neutral settings come from the store
  and the keybind helpers are shell verbs. It deploys to `~/.config/niri/`.
  See `docs/compositors.md`.
- `mango/` the MangoWM config, authored in **mango's ini dialect**. `config.conf`
  is the entry point and `source`s the rest, in override order (last wins): the
  `keyboard.conf`, `gpu.conf`, `monitors.conf` and `monitors_user.conf` seeds,
  then the generated `settings.conf` and `rebinds.conf`, then `user.conf` as the
  last word. A missing source only logs an error, but every file ships so the
  tree is whole from first login. There is no `scripts/`: the keybinds are
  compositor dispatchers or `spawn ryoku-shell`. It deploys to
  `~/.config/mango/`. See `docs/compositors.md`.
- `lockscreen/` `qylock/` (the lock theme and its quickshell lockscreen),
  `install-qylock`, and `sddm/` (the greeter setup).
- `shell/` the desktop shell subsystem: `quickshell/` (the QML UI. Every surface
  runs in-process in a single `qs -c shell` instance under `shell/`, drawn per
  monitor from one scene (`shell.qml`): `modules/` is one directory per surface
  (`bar` the four-edge frame bars with the bounded menu manager, rail status
  popout cards (`bar/popouts/`), and pluggable bar styles (`bar/barstyles/`, see
  `docs/barstyles.md`); `sidebar` the paired global push-aside panels, their
  built-in card catalogue, shared card shell, and `sidebarCard` plugin host (see
  `docs/sidebars.md`); `dock` the app dock on the edge opposite the bar (its own
  surface, shared by every bar style), `launcher`, `overview` (Super+Tab),
  `wallpaper`, `visualizer` (a click-through spectrum layer that renders through
  the shared `Ryoku.Ui` spectrum field, keeping only its per-frame band math in
  its own `Motion.qml`), `osd`, `notifications`, `capture`, `confirm`, and
  `desktop` the wallpaper clock and enabled third-party widgets); `services/`
  holds the shared singletons every surface reads, `components/` the shared UI
  primitives, and `utils/` the shared JS. Beside it are `ryoshot`, `welcome`
  (the first-run
  guided tour), and `plugins` (the third-party shell plugin runtime:
  `discover.sh` merges the catalogue with the user's `plugins.json`, the widget
  host carries desktop placements, and `kit/` is the `Ryoku.PluginKit` QML module
  a plugin imports for the signature look; see `docs/plugins.md`)),
  `plugin/` (`Ryoku.Blobs`, the C++/QML SDF metaball module the frame renders
  with; `build.sh` builds it, and it ships prebuilt), `sidebarfx/`
  (`Ryoku.SidebarFx`, the C++/QML depth edge drawn by the push-aside sidebars),
  `matugen/` (palette templates rendered on every wallpaper change), `qt6ct/`
  (the Qt icon theme, `qt6ct.conf`),
  `scripts/` (the neutral leaf helpers the UI drives by bare name: the
  `ryoku-cmd-*` screen and recorder tools, `ryoku-sysinfo`/`ryoku-profile-stats`,
  `ryoku-app`, `ryostage`, and the stash `.sh` helpers, shipped with the shell so
  they resolve on PATH under any compositor),
  `systemd/` (the user session target), `ipc/` (`ryoku-shell`, the Go shell
  daemon that supervises the Quickshell components, owns wallpaper/clipboard/
  lock and the GNOME keyring password prompt (it registers as the keyring system
  prompter; see `ipc/prompter.go` and `ipc/secretexchange.go`), and serves the
  control socket). `deploy.sh` and `dev-*.sh` are the live
  dev-loop tools.
- `ui/` `Ryoku.Ui`, the shared QML module the shell, the Hub and the apps all
  import for one look: the design tokens (`Singletons/Tokens.qml`), the shared
  primitives, and the wallpaper palette. It also hosts the desktop spectrum
  renderer, so the wallpaper and the Hub preview draw one geometry:
  `SpectrumField.qml` with the analytic `shaders/spectrum.frag(.qsb)` pass draws
  the boxed looks, `AuraField.qml` with `shaders/aura.frag(.qsb)` draws the edge
  field, `Singletons/VizStyles.qml` is the single catalogue of the twelve
  looks, and `lib/spectrum.js` (with `spectrum.test.mjs` beside it) is the pure
  band math, `lib/place.js` (with `place.test.mjs`) the placement math for a box that turns
  and leans, `lib/aura.js` (with `aura.test.mjs`) the sector folding for the field.
  Installs to `/usr/lib/qt6/qml/Ryoku/Ui`.
- `i18n/` the translation catalog and the three runtimes that read it. English
  source strings are the keys, so a developer only ever writes English and a
  missing translation shows English rather than nothing: `langs.json` is the one
  language table (35 languages, add one here and nowhere else),
  `catalog/<code>.json` the generated strings, `catalog/overrides/<code>.json`
  the human fixes the generator may never overwrite, `i18n.go`/`langs.go` the Go
  runtime the CLI and both installers link (module `ryoku-i18n`), and `tools/`
  the extractor/translator (`sync.py`, shipped as `/usr/bin/ryoku-i18n`) and the
  QML AST wrapper. Installs to `/usr/share/ryoku/i18n`, which the QML singleton
  (`ui/Singletons/I18n.qml`), the Go runtime and the installer's shell
  (`installation/backend/lib/i18n.sh`) all read. See `docs/i18n.md`.
- `cli/` the user-facing control CLI, one Go program (`ryoku`): `update`,
  `rollback`, `snapshots`, `status`, `materialize` (lay the base configs into
  `~/.config`), and `reload`. It orchestrates pacman, yay, and snapper; it does
  not reimplement them. `main.go` is a thin dispatcher over the concerns under
  `internal/`: `updater` (update, status, rollback, channel, run-state,
  materialize, version), `doctor` (the convergent reconcilers, report, and
  `--explain`), and `sys` (the shared exec/package/path/terminal primitives,
  defined once). Per-command reference, user- vs developer-facing, in
  `docs/cli.md`.
- `hub/` Ryoku Settings, the central control-center GUI (`Super + ,`): `backend/`
  (`ryoku-hub`, the Go data plane that reads the keybind legend from the live
  Hyprland config, generates the `settings.lua` overlay (in `user_edits`) from JSON, and
  persists hub state as TOML; it also speaks the OpenRGB SDK, so the Lighting tab
  drives keyboards and mice per device) and `quickshell/` (the native Qt6/QML app,
  a `FloatingWindow` with a grouped nav rail and global fuzzy search, with live
  editors for displays, appearance, device lighting, lockscreen, animations,
  input, keybinds, window and layer
  rules, autostart, environment, the shell, and the desktop widgets). The product is "Ryoku Settings"; the binary and
  config keep the internal `hub` name. Deployed to `~/.config/quickshell/hub`;
  built by the shell's `deploy.sh`.
- `rashin/` Ryoku Rashin, the optional agent OS (off by default): `backend/`
  (`ryoku-rashin`, one Go program that maintains the markdown knowledge vault at
  `~/.local/share/ryoku/rashin/`, serves the embedded dashboard on
  `127.0.0.1:3600`, and bridges the Hermes agent over ACP) with its hand-authored
  web dashboard embedded under `backend/web/` (no build step), and the `rashin`
  terminal command (the same binary under a second name: natural language to a
  ready-to-run command plan on the fish prompt, with a `conf.d/rashin.fish`
  weave). The Hub's `RashinPage.qml` is the control surface (enable, one-click
  Hermes setup, open dashboard); built by the shell's `deploy.sh`. See
  `docs/rashin.md` and `docs/rashin-terminal.md`.
- `assets/` `brand/` the 力 logo and icons, `wallpapers/` the shipped wallpaper
  set (installs to `~/Pictures/Wallpapers`), and `ryodecors/` the decor art the
  `Decor`/`Placard` components render (installs to `~/Pictures/ryodecors`, kept
  current by `ryoku doctor`; bake more with `bin/art/ryodither`).

## `system/` the machine

System-level definition installed into the target.

- `boot/` the boot chain: `limine/`, `mkinitcpio/`, `plymouth/`.
- `hardware/` hardware policy and helper scripts (shipped to `/usr/bin` by
  `ryoku-desktop`): `gpu/` (`ryoku-gpu`, `ryoku-gpu-detect`, `ryoku-gpu-mux`,
  udev rule), `display/` (`ryoku-monitor`), `audio/` (`ryoku-mic`, the mic-gain
  normalizer), `drivers/` (per-vendor
  `nvidia`/`intel`/`amd`/`vulkan` install scripts), `power/` (`ryoku-hw-laptop`,
  the shared laptop detector; `ryoku-idle`, the laptop-gated `hypridle` launcher;
  `ryoku-power`, the battery charge ceiling and PCIe link power). What actually
  moves power draw and temperature, measured, is in `docs/power.md`.
- `containers/` the container runtime policy behind the stash Cobalt engine:
  `ryoku-docker` (the one privileged door, shipped to `/usr/bin`) and its polkit
  rule. Its own directory rather than a corner of `hardware/`, because a
  container runtime is not hardware. It provisions `docker.service`, the `docker`
  group, and the single `ryoku-cobalt` container, and exposes no docker
  passthrough on purpose: the polkit grant is passwordless, so every action is a
  fixed argument vector.
- `extras/` the helpers behind the Hub's Extras section, shipped to `/usr/bin` by
  `ryoku-desktop`: `ryostore-install` (installs, removes, and reports the
  optional bundles from the `ryostore` catalogue), the `ryoku-pkg-*` routing
  wrappers (repo, AUR, remove, multilib), and `ryoku-cmd-present`.
- `packages/` the package sets: `base.packages` (every machine, pacstrapped),
  `hardware.packages` (per-profile microcode and GPU drivers), `dev.packages`
  (language toolchains, pacstrapped), `aur.packages` (built post-install).

## `installation/` the build

- `tui/` the Go terminal installer (Bubble Tea). Collects choices, writes the
  `RYOKU_*` contract, gates BIOS/Secure Boot/live-medium/wipe/online, and drives
  the backend.
- `backend/` `ryoku-install` (the orchestrator) and `lib/` (one file per step:
  `preflight`, `disk`, `luks`, `filesystem`, `pacstrap`, `mirrors`, `chroot`,
  `deploy`, `network`, `drivers`, `bootloader`, `aur`, `snapshots`). It reads
  `system/packages/`, adds the `[ryoku]` package repository, and installs the
  desktop onto the target. `alongside` dual-boots with ANY existing OS (Windows or
  another Linux) by creating a 2 GiB XBOOTLDR boot partition + root in free space
  and sharing the disk's existing ESP: Limine lands in its own `/EFI/ryoku`, no
  other vendor's directory is touched.
- `iso/` the archiso profile. `build.sh` bakes the repo payload into the image,
  prebuilds the Go binaries, and runs `mkarchiso` (reproducible for a fixed
  commit). `profiledef.sh`, `packages.x86_64` (live-only set), and `airootfs/`
  complete the live image.
- `tests/` install verification: `container-install.sh` (packaged install in a
  container), `install-vm.py` (real unattended install in QEMU), and
  `iso-stage-check.sh` (the staged ISO tree is byte-reproducible).

## The distribution model

- The desktop ships as signed pacman packages from the `[ryoku]` repository
  (`release/packages/`). `ryoku-desktop` is the umbrella: it version-pins the
  monorepo components (`ryoku-shell`, `ryoku-hub`, `ryoku-rashin`, `ryoku-blobs`,
  `ryoku-sidebarfx`, `ryoku`, and the Hyprland plugins `hypr-dynamic-cursors`,
  `ryoku-hypr-plugins`, `hyprglass`, `imgborders`, `ryoku-keysounds`) and also depends on
  `ryoku-keyring` and the `gpk` package manager, and lays the base config under
  `/usr/share/ryoku/config`.
- The installer adds the `[ryoku]` repo, imports the keyring, and installs
  `ryoku-desktop`; per-user config is then copied into `~/.config` by
  `ryoku materialize`, which clobbers Ryoku-owned files and prunes dropped ones
  but never touches user files.
- It only ever flows **repo to system**. A change starts in the repo, is built
  into a package, and is installed; nothing is harvested back from a live machine.

## Shared, not duplicated

When two subsystems need the same thing, it lives once and both reference it:
`ryoku-hw-laptop` is the single laptop/desktop detector used by both GPU policy
and the idle policy. Reuse the helper; never re-implement its logic.

## `ryoku-shell-installer/` the no-ISO installer

The standalone way in: a curl-able `install.sh` bootstrap plus the
`ryoku-shell-install` Go TUI that converts an existing Arch machine into a
Ryoku one: config backup with a generated `restore.sh`, rival-shell and
daemon migration, `[ryoku]` repo trust, the desktop set, SDDM/qylock wiring,
`ryoku materialize`. After it runs once the machine updates through
`ryoku update` like any other. The binary and its checksum are committed so
raw.githubusercontent.com serves them with no release infrastructure.

## `release/` packaging

- `packages/` one directory per pacman package in the `[ryoku]` repo, each a
  `PKGBUILD`. 40 in all, in four groups by why they exist:
  - built from the checked-out monorepo: the components (`ryoku-shell`,
    `ryoku-hub`, `ryoku-rashin`, `ryoku`, `ryoku-blobs`, `ryoku-sidebarfx`,
    `ryotunes`, `ryogami` the wallpaper daemon), the `ryoku-desktop` umbrella,
    `ryoku-keyring`, and the `gpk` package manager.
  - Hyprland plugins: `hypr-dynamic-cursors`, `ryoku-hypr-plugins`, `hyprglass`,
    `imgborders`, `ryoku-keysounds`; each lays an `.abi` receipt beside its
    `.so` (see `docs/hyprland-plugins.md`).
  - rebuilt from upstream so `ryoku update` can reach them, because it is pacman
    and pacman never touches the AUR: `asusctl`,
    `hyprland-preview-share-picker`, `limine-mkinitcpio-hook`,
    `limine-snapper-sync`, `otf-space-grotesk`, `ryoku-cursors`,
    `ryoku-cursor-material`.
  - hardware support, same reasoning: `xpadneo-dkms` (Xbox pads over Bluetooth,
    which the in-kernel `xpad` does not do), `game-devices-udev` (hidraw
    permissions and battery reporting for 27 vendors' pads),
    `broadcom-bt-firmware` (the `.hcd` patchram blobs the default
    `linux-firmware` set omits, without which a Broadcom adapter never comes up),
    and `dualsensectl` (DualSense lightbar, LEDs and mic; opt-in, since most
    machines have no DualSense).
- `repo/` builds the signed `[ryoku]` repo from those PKGBUILDs: `build-repo.sh`
  runs `makepkg`, signs every artifact with the release key, and `repo-add`s the
  signed `ryoku.db` into `out/`, laid out exactly as the public mirror serves it.

## Tooling

- `bin/` repo tooling: the release version helpers (`ryoku-release-version`,
  `ryoku-release-bump`), the CI/hook checks (`ryoku-dev-scan-slop`,
  `ryoku-dev-audit-shell-binds`), and `art/` for art authoring (`ryodither` bakes
  an image or gif into a 1-bit bone-on-transparent decor; `tiling-demos`
  generates the Appearance tiling-layout preview loops).
- `tests/` standalone CI check scripts (install chroot-safety, shell tool
  availability).
- `.github/` the workflows and issue/PR templates; `.githooks/` the commit gates.
- `VERSION` the base semver; `.woke.yml` the inclusive-language config.
