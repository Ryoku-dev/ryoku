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
// "binary" entry has its mime and byte size. at is when it was copied, in ms.
// Copy re-sets the Wayland selection, there is no synthetic paste.
// read marks a picture whose text has been read, and reading counts the pictures
// still to read while the reader is awake. The text itself stays out of the frame.
Singleton {
    id: root

    property var entries: []
    property int reading: 0

    function apply(frame) {
        root.entries = frame && Array.isArray(frame.entries) ? frame.entries : [];
        root.reading = frame ? Number(frame.reading) || 0 : 0;
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
    // the reader stays up two minutes past the last wake, the palette wakes it while open and as it closes
    function wake() { root.send("clipboard.wake", {}); }
    function words(cb) {
        Bus.call("clipboard.words", {}, (ok, result) => cb(ok && result && typeof result === "object" ? result : {}));
    }

    function send(method, args) {
        Bus.call(method, args);
    }

    BusTopic {
        id: topicFeed
        topic: "clipboard"
        onUpdated: root.apply(topicFeed.data)
    }

    Connections {
        target: Bus
        function onConnectedChanged() {
            if (!Bus.connected) {
                root.entries = [];
                root.reading = 0;
            }
        }
    }
}
