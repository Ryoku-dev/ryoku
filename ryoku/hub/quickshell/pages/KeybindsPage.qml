pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Shapes
import Quickshell.Io
import Quickshell
import Quickshell.Wayland
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../Singletons"
import "../Combos.js" as Combos
import "../fuzzy.js" as Fuzzy

// Keybinds (DESIGN.md section 8, SYSTEM). Three surfaces under one head, built
// for use and not for reading: Shortcuts is the searchable legend (a category
// rail on the left, one calm content column on the right, read live from
// `ryoku-hub keybinds`), Apps swaps the launcher roles and rebinds their keys,
// and Custom is the user-bind editor. A full-width fuzzy search sits in the head
// and drives the legend: type "clw" for Close window, "num 3" for the number-pad
// family, "screen" for the Displays rows. The legend is the source of truth;
// custom binds and rebinds show in it once saved. This is a full-bleed page: it
// owns its whole content region, so it draws its own head, its own tab switch
// and -- because the shell hides its global action bar -- its own Save/Revert
// controls for the shared store. Every value is a Token.
Item {
    id: pg

    property var hub
    readonly property bool fullBleed: true

    // guard the harness and the pre-load window: the shell injects a real hub
    // exposing hyprVal/hyprEdit; a bare probe object exposes neither.
    readonly property bool hubReady: pg.hub && typeof pg.hub.hyprVal === "function"
    // the live custom-bind array from the draft: a list of { keys, action, value }.
    readonly property var customRows: pg.hubReady ? (pg.hub.hyprVal("desktop.keybinds") || []) : []
    // gated so the editor empty state does not flash before `hypr get` returns.
    readonly property bool ready: pg.hubReady ? pg.hub.wmLoaded === true : false
    readonly property int dirtyCount: pg.hubReady ? (pg.hub.dirty || 0) : 0

    // the compositor's own hand-edit config path from the provider; empty with
    // no provider so the copy drops the path rather than printing a bare prefix.
    readonly property string wmCfgPath: {
        var cf = (pg.hub && pg.hub.wmConfigFiles) ? pg.hub.wmConfigFiles : [];
        return cf.length ? "~/.config/" + cf[0] : "";
    }
    // the running compositor's display name, for the "not on <provider>" tag on
    // a legend row the active compositor cannot honour. Read from the provider,
    // never a hardcoded compositor name.
    readonly property string providerName: (Settings.provider && Settings.provider.length) ? Settings.provider : I18n.tr("this desktop")

    // "shortcuts" = the searchable legend (default), "apps" = the app-role
    // launchers, "custom" = the user-bind editor. Transient, not persisted.
    property string tab: "shortcuts"

    // the shipped legend, parsed live by the backend from the neutral catalogue
    // and the active provider. Owned here (a self-contained page fetches its own
    // backend), forwarded to the legend view and the conflict check.
    property var categories: []

    Process {
        id: legendGet
        command: ["ryoku-hub", "keybinds"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var o = JSON.parse(this.text);
                    pg.categories = o.categories || [];
                } catch (e) { console.log("keybinds: legend parse failed: " + e); }
            }
        }
    }

    // ── app roles (Default Apps, merged into this page) ─────────────────────
    // the swappable launcher roles + candidates + each role's shipped combo,
    // from `ryoku-hub apps` (installed candidates flagged). Shown on the Apps
    // tab, where the key rebinds too; the choice is stored in the store's "apps".
    property var roles: []
    Process {
        id: appsGet
        command: ["ryoku-hub", "apps"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    pg.roles = JSON.parse(this.text) || [];
                } catch (e) { console.log("keybinds: apps parse failed: " + e); }
            }
        }
    }

    property var shellState: ({ current: "", choices: [] })
    property string shellError: ""
    property bool shellBusy: false
    Process {
        id: shellGet
        command: ["ryoku-hub", "shell", "get"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    pg.shellState = JSON.parse(this.text);
                } catch (e) {
                    pg.shellError = I18n.tr("Couldn't read the login shell.");
                }
            }
        }
    }
    Process {
        id: shellSet
        stderr: StdioCollector { id: shellSetError }
        onExited: (code, status) => {
            pg.shellBusy = false;
            pg.shellError = code === 0 ? "" : (shellSetError.text.trim() || I18n.tr("Couldn't change the login shell."));
            shellGet.running = true;
        }
    }
    function setShell(key) {
        if (pg.shellBusy || key === pg.shellState.current)
            return;
        pg.shellBusy = true;
        pg.shellError = "";
        shellSet.command = ["ryoku-hub", "shell", "set", key];
        shellSet.running = true;
    }
    function setZshPrompt(mode) {
        if (pg.shellBusy || mode === pg.shellState.zshPrompt)
            return;
        pg.shellBusy = true;
        pg.shellError = "";
        shellSet.command = ["ryoku-hub", "shell", "prompt", mode];
        shellSet.running = true;
    }

    property var starshipState: ({ "layout": "pill", "palette": "fixed" })
    property var starshipLayouts: []
    property string starshipError: ""
    property bool starshipBusy: false
    Process {
        id: starshipGet
        command: ["ryoku-hub", "starship", "get"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    pg.starshipState = JSON.parse(this.text);
                } catch (e) {
                    pg.starshipError = I18n.tr("Couldn't read the Starship settings.");
                }
            }
        }
    }
    Process {
        id: starshipList
        command: ["ryoku-hub", "starship", "list"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    pg.starshipLayouts = JSON.parse(this.text) || [];
                } catch (e) {
                    pg.starshipError = I18n.tr("Couldn't read the Starship layouts.");
                }
            }
        }
    }
    Process {
        id: starshipSet
        stderr: StdioCollector { id: starshipSetError }
        onExited: (code, status) => {
            pg.starshipBusy = false;
            pg.starshipError = code === 0 ? "" : (starshipSetError.text.trim() || I18n.tr("Couldn't change the Starship prompt."));
            starshipGet.running = true;
        }
    }
    function setStarshipLayout(layout) {
        if (pg.starshipBusy || layout === pg.starshipState.layout)
            return;
        pg.starshipBusy = true;
        pg.starshipError = "";
        starshipSet.command = ["ryoku-hub", "starship", "set", layout];
        starshipSet.running = true;
    }
    function setStarshipPalette(mode) {
        if (pg.starshipBusy || mode === pg.starshipState.palette)
            return;
        pg.starshipBusy = true;
        pg.starshipError = "";
        starshipSet.command = ["ryoku-hub", "starship", "palette", mode];
        starshipSet.running = true;
    }
    function starshipName(id) {
        if (id === "pill") return I18n.tr("Pill");
        if (id === "minimal") return I18n.tr("Minimal");
        if (id === "two-line") return I18n.tr("Two line");
        if (id === "powerline") return I18n.tr("Powerline");
        if (id === "lean") return I18n.tr("Lean");
        return id;
    }
    function starshipPreview(id) {
        if (id === "minimal")
            return "󰉋 ~/ryoku  󰊢 unstable  λ";
        if (id === "two-line")
            return "╭─ 󰉋 ~/ryoku  󰊢 unstable\n╰─ λ";
        if (id === "powerline")
            return "󰉋 ~/ryoku󰊢 unstable 󰔚 1s  λ";
        if (id === "lean")
            return "󰉋  ~/ryoku  󰊢 unstable\n❯";
        return " 󰉋 ~/ryoku    󰊢 unstable \nλ";
    }
    function orderedStarshipLayouts() {
        var order = ["pill", "minimal", "two-line", "powerline", "lean"];
        var byID = {};
        for (var i = 0; i < pg.starshipLayouts.length; i++)
            byID[pg.starshipLayouts[i].id] = pg.starshipLayouts[i];
        var out = [];
        for (var j = 0; j < order.length; j++)
            if (byID[order[j]]) out.push(byID[order[j]]);
        for (var k = 0; k < pg.starshipLayouts.length; k++)
            if (order.indexOf(pg.starshipLayouts[k].id) < 0) out.push(pg.starshipLayouts[k]);
        return out;
    }
    // role -> chosen command (draft); empty/absent uses the shipped fallback.
    readonly property var chosen: pg.hubReady ? (pg.hub.hyprVal("desktop.apps") || ({})) : ({})
    function effOf(role, fallback) {
        var v = pg.chosen[role];
        return (v && ("" + v).length) ? ("" + v) : fallback;
    }
    function setApp(role, cmd) {
        if (!pg.hubReady)
            return;
        var cur = pg.hub.hyprVal("desktop.apps") || {};
        var m = {};
        for (var k in cur)
            m[k] = cur[k];
        cmd = ("" + (cmd || "")).trim();
        if (!cmd)
            delete m[role];
        else
            m[role] = cmd;
        pg.hub.hyprEdit("desktop.apps", m);
    }
    function clearApps() { if (pg.hubReady) pg.hub.hyprEdit("desktop.apps", ({})); }
    function hasApps() { return Object.keys(pg.chosen).length > 0; }

    // ── the app catalogue + picker overlay (shared: roles + custom binds) ────
    // every installed application as { name, cmd } for the filterable picker; the
    // command is the parsed Exec, runnable as a keybind or a ryoku-app role.
    readonly property var appCatalog: {
        var out = [];
        var src = (typeof DesktopEntries !== "undefined" && DesktopEntries.applications)
            ? DesktopEntries.applications.values : [];
        for (var i = 0; i < src.length; i++) {
            var e = src[i];
            if (!e || e.noDisplay)
                continue;
            var cmd = ("" + ((e.command || []).join(" "))).trim();
            if (!cmd)
                continue;
            out.push({ name: e.name || cmd, cmd: cmd });
        }
        out.sort(function (a, b) { return a.name.toLowerCase().localeCompare(b.name.toLowerCase()); });
        return out;
    }
    // the picker targets an app role (by name) or a custom row (by index).
    property string appPickRole: ""
    property int appPickCustomRow: -1
    readonly property bool appPicking: pg.appPickRole.length > 0 || pg.appPickCustomRow >= 0
    function openAppPickerRole(role) { pg.appPickCustomRow = -1; pg.appPickRole = role; }
    function openAppPickerCustom(i) { pg.appPickRole = ""; pg.appPickCustomRow = i; }
    function closeAppPicker() { pg.appPickRole = ""; pg.appPickCustomRow = -1; }
    function appPickCurrent() {
        if (pg.appPickRole.length > 0)
            return pg.chosen[pg.appPickRole] || "";
        if (pg.appPickCustomRow >= 0)
            return (pg.customRows[pg.appPickCustomRow] || ({})).value || "";
        return "";
    }
    function appPickTitle() {
        if (pg.appPickCustomRow >= 0)
            return I18n.tr("Command");
        for (var i = 0; i < pg.roles.length; i++)
            if (pg.roles[i].role === pg.appPickRole)
                return I18n.tr(pg.roles[i].label);
        return I18n.tr("App");
    }
    function applyAppPick(cmd) {
        if (pg.appPickRole.length > 0)
            pg.setApp(pg.appPickRole, cmd);
        else if (pg.appPickCustomRow >= 0)
            pg.patch(pg.appPickCustomRow, "value", cmd);
        pg.closeAppPicker();
    }

    // ── conflict detection (shared with the Import page via Combos.js) ───────
    // catches what a hand-edited config never would: a custom combo that shadows
    // a shipped bind, or duplicates another custom one. The rules live in Combos
    // so both pages classify a clash the same way; these thin readers feed it
    // this page's live legend, custom rows and rebinds.
    function normKeys(s) { return Combos.normKeys(s); }
    readonly property var shippedKeys: Combos.shippedKeys(pg.categories, pg.rebinds)
    function customCount(norm) { return Combos.customCount(pg.customRows, norm); }
    function rowConflict(i) { return Combos.rowConflict(pg.customRows, i, pg.shippedKeys); }
    readonly property int conflictCount: {
        var n = 0;
        for (var i = 0; i < pg.customRows.length; i++)
            if (pg.rowConflict(i) !== "")
                n++;
        for (var c = 0; c < pg.categories.length; c++) {
            var binds = pg.categories[c].binds || [];
            for (var b = 0; b < binds.length; b++) {
                var combo = binds[b].combo || "";
                if (pg.isRebound(combo) && pg.comboConflict(combo))
                    n++;
            }
        }
        return n;
    }

    // the four bind actions, key -> visible label. keys are the exact strings the
    // Go backend switches on, so a rename here silently breaks settings.
    readonly property var actionOpts: [
        { "key": "exec", "label": I18n.tr("Run command") },
        { "key": "close", "label": I18n.tr("Close window") },
        { "key": "fullscreen", "label": I18n.tr("Fullscreen") },
        { "key": "togglefloating", "label": I18n.tr("Toggle floating") }
    ]
    function labelIn(opts, key) {
        for (var i = 0; i < opts.length; i++)
            if (opts[i].key === key)
                return opts[i].label;
        return key;
    }
    function keyIn(opts, label) {
        for (var i = 0; i < opts.length; i++)
            if (opts[i].label === label)
                return opts[i].key;
        return label;
    }
    function labelsOf(opts) {
        return opts.map(function (o) { return o.label; });
    }

    // every mutation hands hyprEdit a fresh slice: the array swap rebinds the
    // Repeater and rebuilds the delegate owning a focused field, so text edits
    // commit on editing-finished, not per keystroke.
    function patch(i, key, val) {
        if (!pg.hubReady)
            return;
        var a = (pg.hub.hyprVal("desktop.keybinds") || []).slice();
        a[i] = Object.assign({}, a[i]);
        a[i][key] = val;
        pg.hub.hyprEdit("desktop.keybinds", a);
    }
    // switching action clears the value: only "exec" carries a command.
    function setAction(i, key) {
        if (!pg.hubReady)
            return;
        var a = (pg.hub.hyprVal("desktop.keybinds") || []).slice();
        a[i] = Object.assign({}, a[i]);
        a[i].action = key;
        if (key !== "exec")
            a[i].value = "";
        pg.hub.hyprEdit("desktop.keybinds", a);
    }
    function addRow() {
        if (!pg.hubReady)
            return;
        var a = (pg.hub.hyprVal("desktop.keybinds") || []).slice();
        a.push({ "keys": "", "action": "exec", "value": "", "release": false });
        pg.hub.hyprEdit("desktop.keybinds", a);
    }
    function removeRow(i) {
        if (!pg.hubReady)
            return;
        var a = (pg.hub.hyprVal("desktop.keybinds") || []).slice();
        a.splice(i, 1);
        pg.hub.hyprEdit("desktop.keybinds", a);
    }
    function clearAll() {
        if (pg.hubReady)
            pg.hub.hyprEdit("desktop.keybinds", []);
    }
    function saveAll() {
        if (pg.hubReady)
            pg.hub.save();
    }
    function revertAll() {
        if (pg.hubReady)
            pg.hub.revert();
    }

    // ── chord recorder ──────────────────────────────────────────────────────
    // The star of this page: instead of typing "SUPER + J", the user clicks
    // record and presses the combo. Capture is compositor-neutral -- the page
    // ShortcutInhibitor grabs the compositor's shortcuts while recording, so a
    // live chord like SUPER + Q reaches the Hub instead of firing -- and on
    // Hyprland a do-nothing record submap backs it up. recordRow is the row being
    // recorded (-1 = idle); the overlay below drives it.
    property int recordRow: -1
    // recordCombo holds a shipped bind's default combo while it is being rebound
    // (recordRow stays -1); either one active means the recorder overlay is up.
    property string recordCombo: ""
    // the chord as it forms in the overlay: the held modifiers, before the main
    // key lands and commits.
    property string recordForming: ""
    // recordFamily marks a family rebind in progress: the overlay asks for a
    // number key and stores the {n} form rather than a single chord.
    // recordNeedNumber flashes a nudge when a non-digit key lands mid-record.
    property bool recordFamily: false
    property bool recordNeedNumber: false
    readonly property bool recording: pg.recordRow >= 0 || pg.recordCombo.length > 0

    // Belt and braces on Hyprland: it honours a do-nothing submap so a live chord
    // passes through even before the inhibitor's grant lands. niri has no submap,
    // so the ShortcutInhibitor is the whole story there; gate on the capability
    // rather than the compositor name.
    function enterRecordSubmap() { if (pg.hub && Settings.supports("submap")) pg.hub.wmAct("submap.enter", ["record"]); }
    function exitRecordSubmap() { if (pg.hub && Settings.supports("submap")) pg.hub.wmAct("submap.reset"); }

    function startRecord(i) {
        if (!pg.hubReady || i < 0)
            return;
        pg.recordCombo = "";
        pg.recordForming = "";
        pg.recordFamily = false;
        pg.recordNeedNumber = false;
        pg.recordRow = i;
        pg.enterRecordSubmap();
        recordTimeout.restart();
    }
    // record a new chord for a shipped bind, keyed by its default combo. A family
    // default ("SUPER + {n}") records a number key and stores the {n} form.
    function startRecordShipped(combo) {
        if (!pg.hubReady || !combo)
            return;
        pg.recordRow = -1;
        pg.recordForming = "";
        pg.recordNeedNumber = false;
        pg.recordFamily = Combos.isFamilyChord(combo);
        pg.recordCombo = combo;
        pg.enterRecordSubmap();
        recordTimeout.restart();
    }
    // commit true writes the captured chord to the custom row or the rebind; false cancels.
    function stopRecord(commit, chord) {
        recordTimeout.stop();
        pg.exitRecordSubmap();
        var i = pg.recordRow;
        var combo = pg.recordCombo;
        pg.recordRow = -1;
        pg.recordCombo = "";
        pg.recordForming = "";
        pg.recordFamily = false;
        pg.recordNeedNumber = false;
        if (!commit || !chord)
            return;
        if (combo.length > 0)
            pg.setRebind(combo, chord);
        else if (i >= 0)
            pg.patch(i, "keys", chord);
    }

    // never leave the keyboard stranded in the record submap: a hard ceiling
    // exits it even if every other path fails.
    Timer {
        id: recordTimeout
        interval: 15000
        onTriggered: pg.stopRecord(false, "")
    }

    // the recorder's key reader lives in Combos.js, shared with the Import
    // page so a captured chord reads into the store's token the same way.
    function chordFrom(event) { return Combos.chordFrom(event); }
    // the shipped bind a normalised chord shadows, by description, for the
    // custom editor's conflict note. "" when nothing shipped matches.
    function shippedDescFor(norm) {
        for (var c = 0; c < pg.categories.length; c++) {
            var binds = pg.categories[c].binds || [];
            for (var b = 0; b < binds.length; b++) {
                if (pg.normKeys((binds[b].keys || []).join(" + ")) === norm)
                    return binds[b].desc || I18n.tr("a shipped shortcut");
            }
        }
        return "";
    }

    // ── shipped-bind rebinds ────────────────────────────────────────────────
    // A rebind remaps a shipped chord to a user-chosen one, held as
    // { "SUPER + Q": "SUPER + X" } in the draft under desktop.keybindRebinds. The
    // shipped default combo is the stable id.
    readonly property var rebinds: pg.hubReady ? (pg.hub.hyprVal("desktop.keybindRebinds") || ({})) : ({})
    function effectiveCombo(defCombo) {
        var r = pg.rebinds[defCombo];
        return (r && r.length) ? r : defCombo;
    }
    function isRebound(defCombo) {
        var r = pg.rebinds[defCombo];
        return r !== undefined && r !== null && r.length > 0 && r !== defCombo;
    }
    function hasRebinds() {
        for (var k in pg.rebinds)
            if (pg.rebinds[k] && pg.rebinds[k] !== k)
                return true;
        return false;
    }
    function setRebind(defCombo, chord) {
        if (!pg.hubReady)
            return;
        var cur = pg.hub.hyprVal("desktop.keybindRebinds") || {};
        var m = {};
        for (var k in cur)
            m[k] = cur[k];
        if (!chord || chord === defCombo)
            delete m[defCombo];
        else
            m[defCombo] = chord;
        pg.hub.hyprEdit("desktop.keybindRebinds", m);
    }
    function clearRebind(defCombo) { pg.setRebind(defCombo, ""); }
    function clearRebinds() {
        if (pg.hubReady)
            pg.hub.hyprEdit("desktop.keybindRebinds", ({}));
    }

    // norm -> how many shipped (effective) + custom binds hold it; >1 is a clash.
    readonly property var effectiveCounts: {
        var m = {};
        for (var c = 0; c < pg.categories.length; c++) {
            var binds = pg.categories[c].binds || [];
            for (var b = 0; b < binds.length; b++) {
                // a family stands for ten chords, so expand it before counting:
                // that is how a rebound family clashes with another effective chord.
                var chords = Combos.expandFamily(pg.effectiveCombo(binds[b].combo || ""));
                for (var e = 0; e < chords.length; e++) {
                    var n = pg.normKeys(chords[e]);
                    if (n.length)
                        m[n] = (m[n] || 0) + 1;
                }
            }
        }
        for (var i = 0; i < pg.customRows.length; i++) {
            var cn = pg.normKeys(pg.customRows[i].keys);
            if (cn.length)
                m[cn] = (m[cn] || 0) + 1;
        }
        return m;
    }
    // a family compares its ten expanded chords against the other effective
    // chords; a plain chord expands to itself, so both go through one path.
    function comboConflict(defCombo) {
        var chords = Combos.expandFamily(pg.effectiveCombo(defCombo));
        for (var i = 0; i < chords.length; i++) {
            var n = pg.normKeys(chords[i]);
            if (n.length > 0 && pg.effectiveCounts[n] > 1)
                return true;
        }
        return false;
    }

    // ── display tokens for a raw combo, matching the legend keycaps ──────────
    // mirrors wm.DisplayKeys (ryoku/wm/binds.go) for the chords the recorder can
    // produce, so a row rebound in the draft (before the provider recomputes its
    // keys on save) reads exactly like a shipped one. The map is the same set as
    // the backend's displayNames; anything unlisted passes through, except a
    // number-pad digit which reads "Num 3".
    readonly property var capNames: ({
        "SUPER": "Super", "SHIFT": "Shift", "CTRL": "Ctrl", "ALT": "Alt",
        "Return": "Enter", "Escape": "Esc", "space": "Space",
        "Prior": "Page Up", "Next": "Page Down",
        "grave": "\u0060", "comma": ",", "bracketleft": "[", "bracketright": "]",
        "mouse:272": "LMB", "mouse:273": "RMB",
        "mouse_up": "Scroll Up", "mouse_down": "Scroll Down",
        "XF86AudioRaiseVolume": "Vol +", "XF86AudioLowerVolume": "Vol -",
        "XF86AudioMute": "Mute", "XF86AudioPlay": "Play",
        "XF86AudioNext": "Next", "XF86AudioPrev": "Prev",
        "XF86MonBrightnessUp": "Brightness +", "XF86MonBrightnessDown": "Brightness -",
        "XF86TouchpadToggle": "Touchpad", "XF86TouchpadOn": "Touchpad On", "XF86TouchpadOff": "Touchpad Off"
    })
    function capToken(tok) {
        if (pg.capNames[tok] !== undefined)
            return pg.capNames[tok];
        // a family placeholder reads as its digit range, mirroring wm.DisplayKeys,
        // so a rebound family's caps read like a shipped one ("1 … 0" / "Num 1 … 0").
        if (tok === "{n}")
            return "1 \u2026 0";
        if (tok === "KP_{n}")
            return "Num 1 \u2026 0";
        // a number-pad digit reads "Num 3", mirroring wm.DisplayKeys.
        if (tok.length === 4 && tok.substring(0, 3) === "KP_") {
            var d = tok.charAt(3);
            if (d >= "0" && d <= "9")
                return "Num " + d;
        }
        return tok;
    }
    function comboToCaps(raw) {
        var parts = ("" + raw).split("+");
        var out = [];
        for (var i = 0; i < parts.length; i++) {
            var t = parts[i].trim();
            if (t.length)
                out.push(pg.capToken(t));
        }
        return out;
    }
    // the custom-row index a legend row maps to, matched by its chord, so a
    // custom row's edit can drop into the Custom tab and record that same row.
    // -1 when it is not in the draft yet (only in the saved legend).
    function customIndexFor(combo) {
        var norm = pg.normKeys(combo);
        if (!norm)
            return -1;
        for (var i = 0; i < pg.customRows.length; i++)
            if (pg.normKeys(pg.customRows[i].keys) === norm)
                return i;
        return -1;
    }
    // a chord as one readable line ("Super + Q", "Super + 1 … 0"), for the reset
    // button's "Back to ..." tooltip. Uses the same caps faces the row shows.
    function comboLabel(raw) {
        return pg.comboToCaps(raw).join(" + ");
    }

    // ── legend search + rail ─────────────────────────────────────────────────
    // The search field (in the head) is the source of truth for the query; the
    // rail clears it through clearSearch so tapping a category leaves search.
    readonly property string query: searchInput.text
    readonly property bool searching: pg.query.trim().length > 0
    // "" = All; else a category name. Ignored while searching (search spans all).
    property string selectedCat: ""
    function clearSearch() { searchInput.text = ""; }

    // a family row (its combo carries {n}) matches its whole digit range, so
    // "num 3" and "workspace 3" find the number-pad family without the legend
    // having to expand ten rows. The digits carry a "Num " face for a KP_{n}
    // family so "num" narrows to the number pad.
    function familyExpand(bind) {
        var combo = bind.combo || "";
        if (combo.indexOf("{n}") < 0)
            return "";
        var kp = combo.indexOf("KP_{n}") >= 0;
        var out = [];
        for (var n = 1; n <= 10; n++)
            out.push((kp ? "Num " : "") + (n % 10));
        return out.join(" ");
    }
    // fuzzy score for a bind against the query, over one haystack of its label,
    // hint, category and chord tokens (a family adds its digit range, so "num 3"
    // and "workspace 3" reach the number-pad family). >= 0 is a match; -1 none.
    function bindScore(bind, q) {
        if (!q)
            return 0;
        var keyStr = (bind.keys ? bind.keys.join(" ") : "");
        var fam = pg.familyExpand(bind);
        if (fam.length)
            keyStr += " " + fam;
        var hay = (bind.desc || "") + " " + (bind.hint || "") + " " + (bind.category || "") + " " + keyStr;
        return Fuzzy.score(q, hay);
    }
    // matched (or total, when idle) binds in a category, for the rail counts.
    function catCount(cat) {
        var binds = cat.binds || [];
        if (!pg.searching)
            return binds.length;
        var q = pg.query.trim();
        var n = 0;
        for (var b = 0; b < binds.length; b++)
            if (pg.bindScore(binds[b], q) >= 0)
                n++;
        return n;
    }
    readonly property int allCount: {
        var total = 0;
        for (var i = 0; i < pg.categories.length; i++)
            total += pg.catCount(pg.categories[i]);
        return total;
    }
    // the rail model: All, then every category with its count.
    readonly property var railModel: {
        var out = [{ name: "", label: I18n.tr("All"), count: pg.allCount }];
        for (var i = 0; i < pg.categories.length; i++) {
            var c = pg.categories[i];
            out.push({ name: c.name, label: c.name, count: pg.catCount(c) });
        }
        return out;
    }
    // what the content column shows: filtered categories while searching, else
    // the selected category (All -> every category).
    readonly property var shownCats: {
        if (pg.searching) {
            var q = pg.query.trim();
            var out = [];
            for (var i = 0; i < pg.categories.length; i++) {
                var c = pg.categories[i];
                var binds = c.binds || [];
                var keep = [];
                for (var b = 0; b < binds.length; b++)
                    if (pg.bindScore(binds[b], q) >= 0)
                        keep.push(binds[b]);
                if (keep.length)
                    out.push({ name: c.name, binds: keep });
            }
            return out;
        }
        if (pg.selectedCat === "")
            return pg.categories;
        for (var j = 0; j < pg.categories.length; j++)
            if (pg.categories[j].name === pg.selectedCat)
                return [pg.categories[j]];
        return [];
    }

    // ── the keycap cluster, once ──────────────────────────────────────────────
    // hairline mono caps with a "+" between them. muted dims an unhonoured row's
    // caps to half; hot strengthens the border on a hovered rebindable row; clash
    // inks the border when a rebound chord collides (colour-free, DESIGN.md 1).
    component KeyCaps: Row {
        id: kc
        property var tokens: []
        property bool muted: false
        property bool rebound: false
        property bool clash: false
        property bool hot: false
        spacing: Tokens.s1
        opacity: kc.muted ? 0.45 : 1

        Repeater {
            model: kc.tokens
            delegate: Row {
                id: kcCap
                required property var modelData
                required property int index
                spacing: Tokens.s1
                Text {
                    visible: kcCap.index > 0
                    height: 22; verticalAlignment: Text.AlignVCenter
                    text: "+"; color: Tokens.inkFaint
                    font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                }
                Rectangle {
                    anchors.verticalCenter: parent.verticalCenter
                    implicitHeight: 22
                    implicitWidth: Math.max(22, capT.implicitWidth + 18)
                    radius: Tokens.radius
                    color: "transparent"
                    border.width: Tokens.border
                    border.color: kc.clash ? Tokens.ink : ((kc.hot || kc.rebound) ? Tokens.lineStrong : Tokens.line)
                    Behavior on border.color { ColorAnimation { duration: Tokens.snap } }
                    Text {
                        id: capT
                        anchors.centerIn: parent
                        text: kcCap.modelData
                        color: kc.rebound ? Tokens.inkDim : Tokens.inkFaint
                        font.family: Tokens.mono; font.pixelSize: Tokens.fSmall
                    }
                }
            }
        }
    }

    // a small drawn padlock for a row bound to a dedicated key it cannot rebind
    // (mouse, media, hardware). Drawn, not a font glyph, so it stays flat ink
    // like the rest of the sheet rather than risking a colour emoji fallback.
    component LockMark: Item {
        id: lk
        property color tint: Tokens.inkFaint
        implicitWidth: 12
        implicitHeight: 14
        Shape {
            x: 0; y: 0
            width: 12; height: 8
            preferredRendererType: Shape.CurveRenderer
            antialiasing: true
            ShapePath {
                strokeColor: lk.tint
                strokeWidth: 1.4
                fillColor: "transparent"
                capStyle: ShapePath.RoundCap
                joinStyle: ShapePath.RoundJoin
                PathSvg { path: "M2.4 8 V4.2 A3.6 3.6 0 0 1 9.6 4.2 V8" }
            }
        }
        Rectangle {
            x: 0.5; y: 6
            width: 11; height: 8
            radius: 1
            color: lk.tint
        }
    }

    // ── head: eyebrow, Fraunces title, blurb (matches every settings page) ──
    Column {
        id: head
        anchors.top: parent.top
        anchors.topMargin: Tokens.s6
        // the head sits on the body's grid, so the title starts over the first
        // card column instead of floating in the middle of a page-wide window
        x: Tokens.s6
        width: Math.max(320, pg.width - Tokens.s6 * 2 - Tokens.s3)
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
                text: I18n.tr("APPS & KEYS"); color: Tokens.inkMuted; font.family: Tokens.ui
                font.pixelSize: 9; font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                anchors.verticalCenter: parent.verticalCenter
            }
        }
        Text {
            text: I18n.tr("Keybinds"); color: Tokens.ink
            font.family: Tokens.display; font.pixelSize: Tokens.fTitle
        }
        Text {
            width: Math.min(parent.width, 720)
            text: I18n.tr("Every desktop shortcut in one place. Search, rebind, or add your own.")
            color: Tokens.inkMuted; font.family: Tokens.ui
            font.pixelSize: Tokens.fBody; wrapMode: Text.WordWrap
        }
    }

    // ── tab switch: Shortcuts (legend) | Apps | Custom ──
    Tabs {
        id: tabs
        anchors.left: parent.left
        anchors.leftMargin: Tokens.s6
        anchors.top: head.bottom
        anchors.topMargin: Tokens.s5
        options: [I18n.tr("Shortcuts"), I18n.tr("Apps"), I18n.tr("Custom")]
        current: pg.tab === "apps" ? I18n.tr("Apps") : (pg.tab === "custom" ? I18n.tr("Custom") : I18n.tr("Shortcuts"))
        onChose: (label) => pg.tab = (label === I18n.tr("Apps") ? "apps" : (label === I18n.tr("Custom") ? "custom" : "shortcuts"))
    }

    // ── search: full-width fuzzy finder over the legend (Shortcuts only) ──
    Rectangle {
        id: searchBar
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.leftMargin: Tokens.s6
        anchors.rightMargin: Tokens.s6
        anchors.top: tabs.bottom
        anchors.topMargin: Tokens.s4
        height: 44
        visible: pg.tab === "shortcuts"
        radius: Tokens.radius
        color: Tokens.paper
        border.width: Tokens.border
        border.color: searchInput.activeFocus ? Tokens.lineStrong : Tokens.line
        Behavior on border.color { ColorAnimation { duration: Tokens.snap } }

        Text {
            id: searchGlyph
            anchors.left: parent.left
            anchors.leftMargin: Tokens.s4
            anchors.verticalCenter: parent.verticalCenter
            text: "\uf002"
            color: searchInput.activeFocus ? Tokens.inkDim : Tokens.inkFaint
            font.family: Tokens.mono; font.pixelSize: Tokens.fSmall
            Behavior on color { ColorAnimation { duration: Tokens.snap } }
        }
        TextInput {
            id: searchInput
            anchors.left: searchGlyph.right
            anchors.leftMargin: Tokens.s3
            anchors.right: matchCount.left
            anchors.rightMargin: Tokens.s3
            anchors.verticalCenter: parent.verticalCenter
            clip: true
            color: Tokens.ink
            font.family: Tokens.ui; font.pixelSize: Tokens.fBody
            selectionColor: Tokens.lineStrong
            selectedTextColor: Tokens.ink
            // Esc clears the query (the field never closes a page).
            Keys.onEscapePressed: searchInput.text = ""
        }
        Text {
            anchors.left: searchInput.left
            anchors.verticalCenter: searchInput.verticalCenter
            visible: searchInput.text.length === 0
            text: I18n.tr("Search shortcuts")
            color: Tokens.inkFaint
            font: searchInput.font
        }
        // match count at the field's right edge, shown while searching.
        Text {
            id: matchCount
            anchors.right: parent.right
            anchors.rightMargin: Tokens.s4
            anchors.verticalCenter: parent.verticalCenter
            visible: pg.searching
            text: pg.allCount === 1 ? I18n.tr("%1 match").arg(pg.allCount) : I18n.tr("%1 matches").arg(pg.allCount)
            color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny
        }
    }

    // ── the tab body: a Loader swaps the whole subtree, fading the new one in ──
    Loader {
        id: loader
        anchors {
            left: parent.left; right: parent.right
            top: pg.tab === "shortcuts" ? searchBar.bottom : tabs.bottom
            bottom: bar.top
            leftMargin: Tokens.s6; rightMargin: Tokens.s6
            topMargin: Tokens.s4; bottomMargin: Tokens.s6
        }
        sourceComponent: pg.tab === "apps" ? appsComp : (pg.tab === "custom" ? customComp : shortcutsComp)
        onLoaded: {
            if (!item)
                return;
            item.opacity = 0;
            fade.restart();
        }
    }
    // content exchange -> swap token; nothing travels, so a plain fade is right.
    NumberAnimation {
        id: fade
        target: loader.item; property: "opacity"; to: 1
        duration: Tokens.swap; easing.type: Tokens.ease
    }

    // ── the app-role launchers (Apps): pick the app + rebind its key ─────────
    Component {
        id: appsComp

        Flickable {
            id: appsFlick
            contentWidth: width
            contentHeight: appsCol.height
            clip: true
            boundsBehavior: Flickable.StopAtBounds
            ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
            WheelScroll { }

            Column {
                id: appsCol
                width: appsFlick.width - Tokens.s3
                spacing: Tokens.s5

                // These are launcher shortcuts, not the system's file and link
                // handlers: say so up top so nobody hunts for xdg defaults here.
                SettingCard {
                    id: appsNotice
                    width: appsCol.width
                    collapsible: false
                    title: I18n.tr("Shortcuts, not default apps")

                    Text {
                        width: parent.width
                        leftPadding: Tokens.s4; rightPadding: Tokens.s4
                        topPadding: Tokens.s3; bottomPadding: Tokens.s3
                        wrapMode: Text.WordWrap
                        text: I18n.tr("These are the apps the launcher keys open (Super + Return, Super + B and the rest). They are not the system's default applications for opening files and links.")
                        color: Tokens.inkMuted; font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall; lineHeight: 1.3
                    }
                }

                Text {
                    width: appsCol.width
                    wrapMode: Text.WordWrap
                    text: I18n.tr("Pick what each launcher key opens, and rebind the key itself. The key runs the app through ryoku-app, so a swap takes effect on the next press, with no reload.")
                    color: Tokens.inkMuted; font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall; lineHeight: 1.3
                }

                Repeater {
                    model: pg.roles

                    delegate: Column {
                        id: roleCard
                        required property var modelData
                        width: appsCol.width
                        spacing: Tokens.s2
                        readonly property string role: roleCard.modelData.role
                        readonly property string fallback: roleCard.modelData.fallback || ""
                        readonly property string combo: roleCard.modelData.combo || ""
                        readonly property string eff: pg.effOf(roleCard.role, roleCard.fallback)
                        readonly property bool rebound: pg.isRebound(roleCard.combo)
                        readonly property bool clash: roleCard.rebound && pg.comboConflict(roleCard.combo)
                        readonly property var effKeys: pg.comboToCaps(pg.effectiveCombo(roleCard.combo))

                        // line 1: role label + current command, then the key + rebind cluster.
                        Item {
                            width: parent.width
                            height: 24
                            Row {
                                anchors.left: parent.left
                                anchors.verticalCenter: parent.verticalCenter
                                spacing: Tokens.s2
                                Text {
                                    text: I18n.tr(roleCard.modelData.label)
                                    color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                                    font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                                    font.capitalization: Font.AllUppercase
                                    anchors.verticalCenter: parent.verticalCenter
                                }
                                Text {
                                    anchors.verticalCenter: parent.verticalCenter
                                    width: Math.min(implicitWidth, roleCard.width * 0.32)
                                    text: roleCard.eff !== "" ? roleCard.eff : I18n.tr("not set")
                                    color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny
                                    elide: Text.ElideRight
                                }
                            }

                            Row {
                                anchors.right: parent.right
                                anchors.verticalCenter: parent.verticalCenter
                                spacing: Tokens.s2
                                visible: roleCard.combo.length > 0

                                Row {
                                    anchors.verticalCenter: parent.verticalCenter
                                    spacing: Tokens.s1
                                    Repeater {
                                        model: roleCard.effKeys
                                        delegate: Row {
                                            id: capW
                                            required property var modelData
                                            required property int index
                                            spacing: Tokens.s1
                                            Text {
                                                visible: capW.index > 0
                                                height: 22; verticalAlignment: Text.AlignVCenter
                                                text: "+"; color: Tokens.inkFaint
                                                font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                                            }
                                            Rectangle {
                                                anchors.verticalCenter: parent.verticalCenter
                                                implicitHeight: 22
                                                implicitWidth: Math.max(22, cl.implicitWidth + 14)
                                                radius: Tokens.radius; color: "transparent"
                                                border.width: Tokens.border
                                                border.color: roleCard.clash ? Tokens.ink : (roleCard.rebound ? Tokens.lineStrong : Tokens.line)
                                                Text {
                                                    id: cl
                                                    anchors.centerIn: parent
                                                    text: capW.modelData
                                                    color: roleCard.rebound ? Tokens.inkDim : Tokens.inkFaint
                                                    font.family: Tokens.mono; font.pixelSize: Tokens.fMicro
                                                }
                                            }
                                        }
                                    }
                                }
                                IconBtn {
                                    anchors.verticalCenter: parent.verticalCenter
                                    visible: roleCard.rebound
                                    glyph: "\u21ba"
                                    onAct: pg.clearRebind(roleCard.combo)
                                }
                                IconBtn {
                                    anchors.verticalCenter: parent.verticalCenter
                                    visible: pg.hubReady
                                    glyph: "\u25cf"
                                    onAct: pg.startRecordShipped(roleCard.combo)
                                }
                            }
                        }

                        // line 2: installed candidate chips + Browse (the app catalogue).
                        Flow {
                            width: parent.width
                            spacing: Tokens.s2

                            Repeater {
                                model: roleCard.modelData.candidates || []
                                delegate: Rectangle {
                                    id: chip
                                    required property var modelData
                                    visible: chip.modelData.installed === true
                                    readonly property bool sel: roleCard.eff === chip.modelData.cmd
                                    implicitWidth: chLbl.implicitWidth + Tokens.s4
                                    implicitHeight: 28
                                    radius: Tokens.radius
                                    color: chip.sel ? Tokens.ink : (chHov.hovered ? Tokens.tint10 : "transparent")
                                    border.width: Tokens.border
                                    border.color: chip.sel ? Tokens.ink : (chHov.hovered ? Tokens.lineStrong : Tokens.line)
                                    Behavior on color { ColorAnimation { duration: Tokens.snap } }
                                    Text {
                                        id: chLbl
                                        anchors.centerIn: parent
                                        text: I18n.tr(chip.modelData.label)
                                        color: chip.sel ? Tokens.paper : Tokens.inkDim
                                        font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                                    }
                                    HoverHandler { id: chHov; cursorShape: Qt.PointingHandCursor }
                                    TapHandler { onTapped: pg.setApp(roleCard.role, chip.modelData.cmd) }
                                }
                            }

                            Rectangle {
                                implicitWidth: brLbl.implicitWidth + Tokens.s4
                                implicitHeight: 28
                                radius: Tokens.radius
                                color: brHov.hovered ? Tokens.tint10 : "transparent"
                                border.width: Tokens.border
                                border.color: brHov.hovered ? Tokens.lineStrong : Tokens.line
                                Text {
                                    id: brLbl
                                    anchors.centerIn: parent
                                    text: I18n.tr("Browse\u2026")
                                    color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                                }
                                HoverHandler { id: brHov; cursorShape: Qt.PointingHandCursor }
                                TapHandler { onTapped: pg.openAppPickerRole(roleCard.role) }
                            }
                        }

                        Column {
                            width: parent.width
                            spacing: Tokens.s2
                            visible: roleCard.role === "terminal"

                            Row {
                                spacing: Tokens.s2
                                Text {
                                    text: I18n.tr("SHELL")
                                    color: Tokens.inkMuted
                                    font.family: Tokens.ui
                                    font.pixelSize: Tokens.fMicro
                                    font.weight: Font.Medium
                                    font.letterSpacing: Tokens.trackMark
                                    anchors.verticalCenter: parent.verticalCenter
                                }
                                Text {
                                    text: I18n.tr("Account-wide; new terminals, TTYs and SSH sessions use this shell.")
                                    color: Tokens.inkFaint
                                    font.family: Tokens.ui
                                    font.pixelSize: Tokens.fTiny
                                    anchors.verticalCenter: parent.verticalCenter
                                }
                            }

                            Flow {
                                width: parent.width
                                spacing: Tokens.s2
                                Repeater {
                                    model: pg.shellState.choices || []
                                    Rectangle {
                                        id: shellChip
                                        required property var modelData
                                        readonly property bool selected: pg.shellState.current === shellChip.modelData.key
                                        implicitWidth: shellLabel.implicitWidth + Tokens.s4
                                        implicitHeight: 28
                                        radius: Tokens.radius
                                        opacity: shellChip.modelData.installed && !pg.shellBusy ? 1 : 0.35
                                        color: shellChip.selected ? Tokens.ink : (shellHover.hovered ? Tokens.tint10 : "transparent")
                                        border.width: Tokens.border
                                        border.color: shellChip.selected ? Tokens.ink : Tokens.line
                                        Text {
                                            id: shellLabel
                                            anchors.centerIn: parent
                                            text: I18n.tr(shellChip.modelData.label)
                                            color: shellChip.selected ? Tokens.paper : Tokens.inkDim
                                            font.family: Tokens.ui
                                            font.pixelSize: Tokens.fSmall
                                        }
                                        HoverHandler {
                                            id: shellHover
                                            enabled: shellChip.modelData.installed && !pg.shellBusy
                                            cursorShape: enabled ? Qt.PointingHandCursor : Qt.ArrowCursor
                                        }
                                        TapHandler {
                                            enabled: shellChip.modelData.installed && !pg.shellBusy
                                            onTapped: pg.setShell(shellChip.modelData.key)
                                        }
                                    }
                                }
                            }

                            Row {
                                spacing: Tokens.s2
                                Text {
                                    text: I18n.tr("ZSH PROMPT")
                                    color: Tokens.inkMuted
                                    font.family: Tokens.ui
                                    font.pixelSize: Tokens.fMicro
                                    font.weight: Font.Medium
                                    font.letterSpacing: Tokens.trackMark
                                    anchors.verticalCenter: parent.verticalCenter
                                }
                                Text {
                                    text: I18n.tr("Used when Zsh is selected.")
                                    color: Tokens.inkFaint
                                    font.family: Tokens.ui
                                    font.pixelSize: Tokens.fTiny
                                    anchors.verticalCenter: parent.verticalCenter
                                }
                            }

                            Flow {
                                width: parent.width
                                spacing: Tokens.s2
                                Repeater {
                                    model: [
                                        { "mode": "starship", "label": I18n.tr("Starship") },
                                        { "mode": "oh-my-zsh", "label": I18n.tr("Oh My Zsh") }
                                    ]
                                    delegate: Rectangle {
                                        id: promptChip
                                        required property var modelData
                                        readonly property bool selected: pg.shellState.zshPrompt === promptChip.modelData.mode
                                        implicitWidth: promptLabel.implicitWidth + Tokens.s4
                                        implicitHeight: 28
                                        radius: Tokens.radius
                                        opacity: pg.shellBusy ? 0.35 : 1
                                        color: promptChip.selected ? Tokens.ink : (promptHover.hovered ? Tokens.tint10 : "transparent")
                                        border.width: Tokens.border
                                        border.color: promptChip.selected ? Tokens.ink : Tokens.line
                                        Text {
                                            id: promptLabel
                                            anchors.centerIn: parent
                                            text: I18n.tr(promptChip.modelData.label)
                                            color: promptChip.selected ? Tokens.paper : Tokens.inkDim
                                            font.family: Tokens.ui
                                            font.pixelSize: Tokens.fSmall
                                        }
                                        HoverHandler {
                                            id: promptHover
                                            enabled: !pg.shellBusy
                                            cursorShape: enabled ? Qt.PointingHandCursor : Qt.ArrowCursor
                                        }
                                        TapHandler {
                                            enabled: !pg.shellBusy
                                            onTapped: pg.setZshPrompt(promptChip.modelData.mode)
                                        }
                                    }
                                }
                            }

                            Row {
                                spacing: Tokens.s2
                                Text {
                                    text: I18n.tr("STARSHIP LAYOUT")
                                    color: Tokens.inkMuted
                                    font.family: Tokens.ui
                                    font.pixelSize: Tokens.fMicro
                                    font.weight: Font.Medium
                                    font.letterSpacing: Tokens.trackMark
                                    anchors.verticalCenter: parent.verticalCenter
                                }
                                Text {
                                    text: I18n.tr("Shared by Fish, Bash and Starship-powered Zsh.")
                                    color: Tokens.inkFaint
                                    font.family: Tokens.ui
                                    font.pixelSize: Tokens.fTiny
                                    anchors.verticalCenter: parent.verticalCenter
                                }
                            }

                            Flow {
                                width: parent.width
                                spacing: Tokens.s2
                                Repeater {
                                    model: pg.orderedStarshipLayouts()
                                    delegate: Rectangle {
                                        id: layoutCard
                                        required property var modelData
                                        readonly property bool selected: pg.starshipState.layout === layoutCard.modelData.id
                                        width: 176
                                        height: 82
                                        radius: Tokens.radius
                                        opacity: pg.starshipBusy ? 0.45 : 1
                                        color: layoutCard.selected ? Tokens.ink : (layoutHover.hovered ? Tokens.tint10 : "transparent")
                                        border.width: Tokens.border
                                        border.color: layoutCard.selected ? Tokens.ink : (layoutHover.hovered ? Tokens.lineStrong : Tokens.line)
                                        Behavior on color { ColorAnimation { duration: Tokens.snap } }
                                        Text {
                                            anchors.left: parent.left
                                            anchors.top: parent.top
                                            anchors.margins: Tokens.s2
                                            text: pg.starshipName(layoutCard.modelData.id)
                                            color: layoutCard.selected ? Tokens.paper : Tokens.inkDim
                                            font.family: Tokens.ui
                                            font.pixelSize: Tokens.fSmall
                                            font.weight: Font.Medium
                                        }
                                        Text {
                                            anchors.left: parent.left
                                            anchors.right: parent.right
                                            anchors.bottom: parent.bottom
                                            anchors.margins: Tokens.s2
                                            height: 42
                                            text: pg.starshipPreview(layoutCard.modelData.id)
                                            color: layoutCard.selected ? Tokens.paper : Tokens.inkFaint
                                            font.family: Tokens.mono
                                            font.pixelSize: Tokens.fTiny
                                            wrapMode: Text.Wrap
                                            verticalAlignment: Text.AlignVCenter
                                        }
                                        HoverHandler {
                                            id: layoutHover
                                            enabled: !pg.starshipBusy
                                            cursorShape: enabled ? Qt.PointingHandCursor : Qt.ArrowCursor
                                        }
                                        TapHandler {
                                            enabled: !pg.starshipBusy
                                            onTapped: pg.setStarshipLayout(layoutCard.modelData.id)
                                        }
                                    }
                                }
                            }

                            Row {
                                spacing: Tokens.s2
                                Text {
                                    text: I18n.tr("PROMPT PALETTE")
                                    color: Tokens.inkMuted
                                    font.family: Tokens.ui
                                    font.pixelSize: Tokens.fMicro
                                    font.weight: Font.Medium
                                    font.letterSpacing: Tokens.trackMark
                                    anchors.verticalCenter: parent.verticalCenter
                                }
                                Repeater {
                                    model: [
                                        { "mode": "fixed", "label": I18n.tr("Fixed") },
                                        { "mode": "wallpaper", "label": I18n.tr("Follow wallpaper") }
                                    ]
                                    delegate: Rectangle {
                                        id: starshipPaletteChip
                                        required property var modelData
                                        readonly property bool selected: pg.starshipState.palette === starshipPaletteChip.modelData.mode
                                        implicitWidth: paletteLabel.implicitWidth + Tokens.s4
                                        implicitHeight: 28
                                        radius: Tokens.radius
                                        opacity: pg.starshipBusy ? 0.45 : 1
                                        color: starshipPaletteChip.selected ? Tokens.ink : (paletteHover.hovered ? Tokens.tint10 : "transparent")
                                        border.width: Tokens.border
                                        border.color: starshipPaletteChip.selected ? Tokens.ink : Tokens.line
                                        Text {
                                            id: paletteLabel
                                            anchors.centerIn: parent
                                            text: starshipPaletteChip.modelData.label
                                            color: starshipPaletteChip.selected ? Tokens.paper : Tokens.inkDim
                                            font.family: Tokens.ui
                                            font.pixelSize: Tokens.fSmall
                                        }
                                        HoverHandler {
                                            id: paletteHover
                                            enabled: !pg.starshipBusy
                                            cursorShape: enabled ? Qt.PointingHandCursor : Qt.ArrowCursor
                                        }
                                        TapHandler {
                                            enabled: !pg.starshipBusy
                                            onTapped: pg.setStarshipPalette(starshipPaletteChip.modelData.mode)
                                        }
                                    }
                                }
                            }

                            Text {
                                width: parent.width
                                text: I18n.tr("Custom: ~/.config/fish/user.fish · ~/.config/bash/user.bash · ~/.config/zsh/user.zsh · ~/.config/zsh/oh-my-zsh/")
                                color: Tokens.inkFaint
                                font.family: Tokens.mono
                                font.pixelSize: Tokens.fTiny
                                wrapMode: Text.Wrap
                            }
                            Text {
                                visible: pg.shellError !== ""
                                text: pg.shellError
                                color: Tokens.alert
                                font.family: Tokens.ui
                                font.pixelSize: Tokens.fTiny
                            }
                            Text {
                                visible: pg.starshipError !== ""
                                text: pg.starshipError
                                color: Tokens.alert
                                font.family: Tokens.ui
                                font.pixelSize: Tokens.fTiny
                            }
                        }

                        Rectangle { width: parent.width; height: 1; color: Tokens.lineSoft }
                    }
                }
            }
        }
    }

    // ── the shipped-bind legend (Shortcuts): rail + one content column ────────
    Component {
        id: shortcutsComp

        Item {
            id: shroot
            readonly property int railW: 200

            // ── category rail: All + every category, with counts ──
            Flickable {
                id: railFlick
                anchors { left: parent.left; top: parent.top; bottom: parent.bottom }
                width: shroot.railW
                contentWidth: width
                contentHeight: railCol.height
                clip: true
                boundsBehavior: Flickable.StopAtBounds
                ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
                WheelScroll { }

                Column {
                    id: railCol
                    width: railFlick.width
                    spacing: 2

                    Repeater {
                        model: pg.railModel
                        delegate: Rectangle {
                            id: railItem
                            required property var modelData
                            required property int index
                            width: railCol.width
                            height: 34
                            radius: Tokens.radius
                            readonly property bool active: !pg.searching && pg.selectedCat === railItem.modelData.name
                            readonly property bool dimmed: pg.searching && railItem.modelData.count === 0
                            color: railItem.active ? Tokens.tint10 : (railHov.hovered ? Tokens.tint5 : "transparent")
                            Behavior on color { ColorAnimation { duration: Tokens.snap } }

                            Rectangle {
                                anchors.left: parent.left
                                anchors.verticalCenter: parent.verticalCenter
                                width: 2; height: parent.height - Tokens.s4
                                radius: 1; color: Tokens.ink
                                visible: railItem.active
                            }
                            Text {
                                anchors.left: parent.left
                                anchors.leftMargin: Tokens.s4
                                anchors.right: railCount.left
                                anchors.rightMargin: Tokens.s2
                                anchors.verticalCenter: parent.verticalCenter
                                text: railItem.modelData.label
                                color: railItem.active ? Tokens.ink : (railItem.dimmed ? Tokens.inkFaint : Tokens.inkDim)
                                font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                                elide: Text.ElideRight
                            }
                            Text {
                                id: railCount
                                anchors.right: parent.right
                                anchors.rightMargin: Tokens.s3
                                anchors.verticalCenter: parent.verticalCenter
                                text: "" + railItem.modelData.count
                                color: Tokens.inkFaint
                                font.family: Tokens.mono; font.pixelSize: Tokens.fMicro
                            }
                            HoverHandler { id: railHov; cursorShape: Qt.PointingHandCursor }
                            TapHandler {
                                onTapped: {
                                    pg.clearSearch();
                                    pg.selectedCat = railItem.modelData.name;
                                }
                            }
                        }
                    }
                }
            }

            // divider between the rail and the content column
            Rectangle {
                id: railDiv
                anchors.left: railFlick.right
                anchors.leftMargin: Tokens.s4
                anchors.top: parent.top
                anchors.bottom: parent.bottom
                width: 1; color: Tokens.lineSoft
            }

            // ── content: category cards, laid into balanced columns (All and
            // search) or one full-width card (a picked category), read in the
            // Hub's card vocabulary rather than a bare list. ──
            Flickable {
                id: contentFlick
                anchors {
                    left: railDiv.right; leftMargin: Tokens.s5
                    right: parent.right
                    top: parent.top; bottom: parent.bottom; bottomMargin: Tokens.s3
                }
                contentWidth: width
                contentHeight: Math.max(grid.height, height)
                clip: true
                boundsBehavior: Flickable.StopAtBounds
                ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
                WheelScroll { }

                CardColumns {
                    id: grid
                    width: contentFlick.width - Tokens.s3
                    // two balanced columns when the measure holds them, one otherwise
                    maxColumns: 2
                    fillTo: contentFlick.height

                    Repeater {
                        model: pg.shownCats

                        delegate: Item {
                            id: catBlock
                            required property var modelData
                            required property int index
                            // a picked category takes the whole measure; All and
                            // search spread through the balanced columns.
                            property bool fullWidth: pg.selectedCat !== "" && !pg.searching
                            readonly property var binds: catBlock.modelData.binds || []
                            width: catBlock.fullWidth ? grid.width : grid.colWidth
                            height: card.height

                            SettingCard {
                                id: card
                                width: parent.width
                                collapsible: false
                                title: ("" + catBlock.modelData.name).toUpperCase()

                                Repeater {
                                    model: catBlock.binds

                                    delegate: Item {
                                        id: rowItem
                                        required property var modelData
                                        required property int index
                                        readonly property string combo: rowItem.modelData.combo || ""
                                        readonly property bool reboundRow: pg.isRebound(rowItem.combo)
                                        readonly property bool rebindableRow: rowItem.modelData.rebindable === true
                                        readonly property string unhon: rowItem.modelData.unhonored || ""
                                        readonly property bool customRow: (rowItem.modelData.kind || "") === "custom"
                                        readonly property bool clashRow: rowItem.reboundRow && pg.comboConflict(rowItem.combo)
                                        readonly property var effKeys: rowItem.reboundRow ? pg.comboToCaps(pg.effectiveCombo(rowItem.combo)) : (rowItem.modelData.keys || [])
                                        // a rebindable, honoured row records a new chord; a custom
                                        // row records into its Custom-tab row instead.
                                        readonly property bool editable: rowItem.unhon.length === 0 && pg.hubReady && (rowItem.rebindableRow || rowItem.customRow)
                                        // a fixed dedicated key (mouse, media, hardware): not
                                        // rebindable, still honoured, so it shows a lock not an edit.
                                        readonly property bool dedicated: !rowItem.rebindableRow && !rowItem.customRow && rowItem.unhon.length === 0
                                        // 45% ink on an unhonoured row's own text; the caps mute too.
                                        readonly property color rowInk: rowItem.unhon.length ? Qt.rgba(Tokens.ink.r, Tokens.ink.g, Tokens.ink.b, 0.45) : Tokens.ink

                                        width: card.width
                                        height: 44

                                        function beginEdit() {
                                            if (!rowItem.editable)
                                                return;
                                            if (rowItem.customRow) {
                                                // land on the Custom tab and record that same row
                                                pg.tab = "custom";
                                                var i = pg.customIndexFor(rowItem.combo);
                                                if (i >= 0)
                                                    Qt.callLater(function () { pg.startRecord(i); });
                                            } else {
                                                pg.startRecordShipped(rowItem.combo);
                                            }
                                        }

                                        // hairline between rows, inset like SettingRow.
                                        Rectangle {
                                            visible: rowItem.index > 0
                                            anchors { left: parent.left; right: parent.right; top: parent.top }
                                            anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s4
                                            height: 1; color: Tokens.lineSoft
                                        }
                                        // hover wash, quiet like SettingRow's.
                                        Rectangle {
                                            anchors.fill: parent; anchors.margins: 1
                                            radius: Tokens.radius
                                            color: rowHov.hovered ? Tokens.tint5 : "transparent"
                                            Behavior on color { ColorAnimation { duration: Tokens.snap } }
                                        }
                                        HoverHandler { id: rowHov }

                                        // label over its hint, both one line, centred in the row.
                                        Column {
                                            anchors {
                                                left: parent.left; leftMargin: Tokens.s4
                                                right: cluster.left; rightMargin: Tokens.s3
                                                verticalCenter: parent.verticalCenter
                                            }
                                            spacing: 1
                                            Text {
                                                width: parent.width
                                                text: rowItem.modelData.desc || ""
                                                color: rowItem.rowInk
                                                font.family: Tokens.ui; font.pixelSize: Tokens.fRow
                                                elide: Text.ElideRight
                                            }
                                            Text {
                                                visible: ("" + (rowItem.modelData.hint || "")).length > 0
                                                width: parent.width
                                                text: rowItem.modelData.hint || ""
                                                color: Tokens.inkMuted
                                                font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                                                elide: Text.ElideRight
                                            }
                                        }

                                        Row {
                                            id: cluster
                                            anchors.right: parent.right; anchors.rightMargin: Tokens.s4
                                            anchors.verticalCenter: parent.verticalCenter
                                            spacing: Tokens.s2

                                            // caps; click to record on an editable row.
                                            Item {
                                                anchors.verticalCenter: parent.verticalCenter
                                                implicitWidth: capsRow.implicitWidth
                                                implicitHeight: 22
                                                KeyCaps {
                                                    id: capsRow
                                                    anchors.centerIn: parent
                                                    tokens: rowItem.effKeys
                                                    muted: rowItem.unhon.length > 0
                                                    rebound: rowItem.reboundRow
                                                    clash: rowItem.clashRow
                                                    hot: capHov.hovered && rowItem.editable
                                                }
                                                HoverHandler {
                                                    id: capHov
                                                    enabled: rowItem.editable
                                                    cursorShape: Qt.PointingHandCursor
                                                }
                                                TapHandler {
                                                    enabled: rowItem.editable
                                                    onTapped: rowItem.beginEdit()
                                                }
                                            }

                                            // "not on <provider>" (unhonoured, reason on hover) or "custom".
                                            Rectangle {
                                                anchors.verticalCenter: parent.verticalCenter
                                                visible: rowItem.unhon.length > 0 || rowItem.customRow
                                                implicitHeight: 18
                                                implicitWidth: tagText.implicitWidth + Tokens.s3
                                                radius: Tokens.radius
                                                color: "transparent"
                                                border.width: Tokens.border
                                                border.color: Tokens.line
                                                Text {
                                                    id: tagText
                                                    anchors.centerIn: parent
                                                    text: rowItem.unhon.length ? I18n.tr("not on %1").arg(pg.providerName) : I18n.tr("custom")
                                                    color: Tokens.inkFaint
                                                    font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                                                    font.letterSpacing: Tokens.trackMark
                                                }
                                                HoverHandler { id: tagHov; enabled: rowItem.unhon.length > 0 }
                                                ToolTip.visible: tagHov.hovered && rowItem.unhon.length > 0
                                                ToolTip.text: rowItem.unhon
                                            }

                                            // reset a rebound row to its shipped chord; always shown.
                                            IconBtn {
                                                anchors.verticalCenter: parent.verticalCenter
                                                visible: rowItem.reboundRow
                                                glyph: "\u21ba"
                                                onAct: pg.clearRebind(rowItem.combo)
                                                HoverHandler { id: resetHov }
                                                ToolTip.visible: resetHov.hovered
                                                ToolTip.text: I18n.tr("Back to %1").arg(pg.comboLabel(rowItem.combo))
                                            }
                                            // change the shortcut; always shown on an editable row.
                                            IconBtn {
                                                anchors.verticalCenter: parent.verticalCenter
                                                visible: rowItem.editable
                                                glyph: "\u270e"
                                                onAct: rowItem.beginEdit()
                                                HoverHandler { id: editHov }
                                                ToolTip.visible: editHov.hovered
                                                ToolTip.text: I18n.tr("Change shortcut")
                                            }
                                            // a dedicated key cannot be rebound; a lock says so.
                                            Item {
                                                anchors.verticalCenter: parent.verticalCenter
                                                visible: rowItem.dedicated
                                                implicitWidth: 26; implicitHeight: 26
                                                LockMark { anchors.centerIn: parent; tint: Tokens.inkFaint }
                                                HoverHandler { id: lockHov }
                                                ToolTip.visible: lockHov.hovered
                                                ToolTip.text: I18n.tr("Bound to a dedicated key")
                                            }
                                        }
                                    }
                                }
                            }

                            // the category's row count, muted, where the caret would sit.
                            Text {
                                anchors.right: parent.right; anchors.rightMargin: Tokens.s4
                                anchors.top: parent.top
                                anchors.topMargin: (card.headerH - implicitHeight) / 2
                                text: "" + catBlock.binds.length
                                color: Tokens.inkFaint
                                font.family: Tokens.mono; font.pixelSize: Tokens.fMicro
                            }
                        }
                    }
                }
            }

            // empty state: only once the legend has loaded (or a search misses).
            Text {
                anchors.centerIn: contentFlick
                visible: pg.shownCats.length === 0 && (pg.searching || pg.categories.length > 0)
                text: pg.searching ? I18n.tr("No shortcuts match \u201c%1\u201d").arg(pg.query.trim()) : I18n.tr("No shortcuts here.")
                color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
            }
        }
    }

    // ── the custom-bind editor ──────────────────────────────────────────────
    Component {
        id: customComp

        Item {
            id: editor

            // which row's action catalogue is open; -1 = closed.
            property int pickerRow: -1
            function openPicker(i) { editor.pickerRow = i; picker.open(); }

            // intro: load-bearing product guidance, not decoration.
            Text {
                id: intro
                anchors { left: parent.left; right: parent.right; top: parent.top }
                wrapMode: Text.WordWrap
                text: pg.wmCfgPath
                    ? I18n.tr("Custom shortcuts layered over the ones Ryoku ships and kept in the Hub, so they show in the Shortcuts legend and get conflict-checked. Click record and press the combo -- even SUPER + Q is captured safely -- or type it, e.g. SUPER + J. Binds hand-written in %1 never appear here and are not conflict-checked.").arg(pg.wmCfgPath)
                    : I18n.tr("Custom shortcuts layered over the ones Ryoku ships and kept in the Hub, so they show in the Shortcuts legend and get conflict-checked. Click record and press the combo -- even SUPER + Q is captured safely -- or type it, e.g. SUPER + J. Binds hand-written in the compositor's own config never appear here and are not conflict-checked.")
                color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                lineHeight: 1.3
            }

            // section head: dot + SHORTCUTS + leader + count + add.
            Item {
                id: sect
                anchors { left: parent.left; right: parent.right; top: intro.bottom; topMargin: Tokens.s4 }
                height: 32

                Row {
                    id: sectLabel
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Tokens.s2
                    Rectangle {
                        width: 4; height: 4; color: Tokens.ink
                        anchors.verticalCenter: parent.verticalCenter
                    }
                    Text {
                        text: I18n.tr("SHORTCUTS"); color: Tokens.ink; font.family: Tokens.ui
                        font.pixelSize: Tokens.fMicro; font.weight: Font.Medium
                        font.letterSpacing: Tokens.trackMark
                        anchors.verticalCenter: parent.verticalCenter
                    }
                }

                Row {
                    id: sectActions
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Tokens.s3

                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        // an entry count is file-truth chrome, so mono.
                        text: pg.customRows.length === 1 ? I18n.tr("%1 BIND").arg(pg.customRows.length) : I18n.tr("%1 BINDS").arg(pg.customRows.length)
                        color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny
                    }
                    IconBtn {
                        anchors.verticalCenter: parent.verticalCenter
                        glyph: "+"
                        armed: pg.hubReady
                        // add a row and drop straight into recording it: the
                        // one-click path to a brand-new shortcut.
                        onAct: { pg.addRow(); Qt.callLater(function() { pg.startRecord(pg.customRows.length - 1); }); }
                    }
                }

                Rectangle {
                    anchors.left: sectLabel.right; anchors.right: sectActions.left
                    anchors.leftMargin: Tokens.s3; anchors.rightMargin: Tokens.s3
                    anchors.verticalCenter: parent.verticalCenter
                    height: 1; color: Tokens.lineSoft
                }
            }

            // ── the scrolling bind list ──
            Flickable {
                id: flick
                anchors {
                    left: parent.left; right: parent.right
                    top: sect.bottom; bottom: parent.bottom
                    topMargin: Tokens.s4; bottomMargin: Tokens.s3
                }
                contentWidth: width
                contentHeight: Math.max(rowsCol.height, height)
                clip: true
                boundsBehavior: Flickable.StopAtBounds
                ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
                WheelScroll { }

                Column {
                    id: rowsCol
            // a body of cards fills the measure and splits into balanced columns
            width: flick.width - Tokens.s3
                    spacing: Tokens.s2

                    Repeater {
                        model: pg.customRows

                        delegate: Rectangle {
                            id: rowRect
                            required property int index
                            required property var modelData
                            readonly property bool isRelease: rowRect.modelData.release === true
                            readonly property int lineH: 30
                            readonly property real gap: Tokens.s2
                            readonly property real removeW: 26
                            readonly property real keysW: 200
                            readonly property real recW: 26
                            readonly property string act: rowRect.modelData.action || "exec"
                            readonly property bool needsValue: act === "exec"
                            readonly property string conflict: pg.rowConflict(rowRect.index)

                            width: rowsCol.width
                            height: Tokens.s3 * 2 + lineH + (conflict !== "" ? 18 : 0)
                            radius: Tokens.radius
                            color: rh.hovered ? Tokens.tint5 : "transparent"
                            border.width: Tokens.border
                            // a conflict flags itself in solid ink -- the brightest
                            // tell on the sheet, colour-free (DESIGN.md section 1).
                            border.color: conflict !== "" ? Tokens.ink
                                : (rh.hovered ? Tokens.lineStrong : Tokens.line)
                            Behavior on color { ColorAnimation { duration: Tokens.snap } }
                            Behavior on border.color { ColorAnimation { duration: Tokens.snap } }

                            HoverHandler { id: rh }

                            // ── controls line ──
                            Item {
                                id: ctlRow
                                anchors {
                                    left: parent.left; right: parent.right; top: parent.top
                                    leftMargin: Tokens.s3; rightMargin: Tokens.s3; topMargin: Tokens.s3
                                }
                                height: rowRect.lineH

                                // key combo: a raw combo string, so mono.
                                Field {
                                    id: keysF
                                    anchors.left: parent.left
                                    anchors.verticalCenter: parent.verticalCenter
                                    width: rowRect.keysW - rowRect.recW - rowRect.gap
                                    tabular: true
                                    placeholder: "SUPER + J"
                                    text: rowRect.modelData.keys || ""
                                    onCommitted: (v) => {
                                        if (v !== (rowRect.modelData.keys || ""))
                                            pg.patch(rowRect.index, "keys", v);
                                    }
                                }
                                Rectangle {
                                    id: releaseToggle
                                    anchors.right: removeBtn.left
                                    anchors.rightMargin: rowRect.gap
                                    anchors.verticalCenter: parent.verticalCenter
                                    implicitHeight: 24
                                    implicitWidth: relText.implicitWidth + 12
                                    radius: Tokens.radius
                                    color: rowRect.isRelease ? Tokens.ink : "transparent"
                                    border.width: Tokens.border
                                    border.color: rowRect.isRelease ? Tokens.ink : Tokens.line

                                    Text {
                                        id: relText
                                        anchors.centerIn: parent
                                        text: rowRect.isRelease ? I18n.tr("RELEASE") : I18n.tr("PRESS")
                                        color: rowRect.isRelease ? Tokens.paper : Tokens.inkFaint
                                        font.family: Tokens.mono
                                        font.pixelSize: Tokens.fMicro
                                        font.weight: Font.Medium
                                    }

                                    HoverHandler { id: relHov; cursorShape: Qt.PointingHandCursor }
                                    TapHandler {
                                        onTapped: {
                                            if (pg.hubReady)
                                                pg.patch(rowRect.index, "release", !rowRect.isRelease);
                                        }
                                    }
                                }
                                // record: capture the combo by pressing it. Safe
                                // because the page ShortcutInhibitor holds the
                                // compositor's shortcuts while recording, so the
                                // live chord reaches the field, not the compositor.
                                IconBtn {
                                    id: recBtn
                                    anchors.left: keysF.right
                                    anchors.leftMargin: rowRect.gap
                                    anchors.verticalCenter: parent.verticalCenter
                                    glyph: "\u25CF"
                                    armed: pg.hubReady
                                    onAct: pg.startRecord(rowRect.index)
                                }

                                IconBtn {
                                    id: removeBtn
                                    anchors.right: parent.right
                                    anchors.verticalCenter: parent.verticalCenter
                                    // a paired minus, not a trash icon: remove is not
                                    // danger and there is no red on the sheet.
                                    glyph: "\u2212"
                                    onAct: pg.removeRow(rowRect.index)
                                }

                                // command box: shown only for "Run command"; the foot
                                // bar fills whatever is left when it is hidden.
                                Item {
                                    id: cmdBox
                                    anchors.right: releaseToggle.left
                                    anchors.rightMargin: rowRect.gap
                                    anchors.verticalCenter: parent.verticalCenter
                                    height: rowRect.lineH
                                    width: rowRect.needsValue
                                        ? Math.max(180, Math.round((parent.width - rowRect.keysW - rowRect.removeW - rowRect.gap * 3) * 0.5))
                                        : 0

                                    // pick an app from the catalogue, or type the command.
                                    IconBtn {
                                        id: cmdPick
                                        anchors.right: parent.right
                                        anchors.verticalCenter: parent.verticalCenter
                                        visible: rowRect.needsValue
                                        glyph: "\u2026"
                                        armed: pg.hubReady
                                        onAct: pg.openAppPickerCustom(rowRect.index)
                                    }
                                    // command is a shell string, so mono.
                                    Field {
                                        id: cmdF
                                        anchors.left: parent.left
                                        anchors.right: cmdPick.left
                                        anchors.rightMargin: rowRect.gap
                                        anchors.verticalCenter: parent.verticalCenter
                                        height: parent.height
                                        visible: rowRect.needsValue
                                        tabular: true
                                        placeholder: I18n.tr("command to run")
                                        text: rowRect.modelData.value || ""
                                        onCommitted: (v) => {
                                            if (v !== (rowRect.modelData.value || ""))
                                                pg.patch(rowRect.index, "value", v);
                                        }
                                    }
                                }

                                // the action catalogue foot bar.
                                PickBar {
                                    id: actionBar
                                    anchors.left: recBtn.right
                                    anchors.leftMargin: rowRect.gap
                                    anchors.right: cmdBox.left
                                    anchors.rightMargin: rowRect.needsValue ? rowRect.gap : 0
                                    anchors.verticalCenter: parent.verticalCenter
                                    value: pg.labelIn(pg.actionOpts, rowRect.act)
                                    count: pg.actionOpts.length
                                    onOpened: editor.openPicker(rowRect.index)
                                }
                            }

                            // conflict note: names what it clashes with, in ink,
                            // no colour (DESIGN.md section 1).
                            Text {
                                visible: rowRect.conflict !== ""
                                anchors.left: parent.left; anchors.leftMargin: Tokens.s3
                                anchors.bottom: parent.bottom; anchors.bottomMargin: Tokens.s2
                                text: {
                                    if (rowRect.conflict === "duplicate")
                                        return I18n.tr("Duplicate of another custom bind");
                                    var d = pg.shippedDescFor(pg.normKeys(rowRect.modelData.keys || ""));
                                    return d ? I18n.tr("Shadows shipped: %1").arg(d) : I18n.tr("Shadows a shipped bind");
                                }
                                color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                            }
                        }
                    }
                }
            }

            // empty state, gated on load so it does not flash before data arrives.
            Text {
                anchors.centerIn: flick
                visible: pg.ready && pg.customRows.length === 0
                text: I18n.tr("No custom shortcuts yet. Add one to bind a command or a window action.")
                color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
            }

            // ── the action catalogue overlay (Picker), shared across rows ──
            MouseArea {
                id: scrim
                anchors.fill: parent
                visible: editor.pickerRow >= 0
                z: 100
                // a bare click-catcher: dismiss on an outside click. No fill --
                // translucency is banned on app surfaces (DESIGN.md section 6).
                onClicked: editor.pickerRow = -1

                Picker {
                    id: picker
                    anchors.centerIn: parent
                    title: I18n.tr("Action")
                    options: pg.labelsOf(pg.actionOpts)
                    current: {
                        if (editor.pickerRow < 0)
                            return "";
                        var r = (pg.customRows[editor.pickerRow] || ({})).action || "exec";
                        return pg.labelIn(pg.actionOpts, r);
                    }
                    onChose: (label) => {
                        pg.setAction(editor.pickerRow, pg.keyIn(pg.actionOpts, label));
                        editor.pickerRow = -1;
                    }
                    onDismissed: editor.pickerRow = -1

                    // absorb clicks inside the card so the scrim does not treat a
                    // header/padding tap as an outside dismiss.
                    MouseArea { anchors.fill: parent; z: -1 }
                }
            }

        }
    }

    // ── action bar: status + reset / revert / save, shared by all tabs ──
    // full-bleed page, so the shell's global bar is hidden; this persists the
    // shared store. Page-level so a rebind recorded on the Shortcuts tab saves
    // from the same bar as a custom bind on the Custom tab.
    Rectangle {
        id: bar
        anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
        anchors.leftMargin: Tokens.s6; anchors.rightMargin: Tokens.s6; anchors.bottomMargin: Tokens.s5
        height: 60
        color: "transparent"
        Rectangle {
            anchors { left: parent.left; right: parent.right; top: parent.top }
            height: 1; color: Tokens.line
        }

        Row {
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s3

            Rectangle {
                id: dot
                anchors.verticalCenter: parent.verticalCenter
                width: 6; height: 6; radius: 3
                antialiasing: false
                readonly property bool lit: pg.conflictCount > 0 || pg.dirtyCount > 0
                color: lit ? Tokens.ink : "transparent"
                border.width: lit ? 0 : Tokens.border
                border.color: Tokens.inkFaint
                SequentialAnimation on opacity {
                    running: pg.dirtyCount > 0
                    loops: Animation.Infinite
                    NumberAnimation { to: 0.3; duration: 600; easing.type: Easing.InOutSine }
                    NumberAnimation { to: 1.0; duration: 600; easing.type: Easing.InOutSine }
                    onStopped: dot.opacity = 1
                }
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: pg.conflictCount > 0
                    ? (pg.conflictCount === 1 ? I18n.tr("%1 conflicting shortcut").arg(pg.conflictCount) : I18n.tr("%1 conflicting shortcuts").arg(pg.conflictCount))
                    : (pg.dirtyCount > 0 ? I18n.tr("Unsaved changes") : I18n.tr("Saved"))
                color: (pg.conflictCount > 0 || pg.dirtyCount > 0) ? Tokens.ink : Tokens.inkMuted
                font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                font.weight: Font.Medium
            }
        }

        Row {
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s3

            // tab-aware reset: clears rebinds on Shortcuts, custom binds on Custom.
            Btn {
                anchors.verticalCenter: parent.verticalCenter
                text: pg.tab === "apps" ? I18n.tr("RESET APPS") : (pg.tab === "shortcuts" ? I18n.tr("RESET REBINDS") : I18n.tr("RESTORE DEFAULTS"))
                armed: pg.tab === "apps" ? pg.hasApps() : (pg.tab === "shortcuts" ? pg.hasRebinds() : pg.customRows.length > 0)
                onAct: pg.tab === "apps" ? pg.clearApps() : (pg.tab === "shortcuts" ? pg.clearRebinds() : pg.clearAll())
            }
            Btn {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("REVERT")
                armed: pg.dirtyCount > 0
                onAct: pg.revertAll()
            }
            Btn {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("SAVE")
                primary: true
                armed: pg.dirtyCount > 0
                onAct: pg.saveAll()
            }
        }

    }

    // ── the app catalogue picker (shared: Apps roles + custom Run binds) ─────
    MouseArea {
        id: appScrim
        anchors.fill: parent
        visible: pg.appPicking
        z: 150
        onClicked: pg.closeAppPicker()

        // Ryoku.Ui's AppPicker (the one this page resolves; the hub root's
        // AppPicker.qml is not on this directory's import path) emits picked(cmd).
        AppPicker {
            id: appPicker
            anchors.centerIn: parent
            title: pg.appPickTitle()
            apps: pg.appCatalog
            current: pg.appPickCurrent()
            onPicked: (cmd) => pg.applyAppPick(cmd)
            onDismissed: pg.closeAppPicker()
            Connections {
                target: pg
                function onAppPickingChanged() { if (pg.appPicking) appPicker.open(); }
            }
            MouseArea { anchors.fill: parent; z: -1 }
        }
    }

    // ── chord recorder overlay (page-level: serves both tabs) ──
    // the capture Item holds focus so the chord the compositor releases arrives
    // as a plain KeyEvent; chordFrom turns it into the store's token.
    MouseArea {
        id: recScrim
        anchors.fill: parent
        visible: pg.recording
        z: 200
        onClicked: pg.stopRecord(false, "")
        onVisibleChanged: if (visible) capture.forceActiveFocus()

        Item {
            id: capture
            anchors.fill: parent
            focus: true
            Keys.onPressed: (event) => {
                event.accepted = true;
                if (event.isAutoRepeat)
                    return;
                if (event.key === Qt.Key_Escape) {
                    pg.stopRecord(false, "");
                    return;
                }
                // show the modifiers as they are held, then commit on the key.
                pg.recordForming = Combos.formingChord(event);
                var chord = pg.chordFrom(event);
                if (chord === "")
                    return;
                if (pg.recordFamily) {
                    // a family accepts only a number key; anything else nudges
                    // and keeps the overlay open until a digit lands.
                    var fam = Combos.familyFrom(chord);
                    if (fam === "") {
                        pg.recordNeedNumber = true;
                        return;
                    }
                    pg.stopRecord(true, fam);
                    return;
                }
                pg.stopRecord(true, chord);
            }
        }

        Rectangle {
            anchors.centerIn: parent
            width: 360; height: pg.recordFamily ? 208 : 176
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
                    text: I18n.tr("PRESS YOUR SHORTCUT")
                    color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                    font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                }

                // the chord as it forms: the held modifiers, then a pending cap.
                Row {
                    anchors.horizontalCenter: parent.horizontalCenter
                    spacing: Tokens.s1
                    KeyCaps { tokens: pg.comboToCaps(pg.recordForming) }
                    Text {
                        visible: pg.recordForming.length > 0
                        height: 22; verticalAlignment: Text.AlignVCenter
                        text: "+"; color: Tokens.inkFaint
                        font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                    }
                    Rectangle {
                        implicitHeight: 22; implicitWidth: 28
                        radius: Tokens.radius; color: "transparent"
                        border.width: Tokens.border; border.color: Tokens.line
                        Text {
                            anchors.centerIn: parent
                            text: "\u2026"; color: Tokens.inkFaint
                            font.family: Tokens.mono; font.pixelSize: Tokens.fMicro
                        }
                    }
                }

                Text {
                    width: parent.width
                    horizontalAlignment: Text.AlignHCenter
                    wrapMode: Text.WordWrap
                    text: pg.recordFamily
                        ? I18n.tr("Hold the new modifiers and press any number key. Esc cancels.")
                        : I18n.tr("Hold your modifiers and tap the key. Esc cancels.")
                    color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                }
                // a family record takes only a number key; nudge on anything else.
                Text {
                    anchors.horizontalCenter: parent.horizontalCenter
                    visible: pg.recordFamily && pg.recordNeedNumber
                    text: I18n.tr("Press a number key")
                    color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                    font.weight: Font.Medium
                }
                Btn {
                    anchors.horizontalCenter: parent.horizontalCenter
                    text: I18n.tr("CANCEL")
                    armed: true
                    onAct: pg.stopRecord(false, "")
                }
            }

            MouseArea { anchors.fill: parent; z: -1 }
        }
    }

    // ── shortcut inhibitor: compositor-neutral chord capture ──
    // While recording, this hands the compositor's own shortcuts to the focused
    // Hub window instead of running them, so SUPER + Q lands in the recorder as
    // Qt.Key_Q with MetaModifier rather than closing the window (verified on niri
    // 26.04 + quickshell 0.3.1). It is the whole capture story on niri; on
    // Hyprland the record submap backs it up. ProxyWindowBase.forObject wants the
    // QsWindow (the Hub's FloatingWindow), not QtQuick's QQuickWindow, so this
    // reads the QsWindow attached object rather than Window.window.
    ShortcutInhibitor {
        window: pg.QsWindow ? pg.QsWindow.window : null
        enabled: pg.recording
    }
}
