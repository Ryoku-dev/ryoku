pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io
import Caelestia
import caelestia.services

// The workspace-tracker effect is compiled against KWin's internals, so a KDE
// update can leave the installed binary unloadable and the shell silently
// without per-output desktops. The check reports that once per session.
Singleton {
    id: root

    readonly property string checkScript: Quickshell.shellPath("scripts/check-workspace-tracker.sh")

    // KWin loads its effects long before the shell starts, so the delay only
    // covers a login that brings both up at once.
    Timer {
        interval: 5000
        running: true

        onTriggered: check.running = true
    }

    Process {
        id: check

        command: ["bash", root.checkScript]

        onExited: code => {
            if (code !== 3)
                return;

            console.warn("[WorkspaceTrackerGuard] the effect is enabled but not loaded");
            Toaster.toast(qsTr("Workspace tracker effect is not running"), qsTr("KWin stopped loading it after a KDE update. Run caelestia update, then log out and back in."), "extension_off", Toast.Warning);
        }
    }
}
