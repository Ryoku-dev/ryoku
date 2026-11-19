import QtQuick
import QtQuick.Effects
import QtQuick.Shapes
import Lithic
import "../../look"

// The panel's plane. Lithic: a drawer flush on the right edge, its left corners
// cut, a hairline round the open sides, and one engraved shine that crosses it
// as it lands. Material: a floating side sheet, radius 28 on surfaceContainerLow
// at level 1. Adwaita: the sidebar card, radius 12 with its shade border and a
// soft shadow. Fluent: an acrylic pane, radius 8, outer and inner strokes.
Item {
    id: sheet

    property real u: 1
    // Material's container transform rounds from the button it grew out of
    property real radius: 28 * sheet.u
    // the shine's sweep, -0.3 to 1.3; outside it nothing is drawn
    property real shine: -0.3

    readonly property real cut: 8 * sheet.u
    readonly property real hairline: Math.max(1, Math.round(sheet.u))
    // the plane's colour, for what fades into it
    readonly property color fill: Look.lithic ? Look.planes.pane : Look.material ? Look.stone1
        : Look.adwaita ? Look.planes.side : Look.acrylic

    // Lithic
    RectangularShadow {
        visible: Look.lithic
        anchors.fill: parent
        anchors.topMargin: sheet.cut
        anchors.bottomMargin: sheet.cut
        offset.x: -2 * sheet.u
        blur: 14 * sheet.u
        radius: sheet.cut
        color: Look.shadow
        opacity: 0.6
    }
    CutRect {
        visible: Look.lithic
        anchors.fill: parent
        design: "lithic"
        cutTopLeft: sheet.cut
        cutBottomLeft: sheet.cut
        cutTopRight: 0
        cutBottomRight: 0
        fill: Look.planes.pane
    }
    Shape {
        visible: Look.lithic
        anchors.fill: parent
        preferredRendererType: Shape.CurveRenderer
        ShapePath {
            strokeColor: Look.planes.paneStroke
            strokeWidth: sheet.hairline
            fillColor: "transparent"
            PathPolyline {
                path: {
                    const w = sheet.width, h = sheet.height, c = sheet.cut, t = sheet.hairline / 2;
                    return [Qt.point(w, t), Qt.point(c, t), Qt.point(t, c), Qt.point(t, h - c),
                        Qt.point(c, h - t), Qt.point(w, h - t)];
                }
            }
        }
    }
    Shape {
        anchors.fill: parent
        visible: Look.lithic && sheet.shine > -0.25 && sheet.shine < 1.25
        preferredRendererType: Shape.CurveRenderer
        ShapePath {
            strokeColor: "transparent"
            fillGradient: LinearGradient {
                x1: 0
                y1: sheet.shine * sheet.height - 60 * sheet.u
                x2: sheet.width
                y2: sheet.shine * sheet.height + 20 * sheet.u
                GradientStop { position: 0; color: "transparent" }
                GradientStop { position: 0.5; color: Look.dark ? Lithic.alpha(Look.fg, 0.06) : Lithic.alpha(Look.stone4, 0.8) }
                GradientStop { position: 1; color: "transparent" }
            }
            PathPolyline {
                path: [Qt.point(sheet.cut, 0), Qt.point(sheet.width, 0), Qt.point(sheet.width, sheet.height),
                    Qt.point(sheet.cut, sheet.height), Qt.point(0, sheet.height - sheet.cut), Qt.point(0, sheet.cut),
                    Qt.point(sheet.cut, 0)]
            }
        }
    }

    // the round designs
    Surface {
        visible: !Look.lithic
        anchors.fill: parent
        u: sheet.u
        radius: Look.material ? sheet.radius : (Look.adwaita ? 12 : 8) * sheet.u
        fill: sheet.fill
        stroke: Look.material ? "transparent" : Look.adwaita ? Lithic.alpha(Look.fg, Look.dark ? 0.12 : 0.16) : Look.planes.cardStroke
        elevation: Look.material ? 1 : Look.adwaita ? 2 : 3
    }
}
