.pragma library

// Pure key-combo helpers, shared so the Keybinds page and the Import page reason
// about shortcuts the same way, with one copy of the rules. Nothing here holds
// state or touches QML: normalisation and conflict classification are string
// work, and the chord reader turns a KeyEvent into the neutral token the store
// keeps. The engine owns the authoritative norm on the wire; the UI uses these
// only to compare and to record, never to invent a key.

// normalise a combo for compare: case, spacing and modifier order collapse, so
// "SUPER + Q", "super+q" and "Q + Super" are one key.
function normKeys(s) {
    if (!s)
        return "";
    var parts = ("" + s).split("+");
    var out = [];
    for (var i = 0; i < parts.length; i++) {
        var t = parts[i].trim().toLowerCase();
        if (t.length)
            out.push(t);
    }
    out.sort();
    return out.join("+");
}

// ── chord search ────────────────────────────────────────────────────────────
// The search field accepts a chord the way someone writes it, not only the
// sheet's display tokens: case, spaces, modifier order and the common aliases
// collapse, so "ctrl+l", "CTRL + L" and "Control+l" are one query. A query is
// read as a chord when it carries a "+" or opens with a modifier; a word alias
// like "windows" only counts once the query has a "+" or a second token, so
// bare "windows" still finds the Windows category.
var searchAliases = {
    "meta": "super", "win": "super", "windows": "super",
    "cmd": "super", "command": "super", "logo": "super",
    "control": "ctrl", "ctl": "ctrl",
    "option": "alt", "opt": "alt", "opcion": "alt", "opción": "alt",
    "mayus": "shift", "mayusculas": "shift", "mayúsculas": "shift",
    "enter": "return", "intro": "return",
    "esc": "escape",
    "spacebar": "space", "espacio": "space",
    "repag": "prior", "avpag": "next",
    "izquierda": "left", "derecha": "right", "arriba": "up", "abajo": "down",
    "inicio": "home", "fin": "end",
    "retroceso": "backspace", "supr": "delete", "suprimir": "delete",
    "tabulador": "tab",
    ",": "comma", "-": "minus", "=": "equal",
    "[": "bracketleft", "]": "bracketright", "`": "grave",
    "\\": "backslash", "/": "slash", ";": "semicolon",
    "'": "apostrophe", ".": "period"
};

// fold the multi-word spellings before splitting, so "page up" and "num 3"
// survive as one token.
function foldChordText(s) {
    return ("" + (s || "")).toLowerCase()
        .replace(/page\s*up/g, "prior")
        .replace(/page\s*down/g, "next")
        .replace(/\bpgup\b/g, "prior")
        .replace(/\bpgdn\b/g, "next")
        .replace(/\bnum(?:pad)?\s*([0-9])\b/g, "kp$1");
}

// a chord or query as canonical search tokens. Unknown tokens keep their own
// spelling, so an exotic key still compares literally.
function searchTokens(s) {
    var parts = foldChordText(s).split(/[\s+]+/);
    var out = [];
    for (var i = 0; i < parts.length; i++) {
        var t = parts[i].trim();
        if (!t)
            continue;
        if (t.indexOf("kp_") === 0)
            t = "kp" + t.substring(3);
        t = searchAliases[t] || t;
        out.push(t);
    }
    return out;
}

function isModifierToken(t) {
    return t === "super" || t === "ctrl" || t === "alt" || t === "shift";
}

// the aliases that are also everyday words ("windows", "win", "command",
// "option") only read as a modifier when a chord is clearly meant: a bare
// "windows" is someone looking for the Windows category, not pressing Super.
var literalModifiers = { "super": true, "ctrl": true, "alt": true, "shift": true };

// true when `query` is a chord the field should read as one.
function isChordQuery(query) {
    var s = ("" + (query || "")).trim();
    if (s.length === 0)
        return false;
    if (s.indexOf("+") >= 0)
        return true;
    var parts = foldChordText(s).split(/\s+/);
    if (parts.length > 1)
        return isModifierToken(searchAliases[parts[0]] || parts[0]);
    return literalModifiers[parts[0]] === true;
}

