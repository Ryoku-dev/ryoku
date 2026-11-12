pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io

// The Profile plate's customization, kept in ~/.config/ryoku/profile.json and
// watched, so an edit shows on the plate at once and survives a reopen. A single
// object: preset, heroSide, per-block visibility (blocks.<id>), the hero
// (source/framing/dither), text overrides, and the chosen vitals/specs. Absent or
// empty fields fall back to the plate's built-in defaults, so a missing file
// renders exactly the stock marble plate. Written immediately on every edit, a
// preference, not a Save-gated setting, so it never joins the Hub's dirty diff.
Singleton {
    id: store

    property var data: ({})
    // the outcome of the write in progress, and the error of a failed write
    // whose file has not been read back yet
    property var outcome: null
    property string unsaved: ""

    FileView {
        id: file
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/profile.json"
        blockLoading: true
        // a write finishes inside setText, so each one reports its own outcome:
        // async writes coalesce and report a superseded write as saved
        blockWrites: true
        watchChanges: true
        printErrors: false
        atomicWrites: true
        // reload() re-reads from disk asynchronously; _load() must run in onLoaded
        // (when file.text() is fresh), not inline here, or it reads the stale
        // cached text one write behind and clobbers a just-applied edit.
        onFileChanged: reload()
        onLoaded: {
            store.unsaved = "";
            store._load();
        }
        onLoadFailed: {
            store.unsaved = "";
            store.data = ({});
        }
        onSaved: {
            store.unsaved = "";
            store.outcome = { ok: true, error: "" };
        }
        // the edit never reached disk: report it and show what the file still holds
        onSaveFailed: error => {
            store.unsaved = I18n.tr("The system card could not be saved: %1").arg(FileViewError.toString(error));
            store.outcome = { ok: false, error: store.unsaved };
            reload();
        }
    }

    Component.onCompleted: store._load()

    function _load() {
        var t = file.text();
        if (!t) { store.data = ({}); return; }
        try { store.data = JSON.parse(t) || ({}); } catch (e) { store.data = ({}); }
    }

    // read a dotted path (e.g. "blocks.telemetry", "hero.zoom") defensively;
    // returns `def` when any segment is missing or the value is null / "".
    function get(path, def) {
        var o = store.data;
        var parts = path.split(".");
        for (var i = 0; i < parts.length; i++) {
            if (o === undefined || o === null || typeof o !== "object")
                return def;
            o = o[parts[i]];
        }
        return (o === undefined || o === null || o === "") ? def : o;
    }

    // deep-merge a patch into the store and write through immediately. The merge
    // is reassigned as a fresh reference (a re-parse) so `data`'s binding
    // dependents re-evaluate at once: mutating a var in place keeps the same
    // reference, and QML would not notify. The file write persists it, and
    // done(ok, error), when given, reports that write exactly once.
    function put(patch, done) {
        store._write(store._merge(store.data || ({}), patch), done);
    }

    // replace the whole design in one write; {} is the stock plate
    function replace(value, done) {
        store._write(value && typeof value === "object" ? value : ({}), done);
    }

    function _write(value, done) {
        var json = JSON.stringify(value, null, 2);
        store.data = JSON.parse(json);
        store.outcome = null;
        file.setText(json);
        // no outcome: the file view already held this text, which is on disk
        // unless the failed write that left it there has not been read back yet
        var result = store.outcome || { ok: store.unsaved === "", error: store.unsaved };
        store.outcome = null;
        if (done)
            done(result.ok, result.error);
    }

    function _merge(base, patch) {
        base = (base && typeof base === "object") ? base : ({});
        for (var k in patch) {
            var v = patch[k];
            if (v && typeof v === "object" && !Array.isArray(v))
                base[k] = store._merge(base[k], v);
            else
                base[k] = v;
        }
        return base;
    }

    // wipe back to the stock plate.
    function reset(done) {
        store.replace(({}), done);
    }
}
