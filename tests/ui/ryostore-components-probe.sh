#!/usr/bin/env bash
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
repo="$here/../.."
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

mkdir -p "$work/Ryoku" "$work/bin"
ln -s "$repo/ryoku/ui" "$work/Ryoku/Ui"
cp -a "$repo/ryoku/apps/ryostore/quickshell" "$work/ryostore"
cp "$here/ryostore-components-probe.qml" "$work/probe.qml"
printf '%s' 'R0lGODlhBAACAPAAALI6SAAAACH5BAAAAAAALAAAAAAEAAIAAAIDhG8FADs=' | base64 -d >"$work/aspect.gif"

cat >"$work/bin/ryostore" <<'SCRIPT'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"$RYOSTORE_COMMAND_LOG"
case "${1:-}" in
    catalog)
        if [[ -e "$RYOSTORE_INSTALL_MARKER" ]]; then sleep 0.12; fi
        printf '%s\n' '{"categories":[],"items":[],"offline":false}'
        ;;
    install) : >"$RYOSTORE_INSTALL_MARKER" ;;
esac
SCRIPT
chmod +x "$work/bin/ryostore"

PATH="$work/bin:$PATH" \
RYOSTORE_COMMAND_LOG="$work/commands" \
RYOSTORE_INSTALL_MARKER="$work/install-started" \
RYOSTORE_ASPECT_GIF="file://$work/aspect.gif" \
QML2_IMPORT_PATH="$work:${QML2_IMPORT_PATH:-$HOME/.local/lib/qt6/qml}" \
    timeout 20 qs -p "$work/probe.qml" >"$work/log" 2>&1 || true
if ! grep -q RYOSTORE-COMPONENTS-PROBE-PASS "$work/log"; then
    sed -n '1,160p' "$work/log"
    exit 1
fi
if grep -Eq '(^|[[:space:]])ERROR|TypeError|ReferenceError' "$work/log"; then
    sed -n '1,160p' "$work/log"
    exit 1
fi
if ! grep -Fxq 'install lockscreens broken' "$work/commands"; then
    echo "missing retry command: install lockscreens broken"
    sed -n '1,40p' "$work/commands"
    exit 1
fi
echo "ryostore-components-probe: media, gallery, update, and status states"
