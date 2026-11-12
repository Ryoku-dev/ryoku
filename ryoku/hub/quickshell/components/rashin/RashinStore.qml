import QtQuick
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../../Singletons"

// what Rashin is doing, read with `rashin status --json`, which never starts its
// daemon; turning it on or off goes through `ryoku-hub rashin`, which also
// installs or removes the package
QtObject {
    id: store

    property bool loaded: false
    property bool installed: false
    property bool enabled: false
    property bool running: false
    property string version: ""
    property string askRoute: "auto"
    property string harness: ""
    property int providers: 0
    property int harnesses: 0
    property int indexes: 0

    // "on" or "off" while a switch runs
    property string switching: ""
    property string error: ""
    // the whole line the page shows for error, with its next step
    property string message: ""
    property string readError: ""
    property string lastAsk: ""
    // Cancel past 10 s stops the switch; the status read after it says where Rashin stands
    property bool cancelRequested: false
    property bool stopped: false
    property var switchRun: null

    readonly property string state: !store.loaded ? "" : !store.installed ? "absent"
        : store.running ? "running" : store.enabled ? "idle" : "off"

    function read(key) {
        if (key === "loaded")
            return store.loaded;
        if (key === "state")
            return store.loaded ? store.state : undefined;
        return undefined;
    }

    function write(key, v, done) {
        done(false, "");
    }

    function refresh() {
        if (!statusProc.running)
            statusProc.running = true;
    }

    function turn(on) {
        if (store.switching !== "")
            return;
        store.error = "";
        store.message = "";
        store.stopped = false;
        store.cancelRequested = false;
        store.lastAsk = on ? "on" : "off";
        store.switching = store.lastAsk;
        store.switchRun = Changes.run(["ryoku-hub", "rashin", store.lastAsk], function (ok, out, err, code) {
            store.switchRun = null;
            store.switching = "";
            if (store.cancelRequested) {
                store.cancelRequested = false;
                store.stopped = true;
                store.error = "stopped";
                store.message = I18n.tr("Stopped before it finished. The switch shows whether Rashin is on now.");
            } else if (!ok) {
                var last = err.trim().split("\n").pop().replace(/^ryoku-hub:\s*/, "").replace(/\.+$/, "");
                store.error = last || I18n.tr("it stopped with code %1").arg(code);
                var on = store.lastAsk === "on";
                // the frame's own words for a program that could not run already name the next step
                if (code === -1)
                    store.message = (on ? I18n.tr("Rashin did not turn on. %1.") : I18n.tr("Rashin did not turn off. %1.")).arg(last);
                else
                    store.message = (on ? I18n.tr("Rashin did not turn on: %1.") : I18n.tr("Rashin did not turn off: %1.")).arg(store.error)
                        + " " + I18n.tr("Try again, and enter your password when it is asked for.");
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
        var s = null;
        try {
            s = JSON.parse(text);
        } catch (e) {
            s = null;
        }
        if (code !== 0 || (text.trim().length > 0 && (!s || typeof s !== "object"))) {
            store.readError = I18n.tr("Rashin's status could not be read.");
            Changes.touch();
            return;
        }
        store.readError = "";
        if (!s) {
            store.installed = false;
            store.enabled = false;
            store.running = false;
        } else {
            store.installed = true;
            store.enabled = s.enabled === true;
            store.running = s.running === true;
            store.version = String(s.version || "");
            store.providers = s.gateway && typeof s.gateway.providers === "number" ? s.gateway.providers : 0;
            store.harnesses = typeof s.harnesses === "number" ? s.harnesses : 0;
            store.indexes = typeof s.indexes === "number" ? s.indexes : 0;
            var settings = s.settings || {};
            store.askRoute = settings.ask && settings.ask.model ? String(settings.ask.model) : "auto";
            store.harness = settings.agent && settings.agent.harness ? String(settings.agent.harness) : "";
        }
        store.loaded = true;
        Changes.touch();
    }

    Component.onCompleted: {
        Changes.registerStore("rashin", store);
        store.refresh();
    }

    // a missing rashin command prints nothing, which reads as not installed
    property Process statusReader: Process {
        id: statusProc
        command: ["sh", "-c", "command -v rashin >/dev/null 2>&1 || exit 0; exec rashin status --json"]
        environment: Spawn.env
        stdout: StdioCollector { id: statusOut }
        stderr: StdioCollector {}
        onExited: (code, status) => store.adopt(statusOut.text, code)
    }
}
