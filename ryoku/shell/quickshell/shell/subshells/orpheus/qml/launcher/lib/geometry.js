// Launcher sheet geometry in logical px: the width from the screen, the list
// block and the liquid level the vessel gives the sheet.

var MIN_WIDTH = 720;
var MAX_WIDTH = 880;
var SCREEN_SHARE = 0.44;
var MIN_ROWS = 4;
var MAX_ROWS = 9;
var LIST_SHARE = 0.57;

function clamp(value, lo, hi) {
    return Math.max(lo, Math.min(hi, value));
}

function launcherWidth(screenWidth) {
    var width = Math.round((Number(screenWidth) || 0) * SCREEN_SHARE);
    return clamp(width, MIN_WIDTH, MAX_WIDTH);
}

// Rows the list block shows: at least four so a short answer never collapses
// the sheet, at most nine before the list scrolls, never more than fit.
function visibleRows(count, rowHeight, room) {
    var fit = rowHeight > 0 ? Math.floor(Math.max(0, Number(room) || 0) / rowHeight) : 0;
    var wanted = Math.min(Math.max(Number(count) || 0, MIN_ROWS), MAX_ROWS);
    return Math.max(0, Math.min(wanted, fit));
}

// parts: { count, rowHeight, chrome, waveRoom, maxHeight, leadHeight }. The lead
// may lift the block up to nine rows so a short list never clips its actions.
function sheetLevel(parts) {
    var p = parts || {};
    var rowHeight = Number(p.rowHeight) || 0;
    var fixed = (Number(p.chrome) || 0) + (Number(p.waveRoom) || 0);
    var ceiling = Number(p.maxHeight) > 0 ? Number(p.maxHeight) : Infinity;
    var rows = visibleRows(p.count, rowHeight, ceiling - fixed);
    var blockCap = Math.min(MAX_ROWS * rowHeight, Math.max(0, ceiling - fixed));
    var lead = Math.min(blockCap, Math.max(0, Number(p.leadHeight) || 0));
    var listHeight = Math.max(rows * rowHeight, lead);
    return { rows: rows, listHeight: listHeight, level: Math.min(ceiling, fixed + listHeight) };
}

function columns(width, gap) {
    var room = Math.max(0, (Number(width) || 0) - (Number(gap) || 0));
    var list = Math.round(room * LIST_SHARE);
    return { list: list, lead: room - list };
}

// The bead stretches along its travel and settles back to one row.
function beadHeight(rowHeight, distance) {
    return rowHeight + Math.min(rowHeight * 0.5, Math.abs(Number(distance) || 0) * 0.25);
}

if (typeof module !== "undefined" && module.exports) {
    module.exports = {
        MIN_WIDTH: MIN_WIDTH,
        MAX_WIDTH: MAX_WIDTH,
        MAX_ROWS: MAX_ROWS,
        launcherWidth: launcherWidth,
        visibleRows: visibleRows,
        sheetLevel: sheetLevel,
        columns: columns,
        beadHeight: beadHeight
    };
}
