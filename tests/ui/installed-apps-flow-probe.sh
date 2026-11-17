#!/usr/bin/env bash
# Settings › Apps › Installed apps: a removal's progress line driven by real progress events.
# A stand-in pkexec and ryoku-host print what `ryoku-host apps remove` prints; nothing is removed.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
repo="$here/../.."
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

mkdir -p "$work/Ryoku" "$work/bin" "$work/run"
chmod 700 "$work/run"
ln -s "$repo/ryoku/ui" "$work/Ryoku/Ui"
ln -s "$repo/docs/lithic/library/qml/Lithic" "$work/Lithic"
cp -a "$repo/ryoku/hub/quickshell" "$work/hub"
cp "$here/installed-apps-flow-probe.qml" "$work/hub/probe.qml"

# pkexec: "--closed" answers as a password prompt the user closed
cat >"$work/bin/pkexec" <<'EOF'
#!/usr/bin/env bash
[ "$1" = --closed ] && exit 126
exec "$@"
EOF
# ryoku-host apps --stdin-cancel remove --manager <m> <name>: the events, counted; "missing" fails
# as pacman would; "slow" runs until its stdin says cancel or closes, then ends as ryoku-host does
cat >"$work/bin/ryoku-host" <<'EOF'
#!/usr/bin/env bash
name="${*: -1}"
[ "$2" = --stdin-cancel ] || { echo "no --stdin-cancel" >&2; exit 2; }
printf '{"event":"plan","action":"remove","steps":[{"manager":"pacman","name":"%s","version":"1"}]}\n' "$name"
printf '{"event":"step","index":0,"phase":"start"}\n'
if [ "$name" = missing ]; then
    sleep 0.2
    echo "error: target not found: missing" >&2
    printf '{"event":"step","index":0,"phase":"done","exit":1}\n'
    printf '{"event":"done","ok":false,"exit":1,"error":"error: target not found: missing"}\n'
    exit 1
fi
if [ "$name" = slow ]; then
    while IFS= read -r l && [ "$l" != cancel ]; do :; done
    printf '{"event":"done","ok":false,"exit":130,"error":"cancelled"}\n'
    exit 130
fi
for i in 1 2 3 4; do
    sleep 0.45
    printf '{"event":"log","index":0,"line":"(%d/4) removing %s"}\n' "$i" "$name"
    printf '{"event":"progress","index":0,"done":%d,"total":4}\n' "$i"
done
printf '{"event":"step","index":0,"phase":"done","exit":0}\n'
printf '{"event":"done","ok":true,"exit":0,"error":""}\n'
EOF
# gpk remove -m flatpak --yes --events <id>, run as you: a slow removal that stops on SIGTERM
# and says so in its last event, as gpk does
cat >"$work/bin/gpk" <<'EOF'
#!/usr/bin/env bash
name="${*: -1}"
trap 'printf "{\"event\":\"done\",\"ok\":false,\"exit\":130,\"error\":\"stopped\"}\n"; exit 130' TERM
printf '{"event":"plan","action":"remove","steps":[{"manager":"flatpak","name":"%s","version":"1"}]}\n' "$name"
printf '{"event":"step","index":0,"phase":"start"}\n'
for i in 1 2 3 4 5 6 7 8 9 10; do sleep 0.5 & wait $!; done
printf '{"event":"done","ok":true,"exit":0,"error":""}\n'
EOF
chmod +x "$work/bin/pkexec" "$work/bin/ryoku-host" "$work/bin/gpk"

env -u HYPRLAND_INSTANCE_SIGNATURE -u NIRI_SOCKET -u WAYLAND_DISPLAY \
PATH="$work/bin:$PATH" \
XDG_RUNTIME_DIR="$work/run" \
XDG_CONFIG_HOME="$work/config" \
XDG_STATE_HOME="$work/state" \
XDG_CACHE_HOME="$work/cache" \
QT_QPA_PLATFORM=offscreen \
QML2_IMPORT_PATH="$work:${QML2_IMPORT_PATH:-$HOME/.local/lib/qt6/qml}" \
    timeout 40 qs -p "$work/hub/probe.qml" >"$work/log" 2>&1 || true
if ! grep -q INSTALLED-APPS-FLOW-PASS "$work/log"; then
    grep INSTALLED-APPS-FLOW "$work/log" || sed -n '1,80p' "$work/log"
    exit 1
fi
echo "installed-apps-flow-probe: progress events drive the line to done; a failure shows Retry"
