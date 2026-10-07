# installation/

Everything needed to get Ryoku onto a computer: the live ISO, the guided
installer TUI, and the backend that does the real work. This file is the map;
each subsystem has its own README with the detail.

## The tree

```
installation/
  README.md              this map
  CHANGELOG.md           the installation overhaul, one Unreleased block

  tui/                   the installer front-end (Go, Bubble Tea v2)
    README.md            the front-end reference: step flow, gates, layout math
    main.go              pure UI: screens, layout, wizard state, the layout math
    system.go            the only file that touches the machine (lists, hardware
                         detection, RYOKU_* handoff to the backend)
    apps.go              the browser, login-shell, and app choice tables (what
                         ships, what a row means, which rows are required)
    partition_test.go    unit tests for the layout math + safety gates
    go.mod / go.sum      module (bubbletea/v2, lipgloss/v2, harmonica, qrterminal)
    ryoku-tui            the built binary (git-ignored; build.sh rebuilds it)

  backend/               the installer back-end (bash)
    README.md            the RYOKU_* contract, dry-run, disk strategies
    CHANGELOG.md         backend-only changes
    ryoku-install        the orchestrator: defaults, validation, stages, traps
    lib/                 one file per stage (see lib/README.md)
      README.md          per-file table: sentinel, job, writes, failure behavior
      CHANGELOG.md       lib-only changes
      common.sh          log/step/die, the run/run_sh/run_secret dry-run wrappers,
                         write_file, device helpers (part_dev, part_num, dev_uuid)
      preflight.sh       root/UEFI/Secure-Boot/whole-disk/payload gates
      disk.sh            GPT partitioning: whole vs alongside, reclaim, free-space
      luks.sh            optional LUKS2 on root
      filesystem.sh      mkfs, btrfs subvolumes, mounts, swapfile
      pacstrap.sh        base system + fstab, keyring wait, Broadcom detect
      mirrors.sh         reflector mirror ranking before pacstrap
      chroot.sh          in-target config: locale/keymap/tz/user/HOOKS/crypttab
      deploy.sh          the Ryoku desktop from the [ryoku] repo + materialize
      network.sh         carry wifi into the target; DNS/HTTP/clock gates
      drivers.sh         per-vendor GPU driver scripts in the chroot
      bootloader.sh      Limine install/branding, cmdline, resume, VMD, finalize
      aur.sh             bootstrap yay, build the AUR set (best-effort)
      snapshots.sh       snapper root config, snap-pac, limine-snapper-sync

  iso/                   the live image (archiso, releng-based)
    README.md            what boots, what is baked, build + reproducibility
    CHANGELOG.md         iso-only changes
    build.sh             stage the profile, bake binaries + payload, mkarchiso
    profiledef.sh        archiso profile (name, label, boot modes, permissions)
    packages.x86_64      the live-environment package set (target set is not here)
    pacman.conf          pacman config used to build the live image
    .gitignore           out/, work/, staging/
    airootfs/            files overlaid onto the live root:
      etc/systemd/system/getty@tty1.service.d/autologin.conf   root autologin
      etc/systemd/system/pacman-init.service                   keyring at boot
      etc/systemd/system/{multi-user,sysinit}.target.wants/*   enable NM, pacman-init, timesyncd
      etc/systemd/system/etc-pacman.d-gnupg.mount              tmpfs keyring dir (pacman-init)
      etc/systemd/system/systemd-firstboot.service             masked (-> /dev/null): no firstboot
      root/.bash_profile, root/.zlogin                          tty1 -> session
      usr/local/bin/ryoku-installer-session                     cage+foot+ryoku-tui
      usr/local/bin/ryoku-install                               PATH wrapper -> backend
      etc/NetworkManager/conf.d/wifi-backend.conf               NM over iwd
      etc/pacman.d/mirrorlist                                   shipped mirrors
      etc/motd                                                  version/commit banner
      etc/{hostname,locale.conf,localtime,vconsole.conf,shadow} suppress firstboot
      etc/mkinitcpio*                                           archiso initramfs
    efiboot/loader/loader.conf                                  systemd-boot: default + timeout
    efiboot/loader/entries/                                     UEFI (systemd-boot):
      01-ryoku-linux.conf        normal boot (KMS on), the default
      02-ryoku-nomodeset.conf    safe graphics (nomodeset)
      03-ryoku-copytoram.conf    copy to RAM (copytoram), slow/removable USB
    syslinux/                    BIOS (syslinux), same three entries

  tests/                 install verification (run by the Install test workflow)
    README.md            what each test proves
    container-install.sh build packages, install ryoku-desktop, assert config
    install-vm.py        boot the ISO in QEMU, run the installer, assert the tree
    iso-stage-check.sh   stage twice + diff: the ISO tree is byte-reproducible
    build-ryoku-repo.sh  build a local [ryoku] repo for the VM test to serve
```

