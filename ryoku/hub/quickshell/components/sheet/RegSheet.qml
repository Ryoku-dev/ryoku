import QtQuick
import Lithic

// the registration sheet under the page: a faint dot grid with a + on every fourth crossing
// and print-register crosses in the corners, so the paper reads as an instrument sheet. One
// Canvas, painted again only when its size or ink changes
Canvas {
    id: root

    property int cell: 24
    property int every: 4
    property int margin: 14
    property color ink: Lithic.fg
    property bool dark: Lithic.dark

    onWidthChanged: requestPaint()
    onHeightChanged: requestPaint()
    onInkChanged: requestPaint()
    onDarkChanged: requestPaint()

    onPaint: {
        var c = getContext("2d");
        c.reset();
        var dot = Lithic.alpha(root.ink, Lithic.dark ? 0.07 : 0.1);
        var plus = Lithic.alpha(root.ink, Lithic.dark ? 0.13 : 0.16);
        for (var gy = 1; gy * root.cell < root.height; gy++) {
            for (var gx = 1; gx * root.cell < root.width; gx++) {
                var x = gx * root.cell, y = gy * root.cell;
                if (gx % root.every === 0 && gy % root.every === 0) {
                    c.fillStyle = plus;
                    c.fillRect(x - 3, y, 7, 1);
                    c.fillRect(x, y - 3, 1, 7);
                } else {
                    c.fillStyle = dot;
                    c.fillRect(x, y, 1, 1);
                }
            }
        }
        c.fillStyle = Lithic.alpha(root.ink, Lithic.dark ? 0.32 : 0.36);
        var m = root.margin;
        [[m, m], [root.width - m, m], [m, root.height - m], [root.width - m, root.height - m]].forEach(p => {
            c.fillRect(p[0] - 6, p[1], 13, 1);
            c.fillRect(p[0], p[1] - 6, 1, 13);
        });
    }
}
