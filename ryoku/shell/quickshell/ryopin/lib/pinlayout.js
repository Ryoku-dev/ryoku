var MAX_W_SHARE = 1 / 3;
var MAX_H_SHARE = 0.5;
var GAP = 10;
var MARGIN = 14;

/**
 * Scales a natural image size down to fit within a third of the screen width and
 * half its height, preserving aspect ratio. Never scales up, so a small capture
 * pins at its true size. Returns integral dimensions with a floor of 48 so a
 * sliver capture stays grabbable.
 */
function fitSize(natW, natH, screenW, screenH) {
    var maxW = screenW * MAX_W_SHARE;
    var maxH = screenH * MAX_H_SHARE;
    var scale = Math.min(1, maxW / natW, maxH / natH);
    var w = Math.max(48, Math.floor(natW * scale));
    var h = Math.max(48, Math.floor(natH * scale));
    return { w: w, h: h };
}

/**
 * Places pin `index` in a corner stack that grows up from the bottom-right, then
 * leftward into a fresh column when the current one is full. `size` is the fitted
 * pin size.
 */
function slotPosition(index, size, screenW, screenH) {
    var rowH = size.h + GAP;
    var rows = Math.max(1, Math.floor((screenH - 2 * MARGIN) / rowH));
    var col = Math.floor(index / rows);
    var row = index % rows;
    var x = screenW - MARGIN - size.w - col * (size.w + GAP);
    var y = screenH - MARGIN - size.h - row * rowH;
    return { x: x, y: y };
}

/**
 * Keeps a pin fully on screen, biasing to the top-left when it is oversized.
 * `inset` holds the card that far inside the output: a pin surface carries a
 * transparent gutter for its shadow, and without the inset the compositor
 * clips the far edge of that gutter.
 */
function clamp(pos, size, screenW, screenH, inset) {
    var pad = inset || 0;
    var x = Math.max(pad, Math.min(pos.x, Math.max(pad, screenW - pad - size.w)));
    var y = Math.max(pad, Math.min(pos.y, Math.max(pad, screenH - pad - size.h)));
    return { x: x, y: y };
}

function outputFor(placement, screens, focusedOutput) {
    var names = [];
    for (var i = 0; i < screens.length; ++i) names.push(screens[i].name);
    if (placement && names.indexOf(placement.output) >= 0) return placement.output;
    if (placement && !placement.output && validPosition(placement)) {
        for (var j = 0; j < screens.length; ++j) {
            var screen = screens[j];
            if (placement.x >= screen.x && placement.x < screen.x + screen.width
                    && placement.y >= screen.y && placement.y < screen.y + screen.height)
                return screen.name;
        }
    }
    if (names.indexOf(focusedOutput) >= 0) return focusedOutput;
    return names.length ? names[0] : "";
}

function validPosition(placement) {
    return placement && typeof placement.x === "number" && isFinite(placement.x)
        && typeof placement.y === "number" && isFinite(placement.y);
}

function initialPosition(placement, index, size, output) {
    if (validPosition(placement) && (!placement.output || placement.output === output.name)) {
        return { x: placement.x - output.x, y: placement.y - output.y };
    }
    return clamp(slotPosition(index || 0, size, output.width, output.height),
        size, output.width, output.height);
}

function zoom(geometry, factor, screenW, screenH) {
    var minScale = Math.max(16 / geometry.w, 16 / geometry.h);
    var maxScale = Math.min(screenW / geometry.w, screenH / geometry.h);
    var scale = Math.max(Math.min(minScale, maxScale), Math.min(factor, maxScale));
    var w = Math.max(1, Math.round(geometry.w * scale));
    var h = Math.max(1, Math.round(geometry.h * scale));
    var pos = clamp({ x: geometry.x + (geometry.w - w) / 2,
        y: geometry.y + (geometry.h - h) / 2 }, { w: w, h: h }, screenW, screenH);
    return { x: pos.x, y: pos.y, w: w, h: h };
}

if (typeof module !== "undefined" && module.exports) {
    module.exports = { MAX_W_SHARE, MAX_H_SHARE, GAP, MARGIN, fitSize, slotPosition, clamp,
        outputFor, initialPosition, zoom };
}
