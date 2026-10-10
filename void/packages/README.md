# Void packages (future)

This directory is the home of the xbps-src templates for a signed Ryoku XBPS
repository, the counterpart of `release/packages/` (PKGBUILDs) and
`release/repo/` (build + sign + index). It is intentionally empty until that
milestone starts; nothing here is wired to any build today.

Shape it will take, following the working precedents (DankMaterialShell's
`distro/void/srcpkgs/`, iNiR's planned template):

- one `srcpkgs/<pkg>/template` per component, buildable in a void-packages
  checkout (`./xbps-src pkg <pkg>`) and submittable upstream unchanged;
- templates install runit service directories from `void/init/system/` and
  the user-service tree from `void/init/user/`, never systemd units;
- repo publishing: `xbps-rindex --sign` with a Ryoku PEM key, served from the
  same bucket as the pacman repo under a `void/` mount, configured on the box
  by a `repository=` file in `/etc/xbps.d/`;
- Quickshell stays the distro package (Void rebuilds it in lockstep with Qt);
  Ryoku never ships its own Quickshell build, the ABI receipt model
  (`docs/hyprland-plugins.md`) covers only what Ryoku compiles itself.

Until then, Void installs are from-source: the payload builds the Go programs,
the QML plugin and the config exactly like the Debian lane
(`ryoku-shell-installer/distro.go`, `fromSource: true`).
