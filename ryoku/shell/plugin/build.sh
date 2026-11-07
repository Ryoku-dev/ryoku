#!/usr/bin/env bash
# build the Ryoku.Blobs, Ryoku.Spatial3D and Ryoku.Bus QML plugins, install
# the modules onto a QML import path.
#
#   build.sh <qml-import-dir>
#
# drops <qml-import-dir>/Ryoku/Blobs/{libryoku-blobs.so, qmldir, *.qsb, ...},
# <qml-import-dir>/Ryoku/Spatial3D/{libryoku-spatial3d.so, qmldir, ...} and
# <qml-import-dir>/Ryoku/Bus/{libryoku-bus.so, qmldir, ...}.
# Quickshell picks them up once that dir is on QML2_IMPORT_PATH (ryoku-shell does
# that for the surfaces it supervises). target has no build toolchain, so this
# runs on the dev box (deploy.sh) and the ISO build host (iso/build.sh), never
# on the installed machine. build deps: cmake, ninja, qt6-shadertools,
# qt6-quick3d, a c++20 compiler, Qt6 dev files.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
dest="${1:?usage: build.sh <qml-import-dir>}"
build="${RYOKU_BLOBS_BUILD:-$here/build}"

for tool in cmake ninja; do
  command -v "$tool" >/dev/null 2>&1 || {
    advice="$(ryoku-host pkg advice cmake ninja qt6-shadertools qt6-quick3d 2>/dev/null || printf '%s' 'install cmake, ninja, Qt shader tools, and Qt Quick 3D')"
    printf 'build.sh: error: %s is required (%s)\n' "$tool" "$advice" >&2
    exit 1
  }
done

cmake -S "$here" -B "$build" -G Ninja -DCMAKE_BUILD_TYPE=Release
cmake --build "$build"

# self-contained modules under <build>/qml/Ryoku. nuke any prior copy first so a
# renamed / removed file doesn't linger on the import path.
mkdir -p "$dest/Ryoku"
for module in Blobs Spatial3D Bus; do
  rm -rf "$dest/Ryoku/$module"
  cp -a "$build/qml/Ryoku/$module" "$dest/Ryoku/$module"
  rm -f "$dest/Ryoku/$module"/*.qrc
done
