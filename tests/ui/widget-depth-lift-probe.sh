#!/usr/bin/env bash
# Loads the real components against a mirrored shell/ tree, the way
# center-popout-probe.sh does. Commit nothing.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
repo="$here/../.."
src="$repo/ryoku/shell/quickshell/shell"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

mkdir -p "$work/Ryoku" "$work/shell/modules" "$work/cfg/ryoku" "$work/home"
install -d -m 0700 "$work/xdg"
ln -s "$repo/ryoku/ui" "$work/Ryoku/Ui"
ln -s "$repo/ryoku/shell/framebars" "$work/Ryoku/FrameBars" 2>/dev/null || true
# The widget menu's size stepper reads the Stage Editor's island module.
ln -s "$repo/ryoku/shell/quickshell/stage" "$work/stage"
# The universal dock service imports the iRiS port's services module.
ln -s "$repo/ryoku/shell/quickshell/inir" "$work/inir"
for child in "$src"/*; do
    name="$(basename "$child")"
    [[ "$name" == modules ]] && continue
    ln -s "$child" "$work/shell/$name"
done
for child in "$src/modules"/*; do
    ln -s "$child" "$work/shell/modules/$(basename "$child")"
done

cp "$here/widget-depth-lift-probe.qml" "$work/shell/probe.qml"

import_path="$work:${QML2_IMPORT_PATH:-$HOME/.local/lib/qt6/qml}"

run_probe() {
    RYOKU_PROBE_RESTART="$1" XDG_RUNTIME_DIR="$work/xdg" XDG_CONFIG_HOME="$work/cfg" \
        HOME="$work/home" QT_QPA_PLATFORM=offscreen QML2_IMPORT_PATH="$import_path" \
        timeout 30 qs -p "$work/shell/probe.qml" >"$work/probe.log" 2>&1 || true

    if ! grep -q WIDGET-DEPTH-LIFT-PROBE-PASS "$work/probe.log"; then
        echo "widget-depth-lift-probe: FAILED" >&2
        sed -n '1,160p' "$work/probe.log" >&2
        exit 1
    fi
}

run_probe 0

# The lift the probe ended with must be the file the next shell start reads:
stage="$work/cfg/ryoku/stage.json"
if [[ ! -f "$stage" ]]; then
    echo "widget-depth-lift-probe: stage.json was never written" >&2
    exit 1
fi
grep -q '"clock"' "$stage" || { echo "widget-depth-lift-probe: clock missing from stage.json" >&2; exit 1; }
grep -q '"probe.tile"' "$stage" || { echo "widget-depth-lift-probe: plugin tile missing from stage.json" >&2; exit 1; }
if grep -q '"visualizer"' "$stage"; then
    echo "widget-depth-lift-probe: the visualiser stayed lifted in the file" >&2
    exit 1
fi
grep -q '"version": 3' "$stage" || { echo "widget-depth-lift-probe: stage.json is not v3" >&2; exit 1; }
grep -q '"parallax"' "$stage" || { echo "widget-depth-lift-probe: stage.json lost the parallax section" >&2; exit 1; }

run_probe 1

echo "widget-depth-lift-probe: Depth rows gate on the cut and widget lifts survive a restart"
