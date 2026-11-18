import QtQuick
import "../look"

// The stroke width in use: the figure beside a bar of that weight, in mono in
// Lithic, with round ends in the round designs.
Cell {
    id: root

    property int strokeWidth: 4

    readonly property color inkNow: root.hovered || root.active ? Look.fg : Look.fg2

    Row {
        anchors.centerIn: parent
        spacing: 5 * root.u

        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: root.strokeWidth
            color: root.inkNow
            font.family: Look.lithic ? Look.mono : Look.sans
            font.pixelSize: Math.round((Look.lithic ? 10.5 : 12.5) * root.u)
            font.weight: Look.lithic ? Font.Bold : Font.DemiBold
            font.features: ({ "tnum": 1 })
            renderType: Text.CurveRendering
        }
        Rectangle {
            anchors.verticalCenter: parent.verticalCenter
            width: Math.round(12 * root.u)
            height: Math.max(1, Math.round(Math.min(8, root.strokeWidth * 0.55) * root.u))
            radius: Look.lithic ? 0 : height / 2
            color: root.inkNow
            Behavior on height {
                NumberAnimation { duration: Look.dur(120); easing.type: Easing.OutCubic }
            }
        }
    }
}
