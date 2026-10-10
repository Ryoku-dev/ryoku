# shellcheck shell=bash
startdir=${startdir:?stage-package.sh must set startdir}
srcdir=${srcdir:?stage-package.sh must set srcdir}
pkgdir=${pkgdir:?stage-package.sh must set pkgdir}
_repo="$startdir/../../../.."

build() {
  cd "$_repo/ryoku/wm/hyprland" || exit
  go build -o "$srcdir/ryoku-wm-hyprland" .
}

package() {
  local cfg="$pkgdir/usr/share/ryoku/config"
  local script

  install -d "$cfg/hypr"
  cp -a "$_repo/ryoku/hyprland/." "$cfg/hypr/"

  # Fedora's portal package uses its bundled picker name.
  sed -i \
    's/custom_picker_binary = hyprland-preview-share-picker/custom_picker_binary = hyprland-share-picker/' \
    "$cfg/hypr/xdph.conf"

  install -Dm644 "$_repo/ryoku/hyprland/hyprland-portals.conf" \
    "$cfg/xdg-desktop-portal/hyprland-portals.conf"
  install -Dm644 "$_repo/ryoku/apps/hyprland-preview-share-picker/config.yaml" \
    "$cfg/hyprland-preview-share-picker/config.yaml"
  chmod -R u=rwX,go=rX "$pkgdir/usr/share/ryoku"

  for script in "$_repo"/ryoku/hyprland/scripts/ryoku-*; do
    [[ -f $script ]] || continue
    install -Dm755 "$script" "$pkgdir/usr/bin/${script##*/}"
  done

  install -Dm755 "$srcdir/ryoku-wm-hyprland" \
    "$pkgdir/usr/bin/ryoku-wm-hyprland"
  install -d "$pkgdir/usr/lib/qt6/qml/Ryoku/Wm/Hyprland"
  cp -a "$_repo/ryoku/wm/hyprland/qml/." \
    "$pkgdir/usr/lib/qt6/qml/Ryoku/Wm/Hyprland/"
}
