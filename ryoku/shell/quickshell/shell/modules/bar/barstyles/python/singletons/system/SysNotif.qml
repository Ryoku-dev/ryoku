pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Services.UPower
import "../../"

Item {
    id: root

    readonly property bool isDesktop: UPower.displayDevice.ready ? !UPower.displayDevice.isLaptopBattery : (typeof SystemInfo !== "undefined" ? SystemInfo.isDesktop : true)
    readonly property int batteryPercentage: UPower.displayDevice.ready ? Math.round(UPower.displayDevice.percentage * 100) : 0
    readonly property bool isCharging: UPower.displayDevice.ready && (UPower.displayDevice.state === UPowerDeviceState.Charging || UPower.displayDevice.state === UPowerDeviceState.FullyCharged)

    property bool notifiedFull: false
    property real lastNotifTime: 0
    property string lastNotifType: ""

    onIsDesktopChanged: root.checkBattery()

    function sendNotification(type, summary, body, icon, urgency) {
        let now = Date.now();
        if (root.lastNotifType === type && (now - root.lastNotifTime < 60000)) {
            return;
        }
        if (now - root.lastNotifTime < 3000) {
            return;
        }
        root.lastNotifType = type;
        root.lastNotifTime = now;

        let u = urgency ? urgency : "normal";
        let ic = icon ? icon : "battery";
        let appName = I18n.t("sysnotif.battery.app_name");
        Quickshell.execDetached([
            "notify-send",
            "-a", appName,
            "-u", u,
            "-i", ic,
            summary,
            body
        ]);
    }

    function checkBattery() {
        if (root.isDesktop || !UPower.displayDevice.ready || (typeof I18n !== "undefined" && !I18n.isReady)) return;

        let pct = root.batteryPercentage;
        let state = UPower.displayDevice.state;
        let charging = state === UPowerDeviceState.Charging || state === UPowerDeviceState.FullyCharged;

        if (pct < 95) {
            root.notifiedFull = false;
        }

        if (charging) {
            if ((pct >= 100 || state === UPowerDeviceState.FullyCharged) && !root.notifiedFull) {
                root.notifiedFull = true;
                root.sendNotification(
                    "full",
                    I18n.t("sysnotif.battery.full_title"),
                    I18n.t("sysnotif.battery.full_body"),
                    "battery-full-charged",
                    "normal"
                );
            }
        }
        // Discharge alerts are owned by the global BatteryAlerts service.
    }

    Connections {
        target: (typeof I18n !== "undefined") ? I18n : null
        function onLanguageChanged() { root.checkBattery(); }
    }

    Connections {
        target: UPower.displayDevice
        function onPercentageChanged() { root.checkBattery(); }
        function onStateChanged() { root.checkBattery(); }
        function onReadyChanged() { root.checkBattery(); }
    }
}
