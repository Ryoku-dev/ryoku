pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../lib/apps.js" as AppsLogic

// Where the catalogue comes from: the official Ryostore sources and the ones
// the user added, through `ryostore sources`. One change at a time; every
// change reloads the list and the catalogue.
Singleton {
    id: root

    property var official: []
    property var mine: []
    property bool loading: false
    property bool loaded: false
    property string error: ""
    // the source a change is running on ("" for none, "+" while adding)
    property string busyId: ""
    property string changeError: ""
    property string changeErrorId: ""
    readonly property int attention: root.mine.filter(src => src.status === "error").length
            + root.official.filter(src => src.status === "error").length
    property string _listOut: ""
    property string _listErr: ""
    property string _changeOut: ""
    property string _changeErr: ""

    signal added(var source)

    function refresh() {
        if (listProc.running)
            return;
        root.loading = true;
        root._listOut = "";
        root._listErr = "";
        listProc.running = true;
    }

    function add(url, name) {
        const args = ["add", String(url).trim()];
        if (String(name || "").trim() !== "")
            args.push("--name", String(name).trim());
        return root._change("+", args);
    }

    function remove(id) { return root._change(id, ["remove", String(id)]); }
    function setEnabled(id, on) { return root._change(id, [on ? "enable" : "disable", String(id)]); }

    function clearError() {
        root.changeError = "";
        root.changeErrorId = "";
    }

    function shutdown() {
        listProc.running = false;
        changeProc.running = false;
    }

    function _change(id, args) {
        if (root.busyId !== "" || changeProc.running)
            return false;
        root.busyId = String(id);
        root.clearError();
        root._changeOut = "";
        root._changeErr = "";
        changeProc.command = ["ryostore", "sources"].concat(args);
        changeProc.running = true;
        return true;
    }

    Component.onCompleted: refresh()

    Process {
        id: listProc
        command: ["ryostore", "sources", "list", "--json"]
        environment: Spawn.env
        stdout: StdioCollector { onStreamFinished: root._listOut = text }
        stderr: StdioCollector { onStreamFinished: root._listErr = text }
        onExited: code => {
            root.loading = false;
            if (code !== 0) {
                root.error = root._listErr.trim().split("\n").pop() || I18n.tr("The sources could not be read.");
                return;
            }
            try {
                const lists = AppsLogic.parseSources(root._listOut);
                root.official = lists.official;
                root.mine = lists.mine;
                root.error = "";
                root.loaded = true;
            } catch (e) {
                root.error = I18n.tr("The sources could not be read.");
            }
        }
    }

    Process {
        id: changeProc
        environment: Spawn.env
        stdout: StdioCollector { onStreamFinished: root._changeOut = text }
        stderr: StdioCollector { onStreamFinished: root._changeErr = text }
        onExited: code => {
            const id = root.busyId;
            root.busyId = "";
            if (code !== 0) {
                root.changeErrorId = id;
                root.changeError = root._changeErr.trim().split("\n").pop()
                        || I18n.tr("The source could not be changed.");
                return;
            }
            if (id === "+") {
                let made = null;
                try {
                    made = JSON.parse(root._changeOut);
                } catch (e) {
                }
                root.added(made);
            }
            root.refresh();
            Store.refresh(true);
        }
    }
}
