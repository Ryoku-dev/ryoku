import QtQuick
import Quickshell
import Quickshell.Wayland
import Quickshell.Io
import Quickshell.Widgets
import Ryoku.Ui.Singletons
import "Singletons"
import "clock"
import "calendar"
import "music"
import "aio"
import "stats"
import "weather"
import "notes"
import "dayprogress"
import "shape"
import "iris"
import "iris/IrisRoster.js" as IrisRoster
import "python"
import "python/PythonRoster.js" as PythonRoster
import Ryoku.PluginKit
import shell.services as Services
import "../stage"
import "../stage/Singletons" as StageCfg
import stage as Stage
import stage.modules.ii.editMode as StageEdit
import stage.modules.ii.background as StageBg
import stage.modules.ii.background.shortcuts as StageShortcuts
import stage.modules.common as StageIsland
import stage.modules.common.widgets as StageWidgets
import stage.services as StageServices
import "../visualizer/Singletons" as VizCfg
import "../visualizer" as Viz
import "../wallpaper" as WallpaperMod
// The iRiS frame singleton reports the dock's edge and its visible vs reserved
// depth, so the Edit widgets bar can clear a bottom dock that reserves nothing.
import inir.modules.iris.frame

// desktop widgets layer: WlrLayer.Bottom (below windows), instantiated once per
// monitor by the main shell, carrying the clock. only clicks on bare wallpaper
// land here, so windows above keep their input. drag = move (snaps to the
// fade-in grid), right-click a widget = its menu, right-click empty = global
// menu. every knob is live from Config; drag, menus and Ryoku Settings all
// write the same file, so surfaces retune with no reload.
Scope {
    id: root

    // the monitor this instance renders on, bound by the main shell per screen.
    property var screen
    // controller hook; the desktop layer defaults on.
    property bool active: true
    property bool widgetsEnabled: true
    property string wallpaperUrl: ""
    property string wallpaperPath: ""
    property string wallpaperFit: "Cover"
    property var wallpaperTransition: null
    property string videoUrl: ""
    // The in-shell clip's audio, threaded from the wallpaper bridge to the
    // backdrop's player: muted by default, volume 0-100.
    property bool videoMuted: true
    property int videoVolume: 100
    property bool videoPaused: false
    // The ryogami-live yield flag (default "ryogami" engine): hide the painter
    // while the C player owns the background layer.
    property bool wallpaperLive: false

    // Stage z-ordering (docs/stage.md): behind-layers sit below the widgets
    // (z 2), in-front layers above them (z 4), and a widget the user lifted
    // (Config.front) rises above even the in-front layers (z 5). Gated off for
    // video/live walls, which the daemon skips.
    readonly property bool stageOn: StageCfg.StageBackend.isActiveFor(root.wallpaperPath)
        && root.videoUrl === "" && !root.wallpaperLive
    readonly property bool stageParallax: StageCfg.StageBackend.isParallaxFor(root.wallpaperPath)
        && root.videoUrl === "" && !root.wallpaperLive
    function widgetZ(id) {
        if (!root.stageOn)
            return 0;
        return StageCfg.Config.isFront(id) ? 5 : 3;
    }
    readonly property var stageState: Services.ShellState.forScreen(root.screen)
    // The visualiser paints inside this surface (docs/stage.md) while the
    // stage cuts this wall, or while the Stage Editor frames the monitor: the
    // desktop lifts to Top then, so the inline look still clears open windows
    // and the placement grip shares its surface. In overlay mode outside the
    // editor it keeps its own raised surface.
    readonly property bool hostsVisualizer: VizCfg.Config.enabled
        && (root.stageComposing
            || (root.stageOn && !(root.stageState && root.stageState.visualizerOverlay)))
    readonly property string monitorName: root.screen ? root.screen.name : ""
    // Edit widgets lifts this desktop above open windows. Per-widget locks stay
    // authoritative and can be toggled from the canvas.
    readonly property bool stageComposing: StageCfg.StageSession.onMonitor(root.monitorName)
    // The editor grid the bar drives; slots snap to it while composing.
    readonly property real editGridSize: StageCfg.Config.editGridSize
    readonly property bool editGridSnap: StageCfg.Config.editGridSnap
    // The Stage Editor's shrink: when the ported chrome is framing THIS
    // monitor, the desktop is drawn as the card the toolbar frames (a scale +
    // translate about the usable area's centre), on the mode's one animated
    // scalar, so the desktop and the chrome shrink as one rectangle across two
    // scene graphs. Off the edited screen it stays the identity. desktopContent
    // carries the matrix.
    readonly property bool stageEditing: Stage.GlobalStates.editModeMonitor === root.monitorName
        && (Stage.GlobalStates.editMode || Stage.GlobalStates.editProgress > 0)
    readonly property bool stageFramingHere: root.stageEditing
        && StageServices.WallpaperLayout.liveScreen === root.monitorName
    property real stageFramingDim: root.stageFramingHere ? 1 : 0
    Behavior on stageFramingDim {
        NumberAnimation {
            duration: Tokens.dur(180)
            easing.type: Easing.OutCubic
        }
    }
    readonly property matrix4x4 stageMatrix: root.stageEditing
        ? StageEdit.EditModeInsets.editMatrixFor(root.monitorName,
            win.width, win.height,
            Stage.GlobalStates.editProgress, Stage.GlobalStates.editDrawerProgress)
        : Qt.matrix4x4(1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1)
    // Human titles for every framed widget: the built-ins by name, then the
    // hosted rosters' own labels, so a frame never reads "irisClock".
    function widgetTitle(w) {
        const n = { clock: "Clock", calendar: "Calendar", music: "Music", aio: "All-in-one", stats: "System stats", weather: "Weather", notes: "Notes", dayprogress: "Day Progress", shape: "Shape", visualizer: "Visualizer" };
        if (n[w])
            return n[w];
        const iris = IrisRoster.byPrefix(w);
        if (iris)
            return iris.label;
        const py = PythonRoster.byPrefix(w);
        if (py)
            return py.label;
        return w;
    }
    function widgetValue(widget, suffix) {
        return Config.get(widget + suffix, root.monitorName);
    }
    // The Add drop-down's model (docs/stage.md, "Edit widgets"): every widget
    // with its current on/off state and the group it belongs to. The built-ins
    // and the visualizer are the shell's own widgets (group ""); store-installed
    // plugin widgets carry the group from their manifest's `set`, so a whole
    // installed suite lands under one caption and any future set groups itself
    // with no change here. A plugin whose manifest names no set falls under a
    // generic Plugins group. The array stays flat: built-ins first (visualizer
    // last), then each set contiguously in the order its first widget appeared.
    readonly property var addItems: {
        const bi = [
            { id: "clock", label: "Clock", icon: "schedule", enabled: root.widgetValue("clock", "Enabled"), group: "" },
            { id: "calendar", label: "Calendar", icon: "calendar_month", enabled: root.widgetValue("calendar", "Enabled"), group: "" },
            { id: "music", label: "Music", icon: "music_note", enabled: root.widgetValue("music", "Enabled"), group: "" },
            { id: "aio", label: "All-in-one", icon: "dashboard", enabled: root.widgetValue("aio", "Enabled"), group: "" },
            { id: "stats", label: "System stats", icon: "monitor_heart", enabled: root.widgetValue("stats", "Enabled"), group: "" },
            { id: "weather", label: "Weather", icon: "partly_cloudy_day", enabled: root.widgetValue("weather", "Enabled"), group: "" },
            { id: "notes", label: "Notes", icon: "sticky_note_2", enabled: root.widgetValue("notes", "Enabled"), group: "" },
            { id: "dayprogress", label: "Day Progress", icon: "donut_large", enabled: root.widgetValue("dayprogress", "Enabled"), group: "" },
            { id: "shape", label: "Shape", icon: "category", enabled: root.widgetValue("shape", "Enabled"), group: "" },
            { id: "visualizer", label: "Visualizer", icon: "graphic_eq", enabled: VizCfg.Config.enabled, group: "" }
        ];
        // Fallback group name for a plugin whose manifest names no set. Plain
        // like the built-in labels above; the Hub translates its own copy.
        const other = "Plugins";
        const order = [];
        const byGroup = {};
        // The installed desktop-capable set, enabled or not (discover.sh --all),
        // so a hidden plugin keeps its row and the switch can bring it back.
        // The switch reads the live placed set, the same list the tiles render
        // from, so the row can never disagree with the desktop.
        const placed = win.desktopPluginIds || [];
        const installed = (Registry.allPlugins || []).filter(p => {
            const hosts = (p.manifest && Array.isArray(p.manifest.hosts)) ? p.manifest.hosts : [];
            return (p.placement && p.placement.host === "desktopWidget") || hosts.indexOf("desktopWidget") >= 0;
        });
        for (var i = 0; i < installed.length; i++) {
            const e = installed[i];
            const pid = e.id;
            const set = (e.manifest && typeof e.manifest.set === "string" && e.manifest.set.length > 0)
                ? e.manifest.set : other;
            if (!byGroup.hasOwnProperty(set)) {
                byGroup[set] = [];
                order.push(set);
            }
            byGroup[set].push({
                id: "plugin:" + pid,
                label: (e.manifest && e.manifest.name) ? e.manifest.name : pid,
                icon: (e.manifest && e.manifest.defaults && e.manifest.defaults.icon)
                    ? e.manifest.defaults.icon : "widgets",
                enabled: placed.indexOf(pid) >= 0,
                group: set
            });
        }
        const out = bi.slice();
        // the vendored iRiS faces: one roster row each, on/off from widgets.json,
        // grouped so the Hub's Add drop-down keeps them together.
        const irisFaces = IrisRoster.faces;
        for (var k = 0; k < irisFaces.length; k++) {
            const f = irisFaces[k];
            out.push({ id: f.prefix, label: f.label, icon: f.icon, enabled: root.widgetValue(f.prefix, "Enabled") === true, group: "Shima widgets" });
        }
        const pythonFaces = PythonRoster.faces;
        for (var p = 0; p < pythonFaces.length; p++) {
            const f = pythonFaces[p];
            out.push({ id: f.prefix, label: f.label, icon: f.icon, enabled: root.widgetValue(f.prefix, "Enabled") === true, group: "Python widgets" });
        }
        for (var g = 0; g < order.length; g++) {
            const rows = byGroup[order[g]];
            for (var r = 0; r < rows.length; r++)
                out.push(rows[r]);
        }
        return out;
    }
    // The live WidgetSlot for a built-in or iRiS scope. Every slot rides a
    // full-screen host, so its x/y/width/height are already monitor pixels the
    // inspector can dock beside.
    function _outerFor(w) {
        switch (w) {
        case "clock": return clockLoader.item;
        case "calendar": return calendarLoader.item;
        case "music": return musicLoader.item;
        case "aio": return aioLoader.item;
        case "stats": return statsLoader.item;
        case "weather": return weatherLoader.item;
        case "notes": return notesLoader.item;
        case "dayprogress": return dayprogressLoader.item;
        case "shape": return shapeLoader.item;
        }
        for (var i = 0; i < irisRepeater.count; i++) {
            const ld = irisRepeater.itemAt(i);
            if (ld && ld.item && ld.modelData && ld.modelData.prefix === w)
                return ld.item;
        }
        for (var j = 0; j < pythonRepeater.count; j++) {
            const pd = pythonRepeater.itemAt(j);
            if (pd && pd.item && pd.modelData && pd.modelData.prefix === w)
                return pd.item;
        }
        return null;
    }
    function slotFor(w) {
        // Every visualizer instance has its own grip and footprint.
        const visualizerId = "" + w;
        if (visualizerId === "visualizer" || visualizerId.indexOf("visualizer:") === 0) {
            const index = visualizerId === "visualizer"
                ? VizCfg.Config.active : parseInt(visualizerId.slice(11));
            const grip = !isNaN(index) ? vizGripRepeater.itemAt(index) : null;
            return VizCfg.Config.enabled && grip ? grip.boxItem : null;
        }
        if (String(w).indexOf("plugin:") === 0) {
            const pluginId = String(w).slice(7);
            for (var p = 0; p < pluginRepeater.count; p++) {
                const plugin = pluginRepeater.itemAt(p);
                if (plugin && plugin.pid === pluginId)
                    return plugin;
            }
            return null;
        }
        const outer = root._outerFor(w);
        if (!outer)
            return null;
        const kids = outer.children;
        for (var i = 0; i < kids.length; i++)
            if (kids[i] && kids[i].widget === w)
                return kids[i];
        return null;
    }
    // Arm-and-open for the right-click menus: the first open builds the menu
    // synchronously, and the pending request lands the moment it is ready.
    property var pendingWidgetMenu: null
    property var pendingDesktopMenu: null
    property var pendingPluginMenu: null
    property var pendingInspector: null
    // The bare-wallpaper right-click opens the system desktop menu. While this
    // monitor is being composed, a widget goes straight to the drawer instead
    // of raising a second editing surface over the canvas.
    function openWidgetMenu(widget, x, y) {
        if (widget === "desktop") {
            root.openDesktopMenu(x, y);
            return;
        }
        if (root.stageComposing) {
            root.stageOpenSettings(widget);
            return;
        }
        Config.selectMonitor(root.monitorName, false);
        if (widgetMenuLoader.item) {
            widgetMenuLoader.item.openFor(widget, x, y, root.wallpaperPath);
            return;
        }
        root.pendingWidgetMenu = [widget, x, y];
        widgetMenuLoader.active = true;
    }
    function newDesktopFolder(x, y) {
        const folderId = StageShortcuts.DesktopShortcuts.newFolder(
            root.monitorName, "", x, y);
        if (!folderId)
            return;
        Qt.callLater(() => {
            if (desktopIcons.item)
                desktopIcons.item.openFolder(folderId, true);
        });
    }
    // Outside Edit Mode Customize keeps its lightweight floating sheet. Inside
    // the mode the drawer owns this content and retargets in place.
    function openInspector(widget) {
        if (widget === "desktop")
            return;
        if (root.stageComposing) {
            root.stageOpenSettings(widget);
            return;
        }
        Config.selectMonitor(root.monitorName, false);
        if (inspectorLoader.item) {
            inspectorLoader.item.openFor(widget, root.slotFor(widget));
            return;
        }
        root.pendingInspector = widget;
        inspectorLoader.active = true;
    }
    function openDesktopMenu(x, y) {
        if (desktopMenuLoader.item) {
            desktopMenuLoader.item.openAt(x, y);
            return;
        }
        root.pendingDesktopMenu = [x, y];
        desktopMenuLoader.active = true;
    }
    function openPluginMenu(id, locked, x, y, manifest, placement) {
        Config.selectMonitor(root.monitorName, root.stageComposing);
        if (pluginMenuLoader.item) {
            pluginMenuLoader.item.openFor(id, locked, x, y, manifest, placement, root.wallpaperPath);
            return;
        }
        root.pendingPluginMenu = [id, locked, x, y, manifest, placement];
        pluginMenuLoader.active = true;
    }

    // Any of the desktop right-click menus (bare wallpaper, a widget, a plugin
    // tile) currently on screen. Drives the dedicated menu surface below so it is
    // mapped and takes keyboard only while a menu actually shows.
    readonly property bool menusShowing:
        (widgetMenuLoader.item && widgetMenuLoader.item.showing === true)
        || (desktopMenuLoader.item && desktopMenuLoader.item.showing === true)
        || (pluginMenuLoader.item && pluginMenuLoader.item.showing === true)

    // The inspector sheet currently on screen; drives its own Overlay surface so
    // it is mapped and takes keyboard only while it shows.
    readonly property bool inspectorShowing:
        inspectorLoader.item && inspectorLoader.item.showing === true

    // Off-surface open of a widget's right-click menu (the `desktop menu` IPC /
    // niri routing). Lands near the top-left of this monitor's canvas.
    Connections {
        target: Services.ShellState
        function onWidgetMenuRequested(mon, widget) {
            if (mon !== "" && mon !== root.monitorName)
                return;
            root.openWidgetMenu(widget, 180, 140);
        }
        function onWidgetCustomizeRequested(mon, widget) {
            if (mon !== "" && mon !== root.monitorName)
                return;
            root.openInspector(widget);
        }
    }
    // A widget frame's Settings action hands the selection to the drawer. The
    // hidden widget page remembers which catalogue called it; visualizer
    // instances keep their own gallery and only retarget its active instance.
    function stageOpenSettings(id) {
        const value = String(id || "");
        if (value === "")
            return;
        Config.selectMonitor(root.monitorName, true);
        StageCfg.StageSession.select(value);
        if (value === "visualizer" || value.indexOf("visualizer:") === 0) {
            if (value.indexOf("visualizer:") === 0) {
                const index = parseInt(value.slice(11));
                if (!isNaN(index))
                    VizCfg.Config.setActive(index);
            }
            StageCfg.StageSession.inspecting = "";
            StageCfg.StageSession.openSection("visualizer");
            return;
        }
        const current = Stage.GlobalStates.editDrawerSection;
        if (current !== "widget")
            StageCfg.StageSession.inspectingBack = current || "widgets";
        StageCfg.StageSession.inspecting = value;
        StageCfg.StageSession.openSection("widget");
        if (widgetMenuLoader.item)
            widgetMenuLoader.item.close();
        if (inspectorLoader.item)
            inspectorLoader.item.close();
    }
    // The provider while the chrome frames this screen, else null: the
    // walk-back of a desktop edit is the provider's to record, since it owns
    // the snapshot/restore vocabulary for every widget kind.
    function stageProvider() {
        return (root.stageComposing && Stage.GlobalStates.editMode)
            ? StageIsland.Config.widgetProvider : null;
    }
    // A widget frame's Remove button: hide the built-in and drop any
    // selection, recorded so the chrome's undo brings it back.
    function stageRemoveWidget(id) {
        const p = root.stageProvider();
        const before = p ? p.snapshot(id) : null;
        if (p) {
            p.removeWidget(id);
        } else if (String(id).indexOf("visualizer:") === 0) {
            const index = parseInt(String(id).slice(11));
            if (!isNaN(index))
                VizCfg.Config.removeVisualizer(index);
        } else {
            Config.setFor(root.monitorName, id + "Enabled", false);
        }
        StageCfg.StageSession.remove(id);
        if (before)
            Stage.GlobalStates.editHistoryPush({
                undo: () => p.restore(id, before),
                redo: () => p.restore(id, null)
            });
    }
    // The Stage Editor's walk-back covers Ryoku's own gestures too: a slot
    // records the placement a drag or resize started from
    // (WidgetSlot.gestureBefore) and the desktop turns the commit into one
    // undo entry, so Ctrl+Z in the chrome steps a widget back exactly like the
    // reference's canvas steps its own. Placement and lock state are the whole
    // document for a built-in or a hosted face; plugin tiles go through the
    // provider's snapshot/restore, since their write is the place tool's.
    function stageRecordGesture(s) {
        // Consumed even outside the editor: a snapshot left from a desktop
        // drag would otherwise become the walk-back of the next edit.
        const before = s.gestureBefore;
        s.gestureBefore = null;
        if (!root.stageComposing || before === null)
            return;
        const w = s.widget;
        const after = {
            Anchor: root.widgetValue(w, "Anchor"), X: root.widgetValue(w, "X"),
            Y: root.widgetValue(w, "Y"), Scale: root.widgetValue(w, "Scale"),
            Locked: root.widgetValue(w, "Locked")
        };
        if (JSON.stringify(before) === JSON.stringify(after))
            return;
        Stage.GlobalStates.editHistoryPush({
            undo: () => root._stageApplyPlacement(w, before),
            redo: () => root._stageApplyPlacement(w, after)
        });
    }
    function _stageApplyPlacement(w, p) {
        const patch = {};
        patch[w + "Anchor"] = p.Anchor;
        patch[w + "X"] = p.X;
        patch[w + "Y"] = p.Y;
        patch[w + "Scale"] = p.Scale;
        if (p.Locked !== undefined)
            patch[w + "Locked"] = p.Locked;
        Config.setManyFor(root.monitorName, patch);
    }
    // Each grip snapshots its own complete instance. The map keeps wheel and
    // easing settle timers independent when two instances are touched quickly.
    property var vizGestureBefore: ({})
    function stageVizGestureStart(index) {
        const p = root.stageProvider();
        const id = "visualizer:" + index;
        const next = Object.assign({}, root.vizGestureBefore);
        next[id] = p ? p.snapshot(id) : null;
        root.vizGestureBefore = next;
    }
    function stageVizGestureFinish(index) {
        const p = root.stageProvider();
        const id = "visualizer:" + index;
        const before = root.vizGestureBefore[id];
        const next = Object.assign({}, root.vizGestureBefore);
        delete next[id];
        root.vizGestureBefore = next;
        if (p)
            p.recordVisualizer(id, before);
    }
    // A plugin gesture commits through the place tool, which is asynchronous:
    // the Registry still holds the old placement the moment the gesture
    // releases, and that is the before-state. Undo restores it; redo queues
    // the same command back through the provider's serial queue.
    function stageRecordPluginGesture(pid, cmd) {
        const p = root.stageProvider();
        if (!p)
            return;
        const before = p.snapshot("plugin:" + pid);
        if (!before)
            return;
        Stage.GlobalStates.editHistoryPush({
            undo: () => p.restore("plugin:" + pid, before),
            redo: () => p.enqueue(cmd)
        });
    }
    function _runSettingsQueue() {
        if (settingsProc.running)
            return;
        if (!root._settingsQueue || root._settingsQueue.length === 0)
            return;
        settingsProc.command = root._settingsQueue.shift();
        settingsProc.running = true;
    }
    // Gate every widget's visibility on the layer being sized and Config +
    // Registry loaded, so nothing flashes at its default before the real
    // enabled flags arrive. An undefined here would short-circuit the visible
    // bindings to undefined and leave them at their `true` default, rendering
    // disabled widgets.
    readonly property bool reloadReady: readiness.ready

    ReloadReadiness {
        id: readiness
        width: win.width
        height: win.height
        configReady: Config.ready
        registryReady: Registry.ready
    }

    // Resolve the placement helper the way Registry resolves discover.sh: in a
    // dev run RYOKU_SHELL_DIR points at the shell tree and the tool is NOT on
    // PATH, so a bare "ryoku-plugins-place" Process silently no-ops and every
    // drag / resize / lock / hide / settings write is lost. Packaged installs
    // (no RYOKU_SHELL_DIR) ship it to /usr/bin, where the bare name resolves.
    readonly property string _shellDir: Quickshell.env("RYOKU_SHELL_DIR")
    readonly property string placeTool: (root._shellDir && root._shellDir.length > 0)
        ? root._shellDir + "/quickshell/plugins/ryoku-plugins-place"
        : "ryoku-plugins-place"

    // hand the keyboard back after a widget text field releases its grab. the
    // widget layer never unmaps, and dropping an exclusive grab on a mapped
    // layer strands the keyboard (the focused app can't type). this 1x1 helper
    // takes the grab and unmaps, which hands the keyboard to a real window.
    // same mechanism as the pill's kbBounce.
    property bool kbBounce: false
    function kbRestore() {
        root.kbBounce = true;
        kbBounceT.restart();
    }
    Timer {
        id: kbBounceT
        interval: 90
        onTriggered: root.kbBounce = false
    }
    PanelWindow {
        visible: root.kbBounce
        screen: root.screen
        implicitWidth: 1
        implicitHeight: 1
        color: "transparent"
        exclusionMode: ExclusionMode.Ignore
        WlrLayershell.layer: WlrLayer.Overlay
        WlrLayershell.namespace: "widgets-kbbounce"
        WlrLayershell.keyboardFocus: WlrKeyboardFocus.Exclusive
        anchors { top: true; left: true }
    }

    PanelWindow {
        id: win

        screen: root.screen
        visible: root.active
        color: "transparent"
        exclusionMode: ExclusionMode.Ignore
        // Editing lifts the desktop above open windows so the stage is never
        // obscured by whatever was in front; Done drops it back under them.
        WlrLayershell.layer: root.stageComposing ? WlrLayer.Top : WlrLayer.Bottom
        WlrLayershell.namespace: "ryoku-widgets"
        // Text fields and icon dialogs keep their existing counter. The editor
        // borrows the grab only while no other surface or desktop control is
        // asking for keys, so opening a search or inspector never loses its caret.
        // Where a grab would also take every click, the editor asks on demand
        // instead: the compositor hands it the keyboard as the pointer crosses
        // the desktop, and the toolbar and catalogue on their own surface stay
        // clickable (keyboardGrabSharesPointer in docs/compositors.md).
        property int kbWanted: 0
        readonly property bool stageKeyboardWanted: root.stageComposing
            && kbWanted === 0
            && !root.menusShowing
            && !root.inspectorShowing
            && !Stage.GlobalStates.editSearchFocused
        onKbWantedChanged: if (kbWanted === 0 && !stageKeyboardWanted) root.kbRestore()
        onStageKeyboardWantedChanged: if (!stageKeyboardWanted && kbWanted === 0) root.kbRestore()
        WlrLayershell.keyboardFocus: kbWanted > 0 ? WlrKeyboardFocus.Exclusive
            : !stageKeyboardWanted ? WlrKeyboardFocus.None
            : Wm.caps.keyboardGrabSharesPointer === true ? WlrKeyboardFocus.Exclusive
            : WlrKeyboardFocus.OnDemand

        anchors { top: true; left: true; right: true; bottom: true }


        // enabled desktopWidget-hosted plugins, filtered from the shared
        // Registry. drives the Repeater below so plugin tiles ride the
        // SAME wallpaper layer as the clock: one layer, one input
        // model, no second full-screen surface fighting for input.
        readonly property var desktopPlugins: Registry.plugins.filter(p => p.placement && p.placement.host === "desktopWidget")

        // current persisted desktopWidget block by id. menu handlers use
        // it to round-trip a partial change (lock/scale) without dropping
        // the other coordinates.
        function placementOf(id) {
            const p = win.desktopPlugins.find(pp => pp.id === id);
            return (p && p.placement && p.placement.desktopWidget) || {};
        }

        // the Repeater below is keyed by this id list, not by `desktopPlugins`
        // directly: any placement write (drag, resize, settings) rewrites
        // plugins.json and Registry reparses to a brand-new array, so binding
        // the Repeater to that array would tear down and rebuild every tile -
        // and its service - on every move, throwing away the open search and
        // page. the id list only changes when a plugin is enabled or
        // disabled, so moving a tile keeps its delegate and its live service.
        property var desktopPluginIds: []
        function syncDesktopIds() {
            const ids = win.desktopPlugins.map(p => p.id);
            const same = ids.length === win.desktopPluginIds.length
                && ids.every((id, i) => id === win.desktopPluginIds[i]);
            if (!same)
                win.desktopPluginIds = ids;
        }
        Component.onCompleted: win.syncDesktopIds()
        Connections {
            target: Registry
            function onPluginsChanged() { win.syncDesktopIds(); }
        }


        // on release, flash the slot's four edges plus the centre line it snapped
        // to (centre within half a grid step, the window the guides light up).
        function flashDrop(box) {
            const v = [box.x, box.x + box.width];
            const h = [box.y, box.y + box.height];
            const gs = guides.gridSize;
            if (Math.abs(box.x + box.width / 2 - guides.width / 2) < gs / 2)
                v.push(guides.width / 2);
            if (Math.abs(box.y + box.height / 2 - guides.height / 2) < gs / 2)
                h.push(guides.height / 2);
            guides.flash(v, h);
        }

        // The Stage Editor's card: every visible layer of the desktop (wallpaper,
        // stage, widgets, editor frames) in one container that carries the mode's
        // shrink matrix, so the desktop becomes the object the chrome frames and
        // the widgets under the cursor scale together. Identity when not editing.
        Item {
            id: desktopContent
            anchors.fill: parent
            transform: Matrix4x4 { matrix: root.stageMatrix }

        // The base wallpaper painter: the reveal backdrop composites each new
        // frame over the old one through the preset the daemon attached to the
        // frame (a GPU mask shader), decoding capped at surface resolution.
        // Live clips play inside this same surface (QtMultimedia + optional
        // interpolation), so the backdrop no longer yields to a second Wayland
        // client: it keeps the still decoded and swaps to the video itself.
        WallpaperMod.Backdrop {
            id: backdrop
            anchors.fill: parent
            // The base wallpaper draws under the stage (Parallax's StageBackdrop
            // covers its baked subject, Depth locks the still cut over it), and
            // yields to the ryogami-live player on its own `live` rule: forcing
            // it visible painted the still over every video wallpaper.
            readonly property real screenDpr: (root.screen && root.screen.devicePixelRatio) ? root.screen.devicePixelRatio : 1
            screenName: root.monitorName
            wallpaperPath: root.wallpaperPath
            dpr: screenDpr
            // The provider frame is authoritative per output; its poster stays
            // decoded while video plays so the next reveal has a valid base.
            url: root.wallpaperUrl
            fit: root.wallpaperFit
            transition: root.wallpaperTransition
            videoUrl: root.videoUrl
            live: root.wallpaperLive
            videoMuted: root.videoMuted
            videoVolume: root.videoVolume
            videoPaused: root.videoPaused
        }

        // The Parallax backdrop: the inpainted background.png drifting just
        // above the base wallpaper, covering its baked subject. Only Parallax
        // shows it; it also owns the per-monitor cursor poll (docs/stage.md).
        StageBackdrop {
            z: 1
            screen: root.screen
            wallpaperPath: root.wallpaperPath
            wallpaperFit: root.wallpaperFit
            visible: root.stageParallax
        }

        // While the stage is on (or the Stage Editor frames this monitor), the
        // visualizer lives inside this surface. By default it sits behind every
        // cut-out (above the backdrop, below every layer and widget); the Depth
        // row lifts it above the in-front layers, the way a lifted built-in
        // widget rises to z 5. Its own surface (a sibling window that can never
        // interleave with the subject) is suppressed meanwhile; Above windows
        // outside the editor keep that surface (docs/stage.md).
        Item {
            id: vizHost
            z: StageCfg.Config.isFront("visualizer") ? 5 : 1.5
            anchors.fill: parent
            visible: root.hostsVisualizer
            opacity: 1 - 0.75 * root.stageFramingDim
            Viz.InlineVisualizer { anchors.fill: parent }
        }

        // One grip and one frame per rendered instance. Selecting either makes
        // that instance the catalogue target before any gesture writes.
        Repeater {
            id: vizGripRepeater
            model: VizCfg.Config.enabled ? VizCfg.Config.count : 0
            delegate: Viz.VisualizerGrip {
                required property int index
                z: 5.6
                instanceIndex: index
                composing: root.stageComposing
                    && (Stage.GlobalStates.editDrawerSection === "widgets"
                        || Stage.GlobalStates.editDrawerSection === "visualizer")
                stageController: stageSelection
                enabled: !root.stageFramingHere
                opacity: 1 - 0.75 * root.stageFramingDim
                onActivated: {
                    VizCfg.Config.setActive(index);
                    StageCfg.StageSession.select("visualizer:" + index);
                }
                onGestureStarted: root.stageVizGestureStart(index)
                onGestureFinished: root.stageVizGestureFinish(index)
                onMenuRequested: root.stageOpenSettings("visualizer:" + index)
            }
        }

        Repeater {
            model: VizCfg.Config.enabled ? VizCfg.Config.count : 0
            delegate: StageOutline {
                required property int index
                readonly property string instanceId: "visualizer:" + index
                // Both repeaters follow the same model, in no guaranteed order:
                // re-resolve when the grips' count changes, not once at creation.
                readonly property var gripItem: vizGripRepeater.count > index
                    ? vizGripRepeater.itemAt(index) : null
                readonly property var slotItem: gripItem ? gripItem.boxItem : null
                z: 60
                opacity: 1 - 0.75 * root.stageFramingDim
                visible: root.stageComposing && slotItem !== null
                    && (Stage.GlobalStates.editDrawerSection === "widgets"
                        || Stage.GlobalStates.editDrawerSection === "visualizer")
                box: slotItem ? Qt.rect(slotItem.x, slotItem.y,
                    slotItem.width, slotItem.height) : Qt.rect(0, 0, 0, 0)
                title: I18n.tr("Visualizer") + " " + (index + 1)
                selected: StageCfg.StageSession.contains(instanceId)
                primary: StageCfg.StageSession.selected === instanceId
                targetItem: gripItem
                inputBlocked: root.stageFramingHere
                counterScale: 1 / Math.max(0.05,
                    StageEdit.EditModeInsets.cardRectFor(root.monitorName,
                        win.width, win.height,
                        Stage.GlobalStates.editProgress,
                        Stage.GlobalStates.editDrawerProgress).width / Math.max(1, win.width))
                onPicked: modifiers => {
                    VizCfg.Config.setActive(index);
                    stageSelection.pick(instanceId, modifiers);
                }
                onSettings: root.stageOpenSettings(instanceId)
                onRemove: root.stageRemoveWidget(instanceId)
            }
        }

        // Mirror of the same image for glass widgets: Qt cannot sample another
        // scene graph, so ShaderEffectSource captures the pixels beneath a
        // frosted widget from this offscreen copy. WidgetGlass hides this
        // source after taking its crop. Hidden while a video plays: the glass
        // samples the still, and a live clip would freeze the capture.
        readonly property bool glassWanted: root.widgetsEnabled && root.videoUrl === ""
            && ((root.widgetValue("calendar", "Enabled") === true && Config.calendarStyle === "glass")
                || (root.widgetValue("music", "Enabled") === true && Config.musicStyle === "glass"))
        Image {
            id: glassBackdrop
            anchors.fill: parent
            // An invisible Image still decodes while its source is set: hold the
            // url back until a glass widget actually samples it, or this mirror
            // costs a full-screen decode on every box that has a wallpaper.
            source: desktopContent.glassWanted ? root.wallpaperUrl : ""
            cache: false
            asynchronous: true
            sourceSize.width: Math.ceil(width * backdrop.screenDpr)
            sourceSize.height: Math.ceil(height * backdrop.screenDpr)
            visible: desktopContent.glassWanted
            fillMode: {
                switch (root.wallpaperFit) {
                case "Contain": return Image.PreserveAspectFit;
                case "Fill": return Image.Stretch;
                case "ScaleDown":
                    return sourceSize.width <= width && sourceSize.height <= height
                        ? Image.Pad : Image.PreserveAspectFit;
                default: return Image.PreserveAspectCrop;
                }
            }
        }

        StageSelection {
            id: stageSelection
            anchors.fill: parent
            active: root.stageComposing
            keyboardEnabled: win.stageKeyboardWanted
            framingDim: root.stageFramingDim
            inputBlocked: root.stageFramingHere
            monitor: root.monitorName
            gridSize: root.editGridSize
            snapEnabled: root.editGridSnap
            widgetIds: root.addItems.filter(row => row.enabled
                && row.id !== "visualizer").map(row => row.id).concat(
                    VizCfg.Config.enabled
                        ? Array.from({ length: VizCfg.Config.count },
                            (_, i) => "visualizer:" + i) : [])
            itemFor: id => root.slotFor(id)
            provider: StageIsland.Config.widgetProvider
            onRemoveRequested: id => root.stageRemoveWidget(id)
            onDropped: box => win.flashDrop(box)
            onSettingsRequested: id => root.stageOpenSettings(id)
        }


        // right-click empty desktop = global menu. sits behind the widgets
        // (which own their own right-click) and only takes RightButton, so
        // left-clicks on wallpaper fall through instead of being silently
        // swallowed.
        MouseArea {
            anchors.fill: parent
            acceptedButtons: Qt.RightButton
            onPressed: mouse => {
                if (desktopIcons.item
                    && desktopIcons.item.openContextAt(mouse.x, mouse.y))
                    return;
                root.openWidgetMenu("desktop", mouse.x, mouse.y);
            }
        }
        // A settled band replaces both selections; Shift/Ctrl preserves their
        // existing members. A zero-size plain band is the click-away deselect.
        MouseArea {
            id: widgetMarquee
            anchors.fill: parent
            enabled: stageSelection.selectionEnabled && !root.stageFramingHere
            acceptedButtons: Qt.LeftButton
            onPressed: mouse => {
                if (stageSelection.widgetPressActive) {
                    mouse.accepted = false;
                    return;
                }
                StageCfg.StageSession.closePanel();
                stageSelection.beginMarquee(mouse.x, mouse.y, mouse.modifiers);
            }
            onPositionChanged: mouse =>
                stageSelection.updateMarquee(mouse.x, mouse.y)
            onReleased: mouse => stageSelection.finishMarquee(mouse.modifiers)
            onCanceled: stageSelection.cancelMarquee()

        }

        // click-off for the notes pad: while it holds the keyboard, a press on
        // the bare wallpaper must drop its focus (and the layer's exclusive
        // grab). taking active focus here blurs the pad; it sits above the
        // right-click catcher but below every widget, so a press on a widget
        // still reaches that widget, and it passes the event on (accepted =
        // false) so the right-click desktop menu still opens.
        MouseArea {
            id: notesBlur
            anchors.fill: parent
            enabled: notesLoader.item ? notesLoader.item.editing : false
            acceptedButtons: Qt.LeftButton | Qt.RightButton
            onPressed: (mouse) => { notesBlur.forceActiveFocus(); mouse.accepted = false; }
        }

        DesktopGuides {
            id: guides
            anchors.fill: parent
            active: stageSelection.dragging && !root.stageFramingHere
            showGrid: root.editGridSnap
            gridSize: root.editGridSize
            verticals: stageSelection.guideVerticals
            horizontals: stageSelection.guideHorizontals
        }

        Loader {
            id: clockLoader
            anchors.fill: parent
            z: root.widgetZ("clock")
            active: root.widgetsEnabled && root.reloadReady && root.widgetValue("clock", "Enabled")
            sourceComponent: Component {
            Item {
                anchors.fill: parent
            WidgetSlot {
                id: clockSlot
                widget: "clock"
                monitor: root.monitorName
                z: root.widgetZ("clock")
                visible: true
                anchor: root.widgetValue("clock", "Anchor")
                freeX: root.widgetValue("clock", "X")
                freeY: root.widgetValue("clock", "Y")
                locked: root.widgetValue("clock", "Locked")
                composing: root.stageComposing
                stageController: stageSelection
                gridSize: root.editGridSize
                snapEnabled: root.editGridSnap
                bg: Config.clockBg
                radius: Config.clockRadius
                scaleCfg: root.widgetValue("clock", "Scale")
                pad: Config.clockBg === "none"
                    ? 0 : Math.round(24 * root.widgetValue("clock", "Scale"))
                onMenuRequested: (x, y, w) => root.openWidgetMenu(w, x, y)
                onDropped: (box) => { win.flashDrop(box); root.stageRecordGesture(clockSlot); }
                onResized: root.stageRecordGesture(clockSlot)
                Clock { s: root.widgetValue("clock", "Scale") }
            }
            }
            }
        }

        Loader {
            id: calendarLoader
            anchors.fill: parent
            z: root.widgetZ("calendar")
            active: root.widgetsEnabled && root.reloadReady && root.widgetValue("calendar", "Enabled")
            sourceComponent: Component {
            Item {
                anchors.fill: parent
            WidgetSlot {
                id: calendarSlot
                widget: "calendar"
                monitor: root.monitorName
                z: root.widgetZ("calendar")
                visible: true
                anchor: root.widgetValue("calendar", "Anchor")
                freeX: root.widgetValue("calendar", "X")
                freeY: root.widgetValue("calendar", "Y")
                locked: root.widgetValue("calendar", "Locked")
                composing: root.stageComposing
                stageController: stageSelection
                gridSize: root.editGridSize
                snapEnabled: root.editGridSnap
                bg: "none"
                scaleCfg: root.widgetValue("calendar", "Scale")
                onMenuRequested: (x, y, w) => root.openWidgetMenu(w, x, y)
                onDropped: (box) => { win.flashDrop(box); root.stageRecordGesture(calendarSlot); }
                onResized: root.stageRecordGesture(calendarSlot)
                CalendarWidget {
                    style: Config.calendarStyle
                    weeks: Config.calendarWeeks
                    showWeekNumbers: Config.calendarWeekNumbers
                    holidayRegion: Config.calendarHolidayRegion
                    active: calendarSlot.visible
                    s: root.widgetValue("calendar", "Scale")
                    wallpaperSource: glassBackdrop
                    wallpaperRect: Qt.rect(calendarSlot.x, calendarSlot.y,
                        calendarSlot.width, calendarSlot.height)
                }
            }
            }
            }
        }

        Loader {
            id: musicLoader
            anchors.fill: parent
            z: root.widgetZ("music")
            active: root.widgetsEnabled && root.reloadReady && root.widgetValue("music", "Enabled")
            sourceComponent: Component {
            Item {
                anchors.fill: parent
            WidgetSlot {
                id: musicSlot
                widget: "music"
                monitor: root.monitorName
                z: root.widgetZ("music")
                visible: true
                anchor: root.widgetValue("music", "Anchor")
                freeX: root.widgetValue("music", "X")
                freeY: root.widgetValue("music", "Y")
                locked: root.widgetValue("music", "Locked")
                composing: root.stageComposing
                stageController: stageSelection
                gridSize: root.editGridSize
                snapEnabled: root.editGridSnap
                bg: "none"
                scaleCfg: root.widgetValue("music", "Scale")
                onMenuRequested: (x, y, w) => root.openWidgetMenu(w, x, y)
                onDropped: (box) => { win.flashDrop(box); root.stageRecordGesture(musicSlot); }
                onResized: root.stageRecordGesture(musicSlot)
                MusicWidget {
                    style: Config.musicStyle
                    showLyrics: Config.musicLyrics
                    viz: Config.musicViz
                    active: musicSlot.visible
                    musicApp: Config.musicApp
                    shape: Config.musicShape
                    videoMode: Config.musicVideo
                    videoFile: Config.musicVideoFile
                    s: root.widgetValue("music", "Scale")
                    wallpaperSource: glassBackdrop
                    wallpaperRect: Qt.rect(musicSlot.x, musicSlot.y,
                        musicSlot.width, musicSlot.height)
                }
            }
            }
            }
        }

        Loader {
            id: aioLoader
            anchors.fill: parent
            z: root.widgetZ("aio")
            active: root.widgetsEnabled && root.reloadReady && root.widgetValue("aio", "Enabled")
            sourceComponent: Component {
            Item {
                anchors.fill: parent
            WidgetSlot {
                id: aioSlot
                widget: "aio"
                monitor: root.monitorName
                z: root.widgetZ("aio")
                visible: true
                anchor: root.widgetValue("aio", "Anchor")
                freeX: root.widgetValue("aio", "X")
                freeY: root.widgetValue("aio", "Y")
                locked: root.widgetValue("aio", "Locked")
                composing: root.stageComposing
                stageController: stageSelection
                gridSize: root.editGridSize
                snapEnabled: root.editGridSnap
                bg: "none"
                scaleCfg: root.widgetValue("aio", "Scale")
                onMenuRequested: (x, y, w) => root.openWidgetMenu(w, x, y)
                onDropped: (box) => { win.flashDrop(box); root.stageRecordGesture(aioSlot); }
                onResized: root.stageRecordGesture(aioSlot)
                AioWidget {
                    style: Config.aioStyle
                    s: root.widgetValue("aio", "Scale")
                    active: aioSlot.visible
                }
            }
            }
            }
        }

        Loader {
            id: statsLoader
            anchors.fill: parent
            z: root.widgetZ("stats")
            active: root.widgetsEnabled && root.reloadReady && root.widgetValue("stats", "Enabled")
            sourceComponent: Component {
            Item {
                anchors.fill: parent
            WidgetSlot {
                id: statsSlot
                widget: "stats"
                monitor: root.monitorName
                z: root.widgetZ("stats")
                visible: true
                anchor: root.widgetValue("stats", "Anchor")
                freeX: root.widgetValue("stats", "X")
                freeY: root.widgetValue("stats", "Y")
                locked: root.widgetValue("stats", "Locked")
                composing: root.stageComposing
                stageController: stageSelection
                gridSize: root.editGridSize
                snapEnabled: root.editGridSnap
                bg: "none"
                scaleCfg: root.widgetValue("stats", "Scale")
                onMenuRequested: (x, y, w) => root.openWidgetMenu(w, x, y)
                onDropped: (box) => { win.flashDrop(box); root.stageRecordGesture(statsSlot); }
                onResized: root.stageRecordGesture(statsSlot)
                StatsWidget {
                    s: root.widgetValue("stats", "Scale")
                    active: statsSlot.visible
                }
            }
            }
            }
        }

        Loader {
            id: weatherLoader
            anchors.fill: parent
            z: root.widgetZ("weather")
            active: root.widgetsEnabled && root.reloadReady && root.widgetValue("weather", "Enabled")
            sourceComponent: Component {
            Item {
                anchors.fill: parent
            WidgetSlot {
                id: weatherSlot
                widget: "weather"
                monitor: root.monitorName
                z: root.widgetZ("weather")
                visible: true
                anchor: root.widgetValue("weather", "Anchor")
                freeX: root.widgetValue("weather", "X")
                freeY: root.widgetValue("weather", "Y")
                locked: root.widgetValue("weather", "Locked")
                composing: root.stageComposing
                stageController: stageSelection
                gridSize: root.editGridSize
                snapEnabled: root.editGridSnap
                bg: "none"
                scaleCfg: root.widgetValue("weather", "Scale")
                onMenuRequested: (x, y, w) => root.openWidgetMenu(w, x, y)
                onDropped: (box) => { win.flashDrop(box); root.stageRecordGesture(weatherSlot); }
                onResized: root.stageRecordGesture(weatherSlot)
                WeatherWidget {
                    design: Config.weatherDesign
                    s: root.widgetValue("weather", "Scale")
                    active: weatherSlot.visible
                }
            }
            }
            }
        }

        Loader {
            id: notesLoader
            anchors.fill: parent
            z: root.widgetZ("notes")
            active: root.widgetsEnabled && root.reloadReady && root.widgetValue("notes", "Enabled")
            sourceComponent: Component {
            Item {
                anchors.fill: parent
            WidgetSlot {
                id: notesSlot
                widget: "notes"
                monitor: root.monitorName
                z: root.widgetZ("notes")
                visible: true
                anchor: root.widgetValue("notes", "Anchor")
                freeX: root.widgetValue("notes", "X")
                freeY: root.widgetValue("notes", "Y")
                locked: root.widgetValue("notes", "Locked")
                composing: root.stageComposing
                stageController: stageSelection
                gridSize: root.editGridSize
                snapEnabled: root.editGridSnap
                bg: "none"
                scaleCfg: root.widgetValue("notes", "Scale")
                onMenuRequested: (x, y, w) => root.openWidgetMenu(w, x, y)
                onDropped: (box) => { win.flashDrop(box); root.stageRecordGesture(notesSlot); }
                onResized: root.stageRecordGesture(notesSlot)
                // notes is the first built-in editable widget: while its pad holds
                // focus the layer must grab the keyboard (bump kbWanted), and drop
                // the grab the instant it blurs, or the desktop is stranded.
                onEditingChanged: win.kbWanted += editing ? 1 : -1
                Component.onDestruction: if (editing) win.kbWanted -= 1
                NotesWidget {
                    s: root.widgetValue("notes", "Scale")
                    active: notesSlot.visible
                    wLogical: root.widgetValue("notes", "Width")
                    hLogical: root.widgetValue("notes", "Height")
                }
            }
            }
            }
        }

        Loader {
            id: dayprogressLoader
            anchors.fill: parent
            z: root.widgetZ("dayprogress")
            active: root.widgetsEnabled && root.reloadReady && root.widgetValue("dayprogress", "Enabled")
            sourceComponent: Component {
            Item {
                anchors.fill: parent
            WidgetSlot {
                id: dayprogressSlot
                widget: "dayprogress"
                monitor: root.monitorName
                z: root.widgetZ("dayprogress")
                visible: true
                anchor: root.widgetValue("dayprogress", "Anchor")
                freeX: root.widgetValue("dayprogress", "X")
                freeY: root.widgetValue("dayprogress", "Y")
                locked: root.widgetValue("dayprogress", "Locked")
                composing: root.stageComposing
                stageController: stageSelection
                gridSize: root.editGridSize
                snapEnabled: root.editGridSnap
                bg: "none"
                scaleCfg: root.widgetValue("dayprogress", "Scale")
                onMenuRequested: (x, y, w) => root.openWidgetMenu(w, x, y)
                onDropped: (box) => { win.flashDrop(box); root.stageRecordGesture(dayprogressSlot); }
                onResized: root.stageRecordGesture(dayprogressSlot)
                DayProgressWidget {
                    s: root.widgetValue("dayprogress", "Scale")
                }
            }
            }
            }
        }

        Loader {
            id: shapeLoader
            anchors.fill: parent
            z: root.widgetZ("shape")
            active: root.widgetsEnabled && root.reloadReady && root.widgetValue("shape", "Enabled")
            sourceComponent: Component {
            Item {
                anchors.fill: parent
            WidgetSlot {
                id: shapeSlot
                widget: "shape"
                monitor: root.monitorName
                z: root.widgetZ("shape")
                visible: true
                anchor: root.widgetValue("shape", "Anchor")
                freeX: root.widgetValue("shape", "X")
                freeY: root.widgetValue("shape", "Y")
                locked: root.widgetValue("shape", "Locked")
                composing: root.stageComposing
                stageController: stageSelection
                gridSize: root.editGridSize
                snapEnabled: root.editGridSnap
                bg: "none"
                scaleCfg: root.widgetValue("shape", "Scale")
                onMenuRequested: (x, y, w) => root.openWidgetMenu(w, x, y)
                onDropped: (box) => { win.flashDrop(box); root.stageRecordGesture(shapeSlot); }
                onResized: root.stageRecordGesture(shapeSlot)
                ShapeWidget {
                    s: root.widgetValue("shape", "Scale")
                }
            }
            }
            }
        }

        // one IrisFaceWidget slot per enabled iRiS face. Same host contract as
        // the built-ins -- WidgetSlot owns placement/size/lock/backing and the
        // colour modes; the adapter renders the vendored inir face in Ryoku's
        // look. Driven by the shared roster so a face is registered once.
        Repeater {
            id: irisRepeater
            model: IrisRoster.faces
            delegate: Loader {
                id: irisLoader
                required property var modelData
                anchors.fill: parent
                z: root.widgetZ(irisLoader.modelData.prefix)
                active: root.widgetsEnabled && root.reloadReady
                    && (root.widgetValue(irisLoader.modelData.prefix, "Enabled") === true)
                sourceComponent: Component {
                    Item {
                        anchors.fill: parent
                        WidgetSlot {
                            id: irisSlot
                            widget: irisLoader.modelData.prefix
                            monitor: root.monitorName
                            z: root.widgetZ(irisLoader.modelData.prefix)
                            visible: true
                            anchor: root.widgetValue(irisLoader.modelData.prefix, "Anchor")
                            freeX: root.widgetValue(irisLoader.modelData.prefix, "X")
                            freeY: root.widgetValue(irisLoader.modelData.prefix, "Y")
                            locked: root.widgetValue(irisLoader.modelData.prefix, "Locked")
                            composing: root.stageComposing
                            stageController: stageSelection
                            gridSize: root.editGridSize
                            snapEnabled: root.editGridSnap
                            // iNiR style: the face owns its plate, so the slot draws no
                            // backing. Ryoku style: the slot draws the chosen backing.
                            bg: Config[irisLoader.modelData.prefix + "Style"] === "ryoku"
                                ? Config[irisLoader.modelData.prefix + "Bg"] : "none"
                            scaleCfg: root.widgetValue(irisLoader.modelData.prefix, "Scale")
                            // per-widget geometry (Ryoku-style backing / iNiR plate radius)
                            radiusOverride: Config[irisLoader.modelData.prefix + "Radius"]
                            pad: Config[irisLoader.modelData.prefix + "Pad"] >= 0
                                ? Config[irisLoader.modelData.prefix + "Pad"] : 0
                            borderWidth: Config[irisLoader.modelData.prefix + "Border"]
                            borderOpacity: Config[irisLoader.modelData.prefix + "BorderOpacity"]
                            backingOpacity: Config[irisLoader.modelData.prefix + "BackingOpacity"]
                            onMenuRequested: (x, y, w) => root.openWidgetMenu(w, x, y)
                            onDropped: (box) => { win.flashDrop(box); root.stageRecordGesture(irisSlot); }
                            onResized: root.stageRecordGesture(irisSlot)
                            IrisFaceWidget {
                                faceId: irisLoader.modelData.id
                                kind: irisLoader.modelData.kind
                                prefix: irisLoader.modelData.prefix
                                scaleCfg: root.widgetValue(irisLoader.modelData.prefix, "Scale")
                                sizeCfg: root.widgetValue(irisLoader.modelData.prefix, "Size")
                                screen: root.screen
                                hostX: irisSlot.x
                                hostY: irisSlot.y
                            }
                        }
                    }
                }
            }
        }

        // Serpantinum's face library uses the same global slot contract: Ryoku
        // owns placement and chrome while the selected Python face paints inside.
        Repeater {
            id: pythonRepeater
            model: PythonRoster.faces
            delegate: Loader {
                id: pythonLoader
                required property var modelData
                anchors.fill: parent
                z: root.widgetZ(pythonLoader.modelData.prefix)
                active: root.widgetsEnabled && root.reloadReady
                    && (root.widgetValue(pythonLoader.modelData.prefix, "Enabled") === true)
                sourceComponent: Component {
                    Item {
                        anchors.fill: parent
                        WidgetSlot {
                            id: pythonSlot
                            widget: pythonLoader.modelData.prefix
                            monitor: root.monitorName
                            z: root.widgetZ(pythonLoader.modelData.prefix)
                            visible: true
                            anchor: root.widgetValue(pythonLoader.modelData.prefix, "Anchor")
                            freeX: root.widgetValue(pythonLoader.modelData.prefix, "X")
                            freeY: root.widgetValue(pythonLoader.modelData.prefix, "Y")
                            locked: root.widgetValue(pythonLoader.modelData.prefix, "Locked")
                            composing: root.stageComposing
                            stageController: stageSelection
                            gridSize: root.editGridSize
                            snapEnabled: root.editGridSnap
                            bg: Config[pythonLoader.modelData.prefix + "Style"] === "ryoku"
                                ? Config[pythonLoader.modelData.prefix + "Bg"] : "none"
                            scaleCfg: root.widgetValue(pythonLoader.modelData.prefix, "Scale")
                            radiusOverride: Config[pythonLoader.modelData.prefix + "Radius"]
                            pad: Config[pythonLoader.modelData.prefix + "Pad"] >= 0
                                ? Config[pythonLoader.modelData.prefix + "Pad"] : 0
                            borderWidth: Config[pythonLoader.modelData.prefix + "Border"]
                            borderOpacity: Config[pythonLoader.modelData.prefix + "BorderOpacity"]
                            backingOpacity: Config[pythonLoader.modelData.prefix + "BackingOpacity"]
                            onMenuRequested: (x, y, w) => root.openWidgetMenu(w, x, y)
                            onDropped: (box) => { win.flashDrop(box); root.stageRecordGesture(pythonSlot); }
                            onResized: root.stageRecordGesture(pythonSlot)
                            PythonFaceWidget {
                                faceId: pythonLoader.modelData.id
                                scaleCfg: root.widgetValue(pythonLoader.modelData.prefix, "Scale")
                                prefix: pythonLoader.modelData.prefix
                                screen: root.screen
                            }
                        }
                    }
                }
            }
        }

        // one draggable PluginDesktopSlot per enabled desktopWidget plugin.
        // drag = write free pos. resize bracket = write scale. right-click
        // = per-tile menu. each commit goes through its own Process so a
        // Lock right after a drag can't stomp an in-flight write on
        // `persist`.
        Repeater {
            id: pluginRepeater
            model: root.widgetsEnabled ? win.desktopPluginIds : []
            delegate: PluginDesktopSlot {
                id: slot
                required property string modelData
                readonly property string pid: modelData
                readonly property string stageId: "plugin:" + slot.pid
                // Depth lift: a lifted tile rises above the in-front cut-outs
                // the way a lifted built-in does (docs/stage.md).
                z: root.widgetZ(slot.pid)
                // live registry entry for this id, re-resolved whenever
                // Registry reloads. placement (x/y/scale/bg) updates here
                // without rebuilding the delegate, because the model is the
                // stable id list, not the per-write plugin array.
                readonly property var entry: Registry.plugins.find(p => p.id === slot.pid) || null
                readonly property var dw: (entry && entry.placement && entry.placement.desktopWidget) || ({})
                readonly property bool stagePreviewSettled:
                    stageSelection.hasPluginPreview(slot.stageId)
                    && Math.round(slot.dw.x) === Math.round(stageSelection.pluginPreview(slot.stageId).x)
                    && Math.round(slot.dw.y) === Math.round(stageSelection.pluginPreview(slot.stageId).y)
                // host-supplied accent (the matugen parity): a plugin that
                // declares capabilities.colors gets an accent pushed in the way
                // built-ins do -- Auto (the palette accent), a pinned hex, or off
                // (its own palette). settings.colorAuto (default true) and
                // settings.color ("" = auto) choose; the props land on the
                // content by Binding, since a settings write reparses Registry
                // without rebuilding the content.
                readonly property var pset: (entry && entry.placement && entry.placement.settings) || ({})
                readonly property bool colorsCap: !!(entry && entry.manifest && entry.manifest.capabilities
                    && entry.manifest.capabilities.colors === true)
                readonly property bool colorAuto: slot.pset.colorAuto !== false
                readonly property string pinnedColor: (typeof slot.pset.color === "string") ? slot.pset.color : ""
                readonly property color hostAccent: (!slot.colorAuto && slot.pinnedColor.length > 0)
                    ? slot.pinnedColor : Scheme.accent
                readonly property bool hostAccentOn: slot.colorsCap && (slot.colorAuto || slot.pinnedColor.length > 0)
                readonly property string dir: entry ? entry.dir : ""
                readonly property string versionQuery: entry && entry.version
                    ? "?v=" + encodeURIComponent(entry.version) : ""

                pluginId: slot.pid
                visible: root.reloadReady
                locked: slot.dw.locked === true
                composing: root.stageComposing
                stageController: stageSelection
                gridSize: root.editGridSnap ? root.editGridSize : 1
                scaleCfg: slot.dw.scale || 0.85
                opacityCfg: slot.dw.opacity !== undefined ? slot.dw.opacity : 1
                freeX: slot.dw.x !== undefined ? slot.dw.x : 80
                freeY: slot.dw.y !== undefined ? slot.dw.y : 80
                bg: slot.dw.bg ? slot.dw.bg : ((entry && entry.manifest && entry.manifest.defaults && entry.manifest.defaults.desktopWidget && entry.manifest.defaults.desktopWidget.bg) || "card")
                radius: slot.dw.radius || 26

                onMoved: (x, y) => {
                    const cmd = [root.placeTool, slot.pid, "desktopWidget",
                        "" + x, "" + y];
                    root.stageRecordPluginGesture(slot.pid, cmd);
                    persist.command = cmd;
                    persist.running = true;
                }
                onResized: (sc) => {
                    const x = slot.resizing ? Math.round(slot.dragX)
                        : (slot.dw.x !== undefined) ? slot.dw.x : Math.round(slot.x);
                    const y = slot.resizing ? Math.round(slot.dragY)
                        : (slot.dw.y !== undefined) ? slot.dw.y : Math.round(slot.y);
                    const lk = slot.dw.locked === true;
                    const cmd = [root.placeTool, slot.pid, "desktopWidget",
                        "" + x, "" + y, "" + sc, "" + lk];
                    root.stageRecordPluginGesture(slot.pid, cmd);
                    persist.command = cmd;
                    persist.running = true;
                }
                onLockRequested: (locked) => {
                    const x = slot.dw.x !== undefined ? slot.dw.x : Math.round(slot.x);
                    const y = slot.dw.y !== undefined ? slot.dw.y : Math.round(slot.y);
                    const cmd = [root.placeTool, slot.pid, "desktopWidget",
                        "" + x, "" + y, "" + slot.effectiveScale, "" + locked];
                    root.stageRecordPluginGesture(slot.pid, cmd);
                    persist.command = cmd;
                    persist.running = true;
                }
                onMenuRequested: (mx, my, id) => {
                    root.openPluginMenu(id, slot.dw.locked === true, mx, my,
                        slot.entry ? slot.entry.manifest : null,
                        slot.entry ? slot.entry.placement : null);
                }
                onSettingsRequested: root.stageOpenSettings(slot.stageId)

                // when the content exposes `editing` (a focused text field),
                // the wallpaper layer grabs the keyboard for as long as it
                // stays true. the flag falls back to false if the content is
                // ever torn down, so the grab can't leak.
                readonly property bool editing: slot.visible && !!(item && item.editing)
                onEditingChanged: win.kbWanted += editing ? 1 : -1
                Component.onDestruction: if (editing) win.kbWanted -= 1

                property var api: QtObject {
                    property var mainInstance: svc.item
                    property var pluginSettings: (slot.entry && slot.entry.placement && slot.entry.placement.settings) ? slot.entry.placement.settings : ({})
                    property string pluginDir: slot.dir
                    function saveSettings() {}
                    // host image viewer: a real click on a photo tile calls
                    // this to open a dimmed, full-desktop enlarged view of the
                    // image (see photoViewer). Closed by click-away or Esc.
                    function expandImage(url) { photoViewer.open(url); }
                }

                PluginObjectSlot {
                    id: svc
                    source: slot.dir.length > 0 ? "file://" + slot.dir + "/service/Main.qml" + slot.versionQuery : ""
                    configure: (service) => { service.pluginApi = slot.api; }
                }

                contentUrl: slot.dir.length > 0 ? "file://" + slot.dir + "/content/Widget.qml" + slot.versionQuery : ""
                configure: (it) => {
                    it.pluginApi = slot.api;
                    it.screen = win.screen;
                    it.density = "compact";
                    it.s = 1;
                    it.widthBudget = 360;
                    it.active = true;
                }

                // push the host accent into the content live. accentColor is
                // always a real colour (never ""), and accentFromHost gates
                // whether the plugin honours it, so "off" = the plugin's own
                // palette. both apply only to a content that declares the props.
                Binding {
                    target: slot.item
                    property: "accentColor"
                    value: slot.hostAccent
                    when: slot.item !== null && slot.colorsCap && slot.item.accentColor !== undefined
                }
                Binding {
                    target: slot.item
                    property: "accentFromHost"
                    value: slot.hostAccentOn
                    when: slot.item !== null && slot.colorsCap && slot.item.accentFromHost !== undefined
                }
                Binding {
                    target: slot
                    property: "x"
                    value: stageSelection.pluginPreview(slot.stageId).x
                    when: stageSelection.hasPluginPreview(slot.stageId)
                        && !slot.stagePreviewSettled && !slot.dragging
                    restoreMode: Binding.RestoreBinding
                }
                Binding {
                    target: slot
                    property: "y"
                    value: stageSelection.pluginPreview(slot.stageId).y
                    when: stageSelection.hasPluginPreview(slot.stageId)
                        && !slot.stagePreviewSettled && !slot.dragging
                    restoreMode: Binding.RestoreBinding
                }


                // Edit-session frame, reparented to the overlay so it sits above
                // the tile and its chrome is always placed.
                StageOutline {
                    parent: composeOverlay
                    visible: root.stageComposing
                    box: Qt.rect(slot.x, slot.y, slot.width, slot.height)
                    title: (slot.entry && slot.entry.manifest
                        && slot.entry.manifest.name)
                        ? slot.entry.manifest.name : slot.pid
                    selected: StageCfg.StageSession.contains(slot.stageId)
                    primary: StageCfg.StageSession.selected === slot.stageId
                    targetItem: slot
                    inputBlocked: root.stageFramingHere
                    counterScale: 1 / Math.max(0.05,
                        StageEdit.EditModeInsets.cardRectFor(root.monitorName,
                            win.width, win.height,
                            Stage.GlobalStates.editProgress,
                            Stage.GlobalStates.editDrawerProgress).width
                                / Math.max(1, win.width))
                    onPicked: modifiers =>
                        stageSelection.pick(slot.stageId, modifiers)
                    onSettings: root.stageOpenSettings(slot.stageId)
                    onRemove: root.stageRemoveWidget(slot.stageId)
                }
            }
        }
        // The one place a stage layer is drawn (docs/stage.md): every cut layer,
        // each at its own z by its `front` flag -- behind the widgets (z 2) or in
        // front of them (z 4). Depth renders this stack still (motion off, no
        // backdrop); Parallax adds drift. Gated off for video/live walls.
        Repeater {
            id: stageLayers
            model: root.stageOn ? StageCfg.StageBackend.layerCountFor(root.wallpaperPath) : 0
            delegate: StageLayer {
                required property int index
                layerIndex: index + 1
                screenName: root.monitorName
                wallPath: root.wallpaperPath
                url: StageCfg.StageBackend.layerUrlFor(root.wallpaperPath, index)
                fit: root.wallpaperFit
                z: StageCfg.StageBackend.layerFront(root.wallpaperPath, index) ? 4 : 2
                mouseNX: StageCfg.StageBackend.cursorNXFor(root.screen.name)
                mouseNY: StageCfg.StageBackend.cursorNYFor(root.screen.name)
                energy: VizCfg.Spectrum.energy
                motionEnabled: root.stageParallax
            }
        }

        // A normal wallpaper and the widgets both use their implicit z of 0.
        // Give the base painter a private floor so shortcuts can remain below
        // widgets without disappearing behind the opaque wallpaper.
        Binding {
            target: backdrop
            property: "z"
            value: -2
        }


        Loader {
            id: desktopIcons
            anchors.fill: parent
            // Keep the layer alive after the persistent adapter loads. Gating
            // the Loader on an initial empty itemsFor() result left it dormant
            // when states.json populated a moment later.
            active: StageIsland.Persistent.ready
            z: root.stageOn ? 2.5 : -1
            opacity: 1 - 0.75 * root.stageFramingDim
            enabled: !root.stageFramingHere
            readonly property bool keyboardNeeded: !!item && (item.hasSelection || item.dialogOpen)
            onKeyboardNeededChanged: {
                win.kbWanted += keyboardNeeded ? 1 : -1;
                if (keyboardNeeded && item)
                    item.forceActiveFocus();
            }
            Component.onDestruction: if (keyboardNeeded) win.kbWanted -= 1
            sourceComponent: StageShortcuts.DesktopShortcutsLayer {
                anchors.fill: parent
                screenName: root.monitorName
                wallpaperLight: Scheme.wallLstar >= 50
                surfaceScale: StageEdit.EditModeInsets.cardRectFor(root.monitorName,
                    win.width, win.height,
                    Stage.GlobalStates.editProgress,
                    Stage.GlobalStates.editDrawerProgress).width / Math.max(1, win.width)
                canvas: stageSelection
                overlayParent: desktopIconsOverlay
            }
        }
        // Modal shortcut cards rise independently; lifting the Loader would
        // also lift every tile over the desktop widgets.
        Item {
            id: desktopIconsOverlay
            anchors.fill: parent
            z: 1000
            visible: !!desktopIcons.item && desktopIcons.item.dialogOpen
            enabled: visible && !root.stageFramingHere
            opacity: 1 - 0.75 * root.stageFramingDim
        }


        StageShortcuts.DesktopShortcutDropArea {
            anchors.fill: parent
            z: root.stageOn ? 2.5 : -1
            screenName: root.monitorName
            iconsLayer: desktopIcons.item
            available: StageIsland.Persistent.ready && !root.stageFramingHere
        }

        // ── Edit widgets overlay (docs/stage.md, "Edit widgets") ──────
        // Hover and selection frames observe presses without stealing a drag;
        // only their action strip and corner handles take the pointer.
        Item {
            id: composeOverlay
            anchors.fill: parent
            z: 60
            visible: root.stageComposing
            enabled: !root.stageFramingHere
            opacity: 1 - 0.75 * root.stageFramingDim
            Behavior on opacity {
                NumberAnimation {
                    duration: Tokens.dur(180)
                    easing.type: Easing.OutCubic
                }
            }

            Rectangle {
                x: Math.min(stageSelection.marqueeAnchorX, stageSelection.marqueeX)
                y: Math.min(stageSelection.marqueeAnchorY, stageSelection.marqueeY)
                width: Math.abs(stageSelection.marqueeX - stageSelection.marqueeAnchorX)
                height: Math.abs(stageSelection.marqueeY - stageSelection.marqueeAnchorY)
                visible: stageSelection.marqueeActive && (width > 3 || height > 3)
                color: Qt.alpha(Tokens.sun, 0.08)
                border.color: Tokens.sun
                border.width: Tokens.border
            }
            component WidgetFrame: StageOutline {
                id: wf
                property string wid: ""
                property var slotItem: null
                property int resolveAttempts: 0
                visible: wf.slotItem !== null
                box: wf.slotItem ? Qt.rect(wf.slotItem.x, wf.slotItem.y,
                    wf.slotItem.width, wf.slotItem.height)
                    : Qt.rect(0, 0, 0, 0)
                title: root.widgetTitle(wf.wid)
                selected: StageCfg.StageSession.contains(wf.wid)
                primary: StageCfg.StageSession.selected === wf.wid
                targetItem: wf.slotItem
                inputBlocked: root.stageFramingHere
                counterScale: 1 / Math.max(0.05,
                    StageEdit.EditModeInsets.cardRectFor(root.monitorName,
                        win.width, win.height,
                        Stage.GlobalStates.editProgress,
                        Stage.GlobalStates.editDrawerProgress).width
                            / Math.max(1, win.width))
                Timer {
                    interval: 16
                    repeat: true
                    triggeredOnStart: true
                    running: root.reloadReady && wf.slotItem === null
                        && wf.resolveAttempts < 60
                    onTriggered: {
                        wf.resolveAttempts++;
                        wf.slotItem = root.slotFor(wf.wid);
                    }
                }
                onPicked: modifiers => stageSelection.pick(wf.wid, modifiers)
                onSettings: root.stageOpenSettings(wf.wid)
                onRemove: root.stageRemoveWidget(wf.wid)
            }
            Repeater {
                model: root.addItems.filter(row => row.enabled
                    && row.id !== "visualizer"
                    && String(row.id).indexOf("plugin:") !== 0)
                    .map(row => row.id)
                delegate: WidgetFrame {
                    required property string modelData
                    wid: modelData
                }
            }

            Loader {
                id: alignBar
                z: 200
                active: StageCfg.StageSession.selection.filter(id =>
                    String(id).indexOf("visualizer") !== 0).length >= 2
                    && !stageSelection.dragging && !root.stageFramingHere
                readonly property real counterScale: 1 / Math.max(0.05,
                    StageEdit.EditModeInsets.cardRectFor(root.monitorName,
                        win.width, win.height,
                        Stage.GlobalStates.editProgress,
                        Stage.GlobalStates.editDrawerProgress).width / Math.max(1, win.width))
                readonly property rect selection: stageSelection.selectionRect
                readonly property real gap: Tokens.s3 * alignBar.counterScale
                readonly property real barHeight: alignBar.item
                    ? alignBar.item.implicitHeight : 0
                readonly property bool below: alignBar.selection.y - alignBar.gap
                    - alignBar.barHeight < 0
                x: alignBar.selection.x + (alignBar.selection.width - width) / 2
                y: alignBar.below
                    ? alignBar.selection.y + alignBar.selection.height + alignBar.gap
                    : alignBar.selection.y - alignBar.gap - alignBar.barHeight
                transformOrigin: alignBar.below ? Item.Top : Item.Bottom
                scale: alignBar.counterScale
                sourceComponent: StageEdit.EditAlignBar {
                    count: StageCfg.StageSession.selection.filter(id =>
                        String(id).indexOf("visualizer") !== 0).length
                    onRequested: mode => stageSelection.alignSelection(mode)
                }
            }
        }
        // While the Wallpaper catalogue owns this monitor, the card itself is
        // the gesture surface. It shares the transformed desktop coordinates,
        // then counter-scales its controls back to native pixels.
        StageWidgets.FadeLoader {
            id: wallpaperFramingOverlay
            anchors.fill: parent
            z: 300
            shown: root.stageEditing
                && StageServices.WallpaperLayout.liveScreen !== ""
                && StageServices.WallpaperLayout.liveScreen === root.monitorName
            active: wallpaperFramingOverlay.shown || wallpaperFramingOverlay.opacity > 0
            // The fading tail is visual only. Once another editor section
            // opens, widget presses must reach the widget layer immediately.
            enabled: wallpaperFramingOverlay.shown
            sourceComponent: StageEdit.EditWallpaperFramingOverlay {
                screenName: root.monitorName
                contentScale: StageEdit.EditModeInsets.cardRectFor(root.monitorName,
                    win.width, win.height,
                    Stage.GlobalStates.editProgress,
                    Stage.GlobalStates.editDrawerProgress).width / Math.max(1, win.width)
                shown: wallpaperFramingOverlay.shown
                cardRadius: StageIsland.Appearance.rounding.verylarge
                    * Stage.GlobalStates.editProgress
            }
        }
        }

        // The Stage Editor's card: the reference's EditModeCard (the live
        // wallpaper blurred and dimmed around the shrunk desktop, its rounded
        // corner and its shadow), drawn over the desktop and cut out to the
        // card the matrix above draws it into. Built only while the mode is
        // on or animating on this monitor; non-interactive.
        Loader {
            anchors.fill: parent
            z: 1
            enabled: false
            active: root.stageEditing && Stage.GlobalStates.editProgress > 0
            opacity: Math.max(0, Math.min(1, Stage.GlobalStates.editProgress))
            sourceComponent: StageBg.EditModeCard {
                wallpaperLayer: backdrop
                card: StageEdit.EditModeInsets.cardRectFor(root.monitorName,
                    win.width, win.height,
                    Stage.GlobalStates.editProgress, Stage.GlobalStates.editDrawerProgress)
                cardRadius: StageIsland.Appearance.rounding.verylarge * Stage.GlobalStates.editProgress
            }
        }

        Process { id: paletteProc }

        // Shared image viewer for desktop plugin tiles. A tile (e.g. Photo
        // Frame) calls pluginApi.expandImage(url) on a real click; this dims the
        // whole desktop and shows that image large + centered
        // (PreserveAspectFit, capped per axis at min(85% of the screen, the
        // image's own size) so small photos never upscale). Any click on the
        // scrim or the photo, or Esc, closes it. It rides this same wallpaper
        // layer, so it sits above the tiles as a full-desktop overlay.
        Item {
            id: photoViewer
            anchors.fill: parent
            z: 100

            // the image being shown; empty = closed.
            property url src: ""
            // `src` is a url: `src !== ""` is always true for an empty url (strict
            // type mismatch against a string), which opened the viewer on boot and
            // stranded kbWanted (an exclusive keyboard grab). Test length, as open() does.
            readonly property bool shown: String(photoViewer.src).length > 0

            visible: opacity > 0
            opacity: photoViewer.shown ? 1 : 0
            Behavior on opacity { NumberAnimation { duration: Theme.quick; easing.type: Theme.ease } }

            function open(url) { if (url && String(url).length > 0) photoViewer.src = url; }
            function close() { photoViewer.src = ""; }

            // Esc closes. The wallpaper layer only holds the keyboard while
            // something wants it (kbWanted), so bump it exactly like a focused
            // plugin text field and take active focus while shown.
            Keys.onEscapePressed: photoViewer.close()
            onShownChanged: {
                win.kbWanted += photoViewer.shown ? 1 : -1;
                if (photoViewer.shown)
                    photoViewer.forceActiveFocus();
            }
            Component.onDestruction: if (photoViewer.shown) win.kbWanted -= 1

            // dim scrim + click-away. Accepts both buttons so a click or
            // right-click anywhere closes, instead of falling through to a tile
            // or the global desktop menu behind it.
            Rectangle {
                anchors.fill: parent
                color: Qt.rgba(0, 0, 0, 0.78)
            }
            MouseArea {
                anchors.fill: parent
                enabled: photoViewer.shown
                acceptedButtons: Qt.LeftButton | Qt.RightButton
                onClicked: photoViewer.close()
            }

            // the enlarged print: rounded-clipped to match the tile aesthetic.
            // clicks fall through it to the MouseArea above, so tapping the photo
            // closes too.
            ClippingRectangle {
                anchors.centerIn: parent
                radius: Theme.radiusWidget
                color: "transparent"
                width: big.paintedWidth
                height: big.paintedHeight

                Image {
                    id: big
                    anchors.centerIn: parent
                    source: photoViewer.src
                    fillMode: Image.PreserveAspectFit
                    asynchronous: true
                    cache: true
                    smooth: true
                    width: implicitWidth > 0 ? Math.min(photoViewer.width * 0.85, implicitWidth) : photoViewer.width * 0.85
                    height: implicitHeight > 0 ? Math.min(photoViewer.height * 0.85, implicitHeight) : photoViewer.height * 0.85
                }
            }
        }

        // position/scale writeback for plugin tiles. ryoku-plugins-place
        // merges free x/y (+ optional scale/locked) into plugins.json;
        // Registry's file-watch then retunes every surface.
        Process { id: persist }
        // separate Processes per menu action so a quick Hide-then-Lock or
        // resize-then-Lock doesn't trample an in-flight `persist` command.
        Process { id: hide }
        Process { id: lockProc }
        // settings writeback from the right-click menu.
        Process {
            id: settingsProc
            onRunningChanged: if (!settingsProc.running) root._runSettingsQueue()
        }
        Process { id: sizeProc }
        Process { id: opacityProc }
    }

    // The desktop's right-click menus (bare wallpaper, a widget, a plugin tile)
    // live on their own Overlay layer-shell surface, not inside the Bottom-layer
    // widget window, so an open menu always draws above every window instead of
    // being buried under whatever overlaps the click. exclusiveZone 0 keeps the
    // surface inside the work area (the recording-island idiom), so a card clamped
    // to it lands clear of the frame band and the dock. It is mapped only while a
    // menu shows, so it never blocks the desktop's own input otherwise; while
    // shown the whole surface is live (each menu carries its own click-away
    // catcher) and takes keyboard on demand so Esc dismisses.
    PanelWindow {
        id: menuWin
        screen: root.screen
        visible: root.menusShowing
        color: "transparent"
        exclusiveZone: 0
        WlrLayershell.layer: WlrLayer.Overlay
        WlrLayershell.namespace: "ryoku-desktop-menu"
        WlrLayershell.keyboardFocus: root.menusShowing ? WlrKeyboardFocus.OnDemand : WlrKeyboardFocus.None
        anchors { top: true; left: true; right: true; bottom: true }

        Loader {
            id: widgetMenuLoader
            anchors.fill: parent
            z: 90
            active: false
            onItemChanged: if (item && root.pendingWidgetMenu) {
                const p = root.pendingWidgetMenu;
                root.pendingWidgetMenu = null;
                root.openWidgetMenu(p[0], p[1], p[2]);
            }
            sourceComponent: Component {
                WidgetMenu {
                    monitor: root.monitorName
                    onCustomizeRequested: (w) => root.openInspector(w)
                }
            }
        }

        // The system desktop right-click menu (bare wallpaper): the shared
        // iRiS-structured surface, its own loader so it never rides the widget one.
        Loader {
            id: desktopMenuLoader
            anchors.fill: parent
            z: 90
            active: false
            onItemChanged: if (item && root.pendingDesktopMenu) {
                const p = root.pendingDesktopMenu;
                root.pendingDesktopMenu = null;
                root.openDesktopMenu(p[0], p[1]);
            }
            sourceComponent: Component {
                DesktopContextMenu { desktop: root }
            }
        }

        // per-tile right-click menu, hoisted to PanelWindow level so the
        // click-away catcher covers the whole desktop and a tile that
        // vanishes (Hide) doesn't pull the menu down with it.
        Loader {
            id: pluginMenuLoader
            anchors.fill: parent
            z: 90
            active: false
            onItemChanged: if (item && root.pendingPluginMenu) {
                const p = root.pendingPluginMenu;
                root.pendingPluginMenu = null;
                root.openPluginMenu(p[0], p[1], p[2], p[3], p[4], p[5]);
            }
            sourceComponent: Component {
            PluginWidgetMenu {
                id: pluginMenu
                z: 90
                onHideRequested: (id) => {
                    hide.command = [root.placeTool, id, "enabled", "false"];
                    hide.running = true;
                    pluginMenu.close();
                }
                onLockToggled: (id) => {
                    const dw = win.placementOf(id);
                    const x = (dw.x !== undefined) ? dw.x : 80;
                    const y = (dw.y !== undefined) ? dw.y : 80;
                    const sc = (dw.scale !== undefined) ? dw.scale : 1;
                    const lk = !(dw.locked === true);
                    lockProc.command = [root.placeTool, id, "desktopWidget",
                        "" + x, "" + y, "" + sc, "" + lk];
                    lockProc.running = true;
                }
                onSettingChanged: (id, key, value) => {
                    var obj = {};
                    obj[key] = value;
                    // queue so a two-key change (colour mode: colorAuto + color)
                    // can't stomp itself on the single settings Process.
                    root._settingsQueue.push([root.placeTool, id, "settings", JSON.stringify(obj)]);
                    root._runSettingsQueue();
                }
                onSizeChanged: (id, sc) => {
                    const dw = win.placementOf(id);
                    const x = (dw.x !== undefined) ? dw.x : 80;
                    const y = (dw.y !== undefined) ? dw.y : 80;
                    const lk = (dw.locked === true);
                    // scale only: opacity arg omitted -> ryoku-plugins-place keeps it.
                    sizeProc.command = [root.placeTool, id, "desktopWidget",
                        "" + x, "" + y, "" + sc, "" + lk];
                    sizeProc.running = true;
                }
                onOpacityPicked: (id, op) => {
                    const dw = win.placementOf(id);
                    const x = (dw.x !== undefined) ? dw.x : 80;
                    const y = (dw.y !== undefined) ? dw.y : 80;
                    const lk = (dw.locked === true);
                    // opacity only: scale left "" so the tool keeps the current one.
                    opacityProc.command = [root.placeTool, id, "desktopWidget",
                        "" + x, "" + y, "", "" + lk, "" + op];
                    opacityProc.running = true;
                }
            }
            }
        }
    }

    // The widget inspector (Customize sheet) rides its own Overlay surface next
    // to the menu one: exclusiveZone 0 keeps it inside the work area, clear of
    // the bars, dock and frame. Unlike the menu it never dismisses on an outside
    // click -- the surface masks to the sheet, so a press off it falls through to
    // the widgets and the user watches the widget retune live. Esc and the close
    // button dismiss; keyboard is taken on demand for the text fields.
    PanelWindow {
        id: inspectorWin
        screen: root.screen
        visible: root.inspectorShowing
        color: "transparent"
        exclusiveZone: 0
        WlrLayershell.layer: WlrLayer.Overlay
        WlrLayershell.namespace: "ryoku-desktop-inspector"
        WlrLayershell.keyboardFocus: root.inspectorShowing ? WlrKeyboardFocus.OnDemand : WlrKeyboardFocus.None
        anchors { top: true; left: true; right: true; bottom: true }
        // Masked to the sheet so an outside press reaches the widgets; the video
        // picker draws full-surface, so drop the mask while it is open.
        mask: (root.inspectorShowing && !(inspectorLoader.item && inspectorLoader.item.pickerOpen === true)) ? inspectorMask : null
        Region { id: inspectorMask; item: inspectorLoader.item ? inspectorLoader.item.sheetItem : null }

        Loader {
            id: inspectorLoader
            anchors.fill: parent
            z: 90
            active: false
            onItemChanged: if (item && root.pendingInspector) {
                const p = root.pendingInspector;
                root.pendingInspector = null;
                root.openInspector(p);
            }
            sourceComponent: Component {
                WidgetInspector {}
            }
        }
    }

    Connections {
        target: root
        function onStageComposingChanged() {
            if (!root.stageComposing)
                return;
            root.pendingWidgetMenu = null;
            root.pendingInspector = null;
            if (widgetMenuLoader.item)
                widgetMenuLoader.item.close();
            if (inspectorLoader.item)
                inspectorLoader.item.close();
        }
    }

    // The Stage Editor's store bridge, mounted while the chrome frames this
    // screen: the ported drawer then lists and edits Ryoku's real widgets
    // (widgets.json, plugins.json, the visualizer) instead of the island's own
    // copy of the reference's store (docs/stage.md).
    Loader {
        active: root.stageComposing
        sourceComponent: StageWidgetProvider {
            monitor: root.monitorName
            rows: root.addItems
            placeTool: root.placeTool
            wallpaperPath: root.wallpaperPath
        }
    }
}
