pragma Singleton
pragma ComponentBehavior: Bound
import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "BatteryAlertPolicy.js" as Policy

// One monitor per QML engine, even with several physical displays.
Singleton {
    id: svc
    readonly property var settings: Config.batteryAlerts
    readonly property bool enabled: settings.enabled ?? true
    readonly property bool warningEnabled: settings.warningEnabled ?? true
    readonly property bool criticalEnabled: settings.criticalEnabled ?? true
    readonly property int lowThreshold: Math.max(5, Math.min(50, Number(settings.warningPercent ?? 25)))
    readonly property int criticalThreshold: Math.max(1, Math.min(lowThreshold - 1, Number(settings.criticalPercent ?? 10)))
    readonly property bool present: Battery.present
    readonly property bool ready: present && Battery.batDev.ready
    readonly property bool onAc: Battery.onAc
    readonly property bool discharging: Battery.discharging
    readonly property real percent: ready ? Battery.frac * 100 : NaN
    readonly property int percentage: Number.isFinite(percent) ? Math.round(percent) : -1
    property string lastResult: ""
    property var alertState: Policy.initialState()
    property var pendingState: null
    property int session: 0
    property int pendingSession: 0
    property bool testing: false

    // The Hub tests the same notification path without changing discharge state.
    IpcHandler {
        target: "battery-alerts-native"
        function test(): void { svc.testNotification(); }
        function status(): string {
            return JSON.stringify({ percent: svc.percentage,
                enabled: svc.enabled, warningEnabled: svc.warningEnabled, criticalEnabled: svc.criticalEnabled, onAc: svc.onAc, ready: svc.ready,
                warning: svc.lowThreshold, critical: svc.criticalThreshold,
                lastResult: svc.lastResult });
        }
    }

    function evaluate() {
        if (onAc) { alertState = Policy.initialState(); return; }
        if (notifier.running) return;
        var result = Policy.evaluate(alertState,
            { present: present, ready: ready, onAc: onAc, discharging: discharging, percent: percent },
            { enabled: enabled, warningEnabled: warningEnabled, criticalEnabled: criticalEnabled, low: lowThreshold, critical: criticalThreshold });
        if (!result.level) return;
        pendingState = result.state;
        pendingSession = session;
        testing = false;
        send(result.level === "critical" ? I18n.tr("Battery critical") : I18n.tr("Battery low"),
            I18n.tr("Battery at %1%. Connect your charger.").arg(percentage), result.level === "critical");
    }
    function send(title, body, critical) {
        notifier.command = ["notify-send", "--app-name=Battery Alerts", "--icon=battery-caution-symbolic",
            critical ? "--urgency=critical" : "--urgency=normal", title, body];
        lastResult = I18n.tr("Sending notification…");
        notifier.running = true;
    }
    function testNotification() {
        if (notifier.running) return;
        testing = true;
        pendingState = null;
        var body;
        if (warningEnabled && criticalEnabled)
            body = I18n.tr("Low-battery alerts are ready. Warning: %1%. Critical: %2%.").arg(lowThreshold).arg(criticalThreshold);
        else if (warningEnabled)
            body = I18n.tr("Warning alerts: %1%. Critical alerts are disabled.").arg(lowThreshold);
        else if (criticalEnabled)
            body = I18n.tr("Critical alerts: %1%. Warning alerts are disabled.").arg(criticalThreshold);
        else
            body = I18n.tr("Both battery alert levels are disabled. Test notifications still work.");
        send(I18n.tr("Battery Alerts test"), body, false);
    }
    onOnAcChanged: {
        session++;
        if (onAc) alertState = Policy.initialState();
        soon.restart();
    }
    onPercentChanged: soon.restart()
    onReadyChanged: soon.restart()
    onDischargingChanged: soon.restart()
    onSettingsChanged: soon.restart()
    onEnabledChanged: soon.restart()
    Component.onCompleted: soon.restart()
    Timer { id: soon; interval: 300; onTriggered: svc.evaluate() }
    // Alerts stay alive while their panel is closed or bar hidden.
    Timer {
        interval: 30000
        running: svc.enabled && svc.present && !svc.onAc
        repeat: true
        onTriggered: svc.evaluate()
    }
    Process {
        id: notifier
        onExited: (code) => {
            if (code === 0) {
                if (!svc.testing && svc.pendingState && svc.pendingSession === svc.session && !svc.onAc)
                    svc.alertState = svc.pendingState;
                svc.lastResult = I18n.tr("Notification sent.");
            } else {
                svc.lastResult = I18n.tr("Notification failed. Check notify-send and the notification service.");
            }
            svc.pendingState = null;
            // The periodic timer retries failures; avoid an immediate retry loop.
            if (code === 0) soon.restart();
        }
    }
}
