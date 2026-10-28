pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons

Singleton {
    id: root

    property var catalog: ({})
    property var categories: []
    property var items: []
    property bool loading: false
    property bool offline: false
    property string generatedAt: ""
    readonly property bool refreshing: loading && items.length > 0
    property string error: ""
    property string busyKey: ""
    property string installStage: ""
    property string installProgressStage: ""
    property real installProgressDone: 0
    property real installProgressTotal: 0
    property string installError: ""
    property string installErrorKey: ""
    property string _catalogOutput: ""
    property string _catalogError: ""
    property string _installError: ""
    property bool _clearBusyAfterRefresh: false
    property var _queue: []
    property bool updateAvailable: false
    property string revision: ""
    property bool _forced: false
    property string _checkOutput: ""
    property bool _reloadAfterCheck: false
    property var remoteItems: []
    property string remoteResultQuery: ""
    property bool remoteSearching: false
    property string remoteError: ""
    property string _remoteWanted: ""
    property string _remoteRunning: ""
    property string _remoteOutput: ""
    property string _remoteErrorText: ""
    property string _checkRevision: ""
    property bool _autoRefreshAttempted: false
    property string _op: "install"            // which flow installProc is running
    property string _failedOp: ""

    function operationStage(op) {
        if (op === "remove")
            return "REMOVING";
        if (op === "enable")
            return "ENABLING";
        if (op === "disable")
            return "DISABLING";
        return "INSTALLING";
    }

    function resetInstallProgress() {
        installProgressStage = "";
        installProgressDone = 0;
        installProgressTotal = 0;
    }

    function progressStageLabel(stage) {
        if (stage === "download")
            return "DOWNLOADING";
        if (stage === "extract")
            return "EXTRACTING";
        if (stage === "verify")
            return "VERIFYING";
        if (stage === "apply")
            return "APPLYING";
        return root.operationStage(root._op);
    }

    function readInstallLine(line) {
        var raw = String(line);
        var match = raw.trim().match(/^RYOSTORE-PROGRESS (download|extract|verify|apply) ([0-9]+) ([0-9]+)$/);
        if (match) {
            installProgressStage = match[1];
            installProgressDone = Number(match[2]);
            installProgressTotal = Number(match[3]);
            installStage = progressStageLabel(match[1]);
            return;
        }
        if (raw !== "")
            _installError += raw + "\n";
    }

    function operationFailure(op) {
        if (op === "remove")
            return I18n.tr("Removal failed");
        if (op === "enable")
            return I18n.tr("Enabling failed");
        if (op === "disable")
            return I18n.tr("Disabling failed");
        return I18n.tr("Installation failed");
    }

    function itemKey(item) {
        return item ? String(item.category || "") + ":" + String(item.id || "") : "";
    }

    function category(id) {
        for (let i = 0; i < categories.length; i++)
            if (categories[i].id === id)
                return categories[i];
        return null;
    }

    function searchRemote(category, query) {
        const wanted = String(category) + "\n" + String(query);
        if (wanted === _remoteWanted)
            return;
        _remoteWanted = wanted;
        remoteResultQuery = "";
        remoteItems = [];
        remoteError = "";
        remoteSearching = true;
        if (!remoteProc.running)
            _startRemote();
    }

    function clearRemote() {
        _remoteWanted = "";
        remoteResultQuery = "";
        remoteItems = [];
        remoteError = "";
        remoteSearching = false;
    }

    function _startRemote() {
        const parts = _remoteWanted.split("\n");
        _remoteRunning = _remoteWanted;
        _remoteOutput = "";
        _remoteErrorText = "";
        remoteProc.command = ["ryostore", "search", parts[0], parts.slice(1).join("\n")];
        remoteProc.running = true;
    }

    function refresh(force) {
        if (catalogProc.running)
            return;
        root._forced = force === true;
        loading = true;
        error = "";
        _catalogOutput = "";
        _catalogError = "";
        catalogProc.command = force ? ["ryostore", "catalog", "--refresh"] : ["ryostore", "catalog"];
        catalogProc.running = true;
    }

    function refreshIfStale() {
        if (!root._autoRefreshAttempted && !catalogProc.running && !root.loading
                && root.revision !== "" && root._checkRevision !== ""
                && root.revision !== root._checkRevision) {
            root._autoRefreshAttempted = true;
            root.refresh(true);
        }
    }

    // A lightweight upstream probe (registries only, no downloads): it lights the
    // refresh dot when ryostore has advanced past what the user last pulled,
    // without touching the shown catalogue.
    function check() {
        if (checkProc.running)
            return;
        root._checkOutput = "";
        checkProc.running = true;
    }

    // Stop every child process before the window tears down, so no fetch is in
    // flight when the QML engine is destroyed -- a shutdown-crash guard.
    function shutdown() {
        catalogProc.running = false;
        installProc.running = false;
        checkProc.running = false;
        clearRemote();
        remoteProc.running = false;
    }

    function install(item, dither, components) {
        // A paused item, or one written for another window manager, stays listed
        // but never downloads: install and update are refused here so no UI path
        // (button, keyboard, or accessibility) can start a fetch. Remove is a
        // separate flow and stays allowed.
        if (!item || busyKey !== "" || item.downloadPaused === true || item.unavailable === true)
            return;
        busyKey = itemKey(item);
        root._op = "install";
        installStage = "FETCHING";
        installError = "";
        installErrorKey = "";
        root._failedOp = "";
        _installError = "";
        var cmd = ["ryostore", "install", String(item.category), String(item.id)];
        // what you see is what you install: the catalogue leads with the colour
        // original, so an install with no explicit choice takes that one
        var wantDither = (dither === undefined) ? false : dither;
        if (wantDither && String(item.artRaw || "") !== "")
            cmd.push("--dither");
        if (Array.isArray(components) && components.length > 0)
            cmd.push("--only", components.join(","));
        installProc.command = cmd;
        installProc.running = true;
    }

    // remove uninstalls an installed product, whatever its category (every
    // provider implements remove). Single-flight like install and allowed even
    // for a download-paused item, whose installed copy can still be taken off.
    function remove(item) {
        if (!item || busyKey !== "" || item.installed !== true && item.active !== true
                && item.enabled !== true && Number(item.installedCount || 0) <= 0)
            return;
        busyKey = itemKey(item);
        root._op = "remove";
        installStage = "REMOVING";
        installError = "";
        installErrorKey = "";
        root._failedOp = "";
        _installError = "";
        installProc.command = ["ryostore", "remove", String(item.category), String(item.id)];
        installProc.running = true;
    }

    function setOmarchyEnabled(item, enabled) {
        if (!item || String(item.category || "") !== "omarchy-plugins"
                || item.installed !== true || busyKey !== "")
            return;
        busyKey = itemKey(item);
        root._op = enabled ? "enable" : "disable";
        installStage = enabled ? "ENABLING" : "DISABLING";
        installError = "";
        installErrorKey = "";
        root._failedOp = "";
        _installError = "";
        installProc.command = ["omarchy", "plugin", enabled ? "enable" : "disable", String(item.id)];
        installProc.running = true;
    }

    function clearInstallError(item) {
        if (installErrorKey !== itemKey(item))
            return;
        installError = "";
        installErrorKey = "";
        installStage = "";
        root._failedOp = "";
    }

    function retryInstall(item, dither, components) {
        var failed = root._failedOp;
        clearInstallError(item);
        if (failed === "enable" || failed === "disable")
            setOmarchyEnabled(item, failed === "enable");
        else if (failed === "remove")
            remove(item);
        else
            install(item, dither, components);
    }

    // installAll queues every not-yet-installed item and installs them one at a
    // time (install is single-flight): each completion pumps the next. The batch
    // stops on the first failure so the error stays visible.
    function installAll(list) {
        var q = [];
        var src = Array.isArray(list) ? list : [];
        for (var i = 0; i < src.length; i++)
            if (src[i] && src[i].installed !== true && src[i].downloadPaused !== true && src[i].unavailable !== true)
                q.push(src[i]);
        _queue = q;
        _pumpQueue();
    }
    function _pumpQueue() {
        if (busyKey !== "" || _queue.length === 0)
            return;
        var next = _queue[0];
        _queue = _queue.slice(1);
        install(next);
    }

    function openSettings(item) {
        if (!item)
            return;
        Quickshell.execDetached(["ryostore", "settings", String(item.category), String(item.id)]);
    }

    Component.onCompleted: {
        refresh(false);
        check();
    }

    Process {
        id: catalogProc
        stdout: StdioCollector { onStreamFinished: root._catalogOutput = text }
        stderr: StdioCollector { onStreamFinished: root._catalogError = text }
        onExited: code => {
            root.loading = false;
            if (code !== 0) {
                root.error = root._catalogError.trim() || I18n.tr("Catalogue failed");
                if (root._clearBusyAfterRefresh) {
                    root._clearBusyAfterRefresh = false;
                    root.busyKey = "";
                }
                return;
            }
            try {
                const next = JSON.parse(root._catalogOutput);
                if (!Array.isArray(next.categories) || !Array.isArray(next.items))
                    throw new Error("catalogue arrays are missing");
                root.catalog = next;
                root.categories = next.categories.slice();
                root.items = next.items.slice();
                root.offline = next.offline === true;
                root.generatedAt = String(next.generatedAt || "");
                root.error = "";
                root.revision = String(next.revision || "");
                if (root._forced)
                    root.updateAvailable = false;
                Qt.callLater(() => root.refreshIfStale());
                // Fill the on-disk asset cache out of band so the next open (and
                // this session's later scrolling) renders previews from disk.
                Quickshell.execDetached(["ryostore", "warm"]);
                if (root._clearBusyAfterRefresh) {
                    root._clearBusyAfterRefresh = false;
                    root.installStage = "COMPLETE";
                    root.busyKey = "";
                }
            } catch (e) {
                root.error = I18n.tr("Invalid catalogue: %1").arg(e);
                if (root._clearBusyAfterRefresh) {
                    root._clearBusyAfterRefresh = false;
                    root.busyKey = "";
                }
            }
            root._pumpQueue();
            if (root._reloadAfterCheck) {
                root._reloadAfterCheck = false;
                root.refresh(false);
            }
        }
    }

    Process {
        id: installProc
        stderr: SplitParser { onRead: line => root.readInstallLine(line) }
        onRunningChanged: {
            if (!running)
                return;
            root.resetInstallProgress();
            root._installError = "";
            root.installStage = root.operationStage(root._op);
        }
        onExited: code => {
            if (code !== 0) {
                root.resetInstallProgress();
                root.installStage = "FAILED";
                root.installError = root._installError.trim()
                        || root.operationFailure(root._op);
                root._failedOp = root._op;
                root.busyKey = "";
                root._queue = [];
                return;
            }
            root.resetInstallProgress();
            root.installStage = "VERIFYING";
            root._clearBusyAfterRefresh = true;
            root.refresh(true);
        }
    }

    Process {
        id: remoteProc
        stdout: StdioCollector { onStreamFinished: root._remoteOutput = text }
        stderr: StdioCollector { onStreamFinished: root._remoteErrorText = text }
        onExited: code => {
            const answered = root._remoteRunning;
            root._remoteRunning = "";
            if (root._remoteWanted === "") {
                root.remoteSearching = false;
                return;
            }
            if (root._remoteWanted !== answered) {
                root._startRemote();
                return;
            }
            root.remoteSearching = false;
            if (code !== 0) {
                root.remoteItems = [];
                root.remoteError = root._remoteErrorText.trim() || I18n.tr("Search failed");
                return;
            }
            try {
                const res = JSON.parse(root._remoteOutput);
                if (!Array.isArray(res.items))
                    throw new Error("search items are missing");
                root.remoteResultQuery = answered.split("\n").slice(1).join("\n");
                root.remoteItems = res.items.slice();
            } catch (e) {
                root.remoteItems = [];
                root.remoteError = I18n.tr("Search failed");
            }
        }
    }

    Process {
        id: checkProc
        command: ["ryostore", "check"]
        stdout: StdioCollector { onStreamFinished: root._checkOutput = text }
        onExited: code => {
            if (code !== 0)
                return;
            const reloading = catalogProc.running;
            try {
                const res = JSON.parse(root._checkOutput);
                root.updateAvailable = res.updateAvailable === true;
                root._checkRevision = res.offline === true ? "" : String(res.revision || "");
                root.refreshIfStale();
            } catch (e) {
                // A malformed probe never asserts an update.
            }
            // The probe rebuilt the catalogue live and refreshed the launch
            // snapshot. Installed state is local and not part of the revision,
            // so a part installed outside the store (Settings, a terminal) would
            // otherwise stay missing until a manual refresh. A stale rebuild
            // started just above is already live, so it needs no second read.
            if (reloading)
                root._reloadAfterCheck = true;
            else if (!catalogProc.running)
                root.refresh(false);
        }
    }
}
