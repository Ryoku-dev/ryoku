# Fedora edition

The Fedora edition brings the Ryoku desktop to mutable Fedora x86_64 systems. It uses DNF for installation, updates, release channels and package rollback, while keeping the Hyprland and niri desktops in the same signed repository.

Release jobs prepare source RPMs, rebuild them on Fedora, sign every binary RPM and the repository metadata, then publish the result below `https://repo.ryoku.dev/stable/fedora/<release>/`. Install it with the same public installer used by the other editions:

```sh
curl -fsSL https://raw.githubusercontent.com/ryoku-dev/ryoku/main/ryoku-shell-installer/install.sh | bash
```

Fedora does not ship a Ryoku ISO in this release. It also leaves Fedora's GRUB configuration alone and does not provide snapshot boot or snapshot rollback; package rollback and the boot guard remain available.
