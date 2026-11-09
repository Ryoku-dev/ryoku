pragma ComponentBehavior: Bound

import QtQuick
import "look"

// end-4's WButton at overlay size: a 12px radius, 40px tall, one raised fill,
// a layer wash under the pointer. The primary and danger forms take the accent
// and error roles whole.
Item {
    id: root

    property real s: 1
    property string label: ""
    property bool primary: false
    property bool danger: false
    property bool quiet: false
    property bool compact: false

    signal clicked()

    implicitWidth: text.implicitWidth + (root.compact ? 20 : 30) * root.s
    implicitHeight: (root.compact ? 32 : 40) * root.s

    Rectangle {
        anchors.fill: parent
        radius: GameBarLook.radiusSmall * root.s
        color: {
            if (root.primary)
                return pointer.containsMouse ? Qt.lighter(GameBarLook.accent, 1.1) : GameBarLook.accent;
            if (root.danger)
                return pointer.containsMouse ? Qt.lighter(GameBarLook.danger, 1.1) : GameBarLook.danger;
            if (root.quiet)
                return pointer.containsMouse ? GameBarLook.hover : "transparent";
            return pointer.containsMouse ? GameBarLook.raisedHover : GameBarLook.raised;
        }

        Behavior on color {
            ColorAnimation { duration: 120 }
        }
    }

    Text {
        id: text
        anchors.centerIn: parent
        text: root.label
        color: root.primary ? GameBarLook.accentInk
            : root.danger ? GameBarLook.onDanger
            : GameBarLook.fg
        font.family: GameBarLook.face
        font.pixelSize: (root.compact ? GameBarLook.smallie : GameBarLook.small) * root.s
    }

    MouseArea {
        id: pointer
        anchors.fill: parent
        hoverEnabled: true
        cursorShape: Qt.PointingHandCursor
        onClicked: root.clicked()
    }
}
