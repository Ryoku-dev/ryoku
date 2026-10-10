#!/usr/bin/env bash
set -euo pipefail
name=${1:?package name}
stage=$(realpath -m "${2:?staging directory}")
libdir=${3:-/usr/lib64}
root=$(cd "$(dirname "$0")/../../.." && pwd)
export GOFLAGS="-mod=vendor -trimpath" CGO_ENABLED=0 GOTOOLCHAIN=local
export RYOKU_PKGVER=${RYOKU_PKGVER:?version required}
startdir="$root/fedora/packages/rpm/payload"
srcdir="$root/.rpm-build/$name"
pkgdir="$stage"
mkdir -p "$srcdir" "$pkgdir"
# The payload recipe is Fedora release input. It stages files only: no host
# installation hooks run while building an RPM.
recipe="$startdir/$name.sh"
[[ -f $recipe ]] || { echo "unknown RPM payload: $name" >&2; exit 2; }
# shellcheck source=/dev/null
source "$recipe"
if declare -F build >/dev/null; then build; fi
package
if [[ -d $stage/usr/lib/qt6 ]]; then
  mkdir -p "$stage$libdir"
  mv "$stage/usr/lib/qt6" "$stage$libdir/qt6"
fi
if [[ $name == ryoku-desktop ]]; then
  cp "$root/.rpm-release" "$stage/etc/ryoku-release"
  rm -f "$stage/usr/share/applications/mimeapps.list"
fi
if [[ $name == ryoku-desktop-* ]]; then
  provider=${name#ryoku-desktop-}
  install -Dm644 "$root/ryoku/apps/mimeapps.list" "$stage/usr/share/applications/$provider-mimeapps.list"
fi

while IFS= read -r -d '' link; do
  target=$(readlink "$link")
  if [[ $target == /usr/lib/qt6/* ]]; then
    ln -sfn "$libdir/qt6/${target#/usr/lib/qt6/}" "$link"
  fi
done < <(find "$stage" -type l -print0)
