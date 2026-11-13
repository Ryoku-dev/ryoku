import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../../Singletons"

// voice typing as a Changes store: `ryoku-hub dictation` owns Ryoku's own
// dictation settings file and model store; the store also runs model downloads,
// so one keeps going while you look at another page
QtObject {
    id: store

    property bool loaded: false
    // the engine binary: a Ryoku package on every edition, so this is a package
    // question rather than an AUR one
    property bool installed: false
    property bool packageAvailable: false
    property string packageReason: ""

    property bool enabled: true
    property string device: "auto"
    property string mode: "paste"
    property string source: ""
    property string language: ""
    property int silenceMs: 1500
    property int maxDurationSecs: 120
    property int typeDelayMs: 2
    property string model: ""
    property var models: []
    property var sources: []
    property string gpu: "none"
    property string backends: "cpu"
    // what the running daemon says it is using, e.g. "Vulkan0" or "CPU"
    property string activeBackend: ""

    property string state: "stopped"

    property string downloading: ""
    property real percent: -1
    property bool cancelRequested: false
    property string removing: ""
    property string modelError: ""
    property string failedDownload: ""
    property string failedRemove: ""
    // why the state could not be read; the page shows it with Retry until a read succeeds
    property string readError: ""
    property string notice: ""

    readonly property var current: store.modelFor(store.activeId())

    // the picker's first entry means "whatever PipeWire picks"
    readonly property var sourceNames: {
        var names = [I18n.tr("System default")];
        for (var i = 0; i < store.sources.length; i++)
            names.push(store.sources[i].description + (store.sources[i].default === true ? "  ·  " + I18n.tr("default") : ""));
        return names;
    }

    // the picker's first entry is "System default", so the device list is
    // shifted by one
    function sourceNameAt(index) {
        if (index <= 0 || index > store.sources.length)
            return "";
        return store.sources[index - 1].name;
    }

    function basename(path) {
        var t = String(path || "");
        var slash = t.lastIndexOf("/");
        return slash === -1 ? t : t.substring(slash + 1);
    }

    function modelFor(id) {
        for (var i = 0; i < store.models.length; i++)
            if (store.models[i].id === id)
                return store.models[i];
        return null;
    }

    // which catalogue entry the daemon is actually using: an explicit path wins,
    // otherwise the shipped model
    function activeId() {
        var path = String(store.model || "");
        if (path.length === 0) {
            for (var i = 0; i < store.models.length; i++)
                if (store.models[i].bundled === true)
                    return store.models[i].id;
            return "";
        }
        var name = store.basename(path);
        for (var j = 0; j < store.models.length; j++)
            if (store.models[j].path === path || store.basename(store.models[j].path) === name)
                return store.models[j].id;
        return "";
    }

    function usableWith(id) {
        var m = store.modelFor(id);
        return m !== null && (m.bundled === true || m.installed === true);
    }

    // "model@<id>" is one model row's way to choose the model: the same value as "model"
    function read(key) {
        if (key.indexOf("model@") === 0)
            key = "model";
        switch (key) {
        case "loaded": return store.loaded;
        case "installed": return store.loaded ? store.installed : undefined;
        case "enabled": return store.loaded ? store.enabled : undefined;
        case "model": return store.loaded ? store.activeId() : undefined;
        case "modelLabel": return store.current ? store.current.label : undefined;
        case "device": return store.loaded ? store.device : undefined;
        case "mode": return store.loaded ? store.mode : undefined;
        case "source": return store.loaded ? store.source : undefined;
        case "language": return store.loaded ? store.language : undefined;
        case "silenceMs": return store.loaded ? store.silenceMs : undefined;
        case "maxDurationSecs": return store.loaded ? store.maxDurationSecs : undefined;
        case "typeDelayMs": return store.loaded ? store.typeDelayMs : undefined;
        }
        return undefined;
    }

    // every key goes out as one snapshot, so a page reload never sees half an edit
    function write(key, v, done) {
        if (key.indexOf("model@") === 0)
            key = "model";
        if (key === "model") {
            var selected = store.modelFor(String(v));
            if (selected)
                v = selected.path;
        }
        var numeric = { "silenceMs": true, "maxDurationSecs": true, "typeDelayMs": true };
        var before = numeric[key] ? store[key] : String(store[key]);
        store[key] = numeric[key] ? Number(v) : String(v);
        Changes.touch();
        var body = {};
        body[key] = store[key];
        store.send(body, function (ok, err) {
            if (!ok) {
                store[key] = before;
                Changes.touch();
            }
            done(ok, ok ? "" : store.sentence(I18n.tr("Voice typing did not take the change."), err));
            store.reload();
        });
    }

    // the last line a command printed, without the program's own prefix
    function reason(err) {
        var t = String(err || "").trim().split("\n").filter(l => l.trim().length > 0).pop() || "";
        return t.replace(/^ryoku-hub:\s*/, "");
    }

    // "<lead> <Reason>."; a tool's own line ("systemctl: unit ... failed") is quoted as it came,
    // so the program's name keeps its case
    function sentence(lead, why) {
        var t = String(why || "");
        if (t.length === 0)
            return lead;
        var stop = /[.!?]$/.test(t) ? "" : ".";
        if (/^[a-z0-9_.+-]+: /.test(t))
            return lead + " " + I18n.tr("It said: %1").arg(t) + stop;
        return lead + " " + t.charAt(0).toUpperCase() + t.slice(1) + stop;
    }

    // a write that hangs ends after 10 s with an error and Retry, never a spinner forever
    function send(body, callback) {
        Changes.run(["ryoku-hub", "dictation", "set", JSON.stringify(body)], function (ok, out, err) {
            callback(ok, ok ? "" : store.reason(err));
        }, { timeout: 10000 });
    }

    function reload() {
        getProc.running = true;
    }

    function apply(out) {
        var d = null;
        try {
            d = JSON.parse(out);
        } catch (e) {
            store.readError = I18n.tr("Voice typing settings could not be read.");
            return;
        }
        if (!d)
            return;
        store.loaded = true;
        store.readError = "";
        store.installed = d.engine === true;
        store.packageAvailable = d.packageAvailable !== false;
        store.packageReason = d.packageReason || "";
        store.enabled = d.enabled === true;
        store.device = d.device || "auto";
        store.mode = d.mode || "paste";
        store.source = d.source || "";
        store.language = d.language || "";
        store.silenceMs = d.silenceMs === undefined ? 1500 : d.silenceMs;
        store.maxDurationSecs = d.maxDurationSecs === undefined ? 120 : d.maxDurationSecs;
        store.typeDelayMs = d.typeDelayMs === undefined ? 2 : d.typeDelayMs;
        store.model = d.model || "";
        store.models = d.models || [];
        store.sources = d.sources || [];
        store.gpu = d.gpu || "none";
        store.backends = d.backends || "cpu";
        store.activeBackend = d.activeBackend || "";

        store.state = d.state || "stopped";
        Changes.touch();
    }

    // A QtObject has no default property, so each process hangs off its own
    // property; didStart separates "the tool said nothing" from "the tool never
    // ran", which is the difference between an empty page and a broken one.
    property Process getter: Process {
        id: getProc
        property bool didStart: false
        command: ["ryoku-hub", "dictation", "get"]
        onStarted: getProc.didStart = true
        stdout: StdioCollector { onStreamFinished: store.apply(this.text) }
        stderr: StdioCollector {
            onStreamFinished: {
                if (!getProc.didStart)
                    return;
                var why = store.reason(this.text);
                if (why.length > 0)
                    store.readError = store.sentence(I18n.tr("Voice typing settings could not be read."), why);
            }
        }
    }

    // ── models ───────────────────────────────────────────────────────────────
    function download(id) {
        if (store.downloading !== "")
            return;
        store.modelError = "";
        store.failedDownload = "";
        store.failedRemove = "";
        store.notice = "";
        store.percent = -1;
        store.cancelRequested = false;
        store.downloading = id;
        downloadProc.command = ["ryoku-hub", "dictation", "install", id];
        downloadProc.running = true;
    }

    function cancelDownload() {
        if (store.downloading === "" || store.cancelRequested)
            return;
        store.cancelRequested = true;
        downloadProc.signal(15);
    }

    property Process downloader: Process {
        id: downloadProc
        property bool didStart: false
        onStarted: downloadProc.didStart = true
        stdout: SplitParser {
            onRead: line => {
                var t = String(line).trim();
                if (t.indexOf("progress ") === 0)
                    store.percent = Number(t.substring(9));
                else if (t === "installed")
                    store.notice = I18n.tr("Model installed and selected.");
            }
        }
        stderr: StdioCollector {
            onStreamFinished: {
                var why = store.reason(this.text);
                if (why.length > 0) {
                    store.modelError = store.sentence(I18n.tr("The model could not be installed."), why);
                    store.failedDownload = store.downloading;
                }
            }
        }
        onExited: code => {
            if (code !== 0 && store.modelError === "" && !store.cancelRequested)
                store.modelError = store.sentence(I18n.tr("The model could not be installed."),
                                                  downloadProc.didStart ? "" : I18n.tr("the installer did not start"));
            store.downloading = "";
            store.percent = -1;
            store.cancelRequested = false;
            store.reload();
        }
    }

    function removeModel(id) {
        if (store.downloading !== "" || store.removing !== "")
            return;
        store.modelError = "";
        store.failedDownload = "";
        store.failedRemove = "";
        store.notice = "";
        store.removing = id;
        Changes.run(["ryoku-hub", "dictation", "remove", id], function (ok, out, err) {
            store.removing = "";
            if (!ok) {
                store.failedRemove = id;
                store.modelError = store.sentence(I18n.tr("The model could not be removed."), store.reason(err));
            }
            store.reload();
        }, { timeout: 10000 });
    }

    // ── the engine package ───────────────────────────────────────────────────
    // Installing and removing it asks for a password in a terminal, like every
    // other privileged Ryoku package operation.
    function installEngine() {
        if (store.packageAvailable && !store.installed)
            Spawn.runApp(["kitty", "--hold", "-e", "ryoku-host", "pkg", "install", "ryoku-dictation"], { label: "kitty" });
    }

    function removeEngine() {
        if (!store.packageAvailable || !store.installed)
            return;
        Spawn.runApp(["kitty", "--hold", "-e", "sh", "-c",
            "ryoku-host svc --user stop ryoku-dictate 2>/dev/null || true; "
            + "ryoku-host pkg remove ryoku-dictation"], { label: "kitty" });
    }

    // the frame hands out stores by name; registering is what makes
    // Changes.stores["dictation"] exist for the page and the row bindings
    Component.onCompleted: {
        Changes.registerStore("dictation", store);
        store.reload();
    }
}
