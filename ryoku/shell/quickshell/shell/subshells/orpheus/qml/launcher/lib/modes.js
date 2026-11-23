// Launcher modes: which provider a mode and a query reach, and the glyph that
// names where the query is going. routeFn is the dispatcher's prefix router
// (text -> { provider, query, prefix }).

var MODES = ["all", "apps", "windows", "files", "actions"];

function plain(query) {
    return String(query == null ? "" : query);
}

// { rest, provider, query, prefix }: rest is the empty All state, provider ""
// fans out across the default providers.
function routeFor(mode, query, routeFn) {
    var q = plain(query);
    switch (mode) {
    case "apps":
        return { rest: false, provider: "apps", query: q, prefix: "" };
    case "windows":
        return { rest: false, provider: "windows", query: q, prefix: "" };
    case "files":
        return q.trim().length === 0
            ? { rest: false, provider: "recent", query: "", prefix: "" }
            : { rest: false, provider: "find", query: q, prefix: "/file" };
    case "actions":
        return { rest: false, provider: "actions", query: q, prefix: "/" };
    default:
        if (q.length === 0)
            return { rest: true, provider: "apps", query: "", prefix: "" };
        var routed = routeFn ? routeFn(q) : null;
        return {
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
    var routed = routeFn ? routeFn(q) : null;
    return routed && routed.provider ? plain(routed.query) : q;
}

// "seal" (the design's own search mark) unless the query has a mode identity.
function glyphFor(route) {
    var r = route || {};
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
        MODES: MODES,
        routeFor: routeFor,
        queryForMode: queryForMode,
        glyphFor: glyphFor
    };
}
