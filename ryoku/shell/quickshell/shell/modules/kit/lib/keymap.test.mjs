import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const K = require("./keymap.js");
const S = require("./sections.js");
const Chords = require("../../ask/lib/keys.js");

function equal(actual, expected, label) {
    if (JSON.stringify(actual) !== JSON.stringify(expected)) {
        console.error(label, "\nexpected", expected, "\nactual", actual);
        process.exitCode = 1;
    }
}

const KEY = { A: 0x41, F: 0x46, K: 0x4b, N: 0x4e, ONE: 0x31, SIX: 0x36, SEVEN: 0x37, ESC: 0x01000000, TAB: 0x01000001, BACKTAB: 0x01000002, RETURN: 0x01000004, ENTER: 0x01000005, UP: 0x01000013, DOWN: 0x01000015, PGUP: 0x01000016, PGDN: 0x01000017 };
const field = { field: true };

// a key event as the frame sees it: named by the bar's chord(), then resolved
function press(key, mods, ctx, text) {
    return K.resolve(Chords.chord(key, mods), text || "", ctx);
}

// Ctrl+1..6: sections in rail order
for (let i = 0; i < 6; i++)
    equal(press(KEY.ONE + i, Chords.CTRL, field), { op: "section", arg: i }, "Ctrl+" + (i + 1) + " is " + S.ORDER[i]);
equal(press(KEY.SEVEN, Chords.CTRL, field), null, "Ctrl+7 is nobody's");
equal(K.GLOBAL.filter(entry => entry.op === "section").length, S.ORDER.length, "one chord per section, no more");

equal(press(KEY.TAB, Chords.CTRL, field), { op: "sectionStep", arg: 1 }, "Ctrl+Tab: next section");
equal(press(KEY.TAB, Chords.CTRL | Chords.SHIFT, field), { op: "sectionStep", arg: -1 }, "Ctrl+Shift+Tab: previous section");
equal(press(KEY.BACKTAB, Chords.CTRL | Chords.SHIFT, field), { op: "sectionStep", arg: -1 }, "the Backtab Qt sends for it too");
equal(press(KEY.PGDN, Chords.CTRL, field), { op: "pageStep", arg: 1 }, "Ctrl+PgDn: next page");
equal(press(KEY.PGUP, Chords.CTRL, field), { op: "pageStep", arg: -1 }, "Ctrl+PgUp: previous page");
equal(press(KEY.F, Chords.CTRL, field), { op: "all", arg: 0 }, "Ctrl+F: all of Kit");
equal(press(KEY.K, Chords.CTRL, field), { op: "panel", arg: 0 }, "Ctrl+K: actions");

equal(press(KEY.DOWN, 0, field), { op: "move", arg: 1 }, "Down: next row");
equal(press(KEY.UP, 0, field), { op: "move", arg: -1 }, "Up: previous row");
equal(press(KEY.DOWN, Chords.CTRL, field), { op: "jump", arg: 1 }, "Ctrl+Down: next group");
equal(press(KEY.PGDN, 0, field), { op: "rows", arg: 1 }, "PgDn alone pages the rows, not the section's pages");
equal(press(KEY.RETURN, 0, field), { op: "activate", arg: 0 }, "Enter: the obvious action");
equal(press(KEY.ENTER, 0, field), { op: "activate", arg: 0 }, "the keypad Enter too");
equal(press(KEY.RETURN, Chords.CTRL, field), { op: "secondary", arg: 0 }, "Ctrl+Enter: the second");
equal(press(KEY.ESC, 0, field), { op: "escape", arg: 0 }, "Esc steps back");
equal(press(KEY.ESC, 0, { panel: true }), { op: "escape", arg: 0 }, "from the panel too");
equal(press(KEY.ESC, 0, { textEntry: true, ownsText: true }), { op: "escape", arg: 0 }, "and out of a pane");

// the frame's chords reach it from anywhere: a pane, the typing test, the panel
for (const ctx of [{ textEntry: true }, { ownsText: true, textEntry: true }, { panel: true }, { all: true, field: true }]) {
    equal(press(KEY.ONE + 2, Chords.CTRL, ctx), { op: "section", arg: 2 }, "Ctrl+3 from " + JSON.stringify(ctx));
    equal(press(KEY.TAB, Chords.CTRL, ctx), { op: "sectionStep", arg: 1 }, "Ctrl+Tab from " + JSON.stringify(ctx));
    equal(press(KEY.F, Chords.CTRL, ctx), { op: "all", arg: 0 }, "Ctrl+F from " + JSON.stringify(ctx));
    equal(press(KEY.K, Chords.CTRL, ctx), { op: "panel", arg: 0 }, "Ctrl+K from " + JSON.stringify(ctx));
}

