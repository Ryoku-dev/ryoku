#!/usr/bin/env bash
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
repo="$here/../.."
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

mkdir -p "$work/Ryoku" "$work/bin" "$work/run"
chmod 700 "$work/run"
ln -s "$repo/ryoku/ui" "$work/Ryoku/Ui"
cp -a "$repo/ryoku/hub/quickshell" "$work/hub"
cp "$here/hub-pages-probe.qml" "$work/hub/probe.qml"

# The Hub asks its backends for settings and remembers the last section; stub
# them so the probe never reads or writes the real configuration.
for tool in ryoku-hub ryoku ryoku-shell; do
    printf '#!/usr/bin/env bash\nexit 0\n' >"$work/bin/$tool"
    chmod +x "$work/bin/$tool"
done

env -u HYPRLAND_INSTANCE_SIGNATURE -u NIRI_SOCKET \
PATH="$work/bin:$PATH" \
XDG_RUNTIME_DIR="$work/run" \
XDG_CONFIG_HOME="$work/config" \
XDG_STATE_HOME="$work/state" \
XDG_CACHE_HOME="$work/cache" \
QT_QPA_PLATFORM=offscreen \
QML2_IMPORT_PATH="$work:${QML2_IMPORT_PATH:-$HOME/.local/lib/qt6/qml}" \
    timeout 30 qs -p "$work/hub/probe.qml" >"$work/log" 2>&1 || true
if ! grep -q HUB-PAGES-PROBE-PASS "$work/log"; then
    grep HUB-PAGES-PROBE "$work/log" || sed -n '1,80p' "$work/log"
    exit 1
fi
echo "hub-pages-probe: every Hub section opens a page that loads"
