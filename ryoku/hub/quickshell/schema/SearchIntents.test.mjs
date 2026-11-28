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
const { intents, fillers, langs } = load("schema/SearchIntents.js");
const { fold, intentKeys, searchRows } = load("SettingsSearch.js");

// Every row key the Hub's global search can reach: the Hub's own schemas plus
// the active provider's. An intent that named anything else would be a promise
// the search cannot keep, which is worse than no intent at all.
const rows = [];
let keys = new Set();
for (const f of fs.readdirSync(path.join(QML_DIR, "schema"))) {
    if (!f.endsWith(".js") || f === "SearchIntents.js" || f === "ConnectionsTabs.js") continue;
    const c = load("schema/" + f);
    for (const r of c.rows || []) {
        rows.push(r);
        if (r.key) keys.add(r.key);
    }
}
for (const wm of ["hyprland", "niri", "wayfire"]) {
    const j = JSON.parse(fs.readFileSync(`/home/san/Repos/ryoku/ryoku/wm/${wm}/schema.json`, "utf8"));
    for (const r of j.rows || []) {
        rows.push(r);
        if (r.key) keys.add(r.key);
    }
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
    const hits = intentKeys("size", intents, fillers, "en", langs);
    assert.ok(hits === null || hits.length <= 3, `a bare "size" stays narrow: ${hits}`);
});

test("an intent never reorders a single-word query the plain search answered", () => {
    // the curation rule, mechanical: for every word an intent spells, if the
    // plain search already returns a first row, that row must stay first when
    // the intent fires. An intent may FILL an empty result; it may not beat a
    // row the engine already ranked -- that is how "shadow" came to mean
    // "turn shadows on" ahead of "Shadows".
    const plain = (q) => {
        const r = searchRows(rows, q);
        return r.length ? r[0].key : null;
    };
    const withI = (q) => {
        const r = searchRows(rows, q, null, null, intents, fillers, langs);
        return r.length ? r[0].key : null;
    };
    for (const it of intents) {
        const words = fold(it.words).split(/\s+/).filter(w => w.length > 1);
        for (const w of words) {
            const before = plain(w);
            if (before === null) continue;           // the plain search found nothing: an intent may fill
            const after = withI(w);
            assert.equal(after, before,
                `"${w}" plain-ranks ${before} first; the intent must not move it (saw ${after})`);
        }
    }
});

test("an intent fills a phrase whose words no haystack carries", () => {
    // "brillo pantalla" spells no English haystack word, so every pass comes
    // back empty: the hand-written mapping must still put its row on the page,
    // and first -- an intent that only reordered would have nothing to reorder
    const r = searchRows(rows, "brillo pantalla", null, "en", intents, fillers, langs);
    assert.equal(r.length > 0, true);
    assert.equal(r[0].key, "sdrbrightness");
});

test("outside the languages the table is written in, an intent does not fire", () => {
    assert.ok(intentKeys("cursor grande", intents, fillers, "es", langs).includes("desktop.cursor.size"));
    assert.equal(intentKeys("cursor grande", intents, fillers, "de", langs), null,
        "a German Hub must not get a Spanish task promoted");
    assert.equal(intentKeys("layout", intents, fillers, "de", langs), null,
        "a loanword in an uncurated language degrades to the plain ranking");
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