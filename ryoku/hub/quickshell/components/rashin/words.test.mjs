import assert from "node:assert/strict";
import test from "node:test";
import fs from "node:fs";
import vm from "node:vm";

const plain = value => JSON.parse(JSON.stringify(value));
const source = file => fs.readFileSync(new URL(file, import.meta.url), "utf8");

function methodsFrom(file, names, context) {
    const qml = source(file);
    for (const name of names) {
        const start = qml.indexOf("    function " + name + "(");
        const end = qml.indexOf("\n    }\n", start);
        assert.ok(start >= 0 && end > start, name);
        vm.runInContext(qml.slice(start, end + 7), context);
    }
}

function wordsHarness() {
    const requests = [];
    const field = { text: "" };
    const root = {
        visible: true, enabled: true, store: { active: true }, request: null,
        generation: 0, words: [], loaded: false, error: ""
    };
    const context = vm.createContext({
        root, wordField: field, I18n: { tr: text => text },
        Changes: { run: (argv, done) => {
            const pending = { running: true };
            requests.push({ argv: plain(argv), done, pending });
            return pending;
        } }
    });
    for (const name of ["live", "busy", "editing"]) {
        const expression = source("TheiaWords.qml").match(new RegExp("readonly property bool " + name + ": ([^\\n]+)"))[1];
        Object.defineProperty(root, name, { get: () => vm.runInContext(expression, context) });
    }
    const names = ["call", "refresh", "add", "remove", "stop"];
    methodsFrom("TheiaWords.qml", names, context);
    for (const name of names)
        root[name] = context[name];
    return { root, field, requests };
}

test("Your words reads only while visible, enabled and active, with one process at a time", () => {
    const { root, requests } = wordsHarness();
    for (const [object, key] of [[root, "visible"], [root, "enabled"], [root.store, "active"]]) {
        object[key] = false;
        root.refresh();
        root.call("writing.words.add", { word: "Ryoku" }, null);
        assert.equal(requests.length, 0);
        object[key] = true;
    }
    root.refresh();
    root.refresh();
    assert.deepEqual(requests.map(request => request.argv), [["rashin", "call", "writing.words", "{}"]]);
    requests[0].done(true, '{"words":["Ryoku"]}', "");
    assert.deepEqual(plain(root.words), ["Ryoku"]);
    assert.equal(root.loaded, true);
    assert.equal(root.busy, false);
});

test("Your words adds and removes through exact word parameters and adopts the returned list", () => {
    const { root, field, requests } = wordsHarness();
    root.refresh();
    requests[0].done(true, '{"words":[]}', "");
    field.text = "  Ryoku  ";
    root.add();
    assert.deepEqual(requests[1].argv, ["rashin", "call", "writing.words.add", '{"word":"Ryoku"}']);
    requests[1].done(true, '{"words":["Ryoku"]}', "");
    assert.equal(field.text, "");
    assert.deepEqual(plain(root.words), ["Ryoku"]);
    root.remove("Ryoku");
    assert.deepEqual(requests[2].argv, ["rashin", "call", "writing.words.remove", '{"word":"Ryoku"}']);
    requests[2].done(true, '{"words":[]}', "");
    assert.deepEqual(plain(root.words), []);
});

test("Your words keeps drafts and the last list when an operation fails", () => {
    const { root, field, requests } = wordsHarness();
    root.refresh();
    requests[0].done(true, '{"words":["Ryoku"]}', "");
    field.text = "newword";
    root.refresh();
    assert.equal(requests.length, 1);
    root.add();
    requests[1].done(false, "", "not connected");
    assert.equal(field.text, "newword");
    assert.match(root.error, /not connected/);
    assert.deepEqual(plain(root.words), ["Ryoku"]);
    root.add();
    requests[2].done(true, '{"words":[23]}', "");
    assert.match(root.error, /unreadable/);
    assert.equal(field.text, "newword");
    assert.deepEqual(plain(root.words), ["Ryoku"]);
});

test("parking Your words stops its process and discards stale callbacks", () => {
    const { root, requests } = wordsHarness();
    root.refresh();
    const pending = requests[0];
    root.visible = false;
    root.stop();
    assert.equal(pending.pending.running, false);
    assert.equal(root.busy, false);
    pending.done(true, '{"words":["stale"]}', "");
    assert.equal(root.loaded, false);
    root.visible = true;
    root.refresh();
    requests[1].done(true, '{"words":["fresh"]}', "");
    pending.done(false, "", "late failure");
    assert.deepEqual(plain(root.words), ["fresh"]);
    assert.equal(root.error, "");
});

