// Query text helpers: the ghost completion after the caret and the matched
// characters of a title, for the track list's highlight.

// The rest of the title when the query is its start, case-insensitive.
function ghostFor(query, title) {
    var q = String(query == null ? "" : query);
    var t = String(title == null ? "" : title);
    if (q.length === 0 || q.length >= t.length)
        return "";
    if (t.slice(0, q.length).toLowerCase() !== q.toLowerCase())
        return "";
    return t.slice(q.length);
}

function isWordStart(text, index) {
    if (index <= 0)
        return true;
    var prev = text.charAt(index - 1);
    var here = text.charAt(index);
    if (!/[0-9A-Za-z\u00C0-\uFFFF]/.test(prev))
        return true;
    return prev === prev.toLowerCase() && here !== here.toLowerCase();
}

function segments(text, hits) {
    var out = [];
    for (var i = 0; i < text.length; i++) {
        var hit = hits[i] === true;
        var last = out.length > 0 ? out[out.length - 1] : null;
        if (last && last.hit === hit)
            last.text += text.charAt(i);
        else
            out.push({ text: text.charAt(i), hit: hit });
    }
    return out;
}

// [{ text, hit }] covering the whole title. A contiguous match wins, at a
// word start if there is one; else the query's characters in order; else none.
function matchSpans(title, query) {
    var text = String(title == null ? "" : title);
    var q = String(query == null ? "" : query).trim().toLowerCase();
    if (text.length === 0)
        return [];
    var hits = [];
    if (q.length === 0)
        return segments(text, hits);

    var lower = text.toLowerCase();
    var best = -1;
    var at = lower.indexOf(q);
    while (at >= 0) {
        if (best < 0)
            best = at;
        if (isWordStart(text, at)) {
            best = at;
            break;
        }
        at = lower.indexOf(q, at + 1);
    }
    if (best >= 0) {
        for (var i = best; i < best + q.length; i++)
            hits[i] = true;
        return segments(text, hits);
    }

    var needle = q.replace(/\s+/g, "");
    var from = 0;
    var picked = [];
    for (var n = 0; n < needle.length; n++) {
        var ch = needle.charAt(n);
        var found = -1;
        for (var j = from; j < lower.length; j++) {
            if (lower.charAt(j) !== ch)
                continue;
            if (found < 0)
                found = j;
            if (isWordStart(text, j)) {
                found = j;
                break;
            }
        }
        if (found < 0)
            return segments(text, []);
        picked.push(found);
        from = found + 1;
    }
    for (var k = 0; k < picked.length; k++)
        hits[picked[k]] = true;
    return segments(text, hits);
}

function escapeHtml(value) {
    return String(value == null ? "" : value)
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;");
}

if (typeof module !== "undefined" && module.exports) {
    module.exports = { ghostFor: ghostFor, matchSpans: matchSpans, escapeHtml: escapeHtml };
}
