// Chords as the bar writes them in an action's hint ("Ctrl+Shift+C"), matched
// against a key event and split into the caps the footer and the panel draw.

var CTRL = 0x04000000;
var SHIFT = 0x02000000;
var ALT = 0x08000000;

var NAMED = {
    0x01000000: "Esc", 0x01000001: "Tab", 0x01000002: "Tab", 0x01000003: "Backspace",
    0x01000004: "Enter", 0x01000005: "Enter", 0x01000007: "Delete",
    0x01000012: "Left", 0x01000013: "Up", 0x01000014: "Right", 0x01000015: "Down",
    0x01000016: "PageUp", 0x01000017: "PageDown", 0x20: "Space",
    0x2e: ".", 0x2c: ",", 0x2f: "/", 0x5c: "\\", 0x5b: "[", 0x5d: "]"
};

function keyName(key) {
    if (NAMED[key])
        return NAMED[key];
    if ((key >= 0x30 && key <= 0x39) || (key >= 0x41 && key <= 0x5a))
        return String.fromCharCode(key);
    return "";
}

// chord names an event the way hints are written, modifiers in a fixed order.
function chord(key, modifiers) {
    var name = keyName(key);
    if (name.length === 0)
        return "";
    var parts = [];
    if (modifiers & CTRL)
        parts.push("Ctrl");
    if (modifiers & ALT)
        parts.push("Alt");
    // Backtab already is Shift+Tab
    if ((modifiers & SHIFT) || key === 0x01000002)
        parts.push("Shift");
    parts.push(name);
    return parts.join("+");
}

function normalize(hint) {
    var raw = String(hint || "").split("+").map(function(part) { return part.trim(); }).filter(function(part) { return part.length > 0; });
    if (raw.length === 0)
        return "";
    var key = raw[raw.length - 1];
    var mods = raw.slice(0, -1).map(function(part) { return part.toLowerCase(); });
    var parts = [];
    if (mods.indexOf("ctrl") >= 0)
        parts.push("Ctrl");
    if (mods.indexOf("alt") >= 0)
        parts.push("Alt");
    if (mods.indexOf("shift") >= 0)
        parts.push("Shift");
    parts.push(key.length === 1 ? key.toUpperCase() : key);
    return parts.join("+");
}

// caps: the hint as separate key caps, in the order it is pressed.
function caps(hint) {
    var value = normalize(hint);
    return value.length > 0 ? value.split("+") : [];
}

// find: the first action whose hint is this chord.
function find(actions, key, modifiers) {
    var pressed = chord(key, modifiers);
    if (pressed.length === 0)
        return null;
    var list = actions || [];
    for (var i = 0; i < list.length; i++) {
        if (list[i] && list[i].hint && normalize(list[i].hint) === pressed)
            return list[i];
    }
    return null;
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { chord: chord, normalize: normalize, caps: caps, find: find, CTRL: CTRL, SHIFT: SHIFT, ALT: ALT };
