// Kit's keyboard map (kit-contract section 3). Chords are written the way the
// bar writes them ("Ctrl+Shift+Tab", see ask/lib/keys.js chord()); nothing here
// touches Qt, so the whole map runs under node.

// The chords the frame owns wherever the keyboard is, a text pane included.
// sequences are Qt's spellings, for the window shortcuts that carry them.
var GLOBAL = [
    { op: "section", arg: 0, chord: "Ctrl+1", sequences: ["Ctrl+1"] },
    { op: "section", arg: 1, chord: "Ctrl+2", sequences: ["Ctrl+2"] },
    { op: "section", arg: 2, chord: "Ctrl+3", sequences: ["Ctrl+3"] },
    { op: "section", arg: 3, chord: "Ctrl+4", sequences: ["Ctrl+4"] },
    { op: "section", arg: 4, chord: "Ctrl+5", sequences: ["Ctrl+5"] },
    { op: "section", arg: 5, chord: "Ctrl+6", sequences: ["Ctrl+6"] },
    { op: "sectionStep", arg: 1, chord: "Ctrl+Tab", sequences: ["Ctrl+Tab"] },
    { op: "sectionStep", arg: -1, chord: "Ctrl+Shift+Tab", sequences: ["Ctrl+Shift+Tab"] },
    { op: "pageStep", arg: 1, chord: "Ctrl+PageDown", sequences: ["Ctrl+PgDown"] },
    { op: "pageStep", arg: -1, chord: "Ctrl+PageUp", sequences: ["Ctrl+PgUp"] },
    { op: "all", arg: 0, chord: "Ctrl+F", sequences: ["Ctrl+F"] },
    { op: "panel", arg: 0, chord: "Ctrl+K", sequences: ["Ctrl+K"] }
];

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

function global(chord) {
    for (var i = 0; i < GLOBAL.length; i++) {
        if (GLOBAL[i].chord === chord)
            return GLOBAL[i];
    }
    return null;
}

function printable(text) {
    var value = String(text || "");
    if (value.length === 0)
        return false;
    var code = value.charCodeAt(0);
    return code >= 0x20 && code !== 0x7f;
}

// resolve: what a key that reached the frame does.
// chord is the event's name ("" when it has none), text what it would type.
// ctx: {panel, all, field (the field has the keyboard), textEntry (a text entry
// of the section has it), ownsText (the section takes all typing), primaryHint,
// secondaryHint, actions: [{id, hint}]}.
// Answers {op, arg} or null (leave the key to whoever has it):
//   escape | section n | sectionStep d | pageStep d | all | panel | activate |
//   secondary | move d | jump d | rows d | act id | type text
function resolve(chord, text, ctx) {
    var c = ctx || {};
    if (chord === "Esc")
        return { op: "escape", arg: 0 };
    var owned = global(chord);
    if (owned)
        return { op: owned.op, arg: owned.arg };
    // the Action Panel reads its own keys while it is up
    if (c.panel)
        return null;
    // a pane or a form is being typed in: its keys are its own
    if (c.textEntry)
        return null;
    // Enter and Ctrl+Enter are the two actions' keys unless the section's
    // hints name others (a pane keeps Enter for new lines, so its tool runs
    // on Ctrl+Enter)
    var primary = normalize(c.primaryHint) || "Enter";
    var secondary = normalize(c.secondaryHint) || "Ctrl+Enter";
    if (chord.length > 0 && chord === primary)
        return { op: "activate", arg: 0 };
    if (chord.length > 0 && chord === secondary)
        return { op: "secondary", arg: 0 };
    if (chord === "Down" || chord === "Up")
        return { op: "move", arg: chord === "Down" ? 1 : -1 };
    if (chord === "Ctrl+Down" || chord === "Ctrl+Up")
        return { op: "jump", arg: chord === "Ctrl+Down" ? 1 : -1 };
    if (chord === "PageDown" || chord === "PageUp")
        return { op: "rows", arg: chord === "PageDown" ? 1 : -1 };
    var list = chord.length > 0 ? (c.actions || []) : [];
    for (var i = 0; i < list.length; i++) {
        if (list[i] && list[i].hint && normalize(list[i].hint) === chord)
            return { op: "act", arg: String(list[i].id) };
    }
    // a letter typed with the keyboard somewhere idle belongs to the field
    if (!c.field && (c.all || !c.ownsText) && printable(text) && chord.indexOf("Ctrl+") !== 0 && chord.indexOf("Alt+") !== 0)
        return { op: "type", arg: String(text) };
    return null;
}

// escape: the layers Esc steps back through, nearest first. "section" asks the
// section (its handleEscape may decline), so what follows it stays in the list.
function escape(ctx) {
    var c = ctx || {};
    if (c.panel)
        return ["panel"];
    if (c.all)
        return ["all"];
    var steps = ["section"];
    if (String(c.fieldText || "").length > 0)
        steps.push("clear");
    steps.push("close");
    return steps;
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { GLOBAL: GLOBAL, normalize: normalize, global: global, printable: printable, resolve: resolve, escape: escape };
