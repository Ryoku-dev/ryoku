// The open ripple: one shared clock drives every row, each starting a step
// after the one above, eased on the design's curve (a Qt bezierCurve list).

function cubic(a, b, t) {
    var u = 1 - t;
    return 3 * u * u * t * a + 3 * u * t * t * b + t * t * t;
}

// The y of the curve [x1, y1, x2, y2, 1, 1] at progress x.
function ease(curve, x) {
    var p = Math.max(0, Math.min(1, Number(x) || 0));
    if (!Array.isArray(curve) && !(curve && typeof curve.length === "number"))
        return p;
    if (curve.length < 4)
        return p;
    var x1 = Number(curve[0]), y1 = Number(curve[1]), x2 = Number(curve[2]), y2 = Number(curve[3]);
    if (p === 0 || p === 1)
        return p;
    var lo = 0, hi = 1, t = p;
    for (var i = 0; i < 24; i++) {
        var value = cubic(x1, x2, t) - p;
        if (Math.abs(value) < 1e-5)
            break;
        if (value > 0)
            hi = t;
        else
            lo = t;
        t = (lo + hi) / 2;
    }
    return cubic(y1, y2, t);
}

// Row `index`'s eased progress, 0 hidden to 1 settled, `elapsed` ms after the
// ripple began.
function rippleProgress(elapsed, index, stepMs, durationMs, curve) {
    var local = (Number(elapsed) || 0) - Math.max(0, index) * stepMs;
    if (durationMs <= 0)
        return local >= 0 ? 1 : 0;
    return ease(curve, local / durationMs);
}

function rippleSpan(rows, stepMs, durationMs) {
    return Math.max(0, rows - 1) * stepMs + durationMs;
}

if (typeof module !== "undefined" && module.exports) {
    module.exports = { ease: ease, rippleProgress: rippleProgress, rippleSpan: rippleSpan };
}
