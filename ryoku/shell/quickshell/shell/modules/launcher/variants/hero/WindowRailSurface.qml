pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import Quickshell.Widgets
import "../../shared/Singletons"
import Ryoku.Ui.Singletons
import "." as HeroVariant
import "../../../../services/lib/screens.js" as Screens

// The open-window rail owns a second, deliberately small layer surface. It is
// not part of the launcher card: its transparent envelope has its own input
// region and its only visible pixels are the preview cards themselves.
PanelWindow {
    id: win

    required property var modelData
    property var launcher: null
    property var launcherSurface: null

    signal ready(var surface)

    readonly property string surfaceMonitor: modelData
        ? String(modelData.name || "") : ""
    readonly property real s: Screens.monitorScale(modelData, 1.2)
        * Math.max(0.8, Math.min(1.4, Config.fontScale))
    readonly property bool sameMonitor: launcherSurface
        && String(launcherSurface.surfaceMonitor || "") === surfaceMonitor
    readonly property bool eligible: sameMonitor && launcher
        && launcher.shown && launcher.bodyOpen
        && launcher.selectedResult !== null
        && launcherSurface.backingWindowVisible
        && launcherSurface.visualOpacity > 0.01
    readonly property real cardWidth: launcherSurface
        ? launcherSurface.cardWidth : 0
    readonly property real topOffset: launcherSurface
        ? launcherSurface.stableTopMargin + launcherSurface.shadowPadTop
            + launcherSurface.presentedHeight * launcherSurface.visualScale
            + 14 * s : 0

    property alias rail: windowRail

    screen: modelData
    visible: eligible && windowRail.hasWindows
    color: "transparent"
    surfaceFormat.opaque: false
    implicitWidth: Math.ceil(cardWidth + 32 * s)
    implicitHeight: Math.ceil(windowRail.targetHeight + 24 * s)
    exclusionMode: ExclusionMode.Ignore
    WlrLayershell.namespace: "launcher-window-rail"
    WlrLayershell.layer: WlrLayer.Overlay
    WlrLayershell.keyboardFocus: WlrKeyboardFocus.None
    anchors { top: true }
    margins.top: Math.round(topOffset)
    // This is deliberately pointer-transparent. Keyboard movement is the
    // stable navigation path, and letting a second layer surface accept input
    // would let it interrupt the launcher's exclusive text focus.
    mask: emptyInputRegion

    Region {
        id: emptyInputRegion
    }

    Item {
        id: railBounds
        x: 16 * win.s
        y: 8 * win.s
        width: win.cardWidth
        height: windowRail.targetHeight

        HeroVariant.WindowRail {
            id: windowRail
            anchors.fill: parent
            s: win.s
            open: win.eligible
            animationsEnabled: !Motion.reduce
            entry: win.launcher ? win.launcher.selectedResult : null
            focusActive: win.launcher
                && win.launcher.interactionRegion === "windows"
            onFocusRequested: address => {
                if (win.launcher)
                    win.launcher.focusedWindowAddress = address;
            }
        }
    }

    Component.onCompleted: ready(win)
    onVisibleChanged: {
        if (visible) {
            Qt.callLater(function () {
                if (win.visible && win.launcher)
                    win.launcher.focusInput();
            });
            inputFocusSettler.restart();
        }
    }
    onTopOffsetChanged: if (visible) inputFocusSettler.restart()

    // The parent surface is still receiving configure events while the rail
    // follows its lower edge. Refocus once after that geometry settles; this
    // is a one-shot map transition, never a repeating focus poll.
    Timer {
        id: inputFocusSettler
        interval: 180
        repeat: false
        onTriggered: {
            if (win.visible && win.launcher)
                win.launcher.focusInput();
        }
    }
}
