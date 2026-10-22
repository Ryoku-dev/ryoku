pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import caelestia.components
import caelestia.services
import caelestia.modules.windowinfo as WInfo

Item {
    id: root

    required property ShellScreen screen
    required property DrawerVisibilities visibilities
    required property var panels
    property var animConfig
    property alias windowGrid: windowGrid

    Connections {
        function onOverviewChanged() {
            if (root.visibilities.overview) {
                windowGrid.forceActiveFocus()
            }
        }

        target: root.visibilities
    }
    WindowGrid {
        id: windowGrid

        panels: root.panels
        screen: root.screen
        anchors.fill: parent
        opacity: root.visibilities.overview ? 1 : 0
        onRequestWindowInfo: client => {
            windowGrid.activeInfoClient = client
            windowInfoOverlay.clientAddress = client.address
            windowInfoOverlay.isOpen = true
        }
        onRequestClose: {
            Visibilities.setOverview(false)
        }

        Behavior on opacity { NumberAnimation { duration: root.animConfig ? root.animConfig.gridDuration : 1500; easing.type: root.animConfig ? root.animConfig.easingType : Easing.OutCubic } }
    }
    Shortcut {
        sequence: "Escape"
        onActivated: Visibilities.setOverview(false)
        enabled: root.visibilities.overview
    }
    Item {
        id: windowInfoOverlay

        property string clientAddress: ""
        property bool isOpen: false

        z: 100
        anchors.fill: parent
        visible: opacity > 0
        opacity: isOpen ? 1 : 0
        onOpacityChanged: {
            if (opacity <= 0 && !isOpen) {
                windowInfoOverlay.clientAddress = ""
                windowGrid.activeInfoClient = null
            }
        }

        Behavior on opacity {
            NumberAnimation { duration: 250; easing.type: Easing.OutCubic }
        }
        Rectangle {
            anchors.fill: parent
            color: Qt.rgba(0, 0, 0, 0.5)

            HoverHandler { }
            WheelHandler { }
            TapHandler {
                onTapped: {
                    windowInfoOverlay.isOpen = false
                }
            }
        }
        Item {
            anchors.centerIn: parent
            width: Math.min(parent.width * 0.8, 900)
            height: Math.min(parent.height * 0.8, 600)

            Behavior on width { NumberAnimation { duration: 250; easing.type: Easing.OutCubic } }
            Behavior on height { NumberAnimation { duration: 250; easing.type: Easing.OutCubic } }
            TapHandler { }
            WInfo.WindowInfo {
                id: winInfoItem

                anchors.fill: parent
                clientAddress: windowInfoOverlay.clientAddress
                border.width: 2
                border.color: Colours.palette.m3primary
                onCloseRequested: {
                    windowInfoOverlay.isOpen = false
                }
            }
        }
    }
}
