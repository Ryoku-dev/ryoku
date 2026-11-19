import QtQuick
import QtQuick.Shapes
import "look"

Item {
    id: canvas

    required property int sx
    required property int sy
    property var model: null
    property var draft: null
    property int revision: 0
    property var selectedIndex: null
    property var moveOffset: null

    // Kinds that sample the captured pixels are drawn by EffectLayer; this
    // canvas only strokes vectors.
    readonly property var effectKinds: ["blur", "pixelate", "redact", "magnify", "spotlight"]
    function isEffect(t) { return canvas.effectKinds.indexOf(t) !== -1; }

    function shifted(a, dx, dy) {
        var pts = [];
        for (var i = 0; i < a.points.length; i++)
            pts.push({ x: a.points[i].x + dx, y: a.points[i].y + dy });
        var copy = JSON.parse(JSON.stringify(a));
        copy.points = pts;
        return copy;
    }

    function items() {
        var src = model ? model.items.slice() : [];
        if (selectedIndex !== null && moveOffset
            && (moveOffset.x !== 0 || moveOffset.y !== 0)
            && selectedIndex >= 0 && selectedIndex < src.length)
            src[selectedIndex] = shifted(src[selectedIndex], moveOffset.x, moveOffset.y);
        if (draft) src.push(draft);
        return src;
    }

    function lp(a, i) {
        return Qt.point(a.points[i].x - sx, a.points[i].y - sy);
    }

    function polyPath(a) {
        var out = [];
        for (var i = 0; i < a.points.length; i++)
            out.push(Qt.point(a.points[i].x - sx, a.points[i].y - sy));
        return out;
    }

    function strokeColorOf(a) {
        if (a.type !== "marker") return a.color;
        var c = Qt.color(a.color);
        return Qt.rgba(c.r, c.g, c.b, 0.35);
    }

    function strokeWidthOf(a) {
        return a.type === "marker" ? a.width * 2.5 : a.width;
    }

    function ellipseGeom(a) {
        var p0 = lp(a, 0), p1 = lp(a, 1);
        return {
            cx: (p0.x + p1.x) / 2,
            cy: (p0.y + p1.y) / 2,
            rx: Math.abs(p1.x - p0.x) / 2,
            ry: Math.abs(p1.y - p0.y) / 2
        };
    }

    function arrowHead(a) {
        var p0 = lp(a, 0), p1 = lp(a, 1);
        var ang = Math.atan2(p1.y - p0.y, p1.x - p0.x);
        var len = Math.max(a.width * 5, 22);
        var spread = 0.45;
        return {
            tip: p1,
            a: Qt.point(p1.x - len * Math.cos(ang - spread), p1.y - len * Math.sin(ang - spread)),
            b: Qt.point(p1.x - len * Math.cos(ang + spread), p1.y - len * Math.sin(ang + spread))
        };
    }

    // hand-drawn look: a deterministic per-item jitter (seeded from the item's
    // points so it stays stable across repaints) wobbles the stroke.
    function rSeed(a) {
        var p0 = a.points[0], p1 = a.points[a.points.length - 1];
        return (Math.abs(Math.round(p0.x * 13.1 + p0.y * 7.3 + p1.x * 3.7 + p1.y * 5.9)) % 100000) + 1;
    }
    function rMaker(seed) {
        var s = seed;
        return function () { s = (s * 9301 + 49297) % 233280; return s / 233280; };
    }
    function rSeg(x0, y0, x1, y1, amp, rnd, out) {
        out.push(Qt.point(x0 + (rnd() - 0.5) * amp * 0.6, y0 + (rnd() - 0.5) * amp * 0.6));
        for (var i = 1; i < 3; i++) {
            var t = i / 3;
            out.push(Qt.point(x0 + (x1 - x0) * t + (rnd() - 0.5) * amp,
                              y0 + (y1 - y0) * t + (rnd() - 0.5) * amp));
        }
        out.push(Qt.point(x1 + (rnd() - 0.5) * amp * 0.6, y1 + (rnd() - 0.5) * amp * 0.6));
        return out;
    }
    // Catmull-Rom spline through the jittered control points, so the hand-drawn
    // stroke flows like a pen instead of reading as straight jagged segments.
    function rSmooth(pts) {
        if (!pts || pts.length < 3)
            return pts || [];
        var out = [], segs = 8;
        for (var i = 0; i < pts.length - 1; i++) {
            var p0 = pts[i > 0 ? i - 1 : 0];
            var p1 = pts[i];
            var p2 = pts[i + 1];
            var p3 = pts[i < pts.length - 2 ? i + 2 : i + 1];
            for (var j = 0; j < segs; j++) {
                var t = j / segs, t2 = t * t, t3 = t2 * t;
                out.push(Qt.point(
                    0.5 * (2 * p1.x + (-p0.x + p2.x) * t + (2 * p0.x - 5 * p1.x + 4 * p2.x - p3.x) * t2 + (-p0.x + 3 * p1.x - 3 * p2.x + p3.x) * t3),
                    0.5 * (2 * p1.y + (-p0.y + p2.y) * t + (2 * p0.y - 5 * p1.y + 4 * p2.y - p3.y) * t2 + (-p0.y + 3 * p1.y - 3 * p2.y + p3.y) * t3)));
            }
        }
        out.push(pts[pts.length - 1]);
        return out;
    }
    function roughMain(a) {
        var rnd = rMaker(rSeed(a));
        var amp = Math.min(Math.max((a.width || 4) * 0.9, 2.5), 7);
        var p0 = lp(a, 0), p1 = lp(a, 1);
        var out = [];
        if (a.type === "line" || a.type === "arrow")
            return rSmooth(rSeg(p0.x, p0.y, p1.x, p1.y, amp, rnd, out));
        if (a.type === "rect") {
            var xa = Math.min(p0.x, p1.x), xb = Math.max(p0.x, p1.x);
            var ya = Math.min(p0.y, p1.y), yb = Math.max(p0.y, p1.y);
            rSeg(xa, ya, xb, ya, amp, rnd, out);
            rSeg(xb, ya, xb, yb, amp, rnd, out);
            rSeg(xb, yb, xa, yb, amp, rnd, out);
            rSeg(xa, yb, xa, ya, amp, rnd, out);
            return rSmooth(out);
        }
        if (a.type === "ellipse") {
            var cx = (p0.x + p1.x) / 2, cy = (p0.y + p1.y) / 2;
            var rx = Math.abs(p1.x - p0.x) / 2, ry = Math.abs(p1.y - p0.y) / 2;
            var steps = 22;
            for (var i = 0; i <= steps; i++) {
                var ang = (i / steps) * Math.PI * 2;
                var jr = 1 + (rnd() - 0.5) * 0.06;
                out.push(Qt.point(cx + Math.cos(ang) * rx * jr, cy + Math.sin(ang) * ry * jr));
            }
            return rSmooth(out);
        }
        return out;
    }
    function roughHead(a) {
        if (a.type !== "arrow") return [];
        var rnd = rMaker(rSeed(a) + 7);
        var h = arrowHead(a);
        var out = [];
        rSeg(h.a.x, h.a.y, h.tip.x, h.tip.y, 2.5, rnd, out);
        rSeg(h.tip.x, h.tip.y, h.b.x, h.b.y, 2.5, rnd, out);
        return rSmooth(out);
    }

    Repeater {
        model: { canvas.revision; return canvas.items(); }

        Item {
            id: cell
            required property var modelData
            readonly property var a: modelData
            readonly property bool present: a !== undefined && a !== null && a.points !== undefined
            readonly property bool isText: present && a.type === "text" && a.points.length >= 1
            readonly property bool isCounter: present && a.type === "counter" && a.points.length >= 1
            readonly property bool isOcr: present && a.type === "ocr" && a.points.length >= 2
            readonly property bool valid: present && a.points.length >= 2 && !canvas.isEffect(a.type)
            readonly property string kind: valid ? a.type : (isText ? "text" : "")
            anchors.fill: parent
            visible: valid || isText || isCounter

            Rectangle {
                visible: cell.valid && cell.kind === "rect" && cell.a.rough !== true
                x: cell.valid ? Math.min(cell.a.points[0].x, cell.a.points[1].x) - canvas.sx : 0
                y: cell.valid ? Math.min(cell.a.points[0].y, cell.a.points[1].y) - canvas.sy : 0
                width: cell.valid ? Math.abs(cell.a.points[1].x - cell.a.points[0].x) : 0
                height: cell.valid ? Math.abs(cell.a.points[1].y - cell.a.points[0].y) : 0
                color: (cell.valid && cell.a.filled === true) ? cell.a.color : "transparent"
                border.color: cell.valid ? cell.a.color : "transparent"
                border.width: cell.valid ? cell.a.width : 0
                antialiasing: true
            }

            Rectangle {
                visible: cell.valid && cell.kind === "marker"
                x: cell.valid ? Math.min(cell.a.points[0].x, cell.a.points[1].x) - canvas.sx : 0
                y: cell.valid ? Math.min(cell.a.points[0].y, cell.a.points[1].y) - canvas.sy : 0
                width: cell.valid ? Math.abs(cell.a.points[1].x - cell.a.points[0].x) : 0
                height: cell.valid ? Math.abs(cell.a.points[1].y - cell.a.points[0].y) : 0
                color: {
                    if (!cell.valid) return "transparent";
                    var c = Qt.color(cell.a.color);
                    return Qt.rgba(c.r, c.g, c.b, 0.4);
                }
                antialiasing: true
            }

            Shape {
                id: polyShape
                anchors.fill: parent
                antialiasing: true
                preferredRendererType: Shape.CurveRenderer
                visible: cell.valid && (cell.kind === "pen"
                    || ((cell.kind === "line" || cell.kind === "arrow") && cell.a.rough !== true))

                ShapePath {
                    strokeColor: cell.valid ? canvas.strokeColorOf(cell.a) : "transparent"
                    strokeWidth: cell.valid ? canvas.strokeWidthOf(cell.a) : 0
                    fillColor: "transparent"
                    capStyle: ShapePath.RoundCap
                    joinStyle: ShapePath.RoundJoin
                    startX: cell.valid ? canvas.lp(cell.a, 0).x : 0
                    startY: cell.valid ? canvas.lp(cell.a, 0).y : 0
                    PathPolyline {
                        path: {
                            if (!cell.valid) return [];
                            if (cell.kind === "pen") return canvas.polyPath(cell.a);
                            return [canvas.lp(cell.a, 0), canvas.lp(cell.a, 1)];
                        }
                    }
                }
            }

            Shape {
                id: ellShape
                anchors.fill: parent
                antialiasing: true
                preferredRendererType: Shape.CurveRenderer
                visible: cell.valid && cell.kind === "ellipse" && cell.a.rough !== true
                readonly property var eg: (cell.valid && cell.kind === "ellipse") ? canvas.ellipseGeom(cell.a) : null

                ShapePath {
                    strokeColor: cell.valid ? canvas.strokeColorOf(cell.a) : "transparent"
                    strokeWidth: cell.valid ? canvas.strokeWidthOf(cell.a) : 0
                    fillColor: (cell.valid && cell.a.filled === true) ? cell.a.color : "transparent"
                    capStyle: ShapePath.RoundCap
                    joinStyle: ShapePath.RoundJoin
                    startX: ellShape.eg ? ellShape.eg.cx - ellShape.eg.rx : 0
                    startY: ellShape.eg ? ellShape.eg.cy : 0
                    PathArc {
                        x: ellShape.eg ? ellShape.eg.cx + ellShape.eg.rx : 0
                        y: ellShape.eg ? ellShape.eg.cy : 0
                        radiusX: ellShape.eg ? ellShape.eg.rx : 0
                        radiusY: ellShape.eg ? ellShape.eg.ry : 0
                    }
                    PathArc {
                        x: ellShape.eg ? ellShape.eg.cx - ellShape.eg.rx : 0
                        y: ellShape.eg ? ellShape.eg.cy : 0
                        radiusX: ellShape.eg ? ellShape.eg.rx : 0
                        radiusY: ellShape.eg ? ellShape.eg.ry : 0
                    }
                }
            }

            Shape {
                id: headShape
                anchors.fill: parent
                antialiasing: true
                preferredRendererType: Shape.CurveRenderer
                visible: cell.valid && cell.kind === "arrow" && cell.a.rough !== true
                readonly property var pts: (cell.valid && cell.kind === "arrow") ? canvas.arrowHead(cell.a) : null

                ShapePath {
                    strokeColor: cell.valid ? cell.a.color : "transparent"
                    strokeWidth: cell.valid ? cell.a.width : 0
                    fillColor: cell.valid ? cell.a.color : "transparent"
                    capStyle: ShapePath.RoundCap
                    joinStyle: ShapePath.RoundJoin
                    startX: headShape.pts ? headShape.pts.tip.x : 0
                    startY: headShape.pts ? headShape.pts.tip.y : 0
                    PathLine { x: headShape.pts ? headShape.pts.a.x : 0; y: headShape.pts ? headShape.pts.a.y : 0 }
                    PathLine { x: headShape.pts ? headShape.pts.b.x : 0; y: headShape.pts ? headShape.pts.b.y : 0 }
                    PathLine { x: headShape.pts ? headShape.pts.tip.x : 0; y: headShape.pts ? headShape.pts.tip.y : 0 }
                }
            }

            Text {
                visible: cell.isText && cell.a !== canvas.draft
                x: cell.isText ? cell.a.points[0].x - canvas.sx : 0
                y: cell.isText ? cell.a.points[0].y - canvas.sy : 0
                text: cell.isText ? (cell.a.text || "") : ""
                color: cell.isText ? cell.a.color : "transparent"
                font.family: Look.ui
                font.pixelSize: cell.isText ? cell.a.size : 16
                textFormat: Text.PlainText
                renderType: Text.NativeRendering
            }

            Shape {
                id: roughShape
                anchors.fill: parent
                antialiasing: true
                preferredRendererType: Shape.CurveRenderer
                visible: cell.valid && cell.a.rough === true
                ShapePath {
                    strokeColor: cell.valid ? cell.a.color : "transparent"
                    strokeWidth: cell.valid ? cell.a.width : 0
                    fillColor: "transparent"
                    capStyle: ShapePath.RoundCap
                    joinStyle: ShapePath.RoundJoin
                    PathPolyline { path: (cell.valid && cell.a.rough === true) ? canvas.roughMain(cell.a) : [] }
                }
                ShapePath {
                    strokeColor: (cell.valid && cell.a.type === "arrow") ? cell.a.color : "transparent"
                    strokeWidth: (cell.valid && cell.a.type === "arrow") ? cell.a.width : 0
                    fillColor: "transparent"
                    capStyle: ShapePath.RoundCap
                    joinStyle: ShapePath.RoundJoin
                    PathPolyline { path: (cell.valid && cell.a.rough === true && cell.a.type === "arrow") ? canvas.roughHead(cell.a) : [] }
                }
            }

            Rectangle {
                id: counterBadge
                readonly property real cr: cell.isCounter ? ((cell.a.width || 4) * 2.5 + 9) : 14
                visible: cell.isCounter
                x: cell.isCounter ? cell.a.points[0].x - canvas.sx - cr : 0
                y: cell.isCounter ? cell.a.points[0].y - canvas.sy - cr : 0
                width: cr * 2
                height: cr * 2
                radius: cr
                color: cell.isCounter ? cell.a.color : "transparent"
                border.color: Look.inkOn(cell.isCounter ? cell.a.color : "black")
                border.width: 2
                Text {
                    anchors.centerIn: parent
                    text: cell.isCounter ? cell.a.n : ""
                    color: Look.inkOn(cell.isCounter ? cell.a.color : "black")
                    font.family: Look.ui
                    font.pixelSize: counterBadge.cr * 1.05
                    font.weight: Font.Bold
                }
            }

            // The OCR marquee is a live drag, never a stored annotation, so it
            // draws as a dashed frame that leaves no mark on the export.
            Shape {
                anchors.fill: parent
                antialiasing: true
                preferredRendererType: Shape.CurveRenderer
                visible: cell.isOcr
                ShapePath {
                    strokeColor: cell.isOcr ? Look.accent : "transparent"
                    strokeWidth: 1.5
                    strokeStyle: ShapePath.DashLine
                    dashPattern: [4, 3]
                    fillColor: cell.isOcr
                        ? Qt.rgba(Look.accent.r, Look.accent.g, Look.accent.b, 0.10)
                        : "transparent"
                    PathRectangle {
                        x: cell.isOcr ? Math.min(cell.a.points[0].x, cell.a.points[1].x) - canvas.sx : 0
                        y: cell.isOcr ? Math.min(cell.a.points[0].y, cell.a.points[1].y) - canvas.sy : 0
                        width: cell.isOcr ? Math.abs(cell.a.points[1].x - cell.a.points[0].x) : 0
                        height: cell.isOcr ? Math.abs(cell.a.points[1].y - cell.a.points[0].y) : 0
                    }
                }
            }
        }
    }
}
