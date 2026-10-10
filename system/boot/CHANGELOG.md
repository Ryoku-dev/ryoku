# Changelog: system/boot/

## Unreleased

### Added
- `mkinitcpio/install/ryoku-gpu-trim`: a mkinitcpio install hook that keeps the
  denylisted nouveau driver out of the initramfs, and with it the
  `nvidia/*/gsp` firmware linux-firmware ships for every Turing/Ampere/Ada chip
  and two driver branches. `nvidia.sh` denylists nouveau, but autodetect still
  matches it against the card, so `kms` pulled the module in and mkinitcpio put
  those pre-compressed blobs in the uncompressed early cpio: measured on an
  NVIDIA box, 155 MiB of a 254 MiB image, and the whole thing lands on a 2 GiB
  boot partition twice per kernel (the image plus one history copy for the
  snapshot entries). A box with two kernels sat at 54% before installing
  anything else. When that partition runs out, mkinitcpio still builds the
  image in `/tmp` but the hook cannot copy it in, pacman reports the
  transaction as a success, and the box keeps booting the previous kernel
  against a module tree the upgrade deleted: issue #140, and the "the kernel
  never updates with pacman -Syu" reports behind it. The hook drops nouveau
  from autodetect's allowlist, which is what `kms` filters against, so it has
  to sit between `autodetect` and `kms`; `mkinitcpio/ryoku.conf` places it
  there. Measured on the same box: 254 MiB to 147 MiB per image, boot partition
  54% to 43%. `ryoku-nvidia-guard` lifts the denylist when a kernel has no
  nvidia module and rebuilds, so the recovery image gets nouveau back.

- `mkinitcpio/install/ryoku-console-keys` and
  `mkinitcpio/hooks/ryoku-console-keys`: an early initramfs hook that carries
  `/etc/vconsole.conf` into the image and turns Num Lock on for every console
  before Plymouth starts. The disk unlock prompt now uses the keyboard layout
  picked in the installer and starts with Num Lock on. It sits right after
  `udev` in `mkinitcpio/ryoku.conf`.

### Fixed
- `limine/90-ryoku-restore-default` resets Limine to the restored system's
  first kernel and clears its remembered snapshot entry; Void's boot guard now
  logs through vlogger.
- `limine/50-ryoku-limine` now builds Void kernels under one `/Ryoku Linux`
  menu, keeps snapshot entries in place across kernel changes, and excludes
  ESP kernels whose matching module tree is absent after a snapshot restore.
