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
Ryoku-owned package built from the checkout, `@fetch` for a pinned asset fetched
by the source installer, or `-` for a documented absence. Plain mappings use an
empty `notes` column for Void's main repository. A package outside main declares
`repo=multilib`, `repo=nonfree`, or `repo=multilib-nonfree` there. The resolver
then includes the matching `void-repo-*` enabler in its output; installers put
those enablers through a first `xbps-install -Sy` pass before installing the
remaining package plan. Void uses `swayidle` for the Arch `hypridle` row.
Hyprland and its variant-only closure are recorded but unavailable because Void
does not package Hyprland.

## Lanes

- `desktop`: the shared shell and niri desktop runtime
- `dev`: developer tools
- `system`: the boot and base-system entries the shell installer does not add to
  an existing machine
- `extra`: best-effort packages from `aur.packages`
- `optional`: desktop `optdepends` not already assigned elsewhere
- `hyprland`: the unavailable Hyprland-only variant
- `hardware:amd`, `hardware:intel`, `hardware:nvidia`, `hardware:vm`: the
  matching sections of `hardware.packages`

For example:

```sh
void/packages/resolve --lane desktop --lane dev --session --build
void/packages/resolve --lane desktop --lane hardware:amd --drop chromium
```

`cachyos.packages` is intentionally absent. It is an Arch-only overlay, not part
of the shared package closure.

## Repository layout

The planned `srcpkgs/` directory mirrors `release/packages/`: one xbps-src
template per Ryoku package. The planned `repo/` directory mirrors
`release/repo/`: build, sign, index, and publish the Ryoku XBPS repository. Until
those templates land, `@repo` components are built from the checkout by the
source installer.

`tests/void-packages.sh` recomputes the closure from the source lists and
PKGBUILDs. Adding a package under `system/packages/`, or adding a reachable
PKGBUILD dependency, fails the static test until exactly one Void translation is
added. Its `--repo` mode fetches the x86_64 glibc main, multilib, nonfree, and
multilib/nonfree indexes independently. It checks both that each mapped package
declares its real repository and that the resolver emits the enabler needed to
reach it.
