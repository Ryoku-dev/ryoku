#!/usr/bin/env bash
# center-popout-probe: a frame popout placed at "center" floats in the middle of
# the screen (body + input mask centred, no hover band), an edge popout beside it
# still docks, and the plugin host wires a centred placement end to end: the
# manager carries the modal's rect in `pluginMask`, and a built-in centre surface
# takes the screen from it instead of overlapping it. Loads the real shell
# components against a faithful mirror of shell/ used as the config folder, so
# their deep relative imports resolve. Commit nothing.
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

cp "$here/center-popout-probe.qml" "$work/shell/probe.qml"

# fixture plugin + placement, so the host half discovers a real "center" entry.
mkdir -p "$work/plugins/fixture/content" "$work/plugins/fixture/service" "$work/cfg/ryoku"
cat >"$work/plugins/fixture/manifest.json" <<'JSON'
{
  "id": "fixture",
  "name": "Fixture",
  "version": "1.0.0",
  "entryPoints": { "main": "service/Main.qml", "content": "content/Widget.qml" },
  "hosts": ["framePopout"],
  "defaults": { "host": "framePopout", "framePopout": { "edge": "center", "align": "center" } }
}
JSON
printf 'import QtQuick\nQtObject { property var pluginApi }\n' \
    >"$work/plugins/fixture/service/Main.qml"
printf 'import QtQuick\nItem { property var pluginApi; property string density; property real s: 1; property real widthBudget; property bool active; implicitHeight: 160 }\n' \
    >"$work/plugins/fixture/content/Widget.qml"
cat >"$work/cfg/ryoku/plugins.json" <<'JSON'
{ "fixture": { "enabled": true, "host": "framePopout",
  "framePopout": { "edge": "center", "align": "center", "hoverW": 320, "hoverH": 16 },
  "settings": {} } }
JSON
cp "$here/center-popout-probe.host.qml" "$work/shell/probe-host.qml"

QML2_IMPORT_PATH="$work:${QML2_IMPORT_PATH:-$HOME/.local/lib/qt6/qml}" \
    timeout 30 qs -p "$work/shell/probe.qml" >"$work/probe.log" 2>&1 || true

if ! grep -q CENTER-POPOUT-PROBE-PASS "$work/probe.log"; then
    echo "center-popout-probe: FAILED" >&2
    sed -n '1,120p' "$work/probe.log" >&2
    exit 1
fi

RYOKU_SHELL_DIR="$repo/ryoku/shell" RYOSTORE_PLUGINS_DIR="$work/plugins" \
    XDG_CONFIG_HOME="$work/cfg" \
    QML2_IMPORT_PATH="$work:${QML2_IMPORT_PATH:-$HOME/.local/lib/qt6/qml}" \
    timeout 40 qs -p "$work/shell/probe-host.qml" >"$work/host.log" 2>&1 || true

if ! grep -q CENTER-POPOUT-HOST-PASS "$work/host.log"; then
    echo "center-popout-probe: HOST half FAILED" >&2
    sed -n '1,120p' "$work/host.log" >&2
    exit 1
fi
echo "center-popout-probe: centred popout centres, edge popout still docks, host masks the modal and hands the screen over"
