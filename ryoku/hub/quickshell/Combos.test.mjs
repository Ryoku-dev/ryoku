import fs from "node:fs";
import vm from "node:vm";
import assert from "node:assert/strict";
import test from "node:test";

// Load Combos.js the way the QML engine sees it, but with a stubbed Qt so the
// key-name mapping can be exercised in plain Node. The constants are the real
// Qt.Key_* / Qt.*Modifier values, so a numpad KeyEvent here is byte-for-byte the
// one Quickshell hands the recorder.
const Qt = {
    KeypadModifier: 0x20000000,
    MetaModifier: 0x10000000,
    ControlModifier: 0x04000000,
    AltModifier: 0x08000000,
    ShiftModifier: 0x02000000,

    Key_A: 65, Key_Z: 90, Key_Q: 81,
    Key_0: 48, Key_3: 51, Key_9: 57,
    Key_F1: 16777264, Key_F12: 16777275,

    Key_Return: 16777220, Key_Enter: 16777221,
    Key_Insert: 16777222, Key_Delete: 16777223,
    Key_Clear: 16777227,
    Key_Home: 16777232, Key_End: 16777233,
    Key_Left: 16777234, Key_Up: 16777235, Key_Right: 16777236, Key_Down: 16777237,
    Key_PageUp: 16777238, Key_PageDown: 16777239,
    // punctuation / space referenced by the non-keypad switch
    Key_Space: 0x20, Key_Tab: 0x01000001,
    Key_Comma: 0x2c, Key_Period: 0x2e, Key_Plus: 0x2b, Key_Minus: 0x2d,
    Key_Asterisk: 0x2a, Key_Slash: 0x2f, Key_Print: 0x01000009,
    Key_Backspace: 0x01000003, Key_Equal: 0x3d, Key_Backslash: 0x5c,
    Key_Semicolon: 0x3b, Key_Apostrophe: 0x27,
    Key_BracketLeft: 0x5b, Key_BracketRight: 0x5d, Key_QuoteLeft: 0x60,
};

const source = fs
    .readFileSync(new URL("./Combos.js", import.meta.url), "utf8")
    .replace(/^\.pragma library\s*/, "");
const context = { Qt };
vm.createContext(context);
vm.runInContext(source, context);
const {
    qtKeyName, chordFrom, isFamilyChord, familyFrom, expandFamily, shippedKeys, normKeys,
    rowConflict, effectiveCounts, conflictKeys, chordMatches, isChordQuery,
} = context;

// A KeyEvent as Quickshell delivers it: key code + a modifier bitmask.
const ev = (key, ...mods) => ({ key, modifiers: mods.reduce((a, m) => a | m, 0) });

test("number-pad digits with NumLock on map to KP_<d>", () => {
    assert.equal(qtKeyName(ev(Qt.Key_3, Qt.KeypadModifier)), "KP_3");
    assert.equal(qtKeyName(ev(Qt.Key_0, Qt.KeypadModifier)), "KP_0");
    assert.equal(qtKeyName(ev(Qt.Key_9, Qt.KeypadModifier)), "KP_9");
});

test("number-pad keys with NumLock off fold to the same digit keysym", () => {
    // the verified case: numpad 1 arrives as Key_End with KeypadModifier, and
    // records as KP_1 so the chord reads one way whatever way NumLock sits
    assert.equal(qtKeyName(ev(Qt.Key_End, Qt.KeypadModifier)), "KP_1");
    assert.equal(qtKeyName(ev(Qt.Key_Down, Qt.KeypadModifier)), "KP_2");
    assert.equal(qtKeyName(ev(Qt.Key_PageDown, Qt.KeypadModifier)), "KP_3");
    assert.equal(qtKeyName(ev(Qt.Key_Left, Qt.KeypadModifier)), "KP_4");
    assert.equal(qtKeyName(ev(Qt.Key_Clear, Qt.KeypadModifier)), "KP_5");
    assert.equal(qtKeyName(ev(Qt.Key_Right, Qt.KeypadModifier)), "KP_6");
    assert.equal(qtKeyName(ev(Qt.Key_Home, Qt.KeypadModifier)), "KP_7");
    assert.equal(qtKeyName(ev(Qt.Key_Up, Qt.KeypadModifier)), "KP_8");
    assert.equal(qtKeyName(ev(Qt.Key_PageUp, Qt.KeypadModifier)), "KP_9");
    assert.equal(qtKeyName(ev(Qt.Key_Insert, Qt.KeypadModifier)), "KP_0");
});

