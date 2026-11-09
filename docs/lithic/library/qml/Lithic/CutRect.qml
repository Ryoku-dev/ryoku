import QtQuick
import QtQuick.Shapes

Item {
    id: root

    property color fill: "transparent"
    property color strokeColor: "transparent"
    property real strokeWidth: 0
    property real cut: 0
    property real cutTopLeft: cut
    property real cutTopRight: cut
    property real cutBottomRight: cut
    property real cutBottomLeft: cut
    property alias cutNW: root.cutTopLeft
    property alias cutNE: root.cutTopRight
    property alias cutSE: root.cutBottomRight
    property alias cutSW: root.cutBottomLeft
    default property alias content: contentItem.data

    function _bounded(value) {
        return Math.max(0, Math.min(value, Math.min(width, height) / 2));
    }

    function _innerCut(value) {
        return Math.max(0, _bounded(value) - strokeWidth * (2 - Math.SQRT2));
    }

    Shape {
        anchors.fill: parent
        antialiasing: true
        preferredRendererType: Shape.CurveRenderer

        ShapePath {
            fillColor: root.fill
            strokeColor: "transparent"
            startX: root._bounded(root.cutTopLeft)
            startY: 0
            PathLine { x: root.width - root._bounded(root.cutTopRight); y: 0 }
            PathLine { x: root.width; y: root._bounded(root.cutTopRight) }
            PathLine { x: root.width; y: root.height - root._bounded(root.cutBottomRight) }
            PathLine { x: root.width - root._bounded(root.cutBottomRight); y: root.height }
            PathLine { x: root._bounded(root.cutBottomLeft); y: root.height }
            PathLine { x: 0; y: root.height - root._bounded(root.cutBottomLeft) }
            PathLine { x: 0; y: root._bounded(root.cutTopLeft) }
            PathLine { x: root._bounded(root.cutTopLeft); y: 0 }
        }

        ShapePath {
            fillColor: root.strokeWidth > 0 ? root.strokeColor : "transparent"
            strokeColor: "transparent"
            fillRule: ShapePath.OddEvenFill
            startX: root._bounded(root.cutTopLeft)
            startY: 0
            PathLine { x: root.width - root._bounded(root.cutTopRight); y: 0 }
            PathLine { x: root.width; y: root._bounded(root.cutTopRight) }
            PathLine { x: root.width; y: root.height - root._bounded(root.cutBottomRight) }
            PathLine { x: root.width - root._bounded(root.cutBottomRight); y: root.height }
            PathLine { x: root._bounded(root.cutBottomLeft); y: root.height }
            PathLine { x: 0; y: root.height - root._bounded(root.cutBottomLeft) }
            PathLine { x: 0; y: root._bounded(root.cutTopLeft) }
            PathLine { x: root._bounded(root.cutTopLeft); y: 0 }

            PathMove { x: root.strokeWidth + root._innerCut(root.cutTopLeft); y: root.strokeWidth }
            PathLine { x: root.width - root.strokeWidth - root._innerCut(root.cutTopRight); y: root.strokeWidth }
            PathLine { x: root.width - root.strokeWidth; y: root.strokeWidth + root._innerCut(root.cutTopRight) }
            PathLine { x: root.width - root.strokeWidth; y: root.height - root.strokeWidth - root._innerCut(root.cutBottomRight) }
            PathLine { x: root.width - root.strokeWidth - root._innerCut(root.cutBottomRight); y: root.height - root.strokeWidth }
            PathLine { x: root.strokeWidth + root._innerCut(root.cutBottomLeft); y: root.height - root.strokeWidth }
            PathLine { x: root.strokeWidth; y: root.height - root.strokeWidth - root._innerCut(root.cutBottomLeft) }
            PathLine { x: root.strokeWidth; y: root.strokeWidth + root._innerCut(root.cutTopLeft) }
            PathLine { x: root.strokeWidth + root._innerCut(root.cutTopLeft); y: root.strokeWidth }
        }
    }

    Item {
        id: contentItem
        anchors.fill: parent
    }
}
