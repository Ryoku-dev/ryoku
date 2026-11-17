import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui as RyokuUi
import Ryoku.Ui.Singletons
import "../../Singletons"
import "apps.js" as Apps

// the apps on this computer (`gpk apps`) and Ryoku's own (`ryoku apps list`), with the one
// removal or update that runs at a time. It lives in Changes so a removal keeps going, and is
// shown again, after the page is left. It has no settings of its own: showing an app in app
// search is the launcher's hidden list
Scope {
    id: store

    property var apps: []
    property bool appsLoaded: false
    property string appsError: ""
    // the gpk here is older than the one that lists apps, and the version it names ("" when it
    // named none)
    property bool gpkOld: false
    property string gpkHave: ""
    property var ryoku: []
    property bool ryokuLoaded: false
    property string ryokuError: ""
    readonly property alias job: appJob
    // a fresh read skips gpk's cache, after something was removed or updated
    property bool fresh: false

    // quitting waits a little for a removal or update to end
    readonly property bool unsettled: appJob.running

    function read(key) {
        return undefined;
    }

    function write(key, value, done) {
        done(false, I18n.tr("Installed apps keeps no settings of its own."));
    }

    // gpk's version first: an older gpk takes `apps` for a search across every repository,
    // which is slow and lists no app
    function refresh() {
        if (!versionProc.running && !appsProc.running) {
            versionProc.started = false;
            versionProc.running = true;
        }
        if (!ryokuProc.running)
            ryokuProc.running = true;
    }

    function readApps() {
        appsProc.command = store.fresh ? ["gpk", "apps", "--json", "--no-cache"] : ["gpk", "apps", "--json"];
        store.fresh = false;
        appsProc.running = true;
    }

    Process {
        id: versionProc
        property bool started: false
        command: ["gpk", "--version"]
        stdout: StdioCollector {
            id: versionOut
        }
        onStarted: versionProc.started = true
        onExited: (code, status) => {
            store.gpkOld = code === 0 && !Apps.gpkListsApps(versionOut.text);
            store.gpkHave = store.gpkOld ? Apps.gpkVersion(versionOut.text) : "";
            if (!store.gpkOld) {
                store.readApps();
                return;
            }
            store.apps = [];
            store.appsError = "";
            store.appsLoaded = true;
        }
        // gpk not there at all
        onRunningChanged: {
            if (!versionProc.running && !versionProc.started) {
                store.gpkOld = false;
                store.appsError = I18n.tr("The list of apps could not be read: gpk is missing. Update Ryoku, then try again.");
                store.appsLoaded = true;
            }
        }
    }

    function app(id) {
        return store.apps.find(a => a.id === id) || null;
    }

    function ryokuApp(id) {
        return store.ryoku.find(a => a.id === id) || null;
    }

    function remove(id) {
        var a = store.app(id);
        var how = Apps.removeCommand(a);
        if (!how || how.kind !== "run")
            return false;
        return appJob.start(id, "remove", a.name, how.argv);
    }

    // a Ryoku app is updated or removed by its catalogue id, as root
    function changeRyoku(id, verb) {
        var a = store.ryokuApp(id);
        if (!a)
            return false;
        return appJob.start("ryoku:" + id, verb, a.name, Apps.ryokuCommand(verb, id));
    }

    Component.onCompleted: Changes.registerStore("installed-apps", store)

    RyokuUi.PackageJob {
        id: appJob
        onFinished: (target, action, ok) => {
            // what is on the computer now, whether or not it went to the end
            store.fresh = true;
            store.refresh();
        }
    }

    Process {
        id: appsProc
        command: ["gpk", "apps", "--json"]
        stdout: StdioCollector {
            id: appsOut
        }
        stderr: StdioCollector {
            id: appsErr
        }
        onExited: (code, status) => {
            if (code !== 0) {
                // a gpk that does not know `apps` yet, whatever version it named
                if (/unknown (sub)?command|unknown option|flag provided but not defined/.test(appsErr.text)) {
                    store.gpkOld = true;
                    store.apps = [];
                    store.appsError = "";
                    store.appsLoaded = true;
                    return;
                }
                var said = appsErr.text.trim().split("\n").pop();
                store.appsError = I18n.tr("The list of apps could not be read.") + (said ? " " + I18n.tr("It said: %1").arg(said) : "");
                store.appsLoaded = true;
                return;
            }
            try {
                store.apps = Apps.parseApps(appsOut.text);
                store.appsError = "";
            } catch (e) {
                store.gpkOld = e.message === "old gpk";
                store.apps = [];
                store.appsError = store.gpkOld ? "" : I18n.tr("The list of apps could not be read. Update Ryoku, then try again.");
            }
            store.appsLoaded = true;
        }
    }

    Process {
        id: ryokuProc
        command: ["ryoku", "apps", "list", "--json"]
        stdout: StdioCollector {
            id: ryokuOut
        }
        onExited: (code, status) => {
            try {
                store.ryoku = code === 0 ? Apps.parseRyoku(ryokuOut.text) : [];
                store.ryokuError = code === 0 ? "" : I18n.tr("Ryoku's own apps could not be read.");
            } catch (e) {
                store.ryoku = [];
                store.ryokuError = I18n.tr("Ryoku's own apps could not be read.");
            }
            store.ryokuLoaded = true;
        }
        onRunningChanged: {
            if (!ryokuProc.running && ryokuProc.processId === null && !store.ryokuLoaded) {
                store.ryokuError = I18n.tr("Ryoku's own apps could not be read.");
                store.ryokuLoaded = true;
            }
        }
    }
}
