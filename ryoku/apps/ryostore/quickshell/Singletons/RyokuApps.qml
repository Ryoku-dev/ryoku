pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../lib/apps.js" as AppsLogic

// Ryoku's own apps (Ryotunes, Ryoport, Ryomanager) from `ryoku apps list
// --json`: installed version, size, and what an update would bring.
Singleton {
    id: root

    property var apps: []
    property bool loading: false
    property bool loaded: false
    property string error: ""
    readonly property int updateCount: root.apps.filter(app => app.update).length
    property string _out: ""
    property string _err: ""

    function refresh() {
        if (proc.running)
            return;
        root.loading = true;
        root._out = "";
        root._err = "";
        proc.running = true;
    }

    function find(id) {
        for (let i = 0; i < root.apps.length; i++)
            if (root.apps[i].id === id)
                return root.apps[i];
        return null;
    }

    function shutdown() {
        proc.running = false;
    }

    Component.onCompleted: refresh()

    Connections {
        target: Jobs.app
        function onFinished(key, action, ok) {
            if (String(key).startsWith("ryoku:"))
                root.refresh();
        }
    }

    Process {
        id: proc
        command: ["ryoku", "apps", "list", "--json"]
        environment: Spawn.env
        stdout: StdioCollector { onStreamFinished: root._out = text }
        stderr: StdioCollector { onStreamFinished: root._err = text }
        onExited: code => {
            root.loading = false;
            if (code !== 0) {
                root.error = root._err.trim().split("\n").pop() || I18n.tr("Ryoku's apps could not be read.");
                return;
            }
            try {
                root.apps = AppsLogic.parseRyokuApps(root._out);
                root.error = "";
                root.loaded = true;
            } catch (e) {
                root.error = I18n.tr("Ryoku's apps could not be read.");
            }
        }
    }
}
