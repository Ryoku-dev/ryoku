# Changelog

## Unreleased

### Fixed
- **Dvorak, Colemak and other keyboard variants carry into the installer.** The
  graphical installer switched its keyboard to the base layout only, so picking
  Dvorak captured the disk passphrase on QWERTY while the boot unlock prompt
  used Dvorak, and the passphrase never matched. The installer now relaunches
  under the layout and its variant (`tui/system.go`, `ryoku-installer-session`).

- **The offline package closure now carries `asusctl`.** Supported ASUS Aura
  laptops can select the native keyboard-lighting provider during an offline
  install without making every target install laptop-specific control software.

- **A truncated package no longer ships in the offline repo.** `offline-repo.sh`
  downloaded the closure with `pacman -Sw --needed` into a persistent cache and
  reflinked it in with no integrity check, so a download cut short by one network
  hiccup (most likely on a big package) sat
  corrupt in the cache forever (`--needed` never re-fetches a file that already
  exists by name) and shipped in every ISO, bricking the offline pacstrap with a
  "truncated <pkg>" error. The bake now verifies every cached package with
  `bsdtar -tf` before assembling, deletes and re-downloads a short archive, and
  fails the build on one that stays broken (a cheap rebuild instead of a bricked
  ISO). Covered by `tests/offline-repo-integrity.sh`.

### Added
- The offline closure ships GPU Screen Recorder. `base.packages` swaps
  `wf-recorder` for `gpu-screen-recorder`, so the baked `[offline]` repo carries
  the GPU-only recorder; Ryoku Motion (`ryomotion`) is no longer built, so it
  drops out of the closure.
- **Two ISO variants from one tree (`RYOKU_VARIANT` plain|cachyos), both fully
  offline.** `build.sh` bakes the whole package closure into a `file://`
  `[offline]` repo so the installer pacstraps with no network, and a
  `/usr/share/ryoku/variant` marker selects the layer: plain is stock Arch;
  cachyos bakes the CachyOS kernel, tuning, schedulers and Proton in, boots
  `linux-cachyos`, and wires the CachyOS repos into the target. `build-iso.yml`
  and `build-iso-cachyos.yml` call a shared reusable workflow; each keeps its own
  run counter (plain `r164`, cachyos `r1`) and publishes its own `latest*.json`.
- **The live installer kiosk now shows the Ryoku (Bibata) cursor.** The live set
  gains `ryoku-cursors` from the `[ryoku]` repo and `ryoku-installer-session`
  exports `XCURSOR_THEME=Bibata-Modern-Ice`, so cage/wlroots draws the real
  pointer instead of its built-in fallback bitmap. `build.sh` wires a local
  `file://` `[ryoku]` mirror into the staged `pacman.conf` (SigLevel TrustAll,
  scoped to that build-time repo only; the installed system keeps its strict
  `SigLevel=Required`) and populates it from `RYOKU_ISO_LOCAL_REPO`, a
  `release/repo/out` tree, or a fetch from `repo.ryoku.dev` -- failing with an
  actionable message when the package is unreachable or unpublished, so
  `publish-repo` (main push) must precede `build-iso` (tag). `iso-stage-check.sh`
  normalizes the per-run repo path so the reproducibility diff stays clean.

### Fixed
- **An incomplete offline closure now fails the ISO build instead of shipping.**
  The AUR bake was best-effort at three separate levels: a build host without
  `base-devel` skipped the whole set and returned success, a clone failure skipped
  one package, a build failure skipped one package, and the call site swallowed
  anything else. The consequence was not theoretical. The last released image's own
  build log reads `AUR bake: skip otf-space-grotesk (build failed)` and
  `skip volantes-cursors (build failed)`, so it shipped without the UI sans the
  shell and Hub render, and nothing failed. These images are the only place that
  set ever lands, because an offline install never builds from the AUR, so a miss
  has to stop the release. The desktop set is now fatal on failure and the baked
  repo is checked for every name in `aur.packages` before the db is written.
  `RYOKU_OFFLINE_ALLOW_INCOMPLETE=1` keeps local builds that only need a bootable
  image working.
