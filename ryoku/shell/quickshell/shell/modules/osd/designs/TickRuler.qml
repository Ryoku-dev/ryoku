pragma ComponentBehavior: Bound

import QtQuick
import Lithic

// The Lithic level as one row of a measuring scale: a tick every 5%, a longer
// one at each quarter, and when the scale runs past 100% the 100 tick stands
// tallest and the ticks beyond it are jasper. The lit edge chases the level, so
// a step lights its ticks in a fast stagger and the leading tick stands proud.
// A fresh ruler sweeps up from zero.
Item {
    id: root

    property real value: 0
    property real maxValue: 1
    property bool muted: false
    property real u: 1

    readonly property int perUnit: 20
    readonly property int count: Math.round(root.perUnit * root.maxValue) + 1
    readonly property bool boosted: root.maxValue > 1.005
    readonly property real tickWidth: Math.max(1, Math.round(root.u))
    readonly property real pitch: (root.width - root.tickWidth) / Math.max(1, root.count - 1)
    readonly property color litInk: root.muted ? Lithic.fg3 : Lithic.sel

    // the level in ticks; the lit edge travels with it, a whole step in 110 ms
    property real head: 0
    Behavior on head {
        NumberAnimation {
            duration: Lithic.duration(110)
            easing.type: Easing.OutCubic
        }
    }
    Component.onCompleted: root.head = Qt.binding(() => root.value * root.perUnit)

    implicitHeight: Math.round(12 * root.u)

    Repeater {
        model: root.count

        delegate: Rectangle {
            id: tick

            required property int index
            readonly property bool unity: root.boosted && tick.index === root.perUnit
            readonly property bool quarter: tick.index % 5 === 0
            readonly property bool beyond: tick.index > root.perUnit
            readonly property real lit: root.head <= 0 ? 0 : Math.max(0, Math.min(1, root.head + 1 - tick.index))
            readonly property real crest: root.head <= 0 ? 0 : Math.max(0, 1 - Math.abs(root.head - tick.index))

            x: Math.round(tick.index * root.pitch)
            y: root.height - height
            width: root.tickWidth
            height: Math.round((tick.unity ? 12 : tick.quarter ? 8 : 4) * root.u + tick.crest * 2 * root.u)
            color: Lithic.mix(tick.beyond ? Lithic.alpha(Lithic.jasper, 0.4) : Lithic.vein,
                tick.beyond && !root.muted ? Lithic.jasper : root.litInk, tick.lit)
        }
    }
}
