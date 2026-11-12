import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../../Singletons"

// voice typing as a Changes store: `ryoku-hub voxtype` owns Voxtype's config
// and its user service; the store also runs model downloads, so one keeps going
// while you look at another page
QtObject {
    id: store

    property bool loaded: false
    property bool installed: false
    property bool packageAvailable: false
    property string packageReason: ""
    property var presets: []
    property string selected: "whisper-fast"
    property bool enabled: false
    property bool keySet: false

    property string downloading: ""
    property real percent: -1
    property string stage: ""
    property bool cancelRequested: false
    property string modelError: ""
    property string failedDownload: ""
    property string failedRemove: ""
    // why the state could not be read; the page shows it with Retry until a read succeeds
    property string readError: ""
    property string notice: ""
    property bool keySaving: false
    property string keyError: ""
    property string removing: ""

    readonly property var current: store.presetFor(store.selected)

    function presetFor(key) {
        for (var i = 0; i < store.presets.length; i++)
            if (store.presets[i].key === key)
                return store.presets[i];
        return null;
    }

    function usableWith(key) {
        var p = store.presetFor(key);
        return p !== null && (p.cloud ? store.keySet : p.present === true);
    }

    // "preset@<model>" is one model row's way to choose the model: the same value as "preset"
    function read(key) {
        if (key.indexOf("preset@") === 0)
            key = "preset";
        switch (key) {
        case "loaded": return store.loaded;
        case "installed": return store.loaded ? store.installed : undefined;
        case "enabled": return store.loaded ? store.enabled : undefined;
        case "preset": return store.loaded ? store.selected : undefined;
        case "presetLabel": return store.current ? store.current.label : undefined;
        }
        return undefined;
    }

    // enabled and preset go out together, as one snapshot
    function write(key, v, done) {
        if (key.indexOf("preset@") === 0)
            key = "preset";
        if (key !== "enabled" && key !== "preset") {
            done(false, "");
            return;
        }
        var before = { enabled: store.enabled, selected: store.selected };
        if (key === "enabled")
            store.enabled = v === true;
        else
            store.selected = String(v);
        Changes.touch();
        store.send({ preset: store.selected, enabled: store.enabled, openaiKey: "" }, function (ok, err) {
            if (!ok) {
                store.enabled = before.enabled;
                store.selected = before.selected;
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
    function send(req, callback) {
        Changes.run(["ryoku-hub", "voxtype", "set", JSON.stringify(req)], function (ok, out, err) {
            callback(ok, ok ? "" : store.reason(err));
        }, { timeout: 10000 });
    }

    // the key is write-only: it is never read back, only whether one is saved
    function saveKey(text) {
        var key = String(text || "").trim();
        if (key.length === 0 || store.keySaving)
            return;
        store.keySaving = true;
        store.keyError = "";
        store.send({ preset: store.selected, enabled: store.enabled, openaiKey: key }, function (ok, err) {
            store.keySaving = false;
            store.keyError = ok ? "" : (err || I18n.tr("voice typing did not take it"));
            store.reload();
        });
    }

    function download(key) {
        if (store.downloading !== "")
            return;
        store.modelError = "";
        store.failedDownload = "";
        store.failedRemove = "";
        store.notice = "";
        store.percent = -1;
        store.stage = "";
        store.cancelRequested = false;
        store.downloading = key;
        downloadProc.command = ["ryoku-hub", "voxtype", "download", key];
        downloadProc.didStart = false;
        downloadProc.running = true;
    }

    function cancelDownload() {
        if (store.downloading === "" || store.cancelRequested)
            return;
        store.cancelRequested = true;
        downloadProc.signal(15);
    }

    function removeModel(key) {
        if (store.downloading !== "" || store.removing !== "")
            return;
        store.modelError = "";
        store.failedDownload = "";
        store.failedRemove = "";
        store.notice = "";
        store.removing = key;
        Changes.run(["ryoku-hub", "voxtype", "rmmodel", key], function (ok, out, err) {
            store.removing = "";
            if (!ok) {
                store.failedRemove = key;
                store.modelError = store.sentence(I18n.tr("The model could not be removed."), store.reason(err));
            }
            store.reload();
        }, { timeout: 10000 });
    }

    // installing and removing the engine ask for a password in a terminal
    function installEngine() {
        if (store.packageAvailable)
            Spawn.runApp(["kitty", "--hold", "-e", "ryoku-host", "pkg", "install", "--aur", "voxtype-bin"], { label: "kitty" });
    }

    function removeEngine() {
        if (!store.packageAvailable)
            return;
        Spawn.runApp(["kitty", "--hold", "-e", "sh", "-c",
            "ryoku-host svc --user disable --now voxtype.service 2>/dev/null || true; "
            + "rm -f \"${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user/voxtype.service\"; "
            + "ryoku-host svc --user daemon-reload 2>/dev/null || true; "
            + "sudo ryoku-host pkg remove voxtype-bin"], { label: "kitty" });
    }

    function reload() {
        if (!getProc.running) {
            getProc.didStart = false;
            getProc.running = true;
        }
    }

    Component.onCompleted: {
        Changes.registerStore("voxtype", store);
        store.reload();
    }

    property Process getter: Process {
        id: getProc
        property bool didStart: false
        command: ["ryoku-hub", "voxtype", "get"]
        stdout: StdioCollector { id: getOut }
        stderr: StdioCollector { id: getErr }
        onStarted: getProc.didStart = true
        onExited: (code, status) => {
            var d = null;
            try {
                d = code === 0 ? JSON.parse(getOut.text) : null;
            } catch (e) {
                d = null;
            }
            if (!d || typeof d !== "object") {
                store.readError = store.sentence(I18n.tr("Voice typing's state could not be read."), store.reason(getErr.text));
                Changes.touch();
                return;
            }
            store.readError = "";
            store.presets = d.presets || [];
            store.selected = d.selected || "whisper-fast";
            store.enabled = d.enabled === true;
            store.installed = d.installed === true;
            store.packageAvailable = d.packageAvailable !== false;
            store.packageReason = d.packageReason || "";
            store.keySet = d.openaiKeySet === true;
            store.loaded = true;
            Changes.touch();
        }
        // a program that cannot start reports no exit, only that it stopped running
        onRunningChanged: {
            if (!getProc.running && !getProc.didStart) {
                store.readError = store.sentence(I18n.tr("Voice typing's state could not be read."), Changes.missingText("ryoku-hub"));
                Changes.touch();
            }
        }
    }

    function downloadOver(code, status, err) {
        var key = store.downloading;
        var cancelled = store.cancelRequested;
        store.downloading = "";
        store.percent = -1;
        store.stage = "";
        store.cancelRequested = false;
        var p = store.presetFor(key);
        var name = p ? I18n.tr(p.label) : key;
        if (cancelled) {
            store.notice = I18n.tr("Download stopped. Voice typing keeps the model it had.");
            store.reload();
        } else if (code === 0 && status === 0) {
            store.notice = I18n.tr("%1 is downloaded.").arg(name);
            if (store.selected === key)
                store.reload();
            else
                Changes.set({ store: "voxtype", key: "preset" }, key, "Speech model", "voice-typing");
        } else {
            store.failedDownload = key;
            store.modelError = store.sentence(I18n.tr("%1 could not be downloaded.").arg(name), store.reason(err));
            store.reload();
        }
    }

    property Process downloader: Process {
        id: downloadProc
        property bool didStart: false
        stdout: SplitParser {
            splitMarker: "\n"
            onRead: line => {
                var text = String(line);
                if (text.indexOf("stage: ") === 0) {
                    store.stage = text.slice(7).trim();
                    return;
                }
                var m = text.match(/(\d+(?:\.\d+)?)\s*%/);
                if (m)
                    store.percent = Math.max(0, Math.min(100, parseFloat(m[1])));
            }
        }
        stderr: StdioCollector { id: downloadErr }
        onStarted: {
            downloadProc.didStart = true;
            if (store.cancelRequested)
                downloadProc.signal(15);
        }
        onExited: (code, status) => store.downloadOver(code, status, downloadErr.text)
        onRunningChanged: {
            if (!downloadProc.running && !downloadProc.didStart && store.downloading !== "")
                store.downloadOver(-1, 0, Changes.missingText("ryoku-hub"));
        }
    }
}
