pragma ComponentBehavior: Bound

import QtQuick
import Caelestia.Config
import caelestia.components

Item {
    id: root

    required property DrawerVisibilities visibilities
    required property bool sidebarVisible
    readonly property real nonAnimWidth: content.implicitWidth
    readonly property bool shouldBeActive: visibilities.session && Config.session.enabled && !visibilities.overview
    property real offsetScale: shouldBeActive ? 0 : 1
    property real sidebarOffset: sidebarVisible ? 14 : 0

    visible: offsetScale < 1
    anchors.leftMargin: Config.bar.position === "right" ? (-implicitWidth - 5 - sidebarOffset) * offsetScale : 0
    anchors.rightMargin: Config.bar.position !== "right" ? (-implicitWidth - 5 - sidebarOffset) * offsetScale : 0
    implicitWidth: content.implicitWidth
    implicitHeight: content.implicitHeight || 510
    opacity: 1 - offsetScale

    Behavior on offsetScale {
        Anim {}
    }
    Loader {
        id: content

        anchors.verticalCenter: parent.verticalCenter
        anchors.left: parent.left
        active: root.shouldBeActive || root.visible
        sourceComponent: Content {
            visibilities: root.visibilities
        }
    }
}
