/**
 * The corner a drag or a resize is moving, with the outward direction from the
 * selection at that corner. sel and press are global {x,y,w,h} and {x,y};
 * role is a resize grip role ("tl", "t", ...) or "" for a fresh drag from press.
 * An edge grip moves its midpoint and counts as facing right or down.
 */
function movingCorner(sel, press, role) {
    if (!sel) return null;
    var x0 = sel.x, y0 = sel.y, x1 = sel.x + sel.w, y1 = sel.y + sel.h;
    if (role) {
        var l = role.indexOf("l") >= 0, r = role.indexOf("r") >= 0;
        var t = role.indexOf("t") >= 0, b = role.indexOf("b") >= 0;
        return {
            x: l ? x0 : r ? x1 : (x0 + x1) / 2,
            y: t ? y0 : b ? y1 : (y0 + y1) / 2,
            dx: l ? -1 : 1,
            dy: t ? -1 : 1
        };
    }
    if (!press) return { x: x1, y: y1, dx: 1, dy: 1 };
    var left = Math.abs(press.x - x1) < 0.5 && sel.w > 0;
    var top = Math.abs(press.y - y1) < 0.5 && sel.h > 0;
    return { x: left ? x0 : x1, y: top ? y0 : y1, dx: left ? -1 : 1, dy: top ? -1 : 1 };
}

function fits(r, bounds, margin) {
    return r.x >= margin && r.y >= margin
        && r.x + r.w <= bounds.w - margin && r.y + r.h <= bounds.h - margin;
}

function clampInto(r, bounds, margin) {
    return {
        x: Math.max(margin, Math.min(r.x, bounds.w - margin - r.w)),
        y: Math.max(margin, Math.min(r.y, bounds.h - margin - r.h))
    };
}

// The rect of height h that starts gap off the corner on vertical side s.
function off(cy, h, s, gap) { return s > 0 ? cy + gap : cy - gap - h; }

/**
 * Places a readout (size a) and a loupe (size b) beside corner c, both held
 * gap off the corner horizontally so neither ever covers the corner or the
 * cursor on it. They go on the selection's outward side when it fits, else the
 * inward one. Vertically the readout prefers the outward side and the loupe the
 * inward one, alone or not, so it runs beside the frame's edge rather than past
 * its end; near an edge they stack on whichever side has room. A zero-size
 * readout leaves the loupe on its own. Returns {readout:{x,y}, loupe:{x,y}}.
 */
function aroundCorner(c, dir, a, b, bounds, gap, stackGap, margin) {
    var hasA = a.w > 0 && a.h > 0;
    var w = Math.max(hasA ? a.w : 0, b.w);
    var hx = dir.dx >= 0 ? 1 : -1;
    var vy = dir.dy >= 0 ? 1 : -1;

    function column(side, arrangement) {
        var left = side > 0 ? c.x + gap : c.x - gap - w;
        var ax = side > 0 ? left : left + w - a.w;
        var bx = side > 0 ? left : left + w - b.w;
        if (!hasA)
            return { a: null, b: { x: bx, y: off(c.y, b.h, arrangement === 0 ? -vy : vy, gap), w: b.w, h: b.h } };
        if (arrangement === 0)
            return {
                a: { x: ax, y: off(c.y, a.h, vy, gap), w: a.w, h: a.h },
                b: { x: bx, y: off(c.y, b.h, -vy, gap), w: b.w, h: b.h }
            };
        var s = arrangement === 1 ? -vy : vy;
        var ra = { x: ax, y: off(c.y, a.h, s, gap), w: a.w, h: a.h };
        return { a: ra, b: { x: bx, y: s > 0 ? ra.y + a.h + stackGap : ra.y - stackGap - b.h, w: b.w, h: b.h } };
    }

    var sides = [hx, -hx];
    var arrangements = hasA ? [0, 1, 2] : [0, 1];
    for (var i = 0; i < sides.length; i++) {
        for (var j = 0; j < arrangements.length; j++) {
            var col = column(sides[i], arrangements[j]);
            if ((!col.a || fits(col.a, bounds, margin)) && fits(col.b, bounds, margin))
                return { readout: col.a ? { x: col.a.x, y: col.a.y } : null, loupe: { x: col.b.x, y: col.b.y } };
        }
    }
    var last = column(hx, 0);
    return { readout: last.a ? clampInto(last.a, bounds, margin) : null, loupe: clampInto(last.b, bounds, margin) };
}

/**
 * Places a box of size s beside point p, off the cursor: below-right by gap,
 * flipped left or above when it would leave bounds (inset by margin).
 */
function besidePoint(p, s, bounds, gap, margin) {
    var x = p.x + gap, y = p.y + gap;
    if (x + s.w > bounds.w - margin) x = p.x - gap - s.w;
    if (y + s.h > bounds.h - margin) y = p.y - gap - s.h;
    return clampInto({ x: x, y: y, w: s.w, h: s.h }, bounds, margin);
}

if (typeof module !== "undefined" && module.exports) {
    module.exports = { movingCorner: movingCorner, aroundCorner: aroundCorner, besidePoint: besidePoint };
}
