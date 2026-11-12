// visual only: the variant takes no settings yet

pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Ryoku.Ui
import Ryoku.Ui.Singletons

Item {
    id: root

    property var settings: ({})
    signal editRequested(string key, var value)

    implicitWidth: 720
    implicitHeight: 250
    clip: true

    readonly property color accentContainer: Qt.rgba(Tokens.sun.r, Tokens.sun.g, Tokens.sun.b, 0.28)
    readonly property var suggested: [
        { "glyph": "public", "name": "Firefox" }, { "glyph": "terminal", "name": "Kitty" },
        { "glyph": "folder", "name": "Files" }, { "glyph": "music_note", "name": "Ryotunes" },
        { "glyph": "settings", "name": "Settings" }
    ]
    readonly property var grid: ["calculate", "calendar_month", "photo_camera", "mail", "code"]

    component AppGlyph: Rectangle {
        property string glyph: ""
        width: 44
        height: 44
        radius: 22
        color: Tokens.tint10

        Text {
            anchors.centerIn: parent
            text: parent.glyph
            color: Tokens.ink
            font.family: "Material Symbols Rounded"
            font.pixelSize: 22
            Accessible.ignored: true
        }
    }

    Rectangle {
        anchors.fill: parent
        color: Tokens.paper
    }

    Rectangle {
        id: drawer

        anchors.horizontalCenter: parent.horizontalCenter
        y: 8
        width: 440
        height: 290
        radius: 32
        color: Tokens.paperLift
        border.width: Tokens.border
        border.color: Tokens.lineSoft

        RectangularShadow {
            anchors.fill: parent
            radius: parent.radius
            blur: 16
            offset: Qt.vector2d(0, 3)
            color: Qt.rgba(0, 0, 0, 0.5)
            z: -1
        }

        Text {
            anchors.horizontalCenter: parent.horizontalCenter
            y: 8
            text: "12:07"
            color: Tokens.ink
            font.family: Tokens.mono
            font.pixelSize: 26
            font.weight: Font.Medium
            font.letterSpacing: 1
        }

        Rectangle {
            id: search
            anchors.horizontalCenter: parent.horizontalCenter
            y: 48
            width: parent.width - 40
            height: 40
            radius: height / 2
            color: Tokens.tint10

            Text {
                id: lens
                anchors { left: parent.left; leftMargin: 14; verticalCenter: parent.verticalCenter }
                text: "search"
                color: Tokens.inkDim
                font.family: "Material Symbols Rounded"
                font.pixelSize: 18
                Accessible.ignored: true
            }
            Text {
                anchors { left: lens.right; leftMargin: 10; verticalCenter: parent.verticalCenter }
                text: I18n.tr("Search apps")
                color: Tokens.inkFaint
                font.family: Tokens.ui
                font.pixelSize: 13
            }
            Text {
                anchors { right: parent.right; rightMargin: 14; verticalCenter: parent.verticalCenter }
                text: "visibility_off"
                color: Tokens.inkDim
                font.family: "Material Symbols Rounded"
                font.pixelSize: 18
                Accessible.ignored: true
            }
        }

        Text {
            x: 30
            y: 98
            text: I18n.tr("Suggested")
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: 11
            font.weight: Font.Medium
        }

        Row {
            id: suggestedRow
            anchors.horizontalCenter: parent.horizontalCenter
            y: 116

            Repeater {
                model: root.suggested

                delegate: Item {
                    id: cell
                    required property var modelData
                    required property int index
                    readonly property bool on: cell.index === 0

                    width: 80
                    height: 78

                    Rectangle {
                        anchors.fill: parent
                        anchors.margins: 2
                        radius: 20
                        color: cell.on ? root.accentContainer : "transparent"
                    }
                    AppGlyph {
                        anchors.horizontalCenter: parent.horizontalCenter
                        y: 8
                        glyph: cell.modelData.glyph
                    }
                    Text {
                        anchors.horizontalCenter: parent.horizontalCenter
                        y: 58
                        text: cell.modelData.name
                        color: Tokens.ink
                        font.family: Tokens.ui
                        font.pixelSize: 11
                        font.weight: cell.on ? Font.Medium : Font.Normal
                    }
                }
            }
        }

        Rectangle {
            x: 30
            y: suggestedRow.y + suggestedRow.height + 6
            width: parent.width - 60
            height: 1
            color: Tokens.lineSoft
        }

        Row {
            anchors.horizontalCenter: parent.horizontalCenter
            y: suggestedRow.y + suggestedRow.height + 15

            Repeater {
                model: root.grid

                delegate: Item {
                    id: gridCell
                    required property string modelData
                    width: 80
                    height: 78

                    AppGlyph {
                        anchors.horizontalCenter: parent.horizontalCenter
                        y: 8
                        glyph: gridCell.modelData
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
