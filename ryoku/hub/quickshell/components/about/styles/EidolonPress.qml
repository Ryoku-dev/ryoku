import QtQuick
import QtQuick.Shapes

Shape {
    id: press
    property var points: []
    property color pigment: "transparent"
    property color edge: "transparent"
    property real thickness: 0
    preferredRendererType: Shape.CurveRenderer
    ShapePath {
        fillColor: press.pigment
        strokeColor: press.edge
        strokeWidth: press.thickness
        joinStyle: ShapePath.MiterJoin
        PathPolyline { path: press.points.length ? press.points.concat([press.points[0]]) : [] }
    }
}
