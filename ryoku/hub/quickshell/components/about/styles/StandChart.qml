import QtQuick
import QtQuick.Shapes
import Lithic
import Ryoku.Ui.Singletons
import "../../../registry/stand.js" as Stand

// a Stand's parameter chart: a hexagon of five rings, E inside to A outside, the six
// parameters on its spokes in the volumes' order, the machine's shape filled, and each grade
// set big at its corner. `grow(i)` and `slam(i)` (0..1) let the card draw it in corner by corner
Item {
    id: chart

    // six grades, 1 (E) to 5 (A)
    property var grades: [1, 1, 1, 1, 1, 1]
    property var grow: function (i) { return 1; }
    property var slam: function (i) { return 1; }
    property color ink: "black"
    property color paper: "white"
    property color fill: "magenta"
    property color strong: "purple"
    property color weak: "magenta"
    property real radius: 40
    property real cx: width / 2
    property real cy: height / 2

    function at(i, f) {
        var a = (-90 + i * 60) * Math.PI / 180;
        return Qt.point(chart.cx + Math.cos(a) * chart.radius * f, chart.cy + Math.sin(a) * chart.radius * f);
    }
    function ring(f) {
        var p = [0, 1, 2, 3, 4, 5].map(i => chart.at(i, f));
        return p.concat([p[0]]);
    }

    Shape {
        anchors.fill: parent
        preferredRendererType: Shape.CurveRenderer
        ShapePath {
            fillColor: chart.paper
            strokeColor: chart.ink
            strokeWidth: 2.4
            joinStyle: ShapePath.MiterJoin
            PathPolyline { path: chart.ring(1) }
        }
        ShapePath {
            fillColor: "transparent"
            strokeColor: chart.ink
            strokeWidth: 0.7
            PathMultiline {
                paths: [chart.ring(0.2), chart.ring(0.4), chart.ring(0.6), chart.ring(0.8)]
                    .concat([0, 1, 2, 3, 4, 5].map(i => [Qt.point(chart.cx, chart.cy), chart.at(i, 1)]))
            }
        }
        ShapePath {
            fillColor: chart.fill
            strokeColor: chart.ink
            strokeWidth: 2.2
            joinStyle: ShapePath.MiterJoin
            PathPolyline {
                path: {
                    var p = [0, 1, 2, 3, 4, 5].map(i => chart.at(i, chart.grades[i] / 5 * chart.grow(i)));
                    return p.concat([p[0]]);
                }
            }
        }
    }
    Rectangle {
        x: chart.cx - 2.5
        y: chart.cy - 2.5
        width: 5
        height: 5
        radius: 2.5
        color: chart.ink
    }

    // each grade at its corner; the top and bottom names sit beside their letter, the others outboard
    Repeater {
        model: 6
        delegate: Item {
            id: corner
            required property int index
            readonly property point p: chart.at(corner.index, corner.index % 3 === 0 ? 1.32 : 1.3)
            readonly property int side: corner.index % 3 === 0 ? 0 : corner.index < 3 ? 1 : -1
            readonly property int g: chart.grades[corner.index]
            readonly property real s: chart.slam(corner.index)
            Text {
                x: corner.p.x - width / 2
                y: corner.p.y - height / 2
                textFormat: Text.PlainText
                renderType: Text.QtRendering
                text: Stand.letter(corner.g)
                color: corner.g >= 4 ? chart.strong : corner.g === 3 ? chart.ink : chart.weak
                style: Text.Outline
                styleColor: chart.paper
                font.family: Lithic.font.sans
                font.weight: Font.Black
                font.italic: true
                font.pixelSize: 21
                scale: 1 + 1.8 * (1 - (1 - Math.pow(1 - corner.s, 3)))
                opacity: corner.s > 0 ? 1 : 0
            }
            Rectangle {
                x: names.x - 2
                y: names.y
                width: names.width + 4
                height: names.height
                color: chart.paper
            }
            Column {
                id: names
                x: corner.side < 0 ? corner.p.x - 11 - width : corner.p.x + (corner.side === 0 ? 9 : 11)
                y: corner.p.y - height / 2 + 1
                spacing: -1
                Text {
                    anchors.right: corner.side < 0 ? parent.right : undefined
                    textFormat: Text.PlainText
                    renderType: Text.QtRendering
                    text: Stand.params[corner.index].ja
                    color: chart.ink
                    font.family: "Noto Sans CJK JP"
                    font.weight: Font.Bold
                    font.pixelSize: 8
                }
                Text {
                    anchors.right: corner.side < 0 ? parent.right : undefined
                    textFormat: Text.PlainText
                    renderType: Text.QtRendering
                    text: I18n.tr(Stand.params[corner.index].en).toUpperCase()
                    color: chart.ink
                    font.family: Lithic.font.mono
                    font.weight: Font.Bold
                    font.pixelSize: 5
                    font.letterSpacing: 0.4
                }
            }
        }
    }
}
