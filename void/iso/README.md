# Ryoku Void ISO

The Void image boots straight into the same graphical installer as the Arch
image. Before setup starts, it shows the short `Ryoku on Void` note covering
niri-only support, missing snapshots and rollback, and unavailable packages.
The ISO carries a complete offline XBPS repository and installs x86_64 glibc
Void plus Ryoku, SDDM, niri, Limine, and dracut.

`build.sh` stages the shared installer, builds its static TUI, assembles the
offline repository, and runs a pinned `void-mklive` in the official Void
container. Build from a completed Ryoku repository:

```sh
RYOKU_VOID_ISO_RYOKU_REPO=/path/to/void-repo/x86_64 \
RYOKU_VOID_ISO_KEYRING_DIR=/path/to/key-plists \
RYOKU_VOID_ISO_WORK=/tmp/ryoku-void-iso \
RYOKU_VOID_ISO_OUT="$PWD/void/iso/out" \
RYOKU_VOID_ISO_WORKTREE=1 \
  void/iso/build.sh
```

The keyring directory is optional when the local repository's
`ryoku-keyring` package already contains its plist and required for an HTTP
repository. Set `RYOKU_CONTAINER_ENGINE=podman` to use Podman.
`--stage-only` stops after preparing the root tree. The result is
`out/ryoku-void-<version>-x86_64.iso` plus `out/SHA256SUMS`.

For a release build, reserve 64 GiB for work and 32 GiB for output.

`.github/workflows/build-iso-void.yml` runs the preflight and build on
`void-v*` tags or manual dispatch. It verifies that the plist derived from the
`VOID_REPO_SIGNING_KEY` secret matches the committed
`void/packages/srcpkgs/ryoku-keyring/files/` entry before building.
