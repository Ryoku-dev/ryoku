pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons

// Installed add-on management. RyoStore owns discovery and installation;
// Settings keeps plugin enablement, placement, live options, updates/removal,
// and bundle component removal. The page is full-bleed because every action
// applies immediately instead of participating in the shared Save/Revert flow.
Item {
    id: pg

    property var hub
    readonly property bool fullBleed: true

    // ── installed-management state ──────────────────────────────────────────
    property var plugins: []
    property var catalog: []
    property var bundleCatalog: []
    property string selId: ""
    property string busyId: ""
    property string bundleBusyId: ""
    property bool confirmRemove: false
    property bool loaded: false
    property var bundles: []
    property bool bundlesLoaded: false
    property var omarchyPlugins: []
    property bool omarchyLoaded: false
    property string omarchyBusyId: ""
    property string omarchyMutation: ""
    property string confirmKind: ""
    property string confirmId: ""
    property string confirmName: ""

    property string errorMessage: ""
    property string errorRetry: ""
    property var errorCommand: []
    property string errorTarget: ""
    property string errorItem: ""

    property var placeQueue: []
    property var settingsQueue: []
    property var activePlaceCommand: []
    property var activeSettingsCommand: []
    property string bundleMutationId: ""
    property string bundleMutationItem: ""
    property string bundleMutationAction: ""
    property string statusTarget: ""
    property bool statusQueued: false
    property string queuedStatusTarget: ""
    property var bundleStatusLoaded: ({})
    property string bundleSheetId: ""
    property var bundleSheetData: ({})
    property bool bundleSheetOpen: false
    property bool bundleSheetLoading: false

    readonly property string query: (pg.hub && pg.hub.query) ? ("" + pg.hub.query) : ""

    readonly property var sel: {
        for (var i = 0; i < pg.plugins.length; i++)
            if (pg.plugins[i].id === pg.selId)
                return pg.plugins[i];
        return null;
    }

    readonly property var shown: {
        var q = pg.query.trim().toLowerCase();
        if (q === "")
            return pg.plugins;
        return pg.plugins.filter(function (p) {
            var m = p.manifest || {};
            var name = ("" + (m.name || p.id || "")).toLowerCase();
            return name.indexOf(q) >= 0 || ("" + p.id).toLowerCase().indexOf(q) >= 0;
        });
    }
    readonly property var shownBundles: {
        const q = pg.query.trim().toLowerCase();
        if (q === "")
            return pg.bundles;
        return pg.bundles.filter(bundle => {
            const parts = (bundle.metadata || {}).items || [];
            const hay = [bundle.id, bundle.name, bundle.summary, bundle.description]
                .concat(parts.map(part => part.name + " " + (part.summary || "")))
                .join(" ").toLowerCase();
            return hay.indexOf(q) !== -1;
        });
    }
    readonly property var shownOmarchy: {
        const q = pg.query.trim().toLowerCase();
        if (q === "")
            return pg.omarchyPlugins;
        return pg.omarchyPlugins.filter(plugin => {
            const hay = [plugin.id, plugin.name, plugin.description, (plugin.kinds || []).join(" ")]
                .join(" ").toLowerCase();
            return hay.indexOf(q) !== -1;
        });
    }
    readonly property string barStyle: shellAdapter.barStyle
    readonly property bool allLoaded: pg.loaded && pg.bundlesLoaded
        && (pg.barStyle !== "nomarchy" || pg.omarchyLoaded)
    readonly property bool allSectionsEmpty: pg.plugins.length === 0
        && pg.bundles.length === 0
        && (pg.barStyle !== "nomarchy" || pg.omarchyPlugins.length === 0)

    readonly property string shellDir: Quickshell.env("RYOKU_SHELL_DIR")
    readonly property string script: (pg.shellDir && pg.shellDir.length > 0)
        ? pg.shellDir + "/quickshell/plugins/discover.sh"
        : (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/quickshell/plugins/discover.sh"
    readonly property string placeTool: (pg.shellDir && pg.shellDir.length > 0)
        ? pg.shellDir + "/quickshell/plugins/ryoku-plugins-place"
        : "ryoku-plugins-place"

    function hostLabel(key) {
        return key === "framePopout" ? I18n.tr("Frame popout")
            : key === "desktopWidget" ? I18n.tr("Desktop widget")
            : key === "topbarGlyph" ? I18n.tr("Bar")
            : key === "sidebarCard" ? I18n.tr("Sidebar card") : key;
    }
    function hostKey(label) {
        return label === I18n.tr("Frame popout") ? "framePopout"
            : label === I18n.tr("Desktop widget") ? "desktopWidget"
            : label === I18n.tr("Bar") ? "topbarGlyph"
            : label === I18n.tr("Sidebar card") ? "sidebarCard" : label;
    }

    function manifestHosts(manifest) {
        const raw = manifest.hosts !== undefined ? manifest.hosts : manifest.host;
        if (Array.isArray(raw))
            return raw;
        return raw ? [raw] : [];
    }
    function isDesktopWidget(plugin) {
        return pg.manifestHosts((plugin || {}).manifest || {}).indexOf("desktopWidget") !== -1;
    }
    function bundleStatusIsLoaded(id) {
        return id && pg.bundleStatusLoaded[id] === true;
    }
    function setBundleStatusLoaded(id, loaded) {
        if (!id)
            return;
        var next = {};
        for (var key in pg.bundleStatusLoaded)
            next[key] = pg.bundleStatusLoaded[key];
        next[id] = loaded;
        pg.bundleStatusLoaded = next;
    }
    function openBundle(bundle) {
        if (!bundle || !bundle.id)
            return;
        pg.bundleSheetId = bundle.id;
        pg.bundleSheetData = bundle;
        pg.bundleSheetOpen = true;
        pg.bundleSheetLoading = !pg.bundleStatusIsLoaded(bundle.id);
        if (pg.bundleSheetLoading)
            pg.loadBundleStatus(bundle.id);
    }
    function finishBundleStatus(target) {
        if (!pg.bundleSheetOpen)
            return;
        if (pg.bundleStatusIsLoaded(pg.bundleSheetId)
                || target === pg.bundleSheetId
                || (target === "" && !pg.statusQueued))
            pg.bundleSheetLoading = false;
    }

    function clearError(kind) {
        if (pg.errorRetry !== kind)
            return;
        pg.errorMessage = "";
        pg.errorRetry = "";
        pg.errorCommand = [];
        pg.errorTarget = "";
        pg.errorItem = "";
    }
    function compactError(stderrText, fallback, code) {
        const text = String(stderrText || "").trim();
        if (text.length === 0)
            return fallback.arg(code);
        const lines = text.split("\n").filter(line => line.trim().length > 0);
        return lines.slice(Math.max(0, lines.length - 2)).join(" ");
    }
    function showError(kind, message, command, target, item) {
        pg.errorRetry = kind;
        pg.errorMessage = message;
        pg.errorCommand = command ? command.slice() : [];
        pg.errorTarget = target || "";
        pg.errorItem = item || "";
    }
    function retryFailure() {
        switch (pg.errorRetry) {
        case "plugins-read": pg.refresh(); break;
        case "catalog-read": pg.loadCatalog(); break;
        case "bundles-read": pg.loadBundles(); break;
        case "bundle-status": pg.loadBundleStatus(pg.errorTarget); break;
        case "bundle-remove": pg.removeBundle(pg.errorTarget, pg.errorItem); break;
        case "bundle-repair": pg.repairBundle(pg.errorTarget); break;
        case "plugin-install": pg.install(pg.errorTarget); break;
        case "plugin-remove":
            pg.busyId = pg.errorTarget;
            rmProc.command = pg.errorCommand.slice();
            rmProc.running = true;
            break;
        case "plugin-forget":
            pg.busyId = pg.errorTarget;
            forgetProc.command = pg.errorCommand.slice();
            forgetProc.running = true;
            break;
        case "placement": pg.enqueuePlace(pg.errorCommand); break;
        case "setting": pg.enqueueSetting(pg.errorCommand); break;
        case "omarchy-read": pg.loadOmarchy(); break;
        case "omarchy-mutation":
            pg.omarchyBusyId = pg.errorTarget;
            omarchyMutProc.command = pg.errorCommand.slice();
            omarchyMutProc.running = true;
            break;
        }
    }

    function refresh() {
        if (!listProc.running)
            listProc.running = true;
    }
    function loadCatalog() {
        if (!catProc.running)
            catProc.running = true;
    }
    function loadBundles() {
        if (!bundleProc.running)
            bundleProc.running = true;
    }
    function loadOmarchy() {
        if (pg.barStyle === "nomarchy" && !omarchyListProc.running)
            omarchyListProc.running = true;
    }
    function browseStore() {
        Spawn.run(["ryostore", "open"]);
    }
    function browseOmarchyMarket() {
        Spawn.run(["ryostore", "open", "omarchy-plugins"]);
    }
    function requestPluginRemove(id, name) {
        pg.confirmKind = "plugin";
        pg.confirmId = id;
        pg.confirmName = name;
        pg.confirmRemove = true;
    }
    function mutateOmarchy(id, action) {
        if (!id || pg.omarchyBusyId !== "")
            return;
        pg.omarchyBusyId = id;
        pg.omarchyMutation = action;
        omarchyMutProc.command = ["omarchy", "plugin", action, id];
        if (action === "remove")
            omarchyMutProc.command.push("--yes");
        omarchyMutProc.running = true;
    }
    function requestOmarchyRemove(id, name) {
        pg.confirmKind = "omarchy";
        pg.confirmId = id;
        pg.confirmName = name;
        pg.confirmRemove = true;
    }
    function confirmRemoval() {
        pg.confirmRemove = false;
        if (pg.confirmKind === "omarchy")
            pg.mutateOmarchy(pg.confirmId, "remove");
        else
            pg.removePlugin(pg.confirmId);
    }
    function removeBundle(id, item) {
        if (!id || bundleMutProc.running)
            return;
        pg.bundleBusyId = id;
        pg.bundleMutationId = id;
        pg.bundleMutationItem = item || "";
        pg.bundleMutationAction = "remove";
        const scope = item ? ["item", id, item] : ["bundle", id];
        bundleMutProc.command = ["kitty", "--class", "ryostore", "-e",
            "ryostore-install", "remove"].concat(scope);
        bundleMutProc.running = true;
    }
    function repairBundle(id) {
        if (!id || bundleMutProc.running)
            return;
        pg.bundleBusyId = id;
        pg.bundleMutationId = id;
        pg.bundleMutationItem = "";
        pg.bundleMutationAction = "repair";
        bundleMutProc.command = ["kitty", "--class", "ryostore", "-e",
            "ryostore-install", "install", "bundle", id];
        bundleMutProc.running = true;
    }
    function catalogEntry(id) {
        for (var i = 0; i < pg.catalog.length; i++)
            if (pg.catalog[i].id === id)
                return pg.catalog[i];
        return null;
    }
    function cmpSemver(a, b) {
        var pa = String(a || "0").split(".").map(function (n) { return parseInt(n, 10) || 0; });
        var pb = String(b || "0").split(".").map(function (n) { return parseInt(n, 10) || 0; });
        for (var i = 0; i < Math.max(pa.length, pb.length); i++) {
            var x = pa[i] || 0, y = pb[i] || 0;
            if (x !== y)
                return x < y ? -1 : 1;
        }
        return 0;
    }
    function updateFor(pl) {
        var inst = (pl && pl.manifest && pl.manifest.version) ? pl.manifest.version : "";
        var ce = pg.catalogEntry(pl ? pl.id : "");
        var avail = ce ? (ce.version || "") : "";
        if (!inst || !avail)
            return "";
        return pg.cmpSemver(avail, inst) > 0 ? avail : "";
    }
    function install(id) {
        if (!id || installProc.running || pg.busyId !== "")
            return;
        pg.busyId = id;
        installProc.command = ["ryostore", "internal", "install-guest", "plugins", id];
        installProc.running = true;
    }
    function enqueuePlace(args) {
        if (!args || args.length === 0)
            return;
        var queue = pg.placeQueue.slice();
        queue.push(args.slice());
        pg.placeQueue = queue;
        pg.startNextPlace();
    }
    function startNextPlace() {
        if (placeProc.running || pg.placeQueue.length === 0)
            return;
        var queue = pg.placeQueue.slice();
        pg.activePlaceCommand = queue.shift();
        pg.placeQueue = queue;
        placeProc.command = pg.activePlaceCommand;
        placeProc.running = true;
    }
    function place(id, field, a, b, c, d) {
        if (!id)
            return;
        var args = [pg.placeTool, id, field];
        for (var value of [a, b, c, d])
            if (value !== undefined)
                args.push("" + value);
        pg.enqueuePlace(args);
    }
    function enqueueSetting(args) {
        if (!args || args.length === 0)
            return;
        var queue = pg.settingsQueue.slice();
        queue.push(args.slice());
        pg.settingsQueue = queue;
        pg.startNextSetting();
    }
    function startNextSetting() {
        if (settingsProc.running || pg.settingsQueue.length === 0)
            return;
        var queue = pg.settingsQueue.slice();
        pg.activeSettingsCommand = queue.shift();
        pg.settingsQueue = queue;
        settingsProc.command = pg.activeSettingsCommand;
        settingsProc.running = true;
    }
    function setSetting(id, key, value) {
        if (!id)
            return;
        var obj = {};
        obj[key] = value;
        pg.enqueueSetting([pg.placeTool, id, "settings", JSON.stringify(obj)]);
    }
    function removePlugin(id) {
        if (!id || pg.busyId !== "")
            return;
        pg.busyId = id;
        pg.confirmRemove = false;
        rmProc.command = ["ryostore", "internal", "remove-guest", "plugins", id];
        rmProc.running = true;
    }

    function bundleWithStatus(base, status) {
        var result = JSON.parse(JSON.stringify(base || {}));
        var metadata = result.metadata || {};
        var parts = metadata.items || [];
        var states = {};
        for (var i = 0; i < status.items.length; ++i)
            states[String(status.items[i].name)] = status.items[i].status;
        var count = 0;
        for (var j = 0; j < parts.length; ++j) {
            var part = JSON.parse(JSON.stringify(parts[j]));
            if (states[part.name] !== undefined)
                part.installed = states[part.name] === "present";
            if (part.installed === true)
                count++;
            parts[j] = part;
        }
        metadata.items = parts;
        result.metadata = metadata;
        result.installedCount = count;
        result.totalCount = parts.length;
        return result;
    }
    function applyBundleCatalogue(items) {
        var current = {};
        for (var i = 0; i < pg.bundles.length; ++i)
            current[pg.bundles[i].id] = pg.bundles[i];
        pg.bundleCatalog = items.filter(item => item.category === "bundles");
        var next = [];
        for (var j = 0; j < pg.bundleCatalog.length; ++j) {
            var base = pg.bundleCatalog[j];
            if (current[base.id] !== undefined)
                next.push(current[base.id]);
            else if (Number(base.installedCount || 0) > 0)
                next.push(base);
        }
        pg.bundles = next;
    }
    function applyBundleStatuses(rows) {
        var byId = {};
        var loaded = {};
        for (var key in pg.bundleStatusLoaded)
            loaded[key] = pg.bundleStatusLoaded[key];
        for (var i = 0; i < rows.length; ++i) {
            byId[rows[i].id] = rows[i];
            loaded[rows[i].id] = true;
        }
        var current = {};
        for (var j = 0; j < pg.bundles.length; ++j)
            current[pg.bundles[j].id] = pg.bundles[j];
        var next = [];
        for (var k = 0; k < pg.bundleCatalog.length; ++k) {
            var base = pg.bundleCatalog[k];
            var item = byId[base.id] !== undefined
                ? pg.bundleWithStatus(base, byId[base.id])
                : (current[base.id] || base);
            if (base.id === pg.bundleSheetId)
                pg.bundleSheetData = item;
            if (Number(item.installedCount || 0) > 0)
                next.push(item);
        }
        pg.bundleStatusLoaded = loaded;
        pg.bundles = next;
        if (pg.bundleStatusIsLoaded(pg.bundleSheetId))
            pg.bundleSheetLoading = false;
    }
    function loadBundleStatus(id) {
        const target = id || "";
        if (target.length > 0) {
            pg.setBundleStatusLoaded(target, false);
            if (target === pg.bundleSheetId)
                pg.bundleSheetLoading = true;
        }
        if (statusProc.running) {
            pg.statusQueued = true;
            pg.queuedStatusTarget = target;
            return;
        }
        pg.statusTarget = target;
        statusProc.command = target.length > 0
            ? ["ryostore-install", "status", "bundle", target]
            : ["ryostore-install", "status"];
        statusProc.running = true;
    }
    function runQueuedStatus() {
        if (!pg.statusQueued)
            return;
        const target = pg.queuedStatusTarget;
        pg.statusQueued = false;
        pg.queuedStatusTarget = "";
        Qt.callLater(() => pg.loadBundleStatus(target));
    }

    FileView {
        id: shellFile
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config"))
            + "/ryoku/shell.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()

        JsonAdapter {
            id: shellAdapter
            property string barStyle: "qsbar"
        }
    }

    onBarStyleChanged: {
        if (pg.barStyle === "nomarchy")
            pg.loadOmarchy();
        else {
            pg.omarchyPlugins = [];
            pg.omarchyLoaded = false;
        }
    }

    Component.onCompleted: {
        pg.refresh();
        pg.loadCatalog();
        pg.loadBundles();
        pg.loadOmarchy();
    }

    Process {
        id: listProc
        command: ["bash", pg.script, "--all"]
        environment: Spawn.env
        stdout: StdioCollector { id: listOut }
        stderr: StdioCollector { id: listErr }
        onExited: code => {
            if (code !== 0) {
                pg.showError("plugins-read",
                    pg.compactError(listErr.text, I18n.tr("Couldn't read installed add-ons (exit %1)."), code));
                return;
            }
            try {
                const parsed = JSON.parse(listOut.text || "[]");
                if (!Array.isArray(parsed))
                    throw new Error("not an array");
                const addOns = parsed.filter(plugin => !pg.isDesktopWidget(plugin));
                pg.plugins = addOns;
                if (pg.selId !== "") {
                    var selectedStillExists = false;
                    for (var i = 0; i < addOns.length; ++i)
                        if (addOns[i].id === pg.selId)
                            selectedStillExists = true;
                    if (!selectedStillExists)
                        pg.selId = "";
                }
                pg.loaded = true;
                pg.clearError("plugins-read");
            } catch (error) {
                pg.showError("plugins-read", I18n.tr("Couldn't read installed add-ons."));
            }
        }
    }
    Process {
        id: placeProc
        environment: Spawn.env
        stderr: StdioCollector { id: placeErr }
        onExited: code => {
            if (code !== 0) {
                pg.placeQueue = [];
                pg.showError("placement",
                    pg.compactError(placeErr.text, I18n.tr("Couldn't save placement (exit %1)."), code),
                    pg.activePlaceCommand);
                pg.refresh();
                return;
            }
            pg.clearError("placement");
            if (pg.placeQueue.length > 0)
                pg.startNextPlace();
            else
                pg.refresh();
        }
    }
    Process {
        id: settingsProc
        environment: Spawn.env
        stderr: StdioCollector { id: settingsErr }
        onExited: code => {
            if (code !== 0) {
                pg.settingsQueue = [];
                pg.showError("setting",
                    pg.compactError(settingsErr.text, I18n.tr("Couldn't save add-on setting (exit %1)."), code),
                    pg.activeSettingsCommand);
                pg.refresh();
                return;
            }
            pg.clearError("setting");
            if (pg.settingsQueue.length > 0)
                pg.startNextSetting();
            else
                pg.refresh();
        }
    }
    Process {
        id: rmProc
        environment: Spawn.env
        stderr: StdioCollector { id: rmErr }
        onExited: code => {
            const id = pg.busyId;
            if (code !== 0) {
                pg.busyId = "";
                pg.showError("plugin-remove",
                    pg.compactError(rmErr.text, I18n.tr("Couldn't remove add-on (exit %1)."), code),
                    rmProc.command, id);
                pg.refresh();
                return;
            }
            pg.clearError("plugin-remove");
            forgetProc.command = [pg.placeTool, id, "forget"];
            forgetProc.running = true;
        }
    }
    Process {
        id: forgetProc
        environment: Spawn.env
        stderr: StdioCollector { id: forgetErr }
        onExited: code => {
            const id = pg.busyId;
            if (code !== 0) {
                pg.busyId = "";
                pg.showError("plugin-forget",
                    pg.compactError(forgetErr.text, I18n.tr("Add-on removed, but its settings couldn't be cleared (exit %1)."), code),
                    forgetProc.command, id);
                return;
            }
            pg.clearError("plugin-forget");
            pg.busyId = "";
            pg.selId = "";
            pg.refresh();
            pg.loadCatalog();
        }
    }
    Process {
        id: catProc
        command: ["ryostore", "catalog", "--category", "plugins"]
        environment: Spawn.env
        stdout: StdioCollector { id: catOut }
        stderr: StdioCollector { id: catErr }
        onExited: code => {
            if (code !== 0) {
                pg.showError("catalog-read",
                    pg.compactError(catErr.text, I18n.tr("Couldn't read the add-on catalogue (exit %1)."), code));
                return;
            }
            try {
                const items = (JSON.parse(catOut.text || "{}").items) || [];
                if (!Array.isArray(items))
                    throw new Error("items is not an array");
                pg.catalog = items;
                pg.clearError("catalog-read");
            } catch (error) {
                pg.showError("catalog-read", I18n.tr("Couldn't read the add-on catalogue."));
            }
        }
    }
    Process {
        id: installProc
        environment: Spawn.env
        stderr: StdioCollector { id: installErr }
        onExited: code => {
            const id = pg.busyId;
            pg.busyId = "";
            if (code !== 0) {
                pg.showError("plugin-install",
                    pg.compactError(installErr.text, I18n.tr("Couldn't update add-on (exit %1)."), code),
                    installProc.command, id);
            } else {
                pg.clearError("plugin-install");
            }
            pg.refresh();
            pg.loadCatalog();
        }
    }
    Process {
        id: bundleProc
        command: ["ryostore", "catalog", "--category", "bundles"]
        environment: Spawn.env
        stdout: StdioCollector { id: bundleOut }
        stderr: StdioCollector { id: bundleErr }
        onExited: code => {
            if (code !== 0) {
                pg.showError("bundles-read",
                    pg.compactError(bundleErr.text, I18n.tr("Couldn't read installed bundles (exit %1)."), code));
                return;
            }
            try {
                const items = (JSON.parse(bundleOut.text || "{}").items) || [];
                if (!Array.isArray(items))
                    throw new Error("items is not an array");
                pg.applyBundleCatalogue(items);
                pg.bundlesLoaded = true;
                pg.clearError("bundles-read");
                pg.loadBundleStatus("");
            } catch (error) {
                pg.showError("bundles-read", I18n.tr("Couldn't read installed bundles."));
            }
        }
    }
    Process {
        id: statusProc
        environment: Spawn.env
        stdout: StdioCollector { id: statusOut }
        stderr: StdioCollector { id: statusErr }
        onExited: code => {
            const target = pg.statusTarget;
            if (code !== 0) {
                pg.finishBundleStatus(target);
                pg.showError("bundle-status",
                    pg.compactError(statusErr.text, I18n.tr("Couldn't check bundle state (exit %1)."), code),
                    statusProc.command, target);
                pg.runQueuedStatus();
                return;
            }
            try {
                const parsed = JSON.parse(statusOut.text || "{}");
                const rows = target.length > 0 ? [parsed] : (parsed.bundles || []);
                if (!Array.isArray(rows))
                    throw new Error("bundles is not an array");
                for (var i = 0; i < rows.length; ++i)
                    if (!rows[i] || !Array.isArray(rows[i].items))
                        throw new Error("bundle items is not an array");
                pg.applyBundleStatuses(rows);
                pg.clearError("bundle-status");
            } catch (error) {
                pg.finishBundleStatus(target);
                pg.showError("bundle-status", I18n.tr("Couldn't read bundle state."),
                    statusProc.command, target);
            }
            pg.finishBundleStatus(target);
            pg.runQueuedStatus();
        }
    }
    Process {
        id: bundleMutProc
        environment: Spawn.env
        stderr: StdioCollector { id: bundleMutErr }
        onExited: code => {
            const id = pg.bundleMutationId;
            const item = pg.bundleMutationItem;
            const action = pg.bundleMutationAction;
            const retry = action === "repair" ? "bundle-repair" : "bundle-remove";
            pg.bundleBusyId = "";
            if (code !== 0) {
                pg.showError(retry,
                    pg.compactError(bundleMutErr.text,
                        action === "repair"
                            ? I18n.tr("Couldn't repair bundle (exit %1).")
                            : I18n.tr("Couldn't remove bundle content (exit %1)."),
                        code),
                    bundleMutProc.command, id, item);
            } else {
                pg.clearError(retry);
            }
            pg.loadBundleStatus(id);
        }
    }
    Process {
        id: omarchyListProc
        command: ["omarchy", "plugin", "list", "--json"]
        environment: Spawn.env
        stdout: StdioCollector { id: omarchyListOut }
        stderr: StdioCollector { id: omarchyListErr }
        onExited: code => {
            if (code !== 0) {
                pg.showError("omarchy-read",
                    pg.compactError(omarchyListErr.text,
                        I18n.tr("Couldn't read Omarchy add-ons (exit %1)."), code));
                return;
            }
            try {
                const rows = JSON.parse(omarchyListOut.text || "[]");
                if (!Array.isArray(rows))
                    throw new Error("not an array");
                pg.omarchyPlugins = rows.filter(plugin => plugin.firstParty !== true)
                    .map(plugin => {
                        const item = Object.assign({}, plugin);
                        item.kinds = plugin.kinds || [];
                        item.enabled = plugin.enabled === true;
                        item.canDisable = plugin.canDisable !== false;
                        item.blocked = plugin.blocked === true;
                        item.blockedReason = plugin.blockedReason || "";
                        return item;
                    });
                pg.omarchyLoaded = true;
                pg.clearError("omarchy-read");
            } catch (error) {
                pg.showError("omarchy-read", I18n.tr("Couldn't read Omarchy add-ons."));
            }
        }
    }
    Process {
        id: omarchyMutProc
        environment: Spawn.env
        stderr: StdioCollector { id: omarchyMutErr }
        onExited: code => {
            const id = pg.omarchyBusyId;
            const action = pg.omarchyMutation;
            pg.omarchyBusyId = "";
            if (code !== 0) {
                pg.showError("omarchy-mutation",
                    pg.compactError(omarchyMutErr.text,
                        action === "remove"
                            ? I18n.tr("Couldn't remove Omarchy add-on (exit %1).")
                            : I18n.tr("Couldn't change Omarchy add-on (exit %1)."),
                        code),
                    omarchyMutProc.command, id);
            } else {
                pg.clearError("omarchy-mutation");
                pg.loadOmarchy();
            }
        }
    }
    // ── head: eyebrow, Fraunces title, blurb (matches every page) ───────────
    Column {
        id: head
        anchors.top: parent.top
        anchors.topMargin: Tokens.s6
        // the head sits on the body's grid: left-inset and body-wide, so the
        // title starts over the first card column instead of floating centred
        x: Tokens.s6
        width: Math.max(320, pg.width - Tokens.s6 * 2)
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
                text: I18n.tr("ADD-ONS"); color: Tokens.inkMuted; font.family: Tokens.ui
                font.pixelSize: 9; font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                anchors.verticalCenter: parent.verticalCenter
            }
        }
        Item {
            width: parent.width
            height: title.implicitHeight
            Text {
                id: title
                anchors.left: parent.left
                text: I18n.tr("Add-ons")
                color: Tokens.ink
                font.family: Tokens.display
                font.pixelSize: Tokens.fTitle
            }
            Btn {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("BROWSE RYOSTORE")
                onAct: pg.browseStore()
            }
        }
        Text {
            width: Math.min(parent.width, 720)
            text: I18n.tr("Shell add-ons and bundles you've installed.")
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: Tokens.fBody
            wrapMode: Text.WordWrap
        }
        Row {
            spacing: Tokens.s2
            Rectangle {
                width: addOnStat.implicitWidth + Tokens.s3 * 2
                height: 22
                radius: Tokens.radius
                color: "transparent"
                border.width: Tokens.border
                border.color: Tokens.line
                Text {
                    id: addOnStat
                    anchors.centerIn: parent
                    text: (pg.plugins.length === 1
                        ? I18n.tr("%1 add-on") : I18n.tr("%1 add-ons"))
                        .arg(pg.plugins.length)
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fTiny
                }
            }
            Rectangle {
                width: bundleStat.implicitWidth + Tokens.s3 * 2
                height: 22
                radius: Tokens.radius
                color: "transparent"
                border.width: Tokens.border
                border.color: Tokens.line
                Text {
                    id: bundleStat
                    anchors.centerIn: parent
                    text: (pg.bundles.length === 1
                        ? I18n.tr("%1 bundle") : I18n.tr("%1 bundles"))
                        .arg(pg.bundles.length)
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fTiny
                }
            }
        }
    }


    Rectangle {
        id: errorBanner
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: head.bottom
        anchors.leftMargin: Tokens.s6
        anchors.rightMargin: Tokens.s6
        anchors.topMargin: visible ? Tokens.s3 : 0
        visible: pg.errorMessage.length > 0
        height: visible ? 44 : 0
        radius: Tokens.radius
        color: "transparent"
        border.width: Tokens.border
        border.color: Tokens.lineStrong

        Text {
            anchors.left: parent.left
            anchors.right: retryBtn.left
            anchors.leftMargin: Tokens.s3
            anchors.rightMargin: Tokens.s3
            anchors.verticalCenter: parent.verticalCenter
            text: pg.errorMessage
            color: Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            elide: Text.ElideRight
        }
        Btn {
            id: retryBtn
            anchors.right: parent.right
            anchors.rightMargin: Tokens.s2
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("RETRY")
            compact: true
            onAct: pg.retryFailure()
        }
    }


    // ── the body: a Loader swaps master (list) and detail (one plugin) ──────
    Loader {
        id: body
        anchors {
            left: parent.left; right: parent.right
            top: errorBanner.visible ? errorBanner.bottom : head.bottom
            bottom: parent.bottom
            leftMargin: Tokens.s6; rightMargin: Tokens.s6
            topMargin: errorBanner.visible ? Tokens.s3 : Tokens.s5
            bottomMargin: Tokens.s6
        }
        sourceComponent: pg.selId === "" ? masterComp : detailComp
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
        target: body.item; property: "opacity"; to: 1
        duration: Tokens.swap; easing.type: Tokens.ease
    }

    // ── master: the installed list ──────────────────────────────────────────
    Component {
        id: masterComp

        Flickable {
            id: libraryFlick
            contentWidth: width
            contentHeight: libraryContent.implicitHeight
            clip: true
            boundsBehavior: Flickable.StopAtBounds
            ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
            WheelScroll { }

            Column {
                id: libraryContent
                width: libraryFlick.width
                spacing: Tokens.s5

                Column {
                    width: parent.width
                    spacing: Tokens.s3

                    AddonSectionHeader {
                        width: parent.width
                        title: I18n.tr("SHELL ADD-ONS")
                        countText: (pg.plugins.length === 1
                            ? I18n.tr("%1 ADD-ON") : I18n.tr("%1 ADD-ONS"))
                            .arg(pg.plugins.length)
                        onRefreshRequested: pg.refresh()
                    }

                    AddonGrid {
                        id: pluginGrid
                        width: parent.width
                        visible: pg.shown.length > 0

                        Repeater {
                            model: pg.shown
                            delegate: AddonPluginCard {
                                id: pluginCard
                                required property var modelData
                                readonly property var man: pluginCard.modelData.manifest || ({})
                                readonly property var place: pluginCard.modelData.placement || ({})
                                readonly property string hostKey: pluginCard.place.host
                                    || ((pluginCard.man.defaults && pluginCard.man.defaults.host)
                                        ? pluginCard.man.defaults.host : "framePopout")

                                width: pluginGrid.cardWidth
                                plugin: pluginCard.modelData
                                hostText: pg.hostLabel(pluginCard.hostKey)
                                updateVersion: pg.updateFor(pluginCard.modelData)
                                busy: pg.busyId === pluginCard.modelData.id
                                onSettingsRequested: pg.selId = pluginCard.modelData.id
                                onEnabledRequested: enabled =>
                                    pg.place(pluginCard.modelData.id, "enabled", enabled ? "true" : "false")
                                onRemoveRequested: pg.requestPluginRemove(
                                    pluginCard.modelData.id,
                                    pluginCard.man.name || pluginCard.modelData.id)
                            }
                        }
                    }

                    Rectangle {
                        width: parent.width
                        height: 42
                        visible: !pg.allSectionsEmpty && pg.loaded
                            && pg.plugins.length === 0
                            && pg.errorRetry !== "plugins-read"
                        radius: Tokens.radius
                        color: "transparent"
                        border.width: Tokens.border
                        border.color: Tokens.line
                        Text {
                            anchors.left: parent.left
                            anchors.leftMargin: Tokens.s3
                            anchors.verticalCenter: parent.verticalCenter
                            text: I18n.tr("No shell add-ons yet.")
                            color: Tokens.inkMuted
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall
                        }
                    }

                    Text {
                        width: parent.width
                        visible: pg.query.trim() !== "" && pg.plugins.length > 0
                            && pg.shown.length === 0
                        text: I18n.tr("No shell add-ons match your search.")
                        color: Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                    }
                }

                Column {
                    width: parent.width
                    spacing: Tokens.s3

                    AddonSectionHeader {
                        width: parent.width
                        title: I18n.tr("BUNDLES")
                        countText: (pg.bundles.length === 1
                            ? I18n.tr("%1 BUNDLE") : I18n.tr("%1 BUNDLES"))
                            .arg(pg.bundles.length)
                        onRefreshRequested: pg.loadBundles()
                    }

                    AddonGrid {
                        id: bundleGrid
                        width: parent.width
                        visible: pg.shownBundles.length > 0

                        Repeater {
                            model: pg.shownBundles
                            delegate: AddonBundleCard {
                                id: installedBundleCard
                                required property var modelData
                                width: bundleGrid.cardWidth
                                bundle: installedBundleCard.modelData
                                busy: pg.bundleBusyId === installedBundleCard.modelData.id
                                onOpenRequested: pg.openBundle(installedBundleCard.modelData)
                                onRepairRequested: pg.repairBundle(installedBundleCard.modelData.id)
                                onRemoveRequested: pg.removeBundle(installedBundleCard.modelData.id, "")
                            }
                        }
                    }

                    Rectangle {
                        width: parent.width
                        height: 42
                        visible: !pg.allSectionsEmpty && pg.bundlesLoaded
                            && pg.bundles.length === 0
                            && pg.errorRetry !== "bundles-read"
                            && pg.errorRetry !== "bundle-status"
                        radius: Tokens.radius
                        color: "transparent"
                        border.width: Tokens.border
                        border.color: Tokens.line
                        Text {
                            anchors.left: parent.left
                            anchors.leftMargin: Tokens.s3
                            anchors.verticalCenter: parent.verticalCenter
                            text: I18n.tr("No bundles yet.")
                            color: Tokens.inkMuted
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall
                        }
                    }

                    Text {
                        width: parent.width
                        visible: pg.query.trim() !== "" && pg.bundles.length > 0
                            && pg.shownBundles.length === 0
                        text: I18n.tr("No bundles match your search.")
                        color: Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                    }
                }

                Column {
                    width: parent.width
                    spacing: Tokens.s3
                    visible: pg.barStyle === "nomarchy"

                    AddonSectionHeader {
                        width: parent.width
                        title: I18n.tr("OMARCHY")
                        countText: (pg.omarchyPlugins.length === 1
                            ? I18n.tr("%1 ADD-ON") : I18n.tr("%1 ADD-ONS"))
                            .arg(pg.omarchyPlugins.length)
                        actionText: I18n.tr("OPEN MARKET")
                        onActionRequested: pg.browseOmarchyMarket()
                        onRefreshRequested: pg.loadOmarchy()
                    }

                    AddonGrid {
                        id: omarchyGrid
                        width: parent.width
                        visible: pg.shownOmarchy.length > 0

                        Repeater {
                            model: pg.shownOmarchy
                            delegate: AddonOmarchyCard {
                                id: omarchyCard
                                required property var modelData
                                width: omarchyGrid.cardWidth
                                plugin: omarchyCard.modelData
                                busy: pg.omarchyBusyId === omarchyCard.modelData.id
                                onEnabledRequested: enabled => pg.mutateOmarchy(
                                    omarchyCard.modelData.id, enabled ? "enable" : "disable")
                                onRemoveRequested: pg.requestOmarchyRemove(
                                    omarchyCard.modelData.id,
                                    omarchyCard.modelData.name || omarchyCard.modelData.id)
                            }
                        }
                    }

                    Rectangle {
                        width: parent.width
                        height: 42
                        visible: !pg.allSectionsEmpty && pg.omarchyLoaded
                            && pg.omarchyPlugins.length === 0
                            && pg.errorRetry !== "omarchy-read"
                        radius: Tokens.radius
                        color: "transparent"
                        border.width: Tokens.border
                        border.color: Tokens.line
                        Text {
                            anchors.left: parent.left
                            anchors.leftMargin: Tokens.s3
                            anchors.verticalCenter: parent.verticalCenter
                            text: I18n.tr("No Omarchy add-ons yet.")
                            color: Tokens.inkMuted
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall
                        }
                    }
                }

                Rectangle {
                    width: parent.width
                    height: 132
                    visible: pg.allLoaded && pg.allSectionsEmpty
                        && pg.errorMessage.length === 0
                    radius: Tokens.radius
                    color: "transparent"
                    border.width: Tokens.border
                    border.color: Tokens.line

                    Column {
                        anchors.left: parent.left
                        anchors.leftMargin: Tokens.s5
                        anchors.verticalCenter: parent.verticalCenter
                        width: Math.min(560,
                            parent.width - Tokens.s5 * 2 - emptyBrowse.width - Tokens.s5)
                        spacing: Tokens.s2
                        Text {
                            width: parent.width
                            text: I18n.tr("No add-ons installed")
                            color: Tokens.ink
                            font.family: Tokens.display
                            font.pixelSize: Tokens.fValue
                        }
                        Text {
                            width: parent.width
                            text: I18n.tr("Add-ons bring small tools, app bundles and new shell surfaces to Ryoku.")
                            color: Tokens.inkMuted
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall
                            wrapMode: Text.WordWrap
                        }
                    }
                    Btn {
                        id: emptyBrowse
                        anchors.right: parent.right
                        anchors.rightMargin: Tokens.s5
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("BROWSE RYOSTORE")
                        onAct: pg.browseStore()
                    }
                }

            }
        }
    }


    // ── detail: one plugin's placement + settings ───────────────────────────
    Component {
        id: detailComp

        Item {
            id: detail
            property var pendingImageField: null

            readonly property var sel: pg.sel || ({})
            readonly property var man: detail.sel.manifest || ({})
            readonly property var place: detail.sel.placement || ({})
            readonly property var schema: (detail.man.metadata && detail.man.metadata.settings) || []
            readonly property bool enabled: detail.place.enabled === true
            readonly property string host: (detail.place.host)
                ? detail.place.host
                : ((detail.man.defaults && detail.man.defaults.host) ? detail.man.defaults.host : "framePopout")
            readonly property var hosts: (detail.man.hosts || []).filter(function (h) {
                return h === "framePopout" || h === "desktopWidget"
                    || h === "topbarGlyph" || h === "sidebarCard";
            })
            readonly property string upd: pg.updateFor(detail.sel)

            // ── header: back + name + version .... update + remove ──
            Item {
                id: hdr
                anchors { left: parent.left; right: parent.right; top: parent.top }
                height: 40

                IconBtn {
                    id: backBtn
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    glyph: "\u2039"
                    onAct: pg.selId = ""
                }

                Row {
                    id: acts
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Tokens.s3

                    // Update: only when the catalogue carries a newer version.
                    Btn {
                        anchors.verticalCenter: parent.verticalCenter
                        visible: detail.upd !== ""
                        text: pg.busyId === detail.sel.id ? I18n.tr("UPDATING") : I18n.tr("UPDATE %1").arg(detail.upd)
                        armed: pg.busyId === ""
                        onAct: pg.install(detail.sel.id)
                    }
                    // Remove: destructive, so it arms the confirm plate first.
                    Btn {
                        anchors.verticalCenter: parent.verticalCenter
                        text: pg.busyId === detail.sel.id ? I18n.tr("REMOVING") : I18n.tr("REMOVE")
                        armed: pg.busyId === ""
                        onAct: pg.requestPluginRemove(
                            detail.sel.id, detail.man.name || detail.sel.id)
                    }
                }

                Text {
                    id: nameT
                    anchors.left: backBtn.right; anchors.leftMargin: Tokens.s3
                    anchors.right: verT.left; anchors.rightMargin: Tokens.s2
                    anchors.verticalCenter: parent.verticalCenter
                    text: (detail.man.name) ? detail.man.name : (detail.sel.id || "")
                    color: Tokens.ink; font.family: Tokens.ui
                    font.pixelSize: Tokens.fValue; font.weight: Font.Medium
                    elide: Text.ElideRight
                }
                Text {
                    id: verT
                    anchors.right: acts.left; anchors.rightMargin: Tokens.s4
                    anchors.verticalCenter: parent.verticalCenter
                    visible: text !== ""
                    text: (detail.man.version) ? ("v" + detail.man.version) : ""
                    color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny
                }
            }

            Flickable {
                id: dflick
                anchors {
                    left: parent.left; right: parent.right
                    top: hdr.bottom; bottom: parent.bottom
                    topMargin: Tokens.s5
                }
                contentWidth: width
                contentHeight: Math.max(dcol.height, height)
                clip: true
                boundsBehavior: Flickable.StopAtBounds
                ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
                WheelScroll { }

                Column {
                    id: dcol
                    width: dflick.width - Tokens.s3
                    spacing: Tokens.s5

                    // A plugin Ryoku did not write says so first: the same
                    // warning the Store and QS Bar Settings print.
                    Rectangle {
                        width: parent.width
                        visible: detail.man.official !== true
                        height: visible ? communityRow.implicitHeight + Tokens.s4 * 2 : 0
                        radius: Tokens.radius
                        color: "transparent"
                        border.width: Tokens.border
                        border.color: Tokens.lineStrong
                        Row {
                            id: communityRow
                            anchors {
                                left: parent.left; right: parent.right; verticalCenter: parent.verticalCenter
                                leftMargin: Tokens.s4; rightMargin: Tokens.s4
                            }
                            spacing: Tokens.s3
                            Text {
                                anchors.verticalCenter: parent.verticalCenter
                                text: "warning"
                                color: Tokens.inkDim
                                font.family: "Material Symbols Rounded"
                                font.pixelSize: Tokens.fRow
                            }
                            Text {
                                anchors.verticalCenter: parent.verticalCenter
                                width: parent.width - Tokens.fRow - parent.spacing
                                text: I18n.tr("Community plugin. Ryoku does not review or maintain it: it runs inside your shell with your permissions, so inspect its code before you trust it.")
                                color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                                wrapMode: Text.WordWrap
                            }
                        }
                    }

                    // ── Placement: enable, host, and where a popout sits ──
                    Column {
                        width: parent.width
                        spacing: Tokens.s3

                        Item {
                            width: parent.width; height: 20
                            Row {
                                id: pHead
                                anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                                spacing: Tokens.s2
                                Rectangle { width: 4; height: 4; color: Tokens.ink; anchors.verticalCenter: parent.verticalCenter }
                                Text {
                                    text: I18n.tr("PLACEMENT"); color: Tokens.ink; font.family: Tokens.ui
                                    font.pixelSize: Tokens.fMicro; font.weight: Font.Medium
                                    font.letterSpacing: Tokens.trackMark
                                    anchors.verticalCenter: parent.verticalCenter
                                }
                            }
                            Rectangle {
                                anchors.left: pHead.right; anchors.leftMargin: Tokens.s3
                                anchors.right: parent.right
                                anchors.verticalCenter: parent.verticalCenter
                                height: 1; color: Tokens.lineSoft
                            }
                        }

                        // Enabled: runs it on the desktop, or keeps it dormant.
                        Item {
                            width: parent.width; height: 30
                            Text {
                                anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                                text: I18n.tr("Enabled")
                                color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fBody
                            }
                            Sw {
                                anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                                on: detail.enabled
                                onToggled: (v) => pg.place(detail.sel.id, "enabled", v ? "true" : "false")
                            }
                        }

                        // Show as: only when the plugin offers more than one home.
                        Item {
                            width: parent.width; height: 30
                            visible: detail.hosts.length > 1
                            Text {
                                anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                                text: I18n.tr("Show as")
                                color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fBody
                            }
                            Seg {
                                anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                                options: detail.hosts.map(function (h) { return pg.hostLabel(h); })
                                current: pg.hostLabel(detail.host)
                                onChose: (label) => pg.place(detail.sel.id, "host", pg.hostKey(label))
                            }
                        }

                        // ── frame-popout placement editor ──
                        // Screen-proportioned stage: drag the chip anywhere on
                        // it. Dropped in the middle third of both axes the
                        // popout is centred (a modal surface); dropped near an
                        // edge it docks there, and the thirds along that edge
                        // pick start | center | end. Edits write live via
                        // ryoku-plugins-place. Desktop widgets do not come
                        // through here (they are placed on the wallpaper), so
                        // this renders for framePopout only.
                        Item {
                            id: placer
                            width: parent.width
                            readonly property real stageH: 202
                            implicitHeight: placer.stageH + 28
                                + (centreNote.visible ? centreNote.height + Tokens.s2 : 0)
                            visible: detail.enabled && detail.host === "framePopout"

                            readonly property var fp: (detail.place && detail.place.framePopout) ? detail.place.framePopout : ({})
                            readonly property string edge: placer.fp.edge ? placer.fp.edge : "right"
                            readonly property string align: placer.fp.align ? placer.fp.align : "start"
                            // live mirrors so a drop snaps instantly, before the
                            // ryoku-plugins-place round-trip refreshes the truth.
                            property string edgeLocal: placer.edge
                            property string alignLocal: placer.align
                            onEdgeChanged: placer.edgeLocal = placer.edge
                            onAlignChanged: placer.alignLocal = placer.align
                            readonly property bool centered: placer.edgeLocal === "center"
                            readonly property bool vertical: placer.edgeLocal === "left" || placer.edgeLocal === "right"
                            function hoverW() { return placer.fp.hoverW ? placer.fp.hoverW : 320; }
                            function hoverH() { return placer.fp.hoverH ? placer.fp.hoverH : 16; }
                            // thirds of an axis: start | center | end.
                            function third(fraction) {
                                return fraction < 1 / 3 ? "start" : fraction < 2 / 3 ? "center" : "end";
                            }
                            function alongPos(span, size) {
                                var m = 10;
                                return placer.alignLocal === "start" ? m
                                     : placer.alignLocal === "end" ? span - size - m
                                     : (span - size) / 2;
                            }
                            function chipX() {
                                var m = 10;
                                return placer.centered ? (stage.width - chip.width) / 2
                                     : placer.edgeLocal === "left" ? m
                                     : placer.edgeLocal === "right" ? stage.width - chip.width - m
                                     : placer.alongPos(stage.width, chip.width);
                            }
                            function chipY() {
                                var m = 10;
                                return placer.centered ? (stage.height - chip.height) / 2
                                     : placer.edgeLocal === "top" ? m
                                     : placer.edgeLocal === "bottom" ? stage.height - chip.height - m
                                     : placer.alongPos(stage.height, chip.height);
                            }
                            // the middle third of both axes is the screen centre;
                            // anywhere else docks to whichever edge is nearest.
                            function dropEdge(fx, fy) {
                                if (placer.third(fx) === "center" && placer.third(fy) === "center")
                                    return "center";
                                var d = { left: fx, right: 1 - fx, top: fy, bottom: 1 - fy };
                                var best = "top";
                                for (var e in d) if (d[e] < d[best]) best = e;
                                return best;
                            }
                            function dropAlign(edge, fx, fy) {
                                return edge === "center" ? "center"
                                     : (edge === "left" || edge === "right") ? placer.third(fy)
                                     : placer.third(fx);
                            }

                            Rectangle {
                                id: stage
                                anchors.horizontalCenter: parent.horizontalCenter
                                anchors.top: parent.top
                                width: Math.min(placer.width, placer.stageH * 1.6)   // ~16:10
                                height: placer.stageH
                                radius: Tokens.radius
                                color: "transparent"   // no gradient; depth is the hairline
                                border.width: Tokens.border
                                border.color: Tokens.line
                                clip: true

                                // faint inner screen frame, so it reads as "your display".
                                Rectangle {
                                    anchors.fill: parent; anchors.margins: 8
                                    radius: Tokens.radius; color: "transparent"
                                    border.width: Tokens.border; border.color: Tokens.lineSoft
                                }

                                Text {
                                    anchors.top: parent.top; anchors.left: parent.left; anchors.margins: 10
                                    text: I18n.tr("LIVE PLACEMENT")
                                    color: Tokens.inkFaint; font.family: Tokens.mono
                                    font.pixelSize: Tokens.fTiny; font.weight: Font.Medium
                                    font.letterSpacing: 2
                                }

                                // the centre drop zone: land the chip here for a
                                // popout that floats in the middle of the screen.
                                Rectangle {
                                    id: centreZone
                                    x: parent.width / 3
                                    y: parent.height / 3
                                    width: parent.width / 3
                                    height: parent.height / 3
                                    radius: Tokens.radius
                                    color: placer.centered ? Tokens.tint5 : "transparent"
                                    Canvas {
                                        anchors.fill: parent
                                        onPaint: {
                                            var ctx = getContext("2d");
                                            ctx.reset();
                                            ctx.strokeStyle = Tokens.inkFaint.toString();
                                            ctx.lineWidth = 1;
                                            ctx.setLineDash([4, 3]);
                                            ctx.strokeRect(0.5, 0.5, width - 1, height - 1);
                                        }
                                        onWidthChanged: requestPaint()
                                        onHeightChanged: requestPaint()
                                    }
                                    Text {
                                        anchors.centerIn: parent
                                        visible: !placer.centered
                                        text: I18n.tr("centre")
                                        color: Tokens.inkFaint; font.family: Tokens.mono
                                        font.pixelSize: Tokens.fTiny; font.letterSpacing: 1.5
                                    }
                                }

                                // the popout body: drag it anywhere on the stage;
                                // where it lands picks the edge and the align.
                                Rectangle {
                                    id: chip
                                    width: placer.vertical ? 64 : 96
                                    height: placer.vertical ? 96 : 60
                                    radius: Tokens.radius
                                    color: Tokens.paperLift
                                    border.width: Tokens.border
                                    border.color: Tokens.ink
                                    x: placer.chipX()
                                    y: placer.chipY()

                                    Text {
                                        anchors.centerIn: parent
                                        text: I18n.tr("popout")
                                        color: Tokens.ink; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny
                                    }

                                    MouseArea {
                                        anchors.fill: parent
                                        cursorShape: Qt.PointingHandCursor
                                        drag.target: chip
                                        drag.minimumX: 0
                                        drag.maximumX: stage.width - chip.width
                                        drag.minimumY: 0
                                        drag.maximumY: stage.height - chip.height
                                        onReleased: {
                                            var fx = (chip.x + chip.width / 2) / stage.width;
                                            var fy = (chip.y + chip.height / 2) / stage.height;
                                            var e = placer.dropEdge(fx, fy);
                                            var a = placer.dropAlign(e, fx, fy);
                                            placer.edgeLocal = e;                     // snap instantly
                                            placer.alignLocal = a;
                                            chip.x = Qt.binding(function () { return placer.chipX(); });
                                            chip.y = Qt.binding(function () { return placer.chipY(); });
                                            pg.place(detail.sel.id, "framePopout", e, a, placer.hoverW(), placer.hoverH());
                                        }
                                    }
                                }
                            }

                            // edge selector: the four edges plus the screen centre.
                            Seg {
                                id: edgeSeg
                                anchors.horizontalCenter: parent.horizontalCenter
                                anchors.top: stage.bottom
                                anchors.topMargin: Tokens.s2
                                options: ["Center", "Top", "Right", "Bottom", "Left"]
                                current: placer.edgeLocal.charAt(0).toUpperCase() + placer.edgeLocal.slice(1)
                                onChose: (label) => {
                                    var e = label.toLowerCase();
                                    var a = e === "center" ? "center" : placer.alignLocal;
                                    placer.edgeLocal = e;
                                    placer.alignLocal = a;
                                    pg.place(detail.sel.id, "framePopout", e, a, placer.hoverW(), placer.hoverH());
                                }
                            }

                            // a centred popout has no edge to hover, and it shares
                            // the middle of the screen with the shell's own
                            // surfaces. Only one surface is open at a time, so this
                            // is a note, not a restriction.
                            Text {
                                id: centreNote
                                anchors.top: edgeSeg.bottom
                                anchors.topMargin: Tokens.s2
                                anchors.left: parent.left
                                anchors.right: parent.right
                                visible: placer.centered
                                wrapMode: Text.WordWrap
                                text: I18n.tr("A centred popout has no hover edge: open it with its keybind, or with ryoku-shell plugin <id>. It shares the middle of the screen with the sidebars (Super+Escape and Super+S), and the shell shows one surface at a time, so they take turns instead of overlapping.")
                                color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fTiny
                            }
                        }

                        Item {
                            id: sidebarPlacement
                            width: parent.width
                            visible: detail.enabled && detail.host === "sidebarCard"
                            implicitHeight: visible ? sidebarPlacementCol.implicitHeight : 0

                            readonly property var card: (detail.place && detail.place.sidebarCard)
                                ? detail.place.sidebarCard : ({})
                            readonly property var defaults: (detail.man.defaults && detail.man.defaults.sidebar)
                                ? detail.man.defaults.sidebar : ({})
                            readonly property string tabName: sidebarPlacement.card.tab
                                || sidebarPlacement.defaults.tab || "Plugins"
                            readonly property int order: sidebarPlacement.card.order !== undefined
                                ? Number(sidebarPlacement.card.order)
                                : (sidebarPlacement.defaults.order !== undefined
                                    ? Number(sidebarPlacement.defaults.order) : 10)
                            readonly property string label: sidebarPlacement.card.label
                                || ((detail.man.defaults || {}).label) || detail.man.name || detail.sel.id
                            readonly property string glyph: sidebarPlacement.card.glyph
                                || ((detail.man.defaults || {}).glyph) || "extension"

                            function commit(tab, order) {
                                pg.place(detail.sel.id, "sidebarCard", tab, order,
                                    sidebarPlacement.label, sidebarPlacement.glyph);
                            }

                            Column {
                                id: sidebarPlacementCol
                                width: parent.width
                                spacing: Tokens.s3


                                Item {
                                    width: parent.width
                                    height: 30
                                    Text {
                                        anchors.left: parent.left
                                        anchors.right: sidebarTab.left
                                        anchors.rightMargin: Tokens.s3
                                        anchors.verticalCenter: parent.verticalCenter
                                        text: I18n.tr("Tab")
                                        color: Tokens.inkDim
                                        font.family: Tokens.ui
                                        font.pixelSize: Tokens.fBody
                                    }
                                    Field {
                                        id: sidebarTab
                                        anchors.right: parent.right
                                        anchors.verticalCenter: parent.verticalCenter
                                        width: 220
                                        text: sidebarPlacement.tabName
                                        placeholder: I18n.tr("Plugins")
                                        onCommitted: value => sidebarPlacement.commit(
                                            String(value).trim() || "Plugins",
                                            sidebarPlacement.order)
                                    }
                                }

                                Item {
                                    width: parent.width
                                    height: 30
                                    Text {
                                        anchors.left: parent.left
                                        anchors.verticalCenter: parent.verticalCenter
                                        text: I18n.tr("Order")
                                        color: Tokens.inkDim
                                        font.family: Tokens.ui
                                        font.pixelSize: Tokens.fBody
                                    }
                                    Row {
                                        anchors.right: parent.right
                                        anchors.verticalCenter: parent.verticalCenter
                                        spacing: Tokens.s2
                                        Text {
                                            anchors.verticalCenter: parent.verticalCenter
                                            text: sidebarPlacement.order
                                            color: Tokens.ink
                                            font.family: Tokens.mono
                                            font.pixelSize: Tokens.fBody
                                        }
                                        Step {
                                            value: sidebarPlacement.order
                                            from: -100
                                            to: 100
                                            stepBy: 1
                                            onModified: value => sidebarPlacement.commit(
                                                sidebarPlacement.tabName, value)
                                        }
                                    }
                                }
                            }
                        }

                        // desktop-widget hint: those are placed on the wallpaper.
                        Text {
                            width: parent.width
                            visible: detail.enabled && detail.host === "desktopWidget"
                            text: I18n.tr("Desktop widgets are moved, resized and hidden on the wallpaper. Drag the tile, or right-click it for its menu.")
                            color: Tokens.inkMuted; font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap
                        }
                    }

                    // ── Settings: the plugin's own fields, from its schema ──
                    Column {
                        width: parent.width
                        spacing: Tokens.s3
                        visible: detail.schema.length > 0

                        Item {
                            width: parent.width; height: 20
                            Row {
                                id: sHead
                                anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                                spacing: Tokens.s2
                                Rectangle { width: 4; height: 4; color: Tokens.ink; anchors.verticalCenter: parent.verticalCenter }
                                Text {
                                    text: I18n.tr("SETTINGS"); color: Tokens.ink; font.family: Tokens.ui
                                    font.pixelSize: Tokens.fMicro; font.weight: Font.Medium
                                    font.letterSpacing: Tokens.trackMark
                                    anchors.verticalCenter: parent.verticalCenter
                                }
                            }
                            Rectangle {
                                anchors.left: sHead.right; anchors.leftMargin: Tokens.s3
                                anchors.right: parent.right
                                anchors.verticalCenter: parent.verticalCenter
                                height: 1; color: Tokens.lineSoft
                            }
                        }

                        // The settings form: a generic renderer that turns the
                        // plugin's declared schema into native controls. Nothing
                        // here is hardcoded per plugin; the type -> control switch
                        // (default arm = text, so unknown types stay editable) and
                        // a live local value mirror carry every field. Each change
                        // fires setSetting() -> plugins.json.
                        Column {
                            id: form
                            width: parent.width
                            spacing: Tokens.s4

                            property var values: detail.place.settings || ({})
                            property var _local: ({})
                            onValuesChanged: form._local = JSON.parse(JSON.stringify(form.values || {}))
                            Component.onCompleted: form._local = JSON.parse(JSON.stringify(form.values || {}))

                            function _val(field) {
                                if (form._local && form._local[field.key] !== undefined)
                                    return form._local[field.key];
                                return field.default;
                            }
                            function _set(key, value) {
                                var n = JSON.parse(JSON.stringify(form._local || {}));
                                n[key] = value;
                                form._local = n;
                                pg.setSetting(detail.sel.id, key, value);
                            }
                            function _choiceLabel(field, val) {
                                var os = field.options || [];
                                for (var i = 0; i < os.length; i++)
                                    if (String(os[i].value) === String(val))
                                        return os[i].label;
                                return String(val);
                            }
                            function _choiceKeyOf(field, label) {
                                var os = field.options || [];
                                for (var i = 0; i < os.length; i++)
                                    if (os[i].label === label)
                                        return os[i].value;
                                return label;
                            }

                            Repeater {
                                model: detail.schema

                                delegate: Column {
                                    id: fieldWrap
                                    required property var modelData
                                    required property int index
                                    width: form.width
                                    spacing: Tokens.s3

                                    readonly property string grp: modelData.group || ""
                                    readonly property bool startsGroup: fieldWrap.index === 0
                                        || ((detail.schema[fieldWrap.index - 1].group || "") !== fieldWrap.grp)

                                    // group header (grotesk section caps + hairline),
                                    // once per distinct group; blank group = none.
                                    Item {
                                        width: parent.width
                                        height: 16
                                        visible: fieldWrap.startsGroup && fieldWrap.grp.length > 0
                                        Text {
                                            id: gHead
                                            anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                                            text: fieldWrap.grp
                                            color: Tokens.inkMuted; font.family: Tokens.ui
                                            font.pixelSize: Tokens.fMicro; font.weight: Font.Medium
                                            font.letterSpacing: Tokens.trackMark
                                            font.capitalization: Font.AllUppercase
                                        }
                                        Rectangle {
                                            anchors.left: gHead.right; anchors.leftMargin: Tokens.s3
                                            anchors.right: parent.right
                                            anchors.verticalCenter: parent.verticalCenter
                                            height: 1; color: Tokens.lineSoft
                                        }
                                    }

                                    Loader {
                                        width: parent.width
                                        sourceComponent: {
                                            switch (fieldWrap.modelData.type) {
                                            case "choice": return cChoice;
                                            case "toggle": return cToggle;
                                            case "slider": return cSlider;
                                            case "image": return cImage;
                                            default: return cText;   // text + anything unknown
                                            }
                                        }
                                        onLoaded: item.field = fieldWrap.modelData
                                    }
                                }
                            }

                            // ── field control templates ──

                            Component {
                                id: cToggle
                                Item {
                                    id: ct
                                    property var field: ({})
                                    width: form.width
                                    implicitHeight: 30
                                    Text {
                                        anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                                        anchors.right: ctSw.left; anchors.rightMargin: Tokens.s3
                                        elide: Text.ElideRight
                                        text: ct.field.label || ct.field.key
                                        color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fBody
                                    }
                                    Sw {
                                        id: ctSw
                                        anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                                        on: form._val(ct.field) === true || form._val(ct.field) === "true"
                                        onToggled: (v) => form._set(ct.field.key, v)
                                    }
                                }
                            }

                            // choice: a seg for <=4 options, a wrapped chip band
                            // for more (both invert the selected member; neither
                            // needs an overlay, so the form stays self-contained).
                            Component {
                                id: cChoice
                                Column {
                                    id: cc
                                    property var field: ({})
                                    width: form.width
                                    spacing: Tokens.s2
                                    readonly property var _labels: (cc.field.options || []).map(function (o) { return o.label; })
                                    readonly property string _curLabel: form._choiceLabel(cc.field, String(form._val(cc.field)))
                                    readonly property bool _few: cc._labels.length <= 4

                                    Item {
                                        visible: cc._few
                                        width: cc.width
                                        height: 30
                                        Text {
                                            anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                                            anchors.right: segFew.left; anchors.rightMargin: Tokens.s3
                                            elide: Text.ElideRight
                                            text: cc.field.label || cc.field.key
                                            color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fBody
                                        }
                                        Seg {
                                            id: segFew
                                            anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                                            options: cc._labels
                                            current: cc._curLabel
                                            onChose: (label) => form._set(cc.field.key, form._choiceKeyOf(cc.field, label))
                                        }
                                    }
                                    Text {
                                        visible: !cc._few
                                        width: cc.width
                                        text: cc.field.label || cc.field.key
                                        color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fBody
                                    }
                                    Chips {
                                        visible: !cc._few
                                        width: cc.width
                                        options: cc._labels
                                        current: cc._curLabel
                                        onChose: (label) => form._set(cc.field.key, form._choiceKeyOf(cc.field, label))
                                    }
                                }
                            }

                            // slider: reuses the integer-domain Slid by mapping the
                            // field's range onto its step count, so decimals and
                            // step survive; the numeral is the live readout.
                            Component {
                                id: cSlider
                                Item {
                                    id: cs
                                    property var field: ({})
                                    width: form.width
                                    implicitHeight: 30
                                    readonly property real _from: cs.field.min !== undefined ? cs.field.min : 0
                                    readonly property real _to: cs.field.max !== undefined ? cs.field.max : 1
                                    readonly property int _dec: cs.field.decimals !== undefined ? cs.field.decimals : 2
                                    readonly property real _step: cs.field.step !== undefined ? cs.field.step : (cs._dec === 0 ? 1 : 0.01)
                                    readonly property real _v: Number(form._val(cs.field))
                                    readonly property int _steps: Math.max(1, Math.round((cs._to - cs._from) / cs._step))

                                    Text {
                                        anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                                        anchors.right: csNum.left; anchors.rightMargin: Tokens.s3
                                        elide: Text.ElideRight
                                        text: cs.field.label || cs.field.key
                                        color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fBody
                                    }
                                    Text {
                                        id: csNum
                                        anchors.right: csTrack.left; anchors.rightMargin: Tokens.s3
                                        anchors.verticalCenter: parent.verticalCenter
                                        text: cs._dec === 0 ? ("" + Math.round(cs._v)) : cs._v.toFixed(cs._dec)
                                        color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fBody
                                    }
                                    Slid {
                                        id: csTrack
                                        anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                                        width: 220
                                        from: 0
                                        to: cs._steps
                                        value: Math.round((cs._v - cs._from) / cs._step)
                                        onModified: (iv) => {
                                            var actual = cs._from + iv * cs._step;
                                            form._set(cs.field.key, cs._dec === 0 ? Math.round(actual) : Number(actual.toFixed(cs._dec)));
                                        }
                                    }
                                }
                            }

                            Component {
                                id: cText
                                Item {
                                    id: cx
                                    property var field: ({})
                                    width: form.width
                                    implicitHeight: 30
                                    Text {
                                        anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                                        anchors.right: cxBox.left; anchors.rightMargin: Tokens.s3
                                        elide: Text.ElideRight
                                        text: cx.field.label || cx.field.key
                                        color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fBody
                                    }
                                    Field {
                                        id: cxBox
                                        anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                                        width: 220
                                        placeholder: cx.field.placeholder || ""
                                        text: String(form._val(cx.field) || "")
                                        // commit on editing-finished only, like every field.
                                        onCommitted: (v) => form._set(cx.field.key, v)
                                    }
                                }
                            }

                            // image: a labelled box that opens the system file
                            // chooser (through the XDG portal); stores a file:// URL.
                            Component {
                                id: cImage
                                Item {
                                    id: ci
                                    property var field: ({})
                                    width: form.width
                                    implicitHeight: 30
                                    readonly property string cur: String(form._val(ci.field) || "")
                                    Text {
                                        anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                                        anchors.right: ciBox.left; anchors.rightMargin: Tokens.s3
                                        elide: Text.ElideRight
                                        text: ci.field.label || ci.field.key
                                        color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fBody
                                    }
                                    Rectangle {
                                        id: ciBox
                                        anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                                        width: 220; height: 30
                                        radius: Tokens.radius
                                        color: ciHov.hovered ? Tokens.tint10 : "transparent"
                                        border.width: Tokens.border
                                        border.color: ciHov.hovered ? Tokens.lineStrong : Tokens.line
                                        Behavior on color { ColorAnimation { duration: Tokens.snap } }
                                        Behavior on border.color { ColorAnimation { duration: Tokens.snap } }
                                        Text {
                                            anchors.left: parent.left; anchors.leftMargin: 9
                                            anchors.right: parent.right; anchors.rightMargin: 9
                                            anchors.verticalCenter: parent.verticalCenter
                                            elide: Text.ElideLeft
                                            text: ci.cur.length === 0 ? I18n.tr("Choose image\u2026") : ci.cur.replace(/^.*\//, "")
                                            color: ci.cur.length === 0 ? Tokens.inkMuted : Tokens.ink
                                            font.family: ci.cur.length === 0 ? Tokens.ui : Tokens.mono
                                            font.pixelSize: ci.cur.length === 0 ? 12 : 11
                                        }
                                        HoverHandler { id: ciHov; cursorShape: Qt.PointingHandCursor }
                                        TapHandler { onTapped: { detail.pendingImageField = ci.field; ciDlg.open(); } }
                                    }
                                }
                            }
                        }
                    }

                    // sibling empty state, agreeing with the form's own gate.
                    Text {
                        width: parent.width
                        visible: detail.schema.length === 0
                        text: I18n.tr("This add-on has no configurable settings.")
                        color: Tokens.inkFaint; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                    }
                }
            }

            // image-type plugin option picker; lifted to the detail root so it
            // overlays as a full modal (the delegate row itself is only 30px tall).
            // The tapped field is captured in pendingImageField, like SchemaPage.
            PickFile {
                id: ciDlg
                title: I18n.tr("Choose an image")
                onPicked: (p) => { if (detail.pendingImageField) form._set(detail.pendingImageField.key, "" + p); ciDlg.active = false; }
                onCanceled: ciDlg.active = false
            }
        }
    }

    AddonBundleSheet {
        anchors.fill: parent
        z: 90
        open: pg.bundleSheetOpen
        bundle: pg.bundleSheetData
        loading: pg.bundleSheetLoading
        busy: pg.bundleBusyId === pg.bundleSheetId
        onDismissed: pg.bundleSheetOpen = false
        onRepairRequested: pg.repairBundle(pg.bundleSheetId)
        onRemoveRequested: pg.removeBundle(pg.bundleSheetId, "")
    }

    // ── destructive confirm: a bone plate, 2px border, an unambiguous verb ──
    // No red anywhere: inversion and the word carry the weight (DESIGN.md
    // sections 1 and 4). The scrim has no fill (translucency is banned); the
    // plate reads as an overlay by being bone on black.
    MouseArea {
        id: confirmScrim
        anchors.fill: parent
        visible: pg.confirmRemove
        z: 100
        onClicked: pg.confirmRemove = false

        Rectangle {
            id: plate
            anchors.centerIn: parent
            width: 380
            height: plateCol.implicitHeight + Tokens.s5 * 2
            radius: Tokens.radius
            color: Tokens.bone
            border.width: 2
            border.color: Tokens.inkOnBone

            // absorb clicks inside the plate so they do not dismiss it.
            MouseArea { anchors.fill: parent }

            Column {
                id: plateCol
                anchors.centerIn: parent
                width: parent.width - Tokens.s5 * 2
                spacing: Tokens.s4

                Text {
                    width: parent.width
                    text: I18n.tr("Remove %1?").arg(pg.confirmName || pg.confirmId || I18n.tr("add-on"))
                    color: Tokens.inkOnBone; font.family: Tokens.ui
                    font.pixelSize: Tokens.fValue; font.weight: Font.Medium
                    wrapMode: Text.WordWrap
                }
                Text {
                    width: parent.width
                    text: pg.confirmKind === "omarchy"
                        ? I18n.tr("This removes the add-on from the Omarchy bar. You can install it again from the market.")
                        : I18n.tr("This deletes the add-on and its settings from your desktop. You can reinstall it from RyoStore.")
                    color: Tokens.inkOnBoneDim; font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap
                }
                Row {
                    anchors.right: parent.right
                    spacing: Tokens.s3

                    Rectangle {
                        width: cancelT.implicitWidth + 30; height: 32; radius: Tokens.radius
                        color: cancelH.hovered ? Tokens.lineOnBone : "transparent"
                        border.width: Tokens.border; border.color: Tokens.inkOnBoneDim
                        Behavior on color { ColorAnimation { duration: Tokens.snap } }
                        Text {
                            id: cancelT
                            anchors.centerIn: parent
                            text: I18n.tr("CANCEL"); color: Tokens.inkOnBone
                            font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                            font.weight: Font.Medium; font.letterSpacing: Tokens.trackLabel
                        }
                        HoverHandler { id: cancelH; cursorShape: Qt.PointingHandCursor }
                        TapHandler { onTapped: pg.confirmRemove = false }
                    }
                    // the committed verb: black on bone, inversion within inversion.
                    Rectangle {
                        width: rmT.implicitWidth + 30; height: 32; radius: Tokens.radius
                        color: rmH.hovered ? Tokens.inkOnBoneDim : Tokens.inkOnBone
                        Behavior on color { ColorAnimation { duration: Tokens.snap } }
                        Text {
                            id: rmT
                            anchors.centerIn: parent
                            text: I18n.tr("REMOVE"); color: Tokens.bone
                            font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                            font.weight: Font.Medium; font.letterSpacing: Tokens.trackLabel
                        }
                        HoverHandler { id: rmH; cursorShape: Qt.PointingHandCursor }
                        TapHandler { onTapped: pg.confirmRemoval() }
                    }
                }
            }
        }
    }
}
