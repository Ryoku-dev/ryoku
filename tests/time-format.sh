#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/imports/Ryoku" "$tmp/config/ryoku" "$tmp/runtime"
ln -s "$root/ryoku/ui" "$tmp/imports/Ryoku/Ui"
mkdir -p "$tmp/ryoku/hub"
cp -r "$root/ryoku/hub/quickshell" "$tmp/ryoku/hub/"
sed 's|"../ryoku/|"./ryoku/|g' "$root/tests/time-format.qml" > "$tmp/shell.qml"
printf '%s\n' '{}' > "$tmp/config/ryoku/shell.json"
XDG_RUNTIME_DIR="$tmp/runtime" QT_QPA_PLATFORM=offscreen QML_IMPORT_PATH="$tmp/imports:${QML_IMPORT_PATH:-}" QML2_IMPORT_PATH="$tmp/imports:${QML2_IMPORT_PATH:-}" \
    XDG_CONFIG_HOME="$tmp/config" RYOKU_TIME_TEST_FILE="$tmp/config/ryoku/shell.json" \
    timeout 15s qs -p "$tmp/shell.qml" > "$tmp/output" 2>&1 || {
        cat "$tmp/output"
        exit 1
    }
if grep -q 'FAIL:' "$tmp/output" || ! grep -q 'PASS:' "$tmp/output"; then
    cat "$tmp/output"
    exit 1
fi
grep 'PASS:' "$tmp/output"
