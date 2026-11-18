import QtQuick
import Lithic
import "../look"

// The ink in use: Lithic's 14 px cut chip, a ringed disc in Material and
// Adwaita, Fluent's small rounded square. The outline keeps a dark ink visible
// on a dark plane.
Cell {
    id: root

    property color ink: Look.sel

    readonly property real chip: (Look.lithic ? 14 : Look.material ? 20 : 16) * root.u

    CutRect {
        visible: Look.lithic
        anchors.centerIn: parent
        width: Math.round(root.chip)
        height: width
        design: "lithic"
        cut: 3 * root.u
        fill: root.ink
        strokeColor: Lithic.alpha(Look.fg, 0.35)
        strokeWidth: Math.max(1, Math.round(root.u))
        Behavior on fill {
            ColorAnimation { duration: Look.dur(Look.stateMs) }
        }
    }

    Rectangle {
        visible: !Look.lithic
        anchors.centerIn: parent
        width: Math.round(root.chip)
        height: width
        radius: Look.fluent ? 4 * root.u : width / 2
        color: root.ink
        border.width: Math.max(1, Math.round((Look.material ? 2 : 1) * root.u))
        border.color: Look.material ? Look.plane : Lithic.alpha(Look.fg, 0.3)
        antialiasing: true
        Behavior on color {
            ColorAnimation { duration: Look.dur(Look.stateMs) }
        }

        // Material rings the disc in the outline colour around its surface gap
        Rectangle {
            visible: Look.material
            anchors.centerIn: parent
            width: parent.width + 4 * root.u
            height: width
            radius: width / 2
            color: "transparent"
            border.width: Math.max(1, Math.round(root.u))
            border.color: Look.fg3
            antialiasing: true
        }
    }
}
