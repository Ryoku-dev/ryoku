pragma ComponentBehavior: Bound

import QtQuick
import "../../components"
import "look"

// end-4's SecondaryTabBar: a 42px row of icon and label tabs over a 1px bottom
// border, with a 3px primary indicator tucked under the active one.
Item {
    id: root

    property real s: 1
    property var model: []
    property int currentIndex: 0

    signal picked(int index)

    implicitHeight: 42 * root.s

    Row {
        id: tabs
        anchors.fill: parent
        anchors.bottomMargin: 1 * root.s

        Repeater {
            model: root.model

            delegate: Item {
                id: tab
                required property var modelData
                required property int index

                readonly property bool current: root.currentIndex === index
                readonly property real cellWidth: root.width / Math.max(1, root.model.length)

                width: cellWidth
                height: parent.height

                Rectangle {
                    anchors.fill: parent
                    anchors.margins: 3 * root.s
                    radius: GameBarLook.radiusNormal * root.s
                    color: pointer.containsMouse && !tab.current ? GameBarLook.hover : "transparent"
                }

                Row {
                    anchors.centerIn: parent
                    spacing: 5 * root.s

                    GlyphIcon {
                        anchors.verticalCenter: parent.verticalCenter
                        width: 22 * root.s
                        height: 22 * root.s
                        name: tab.modelData.icon
                        color: tab.current ? GameBarLook.accent : GameBarLook.onLayer
                        stroke: 1.7
                    }

                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: tab.modelData.label
                        color: tab.current ? GameBarLook.accent : GameBarLook.onLayer
                        font.family: GameBarLook.face
                        font.pixelSize: GameBarLook.small * root.s
                    }
                }

                MouseArea {
                    id: pointer
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onClicked: root.picked(tab.index)
                }
            }
        }
    }

    Rectangle {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        height: 1
        color: GameBarLook.border
    }

    Rectangle {
        readonly property real baseWidth: root.width / Math.max(1, root.model.length)
        readonly property real padding: 8 * root.s
        x: root.currentIndex * baseWidth + padding
        width: Math.max(0, baseWidth - padding * 2)
        anchors.bottom: parent.bottom
        height: 3 * root.s
        topLeftRadius: height
        topRightRadius: height
        color: GameBarLook.accent
    }
}
