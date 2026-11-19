pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Shapes
import "../kit" as K

// shapes stay put and only their numbers move, so each phase morphs out of the last one
Item {
    id: root

    property string phase: "idle"
    // what's left of the current countdown second, 1 down to 0
    property real countdown: 1

    readonly property real d: Math.round(12 * K.Style.scale)
    readonly property real r: d / 2
    readonly property real thin: Math.max(1.5, 1.6 * K.Style.scale)
    readonly property int morphMs: K.Style.ms(180)

    readonly property bool ringPhase: ["arming", "starting", "recording", "stopping"]
        .indexOf(phase) >= 0
    property real fill: phase === "recording" ? 1 : 0
    property real sweep: phase === "arming" ? 360 * countdown
        : phase === "stopping" ? 100
        : phase === "saved" ? 0 : 360
    property real ring: ringPhase || phase === "saved" ? 1 : 0
    property real track: phase === "arming" ? 1 : 0
    property real bars: phase === "paused" ? 1 : 0
    property real check: phase === "saved" ? 1 : 0
    property real bang: phase === "failed" ? 1 : 0
    property real spin: 0

    implicitWidth: d
    implicitHeight: d
    width: d
    height: d

    Behavior on fill { enabled: !K.Style.still; NumberAnimation { duration: root.morphMs; easing.type: Easing.OutCubic } }
    Behavior on sweep {
        enabled: !K.Style.still && root.phase !== "arming"
        NumberAnimation { duration: root.morphMs; easing.type: Easing.OutCubic }
    }
    Behavior on ring { enabled: !K.Style.still; NumberAnimation { duration: root.morphMs; easing.type: Easing.OutCubic } }
    Behavior on track { enabled: !K.Style.still; NumberAnimation { duration: root.morphMs } }
    Behavior on bars { enabled: !K.Style.still; NumberAnimation { duration: root.morphMs; easing.type: Easing.OutCubic } }
    // slow start so the arc is gone before the check draws
    Behavior on check { enabled: !K.Style.still; NumberAnimation { duration: K.Style.ms(200); easing.type: Easing.InOutCubic } }
    Behavior on bang { enabled: !K.Style.still; NumberAnimation { duration: root.morphMs; easing.type: Easing.OutBack } }

    // only the saving arc turns, and only while it saves
    NumberAnimation {
        target: root
        property: "spin"
        running: root.phase === "stopping" && !K.Style.still && root.visible
        from: 0
        to: 360
        duration: 900
        loops: Animation.Infinite
        onStopped: root.spin = 0
    }

    Shape {
        id: ringShape
        anchors.fill: parent
        preferredRendererType: Shape.CurveRenderer
        opacity: root.ring * (1 - root.bars)
        visible: opacity > 0.01
        transform: Scale {
            origin.x: root.r
            origin.y: root.r
            xScale: 1 - 0.55 * root.bars
        }

        ShapePath {
            strokeColor: K.Style.alpha(K.Style.alert, 0.3 * root.track)
            strokeWidth: root.thin
            fillColor: "transparent"
            PathAngleArc {
                centerX: root.r
                centerY: root.r
                radiusX: root.r - root.thin / 2
                radiusY: root.r - root.thin / 2
                startAngle: 0
                sweepAngle: 360
            }
        }

        // a stroke growing inward until it covers the centre is the ring filling to the dot
        ShapePath {
            id: arc
            readonly property real stroke: root.thin + (root.r - root.thin) * root.fill
            strokeColor: root.sweep > 0.5 ? K.Style.alert : "transparent"
            strokeWidth: arc.stroke
            fillColor: "transparent"
            capStyle: root.sweep >= 359.5 ? ShapePath.FlatCap : ShapePath.RoundCap
            PathAngleArc {
                centerX: root.r
                centerY: root.r
                radiusX: Math.max(0.01, root.r - arc.stroke / 2)
                radiusY: Math.max(0.01, root.r - arc.stroke / 2)
                startAngle: -90 + root.spin
                sweepAngle: root.sweep
            }
        }
    }

    Repeater {
        model: 2

        Rectangle {
            required property int index
            readonly property real barW: Math.max(2, Math.round(root.d * 0.28))
            readonly property real gap: root.d * 0.22 * root.bars
            x: root.r + (index === 0 ? -gap / 2 - barW : gap / 2)
            y: (root.d - height) / 2
            width: barW
            height: root.d * (0.45 + 0.5 * root.bars)
            radius: Math.min(1.5, barW / 2)
            color: K.Style.ink
            opacity: root.bars
            visible: opacity > 0.01
        }
    }

    Shape {
        anchors.fill: parent
        preferredRendererType: Shape.CurveRenderer
        visible: root.check > 0.02

        ShapePath {
            id: tick
            readonly property real ax: root.d * 0.16
            readonly property real ay: root.d * 0.54
            readonly property real bx: root.d * 0.41
            readonly property real by: root.d * 0.78
            readonly property real cx: root.d * 0.86
            readonly property real cy: root.d * 0.26
            readonly property real first: Math.hypot(bx - ax, by - ay)
            readonly property real second: Math.hypot(cx - bx, cy - by)
            readonly property real drawn: root.check * (first + second)
            readonly property real a: Math.min(1, drawn / first)
            readonly property real b: Math.max(0, Math.min(1, (drawn - first) / second))

            strokeColor: K.Style.ink
            strokeWidth: root.thin + 0.2
            fillColor: "transparent"
            capStyle: ShapePath.RoundCap
            joinStyle: ShapePath.RoundJoin
            startX: tick.ax
            startY: tick.ay
            PathLine {
                x: tick.ax + (tick.bx - tick.ax) * tick.a
                y: tick.ay + (tick.by - tick.ay) * tick.a
            }
            PathLine {
                x: tick.bx + (tick.cx - tick.bx) * tick.b
                y: tick.by + (tick.cy - tick.by) * tick.b
            }
        }
    }

    Item {
        anchors.fill: parent
        opacity: root.bang
        visible: opacity > 0.01
        scale: 0.6 + 0.4 * root.bang

        Rectangle {
            x: (root.d - width) / 2
            y: root.d * 0.04
            width: Math.max(2, Math.round(root.thin + 0.5))
            height: root.d * 0.6
            radius: width / 2
            color: K.Style.alert
        }

        Rectangle {
            x: (root.d - width) / 2
            y: root.d - width - root.d * 0.02
            width: Math.max(2, Math.round(root.thin + 0.7))
            height: width
            radius: width / 2
            color: K.Style.alert
        }
    }
}
