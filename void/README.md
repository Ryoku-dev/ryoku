# Void Linux

This directory contains the x86_64 glibc Void edition. It uses runit,
Turnstile, elogind, SDDM, and niri. XBPS and runit integration stays here;
runtime code reaches it through `ryoku-host`.

Hyprland is not shipped. Ryoku requires Hyprland 0.55+, which requires GCC 15
and C++26, while Void currently ships GCC 14.

## Layout

| Path | Purpose |
|---|---|
| `packages/translations.tsv` | Maps the shared package closure to XBPS, including documented omissions. |
| `packages/sets/`, `packages/resolve` | Adds Void-only sets and resolves install lanes. |
| `packages/srcpkgs/` | Ryoku's XBPS templates and the committed `ryoku-keyring` trust plist. |
| `packages/repo/` | Builds, signs, and indexes the XBPS repository. |
| `init/` | Translates system and user services from systemd to runit and Turnstile. |
| `iso/` | Builds the Void live image and owns its install backend. |

The Arch and CachyOS definitions remain the shared source. Void files translate
host package, init, and installer behavior rather than duplicating the desktop.

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
the Void base or kernel. Testing builds from `unstable-dev` are not frozen and
have no release target to roll back to.

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

## Remaining limits

- niri only, because Ryoku's Hyprland needs GCC 15 and Void ships GCC 14
- Zen Browser, LocalSend, and Voxtype are AUR-only
- no asusctl
- Ryotunes and a few small extras are not packaged yet
- GPU passthrough needs Looking Glass and kvmfr from the AUR

## Init contract

`init/translations.tsv` records every systemd unit, target, timer, drop-in, and
environment generator, including explicit accepted losses. `tests/void-init.sh`
fails if a source unit has no row.

System services install under `/etc/sv/`. Packaged user services live under
`/usr/lib/ryoku/runit/user/` and are provisioned to
`~/.config/service/` for Turnstile. `init/session/session-start` publishes the
compositor environment, prepares the user D-Bus session, and starts the ordered
`init/session-services` roster. `init/lib/wait-for` provides bounded dependency
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

`.github/workflows/publish-repo-void.yml` publishes testing from
`unstable-dev` and frozen repositories from `v*` tags. It rebuilds
`void/releases/index.json` after a tagged publish.
`.github/workflows/build-iso-void.yml` builds images from `void-v*` tags or a
manual dispatch and refreshes the ledger after a tagged ISO upload.
`.github/workflows/release-ledger.yml` can rebuild both the Arch and Void
ledgers. The shared command is:

```sh
bin/ryoku-release-ledger <rclone-remote> arch
bin/ryoku-release-ledger <rclone-remote> void
```

The Arch ledger is `releases/index.json` and records plain and CachyOS images.
The Void ledger is `void/releases/index.json` and records Void images.
`RYOKU_RELEASE_BASE` overrides the base used for repository and ledger URLs in
tests and mirrors.

The focused checks are `tests/void-packages.sh`, `tests/void-init.sh`,
`tests/void-init-runit.sh`, and the Void installer tests under
`installation/tests/`. `.github/workflows/void.yml` runs the package and init
contracts.
