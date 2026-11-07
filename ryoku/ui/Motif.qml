import QtQuick
import Ryoku.Ui.Singletons

// Line ornaments from the reference sheet, drawn in ink: concentric rings, a
// stepped pyramid, a starburst, a dither block. They dress a genuinely empty
// zone (an empty state, an idle plate); they never sit behind content.
Canvas {
    id: m

    property string kind: "rings"   // rings | steps | burst | dither
    property real strength: 0.5     // stroke alpha

    onKindChanged: requestPaint()
    onWidthChanged: requestPaint()
    onHeightChanged: requestPaint()
    Component.onCompleted: requestPaint()

    onPaint: {
        var c = getContext("2d");
        c.reset();
        var r = 205 / 255, g = 196 / 255, b = 186 / 255;
        c.strokeStyle = Qt.rgba(r, g, b, strength);
        c.fillStyle = Qt.rgba(r, g, b, strength);
        c.lineWidth = 1;
        var w = width, h = height, cx = w / 2, cy = h / 2;

        if (kind === "rings") {
            var n = 7, rr = Math.min(w, h) / 2 - 1;
            for (var i = 0; i < n; i++) {
                c.beginPath();
                c.arc(cx, cy, rr * (1 - i / n), 0, 2 * Math.PI);
                c.stroke();
            }
        } else if (kind === "steps") {
            var levels = 7, base = Math.min(w, h) - 2;
            for (var j = 0; j < levels; j++) {
                var sw = base * (1 - j / levels);
                var sh = (h - 4) / levels;
                c.strokeRect(cx - sw / 2, h - (j + 1) * sh - 1, sw, sh);
            }
        } else if (kind === "burst") {
            var rays = 16, ro = Math.min(w, h) / 2 - 1;
            for (var k = 0; k < rays; k++) {
                var a = k * Math.PI * 2 / rays;
                var ri = ro * (k % 2 === 0 ? 0.25 : 0.55);
                c.beginPath();
                c.moveTo(cx + Math.cos(a) * ri, cy + Math.sin(a) * ri);
                c.lineTo(cx + Math.cos(a) * ro, cy + Math.sin(a) * ro);
                c.stroke();
            }
        } else if (kind === "dither") {
            var cell = 4;
            for (var y = 0; y * cell < h; y++)
                for (var x = 0; x * cell < w; x++)
                    if ((x + y) % 2 === 0)
                        c.fillRect(x * cell, y * cell, cell, cell);
        }
    }
}
