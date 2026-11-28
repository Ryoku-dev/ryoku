import fs from "node:fs";
import vm from "node:vm";
import assert from "node:assert/strict";
import test from "node:test";

const source = fs
    .readFileSync(new URL("./SettingsSearch.js", import.meta.url), "utf8")
    .replace(/^\.pragma library\s*/, "");
const context = {};
vm.createContext(context);
vm.runInContext(source, context);
const { fold, searchRows } = context;

// The rows a schema page hands the sheet: untranslated source rows, because
// the sheet translates a row when it draws it (I18n.tr(r.label)).
const page = [
    { key: "disableBlur", label: "Disable blur", desc: "Drops the frosted-glass look everywhere", group: "EFFECTS", tab: "Look" },
    { key: "gapIn", label: "Window gaps", desc: "Space between windows", group: "WINDOWS", tab: "" },
    { key: "saveBattery", label: "Battery saver", desc: "Trades speed for battery", group: "POWER", tab: "",
      keywords: "battery performance power saving" },
    { key: "zoomModifier", label: "Zoom modifier", desc: "Hold this modifier while scrolling to zoom", group: "ZOOM", tab: "Behaviour" },
    { key: "themeMode", label: "Colour scheme", desc: "Light, dark or auto", group: "APPEARANCE", tab: "" },
    { key: "winOpacity", label: "Window opacity", desc: "How see-through a window is", group: "WINDOWS", tab: "" },
    { key: "closeWindow", label: "Close window", desc: "Close the focused window", group: "WINDOWS", tab: "" },
];
// A page that bakes its translations into the rows instead (PluginsPage's
// extraRows, and the bespoke pages): the same shape, translated text.
const baked = [
    { key: "zoomModifier", label: "修飾キー", desc: "スクロールでズーム", group: "ズーム" },
    { key: "saveBattery", label: "बैटरी बचत", desc: "बैटरी", group: "ऊर्जा" },
    { key: "saveBattery2", label: "البحث عن البطارية", desc: "موفر الطاقة", group: "الطاقة" },
    { key: "saveBattery3", label: "พลังงาน", desc: "พลังงาน", group: "พลังงาน" },
    { key: "saveBattery4", label: "Ahorro de energía", desc: "Rinde más, gasta menos", group: "Energía" },
    { key: "saveBattery5", label: "Batería", desc: "Potencia", group: "Energía" },
];
// the module runs in a vm context, whose Array.prototype is not this realm's;
// rebuild every result as a host array so assert.deepEqual compares values.
const keys = (q, rows) => Array.prototype.map.call(searchRows(rows, q), r => r.key);

test("the row its words name comes first", () => {
    assert.deepEqual(keys("blur", page).slice(0, 1), ["disableBlur"]);
    assert.deepEqual(keys("zoom modifier", page).slice(0, 1), ["zoomModifier"]);
});

test("a typo in the typed query still lands", () => {
    assert.deepEqual(keys("disabel blur", page).slice(0, 1), ["disableBlur"]);
    assert.deepEqual(keys("batery", page).slice(0, 1), ["saveBattery"]);
});

test("a synonym reaches the row the word never appears on", () => {
    assert.deepEqual(keys("glass", page).slice(0, 1), ["disableBlur"]);
    assert.deepEqual(keys("transparency", page), ["winOpacity"]);
    // a near-miss of a synonym resolves too: "trasparency" -> transparency
    assert.deepEqual(keys("trasparency", page), ["winOpacity"]);
    // synonyms fire only when the typed words found nothing, so a word that
    // expands to a common one ("blur" -> windows) drags in no window rows
    assert.deepEqual(keys("blur", page), ["disableBlur"]);
});

test("a near-miss never drags in the row it is one edit from", () => {
    // "blue" is one edit from "blur" and a scatter inside "blurEnabled", so a
    // colour query must not walk the user into the blur settings. It reaches
    // only the row that really carries the word, and the blur row appears when
    // "blur" itself is typed.
    const blue = keys("blue", page.concat([{ key: "keysounds", label: "Key sound profile", desc: "Cherry MX blue", group: "SOUND" }]));
    assert.deepEqual(blue, ["keysounds"]);
    assert.deepEqual(keys("blur", page).slice(0, 1), ["disableBlur"]);
});

test("an abbreviation is a loose reading, offered only when nothing exact did", () => {
    // "clw" is a scatter inside "Close window", so it is a valid query -- but it
    // must not outrank, or arrive beside, the rows the exact pass already found
    assert.deepEqual(keys("clw", page), ["closeWindow"]);
    // a word the page does not spell loosely still reaches its row, and the
    // scatter of a word the page spells exactly never adds rows
    assert.deepEqual(keys("zoom", page), ["zoomModifier"]);
    const zoom = keys("zoom", page);
    assert.ok(!zoom.includes("closeWindow"), "an exact hit does not absorb scatters");
});

test("a page's group or tab answers to its whole phrase, never one word", () => {
    assert.ok(keys("zoom", page).includes("zoomModifier"));
    assert.deepEqual(keys("window gaps", page), ["gapIn"]);
});

test("a query that matches nothing returns nothing", () => {
    assert.deepEqual(keys("zzzqqq", page), []);
    // one out-of-vocabulary word must not blank a matchable one
    assert.ok(keys("battery zzzqqq", page).includes("saveBattery"));
    // every row a plain word returns carries that word in its own text: no row
    // arrives because of its group, its tab or a near-miss
    for (const k of keys("window", page)) {
        const r = page.find(x => x.key === k);
        assert.ok((r.label + " " + r.desc).toLowerCase().includes("window"), `${k} carries the word`);
    }
    // and a word the page spells only inside one row's description does not
    // surface the rows of the group that word names
    const look = keys("look", page);
    assert.ok(look.every(k => ["disableBlur", "zoomModifier"].includes(k)), "a one-word tab hit stays narrow");
});

test("diacritics fold, so a row drawn in another language is still reachable", () => {
    assert.deepEqual(keys("energia", baked).slice(0, 1), ["saveBattery4"]);
    assert.deepEqual(keys("Ahorro de energía", baked).slice(0, 1), ["saveBattery4"]);
    assert.ok(keys("bateria", baked).includes("saveBattery5"));
});

test("a script the tokeniser cannot read keeps the literal substring", () => {
    // the plain indexOf the sheet used before must not regress
    assert.ok(keys("ズーム", baked).includes("zoomModifier"));
    assert.ok(keys("बैटरी", baked).includes("saveBattery"));
    assert.ok(keys("البطارية", baked).includes("saveBattery2"));
    assert.ok(keys("พลังงาน", baked).includes("saveBattery3"));
});

test("a whitespace-only query returns every row", () => {
    assert.deepEqual(searchRows(page, "  ", context), page);
    assert.deepEqual(searchRows(page, "", context), page);
});

test("fold flattens the Latin letters the catalogs use", () => {
    assert.equal(fold("Energía"), "energia");
    assert.equal(fold("BATERÍA"), "bateria");
    assert.equal(fold("Änderung"), "anderung");
    assert.equal(fold("Żółć"), "zolc");
    assert.equal(fold("țiță"), "tita");
    // scripts it does not know are returned untouched, not mangled
    assert.equal(fold("日本語"), "日本語");
    assert.equal(fold(" energía"), " energia");
    assert.equal(fold("  Batería  "), "  bateria  ");
});