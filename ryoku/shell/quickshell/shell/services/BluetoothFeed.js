// BluetoothPower.qml and its regression tests share this policy for the daemon's
// `bluetooth` topic: {running, adapters: [{path, name, address, powered}]}.

// Returns a clean copy of the bus frame f (already parsed), or null for junk.
function frameFrom(f) {
    if (!f || typeof f !== "object" || typeof f.running !== "boolean")
        return null;
    var adapters = Array.isArray(f.adapters) ? f.adapters.filter(function (a) {
        return a && typeof a.path === "string";
    }) : [];
    return { running: f.running, adapters: adapters };
}

// Quickshell stays the source while it has an adapter and BlueZ is not known gone.
// feed is the last frame, or null while the daemon has sent none.
//   qsAdapter: whether Quickshell lists an adapter; qsEnabled: its power.
// Returns { useQuickshell, present, enabled }.
function resolve(qsAdapter, qsEnabled, feed) {
    var gone = feed !== null && !feed.running;
    if (qsAdapter && !gone)
        return { useQuickshell: true, present: true, enabled: qsEnabled };
    var fallback = feed !== null && feed.running && feed.adapters.length > 0 ? feed.adapters[0] : null;
    return { useQuickshell: false, present: fallback !== null, enabled: fallback !== null && fallback.powered === true };
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { frameFrom, resolve };
