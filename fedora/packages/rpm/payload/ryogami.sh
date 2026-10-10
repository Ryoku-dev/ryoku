# Maintainer: Ryoku <releases@ryoku.dev>
#
# Ryogami: the Ryoku wallpaper daemon (Go). One process owns the wallpaper
# catalog (thumbnails, colours, favourites), applies through the in-shell
# surface topic, the matugen palette, and the depth surface, behind ryoku's
# line pub/sub on $XDG_RUNTIME_DIR/ryogami.sock. The vendored skwd-wall picker
# (wall-ui) is its front end.
#
# built from the in-repo Go module at ryoku/shell/ryogami/daemon, no tarball
# fetched. publish CI runs makepkg inside this dir against a full checkout, so
# the repo root is three levels up; go builds into $srcdir so the source tree
# stays untouched.
pkgname=ryogami
pkgver=${RYOKU_PKGVER:-0.2.0}
pkgrel=1
pkgdesc="Ryogami: the Ryoku wallpaper daemon (catalog, thumbnails, applies, depth)"
arch=('x86_64')
url="https://ryoku.dev"
license=('MIT')
# Runtime tools for the daemon and picker: ImageMagick and ffmpeg for image/video
# processing, quickshell for the Qt Quick picker, curl for remote wallpaper
# sources, and inotify-tools for live wallpaper directory updates.
depends=('imagemagick' 'ffmpeg' 'quickshell' 'curl' 'inotify-tools')
makedepends=('go' 'cmake' 'ninja' 'qt6-base' 'qt6-declarative' 'qt6-shadertools' 'qt6-multimedia' 'wayland' 'wayland-protocols')
source=()

_repo="$startdir/../../../.."

build() {
  cd "$_repo/ryoku/shell/ryogami/daemon"
  # -mod=vendor keeps the build off proxy.golang.org so it is reproducible and
  # works in the offline publish container (the module is stdlib-only, so there
  # is no vendor/ tree to carry). Matches every other [ryoku] Go package.
  CGO_ENABLED=0 go build -trimpath -mod=vendor -o "$srcdir/ryogami" .
  "$_repo/ryoku/shell/livewall/build.sh" "$srcdir/ryogami-live"
  RYOGAMI_PICKER_BUILD="$srcdir/picker-build" \
    "$_repo/ryoku/shell/ryogami/picker/build.sh" "$srcdir/qml"
}

package() {
  install -Dm755 "$srcdir/ryogami" "$pkgdir/usr/bin/ryogami"
  install -Dm755 "$srcdir/ryogami-live" "$pkgdir/usr/bin/ryogami-live"
  # user unit: systemd finds it under /usr/lib/systemd/user without materialize,
  # exactly where ExecStart already points (/usr/bin/ryogami).
  install -Dm644 "$_repo/ryoku/shell/systemd/user/ryogami.service" \
    "$pkgdir/usr/lib/systemd/user/ryogami.service"
  # Install the compiled Qt Quick picker module and its shell entry point.
  install -d "$pkgdir/usr/lib/qt6/qml/Ryoku/Ryogami"
  cp -a "$srcdir/qml/Ryoku/Ryogami/." "$pkgdir/usr/lib/qt6/qml/Ryoku/Ryogami/"
  chmod -R u=rwX,go=rX "$pkgdir/usr/lib/qt6"
  install -Dm644 "$_repo/ryoku/shell/ryogami/picker/shell.qml" \
    "$pkgdir/usr/share/ryogami/shell.qml"
  install -Dm644 "$_repo/ryoku/shell/ryogami/picker/data/ryogami.desktop" \
    "$pkgdir/usr/share/applications/ryogami.desktop"
  install -Dm644 "$_repo/ryoku/shell/ryogami/picker/LICENSE" \
    "$pkgdir/usr/share/licenses/ryogami/LICENSE"
  install -Dm644 "$_repo/ryoku/shell/ryogami/picker/NOTICE" \
    "$pkgdir/usr/share/licenses/ryogami/NOTICE"
  install -Dm644 -t "$pkgdir/usr/share/licenses/ryogami/" \
    "$_repo"/ryoku/shell/ryogami/picker/qml/theme/fonts/*.txt
}
