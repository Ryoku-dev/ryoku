// Keyboard intent for the launcher sheet: a key plus the sheet's state in, one
// intent out, so the whole keymap reads (and is tested) in one place. Key and
// modifier values are Qt's (Qt.Key_*, Qt.*Modifier).

var KEY = {
    Escape: 0x01000000, Tab: 0x01000001, Backtab: 0x01000002,
    Return: 0x01000004, Enter: 0x01000005, Delete: 0x01000007,
    Home: 0x01000010, End: 0x01000011,
    Left: 0x01000012, Up: 0x01000013, Right: 0x01000014, Down: 0x01000015,
    PageUp: 0x01000016, PageDown: 0x01000017,
    J: 0x4a, K: 0x4b, N: 0x4e, P: 0x50, Digit1: 0x31, Digit9: 0x39
};
var MOD = { Shift: 0x02000000, Control: 0x04000000, Alt: 0x08000000, Meta: 0x10000000 };
var PAGE = 5;

// state: { actionsFocused, ghost, caretAtEnd, queryEmpty, canKill, modeCount }
// Returns { kind, delta, n } or null when the field should handle the key.
function intentFor(key, modifiers, state) {
    var s = state || {};
    var mods = Number(modifiers) & (MOD.Shift | MOD.Control | MOD.Alt | MOD.Meta);
    var ctrl = mods === MOD.Control;
    var alt = mods === MOD.Alt;
    var bare = mods === 0;
    var shift = mods === MOD.Shift;
    var inLead = s.actionsFocused === true;

    function move(delta) {
        return inLead ? { kind: "moveAction", delta: delta } : { kind: "move", delta: delta };
    }

    if (key === KEY.Escape && (bare || shift)) {
        if (s.actionsFocused)
            return { kind: "leaveActions" };
        if (!s.queryEmpty)
            return { kind: "clear" };
        return { kind: "close" };
    }
    if ((key === KEY.Return || key === KEY.Enter) && (bare || shift))
        return { kind: inLead ? "runAction" : "run" };
    // Shift+Delete never edits text, so it can stop the selected window's program
    if (key === KEY.Delete && shift && s.canKill)
        return { kind: "kill" };
    if (key === KEY.Backtab || (key === KEY.Tab && shift))
        return { kind: s.actionsFocused ? "leaveActions" : "swallow" };
    if (key === KEY.Tab && bare)
        return s.actionsFocused ? { kind: "moveAction", delta: 1 } : { kind: "enterActions" };
    if (bare && key === KEY.Up)
        return move(-1);
    if (bare && key === KEY.Down)
        return move(1);
    if (ctrl && (key === KEY.K || key === KEY.P))
        return move(-1);
    if (ctrl && (key === KEY.J || key === KEY.N))
        return move(1);
    if (bare && key === KEY.PageUp)
        return inLead ? move(-1) : { kind: "move", delta: -PAGE };
    if (bare && key === KEY.PageDown)
        return inLead ? move(1) : { kind: "move", delta: PAGE };
    if (bare && (key === KEY.Right || key === KEY.End) && s.ghost && s.caretAtEnd)
        return { kind: "acceptGhost" };
    if (key >= KEY.Digit1 && key <= KEY.Digit9) {
        var n = key - KEY.Digit1 + 1;
        if (alt)
            return { kind: "runIndex", n: n };
        if (ctrl && n <= (s.modeCount || 0))
            return { kind: "mode", n: n };
    }
    return null;
}

if (typeof module !== "undefined" && module.exports) {
    module.exports = { KEY: KEY, MOD: MOD, PAGE: PAGE, intentFor: intentFor };
}
