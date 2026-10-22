pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Caelestia.Config
import Caelestia.Services
import caelestia.services
import caelestia.utils

Singleton {
    id: root

    readonly property var controller: Nmcli.hotspot

    function toggle(): void {
        if (root.controller.busy)
            return;

        if (root.controller.enabled) {
            root.controller.disable(root.report);
            return;
        }

        const { hotspotSsid, hotspotPassword } = GlobalConfig.services;
        if (hotspotSsid.length === 0 && hotspotPassword.length === 0) {
            Toaster.toast(qsTr("Hotspot"), qsTr("Give it a name and a password in Settings > Network > Hotspot"), "wifi_tethering");
            return;
        }

        root.controller.enable(resolveName(hotspotSsid), hotspotPassword, root.report);
    }

    function resolveName(ssid: string): string {
        return ssid.length > 0 ? ssid : (SysInfo.hostname || "caelestia");
    }

    function report(result: var): void {
        if (!result || result.success)
            return;
        Toaster.toast(qsTr("Hotspot"), result.error || qsTr("The hotspot could not be started"), "wifi_tethering");
    }
}
