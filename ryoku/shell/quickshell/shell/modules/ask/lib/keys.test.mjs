import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const K = require("./keys.js");

function equal(actual, expected, label) {
    if (JSON.stringify(actual) !== JSON.stringify(expected)) {
        console.error(label, "\nexpected", expected, "\nactual", actual);
        process.exitCode = 1;
    }
}

const KEY_K = 0x4b, KEY_C = 0x43, KEY_1 = 0x31, RETURN = 0x01000004, ENTER = 0x01000005, TAB = 0x01000001, BACKTAB = 0x01000002, PERIOD = 0x2e;

equal(K.chord(KEY_K, K.CTRL), "Ctrl+K", "a control chord");
equal(K.chord(KEY_C, K.CTRL | K.SHIFT), "Ctrl+Shift+C", "modifiers come in a fixed order");
equal(K.chord(RETURN, 0), "Enter", "Return is Enter");
equal(K.chord(ENTER, K.CTRL), "Ctrl+Enter", "the keypad Enter too");
equal(K.chord(BACKTAB, K.CTRL | K.SHIFT), "Ctrl+Shift+Tab", "Backtab is Shift+Tab");
equal(K.chord(BACKTAB, K.CTRL), "Ctrl+Shift+Tab", "Backtab implies Shift");
equal(K.chord(KEY_1, K.CTRL), "Ctrl+1", "digits");
equal(K.chord(PERIOD, K.CTRL), "Ctrl+.", "punctuation");
equal(K.chord(0x01000020, K.SHIFT), "", "a lone modifier key is no chord");

equal(K.normalize("shift + ctrl + c"), "Ctrl+Shift+C", "a hint is normalised");
equal(K.normalize(""), "", "an empty hint stays empty");
equal(K.find([{ id: "tabs", hint: "ctrl + Tab" }], TAB, K.CTRL).id, "tabs", "a hint written loosely still matches its chord");
equal(K.find([{ id: "k", hint: "Ctrl+K" }], KEY_K, K.CTRL | K.SHIFT), null, "extra modifiers do not match");

equal(K.caps("Ctrl+Shift+C"), ["Ctrl", "Shift", "C"], "a hint splits into caps");
equal(K.caps("Enter"), ["Enter"], "a single key is one cap");
equal(K.caps(""), [], "no hint, no caps");

const actions = [{ id: "copy", hint: "Ctrl+Shift+C" }, { id: "open", hint: "Ctrl+O" }, { id: "plain" }];
equal(K.find(actions, 0x4f, K.CTRL).id, "open", "an action is found by its chord");
equal(K.find(actions, 0x4f, 0), null, "a bare letter finds nothing");
equal(K.find(null, KEY_K, K.CTRL), null, "no actions, nothing found");
