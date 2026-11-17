pragma ComponentBehavior: Bound

import QtQuick
import "../../components"
import "look"

// A tile action in the capture grid: a 24px glyph over a 13px label on a raised
// 12px surface, the same fill the overlay's controls use.
Item {
    id: root

    property real s: 1
    property string icon: ""
    property string label: ""
    property bool available: true

    signal clicked()

    implicitWidth: 68 * root.s
    implicitHeight: 62 * root.s

    Rectangle {
        anchors.fill: parent
        radius: GameBarLook.radiusSmall * root.s
        color: pointer.containsMouse && root.available ? GameBarLook.raisedHover : GameBarLook.raised
        opacity: root.available ? 1 : 0.4

        Column {
            anchors.centerIn: parent
            spacing: 6 * root.s

            GlyphIcon {
                anchors.horizontalCenter: parent.horizontalCenter
                width: 24 * root.s
                height: 24 * root.s
                name: root.icon
                color: GameBarLook.fg
                stroke: 1.6
            }

            Text {
                anchors.horizontalCenter: parent.horizontalCenter
                text: root.label
                color: GameBarLook.onLayer
                font.family: GameBarLook.face
                font.pixelSize: GameBarLook.smallie * root.s
            }
        }
    }

    MouseArea {
        id: pointer
        anchors.fill: parent
        hoverEnabled: true
        enabled: root.available
        cursorShape: Qt.PointingHandCursor
        onClicked: root.clicked()
    }
}
