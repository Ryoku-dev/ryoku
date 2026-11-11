import QtQuick
import QtQuick.Shapes
import "look"

// A sparkline: a 2px line filled to half opacity underneath, newest sample
// at the right edge. Values are fractions, so a percentage history fills from
// the floor instead of stretching to its own peak.
Item {
    id: root

    property var values: []
    property color color: GameBarLook.accent

    readonly property var samples: {
        const source = root.values || [];
        const result = [];
        for (let i = 0; i < source.length; i++)
            result.push(Math.max(0, Math.min(1, Number(source[i]) || 0)));
        return result;
    }

    readonly property var points: {
        const list = root.samples;
        if (list.length === 0)
            return [];
        if (list.length === 1)
            return [Qt.point(0, (1 - list[0]) * height), Qt.point(width, (1 - list[0]) * height)];
        return list.map((v, i) => Qt.point(width * i / (list.length - 1), (1 - v) * height));
    }

    readonly property var areaPoints: points.length > 0
        ? [Qt.point(0, height)].concat(points, [Qt.point(width, height)]) : []

    Shape {
        anchors.fill: parent
        visible: root.points.length > 0
        preferredRendererType: Shape.CurveRenderer

        ShapePath {
            startX: 0
            startY: root.height
            strokeColor: "transparent"
            fillColor: Qt.rgba(root.color.r, root.color.g, root.color.b, 0.5)
            PathPolyline { path: root.areaPoints }
        }

        ShapePath {
            startX: root.points.length > 0 ? root.points[0].x : 0
            startY: root.points.length > 0 ? root.points[0].y : 0
            strokeColor: root.color
            strokeWidth: 2
            fillColor: "transparent"
            capStyle: ShapePath.RoundCap
            joinStyle: ShapePath.RoundJoin
            PathPolyline { path: root.points }
        }
    }
}
