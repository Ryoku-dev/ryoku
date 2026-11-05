import test from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const Subshells = require("./Subshells.js");
const styles = ["sumi", "iris", "python", "orpheus", "gokko", "tansu", "caelestia", "imi"]
    .map(id => ({ id }));

test("Bar Studio keeps every bar style", () => {
    assert.deepEqual(Subshells.visibleStyles(styles, false).map(style => style.id),
        ["sumi", "iris", "python", "orpheus", "gokko", "tansu", "caelestia", "imi"]);
});

test("Subshells keeps its focused projection", () => {
    assert.deepEqual(Subshells.visibleStyles(styles, true).map(style => style.id),
        ["python", "gokko", "tansu", "caelestia", "imi"]);
    assert.equal(Subshells.isStyle("iris"), false);
    assert.equal(Subshells.isStyle("python"), true);
    assert.equal(Subshells.isStyle("imi"), true);
});
