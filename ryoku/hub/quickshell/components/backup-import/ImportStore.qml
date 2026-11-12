import QtQuick
import Quickshell
import Ryoku.Ui.Singletons
import "../../Singletons"

// the imports in place as a Changes store, so an import has its notice and its
// Undo on Home: the value names the newest import still applied; writing a name
// handed over with choices applies them, writing an earlier name (or "") undoes
// every import after it, newest first
QtObject {
    id: root

    // oldest first: { name, ts, result }
    property var applied: []
    // name -> the choices for an import not applied yet
    property var pending: ({})
    // apply | undo
    property string busy: ""
    // whether another desktop's configuration was found on this computer; undefined until
    // the import has looked, read as "found" (the found row and its search result follow it)
    property var found: undefined
    // the choices go to the importer as a file in the private runtime dir
    readonly property string decisionsPath: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-import-decisions.json"

    signal applyFinished(string name, var result, string error)
    signal undoFinished(var result, string error)

    function read(key) {
        if (key === "found")
            return root.found;
        return root.applied.length > 0 ? root.applied[root.applied.length - 1].name : "";
    }

    function hand(name, payload) {
        var m = Object.assign({}, root.pending);
        m[name] = payload;
        root.pending = m;
    }

    function isApplied(name) {
        return root.applied.some(a => a.name === name);
    }

    function write(key, value, done) {
        if (root.busy !== "") {
            done(false, I18n.tr("An import is still running. Try again when it finishes."));
            return;
        }
        var target = String(value || "");
        if (target !== "" && root.pending[target] !== undefined)
            root.apply(target, done);
        else
            root.undoTo(target, done);
    }

    function lastLine(text) {
        var lines = String(text || "").trim().split("\n");
        return lines[lines.length - 1].replace(/^ryoku-hub:\s*/, "").replace(/\.+$/, "");
    }

    // a job that could not start (code -1, the missing words) or was stopped at its limit
    function stopped(argv, err, limitText) {
        return err === Changes.missingText(argv[0])
            ? I18n.tr("The import tool is missing from this Ryoku install. Update Ryoku, then try again.")
            : limitText;
    }

    function apply(name, done) {
        root.busy = "apply";
        var fail = function (message) {
            root.busy = "";
            root.applyFinished(name, null, message);
            done(false, message);
        };
        Changes.run(["sh", "-c", "printf '%s' \"$1\" > \"$2\"", "sh", root.pending[name], root.decisionsPath], function (wrote, o, werr) {
            if (!wrote) {
                fail(I18n.tr("Your choices could not be handed to the importer: %1. Nothing was changed; try again.")
                    .replace("%1", root.lastLine(werr) || I18n.tr("it stopped early")));
                return;
            }
            var argv = ["ryoku-hub", "import", "apply", root.decisionsPath];
            Changes.run(argv, function (ok, out, err, code) {
                if (!ok) {
                    fail(code === -1
                        ? root.stopped(argv, err, I18n.tr("Applying the import took too long, so Settings stopped waiting. Its backup is in ~/.config/ryoku/import-backups; try again."))
                        : I18n.tr("The import did not apply: %1. Nothing was changed; try again.")
                            .replace("%1", root.lastLine(err) || I18n.tr("it stopped early")));
                    return;
                }
                var result = {};
                try { result = JSON.parse(out) || {}; } catch (e) { result = {}; }
                var p = Object.assign({}, root.pending);
                delete p[name];
                root.pending = p;
                root.applied = root.applied.concat([{ name: name, ts: String(result.ts || ""), result: result }]);
                root.busy = "";
                Changes.reloadDesktop();
                Changes.touch();
                root.applyFinished(name, result, "");
                done(true, "");
            }, { timeout: 120000 });
        }, { timeout: 10000 });
    }

    // newest first, one backup at a time, so an older import never restores over a newer one
    function undoTo(target, done) {
        var keep = target === "" ? 0 : root.applied.findIndex(a => a.name === target) + 1;
        if (target !== "" && keep === 0) {
            done(false, I18n.tr("That import is no longer in place, so there is nothing to undo."));
            return;
        }
        var restored = [];
        var step = function () {
            if (root.applied.length <= keep) {
                root.busy = "";
                Changes.reloadDesktop();
                Changes.touch();
                root.undoFinished({ restored: restored }, "");
                done(true, "");
                return;
            }
            var last = root.applied[root.applied.length - 1];
            var argv = ["ryoku-hub", "import", "undo"].concat(last.ts !== "" ? [last.ts] : []);
            Changes.run(argv, function (ok, out, err, code) {
                if (!ok) {
                    root.busy = "";
                    Changes.touch();
                    const message = code === -1
                        ? root.stopped(argv, err, I18n.tr("Putting the backup back took too long, so Settings stopped waiting. Its backup is still in ~/.config/ryoku/import-backups; try again."))
                        : I18n.tr("The import could not be undone: %1. Its backup is still in ~/.config/ryoku/import-backups; try again.")
                            .replace("%1", root.lastLine(err) || I18n.tr("it stopped early"));
                    root.undoFinished(null, message);
                    done(false, message);
                    return;
                }
                var r = {};
                try { r = JSON.parse(out) || {}; } catch (e) { r = {}; }
                (r.restored || []).forEach(f => { if (restored.indexOf(f) < 0) restored.push(f); });
                root.applied = root.applied.slice(0, -1);
                step();
            }, { timeout: 120000 });
        };
        root.busy = "undo";
        step();
    }

    Component.onCompleted: Changes.registerStore("import", root)
}
