function frameFrom(f) {
    if (!f || typeof f !== "object" || typeof f.running !== "boolean")
        return null;
    var adapters = Array.isArray(f.adapters) ? f.adapters.filter(function (a) {
        return a && typeof a.path === "string";
    }) : [];
    return { running: f.running, adapters: adapters };
}

function resolve(qsAdapter, qsEnabled, feed) {
    var gone = feed !== null && !feed.running;
    if (qsAdapter && !gone)
        return { useQuickshell: true, present: true, enabled: qsEnabled };
    var fallback = feed !== null && feed.running && feed.adapters.length > 0 ? feed.adapters[0] : null;
    return { useQuickshell: false, present: fallback !== null, enabled: fallback !== null && fallback.powered === true };
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { frameFrom, resolve };
