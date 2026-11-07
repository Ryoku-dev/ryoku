#!/usr/bin/env bash
# Loads the shell popout and media service against a faithful shell tree mirror.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
repo="$here/../.."
src="$repo/ryoku/shell/quickshell/shell"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

mkdir -p "$work/Ryoku" "$work/shell/modules"
ln -s "$repo/ryoku/ui" "$work/Ryoku/Ui"
ln -s "$repo/ryoku/shell/framebars" "$work/Ryoku/FrameBars"
for child in "$src"/*; do
    name="$(basename "$child")"
    [[ "$name" == modules ]] && continue
    ln -s "$child" "$work/shell/$name"
done
for child in "$src/modules"/*; do
    ln -s "$child" "$work/shell/modules/$(basename "$child")"
done

cp "$here/shell-popout-probe.qml" "$work/shell/probe.qml"
impath="$work:${QML2_IMPORT_PATH:-$HOME/.local/lib/qt6/qml}"
QML2_IMPORT_PATH="$impath" timeout 20 qs -p "$work/shell/probe.qml" >"$work/probe.log" 2>&1 || true

if ! grep -q SHELL-POPOUT-PROBE-PASS "$work/probe.log"; then
    echo "shell-popout-probe: Popout or Media did not load" >&2
    sed -n '1,100p' "$work/probe.log" >&2
    exit 1
fi
if grep -Eq ' ERROR|TypeError|ReferenceError|is not a type|Type .* unavailable' "$work/probe.log"; then
    echo "shell-popout-probe: loaded with errors" >&2
    sed -n '1,100p' "$work/probe.log" >&2
    exit 1
fi

echo "shell-popout-probe: Popout and Media load"
