import QtQuick
import QtQuick.Layouts
import Quickshell
import "Singletons"
import Ryoku.Ui.Singletons

// The region-selector front: a floating bar that picks what a drawn region does
// (Shot, Edit, OCR, Search, Record), exposes recording options, and keeps the
// delay, colour picker, monitor capture, screenshots folder, and close controls
// beside them. The shell reads activeAction and dispatches the chosen operation.
Item {
    id: bar
    implicitWidth: glass.implicitWidth
    implicitHeight: glass.implicitHeight

    property var actions: []
    property string activeAction: "shot"
    property bool audioOn: false
    property bool micOn: false
    property bool keypressesOn: false
    property bool webcamOn: false
    property int delaySeconds: 0
    property bool hintsVisible: true

    Timer {
        id: hintTimer
        interval: 1400
        onTriggered: bar.hintsVisible = false
    }

    onVisibleChanged: {
        if (visible) {
            hintsVisible = true;
            hintTimer.restart();
        } else {
            hintTimer.stop();
            hintsVisible = false;
        }
    }

    Component.onCompleted: {
        if (visible) {
            hintsVisible = true;
            hintTimer.restart();
        }
    }

    signal actionPicked(string id)
    signal colorPickRequested()
    signal fullscreenRequested()
    signal audioToggled()
    signal delayCycled()
    signal micToggled()
    signal keypressesToggled()
    signal webcamToggled()
    signal folderRequested()
    signal closeRequested()

    Rectangle {
        id: glass
        anchors.fill: parent
        radius: Theme.radius
        color: Theme.panel
        border.color: Theme.hair
        border.width: 1
        implicitWidth: row.implicitWidth + 16
        implicitHeight: row.implicitHeight + 12

        RowLayout {
            id: row
            anchors.centerIn: parent
            spacing: 6

            // Action selector: one segment per capture action. The chosen segment
            // wears the accent so the next drawn region reads its intent at a glance.
            Rectangle {
                Layout.alignment: Qt.AlignVCenter
                implicitWidth: actionRow.implicitWidth + 6
                implicitHeight: 40
                radius: height / 2
                color: Theme.field

                Row {
                    id: actionRow
                    anchors.centerIn: parent
                    spacing: 2

                    Repeater {
                        model: bar.actions
                        Rectangle {
                            id: tab
                            required property var modelData
                            readonly property bool current: bar.activeAction === modelData.id
                            readonly property bool showKey: bar.hintsVisible && !!(tab.modelData && tab.modelData.key)
                            width: tabContent.implicitWidth + 22
                            height: 34
                            radius: height / 2
                            color: current ? Theme.accent
                                : (tabMa.containsMouse ? Theme.hover : "transparent")

                            Row {
                                id: tabContent
                                anchors.centerIn: parent
                                spacing: 6

                                Icon {
                                    anchors.verticalCenter: parent.verticalCenter
                                    name: tab.modelData.icon
                                    size: 17
                                    tint: tab.current ? Theme.accentInk
                                        : (tabMa.containsMouse ? Theme.ink : Theme.inkDim)
                                }
                                Row {
                                    anchors.verticalCenter: parent.verticalCenter
                                    spacing: tab.showKey ? 5 : 0
                                    Behavior on spacing {
                                        NumberAnimation { duration: 250; easing.type: Easing.OutCubic }
                                    }

                                    Text {
                                        anchors.verticalCenter: parent.verticalCenter
                                        text: tab.modelData.label
                                        font.family: Theme.ui
                                        font.pixelSize: 13
                                        font.weight: tab.current ? Font.DemiBold : Font.Normal
                                        color: tab.current ? Theme.accentInk
                                            : (tabMa.containsMouse ? Theme.ink : Theme.inkDim)
                                    }
                                    Item {
                                        id: keySlot
                                        anchors.verticalCenter: parent.verticalCenter
                                        width: tab.showKey ? keyText.implicitWidth : 0
                                        height: keyText.implicitHeight
                                        clip: true
                                        visible: tab.showKey || width > 0

                                        Behavior on width {
                                            NumberAnimation { duration: 250; easing.type: Easing.OutCubic }
                                        }

                                        Text {
                                            id: keyText
                                            anchors.centerIn: parent
                                            text: (tab.modelData && tab.modelData.key ? tab.modelData.key : "").toUpperCase()
                                            font.family: Theme.mono
                                            font.pixelSize: 10
                                            font.weight: Font.Medium
                                            color: tab.current ? Theme.accentInk
                                                : (tabMa.containsMouse ? Theme.inkDim : Theme.inkFaint)
                                            opacity: tab.showKey ? (tab.current ? 0.8 : 0.55) : 0

                                            Behavior on opacity {
                                                NumberAnimation { duration: 200; easing.type: Easing.OutCubic }
                                            }
                                        }
                                    }
                                }
                            }

                            MouseArea {
                                id: tabMa
                                anchors.fill: parent
                                hoverEnabled: true
                                cursorShape: Qt.PointingHandCursor
                                onClicked: bar.actionPicked(tab.modelData.id)
                            }
                        }
                    }
                }
            }

            Rectangle { Layout.preferredWidth: 1; Layout.preferredHeight: 22; color: Theme.hair }

            IconButton {
                Layout.alignment: Qt.AlignVCenter
                icon: "sound"
                visible: bar.activeAction === "record"
                active: bar.audioOn
                tooltip: I18n.tr("Record system audio")
                onClicked: bar.audioToggled()
            }
            IconButton {
                Layout.alignment: Qt.AlignVCenter
                icon: "mic"
                visible: bar.activeAction === "record"
                active: bar.micOn
                tooltip: I18n.tr("Record microphone audio")
                onClicked: bar.micToggled()
            }
            IconButton {
                Layout.alignment: Qt.AlignVCenter
                icon: "keyboard"
                visible: bar.activeAction === "record"
                active: bar.keypressesOn
                tooltip: bar.keypressesOn ? I18n.tr("Hide key presses") : I18n.tr("Show key presses")
                onClicked: bar.keypressesToggled()
            }
            IconButton {
                Layout.alignment: Qt.AlignVCenter
                icon: "webcam"
                visible: bar.activeAction === "record"
                active: bar.webcamOn
                tooltip: bar.webcamOn ? I18n.tr("Hide webcam") : I18n.tr("Show webcam")
                onClicked: bar.webcamToggled()
            }

            Rectangle {
                visible: bar.activeAction === "record"
                Layout.preferredWidth: 1
                Layout.preferredHeight: 22
                color: Theme.hair
            }

            Item {
                Layout.alignment: Qt.AlignVCenter
                Layout.preferredWidth: 32
                Layout.preferredHeight: 32

                IconButton {
                    anchors.fill: parent
                    icon: "timer"
                    active: bar.delaySeconds > 0
                    tooltip: bar.delaySeconds === 0
                        ? I18n.tr("No delay")
                        : I18n.tr("%1-second delay").arg(bar.delaySeconds)
                    onClicked: bar.delayCycled()
                }

                Rectangle {
                    visible: bar.delaySeconds > 0
                    anchors.right: parent.right
                    anchors.top: parent.top
                    anchors.rightMargin: -2
                    anchors.topMargin: -2
                    width: 14
                    height: 14
                    radius: 7
                    color: Theme.accent
                    z: 2

                    Text {
                        anchors.centerIn: parent
                        text: bar.delaySeconds
                        color: Theme.ink
                        font.family: Theme.mono
                        font.pixelSize: 10
                    }
                }
            }

            IconButton {
                Layout.alignment: Qt.AlignVCenter
                icon: "eyedropper"
                tooltip: I18n.tr("Pick a colour") + "  (C)"
                onClicked: bar.colorPickRequested()
            }
            IconButton {
                Layout.alignment: Qt.AlignVCenter
                icon: "fullscreen"
                tooltip: I18n.tr("Capture this monitor") + "  (Enter)"
                onClicked: bar.fullscreenRequested()
            }
            IconButton {
                Layout.alignment: Qt.AlignVCenter
                icon: "folder"
                tooltip: I18n.tr("Open screenshots folder")
                onClicked: bar.folderRequested()
            }

            Rectangle { Layout.preferredWidth: 1; Layout.preferredHeight: 22; color: Theme.hair }

            Rectangle {
                id: closeBtn
                Layout.alignment: Qt.AlignVCenter
                Layout.preferredWidth: 32
                Layout.preferredHeight: 32
                radius: 7
                color: closeMa.containsMouse ? Qt.rgba(Theme.danger.r, Theme.danger.g, Theme.danger.b, 0.16) : "transparent"
                Icon {
                    anchors.centerIn: parent
                    name: "cancel"
                    size: 18
                    tint: closeMa.containsMouse ? Theme.danger : Theme.inkDim
                }
                MouseArea {
                    id: closeMa
                    anchors.fill: parent
                    hoverEnabled: true
                    onClicked: bar.closeRequested()
                }
            }
        }
    }
}