const shellPrefix = "../../../../shell/quickshell/shell/modules/sight/";

function issueHarness() {
    const calls = [];
    const checks = [];
    const root = {
        addingWord: false, checking: false, wordError: "", currentText: "😀 Ryoku is here",
        lifetime: { closed: false, serial: 1 }, run: keep => checks.push(keep)
    };
    const context = vm.createContext({
        root, fixMotion: { running: false }, I18n: { tr: text => text },
        session: { addWord: (word, done) => calls.push({ word, done }) }
    });
    for (const key of Object.keys(root))
        Object.defineProperty(context, key, { get: () => root[key], set: value => { root[key] = value; }, configurable: true });
    methodsFrom(shellPrefix + "TheiaIssues.qml", ["addWord"], context);
    root.addWord = context.addWord;
    return { root, calls, checks };
}

test("spelling Add to dictionary extracts UTF-16 spans and rechecks only after saving", () => {
    const { root, calls, checks } = issueHarness();
    root.addWord({ category: "grammar", start: 3, end: 8 });
    assert.equal(calls.length, 0);
    root.addWord({ category: "spelling", start: 3, end: 8 });
    assert.equal(calls[0].word, "Ryoku");
    assert.equal(root.addingWord, true);
    assert.deepEqual(checks, []);
    root.addWord({ category: "spelling", start: 3, end: 8 });
    assert.equal(calls.length, 1);
    calls[0].done({ words: ["Ryoku"] }, null);
    assert.equal(root.addingWord, false);
    assert.deepEqual(checks, [true]);
});

test("dictionary issue failures and stale replies never recheck a different card", () => {
    const { root, calls, checks } = issueHarness();
    const issue = { category: "spelling", start: 3, end: 8 };
    root.addWord(issue);
    calls[0].done(null, { message: "not connected" });
    assert.equal(root.wordError, "not connected");
    assert.equal(root.addingWord, false);
    assert.deepEqual(checks, []);
    root.addWord(issue);
    ++root.lifetime.serial;
    calls[1].done({ words: ["Ryoku"] }, null);
    assert.deepEqual(checks, []);
    root.lifetime.closed = true;
    calls[1].done({ words: ["Ryoku"] }, null);
    assert.deepEqual(checks, []);
});

test("the session dictionary helper sends only a word and drops replies after disposal", () => {
    const calls = [];
    const replies = [];
    const lease = { closed: false };
    const client = { ready: true, call: (method, params, done) => calls.push({ method, params: plain(params), done }) };
    const context = vm.createContext({ lease, client, I18n: { tr: text => text } });
    methodsFrom(shellPrefix + "SightSession.qml", ["addWord"], context);
    context.addWord("Ryoku", (...args) => replies.push(args));
    assert.equal(calls[0].method, "writing.words.add");
    assert.deepEqual(calls[0].params, { word: "Ryoku" });
    calls[0].done({ words: ["Ryoku"] }, null);
    assert.equal(replies.length, 1);
    lease.closed = true;
    calls[0].done({}, null);
    assert.equal(replies.length, 1);
    context.addWord("ignored", () => assert.fail("closed session callback"));
    assert.equal(calls.length, 1);
    lease.closed = false;
    client.ready = false;
    context.addWord("Ryoku", (result, error) => assert.match(error.message, /not connected/));
    assert.equal(calls.length, 1);
});

test("a cached writing voice refresh does not insert a temporary loading row above a landing", () => {
    const binding = source("TheiaVoice.qml").match(/Row \{\s+visible: (.+)\n\s+spacing: LithicTokens\.space\.s2\s+Working/)[1];
    const visible = root => vm.runInNewContext(binding, { root });
    assert.equal(visible({ busy: true, operation: "read", loaded: true }), false);
    assert.equal(visible({ busy: true, operation: "read", loaded: false }), true);
    assert.equal(visible({ busy: true, operation: "save", loaded: true }), true);
    assert.equal(visible({ busy: true, operation: "learn", loaded: true }), true);
    assert.equal(visible({ busy: false, operation: "", loaded: true }), false);
});
