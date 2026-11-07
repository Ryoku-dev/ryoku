import QtQuick
import QtQuick.Shapes

Item {
    id: root

    property var values: []
    property color color: Style.ink
    property bool fill: false
    property real lineWidth: 1.5

    readonly property var samples: {
        const source = values || [];
        const result = [];
        for (let index = 0; index < source.length; ++index)
            result.push(Math.max(0, Math.min(1, Number(source[index]) || 0)));
        return result;
    }
    readonly property var points: {
        if (samples.length === 0)
            return [];
        if (samples.length === 1) {
            const y = (1 - samples[0]) * height;
            return [Qt.point(0, y), Qt.point(width, y)];
        }
        return samples.map((sample, index) => Qt.point(
            width * index / (samples.length - 1), (1 - sample) * height));
    }
    readonly property var areaPoints: points.length > 0
        ? [Qt.point(0, height)].concat(points, [Qt.point(width, height)]) : []

    implicitWidth: 120
    implicitHeight: 32

    Shape {
        anchors.fill: parent
        visible: root.points.length > 0
        preferredRendererType: Shape.CurveRenderer

        ShapePath {
            strokeColor: "transparent"
            fillColor: root.fill ? Style.tint5 : "transparent"
            PathPolyline { path: root.areaPoints }
        }

        ShapePath {
            strokeColor: root.color
            strokeWidth: root.lineWidth
            fillColor: "transparent"
            capStyle: ShapePath.RoundCap
            joinStyle: ShapePath.RoundJoin
            PathPolyline { path: root.points }
        }
    }
}
