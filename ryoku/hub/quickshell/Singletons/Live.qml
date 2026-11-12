pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons

// live sources for the page list and Home, opened on first read and pushed from
// then on, so nothing polls: daemon topic frames, and JSON files under
// $XDG_CONFIG_HOME/ryoku that are not part of shell.json
Singleton {
    id: root

    property int revision: 0
    // topic -> last frame; shared with the bus, never mutate one
    property var frames: ({})
    property var tokens: ({})
    // file name -> parsed JSON, and the watchers
    property var files: ({})
    property var watchers: ({})

    readonly property string configHome: Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")

    function topic(name) {
        root.revision;
        if (!name)
            return undefined;
        if (root.tokens[name] === undefined) {
            root.tokens[name] = 0;
            Qt.callLater(root.listen, name);
        }
        return root.frames[name];
    }

    function listen(name) {
        root.tokens[name] = Bus.subscribe(name, function (data) {
            root.frames[name] = data;
            root.revision++;
        });
    }

    // a JSON file under ~/.config/ryoku by name ("launcher.json"), undefined until read or when absent
    function config(name) {
        return root.file("json:" + name, "ryoku/" + name, true);
    }

    // a plain file under ~/.config ("qylock/theme"), trimmed; "" when empty, undefined when absent
    function text(path) {
        return root.file("text:" + path, path, false);
    }

    // whether text(path) has been read: undefined then means the file is absent
    function textRead(path) {
        root.revision;
        return root.settled["text:" + path] === true;
    }
    property var settled: ({})

    function file(key, path, json) {
        root.revision;
        if (!path)
            return undefined;
        if (root.watchers[key] === undefined) {
            root.watchers[key] = null;
            Qt.callLater(root.watch, key, path, json);
        }
        return root.files[key];
    }

    function watch(key, path, json) {
        root.watchers[key] = fileWatcher.createObject(root, { key: key, json: json, path: root.configHome + "/" + path });
    }

    function settle(key, json, ok, text) {
        var v = undefined;
        if (ok && json) {
            try {
                v = JSON.parse(text);
            } catch (e) {
                v = undefined;
            }
        } else if (ok) {
            v = text.trim();
        }
        root.files[key] = v;
        root.settled[key] = true;
        root.revision++;
    }

    Component {
        id: fileWatcher
        FileView {
            id: view
            property string key: ""
            property bool json: true
            watchChanges: true
            printErrors: false
            onLoaded: root.settle(view.key, view.json, true, view.text())
            onLoadFailed: root.settle(view.key, view.json, false, "")
            onFileChanged: view.reload()
        }
    }
}
