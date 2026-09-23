pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io

// QML view of the daemon `clipboard` topic. The selection watcher and the whole
// history live in ryoku-shell (clipboard.go), so QML never runs an external
// history tool: `subscribe clipboard` streams a full {entries:[...]} frame,
// newest first, on every change, and copy/delete/clear intents ride back over a
// second connection. Each entry already carries its resolved view: a "text"
// entry has a 200-character preview, an "image" entry a persisted thumbnail file
// (thumb) with its pixel size, and a "binary" entry its mime and byte size.
// Copy re-sets the Wayland selection; there is no synthetic paste. Contract 07
// sec 2.2/3/4.2.
Singleton {
    id: root

    property var entries: []
    readonly property string sockPath: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-shell.sock"

    function apply(line) {
        try {
            const frame = JSON.parse(line);
            root.entries = Array.isArray(frame.entries) ? frame.entries : [];
        } catch (e) {
            // A malformed frame must never wedge the panel; keep the last good set.
        }
    }

    // Copy re-sets the Wayland selection to this entry and promotes it to the
    // front (no synthetic paste; the user pastes normally). Delete drops one
    // entry; clear empties the history.
    function copy(id) { root.send("clipboard.copy", { entry: id }); }
    function del(id) { root.send("clipboard.delete", { entry: id }); }
    function clear() { root.send("clipboard.clear", {}); }
    // Star toggles an entry between the history and Starred panes; a starred
    // entry survives a clear and is protected from history overflow.
    function star(id, starred) { root.send("clipboard.star", { entry: id, starred: starred }); }

    function send(method, args) {
        ctl.queued += "call " + method + " " + JSON.stringify(args) + "\n";
        if (ctl.connected)
            ctl.flushQueued();
        else
            ctl.connected = true;
    }

    // Subscription: connect, ask once, then stream. A second write to this
    // connection would half-close the stream (daemon rule), so calls use ctl.
    Socket {
        id: sub
        path: root.sockPath
        parser: SplitParser { onRead: line => root.apply(line) }
        Component.onCompleted: connected = true
        onConnectionStateChanged: {
            if (connected) {
                write("subscribe clipboard\n");
                flush();
            } else {
                root.entries = [];
                retry.restart();
            }
        }
    }

    // The daemon may be down when the shell loads (or restart under it); retry
    // quietly so the panel repopulates once it returns.
    Timer {
        id: retry
        interval: 2000
        onTriggered: if (!sub.connected) sub.connected = true
    }

    Socket {
        id: ctl
        path: root.sockPath
        property string queued: ""

        function flushQueued() {
            if (queued.length === 0)
                return;
            write(queued);
            flush();
            queued = "";
        }

        onConnectionStateChanged: if (connected) flushQueued()
    }
}
