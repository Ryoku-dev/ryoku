import fs from "node:fs";
import vm from "node:vm";
import assert from "node:assert/strict";
import test from "node:test";

const source = fs.readFileSync(new URL("./OutputScope.js", import.meta.url), "utf8").replace(/^\.pragma library\s*$/m, "");
const sandbox = { module: { exports: {} } };
vm.runInNewContext(source, sandbox);
const Scope = sandbox.module.exports;

test("linked: only the first output tab is active, and every write targets the global scope", () => {
    const outs = ["eDP-1", "DP-1"];
    assert.deepEqual(Scope.activeOutputs(outs, true), ["eDP-1"]);
    assert.equal(Scope.writeScope(outs, true, "DP-1"), "");
    assert.equal(Scope.readScope(outs, true, "DP-1"), "");
});

test("unlinked: every output tab is active and reads and writes its own scope", () => {
    const outs = ["eDP-1", "DP-1"];
    assert.deepEqual(Scope.activeOutputs(outs, false), ["eDP-1", "DP-1"]);
    assert.equal(Scope.writeScope(outs, false, "DP-1"), "DP-1");
    assert.equal(Scope.readScope(outs, false, "DP-1"), "DP-1");
});

test("the link toggle is only offered with two or more outputs", () => {
    assert.equal(Scope.showLink(["eDP-1"]), false);
    assert.equal(Scope.showLink(["eDP-1", "DP-1"]), true);
});

test("a selected output that disappears falls back to the first output", () => {
    assert.equal(Scope.writeScope(["eDP-1"], false, "DP-1"), "eDP-1");
});

test("toggling the link flips the state", () => {
    assert.equal(Scope.toggleLink(true), false);
    assert.equal(Scope.toggleLink(false), true);
});

test("linked edits keep the global key", () => {
    assert.equal(Scope.editKey(true, "DP-1", "desktop.appearance.layout"), "desktop.appearance.layout");
});

test("unlinked edits of a per-output setting write the output's own key", () => {
    assert.equal(Scope.editKey(false, "DP-1", "desktop.appearance.layout"), "desktop.outputs.DP-1.appearance.layout");
});

test("unlinked edits of a setting that is not per-output stay global", () => {
    assert.equal(Scope.editKey(false, "DP-1", "desktop.appearance.gapsIn"), "desktop.appearance.gapsIn");
});

test("a per-output read falls back to the global value until the output has its own", () => {
    const draft = { "desktop.appearance.layout": "dwindle" };
    assert.equal(Scope.readValue(draft, false, "DP-1", "desktop.appearance.layout"), "dwindle");
    draft["desktop.outputs.DP-1.appearance.layout"] = "scrolling";
    assert.equal(Scope.readValue(draft, false, "DP-1", "desktop.appearance.layout"), "scrolling");
});
