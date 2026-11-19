import QtQuick
import "../../look"
import "../.."

// The shadow's direction: its name over the angle, the dial at the row's end.
// Lithic's dial is the compass with its tick ring.
Field {
    id: dr

    required property var studio

    readonly property real angle: dr.studio ? dr.studio.shadowAngle : 90
    readonly property real dialSize: Look.lithic ? 60 : Look.material ? 52 : 44

    subtitle: Math.round(dr.angle) + "\u00b0"
    minLine: Math.round((dr.dialSize + (Look.lithic ? 8 : 16)) * dr.u)

    Dial {
        u: dr.u
        size: dr.dialSize
        detailed: true
        angle: dr.angle
        onMoved: (a) => dr.studio.shadowAngle = Math.round(a)
    }
}
