pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import "lib/memory.js" as Memory

// The shared flat memory map, flushed synchronously before Kit quits.
Item {
    id: root

    readonly property string stateDir: (Quickshell.env("XDG_STATE_HOME")
        || (Quickshell.env("HOME") + "/.local/state")) + "/ryoku"
    property var values: ({})
    property bool ready: false
    property bool active: false
    property bool dirty: false
    // the folder is only made when the first read found no file
    property bool folderKnown: true

    function get(key, fallback) {
        return Memory.get(root.values, key, fallback);
    }

    function set(key, value) {
        const next = Memory.set(root.values, key, value);
        if (next === root.values)
            return;
        root.values = next;
        root.dirty = true;
        if (root.active)
            writeSoon.restart();
        else
            root.flush();
    }

    function load() {
        // what is waiting to be written is newer than the file
        if (root.dirty)
            return;
        root.values = Memory.parse(file.text());
        root.ready = true;
    }

    function write() {
        writeSoon.stop();
        if (!root.dirty)
            return;
        if (!root.folderKnown) {
            makeFolder.running = true;
            return;
        }
        root.dirty = false;
        file.setText(JSON.stringify(root.values));
    }

    function flush() {
        writeSoon.stop();
        if (!root.dirty)
            return;
        file.blockWrites = true;
        root.write();
        file.blockWrites = false;
    }

    // closing Kit inside the write delay must not lose the last change
    Component.onDestruction: {
        if (!root.dirty || !root.folderKnown)
            return;
        file.blockWrites = true;
        file.setText(JSON.stringify(root.values));
    }

    Timer {
        id: writeSoon
        interval: 60
        onTriggered: root.write()
    }

    Process {
        id: makeFolder
        command: ["mkdir", "-p", root.stateDir]
        onExited: code => {
            root.folderKnown = code === 0;
            if (root.folderKnown)
                root.write();
        }
    }

    FileView {
        id: file
        path: root.stateDir + "/kit.json"
        blockLoading: true
        watchChanges: root.active
        atomicWrites: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: root.load()
        onLoadFailed: error => {
            if (error === FileViewError.FileNotFound)
                root.folderKnown = false;
            root.load();
        }
    }
}
