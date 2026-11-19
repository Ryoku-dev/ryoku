import assert from "node:assert/strict";
import test from "node:test";
import fs from "node:fs";
import vm from "node:vm";

const schema = vm.createContext({});
vm.runInContext(fs.readFileSync(new URL("rashin.js", import.meta.url), "utf8").replace(/^\.pragma .*$/gm, ""), schema);
const plain = value => JSON.parse(JSON.stringify(value));
const row = id => schema.rows.find(value => value.id === id);
const desktop = ["inplace", "summarize", "deepread", "math", "learn", "readaloud", "proofread", "rewrite", "tone", "fluent", "humanize", "aicheck", "inclusive", "reactions", "reply", "voice", "translatestyle", "dictionary", "overimage", "quiet", "save"];
const options = {
    bubble: [["translate", "explain", "ask"], "highlight"], pinned: [[], "on"],
    tapKey: ["Alt_R", "enabled"], highlightKey: ["Alt_L", "highlight"],
    "summarize.style": ["points", "summarize"],
    "deepread.lenses": [["factcheck", "assumptions", "concepts", "contrasts", "related", "passages", "section"], "deepread"],
    "learn.count": [5, "learn"], "readaloud.voice": ["", "readaloud"], "readaloud.rate": [1, "readaloud"],
    "proofread.engine": ["auto", "proofread"], "proofread.dialect": ["us", "proofread"], "proofread.off": [[], "proofread"],
    "rewrite.variant": ["rephrase", "rewrite"], "rewrite.audience": ["general", "rewrite"],
    "tone.favorites": [["formal", "friendly", "confident", "direct", "empathetic"], "tone"],
    "voice.use": [true, "voice"], "translate.style": ["natural", "translatestyle"],
    "translate.simpler": [false, "translatestyle"], "translate.back": [false, "translatestyle"],
    "inplace.key": ["Control_R", "inplace"], "inplace.action": ["auto", "inplace"], "inplace.paste": [true, "inplace"],
    "quiet.apps": [[], "quiet"], "quiet.snooze": [0, "quiet"]
};

test("Rashin sections keep the agreed order and all rows have unique deep links", () => {
    assert.deepEqual(plain(schema.sections.map(value => value.id)), ["rashin", "theia", "theia-reading", "theia-writing", "theia-translate", "theia-listening", "theia-quiet", "theia-engine", "status", "open"]);
    assert.equal(new Set(schema.rows.map(value => value.id)).size, schema.rows.length);
    for (const value of schema.rows) {
        assert.ok(schema.sections.some(section => section.id === value.section), value.id);
        if (value.bind)
            assert.deepEqual(plain(value.bind.fallback), plain(value.default), value.id);
        if (value.when)
            assert.ok(schema.rows.some(parent => parent.bind && parent.bind.store === value.when.bind.store && parent.bind.key === value.when.bind.key), value.id);
    }
});

test("every desktop feature has one off-by-default chip switch", () => {
    const allowed = new Set(["look", "highlight", "inplace", "panel", "bubble", "settings"]);
    assert.equal(desktop.length, 21);
    assert.deepEqual(plain(schema.rows.filter(value => value.id.startsWith("theia-f-")).map(value => value.feature)).sort(), [...desktop].sort());
    for (const id of desktop) {
        const value = row("theia-f-" + id);
        assert.deepEqual(plain(value.bind), { store: "sight", key: "features." + id, fallback: false });
        assert.equal(value.default, false, id);
        assert.equal(value.control, "custom", id);
        assert.equal(value.activatable, true, id);
        assert.ok(value.shows.length > 0 && value.shows.every(place => allowed.has(place)), id);
        assert.ok(value.keywords.includes("sight"), id);
    }
    assert.deepEqual(plain(row("theia-on").bind), { store: "sight", key: "on", fallback: true });
    assert.equal(row("theia-f-save").section, "theia");
});

