import QtQuick
import "../modules"
import Quickshell
import Quickshell.Io
import Quickshell.Wayland
import shell.services
import Ryoku.Ui.Singletons

PanelWindow {
    id: profilePanel
    required property var root

    screen: root.activePopupScreen

    color: "transparent"
    anchors { top: true; bottom: true; left: true; right: true }
    exclusionMode: ExclusionMode.Ignore
    WlrLayershell.layer: WlrLayer.Overlay
    WlrLayershell.namespace: "ryoku-power-profile"

    readonly property int barBottom: root.v2BarHeight
    readonly property int gap: 6

    readonly property var allProfiles: [
        { key: "power-saver",  icon: "\uF06C",  label: I18n.tr("Power Saver") },
        { key: "balanced",     icon: "\uF24E", label: I18n.tr("Balanced") },
        { key: "performance",  icon: "\uF0E7", label: I18n.tr("Performance") },
    ]

    // Only offer profiles the daemon reports (root.powerProfileAvailable),
    // keeping the canonical order and look. Falls back to all three if the
    // stream is down, so a shown button always applies and nothing regresses.
    readonly property var profiles: {
        var avail = root.powerProfileAvailable
        if (!avail || avail.length === 0)
            return allProfiles
        return allProfiles.filter(function(p) { return avail.indexOf(p.key) !== -1 })
    }

    property real reveal: root.powerProfileVisible ? 1 : 0
    Behavior on reveal {
        NumberAnimation {
            duration: root.powerProfileVisible ? 160 : 120
            easing.type: root.powerProfileVisible ? Easing.OutCubic : Easing.InCubic
        }
    }
    visible: reveal > 0.001
    WlrLayershell.keyboardFocus: root.powerProfileVisible ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None

    MouseArea {
        anchors.fill: parent
        onClicked: root.powerProfileVisible = false
    }

    Rectangle {
        id: card
        width: 220
        height: col.implicitHeight + 24
        radius: reveal > 0.001 ? root.panelRadius : 0
        color: "transparent"
        border.color: root.panelBorder
        border.width: 0
        PillShadow { theme: root }
        ConnectedPanelSurface {
            root: profilePanel.root
            ownerActive: profilePanel.root.powerProfileVisible
            targetX: profilePanel.root.powerBarX
            reveal: profilePanel.reveal
        }

        x: Math.round(Math.max(6, Math.min(root.powerBarX - width / 2, parent.width - width - 6)))
        y: root.barPosition === "bottom"
            ? (parent.height - barBottom - gap - height) + 2 * (1 - profilePanel.reveal)
            : (barBottom + gap) - 2 * (1 - profilePanel.reveal)
        opacity: profilePanel.reveal
        focus: root.powerProfileVisible

        Keys.onPressed: function(event) {
            if (event.key === Qt.Key_Escape) {
                root.powerProfileVisible = false;
                event.accepted = true;
            }
        }

        MouseArea { anchors.fill: parent; onClicked: {} }

        Column {
            id: col
            anchors.fill: parent
            anchors.margins: 12
            spacing: 4

            // ── header ──
            Item {
                width: parent.width
                height: 24
                UiText {
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("Power Profile")
                    color: root.ink
                    font.family: root.mono
                    font.pixelSize: 13
                    font.letterSpacing: 2
                    font.weight: Font.Medium
                }
                UiText {
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    text: "\u2715"
                    color: closeMa.containsMouse ? root.seal : root.sumi
                    font.pixelSize: 12
                    Behavior on color { ColorAnimation { duration: 120 } }
                    MouseArea {
                        id: closeMa
                        anchors.fill: parent
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onClicked: root.powerProfileVisible = false
                    }
                }
            }

            Rectangle { width: parent.width; height: 1; color: root.sep }

            // ── profile buttons ──
            Repeater {
                model: profilePanel.profiles

                delegate: Item {
                    required property var modelData
                    required property int index

                    width: parent.width
                    height: 32

                    property bool isActive: root.powerProfileCurrent === modelData.key

                    Rectangle {
                        anchors.fill: parent
                        radius: root.panelButtonRadius
                        color: isActive ? root.fillActive
                               : ma.containsMouse ? root.fillHover : root.fillIdle
                        border.color: (ma.containsMouse || isActive) ? root.seal : root.sep
                        border.width: 1
                        Behavior on color { ColorAnimation { duration: 120 } }
                    }

                    Row {
                        anchors.verticalCenter: parent.verticalCenter
                        anchors.left: parent.left; anchors.leftMargin: 8
                        anchors.right: parent.right; anchors.rightMargin: 8
                        spacing: 8

                        UiText {
                            text: modelData.icon
                            renderType: Text.QtRendering
                            color: (ma.containsMouse || isActive) ? root.seal : root.ink
                            font.family: root.mono
                            font.pixelSize: 14
                            anchors.verticalCenter: parent.verticalCenter
                        }
                        UiText {
                            text: I18n.tr(modelData.label)
                            color: (ma.containsMouse || isActive) ? root.seal : root.ink
                            font.family: root.mono
                            font.pixelSize: 12
                            font.weight: isActive ? Font.Medium : Font.Normal
                            anchors.verticalCenter: parent.verticalCenter
                        }
                    }

                    MouseArea {
                        id: ma
                        anchors.fill: parent
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onClicked: {
                            PowerProfiles.setProfile(modelData.key)
                            root.powerProfileVisible = false
                        }
                    }
                }
            }

            Rectangle { width: parent.width; height: 1; color: root.sep; visible: root.barTemperatureAvailable }
            Item {
                width: parent.width
                height: 26
                visible: root.barTemperatureAvailable
                UiText {
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("Thermal")
                    color: root.sumiHi
                    font.family: root.mono; font.pixelSize: 11; font.letterSpacing: 1
                }
                UiText {
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    text: root.barTemperatureC + "\u00B0C"
                    color: root.barTemperatureC >= 80 ? root.sealRaw : root.seal
                    font.family: root.mono; font.pixelSize: 12; font.weight: Font.Medium
                }
            }
        }
    }
}
