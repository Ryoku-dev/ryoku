import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test from "node:test";
const require = createRequire(import.meta.url);
const { editScript, apply } = require("./listdiff.js");

test("typing a letter keeps the rows that still match and adds the new ones", () => {
  const before = ["firefox", "files", "foot", "fish"];
  const after = ["firefox", "files", "filelight"];
  const ops = editScript(before, after);
  assert.deepEqual(apply(before, ops), after);
  assert.ok(!ops.some(op => op.op === "insert" && (op.key === "firefox" || op.key === "files")),
    "a row that stays is never rebuilt");
  assert.equal(ops.filter(op => op.op === "insert").length, 1);
});

test("a reordered list moves rows instead of rebuilding them", () => {
  const before = ["a", "b", "c", "d"];
  const after = ["d", "a", "c", "b"];
  const ops = editScript(before, after);
  assert.deepEqual(apply(before, ops), after);
  assert.ok(ops.every(op => op.op === "move"));
});

test("an unchanged list needs no edits", () => {
  assert.deepEqual(editScript(["a", "b"], ["a", "b"]), []);
});

test("any list reaches any other, duplicates and empties included", () => {
  let seed = 7;
  const random = () => {
    seed = (seed * 1103515245 + 12345) % 2147483648;
    return seed / 2147483648;
  };
  const pool = ["a", "b", "c", "d", "e", "f", "g"];
  const make = () => Array.from({ length: Math.floor(random() * 8) },
    () => pool[Math.floor(random() * pool.length)]);
  for (let i = 0; i < 400; i++) {
    const before = make();
    const after = make();
    assert.deepEqual(apply(before, editScript(before, after)), after, JSON.stringify({ before, after }));
  }
});
