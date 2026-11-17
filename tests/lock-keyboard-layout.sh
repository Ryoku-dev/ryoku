#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'kill "${srv:-0}" 2>/dev/null || true; rm -rf "$tmp"' EXIT
mkdir -p "$tmp/bin" "$tmp/runtime" "$tmp/imports/Ryoku"
ln -s "$root/ryoku/ui" "$tmp/imports/Ryoku/Ui"
cp "$root/ryoku/lockscreen/qylock/quickshell-lockscreen/LayoutFeed.qml" \
    "$root/ryoku/lockscreen/qylock/quickshell-lockscreen/LayoutBadge.qml" "$tmp/"
cp "$root/tests/lock-keyboard-layout.qml" "$tmp/shell.qml"
printf '#!/bin/sh\nprintf "%%s\\n" "$*" >> "%s/calls"\n' "$tmp" > "$tmp/bin/ryoku"
chmod +x "$tmp/bin/ryoku"
: > "$tmp/calls"
# a stand-in for the shell daemon: replies to the subscription, and sends the
# Russian frame once the lock asked the provider to cycle
python3 - "$tmp" <<'PY' &
import json, os, socket, sys, time
tmp = sys.argv[1]
path = tmp + "/runtime/ryoku-shell.sock"
s = socket.socket(socket.AF_UNIX)
s.bind(path)
s.listen(1)
c, _ = s.accept()
c.recv(64)
def send(o): c.sendall((json.dumps(o) + "\n").encode())
send({"caps": {"keyboardLayoutSwitch": True}, "keyboardLayout": "English (US)", "versions": {"keyboard": 3}})
while "keyboard.cycleLayout" not in open(tmp + "/calls").read():
    time.sleep(0.05)
send({"caps": {"keyboardLayoutSwitch": True}, "keyboardLayout": "Russian", "versions": {"keyboard": 4}})
time.sleep(0.4)
send({"caps": {"keyboardLayoutSwitch": False}, "keyboardLayout": "English (US)", "versions": {"keyboard": 3}})
time.sleep(3)
PY
srv=$!
for _ in $(seq 50); do [ -S "$tmp/runtime/ryoku-shell.sock" ] && break; sleep 0.1; done
PATH="$tmp/bin:$PATH" XDG_RUNTIME_DIR="$tmp/runtime" HOME="$tmp" QT_QPA_PLATFORM=offscreen \
    QML_IMPORT_PATH="$tmp/imports:$root/ryoku/shell/quickshell:${QML_IMPORT_PATH:-}" \
    QML2_IMPORT_PATH="$tmp/imports:$root/ryoku/shell/quickshell:${QML2_IMPORT_PATH:-}" \
    XDG_CONFIG_HOME="$tmp/config" timeout 20s qs -p "$tmp/shell.qml" > "$tmp/output" 2>&1 || {
        cat "$tmp/output"
        exit 1
    }
if grep -q 'FAIL:' "$tmp/output" || ! grep -q 'PASS:' "$tmp/output"; then
    cat "$tmp/output"
    exit 1
fi
grep -qx 'wm act keyboard.cycleLayout' "$tmp/calls" || { echo "FAIL: the lock never asked the provider to cycle"; exit 1; }
grep 'PASS:' "$tmp/output"