// a pane, a form or the open panel keeps every other key
equal(press(KEY.RETURN, 0, { textEntry: true }), null, "Enter in a form is the form's");
equal(press(KEY.DOWN, 0, { textEntry: true }), null, "arrows in a pane are the pane's");
equal(press(KEY.A, 0, { textEntry: true }, "a"), null, "letters in a pane stay in it");
equal(press(KEY.RETURN, 0, { panel: true }), null, "Enter belongs to the open panel");
equal(press(KEY.DOWN, 0, { panel: true }), null, "so do the arrows");
equal(press(KEY.A, 0, { panel: true }, "a"), null, "and what is typed filters it");

// printable keys go to the field unless the section owns text entry
equal(press(KEY.A, 0, {}, "a"), { op: "type", arg: "a" }, "a letter typed with the keyboard idle lands in the field");
equal(press(KEY.A, Chords.SHIFT, {}, "A"), { op: "type", arg: "A" }, "a capital too");
equal(press(KEY.A, 0, field, "a"), null, "the field types its own letters");
equal(press(KEY.A, 0, { ownsText: true }, "a"), null, "the typing test keeps its letters out of the field");
equal(press(KEY.A, 0, { ownsText: true, all: true }, "a"), { op: "type", arg: "a" }, "but all of Kit searches over any section");
equal(press(KEY.A, Chords.CTRL, {}, "\u0001"), null, "a control chord types nothing");
equal(press(KEY.A, Chords.ALT, {}, "a"), null, "nor an Alt chord");
equal(press(0x01000020, 0, {}, ""), null, "a lone modifier does nothing");
equal(K.printable("\u007f"), false, "Delete is not text");
equal(K.printable("\r"), false, "nor a control character");
equal(K.printable("é"), true, "an accented letter is");

// a section's own chords, read from the hints it shows
const section = { field: true, primaryHint: "Tab", secondaryHint: "Ctrl+Shift+C", actions: [{ id: "new", hint: "Ctrl+N" }, { id: "plain", hint: "" }, { id: "steal", hint: "Ctrl+K" }] };
equal(press(KEY.TAB, 0, section), { op: "activate", arg: 0 }, "a primary action bound off Enter (Restart, Tab)");
equal(press(0x43, Chords.CTRL | Chords.SHIFT, section), { op: "secondary", arg: 0 }, "a secondary bound off Ctrl+Enter");
equal(press(KEY.N, Chords.CTRL, section), { op: "act", arg: "new" }, "an action's chord runs it");
equal(press(KEY.K, Chords.CTRL, section), { op: "panel", arg: 0 }, "a section cannot take a frame chord");
equal(press(KEY.RETURN, 0, section), null, "Enter is not the action when the section names another key for it");
equal(press(KEY.RETURN, Chords.CTRL, section), null, "nor is Ctrl+Enter the second");
const pane = { field: true, primaryHint: "Ctrl+Enter", secondaryHint: "Ctrl+Shift+C" };
equal(press(KEY.RETURN, Chords.CTRL, pane), { op: "activate", arg: 0 }, "a tool that runs on Ctrl+Enter runs, it does not copy");
equal(press(KEY.RETURN, 0, pane), null, "and leaves Enter alone");
equal(press(KEY.N, Chords.CTRL, { field: true, all: true, primaryHint: "Enter", actions: [] }), null, "all of Kit has no section chords: the frame hands it the search's own actions");
equal(press(KEY.N, Chords.CTRL, { ...section, textEntry: true, field: false }), null, "and while a form is typed in");
equal(press(KEY.TAB, 0, field), null, "Tab with no action bound is left alone");

// Esc: panel, section layer, clear the field, close
equal(K.escape({ panel: true, all: true, fieldText: "x" }), ["panel"], "the panel goes first");
equal(K.escape({ all: true, fieldText: "x" }), ["all"], "then all of Kit, back to the section");
equal(K.escape({ fieldText: "x" }), ["section", "clear", "close"], "then the section's layer, then the field, then Kit");
equal(K.escape({ fieldText: "" }), ["section", "close"], "an empty field has nothing to clear");
equal(K.escape({}), ["section", "close"], "nor has no field at all");

// the table itself
const chords = K.GLOBAL.map(entry => entry.chord);
equal(new Set(chords).size, chords.length, "no frame chord is bound twice");
equal(K.GLOBAL.every(entry => entry.sequences.length > 0 && entry.sequences.every(s => /^Ctrl\+/.test(s))), true, "every frame chord has a Qt sequence under Ctrl");
equal(K.GLOBAL.every(entry => K.normalize(entry.chord) === entry.chord), true, "frame chords are written in the normal order");
equal(K.normalize("shift + ctrl + c"), "Ctrl+Shift+C", "hints are normalised before they are compared");