// does `chord` answer `query`? A query that names a key must match the whole
// chord, so "super l" does not find Super + Shift + L; a query of modifiers
// alone filters every chord that holds them, so "ctrl" lists the Ctrl family.
function chordMatches(query, chord) {
    var q = searchTokens(query);
    var c = searchTokens(chord);
    if (q.length === 0 || c.length === 0)
        return false;
    var modsOnly = true;
    for (var i = 0; i < q.length; i++)
        if (!isModifierToken(q[i])) {
            modsOnly = false;
            break;
        }
    if (!modsOnly && q.length !== c.length)
        return false;
    for (var j = 0; j < q.length; j++)
        if (c.indexOf(q[j]) < 0)
            return false;
    return true;
}

// the effective combo a shipped bind fires on: a rebind wins over the default.
function effectiveCombo(rebinds, defCombo) {
    var r = rebinds ? rebinds[defCombo] : undefined;
    return (r && r.length) ? r : defCombo;
}

// the set of normalised combos the shipped legend holds (rebinds applied), keyed
// for O(1) shadow lookups. `categories` is the `ryoku-hub keybinds` legend. A
// family stands for ten chords, so it is expanded before it lands in the set:
// that is how a custom "SUPER + 3" is caught shadowing the workspace family.
// The legend also carries the user's custom rows; those are not shipped, so they
// are skipped here or a custom chord would shadow itself. A switched-off bind
// holds no chord either, so it is skipped like it is when counting.
function shippedKeys(categories, rebinds, disabled) {
    var off = disabledSet(disabled);
    var set = {};
    categories = categories || [];
    for (var c = 0; c < categories.length; c++) {
        var binds = categories[c].binds || [];
        for (var b = 0; b < binds.length; b++) {
            if ((binds[b].kind || "") === "custom")
                continue;
            var def = binds[b].combo || "";
            if (off[def])
                continue;
            var chords = expandFamily(effectiveCombo(rebinds, def));
            for (var e = 0; e < chords.length; e++) {
                var k = normKeys(chords[e]);
                if (k.length)
                    set[k] = true;
            }
        }
    }
    return set;
}

// the disabled list as an object, for O(1) lookups.
function disabledSet(disabled) {
    var off = {};
    disabled = disabled || [];
    for (var i = 0; i < disabled.length; i++)
        if (disabled[i])
            off[disabled[i]] = true;
    return off;
}

// how many shortcuts hold each effective chord: the shipped legend (rebinds
// applied, families expanded) plus the custom rows. The legend carries the
// custom rows too, so they are skipped there and counted once from `customRows`
// or every custom chord would look like a clash with itself.
function effectiveCounts(categories, rebinds, customRows, disabled) {
    var off = disabledSet(disabled);
    var m = {};
    categories = categories || [];
    for (var c = 0; c < categories.length; c++) {
        var binds = categories[c].binds || [];
        for (var b = 0; b < binds.length; b++) {
            if ((binds[b].kind || "") === "custom")
                continue;
            var def = binds[b].combo || "";
            if (off[def])
                continue;
            var chords = expandFamily(effectiveCombo(rebinds, def));
            for (var e = 0; e < chords.length; e++) {
                var k = normKeys(chords[e]);
                if (k.length)
                    m[k] = (m[k] || 0) + 1;
            }
        }
    }
    customRows = customRows || [];
    for (var i = 0; i < customRows.length; i++) {
        var cn = normKeys(customRows[i].keys);
        if (cn.length)
            m[cn] = (m[cn] || 0) + 1;
    }
    return m;
}

// the normalised chords held by more than one shortcut. A rebind that lands on
// another bind and a custom row that shadows one both land here, which is what
// lets the page show both sides of a clash instead of only the flagged row.
function conflictKeys(categories, rebinds, customRows, disabled) {
    var counts = effectiveCounts(categories, rebinds, customRows, disabled);
    var m = {};
    for (var k in counts)
        if (counts[k] > 1)
            m[k] = true;
    return m;
}

