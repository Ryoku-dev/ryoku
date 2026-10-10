pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../Singletons"

Column {
    id: root

    property bool busy: false
    property string feedback: ""
    readonly property bool alertsEnabled: {
        Settings.revision;
        return Settings.get("batteryAlerts.enabled") !== false;
    }
    readonly property bool warningEnabled: {
        Settings.revision;
        return Settings.get("batteryAlerts.warningEnabled") !== false;
    }
    readonly property bool criticalEnabled: {
        Settings.revision;
        return Settings.get("batteryAlerts.criticalEnabled") !== false;
    }
    readonly property int warningPercent: {
        Settings.revision;
        var value = Settings.get("batteryAlerts.warningPercent");
        return typeof value === "number" ? Math.max(5, Math.min(50, Math.round(value))) : 25;
    }
    readonly property int criticalPercent: {
        Settings.revision;
        var value = Settings.get("batteryAlerts.criticalPercent");
        return Math.max(1, Math.min(root.warningPercent - 1, typeof value === "number" ? Math.round(value) : 10));
    }

    function save(key, value) {
        root.busy = true;
        root.feedback = "";
        Settings.patch("batteryAlerts." + key, value, function(ok, error) {
            root.busy = false;
            root.feedback = ok ? "" : I18n.tr(error);
        });
    }

    function testNotification() {
        root.busy = true;
        root.feedback = "";
        Settings.send("battery-alerts.test", {}, function(ok, error) {
            root.busy = false;
            root.feedback = ok ? I18n.tr("Test notification requested.") : I18n.tr(error);
        });
    }

    SettingRow {
        width: parent.width
        label: I18n.tr("Low-battery alerts")
        desc: I18n.tr("Warn while the battery is discharging. Alerts respect Do Not Disturb.")
        controlWidth: 54
        enabled: Settings.ready && !root.busy
        Sw {
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            on: root.alertsEnabled
            onToggled: value => root.save("enabled", value)
        }
    }

    SettingRow {
        width: parent.width
        divider: true
        label: I18n.tr("Warning alerts")
        desc: I18n.tr("Send the first reminder to plug in.")
        controlWidth: 54
        enabled: Settings.ready && root.alertsEnabled && !root.busy
        Sw {
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            on: root.warningEnabled
            onToggled: value => root.save("warningEnabled", value)
        }
    }

    SettingRow {
        width: parent.width
        divider: true
        label: I18n.tr("Warning percentage")
        desc: I18n.tr("First reminder to plug in. Must be above the critical percentage.")
        controlWidth: 58
        unit: "%"
        value: String(root.warningPercent)
        editableValue: true
        enabled: Settings.ready && root.alertsEnabled && root.warningEnabled && !root.busy
        onValueCommitted: text => {
            var value = Number(text);
            if (text.trim() !== "" && isFinite(value))
                root.save("warningPercent", Math.max(Math.max(5, root.criticalPercent + 1), Math.min(50, Math.round(value))));
        }
        Step {
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            from: Math.max(5, root.criticalPercent + 1)
            to: 50
            stepBy: 1
            value: root.warningPercent
            onModified: value => root.save("warningPercent", value)
        }
    }

    SettingRow {
        width: parent.width
        divider: true
        label: I18n.tr("Critical alerts")
        desc: I18n.tr("Send the urgent reminder to plug in.")
        controlWidth: 54
        enabled: Settings.ready && root.alertsEnabled && !root.busy
        Sw {
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            on: root.criticalEnabled
            onToggled: value => root.save("criticalEnabled", value)
        }
    }

    SettingRow {
        width: parent.width
        divider: true
        label: I18n.tr("Critical percentage")
        desc: I18n.tr("Urgent reminder to plug in. Must be below the warning percentage.")
        controlWidth: 58
        unit: "%"
        value: String(root.criticalPercent)
        editableValue: true
        enabled: Settings.ready && root.alertsEnabled && root.criticalEnabled && !root.busy
        onValueCommitted: text => {
            var value = Number(text);
            if (text.trim() !== "" && isFinite(value))
                root.save("criticalPercent", Math.max(1, Math.min(root.warningPercent - 1, Math.round(value))));
        }
        Step {
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            from: 1
            to: root.warningPercent - 1
            stepBy: 1
            value: root.criticalPercent
            onModified: value => root.save("criticalPercent", value)
        }
    }

    SettingRow {
        width: parent.width
        divider: true
        label: I18n.tr("Test notification")
        desc: root.feedback !== "" ? root.feedback : I18n.tr("Preview a battery alert without changing your battery level.")
        controlWidth: testButton.implicitWidth
        Btn {
            id: testButton
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("Test")
            armed: Settings.ready && root.alertsEnabled && !root.busy
            onAct: root.testNotification()
        }
    }
}
