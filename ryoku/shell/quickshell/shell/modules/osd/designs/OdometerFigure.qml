pragma ComponentBehavior: Bound

import QtQuick
import Lithic

// A zero-padded figure whose digits roll like an odometer: each wheel turns
// through the steps between the old value and the new one (at most one
// revolution, so a long jump never blurs), and the higher places land a beat
// later. A fresh figure rolls in. Leading zeros keep the quiet ink, so the
// significant digits read first.
Row {
    id: root

    property int value: 0
    property int digits: 3
    property real size: 32
    property color ink: Lithic.fg
    property color quietInk: Lithic.fg3
    property int duration: 340

    readonly property real lineHeight: Math.ceil(root.size * 1.2)

    TextMetrics {
        id: digitMetrics
        font.family: Lithic.font.mono
        font.pixelSize: root.size
        text: "0"
    }

    Repeater {
        model: root.digits

        delegate: Item {
            id: wheel

            required property int index
            readonly property int place: root.digits - 1 - wheel.index
            readonly property real unit: Math.pow(10, wheel.place)
            // a wheel counts every turn it has made, so 99 to 100 rolls forward
            readonly property int target: Math.floor(Math.max(0, root.value) / wheel.unit)
            readonly property bool lead: wheel.place > 0 && root.value < wheel.unit
            property real turn: 0
            readonly property int low: Math.floor(wheel.turn)
            readonly property real frac: wheel.turn - wheel.low

            width: Math.ceil(digitMetrics.advanceWidth)
            height: root.lineHeight
            clip: true
            Component.onCompleted: wheel.roll(Math.max(0, wheel.target - 10))
            onTargetChanged: wheel.roll(wheel.turn)

            function roll(from) {
                spin.stop();
                const gap = wheel.target - from;
                spin.from = Math.abs(gap) > 10 ? wheel.target - (gap > 0 ? 10 : -10) : from;
                spin.to = wheel.target;
                spin.start();
            }

            NumberAnimation {
                id: spin
                target: wheel
                property: "turn"
                duration: Lithic.duration(root.duration + wheel.place * 80)
                easing.type: Easing.BezierSpline
                easing.bezierCurve: Lithic.curve
            }

            Text {
                y: -wheel.frac * wheel.height
                opacity: Math.max(0, 1 - wheel.frac * 1.7)
                width: wheel.width
                height: wheel.height
                text: String(((wheel.low % 10) + 10) % 10)
                color: wheel.lead ? root.quietInk : root.ink
                font: digitMetrics.font
                horizontalAlignment: Text.AlignHCenter
                verticalAlignment: Text.AlignVCenter
                renderType: Text.CurveRendering
            }
            Text {
                visible: wheel.frac > 0.001
                y: (1 - wheel.frac) * wheel.height
                opacity: Math.max(0, wheel.frac * 1.7 - 0.7)
                width: wheel.width
                height: wheel.height
                text: String(((wheel.low + 1) % 10 + 10) % 10)
                color: wheel.lead ? root.quietInk : root.ink
                font: digitMetrics.font
                horizontalAlignment: Text.AlignHCenter
                verticalAlignment: Text.AlignVCenter
                renderType: Text.CurveRendering
            }
        }
    }
}
