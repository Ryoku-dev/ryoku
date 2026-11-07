import assert from "node:assert/strict";
import test from "node:test";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const W = require("./SettingsWrites.js");

// Drive the module the way Settings.qml does: stage on every change, send
// whatever is staged once dueAt() passes.
function drag(state, path, values, stepMs, start) {
    const sent = [];
    let now = start;
    for (const v of values) {
        W.stage(state, path, v, null, now);
        now += stepMs;
        const due = W.dueAt(state);
        if (due >= 0 && due <= now)
            sent.push(...W.take(state).map((w) => w.value));
    }
    return { sent, now };
}

test("a burst on one path sends only its last value after the debounce", () => {
    const s = W.create();
    for (let i = 0; i <= 10; i++)
        W.stage(s, "frameOpacity", i / 10, null, 1000 + i * 10);
    assert.equal(W.dueAt(s), 1100 + W.DEBOUNCE_MS);
    const ws = W.take(s);
    assert.deepEqual(ws.map((w) => [w.path, w.value]), [["frameOpacity", 1]]);
    assert.equal(W.unsent(s), 0);
});

test("a long drag still lands at most every MAX_WAIT_MS", () => {
    const s = W.create();
    const values = Array.from({ length: 100 }, (_, i) => i);
    const { sent } = drag(s, "fontSize", values, 16, 0);
    // 100 moves at 16 ms is 1.6 s of dragging: a write lands while it runs.
    assert.ok(sent.length >= 3, `sent ${sent.length}`);
    assert.ok(sent.length < 10, `sent ${sent.length}`);
    for (let i = 1; i < sent.length; i++)
        assert.ok(sent[i] > sent[i - 1]);
    // The final value is never lost: it is staged or already sent.
    const tail = W.take(s).map((w) => w.value);
    assert.equal(tail.length ? tail[tail.length - 1] : sent[sent.length - 1], 99);
});

test("separate paths each keep one write, in the order they were made", () => {
    const s = W.create();
    W.stage(s, "a", 1, null, 0);
    W.stage(s, "b", 2, null, 1);
    W.stage(s, "a", 3, null, 2);
    assert.deepEqual(W.take(s).map((w) => [w.path, w.value]), [["a", 3], ["b", 2]]);
});

test("re-staging a path behind an overlapping write moves it after that write", () => {
    const s = W.create();
    W.stage(s, "theme.motion.scale", 1, null, 0);
    W.stage(s, "theme.motion", { scale: 2, speed: 1 }, null, 1);
    W.stage(s, "theme.motion.scale", 3, null, 2);
    const ws = W.take(s);
    assert.deepEqual(ws.map((w) => w.path), ["theme.motion", "theme.motion.scale"]);
    assert.deepEqual(W.read(s, {}, "theme.motion"), { scale: 3, speed: 1 });
});

test("reads show staged values at once and survive until the daemon tree agrees", () => {
    const s = W.create();
    const tree = { frameOpacity: 0.4, clipboard: { widthPercent: 50, heightPercent: 40 } };
    W.stage(s, "frameOpacity", 0.9, null, 0);
    W.stage(s, "clipboard.widthPercent", 70, null, 0);
    assert.equal(W.read(s, tree, "frameOpacity"), 0.9);
    assert.deepEqual(W.read(s, tree, "clipboard"), { widthPercent: 70, heightPercent: 40 });
    assert.equal(W.read(s, tree, "clipboard.heightPercent"), 40);
    // The overlay never mutates the daemon tree.
    assert.equal(tree.clipboard.widthPercent, 50);

    const ws = W.take(s);
    // In flight: still the staged value, an older frame must not pull it back.
    assert.equal(W.read(s, tree, "frameOpacity"), 0.9);
    W.frame(s);
    assert.equal(W.read(s, tree, "frameOpacity"), 0.9);

    // Acked and the tree already holds the value: retired.
    const landed = { ...tree, frameOpacity: 0.9 };
    W.ack(s, ws[0].seq, true, landed, 10);
    assert.equal(W.read(s, landed, "frameOpacity"), 0.9);
    assert.equal(W.unacked(s), 1);

    // Acked before its frame: held until the next frame lands.
    W.ack(s, ws[1].seq, true, tree, 10);
    assert.equal(W.read(s, tree, "clipboard.widthPercent"), 70);
    W.frame(s);
    assert.equal(W.read(s, tree, "clipboard.widthPercent"), 50);
    assert.equal(W.unacked(s), 0);
});

test("a rejected write drops its overlay and reports to every coalesced caller", () => {
    const s = W.create();
    const calls = [];
    W.stage(s, "dock", { size: 1 }, (ok) => calls.push(["first", ok]), 0);
    W.stage(s, "dock", { size: 2 }, (ok) => calls.push(["second", ok]), 5);
    const [w] = W.take(s);
    const done = W.ack(s, w.seq, false, { dock: { size: 0 } }, 20);
    for (const c of done) c(false);
    assert.deepEqual(calls, [["first", false], ["second", false]]);
    assert.deepEqual(W.read(s, { dock: { size: 0 } }, "dock"), { size: 0 });
});

test("an acked write the daemon clamped yields to the tree after SETTLE_MS", () => {
    const s = W.create();
    const tree = { fontSize: 40 };
    W.stage(s, "fontSize", 99, null, 0);
    const [w] = W.take(s);
    W.ack(s, w.seq, true, tree, 100);
    assert.equal(W.read(s, tree, "fontSize"), 99);
    let r = W.expire(s, 100 + W.SETTLE_MS - 1);
    assert.equal(r.changed, false);
    assert.equal(r.next, 100 + W.SETTLE_MS);
    r = W.expire(s, 100 + W.SETTLE_MS);
    assert.equal(r.changed, true);
    assert.equal(r.next, -1);
    assert.equal(W.read(s, tree, "fontSize"), 40);
});

test("a newer write in flight keeps the control steady when an older one settles", () => {
    const s = W.create();
    const tree = { frameOpacity: 0.1 };
    W.stage(s, "frameOpacity", 0.5, null, 0);
    const [first] = W.take(s);
    W.stage(s, "frameOpacity", 0.8, null, 500);
    W.take(s);
    W.ack(s, first.seq, true, { frameOpacity: 0.5 }, 510);
    assert.equal(W.read(s, { frameOpacity: 0.5 }, "frameOpacity"), 0.8);
});

test("nothing staged reads straight from the tree and has no deadline", () => {
    const s = W.create();
    const tree = { a: { b: [1, 2] } };
    assert.equal(W.read(s, tree, "a.b"), tree.a.b);
    assert.equal(W.read(s, tree, "a.missing.deep"), undefined);
    assert.equal(W.dueAt(s), -1);
    assert.deepEqual(W.take(s), []);
});
