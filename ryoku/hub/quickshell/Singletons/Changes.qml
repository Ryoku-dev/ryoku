pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons

// the one write path for the settings window: shell settings over the bus, the
// window manager store through ryoku-hub, and backend commands, with undo,
// changed marks against defaults, reset and per-row errors
Singleton {
    id: root

    // ticks whenever a value this service owns changes, so readers rebind
    property int revision: 0

    // desktop store: what disk holds, what is live, and the provider defaults
    property var desktopCommitted: ({})
    property var desktopDraft: ({})
    property var desktopDefaults: ({})
    property bool desktopReady: false
    property bool desktopDefaultsReady: false
    // paths edited since the last save started, with the values sent
    property var desktopPending: ({})
    property var desktopInFlight: ({})
    property bool desktopSaveAgain: false

    property var commandValues: ({})
    property var errors: ({})
    // in-place set of command reads already started; never reassigned
    property var requested: ({})
    property var failedValues: ({})

    // undo stack and this session's changes, newest first
    property var undoStack: []
    property var recent: []

    property var flushCallback: null

    signal announced(var entry)
    signal undone(var entry)

    function keyOf(bind) {
        if (!bind)
            return "";
        if (bind.store === "command")
            return "command:" + (bind.get || []).join(" ");
        return bind.store + ":" + bind.key;
    }

    // the desktop store's own dotted path; schema keys omit "desktop."
    function storePath(key) {
        key = String(key || "");
        return key.indexOf("wm.") === 0 ? key : "desktop." + key;
    }

    function pathGet(obj, path) {
        var parts = path.split("."), cur = obj;
        for (var i = 0; i < parts.length; i++) {
            if (cur === undefined || cur === null)
                return undefined;
            cur = cur[parts[i]];
        }
        return cur;
    }

    function pathSet(obj, path, v) {
        var parts = path.split("."), cur = obj;
        for (var i = 0; i < parts.length - 1; i++) {
            if (typeof cur[parts[i]] !== "object" || cur[parts[i]] === null)
                cur[parts[i]] = {};
            cur = cur[parts[i]];
        }
        cur[parts[parts.length - 1]] = v;
    }

    function clone(v) {
        return v === undefined ? undefined : JSON.parse(JSON.stringify(v));
    }

    function same(a, b) {
        return JSON.stringify(a) === JSON.stringify(b);
    }

    // a key unset until first written reads as bind.fallback
    function value(bind) {
        if (!bind)
            return undefined;
        var v = root.stored(bind);
        return v === undefined && bind.fallback !== undefined ? bind.fallback : v;
    }

    function stored(bind) {
        root.revision;
        switch (bind.store) {
        case "settings":
            return Settings.get(bind.key);
        case "desktop":
            return root.pathGet(root.desktopDraft, root.storePath(bind.key));
        case "command":
            var k = root.keyOf(bind);
            if (!root.requested[k]) {
                root.requested[k] = true;
                Qt.callLater(root.readCommand, bind);
            }
            return root.commandValues[k];
        }
        var h = root.stores[bind.store];
        return h ? h.read(bind.key) : undefined;
    }

    // a page that owns a live source (a daemon topic) registers it as a store:
    // handler { read(key) -> value, write(key, value, done(ok, error)) }
    property var stores: ({})

    function registerStore(name, handler) {
        var s = Object.assign({}, root.stores);
        s[name] = handler;
        root.stores = s;
        root.revision++;
    }

    // a store that must outlive its page (undo after leaving it) is created
    // here, in this context; the object registers itself when it completes
    property var owned: []

    function ensureStore(name, url) {
        if (root.stores[name])
            return;
        var c = Qt.createComponent(url);
        if (c.status !== Component.Ready) {
            console.warn("hub: store " + name + ": " + c.errorString());
            return;
        }
        var o = c.createObject(root);
        if (o)
            root.owned = root.owned.concat([o]);
        else
            console.warn("hub: store " + name + ": " + c.errorString());
    }

    function unregisterStore(name) {
        var s = Object.assign({}, root.stores);
        delete s[name];
        root.stores = s;
        root.revision++;
    }

    // the store's source changed; readers rebind
    function touch() {
        root.revision++;
    }

    function defaultOf(row) {
        root.revision;
        if (!row)
            return undefined;
        if (row["default"] !== undefined)
            return row["default"];
        if (row.bind && row.bind.store === "desktop" && root.desktopDefaultsReady)
            return root.pathGet(root.desktopDefaults, root.storePath(row.bind.key));
        return undefined;
    }

    function isChanged(row) {
        var d = root.defaultOf(row);
        if (d === undefined || !row.bind)
            return false;
        var v = root.value(row.bind);
        if (v === undefined)
            return false;
        if (typeof d === "number" && typeof v === "number")
            return Math.abs(d - v) > 1e-9;
        return !root.same(d, v);
    }

    function reset(row) {
        var d = root.defaultOf(row);
        if (d === undefined)
            return;
        root.set(row.bind, root.clone(d), row.label, row.page || "");
    }

    function error(bind) {
        root.revision;
        return root.errors[root.keyOf(bind)] || "";
    }

    function failures() {
        root.revision;
        var out = [];
        for (var k in root.errors)
            if (root.errors[k])
                out.push({ key: k, error: root.errors[k], entry: root.failedValues[k] });
        return out;
    }

    function setError(bind, message, attempted, label, page) {
        var k = root.keyOf(bind);
        var e = Object.assign({}, root.errors);
        var f = Object.assign({}, root.failedValues);
        if (message) {
            e[k] = message;
            f[k] = { bind: bind, value: attempted, label: label || "", page: page || "" };
        } else {
            delete e[k];
            delete f[k];
        }
        root.errors = e;
        root.failedValues = f;
        root.revision++;
    }

    function retry(bind) {
        var f = root.failedValues[root.keyOf(bind)];
        if (!f)
            return;
        root.setError(bind, "", undefined);
        root.write(f.bind, f.value, f.label, f.page);
    }

    // a change from the UI: write it, remember it for undo, and say so;
    // notice, already translated, replaces the plain "<label> changed"
    function set(bind, v, label, page, notice) {
        if (!bind)
            return;
        var before = root.clone(root.value(bind));
        if (root.same(before, v))
            return;
        var entry = { bind: bind, before: before, after: root.clone(v), label: String(label || ""), page: String(page || ""),
                      notice: String(notice || ""), at: Date.now() };
        var stack = root.undoStack;
        // a drag or held key is one change: keep the first value it started from
        if (stack.length > 0 && root.sameSetting(stack[0], entry) && entry.at - stack[0].at < 2500) {
            entry.before = stack[0].before;
            stack = stack.slice(1);
        }
        root.undoStack = [entry].concat(stack).slice(0, 50);
        var rec = root.recent.filter(function (r) { return !root.sameSetting(r, entry); });
        root.recent = [entry].concat(rec).slice(0, 12);
        root.write(bind, v, label, page);
        root.announced(entry);
    }

    function undo() {
        if (root.undoStack.length === 0)
            return;
        root.undoEntry(root.undoStack[0]);
    }

    // one stored value can hold many settings, so the row's label tells them apart
    function sameSetting(a, b) {
        return root.keyOf(a.bind) === root.keyOf(b.bind) && a.label === b.label;
    }

    // undo puts back only what its change changed, keeping later changes to the same value
    function reverted(entry) {
        var before = entry.before, after = entry.after, now = root.clone(root.value(entry.bind));
        var key = function (x) { return JSON.stringify(x); };
        if (Array.isArray(after) && Array.isArray(now) && (before === undefined || Array.isArray(before))) {
            var was = (before || []).map(key), is = after.map(key);
            var out = now.filter(function (x) { return was.indexOf(key(x)) >= 0 || is.indexOf(key(x)) < 0; });
            (before || []).forEach(function (x) {
                if (is.indexOf(key(x)) < 0 && out.map(key).indexOf(key(x)) < 0)
                    out.push(root.clone(x));
            });
            return before === undefined && out.length === 0 ? undefined : out;
        }
        if (root.isMap(after) && root.isMap(now) && (before === undefined || root.isMap(before))) {
            var prior = before || {};
            Object.keys(prior).concat(Object.keys(after)).forEach(function (k) {
                if (root.same(prior[k], after[k]))
                    return;
                if (prior[k] === undefined)
                    delete now[k];
                else
                    now[k] = root.clone(prior[k]);
            });
            return before === undefined && Object.keys(now).length === 0 ? undefined : now;
        }
        return root.clone(before);
    }

    function isMap(v) {
        return v !== null && typeof v === "object" && !Array.isArray(v);
    }

    // undo one entry wherever it sits in the stack
    function undoEntry(entry) {
        if (!entry)
            return;
        root.undoStack = root.undoStack.filter(function (e) { return e !== entry; });
        root.recent = root.recent.filter(function (e) { return e !== entry; });
        var back = root.reverted(entry);
        // an absent desktop key is written back as absent: the save drops it
        if (back !== undefined || entry.bind.store === "desktop")
            root.write(entry.bind, back, entry.label, entry.page);
        root.undone(entry);
    }

    function write(bind, v, label, page) {
        root.setError(bind, "", undefined);
        switch (bind.store) {
        case "settings":
            Settings.patch(bind.key, v, function (ok, err) {
                if (!ok)
                    root.setError(bind, err || "The settings service did not take the change.", v, label, page);
            });
            // a value the shell applies in a second step (bar style: shells commit)
            if (bind.after) {
                Settings.flushWrites();
                Settings.afterWrites(() => Bus.run(bind.after));
            }
            root.revision++;
            break;
        case "desktop":
            root.desktopEdit(bind, v, label, page);
            break;
        case "command":
            root.runCommand(bind, v, label, page);
            break;
        default:
            var h = root.stores[bind.store];
            if (!h)
                break;
            h.write(bind.key, v, function (ok, err) {
                if (!ok)
                    root.setError(bind, err || "The change did not apply.", v, label, page);
            });
            root.revision++;
            break;
        }
    }

    // desktop store

    function desktopEdit(bind, v, label, page) {
        if (!root.desktopReady)
            return;
        var path = root.storePath(bind.key);
        var d = root.clone(root.desktopDraft);
        root.pathSet(d, path, root.clone(v));
        root.desktopDraft = d;
        var p = Object.assign({}, root.desktopPending);
        p[path] = { bind: bind, value: root.clone(v), label: label || "", page: page || "" };
        root.desktopPending = p;
        root.revision++;
        previewThrottle.restart();
        saveDelay.restart();
    }

    function saveDesktop() {
        saveDelay.stop();
        if (Object.keys(root.desktopPending).length === 0)
            return;
        if (saveProc.running) {
            root.desktopSaveAgain = true;
            return;
        }
        root.desktopInFlight = root.desktopPending;
        root.desktopPending = ({});
        saveProc.draft = root.clone(root.desktopDraft);
        saveProc.command = ["ryoku-hub", "desktop", "save", JSON.stringify(saveProc.draft)];
        saveProc.running = true;
    }

    function desktopSaved(ok, message) {
        var sent = root.desktopInFlight;
        root.desktopInFlight = ({});
        if (ok) {
            root.desktopCommitted = saveProc.draft;
        } else {
            // the old value stays on screen and live
            var d = root.clone(root.desktopDraft);
            for (var path in sent) {
                if (root.desktopPending[path])
                    continue;
                root.pathSet(d, path, root.clone(root.pathGet(root.desktopCommitted, path)));
                root.setError(sent[path].bind, message || "The window manager did not take the change.",
                              sent[path].value, sent[path].label, sent[path].page);
            }
            root.desktopDraft = d;
            restoreProc.running = true;
        }
        root.revision++;
        if (root.desktopSaveAgain || Object.keys(root.desktopPending).length > 0) {
            root.desktopSaveAgain = false;
            root.saveDesktop();
        }
        root.checkFlushed();
    }

    Process {
        id: desktopGet
        command: ["ryoku-hub", "desktop", "get"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var o = JSON.parse(this.text);
                    root.desktopCommitted = o;
                    root.desktopDraft = JSON.parse(this.text);
                    root.desktopReady = true;
                    root.revision++;
                } catch (e) {
                    root.desktopReady = false;
                }
            }
        }
    }

    Process {
        id: desktopDefaultsGet
        command: ["ryoku-hub", "desktop", "defaults"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    root.desktopDefaults = JSON.parse(this.text);
                    root.desktopDefaultsReady = true;
                    root.revision++;
                } catch (e) {
                    root.desktopDefaultsReady = false;
                }
            }
        }
    }

    Timer {
        id: previewThrottle
        interval: 140
        onTriggered: {
            if (previewProc.running) {
                previewThrottle.restart();
                return;
            }
            previewProc.command = ["ryoku-hub", "desktop", "preview", JSON.stringify(root.desktopDraft)];
            previewProc.running = true;
        }
    }

    Timer {
        id: saveDelay
        interval: 900
        onTriggered: root.saveDesktop()
    }

    Process { id: previewProc }
    Process { id: restoreProc; command: ["ryoku-hub", "desktop", "restore"] }

    Process {
        id: saveProc
        property var draft: ({})
        stderr: StdioCollector { id: saveErr }
        onExited: (code, status) => root.desktopSaved(code === 0, saveErr.text.trim())
    }

    // backend commands: get and set argv per row

    function readCommand(bind) {
        var k = root.keyOf(bind);
        var p = commandProc.createObject(root, { command: bind.get });
        p.finished.connect(function (ok, out) {
            var next = Object.assign({}, root.commandValues);
            var v = out.trim();
            try { v = JSON.parse(v); } catch (e) {}
            next[k] = v;
            root.commandValues = next;
            root.revision++;
            p.destroy();
        });
        p.running = true;
    }

    function runCommand(bind, v, label, page) {
        var argv = [];
        var placed = false;
        for (var i = 0; i < bind.set.length; i++) {
            if (bind.set[i] === "{value}") {
                argv.push(String(v));
                placed = true;
            } else {
                argv.push(bind.set[i]);
            }
        }
        if (!placed)
            argv.push(String(v));
        var p = commandProc.createObject(root, { command: argv });
        p.finished.connect(function (ok, out, err) {
            if (!ok)
                root.setError(bind, err.trim() || "The change did not apply.", v, label, page);
            root.readCommand(bind);
            p.destroy();
            root.checkFlushed();
        });
        p.running = true;
    }

    Component {
        id: commandProc
        Process {
            id: proc
            signal finished(bool ok, string out, string err)
            stdout: StdioCollector { id: cout }
            stderr: StdioCollector { id: cerr }
            onExited: (code, status) => proc.finished(code === 0, cout.text, cerr.text)
        }
    }

    // quit path: send everything pending, then call back once it settled
    readonly property bool unsettled: {
        root.revision;
        return Settings.writesUnsettled || saveProc.running || saveDelay.running
            || Object.keys(root.desktopPending).length > 0;
    }

    function flush(callback) {
        root.flushCallback = callback;
        Settings.flushWrites();
        previewThrottle.stop();
        root.saveDesktop();
        flushCap.restart();
        root.checkFlushed();
    }

    function checkFlushed() {
        if (!root.flushCallback || root.unsettled)
            return;
        var cb = root.flushCallback;
        root.flushCallback = null;
        flushCap.stop();
        cb();
    }

    Connections {
        target: Settings
        function onWritesUnsettledChanged() { root.checkFlushed(); }
    }

    // a dead daemon or backend never holds the window open
    Timer {
        id: flushCap
        interval: 3000
        onTriggered: {
            var cb = root.flushCallback;
            root.flushCallback = null;
            if (cb)
                cb();
        }
    }
}
