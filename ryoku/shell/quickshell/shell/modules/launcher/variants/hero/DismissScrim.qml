pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import "../../shared/lib/lifecycle.js" as Lifecycle

// Where the compositor has no focus-grab protocol, a full-screen scrim closes
// the launcher on an outside press. It is a top-level layer surface (a sibling
// of the launcher and the window rail), not a window inside the launcher's own
// PanelWindow: a nested layer surface is the lifecycle quickshell faults on
// under compositors with no grab protocol.
//
// `surface` is the active LauncherSurface, or null. The mapped state flips one
// turn after the driving condition changes, and the close is deferred, so the
// window is never unmapped from inside its own pointer handler.
PanelWindow {
    id: win

    required property var surface
    required property var screenData

    property bool wanted: !!surface && surface.dismissWanted
    property bool mapped: false

    screen: screenData
    visible: mapped
    color: "transparent"
    exclusionMode: ExclusionMode.Ignore
    WlrLayershell.namespace: "launcher-dismiss"
    WlrLayershell.layer: WlrLayer.Top
    anchors { top: true; bottom: true; left: true; right: true }

    onWantedChanged: Qt.callLater(function () {
        win.mapped = win.wanted;
    })

    MouseArea {
        anchors.fill: parent
        onPressed: {
            if (!win.surface || !win.surface.invocationSurface
                    || win.surface.lifecycleState.phase === Lifecycle.PHASES.CLOSING)
                return;
            const target = win.surface;
            const generation = target.dismissGeneration;
            const monitor = target.surfaceMonitor;
            Qt.callLater(function () {
                target.requestClose(generation, monitor);
            });
        }
    }
}
