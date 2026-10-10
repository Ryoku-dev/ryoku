# Maintainer: Ryoku <releases@ryoku.dev>
#
# Ryoku Hub backend (Go). A standalone helper the Hub's Quickshell config shells
# out to for the keybind legend and the Hub's TOML config. It parses config
# files and prints results, so it has no runtime dependencies of its own.
#
# Built from the in-repo source at ryoku/hub/backend; no tarball is fetched.
# go build reads the committed vendor/ tree (-mod=vendor), so the signed-repo CI
# builds with no network regardless of whatever GOFLAGS makepkg picked up.
pkgname=ryoku-hub
pkgver=${RYOKU_PKGVER:-0.1.0}
pkgrel=1
pkgdesc="Ryoku Hub backend: keybind legend and config service"
arch=('x86_64')
url="https://ryoku.dev"
license=('GPL-3.0-or-later')
depends=()
makedepends=('go')
source=()

_repo="$startdir/../../../.."

build() {
  cd "$_repo/ryoku/hub/backend"
  CGO_ENABLED=0 go build -trimpath -mod=vendor -o "$srcdir/ryoku-hub" .
}

package() {
  install -Dm755 "$srcdir/ryoku-hub" "$pkgdir/usr/bin/ryoku-hub"
}
