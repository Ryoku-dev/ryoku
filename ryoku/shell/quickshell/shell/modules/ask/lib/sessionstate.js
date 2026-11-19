// One Quick Ask conversation folded from Rashin's session events. Ask turns are
// read-only, so events only an agent turn sends (permissions, plans, models,
// thoughts) pass through without a trace.

function initialState() {
    return {
        id: "",
        seq: 0,
        phase: "idle",
        busy: false,
        question: "",
        answer: "",
        actions: [],
        needsAgent: false,
        needsAgentReason: "",
        via: "",
        error: "",
        tools: []
    };
}

function clone(state) {
    var next = {};
    for (var key in state)
        next[key] = state[key];
    return next;
}

function replaceBy(items, key, value, merge) {
    var out = items.slice();
    var index = -1;
    for (var i = 0; i < out.length; i++) {
        if (String(out[i][key] || "") === String(value || "")) {
            index = i;
            break;
        }
    }
    if (index < 0) {
        out.push(merge);
        return out;
    }
    var row = clone(out[index]);
    for (var field in merge) {
        if (merge[field] !== undefined && merge[field] !== "")
            row[field] = merge[field];
    }
    out[index] = row;
    return out;
}

function applyEvent(state, event) {
    var next = clone(state);
    var data = event && event.data ? event.data : {};
    var kind = String(event && event.kind || "");
    var seq = Number(event && event.seq || 0);
    next.seq = Math.max(Number(state.seq || 0), seq);

    if (kind === "user") {
        next.question = String(data.text || state.question || "");
        next.error = "";
    } else if (kind === "text") {
        next.answer = String(state.answer || "") + String(data.delta || "");
    } else if (kind === "tool") {
        next.tools = replaceBy(state.tools, "call", data.call, {
            call: String(data.call || ""),
            title: String(data.title || ""),
            toolKind: String(data.kind || ""),
            status: String(data.status || ""),
            input: String(data.input || ""),
            output: String(data.output || "")
        });
    } else if (kind === "turn_start") {
        next.busy = true;
        next.phase = "working";
        next.error = "";
    } else if (kind === "turn_end") {
        next.busy = false;
        next.phase = String(data.stop || "") === "cancelled" ? "cancelled" : "done";
    } else if (kind === "error") {
        next.error = String(data.message || "");
        next.busy = false;
        next.phase = "failed";
    } else if (kind === "answer") {
        next.actions = data.actions || [];
        next.needsAgent = data.needsAgent === true;
        next.needsAgentReason = String(data.reason || "");
        next.via = String(data.via || "");
    }
    return next;
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { initialState: initialState, applyEvent: applyEvent };
