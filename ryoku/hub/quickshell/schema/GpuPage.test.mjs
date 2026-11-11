import fs from "node:fs";
import vm from "node:vm";
import assert from "node:assert/strict";
import test from "node:test";

const source = fs
    .readFileSync(new URL("./GpuPage.js", import.meta.url), "utf8")
    .replace(/^\.pragma library\s*/, "");
const context = {};
vm.createContext(context);
vm.runInContext(source, context);

const options = available => Array.from(context.graphicsModeOptions(available));
const modeRow = context.rows.find(row => row.label === "Graphics mode");

test("Performance is absent when the GPU policy marks it unavailable", () => {
    assert.deepEqual(options(false), ["Hybrid", "Passthrough"]);
    assert.match(modeRow.gate, /ryoku-gpu status --json/);
    assert.match(modeRow.unavailable, /MUX.+discrete/i);
});

test("Performance remains available on a panel-safe topology", () => {
    assert.deepEqual(options(true), ["Hybrid", "Performance", "Passthrough"]);
});
