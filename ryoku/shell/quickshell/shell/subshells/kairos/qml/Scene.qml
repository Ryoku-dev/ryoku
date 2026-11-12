// Kairos: one dynamic island on the top edge, carrying the clock. The window
// holds the open island whole, masks input to the island, and reserves only the
// resting band, so tiled windows clear the pill without the surface ever
// resizing during the morph.

pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import shell.services
import shell.modules.stage.frame
import Ryoku.Ui.Singletons
import "components" as C
import "settings" as S

Item {
    id: scene

    // The screen, set by Frame's per-monitor Loader.
    property var modelData: null
    width: 0
    height: 0

    // The gear and the quick-settings button each own one window; only one opens.
    readonly property bool settingsOpen: S.SettingsState.open
        && S.SettingsState.screen === scene.modelData
    property bool quickOpen: false
    function toggleSettings() {
        if (scene.settingsOpen) {
            S.SettingsState.close();
        } else {
            scene.quickOpen = false;
            S.SettingsState.show(scene.modelData);
        }
    }
    function toggleQuickSettings() {
        scene.quickOpen = !scene.quickOpen;
        if (scene.quickOpen)
            S.SettingsState.close();
    }
    Connections {
        target: S.SettingsState
        function onQuickSettingsRequested() {
            var name = scene.modelData ? String(scene.modelData.name) : "";
            var focused = String(Wm.focusedOutput || "");
            var mine = focused.length > 0 ? focused === name
                : Quickshell.screens.length > 0 && String(Quickshell.screens[0].name) === name;
            if (mine)
                scene.toggleQuickSettings();
        }
    }

    PanelWindow {
        id: win

        screen: scene.modelData

        // This screen's shell state: while Super+Space has the launcher open, the
        // launcher's own pill stands in for this island, and this one drops back to
        // its resting size underneath it.
        readonly property var state: scene.modelData ? ShellState.forScreen(scene.modelData) : null

        color: "transparent"
        exclusionMode: ExclusionMode.Normal
        // Reserve the resting band so tiled windows clear the island. The open
        // island overhangs it, which is the point of a floating one.
        exclusiveZone: island.band
        WlrLayershell.layer: WlrLayer.Top
        WlrLayershell.keyboardFocus: island.quickOpen
            ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None
        WlrLayershell.namespace: "ryoku-bar"

        // as tall as the screen so the surface never resizes mid-morph, and so the shade's
        // click-away is this one window: hyprland keeps the pointer on an exclusive layer
        anchors { top: true; left: true; right: true }
        implicitHeight: frameLayer.extent

        readonly property rect inputRect: frameLayer.maskRect(island.x, island.y,
            island.width, island.height)

        // The bar belongs to the desktop card while this monitor is being edited.
        mask: Region {
            x: scene.quickOpen ? 0 : Math.round(win.inputRect.x)
            y: scene.quickOpen ? 0 : Math.round(win.inputRect.y)
            width: scene.quickOpen ? win.width : Math.round(win.inputRect.width)
            height: scene.quickOpen ? win.height : Math.round(win.inputRect.height)
        }

        MouseArea {
            anchors.fill: parent
            enabled: scene.quickOpen
            acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
            onPressed: scene.quickOpen = false
        }

        StageFramedLayer {
            id: frameLayer
            interactive: false
            screenName: win.screen?.name ?? ""
            edge: "top"
            restThickness: win.screen?.height ?? 1080
            screenWidth: win.screen?.width ?? 1920
            screenHeight: win.screen?.height ?? 1080

            C.Island {
                id: island
                y: island.topGap
                // The clock's resting centre lands on the screen's centre line, so the
                // clock rests centred and expands there; the music grows leftward from
                // just beside it.
                x: Math.round(parent.width / 2 - island.clockRestCentre)
                suspended: win.state ? win.state.launcherOpen : false
                quickOpen: scene.quickOpen
                screenHeight: win.screen?.height ?? 1080
                onSettingsToggled: scene.toggleSettings()
                onQuickSettingsToggled: scene.toggleQuickSettings()
                onQuickCloseRequested: scene.quickOpen = false
            }
        }
    }

    S.SettingsWindow {
        targetScreen: scene.modelData
        open: scene.settingsOpen
        onCloseRequested: S.SettingsState.close()
    }
}
