#!/usr/bin/env bash
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
repo="$here/../.."
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/services" "$work/Ryoku/Ui/Singletons" "$work/bin" "$work/run"
chmod 700 "$work/run"
cp "$repo/ryoku/shell/quickshell/shell/services/"{BatteryAlerts.qml,BatteryAlertPolicy.js} "$work/services/"
cat > "$work/services/qmldir" <<'QML'
singleton BatteryAlerts BatteryAlerts.qml
singleton Battery Battery.qml
singleton Config Config.qml
QML
cat > "$work/services/Battery.qml" <<'QML'
pragma Singleton
import QtQuick
QtObject {
 property bool present: true
 property var batDev: ({ ready: true })
 property bool onAc: false
 property bool discharging: true
 property real frac: 0.25
}
QML
cat > "$work/services/Config.qml" <<'QML'
pragma Singleton
import QtQuick
QtObject { property var batteryAlerts: ({ enabled: true, warningPercent: 25, criticalPercent: 10 }) }
QML
cat > "$work/Ryoku/Ui/Singletons/qmldir" <<'QML'
module Ryoku.Ui.Singletons
singleton I18n I18n.qml
QML
cat > "$work/Ryoku/Ui/Singletons/I18n.qml" <<'QML'
pragma Singleton
import QtQuick
QtObject { function tr(text) { return text; } }
QML
cat > "$work/bin/notify-send" <<'SH'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$BATTERY_PROBE_LOG"
[[ ! -e "$BATTERY_PROBE_FAIL" ]]
SH
chmod +x "$work/bin/notify-send"
cp "$here/battery-alerts-probe.qml" "$work/probe.qml"
env PATH="$work/bin:$PATH" BATTERY_PROBE_LOG="$work/notifications" BATTERY_PROBE_FAIL="$work/fail" \
 XDG_RUNTIME_DIR="$work/run" XDG_CONFIG_HOME="$work/config" XDG_STATE_HOME="$work/state" \
 XDG_CACHE_HOME="$work/cache" QT_QPA_PLATFORM=offscreen QML2_IMPORT_PATH="$work" \
 timeout 20 qs -p "$work/probe.qml" > "$work/log" 2>&1 || true
if ! rg -q BATTERY-ALERTS-PROBE-PASS "$work/log"; then
 cat "$work/log"; exit 1
fi
[[ $(wc -l < "$work/notifications") -eq 12 ]]
rg -q -- '--urgency=normal Battery low Battery at 25%' "$work/notifications"
rg -q -- '--urgency=critical Battery critical Battery at 10%' "$work/notifications"
rg -q 'Critical alerts: 10%. Warning alerts are disabled.' "$work/notifications"
rg -q 'Warning alerts: 25%. Critical alerts are disabled.' "$work/notifications"
rg -q 'Both battery alert levels are disabled.' "$work/notifications"
echo 'battery-alerts-probe: native delivery, discharge reset, deduplication, settings, critical startup, failure retry and independent levels pass'
