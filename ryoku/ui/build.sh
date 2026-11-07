#!/usr/bin/env bash
# build the compiled Ryoku.Ui and Ryoku.Ui.Singletons QML modules
# (CMakeLists.txt) and install them onto a QML import path.
#
#   build.sh <qml-import-dir>
#
# drops <qml-import-dir>/Ryoku/Ui/{libryoku-ui.so, qmldir, ryoku-ui.qmltypes}
# and the same three for Ryoku/Ui/Singletons, and nothing else: the QML sources
# ride inside each .so, so no plain copy of the module can sit on that root. The
# packages run this; a dev box copies the sources instead (install.sh), so a
# checkout edit shows up on the next reload without a compile.
#
# RYOKU_UI_BUILD picks the CMake build dir (default ./build). build deps:
# cmake, ninja, a c++20 compiler, Qt6 Qml/Quick dev files (qmlcachegen).
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
