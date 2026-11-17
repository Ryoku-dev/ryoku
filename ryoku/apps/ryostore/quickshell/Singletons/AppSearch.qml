pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../lib/apps.js" as AppsLogic

// The Apps search: one `gpk search --json` across Flathub, the system packages
// and the AUR. A newer query waits for the running one, then replaces it.
Singleton {
    id: root

    property var rows: []
    property string resultQuery: ""
    property bool searching: false
    property string error: ""
    property string _wanted: ""
    property string _running: ""
    property string _out: ""
    property string _err: ""

    function search(query) {
        const wanted = String(query || "").trim();
        if (wanted === root._wanted && (root.searching || root.resultQuery === wanted))
            return;
        root._wanted = wanted;
        root.error = "";
        if (wanted === "") {
            root.clear();
            return;
        }
        root.searching = true;
        if (!proc.running)
            root._start();
    }

    // the same query again, after an install or removal changed what is installed
    function again() {
        if (root._wanted === "")
            return;
        root.resultQuery = "";
        root.searching = true;
        if (!proc.running)
            root._start();
    }

    function clear() {
        root._wanted = "";
        root.rows = [];
        root.resultQuery = "";
        root.searching = false;
        root.error = "";
    }

    function shutdown() {
        root.clear();
        proc.running = false;
    }

    function _start() {
        root._running = root._wanted;
        root._out = "";
        root._err = "";
        proc.command = AppsLogic.searchCommand(root._running);
        proc.running = true;
    }

    Connections {
        target: Jobs.app
        function onFinished(key, action, ok) {
            if (!String(key).startsWith("ryoku:"))
                root.again();
        }
    }

    Process {
        id: proc
        environment: Spawn.env
        stdout: StdioCollector { onStreamFinished: root._out = text }
        stderr: StdioCollector { onStreamFinished: root._err = text }
        onExited: code => {
            const answered = root._running;
            root._running = "";
            if (root._wanted === "") {
                root.searching = false;
                return;
            }
            if (root._wanted !== answered) {
                root._start();
                return;
            }
            root.searching = false;
            root.resultQuery = answered;
            // gpk says "no" with exit 2 and no envelope when nothing matched
            if (code === 2) {
                root.rows = [];
                root.error = "";
                return;
            }
            if (code !== 0) {
                root.rows = [];
                root.error = root._err.trim().split("\n").pop() || I18n.tr("Search failed");
                return;
            }
            try {
                Gpk.seen(AppsLogic.searchVersion(root._out));
                root.rows = AppsLogic.parseSearch(root._out);
                root.error = "";
            } catch (e) {
                root.rows = [];
                root.error = I18n.tr("Search failed");
            }
        }
    }
}
