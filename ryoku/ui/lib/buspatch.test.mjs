import assert from "node:assert/strict";
import test from "node:test";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const { apply } = require("./buspatch.js");

function frame() {
    return {
        language: "en",
        bar: { style: "orpheus", height: 32 },
        theme: { motion: { scale: 1, reduce: false } },
        pinned: ["kitty", "firefox"],
    };
}

test("a set replaces one leaf and copies only its parents", () => {
    const before = frame();
    const snapshot = JSON.stringify(before);
    const after = apply(before, [{ p: ["bar", "style"], v: "kairos" }]);
    assert.equal(after.bar.style, "kairos");
    assert.equal(JSON.stringify(before), snapshot, "the shared frame is never mutated");
    assert.notEqual(after, before);
    assert.notEqual(after.bar, before.bar);
    assert.equal(after.theme, before.theme, "an untouched subtree keeps its identity");
    assert.equal(after.pinned, before.pinned);
});

test("a set creates missing parents", () => {
    const after = apply(frame(), [{ p: ["displays", "ui_scale", "DP-1"], v: 1.25 }]);
    assert.deepEqual(after.displays, { ui_scale: { "DP-1": 1.25 } });
});

test("a set through a non-object replaces it with an object", () => {
    const after = apply(frame(), [{ p: ["language", "code"], v: "fr" }]);
    assert.deepEqual(after.language, { code: "fr" });
});

test("a delete removes the key and leaves the original alone", () => {
    const before = frame();
    const after = apply(before, [{ p: ["theme", "motion", "reduce"], x: true }]);
    assert.deepEqual(after.theme.motion, { scale: 1 });
    assert.equal(before.theme.motion.reduce, false);
    assert.equal(after.bar, before.bar);
});

test("a delete under a missing parent is a no-op", () => {
    const before = frame();
    const after = apply(before, [{ p: ["nope", "deeper"], x: true }]);
    assert.deepEqual(after, before);
    assert.equal("nope" in after, false);
});

test("a top-level delete drops the key", () => {
    const after = apply(frame(), [{ p: ["language"], x: true }]);
    assert.equal("language" in after, false);
});

test("arrays are replaced whole at their path", () => {
    const before = frame();
    const after = apply(before, [{ p: ["pinned"], v: ["foot"] }]);
    assert.deepEqual(after.pinned, ["foot"]);
    assert.deepEqual(before.pinned, ["kitty", "firefox"]);
});

test("several ops under one parent copy it once and all land", () => {
    const before = frame();
    const after = apply(before, [
        { p: ["bar", "style"], v: "kairos" },
        { p: ["bar", "height"], v: 40 },
        { p: ["bar", "gap"], v: 4 },
        { p: ["bar", "style"], x: true },
    ]);
    assert.deepEqual(after.bar, { height: 40, gap: 4 });
    assert.deepEqual(before.bar, { style: "orpheus", height: 32 });
});

test("an object value is stored as sent", () => {
    const v = { scale: 2 };
    const after = apply(frame(), [{ p: ["theme", "motion"], v }]);
    assert.equal(after.theme.motion, v);
});

test("an empty path replaces or clears the whole frame", () => {
    assert.deepEqual(apply(frame(), [{ p: [], v: { a: 1 } }]), { a: 1 });
    assert.deepEqual(apply(frame(), [{ p: [], x: true }]), {});
});

test("no ops returns the same object; malformed ops are skipped", () => {
    const before = frame();
    assert.equal(apply(before, []), before);
    assert.equal(apply(before, null), before);
    const after = apply(before, [{ v: 1 }, null, { p: ["bar", "height"], v: 33 }]);
    assert.equal(after.bar.height, 33);
});
