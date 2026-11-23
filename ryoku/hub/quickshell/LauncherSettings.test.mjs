import fs from "node:fs";
import vm from "node:vm";
import assert from "node:assert/strict";
import test from "node:test";

const source = fs.readFileSync(new URL("./LauncherSettings.js", import.meta.url), "utf8");
const context = {};
vm.runInNewContext(source.replace(".pragma library", ""), context);
const parse = text => JSON.parse(JSON.stringify(context.parse(text)));
const shellIndex = {};
vm.runInNewContext(fs.readFileSync(new URL("./ShellIndex.js", import.meta.url), "utf8")
    .replace(".pragma library", ""), shellIndex);
// the descriptor Settings reads for a contributed launcher, the way LauncherStyles.qml finds it
const contributionSettings = (record, entry, shellRoot) =>
    context.besideEntry(shellIndex.filePath(record, entry, shellRoot));

test("a descriptor keeps known capabilities and well-formed sections", () => {
    const got = parse(JSON.stringify({
        capabilities: ["overlay", "sections", "hiddenApps", "overlay", "teleport"],
        sections: [{ id: "internet", label: "Internet" }, { id: "dev-tools", label: "Develop" }]
    }));
    assert.deepEqual(got.capabilities, ["overlay", "sections", "hiddenApps"]);
    assert.deepEqual(got.sections.map(s => s.id), ["internet", "dev-tools"]);
});

test("a missing, empty or malformed file offers nothing", () => {
    for (const text of ["", "not json", "null", "[]", "42", undefined])
        assert.deepEqual(parse(text), { capabilities: [], sections: [] });
});

test("bad section rows are dropped, labels are bounded and repeats keep the first", () => {
    const got = parse(JSON.stringify({
        capabilities: ["sections"],
        sections: [
            { id: "Media", label: "Media" }, { id: "media", label: "" },
            { id: "media", label: "  Media  " }, { id: "media", label: "Again" },
            { id: "office" }, { id: "games", label: "Games\nNow" },
            { id: "long", label: "x".repeat(81) }, null, "utility"
        ]
    }));
    assert.deepEqual(got.sections, [{ id: "media", label: "Media" }]);
});

test("the sections capability needs sections to order", () => {
    const got = parse(JSON.stringify({ capabilities: ["sections", "grid"], sections: [] }));
    assert.deepEqual(got.capabilities, ["grid"]);
});

test("oversized descriptors are rejected before parsing", () => {
    assert.deepEqual(parse(`{"capabilities":[],"padding":"${"x".repeat(65536)}"}`),
        { capabilities: [], sections: [] });
});

test("the descriptor sits beside the entry file", () => {
    assert.equal(context.besideEntry("/opt/item/qml/launcher/Main.qml"), "/opt/item/qml/launcher/settings.json");
    for (const entry of ["", undefined, "qml/launcher/Main.qml", "/Main.qml", "/opt/item/"])
        assert.equal(context.besideEntry(entry), "");
});

test("a shipped contribution resolves under the shell root, a store one under its own path", () => {
    const root = "/home/u/.config/quickshell/ryoku";
    assert.equal(contributionSettings({ source: "shipped", prepared: true }, "subshells/orpheus/qml/launcher/Main.qml", root),
        root + "/subshells/orpheus/qml/launcher/settings.json");
    assert.equal(contributionSettings({ source: "store", prepared: true }, "/state/store/items/bar/qml/launcher/Main.qml", root),
        "/state/store/items/bar/qml/launcher/settings.json");
    assert.equal(contributionSettings({ source: "store", prepared: true }, "qml/launcher/Main.qml", root), "");
    assert.equal(contributionSettings({ source: "store", prepared: false }, "/state/store/items/bar/qml/launcher/Main.qml", root), "");
});

test("the Orpheus launcher offers hidden and pinned apps", () => {
    const text = fs.readFileSync(new URL("../../shell/quickshell/shell/subshells/orpheus/qml/launcher/settings.json", import.meta.url), "utf8");
    assert.deepEqual(parse(text).capabilities, ["hiddenApps", "pinnedApps"]);
});