- `plymouth/ryoku/ryoku.script`: the unlock dialog says what it wants ("Enter
  the disk passphrase") and says so when a passphrase is rejected ("Wrong
  passphrase, try again"). Before, a wrong passphrase just emptied the box and
  waited again with no sign anything had happened.
- `limine/ryoku-windows-entry` now also restores the installer-recorded
  Linux/Ryoku neighbor after Limine regenerates its menu on dedicated-ESP
  dual-boot systems.
- `limine/ryoku-windows-entry`: `strip_managed` is now title-anchored and
  fence-tolerant instead of deleting everything between the begin/end fences.
  When limine-entry-tool 1.37 adopts the flat `/Ryoku Linux` placeholder it
  re-serializes our begin fence ABOVE the generated `//<kernel>` UKI children
  (the fence rode along as an attached configLine), so the old fence-to-fence
  delete run by the shipped `45-ryoku-windows` sync hook swallowed the freshly
  built UKI entries on every menu rewrite: the menu went permanently flat and,
  after the first kernel update, the default booted a stale kernel with no
  modules. The strip now removes ONLY the fence marker lines (wherever they
  land, even indented) and the `/Windows` node (its title plus its indented
  body); any other top-level entry (`/` or `//`) ends the removal region, so
  UKI children and foreign entries are never spanned. The managed block format
  is unchanged, so existing installs still update in place.
- `limine/default.conf`: document what `TARGET_OS_NAME` is for and why it is
  `Ryoku`. It must match the OS entry name limine-snapper-sync hangs the Snapshots
  submenu under (the `/+Ryoku` UKI tree, name "Ryoku"); `ryoku doctor` now
  re-aligns it if a box is stuck on the flat `/Ryoku Linux` fallback entry, where
  the mismatch failed snapper-cleanup and left no rollback snapshots in the menu.
- `limine/limine.conf` documents its real deploy target: `/boot/limine.conf`
  (the ESP root), the one config `limine-entry-tool` manages. The old comment
  described `/boot/limine/limine.conf`, a location Limine scans FIRST, so a
  config there shadows every generated entry (UKI tree, snapshots submenu).
- `limine/limine.conf`: `default_entry: 2`, matching the tool-managed menu
  (entry 1 is the `/+Ryoku` directory, which Limine refuses to autoboot; 2 is
  the newest UKI inside it). The installer rewrites it to 1 while the menu is
  still the flat placeholder. The placeholder path now names the UKI the hook
  actually builds (`ryoku_linux.efi`, from `CUSTOM_UKI_NAME="ryoku"` + the
  `linux` pkgbase), not the never-produced `ryoku.efi`.
- `limine/default.conf`: comment named the shadow path; now points at
  `/boot/limine.conf`.
- efibootmgr NVRAM registration is now best-effort. On firmware that exposes
  NVRAM as readonly or reports it full (HP / Insyde-class), `--create` /
  `--bootnext` used to abort the whole install under `set -e`. They now log a
  loud warning and continue: the UEFI removable-path fallback
  `EFI/BOOT/BOOTX64.EFI` on Ryoku's own ESP keeps the machine bootable. That
  fallback copy is always safe now that both partitioning strategies install
  onto Ryoku's own ESP (never the Windows ESP), so it can't clobber a foreign
  fallback loader.
- Intel VMD carry-over: when the live installer kernel needed the `vmd` module
  to see the NVMe (Intel RST "VMD" mode), the installer now writes
  `/etc/mkinitcpio.conf.d/ryoku-vmd.conf` with `MODULES+=(vmd)` before building
  the initramfs (both the limine-mkinitcpio UKI path and plain `mkinitcpio -P`),
  so the installed system can still find its own root disk at boot instead of
  dropping to an emergency shell.

### Added
- `mkinitcpio/ryoku.conf`: `resume` hook, right after `encrypt`, for
  hibernation. When a swapfile exists (`RYOKU_SWAP_GIB > 0`), the installer
  appends `resume=<dev> resume_offset=<n>` to the kernel command line (`<dev>`
  mirrors `root=`: `/dev/mapper/root` under LUKS, else the root fs `UUID=`;
  `<n>` from `btrfs inspect-internal map-swapfile -r`). Older btrfs-progs
  (< 5.16) without `map-swapfile` are skipped cleanly (no resume=, still boots).
  `resume` stays a separate word so the installer's `encrypt`-stripping sed is
  unaffected, and it is a no-op with no `resume=` cmdline, so it is safe on
  every install.
- ESP free-space guard: before writing the Limine binary the installer asserts
  `/mnt/boot` has >= 64 MiB free and dies with a clear message otherwise -- a
  last-line guard that should never fire with the dedicated >= 1 GiB ESP, but
  fails clearly instead of half-writing and erroring deep in efibootmgr.
- `limine/limine.conf`: Ryoku-branded Limine config (branding string, orange
  accent, Greek Noir palette, timeout, default UKI entry placeholder).
- `limine/default.conf`: UKI build settings (TARGET_OS_NAME, ESP_PATH,
  ENABLE_UKI, CUSTOM_UKI_NAME, snapshot entries, `quiet splash` cmdline).
- `plymouth/ryoku/`: vendored Ryoku Plymouth splash (manifest, script, assets).
- `mkinitcpio/ryoku.conf`: HOOKS drop-in with `plymouth` and `kms`; `encrypt`
  documented as LUKS-only.