test("number-pad Enter, operators and decimal", () => {
    assert.equal(qtKeyName(ev(Qt.Key_Enter, Qt.KeypadModifier)), "KP_Enter");
    assert.equal(qtKeyName(ev(Qt.Key_Return, Qt.KeypadModifier)), "KP_Enter");
    assert.equal(qtKeyName(ev(Qt.Key_Plus, Qt.KeypadModifier)), "KP_Add");
    assert.equal(qtKeyName(ev(Qt.Key_Minus, Qt.KeypadModifier)), "KP_Subtract");
    assert.equal(qtKeyName(ev(Qt.Key_Asterisk, Qt.KeypadModifier)), "KP_Multiply");
    assert.equal(qtKeyName(ev(Qt.Key_Slash, Qt.KeypadModifier)), "KP_Divide");
    // numpad "." reads Delete with NumLock off, Period with it on; both decimal
    assert.equal(qtKeyName(ev(Qt.Key_Delete, Qt.KeypadModifier)), "KP_Decimal");
    assert.equal(qtKeyName(ev(Qt.Key_Period, Qt.KeypadModifier)), "KP_Decimal");
    assert.equal(qtKeyName(ev(Qt.Key_Comma, Qt.KeypadModifier)), "KP_Decimal");
});

test("the same navigation keys off the number pad keep their main-block names", () => {
    assert.equal(qtKeyName(ev(Qt.Key_End)), "End");
    assert.equal(qtKeyName(ev(Qt.Key_PageDown)), "Next");
    assert.equal(qtKeyName(ev(Qt.Key_Insert)), "Insert");
    assert.equal(qtKeyName(ev(Qt.Key_3)), "3");
});

test("chordFrom keeps the KeypadModifier out of the combo", () => {
    // numpad 1 with Super held, NumLock off -> the canonical KP_1 the shipped
    // workspace.focus.numpad family carries, not the raw KP_End keysym
    assert.equal(chordFrom(ev(Qt.Key_End, Qt.MetaModifier, Qt.KeypadModifier)), "SUPER + KP_1");
    // numpad 3 with Super held, NumLock on
    assert.equal(chordFrom(ev(Qt.Key_3, Qt.MetaModifier, Qt.KeypadModifier)), "SUPER + KP_3");
});

test("chordFrom orders modifiers SUPER, CTRL, ALT, SHIFT before the key", () => {
    assert.equal(chordFrom(ev(Qt.Key_Q, Qt.MetaModifier)), "SUPER + Q");
    assert.equal(
        chordFrom(ev(Qt.Key_Q, Qt.ShiftModifier, Qt.AltModifier, Qt.ControlModifier, Qt.MetaModifier)),
        "SUPER + CTRL + ALT + SHIFT + Q",
    );
});

test("a lone modifier press yields no chord yet", () => {
    // Meta alone: no main key, so nothing to commit (the overlay keeps waiting)
    assert.equal(chordFrom({ key: 0, modifiers: Qt.MetaModifier }), "");
});

test("isFamilyChord spots the {n} placeholder, digit or numpad", () => {
    assert.equal(isFamilyChord("SUPER + {n}"), true);
    assert.equal(isFamilyChord("SUPER + KP_{n}"), true);
    assert.equal(isFamilyChord("SUPER + CTRL + {n}"), true);
    assert.equal(isFamilyChord("SUPER + Q"), false);
    assert.equal(isFamilyChord("SUPER + 3"), false);
    assert.equal(isFamilyChord(""), false);
});

