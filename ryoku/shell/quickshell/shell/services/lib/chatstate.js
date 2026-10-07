// The pure protocol-v3 reducer behind the Super+S chat. It owns every piece of
// transcript state so the QML view is a projection of it and the daemon bridge
// only feeds it frames. applyEvent never mutates its input; it returns a fresh
// state the view reconciles against, so the node tests and QML share one source
// of truth. It handles the v3 stream: tool partial merges (input, output,
// diffs, auto), permissions keyed by requestId and dropped on
// permission_resolved, and an approvals mode.

function initialState() {
    return {
        items: [], // ordered stream: {kind:'msg'|'tool', ...}
        permissions: [], // pending approvals, keyed by requestId
        banner: { state: "starting", error: "" },
        approvals: "read-only", // what runs unasked: "ask" nothing, "read-only" reads, "auto" everything
        busy: false,
        seq: 0,
        models: [],
        currentModel: "",
        agent: "",
        prowl: "",
        prowlReason: "",
        commands: [],
        session: { id: "", title: "" },
        usage: null,
        history: [],
        replaying: false,
        activity: "" // what the agent is doing right now ("" = idle)
    };
}

// activityFor derives the working-strip label from a live frame.
function activityFor(ev, state) {
    switch (ev.type) {
    case "tool":
        if (ev.status === "pending" || ev.status === "in_progress")
            return ev.title || "running a tool";
        return state.activity;
    case "agent_thought":
        return "thinking";
    case "agent_text":
        return "writing";
    case "permission":
        return "waiting for your approval";
    default:
        return state.activity;
    }
}

// A reply is written in segments: text and thinking that arrive after a tool
// call start a new segment below it, so the stream reads in the order the
// agent worked. cont marks a segment that continues the same reply.
function openTail(items) {
    var tail = items[items.length - 1];
    return tail && tail.kind === "msg" && tail.role === "agent" && tail.open ? tail : null;
}

function pushUser(s, state, text, images) {
    s.items = state.items.concat({
        kind: "msg", role: "user", text: text || "", thought: "", cont: false,
        open: false, failed: false, images: images || [], id: "u" + (++s.seq)
    });
}

// appendToSegment adds text to the open tail segment when canJoin allows it,
// and otherwise opens a new segment below whatever came last.
function appendToSegment(s, state, field, text, canJoin) {
    var tail = openTail(state.items);
    if (tail && canJoin(tail)) {
        var joined = {};
        joined[field] = tail[field] + text;
        s.items = state.items.slice(0, -1).concat(Object.assign({}, tail, joined));
        return s;
    }
    var prev = state.items[state.items.length - 1];
    var seg = {
        kind: "msg", role: "agent", text: "", thought: "", open: true, failed: false, images: [],
        cont: !!prev && (prev.kind === "tool" || (prev.kind === "msg" && prev.role === "agent")),
        id: "a" + (++s.seq)
    };
    seg[field] = text;
    s.items = state.items.concat(seg);
    return s;
}

function toolIndex(items, id) {
    for (var i = 0; i < items.length; i++)
        if (items[i].kind === "tool" && items[i].id === id)
            return i;
    return -1;
}

// applyEvent(state, ev) -> next state. ev is a daemon->client frame.
function applyEvent(state, ev) {
    var s = Object.assign({}, state);
    if (!state.replaying)
        s.activity = activityFor(ev, state);
    switch (ev.type) {
    case "user":
    case "user_text":
        pushUser(s, state, ev.text || "", ev.images || []);
        return s;
    case "agent_text":
        return appendToSegment(s, state, "text", ev.text || "", function () { return true; });
    case "agent_thought":
        // Thinking after the answer began opens a fresh segment below it.
        return appendToSegment(s, state, "thought", ev.text || "", function (tail) { return tail.text === ""; });
    case "tool": {
        var at = toolIndex(state.items, ev.id);
        if (at === -1) {
            s.items = state.items.concat({
                kind: "tool", id: ev.id, title: ev.title || "", kind2: ev.kind || "",
                status: ev.status || "pending", input: ev.input || "", output: ev.output || "",
                diffs: ev.diffs || [], auto: ev.auto === true
            });
            return s;
        }
        s.items = state.items.slice();
        var prev = s.items[at];
        // A partial update: absent fields keep their previous value.
        s.items[at] = Object.assign({}, prev, {
            title: ev.title != null ? ev.title : prev.title,
            kind2: ev.kind != null ? ev.kind : prev.kind2,
            status: ev.status != null ? ev.status : prev.status,
            input: ev.input != null ? ev.input : prev.input,
            output: ev.output != null ? ev.output : prev.output,
            diffs: ev.diffs != null ? ev.diffs : prev.diffs,
            auto: ev.auto != null ? (ev.auto === true) : prev.auto
        });
        return s;
    }
    case "permission": {
        // A repeat of a requestId replaces the stale entry rather than stacking.
        var kept = state.permissions.filter(function (p) { return p.requestId !== ev.requestId; });
        s.permissions = kept.concat({
            requestId: ev.requestId, toolId: ev.toolId || "", title: ev.title || "",
            kind: ev.kind || "", input: ev.input || "", options: ev.options || []
        });
        return s;
    }
    case "permission_resolved":
        s.permissions = state.permissions.filter(function (p) { return p.requestId !== ev.requestId; });
        return s;
    case "approvals":
        s.approvals = ev.mode === "ask" || ev.mode === "auto" ? ev.mode : "read-only";
        return s;
    case "turn_end": {
        var anyOpen = state.items.some(function (it) { return it.open; });
        if (anyOpen)
            s.items = state.items.map(function (it) { return it.open ? Object.assign({}, it, { open: false }) : it; });
        // A replayed turn_end is historical; it must not clear the live busy flag.
        if (!state.replaying) {
            s.busy = false;
            s.activity = "";
        }
        return s;
    }
    case "state":
        s.banner = { state: ev.state || "", error: ev.error || "" };
        s.busy = ev.state === "busy" || ev.state === "starting";
        if (!s.busy)
            s.activity = "";
        return s;
    case "models":
        s.models = ev.models || [];
        s.currentModel = ev.current || "";
        s.agent = ev.agent || "";
        s.prowl = ev.prowl || "";
        s.prowlReason = ev.reason || "";
        return s;
    case "commands":
        s.commands = ev.commands || [];
        return s;
    case "session_info":
        s.session = { id: ev.sessionId || "", title: ev.title || "" };
        return s;
    case "usage":
        s.usage = { size: ev.size || 0, used: ev.used || 0 };
        return s;
    case "history":
        s.history = ev.sessions || [];
        return s;
    case "replay_start":
        // A load, a /new or a fresh join replays the whole transcript: clear the
        // stream and any stale approvals before the historical frames arrive.
        s.items = [];
        s.permissions = [];
        s.replaying = true;
        return s;
    case "replay_end":
        s.replaying = false;
        return s;
    default:
        return state;
    }
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { initialState: initialState, applyEvent: applyEvent };
