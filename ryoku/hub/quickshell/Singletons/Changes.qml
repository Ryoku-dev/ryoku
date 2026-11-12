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

    // values cross QML signals as copies whose object keys come back sorted, so equality
    // and list membership use a key-order-free form
    function stable(v) {
        return JSON.stringify(v, function (k, x) {
            if (x === null || typeof x !== "object" || Array.isArray(x))
                return x;
            // a list copied through a signal is a sequence, not an Array
            if (typeof x.length === "number")
                return Array.prototype.slice.call(x);
            var out = {};
            Object.keys(x).sort().forEach(function (key) { out[key] = x[key]; });
            return out;
        });
    }

    function same(a, b) {
        return root.stable(a) === root.stable(b);
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

    // every row of one page that stands away from its default, put back as a
    // single change: one notice, and one Ctrl+Z returns the whole page
    function resetPage(rows, pageId, label) {
        var parts = [];
        for (var i = 0; i < rows.length; i++) {
            var row = rows[i];
            if (!row || !row.bind || !root.isChanged(row))
                continue;
            var d = root.defaultOf(row);
            if (d === undefined)
                continue;
            parts.push({ bind: row.bind, before: root.clone(root.value(row.bind)), after: root.clone(d),
                         label: String(row.changeLabel || row.label || "") });
        }
        if (parts.length === 0)
            return false;
        var entry = { bind: parts[0].bind, parts: parts, before: undefined, after: undefined,
                      label: String(label || ""), page: String(pageId || ""),
                      notice: I18n.tr("Reset to defaults"), at: Date.now() };
        root.undoStack = [entry].concat(root.undoStack).slice(0, 50);
        root.recent = [entry].concat(root.recent).slice(0, 12);
        parts.forEach(function (p) { root.writeChange(p.bind, p.after, p.label, pageId); });
        root.announced(entry);
        root.revision++;
        return true;
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
        var failed = x => root.keyOf(x.bind) === k && root.same(x.after, attempted);
        if (message) {
            e[k] = message;
            // Retry repeats the change in its own words ("kitty removed", not "changed"); only a
            // value last sent by a plain write() (Copy, Run) is an action that retries without Undo
            var attempt = root.undoStack.find(failed);
            f[k] = { bind: bind, value: attempted, label: label || "", page: page || "", notice: attempt ? attempt.notice : "",
                     undoable: !!attempt || !root.actionKeys[k], before: attempt ? attempt.before : undefined };
        } else {
            delete e[k];
            delete f[k];
        }
        root.errors = e;
        root.failedValues = f;
        // a write that failed changed nothing: its Undo entry and notice go
        if (message) {
            root.undoStack = root.undoStack.filter(x => !failed(x));
            root.recent = root.recent.filter(x => !failed(x));
            root.relist(x => root.keyOf(x.bind) === k);
            root.undone({ bind: bind });
        }
        root.revision++;
    }

    // the same setting's earlier change, still undoable, comes back to the recent list when the
    // newer one leaves it (undone or failed), so Home and Ctrl+Z agree
    function relist(sameAs) {
        var earlier = root.undoStack.find(sameAs);
        if (earlier && root.recent.indexOf(earlier) < 0)
            root.recent = root.recent.concat([earlier]).sort((a, b) => b.at - a.at).slice(0, 12);
    }

    // a retry is the same change made again: a change gets its Undo entry and notice once it
    // takes; an action written without one (Copy, Run) stays without one
    function retry(bind) {
        var f = root.failedValues[root.keyOf(bind)];
        if (!f)
            return;
        root.setError(bind, "", undefined);
        // a store that already reads the failed value (it applied part way) is written again,
        // since set() would see no change and do nothing
        if (f.undoable && !root.same(root.value(f.bind), f.value))
            root.set(f.bind, f.value, f.label, f.page, f.notice);
        else if (f.undoable) {
            // the change still gets its Undo entry (back to the value before the first try) and
            // its notice once it takes
            var entry = { bind: f.bind, before: f.before, after: root.clone(f.value), label: f.label, page: f.page,
                          notice: f.notice, at: Date.now() };
            var known = f.before !== undefined;
            if (known) {
                root.undoStack = [entry].concat(root.undoStack).slice(0, 50);
                root.recent = [entry].concat(root.recent.filter(r => !root.sameSetting(r, entry))).slice(0, 12);
            }
            root.writeChange(f.bind, f.value, f.label, f.page, ok => {
                if (ok && known)
                    root.announced(entry);
            });
        }
        else
            root.write(f.bind, f.value, f.label, f.page);
    }

    // a change from the UI: write it, remember it for undo, and say so;
    // notice, already translated, replaces the plain "<label> changed" and
    // marks a discrete action (a list item added or removed) that never merges
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
        if (stack.length > 0 && entry.notice === "" && stack[0].notice === "" && root.sameSetting(stack[0], entry) && entry.at - stack[0].at < 2500) {
            entry.before = stack[0].before;
            stack = stack.slice(1);
        }
        var rec = root.recent.filter(function (r) { return !root.sameSetting(r, entry); });
        // a quick change back to where it started is no change: nothing to undo or announce
        if (root.same(entry.before, entry.after)) {
            root.undoStack = stack;
            root.recent = rec;
            root.relist(e => root.sameSetting(e, entry));
            root.undone(entry);
            root.writeChange(bind, v, label, page);
            return;
        }
        root.undoStack = [entry].concat(stack).slice(0, 50);
        root.recent = [entry].concat(rec).slice(0, 12);
        // shell settings apply at once; the window manager store says so once the save holding
        // the change took, and a backend or page store write once it took (it can wait on a prompt)
        if (bind.store === "settings") {
            root.writeChange(bind, v, label, page);
            root.announced(entry);
        } else if (bind.store === "desktop") {
            root.writeChange(bind, v, label, page);
            root.afterDesktopSave(ok => {
                if (ok)
                    root.announced(entry);
            });
        } else {
            root.writeChange(bind, v, label, page, ok => {
                if (ok)
                    root.announced(entry);
            });
        }
    }

    // keys whose last write came from write() itself, not from a change made with set()
    property var actionKeys: ({})
    property bool writingChange: false
    function writeChange(bind, v, label, page, settled) {
        root.writingChange = true;
        try {
            root.write(bind, v, label, page, settled);
        } finally {
            root.writingChange = false;
        }
    }

    // the newest change, or with a page id the newest made on that page
    function undo(page) {
        var entry = page ? root.undoStack.find(e => e.page === page) : root.undoStack[0];
        if (entry)
            root.undoEntry(entry);
    }

    // one stored value can hold many settings, so the row's label tells them apart
    function sameSetting(a, b) {
        return root.keyOf(a.bind) === root.keyOf(b.bind) && a.label === b.label;
    }

    // undo puts back only what its change changed, keeping later changes to the same value
    function reverted(entry) {
        var before = entry.before, after = entry.after, now = root.clone(root.value(entry.bind));
        var key = function (x) { return root.stable(x); };
        if (Array.isArray(after) && Array.isArray(now) && (before === undefined || Array.isArray(before))) {
            // nothing changed since: the list goes back exactly as it was
            if (key(now) === key(after))
                return root.clone(before);
            var was = (before || []).map(key), is = after.map(key);
            // a pure reorder: put the members still there back in their old order
            if (before && was.slice().sort().join("\n") === is.slice().sort().join("\n")) {
                var kept = before.filter(function (x) { return now.some(function (y) { return key(y) === key(x); }); });
                var rest = now.filter(function (x) { return was.indexOf(key(x)) < 0; });
                return root.clone(kept).concat(rest);
            }
            // drop what this change added, put back what it removed at its old place
            var out = now.filter(function (x) { return was.indexOf(key(x)) >= 0 || is.indexOf(key(x)) < 0; });
            (before || []).forEach(function (x, i) {
                if (is.indexOf(key(x)) < 0 && out.map(key).indexOf(key(x)) < 0)
                    out.splice(Math.min(i, out.length), 0, root.clone(x));
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
    // a delegate's modelData is a copy (its arrays turn into lists), so the entry is
    // found again by when it was made and which setting it changed
    function undoEntry(given) {
        if (!given)
            return;
        var match = function (e) {
            return e === given || (e.at === given.at && root.sameSetting(e, given) && e.notice === given.notice && root.same(e.after, given.after));
        };
        var entry = root.undoStack.find(match) || root.recent.find(match) || given;
        root.undoStack = root.undoStack.filter(function (e) { return e !== entry; });
        root.recent = root.recent.filter(function (e) { return e !== entry; });
        // a page reset is several settings written at once; undo puts each back
        if (entry.parts) {
            entry.parts.forEach(function (p) {
                root.relist(e => root.sameSetting(e, p));
                var back = root.reverted(p);
                if (back !== undefined || p.bind.store !== "settings")
                    root.write(p.bind, back, p.label, entry.page);
            });
            root.undone(entry);
            return;
        }
        root.relist(e => root.sameSetting(e, entry));
        var back = root.reverted(entry);
        // an absent key is written back as absent: the desktop save drops it and a page
        // store unsets it; shell settings have no unset, so they keep the value
        if (back !== undefined || entry.bind.store !== "settings")
            root.write(entry.bind, back, entry.label, entry.page);
        root.undone(entry);
    }

    function write(bind, v, label, page, settled) {
        root.setError(bind, "", undefined);
        var ak = root.keyOf(bind);
        if (root.writingChange)
            delete root.actionKeys[ak];
        else
            root.actionKeys[ak] = true;
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
            root.runCommand(bind, v, label, page, settled);
            break;
        default:
            var h = root.stores[bind.store];
            if (!h)
                break;
            h.write(bind.key, v, function (ok, err) {
                if (!ok)
                    root.setError(bind, err || "The change did not apply.", v, label, page);
                if (settled)
                    settled(ok);
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
        if (root.saving) {
            root.desktopSaveAgain = true;
            return;
        }
        root.desktopInFlight = root.desktopPending;
        root.desktopPending = ({});
        root.savingDraft = root.clone(root.desktopDraft);
        root.saving = true;
        root.saveSeq++;
        // a save that hangs ends as a failure after 10 s, so no control shows a value that is not real
        root.run(["ryoku-hub", "desktop", "save", JSON.stringify(root.savingDraft)], function (ok, out, err, code, late) {
            root.saving = false;
            if (late)
                root.desktopSaved(false, I18n.tr("The window manager did not answer in time. Retry, and if it fails again, check that the window manager is running."), true);
            else
                root.desktopSaved(ok, err.trim(), code === -1);
        }, { timeout: 10000 });
    }

    // whole: the message already says what happened and what to do (a missing program, a hang)
    function desktopSaved(ok, message, whole) {
        var sent = root.desktopInFlight;
        root.desktopInFlight = ({});
        if (ok) {
            root.desktopCommitted = root.savingDraft;
        } else {
            // the old value stays on screen and live; then the store is re-read, since the save
            // may have reached disk before the window manager refused it, and disk is what lasts
            var d = root.clone(root.desktopDraft);
            var reason = whole ? message : root.desktopReason(message);
            for (var path in sent) {
                if (root.desktopPending[path])
                    continue;
                root.pathSet(d, path, root.clone(root.pathGet(root.desktopCommitted, path)));
                root.setError(sent[path].bind, reason, sent[path].value, sent[path].label, sent[path].page);
            }
            root.desktopDraft = d;
            root.desktopReloadWanted = true;
            restoreProc.running = true;
        }
        root.revision++;
        var due = root.saveWaiters.filter(w => w.seq <= root.saveSeq);
        root.saveWaiters = root.saveWaiters.filter(w => w.seq > root.saveSeq);
        due.forEach(w => w.done(ok, ok ? "" : reason));
        if (root.desktopSaveAgain || Object.keys(root.desktopPending).length > 0) {
            root.desktopSaveAgain = false;
            root.saveDesktop();
        } else if (root.desktopReloadWanted) {
            root.reloadDesktop();
        }
        root.checkFlushed();
    }

    // the lead, the window manager's own reason as a sentence of its own (dropped when it only
    // repeats the lead), then the next step
    function desktopReason(message) {
        var lead = I18n.tr("The window manager did not take this change.");
        var next = I18n.tr("Retry, and if it fails again, check that the window manager is running.");
        var why = String(message || "").trim().replace(/^ryoku-hub:\s*/, "")
            .replace(/^the window manager refused the settings:?\s*/i, "").replace(/[.\s]+$/, "");
        if (why === "")
            return lead + " " + next;
        return lead + " " + why.charAt(0).toUpperCase() + why.slice(1) + ". " + next;
    }

    // a page store that edits the window manager store through Changes calls this
    // after its edit: done(ok, error) arrives once the save holding that edit settled
    property int saveSeq: 0
    property var saveWaiters: []
    function afterDesktopSave(done) {
        if (!root.desktopReady)
            done(false, "The window manager settings are not loaded yet. Try again in a moment.");
        else if (Object.keys(root.desktopPending).length > 0 || saveDelay.running)
            root.saveWaiters = root.saveWaiters.concat([{ seq: root.saveSeq + 1, done: done }]);
        else if (root.saving)
            root.saveWaiters = root.saveWaiters.concat([{ seq: root.saveSeq, done: done }]);
        else
            done(true, "");
    }

    // write text through a FileView; done(ok, error) arrives exactly once. Writes to
    // one view run one after another, and one that leaves the file as it is answers at
    // once (FileView then never reports a save)
    property var textJobs: []
    // views re-reading their file after a failed save: FileView then holds the attempted
    // text, not the disk's, so the next write waits for the re-read
    property var textRereads: []
    function writeText(view, text, done) {
        root.textJobs = root.textJobs.concat([{ view: view, text: text, done: done, running: false }]);
        root.nextText(view);
    }

    // after a failed save FileView keeps the text it tried to write, and reload() does
    // not drop it; setting the path again makes it read the disk afresh
    function reread(view) {
        root.textRereads = root.textRereads.concat([view]);
        var watch = rereadWatch.createObject(root, { target: view });
        var cap = deadlineTimer.createObject(root, { interval: 2000 });
        var over = false;
        var back = function () {
            if (over)
                return;
            over = true;
            cap.destroy();
            watch.destroy();
            root.textRereads = root.textRereads.filter(v => v !== view);
            root.nextText(view);
        };
        watch.settled.connect(back);
        cap.triggered.connect(back);
        cap.start();
        // after the failed save has finished unwinding, and in two steps so the path really changes
        var path = view.path;
        Qt.callLater(function () {
            view.path = "";
            Qt.callLater(function () { view.path = path; });
        });
    }

    Component {
        id: rereadWatch
        Connections {
            id: watch
            signal settled()
            function onLoaded() { watch.settled(); }
            function onLoadFailed(error) { watch.settled(); }
        }
    }

    function nextText(view) {
        var mine = root.textJobs.filter(j => j.view === view);
        if (mine.length === 0 || mine[0].running || root.textRereads.indexOf(view) >= 0)
            return;
        var job = mine[0];
        var saved = null, failed = null;
        var finish = function (ok, error) {
            if (!job.running)
                return;
            job.running = false;
            if (job.connected) {
                view.saved.disconnect(saved);
                view.saveFailed.disconnect(failed);
            }
            root.textJobs = root.textJobs.filter(j => j !== job);
            job.done(ok, error);
            root.nextText(view);
        };
        saved = function () { finish(true, ""); };
        failed = function (error) {
            root.reread(view);
            finish(false, "The file could not be saved: " + FileViewError.toString(error));
        };
        job.running = true;
        if (view.loaded && view.text() === job.text) {
            finish(true, "");
            return;
        }
        job.connected = true;
        view.saved.connect(saved);
        view.saveFailed.connect(failed);
        view.setText(job.text);
    }

    // re-read the store after something outside the Hub wrote it, once the
    // Hub's own edits have settled; edits made meanwhile stay on top
    property bool desktopReloadWanted: false
    function reloadDesktop() {
        if (root.saving || saveDelay.running || Object.keys(root.desktopPending).length > 0) {
            root.desktopReloadWanted = true;
            return;
        }
        root.desktopReloadWanted = false;
        desktopGet.running = false;
        desktopGet.running = true;
    }

    Process {
        id: desktopGet
        command: ["ryoku-hub", "desktop", "get"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var o = JSON.parse(this.text);
                    var d = JSON.parse(this.text);
                    for (var path in root.desktopPending)
                        root.pathSet(d, path, root.clone(root.desktopPending[path].value));
                    root.desktopCommitted = o;
                    root.desktopDraft = d;
                    root.desktopReady = true;
                    root.revision++;
                } catch (e) {
                    // a failed read keeps what was there; the first one leaves the store not ready
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

    // desktop save in flight, and the draft it sent
    property bool saving: false
    property var savingDraft: ({})

    // backend commands: get and set argv per row

    function readCommand(bind) {
        var k = root.keyOf(bind);
        root.run(bind.get, function (ok, out) {
            var next = Object.assign({}, root.commandValues);
            var v = out.trim();
            try { v = JSON.parse(v); } catch (e) {}
            next[k] = v;
            root.commandValues = next;
            root.revision++;
        });
    }

    function runCommand(bind, v, label, page, settled) {
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
        root.commandsInFlight++;
        root.run(argv, function (ok, out, err) {
            if (!ok)
                root.setError(bind, err.trim() || "The change did not apply.", v, label, page);
            root.readCommand(bind);
            root.commandsInFlight--;
            if (settled)
                settled(ok);
        });
    }
    property int commandsInFlight: 0

    // run argv once; done(ok, out, err, code, late) is called exactly once: when it exits, when
    // the program cannot start (not installed, code -1), or after options.timeout ms, when
    // it is stopped (code -1, late true). options.env adds environment variables
    function run(argv, done, options) {
        var o = options || {};
        var props = { command: argv, deadline: o.timeout || 0 };
        if (o.env)
            props.environment = o.env;
        var p = commandProc.createObject(root, props);
        p.finished.connect(function (ok, out, err, code) {
            var late = p.timedOut;
            p.destroy();
            if (done)
                done(ok, out, err, code, late);
        });
        p.running = true;
        return p;
    }

    Component {
        id: commandProc
        Process {
            id: proc
            property int deadline: 0
            property bool didStart: false
            property bool over: false
            property bool timedOut: false
            signal finished(bool ok, string out, string err, int code)
            function finish(ok, out, err, code) {
                if (proc.over)
                    return;
                proc.over = true;
                limit.stop();
                proc.finished(ok, out, err, code);
            }
            stdout: StdioCollector { id: cout }
            stderr: StdioCollector { id: cerr }
            onStarted: {
                proc.didStart = true;
                if (proc.deadline > 0)
                    limit.restart();
            }
            onExited: (code, status) => {
                // a script whose interpreter or helper is gone exits 126/127 with the shell's own words
                if ((code === 126 || code === 127) && /bad interpreter|command not found|cannot execute|No such file or directory/.test(cerr.text))
                    proc.finish(false, cout.text, root.missingText(String(proc.command[0])), -1);
                else
                    proc.finish(code === 0, cout.text, cerr.text, code);
            }
            // a program that cannot start reports no exit, only that it stopped running
            onRunningChanged: {
                if (!proc.running && !proc.didStart)
                    proc.finish(false, "", root.missingText(String(proc.command[0])), -1);
            }
            property Timer limit: Timer {
                interval: Math.max(1, proc.deadline)
                onTriggered: {
                    proc.timedOut = true;
                    proc.running = false;
                    proc.finish(false, cout.text, I18n.tr("The %1 program did not finish in time. Try again.").replace("%1", String(proc.command[0]).replace(/^.*\//, "")), -1);
                }
            }
        }
    }

    // Ryoku ships its own tools, so a missing one means an incomplete install
    function missingText(program) {
        var name = program.replace(/^.*\//, "");
        // the name never starts the sentence, so it reads right after other words
        return /^ryo/.test(name)
            ? I18n.tr("The %1 tool is missing from this Ryoku install, so this could not run. Update Ryoku, then try again.").replace("%1", name)
            : I18n.tr("The %1 program is not installed, so this could not run. Install it, then try again.").replace("%1", name);
    }

    // a daemon call with a deadline; done(ok, error, result) is called exactly once, also
    // when the reply never comes (the daemon is down) or arrives after the deadline
    function call(method, args, done, timeout) {
        var over = false;
        var timer = deadlineTimer.createObject(root, { interval: timeout || 10000 });
        var finish = function (ok, error, result) {
            if (over)
                return;
            over = true;
            timer.stop();
            timer.destroy();
            if (done)
                done(ok, error, result);
        };
        var id = Bus.call(method, args || {}, function (ok, result, error) {
            finish(ok === true, String(error || ""), result);
        });
        timer.triggered.connect(function () {
            Bus.cancel(id);
            finish(false, I18n.tr("The desktop service did not answer. Try again once it is running."));
        });
        timer.start();
    }

    Component {
        id: deadlineTimer
        Timer {}
    }

    // quit path: send everything pending, then call back once it settled; a
    // page store may offer flush() and an unsettled flag (touch() when it settles)
    readonly property bool unsettled: {
        root.revision;
        return Settings.writesUnsettled || root.saving || saveDelay.running
            || Object.keys(root.desktopPending).length > 0 || root.commandsInFlight > 0
            || Object.keys(root.stores).some(n => root.stores[n] && root.stores[n].unsettled === true);
    }
    onUnsettledChanged: root.checkFlushed()

    function flush(callback) {
        root.flushCallback = callback;
        Settings.flushWrites();
        previewThrottle.stop();
        root.saveDesktop();
        for (var n in root.stores)
            if (root.stores[n] && typeof root.stores[n].flush === "function")
                root.stores[n].flush();
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
