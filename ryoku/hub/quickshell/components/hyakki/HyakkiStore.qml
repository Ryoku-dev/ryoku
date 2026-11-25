import QtQuick
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../../Singletons"

QtObject {
    id: store

    property bool loaded: false
    property bool installed: false
    property bool enabled: false
    property bool running: false
    property string version: ""
    property string switching: ""
    property string message: ""
    property string readError: ""
    property string lastAsk: ""
    property bool cancelRequested: false
    property var switchRun: null

    readonly property string state: !store.loaded ? "" : !store.installed ? "absent"
        : store.running ? "running" : store.enabled ? "idle" : "off"

    function read(key) {
        return key === "state" && store.loaded ? store.state : undefined;
    }

    function write(key, value, done) {
        done(false, "");
    }

    function refresh() {
        if (!statusProc.running)
            statusProc.running = true;
    }

    function turn(on) {
        if (store.switching !== "")
            return;
        store.message = "";
        store.cancelRequested = false;
        store.lastAsk = on ? "on" : "off";
        store.switching = store.lastAsk;
        store.switchRun = Changes.run(["ryoku-hub", "hyakki", store.lastAsk], function (ok, out, err, code) {
            store.switchRun = null;
            store.switching = "";
            if (store.cancelRequested) {
                store.cancelRequested = false;
                store.message = I18n.tr("Stopped before it finished. The switch shows whether Hyakki is on now.");
            } else if (!ok) {
                var why = err.trim().split("\n").pop().replace(/^ryoku-hub:\s*/, "");
                store.message = why || I18n.tr("Hyakki could not be changed. Try again.");
            }
            Changes.touch();
            store.refresh();
        }, { env: Spawn.env });
        Changes.touch();
    }

    function cancel() {
        if (store.switching === "" || store.cancelRequested)
            return;
        store.cancelRequested = true;
        if (store.switchRun)
            store.switchRun.running = false;
        Changes.touch();
    }

    function adopt(text, code) {
        var status = null;
        try {
            status = JSON.parse(text);
        } catch (e) {}
        if (code !== 0 || (text.trim().length > 0 && (!status || typeof status.enabled !== "boolean" || typeof status.running !== "boolean"))) {
            store.readError = I18n.tr("Hyakki's status could not be read. Check hyakki status --json in a terminal, then retry.");
            Changes.touch();
            return;
        }
        store.readError = "";
        store.installed = status !== null;
        store.enabled = status !== null && status.enabled === true;
        store.running = status !== null && status.running === true;
        store.version = status !== null ? String(status.version || "") : "";
        store.loaded = true;
        Changes.touch();
    }

    Component.onCompleted: {
        Changes.registerStore("hyakki", store);
        store.refresh();
    }

    // status does not start Oni; an absent executable prints nothing.
    property Process statusReader: Process {
        id: statusProc
        command: ["sh", "-c", "command -v hyakki >/dev/null 2>&1 || exit 0; exec hyakki status --json"]
        environment: Spawn.env
        stdout: StdioCollector { id: statusOut }
        stderr: StdioCollector {}
        onExited: code => store.adopt(statusOut.text, code)
    }
}
