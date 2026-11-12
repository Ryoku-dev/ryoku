import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../.."
import "../../Singletons"
import "../../ShellIndex.js" as ShellIndex
import "../../LauncherSettings.js" as LauncherSettings
import "running.js" as Running

// every launcher Super+Space can open: the shipped ones, RyoStore's, and those a bar style brings
Item {
    id: root

    readonly property var bind: ({ store: "launcher", key: "variant" })
    readonly property string launcherRoot: shells.shellRoot + "/modules/launcher"
    property var catalog: ({ variants: [] })
    property bool catalogMissing: false
    property var store: []
    property var storeSettings: LauncherSettings.parse("")
    // what the launcher in use offers is known once RyoStore's list and its settings are read
    property bool storeKnown: false
    property bool settingsKnown: true
    readonly property bool ready: root.storeKnown && root.settingsKnown
    property string busyId: ""
    property string busyVerb: ""
    property var failed: null
    // the install or update Cancel stopped: { id, verb }
    property var stopped: null
    property bool cancelling: false
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

    readonly property string storeRoot: (Quickshell.env("XDG_STATE_HOME") || (Quickshell.env("HOME") + "/.local/state")) + "/ryoku/store"

    // the shipped launchers in Settings' own words, so they read plainly and translate; others keep the catalogue's
    function shippedSummary(v) {
        switch (String(v.id || "")) {
        case "main": return I18n.tr("A compact search bar over a picture card");
        case "hero": return I18n.tr("A large picture header over modes, actions and search results");
        case "okshell": return I18n.tr("A fast list of apps only, in your current colours");
        }
        return String(v.description || "");
    }

    readonly property var shipped: (Array.isArray(root.catalog.variants) ? root.catalog.variants : []).map(v => ({
        id: String(v.id || ""), name: String(v.name || v.id || ""), summary: root.shippedSummary(v),
        kind: "shipped", installed: true, unavailable: false, capabilities: Array.isArray(v.capabilities) ? v.capabilities : [],
        previewUrl: v.preview ? "file://" + root.launcherRoot + "/" + v.preview : "", art: "",
        version: "", installedVersion: "", updateAvailable: false, downloadPaused: false
    })).filter(v => v.id !== "")

    readonly property var contributed: ShellIndex.contributions(shells.index, "launcher").map(entry => ({
        id: entry.id, name: entry.name, summary: entry.description, kind: "contribution", itemName: entry.itemName,
        prepared: entry.prepared === true, installed: true, unavailable: false, capabilities: [],
        previewUrl: ShellIndex.fileUrl(entry.record, entry.preview, shells.shellRoot), art: "",
        version: "", installedVersion: "", updateAvailable: false, downloadPaused: false
    }))

    // fixed order, never by use: shipped, then from bar styles, then RyoStore A to Z
    readonly property var cards: {
        var seen = {}, out = [];
        var add = c => {
            if (seen[c.id])
                return;
            seen[c.id] = true;
            out.push(c);
        };
        root.shipped.forEach(add);
        root.contributed.slice().sort((a, b) => a.name.localeCompare(b.name)).forEach(add);
        root.store.slice().sort((a, b) => a.name.localeCompare(b.name)).forEach(add);
        return out;
    }

    function card(id) {
        for (var i = 0; i < root.cards.length; i++)
            if (root.cards[i].id === id)
                return root.cards[i];
        return null;
    }

    function wearable(c) {
        return !!c && c.installed && !c.unavailable && (c.kind !== "contribution" || c.prepared);
    }

    // the stored choice, or the launcher Super+Space falls back to when it cannot run
    readonly property string active: {
        var wanted = String(Changes.value(root.bind) || "");
        var c = root.card(wanted);
        if (c && (root.wearable(c) || c.kind === "contribution"))
            return wanted;
        var fallback = String(root.catalog["default"] || "");
        return root.card(fallback) ? fallback : (root.cards.length > 0 ? root.cards[0].id : wanted);
    }
    readonly property var activeCard: root.card(root.active)
    readonly property var capabilities: !root.activeCard ? []
        : root.activeCard.kind === "store" ? root.storeSettings.capabilities : root.activeCard.capabilities

    function supports(capability) {
        return root.capabilities.indexOf(capability) >= 0;
    }

    function updatable(c) {
        return !!c && c.kind === "store" && c.installed && c.updateAvailable && c.version.length > 0 && !c.unavailable && !c.downloadPaused;
    }

    function blockedReason(c) {
        if (!c)
            return "";
        if (c.unavailable) {
            if (c.unavailableEdition)
                return I18n.tr("Not available on this system");
            var wm = String(c.requiredWindowManager || "");
            return wm.length > 0 ? I18n.tr("Needs %1").arg(wm.charAt(0).toUpperCase() + wm.slice(1)) : I18n.tr("Cannot run here");
        }
        if (!c.installed && c.downloadPaused)
            return I18n.tr("Not offered right now");
        return "";
    }

    function statusOf(c) {
        if (root.busyId === c.id)
            return root.busyVerb === "update" ? I18n.tr("Updating") : I18n.tr("Installing");
        if (c.id === root.active)
            return I18n.tr("In use");
        if (root.updatable(c))
            return I18n.tr("Update available");
        if (c.kind === "shipped")
            return I18n.tr("Comes with Ryoku");
        if (c.kind === "contribution")
            return I18n.tr("From %1").arg(c.itemName);
        return c.installed ? I18n.tr("Installed") : I18n.tr("Available in RyoStore");
    }

    function actionOf(c) {
        if (root.busyId !== "" || root.blockedReason(c) !== "")
            return "";
        if (c.id === root.active)
            return root.updatable(c) ? "update" : "";
        if (root.updatable(c))
            return "update";
        if (c.kind === "store" && !c.installed)
            return "install";
        return "use";
    }

    function actionText(action) {
        switch (action) {
        case "use": return I18n.tr("Use");
        case "install": return I18n.tr("Install");
        case "update": return I18n.tr("Update");
        }
        return "";
    }

    readonly property var tiles: root.cards.map(c => {
        var action = root.actionOf(c);
        return {
            id: c.id, title: c.name, status: root.statusOf(c), resource: "", source: c.art,
            current: c.id === root.active, unavailable: root.blockedReason(c), busy: root.busyId === c.id,
            action: action, actionText: root.actionText(action), actionKind: action === "use" ? "tonal" : "line"
        };
    })

    function describe(c) {
        if (!c)
            return "";
        var parts = [];
        if (c.summary)
            parts.push(c.summary);
        if (c.kind === "contribution" && !c.prepared)
            parts.push(I18n.tr("It gets ready the first time you choose it; until then the launcher shows Ryoku's own"));
        if (c.unavailable && c.unavailableReason)
            parts.push(I18n.tr(c.unavailableReason));
        else if (!c.installed && c.downloadPaused && c.downloadPauseReason)
            parts.push(I18n.tr(c.downloadPauseReason));
        if (root.updatable(c))
            parts.push(I18n.tr("Version %1 is installed and %2 is ready").arg(c.installedVersion).arg(c.version));
        else if (c.kind === "store" && c.installed && c.installedVersion)
            parts.push(I18n.tr("Version %1 from RyoStore").arg(c.installedVersion));
        else if (c.kind === "store" && !c.installed && c.version)
            parts.push(I18n.tr("Version %1 in RyoStore").arg(c.version));
        return I18n.tr("%1: %2.").arg(c.name).arg(parts.filter(p => p.length > 0).map(p => p.replace(/[.\s]+$/, "")).join(". "));
    }

    function act(id, action) {
        if (action === "use")
            root.use(id);
        else if (action === "install" || action === "update")
            root.install(id, action);
    }

    function use(id) {
        var c = root.card(id);
        if (!c || id === root.active || root.blockedReason(c) !== "")
            return;
        Changes.set(root.bind, id, "Launcher style", "launcher", I18n.tr("Now using %1").arg(c.name));
    }

    function install(id, verb) {
        if (root.busyId !== "")
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
        run.proc = Changes.run(["ryostore", "install", "launchers", id], function (ok, out, err, code) {
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
        storeRead.running = true;
    }

    function cancel() {
        if (!root.installProc || !root.installProc.running)
            return;
        root.cancelling = true;
        root.installProc.signal(15);
    }

    // a RyoStore launcher runs from its installed view; the path is checked before it becomes a file path
    function viewDir(item) {
        var view = String((item.metadata || {}).view || "");
        var pattern = new RegExp("^launcher-views/" + item.id + "/[a-f0-9]{64}$");
        return pattern.test(view) ? root.storeRoot + "/" + view : "";
    }

    function storeCards(text) {
        try {
            var catalog = JSON.parse(text || "{}");
            // a launcher for another window manager that is not installed can neither run nor be fetched here
            return (catalog.items || []).filter(item => item.category === "launchers"
                    && !(item.unavailable === true && item.installed !== true)).map(item => {
                var dir = item.installed === true ? root.viewDir(item) : "";
                return {
                    id: item.id, name: item.name || item.id, summary: item.summary || item.description || "",
                    kind: "store", art: item.art || "", viewDir: dir,
                    previewUrl: dir.length > 0 && (item.metadata || {}).preview === "Preview.qml" ? "file://" + dir + "/Preview.qml" : "",
                    installed: item.installed === true, capabilities: [],
                    version: item.version || "", installedVersion: item.installedVersion || "",
                    updateAvailable: item.updateAvailable === true, unavailable: item.unavailable === true,
                    unavailableEdition: item.unavailableEdition === true, unavailableReason: item.unavailableReason || "",
                    requiredWindowManager: item.requiredWindowManager || "",
                    downloadPaused: item.downloadPaused === true, downloadPauseReason: item.downloadPauseReason || ""
                };
            });
        } catch (e) {
            return [];
        }
    }

    ShellIndexFile { id: shells }

    FileView {
        id: catalogFile
        path: root.launcherRoot + "/catalog.json"
        // read at once, so a search landing knows what the launcher in use offers
        blockLoading: true
        printErrors: false
        onLoaded: {
            try {
                root.catalog = JSON.parse(catalogFile.text());
                root.catalogMissing = false;
            } catch (e) {
                root.catalogMissing = true;
            }
        }
        onLoadFailed: root.catalogMissing = true
    }

    // what a RyoStore launcher lets Settings change, from the settings.json in its view
    FileView {
        id: storeSettingsFile
        path: root.activeCard && root.activeCard.kind === "store" && root.activeCard.viewDir ? root.activeCard.viewDir + "/settings.json" : ""
        printErrors: false
        onPathChanged: {
            root.storeSettings = LauncherSettings.parse("");
            root.settingsKnown = path === "";
        }
        onLoaded: {
            root.storeSettings = LauncherSettings.parse(storeSettingsFile.text());
            root.settingsKnown = true;
        }
        onLoadFailed: {
            root.storeSettings = LauncherSettings.parse("");
            root.settingsKnown = true;
        }
    }

    Process {
        id: storeRead
        command: ["ryostore", "catalog", "--category", "launchers"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                root.store = root.storeCards(this.text);
                root.storeKnown = true;
            }
        }
    }

    Timer {
        id: slowTimer
        interval: 10000
        onTriggered: root.slow = root.installProc !== null
    }
}
