import QtQuick
import QtQuick.Templates as T
import Quickshell.Io
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../registry/provider-map.js" as ProviderMap
import "../schema/window-manager.js" as Schema

// which window manager runs, switching to another, what it cannot do, and its plugins
Item {
    id: root

    required property var hub

    function title(name) {
        var s = String(name || "");
        return s.length ? s.charAt(0).toUpperCase() + s.slice(1) : "";
    }
    function shq(s) {
        return "'" + String(s).replace(/'/g, "'\\''") + "'";
    }

    // window managers

    property var providers: []
    property bool listed: false
    property string listError: ""
    // the switch was handed to a terminal window
    property string terminalTarget: ""

    readonly property var activeProvider: root.providers.find(p => p.active === true) || null
    readonly property string activeName: Settings.provider !== "" ? Settings.provider : (root.activeProvider ? root.activeProvider.name : "")
    readonly property var pendingItem: Attention.items.find(i => i.id === "wm-switch") || null
    readonly property string pendingName: root.pendingItem && root.pendingItem.target ? String(root.pendingItem.target) : ""
    readonly property bool hasAlternative: root.providers.some(p => p.active !== true && p.selectable === true)

    // a read that failed, in plain words with the next step; a refusal keeps the backend's reason
    function readError(lead, err, code, late) {
        if (late)
            return lead + " " + I18n.tr("It did not answer in time. Retry, and if it fails again, update Ryoku.");
        // Changes.run already put a missing program in plain words
        if (code === -1)
            return lead + " " + String(err || "").trim();
        var lines = String(err || "").trim().split("\n").filter(l => l.trim().length > 0);
        var why = lines.length > 0 ? lines[lines.length - 1].trim() : "";
        return lead + " " + I18n.tr("Retry, and if it fails again, update Ryoku.") + (why ? " (" + why + ")" : "");
    }

    // a program that cannot start or whose interpreter is gone, from a Process's exit
    function missingRun(code, err) {
        return (code === 126 || code === 127) && /bad interpreter|command not found|cannot execute|No such file or directory/.test(String(err || ""));
    }

    // a reload supersedes the one still running; only the newest answer counts
    property int listGen: 0
    function reloadProviders() {
        var gen = ++root.listGen;
        root.listed = false;
        root.listError = "";
        Changes.run(["ryoku-hub", "wm", "list"], (ok, out, err, code, late) => {
            if (gen !== root.listGen)
                return;
            var parsed = null;
            try {
                parsed = JSON.parse(out);
            } catch (e) {}
            root.listed = true;
            if (!ok || !Array.isArray(parsed)) {
                root.listError = root.readError(I18n.tr("The list of window managers could not be read."), err, code, late);
                return;
            }
            root.providers = parsed;
        }, { timeout: 15000 });
    }

    function providerStatus(p) {
        if (p.active === true)
            return root.pendingName !== "" && root.pendingName !== p.name
                ? I18n.tr("In use now, until your next sign-in") : I18n.tr("In use now");
        if (p.name === root.pendingName)
            return I18n.tr("Starts at your next sign-in");
        if (p.installed === true)
            return I18n.tr("Installed");
        if (p.deployed === true)
            return I18n.tr("Ready to use");
        return I18n.tr("Installs when you switch to it");
    }

    function providerRow(p) {
        var switchable = p.active !== true && p.selectable === true && p.name !== root.pendingName;
        return {
            id: "wm-" + p.name, section: "current", label: root.title(p.name),
            detail: root.providerStatus(p), keywords: [p.name],
            control: "custom", activatable: switchable, provider: p, switchable: switchable,
            unavailable: p.active !== true && p.selectable !== true
                ? (p.reason ? I18n.tr("Not available here: %1").replace("%1", p.reason) : I18n.tr("Not available on this computer"))
                : ""
        };
    }

    // pending switch: remembered by the backend, shown on Home until it runs

    function remember(name) {
        pendingProc.command = name === "" ? ["ryoku-hub", "wm", "pending", "clear"] : ["ryoku-hub", "wm", "pending", "set", name];
        pendingProc.running = false;
        pendingProc.running = true;
    }

    function stay() {
        root.terminalTarget = "";
        root.remember("");
    }

    // the switch dialog

    property var swTarget: null
    property var swReport: null
    property bool swLoading: false
    property string swKeep: "keep"
    // review, running, done, failed
    property string swPhase: "review"
    property string swError: ""
    property bool swStopped: false
    property bool swWorking: false
    property bool swSlow: false
    property bool swCancelling: false
    property int swGen: 0

    readonly property string swName: root.swTarget ? root.title(root.swTarget.name) : ""
    readonly property string swActiveName: root.swReport && root.swReport.active ? root.title(root.swReport.active) : root.title(root.activeName)
    readonly property bool swLeaving: root.swReport !== null && root.swReport.active !== "" && root.swReport.active !== root.swTarget.name
    readonly property var swReclaim: root.swReport && root.swReport.reclaim ? root.swReport.reclaim : null
    readonly property bool swRemovable: root.swReclaim !== null && root.swReclaim.removable === true
    readonly property bool swSelectable: root.swReport !== null && root.swReport.selectable === true
    readonly property var swReasons: {
        var seen = {}, out = [];
        var u = root.swReport && root.swReport.unhonored ? root.swReport.unhonored : [];
        for (var i = 0; i < u.length; i++)
            if (u[i] && u[i].reason && !seen[u[i].reason]) {
                seen[u[i].reason] = true;
                out.push(u[i].reason);
            }
        return out;
    }

    function humanSize(bytes) {
        var b = Number(bytes) || 0;
        return b >= 1073741824 ? (b / 1073741824).toFixed(1) + " GB" : Math.max(1, Math.round(b / 1048576)) + " MB";
    }

    function openSwitch(p) {
        if (!p || p.active === true || p.selectable !== true)
            return;
        root.swTarget = p;
        root.swReport = null;
        root.swKeep = "keep";
        root.swPhase = "review";
        root.swError = "";
        root.swStopped = false;
        root.swLoading = true;
        var gen = ++root.swGen;
        Changes.run(["ryoku-hub", "wm", "preview", p.name], (ok, out, err, code, late) => {
            if (gen !== root.swGen)
                return;
            var r = null;
            try {
                r = JSON.parse(out);
            } catch (e) {}
            root.swReport = ok ? r : null;
            if (!root.swReport) {
                root.swError = root.readError(I18n.tr("What carries over could not be worked out."), err, code, late);
                root.swPhase = "failed";
            }
            root.swLoading = false;
        }, { timeout: 15000 });
        switchDialog.open();
    }

    function startSwitch() {
        if (!root.swTarget || root.swLoading || !root.swSelectable || root.swPhase === "running")
            return;
        var name = root.swTarget.name;
        var removing = root.swKeep === "remove" && root.swRemovable;
        // nothing to install and nothing to remove needs no password, so it runs here
        if (root.swReport.deployed === true && !removing) {
            root.swPhase = "running";
            root.swError = "";
            root.swStopped = false;
            root.swCancelling = false;
            useProc.command = ["ryoku", "wm", "use", name, "--keep-previous"];
            useProc.running = false;
            useProc.running = true;
            swWorkingTimer.restart();
            swSlowTimer.restart();
            return;
        }
        // installing asks for a password, so a terminal shows the progress and the prompt
        var line = "ryoku wm use " + root.shq(name) + (removing ? " --remove-previous" : " --keep-previous");
        root.hub.runInTerminal(line, "dev.ryoku.wm_switch", null, I18n.tr("Done. Press Enter to close."));
        root.terminalTarget = name;
        root.remember(name);
        switchDialog.close();
    }

    function cancelSwitch() {
        root.swCancelling = true;
        useProc.running = false;
    }

    function switchFinished(code, err) {
        swWorkingTimer.stop();
        swSlowTimer.stop();
        root.swWorking = false;
        root.swSlow = false;
        if (root.swCancelling) {
            root.swCancelling = false;
            root.swPhase = "review";
            root.swStopped = true;
            return;
        }
        if (code === 0) {
            root.swPhase = "done";
            root.remember(root.swTarget.name);
            root.reloadProviders();
            return;
        }
        if (code === -1 || root.missingRun(code, err)) {
            root.swError = Changes.missingText("ryoku");
        } else {
            var lines = String(err || "").trim().split("\n").filter(l => l.trim().length > 0);
            root.swError = lines.length > 0 ? lines[lines.length - 1].replace(/^ryoku:\s*/, "") : I18n.tr("The switch stopped without saying why.");
        }
        root.swPhase = "failed";
    }

    function logOut() {
        root.hub.logOut();
    }

    // what this one cannot do: the provider's reasons, each once
    readonly property var cannotDo: {
        ProviderSchema.revision;
        var seen = {}, out = [];
        var u = ProviderSchema.unhonored || [];
        for (var i = 0; i < u.length; i++) {
            var r = u[i] ? u[i].reason : "";
            if (r && !seen[r]) {
                seen[r] = true;
                out.push(r);
            }
        }
        return out;
    }

    // plugins

    readonly property bool pluginsOn: Settings.supports("plugins")
    property var roster: ({ compositor: {}, toolchain: { ok: true, missing: [] }, plugins: [] })
    property bool rosterLoaded: false
    property string rosterError: ""
    // the action in flight: rebuild:<id>, rebuild:all, inspect, add, remove:<id>
    property string busy: ""
    property var lastAction: null
    property var log: []
    property string notice: ""
    property bool noticeError: false
    property bool busyWorking: false
    property bool busySlow: false
    property bool busyCancelled: false
    property string addUrl: ""
    property var inspected: null
    property var picked: ({})
    // the roster's and the add form's activity blocks, and what a repository offers, revealed
    // when they appear out of view
    property Item activityItem: null
    property Item addActivityItem: null
    property Item addResultsItem: null
    readonly property string activityGroup: root.lastAction === null ? ""
        : root.lastAction.kind === "inspect" || root.lastAction.kind === "add" ? "add" : "roster"

    readonly property var plugins: root.roster.plugins || []
    readonly property var extras: root.plugins.filter(p => !p.bundled)
    readonly property var stale: root.plugins.filter(p => p.status === "stale" && p.rebuildable)
    readonly property int runningCount: root.plugins.filter(p => p.loaded).length
    readonly property bool canBuild: root.roster.toolchain ? root.roster.toolchain.ok === true : false
    readonly property string extraBase: "wm." + Settings.provider + ".plugins.extra."

    property int rosterGen: 0
    function refreshRoster() {
        if (!root.pluginsOn)
            return;
        var gen = ++root.rosterGen;
        root.rosterError = "";
        Changes.run(["ryoku-hub", "desktop", "plugins", "list"], (ok, out, err, code, late) => {
            if (gen !== root.rosterGen)
                return;
            var r = null;
            try {
                r = JSON.parse(out);
            } catch (e) {}
            if (!ok || !r) {
                root.rosterError = root.readError(I18n.tr("The plugin list could not be read."), err, code, late);
                return;
            }
            root.roster = r;
            root.rosterLoaded = true;
        }, { timeout: 15000 });
    }

    function pluginStatus(p) {
        switch (p.status) {
        case "running": return I18n.tr("Running");
        case "ready": return p.enabled ? I18n.tr("Starts with the window manager") : I18n.tr("Installed and turned off");
        case "stale": return I18n.tr("Needs a rebuild for this version");
        case "failed": return I18n.tr("Did not load");
        case "unreachable": return I18n.tr("The window manager is not answering");
        case "missing": return I18n.tr("Not installed");
        }
        return "";
    }

    // where a bundled plugin's settings live: the page, section and its first switch
    function settingsPlace(p) {
        if (!p.bundled)
            return { page: "window-manager", row: "extra-" + p.id };
        var tab = Schema.bundled[p.id];
        if (!tab)
            return null;
        var where = ProviderMap.placeOf({ page: "plugins", tab: tab, group: "" });
        if (!where.place)
            return null;
        var rows = ProviderSchema.rows.filter(r => r.page === "plugins" && r.tab === tab && r.ctl === "sw");
        return { page: where.place[0], row: rows.length > 0 ? ProviderMap.normalize(rows[0], where.place).id : "" };
    }

    function pluginRow(p) {
        var status = root.pluginStatus(p);
        return {
            id: "plugin-" + p.id, section: "plugins", label: p.name,
            detail: [p.desc ? I18n.tr(p.desc) : "", status, p.detail || "",
                     p.source === "built" && p.installed ? (p.builtAt ? I18n.tr("Built on this computer on %1").replace("%1", String(p.builtAt).slice(0, 10)) : I18n.tr("Built on this computer")) : ""]
                .filter(s => s.length > 0).join(". "),
            keywords: [p.id], control: "custom", plugin: p,
            needs: { cap: "plugins" }
        };
    }

    function humanize(s) {
        var t = String(s).replace(/[_-]+/g, " ").trim();
        return t.charAt(0).toUpperCase() + t.slice(1);
    }

    // a plugin added here: its switch, then one control per setting it declares
    function extraRows(p) {
        var base = root.extraBase + p.id;
        var on = { store: "desktop", key: base + ".enabled", fallback: p.enabled === true };
        var out = [{
            id: "extra-" + p.id, section: "added", label: p.name,
            detail: p.desc ? p.desc : I18n.tr("Loads it into the window manager"),
            keywords: [p.id, "plugin"], control: "toggle", bind: on, "default": on.fallback,
            needs: { cap: "plugins" }
        }];
        var ss = p.settings || [];
        for (var j = 0; j < ss.length; j++) {
            var s = ss[j], parts = String(s.key).split(":");
            var d = s["default"];
            // a switch's default reads as on or off, never true or false
            var shownDefault = s.type === "bool" ? (d === true || d === "true" ? I18n.tr("on") : I18n.tr("off")) : String(d);
            var row = {
                id: "extra-" + p.id + "-" + String(s.key).replace(/[^A-Za-z0-9]+/g, "-").toLowerCase(),
                section: "added", label: root.humanize(parts[parts.length - 1]),
                detail: d !== undefined && d !== null ? I18n.tr("%1, %2 unless you change it").replace("%1", p.name).replace("%2", shownDefault) : p.name,
                keywords: [s.key, p.id], needs: { cap: "plugins" },
                when: { bind: on, is: [true] }
            };
            var n = Number(d);
            switch (s.type) {
            case "bool":
                row.control = "toggle";
                d = d === true || d === "true";
                break;
            case "int":
                row.control = "stepper";
                row.min = Math.min(0, isNaN(n) ? 0 : n);
                row.max = Math.max(100, isNaN(n) ? 0 : n * 4);
                d = isNaN(n) ? 0 : n;
                break;
            case "float":
                row.control = "slider";
                if (!isNaN(n) && n >= 0 && n <= 1) {
                    row.min = 0;
                    row.max = 1;
                    row.step = 0.01;
                } else {
                    row.min = 0;
                    row.max = Math.max(10, isNaN(n) ? 0 : n * 4);
                    row.step = row.max / 100;
                }
                d = isNaN(n) ? 0 : n;
                break;
            default:
                row.control = "text";
                row.mono = true;
                d = d === undefined || d === null ? "" : String(d);
            }
            row.bind = { store: "desktop", key: base + ".config." + s.key, fallback: d };
            row["default"] = d;
            out.push(row);
        }
        return out;
    }

    function act(kind, argv) {
        if (root.busy !== "")
            return;
        root.busy = kind;
        root.lastAction = { kind: kind, argv: argv };
        root.log = [];
        root.notice = "";
        root.noticeError = false;
        root.busyCancelled = false;
        actProc.command = argv;
        actProc.running = true;
        busyWorkingTimer.restart();
        busySlowTimer.restart();
        revealTimer.restart();
    }

    function revealActivity() {
        root.revealIfHidden(root.activityGroup === "add" ? root.addActivityItem : root.activityItem);
    }

    function revealIfHidden(a) {
        if (!a || !a.visible)
            return;
        var y = a.mapToItem(page.flickable, 0, 0).y;
        if (y < 0 || y + a.height > page.flickable.height)
            page.reveal(a);
    }

    function rebuild(id) {
        if (!root.canBuild)
            return;
        root.act(id === "all" ? "rebuild:all" : "rebuild:" + id,
                 id === "all" ? ["ryoku-hub", "desktop", "plugins", "rebuild", "--stale"] : ["ryoku-hub", "desktop", "plugins", "rebuild", id]);
    }

    function inspect() {
        var url = root.addUrl.trim();
        if (url === "" || !root.canBuild)
            return;
        root.inspected = null;
        root.picked = ({});
        root.act("inspect", ["ryoku-hub", "desktop", "plugins", "add", "--inspect", url]);
    }

    function addPicked() {
        var ids = Object.keys(root.picked).filter(k => root.picked[k]);
        if (!root.inspected || ids.length === 0)
            return;
        root.act("add", ["ryoku-hub", "desktop", "plugins", "add", root.inspected.repo].concat(ids));
    }

    function askRemove(p) {
        root.hub.confirm({
            title: I18n.tr("Remove %1?").replace("%1", p.name),
            text: I18n.tr("Its files are deleted from this computer. You can add it again from its repository."),
            actionText: I18n.tr("Remove"),
            destructive: true
        }, () => root.act("remove:" + p.id, ["ryoku-hub", "desktop", "plugins", "remove", p.id]));
    }

    function retry() {
        if (root.lastAction)
            root.act(root.lastAction.kind, root.lastAction.argv);
    }

    function cancelAction() {
        root.busyCancelled = true;
        actProc.running = false;
    }

    function busyText() {
        var k = root.busy;
        if (k === "rebuild:all")
            return I18n.tr("Rebuilding the plugins that need it");
        if (k.indexOf("rebuild:") === 0)
            return I18n.tr("Building %1").replace("%1", k.slice(8));
        if (k === "inspect")
            return I18n.tr("Reading the repository");
        if (k === "add")
            return I18n.tr("Building the plugins you picked");
        if (k.indexOf("remove:") === 0)
            return I18n.tr("Removing %1").replace("%1", k.slice(7));
        return "";
    }

    // the backend's verdict, from its JSON on stdout
    function finish(code, out) {
        var kind = root.busy;
        root.busy = "";
        busyWorkingTimer.stop();
        busySlowTimer.stop();
        root.busyWorking = false;
        root.busySlow = false;
        if (root.busyCancelled) {
            root.busyCancelled = false;
            root.notice = I18n.tr("Stopped. Nothing was added or changed.");
            root.noticeError = false;
            root.refreshRoster();
            return;
        }
        var r = null;
        try {
            r = JSON.parse(out);
        } catch (e) {}
        if (code !== 0 || !r) {
            var last = root.log.length ? root.log[root.log.length - 1] : "";
            var next = kind.indexOf("remove:") === 0 ? I18n.tr("Retry, and if it fails again, update Ryoku.")
                : I18n.tr("Retry, and if it fails again, check the plugin's repository for what it needs.");
            if (code === -1 || root.missingRun(code, last))
                root.notice = Changes.missingText(String(root.lastAction.argv[0]));
            else
                root.notice = (last ? I18n.tr("That did not work: %1").replace("%1", last) : I18n.tr("That did not work, and the window manager gave no reason.")) + " " + next;
            root.noticeError = true;
            root.refreshRoster();
            Qt.callLater(root.revealActivity);
            return;
        }
        root.log = [];
        if (kind === "inspect") {
            root.inspected = r;
            var pk = {};
            for (var i = 0; i < r.plugins.length; i++)
                if (!r.plugins[i].bundled && (r.plugins.length === 1 || !r.plugins[i].installed))
                    pk[r.plugins[i].id] = r.plugins.length === 1;
            root.picked = pk;
            root.notice = r.plugins.length === 0 ? I18n.tr("That repository offers no plugin.") : "";
            root.noticeError = r.plugins.length === 0;
            // what it offers shows under the address, maybe below the fold; once laid out
            resultsReveal.restart();
            return;
        }
        if (kind.indexOf("remove:") === 0) {
            // the backend dropped its key on disk; the live store follows quietly
            if (r.storeKey)
                Changes.desktopEdit({ store: "desktop", key: r.storeKey }, undefined, "", "window-manager");
            root.notice = I18n.tr("Removed %1.").replace("%1", r.removed);
            root.refreshRoster();
            return;
        }
        var built = r.built || [], failed = Object.keys(r.failed || {});
        var parts = [];
        if (built.length)
            parts.push(I18n.tr("Built %1.").replace("%1", built.join(", ")));
        if (r.skipped && r.skipped.length)
            parts.push(I18n.tr("%1 already up to date.").replace("%1", r.skipped.length));
        for (var f = 0; f < failed.length; f++)
            parts.push(I18n.tr("%1 did not build: %2").replace("%1", failed[f]).replace("%2", r.failed[failed[f]]));
        root.notice = parts.join(" ");
        root.noticeError = failed.length > 0;
        if (kind === "add") {
            // a plugin just added is one you want running; the build is the change, so no notice of its own
            for (var b = 0; b < built.length; b++)
                Changes.desktopEdit({ store: "desktop", key: root.extraBase + built[b] + ".enabled" }, true, built[b], "window-manager");
            if (built.length) {
                root.inspected = null;
                root.addUrl = "";
            }
        }
        root.refreshRoster();
    }

    // the page's schema: search rows plus the live ones
    readonly property var schema: ({
        title: Schema.title,
        lead: Schema.lead,
        sections: Schema.sections,
        related: Schema.related,
        rows: Schema.rows
            .concat(root.providers.map(p => root.providerRow(p)))
            .concat(root.pluginsOn ? root.plugins.map(p => root.pluginRow(p)) : [])
            .concat(root.pluginsOn ? root.extras.reduce((all, p) => all.concat(root.extraRows(p)), []) : [])
    })

    readonly property string stateLine: root.pendingName !== ""
        ? I18n.tr("Switching to %1 at your next sign-in.").replace("%1", root.title(root.pendingName))
        : root.activeName !== "" ? I18n.tr("%1 is running.").replace("%1", root.title(root.activeName)) : ""

    function focusRow(rowId) {
        page.focusRow(rowId);
    }

    Component.onCompleted: {
        root.reloadProviders();
        root.refreshRoster();
    }
    onPluginsOnChanged: root.refreshRoster()

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "window-manager"
        schema: root.schema
        stateLine: root.stateLine
        bodies: root.pluginsOn ? { cannot: cannotBody, plugins: pluginsHead } : { cannot: cannotBody }
        footers: ({ current: currentFoot })
        moreBodies: root.pluginsOn ? { plugins: addBody } : {}
        controls: {
            var c = {};
            root.providers.forEach(p => c["wm-" + p.name] = providerControl);
            root.plugins.forEach(p => c["plugin-" + p.id] = pluginControl);
            return c;
        }
    }

    // processes

    Process {
        id: pendingProc
        onExited: (code, status) => Attention.probe("wm-switch")
    }

    // the switch runs on the page, so leaving the page stops it; a program that never
    // started reports no exit, only that it stopped running
    Process {
        id: useProc
        property bool didStart: false
        stdout: StdioCollector { }
        stderr: StdioCollector { id: useErr }
        onStarted: useProc.didStart = true
        onExited: (code, status) => root.switchFinished(code, useErr.text)
        onRunningChanged: {
            if (useProc.running)
                return;
            if (!useProc.didStart && root.swPhase === "running")
                root.switchFinished(-1, "");
            useProc.didStart = false;
        }
    }

    Timer {
        id: swWorkingTimer
        interval: 1000
        onTriggered: root.swWorking = true
    }
    Timer {
        id: swSlowTimer
        interval: 10000
        onTriggered: root.swSlow = true
    }

    Process {
        id: actProc
        property bool didStart: false
        stdout: StdioCollector { id: actOut }
        stderr: SplitParser {
            onRead: line => {
                if (line.trim() !== "")
                    root.log = root.log.concat([line]).slice(-40);
            }
        }
        onStarted: actProc.didStart = true
        onExited: (code, status) => root.finish(code, actOut.text)
        onRunningChanged: {
            if (actProc.running)
                return;
            if (!actProc.didStart && root.busy !== "")
                root.finish(-1, "");
            actProc.didStart = false;
        }
    }

    Timer {
        id: busyWorkingTimer
        interval: 1000
        onTriggered: root.busyWorking = true
    }
    Timer {
        id: busySlowTimer
        interval: 10000
        onTriggered: root.busySlow = true
    }
    Timer {
        id: revealTimer
        interval: 1100
        onTriggered: root.revealActivity()
    }
    Timer {
        id: resultsReveal
        interval: 120
        onTriggered: root.revealIfHidden(root.inspected && root.inspected.plugins.length > 0 ? root.addResultsItem : root.addActivityItem)
    }

    // section content

    Component {
        id: providerControl
        Row {
            property var row: null
            readonly property var p: row ? row.provider : null
            spacing: LithicTokens.space.s3

            function activate() {
                if (row && row.switchable)
                    root.openSwitch(p);
            }

            Plaque {
                visible: !!parent.p && parent.p.active === true && root.pendingName !== ""
                anchors.verticalCenter: parent.verticalCenter
                kind: "quiet"
                small: true
                text: I18n.tr("Stay on %1").replace("%1", root.title(root.activeName))
                onClicked: root.stay()
            }
            Claim {
                visible: !!parent.p && parent.p.active === true
                anchors.verticalCenter: parent.verticalCenter
                tone: "alive"
                text: I18n.tr("In use")
            }
            Claim {
                visible: !!parent.p && parent.p.name === root.pendingName
                anchors.verticalCenter: parent.verticalCenter
                tone: "becoming"
                text: I18n.tr("Next sign-in")
            }
            Plaque {
                visible: !!parent.row && parent.row.switchable === true
                anchors.verticalCenter: parent.verticalCenter
                kind: "line"
                small: true
                text: I18n.tr("Switch")
                accessibleName: I18n.tr("Switch to %1").replace("%1", parent.row ? parent.row.label : "")
                onClicked: parent.activate()
            }
        }
    }

    Component {
        id: currentFoot
        Column {
            spacing: LithicTokens.space.s2

            Row {
                visible: !root.listed
                spacing: LithicTokens.space.s2
                Working {
                    anchors.verticalCenter: parent.verticalCenter
                    active: !root.listed && listWait.elapsed
                    visible: active
                }
                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("Looking for window managers")
                    typeRole: "detail"
                    inkRole: "fg2"
                }
                Timer {
                    id: listWait
                    property bool elapsed: false
                    interval: 1000
                    running: !root.listed
                    onRunningChanged: if (running) elapsed = false
                    onTriggered: elapsed = true
                }
            }
            Row {
                visible: root.listError !== ""
                width: parent.width
                spacing: LithicTokens.space.s3
                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    width: Math.min(implicitWidth, parent.width - listRetry.width - LithicTokens.space.s3)
                    text: root.listError
                    typeRole: "detail"
                    ink: Lithic.jasper
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }
                Plaque {
                    id: listRetry
                    kind: "quiet"
                    small: true
                    glyph: "refresh"
                    text: I18n.tr("Retry")
                    onClicked: root.reloadProviders()
                }
            }
            Row {
                visible: root.terminalTarget !== ""
                width: parent.width
                spacing: LithicTokens.space.s3
                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    width: Math.min(implicitWidth, parent.width - checkAgain.width - LithicTokens.space.s3)
                    text: I18n.tr("Finish the switch in the terminal window. It asks for your password to install %1.").replace("%1", root.title(root.terminalTarget))
                    typeRole: "detail"
                    inkRole: "fg"
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }
                Plaque {
                    id: checkAgain
                    kind: "quiet"
                    small: true
                    glyph: "refresh"
                    text: I18n.tr("Check again")
                    onClicked: {
                        root.reloadProviders();
                        Attention.probe("wm-switch");
                    }
                }
            }
            Label {
                visible: root.listed && root.listError === ""
                width: parent.width
                text: root.hasAlternative
                    ? I18n.tr("A switch finishes at your next sign-in, and installs the window manager first if this computer does not have it yet. Your settings are kept in one place, so switching loses none of them.")
                    : I18n.tr("This computer runs one window manager. Others appear here when they become available.")
                typeRole: "detail"
                inkRole: "fg3"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
        }
    }

    Component {
        id: cannotBody
        Column {
            spacing: LithicTokens.space.s2

            Label {
                width: parent.width
                text: !ProviderSchema.ready ? I18n.tr("Checking your settings against %1.").replace("%1", root.title(root.activeName))
                    : root.cannotDo.length === 0 ? I18n.tr("Nothing. %1 can do everything your settings ask for.").replace("%1", root.title(root.activeName))
                    : I18n.tr("These settings have no equivalent in %1. They stay saved and work again if you switch back.").replace("%1", root.title(root.activeName))
                typeRole: "detail"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }

            Slab {
                visible: root.cannotDo.length > 0
                width: parent.width

                Column {
                    width: parent.width
                    spacing: 0

                    Repeater {
                        model: root.cannotDo.slice(0, 4)
                        delegate: ReasonLine {}
                    }
                    Disclosure {
                        visible: root.cannotDo.length > 4
                        width: parent.width
                        text: I18n.tr("Show all %1").replace("%1", root.cannotDo.length)

                        Repeater {
                            model: root.cannotDo.slice(4)
                            delegate: ReasonLine {}
                        }
                    }
                }
            }
        }
    }

    component ReasonLine: Label {
        required property var modelData
        width: parent ? parent.width : 0
        topPadding: LithicTokens.space.s1
        bottomPadding: LithicTokens.space.s1
        text: I18n.tr(modelData)
        typeRole: "body"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }

    // what a switch keeps, what the new one cannot do, and the outgoing one's packages
    component SwitchReport: Column {
        spacing: LithicTokens.space.s4

        Column {
            width: parent.width
            spacing: LithicTokens.space.s1
            Label {
                text: I18n.tr("Carries over")
                typeRole: "row"
            }
            Label {
                width: parent.width
                text: I18n.tr("Your settings carry over, and %1 uses every one it can.").replace("%1", root.swName)
                typeRole: "detail"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Label {
                visible: root.swReport !== null && root.swReport.keybindCount > 0
                width: parent.width
                text: I18n.tr("Your %1 keyboard shortcuts carry over unchanged.").replace("%1", root.swReport ? root.swReport.keybindCount : 0)
                typeRole: "detail"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Label {
                visible: root.swLeaving
                width: parent.width
                text: I18n.tr("Settings only %1 has stay saved and come back if you switch back.").replace("%1", root.swActiveName)
                typeRole: "detail"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
        }

        Column {
            width: parent.width
            spacing: LithicTokens.space.s1
            Label {
                width: parent.width
                text: I18n.tr("What %1 cannot do").replace("%1", root.swName)
                typeRole: "row"
            }
            Label {
                width: parent.width
                text: root.swReport && root.swReport.exact !== true
                    ? I18n.tr("The exact list shows once %1 is installed.").replace("%1", root.swName)
                    : root.swReasons.length === 0 ? I18n.tr("Nothing. %1 can do everything your settings ask for.").replace("%1", root.swName)
                    : I18n.tr("These settings have no equivalent there, and stay saved for when you come back.")
                typeRole: "detail"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Repeater {
                model: root.swReport && root.swReport.exact === true ? root.swReasons.slice(0, 3) : []
                delegate: ReasonLine {}
            }
            Disclosure {
                visible: root.swReport !== null && root.swReport.exact === true && root.swReasons.length > 3
                width: parent.width
                text: I18n.tr("Show all %1").replace("%1", root.swReasons.length)
                Repeater {
                    model: root.swReport && root.swReport.exact === true ? root.swReasons.slice(3) : []
                    delegate: ReasonLine {}
                }
            }
        }

        Column {
            visible: root.swLeaving
            width: parent.width
            spacing: LithicTokens.space.s2
            Label {
                width: parent.width
                text: I18n.tr("Leaving %1").replace("%1", root.swActiveName)
                typeRole: "row"
            }
            Segmented {
                visible: root.swRemovable
                model: [I18n.tr("Keep it"), I18n.tr("Remove it")]
                currentIndex: root.swKeep === "remove" ? 1 : 0
                enabled: root.swPhase !== "running"
                accessibleName: I18n.tr("Leaving %1").replace("%1", root.swActiveName)
                onActivated: index => root.swKeep = index === 1 ? "remove" : "keep"
            }
            Label {
                width: parent.width
                text: !root.swRemovable
                    ? I18n.tr("Nothing of %1 can be removed here, so both stay available and switching back costs nothing.").replace("%1", root.swActiveName)
                    : root.swKeep === "remove"
                    ? I18n.tr("Removes %1 packages (%2). Switching back later installs them again.")
                        .replace("%1", root.swReclaim ? root.swReclaim.count : 0).replace("%2", root.humanSize(root.swReclaim ? root.swReclaim.size : 0))
                    : I18n.tr("Switch back any time with no download. Its %1 packages (%2) stay on this computer.")
                        .replace("%1", root.swReclaim ? root.swReclaim.count : 0).replace("%2", root.humanSize(root.swReclaim ? root.swReclaim.size : 0))
                typeRole: "detail"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
        }

        Label {
            visible: root.swReport !== null && root.swReport.deployed !== true && root.swPhase === "review"
            width: parent.width
            text: I18n.tr("%1 is installed first. A terminal window opens for your password and shows the progress.").replace("%1", root.swName)
            typeRole: "detail"
            inkRole: "fg2"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Label {
            visible: root.swReport !== null && !root.swSelectable
            width: parent.width
            text: I18n.tr("%1 cannot be installed on this computer, so this switch cannot start here.").replace("%1", root.swName)
            typeRole: "detail"
            ink: Lithic.jasper
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
    }

    Component {
        id: pluginsHead
        Column {
            spacing: LithicTokens.space.s2

            Row {
                width: parent.width
                spacing: LithicTokens.space.s3

                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    width: parent.width - headActions.width - parent.spacing
                    text: !root.rosterLoaded ? (root.rosterError !== "" ? "" : I18n.tr("Reading the plugins"))
                        : I18n.tr("Plugins are built for %1 %2. %3 running.")
                            .replace("%1", root.title(Settings.provider))
                            .replace("%2", root.roster.compositor.version || "")
                            .replace("%3", root.runningCount)
                    typeRole: "detail"
                    inkRole: "fg2"
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }
                Row {
                    id: headActions
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: LithicTokens.space.s2
                    Plaque {
                        visible: root.stale.length > 0
                        kind: "gold"
                        small: true
                        glyph: "build"
                        text: root.stale.length === 1 ? I18n.tr("Rebuild the one that needs it") : I18n.tr("Rebuild %1 that need it").replace("%1", root.stale.length)
                        enabled: root.busy === "" && root.canBuild
                        busy: root.busy === "rebuild:all"
                        onClicked: root.rebuild("all")
                    }
                    IconPlaque {
                        kind: "quiet"
                        small: true
                        glyph: "refresh"
                        accessibleName: I18n.tr("Check the plugins again")
                        enabled: root.busy === ""
                        onClicked: root.refreshRoster()
                    }
                }
            }
            Label {
                visible: root.rosterLoaded && !root.canBuild
                width: parent.width
                text: I18n.tr("Plugins cannot be built on this computer. Missing build tools: %1.")
                    .replace("%1", ((root.roster.toolchain && root.roster.toolchain.missing) || []).join(", "))
                typeRole: "detail"
                ink: Lithic.amber
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Row {
                visible: root.rosterError !== ""
                width: parent.width
                spacing: LithicTokens.space.s3
                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    width: Math.min(implicitWidth, parent.width - rosterRetry.width - LithicTokens.space.s3)
                    text: root.rosterError
                    typeRole: "detail"
                    ink: Lithic.jasper
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }
                Plaque {
                    id: rosterRetry
                    kind: "quiet"
                    small: true
                    glyph: "refresh"
                    text: I18n.tr("Retry")
                    onClicked: root.refreshRoster()
                }
            }

            Activity {
                width: parent.width
                group: "roster"
            }
        }
    }

    Component {
        id: pluginControl
        Row {
            id: pc
            property var row: null
            readonly property var p: row ? row.plugin : null
            readonly property var place: p ? root.settingsPlace(p) : null
            spacing: LithicTokens.space.s1

            Plaque {
                visible: !!pc.p && pc.p.rebuildable === true
                anchors.verticalCenter: parent.verticalCenter
                kind: pc.p && (pc.p.status === "stale" || pc.p.status === "missing") ? "gold" : "line"
                small: true
                text: pc.p && pc.p.status === "missing" ? I18n.tr("Build here") : I18n.tr("Rebuild")
                busy: !!pc.p && (root.busy === "rebuild:" + pc.p.id || (root.busy === "rebuild:all" && pc.p.status === "stale"))
                enabled: root.busy === "" && root.canBuild
                onClicked: root.rebuild(pc.p.id)
            }
            IconPlaque {
                visible: pc.place !== null
                anchors.verticalCenter: parent.verticalCenter
                kind: "quiet"
                small: true
                glyph: "tune"
                accessibleName: I18n.tr("Settings for %1").replace("%1", pc.p ? pc.p.name : "")
                onClicked: {
                    if (pc.place.page === "window-manager")
                        page.focusRow(pc.place.row);
                    else
                        root.hub.navigate(pc.place.page, pc.place.row);
                }
            }
            IconPlaque {
                visible: !!pc.p && !!pc.p.docs
                anchors.verticalCenter: parent.verticalCenter
                kind: "quiet"
                small: true
                glyph: "menu_book"
                accessibleName: I18n.tr("Read about %1").replace("%1", pc.p ? pc.p.name : "")
                onClicked: Spawn.runApp(["xdg-open", pc.p.docs], { label: "browser" })
            }
            IconPlaque {
                visible: !!pc.p && pc.p.removable === true && pc.p.source === "built"
                anchors.verticalCenter: parent.verticalCenter
                kind: "quiet"
                small: true
                glyph: "delete"
                accessibleName: I18n.tr("Remove %1").replace("%1", pc.p ? pc.p.name : "")
                enabled: root.busy === ""
                onClicked: root.askRemove(pc.p)
            }
        }
    }

    // an action's working mark, progress and log, then what came of it; shown where it started
    component Activity: Column {
        id: activity
        property string group: ""
        visible: root.activityGroup === activity.group && (root.busy !== "" ? root.busyWorking : root.notice !== "")
        spacing: LithicTokens.space.s2
        Component.onCompleted: {
            if (activity.group === "roster")
                root.activityItem = activity;
            else if (activity.group === "add")
                root.addActivityItem = activity;
        }

        Row {
            visible: root.busy !== "" && root.busyWorking
            width: parent.width
            spacing: LithicTokens.space.s3
            Working {
                anchors.verticalCenter: parent.verticalCenter
                active: parent.visible
            }
            Label {
                anchors.verticalCenter: parent.verticalCenter
                width: parent.width - LithicTokens.space.s6 - cancelAct.width
                text: root.busyText()
                typeRole: "detail"
                elide: Text.ElideRight
            }
            Plaque {
                id: cancelAct
                visible: root.busySlow
                anchors.verticalCenter: parent.verticalCenter
                kind: "quiet"
                small: true
                text: I18n.tr("Cancel")
                onClicked: root.cancelAction()
            }
        }
        BladeProgress {
            visible: root.busy !== "" && root.busySlow
            width: parent.width
            active: visible
            indeterminate: true
        }
        Label {
            visible: root.busy !== "" && root.busyWorking && root.log.length > 0
            width: parent.width
            text: root.log.slice(-6).join("\n")
            typeRole: "monoSm"
            inkRole: "fg3"
            wrapMode: Text.WrapAnywhere
            elide: Text.ElideNone
        }
        Row {
            visible: root.busy === "" && root.notice !== ""
            width: parent.width
            spacing: LithicTokens.space.s3
            Label {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.min(implicitWidth, parent.width - (actRetry.visible ? actRetry.width + LithicTokens.space.s3 : 0))
                text: root.notice
                typeRole: "detail"
                ink: root.noticeError ? Lithic.jasper : Lithic.fg2
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Plaque {
                id: actRetry
                visible: root.noticeError && root.lastAction !== null
                kind: "quiet"
                small: true
                glyph: "refresh"
                text: I18n.tr("Retry")
                onClicked: root.retry()
            }
        }
    }

    // add from a repository, under More options
    Component {
        id: addBody
        Column {
            spacing: LithicTokens.space.s3
            topPadding: LithicTokens.space.s2

            Label {
                width: parent.width
                text: I18n.tr("Add a plugin from its source repository. It is downloaded and built on this computer for the window manager you run, then loaded into it, so add only repositories you trust.")
                typeRole: "detail"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Row {
                width: parent.width
                spacing: LithicTokens.space.s2

                TextField {
                    id: urlField
                    width: parent.width - lookUp.width - parent.spacing
                    label: I18n.tr("Repository address")
                    placeholderText: "https://github.com/owner/plugins"
                    mono: true
                    text: root.addUrl
                    enabled: root.canBuild
                    onTextEdited: root.addUrl = text
                    onAccepted: root.inspect()
                }
                Plaque {
                    id: lookUp
                    anchors.bottom: urlField.bottom
                    kind: "line"
                    text: I18n.tr("Look up")
                    busy: root.busy === "inspect"
                    enabled: root.busy === "" && root.canBuild && root.addUrl.trim() !== ""
                    onClicked: root.inspect()
                }
            }
            Column {
                id: addResults
                visible: root.inspected !== null && root.inspected.plugins.length > 0
                width: parent.width
                spacing: LithicTokens.space.s3
                Component.onCompleted: root.addResultsItem = addResults

                Slab {
                    width: parent.width

                    Column {
                        width: parent.width

                        Repeater {
                            model: root.inspected ? root.inspected.plugins : []
                            delegate: SettingRow {
                                required property var modelData
                                required property int index
                                width: parent.width
                                first: index === 0
                                title: modelData.id
                                detail: modelData.bundled ? I18n.tr("Comes with Ryoku already")
                                    : modelData.installed ? I18n.tr("Already added") : (modelData.desc || "")
                                activatable: !modelData.bundled
                                onActivated: offerToggle.toggle()

                                Toggle {
                                    id: offerToggle
                                    enabled: !modelData.bundled
                                    checked: root.picked[modelData.id] === true
                                    Accessible.name: modelData.id
                                    function toggle() {
                                        var p = Object.assign({}, root.picked);
                                        p[modelData.id] = !(root.picked[modelData.id] === true);
                                        root.picked = p;
                                    }
                                    onToggled: {
                                        var p = Object.assign({}, root.picked);
                                        p[modelData.id] = checked;
                                        root.picked = p;
                                    }
                                }
                            }
                        }
                    }
                }
                Plaque {
                    kind: "gold"
                    glyph: "add"
                    text: I18n.tr("Build and add")
                    busy: root.busy === "add"
                    enabled: root.busy === "" && Object.keys(root.picked).some(k => root.picked[k])
                    onClicked: root.addPicked()
                }
            }

            Activity {
                width: parent.width
                group: "add"
            }
        }
    }

    Dialog {
        id: switchDialog
        parent: root.hub.overlayItem
        dialogWidth: 560
        title: root.swPhase === "done" ? I18n.tr("%1 is ready").replace("%1", root.swName)
            : I18n.tr("Switch to %1").replace("%1", root.swName)
        closePolicy: root.swPhase === "running" ? T.Popup.NoAutoClose : (T.Popup.CloseOnEscape | T.Popup.CloseOnPressOutside)
        actions: root.swPhase === "done" ? [
                { text: I18n.tr("Later"), kind: "quiet", closeOnTrigger: true },
                { text: I18n.tr("Log out now"), kind: "gold", glyph: "logout", closeOnTrigger: false }
            ] : root.swPhase === "running" ? (root.swSlow ? [
                { text: I18n.tr("Cancel"), kind: "quiet", closeOnTrigger: false },
                { text: I18n.tr("Switching"), kind: "gold", busy: true, enabled: false, closeOnTrigger: false }
            ] : [
                { text: I18n.tr("Switching"), kind: "gold", busy: true, enabled: false, closeOnTrigger: false }
            ]) : root.swPhase === "failed" ? [
                { text: I18n.tr("Close"), kind: "quiet", closeOnTrigger: true },
                { text: I18n.tr("Try again"), kind: "gold", glyph: "refresh", closeOnTrigger: false }
            ] : [
                { text: I18n.tr("Cancel"), kind: "quiet", closeOnTrigger: true },
                { text: I18n.tr("Switch to %1").replace("%1", root.swName), kind: "gold", enabled: !root.swLoading && root.swSelectable, closeOnTrigger: false }
            ]
        onActionTriggered: index => {
            if (root.swPhase === "done") {
                if (index === 1)
                    root.logOut();
            } else if (root.swPhase === "running") {
                if (root.swSlow && index === 0)
                    root.cancelSwitch();
            } else if (index === 1) {
                if (root.swReport === null)
                    root.openSwitch(root.swTarget);
                else
                    root.startSwitch();
            }
        }

        Column {
            width: parent.width
            spacing: LithicTokens.space.s4

            Row {
                visible: root.swLoading
                spacing: LithicTokens.space.s2
                Working {
                    anchors.verticalCenter: parent.verticalCenter
                    active: root.swLoading
                }
                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("Working out what carries over")
                    typeRole: "body"
                    inkRole: "fg2"
                }
            }

            // after the switch: the one next step
            Label {
                visible: root.swPhase === "done"
                width: parent.width
                text: I18n.tr("Log out, then pick %1 on the sign-in screen. Your session stays as it is until you do, and %2 stays installed so you can come back the same way.")
                    .replace("%1", root.swName).replace("%2", root.swActiveName)
                typeRole: "body"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }

            Item {
                visible: !root.swLoading && root.swReport !== null && root.swPhase !== "done"
                width: parent.width
                implicitHeight: Math.min(report.implicitHeight, Math.max(120, root.hub.overlayItem.height - 340))

                Flickable {
                    id: reportFlick
                    anchors.fill: parent
                    contentWidth: width
                    contentHeight: report.implicitHeight
                    boundsBehavior: Flickable.StopAtBounds
                    clip: true

                    SwitchReport {
                        id: report
                        width: reportFlick.width - LithicTokens.space.s4
                    }
                }
                ScrollRail {
                    flickable: reportFlick
                    anchors.top: parent.top
                    anchors.bottom: parent.bottom
                    anchors.right: parent.right
                }
            }
            Label {
                visible: root.swStopped && root.swPhase === "review"
                width: parent.width
                text: I18n.tr("Stopped before it finished. Your session is unchanged; switch again to finish.")
                typeRole: "detail"
                ink: Lithic.amber
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }

            Row {
                visible: root.swPhase === "running" && root.swWorking
                width: parent.width
                spacing: LithicTokens.space.s2
                Working {
                    anchors.verticalCenter: parent.verticalCenter
                    active: parent.visible
                }
                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("Getting %1 ready for your next sign-in").replace("%1", root.swName)
                    typeRole: "detail"
                }
            }
            BladeProgress {
                visible: root.swPhase === "running" && root.swSlow
                width: parent.width
                active: visible
                indeterminate: true
            }
            Label {
                visible: root.swPhase === "failed"
                width: parent.width
                text: root.swReport === null ? root.swError : I18n.tr("The switch did not finish: %1").replace("%1", root.swError)
                typeRole: "detail"
                ink: Lithic.jasper
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
        }
    }
}
