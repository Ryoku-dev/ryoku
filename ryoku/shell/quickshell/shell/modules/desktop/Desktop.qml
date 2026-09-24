import QtQuick
import Quickshell
import Quickshell.Wayland
import Quickshell.Io
import Quickshell.Widgets
import "Singletons"
import "clock"
import "calendar"
import "music"
import "aio"
import "stats"
import "weather"
import "notes"
import Ryoku.PluginKit
import shell.services as Services
import "../stage"
import "../stage/Singletons" as StageCfg
import "../visualizer/Singletons" as VizCfg
import "../visualizer" as Viz
import "../wallpaper" as WallpaperMod

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
    readonly property bool hostsVisualizer: root.stageOn && VizCfg.Config.enabled
        && !(root.stageState && (root.stageState.visualizerOverlay || root.stageState.visualizerPlacing))
    readonly property string monitorName: root.screen ? root.screen.name : ""
    // Edit widgets on this monitor frees every widget for dragging and lifts
    // this desktop above open windows; Done restores the per-widget locks.
    readonly property bool stageComposing: StageCfg.StageSession.onMonitor(root.monitorName)
    // Grab the keyboard while composing so Esc/Enter exit the mode (the bar owns
    // the keys); dropping it hands the keyboard back like any widget edit.
    onStageComposingChanged: {
        win.kbWanted += root.stageComposing ? 1 : -1;
        if (root.stageComposing)
            root._snapshot();
    }
    // Human titles for the built-in widgets, for the frame labels.
    function widgetTitle(w) {
        const n = { clock: "Clock", calendar: "Calendar", music: "Music", aio: "All-in-one", stats: "System stats", weather: "Weather", notes: "Notes" };
        return n[w] || w;
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
            { id: "clock", label: "Clock", icon: "schedule", enabled: Config.clockEnabled, group: "" },
            { id: "calendar", label: "Calendar", icon: "calendar_month", enabled: Config.calendarEnabled, group: "" },
            { id: "music", label: "Music", icon: "music_note", enabled: Config.musicEnabled, group: "" },
            { id: "aio", label: "All-in-one", icon: "dashboard", enabled: Config.aioEnabled, group: "" },
            { id: "stats", label: "System stats", icon: "monitor_heart", enabled: Config.statsEnabled, group: "" },
            { id: "weather", label: "Weather", icon: "partly_cloudy_day", enabled: Config.weatherEnabled, group: "" },
            { id: "notes", label: "Notes", icon: "sticky_note_2", enabled: Config.notesEnabled, group: "" },
            { id: "visualizer", label: "Visualizer", icon: "graphic_eq", enabled: VizCfg.Config.enabled, group: "" }
        ];
        // Fallback group name for a plugin whose manifest names no set. Plain
        // like the built-in labels above; the Hub translates its own copy.
        const other = "Plugins";
        const order = [];
        const byGroup = {};
        const pids = win.desktopPluginIds || [];
        for (var i = 0; i < pids.length; i++) {
            const pid = pids[i];
            const e = Registry.plugins.find(p => p.id === pid);
            const set = (e && e.manifest && typeof e.manifest.set === "string" && e.manifest.set.length > 0)
                ? e.manifest.set : other;
            if (!byGroup.hasOwnProperty(set)) {
                byGroup[set] = [];
                order.push(set);
            }
            byGroup[set].push({
                id: "plugin:" + pid,
                label: (e && e.manifest && e.manifest.name) ? e.manifest.name : pid,
                icon: (e && e.manifest && e.manifest.defaults && e.manifest.defaults.icon)
                    ? e.manifest.defaults.icon : "widgets",
                enabled: true,
                group: set
            });
        }
        const out = bi.slice();
        for (var g = 0; g < order.length; g++) {
            const rows = byGroup[order[g]];
            for (var r = 0; r < rows.length; r++)
                out.push(rows[r]);
        }
        return out;
    }
    // Add-drop-down toggle: on enables (a built-in flag, a plugin placement, or
    // the visualizer), off runs the same Remove path. Every toggle marks the
    // session dirty so Reset appears.
    function stageAddToggle(id) {
        if (id === "visualizer") {
            VizCfg.Config.setEnabled(!VizCfg.Config.enabled);
            StageCfg.StageSession.markDirty();
            return;
        }
        if (id.indexOf("plugin:") === 0) {
            const pid = id.slice(7);
            const on = (win.desktopPluginIds || []).indexOf(pid) >= 0;
            paletteProc.command = [root.placeTool, pid, "enabled", on ? "false" : "true"];
            paletteProc.running = true;
            StageCfg.StageSession.markDirty();
            return;
        }
        Config.set(id + "Enabled", !Config[id + "Enabled"]);
        StageCfg.StageSession.markDirty();
    }
    // The built-in slot behind a widget id, for placing its Settings menu.
    // Read through the loaders: a disabled widget has no slot item.
    function _builtinSlot(id) {
        switch (id) {
        case "clock": return clockLoader.item;
        case "calendar": return calendarLoader.item;
        case "music": return musicLoader.item;
        case "aio": return aioLoader.item;
        case "stats": return statsLoader.item;
        case "weather": return weatherLoader.item;
        case "notes": return notesLoader.item;
        }
        return null;
    }
    // Arm-and-open for the right-click menus: the first open builds the menu
    // synchronously, and the pending request lands the moment it is ready.
    property var pendingWidgetMenu: null
    property var pendingPluginMenu: null
    function openWidgetMenu(widget, x, y) {
        if (widgetMenuLoader.item) {
            if (widget === "desktop") widgetMenuLoader.item.openDesktop(x, y);
            else widgetMenuLoader.item.openFor(widget, x, y);
            return;
        }
        root.pendingWidgetMenu = [widget, x, y];
        widgetMenuLoader.active = true;
    }
    function openPluginMenu(id, locked, x, y, manifest, placement) {
        if (pluginMenuLoader.item) {
            pluginMenuLoader.item.openFor(id, locked, x, y, manifest, placement);
            return;
        }
        root.pendingPluginMenu = [id, locked, x, y, manifest, placement];
        pluginMenuLoader.active = true;
    }
    // A widget frame's Settings button: open that built-in's own menu at the
    // frame's corner (its design, lock, size, opacity, colour, snap).
    function stageOpenSettings(id) {
        const s = root._builtinSlot(id);
        root.openWidgetMenu(id, s ? s.x : 120, s ? s.y : 120);
    }
    // A widget frame's Remove button: hide the built-in and drop any selection.
    function stageRemoveWidget(id) {
        Config.set(id + "Enabled", false);
        if (StageCfg.StageSession.selected === id)
            StageCfg.StageSession.deselect();
        StageCfg.StageSession.markDirty();
    }
    // Reset snapshot (docs/stage.md, "Edit widgets"): the widgets Config and the
    // placed plugin set as they were when this session opened, captured on enter
    // and written back on resetRequested.
    readonly property var _widgetKeys: [
        "clockEnabled", "clockDesign", "clock24h", "clockSeconds", "clockAccent", "clockScale", "clockAnchor", "clockX", "clockY", "clockLocked", "clockOpacity", "clockBg", "clockRadius", "clockColor", "clockColor2", "clockGradient", "dateShow", "dateDesign", "widgetFont",
        "calendarEnabled", "calendarStyle", "calendarWeeks", "calendarWeekNumbers", "calendarHolidayRegion", "calendarScale", "calendarAnchor", "calendarX", "calendarY", "calendarLocked", "calendarOpacity", "calendarColor", "calendarColor2", "calendarGradient",
        "musicEnabled", "musicStyle", "musicLyrics", "musicViz", "musicScale", "musicAnchor", "musicX", "musicY", "musicLocked", "musicOpacity", "musicApp", "musicShape", "musicVideo", "musicVideoFile", "musicColor", "musicColor2", "musicGradient",
        "aioEnabled", "aioStyle", "aioScale", "aioAnchor", "aioX", "aioY", "aioLocked", "aioOpacity", "aioColor", "aioColor2", "aioGradient",
        "statsEnabled", "statsScale", "statsAnchor", "statsX", "statsY", "statsLocked", "statsOpacity", "statsColor", "statsColor2", "statsGradient",
        "weatherEnabled", "weatherDesign", "weatherScale", "weatherAnchor", "weatherX", "weatherY", "weatherLocked", "weatherOpacity", "weatherColor", "weatherColor2", "weatherGradient",
        "notesEnabled", "notesScale", "notesAnchor", "notesX", "notesY", "notesLocked", "notesOpacity", "notesWidth", "notesHeight", "notesColor", "notesColor2", "notesGradient"
    ]
    property var _snapConfig: null
    property var _snapPlugins: null
    property var _resetQueue: []
    property var _settingsQueue: []
    function _snapshot() {
        const c = {};
        const ks = root._widgetKeys;
        for (var i = 0; i < ks.length; i++)
            c[ks[i]] = Config[ks[i]];
        root._snapConfig = c;
        const pl = {};
        const dps = win.desktopPlugins || [];
        for (var j = 0; j < dps.length; j++) {
            const p = dps[j];
            const dw = (p.placement && p.placement.desktopWidget) || {};
            pl[p.id] = { x: dw.x, y: dw.y, scale: dw.scale, locked: dw.locked === true, opacity: dw.opacity };
        }
        root._snapPlugins = pl;
    }
    function _restore() {
        if (root._snapConfig) {
            const c = root._snapConfig;
            const ks = root._widgetKeys;
            const back = {};
            for (var i = 0; i < ks.length; i++)
                if (Config[ks[i]] !== c[ks[i]])
                    back[ks[i]] = c[ks[i]];
            Config.setMany(back);
        }
        const q = [];
        const snap = root._snapPlugins || {};
        const nowIds = win.desktopPluginIds || [];
        for (var n = 0; n < nowIds.length; n++)
            if (!snap.hasOwnProperty(nowIds[n]))
                q.push([root.placeTool, nowIds[n], "enabled", "false"]);
        for (var pid in snap) {
            const s = snap[pid];
            if (nowIds.indexOf(pid) < 0)
                q.push([root.placeTool, pid, "enabled", "true"]);
            const cmd = [root.placeTool, pid, "desktopWidget",
                "" + (s.x !== undefined ? s.x : 80), "" + (s.y !== undefined ? s.y : 80)];
            const hasScale = s.scale !== undefined;
            const hasLocked = s.locked !== undefined;
            const hasOpacity = s.opacity !== undefined;
            // positional args: pad an earlier one with "" (= keep existing) when
            // only a later one is present, so opacity lands in slot 5.
            if (hasScale || hasLocked || hasOpacity)
                cmd.push(hasScale ? "" + s.scale : "");
            if (hasLocked || hasOpacity)
                cmd.push(hasLocked ? "" + (s.locked === true) : "");
            if (hasOpacity)
                cmd.push("" + s.opacity);
            q.push(cmd);
        }
        root._resetQueue = q;
        root._runResetQueue();
    }
    function _runResetQueue() {
        if (resetProc.running)
            return;
        if (!root._resetQueue || root._resetQueue.length === 0)
            return;
        const next = root._resetQueue.shift();
        resetProc.command = next;
        resetProc.running = true;
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
        // None while nothing on this layer wants the keyboard, so this
        // full-screen Bottom layer never holds focus on an empty workspace
        // (which would otherwise leave the next-opened window unfocused).
        // A plugin tile's focused text field bumps `kbWanted`; the layer
        // then grabs the keyboard (the same exclusive grab the pill uses for
        // its launcher) so the field can be typed in, and releases it the
        // moment the field blurs. pointer input is unaffected either way -
        // layer-shell routes clicks by input region, not kb interactivity -
        // so drag and the right-click menu always fire.
        property int kbWanted: 0
        onKbWantedChanged: if (kbWanted === 0) root.kbRestore()
        WlrLayershell.keyboardFocus: kbWanted > 0 ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None

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

        // the built-in slot currently being dragged (a single pointer, so at most
        // one), for the drag guides: their grid step and centre-snap lighting.
        readonly property var dragSlot: (clockLoader.item && clockLoader.item.dragging) ? clockLoader.item
            : (calendarLoader.item && calendarLoader.item.dragging) ? calendarLoader.item
            : (musicLoader.item && musicLoader.item.dragging) ? musicLoader.item
            : (aioLoader.item && aioLoader.item.dragging) ? aioLoader.item
            : (statsLoader.item && statsLoader.item.dragging) ? statsLoader.item
            : (weatherLoader.item && weatherLoader.item.dragging) ? weatherLoader.item
            : (notesLoader.item && notesLoader.item.dragging) ? notesLoader.item : null

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
            if (root.stageComposing)
                StageCfg.StageSession.markDirty();
        }

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
            dpr: screenDpr
            // Keep the still decoded while a video plays: the frame path is
            // always a paintable still now, and holding it means the reveal
            // off a video has its old texture instead of a black cut.
            url: root.wallpaperUrl
            fit: root.wallpaperFit
            transition: root.wallpaperTransition
            videoUrl: root.videoUrl
            live: root.wallpaperLive
            videoMuted: root.videoMuted
            videoVolume: root.videoVolume
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

        // While the stage is on, the visualizer lives inside this surface so it
        // sits behind every cut-out: above the backdrop, below every layer and
        // widget. Its own surface (a sibling window that can never interleave
        // with the subject) is suppressed meanwhile; Above windows and the
        // Placer keep that surface (docs/stage.md).
        Item {
            z: 1.5
            anchors.fill: parent
            visible: root.hostsVisualizer
            Viz.InlineVisualizer { anchors.fill: parent }
        }

        // Mirror of the same image for glass widgets: Qt cannot sample another
        // scene graph, so ShaderEffectSource captures the pixels beneath a
        // frosted widget from this offscreen copy. WidgetGlass hides this
        // source after taking its crop. Hidden while a video plays: the glass
        // samples the still, and a live clip would freeze the capture.
        readonly property bool glassWanted: root.widgetsEnabled && root.videoUrl === ""
            && ((Config.calendarEnabled && Config.calendarStyle === "glass")
                || (Config.musicEnabled && Config.musicStyle === "glass"))
        Image {
            id: glassBackdrop
            anchors.fill: parent
            // An invisible Image still decodes while its source is set: hold the
            // url back until a glass widget actually samples it, or this mirror
            // costs a full-screen decode on every box that has a wallpaper.
            source: win.glassWanted ? root.wallpaperUrl : ""
            cache: false
            asynchronous: true
            sourceSize.width: Math.ceil(width * backdrop.screenDpr)
            sourceSize.height: Math.ceil(height * backdrop.screenDpr)
            visible: win.glassWanted
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

        // right-click empty desktop = global menu. sits behind the widgets
        // (which own their own right-click) and only takes RightButton, so
        // left-clicks on wallpaper fall through instead of being silently
        // swallowed.
        MouseArea {
            anchors.fill: parent
            acceptedButtons: Qt.RightButton
            onPressed: (mouse) => root.openWidgetMenu("desktop", mouse.x, mouse.y)
        }
        // Left-click on bare wallpaper unwinds one level while composing (an
        // open drop-down, then the selection, then the session), the same order
        // Escape uses. It sits below the widgets, so a click on a widget still
        // reaches it, and it takes only the left button so the right-click menu
        // still opens.
        MouseArea {
            anchors.fill: parent
            enabled: root.stageComposing
            acceptedButtons: Qt.LeftButton
            onPressed: {
                if (StageCfg.StageSession.panel !== "")
                    StageCfg.StageSession.closePanel();
                else if (StageCfg.StageSession.selected !== "")
                    StageCfg.StageSession.deselect();
                else
                    StageCfg.StageSession.leave();
            }
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
            active: win.dragSlot !== null
            gridSize: win.dragSlot ? win.dragSlot.gridSize : 32
            dragCentreX: win.dragSlot ? win.dragSlot.x + win.dragSlot.width / 2 : 0
            dragCentreY: win.dragSlot ? win.dragSlot.y + win.dragSlot.height / 2 : 0
        }

        Loader {
            id: clockLoader
            anchors.fill: parent
            z: root.widgetZ("clock")
            active: root.widgetsEnabled && root.reloadReady && Config.clockEnabled
            sourceComponent: Component {
            Item {
                anchors.fill: parent
            WidgetSlot {
                id: clockSlot
                widget: "clock"
                z: root.widgetZ("clock")
                visible: true
                anchor: Config.clockAnchor
                freeX: Config.clockX
                freeY: Config.clockY
                locked: root.stageComposing ? false : Config.clockLocked
                composing: root.stageComposing
                bg: Config.clockBg
                radius: Config.clockRadius
                scaleCfg: Config.clockScale
                pad: Config.clockBg === "none" ? 0 : Math.round(24 * Config.clockScale)
                onMenuRequested: (x, y, w) => root.openWidgetMenu(w, x, y)
                onDropped: (box) => win.flashDrop(box)
                onResized: if (root.stageComposing) StageCfg.StageSession.markDirty()
                Clock {}
            }
            }
            }
        }

        Loader {
            id: calendarLoader
            anchors.fill: parent
            z: root.widgetZ("calendar")
            active: root.widgetsEnabled && root.reloadReady && Config.calendarEnabled
            sourceComponent: Component {
            Item {
                anchors.fill: parent
            WidgetSlot {
                id: calendarSlot
                widget: "calendar"
                z: root.widgetZ("calendar")
                visible: true
                anchor: Config.calendarAnchor
                freeX: Config.calendarX
                freeY: Config.calendarY
                locked: root.stageComposing ? false : Config.calendarLocked
                composing: root.stageComposing
                bg: "none"
                scaleCfg: Config.calendarScale
                onMenuRequested: (x, y, w) => root.openWidgetMenu(w, x, y)
                onDropped: (box) => win.flashDrop(box)
                onResized: if (root.stageComposing) StageCfg.StageSession.markDirty()
                CalendarWidget {
                    style: Config.calendarStyle
                    weeks: Config.calendarWeeks
                    showWeekNumbers: Config.calendarWeekNumbers
                    holidayRegion: Config.calendarHolidayRegion
                    active: calendarSlot.visible
                    s: Config.calendarScale
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
            active: root.widgetsEnabled && root.reloadReady && Config.musicEnabled
            sourceComponent: Component {
            Item {
                anchors.fill: parent
            WidgetSlot {
                id: musicSlot
                widget: "music"
                z: root.widgetZ("music")
                visible: true
                anchor: Config.musicAnchor
                freeX: Config.musicX
                freeY: Config.musicY
                locked: root.stageComposing ? false : Config.musicLocked
                composing: root.stageComposing
                bg: "none"
                scaleCfg: Config.musicScale
                onMenuRequested: (x, y, w) => root.openWidgetMenu(w, x, y)
                onDropped: (box) => win.flashDrop(box)
                onResized: if (root.stageComposing) StageCfg.StageSession.markDirty()
                MusicWidget {
                    style: Config.musicStyle
                    showLyrics: Config.musicLyrics
                    viz: Config.musicViz
                    active: musicSlot.visible
                    musicApp: Config.musicApp
                    shape: Config.musicShape
                    videoMode: Config.musicVideo
                    videoFile: Config.musicVideoFile
                    s: Config.musicScale
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
            active: root.widgetsEnabled && root.reloadReady && Config.aioEnabled
            sourceComponent: Component {
            Item {
                anchors.fill: parent
            WidgetSlot {
                id: aioSlot
                widget: "aio"
                z: root.widgetZ("aio")
                visible: true
                anchor: Config.aioAnchor
                freeX: Config.aioX
                freeY: Config.aioY
                locked: root.stageComposing ? false : Config.aioLocked
                composing: root.stageComposing
                bg: "none"
                scaleCfg: Config.aioScale
                onMenuRequested: (x, y, w) => root.openWidgetMenu(w, x, y)
                onDropped: (box) => win.flashDrop(box)
                onResized: if (root.stageComposing) StageCfg.StageSession.markDirty()
                AioWidget {
                    style: Config.aioStyle
                    s: Config.aioScale
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
            active: root.widgetsEnabled && root.reloadReady && Config.statsEnabled
            sourceComponent: Component {
            Item {
                anchors.fill: parent
            WidgetSlot {
                id: statsSlot
                widget: "stats"
                z: root.widgetZ("stats")
                visible: true
                anchor: Config.statsAnchor
                freeX: Config.statsX
                freeY: Config.statsY
                locked: root.stageComposing ? false : Config.statsLocked
                composing: root.stageComposing
                bg: "none"
                scaleCfg: Config.statsScale
                onMenuRequested: (x, y, w) => root.openWidgetMenu(w, x, y)
                onDropped: (box) => win.flashDrop(box)
                onResized: if (root.stageComposing) StageCfg.StageSession.markDirty()
                StatsWidget {
                    s: Config.statsScale
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
            active: root.widgetsEnabled && root.reloadReady && Config.weatherEnabled
            sourceComponent: Component {
            Item {
                anchors.fill: parent
            WidgetSlot {
                id: weatherSlot
                widget: "weather"
                z: root.widgetZ("weather")
                visible: true
                anchor: Config.weatherAnchor
                freeX: Config.weatherX
                freeY: Config.weatherY
                locked: root.stageComposing ? false : Config.weatherLocked
                composing: root.stageComposing
                bg: "none"
                scaleCfg: Config.weatherScale
                onMenuRequested: (x, y, w) => root.openWidgetMenu(w, x, y)
                onDropped: (box) => win.flashDrop(box)
                onResized: if (root.stageComposing) StageCfg.StageSession.markDirty()
                WeatherWidget {
                    design: Config.weatherDesign
                    s: Config.weatherScale
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
            active: root.widgetsEnabled && root.reloadReady && Config.notesEnabled
            sourceComponent: Component {
            Item {
                anchors.fill: parent
            WidgetSlot {
                id: notesSlot
                widget: "notes"
                z: root.widgetZ("notes")
                visible: true
                anchor: Config.notesAnchor
                freeX: Config.notesX
                freeY: Config.notesY
                locked: root.stageComposing ? false : Config.notesLocked
                composing: root.stageComposing
                bg: "none"
                scaleCfg: Config.notesScale
                onMenuRequested: (x, y, w) => root.openWidgetMenu(w, x, y)
                onDropped: (box) => win.flashDrop(box)
                onResized: if (root.stageComposing) StageCfg.StageSession.markDirty()
                // notes is the first built-in editable widget: while its pad holds
                // focus the layer must grab the keyboard (bump kbWanted), and drop
                // the grab the instant it blurs, or the desktop is stranded.
                onEditingChanged: win.kbWanted += editing ? 1 : -1
                Component.onDestruction: if (editing) win.kbWanted -= 1
                NotesWidget {
                    s: Config.notesScale
                    active: notesSlot.visible
                    wLogical: Config.notesWidth
                    hLogical: Config.notesHeight
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
            model: root.widgetsEnabled ? win.desktopPluginIds : []
            delegate: PluginDesktopSlot {
                id: slot
                required property string modelData
                readonly property string pid: modelData
                // live registry entry for this id, re-resolved whenever
                // Registry reloads. placement (x/y/scale/bg) updates here
                // without rebuilding the delegate, because the model is the
                // stable id list, not the per-write plugin array.
                readonly property var entry: Registry.plugins.find(p => p.id === slot.pid) || null
                readonly property var dw: (entry && entry.placement && entry.placement.desktopWidget) || ({})
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
                locked: root.stageComposing ? false : (slot.dw.locked === true)
                composing: root.stageComposing
                scaleCfg: slot.dw.scale || 0.85
                opacityCfg: slot.dw.opacity !== undefined ? slot.dw.opacity : 1
                freeX: slot.dw.x !== undefined ? slot.dw.x : 80
                freeY: slot.dw.y !== undefined ? slot.dw.y : 80
                bg: slot.dw.bg ? slot.dw.bg : ((entry && entry.manifest && entry.manifest.defaults && entry.manifest.defaults.desktopWidget && entry.manifest.defaults.desktopWidget.bg) || "card")
                radius: slot.dw.radius || 26

                onMoved: (x, y) => {
                    persist.command = [root.placeTool, slot.pid, "desktopWidget", "" + x, "" + y];
                    persist.running = true;
                    if (root.stageComposing) StageCfg.StageSession.markDirty();
                }
                onResized: (sc) => {
                    const x = (slot.dw.x !== undefined) ? slot.dw.x : Math.round(slot.x);
                    const y = (slot.dw.y !== undefined) ? slot.dw.y : Math.round(slot.y);
                    const lk = (slot.dw.locked === true);
                    persist.command = [root.placeTool, slot.pid, "desktopWidget",
                        "" + x, "" + y, "" + sc, "" + lk];
                    persist.running = true;
                    if (root.stageComposing) StageCfg.StageSession.markDirty();
                }
                onMenuRequested: (mx, my, id) => {
                    root.openPluginMenu(id, slot.dw.locked === true, mx, my,
                        slot.entry ? slot.entry.manifest : null,
                        slot.entry ? slot.entry.placement : null);
                }

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

                // Edit-session frame, reparented to the overlay so it sits above
                // the tile and its chrome is always placed.
                StageOutline {
                    parent: composeOverlay
                    visible: root.stageComposing
                    box: Qt.rect(slot.x, slot.y, slot.width, slot.height)
                    title: (slot.entry && slot.entry.manifest && slot.entry.manifest.name) ? slot.entry.manifest.name : slot.pid
                    selected: StageCfg.StageSession.selected === slot.pid
                    onPicked: { StageCfg.StageSession.select(slot.pid); StageCfg.StageSession.closePanel(); }
                    onSettings: root.openPluginMenu(slot.pid, slot.dw.locked === true, slot.x, slot.y, slot.entry ? slot.entry.manifest : null, slot.entry ? slot.entry.placement : null)
                    onRemove: {
                        hide.command = [root.placeTool, slot.pid, "enabled", "false"];
                        hide.running = true;
                        if (StageCfg.StageSession.selected === slot.pid)
                            StageCfg.StageSession.deselect();
                        StageCfg.StageSession.markDirty();
                    }
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

        // ── Edit widgets overlay (docs/stage.md, "Edit widgets") ──────
        // A frame on every enabled widget while composing: its outline, name and
        // Settings/Remove buttons. The frames are input-transparent (a passive
        // tap selects, presses fall through), so the widget's own grip still
        // drags underneath; the ribbon draws the subject and layer outlines on
        // its own surface, never here.
        Item {
            id: composeOverlay
            anchors.fill: parent
            z: 60
            visible: root.stageComposing

            component WidgetFrame: StageOutline {
                id: wf
                property var slotItem: null
                property string wid: ""
                visible: wf.slotItem ? wf.slotItem.visible : false
                box: wf.slotItem ? Qt.rect(wf.slotItem.x, wf.slotItem.y, wf.slotItem.width, wf.slotItem.height) : Qt.rect(0, 0, 0, 0)
                title: root.widgetTitle(wf.wid)
                selected: StageCfg.StageSession.selected === wf.wid
                onPicked: { StageCfg.StageSession.select(wf.wid); StageCfg.StageSession.closePanel(); }
                onSettings: root.stageOpenSettings(wf.wid)
                onRemove: root.stageRemoveWidget(wf.wid)
            }
            WidgetFrame { wid: "clock"; slotItem: clockLoader.item }
            WidgetFrame { wid: "calendar"; slotItem: calendarLoader.item }
            WidgetFrame { wid: "music"; slotItem: musicLoader.item }
            WidgetFrame { wid: "aio"; slotItem: aioLoader.item }
            WidgetFrame { wid: "stats"; slotItem: statsLoader.item }
            WidgetFrame { wid: "weather"; slotItem: weatherLoader.item }
            WidgetFrame { wid: "notes"; slotItem: notesLoader.item }
        }

        Process { id: paletteProc }

        // Menus sit above the whole stage stack (backdrop z 1, layers up to z 5),
        // or a Parallax backdrop paints over an open right-click menu.
        // Built on the first right-click (the sync build hides behind the press)
        // and kept for the session.
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
                WidgetMenu {}
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
                onOpacityChanged: (id, op) => {
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
        // The Edit widgets toolbar (docs/stage.md, "Edit widgets"): one row
        // docked top-centre. The desktop draws the per-widget frames and runs
        // the drag/resize; this owns the toolbar and the Add drop-down.
        StageWidgetsEditor {
            z: 101
            visible: root.stageComposing
            monitor: root.screen ? root.screen.name : ""
            items: root.addItems
            onDone: StageCfg.StageSession.leave()
            onAddToggle: id => root.stageAddToggle(id)
            onVisualizer: {
                StageCfg.StageSession.leave();
                if (!VizCfg.Config.enabled)
                    VizCfg.Config.setEnabled(true);
                if (root.stageState)
                    root.stageState.visualizerPlacing = true;
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
        // Reset (docs/stage.md, "Edit widgets"): restore the snapshot taken when
        // the session opened. Config keys write directly; plugin re-place and
        // unplace commands run one at a time through this Process.
        Connections {
            target: StageCfg.StageSession
            function onResetRequested() {
                if (!root.stageComposing)
                    return;
                root._restore();
            }
        }
        Process {
            id: resetProc
            onRunningChanged: if (!resetProc.running) root._runResetQueue()
        }
    }
}
