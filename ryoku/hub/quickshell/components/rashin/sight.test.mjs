import assert from "node:assert/strict";
import test from "node:test";
import fs from "node:fs";
import vm from "node:vm";

const sight = vm.createContext({});
vm.runInContext(fs.readFileSync(new URL("sight.js", import.meta.url), "utf8").replace(/^\.pragma .*$/gm, ""), sight);
const plain = value => JSON.parse(JSON.stringify(value));

test("only selected, missing, curated OCR data is installed, once", () => {
    assert.deepEqual(plain(sight.missingLanguages(["eng", "jpn", "jpn", "chi_sim", "../../other", "unlisted"], ["eng"])),
        ["jpn", "chi_sim"]);
    assert.deepEqual(plain(sight.missingLanguages(["jpn"], ["eng", "jpn"])), []);
});

test("disabling languages never asks for a package removal or installation", () => {
    assert.deepEqual(plain(sight.missingLanguages([], ["eng", "jpn"])), []);
    assert.deepEqual(plain(sight.missingLanguages(["eng"], ["eng", "jpn"])), []);
});

test("model suggestions parse routes and provider rows without imposing a default", () => {
    assert.deepEqual(plain(sight.modelSuggestions("auto:fast\tFastest\nprovider/model\tfree\tready\nprovider/model\tfree\tready\n\n")),
        [{ id: "auto:fast", name: "auto:fast" }, { id: "provider/model", name: "provider/model" }]);
    assert.deepEqual(plain(sight.modelSuggestions("")), []);
    assert.deepEqual(plain(sight.modelSuggestions("toString\tready\n__proto__\tready")),
        [{ id: "toString", name: "toString" }, { id: "__proto__", name: "__proto__" }]);
});

const langs = JSON.parse(fs.readFileSync(new URL("../../../../i18n/langs.json", import.meta.url), "utf8")).languages;

test("translation targets use every canonical language code after Your language", () => {
    const options = plain(sight.targetOptions(langs, text => text));
    assert.deepEqual(options[0], { label: "Your language", value: "" });
    assert.deepEqual(options.slice(1).map(option => option.value), langs.map(language => language.code));
    for (const language of langs) {
        const option = options.find(option => option.value === language.code);
        assert.equal(option.label, language.name + (language.native !== language.name ? " · " + language.native : ""));
    }
});

test("translation labels use the translated name and keep a distinct native name", () => {
    const tr = text => ({ "Your language": "Your translated language", "German": "Allemand", "English": "English" }[text] || text);
    assert.deepEqual(plain(sight.targetOptions([
        { code: "de", name: "German", native: "Deutsch" },
        { code: "en", name: "English", native: "English" }
    ], tr)), [
        { label: "Your translated language", value: "" },
        { label: "Allemand · Deutsch", value: "de" },
        { label: "English", value: "en" }
    ]);
    assert.deepEqual(plain(sight.targetOptions([{ code: "de", name: "German", native: "Deutsch" }], text => text === "German" ? "Deutsch" : text))[1],
        { label: "Deutsch", value: "de" });
});

test("stored translation targets map old Chinese codes and fall back without rewriting preferences", () => {
    assert.equal(sight.targetCode("zh-Hans", langs), "zh_CN");
    assert.equal(sight.targetCode("zh-Hant", langs), "zh_TW");
    for (const language of langs)
        assert.equal(sight.targetCode(language.code, langs), language.code);
    for (const value of ["", "unlisted", "zh-CN", null, undefined])
        assert.equal(sight.targetCode(value, langs), "");
});
