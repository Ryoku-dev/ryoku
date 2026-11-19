// Kit's sections in rail order, and stepping through them and through a
// section's own pages. Labels are QML's (translated); this only knows ids.

var ORDER = ["downloads", "keys", "commands", "phone", "devtools", "typing"];
// Super+K is the shortcut legend's key, so a first open with no memory lands there
var FIRST = "keys";
var GLYPHS = {
    downloads: "download", keys: "keyboard", commands: "terminal",
    phone: "smartphone", devtools: "build", typing: "speed"
};

function has(id) {
    return ORDER.indexOf(String(id || "")) >= 0;
}

function index(id) {
    return ORDER.indexOf(String(id || ""));
}

function at(position) {
    return position >= 0 && position < ORDER.length ? ORDER[position] : "";
}

// step goes round the rail: past the last section comes the first.
function step(id, delta) {
    var from = index(id);
    if (from < 0)
        return FIRST;
    var count = ORDER.length;
    return ORDER[(((from + delta) % count) + count) % count];
}

function pageIndex(pages, id) {
    var list = pages || [];
    for (var i = 0; i < list.length; i++) {
        if (String(list[i].id) === String(id))
            return i;
    }
    return -1;
}

// stepPage stops at the ends: a strip is read left to right, it is not a ring.
// An unknown current page starts from the first.
function stepPage(pages, id, delta) {
    var list = pages || [];
    if (list.length === 0)
        return "";
    var from = pageIndex(list, id);
    if (from < 0)
        return String(list[0].id);
    var to = Math.max(0, Math.min(list.length - 1, from + (delta < 0 ? -1 : 1)));
    return String(list[to].id);
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { ORDER: ORDER, FIRST: FIRST, GLYPHS: GLYPHS, has: has, index: index, at: at, step: step, pageIndex: pageIndex, stepPage: stepPage };
