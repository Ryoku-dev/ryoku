pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Shapes as Shapes
import Lithic
import "../../../../ask/look"
import "../../../parts"
import "../../../../ask/lib/downloads.js" as Logic
import "../../../../ask/lib/format.js" as Format

// A rate over the last minute, newest at the right. Lithic engraves it (a line
// over 45 degree strokes, as the library's EngravedChart does), the others wash
// the area. The path is rebuilt once per sample; between samples the whole
// trace slides one slot on the animation clock, and rests when samples stop.
Item {
    id: root

    property real s: 1
    // rates in B/s, oldest first
    property var samples: []
    property int slots: 60
    property color ink: Lithic.lapis
    // name the scale's top in the plot
    property bool scaled: true
    property bool ruled: true

    readonly property var series: Logic.trace(root.samples, root.slots)
    readonly property real crest: Logic.ceiling(root.series.peak)
    // one sample waits beyond each edge, so a gliding trace never shows a gap
    readonly property real slot: root.width / Math.max(1, root.slots - 2)
    readonly property real lineWidth: Math.max(1.5, 1.5 * root.s)
    readonly property real floorY: root.height - (root.ruled ? 1 : 0)
    // how high the scale's top sits: the named line keeps room for its name above it
    readonly property real reach: root.scaled ? Math.max(1, root.floorY - Math.round(15 * root.s)) : root.floorY - root.lineWidth
    readonly property var line: {
        const out = [];
        const points = root.series.points;
        for (var i = 0; i < points.length; i++)
            out.push(Qt.point(root.width - (points.length - 1 - i) * root.slot,
                root.floorY - Math.min(1, points[i].y / root.crest) * root.reach));
        return out;
    }
    readonly property var area: {
        if (root.line.length < 2)
            return [];
        const out = root.line.slice();
        out.push(Qt.point(root.line[root.line.length - 1].x, root.floorY));
        out.push(Qt.point(root.line[0].x, root.floorY));
        out.push(root.line[0]);
        return out;
    }
    // each stroke climbs from the floor until it meets the line
    readonly property var hatch: {
        const out = [];
        const pts = root.line;
        if (!Look.lithic || pts.length < 2)
            return out;
        const gap = Math.max(4, Math.round(5 * root.s));
        const first = pts[0].x;
        const last = pts[pts.length - 1].x;
        function top(x) {
            const i = Math.max(1, Math.min(pts.length - 1, Math.ceil((x - first) / root.slot)));
            const span = pts[i].x - pts[i - 1].x;
            return span <= 0 ? pts[i].y : pts[i - 1].y + (pts[i].y - pts[i - 1].y) * (x - pts[i - 1].x) / span;
        }
        for (var x0 = first - root.floorY; x0 < last; x0 += gap) {
            var open = null;
            for (var x = Math.max(first, x0); x <= last + 0.5; x += 1) {
                const y = root.floorY - (x - x0);
                const inside = y >= top(Math.min(x, last)) && y <= root.floorY;
                if (inside && open === null)
                    open = Qt.point(x, y);
                if ((!inside || x + 1 > last + 0.5) && open !== null) {
                    if (x - open.x > 1.5)
                        out.push([open, Qt.point(x, y)]);
                    open = null;
                }
                if (y < 0)
                    break;
            }
        }
        return out;
    }

    clip: true

    // a new sample arrives one slot to the right and glides in
    property real slide: 0
    NumberAnimation {
        id: glide
        target: root
        property: "slide"
        from: root.slot
        to: 0
        duration: Look.dur(1000)
    }
    onSamplesChanged: {
        if (root.visible && root.samples.length > 1 && !Look.still)
            glide.restart();
    }

    Rectangle {
        visible: root.ruled
        y: root.height - 1
        width: parent.width
        height: 1
        color: Look.lithic ? Lithic.vein : Lithic.alpha(Lithic.fg, Look.dark ? 0.16 : 0.2)
    }

    Shapes.Shape {
        x: root.slide
        width: parent.width
        height: parent.height
        visible: root.area.length > 0
        preferredRendererType: Shapes.Shape.CurveRenderer

        Shapes.ShapePath {
            strokeColor: "transparent"
            fillColor: Look.lithic ? "transparent" : Lithic.alpha(root.ink, 0.14)
            PathPolyline { path: root.area }
        }
        Shapes.ShapePath {
            strokeColor: Look.lithic ? Lithic.alpha(root.ink, 0.36) : "transparent"
            strokeWidth: 1
            fillColor: "transparent"
            capStyle: Shapes.ShapePath.FlatCap
            PathMultiline { paths: root.hatch }
        }
        Shapes.ShapePath {
            strokeColor: root.ink
            strokeWidth: root.lineWidth
            fillColor: "transparent"
            joinStyle: Look.lithic ? Shapes.ShapePath.MiterJoin : Shapes.ShapePath.RoundJoin
            capStyle: Look.lithic ? Shapes.ShapePath.FlatCap : Shapes.ShapePath.RoundCap
            PathPolyline { path: root.line }
        }
    }

    Rectangle {
        visible: root.scaled && root.series.peak > 0
        y: Math.round(root.floorY - root.reach)
        width: parent.width
        height: 1
        color: Lithic.alpha(Lithic.fg, Look.dark ? 0.08 : 0.1)
    }
    Line {
        visible: root.scaled && root.series.peak > 0
        y: 0
        s: root.s
        size: 10.5
        figures: true
        text: Format.speed(root.crest)
        ink: Lithic.fg3
    }
}
