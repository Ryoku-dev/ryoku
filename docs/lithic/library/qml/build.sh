#!/usr/bin/env bash
# packages run this; a dev box copies the sources with install.sh so an edit shows on reload
#
#   build.sh <qml-import-dir>   (LITHIC_QML_BUILD names the build tree)
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
dest="${1:?usage: build.sh <qml-import-dir>}"
build="${LITHIC_QML_BUILD:-$here/build}"

for tool in cmake ninja; do
  command -v "$tool" >/dev/null 2>&1 || {
    printf 'build.sh: error: %s is required to build the Lithic QML module\n' "$tool" >&2
    exit 1
  }
done

cmake -S "$here" -B "$build" -G Ninja -DCMAKE_BUILD_TYPE=Release
cmake --build "$build"

rm -rf "$dest/Lithic"
install -d "$dest/Lithic"
install -m755 "$build/qml/Lithic"/*.so "$dest/Lithic/"
install -m644 "$build/qml/Lithic/qmldir" "$build/qml/Lithic"/*.qmltypes "$dest/Lithic/"
