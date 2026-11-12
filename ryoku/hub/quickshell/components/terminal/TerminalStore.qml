import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../../Singletons"

// the login shell, the prompt and the greeting as one Changes store, so undo
// works from any page; every value is read back from ryoku-hub after a write
QtObject {
    id: store

    property var shell: ({ current: "", choices: [], zshPrompt: "" })
    property bool shellLoaded: false
    property string shellReadError: ""
    property var starship: ({ layout: "", palette: "" })
    property var layouts: []
    property bool starshipLoaded: false
    property string starshipReadError: ""

    // the greeting: what disk holds and what is on screen
    property var ffCommitted: null
    property var ffModel: null
    property bool ffLoaded: false
    property string ffReadError: ""
    property var previewLines: []
    property var styles: []
    property var installedArt: []

    // one privileged change at a time; the page shows Working and Cancel from these
    property bool shellBusy: false
    property real shellBusySince: 0
    property var shellRun: null
    property string shellWant: ""
    property bool shellCancelled: false
    // a cancelled change settles once the re-read shows whether it still took
    property var shellCheck: null

    property var starshipDone: null
    property var ffDone: []
    property var ffSaving: null

    readonly property string configPath: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/fastfetch/config.jsonc"

    function clone(v) {
        return v === undefined || v === null ? v : JSON.parse(JSON.stringify(v));
    }

    function hexOf(triple) {
        var p = String(triple || "226;52;42").split(";");
        function h(n) { return ("0" + Math.max(0, Math.min(255, parseInt(n) || 0)).toString(16)).slice(-2); }
        return "#" + h(p[0]) + h(p[1]) + h(p[2]);
    }

    function tripleOf(hex) {
        var c = String(hex || "").replace(/^#/, "");
        if (c.length === 3)
            c = c.charAt(0) + c.charAt(0) + c.charAt(1) + c.charAt(1) + c.charAt(2) + c.charAt(2);
        return parseInt(c.substr(0, 2), 16) + ";" + parseInt(c.substr(2, 2), 16) + ";" + parseInt(c.substr(4, 2), 16);
    }

    function cleanError(text, fallback) {
        var t = String(text || "").trim().split("\n").filter(l => l.trim().length > 0).pop() || "";
        t = t.replace(/^ryoku-hub:\s*/, "").replace(/^ryostore:\s*/, "");
        // a program's name keeps its case ("ryoku-hub is missing from this Ryoku install")
        var named = /^[a-z0-9_.-]+ (is not installed|is missing from|did not finish in time)/.test(t);
        var lead = named ? t : t.charAt(0).toUpperCase() + t.slice(1);
        return t.length > 0 ? fallback + " " + lead + (/[.!?]$/.test(t) ? "" : ".") : fallback;
    }

    function read(key) {
        switch (key) {
        case "shell": return store.shellLoaded ? store.shell.current : undefined;
        case "zshPrompt": return store.shellLoaded ? store.shell.zshPrompt : undefined;
        case "starshipLayout": return store.starshipLoaded ? store.starship.layout : undefined;
        case "starshipPalette": return store.starshipLoaded ? store.starship.palette : undefined;
        }
        if (key.indexOf("ff.") !== 0 || !store.ffModel)
            return undefined;
        var path = key.slice(3);
        if (path === "textArt")
            return store.ffModel.logo.kind === "ascii" ? (store.ffModel.logo.source || "") : "";
        if (path === "accent")
            return store.hexOf(store.ffModel.accent);
        var cur = store.ffModel;
        var parts = path.split(".");
        for (var i = 0; i < parts.length && cur !== undefined && cur !== null; i++)
            cur = cur[parts[i]];
        return cur;
    }

    function write(key, v, done) {
        switch (key) {
        case "shell":
            store.setShell(String(v), done);
            return;
        case "zshPrompt":
            store.runShellPrompt(String(v), done);
            return;
        case "starshipLayout":
            store.runStarship(["set", String(v)], done);
            return;
        case "starshipPalette":
            store.runStarship(["palette", String(v)], done);
            return;
        }
        if (key.indexOf("ff.") !== 0 || !store.ffModel) {
            done(false, I18n.tr("The greeting has not loaded yet. Try again in a moment."));
            return;
        }
        if (key === "ff.logo.dither") {
            store.dither(v === true, done);
            return;
        }
        if (key === "ff.logo.kind") {
            store.setKind(String(v), done);
            return;
        }
        var m = store.clone(store.ffModel);
        var path = key.slice(3);
        if (path === "accent") {
            m.accent = store.tripleOf(v);
        } else {
            var parts = path.split("."), cur = m;
            for (var i = 0; i < parts.length - 1; i++) {
                if (typeof cur[parts[i]] !== "object" || cur[parts[i]] === null)
                    cur[parts[i]] = {};
                cur = cur[parts[i]];
            }
            cur[parts[parts.length - 1]] = store.clone(v);
        }
        store.setModel(m);
        store.queueSave(done);
    }

    // a picture and a text art file are different files: each kind keeps its own while you
    // switch, so text art never draws a PNG's bytes and Undo brings the picture back
    property var kindSources: ({})

    function fitsKind(kind, source) {
        var image = /\.(png|jpe?g|webp|svg|gif|bmp)$/i.test(String(source || ""));
        return kind === "image" ? image : kind === "ascii" ? !image : true;
    }

    function setKind(kind, done) {
        var m = store.clone(store.ffModel);
        var was = m.logo.kind;
        var saved = Object.assign({}, store.kindSources);
        if (was === "image" || was === "ascii")
            saved[was] = m.logo.source || "";
        store.kindSources = saved;
        if (kind === "image" || kind === "ascii") {
            var keep = saved.hasOwnProperty(kind) ? saved[kind] : (m.logo.source || "");
            m.logo.source = store.fitsKind(kind, keep) ? keep : "";
        }
        m.logo.kind = kind;
        store.setModel(m);
        store.queueSave(done);
    }

    // login shell

    function setShell(key, done) {
        if (key === store.shell.current) {
            done(true, "");
            return;
        }
        if (store.shellBusy) {
            done(false, I18n.tr("The login shell is still changing. Try again when it finishes."));
            return;
        }
        store.shellWant = key;
        store.shellCancelled = false;
        store.shellBusy = true;
        store.shellBusySince = Date.now();
        store.shellRun = Changes.run(["ryoku-hub", "shell", "set", key], function (ok, out, err) {
            store.shellRun = null;
            store.shellBusy = false;
            if (store.shellCancelled)
                store.shellCheck = { done: done, want: store.shellWant };
            else
                done(ok, ok ? "" : store.shellError(err));
            store.shellCancelled = false;
            shellGet.running = true;
            if (ok)
                Changes.reloadDesktop();
        });
        Changes.touch();
    }

    // pkexec reports a closed or refused password prompt as "Request dismissed" or "Not
    // authorized" and exits 126 or 127, which the backend passes on as an exit status
    function shellError(err) {
        if (/Request dismissed|Not authorized|exit status 12[67]/i.test(String(err || "")))
            return I18n.tr("The login shell did not change: the password prompt was closed or refused. Press Retry and enter your password.");
        return store.cleanError(err, I18n.tr("The login shell did not change."));
    }

    function runShellPrompt(mode, done) {
        if (mode === store.shell.zshPrompt) {
            done(true, "");
            return;
        }
        Changes.run(["ryoku-hub", "shell", "prompt", mode], function (ok, out, err) {
            done(ok, ok ? "" : store.cleanError(err, I18n.tr("The Zsh prompt did not change.")));
            shellGet.running = true;
        }, { timeout: store.deadline });
    }

    function cancelShell() {
        if (!store.shellBusy)
            return;
        store.shellCancelled = true;
        if (store.shellRun)
            store.shellRun.running = false;
    }

    // prompt

    function runStarship(args, done) {
        Changes.run(["ryoku-hub", "starship"].concat(args), function (ok, out, err) {
            done(ok, ok ? "" : store.cleanError(err, I18n.tr("The prompt did not change.")));
            starshipGet.running = true;
        }, { timeout: store.deadline });
    }

    // greeting

    function setModel(m) {
        store.ffModel = m;
        Changes.touch();
        store.queuePreview();
    }

    function queueSave(done) {
        if (done)
            store.ffDone = store.ffDone.concat([done]);
        saveDelay.restart();
    }

    function saveNow() {
        if (store.ffSaving) {
            saveDelay.restart();
            return;
        }
        if (store.ffDone.length === 0)
            return;
        store.ffSaving = { model: store.clone(store.ffModel), dones: store.ffDone };
        store.ffDone = [];
        Changes.run(["ryoku-hub", "fastfetch", "save", JSON.stringify(store.ffSaving.model)], function (ok, out, err) {
            store.saved(ok, err);
        }, { timeout: store.deadline });
    }

    function saved(ok, err) {
        var batch = store.ffSaving;
        store.ffSaving = null;
        if (!batch)
            return;
        if (ok) {
            store.ffCommitted = batch.model;
        } else if (store.ffDone.length === 0) {
            store.setModel(store.clone(store.ffCommitted));
        }
        var message = ok ? "" : store.cleanError(err, I18n.tr("The greeting did not save."));
        batch.dones.forEach(d => d(ok, message));
        Changes.touch();
        if (store.ffDone.length > 0)
            saveDelay.restart();
    }

    function dither(on, done) {
        var src = store.ffModel.logo.source || "";
        if (src.length === 0) {
            done(false, I18n.tr("Choose a picture first."));
            return;
        }
        Changes.run(["ryoku-hub", "fastfetch", "dither-logo", on ? "on" : "off", src], function (ok, out, err) {
            var path = out.trim();
            if (!ok || path.length === 0) {
                done(false, store.cleanError(err, I18n.tr("The picture did not change.")));
                return;
            }
            var m = store.clone(store.ffModel);
            m.logo.source = path;
            m.logo.dither = on;
            store.setModel(m);
            store.queueSave(done);
        }, { timeout: store.deadline });
    }

    function reloadGreeting() {
        ffGet.running = true;
        stylesGet.running = true;
        artScan.running = true;
    }

    function queuePreview() {
        if (previewThrottle.running) {
            previewThrottle.again = true;
            return;
        }
        store.previewNow();
        previewThrottle.restart();
    }

    function previewNow() {
        if (!store.ffModel)
            return;
        previewProc.command = ["ryoku-hub", "fastfetch", "preview", JSON.stringify(store.ffModel)];
        previewProc.running = true;
    }

    // truecolour SGR into lines of { text, color, bold } runs; colour "" is the plate's ink
    function parseAnsi(s) {
        var lines = [];
        var raw = String(s || "").replace(/\n+$/, "").split("\n");
        for (var li = 0; li < raw.length; li++) {
            var line = raw[li], runs = [], color = "", bold = false, buf = "", i = 0;
            while (i < line.length) {
                if (line.charCodeAt(i) === 27 && line.charAt(i + 1) === "[") {
                    if (buf.length) {
                        runs.push({ text: buf, color: color, bold: bold });
                        buf = "";
                    }
                    var j = line.indexOf("m", i);
                    if (j < 0)
                        break;
                    var codes = line.substring(i + 2, j).split(";");
                    for (var k = 0; k < codes.length; k++) {
                        if (codes[k] === "0" || codes[k] === "") {
                            color = "";
                            bold = false;
                        } else if (codes[k] === "1") {
                            bold = true;
                        } else if (codes[k] === "38" && codes[k + 1] === "2") {
                            color = Qt.rgba((parseInt(codes[k + 2]) || 0) / 255, (parseInt(codes[k + 3]) || 0) / 255,
                                            (parseInt(codes[k + 4]) || 0) / 255, 1).toString();
                            k += 4;
                        }
                    }
                    i = j + 1;
                } else {
                    buf += line.charAt(i);
                    i++;
                }
            }
            if (buf.length)
                runs.push({ text: buf, color: color, bold: bold });
            lines.push(runs);
        }
        return lines;
    }

    // one-shot commands the page runs (imports, styles, removal, reset); like every write here
    // but the password-gated shell change, a command that hangs ends as a failure after 10 s
    readonly property int deadline: 10000

    function run(argv, done) {
        Changes.run(argv, done, { timeout: store.deadline });
    }

    property Process shellGet: Process {
        command: ["ryoku-hub", "shell", "get"]
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var s = JSON.parse(this.text);
                    store.shell = { current: s.current || "", choices: s.choices || [], zshPrompt: s.zshPrompt || "" };
                    store.shellLoaded = true;
                    store.shellReadError = "";
                } catch (e) {
                    store.shellReadError = I18n.tr("The login shell could not be read.");
                }
                var check = store.shellCheck;
                store.shellCheck = null;
                if (check) {
                    var took = store.shell.current === check.want;
                    check.done(took, took ? "" : I18n.tr("Stopped before it finished, so the login shell did not change."));
                }
                Changes.touch();
            }
        }
    }

    property Process starshipGet: Process {
        command: ["ryoku-hub", "starship", "get"]
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var s = JSON.parse(this.text);
                    store.starship = { layout: s.layout || "", palette: s.palette || "" };
                    store.starshipLoaded = true;
                    store.starshipReadError = "";
                } catch (e) {
                    store.starshipReadError = I18n.tr("The prompt settings could not be read.");
                }
                Changes.touch();
            }
        }
    }

    property Process starshipList: Process {
        command: ["ryoku-hub", "starship", "list"]
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    store.layouts = (JSON.parse(this.text) || []).map(l => l.id);
                } catch (e) {
                    store.layouts = [];
                }
            }
        }
    }

    property Process ffGet: Process {
        command: ["ryoku-hub", "fastfetch", "get"]
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var m = JSON.parse(this.text);
                    if (!m.rows)
                        m.rows = [];
                    store.ffCommitted = store.clone(m);
                    store.ffLoaded = true;
                    store.ffReadError = "";
                    store.setModel(m);
                } catch (e) {
                    store.ffReadError = I18n.tr("The greeting could not be read. Its file may hold a mistake: open it to check.");
                    Changes.touch();
                }
            }
        }
    }

    property Process previewProc: Process {
        stdout: StdioCollector {
            onStreamFinished: store.previewLines = store.parseAnsi(this.text)
        }
    }

    property Process stylesGet: Process {
        command: ["ryostore", "catalog", "--category", "fastfetch"]
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    store.styles = (JSON.parse(this.text).items || []).filter(i => i.installed);
                } catch (e) {
                    store.styles = [];
                }
            }
        }
    }

    function flush() {
        saveDelay.stop();
        store.saveNow();
    }

    // pictures the store installed: one flat PNG each in <Pictures>/ryoemblems
    property Process artScan: Process {
        command: ["sh", "-c", "ls -1 \"$1\" 2>/dev/null | grep -i '\\.png$' || true", "sh", Paths.picturesDir + "/ryoemblems"]
        stdout: StdioCollector {
            onStreamFinished: {
                store.installedArt = String(this.text || "").split("\n").map(l => l.trim()).filter(l => l.length > 0);
            }
        }
    }

    property Timer saveDelay: Timer {
        interval: 400
        onTriggered: store.saveNow()
    }

    property Timer previewThrottle: Timer {
        property bool again: false
        interval: 90
        onTriggered: {
            if (previewThrottle.again) {
                previewThrottle.again = false;
                store.previewNow();
                previewThrottle.restart();
            }
        }
    }

    // the window closing waits for the greeting to reach disk
    readonly property bool unsettled: store.ffDone.length > 0 || store.ffSaving !== null

    Component.onCompleted: {
        Changes.registerStore("terminal", store);
        shellGet.running = true;
        starshipGet.running = true;
        starshipList.running = true;
        store.reloadGreeting();
    }
}
