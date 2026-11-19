// Kit's memory: one flat JSON map in ~/.local/state/ryoku/kit.json, keys
// "<section>.<name>". Every key a reader already has keeps its meaning:
//   frame.tab                       the section Kit reopens on
//   commands.rail, phone.rail       the page those sections were on
//   devtools.tool, devtools.options.<tool>
//   typing.*                        settings, history.results, history.personalBests
// New keys: <section>.count (what a section last counted, for its tab's figure).

var SECTION_KEY = "frame.tab";

// parse never throws: a missing, empty or broken file is an empty memory.
function parse(text) {
    var raw = String(text || "");
    if (raw.trim().length === 0)
        return {};
    try {
        var value = JSON.parse(raw);
        if (value && typeof value === "object" && !Array.isArray(value))
            return value;
    } catch (error) {
        return {};
    }
    return {};
}

function key(section, name) {
    return String(section) + "." + String(name);
}

function get(values, name, fallback) {
    return values && Object.prototype.hasOwnProperty.call(values, name) ? values[name] : fallback;
}

// set answers a new map, or the same one when nothing would change (so a
// section that re-states what is stored does not rewrite the file).
function set(values, name, value) {
    var current = values || {};
    if (Object.prototype.hasOwnProperty.call(current, name) && JSON.stringify(current[name]) === JSON.stringify(value))
        return current;
    var next = {};
    for (var k in current) {
        if (Object.prototype.hasOwnProperty.call(current, k))
            next[k] = current[k];
    }
    next[name] = value;
    return next;
}

// startSection: where Kit opens with no route. frame.tab is the old frame's
// key; it now also names Downloads.
function startSection(values, order, first) {
    var saved = String(get(values, SECTION_KEY, ""));
    return order.indexOf(saved) >= 0 ? saved : first;
}

function count(values, section) {
    var n = Number(get(values, key(section, "count"), 0));
    return isFinite(n) && n > 0 ? Math.round(n) : 0;
}

// typingBest: the fastest personal best on record, in whole words per minute.
function typingBest(values) {
    var bests = get(values, "typing.history.personalBests", []);
    if (!Array.isArray(bests))
        return 0;
    var top = 0;
    for (var i = 0; i < bests.length; i++) {
        var wpm = Number(bests[i] && bests[i].wpm);
        if (isFinite(wpm) && wpm > top)
            top = wpm;
    }
    return Math.round(top);
}

function typingTests(values) {
    var results = get(values, "typing.history.results", []);
    return Array.isArray(results) ? results.length : 0;
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { SECTION_KEY: SECTION_KEY, parse: parse, key: key, get: get, set: set, startSection: startSection, count: count, typingBest: typingBest, typingTests: typingTests };
