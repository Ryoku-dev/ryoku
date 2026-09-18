// qsbar bar root: bind one bar to each real Wayland output, skip transient
// nameless/0x0 placeholder screens, and recreate a BarSlot when that output
// disappears and returns. If a screen remains valid but the layer window loses
// resources or closes, recreate only that window instead of reloading the
// complete Quickshell configuration. This is the single qsbar; the bar form
// (islands · full · fit · dock · notch) is selected via Theme.barShellStyle.

import Quickshell
import Quickshell.Io
import Quickshell.Wayland
import QtQuick
import Ryoku.Ui.Singletons
import "panels"
import "controlcenter"
import "../../../../services/lib/screens.js" as Screens

Item {
    id: root

    readonly property bool ready: lifecycleReady()

    width: 0
    height: 0

    Theme {
        id: theme
    }

    function closePopups() { theme.closePopups() }
    function layoutLock() { theme.barUnlocked = false }
    function layoutUnlock() { theme.barUnlocked = true }
    function systemUpdateRefresh() { theme.updateRefreshTick++ }
    function runReactor(kind, arg) { theme.reactorTest(kind, arg) }
    function applyTheme(payload) { theme.ipcApplyTheme(payload) }
    function applyLauncher(payload) { theme.ipcApplyLauncher(payload) }
    function reloadTheme() { theme.ipcReloadTheme() }
    function openPicker(mode) { theme.ipcOpenPicker(mode) }

    // One bar per physical output, from the shared deduped screen list
    // (services/lib/screens.js): it drops the transient nameless/0x0 placeholder
    // and collapses a duplicate output announce, so a monitor re-add can never
    // stack a second BarSlot on the same output. A genuinely new ShellScreen still
    // makes Variants destroy the old BarSlot and instantiate a fresh one.
    readonly property var barScreens: Screens.uniqueByName(Quickshell.screens)
        .filter(screen => Tokens.barEnabledFor(screen.name))

    function lifecycleReady() {
        if (!theme._widgetsLoaded || barScreens.length === 0) return false

        var controllers = theme.barLayoutControllerKeys()
        if (controllers.length !== barScreens.length) return false

        for (var i = 0; i < barScreens.length; i++) {
            var controller = theme.barLayoutControllers[barScreens[i].name]
            if (!controller || !controller.ready || !controller.ready()) return false
        }
        return true
    }

    function activeScreenStillValid() {
        if (!theme.activePopupScreenName) return false

        for (var i = 0; i < barScreens.length; i++) {
            if (barScreens[i].name === theme.activePopupScreenName) return true
        }

        return false
    }

    function ensureActivePopupScreen() {
        if (barScreens.length === 0) {
            theme.closePopups()
            theme.activePopupScreen = null
            theme.activePopupScreenName = ""
        } else if (!activeScreenStillValid()) {
            if (theme.anyPopupVisible) theme.closePopups()
            theme.activatePopupScreen(barScreens[0])
        }
    }

    onBarScreensChanged: ensureActivePopupScreen()
    Component.onCompleted: ensureActivePopupScreen()

    // Secondary guard for failures that do not replace the ShellScreen object.
    // resourcesLost is followed by closed, so one pending flag handles the pair
    // once. A closed PanelWindow drops its backing layer-shell window; setting
    // visible=true creates a fresh one without resetting the rest of the shell.
    //
    // There is deliberately no give-up. A bar left closed is dead for the rest of
    // the session and only a hand-typed `ryoku reload` brings it back, so the
    // retry backs off instead of stopping. Variants destroys this Scope when it
    // replaces the delegate, which is what bounds the waiting.
    component BarWindowRecovery: Scope {
        id: recovery

        required property var targetWindow
        required property var targetScreen

        property bool pending: false
        property int attempt: 0
        property string reason: ""

        function screenReady() {
            return targetScreen !== null
                && targetScreen.name !== ""
                && targetScreen.width > 0
                && targetScreen.height > 0
        }

        function schedule(reason_) {
            if (pending) return

            pending = true
            attempt = 0
            reason = reason_
            console.warn("[BarWindowRecovery] window lost: " + reason)
            retryTimer.restart()
        }

        Connections {
            target: recovery.targetWindow

            function onResourcesLost() { recovery.schedule("resourcesLost") }
            function onClosed() { recovery.schedule("closed") }
        }

        // Grows with each failed attempt and caps, so a window that needs a moment
        // comes straight back while one that cannot map costs almost nothing.
        readonly property int retryDelay: Math.min(750 * Math.max(1, recovery.attempt), 15000)

        Timer {
            id: retryTimer
            interval: recovery.retryDelay
            repeat: false
            onTriggered: {
                // Mid-teardown the screen has no geometry yet. Wait for it rather
                // than abandoning the window: if Variants is replacing the
                // delegate this Scope goes with it, so waiting cannot outlive the
                // surface it belongs to.
                if (!recovery.screenReady()) {
                    retryTimer.restart()
                    return
                }

                recovery.attempt++
                console.warn("[BarWindowRecovery] recreating bar window (attempt "
                             + recovery.attempt + ")")
                recovery.targetWindow.visible = true
                verifyTimer.restart()
            }
        }

        Timer {
            id: verifyTimer
            interval: 1200
            repeat: false
            onTriggered: {
                if (recovery.targetWindow.backingWindowVisible) {
                    console.log("[BarWindowRecovery] bar window recovered after "
                                + recovery.attempt + " attempt(s)")
                    recovery.pending = false
                    recovery.attempt = 0
                    return
                }
                retryTimer.restart()
            }
        }
    }

    component PopupDismissLayer: PanelWindow {
        id: dismissLayer

        required property var root
        required property var targetScreen

        screen: targetScreen
        color: Qt.rgba(0, 0, 0, 0.001)
        anchors { top: true; bottom: true; left: true; right: true }
        exclusionMode: ExclusionMode.Ignore
        WlrLayershell.layer: WlrLayer.Overlay
        WlrLayershell.focusable: dismissLayer.visible
        WlrLayershell.keyboardFocus: dismissLayer.visible ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None
        WlrLayershell.namespace: "quickshell-popup-dismiss"
        mask: Region { item: hitArea }

        Rectangle {
            id: hitArea
            x: 0
            y: 0
            width: dismissLayer.width
            height: dismissLayer.height
            color: Qt.rgba(0, 0, 0, 0.001)

            MouseArea {
                anchors.fill: parent
                enabled: dismissLayer.visible
                acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
                onClicked: dismissLayer.root.closePopups()
            }
        }
        visible: root.anyPopupVisible
            && !root.keyboardPopupVisible
            && targetScreen
            && targetScreen.name !== ""
            && !root.isActivePopupScreenName(targetScreen.name)
    }

    Variants {
        model: root.barScreens

        delegate: Component {
            BarSlot {
                id: barWindow
                required property var modelData

                root: theme
                screen: modelData

                BarWindowRecovery {
                    targetWindow: barWindow
                    targetScreen: barWindow.modelData
                }
            }
        }
    }

    Variants {
        model: root.barScreens

        delegate: Component {
            PopupDismissLayer {
                required property var modelData

                root: theme
                targetScreen: modelData
            }
        }
    }

    TooltipOverlay { root: theme }
    DashboardPopup { root: theme }
    PowerProfilePanel { root: theme }
    MemoryPanel { root: theme }
    CpuPanel { root: theme }
    GpuPanel { root: theme }
    ThermalsPanel { root: theme }
    StoragePanel { root: theme }
    AiUsagePanel { root: theme }
    VolumePanel { root: theme }
    TrayPanel { root: theme }
    NotificationPanel { root: theme }
    NetworkPanel { root: theme }
    BluetoothPanel { root: theme }
    BatteryPanel { root: theme }
    PluginPanel { root: theme }
    BrightnessPanel { root: theme }
    MprisPanel { root: theme }
    WorkspacePanel { root: theme }
    ControlCenter { id: controlCenter; root: theme }
    TrayMenu { root: theme }

    // Picker variants: only the selected pickerStyle is instantiated.
    LazyLoader { active: theme.pickerStyle === "tanzaku" || theme.pickerStyle === "";  ImageCarouselPanel       { root: theme } }
    LazyLoader { active: theme.pickerStyle === "hearthstone";                           ImageCarouselHearthstone { root: theme } }
    LazyLoader { active: theme.pickerStyle === "carousel";                              ImageCarouselCarousel    { root: theme } }
    LazyLoader { active: theme.mediaBrowserVisible && (theme.pickerStyle === "tanzaku" || theme.pickerStyle === "");  MediaBrowserPanel        { root: theme } }
    LazyLoader { active: theme.mediaBrowserVisible && theme.pickerStyle === "hearthstone";                             MediaBrowserHearthstone  { root: theme } }
    LazyLoader { active: theme.mediaBrowserVisible && theme.pickerStyle === "carousel";                                MediaBrowserCarousel     { root: theme } }

    // The bar's own IPC surface (contract 5): open QS Bar Settings on a route
    // (a keybind or `ryoku-shell bar settings`), close it (`bar settings close`,
    // so a script can put it away), and read the live layout as JSON.
    // settings() drives the same ControlCenter.open the logo uses.
    IpcHandler {
        target: "qsbar"
        function settings(route: string): void {
            if (route === "close") controlCenter.close()
            else controlCenter.open(route)
        }
        function layout(): string { return JSON.stringify(theme.barLayout) }
    }

    // Theme.openBarSettings() (the contract-4 root API) requests the panel; the
    // panel lives here, so wire the request to the ControlCenter instance.
    Connections {
        target: theme
        function onBarSettingsOpenRequested(route) { controlCenter.open(route) }
    }
}
