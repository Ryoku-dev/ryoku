import QtQuick
import QtQuick.Shapes

Item {
    id: root

    property string edge: "leading"
    property real cutStart: 0
    property real cutEnd: 0
    property real thickness: LithicTokens.size.selBlade
    property color ink: Lithic.onSel

    readonly property bool _bottom: edge === "bottom"
    readonly property real _start: _bounded(cutStart)
    readonly property real _end: _bounded(cutEnd)
    readonly property real _startStep: Math.min(thickness, _start)
    readonly property real _endStep: Math.min(thickness, _end)

    function _bounded(value) {
        return Math.max(0, Math.min(value, Math.min(width, height) / 2));
    }

    Shape {
        anchors.fill: parent
        visible: !root._bottom
        antialiasing: true
        preferredRendererType: Shape.CurveRenderer

        ShapePath {
            fillColor: root.ink
            strokeColor: "transparent"
            startX: 0
            startY: root._start
            PathLine { x: root._startStep; y: root._start - root._startStep }
            PathLine { x: root.thickness; y: Math.max(0, root._start - root.thickness) }
            PathLine { x: root.thickness; y: root.height - Math.max(0, root._end - root.thickness) }
            PathLine { x: root._endStep; y: root.height - root._end + root._endStep }
            PathLine { x: 0; y: root.height - root._end }
            PathLine { x: 0; y: root._start }
        }
    }

    Shape {
        anchors.fill: parent
        visible: root._bottom
        antialiasing: true
        preferredRendererType: Shape.CurveRenderer

        ShapePath {
            fillColor: root.ink
            strokeColor: "transparent"
            startX: root._start
            startY: root.height
            PathLine { x: root._start - root._startStep; y: root.height - root._startStep }
            PathLine { x: Math.max(0, root._start - root.thickness); y: root.height - root.thickness }
            PathLine { x: root.width - Math.max(0, root._end - root.thickness); y: root.height - root.thickness }
            PathLine { x: root.width - root._end + root._endStep; y: root.height - root._endStep }
            PathLine { x: root.width - root._end; y: root.height }
            PathLine { x: root._start; y: root.height }
        }
    }
}
