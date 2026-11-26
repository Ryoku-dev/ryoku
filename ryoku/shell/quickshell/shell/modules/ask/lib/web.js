// The Web view's logic over what ryoseek streams: when a search is asked for,
// which scopes exist, how far each one is, and how what arrived falls into
// sections. Ranking and merging belong to the daemon; this only decides and groups.

var SCOPE_ORDER = ["web", "knowledge", "code", "linux", "video", "discuss"];

// the merged web lane's source id, and the one that only answers while typing
var LANE = "web";
var HINTS = "suggest";

var SUGGEST_MS = 170;
var STILL_MS = 1200;
var STILL_MIN = 3;

// langTag: a locale name ("en_US.UTF-8") as the tag the daemon speaks ("en-US").
// A chosen interface language that is not the locale's goes alone.
function langTag(locale, chosen) {
    var name = String(locale || "").split(".")[0].split("@")[0];
    var parts = name.split("_");
    var base = /^[a-z]{2,3}$/i.test(parts[0]) ? parts[0].toLowerCase() : "en";
    var pick = String(chosen || "").split(/[-_]/)[0].toLowerCase();
    if (pick.length > 0 && pick !== base)
        return String(chosen).replace("_", "-");
    return parts.length > 1 && /^[a-z]{2}$/i.test(parts[1]) ? base + "-" + parts[1].toUpperCase() : base;
}

// ask: what a moment in the field asks of the daemon, or null for nothing.
// event: "paused" (the hand rested SUGGEST_MS), "still" (it rested STILL_MS),
// "enter" (on the typed text), "picked" (a suggestion), "opened" (the view was
// pushed with a query), "scope" (the scope changed).
// field: {text, fullText (what the last full search ran for), picked, choosing
// (the hand is moving through the list)}.
function ask(event, field) {
    var text = String(field.text || "").trim();
    var typing = text !== String(field.fullText || "");
    if (event === "picked") {
        var picked = String(field.picked || "").trim();
        return picked.length > 0 ? { mode: "full", text: picked } : null;
    }
    if (text.length === 0)
        return event === "paused" || event === "scope" ? { mode: "suggest", text: "" } : null;
    if (event === "enter" || event === "opened")
        return { mode: "full", text: text };
    if (event === "still")
        return typing && text.length >= STILL_MIN && field.choosing !== true ? { mode: "full", text: text } : null;
    if (event === "paused")
        return typing ? { mode: "suggest", text: text } : null;
    if (event === "scope")
        return { mode: typing ? "suggest" : "full", text: text };
    return null;
}

function scopes(sources) {
    var seen = {};
    var list = sources || [];
    for (var i = 0; i < list.length; i++)
        seen[String(list[i].scope || "")] = true;
    var out = ["all"];
    for (var k = 0; k < SCOPE_ORDER.length; k++) {
        if (seen[SCOPE_ORDER[k]])
            out.push(SCOPE_ORDER[k]);
    }
    for (var name in seen) {
        if (name.length > 0 && out.indexOf(name) < 0)
            out.push(name);
    }
    return out;
}

// sourcesIn: a scope is "all", a scope name, or one source's id.
function sourcesIn(sources, scope) {
    var list = sources || [];
    if (!scope || scope === "all")
        return list.slice();
    var byScope = list.filter(function(source) { return source.scope === scope; });
    if (byScope.length > 0)
        return byScope;
    return list.filter(function(source) { return source.id === scope; });
}

function stateOf(states, id) {
    var entry = states ? states[id] : null;
    return entry ? String(entry.state || "idle") : "idle";
}

// feeding: the engines behind one source, in the daemon's order.
function feeding(engines, sourceId) {
    return (engines || []).filter(function(engine) { return (engine.sources || []).indexOf(sourceId) >= 0; });
}

// progress: one tick per source in the scope, in source order, so a strip can
// fill as each answers. The web lane shows one tick per engine instead: filled
// when that engine found a row, paused while it is blocked, and otherwise the
// lane's own state while it is out. settled counts every source that will not
// answer any more (done, error, paused); waiting is held and will still run.
function progress(sources, states, scope, engines) {
    var members = sourcesIn(sources, scope);
    var ticks = [];
    var settled = 0;
    var failed = 0;
    var loading = 0;
    var waiting = 0;
    var paused = 0;
    var found = 0;
    for (var i = 0; i < members.length; i++) {
        var id = members[i].id;
        var state = stateOf(states, id);
        var items = state === "done" ? ((states[id] || {}).items || []) : [];
        if (state === "done" || state === "error" || state === "paused")
            settled++;
        if (state === "error")
            failed++;
        if (state === "paused")
            paused++;
        if (state === "loading")
            loading++;
        if (state === "waiting")
            waiting++;
        found += items.length;
        var lane = id === LANE ? feeding(engines, LANE) : [];
        if (lane.length === 0) {
            ticks.push({ id: id, state: state });
            continue;
        }
        var gave = {};
        for (var k = 0; k < items.length; k++) {
            var from = items[k].meta && items[k].meta.engines ? items[k].meta.engines : [];
            for (var e = 0; e < from.length; e++)
                gave[from[e]] = true;
        }
        for (var n = 0; n < lane.length; n++) {
            var mark = lane[n].state === "paused" ? "paused"
                : state === "done" ? (gave[lane[n].id] ? "done" : "idle")
                : state === "paused" ? "idle" : state;
            ticks.push({ id: id + ":" + lane[n].id, state: mark });
        }
    }
    return {
        total: members.length, settled: settled, failed: failed, loading: loading, waiting: waiting, paused: paused,
        found: found, ticks: ticks, fraction: members.length > 0 ? settled / members.length : 0
    };
}

