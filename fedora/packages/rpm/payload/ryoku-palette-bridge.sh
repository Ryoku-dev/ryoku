#!/usr/bin/env bash
set -euo pipefail

startdir=${startdir:?stage-package.sh must set startdir}
srcdir=${srcdir:?stage-package.sh must set srcdir}
pkgdir=${pkgdir:?stage-package.sh must set pkgdir}
_repo="$startdir/../../../.."

build() {
  ( cd "$_repo/ryoku/palette-bridge" && go build -o "$srcdir/ryoku-palette-bridge" . )
}

package() {
  install -Dm755 "$srcdir/ryoku-palette-bridge" "$pkgdir/usr/bin/ryoku-palette-bridge"
  install -Dm755 "$_repo/ryoku/palette-bridge/doctor.sh" \
    "$pkgdir/usr/bin/ryoku-palette-bridge-doctor"
  install -Dm755 "$_repo/ryoku/palette-bridge/remove-integrations.sh" \
    "$pkgdir/usr/bin/ryoku-palette-bridge-remove-integrations"
  install -Dm644 "$_repo/ryoku/palette-bridge/packaging/systemd/ryoku-palette-bridge.service" \
    "$pkgdir/usr/lib/systemd/user/ryoku-palette-bridge.service"
  install -d "$pkgdir/usr/share/ryoku/palette-bridge"
  cp -a "$_repo/ryoku/palette-bridge/." "$pkgdir/usr/share/ryoku/palette-bridge/"
}
