// The pinned Voice overlay geometry. The compact panel and the card that draws
// it must agree on the same numbers, so both read them here.

var PAD = 12;
var HEADER = 40;
var GAP = 10;
var SPACE = 9;
var NAME = 15;
var FLOOR = 344;
var CAP = 720;
var AVATAR_MIN = 32;
var AVATAR_MAX = 72;
var CELL_MIN = 76;

function avatar(value) {
    var size = typeof value === "number" ? value : 52;
    return Math.max(AVATAR_MIN, Math.min(AVATAR_MAX, size));
}

function cell(avatarSize, names) {
    return Math.max(avatarSize, names ? CELL_MIN : avatarSize + 8);
}

function columns(width, count, avatarSize, names) {
    if (count <= 0)
        return 1;
    var size = cell(avatarSize, names);
    var design = Math.floor((CAP + SPACE) / (size + SPACE));
    var frame = Math.floor((width - PAD * 2 + SPACE) / (size + SPACE));
    return Math.max(1, Math.min(8, count, design, frame));
}

function size(fullWidth, count, avatarSize, names) {
    var shown = Math.max(0, Math.min(8, count));
    var cols = columns(fullWidth, shown, avatarSize, names);
    var rows = shown > 0 ? Math.ceil(shown / cols) : 0;
    var grid = cols * cell(avatarSize, names) + (cols - 1) * SPACE;
    var height = PAD * 2 + HEADER;
    if (rows > 0)
        height += GAP + rows * (avatarSize + (names ? NAME : 0)) + (rows - 1) * SPACE;
    return {
        w: Math.min(fullWidth, Math.max(FLOOR, grid + PAD * 2)),
        h: height
    };
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { PAD, HEADER, GAP, SPACE, NAME, avatar, cell, columns, size };
