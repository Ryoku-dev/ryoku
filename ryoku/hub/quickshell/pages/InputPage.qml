pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Quickshell
import Quickshell.Io
import Quickshell.Wayland
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../Singletons"
import "../Combos.js" as Combos

// Input (DESIGN.md, SYSTEM). Keyboard layout and remaps, pointer and touchpad
// behaviour, and key repeat for the Hyprland session. Rendered as rows off the
// shared taxonomy, but three surfaces refuse a plain schema and are built by
// hand:
//
//   * kb_layout / kb_variant are ONE positional comma string each ("fr,us" +
//     "azerty,"): two catalogue picks edit the halves and keep the variant slot
//     aligned to its layout by position.
//   * kb_options is ONE comma string multiplexed across four controls plus a
//     free-text escape hatch. Caps Lock, Swap Alt/Super, Compose and Switch
//     layouts each own a family of xkb tokens; the family is replaced in place,
//     everything unrecognised round-trips through Extra options so power users
//     lose nothing. grpIds carries grp:caps_toggle deliberately (so it is
//     stripped from Extra options) while the Switch seg never offers it: that
//     asymmetry is load-bearing and reproduced here on purpose.
//   * Apply system-wide is an out-of-band, privileged `ryoku keyboard apply`
//     covering the login screen, the TTYs and the disk passphrase prompt; it
//     has no draft, no revert, no dirty state, and is gated on the keyboard
//     keys already being saved.
//
// The MOUSE tab is not schema: it drives the shell daemon's evdev remapper
// (ryoku/shell/ipc/mousemap.go), which is compositor-neutral by construction.
// It pairs a mouse's pointer and MMO side-grid interfaces, replays a grabbed
// interface through a uinput clone, and substitutes chords, buttons or timed
// sequences. State lives in mousemap.json, not the desktop store, and the page
// talks to it over the shell socket exactly like the Recording page talks to
// the keypress overlay. A machine with no mouse (a laptop's built-in pad is
// classified out) shows the empty plate and binds nothing.
//
// The layout/variant catalogues are scanned at runtime (xkb rules), so they are
// filtered picks, not enums. Everything else reads hub.hyprVal / writes
// hub.hyprEdit; the shell owns the rail, side panel, action bar, live preview
// and restore. Nothing here writes a file except Apply system-wide and the
// daemon's own settings.
Item {
    id: pg

    property var hub

    // gated so nothing paints stale before the first `hypr get` returns.
    readonly property bool ready: pg.hub ? pg.hub.wmLoaded === true : false

    // Installed cursor themes, enumerated by the backend so the row offers what
    // this machine can actually wear (the Cursor page read the same command).
    property var cursorThemeList: [{ "code": "DYNAMIC", "name": I18n.tr("Follow the wallpaper") }]
    Process {
        id: cursorEnum
        running: false
        command: ["ryoku-hub", "desktop", "cursors"]
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var names = JSON.parse(this.text);
                    if (!Array.isArray(names)) return;
                    var out = [{ "code": "DYNAMIC", "name": I18n.tr("Follow the wallpaper") }];
                    for (var i = 0; i < names.length; i++)
                        out.push({ "code": String(names[i]), "name": String(names[i]) });
                    pg.cursorThemeList = out;
                } catch (e) {}
            }
        }
    }

    // The touchpad lock is a live device state the seam owns, not a saved
    // setting, so it is read once through the seam (on|off) rather than off the
    // settings draft, and re-read after a flip so the switch shows what the
    // compositor actually did. Only wired where the compositor can toggle a pad.
    property bool touchpadOn: true
    Process {
        id: touchpadStatus
        running: false
        command: ["ryoku", "wm", "act", "input.touchpad", "status"]
        stdout: StdioCollector {
            onStreamFinished: pg.touchpadOn = String(this.text).trim() !== "off"
        }
    }
    Process {
        id: touchpadSet
        running: false
        onExited: touchpadStatus.running = true
    }
    function setTouchpad(on) {
        touchpadSet.command = ["ryoku", "wm", "act", "input.touchpad", on ? "on" : "off"];
        touchpadSet.running = false;
        touchpadSet.running = true;
    }

    // ── tabs ────────────────────────────────────────────────────────────────
    readonly property var tabs: ["KEYBOARD", "MOUSE"]
    property string tab: "KEYBOARD"
    function switchTab(t) {
        if (pg.tab === t)
            return;
        pg.tab = t;
        if (t === "MOUSE")
            mouseCtl.send("mouse.state", {});
        tabHost.travel();
    }

    // ── mouse tab: the shell daemon's evdev remapper ─────────────────────
    // One subscription for frames (devices, maps, captured presses), one
    // control line per call; the daemon owns mousemap.json and the grabs, so
    // the page never touches a file. Same socket grammar the Recording page
    // uses for the keypress overlay.
    readonly property string shellSockPath: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-shell.sock"
    property var mouseDevices: []
    property var mouseMaps: ({})
    property bool mouseDaemonSeen: false
    property string mouseError: ""
    property var captureFor: null
    property bool captureWaitClick: false
    property string captureForming: ""
    property var macroEditor: null
    property bool macroKeyCapture: false
    property bool macroRecording: false
    property string macroAction: "tap"
    property string macroModifierOnly: ""
    property double macroLastAt: 0
    readonly property bool capturing: pg.captureFor !== null || pg.macroKeyCapture || pg.macroRecording

    function applyMouseFrame(line) {
        try {
            var frame = JSON.parse(line);
            if (!frame || !frame.devices)
                return;
            pg.mouseDaemonSeen = true;
            pg.mouseDevices = frame.devices;
            pg.mouseMaps = frame.maps || {};
            var p = frame.pressed;
            if (p && pg.capturing && pg.captureWaitClick) {
                if (p.device === "" && p.button === 0) {
                    // the daemon's waiter expired; re-arm so the overlay keeps
                    // listening until the user presses something or cancels.
                    mouseCtl.send("mouse.capture", { timeoutMs: 8000 });
                    return;
                }
                pg.captureFor = { "device": p.device, "button": p.button };
                pg.captureWaitClick = false;
            }
        } catch (e) {}
    }

    function applyMouseReply(line) {
        try {
            var reply = JSON.parse(line);
            if (reply.ok) {
                pg.mouseError = "";
                if (reply.result && reply.result.devices)
                    pg.applyMouseFrame(JSON.stringify(reply.result));
                return;
            }
            pg.mouseError = reply.error || I18n.tr("The mouse remapper refused the change.");
            if (reply.result && reply.result.devices)
                pg.applyMouseFrame(JSON.stringify(reply.result));
        } catch (e) {}
    }

    Socket {
        id: mouseSub
        path: pg.shellSockPath
        parser: SplitParser { onRead: line => pg.applyMouseFrame(line) }
        Component.onCompleted: connected = true
        onConnectionStateChanged: {
            if (connected) {
                write("subscribe mouse\n");
                flush();
            } else {
                mouseSubRetry.restart();
            }
        }
    }
    Timer {
        id: mouseSubRetry
        interval: 2000
        onTriggered: if (!mouseSub.connected) mouseSub.connected = true
    }
    Socket {
        id: mouseCtl
        path: pg.shellSockPath
        property string queued: ""
        parser: SplitParser { onRead: line => pg.applyMouseReply(line) }
        function send(method, args) {
            queued += "call " + method + " " + JSON.stringify(args) + "\n";
            if (connected)
                flushQueued();
            else
                connected = true;
        }
        function flushQueued() {
            if (queued.length === 0)
                return;
            write(queued);
            flush();
            queued = "";
        }
        onConnectionStateChanged: if (connected) flushQueued()
    }

    function mouseDevice(device) {
        for (var i = 0; i < pg.mouseDevices.length; i++)
            if (pg.mouseDevices[i].id === device)
                return pg.mouseDevices[i];
        return null;
    }
    function mouseDisplayName(device) {
        var dev = pg.mouseDevice(device);
        if (!dev) return device;
        if (dev.brand && dev.model) return dev.brand + " · " + dev.model;
        return dev.name || device;
    }
    function buttonLabel(code, device) {
        var dev = device ? pg.mouseDevice(device) : null;
        if (dev && dev.labels && dev.labels[String(code)])
            return I18n.tr(String(dev.labels[String(code)]));
        if (code >= 2 && code <= 13)
            return I18n.tr("Side") + " " + (code === 11 ? 10 : code === 12 ? 11 : code === 13 ? 12 : code - 1);
        if (code >= 656 && code <= 685)
            return "G" + (code - 655);
        if (code >= 183 && code <= 194)
            return "F" + (code - 170);
        if (code === 148 || code === 149)
            return "G" + (code - 147);
        if (code === 202 || code === 203)
            return "G" + (code - 199);
        if (code === 275) return I18n.tr("Side");
        if (code === 276) return I18n.tr("Extra");
        if (code === 277) return I18n.tr("Thumb");
        if (code === 278) return I18n.tr("Thumb 2");
        if (code >= 704 && code <= 743) return I18n.tr("Macro") + " " + (code - 703);
        return I18n.tr("Button") + " " + code;
    }
    function buttonDefault(code) {
        if (code === 275) return I18n.tr("Back");
        if (code === 276) return I18n.tr("Forward");
        return "";
    }
    function mapFor(device, code) {
        var m = pg.mouseMaps[device];
        return m ? m[String(code)] : undefined;
    }
    function bindingText(t) {
        if (!t) return "";
        if (t.kind === "disabled") return I18n.tr("Off");
        if (t.kind === "button") return pg.buttonLabel(t.button) + " " + I18n.tr("click");
        if (t.kind === "sequence")
            return (t.sequence ? t.sequence.length : 0) + " " + I18n.tr("steps")
                + ((t.repeat || 1) > 1 ? " ×" + t.repeat : "");
        var out = [];
        for (var i = 0; i < (t.keys || []).length; i++)
            out.push(String(t.keys[i]).toUpperCase());
        return out.join(" + ");
    }
    function targetText(device, code) {
        var t = pg.mapFor(device, code);
        if (!t) {
            var d = pg.buttonDefault(code);
            return d ? I18n.tr("Default") + " (" + d + ")" : I18n.tr("Default");
        }
        return pg.bindingText(t);
    }

    // KeyEvent tokens (what Combos.chordFrom builds) -> the daemon's key names.
    // Anything outside the shared table is refused, not silently dropped.
    function keyToken(t) {
        var m = { "SUPER": "super", "CTRL": "ctrl", "ALT": "alt", "SHIFT": "shift",
            "Return": "enter", "Space": "space", "Tab": "tab", "BackSpace": "backspace",
            "Prior": "pgup", "Next": "pgdn", "Left": "left", "Right": "right",
            "Up": "up", "Down": "down", "Home": "home", "End": "end",
            "Insert": "insert", "Delete": "delete", "Print": "print" };
        if (m[t] !== undefined) return m[t];
        if (/^[A-Z]$/.test(t) || /^[0-9]$/.test(t)) return t.toLowerCase();
        if (/^F([1-9]|1[0-2])$/.test(t)) return t.toLowerCase();
        if (["minus", "equal", "bracketleft", "bracketright", "comma", "period",
             "slash", "semicolon", "apostrophe", "backslash", "grave"].indexOf(t) >= 0)
            return t;
        return "";
    }
    function modifierKeyToken(key) {
        if (key === Qt.Key_Shift) return "SHIFT";
        if (key === Qt.Key_Control) return "CTRL";
        if (key === Qt.Key_Alt) return "ALT";
        if (key === Qt.Key_Meta) return "SUPER";
        return "";
    }
    function chordToKeys(chord) {
        var parts = String(chord).split("+");
        var out = [];
        for (var i = 0; i < parts.length; i++) {
            var k = pg.keyToken(parts[i].trim());
            if (k === "")
                return null;
            out.push(k);
        }
        return out;
    }

    function startBind(device, code) {
        pg.captureFor = { "device": device, "button": code };
        pg.captureWaitClick = false;
        pg.captureForming = "";
        pg.enterRecordSubmap();
        recordTimeout.restart();
    }
    function startDetect() {
        pg.captureFor = { "device": "", "button": 0 };
        pg.captureWaitClick = true;
        pg.captureForming = "";
        mouseCtl.send("mouse.capture", { timeoutMs: 8000 });
        pg.enterRecordSubmap();
        recordTimeout.restart();
    }
    function stopBind(commit, chord) {
        recordTimeout.stop();
        pg.exitRecordSubmap();
        var c = pg.captureFor;
        pg.captureFor = null;
        pg.captureWaitClick = false;
        pg.captureForming = "";
        if (!commit || !c)
            return;
        var keys = pg.chordToKeys(chord);
        if (keys === null) {
            pg.mouseError = I18n.tr("That key is outside the bindable set.");
            return;
        }
        mouseCtl.send("mouse.map", { "device": c.device, "button": c.button,
            "target": { "kind": "chord", "keys": keys } });
    }
    function setDisabled(device, code) {
        mouseCtl.send("mouse.map", { "device": device, "button": code,
            "target": { "kind": "disabled" } });
    }
    function clearBinding(device, code) {
        mouseCtl.send("mouse.map", { "device": device, "button": code, "target": null });
    }
    function openMacro(device, code) {
        var current = pg.mapFor(device, code);
        var target = current && current.kind === "sequence" ? JSON.parse(JSON.stringify(current)) :
            { "kind": "sequence", "sequence": [], "repeat": 1, "cancelOnRelease": false };
        pg.macroEditor = { "device": device, "button": code, "target": target };
    }
    function updateMacroTarget(target) {
        var editor = JSON.parse(JSON.stringify(pg.macroEditor));
        editor.target = target;
        pg.macroEditor = editor;
    }
    function addMacroStep(step) {
        if (!pg.macroEditor) return;
        var target = JSON.parse(JSON.stringify(pg.macroEditor.target));
        target.sequence.push(step);
        pg.updateMacroTarget(target);
    }
    function removeMacroStep(index) {
        var target = JSON.parse(JSON.stringify(pg.macroEditor.target));
        target.sequence.splice(index, 1);
        pg.updateMacroTarget(target);
    }
    function moveMacroStep(index, delta) {
        var target = JSON.parse(JSON.stringify(pg.macroEditor.target));
        var other = index + delta;
        if (other < 0 || other >= target.sequence.length) return;
        var step = target.sequence[index];
        target.sequence.splice(index, 1);
        target.sequence.splice(other, 0, step);
        pg.updateMacroTarget(target);
    }
    function adjustMacroDelay(index, delta) {
        var target = JSON.parse(JSON.stringify(pg.macroEditor.target));
        target.sequence[index].delayMs = Math.max(10, Math.min(60000,
            Number(target.sequence[index].delayMs || 100) + delta));
        pg.updateMacroTarget(target);
    }
    function setMacroRepeat(delta) {
        var target = JSON.parse(JSON.stringify(pg.macroEditor.target));
        target.repeat = Math.max(1, Math.min(100, Number(target.repeat || 1) + delta));
        pg.updateMacroTarget(target);
    }
    function toggleMacroRelease() {
        var target = JSON.parse(JSON.stringify(pg.macroEditor.target));
        target.cancelOnRelease = !target.cancelOnRelease;
        pg.updateMacroTarget(target);
    }
    function startMacroKey(action) {
        pg.macroAction = action;
        pg.macroKeyCapture = true;
        pg.macroRecording = false;
        pg.captureForming = "";
        pg.macroModifierOnly = "";
        pg.enterRecordSubmap();
        recordTimeout.restart();
    }
    function startMacroRecording() {
        var target = JSON.parse(JSON.stringify(pg.macroEditor.target));
        target.sequence = [];
        pg.updateMacroTarget(target);
        pg.macroRecording = true;
        pg.macroKeyCapture = false;
        pg.captureForming = "";
        pg.macroLastAt = Date.now();
        pg.macroModifierOnly = "";
        pg.enterRecordSubmap();
        recordTimeout.restart();
    }
    function stopMacroCapture() {
        recordTimeout.stop();
        pg.exitRecordSubmap();
        pg.macroKeyCapture = false;
        pg.macroRecording = false;
        pg.captureForming = "";
        pg.macroModifierOnly = "";
    }
    function acceptMacroChord(chord) {
        pg.macroModifierOnly = "";
        var keys = pg.chordToKeys(chord);
        if (keys === null) {
            pg.mouseError = I18n.tr("That key is outside the bindable set.");
            return;
        }
        if (pg.macroRecording) {
            var now = Date.now();
            if (pg.macroEditor.target.sequence.length > 0) {
                var delay = Math.max(1, Math.min(60000, Math.round(now - pg.macroLastAt)));
                pg.addMacroStep({ "kind": "delay", "delayMs": delay });
            }
            pg.addMacroStep({ "kind": "tap", "keys": keys });
            pg.macroLastAt = now;
            recordTimeout.restart();
            return;
        }
        pg.addMacroStep({ "kind": pg.macroAction, "keys": keys });
        pg.stopMacroCapture();
    }
    function macroStepText(step) {
        if (step.kind === "delay") return I18n.tr("Wait") + " " + step.delayMs + " ms";
        var keys = [];
        for (var i = 0; i < (step.keys || []).length; i++)
            keys.push(String(step.keys[i]).toUpperCase());
        return I18n.tr(step.kind === "down" ? "Key down" : step.kind === "up" ? "Key up" : "Tap")
            + " · " + keys.join(" + ");
    }
    function saveMacro() {
        if (!pg.macroEditor || pg.macroEditor.target.sequence.length === 0) return;
        mouseCtl.send("mouse.map", { "device": pg.macroEditor.device,
            "button": pg.macroEditor.button, "target": pg.macroEditor.target });
        pg.macroEditor = null;
    }
    function testMacro() {
        if (pg.macroEditor && pg.macroEditor.target.sequence.length > 0)
            mouseCtl.send("mouse.test", { "target": pg.macroEditor.target });
    }
    function addMacroDelay() {
        pg.addMacroStep({ "kind": "delay", "delayMs": 100 });
    }
    function enterRecordSubmap() { if (pg.hub && Settings.supports("submap")) pg.hub.wmAct("submap.enter", ["record"]); }
    function exitRecordSubmap() { if (pg.hub && Settings.supports("submap")) pg.hub.wmAct("submap.reset"); }
    Timer {
        id: recordTimeout
        interval: 12000
        onTriggered: {
            if (pg.macroKeyCapture || pg.macroRecording)
                pg.stopMacroCapture();
            else
                pg.stopBind(false, "");
        }
    }

    Component.onCompleted: {
        pg.refreshVariants();
        cursorEnum.running = true;
        if (Settings.supports("touchpadToggle"))
            touchpadStatus.running = true;
    }

    // ── hub access ──────────────────────────────────────────────────────────
    function hv(path) { return pg.hub ? pg.hub.hyprVal(path) : undefined }
    function cv(path) { return pg.hub ? pg.hub.hyprCommittedVal(path) : undefined }
    function he(path, v) { if (pg.hub) pg.hub.hyprEdit(path, v) }

    // ── keyboard layout: positional comma strings ───────────────────────────
    // kb_layout may hold "fr,us" (primary + secondary). The variant applies to
    // the primary, so a secondary keeps the variant's slot empty ("azerty,")
    // because xkb aligns variants to layouts by position. `committed` reads disk
    // for the struck default.
    function kbLayoutStr(committed) {
        return String((committed ? pg.cv("desktop.input.kbLayout") : pg.hv("desktop.input.kbLayout")) || "");
    }
    function kbVariantStr(committed) {
        return String((committed ? pg.cv("desktop.input.kbVariant") : pg.hv("desktop.input.kbVariant")) || "");
    }
    function primaryLayout(committed) { return pg.kbLayoutStr(committed).split(",")[0]; }
    function secondaryLayout(committed) {
        var parts = pg.kbLayoutStr(committed).split(",");
        return parts.length > 1 ? parts[1] : "";
    }
    function primaryVariant(committed) { return pg.kbVariantStr(committed).split(",")[0]; }

    function setLayouts(primary, secondary) {
        pg.he("desktop.input.kbLayout", secondary ? primary + "," + secondary : primary);
        var v = pg.primaryVariant(false);
        pg.he("desktop.input.kbVariant", secondary && v ? v + "," : v);
    }
    function setVariant(v) {
        pg.he("desktop.input.kbVariant", pg.secondaryLayout(false) && v ? v + "," : v);
    }

    // ── curated remaps over kb_options ──────────────────────────────────────
    // Each control owns one xkb option family; anything it does not recognise
    // stays in Extra options. All state lives in the single kb_options string.
    readonly property var capsIds: ["caps:escape", "ctrl:nocaps", "caps:swapescape", "caps:none"]
    readonly property var composeIds: ["compose:ralt", "compose:menu"]
    // grp:caps_toggle is a recognised family member (so it is stripped from
    // Extra options) but is NOT offered by the Switch seg: the asymmetry is
    // deliberate, so a caps_toggle set elsewhere is not leaked into free text.
    readonly property var grpIds: ["grp:alt_shift_toggle", "grp:win_space_toggle", "grp:caps_toggle"]
    readonly property string swapId: "altwin:swap_alt_win"

    function kbOptionsStr(committed) {
        return String((committed ? pg.cv("desktop.input.kbOptions") : pg.hv("desktop.input.kbOptions")) || "");
    }
    function optTokens(committed) {
        var raw = pg.kbOptionsStr(committed).split(",");
        var out = [];
        for (var i = 0; i < raw.length; i++) {
            var t = raw[i].trim();
            if (t.length)
                out.push(t);
        }
        return out;
    }
    // the first recognised token of a family wins, matching the old page.
    function pickFrom(ids, committed) {
        var toks = pg.optTokens(committed);
        for (var i = 0; i < toks.length; i++)
            if (ids.indexOf(toks[i]) !== -1)
                return toks[i];
        return "";
    }
    function knownIds() {
        return pg.capsIds.concat(pg.composeIds).concat(pg.grpIds).concat([pg.swapId]);
    }
    function extraOptions(committed) {
        var known = pg.knownIds();
        var toks = pg.optTokens(committed);
        var out = [];
        for (var i = 0; i < toks.length; i++)
            if (known.indexOf(toks[i]) === -1)
                out.push(toks[i]);
        return out.join(",");
    }
    // rebuild kb_options with one family replaced; "" drops the family.
    function setOption(ids, value) {
        var toks = pg.optTokens(false);
        var out = [];
        for (var j = 0; j < toks.length; j++)
            if (ids.indexOf(toks[j]) === -1)
                out.push(toks[j]);
        if (value.length)
            out.push(value);
        pg.he("desktop.input.kbOptions", out.join(","));
    }
    function setExtra(text) {
        var known = pg.knownIds();
        var keep = [];
        var toks = pg.optTokens(false);
        for (var i = 0; i < toks.length; i++)
            if (known.indexOf(toks[i]) !== -1)
                keep.push(toks[i]);
        var raw = String(text || "").split(",");
        for (var j = 0; j < raw.length; j++) {
            var t = raw[j].trim();
            if (t.length)
                keep.push(t);
        }
        pg.he("desktop.input.kbOptions", keep.join(","));
    }

    // family key <-> visible label, offered choices per family.
    readonly property var capsMap: [
        { "key": "", "label": I18n.tr("Default") },
        { "key": "caps:escape", "label": I18n.tr("Escape") },
        { "key": "ctrl:nocaps", "label": "Ctrl" },
        { "key": "caps:swapescape", "label": I18n.tr("Swap Esc") },
        { "key": "caps:none", "label": I18n.tr("Off") }
    ]
    readonly property var composeMap: [
        { "key": "", "label": I18n.tr("Off") },
        { "key": "compose:ralt", "label": I18n.tr("Right Alt") },
        { "key": "compose:menu", "label": I18n.tr("Menu") }
    ]
    readonly property var grpMap: [
        { "key": "", "label": I18n.tr("Off") },
        { "key": "grp:alt_shift_toggle", "label": "Alt+Shift" },
        { "key": "grp:win_space_toggle", "label": "Super+Space" }
    ]
    function mapLabel(m, k) {
        for (var i = 0; i < m.length; i++)
            if (String(m[i].key) === String(k === undefined ? "" : k))
                return m[i].label;
        return String(k === undefined ? "" : k);
    }
    function mapKey(m, lb) {
        for (var i = 0; i < m.length; i++)
            if (m[i].label === lb)
                return m[i].key;
        return lb;
    }
    function labels(m) {
        var out = [];
        for (var i = 0; i < m.length; i++)
            out.push(m[i].label);
        return out;
    }

    // ── dynamic xkb catalogues ──────────────────────────────────────────────
    property var layoutOptions: []                                 // [{ code, name }]
    property var variantOptions: [{ "code": "", "name": I18n.tr("Default") }]

    Process {
        id: layoutsProc
        command: ["ryoku-hub", "desktop", "layouts"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try { pg.layoutOptions = JSON.parse(this.text); } catch (e) {}
            }
        }
    }

    // xkb variant names repeat the language ("French (AZERTY)"); inside an
    // already-chosen layout that prefix is noise, so keep only the qualifier.
    function variantLabel(name) {
        var m = String(name || "").match(/^[^(]+\((.+)\)$/);
        if (!m)
            return name;
        var q = m[1].trim();
        return q.charAt(0).toUpperCase() + q.slice(1);
    }
    Process {
        id: variantsProc
        property string forLayout: ""
        command: ["ryoku-hub", "desktop", "variants", forLayout]
        stdout: StdioCollector {
            onStreamFinished: {
                var out = [{ "code": "", "name": I18n.tr("Default") }];
                try {
                    var arr = JSON.parse(this.text);
                    for (var i = 0; i < arr.length; i++)
                        out.push({ "code": arr[i].code, "name": pg.variantLabel(arr[i].name) });
                } catch (e) {}
                pg.variantOptions = out;
            }
        }
    }
    // guarded so the same layout never refetches and an empty layout is skipped;
    // driven by the primary-layout change below plus first paint.
    function refreshVariants() {
        var l = pg.primaryLayout(false);
        if (l.length === 0 || variantsProc.forLayout === l)
            return;
        variantsProc.forLayout = l;
        variantsProc.running = false;
        variantsProc.running = true;
    }
    readonly property string curPrimary: pg.primaryLayout(false)
    onCurPrimaryChanged: pg.refreshVariants()

    function nameIn(list, code) {
        for (var i = 0; i < list.length; i++)
            if (list[i].code === code)
                return list[i].name;
        return code;
    }

    // ── system-wide keymap: greeter, TTY, and the boot prompt (privileged) ──
    // Out of band: this writes to /etc and rebuilds the boot image, not the
    // draft, and escalates through polkit.
    property string sysApplyState: ""
    function applyLayoutArg() { return String(pg.hv("desktop.input.kbLayout") || "us"); }
    function applyVariantArg() { return String(pg.hv("desktop.input.kbVariant") || ""); }
    function applyOptionsArg() { return String(pg.hv("desktop.input.kbOptions") || ""); }
    // `ryoku keyboard apply` owns all four layers: it sets the greeter and the
    // console through localectl AND rebuilds the boot image, which localectl
    // alone cannot do. mkinitcpio bakes a copy of /etc/vconsole.conf into the
    // image at build time, so without that rebuild the disk passphrase prompt
    // keeps the keymap it was built with however correct /etc looks.
    Process {
        id: sysApplyProc
        command: ["ryoku", "keyboard", "apply", pg.applyLayoutArg()]
        onExited: (code, status) => {
            pg.sysApplyState = code === 0 ? "ok" : "err";
            sysApplyClear.restart();
        }
    }
    Timer {
        id: sysApplyClear
        interval: 6000
        onTriggered: pg.sysApplyState = ""
    }
    // apply what is on disk: gated on the keyboard keys being saved first.
    readonly property bool kbClean: {
        if (!pg.hub)
            return false;
        return JSON.stringify(pg.hv("desktop.input.kbLayout")) === JSON.stringify(pg.cv("desktop.input.kbLayout"))
            && JSON.stringify(pg.hv("desktop.input.kbVariant")) === JSON.stringify(pg.cv("desktop.input.kbVariant"))
            && JSON.stringify(pg.hv("desktop.input.kbOptions")) === JSON.stringify(pg.cv("desktop.input.kbOptions"));
    }

    readonly property bool swipeOn: pg.hv("desktop.input.workspaceSwipe") === true

    // ── the catalogue overlay (filtered pick over the runtime xkb lists) ─────
    property var catList: null            // [{ code, name }] while open, else null
    property string catTitle: ""
    property string catCurrent: ""        // current code
    property var catApply: null           // function(code)
    function openCat(title, list, current, apply) {
        pg.catTitle = title;
        pg.catList = list;
        pg.catCurrent = current;
        pg.catApply = apply;
        catPicker.open();
    }
    function closeCat() { pg.catList = null; pg.catApply = null; }
    function chooseCat(name) {
        var code = name;
        if (pg.catList)
            for (var i = 0; i < pg.catList.length; i++)
                if (pg.catList[i].name === name) { code = pg.catList[i].code; break; }
        if (pg.catApply)
            pg.catApply(code);
        pg.closeCat();
    }
    readonly property var catNames: {
        var out = [];
        if (pg.catList)
            for (var i = 0; i < pg.catList.length; i++)
                out.push(pg.catList[i].name);
        return out;
    }
    readonly property string catCurrentName: {
        if (pg.catList)
            for (var i = 0; i < pg.catList.length; i++)
                if (pg.catList[i].code === pg.catCurrent)
                    return pg.catList[i].name;
        return "";
    }

    // ── a scalar row driven straight off a hypr path ────────────────────────
    // sw / seg / slid / step. Fractional ratios ride an integer control scaled
    // by `sc` (the module Slid/Step are integer), and the row numeral shows the
    // real value. Enums whose stored key is not its label carry an opts map, and
    // int-backed enums (followMouse, swipeFingers) write back an int so the
    // draft never diverges from disk as a string.
    component Setting: SettingRow {
        id: st
        property string path: ""
        property string ctl: "sw"
        property var opts: []
        property real lo: 0
        property real hi: 1
        property real sc: 1
        property int dec: 0
        property int stepBy: 1
        property bool asInt: false
        property bool gate: true

        readonly property int optCount: st.opts.length
        readonly property var rawV: (pg.hub && st.path) ? pg.hub.hyprVal(st.path) : undefined
        readonly property var rawD: (pg.hub && st.path) ? pg.hub.hyprCommittedVal(st.path) : undefined
        readonly property real numV: Number(st.rawV) || 0
        readonly property real numD: Number(st.rawD) || 0

        function keyLabel(k) {
            for (var i = 0; i < st.opts.length; i++)
                if (String(st.opts[i].key) === String(k === undefined ? "" : k))
                    return st.opts[i].label;
            return String(k === undefined ? "" : k);
        }
        function labelKey(lb) {
            for (var i = 0; i < st.opts.length; i++)
                if (st.opts[i].label === lb)
                    return st.opts[i].key;
            return lb;
        }
        function fmt(n) { return Number(n).toFixed(st.dec); }

        // a 3+-option segmented needs its own band beneath the text; a switch, a
        // stepper, a slider or a 2-option segmented sits inline at the right.
        readonly property bool segBand: st.ctl === "seg" && st.optCount >= 3

        anchors.left: parent.left
        anchors.right: parent.right
        divider: true
        // a row nothing in the active provider would write is dropped, same rule
        // the schema pages gate on, so a pointer/touchpad knob the compositor has
        // no key for is absent rather than a dead switch.
        visible: st.gate && Settings.modelsKey(st.path)
        source: "hypr"
        block: st.segBand
        controlWidth: st.ctl === "sw" ? 54
            : st.ctl === "step" ? 58
            : st.ctl === "slid" ? Math.min(240, Math.max(160, Math.round(st.width * 0.34)))
            : st.ctl === "seg" ? Math.max(120, 62 * Math.max(2, st.optCount))
            : 54

        // the numeric readout is a stepper's or slider's alone; a switch or a
        // segmented shows its own state, so it leaves the readout empty.
        value: (st.ctl === "step" || st.ctl === "slid") ? st.fmt(st.numV) : ""
        def: st.ctl === "sw" ? (st.rawD === true ? I18n.tr("ON") : I18n.tr("OFF"))
            : st.ctl === "seg" ? st.keyLabel(st.rawD)
            : st.fmt(st.numD)
        changed: (st.ctl === "sw" || st.ctl === "seg")
            ? String(st.rawV) !== String(st.rawD)
            : st.numV !== st.numD

        Loader {
            anchors.fill: parent
            sourceComponent: st.ctl === "sw" ? swC
                : st.ctl === "seg" ? segC
                : st.ctl === "slid" ? slidC
                : st.ctl === "step" ? stepC : null
        }
        Component {
            id: swC
            Sw {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                on: st.rawV === true
                onToggled: (v) => pg.he(st.path, v)
            }
        }
        Component {
            id: segC
            Seg {
                anchors.right: parent.right
                anchors.left: st.segBand ? parent.left : undefined
                anchors.verticalCenter: parent.verticalCenter
                options: pg.labels(st.opts)
                current: st.keyLabel(st.rawV)
                onChose: (lb) => {
                    var k = st.labelKey(lb);
                    pg.he(st.path, st.asInt ? parseInt(k, 10) : k);
                }
            }
        }
        Component {
            id: slidC
            Slid {
                anchors.fill: parent
                from: Math.round(st.lo * st.sc)
                to: Math.round(st.hi * st.sc)
                value: Math.round(st.numV * st.sc)
                onModified: (v) => pg.he(st.path,
                    st.dec > 0 ? Number((v / st.sc).toFixed(st.dec)) : Math.round(v / st.sc))
            }
        }
        Component {
            id: stepC
            Step {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                from: Math.round(st.lo)
                to: Math.round(st.hi)
                stepBy: st.stepBy
                value: Math.round(st.numV)
                onModified: (v) => pg.he(st.path, v)
            }
        }
    }

    // ── a kb_options family row (seg / chips / sw over the shared string) ────
    component OptCell: SettingRow {
        id: oc
        property string cellLabel: ""
        property string cellDesc: ""
        property var ids: []
        property var map: []
        property string kind: "seg"
        property string onKey: ""        // the token written when a sw is on
        property bool gate: true

        readonly property int optCount: oc.map.length
        readonly property string curKey: pg.pickFrom(oc.ids, false)
        readonly property string defKey: pg.pickFrom(oc.ids, true)

        // chips and a 3+-option segmented take a band beneath the text; a switch
        // sits inline at the row's right.
        readonly property bool segBand: oc.kind === "seg" && oc.optCount >= 3

        anchors.left: parent.left
        anchors.right: parent.right
        divider: true
        visible: oc.gate
        source: "hypr"
        block: oc.kind === "chips" || oc.segBand
        controlWidth: oc.kind === "sw" ? 54 : Math.max(120, 62 * Math.max(2, oc.optCount))
        label: oc.cellLabel
        desc: oc.cellDesc
        value: ""
        def: oc.kind === "sw" ? (oc.defKey === oc.onKey ? I18n.tr("ON") : I18n.tr("OFF"))
            : oc.kind === "chips" ? "" : pg.mapLabel(oc.map, oc.defKey)
        changed: oc.curKey !== oc.defKey

        Loader {
            anchors.fill: parent
            sourceComponent: oc.kind === "sw" ? swCmp : oc.kind === "chips" ? chipsCmp : segCmp
        }
        Component {
            id: swCmp
            Sw {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                on: oc.curKey === oc.onKey
                onToggled: (v) => pg.setOption(oc.ids, v ? oc.onKey : "")
            }
        }
        Component {
            id: segCmp
            Seg {
                anchors.right: parent.right
                anchors.left: oc.segBand ? parent.left : undefined
                anchors.verticalCenter: parent.verticalCenter
                options: pg.labels(oc.map)
                current: pg.mapLabel(oc.map, oc.curKey)
                onChose: (lb) => pg.setOption(oc.ids, pg.mapKey(oc.map, lb))
            }
        }
        Component {
            id: chipsCmp
            Chips {
                anchors.fill: parent
                options: pg.labels(oc.map)
                current: pg.mapLabel(oc.map, oc.curKey)
                onChose: (lb) => pg.setOption(oc.ids, pg.mapKey(oc.map, lb))
            }
        }
    }

    // ── a catalogue pick row (layout / variant) ─────────────────────────────
    // The catalogue rides a PickBar in the row's foot band; the current value is
    // the bar's own readout, and the 2px changed edge carries the dirty cue.
    component PickCell: SettingRow {
        id: pc
        property string cellLabel: ""
        property string cellDesc: ""
        property string pickTitle: ""
        property var list: []
        property string currentCode: ""
        property string committedCode: ""
        property var applyFn: null

        anchors.left: parent.left
        anchors.right: parent.right
        divider: true
        footH: 32
        source: "hypr"
        label: pc.cellLabel
        desc: pc.cellDesc
        value: ""
        changed: pc.currentCode !== pc.committedCode

        PickBar {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            value: pg.nameIn(pc.list, pc.currentCode)
            count: pc.list.length
            onOpened: pg.openCat(pc.pickTitle, pc.list, pc.currentCode, pc.applyFn)
        }
    }

    // background typography: the section kanji set huge and faint behind the
    // whole page, bled off the lower-right and softened, so it dresses the dead
    // margin without ever competing with a control. Texture, not text to read.
    Watermark {
        anchors.fill: parent
        text: "入力"
    }

    // ── head: eyebrow, Fraunces title, blurb (matches every settings page) ──
    Column {
        id: head
        anchors.top: parent.top
        // the head sits on the body's grid: the left inset and width of the
        // cards below, so the title lands over the first column, not centred.
        width: parent.width - Tokens.s3
        x: 0
        // the register row sits off the title: a rule over a 32px
        // title needs more than the gap between two lines of body text
        spacing: Tokens.s3

        Row {
            // the register row holds a fixed box, so the rule and the seal keep
            // their distance from the title on every page
            height: Tokens.s5
            spacing: Tokens.s2
            Rectangle {
                width: 16; height: 1; color: Tokens.ink
                anchors.verticalCenter: parent.verticalCenter
            }
            Text {
                text: "力"; color: Tokens.ink; font.family: Tokens.jp
                font.pixelSize: 11; anchors.verticalCenter: parent.verticalCenter
            }
            Text {
                text: I18n.tr("DEVICES"); color: Tokens.inkMuted; font.family: Tokens.ui
                font.pixelSize: 9; font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                anchors.verticalCenter: parent.verticalCenter
            }
        }
        Text {
            text: I18n.tr("Input"); color: Tokens.ink
            font.family: Tokens.display; font.pixelSize: Tokens.fTitle
        }
        Text {
            width: Math.min(parent.width, 720)
            text: pg.tab === "MOUSE"
                ? I18n.tr("Extra buttons on a connected mouse, rebound to keys. Touchpads and laptop pads are left alone.")
                : I18n.tr("Keyboard, pointer and touchpad, and key repeat.")
            color: Tokens.inkMuted; font.family: Tokens.ui
            font.pixelSize: Tokens.fBody; wrapMode: Text.WordWrap
        }
        Tabs {
            options: pg.tabs
            current: pg.tab
            onChose: (label) => pg.switchTab(label)
        }
    }

    // ── the tab host: the keyboard band + body and the mouse body swap with a
    // settle slide, never a fade. Both panes are live; only the visible one
    // takes input, and the host clips, so a mid-slide pane never paints over
    // the rail.
    Item {
        id: tabHost
        anchors { left: parent.left; right: parent.right; top: head.bottom; bottom: parent.bottom }
        clip: true
        property int dir: 1
        property string prevTab: "KEYBOARD"
        function travel() {
            dir = pg.tabs.indexOf(pg.tab) >= pg.tabs.indexOf(prevTab) ? 1 : -1;
            prevTab = pg.tab;
            var pane = pg.tab === "MOUSE" ? mousePane : kbPane;
            var other = pg.tab === "MOUSE" ? kbPane : mousePane;
            pane.x = dir * Tokens.s6 * 2;
            pane.visible = true;
            other.visible = false;
            other.x = 0;
            paneAnim.target = pane;
            paneAnim.restart();
        }
        NumberAnimation {
            id: paneAnim
            property: "x"
            to: 0
            duration: Tokens.move
            easing.type: Tokens.ease
        }

    Item {
        id: kbPane
        anchors.fill: parent

    // ── KEYBOARD MAP: pinned under the head so it stays in view while you edit,
    // never scrolled to. A compact live diagram of the layout and the remaps,
    // beside the facts behind it -- the held layout, its variant, and the keys
    // the remaps below will write -- so the band says something at a glance
    // instead of leaving half its width blank.
    Section {
        id: kbmSect
        anchors { top: parent.top; topMargin: Tokens.s5 }
        // the showcase spans the body: its rule runs the width of the cards
        width: parent.width - Tokens.s3
        x: 0
        title: I18n.tr("KEYBOARD MAP")
        titleColor: Tokens.sunDeep

        Row {
            spacing: Tokens.s6
            KeyboardMap {
                id: pinnedMap
                compact: true
                keyMax: 40
                width: Math.min(500, Math.round(kbmSect.span(Spans.cols) * 0.46))
                height: implicitHeight
                layoutCode: pg.primaryLayout(false)
                layoutName: pg.nameIn(pg.layoutOptions, pg.primaryLayout(false))
                styleName: pg.nameIn(pg.variantOptions, pg.primaryVariant(false))
                capsFn: pg.pickFrom(pg.capsIds, false)
                swapAltSuper: pg.pickFrom([pg.swapId], false) === pg.swapId
                composeKey: pg.pickFrom(pg.composeIds, false)
                switchChord: pg.pickFrom(pg.grpIds, false)
                numlock: pg.hv("desktop.input.numlockByDefault") === true
            }

            // the facts the diagram draws, in words: a mono label over its
            // value, laid out across the whole band beside the map. They used
            // to stack in one narrow column and leave the right half of the
            // page empty; three columns of paired label/value fill the measure
            // and read as a register, not a queue.
            Grid {
                width: Math.max(200, kbmSect.span(Spans.cols) - pinnedMap.width - Tokens.s6)
                anchors.verticalCenter: pinnedMap.verticalCenter
                columns: 3
                rowSpacing: Tokens.s4
                columnSpacing: Tokens.s5
                readonly property real cellW: (width - 2 * columnSpacing) / 3
                Repeater {
                    model: [
                        { k: I18n.tr("LAYOUT"), v: pg.nameIn(pg.layoutOptions, pg.primaryLayout(false)) },
                        { k: I18n.tr("VARIANT"), v: pg.nameIn(pg.variantOptions, pg.primaryVariant(false)) },
                        { k: I18n.tr("CAPS"), v: pg.mapLabel(pg.capsMap, pg.pickFrom(pg.capsIds, false)) },
                        { k: I18n.tr("COMPOSE"), v: pg.mapLabel(pg.composeMap, pg.pickFrom(pg.composeIds, false)) },
                        { k: I18n.tr("SWITCH"), v: pg.mapLabel(pg.grpMap, pg.pickFrom(pg.grpIds, false)) },
                        { k: I18n.tr("NUMLOCK"), v: pg.hv("desktop.input.numlockByDefault") === true ? I18n.tr("On at login") : I18n.tr("Off") }
                    ]
                    delegate: Column {
                        required property var modelData
                        width: parent.cellW
                        spacing: 1
                        Text {
                            text: modelData.k
                            color: Tokens.inkFaint
                            font.family: Tokens.mono; font.pixelSize: Tokens.fTiny
                            font.letterSpacing: Tokens.trackMark
                        }
                        Text {
                            width: parent.width
                            text: modelData.v || "—"
                            color: Tokens.ink
                            font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                            elide: Text.ElideRight
                        }
                    }
                }
            }
        }
    }

    // ── the scrolling body ──────────────────────────────────────────────────
    Flickable {
        id: flick
        anchors {
            left: parent.left; right: parent.right
            top: kbmSect.bottom; bottom: parent.bottom
            topMargin: Tokens.s5
        }
        contentWidth: width
        contentHeight: Math.max(body.height, height)
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
        WheelScroll { }

        CardColumns {
            id: body
            // a body of cards fills the measure and splits into balanced columns
            width: flick.width - Tokens.s3
            spacing: Tokens.s5
            // short once the rarely-touched drawers fold: centre the cards in
            // the body instead of hanging them off the top of a wide window.
            fillTo: flick.height

            SettingCard {
                width: body.colWidth
                title: I18n.tr("KEYBOARD")

                PickCell {
                    divider: false
                    cellLabel: I18n.tr("Layout")
                    cellDesc: I18n.tr("What your keys type: QWERTY, AZERTY, Dvorak.")
                    pickTitle: I18n.tr("KEYBOARD LAYOUT")
                    list: pg.layoutOptions
                    currentCode: pg.primaryLayout(false)
                    committedCode: pg.primaryLayout(true)
                    applyFn: function (code) { pg.setLayouts(code, pg.secondaryLayout(false)); }
                }
                PickCell {
                    cellLabel: I18n.tr("Style")
                    cellDesc: I18n.tr("A tweak on the layout, like intl or Colemak.")
                    pickTitle: I18n.tr("LAYOUT STYLE")
                    list: pg.variantOptions
                    currentCode: pg.primaryVariant(false)
                    committedCode: pg.primaryVariant(true)
                    applyFn: function (code) { pg.setVariant(code); }
                }
                PickCell {
                    cellLabel: I18n.tr("Second layout")
                    cellDesc: I18n.tr("A spare layout kept loaded; a chord switches to it.")
                    pickTitle: I18n.tr("SECOND LAYOUT")
                    list: [{ "code": "", "name": I18n.tr("None") }].concat(pg.layoutOptions)
                    currentCode: pg.secondaryLayout(false)
                    committedCode: pg.secondaryLayout(true)
                    applyFn: function (code) { pg.setLayouts(pg.primaryLayout(false), code); }
                }
                OptCell {
                    gate: pg.secondaryLayout(false).length > 0
                    cellLabel: I18n.tr("Switch layouts")
                    cellDesc: I18n.tr("The chord that flips between your two layouts.")
                    ids: pg.grpIds
                    map: pg.grpMap
                    kind: "seg"
                }
                Setting {
                    path: "desktop.input.numlockByDefault"
                    ctl: "sw"
                    label: I18n.tr("Numlock on at login")
                    desc: I18n.tr("Start each session with the keypad typing digits.")
                }
            }

            SettingCard {
                width: body.colWidth
                title: I18n.tr("POINTER")

                Setting {
                    divider: false
                    path: "desktop.input.sensitivity"
                    ctl: "slid"; lo: -1; hi: 1; sc: 100; dec: 2
                    label: I18n.tr("Sensitivity")
                    desc: I18n.tr("Pointer speed offset; 0 is default.")
                }
                Setting {
                    path: "desktop.input.mouseScrollFactor"
                    ctl: "slid"; lo: 0.2; hi: 3; sc: 10; dec: 1; unit: "×"
                    label: I18n.tr("Scroll speed")
                    desc: I18n.tr("Multiplier on each wheel notch.")
                }
                Setting {
                    path: "desktop.input.followMouse"
                    ctl: "seg"; asInt: true
                    opts: [{ "key": 0, "label": I18n.tr("Ignore pointer movement") }, { "key": 1, "label": I18n.tr("Focus under pointer") }, { "key": 2, "label": I18n.tr("Click to focus") }]
                    label: I18n.tr("Focus behavior")
                    desc: I18n.tr("How windows take focus as the pointer moves.")
                }
                Setting {
                    path: "desktop.input.leftHanded"
                    ctl: "sw"
                    label: I18n.tr("Left-handed buttons")
                    desc: I18n.tr("Swap the left and right mouse buttons.")
                }
                Setting {
                    path: "desktop.input.accelProfile"
                    ctl: "seg"
                    opts: [{ "key": "", "label": I18n.tr("Default") }, { "key": "flat", "label": I18n.tr("Flat") }, { "key": "adaptive", "label": I18n.tr("Adaptive") }]
                    label: I18n.tr("Acceleration")
                    desc: I18n.tr("Flat ties travel to the hand; Adaptive speeds quick moves.")
                }
                Setting {
                    path: "desktop.input.mouseNaturalScroll"
                    ctl: "sw"
                    label: I18n.tr("Natural scroll")
                    desc: I18n.tr("Roll the wheel up and the page moves up.")
                }
                Setting {
                    path: "desktop.input.middleClickPaste"
                    ctl: "sw"
                    label: I18n.tr("Middle-click pastes")
                    desc: I18n.tr("Press the wheel to insert the last highlighted text.")
                }
            }

            SettingCard {
                id: cursorSect
                width: body.colWidth
                title: I18n.tr("CURSOR")

                PickCell {
                    divider: false
                    cellLabel: I18n.tr("Theme")
                    cellDesc: I18n.tr("The installed pointer set; applies now and to new apps.")
                    pickTitle: I18n.tr("POINTER THEME")
                    list: pg.cursorThemeList
                    currentCode: pg.hub ? String(pg.hub.hyprVal("desktop.cursor.theme") || "DYNAMIC") : ""
                    committedCode: pg.hub ? String(pg.hub.hyprCommittedVal("desktop.cursor.theme") || "DYNAMIC") : ""
                    applyFn: function (code) { pg.he("desktop.cursor.theme", code); }
                }
                Setting {
                    path: "desktop.cursor.size"
                    ctl: "step"; lo: 12; hi: 64; stepBy: 2; asInt: true
                    label: I18n.tr("Size")
                    desc: I18n.tr("How large the pointer is drawn.")
                    unit: "px"
                }
                Setting {
                    path: "desktop.cursor.inactiveTimeout"
                    ctl: "step"; lo: 0; hi: 30; stepBy: 1; asInt: true
                    label: I18n.tr("Hide after idle")
                    desc: I18n.tr("Seconds of stillness before it hides; 0 never hides.")
                    unit: "s"
                }
                Setting {
                    path: "desktop.cursor.hideOnKeyPress"
                    ctl: "sw"
                    label: I18n.tr("Hide while typing")
                    desc: I18n.tr("It vanishes on a keypress and returns when moved.")
                }
            }

            SettingCard {
                width: body.colWidth
                title: I18n.tr("KEY REPEAT")

                Setting {
                    divider: false
                    path: "desktop.input.repeatRate"
                    ctl: "step"; lo: 1; hi: 100; stepBy: 1; unit: "/s"
                    label: I18n.tr("Repeat rate")
                    desc: I18n.tr("Characters per second while a key is held.")
                }
                Setting {
                    path: "desktop.input.repeatDelay"
                    ctl: "step"; lo: 100; hi: 2000; stepBy: 50; unit: "ms"
                    label: I18n.tr("Repeat delay")
                    desc: I18n.tr("Pause before a held key starts repeating.")
                }
            }

            // rarely touched once set: folded by default so the page leads with
            // layout, pointer and cursor. The summary says what the drawer holds.
            SettingCard {
                width: body.colWidth
                title: I18n.tr("KEY REMAPS")
                expanded: false
                summary: I18n.tr("CAPS, COMPOSE, XKB")

                OptCell {
                    divider: false
                    cellLabel: I18n.tr("Caps Lock")
                    cellDesc: I18n.tr("Remap it to Escape, Ctrl, or switch it off.")
                    ids: pg.capsIds
                    map: pg.capsMap
                    kind: "chips"
                }
                OptCell {
                    cellLabel: I18n.tr("Swap Alt and Super")
                    cellDesc: I18n.tr("Trade the two keys for macOS-style shortcuts.")
                    ids: [pg.swapId]
                    map: []
                    kind: "sw"
                    onKey: pg.swapId
                }
                OptCell {
                    cellLabel: I18n.tr("Compose key")
                    cellDesc: I18n.tr("Starts a sequence for accents like é and ñ.")
                    ids: pg.composeIds
                    map: pg.composeMap
                    kind: "seg"
                }

                // Extra options: the free-text escape hatch. Everything the family
                // pickers do not recognise round-trips here. Commit on
                // editing-finished (not per keystroke, which would fight the
                // tokeniser mid-word) and re-bind to the draft on focus loss so
                // Reset/Revert refresh the shown text after a manual edit.
                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    divider: true
                    footH: 32
                    source: "hypr"
                    label: I18n.tr("Extra options")
                    desc: I18n.tr("Raw xkb options, comma separated, for power users.")
                    changed: pg.extraOptions(false) !== pg.extraOptions(true)

                    Field {
                        id: extraField
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        tabular: true
                        placeholder: I18n.tr("raw xkb options, comma separated")
                        text: pg.extraOptions(false)
                        onCommitted: (v) => pg.setExtra(v)
                        onFocusedChanged: if (!extraField.focused)
                            extraField.text = Qt.binding(function () { return pg.extraOptions(false); })
                    }
                }

                // Apply system-wide: privileged localectl write to /etc, gated on
                // the keyboard keys already being saved so the console matches
                // what the compositor shows.
                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    divider: true
                    controlWidth: 84
                    source: "vconsole"
                    label: I18n.tr("Apply system-wide")
                    desc: I18n.tr("Match the login screen, TTYs, and boot prompt too.")
                    changed: false

                    Btn {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("APPLY")
                        armed: pg.kbClean && !sysApplyProc.running
                        onAct: sysApplyProc.running = true
                    }
                }
                // paired readout: the last apply result, self-clearing after 6s.
                // No colour; the word carries the state (DESIGN.md section 1).
                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    divider: true
                    source: "vconsole"
                    label: I18n.tr("Login screen, TTY, and boot")
                    value: pg.sysApplyState === "ok" ? I18n.tr("APPLIED")
                        : pg.sysApplyState === "err" ? I18n.tr("FAILED") : I18n.tr("READY")
                    desc: pg.sysApplyState === "ok" ? I18n.tr("Applied to the login screen, console, and boot prompt.")
                        : pg.sysApplyState === "err" ? I18n.tr("Not applied. Cancelled or failed.")
                        : I18n.tr("They keep their own keymap until you apply.")
                    changed: false
                }
            }

            SettingCard {
                width: body.colWidth
                title: I18n.tr("TOUCHPAD")
                expanded: false
                summary: I18n.tr("TAP, SCROLL, SWIPE")

                // The touchpad lock, the FN touchpad key's job, as a switch. A
                // live seam action rather than a saved setting, so it applies at
                // once and shows the pad's real state; hidden where the
                // compositor cannot toggle a pad.
                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    visible: Settings.supports("touchpadToggle")
                    divider: false
                    controlWidth: 54
                    source: "hypr"
                    label: I18n.tr("Touchpad")
                    desc: I18n.tr("Turn the touchpad off, the way the FN touchpad key does.")
                    changed: false

                    Sw {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        on: pg.touchpadOn
                        onToggled: (v) => pg.setTouchpad(v)
                    }
                }
                Setting {
                    divider: true
                    path: "desktop.input.naturalScroll"
                    ctl: "sw"
                    label: I18n.tr("Natural scroll")
                    desc: I18n.tr("Two fingers drag the content like a touchscreen.")
                }
                Setting {
                    path: "desktop.input.tapToClick"
                    ctl: "sw"
                    label: I18n.tr("Tap to click")
                    desc: I18n.tr("A tap counts as a click; two fingers right, three middle.")
                }
                Setting {
                    path: "desktop.input.tapAndDrag"
                    ctl: "sw"
                    label: I18n.tr("Tap and drag")
                    desc: I18n.tr("Tap, then hold the finger down to drag what you tapped.")
                }
                Setting {
                    path: "desktop.input.disableWhileTyping"
                    ctl: "sw"
                    label: I18n.tr("Disable while typing")
                    desc: I18n.tr("Ignore the pad while typing so a palm can't nudge it.")
                }
                Setting {
                    path: "desktop.input.clickfinger"
                    ctl: "sw"
                    label: I18n.tr("Click by finger count")
                    desc: I18n.tr("One finger clicks left, two right, three middle.")
                }
                Setting {
                    path: "desktop.input.middleEmulation"
                    ctl: "sw"
                    label: I18n.tr("Emulate middle click")
                    desc: I18n.tr("Press left and right together for a middle click.")
                }
                Setting {
                    path: "desktop.input.touchScrollFactor"
                    ctl: "slid"; lo: 0.2; hi: 3; sc: 10; dec: 1; unit: "×"
                    label: I18n.tr("Scroll speed")
                    desc: I18n.tr("Multiplier on two-finger scroll.")
                }
                Setting {
                    path: "desktop.input.workspaceSwipe"
                    ctl: "sw"
                    label: I18n.tr("Swipe between workspaces")
                    desc: I18n.tr("A horizontal swipe slides to the next workspace.")
                }
                Setting {
                    path: "desktop.input.swipeFingers"
                    ctl: "seg"; asInt: true; gate: pg.swipeOn
                    opts: [{ "key": 3, "label": "3" }, { "key": 4, "label": "4" }]
                    label: I18n.tr("Swipe fingers")
                    desc: I18n.tr("How many fingers count as a workspace swipe.")
                }
                Setting {
                    path: "desktop.input.swipeInvert"
                    ctl: "sw"; gate: pg.swipeOn
                    label: I18n.tr("Natural swipe direction")
                    desc: I18n.tr("The workspace row follows your fingers.")
                }
                Setting {
                    path: "desktop.input.swipeCreateNew"
                    ctl: "sw"; gate: pg.swipeOn
                    label: I18n.tr("Swipe past the last workspace")
                    desc: I18n.tr("Swiping past the end opens a fresh workspace.")
                }
                Setting {
                    path: "desktop.input.swipeDistance"
                    ctl: "slid"; lo: 100; hi: 600; sc: 1; dec: 0; unit: "px"; gate: pg.swipeOn
                    label: I18n.tr("Swipe distance")
                    desc: I18n.tr("Finger travel for a full switch.")
                }
            }
        }
    }

    }   // kbPane

    // ── MOUSE: the extra buttons of a connected mouse, rebound to keys ────
    // The daemon enumerates /dev/input by capability (a pad is never a mouse,
    // a laptop alone shows the empty plate), and the remapper lives under the
    // compositor: it grabs the device, replays it through a uinput clone, and
    // swaps a bound button for the key chord. Bindings apply the moment they
    // are set -- no Save, because the daemon owns the file, not the store.
    Flickable {
        id: mousePane
        anchors { left: parent.left; right: parent.right; top: parent.top; bottom: parent.bottom }
        topMargin: Tokens.s5
        visible: false
        contentWidth: width
        contentHeight: Math.max(mouseBody.height, height)
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
        WheelScroll { }

        CardColumns {
            id: mouseBody
            width: mousePane.width - Tokens.s3
            spacing: Tokens.s5
            fillTo: mousePane.height

            // the cheap way in: press the button you mean and the daemon finds
            // its device and code, so you never guess which of five is "Side".
            SettingCard {
                width: mouseBody.colWidth
                visible: pg.mouseDevices.length > 0
                collapsible: false
                title: I18n.tr("BIND A BUTTON")
                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    divider: false
                    controlWidth: 110
                    label: I18n.tr("Press to detect")
                    desc: I18n.tr("Click an extra button on the mouse, then the key chord you want it to send.")
                    changed: false
                    Btn {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        compact: true
                        text: I18n.tr("DETECT")
                        onAct: pg.startDetect()
                    }
                }
                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    visible: pg.mouseError !== ""
                    divider: false
                    label: I18n.tr("Last change")
                    desc: pg.mouseError
                    changed: false
                }
            }

            SettingCard {
                width: mouseBody.colWidth
                visible: pg.macroEditor !== null
                collapsible: false
                title: I18n.tr("MACRO SEQUENCE")

                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    divider: false
                    label: pg.macroEditor
                        ? pg.buttonLabel(pg.macroEditor.button, pg.macroEditor.device)
                        : ""
                    desc: I18n.tr("Runs these steps in order without holding up pointer input.")
                    value: pg.macroEditor ? pg.mouseDisplayName(pg.macroEditor.device) : ""
                    changed: false
                }
                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    label: I18n.tr("Record timing")
                    desc: I18n.tr("Play a key sequence naturally; pauses are kept as delay steps.")
                    controlWidth: 112
                    changed: false
                    Btn {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        compact: true
                        text: I18n.tr("RECORD")
                        armed: true
                        onAct: pg.startMacroRecording()
                    }
                }
                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    label: I18n.tr("Add manually")
                    desc: I18n.tr("Add a tap, held edge, release edge, or a 100 ms wait.")
                    controlWidth: 260
                    changed: false
                    Row {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: Tokens.s1
                        Btn { compact: true; text: I18n.tr("TAP"); onAct: pg.startMacroKey("tap") }
                        Btn { compact: true; text: I18n.tr("DOWN"); onAct: pg.startMacroKey("down") }
                        Btn { compact: true; text: I18n.tr("UP"); onAct: pg.startMacroKey("up") }
                        Btn { compact: true; text: I18n.tr("WAIT"); onAct: pg.addMacroDelay() }
                    }
                }
                Repeater {
                    model: pg.macroEditor ? pg.macroEditor.target.sequence : []
                    delegate: SettingRow {
                        required property int index
                        required property var modelData
                        anchors.left: parent.left
                        anchors.right: parent.right
                        label: (index + 1) + ". " + pg.macroStepText(modelData)
                        desc: modelData.kind === "delay"
                            ? I18n.tr("Adjust the pause in 10 ms steps.")
                            : I18n.tr("Chord keys are emitted together.")
                        controlWidth: modelData.kind === "delay" ? 244 : 174
                        changed: false
                        Row {
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            spacing: Tokens.s1
                            Btn {
                                visible: modelData.kind === "delay"
                                compact: true; text: "−"
                                onAct: pg.adjustMacroDelay(index, -10)
                            }
                            Btn {
                                visible: modelData.kind === "delay"
                                compact: true; text: "+"
                                onAct: pg.adjustMacroDelay(index, 10)
                            }
                            Btn {
                                compact: true; text: I18n.tr("UP")
                                armed: index > 0
                                onAct: pg.moveMacroStep(index, -1)
                            }
                            Btn {
                                compact: true; text: I18n.tr("DOWN")
                                armed: pg.macroEditor && index < pg.macroEditor.target.sequence.length - 1
                                onAct: pg.moveMacroStep(index, 1)
                            }
                            Btn {
                                compact: true; text: I18n.tr("REMOVE")
                                armed: true
                                onAct: pg.removeMacroStep(index)
                            }
                        }
                    }
                }
                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    label: I18n.tr("Repeat")
                    desc: I18n.tr("Number of times the full sequence runs.")
                    controlWidth: 126
                    changed: false
                    Row {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: Tokens.s2
                        Btn { compact: true; text: "−"; onAct: pg.setMacroRepeat(-1) }
                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            text: pg.macroEditor ? String(pg.macroEditor.target.repeat || 1) : "1"
                            color: Tokens.ink
                            font.family: Tokens.mono
                            font.pixelSize: Tokens.fSmall
                        }
                        Btn { compact: true; text: "+"; onAct: pg.setMacroRepeat(1) }
                    }
                }
                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    label: I18n.tr("On button release")
                    desc: I18n.tr("Cancel immediately and release any held keys, or let the sequence finish.")
                    controlWidth: 148
                    changed: false
                    Btn {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        compact: true
                        text: pg.macroEditor && pg.macroEditor.target.cancelOnRelease
                            ? I18n.tr("CANCEL") : I18n.tr("FINISH")
                        armed: pg.macroEditor && pg.macroEditor.target.cancelOnRelease
                        onAct: pg.toggleMacroRelease()
                    }
                }
                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    divider: false
                    label: I18n.tr("Sequence")
                    desc: I18n.tr("Test sends the draft now. Save assigns it to the mouse button.")
                    controlWidth: 220
                    changed: false
                    Row {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: Tokens.s2
                        Btn {
                            compact: true; text: I18n.tr("TEST")
                            armed: pg.macroEditor && pg.macroEditor.target.sequence.length > 0
                            onAct: pg.testMacro()
                        }
                        Btn {
                            compact: true; text: I18n.tr("CLOSE")
                            onAct: pg.macroEditor = null
                        }
                        Btn {
                            compact: true; text: I18n.tr("SAVE")
                            armed: pg.macroEditor && pg.macroEditor.target.sequence.length > 0
                            onAct: pg.saveMacro()
                        }
                    }
                }
            }
            // one card per connected mouse, each row an extra button
            Repeater {
                model: pg.mouseDevices
                delegate: SettingCard {
                    id: devCard
                    required property var modelData
                    width: mouseBody.colWidth
                    title: ((modelData.brand ? modelData.brand + " · " : "")
                        + (modelData.model || modelData.name || modelData.id)).toUpperCase()

                    readonly property var side: modelData.sideButtons || []
                    readonly property var extra: (modelData.buttons || []).filter(function (code) {
                        return code >= 275 && devCard.side.indexOf(code) < 0;
                    })

                    SettingRow {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        divider: false
                        source: "mousemap.json"
                        label: modelData.model
                            ? I18n.tr("Recognized model")
                            : I18n.tr("Generic mouse")
                        desc: modelData.model
                            ? I18n.tr("Friendly names come from the built-in device catalogue.")
                            : I18n.tr("Every Linux mouse remains bindable even when it is not in the catalogue.")
                        value: String(modelData.bus || "").toUpperCase()
                        changed: false
                    }

                    Column {
                        width: parent.width
                        visible: devCard.side.length > 0
                        spacing: Tokens.s3
                        topPadding: Tokens.s3
                        bottomPadding: Tokens.s3

                        Text {
                            leftPadding: Tokens.s4
                            text: I18n.tr("SIDE GRID")
                            color: Tokens.inkDim
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall
                            font.weight: Font.Medium
                            font.letterSpacing: Tokens.trackLabel
                        }
                        Grid {
                            width: parent.width - Tokens.s4 * 2
                            anchors.horizontalCenter: parent.horizontalCenter
                            columns: 3
                            spacing: Tokens.s2
                            Repeater {
                                model: devCard.side
                                delegate: Rectangle {
                                    required property var modelData
                                    width: (parent.width - Tokens.s2 * 2) / 3
                                    height: 78
                                    radius: Tokens.radius
                                    color: sideHover.hovered ? Tokens.tint5 : "transparent"
                                    border.width: Tokens.border
                                    border.color: pg.mapFor(devCard.modelData.id, modelData)
                                        ? Tokens.lineStrong : Tokens.line
                                    Behavior on color { ColorAnimation { duration: Tokens.snap } }

                                    Column {
                                        anchors {
                                            left: parent.left; right: parent.right
                                            top: parent.top; margins: Tokens.s2
                                        }
                                        spacing: Tokens.s1
                                        Text {
                                            width: parent.width
                                            text: pg.buttonLabel(modelData, devCard.modelData.id)
                                            color: Tokens.ink
                                            font.family: Tokens.ui
                                            font.pixelSize: Tokens.fSmall
                                            elide: Text.ElideRight
                                        }
                                        Text {
                                            width: parent.width
                                            text: pg.targetText(devCard.modelData.id, modelData)
                                            color: Tokens.inkDim
                                            font.family: Tokens.mono
                                            font.pixelSize: Tokens.fTiny
                                            elide: Text.ElideRight
                                        }
                                    }
                                    HoverHandler { id: sideHover; cursorShape: Qt.PointingHandCursor }
                                    MouseArea {
                                        anchors.fill: parent
                                        cursorShape: Qt.PointingHandCursor
                                        onPressed: mouse => {
                                            var macroPoint = sideMacro.mapFromItem(parent, mouse.x, mouse.y);
                                            var clearPoint = sideClear.mapFromItem(parent, mouse.x, mouse.y);
                                            var overMacro = macroPoint.x >= 0 && macroPoint.x <= sideMacro.width
                                                && macroPoint.y >= 0 && macroPoint.y <= sideMacro.height;
                                            var overClear = sideClear.visible
                                                && clearPoint.x >= 0 && clearPoint.x <= sideClear.width
                                                && clearPoint.y >= 0 && clearPoint.y <= sideClear.height;
                                            if (overMacro || overClear)
                                                mouse.accepted = false;
                                        }
                                        onClicked: pg.startBind(devCard.modelData.id, modelData)
                                    }
                                    Btn {
                                        id: sideMacro
                                        anchors {
                                            left: parent.left; bottom: parent.bottom
                                            margins: Tokens.s1
                                        }
                                        compact: true
                                        text: I18n.tr("MACRO")
                                        onAct: pg.openMacro(devCard.modelData.id, modelData)
                                    }
                                    Btn {
                                        id: sideClear
                                        visible: !!pg.mapFor(devCard.modelData.id, modelData)
                                        anchors {
                                            right: parent.right; bottom: parent.bottom
                                            margins: Tokens.s1
                                        }
                                        compact: true
                                        text: I18n.tr("CLEAR")
                                        onAct: pg.clearBinding(devCard.modelData.id, modelData)
                                    }
                                }
                            }
                        }
                    }

                    Repeater {
                        model: devCard.extra
                        delegate: SettingRow {
                            id: btnRow
                            required property int index
                            required property var modelData
                            anchors.left: parent.left
                            anchors.right: parent.right
                            divider: true
                            controlWidth: 224
                            source: "mousemap.json"
                            label: pg.buttonLabel(modelData, devCard.modelData.id)
                            desc: pg.mapFor(devCard.modelData.id, modelData)
                                ? I18n.tr("Rewritten before the focused app receives it.")
                                : I18n.tr("Still sends its default to apps.")
                            value: ""
                            changed: false

                            Row {
                                anchors.right: parent.right
                                anchors.verticalCenter: parent.verticalCenter
                                spacing: Tokens.s1
                                Text {
                                    anchors.verticalCenter: parent.verticalCenter
                                    text: pg.targetText(devCard.modelData.id, btnRow.modelData)
                                    color: pg.mapFor(devCard.modelData.id, btnRow.modelData) ? Tokens.ink : Tokens.inkDim
                                    font.family: Tokens.mono
                                    font.pixelSize: Tokens.fTiny
                                }
                                Btn {
                                    compact: true
                                    text: I18n.tr("CHORD")
                                    onAct: pg.startBind(devCard.modelData.id, btnRow.modelData)
                                }
                                Btn {
                                    compact: true
                                    text: I18n.tr("MACRO")
                                    onAct: pg.openMacro(devCard.modelData.id, btnRow.modelData)
                                }
                                Btn {
                                    visible: !!pg.mapFor(devCard.modelData.id, btnRow.modelData)
                                    compact: true
                                    text: I18n.tr("CLEAR")
                                    onAct: pg.clearBinding(devCard.modelData.id, btnRow.modelData)
                                }
                            }
                        }
                    }

                    SettingRow {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        visible: devCard.extra.length === 0 && devCard.side.length === 0
                        divider: false
                        source: "mousemap.json"
                        label: I18n.tr("No extra buttons")
                        desc: I18n.tr("Left, right and wheel only; nothing here to rebind.")
                        changed: false
                    }
                }
            }

            SettingCard {
                width: mouseBody.colWidth
                title: I18n.tr("HOW IT WORKS")

                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    divider: false
                    source: "mousemap.json"
                    label: I18n.tr("Compositor-neutral")
                    desc: I18n.tr("The keypress is rewritten before the desktop sees it, so it works in every app, on either window manager, and on the lock screen.")
                    changed: false
                }
                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    source: "mousemap.json"
                    label: I18n.tr("Per mouse")
                    desc: I18n.tr("Each device keeps its own bindings; unplug it and nothing else changes.")
                    changed: false
                }
                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    source: "mousemap.json"
                    label: I18n.tr("Left-handed")
                    desc: I18n.tr("Bindings follow the physical button, so the Left-handed switch does not move them.")
                    changed: false
                }
            }

            // the empty plate: a laptop (or a bare three-button box) has
            // nothing to bind, and the page says so dressed, never blank.
            SettingCard {
                width: mouseBody.colWidth
                visible: pg.mouseDevices.length === 0
                collapsible: false
                title: I18n.tr("NO MOUSE DETECTED")

                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    divider: false
                    label: I18n.tr("Extra buttons")
                    desc: pg.mouseDaemonSeen
                        ? I18n.tr("No mouse is connected. A laptop's own pad is never a mouse, so it stays out of this page.")
                        : I18n.tr("The shell daemon is not answering; ryoku doctor can repair the session.")
                    changed: false
                }
                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    controlWidth: 96
                    label: I18n.tr("Re-detect")
                    desc: I18n.tr("Plug a mouse in and ask again; hotplugs are noticed on their own too.")
                    changed: false
                    Btn {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("RE-DETECT")
                        armed: pg.mouseDaemonSeen
                        onAct: mouseCtl.send("mouse.state", {})
                    }
                }
                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    visible: pg.mouseError !== ""
                    label: I18n.tr("Last change")
                    desc: pg.mouseError
                    changed: false
                }
            }
        }
    }

    }   // tabHost

    // ── the catalogue overlay: paperLift + lineStrong, one z-plane above ──────
    Item {
        anchors.fill: parent
        visible: pg.catList !== null
        z: 900

        Rectangle {
            anchors.fill: parent
            color: Tokens.paper
            opacity: 0.55
            TapHandler { onTapped: pg.closeCat() }
        }
        Picker {
            id: catPicker
            anchors.centerIn: parent
            title: pg.catTitle
            options: pg.catNames
            current: pg.catCurrentName
            onChose: (name) => pg.chooseCat(name)
            onDismissed: pg.closeCat()
        }
    }

    // ── chord and macro recorder ─────────────────────────────────────────────
    // Shortcut inhibition and the record submap keep desktop bindings from
    // firing while a direct chord or a timed sequence is being captured.
    Item {
        visible: pg.capturing
        z: 950
        anchors.fill: parent
        onVisibleChanged: if (visible) chordCapture.forceActiveFocus()

        Rectangle {
            anchors.fill: parent
            color: Tokens.paper
            opacity: 0.55
            TapHandler {
                onTapped: Qt.callLater(function() {
                    if (!pg.capturing)
                        return;
                    if (pg.macroKeyCapture || pg.macroRecording)
                        pg.stopMacroCapture();
                    else
                        pg.stopBind(false, "");
                })
            }
        }

        Item {
            id: chordCapture
            anchors.fill: parent
            focus: true
            Keys.onPressed: (event) => {
                event.accepted = true;
                if (event.isAutoRepeat)
                    return;
                if (event.key === Qt.Key_Escape) {
                    if (pg.macroKeyCapture || pg.macroRecording)
                        pg.stopMacroCapture();
                    else
                        pg.stopBind(false, "");
                    return;
                }
                if (pg.captureWaitClick)
                    return;
                pg.captureForming = Combos.formingChord(event);
                var chord = Combos.chordFrom(event);
                if (chord === "") {
                    if (pg.macroKeyCapture || pg.macroRecording) {
                        var modifier = pg.modifierKeyToken(event.key);
                        pg.macroModifierOnly = modifier !== "" && pg.captureForming === modifier
                            ? modifier : "";
                    }
                    return;
                }
                pg.macroModifierOnly = "";
                if (pg.macroKeyCapture || pg.macroRecording)
                    pg.acceptMacroChord(chord);
                else
                    pg.stopBind(true, chord);
            }
            Keys.onReleased: (event) => {
                if (!(pg.macroKeyCapture || pg.macroRecording) || event.isAutoRepeat)
                    return;
                event.accepted = true;
                var modifier = pg.modifierKeyToken(event.key);
                if (modifier !== "" && modifier === pg.macroModifierOnly) {
                    pg.captureForming = "";
                    pg.acceptMacroChord(modifier);
                }
            }
        }

        Rectangle {
            anchors.centerIn: parent
            width: 380; height: 176
            radius: Tokens.radius
            color: Tokens.paper
            border.width: Tokens.border
            border.color: Tokens.lineStrong

            Column {
                anchors.centerIn: parent
                width: parent.width - Tokens.s4 * 2
                spacing: Tokens.s3

                Text {
                    anchors.horizontalCenter: parent.horizontalCenter
                    text: pg.captureWaitClick
                        ? I18n.tr("PRESS THE MOUSE BUTTON")
                        : pg.macroRecording
                            ? I18n.tr("RECORD KEYS · ESC TO FINISH")
                            : pg.macroKeyCapture
                                ? I18n.tr("PRESS A KEY CHORD")
                                : I18n.tr("PRESS THE KEY CHORD")
                    color: Tokens.ink
                    font.family: Tokens.ui; font.pixelSize: 12
                    font.weight: Font.Medium; font.letterSpacing: Tokens.trackLabel
                }
                Text {
                    anchors.horizontalCenter: parent.horizontalCenter
                    visible: !pg.captureWaitClick
                    text: pg.macroRecording
                        ? (pg.macroEditor ? pg.macroEditor.target.sequence.length + " " + I18n.tr("steps recorded") : "")
                        : pg.macroKeyCapture
                            ? I18n.tr(pg.macroAction.toUpperCase())
                            : pg.buttonLabel(pg.captureFor ? pg.captureFor.button : 0,
                                pg.captureFor ? pg.captureFor.device : "")
                                + (pg.captureFor && pg.captureFor.device ? " · " + pg.captureFor.device : "")
                    color: Tokens.inkDim
                    font.family: Tokens.mono; font.pixelSize: 10
                }
                Text {
                    anchors.horizontalCenter: parent.horizontalCenter
                    text: pg.captureForming !== ""
                        ? pg.captureForming
                        : pg.macroRecording ? I18n.tr("Esc saves the recording") : I18n.tr("Esc to cancel")
                    color: pg.captureForming !== "" ? Tokens.ink : Tokens.inkFaint
                    font.family: Tokens.mono; font.pixelSize: 11
                }
            }
        }
    }

    ShortcutInhibitor {
        window: pg.QsWindow ? pg.QsWindow.window : null
        enabled: pg.capturing
    }
}
