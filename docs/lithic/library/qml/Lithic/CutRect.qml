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

    // the same octagon as before, as one point list per ring
    readonly property var _outer: {
        const tl = _bounded(cutTopLeft), tr = _bounded(cutTopRight)
        const br = _bounded(cutBottomRight), bl = _bounded(cutBottomLeft)
        return [Qt.point(tl, 0), Qt.point(width - tr, 0), Qt.point(width, tr), Qt.point(width, height - br),
                Qt.point(width - br, height), Qt.point(bl, height), Qt.point(0, height - bl), Qt.point(0, tl),
                Qt.point(tl, 0)]
    }
    readonly property var _inner: {
        const s = strokeWidth
        const tl = _innerCut(cutTopLeft), tr = _innerCut(cutTopRight)
        const br = _innerCut(cutBottomRight), bl = _innerCut(cutBottomLeft)
        return [Qt.point(s + tl, s), Qt.point(width - s - tr, s), Qt.point(width - s, s + tr),
                Qt.point(width - s, height - s - br), Qt.point(width - s - br, height - s),
                Qt.point(s + bl, height - s), Qt.point(s, height - s - bl), Qt.point(s, s + tl), Qt.point(s + tl, s)]
    }

    Shape {
        anchors.fill: parent
        antialiasing: true
        preferredRendererType: Shape.CurveRenderer

        ShapePath {
            fillColor: root.fill
            strokeColor: "transparent"
            PathPolyline { path: root._outer }
        }

        ShapePath {
            fillColor: root.strokeWidth > 0 ? root.strokeColor : "transparent"
            strokeColor: "transparent"
            fillRule: ShapePath.OddEvenFill
            PathMultiline { paths: [root._outer, root._inner] }
        }
    }

    Item {
        id: contentItem
        anchors.fill: parent
    }
}
