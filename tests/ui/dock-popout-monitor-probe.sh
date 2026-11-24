#!/usr/bin/env bash
# dock-popout-monitor-probe: the DockPreviewPopout on each monitor must only
# show the strip when that monitor's dock owns the hover. The DockPreview
# singleton now carries `monitorName` so only the popout on that output opens;
# the other stays closed. Loads the real shell components against a faithful
# mirror of shell/ used as the config folder, the way nacre-popup-probe.sh does,
# so their deep relative imports resolve.
# Commit nothing.
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

cp "$here/dock-popout-monitor-probe.qml" "$work/shell/probe.qml"

QML2_IMPORT_PATH="$work:${QML2_IMPORT_PATH:-$HOME/.local/lib/qt6/qml}" \
    timeout 30 qs -p "$work/shell/probe.qml" >"$work/probe.log" 2>&1 || true

if ! grep -q DOCK-POPOUT-MONITOR-PROBE-PASS "$work/probe.log"; then
    echo "dock-popout-monitor-probe: FAILED" >&2
    sed -n '1,120p' "$work/probe.log" >&2
    exit 1
fi

echo "dock-popout-monitor-probe: both strips scope to their own monitor, hover hand-over and guarded clear work"