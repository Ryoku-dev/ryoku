# Ryoku CLI Changelog

## Unreleased

### Added
- **`ryoku wm caps` prints the active provider's capabilities as JSON**, the
  same payload the daemon and the Hub gate on, so a script can read the night
  light backend or a capability without spelling a compositor. `ryoku wm act`
  now forwards the provider's stdout, which is how `input.touchpad status`
  answers (`wm.go`).
- **`ryoku update` re-authors the compositor settings.** The generated config
  is a pure function of the store and the provider that wrote it, so after the
  new config tree lands the live provider applies the store again; a provider
  fix that changes what it emits (niri's border needing an explicit on) reaches
  a box on the update instead of waiting for the next Hub edit
  (`internal/updater/update.go`).

### Changed
- **`ryoku update` adopts the sleep policy as a guarded transaction.** Stage
  two takes a durable login1 sleep block, activates logind's sessionless
  fallback, and refuses to quiesce the shell unless the old `ryoku-idle` and
  `ryoku-clamshell` services are verifiably stopped. The canonical
  `ryoku-power-cutover` helper binds the replacement target to the exact active
  login1 session, reloads the compositor's power bindings, waits for the new
  shell's inhibitor state, and verifies both service owners before protection
  is released; Ryogami is then restarted onto the installed binary. Any earlier
  failure leaves the durable block active until retry or reboot. Package hooks
  use the same all-session helper for every logged-in Ryoku user, while later
  managed updates hand their already-protected active session to it instead of
  racing a second transaction. Stale compositor variables in a lingering user
  manager no longer make an SSH/TTY update own desktop power policy. The stop
  path also adopts releases that had no daemon PID state
  (`internal/updater/update.go`).
- **`ryoku doctor` keeps your login shell honest.** Changing your shell in the
  Hub writes it in two places: your account shell, and a session override the
  compositor exports so everything it launches agrees. Nothing noticed when the
  two drifted, so a box could run fish while `$SHELL` said zsh, and fastfetch,
  terminals and scripts all believed the wrong one. Doctor now names both values
  and points the override back at your real account shell, refreshing the
  running session so it takes effect without a logout
  (`internal/doctor/reconcile_login_shell.go`).
- **The Rashin AI assistant is on by default now.** The needle (Super+S) and its
  dashboard used to sit dormant until you found the switch in the Hub; a fresh
  box now brings the daemon up at boot. `ryoku-rashin disable` turns it off for
  good (recorded as `optedOut`, so an update never flips it back on), `enable`
  turns it back on. A new `ryoku-rashin ensure` is the quiet default-on
  convergence the installer, `ryoku materialize`, and `ryoku doctor` run.
- **`ryoku doctor` keeps the assistant healthy.** It brings Rashin up at boot
  unless you opted out, and enables the AI usage collector timer that feeds the
  bar pill (`internal/doctor/reconcile_rashin_daemon.go`).
- **`ryoku doctor` installs the fingerprint unlock module on a box with a
  reader.** The lock and greeter PAM stacks load `pam_fprintd_grosshack.so`, but
  it was never shipped, so touch-to-unlock did nothing (fprintd enroll/verify in
  Settings still worked). A new reconciler installs `pam-fprint-grosshack` (AUR)
  when a fingerprint reader is present and the module is missing, and is silent
  on a machine without a reader (`internal/doctor/reconcile_fingerprint.go`).
- **`ryoku update` reaps the Hub so a new settings page appears without a
  relogin.** Ryoku Settings is a session-resident quickshell instance the shell
  daemon does not own, so an update left it running on the old QML and a
  just-shipped page (like the bar's silent-drift toggle) only showed after a
  relogin. The shell stop now closes it too (`internal/updater/update.go`).
- **`ryoku doctor` adds `QML_XHR_ALLOW_FILE_READ` to the SDDM greeter env
  (#162).** The greeter theme's bundled I18n reads the shipped catalog with a
  `file://` request, which Qt6 blocks without this flag, so existing boxes
  converge onto a login screen that localises without the shell's Quickshell
  singletons (`internal/doctor/doctor.go`).
- **Neovim config seeds once, like ghostty, so updates stop resetting it.**
  `ryoku update` no longer overwrites `~/.config/nvim` every run: it lays the
  LazyVim starting point on a fresh install and then leaves the tree to the
  user, so edits, plugins and LazyVim's own state persist across updates
  (`internal/updater/materialize.go`, `internal/sys/useredits.go`).
- **`ryoku update` tracks Ryotunes on its own release channel; `ryoku doctor`
  reports it.** Ryotunes is released independently as a prebuilt Arch package on
  ryoku-dev/ryotunes' GitHub releases, so `ryoku update` now installs a new build
  directly through `internal/ryotunesrelease` (fresh release read, sha256 +
  pacman name/version/arch verification, `pacman -U`, upgrade-only) on both the
  git and packaged channels, outside the `[ryoku]` set -- a box with no other
  changes still picks it up, and a newer external build is never downgraded.
  `ryoku doctor` and `ryoku status --json` report a pending release without
  installing it (`internal/ryotunesrelease.Check`), an offline check is never
  rendered as up to date, and Ryotunes is dropped from the explicit `[ryoku]`
  update set so the repo's base build cannot overwrite a newer one
  (`internal/updater/ryotunes.go`, `internal/updater/ryokuset.go`,
  `internal/doctor/reconcile_ryotunes.go`).
- **`ryoku update` moves the Ryoku packages, and nothing else.** It was a full
  `pacman -Syu`, so a Ryoku update decided when a box changed kernel, rebuilt
  its DKMS modules and rewrote its boot image, and `ryoku rollback` could never
  put that half back. It now refreshes the databases, reads the installed
  packages the `[ryoku]` repo serves, and installs exactly those by name
  (`pacman -S --needed ryoku/<pkg>...`, never `-Su`). The base system and its
  kernel come from Arch or CachyOS, whichever the box installed, and
  `sudo pacman -Syu` is what moves them: expected, supported, never blocked by
  a hook. Every run reports what that lane is holding, `ryoku status` prints it
  as `system:`, and `ryoku update --system` runs both lanes in one command (the
  full `-Syu`, `yay -Sua`, `flatpak update`). The refresh now happens before
  the set is read, so a rollback onto a frozen release only asks for packages
  that release served, and targets are repo-qualified so pacman takes our build
  of a name that also exists in `extra` and moves it down as readily as up
  (`internal/updater/ryokuset.go`, `internal/updater/update.go`).
- **`available` in `ryoku status --json` is about the Ryoku lane alone.**
  Pending Arch packages used to set it, so the Hub's update button offered a run
  that would have moved none of them; they are now reported as `systemUpdates`
  beside the existing `packages` list.
- **The boot default is decided by the box, never by a kernel's name.**
  `limineDefaultKernelPath` preferred any entry containing "cachyos", so a plain
  install that added `linux-cachyos` from the Extras toggle had `default_entry`
  repointed at a kernel it never chose as primary. The pick is now
  `/etc/ryoku/default-kernel` (what the installer recorded), then the kernel this
  session booted (`/usr/lib/modules/<release>/pkgbase`), then menu order. No
  kernel name and no brand is preferred anywhere
  (`internal/doctor/reconcile_limine.go`).

### Added
- **`ryoku doctor` reclaims the boot partition a kernel update needs, and says so
  when it cannot.** Two checks, both aimed at the failure behind "pacman -Syu
  never updates the kernel": `initramfs GPU trim` adds `ryoku-gpu-trim` to
  `/etc/mkinitcpio.conf.d/ryoku.conf` and rebuilds the images once, which drops
  the denylisted nouveau driver and its GSP firmware from every kernel image
  (measured on an NVIDIA box: 254 MiB to 147 MiB per image, `/boot` 54% to 43%);
  `boot partition headroom` reports the free space against the largest image and
  warns when there is no room for a rebuild, because in that state mkinitcpio
  builds the image and the hook cannot copy it in while pacman still reports
  success, so the box keeps booting the previous kernel against a module tree
  the upgrade deleted (#140). Nothing on a full `/boot` is safe to delete
  unasked (every candidate is a running kernel's image or a snapshot's only
  matching kernel), so that one names the wall and what to remove
  (`internal/doctor/reconcile_initramfs.go`,
  `internal/doctor/reconcile_boot_space.go`).
- **`ryoku doctor` removes a boot menu entry that boots nothing.** The installer
  writes one flat entry so a fresh box boots before limine-entry-tool ever runs;
  when that tool adopts it as the tree root, any OTHER flat entry an earlier
  installer left is stranded, and `limineDropFlat` only clears those in the
  retired standalone `/+` layout. A box installed as plain Arch that a
  CachyOS-variant installer once touched therefore kept offering
  "Ryoku Linux (CachyOS)", pointing at a vmlinuz that is not on the ESP:
  selecting it drops to the Limine console. The new `boot menu dead entries`
  check removes a top-level entry by evidence, never by name: it has no
  generated children, it names its image on this ESP, and that image is not
  there. A directory with children, an unresolvable volume (a `guid()`
  chainload into another disk) and an image that exists are all left alone, and
  a `default_entry` that named a removed entry is repointed
  (`internal/doctor/reconcile_limine_entries.go`).

### Changed
- `update`: `--overwrite` now also covers `/usr/lib/initcpio/install/ryoku-*`, so
  the install hook an offline install seeds unowned is adopted by the package
  instead of aborting the first `-Syu` with "exists in filesystem"
  (`internal/updater/update.go`).
- **The Depth and Parallax tabs fold into one Stage tab, and the doctor migrates a
  persisted rail.** Depth and Parallax became one feature, so `quick-settings stage
  tab` replaces the retired `depth`/`parallax` modules of a persisted quick-settings
  rail with a single `stage` where the first of them sat (and appends `stage` to an
  older rail that had neither), while `ryostage cache` reclaims a leftover
  `~/.local/state/ryoku/{depth,parallax}` runtime tree once the unified `ryostage`
  cache exists (`internal/doctor/reconcile_stage_module.go`).
- **`ryoku track unstable-dev` means testing packages, `ryoku track main` means
  stable.** A branch name now selects a package channel: `unstable-dev` is the
  `testing` channel (rebuilt on every push to `unstable-dev`, delivered as signed
  packages through `ryoku update`), `main` is `stable` (named releases); both are
  aliases for `ryoku track testing | stable`, and `stable | testing | v<tag>`
  still work. A box whose updates come from a source checkout is migrated onto
  packages by the switch: the recorded repo pointer and the `RYOKU_CHANNEL` line
  in `environment.d`/`hypr/user.lua` are dropped (the `~/ryoku-arch` clone stays
  on disk but no longer drives updates), `ryoku-desktop` is installed from the
  channel when absent, and `ryoku status`/`version`/`doctor` then report the
  packaged channel (`testing`) instead of the stale `unstable-dev`. Building from
  a checkout is now explicit: `ryoku track <main|unstable-dev> --source`
  (`track.go`, `internal/updater/release.go`, `internal/updater/channel.go`,
  `internal/sys/repo.go`, `internal/sys/release.go`,
  `internal/doctor/reconcile_channel.go`, `bin/ryoku-track`).
- **ghostty's config is the user's, and an existing one is migrated.** Materialize
  seeds `ghostty/config` (user-owned, overlay-able through `user_edits`) and
  `ghostty/ryoku-colors` (matugen-owned, so the overlay never re-lays a frozen
  copy), and the new `ghostty theme include` check converts a box that predates
  the split: a config that is nothing but the old generated palette becomes the
  include wrapper with its colours preserved, a config with the user's own
  settings keeps every byte and only gains the include, and once that migration
  has run a removed include is treated as the user's choice and never re-added
  (`internal/doctor/reconcile_ghostty.go`, `internal/updater/materialize.go`,
  `internal/sys/useredits.go`).
- **`ryoku doctor` never puts back an app you deleted.** The applications Ryoku
  ships left `ryoku-desktop`'s depends (pacman rebuilt them on every upgrade), so
  the doctor now owns their delivery: `shipped app packages` installs an app the
  box has never had in one pacman transaction, records it in
  `~/.local/state/ryoku/provisioned`, and from then on treats its absence as the
  user's decision. It also re-marks present apps as explicitly installed so an
  orphan sweep cannot delete them, and the Ryotunes check no longer reinstalls
  the package (`internal/doctor/reconcile_shipped_apps.go`,
  `reconcile_ryotunes.go`). The two spicetify reconcilers are gone with Spotify.
  git and packaged channels -- a box with no other changes still picks it up, and
  a newer external build is never downgraded. `ryoku doctor` and
  `ryoku status --json` report a pending release without installing it
  (`internal/ryotunesrelease.Check`), and an offline check is never rendered as
  up to date (`internal/updater/ryotunes.go`,
  `internal/doctor/reconcile_ryotunes.go`).
- **An edit to a shipped file survives the update as a fork.** `ryoku
  materialize` re-lays every shipped config on each update, so a hand edit
  to, say, `hypr/modules/window_rules.lua` was thrown away. The manifest now
  records the bytes each update left on disk; a shipped file whose live
  bytes match neither those nor the new shipped ones was edited by hand and
  is copied to `~/.config/ryoku/user_edits/<path>` before the base is laid,
  where it wins on top, and the update lists the files it kept. Deleting the
  fork takes Ryoku's version again. The shell's own QML tree is never forked
  (`internal/updater/materialize.go`).

### Fixed
- **`ryoku doctor` re-enables the NVIDIA sleep units the installer sets up.**
  Boxes installed before nvidia.sh grew its enable step, or converted from
  another distro, carry nvidia-suspend/hibernate/resume disabled, so VRAM is
  not preserved across suspend the way the shipped contract arranges; the
  reconciler repairs exactly that drift and stays silent on mesa-only boxes
  and healthy installs (`internal/doctor/reconcile_hardware.go`).

- **"Apply system-wide" for the keyboard layout no longer just says FAILED.**
  Setting the login screen, TTYs, and disk-passphrase keymap rebuilds the boot
  image, which needs root; run from the Hub there is no terminal for the sudo
  password prompt, so it failed every time. It now escalates once through the
  desktop's password prompt (pkexec) instead, so the whole apply goes through in
  one go (`internal/keyboard`).
- **Hybrid-GPU laptops no longer boot to a black screen through the login
  screen.** On a machine with two GPUs (an AMD/Intel iGPU plus an NVIDIA dGPU),
  SDDM could start your session on one virtual terminal while the Wayland login
  greeter was still shutting down on another and still holding a GPU. The
  compositor then found that card busy, dropped it -- usually the very iGPU the
  displays hang off -- and came up headless on the other: a black, blank screen.
  A tiny wait for the greeter to finish letting go of the GPU before the session
  starts fixes it, wired in as SDDM's session command so it covers every Wayland
  session; `ryoku doctor` adds it to existing machines and it is a no-op on a
  single-GPU box (`ryoku/lockscreen/sddm/ryoku-wayland-session`, doctor's SDDM
  greeter reconciler).
- **`ryoku status` no longer reports `snapshots: 0` on a machine that has them.**
  The count came from a `sudo` call that fails whenever no credential is cached
  (every GUI poll, and any cold terminal), and the empty result parsed as a real
  `0` -- "no safety net" when the safety net was fine. The count now runs snapper
  unprivileged first (working with no sudo at all once access is granted), falls
  back to a cached-credential `sudo -n` that never prompts, and treats snapper's
  exit-0 "No permissions." as the failure it is. A read it genuinely cannot make
  now prints `snapshots: unavailable`, never a misleading `0`; `--json` carries a
  `snapshotsKnown` flag so the Hub and the bar island can tell the two apart
  (`internal/updater/update.go`). `ryoku doctor` grants the primary user snapper
  read access (`ALLOW_USERS` + `SYNC_ACL`), so the count shows without any sudo
  (`internal/doctor` snapshot read access reconciler).
- **A symlinked config file (dotfiles) is no longer overwritten with Ryoku's
  default.** If you symlink a seeded file like `hypr/user.lua` or
  `hypr/keyboard.lua` from a dotfiles repo, `ryoku materialize` kept the link --
  unless its target was momentarily unavailable (the repo not mounted yet at
  that point), in which case the existence check followed the dead link, read
  the slot as empty, and laid the shipped default over your symlink, losing it.
  It now tests the link itself, so a symlinked seed is always left alone; a fresh
  install with nothing there still seeds normally (`internal/updater/materialize.go`,
  `sys.PathPresent`). `ryoku deploy` carries symlinked user files the same way.
- **`ryotunes` opened the old Tauri app after the package update.** The launcher
  defers to the native client only while `ryotunesd.socket` exists, and nothing
  enabled that user unit on a fresh install. The doctor now enables it
  (`--now`, no relogin) on every update; the package ships a user preset and an
  install hook so a plain `pacman -Syu` does the same.
- **The wallpaper cutover restarts a stale Ryogami so it repaints (#149).**
  `ryoku doctor` stopped the retired awww-daemon and enabled the Ryogami unit,
  but left a Ryogami already running the pre-cutover binary in place -- which
  painted nothing when no wallpaper was recorded, so the desktop went black
  once the doctor killed a hand-started awww. The reconciler now try-restarts
  the unit when it takes a cutover action, so the delivered daemon takes over
  and restores the recorded wallpaper (or a shipped default when none is
  recorded) instead of the empty grey frame
  (`internal/doctor/reconcile_ryogami_wallpaper.go`).
- **`ryoku update` clears an unowned file that blocks the upgrade, then
  retries.** A `pacman -Syu` aborts the whole transaction when any package (not
  just a Ryoku one) is about to install a file that already exists on disk owned
  by no package -- an installer or deploy stray, a partial extraction. The
  update now reads the "exists in filesystem" paths pacman reports, removes the
  ones no package owns (a file another package ships is a real conflict and is
  left for the user), and retries once, in both the packaged update path and the
  checkout `deploy.sh` upgrade (`internal/updater/update.go`,
  `internal/updater/upgradelog.go`, `ryoku/shell/deploy.sh`).
- **An unowned Ryoku system file no longer freezes every package update.** When
  `ryoku-desktop` began owning paths the ISO installer and `deploy.sh` had
  seeded unowned -- the `ryoku-*` systemd units and the boot configs under
  `/usr/share/ryoku/boot`, on top of the helpers, polkit rules and Plymouth
  theme already covered -- `pacman -Syu` aborted the whole transaction with
  "exists in filesystem", so no package (including the new `ryotunes` app that
  replaces the Chromium wrapper) ever installed and the box silently stopped
  updating. The overwrite set now covers those two families in the packaged
  update path, the checkout `deploy.sh` upgrade, and the doctor's stray-file
  cleanup (`internal/updater/update.go`, `internal/doctor/doctor.go`,
  `ryoku/shell/deploy.sh`).
- **A rejected package database no longer wedges updates.** A box could cache
  a `[ryoku]` sync db whose bytes no longer matched its signature (the mirror
  briefly serves a db and `.sig` from different builds, and pacman refetches a
  db's signature even when it keeps the db), after which every `pacman -S`
  failed with "invalid or corrupted database (PGP signature)" and `-Sy` would
  not replace a db it thought current. `ryoku update` now drops the cached db
  and forces one full refresh when the upgrade is rejected, and `ryoku doctor`
  detects and heals the same wedge (`internal/updater/update.go`,
  `internal/doctor/doctor.go`, `internal/sys/release.go`).
- **Zen scrolls and switches workspaces smoothly.** The shipped Zen policy
  already turned on WebRender and hardware decoding; it now also sets the
  Wayland vsync prefs that issue zen-browser/desktop#5588 identifies as the
  scroll and compositing fix (`layout.frame_rate` -1,
  `widget.wayland.vsync.enabled`, `keep-firing-at-idle`,
  `fractional-scale.enabled`), as unlocked defaults a user can still override
  (`internal/doctor/zen_policies.json`).
- **A package you removed stays removed.** The doctor installed
  spotify-launcher, spicetify-cli and asusctl on its own whenever it saw a
  reason (a flatpak Spotify, an ASUS laptop), so removing them by hand lasted
  until the next `ryoku update`. It now records what it provisioned
  (`~/.local/state/ryoku/provisioned`) and treats a recorded package that is
  gone as your decision; delete its line to let the doctor bring it back
  (`internal/doctor/provision.go`).
- **The Zen policy lands on a packaged Zen.** The doctor wrote
  `<root>/distribution/policies.json` as the user, which fails with
  "permission denied" under `/opt/zen-browser-bin` and `/usr/lib/zen-browser`
  (the AUR and repo installs) on every update. It now writes through sudo when
  the install dir is root's and as the user when it is a tarball under `~`
  (`internal/doctor/reconcile_zen.go`).
- **The CachyOS kernel entry no longer lands in emergency mode after an update.**
  A limine box boots each kernel from a self-contained UKI, and nothing re-checked
  that a kernel's image still matched its module tree. When an update left the
  linux-cachyos image stale -- built for a version no longer installed, missing, or
  older than the kernel -- booting it dropped to an emergency shell while the stock
  linux entry stayed fine (#140). A new `reconcileLimineKernelImages` rebuilds any
  installed kernel's stale or missing boot image on every `ryoku update`, and
  reports and prunes a boot entry for a kernel the box no longer has, so a dead
  second entry stops lingering (`internal/doctor/reconcile_limine_images.go`).
- **A CachyOS install boots the CachyOS kernel by default.** The autoboot default
  pointed at the first kernel the tool listed -- stock `linux` -- so a CachyOS box
  silently booted the Arch kernel (the "it says Arch, not CachyOS" half of #140).
  The default now prefers the linux-cachyos entry when the menu carries one, in the
  doctor and the installer alike; a default the user set by hand is left untouched
  (`internal/doctor/reconcile_limine.go`).
- **`ryoku update` stops resetting hand-edited `/boot/limine.conf` globals.** The
  limine reconcilers rewrote the branding header and the autoboot default on every
  update, clobbering a changed timeout, menu colour, wallpaper, or default kernel.
  They now add a Ryoku global only when it is missing and force just the boot
  identity (`interface_branding`) and the snapshot-safety flag
  (`hash_mismatch_panic`); every other global -- timeout, default_entry,
  remember_last_entry, and all colours -- and any entry or key the user added are
  preserved (`internal/doctor/reconcile_limine.go`).
- **Ryotunes opens the packaged app on every box.** A Chromium YouTube Music
  wrapper or a locally built copy left in `~/.local/bin` shadowed
  `/usr/bin/ryotunes` on PATH, so Super+J and the dock kept opening the old
  Chrome window. The `ryotunes` reconciler removes that copy and its desktop
  entry, installs the package on a box whose channel switch never did, and
  flags an unowned `/usr/bin/ryotunes` (`internal/doctor/reconcile_ryotunes.go`).
- **The login screen keeps its mouse pointer.** The SDDM greeter runs on a
  weston kiosk, and where the greeter or weston falls back to the freedesktop
  cursor theme literally named `default` (SDDM's Wayland greeter ignores
  `XCURSOR_THEME`), a box with no `/usr/share/icons/default` drew no pointer at
  all -- reproducible at every boot and after logout, while the in-session lock
  (which the running Hyprland session draws) stayed fine. A new
  `reconcileGreeterCursor` establishes the fallback, pointing `default` at the
  shipped Bibata set, and only when the box has no default of its own so a user's
  choice is left untouched (`internal/doctor/doctor.go`).
- **Moving to an earlier release works on the first try.** `ryoku rollback
  --to <tag>` (and `ryoku track` in general) failed with "invalid or
  corrupted database (PGP signature)": pacman only refetches a sync db it
  thinks is newer, and a frozen release directory is older than the channel
  the box just left, so the stale cached db met the new signature. A channel
  move now drops the cached `[ryoku]` sync db and runs `-Syyu`; the boot
  guard's revert does the same. `ryoku track <x>` on a box already pointed at
  x whose set never moved (a switch that failed midway) now finishes the move
  instead of saying "already on x". The doctor's OS-line reconciler also
  recognises the Hub-saved spelling of the line (`2\u003e`), which is what
  every box that ever saved through the Hub carries (`internal/sys/release.go`,
  `internal/updater/update.go`, `internal/updater/release.go`,
  `internal/updater/bootguard.go`, `internal/doctor/doctor.go`).

### Added
- **Each release line has an ASCII mark.** `ryoku version --pretty` on a
  terminal draws the line's art (Onogoro: the spear, the drop, the island
  rising from the sea; Amaterasu: the sun) in brand vermilion above
  "Onogoro v0.56.x". Piped output, which is what fastfetch and scripts read,
  stays the one line (`internal/updater/art/`).

### Changed
- **`ryoku rollback` reads as two ways back.** It opens by saying what each
  does (the Ryoku set to a published release, live; the whole system to a
  snapshot, from the boot menu), then a RELEASES block (channel, the running
  release, the ledger with the running one marked, the `--to` and
  `track stable` commands) and a SNAPSHOTS block (id, minute, pre/post,
  description; a one-line summary with free space and whether the boot menu
  lists them). A checkout box is told releases do not apply to it instead of
  being shown nothing. `ryoku rollback <id>` names the snapshot it guides
  (`internal/updater/update.go`).

### Added
- **The doctor moves fastfetch's OS line to `ryoku version --pretty`.**
  `fastfetch/config.jsonc` is seeded once and then owned by the box, so the
  shipped change to the OS line never reached an existing install; the
  `fastfetch OS line` reconciler rewrites that one command in place (the
  BRANCH line and everything else untouched), so the readout says "Ryoku
  Onogoro v0.56.x" everywhere (`internal/doctor/doctor.go`).

### Fixed
- **A testing build's name is not a release tag.** `v0.56.0-beta.19.dev.363+g4d1cf63`
  matched the release-tag shape, so a box moving from testing to a release
  armed the boot guard with a "previous release" nothing frozen stands behind
  (a revert would have failed), and `ryoku track`/`rollback --to` accepted it.
  Only `vX.Y.Z` with an optional `-alpha|beta|rc.N` counts now
  (`internal/sys/release.go`).

### Added
- **`ryoku version --pretty` leads with the release line's name** ("Onogoro
  v0.56.0-beta.19"; fastfetch's OS line uses it), and the name reaches
  `ryoku status` (text and `releaseName`/`channelReleaseName` in the JSON the
  island and the Hub read) and the `ryoku rollback` release list. It comes
  from `/etc/ryoku-release` (`NAME=`) on a packaged box and the checkout's
  `CODENAME` on a dev box (`internal/updater/version.go`,
  `internal/sys/release.go`).
- **Package channels: `ryoku track stable | testing | v<tag>` and
  `ryoku rollback --to v<tag>`.** On a packaged box the channel is the
  `[ryoku]` `Server` line and nothing else; `track` rewrites it and runs an
  update that moves the Ryoku set to what the channel serves, down as well as
  up (`pacman -Syu`, then an explicit `-S ryoku-desktop` whose exact-version
  depends bring the set along). A release tag pins the box to that frozen
  release; `rollback --to` is `track` onto one, so the Ryoku set goes back in
  one pacman transaction while Arch stays current. Bare `ryoku rollback` lists
  the release ledger and the snapshots. `ryoku version` prints the release
  from `/etc/ryoku-release`; `ryoku status` gains `release` and
  `channelRelease` (what the channel serves, read from its `release.json`,
  cached ten minutes), which the update island and the Hub show instead of a
  commit pair. The doctor names the channel it finds and warns, without
  touching it, when `[ryoku]` points at a mirror Ryoku does not publish.
  Checkout boxes keep `ryoku track main | unstable-dev` (`internal/sys/release.go`,
  `internal/updater/release.go`, `track.go`).
- **A boot guard reverts a packaged update whose boots fail.** After a
  release moves the box, stage2 arms `/var/lib/ryoku/update-pending.json`
  (previous release, new release, pre-update snapshot, the boot it ran in).
  `ryoku-boot-guard.service` runs `ryoku boot-guard` early in every boot as
  root: a boot the shell daemon recorded as good (`/var/lib/ryoku/boot/ok-<uid>`,
  written once the shell stays up 45 s) disarms it; otherwise it counts, and
  on the second failed boot tracks the previous release back (the Ryoku set
  only; Arch untouched), re-materializes every user's config from it, and
  leaves a notice the doctor shows once. On a third it points the Limine boot
  menu at the pre-update snapshot entry. The unit and its tmpfiles ship with
  the `ryoku` package; the doctor enables the unit on every update so boxes
  installed before it get it, and `sudo ryoku boot-guard --disarm` clears a
  marker by hand (`internal/updater/bootguard.go`, `systemd/`,
  `internal/doctor/reconcile_bootguard.go`).


- **`ryoku plugin new` scaffolds a plugin in the right place, and `validate`
  audits it.** `new <id> [--bar|--desktop|--popout]` writes a working plugin
  under `~/Documents/ryoku-plugins/<id>/` (manifest, service, view, bar panel
  for `--bar`, README, LICENSE, and an `AGENTS.md` carrying the eleven plugin
  rules so any agent opening the folder follows them) and git-inits it as the
  author. `validate <dir>` now runs a static audit beside the manifest checks:
  blocking rules `symlink`, `escalation` (sudo/doas/su, or pkexec not declared
  in `capabilities.privileged`), `pipe-shell`, `internal-import`,
  `config-write`, `secret`, `binary`; warnings `undeclared-command`,
  `undeclared-host`, `dynamic-shell`, `outside-write`, `large-tree`. Findings
  print as `rule  path:line  message` (`--json` for machines; `--allow <rule>`
  to downgrade one); `add` refuses blocking findings unless `--allow-findings`.
  `list --json` carries each plugin's `capabilities` (`plugin_audit.go`,
  `plugin_new.go`, `plugin_template/`).

### Added
- **`ryoku plugin add` takes a folder, and `export` / `share` carry a widget
  to Ryostore.** `add <dir>` copies a plugin written on this desktop (by hand or
  by Rashin) through the same validation and store transaction a git URL gets,
  so no git repo is needed. `export <id>` writes the installed plugin to
  `~/Documents/ryoku-plugins/<id>/` with `product-manifest.json` (the
  catalogue's per-file sha256/size/mode, docs and preview media `install:
  false`) and `registry-entry.json` (a complete, community `plugins/registry.json`
  row with `hosts` and the `bar-widget`/`desktop-widget` tag), under git.
  `share <id>` exports if needed, then lays it into a fork of `ryoku-dev/ryostore`
  as `plugins/<id>/`, upserts the registry entry, pushes `plugin/<id>` and opens
  the pull request with the catalogue's checklist (as the plugin's author when
  git has no identity); without `gh` it opens the submission form prefilled
  (`plugin.go`, `plugin_share.go`).

- **Doctor keeps the `ryoku` agent skill wired.** When Rashin is enabled but
  its shipped `ryoku` skill is not linked into the always-created agent skills
  dirs (`~/.agents`, `~/.hermes`), the rashin reconciler now runs
  `ryoku-rashin wire`, so an update that ships a new skill reaches every agent
  without a manual step. It stays a no-op when the skill is not installed or the
  links are already in place, and never wires a box that left Rashin off
  (`internal/doctor/reconcile_rashin_daemon.go`).
- **`ryoku update` keeps prowl-agent current, and doctor flags it when
  missing.** After the post-update Rashin reindex, `ryoku update` refreshes
  prowl-agent: on a dev box (on PATH but not owned by a pacman package) it runs
  `prowl-agent update`, and on a packaged box it logs that the binary is managed
  by pacman (the system upgrade already delivered it). A new doctor reconciler
  reports a rashin-enabled box that lacks the binary with the fix
  `sudo pacman -S prowl-agent` (`internal/updater/update.go`,
  `internal/doctor/reconcile_rashin_daemon.go`, `internal/doctor/doctor.go`).
- **`ryoku debug` prints a shareable diagnostic bundle.** The bug issue
  template asked reporters to attach `ryoku-debug` output, but no such command
  existed. `ryoku debug` now prints the same read-only, secrets-free report as
  `ryoku doctor --report` (versions, system state, doctor findings, recent
  error logs) straight to stdout so it pastes into an issue
  (`main.go`, `internal/doctor/report.go`).
- **Browser animations for Zen.** Doctor deploys a Ryoku `userChrome.css`
  animation sheet into each Zen profile (`chrome/ryoku-animations.css`,
  `@import`ed without disturbing a user's own rules) and the Zen policy enables
  the legacy-stylesheet load. CSS-only motion on Zen's own chrome hooks, tinted
  from the palette via `--zen-primary-color`: tab open/switch, url-bar focus
  blur, trackpad swipe, workspace switch, and a top page-load bar. Honours
  `prefers-reduced-motion`; no privileged JavaScript
  (`internal/doctor/reconcile_zen_userchrome.go`).
- **Doctor applies the Ryoku Zen policy whenever Zen is detected.** A new
  reconciler writes a Firefox enterprise `policies.json` into any Zen install it
  finds: the shipped extensions (uBlock Origin and Privacy Badger, installed
  removable, not forced) plus the Wayland / hardware-decode and privacy pref
  defaults, set as defaults the user can still change. It is a no-op without
  Zen, so a user who installs Zen themselves picks up the extensions and
  optimizations on the next `ryoku doctor`, and a box without Zen is untouched.
  The palette-follow Ryoku theme extension is added to the same policy once its
  AMO-signed xpi is shipped (`ryoku/browser/sign.sh`), since Zen refuses unsigned
  extensions (`internal/doctor/reconcile_zen.go`).
- **Doctor clears stale GPU render pins and audits power-profiles-daemon's
  amdgpu actions.** A laptop pinned by an older `ryoku-gpu persist` policy kept
  rendering the whole desktop on the discrete GPU, which then could never
  runtime-suspend (~10 W and a hot idle floor on a static wallpaper); doctor
  now asks `ryoku-gpu check-pin` and clears a pin today's policy would not
  write, keeping deliberate `RYOKU_GPU_FORCE=1` overrides. A second report-only
  check warns when ppd's optional `amdgpu_dpm` / `amdgpu_panel_power` actions
  are enabled against the AMD GPU that composites the desktop: the first turns
  power-saver into whole-desktop lag, the second washes panel colours
  (`internal/doctor/reconcile_gpu_pin.go`, `reconcile_ppd_amdgpu.go`).
- **`ryoku plugin` installs shell plugins from git, through the store's
  supply-chain transaction.** `ryoku plugin add <git-url> [--bar] [--yes]`
  clones to a staging dir, validates the manifest (a well-formed lowercase id
  that is neither a reserved built-in widget nor already installed, a name and
  version, entry points that are relative with no `..` and actually exist, a host
  set that is a subset of `framePopout|desktopWidget|topbarGlyph`, and no
  symlinks anywhere in the tree), then installs it with `ryostore install plugins
  <id> --from <dir>` so the shell's `discover.sh` finds it (the same receipt +
  content-hashed view + journal a store install writes); `--bar` enables it on
  the bar through `ryoku-plugins-place`. It prints an unsandboxed-code warning
  and requires `--yes` or an interactive y/N before cloning, and never runs
  anything from the plugin. `ryoku plugin remove <id>` uninstalls through
  `ryostore remove` and drops the placement; `list [--json]` reports store
  receipts and dev overrides (each row marked `store` or `dev`) merged with the
  placement; `validate <dir>` checks a local tree (`plugin.go`, `main.go`).

### Fixed
- **`ryoku update` no longer needs a manual `sudo -v` first.** The first
  escalation, the pre-update snapshot, ran through `sudo` with no terminal
  attached, so on a fresh credential it could not prompt and the snapshot was
  silently skipped; the only priming lived inside the pacman renderer and never
  covered the snapshot, yay's own escalation, or the stage2 process. The update
  now caches the credential once up front on the terminal (a single prompt, in the
  hand-run terminal and the one-click kitty window alike) and refreshes it for the
  duration, so the snapshot, pacman, yay, the post snapshot and doctor all run off
  it. `sudo -v && ryoku update` is no longer needed (`internal/updater/update.go`,
  `internal/updater/upgradelog.go`).
- **`ryoku update` no longer aborts on the Plymouth splash theme.** Once
  `ryoku-desktop` began owning `/usr/share/plymouth/themes/ryoku/`, boxes whose
  ISO installer had seeded that theme unowned hit
  `ryoku-desktop: /usr/share/plymouth/themes/ryoku/bullet.png exists in filesystem`
  on the next `pacman -Syu`, and the whole transaction rolled back so no update
  landed. The upgrade, the channel switch, and the boot-guard revert now
  `--overwrite` the theme path alongside the privileged helpers and polkit rules
  (one shared `ryokuOverwriteGlob`), and `ryoku doctor` clears the same unowned
  theme files on a box already wedged so its next update adopts them
  (`internal/updater/update.go`, `internal/updater/bootguard.go`,
  `internal/doctor/doctor.go`).
- **Doctor ignores Zen launcher-wrapper directories.** Zen installations must
  carry Firefox's `application.ini` marker before the policy reconciler treats
  them as an install root, so a `/usr/bin/zen-browser` wrapper no longer makes
  doctor try to create `/usr/bin/distribution/policies.json`
  (`internal/doctor/reconcile_zen.go`).
- **`ryoku track` takes effect without a relogin.** The channel it persists
  to `environment.d` now wins over the live `RYOKU_CHANNEL`, which the session
  captured at login and kept reporting after a switch (`ryoku status` and the
  Hub said unstable-dev on a box tracking main); the script also pushes the new
  channel into the user manager and the D-Bus activation env so the shell it
  restarts carries it (`internal/updater/channel.go`, `bin/ryoku-track`).
- **Lock fixes reach existing boxes.** The in-session lock bundle lives under
  `~/.local/share`, outside materialize, and was laid once at install: every
  lockscreen fix since shipped only to fresh installs. `reconcileLockscreen`
  now compares the installed runner and the shipped default skin with the
  package's bundle and re-runs the installer on drift, and refreshes the SDDM
  greeter skin when it is the shipped default (never a skin the user picked)
  (`internal/doctor/reconcile_lockscreen.go`).
- **The login screen's pointer has a cursor theme.** The SDDM greeter runs on a
  weston kiosk with no session env, so it had no XCURSOR theme and depended on
  the "default" chain resolving to something; `reconcileGreeterDisplayServer`
  pins the shipped Bibata set in `GreeterEnvironment` and refreshes an
  out-of-date conf (`internal/doctor/doctor.go`).
- **A rice's fastfetch emblem comes back after the update that reset it.** A
  new `rice fastfetch emblem` reconciler runs `ryoku-hub rice emblem` while the
  readout still draws the shipped brand mark and the active rice carries an
  emblem; a logo the user imported is never touched (`internal/doctor/doctor.go`).
- **Browser theme host installs where Zen actually reads it.** The host
  reconciler keyed Zen off `~/.zen`, but Zen keeps its profiles under XDG
  `~/.config/zen`, so the native-messaging manifest was never installed and the
  palette host was unreachable. It now detects Zen at `~/.config/zen` and writes
  the manifest into `~/.mozilla/native-messaging-hosts`, the classic dir the
  shipped Zen resolves manifests from (verified live)
  (`internal/doctor/reconcile_browser.go`).
- **Doctor repairs stock plocate configs instead of breaking their update
  service.** `updatedb.conf` normally spells its variable as
  `PRUNEPATHS = "…"`, while Doctor only recognized a compact spelling and
  appended a second definition. Plocate rejects duplicate variables, leaving
  `plocate-updatedb.service` failed and `locate` stale. Doctor now updates the
  package-owned assignment in place and collapses the duplicate definitions
  older Ryoku versions left behind without losing configured paths
  (`internal/doctor/reconcile_snapshots.go`).
- **The SDDM Wayland greeter now selects Qt's Wayland platform explicitly.**
  The generated configuration started Weston but omitted
  `QT_QPA_PLATFORM=wayland`; Qt5 SDDM greeters then selected `xcb`, found no X
  server, and aborted to a black screen. Doctor now writes the environment with
  the display-server configuration, and the delivered package set includes the
  ABI-matched `qt5-wayland` QPA plugin for CachyOS's Qt5 greeter
  (`internal/doctor/doctor.go`, `release/packages/ryoku-desktop/PKGBUILD`).
- **Doctor unpins window borders stuck on a stale colour.** While colours
  follow the wallpaper (or a named scheme) the Hub's generated
  hypr/settings.lua deliberately omits `col.active_border` so the palette
  drives the border; a file generated before that rule pinned a fixed colour
  forever, because it only regenerates on a Hub save ("the border is stuck on
  red" while the palette renders fresh colours nobody applies). Doctor now
  spots the stale pin and has the Hub re-emit the file from today's state,
  then reloads Hyprland config-only
  (`internal/doctor/reconcile_border_pin.go`).
- **Updates, rollbacks and resets no longer reset the fastfetch readout (or
  any live-edited seed).** The Hub's Fastfetch editor and the store's readout
  styles edit `fastfetch/config.jsonc` in place, but a frozen copy captured by
  the retired adopt step sat in the user_edits overlay and was re-laid on
  every materialize, reverting the readout each time (users noticed at the
  next shell greeting: reloads, logouts, reboots). Every generated seed
  (fastfetch, monitors.lua, gpu.lua, keyboard.lua, kitty/current-theme.conf)
  now joins user.lua in the never-overlaid set, and doctor migrates any stale
  overlay copy out with a `.overlay.bak` beside the live file, so affected
  machines self-heal on their next update. A stale gpu.lua overlay copy could
  even resurrect a cleared GPU render pin; that door is closed too
  (`internal/sys/useredits.go`, `internal/updater/materialize.go`).
- **The spicetify remedy names the right fix for a flatpak Spotify.** A root-owned
  system flatpak (`/var/lib/flatpak`) cannot be patched without root; the Canvas
  and Marketplace doctor warnings now say to reinstall it per-user
  (`flatpak install --user`) or use the shipped `spotify-launcher`, instead of the
  irrelevant `/opt` chmod (`internal/doctor/reconcile_spicetify.go`).
- **The system font persists again.** `ryoku doctor` listed `fontFamily` among
  the retired style knobs and stripped it from `shell.json` on every run, so a
  font chosen in Hub > Global reverted on the next update. It is a live key
  again and is left untouched (`internal/doctor/doctor.go`).

### Added
- **`ryoku doctor` provisions a patchable Spotify when the only one is root-owned.**
  When Spotify is present solely as a client spicetify cannot patch without root --
  a system-scope flatpak (`/var/lib/flatpak`) or a root-owned native `/opt` -- and
  the shipped writable `spotify-launcher` is not installed, the Canvas reconciler
  now installs `spotify-launcher` so `ryoku update` gets a per-user tree spicetify
  can own. spicetify (Canvas + Marketplace) then wires up on the next update once
  the user opens Spotify once (its tree unpacks on first launch). Best-effort and
  bounded so it never blocks an update; a check-only run reports it as a todo
  instead (`internal/doctor/reconcile_spicetify.go`).
- **The Spicetify Marketplace ships wired in.** A new `reconcileSpicetifyMarketplace`
  drops the shipped Marketplace app (the `spicetify-marketplace` [ryoku] package)
  into a Spotify user's spicetify CustomApps, enables it, lays the transparent
  placeholder theme so theme installs land, and applies it -- so the "store" icon
  in Spotify's sidebar is there out of the box instead of the fiddly manual install
  (fetch a release zip, unzip, `spicetify config custom_apps marketplace`, apply).
  Gated on Spotify installed, aimed at the per-user `spotify-launcher` tree so
  `apply` needs no root, best-effort so it never blocks `ryoku update`, and inert
  for anyone without Spotify (`internal/doctor/reconcile_spicetify_marketplace.go`).
- **`ryoku doctor` flags a phantom Wayland output.** Ryoku's `monitors.lua`
  ends in a catch-all rule so a hotplugged display needs no hand-written entry,
  but that also makes Hyprland enable every connector it reports as connected --
  including a connector with no display on it (a port routed through a hybrid
  laptop's discrete GPU, or a KVM/dock that keeps a dead EDID line alive), which
  comes up as a blank second desktop windows can get lost on. A new
  `phantom Wayland output` reconciler names any enabled, non-internal output
  with no EDID (empty make and model) and zero physical size, and points at the
  one-line `monitors_user.lua` rule that disables it -- unless it is the only
  display, which is treated as a real (if quirky) screen. Report-only: doctor
  never disables an output itself, since that could black out a real monitor
  with a broken EDID (`internal/doctor/reconcile_phantom_output.go`,
  `TestPlanPhantomOutput`; the override is documented in
  `hypr/monitors_user.lua.example`).
- **`ryoku doctor` moves the SDDM greeter to Wayland.** A new
  `SDDM greeter display server` reconciler writes
  `/etc/sddm.conf.d/10-ryoku-wayland.conf` (`DisplayServer=wayland`,
  `CompositorCommand=weston --shell=kiosk`) so the greeter runs on Wayland like
  the session and SDDM tears it down at login, instead of orphaning
  `sddm-greeter-qt6` onto a leftover Xorg where it kept drawing power. Gated on
  weston being installed (a `ryoku-desktop` depend `pacman -Syu` lands first), so
  it never writes a config the greeter cannot start; idempotent, and only ever
  touches its own conf file (`internal/doctor/doctor.go`).
- **`ryoku doctor` migrates the dock's settings out of the qsbar bar style.** The
  dock is a shell surface of its own now, so its five old `qsbar` keys
  (`dockEnabled`, `dockMagnify`, `dockPinned`, `dockFrost`, `dockShadow`) move
  into a top-level `dock` object in shell.json, keeping the user's choices and
  their pin list. The reconciler is idempotent, never clobbers a `dock` object
  that is already there, and leaves the new keys (edge, auto-hide, labels, media)
  absent so the shell's own defaults apply.
- **Pacman draws Ryoku's progress bar.** `ILoveCandy` is now a Ryoku default:
  pacman renders the transfer bar as Pac-Man eating pellets instead of a row of
  hashes. The installer sets it in the target's `pacman.conf`, and a new
  `pacman progress bar` reconciler seeds it into `/etc/pacman.conf` on a box
  installed before it shipped -- uncommenting an existing commented directive,
  else inserting it under `[options]`, and never anywhere pacman would ignore
  it. The seed is one-shot (a marker under the state dir), so deleting the line
  is a decision that stands; `/etc/pacman.conf` stays the user's file
  (`internal/doctor/reconcile_pacman.go`, `TestEnableILoveCandy`).
- **`ryoku doctor` auto-provisions QMK/VIA keyboard lighting.** A new reconciler
  detects a connected VIA board by its 0xFF60 raw HID interface (via the shared
  `ryoku-hw-qmk` detector, no vendor list) and installs `qmk-hid` when one is
  present, skipping every other machine. A shipped udev rule
  (`62-ryoku-qmk-hid.rules`) grants the seat user a uaccess ACL on just that
  device, so the shell's lighting provider drives its RGB matrix without root
  (`internal/doctor/reconcile_qmk.go`, `system/hardware/input/`).
- **Snapshots stop piling up unbounded.** Pruning ran only through
  `snapper-cleanup.timer` -> `snapper-cleanup.service`, whose success is coupled
  to `limine-snapper-sync` (its `ExecStopPost`); when that broke nothing was
  pruned and every `ryoku update` and pacman transaction left another
  unremovable snapshot pair, filling disks with hundreds of GB. `ryoku update`
  now runs `snapper cleanup number` inline after the post snapshot, and a new
  `snapshot cleanup` doctor reconciler keeps `snapper-cleanup.timer` enabled,
  `snapper-timeline.timer` disabled (Ryoku prunes by number only), and batch
  deletes leaked `Cleanup=timeline` snapshots under a number-only config
  (`internal/updater/update.go`, `internal/doctor/reconcile_snapshots.go`).
- **`ryoku snapshots` shows a table, not a dump.** Aligned columns (number,
  type, date, cleanup, description) with a footer for the count, free space, and
  whether the snapshots are in the Limine boot menu, in place of raw `snapper
  list`. A `updatedb snapshot prune` reconciler also stops `plocate` indexing
  `/.snapshots` (`internal/doctor/reconcile_snapshots.go`).
- **Flatpak works out of the box, and its apps update with everything else.**
  Ryoku already shipped the `flatpak` client in `base.packages`, which means it
  is in the ISO's offline closure and installs with no network. What it never
  shipped was a configured remote: the only thing that ever added flathub was
  `stash-install.sh`, per-user, and only while installing a `.flatpak` bundle the
  user had already downloaded. So a fresh box had the client and no catalogue, and
  `flatpak install` had nothing to search. A new reconciler adds the system
  flathub remote, and `ryoku update` gained a Flatpak pass so those apps stop
  rotting a release behind.

  Both halves stay quiet when they cannot help: no flatpak binary, no network, or
  no remote configured yet is a silent skip, not a warning. That ordering is the
  design, and it is what keeps an offline install clean: ship the client offline,
  wire the catalogue on the first run that has a network to wire it to
  (`internal/doctor/reconcile_flatpak.go`, `internal/updater/update.go`).
- **`ryoku doctor` can finish ASUS Aura keyboard support after an update.** On a
  laptop identified by the shared hardware probe it installs `asusctl` when
  missing and starts `asusd`; unsupported machines remain untouched, and an
  installed TLP stack produces an explicit conflict report instead of a failed
  package transaction.

- **`ryoku import <path>` brings an existing setup onto Ryoku from the terminal.**
  A thin front door to the Hub's import engine: it scans a folder, an existing
  `~/.config`, or `--url <git>`, auto-resolves keybind clashes by
  `--keep mine|ryoku` (default mine), applies, and prints what changed;
  `ryoku import --undo [<ts>]` reverses it. Wraps `ryoku-hub import`
  (`internal/importer`).
- **`ryoku doctor` reports a discrete GPU that is pinned awake, either way it happens.**
  On an ASUS MUX laptop set to Discrete mode the panel is wired straight to the
  dGPU, so the card can never enter runtime D3cold: it stays active for the whole
  session and burns ~10 W and ~60 C of parasitic heat while the desktop idles --
  the largest battery and idle-temperature cost on this class of machine. NVIDIA's
  driver documentation names the cause exactly ("the NVIDIA GPU will remain in an
  active state if it is driving a display"), so the fix is the MUX, not a module
  parameter. The new `discrete GPU idle drain` check covers both shapes of the
  failure. Panel on the dGPU: it points at `ryoku-gpu-mux set hybrid` plus a
  reboot. Panel already on the iGPU yet the card has still never slept: it asks
  the driver itself, via `/proc/driver/nvidia/gpus/<slot>/power`, whether runtime
  D3 is on, and splits the remedy accordingly -- the
  `NVreg_DynamicPowerManagement=0x02` module parameter when the feature is off,
  or the real blockers NVIDIA documents (an attached display, a running CUDA
  process) when it is on. Processes holding the card's device nodes are listed as
  leads only, never as a verdict, because fine-grained mode tracks GPU usage and
  tolerates an idle open device; the check must not tell anyone to kill their
  compositor. That second shape exists because a machine that switched the MUX and
  gained nothing would otherwise report clean, the worst outcome: the user believes
  it worked and keeps paying the power. Either shape quotes the card's live draw
  when it can read it and never a made-up wattage when it cannot, and an
  unreadable runtime-D3 status is never treated as a fault. Silent whenever the
  dGPU is suspended now or has spent any time suspended since boot, so a healthy
  box is never nagged. Report-only: it never writes the MUX knob, changes a module
  parameter, kills a process, or reboots
  (`internal/doctor/reconcile_dgpu_panel.go`).

### Fixed
- **`ryoku doctor` re-adds the [ryoku] repo when it falls out of pacman.conf.**
  A packaged box with ryoku-desktop but no `[ryoku]` stanza (a pacnew merge or a
  hand-edit dropped it) got only a warning, so updates silently stopped arriving.
  With the keyring still present, doctor now re-adds the deterministic stanza the
  installer writes and re-populates the key, instead of stranding the box.
- **The Ryoku Canvas spicetify setup defers to spotify-launcher instead of
  warning about a client it cannot patch.** On a converted box that already has a
  root-owned flatpak or /opt Spotify, the setup aims spicetify at the writable
  spotify-launcher tree Ryoku ships; while that launcher is installed but not yet
  launched, the Canvas waits for its first launch rather than telling the user to
  install a client that is already there.
- **`ryoku doctor` can no longer hang behind a second desktop.** Its shell-load
  check runs `qs -c shell` and waits for the report, and `ryoku` is usually run
  from a terminal the desktop opened. A Quickshell instance that has crashed once
  leaves `__QUICKSHELL_CRASH_INFO_FD` in the environment it hands its children,
  and Quickshell reads that before it parses arguments: the check became a full
  second desktop that draws over the first and never exits, with doctor waiting on
  it. `ryoku` now clears Quickshell's crash variables from its own environment at
  startup (`crashenv.go`), so every `qs` it starts honours the config it was given.
- **`ryoku doctor` stopped reporting the spicetify patch as applied without
  checking.** The verdict was reached from three things that say nothing about
  whether Spotify was ever patched: the CLI exists, the extension file matches,
  and the config lists it. On a flatpak client (root-owned `/var/lib/flatpak`) or
  a native `/opt` client, `spicetify apply` fails with `permission denied` on
  `Apps/login.spa` while every one of those three stayed true, so doctor printed a
  green tick over an unpatched client. That is the state a user reports as
  "spicetify is broken", and the tick is worse than a warning because it sends
  them looking somewhere else.

  The client tree is now checked for writability before anything claims to have
  applied, and an unwritable one is reported with the fix: install the shipped
  `spotify-launcher`, whose per-user tree spicetify patches without root. A target
  the probe cannot resolve still reports ok, because a check that cannot see its
  subject must not invent a problem. `TestSpicetifyUnwritableClientIsNotReportedOk`
  reproduces the exact old state and fails if the gate is ever removed
  (`internal/doctor/reconcile_spicetify.go`).
- **`ryoku doctor` stops calling a stray `hyprctl dispatch` a broken config.**
  Hyprland files the Lua error from a bad dispatch in the very buffer
  `hyprctl configerrors` reports, so one typo at the prompt (or a tool probing
  which config mode is live) made the config-integrity check announce "Hyprland
  is rejecting its config" while `hyprctl reload` answered ok and a later
  `configerrors` came back empty: the reload had already cleared it. The check
  now tells the two apart by Lua's chunk name (a dispatch error carries the
  dispatched source, a config error carries the file's path), says so plainly,
  and clears the buffer with a reload instead of sending you to audit config you
  never broke. A genuine config error reads exactly as before
  (`internal/doctor/doctor.go`).
- **`ryoku doctor` gets a black screen back instead of leaving you at one.** A
  single QML file that cannot load takes every surface with it, so the desktop is
  empty at login and after an update, and nothing looked for it. The new
  `desktop loads` check reads the failure out of the shell daemon's surface log,
  scopes the repair to the module the loader blamed, moves an override of yours
  that breaks the desktop aside as `.broken`, puts back the shipped files the live
  tree no longer matches, and restarts the shell. A shipped file at fault is
  reported with `ryoku update` and `ryoku rollback` rather than guessed at
  (`internal/doctor/reconcile_shell_load.go`).
- **`ryoku doctor` explains a black screen instead of passing it.** Quickshell
  links Qt's private API, so a build made against another Qt stops loading:
  `undefined symbol ..., version Qt_6_PRIVATE_API`, and since every Ryoku surface
  is a Quickshell process the desktop comes up black with `ryoku reload` looking
  inert. The repo package is rebuilt with Qt, an AUR one (quickshell-git) is not,
  and it satisfies the dependency through provides, so the breakage arrives with
  an unrelated Qt update. The new `quickshell runtime` check runs the renderer:
  broken plus provided by a foreign package is repaired by taking the repository
  build, broken any other way is reported with what the loader said, a locally
  built QML module at fault is moved aside so the desktop starts, and a foreign
  build that still works is called out before the next Qt update
  (`internal/doctor/reconcile_quickshell.go`).
- **`ryoku doctor` finds and clears a desktop that is running twice.** A leftover
  shell surface kept drawing beside the live one, and nothing looked for it:
  `reconcileShellDaemon` only asks whether the daemon answers its socket, which a
  fresh daemon does while an orphan from the previous one is still on screen. The
  new `duplicate desktop instances` check lists the live Quickshell instances that
  render the shell, keeps the one the supervising daemon started (the unit's
  MainPID, or any live `ryoku-shell daemon` on a checkout, or the oldest when
  every one was orphaned), and stops the rest, escalating to SIGKILL for a wedged
  one. Check-only reports the count and which pid keeps the desktop
  (`internal/doctor/reconcile_shell_instances.go`).
- **Your default apps stop resetting on every update.** Ryoku's default-app map
  was materialized straight into `~/.config/mimeapps.list`, which is the one file
  every "Set as default" writes (GNOME's Open With, a browser making itself
  default, `xdg-mime default`), so each update copied Ryoku's map over the user's
  choices: a box where Firefox had been made default came back opening links in
  whatever else claimed the type. The map now ships to
  `/usr/share/applications/mimeapps.list`, the bottom of the XDG mimeapps chain,
  where it still sets the defaults and always loses to the user's own file.
  `materialize` keeps its hands off the retired path instead of pruning it, so
  the picks on an existing box survive the release that moves it, and
  `reconcileMimeDefaults` finishes the move: entries that are only a copy of
  Ryoku's values are dropped (the file goes if that is all it held), anything the
  user actually chose stays (`internal/doctor/reconcile_mime_defaults.go`,
  `internal/updater/materialize.go`).
- **`ryoku doctor` restores 5 GHz Wi-Fi on a box that never had a regulatory
  domain.** With no country set the kernel stays on the worldwide default `00`,
  which disables or marks no-IR most 5 GHz channels, so only 2.4 GHz networks
  appear. `reconcileWifiRegdom` runs only on a box that has a radio: it reads the
  effective domain (`ryoku-wifi-regdom get`, falling back to the first `country`
  line of `iw reg get`), and when it is `00` sets it from the country in
  `/etc/locale.conf`, or warns to run `ryoku-wifi-regdom set <CC>` when no
  country can be inferred (`internal/doctor/reconcile_wifi_regdom.go`).
- **A WirePlumber drop-in Ryoku stopped shipping is now actually removed.**
  `materialize` pruned only what the previous manifest recorded, but `deploy.sh`
  copies configs without writing that manifest, so on a checkout-deployed box a
  withdrawn drop-in survived every update and `wpConfigPruned` never fired to
  restart WirePlumber. The global ALSA soft-mixer override outlived its removal
  in 280fc656 that way: `api.alsa.soft-mixer=true` stayed live, PipeWire kept its
  hands off the hardware mixer, and playback sat at whatever level the card
  booted with while the desktop showed 100%. `wireplumber.conf.d` is now
  converged against the shipped set, matched on the `NN-ryoku-*.conf` naming so
  the user's own drop-ins in the same directory are left alone
  (`internal/updater/materialize.go`).
- **Your `~/.config/hypr/user.lua` stops getting wiped on every update.** The
  retired "adopt" step copied the tool's own user files (`hypr/user.lua`,
  `hypr/monitors_user.lua`, `kitty/user.conf`) into the `user_edits` overlay,
  and `materialize` then re-laid that frozen copy over the live file on every
  update, silently reverting edits made afterward. Those files are edited in
  place: `overlayUserEdits` now never lays them (`internal/sys/useredits.go`
  `LiveOwnedConfig`), the reconciler no longer adopts them, and it moves any
  stale overlay copy back out without losing data (identical drops, diverged is
  backed up to `<file>.overlay.bak`, a missing live file is restored). The
  overlay guide now says plainly that simple tweaks go in the live file and the
  overlay is only for forking a whole Ryoku file plus the Hub's settings.lua /
  rebinds.lua (`internal/doctor/reconcile_useredits.go`,
  `internal/updater/materialize.go`).
- **`ryoku doctor` installs the NVIDIA login-loop guard hook on existing boxes.**
  A `-dkms` module that fails to rebuild on a kernel update leaves nouveau
  blacklisted with no nvidia module -- no driver binds the card, so the greeter
  draws but Hyprland cannot and SDDM loops the login. `ryoku update` already heals
  that, but a plain `pacman -Syu` never runs doctor; the new
  `reconcileNvidiaGuardHook` writes `/etc/pacman.d/hooks/ryoku-nvidia.hook` (the
  same one `system/hardware/drivers/nvidia.sh` installs) so `ryoku-nvidia-guard`
  runs in that transaction and restores nouveau before the next boot loops
  (`internal/doctor/reconcile_hardware.go`).
- **`ryoku update` no longer wedges on a `ryoku-dns`/`ryoku-wifi-powersave` file
  conflict.** `deploy.sh` seeds those privileged helpers and their polkit rules
  into `/usr/bin` and `/usr/share/polkit-1/rules.d` unowned; once `ryoku-desktop`
  packaged the same paths, `pacman -Syu` aborted the whole transaction ("exists in
  filesystem", no packages upgraded), silently blocking every update until the
  files were deleted by hand. The upgrade now passes `--overwrite` scoped to those
  Ryoku paths so the package adopts them, with a regression test on the glob
  (`internal/updater/update.go`).
- **`ryoku doctor` self-heals a box already wedged by that conflict.** The
  `--overwrite` fix rides in the `ryoku` binary, which a stuck `-Syu` can't
  install, so a new "conflicting Ryoku files" reconciler removes any unowned
  `/usr/bin/ryoku-*` or `*ryoku*.rules` left by a dev deploy or `ryoku recovery`
  (packaged boxes only), letting the next update adopt them. "failed services"
  also now auto-clears the lingering transient GUI app scopes (`app-*.scope`)
  instead of only reporting them (`internal/doctor/doctor.go`).
- **`ryoku doctor` heals an NVIDIA box left without `fbdev=1`.** The config check
  passed on any drop-in containing `nvidia_drm modeset=1`, so an install that had
  modeset but not `fbdev=1` never got it added, and external displays could come
  up wrong or not at all. The check now requires `modeset=1 fbdev=1`, so doctor
  rewrites the drop-in and rebuilds the initramfs
  (`internal/doctor/reconcile_hardware.go`).
- **`ryoku track` no longer leaves a box measuring against the wrong channel.**
  `ryoku status`/`update` read the channel from the live `RYOKU_CHANNEL`, which
  `ryoku track` writes to `environment.d` (loaded only at the next login), so a
  just-switched box measured against the default `main`: it showed updates that
  never cleared, `update` no-oped, and it could redeploy a mismatched tree (the
  `shell.qml: File not found` config load). They now fall back to the persisted
  channel (new `sys.TrackedChannel`), and a new `reconcileUpdateChannel` doctor
  step puts the update checkout back on the tracked branch, so status, update,
  and the checkout agree without a relogin (`internal/updater/channel.go`,
  `internal/sys/repo.go`, `internal/doctor/reconcile_channel.go`).

### Added
- **`ryoku doctor --report` records the desktop stack's package versions.**
  `ryoku status` shows only the ryoku-desktop channel commit, so a report never
  carried the quickshell, Qt, compositor, audio, and GPU versions that decide
  whether the shell renders. The report's packages section now lists `pacman -Q`
  for the Ryoku components and those key dependencies
  (`internal/doctor/report.go`).
- **Safer updates: one at a time, sleep-inhibited, space-checked, fewer snapshots.**
  `ryoku update` now holds an flock so a second run cannot race it, refuses to
  start when `/` has under 1 GiB free (a mid-update disk-full leaves the system
  half-upgraded), and runs pacman/yay under `systemd-inhibit` so a lid-close or
  idle suspend cannot corrupt a transaction. It also skips snap-pac's redundant
  per-transaction snapshot (the run is already bracketed by one snapper pre/post
  pair, so a packaged update drops from about four snapshots to two) and labels
  that snapshot with the version it updated from, so boot-menu rollback entries
  are distinguishable. Adapted from omarchy's update pipeline
  (`internal/updater/update.go`).
- **Dropping a shipped WirePlumber config takes effect on update, not next login.**
  `materialize` now restarts `wireplumber.service` whenever a `wireplumber/`
  drop-in is pruned, not only when the bluetooth policy changes, so removing the
  global ALSA soft-mixer override applies immediately
  (`internal/updater/materialize.go`).
- **The Spotify Canvas wires up against a per-user Spotify.** `reconcileSpicetifyCanvas`
  now points spicetify at a `spotify-launcher` install (per-user, under
  `$XDG_DATA_HOME`), so `spicetify apply` needs no root chmod of `/opt/spotify`
  (`internal/doctor/reconcile_spicetify.go`).
- **Existing boxes land on the QS Bar default on update.** `ryoku doctor` no
  longer strips the live top-level `barStyle` (it was wrongly listed as a retired
  key, so an update stripped it and reseeded `sumi`, flipping every QS Bar user to
  the minimal Sumi rail); `reconcileSumiBar` now defaults an absent or retired
  Atoll-era bar style to `qsbar`, while an explicit sumi or installed store style
  is kept (`internal/doctor/doctor.go`).
- **Spotify Canvas is wired up for a Spotify user automatically.** A new
  `reconcileSpicetifyCanvas` drops the bundled `ryoku-canvas.js` into a Spotify
  user's spicetify Extensions, enables it, and applies it (building `spicetify-cli`
  from the AUR if missing). Gated on Spotify being installed and best-effort, so it
  never blocks an update and stays inert for anyone without Spotify
  (`internal/doctor/reconcile_spicetify.go`).
- **`ryoku track <main|unstable-dev>` switches the update channel.** It points a
  box at a channel and makes `ryoku update` follow it: installs the build tools,
  checks out the branch, records the channel, and rebuilds the desktop from the
  checkout, keeping your user_edits overlay and Hub settings (deploy re-applies
  them). It mirrors `ryoku recovery`'s handoff, running `bin/ryoku-track` from a
  local checkout or fetching the canonical copy, so a packaged install with no
  checkout can still hop onto the source-tracked edge and back. `ryoku recovery`
  stays the heavy factory reset to stable main (`track.go`, `main.go`,
  `bin/ryoku-track`).

### Fixed
- **Materialize activates WirePlumber policy changes immediately.** It compares
  the effective Bluetooth fragment before and after the base plus user overlay,
  then try-restarts WirePlumber only when those bytes changed. Updated installs
  get the A2DP default in the current session without interrupting audio on
  ordinary updates (`internal/updater/materialize.go`).
- **Doctor restarts a shell daemon left running on a replaced binary.** Updaters
  before beta-17 swapped the packages without quiescing the shell, so the old
  daemon kept serving surfaces that hot-reload QML newer than it can host
  ("module Ryoku.FrameBars is not installed" after updating to beta-18). Doctor
  now spots a daemon whose /proc exe link reads deleted and restarts it on the
  installed binary, making `ryoku doctor` the one-command cure
  (`internal/doctor/doctor.go`).
- **Doctor can revive a shell skipped by a missing session env.** The
  ryoku-shell unit is gated on `ConditionEnvironment=WAYLAND_DISPLAY`; when
  login's env import never reached the systemd user manager, `systemctl
  restart` reported success while starting nothing, so doctor's shell-daemon
  reconciler pushed a restart into a void. It now imports the live session's
  env into the user manager first (`internal/doctor/doctor.go`).
- **Doctor moves a persisted Stash sidebar to the right.** The Stash board is now
  the floating Features page on the right, but a box that persisted frameBars
  still carried `surfaces.stash.anchor: "left"` (the old full-span default), which
  normalize keeps, so the page would grow from the wrong edge. Doctor flips that
  one leaf to `right` in place, leaving every other key untouched
  (`internal/doctor/reconcile_stash_sidebar.go`, `internal/doctor/doctor.go`).
- **Doctor installs the missing in-session lockscreen.** Only the ISO installer
  ever laid down the qylock lock, so a box that predates the step (or where it
  failed) had a dead lock button and suspended without locking, silently:
  hypridle's before_sleep runs the same command. ryoku-desktop ships the bundle
  now and doctor heals the user-side install, never touching the greeter half or
  a theme the user picked (`internal/doctor/reconcile_lockscreen.go`,
  `release/packages/ryoku-desktop/PKGBUILD`, `ryoku/lockscreen/install-qylock`).
- **The shell daemon's systemd unit reaches package installs.** The shipped
  autostart now starts the unit, but only the dev deploy laid it down:
  ryoku-desktop shipped the session target alone, so a package user's next
  login had no unit to start and no shell. The package ships the whole
  systemd/user tree now, and the updater, doctor, and deploy all stop and
  start the daemon through the unit where it exists (with a daemon-reload so
  a just-delivered unit is found), falling back to the bare start where it
  does not (`release/packages/ryoku-desktop/PKGBUILD`,
  `internal/updater/update.go`, `internal/doctor/doctor.go`).

### Added
- **`ryoku keyboard`: the layout on every screen that asks for one.** `status`
  shows what the desktop, greeter, console and boot image each use and whether
  they agree, `detect` reports the layout this system records and where it came
  from, and `apply` puts one on the greeter and the console AND rebuilds the
  boot image, which localectl alone cannot do (`internal/keyboard/`).
- **Doctor adopts the keyboard the installer was told about.** A keyboard cannot
  report the legends printed on its keys, so a box installed with an AZERTY
  keymap still came up on QWERTY. The layout is now read back from what the
  system already records, strongest source first: the X11 keymap, then
  /etc/vconsole.conf, then the locale's country. It is adopted only while the
  desktop is on the untouched shipped default, once, and a marker keeps a later
  deliberate pick from being undone (`internal/doctor/keyboard_detect.go`,
  `internal/doctor/reconcile_keymap.go`).

### Changed
- **`ryoku doctor` follows the trimmed frame-bar catalogue.** Its `frameBars`
  normalizer mirrors the shell's catalogue, which dropped App Launcher,
  Clipboard, Layout Switcher, Color Picker, Power Profile, Reboot, Screenshot and
  Wallpaper as bar widgets: doctor now strips any of the eight from a saved rail
  and drops `reboot` from the legacy-migration default, while `music` -- a bar
  widget the allowlist had never carried -- is preserved instead of being
  silently stripped (`internal/doctor/doctor.go`).

### Added
- **doctor keeps every recording in one directory.** Clips were landing in three
  places: the shell's own, Ryoku Motion's Electron userData directory, and an
  empty `~/Videos/Ryoku Motion` a deleted prototype left behind. Ryoku Motion
  hardcodes its path with nothing to redirect it, so its directory becomes a link
  into the real one and whatever it already recorded moves across, never
  overwriting a file already there. gpu-screen-recorder's own default is reported
  with the command to merge it, not migrated, since it may be running
  deliberately (`internal/doctor/reconcile_recordings.go`).

### Fixed
- **`ryoku doctor` carries existing launcher blur settings into the local-frost
  design without taking ownership from the user.** Before a one-time marker, an
  exact saved `bgBlur: 12`, the retired shipped global-blur default, moves to the
  new 2 px card-local frost default. Every other numeric value, a missing config,
  or a missing key is preserved and marked complete; malformed/non-numeric JSON
  is warned about and never marked. `--check` reports the exact migration
  without writing either file. Once marked, even a deliberate 12 stays 12
  (`internal/doctor/reconcile_launcher.go`,
  `TestReconcileLauncherLocalFrost*`).
- **`ryoku doctor` heals a shell daemon left on a dead Hyprland instance.** A
  daemon that outlived its compositor (a relogin or crash brings up a new
  Hyprland; the daemon runs detached and kept the old signature) still answers
  ping, so the daemon check passed it as healthy while workspaces stayed frozen
  and the power menu and every monitor-aware keybind did nothing. The check now
  compares the daemon's Hyprland instance (a new `ryoku-shell signature` command)
  against the live session and, on a mismatch, restarts the daemon against the
  live compositor -- quit it (it reaps its own quickshell children), then start a
  fresh one bound to this session. A daemon it cannot identify (an older binary)
  or one already on the live instance is left alone (`internal/doctor/doctor.go`
  `reconcileShellDaemon`/`daemonIsStale`, covered by `TestDaemonIsStale`,
  `TestReconcileShellDaemonStale`).
- **`ryoku doctor` stops nagging about the `.pacnew` files it creates itself.** The
  `.pacnew` check is now a reconciler: it auto-clears provably-safe pending configs
  -- a `.pacnew` byte-identical to the live file, and `pacman.conf` whose live copy
  differs only by Ryoku's appended `[ryoku]` repo stanza -- and reports only genuine
  conflicts for `pacdiff`. It never overwrites a user-modified config.

### Added
- **The keyring never prompts out of the box.** The default policy is now
  `never-ask` (a blank, passwordless default keyring) instead of `ask`: an
  unconfigured box with no PAM wiring infers `never-ask`, and a new
  `ryoku keyring init` (run from the Hyprland autostart at first login) records
  the mode and seeds the blank keyring, so no libsecret app (browser, editor, SSH
  agent) ever asks for a keyring password, and it persists across reboots. `init`
  is idempotent (a no-op once a mode is chosen) and non-destructive (a
  pre-existing password-protected keyring is left intact; it records the policy
  and points at `set never-ask --reset`). `internal/keyring/init.go`,
  `TestInit*`, `TestStatusModeInference`.
- **`ryoku keyring` chooses how the GNOME keyring unlocks at sign-in.** Three
  modes: `unlock-on-login` (PAM unlocks the login keyring with the login password
  at sign-in, encrypted at rest, silent), `never-ask` (a blank plaintext default
  keyring that never prompts, the only silent option under SDDM autologin), and
  `ask` (locked until an app asks). `status [--json]` reports the configured (or
  inferred) mode, the PAM state, autologin, the daemon, and each keyring file's
  encrypted/plaintext/absent format for the Hub; `set <mode> [--convert|--reset]
  [--password-stdin]` runs the user-side state machine over the keyring files via
  gnome-keyring's D-Bus interface (godbus, vendored) and escalates the privileged
  PAM half through `pkexec`; `apply-pam <mode>` idempotently wires or strips
  `pam_gnome_keyring` in `/etc/pam.d/sddm`. `$RYOKU_PAM_FILE` overrides the PAM
  path so tests and the sandbox never touch the real stack
  (`internal/keyring/`, `TestApplyPAMText*`, `TestSet*`, `TestE2EKeyringLifecycle`).
- **`doctor` watches the keyring policy for drift.** `keyring unlock policy`
  records the inferred mode when none is set and points the default keyring at
  `login` for unlock-on-login (safe, user-side); it only warns -- never edits the
  root file or rekeys a keyring -- when `/etc/pam.d/sddm` disagrees with the mode
  (with the exact `sudo ryoku keyring apply-pam <mode>` fix), when autologin
  conflicts with unlock-on-login, or when never-ask still finds an encrypted
  keyring (`internal/doctor/reconcile_keyring.go`, `TestReconcileKeyring*`).
- **`materialize` overlays `~/.config/ryoku/user_edits` after the base.** A
  regular file in the overlay wins at its mirrored `~/.config` path; the base is
  laid in full first, so fixes and new files still land, and an empty overlay is a
  no-op. Symlinks and `.md` notes are skipped (`internal/updater/materialize.go`,
  `internal/sys/useredits.go`, `TestMaterializeUserEditsOverlay`).
- **`ryoku reset [path]`.** Drops a `user_edits` override (or, with no path and a
  confirm, the whole overlay) and re-lays the base, so a customization returns to
  the Ryoku default (`internal/updater/reset.go`).
- **`doctor` sets up the overlay.** `user edits overlay` seeds a how-to
  `README.md` and adopts a machine's legacy loose files (`hypr/user.lua`,
  `hypr/monitors_user.lua`, `kitty/user.conf`) into it
  (`internal/doctor/reconcile_useredits.go`, `TestReconcileUserEditsAdopt`).
- **`doctor` seeds the decor art into `~/Pictures/ryodecors`.** The `Decor` and
  `Placard` components render their baked art from that folder (beside
  `Wallpapers` and `livewalls`); the installer seeds a fresh box, and this
  reconciler delivers the shipped set -- and anything a later release adds -- to a
  box that updated before it shipped or lost a file. Missing-only, so a swapped or
  added file is left alone and nothing is pruned (`internal/doctor/doctor.go`,
  `TestReconcileRyodecors`).
- **`doctor` heals the looping limine boot countdown.** On the
  limine-mkinitcpio-hook layout the OS entry is a directory and the kernel is a
  `//` sub-entry, but `default_entry` was a bare `2`, which Limine resolves as
  the second top-level entry (the `/EFI fallback`, which chainloads Limine), so
  the countdown looped forever and the user had to pick the kernel by hand. A
  new `limine autoboot` reconciler repoints `default_entry` at the kernel's
  entry path and enables `remember_last_entry`, so existing installs autoboot
  the last kernel used on the next `ryoku update` and fresh installs match
  (`internal/doctor/reconcile_limine.go`, `TestLimineEnsureAutoboot`).
- **`doctor` keeps the desktop brand off a broken logo image.** brand.json's
  `markImage` override (Ryoku Settings, Shell, Global) wins over the text seal
  everywhere in system chrome, but a moved or unreadable image leaves every
  branded surface empty. A new reconciler clears a dangling `markImage` back to
  the text seal, preserving the chosen name and tint; a no-op when the file is
  absent, the image is unset, or it resolves (`internal/doctor/doctor.go`,
  covered by `TestReconcileBrandLogo`).
- **`doctor` clears a crashed update's stuck progress.** A `ryoku update` that
  dies mid-run (power loss, OOM, a kill) leaves the run-state file in
  "running", so the shell's update island and the Hub keep rendering a phantom
  update for the rest of the session. A new reconciler idles a running/prompt
  run-state with no live `ryoku update` process behind it
  (`internal/doctor/doctor.go`, covered by `TestReconcileStaleUpdateRun`).
- **`doctor` prunes an orphaned `theme.lua`.** Removing the Appearance Themes
  feature left a `~/.config/hypr/theme.lua` on boxes that had a theme applied, and
  `hyprland.lua` no longer loads it. A new reconciler removes the dead file so the
  config dir matches the shipped layout (`internal/doctor/doctor.go`).
- **`ryoku recovery` restores the `awww` wallpaper daemon and `wallust` palette
  generator.** Both now ship from the `[ryoku]` repo as hard `ryoku-desktop`
  dependencies, so a fresh install and `ryoku update` (pacman) already carry them.
  Recovery now also ensures them (`pacman -S --needed awww wallust` on a box with
  `[ryoku]` configured, gated on the package step so `--no-packages` skips it), so
  the panic button puts back a daemon or generator an old broken AUR build had
  dropped and the wallpaper and its colors come back.
- **`ryoku update` shows real, determinate progress.** The run-state the update
  island and the Hub's Updates page watch now carries the update's ordered
  stages (snapshot, packages, AUR, apply, reload, doctor, finalize), each with
  its own state, the current step's human label, and a live log tail, written
  atomically (temp + rename) so a watcher never reads a half-written file. The
  Hub renders a determinate multi-segment bar and streams the log instead of the
  old fixed progress "wave". On failure the run-state names the step that broke
  and carries the pre-update snapshot id, so the Hub can offer a one-click
  rollback.
- **`ryoku doctor --json`** emits the reconciler findings as a JSON array
  (name, status, detail, remedy): the read-only data seam a GUI System Check can
  render without parsing the human output.
- **`ryoku update` hands itself to the freshly installed binary.** The whole
  update used to run inside the old release's binary, so every fix to
  materialize or the restart flow shipped one release late (the beta-16
  breakage was the old updater deploying the new desktop with old semantics).
  After the pacman step the updater now re-execs `/usr/bin/ryoku update
  --stage2`, so the release being installed also runs its own deploy and
  doctor. If the exec fails it finishes in-process exactly as before.
- **The shell is quiesced while configs swap.** Materialize used to rewrite
  `~/.config/quickshell` under a running quickshell, which hot-reloaded the
  half-copied tree against whatever plugin the old process still had mapped,
  a recipe for glitched and ballooned surfaces right after an update. Stage 2
  now stops the shell daemon (and reaps orphaned `qs` components using the
  real component list; the old one named a `sidebar` that never existed and
  missed `launcher`/`widgets`) before materialize, and starts it after.
- **Rollback snapshots finally appear in the Limine boot menu.** A new
  `limine UKI boot tree` reconciler converges boxes stuck on the flat
  install-time placeholder entry: limine-snapper-sync refuses to hang the
  Snapshots submenu under an entry with no kernel sub-entries, so those boxes
  never showed a rollback at boot no matter how many snapshots snapper kept
  (the design always shipped `ENABLE_UKI=yes` and listed
  limine-mkinitcpio-hook in the AUR set; omarchy works because its installer
  hard-requires that hook). Doctor now installs the hook, whose deploy
  rebuilds the menu as the `/+Ryoku` UKI tree, drops the flat placeholder the
  same way the installer's finalize does, and runs one sync so the snapshots
  show up immediately.
- **New doctor reconcilers for the beta-16 fallout.** `Material Symbols icon
  font` installs the font on boxes that predate it being a package dependency
  (every glyph rendered as its ligature name). `stale dev residue` removes
  home-deployed binaries and QML modules that shadow the packaged install on
  pacman-channel boxes (one old recovery run used to pin the desktop at that
  vintage forever). `shell config schema` migrates a pre-rework
  `~/.config/ryoku/shell.json`: drops the retired island knobs, revives the
  bar they pointed at, and clamps out-of-range frame geometry.
- `ryoku doctor` installs the wallpaper backends (`awww` + `mpvpaper`) when a
  desktop lacks them, instead of only printing the command. Both are AUR, so
  `ryoku update` (pacman) never pulls them: a box that predates them can't set a
  wallpaper, and without mpvpaper a live pick only shows a still frame. `--check`
  just reports what it would add, and the doctor pass at the end of every
  `ryoku update` heals it automatically.
- `ryoku update` offers to install the snapshot helpers when they are missing
  (`snap-pac`, and `limine-snapper-sync` on a Limine system) rather than leaving
  them as a standing `doctor` recommendation. It asks first: a Hub-launched update
  (`RYOKU_UPDATE_UI=hub`) raises the question in the Hub's Updates page and waits
  for the answer; a terminal update asks y/N; a non-interactive run declines. Skip
  or no answer leaves them to `ryoku doctor`, and a failed install never aborts the
  update. The consent rides the existing run-state file (a `prompt` phase plus a
  one-line answer back-channel). Standalone `ryoku doctor` stays recommend-only.
- **Doctor unhijacks the desktop portal routing.** A box migrated from another
  compositor can carry a leftover `~/.config/xdg-desktop-portal/portals.conf`
  (or an `/etc` one), and the portals.conf(5) lookup lets that generic file
  outrank the packaged `hyprland-portals.conf`, so xdg-desktop-portal keeps
  loading the old desktop's backend. With `xdg-desktop-portal-gnome` installed
  (niri's own docs require it) that backend hangs under Hyprland, and every
  app that touches the portal bus at startup (GTK apps read the settings
  portal first thing) waits out a ~25s D-Bus timeout before its window shows:
  "apps are slow to open". Screenshare picks the wrong backend the same way.
  A new `desktop portal routing` reconciler resolves the winning config
  exactly like the portal does, moves every misrouted file aside (kept as
  `.ryoku-bak`), and restarts the portal services. The shell installer has
  moved the user-level file aside since early July; this heals the boxes
  converted before that, and the `/etc` case the installer never handled.

### Changed
- **`doctor`'s AUR wallpaper reconciler is removed.** `awww` now ships from the
  `[ryoku]` repo as a hard `ryoku-desktop` dependency (see the release changelog),
  so `pacman -Syu` guarantees the image daemon on every install and update; the
  reconciler that installed `awww-git` from the AUR (and, before that, phonto and
  mpvpaper) is redundant and dropped, matching `wallust`, which has none
  (`internal/doctor/doctor.go`).
- **The CLI is split into focused packages.** The one-package `ryoku` program is
  now a thin dispatcher over `internal/updater` (update, status, rollback,
  channel, run-state, materialize, version), `internal/doctor` (the convergent
  reconcilers, report, and `--explain`), and `internal/sys` (the shared exec,
  package, filesystem, path, and terminal primitives, defined once). `doctor.go`
  no longer holds every reconciler: the limine, hardware, and diagnostic-report
  concerns move to their own files. Behaviour and the command surface (`ryoku
  update`/`doctor`/`status`/...) are unchanged.

### Fixed
- **`ryoku update` on a dev checkout clears "behind" instead of nagging
  forever.** The update fast-forwarded the checkout onto origin/<channel> only
  when the current branch was literally named after the channel (`main`); a dev
  box on `unstable-dev` (or any other branch) skipped the fast-forward and just
  redeployed the same commit, so `deploy.sh` re-recorded the unchanged deployed
  commit and `ryoku status` kept reporting the same commits behind after every
  update. `ryoku update` now fast-forwards any clean checkout that is strictly
  behind the channel onto it, whatever the branch is named (always lossless), so
  updating actually advances the box and the count reaches zero. A branch with
  its own commits (a maintainer mid-dev) or a dirty tree is left untouched and
  redeployed as-is; only the channel branch is still reset onto upstream when it
  has diverged (new `syncChannel`/`isAncestor` in `internal/updater/channel.go`,
  covered by `TestSyncChannel*` and `TestUpdateClearsBehindOnDevBranch`).
- **The Hub's Updates section shows real commit messages on a packaged install,
  not bare package names.** `ryoku status` surfaced the channel's incoming commit
  subjects only on a dev checkout; a packaged box (every ISO or shell install) has
  no checkout, so it fell back to the pacman view and the Hub listed pending
  package names ("ryoku-desktop") under the "INCOMING COMMITS" header, with "N
  commits behind" counting all pending packages. The packaged path now reads the
  running and available commits from the installed and `[ryoku]`-repo
  `ryoku-desktop` versions and lists the commits between them via the public
  GitHub compare API, so a user sees the same commit list a dev box does. The
  lookup is cached by the installed..latest sha pair (a polled status fetches once
  per release) and best-effort: offline or rate-limited, it degrades to a single
  `ryoku-desktop` version-bump row and never hangs the status query. New
  `internal/updater/commits.go` (covered by `commits_test.go`),
  `internal/updater/update.go`, `internal/updater/channel.go`; `RYOKU_REPO_SLUG`
  overrides the repo for a fork.
- **`ryoku update` can no longer run stale home-deployed binaries over the
  freshly materialized configs.** Stage2 resolved `ryoku-shell` (and `ryoku`
  for the doctor step, and `ryoku-rashin`) on PATH, where a past `ryoku
  recovery` or dev deploy in `~/.local/bin` outranks `/usr/bin`, so the update
  quiesced and then relaunched the OLD daemon against the new QML tree -- on
  beta 16 -> 17 that replayed the retired resident wallpaper switcher as an
  endless reopen loop (the stale supervisor respawned the new one-shot
  switcher every time it quit) -- and ran the OLD doctor, whose reconcilers
  predate the release doing the healing. Every self-invocation now prefers the
  packaged `/usr/bin` binary (`pkgBin`; PATH only on package-less checkouts).
  stopShell also quiesces the previously missed `overview` component plus the
  retired `plugins`/`wallpaper` residents (patterns anchored so a user's own
  `qs -c wallpaper...`-named config never matches), and kills the video
  players (`ryoku-livewall`, plus legacy `mpvpaper`/`phonto`) so no orphan
  from the old release survives the swap (`internal/updater/update.go`).
- **`doctor` clears every home-deployed binary shadowing a packaged one, not
  just four.** The dev-residue reconciler's fixed name list missed
  `ryoku-livewall`, the hardware helpers, and the app bins deploy.sh installs,
  leaving a stale player and stale tools pinning every later update. It now
  scans `~/.local/bin` and treats any entry whose `/usr/bin` twin is owned by
  a `ryoku*` package as residue; files the packages never shipped are
  untouched, and paths that fail to delete are reported instead of silently
  claimed removed. Doctor's shell-daemon restart also prefers the packaged
  binary on packaged boxes (`internal/doctor/doctor.go`).
- **`rollback` no longer runs a `snapper rollback` that cannot restore the
  system.** Ryoku pins the root subvolume on the kernel cmdline and in fstab
  (`rootflags=subvol=@`), and snapper's rollback works by flipping the btrfs
  default subvolume, which a pinned `subvol=` ignores; limine-snapper-sync's
  own tooling states the layout is "not compatible with 'snapper rollback'".
  So `ryoku rollback <id>` either failed with a cryptic snapper error or
  flipped a default subvolume nothing reads, while the Hub's one-click "Roll
  back" after a failed update inherited the same dead end. The command now
  teaches the supported flow: reboot into the snapshot from the Limine
  Snapshots menu and run `sudo limine-snapper-restore` there (it restores the
  booted snapshot with its matching kernels from the ESP); with no id it still
  lists the snapshots first, and it points at the sync package when the boot
  menu has no snapshots (`internal/updater/update.go`, `docs/cli.md`).
- **`doctor` respects an install-time "no snapshots" choice.** The snapper
  reconciler converges every btrfs root missing the snapper config onto the
  canonical layout, which silently re-enabled snapshots (creating the
  /.snapshots subvolume and config) on the first update after a user declined
  them in the installer. The installer now records the opt-out as
  `/etc/ryoku/snapshots-disabled`, and the reconciler reads it: marker present
  and no config, snapshots stay off with an explanatory ok (delete the marker
  and run `ryoku doctor` to enable); an existing config always wins over a
  stale marker. Installs that predate the marker keep the old converging
  behavior (`internal/doctor/doctor.go`, covered in `TestPlanSnapper`).
- `update` no longer points a failed `pacman -Syu` at `ryoku rollback` when the
  pre-update snapshot it needs was skipped. Snapshots are best-effort (no
  snapper, no root config), and `snapperPre` then returns empty; the failure
  message still advertised a rollback that had nothing to restore. The hint now
  names the actual pre snapshot when one exists, and says to recover with
  pacman directly when none does (`internal/updater/update.go`).
- **`materialize` guarantees `~/.config/ryoku` exists.** The shell's JSON
  stores (shell.json, launcher.json, hypr.json) live there, but the package
  ships no file under it and the shell's QML self-seed cannot create parent
  directories, so on a box where nothing had written a setting yet the seeds
  failed silently. Materialize now creates the directory at install and on
  every update (`internal/updater/materialize.go`).
- **A stale pacman lock no longer fails `ryoku update`.** A `db.lck` left by a
  crashed pacman made `pacman -Syu` abort, and the fix (doctor's stale-lock
  repair) only ran later in the same update it had just failed. The updater now
  runs that repair right before `pacman -Syu`: an in-use lock (a pacman
  actually running) is left alone, a stateless leftover is removed
  (`internal/updater/update.go`).
- **Doctor heals the boot-menu countdown loop.** On boxes where
  limine-mkinitcpio-hook 1.37+ adopted the `/Ryoku Linux` placeholder as the
  menu directory, the flat placeholder's boot stanza
  (`protocol`/`kernel_path`/`cmdline`/`module_path`) stayed wedged under the
  directory title, where Limine allows only a `comment`. A directory that is
  also a boot entry cannot autoboot: `default_entry` resolved to nothing
  bootable and the timeout restarted forever until an entry was selected by
  hand. The `limine boot menu layout` reconciler now recognises the adopted
  tree (not just the standalone `/+Ryoku` shape) and strips that stanza,
  leaving a clean directory that autoboots.
- **Materialize converges `~/.config/quickshell` against the shipped tree.**
  Pruning used to rely entirely on the recorded manifest, so a box whose state
  file was missing or stale (a lost state dir, an old `deploy.sh` or recovery
  run) kept every QML file a release had dropped, sitting live next to the new
  tree forever. The quickshell dir is wholly Ryoku-owned, so materialize now
  sweeps anything there the package does not ship; other dirs are mixed with
  user files and stay manifest-pruned.
- **`ryoku doctor --check` no longer edits the Hyprland config.** The
  follow-mouse check shelled out to `ryoku-hub hypr get`, which rewrites
  `settings.lua`/`theme.lua` as a side effect, breaking the read-only contract
  of `--check`/`--report`. The check now reads `~/.config/ryoku/hypr.json`
  directly; only the fix path goes through the hub.
- **`ryoku recovery` keeps packaged boxes on the pacman channel.** The rescue
  deploys from a fresh checkout, and that deploy recorded the checkout as the
  update channel: a packaged box that ran recovery silently stopped getting
  `pacman -Syu` through `ryoku update` and tracked raw main tip forever. On a
  box where `ryoku-desktop` is installed, recovery now clears the channel
  record after deploying, and the next update's doctor removes the leftover
  home artifacts once the packages are current.
- **`ryoku doctor` heals the update breakage on users' machines.** A new `limine
  snapshot sync` reconciler aligns `TARGET_OS_NAME` in `/etc/default/limine` with
  the actual Ryoku boot entry name, so `limine-snapper-sync` finds it,
  `snapper-cleanup.service` stops failing on every run, and the boot menu's
  rollback Snapshots submenu syncs again. It reads the real entry name (the
  `/+Ryoku` UKI tree is "Ryoku", a flat fallback is "Ryoku Linux") and converges
  to it, so a healthy box is a no-op and a healthy name is never re-pointed. A new
  `wallpaper daemons` reconciler flags a box missing `awww`/`swww` or `mpvpaper`
  and points at the one-shot `ryoku-pkg-aur-add`, so ryowalls' image and Live tabs
  come back.
- **`ryoku doctor` stops nagging every update about things it cannot fix.**
  Orphaned packages and the hybrid-GPU backlight advisory are now `note`s: shown
  in `ryoku doctor --verbose` and the shared report, silent on a plain run and
  inside `ryoku update`, so a healthy box's update ends quiet. A new `--verbose`
  (`-v`) lists the passing checks and the notes.
- `ryoku update` no longer resets the terminal palette to the shipped default.
  wallust writes the wallpaper-derived colours to `kitty/current-theme.conf`, but
  `materialize` reclobbers every shipped config on each update, so kitty snapped
  back to the "Ryoku dark" seed until you reapplied the wallpaper. it is now a
  seed like the fastfetch readout: laid down once on a fresh install, then left to
  whatever wallust last wrote. the shell widgets and window borders never had this
  problem, they read the palette from `~/.cache/wallust`, which the update leaves
  alone.
- `doctor` restores follow-mouse to the intended default on boxes seeded before
  it changed. The hub default moved from 1 ("Normal", keyboard focus chases the
  cursor) to 2 ("Loose", focus detached from the pointer), but an existing
  `~/.config/ryoku/hypr.json` kept the old 1 baked in, so the generated
  settings.lua pinned `follow_mouse = 1` over the base module's 2 and keyboard
  focus followed the mouse (the "cursor issue" seen around the launcher and pill).
  A one-time reconciler bumps a still-default 1 to 2 and regenerates settings.lua,
  then records a marker so re-picking "Normal" in Settings afterward is left alone.
- `ryoku update` no longer overwrites a customized fastfetch readout. `materialize`
  clobbers every shipped config on each update by design; a user's own edits are
  meant to live in a separate override file it never touches (kitty `user.conf`,
  hypr `user.lua`). fastfetch reads a single config with no include, so editing
  `fastfetch/config.jsonc` directly was the only way to customize the readout, and
  the update wiped it out every time. it is now a seed (like `hypr/keyboard.lua`):
  laid down once on a fresh install, never clobbered after. the emblem it draws
  stays managed so the logo keeps updating, and `doctor` still restores it, so the
  Arch-logo fallback does not come back.
- `doctor` no longer deletes a machine's only UEFI boot entry, and restores one
  that already went missing (the "after a `ryoku update` the boot option is gone,
  not even in the BIOS" report). The "limine boot menu layout" migration retired
  the legacy hand-copied `\EFI\limine\limine.efi` NVRAM entry whenever
  `limine_x64.efi` existed, but on a box without `limine-install` it wrote that
  binary and then removed the entry with nothing registered in its place, so the
  machine dropped off the firmware boot menu entirely and could not boot. The
  migration now retires the old entry only once a replacement is present:
  `limine-install` when it exists, else the installer's own
  `efibootmgr --create ... --loader \EFI\limine\limine_x64.efi`, and it leaves
  the working legacy entry alone if neither can. A new "limine boot entry"
  reconciler re-registers a vanished entry on boxes that still start (via the
  removable EFI/BOOT fallback); it recognizes both limine-install's "Limine"
  entry (a VenHw device path, no file path) and the installer's "Ryoku" entry, so
  it never false-fires on a healthy machine. Covered by `doctor_test.go`.
- `doctor` gains a "fastfetch readout emblem" reconciler. The branded terminal
  readout draws a `kitty-direct` logo from an image file; when that file is
  missing fastfetch silently drops to its built-in Arch logo (empty stderr), so
  the terminal greeted with Arch instead of the Ryoku emblem. A redraw renamed
  the emblem and `config.jsonc` now sources it from `~/.config/fastfetch/` (laid
  beside the config by `ryoku materialize`), but a box that updated before that
  shipped points at an emblem it never received. The reconciler restores it from
  the packaged base config tree, the same file materialize lays; it leaves a
  user-customized logo alone, warns to run `ryoku update` on a box whose package
  predates the shipped emblem, and stays idempotent and quiet on a healthy box.
  Covered by `doctor_test.go`.
- `doctor` gains a "limine boot menu layout" reconciler. Earlier installers
  wrote the branded config to `/boot/limine/limine.conf`, a location Limine
  scans BEFORE `/boot/limine.conf` (the only file `limine-entry-tool`
  manages), so the generated UKI entries and the snapper Snapshots submenu
  were shadowed forever: the boot menu stayed frozen at its install-time
  shape. They also hand-copied the bootloader to `EFI/limine/limine.efi`, a
  path no pacman hook refreshes, so the booted binary silently aged while the
  `limine` package moved on (stale, off-looking menu rendering). The
  reconciler merges the shadow's branding into `/boot/limine.conf` (keeping
  every generated entry, foreign entry, and non-branding global), removes the
  shadow, repoints `default_entry` past the `/+Ryoku` directory at the newest
  UKI, re-deploys the binary onto the tool-refreshed
  `EFI/limine/limine_x64.efi` via `limine-install`, retires the stale NVRAM
  entry, and re-syncs the Windows chainload entry. Config first, binary
  second: the box stays bootable at every interruption point. Covered by
  `doctor_test.go`.
- `doctor` now checks that `limine-snapper-sync.service` is *enabled*, not just
  installed: the package alone never syncs a snapshot into the boot menu.
- `doctor` gains an "SDDM greeter theme" reconciler. Picking a lockscreen skin in
  Ryoku Settings copies it into the SDDM greeter dir; a catalogue skin downloaded
  into a 0700 user-owned temp dir was copied verbatim, so the unprivileged `sddm`
  greeter could not read `/usr/share/sddm/themes/ryoku` and SDDM fell back to its
  embedded theme on every boot. The reconciler normalizes that one fixed dir to
  root-owned and world-readable (`a+rX`) when it has drifted, healing boxes that
  picked a skin before the `ryoku-hub` fix. Idempotent and quiet on a healthy box.
  Covered by `doctor_test.go`.
- `ryoku update` reconciles a checkout that diverged from its channel instead of
  dead-ending. A box that ever deployed `unstable-dev` sits on commits
  `origin/main` lacks, so the fast-forward failed and the Hub kept showing the
  same commits as pending after every "Update now". On the channel branch with a
  clean tree it now resets onto `origin/<channel>` (which mirrors upstream and
  holds no local work to keep) and redeploys. Covered by `channel_test.go`.
- `doctor` gains a "stale install crypt mapper" reconciler. ryoku-install opens
  the encrypted root as `/dev/mapper/root`; a run that died after the open, or a
  retry in the same live session, left that name held, so the next
  `cryptsetup open ... root` aborted with "Device root already exists" and a
  reinstall could not proceed. The reconciler closes a `root` crypt node only
  when it is a true orphan (present, not the device backing `/`, and holding no
  mount anywhere), so a normal encrypted box, where `/dev/mapper/root` IS the
  running root, is never touched. The installer self-heals too (see
  `installation/backend`). Covered by `doctor_test.go`.
- `materialize` now seeds `hypr/keyboard.lua` the way it seeds `monitors.lua` and
  `gpu.lua`: laid down only on a fresh install, never clobbered on update. The
  file documents itself as user-owned ("edits here survive Ryoku updates"), but it
  was overwritten back to the `us` default on every `ryoku update`, so anyone with
  several keyboard layouts had to re-add them after each update. Its comment now
  shows the multi-layout syntax (`kb_layout = "us,ru,de,fr"` with a switch key).
  Covered by `materialize_test.go`.
- `doctor` gains a "cursor theme" reconciler that converges the configured cursor
  onto a theme that is actually on disk. The Bibata family now ships as the
  `ryoku-cursors` package (a hard `ryoku-desktop` dependency, from the `[ryoku]`
  repo), so the shipped default is package-guaranteed; the failure worth healing
  is a Hub-picked third-party theme that was never installed, which drops the
  pointer to a bare bitmap. The reconciler resets `hypr.json`'s `cursor.theme`
  back to `Bibata-Modern-Ice` (config-side, never a pacman call) and names both
  sides so a user who set the theme on purpose can re-pick it after installing
  it; it only warns when even the default is absent (a dev checkout with no
  package). Covered by `doctor_test.go`.
- `doctor` gains a "display resolution" reconciler that recovers a monitor a
  degraded link left below its available resolution. A cold boot or the
  post-upgrade `hyprctl reload` can briefly leave a DP/HDMI link advertising only
  a fallback mode (e.g. 800x600); Hyprland resolves `monitors.lua`'s `highrr`
  against that list and never re-picks once the link trains, so the panel stays
  low-res until a relogin. The reconciler (run by every `ryoku update` and by
  hand) re-asserts each output's intended mode via `ryoku-monitor settle`,
  respecting an explicit Ryoku Settings resolution and `monitors_user.lua`.
  Covered by `doctor_test.go` and `tests/monitor-profiles.sh`.
- `materialize` now points at `ryoku deploy` when the base config dir is absent,
  instead of failing with a bare `base config dir not found: /usr/share/ryoku/config`.
  That path only exists on a packaged install; on a dev checkout `ryoku deploy` is
  the command, and the error now says so (a set-but-missing `RYOKU_CONFIG_BASE` is
  called out separately).
- `bin/ryoku-recovery` (the `curl | bash` panic button) now always restores the
  stable `main` branch and repairs the broken checkout in place. A machine from
  an old ISO could be stranded on `unstable-dev`: that ISO's `ryoku-update`
  switched the checkout to a release branch, but the rewritten tree ships none of
  its old helper commands (`ryoku-snapshot`, `ryoku-update-perform`), so the
  update self-destructed and every `ryoku` command broke. Recovery could not dig
  the box out because it honored a leaked `RYOKU_CHANNEL` and cloned a fresh
  checkout beside the broken one, leaving it on `unstable-dev`. Recovery now
  hardcodes `main`, force-resets whichever checkout the machine actually has (the
  pre-rewrite one at the data root, or the current `repo/`) to `origin/main` and
  cleans its stale `bin/` scripts, and drops the dangling pre-rewrite
  `~/.local/lib/runtime-env.sh` PATH bridge. Covered by `tests/ryoku-recovery.sh`.
- `doctor` now creates the snapper `root` config when it is missing instead of
  reporting "not configured" as healthy. The snapshot safety net behind every
  `ryoku update` (the pre/post snapshot pair and the Limine rollback entries) was
  set up only by the installer, so a `ryoku deploy` box, an upgrade from an older
  release, or hand drift left a machine with no snapshots and nothing to restore
  them, and `ryoku snapshots` failed with "config 'root' does not exist". On a
  btrfs root `ryoku doctor` now lays down the same layout the installer writes
  (the `/.snapshots` subvolume, `/etc/snapper/configs/root`, `/etc/conf.d/snapper`,
  ownership, the cleanup timer, and limine-snapper-sync), and on a non-btrfs root
  it warns honestly that snapshots are unavailable. `ryoku status` and the
  pre-update note now distinguish "not configured" from an empty snapshot list.
  When a prerequisite is missing (`snapper` itself, or `snap-pac` and
  `limine-snapper-sync`), doctor does not write a config nothing can use: it
  reports the gap and recommends the exact install command instead.
- `status` no longer escalates to `sudo` unless a real terminal is driving it. The
  Hub and the update island poll `ryoku status --json` on a timer with no
  controlling terminal, but the snapshot count shelled out to interactive
  `sudo snapper list`; with no tty the PAM conversation failed, and `pam_faillock`
  counted every failure until the account was locked out of `sudo` even with the
  right password ("it is not taking my sudo password"). The count now runs only
  from a tty, and even then via `sudo -n` (a cached credential, never a prompt), so
  a background poll can never lock the user out. This also unblocks the Hub Updates
  panel, which sat blank on "checked not yet" while the hung `sudo` kept the status
  query from ever returning.
- `materialize` no longer resets a user's display or GPU configuration on update.
  The package ships seeds for `hypr/monitors.lua` (written by `ryoku-monitor`) and
  `hypr/gpu.lua` (written by `ryoku-gpu`); materialize now seeds them only when
  absent and never clobbers or prunes them. `ryoku update` refreshes shipped
  config while leaving every per-machine and user file (settings.lua, theme.lua,
  user.lua, monitors_user.lua, monitors.lua, gpu.lua) exactly as it found it.
- `doctor` backlight remedy no longer recommends `supergfxctl -m Hybrid` when
  that tool is not installed (it is ASUS-specific and absent on most machines, so
  users hit "Unknown command"). The fix now leads with the universal BIOS GPU/MUX
  route and only mentions the `supergfxctl` shortcut when the binary is present.
  This also cleans up `--explain`, which was echoing the bad command from the
  report it is fed.
- `status` bounds the `checkupdates` call (120s) so a slow or stuck update check
  can never hang `ryoku status` -- the data seam the Hub and the update island
  poll for pending updates.
- Ryoku Hub "check for updates" surfaced nothing on a live mirror: `status` read
  only the `[ryoku]` pacman repo and `checkupdates`, neither of which a dev
  checkout has, so commits pushed to `main` never showed. It now reads the git
  channel below.
- `ryoku update` on a dev checkout crashed at "Materializing desktop configs"
  (`base config dir not found: /usr/share/ryoku/config`): it fell through to the
  packaged path. A checkout now always updates through the git channel + deploy.

### Changed
- `status` and `update` track a git **update channel** (`main` for everyone) on a
  Ryoku checkout, the model the Hub and update island were built for. `status`
  reports how far the **deployed commit** (what the machine is running, recorded
  at deploy) is behind `origin/main`, listing the incoming commits (subject +
  bare short hash). The version it shows is the channel's latest commit, a 7-char
  hash matching GitHub, so a commit pushed to the channel shows as an update until
  the machine redeploys onto it. The fetch takes no credential prompt and is
  bounded so it never hangs. `update` brings
  the channel in (fast-forwarding a clean on-channel checkout) and redeploys from
  the checkout. A packaged install has no checkout, so both fall back to the
  `[ryoku]` pacman view. The `--json` seam gains a `channel` field and
  `pendingUpdates` counts the channel's commits. New `channel.go`;
  `ryoku/shell/deploy.sh` records the checkout and the deployed commit;
  `RYOKU_CHANNEL` overrides the branch.
- On a packaged install `status` now reports the running **commit**. The `[ryoku]`
  packages are versioned `<core>.r<count>.g<sha>` (see release/), so `status` reads
  the `g<sha>` out of the installed and available package versions and shows it as
  the version, with the channel, matching the git-checkout view. The Hub and the
  island then show the commit a machine is on instead of a blank or a static
  `0.1.0-3`.

### Added
- `doctor`: a "Hyprland config integrity" reconciler. It validates that the
  runtime-generated Hyprland drop-ins (`monitors.lua`, `gpu.lua`) still parse and,
  in a live session, reads `hyprctl configerrors`. A corrupt drop-in (a crash or a
  GPU reset that fires monitor events can truncate one mid-write) is regenerated
  from live state, or reset to a safe seed, then the config is reloaded -- so a
  desktop wedged in Hyprland's "emergency mode" recovers without a reboot, which
  `reload`/`update` could not do. Hardware-agnostic. `doctor --report` also gains a
  gpu/compositor stability section (vendor-agnostic GPU resets/hangs across recent
  boots, plus compositor coredumps) so a GPU-reset-induced crash is diagnosable.
- `doctor`: convergent reconcilers for stateful drift the package and config
  layers cannot reach, each idempotent (reports `ok`, converges where safe, or
  proposes the exact fix; `--check` previews) and retireable so the set never
  piles up like ordered migrations. `ryoku update` invokes the `ryoku doctor`
  command after it installs the new binary, so it is one command (not a copy
  baked into update) running the reconcilers from the release just installed. The
  batch covers swap-out-of-snapshots, snapper config consistency, stale pacman
  lock, the `[ryoku]` channel + keyring, desktop session components, the running
  shell daemon (restarted when its control socket is unreachable, so a crashed
  shell with dead keybinds and panels heals itself), failed
  services, btrfs device health, display backlight (no interface, missing
  brightnessctl, or a hybrid-GPU firmware-only backlight that does not dim the
  panel -- read from the kernel's own "no native backlight" verdict, not a sysfs
  value the panel ignores, with the route-to-iGPU fix), pending `.pacnew`, and
  orphaned packages.
- `doctor --report [file]`: when a problem cannot be auto-fixed (or is unknown),
  doctor writes one shareable `.txt` -- the findings plus system state (btrfs
  usage/errors, swaps, failed units, recent journal errors, pacman and ryoku
  channel state, session env) -- and points the user to it, so maintainers have
  the context to diagnose further. Default `~/.local/state/ryoku/doctor-report.txt`;
  no secrets included.
- `doctor --explain`: the reasoning layer over the deterministic reconcilers. It
  sends the diagnostic report to the user's own cloud model (defaults to Groq,
  free; OpenRouter and any OpenAI-compatible endpoint work via `RYOKU_AI_URL`/
  `RYOKU_AI_MODEL`) and prints a root-cause analysis with fix steps, reaching the
  long tail the rules cannot pre-encode. Strictly advisory and read-only: it never
  executes anything (cognition kept separate from actuation), and it is opt-in --
  nothing is sent unless `RYOKU_AI_KEY` (or `~/.config/ryoku/ai-key`) is set.
- `doctor` output is styled for the terminal: colored status glyphs in the Ryoku
  vermilion, terminal-width word-wrap (no more mid-word breaks), and a visible
  `ryoku doctor --explain` hint when issues are found. Color is suppressed when
  the output is piped or `NO_COLOR` is set, so the report file stays plain text.
- `recovery`: a last-resort restore for a broken desktop. It resets a clean
  checkout to `origin/main`, reinstalls the base packages, and rebuilds and
  redeploys the desktop, overwriting the Ryoku configs in `~/.config`. A preflight
  refuses to run on a non-Ryoku machine, and it confirms before touching anything
  (`--yes` skips the prompt, `--no-packages` does configs only). The logic lives
  in the standalone `bin/ryoku-recovery`, which `ryoku recovery` runs from the
  checkout and which doubles as a `curl | bash` rescue when the CLI itself is gone.
- `ryoku` (Go): the user-facing update CLI and single front door to the distro.
  - `update`: snapper pre-snapshot, `pacman -Syu` + `yay`, `materialize`, shell
    reload, snapper post-snapshot. Aborts safely on a package failure.
  - `rollback` / `snapshots`: snapper-backed restore + listing.
  - `status [--json]`: installed/available version, pending updates, snapshot
    count (the `--json` form is the Hub's data seam).
  - `materialize`: lays `/usr/share/ryoku/config` into `~/.config` declaratively --
    copies shipped files, prunes ones dropped from a release, and NEVER touches
    user overrides (`hypr/user.lua`, `kitty/user.conf`, `fish/user.fish`). No
    migrations directory; the production replacement for `deploy.sh`'s config copy.
  - `reload`: delegates to `ryoku-shell reload` (removes the triplicated restart
    logic).
  - `deploy`: DEV-only build-from-checkout loop (RYOKU_REPO).

### Verified
- Builds clean, `go vet` clean. `materialize` tested end to end: copies base
  configs, prunes dropped files, and leaves a user override intact.
- `go test ./...` covers the channel: the deployed-commit baseline, behind counts,
  the commit list, the fast-forward update, and the off-channel/dirty redeploy
  guards. Confirmed live: `ryoku status --json` reports `channel: main` and the
  count goes from 0 to N when `main` advances past the deployed commit.
