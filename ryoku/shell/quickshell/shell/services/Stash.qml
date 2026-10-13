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

    // ── Format probe ────────────────────────────────────────────────────
    // What a link can be fetched as, read with yt-dlp while the field is
    // being typed. The mode travels with the probe because the three modes
    // ask different questions: a video wants a resolution ladder, a track
    // wants a bitrate, and Mute wants the ladder with the sound dropped.
    // The engine is irrelevant here on purpose: a format list is a yt-dlp
    // fact, and asking cobalt what it can do would make the picker change
    // with a switch the user did not think about.
    property string probeState: "idle"   // idle | probing | ready | error
    // All three mode lists from one extraction: the script answers once with
    // tiers.auto/audio/mute, so switching chips re-renders from this instead of
    // paying for a second yt-dlp call. probeTiers is the current chip's view.
    property var probeTiersAll: ({})
    readonly property var probeTiers: root.probeTiersAll[root.dlMode] || []
    property string probeTitle: ""
    property string probeMsg: ""
    property string probedUrl: ""
    // Counts probes this code killed by hand. Their onExited lands after the
    // replacement probe has already set state back to "probing", so without
    // this a supersede would read as a failure, flip to "error", and the fresh
    // probe's TIERS line would then be dropped by the state guard.
    property int probeKills: 0

    // Short enough to skip a half-typed scheme, long enough not to probe
    // "https://" alone.
    function probeAccepts(url) {
        var u = ("" + url).trim();
        return u.length >= 8 && /^https?:\/\//i.test(u);
    }

    function probe(url) {
        var u = ("" + url).trim();
        if (!root.probeAccepts(u)) {
            root.resetProbe();
            return;
        }
        root.probedUrl = u;
        root.probeTiersAll = ({});
        root.probeTitle = "";
        root.probeMsg = "";
        root.probeState = "probing";
        // Re-running the same Process is what supersedes an in-flight probe:
        // a slow answer for a link the user has already replaced must not
        // arrive after the fresh one and repaint the row.
        probeProc.command = ["bash", root.cobaltScript, "probe", u];
        if (probeProc.running) {
            root.probeKills++;
            probeProc.running = false;
        }
        probeProc.running = true;
    }

    function resetProbe() {
        if (probeProc.running) {
            root.probeKills++;
            probeProc.running = false;
        }
        root.probeState = "idle";
        root.probeTiersAll = ({});
        root.probeTitle = "";
        root.probeMsg = "";
        root.probedUrl = "";
    }

    function onProbeLine(line) {
        // A line that arrives after the state moved on belongs to a probe that
        // has been superseded or cleared, and is dropped rather than shown.
        if (root.probeState !== "probing")
            return;
        var t = ("" + line).split("\t");
        if (t[0] === "TIERS") {
            var parsed = null;
            try {
                parsed = JSON.parse(t[1] || "{}");
            } catch (err) {
                parsed = null;
            }
            // One extraction answers every mode; sanitize each list so a
            // malformed rung never reaches the picker. ProbeTiers is the
            // current chip's view of this, so a mode switch needs no probe.
            var all = ({});
            var count = 0;
            var modes = ["auto", "audio", "mute"];
            if (parsed && parsed.tiers) {
                root.probeTitle = parsed.title || "";
                for (var m = 0; m < modes.length; m++) {
                    const list = parsed.tiers[modes[m]] || [];
                    var tiers = [];
                    for (var i = 0; i < list.length; i++) {
                        const e = list[i];
                        if (e && e.fmt)
                            tiers.push({ key: e.key || "",
                                         height: (e.height === undefined ? null : e.height),
                                         bitrate: (e.bitrate === undefined ? null : e.bitrate),
                                         sizeMB: (e.sizeMB === undefined ? null : e.sizeMB),
                                         fmt: e.fmt });
                    }
                    all[modes[m]] = tiers;
                    count += tiers.length;
                }
            }
            root.probeTiersAll = all;
            root.probeState = count > 0 ? "ready" : "error";
        } else if (t[0] === "ERROR") {
            root.probeMsg = t[1] || "";
            root.probeState = "error";
        }
    }

    // ── Cobalt download + remux ─────────────────────────────────────────
    // `tier` is the probe entry the user tapped. Its selector travels with the
    // queue row so a retry asks for the same quality rather than the default.
    function enqueueDownload(url, mode, tier) {
        var u = ("" + url).trim();
        if (u.length === 0)
            return;
        // Seed the row with the title the probe already read, so it names the
        // video from the first frame instead of a generic "link" until the
        // worker's own START/SAVED line lands.
        var seeded = (root.probeTitle.length > 0 && root.probedUrl === u)
            ? root.probeTitle : I18n.tr("link");
        queueModel.append({ kind: "download", arg: u, mode: mode || root.dlMode,
            name: seeded, state: "queued", pct: 0, msg: "", saved: false,
            fmt: (tier && tier.fmt) ? tier.fmt : "", key: (tier && tier.key) ? tier.key : "" });
        pumpQueue();
    }

    function enqueueRemux(file) {
        queueModel.append({ kind: "remux", arg: file, mode: "", fmt: "", key: "",
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
                // An empty selector is passed as an empty argument rather than
                // dropped, so the script's own argument count stays fixed.
                workerProc.command = e.kind === "remux"
                    ? ["env", envp, "bash", root.cobaltScript, "remux", e.arg]
                    : ["env", envp, "bash", root.cobaltScript, "download", e.arg, e.mode, e.fmt || ""];
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

    // Cancel the row the user tapped: a running worker is killed (the script's
    // TERM trap takes its yt-dlp down with it), a queued one is only marked so
    // pumpQueue skips it. The state flips before the kill so onExited reads
    // "cancelled", not "running", and never paints it as a failure.
    function cancelJob(i) {
        if (i < 0 || i >= queueModel.count)
            return;
        var st = queueModel.get(i).state;
        if (st !== "running" && st !== "queued")
            return;
        queueModel.setProperty(i, "state", "cancelled");
        // onExited clears activeJob and pumps the next queued row.
        if (i === root.activeJob && workerProc.running)
            workerProc.running = false;
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
            if (s === "done" || s === "error" || s === "cancelled")
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
        id: probeProc
        stdout: SplitParser {
            splitMarker: "\n"
            onRead: (line) => root.onProbeLine(line)
        }
        onExited: {
            // A probe this code killed by hand is a supersede, not a failure:
            // its exit lands after the replacement already set "probing" again.
            if (root.probeKills > 0) {
                root.probeKills--;
                return;
            }
            // The helper prints its own ERROR line, so silence on a failing
            // exit is the only path here: a link it could not read at all
            // still has to leave the picker in a state the user can act on.
            if (root.probeState === "probing") {
                root.probeMsg = "";
                root.probeState = "error";
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
