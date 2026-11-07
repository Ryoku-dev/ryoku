#!/usr/bin/env bash
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
repo="$here/../.."
src="$repo/ryoku/shell/quickshell/shell"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

mkdir -p "$work/Ryoku" "$work/shell/modules" "$work/cfg/ryoku" \
    "$work/home" "$work/xdg"
ln -s "$repo/ryoku/ui" "$work/Ryoku/Ui"
ln -s "$repo/ryoku/shell/quickshell/stage" "$work/stage"
for child in "$src"/*; do
    name="$(basename "$child")"
    [[ "$name" == modules ]] && continue
    ln -s "$child" "$work/shell/$name"
done
for child in "$src/modules"/*; do
    ln -s "$child" "$work/shell/modules/$(basename "$child")"
done

cat >"$work/cfg/ryoku/widgets.json" <<'JSON'
{
  "widgetDepths": {
    "clock": 0.37
  }
}
JSON
cp "$here/widget-depth-per-output-probe.qml" "$work/shell/probe.qml"

import_path="$work:${QML2_IMPORT_PATH:-$HOME/.local/lib/qt6/qml}"
XDG_RUNTIME_DIR="$work/xdg" XDG_CONFIG_HOME="$work/cfg" HOME="$work/home" \
    QT_QPA_PLATFORM=offscreen QML2_IMPORT_PATH="$import_path" \
    timeout 30 qs -p "$work/shell/probe.qml" >"$work/probe.log" 2>&1 || true

if ! grep -q WIDGET-DEPTH-PER-OUTPUT-PROBE-PASS "$work/probe.log"; then
    echo "widget-depth-per-output-probe: FAILED" >&2
    cat "$work/probe.log" >&2
    exit 1
fi

widgets="$work/cfg/ryoku/widgets.json"
jq -e '
    .widgetDepths.clock == 0.37 and
    .monitors["DP-1"].clockDepth == 0.2 and
    .monitors["HDMI-A-1"].clockDepth == 0.8
' "$widgets" >/dev/null

echo "widget-depth-per-output-probe: outputs keep independent depths and legacy values remain the fallback"
