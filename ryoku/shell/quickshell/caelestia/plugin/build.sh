#!/usr/bin/env bash
# build the Caelestia QML plugin set, install the modules onto a QML import path.
#
#   build.sh <qml-import-dir>
#
# drops <qml-import-dir>/Caelestia/{lib/, Config/, Services/, ...} and M3Shapes/. Quickshell picks
# it up once that dir is on QML2_IMPORT_PATH. target has no build toolchain, so this
# runs on the dev box (deploy.sh) and the ISO build host, never on the installed
# machine. build deps: cmake, ninja, qt6-shadertools, a c++20 compiler, Qt6 dev files,
# libqalculate, pipewire, aubio and lm_sensors headers.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
dest="${1:?usage: build.sh <qml-import-dir>}"
build="${RYOKU_CAELESTIA_BUILD:-$here/build}"

for tool in cmake ninja; do
  command -v "$tool" >/dev/null 2>&1 || {
    advice="$(ryoku-host pkg advice cmake ninja qt6-shadertools 2>/dev/null || printf '%s' 'install cmake, ninja, and Qt shader tools')"
    printf 'build.sh: error: %s is required (%s)\n' "$tool" "$advice" >&2
    exit 1
  }
done

cmake -S "$here" -B "$build" -G Ninja -DCMAKE_BUILD_TYPE=Release -DINSTALL_QMLDIR=qml -DENABLE_MODULES="plugin;m3shapes"
cmake --build "$build"

# install into a scratch prefix, then replace the module dir whole so a renamed or
# removed file never lingers on the import path.
stage="$build/stage"
rm -rf "$stage"
cmake --install "$build" --prefix "$stage" >/dev/null
mkdir -p "$dest"
rm -rf "$dest/Caelestia" "$dest/M3Shapes"
cp -a "$stage/qml/Caelestia" "$dest/Caelestia"
cp -a "$stage/qml/M3Shapes" "$dest/M3Shapes"
