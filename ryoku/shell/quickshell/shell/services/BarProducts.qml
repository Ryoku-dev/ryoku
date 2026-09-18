pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io

Singleton {
    id: root

    readonly property string stateRoot: (Quickshell.env("XDG_STATE_HOME")
        || (Quickshell.env("HOME") + "/.local/state")) + "/ryoku/store"
    property var rows: []
    property string revisionKey: ""
    property var failedStyles: ({})

    function parseRows(raw) {
        try {
            const value = JSON.parse(raw || "[]");
            if (!Array.isArray(value))
                return [];
            return value.filter(row => {
                const id = String(row?.id || "");
                const view = String(row?.view || "");
                return /^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(id)
                    && String(row?.version || "") !== ""
                    && String(row?.scene || "") === "Scene.qml"
                    && new RegExp("^barstyle-views/" + id + "/[a-f0-9]{64}$").test(view);
            });
        } catch (e) {
            return [];
        }
    }
    function fail(id) {
        // A builtin style (qsbar, kairos) ships with the shell and cannot be
        // legitimately broken; a load error is a transient hiccup (an update's
        // config/plugin swap, a cold-start import race), so never record it as
        // failed - Frame retries it instead of permanently dropping the bar to
        // the sumi rail.
        if (!id || root.builtins[id] || root.failedStyles[id])
            return;
        const next = Object.assign({}, root.failedStyles);
        next[id] = true;
        root.failedStyles = next;
    }
    function isBuiltin(id) {
        return !!(id && root.builtins[id]);
    }


    // Built-in folder styles ship inside the shell and resolve relative to the
    // Frame Loader, so no store install is needed. "sumi" stays the painted
    // frame scene (empty scene url); the other entries are shipped folder styles.
    readonly property var builtins: ({
        "qsbar": "barstyles/qsbar/Scene.qml",
        "chroma": "barstyles/chroma/Scene.qml",
        "kairos": "barstyles/kairos/Scene.qml"
    })

    function sceneUrl(id) {
        if (!id || id === "sumi" || root.failedStyles[id])
            return "";
        if (root.builtins[id])
            return root.builtins[id];
        for (const row of root.rows) {
            if (row.id === id)
                return "file://" + root.stateRoot + "/" + row.view + "/" + row.scene;
        }
        return "";
    }

    function loadRevision(raw) {
        try {
            const revision = JSON.parse(raw || "{}");
            if (revision.category !== "barstyles")
                return;
            const key = String(revision.revision || "") + ":" + String(revision.id || "");
            if (key === root.revisionKey)
                return;
            root.revisionKey = key;
            indexFile.reload();
        } catch (e) {
        }
    }

    FileView {
        id: indexFile
        path: root.stateRoot + "/barstyles.json"
        blockLoading: true
        watchChanges: true
        atomicWrites: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: {
            root.failedStyles = {};
            root.rows = root.parseRows(text());
        }
        onLoadFailed: root.rows = []
    }

    FileView {
        id: revisionFile
        path: root.stateRoot + "/revision.json"
        blockLoading: false
        watchChanges: true
        atomicWrites: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: root.loadRevision(text())
    }
}
