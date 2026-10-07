import assert from "node:assert/strict";
import test from "node:test";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const { initialState, applyEvent } = require("./chatstate.js");

// Fold a sequence of frames into one state, the way the bridge feeds the view.
function run(events, start) {
    return events.reduce(applyEvent, start || initialState());
}

test("every user turn is a plain message, including a 'Fix with AI:' one from history", () => {
    const s = run([
        { type: "user_text", text: "hello there" },
        { type: "user_text", text: "Fix with AI: the bar overlaps the dock" }
    ]);
    assert.equal(s.items.length, 2);
    assert.deepEqual(
        { kind: s.items[0].kind, role: s.items[0].role, text: s.items[0].text },
        { kind: "msg", role: "user", text: "hello there" }
    );
    assert.deepEqual(
        { kind: s.items[1].kind, role: s.items[1].role, text: s.items[1].text },
        { kind: "msg", role: "user", text: "Fix with AI: the bar overlaps the dock" }
    );
});

test("agent thought and text chunks coalesce into one open message", () => {
    const s = run([
        { type: "agent_thought", text: "let me " },
        { type: "agent_thought", text: "check the log" },
        { type: "agent_text", text: "The bar " },
        { type: "agent_text", text: "is fine." }
    ]);
    assert.equal(s.items.length, 1);
    assert.equal(s.items[0].thought, "let me check the log");
    assert.equal(s.items[0].text, "The bar is fine.");
    assert.equal(s.items[0].open, true);
});

test("turn_end closes the open message and clears busy while live", () => {
    let s = run([
        { type: "state", state: "busy" },
        { type: "agent_text", text: "done" }
    ]);
    assert.equal(s.busy, true);
    s = applyEvent(s, { type: "turn_end", stopReason: "end_turn" });
    assert.equal(s.items[0].open, false);
    assert.equal(s.busy, false);
    assert.equal(s.activity, "");
});

test("tool frames merge by id and absent fields keep their previous value", () => {
    let s = applyEvent(initialState(), {
        type: "tool", id: "t1", title: "Run", kind: "execute",
        status: "pending", input: "uname -r", auto: true
    });
    // A later frame only carries progress: title/input/auto must survive.
    s = applyEvent(s, { type: "tool", id: "t1", status: "in_progress" });
    // Then the output lands and it completes; output replaces wholesale.
    s = applyEvent(s, { type: "tool", id: "t1", status: "completed", output: "7.2.7-arch1-1\n" });
    assert.equal(s.items.length, 1);
    assert.deepEqual(s.items[0], {
        kind: "tool", id: "t1", title: "Run", kind2: "execute",
        status: "completed", input: "uname -r", output: "7.2.7-arch1-1\n",
        diffs: [], auto: true
    });
});

test("a diff-bearing edit tool keeps its diffs across a status update", () => {
    let s = applyEvent(initialState(), {
        type: "tool", id: "e1", title: "Edit", kind: "edit", status: "pending",
        diffs: [{ path: "a.qml", old: "1", new: "2" }]
    });
    s = applyEvent(s, { type: "tool", id: "e1", status: "completed" });
    assert.deepEqual(s.items[0].diffs, [{ path: "a.qml", old: "1", new: "2" }]);
    assert.equal(s.items[0].status, "completed");
});

test("a permission that arrives before its tool is kept and later matched by toolId", () => {
    // omp asks first, then emits the tool_call for the same id.
    let s = applyEvent(initialState(), {
        type: "permission", requestId: "r1", toolId: "t9", title: "Run rm -rf",
        kind: "execute", input: "rm -rf /tmp/x",
        options: [{ id: "allow", name: "Allow once", kind: "allow_once" }]
    });
    assert.equal(s.permissions.length, 1);
    assert.equal(s.permissions[0].toolId, "t9");
    // The tool row appears afterwards; the permission still points at it.
    s = applyEvent(s, { type: "tool", id: "t9", title: "Run", kind: "execute", status: "pending" });
    assert.equal(toolIdOf(s, "t9") !== -1, true);
    assert.equal(s.permissions[0].toolId, "t9");
});

