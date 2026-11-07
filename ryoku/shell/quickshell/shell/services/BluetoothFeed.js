// BluetoothPower.qml and its regression tests share this policy for the daemon's
// `bluetooth` topic: {running, adapters: [{path, name, address, powered}]}.

// Returns the frame, or null for an error line ("err unknown topic: ...") or junk.
function parseFrame(line) {
    var f;
    try {
        f = JSON.parse(line);
    } catch (e) {
        return null;
    }
    if (!f || typeof f.running !== "boolean")
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

var firstRetry = 2000;
var maxRetry = 60000;

// A dropped stream (daemon restarting) retries at the base delay; a daemon that
// does not know the topic backs off, so an older daemon is not reconnected every 2 s.
function nextRetry(previous, unknownTopic) {
    if (!unknownTopic)
        return firstRetry;
    return Math.min(maxRetry, Math.max(firstRetry, previous * 2));
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { parseFrame, resolve, nextRetry, firstRetry, maxRetry };
