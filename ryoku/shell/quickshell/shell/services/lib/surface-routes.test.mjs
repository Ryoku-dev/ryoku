import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test from "node:test";

const require = createRequire(import.meta.url);
const { styleOwns } = require("./surface-routes.js");

const styles = ["python", "iris", "caelestia", "orpheus", "qsbar"];

test("Ryoku-owned shortcuts never route into a bar style", () => {
    for (const style of styles) {
        for (const surface of ["launcher", "clipboard", "settings", "sidebar-left"])
            assert.equal(styleOwns(surface, style), false, `${style} must not own ${surface}`);
    }
});

test("Shima keeps its unique shortcuts sheet", () => {
    assert.equal(styleOwns("cheatsheet", "iris"), true);
    for (const style of styles.filter(style => style !== "iris"))
        assert.equal(styleOwns("cheatsheet", style), false);
});
