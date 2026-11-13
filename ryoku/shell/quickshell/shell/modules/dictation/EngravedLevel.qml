import QtQuick

// The microphone level as engraved data: one fine crest line, mirrored, over a
// 45 degree hatch, on a vein baseline. No area fill, no glow, no gradient: the
// drawing is a measurement, and it lies flat when nothing is being said.
Canvas {
    id: root

    property var levels: []
    property bool live: false
    property real drift: 0
    property color ink: "#cccccc"
    property color vein: "#555555"
    property real scale: 1

    onLevelsChanged: requestPaint()
    onLiveChanged: requestPaint()
    onDriftChanged: requestPaint()
    onWidthChanged: requestPaint()
    onHeightChanged: requestPaint()

    onPaint: {
        const ctx = getContext("2d");
        ctx.reset();
        const w = width;
        const h = height;
        if (w <= 0 || h <= 0)
            return;
        const mid = h / 2;
        const values = levels.length > 0 ? levels : [0.04];
        const n = values.length;

        // mirrored envelope, tapered at both ends so the shape closes
        const points = [];
        for (let x = 0; x <= w; x += 2) {
            const t = w > 0 ? x / w : 0;
            const index = Math.min(n - 1, Math.floor(t * (n - 1)));
            const taper = Math.sin(Math.PI * t);
            const flow = 0.9 + 0.1 * Math.sin(x * 0.05 + drift);
            points.push({ x: x, y: mid - values[index] * mid * 0.9 * taper * flow });
        }

        // the hatch, clipped to the band between the crest and the baseline
        ctx.save();
        ctx.beginPath();
        ctx.moveTo(0, mid);
        for (const p of points)
            ctx.lineTo(p.x, p.y);
        ctx.lineTo(w, mid);
        ctx.closePath();
        ctx.clip();
        ctx.lineWidth = 1;
        ctx.strokeStyle = Qt.alpha(vein, 0.42);
        const step = 6 * scale;
        for (let x = -h; x < w + h; x += step) {
            ctx.beginPath();
            ctx.moveTo(x, mid);
            ctx.lineTo(x + h, mid - h);
            ctx.stroke();
        }
        ctx.restore();

        // baseline vein
        ctx.lineWidth = 1;
        ctx.strokeStyle = Qt.alpha(vein, 0.6);
        ctx.beginPath();
        ctx.moveTo(0, mid);
        ctx.lineTo(w, mid);
        ctx.stroke();

        if (!live)
            return;

        // the crest: one fine line, both halves
        ctx.lineWidth = 1.4 * scale;
        ctx.lineJoin = "round";
        ctx.strokeStyle = ink;
        for (let half = 0; half < 2; half++) {
            const sign = half === 0 ? -1 : 1;
            ctx.beginPath();
            for (let i = 0; i < points.length; i++) {
                const p = points[i];
                const y = mid + sign * (mid - p.y);
                if (i === 0)
                    ctx.moveTo(p.x, y);
                else
                    ctx.lineTo(p.x, y);
            }
            ctx.stroke();
        }
    }
}
