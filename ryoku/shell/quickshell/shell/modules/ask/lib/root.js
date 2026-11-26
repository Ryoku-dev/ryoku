// The root view's logic: what a typed line is (a link, a sum, a prefix), which
// commands it matches and in what order, and the sections the list shows.
// Labels arrive translated from QML; nothing here names a string a reader sees.

var PREFIXES = { "?": "web", "/": "downloads" };

function prefixRoute(text) {
    var value = String(text || "");
    var view = PREFIXES[value.charAt(0)];
    if (!view)
        return null;
    return { view: view, rest: value.slice(1).replace(/^\s+/, ""), external: view === "downloads" };
}

function hostOf(url) {
    var m = /^[a-z][a-z0-9+.-]*:\/\/(?:[^@\/?#]*@)?(\[[^\]]+\]|[^:\/?#]+)/i.exec(String(url || ""));
    return m ? m[1].replace(/^www\./i, "").toLowerCase() : "";
}

function magnetName(uri) {
    var m = /[?&]dn=([^&]+)/i.exec(uri);
    if (!m)
        return "";
    try {
        return decodeURIComponent(m[1].replace(/\+/g, " "));
    } catch (error) {
        return m[1];
    }
}

// detectLink reads a whole line as one link: a magnet, a web or ftp address, or
// a bare domain a person would paste. Anything with a space is a sentence.
function detectLink(text) {
    var value = String(text || "").trim();
    if (value.length === 0 || /\s/.test(value))
        return null;
    if (/^magnet:\?/i.test(value)) {
        if (!/[?&]xt=urn:[a-z0-9]+:[a-z0-9]{20,}/i.test(value))
            return null;
        return { kind: "magnet", url: value, host: "", name: magnetName(value) };
    }
    if (/^(https?|ftp|sftp):\/\//i.test(value)) {
        var host = hostOf(value);
        if (host.length === 0)
            return null;
        return { kind: /\.torrent(?:$|[?#])/i.test(value) ? "torrent" : "url", url: value, host: host, name: "" };
    }
    if (/^(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\.)+[a-z]{2,}(?::\d+)?(?:[\/?#]\S*)?$/i.test(value)
            && !/^\d+(\.\d+)+$/.test(value)) {
        var full = "https://" + value;
        return { kind: /\.torrent(?:$|[?#])/i.test(value) ? "torrent" : "url", url: full, host: hostOf(full), name: "" };
    }
    return null;
}

// looksLikeMath keeps the calculator off plain words: a sum needs a digit and
// something to do with it, or a conversion.
function looksLikeMath(text) {
    var value = String(text || "").trim();
    if (value.length < 2 || value.length > 120 || !/\d/.test(value))
        return false;
    if (detectLink(value) || countLinks(value) > 0)
        return false;
    if (/^[\d\s.,]+$/.test(value))
        return false;
    if (/[+*\/^%=()]|\d\s*-\s*\d|\d\s*x\s*\d/i.test(value))
        return true;
    if (/\b(to|in|as)\b/i.test(value) && /\d\s*[a-z°$€£]/i.test(value))
        return true;
    return /\b(sqrt|sin|cos|tan|log|ln|abs|pi|mod)\b/i.test(value);
}

function fold(text) {
    return String(text || "").toLowerCase();
}

// matchScore: 0 is no match. A whole match beats a prefix, a prefix beats a
// word start, a word start beats text inside a word, and letters in order come last.
function matchScore(query, text) {
    var q = fold(query).trim();
    var t = fold(text);
    if (q.length === 0)
        return 1;
    if (t.length === 0)
        return 0;
    if (t === q)
        return 100;
    if (t.indexOf(q) === 0)
        return 80;
    var at = t.indexOf(q);
    if (at > 0)
        return /[\s\-_\/.]/.test(t.charAt(at - 1)) ? 60 : 40;
    var qi = 0;
    for (var i = 0; i < t.length && qi < q.length; i++) {
        if (t.charAt(i) === q.charAt(qi))
            qi++;
    }
    return qi === q.length && q.length >= 2 ? 15 : 0;
}

// rank filters commands {id, title, keywords:[...]} by the query and orders
// them best first; ties keep the order they were given in.
function rank(commands, query) {
    var scored = [];
    for (var i = 0; i < commands.length; i++) {
        var command = commands[i];
        var best = matchScore(query, command.title);
        var words = command.keywords || [];
        for (var k = 0; k < words.length; k++)
            best = Math.max(best, matchScore(query, words[k]) - 5);
        if (best > 0)
            scored.push({ command: command, score: best, order: i });
    }
    scored.sort(function(a, b) { return b.score - a.score || a.order - b.order; });
    return scored.map(function(entry) { return entry.command; });
}

// what the home list always shows; a paused download waits on the reader, so it is news like a finished one
var MOVING = ["resolving", "active", "merging", "queued", "seeding"];
var SETTLED = ["done", "error", "stopped", "paused"];
// a settled row stays this long after the home list first showed it
var GRACE = 10 * 60 * 1000;
// what settled longer ago than this is history, not news
var STALE = 24 * 60 * 60 * 1000;

function at(text) {
    var value = Date.parse(String(text || ""));
    return isNaN(value) ? 0 : value;
}

function stamp(task) {
    return at(task.finished || task.started || task.created);
}

// news: shown less than GRACE ago, or never shown and settled in the last day. a pause has no
// stamp of its own, so a paused download stays news until it is seen
function news(task, now) {
    var state = String(task.state || "");
    if (SETTLED.indexOf(state) < 0)
        return false;
    var seen = at(task.seen);
    if (seen > 0)
        return now - seen < GRACE;
    if (state === "paused")
        return true;
    var settledAt = stamp(task);
    return settledAt > 0 && now - settledAt < STALE;
}

// pickDownloads: everything moving, in the daemon's order, then the newest news up to recentLimit.
function pickDownloads(tasks, now, recentLimit) {
    var moving = [];
    var fresh = [];
    var list = tasks || [];
    for (var i = 0; i < list.length; i++) {
        if (MOVING.indexOf(String(list[i].state || "")) >= 0)
            moving.push(list[i]);
        else if (news(list[i], now))
            fresh.push(list[i]);
    }
    fresh.sort(function(a, b) { return stamp(b) - stamp(a); });
    return moving.concat(fresh.slice(0, Math.max(0, recentLimit === undefined ? 3 : recentLimit)));
}

// unseen: the news the engine has not been told about, all of it, so a long batch leaves the
// home list together instead of a few at a time. stale rows stay unmarked, a stamp would make
// them news again
function unseen(tasks, now) {
    var out = [];
    var list = tasks || [];
    for (var i = 0; i < list.length; i++) {
        // a stamp from a clock that ran ahead counts as no stamp, or the row would stay until that time
        var seen = at(list[i].seen);
        if ((seen === 0 || seen > now) && news(list[i], now))
            out.push(String(list[i].id));
    }
    return out;
}

// the stamp that takes a row off the home list at once
function dismissed(now) {
    return new Date(now - GRACE - 60 * 1000);
}

function quote(text, limit) {
    var value = String(text || "").replace(/\s+/g, " ").trim();
    var max = limit || 48;
    return value.length > max ? value.slice(0, max - 1).replace(/\s+$/, "") + "\u2026" : value;
}

var LINK_STARTS = /(?:https?|ftp|sftp):\/\/(?=[^\s\/])|magnet:\?(?=\S)|(?:^|[^\/.\w-])www\.[a-z0-9-]+\.[a-z]{2,}/gi;

// countLinks: how many links a paste may hold, by their starts alone (scheme
// links, magnets, www. hosts). It only decides whether the engine is asked to
// read the text; the engine's answer is what counts.
function countLinks(text) {
    var found = String(text || "").match(LINK_STARTS);
    return found ? found.length : 0;
}

// asLink: one of the engine's links in the shape detectLink gives.
function asLink(entry) {
    var url = String(entry && entry.url || "");
    var kind = String(entry && entry.kind || "url");
    return { kind: kind, url: url, host: kind === "magnet" ? "" : hostOf(url), name: kind === "magnet" ? magnetName(url) : "" };
}

// batch: the engine's reading of a paste (links {text}), as the root shows it:
// the links worth adding with duplicates dropped, and what was adjusted. Cleaned
// counts the links kept, so the line agrees with the number in the title.
function batch(result) {
    var list = (result && result.links) || [];
    var links = [];
    var hosts = [];
    var touched = 0;
    var dropped = 0;
    for (var i = 0; i < list.length; i++) {
        var entry = list[i];
        if (entry && entry.duplicate === true)
            dropped++;
        if (!entry || entry.duplicate === true || String(entry.url || "").length === 0)
            continue;
        var link = asLink(entry);
        links.push(link);
        if ((entry.notes || []).length > 0)
            touched++;
        var host = link.kind === "magnet" ? link.name : link.host;
        if (host.length > 0 && hosts.indexOf(host) < 0)
            hosts.push(host);
    }
    return {
        links: links,
        urls: links.map(function(link) { return link.url; }),
        count: links.length,
        cleaned: touched,
        duplicates: dropped,
        hosts: hosts
    };
}

// sections builds the list. input:
//   query, commands (already translated), link (detectLink result or null),
//   many (several links in the field: {state: "reading"|"ready"|"failed", ...}
//   or null; it takes the place of link), calc ({expr, result} or null),
//   downloads (rows), labels ({downloads, goTo, results, link, links, calc,
//   fallbacks}).
// Every section is {id, title, rows}; a row carries {id, kind, ...}.
function sections(input) {
    var query = String(input.query || "").trim();
    var labels = input.labels || {};
    var out = [];
    if (query.length === 0) {
        if ((input.downloads || []).length > 0)
            out.push({ id: "downloads", title: labels.downloads || "", rows: input.downloads });
        out.push({ id: "goto", title: labels.goTo || "", rows: (input.commands || []).map(function(command) {
            return { id: "cmd:" + command.id, kind: "command", command: command };
        }) });
        return out;
    }
    var top = [];
    if (input.many)
        top.push({ id: "many", kind: "many", many: input.many });
    else if (input.link)
        top.push({ id: "link", kind: "link", link: input.link });
    if (input.calc && String(input.calc.result || "").length > 0 && input.calc.expr === query)
        top.push({ id: "calc", kind: "calc", calc: input.calc });
    if (top.length > 0)
        out.push({ id: "top", title: input.many ? (labels.links || "") : input.link ? (labels.link || "") : (labels.calc || ""), rows: top });
    var matched = rank(input.commands || [], query);
    if (matched.length > 0)
        out.push({ id: "results", title: labels.results || "", rows: matched.map(function(command) {
            return { id: "cmd:" + command.id, kind: "command", command: command };
        }) });
    out.push({ id: "fallbacks", title: labels.fallbacks || "", rows: [
        { id: "fallback:web", kind: "fallback", target: "web", text: query }
    ] });
    return out;
}

if (typeof module !== "undefined" && module.exports)
    module.exports = {
        prefixRoute: prefixRoute, hostOf: hostOf, detectLink: detectLink, looksLikeMath: looksLikeMath,
        matchScore: matchScore, rank: rank, pickDownloads: pickDownloads, unseen: unseen, dismissed: dismissed,
        quote: quote, sections: sections, countLinks: countLinks, asLink: asLink, batch: batch,
        GRACE: GRACE
    };
