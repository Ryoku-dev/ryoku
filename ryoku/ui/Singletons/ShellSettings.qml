pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Bus
import "buspath.js" as BusPath

Singleton {
    id: root

    readonly property var data: root._data
    readonly property bool ready: root._ready
    // only set by the start-up read
    readonly property bool fileMissing: root._fileMissing

    // paths arrive here as QML sequences, test them with touches() or read lastPaths
    signal changed(var paths)
    readonly property var lastPaths: root._lastPaths

    function touches(path, paths) {
        return BusPath.touches(path, paths);
    }

    property var _data: ({})
    property var _lastPaths: null
    property bool _ready: false
    property bool _fileMissing: false
    property bool _started: false

    function get(path, fallback) {
        return BusPath.get(root._data, path, fallback);
    }

    function patch(path, value, cb) {
        return BusClient.call("settings.patch", { path: BusPath.join(path), value: value }, cb);
    }

    function _fromFile(atStart) {
        let next = null;
        try {
            const text = file.text();
            if (text && text.length) {
                const o = JSON.parse(text);
                if (o && typeof o === "object" && !Array.isArray(o))
                    next = o;
            }
        } catch (e) {
            next = null;
        }
        if (atStart)
            root._fileMissing = next === null && !file.loaded;
        if (next === null)
            return;
        // the file has no runtime keys so keep the last frame's
        const keep = ["caps", "deadKeys", "windowRuleActions", "appOverrideFields"];
        for (let i = 0; i < keep.length; i++)
            if (!(keep[i] in next) && root._data && keep[i] in root._data)
                next[keep[i]] = root._data[keep[i]];
        // comes back through _onFrame unless the daemon's frame is already in
        BusClient.seed("settings", next);
    }

    function _onFrame(data, info) {
        if (data === null || typeof data !== "object" || Array.isArray(data))
            return;
        root._data = data;
        root._lastPaths = info ? info.paths : null;
        root._ready = true;
        root.changed(root._lastPaths);
    }

    FileView {
        id: file
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/shell.json"
        blockLoading: true
        printErrors: false
        watchChanges: root._started && !BusClient.connected
        onFileChanged: reload()
        onLoaded: {
            if (root._started && !BusClient.connected)
                root._fromFile(false);
        }
    }

    Component.onCompleted: {
        BusClient.subscribe("settings", root._onFrame);
        root._fromFile(true);
        root._started = true;
    }
}
