# Changelog: installation/backend/lib/

## Unreleased

### Added
- `bootloader`: **the install records the kernel it boots**, in
  `/etc/ryoku/default-kernel` (`linux-cachyos` on the CachyOS variant, `linux`
  on plain). On a live box a kernel package the user added later is
  indistinguishable from the one the install was built around, which is why the
  doctor used to sniff for "cachyos" in the entry names and repointed
  `default_entry` on plain installs that had simply added a second kernel. The
  doctor now reads this file first.
- `bootloader`: **`ryoku_limine_autoboot` picks the kernel the install chose, by
  the name it already knows.** It used to scan the generated menu for any entry
  containing "cachyos" and prefer that, which is a guess where the variant is a
  fact: it now matches `RYOKU_VARIANT`'s kernel (`linux` or `linux-cachyos`)
  exactly, and falls back to the first kernel in the menu. Mirrors the doctor's
  `pickKernelPath`, which no longer prefers a brand either.
- `chroot`/`deploy`: **the initramfs stops carrying the denylisted nouveau
  driver.** The shipped HOOKS drop-in now names `ryoku-gpu-trim` between
  `autodetect` and `kms` (`system/boot/mkinitcpio/install/ryoku-gpu-trim`),
  which keeps nouveau and the ~107 MiB of GSP firmware it pulls out of every
  kernel image, so a 2 GiB `/boot` keeps the room a kernel update needs to copy
  its new image in. `ryoku_cfg_initramfs` drops the name again when the repo
  has no hook to deliver, since a HOOKS entry mkinitcpio cannot find aborts
  every image build; `ryoku_seed_initcpio_hook` lays the hook after the desktop
  set, and only when that set never installed it (an offline install with no
  baked payload), so the packaged path stays owned by `ryoku-desktop` on a
  normal install.

### Fixed
- `cachyos`: **non-v3 CPUs keep compatible repositories after install.**
  The target now enables the v3 core/extra rebuilds only when glibc reports
  x86-64-v3 support; baseline x86-64 uses the generic `[cachyos]` repository.
