import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test from "node:test";
const require = createRequire(import.meta.url);
const { KEY, MOD, PAGE, intentFor } = require("./keys.js");

const list = { queryEmpty: false, modeCount: 5 };
const kind = (key, mods, state) => (intentFor(key, mods || 0, state || list) || {}).kind || null;

test("Escape steps back: actions, then the query, then closes", () => {
  assert.equal(kind(KEY.Escape, 0, { actionsFocused: true, queryEmpty: false }), "leaveActions");
  assert.equal(kind(KEY.Escape, 0, { queryEmpty: false }), "clear");
  assert.equal(kind(KEY.Escape, 0, { queryEmpty: true }), "close");
});

test("arrows and the Emacs and vim chords move the selection", () => {
  assert.deepEqual(intentFor(KEY.Down, 0, list), { kind: "move", delta: 1 });
  assert.deepEqual(intentFor(KEY.Up, 0, list), { kind: "move", delta: -1 });
  assert.deepEqual(intentFor(KEY.J, MOD.Control, list), { kind: "move", delta: 1 });
  assert.deepEqual(intentFor(KEY.K, MOD.Control, list), { kind: "move", delta: -1 });
  assert.deepEqual(intentFor(KEY.N, MOD.Control, list), { kind: "move", delta: 1 });
  assert.deepEqual(intentFor(KEY.P, MOD.Control, list), { kind: "move", delta: -1 });
  assert.equal(kind(KEY.J, 0), null, "a bare letter is typed into the field");
});

test("page keys jump five rows", () => {
  assert.deepEqual(intentFor(KEY.PageDown, 0, list), { kind: "move", delta: PAGE });
  assert.equal(PAGE, 5);
});

test("Tab enters the actions where arrows and Enter then work", () => {
  assert.equal(kind(KEY.Tab), "enterActions");
  const actions = { actionsFocused: true, queryEmpty: false };
  assert.equal(kind(KEY.Down, 0, actions), "moveAction");
  assert.equal(kind(KEY.Return, 0, actions), "runAction");
  assert.equal(kind(KEY.Backtab, MOD.Shift, actions), "leaveActions");
});

test("Enter runs the selected row", () => {
  assert.equal(kind(KEY.Return), "run");
  assert.equal(kind(KEY.Enter), "run");
});

test("Shift+Delete kills only when a window is the target, and plain Delete stays with the text", () => {
  assert.equal(kind(KEY.Delete, MOD.Shift, { canKill: true }), "kill");
  assert.equal(kind(KEY.Delete, MOD.Shift, { canKill: false }), null);
  assert.equal(kind(KEY.Delete, 0, { canKill: true }), null);
});

test("Right and End take the ghost only at the end of the text", () => {
  assert.equal(kind(KEY.Right, 0, { ghost: true, caretAtEnd: true }), "acceptGhost");
  assert.equal(kind(KEY.End, 0, { ghost: true, caretAtEnd: true }), "acceptGhost");
  assert.equal(kind(KEY.Right, 0, { ghost: true, caretAtEnd: false }), null);
  assert.equal(kind(KEY.Right, 0, { ghost: false, caretAtEnd: true }), null);
});

test("Alt+N runs row N and Ctrl+N picks a tab that exists", () => {
  assert.deepEqual(intentFor(KEY.Digit1 + 2, MOD.Alt, list), { kind: "runIndex", n: 3 });
  assert.deepEqual(intentFor(KEY.Digit1 + 1, MOD.Control, list), { kind: "mode", n: 2 });
  assert.equal(kind(KEY.Digit1 + 5, MOD.Control, list), null, "there is no sixth tab");
  assert.equal(kind(KEY.Digit1, 0), null, "a bare digit is typed");
});
