import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../../Singletons"

// the screenshot tool's own settings file as a Changes store; the tool reads it
// each time it opens, and keys this page does not show are kept as they are
QtObject {
    id: store

    readonly property var defaults: ({
        blurRadius: 64, mosaicBlock: 12, zoomFactor: 2.0, copyOnSave: false,
        saveDir: "", delaySeconds: 0, recordMic: false
    })
    property var data: ({})
    property bool loaded: false

    function read(key) {
        if (!store.loaded)
            return undefined;
        var v = store.data[key];
        return v === undefined || v === null ? store.defaults[key] : v;
    }

    function write(key, v, done) {
        var next = Object.assign({}, store.data);
        next[key] = v;
        store.data = next;
        Changes.touch();
        Changes.writeText(file, JSON.stringify(Object.assign({}, store.defaults, next), null, 2), function (ok, error) {
            if (!ok)
                file.reload();
            done(ok, ok ? "" : I18n.tr("The screenshot tool's settings could not be saved in %1. Check that you can write to that folder, then press Retry.").arg(store.folder()));
        });
    }

    // the settings folder as people see it, with ~ for home
    function folder() {
        var dir = String(file.path).replace(/\/[^/]*$/, "");
        var home = Quickshell.env("HOME") || "";
        return home !== "" && dir.indexOf(home + "/") === 0 ? "~" + dir.slice(home.length) : dir;
    }

    function adopt(text) {
        try {
            var o = JSON.parse(text);
            store.data = o && typeof o === "object" && !Array.isArray(o) ? o : {};
        } catch (e) {
            store.data = {};
        }
        store.loaded = true;
        Changes.touch();
    }

    property FileView file: FileView {
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/ryoshot.json"
        atomicWrites: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: store.adopt(text())
        onLoadFailed: store.adopt("{}")
    }

    Component.onCompleted: Changes.registerStore("ryoshot", store)
}
