pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io
import "../registry/progression.js" as Model

Singleton {
    id: root

    readonly property string directory: (Quickshell.env("XDG_DATA_HOME") ||
        ((Quickshell.env("HOME") || "") + "/.local/share")) + "/ryoku"
    property var data: Model.clean({})
    property bool ready: false
    property bool directoryReady: false
    property string error: ""
    property var pending: []
    readonly property int balance: data.balance || 0
    readonly property var achievements: Model.achievements
    readonly property var cosmetics: Model.cosmetics
    readonly property var activePalette: Model.palette(root.data)

    Process {
        running: true
        command: ["mkdir", "-p", root.directory]
        onExited: (code) => {
            if (code !== 0)
                root.error = "Could not create the progression folder";
            else {
                root.directoryReady = true;
                file.reload();
            }
        }
    }

    FileView {
        id: file
        path: root.directory + "/progression.json"
        watchChanges: true
        atomicWrites: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: {
            try {
                root.data = Model.clean(JSON.parse(text()));
                root.error = "";
            } catch (e) {
                root.error = "Progression data is invalid. Restore it before continuing.";
            }
            root.ready = root.directoryReady;
            root.flushPending();
        }
        onLoadFailed: {
            if (root.directoryReady && !root.ready) {
                root.data = Model.clean({});
                root.ready = true;
                root.flushPending();
            }
        }
    }

    function flushPending() {
        if (!root.ready || root.error !== "")
            return;
        var queued = root.pending;
        root.pending = [];
        for (var i = 0; i < queued.length; i++)
            root.award(queued[i]);
    }

    function save(state) {
        if (!root.ready || root.error !== "")
            return;
        var next = Model.clean(state);
        root.data = next;
        file.setText(JSON.stringify(next, null, 2) + "\n");
    }

    function award(id) {
        if (!root.ready) {
            root.pending = root.pending.concat([id]);
            return;
        }
        if (root.error !== "")
            return;
        var result = Model.unlock(root.data, id);
        if (result.changed)
            root.save(result.state);
    }

    function purchase(id) {
        if (!root.ready || root.error !== "")
            return;
        var result = Model.purchase(root.data, id);
        if (result.changed)
            root.save(result.state);
    }

    function equip(id) {
        if (!root.ready || root.error !== "")
            return;
        var result = Model.equip(root.data, id);
        if (result.changed)
            root.save(result.state);
    }

    function inspectProfile(profile) {
        var p = profile || {};
        if (p.text && (String(p.text.name || "").trim() !== "" || String(p.text.tagline || "").trim() !== ""))
            root.award("identity");
        if (p.hero && p.hero.kind && p.hero.kind !== "default")
            root.award("portrait");
        if (p.preset === "minimal" || p.homeView === "card" || p.heroSide === "left")
            root.award("layout");
        if (p.blocks && Object.keys(p.blocks).some(function(key) { return p.blocks[key] === false; }))
            root.award("layout");
    }
}