// how many custom rows hold a given normalised combo.
function customCount(customRows, norm) {
    customRows = customRows || [];
    var n = 0;
    for (var i = 0; i < customRows.length; i++)
        if (normKeys(customRows[i].keys) === norm)
            n++;
    return n;
}

// classify custom row i: "" none, "shipped" shadows a Ryoku bind, "duplicate"
// repeats another custom one. `shipped` is a shippedKeys() set.
function rowConflict(customRows, i, shipped) {
    customRows = customRows || [];
    shipped = shipped || {};
    var k = normKeys((customRows[i] || {}).keys);
    if (!k)
        return "";
    if (shipped[k])
        return "shipped";
    return customCount(customRows, k) > 1 ? "duplicate" : "";
}

// the held modifiers of a KeyEvent, in the order a normalised chord carries
// them. The keypad modifier is never one of these: it flags which physical
// block a key came from, not a chord modifier, so it stays out of the combo
// (chordFrom reads it only to pick the KP_ keysym in qtKeyName).
function modTokens(event) {
    var mods = [];
    if (event.modifiers & Qt.MetaModifier) mods.push("SUPER");
    if (event.modifiers & Qt.ControlModifier) mods.push("CTRL");
    if (event.modifiers & Qt.AltModifier) mods.push("ALT");
    if (event.modifiers & Qt.ShiftModifier) mods.push("SHIFT");
    return mods;
}

// the modifiers held so far, as a combo string, for the recorder overlay to
// show the chord forming before the main key lands. "" when nothing is held.
function formingChord(event) {
    return modTokens(event).join(" + ");
}

// KeyEvent -> the token the store binds on. Covers letters, digits, the
// function row, navigation, the common punctuation and the whole number pad;
// anything unmapped returns "" so the recorder keeps waiting (and the field
// stays typeable for the exotic rest).
function qtKeyName(event) {
    var k = event.key;
    // Number pad. Qt tags every number-pad key with KeypadModifier, whichever
    // way NumLock sits: with it on the digits arrive as Key_0..Key_9, with it
    // off they arrive as the navigation keys (numpad 1 is Key_End, 5 is Clear).
    // Fold both faces to the one digit keysym KP_<d> the shipped number-pad
    // families carry, so a recorded numpad chord reads one canonical way whichever
    // way NumLock sits. The provider emits the NumLock-off twin (NumpadAliases in
    // wm/binds.go) beside it, so the chord fires either way and the cap reads
    // "Num n" rather than a raw KP_End.
    if (event.modifiers & Qt.KeypadModifier) {
        if (k >= Qt.Key_0 && k <= Qt.Key_9)
            return "KP_" + String.fromCharCode(k);
        switch (k) {
        case Qt.Key_End: return "KP_1";
        case Qt.Key_Down: return "KP_2";
        case Qt.Key_PageDown: return "KP_3";
        case Qt.Key_Left: return "KP_4";
        case Qt.Key_Clear: return "KP_5";
        case Qt.Key_Right: return "KP_6";
        case Qt.Key_Home: return "KP_7";
        case Qt.Key_Up: return "KP_8";
        case Qt.Key_PageUp: return "KP_9";
        case Qt.Key_Insert: return "KP_0";
        case Qt.Key_Delete: return "KP_Decimal";
        case Qt.Key_Enter: case Qt.Key_Return: return "KP_Enter";
        case Qt.Key_Plus: return "KP_Add";
        case Qt.Key_Minus: return "KP_Subtract";
        case Qt.Key_Asterisk: return "KP_Multiply";
        case Qt.Key_Slash: return "KP_Divide";
        case Qt.Key_Period: case Qt.Key_Comma: return "KP_Decimal";
        }
    }
    // number row by position: Shift+1 is Key_Exclam, binds still name the digit
    var code = event.nativeScanCode;
    if (code >= 10 && code <= 19)
        return "" + ((code - 9) % 10);
    if (k >= Qt.Key_A && k <= Qt.Key_Z)
        return String.fromCharCode(k);
    if (k >= Qt.Key_0 && k <= Qt.Key_9)
        return String.fromCharCode(k);
    if (k >= Qt.Key_F1 && k <= Qt.Key_F12)
        return "F" + (k - Qt.Key_F1 + 1);
    switch (k) {
    case Qt.Key_Return: case Qt.Key_Enter: return "Return";
    case Qt.Key_Space: return "Space";
    case Qt.Key_Tab: return "Tab";
    case Qt.Key_Left: return "Left";
    case Qt.Key_Right: return "Right";
    case Qt.Key_Up: return "Up";
    case Qt.Key_Down: return "Down";
    case Qt.Key_Backspace: return "BackSpace";
    case Qt.Key_Delete: return "Delete";
    case Qt.Key_Home: return "Home";
    case Qt.Key_End: return "End";
    case Qt.Key_PageUp: return "Prior";
    case Qt.Key_PageDown: return "Next";
    case Qt.Key_Insert: return "Insert";
    case Qt.Key_Print: return "Print";
    case Qt.Key_Minus: return "minus";
    case Qt.Key_Equal: return "equal";
    case Qt.Key_Comma: return "comma";
    case Qt.Key_Period: return "period";
    case Qt.Key_Slash: return "slash";
    case Qt.Key_Backslash: return "backslash";
    case Qt.Key_Semicolon: return "semicolon";
    case Qt.Key_Apostrophe: return "apostrophe";
    case Qt.Key_BracketLeft: return "bracketleft";
    case Qt.Key_BracketRight: return "bracketright";
    case Qt.Key_QuoteLeft: return "grave";
    }
    return "";
}

