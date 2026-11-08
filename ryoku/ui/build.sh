#!/usr/bin/env bash
# packages run this, a dev box copies the sources with install.sh so edits show on reload
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
dest="${1:?usage: build.sh <qml-import-dir>}"
build="${RYOKU_UI_BUILD:-$here/build}"

for tool in cmake ninja; do
  command -v "$tool" >/dev/null 2>&1 || {
    advice="$(ryoku-host pkg advice cmake ninja 2>/dev/null || printf '%s' 'install cmake and ninja')"
    printf 'build.sh: error: %s is required (%s)\n' "$tool" "$advice" >&2
    exit 1
  }
done

cmake -S "$here" -B "$build" -G Ninja -DCMAKE_BUILD_TYPE=Release
cmake --build "$build"

rm -rf "$dest/Ryoku/Ui"
for module in Ui Ui/Singletons; do
  src="$build/qml/Ryoku/$module"
  install -d "$dest/Ryoku/$module"
  install -m755 "$src"/*.so "$dest/Ryoku/$module/"
  install -m644 "$src/qmldir" "$src"/*.qmltypes "$dest/Ryoku/$module/"
done
