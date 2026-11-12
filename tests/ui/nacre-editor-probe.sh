#!/usr/bin/env bash
# the Bar page's Nacre editor: lane moves and appearance rows store the nacre keys
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
repo="$here/../.."
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

mkdir -p "$work/Ryoku" "$work/bin" "$work/run" "$work/state/ryoku/store"
chmod 700 "$work/run"
ln -s "$repo/ryoku/ui" "$work/Ryoku/Ui"
ln -s "$repo/ryoku/shell/framebars" "$work/Ryoku/FrameBars"
ln -s "$repo/docs/lithic/library/qml/Lithic" "$work/Lithic"
cp -a "$repo/ryoku/hub/quickshell" "$work/hub"
cp "$here/nacre-editor-probe.qml" "$work/hub/probe.qml"

# Nacre in use: the shells index names it as the fallback with its Hub editor
cat >"$work/state/ryoku/store/shells.json" <<'EOF'
{"schema": 1, "active": "nacre", "fallback": "nacre", "items": [
 {"id": "nacre", "category": "barstyles", "source": "store", "name": "Nacre", "version": "1.0.0",
  "summary": "", "owns": [], "ownsSetting": {}, "settings": {"hubEditor": "nacre"},
  "provides": {"dock": [], "launcher": [], "desktopWidgets": []}}]}
EOF

# no backend, store or shell call may reach the real system; the desktop
# store reads an empty document so the page has what it waits for
for tool in ryoku ryoku-shell ryostore; do
    printf '#!/usr/bin/env bash\nexit 0\n' >"$work/bin/$tool"
    chmod +x "$work/bin/$tool"
done
cat >"$work/bin/ryoku-hub" <<'EOF'
#!/usr/bin/env bash
case "$1 $2" in
"desktop get" | "desktop defaults") echo '{}' ;;
"desktop schema" | "desktop unhonored") echo '[]' ;;
esac
exit 0
EOF
chmod +x "$work/bin/ryoku-hub"

env -u HYPRLAND_INSTANCE_SIGNATURE -u NIRI_SOCKET \
PATH="$work/bin:$PATH" \
HOME="$work/home" \
XDG_RUNTIME_DIR="$work/run" \
XDG_CONFIG_HOME="$work/config" \
XDG_STATE_HOME="$work/state" \
XDG_CACHE_HOME="$work/cache" \
QT_QPA_PLATFORM=offscreen \
QML2_IMPORT_PATH="$work:${QML2_IMPORT_PATH:-$HOME/.local/lib/qt6/qml}" \
    timeout 40 qs -p "$work/hub/probe.qml" >"$work/log" 2>&1 || true
if ! grep -q NACRE-EDITOR-PROBE-PASS "$work/log"; then
    grep NACRE-EDITOR-PROBE "$work/log" || sed -n '1,100p' "$work/log"
    exit 1
fi
if grep -Eq 'TypeError|ReferenceError' "$work/log"; then
    sed -n '1,100p' "$work/log"
    exit 1
fi
echo "nacre-editor-probe: lane moves and appearance rows store the nacre layout"
