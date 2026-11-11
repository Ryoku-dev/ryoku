pragma ComponentBehavior: Bound

import QtQuick
import "../../components"
import "look"

// One round voice control: a glyph on a filled circle that switches to the
// toggled tone while the mic or headphones are on.
Item {
    id: toggle

    property real s: 1
    property string glyph: ""
    property bool on: false
    property real diameter: 34 * toggle.s

    signal clicked()

    width: toggle.diameter
    height: toggle.diameter

    Rectangle {
        anchors.fill: parent
        radius: height / 2
        color: toggle.on ? GameBarLook.toggled
            : (togglePointer.containsMouse ? GameBarLook.hover : GameBarLook.raised)

        Behavior on color {
            ColorAnimation { duration: 120 }
        }
    }

    GlyphIcon {
        anchors.centerIn: parent
        width: toggle.diameter * 0.5
        height: toggle.diameter * 0.5
        name: toggle.glyph
        color: toggle.on ? GameBarLook.toggledInk : GameBarLook.fg
        stroke: 1.8
    }

    MouseArea {
        id: togglePointer
        anchors.fill: parent
        hoverEnabled: true
        cursorShape: Qt.PointingHandCursor
        onClicked: toggle.clicked()
    }
}
