// All of Kit (Ctrl+F): each section answers one query with rows, the frame
// groups them by section in rail order. A row is a ResultRow row plus
// open: {page?, query?, args?}, what its section should show when it is chosen.

function fold(text) {
    return String(text || "").toLowerCase();
}

function terms(query) {
    return fold(query).split(/\s+/).filter(function(term) { return term.length > 0; });
}

// matches: every word of the query appears somewhere in the texts.
function matches(query, texts) {
    var words = terms(query);
    if (words.length === 0)
        return false;
    var hay = fold((texts || []).join("\n"));
    for (var i = 0; i < words.length; i++) {
        if (hay.indexOf(words[i]) < 0)
            return false;
    }
    return true;
}

// chordForms: a shortcut's keys the ways a reader types them:
// ["Super", "Shift", "P"] -> "super+shift+p", "super shift p", "super + shift + p".
function chordForms(keys) {
    var list = (keys || []).map(fold);
    if (list.length === 0)
        return [];
    return [list.join("+"), list.join(" "), list.join(" + ")];
}

function chordMatches(query, keys) {
    var q = fold(query).trim().replace(/\s*\+\s*/g, "+");
    if (q.length === 0)
        return false;
    return chordForms(keys)[0] !== undefined && chordForms(keys)[0].indexOf(q) >= 0;
}

// keyRows: the legend's shortcuts that answer the query, by name, hint (in the
// reader's language too, through tr) or chord. categories: [{name, binds:
// [{desc, hint, keys, unhonored}]}]; a bind's page is its category.
function keyRows(categories, query, tr) {
    var say = tr || function(text) { return text; };
    var out = [];
    var list = categories || [];
    for (var c = 0; c < list.length; c++) {
        var binds = list[c].binds || [];
        for (var b = 0; b < binds.length; b++) {
            var bind = binds[b];
            if (bind.unhonored)
                continue;
            var name = say(String(bind.desc || ""));
            if (!matches(query, [bind.desc, bind.hint, name, say(String(bind.hint || ""))]) && !chordMatches(query, bind.keys))
                continue;
            out.push({
                id: "c" + c + "b" + b, size: "one", glyph: "keyboard",
                title: name, subtitle: say(String(list[c].name || "")),
                trail: (bind.keys || []).join(" + "), trailMono: true,
                open: { page: "category-" + c, query: String(bind.desc || "") }
            });
        }
    }
    return out;
}

// listRows: a plain list (tools, pages, tasks, commands) filtered by the query.
// read(item) answers {id, title, subtitle?, trail?, glyph?, texts?: [more to
// match on], open}.
function listRows(items, query, read) {
    var out = [];
    var list = items || [];
    for (var i = 0; i < list.length; i++) {
        var row = read(list[i], i);
        if (!row)
            continue;
        if (!matches(query, [row.title, row.subtitle, row.trail].concat(row.texts || [])))
            continue;
        out.push({
            id: String(row.id), size: "one", glyph: String(row.glyph || ""),
            title: String(row.title || ""), subtitle: String(row.subtitle || ""),
            trail: String(row.trail || ""), trailMono: row.trailMono === true,
            open: row.open || {}
        });
    }
    return out;
}

var SEP = "\u0000";

// group: sections for the result list, in rail order, empty ones left out.
// bySection: {id: [rows]}; labels: {id: label}; opts: {limit (rows shown per
// section), more(count, label) -> the last row's words}. A section with more
// rows than the limit ends on a row that opens it with the query in its field.
function group(order, labels, bySection, query, opts) {
    var o = opts || {};
    var limit = o.limit > 0 ? o.limit : 6;
    var out = [];
    for (var i = 0; i < order.length; i++) {
        var id = order[i];
        var rows = (bySection && bySection[id]) || [];
        if (rows.length === 0)
            continue;
        var shown = rows.slice(0, limit).map(function(row) {
            var copy = {};
            for (var k in row)
                copy[k] = row[k];
            copy.id = id + SEP + row.id;
            copy.section = id;
            return copy;
        });
        if (rows.length > limit) {
            var rest = rows.length - limit;
            shown.push({
                id: id + SEP + "more", section: id, size: "one", glyph: "arrow_forward",
                title: o.more ? String(o.more(rest, labels[id] || id)) : String(rest),
                subtitle: "", open: { query: String(query || "") }
            });
        }
        out.push({ id: id, title: String(labels[id] || id), note: String(rows.length), rows: shown });
    }
    return out;
}

function total(sections) {
    var n = 0;
    for (var i = 0; i < sections.length; i++)
        n += Number(sections[i].note) || 0;
    return n;
}

// target: where a chosen row leads.
function target(row) {
    if (!row || !row.section)
        return null;
    var open = row.open || {};
    return { section: String(row.section), page: String(open.page || ""), query: String(open.query || ""), args: open.args || null };
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { terms: terms, matches: matches, chordForms: chordForms, chordMatches: chordMatches, keyRows: keyRows, listRows: listRows, group: group, total: total, target: target, SEP: SEP };
