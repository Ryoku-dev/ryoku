import QtQuick
import Lithic
import "look"

// A literal colour to pick, in the design's shape: Lithic's cut chip, a disc in
// Material and Adwaita, Fluent's small rounded square. The chosen one is ringed
// in the ink colour, with a gap of the plane between ring and chip.
Item {
    id: root

    property real u: 1
    property color ink: "white"
    property bool selected: false
    property real size: 18

    signal tapped()

    implicitWidth: Math.round(root.size * root.u)
    implicitHeight: implicitWidth

    readonly property real ring: Math.max(1, Math.round((Look.material ? 2 : 1.5) * root.u))

    Item {
        id: face
        anchors.centerIn: parent
        width: parent.width
        height: parent.height
        scale: hover.hovered && !root.selected ? 1.1 : 1
        Behavior on scale {
            NumberAnimation { duration: Look.dur(120); easing.type: Easing.OutCubic }
        }

        CutRect {
            visible: Look.lithic
            anchors.fill: parent
            anchors.margins: root.selected ? root.ring * 2 : 0
            design: "lithic"
            cut: 3 * root.u
            fill: root.ink
            strokeColor: Lithic.alpha(Look.fg, 0.28)
            strokeWidth: Math.max(1, Math.round(root.u))
        }
        CutRect {
            visible: Look.lithic && root.selected
            anchors.fill: parent
            design: "lithic"
            cut: 4 * root.u
            strokeColor: Look.fg
            strokeWidth: root.ring
        }

        Rectangle {
            visible: !Look.lithic
            anchors.fill: parent
            anchors.margins: root.selected ? root.ring * 2 : 0
            radius: Look.fluent ? 4 * root.u : width / 2
            color: root.ink
            border.width: Math.max(1, Math.round(root.u))
            border.color: Lithic.alpha(Look.fg, 0.22)
            antialiasing: true
            Behavior on anchors.margins {
                NumberAnimation { duration: Look.dur(140); easing.type: Easing.OutCubic }
            }
        }
        Rectangle {
            visible: !Look.lithic && root.selected
            anchors.fill: parent
            radius: Look.fluent ? 6 * root.u : width / 2
            color: "transparent"
            border.width: root.ring
            border.color: Look.fg
            antialiasing: true
        }
    }

    HoverHandler {
        id: hover
        cursorShape: Qt.PointingHandCursor
    }
    TapHandler {
        onTapped: root.tapped()
    }
}
