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

test("model suggestions use JSON availability, route names and provider names", () => {
    const data = { routes: [{ id: "auto:fast", name: "Fast" }], models: [
        { id: "provider/model", name: "Model", provider: "provider", available: true },
        { id: "provider/model", available: true },
        { id: "missing/model", available: false },
        { id: "unverified/model", available: "true" },
        { id: "__proto__", available: true }
    ] };
    assert.deepEqual(plain(sight.modelSuggestions(JSON.stringify(data))),
        [{ id: "auto:fast", name: "Fast · auto:fast" }, { id: "provider/model", name: "Model · provider/model" }, { id: "__proto__", name: "__proto__" }]);
    assert.deepEqual(plain(sight.modelSuggestions(data, "provider")), [{ id: "provider/model", name: "Model · provider/model" }]);
    assert.deepEqual(plain(sight.modelSuggestions({})), []);
    assert.throws(() => sight.modelSuggestions("auto:fast\tFast"));
});

test("the picker is bounded while typed search still finds models beyond the first hundred", () => {
    const data = { models: Array.from({ length: 4803 }, (_, i) => ({ id: "p/model" + i, available: true })) };
    assert.equal(sight.modelSuggestions(data).length, 100);
    assert.deepEqual(plain(sight.modelSuggestions(data, "model4802")), [{ id: "p/model4802", name: "p/model4802" }]);
});

test("saved model availability distinguishes absent keys from temporarily unavailable models", () => {
    const data = { routes: [{ id: "auto:smart" }], models: [
        { id: "anthropic/sonnet", provider: "anthropic", available: false },
        { id: "p/ready", provider: "p", available: true }
    ] };
    assert.equal(sight.modelUnavailable(data, "", []), "");
    assert.equal(sight.modelUnavailable(data, "auto:smart", []), "");
    assert.equal(sight.modelUnavailable(data, "p/ready", []), "");
    assert.equal(sight.modelUnavailable(JSON.stringify(data), "anthropic/sonnet", [{ id: "anthropic", configured: false }]), "no-key");
    assert.equal(sight.modelUnavailable(data, "anthropic/sonnet", [{ id: "anthropic", configured: true }]), "unavailable");
    assert.equal(sight.modelUnavailable(data, "unknown/model", []), "unavailable");
    assert.equal(sight.modelUnavailable(data, "anthropic/custom", [{ id: "anthropic", configured: false }]), "no-key");
});

test("unavailable notices use model and provider display names with id fallbacks", () => {
    const data = { models: [{ id: "anthropic/claude-sonnet-5-5", name: "Claude Sonnet 5.5", provider: "anthropic" }] };
    const providers = [{ id: "anthropic", name: "Anthropic", configured: false }];
    assert.deepEqual(plain(sight.modelIdentity(JSON.stringify(data), "anthropic/claude-sonnet-5-5", providers)),
        { model: "Claude Sonnet 5.5", provider: "Anthropic" });
    assert.deepEqual(plain(sight.modelIdentity(data, "anthropic/custom", providers)),
        { model: "anthropic/custom", provider: "Anthropic" });
    assert.deepEqual(plain(sight.modelIdentity({}, "unknown/custom", [])),
        { model: "unknown/custom", provider: "unknown" });
    assert.deepEqual(plain(sight.modelIdentity({ models: [{ id: "p/no-name", provider: "p" }] }, "p/no-name", [{ id: "p" }])),
        { model: "p/no-name", provider: "p" });
});

