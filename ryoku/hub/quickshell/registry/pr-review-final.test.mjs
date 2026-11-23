import assert from "node:assert/strict";
import test from "node:test";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const directory = path.dirname(fileURLToPath(import.meta.url));
const hub = path.dirname(directory);
const read = name => fs.readFileSync(path.join(hub, name), "utf8");

test("the keepsake reveal skips entrance animation under reduced motion", () => {
    const keepsake = read("pages/KeepsakePage.qml");
    assert.match(keepsake, /if \(Lithic\.reduceMotion\) pg\.reveal = 1/);
    assert.match(keepsake, /else enter\.start\(\)/);
});

test("keepsake story and contributor text use translations", () => {
    const keepsake = read("pages/KeepsakePage.qml");
    for (const field of [
        "pg.beginning", "pg.people", "pg.letter", "contributor.modelData.quote",
        "contributor.modelData.role"
    ]) {
        assert.ok(keepsake.includes("I18n.tr(" + field + ")"), field);
    }
    for (const heading of ["The Human Side of Ryoku", "How Ryoku on NixOS was made",
        "The people behind Ryoku on NixOS", "A note to you", "EARLY ADOPTERS"]) {
        assert.ok(keepsake.includes('I18n.tr("' + heading + '")'), heading);
    }
});
