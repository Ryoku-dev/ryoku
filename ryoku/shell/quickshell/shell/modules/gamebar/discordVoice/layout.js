// The pinned Voice overlay geometry. The compact panel and the card that draws
// it must agree on the same numbers, so both read them here.

var PAD = 12;
var HEADER = 40;
var GAP = 10;
var SPACE = 9;
var NAME = 16;
var NAME_SIDE = 56;
var NAME_CHAR = 6;
var NAME_MAX = 210;
var FLOOR = 344;
var CAP = 1100;
var AVATAR_MIN = 32;
var AVATAR_MAX = 72;
var CELL_MIN = 76;

function avatar(value) {
    var size = typeof value === "number" ? value : 52;
    return Math.max(AVATAR_MIN, Math.min(AVATAR_MAX, size));
}

function beside(namePosition) {
    return namePosition === "left" || namePosition === "right";
}

function nameWidth(nameChars) {
    if (typeof nameChars !== "number")
        return NAME_SIDE;
    return Math.max(NAME_SIDE, Math.min(NAME_MAX, nameChars * NAME_CHAR));
}

function cell(avatarSize, names, namePosition, gap, nameChars) {
    var space = typeof gap === "number" ? gap : 0;
    if (!names)
        return avatarSize + 8;
    var text = nameWidth(nameChars);
    // A left-side name narrows to make the gap, so only a right-side name
    // needs extra width. The avatar stays put either way.
    var extra = namePosition === "right" ? space : 0;
    return beside(namePosition)
        ? Math.max(avatarSize + text + extra, CELL_MIN)
        : Math.max(avatarSize, text, CELL_MIN);
}

function rowHeight(avatarSize, names, namePosition, gap) {
    var space = typeof gap === "number" ? gap : 0;
    return avatarSize + (names && !beside(namePosition) ? NAME + space : 0);
}

// How many rows fit in a surface's available height: chrome + rows*rowHeight
// + (rows-1)*spacing must stay under available.
function maxRows(chrome, rowH, spacing, available) {
    var budget = available - chrome + spacing;
    if (budget < rowH)
        return 1;
    return Math.max(1, Math.floor(budget / (rowH + spacing)));
}

function columns(width, count, avatarSize, names, vertical, namePosition, gap, nameChars) {
    if (count <= 0 || vertical)
        return 1;
    var size = cell(avatarSize, names, namePosition, gap, nameChars);
    var design = Math.floor((CAP + SPACE) / (size + SPACE));
    var frame = Math.floor((width - PAD * 2 + SPACE) / (size + SPACE));
    return Math.max(1, Math.min(8, count, design, frame));
}

function size(fullWidth, count, avatarSize, names, vertical, namePosition, gap, nameChars) {
    var shown = Math.max(0, Math.min(8, count));
    var cols = columns(fullWidth, shown, avatarSize, names, vertical, namePosition, gap, nameChars);
    var rows = shown > 0 ? Math.ceil(shown / cols) : 0;
    var grid = cols * cell(avatarSize, names, namePosition, gap, nameChars) + (cols - 1) * SPACE;
    var height = PAD * 2 + HEADER;
    if (rows > 0)
        height += GAP + rows * rowHeight(avatarSize, names, namePosition, gap) + (rows - 1) * SPACE;
    return {
        w: Math.min(fullWidth, Math.max(FLOOR, grid + PAD * 2)),
        h: height
    };
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { PAD, HEADER, GAP, SPACE, NAME, NAME_SIDE, NAME_CHAR, NAME_MAX, avatar, beside, nameWidth, cell, rowHeight, maxRows, columns, size };
