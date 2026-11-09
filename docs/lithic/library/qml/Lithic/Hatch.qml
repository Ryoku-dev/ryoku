import QtQuick
import QtQuick.Shapes

Item {
    id: root

    property real spacing: LithicTokens.size.chartHatchGap
    property real lineWidth: LithicTokens.size.chartHatchLine
    property color color: Lithic.alpha(Lithic.fg, LithicTokens.alpha.hatch)
    clip: true

    readonly property var _hatchLines: {
        var result = [];
        var step = Math.max(1, spacing);
        for (var offset = -height; offset <= width; offset += step)
            result.push([Qt.point(offset, height), Qt.point(offset + height, 0)]);
        return result;
    }

    Shape {
        anchors.fill: parent
        antialiasing: true
        ShapePath {
            fillColor: "transparent"
            strokeColor: root.color
            strokeWidth: root.lineWidth
            capStyle: ShapePath.FlatCap
            PathMultiline { paths: root._hatchLines }
        }
    }
}
