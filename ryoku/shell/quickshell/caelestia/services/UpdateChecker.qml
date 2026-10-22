pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io

/// Update state for the bar indicator and its popout, read from the ryoku-shell
/// `updates` topic. The daemon runs the check, so this only mirrors the frame and
/// asks for a new check on request.
Singleton {
    id: root

    readonly property string sockPath: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-shell.sock"

    property var _frame: ({})

    readonly property int pendingCount: root._frame.pendingUpdates || 0
    readonly property bool hasUpdate: root.pendingCount > 0
    readonly property string currentBranch: root._frame.channel || ""
    // A box on a named release is told "new version", one on a checkout counts commits.
    readonly property bool versionSummaryMode: !!(root._frame.release && root._frame.channelRelease)
    readonly property int checkIntervalMs: 1800000
    property bool checkingUpdates: false
    property double lastCheckMs: 0

    function checkUpdates(branch: var, persistBranch: var): void {
        root.checkingUpdates = true;
        root._call("updates.check", {});
    }

    function _apply(line: string): void {
        try {
            root._frame = JSON.parse(line);
            root.checkingUpdates = false;
            root.lastCheckMs = Date.now();
        } catch (e) {
            // A malformed frame keeps the last good state.
        }
    }

    function _call(method: string, args: var): void {
        ctl.queued += "call " + method + " " + JSON.stringify(args) + "\n";
        if (ctl.connected)
            ctl.flushQueued();
        else
            ctl.connected = true;
    }

    Timer {
        id: retry

        interval: 2000
        onTriggered: if (!sub.connected) sub.connected = true
    }

    Socket {
        id: sub

        path: root.sockPath
        parser: SplitParser { onRead: line => root._apply(line) }
        Component.onCompleted: connected = true
        onConnectionStateChanged: {
            if (connected) {
                write("subscribe updates\n");
                flush();
            } else {
                retry.restart();
            }
        }
    }

    Socket {
        id: ctl

        property string queued: ""

        path: root.sockPath

        function flushQueued(): void {
            if (queued.length === 0)
                return;
            write(queued);
            flush();
            queued = "";
        }

        onConnectionStateChanged: if (connected) flushQueued()
    }
}
