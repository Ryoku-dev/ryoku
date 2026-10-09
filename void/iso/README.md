# Void ISO

This is the build contract for Ryoku's x86_64 glibc Void image. It mirrors
`installation/iso/`: the image boots into the shared installer, carries a
complete offline target repository, and installs the same desktop. Void-specific
work is limited to image assembly, XBPS, runit, and the target configuration
commands that differ from Arch.

## Live image

The image is assembled with `void-mklive`. Its profile contains:

- a live-only package set for the kernel, firmware, storage and filesystem
  tools, networking, `xchroot`, `dracut`, Limine, `cage`, `foot`, and the font
  used by the installer;
- a tty1 `agetty` service that logs root in to the installer session, which
  launches the prebuilt `installation/tui` binary under `cage` and `foot` and
  relaunches it after a crash;
- the shared `installation/backend`, the tracked repository payload, the
  prebuilt Go binaries and QML plugin, and the Void init files;
- a signed local XBPS repository containing the complete target dependency
  closure.

The live-only set is separate from the installed desktop set, just as
`installation/iso/packages.x86_64` is separate from `system/packages/`. Target
packages come from `void/packages/resolve`; adding a desktop package never
silently turns it into a live-image dependency.

The serial console and additional VTs remain root recovery shells. The graphical
path uses the same `cage` and `foot` presentation as the Arch image, with
software rendering, and exports the same `RYOKU_*` installer contract.

## Offline XBPS repository

The Void counterpart of `installation/iso/offline-repo.sh` is built before
`void-mklive` assembles the image:

1. Run `void/packages/resolve` for `system`, `desktop`, `dev`, and every
   `hardware:amd`, `hardware:intel`, `hardware:nvidia`, and `hardware:vm` lane,
   adding the session set. The image therefore supports every hardware choice
   without a network connection.
2. Configure a clean XBPS root against the official Void repositories and the
   Ryoku XBPS repository. Use `xbps-install -D` to download the resolved
   dependency closure into a dedicated package cache.
3. Copy only the selected `.xbps` files into the image's local repository and
   index them with `xbps-rindex -a`.
4. Sign packages and the repository index with the release key using
   `xbps-rindex --sign-pkg` and `xbps-rindex --sign`. The public key is baked
   into the live image and installed target.
5. Prove the repository is complete by installing the full resolved set into an
   empty temporary root with every network repository disabled.

The backend points the target transaction at this repository and uses
`xbps-install -r /mnt`; it never reaches a mirror after disk changes begin.
Superseded package files are pruned after indexing so repeated builds do not
grow the image from cache history.

Until the signed Ryoku XBPS repository exists, `@repo` rows are supplied from
the checked-out payload. The image includes `void/packages/sets/build.packages`,
builds those components through `ryoku/shell/deploy.sh`, and converges the rest
of the target from the offline repository. This is the same from-source path
used by the Void script installer, not a second packaging model.

## Shared installer and Void routing

The image reuses `installation/tui` unchanged. It also reuses
`installation/backend/ryoku-install` and its progress sentinels; a distro route
selects the command implementation for each host operation. Disk layout,
encryption, filesystem policy, user choices, and failure handling remain one
code path.

| Backend concern | Void equivalent |
|---|---|
| `common.sh` and progress protocol | Unchanged command wrappers, dry-run behavior, sentinels, and failure trap |
| `preflight.sh` | The same firmware, disk, Secure Boot, and payload gates, plus checks for XBPS, `xchroot`, `dracut`, and the signed local repository |
| `disk.sh` and `resize.sh` | Unchanged GPT, whole-disk, alongside, ESP, and resize operations |
| `luks.sh` | Unchanged LUKS2 layout and mapper naming |
| `filesystem.sh` | Unchanged Btrfs subvolumes, FAT boot filesystem, mounts, and swapfile |
| `offline.sh` | Verify the local XBPS signature and prove the requested package set resolves with network repositories disabled |
| `mirrors.sh` | Write `/etc/xbps.d` repository configuration; the install transaction uses the image's local repository |
| `pacstrap.sh` | Resolve the selected Void lanes and run `xbps-install -r /mnt` instead of `pacstrap` |
| `chroot.sh` | Run target commands with `xchroot` instead of `arch-chroot`; install `glibc-locales` and reconfigure it instead of `locale-gen`; generate initramfs images with `dracut` instead of `mkinitcpio`; create runit enablement links instead of calling `systemctl enable` |
| Locale, timezone, hostname, users, and sudoers | Keep the same values and files; user and sudoers creation is identical |
| `deploy.sh` | Install Ryoku from the signed XBPS repository when available, otherwise build from the recorded checkout through the existing source lane |
| `network.sh` | Carry NetworkManager and iwd state into the target as today, then enable packaged services with `/etc/runit/runsvdir/default` links |
| `drivers.sh` | Select the same detected hardware profile, resolve its Void package names, install them through XBPS, and use the Void driver implementation |
| `bootloader.sh` | Install and configure Limine with the same ESP, XBOOTLDR, encryption, resume, and chainload policy; the Limine path is otherwise identical, but enumerates dracut images instead of mkinitcpio images |
| `aur.sh` | No AUR transaction; `@fetch` assets are installed by their pinned fetch step and source-only Ryoku components are built by deploy |
| `snapshots.sh` | Configure Snapper directly when selected; omit Arch pacman hooks and regenerate Limine entries from the resulting snapshots |
| `cachyos.sh` | Not selected on Void |
| Final cleanup | Sync, unmount, and emit the same `@@RYOKU_DONE` sentinel |

The init payload comes from `void/init/`. System services are installed below
`/etc/sv` and enabled by links in `/etc/runit/runsvdir/default`; per-user
services are installed for Turnstile. Runtime operations go through
`ryoku-host`, so the backend does not introduce scattered runit or XBPS checks.

## Release workflow

`.github/workflows/build-iso-void.yml` is the Void counterpart of
`build-iso-cachyos.yml`. It is manually dispatchable and runs for `void-v*`
tags. It keeps an independent run number and publishes a Void-specific image
entry rather than sharing the Arch or CachyOS artifact.

The workflow:

1. runs the package-table, init translation, installer-contract, and offline
   closure preflight gates;
2. builds in a clean x86_64 glibc Void environment, installs the build set, and
   invokes the `void-mklive` profile;
3. verifies the payload commit and Void variant marker in the finished image;
4. scans the extracted root filesystem, writes checksums, and signs the image;
5. uploads the image, signature, checksum, and manifest to the release bucket
   and updates the Void release pointer.

This mirrors the CachyOS workflow's checkout, provenance, signing, scanning, and
publication contract. It does not call the Arch reusable builder because
`void-mklive` and XBPS require a Void build environment.

## Inputs already in place

- `void/packages/translations.tsv`, including the `system`, `desktop`, `dev`,
  and hardware lanes;
- `void/packages/resolve`;
- `void/packages/sets/session.packages` and
  `void/packages/sets/build.packages`;
- system service translations under `void/init/system/` and the rest of the
  runit and Turnstile session payload under `void/init/`;
- `ryoku-host`, implemented by `ryoku/cli/cmd/ryoku-host` and
  `ryoku/cli/internal/host`.

## Open prerequisite

The release image ultimately needs the signed Ryoku XBPS repository represented
by `void/packages/srcpkgs/` and `void/packages/repo/`. Until that repository is
published, the image uses the source-build path described above, exactly like
the Void script installer.
