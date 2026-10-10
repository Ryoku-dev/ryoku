import QtQuick
import Quickshell
import Quickshell.Io
import "services"
ShellRoot {
    Process { id: fixture }
    property int phase: 0
    function check(ok, label) { if (!ok) throw new Error(label); }
    Timer {
        interval: 700
        running: true
        repeat: true
        onTriggered: {
            switch (phase++) {
            case 0:
                check(BatteryAlerts.alertState.lowSent && !BatteryAlerts.alertState.criticalSent, "warning delivery");
                Battery.frac = 0.24;
                break;
            case 1:
                check(BatteryAlerts.alertState.lowSent, "warning deduplication");
                Battery.frac = 0.10;
                break;
            case 2:
                check(BatteryAlerts.alertState.criticalSent, "critical delivery");
                Battery.onAc = true;
                break;
            case 3:
                check(!BatteryAlerts.alertState.lowSent && !BatteryAlerts.alertState.criticalSent, "AC reset");
                Config.batteryAlerts = { enabled: false, warningPercent: 30, criticalPercent: 12 };
                Battery.onAc = false;
                break;
            case 4:
                check(!BatteryAlerts.alertState.lowSent, "disabled alerts");
                Config.batteryAlerts = { enabled: true, warningPercent: 30, criticalPercent: 12 };
                break;
            case 5:
                check(BatteryAlerts.alertState.lowSent && BatteryAlerts.alertState.criticalSent, "critical startup skips warning");
                BatteryAlerts.testNotification();
                break;
            case 6:
                check(BatteryAlerts.lastResult === "Notification sent.", "test delivery");
                check(BatteryAlerts.alertState.criticalSent, "test preserves discharge state");
                Battery.onAc = true;
                break;
            case 7:
                Battery.frac = 0.30;
                Battery.onAc = false;
                break;
            case 8:
                check(BatteryAlerts.alertState.lowSent && !BatteryAlerts.alertState.criticalSent, "custom threshold");
                Battery.onAc = true;
                fixture.command = ["touch", Quickshell.env("BATTERY_PROBE_FAIL")];
                fixture.running = true;
                break;
            case 9:
                Battery.frac = 0.10;
                Battery.onAc = false;
                break;
            case 10:
                check(!BatteryAlerts.alertState.criticalSent, "failed delivery does not consume alert");
                check(BatteryAlerts.lastResult.indexOf("failed") >= 0, "failure reported");
                fixture.command = ["rm", Quickshell.env("BATTERY_PROBE_FAIL")];
                fixture.running = true;
                break;
            case 11:
                BatteryAlerts.evaluate();
                break;
            case 12:
                check(BatteryAlerts.alertState.criticalSent, "failed delivery can retry");
                Battery.onAc = true;
                Battery.frac = 0.25;
                Config.batteryAlerts = { enabled: true, warningEnabled: false, criticalEnabled: true, warningPercent: 25, criticalPercent: 10 };
                break;
            case 13:
                Battery.onAc = false;
                break;
            case 14:
                check(!BatteryAlerts.alertState.lowSent && !BatteryAlerts.alertState.criticalSent, "critical-only skips warning");
                Battery.frac = 0.10;
                break;
            case 15:
                check(BatteryAlerts.alertState.criticalSent, "critical-only delivery");
                BatteryAlerts.testNotification();
                Battery.onAc = true;
                Config.batteryAlerts = { enabled: true, warningEnabled: true, criticalEnabled: false, warningPercent: 25, criticalPercent: 10 };
                break;
            case 16:
                Battery.onAc = false;
                break;
            case 17:
                check(BatteryAlerts.alertState.lowSent && !BatteryAlerts.alertState.criticalSent, "warning-only delivery below critical threshold");
                BatteryAlerts.testNotification();
                Battery.onAc = true;
                Battery.frac = 0.01;
                Config.batteryAlerts = { enabled: true, warningEnabled: false, criticalEnabled: false, warningPercent: 25, criticalPercent: 10 };
                break;
            case 18:
                Battery.onAc = false;
                break;
            case 19:
                check(!BatteryAlerts.alertState.lowSent && !BatteryAlerts.alertState.criticalSent, "both levels disabled");
                BatteryAlerts.testNotification();
                break;
            case 20:
                check(BatteryAlerts.lastResult === "Notification sent.", "both-off test notification");
                check(!BatteryAlerts.alertState.lowSent && !BatteryAlerts.alertState.criticalSent, "test does not consume disabled levels");
                console.log("BATTERY-ALERTS-PROBE-PASS");
                Qt.quit();
            }
        }
    }
    Component.onCompleted: { void BatteryAlerts.enabled; }
}