- **Pre-Turing NVIDIA cards get their driver from the image.**
  `system/hardware/drivers/nvidia.sh` installs the AUR-only 580xx branch for
  Maxwell/Pascal/Volta and told the user it was "bundled in the offline repo". It
  was not: those packages are AUR, so the `pacman -Sw` passes could not reach them,
  and they were in no package list either, which meant a GTX 10xx installing
  offline hit exactly the driverless desktop that the required-driver check exists
  to prevent. The 580xx and 470xx branches (plus their lib32 halves) are now built
  into the repo alongside the rest of the AUR set. They are large vendor blobs, so
  a failed bake warns and names the affected hardware rather than failing the
  release, and the driver script's messages now say so instead of promising more
  than the image carries.
- The `[vm]` hardware profile is read into the closure and its verification. It
  carries no packages today, so nothing was missing, but three of the four profiles
  were baked by name and a package added there later would have gone missing at
  install time rather than at build time.
- **The installer session sizes text to the panel, so the TUI is readable and
  complete on any monitor.** The kiosk launched `foot` at a hard-coded
  `size=14`, and the installer's usable size is a character grid: panel pixels
  divided by a font cell. Nothing in the terminal stack scales for you (bubbletea
  only ever learns columns and rows; `ws_xpixel` is unused), so one fixed size
  meant the grid swung with the display: 1080p got a comfortable ~174x43, but
  1366x768 got 30 rows and cut the Review screen's confirm buttons off the
  bottom, and 4K got ~349x86 of microscopic text. That is the reported "different
  scale on their PC", same font, different panel. Both ISO variants share this
  path, so both were affected. The session now reads the panel's native mode from
  DRM and picks the point size from it, anchored so 1080p still resolves to
  exactly 14pt (the resolution that already worked is untouched); the grid now
  stays near 170x43 from 480p to 4K while text holds a constant physical size.
  `dpi-aware=no` is pinned so the pt-to-px conversion is foot's deterministic
  `round(pt * scale * 4/3)` instead of whatever DPI an EDID claims. The console
  fallback gets the same treatment through `setfont`, sized from the framebuffer
  (not the DRM fallback, which is absent exactly when that path runs) and only
  ever enlarged while the grid stays at least 96x30, using cells `kbd` already
  ships: `default8x16`, `iso01-12x22`, `latarcyrheb-sun32` (16x32) and
  `solar24x32`. The session also waits for KMS before measuring, since tty1 is
  the 80-column boot console until the real framebuffer arrives and anything
  decided against it is about to be wrong (`tests/installer-session-scale.sh`).
- **A file conflict in the baked closure now fails the build, not the offline
  install.** `offline-repo.sh` verifies the exact pacstrap set resolves and that
  no two packages own the same path before the ISO is assembled. An upstream
  churn window (e.g. `default-cursors` taking over
  `/usr/share/icons/default/index.theme` while an older cursor package still
  shipped it) otherwise froze a file conflict into the `[offline]` repo; the
  target then aborted at pacstrap ("conflicting files ... exists in filesystem")
  with no network to recover from, bricking the install. The check runs on the
  networked build host where a rebuild is cheap, and also asserts every resolved
  dependency is present (a missing one is the same offline dead end).
- **The live ISO no longer hangs at boot; Ventoy boots reliably.** The
  `cow_label=vtoycow` parameter (added for Ventoy persistence) made archiso wait
  30 s for a `/dev/disk/by-label/vtoycow` device and then drop to an initramfs
  emergency shell when it was absent -- that is, on every plain USB/`dd` boot and
  every Ventoy boot without a `vtoycow` persistence partition, which is the normal
  case. It is removed. The `cow_spacesize=1G` tmpfs overlay is the live writable
  layer on all media. Ventoy's normal mode boots the image through its
  device-mapper virtualization (it presents the ISO as a virtual block device, not
  via `img_dev`/`img_loop` injection); the README is corrected to match.
