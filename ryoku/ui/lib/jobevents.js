// The progress lines `gpk ... --events` and `ryoku-host apps` print, one JSON object a line:
// plan, step start, log, progress, step done, done. PackageJob reads them through this, for
// Settings and RyoStore alike; jobevents.test.mjs holds it.

function initial() {
    return { plan: [], index: -1, phase: "", stepsDone: 0, done: 0, total: 0, finished: false,
             ok: false, exit: -1, error: "", lastLog: "" };
}

// one line of output, or null when it is not an event
function parse(line) {
    var text = String(line || "").trim();
    if (text === "" || text.charAt(0) !== "{")
        return null;
    try {
        var ev = JSON.parse(text);
        return ev && typeof ev.event === "string" ? ev : null;
    } catch (e) {
        return null;
    }
}

// the state after one event; the old state is never changed
function reduce(state, ev) {
    var s = Object.assign({}, state);
    if (!ev)
        return s;
    switch (ev.event) {
    case "plan":
        s.plan = Array.isArray(ev.steps) ? ev.steps.map(x => ({
            manager: String(x.manager || ""), name: String(x.name || ""), version: String(x.version || ""),
            requiredBy: Array.isArray(x.required_by) ? x.required_by.map(String) : []
        })) : [];
        break;
    case "step":
        var index = Number(ev.index);
        if (isNaN(index))
            break;
        if (ev.phase === "start" && index !== s.index) {
            s.done = 0;
            s.total = 0;
        }
        s.index = index;
        s.phase = String(ev.phase || "");
        if (ev.phase === "done" && Number(ev.exit) === 0) {
            s.stepsDone = Math.max(s.stepsDone, index + 1);
            s.done = s.total;
        }
        break;
    case "progress":
        var total = Number(ev.total), done = Number(ev.done);
        if (total > 0 && done >= 0) {
            s.total = total;
            s.done = Math.min(done, total);
        }
        break;
    case "log":
        var line = String(ev.line || "").trim();
        if (line !== "")
            s.lastLog = line;
        break;
    case "done":
        s.finished = true;
        s.ok = ev.ok === true;
        s.exit = Number(ev.exit || 0);
        s.error = String(ev.error || "");
        break;
    }
    return s;
}

// how far the work is across the whole plan, 0..1, or -1 while nothing counts it
function fraction(s) {
    if (s.finished && s.ok)
        return 1;
    var steps = Math.max(1, s.plan.length);
    if (s.total > 0)
        return Math.max(0, Math.min(1, (Math.max(0, s.index) + s.done / s.total) / steps));
    if (s.stepsDone > 0)
        return Math.min(1, s.stepsDone / steps);
    return -1;
}

// the package the job is on, from its plan
function stepName(s) {
    var step = s.index >= 0 && s.index < s.plan.length ? s.plan[s.index] : null;
    return step ? step.name : "";
}

// a job that stops by a line on its stdin (ryoku-host apps --stdin-cancel) rather than a signal
function cancelsByStdin(argv) {
    return Array.isArray(argv) && argv.indexOf("--stdin-cancel") >= 0;
}

// whether a running job can be stopped from here: pkexec's child is root, so only one that
// listens on its stdin can
function cancellable(argv) {
    return Array.isArray(argv) && argv.length > 0 && (String(argv[0]) !== "pkexec" || cancelsByStdin(argv));
}

// the last few kilobytes of a stream, so a long transaction's output never grows without end
function tail(text, more, limit) {
    var next = String(text || "") + String(more || "");
    var max = limit || 4096;
    return next.length > max ? next.slice(next.length - max) : next;
}

// why a job ended, as a kind the job puts into words, plus the tool's own text when it said any
function failure(argv, code, s, stderr, cancelled) {
    if (cancelled)
        return { kind: "cancelled", text: "" };
    var first = Array.isArray(argv) && argv.length > 0 ? String(argv[0]) : "";
    // pkexec's own codes, only when the helper never reported an end itself
    if (first === "pkexec" && !(s && s.finished) && code === 126)
        return { kind: "dismissed", text: "" };
    if (first === "pkexec" && !(s && s.finished) && code === 127)
        return { kind: "denied", text: "" };
    if (s && s.error !== "")
        return { kind: "message", text: s.error };
    var lines = String(stderr || "").trim().split("\n").filter(l => l.trim() !== "");
    if (lines.length > 0)
        return { kind: "message", text: lines[lines.length - 1].trim() };
    if (s && s.lastLog !== "")
        return { kind: "message", text: s.lastLog };
    return { kind: "fallback", text: "" };
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { initial, parse, reduce, fraction, stepName, cancelsByStdin, cancellable, tail, failure };