Ten backend fixtures live at the repo root (`tests/install-*.sh`) because they
guard the backend from outside `installation/`: the two partitioners
(`install-partition-whole.sh`, `install-partition-alongside.sh`), the free-region
prober (`install-largest-free.sh`), the Secure Boot preflight gate
(`install-preflight.sh`), the clock-skew heal (`install-clock-skew.sh`), the
dry-run step/sentinel matrix (`install-dryrun-matrix.sh`), the disk teardown
(`install-disk-teardown.sh`), the DNS and mirror gates (`install-dns.sh`,
`install-mirrors.sh`), and the chroot-safety scan (`install-chroot-safety.sh`).

## The flow

End to end, USB stick to first login:

1. **ISO boot.** The kernel and the archiso initramfs bring up the live system
   from the squashfs. The default boot entry is normal KMS; two fallbacks exist
   (safe graphics / copy-to-RAM).
2. **Autologin.** `agetty` logs root in on tty1 with no password
   (`getty@tty1.service.d/autologin.conf`). `/root/.zlogin` runs only on tty1.
3. **cage + foot.** `.zlogin` runs `ryoku-installer-session`, which starts a tiny
   Wayland kiosk (`cage`, software rendering) hosting a truecolor terminal
   (`foot`, JetBrains Mono Nerd Font). It relaunches on a crash so the console
   never drops mid-install; the serial console stays a plain root shell.
4. **ryoku-tui.** The terminal runs the installer. It collects keyboard, locale,
   time zone, network, hardware profile, graphics mode, window manager, one
   browser (Firefox recommended, then Chromium or Zen), one login shell (Fish
   recommended, then Zsh or Bash), the keep/remove checklist over optional apps,
   target disk, disk strategy, layout, user, and encryption, and refuses to
   proceed past its safety gates (BIOS, Secure Boot, live-medium exclusion,
   wipe acknowledgement, online).
5. **RYOKU_\* handoff.** On the Review screen `system.go` builds the `RYOKU_*`
   environment and streams `ryoku-install` (the `/usr/local/bin/ryoku-install`
   wrapper `exec`s the real backend under `/usr/local/lib/ryoku/backend`). The
   full contract is in `backend/README.md`.
6. **ryoku-install stages.** The backend runs preflight (no sentinel), then six
   sentinel-marked stages, then a few unsentineled steps:

   | Sentinel `@@RYOKU_STEP` | Runs | In |
   |---|---|---|
   | (preflight) | `ryoku_preflight`, `ryoku_ensure_dns`, `ryoku_ensure_mirrors` | preflight.sh, network.sh |
   | `partition` | `ryoku_partition`, `ryoku_luks` | disk.sh, luks.sh |
   | `filesystems` | `ryoku_filesystems` | filesystem.sh |
   | `mount` | `ryoku_mount` | filesystem.sh |
   | `pacstrap` | `ryoku_rank_mirrors`, `ryoku_pacstrap` | mirrors.sh, pacstrap.sh |
   | `configure` | `ryoku_configure`, `ryoku_deploy`, `ryoku_network`, `ryoku_drivers` | chroot.sh, deploy.sh, network.sh, drivers.sh |
   | `bootloader` | `ryoku_bootloader` | bootloader.sh |
   | (post) | `ryoku_aur`, `ryoku_bootloader_finalize`, `ryoku_snapshots` | aur.sh, bootloader.sh, snapshots.sh |

   On success it prints `@@RYOKU_DONE` and exits 0; any failure exits non-zero
   without the sentinel, and the EXIT trap names the stage and leaves `/mnt`
   mounted for inspection.
7. **Reboot.** The done screen offers reboot / power off / shell. First boot
   lands on SDDM, then the first graphical login applies the shipped default
   rice with a random wallpaper before removing its one-shot state marker.

## The two disk strategies

`RYOKU_DISK_STRATEGY` is required (no default; an empty value aborts rather than
risk a silent wipe) and decides the whole layout.

### whole: erase the disk

