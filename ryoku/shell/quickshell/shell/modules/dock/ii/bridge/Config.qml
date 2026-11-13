pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import ".."
import Quickshell
import stage.modules.common as StageCfg
import shell.services as Shell

/**
 * The ii dock's options tree.
 *
 * The `dock` subtree is generated from the stage's own dock section and reads
 * through DockStore: the shared keys (enable/position/height) mirror
 * shell.json's top-level dock object (what the Hub, the CLI and the stage
 * edit pages own), `pinnedApps` and `order` fall back to the shared pins,
 * and every other leaf lives beside them under `dock.ii`. Writes commit the
 * same way, so the daemon stays the sole writer and the designs stay in sync.
 *
 * Every other section the dock reads aliases onto the stage's options tree:
 * one physical store shared with the Stage Editor, exactly where the ported
 * pages already read and write.
 */
Singleton {
    id: root

    readonly property bool ready: StageCfg.Config.ready

    function saveOptionsNow() {
        // Leaves commit per-change through DockStore; nothing to flush.
    }

    function setNestedValue(key: string, value: var, strict: bool) {
        DockStore.commit(key, value);
    }

    readonly property QtObject options: QtObject {
        readonly property var ai: StageCfg.Config.options.ai
        readonly property var appStats: StageCfg.Config.options.appStats
        readonly property var appearance: StageCfg.Config.options.appearance
        readonly property var apps: StageCfg.Config.options.apps
        readonly property var audio: StageCfg.Config.options.audio
        readonly property var background: StageCfg.Config.options.background
        readonly property var bar: StageCfg.Config.options.bar
        readonly property var battery: StageCfg.Config.options.battery
        readonly property var bluetooth: StageCfg.Config.options.bluetooth
        readonly property var calendar: StageCfg.Config.options.calendar
        readonly property var clockApp: StageCfg.Config.options.clockApp
        readonly property var googleDrive: StageCfg.Config.options.googleDrive
        readonly property var notes: StageCfg.Config.options.notes
        readonly property var bluetoothDeviceImages: StageCfg.Config.options.bluetoothDeviceImages
        readonly property var contextMenu: StageCfg.Config.options.contextMenu
        readonly property var dockToPanel: StageCfg.Config.options.dockToPanel
        readonly property var dynamicIsland: StageCfg.Config.options.dynamicIsland
        readonly property var hacks: StageCfg.Config.options.hacks
        readonly property var idle: StageCfg.Config.options.idle
        readonly property var interactions: StageCfg.Config.options.interactions
        readonly property var language: StageCfg.Config.options.language
        readonly property var launcher: StageCfg.Config.options.launcher
        readonly property var light: StageCfg.Config.options.light
        readonly property var liveDraw: StageCfg.Config.options.liveDraw
        readonly property var localsend: StageCfg.Config.options.localsend
        readonly property var lock: StageCfg.Config.options.lock
        readonly property var media: StageCfg.Config.options.media
        readonly property var modes: StageCfg.Config.options.modes
        readonly property var networking: StageCfg.Config.options.networking
        readonly property var notifications: StageCfg.Config.options.notifications
        readonly property var phone: Shell.Config.phone
        readonly property var policies: StageCfg.Config.options.policies
        readonly property var resources: StageCfg.Config.options.resources
        readonly property var screenRecord: StageCfg.Config.options.screenRecord
        readonly property var screenSnip: StageCfg.Config.options.screenSnip
        readonly property var screenTime: StageCfg.Config.options.screenTime
        readonly property var search: StageCfg.Config.options.search
        readonly property var sidebar: StageCfg.Config.options.sidebar
        readonly property var sounds: StageCfg.Config.options.sounds
        readonly property var tablet: StageCfg.Config.options.tablet
        readonly property var time: StageCfg.Config.options.time
        readonly property var todo: StageCfg.Config.options.todo
        readonly property var userProfile: StageCfg.Config.options.userProfile
        readonly property var wallpaperSelector: StageCfg.Config.options.wallpaperSelector

        readonly property QtObject dock: QtObject {
        property var appGroups: DockStore.pull("appGroups", [])
        onAppGroupsChanged: if (DockStore.commit("appGroups", appGroups)) appGroups = DockStore.rebind("appGroups", [])
        property bool blockHoverInFullscreen: DockStore.pull("blockHoverInFullscreen", true)
        onBlockHoverInFullscreenChanged: if (DockStore.commit("blockHoverInFullscreen", blockHoverInFullscreen)) blockHoverInFullscreen = DockStore.rebind("blockHoverInFullscreen", true)
        property bool dimInactiveIcons: DockStore.pull("dimInactiveIcons", false)
        onDimInactiveIconsChanged: if (DockStore.commit("dimInactiveIcons", dimInactiveIcons)) dimInactiveIcons = DockStore.rebind("dimInactiveIcons", false)
        property real dockRadius: DockStore.pull("dockRadius", -1)
        onDockRadiusChanged: if (DockStore.commit("dockRadius", dockRadius)) dockRadius = DockStore.rebind("dockRadius", -1)
        property string dockStyle: DockStore.pull("dockStyle", "floating")
        onDockStyleChanged: if (DockStore.commit("dockStyle", dockStyle)) dockStyle = DockStore.rebind("dockStyle", "floating")
        property bool enable: DockStore.pull("enable", false)
        onEnableChanged: if (DockStore.commit("enable", enable)) enable = DockStore.rebind("enable", false)
        property real edgeGap: DockStore.pull("edgeGap", 0)
        onEdgeGapChanged: if (DockStore.commit("edgeGap", edgeGap)) edgeGap = DockStore.rebind("edgeGap", 0)
        property bool enableAppGroups: DockStore.pull("enableAppGroups", true)
        onEnableAppGroupsChanged: if (DockStore.commit("enableAppGroups", enableAppGroups)) enableAppGroups = DockStore.rebind("enableAppGroups", true)
        property bool enableAppTooltip: DockStore.pull("enableAppTooltip", false)
        onEnableAppTooltipChanged: if (DockStore.commit("enableAppTooltip", enableAppTooltip)) enableAppTooltip = DockStore.rebind("enableAppTooltip", false)
        property bool enableLivePreviewWidget: DockStore.pull("enableLivePreviewWidget", false)
        onEnableLivePreviewWidgetChanged: if (DockStore.commit("enableLivePreviewWidget", enableLivePreviewWidget)) enableLivePreviewWidget = DockStore.rebind("enableLivePreviewWidget", false)
        property bool enableMagnification: DockStore.pull("enableMagnification", false)
        onEnableMagnificationChanged: if (DockStore.commit("enableMagnification", enableMagnification)) enableMagnification = DockStore.rebind("enableMagnification", false)
        property bool enableMediaWidget: DockStore.pull("enableMediaWidget", false)
        onEnableMediaWidgetChanged: if (DockStore.commit("enableMediaWidget", enableMediaWidget)) enableMediaWidget = DockStore.rebind("enableMediaWidget", false)
        property bool enablePreview: DockStore.pull("enablePreview", true)
        onEnablePreviewChanged: if (DockStore.commit("enablePreview", enablePreview)) enablePreview = DockStore.rebind("enablePreview", true)
        property bool enableShapeMask: DockStore.pull("enableShapeMask", true)
        onEnableShapeMaskChanged: if (DockStore.commit("enableShapeMask", enableShapeMask)) enableShapeMask = DockStore.rebind("enableShapeMask", true)
        property bool enableSportsWidget: DockStore.pull("enableSportsWidget", false)
        onEnableSportsWidgetChanged: if (DockStore.commit("enableSportsWidget", enableSportsWidget)) enableSportsWidget = DockStore.rebind("enableSportsWidget", false)
        property bool enableTasksWidget: DockStore.pull("enableTasksWidget", false)
        onEnableTasksWidgetChanged: if (DockStore.commit("enableTasksWidget", enableTasksWidget)) enableTasksWidget = DockStore.rebind("enableTasksWidget", false)
        property bool enableWeatherWidget: DockStore.pull("enableWeatherWidget", false)
        onEnableWeatherWidgetChanged: if (DockStore.commit("enableWeatherWidget", enableWeatherWidget)) enableWeatherWidget = DockStore.rebind("enableWeatherWidget", false)
        property bool enableWidgetStack: DockStore.pull("enableWidgetStack", false)
        onEnableWidgetStackChanged: if (DockStore.commit("enableWidgetStack", enableWidgetStack)) enableWidgetStack = DockStore.rebind("enableWidgetStack", false)
        property real height: DockStore.pull("height", 56)
        onHeightChanged: if (DockStore.commit("height", height)) height = DockStore.rebind("height", 56)
        property real hoverRegionHeight: DockStore.pull("hoverRegionHeight", 2)
        onHoverRegionHeightChanged: if (DockStore.commit("hoverRegionHeight", hoverRegionHeight)) hoverRegionHeight = DockStore.rebind("hoverRegionHeight", 2)
        property bool hoverToReveal: DockStore.pull("hoverToReveal", true)
        onHoverToRevealChanged: if (DockStore.commit("hoverToReveal", hoverToReveal)) hoverToReveal = DockStore.rebind("hoverToReveal", true)
        property real iconSpacing: DockStore.pull("iconSpacing", -1)
        onIconSpacingChanged: if (DockStore.commit("iconSpacing", iconSpacing)) iconSpacing = DockStore.rebind("iconSpacing", -1)
        property var ignoredAppRegexes: DockStore.pull("ignoredAppRegexes", [])
        onIgnoredAppRegexesChanged: if (DockStore.commit("ignoredAppRegexes", ignoredAppRegexes)) ignoredAppRegexes = DockStore.rebind("ignoredAppRegexes", [])
        property real islandSpacing: DockStore.pull("islandSpacing", 8)
        onIslandSpacingChanged: if (DockStore.commit("islandSpacing", islandSpacing)) islandSpacing = DockStore.rebind("islandSpacing", 8)
        property bool islandsStyle: DockStore.pull("islandsStyle", false)
        onIslandsStyleChanged: if (DockStore.commit("islandsStyle", islandsStyle)) islandsStyle = DockStore.rebind("islandsStyle", false)
        property bool isolateMonitors: DockStore.pull("isolateMonitors", false)
        onIsolateMonitorsChanged: if (DockStore.commit("isolateMonitors", isolateMonitors)) isolateMonitors = DockStore.rebind("isolateMonitors", false)
        property string launchAnimation: DockStore.pull("launchAnimation", "bounce")
        onLaunchAnimationChanged: if (DockStore.commit("launchAnimation", launchAnimation)) launchAnimation = DockStore.rebind("launchAnimation", "bounce")
        property string livePreviewAppId: DockStore.pull("livePreviewAppId", "")
        onLivePreviewAppIdChanged: if (DockStore.commit("livePreviewAppId", livePreviewAppId)) livePreviewAppId = DockStore.rebind("livePreviewAppId", "")
        property string livePreviewCaptureMode: DockStore.pull("livePreviewCaptureMode", "visible")
        onLivePreviewCaptureModeChanged: if (DockStore.commit("livePreviewCaptureMode", livePreviewCaptureMode)) livePreviewCaptureMode = DockStore.rebind("livePreviewCaptureMode", "visible")
        property bool livePreviewFollowActiveWindow: DockStore.pull("livePreviewFollowActiveWindow", true)
        onLivePreviewFollowActiveWindowChanged: if (DockStore.commit("livePreviewFollowActiveWindow", livePreviewFollowActiveWindow)) livePreviewFollowActiveWindow = DockStore.rebind("livePreviewFollowActiveWindow", true)
        property bool livePreviewPaintCursor: DockStore.pull("livePreviewPaintCursor", false)
        onLivePreviewPaintCursorChanged: if (DockStore.commit("livePreviewPaintCursor", livePreviewPaintCursor)) livePreviewPaintCursor = DockStore.rebind("livePreviewPaintCursor", false)
        property int livePreviewSlots: DockStore.pull("livePreviewSlots", 2)
        onLivePreviewSlotsChanged: if (DockStore.commit("livePreviewSlots", livePreviewSlots)) livePreviewSlots = DockStore.rebind("livePreviewSlots", 2)
        property bool lockReorder: DockStore.pull("lockReorder", false)
        onLockReorderChanged: if (DockStore.commit("lockReorder", lockReorder)) lockReorder = DockStore.rebind("lockReorder", false)
        property string magnificationCurve: DockStore.pull("magnificationCurve", "cosine")
        onMagnificationCurveChanged: if (DockStore.commit("magnificationCurve", magnificationCurve)) magnificationCurve = DockStore.rebind("magnificationCurve", "cosine")
        property bool magnificationDynamicSpacing: DockStore.pull("magnificationDynamicSpacing", true)
        onMagnificationDynamicSpacingChanged: if (DockStore.commit("magnificationDynamicSpacing", magnificationDynamicSpacing)) magnificationDynamicSpacing = DockStore.rebind("magnificationDynamicSpacing", true)
        property real magnificationInfluenceRadius: DockStore.pull("magnificationInfluenceRadius", 2.35)
        onMagnificationInfluenceRadiusChanged: if (DockStore.commit("magnificationInfluenceRadius", magnificationInfluenceRadius)) magnificationInfluenceRadius = DockStore.rebind("magnificationInfluenceRadius", 2.35)
        property string magnificationMotion: DockStore.pull("magnificationMotion", "balanced")
        onMagnificationMotionChanged: if (DockStore.commit("magnificationMotion", magnificationMotion)) magnificationMotion = DockStore.rebind("magnificationMotion", "balanced")
        property real magnificationScale: DockStore.pull("magnificationScale", 1.5)
        onMagnificationScaleChanged: if (DockStore.commit("magnificationScale", magnificationScale)) magnificationScale = DockStore.rebind("magnificationScale", 1.5)
        property var manualOrder: DockStore.pull("manualOrder", [])
        onManualOrderChanged: if (DockStore.commit("manualOrder", manualOrder)) manualOrder = DockStore.rebind("manualOrder", [])
        property string monitor: DockStore.pull("monitor", "")
        onMonitorChanged: if (DockStore.commit("monitor", monitor)) monitor = DockStore.rebind("monitor", "")
        property bool monochromeIcons: DockStore.pull("monochromeIcons", false)
        onMonochromeIconsChanged: if (DockStore.commit("monochromeIcons", monochromeIcons)) monochromeIcons = DockStore.rebind("monochromeIcons", false)
        property string notificationAnimation: DockStore.pull("notificationAnimation", "bounce")
        onNotificationAnimationChanged: if (DockStore.commit("notificationAnimation", notificationAnimation)) notificationAnimation = DockStore.rebind("notificationAnimation", "bounce")
        property bool onTop: DockStore.pull("onTop", false)
        onOnTopChanged: if (DockStore.commit("onTop", onTop)) onTop = DockStore.rebind("onTop", false)
        property var order: DockStore.pull("order", ["pin", "app:org.kde.dolphin", "app:kitty", "runningApps", "media", "weather", "sports", "livePreview", "phone", "trash", "overview"])
        onOrderChanged: if (DockStore.commit("order", order)) order = DockStore.rebind("order", ["pin", "app:org.kde.dolphin", "app:kitty", "runningApps", "media", "weather", "sports", "livePreview", "phone", "trash", "overview"])
        property var pinnedApps: DockStore.pull("pinnedApps", ["org.kde.dolphin", "kitty"])
        onPinnedAppsChanged: if (DockStore.commit("pinnedApps", pinnedApps)) pinnedApps = DockStore.rebind("pinnedApps", ["org.kde.dolphin", "kitty"])
        property var pinnedFiles: DockStore.pull("pinnedFiles", [])
        onPinnedFilesChanged: if (DockStore.commit("pinnedFiles", pinnedFiles)) pinnedFiles = DockStore.rebind("pinnedFiles", [])
        property bool pinnedOnStartup: DockStore.pull("pinnedOnStartup", true)
        onPinnedOnStartupChanged: if (DockStore.commit("pinnedOnStartup", pinnedOnStartup)) pinnedOnStartup = DockStore.rebind("pinnedOnStartup", true)
        property string position: DockStore.pull("position", "auto")
        onPositionChanged: if (DockStore.commit("position", position)) position = DockStore.rebind("position", "auto")
        property string shapeMask: DockStore.pull("shapeMask", "Circle")
        onShapeMaskChanged: if (DockStore.commit("shapeMask", shapeMask)) shapeMask = DockStore.rebind("shapeMask", "Circle")
        property bool showDividers: DockStore.pull("showDividers", true)
        onShowDividersChanged: if (DockStore.commit("showDividers", showDividers)) showDividers = DockStore.rebind("showDividers", true)
        property bool showNotificationBadges: DockStore.pull("showNotificationBadges", true)
        onShowNotificationBadgesChanged: if (DockStore.commit("showNotificationBadges", showNotificationBadges)) showNotificationBadges = DockStore.rebind("showNotificationBadges", true)
        property bool showOnlyOnFocusedMonitor: DockStore.pull("showOnlyOnFocusedMonitor", false)
        onShowOnlyOnFocusedMonitorChanged: if (DockStore.commit("showOnlyOnFocusedMonitor", showOnlyOnFocusedMonitor)) showOnlyOnFocusedMonitor = DockStore.rebind("showOnlyOnFocusedMonitor", false)
        property bool showOverviewButton: DockStore.pull("showOverviewButton", true)
        onShowOverviewButtonChanged: if (DockStore.commit("showOverviewButton", showOverviewButton)) showOverviewButton = DockStore.rebind("showOverviewButton", true)
        property bool showPhoneButton: DockStore.pull("showPhoneButton", false)
        onShowPhoneButtonChanged: if (DockStore.commit("showPhoneButton", showPhoneButton)) showPhoneButton = DockStore.rebind("showPhoneButton", false)
        property bool showPinButton: DockStore.pull("showPinButton", true)
        onShowPinButtonChanged: if (DockStore.commit("showPinButton", showPinButton)) showPinButton = DockStore.rebind("showPinButton", true)
        property bool showTrashButton: DockStore.pull("showTrashButton", false)
        onShowTrashButtonChanged: if (DockStore.commit("showTrashButton", showTrashButton)) showTrashButton = DockStore.rebind("showTrashButton", false)
        property bool smartGrouping: DockStore.pull("smartGrouping", false)
        onSmartGroupingChanged: if (DockStore.commit("smartGrouping", smartGrouping)) smartGrouping = DockStore.rebind("smartGrouping", false)
        property bool switchPresetsOnScroll: DockStore.pull("switchPresetsOnScroll", true)
        onSwitchPresetsOnScrollChanged: if (DockStore.commit("switchPresetsOnScroll", switchPresetsOnScroll)) switchPresetsOnScroll = DockStore.rebind("switchPresetsOnScroll", true)
        property var utilityWidgets: DockStore.pull("utilityWidgets", [])
        onUtilityWidgetsChanged: if (DockStore.commit("utilityWidgets", utilityWidgets)) utilityWidgets = DockStore.rebind("utilityWidgets", [])
        property real widgetRadius: DockStore.pull("widgetRadius", -1)
        onWidgetRadiusChanged: if (DockStore.commit("widgetRadius", widgetRadius)) widgetRadius = DockStore.rebind("widgetRadius", -1)
        property var widgetStackItems: DockStore.pull("widgetStackItems", ["media", "weather", "tasks"])
        onWidgetStackItemsChanged: if (DockStore.commit("widgetStackItems", widgetStackItems)) widgetStackItems = DockStore.rebind("widgetStackItems", ["media", "weather", "tasks"])
        property QtObject overviewButton: QtObject {
            property bool alwaysShowShape: DockStore.pull("overviewButton.alwaysShowShape", false)
            onAlwaysShowShapeChanged: if (DockStore.commit("overviewButton.alwaysShowShape", alwaysShowShape)) alwaysShowShape = DockStore.rebind("overviewButton.alwaysShowShape", false)
            property string iconFile: DockStore.pull("overviewButton.iconFile", "")
            onIconFileChanged: if (DockStore.commit("overviewButton.iconFile", iconFile)) iconFile = DockStore.rebind("overviewButton.iconFile", "")
            property bool iconTint: DockStore.pull("overviewButton.iconTint", true)
            onIconTintChanged: if (DockStore.commit("overviewButton.iconTint", iconTint)) iconTint = DockStore.rebind("overviewButton.iconTint", true)
            property string shape: DockStore.pull("overviewButton.shape", "SoftBurst")
            onShapeChanged: if (DockStore.commit("overviewButton.shape", shape)) shape = DockStore.rebind("overviewButton.shape", "SoftBurst")
            property string symbol: DockStore.pull("overviewButton.symbol", "")
            onSymbolChanged: if (DockStore.commit("overviewButton.symbol", symbol)) symbol = DockStore.rebind("overviewButton.symbol", "")
        }
        property QtObject utilities: QtObject {
            property QtObject aiUsage: QtObject {
                property string provider: DockStore.pull("utilities.aiUsage.provider", "claude")
                onProviderChanged: if (DockStore.commit("utilities.aiUsage.provider", provider)) provider = DockStore.rebind("utilities.aiUsage.provider", "claude")
                property string squareWindow: DockStore.pull("utilities.aiUsage.squareWindow", "long")
                onSquareWindowChanged: if (DockStore.commit("utilities.aiUsage.squareWindow", squareWindow)) squareWindow = DockStore.rebind("utilities.aiUsage.squareWindow", "long")
            }
            property QtObject converter: QtObject {
                property string category: DockStore.pull("utilities.converter.category", "length")
                onCategoryChanged: if (DockStore.commit("utilities.converter.category", category)) category = DockStore.rebind("utilities.converter.category", "length")
                property string from: DockStore.pull("utilities.converter.from", "cm")
                onFromChanged: if (DockStore.commit("utilities.converter.from", from)) from = DockStore.rebind("utilities.converter.from", "cm")
                property string to: DockStore.pull("utilities.converter.to", "in")
                onToChanged: if (DockStore.commit("utilities.converter.to", to)) to = DockStore.rebind("utilities.converter.to", "in")
            }
            property QtObject favorites: QtObject {
                property var sites: DockStore.pull("utilities.favorites.sites", [])
                onSitesChanged: if (DockStore.commit("utilities.favorites.sites", sites)) sites = DockStore.rebind("utilities.favorites.sites", [])
            }
            property QtObject files: QtObject {
                property string folder: DockStore.pull("utilities.files.folder", "")
                onFolderChanged: if (DockStore.commit("utilities.files.folder", folder)) folder = DockStore.rebind("utilities.files.folder", "")
            }
            property QtObject pomodoro: QtObject {
                property bool clickToggles: DockStore.pull("utilities.pomodoro.clickToggles", true)
                onClickTogglesChanged: if (DockStore.commit("utilities.pomodoro.clickToggles", clickToggles)) clickToggles = DockStore.rebind("utilities.pomodoro.clickToggles", true)
            }
            property QtObject screenshots: QtObject {
                property string folder: DockStore.pull("utilities.screenshots.folder", "")
                onFolderChanged: if (DockStore.commit("utilities.screenshots.folder", folder)) folder = DockStore.rebind("utilities.screenshots.folder", "")
            }
            property QtObject search: QtObject {
                property bool background: DockStore.pull("utilities.search.background", true)
                onBackgroundChanged: if (DockStore.commit("utilities.search.background", background)) background = DockStore.rebind("utilities.search.background", true)
                property string squareShape: DockStore.pull("utilities.search.squareShape", "Cookie9Sided")
                onSquareShapeChanged: if (DockStore.commit("utilities.search.squareShape", squareShape)) squareShape = DockStore.rebind("utilities.search.squareShape", "Cookie9Sided")
            }
            property QtObject send: QtObject {
                property string backend: DockStore.pull("utilities.send.backend", "kdeconnect")
                onBackendChanged: if (DockStore.commit("utilities.send.backend", backend)) backend = DockStore.rebind("utilities.send.backend", "kdeconnect")
                property string kdeDevice: DockStore.pull("utilities.send.kdeDevice", "")
                onKdeDeviceChanged: if (DockStore.commit("utilities.send.kdeDevice", kdeDevice)) kdeDevice = DockStore.rebind("utilities.send.kdeDevice", "")
                property string localsendAlias: DockStore.pull("utilities.send.localsendAlias", "")
                onLocalsendAliasChanged: if (DockStore.commit("utilities.send.localsendAlias", localsendAlias)) localsendAlias = DockStore.rebind("utilities.send.localsendAlias", "")
                property string localsendIp: DockStore.pull("utilities.send.localsendIp", "")
                onLocalsendIpChanged: if (DockStore.commit("utilities.send.localsendIp", localsendIp)) localsendIp = DockStore.rebind("utilities.send.localsendIp", "")
            }
            property QtObject shelf: QtObject {
                property bool removeAfterDrag: DockStore.pull("utilities.shelf.removeAfterDrag", false)
                onRemoveAfterDragChanged: if (DockStore.commit("utilities.shelf.removeAfterDrag", removeAfterDrag)) removeAfterDrag = DockStore.rebind("utilities.shelf.removeAfterDrag", false)
            }
            property QtObject stopwatch: QtObject {
                property bool clickToggles: DockStore.pull("utilities.stopwatch.clickToggles", true)
                onClickTogglesChanged: if (DockStore.commit("utilities.stopwatch.clickToggles", clickToggles)) clickToggles = DockStore.rebind("utilities.stopwatch.clickToggles", true)
            }
            property QtObject timer: QtObject {
                property bool clickPauses: DockStore.pull("utilities.timer.clickPauses", true)
                onClickPausesChanged: if (DockStore.commit("utilities.timer.clickPauses", clickPauses)) clickPauses = DockStore.rebind("utilities.timer.clickPauses", true)
                property var presets: DockStore.pull("utilities.timer.presets", [1, 5, 10, 25])
                onPresetsChanged: if (DockStore.commit("utilities.timer.presets", presets)) presets = DockStore.rebind("utilities.timer.presets", [1, 5, 10, 25])
            }
            property QtObject tools: QtObject {
                property var shown: DockStore.pull("utilities.tools.shown", ["screenSnip", "screenRecord", "colorPicker", "wallpaper", "darkMode"])
                onShownChanged: if (DockStore.commit("utilities.tools.shown", shown)) shown = DockStore.rebind("utilities.tools.shown", ["screenSnip", "screenRecord", "colorPicker", "wallpaper", "darkMode"])
            }
            property QtObject water: QtObject {
                property int glassMl: DockStore.pull("utilities.water.glassMl", 250)
                onGlassMlChanged: if (DockStore.commit("utilities.water.glassMl", glassMl)) glassMl = DockStore.rebind("utilities.water.glassMl", 250)
            }
        }
        }
    }
}
