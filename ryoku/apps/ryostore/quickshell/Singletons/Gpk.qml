pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../lib/apps.js" as AppsLogic

// Which gpk this computer has. Installs and removals read gpk 0.7.0's progress
// events, so an older gpk (searching still works) is named up front instead of
// failing halfway through a job. Read once at start, then from every search.
Singleton {
    id: root

    property string version: ""
    readonly property bool canInstall: AppsLogic.gpkCanInstall(root.version)
    property string _out: ""

    // a search envelope carries the version too; keep the newest word on it
    function seen(version) {
        if (String(version || "") !== "")
            root.version = String(version);
    }

    Component.onCompleted: probe.running = true

    Process {
        id: probe
        command: ["gpk", "--version"]
        environment: Spawn.env
        stdout: StdioCollector { onStreamFinished: root._out = text }
        onExited: code => {
            if (code === 0 && AppsLogic.gpkVersion(root._out))
                root.seen(root._out.trim());
        }
    }
}
