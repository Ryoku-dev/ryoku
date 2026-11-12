import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../../Singletons"

// one `ryoku update` run as the page sees it: `ryoku update --gui` works in the
// background and publishes its steps, questions and watchdog to a run-state
// file; this reads it, answers it, and never stays on a run that is gone
Item {
    id: run

    // idle | running | prompt | auth | done | error
    property string phase: "idle"
    property real progress: 0
    property string label: ""
    property int ownerPid: 0
    property double started: 0
    property double beat: 0
    property var steps: []
    property var pieces: []
    property string piecesStep: ""
    property bool firstBuild: false
    property var logLines: []
    property string activity: ""
    property var watch: ({})
    property string errorMsg: ""
    property string snapshot: ""
    property string logPath: ""
    property string promptId: ""
    property string promptTitle: ""
    property string promptDetail: ""
    property string promptError: ""
    property var promptOptions: []
    property bool runSnapshotsKnown: false
    property bool runSnapshotsSupported: false
    property string runSnapshotReason: ""
    readonly property bool snapshotsSupported: run.runSnapshotsKnown ? run.runSnapshotsSupported : Updates.snapshotsSupported
    readonly property string snapshotReason: run.runSnapshotsKnown ? run.runSnapshotReason : Updates.snapshotReason

    readonly property string runtimeDir: Quickshell.env("XDG_RUNTIME_DIR") || "/tmp"
    readonly property string stateDir: Quickshell.env("XDG_STATE_HOME") || (Quickshell.env("HOME") + "/.local/state")
    readonly property string statePath: run.runtimeDir + "/ryoku-update.json"
    readonly property string answerPath: run.runtimeDir + "/ryoku-update-answer"
    readonly property string defaultLogPath: run.stateDir + "/ryoku/update-log.txt"
    readonly property string reportPath: run.stateDir + "/ryoku/doctor-report.txt"
    readonly property string shownLogPath: run.logPath !== "" ? run.logPath : run.defaultLogPath

    // starting: Update now was pressed and the run has not published yet
    property bool starting: false
    property string actionError: ""
    property bool authBusy: false
    property bool keepWaiting: false
    // how the last press that moved the run came: focus handed on as the run moves shows
    // the keyboard mark only when the keys pressed (a pointer press takes focus quietly)
    property int handReason: Qt.TabFocusReason
    function pressedBy(control) {
        run.handReason = control.visualFocus ? Qt.TabFocusReason : Qt.MouseFocusReason;
    }
    property double now: Date.now()
    readonly property bool live: run.phase === "running" || run.phase === "prompt" || run.phase === "auth"
    readonly property bool settled: run.phase === "done" || run.phase === "error"
    readonly property bool shown: run.starting || run.phase !== "idle"
    readonly property bool stopped: run.phase === "error" && run.errorMsg === "stopped by request"
    // a live pid whose heartbeat stopped: the run is wedged, not working
    readonly property bool unresponsive: run.live && run.beat > 0 && run.now - run.beat > 20000
    readonly property bool quiet: run.phase === "running" && run.watch.state === "quiet"
    readonly property bool stalled: run.phase === "running" && (run.watch.state === "stalled" || run.unresponsive)
    readonly property real fillFraction: run.phase === "done" ? 1 : Math.max(0, Math.min(1, run.progress))

    // a span of milliseconds the way the console prints it
    function human(ms) {
        var s = Math.max(0, Math.floor(ms / 1000));
        if (ms < 10000)
            return I18n.tr("%1 s").arg((Math.max(0, ms) / 1000).toFixed(1));
        if (s < 60)
            return I18n.tr("%1 s").arg(s);
        if (s < 3600)
            return I18n.tr("%1 min %2 s").arg(Math.floor(s / 60)).arg(s % 60);
        return I18n.tr("%1 h %2 min").arg(Math.floor(s / 3600)).arg(Math.floor((s % 3600) / 60));
    }

    function since(ms) {
        return ms > 0 ? run.human(run.now - ms) : "";
    }

    // the clock ticks only while a run is on screen and moving
    Timer {
        interval: 1000
        running: run.live || run.starting
        repeat: true
        triggeredOnStart: true
        onTriggered: run.now = Date.now()
    }

    // a "running" document is believed only while the pid it names is a live
    // `ryoku update`; one with no owner is cleared, one whose process is gone
    // is recorded as stopped
    Timer {
        interval: 2000
        running: run.live
        repeat: true
        triggeredOnStart: true
        onTriggered: run.checkOwner()
    }

    function checkOwner() {
        if (!run.live)
            return;
        if (run.ownerPid < 2) {
            run.dismiss();
            return;
        }
        if (ownerProbe.running)
            return;
        ownerProbe.command = ["sh", "-c", "tr '\\0' ' ' < /proc/\"$1\"/cmdline 2>/dev/null", "sh", "" + run.ownerPid];
        ownerProbe.running = true;
    }

    Process {
        id: ownerProbe
        stdout: StdioCollector {
            onStreamFinished: {
                var cmd = this.text;
                if (!run.live || (/ryoku/.test(cmd) && /\bupdate\b/.test(cmd)))
                    return;
                run.settleOrphan();
            }
        }
    }

    // a vanished run is recorded as stopped in the run-state itself, keeping
    // the steps it reached, so every reader agrees whatever ryoku the box has
    property var lastDoc: ({})
    function settleOrphan() {
        var o = Object.assign({}, run.lastDoc);
        o.phase = "error";
        o.error = I18n.tr("The update was no longer running.");
        delete o.watch;
        delete o.prompt;
        o.steps = (o.steps || []).map(s => s.state === "running" ? Object.assign({}, s, { state: "failed" }) : s);
        Quickshell.execDetached(["sh", "-c", "printf '%s' \"$1\" > \"$2.tmp\" && mv \"$2.tmp\" \"$2\"", "sh", JSON.stringify(o), run.statePath]);
        run.applyState(JSON.stringify(o));
    }

    FileView {
        id: stateFile
        path: run.statePath
        watchChanges: true
        atomicWrites: false
        // no file is the idle state
        printErrors: false
        onLoaded: run.applyState(stateFile.text())
        onFileChanged: stateFile.reload()
        onLoadFailed: run.phase = "idle"
    }

    // the heartbeat rewrites the file every two seconds; lists are handed on
    // only when they changed, so delegates do not rebuild on each beat
    property string stepsKey: ""
    property string piecesKey: ""
    property string logKey: ""
    property double runStartedAt: 0

    function applyState(t) {
        var prev = run.phase;
        var o;
        try {
            o = JSON.parse(t);
        } catch (e) {
            o = { phase: "idle" };
        }
        if (!o || typeof o !== "object")
            o = { phase: "idle" };
        run.lastDoc = o;
        run.phase = o.phase || "idle";
        run.progress = typeof o.progress === "number" ? o.progress : 0;
        run.label = o.label || "";
        run.ownerPid = typeof o.pid === "number" ? Math.floor(o.pid) : 0;
        run.started = o.started || 0;
        run.beat = o.beat || 0;
        run.activity = o.activity || "";
        run.watch = o.watch || ({});
        run.errorMsg = o.error || "";
        run.snapshot = o.snapshot || "";
        run.runSnapshotsKnown = o.snapshotsSupported === true || (o.snapshotReason || "") !== "";
        if (run.runSnapshotsKnown) {
            run.runSnapshotsSupported = o.snapshotsSupported === true;
            run.runSnapshotReason = o.snapshotReason || "";
        }
        run.logPath = o.logPath || "";
        var sk = JSON.stringify(o.steps || []);
        if (sk !== run.stepsKey) {
            run.stepsKey = sk;
            run.steps = o.steps || [];
        }
        var pieces = Array.isArray(o.pieces) ? o.pieces : [];
        var pk = JSON.stringify(pieces);
        if (pk !== run.piecesKey) {
            run.piecesKey = pk;
            run.pieces = pieces;
        }
        run.piecesStep = typeof o.piecesStep === "string" ? o.piecesStep : "";
        run.firstBuild = o.firstBuild === true;
        var lk = JSON.stringify(o.log || []);
        if (lk !== run.logKey) {
            run.logKey = lk;
            run.logLines = o.log || [];
        }
        var p = o.prompt || {};
        // a fresh question: the last answer was judged
        if ((p.id || "") !== run.promptId) {
            run.promptId = p.id || "";
            run.authBusy = false;
        }
        run.promptTitle = p.title || "";
        run.promptDetail = p.detail || "";
        run.promptError = p.error || "";
        var opts = p.options || [];
        if (JSON.stringify(opts) !== JSON.stringify(run.promptOptions))
            run.promptOptions = opts;
        if (run.phase !== "auth")
            run.authBusy = false;
        if (run.phase !== "idle") {
            run.starting = false;
            startWatch.stop();
        }
        if (run.watch.state === "working")
            run.keepWaiting = false;
        run.now = Date.now();
        if (run.live && run.runStartedAt <= 0)
            run.runStartedAt = run.started > 0 ? run.started / 1000 : Date.now() / 1000;
        // settled: offer Rashin's fixer when the health check left a fresh report
        if (run.settled && prev !== run.phase) {
            rashinProbe.running = true;
            run.checkReportFresh();
        }
        // settled back to idle is finished: refresh so the list clears
        if (prev !== "idle" && run.phase === "idle") {
            run.clearFix();
            Updates.check();
        }
        run.checkOwner();
    }

    // a choice goes to the back-channel the run polls, positional so a quote in it cannot break out
    function answer(choice) {
        Quickshell.execDetached(["sh", "-c", "printf '%s' \"$1\" > \"$2\"", "sh", String(choice), run.answerPath]);
        run.phase = "running";
    }

    // the boot-menu restore steps for the snapshot taken before this run, in a terminal
    function rollback() {
        if (!run.snapshotsSupported || run.snapshot === "")
            return;
        Spawn.runApp(["kitty", "--class=dev.ryoku.update", "-e", "sh", "-c",
            "ryoku rollback \"$1\"; printf '\\n%s ' \"$2\"; read -r _", "sh", run.snapshot, I18n.tr("Press Enter to close.")], { label: "kitty" });
    }

    function clearFix() {
        run.runStartedAt = 0;
        run.reportFresh = false;
        run.fixState = "";
        run.fixMsg = "";
    }

    // a finished or failed run is cleared so the page returns to its status
    function dismiss() {
        Quickshell.execDetached(["sh", "-c", "printf '%s' '{\"phase\":\"idle\"}' > \"$1\"", "sh", run.statePath]);
        run.phase = "idle";
        run.actionError = "";
        run.clearFix();
        Updates.check();
    }

    // a command's own refusal as a sentence: no program prefix, a capital, a full stop
    function said(text, fallback) {
        var t = String(text || "").trim().split("\n").pop().replace(/^ryoku: /, "");
        if (t === "")
            return fallback;
        t = t.charAt(0).toUpperCase() + t.slice(1);
        return /[.!?]$/.test(t) ? t : t + ".";
    }

    function start(withSystem) {
        if (run.starting || run.live)
            return;
        run.starting = true;
        run.actionError = "";
        run.keepWaiting = false;
        guiProc.didStart = false;
        guiProc.command = withSystem ? ["ryoku", "update", "--gui", "--system"] : ["ryoku", "update", "--gui"];
        guiProc.running = true;
        startWatch.restart();
    }

    Process {
        id: guiProc
        property bool didStart: false
        environment: Spawn.env
        stderr: StdioCollector { id: guiErr }
        onStarted: guiProc.didStart = true
        // a program that cannot start reports no exit, only that it stopped running
        onRunningChanged: {
            if (!guiProc.running && !guiProc.didStart && run.starting) {
                run.starting = false;
                startWatch.stop();
                run.actionError = I18n.tr("The update could not start because a Ryoku tool is missing. Update Ryoku, then try again.");
            }
        }
        onExited: code => {
            if (code !== 0 && run.starting) {
                run.starting = false;
                startWatch.stop();
                run.actionError = run.said(guiErr.text, I18n.tr("The update could not start. Run ryoku update in a terminal to see why."));
            }
        }
    }

    Timer {
        id: startWatch
        interval: 15000
        onTriggered: {
            if (run.starting && run.phase === "idle") {
                run.starting = false;
                guiProc.running = false;
                run.actionError = I18n.tr("The update did not start. Run ryoku update in a terminal to see why.");
            }
        }
    }

    // the password goes to the waiting run on stdin, never on a command line
    property string pendingSecret: ""
    function submitPassword(pw) {
        if (pw.length === 0 || run.authBusy)
            return;
        run.authBusy = true;
        run.actionError = "";
        run.pendingSecret = pw;
        authProc.didStart = false;
        authProc.running = true;
        authLimit.restart();
    }
    function cancelPassword() {
        run.actionError = "";
        run.runSide(["ryoku", "update", "--auth", "--cancel"],
            I18n.tr("The update could not be told to stop asking because a Ryoku tool is missing. Press Stop instead."),
            I18n.tr("The update did not answer. Press Stop instead."));
    }

    Process {
        id: authProc
        property bool didStart: false
        command: ["ryoku", "update", "--auth"]
        stdinEnabled: true
        onStarted: {
            authProc.didStart = true;
            write(run.pendingSecret + "\n");
            run.pendingSecret = "";
        }
        // a program that cannot start reports no exit, only that it stopped running
        onRunningChanged: {
            if (!authProc.running && !authProc.didStart && run.authBusy) {
                authLimit.stop();
                run.pendingSecret = "";
                run.authBusy = false;
                run.actionError = I18n.tr("The password could not be passed on because a Ryoku tool is missing. Update Ryoku, then try again.");
            }
        }
        stderr: StdioCollector { id: authErr }
        onExited: code => {
            authLimit.stop();
            if (code !== 0 && run.authBusy) {
                run.authBusy = false;
                run.actionError = run.said(authErr.text, I18n.tr("The password did not reach the update. Try again."));
            }
        }
    }
    Timer {
        id: authLimit
        interval: 15000
        onTriggered: {
            if (!run.authBusy)
                return;
            run.authBusy = false;
            authProc.running = false;
            run.actionError = I18n.tr("The password did not reach the update in time. Try again.");
        }
    }

    // the run gives back what it paused (the shell, the sleep guard) and
    // records the stop; a package transaction finishes on its own
    function stop() {
        run.actionError = "";
        run.runSide(["ryoku", "update", "--cancel"],
            I18n.tr("The update could not be stopped because a Ryoku tool is missing. Run ryoku update --cancel in a terminal."),
            I18n.tr("The update did not stop. Run ryoku update --cancel in a terminal."));
    }

    // a short command to the waiting run: its refusal, a missing program or 10 s of silence ends in words
    function runSide(argv, missing, failed) {
        Changes.run(argv, function (ok, out, err, code, late) {
            if (ok)
                return;
            run.actionError = code === -1 && !late ? missing
                : late ? failed
                : run.said(err, failed);
        }, { timeout: 10000, env: Spawn.env });
    }

    // the update runs the health check near its end; when it found trouble in
    // this run and Rashin is on, Rashin's agent can look into it
    property bool rashinOn: false
    property bool reportFresh: false
    readonly property bool canFix: run.settled && run.rashinOn && run.reportFresh

    // reads Rashin's switch without starting it; an absent binary reads as off
    Process {
        id: rashinProbe
        command: ["/bin/sh", "-c", "exec rashin status --json"]
        stderr: StdioCollector {}
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    run.rashinOn = JSON.parse(this.text).enabled === true;
                } catch (e) {
                    run.rashinOn = false;
                }
            }
        }
    }

    // was the report rewritten during this run; with no run start known the offer stays off
    Process {
        id: reportProbe
        stdout: StdioCollector {
            onStreamFinished: run.reportFresh = this.text.trim() === "fresh"
        }
    }
    function checkReportFresh() {
        if (run.runStartedAt <= 0) {
            run.reportFresh = false;
            return;
        }
        reportProbe.command = ["sh", "-c",
            "m=$(stat -c %Y \"$1\" 2>/dev/null || echo 0); [ \"$m\" -ge \"$2\" ] && echo fresh || echo stale",
            "sh", run.reportPath, "" + Math.floor(run.runStartedAt)];
        reportProbe.running = true;
    }

    // "" | opening | opened | failed
    property string fixState: ""
    property string fixMsg: ""
    function fix() {
        if (fixProc.running)
            return;
        run.fixState = "opening";
        run.fixMsg = "";
        fixProc.running = true;
        fixLimit.restart();
    }
    Process {
        id: fixProc
        command: ["/bin/sh", "-c", "exec rashin fix doctor"]
        environment: Spawn.env
        stdout: StdioCollector { id: fixOut }
        stderr: StdioCollector { id: fixErr }
        onExited: code => {
            fixLimit.stop();
            if (run.fixState !== "opening")
                return;
            var out = fixOut.text.trim().split("\n").pop();
            // 127 is no rashin at all, 126 one the shell found but could not run
            var gone = code === 127 || (code === 126 && /bad interpreter|No such file|cannot execute/.test(fixErr.text));
            // a refusal is Rashin's own sentence, then where to go next (the plaque stays as the retry)
            var why = run.said(String(fixErr.text).trim().split("\n").pop().replace(/^rashin: /, ""), "");
            run.fixState = code === 0 ? "opened" : "failed";
            run.fixMsg = code === 0 ? (out || I18n.tr("Rashin opened in a terminal."))
                : gone ? I18n.tr("Rashin is not installed, so it could not look. Turn Rashin on in Settings, then try again.")
                : why !== "" ? why + " " + I18n.tr("Try again, or run rashin fix doctor in a terminal to see why.")
                : I18n.tr("Rashin could not start. Run rashin fix doctor in a terminal to see why.");
        }
    }
    Timer {
        id: fixLimit
        interval: 15000
        onTriggered: {
            if (run.fixState !== "opening")
                return;
            fixProc.running = false;
            run.fixState = "failed";
            run.fixMsg = I18n.tr("Rashin did not answer in time. Try again, or run rashin fix doctor in a terminal.");
        }
    }
}
