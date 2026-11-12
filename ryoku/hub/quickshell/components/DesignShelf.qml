import QtQuick
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../Singletons"

// the designs one surface can wear from a RyoStore category: Ryoku's own first,
// then what RyoStore offers; use, install and update, with Undo and a stoppable slow install
Item {
    id: root

    // RyoStore category ("notifications", "osds", "sidebars", ...) and the shell.json key that wears it
    required property string category
    required property string settingKey
    required property string changeLabel
    property string pageId: ""
    // the Undo notice after a switch; %1 is the design's name
    property string useNotice: I18n.tr("Now using %1")

    property var catalogue: []
    property string busyId: ""
    property string busyVerb: ""
    property var failed: null
    // the install or update Cancel stopped: { id, verb }
    property var stopped: null
    property bool cancelling: false
    // a slow install shows its progress and a way to stop it
    property bool slow: false

    readonly property var bind: ({ store: "settings", key: root.settingKey, fallback: "ryoku" })
    readonly property string active: String(Changes.value(root.bind) || "ryoku")
    readonly property var shipped: ({
        id: "ryoku", name: "Ryoku", summary: I18n.tr("The look that comes with Ryoku"), art: "",
        installed: true, source: "shipped", version: "", installedVersion: "", updateAvailable: false,
        unavailable: false, unavailableEdition: false, unavailableReason: "", requiredWindowManager: "",
        downloadPaused: false, downloadPauseReason: ""
    })
    readonly property var cards: root.cardModel()
    readonly property var activeCard: root.card(root.active) || root.shipped

    function tierOf(v) {
        v = String(v || "").toLowerCase();
        return v === "light" || v === "standard" || v === "heavy" ? v : "";
    }

    function catalogueCards(text) {
        try {
            var catalog = JSON.parse(text || "{}");
            // a design nobody can wear here and that is not installed is not offered
            return (catalog.items || []).filter(item => item.category === root.category
                    && !(item.unavailable === true && item.installed !== true)).map(item => ({
                id: item.id,
                name: item.name || item.id,
                summary: item.summary || item.description || "",
                art: item.art || "",
                resourceTier: root.tierOf(item.metadata && item.metadata.resourceTier),
                installed: item.installed === true,
                source: item.metadata && item.metadata.shipped === true ? "shipped" : "store",
                version: item.version || "",
                installedVersion: item.installedVersion || "",
                updateAvailable: item.updateAvailable === true,
                unavailable: item.unavailable === true,
                unavailableEdition: item.unavailableEdition === true,
                unavailableReason: item.unavailableReason || "",
                requiredWindowManager: item.requiredWindowManager || "",
                downloadPaused: item.downloadPaused === true,
                downloadPauseReason: item.downloadPauseReason || ""
            }));
        } catch (e) {
            return [];
        }
    }

    // fixed order, never by use: Ryoku's own, installed, offered, blocked; each A to Z
    function cardModel() {
        var installed = [], offered = [], blocked = [];
        for (var i = 0; i < root.catalogue.length; i++) {
            var c = root.catalogue[i];
            if (c.id === "ryoku")
                continue;
            if (c.installed && !c.unavailable)
                installed.push(c);
            else if (c.unavailable || c.downloadPaused)
                blocked.push(c);
            else
                offered.push(c);
        }
        var byName = (a, b) => a.name.localeCompare(b.name);
        return [root.shipped].concat(installed.sort(byName)).concat(offered.sort(byName)).concat(blocked.sort(byName));
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

    function blockedReason(c) {
        if (!c || (c.installed && !c.unavailable))
            return "";
        if (c.unavailable) {
            if (c.unavailableEdition)
                return I18n.tr("Not available on this system");
            var wm = String(c.requiredWindowManager || "");
            return wm.length > 0 ? I18n.tr("Needs %1").arg(wm.charAt(0).toUpperCase() + wm.slice(1)) : I18n.tr("Cannot run here");
        }
        if (c.downloadPaused)
            return I18n.tr("Not offered right now");
        return "";
    }

    function statusOf(c) {
        if (root.busyId === c.id)
            return root.busyVerb === "update" ? I18n.tr("Updating") : I18n.tr("Installing");
        if (c.id === root.active && root.wearable(c))
            return I18n.tr("In use");
        if (root.updatable(c))
            return I18n.tr("Update available");
        if (c.source === "shipped")
            return I18n.tr("Comes with Ryoku");
        return c.installed ? I18n.tr("Installed") : I18n.tr("Available in RyoStore");
    }

    function actionOf(c) {
        if (root.busyId !== "" || root.blockedReason(c) !== "")
            return "";
        if (c.id === root.active && root.wearable(c))
            return root.updatable(c) ? "update" : "";
        if (root.updatable(c))
            return "update";
        return c.installed ? "use" : "install";
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
            id: c.id,
            title: c.name,
            status: root.statusOf(c),
            resource: c.resourceTier || "",
            source: c.art,
            current: c.id === root.active && root.wearable(c),
            unavailable: root.blockedReason(c),
            busy: root.busyId === c.id,
            action: action,
            actionText: root.actionText(action),
            actionKind: action === "use" ? "tonal" : "line"
        };
    })

    function describe(c) {
        if (!c)
            return "";
        var parts = [];
        if (c.summary)
            parts.push(c.summary);
        var blocked = root.blockedReason(c);
        if (c.unavailable && c.unavailableReason)
            parts.push(I18n.tr(c.unavailableReason));
        else if (c.downloadPaused && c.downloadPauseReason)
            parts.push(I18n.tr(c.downloadPauseReason));
        else if (blocked !== "")
            parts.push(blocked);
        if (root.updatable(c))
            parts.push(I18n.tr("Version %1 is installed and %2 is ready").arg(c.installedVersion).arg(c.version));
        else if (c.installed && c.source !== "shipped" && c.installedVersion)
            parts.push(I18n.tr("Version %1 from RyoStore").arg(c.installedVersion));
        else if (!c.installed && c.version)
            parts.push(I18n.tr("Version %1 in RyoStore").arg(c.version));
        // two sentences, so a summary that starts with a capital ("The look…") reads right
        return I18n.tr("%1. %2.").arg(c.name).arg(parts.filter(p => p.length > 0).map(p => p.replace(/[.\s]+$/, "")).join(". "));
    }

    function act(id, action) {
        if (action === "use")
            root.use(id);
        else if (action === "install" || action === "update")
            root.install(id, action);
    }

    function use(id) {
        var c = root.card(id);
        if (!root.wearable(c) || id === root.active)
            return;
        Changes.set(root.bind, id, root.changeLabel, root.pageId, root.useNotice.arg(c.name));
    }

    // the install runs through Changes, so a missing ryostore is reported (code -1) and the
    // install carries on when the page closes
    property var installProc: null
    function install(id, verb) {
        var c = root.card(id);
        if (!c || root.busyId !== "")
            return;
        root.failed = null;
        root.stopped = null;
        root.cancelling = false;
        root.busyId = id;
        root.busyVerb = verb;
        slowTimer.restart();
        var shelf = root;
        root.installProc = Changes.run(["ryostore", "install", root.category, id], function (ok, out, err, code) {
            // the page may have closed while it ran
            if (!shelf)
                return;
            shelf.installProc = null;
            if (shelf.cancelling)
                shelf.stopped = { id: shelf.busyId, verb: shelf.busyVerb };
            else if (!ok)
                shelf.failed = { id: shelf.busyId, verb: shelf.busyVerb, message: err.trim(), missing: code === -1 };
            slowTimer.stop();
            shelf.slow = false;
            shelf.cancelling = false;
            shelf.busyId = "";
            shelf.busyVerb = "";
            shelf.refresh();
        });
    }

    function cancel() {
        if (!root.installProc || !root.installProc.running)
            return;
        root.cancelling = true;
        root.installProc.signal(15);
    }

    function refresh() {
        catalogueRead.running = true;
    }

    Process {
        id: catalogueRead
        command: ["ryostore", "catalog", "--category", root.category]
        running: true
        stdout: StdioCollector {
            onStreamFinished: root.catalogue = root.catalogueCards(this.text)
        }
    }

    Timer {
        id: slowTimer
        interval: 10000
        onTriggered: root.slow = !!root.installProc && root.installProc.running
    }
}
