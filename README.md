<div align="center">

<img src="ryoku/assets/brand/logo-mark-v2.png" alt="Ryoku" width="140" />

# Ryoku

**力と美のために** &middot; *For the sake of power and beauty.*

A hand-built Linux desktop: one shell, three compositors, four distributions,
all from one repository.

[![License: GPL-3.0](https://img.shields.io/badge/license-GPL--3.0-E2342A?style=for-the-badge)](LICENSE)
[![Release status](https://img.shields.io/badge/dynamic/json?url=https%3A%2F%2Fiso.ryoku.dev%2Fstable%2Flatest.json&query=%24.channel&label=status&color=E2342A&style=for-the-badge)](https://ryoku.dev)
[![Build ISO](https://github.com/ryoku-dev/ryoku/actions/workflows/build-iso.yml/badge.svg)](https://github.com/ryoku-dev/ryoku/actions/workflows/build-iso.yml)
[![Discord](https://img.shields.io/badge/Discord-join-5865F2?style=for-the-badge&logo=discord&logoColor=white)](https://discord.gg/8KjBmUEyKA)
[![Reddit](https://img.shields.io/badge/Reddit-r%2FRyokuArch-FF4500?style=for-the-badge&logo=reddit&logoColor=white)](https://www.reddit.com/r/RyokuArch/)

[![Arch Linux](https://img.shields.io/badge/Arch_Linux-0d0d0d?style=for-the-badge&logo=archlinux&logoColor=white)](#editions)
[![CachyOS](https://img.shields.io/badge/CachyOS-0d0d0d?style=for-the-badge&logo=cachyos&logoColor=white)](#editions)
[![Void Linux](https://img.shields.io/badge/Void_Linux-0d0d0d?style=for-the-badge&logo=voidlinux&logoColor=white)](#editions)
[![Fedora](https://img.shields.io/badge/Fedora-0d0d0d?style=for-the-badge&logo=fedora&logoColor=white)](#editions)
[![Hyprland](https://img.shields.io/badge/Hyprland-0d0d0d?style=for-the-badge&logo=hyprland&logoColor=white)](https://hypr.land)
[![niri](https://img.shields.io/badge/niri-0d0d0d?style=for-the-badge&logo=niri&logoColor=white)](https://github.com/YaLTeR/niri)
[![Wayfire](https://img.shields.io/badge/Wayfire-0d0d0d?style=for-the-badge)](https://wayfire.org)

<kbd>[Download](https://ryoku.dev)</kbd> &middot; <kbd>[Showroom](https://ryoku.dev/showroom)</kbd> &middot; <kbd>[What Ryoku is](docs/ryoku.md)</kbd> &middot; <kbd>[Docs](docs/)</kbd> &middot; <kbd>[Contributing](CONTRIBUTING.md)</kbd>

</div>

<img src="docs/media/showroom/blue-hour.webp" alt="A Ryoku desktop at blue hour: the bar along the top, a large clock on the wallpaper, nothing else asking for attention" width="100%" />

<table>
  <tr>
    <td width="33%"><img src="docs/media/showroom/red-august.webp" alt="A samurai under a red sun, the date in the corner" width="100%" /><br /><sub>Red August</sub></td>
    <td width="33%"><img src="docs/media/showroom/rose.webp" alt="A pink room with the clock, system readings and a music card" width="100%" /><br /><sub>Rose</sub></td>
    <td width="33%"><img src="docs/media/showroom/teal.webp" alt="A teal figure with a red saber, a good-night card" width="100%" /><br /><sub>Teal</sub></td>
  </tr>
  <tr>
    <td width="33%"><img src="docs/media/showroom/aether.webp" alt="A tan room with the Hub profile and a terminal open" width="100%" /><br /><sub>Aether</sub></td>
    <td width="33%"><img src="docs/media/showroom/olive-dusk.webp" alt="An olive desktop with the clock, a calendar and a visualizer" width="100%" /><br /><sub>Olive dusk</sub></td>
    <td width="33%"><img src="docs/media/showroom/katakana-red.webp" alt="A horned silhouette on red, a music card and a visualizer" width="100%" /><br /><sub>Katakana red</sub></td>
  </tr>
</table>

<div align="center"><sub>Real desktops, from the <a href="https://ryoku.dev/showroom">showroom</a>. The bar, the widgets and the frame all retint from the wallpaper.</sub></div>

## What it is

Ryoku means power, and the name is the point. The desktop is one continuous
surface: the bar, the panels, the launcher, the lockscreen and the session
controls move as a single thing, cut stone in the colours of your wallpaper,
with the frame retinting live from it. Click a widget on the bar and its
controls grow out of the rail; let go and they melt back. It is built from
small independent surfaces, so it grows with what you actually use: seven bar
styles, desktop widgets, a store for more, and a plugin host.

Rashin is the optional AI layer: Ask answers read-only questions about the
machine through free models, Agent connects external coding harnesses, and
Index searches projects and system code. It is off by default and uses no
process, socket, timer or watcher until you turn it on.

Underneath, Ryoku is a distribution, not a config dump. The desktop, the
installer and the system definition live in this repository, every machine is
built from it, and a live machine is only ever a deployment target. It is a
community-first passion project led by one maintainer in their spare time and
improved by generous contributors, and it favours a rich, deliberate desktop on
capable hardware over a lightweight one.

Ryoku 1.0, Amaterasu, is the first stable release. Arch and CachyOS are the
stable editions. Void and Fedora ship the same desktop and update path, but
remain experimental while those editions get wider hardware coverage.

## Editions

| Edition | Compositors | Get it | Status |
|---|---|---|---|
| **Arch Linux** | Hyprland, niri, Wayfire | signed ISO, or the [one-liner on an existing install](docs/install-without-iso.md#existing-arch-linux) | stable |
| **CachyOS** | Hyprland, niri, Wayfire | signed ISO, or [the one-liner](docs/install-without-iso.md#existing-cachyos) | stable |
| **Void Linux** glibc, runit | niri | signed ISO, or [the one-liner](docs/install-without-iso.md#void-linux) | experimental |
| **Fedora** 44 and 45 | Hyprland, niri | [the one-liner, on Workstation, KDE Plasma, Server or a spin](docs/install-without-iso.md#existing-fedora-44-or-45) | experimental |
| **NixOS** | Hyprland, niri | [Ryoku on NixOS](https://github.com/Ryoku-dev/Ryoku-on-NixOS), a port for flake-based systems | community |

Every edition is `x86_64` and ships the same shell from a signed native
repository (pacman, XBPS or RPM). The Arch edition keeps an Arch userland and
can opt into **Gaming with CachyOS** during installation or add the same
**CachyOS Gaming** bundle from RyoStore later. The CachyOS ISO is CachyOS from
the live kernel through the installed system: CachyOS repositories, kernels,
identity, tuning, hooks, and tools with Ryoku as its desktop. The Void ISO can
add Steam and its 32-bit graphics stack with the installer Gaming question.

Hyprland and niri are the first-class compositors; Wayfire 0.11 is the
experimental third on Arch and CachyOS, reached by switching desktops in the
Hub or with `ryoku wm use wayfire`. Void is niri-only until it ships GCC 15
([why](void/README.md)). Fedora converts an existing mutable system in place,
keeps GRUB with a Ryoku snapshots submenu, and leaves NVIDIA on nouveau
([details](fedora/README.md)).

The [no-ISO guide](docs/install-without-iso.md) covers each existing-system
path, starting from a stock Void installation.

## The desktop

<table>
  <tr>
    <td width="33%"><img src="docs/media/launcher.webp" alt="The launcher" width="100%" /><br /><sub><b>Launcher.</b> At rest a clock, the weather and a plate of art; type, and apps, commands, files, packages, radio and the calculator come out of one search.</sub></td>
    <td width="33%"><img src="docs/media/controls.webp" alt="The control sidebar" width="100%" /><br /><sub><b>Control sidebar.</b> Session, connect tiles, sound and brightness, media, calendar and the power profile on one rail.</sub></td>
    <td width="33%"><img src="docs/media/profile.webp" alt="The Ryoku Hub" width="100%" /><br /><sub><b>Ryoku Hub.</b> Every setting, a live dossier of the machine, the store, and the desktop switch.</sub></td>
  </tr>
</table>

On first login a short welcome walks you through the handful of keybinds that
open almost everything and lets you pick the interface scale, the bar and the
desktop widgets. Everything else lives in the Ryoku Hub (`Super + ,`). The bar
is a choice, not a given: Ryoku ships two subshells, **Orpheus** (a liquid top
bar, and the fallback) and **Kairos** (one island carrying the clock, worn by
the default rice), and RyoStore
installs the rest, among them **QS Bar** (in full colour), **Sumi** (the
monochrome rail), **Shima**, **Python** and **Nomarchy**. Every one of them reads
the desktop through the same window-manager seam rather than a compositor's own
API, and an installed one costs nothing until you pick it
([`docs/subshells.md`](docs/subshells.md)).

<div align="center">
  <a href="https://youtu.be/kx7VW4Mg0m4"><img src="https://img.youtube.com/vi/kx7VW4Mg0m4/maxresdefault.jpg" alt="Ryoku showcase on YouTube" width="560" /></a>
  <br /><sub>&#9654; <a href="https://youtu.be/kx7VW4Mg0m4">Watch the showcase</a></sub>
</div>

## Install

**From the ISO.** Download the Arch, CachyOS or Void image from
[ryoku.dev](https://ryoku.dev) with its signature, write it to a USB stick and
boot it. The guided installer partitions the disk, installs the base system and
the desktop from the signed repository, sets up Limine, and configures Btrfs
snapshots unless you turn them off. Releases are signed by `Ryoku Releases
<releases@ryoku.dev>`, fingerprint `EB6D 3C0F 55A7 B3CA BA6B  2838 847B 274F
025D D6E3`, key in [`keys/`](keys/ryoku-release-key.pub.asc):

```bash
gpg --import keys/ryoku-release-key.pub.asc
gpg --verify ryoku-*.iso.sig ryoku-*.iso
```

**On a machine you already have.** One line converts an existing Arch, CachyOS,
Void glibc or Fedora system in place. It backs up your configs (with a
`restore.sh` to undo), trusts the signed repository, moves you off conflicting
shells and daemons, and wires up the full desktop. It never partitions a disk.

```bash
curl -fsSL https://raw.githubusercontent.com/ryoku-dev/ryoku/main/ryoku-shell-installer/install.sh | bash
```

Append `-s -- --dry-run` after `bash` to see everything it would do first.

> [!WARNING]
> The shell installer rewrites your desktop configuration in place and is still
> being tested across hardware and setups. Back up first, run the dry run, and
> prefer a machine you can afford to reinstall. What it does on each
> distribution, and the limits there, are in
> [`ryoku-shell-installer/`](ryoku-shell-installer/README.md).

The [step-by-step no-ISO guide](docs/install-without-iso.md) starts with a stock
Void installation and also covers Arch, CachyOS, Fedora and NixOS.

## Day to day

```bash
ryoku update            # Ryoku packages, configs and the doctor
ryoku update --system   # the same, plus the Void or Fedora base system
sudo pacman -Syu        # Arch or CachyOS base system and kernel
ryoku rollback          # the ten frozen builds you can return to
ryoku recovery          # last resort: rebuild the whole desktop from main
```

`ryoku update` snapshots first where the host can, moves the packages the
signed repository serves, re-lays the desktop configs, runs the doctor and
reloads the shell. Your own edits live in override files it never touches, so
they survive every update. When even the `ryoku` command is gone, the recovery
runs straight from the repo at a TTY:

```bash
curl -fsSL https://raw.githubusercontent.com/ryoku-dev/ryoku/main/bin/ryoku-recovery | bash
```

On Arch, **Gaming with CachyOS** is offered during installation and the same
bundle is available later in RyoStore ([`docs/kernels.md`](docs/kernels.md)).
[`docs/updates.md`](docs/updates.md) is how a change reaches you;
[`docs/cli.md`](docs/cli.md) is every `ryoku` command.

## Hardware

|  | Minimum | Recommended |
|---|---|---|
| CPU | 64-bit x86_64, dual-core | quad-core or better |
| RAM | 4 GB | 8 GB, 16 GB with the dev toolchains |
| GPU | any card with working KMS and OpenGL/Vulkan | recent integrated or discrete |
| Storage | 32 GB (installer floor) | 64 GB+ SSD |
| Firmware | Arch/CachyOS ISO: UEFI; Void ISO: BIOS or UEFI; Secure Boot off | UEFI, Secure Boot off |

The shell is an accelerated Qt surface, so it wants a real GPU: AMD on Mesa,
Intel from Broadwell on, NVIDIA on the open modules from Turing on and the
proprietary ones before that, all picked at install time. A resting session
uses under 1 GB of RAM. Driver details, hybrid laptops, and the playbook for
awkward hardware (Intel VMD, NVIDIA modeset, Windows dual-boot, Broadcom Wi-Fi)
are in [`docs/installation-hardware.md`](docs/installation-hardware.md).

## Where things are

| Guide | What it covers |
|---|---|
| [`docs/install-without-iso.md`](docs/install-without-iso.md) | Install on stock Void or an existing Arch, CachyOS or Fedora system. |
| [`docs/ryoku.md`](docs/ryoku.md) | What Ryoku is and how the parts fit. |
| [`docs/structure.md`](docs/structure.md) | The repo map: `ryoku/` is the desktop, `system/` the machine, `installation/`, `void/` and `fedora/` the installers and packaging, `release/` the signed repo. |
| [`docs/compositors.md`](docs/compositors.md) | The window-manager seam: what each compositor can do, and how to add one. |
| [`docs/subshells.md`](docs/subshells.md) | Subshells and bar styles: what ships, what the store adds, and how they load. |
| [`docs/updates.md`](docs/updates.md) | Channels: `main` is stable, `unstable-dev` is the gated preview, and how a change reaches an installed machine. |
| [`CONTRIBUTING.md`](CONTRIBUTING.md) | The commit gates and how to send a change. |

## Credits and license

Ryoku's alpha series began as a fork of Omarchy by David Heinemeier Hansson and
contributors; from the beta series on it was rebuilt from an empty root and
shares no code with it. The shell is Ryoku's own, with its frame-blob rendering
and some animation adapted from the
[Caelestia shell](https://github.com/caelestia-dots/shell) and parts of the
display configuration UI from
[DankMaterialShell](https://github.com/AvengeMedia/DankMaterialShell). The bar
styles carry their upstreams with them: QS Bar from HANCORE-linux's Quickshell
Rise, Shima from snowarch's iNiR, Python from ilyamiro's Serpantinum, Nomarchy
from Omarchy quattro. Full attribution is in [`NOTICE`](NOTICE). Ryoku is
released under the [GNU GPL v3](LICENSE).
