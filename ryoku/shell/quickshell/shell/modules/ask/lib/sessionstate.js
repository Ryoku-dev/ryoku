function initialState(kind) {
    return {
        kind: kind || "ask",
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
        items: [],
        tools: [],
        permissions: [],
        plan: [],
        model: "",
        models: []
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

function appendDelta(items, kind, message, delta) {
    var out = items.slice();
    var index = -1;
    for (var i = out.length - 1; i >= 0; i--) {
        if (out[i].kind === kind && String(out[i].message || "") === String(message || "")) {
            index = i;
            break;
        }
    }
    if (index < 0) {
        out.push({ kind: kind, message: String(message || ""), text: String(delta || "") });
        return out;
    }
    var row = clone(out[index]);
    row.text = String(row.text || "") + String(delta || "");
    out[index] = row;
    return out;
}

function removePermission(items, request) {
    return items.filter(function(item) { return String(item.request || "") !== String(request || ""); });
}

function applyEvent(state, event) {
    var next = clone(state);
    var data = event && event.data ? event.data : {};
    var kind = String(event && event.kind || "");
    var seq = Number(event && event.seq || 0);
    next.seq = Math.max(Number(state.seq || 0), seq);

    if (kind === "user") {
        next.question = String(data.text || state.question || "");
        next.items = state.items.concat({
            kind: "user",
            text: String(data.text || ""),
            images: data.images || [],
            handoff: data.handoff || null
        });
        next.error = "";
    } else if (kind === "text") {
        next.answer = String(state.answer || "") + String(data.delta || "");
        next.items = appendDelta(state.items, "text", data.message, data.delta);
    } else if (kind === "thought") {
        next.items = appendDelta(state.items, "thought", data.message, data.delta);
    } else if (kind === "tool") {
        var tool = {
            call: String(data.call || ""),
            title: String(data.title || ""),
            toolKind: String(data.kind || ""),
            status: String(data.status || ""),
            input: String(data.input || ""),
            output: String(data.output || "")
        };
        next.tools = replaceBy(state.tools, "call", data.call, tool);
        next.items = replaceBy(state.items, "call", data.call, {
            kind: "tool",
            call: tool.call,
            title: tool.title,
            toolKind: tool.toolKind,
            status: tool.status,
            input: tool.input,
            output: tool.output
        });
    } else if (kind === "permission") {
        next.permissions = replaceBy(state.permissions, "request", data.request, {
            request: String(data.request || ""),
            call: String(data.call || ""),
            title: String(data.title || ""),
            kind: String(data.kind || ""),
            input: String(data.input || ""),
            options: data.options || []
        });
        next.phase = "waiting";
    } else if (kind === "permission_resolved") {
        next.permissions = removePermission(state.permissions, data.request);
        if (next.permissions.length === 0 && state.busy)
            next.phase = "working";
    } else if (kind === "plan") {
        next.plan = data.entries || [];
    } else if (kind === "turn_start") {
        next.busy = true;
        next.phase = "working";
        next.error = "";
    } else if (kind === "turn_end") {
        next.busy = false;
        next.permissions = [];
        next.phase = String(data.stop || "") === "cancelled" ? "cancelled" : "done";
    } else if (kind === "error") {
        next.error = String(data.message || "");
        next.busy = false;
        next.phase = "failed";
    } else if (kind === "model") {
        next.model = String(data.current || "");
        next.models = data.options || [];
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
