import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../../Singletons"

// ~/.config/ryoku/launcher.json as a Changes store; the launcher watches the same file
QtObject {
    id: store

    readonly property var factory: ({
        variant: "hero", radius: 16, bgBlur: 2, heroImage: "", heroStrength: 0.6, heroPosX: 0.5, heroPosY: 0.5,
        showWeather: true, showGreeting: true, resultSettleMs: 360, horizonMode: "auto", horizonColor: "#ffc777",
        overlayOpacity: 0.94, hiddenApps: [], columns: 8, matWidth: 144, sectionOrder: [], hiddenSections: [],
        pinnedApps: []
    })
    property var data: ({})
    property bool loaded: false
    // writes made in one tick go out as one save of the whole file; a reload waits until they land
    property var waiting: []
    property bool flushQueued: false
    property int saving: 0
    property bool changedMeanwhile: false

    function read(key) {
        var v = store.data[key];
        return v === undefined ? store.factory[key] : v;
    }

    function write(key, v, done) {
        var next = Object.assign({}, store.data);
        if (v === undefined)
            delete next[key];
        else
            next[key] = v;
        store.data = next;
        Changes.touch();
        store.waiting = store.waiting.concat([{ done: done, variant: key === "variant" }]);
        if (!store.flushQueued) {
            store.flushQueued = true;
            Qt.callLater(store.flush);
        }
    }

    function flush() {
        store.flushQueued = false;
        var jobs = store.waiting;
        store.waiting = [];
        if (jobs.length === 0)
            return;
        store.saving++;
        Changes.writeText(file, JSON.stringify(store.data, null, 2) + "\n", function (ok, error) {
            store.saving--;
            // a launcher a bar style brings counts toward that style being in use
            if (ok && jobs.some(j => j.variant))
                Bus.run("shells commit");
            var why = ok ? "" : I18n.tr("Could not save the launcher's settings. Check that your settings folder can be written, then try again.");
            jobs.forEach(j => j.done(ok, why));
            store.reloadIfChanged();
        });
    }

    // an edit from outside (the launcher, a restore) is read once the Hub's own saves have landed
    function fileChanged() {
        store.changedMeanwhile = true;
        store.reloadIfChanged();
    }
    function reloadIfChanged() {
        if (!store.changedMeanwhile || store.saving > 0 || store.flushQueued)
            return;
        store.changedMeanwhile = false;
        file.reload();
    }

    function parse(text) {
        try {
            var o = JSON.parse(text || "{}");
            store.data = o && typeof o === "object" && !Array.isArray(o) ? o : {};
        } catch (e) {
            store.data = {};
        }
        store.loaded = true;
        Changes.touch();
    }

    property FileView fileView: FileView {
        id: file
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/launcher.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        atomicWrites: true
        onFileChanged: store.fileChanged()
        onLoaded: store.parse(file.text())
        onLoadFailed: store.parse("")
    }

    Component.onCompleted: Changes.registerStore("launcher", store)
}
