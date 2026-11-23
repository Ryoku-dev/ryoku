// Launcher modes: which provider a mode and a query reach, and the glyph that
// names where the query is going. routeFn is the dispatcher's prefix router
// (text -> { provider, query, prefix }).

var BASE_MODES = ["all", "apps", "windows", "files", "actions"];
var ASK_PREFIX = "\\";

function modesFor(askAvailable) {
    return askAvailable ? BASE_MODES.concat(["ask"]) : BASE_MODES.slice();
}

function plain(query) {
    return String(query == null ? "" : query);
}

function askQuestion(query) {
    var q = plain(query);
    if (q.charAt(0) === ASK_PREFIX)
        q = q.slice(1);
    return q.replace(/^\s+/, "");
}

// { ask, rest, provider, query, prefix }: ask runs Rashin, rest is the empty
// All state, provider "" fans out across the default providers.
function routeFor(mode, query, routeFn, askAvailable) {
    var q = plain(query);
    switch (mode) {
    case "ask":
        return { ask: true, rest: false, provider: "", query: askQuestion(q), prefix: ASK_PREFIX };
    case "apps":
        return { ask: false, rest: false, provider: "apps", query: q, prefix: "" };
    case "windows":
        return { ask: false, rest: false, provider: "windows", query: q, prefix: "" };
    case "files":
        return q.trim().length === 0
            ? { ask: false, rest: false, provider: "recent", query: "", prefix: "" }
            : { ask: false, rest: false, provider: "find", query: q, prefix: "/file" };
    case "actions":
        return { ask: false, rest: false, provider: "actions", query: q, prefix: "/" };
    default:
        if (askAvailable && q.charAt(0) === ASK_PREFIX)
            return { ask: true, rest: false, provider: "", query: askQuestion(q), prefix: ASK_PREFIX };
        if (q.length === 0)
            return { ask: false, rest: true, provider: "apps", query: "", prefix: "" };
        var routed = routeFn ? routeFn(q) : null;
        return {
            ask: false,
            rest: false,
            provider: String(routed && routed.provider || ""),
            query: routed ? plain(routed.query) : q,
            prefix: String(routed && routed.prefix || "")
        };
    }
}

// The query a mode keeps when switched to: a routing prefix typed in All is
// dropped, since the mode itself now says where the query goes.
function queryForMode(mode, query, routeFn) {
    var q = plain(query);
    if (mode === "all")
        return q;
    if (q.charAt(0) === ASK_PREFIX)
        return askQuestion(q);
    var routed = routeFn ? routeFn(q) : null;
    return routed && routed.provider ? plain(routed.query) : q;
}

// "seal" (the design's own search mark) unless the query has a mode identity.
function glyphFor(route) {
    var r = route || {};
    if (r.ask)
        return "auto_awesome";
    if (r.provider === "actions")
        return "bolt";
    if (r.provider === "calc")
        return "calculate";
    if (r.provider === "web" && r.prefix === "?")
        return "public";
    return "seal";
}

if (typeof module !== "undefined" && module.exports) {
    module.exports = {
        BASE_MODES: BASE_MODES,
        modesFor: modesFor,
        routeFor: routeFor,
        queryForMode: queryForMode,
        glyphFor: glyphFor
    };
}
