pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../Singletons"

// System > Updates (DESIGN.md section 8, SYSTEM). The Ryoku update channel as a
// paper-and-ink instrument: how far the install sits behind origin, the commits
// that would land (or the recent history it already runs), and a one-click
// update that runs right here: `ryoku update --gui` works in the background,
// asks this page for its password and its questions, and publishes its steps,
// its live output and its own watchdog to the run-state this page draws.
//
// This is a full-bleed page -- the shell hides its side panel and global action
// bar and keeps the rail -- so it draws its own head, content and action bar.
// The run itself is drawn by UpdateRun (the timeline), UpdateAuth (the
// password) and UpdateLog (the raw log behind DETAILS); this file owns the
// run-state, the watchdog that never lets the page sit on a run that is gone,
// and the actions. Ink on black, inversion for emphasis, every colour, face,
// size, radius and duration from Tokens.
Item {
    id: pg

    property var hub
    // A full-bleed page owns the whole content region itself.
    readonly property bool fullBleed: true

    // ── automatic-check schedule (persisted in the hub's TOML) ──────────────
    property string interval: "daily"

    readonly property var intervalModel: [
        { "key": "off",    "label": I18n.tr("Off") },
        { "key": "hourly", "label": I18n.tr("Hourly") },
        { "key": "daily",  "label": I18n.tr("Daily") },
        { "key": "weekly", "label": I18n.tr("Weekly") }
    ]
    readonly property var intervalLabels: [I18n.tr("Off"), I18n.tr("Hourly"), I18n.tr("Daily"), I18n.tr("Weekly")]

    function intervalLabel(k) {
        for (var i = 0; i < pg.intervalModel.length; i++)
            if (pg.intervalModel[i].key === k)
                return pg.intervalModel[i].label;
        return I18n.tr("Daily");
    }
    function intervalKey(l) {
        for (var i = 0; i < pg.intervalModel.length; i++)
            if (pg.intervalModel[i].label === l)
                return pg.intervalModel[i].key;
        return "daily";
    }
    function intervalBlurb(k) {
        switch (k) {
        case "off":    return I18n.tr("manual only");
        case "hourly": return I18n.tr("every hour");
        case "weekly": return I18n.tr("once a week");
        default:       return I18n.tr("once a day");
        }
    }
    function setInterval(k) {
        if (pg.interval === k)
            return;
        pg.interval = k;
        saveInterval.command = ["ryoku-hub", "config", "set", "update_interval", k];
        saveInterval.running = true;
    }

    Process {
        id: loadInterval
        command: ["ryoku-hub", "config", "get", "update_interval"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                var v = this.text.trim();
                if (v === "off" || v === "hourly" || v === "daily" || v === "weekly")
                    pg.interval = v;
            }
        }
    }

    Process { id: saveInterval }

    // re-check on the configured cadence.
    readonly property int intervalMs: {
        switch (pg.interval) {
        case "hourly": return 3600 * 1000;
        case "weekly": return 7 * 24 * 3600 * 1000;
        default:       return 24 * 3600 * 1000;
        }
    }

    Timer {
        interval: pg.intervalMs
        running: pg.interval !== "off"
        repeat: true
        onTriggered: Updates.check()
    }

    // ── live run state (published by `ryoku update`) ────────────────────────
    property string phase: "idle"   // idle | running | prompt | auth | done | error
    property real progress: 0
    property string label: ""
    property int ownerPid: 0
    property double started: 0
    property double beat: 0
    property var steps: []
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
    readonly property bool snapshotsSupported: pg.runSnapshotsKnown ? pg.runSnapshotsSupported : Updates.snapshotsSupported
    readonly property string snapshotReason: pg.runSnapshotsKnown ? pg.runSnapshotReason : Updates.snapshotReason
    readonly property string statePath: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-update.json"
    readonly property string answerPath: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-update-answer"
    readonly property string defaultLogPath: (Quickshell.env("XDG_STATE_HOME") || (Quickshell.env("HOME") + "/.local/state")) + "/ryoku/update-log.txt"

    // ── the Hub's side of a run ─────────────────────────────────────────────
    // starting: UPDATE NOW was pressed and the run has not published yet.
    // keepWaiting: the user saw the stall notice and chose to wait it out.
    property bool starting: false
    property string actionError: ""
    property bool authBusy: false
    property bool keepWaiting: false
    property bool stopArmed: false
    property bool showLog: false
    property double now: Date.now()
    readonly property bool live: pg.phase === "running" || pg.phase === "prompt" || pg.phase === "auth"
    // a live pid whose heartbeat stopped: the run is wedged, not working
    readonly property bool unresponsive: pg.live && pg.beat > 0 && pg.now - pg.beat > 20000

    // human renders a span of milliseconds the way the console does.
    function human(ms) {
        const s = Math.max(0, Math.floor(ms / 1000));
        if (ms < 10000)
            return (Math.max(0, ms) / 1000).toFixed(1) + "s";
        if (s < 60)
            return s + "s";
        if (s < 3600)
            return Math.floor(s / 60) + "m " + ("0" + (s % 60)).slice(-2) + "s";
        return Math.floor(s / 3600) + "h " + ("0" + Math.floor((s % 3600) / 60)).slice(-2) + "m";
    }

    // ── Rashin "Fix with AI" offer ──────────────────────────────────────────
    // `ryoku update` runs `ryoku doctor` near the end; when the health check
    // finds trouble it rewrites its report. If that happened during this run and
    // Rashin (the local agent OS) is switched on, offer to let Rashin's agent
    // investigate and fix, instead of leaving the user to read a failed check.
    property bool rashinOn: false
    property double runStartedAt: 0
    property bool reportFresh: false
    readonly property string reportPath: (Quickshell.env("XDG_STATE_HOME") || (Quickshell.env("HOME") + "/.local/state")) + "/ryoku/doctor-report.txt"
    readonly property bool showFixWithAI: (pg.phase === "done" || pg.phase === "error") && pg.rashinOn && pg.reportFresh

    // Rashin is on when its master switch reads enabled; an absent binary closes
    // stdout with empty output, which parses as off. Mirrors RashinPage.
    Process {
        id: rashinProbe
        command: ["sh", "-c", "ryoku-rashin status --json"]
        stderr: StdioCollector {}
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    pg.rashinOn = JSON.parse(this.text).enabled === true;
                } catch (e) {
                    pg.rashinOn = false;
                }
            }
        }
    }

    // Was the doctor report rewritten during this run? Compare its mtime to when
    // the run started; with no run start known (page opened after the fact) the
    // offer stays off rather than firing on a stale report.
    Process {
        id: reportProbe
        stdout: StdioCollector {
            onStreamFinished: pg.reportFresh = this.text.trim() === "fresh"
        }
    }
    function checkReportFresh() {
        if (pg.runStartedAt <= 0) {
            pg.reportFresh = false;
            return;
        }
        reportProbe.command = ["sh", "-c",
            "m=$(stat -c %Y \"$1\" 2>/dev/null || echo 0); [ \"$m\" -ge \"$2\" ] && echo fresh || echo stale",
            "sh", pg.reportPath, "" + Math.floor(pg.runStartedAt)];
        reportProbe.running = true;
    }

    // hand the fixing to Rashin: with no TTY here `ryoku-rashin fix doctor` opens
    // the agent in a terminal that investigates and repairs. detached, as always.
    function fixWithAI() {
        Spawn.run(["ryoku-rashin", "fix", "doctor"]);
    }

    // the fill fraction for the shared progress track: the run's own reading
    // (it counts the running step by what its output says, "(3/12)"), pinned
    // full once done.
    readonly property real fillFraction: pg.phase === "done" ? 1 : Math.max(0, Math.min(1, pg.progress))

    // ── the watchdog: never leave the page on a run that is not there ───────
    // A "running" document is only believed while the pid it names is a live
    // `ryoku update`. One with no pid has no owner at all (an old build wrote
    // it outside an update) and is cleared on sight; one whose process is gone
    // is recorded as stopped by the CLI. A live run that stops beating, or
    // whose own watchdog sees no progress, gets a notice with a way out.
    Timer {
        interval: 1000
        running: pg.live || pg.starting
        repeat: true
        triggeredOnStart: true
        onTriggered: pg.now = Date.now()
    }
    Timer {
        interval: 2000
        running: pg.live
        repeat: true
        triggeredOnStart: true
        onTriggered: pg.checkOwner()
    }

    function checkOwner() {
        if (!pg.live)
            return;
        if (pg.ownerPid < 2) {
            pg.dismiss();
            return;
        }
        if (ownerProbe.running)
            return;
        ownerProbe.command = ["sh", "-c", "tr '\\0' ' ' < /proc/\"$1\"/cmdline 2>/dev/null", "sh", "" + pg.ownerPid];
        ownerProbe.running = true;
    }

    Process {
        id: ownerProbe
        stdout: StdioCollector {
            onStreamFinished: {
                const cmd = this.text;
                if (!pg.live || (/ryoku/.test(cmd) && /\bupdate\b/.test(cmd)))
                    return;
                pg.settleOrphan();
            }
        }
    }

    // record a vanished run as stopped in the run-state itself, keeping the
    // steps it reached, so every reader (and this page, next time it opens)
    // agrees. Written here rather than by the CLI: the page must be able to
    // leave a dead run whatever ryoku binary the box has.
    property var lastDoc: ({})
    function settleOrphan() {
        const o = Object.assign({}, pg.lastDoc);
        o.phase = "error";
        o.error = I18n.tr("The update was no longer running.");
        delete o.watch;
        delete o.prompt;
        o.steps = (o.steps || []).map(s => s.state === "running" ? Object.assign({}, s, { "state": "failed" }) : s);
        Quickshell.execDetached(["sh", "-c", "printf '%s' \"$1\" > \"$2.tmp\" && mv \"$2.tmp\" \"$2\"", "sh", JSON.stringify(o), pg.statePath]);
        pg.applyState(JSON.stringify(o));
    }

    FileView {
        id: stateFile
        path: pg.statePath
        watchChanges: true
        atomicWrites: false
        onLoaded: pg.applyState(stateFile.text())
        onFileChanged: stateFile.reload()
        onLoadFailed: pg.phase = "idle"
    }

    // the heartbeat rewrites the file every two seconds; only hand a list on
    // when it changed, or every delegate would rebuild (and every pulse
    // restart) on each beat.
    property string stepsKey: ""
    property string logKey: ""

    function applyState(t) {
        const prev = pg.phase;
        let o;
        try {
            o = JSON.parse(t);
        } catch (e) {
            o = { "phase": "idle" };
        }
        pg.lastDoc = o;
        pg.phase = o.phase || "idle";
        pg.progress = typeof o.progress === "number" ? o.progress : 0;
        pg.label = o.label || "";
        pg.ownerPid = typeof o.pid === "number" ? Math.floor(o.pid) : 0;
        pg.started = o.started || 0;
        pg.beat = o.beat || 0;
        pg.activity = o.activity || "";
        pg.watch = o.watch || ({});
        pg.errorMsg = o.error || "";
        pg.snapshot = o.snapshot || "";
        pg.runSnapshotsKnown = o.snapshotsSupported === true || (o.snapshotReason || "") !== "";
        if (pg.runSnapshotsKnown) {
            pg.runSnapshotsSupported = o.snapshotsSupported;
            pg.runSnapshotReason = o.snapshotReason || "";
        }
        pg.logPath = o.logPath || "";
        const sk = JSON.stringify(o.steps || []);
        if (sk !== pg.stepsKey) {
            pg.stepsKey = sk;
            pg.steps = o.steps || [];
        }
        const lk = JSON.stringify(o.log || []);
        if (lk !== pg.logKey) {
            pg.logKey = lk;
            pg.logLines = o.log || [];
        }
        const p = o.prompt || {};
        if ((p.id || "") !== pg.promptId) {
            pg.promptId = p.id || "";
            pg.authBusy = false; // a fresh question: the last answer was judged
        }
        pg.promptTitle = p.title || "";
        pg.promptDetail = p.detail || "";
        pg.promptError = p.error || "";
        const opts = p.options || [];
        if (JSON.stringify(opts) !== JSON.stringify(pg.promptOptions))
            pg.promptOptions = opts;
        if (pg.phase !== "auth")
            pg.authBusy = false;
        if (pg.phase !== "idle")
            pg.starting = false;
        if (pg.watch.state === "working")
            pg.keepWaiting = false;
        if (!pg.live)
            pg.stopArmed = false;
        pg.now = Date.now();
        // note when the run began, so a settled run can tell whether the doctor
        // report is from this run or an old one.
        if (pg.live && pg.runStartedAt <= 0)
            pg.runStartedAt = pg.started > 0 ? pg.started / 1000 : Date.now() / 1000;
        // settled: offer Rashin's fixer when the health check left a fresh report.
        if ((pg.phase === "done" || pg.phase === "error") && prev !== pg.phase) {
            rashinProbe.running = true;
            pg.checkReportFresh();
        }
        // settled back to idle = finished. refresh so the list clears.
        if (prev !== "idle" && pg.phase === "idle") {
            pg.runStartedAt = 0;
            pg.reportFresh = false;
            pg.showLog = false;
            Updates.check();
        }
        pg.checkOwner();
    }

    // answer a prompt phase: write the choice to the back-channel `ryoku update`
    // is polling (positional args, so a quote in the label can't break out),
    // then optimistically resume the running view so the buttons clear.
    function answer(choice) {
        Quickshell.execDetached(["sh", "-c", "printf '%s' \"$1\" > \"$2\"", "sh", choice, pg.answerPath]);
        pg.phase = "running";
    }

    // guide restoring the pre-update snapshot after a failed run, in a terminal
    // (`ryoku rollback` prints the boot-menu restore steps and exits, so hold
    // the window for the user to read), then clear the error state.
    function rollback() {
        if (!pg.snapshotsSupported || pg.snapshot === "")
            return;
        Spawn.run(["kitty", "--class=dev.ryoku.update", "-e", "sh", "-c", "ryoku rollback \"$1\"; printf '\\npress enter to close '; read -r _", "sh", pg.snapshot]);
        pg.dismiss();
    }

    // dismiss a finished/failed run: clear the run-state file so the page
    // returns to idle.
    function dismiss() {
        Quickshell.execDetached(["sh", "-c", "printf '%s' '{\"phase\":\"idle\"}' > \"$1\"", "sh", pg.statePath]);
        pg.phase = "idle";
        pg.runStartedAt = 0;
        pg.reportFresh = false;
        pg.showLog = false;
        pg.actionError = "";
        Updates.check();
    }

    // The run happens here, not in a terminal: `ryoku update --gui` starts it
    // in the background and returns, and the run reports through the
    // run-state, asking this page for its password and its questions.
    function startUpdate() {
        pg.starting = true;
        pg.actionError = "";
        pg.keepWaiting = false;
        pg.showLog = false;
        guiProc.running = true;
        startWatch.restart();
    }

    Process {
        id: guiProc
        command: ["ryoku", "update", "--gui"]
        environment: Spawn.env
        stderr: StdioCollector { id: guiErr }
        onExited: (code) => {
            if (code !== 0) {
                pg.starting = false;
                pg.actionError = guiErr.text.trim().replace(/^ryoku: /, "") || I18n.tr("The update could not start.");
            }
        }
    }
    Timer {
        id: startWatch
        interval: 15000
        onTriggered: {
            if (pg.starting && pg.phase === "idle") {
                pg.starting = false;
                pg.actionError = I18n.tr("The update did not start. Run `ryoku update` in a terminal to see why.");
            }
        }
    }

    // the password goes to the waiting run on stdin, never on a command line
    property string pendingSecret: ""
    function submitPassword(pw) {
        pg.authBusy = true;
        pg.pendingSecret = pw;
        authProc.running = true;
    }
    function cancelPassword() {
        authCancelProc.running = true;
    }
    Process {
        id: authProc
        command: ["ryoku", "update", "--auth"]
        stdinEnabled: true
        onStarted: {
            write(pg.pendingSecret + "\n");
            pg.pendingSecret = "";
        }
        stderr: StdioCollector { id: authErr }
        onExited: (code) => {
            if (code !== 0) {
                pg.authBusy = false;
                pg.actionError = authErr.text.trim().replace(/^ryoku: /, "");
            }
        }
    }
    Process {
        id: authCancelProc
        command: ["ryoku", "update", "--auth", "--cancel"]
    }

    // stop the run: it gives back what it quiesced (the shell, the sleep
    // guard) and records the stop; a package transaction finishes on its own
    function stopUpdate() {
        pg.stopArmed = false;
        stopProc.running = true;
    }
    Process {
        id: stopProc
        command: ["ryoku", "update", "--cancel"]
        environment: Spawn.env
        stderr: StdioCollector { id: stopErr }
        onExited: (code) => {
            if (code !== 0)
                pg.actionError = stopErr.text.trim().replace(/^ryoku: /, "");
        }
    }

    // one calm row under a settled run: a short line plus the Rashin handoff. The
    // caller sets its width and gates it on showFixWithAI, so it shares the done
    // and error views without either laying it out when Rashin has nothing to fix.
    component FixRow: Column {
        id: fr
        spacing: Tokens.s3

        Rectangle { width: fr.width; height: 1; color: Tokens.lineSoft }

        Row {
            width: fr.width
            spacing: Tokens.s3
            Text {
                width: fr.width - fixBtn.width - Tokens.s3
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("The health check found issues after this update.")
                color: Tokens.inkMuted; font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap
            }
            Btn {
                id: fixBtn
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("FIX WITH AI")
                primary: true
                onAct: pg.fixWithAI()
            }
        }
    }

    // idle list: incoming commits when behind, else the recent history the
    // installed version contains, so the page is informative either way.
    readonly property var sectionModel: Updates.available ? Updates.updates : Updates.recent
    readonly property string sectionLabel: Updates.available ? I18n.tr("INCOMING COMMITS") : I18n.tr("RECENT CHANGES")

    // ── head: eyebrow, Fraunces title, blurb (matches every settings page) ──
    Column {
        id: head
        anchors { left: parent.left; right: parent.right; top: parent.top }
        anchors.topMargin: Tokens.s6
        // the register row sits off the title: a rule over a 32px
        // title needs more than the gap between two lines of body text
        spacing: Tokens.s3

        Row {
            // the register row holds a fixed box, so the rule and the seal keep
            // their distance from the title on every page
            height: Tokens.s5
            spacing: Tokens.s2
            Rectangle {
                width: 16; height: 1; color: Tokens.ink
                anchors.verticalCenter: parent.verticalCenter
            }
            Text {
                text: "力"; color: Tokens.ink; font.family: Tokens.jp
                font.pixelSize: 11; anchors.verticalCenter: parent.verticalCenter
            }
            Text {
                text: I18n.tr("SYSTEM"); color: Tokens.inkMuted; font.family: Tokens.ui
                font.pixelSize: 9; font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                anchors.verticalCenter: parent.verticalCenter
            }
        }
        Text {
            text: I18n.tr("Updates"); color: Tokens.ink
            font.family: Tokens.display; font.pixelSize: Tokens.fTitle
        }
        Text {
            width: Math.min(parent.width, 720)
            text: I18n.tr("What sits behind origin, and a one-click update.")
            color: Tokens.inkMuted; font.family: Tokens.ui
            font.pixelSize: Tokens.fBody; wrapMode: Text.WordWrap
        }
        Text {
            visible: pg.snapshotReason !== ""
            width: Math.min(parent.width, 720)
            text: pg.snapshotReason
            color: Tokens.inkFaint; font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap
        }
    }

    // ── idle: live status + the commit list, in one scroll container ─────────
    Flickable {
        id: idle
        visible: pg.phase === "idle" && !pg.starting
        anchors {
            left: parent.left; right: parent.right
            top: head.bottom; bottom: footer.top
            leftMargin: Tokens.s6; rightMargin: Tokens.s6
            topMargin: Tokens.s5; bottomMargin: Tokens.s4
        }
        contentWidth: width
        contentHeight: idleCol.height
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
        WheelScroll { }

        Column {
            id: idleCol
            width: idle.width - Tokens.s3   // reserve a lane for the scroll rail
            spacing: Tokens.s5

            // ── status readout (left) + automatic-check schedule (right) ──
            Item {
                width: idleCol.width
                implicitHeight: Math.max(status.implicitHeight, autoCol.implicitHeight)

                // editorial status, not a boxed banner. a 2px rule encodes state
                // (an update is available), not style: bright when behind, faint
                // when current -- the ink translation of the old ember bar.
                Item {
                    id: status
                    anchors.left: parent.left
                    anchors.right: autoCol.left
                    anchors.rightMargin: Tokens.s6
                    anchors.top: parent.top
                    implicitHeight: statusCol.implicitHeight

                    Rectangle {
                        id: stateBar
                        anchors.left: parent.left; anchors.top: parent.top
                        width: 2; height: statusCol.implicitHeight
                        color: Tokens.ink
                        opacity: Updates.available ? 1.0 : 0.3
                        antialiasing: false
                    }

                    Column {
                        id: statusCol
                        anchors.left: stateBar.right; anchors.leftMargin: Tokens.s4
                        anchors.top: parent.top
                        spacing: Tokens.s2

                        Text {
                            text: Updates.available ? I18n.tr("UPDATE AVAILABLE") : I18n.tr("UP TO DATE")
                            color: Tokens.ink; font.family: Tokens.ui
                            font.pixelSize: Tokens.fMicro; font.weight: Font.Medium
                            font.letterSpacing: Tokens.trackMark
                        }

                        // the release line this box runs, and the next one when
                        // the channel has moved on ("Onogoro -> Amaterasu").
                        Text {
                            visible: Updates.currentName !== ""
                            text: "Ryoku " + Updates.currentName
                                  + (Updates.available && Updates.latestName !== "" && Updates.latestName !== Updates.currentName
                                     ? "  \u2192  " + Updates.latestName : "")
                            color: Tokens.ink; font.family: Tokens.display
                            font.pixelSize: Tokens.fHero; font.weight: Font.Medium
                        }

                        // installed -> latest bump. a version is file-truth, so mono.
                        Row {
                            spacing: Tokens.s3
                            Text {
                                anchors.verticalCenter: parent.verticalCenter
                                text: Updates.currentVersion
                                color: Tokens.inkDim; font.family: Tokens.mono
                                font.pixelSize: Tokens.fValue
                            }
                            Text {
                                anchors.verticalCenter: parent.verticalCenter
                                visible: Updates.available && Updates.latestVersion !== "" && Updates.latestVersion !== Updates.currentVersion
                                text: "\u2192"
                                color: Tokens.inkFaint; font.family: Tokens.ui
                                font.pixelSize: Tokens.fValue
                            }
                            Text {
                                anchors.verticalCenter: parent.verticalCenter
                                visible: Updates.available && Updates.latestVersion !== "" && Updates.latestVersion !== Updates.currentVersion
                                text: Updates.latestVersion
                                color: Tokens.ink; font.family: Tokens.mono
                                font.pixelSize: Tokens.fValue
                            }
                        }

                        Text {
                            text: Updates.available
                                ? (Updates.behind === 1
                                    ? I18n.tr("%1 commit behind  \u00b7  checked %2").arg(Updates.behind).arg(Updates.checkedAgo)
                                    : I18n.tr("%1 commits behind  \u00b7  checked %2").arg(Updates.behind).arg(Updates.checkedAgo))
                                : I18n.tr("on %1  \u00b7  checked %2").arg(Updates.branch).arg(Updates.checkedAgo)
                            color: Tokens.inkMuted; font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall
                        }
                    }
                }

                // automatic checks: a Seg over the cadence + a derived blurb.
                Column {
                    id: autoCol
                    anchors.right: parent.right; anchors.top: parent.top
                    anchors.topMargin: 2
                    spacing: Tokens.s2

                    Text {
                        anchors.right: parent.right
                        text: I18n.tr("AUTOMATIC CHECKS")
                        color: Tokens.inkMuted; font.family: Tokens.ui
                        font.pixelSize: Tokens.fMicro; font.weight: Font.Medium
                        font.letterSpacing: Tokens.trackMark
                    }
                    Seg {
                        anchors.right: parent.right
                        options: pg.intervalLabels
                        current: pg.intervalLabel(pg.interval)
                        onChose: (l) => pg.setInterval(pg.intervalKey(l))
                    }
                    Text {
                        anchors.right: parent.right
                        text: pg.intervalBlurb(pg.interval)
                        color: Tokens.inkMuted; font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                    }
                }
            }

            // ── the commit list ──
            Column {
                width: idleCol.width
                spacing: 0

                // section head: dot + caps + hairline leader. content flips with
                // availability (incoming vs recent).
                Item {
                    width: parent.width
                    height: 30

                    Row {
                        id: secLabel
                        anchors.left: parent.left
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: Tokens.s2
                        Rectangle {
                            width: 4; height: 4; color: Tokens.ink
                            anchors.verticalCenter: parent.verticalCenter
                        }
                        Text {
                            text: pg.sectionLabel; color: Tokens.ink
                            font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                            font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                            anchors.verticalCenter: parent.verticalCenter
                        }
                    }
                    Rectangle {
                        anchors.left: secLabel.right; anchors.leftMargin: Tokens.s3
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        height: 1; color: Tokens.lineSoft
                    }
                }

                // one incoming commit: a node on a vertical git rail, the commit
                // subject, and a right-aligned version pair. mirrors
                // `git log <channel>..origin/<channel>`.
                Repeater {
                    model: pg.sectionModel

                    delegate: Item {
                        id: row
                        required property var modelData
                        required property int index

                        readonly property real railX: 13
                        readonly property real nodeY: height / 2
                        readonly property bool isFirst: index === 0
                        readonly property bool isLast: index === pg.sectionModel.length - 1
                        readonly property string fromVersion: row.modelData.old || ""
                        readonly property string toVersion: row.modelData.new || ""

                        width: idleCol.width
                        height: 44

                        // upper rail, hidden on the first node.
                        Rectangle {
                            x: row.railX; width: 1; y: 0
                            height: row.nodeY - 6
                            color: Tokens.line; visible: !row.isFirst
                        }
                        // lower rail, hidden on the last node.
                        Rectangle {
                            x: row.railX; width: 1; y: row.nodeY + 6
                            height: row.height - (row.nodeY + 6)
                            color: Tokens.line; visible: !row.isLast
                        }
                        // the node: a hollow ink ring (a true circle is allowed).
                        Rectangle {
                            x: row.railX - 4; y: row.nodeY - 4
                            width: 8; height: 8; radius: 4
                            color: "transparent"
                            border.width: Tokens.border; border.color: Tokens.inkDim
                        }

                        // hover highlight (a cell under the pointer takes tint5).
                        Rectangle {
                            anchors.fill: parent
                            anchors.leftMargin: 28; anchors.topMargin: 3; anchors.bottomMargin: 3
                            radius: Tokens.radius
                            color: rowHover.hovered ? Tokens.tint5 : "transparent"
                            Behavior on color { ColorAnimation { duration: Tokens.snap } }
                        }
                        // parted from the row above the way every card's rows are
                        Rectangle {
                            anchors { left: parent.left; right: parent.right; top: parent.top }
                            anchors.leftMargin: 28; anchors.rightMargin: Tokens.s4
                            height: 1
                            color: Tokens.lineSoft
                            visible: !row.isFirst
                        }

                        Text {
                            id: subj
                            anchors.left: parent.left; anchors.leftMargin: 40
                            anchors.right: ver.left; anchors.rightMargin: Tokens.s4
                            anchors.verticalCenter: parent.verticalCenter
                            text: row.modelData.name
                            color: rowHover.hovered ? Tokens.ink : Tokens.inkDim
                            font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                            font.weight: Font.Medium
                            elide: Text.ElideRight
                            Behavior on color { ColorAnimation { duration: Tokens.snap } }
                        }
                        Row {
                            id: ver
                            anchors.right: parent.right; anchors.rightMargin: Tokens.s4
                            anchors.verticalCenter: parent.verticalCenter
                            spacing: Tokens.s2
                            Text {
                                visible: row.fromVersion !== ""
                                anchors.verticalCenter: parent.verticalCenter
                                text: row.fromVersion
                                color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fMicro
                            }
                            Text {
                                visible: row.fromVersion !== ""
                                anchors.verticalCenter: parent.verticalCenter
                                text: "\u2192"
                                color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fMicro
                            }
                            Text {
                                anchors.verticalCenter: parent.verticalCenter
                                text: row.toVersion
                                color: Tokens.inkMuted; font.family: Tokens.mono; font.pixelSize: Tokens.fMicro
                                font.weight: Font.Medium
                            }
                        }

                        HoverHandler { id: rowHover }
                    }
                }

                Text {
                    visible: pg.sectionModel.length === 0
                    text: Updates.available
                        ? I18n.tr("No commit details available.")
                        : I18n.tr("You're up to date. Recent changes will appear here once loaded.")
                    color: Tokens.inkFaint; font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                    topPadding: Tokens.s2
                    leftPadding: 40
                }
            }

            // ── system packages (pacman -Syu, check-only) ──
            Column {
                width: idleCol.width
                spacing: 0
                visible: Updates.packages.length > 0

                Item {
                    width: parent.width
                    height: 30
                    Row {
                        id: pkgLabel
                        anchors.left: parent.left
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: Tokens.s2
                        Rectangle {
                            width: 4; height: 4; color: Tokens.ink
                            anchors.verticalCenter: parent.verticalCenter
                        }
                        Text {
                            text: I18n.tr("SYSTEM PACKAGES") + "  " + Updates.packages.length
                            color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                            font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                            anchors.verticalCenter: parent.verticalCenter
                        }
                    }
                    Rectangle {
                        anchors.left: pkgLabel.right; anchors.leftMargin: Tokens.s3
                        anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                        height: 1; color: Tokens.lineSoft
                    }
                }

                // These come from Arch or CachyOS, not from Ryoku: `ryoku update`
                // does not move them, so the section has to name what does.
                Text {
                    width: idleCol.width
                    text: I18n.tr("From your distribution, kernel included. Take them with:  sudo pacman -Syu")
                    color: Tokens.inkFaint
                    font.family: Tokens.ui; font.pixelSize: Tokens.fTiny
                    wrapMode: Text.WordWrap
                    bottomPadding: Tokens.s2
                }

                Repeater {
                    model: Updates.packages
                    delegate: Item {
                        id: prow
                        required property var modelData
                        required property int index
                        width: idleCol.width
                        // the same row rhythm as the commit list above and the
                        // settings cards: one height, one hairline between rows
                        height: 44

                        Rectangle {
                            anchors.fill: parent
                            anchors.topMargin: 3; anchors.bottomMargin: 3
                            radius: Tokens.radius
                            color: pkgHover.hovered ? Tokens.tint5 : "transparent"
                            Behavior on color { ColorAnimation { duration: Tokens.snap } }
                        }
                        // the row law: a hairline between rows, the same rhythm the
                        // settings cards use
                        Rectangle {
                            anchors { left: parent.left; right: parent.right; top: parent.top }
                            anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s4
                            height: 1
                            color: Tokens.lineSoft
                            visible: prow.index > 0
                        }
                        Text {
                            anchors.left: parent.left; anchors.leftMargin: Tokens.s4
                            anchors.right: pver.left; anchors.rightMargin: Tokens.s3
                            anchors.verticalCenter: parent.verticalCenter
                            text: prow.modelData.name
                            color: pkgHover.hovered ? Tokens.ink : Tokens.inkDim
                            font.family: Tokens.ui; font.pixelSize: Tokens.fSmall; font.weight: Font.Medium
                            elide: Text.ElideRight
                            Behavior on color { ColorAnimation { duration: Tokens.snap } }
                        }
                        Row {
                            id: pver
                            anchors.right: parent.right; anchors.rightMargin: Tokens.s4
                            anchors.verticalCenter: parent.verticalCenter
                            spacing: Tokens.s2
                            // the pair reads as two columns rather than one run-on
                            // string, and the incoming version carries more ink
                            // than the one being replaced
                            Text {
                                visible: (prow.modelData.old || "") !== ""
                                anchors.verticalCenter: parent.verticalCenter
                                text: prow.modelData.old || ""
                                color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fMicro
                            }
                            Text {
                                visible: (prow.modelData.old || "") !== ""
                                anchors.verticalCenter: parent.verticalCenter
                                text: "\u2192"
                                color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fMicro
                            }
                            Text {
                                anchors.verticalCenter: parent.verticalCenter
                                text: prow.modelData.new || ""
                                color: Tokens.inkMuted; font.family: Tokens.mono; font.pixelSize: Tokens.fMicro
                                font.weight: Font.Medium
                            }
                        }
                        HoverHandler { id: pkgHover }
                    }
                }
            }

        }
    }

    // ── a run: starting, running, done or failed. The timeline stays up once
    // the run settles, so a failure shows where it stopped and a success what
    // it did; the raw log opens beneath it on DETAILS. ──
    Flickable {
        id: runView
        visible: pg.starting || pg.phase === "running" || pg.phase === "done" || pg.phase === "error"
        anchors {
            left: parent.left; right: parent.right
            top: head.bottom; bottom: footer.top
            leftMargin: Tokens.s6; rightMargin: Tokens.s6
            topMargin: Tokens.s5; bottomMargin: Tokens.s4
        }
        contentWidth: width
        contentHeight: runCol.implicitHeight
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
        WheelScroll { }

        Column {
            id: runCol
            x: Math.max(0, (runView.width - width) / 2)
            width: Math.min(runView.width - Tokens.s3, 680)
            topPadding: Tokens.s4
            spacing: Tokens.s5

            UpdateRun {
                width: runCol.width
                run: pg
            }

            Text {
                width: runCol.width
                visible: pg.phase === "error" && pg.errorMsg !== "stopped by request"
                text: pg.snapshotsSupported && pg.snapshot !== ""
                    ? I18n.tr("The system was snapshotted before the update. Roll back to undo every change.")
                    : I18n.tr("DETAILS shows everything the update printed.")
                color: Tokens.inkFaint; font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                lineHeight: 1.35
                wrapMode: Text.WordWrap
            }

            FixRow {
                width: runCol.width
                visible: pg.showFixWithAI
            }

            UpdateLog {
                width: runCol.width
                height: Math.max(220, runView.height * 0.45)
                visible: pg.showLog
                path: pg.logPath !== "" ? pg.logPath : pg.defaultLogPath
            }
        }
    }

    // ── prompt: the editorial question `ryoku update` is blocked on -- an ink
    // rule + title + detail, with the option stamps `ryoku update` offers. ──
    Item {
        visible: pg.phase === "prompt"
        anchors {
            left: parent.left; right: parent.right
            top: head.bottom; bottom: footer.top
            leftMargin: Tokens.s6; rightMargin: Tokens.s6
        }

        Row {
            anchors.centerIn: parent
            width: Math.min(parent.width, 540)
            spacing: Tokens.s4

            Rectangle {
                width: 2; height: promptCol.implicitHeight
                color: Tokens.ink; antialiasing: false
            }

            Column {
                id: promptCol
                width: parent.width - 2 - Tokens.s4
                spacing: Tokens.s4

                Text {
                    width: parent.width
                    text: pg.promptTitle
                    color: Tokens.ink; font.family: Tokens.ui
                    font.pixelSize: Tokens.fValue; font.weight: Font.DemiBold
                    wrapMode: Text.WordWrap
                }
                Text {
                    width: parent.width
                    text: pg.promptDetail
                    color: Tokens.inkMuted; font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                    lineHeight: 1.35
                    wrapMode: Text.WordWrap
                }

                Item { width: 1; height: Tokens.s1 }

                // the option stamps: arbitrary strings from the CLI, uppercased.
                // the first is the armed primary (inverts to bone), the rest are
                // ghosts -- the beta18 button carries both.
                Row {
                    spacing: Tokens.s3
                    Repeater {
                        model: pg.promptOptions
                        delegate: Btn {
                            required property var modelData
                            required property int index
                            text: ("" + modelData).toUpperCase()
                            primary: index === 0
                            onAct: pg.answer(modelData)
                        }
                    }
                }
            }
        }
    }

    // ── auth: the password the run needs for sudo, asked for right here ──
    Item {
        visible: pg.phase === "auth"
        anchors {
            left: parent.left; right: parent.right
            top: head.bottom; bottom: footer.top
            leftMargin: Tokens.s6; rightMargin: Tokens.s6
        }

        UpdateAuth {
            anchors.centerIn: parent
            width: Math.min(parent.width, 560)
            run: pg
        }
    }

    // ── action bar, pinned at the bottom (60 tall, one hairline) ─────────────
    Item {
        id: footer
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        height: 60

        Rectangle {
            anchors.left: parent.left; anchors.right: parent.right; anchors.top: parent.top
            height: 1; color: Tokens.line
        }

        Text {
            anchors.left: parent.left; anchors.leftMargin: Tokens.s6
            anchors.right: actions.left; anchors.rightMargin: Tokens.s4
            anchors.verticalCenter: parent.verticalCenter
            text: pg.actionError !== "" ? pg.actionError
                : pg.live ? I18n.tr("Update running") + (pg.started > 0 ? "  \u00b7  " + pg.human(pg.now - pg.started) : "")
                : pg.starting ? I18n.tr("Starting the update")
                : pg.phase === "error" ? (pg.errorMsg === "stopped by request" ? I18n.tr("Update stopped") : I18n.tr("Update failed"))
                : pg.phase === "done" ? I18n.tr("Update complete")
                : (Updates.branch + (Updates.currentVersion !== "" ? ("  \u00b7  " + Updates.currentVersion) : ""))
            color: pg.actionError !== "" ? Tokens.ink : Tokens.inkFaint
            font.family: pg.actionError !== "" ? Tokens.ui : Tokens.mono
            font.pixelSize: pg.actionError !== "" ? Tokens.fSmall : Tokens.fMicro
            elide: Text.ElideRight
        }

        // STOP asks twice: a stray click must not end a healthy run
        Timer {
            interval: 4000
            running: pg.stopArmed
            onTriggered: pg.stopArmed = false
        }

        Row {
            id: actions
            anchors.right: parent.right; anchors.rightMargin: Tokens.s6
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s3

            Btn {
                anchors.verticalCenter: parent.verticalCenter
                visible: pg.phase === "idle" && !pg.starting
                text: I18n.tr("CHECK AGAIN")
                onAct: Updates.check()
            }
            Btn {
                anchors.verticalCenter: parent.verticalCenter
                visible: pg.phase === "idle" && !pg.starting && Updates.available
                text: I18n.tr("UPDATE NOW")
                primary: true
                onAct: pg.startUpdate()
            }
            Btn {
                anchors.verticalCenter: parent.verticalCenter
                visible: pg.starting || pg.phase === "running" || pg.phase === "done" || pg.phase === "error"
                text: pg.showLog ? I18n.tr("HIDE DETAILS") : I18n.tr("DETAILS")
                onAct: pg.showLog = !pg.showLog
            }
            Btn {
                anchors.verticalCenter: parent.verticalCenter
                visible: pg.live
                text: pg.stopArmed ? I18n.tr("CONFIRM STOP") : I18n.tr("STOP")
                primary: pg.stopArmed
                onAct: pg.stopArmed ? pg.stopUpdate() : (pg.stopArmed = true)
            }
            Btn {
                anchors.verticalCenter: parent.verticalCenter
                visible: pg.snapshotsSupported && pg.phase === "error" && pg.snapshot !== ""
                text: I18n.tr("ROLL BACK")
                onAct: pg.rollback()
            }
            Btn {
                anchors.verticalCenter: parent.verticalCenter
                visible: pg.phase === "done" || pg.phase === "error"
                text: pg.phase === "done" ? I18n.tr("DONE") : I18n.tr("DISMISS")
                primary: true
                onAct: pg.dismiss()
            }
        }
    }
}
