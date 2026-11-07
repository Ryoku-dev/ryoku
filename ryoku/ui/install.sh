#!/usr/bin/env bash
# Put Ryoku.Ui on the QML import path so every surface -- the shell's configs,
# the Hub, and the first-party apps -- imports one design system instead of
# carrying its own copy. Mirrors plugins/kit/install.sh. This is the dev box
# path: a plain copy of the sources, which Qt's disk cache compiles on first
# load, so a checkout edit needs no rebuild. Packages ship the compiled module
# from build.sh instead; each script empties Ryoku/Ui first, so one import root
# never holds both.
#
#   install.sh [<qml-import-root>]   (default: ~/.local/lib/qt6/qml)
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
root="${1:-$HOME/.local/lib/qt6/qml}"
dest="$root/Ryoku/Ui"

rm -rf "$dest"
mkdir -p "$dest"
cp -r "$here/." "$dest/"
rm -rf "$dest/install.sh" "$dest/build.sh" "$dest/CMakeLists.txt" "$dest/build"
echo "installed Ryoku.Ui -> $dest"
