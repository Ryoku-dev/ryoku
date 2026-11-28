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
const { fold, searchRows, highlightHtml, textHasQuery } = context;

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
const keys = (q, rows) => searchKeys(searchRows(rows, q), r => r.key);

test("the row its words name comes first", () => {
    assert.deepEqual(keys("blur", page).slice(0, 1), ["disableBlur"]);
    assert.deepEqual(keys("zoom modifier", page).slice(0, 1), ["zoomModifier"]);
});

test("a typo in the typed query still lands", () => {
    assert.deepEqual(keys("disabel blur", page).slice(0, 1), ["disableBlur"]);
    assert.deepEqual(keys("batery", page).slice(0, 1), ["saveBattery"]);
});

test("a row that answers every word wins over one that answers a single word", () => {
    // "window gap" is answered in full by "Window gaps", so the rows carrying
    // only "window" are not offered at all; before the coverage scaling they
    // were, and one of them outranked the row that answered both
    assert.deepEqual(keys("window gap", page), ["gapIn"]);
    assert.deepEqual(keys("close window", page)[0], "closeWindow");
    // and when nothing answers the whole query, the partial ones still follow
    assert.ok(keys("window frosted", page).includes("gapIn"), "the partial follows");
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
// A Spanish catalog for the page's source rows: what I18n.tr returns once the
// language is switched, which is the second half of every row's haystack.
const es = {
    "Disable blur": "Desactivar desenfoque",
    "Drops the frosted-glass look everywhere": "Quita el aspecto de cristal en todas partes",
    "Window gaps": "Separación de ventanas",
    "Battery saver": "Ahorro de batería",
    "Colour scheme": "Esquema de color",
    "Light, dark or auto": "Claro, oscuro o automático",
    "Window opacity": "Opacidad de ventana",
    "How see-through a window is": "Cuán transparente es una ventana",
};
const trEs = (s) => (s === undefined || s === null || s === "") ? s : (es[s] || s);

const searchKeys = (arr) => Array.prototype.map.call(arr, r => r.key);

test("a query in the language the row is drawn in reaches it", () => {
    const es_ = (q) => searchKeys(searchRows(page, q, trEs, "es"));
    assert.deepEqual(es_("desenfoque").slice(0, 1), ["disableBlur"]);
    assert.deepEqual(es_("opacidad").slice(0, 1), ["winOpacity"]);
    // a typo still lands in the translated text: "trasparency" -> transparency
    // -> the "opacity" of "Opacidad de ventana"
    assert.ok(es_("trasparency").includes("winOpacity"));
    // and the English source keeps working while the UI is in Spanish
    assert.deepEqual(es_("blur").slice(0, 1), ["disableBlur"]);
    // accents fold on the translated side too
    assert.deepEqual(searchKeys(searchRows(baked, "ahorro de energia", trEs, "es")).slice(0, 1), ["saveBattery4"]);
});

test("the per-row cache is rebuilt when the language changes", () => {
    const keysEs = (q) => searchKeys(searchRows(page, q, trEs, "es"));
    // the same row objects, searched in English first and then in Spanish; a
    // cache that survived the switch would keep answering only English
    assert.deepEqual(keys("blur", page).slice(0, 1), ["disableBlur"]);
    assert.deepEqual(keysEs("desenfoque").slice(0, 1), ["disableBlur"]);
    assert.deepEqual(keysEs("blur").slice(0, 1), ["disableBlur"]);
});

test("highlight underlines the run the query names, in its own spelling", () => {
    assert.equal(highlightHtml("Desactivar desenfoque", "desenfoque"), "Desactivar <u>desenfoque</u>");
    // folded: "energia" underlines "Energía" without slicing it
    assert.equal(highlightHtml("Energía", "energia"), "<u>Energía</u>");
    // a two-word query underlines the phrase first, then each word
    assert.equal(highlightHtml("Separación de ventanas", "ventanas"), "Separación de <u>ventanas</u>");
    assert.equal(highlightHtml("Window gaps", "window gap"), "<u>Window gap</u>s");
    // markup in the label is escaped, never interpreted
    // markup in the label is escaped, never interpreted
    assert.equal(highlightHtml("Blur & <glow>", "blur"),
        "<u>Blur</u> &amp; " + "&lt;glow&gt;");
    // no match leaves the text intact
    assert.equal(highlightHtml("Blur", "zzz"), "Blur");
    assert.equal(highlightHtml("", "zzz"), "");
});

test("a row's search synonyms are read in the language they are drawn in", () => {
    // the catalog now translates a row's `keywords` (i18n tools harvest them the
    // way they harvest label/desc), so a query in the drawn language meets the
    // synonyms too, not only the label the user can already see
    const esKw = { "battery performance power saving": "bateria rendimiento energia ahorro" };
    const trKw = (s) => (s === undefined || s === null || s === "") ? s : (esKw[s] || s);
    const hit = (q) => searchKeys(searchRows(page, q, trKw, "es"));
    assert.ok(hit("ahorro").includes("saveBattery"), "the translated keyword reaches its row");
    // and the English synonyms keep working while the UI is in Spanish
    assert.ok(hit("power saving").includes("saveBattery"));
    // a synonym the row carries in English only is not a false hit
    assert.ok(!hit("zzzqqq").includes("saveBattery"));
});

test("textHasQuery answers what highlightHtml would underline", () => {
    assert.ok(textHasQuery("Energía", "energia"));
    assert.ok(textHasQuery("Window gaps", "gap"));
    assert.ok(!textHasQuery("Window gaps", "zzz"));
    assert.ok(!textHasQuery("Window gaps", "   "));
});
