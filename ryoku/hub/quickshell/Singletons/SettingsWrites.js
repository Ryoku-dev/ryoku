// Coalesced settings.patch writes for Singletons/Settings.qml. A slider or drag
// stages many values per second; each path keeps only its latest value and is
// sent after a quiet spell (debounce) or, during a long drag, at most maxWait
// after its first unsent value. Staged and in-flight values overlay the daemon
// tree on read so a control never jumps back while its write is pending.

var DEBOUNCE_MS = 120;
var MAX_WAIT_MS = 400;
// After an acknowledged write whose frame has not matched yet (the daemon may
// clamp it), the overlay waits this long for the frame, then yields to the tree.
var SETTLE_MS = 1000;

function create() {
    return { pending: [], inflight: [], nextSeq: 0 };
}

function overlaps(a, b) {
    return a === b || b.indexOf(a + ".") === 0 || a.indexOf(b + ".") === 0;
}

function clone(v) {
    return v === undefined ? undefined : JSON.parse(JSON.stringify(v));
}

function same(a, b) {
    return JSON.stringify(a) === JSON.stringify(b);
}

function getIn(obj, parts) {
    var cur = obj;
    for (var i = 0; i < parts.length; i++) {
        if (cur === undefined || cur === null || typeof cur !== "object")
            return undefined;
        cur = cur[parts[i]];
    }
    return cur;
}

function splitPath(path) {
    var s = String(path);
    return s === "" ? [] : s.split(".");
}

// Stage a value for path. Returns nothing; the caller schedules dueAt().
function stage(state, path, value, completion, now) {
    var p = String(path);
    var idx = -1;
    for (var i = 0; i < state.pending.length; i++) {
        if (state.pending[i].path === p) {
            idx = i;
            break;
        }
    }
    var entry;
    if (idx >= 0) {
        entry = state.pending[idx];
        // A later overlapping write would land after this one; move it to the
        // end so the daemon sees the writes in the order they were made.
        var overlapped = false;
        for (var j = idx + 1; j < state.pending.length; j++) {
            if (overlaps(state.pending[j].path, p)) {
                overlapped = true;
                break;
            }
        }
        if (overlapped) {
            state.pending.splice(idx, 1);
            state.pending.push(entry);
        }
    } else {
        entry = { path: p, value: undefined, completions: [], firstAt: now, lastAt: now };
        state.pending.push(entry);
    }
    entry.value = clone(value);
    entry.lastAt = now;
    if (typeof completion === "function")
        entry.completions.push(completion);
}

// Earliest time a staged write must be sent, or -1 when nothing is staged.
function dueAt(state) {
    var due = -1;
    for (var i = 0; i < state.pending.length; i++) {
        var e = state.pending[i];
        var t = Math.min(e.lastAt + DEBOUNCE_MS, e.firstAt + MAX_WAIT_MS);
        if (due < 0 || t < due)
            due = t;
    }
    return due;
}

// Move every staged write in flight, in staging order. Each returned write
// carries a seq for ack(); the caller sends { path, value }.
function take(state) {
    var out = [];
    for (var i = 0; i < state.pending.length; i++) {
        var e = state.pending[i];
        var w = { seq: ++state.nextSeq, path: e.path, value: e.value, completions: e.completions, acked: false, retireAt: 0 };
        state.inflight.push(w);
        out.push(w);
    }
    state.pending = [];
    return out;
}

function removeInflight(state, seq) {
    for (var i = 0; i < state.inflight.length; i++) {
        if (state.inflight[i].seq === seq)
            return state.inflight.splice(i, 1)[0];
    }
    return null;
}

// The daemon answered write seq. A failed write drops its overlay at once so
// the control shows the daemon's value; an accepted one stays until the tree
// agrees or SETTLE_MS passes. Returns the completions to call.
function ack(state, seq, ok, data, now) {
    var w = null;
    for (var i = 0; i < state.inflight.length; i++) {
        if (state.inflight[i].seq === seq) {
            w = state.inflight[i];
            break;
        }
    }
    if (!w)
        return [];
    if (!ok || same(getIn(data, splitPath(w.path)), w.value))
        removeInflight(state, seq);
    else {
        w.acked = true;
        w.retireAt = now + SETTLE_MS;
    }
    return w.completions;
}

// A new daemon tree landed: acknowledged writes are settled now.
function frame(state) {
    var kept = [];
    for (var i = 0; i < state.inflight.length; i++) {
        if (!state.inflight[i].acked)
            kept.push(state.inflight[i]);
    }
    var changed = kept.length !== state.inflight.length;
    state.inflight = kept;
    return changed;
}

// Drop acknowledged overlays whose settle window passed. Returns whether any
// overlay changed, and the next retire time (-1 when none).
function expire(state, now) {
    var kept = [];
    var next = -1;
    for (var i = 0; i < state.inflight.length; i++) {
        var w = state.inflight[i];
        if (w.acked && w.retireAt <= now)
            continue;
        kept.push(w);
        if (w.acked && (next < 0 || w.retireAt < next))
            next = w.retireAt;
    }
    var changed = kept.length !== state.inflight.length;
    state.inflight = kept;
    return { changed: changed, next: next };
}

function unsent(state) {
    return state.pending.length;
}

function unacked(state) {
    var n = 0;
    for (var i = 0; i < state.inflight.length; i++) {
        if (!state.inflight[i].acked)
            n++;
    }
    return n;
}

// Read path from data with every in-flight then staged write laid over it, in
// the order they were made.
function read(state, data, path) {
    var parts = splitPath(path);
    var p = parts.join(".");
    var value = getIn(data, parts);
    var writes = state.inflight.concat(state.pending);
    var copied = false;
    for (var i = 0; i < writes.length; i++) {
        var w = writes[i];
        if (!overlaps(w.path, p))
            continue;
        if (w.path === p) {
            value = clone(w.value);
            copied = true;
        } else if (p.indexOf(w.path + ".") === 0) {
            value = clone(getIn(w.value, splitPath(p.slice(w.path.length + 1))));
            copied = true;
        } else {
            var rest = splitPath(w.path.slice(p.length + 1));
            if (!copied) {
                value = clone(value);
                copied = true;
            }
            if (value === undefined || value === null || typeof value !== "object")
                value = {};
            var cur = value;
            for (var k = 0; k < rest.length - 1; k++) {
                var next = cur[rest[k]];
                if (next === undefined || next === null || typeof next !== "object")
                    cur[rest[k]] = next = {};
                cur = next;
            }
            cur[rest[rest.length - 1]] = clone(w.value);
        }
    }
    return value;
}

if (typeof module !== "undefined" && module.exports)
    module.exports = {
        DEBOUNCE_MS, MAX_WAIT_MS, SETTLE_MS,
        create, stage, dueAt, take, ack, frame, expire, unsent, unacked, read
    };
