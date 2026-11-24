import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";

const require = createRequire(import.meta.url);
const StyleCard = require("./StyleCard.js");

const here = path.dirname(fileURLToPath(import.meta.url));
const barstyles = path.join(here, "../../../shell/quickshell/shell/modules/bar/barstyles");
const hubCards = path.join(here, "cards");
const page = fs.readFileSync(path.join(here, "../pages/BarStudioPage.qml"), "utf8");

// the editor names the page hands to StyleCardSource, read from its map
const editors = [...page.match(/studioEditors: \(\{([\s\S]*?)\}\)/)[1].matchAll(/"([a-z-]+)":/g)].map(m => m[1]);

function declarations() {
    const out = [];
    for (const id of fs.readdirSync(barstyles)) {
        const file = path.join(barstyles, id, "studio-card.json");
        if (fs.existsSync(file))
            out.push({ id, file });
    }
    for (const name of fs.readdirSync(hubCards))
        if (name.endsWith(".json"))
            out.push({ id: name.replace(/\.json$/, ""), file: path.join(hubCards, name) });
    return out;
}

test("every built-in style that had a hand-built card declares it", () => {
    const ids = declarations().map(d => d.id).sort();
    assert.deepEqual(ids, ["iris", "nacre", "nomarchy", "obi", "orpheus", "python", "qsbar", "sumi"]);
});

test("every shipped declaration parses against the page's editors", () => {
    for (const { id, file } of declarations()) {
        const parsed = StyleCard.parse(fs.readFileSync(file, "utf8"), { id, shipped: true, editors });
        assert.equal(parsed.error, "", `${file}: ${parsed.error}`);
        assert.ok(parsed.cards.length > 0, `${file} draws a card`);
    }
});

test("every editor the page offers is placed by a declaration", () => {
    const used = new Set();
    for (const { id, file } of declarations())
        for (const card of StyleCard.parse(fs.readFileSync(file, "utf8"), { id, shipped: true, editors }).cards)
            for (const block of card.blocks)
                if (block.kind === "editor")
                    used.add(block.editor);
    assert.deepEqual([...used].sort(), [...editors].sort());
});

test("the hand-built buttons keep their exact commands", () => {
    const commands = {};
    for (const { id, file } of declarations())
        for (const card of StyleCard.parse(fs.readFileSync(file, "utf8"), { id, shipped: true, editors }).cards)
            for (const block of card.blocks)
                for (const button of block.buttons || [])
                    commands[button.label] = button.command;
    assert.deepEqual(commands, {
        "OPEN QS BAR SETTINGS": ["ryoku-shell", "bar", "settings"],
        "OPEN ORPHEUS SETTINGS": ["qs", "-c", "shell", "ipc", "call", "orpheus", "settings", ""],
        "OPEN SHIMA SETTINGS": ["qs", "-c", "shell", "ipc", "call", "iris", "settings", ""],
        "OPEN PYTHON SETTINGS": ["qs", "-c", "shell", "ipc", "call", "main", "handleCommand", "toggle", "guide", ""],
        "CHOOSE THEME": ["qs", "-c", "shell", "ipc", "call", "nomarchy", "summon", "omarchy.image-picker", "{\"source\":\"themes\"}"],
        "OPEN PLUGINS": ["qs", "-c", "shell", "ipc", "call", "nomarchy", "summon", "omarchy.menu", "{\"menu\":\"setup.plugins\"}"]
    });
});
