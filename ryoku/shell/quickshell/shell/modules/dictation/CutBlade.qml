import QtQuick

// A bar whose leading edge is cut at 45 degrees, so progress reads as a stroke
// of the chisel rather than a rounded fill. Here it measures the recording
// against the automatic stop.
Canvas {
    id: root

    property real fraction: 0
    property color track: "#444444"
    property color fill: "#cccccc"

    onFractionChanged: requestPaint()
    onWidthChanged: requestPaint()
    onHeightChanged: requestPaint()

    onPaint: {
        const ctx = getContext("2d");
        ctx.reset();
        const w = width;
        const h = height;
        if (w <= 0 || h <= 0)
            return;
        ctx.fillStyle = Qt.alpha(track, 0.6);
        ctx.fillRect(0, 0, w, h);
        const filled = Math.max(0, Math.min(1, fraction)) * w;
        if (filled <= 0)
            return;
        ctx.fillStyle = fill;
        ctx.beginPath();
        ctx.moveTo(0, 0);
        ctx.lineTo(Math.max(0, filled - h), 0);
        ctx.lineTo(filled, h);
        ctx.lineTo(0, h);
        ctx.closePath();
        ctx.fill();
    }
}
