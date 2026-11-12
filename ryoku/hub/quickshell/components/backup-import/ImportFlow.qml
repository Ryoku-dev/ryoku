import QtQuick
import Quickshell
import Ryoku.Ui.Singletons
import "../../Singletons"
import "../../Combos.js" as Combos

// the import's state: `ryoku-hub import` detects and scans here, the import
// store applies and undoes; this holds the scan, the choices and the step, never
// parsing config
Item {
    id: root

    // source | review | resolve | preview | done
    property string step: "source"
    property bool scanning: false
    property string error: ""
    property string lastSource: ""
    property var detected: ({})
    property bool detectDone: false
    property var scan: null
    property var include: ({})
    property var decisions: ({})
    property var applied: null
    property var undoneResult: null
    property bool undone: false
    property var scanJob: null
    property bool scanCancelled: false
    // apply and undo go through the import store, so an import has its notice and Undo on Home
    readonly property var bind: ({ store: "import", key: "applied" })
    readonly property var store: Changes.stores["import"] || null
    // the name the import applied last from here goes by in the store
    property string token: ""
    readonly property string busy: root.scanning ? "scan" : root.store ? root.store.busy : ""

    readonly property bool supported: !root.detectDone || root.detected.supported === true
    readonly property bool found: root.detected.found === true
    readonly property string providerName: root.detected.name || ""
    readonly property var apps: root.scan && root.scan.apps ? root.scan.apps.filter(a => a && a.present !== false) : []
    readonly property var conflicts: {
        var out = [];
        root.apps.forEach(a => {
            if (!root.included(a.id))
                return;
            (a.conflicts || []).forEach(c => out.push({ app: a.name, norm: c.norm, combo: c.combo,
                                                         ryoku: c.ryoku || ({}), mine: c.mine || ({}), kind: c.kind || "" }));
        });
        return out;
    }
    readonly property int includedCount: root.apps.filter(a => root.included(a.id)).length
    readonly property int resolvedCount: root.conflicts.filter(c => root.decided(c.norm)).length
    // what goes over as it is: binds and window rules Ryoku takes in are not among them
    readonly property var layered: {
        var out = [];
        root.apps.forEach(a => {
            if (root.included(a.id))
                (a.items || []).forEach(it => {
                    if (it.kind !== "bind" && !(it.ingestable && (it.kind === "windowrule" || it.kind === "layerrule")))
                        out.push({ app: a.name, raw: it.raw || "" });
                });
        });
        return out;
    }
    // what applying adds, as the importer settles each clash: a shortcut left to Ryoku is dropped,
    // a used or moved one keeps one bind of its keys (two for a moved duplicate)
    readonly property var forecast: {
        var binds = 0, rules = 0, unbinds = 0;
        root.apps.forEach(a => {
            if (!root.included(a.id))
                return;
            var onKeys = {};
            (a.items || []).forEach(it => {
                if (it.kind === "bind" && it.ingestable) {
                    binds++;
                    var n = Combos.normKeys(it.combo || "");
                    onKeys[n] = (onKeys[n] || 0) + 1;
                } else if ((it.kind === "windowrule" || it.kind === "layerrule") && it.ingestable) {
                    rules++;
                }
            });
            (a.conflicts || []).forEach(c => {
                var n = onKeys[c.norm] || 0;
                var d = root.decided(c.norm) ? root.decisions[c.norm] : "ryoku";
                var kept = d === "ryoku" ? (c.kind === "duplicate" ? 1 : 0) : (d === "mine" ? 1 : (c.kind === "duplicate" ? 2 : 1));
                binds -= Math.max(0, n - kept);
                if (d === "mine" && c.kind === "shipped" && n > 0)
                    unbinds++;
            });
        });
        return { binds: binds, rules: rules, unbinds: unbinds };
    }

    function included(id) {
        return root.include[id] !== false;
    }

    function setIncluded(id, on) {
        var m = Object.assign({}, root.include);
        m[id] = on;
        root.include = m;
    }

    // a move waiting for its new keys is not settled yet: applying it would keep Ryoku's
    function decided(norm) {
        var d = root.decisions[norm];
        if (d && typeof d === "object")
            return !!d.remap;
        return d !== undefined && d !== null && d !== "";
    }

    function decide(norm, value) {
        var m = Object.assign({}, root.decisions);
        m[norm] = value;
        root.decisions = m;
    }

    // a changed shortcut that lands on another clashing one still clashes
    function remapClashes(norm) {
        var d = root.decisions[norm];
        if (!d || !d.remap)
            return false;
        var nk = Combos.normKeys(d.remap);
        return root.conflicts.some(c => c.norm !== norm && c.norm === nk);
    }

    // a job that could not start (code -1, the missing words) or was stopped at its limit
    function stopped(argv, err, limitText) {
        return err === Changes.missingText(argv[0])
            ? I18n.tr("The import tool is missing from this Ryoku install. Update Ryoku, then try again.")
            : limitText;
    }

    // a failed or hung look leaves the import usable; trying it shows what is wrong
    function detect() {
        Changes.run(["ryoku-hub", "import", "detect"], function (ok, out) {
            var d = null;
            try { d = ok ? JSON.parse(out.trim() || "{}") : null; } catch (e) { d = null; }
            d = d || ({ supported: true });
            // search offers the found row only while there is one; the store outlives the page
            if (root.store) {
                root.store.found = d.found === true;
                Changes.touch();
            }
            root.detected = d;
            root.detectDone = true;
        }, { timeout: 15000 });
    }

    function choose(source) {
        source = String(source || "").trim();
        if (source === "" || root.busy !== "")
            return;
        root.error = "";
        root.scan = null;
        root.applied = null;
        root.undone = false;
        root.token = "";
        root.include = ({});
        root.decisions = ({});
        root.lastSource = source;
        root.scanning = true;
        root.scanCancelled = false;
        var argv = ["ryoku-hub", "import", "scan", source];
        root.scanJob = Changes.run(argv, function (ok, out, err, code) {
            root.scanJob = null;
            root.scanning = false;
            if (root.scanCancelled) {
                root.scanCancelled = false;
                return;
            }
            if (!ok) {
                var why = root.lastLine(err);
                root.error = code === -1
                    ? root.stopped(argv, err, I18n.tr("Looking through %1 took too long, so Settings stopped. Check the folder or address, then try again.").replace("%1", source))
                    : /^source not found/.test(why)
                    ? I18n.tr("There is no folder at %1. Check the path, then try again.").replace("%1", source)
                    : /^clone /.test(why)
                    ? I18n.tr("%1 could not be copied. Check the address and your connection, then try again.").replace("%1", source)
                    : I18n.tr("Could not look through %1: %2. Check the folder or address, then try again.")
                        .replace("%1", source).replace("%2", why || I18n.tr("it stopped early"));
                return;
            }
            try {
                root.scan = JSON.parse(out);
                root.step = "review";
            } catch (e) {
                root.error = I18n.tr("The importer answered with something Settings could not read. Try again.");
            }
        }, { timeout: 120000 });
    }

    function retry() {
        if (root.busy !== "")
            return;
        root.error = "";
        if (root.step !== "preview" && root.step !== "done")
            root.choose(root.lastSource);
        else if (Changes.error(root.bind) !== "")
            Changes.retry(root.bind);
        else if (root.step === "preview")
            root.apply();
        else
            root.undo();
    }

    function cancel() {
        if (root.busy !== "scan" || !root.scanJob)
            return;
        root.scanCancelled = true;
        root.scanJob.running = false;
    }

    function next() {
        if (root.step === "review")
            root.step = root.conflicts.length > 0 ? "resolve" : "preview";
        else if (root.step === "resolve")
            root.step = "preview";
        else if (root.step === "preview")
            root.apply();
    }

    function back() {
        if (root.step === "review")
            root.restart();
        else if (root.step === "resolve")
            root.step = "review";
        else if (root.step === "preview")
            root.step = root.conflicts.length > 0 ? "resolve" : "review";
    }

    function restart() {
        root.token = "";
        root.scan = null;
        root.applied = null;
        root.undoneResult = null;
        root.undone = false;
        root.error = "";
        root.include = ({});
        root.decisions = ({});
        root.step = "source";
    }

    function apply() {
        if (!root.store || root.busy !== "")
            return;
        var apps = {};
        root.apps.forEach(a => apps[a.id] = { include: root.included(a.id) });
        var conflicts = {};
        for (var norm in root.decisions)
            if (root.decided(norm))
                conflicts[norm] = root.decisions[norm];
        root.error = "";
        root.token = "import-" + Date.now();
        root.store.hand(root.token, JSON.stringify({ source: root.scan ? root.scan.source : "", apps: apps, conflicts: conflicts }));
        Changes.set(root.bind, root.token, I18n.tr("Import"), "backup-import", I18n.tr("Your configuration was imported"));
    }

    // the same Undo as the notice and Home's, so all three agree
    function undo() {
        if (!root.store || root.busy !== "")
            return;
        root.error = "";
        var entry = Changes.undoStack.find(e => Changes.keyOf(e.bind) === Changes.keyOf(root.bind) && e.after === root.token);
        if (entry) {
            Changes.undoEntry(entry);
            return;
        }
        var at = root.store.applied.findIndex(a => a.name === root.token);
        if (at >= 0)
            Changes.write(root.bind, at > 0 ? root.store.applied[at - 1].name : "", I18n.tr("Import"), "backup-import");
    }

    Connections {
        target: root.store
        function onApplyFinished(name, result, error) {
            if (name !== root.token)
                return;
            if (error !== "") {
                root.error = error;
                return;
            }
            root.applied = result || ({});
            root.undone = false;
            root.undoneResult = null;
            root.step = "done";
        }
        function onUndoFinished(result, error) {
            if (root.step !== "done" || root.token === "")
                return;
            if (error !== "") {
                root.error = error;
                return;
            }
            if (!root.store.isApplied(root.token)) {
                root.undoneResult = result || ({});
                root.undone = true;
            }
        }
    }

    function reloadDesktop() {
        Settings.send("wm.act", { action: "config.reload", args: [] });
    }

    function lastLine(text) {
        var lines = String(text).trim().split("\n");
        return lines[lines.length - 1].replace(/^ryoku-hub:\s*/, "").replace(/\.+$/, "");
    }

    Component.onCompleted: {
        Changes.ensureStore("import", Qt.resolvedUrl("ImportStore.qml"));
        root.detect();
    }
}
