# Void packages

This directory is the XBPS side of Ryoku's package definition. It mirrors
`system/packages/` without maintaining a second desktop package list: the Arch
names remain the common contract and `translations.tsv` says how each name is
provided on Void.

- `translations.tsv` covers the complete Arch package closure. It includes the
  four lists under `system/packages/`, the transitive `depends` and `optdepends`
  closure of the neutral, niri, and Hyprland desktop packages, and
  `ryoku-keyring`.
- `sets/session.packages` contains the Void login and service foundation that an
  Arch system gets from its base system rather than from that closure.
- `sets/build.packages` contains the toolchain needed for source installs.
- `resolve` selects lanes, applies Arch-name drops, adds the optional sets, and
  prints sorted unique XBPS package names.

The table's `void` column is either one or more package names, `@repo` for a
Ryoku-owned monorepo package built from the checkout, or `-` for a documented
absence. Plain mappings use an empty `notes` column for Void's main repository.
Packages built by Ryoku from pinned third-party source use `repo=ryoku`.
Packages outside main use `repo=multilib`, `repo=nonfree`, or
`repo=multilib-nonfree`; the resolver adds the matching `void-repo-*` enabler.
`@repo` and `repo=ryoku` rows resolve to their package names because packaged
installs receive both from the Ryoku repository. Void uses `swayidle` for the
Arch `hypridle` row. Hyprland and its variant-only closure are recorded but
unavailable because Void does not package Hyprland.

## Lanes

- `desktop`: the shared shell and niri desktop runtime
- `dev`: developer tools
- `system`: the boot and base-system entries the shell installer does not add to
  an existing machine
- `extra`: best-effort packages from `aur.packages`
- `optional`: desktop `optdepends` not already assigned elsewhere
- `hyprland`: the unavailable Hyprland-only variant
- `hardware:amd`, `hardware:intel`, `hardware:nvidia`, `hardware:vm`: the
  matching sections of `hardware.packages`. `hardware:nvidia` leaves
  proprietary driver selection to `system/hardware/drivers/nvidia.sh`, which
  enables Void's nonfree repositories and installs `nvidia`, `nvidia580`, or
  `nvidia470` for the detected GPU while retaining nouveau when no matching
  module builds.

For example:

```sh
void/packages/resolve --lane desktop --lane dev --session --build
void/packages/resolve --lane desktop --lane hardware:amd --drop chromium
```

`cachyos.packages` is intentionally absent. It is an Arch-only overlay, not part
of the shared package closure.

## Package repository

`srcpkgs/` is the Ryoku overlay for a pinned clone of `void-packages`. The
container runner computes the release identity and source archive on the host,
so ordinary clones and linked Git worktrees behave the same even though the
container cannot see an external worktree Git directory. `repo/build-repo.sh`
stamps that archive's version and checksum into the overlay copy, bootstraps
xbps-src, builds packages in dependency order, and writes an indexed repository
under `repo/out/x86_64/`. It never changes the committed templates. The directory
contains the `.xbps` files, optional `.xbps.sig2` signatures,
`x86_64-repodata`, and `release.json`.

Run the build through the official Void glibc image:

```sh
void/packages/repo/container-runner.sh
```

The runner uses a privileged container with `/dev` mounted, but xbps-src itself
runs as the non-root `builder` user with `uchroot`. Override scratch and output
locations with `RYOKU_XBPS_WORK` and `RYOKU_XBPS_OUT`. The scratch directory's
external `hostdir/` persists built packages and downloaded sources across fresh
void-packages clones, so xbps-src skips an exact version already in its binary
repository. Set `RYOKU_XBPS_PACKAGES` to a whitespace-separated subset; required
overlay dependencies are included and only that closure is copied to the final
repository. Set `RYOKU_XBPS_WORKTREE=1` to archive tracked files plus untracked,
non-ignored files instead of `HEAD`. Worktree package versions gain a `.wt`
suffix from the source archive hash, so unchanged source reuses cached packages
while an edit produces a new version.

Unsigned output is suitable for a local file repository. To sign repository
metadata and every package, pass an RSA PEM key:

```sh
RYOKU_XBPS_KEY=/secure/ryoku-xbps.pem \
  void/packages/repo/container-runner.sh
```

`XBPS_PASSPHRASE` supplies its passphrase. The private key is copied into the
container's private work area and is never put in the repository output.

The `ryoku-keyring` template refuses to build until its `files/` directory
contains a fingerprint-named plist. The maintainer creates the production key
and plist with:

```sh
void/packages/repo/new-signing-key /secure/ryoku-xbps.pem
```

The filename is XBPS's lower-case, colon-separated MD5 fingerprint of the SSH
RSA wire key, without the `MD5:` prefix. Before publishing, sync the signed
repository in an isolated XBPS root, accept the offered key, and byte-compare
the imported `/var/db/xbps/keys/*.plist` with the generated plist. Local and CI
builds can substitute an uncommitted development plist directory through
`RYOKU_XBPS_KEYRING_DIR`.

The keyring package installs the trusted plist and
`/usr/share/xbps.d/20-ryoku.conf`, which points at
`https://repo.ryoku.dev/stable/void/x86_64`. An administrator selects another
channel by writing `/etc/xbps.d/20-ryoku.conf`; XBPS gives that same-basename
file precedence.

`tests/void-packages.sh` recomputes the Arch package closure, validates table
semantics and template coverage, and checks that each template covers the
translated runtime dependencies of its PKGBUILD. Its optional `--repo` mode
also verifies declarations for packages supplied by official Void repositories.
