.pragma library

// How each notification look stacks its cards, in logical px at a UI scale of 1:
// the card width, the gap between cards, the breathing room off the screen edge,
// the room left round the card for its shadow (bleed), and how the cards below a
// new or leaving one settle into place. The cards themselves own their arrival
// and exit; the column reads these before any card exists, to size its window.
var looks = {
    "lithic": { width: 360, spacing: 10, inset: 12, bleed: 30, settleMs: 240, settleCurve: [0.2, 0, 0, 1, 1, 1] },
    "material": { width: 360, spacing: 8, inset: 12, bleed: 24, settleMs: 250, settleCurve: [0.34, 1.32, 0.64, 1, 1, 1] },
    "adwaita": { width: 380, spacing: 10, inset: 12, bleed: 30, settleMs: 250, settleCurve: [0.25, 0.46, 0.45, 0.94, 1, 1] },
    "fluent": { width: 364, spacing: 12, inset: 12, bleed: 30, settleMs: 267, settleCurve: [0, 0, 0, 1, 1, 1] }
};

function of(design) {
    return looks[design] || looks.lithic;
}

// how long a card arriving over `count - 1` others waits while they make room
function enterDelay(look, count) {
    return count > 1 ? Math.round(look.settleMs * 0.4) : 0;
}
