import QtQuick
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../../Singletons"
import "keymap.js" as Keymap
import "../../schema/keyboard.js" as Schema

// the keyboard as a Changes store: each control edits its part of the desktop
// store's layout, variant and option strings, and "everywhere" copies the
// layout to the sign-in screen, the console and the startup image
QtObject {
    id: store

    readonly property var layoutBind: ({ store: "desktop", key: "input.kbLayout" })
    readonly property var variantBind: ({ store: "desktop", key: "input.kbVariant" })
    readonly property var optionsBind: ({ store: "desktop", key: "input.kbOptions" })

    // what `ryoku keyboard status --json` last said, or null
    property var status: null
    // "", "running", "done", "stopped"
    property string phase: ""
    property bool slow: false
    property bool verySlow: false
    property bool cancelled: false
    property var applyDone: null
    property var applyRun: null

    // desktop key -> the keyboard write that last changed it, for its error
    property var lastWrite: ({})
    // layout code -> the style last used with it, so going back restores it
    property var variantFor: ({})

    function desk(bind) {
        var v = Changes.value(bind);
        return v === undefined || v === null ? "" : String(v);
    }

    function read(key) {
        var layout = store.desk(store.layoutBind), options = store.desk(store.optionsBind);
        switch (key) {
        case "layout": return Keymap.primary(layout);
        case "variant": return Keymap.primary(store.desk(store.variantBind));
        case "second": return Keymap.secondary(layout);
        case "hasSecond": return Keymap.secondary(layout) !== "";
        case "switch": return Keymap.pick(options, Keymap.switchIds);
        case "caps": return Keymap.pick(options, Keymap.capsIds);
        case "compose": return Keymap.pick(options, Keymap.composeIds);
        case "swap": return Keymap.pick(options, [Keymap.swapId]) === Keymap.swapId;
        case "extra": return Keymap.extra(options);
        case "everywhere": return store.phase;
        }
        return undefined;
    }

    function put(bind, value, key, v) {
        if (store.desk(bind) === value)
            return;
        var w = Object.assign({}, store.lastWrite);
        w[bind.key] = { bind: { store: "keyboard", key: key }, value: v };
        store.lastWrite = w;
        Changes.write(bind, value, "", "keyboard");
    }

    function write(key, v, done) {
        var layout = store.desk(store.layoutBind), options = store.desk(store.optionsBind);
        var first = Keymap.primary(layout), second = Keymap.secondary(layout);
        var variant = Keymap.primary(store.desk(store.variantBind));
        switch (key) {
        case "layout":
            var memory = Object.assign({}, store.variantFor);
            memory[first] = variant;
            store.variantFor = memory;
            var chosen = String(v || "us");
            store.put(store.layoutBind, Keymap.layoutString(chosen, second === chosen ? "" : second), key, v);
            store.put(store.variantBind, Keymap.variantString(memory[chosen] || "", second === chosen ? "" : second), key, v);
            break;
        case "variant":
            store.put(store.variantBind, Keymap.variantString(String(v || ""), second), key, v);
            break;
        case "second":
            var next = String(v || "") === first ? "" : String(v || "");
            store.put(store.layoutBind, Keymap.layoutString(first, next), key, v);
            store.put(store.variantBind, Keymap.variantString(variant, next), key, v);
            break;
        case "switch":
            store.put(store.optionsBind, Keymap.withFamily(options, Keymap.switchIds, String(v || "")), key, v);
            break;
        case "caps":
            store.put(store.optionsBind, Keymap.withFamily(options, Keymap.capsIds, String(v || "")), key, v);
            break;
        case "compose":
            store.put(store.optionsBind, Keymap.withFamily(options, Keymap.composeIds, String(v || "")), key, v);
            break;
        case "swap":
            store.put(store.optionsBind, Keymap.withFamily(options, [Keymap.swapId], v === true ? Keymap.swapId : ""), key, v);
            break;
        case "extra":
            store.put(store.optionsBind, Keymap.withExtra(options, String(v || "")), key, v);
            break;
        case "everywhere":
            store.apply(done);
            return;
        }
        Changes.afterDesktopSave(done);
    }

    // a desktop save that failed lands on the keyboard row that caused it
    function moveErrors() {
        var binds = [store.layoutBind, store.variantBind, store.optionsBind];
        for (var i = 0; i < binds.length; i++) {
            var message = Changes.error(binds[i]);
            var w = store.lastWrite[binds[i].key];
            if (!message || !w)
                continue;
            Changes.setError(binds[i], "", undefined);
            Changes.setError(w.bind, message, w.value, store.labelOf(w.bind.key), "keyboard");
        }
    }

    // the row's own name, so Retry's notice says what changed
    function labelOf(key) {
        var row = Schema.rows.find(r => r.bind && r.bind.store === "keyboard" && r.bind.key === key);
        return row ? (row.changeLabel || row.label) : "";
    }

    property Connections changes: Connections {
        target: Changes
        function onRevisionChanged() { Qt.callLater(store.moveErrors); }
    }

    // sign-in screen, console and startup image

    function refreshStatus() {
        if (!store.statusProc.running)
            store.statusProc.running = true;
    }

    property Process statusProc: Process {
        command: ["ryoku", "keyboard", "status", "--json"]
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    store.status = JSON.parse(this.text);
                } catch (e) {
                    store.status = null;
                }
                Changes.touch();
            }
        }
    }

    function apply(done) {
        if (store.applyRun) {
            done(false, I18n.tr("Copying is still running. Try again when it finishes."));
            return;
        }
        var argv = ["ryoku-host", "keymap", "apply", Keymap.primary(store.desk(store.layoutBind)) || "us",
                    Keymap.primary(store.desk(store.variantBind)), store.desk(store.optionsBind)];
        store.applyDone = done;
        store.cancelled = false;
        store.slow = false;
        store.verySlow = false;
        store.phase = "running";
        store.slowTimer.restart();
        store.verySlowTimer.restart();
        Changes.touch();
        store.applyRun = Changes.run(argv, function (ok, out, err, code) {
            store.applyRun = null;
            store.finished(code, err);
        });
    }

    // stops waiting; a step already running as root may still finish
    function cancel() {
        if (!store.applyRun)
            return;
        store.cancelled = true;
        store.applyRun.signal(15);
    }

    function finished(code, errText) {
        store.slowTimer.stop();
        store.verySlowTimer.stop();
        store.slow = false;
        store.verySlow = false;
        var done = store.applyDone;
        store.applyDone = null;
        // the helper's last line, without its own name in front
        var err = String(errText || "").trim().split("\n").pop().trim().replace(/^ryoku-host:\s*/, "").replace(/[.:]$/, "");
        if (store.cancelled) {
            store.phase = "stopped";
            if (done)
                done(true, "");
        } else if (code === 0) {
            store.phase = "done";
            if (done)
                done(true, "");
        } else {
            store.phase = "";
            // pkexec reports a closed or refused password prompt as 126 or 127; -1 is a missing
            // or stuck program, whose message already says what to do
            var message = code === -1 ? String(errText || "").trim()
                : code === 126 || code === 127
                ? I18n.tr("The password prompt was closed, so nothing changed. Try again and enter your password.")
                : err.length > 0 ? I18n.tr("Copying did not finish: %1. Check the message, then try again.").arg(err)
                : I18n.tr("Copying did not finish. Try again; if it fails again, run ryoku keyboard apply in a terminal to see why.");
            if (done)
                done(false, message);
        }
        store.refreshStatus();
        Changes.touch();
    }

    property Timer slowTimer: Timer {
        interval: 1000
        onTriggered: {
            store.slow = true;
            Changes.touch();
        }
    }

    property Timer verySlowTimer: Timer {
        interval: 10000
        onTriggered: {
            store.verySlow = true;
            Changes.touch();
        }
    }

    Component.onCompleted: {
        Changes.registerStore("keyboard", store);
        store.refreshStatus();
    }
}