test("harness model menus stay empty until requested and are bounded and searchable", () => {
    const store = { modelListRequested: false, effectiveEngine: "omp",
        suggestions: Array.from({ length: 997 }, (_, i) => ({ id: "model" + i, name: "Model " + i })) };
    const context = vm.createContext({ store, Sight: sight });
    methodsFrom("SightStore.qml", ["modelOptions"], context);
    assert.deepEqual(plain(context.modelOptions("")), []);
    store.modelListRequested = true;
    assert.equal(context.modelOptions("").length, 100);
    assert.deepEqual(plain(context.modelOptions("model996")), [{ id: "model996", name: "Model 996 · model996" }]);
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

test("Theia defaults keep core gestures on and every optional feature off", () => {
    const defaults = plain(sight.defaults());
    assert.equal(defaults.on, true);
    assert.equal(defaults.enabled, true);
    assert.equal(defaults.highlight, true);
    assert.equal(defaults.tapKey, "Alt_R");
    assert.equal(defaults.highlightKey, "Alt_L");
    assert.equal(defaults["inplace.key"], "Control_R");
    assert.equal(defaults["inplace.action"], "auto");
    assert.equal(defaults["inplace.paste"], true);
    assert.deepEqual(defaults.bubble, ["translate", "explain", "ask"]);
    assert.deepEqual(defaults.pinned, []);
    assert.equal(sight.featureIDs.length, 21);
    assert.equal(new Set(sight.featureIDs).size, 21);
    for (const id of sight.featureIDs)
        assert.equal(defaults["features." + id], false, id);
    defaults.bubble.push("rewrite");
    assert.deepEqual(plain(sight.defaults().bubble), ["translate", "explain", "ask"]);
});

test("SightStore fallbacks match every shell-bound schema row", () => {
    const schema = vm.createContext({});
    vm.runInContext(fs.readFileSync(new URL("../../schema/rashin.js", import.meta.url), "utf8").replace(/^\.pragma .*$/gm, ""), schema);
    const defaults = plain(sight.defaults());
    for (const row of schema.rows.filter(row => row.bind && row.bind.store === "sight")) {
        assert.ok(Object.hasOwn(defaults, row.bind.key), row.id);
        assert.deepEqual(defaults[row.bind.key], plain(row.default), row.id);
    }
    assert.deepEqual(defaults["tone.favorites"], ["formal", "friendly", "confident", "direct", "empathetic"]);
    assert.deepEqual(defaults["deepread.lenses"], ["factcheck", "assumptions", "concepts", "contrasts", "related", "passages", "section"]);
    assert.equal(defaults["readaloud.rate"], 1);
    assert.equal(defaults["voice.use"], true);
    assert.ok(!Object.keys(defaults).some(key => /^(features\.)?(minutes|imagetools|images)(\.|$)/.test(key)));
});

test("Rashin feature snapshots read settings or status without enabling unknown values", () => {
    const direct = plain(sight.readFeatures({ features: { compare: true, research: true, prompts: "true", unknown: true } }));
    assert.equal(direct.compare, true);
    assert.equal(direct.research, true);
    assert.equal(direct.prompts, false);
    assert.equal(direct.collections, false);
    assert.ok(!Object.hasOwn(direct, "unknown"));
    assert.deepEqual(Object.keys(direct).sort(), plain(sight.rashinFeatureIDs).sort());
    assert.equal(sight.rashinFeatureIDs.length, 20);
    assert.ok(!sight.rashinFeatureIDs.includes("images"));
    assert.deepEqual(plain(sight.readFeatures({ settings: { features: { compare: true, research: true, prompts: "true" } } })), direct);
    assert.ok(Object.values(plain(sight.readFeatures({}))).every(value => value === false));
    for (const invalid of [null, [], { features: [] }, { features: "on" }])
        assert.throws(() => sight.readFeatures(invalid));
});

test("the app feature summary reports only app settings and hides while Rashin is off", () => {
    const root = {
        loaded: true, switching: false, featuresReady: true,
        store: { enabled: true }, featuresStore: { enabledCount: 3 }
    };
    const context = vm.createContext({ root, Sight: sight, I18n: { tr: text => text } });
    vm.runInContext("String.prototype.arg = function (value) { return this.replace(/%[1-9]/, value); };", context);
    methodsFrom("../../pages/Rashin.qml", ["prepared", "shown"], context);
    const row = { id: "rashin-app-features", section: "rashin" };
    assert.equal(context.prepared(row).detail, "3 of 20 on");
    assert.equal(context.shown(row), true);
    root.featuresReady = false;
    assert.equal(context.prepared(row).detail, "Rashin's feature settings are not loaded");
    assert.equal(context.shown(row), true);
    root.store.enabled = false;
    assert.equal(context.shown(row), false);
    root.store.enabled = true;
    root.switching = true;
    assert.equal(context.shown(row), false);
});

test("Theia's summary counts its 21 desktop features without app settings", () => {
    const schema = vm.createContext({});
    vm.runInContext(fs.readFileSync(new URL("../../schema/rashin.js", import.meta.url), "utf8").replace(/^\.pragma .*$/gm, ""), schema);
    const rows = plain(schema.rows.filter(row => row.id.startsWith("theia-f-")));
    assert.equal(rows.length, 21);
    assert.ok(rows.every(row => row.bind.store === "sight"));
    const enabled = new Set(["features.summarize", "features.save", "compare", "research"]);
    const root = { visible: true, featureRows: rows };
    const context = vm.createContext({ root, Changes: { value: bind => enabled.has(bind.key) } });
    const source = fs.readFileSync(new URL("TheiaHero.qml", import.meta.url), "utf8");
    const start = source.indexOf("readonly property int featureCount: {");
    const opening = source.indexOf("{", start);
    const end = source.indexOf("\n    }", opening);
    assert.ok(start >= 0 && end > opening);
    const count = () => vm.runInContext("(function () " + source.slice(opening, end + 6) + ")()", context);
    assert.equal(count(), 2);
    enabled.add("features.quiet");
    assert.equal(count(), 3);
    root.visible = false;
    assert.equal(count(), 0);
    assert.ok(source.includes('I18n.tr("%1 of %2 on")'));
});

test("store callbacks finish once even when a transport calls back again", () => {
    const received = [];
    const done = sight.once((ok, error) => received.push([ok, error]));
    done(true);
    done(false, "late error");
    assert.deepEqual(received, [[true, ""]]);
    sight.once(null)(true);
});

function methodsFrom(file, names, context) {
    const source = fs.readFileSync(new URL(file, import.meta.url), "utf8");
    for (const name of names) {
        const start = source.indexOf("    function " + name + "(");
        const end = source.indexOf("\n    }\n", start);
        assert.ok(start >= 0 && end > start, name);
        vm.runInContext(source.slice(start, end + 7), context);
    }
}

test("saved model checks do no work for Auto, hidden pages or harnesses and filter by provider", () => {
    const store = { active: true, effectiveEngine: "providers", savedModel: "", modelIssue: "old", checkedModel: "" };
    const savedModelProc = { running: false };
    const providerProc = { running: false };
    const context = vm.createContext({ store, savedModelProc, providerProc });
    methodsFrom("SightStore.qml", ["checkModel"], context);
    for (const model of ["", "auto", "auto:fast", "auto:smart"]) {
        store.savedModel = model;
        context.checkModel();
        assert.equal(savedModelProc.running, false);
        assert.equal(store.modelIssue, "");
    }
    store.savedModel = "anthropic/claude-sonnet-5-5";
    store.active = false;
    context.checkModel();
    assert.equal(savedModelProc.running, false);
    store.active = true;
    store.effectiveEngine = "harness";
    context.checkModel();
    assert.equal(savedModelProc.running, false);
    store.effectiveEngine = "providers";
    context.checkModel();
    assert.equal(savedModelProc.running, true);
    assert.deepEqual(plain(savedModelProc.command), ["rashin", "call", "models.list", '{"provider":"anthropic"}']);
    assert.equal(store.checkedModel, store.savedModel);
});

function saveHarness({ loaded = true, previous = false, oldSave = false, collectionsOK = true, shellOK = true, restoreOK = true, error = "" } = {}) {
    const writes = [];
    const completed = [];
    let collectionValue = previous;
    let saveValue = oldSave;
    let shellWrites = 0;
    const collections = {
        loaded, error,
        read: () => loaded ? collectionValue : undefined,
        write: (key, value, done) => {
            writes.push(["rashinFeatures", key, value]);
            const ok = writes.length === 1 ? collectionsOK : restoreOK;
            if (ok)
                collectionValue = value;
            done(ok, ok ? "" : "collections failed");
        }
    };
    const store = {
        savingCollection: false,
        read: () => saveValue,
        patch: (key, value, done) => {
            writes.push(["sight", key, value]);
            const ok = shellWrites++ > 0 || shellOK;
            if (ok)
                saveValue = value;
            done(ok, ok ? "" : "shell failed");
        }
    };
    const context = vm.createContext({ store, Changes: { stores: { rashinFeatures: collections } }, Sight: sight, I18n: { tr: text => text } });
    methodsFrom("SightStore.qml", ["changeValue", "writeSave"], context);
    store.changeValue = context.changeValue;
    store.write = (key, value, done) => context.writeSave(value, done);
    function apply(value) {
        context.writeSave(value, sight.once((ok, message) => completed.push([ok, message])));
    }
    return { writes, completed, store, collections, apply };
}

test("Save to Rashin writes both stores and snapshots their independent values", () => {
    const result = saveHarness({ previous: true });
    const before = plain(result.store.changeValue("features.save", false, true));
    const after = plain(result.store.changeValue("features.save", true, false));
    assert.deepEqual(before, { save: false, collections: true });
    assert.deepEqual(after, { save: true, collections: true });
    result.apply(after);
    assert.deepEqual(result.writes, [["rashinFeatures", "collections", true], ["sight", "features.save", true]]);
    assert.deepEqual(result.completed, [[true, ""]]);
    result.apply(before);
    assert.equal(result.store.read("features.save"), false);
    assert.equal(result.collections.read("collections"), true);
    assert.equal(result.store.savingCollection, false);
});

test("paired Reset restores both original values when undone", () => {
    const result = saveHarness({ previous: false, oldSave: true });
    const before = plain(result.store.changeValue("features.save", true, true));
    result.apply(result.store.changeValue("features.save", false, false));
    assert.equal(result.collections.read("collections"), false);
    assert.equal(result.store.read("features.save"), false);
    result.apply(before);
    assert.equal(result.collections.read("collections"), false);
    assert.equal(result.store.read("features.save"), true);
});

test("a failed paired save rolls both stores back independently", () => {
    const result = saveHarness({ shellOK: false, previous: true });
    result.apply(true);
    assert.deepEqual(result.writes, [["rashinFeatures", "collections", true], ["sight", "features.save", true], ["sight", "features.save", false], ["rashinFeatures", "collections", true]]);
    assert.deepEqual(result.completed, [[false, "shell failed"]]);
    assert.equal(result.store.savingCollection, false);
    const refused = saveHarness({ collectionsOK: false });
    refused.apply(true);
    assert.deepEqual(refused.writes, [["rashinFeatures", "collections", true]]);
    assert.deepEqual(refused.completed, [[false, "collections failed"]]);
    const unreadable = saveHarness({ loaded: false });
    unreadable.apply(true);
    assert.deepEqual(unreadable.writes, []);
    assert.equal(unreadable.completed[0][0], false);
    const brokenRollback = saveHarness({ shellOK: false, restoreOK: false });
    brokenRollback.apply(true);
    assert.deepEqual(brokenRollback.completed, [[false, "shell failed collections failed"]]);
});

function featureStoreHarness() {
    const requests = [];
    let touches = 0;
    const store = { active: false, loaded: false, reading: false, writing: false, error: "", features: {}, queue: [], readers: [], refreshPending: false };
    const source = fs.readFileSync(new URL("RashinFeaturesStore.qml", import.meta.url), "utf8");
    const start = source.indexOf("    function read(");
    const end = source.indexOf("    onActiveChanged:", start);
    assert.ok(start >= 0 && end > start);
    const context = vm.createContext({
        store, Sight: sight, Spawn: { env: {} }, I18n: { tr: text => text },
        Changes: { touch: () => touches++, run: (argv, done) => requests.push({ argv: plain(argv), done }) }
    });
    vm.runInContext(source.slice(start, end), context);
    const countExpression = source.match(/readonly property int enabledCount: ([^\n]+)/)[1];
    Object.defineProperty(store, "enabledCount", { get: () => vm.runInContext(countExpression, context) });
    for (const name of ["read", "refresh", "retry", "write", "pump"])
        store[name] = context[name];
    return { store, requests, touches: () => touches };
}

test("Rashin feature reads are on demand and parked reads do not update the UI", () => {
    const harness = featureStoreHarness();
    assert.equal(harness.store.read("compare"), undefined);
    harness.store.refresh();
    assert.equal(harness.requests.length, 0);
    harness.store.active = true;
    harness.store.refresh();
    harness.store.refresh();
    assert.equal(harness.requests.length, 1);
    assert.deepEqual(harness.requests[0].argv, ["rashin", "status", "--json"]);
    harness.store.active = false;
    harness.requests[0].done(true, '{"settings":{"features":{"compare":true}}}', "");
    assert.equal(harness.store.loaded, false);
    assert.equal(harness.touches(), 0);
    const completed = [];
    harness.store.refresh((ok, error) => completed.push([ok, error]));
    assert.equal(harness.requests.length, 2);
    harness.requests[1].done(true, '{"settings":{"features":{"compare":true}}}', "");
    assert.deepEqual(completed, [[true, ""]]);
    assert.equal(harness.store.read("compare"), true);
    assert.equal(harness.store.read("prompts"), false);
    assert.equal(harness.store.enabledCount, 1);
    assert.equal(harness.store.read("unknown"), undefined);
    assert.equal(harness.touches(), 0);
});

test("Rashin feature writes are merge patches serialized without losing other features", () => {
    const harness = featureStoreHarness();
    harness.store.loaded = true;
    const completed = [];
    harness.store.write("compare", true, (ok, error) => completed.push(["compare", ok, error]));
    harness.store.write("prompts", true, (ok, error) => completed.push(["prompts", ok, error]));
    assert.equal(harness.requests.length, 1);
    assert.deepEqual(harness.requests[0].argv, ["rashin", "call", "settings.set", '{"patch":{"features":{"compare":true}}}']);
    harness.requests[0].done(true, "{}", "");
    assert.equal(harness.requests.length, 2);
    assert.deepEqual(harness.requests[1].argv, ["rashin", "call", "settings.set", '{"patch":{"features":{"prompts":true}}}']);
    harness.requests[1].done(true, "{}", "");
    assert.deepEqual(completed, [["compare", true, ""], ["prompts", true, ""]]);
    assert.equal(harness.store.read("compare"), true);
    assert.equal(harness.store.read("prompts"), true);
    assert.equal(harness.store.enabledCount, 2);
    assert.equal(harness.store.writing, false);
    assert.equal(harness.touches(), 2);
    harness.store.write("not-a-feature", true, (ok, error) => completed.push(["invalid", ok, error]));
    assert.equal(harness.requests.length, 2);
    assert.equal(completed[2][1], false);
});

test("Rashin feature write errors preserve the last snapshot and finish once", () => {
    const harness = featureStoreHarness();
    harness.store.loaded = true;
    const completed = [];
    harness.store.features = { compare: true };
    harness.store.write("compare", false, (ok, error) => completed.push([ok, error]));
    harness.requests[0].done(false, "", "not connected");
    assert.equal(completed.length, 1);
    assert.equal(completed[0][0], false);
    assert.ok(completed[0][1].includes("not connected"));
    assert.equal(harness.store.read("compare"), true);
});

test("unknown or failed Rashin snapshots block writes until a successful Retry", () => {
    const harness = featureStoreHarness();
    const completed = [];
    harness.store.write("compare", true, (ok, error) => completed.push([ok, error]));
    assert.equal(completed.length, 1);
    assert.equal(completed[0][0], false);
    assert.equal(harness.requests.length, 0);
    assert.equal(harness.store.read("compare"), undefined);
    harness.store.active = true;
    harness.store.retry();
    harness.requests[0].done(false, "", "not installed");
    assert.equal(harness.store.loaded, false);
    assert.equal(harness.store.read("compare"), undefined);
    assert.ok(harness.store.error.includes("not installed"));
    harness.store.write("compare", true, (ok, error) => completed.push([ok, error]));
    assert.equal(harness.requests.length, 1);
    harness.store.retry();
    assert.ok(harness.store.error.includes("not installed"));
    harness.requests[1].done(true, '{"settings":{"features":{"compare":true}}}', "");
    assert.equal(harness.store.loaded, true);
    assert.equal(harness.store.error, "");
    assert.equal(harness.store.read("compare"), true);
    harness.store.retry();
    harness.requests[2].done(true, '{"running":false}', "");
    assert.ok(harness.store.error.length > 0);
    assert.equal(harness.store.read("compare"), true);
    harness.store.write("compare", false, (ok, error) => completed.push([ok, error]));
    assert.equal(harness.requests.length, 3);
    assert.equal(completed[2][0], false);
});

function changeHarness(store) {
    const root = {
        stores: { sight: store }, undoStack: [], recent: [], revision: 0, writingChange: false,
        clone: value => value === undefined ? undefined : plain(value),
        same: (a, b) => JSON.stringify(a) === JSON.stringify(b),
        stable: value => JSON.stringify(value),
        value: bind => store.read(bind.key),
        keyOf: bind => bind.store + ":" + bind.key,
        isChanged: row => JSON.stringify(store.read(row.bind.key)) !== JSON.stringify(row.default),
        defaultOf: row => row.default,
        relist: () => {}, announced: () => {}, undone: () => {},
        write: (bind, value, label, page, settled) => store.write(bind.key, value, ok => settled?.(ok))
    };
    const context = vm.createContext({ root, I18n: { tr: text => text } });
    methodsFrom("../../Singletons/Changes.qml", ["changeValue", "set", "writeChange", "resetPage", "reverted", "undoEntry", "sameSetting", "isMap"], context);
    for (const name of ["changeValue", "set", "writeChange", "resetPage", "reverted", "undoEntry", "sameSetting", "isMap"])
        root[name] = context[name];
    return root;
}

test("Changes records one composite Save entry and Undo preserves pre-existing Collections", () => {
    const harness = saveHarness({ previous: true });
    const changes = changeHarness(harness.store);
    const bind = { store: "sight", key: "features.save", fallback: false };
    changes.set(bind, true, "Save to Rashin", "rashin");
    assert.equal(changes.undoStack.length, 1);
    assert.deepEqual(plain(changes.undoStack[0].before), { save: false, collections: true });
    assert.deepEqual(plain(changes.undoStack[0].after), { save: true, collections: true });
    changes.undoEntry(changes.undoStack[0]);
    assert.equal(harness.store.read("features.save"), false);
    assert.equal(harness.collections.read("collections"), true);
});

test("page Reset records each original Save and Collections value in its composite part", () => {
    const harness = saveHarness({ oldSave: true, previous: false });
    const changes = changeHarness(harness.store);
    const row = { bind: { store: "sight", key: "features.save" }, default: false, label: "Save to Rashin" };
    changes.resetPage([row], "rashin", "Rashin");
    assert.equal(changes.undoStack.length, 1);
    assert.deepEqual(plain(changes.undoStack[0].parts[0].before), { save: true, collections: false });
    assert.deepEqual(plain(changes.undoStack[0].parts[0].after), { save: false, collections: false });
    assert.equal(harness.store.read("features.save"), false);
    changes.undoEntry(changes.undoStack[0]);
    assert.equal(harness.store.read("features.save"), true);
    assert.equal(harness.collections.read("collections"), false);
});

test("Changes keeps the existing scalar lifecycle for stores without the hook", () => {
    let value = false;
    const store = { read: () => value, write: (key, next, done) => { value = next; done(true); } };
    const changes = changeHarness(store);
    const bind = { store: "sight", key: "enabled" };
    changes.set(bind, true, "Enabled", "rashin");
    assert.equal(changes.undoStack[0].before, false);
    assert.equal(changes.undoStack[0].after, true);
    assert.equal(value, true);
    changes.undoEntry(changes.undoStack[0]);
    assert.equal(value, false);
    value = true;
    changes.resetPage([{ bind, default: false, label: "Enabled" }], "rashin", "Rashin");
    assert.equal(value, false);
    changes.undoEntry(changes.undoStack[0]);
    assert.equal(value, true);
});

function gestureHarness(values = {}) {
    const current = Object.assign({ tapKey: "Alt_R", highlightKey: "Alt_L", "inplace.key": "Control_R" }, values);
    const writes = [];
    const store = {
        defaults: plain(sight.defaults()), gestureWriting: false, gestureQueue: [],
        read: key => current[key],
        patch: (key, value, done) => { current[key] = value; writes.push([key, value]); done(true, ""); }
    };
    const context = vm.createContext({ store, Sight: sight, I18n: { tr: text => text } });
    methodsFrom("SightStore.qml", ["changeValue", "gestureOwner", "writeGesture", "pumpGesture"], context);
    for (const name of ["changeValue", "gestureOwner", "writeGesture", "pumpGesture"])
        store[name] = context[name];
    store.write = store.writeGesture;
    return { store, current, writes };
}

test("gesture Reset hands the held default key over and Undo restores both keys", () => {
    const harness = gestureHarness({ tapKey: "Super_R", highlightKey: "Alt_R" });
    const changes = changeHarness(harness.store);
    const bind = { store: "sight", key: "tapKey" };
    changes.set(bind, "Alt_R", "Look key", "rashin");
    assert.equal(harness.current.tapKey, "Alt_R");
    assert.equal(harness.current.highlightKey, "Super_R");
    assert.equal(new Set(Object.values(harness.current)).size, 3);
    changes.undoEntry(changes.undoStack[0]);
    assert.equal(harness.current.tapKey, "Super_R");
    assert.equal(harness.current.highlightKey, "Alt_R");
    assert.equal(new Set(Object.values(harness.current)).size, 3);
});

test("gesture writes reject reserved nondefault keys even when a gesture is off", () => {
    const harness = gestureHarness();
    const completed = [];
    harness.store.writeGesture("tapKey", "Control_R", (ok, error) => completed.push([ok, error]));
    assert.equal(completed[0][0], false);
    assert.equal(harness.writes.length, 0);
    harness.store.writeGesture("tapKey", "Super_R", (ok, error) => completed.push([ok, error]));
    assert.equal(completed[1][0], true);
    assert.equal(harness.current.tapKey, "Super_R");
});

test("page Reset resolves a three-gesture cycle and Undo restores it", () => {
    const harness = gestureHarness({ tapKey: "Control_R", highlightKey: "Alt_R", "inplace.key": "Alt_L" });
    const changes = changeHarness(harness.store);
    const rows = ["tapKey", "highlightKey", "inplace.key"].map(key => ({
        bind: { store: "sight", key }, default: harness.store.defaults[key], label: key
    }));
    changes.resetPage(rows, "rashin", "Rashin");
    assert.deepEqual(harness.current, { tapKey: "Alt_R", highlightKey: "Alt_L", "inplace.key": "Control_R" });
    changes.undoEntry(changes.undoStack[0]);
    assert.deepEqual(harness.current, { tapKey: "Control_R", highlightKey: "Alt_R", "inplace.key": "Alt_L" });
});
