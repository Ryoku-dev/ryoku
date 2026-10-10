pragma ComponentBehavior: Bound

import inir
import inir.services
import inir.modules.common
import inir.modules.common.widgets
import inir.modules.common.widgets.widgetCanvas
import inir.modules.common.functions as CF
import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Wayland

import inir.modules.background.widgets
import inir.modules.background.widgets.clock
import inir.modules.background.widgets.mediaControls
import inir.modules.background.widgets.weather
import inir.modules.background.widgets.visualizer
import inir.modules.background.widgets.imageConverter
import inir.modules.background.widgets.systemMonitor
import inir.modules.background.widgets.battery
import inir.modules.background.widgets.notes
import inir.modules.background.widgets.calendar
import inir.modules.background.widgets.todo
import inir.modules.background.widgets.timers
import inir.modules.background.widgets.shape
import inir.modules.background.widgets.dateBadge
import inir.modules.background.widgets.uptime
import inir.modules.background.widgets.controls
import inir.modules.background.widgets.screenTime
import inir.modules.background.widgets.dayProgress
import inir.modules.background.widgets.worldClock
import inir.modules.background.widgets.userCard
import inir.modules.background.widgets.newsTicker
import inir.modules.background.widgets.japaneseTypography
import inir.modules.iris.components
import inir.modules.iris.style
import inir.modules.iris.frame
import "widgets/OrganicEdgeConfig.js" as OrganicEdgeConfig

