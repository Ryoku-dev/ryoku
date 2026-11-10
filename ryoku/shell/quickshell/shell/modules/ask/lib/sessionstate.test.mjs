import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const { initialState, applyEvent } = require("./sessionstate.js");

function fold(events, kind = "agent") {
    return events.reduce((state, event, index) => applyEvent(state, { seq: index + 1, ...event }), initialState(kind));
}
function foldPage(state, events) {
    return events.reduce((current, event) => applyEvent(current, event), state);
}

function equal(actual, expected, label) {
    if (JSON.stringify(actual) !== JSON.stringify(expected)) {
        console.error(label, "\nexpected", expected, "\nactual", actual);
        process.exitCode = 1;
    }
}

const streamed = fold([
    { kind: "turn_start", data: { turn: 1 } },
    { kind: "text", data: { message: "m1", delta: "Hello " } },
    { kind: "text", data: { message: "m1", delta: "world" } },
    { kind: "tool", data: { call: "t1", title: "Read config", status: "running" } },
    { kind: "tool", data: { call: "t1", status: "done", output: "ok" } },
    { kind: "turn_end", data: { turn: 1, stop: "end_turn" } }
]);
equal(streamed.answer, "Hello world", "text deltas form one answer");
equal(streamed.items.filter(item => item.kind === "text").map(item => item.text), ["Hello world"], "text deltas share one transcript row");
equal(streamed.tools, [{ call: "t1", title: "Read config", toolKind: "", status: "done", input: "", output: "ok" }], "tool updates keep earlier fields");
equal([streamed.busy, streamed.phase], [false, "done"], "turn end settles the session");
let paged = foldPage(initialState("agent"), [
    { seq: 1, kind: "turn_start", data: { turn: 1 } },
    { seq: 2, kind: "text", data: { message: "m1", delta: "Page one " } }
]);
paged = foldPage(paged, [
    { seq: 3, kind: "text", data: { message: "m1", delta: "then page two" } },
    { seq: 4, kind: "turn_end", data: { turn: 1, stop: "end_turn" } }
]);
equal(paged.answer, "Page one then page two", "attach pages fold into one ordered transcript");
equal(paged.seq, 4, "attach page folding keeps the final sequence");

const permission = fold([
    { kind: "turn_start", data: {} },
    { kind: "permission", data: { request: "p1", title: "Edit file", options: [{ id: "once", kind: "allow_once" }] } },
    { kind: "permission_resolved", data: { request: "p1", option: "once", outcome: "selected" } }
]);
equal(permission.permissions, [], "resolved permissions leave the prompt");
equal(permission.phase, "working", "a resolved permission resumes the turn");

const ask = fold([
    { kind: "user", data: { text: "What changed?" } },
    { kind: "text", data: { message: "a1", delta: "Two files." } },
    { kind: "answer", data: { actions: [{ kind: "copy", label: "Copy", value: "Two files." }], needsAgent: true, reason: "A write is needed", via: "auto/free" } }
], "ask");
equal([ask.question, ask.answer, ask.needsAgent, ask.needsAgentReason, ask.via], ["What changed?", "Two files.", true, "A write is needed", "auto/free"], "Ask answer metadata stays with streamed text");
