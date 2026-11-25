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

test("shortcuts retain rebind, disable and unavailable semantics", () => {
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
