pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import shell.services as Svc
import "../../../../modules/launcher/shared/Singletons" as LS
import "../../../../modules/launcher/shared/lib/results.js" as Results
import "lib/modes.js" as Modes
import "lib/rest.js" as Rest
import "lib/rows.js" as Rows
import "lib/text.js" as Words
import "lib/rashin.js" as Rashin

// The launcher's query and selection model. Provider queries are impure, so
// rows are an imperative snapshot refreshed on the next tick after the query,
// the mode or Dispatcher.revision moves, never a binding.
Item {
    id: engine

    visible: false

    property bool active: false
    property string query: ""
    property string mode: "all"

    readonly property var rows: engine._rows
    readonly property int selectedIndex: engine._index
    readonly property var selectedRow: engine._index >= 0 && engine._index < engine._rows.length
        ? engine._rows[engine._index] : null
    readonly property string selectedKey: engine._key
    readonly property int resultCount: engine._rows.length
    readonly property string section: engine._section
    readonly property bool askAvailable: engine._askAvailable
    readonly property var modes: Modes.modesFor(engine.askAvailable)
    readonly property var route: Modes.routeFor(engine.mode, engine.query,
        function (text) { return LS.Dispatcher.route(text); }, engine.askAvailable)
    readonly property bool askMode: engine.route.ask === true
    readonly property string askQuestion: engine.askMode ? engine.route.query : ""
    readonly property bool resting: engine.route.rest === true
    // "seal" is the design's own search mark, anything else a Material symbol
    readonly property string glyph: Modes.glyphFor(engine.route)
    readonly property var player: Svc.Media.player
    readonly property bool busy: {
        void LS.Dispatcher.busyRevision;
        const busy = LS.Dispatcher.busyProviders;
        if (engine.route.ask || engine.route.rest)
            return false;
        if (engine.route.provider.length > 0)
            return Boolean(busy[engine.route.provider]);
        for (const id in busy) {
            const provider = LS.Dispatcher.registry[id];
            if (provider && (provider.defaultProvider || provider.numericFallback))
                return true;
        }
        return false;
    }
    readonly property string ghost: {
        const route = engine.route;
        if (engine._rows.length === 0 || route.ask || route.rest || route.prefix.length > 0)
            return "";
        if (engine.mode !== "all" && engine.mode !== "apps")
            return "";
        const top = engine._rows[0];
        if (!top || top.providerId !== "apps")
            return "";
        return Words.ghostFor(engine.query, top.title);
    }

    // Every open window joined to its workspace, for running dots and the lead.
    readonly property var openWindows: {
        void LS.Dispatcher.revision;
        if (!engine.active)
            return [];
        const residue = Wm.windows;
        const workspaceOf = {};
        for (let i = 0; i < residue.length; i++)
            if (residue[i] && residue[i].id !== undefined)
                workspaceOf["$" + String(residue[i].id)] = residue[i].workspace;
        const rows = LS.Dispatcher.resultsFor("windows", "", "", 0);
        const out = [];
        for (let j = 0; j < rows.length; j++) {
            const row = rows[j];
            out.push({
                row: row,
                id: String(row.windowAddress),
                appId: String(row.appId || ""),
                title: String(row.title || ""),
                workspace: engine.workspaceLabel(workspaceOf["$" + String(row.windowAddress)])
            });
        }
        return out;
    }

    property var _rows: []
    property int _index: -1
    property string _key: ""
    property string _section: ""
    property string _token: ""
    property string _signature: ""
    property string _prefer: ""
    property bool _queued: false
    // the selection rides the top result until the user moves it, so a row an
    // async provider adds above (the calculator) takes it
    property bool _followTop: true
    property bool _askAvailable: false

    signal closeRequested()
    signal flashed(string text)
    // a row ran and the vessel is closing: its icon pinches
    signal ran(string resultKey)

    onQueryChanged: engine.requery()
    onModeChanged: engine.requery()
    onActiveChanged: {
        if (engine.active)
            engine.schedule();
    }
    onAskAvailableChanged: {
        if (!engine.askAvailable && engine.mode === "ask")
            engine.mode = "all";
        engine.requery();
    }

    Connections {
        target: LS.Dispatcher
        function onRevisionChanged() { engine.schedule(); }
    }

    FileView {
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config"))
            + "/ryoku/rashin.json"
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: engine._askAvailable = Rashin.rashinEnabled(text())
        onLoadFailed: engine._askAvailable = false
    }

    function requery() {
        engine._followTop = true;
        engine.schedule();
    }

    function schedule() {
        if (!engine.active || engine._queued)
            return;
        engine._queued = true;
        Qt.callLater(engine.evaluate);
    }

    function reset() {
        engine._prefer = "";
        engine._token = "";
        engine._signature = "";
        engine.query = "";
        engine.mode = "all";
        engine._followTop = true;
        engine._rows = [];
        engine._index = -1;
        engine._key = "";
        engine._section = "";
        // the first rows land in the same frame the sheet opens, so the level
        // starts at its height instead of growing from four rows
        engine.evaluate();
    }

    function workspaceLabel(id) {
        const key = String(id === undefined || id === null ? "" : id);
        if (key.length === 0)
            return "";
        const ws = Wm.workspaceById(key) || Wm.workspaceByName(key);
        return ws ? String(ws.name || ws.index || key) : key;
    }

    function categoriesOf(row) {
        const entry = DesktopEntries.byId(String(row && (row.appId || row.id) || ""));
        return entry ? entry.categories : [];
    }

    function pinnedList() {
        const source = LS.LauncherConfig.pinnedApps || [];
        const out = [];
        for (let i = 0; i < source.length; i++)
            out.push(String(source[i]));
        return out;
    }

    function askRowFor(question) {
        return {
            id: "ask",
            type: "Ask",
            title: question,
            question: question,
            subtitle: "",
            icon: "",
            score: 0,
            actions: [{
                id: "ask",
                name: I18n.tr("Ask Rashin"),
                icon: "",
                closeOnExecute: false,
                execute: function () {}
            }]
        };
    }

    function evaluate() {
        engine._queued = false;
        if (!engine.active)
            return;
        const route = engine.route;
        const token = [engine.mode, route.provider, route.prefix, route.ask, route.rest].join("|");
        let section = "";
        let next;
        if (route.ask) {
            next = Results.normalizeRows("ask", [engine.askRowFor(route.query)]);
        } else if (route.rest) {
            const rest = Rest.restOrder(LS.Dispatcher.resultsFor("apps", "", "", 0),
                engine.pinnedList(), LS.Frecency.usage, engine.categoriesOf, Rest.REST_LIMIT);
            next = rest.rows;
            section = rest.section;
        } else {
            next = LS.Dispatcher.resultsFor(route.provider.length > 0 ? route.provider : null,
                route.query, route.prefix, engine.mode === "all" ? LS.Metrics.maxResults : 0);
            if (engine.mode === "all")
                next = Results.hideDuplicateWindowRows(next);
        }
        // an async provider still searching the same route keeps its last rows,
        // so the level does not dip and refill between keystrokes
        if (next.length === 0 && engine.busy && token === engine._token && engine._rows.length > 0)
            return;
        engine._token = token;
        engine._section = section;
        const signature = Rows.signature(next);
        if (signature !== engine._signature) {
            engine._signature = signature;
            engine._rows = next;
        }
        engine.reselect();
    }

    function reselect() {
        const rows = engine._rows;
        if (rows.length === 0) {
            engine._index = -1;
            engine._key = "";
            return;
        }
        if (engine._prefer.length > 0) {
            const prefer = engine._prefer;
            engine._prefer = "";
            engine._followTop = false;
            for (let i = 0; i < rows.length; i++) {
                if (rows[i].providerId === prefer) {
                    engine.select(i);
                    return;
                }
            }
            engine.select(0);
            return;
        }
        if (engine._followTop) {
            engine.select(0);
            return;
        }
        const kept = Results.reconcileSelection(rows, engine._key, Math.max(0, engine._index));
        engine._index = kept.index;
        engine._key = kept.key;
    }

    function select(index) {
        const rows = engine._rows;
        if (rows.length === 0)
            return;
        const bounded = Math.max(0, Math.min(rows.length - 1, Number(index) | 0));
        engine._index = bounded;
        engine._key = String(rows[bounded].resultKey || "");
    }

    function move(delta) {
        if (engine._rows.length === 0)
            return;
        engine._followTop = false;
        engine.select(engine._index < 0 ? 0 : engine._index + delta);
    }

    // a selection the user made (hover, click) that later rows must not move
    function pick(index) {
        engine._followTop = false;
        engine.select(index);
    }

    function run(row, action) {
        if (!row)
            return false;
        const chosen = action || row.primaryAction;
        if (!chosen || chosen.enabled === false || typeof chosen.execute !== "function")
            return false;
        try {
            chosen.execute();
        } catch (error) {
            engine.flashed(String(error || I18n.tr("Action failed")).split("\n")[0]);
            return false;
        }
        if (row.providerId === "calc") {
            engine.flashed(I18n.tr("Copied"));
            return true;
        }
        if (chosen.closeOnExecute !== false) {
            engine.ran(String(row.resultKey || ""));
            engine.closeRequested();
        }
        return true;
    }

    function runIndex(n) {
        const index = Number(n) - 1;
        if (index < 0 || index >= engine._rows.length)
            return false;
        engine.select(index);
        return engine.run(engine._rows[index], null);
    }

    function acceptGhost() {
        const rest = engine.ghost;
        if (rest.length > 0)
            engine.query = engine.query + rest;
    }

    function setMode(m) {
        const next = String(m || "");
        if (engine.modes.indexOf(next) < 0 || next === engine.mode)
            return;
        const query = Modes.queryForMode(next, engine.query,
            function (text) { return LS.Dispatcher.route(text); });
        engine.mode = next;
        engine.query = query;
    }

    // Enter on an empty Apps search: the same words go to the web row in All.
    function searchWeb() {
        engine._prefer = "web";
        if (engine.mode !== "all")
            engine.mode = "all";
        else
            engine.requery();
    }

    function appEntry(row) {
        if (!row || row.providerId !== "apps")
            return null;
        return DesktopEntries.byId(String(row.appId || row.id)) || null;
    }

    function focusWindow(window) {
        const action = window && window.row ? window.row.primaryAction : null;
        if (!action || typeof action.execute !== "function")
            return;
        // the windows provider defers the focus past the vessel's close
        action.execute();
        engine.closeRequested();
    }

    function windowsFor(row) {
        if (!row)
            return [];
        let matches;
        if (row.providerId === "windows") {
            matches = engine.openWindows.filter(w => w.id === String(row.windowAddress));
        } else if (row.providerId === "apps") {
            const entry = engine.appEntry(row);
            const id = String(row.appId || row.id);
            const startupClass = entry ? String(entry.startupClass || "") : "";
            matches = engine.openWindows.filter(w => Rows.sameApp(id, startupClass, w.appId));
        } else {
            return [];
        }
        return matches.map(w => ({
            id: w.id,
            title: w.title,
            workspace: w.workspace,
            focus: function () { engine.focusWindow(w); }
        }));
    }

    function workspaceFor(row) {
        if (!row || row.providerId !== "windows")
            return "";
        const address = String(row.windowAddress);
        const list = engine.openWindows;
        for (let i = 0; i < list.length; i++)
            if (list[i].id === address)
                return list[i].workspace;
        return "";
    }

    function isPlayingApp(row) {
        if (!row || row.providerId !== "apps")
            return false;
        const player = Svc.Media.player;
        if (!player || !Svc.Media.playing)
            return false;
        const entry = engine.appEntry(row);
        if (Rows.sameApp(String(row.appId || row.id), entry ? String(entry.startupClass || "") : "",
                player.desktopEntry))
            return true;
        return Boolean(entry) && String(player.identity || "").toLowerCase()
            === String(entry.name || "").toLowerCase();
    }
}
