pragma ComponentBehavior: Bound

import QtQuick
import "look"

// end-4's StyledText pair: a subtext label on the left, the value in the body
// size on the right.
Item {
    id: root

    property real s: 1
    property string label: ""
    property string value: ""

    implicitHeight: 24 * root.s

    Text {
        anchors.left: parent.left
        anchors.verticalCenter: parent.verticalCenter
        text: root.label
        color: GameBarLook.subtext
        font.family: GameBarLook.face
        font.pixelSize: GameBarLook.smallie * root.s
        elide: Text.ElideRight
        width: Math.min(implicitWidth, root.width * 0.58)
    }

    Text {
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        text: root.value
        color: GameBarLook.fg
        font.family: GameBarLook.face
        font.pixelSize: GameBarLook.small * root.s
        elide: Text.ElideRight
        width: Math.min(implicitWidth, root.width * 0.55)
        horizontalAlignment: Text.AlignRight
    }
}