// build the combo from a KeyEvent: held modifiers + the main key, in the order
// the store writes them. "" until a non-modifier key lands.
function chordFrom(event) {
    var name = qtKeyName(event);
    if (name === "")
        return "";
    var mods = modTokens(event);
    mods.push(name);
    return mods.join(" + ");
}

// A family chord stands for its whole digit range: the {n} placeholder sits in
// the key position and expands to ten binds the provider lays down. The Keybinds
// page reasons about a family differently from a plain chord, so it needs to spot
// one, turn a recorded number key into the family form, and expand it for clash
// checks. These mirror wm/binds.go (CatalogBind.Family and Expand) on the UI side.
function isFamilyChord(chord) {
    return ("" + (chord || "")).indexOf("{n}") >= 0;
}

// familyFrom turns a chord the recorder captured into the family form the store
// keeps: the same modifiers with the digit swapped for {n}, or KP_{n} for a
// number-pad digit. A chord whose key is not a digit is not a family, so it
// returns "" and the recorder keeps waiting for a number key.
function familyFrom(chord) {
    var parts = ("" + (chord || "")).split("+");
    var toks = [];
    for (var i = 0; i < parts.length; i++) {
        var t = parts[i].trim();
        if (t.length)
            toks.push(t);
    }
    if (toks.length === 0)
        return "";
    var key = toks[toks.length - 1];
    var placeholder = "";
    if (key.length === 1 && key >= "0" && key <= "9")
        placeholder = "{n}";
    else if (key.length === 4 && key.substring(0, 3) === "KP_" && key.charAt(3) >= "0" && key.charAt(3) <= "9")
        placeholder = "KP_{n}";
    else
        return "";
    toks[toks.length - 1] = placeholder;
    return toks.join(" + ");
}

// the ten chords a family binds: {n} runs onto 1..9 then 0, and KP_{n} onto
// KP_1..KP_9 then KP_0, mirroring wm.CatalogBind.Expand. A plain chord yields
// itself, so a caller can expand any effective chord uniformly for clash checks.
function expandFamily(chord) {
    chord = "" + (chord || "");
    if (chord.indexOf("{n}") < 0)
        return chord.length ? [chord] : [];
    var out = [];
    for (var n = 1; n <= 10; n++)
        out.push(chord.replace("{n}", "" + (n % 10)));
    return out;
}
