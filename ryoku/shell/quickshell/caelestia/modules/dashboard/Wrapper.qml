pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Caelestia
import Caelestia.Config
import caelestia.components
import caelestia.components.filedialog
import caelestia.utils

Item {
    id: root

    required property DrawerVisibilities visibilities
    // The dashboard's tab and the month its calendar shows are the screen's own
    // state, not this widget's: that is where upstream keeps them, and it is what
    // makes both survive the dashboard closing and a shell reload.
    required property ScreenState screenState
    readonly property FileDialog facePicker: FileDialog {
        title: qsTr("Select a profile picture")
        filterLabel: qsTr("Image files")
        filters: Images.validImageExtensions
        onAccepted: path => {
            if (CUtils.copyFile(Qt.resolvedUrl(path), Qt.resolvedUrl(`${Paths.home}/.face`)))
                Quickshell.execDetached(["notify-send", "-a", "caelestia-shell", "-u", "low", "-h", `STRING:image-path:${path}`, qsTr("Profile picture changed"), qsTr("Profile picture changed to %1").arg(Paths.shortenHome(path))]);
            else
                Quickshell.execDetached(["notify-send", "-a", "caelestia-shell", "-u", "critical", qsTr("Unable to change profile picture"), qsTr("Failed to change profile picture to %1").arg(Paths.shortenHome(path))]);
        }
    }
    readonly property real nonAnimHeight: (content.item as Content)?.nonAnimHeight ?? 0
    readonly property bool shouldBeActive: visibilities.dashboard && Config.dashboard.enabled && !visibilities.overview
    property real offsetScale: shouldBeActive ? 0 : 1
    property bool keepAlive: false

    clip: Config.bar.position === "top"
    visible: offsetScale < 1
    anchors.topMargin: (Config.bar.position === "top" ? 0 : -implicitHeight - 5) * offsetScale
    height: Config.bar.position === "top" ? implicitHeight * (1 - offsetScale) : implicitHeight
    implicitHeight: content.implicitHeight
    implicitWidth: content.implicitWidth || 854
    opacity: 1 - offsetScale

    Timer {
        running: Config.dashboard.enabled && !root.keepAlive
        interval: 2500

        onTriggered: root.keepAlive = true
    }

    Behavior on offsetScale {
        Anim {}
    }
    Loader {
        id: content

        anchors.horizontalCenter: parent.horizontalCenter
        anchors.bottom: parent.bottom
        active: root.shouldBeActive || root.visible || root.keepAlive
        sourceComponent: Content {
            visibilities: root.visibilities
            screenState: root.screenState
            facePicker: root.facePicker
        }
    }
}
