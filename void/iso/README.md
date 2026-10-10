# Ryoku Void ISO

`build.sh` stages the shared installer, builds the static TUI, creates a complete
offline XBPS repository, and runs a pinned `void-mklive` in a privileged Void
container. The result is `out/ryoku-void-<version>-x86_64.iso` plus
`out/SHA256SUMS`.

Build from a local Ryoku XBPS repository:

```sh
RYOKU_VOID_ISO_RYOKU_REPO=/path/to/void-repo/x86_64 \
RYOKU_VOID_ISO_KEYRING_DIR=/path/to/void-repo/keys \
RYOKU_VOID_ISO_WORK=/mnt/ryoku-iso-tmpfs/work \
RYOKU_VOID_ISO_OUT="$PWD/void/iso/out" \
RYOKU_VOID_ISO_WORKTREE=1 \
  void/iso/build.sh
```

The keyring input is optional for a local repository when its `ryoku-keyring`
package contains the key plist. It is required for an HTTP repository. Use
`RYOKU_CONTAINER_ENGINE=podman` when appropriate. `--stage-only` stops after the
fresh rootfs tree is ready; `installation/tests/void-iso-stage-check.sh` compares
two such trees.

The live package set contains only installer tools. Target packages are resolved
from every supported lane, collected from the Void mirror downloads and the
supplied Ryoku repository, hash-checked, and indexed as one local repository. A
patch to the pinned `mklive.sh` copies that repository into the ISO tree after
SquashFS generation, preserving upstream BIOS and UEFI boot records. At boot it
is available at `/run/initramfs/live/ryoku/repo`.

Set `RYOKU_VOID_ISO_WORK`, `RYOKU_VOID_ISO_STAGE`, and
`RYOKU_VOID_ISO_OUT` to tmpfs or dedicated build storage. Peak usage depends on
the package closure: `void-mklive` briefly holds the unpacked live root, an ext3
image with twice that apparent size, SquashFS, package caches, and the offline
repository. Cached and local package archives are hardlinked into the repository
when the work paths share a filesystem. Reserve 64 GiB for work and 32 GiB for
output for a release build; the final totals are printed by both builders.
Direct `offline-repo.sh` callers may set `RYOKU_VOID_ISO_VERIFY_ROOT` to a
large, empty filesystem; `build.sh` supplies a nominal 64 GiB tmpfs because XBPS
checks free space even for dry-run transactions.
