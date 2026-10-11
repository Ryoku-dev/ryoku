# Fedora edition

The Fedora edition installs Ryoku on mutable Fedora 44 Workstation or Server,
`x86_64`, with systemd and either Hyprland or niri. rpm-ostree systems are
refused. Use the same public installer as the other existing-system editions:

```sh
curl -fsSL https://raw.githubusercontent.com/ryoku-dev/ryoku/main/ryoku-shell-installer/install.sh | bash
```

The installer uses dnf5 or dnf, verifies and imports Ryoku's release key, adds
the signed RPM repository for the running Fedora release, and replaces GDM with
SDDM. Package-restricted COPR repositories provide only the Hyprland stack,
starship, lazygit, and yazi packages Fedora lacks. Ryoku's own packages come
from its signed repository.

Release jobs prepare SRPMs from the shared tree, rebuild and sign the binary
RPMs and `repomd.xml`, install-test both desktops in a clean Fedora container,
then publish the tested repository. For Fedora release `<N>`, the public layout
below `https://repo.ryoku.dev/` is:

```text
stable/fedora/<N>/x86_64
stable/fedora/<N>/releases/<tag>/x86_64
stable/fedora/<N>/releases/index.json
stable/fedora/<N>/channels/testing/x86_64
stable/fedora/<N>/channels/testing/builds/<build>/x86_64
stable/fedora/<N>/channels/testing/index.json
```

The build name replaces `+` with `_` in the frozen directory. Public repository
directories contain signed binary RPMs, `repodata/`, the detached armored
`repomd.xml` signature, and `release.json`; SRPMs are not published.

`ryoku update`, `ryoku track`, and `ryoku rollback` use DNF. Frozen release and
testing targets are retained for package rollback, and the boot guard can
downgrade the Ryoku set after a failed update. Fedora has no Ryoku ISO,
snapshots, or snapshot boot menu because it keeps GRUB. NVIDIA stays on nouveau
in this release.
