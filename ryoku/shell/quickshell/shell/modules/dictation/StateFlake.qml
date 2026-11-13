import QtQuick

// The state token: a slanted quadrilateral that is outlined when nothing is
// happening, filled by its mineral while something is, and part filled while the
// engine is becoming ready. Lithic never lets colour carry a state alone, so the
// word always sits beside it.
Canvas {
    id: root

    required property color mineral
    // 0 outline only, 0..1 filled to that fraction ("becoming"), 1 fully filled
    property real fraction: 1.0
    property real hairline: 1.5
    property color outline: "#888888"

    onMineralChanged: requestPaint()
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
        const skew = h * 0.55;
        ctx.beginPath();
        ctx.moveTo(skew, 0);
        ctx.lineTo(w, 0);
        ctx.lineTo(w - skew, h);
        ctx.lineTo(0, h);
        ctx.closePath();
        if (fraction >= 0.001) {
            ctx.save();
            ctx.clip();
            ctx.fillStyle = mineral;
            ctx.fillRect(0, 0, w * Math.min(1, fraction), h);
            ctx.restore();
        }
        if (fraction < 0.999 || mineral === outline) {
            ctx.lineWidth = hairline;
            ctx.strokeStyle = outline;
            ctx.beginPath();
            ctx.moveTo(skew, 0);
            ctx.lineTo(w, 0);
            ctx.lineTo(w - skew, h);
            ctx.lineTo(0, h);
            ctx.closePath();
            ctx.stroke();
        }
    }
}
