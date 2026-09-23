pragma Singleton
import QtQuick
import Qt.labs.folderlistmodel
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons

/**
 * ~/Downloads/Stash: the download landing plus the compress/install backends.
 * A live FolderListModel tracks the folder (created on first load); cobalt
 * drives a one-at-a-time download queue through stash-cobalt.sh. compressPick /
 * installPick open a multi-file picker and run the helper on the selection.
 */
Singleton {
    id: root

    readonly property string home: Quickshell.env("HOME") || ""
    // The Downloads root is the localized XDG dir the session exports (the
    // ryoku user-environment generator does); ~/Downloads only when it is unset,
    // so a Spanish desktop lands in ~/Descargas/Stash instead of the generator
    // recreating ~/Downloads on every shell start (#246).
    readonly property string dir: (Quickshell.env("XDG_DOWNLOAD_DIR") || (home + "/Downloads")) + "/Stash"
    // On PATH, installed with the shell: these are shell helpers, not compositor
    // config, so they must resolve without a compositor config tree.
    readonly property string cobaltScript: "stash-cobalt.sh"
    readonly property string serverScript: "stash-cobalt-server.sh"

    readonly property alias files: files
    readonly property int count: files.count
    readonly property alias queueModel: queueModel

    // Newest-first, capped: the Tools "recently downloaded" list. Sorted here in
    // JS because FolderListModel's own time sort is unreliable across builds.
    readonly property var recentFiles: {
        const arr = [];
        const n = files.count;
        for (let i = 0; i < n; i++)
            arr.push({ name: files.get(i, "fileName"),
                       path: files.get(i, "filePath"),
                       t: files.get(i, "fileModified") });
        arr.sort((a, b) => b.t - a.t);
        return arr.slice(0, 6);
    }

    // Cobalt download queue.
    property string dlMode: "auto"        // auto | audio | mute
    property int activeJob: -1            // index of the running queue entry, -1 idle
    property var supportedSites: []       // cobalt's supported services (for the Tools bubble)

    // ── Cobalt engine (Docker) ──────────────────────────────────────────
    // Off = yt-dlp only (the fallback). On = a local cobalt container drives
    // downloads, with yt-dlp still catching whatever cobalt declines. cobalt
    // ships only as a Docker image, so the switch manages a container via
    // stash-cobalt-server.sh; dockerState gates whether it can turn on at all.
    property string dockerState: "unknown"   // unknown | missing | denied | setup | ready
    property string cobaltState: "off"       // off | starting | running | error
    property string cobaltMsg: ""
    property alias cobaltEnabled: engineAdapter.cobaltEnabled
    // Passed to the download worker as COBALT_API_URL; empty means yt-dlp-only.
    // No trailing slash: stash-cobalt.sh appends one ("$COBALT/"), and a double
    // slash 404s cobalt's POST endpoint.
    readonly property string cobaltUrl: (cobaltEnabled && cobaltState === "running")
        ? "http://localhost:9000" : ""

    // ── Cobalt first-run setup ──────────────────────────────────────────
    // The switch used to dead-end on "Install Docker to use cobalt", naming two
    // chores and doing neither. The wizard drives ryoku-docker instead: start
    // the service, pull the image, start cobalt, each step reporting for itself.
    // No reboot or "add yourself to a group" step exists: the helper escalates
    // through polkit and does the docker work as root, so the user's session is
    // never granted docker access of its own (that would be passwordless root).
    //
    // Every step is convergent, which is what makes a single Retry honest: it
    // re-runs the whole flow and the finished steps no-op.
    property string setupState: "idle"       // idle | running | done | failed
    property int setupStep: -1               // index of the running step, -1 when none
    property alias setupSteps: setupModel
    // True while the wizard owns the engine start, so a READY line can finish
    // the wizard instead of looking like a plain toggle.
    property bool setupOwnsEngine: false

    ListModel { id: setupModel }

    function setupReset() {
        setupModel.clear();
        var defs = [
            { key: "runtime", label: I18n.tr("Container runtime installed") },
            { key: "service", label: I18n.tr("Start the container service") },
            { key: "image",   label: I18n.tr("Download the cobalt image") },
            { key: "start",   label: I18n.tr("Start cobalt") }
        ];
        for (var i = 0; i < defs.length; i++)
            // stepState, not state: an Item delegate already has `state`, and a
            // role of that name shadows it under ComponentBehavior: Bound.
            setupModel.append({ key: defs[i].key, label: defs[i].label, stepState: "pending", msg: "" });
        root.setupState = "idle";
        root.setupStep = -1;
        root.setupOwnsEngine = false;
    }

    function setupMark(key, state, msg) {
        for (var i = 0; i < setupModel.count; i++) {
            if (setupModel.get(i).key !== key) continue;
            setupModel.setProperty(i, "stepState", state);
            setupModel.setProperty(i, "msg", msg || "");
            if (state === "running") root.setupStep = i;
            return;
        }
    }

    function setupFail(key, msg) {
        setupMark(key, "failed", msg);
        root.setupState = "failed";
        root.setupStep = -1;
        root.setupOwnsEngine = false;
    }

    function startSetup() {
        setupReset();
        root.setupState = "running";
        // Nothing here can install a package, so a missing runtime is the one
        // step the wizard has to hand back rather than fix.
        if (root.dockerState === "missing") {
            setupFail("runtime", I18n.tr("Docker is not installed. `ryoku update` installs it."));
            return;
        }
        setupMark("runtime", "done");
        setupMark("service", "running");
        provisionProc.running = true;
    }

    function onProvisionLine(line) {
        var t = ("" + line).split("\t");
        // The helper's STEP lines (service, socket) are progress detail under the
        // single "Start the container service" step, which is already running;
        // only OK and ERROR move the wizard on.
        if (t[0] === "OK") {
            setupMark("service", "done");
            setupMark("image", "running");
            root.setupOwnsEngine = true;
            root.setEngine(true);
        } else if (t[0] === "ERROR") {
            var k = root.setupStep >= 0 ? setupModel.get(root.setupStep).key : "service";
            setupFail(k, t[1] || I18n.tr("failed"));
        }
    }

    Process {
        id: provisionProc
        command: ["ryoku-docker", "provision"]
        stdout: SplitParser {
            splitMarker: "\n"
            onRead: (line) => root.onProvisionLine(line)
        }
        onExited: (code) => {
            // A helper that dies without a line of its own (missing binary, a
            // denied polkit prompt) would otherwise leave the wizard spinning.
            if (code !== 0 && root.setupState === "running" && !root.setupOwnsEngine)
                root.setupFail(root.setupStep >= 0 ? setupModel.get(root.setupStep).key : "service",
                    I18n.tr("The container helper could not complete setup"));
        }
    }

    function openFile(path) {
        Spawn.run(["xdg-open", path]);
    }

    function removeFile(path) {
        Quickshell.execDetached(["rm", "-f", path]);
    }

    function clearAll() {
        Quickshell.execDetached(["sh", "-c", "rm -f \"$1\"/*", "--", root.dir]);
    }

    // ── Compress / install ──────────────────────────────────────────────
    // Run the helpers on an explicit set of files chosen in the in-shell picker
    // (PanelPicker); the launcher entries deep-link to the same picker.
    function compress(paths) {
        if (!paths || paths.length === 0) return;
        Quickshell.execDetached(["stash-compress.sh"].concat(paths));
    }
    function install(paths) {
        if (!paths || paths.length === 0) return;
        Quickshell.execDetached(["stash-install.sh"].concat(paths));
    }

    // ── Cobalt download + remux ─────────────────────────────────────────
    function enqueueDownload(url, mode) {
        var u = ("" + url).trim();
        if (u.length === 0)
            return;
        queueModel.append({ kind: "download", arg: u, mode: mode || root.dlMode,
            name: I18n.tr("link"), state: "queued", pct: 0, msg: "", saved: false });
        pumpQueue();
    }

    function enqueueRemux(file) {
        queueModel.append({ kind: "remux", arg: file, mode: "",
            name: ("" + file).split("/").pop(), state: "queued", pct: 0, msg: "", saved: false });
        pumpQueue();
    }

    // One worker at a time walks the queue, so a burst of links downloads in
    // order instead of fighting over the network.
    function pumpQueue() {
        if (root.activeJob >= 0)
            return;
        for (var i = 0; i < queueModel.count; i++) {
            if (queueModel.get(i).state === "queued") {
                root.activeJob = i;
                queueModel.setProperty(i, "state", "running");
                var e = queueModel.get(i);
                var envp = "COBALT_API_URL=" + root.cobaltUrl;
                workerProc.command = e.kind === "remux"
                    ? ["env", envp, "bash", root.cobaltScript, "remux", e.arg]
                    : ["env", envp, "bash", root.cobaltScript, "download", e.arg, e.mode];
                workerProc.running = true;
                return;
            }
        }
    }

    function onWorkerLine(line) {
        if (root.activeJob < 0)
            return;
        var i = root.activeJob;
        var t = ("" + line).split("\t");
        if (t[0] === "START") {
            if (t[1]) queueModel.setProperty(i, "name", t[1]);
        } else if (t[0] === "PROGRESS") {
            queueModel.setProperty(i, "pct", parseInt(t[1]) || 0);
        } else if (t[0] === "SAVED") {
            if (t[1]) queueModel.setProperty(i, "name", t[1]);
            queueModel.setProperty(i, "saved", true);
            queueModel.setProperty(i, "state", "done");
        } else if (t[0] === "ERROR") {
            queueModel.setProperty(i, "msg", t[1] || I18n.tr("failed"));
            queueModel.setProperty(i, "state", "error");
        }
    }

    // Re-run a finished job in place: reset it to queued and let the worker pick
    // it up. Used by the retry affordance on a failed row.
    function retryJob(i) {
        if (i < 0 || i >= queueModel.count || queueModel.get(i).state === "running")
            return;
        queueModel.setProperty(i, "state", "queued");
        queueModel.setProperty(i, "pct", 0);
        queueModel.setProperty(i, "msg", "");
        queueModel.setProperty(i, "saved", false);
        pumpQueue();
    }

    // Drop a finished row from the queue list (never the running one).
    function dismissJob(i) {
        if (i < 0 || i >= queueModel.count || i === root.activeJob)
            return;
        queueModel.remove(i);
        if (root.activeJob > i)
            root.activeJob -= 1;
    }

    function clearQueueDone() {
        for (var i = queueModel.count - 1; i >= 0; i--) {
            var s = queueModel.get(i).state;
            if (s === "done" || s === "error")
                queueModel.remove(i);
        }
    }

    // ── Cobalt engine control ───────────────────────────────────────────
    function refreshDocker() {
        dockerProc.running = false;
        dockerProc.running = true;
    }

    // Flip the engine. On: bring the container up (first run pulls the image, so
    // cobaltState passes through "starting"). Off: stop it. State is persisted so
    // the switch survives a shell restart.
    function setEngine(on) {
        engineAdapter.cobaltEnabled = on;
        engineFile.writeAdapter();
        root.cobaltMsg = "";
        root.cobaltState = on ? "starting" : "off";
        serverProc.command = ["bash", root.serverScript, on ? "up" : "down"];
        serverProc.running = true;
    }

    function onServerLine(line) {
        var t = ("" + line).split("\t");
        if (t[0] === "docker") {
            root.dockerState = t[1] || "unknown";
        } else if (t[0] === "cobalt") {
            // status probe: reconcile the switch to the real container state.
            if (t[1] === "running")
                root.cobaltState = "running";
        } else if (t[0] === "STATUS") {
            root.cobaltState = "starting";
            root.cobaltMsg = t[1] || "";
            // "pulling" arrives after "starting" on purpose (the server emits it
            // from inside the pull), so this is the message that stays up for
            // the long wait rather than being overwritten by it.
            if (root.setupOwnsEngine && t[1] === "pulling")
                root.setupMark("image", "running",
                    I18n.tr("First run only. The image is a few hundred megabytes."));
        } else if (t[0] === "READY") {
            root.cobaltState = "running";
            root.cobaltMsg = "";
            sitesProc.running = true;   // refresh the live services list
            if (root.setupOwnsEngine) {
                root.setupMark("image", "done");
                root.setupMark("start", "done");
                root.setupState = "done";
                root.setupStep = -1;
                root.setupOwnsEngine = false;
            }
        } else if (t[0] === "STOPPED") {
            root.cobaltState = "off";
            root.cobaltMsg = "";
        } else if (t[0] === "ERROR") {
            root.cobaltState = "error";
            root.cobaltMsg = t[1] || I18n.tr("failed");
            if (root.setupOwnsEngine)
                root.setupFail(root.setupStep >= 0 ? setupModel.get(root.setupStep).key : "start",
                    t[1] || I18n.tr("failed"));
        }
    }

    FolderListModel {
        id: files
        folder: "file://" + root.dir
        showDirs: false
        showHidden: false
        nameFilters: ["*"]
    }

    ListModel {
        id: queueModel
    }

    Process {
        id: workerProc
        stdout: SplitParser {
            splitMarker: "\n"
            onRead: (line) => root.onWorkerLine(line)
        }
        onExited: (code) => {
            if (root.activeJob >= 0) {
                var st = queueModel.get(root.activeJob).state;
                if (st === "running") {
                    // No SAVED marker arrived: nothing landed, even if the script
                    // exited 0 (e.g. yt-dlp skipped a same-named file). Surface a
                    // retryable error rather than a misleading "done".
                    queueModel.setProperty(root.activeJob, "state", "error");
                    if (!queueModel.get(root.activeJob).msg)
                        queueModel.setProperty(root.activeJob, "msg",
                            code === 0 ? I18n.tr("nothing downloaded") : I18n.tr("failed"));
                }
            }
            root.activeJob = -1;
            root.pumpQueue();
        }
    }

    // Cobalt's supported services, for the Tools "works with" bubble. Live from
    // the instance when it's up, else the script's built-in list.
    Process {
        id: sitesProc
        command: ["bash", root.cobaltScript, "sites"]
        stdout: StdioCollector {
            id: sitesOut
            onStreamFinished: {
                const out = ("" + sitesOut.text).trim();
                root.supportedSites = out.length > 0 ? out.split("\n").filter(l => l.length > 0) : [];
            }
        }
    }

    Process {
        id: serverProc
        stdout: SplitParser {
            splitMarker: "\n"
            onRead: (line) => root.onServerLine(line)
        }
    }

    // Docker + container probe on load. Reconciles the persisted switch: if the
    // engine was left on but the container isn't up, bring it back.
    Process {
        id: dockerProc
        command: ["bash", root.serverScript, "status"]
        stdout: SplitParser {
            splitMarker: "\n"
            onRead: (line) => root.onServerLine(line)
        }
        onExited: {
            if (root.cobaltEnabled && root.dockerState === "ready" && root.cobaltState !== "running")
                root.setEngine(true);
        }
    }

    FileView {
        id: engineFile
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/stash.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        atomicWrites: true
        onFileChanged: reload()
        JsonAdapter {
            id: engineAdapter
            property bool cobaltEnabled: false
        }
    }

    Component.onCompleted: {
        Quickshell.execDetached(["mkdir", "-p", root.dir]);
        if (!engineFile.text())
            engineFile.writeAdapter();
        sitesProc.running = true;
        // Seed the wizard's step list so the modal can render before any run.
        root.setupReset();
        dockerProc.running = true;
    }
}
