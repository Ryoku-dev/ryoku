#!/usr/bin/env bash
# a dev box imports Lithic from its sources so an edit shows on the next reload;
# packages ship the compiled module from build.sh
#
#   install.sh [<qml-import-root>]   (default: ~/.local/lib/qt6/qml)
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
root="${1:-$HOME/.local/lib/qt6/qml}"

rm -rf "$root/Lithic"
install -d "$root/Lithic"
cp -a "$here/Lithic/." "$root/Lithic/"

# Lithic.qml loads its faces from ../../go/fonts beside the module
fonts="$root/../go/fonts"
install -d "$fonts"
install -m644 "$here/../go/fonts"/*.ttf "$here/../go/fonts"/*-OFL.txt "$fonts/"
echo "installed Lithic -> $root/Lithic"
