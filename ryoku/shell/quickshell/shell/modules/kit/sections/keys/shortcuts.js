function fold(value) {
    return String(value || "").normalize("NFKD").replace(/[\u0300-\u036f]/g, "").toLowerCase();
}

function key(value) {
    var k = fold(value).replace(/\s+/g, "");
    var aliases = { meta: "super", win: "super", control: "ctrl", return: "enter", escape: "esc", pgup: "pageup", pgdn: "pagedown", "←": "left", "→": "right", "↑": "up", "↓": "down" };
    return aliases[k] || k;
}

function chord(value) {
    var parts = (Array.isArray(value) ? value : String(value || "").split(/[+\s]+/)).map(key).filter(Boolean);
    var modifiers = ["super", "ctrl", "alt", "shift"];
    return modifiers.filter(function(m) { return parts.indexOf(m) >= 0; }).concat(parts.filter(function(p) { return modifiers.indexOf(p) < 0; })).join("+");
}

function matches(bind, query, tr) {
    var q = fold(query).trim();
    if (!q)
        return true;
    var say = tr || function(text) { return text; };
    var hay = fold([bind.desc, bind.hint, say(bind.desc || ""), say(bind.hint || ""), (bind.keys || []).join(" ")].join("\n"));
    return q.split(/\s+/).every(function(term) { return hay.indexOf(term) >= 0; }) || chord(bind.keys).indexOf(chord(q)) >= 0;
}

function groups(categories, query, tr) {
    var out = [];
    (categories || []).forEach(function(category, c) {
        var rows = [];
        (category.binds || []).forEach(function(bind, b) {
            if (!bind.disabled && matches(bind, query, tr))
                rows.push({ bind: bind, id: "c" + c + "b" + b, category: c });
        });
        if (rows.length)
            out.push({ name: category.name, category: c, rows: rows });
    });
    return out;
}

function layout(groups, columns, rowHeight, headHeight, gap) {
    var heights = Array(columns).fill(6);
    var index = 0;
    return groups.map(function(group) {
        var column = heights.indexOf(Math.min.apply(null, heights));
        var height = headHeight + group.rows.length * rowHeight;
        var block = { group: group, column: column, y: heights[column], height: height, start: index };
        index += group.rows.length;
        heights[column] += height + gap;
        return block;
    });
}

function eventChord(code, modifiers, text) {
    if (code >= 0x01000020 && code <= 0x01000026)
        return "";
    var names = { 16777216: "Esc", 16777217: "Tab", 16777219: "Backspace", 16777220: "Enter", 16777221: "Enter", 16777223: "Delete", 16777232: "Home", 16777233: "End", 16777234: "Left", 16777235: "Up", 16777236: "Right", 16777237: "Down", 16777238: "PageUp", 16777239: "PageDown", 32: "Space" };
    var name = names[code] || (code >= 0x01000030 && code <= 0x01000052 ? "F" + (code - 0x01000030 + 1) : code < 0x01000000 ? String.fromCharCode(code) : text);
    var media = { 0x01000070: "Vol -", 0x01000071: "Mute", 0x01000072: "Vol +", 0x01000080: "Play", 0x01000082: "Prev", 0x01000083: "Next", 0x01000085: "Pause", 0x01000113: "Mic mute", 0x010000b2: "Brightness +", 0x010000b3: "Brightness -", 0x01000110: "Touchpad", 0x01000111: "Touchpad On", 0x01000112: "Touchpad Off" };
    if (media[code])
        name = media[code];
    if ((modifiers & 0x20000000) && code >= 0x30 && code <= 0x39)
        name = "Num " + name;
    if (!name)
        return "";
    var parts = [];
    if (modifiers & 0x10000000) parts.push("Super");
    if (modifiers & 0x04000000) parts.push("Ctrl");
    if (modifiers & 0x08000000) parts.push("Alt");
    if (modifiers & 0x02000000) parts.push("Shift");
    parts.push(name);
    return chord(parts);
}

function find(rows, pressed) {
    for (var i = 0; i < rows.length; i++) {
        var keys = rows[i].bind.keys || [];
        var forms = [chord(keys)];
        if (keys.length && /^(Num )?1 … 0$/.test(keys[keys.length - 1])) {
            for (var digit = 0; digit < 10; digit++)
                forms.push(chord(keys.slice(0, -1).concat((keys[keys.length - 1].indexOf("Num ") === 0 ? "Num " : "") + digit)));
        }
        if (!rows[i].bind.unhonored && forms.indexOf(pressed) >= 0)
            return rows[i].id;
    }
    return "";
}

if (typeof module !== "undefined")
    module.exports = { fold: fold, chord: chord, matches: matches, groups: groups, layout: layout, eventChord: eventChord, find: find };
