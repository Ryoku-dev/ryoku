pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import shell.services
import "../../components"
import "look"

// end-4's OverlayTaskbar: widget buttons, a separator, the clock and the
// battery. A button shows its widget, a second press hides it.
Rectangle {
    id: root

    property real s: 1
    property var cards: []
    property var activeIds: []
    property bool snapping: true

    signal picked(string id)
    signal snapToggled()

    readonly property real padding: 8 * root.s
    readonly property real button: 32 * root.s

    implicitWidth: row.implicitWidth + root.padding * 2
    implicitHeight: root.button + root.padding * 2
    radius: GameBarLook.radiusWindow * root.s
    color: GameBarLook.bar
    border.width: 1
    border.color: GameBarLook.border

    property string clock: ""

    function tick() {
        const d = new Date();
        root.clock = Qt.formatTime(d, "HH:mm");
    }

    Component.onCompleted: root.tick()

    Timer {
        interval: 10000
        running: root.visible && root.enabled
        repeat: true
        onTriggered: root.tick()
    }

    onVisibleChanged: if (root.visible) root.tick()
    onEnabledChanged: if (root.enabled) root.tick()

    Row {
        id: row
        anchors.fill: parent
        anchors.margins: root.padding
        spacing: 6 * root.s

        Row {
            spacing: 4 * root.s
            anchors.verticalCenter: parent.verticalCenter

            Repeater {
                model: root.cards

                delegate: Item {
                    id: button
                    required property var modelData

                    readonly property bool on: root.activeIds.indexOf(modelData.id) !== -1

                    width: root.button
                    height: root.button

                    Rectangle {
                        anchors.fill: parent
                        radius: Math.max(0, (GameBarLook.radiusNormal - 8) * root.s)
                        color: button.on ? GameBarLook.toggled
                            : (pointer.containsMouse ? GameBarLook.hover : "transparent")

                        Behavior on color {
                            ColorAnimation { duration: 120 }
                        }
                    }

                    Item {
                        anchors.centerIn: parent
                        width: root.button
                        height: root.button

                        MaterialIcon {
                            anchors.fill: parent
                            visible: !button.modelData.glyph
                            text: button.modelData.symbol
                            color: GameBarLook.onLayer
                            font.pixelSize: 21 * root.s
                        }

                        GlyphIcon {
                            anchors.centerIn: parent
                            width: 20 * root.s
                            height: 20 * root.s
                            visible: !!button.modelData.glyph
                            name: button.modelData.glyph || ""
                            color: GameBarLook.onLayer
                            stroke: 1.6
                        }
                    }

                    MouseArea {
                        id: pointer
                        anchors.fill: parent
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onClicked: root.picked(button.modelData.id)
                    }
                }
            }
        }

        Rectangle {
            width: 1
            height: root.button - 20 * root.s
            anchors.verticalCenter: parent.verticalCenter
            color: GameBarLook.border
        }

        Item {
            id: snapButton
            width: root.button
            height: root.button
            anchors.verticalCenter: parent.verticalCenter

            Rectangle {
                anchors.fill: parent
                radius: GameBarLook.radiusNormal * root.s
                color: root.snapping ? GameBarLook.toggled
                    : (snapPointer.containsMouse ? GameBarLook.hover : "transparent")

                Behavior on color {
                    ColorAnimation { duration: 120 }
                }
            }

            MaterialIcon {
                anchors.centerIn: parent
                width: root.button
                height: root.button
                text: "grid_view"
                color: GameBarLook.onLayer
                font.pixelSize: 21 * root.s
            }

            MouseArea {
                id: snapPointer
                anchors.fill: parent
                hoverEnabled: true
                cursorShape: Qt.PointingHandCursor
                onClicked: root.snapToggled()
            }
        }

        Text {
            anchors.verticalCenter: parent.verticalCenter
            anchors.leftMargin: 8 * root.s
            anchors.rightMargin: 6 * root.s
            text: root.clock
            color: GameBarLook.fg
            font.family: GameBarLook.face
            font.pixelSize: GameBarLook.huge * root.s
        }

        Rectangle {
            visible: Battery.present
            width: 1
            height: root.button - 20 * root.s
            anchors.verticalCenter: parent.verticalCenter
            color: GameBarLook.border
        }

        Row {
            visible: Battery.present
            anchors.verticalCenter: parent.verticalCenter
            spacing: 2 * root.s

            GlyphIcon {
                anchors.verticalCenter: parent.verticalCenter
                width: 24 * root.s
                height: 24 * root.s
                name: "battery"
                color: Battery.low && !Battery.charging ? GameBarLook.danger : GameBarLook.fg
                stroke: 1.7
            }

            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: Battery.pct + "%"
                color: Battery.low && !Battery.charging ? GameBarLook.danger : GameBarLook.fg
                font.family: GameBarLook.face
                font.pixelSize: 18 * root.s
            }
        }
    }
}
