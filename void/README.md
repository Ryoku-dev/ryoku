# Void Linux

This directory contains the x86_64 glibc Void edition. It uses runit,
Turnstile, elogind, SDDM, and niri. XBPS and runit integration stays here;
runtime code reaches it through `ryoku-host`.

Hyprland is not shipped. Ryoku requires Hyprland 0.55+, which requires GCC 15
and C++26, while Void currently ships GCC 14. Snapshots and boot-menu rollback
are also unavailable because Ryoku's snapper and `limine-snapper-sync` stack is
tied to pacman.

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
services and skips snapshot checks with the single unsupported reason. Service
scripts must use the host seam rather than adding distro checks to callers.

## Build and release

Build the XBPS repository with `packages/repo/container-runner.sh`; see
[`packages/README.md`](packages/README.md). Build the ISO with `iso/build.sh`;
see [`iso/README.md`](iso/README.md).

`.github/workflows/publish-repo-void.yml` publishes testing from
`unstable-dev` and stable from release tags.
`.github/workflows/build-iso-void.yml` builds images from `void-v*` tags or a
manual dispatch.

The focused checks are `tests/void-packages.sh`, `tests/void-init.sh`,
`tests/void-init-runit.sh`, and the Void installer tests under
`installation/tests/`. `.github/workflows/void.yml` runs the package and init
contracts.
