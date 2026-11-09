pragma ComponentBehavior: Bound

import QtQuick
import "../../components"
import "look"

// end-4's TitlebarButton: a 30px circle holding a 20px glyph, filled with the
// secondary container while its state is on, a layer wash under the pointer.
// No red hover: the overlay's close button is an ordinary button.
Item {
    id: root

    property real s: 1
    property real size: 30
    property string glyph: ""
    property bool toggled: false

    signal clicked()

    implicitWidth: root.size * root.s
    implicitHeight: root.size * root.s

    Rectangle {
        anchors.fill: parent
        radius: height / 2
        color: root.toggled ? GameBarLook.toggled
            : (pointer.containsMouse ? GameBarLook.hover : "transparent")

        Behavior on color {
            ColorAnimation { duration: 120 }
        }
    }

    GlyphIcon {
        anchors.centerIn: parent
        width: 20 * root.s
        height: 20 * root.s
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
