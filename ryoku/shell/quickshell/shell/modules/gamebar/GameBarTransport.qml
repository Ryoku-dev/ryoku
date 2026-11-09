pragma ComponentBehavior: Bound

import QtQuick
import "../../components"
import "look"

// One media transport button: a glyph in a borderless circle that dims when
// that direction is unavailable.
Item {
    id: button

    property real s: 1
    property string glyph: ""
    property bool available: true

    width: 40 * button.s
    height: 40 * button.s
    opacity: button.available ? 1 : 0.35

    signal tapped()

    Rectangle {
        anchors.fill: parent
        radius: height / 2
        color: transportHover.containsMouse ? GameBarLook.hover : "transparent"
    }

    GlyphIcon {
        anchors.centerIn: parent
        width: 18 * button.s
        height: 18 * button.s
        name: button.glyph
        color: GameBarLook.fg
        stroke: 1.7
    }

    MouseArea {
        id: transportHover
        anchors.fill: parent
        hoverEnabled: true
        enabled: button.available
        cursorShape: Qt.PointingHandCursor
        onClicked: button.tapped()
    }
}