test("familyFrom keeps the modifiers and swaps a digit key for {n}", () => {
    // a recorded chord whose key is a plain digit becomes the digit family
    assert.equal(familyFrom("SUPER + 3"), "SUPER + {n}");
    assert.equal(familyFrom("SUPER + CTRL + 0"), "SUPER + CTRL + {n}");
    // a number-pad digit becomes the numpad family, so the twin still binds
    assert.equal(familyFrom("SUPER + KP_3"), "SUPER + KP_{n}");
    assert.equal(familyFrom("SUPER + ALT + KP_0"), "SUPER + ALT + KP_{n}");
});

test("familyFrom rejects a non-number key so the recorder waits", () => {
    assert.equal(familyFrom("SUPER + Q"), "");
    assert.equal(familyFrom("SUPER + Return"), "");
    assert.equal(familyFrom("SUPER + KP_Enter"), "");
    assert.equal(familyFrom(""), "");
});

test("chordFrom then familyFrom is the record-to-store path", () => {
    // Super + numpad 3, NumLock on: the recorder yields SUPER + KP_3, which the
    // page folds to the numpad family it stores under the row's default
    assert.equal(familyFrom(chordFrom(ev(Qt.Key_3, Qt.MetaModifier, Qt.KeypadModifier))), "SUPER + KP_{n}");
    // Super + a plain 3 on the number row folds to the digit family
    assert.equal(familyFrom(chordFrom(ev(Qt.Key_3, Qt.MetaModifier))), "SUPER + {n}");
});

test("expandFamily runs {n} onto 1..9,0 and KP_{n} onto the number pad", () => {
    // expandFamily runs inside the vm realm, so its array joins to a string here
    // rather than deep-comparing across realms (a different Array prototype).
    assert.equal(expandFamily("SUPER + {n}").join(","),
        "SUPER + 1,SUPER + 2,SUPER + 3,SUPER + 4,SUPER + 5,SUPER + 6,SUPER + 7,SUPER + 8,SUPER + 9,SUPER + 0");
    const kp = expandFamily("SUPER + KP_{n}");
    assert.equal(kp.length, 10);
    assert.equal(kp[0], "SUPER + KP_1");
    assert.equal(kp[2], "SUPER + KP_3");
    assert.equal(kp[9], "SUPER + KP_0");
    // a plain chord expands to itself, so a caller can expand any chord
    assert.equal(expandFamily("SUPER + Q").join(","), "SUPER + Q");
    assert.equal(expandFamily("").length, 0);
});

test("shippedKeys expands a family so a digit custom bind is caught shadowing it", () => {
    const cats = [{ name: "Workspaces", binds: [{ combo: "SUPER + {n}" }] }];
    const keys = shippedKeys(cats, {});
    // every one of the ten workspace chords is in the shadow set
    assert.ok(keys[normKeys("SUPER + 3")], "SUPER + 3 shadows the family");
    assert.ok(keys[normKeys("SUPER + 0")], "the tenth (0) shadows the family");
    assert.ok(!keys["{n}+super"], "the raw {n} placeholder is never a shadow key");
});

test("the legend's custom rows are not shipped, so they do not shadow themselves", () => {
    const cats = [
        { name: "Windows", binds: [{ combo: "SUPER + Q", kind: "wm" }] },
        { name: "Custom", binds: [{ combo: "SUPER + J", kind: "custom" }] },
    ];
    const keys = shippedKeys(cats, {});
    assert.ok(keys[normKeys("SUPER + Q")]);
    assert.ok(!keys[normKeys("SUPER + J")], "a custom chord is not a shipped chord");
    assert.equal(rowConflict([{ keys: "SUPER + J" }], 0, keys), "");
    assert.equal(rowConflict([{ keys: "SUPER + Q" }], 0, keys), "shipped");
});

