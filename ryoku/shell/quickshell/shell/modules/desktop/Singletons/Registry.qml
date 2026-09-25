pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io

// Plugin discovery + runtime registry. discover.sh merges receipt-owned
// installed manifests with the user's plugins.json placement and emits enabled
// entries with their exact installed version. The registry watches placement
// and Store revision state, so Settings edits and product install/update/remove
// transactions retune the running shell without a restart.
//
// discover.sh path = RYOKU_SHELL_DIR in dev, else the installed quickshell
// tree.
Singleton {
    id: root

    property var plugins: []
    property bool ready: false

    readonly property string _shellDir: Quickshell.env("RYOKU_SHELL_DIR")
    readonly property string _script: (_shellDir && _shellDir.length > 0)
        ? _shellDir + "/quickshell/plugins/discover.sh"
        : (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/quickshell/plugins/discover.sh"
    readonly property string _stateHome: Quickshell.env("XDG_STATE_HOME")
        || ((Quickshell.env("HOME") + "/.local/state"))
    readonly property string _revision: root._stateHome + "/ryoku/store/revision.json"

    // one sweep per burst. a dragged tile writes placement on release, a resize
    // writes it again as the wheel settles, and a Store transaction writes its
    // revision in the same moment; without this each write re-ran discovery and
    // re-resolved every tile on the desktop while the pointer was still moving.
    function reload() {
        coalesce.restart();
    }
    Timer {
        id: coalesce
        // under the slot guard's 90 ms: a dropped tile keeps its drag position
        // until this settles, so the sweep must land before the guard expires
        // or the tile flashes back to its old spot for a frame.
        interval: 40
        onTriggered: {
            discoverProc.running = false;
            discoverProc.running = true;
        }
    }
    function handleRevision(raw) {
        try {
            const revision = JSON.parse(raw || "{}");
            if (revision.category === "plugins")
                root.reload();
        } catch (error) {
        }
    }

    Process {
        id: discoverProc
        command: ["bash", root._script]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    root.plugins = JSON.parse(text || "[]");
                } catch (e) {
                    root.plugins = [];
                }
                root.ready = true;
            }
        }
    }

    // watch the user's placement file. any enable / placement / settings
    // change re-discovers, so the shell retunes live like the rest of the
    // desktop.
    FileView {
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/plugins.json"
        watchChanges: true
        printErrors: false
        onFileChanged: root.reload()
        onLoaded: root.reload()
    }

    FileView {
        id: revisionFile
        path: root._revision
        watchChanges: true
        atomicWrites: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: root.handleRevision(text())
    }
}
