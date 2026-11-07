pragma Singleton

import QtQuick
import Quickshell
import Ryoku.Ui
import Ryoku.Ui.Singletons

// QML view of the daemon `clipboard` topic. The selection watcher and the whole
// history live in ryoku-shell (clipboard.go), so QML never runs an external
// history tool: the bus streams a full {entries:[...]} frame, newest first, on
// every change, and copy/delete/clear intents ride back as bus calls. Each
// entry already carries its resolved view: a "text"
// entry has a 200-character preview, an "image" entry has a persisted thumbnail
// file and size (thumb, thumbW/thumbH) plus its source pixel size (w/h), and a
// "binary" entry has its mime and byte size.
// Copy re-sets the Wayland selection; there is no synthetic paste. Contract 07
// sec 2.2/3/4.2.
Singleton {
    id: root

    property var entries: []

    function apply(frame) {
        root.entries = frame && Array.isArray(frame.entries) ? frame.entries : [];
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
        Bus.call(method, args);
    }

    BusTopic {
        id: topicFeed
        topic: "clipboard"
        onUpdated: root.apply(topicFeed.data)
    }

    // The history lives in the daemon; clear it while the daemon is away and let
    // the bus resync the panel when it returns.
    Connections {
        target: Bus
        function onConnectedChanged() {
            if (!Bus.connected)
                root.entries = [];
        }
    }
}
