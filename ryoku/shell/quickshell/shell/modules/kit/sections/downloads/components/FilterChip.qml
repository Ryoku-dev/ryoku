pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import "../../../../ask/look"
import "../../../parts"

// One filter over the queue and how many it holds. The look kit's Chip draws the
// plane, so each design keeps its own, and a count that changes lifts into place.
Chip {
    id: root

    property string label: ""
    property int count: 0
    property real lift: 0

    tier: root.current ? "accent" : "line"
    width: Math.ceil(words.implicitWidth + Math.round(20 * root.s))

    onCountChanged: if (!Look.still) bump.restart()
    NumberAnimation {
        id: bump
        target: root
        property: "lift"
        from: 1
        to: 0
        duration: Look.dur(Look.plateMs)
        easing.type: Easing.BezierSpline
        easing.bezierCurve: Look.arrive
    }

    Row {
        id: words
        anchors.centerIn: parent
        spacing: Math.round(6 * root.s)

        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: root.label
            color: root.ink
            font.family: Look.sans
            font.pixelSize: Math.round(12 * root.s)
            font.weight: Font.Medium
            renderType: Text.NativeRendering
            Behavior on color {
                ColorAnimation { duration: Look.dur(Look.stateMs) }
            }
        }
        Line {
            anchors.verticalCenter: parent.verticalCenter
            anchors.verticalCenterOffset: Math.round(4 * root.s * root.lift)
            s: root.s
            size: 12
            figures: true
            text: String(root.count)
            ink: root.current ? Lithic.alpha(Lithic.onSel, 0.78) : Lithic.fg2
            opacity: 1 - 0.7 * root.lift
        }
    }
}
