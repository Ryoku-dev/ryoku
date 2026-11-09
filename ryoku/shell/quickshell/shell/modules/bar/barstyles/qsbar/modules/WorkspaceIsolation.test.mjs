import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import vm from "node:vm";

const source = await readFile(new URL("./WorkspaceIsolation.js", import.meta.url), "utf8");
const module = { exports: {} };
vm.runInNewContext(source.replace(/^\.pragma library\s*/, "") +
    "\nmodule.exports = { activeWorkspace, workspaceList, hasMonitorFocus, markerOpacity };",
    { module, Number, Object });
const { activeWorkspace, workspaceList, hasMonitorFocus, markerOpacity } = module.exports;

const workspaces = [
    { name: "1", output: "eDP-1", active: true, special: false },
    { name: "2", output: "DP-1", active: false, special: false },
    { name: "12", output: "DP-1", active: true, special: false },
    { name: "special:1", output: "eDP-1", active: false, special: true }
];

assert.equal(activeWorkspace(workspaces, "eDP-1", true, workspaces[2]), workspaces[0],
    "each output selects its own active workspace");
assert.equal(activeWorkspace(workspaces, "DP-1", true, workspaces[0]), workspaces[2],
    "the other output selects its own active workspace");
assert.equal(activeWorkspace(workspaces, "eDP-1", false, workspaces[2]), workspaces[2],
    "disabling isolation keeps the globally focused workspace");

assert.deepEqual(Array.from(workspaceList(workspaces, "5", false, "eDP-1", true, workspaces[0])),
    ["1", "3", "4", "5"], "fixed mode filters assigned workspaces and keeps uncreated slots");
assert.deepEqual(Array.from(workspaceList(workspaces, "5", false, "DP-1", true, workspaces[2])),
    ["2", "3", "4", "5", "12"], "fixed mode includes the local active workspace beyond its range");

let focusedOutput = "DP-1";
assert.equal(markerOpacity(hasMonitorFocus(true, "eDP-1", focusedOutput)), 0.72,
    "an active marker dims when another output has focus");
focusedOutput = "eDP-1";
assert.equal(markerOpacity(hasMonitorFocus(true, "eDP-1", focusedOutput)), 1.0,
    "the active marker brightens when focus moves to its output");
