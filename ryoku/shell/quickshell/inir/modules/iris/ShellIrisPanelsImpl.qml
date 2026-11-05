pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import inir
import inir.services
import inir.modules.common
import inir.modules.cheatsheet
import inir.modules.ii.overlay
import inir.modules.iris.notificationPopup
import inir.modules.iris.onScreenDisplay
import inir.modules.iris.session
import inir.modules.iris.style
import inir.modules.iris.pieces
import inir.modules.iris.palette
import inir.modules.iris.settings
import inir.modules.iris.studio
import inir.modules.iris.equalizer
import inir.modules.iris.polkit
import inir.modules.iris.wallpaper
import inir.modules.iris.sidebar

Item {
    id: root

    component PanelLoader: LazyLoader {
        required property string identifier
        property bool extraCondition: true
        readonly property bool enabledPanel: Config.ready
            && (Config.options?.enabledPanels ?? []).includes(identifier)
            && extraCondition
        loading: enabledPanel
        activeAsync: enabledPanel
    }

    component OnDemandPanelLoader: LazyLoader {
        id: loader
        required property string identifier
        required property bool open
        property bool extraCondition: true
        property bool requireEnabledPanel: true
        property int closeGraceMs: IrisStyle.revealDuration + 30
        property bool resident: open
        property Timer closeGrace: Timer {
            interval: loader.closeGraceMs
            onTriggered: loader.resident = loader.open
        }
        readonly property bool enabledPanel: Config.ready
            && (!requireEnabledPanel || (Config.options?.enabledPanels ?? []).includes(identifier))
            && extraCondition

        onOpenChanged: {
            if (open) {
                closeGrace.stop()
                resident = true
            } else {
                closeGrace.restart()
            }
        }

        loading: enabledPanel && resident
        activeAsync: enabledPanel && GlobalStates.deferredPanelsReady && resident
    }

    IrisSidebarEdge { side: "left" }
    IrisSidebarEdge { side: "right" }

    OnDemandPanelLoader {
        identifier: "irisSidebarLeft"
        requireEnabledPanel: false
        open: GlobalStates.sidebarLeftOpen
        extraCondition: Config.options?.iris?.sidebars?.left?.enable ?? true
        closeGraceMs: IrisStyle.settleDuration + 80
        component: IrisSidebar { side: "left" }
    }

    OnDemandPanelLoader {
        identifier: "irisSidebarRight"
        requireEnabledPanel: false
        open: GlobalStates.sidebarRightOpen
        extraCondition: Config.options?.iris?.sidebars?.right?.enable ?? true
        closeGraceMs: IrisStyle.settleDuration + 80
        component: IrisSidebar { side: "right" }
    }

    OnDemandPanelLoader {
        identifier: "irisNotificationPopup"
        open: (Notifications.popupList?.length ?? 0) > 0
        closeGraceMs: IrisStyle.settleDuration * 2 + 160
        extraCondition: (Config.options?.iris?.modules?.notificationPopup ?? true)
            && (!(Config.options?.enabledPanels ?? []).includes("irisBar")
                || (GameMode.hasFullscreenOnOutput(GlobalStates.focusedScreen?.name ?? "") && !CompositorService.overviewOpen
                    && (Config.options?.iris?.notifications?.fullscreen ?? true)))
        component: IrisNotificationPopup {}
    }

    OnDemandPanelLoader {
        identifier: "irisStudio"
        requireEnabledPanel: false
        open: GlobalStates.irisStudioOpen
        closeGraceMs: IrisStyle.settleDuration + 120
        component: IrisStudio {}
    }

    Component {
        id: irisSettingsOverlayComponent
        IrisSettingsOverlay {}
    }
    Component {
        id: irisSettingsWindowComponent
        IrisSettingsWindow {}
    }

    OnDemandPanelLoader {
        identifier: "irisSettings"
        requireEnabledPanel: false
        open: GlobalStates.settingsOverlayOpen || GlobalStates.irisSettingsWarm
        closeGraceMs: IrisStyle.settleDuration + 120
        component: String(Config.options?.iris?.appearance?.settingsHost ?? "overlay") === "window"
            ? irisSettingsWindowComponent : irisSettingsOverlayComponent
    }

    OnDemandPanelLoader {
        identifier: "irisEqualizer"
        requireEnabledPanel: false
        open: GlobalStates.irisEqualizerOpen
        closeGraceMs: IrisStyle.settleDuration + 120
        component: IrisEqualizerPanel {
            open: GlobalStates.irisEqualizerOpen
            targetOutput: GlobalStates.irisEqualizerTargetOutput
            onCloseRequested: GlobalStates.closeIrisEqualizer()
        }
    }

    PanelLoader {
        identifier: "irisOnScreenDisplay"
        extraCondition: (Config.options?.iris?.modules?.osd ?? true)
            && (!GlobalStates.barOpen
                || (GameMode.hasFullscreenOnOutput(GlobalStates.focusedScreen?.name ?? "") && !CompositorService.overviewOpen)
                || !(Config.options?.enabledPanels ?? []).includes("irisBar")
                || ((Config.options?.iris?.bar?.screenList ?? []).length > 0
                    && !(Config.options.iris.bar.screenList).includes(GlobalStates.focusedScreen?.name ?? "")))
        component: IrisOSD {}
    }

    OnDemandPanelLoader {
        identifier: "irisSessionScreen"
        open: GlobalStates.sessionOpen
        extraCondition: Config.options?.iris?.modules?.sessionScreen ?? true
        component: IrisSessionScreen {}
    }

    OnDemandPanelLoader {
        identifier: "irisPalette"
        open: GlobalStates.searchOpen
        closeGraceMs: IrisStyle.settleDuration + 120
        extraCondition: Config.options?.iris?.modules?.palette ?? true
        component: IrisPalette {}
    }
    OnDemandPanelLoader {
        identifier: "irisCheatsheet"
        requireEnabledPanel: false
        open: GlobalStates.cheatsheetOpen
        closeGraceMs: IrisStyle.settleDuration + 120
        component: Cheatsheet {}
    }

    OnDemandPanelLoader {
        identifier: "irisOverlay"
        requireEnabledPanel: false
        open: GlobalStates.overlayOpen
        closeGraceMs: IrisStyle.settleDuration + 120
        component: Overlay {}
    }




    // The daemon is the single PolicyKit agent; this only presents its prompt in
    // the iRiS look (see IrisPolkit).
    LazyLoader {
        activeAsync: Config.ready && GlobalStates.deferredPanelsReady
            && (Config.options?.enabledPanels ?? []).includes("irisPolkit")
            && (Config.options?.iris?.modules?.polkit ?? true)
        component: IrisPolkit {}
    }
}
