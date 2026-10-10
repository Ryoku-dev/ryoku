# shellcheck shell=bash
startdir=${startdir:?stage-package.sh must set startdir}
srcdir=${srcdir:?stage-package.sh must set srcdir}
pkgdir=${pkgdir:?stage-package.sh must set pkgdir}
pkgver=${RYOKU_PKGVER:?stage-package.sh must set RYOKU_PKGVER}
_repo="$startdir/../../../.."
_payload="$_repo/release/packages/ryoku-desktop/payload.sh"

build() {
  RYOKU_SRC="$_repo" RYOKU_BUILD_DIR="$srcdir" RYOKU_PKGVER="$pkgver" \
    "$_payload" build
}

package() {
  RYOKU_SRC="$_repo" RYOKU_BUILD_DIR="$srcdir" RYOKU_PKGVER="$pkgver" \
    RYOKU_RELEASE="${RYOKU_RELEASE:-}" RYOKU_CHANNEL="${RYOKU_CHANNEL:-}" \
    RYOKU_NAME="${RYOKU_NAME:-}" RYOKU_COMMIT="${RYOKU_COMMIT:-}" \
    "$_payload" install "$pkgdir" systemd

  rm -rf "$pkgdir/etc/boot" \
    "$pkgdir/usr/lib/initcpio" \
    "$pkgdir/usr/share/libalpm" \
    "$pkgdir/usr/share/ryoku/boot"
  rm -f "$pkgdir/usr/bin/ryoku-boot-apply" \
    "$pkgdir/usr/bin/ryoku-windows-entry"

  install -Dm644 "$_repo/fedora/system/hardware/network/ryoku-wifi-regdom.service" \
    "$pkgdir/usr/lib/systemd/system/ryoku-wifi-regdom.service"
}
