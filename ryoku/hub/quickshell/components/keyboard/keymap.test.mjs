import fs from "node:fs";
import vm from "node:vm";
import assert from "node:assert/strict";
import test from "node:test";

const source = fs.readFileSync(new URL("./keymap.js", import.meta.url), "utf8").replace(/^\.pragma library\s*$/m, "");
const sandbox = {};
vm.runInNewContext(source + "\nthis.api = { primary, secondary, secondaryVariant, variantString, hasSwitchKey, withSwitchKey, layoutString };", sandbox);
const k = sandbox.api;

test("one layout keeps only its own style", () => {
    assert.equal(k.variantString("azerty", "", false), "azerty");
    assert.equal(k.variantString("", "winkeys", false), "");
});

test("each style stands in the slot of its layout", () => {
    assert.equal(k.variantString("azerty", "", true), "azerty,");
    assert.equal(k.variantString("", "winkeys", true), ",winkeys");
    assert.equal(k.variantString("azerty", "winkeys", true), "azerty,winkeys");
    assert.equal(k.variantString("", "", true), "");
});

test("changing the first style leaves the second one's slot as it was", () => {
    var now = ",winkeys";
    assert.equal(k.variantString("nodeadkeys", k.secondaryVariant(now), true), "nodeadkeys,winkeys");
    now = "azerty,winkeys";
    assert.equal(k.variantString("", k.secondaryVariant(now), true), ",winkeys");
});

test("the second style is read from its slot, never from the first", () => {
    assert.equal(k.secondaryVariant("azerty"), "");
    assert.equal(k.secondaryVariant("azerty,"), "");
    assert.equal(k.secondaryVariant(",winkeys"), "winkeys");
    assert.equal(k.secondaryVariant("azerty, winkeys "), "winkeys");
    assert.equal(k.primary(",winkeys"), "");
});

test("a second layout adds Alt+Shift only when nothing switches layouts yet", () => {
    assert.equal(k.withSwitchKey(""), "grp:alt_shift_toggle");
    assert.equal(k.withSwitchKey("caps:escape"), "caps:escape,grp:alt_shift_toggle");
    assert.equal(k.withSwitchKey("grp:win_space_toggle,caps:escape"), "grp:win_space_toggle,caps:escape");
    // a key the Settings controls do not offer still counts as a switch
    assert.equal(k.withSwitchKey("grp:ctrl_shift_toggle"), "grp:ctrl_shift_toggle");
    assert.equal(k.hasSwitchKey("ctrl:nocaps"), false);
    assert.equal(k.hasSwitchKey(""), false);
});