Scope {
    id: backgroundScope
    property var organicEdgeHosts: ({})

    // Bounded diagnostics for the desktop clock. They are inert unless the
    // supervised shell is loaded with INIR_REGION_DEBUG=1.
    property bool clockDebugRegionActive: false
    property color clockDebugRegionColor: "transparent"
    property real clockDebugRegionBrightness: -1
    property real clockDebugRegionSpread: 0
    property bool clockDebugQuickControlsOpen: false
    property bool clockDebugLayoutProbeActive: false
    property int clockDebugLayoutProbeX: 0
    property int clockDebugLayoutProbeY: 0
    property var _clockDebugSnapshot: null
    property bool _clockDebugEditModeSnapshot: false
    property bool _clockDebugEditModeSnapshotValid: false
    property string clockDebugPaletteReport: "{}"
    property string clockDebugControlsReport: "{}"

    function promoteDesktopWidgetKey(instanceKey: string): var {
        const key = String(instanceKey ?? "")
        if (key.length === 0)
            return Config.getNestedValue("background.widgets.layerOrder", []) ?? []
        const stored = Config.getNestedValue("background.widgets.layerOrder", []) ?? []
        const order = []
        for (let i = 0; i < stored.length; ++i) {
            const candidate = String(stored[i] ?? "")
            if (candidate.length > 0 && candidate !== key
                    && order.indexOf(candidate) === -1)
                order.push(candidate)
        }
        order.push(key)
        Config.setNestedValue("background.widgets.layerOrder", order)
        return order
    }

    function applyOrganicEdgeNamedPreset(presets, name: string, label: string): string {
        const preset = presets.find(p => p.name.toLowerCase() === name.toLowerCase())
        if (!preset) return "Unknown Organic edge " + label.toLowerCase()
        const updates = {}
        for (const key of Object.keys(preset.values))
            updates[OrganicEdgeConfig.path + "." + key] = preset.values[key]
        Config.setNestedValues(updates)
        return "Organic edge " + label + ": " + preset.name
    }

    IpcHandler {
        target: "background"
        function toggleEditMode(): string {
            GlobalStates.setWidgetEditMode(!GlobalStates.widgetEditMode)
            return GlobalStates.widgetEditMode ? "edit mode on" : "edit mode off"
        }

        function toggleWidgetManager(): string {
            if (!GlobalStates.widgetEditMode) GlobalStates.setWidgetEditMode(true)
            GlobalStates.desktopWidgetManagerToggleRequested(GlobalStates.focusedScreen?.name ?? "")
            return "widget manager toggled"
        }

        function setEditMode(enabled: bool): string {
            GlobalStates.setWidgetEditMode(enabled)
            return GlobalStates.widgetEditMode ? "edit mode on" : "edit mode off"
        }

        function editState(): string {
            return JSON.stringify({
                active: GlobalStates.widgetEditMode,
                selected: GlobalStates.selectedDesktopWidget,
                quickControls: GlobalStates.desktopWidgetQuickControls,
                layerOrder: Config.getNestedValue("background.widgets.layerOrder", []) ?? [],
                outputOverrides: Config.options?.background?.widgets?.outputOverrides ?? [],
                outputs: Quickshell.screens.map(screen => ({
                    name: screen?.name ?? "",
                    width: screen?.width ?? 0,
                    height: screen?.height ?? 0,
                    widgetsAllowed: DesktopWidgetLayout.outputAllowed(screen?.name ?? ""),
                    insets: ShellLayoutController.desktopInsets(screen?.name ?? ""),
                    workArea: ShellLayoutController.desktopWorkArea(
                        screen?.name ?? "", screen?.width ?? 0,
                        screen?.height ?? 0),
                    zoneWorkArea: ShellLayoutController.desktopZoneWorkArea(
                        screen?.name ?? "", screen?.width ?? 0,
                        screen?.height ?? 0)
                }))
            })
        }

        function applyOrganicEdgePreset(name: string): string {
            return backgroundScope.applyOrganicEdgeNamedPreset(OrganicEdgeConfig.presets, name, "scene")
        }

        function applyOrganicEdgeComposition(name: string): string {
            return backgroundScope.applyOrganicEdgeNamedPreset(OrganicEdgeConfig.compositionPresets, name, "composition")
        }

        function applyOrganicEdgeMaterial(name: string): string {
            return backgroundScope.applyOrganicEdgeNamedPreset(OrganicEdgeConfig.materialPresets, name, "material")
        }

        function applyOrganicEdgeResponse(name: string): string {
            return backgroundScope.applyOrganicEdgeNamedPreset(OrganicEdgeConfig.responsePresets, name, "response")
        }

        function organicEdgeState(): string {
            return JSON.stringify(Object.values(backgroundScope.organicEdgeHosts)
                .map(host => host.diagnostics()))
        }

        function setOrganicEdgeEnabled(enabled: bool): string {
            Config.setNestedValue("background.edgeWidgets.organic.enable", enabled)
            return enabled ? "Organic edge enabled" : "Organic edge disabled"
        }

        function focusWidget(widgetName: string, openControls: bool): string {
            const name = String(widgetName ?? "").trim()
            if (name.length === 0)
                return "widget name is required"

            const builtinDefaults = ({
                weather: false, clock: true, customImage: false,
                imageConverter: false, mediaControls: false,
                visualizer: false, systemMonitor: false, battery: false,
                notes: false, calendarUpcoming: false, monthCalendar: false,
                todo: false, timers: false, dayProgress: false, uptime: false, shape: false, dateBadge: false, editorial: false,
                newsTicker: false, japaneseTypography: false,
                worldClock: false, userCard: false, controls: false, screenTime: false
            })
            let known = builtinDefaults[name] !== undefined
            let baseEnabled = known
                ? Boolean(Config.getNestedValue(
                    "background.widgets." + name + ".enable",
                    builtinDefaults[name]))
                : false

            if (name.startsWith("custom.")) {
                const customId = name.slice("custom.".length)
                known = CustomWidgets.ready
                    && CustomWidgets.widgets.some(widget => widget.id === customId)
                baseEnabled = known && Boolean(Config.getNestedValue(
                    "background.widgets.custom." + customId + ".enable", false))
            }

            if (!known)
                return "unknown widget: " + name
            if (name === "battery" && !Battery.available)
                return "widget unavailable: " + name

            const screen = GlobalStates.focusedScreen ?? Quickshell.screens[0]
            if (!screen)
                return "no output available"
            if (!DesktopWidgetLayout.enabled(screen.name ?? "", name, baseEnabled))
                return "widget disabled on output: " + (screen.name ?? "")

            const key = (screen.name ?? "") + "::" + name
            GlobalStates.setWidgetEditMode(true)
            if (openControls)
                GlobalStates.requestDesktopWidgetQuickControls(key)
            else
                GlobalStates.selectDesktopWidget(key)
            return key
        }

        function promoteWidget(widgetName: string): string {
            const name = String(widgetName ?? "").trim()
            if (name.length === 0)
                return "widget name is required"
            const screen = GlobalStates.focusedScreen ?? Quickshell.screens[0]
            if (!screen)
                return "no output available"
            const key = (screen.name ?? "") + "::" + name
            return JSON.stringify({
                promoted: key,
                layerOrder: backgroundScope.promoteDesktopWidgetKey(key)
            })
        }

        function resetLayerOrder(): string {
            Config.setNestedValue("background.widgets.layerOrder", [])
            return "widget layer order reset"
        }

        function setWidgetEnabled(widgetName: string, enabled: bool): string {
            const knownWidgets = ["weather", "clock", "customImage", "imageConverter",
                "mediaControls", "visualizer", "systemMonitor", "battery", "notes",
                "calendarUpcoming", "monthCalendar", "todo", "timers", "dayProgress", "uptime", "shape", "dateBadge", "editorial",
                "newsTicker", "japaneseTypography",
                "worldClock", "userCard", "controls", "screenTime"];
            if (!knownWidgets.includes(widgetName))
                return "unknown widget: " + widgetName;
            DesktopWidgetLayout.setGloballyEnabled(widgetName, enabled);
            return widgetName + (enabled ? " enabled" : " disabled");
        }

        function clockDebugState(): string {
            return JSON.stringify({
                enabled: Quickshell.env("INIR_REGION_DEBUG") === "1",
                config: {
                    style: Config.getNestedValue("background.widgets.clock.style", "cookie"),
                    adaptToWallpaper: Config.getNestedValue("background.widgets.clock.digital.adaptToWallpaper", true),
                    placementStrategy: Config.getNestedValue("background.widgets.clock.placementStrategy", "free"),
                    x: Config.getNestedValue("background.widgets.clock.x", 0),
                    y: Config.getNestedValue("background.widgets.clock.y", 0)
                },
                injectedRegion: {
                    active: backgroundScope.clockDebugRegionActive,
                    color: String(backgroundScope.clockDebugRegionColor),
                    brightness: backgroundScope.clockDebugRegionBrightness,
                    spread: backgroundScope.clockDebugRegionSpread
                },
                palette: backgroundScope.clockDebugPaletteReport,
                controls: backgroundScope.clockDebugControlsReport,
                snapshotActive: backgroundScope._clockDebugSnapshot !== null
                    || backgroundScope._clockDebugEditModeSnapshotValid
            });
        }

        function clockDebugSetMode(style: string, adaptToWallpaper: bool): string {
            if (Quickshell.env("INIR_REGION_DEBUG") !== "1")
                return "clock diagnostics disabled; load with INIR_REGION_DEBUG=1";
            if (style !== "digital" && style !== "cookie")
                return "invalid clock style: " + style;
            backgroundScope._captureClockDebugSnapshot();
            let updates = {};
            updates["background.widgets.clock.style"] = style;
            updates["background.widgets.clock.digital.adaptToWallpaper"] = adaptToWallpaper;
            if (style === "cookie") {
                updates["background.widgets.clock.cookie.hourMarks"] = true;
                updates["background.widgets.clock.cookie.timeIndicators"] = true;
                updates["background.widgets.clock.cookie.dialNumberStyle"] = "full";
                updates["background.widgets.clock.cookie.minuteHandStyle"] = "medium";
                updates["background.widgets.clock.cookie.hourHandStyle"] = "fill";
                updates["background.widgets.clock.cookie.secondHandStyle"] = "classic";
                updates["background.widgets.clock.cookie.dateStyle"] = "bubble";
            }
            Config.setNestedValues(updates);
            return style + (adaptToWallpaper ? " adaptive" : " static");
        }

        function clockDebugSetRegion(color: string, brightness: real, spread: real): string {
            if (Quickshell.env("INIR_REGION_DEBUG") !== "1")
                return "clock diagnostics disabled; load with INIR_REGION_DEBUG=1";
            const parsed = Qt.color(color);
            if (!parsed.valid)
                return "invalid color: " + color;
            backgroundScope.clockDebugRegionColor = parsed;
            backgroundScope.clockDebugRegionBrightness = Math.max(0, Math.min(1, brightness));
            backgroundScope.clockDebugRegionSpread = Math.max(0, Math.min(1, spread));
            backgroundScope.clockDebugRegionActive = true;
            return "region injected";
        }

        function clockDebugSetLayout(x: int, y: int, quickControlsOpen: bool): string {
            if (Quickshell.env("INIR_REGION_DEBUG") !== "1")
                return "clock diagnostics disabled; load with INIR_REGION_DEBUG=1";
            if (!backgroundScope._clockDebugEditModeSnapshotValid) {
                backgroundScope._clockDebugEditModeSnapshot = GlobalStates.widgetEditMode;
                backgroundScope._clockDebugEditModeSnapshotValid = true;
            }
            backgroundScope.clockDebugLayoutProbeX = x;
            backgroundScope.clockDebugLayoutProbeY = y;
            backgroundScope.clockDebugLayoutProbeActive = true;
            GlobalStates.setWidgetEditMode(true);
            backgroundScope.clockDebugQuickControlsOpen = quickControlsOpen;
            return "layout probe requested";
        }

        function clockDebugRestore(): string {
            backgroundScope.clockDebugRegionActive = false;
            backgroundScope.clockDebugQuickControlsOpen = false;
            backgroundScope.clockDebugLayoutProbeActive = false;
            if (backgroundScope._clockDebugSnapshot !== null) {
                Config.setNestedValues(backgroundScope._clockDebugSnapshot);
                backgroundScope._clockDebugSnapshot = null;
            }
            if (backgroundScope._clockDebugEditModeSnapshotValid) {
                GlobalStates.setWidgetEditMode(backgroundScope._clockDebugEditModeSnapshot);
                backgroundScope._clockDebugEditModeSnapshotValid = false;
            }
            return "clock diagnostics restored";
        }
    }

    function _captureClockDebugSnapshot(): void {
        if (backgroundScope._clockDebugSnapshot !== null)
            return;
        const prefix = "background.widgets.clock";
        let snapshot = {};
        snapshot[prefix + ".style"] = Config.getNestedValue(prefix + ".style", "cookie");
        snapshot[prefix + ".digital.adaptToWallpaper"] = Config.getNestedValue(prefix + ".digital.adaptToWallpaper", true);
        snapshot[prefix + ".cookie.hourMarks"] = Config.getNestedValue(prefix + ".cookie.hourMarks", false);
        snapshot[prefix + ".cookie.timeIndicators"] = Config.getNestedValue(prefix + ".cookie.timeIndicators", true);
        snapshot[prefix + ".cookie.dialNumberStyle"] = Config.getNestedValue(prefix + ".cookie.dialNumberStyle", "none");
        snapshot[prefix + ".cookie.minuteHandStyle"] = Config.getNestedValue(prefix + ".cookie.minuteHandStyle", "medium");
        snapshot[prefix + ".cookie.hourHandStyle"] = Config.getNestedValue(prefix + ".cookie.hourHandStyle", "fill");
        snapshot[prefix + ".cookie.secondHandStyle"] = Config.getNestedValue(prefix + ".cookie.secondHandStyle", "dot");
        snapshot[prefix + ".cookie.dateStyle"] = Config.getNestedValue(prefix + ".cookie.dateStyle", "bubble");
        backgroundScope._clockDebugSnapshot = snapshot;
    }

    Variants {
        id: root
        model: Quickshell.screens

    PanelWindow {
        id: bgRoot

        required property var modelData

        // Hide the widget canvas while a fullscreen window owns the output. The
        // seam answers this per output on whichever compositor is running.
        readonly property bool hasFullscreenWindow: CompositorService.nativeOverview
            && GameMode.hasFullscreenOnOutput(bgRoot.modelData?.name ?? "")
            && !WorkspaceService.inOverview
        visible: GlobalStates.screenLocked
            || !hasFullscreenWindow
            || !(Config.options?.background?.hideWhenFullscreen ?? false)

        readonly property string screenName: screen?.name ?? ""
        function _widgetConfigValue(widgetKey: string, key: string, fallback: var): var {
            return DesktopWidgetLayout.value(bgRoot.screenName, widgetKey, key, fallback);
        }
        function _widgetEnabled(widgetKey: string, fallback: bool): bool {
            return DesktopWidgetLayout.enabled(bgRoot.screenName, widgetKey, fallback);
        }

        function _loadedWidget(widgetName: string): var {
            if (!widgetCanvas || typeof widgetCanvas._loadedDesktopWidgets !== "function")
                return null
            return widgetCanvas._loadedDesktopWidgets().find(item =>
                String(item?.configEntryName ?? "") === widgetName) ?? null
        }

        // An OnDemand layer that took keyboard focus (a click on the desktop)
        // keeps it across workspace switches and focus-window actions on some
        // compositors, so the new workspace's window never receives focus and the
        // Dock needs a second click. Dropping to None for a moment hands focus back
        // to the compositor's focused window; re-arming a mapped surface never grabs it.
        property bool _keyboardReleased: false
        function releaseKeyboard(): void {
            if (bgRoot._menuOpen || (!bgRoot._needsKeyboardFocus && !bgRoot._keyboardReleased)) return
            bgRoot._keyboardReleased = true
            keyboardRearm.restart()
        }
        Timer { id: keyboardRearm; interval: 120; onTriggered: bgRoot._keyboardReleased = false }
        Connections {
            target: WorkspaceService
            function onFocusedWorkspaceIdChanged(): void { bgRoot.releaseKeyboard() }
            function onActiveWindowChanged(): void { bgRoot.releaseKeyboard() }
        }

        // True if any widget on this background needs keyboard input (sticky notes
        // today, future text-entry widgets later). Used to flip the layer-shell
        // surface to focusable=true so TextEdits actually receive key events.
        // Without this the Bottom layer is keyboard-inert and clicks reach the
        // TextEdit but typing does nothing.
        // A desktop menu is a grabbing popup of this surface: its parent's keyboard
        // mode never changes while one is open.
        readonly property bool _menuOpen: irisDesktopMenu.active
        // Once a desktop menu has closed, the keyboard its right-click took goes back too.
        on_MenuOpenChanged: if (!bgRoot._menuOpen) Qt.callLater(bgRoot.releaseKeyboard)
        readonly property bool _needsKeyboardFocus: GlobalStates.deferredPanelsReady
            && (bgRoot._menuOpen || !bgRoot._keyboardReleased)
            && (GlobalStates.widgetEditMode
                || bgRoot._widgetEnabled("notes", false)
                || bgRoot._widgetEnabled("todo", false))

        // Zone occupancy: map zone name → array of widget names
        readonly property var _builtinWidgets: [
            { key: "weather",            defaultOn: false, icon: "cloud" },
            { key: "clock",              defaultOn: true,  icon: "schedule" },
            { key: "customImage",        defaultOn: false, icon: "add_photo_alternate" },
            { key: "imageConverter",     defaultOn: false, icon: "transform" },
            { key: "mediaControls",      defaultOn: false, icon: "album" },
            { key: "visualizer",         defaultOn: false, icon: "graphic_eq" },
            { key: "systemMonitor",      defaultOn: false, icon: "monitor_heart" },
            { key: "battery",            defaultOn: false, icon: "battery_full" },
            { key: "notes",              defaultOn: false, icon: "sticky_note_2" },
            { key: "calendarUpcoming",   defaultOn: false, icon: "event" },
            { key: "monthCalendar",      defaultOn: false, icon: "calendar_month" },
            { key: "todo",               defaultOn: false, icon: "checklist" },
            { key: "timers",             defaultOn: false, icon: "timer" },
            { key: "uptime",             defaultOn: false, icon: "avg_pace" },
            { key: "shape", defaultOn: false, icon: "category" },
            { key: "dateBadge", defaultOn: false, icon: "today" },
            { key: "editorial", defaultOn: false, icon: "text_fields" },
            { key: "newsTicker",         defaultOn: false, icon: "newspaper" },
            { key: "japaneseTypography", defaultOn: false, icon: "translate" },
            { key: "worldClock",         defaultOn: false, icon: "public" },
            { key: "userCard",           defaultOn: false, icon: "account_circle" },
            { key: "controls",           defaultOn: false, icon: "toggle_on" },
            { key: "screenTime",         defaultOn: false, icon: "hourglass_bottom" }
        ]
        // Revision counter to force re-evaluation
        property int _zoneRevision: 0
        Connections {
            target: Config
            function onConfigChanged() { bgRoot._zoneRevision++ }
        }
        function _computeZoneOccupants(): var {
            void bgRoot._zoneRevision; // bind to revision
            const zones = ["topLeft", "topCenter", "topRight", "centerLeft", "center", "centerRight", "bottomLeft", "bottomCenter", "bottomRight"];
            let occ = {};
            for (const z of zones) occ[z] = [];
            for (const w of bgRoot._builtinWidgets) {
                if (!bgRoot._widgetEnabled(w.key, w.defaultOn)) continue;
                const strat = bgRoot._widgetConfigValue(w.key, "placementStrategy", "free");
                if (zones.indexOf(strat) >= 0)
                    occ[strat].push({ name: w.key, icon: w.icon, locked: Boolean(bgRoot._widgetConfigValue(w.key, "locked", false)) });
            }
            // Custom widgets
            if (typeof CustomWidgets !== "undefined" && CustomWidgets.ready) {
                const list = CustomWidgets.widgets;
                for (let i = 0; i < list.length; i++) {
                    const cw = list[i];
                    if (!Config.getNestedValue("background.widgets.custom." + cw.id + ".enable", false)) continue;
                    const strat = Config.getNestedValue("background.widgets.custom." + cw.id + ".placementStrategy", "free");
                    if (zones.indexOf(strat) >= 0)
                        occ[strat].push({ name: cw.name || cw.id, icon: cw.icon || "widgets", locked: Boolean(Config.getNestedValue("background.widgets.custom." + cw.id + ".locked", false)) });
                }
            }
            return occ;
        }
        readonly property var zoneOccupants: _computeZoneOccupants()

        // Dynamic focus based on windows — drives the widget canvas dynamic
        // opacity so widgets fade back while windows own the active workspace.
        readonly property bool hasWindowsOnCurrentWorkspace:
            CompositorService.hasWindowsOnActiveWorkspace(bgRoot.modelData?.name ?? "")

        property bool focusWindowsPresent: !GlobalStates.screenLocked && hasWindowsOnCurrentWorkspace
        property real focusPresenceProgress: focusWindowsPresent ? 1 : 0
        Behavior on focusPresenceProgress {
            enabled: Appearance.animationsEnabled
            animation: NumberAnimation { duration: Appearance.animation.elementMoveFast.duration; easing.type: Appearance.animation.elementMoveFast.type; easing.bezierCurve: Appearance.animation.elementMoveFast.bezierCurve }
        }

        // Layer props
        screen: modelData
        exclusionMode: ExclusionMode.Ignore
        // ryogami paints the wallpaper on the Background layer below. This surface
        // sits above it, transparent, and only carries the desktop-widget canvas.
        // Keep it behind the lock surface; moving to Overlay can capture input.
        WlrLayershell.layer: WlrLayer.Bottom
        WlrLayershell.namespace: "quickshell:background"
        // Host for LiveLayer (continuous motion in a widget moves to a small surface of its own instead of
        // repainting this whole output every frame): nothing here covers or moves a still widget outside edit.
        readonly property bool liveCalm: !GlobalStates.widgetEditMode && !GlobalStates.shellLayoutEditMode
            && !GlobalStates.screenLocked
        readonly property int liveLayer: WlrLayer.Bottom
        readonly property int liveEpoch: 0
        // Map the desktop keyboard-inert during startup, then arm OnDemand after
        // the first-frame/deferred lifecycle has settled. A compositor can
        // temporarily focus a newly mapped OnDemand layer surface during shell
        // restart, which loses the previously focused app. Changing an
        // already-mapped surface to OnDemand is safe and still lets Notes/Todo
        // receive keyboard input.
        WlrLayershell.keyboardFocus: bgRoot._needsKeyboardFocus
            ? WlrKeyboardFocus.OnDemand
            : WlrKeyboardFocus.None
        anchors { top: true; bottom: true; left: true; right: true }
        color: "transparent"

        Item {
            anchors.fill: parent

            FocusScope {
                id: desktopFocusSink
                width: 0
                height: 0
                focus: false
            }

            // Desktop right-click context menu
            MouseArea {
                anchors.fill: parent
                z: 15  // Below WidgetCanvas (z: 20) so widgets can receive input
                // Left button too, so a click on the bare desktop closes an
                // already-open menu.
                acceptedButtons: Qt.RightButton | Qt.LeftButton
                onClicked: function(mouse) {
                    if (mouse.button === Qt.LeftButton) {
                        desktopFocusSink.forceActiveFocus()
                        // A click on the bare desktop is not typing: the compositor
                        // gave this OnDemand surface the keyboard for it, so hand it
                        // straight back to the window that had it.
                        bgRoot.releaseKeyboard()
                        if (irisDesktopMenu.active) irisDesktopMenu.close()
                        return
                    }
                    desktopMenuAnchor.x = mouse.x
                    desktopMenuAnchor.y = mouse.y
                    irisDesktopMenu.requestOpen()
                }
            }

            Item {
                id: desktopMenuAnchor
                z: 26
                width: 1; height: 1
            }

            // The Island's desktop button asks for this screen's menu: the same
            // open the pointer's right-click takes, anchored at the request point.
            Connections {
                target: GlobalStates
                function onIrisDesktopMenuRequested(outputName: string, x: real, y: real): void {
                    if (outputName !== bgRoot.screenName) return
                    desktopMenuAnchor.x = x
                    desktopMenuAnchor.y = y
                    irisDesktopMenu.requestOpen()
                }
            }

            // iRiS desktop menu: the Island's material, quick-action tiles and
            // keyboard, growing out of the pointer.
            IrisDesktopMenu {
                id: irisDesktopMenu
                z: 27
                anchorItem: desktopMenuAnchor
                readonly property int gridSize: Config.getNestedValue("background.widgets.editGrid.size", 32)
                readonly property bool gridSnap: Config.getNestedValue("background.widgets.editGrid.snap", true)
                model: GlobalStates.widgetEditMode ? [
                    { type: "quick", items: [
                        { text: Translation.tr("Widgets"), iconName: "dashboard_customize",
                            action: () => { widgetManagerPanel.shown = true } },
                        { text: irisDesktopMenu.gridSnap ? Translation.tr("Snap on") : Translation.tr("Snap off"),
                            iconName: irisDesktopMenu.gridSnap ? "grid_on" : "grid_off",
                            action: () => Config.setNestedValue("background.widgets.editGrid.snap", !irisDesktopMenu.gridSnap) },
                        { text: Translation.tr("Done"), iconName: "check", accent: true,
                            action: () => { widgetManagerPanel.shown = false; GlobalStates.setWidgetEditMode(false) } }
                    ] },
                    { type: "separator" },
                    { text: Translation.tr("Grid size"), iconName: "grid_4x4", detail: irisDesktopMenu.gridSize + " px",
                        action: () => {
                            const sizes = [16, 32, 48, 64]
                            Config.setNestedValue("background.widgets.editGrid.size",
                                sizes[(sizes.indexOf(irisDesktopMenu.gridSize) + 1) % sizes.length])
                        } },
                    { text: Translation.tr("Widget settings"), iconName: "settings",
                        action: () => GlobalStates.openSettingsPage(14) }
                ] : [
                    { type: "quick", items: [
                        { text: Translation.tr("Wallpaper"), iconName: "wallpaper",
                            image: Wallpapers.effectiveWallpaperPath ?? "",
                            action: () => { GlobalStates.wallpaperSelectorOpen = true } },
                        { text: Translation.tr("Widgets"), iconName: "widgets",
                            action: () => GlobalStates.setWidgetEditMode(true) },
                        { text: Translation.tr("Studio"), iconName: "palette",
                            action: () => { GlobalStates.irisStudioOpen = true } },
                        { text: Translation.tr("Search"), iconName: "search",
                            action: () => { GlobalStates.searchOpen = true } }
                    ] },
                    { type: "separator" },
                    { text: Translation.tr("Edit Shima"), iconName: "edit",
                        action: () => { GlobalStates.irisEdit = true } },
                    { text: Translation.tr("Quick controls"), iconName: "tune",
                        action: () => { GlobalStates.controlPanelOpen = true } },
                    { text: Translation.tr("Settings"), iconName: "settings",
                        action: () => { GlobalStates.openSettings() } },
                    { type: "separator" },
                    { text: Translation.tr("Reload shell"), iconName: "refresh",
                        action: () => { Quickshell.execDetached(["ryoku-shell", "reload"]) } }
                ]
            }

            OrganicEdgeWidget {
                id: organicEdge
                Component.onCompleted: backgroundScope.organicEdgeHosts[screenName] = organicEdge
                Component.onDestruction: delete backgroundScope.organicEdgeHosts[screenName]
                anchors.fill: parent
                z: 19
                screenName: modelData?.name ?? ""
            }

            WidgetCanvas {
                id: widgetCanvas
                z: 20
                visible: !GlobalStates.shellLayoutEditMode
                    && DesktopWidgetLayout.outputAllowed(modelData?.name ?? "")
                enabled: visible && !GlobalStates.screenLocked  // Disable all widget input during lock
                opacity: {
                    const dynOp = Math.max(0, Math.min(100, Number(Config.options?.background?.widgets?.dynamicOpacity) || 0));
                    if (dynOp <= 0 || !bgRoot.focusWindowsPresent) return 1;
                    return 1 - (dynOp / 100) * bgRoot.focusPresenceProgress;
                }
                Behavior on opacity {
                    enabled: Appearance.animationsEnabled
                    animation: NumberAnimation { duration: Appearance.animation.elementMoveFast.duration; easing.type: Appearance.animation.elementMoveFast.type; easing.bezierCurve: Appearance.animation.elementMoveFast.bezierCurve }
                }
                anchors {
                    left: parent.left
                    right: parent.right
                    top: parent.top
                    bottom: parent.bottom
                }
                width: parent.width
                height: parent.height

                // The canvas owns layer discovery. Individual widgets should not
                // walk the visual tree themselves: loaders, repeater delegates and
                // custom widgets all live here, and this is the only place with a
                // complete view of the current output.
                function _loadedDesktopWidgets(): var {
                    const widgets = []
                    for (let i = 0; i < widgetCanvas.children.length; ++i) {
                        const holder = widgetCanvas.children[i]
                        const item = holder?.item ?? holder
                        if (!item || item.editInstanceKey === undefined || !item.visible)
                            continue
                        widgets.push(item)
                    }
                    return widgets
                }

                // The widget lit as a drop target while another is carried over it (iRiS stacks).
                property string stackHint: ""

                function loadedWidget(instanceKey: string): var {
                    return widgetCanvas._loadedDesktopWidgets().find(item => item.editInstanceKey === instanceKey) ?? null
                }

                // The stackable widget most covered by the one being carried, or "" when none is covered enough.
                function stackDropCandidate(instanceKey: string): string {
                    const widgets = widgetCanvas._loadedDesktopWidgets()
                    const carried = widgets.find(item => item.editInstanceKey === instanceKey)
                    if (!carried || !carried.stackable)
                        return ""
                    const area = Math.max(1, carried.width * carried.height)
                    let best = ""
                    let bestShare = 0.4
                    for (const other of widgets) {
                        if (other === carried || !other.stackable || (carried.stacked && other.stacked))
                            continue
                        const across = Math.min(carried.x + carried.width, other.x + other.width) - Math.max(carried.x, other.x)
                        const down = Math.min(carried.y + carried.height, other.y + other.height) - Math.max(carried.y, other.y)
                        if (across <= 0 || down <= 0)
                            continue
                        const share = across * down / Math.min(area, Math.max(1, other.width * other.height))
                        if (share > bestShare) {
                            bestShare = share
                            best = other.editInstanceKey
                        }
                    }
                    return best
                }

                function _rectOverlaps(a, b, gap): bool {
                    return a.x < b.x + b.width + gap
                        && a.x + a.width + gap > b.x
                        && a.y < b.y + b.height + gap
                        && a.y + a.height + gap > b.y
                }

                function _positionIsFree(x, y, width, height, placed, gap): bool {
                    const candidate = { x: x, y: y, width: width, height: height }
                    for (const rect of placed) {
                        if (widgetCanvas._rectOverlaps(candidate, rect, gap))
                            return false
                    }
                    return true
                }

                function _nearestFreePosition(item, desiredX, desiredY, placed, work): var {
                    const left = Number(work.left ?? 0)
                    const top = Number(work.top ?? 0)
                    const right = Number(work.right ?? widgetCanvas.width)
                    const bottom = Number(work.bottom ?? widgetCanvas.height)
                    const maxX = Math.max(left, right - item.width)
                    const maxY = Math.max(top, bottom - item.height)
                    const startX = Math.max(left, Math.min(maxX, desiredX))
                    const startY = Math.max(top, Math.min(maxY, desiredY))
                    const gap = 14
                    if (widgetCanvas._positionIsFree(
                            startX, startY, item.width, item.height, placed, gap))
                        return { x: Math.round(startX), y: Math.round(startY) }

                    const step = 24
                    let best = null
                    let bestDistance = Infinity
                    function consider(x, y): void {
                        const px = Math.max(left, Math.min(maxX, x))
                        const py = Math.max(top, Math.min(maxY, y))
                        if (!widgetCanvas._positionIsFree(
                                px, py, item.width, item.height, placed, gap))
                            return
                        const dx = px - startX
                        const dy = py - startY
                        const distance = dx * dx + dy * dy
                        if (distance < bestDistance) {
                            bestDistance = distance
                            best = { x: Math.round(px), y: Math.round(py) }
                        }
                    }
                    for (let y = top; y <= maxY; y += step) {
                        for (let x = left; x <= maxX; x += step)
                            consider(x, y)
                    }
                    consider(maxX, top)
                    consider(left, maxY)
                    consider(maxX, maxY)
                    return best ?? { x: Math.round(startX), y: Math.round(startY) }
                }

                function initializeOutputWidgetLayout(): void {
                    if (!Config.ready || !widgetCanvas.visible)
                        return
                    const outputName = String(bgRoot.screen?.name ?? "")
                    const outputWidth = Math.round(widgetCanvas.width)
                    const outputHeight = Math.round(widgetCanvas.height)
                    if (!outputName || outputWidth <= 0 || outputHeight <= 0)
                        return

                    const widgets = widgetCanvas._loadedDesktopWidgets()
                        .filter(item => item.width > 0 && item.height > 0)
                    if (widgets.length === 0) {
                        if (widgetCanvas._outputLayoutAttempts < 8) {
                            widgetCanvas._outputLayoutAttempts++
                            outputLayoutTimer.restart()
                        }
                        return
                    }

                    const geometryChanged = !DesktopWidgetLayout.outputLayoutMatches(
                        outputName, outputWidth, outputHeight)
                    let missingGeometry = false
                    for (const item of widgets) {
                        const strategy = String(item.placementStrategy ?? "free")
                        if (DesktopWidgetStacks.isSplit(item.configEntryName)
                                || (strategy === "free"
                                && (!DesktopWidgetLayout.hasValue(outputName,
                                        item.configEntryName, "x")
                                    || !DesktopWidgetLayout.hasValue(outputName,
                                        item.configEntryName, "y")))) {
                            missingGeometry = true
                            break
                        }
                    }
                    if (!geometryChanged && !missingGeometry)
                        return

                    const work = ShellLayoutController.desktopZoneWorkArea(
                        outputName, outputWidth, outputHeight)
                    const ordered = widgets.slice().sort((a, b) => {
                        const aLocal = DesktopWidgetLayout.hasValue(
                            outputName, a.configEntryName, "x") ? 1 : 0
                        const bLocal = DesktopWidgetLayout.hasValue(
                            outputName, b.configEntryName, "x") ? 1 : 0
                        if (!geometryChanged && aLocal !== bLocal)
                            return bLocal - aLocal
                        // A widget that just left a stack is the one that moves aside.
                        const aSplit = DesktopWidgetStacks.isSplit(a.configEntryName)
                        if (aSplit !== DesktopWidgetStacks.isSplit(b.configEntryName))
                            return aSplit ? 1 : -1
                        if (Boolean(a.locked) !== Boolean(b.locked))
                            return a.locked ? -1 : 1
                        return b.width * b.height - a.width * a.height
                    })
                    const placed = []
                    const updates = ({})
                    const left = Number(work.left ?? 0)
                    const top = Number(work.top ?? 0)
                    const right = Number(work.right ?? outputWidth)
                    const bottom = Number(work.bottom ?? outputHeight)
                    const leaving = ({})
                    const remembered = ({})
                    const restored = ({})
                    for (const item of widgets) {
                        const spot = geometryChanged
                            ? DesktopWidgetLayout.rememberedPosition(
                                outputName, item.configEntryName, outputWidth, outputHeight) : null
                        remembered[item.configEntryName] = spot
                        if (spot?.x !== undefined) {
                            // Kept where it was arranged on this size, held only by the output's edges: the
                            // work area can be narrower than where it was placed (over the Dock's band), and
                            // clamping to it moved a widget on the way back.
                            restored[item.configEntryName] = {
                                x: Math.round(Math.max(0, Math.min(Math.max(0, outputWidth - item.width), spot.x))),
                                y: Math.round(Math.max(0, Math.min(Math.max(0, outputHeight - item.height), spot.y)))
                            }
                            placed.push({ x: restored[item.configEntryName].x,
                                y: restored[item.configEntryName].y, width: item.width, height: item.height })
                        }
                    }
                    for (const item of widgets) {
                        const was = String(item.placementStrategy ?? "free")
                        leaving[item.configEntryName] = { placementStrategy: was }
                        if (was === "free") {
                            leaving[item.configEntryName].x = Number(DesktopWidgetLayout.value(
                                outputName, item.configEntryName, "x", item.x))
                            leaving[item.configEntryName].y = Number(DesktopWidgetLayout.value(
                                outputName, item.configEntryName, "y", item.y))
                        }
                    }

                    for (const item of ordered) {
                        const strategy = String(item.placementStrategy ?? "free")
                        const maxX = Math.max(left, right - item.width)
                        const maxY = Math.max(top, bottom - item.height)
                        const localX = DesktopWidgetLayout.hasValue(
                            outputName, item.configEntryName, "x")
                        const localY = DesktopWidgetLayout.hasValue(
                            outputName, item.configEntryName, "y")
                        const spot = remembered[item.configEntryName]
                        const target = spot?.placementStrategy ?? strategy
                        if (target !== "free" && target !== strategy) {
                            updates[item.configEntryName] = { placementStrategy: target }
                            placed.push({ x: item.x, y: item.y, width: item.width, height: item.height })
                            continue
                        }
                        let wantX = Number(item.x) || 0
                        let wantY = Number(item.y) || 0
                        if (geometryChanged && target === "free" && localX && localY) {
                            const storedX = Number(DesktopWidgetLayout.value(
                                outputName, item.configEntryName, "x", wantX))
                            const storedY = Number(DesktopWidgetLayout.value(
                                outputName, item.configEntryName, "y", wantY))
                            if (spot?.x !== undefined) {
                                wantX = spot.x
                                wantY = spot.y
                            } else if (previousGeometry && Number.isFinite(storedX)
                                    && Number.isFinite(storedY)) {
                                wantX = (storedX + item.width / 2) / previousGeometry.width
                                    * outputWidth - item.width / 2
                                wantY = (storedY + item.height / 2) / previousGeometry.height
                                    * outputHeight - item.height / 2
                            }
                        }
                        const desiredX = Math.max(left, Math.min(maxX, wantX))
                        const desiredY = Math.max(top, Math.min(maxY, wantY))
                        const needsLocal = target === "free"
                            && (geometryChanged || !localX || !localY)
                        let position = restored[item.configEntryName]
                            ?? { x: Math.round(desiredX), y: Math.round(desiredY) }
                        const collides = !restored[item.configEntryName] && !widgetCanvas._positionIsFree(
                            position.x, position.y, item.width, item.height, placed, 14)
                        if (collides && (!item.locked || DesktopWidgetStacks.isSplit(item.configEntryName)))
                            position = widgetCanvas._nearestFreePosition(
                                item, desiredX, desiredY, placed, work)

                        const moved = Math.round(position.x) !== Math.round(item.x)
                            || Math.round(position.y) !== Math.round(item.y)
                        if (needsLocal || moved || (collides && (!item.locked || DesktopWidgetStacks.isSplit(item.configEntryName)))) {
                            updates[item.configEntryName] = {
                                x: position.x,
                                y: position.y,
                                placementStrategy: "free"
                            }
                        }
                        if (!restored[item.configEntryName])
                            placed.push({
                                x: position.x,
                                y: position.y,
                                width: item.width,
                                height: item.height
                            })
                    }

                    widgetCanvas._outputLayoutAttempts = 0
                    DesktopWidgetLayout.initializeOutputLayout(
                        outputName, outputWidth, outputHeight, updates, geometryChanged ? leaving : null)
                    DesktopWidgetStacks.clearSplits()
                }

                property int _outputLayoutAttempts: 0

                Timer {
                    id: outputLayoutTimer
                    interval: DesktopWidgetStacks.splitPending ? 250 : 1400
                    repeat: false
                    onTriggered: widgetCanvas.initializeOutputWidgetLayout()
                }

                Component.onCompleted: outputLayoutTimer.restart()

                Connections {
                    target: Config
                    function onRevisionChanged(): void {
                        if (!outputLayoutTimer.running)
                            outputLayoutTimer.restart()
                    }
                }

                Connections {
                    target: bgRoot.screen
                    function onWidthChanged(): void { outputLayoutTimer.restart() }
                    function onHeightChanged(): void { outputLayoutTimer.restart() }
                }

                function overlappingDesktopWidgets(instanceKey: string): var {
                    const widgets = widgetCanvas._loadedDesktopWidgets()
                    const current = widgets.find(item => item.editInstanceKey === instanceKey)
                    if (!current || current.width <= 0 || current.height <= 0)
                        return []
                    // The pages of one stack are one layer, not several piled up.
                    const matches = widgets.filter(item => item.width > 0 && item.height > 0
                        && (item === current || !current.stacked || item.stack?.id !== current.stack.id)
                        && item.x < current.x + current.width
                        && item.x + item.width > current.x
                        && item.y < current.y + current.height
                        && item.y + item.height > current.y)
                    // Topmost first. The selected widget has a temporary edit z,
                    // so use the persistent z when deciding which underlying
                    // widget should be promoted next.
                    matches.sort((a, b) => {
                        const order = Number(b.desktopPersistentZ ?? b.widgetIndex ?? 0)
                            - Number(a.desktopPersistentZ ?? a.widgetIndex ?? 0)
                        return order !== 0 ? order
                            : String(a.editInstanceKey).localeCompare(String(b.editInstanceKey))
                    })
                    return matches
                }

                function overlappingDesktopWidgetCount(instanceKey: string): int {
                    return widgetCanvas.overlappingDesktopWidgets(instanceKey).length
                }

                function cycleOverlappingDesktopWidget(instanceKey: string): string {
                    const matches = widgetCanvas.overlappingDesktopWidgets(instanceKey)
                    if (matches.length < 2)
                        return instanceKey
                    const current = matches.findIndex(item => item.editInstanceKey === instanceKey)
                    if (current < 0)
                        return instanceKey
                    const next = matches[(current + 1) % matches.length]
                    const nextKey = String(next?.editInstanceKey ?? instanceKey)
                    backgroundScope.promoteDesktopWidgetKey(nextKey)
                    GlobalStates.selectDesktopWidget(nextKey)
                    if (Quickshell.env("INIR_REGION_DEBUG") === "1")
                        console.debug("[WidgetEdit] layer promote", instanceKey, "->", nextKey,
                            "overlaps=", matches.length)
                    return nextKey
                }

                function promoteDesktopWidget(instanceKey: string, layerKey: string): string {
                    const key = String(instanceKey ?? "")
                    if (!key)
                        return ""
                    backgroundScope.promoteDesktopWidgetKey(String(layerKey ?? "") || key)
                    GlobalStates.selectDesktopWidget(key)
                    return key
                }

                // ── Edit Mode Scrim ──────────────────────────────
                Rectangle {
                    anchors.fill: parent
                    z: -2
                    visible: opacity > 0
                    opacity: GlobalStates.widgetEditMode ? 1 : 0
                    color: Qt.rgba(0, 0, 0, 0.15)
                    Behavior on opacity {
                        enabled: Appearance.animationsEnabled
                        NumberAnimation { duration: Appearance.animation.elementMoveEnter.duration; easing.type: Appearance.animation.elementMoveEnter.type; easing.bezierCurve: Appearance.animation.elementMoveEnter.bezierCurve }
                    }
                }

                // ── Edit Mode Overlay ─────────────────────────────
                Item {
                    id: editGridOverlay
                    anchors.fill: parent
                    visible: opacity > 0
                    opacity: GlobalStates.widgetEditMode ? 1 : 0
                    z: -1

                    Behavior on opacity {
                        enabled: Appearance.animationsEnabled
                        NumberAnimation { duration: Appearance.animation.elementMoveEnter.duration; easing.type: Appearance.animation.elementMoveEnter.type; easing.bezierCurve: Appearance.animation.elementMoveEnter.bezierCurve }
                    }

                    readonly property int gridSize: Config.getNestedValue("background.widgets.editGrid.size", 32)
                    readonly property bool gridVisible: Config.getNestedValue("background.widgets.editGrid.snap", true)
                    readonly property color gridColor: Appearance.angelEverywhere ? Appearance.angel.colPrimary
                        : Appearance.inirEverywhere ? Appearance.inir.colAccent
                        : Appearance.auroraEverywhere ? Appearance.colors.colPrimary
                        : Appearance.colors.colPrimary
                    readonly property color crosshairColor: Appearance.angelEverywhere ? Appearance.angel.colTertiary
                        : Appearance.inirEverywhere ? Appearance.inir.colTertiary
                        : Appearance.auroraEverywhere ? Appearance.colors.colTertiary
                        : Appearance.colors.colTertiary
                    readonly property var workArea: ShellLayoutController.desktopWorkArea(
                        bgRoot.screen?.name ?? "", width, height)
                    readonly property var zoneWorkArea: ShellLayoutController.desktopZoneWorkArea(
                        bgRoot.screen?.name ?? "", width, height)
                    readonly property int zoneMargin: 16
                    readonly property real safeLeft: workArea.left ?? 0
                    readonly property real safeTop: workArea.top ?? 0
                    readonly property real safeRight: workArea.right ?? width
                    readonly property real safeBottom: workArea.bottom ?? height
                    readonly property real safeWidth: workArea.width ?? 0
                    readonly property real safeHeight: workArea.height ?? 0
                    readonly property real zoneLeft: zoneWorkArea.left ?? safeLeft
                    readonly property real zoneTop: zoneWorkArea.top ?? safeTop
                    readonly property real zoneWidth: zoneWorkArea.width ?? safeWidth
                    readonly property real zoneHeight: zoneWorkArea.height ?? safeHeight
                    readonly property bool hasSelection: GlobalStates.selectedDesktopWidget
                        .startsWith((bgRoot.screen?.name ?? "") + "::")
                    readonly property bool manipulating: {
                        if (!hasSelection) return false
                        const widget = bgRoot._loadedWidget(GlobalStates.selectedDesktopWidget.split("::")[1])
                        return widget !== null && (widget.isDragging || widget._isResizing)
                    }

                    // Grid dots at intersections. The lattice uses the same
                    // panel-aware bounds as drag snapping, so moving the bar or
                    // dock changes both the visible guide and the committed
                    // position instead of leaving two competing coordinate systems.
                    readonly property bool gridNonDefault: gridSize !== 32
                    Canvas {
                        id: editGridCanvas
                        x: editGridOverlay.zoneLeft
                        y: editGridOverlay.zoneTop
                        width: editGridOverlay.zoneWidth
                        height: editGridOverlay.zoneHeight
                        visible: editGridOverlay.gridVisible
                        onPaint: {
                            const ctx = getContext("2d");
                            ctx.clearRect(0, 0, width, height);
                            if (width <= 0 || height <= 0) return;
                            const gs = editGridOverlay.gridSize;
                            const dotColor = editGridOverlay.gridColor;
                            const custom = editGridOverlay.gridNonDefault;
                            const alpha = custom ? 0.18 : 0.10;
                            const dotR = custom ? 1.8 : 1.4;
                            ctx.fillStyle = Qt.rgba(dotColor.r, dotColor.g, dotColor.b, alpha);
                            const cols = Math.floor(width / gs) + 1;
                            const rows = Math.floor(height / gs) + 1;
                            for (let r = 0; r < rows; ++r) {
                                for (let c = 0; c < cols; ++c) {
                                    ctx.beginPath();
                                    ctx.arc(c * gs, r * gs, dotR, 0, 2 * Math.PI);
                                    ctx.fill();
                                }
                            }
                            // Subtle grid lines for non-default sizes
                            if (custom) {
                                ctx.strokeStyle = Qt.rgba(dotColor.r, dotColor.g, dotColor.b, 0.05);
                                ctx.lineWidth = 0.5;
                                for (let c = 0; c < cols; ++c) {
                                    ctx.beginPath();
                                    ctx.moveTo(c * gs, 0);
                                    ctx.lineTo(c * gs, height);
                                    ctx.stroke();
                                }
                                for (let r = 0; r < rows; ++r) {
                                    ctx.beginPath();
                                    ctx.moveTo(0, r * gs);
                                    ctx.lineTo(width, r * gs);
                                    ctx.stroke();
                                }
                            }
                        }
                        onVisibleChanged: if (visible && available) requestPaint()
                        onWidthChanged: if (available) requestPaint()
                        onHeightChanged: if (available) requestPaint()
                        Component.onCompleted: requestPaint()
                        Connections {
                            target: editGridOverlay
                            function onGridSizeChanged() { editGridCanvas.requestPaint() }
                            function onGridNonDefaultChanged() { editGridCanvas.requestPaint() }
                            function onGridColorChanged() { editGridCanvas.requestPaint() }
                            function onWidthChanged() { editGridCanvas.requestPaint() }
                            function onHeightChanged() { editGridCanvas.requestPaint() }
                        }
                        Connections {
                            target: GlobalStates
                            function onWidgetEditModeChanged() {
                                if (GlobalStates.widgetEditMode && editGridCanvas.available)
                                    editGridCanvas.requestPaint();
                            }
                        }
                    }

                    Rectangle {
                        x: editGridOverlay.zoneLeft
                        y: editGridOverlay.zoneTop
                        width: editGridOverlay.zoneWidth
                        height: editGridOverlay.zoneHeight
                        color: "transparent"
                        radius: Appearance.rounding.small
                        border.width: 1
                        border.color: CF.ColorUtils.applyAlpha(editGridOverlay.gridColor, 0.18)
                    }

                    // Crosshair follows the adaptive panel-safe widget area.
                    Rectangle {
                        x: Math.floor(editGridOverlay.zoneLeft + editGridOverlay.zoneWidth / 2)
                        y: editGridOverlay.zoneTop
                        width: 1; height: editGridOverlay.zoneHeight
                        color: CF.ColorUtils.applyAlpha(editGridOverlay.crosshairColor, 0.08)
                    }
                    Rectangle {
                        x: editGridOverlay.zoneLeft
                        y: Math.floor(editGridOverlay.zoneTop + editGridOverlay.zoneHeight / 2)
                        width: editGridOverlay.zoneWidth; height: 1
                        color: CF.ColorUtils.applyAlpha(editGridOverlay.crosshairColor, 0.08)
                    }
                    // Center dot
                    Rectangle {
                        x: Math.floor(editGridOverlay.zoneLeft + editGridOverlay.zoneWidth / 2) - 3
                        y: Math.floor(editGridOverlay.zoneTop + editGridOverlay.zoneHeight / 2) - 3
                        width: 6; height: 6; radius: 3
                        color: CF.ColorUtils.applyAlpha(editGridOverlay.crosshairColor, 0.25)
                    }

                    // ── Snap Zone Indicators (3x3 grid) ──────────────
                    Repeater {
                        model: [
                            { zone: "topLeft",      col: 0, row: 0 },
                            { zone: "topCenter",    col: 1, row: 0 },
                            { zone: "topRight",     col: 2, row: 0 },
                            { zone: "centerLeft",   col: 0, row: 1 },
                            { zone: "center",       col: 1, row: 1 },
                            { zone: "centerRight",  col: 2, row: 1 },
                            { zone: "bottomLeft",   col: 0, row: 2 },
                            { zone: "bottomCenter", col: 1, row: 2 },
                            { zone: "bottomRight",  col: 2, row: 2 }
                        ]
                        delegate: Rectangle {
                            id: zoneRect
                            required property var modelData
                            readonly property int col: modelData.col
                            readonly property int row: modelData.row
                            readonly property real zw: (editGridOverlay.zoneWidth - editGridOverlay.zoneMargin * 2) / 3
                            readonly property real zh: (editGridOverlay.zoneHeight - editGridOverlay.zoneMargin * 2) / 3
                            readonly property var occupants: bgRoot.zoneOccupants[modelData.zone] ?? []
                            readonly property bool occupied: occupants.length > 0
                            readonly property bool hasLocked: {
                                for (let i = 0; i < occupants.length; i++)
                                    if (occupants[i].locked) return true;
                                return false;
                            }

                            x: editGridOverlay.zoneLeft + editGridOverlay.zoneMargin + col * zw + 4
                            y: editGridOverlay.zoneTop + editGridOverlay.zoneMargin + row * zh + 4
                            width: zw - 8
                            height: zh - 8
                            radius: Appearance.rounding.small
                            visible: editGridOverlay.manipulating
                            opacity: 0.65
                            color: occupied
                                ? CF.ColorUtils.applyAlpha(hasLocked ? Appearance.colors.colError : editGridOverlay.gridColor, 0.04)
                                : "transparent"
                            border {
                                width: occupied ? 1.5 : 1
                                color: CF.ColorUtils.applyAlpha(
                                    hasLocked ? Appearance.colors.colError : editGridOverlay.gridColor,
                                    occupied ? 0.25 : 0.10)
                            }
                            Behavior on opacity {
                                enabled: Appearance.animationsEnabled
                                NumberAnimation { duration: Appearance.animation.elementMoveFast.duration }
                            }
                            Behavior on color { enabled: Appearance.animationsEnabled; ColorAnimation { duration: 200 } }
                            Behavior on border.color { enabled: Appearance.animationsEnabled; ColorAnimation { duration: 200 } }

                            // Zone content: arrow + occupant icons
                            Column {
                                anchors.centerIn: parent
                                spacing: 4

                                // Direction arrow
                                Text {
                                    anchors.horizontalCenter: parent.horizontalCenter
                                    text: {
                                        const labels = {
                                            topLeft: "↖", topCenter: "↑", topRight: "↗",
                                            centerLeft: "←", center: "⊙", centerRight: "→",
                                            bottomLeft: "↙", bottomCenter: "↓", bottomRight: "↘"
                                        };
                                        return labels[zoneRect.modelData.zone] ?? "";
                                    }
                                    font.pixelSize: zoneRect.occupied ? 14 : 16
                                    color: CF.ColorUtils.applyAlpha(editGridOverlay.gridColor, zoneRect.occupied ? 0.35 : 0.20)
                                }

                                // Occupant widget icons
                                Row {
                                    visible: zoneRect.occupied
                                    anchors.horizontalCenter: parent.horizontalCenter
                                    spacing: 4
                                    Repeater {
                                        model: zoneRect.occupants
                                        MaterialSymbol {
                                            required property var modelData
                                            text: modelData.icon
                                            iconSize: 14
                                            color: CF.ColorUtils.applyAlpha(
                                                modelData.locked ? Appearance.colors.colError : editGridOverlay.gridColor, 0.45)
                                        }
                                    }
                                }
                            }
                        }
                    }
                }

                MouseArea {
                    anchors.fill: parent
                    z: 0
                    enabled: GlobalStates.widgetEditMode
                    acceptedButtons: Qt.LeftButton
                    onClicked: GlobalStates.clearDesktopWidgetSelection()
                }

                Item {
                    id: editControlsOverlay
                    anchors.fill: parent
                    visible: opacity > 0
                    opacity: GlobalStates.widgetEditMode ? 1 : 0
                    z: 20000
                    enabled: GlobalStates.widgetEditMode

                    Behavior on opacity {
                        enabled: Appearance.animationsEnabled
                        NumberAnimation { duration: Appearance.animation.elementMoveEnter.duration; easing.type: Appearance.animation.elementMoveEnter.type; easing.bezierCurve: Appearance.animation.elementMoveEnter.bezierCurve }
                    }

                    DesktopEditToolbar {
                        id: editControlsBar
                        availableWidth: Math.max(0, editGridOverlay.safeWidth - 16)
                        availableHeight: editGridOverlay.safeHeight
                        outputName: bgRoot.screenName
                        hasSelection: editGridOverlay.hasSelection
                        libraryOpen: widgetManagerPanel.shown
                        x: Math.round(editGridOverlay.safeLeft
                            + (editGridOverlay.safeWidth - width) / 2)
                        // iRiS: the Dock steps aside while editing and the toolbar takes the
                        // edge opposite the Island, so it never lands on the Island.
                        readonly property bool irisTopEdge: (Config.options?.panelFamily ?? "ii") === "iris"
                            && editGridOverlay.workArea?.insets?.barEdge === "bottom"
                        attachedTopEdge: irisTopEdge
                        y: editControlsBar.iris ? (irisTopEdge ? IrisFrame.band : parent.height - height - IrisFrame.band)
                            : Math.max(editGridOverlay.safeTop, editGridOverlay.safeBottom - height - 12)
                        onLibraryRequested: widgetManagerPanel.shown = !widgetManagerPanel.shown
                        onEdgeSettingsRequested: GlobalStates.openSettingsPage(14, "Organic edge")
                        onSettingsRequested: GlobalStates.openSettingsPage(14)
                        onDoneRequested: {
                            widgetManagerPanel.shown = false
                            GlobalStates.setWidgetEditMode(false)
                        }
                    }

                    // ── Widget Manager Panel ─────────────────────────
                    Loader {
                        id: widgetManagerPanel
                        property bool shown: false
                        property bool geometryReady: false
                        Connections {
                            target: GlobalStates
                            function onDesktopWidgetManagerToggleRequested(outputName: string): void {
                                if (outputName.length === 0 || outputName === bgRoot.screenName)
                                    widgetManagerPanel.shown = !widgetManagerPanel.shown
                            }
                            function onWidgetEditModeChanged(): void {
                                if (!GlobalStates.widgetEditMode) widgetManagerPanel.shown = false
                            }
                        }
                        active: shown
                        visible: shown
                        enabled: shown && (!editControlsBar.iris || geometryReady)
                        opacity: editControlsBar.iris && !geometryReady ? 0 : 1
                        z: 150
                        x: 0
                        y: 0

                        Behavior on opacity {
                            enabled: editControlsBar.iris && IrisStyle.motionEnabled
                            NumberAnimation {
                                duration: IrisStyle.revealDuration
                                easing.type: Easing.BezierSpline
                                easing.bezierCurve: IrisStyle.morphCurve
                            }
                        }

                        function restoreGeometry(): void {
                            if (!widgetManagerPanel.shown)
                                return
                            Qt.callLater(() => {
                                const panel = widgetManagerPanel.item
                                if (!panel)
                                    return
                                const inset = 12
                                const canvasWidth = widgetManagerPanel.parent?.width ?? 0
                                const canvasHeight = widgetManagerPanel.parent?.height ?? 0
                                const spanX = Math.max(0, canvasWidth - panel.width - inset * 2)
                                const spanY = Math.max(0, canvasHeight - panel.height - inset * 2)
                                const rx = Math.max(0, Math.min(1,
                                    Number(Persistent.states?.desktopWidgets?.managerXRatio ?? 0.68)))
                                const ry = Math.max(0, Math.min(1,
                                    Number(Persistent.states?.desktopWidgets?.managerYRatio ?? 0.48)))
                                widgetManagerPanel.x = inset + Math.round(spanX * rx)
                                widgetManagerPanel.y = inset + Math.round(spanY * ry)
                                widgetManagerPanel.geometryReady = true
                            })
                        }

                        onShownChanged: {
                            if (!shown) {
                                geometryReady = false
                                return
                            }
                            geometryReady = false
                            restoreGeometry()
                        }
                        onLoaded: restoreGeometry()

                        sourceComponent: WidgetManagerPanel {
                            outputName: bgRoot.screen?.name ?? ""
                            canvasWidth: widgetManagerPanel.parent?.width ?? 800
                            canvasHeight: widgetManagerPanel.parent?.height ?? 600
                            screenWidth: bgRoot.screen.width
                            screenHeight: bgRoot.screen.height
                            onCloseRequested: widgetManagerPanel.shown = false
                            onFocusWidgetRequested: layoutKey => {
                                GlobalStates.selectDesktopWidget(
                                    bgRoot.screenName + "::" + layoutKey)
                            }
                            Component.onCompleted: widgetManagerPanel.restoreGeometry()
                        }
                    }
                }

                FadeLoader {
                    shown: bgRoot._widgetEnabled("weather", false)
                    z: item?.desktopStackZ ?? 0
                    containmentMask: GlobalStates.widgetEditMode ? _hitMask : null
                    WidgetInputMask { id: _hitMask; loader: parent }
                    sourceComponent: WeatherWidget {
                        widgetIndex: 0
                        outputName: bgRoot.screen?.name ?? ""
                        screenWidth: bgRoot.screen.width
                        screenHeight: bgRoot.screen.height
                        scaledScreenWidth: bgRoot.screen.width
                        scaledScreenHeight: bgRoot.screen.height
                        wallpaperScale: 1
                    }
                }

                FadeLoader {
                    shown: bgRoot._widgetEnabled("customImage", false)
                    z: item?.desktopStackZ ?? 0
                    containmentMask: GlobalStates.widgetEditMode ? _hitMaskCustomImage : null
                    WidgetInputMask { id: _hitMaskCustomImage; loader: parent }
                    sourceComponent: CustomImageWidget {
                        widgetIndex: 16
                        outputName: bgRoot.screen?.name ?? ""
                        screenWidth: bgRoot.screen.width
                        screenHeight: bgRoot.screen.height
                        scaledScreenWidth: bgRoot.screen.width
                        scaledScreenHeight: bgRoot.screen.height
                        wallpaperScale: 1
                    }
                }

                FadeLoader {
                    shown: bgRoot._widgetEnabled("imageConverter", false)
                    z: item?.desktopStackZ ?? 0
                    containmentMask: GlobalStates.widgetEditMode ? _hitMaskImageConverter : null
                    WidgetInputMask { id: _hitMaskImageConverter; loader: parent }
                    sourceComponent: ImageConverterWidget {
                        widgetIndex: 17
                        outputName: bgRoot.screen?.name ?? ""
                        screenWidth: bgRoot.screen.width
                        screenHeight: bgRoot.screen.height
                        scaledScreenWidth: bgRoot.screen.width
                        scaledScreenHeight: bgRoot.screen.height
                        wallpaperScale: 1
                    }
                }

                FadeLoader {
                    shown: bgRoot._widgetEnabled("clock", true)
                    z: item?.desktopStackZ ?? 0
                    containmentMask: GlobalStates.widgetEditMode ? _hitMask2 : null
                    WidgetInputMask { id: _hitMask2; loader: parent }
                    sourceComponent: ClockWidget {
                        widgetIndex: 1
                        outputName: bgRoot.screen?.name ?? ""
                        screenWidth: bgRoot.screen.width
                        screenHeight: bgRoot.screen.height
                        scaledScreenWidth: bgRoot.screen.width
                        scaledScreenHeight: bgRoot.screen.height
                        wallpaperScale: 1
                        debugRegionActive: backgroundScope.clockDebugRegionActive
                        debugRegionColor: backgroundScope.clockDebugRegionColor
                        debugRegionBrightness: backgroundScope.clockDebugRegionBrightness
                        debugRegionSpread: backgroundScope.clockDebugRegionSpread
                        debugQuickControlsOpen: backgroundScope.clockDebugQuickControlsOpen
                        debugLayoutProbeActive: backgroundScope.clockDebugLayoutProbeActive
                        debugLayoutProbeX: backgroundScope.clockDebugLayoutProbeX
                        debugLayoutProbeY: backgroundScope.clockDebugLayoutProbeY
                        onDebugPaletteReportChanged: backgroundScope.clockDebugPaletteReport = debugPaletteReport
                        onEditControlsGeometryReportChanged: backgroundScope.clockDebugControlsReport = editControlsGeometryReport
                    }
                }

                FadeLoader {
                    shown: bgRoot._widgetEnabled("mediaControls", false)
                    z: item?.desktopStackZ ?? 0
                    containmentMask: GlobalStates.widgetEditMode ? _hitMask3 : null
                    WidgetInputMask { id: _hitMask3; loader: parent }
                    sourceComponent: MediaControlsWidget {
                        widgetIndex: 2
                        outputName: bgRoot.screen?.name ?? ""
                        screenWidth: bgRoot.screen.width
                        screenHeight: bgRoot.screen.height
                        scaledScreenWidth: bgRoot.screen.width
                        scaledScreenHeight: bgRoot.screen.height
                        wallpaperScale: 1
                    }
                }

                FadeLoader {
                    shown: bgRoot._widgetEnabled("visualizer", false)
                    z: item?.desktopStackZ ?? 0
                    containmentMask: GlobalStates.widgetEditMode ? _hitMask4 : null
                    WidgetInputMask { id: _hitMask4; loader: parent }
                    sourceComponent: VisualizerWidget {
                        widgetIndex: 3
                        outputName: bgRoot.screen?.name ?? ""
                        screenWidth: bgRoot.screen.width
                        screenHeight: bgRoot.screen.height
                        scaledScreenWidth: bgRoot.screen.width
                        scaledScreenHeight: bgRoot.screen.height
                        wallpaperScale: 1
                    }
                }

                FadeLoader {
                    shown: bgRoot._widgetEnabled("systemMonitor", false)
                    z: item?.desktopStackZ ?? 0
                    containmentMask: GlobalStates.widgetEditMode ? _hitMask5 : null
                    WidgetInputMask { id: _hitMask5; loader: parent }
                    sourceComponent: SystemMonitorWidget {
                        widgetIndex: 4
                        outputName: bgRoot.screen?.name ?? ""
                        screenWidth: bgRoot.screen.width
                        screenHeight: bgRoot.screen.height
                        scaledScreenWidth: bgRoot.screen.width
                        scaledScreenHeight: bgRoot.screen.height
                        wallpaperScale: 1
                    }
                }

                FadeLoader {
                    shown: bgRoot._widgetEnabled("battery", false) && Battery.available
                    z: item?.desktopStackZ ?? 0
                    containmentMask: GlobalStates.widgetEditMode ? _hitMask6 : null
                    WidgetInputMask { id: _hitMask6; loader: parent }
                    sourceComponent: BatteryWidget {
                        widgetIndex: 5
                        outputName: bgRoot.screen?.name ?? ""
                        screenWidth: bgRoot.screen.width
                        screenHeight: bgRoot.screen.height
                        scaledScreenWidth: bgRoot.screen.width
                        scaledScreenHeight: bgRoot.screen.height
                        wallpaperScale: 1
                    }
                }

                FadeLoader {
                    shown: bgRoot._widgetEnabled("notes", false)
                    z: item?.desktopStackZ ?? 0
                    containmentMask: GlobalStates.widgetEditMode ? _hitMask7 : null
                    WidgetInputMask { id: _hitMask7; loader: parent }
                    sourceComponent: NotesWidget {
                        widgetIndex: 6
                        outputName: bgRoot.screen?.name ?? ""
                        screenWidth: bgRoot.screen.width
                        screenHeight: bgRoot.screen.height
                        scaledScreenWidth: bgRoot.screen.width
                        scaledScreenHeight: bgRoot.screen.height
                        wallpaperScale: 1
                    }
                }

                FadeLoader {
                    shown: bgRoot._widgetEnabled("calendarUpcoming", false)
                    z: item?.desktopStackZ ?? 0
                    containmentMask: GlobalStates.widgetEditMode ? _hitMask8 : null
                    WidgetInputMask { id: _hitMask8; loader: parent }
                    sourceComponent: CalendarUpcomingWidget {
                        widgetIndex: 7
                        outputName: bgRoot.screen?.name ?? ""
                        screenWidth: bgRoot.screen.width
                        screenHeight: bgRoot.screen.height
                        scaledScreenWidth: bgRoot.screen.width
                        scaledScreenHeight: bgRoot.screen.height
                        wallpaperScale: 1
                    }
                }

                FadeLoader {
                    shown: bgRoot._widgetEnabled("monthCalendar", false)
                    z: item?.desktopStackZ ?? 0
                    containmentMask: GlobalStates.widgetEditMode ? _hitMaskMonthCalendar : null
                    WidgetInputMask { id: _hitMaskMonthCalendar; loader: parent }
                    sourceComponent: MonthCalendarWidget {
                        widgetIndex: 8
                        outputName: bgRoot.screen?.name ?? ""
                        screenWidth: bgRoot.screen.width
                        screenHeight: bgRoot.screen.height
                        scaledScreenWidth: bgRoot.screen.width
                        scaledScreenHeight: bgRoot.screen.height
                        wallpaperScale: 1
                    }
                }

                FadeLoader {
                    shown: bgRoot._widgetEnabled("todo", false)
                    z: item?.desktopStackZ ?? 0
                    containmentMask: GlobalStates.widgetEditMode ? _hitMaskTodo : null
                    WidgetInputMask { id: _hitMaskTodo; loader: parent }
                    sourceComponent: TodoWidget {
                        widgetIndex: 10
                        outputName: bgRoot.screen?.name ?? ""
                        screenWidth: bgRoot.screen.width
                        screenHeight: bgRoot.screen.height
                        scaledScreenWidth: bgRoot.screen.width
                        scaledScreenHeight: bgRoot.screen.height
                        wallpaperScale: 1
                    }
                }

                FadeLoader {
                    shown: bgRoot._widgetEnabled("timers", false)
                    z: item?.desktopStackZ ?? 0
                    containmentMask: GlobalStates.widgetEditMode ? _hitMaskTimers : null
                    WidgetInputMask { id: _hitMaskTimers; loader: parent }
                    sourceComponent: TimerWidget {
                        widgetIndex: 18
                        outputName: bgRoot.screen?.name ?? ""
                        screenWidth: bgRoot.screen.width
                        screenHeight: bgRoot.screen.height
                        scaledScreenWidth: bgRoot.screen.width
                        scaledScreenHeight: bgRoot.screen.height
                        wallpaperScale: 1
                    }
                }

                FadeLoader {
                    shown: bgRoot._widgetEnabled("dayProgress", false)
                    z: item?.desktopStackZ ?? 0
                    containmentMask: GlobalStates.widgetEditMode ? _hitMaskDayProgress : null
                    WidgetInputMask { id: _hitMaskDayProgress; loader: parent }
                    sourceComponent: DayProgressWidget {
                        widgetIndex: 8
                        outputName: bgRoot.screen?.name ?? ""
                        screenWidth: bgRoot.screen.width
                        screenHeight: bgRoot.screen.height
                        scaledScreenWidth: bgRoot.screen.width
                        scaledScreenHeight: bgRoot.screen.height
                        wallpaperScale: 1
                    }
                }

                FadeLoader {
                    shown: bgRoot._widgetEnabled("uptime", false)
                    z: item?.desktopStackZ ?? 0
                    containmentMask: GlobalStates.widgetEditMode ? _hitMask10 : null
                    WidgetInputMask { id: _hitMask10; loader: parent }
                    sourceComponent: UptimeWidget {
                        widgetIndex: 9
                        outputName: bgRoot.screen?.name ?? ""
                        screenWidth: bgRoot.screen.width
                        screenHeight: bgRoot.screen.height
                        scaledScreenWidth: bgRoot.screen.width
                        scaledScreenHeight: bgRoot.screen.height
                        wallpaperScale: 1
                    }
                }

                FadeLoader {
                    shown: bgRoot._widgetEnabled("controls", false) && (Config.options?.panelFamily ?? "ii") === "iris"
                    z: item?.desktopStackZ ?? 0
                    containmentMask: GlobalStates.widgetEditMode ? _hitMaskControls : null
                    WidgetInputMask { id: _hitMaskControls; loader: parent }
                    sourceComponent: ControlsWidget {
                        widgetIndex: 19
                        outputName: bgRoot.screen?.name ?? ""
                        screenWidth: bgRoot.screen.width
                        screenHeight: bgRoot.screen.height
                        scaledScreenWidth: bgRoot.screen.width
                        scaledScreenHeight: bgRoot.screen.height
                        wallpaperScale: 1
                    }
                }

                FadeLoader {
                    shown: bgRoot._widgetEnabled("screenTime", false) && (Config.options?.panelFamily ?? "ii") === "iris"
                    z: item?.desktopStackZ ?? 0
                    containmentMask: GlobalStates.widgetEditMode ? _hitMaskScreenTime : null
                    WidgetInputMask { id: _hitMaskScreenTime; loader: parent }
                    sourceComponent: ScreenTimeWidget {
                        widgetIndex: 39
                        outputName: bgRoot.screen?.name ?? ""
                        screenWidth: bgRoot.screen.width
                        screenHeight: bgRoot.screen.height
                        scaledScreenWidth: bgRoot.screen.width
                        scaledScreenHeight: bgRoot.screen.height
                        wallpaperScale: 1
                    }
                }

                FadeLoader {
                    shown: bgRoot._widgetEnabled("editorial", false)
                    z: item?.desktopStackZ ?? 0
                    containmentMask: GlobalStates.widgetEditMode ? editorialHitMask : null
                    WidgetInputMask { id: editorialHitMask; loader: parent }
                    sourceComponent: EditorialWidget {
                        widgetIndex: 22
                        outputName: bgRoot.screen?.name ?? ""
                        screenWidth: bgRoot.screen.width
                        screenHeight: bgRoot.screen.height
                        scaledScreenWidth: bgRoot.screen.width
                        scaledScreenHeight: bgRoot.screen.height
                        wallpaperScale: 1
                    }
                }

                FadeLoader {
                    shown: bgRoot._widgetEnabled("shape", false)
                    z: item?.desktopStackZ ?? 0
                    containmentMask: GlobalStates.widgetEditMode ? _hitMaskshape : null
                    WidgetInputMask { id: _hitMaskshape; loader: parent }
                    sourceComponent: ShapeWidget {
                        widgetIndex: 20
                        outputName: bgRoot.screen?.name ?? ""
                        screenWidth: bgRoot.screen.width
                        screenHeight: bgRoot.screen.height
                        scaledScreenWidth: bgRoot.screen.width
                        scaledScreenHeight: bgRoot.screen.height
                        wallpaperScale: 1
                    }
                }

                FadeLoader {
                    shown: bgRoot._widgetEnabled("dateBadge", false)
                    z: item?.desktopStackZ ?? 0
                    containmentMask: GlobalStates.widgetEditMode ? _hitMaskdateBadge : null
                    WidgetInputMask { id: _hitMaskdateBadge; loader: parent }
                    sourceComponent: DateBadgeWidget {
                        widgetIndex: 21
                        outputName: bgRoot.screen?.name ?? ""
                        screenWidth: bgRoot.screen.width
                        screenHeight: bgRoot.screen.height
                        scaledScreenWidth: bgRoot.screen.width
                        scaledScreenHeight: bgRoot.screen.height
                        wallpaperScale: 1
                    }
                }


                FadeLoader {
                    shown: bgRoot._widgetEnabled("worldClock", false)
                    z: item?.desktopStackZ ?? 0
                    containmentMask: GlobalStates.widgetEditMode ? _hitMask15 : null
                    WidgetInputMask { id: _hitMask15; loader: parent }
                    sourceComponent: WorldClockWidget {
                        widgetIndex: 14
                        outputName: bgRoot.screen?.name ?? ""
                        screenWidth: bgRoot.screen.width
                        screenHeight: bgRoot.screen.height
                        scaledScreenWidth: bgRoot.screen.width
                        scaledScreenHeight: bgRoot.screen.height
                        wallpaperScale: 1
                    }
                }

                FadeLoader {
                    shown: bgRoot._widgetEnabled("userCard", false)
                    z: item?.desktopStackZ ?? 0
                    containmentMask: GlobalStates.widgetEditMode ? _hitMask16 : null
                    WidgetInputMask { id: _hitMask16; loader: parent }
                    sourceComponent: UserCardWidget {
                        widgetIndex: 15
                        outputName: bgRoot.screen?.name ?? ""
                        screenWidth: bgRoot.screen.width
                        screenHeight: bgRoot.screen.height
                        scaledScreenWidth: bgRoot.screen.width
                        scaledScreenHeight: bgRoot.screen.height
                        wallpaperScale: 1
                    }
                }

                FadeLoader {
                    shown: bgRoot._widgetEnabled("newsTicker", false)
                    z: item?.desktopStackZ ?? 0
                    containmentMask: GlobalStates.widgetEditMode ? _hitMask12 : null
                    WidgetInputMask { id: _hitMask12; loader: parent }
                    sourceComponent: NewsTickerWidget {
                        widgetIndex: 11
                        outputName: bgRoot.screen?.name ?? ""
                        screenWidth: bgRoot.screen.width
                        screenHeight: bgRoot.screen.height
                        scaledScreenWidth: bgRoot.screen.width
                        scaledScreenHeight: bgRoot.screen.height
                        wallpaperScale: 1
                    }
                }

                FadeLoader {
                    shown: bgRoot._widgetEnabled("japaneseTypography", false)
                    z: item?.desktopStackZ ?? 0
                    containmentMask: GlobalStates.widgetEditMode ? _hitMask14 : null
                    WidgetInputMask { id: _hitMask14; loader: parent }
                    sourceComponent: JapaneseTypographyWidget {
                        widgetIndex: 13
                        outputName: bgRoot.screen?.name ?? ""
                        screenWidth: bgRoot.screen.width
                        screenHeight: bgRoot.screen.height
                        scaledScreenWidth: bgRoot.screen.width
                        scaledScreenHeight: bgRoot.screen.height
                        wallpaperScale: 1
                    }
                }

                // Custom user widgets from ~/.config/inir/widgets/
                Repeater {
                    model: CustomWidgets.ready ? CustomWidgets.widgets : []

                    Loader {
                        id: customWidgetLoader
                        z: item?.desktopStackZ ?? 0
                        containmentMask: GlobalStates.widgetEditMode ? _customHitMask : null
                        WidgetInputMask { id: _customHitMask; loader: parent }
                        required property var modelData
                        required property int index

                        active: false

                        function _configEnabled(): bool {
                            return DesktopWidgetLayout.enabled(bgRoot.screenName,
                                "custom." + modelData.id,
                                Config.getNestedValue("background.widgets.custom." + modelData.id + ".enable", false));
                        }

                        // setSource passes required properties at construction time
                        function _load(): void {
                            const props = {
                                widgetIndex: 40 + index,
                                outputName: bgRoot.screen?.name ?? "",
                                screenWidth: bgRoot.screen.width,
                                screenHeight: bgRoot.screen.height,
                                scaledScreenWidth: bgRoot.screen.width,
                                scaledScreenHeight: bgRoot.screen.height,
                                wallpaperScale: 1,
                            };
                            // Pass manifest data for auto-popover and resize
                            if (modelData.configKeys && Object.keys(modelData.configKeys).length > 0)
                                props.manifestConfigKeys = modelData.configKeys;
                            // Default to uniform resize via widgetScale for all custom widgets
                            const axes = (modelData.resizableAxes && Object.keys(modelData.resizableAxes).length > 0)
                                ? modelData.resizableAxes : { uniform: "widgetScale" };
                            props.resizableAxes = axes;
                            active = true;
                            setSource(modelData.qmlPath, props);
                        }

                        function _unload(): void {
                            active = false;
                            source = "";
                        }

                        function _syncLoaded(): void {
                            if (_configEnabled()) {
                                if (!item)
                                    _load();
                            } else if (item || active) {
                                _unload();
                            }
                        }

                        Component.onCompleted: Qt.callLater(_syncLoaded)

                        Connections {
                            target: Config
                            function onConfigChanged() {
                                Qt.callLater(customWidgetLoader._syncLoaded);
                            }
                        }

                        Connections {
                            target: bgRoot.screen
                            function onWidthChanged() {
                                if (!customWidgetLoader.item) return;
                                customWidgetLoader.item.screenWidth = bgRoot.screen.width;
                                customWidgetLoader.item.scaledScreenWidth = bgRoot.screen.width;
                            }
                            function onHeightChanged() {
                                if (!customWidgetLoader.item) return;
                                customWidgetLoader.item.screenHeight = bgRoot.screen.height;
                                customWidgetLoader.item.scaledScreenHeight = bgRoot.screen.height;
                            }
                        }
                    }
                }

            }
        }
    }
    }
}
