# Changelog: ryoku-shell-installer

## Unreleased

### Fixed
- **Converting a box that runs oh-my-zsh-git no longer dies at the desktop
  transaction.** ryoku-oh-my-zsh provides and replaces both upstream
  frameworks, but a plain removal under --noconfirm refuses while an installed
  plugin package depends on oh-my-zsh-git, and the resulting dependency conflict
  aborted the whole desktop install ("la preparation de la transaction a
  echoue"). The conflict step now drops the upstream framework with -Rdd first;
  its plugins re-resolve against ryoku-oh-my-zsh's provides in the same install
  step, and the undo script puts the framework back. Rebuilt the committed
  binary + checksum.

### Fixed
- **Converting a box no longer aborts on the Plymouth splash theme.**
  `ryoku-desktop` owns `/usr/share/plymouth/themes/ryoku/`, so a resume after a
  killed run, a box carrying an older Ryoku deploy, or the ISO installer's seeded
  theme made `pacman -Syu ryoku-desktop` die with
  `/usr/share/plymouth/themes/ryoku/bullet.png exists in filesystem`. The desktop
  transaction now `--overwrite`s that path alongside the privileged helpers and
  polkit rules (`ryokuOverwriteGlob`), matching the ISO installer and `ryoku
  update`, so the package adopts the unowned copies instead of aborting. Rebuilt
  the committed binary + checksum.
- **Converting a host that keeps its own bootloader no longer aborts on the
  limine hooks.** `limine-mkinitcpio-hook` and `limine-snapper-sync` pull limine
  and mkinitcpio back in as depends and ship hooks under `/etc/pacman.d/hooks`,
  which collide with a distro that already owns those paths (Garuda's
  `garuda-hooks`). They now sit in the boot-chain skip list next to limine,
  mkinitcpio and snapper, so a converted box keeps its existing boot stack.
- **Existing Arch systems with a supported ASUS Aura laptop keyboard now select
  the native `asusctl` provider during conversion.** The installer sparse-checks
  out and runs the same hardware probe as the ISO backend; other hardware never
  receives the package.

- **Installing over Omarchy 4 no longer dies at "Updating the system".** Omarchy
  ships an alpm hook (`00-omarchy-update-guard.hook`, `AbortOnFail`) that refuses
  any `pacman -S -u` it did not start, and both `updateCmd` and `installCmd` are
  `-Syu`, so the conversion aborted with `failed to commit transaction (failed to
  run transaction hooks)` before a single package was fetched. The hook is now part
  of what "Retire the Omarchy repo" retires -- moved aside as `.pre-ryoku` with the
  undo in `restore.sh`, because it would otherwise keep blocking `ryoku update` on
  the converted box -- and every `pacman` call carries the escape hatch the guard
  itself documents until that happens, so declining the retirement no longer means
  declining the install. Rebuilt the committed binary + checksum.
- **Converting a box no longer seeds over its default apps.** The installer
  seeded Ryoku's map into `~/.config/mimeapps.list`, the file every "Set as
  default" writes, which then froze: the user's later picks were what an update
  overwrote. `ryoku-desktop` (and, from source, `deploy.sh`) installs the map to
  `/usr/share/applications/mimeapps.list` instead, so the seed is gone from
  `engine.go` and the user's own file is never written by the install. Rebuilt
  the committed binary + checksum.
- **The GPU driver step no longer aborts the whole install, and no longer
  installs against a stale package db.** `stepDrivers` ran the per-vendor
  `pacman -S` scripts (amd/intel/vulkan/nvidia) with no db refresh and returned
  the first non-zero straight up, so a transient mirror hiccup -- or a repo
  publish or a resume landing on this step after the earlier `pacman -Syu`
  steps were already skipped -- made pacman try to fetch package versions the
  mirror no longer serves ("failed retrieving file"), killing the desktop
  install at "Setting up GPU drivers" for AMD and NVIDIA boxes alike. It now
  clears resumed `.part` downloads and runs `pacman -Syu` first (the same
  hardening `stepPackages` already has), then treats a failing vendor script as
  a warning and continues -- matching `installation/backend/lib/drivers.sh` --
  so the box still boots on the iGPU/software renderer and `ryoku doctor` heals
  the driver on first boot. Rebuilt the committed binary + checksum.

- **A shell-installed box now gets the identical package set an ISO box does.** The
  installer reads `system/packages/base.packages` from the payload -- the same
  manifest the ISO pacstraps -- instead of a hand-kept `sessionPkgs` list that had
  drifted (missing `bluez`, `rtkit`, `ddcutil`, `gpu-screen-recorder`, `hyprsunset`,
  fonts and more), so features silently differed between ISO and shell users. The
  boot chain (bootloader/initramfs/encryption/snapshots) is skipped since a
  converting machine owns its own. `ryokuPkgs` is reduced to `ryoku-keyring` +
  `ryoku-desktop` (the umbrella pulls the rest), matching the ISO. Rebuilt the
  committed binary + checksum.

### Added

- Safety gates: non-systemd systems (Artix/openrc/runit/s6/dinit) are
  refused before anything runs; Secure Boot is read from the
  SecureBoot/SetupMode efivars and, when enforcing, forces the NVIDIA
  toggle off and locked (unsigned DKMS modules are rejected at boot while
  nouveau gets denylisted: black screen), with the way out named in the
  plan and at verify time; Manjaro requires typed consent in the TUI, is
  refused under `--yes`, and `RYOKU_ALLOW_MANJARO=1` overrides both.
- DE coexistence: GNOME/KDE/Cinnamon/Xfce are detected, never uninstalled,
  and named as still selectable at login. The greeter theme is its own
  toggle, default off when KDE's `kde_settings.conf` owns
  `/etc/sddm.conf.d` (turning it on writes a `zz-ryoku.conf` that sorts
  last and wins). Ex-GNOME boxes get `pam_gnome_keyring` re-added to
  `/etc/pam.d/sddm` so the GDM-era login keyring keeps auto-unlocking.
  Keyboard and monitor salvage learned GNOME (gsettings input-sources,
  `monitors.xml`) and KDE (`kxkbrc` with `Use=true`, Plasma 6
  `kwinoutputconfig.json`).
- Rice migration: ML4W, HyDE (and hyprdots), JaKooLit, end-4
  illogical-impulse, and Caelestia are detected by marker paths and named
  in the plan; HyDE's user units joined the conflict list and
  `illogical-impulse-*` metas are swept into the rival removal. Plain
  Hyprland setups get their `monitor=`/`monitorv2`/`input` intent salvaged
  from the hyprlang tree (`source=` includes, `$var` expansion, `desc:`
  pins kept) into `monitors_user.lua`/`keyboard.lua`.
- sway salvage: output and input grammar (multi-subcommand lines folded per
  output, `type:keyboard` > `*` > first device) feeds the same pin path;
  sway stays installed as a fallback session and its config joins the
  backup.
- Cross-run resume: completed step ids and the backup dir persist in
  `~/.local/state/ryoku/shell-install-state.json`; a rerun offers to resume
  from the failed step (automatic with `--yes`) and continues the same
  backup dir. Deleted after a fully successful run.
- `--uninstall`: removes the ryoku packages, drops the `[ryoku]` stanza,
  and walks the backup chain newest to oldest running each `restore.sh`
  with confirmation, which also re-enables the services recorded there.
- The plan screen groups toggles under section headers once the list grows
  past ten entries.

### Added

- Standalone no-ISO installer: `install.sh` curl bootstrap plus the
  `ryoku-shell-install` bubbletea TUI. Scans the machine, backs up configs
  with a generated `restore.sh`, removes rival quickshell shells, disables
  conflicting daemons and display managers, trusts the `[ryoku]` repo, installs
  the desktop set, wires SDDM/qylock/NetworkManager, materializes per-user
  config (salvaging the keyboard layout from a niri setup), builds the AUR
  extras, and converges with `ryoku doctor`. Headless `--yes` and `--dry-run`
  modes included.

### Added

- Developer toolchain toggle (default on): installs `dev.packages`
  (go/rust/node/python/mise) for ISO parity. Without go, `ryoku recovery`
  (rebuild from source) failed its preflight on shell-installed machines.
- Omarchy retirement: an ex-Omarchy box keeps the `[omarchy]` repo and routes
  core/extra through Omarchy's own mirror. When detected, the installer drops
  the repo stanza, restores a standard Arch mirrorlist, and removes
  `omarchy-keyring` (originals kept as `*.pre-ryoku`, undo lines in
  `restore.sh`).

- niri migration now carries real intent over: the full xkb setup (layout,
  variant, options) read across config.kdl and its includes lands in
  keyboard.lua, and output blocks (rotation, scale, position, mode, off, VRR)
  become monitors_user.lua pins that autoscale respects. User-level
  xdg-desktop-portal config is moved aside, the systemd user tree is backed
  up so restore.sh can put wants-wiring back, and clipboard/gamma daemons
  joined the conflict list.

### Changed

- `wallust` dropped from the installer's AUR extras (`aurPkgs`): it now ships from
  the `[ryoku]` repo as a hard `ryoku-desktop` dependency, so the packages step
  already pulls it. `stepVerify` treats it as a hard check (colors follow the
  wallpaper) and `stepAUR` no longer best-effort-builds it; awww/mpvpaper stay
  healed by the `ryoku doctor` pass.
- `rust` installs by default now (moved from the devtools-gated `devPkgs` into the
  always-installed `sessionPkgs`), matching the ISO. The shell's Rust AUR
  dependencies (such as `awww`) need the toolchain to build, so it ships whether or
  not the developer-tools toggle is on.

### Fixed

- The `[ryoku]` stanza swap pins `/etc/pacman.conf` to 0644. The whole-file
  rename writes a new inode whose mode followed the invoking user's umask
  (sudo propagates it), so a umask-077 box flipped pacman.conf to 0600 and
  broke every non-root pacman reader, including this installer's own resume
  read; the swap now chmods before the atomic mv, and the stanza is composed
  in one redirection so no helper temp file can be left behind.
- `install.sh` refuses a non-systemd system before downloading anything. The
  binary already refuses (the session and services are systemd units), but the
  bootstrap let Artix users fetch the installer first and fail mid-run; the
  gate now sits with the other preflight checks.
- The `[ryoku]` pacman.conf stanza lands via a whole-file rename, not `>>`.
  A crash mid-append could leave a truncated stanza that pacman rejects while
  a resumed install's "already present" check still matched `[ryoku]` and
  skipped the repair, wedging pacman until a hand edit.
- The post-driver initramfs rebuild probes for the box's actual generator
  (limine-mkinitcpio, mkinitcpio, dracut) and warns instead of aborting the
  install when none is found or the rebuild fails.

- Icon theme reaches shell-installed boxes: `ryoku-desktop` now depends on
  `papirus-icon-theme` (the theme its shipped `qt6ct.conf` selects), so the
  installer pulls it in with the desktop set and the launcher's all-apps grid
  resolves every app logo. The migration backup also carries `.config/qt6ct`
  aside (was `.config/kdeglobals`, no longer shipped) so a prior Qt config is
  preserved before `ryoku materialize` lays the Ryoku one.

- First boot flashed Hyprland's "Your config has errors" overlay: the shipped
  config pcall-requires optional drop-ins that don't exist on a fresh home and
  Hyprland reports the caught failure anyway. The installer now seeds
  comment-only stubs for the six optional files after `ryoku materialize`;
  they become redundant (but stay harmless) once the searchpath-probing
  loader ships in `ryoku-desktop`.
- The package step now survives a repo publish landing mid-install. It
  clears leftover `.part` downloads first (a resume against a mirror whose
  same-name bytes changed trips pacman's "Maximum file size exceeded" cap,
  and every retry inherited the stale prefix), and installs with
  `pacman -Syu` instead of `-S`, so a resumed run transacts against the db
  the mirror serves now rather than the one its first attempt synced. Pairs
  with the publish-side fix that makes published filenames immutable.
- Child output no longer tears the install screen. AUR builds shell out to
  curl, whose progress meter repaints with bare carriage returns; rendered
  raw inside the log panel they jump the cursor to column 0, smear box
  borders across the terminal, and leave panels drawn in the wrong place.
  Every line now passes through a terminal-semantics cleanup (keep the text
  after the last \r, space out tabs, drop control bytes) before it reaches
  the TUI or the log file.
- No child of the installer can ask for a password over the TUI anymore. A
  CachyOS convert watched "[sudo] password for ..." sit on top of the install
  screen until the fish step: the AUR step's `makepkg -si` and the helper's
  own sudo prompt straight on /dev/tty when the cached credential lapses, and
  the TUI never repaints that region, so the prompt both blocks and lingers.
  The yay bootstrap now builds unprivileged and installs through the engine's
  `sudo -n`, helpers run with `--sudoflags=-n`, the driver/sddm/qylock
  scripts use `sudo -n`, and the keepalive refreshes every 20s instead of
  60s, so short sudoers timeouts stay covered and any lapse fails loudly
  into the log instead of hanging invisibly. Each step change also clears
  the screen, so nothing a child writes to the tty can stick around.
- Downloads no longer look like a stalled password prompt. pacman's progress
  repaints end in carriage returns, and the log panel only surfaced
  newline-terminated lines, so multi-minute downloads showed a frozen panel
  with ":: Proceed with installation? [Y/n]" as the last visible line. The
  output reader now treats \r as a line ending too and streams the repaints
  as live progress lines that replace themselves in the panel, rate-capped,
  while the log file keeps only completed lines.
