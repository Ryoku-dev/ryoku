import fs from "node:fs";
import vm from "node:vm";
import assert from "node:assert/strict";
import test from "node:test";

const Combos = {};
vm.createContext(Combos);
vm.runInContext(fs.readFileSync(new URL("../../Combos.js", import.meta.url), "utf8")
    .replace(/^\.pragma library\s*/, ""), Combos);
const Binds = { Combos };
vm.createContext(Binds);
vm.runInContext(fs.readFileSync(new URL("./binds.js", import.meta.url), "utf8")
    .replace(/^\.(?:pragma|import)[^\n]*\n/gm, ""), Binds);

const sight = { id: "shell.sight", label: "Sight", kind: "daemon", combo: "Alt_R", chord: "Control_R", taps: 3 };
const highlight = { id: "shell.sightHighlight", label: "Sight on highlighted text", kind: "daemon", combo: "Alt_L", chord: "Shift_L", whileHighlighting: true };

test("daemon gestures stay reserved despite compositor rebinds, disable or refusal", () => {
    const holders = Binds.holders([{ ...sight, unhonored: "old provider" }, highlight],
        { "Alt_R": "SUPER + Z", "Alt_L": "SUPER + Y" }, ["Alt_R", "Alt_L"], []);
    assert.equal(Binds.clash(holders, "Control_R", { kind: "custom", index: 0 }).id, sight.id);
    assert.equal(Binds.clash(holders, "Shift_L", { kind: "custom", index: 0 }).id, highlight.id);
    assert.equal(Binds.clash(holders, "SUPER + Z", { kind: "custom", index: 0 }), null);
    assert.equal(Binds.clash(holders, "Alt_R", { kind: "custom", index: 0 }), null);
    assert.equal(Binds.clash(holders, "Control_R", { kind: "bind", id: sight.id }), null);
});

test("a default daemon gesture conflicts with a custom compositor shortcut on that key", () => {
    const holders = Binds.holders([{ ...sight, chord: "" }], {}, [], []);
    assert.equal(Binds.clash(holders, "Alt_R", { kind: "custom", index: 0 }).id, sight.id);
});

test("non-daemon shortcuts retain rebind, disable and unavailable semantics", () => {
    const bind = { id: "window.close", label: "Close", kind: "wm", combo: "SUPER + Q" };
    const rebinds = { "SUPER + Q": "SUPER + X" };
    let holders = Binds.holders([bind], rebinds, [], []);
    assert.equal(Binds.clash(holders, "Super+Q", { kind: "custom", index: 0 }), null);
    assert.equal(Binds.clash(holders, "Super+X", { kind: "custom", index: 0 }).id, bind.id);
    holders = Binds.holders([bind], rebinds, [bind.combo], []);
    assert.equal(Binds.clash(holders, "Super+X", { kind: "custom", index: 0 }), null);
    holders = Binds.holders([{ ...bind, unhonored: "not available" }], rebinds, [], []);
    assert.equal(Binds.clash(holders, "Super+X", { kind: "custom", index: 0 }), null);
});