// plan: the sections of the list, top to bottom.
//   Suggestions  while typing, led by the typed text itself (Enter on it is the
//                full search); they stay under a full search only until its
//                first row lands, so the list never empties in between
//   Definition   the dictionary's card (one sense in All)
//   Web          the merged lane; when every engine failed or is paused, a
//                way on takes its place (search in the browser)
//   the rest     by the rank the daemon gave them in All (ties keep its order),
//                the first `cap` rows each; inside a scope every row, in its order
// input: {sources, states, scope, cap, query, typing, flying, available, suggestions}
function plan(input) {
    var query = String(input.query || "").trim();
    var ways = [
        { id: "fallback:browser", kind: "fallback", target: "browser" }
    ];
    if (input.available === false)
        return query.length > 0 ? [{ id: "fallback", kind: "fallback", state: "unavailable", until: "", error: "", title: "", rows: ways, total: 2, more: false }] : [];

    var scope = input.scope || "all";
    var all = scope === "all";
    var states = input.states || {};
    var members = sourcesIn(input.sources, scope);
    var limit = all ? Math.max(1, input.cap || 3) : Infinity;
    var typing = input.typing === true;
    var completions = [];
    var senses = [];
    var dictionary = null;
    var lane = null;
    var rest = [];
    for (var i = 0; i < members.length; i++) {
        var source = members[i];
        var entry = states[source.id];
        if (!entry || (typing && source.typing !== true))
            continue;
        var down = entry.state === "error" || entry.state === "paused";
        if (down && source.id === LANE) {
            lane = {
                id: "fallback", kind: "fallback", state: String(entry.state), until: String(entry.until || ""), error: String(entry.error || ""),
                title: String(source.name || source.id), source: source, rows: ways, total: 2, more: false
            };
            continue;
        }
        // a source asked again keeps showing what it last found until it answers
        var items = down ? [] : (entry.items || []);
        var rows = [];
        var total = 0;
        for (var k = 0; k < items.length; k++) {
            var item = items[k];
            var id = source.id + ":" + item.key;
            if (item.kind === "suggestion") {
                completions.push({ id: id, kind: "suggestion", item: item, source: source });
            } else if (item.kind === "definition") {
                senses.push({ id: id, kind: "definition", item: item, source: source });
                dictionary = source;
            } else {
                total++;
                if (rows.length < limit)
                    rows.push({ id: id, kind: "result", item: item, source: source });
            }
        }
        if (rows.length === 0)
            continue;
        var section = {
            id: source.id, kind: "source", title: String(source.name || source.id), source: source, rows: rows, total: total,
            more: total > rows.length || (entry.more === true && !all),
            rank: Number(entry.rank || 0), order: i
        };
        if (source.id === LANE && all)
            lane = section;
        else
            rest.push(section);
    }
    if (all)
        rest.sort(function(a, b) { return b.rank - a.rank || a.order - b.order; });

    var out = [];
    var nothingYet = senses.length === 0 && lane === null && rest.length === 0;
    if (query.length > 0 && (typing || (input.flying === true && nothingYet))) {
        var asked = query.toLowerCase();
        var most = input.suggestions || 5;
        var offers = [{ id: "query", kind: "query", text: query }];
        for (var c = 0; c < completions.length && offers.length <= most; c++) {
            if (String(completions[c].item.title || "").trim().toLowerCase() !== asked)
                offers.push(completions[c]);
        }
        out.push({ id: "suggestions", kind: "suggestions", title: "", rows: offers, total: offers.length, more: false });
    }
    if (senses.length > 0)
        out.push({
            id: "definition", kind: "definition", title: "", source: dictionary, rows: all ? senses.slice(0, 1) : senses,
            total: senses.length, more: all && senses.length > 1
        });
    if (lane)
        out.push(lane);
    return out.concat(rest);
}

