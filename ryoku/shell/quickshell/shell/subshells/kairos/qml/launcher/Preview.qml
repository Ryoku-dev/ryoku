// The Kairos launcher's Ryoku Settings preview. Visual only: the variant takes
// no settings yet, so nothing here is editable.

pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import QtQuick.Effects
import Ryoku.Ui
import Ryoku.Ui.Singletons

Item {
    id: root

    property var settings: ({})
    signal editRequested(string key, var value)

    implicitWidth: 720
    implicitHeight: 250

    readonly property var days: [
        { "week": "M", "day": "7" }, { "week": "T", "day": "8" },
        { "week": "WED", "day": "9" }, { "week": "T", "day": "10" },
        { "week": "F", "day": "11" }, { "week": "S", "day": "12" }
    ]
    readonly property var apps: ["Firefox", "Kitty", "Files"]

    Rectangle {
        anchors.fill: parent
        color: Tokens.paper
    }

    Rectangle {
        id: island

        anchors.centerIn: parent
        width: Math.min(parent.width - Tokens.s6, 420)
        height: 214
        radius: 34
        color: Tokens.paperLift
        border.width: Tokens.border
        border.color: Tokens.line

        RectangularShadow {
            anchors.fill: parent
            radius: parent.radius
            blur: 16
            offset: Qt.vector2d(0, 3)
            color: Qt.rgba(0, 0, 0, 0.5)
            z: -1
        }

        Text {
            id: clock
            anchors.horizontalCenter: parent.horizontalCenter
            y: 16
            text: "12:07"
            color: Tokens.ink
            font.family: Tokens.mono
            font.pixelSize: 30
            font.weight: Font.Medium
            font.letterSpacing: 1
        }

        Row {
            id: wheel
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.top: clock.bottom
            anchors.topMargin: 8
            spacing: 6

            Repeater {
                model: root.days

                delegate: Item {
                    id: slot
                    required property int index
                    required property var modelData
                    readonly property bool on: slot.index === 2

                    width: 46
                    height: 66

                    Rectangle {
                        anchors.centerIn: parent
                        width: 40
                        height: 56
                        radius: 11
                        color: slot.on ? Tokens.tint10 : "transparent"
                    }
                    Column {
                        anchors.centerIn: parent
                        spacing: 1
                        Text {
                            anchors.horizontalCenter: parent.horizontalCenter
                            text: slot.modelData.week
                            color: slot.on ? Tokens.ink : Tokens.inkFaint
                            font.family: Tokens.ui
                            font.pixelSize: 10
                            font.weight: slot.on ? Font.DemiBold : Font.Normal
                            font.letterSpacing: slot.on ? 1 : 0
                        }
                        Text {
                            anchors.horizontalCenter: parent.horizontalCenter
                            text: slot.modelData.day
                            color: slot.on ? Tokens.sun : Tokens.inkDim
                            font.family: Tokens.mono
                            font.pixelSize: slot.on ? 21 : 18
                        }
                    }
                }
            }
        }

        Rectangle {
            id: searchRow
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.top: wheel.bottom
            anchors.topMargin: 4
            width: parent.width - 2 * Tokens.s5
            height: 34
            radius: height / 2
            color: Tokens.tint5
            border.width: Tokens.border
            border.color: Tokens.lineSoft

            RowLayout {
                anchors.fill: parent
                anchors.leftMargin: 13
                spacing: 8

                Text {
                    text: "search"
                    color: Tokens.inkDim
                    font.family: "Material Symbols Rounded"
                    font.pixelSize: 16
                    Accessible.ignored: true
                }
                Text {
                    Layout.fillWidth: true
                    text: I18n.tr("Search")
                    color: Tokens.inkFaint
                    font.family: Tokens.ui
                    font.pixelSize: 12
                }
            }
        }

        Column {
            id: appRows
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.top: searchRow.bottom
            anchors.topMargin: 3
            width: parent.width - 2 * Tokens.s5
            spacing: 1

            Repeater {
                model: root.apps

                delegate: Item {
                    id: appRow
                    required property var modelData
                    required property int index
                    readonly property bool on: appRow.index === 0

                    width: appRows.width
                    height: 22

                    Rectangle {
                        anchors.fill: parent
                        radius: 8
                        color: appRow.on ? Tokens.tint10 : "transparent"
                    }
                    Text {
                        anchors.left: parent.left
                        anchors.leftMargin: 10 + (appRow.on ? 5 : 0)
                        anchors.verticalCenter: parent.verticalCenter
                        text: appRow.modelData
                        color: appRow.on ? Tokens.ink : Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: 11
                        font.weight: appRow.on ? Font.DemiBold : Font.Normal
                    }
                    Text {
                        anchors.right: parent.right
                        anchors.rightMargin: 12
                        anchors.verticalCenter: parent.verticalCenter
                        visible: appRow.on
                        text: I18n.tr("Run")
                        color: Tokens.sun
                        font.family: Tokens.ui
                        font.pixelSize: 9
                    }
                }
            }
        }
    }

    Grain {
        anchors.fill: parent
        Accessible.ignored: true
    }
}
