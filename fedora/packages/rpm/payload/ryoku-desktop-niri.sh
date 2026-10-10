# Maintainer: Ryoku <releases@ryoku.dev>
#
# ryoku-desktop-niri: the niri compositor variant of the Ryoku desktop.
# Carries niri, the xwayland-satellite X11 bridge, the GNOME portal backend niri's
# caps report, the niri config tree, and the ryoku-wm-niri provider binary
# (ryoku/wm/niri). It provides the ryoku-desktop-compositor virtual, so an
# installed ryoku-desktop is satisfied by it, and may be installed beside
# ryoku-desktop-hyprland: a switch is then a config change, not a package swap.
#
# There is no plugin subsystem here: niri has no plugins, and its caps say so.
#
# RPM metadata and dependencies live in ryoku-desktop-niri.spec. This sourced
# recipe only stages the payload assembled by stage-package.sh.
#
# niri has no built-in Xwayland; Fedora 44's xwayland-satellite 0.8.2 regresses
# override-redirect popups (upstream #468, fixed in add2795, no release yet).
# The distro package is the display bridge; the popup fix is not delivered and
# must not be claimed as ported. Screen sharing rides xdg-desktop-portal-gnome.
# Night light rides gammastep. Both variants may be installed, so a switch is a
# config change, not a package swap.
startdir=${startdir:?stage-package.sh must set startdir}
pkgdir=${pkgdir:?stage-package.sh must set pkgdir}
_repo="$startdir/../../../.."

package() {
  local cfg="$pkgdir/usr/share/ryoku/config"

  # niri config tree: config.kdl plus every static file it includes (session
  # startup, lid events, machine seeds, and the user override). A missing include
  # is a hard niri config error, so the whole tree ships as one unit. The
  # generated settings.kdl/rebinds.kdl are written by `ryoku-wm-niri apply`, not
  # packaged (see caps.GeneratedFiles). No scripts/ or share picker: niri's leaf
  # keybinds are compositor actions or `spawn ryoku-shell`, and the share picker
  # is a hyprland-only portal helper.
  install -d "$cfg/niri"
  cp -a "$_repo/ryoku/niri/." "$cfg/niri/"

  # xdg-desktop-portal: niri has no backend of its own, so screen sharing rides
  # the GNOME backend (caps.PortalBackend) but the GNOME FileChooser hangs off a
  # GNOME session. This routing pins FileChooser to gtk so file pickers open;
  # doctor's portal reconciler reads it by the running desktop's name.
  install -Dm644 "$_repo/ryoku/niri/niri-portals.conf" \
    "$cfg/xdg-desktop-portal/niri-portals.conf"

  # cp -a kept source modes; the tree is all data, so world-readable is right.
  chmod -R u=rwX,go=rX "$pkgdir/usr/share/ryoku"

  # ryoku-wm-niri: the niri half of the window-manager seam. Built from the
  # ryoku/wm module (its root is one dir up from the niri package).
  ( cd "$_repo/ryoku/wm/niri" && go build -o ryoku-wm-niri . )
  install -Dm755 "$_repo/ryoku/wm/niri/ryoku-wm-niri" \
    "$pkgdir/usr/bin/ryoku-wm-niri"
  # order the config bootstrap ahead of niri.service, so the session's first login
  # reads a laid-down ~/.config/niri instead of the compositor's own defaults.
  install -Dm644 "$_repo/ryoku/shell/systemd/user/niri.service.d/ryoku-bootstrap.conf" \
    "$pkgdir/usr/lib/systemd/user/niri.service.d/ryoku-bootstrap.conf"
}
