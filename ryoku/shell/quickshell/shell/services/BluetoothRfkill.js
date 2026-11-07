// BluetoothPower.qml and its regression tests share this policy.

// Entries are { pending, type, soft, hard } read from /sys/class/rfkill/rfkill*.
// Returns null while any entry is still loading, so a rescan never flashes a stale value.
function blockState(entries) {
    var soft = false;
    var hard = false;
    for (var i = 0; i < entries.length; i++) {
        var entry = entries[i];
        if (!entry || entry.pending > 0)
            return null;
        if (entry.type !== "bluetooth")
            continue;
        soft = soft || entry.soft === "1";
        hard = hard || entry.hard === "1";
    }
    return { soft: soft, hard: hard };
}

// wanted: -1 no choice made, 0 off, 1 on.
function settled(wanted, enabled, blocked) {
    if (wanted < 0)
        return true;
    return wanted === 1 ? enabled && !blocked : !enabled;
}

var firstRecheck = 250;
var lastRecheck = 8000;

// Delay before the next re-check after one at `interval`, or 0 once the schedule is spent.
function nextRecheck(interval) {
    return interval < lastRecheck ? interval * 2 : 0;
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { blockState, settled, nextRecheck, firstRecheck, lastRecheck };
