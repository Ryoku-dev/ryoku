#!/usr/bin/env bash
# BluetoothDiscovery must not leave the radio scanning after nobody wants it.
# Quickshell only forwards a discovering write that differs from the last state
# BlueZ reported, so a stop sent while a start is unconfirmed is swallowed and the
# scan ran until the shell restarted (#869). The fake radio below reproduces that
# rule (reads show the confirmed state, a late confirmation) and stands in for
# Quickshell.Bluetooth, so no real adapter or D-Bus is involved.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
svc="$root/ryoku/shell/quickshell/shell/services"
mkdir -p "$tmp/imports/FakeBt" "$tmp/services" "$tmp/utils" "$tmp/config" "$tmp/runtime"
chmod 700 "$tmp/runtime"
sed 's/^import Quickshell.Bluetooth$/import FakeBt/' "$svc/BluetoothDiscovery.qml" >"$tmp/services/BluetoothDiscovery.qml"
cp "$root/ryoku/shell/quickshell/shell/utils/menupoll.js" "$tmp/utils/"
printf 'singleton BluetoothDiscovery BluetoothDiscovery.qml\n' >"$tmp/services/qmldir"
sed 's/^import Quickshell.Bluetooth$/import FakeBt/' "$root/tests/bluetooth-discovery-leak.qml" >"$tmp/shell.qml"

cat >"$tmp/imports/FakeBt/qmldir" <<'EOF'
module FakeBt
singleton Bluetooth Bluetooth.qml
EOF
cat >"$tmp/imports/FakeBt/Bluetooth.qml" <<'EOF'
pragma Singleton
import QtQuick
QtObject {
    property QtObject defaultAdapter: QtObject {
        id: adapter
        property bool enabled: true
        property bool discovering: false
        property bool confirmed: false
        property bool want: false
        property bool reverting: false
        onDiscoveringChanged: {
            if (reverting) return;
            want = discovering;
            reverting = true;
            discovering = confirmed;
            reverting = false;
            settle.restart();
        }
        property Timer settle: Timer {
            interval: 300
            onTriggered: {
                adapter.confirmed = adapter.want;
                adapter.reverting = true;
                adapter.discovering = adapter.confirmed;
                adapter.reverting = false;
            }
        }
    }
}
EOF

XDG_RUNTIME_DIR="$tmp/runtime" XDG_CONFIG_HOME="$tmp/config" QT_QPA_PLATFORM=offscreen \
  QML_IMPORT_PATH="$tmp/imports" QML2_IMPORT_PATH="$tmp/imports" \
  timeout 40s qs -p "$tmp/shell.qml" >"$tmp/output" 2>&1 || {
    cat "$tmp/output"
    exit 1
  }
if [[ -n $(awk '/FAIL:/' "$tmp/output") ]] || [[ -z $(awk '/PASS:/' "$tmp/output") ]]; then
  cat "$tmp/output"
  exit 1
fi
awk '/PASS:/' "$tmp/output"
