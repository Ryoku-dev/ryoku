pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import "../../components"
import "look"

// A 30px circle holding a 20px glyph, filled with the
// secondary container while its state is on, a layer wash under the pointer.
// No red hover: the overlay's close button is an ordinary button.
Item {
    id: root

    property real s: 1
    property real size: 30
    property string glyph: ""
    property string symbol: ""
    property bool toggled: false

    signal clicked()

    implicitWidth: root.size * root.s
    implicitHeight: root.size * root.s
    scale: pointer.pressed ? 0.92 : 1

    readonly property real iconSize: root.size * 0.7 * root.s

    Behavior on scale {
        NumberAnimation { duration: Motion.dur(110); easing.type: Easing.OutCubic }
    }

    Rectangle {
        anchors.fill: parent
        radius: height / 2
        color: root.toggled ? GameBarLook.toggled
            : (pointer.containsMouse ? GameBarLook.hover : "transparent")

        Behavior on color {
            ColorAnimation { duration: Motion.dur(120) }
        }
    }

    MaterialIcon {
        anchors.centerIn: parent
        width: root.iconSize
        height: root.iconSize
        visible: root.symbol.length > 0
        text: root.symbol
        color: root.toggled ? GameBarLook.toggledInk : GameBarLook.fg
        fill: root.toggled ? 1 : 0
        font.pixelSize: root.iconSize

        Behavior on color {
            ColorAnimation { duration: Motion.dur(120) }
        }
    }

    GlyphIcon {
        anchors.centerIn: parent
        width: root.size * 2 / 3 * root.s
        height: root.size * 2 / 3 * root.s
        visible: root.symbol.length === 0
        name: root.glyph
        color: root.toggled ? GameBarLook.toggledInk : GameBarLook.fg
        stroke: 1.7
    }

    MouseArea {
        id: pointer
        anchors.fill: parent
        hoverEnabled: true
        cursorShape: Qt.PointingHandCursor
        onClicked: root.clicked()
    }
}
