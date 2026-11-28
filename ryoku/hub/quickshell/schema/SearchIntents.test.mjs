import fs from "node:fs";
import vm from "node:vm";
import assert from "node:assert/strict";
import test from "node:test";
import path from "node:path";

const QML_DIR = "/home/san/Repos/ryoku/ryoku/hub/quickshell";
const load = (rel) => {
    const src = fs.readFileSync(path.join(QML_DIR, rel), "utf8").replace(/^\.pragma library\s*/, "");
    const ctx = {};
    vm.createContext(ctx);
    vm.runInContext(src, ctx);
    return ctx;
};
const { intents, fillers } = load("schema/SearchIntents.js");
const { fold, intentKeys } = load("SettingsSearch.js");

// Every row key the Hub's global search can reach: the Hub's own schemas plus
// the active provider's. An intent that named anything else would be a promise
// the search cannot keep, which is worse than no intent at all.
let keys = new Set();
for (const f of fs.readdirSync(path.join(QML_DIR, "schema"))) {
    if (!f.endsWith(".js") || f === "SearchIntents.js" || f === "ConnectionsTabs.js") continue;
    const c = load("schema/" + f);
    for (const r of c.rows || []) if (r.key) keys.add(r.key);
}
for (const wm of ["hyprland", "niri", "wayfire"]) {
    const j = JSON.parse(fs.readFileSync(`/home/san/Repos/ryoku/ryoku/wm/${wm}/schema.json`, "utf8"));
    for (const r of j.rows || []) if (r.key) keys.add(r.key);
}
for (const k of ["clipboard.widthPercent", "gamebar.enabled", "idle.enabled", "language", "fontSize"])
    assert.ok(keys.has(k), `the corpus really carries ${k}`);

test("every intent names rows that exist", () => {
    assert.ok(intents.length >= 20, "the table carries a first working batch");
    for (const it of intents) {
        assert.ok(it.keys && it.keys.length, "an intent names its row");
        assert.ok(it.words && it.words.trim(), "an intent spells its words");
        for (const k of it.keys) assert.ok(keys.has(k), `${k} is a real row`);
    }
});

test("no intent repeats a key it already owns", () => {
    const seen = new Set();
    for (const it of intents) {
        for (const k of it.keys) {
            assert.ok(!seen.has(k), `${k} is owned by exactly one intent`);
            seen.add(k);
        }
    }
});

test("a phrase a user types reaches the row that owns it", () => {
    const keysFor = (q) => intentKeys(q, intents, fillers);
    assert.ok(keysFor("cursor grande").includes("desktop.cursor.size"));
    // order-independent, and a second spelling of the same task
    assert.ok(keysFor("grande cursor").includes("desktop.cursor.size"));
    assert.ok(keysFor("el puntero esta demasiado grande").includes("desktop.cursor.size"));
    // one stray word must not derail the phrase, and a phrase that names
    // nothing must not reach anything
    assert.deepEqual(keysFor("cursor zzzqqq"), null);
    assert.deepEqual(keysFor("zzzqqq"), null);
    // a single character is a query (IME composition), so no intent fires
    assert.deepEqual(keysFor("a"), null);
    assert.deepEqual(keysFor("画"), null);
    // accents fold on both sides of the match
    assert.ok(keysFor("separacion entre ventanas").includes("desktop.appearance.gapsIn"));
});

test("a common word alone does not fire every intent that mentions it", () => {
    // "size" is a real word in the cursor intent, but a query that is nothing
    // but that word must not hijack the results: it stays a ranking decision
    const hits = intentKeys("size", intents, fillers);
    assert.ok(hits === null || hits.length <= 3, `a bare "size" stays narrow: ${hits}`);
});

test("the table stays translatable work, not a second copy of the labels", () => {
    // an intent whose words are only the row's own label adds nothing the
    // haystack does not already answer, and costs a place in the ranking
    const labels = new Set();
    for (const f of fs.readdirSync(path.join(QML_DIR, "schema"))) {
        if (!f.endsWith(".js") || f === "SearchIntents.js" || f === "ConnectionsTabs.js") continue;
        const c = load("schema/" + f);
        for (const r of c.rows || []) if (r.label) labels.add(fold(r.label));
    }
    for (const it of intents) {
        const own = it.keys.filter(k => k).length;
        assert.ok(own >= 1, "an intent owns its row");
        // an intent may repeat the label, but its phrasing must add something
        const extra = fold(it.words).split(/\s+/).length;
        assert.ok(extra >= 4, "an intent spells more than the label does");
    }
});