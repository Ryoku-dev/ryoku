// Where the annotation dock and its popovers stand on one output, in that
// output's logical px. The dock sits under the selection, else over it, else
// inside its bottom edge, and is always whole on the output. A popover opens
// away from the selection, so it covers the shot only when nothing else fits.

function clamp(v, lo, hi) {
    return hi < lo ? lo : Math.max(lo, Math.min(v, hi));
}

// sel {x,y,w,h}; size {w,h}; area {w,h};
// opts { align: "left"|"center", gap, gapAbove, inset, margin, dx, dy }
function dock(sel, size, area, opts) {
    var o = opts || {};
    var gap = o.gap === undefined ? 12 : o.gap;
    var gapAbove = o.gapAbove === undefined ? gap : o.gapAbove;
    var inset = o.inset === undefined ? 12 : o.inset;
    var margin = o.margin === undefined ? 8 : o.margin;
    var side, y;
    if (sel.y + sel.h + gap + size.h <= area.h - margin) {
        side = "below";
        y = sel.y + sel.h + gap;
    } else if (sel.y - gapAbove - size.h >= margin) {
        side = "above";
        y = sel.y - gapAbove - size.h;
    } else {
        side = "inside";
        y = sel.y + sel.h - inset - size.h;
    }
    var x = o.align === "left" ? sel.x : sel.x + sel.w / 2 - size.w / 2;
    return {
        x: clamp(x + (o.dx || 0), margin, area.w - size.w - margin),
        y: clamp(y + (o.dy || 0), margin, area.h - size.h - margin),
        side: side
    };
}

// anchor {x,w} is the opening button, edge {top,bottom} the dock;
// prefer "down" or "up"; opts { align: "start"|"center", gap, margin }
function popover(anchor, edge, size, area, prefer, opts) {
    var o = opts || {};
    var gap = o.gap === undefined ? 6 : o.gap;
    var margin = o.margin === undefined ? 8 : o.margin;
    var down = edge.bottom + gap;
    var up = edge.top - gap - size.h;
    var fitsDown = down + size.h <= area.h - margin;
    var fitsUp = up >= margin;
    var opensDown = prefer === "down" ? (fitsDown || !fitsUp) : (!fitsUp && fitsDown);
    var x = o.align === "start" ? anchor.x : anchor.x + anchor.w / 2 - size.w / 2;
    return {
        x: clamp(x, margin, area.w - size.w - margin),
        y: clamp(opensDown ? down : up, margin, area.h - size.h - margin),
        side: opensDown ? "down" : "up"
    };
}

if (typeof module !== "undefined" && module.exports) {
    module.exports = { clamp, dock, popover };
}
