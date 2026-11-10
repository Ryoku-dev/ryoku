import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import vm from "node:vm";

const source = await readFile(new URL("./WorkspaceIsolation.js", import.meta.url), "utf8");
const module = { exports: {} };
vm.runInNewContext(source.replace(/^\.pragma library\s*/, "") +
    "\nmodule.exports = { activeWorkspace, workspaceOn, needsWorkspaceData, workspaceList, extraWorkspace, hasMonitorFocus, markerOpacity };",
    { module, Number, Object });
const { activeWorkspace, workspaceOn, needsWorkspaceData, workspaceList, extraWorkspace, hasMonitorFocus, markerOpacity } = module.exports;

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

assert.equal(needsWorkspaceData("5", false, false), false,
    "fixed mode without isolation does not track live workspace data");
assert.equal(needsWorkspaceData("5", false, true), true,
    "fixed mode needs live workspace data to filter monitor assignments");
assert.equal(needsWorkspaceData("active", false, false), true,
    "active mode needs live workspace data");
assert.equal(needsWorkspaceData("5", true, false), true,
    "dynamic mode needs live workspace data");

assert.deepEqual(Array.from(workspaceList(workspaces, "5", false, "eDP-1", true, null, "")),
    ["1", "3", "4", "5"], "fixed mode filters assigned workspaces and keeps uncreated slots");
assert.deepEqual(Array.from(workspaceList(workspaces, "5", false, "DP-1", true, null, "12")),
    ["2", "3", "4", "5", "12"], "fixed mode includes the local active workspace beyond its range");
assert.equal(extraWorkspace({ name: "1" }, "5"), "");
assert.equal(extraWorkspace({ name: "3" }, "5"), "",
    "in-range focus changes keep the extra workspace binding unchanged");
assert.equal(extraWorkspace({ name: "12" }, "5"), "12",
    "fixed mode adds a workspace beyond its numbered range");

let focusedOutput = "DP-1";
assert.equal(markerOpacity(hasMonitorFocus(true, "eDP-1", focusedOutput)), 0.72,
    "an active marker dims when another output has focus");
focusedOutput = "eDP-1";
assert.equal(markerOpacity(hasMonitorFocus(true, "eDP-1", focusedOutput)), 1.0,
    "the active marker brightens when focus moves to its output");

// per-output numbering: both screens have a "1" and a "2", told apart only by id
const perOutput = [
    { id: "3", name: "1", output: "DP-1", active: false, occupied: true, special: false },
    { id: "4", name: "2", output: "DP-1", active: true, occupied: false, special: false },
    { id: "1", name: "1", output: "HDMI-A-1", active: true, occupied: false, special: false },
    { id: "2", name: "2", output: "HDMI-A-1", active: false, occupied: true, special: false }
];
assert.deepEqual(Array.from(workspaceList(perOutput, "active", true, "HDMI-A-1", true, null, "")),
    ["1", "2"], "a dynamic row lists only its own screen's workspaces");
assert.equal(workspaceOn(perOutput, "2", "HDMI-A-1", true).id, "2",
    "a cell resolves to the workspace on its own screen");
assert.equal(workspaceOn(perOutput, "2", "DP-1", true).id, "4",
    "the same name on the other screen resolves to that screen's workspace");
assert.equal(workspaceOn(perOutput, "2", "HDMI-A-1", false).id, "4",
    "without isolation a name keeps resolving to its first match");
assert.equal(workspaceOn(perOutput, "3", "HDMI-A-1", true), null,
    "an uncreated slot has no workspace to resolve");
assert.equal(activeWorkspace(perOutput, "HDMI-A-1", true, perOutput[1]).id, "1",
    "each screen marks its own shown workspace even when names repeat");
