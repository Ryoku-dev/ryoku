# Void packages

This directory translates Ryoku's shared package definition to XBPS without
maintaining a second desktop list.

- `translations.tsv` maps each Arch package and lane to one or more Void
  packages, `@repo` for a Ryoku-built package, or `-` with the reason it is
  unavailable.
- `sets/session.packages` adds the runit, Turnstile, elogind, and login
  foundation. `sets/build.packages` adds source-build tools.
- `resolve` selects lanes, drops requested packages, enables required Void
  repositories, and prints a sorted XBPS set.
- `srcpkgs/` contains the Ryoku package templates. The
  `limine-snapper-sync` 1.32.1 template builds a native image with Void's
  Mandrel and installs its runit watcher.

The main lanes are `desktop`, `dev`, `system`, `extra`, `optional`, and the four
`hardware:*` lanes. The recorded `hyprland` lane is unavailable: Ryoku needs
Hyprland 0.55+ and GCC 15, while Void ships GCC 14. The table also records
unavailable packages such as Zen Browser, LocalSend, Voxtype, asusctl, and
Ryotunes.

The NVIDIA hardware lane leaves driver selection to
`system/hardware/drivers/nvidia.sh`. It picks Void's `nvidia`, `nvidia580`, or
`nvidia470` package for the detected GPU and keeps nouveau if no matching
module builds.

```sh
void/packages/resolve --lane desktop --lane dev --session --build
void/packages/resolve --lane desktop --lane hardware:amd --drop chromium
```

## Build the repository

Use the official Void glibc container:

```sh
RYOKU_XBPS_WORK=/tmp/ryoku-xbps-work \
RYOKU_XBPS_OUT=/tmp/ryoku-xbps-out \
RYOKU_XBPS_WORKTREE=1 \
  void/packages/repo/container-runner.sh
```

`RYOKU_XBPS_PACKAGES` limits the build to a whitespace-separated package list
and its overlay dependencies. `RYOKU_XBPS_WORKTREE=1` includes current tracked
files plus untracked, non-ignored files. The output under `x86_64/` contains the
packages, repository index, signatures when enabled, and `release.json`.

Unsigned output is suitable for a local file repository. Sign a development
repository with `RYOKU_XBPS_KEY=/path/to/key.pem`; use
`RYOKU_XBPS_KEYRING_DIR` only to supply that development key's plist.

For production, create or derive the committed trust plist once:

```sh
void/packages/repo/new-signing-key /secure/ryoku-xbps.pem
RYOKU_XBPS_KEY=/secure/ryoku-xbps.pem \
RYOKU_XBPS_PRODUCTION=1 \
  void/packages/repo/container-runner.sh
```

The helper writes a fingerprint-named plist to
`srcpkgs/ryoku-keyring/files/`. Commit the plist, not the private key.
Production mode requires the private key, rejects a keyring override, and
checks that `ryoku-keyring` contains exactly the committed plist. CI stores the
private key in `VOID_REPO_SIGNING_KEY` and derives the plist again before every
repository or ISO build; a missing or different committed file stops the
workflow.

`ryoku-keyring` installs the trust key and the stable configuration for
`https://repo.ryoku.dev/stable/void/x86_64`. Every `unstable-dev` push publishes
testing at
`https://repo.ryoku.dev/stable/void/channels/testing/x86_64`. A `v*` tag creates
the frozen `void/releases/<tag>/x86_64/` repository and moves the stable
repository. Testing builds are not frozen release targets.

`.github/workflows/publish-repo-void.yml` owns that publish flow and rebuilds
`void/releases/index.json` after a tagged publish. The shared ledger tool can
rebuild it directly:

```sh
bin/ryoku-release-ledger <rclone-remote> void
```

`.github/workflows/build-iso-void.yml` refreshes the ledger after a tagged ISO
upload, while `.github/workflows/release-ledger.yml` rebuilds both the Arch and
Void ledgers. `RYOKU_RELEASE_BASE` overrides both repository and ledger URL
bases for tests and mirrors.

`tests/void-packages.sh` checks the translated closure, lane semantics,
templates, and optional live Void repository resolution.
