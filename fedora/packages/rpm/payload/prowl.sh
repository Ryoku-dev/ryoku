# shellcheck shell=bash
startdir=${startdir:?stage-package.sh must set startdir}
srcdir=${srcdir:?stage-package.sh must set srcdir}
pkgdir=${pkgdir:?stage-package.sh must set pkgdir}
pkgver=${RYOKU_PKGVER:?stage-package.sh must set RYOKU_PKGVER}
commit=${PROWL_COMMIT:?prowl.spec must set PROWL_COMMIT}
_repo="$startdir/../../../.."
_source="$_repo/.rpm-prowl/prowl-$commit"

build() {
  [[ -f $_source/go.mod && -d $_source/vendor ]] || {
    printf 'prowl: prepared vendored source is missing\n' >&2
    return 1
  }
  mkdir -p "$srcdir/out" "$srcdir/gopath" "$srcdir/gocache"
  (
    cd "$_source" || exit
    CGO_ENABLED=1 \
      GOPATH="$srcdir/gopath" \
      GOCACHE="$srcdir/gocache" \
      GOFLAGS='-mod=vendor -modcacherw -buildmode=pie -trimpath' \
      go build -tags sqlite_fts5 \
        -ldflags "-s -w -X main.version=v$pkgver -X main.commit=$commit -X main.managedBy=dnf" \
        -o "$srcdir/out/prowl" ./cmd/prowl
  )
}

package() {
  install -Dm755 "$srcdir/out/prowl" "$pkgdir/usr/bin/prowl"
  install -Dm644 "$_source/LICENSE" "$pkgdir/usr/share/licenses/prowl/LICENSE"
  install -Dm644 "$_source/NOTICE.md" "$pkgdir/usr/share/licenses/prowl/NOTICE.md"
}
