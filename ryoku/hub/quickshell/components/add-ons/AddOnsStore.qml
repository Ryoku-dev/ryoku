import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../../Singletons"

// add-ons, bundles and Omarchy add-ons; keys "p/<id>/on|host|edge|align|pos|set/<key>" and "o/<id>"
Scope {
    id: store

    property var plugins: []
    property int widgetCount: 0
    property bool pluginsLoaded: false
    property var catalog: []
    property bool catalogKnown: false
    readonly property bool catalogChecking: catProc.running
    property var bundleCatalog: []
    property var bundles: []
    property bool bundlesLoaded: false
    property var statusLoaded: ({})
    property var omarchy: []
    property bool omarchyLoaded: false
    property bool omarchyAvailable: false
    property var readErrors: ({})

    property var queue: []
    property var active: null
    property bool rereadPlugins: false
    property bool rereadOmarchy: false

    readonly property string shellDir: Quickshell.env("RYOKU_SHELL_DIR") || ""
    readonly property string discoverScript: store.shellDir.length > 0
        ? store.shellDir + "/quickshell/plugins/discover.sh"
        : (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/quickshell/plugins/discover.sh"
    readonly property string placeTool: store.shellDir.length > 0
        ? store.shellDir + "/quickshell/plugins/ryoku-plugins-place"
        : "ryoku-plugins-place"

    readonly property alias pluginTask: pluginTask
    readonly property alias bundleTask: bundleTask
    readonly property alias omarchyTask: omarchyTask

    // where an add-on can show; desktop widgets belong to Desktop & dock
    readonly property var hostOrder: ["framePopout", "topbarGlyph", "sidebarCard"]

    function hostsOf(plugin) {
        var m = (plugin || {}).manifest || {};
        var raw = m.hosts !== undefined ? m.hosts : m.host;
        return Array.isArray(raw) ? raw : (raw ? [raw] : []);
    }

    function isWidget(plugin) {
        return store.hostsOf(plugin).indexOf("desktopWidget") >= 0;
    }

    function plugin(id) {
        for (var i = 0; i < store.plugins.length; i++)
            if (store.plugins[i].id === id)
                return store.plugins[i];
        return null;
    }

    function bundle(id) {
        for (var i = 0; i < store.bundles.length; i++)
            if (store.bundles[i].id === id)
                return store.bundles[i];
        for (var j = 0; j < store.bundleCatalog.length; j++)
            if (store.bundleCatalog[j].id === id)
                return store.bundleCatalog[j];
        return null;
    }

    function omarchyItem(id) {
        for (var i = 0; i < store.omarchy.length; i++)
            if (store.omarchy[i].id === id)
                return store.omarchy[i];
        return null;
    }

    function nameOf(plugin) {
        var m = (plugin || {}).manifest || {};
        return String(m.name || (plugin || {}).id || "");
    }

    function hostOf(plugin) {
        var place = (plugin || {}).placement || {};
        var m = (plugin || {}).manifest || {};
        return String(place.host || (m.defaults && m.defaults.host) || "framePopout");
    }

    function popoutOf(plugin) {
        var fp = ((plugin || {}).placement || {}).framePopout || {};
        return {
            edge: ["center", "top", "right", "bottom", "left"].indexOf(fp.edge) >= 0 ? fp.edge : "top",
            align: ["start", "center", "end"].indexOf(fp.align) >= 0 ? fp.align : "start",
            hoverW: fp.hoverW !== undefined ? fp.hoverW : 320,
            hoverH: fp.hoverH !== undefined ? fp.hoverH : 16
        };
    }

    function cmpVersion(a, b) {
        var pa = String(a || "0").split(".").map(n => parseInt(n, 10) || 0);
        var pb = String(b || "0").split(".").map(n => parseInt(n, 10) || 0);
        for (var i = 0; i < Math.max(pa.length, pb.length); i++) {
            var x = pa[i] || 0, y = pb[i] || 0;
            if (x !== y)
                return x < y ? -1 : 1;
        }
        return 0;
    }

    function installedVersion(plugin) {
        return String((plugin || {}).version || ((plugin || {}).manifest || {}).version || "");
    }

    // the newer version the catalogue offers, or ""
    function updateFor(plugin) {
        var have = store.installedVersion(plugin);
        var entry = store.catalog.find(c => c.id === (plugin || {}).id);
        var offered = entry ? String(entry.version || "") : "";
        if (have === "" || offered === "")
            return "";
        return store.cmpVersion(offered, have) > 0 ? offered : "";
    }

    function readError(source) {
        return store.readErrors[source] || "";
    }

    function setReadError(source, message) {
        var next = Object.assign({}, store.readErrors);
        if (message)
            next[source] = message;
        else
            delete next[source];
        store.readErrors = next;
    }

    function lastLines(text) {
        var lines = String(text || "").trim().split("\n").filter(l => l.trim().length > 0);
        return lines.slice(Math.max(0, lines.length - 2)).join(" ");
    }

    function readFailed(source, text, plain, code) {
        var said = store.lastLines(text);
        store.setReadError(source, said.length > 0
            ? plain.replace(/ \(exit %1\)/, "").replace("%1", code) + " " + I18n.tr("It said: %1").arg(said) + (/[.!?]$/.test(said) ? "" : ".")
            : plain.replace("%1", code));
    }

    function refresh() {
        store.loadPlugins();
        store.loadCatalog();
        store.loadBundles();
        store.loadOmarchy();
    }

    function loadPlugins() {
        if (listProc.running)
            store.rereadPlugins = true;
        else
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
        if (omarchyProc.running)
            store.rereadOmarchy = true;
        else
            omarchyProc.running = true;
    }

    property string statusTarget: ""
    property var statusWaiting: []

    function loadStatus(id) {
        var target = id || "";
        if (target !== "") {
            var loaded = Object.assign({}, store.statusLoaded);
            loaded[target] = false;
            store.statusLoaded = loaded;
        }
        if (statusProc.running) {
            if (store.statusWaiting.indexOf(target) < 0)
                store.statusWaiting = store.statusWaiting.concat([target]);
            return;
        }
        store.statusTarget = target;
        statusProc.command = target !== "" ? ["ryostore-install", "status", "bundle", target] : ["ryostore-install", "status"];
        statusProc.running = true;
    }

    function statusKnown(id) {
        return store.statusLoaded[id] === true;
    }

    // one status row joined onto its catalogue bundle; totalCount counts the parts this system can have
    function withStatus(base, status) {
        var out = JSON.parse(JSON.stringify(base || {}));
        var meta = out.metadata || {};
        var parts = meta.items || [];
        var states = {};
        for (var i = 0; i < status.items.length; i++)
            states[String(status.items[i].name)] = status.items[i];
        var installed = 0, available = 0;
        for (var j = 0; j < parts.length; j++) {
            var s = states[parts[j].name];
            if (s !== undefined) {
                parts[j].installed = s.status === "present";
                parts[j].available = s.status !== "unavailable";
                parts[j].unavailableReason = parts[j].available ? "" : String(s.reason || "");
            }
            if (parts[j].installed === true)
                installed++;
            if (parts[j].available !== false)
                available++;
        }
        meta.items = parts;
        out.metadata = meta;
        out.installedCount = installed;
        out.totalCount = available;
        return out;
    }

    function applyBundleCatalog(items) {
        var current = {};
        store.bundles.forEach(b => current[b.id] = b);
        store.bundleCatalog = items.filter(i => i.category === "bundles");
        store.bundles = store.bundleCatalog
            .map(b => current[b.id] !== undefined ? current[b.id] : b)
            .filter(b => Number(b.installedCount || 0) > 0);
    }

    function applyStatuses(rows) {
        var byId = {};
        var loaded = Object.assign({}, store.statusLoaded);
        rows.forEach(r => { byId[r.id] = r; loaded[r.id] = true; });
        var current = {};
        store.bundles.forEach(b => current[b.id] = b);
        // a bundle opened from the list keeps its place while it is looked at
        store.bundles = store.bundleCatalog
            .map(b => byId[b.id] !== undefined ? store.withStatus(b, byId[b.id]) : (current[b.id] || b))
            .filter(b => Number(b.installedCount || 0) > 0 || current[b.id] !== undefined);
        store.statusLoaded = loaded;
    }

    // the parts a bundle needs, has and lacks
    function partsOf(bundle) {
        var parts = ((bundle || {}).metadata || {}).items || [];
        var needed = parts.filter(p => p.available !== false && p.tier !== "optional");
        return {
            all: parts,
            installed: parts.filter(p => p.installed === true),
            missing: needed.filter(p => p.installed !== true),
            optional: parts.filter(p => p.installed !== true && p.available !== false && p.tier === "optional"),
            unavailable: parts.filter(p => p.installed !== true && p.available === false),
            needed: needed,
            neededInstalled: needed.filter(p => p.installed === true).length
        };
    }

    function read(key) {
        var k = String(key).split("/");
        if (k[0] === "o") {
            var o = store.omarchyItem(k[1]);
            return o ? o.enabled === true : undefined;
        }
        var p = store.plugin(k[1]);
        if (!p)
            return undefined;
        switch (k[2]) {
        case "on": return (p.placement || {}).enabled === true;
        case "host": return store.hostOf(p);
        case "edge": return store.popoutOf(p).edge;
        case "align": return store.popoutOf(p).align;
        case "pos": return { edge: store.popoutOf(p).edge, align: store.popoutOf(p).align };
        case "set": return ((p.placement || {}).settings || {})[k.slice(3).join("/")];
        }
        return undefined;
    }

    // shown at once; the reread after the queue empties puts back what was really written
    function mirror(id, change) {
        store.plugins = store.plugins.map(p => {
            if (p.id !== id)
                return p;
            var next = JSON.parse(JSON.stringify(p));
            next.placement = next.placement || {};
            change(next.placement);
            return next;
        });
        Changes.touch();
    }

    function write(key, v, done) {
        var k = String(key).split("/");
        if (k[0] === "o") {
            var id = k[1];
            store.omarchy = store.omarchy.map(o => o.id === id ? Object.assign({}, o, { enabled: v === true }) : o);
            Changes.touch();
            store.enqueue(["omarchy", "plugin", v === true ? "enable" : "disable", id], done, "omarchy");
            return;
        }
        var p = store.plugin(k[1]);
        if (!p) {
            done(false, I18n.tr("This add-on is no longer installed."));
            return;
        }
        var argv;
        var fp = store.popoutOf(p);
        switch (k[2]) {
        case "on":
            store.mirror(p.id, place => place.enabled = v === true);
            argv = [store.placeTool, p.id, "enabled", v === true ? "true" : "false"];
            break;
        case "host":
            store.mirror(p.id, place => place.host = String(v));
            argv = [store.placeTool, p.id, "host", String(v)];
            break;
        case "edge":
        case "align":
        case "pos":
            var pos = k[2] === "pos" ? (v || {}) : {};
            var edge = k[2] === "edge" ? String(v) : k[2] === "pos" && pos.edge ? String(pos.edge) : fp.edge;
            var align = k[2] === "align" ? String(v) : k[2] === "pos" && pos.align ? String(pos.align) : fp.align;
            store.mirror(p.id, place => place.framePopout = Object.assign({}, place.framePopout || {}, { edge: edge, align: align }));
            argv = [store.placeTool, p.id, "framePopout", edge, align, String(fp.hoverW), String(fp.hoverH)];
            break;
        case "set":
            var field = k.slice(3).join("/");
            var obj = {};
            obj[field] = v;
            store.mirror(p.id, place => {
                place.settings = Object.assign({}, place.settings || {});
                place.settings[field] = v;
            });
            argv = [store.placeTool, p.id, "settings", JSON.stringify(obj)];
            break;
        default:
            done(false, "");
            return;
        }
        store.enqueue(argv, done, "plugins");
    }

    function enqueue(argv, done, source) {
        store.queue = store.queue.concat([{ argv: argv, done: done, source: source }]);
        store.next();
    }

    function next() {
        if (store.active)
            return;
        if (store.queue.length === 0) {
            if (store.rereadAfterWrites.plugins)
                store.loadPlugins();
            if (store.rereadAfterWrites.omarchy)
                store.loadOmarchy();
            store.rereadAfterWrites = ({});
            return;
        }
        store.active = store.queue[0];
        store.queue = store.queue.slice(1);
        var flags = Object.assign({}, store.rereadAfterWrites);
        flags[store.active.source] = true;
        store.rereadAfterWrites = flags;
        var job = store.active;
        Changes.run(job.argv, function (ok, out, err, code) {
            store.active = null;
            if (job.done)
                job.done(ok, ok ? "" : store.writeError(err, code));
            store.next();
        }, { env: Spawn.env });
    }

    // a program that could not start already says what to do; otherwise plain words first, then what it said
    function writeError(err, code) {
        var said = store.lastLines(err);
        if (code < 0)
            return said;
        return said.length > 0
            ? I18n.tr("The change was not saved. Try again. It said: %1").arg(said)
            : I18n.tr("The change was not saved (exit %1). Try again.").arg(code);
    }

    property var rereadAfterWrites: ({})

    function update(id) {
        var p = store.plugin(id);
        if (!p)
            return;
        var name = store.nameOf(p);
        pluginTask.start(id, "update", [{
            argv: ["ryostore", "internal", "install-guest", "plugins", id],
            failText: I18n.tr("Could not update %1 (exit %2). Try again, or update it in RyoStore.").replace("%1", name).replace("%2", "%1")
        }]);
    }

    function remove(id) {
        var p = store.plugin(id);
        if (!p)
            return;
        var name = store.nameOf(p);
        pluginTask.start(id, "remove", [{
            argv: ["ryostore", "internal", "remove-guest", "plugins", id],
            failText: I18n.tr("Could not remove %1 (exit %2). Try again, or remove it in RyoStore.").replace("%1", name).replace("%2", "%1")
        }, {
            argv: [store.placeTool, id, "forget"],
            failText: I18n.tr("%1 is removed, but its settings stayed behind (exit %2). Try again to clear them.").replace("%1", name).replace("%2", "%1")
        }]);
    }

    function bundleArgv(verb, id) {
        return ["ryoku-host", "app", "--label", "ryostore", "--",
                "kitty", "--class", "ryostore", "-e", "ryostore-install", verb, "bundle", id];
    }

    function installBundle(id) {
        var b = store.bundle(id);
        var name = b ? String(b.name || id) : id;
        bundleTask.start(id, "install", [{
            argv: store.bundleArgv("install", id),
            failText: I18n.tr("Could not install the missing parts of %1 (exit %2). Check the terminal window's last lines, then try again.").replace("%1", name).replace("%2", "%1")
        }], false);
    }

    function removeBundle(id) {
        var b = store.bundle(id);
        var name = b ? String(b.name || id) : id;
        bundleTask.start(id, "remove", [{
            argv: store.bundleArgv("remove", id),
            failText: I18n.tr("Could not remove the parts of %1 (exit %2). Check the terminal window's last lines, then try again.").replace("%1", name).replace("%2", "%1")
        }], false);
    }

    function removeOmarchy(id) {
        var o = store.omarchyItem(id);
        var name = o ? String(o.name || id) : id;
        omarchyTask.start(id, "remove", [{
            argv: ["omarchy", "plugin", "remove", id, "--yes"],
            failText: I18n.tr("Could not remove %1 (exit %2). Try again, or remove it in RyoStore.").replace("%1", name).replace("%2", "%1")
        }]);
    }

    // another surface placing or the store installing shows here without a reopen
    FileView {
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/plugins.json"
        watchChanges: true
        printErrors: false
        onFileChanged: {
            if (!store.active && store.queue.length === 0)
                store.loadPlugins();
        }
    }
    FileView {
        path: (Quickshell.env("XDG_STATE_HOME") || (Quickshell.env("HOME") + "/.local/state")) + "/ryoku/store/plugins.json"
        watchChanges: true
        printErrors: false
        onFileChanged: {
            store.loadPlugins();
            store.loadCatalog();
        }
    }

    Task {
        id: pluginTask
        onFinished: (target, action, ok) => {
            store.loadPlugins();
            store.loadCatalog();
        }
    }

    Task {
        id: bundleTask
        onFinished: (target, action, ok) => store.loadStatus(target)
    }

    Task {
        id: omarchyTask
        onFinished: (target, action, ok) => store.loadOmarchy()
    }

    Process {
        id: listProc
        command: ["bash", store.discoverScript, "--all"]
        environment: Spawn.env
        stdout: StdioCollector { id: listOut }
        stderr: StdioCollector { id: listErr }
        onExited: code => {
            if (code !== 0) {
                store.readFailed("plugins", listErr.text, I18n.tr("Could not read the installed add-ons (exit %1)."), code);
            } else {
                try {
                    var parsed = JSON.parse(listOut.text || "[]");
                    if (!Array.isArray(parsed))
                        throw new Error("not a list");
                    store.widgetCount = parsed.filter(p => store.isWidget(p)).length;
                    store.plugins = parsed.filter(p => !store.isWidget(p));
                    store.pluginsLoaded = true;
                    store.setReadError("plugins", "");
                    Changes.touch();
                } catch (e) {
                    store.setReadError("plugins", I18n.tr("Could not read the installed add-ons: the list came back damaged."));
                }
            }
            if (store.rereadPlugins) {
                store.rereadPlugins = false;
                listProc.running = true;
            }
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
                store.readFailed("catalog", catErr.text, I18n.tr("Could not check for add-on updates (exit %1)."), code);
                return;
            }
            try {
                var doc = JSON.parse(catOut.text || "{}");
                var items = doc.items || [];
                if (!Array.isArray(items))
                    throw new Error("not a list");
                store.catalog = items;
                store.catalogKnown = items.length > 0 || doc.offline !== true;
                store.setReadError("catalog", "");
            } catch (e) {
                store.setReadError("catalog", I18n.tr("Could not check for add-on updates: RyoStore's answer came back damaged."));
            }
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
                store.readFailed("bundles", bundleErr.text, I18n.tr("Could not read your bundles (exit %1)."), code);
                return;
            }
            try {
                var items = (JSON.parse(bundleOut.text || "{}").items) || [];
                if (!Array.isArray(items))
                    throw new Error("not a list");
                store.applyBundleCatalog(items);
                store.bundlesLoaded = true;
                store.setReadError("bundles", "");
                store.loadStatus("");
            } catch (e) {
                store.setReadError("bundles", I18n.tr("Could not read your bundles: RyoStore's answer came back damaged."));
            }
        }
    }

    Process {
        id: statusProc
        environment: Spawn.env
        stdout: StdioCollector { id: statusOut }
        stderr: StdioCollector { id: statusErr }
        onExited: code => {
            var target = store.statusTarget;
            if (code !== 0) {
                store.readFailed("status", statusErr.text, I18n.tr("Could not check which bundle parts are installed (exit %1)."), code);
            } else {
                try {
                    var parsed = JSON.parse(statusOut.text || "{}");
                    var rows = target !== "" ? [parsed] : (parsed.bundles || []);
                    if (!Array.isArray(rows) || rows.some(r => !r || !Array.isArray(r.items)))
                        throw new Error("not a list");
                    store.applyStatuses(rows);
                    store.setReadError("status", "");
                } catch (e) {
                    store.setReadError("status", I18n.tr("Could not check which bundle parts are installed: the answer came back damaged."));
                }
            }
            if (store.statusWaiting.length > 0) {
                var next = store.statusWaiting[0];
                store.statusWaiting = store.statusWaiting.slice(1);
                Qt.callLater(store.loadStatus, next);
            }
        }
    }

    Process {
        id: omarchyProc
        // the omarchy command exists only while an item that ships it is in use
        command: ["sh", "-c", "command -v omarchy >/dev/null 2>&1 || exit 127; exec omarchy plugin list --json"]
        environment: Spawn.env
        stdout: StdioCollector { id: omarchyOut }
        stderr: StdioCollector { id: omarchyErr }
        onExited: code => {
            store.omarchyLoaded = true;
            if (code === 127) {
                store.omarchyAvailable = false;
                store.omarchy = [];
                store.setReadError("omarchy", "");
            } else if (code !== 0) {
                store.readFailed("omarchy", omarchyErr.text, I18n.tr("Could not read your Omarchy add-ons (exit %1)."), code);
            } else {
                try {
                    var rows = JSON.parse(omarchyOut.text || "[]");
                    if (!Array.isArray(rows))
                        throw new Error("not a list");
                    store.omarchyAvailable = true;
                    store.omarchy = rows.filter(r => r.firstParty !== true).map(r => Object.assign({}, r, {
                        kinds: r.kinds || [],
                        enabled: r.enabled === true,
                        canDisable: r.canDisable !== false,
                        blocked: r.blocked === true,
                        blockedReason: String(r.blockedReason || "")
                    }));
                    store.setReadError("omarchy", "");
                    Changes.touch();
                } catch (e) {
                    store.setReadError("omarchy", I18n.tr("Could not read your Omarchy add-ons: the list came back damaged."));
                }
            }
            if (store.rereadOmarchy) {
                store.rereadOmarchy = false;
                omarchyProc.running = true;
            }
        }
    }

    Component.onCompleted: Changes.registerStore("addons", store)
}
