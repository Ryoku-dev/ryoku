#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/bin" "$tmp/runtime" "$tmp/assets/pam" "$tmp/.config/qylock/pam"
cp -r "$root/ryoku/lockscreen/qylock/quickshell-lockscreen/shim" "$tmp/shim"
cp "$root/tests/lock-security-key-routing.qml" "$tmp/shell.qml"
printf '#!/bin/sh\necho yes\n' > "$tmp/bin/loginctl"
printf '#!/bin/sh\nexec sleep 60\n' > "$tmp/bin/dbus-monitor"
printf '#!/bin/sh\nexit 1\n' > "$tmp/bin/fprintd-list"
printf '#!/bin/sh\nif [ "$1 $2" = "security-key check-lock" ]; then exit 0; fi\nexit 1\n' > "$tmp/bin/ryoku"
printf '#!/bin/sh\necho "$$" >> "$HOME/udevadm-starts"\ntrap "exit 0" TERM INT\nwhile :; do sleep 1; done\n' > "$tmp/bin/udevadm"
chmod +x "$tmp/bin/"*
printf '%s\n' 'auth required pam_deny.so' 'account required pam_deny.so' > "$tmp/assets/pam/ryoku-lock"
printf '%s\n' 'auth required pam_exec.so expose_authtok /bin/false' 'account required pam_deny.so' > "$tmp/assets/pam/ryoku-lock-pw"
printf '%s\n' 'auth required pam_exec.so expose_authtok /bin/false' 'account required pam_deny.so' > "$tmp/.config/qylock/pam/ryoku-lock-key"
PATH="$tmp/bin:$PATH" XDG_SESSION_ID=lock-security-key XDG_RUNTIME_DIR="$tmp/runtime" \
    XDG_CONFIG_HOME="$tmp/.config" HOME="$tmp" QT_QPA_PLATFORM=offscreen \
    timeout 15s qs -p "$tmp/shell.qml" > "$tmp/output" 2>&1 || {
        cat "$tmp/output"
        exit 1
    }
if grep -q 'FAIL:' "$tmp/output" || ! grep -q 'PASS:' "$tmp/output"; then
    cat "$tmp/output"
    exit 1
fi
if [[ $(wc -l < "$tmp/udevadm-starts") -ne 1 ]]; then
    echo "FAIL: udev monitor did not stay single-instance"
    exit 1
fi
monitor_pid="$(cat "$tmp/udevadm-starts")"
for _ in {1..20}; do
    kill -0 "$monitor_pid" 2>/dev/null || break
    sleep 0.05
done
if kill -0 "$monitor_pid" 2>/dev/null; then
    echo "FAIL: udev monitor leaked after the lock probe exited"
    exit 1
fi
grep -o 'PASS:.*' "$tmp/output"
echo "PASS: one udev monitor was started and reaped"