- `bootloader`: **a CachyOS install autoboots the CachyOS kernel.**
  `ryoku_limine_autoboot` pointed default_entry at the first kernel the menu listed
  (stock `linux`), so a fresh CachyOS box booted the Arch kernel; it now prefers the
  linux-cachyos entry when present, matching `limineDefaultKernelPath` and
  `reconcileLimineAutoboot` in the doctor (#140).
- `deploy`/`bootloader`: **the Plymouth splash no longer breaks the install.**
  `ryoku-desktop` (installed in the configure stage) already owns
  `/usr/share/plymouth/themes/ryoku/`, but `bootloader` then laid a second,
  unowned copy over it, so the package's own files read as strays and the first
  `ryoku update` aborted with "exists in filesystem". `ryoku_boot_plymouth` now
  only sets the default theme (guarded on the theme being present), never
  re-deploys it. The desktop `pacman -S` (online and the baked-offline path) also
  retries once with `--overwrite '*'`, so a resumed install adopts files a killed
  first attempt left half-extracted instead of dying, matching `pacstrap`.
- `pacstrap`: **a resumed install no longer dies on "exists in filesystem."**
  When the first pacstrap died mid-transaction (a dropped connection or a
  package that downloaded corrupt), it left some packages' files extracted but
  unowned; the `--needed` retry only skips fully-installed packages, so it
  re-tried those and aborted with e.g. `gamemode: ... exists in filesystem`, and
  retrying the step kept hitting the same leftovers. The online retry now also
  passes `--overwrite '*'`, so it adopts the orphaned files and resumes cleanly.
- `bootloader`: fresh installs now `systemctl enable power-profiles-daemon.service`
  alongside sddm/NetworkManager/bluetooth/rtkit, so the shell's power-mode switching
  works out of the box instead of depending on transient D-Bus activation.
- **One fewer `.pacnew` out of the box.** `chroot` no longer overwrites the
  `filesystem`-owned `/etc/hosts` (the default `nss-myhostname` already resolves
  localhost and the machine hostname), so the first `ryoku update` no longer spawns
  an `/etc/hosts.pacnew` when `filesystem` upgrades.

### Added
- `snapshots`: prune `/.snapshots` from `updatedb` (`PRUNEPATHS`) so `plocate`
  indexes the live system, not every snapshot; mirrors the `ryoku doctor`
  reconciler and is skipped when the target has no `updatedb.conf`.
- `seed`: the decor art the desktop's Decor/Placard components render is laid into
  `~/Pictures/ryodecors` (beside `Wallpapers`), from `ryoku/assets/ryodecors`, so a
  fresh install has the set; `ryoku doctor` keeps it current after.

### Fixed
- `bootloader`: the post-AUR limine finalize points `default_entry` at the
  kernel's entry path (`<dir>/<kernel>`) and sets `remember_last_entry: yes`,
  instead of the bare `default_entry: 2` that lands on the `/EFI fallback` once
  the hook makes the OS entry a directory, chainloading Limine and looping the
  countdown. New `ryoku_limine_autoboot` helper, covered by
  `tests/limine-bootloader.sh`.

### Added
- Step helpers split by concern: `common`, `preflight`, `disk`, `luks`,
  `filesystem`, `pacstrap`, `chroot`, `deploy`, `bootloader`.
- `snapshots`: a snapper `root` config, snap-pac registration, the snapper cleanup
  timer, and `limine-snapper-sync`, wired after the AUR step (dry-run safe, gated
  on the `@snapshots` subvolume).
- `common`: `append_file`, a dry-run-safe stdin-to-file appender.
- `drivers`: consume `RYOKU_GPU_MODE` -- after the vendor scripts, run `ryoku-gpu
  mode` (offload->hybrid, sync->performance, vfio->passthrough) as the user
  against `~/.config/hypr/gpu.lua`, best-effort and dry-run narrated.

### Changed
- `deploy`: install the desktop from the signed `[ryoku]` pacman repo plus
  `ryoku materialize`, replacing the per-file binary/dotfile/QML/udev copies (now
  package-owned). Brand assets, wallpapers, `~/.npmrc`, the neovim default-editor
  registration, and the qylock + SDDM theme are still seeded here.
- `bootloader`: `limine.conf` now lands at `/boot/limine.conf` (the ESP root),
  the one location `limine-entry-tool` manages, instead of
  `/boot/limine/limine.conf`, which Limine scans FIRST on the same partition
  and which therefore shadowed every generated entry: the UKI tree and the
  snapper snapshots never appeared in the boot menu. All shadowing config
  candidates are removed (mirrors `limine-install`'s conflict list).
- `bootloader`: the EFI binary now lands on `EFI/limine/limine_x64.efi` (plus
  the `EFI/BOOT` fallback), the exact path the tool's pacman hook refreshes on
  every `limine` upgrade, so the firmware never keeps booting an install-time
  binary that ages out of sync with the package (stale menu rendering). The
  NVRAM entry points there too, so `limine-install`'s dedup adopts it instead
  of registering a second one.
- `bootloader`: `default_entry` now matches the menu shape: `1` for the flat
  placeholder menu (offline installs), `2` once `limine-mkinitcpio-hook` owns
  the file (entry 1 becomes the `/+Ryoku` directory, which Limine refuses to
  autoboot; 2 is the newest UKI inside it).
- `bootloader`: new `ryoku_bootloader_finalize` (runs after the AUR step)
  retires the flat placeholder entry and repoints the default once the hook's
  UKI tree exists, then re-syncs the Windows chainload entry.

### Fixed
- `deploy`: install the `ryoku-mic` microphone normalizer onto the target. The
  Hyprland autostart runs it on login, but the bin step skipped it, so a fresh
  install lacked the mic-gain capping the live desktop applies.
- `filesystem`: the swapfile now lives in its own `@swap` subvolume instead of a
  `/swap` directory inside `@`. btrfs cannot snapshot a subvolume that holds an
  active swapfile, so the old layout made every snapper snapshot fail (`Creating
  snapshot failed`, then snap-pac's `Invalid snapshot '--type'`) on a default
  install with swap. `@swap` is created and mounted only when `RYOKU_SWAP_GIB > 0`.
- `disk`: a TUI retry no longer wedges. `ryoku_partition` releases a `/mnt` left
  mounted by a failed attempt (`ryoku_release_previous_attempt`: swapoff the
  installer swapfile, `umount -R /mnt`) before touching the disk;
  `ryoku_release_disk` reads `/proc/mounts` (all of a device's mountpoints, not
  the one lsblk shows) and `/proc/swaps` (swapFILEs on the disk), so nothing pins
  the disk on the wipe.
- `disk`: reclaiming leftover `ryoku`/`ryokuboot` partitions is gated behind
  `RYOKU_RECLAIM_LEFTOVERS=1`; without the ack, `alongside` dies listing them
  rather than deleting what might be a healthy completed install.
- `disk`: whole-disk ESP sizing uses MiB math, so `RYOKU_ESP_GIB=1` is a true
  1 GiB ESP (`1MiB..1025MiB`), not ~2 GiB.
- `common`: `dev_uuid` returns non-zero on empty `blkid` output; `bootloader`
  (`ryoku_cmdline`) and `chroot` (crypttab) now die on an empty UUID instead of
  writing an unbootable `root=UUID=`/crypttab (command substitution had swallowed
  the errexit).
- `filesystem`/`bootloader`: the >= 64 MiB ESP capacity check moved to the end of
  `ryoku_mount` (right after the ESP mounts), so a too-small ESP is caught before
  pacstrap/mkinitcpio fill `/boot` instead of deep in the bootloader step.
