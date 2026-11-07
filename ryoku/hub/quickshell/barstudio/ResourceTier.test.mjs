import test from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const Tier = require("./ResourceTier.js");

test("the published tier is read as is", () => {
    assert.equal(Tier.tierOf("light"), "light");
    assert.equal(Tier.tierOf("standard"), "standard");
    assert.equal(Tier.tierOf("heavy"), "heavy");
});

test("anything else is unmeasured", () => {
    for (const value of [undefined, null, "", "HEAVY", "huge", 3, {}])
        assert.equal(Tier.tierOf(value), "", String(value));
});

test("heavy and standard get distinct corner marks, light and unmeasured none", () => {
    assert.equal(Tier.markOf("heavy"), "▲");
    assert.equal(Tier.markOf("standard"), "△");
    assert.notEqual(Tier.markOf("standard"), Tier.markOf("heavy"));
    assert.equal(Tier.markOf("light"), "");
    assert.equal(Tier.markOf(""), "");
});
