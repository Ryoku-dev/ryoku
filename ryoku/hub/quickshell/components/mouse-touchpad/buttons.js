.pragma library

// mouse button names and bindings in words; tr is I18n.tr

function buttonLabel(code, device, tr) {
    if (device && device.labels && device.labels[String(code)])
        return tr(String(device.labels[String(code)]));
    if (code >= 2 && code <= 13)
        return tr("Side %1").replace("%1", code === 11 ? 10 : code === 12 ? 11 : code === 13 ? 12 : code - 1);
    if (code >= 656 && code <= 685)
        return "G" + (code - 655);
    if (code >= 183 && code <= 194)
        return "F" + (code - 170);
    if (code === 148 || code === 149)
        return "G" + (code - 147);
    if (code === 202 || code === 203)
        return "G" + (code - 199);
    if (code === 275)
        return tr("Side button");
    if (code === 276)
        return tr("Extra button");
    if (code === 277)
        return tr("Thumb button");
    if (code === 278)
        return tr("Second thumb button");
    if (code >= 704 && code <= 743)
        return tr("Macro key %1").replace("%1", code - 703);
    return tr("Button %1").replace("%1", code);
}

function buttonDefault(code, tr) {
    if (code === 275)
        return tr("Back");
    if (code === 276)
        return tr("Forward");
    return "";
}

function keysText(keys) {
    return (keys || []).map(k => String(k).length === 1 ? String(k).toUpperCase() : String(k).charAt(0).toUpperCase() + String(k).slice(1)).join("+");
}

function bindingText(target, code, tr) {
    if (!target) {
        var d = buttonDefault(code, tr);
        return d ? tr("Does what it always does: %1").replace("%1", d) : tr("Does what it always does");
    }
    if (target.kind === "disabled")
        return tr("Off");
    if (target.kind === "button")
        return tr("Clicks %1").replace("%1", buttonLabel(target.button, null, tr));
    if (target.kind === "sequence") {
        var n = target.sequence ? target.sequence.length : 0;
        var line = n === 1 ? tr("Runs 1 step") : tr("Runs %1 steps").replace("%1", n);
        if ((target.repeat || 1) > 1)
            line += ", " + tr("%1 times").replace("%1", target.repeat);
        return line;
    }
    return tr("Sends %1").replace("%1", keysText(target.keys));
}

function stepText(step, tr) {
    if (step.kind === "delay")
        return tr("Wait %1 ms").replace("%1", step.delayMs);
    var keys = keysText(step.keys);
    if (step.kind === "down")
        return tr("Hold %1 down").replace("%1", keys);
    if (step.kind === "up")
        return tr("Let go of %1").replace("%1", keys);
    return tr("Press %1").replace("%1", keys);
}

// a recorded key (Combos token) as the remapper names it, or "" when a mouse
// button cannot send it
function keyToken(t) {
    var m = { "SUPER": "super", "CTRL": "ctrl", "ALT": "alt", "SHIFT": "shift",
              "Return": "enter", "Space": "space", "Tab": "tab", "BackSpace": "backspace", "Escape": "esc",
              "Prior": "pgup", "Next": "pgdn", "Left": "left", "Right": "right",
              "Up": "up", "Down": "down", "Home": "home", "End": "end",
              "Insert": "insert", "Delete": "delete", "Print": "print" };
    if (m[t] !== undefined)
        return m[t];
    if (/^[A-Z]$/.test(t) || /^[0-9]$/.test(t))
        return t.toLowerCase();
    if (/^F([1-9]|1[0-2])$/.test(t))
        return t.toLowerCase();
    if (["minus", "equal", "bracketleft", "bracketright", "comma", "period",
         "slash", "semicolon", "apostrophe", "backslash", "grave"].indexOf(t) >= 0)
        return t;
    return "";
}

// "CTRL + C" -> ["ctrl", "c"], or null when a key is outside the set
function chordToKeys(chord) {
    var parts = String(chord).split("+");
    var out = [];
    for (var i = 0; i < parts.length; i++) {
        var k = keyToken(parts[i].trim());
        if (k === "")
            return null;
        out.push(k);
    }
    return out.length > 0 ? out : null;
}

// the extra buttons of one mouse, side grid first
function buttonsOf(device) {
    var side = device.sideButtons || [];
    var extra = (device.buttons || []).filter(c => c >= 275 && side.indexOf(c) < 0);
    return side.concat(extra);
}

function deviceName(device) {
    if (device.brand && device.model)
        return device.brand + " " + device.model;
    return device.model || device.name || device.id;
}

// one step of a sequence with sane values
function cleanStep(step) {
    if (step.kind === "delay")
        return { kind: "delay", delayMs: Math.max(10, Math.min(60000, Math.round(Number(step.delayMs) || 100))) };
    return { kind: ["tap", "down", "up"].indexOf(step.kind) >= 0 ? step.kind : "tap", keys: (step.keys || []).slice() };
}
