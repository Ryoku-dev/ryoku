import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test from "node:test";
const require = createRequire(import.meta.url);
const { rashinEnabled } = require("./rashin.js");

test("Ask is offered when Rashin is switched on", () => {
  assert.ok(rashinEnabled(JSON.stringify({ version: 2, enabled: true })));
});

test("Ask stays hidden when Rashin is off, never set up, or its file is broken", () => {
  assert.ok(!rashinEnabled(JSON.stringify({ version: 2, enabled: false })));
  assert.ok(!rashinEnabled(""));
  assert.ok(!rashinEnabled("{not json"));
  assert.ok(!rashinEnabled("[]"));
});

test("a Rashin 1 user who opted out does not get Ask", () => {
  assert.ok(rashinEnabled(JSON.stringify({ enabled: true, optedOut: false })));
  assert.ok(!rashinEnabled(JSON.stringify({ enabled: true, optedOut: true })));
});
