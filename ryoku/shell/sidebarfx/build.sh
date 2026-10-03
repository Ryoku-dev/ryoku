#!/usr/bin/env bash
# build the Ryoku.SidebarFx QML plugin, install the module onto a QML import path.
#
#   build.sh <qml-import-dir>
#
# drops <qml-import-dir>/Ryoku/SidebarFx/{libryoku-sidebarfx.so, qmldir, *.qsb, ...}.
# Quickshell picks it up once that dir is on QML2_IMPORT_PATH. Installed systems
# receive the prebuilt module, so this only runs on build and development hosts.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
dest="${1:?usage: build.sh <qml-import-dir>}"
build="${RYOKU_SIDEBARFX_BUILD:-$here/build}"

for tool in cmake ninja; do
  command -v "$tool" >/dev/null 2>&1 || {
    printf 'build.sh: error: %s is required (pacman -S cmake ninja qt6-shadertools)\n' "$tool" >&2
    exit 1
  }
done

cmake -S "$here" -B "$build" -G Ninja -DCMAKE_BUILD_TYPE=Release
cmake --build "$build"

mkdir -p "$dest/Ryoku"
rm -rf "$dest/Ryoku/SidebarFx"
cp -a "$build/qml/Ryoku/SidebarFx" "$dest/Ryoku/SidebarFx"
rm -f "$dest/Ryoku/SidebarFx"/*.qrc
