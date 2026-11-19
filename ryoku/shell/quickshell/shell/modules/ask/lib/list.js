// Geometry and movement for a sectioned result list. Rows have fixed heights by
// size, so every position is known before anything is drawn: the selection plate
// can glide to a row that was never laid out, and a click finds its row by y.

var SIZES = { one: 40, two: 52, task: 58, calc: 60, answer: 86, note: 44, menu: 36, map: 50 };

function rowHeight(row) {
    return SIZES[row && row.size] || SIZES.one;
}

// build lays sections {id, title, rows:[{id, size}]} out top to bottom.
// metrics: {scale, head (titled section head), gap (untitled), top, bottom, boxed
// (each section's rows sit in one box, padded by boxPad)}.
// Returns {heads:[{y, h, section}], rows:[{y, h, row, section, index}],
// boxes:[{y, h}], height}.
function build(sections, metrics) {
    var m = metrics || {};
    var k = m.scale || 1;
    var head = (m.head === undefined ? 30 : m.head) * k;
    var gap = (m.gap === undefined ? 8 : m.gap) * k;
    var boxPad = m.boxed ? (m.boxPad === undefined ? 4 : m.boxPad) * k : 0;
    var y = (m.top === undefined ? 6 : m.top) * k;
    var heads = [];
    var rows = [];
    var boxes = [];
    var list = sections || [];
    for (var i = 0; i < list.length; i++) {
        var section = list[i];
        var members = section.rows || [];
        if (members.length === 0)
            continue;
        if (String(section.title || "").length > 0) {
            heads.push({ y: Math.round(y), h: Math.round(head), section: section });
            y += head;
        } else if (rows.length > 0) {
            y += gap;
        }
        var boxTop = y;
        y += boxPad;
        for (var r = 0; r < members.length; r++) {
            var h = rowHeight(members[r]) * k;
            rows.push({ y: Math.round(y), h: Math.round(h), row: members[r], section: i, index: rows.length, last: r === members.length - 1 });
            y += h;
        }
        y += boxPad;
        if (m.boxed)
            boxes.push({ y: Math.round(boxTop), h: Math.round(y - boxTop) });
    }
    if (rows.length === 0)
        return { heads: [], rows: [], boxes: [], height: 0 };
    y += (m.bottom === undefined ? 8 : m.bottom) * k;
    return { heads: heads, rows: rows, boxes: boxes, height: Math.round(y) };
}

function indexOf(rows, id) {
    for (var i = 0; i < rows.length; i++) {
        if (String(rows[i].row.id) === String(id))
            return i;
    }
    return -1;
}

function selectable(rows, index) {
    return index >= 0 && index < rows.length && rows[index].row.disabled !== true;
}

// move steps the selection, skipping rows that cannot be chosen, and stops at the ends.
function move(rows, index, delta) {
    if (rows.length === 0)
        return -1;
    var step = delta < 0 ? -1 : 1;
    var left = Math.abs(delta);
    var at = index;
    if (at < 0 || at >= rows.length) {
        at = step > 0 ? -1 : rows.length;
        left = Math.max(1, left);
    }
    var last = selectable(rows, at) ? at : -1;
    while (left > 0) {
        var next = at + step;
        while (next >= 0 && next < rows.length && !selectable(rows, next))
            next += step;
        if (next < 0 || next >= rows.length)
            break;
        at = next;
        last = at;
        left--;
    }
    return last >= 0 ? last : first(rows);
}

function first(rows) {
    for (var i = 0; i < rows.length; i++) {
        if (selectable(rows, i))
            return i;
    }
    return -1;
}

// jump goes to the first row of the next section, or back to the start of this
// one and then the one before.
function jump(rows, index, delta) {
    if (rows.length === 0)
        return -1;
    if (index < 0 || index >= rows.length)
        return first(rows);
    var section = rows[index].section;
    var i;
    if (delta > 0) {
        for (i = index + 1; i < rows.length; i++) {
            if (rows[i].section !== section && selectable(rows, i))
                return i;
        }
        return index;
    }
    var start = index;
    while (start > 0 && rows[start - 1].section === section)
        start--;
    if (start < index && selectable(rows, start))
        return start;
    if (start === 0)
        return index;
    var before = rows[start - 1].section;
    i = start - 1;
    while (i > 0 && rows[i - 1].section === before)
        i--;
    return selectable(rows, i) ? i : index;
}

// page moves by about one viewport of rows.
function page(rows, index, delta, viewport) {
    if (rows.length === 0)
        return -1;
    var at = index < 0 ? first(rows) : index;
    if (at < 0)
        return -1;
    var target = rows[at].y + (delta > 0 ? 1 : -1) * Math.max(1, viewport - rows[at].h);
    var best = at;
    for (var i = at; i >= 0 && i < rows.length; i += delta > 0 ? 1 : -1) {
        if (delta > 0 ? rows[i].y > target : rows[i].y < target)
            break;
        if (selectable(rows, i))
            best = i;
    }
    return best;
}

// hit: the row under y, or -1 over a head or a gap.
function hit(rows, y) {
    var lo = 0;
    var hi = rows.length - 1;
    while (lo <= hi) {
        var mid = (lo + hi) >> 1;
        if (y < rows[mid].y)
            hi = mid - 1;
        else if (y >= rows[mid].y + rows[mid].h)
            lo = mid + 1;
        else
            return mid;
    }
    return -1;
}

// reveal: the scroll offset that brings a row fully into a viewport, keeping a
// section's head in sight when its first row is the target.
function reveal(rows, index, offset, viewport, height, headRoom) {
    if (index < 0 || index >= rows.length)
        return offset;
    var entry = rows[index];
    var top = entry.y;
    if (index === 0 || rows[index - 1].section !== entry.section)
        top -= headRoom || 0;
    var next = offset;
    if (top < next)
        next = top;
    if (entry.y + entry.h > next + viewport)
        next = entry.y + entry.h - viewport;
    return Math.max(0, Math.min(Math.max(0, height - viewport), next));
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { SIZES: SIZES, rowHeight: rowHeight, build: build, indexOf: indexOf, move: move, first: first, jump: jump, page: page, hit: hit, reveal: reveal };