- **Ventoy's GRUB2 mode can now boot the ISO.** Ventoy's normal (device-mapper)
  mode is still the primary path, but when its injection fails on a machine's
  firmware archiso finds no medium, waits, and drops to an initramfs rescue
  prompt where the USB keyboard is typically dead -- the reported "install fails
  and no key works" case. Without a `grub/loopback.cfg` the documented GRUB2-mode
  fallback (Ventoy's secondary menu / `Ctrl+r`) was unavailable too, so such a
  machine had no way in. The ISO now ships `installation/iso/grub/loopback.cfg`;
  because no `grub` bootmode is active, `mkarchiso` copies it to
  `/boot/grub/loopback.cfg` (`_make_common_grubenv_and_loopbackcfg`), so a
  loopback loader boots the image by file with `img_dev`/`img_loop` and
  `archiso_loop_mnt` mounts the ISO directly instead of searching for a label.
  `build.sh` stages the new `grub/` profile directory alongside `efiboot` and
  `syslinux`.
- **The live serial console (`ttyS0`) now gets a login.** The boot entries make
  `tty0` the primary console, so systemd-getty-generator never spawned
  `serial-getty@ttyS0`; the serial console was dead, which also meant the
  automated `install-vm.py` boot/install test could never drive the ISO. The
  profile now enables `serial-getty@ttyS0.service`, so headless/recovery and the
  install test reach a root shell.
- **The initramfs builds clean.** The stock archiso PXE hooks (`archiso_pxe_*`)
  pulled `ipconfig`/`nfsmount`/`nbd-client` from packages the ISO does not ship,
  so every build errored per-hook and flagged the image "possibly incomplete".
  Ryoku boots from USB/Ventoy/`dd`, never PXE, so those hooks are dropped from
  `mkinitcpio.conf.d/archiso.conf`; a real initramfs error is now visible.
- **`build.sh` no longer ships a stale ISO.** mkarchiso reuses a populated work
  dir and skips the airootfs rebuild, silently re-emitting the previous image
  without the edits just made; the build now clears the work dir first.
- The live installer session (`ryoku-installer-session`) no longer drops a Ventoy
  or GPU-less user to a bare shell with no installer. It checks for a DRM/KMS card
  node before launching cage (skipping three dead retries when wayland can never
  start), verifies `ryoku-tui` exists, prints the graphical-failure reason and log
  path to the console, always falls back to the console TUI, and ends with a
  recovery notice on how to install by hand if both TUIs exit.
- The live MOTD now tells a manual installer that a whole-disk wipe of a disk
  that already holds partitions needs `RYOKU_WIPE_CONFIRMED=1` (the graphical
  installer's typed ERASE step). A user who fell back to the console shell (e.g.
  a Ventoy boot where the graphical installer never took the console) and set
  `RYOKU_DISK_STRATEGY=whole` hit the backend's wipe guard with no hint that the
  confirmation lives in an env var.
- The live environment gets a 1 GiB copy-on-write overlay (`cow_spacesize=1G`
  on both boot entries) instead of archiso's 256 MiB default. A long install
  session writes sync databases, keyring state, and logs into that overlay,
  and running it dry mid-install surfaces as random "no space left" failures,
  reported from a Ventoy boot.

### Added
- Reproducible ISO builds: `build.sh` derives `SOURCE_DATE_EPOCH` from the
  commit's committer date and exports it to `mkarchiso` and `profiledef.sh`; the
  three prebuilt Go binaries build with `-trimpath -ldflags '-s -w -buildid='`
  and `CGO_ENABLED=0` under a pinned `toolchain`; and the build emits
  `SHA256SUMS` next to the ISO. `RYOKU_ISO_REPRO=1` additionally pins
  `[core]`/`[extra]` to the commit-dated Arch Linux Archive to freeze the baked
  package set. See README, "Reproducibility".
- Payload provenance stamp: `build.sh` writes `/usr/share/ryoku/.payload`
  (commit, commit date, `VERSION`) and fills the same values into `/etc/motd`, so
  the target's deploy step can warn on ISO-vs-`[ryoku]`-repo version skew.
- Two boot fallbacks on both firmware paths (UEFI + BIOS): "safe graphics"
  (`nomodeset`) for machines that boot to a black or garbled screen, and "copy
  to RAM" (`copytoram`) for flaky or removable USB media. The normal installer
  entry stays the default.
- Live ISO packages: `pciutils` (`lspci` GPU/Wi-Fi/VMD probing), `broadcom-wl`
  (Broadcom BCM43xx Wi-Fi has no in-kernel driver), and `mdadm` + `lvm2` (so
  free-space probing reads disks with existing RAID/LVM correctly).
- Add the Ryoku live ISO profile (archiso, releng-based).
  - `profiledef.sh`: iso_name `ryoku`, label `RYOKU_<YYYYMM>`, date-stamped
    version, `install_dir=arch`, BIOS (syslinux) and UEFI (systemd-boot) boot
    modes, x86_64, zstd squashfs, and file permissions for the launchers.
  - `packages.x86_64`: the live environment only. Base system, kernel, archiso
    hooks, both bootloaders, the backend toolchain, NetworkManager + iwd, and
    cage + foot + the JetBrains Mono Nerd Font. No Go: the TUI ships prebuilt.
  - airootfs overlay: root autologin on tty1, a tty1-only login path
    (`.bash_profile` -> `.zlogin`), and `ryoku-installer-session`, which runs the
    TUI in cage + foot with a crash-relaunch loop and exports `RYOKU_REPO` and
    `RYOKU_BACKEND`. NetworkManager is enabled with iwd as its Wi-Fi backend so
    the TUI's `nmcli` calls work. The serial console stays a plain root shell.
  - `build.sh`: stages a throwaway copy of the profile, builds the TUI from
    `../tui`, bakes the TUI, the backend (under `/usr/local/lib/ryoku/backend`
    with a `/usr/local/bin/ryoku-install` wrapper), and the repo payload (at
    `/usr/share/ryoku`, tracked files only via `git archive`) into the staged
    airootfs, then runs `mkarchiso`. The committed profile is never mutated.
  - `build.sh` also prebuilds the `ryoku-shell` daemon (Go) from `ryoku/shell/ipc`
    into the repo payload, so the backend can install it on the target, which has
    no Go toolchain.

### Fixed
- Drop the inline comments from `packages.x86_64`: mkarchiso left their trailing
  whitespace on the package names, so pacstrap reported "target not found".
- Suppress `systemd-firstboot`: ship `/etc/locale.conf`, `hostname`, `localtime`,
  and `vconsole.conf` and mask the service, so the image autostarts the installer
  instead of the stock Arch "Initial Setup" prompt.
- Ship a working `/etc/pacman.d/mirrorlist`: the Fastly CDN mirror
  (`fastly.mirror.pkgbuild.com`) leads, then the routed `geo.mirror.pkgbuild.com`
  and global backups. The default all-commented list left pacstrap with no
  servers; Fastly's edge POPs (incl. South America) keep downloads fast where the
  geo mirror has no nearby origin (it otherwise routes Brazil to Los Angeles).
- Ship `reflector` in the live set so the backend re-ranks the package mirrors by
  measured speed before pacstrap (see the backend's `lib/mirrors.sh`). The static
  Fastly-led list still stalled for users its CDN routes badly; ranking at install
  time picks a nearby fast mirror and falls back to the shipped list.
- Initialize the live pacman keyring: ship and enable `pacman-init.service` plus
  the gnupg tmpfs mount (as releng does), so pacstrap can verify packages.
- Force a truecolor TUI and quiet the boot: the session exports
  `COLORTERM=truecolor`, and the kernel cmdline gains `quiet loglevel=3` to hide
  amdgpu link-training console spam before the installer.
- Add `xorg-xwayland` to the live image so cage can start its Xwayland server
  (removes the "cannot create xwayland server" error).
- Send the cage session's output to `/var/log/ryoku-session.log` and set
  `WLR_RENDERER_ALLOW_SOFTWARE=1`, so the harmless software-render "renderer did
  not support importing dma-bufs" line stays in the log instead of on the console
  where it looked like an install error. Real session failures are still logged.
