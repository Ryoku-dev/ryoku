// a drag stages many values a second, only each path's latest one goes out

var DEBOUNCE_MS = 120;
var MAX_WAIT_MS = 400;
// the daemon may clamp a write, so the overlay waits this long for its frame
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
        // move it after any later overlapping write so the daemon sees them in order
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
