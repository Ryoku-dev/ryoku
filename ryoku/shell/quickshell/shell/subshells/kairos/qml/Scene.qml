// Kairos: one dynamic island on the top edge, carrying the clock. The window
// holds the open island whole, masks input to the island, and reserves only the
// resting band, so tiled windows clear the pill without the surface ever
// resizing during the morph. While the now-playing panel is pinned open the
// window covers the screen and a click anywhere off the panel dismisses it.

pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import shell.services
import shell.modules.stage.frame
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

        // Rests as tall as the island's tallest state so the surface never
        // resizes: the pill is what grows.
        anchors { top: true; left: true; right: true }
        implicitHeight: frameLayer.extent

        readonly property rect inputRect: frameLayer.maskRect(island.x, island.y,
            island.width, island.height)

        // The bar belongs to the desktop card while this monitor is being edited.
        mask: Region {
            x: Math.round(win.inputRect.x)
            y: Math.round(win.inputRect.y)
            width: Math.round(win.inputRect.width)
            height: Math.round(win.inputRect.height)
        }

        StageFramedLayer {
            id: frameLayer
            interactive: false
            screenName: win.screen?.name ?? ""
            edge: "top"
            restThickness: island.reach
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

    // A transparent full-output layer under the bar that exists only while quick
    // settings is open, so a click anywhere off the island closes it. It is below
    // the bar (Bottom < Top), so the island still takes its own clicks, and it is
    // gone when closed, so the desktop is never blocked.
    PanelWindow {
        id: dismiss
        screen: scene.modelData
        visible: scene.quickOpen
        color: "transparent"
        anchors { top: true; bottom: true; left: true; right: true }
        exclusionMode: ExclusionMode.Ignore
        WlrLayershell.layer: WlrLayer.Bottom
        WlrLayershell.keyboardFocus: WlrKeyboardFocus.None
        WlrLayershell.namespace: "ryoku-kairos-dismiss"

        MouseArea {
            anchors.fill: parent
            acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
            onPressed: scene.quickOpen = false
        }
    }
}
