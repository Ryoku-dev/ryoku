// Ranking for launcher results: prefix beats substring beats subsequence, with a
// usage-frequency tiebreak. Shared by the apps provider (QML import) and node
// tests (CommonJS export). The match contract matches the pill launcher's so an
// entry ranked here lands the same way it did before the rework.

function haystacks(e) {
    var parts = [];
    if (e.name) parts.push(String(e.name));
    if (e.genericName) parts.push(String(e.genericName));
    if (e.keywords) for (var i = 0; i < e.keywords.length; i++) parts.push(String(e.keywords[i]));
    return parts;
}

function subsequence(needle, hay) {
    var j = 0;
    for (var i = 0; i < hay.length && j < needle.length; i++)
        if (hay[i] === needle[j]) j++;
    return j === needle.length;
}

function score(e, q) {
    var name = (e.name || "").toLowerCase();
    if (name.indexOf(q) === 0) return 0;
    var fields = haystacks(e);
    var best = 99;
    for (var i = 0; i < fields.length; i++) {
        var f = fields[i].toLowerCase();
        if (f.indexOf(q) !== -1) { best = Math.min(best, 1); continue; }
        if (subsequence(q, f)) best = Math.min(best, 2);
    }
    return best;
}

function uses(usage, e) {
    if (!usage || !e || !e.id) return 0;
    var c = usage[e.id];
    return typeof c === "number" ? c : 0;
}

function rank(entries, query, usage) {
    usage = usage || {};
    var visible = [];
    for (var i = 0; i < entries.length; i++)
        if (!entries[i].noDisplay) visible.push(entries[i]);

    var q = (query || "").trim().toLowerCase();
    if (q.length === 0)
        return visible.slice().sort(function (a, b) {
            var ua = uses(usage, a);
            var ub = uses(usage, b);
            if (ua !== ub) return ub - ua;
            return (a.name || "").toLowerCase().localeCompare((b.name || "").toLowerCase());
        });

    var scored = [];
    for (var k = 0; k < visible.length; k++) {
        var s = score(visible[k], q);
        if (s < 99) scored.push({ e: visible[k], s: s });
    }
    scored.sort(function (a, b) {
        if (a.s !== b.s) return a.s - b.s;
        var ua = uses(usage, a.e);
        var ub = uses(usage, b.e);
        if (ua !== ub) return ub - ua;
        return (a.e.name || "").toLowerCase().localeCompare((b.e.name || "").toLowerCase());
    });
    return scored.map(function (x) { return x.e; });
}

// Spans of `query` matched against `name` as a subsequence, for the result
// row's char-highlight. Adjacent matched chars merge into one span; a name that
// doesn't contain the full subsequence (or an empty query) yields no spans.
function highlight(name, query) {
    var q = (query || "").toLowerCase();
    if (q.length === 0) return [];
    var hay = String(name || "").toLowerCase();
    var hits = [];
    var j = 0;
    for (var i = 0; i < hay.length && j < q.length; i++)
        if (hay[i] === q[j]) { hits.push(i); j++; }
    if (j < q.length) return [];
    var spans = [];
    for (var k = 0; k < hits.length; k++) {
        var last = spans[spans.length - 1];
        if (last && hits[k] === last.start + last.len) last.len++;
        else spans.push({ start: hits[k], len: 1 });
    }
    return spans;
}

if (typeof module !== "undefined" && module.exports) {
    module.exports = { rank, score, subsequence, highlight };
}