```
/dev/DISK  (fresh GPT, everything on the disk is erased)
+-----------------+----------------------------------------------+
| 1  ESP          | 2  root                                      |
| FAT32  "BOOT"   | btrfs  "ryoku"                               |
| EF00            | @ @home @log @pkg @snapshots [@backups]      |
| RYOKU_ESP_GIB   | takes the rest of the disk (100%)            |
+-----------------+----------------------------------------------+
```

A disk that already holds partitions needs `RYOKU_WIPE_CONFIRMED=1` (the TUI sets
it after the typed `ERASE` acknowledgement); a blank disk installs without it.

### alongside: dual-boot with automatic ESP placement

Keeps every existing partition and works beside Windows, another Linux, or an
older Ryoku. It creates a fixed 2 GiB FAT boot partition plus the Btrfs root in
the chosen contiguous free region; the user can leave that space unallocated or
let the layout step carve it from NTFS, ext2/3/4, or Btrfs.

Boot placement is automatic:

- If the existing ESP has at least 8 MiB free, the new FAT partition is an
  XBOOTLDR. The existing ESP is backed up and receives only `/EFI/ryoku/*`, a
  static hop to the XBOOTLDR.
- If the existing ESP has less than 8 MiB free, the new FAT partition is a
  dedicated Ryoku ESP. The existing ESP is never mounted read-write or modified.

Both layouts use the same free-space footprint. Windows or another Linux keeps
its firmware entry and is added to the Limine menu by the existing ESP's
partition GUID when a loader is found; managed entries survive menu updates.

Before any `wipefs`/`mkfs`, `disk.sh` proves each new partition is a real new
block device, was absent before `sgdisk`, and has the target disk as its parent;
anything else aborts. Partitions labeled exactly `ryoku`/`ryokuboot` (leftovers
of a prior failed run) abort the install unless `RYOKU_RECLAIM_LEFTOVERS=1` (the
TUI's typed `ERASE` ack) deletes only the unmounted ones, so re-runs never stack;
a mounted match is always left alone. Minimum free region is
`2 + 20 + RYOKU_SWAP_GIB` GiB (a 2 GiB boot partition plus the root floor).

## What runs where

| Where | What happens |
|---|---|
| **Build host** | `iso/build.sh`: compile `ryoku-tui` and the payload Go binaries (`ryoku-shell`, `ryoku-hub`) + the `Ryoku.Blobs` QML plugin, bake the backend and the repo payload into a staged airootfs, `mkarchiso`. Needs `go`, `cmake`, `ninja`, `archiso`, root. |
| **Live ISO** | autologin -> `ryoku-installer-session` -> `ryoku-tui` -> `ryoku-install`. Preflight, partition, LUKS, mkfs, mount, `pacstrap`, mirror ranking all run here as root on the live system. |
| **Target chroot** (`arch-chroot /mnt`) | in-target config (locale, user, HOOKS, crypttab), the per-vendor GPU driver scripts, the desktop `pacman -S` from `[ryoku]`, the AUR `yay` build, `snapper`/`limine-snapper-sync` enablement, the Limine EFI install. |
| **Installed system** | first boot: SDDM, NetworkManager, and the Ryoku session for the chosen compositor (`ryoku-gpu`, `ryoku-monitor`). Later changes arrive through `ryoku update`, not the installer. |

## How a change here reaches users

The installer runs **once, from the ISO**. A fix in `installation/` reaches only
new installs built from a **new ISO**; it never touches an existing machine
(those update through `ryoku update` from the `[ryoku]` repo). Two mechanisms
keep a long-lived ISO usable against a moving repo:

- **Payload stamp.** `build.sh` writes `/usr/share/ryoku/.payload` (commit,
  date, `VERSION`) and the deploy step warns (never fatally) when the baked
  payload has drifted from the `ryoku-desktop` version the live repo now serves.
- **Umbrella-package indirection.** The deploy step installs `ryoku-keyring` and
  the `ryoku-desktop` umbrella, not a hand-listed set. The umbrella version-pins
  and pulls every monorepo component, so an old ISO survives component renames,
  splits, and additions.

## Where to go next

- `backend/README.md` -- the `RYOKU_*` contract, dry-run, disk strategies, the
  progress protocol. `backend/lib/README.md` -- the per-stage reference.
- `iso/README.md` -- what boots, what is baked, the build, reproducibility, and
  ISO-vs-repo version skew.
- `tui/README.md` -- the installer front-end: step flow, safety gates, layout
  math, and how to run it off-ISO.
- `../docs/installation-hardware.md` -- the real-hardware playbook (VMD, Secure
  Boot, NVIDIA, Windows dual-boot, Broadcom, clock skew, NVRAM, media).
```
