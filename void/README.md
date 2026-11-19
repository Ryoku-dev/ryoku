# Void Linux

This directory contains the experimental x86_64 glibc Void edition. It uses
runit, Turnstile, elogind, SDDM, and niri. XBPS and runit integration stays
here; runtime code reaches it through `ryoku-host`.

Hyprland is not shipped. Ryoku requires Hyprland 0.55+, which requires GCC 15
and C++26, while Void currently ships GCC 14.

For a stock Void installation followed by Ryoku's one-line installer, see
[`../docs/install-without-iso.md`](../docs/install-without-iso.md#void-linux).

## Layout

| Path | Purpose |
|---|---|
| `packages/translations.tsv` | Maps the shared package closure to XBPS, including documented omissions. |
| `../system/packages/resolve` | Resolves shared package lanes for the Void table. |
| `packages/srcpkgs/` | Ryoku's XBPS templates and the committed `ryoku-keyring` trust plist. |
| `packages/repo/` | Builds, signs, and indexes the XBPS repository. |
| `init/` | Translates system and user services from systemd to runit and Turnstile. |
| `iso/` | Builds the Void live image and owns its install backend. |

The Arch and CachyOS definitions remain the shared source. Void files translate
host package, init, and installer behavior rather than duplicating the desktop.

## Live ISO

The live medium uses Limine for both BIOS and UEFI boot. The image is hybrid:
the same file boots when attached as an optical disc or written byte-for-byte
to a USB drive. Use `dd` or Rufus in DD mode; Ventoy's normal and grub2 modes
are also supported.

Secure Boot must be disabled. The live Limine EFI executable and kernel are
unsigned, as they are on the Arch live image.

On a machine booted in legacy BIOS mode the installer uses the whole disk: it
creates a GPT BIOS boot partition for Limine's disk stage and a FAT `/boot`.
Installing alongside another system needs UEFI. Each kernel install refreshes
the Limine loader, and on a BIOS install the disk stage with it.

## Update and rollback

The ISO installer creates the `@snapshots` subvolume at `/.snapshots` when the
Snapshots & rollback toggle is enabled. It is on by default. Turning it off
writes `/etc/ryoku/snapshots-disabled`, the same opt-out marker Arch uses.

Void uses the shared snapper root config: numbered cleanup, 10 snapshots kept,
and no timeline snapshots. The `snapper-cleanup` runit service performs daily
cleanup. Ryoku builds `limine-snapper-sync` 1.32.1 as a native image with
Void's Mandrel and runs its watcher as a runit service. It maintains the
**Ryoku Linux -> Snapshots** submenu in Limine.

Every `ryoku update`, including `ryoku update --system`, takes a snapper pre/post
pair. XBPS has no transaction hooks, so a direct `xbps-install -Su` is not
snapshotted. Use `ryoku update --system` to update the Void base and kernel
inside the safety net.

`ryoku rollback` lists tagged releases from
`https://repo.ryoku.dev/stable/void/releases/index.json`.
`ryoku rollback --to <tag>` and `ryoku track <tag>` move the Ryoku package set
to that frozen signed release in one forced XBPS transaction. They do not move
the Void base or kernel. The latest ten testing builds from `unstable-dev` are
also frozen and listed in the testing ledger for pinning and rollback.

The `ryoku-boot-guard` runit service watches a tagged release update. After two
boots where the desktop never came up, it returns the Ryoku set to the previous
release. On a third failed boot it points Limine at the pre-update snapshot.

To restore the whole system, reboot and choose **Ryoku Linux -> Snapshots** in
Limine. Boot the snapshot, run `sudo limine-snapper-restore`, then reboot.

The kernel post-install hook (`/etc/kernel.d/post-install/50-ryoku-limine`,
source `system/boot/limine/50-ryoku-limine`) writes `/boot/limine.conf` as a
**Ryoku Linux** tree with one entry per installed kernel series and its dracut
initramfs. It preserves the Snapshots subtree owned by `limine-snapper-sync`.

The shell installer does not provision a bootloader or snapshot layout on an
existing Void machine. Snapshot restore works when that machine already has a
btrfs root and snapper; `ryoku doctor` converges the root snapper config.

## Gaming

The Void ISO has a Gaming question after Rashin. It defaults to No and works
offline. Choosing Yes installs Steam from Void nonfree plus the 32-bit graphics
baseline: libgcc, libstdc++, libdrm, libglvnd, libva, Mesa DRI, and the Vulkan
loader. The existing hardware lanes add the matching 32-bit Vulkan package for
AMD, Intel, nouveau, or a VM. NVIDIA follows the installed driver's branch and
adds its matching 32-bit libraries.

The script installer offers the same set interactively, or through `--gaming`
and `RYOKU_GAMING=1`.

This is Void gaming, not the CachyOS layer. There is no maintained
`linux-cachyos` XBPS package, and Void's stock kernel has sched-ext disabled, so
the CachyOS kernel, sched-ext tuning, and Proton-CachyOS are not available.
Steam uses its own Proton.

## Remaining limits

- niri only, because Ryoku's Hyprland needs GCC 15 and Void ships GCC 14
- Zen Browser, LocalSend, and Voxtype are AUR-only
- no asusctl or adw-gtk-theme package
- GPU passthrough needs Looking Glass and kvmfr from the AUR

## Init contract

`init/translations.tsv` records every systemd unit, target, timer, drop-in, and
environment generator, including explicit accepted losses. `tests/void-init.sh`
fails if a source unit has no row.

Wi-Fi on Void runs NetworkManager over iwd like the other editions, with one
runit-only piece: Void's iwd has no D-Bus activation file, so iwd runs as a
supervised service while `/var/lib/ryoku/wifi-iwd.enabled` exists (the
manifest's `wifi-iwd` row). The installer records the marker for the backend it
pins and `ryoku-wifi-backend` maintains it on every switch, so a box moved to
wpa_supplicant stops supervising iwd and one moved back starts again.

System services install under `/etc/sv/`. Packaged user services live under
`/usr/lib/ryoku/runit/user/` and are provisioned to
`~/.config/service/` for Turnstile. `system/services/manifest.tsv` owns their
activation and order. `ryoku-host svc reconcile` writes the per-user roster;
`init/session/session-start` publishes the compositor environment, reconciles
that roster, and starts it. `init/lib/wait-for` provides bounded dependency
waits because runit has no declarative ordering.

Ryoku configures Turnstile, PAM, elogind-owned `/run/user`, and the user D-Bus
service during installation. `ryoku-host session ensure` and doctor repair
later drift. Doctor reads runit's supervise state when reporting failed
services and converges the snapper root config on btrfs systems. Service scripts
must use the host seam rather than adding distro checks to callers.

## Build and release

Build the XBPS repository with `packages/repo/container-runner.sh`; see
[`packages/README.md`](packages/README.md). Build the ISO with `iso/build.sh`;
see [`iso/README.md`](iso/README.md).

`stable-release.yml` dispatches `.github/workflows/publish-repo-void.yml` on
each `v*` release tag. The publish workflow writes the frozen repository and
stable pointer, rebuilds `void/releases/index.json`, and then dispatches
`.github/workflows/build-iso-void.yml` from that exact frozen repository. The
ISO workflow signs and uploads the image plus `latest-void.json` and
`latest-void.js`, then refreshes the Void ledger so its `images.void` entry
names the image. An `unstable-dev` push publishes and freezes testing instead.
`.github/workflows/release-ledger.yml` can rebuild the Arch, Void, and Fedora
stable and testing ledgers. The shared command is:

```sh
bin/ryoku-release-ledger <rclone-remote> arch
bin/ryoku-release-ledger <rclone-remote> void
bin/ryoku-release-ledger <rclone-remote> fedora/44
```

The Arch ledger is `releases/index.json` and records plain and CachyOS images.
The Void ledger is `void/releases/index.json` and records Void images.
`RYOKU_RELEASE_BASE` overrides the base used for repository and ledger URLs in
tests and mirrors.

The focused checks are `tests/void-packages.sh`, `tests/void-init.sh`,
`tests/void-init-runit.sh`, and the Void installer tests under
`installation/tests/`. `.github/workflows/void.yml` runs the package and init
contracts.