function failures(sources, states, scope) {
    var members = sourcesIn(sources, scope);
    var out = [];
    for (var i = 0; i < members.length; i++) {
        var entry = states ? states[members[i].id] : null;
        if (entry && entry.state === "error")
            out.push({ id: members[i].id, name: String(members[i].name || members[i].id), error: String(entry.error || "") });
    }
    return out;
}

// paused: the engines resting behind the scope's sources, each name once, the
// soonest back first.
function paused(sources, engines, scope) {
    var members = {};
    sourcesIn(sources, scope).forEach(function(source) { members[source.id] = true; });
    var seen = {};
    var out = [];
    var list = engines || [];
    for (var i = 0; i < list.length; i++) {
        var engine = list[i];
        var name = String(engine.name || engine.id);
        var feeds = (engine.sources || []).some(function(id) { return members[id] === true; });
        if (engine.state !== "paused" || !feeds || seen[name])
            continue;
        seen[name] = true;
        out.push({ id: engine.id, name: name, until: String(engine.until || "") });
    }
    // a pause with no end named sorts last
    return out.sort(function(a, b) {
        var x = a.until || "\uffff";
        var y = b.until || "\uffff";
        return x < y ? -1 : x > y ? 1 : 0;
    });
}

// foundBy: the names of the engines a merged row came from.
function foundBy(item, engines) {
    var ids = item && item.meta && item.meta.engines ? item.meta.engines : [];
    var names = {};
    (engines || []).forEach(function(engine) { names[engine.id] = String(engine.name || engine.id); });
    return ids.map(function(id) { return names[id] || String(id); });
}

var FILE_EXT = /\.(zip|7z|rar|tar|tgz|gz|xz|zst|bz2|iso|img|deb|rpm|appimage|flatpak|apk|exe|msi|dmg|pkg|pdf|epub|mp4|mkv|webm|mov|avi|mp3|flac|ogg|opus|wav|m4a|torrent)(?:$|[?#])/i;

// downloadable: a result the download engine can take as it is.
function downloadable(item) {
    if (!item || !item.url)
        return false;
    if (item.kind === "video")
        return true;
    if (item.meta && (item.meta.download === true || typeof item.meta.download === "string"))
        return true;
    return FILE_EXT.test(String(item.url));
}

function downloadUrl(item) {
    return item && item.meta && typeof item.meta.download === "string" ? item.meta.download : String(item ? item.url : "");
}

var ALT_KEYS = ["discussion", "repository", "homepage", "documentation"];

// altLinks: other places a result points at (a thread beside its article, a
// package's repository), read from meta values that are web addresses.
function altLinks(item) {
    var meta = item && item.meta ? item.meta : {};
    var out = [];
    for (var i = 0; i < ALT_KEYS.length; i++) {
        var value = meta[ALT_KEYS[i]];
        if (typeof value === "string" && /^https?:\/\//i.test(value) && value !== item.url)
            out.push({ id: ALT_KEYS[i], url: value });
    }
    return out;
}

// facts: the one or two figures a result row sets at its right edge, from the
// source's own meta. count formats a tally (format.js count).
function facts(item, count) {
    var meta = item && item.meta ? item.meta : {};
    var out = [];
    function tally(glyph, value) {
        if (typeof value === "number" && value >= 0)
            out.push({ glyph: glyph, text: count(value) });
    }
    function word(glyph, value) {
        if (typeof value === "string" && value.length > 0)
            out.push({ glyph: glyph, text: value });
    }
    tally("star", meta.stars);
    tally("arrow_upward", meta.score);
    tally("arrow_upward", meta.points);
    tally("arrow_upward", meta.votes);
    if (meta.accepted === true)
        out.push({ glyph: "check", text: count(meta.answers || 0) });
    else
        tally("forum", meta.answers);
    tally("forum", meta.comments);
    word("", meta.durationText);
    if (typeof meta.version === "string" && meta.version.length > 0)
        out.push({ glyph: "", text: meta.version, mono: true });
    tally("download", meta.weeklyDownloads);
    tally("download", meta.recentDownloads);
    word("", meta.language);
    if (meta.verified === true)
        out.push({ glyph: "verified", text: "" });
    return out.slice(0, 2);
}

function browserSearchUrl(query) {
    return "https://duckduckgo.com/?q=" + encodeURIComponent(String(query || ""));
}

if (typeof module !== "undefined" && module.exports)
    module.exports = {
        SUGGEST_MS: SUGGEST_MS, STILL_MS: STILL_MS, STILL_MIN: STILL_MIN, langTag: langTag, ask: ask,
        scopes: scopes, sourcesIn: sourcesIn, feeding: feeding, progress: progress, plan: plan, failures: failures,
        paused: paused, foundBy: foundBy, facts: facts, downloadable: downloadable, downloadUrl: downloadUrl,
        altLinks: altLinks, browserSearchUrl: browserSearchUrl
    };
