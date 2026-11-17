#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/bin" "$tmp/runtime" "$tmp/assets/pam" "$tmp/config/qylock"
cp -r "$root/ryoku/lockscreen/qylock/quickshell-lockscreen/shim" "$tmp/shim"
cp "$root/tests/lock-auth-retry.qml" "$tmp/shell.qml"
printf '#!/bin/sh\necho yes\n' > "$tmp/bin/loginctl"
printf '#!/bin/sh\nexec sleep 60\n' > "$tmp/bin/dbus-monitor"
printf '#!/bin/sh\nexit 1\n' > "$tmp/bin/fprintd-list"
chmod +x "$tmp/bin/"*
printf '%s\n' 'auth required pam_deny.so' 'account required pam_deny.so' > "$tmp/assets/pam/ryoku-lock"
# the stack prompts, then always refuses, so no run can unlock
printf '%s\n' "auth required pam_exec.so expose_authtok $(command -v false)" 'account required pam_deny.so' \
    > "$tmp/assets/pam/ryoku-lock-pw"

run() {
    local mode="$1"
    PATH="$tmp/bin:$PATH" XDG_SESSION_ID=lock-auth-retry XDG_RUNTIME_DIR="$tmp/runtime" \
        HOME="$tmp" XDG_CONFIG_HOME="$tmp/config" QT_QPA_PLATFORM=offscreen LOCK_RETRY_RUN="$mode" \
        timeout 15s qs -p "$tmp/shell.qml" > "$tmp/output" 2>&1 || {
            cat "$tmp/output"
            exit 1
        }
    if grep -q 'FAIL:' "$tmp/output" || ! grep -q 'PASS:' "$tmp/output"; then
        cat "$tmp/output"
        exit 1
    fi
    echo "$mode: $(grep -o 'PASS:.*' "$tmp/output")"
}

run off
printf 'on\n' > "$tmp/config/qylock/auth-retry"
run on
