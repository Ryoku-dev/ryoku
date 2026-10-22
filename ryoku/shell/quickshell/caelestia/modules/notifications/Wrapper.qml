pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Caelestia.Config
import caelestia.components
import caelestia.services

Item {
    id: root

    required property DrawerVisibilities visibilities
    required property Item sidebarPanel
    property ShellScreen screen
    property alias osdPanel: content.osdPanel
    property alias sessionPanel: content.sessionPanel
    property alias utilitiesPanel: content.utilitiesPanel
    readonly property real baseTopMargin: -5

    readonly property bool isTargetScreen: {
        if (GlobalConfig.notifs.monitor !== "focused")
            return true;
        const targetOutput = Notifs.activeTargetOutput || Notifs.getTargetOutput();
        return targetOutput !== "" && targetOutput === root.screen?.name;
    }
    readonly property bool isAllowedByFullscreen: {
        if (GlobalConfig.notifs.fullscreen === "on")
            return true;
        return !Kwin.hasFullscreenOn(root.screen?.name ?? "");
    }

    visible: height > 0 && !visibilities.overview && isTargetScreen && isAllowedByFullscreen
    anchors.topMargin: baseTopMargin
    implicitWidth: (isTargetScreen && isAllowedByFullscreen) ? Math.max(sidebarPanel.width, content.implicitWidth) : 0
    implicitHeight: (isTargetScreen && isAllowedByFullscreen) ? content.implicitHeight : 0

    Content {
        id: content

        anchors.topMargin: -root.baseTopMargin
        visibilities: root.visibilities
    }
}
