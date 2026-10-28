pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import stage
import stage.modules.common as StageIsland
import "Singletons" as StageCfg
import "../desktop/Singletons" as WidgetStore
import "../visualizer/Singletons" as VizCfg
import Ryoku.Ui.Singletons

// The Stage Editor's store bridge: Ryoku's real desktop widgets, seen through
// the ported chrome's own widget API. The chrome (stage.modules.common.Config)
// reads its catalogue, counts and placements through this object instead of
// its island store, so the toolbar edits the desktop the user actually has:
// widgets.json for the built-ins and hosted faces, plugins.json (through the
// place tool) for plugin tiles, and the visualizer's own config for the bar.
//
// One per editing screen, owned by that screen's Desktop; the chrome installs
// it while the mode frames that monitor and drops it on the way out.
Scope {
    id: root

    // The output whose widget fork this provider reads and writes.
    property string monitor: ""
    // The roster rows, in the chrome's row shape: {id, label, icon, enabled,
    // group}. Fed from Desktop.addItems, so the catalogue is exactly what the
    // desktop can place, and stays reactive with it.
    property var rows: []
    // The plugin place tool (dev checkout or installed path).
    property string placeTool: "ryoku-plugins-place"
    // Ryoku's live wallpaper, mirrored into the chrome's read path so the
    // folder browser and the colour previews start from what the desktop
    // actually shows (the picks themselves flow back through the shim).
    property string wallpaperPath: ""

    // ── The chrome's view ────────────────────────────────────────────────────

    // The catalogue, in the drawer's row shape.
    readonly property var catalogue: {
        const out = [];
        for (const r of root.rows)
            out.push({ widgetId: r.id, name: r.label, icon: r.icon,
                category: r.group === "" ? "Ryoku" : r.group, description: "" });
        return out;
    }

    // The placed widgets, in the island's entry shape (Ryoku widgets are
    // singletons: one entry, id == widgetId).
    readonly property var activeWidgets: {
        const out = [];
        for (const r of root.rows)
            if (r.enabled)
                out.push({ id: r.id, widgetId: r.id });
        return out;
    }

    // The drop grid, from Ryoku's own stage settings.
    readonly property bool snapEnabled: StageCfg.Config.editGridSnap
    readonly property real gridSize: StageCfg.Config.editGridSize
    function toggleSnap() {
        StageCfg.Config.toggleEditGridSnap();
    }

    function count(widgetId) {
        for (const r of root.rows)
            if (r.id === widgetId)
                return r.enabled ? 1 : 0;
        return 0;
    }
    readonly property var inspectingRow: {
        const id = StageCfg.StageSession.inspecting;
        for (const row of root.rows || [])
            if (row.id === id)
                return row;
        return null;
    }
    readonly property string inspectingLabel: root.inspectingRow && root.inspectingRow.label
        ? root.inspectingRow.label : I18n.tr("Widget")
    readonly property string inspectingIcon: root.inspectingRow && root.inspectingRow.icon
        ? root.inspectingRow.icon : "widgets"

    // Visualizer and Depth are toolbar catalogues. Widget settings uses the
    // same provider path but stays hidden until a selected frame hands off to it.
    readonly property var extraSections: [
        {
            "section": "widget",
            "label": root.inspectingLabel,
            "icon": root.inspectingIcon,
            "tooltip": I18n.tr("Tune the selected widget"),
            "intro": "",
            "hidden": true,
            "back": StageCfg.StageSession.inspectingBack || "widgets",
            "page": widgetPage
        },
        {
            "section": "visualizer",
            "label": I18n.tr("Visualizer"),
            "icon": "graphic_eq",
            "tooltip": I18n.tr("The audio visualizer: look, place, colour and motion"),
            "intro": I18n.tr("Drag the look on the desktop to move it, its corner to size it and the dot to turn it, or set it by number below."),
            "page": visualizerPage
        },
        {
            "section": "depth",
            "label": I18n.tr("Depth"),
            "icon": "layers",
            "tooltip": I18n.tr("Cut the wallpaper into layers and lift widgets between them"),
            "intro": I18n.tr("Give the wallpaper depth: cut its subject out so widgets sit behind it, let layers drift, or turn it into a 3D scene."),
            "page": depthPage
        }
    ]
    Component {
        id: widgetPage
        StageWidgetPage {}
    }
    Component {
        id: visualizerPage
        StageVisualizerPage {}
    }
    Component {
        id: depthPage
        StageDepthPage {}
    }

    // ── Writers ──────────────────────────────────────────────────────────────

    // Enable a widget, optionally at a drop point, and record the walk-back:
    // the undo of an add is the state before it (a disable, or the old
    // placement a re-add moved), the redo is the same write again. Ryoku's
    // built-ins and hosted faces place through widgets.json (anchor "free" +
    // x/y); plugin tiles through the place tool; the visualizer owns its own
    // placement and is only switched on here.
    function addWidget(widgetId, x, y, monitorName) {
        const before = root.snapshot(widgetId);
        root._applyAdd(widgetId, x, y);
        GlobalStates.editHistoryPush({
            "undo": () => root.restore(widgetId, before),
            "redo": () => root._applyAdd(widgetId, x, y)
        });
        return widgetId;
    }

    function _applyAdd(widgetId, x, y) {
        const store = WidgetStore.Config;
        if (widgetId.indexOf("plugin:") === 0) {
            const pid = widgetId.slice(7);
            // A plugin draws on one host at a time, and the desktop draws only
            // the desktopWidget one: a store plugin that also offers a bar
            // glyph arrives on the bar host and would stay there, invisible.
            // QS Bar's own add claims topbarGlyph the same way.
            root.enqueue([root.placeTool, pid, "host", "desktopWidget"]);
            root.enqueue([root.placeTool, pid, "enabled", "true"]);
            if (x !== undefined && y !== undefined)
                root.enqueue([root.placeTool, pid, "desktopWidget", "" + Math.round(x), "" + Math.round(y)]);
            return;
        }
        if (widgetId === "visualizer") {
            VizCfg.Config.setEnabled(true);
            // A drop places the look's centre where the pointer let go, the same
            // way a built-in lands under the cursor: the visualiser box is
            // fractions of the monitor, so the chrome's pixel point converts here.
            if (x !== undefined && y !== undefined)
                root.placeVisualizerAt(x, y);
            return;
        }
        if (x === undefined || y === undefined) {
            store.setFor(root.monitor, widgetId + "Enabled", true);
            return;
        }
        const patch = {};
        patch[widgetId + "Enabled"] = true;
        patch[widgetId + "Anchor"] = "free";
        patch[widgetId + "X"] = Math.round(x);
        patch[widgetId + "Y"] = Math.round(y);
        store.setManyFor(root.monitor, patch);
    }

    function addVisualizer() {
        const v = VizCfg.Config;
        if (v.count >= v.maxVisualizers)
            return "";
        const wasEnabled = v.enabled;
        const stableId = v.addVisualizer();
        if (!stableId)
            return "";
        v.setEnabled(true);
        const id = "visualizer:" + stableId;
        const after = root.snapshot(id);
        GlobalStates.editHistoryPush({
            "undo": () => {
                root.restore(id, null);
                if (!wasEnabled)
                    v.setEnabled(false);
            },
            "redo": () => root.restore(id, after)
        });
        return stableId;
    }

    // The drop point is the widget's top-left in screen px; the visualiser has
    // no top-left to keep (it is a centred box), so the box centre lands there
    // and the store's own clamp keeps it on screen.
    function placeVisualizerAt(px, py) {
        const scr = root._screen();
        if (!scr)
            return;
        const v = VizCfg.Config;
        const nx = px / scr.width - v.w / 2;
        const ny = py / scr.height - v.h / 2;
        v.setBox(nx, ny, v.w, v.h, scr.width / Math.max(1, scr.height));
    }

    // The screen this provider frames, falling back to the first one.
    function _screen() {
        return Quickshell.screens.find(s => s.name === root.monitor)
            || Quickshell.screens[0] || null;
    }

    function removeWidget(instanceId) {
        root.restore(instanceId, null);
    }

    function visualizerId(instanceId) {
        const text = "" + instanceId;
        return text.indexOf("visualizer:") === 0 ? text.slice(11) : "";
    }
    function visualizerIndex(instanceId) {
        const stableId = root.visualizerId(instanceId);
        return stableId === "" ? -1 : VizCfg.Config.indexOfId(stableId);
    }

    // An indexed visualizer snapshot is its complete settings object plus the
    // roster size. The size distinguishes restoring a moved instance from
    // inserting one that was removed.
    function snapshot(instanceId) {
        const store = WidgetStore.Config;
        if (instanceId.indexOf("plugin:") === 0) {
            const pid = instanceId.slice(7);
            const e = (WidgetStore.Registry.allPlugins || []).find(p => p.id === pid) || null;
            if (!e || !(e.placement && e.placement.enabled === true))
                return null;
            return { plugin: pid, entry: JSON.parse(JSON.stringify(e.placement)) };
        }
        const vi = root.visualizerIndex(instanceId);
        if (vi >= 0) {
            if (!VizCfg.Config.enabled || vi >= VizCfg.Config.count)
                return null;
            return {
                viz: true,
                id: VizCfg.Config.idAt(vi),
                index: vi,
                count: VizCfg.Config.count,
                data: VizCfg.Config.cloneData(VizCfg.Config.dataAt(vi))
            };
        }
        if (instanceId === "visualizer") {
            const all = [];
            for (let i = 0; i < VizCfg.Config.count; ++i)
                all.push(VizCfg.Config.cloneData(VizCfg.Config.dataAt(i)));
            return {
                visualizers: true,
                enabled: VizCfg.Config.enabled,
                active: VizCfg.Config.activeId,
                instances: all
            };
        }
        if (store.get(instanceId + "Enabled", root.monitor) !== true)
            return null;
        return { key: instanceId,
            anchor: store.get(instanceId + "Anchor", root.monitor),
            x: store.get(instanceId + "X", root.monitor),
            y: store.get(instanceId + "Y", root.monitor) };
    }

    // Put a widget back the way a snapshot found it; `null` means it was not
    // on the desktop, so this is the disable or remove path.
    function restore(instanceId, snap) {
        const store = WidgetStore.Config;
        if (instanceId.indexOf("plugin:") === 0) {
            const pid = instanceId.slice(7);
            if (snap === null || snap === undefined) {
                root.enqueue([root.placeTool, pid, "enabled", "false"]);
                return;
            }
            if (snap.entry && Object.keys(snap.entry).length > 0)
                root.enqueue([root.placeTool, pid, "restore", JSON.stringify(snap.entry), "true"]);
            else
                root.enqueue([root.placeTool, pid, "enabled", "true"]);
            return;
        }
        const stableId = root.visualizerId(instanceId);
        if (stableId !== "") {
            const v = VizCfg.Config;
            const vi = v.indexOfId(stableId);
            if (snap === null || snap === undefined) {
                if (vi >= 0)
                    v.removeVisualizer(stableId);
                return;
            }
            if (vi < 0)
                v.insertVisualizer(snap.index, snap.data);
            else
                v.replaceVisualizer(vi, snap.data);
            v.setEnabled(true);
            v.setActive(snap.id || stableId);
            return;
        }
        if (instanceId === "visualizer") {
            if (snap && snap.visualizers)
                VizCfg.Config.replaceAll(snap.instances, snap.active, snap.enabled);
            else
                VizCfg.Config.setEnabled(false);
            return;
        }
        if (snap === null || snap === undefined) {
            store.setFor(root.monitor, instanceId + "Enabled", false);
            return;
        }
        const patch = {};
        patch[instanceId + "Enabled"] = true;
        patch[instanceId + "Anchor"] = snap.anchor;
        patch[instanceId + "X"] = snap.x;
        patch[instanceId + "Y"] = snap.y;
        store.setManyFor(root.monitor, patch);
    }

    // A visualizer gesture records one complete instance document. Full data,
    // rather than only placement, also makes a remove undo exact.
    function recordVisualizer(instanceId, before) {
        if (!before)
            return;
        const after = root.snapshot(instanceId);
        if (!after || JSON.stringify(before) === JSON.stringify(after))
            return;
        GlobalStates.editHistoryPush({
            "undo": () => root.restore(instanceId, before),
            "redo": () => root.restore(instanceId, after)
        });
    }

    // The framed screen's width over its height: the visualiser box is
    // fractions of the monitor, and its clamp needs the real proportions.
    function aspect() {
        const scr = root._screen();
        return scr ? scr.width / Math.max(1, scr.height) : 1;
    }

    // ── The place tool, one command at a time ────────────────────────────────

    // The chrome's store seam is one global; a monitor switch mounts the next
    // screen's provider before this one dies, so only clear the slot when it
    // still holds this provider.
    Component.onCompleted: {
        StageIsland.Config.widgetProvider = root;
        WidgetStore.Config.selectMonitor(root.monitor, true);
    }
    Component.onDestruction: {
        if (StageIsland.Config.widgetProvider === root) {
            StageIsland.Config.widgetProvider = null;
            if (WidgetStore.Config.writeMonitor === root.monitor)
                WidgetStore.Config.selectMonitor("");
        }
    }

    property var _queue: []
    Process {
        id: placeProc
        onExited: (code, status) => root._runNext()
    }
    // The queue seam: the desktop records a plugin gesture's redo as the same
    // place-tool command, run back through here so it can't trample an
    // in-flight undo.
    function enqueue(cmd) {
        root._queue = root._queue.concat([cmd]);
        if (!placeProc.running)
            root._runNext();
    }
    function _runNext() {
        if (root._queue.length === 0)
            return;
        const cmd = root._queue[0];
        root._queue = root._queue.slice(1);
        placeProc.command = cmd;
        placeProc.running = true;
    }
}