test("Rashin app features have one handoff instead of switches in the Hub", () => {
    const value = row("rashin-app-features");
    assert.equal(value.section, "rashin");
    assert.equal(value.label, "Rashin app features");
    assert.equal(value.control, "handoff");
    assert.equal(value.actionText, "Open in Rashin");
    assert.deepEqual(plain(value.command), ["rashin", "open", "--tab", "settings", "--view", "features"]);
    assert.equal(value.bind, undefined);
    assert.ok(schema.rows.every(row => !row.app && (!row.bind || row.bind.store !== "rashinFeatures")));
    assert.ok(schema.features.every(feature => !["minutes", "imagetools", "images"].includes(feature.id)));
    for (const section of schema.sections)
        assert.ok(schema.rows.some(row => row.section === section.id), section.id);
});

test("all Theia options use their exact keys, defaults and feature conditions", () => {
    for (const [key, [fallback, feature]] of Object.entries(options)) {
        const value = row("theia-o-" + key);
        assert.ok(value, key);
        assert.deepEqual(plain(value.bind), { store: "sight", key, fallback });
        assert.deepEqual(plain(value.default), fallback);
        assert.deepEqual(plain(value.when), { bind: { store: "sight", key: ["enabled", "highlight", "on"].includes(feature) ? feature : "features." + feature, fallback: ["enabled", "highlight", "on"].includes(feature) }, is: [true] });
        const parentIndex = schema.rows.findIndex(parent => parent.bind && parent.bind.store === value.when.bind.store && parent.bind.key === value.when.bind.key);
        assert.ok(parentIndex < schema.rows.indexOf(value), key);
    }
    const groups = new Map();
    for (const value of schema.rows.filter(value => value.id.startsWith("theia-o-") && value.when)) {
        const key = value.when.bind.key;
        const group = groups.get(key) || [];
        group.push(value);
        groups.set(key, group);
    }
    for (const group of groups.values())
        for (const value of group.slice(2))
            assert.equal(value.more, true, value.id);
    const rate = row("theia-o-readaloud.rate");
    assert.equal(rate.min, 0.75);
    assert.equal(rate.max, 1.5);
    assert.deepEqual(plain(row("theia-o-learn.count").options.map(value => value.value)), [3, 5, 8]);
    assert.deepEqual(plain(row("theia-o-quiet.snooze").options.map(value => value.value)), [0, 15, 60]);
    assert.equal(row("theia-o-inplace.action").options.length, 28);
    assert.deepEqual(plain(row("theia-o-inplace.action").options[0]), { label: "Fix or translate", value: "auto" });
    for (const value of row("theia-o-inplace.action").options)
        assert.equal(typeof value.label, "string");
});

test("existing Sight row ids survive their move into Theia without an engine picker", () => {
    const moved = {
        "sight-enabled": "theia", "sight-highlight": "theia", "sight-mode": "theia", "sight-shortcut": "theia", "sight-status": "theia",
        "sight-target": "theia-translate", "sight-languages": "theia-translate",
        "sight-local-only": "theia-engine", "sight-model": "theia-engine"
    };
    for (const [id, section] of Object.entries(moved))
        assert.equal(row(id).section, section, id);
    assert.equal(schema.rows.find(value => value.id === "sight-engine"), undefined);
    for (const value of schema.rows) {
        assert.ok(!/\bSight\b|\bAI\b|\u2014/.test(value.label + " " + value.detail), value.id);
        for (const option of value.options || [])
            assert.ok(!/\bSight\b|\bAI\b|\u2014/.test(option.label), value.id);
    }
});

test("writing checks use the built-in English checker and a personal dictionary", () => {
    const engine = row("theia-o-proofread.engine");
    assert.equal(engine.detail, "English on this computer, other languages with your model");
    assert.deepEqual(plain(engine.options), [
        { label: "Automatic", value: "auto" },
        { label: "This computer", value: "local" },
        { label: "Model", value: "model" }
    ]);
    assert.deepEqual(plain(row("theia-o-proofread.dialect").options.map(option => option.value)), ["us", "uk", "ca", "au", "in"]);
    const words = row("theia-o-proofread.words");
    assert.equal(words.label, "Your words");
    assert.equal(words.control, "custom");
    assert.equal(words.more, true);
    assert.equal(words.bind, undefined);
    assert.deepEqual(plain(words.when), { bind: { store: "sight", key: "features.proofread", fallback: false }, is: [true] });
    assert.ok(!schema.rows.filter(value => value.id.startsWith("theia-o-proofread.")).some(value => /\binstall\b/i.test(JSON.stringify(value))));
});
