import assert from "node:assert/strict";
import test from "node:test";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const { split, join, get, touches } = require("./buspath.js");

test("a dotted path and a key array name the same keys", () => {
    assert.deepEqual(split("bar.style"), ["bar", "style"]);
    assert.deepEqual(split(["bar", "style"]), ["bar", "style"]);
    assert.deepEqual(split(""), []);
    assert.equal(join(["general", "clock_format_24_h"]), "general.clock_format_24_h");
});

test("an array-like key list (a QML sequence) splits like an array", () => {
    const seq = { length: 2, 0: "bar", 1: "style" };
    assert.deepEqual(split(seq), ["bar", "style"]);
    assert.equal(touches("bar.style", { length: 1, 0: seq }), true);
    assert.equal(touches("bar.height", { length: 1, 0: seq }), false);
});

test("get walks objects and falls back on any missing step", () => {
    const root = { bar: { style: "orpheus", off: false, none: null }, list: [1, 2] };
    assert.equal(get(root, "bar.style", "x"), "orpheus");
    assert.equal(get(root, ["bar", "off"], true), false);
    assert.equal(get(root, "bar.none", "x"), null);
    assert.equal(get(root, "bar.missing", "x"), "x");
    assert.equal(get(root, "bar.style.deeper", "x"), "x");
    assert.equal(get(null, "bar", "x"), "x");
    assert.deepEqual(get(root, "list", []), [1, 2]);
    assert.equal(get(root, "", "x"), root);
});

test("a full frame touches every path", () => {
    assert.equal(touches("bar.style", null), true);
    assert.equal(touches("bar.style", undefined), true);
});

test("a change at, inside, or above the path touches it", () => {
    assert.equal(touches("bar.style", [["bar", "style"]]), true);
    assert.equal(touches("bar", [["bar", "style"]]), true);
    assert.equal(touches("bar.style", [["bar"]]), true);
    assert.equal(touches("bar.style", [["theme", "motion"], ["bar", "style"]]), true);
    assert.equal(touches(["bar", "style"], ["bar.style"]), true);
});

test("a sibling or a key sharing a prefix of letters does not", () => {
    assert.equal(touches("bar.style", [["bar", "styles"]]), false);
    assert.equal(touches("bar.style", [["bar", "height"]]), false);
    assert.equal(touches("language", [["languages"]]), false);
    assert.equal(touches("bar.style", []), false);
});

test("the root path is touched by any change", () => {
    assert.equal(touches("", [["anything"]]), true);
});