test("a switched-off shipped bind does not shadow a custom row", () => {
    const cats = [{ name: "Windows", binds: [{ combo: "SUPER + Q", kind: "wm" }] }];
    assert.ok(!shippedKeys(cats, {}, ["SUPER + Q"])[normKeys("SUPER + Q")]);
    assert.equal(rowConflict([{ keys: "SUPER + Q" }], 0, shippedKeys(cats, {}, ["SUPER + Q"])), "");
    const moved = shippedKeys(cats, { "SUPER + Q": "SUPER + W" }, []);
    assert.ok(moved[normKeys("SUPER + W")], "a rebind moves the shipped chord");
    assert.ok(!moved[normKeys("SUPER + Q")]);
});

test("effectiveCounts counts a custom row once and sees real clashes only", () => {
    const cats = [
        { name: "Windows", binds: [{ combo: "SUPER + Q", kind: "wm" }] },
        { name: "Custom", binds: [{ combo: "SUPER + J", kind: "custom" }] },
    ];
    const custom = [{ keys: "SUPER + J" }];
    const counts = effectiveCounts(cats, {}, custom, []);
    assert.equal(counts[normKeys("SUPER + J")], 1, "the legend copy is skipped");
    assert.ok(!conflictKeys(cats, {}, custom, [])[normKeys("SUPER + J")]);

    const shadow = conflictKeys(cats, {}, [{ keys: "SUPER + Q" }], []);
    assert.ok(shadow[normKeys("SUPER + Q")], "a custom shadow is a clash");
});

test("conflictKeys sees rebinds, families and switched-off binds", () => {
    const cats = [{
        name: "Windows",
        binds: [{ combo: "SUPER + Q", kind: "wm" }, { combo: "SUPER + W", kind: "wm" }],
    }];
    assert.ok(conflictKeys(cats, { "SUPER + Q": "SUPER + W" }, [], [])[normKeys("SUPER + W")]);
    assert.ok(!conflictKeys(cats, { "SUPER + Q": "SUPER + W" }, [], ["SUPER + W"])[normKeys("SUPER + W")]);

    const fam = [{ name: "Workspaces", binds: [{ combo: "SUPER + {n}", kind: "wm" }] }];
    assert.ok(conflictKeys(fam, {}, [{ keys: "SUPER + 3" }], [])[normKeys("SUPER + 3")]);
});

test("chord search folds case, spacing, order and modifier aliases", () => {
    assert.equal(chordMatches("ctrl+l", "CTRL + L"), true);
    assert.equal(chordMatches("CTRL+L", "CTRL + L"), true);
    assert.equal(chordMatches("CONTROL + l", "CTRL + L"), true);
    assert.equal(chordMatches("l + control", "CTRL + L"), true);
    assert.equal(chordMatches("ctrl", "SUPER + CTRL + L"), true);
    assert.equal(chordMatches("ctrl+l", "SUPER + L"), false, "a different modifier is a different chord");
    assert.equal(chordMatches("super+l", "SUPER + SHIFT + L"), false, "a named key must match the whole chord");
    assert.equal(chordMatches("ctrl+shift+l", "CTRL + L"), false, "a named key must match the whole chord");
    assert.equal(chordMatches("super+enter", "SUPER + Return"), true);
    assert.equal(chordMatches("super+page up", "SUPER + Prior"), true);
    assert.equal(chordMatches("super+num 3", "SUPER + KP_3"), true);
    assert.equal(chordMatches("super+,", "SUPER + comma"), true);
});

test("isChordQuery leaves ordinary text to the fuzzy scorer", () => {
    assert.equal(isChordQuery("ctrl+l"), true);
    assert.equal(isChordQuery("super"), true);
    assert.equal(isChordQuery("ctrl l"), true);
    assert.equal(isChordQuery("close window"), false);
    assert.equal(isChordQuery(""), false);
});

test("a word that doubles as a modifier alias only reads as one in a chord", () => {
    // "windows" is a category someone searches for, "windows+l" is a chord.
    assert.equal(isChordQuery("windows"), false);
    assert.equal(isChordQuery("win"), false);
    assert.equal(isChordQuery("option"), false);
    assert.equal(isChordQuery("windows+l"), true);
    assert.equal(isChordQuery("win l"), true);
});
