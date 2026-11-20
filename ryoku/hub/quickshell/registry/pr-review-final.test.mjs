import assert from "node:assert/strict";
import test from "node:test";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const directory = path.dirname(fileURLToPath(import.meta.url));
const hub = path.dirname(directory);
const read = name => fs.readFileSync(path.join(hub, name), "utf8");

test("save errors keep last confirmed data and prevent further writes", () => {
    const progression = read("Singletons/Progression.qml");
    assert.match(progression, /property var lastRead: Model\.clean\(\{\}\)/);
    assert.match(progression, /property bool saveBlocked: false/);
    assert.match(progression, /root\.lastRead = loaded/);
    assert.match(progression, /onSaveFailed: \(saveError\) => \{/);
    assert.match(progression, /root\.saveBlocked = true/);
    assert.match(progression, /root\.data = root\.lastRead/);
    assert.match(progression, /root\.error = "Could not save progression data/);
    assert.match(progression, /if \(!root\.saveBlocked\) root\.error = ""/);
    assert.match(progression, /function save\(state\) \{\s*if \(!root\.ready \|\| root\.error !== ""\)/);
});

test("progression errors are visibly surfaced across Hub pages", () => {
    const hubShell = read("Hub.qml");
    assert.match(hubShell, /visible: Progression\.error !== ""/);
    assert.match(hubShell, /text: I18n\.tr\(Progression\.error\)/);
    assert.match(hubShell, /id: progressionWarning/);
});

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

test("a contributor portrait is already bounded and no unrelated hero trigger exists", () => {
    const keepsake = read("pages/KeepsakePage.qml");
    const progression = read("registry/progression.js");
    assert.match(keepsake,
        /source: contributor\.modelData\.avatar\s+sourceSize\.width: Math\.max\(1, Math\.round\(width \* 2\)\)/);
    assert.doesNotMatch(progression, /hero\.custom/);
    assert.doesNotMatch(progression, /allBlocksOff/);
});
