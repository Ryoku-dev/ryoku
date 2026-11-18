import QtQuick
import QtQuick.Shapes

// A two-colour gradient at an angle, in a cut, round or circular tile. The angle
// is the background's own: 0 runs left to right, 90 top to bottom.
Shape {
    id: tile

    property color a: "white"
    property color b: "black"
    property real angle: 135
    // "cut", "round" or "circle"
    property string form: "round"
    property real corner: 4

    readonly property real dx: Math.cos(tile.angle * Math.PI / 180)
    readonly property real dy: Math.sin(tile.angle * Math.PI / 180)
    readonly property real reach: Math.abs(tile.width / 2 * tile.dx) + Math.abs(tile.height / 2 * tile.dy)

    preferredRendererType: Shape.CurveRenderer
    antialiasing: true

    ShapePath {
        strokeColor: "transparent"
        fillGradient: LinearGradient {
            x1: tile.width / 2 - tile.dx * tile.reach
            y1: tile.height / 2 - tile.dy * tile.reach
            x2: tile.width / 2 + tile.dx * tile.reach
            y2: tile.height / 2 + tile.dy * tile.reach
            GradientStop { position: 0; color: tile.a }
            GradientStop { position: 1; color: tile.b }
        }
        PathPolyline {
            path: {
                const w = tile.width, h = tile.height;
                if (tile.form === "cut") {
                    const c = Math.min(tile.corner, w / 2, h / 2);
                    return [Qt.point(c, 0), Qt.point(w - c, 0), Qt.point(w, c), Qt.point(w, h - c),
                        Qt.point(w - c, h), Qt.point(c, h), Qt.point(0, h - c), Qt.point(0, c), Qt.point(c, 0)];
                }
                // a circle or a rounded rect as a polyline, so one path serves every form
                const r = tile.form === "circle" ? Math.min(w, h) / 2 : Math.min(tile.corner, w / 2, h / 2);
                const pts = [];
                const corners = [[w - r, r, -90], [w - r, h - r, 0], [r, h - r, 90], [r, r, 180]];
                for (let i = 0; i < 4; i++) {
                    for (let s = 0; s <= 8; s++) {
                        const t = (corners[i][2] + s * 90 / 8) * Math.PI / 180;
                        pts.push(Qt.point(corners[i][0] + Math.cos(t) * r, corners[i][1] + Math.sin(t) * r));
                    }
                }
                pts.push(pts[0]);
                return pts;
            }
        }
    }
}
