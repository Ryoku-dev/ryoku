pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Shapes
import Lithic

// Material 3 Expressive's wavy circular indicator: a ring whose stroke waves
// round a flat track, filled clockwise from the top to `progress`. The wave is
// built once per size; a new progress only shortens the run of points drawn,
// and only when it crosses a point.
Item {
    id: root

    property real progress: 1
    property real thickness: 2.5
    property real amplitude: 1.4
    property int waves: 10
    property color ink: Lithic.sel
    property color trackInk: Lithic.alpha(Lithic.sel, 0.22)

    readonly property int steps: Math.max(48, root.waves * 18)
    readonly property real radius: Math.max(0, Math.min(width, height) / 2 - root.thickness / 2 - root.amplitude)
    readonly property var points: {
        const out = [];
        const cx = width / 2, cy = height / 2;
        for (let i = 0; i <= root.steps; i++) {
            const a = -Math.PI / 2 + i / root.steps * Math.PI * 2;
            const r = root.radius + root.amplitude * Math.sin(a * root.waves);
            out.push(Qt.point(cx + r * Math.cos(a), cy + r * Math.sin(a)));
        }
        return out;
    }
    readonly property int shown: Math.round(Math.max(0, Math.min(1, root.progress)) * root.steps)

    Rectangle {
        anchors.centerIn: parent
        width: root.radius * 2 + root.thickness * 0.6
        height: width
        radius: width / 2
        color: "transparent"
        border.width: root.thickness * 0.6
        border.color: root.trackInk
    }

    Shape {
        anchors.fill: parent
        visible: root.shown > 0
        preferredRendererType: Shape.CurveRenderer

        ShapePath {
            fillColor: "transparent"
            strokeColor: root.ink
            strokeWidth: root.thickness
            capStyle: ShapePath.RoundCap
            joinStyle: ShapePath.RoundJoin
            PathPolyline { path: root.points.slice(0, root.shown + 1) }
        }
    }
}
