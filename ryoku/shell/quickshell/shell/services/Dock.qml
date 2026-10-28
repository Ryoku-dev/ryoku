pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io
import inir.services as InirServices
import Ryoku.Ui.Singletons
import shell.services
import "lib/dock.js" as DockList

// Shared dock model: canonical desktop-entry pins, running windows and actions
// for the three universal dock designs and Sumi's in-rail RailDock. The list
// maths live in the tested lib/dock.js; settings persist in shell.json's one
// top-level dock object.
Singleton {
    id: root

    // The running design's stateful actions the Stage Editor's dock pages
    // need beyond the shared store (the ii design's saved layouts and
    // folders), published by that design's host while it is up; null
    // otherwise.
    property var designActions: null
    property int configRevision: 0

    // Icon resolution is not reactive on its own: iconFor() reads the desktop DB and
    // the icon theme through plain function calls, so a binding that resolves before
    // the desktop DB is scanned or the icon-theme cache is warm -- a fresh boot, or
    // right after an app updates its .desktop/icon -- sticks on the generic app
    // glyph until a shell reload (the "zen icon keeps resetting to a gear" break).
    // Bump iconRev to force every icon binding to re-resolve: on a desktop-DB
    // change and via a bounded warm-up poll after load.
    property int iconRev: 0
    property int _iconTries: 0
    property var _unresolvedIcons: new Map()
    readonly property int _unresolvedIconCount: root._unresolvedIcons.size
    property var _canonicalIds: new Map()
    property string _windowSignature: ""

    // A desktop-DB change -- an app installed, updated, or removed -- can add or fix
    // the icon a pin resolves to; re-resolve every dock icon when it fires so a pin
    // recovers its real icon live instead of waiting for a reload.
    Connections {
        target: DesktopEntries
        function onApplicationsChanged() {
            root._canonicalIds = new Map();
            root.iconRev++;
            root._syncClients(true);
        }
    }
    // The icon index lands a moment after startup (a `ryoku-shell icons` run);
    // bump iconRev the instant it does so pins re-resolve off the fallback
    // without waiting on the warm-up poll below.
    Connections {
        target: Icons
        function onRevChanged() { root.iconRev++; }
    }
    // The icon-theme cache warms a little after the shell starts and nothing
    // signals it. Poll only while a requested dock icon is unresolved.
    Timer {
        id: iconWarmup
        interval: 400
        repeat: true
        running: root._unresolvedIconCount > 0 && root._iconTries < 12
        onTriggered: {
            root._iconTries++;
            root.iconRev++;
        }
    }

    // Quickshell applications and a surprising number of desktop files expose
    // more than one identity. Persist desktop-entry ids, then resolve window
    // classes through the desktop database so every dock design groups the same
    // application and aliases do not create duplicate pins.
    readonly property var quickshellApps: ({ "Ryoku Settings": "ryoku-hub", "Ryostore": "ryostore", "ryovm": "ryovm" })
    function plainId(value) {
        return String(value ?? "").trim().replace(/\.desktop$/i, "");
    }
    function entryFor(value) {
        void root.iconRev
        return InirServices.AppSearch.lookupDesktopEntry(root.plainId(value), true)
    }
    function canonicalId(value) {
        const id = root.plainId(value);
        if (id.length === 0)
            return "";
        const key = id.toLowerCase();
        if (root._canonicalIds.has(key))
            return root._canonicalIds.get(key);
        const entry = root.entryFor(id);
        const canonical = entry?.id ? root.plainId(entry.id) : key;
        root._canonicalIds.set(key, canonical);
        return canonical;
    }
    function sameApp(a, b) {
        const left = root.canonicalId(a);
        return left.length > 0 && left === root.canonicalId(b);
    }
    function normalizedPins(values) {
        const out = [];
        for (const value of Array.from(values ?? [])) {
            const id = root.canonicalId(value?.desktop_id ?? value?.id ?? value);
            if (id.length > 0 && !out.includes(id))
                out.push(id);
        }
        return out;
    }
    function classOf(w) {
        if (!w) return "";
        const cls = w.appId;
        if (cls === "org.quickshell" && root.quickshellApps[w.title])
            return root.canonicalId(root.quickshellApps[w.title]);
        return root.canonicalId(cls);
    }

    property var _clients: []
    readonly property var clients: root._clients
    property var _focusedClient: null
    readonly property var focusedClient: root._focusedClient
    readonly property bool anyFocused: root._focusedClient !== null
    property string _activeClass: ""
    readonly property string activeClass: root._activeClass

    function _syncClients(force) {
        const wins = Array.from(Wm.windows ?? []);
        const toplevels = Array.from(Wm.toplevels ?? []);
        const focused = Wm.focusedWindow;
        const focusedId = focused ? String(focused.id ?? "") : "";
        const records = [];
        for (let i = 0; i < wins.length; ++i) {
            const w = wins[i];
            const id = String(w?.id ?? "");
            const appId = String(w?.appId ?? "");
            const specialId = appId === "org.quickshell"
                ? String(root.quickshellApps[w?.title] ?? "") : "";
            records.push({
                id: id,
                appId: appId,
                specialId: specialId,
                workspace: String(w?.workspace ?? "")
            });
        }
        const frameCounts = new Map();
        for (const record of records) {
            const id = root.canonicalId(record.specialId || record.appId);
            frameCounts.set(id, (frameCounts.get(id) || 0) + 1);
        }
        const directCounts = new Map();
        let activeToplevel = null;
        for (const toplevel of toplevels) {
            if (!toplevel?.appId)
                continue;
            const id = root.canonicalId(toplevel.appId === "org.quickshell"
                ? (root.quickshellApps[toplevel.title] || toplevel.appId) : toplevel.appId);
            if (!id)
                continue;
            directCounts.set(id, (directCounts.get(id) || 0) + 1);
            if (toplevel.activated)
                activeToplevel = toplevel;
        }
        for (const [id, count] of directCounts) {
            for (let i = frameCounts.get(id) || 0; i < count; ++i)
                records.push({ id: "pending:" + id + ":" + i, appId: id, specialId: "", workspace: "" });
        }
        records.sort((a, b) => a.id < b.id ? -1 : (a.id > b.id ? 1 : 0));
        const signature = (focusedId || (activeToplevel ? "active:" + activeToplevel.appId : "")) + "\u0001" + records.map(record =>
            record.id + "\u0000" + record.appId + "\u0000"
                + record.specialId + "\u0000" + record.workspace).join("\u0001");
        if (force !== true && signature === root._windowSignature)
            return;

        const result = [];
        for (let i = 0; i < records.length; ++i) {
            const record = records[i];
            const className = root.canonicalId(record.specialId || record.appId);
            if (className)
                result.push({ className: className, address: record.id });
        }
        root._windowSignature = signature;
        root._clients = result;
        root._focusedClient = focused || activeToplevel;
        root._activeClass = focused
            ? root.canonicalId((focused.appId === "org.quickshell"
                ? root.quickshellApps[focused.title] : "") || focused.appId)
            : activeToplevel ? root.canonicalId(activeToplevel.appId === "org.quickshell"
                ? (root.quickshellApps[activeToplevel.title] || activeToplevel.appId) : activeToplevel.appId) : "";
    }

    Connections {
        target: Wm
        function onWindowsChanged() { root._syncClients(false); }
        function onToplevelsChanged() { root._syncClients(false); }
        function onFocusedWindowChanged() { root._syncClients(false); }
    }
    Component.onCompleted: root._syncClients(true)
    // Pinned first, then running-unpinned in id order.
    function resolve(pinned, activeClients) {
        const canonicalPins = root.normalizedPins(pinned);
        const source = activeClients === undefined ? root.clients : activeClients;
        const canonicalClients = source === root.clients ? Array.from(source) : Array.from(source ?? []).map(client => ({
            className: root.canonicalId(client.className),
            address: client.address
        })).filter(client => client.className.length > 0);
        return DockList.resolve(canonicalPins, canonicalClients);
    }
    function pin(pinned, className) {
        const pins = root.normalizedPins(pinned);
        const id = root.canonicalId(className);
        return id.length > 0 && !pins.includes(id) ? pins.concat([id]) : pins;
    }
    function unpin(pinned, className) {
        const id = root.canonicalId(className);
        return root.normalizedPins(pinned).filter(pinId => pinId !== id);
    }

    function countFor(className) {
        const list = root.clients;
        let n = 0;
        for (let i = 0; i < list.length; ++i)
            if (root.sameApp(list[i].className, className)) ++n;
        return n;
    }

    // Fallback pins so an empty dock is not mistaken for a missing one.
    function starterPins() {
        const out = [];
        for (const className of ["kitty", "chromium", "nautilus"])
            if (DesktopEntries.heuristicLookup(className)) out.push(className);
        return out;
    }

    // ── the dock store (shell.json top-level `dock`) ─────────────────────────
    // The dock is a first-class shell surface now, so its look and pins live in
    // one top-level store rather than per bar style. Every consumer reads and
    // writes it through here, so the copy-on-write below is the single path that
    // persists it.

    // The look registry, shaped like Theme.workspaceStyleOptions so one control
    // renders them all. Persisted under `dock.style` (default islands, so an
    // existing desktop is unchanged). A style only changes what the band draws.
    readonly property var styleOptions: [
        { key: "islands", label: I18n.tr("Islands"), detail: I18n.tr("Split pills") },
        { key: "rail",    label: I18n.tr("Rail"),    detail: I18n.tr("One continuous plate") },
        { key: "ledger",  label: I18n.tr("Ledger"),  detail: I18n.tr("Numbered cells") },
        { key: "tanzaku", label: I18n.tr("Tanzaku"), detail: I18n.tr("Hanging strips") },
        { key: "seal",    label: I18n.tr("Seal"),    detail: I18n.tr("Colour means running") }
    ]
    readonly property string design: String(root.cfg("design", "ryoku"))
    onDesignChanged: root.closeMenu()
    // The designs the shell ships. Any other design names a dock installed from
    // RyoStore (DockProducts), which UniversalDockHost runs from its own view and
    // which shares the Ryoku design's context menu below.
    readonly property var builtinDesigns: ["ryoku", "python", "shima", "ii", "none"]
    readonly property bool storeDesign: !root.builtinDesigns.includes(root.design)

    function cfg(key, fallback) {
        void root.configRevision;
        const d = Config.dock;
        if (d && d[key] !== undefined && d[key] !== null)
            return d[key];
        if (key === "design" && Config.barStyle === "iris")
            return "shima";
        return fallback;
    }
    function designCfg(designName, key, fallback) {
        const d = root.cfg(designName, {});
        return d && d[key] !== undefined && d[key] !== null ? d[key] : fallback;
    }
    function writeCfg(next) {
        Config.dock = next;
        root.configRevision++;
        cfgCtl.queued += "call settings.patch " + JSON.stringify({ path: "dock", value: next }) + "\n";
        if (cfgCtl.connected)
            cfgCtl.flushQueued();
        else
            cfgCtl.connected = true;
    }
    function setCfg(key, value) {
        const cur = Config.dock || {};
        const next = {};
        for (const k in cur) next[k] = cur[k];
        next[key] = value;
        root.writeCfg(next);
    }
    function setDesignCfg(designName, key, value) {
        const cur = root.cfg(designName, {});
        const next = {};
        for (const k in cur) next[k] = cur[k];
        next[key] = value;
        root.setCfg(designName, next);
    }
    function setPinned(array) {
        const cur = Config.dock || {};
        const next = {};
        for (const k in cur) next[k] = cur[k];
        next.pinned = root.normalizedPins(array);
        next.pinnedConfigured = true;
        root.writeCfg(next);
    }
    function movePinned(from, to) {
        const pins = root.pinnedOrStarter();
        if (from < 0 || from >= pins.length || to < 0 || to >= pins.length || from === to)
            return;
        const value = pins.splice(from, 1)[0];
        pins.splice(to, 0, value);
        root.setPinned(pins);
    }
    function togglePin(className) {
        const pins = root.pinnedOrStarter();
        root.setPinned(pins.includes(root.canonicalId(className))
            ? root.unpin(pins, className) : root.pin(pins, className));
    }
    function resolvedEdge() {
        const edge = String(root.cfg("edge", "auto"));
        if (["top", "bottom", "left", "right"].includes(edge))
            return edge;
        if (Config.barStyle === "qsbar")
            return Config.qsbar?.barPosition === "bottom" ? "top" : "bottom";
        return "bottom";
    }
    // The daemon's control socket, connected only when there is something to say.
    Socket {
        id: cfgCtl
        path: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-shell.sock"
        property string queued: ""
        function flushQueued() {
            if (cfgCtl.queued.length === 0)
                return;
            cfgCtl.write(cfgCtl.queued);
            cfgCtl.flush();
            cfgCtl.queued = "";
        }
        onConnectionStateChanged: if (cfgCtl.connected) cfgCtl.flushQueued()
    }

    // Starter pins are only a first-run fallback. Once the user has edited the
    // list, an empty list is deliberate and must survive a shell reload.
    function pinnedOrStarter() {
        const p = root.normalizedPins(root.cfg("pinned", []));
        if (p.length || root.cfg("pinnedConfigured", false) === true)
            return p;
        return root.normalizedPins(root.starterPins());
    }

    // Desktop-entry icon, then class-as-icon-name; "" so callers can draw the
    // shell's generic app glyph.
    function iconFor(className) {
        void root.iconRev;
        const id = root.canonicalId(className);
        const desktop = root.entryFor(id);
        const byEntry = (desktop && desktop.icon) ? Icons.path(desktop.icon, true) : "";
        const resolved = byEntry !== "" ? byEntry : Icons.path(id.toLowerCase(), true);
        const unresolved = new Map(root._unresolvedIcons);
        if (resolved === "" && id.length > 0) {
            if (!unresolved.has(id)) {
                if (unresolved.size === 0)
                    root._iconTries = 0;
                unresolved.set(id, true);
                root._unresolvedIcons = unresolved;
            }
        } else if (unresolved.has(id)) {
            unresolved.delete(id);
            root._unresolvedIcons = unresolved;
        }
        return resolved;
    }

    function launch(className) {
        const entry = root.entryFor(className);
        if (entry)
            AppLaunch.run(entry, null);
    }
    // No clients -> launch; focused already -> cycle by address; else focus,
    // preferring a client on the active workspace.
    function activate(className) {
        const id = root.canonicalId(className);
        const wins = Wm.windows;
        const matches = [];
        for (let i = 0; i < wins.length; ++i)
            if (root.sameApp(root.classOf(wins[i]), id))
                matches.push(wins[i]);
        if (matches.length === 0) {
            const direct = Array.from(Wm.toplevels ?? []).find(toplevel =>
                root.sameApp(toplevel?.appId === "org.quickshell"
                    ? (root.quickshellApps[toplevel.title] || toplevel.appId) : toplevel?.appId, id));
            if (direct)
                direct.activate();
            else
                root.launch(id);
            return;
        }
        matches.sort((a, b) => a.id < b.id ? -1 : (a.id > b.id ? 1 : 0));
        const focused = Wm.focusedWindow;
        const active = focused ? focused.id : "";
        const idx = matches.findIndex(m => m.id === active);
        let target;
        if (idx >= 0)
            target = matches[(idx + 1) % matches.length];
        else {
            const wsName = Wm.focusedWorkspace ? Wm.focusedWorkspace.name : "";
            target = matches.find(m => m.workspace === wsName) || matches[0];
        }
        Wm.focusWindow(target.id);
    }

    // Close every window of a class (dock menu Close).
    function closeAll(className) {
        const wins = Wm.windows;
        for (let i = 0; i < wins.length; ++i)
            if (root.sameApp(root.classOf(wins[i]), className))
                Wm.closeWindow(wins[i].id);
    }

    // ── right-click context menu ───────────────────────────────────────────────
    // One menu at a time, owned by the monitor the click came from (menuScreen);
    // the per-monitor DockMenuOverlay renders it and dismisses on an outside click.
    property string menuClass: ""
    property bool menuPinned: false
    property int menuCount: 0
    property real menuGx: 0
    property real menuGy: 0
    property string menuScreen: ""
    property string menuEdge: "bottom"
    property real menuEdgeClear: 54
    readonly property bool menuOpen: root.menuClass !== ""
    function openMenu(className, pinned, count, gx, gy, screenName, edge, edgeClear) {
        root.menuClass = className;
        root.menuPinned = pinned;
        root.menuCount = count;
        root.menuGx = gx;
        root.menuGy = gy;
        root.menuScreen = screenName;
        root.menuEdge = edge;
        root.menuEdgeClear = edgeClear;
    }
    function closeMenu() { root.menuClass = ""; }
    function menuActOpen() {
        if (root.menuCount > 0)
            root.launch(root.menuClass);
        else {
            root.activate(root.menuClass);
        }
        root.closeMenu();
    }
    function menuActPin() {
        const pins = root.pinnedOrStarter();
        root.setPinned(root.menuPinned ? root.unpin(pins, root.menuClass)
                                       : root.pin(pins, root.menuClass));
        root.closeMenu();
    }
    function menuActClose() {
        root.closeAll(root.menuClass);
        root.closeMenu();
    }
}
