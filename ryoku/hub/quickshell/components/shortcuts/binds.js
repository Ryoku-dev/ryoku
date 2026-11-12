.pragma library
.import "../../Combos.js" as Combos

// the shortcuts page's chord logic: the store writes "SUPER + SHIFT + Return",
// ChordField writes "Super+Shift+Return"; both meet in Combos.normKeys

var storeMods = ["SUPER", "CTRL", "ALT", "SHIFT"];
var fieldMods = ["Super", "Ctrl", "Alt", "Shift"];
var aliases = { "CONTROL": "CTRL", "MOD4": "SUPER", "META": "SUPER", "LOGO": "SUPER" };

function tokens(chord) {
    var out = [];
    var parts = String(chord || "").split("+");
    for (var i = 0; i < parts.length; i++) {
        var t = parts[i].trim();
        if (t.length)
            out.push(t);
    }
    return out;
}

function spell(chord, names, joiner) {
    var mods = [], rest = [];
    var toks = tokens(chord);
    for (var i = 0; i < toks.length; i++) {
        var up = toks[i].toUpperCase();
        if (aliases[up])
            up = aliases[up];
        var at = storeMods.indexOf(up);
        if (at >= 0) {
            if (mods.indexOf(at) < 0)
                mods.push(at);
        } else {
            rest.push(toks[i]);
        }
    }
    mods.sort(function (a, b) { return a - b; });
    return mods.map(function (m) { return names[m]; }).concat(rest).join(joiner);
}

function toStore(chord) {
    return spell(chord, storeMods, " + ");
}

function toField(chord) {
    return spell(chord, fieldMods, "+");
}

function norm(chord) {
    return Combos.normKeys(toStore(chord));
}

function isFamily(chord) {
    return Combos.isFamilyChord(chord);
}

// a recorded "Super+Ctrl+3" as the family form "SUPER + CTRL + {n}", or ""
function familyFrom(chord) {
    return Combos.familyFrom(toStore(chord));
}

function effective(rebinds, def) {
    return Combos.effectiveCombo(rebinds || {}, def);
}

function isRebound(rebinds, def) {
    var r = (rebinds || {})[def];
    return !!r && r.length > 0 && norm(r) !== norm(def);
}

function isOff(disabled, def) {
    return (disabled || []).indexOf(def) >= 0;
}

function withRebind(rebinds, def, chord) {
    var m = Object.assign({}, rebinds || {});
    if (!chord || norm(chord) === norm(def))
        delete m[def];
    else
        m[def] = chord;
    return m;
}

function withOff(disabled, def, off) {
    var next = (disabled || []).filter(function (c) { return c !== def; });
    if (off)
        next.push(def);
    return next;
}

// every chord in use: norm -> [{ kind: "bind", id, label } | { kind: "custom", index }]
function holders(binds, rebinds, disabled, custom) {
    var m = {};
    function add(chord, holder) {
        var chords = Combos.expandFamily(chord);
        for (var i = 0; i < chords.length; i++) {
            var n = norm(chords[i]);
            if (n.length)
                (m[n] = m[n] || []).push(holder);
        }
    }
    for (var b = 0; b < (binds || []).length; b++) {
        var bind = binds[b];
        if (bind.unhonored || isOff(disabled, bind.combo))
            continue;
        add(effective(rebinds, bind.combo), { kind: "bind", id: bind.id, label: bind.label });
    }
    for (var c = 0; c < (custom || []).length; c++)
        add((custom[c] || {}).keys || "", { kind: "custom", index: c });
    return m;
}

function sameHolder(a, b) {
    if (!a || !b || a.kind !== b.kind)
        return false;
    return a.kind === "bind" ? a.id === b.id : a.index === b.index;
}

function clash(map, candidate, self) {
    var chords = Combos.expandFamily(toStore(candidate));
    for (var i = 0; i < chords.length; i++) {
        var hs = map[norm(chords[i])] || [];
        for (var h = 0; h < hs.length; h++)
            if (!sameHolder(hs[h], self))
                return hs[h];
    }
    return null;
}

function shown(chord, faces) {
    var toks = tokens(toField(chord));
    for (var i = 0; i < toks.length; i++)
        if (faces[toks[i]] !== undefined)
            toks[i] = faces[toks[i]];
    return toks.join("+");
}

// a family stands for its digits, so "workspace 3" finds it
function familyWords(chord) {
    if (String(chord || "").indexOf("{n}") < 0)
        return "";
    var kp = chord.indexOf("KP_{n}") >= 0;
    var out = [];
    for (var n = 1; n <= 10; n++)
        out.push((kp ? "Num " : "") + (n % 10));
    return out.join(" ");
}

function matches(query, text) {
    var words = String(query || "").toLowerCase().split(/\s+/).filter(function (w) { return w.length > 0; });
    var hay = String(text || "").toLowerCase();
    for (var i = 0; i < words.length; i++)
        if (hay.indexOf(words[i]) < 0)
            return false;
    return true;
}
