pragma Singleton
pragma ComponentBehavior: Bound

import inir.modules.common
import inir.services
import QtQuick
import Quickshell
import Quickshell.Io
import shell.services as Ryoku

Singleton {
    id: root
    // Shell entry animation gate — starts false, set true after delay so panels slide in
    property bool shellEntryReady: false
    // Deferred panel loading gate — non-critical panels wait for this before activating
    property bool deferredPanelsReady: false
    property bool barOpen: true
    property bool sidebarLeftOpen: false
    property string sidebarLeftTargetOutput: ""
    property bool sidebarRightOpen: false
    property string sidebarRightTargetOutput: ""
    property bool mediaControlsOpen: false
    property bool irisEqualizerOpen: false
    property string irisEqualizerTargetOutput: ""

    function openIrisEqualizer(outputName): void {
        irisEqualizerTargetOutput = root.resolveOutputName(outputName, [])
        irisEqualizerOpen = true
    }

    function closeIrisEqualizer(): void {
        irisEqualizerOpen = false
    }

    function toggleIrisEqualizer(outputName): void {
        if (irisEqualizerOpen)
            closeIrisEqualizer()
        else
            openIrisEqualizer(outputName)
    }
    property real irisLevelQuietUntil: 0
    function quietIrisLevels(): void { root.irisLevelQuietUntil = Date.now() + 700 }
    property bool osdBrightnessOpen: false
    property bool osdVolumeOpen: false
    property bool osdMicOpen: false
    property bool osdMediaOpen: false
    signal osdMediaActionTriggered(string action)
    readonly property bool userMediaFeedback: Config.options?.panelFamily === "iris"
        || (Config.options?.osd?.mediaEnabled ?? true)

    function showMediaAction(action: string): void {
        const normalized = String(action ?? "")
        if (!["play", "pause", "next", "previous"].includes(normalized))
            return
        root.osdMediaOpen = true
        root.osdMediaActionTriggered(normalized)
    }

    Connections {
        target: Ryoku.OsdFeed
        function onPreviewSeqChanged(): void { root.osdVolumeOpen = true }
        function onVolumeRequested(): void { root.osdVolumeOpen = true }
        function onBrightnessRequested(): void { root.osdBrightnessOpen = true }
        function onMicRequested(): void { root.osdMicOpen = true }
        function onKeyboardRequested(): void { root.osdKeyboardLayoutOpen = true }
        function onMediaRequested(action: string): void { root.showMediaAction(action) }
        function onHideRequested(): void {
            root.osdVolumeOpen = false
            root.osdBrightnessOpen = false
            root.osdMicOpen = false
            root.osdMediaOpen = false
            root.osdKeyboardLayoutOpen = false
        }
    }

    property bool osdKeyboardLayoutOpen: false
    property bool oskOpen: false
    property int activeContextMenuCount: 0
    property var activeContextMenu: null
    property bool settingsOverlayOpen: false
    property bool settingsWindowBehind: false
    property int settingsRaiseRequest: 0
    property int settingsOverlayRequestedPage: -1 // Set before opening to navigate to a specific page
    property string settingsOverlayRequestedSection: ""
    property int settingsOverlayCurrentPage: -1 // Published by whichever overlay chrome is loaded
    // True only while the shell asks Niri for internal window-preview frames.
    property bool windowPreviewCaptureActive: false
    // Non-zero while a native file chooser is up over the settings overlay;
    // the overlay steps down a layer so the chooser is never under it.
    property var _settingsNativeDialogs: ({})
    readonly property bool settingsNativeDialogOpen:
        Object.keys(root._settingsNativeDialogs).length > 0

    function setSettingsNativeDialogVisible(key: string, visible: bool): void {
        const next = Object.assign({}, root._settingsNativeDialogs)
        if (visible) next[key] = true
        else delete next[key]
        root._settingsNativeDialogs = next
    }
    function raiseSettingsIfNeeded(): void {
        if (root.settingsOverlayOpen && root.settingsWindowBehind)
            root.settingsRaiseRequest += 1
    }

    // The frame's own settings overlay. Ryoku Hub owns every non-frame
    // setting page, so the family chrome opens locally and deep-links the
    // requested section; the legacy page index is ignored.
    function openSettingsPage(index: int, section): void {
        root.raiseSettingsIfNeeded()
        root.settingsOverlayRequestedPage = -1
        root.settingsOverlayRequestedSection = String(section ?? "")
        root.settingsOverlayOpen = true
    }

    function openSettings(): void {
        root.raiseSettingsIfNeeded()
        root.settingsOverlayRequestedPage = -1
        root.settingsOverlayRequestedSection = ""
        root.settingsOverlayOpen = true
    }

    property bool screenLocked: false
    property bool sessionOpen: false

    // ── Region capture (screenshot / record / OCR / search) ────────────────
    // ryoshot owns the region-selector front now, so every entry point launches
    // that separate config instead of an in-shell overlay. A bind-spawned qs
    // surface does not inherit the daemon's shared QML module path, so set it
    // here the same way the compositor binds do. An action preselects the front
    // tool (matching ryoshot's RYOSHOT_ACTION); omit it for the plain tool.
    function launchRegionCapture(action): void {
        const qmlPath = Quickshell.env("HOME") + "/.local/lib/qt6/qml"
        const args = ["env", "QML_IMPORT_PATH=" + qmlPath, "QML2_IMPORT_PATH=" + qmlPath]
        if (action && action.length > 0)
            args.push("RYOSHOT_ACTION=" + action)
        args.push("ryoku-cmd-ryoshot")
        Quickshell.execDetached(args)
    }

    // ── Wallpaper picker ──────────────────────────────────────────────────
    property bool wallpaperSelectorOpen: false
    property string wallpaperSelectorSource: ""
    property string wallpaperSelectorQuery: ""
    // Selection targets: "main", "backdrop" (the picker keeps the vocabulary;
    // the Ryoku wallpaper path only acts on "main").
    property string wallpaperSelectionTarget: "main"
    // Target monitor for the picker (set before opening, avoids config timing).
    property string wallpaperSelectorTargetMonitor: ""
    onWallpaperSelectorOpenChanged: {
        if (!wallpaperSelectorOpen) {
            wallpaperSelectionTarget = "main"
            wallpaperSelectorTargetMonitor = ""
            if (Config.options?.wallpaperSelector?.selectionTarget
                    && Config.options.wallpaperSelector.selectionTarget !== "main")
                Config.setNestedValue("wallpaperSelector.selectionTarget", "main")
            if (Config.options?.wallpaperSelector?.targetMonitor)
                Config.setNestedValue("wallpaperSelector.targetMonitor", "")
        }
    }
    property string wallpaperSelectorKind: ""
    property var wallpaperSelectorSeries: null
    property string wallpaperSelectorKindActive: "all"
    property bool wallpaperLauncherOpen: false
    property string wallpaperLauncherMode: "static"
    property bool widgetEditMode: false
    // Finding a widget while arranging (the iRiS widget bar); `inir background widgetSearch` drives the same state.
    property bool widgetSearchOpen: false
    property string widgetSearchText: ""
    signal widgetSearchCommand(string verb)
    // Arrow keys while arranging under iRiS: the chassis owns the keyboard and hands the step to the selected widget.
    signal desktopWidgetNudge(int dx, int dy)
    property string selectedDesktopWidget: ""
    property string desktopWidgetQuickControls: ""
    // Desktop widget manager toggle routed to the output that should show it.
    signal desktopWidgetManagerToggleRequested(string outputName)
    // Which output's desktop is showing the widget manager (Ryoku routes the
    // toggle per output; the manager surface reads this).
    property string desktopWidgetManagerOutput: ""
    // Shell-layout edit is not a frame surface here; the widget canvas reads
    // this to stand down during it, so it stays a constant.
    property bool shellLayoutEditMode: false

    function setWidgetEditMode(enabled: bool): void {
        if (enabled) {
            shellLayoutEditMode = false
            irisEdit = false
        }
        else {
            selectedDesktopWidget = ""
            desktopWidgetQuickControls = ""
            widgetSearchOpen = false
            widgetSearchText = ""
        }
        widgetEditMode = enabled
    }

    function selectDesktopWidget(instanceKey: string): void {
        if (!widgetEditMode)
            return
        selectedDesktopWidget = String(instanceKey ?? "")
    }

    function clearDesktopWidgetSelection(): void {
        selectedDesktopWidget = ""
        desktopWidgetQuickControls = ""
    }

    function requestDesktopWidgetQuickControls(instanceKey: string): void {
        if (!widgetEditMode)
            return
        const key = String(instanceKey ?? "")
        selectedDesktopWidget = key
        desktopWidgetQuickControls = key
    }

    property bool controlPanelOpen: false
    // iRiS: screen-local geometry ({x, y, width, height, radius, screen}) of the
    // Island part that last opened a surface, so it can morph out of and back
    // into that exact shape. Transient coordination only; null means "no origin".
    property var irisMorphOrigin: null
    // Who published that origin when it is not the Island (e.g. "left" for a
    // side panel's button). The Island leaves a foreign origin alone on open and
    // close; the morph surface clears it once it has fully collapsed.
    property string irisMorphOwner: ""
    // True while a surface is mid-morph out of or back into that origin; the
    // Island hides the published part so only one shape is ever on screen.
    // iRiS intent preloading: the pointer resting on the Island (controls) or an
    // expanded Island (settings) instantiates those surfaces hidden, so the
    // morph starts on the click frame instead of after an async load.
    property bool irisControlsWarm: false
    // Asks the focused Island to open a page ("media", "desktop", "activity",
    // "tray", "tools") from a surface that is not the Island (card, floating bubble).
    property string irisIslandPageRequest: ""
    // A bubble's own card: { kind, x, y, width, height, radius, screen, source }
    // with the screen-local rect of the bubble it grows out of; `source` names
    // that bubble ("island-<slot>" or "float-<slot>") so it can hide meanwhile.
    property var irisBubbleCard: null
    // The Island's desktop page is being arranged in place (iRiS Studio).
    property bool irisArrange: false
    // The Dock's body per output, so the chassis field draws it in the same pass
    // as the frame and the Island instead of the Dock carrying a second surface.
    property var irisDockBody: ({})
    // iRiS is being edited in place: every piece is grabbable, the edit bar
    // holds the pieces, the look and the sizes, and Done ends it.
    // Arranging the Control Center in place; closing the panel ends it.
    property bool irisControlEdit: false
    property string irisControlTab: "controls"
    property bool irisEdit: false
    // What the edit bar is inspecting: a piece slot ("extra:vitals", "left",
    // "app:kitty"), a surface ("island", "dock", "cards"…) or "" for the family.
    property string irisEditSelection: ""
    // A Studio target the edit bar should inspect ("" = keep what it shows).
    property string irisEditTarget: ""
    property int irisChassisEpoch: 0
    onIrisEditChanged: {
        if (!irisEdit) { irisEditSelection = ""; irisEditTarget = "" }
        else if (widgetEditMode) setWidgetEditMode(false)
        if (irisEdit && irisStudioOpen) irisStudioOpen = false
    }
    onIrisEditSelectionChanged: if (irisEditSelection.length > 0) irisEditTarget = ""
    onIrisEditTargetChanged: if (irisEditTarget.length > 0) irisEditSelection = ""
    // The in-place editors belong to iRiS: leaving the family ends them, or they are destroyed open and
    // come back open when the family does.
    function endIrisEditing(): void {
        irisLockEdit = false
        irisEdit = false
        irisControlEdit = false
    }
    onControlPanelOpenChanged: if (!controlPanelOpen) irisControlEdit = false
    onIrisControlEditChanged: if (!irisControlEdit) irisControlTab = "controls"
    // iRiS Studio, the panel form of Customize, is open. It and Customize on the shell never show together.
    property bool irisStudioOpen: false
    onIrisStudioOpenChanged: if (irisStudioOpen && irisEdit) irisEdit = false
    // Customize, in the form the person chose (iris.appearance.customize), on a target ("" = where it was).
    function openIrisCustomize(target): void {
        const wanted = String(target ?? "")
        if (String(Config.options?.iris?.appearance?.customize ?? "shell") === "studio") {
            irisStudioTarget = wanted
            irisStudioOpen = true
        } else {
            irisEditTarget = wanted
            irisEdit = true
        }
    }
    // A target Studio should show when it opens or is already open ("" = keep).
    property string irisStudioTarget: ""
    // The `source` of the card on screen (kept while it collapses), "" when none.
    // Asks whichever bubble shows this kind (floating first, then the Island)
    // to open its card; IPC and keyboard paths use it.
    property string irisBubbleCardRequest: ""
    // A floating piece asked to open its own menu, by kind (IPC).
    property string irisBubbleMenuRequest: ""
    // A bubble being carried: { slot, kind, screen, x, y (screen-local centre),
    // size, released }. The bubble layer draws it and resolves the drop.
    property var irisBubbleDrag: null
    // Per output name: what each bubble slot shows ({ left, right, utility }) and
    // the resting Island's screen-local geometry, published by each Island.
    property var irisBubbleKinds: ({})
    property var irisIslandGeometry: ({})
    // Per output: pieces an edge owner carries instead of floating over it ({ island: [...], dock: [...] }).
    property var irisAbsorbed: ({})
    property bool irisSettingsWarm: false
    // iRiS side panel ("left"/"right") that was revealed by resting at its screen
    // edge: it closes when the pointer leaves until a press inside commits it.
    property string irisSidebarPeek: ""
    // iRiS Dock held on screen by IPC (`inir iris dock show`) until hidden again
    // or an app is chosen from it.
    property bool irisDockShown: false
    // Opens an app's windows or menu on the focused Dock: { appId, mode: "windows" | "menu" }.
    property var irisDockMenuRequest: null
    // A query for Spotlight to type as it opens (IPC); taken and cleared by the palette.
    property string irisSpotlightQuery: ""
    // The iRiS desktop menu opened at a point of an output (`inir iris desktopMenu`).
    signal irisDesktopMenuRequested(string outputName, real x, real y)
    // Whether any output's Island is expanded, published for `inir iris status`.
    property bool irisIslandExpanded: false
    property string irisIslandShape: ""
    property string irisControlPickerRequest: ""
    property string irisIslandPage: ""
    // The palette's open flag: the frame's search surface and the Spotlight
    // launcher variant share it, so one morph answers Super+Space, the island
    // search bubble and the dock's launcher alike.
    property bool searchOpen: false
    property bool cheatsheetOpen: false
    property bool overlayOpen: false

    // User-configured fallback for singular panels such as wallpaper pickers.
    // Empty string uses the first available Quickshell screen.
    readonly property var primaryScreen: {
        const name = Config.options?.display?.primaryMonitor ?? ""
        if (name.length > 0) {
            const s = Quickshell.screens.find(scr => scr.name === name)
            if (s) return s
        }
        return Quickshell.screens[0]
    }

    // Focus-following screen for singular interactive surfaces. Keep this
    // separate from primaryScreen: the latter is a user fallback, while this
    // follows the compositor and only falls back when focus cannot be resolved.
    readonly property var focusedScreen: {
        const name = String(CompositorService.currentOutput ?? "")
        return Quickshell.screens.find(screen => (screen?.name ?? "") === name)
            ?? root.primaryScreen
            ?? Quickshell.screens[0]
            ?? null
    }

    function connectedOutputNames(allowedOutputs): var {
        const connected = Quickshell.screens
            .map(screen => String(screen?.name ?? ""))
            .filter(name => name.length > 0)
        if (!Array.isArray(allowedOutputs) || allowedOutputs.length === 0)
            return connected
        const enabled = connected.filter(name => allowedOutputs.includes(name))
        return enabled.length > 0 ? enabled : connected
    }

    function resolveOutputName(requestedOutput, allowedOutputs): string {
        const names = root.connectedOutputNames(allowedOutputs)
        if (names.length === 0)
            return ""
        const requested = String(requestedOutput ?? "")
        if (requested.length > 0 && names.includes(requested))
            return requested
        const focused = String(root.focusedScreen?.name ?? "")
        if (focused.length > 0 && names.includes(focused))
            return focused
        const primary = String(root.primaryScreen?.name ?? "")
        if (primary.length > 0 && names.includes(primary))
            return primary
        return names[0]
    }

    readonly property var sidebarScreenList: (Config.options?.panelFamily ?? "ii") === "iris"
        ? [] : (Config.options?.sidebar?.screenList ?? [])
    readonly property string sidebarLeftPresentationOutput:
        root.resolveOutputName(root.sidebarLeftTargetOutput,
            root.sidebarScreenList)
    readonly property string sidebarRightPresentationOutput:
        root.resolveOutputName(root.sidebarRightTargetOutput,
            root.sidebarScreenList)

    function openSidebarLeft(outputName): void {
        if (Config.options?.panelFamily === "iris" && !(Config.options?.iris?.sidebars?.left?.enable ?? true)) return
        sidebarLeftTargetOutput = root.resolveOutputName(outputName,
            root.sidebarScreenList)
        sidebarLeftOpen = true
    }

    function closeSidebarLeft(): void {
        sidebarLeftOpen = false
    }

    function openSidebarRight(outputName): void {
        if (Config.options?.panelFamily === "iris" && !(Config.options?.iris?.sidebars?.right?.enable ?? true)) return
        sidebarRightTargetOutput = root.resolveOutputName(outputName,
            root.sidebarScreenList)
        sidebarRightOpen = true
    }

    function closeSidebarRight(): void {
        sidebarRightOpen = false
    }

    onSidebarLeftOpenChanged: {
        if (sidebarLeftOpen && sidebarLeftTargetOutput.length === 0)
            sidebarLeftTargetOutput = root.resolveOutputName("",
                root.sidebarScreenList)
    }

    onSidebarRightOpenChanged: {
        if (sidebarRightOpen && sidebarRightTargetOutput.length === 0)
            sidebarRightTargetOutput = root.resolveOutputName("",
                root.sidebarScreenList)
        if (sidebarRightOpen) {
            Notifications.timeoutAll()
            Notifications.markAllRead()
        }
    }

    property real screenZoom: 1
    // Screen magnification is applied by the window-manager seam's zoom
    // action; the frame only owns the level and its easing.
    Behavior on screenZoom {
        animation: NumberAnimation { duration: Appearance.animation.elementMoveFast.duration; easing.type: Appearance.animation.elementMoveFast.type; easing.bezierCurve: Appearance.animation.elementMoveFast.bezierCurve }
    }

    IpcHandler {
		target: "zoom"

		function zoomIn(): void {
            screenZoom = Math.min(screenZoom + 0.4, 3.0)
        }

        function zoomOut(): void {
            screenZoom = Math.max(screenZoom - 0.4, 1)
        }
	}
}