test("permission_resolved drops exactly that request, whoever answered it", () => {
    let s = run([
        { type: "permission", requestId: "r1", title: "A", options: [] },
        { type: "permission", requestId: "r2", title: "B", options: [] }
    ]);
    assert.equal(s.permissions.length, 2);
    s = applyEvent(s, { type: "permission_resolved", requestId: "r1", outcome: "allowed" });
    assert.equal(s.permissions.length, 1);
    assert.equal(s.permissions[0].requestId, "r2");
});

test("approvals frame carries the mode into state", () => {
    let s = applyEvent(initialState(), { type: "approvals", mode: "read-only" });
    assert.equal(s.approvals, "read-only");
    s = applyEvent(s, { type: "approvals", mode: "ask" });
    assert.equal(s.approvals, "ask");
    s = applyEvent(s, { type: "approvals", mode: "auto" });
    assert.equal(s.approvals, "auto");
    s = applyEvent(s, { type: "approvals", mode: "yolo" });
    assert.equal(s.approvals, "read-only", "an unknown mode is the default");
});

test("replay_start clears the stream and stale approvals, replay_end resumes live", () => {
    let s = run([
        { type: "user_text", text: "old question" },
        { type: "permission", requestId: "r1", title: "stale", options: [] }
    ]);
    assert.equal(s.items.length, 1);
    assert.equal(s.permissions.length, 1);
    s = applyEvent(s, { type: "replay_start" });
    assert.deepEqual(s.items, []);
    assert.deepEqual(s.permissions, []);
    assert.equal(s.replaying, true);
    // A replayed turn_end must not clear a busy flag set before the replay.
    s = applyEvent({ ...s, busy: true }, { type: "turn_end" });
    assert.equal(s.busy, true);
    s = applyEvent(s, { type: "replay_end" });
    assert.equal(s.replaying, false);
});

test("models frame records model and Prowl routing state", () => {
    const s = applyEvent(initialState(), {
        type: "models", agent: "Oh My Pi", current: "anthropic/claude-haiku-4-5",
        models: [{ id: "anthropic/claude-haiku-4-5", name: "Claude Haiku 4.5" }],
        prowl: "pending", reason: "no provider"
    });
    assert.equal(s.agent, "Oh My Pi");
    assert.equal(s.currentModel, "anthropic/claude-haiku-4-5");
    assert.equal(s.models.length, 1);
    assert.equal(s.prowl, "pending");
    assert.equal(s.prowlReason, "no provider");
});

test("applyEvent never mutates the state it was given", () => {
    const a = initialState();
    const b = applyEvent(a, { type: "user_text", text: "hi" });
    assert.equal(a.items.length, 0);
    assert.notEqual(a, b);
});

function toolIdOf(s, id) {
    for (let i = 0; i < s.items.length; i++)
        if (s.items[i].kind === "tool" && s.items[i].id === id) return i;
    return -1;
}

test("text after a tool call lands below it, not in the reply above", () => {
    const s = run([
        { type: "agent_thought", text: "check the log" },
        { type: "tool", id: "t1", title: "journalctl -b", status: "completed" },
        { type: "agent_text", text: "Found it." },
        { type: "turn_end" },
    ]);
    assert.deepEqual(s.items.map((i) => i.kind), ["msg", "tool", "msg"]);
    assert.equal(s.items[0].thought, "check the log");
    assert.equal(s.items[2].text, "Found it.");
    assert.equal(s.items[2].cont, true, "a continuation carries no second header");
    assert.equal(s.items.every((i) => !i.open), true, "turn_end closes every segment");
});

test("thinking after the answer began starts a new segment", () => {
    const s = run([
        { type: "agent_text", text: "step one" },
        { type: "agent_thought", text: "reconsider" },
    ]);
    assert.equal(s.items.length, 2);
    assert.equal(s.items[1].thought, "reconsider");
});
