import QtQuick
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../.."
import "../../Singletons"
import "../../ShellIndex.js" as ShellIndex
import "running.js" as Running

// bar styles and complete desktops: installed ones from the shells index, the rest from the catalogue
Item {
    id: root

    readonly property var index: indexFile.index
    property var barstyles: []
    property var subshells: []
    property string busyId: ""
    property string busyVerb: ""
    property var failed: null
    // the install or update Cancel stopped: { id, verb }
    property var stopped: null
    property bool cancelling: false
    // a slow install shows its progress and a way to stop it
    property bool slow: false
    // the running install; it lives in Changes, so leaving the page does not stop it
    property var installProc: null
    readonly property var life: ({ alive: true })
    Component.onDestruction: root.life.alive = false
    // an install started before the page last closed is still this page's to show and to stop
    Component.onCompleted: {
        var run = Running.current;
        if (!run || run.over || !run.proc)
            return;
        root.busyId = run.id;
        root.busyVerb = run.verb;
        root.installProc = run.proc;
        var life = root.life;
        run.proc.finished.connect(function (ok, out, err, code) {
            if (life.alive)
                root.installed(ok, err, code);
        });
        slowTimer.interval = Math.max(1, 10000 - (Date.now() - run.started));
        slowTimer.restart();
    }

    readonly property string active: {
        Settings.revision;
        return String(Settings.get("barStyle") || "") || root.index.fallback;
    }
    readonly property var styleBind: ({ store: "settings", key: "barStyle", fallback: root.index.fallback, after: "shells commit" })
    readonly property var cards: root.cardModel()
    readonly property var activeCard: root.card(root.active)

    function tierOf(value) {
        var tier = String(value || "");
        return tier === "light" || tier === "standard" || tier === "heavy" ? tier : "";
    }

    function catalogueCards(text, category) {
        try {
            var catalog = JSON.parse(text || "{}");
            return (catalog.items || []).filter(item => item.category === category).map(item => ({
                id: item.id,
                category: category,
                name: item.name || item.id,
                summary: item.summary || item.description || "",
                art: item.art || "",
                installed: item.installed === true,
                version: item.version || "",
                installedVersion: item.installedVersion || "",
                updateAvailable: item.updateAvailable === true,
                unavailable: item.unavailable === true,
                unavailableEdition: item.unavailableEdition === true,
                unavailableReason: item.unavailableReason || "",
                requiredWindowManager: item.requiredWindowManager || "",
                downloadPaused: item.downloadPaused === true,
                downloadPauseReason: item.downloadPauseReason || "",
                resourceTier: root.tierOf(item.metadata && item.metadata.resourceTier),
                source: item.metadata && item.metadata.shipped === true ? "shipped" : "store",
                settings: item.settings || ({})
            }));
        } catch (e) {
            return [];
        }
    }

    // fixed order, never by use: installed styles, installed desktops, store offer, blocked; each A to Z
    function cardModel() {
        var listed = {};
        var catalogue = root.subshells.concat(root.barstyles);
        for (var k = 0; k < catalogue.length; k++)
            listed[catalogue[k].id] = catalogue[k];
        var styles = [], desktops = [], offered = [], blocked = [], seen = {};
        var local = ShellIndex.items(root.index, ["subshells", "barstyles"]);
        for (var i = 0; i < local.length; i++) {
            var entry = local[i];
            if (seen[entry.id])
                continue;
            seen[entry.id] = true;
            var base = listed[entry.id] ? Object.assign({}, listed[entry.id]) : {
                id: entry.id, category: entry.category, name: entry.name || entry.id,
                summary: entry.summary || "", art: "", version: "", updateAvailable: false,
                unavailable: false, unavailableEdition: false, unavailableReason: "",
                requiredWindowManager: "", downloadPaused: false, downloadPauseReason: "",
                resourceTier: root.tierOf(entry.resourceTier)
            };
            base.installed = true;
            base.source = entry.source || "store";
            base.installedVersion = entry.version || base.installedVersion || "";
            if (!base.version)
                base.version = base.installedVersion;
            base.settings = entry.settings || base.settings || ({});
            (base.category === "subshells" ? desktops : styles).push(base);
        }
        for (var j = 0; j < catalogue.length; j++) {
            var item = catalogue[j];
            if (seen[item.id])
                continue;
            seen[item.id] = true;
            (item.unavailable || item.downloadPaused ? blocked : offered).push(item);
        }
        var byName = (a, b) => a.name.localeCompare(b.name);
        var byKind = (a, b) => a.category === b.category ? byName(a, b) : (a.category === "barstyles" ? -1 : 1);
        return styles.sort(byName).concat(desktops.sort(byName)).concat(offered.sort(byKind)).concat(blocked.sort(byKind));
    }

    function card(id) {
        for (var i = 0; i < root.cards.length; i++)
            if (root.cards[i].id === id)
                return root.cards[i];
        return null;
    }

    function wearable(c) {
        return !!c && c.installed && !c.unavailable;
    }

    function updatable(c) {
        return !!c && c.installed && c.updateAvailable && c.version.length > 0 && !c.unavailable && !c.downloadPaused;
    }

    function hasOwnWindow(c) {
        return !!c && !!c.settings && String(c.settings.ipc || "") !== "" && String(c.settings.call || "") !== "";
    }

    function editorOf(c) {
        return c && c.settings ? String(c.settings.hubEditor || "") : "";
    }

    function managerName(c) {
        var wm = String(c.requiredWindowManager || "");
        return wm.length > 0 ? wm.charAt(0).toUpperCase() + wm.slice(1) : "";
    }

    function blockedReason(c) {
        if (c.installed && !c.unavailable)
            return "";
        if (c.unavailable) {
            if (c.unavailableEdition)
                return I18n.tr("Not available on this system");
            var wm = root.managerName(c);
            return wm.length > 0 ? I18n.tr("Needs %1").arg(wm) : I18n.tr("Cannot run here");
        }
        if (c.downloadPaused)
            return I18n.tr("Not offered right now");
        return "";
    }

    function statusOf(c) {
        var desktop = c.category === "subshells";
        if (root.busyId === c.id)
            return root.busyVerb === "update" ? I18n.tr("Updating") : I18n.tr("Installing");
        if (c.id === root.active && root.wearable(c))
            return I18n.tr("In use");
        if (root.updatable(c))
            return I18n.tr("Update available");
        // a complete desktop says so; its plaque tells installed from offered
        if (desktop)
            return I18n.tr("Complete desktop");
        return c.installed ? I18n.tr("Installed") : I18n.tr("Available in RyoStore");
    }

    // an update comes first; the options of the style in use stay in their own section
    function actionOf(c) {
        if (root.busyId !== "" || root.blockedReason(c) !== "")
            return "";
        if (root.updatable(c))
            return "update";
        if (c.id === root.active && root.wearable(c)) {
            if (root.editorOf(c) !== "")
                return "options";
            return root.hasOwnWindow(c) ? "settings" : "";
        }
        return c.installed ? "use" : "install";
    }

    function actionText(action) {
        switch (action) {
        case "use": return I18n.tr("Use");
        case "install": return I18n.tr("Install");
        case "update": return I18n.tr("Update");
        case "options": return I18n.tr("Options");
        case "settings": return I18n.tr("Settings");
        }
        return "";
    }

    readonly property var tiles: root.cards.map(c => {
        var action = root.actionOf(c);
        return {
            id: c.id,
            title: c.name,
            status: root.statusOf(c),
            resource: c.resourceTier,
            source: c.art,
            current: c.id === root.active && root.wearable(c),
            unavailable: root.blockedReason(c),
            busy: root.busyId === c.id,
            action: action,
            actionText: root.actionText(action),
            actionKind: action === "install" || action === "update" ? "line" : "tonal"
        };
    })

    // switch the desktop the way the old studio committed it: barStyle, then shells commit
    function use(id) {
        var c = root.card(id);
        if (!root.wearable(c) || id === root.active)
            return;
        Changes.set(root.styleBind, id, "Bar style", "bar", I18n.tr("Now using %1").arg(c.name));
    }

    function install(id, verb) {
        var c = root.card(id);
        if (!c || root.busyId !== "")
            return;
        root.failed = null;
        root.stopped = null;
        root.cancelling = false;
        root.busyId = id;
        root.busyVerb = verb;
        var life = root.life;
        var finish = Running.finish;
        var run = { id: id, verb: verb, proc: null, started: Date.now(), over: false };
        // a missing ryostore ends in an error at once instead of a tile busy forever
        run.proc = Changes.run(["ryostore", "install", c.category, c.id], function (ok, out, err, code) {
            run.over = true;
            finish(run);
            if (life.alive)
                root.installed(ok, err, code);
        });
        if (run.over)
            return;
        Running.start(run);
        root.installProc = run.proc;
        slowTimer.interval = 10000;
        slowTimer.restart();
    }

    function installed(ok, err, code) {
        root.installProc = null;
        if (root.cancelling)
            root.stopped = { id: root.busyId, verb: root.busyVerb };
        else if (!ok)
            root.failed = { id: root.busyId, verb: root.busyVerb, message: err.trim(), missing: code === -1 };
        slowTimer.stop();
        root.slow = false;
        root.cancelling = false;
        root.busyId = "";
        root.busyVerb = "";
        root.refresh();
    }

    function cancel() {
        if (!root.installProc || !root.installProc.running)
            return;
        root.cancelling = true;
        root.installProc.signal(15);
    }

    function openOwnWindow(c) {
        if (!root.hasOwnWindow(c))
            return;
        Spawn.run(root.ownWindowCommand(c));
    }

    function ownWindowCommand(c) {
        var args = Array.isArray(c.settings.args) ? c.settings.args.map(String) : [];
        return ["ryoku-shell", "ipc", "call", String(c.settings.ipc), String(c.settings.call)].concat(args);
    }

    function refresh() {
        barstylesRead.running = true;
        subshellsRead.running = true;
    }

    ShellIndexFile { id: indexFile }

    Process {
        id: barstylesRead
        command: ["ryostore", "catalog", "--category", "barstyles"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: root.barstyles = root.catalogueCards(this.text, "barstyles")
        }
    }

    Process {
        id: subshellsRead
        command: ["ryostore", "catalog", "--category", "subshells"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: root.subshells = root.catalogueCards(this.text, "subshells")
        }
    }

    Timer {
        id: slowTimer
        interval: 10000
        onTriggered: root.slow = root.installProc !== null
    }
}
