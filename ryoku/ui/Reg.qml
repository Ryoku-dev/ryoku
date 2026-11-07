import QtQuick
import Ryoku.Ui.Singletons

// The registration sheet: the reference HUD's backdrop, drawn once. A faint
// dot grid with sparse + marks at the intersections, and four print-register
// crosses in the corners, so the paper reads as an instrument sheet before a
// single control lands on it. Ink only, one static Canvas, no animation.
Canvas {
    id: reg

    property int cell: 46          // grid pitch, px
    property int every: 4          // a + mark each N intersections
    property real dotAlpha: 0.07
    property real crossAlpha: 0.13
    property real cornerAlpha: 0.32
    property int margin: 18        // corner-cross inset

    onWidthChanged: requestPaint()
    onHeightChanged: requestPaint()
    Component.onCompleted: requestPaint()

    onPaint: {
        var c = getContext("2d");
        c.reset();
        var r = 205 / 255, g = 196 / 255, b = 186 / 255;

        // the dot grid, a + mark on the sparse beat
        for (var gy = 0; gy * cell < height + cell; gy++) {
            for (var gx = 0; gx * cell < width + cell; gx++) {
                var x = gx * cell, y = gy * cell;
                if (gx % every === 0 && gy % every === 0) {
                    c.fillStyle = Qt.rgba(r, g, b, crossAlpha);
                    c.fillRect(x - 3, y, 7, 1);
                    c.fillRect(x, y - 3, 1, 7);
                } else {
                    c.fillStyle = Qt.rgba(r, g, b, dotAlpha);
                    c.fillRect(x, y, 1, 1);
                }
            }
        }

        // print-register crosses in the four corners
        c.fillStyle = Qt.rgba(r, g, b, cornerAlpha);
        var pts = [[margin, margin], [width - margin, margin],
                   [margin, height - margin], [width - margin, height - margin]];
        for (var i = 0; i < pts.length; i++) {
            var px = pts[i][0], py = pts[i][1];
            c.fillRect(px - 7, py, 15, 1);
            c.fillRect(px, py - 7, 1, 15);
        }
    }
}
