pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "lib/screens.js" as Screens

// Screen recording state + control for GPU Screen Recorder. ryoku-cmd-record
// keeps recorder-status.json while a capture is live and writes
// recorder-result.json before it removes it, so the state here is read, never
// guessed, for a capture started anywhere and across a shell reload. Control goes
// through the daemon's record verbs over the bus. Every recording surface reads
// this one service.
Singleton {
    id: root

    readonly property string statePath: Quickshell.env("RYOKU_STATE_PATH") || (Quickshell.env("HOME") + "/.local/state/ryoku")

    // idle | arming | starting | recording | paused | stopping | saved | failed
    // a binding reading another derived binding flashed recording before paused when I loaded
    // a paused capture, so every flag here reads the base state through the functions below
    readonly property string phase: root._phaseOf(root._isLive())
    // on from the start request until the file goes, stop() alone never clears it
    readonly property bool active: {
        const p = root._phaseOf(root._isLive());
        return p === "starting" || p === "recording" || p === "paused" || p === "stopping";
    }
    readonly property bool anyActive: root.active
    readonly property bool stopping: root._isStopping(root._isLive())
    readonly property bool paused: root._isPaused(root._isLive())

    property bool available: true
    // the status file is here and its capture wasn't proven dead
    readonly property bool live: root._isLive()

    function _isLive() {
        return root.available && root.statusPresent && root.startedAt > 0 && root.startedAt !== root._deadStartedAt;
    }
    function _isStopping(live) {
        return live ? (root._fileStopping || root._stopSent) : (root._startSentAt > 0 && root._stopPending);
    }
    function _isPaused(live) {
        return live && (root._flipAt > 0 ? root._flipPaused : root._filePaused);
    }
    function _phaseOf(live) {
        if (live)
            return root._isStopping(true) ? "stopping" : root._isPaused(true) ? "paused" : "recording";
        if (root.countingDown)
            return "arming";
        if (root._startSentAt > 0)
            return root._stopPending ? "stopping" : "starting";
        return root._hold !== "" ? root._hold : "idle";
    }

    // pauses left out, frozen while paused or saving, kept through the saved/failed hold
    property int elapsedSec: 0
    readonly property string elapsedText: fmt(elapsedSec)

    // target is GSR's -w value, a monitor, "screen" or a WxH+X+Y box, "" when unknown
    property bool statusPresent: false
    property int recorderPid: 0
    property real startedAt: 0
    property string target: ""
    property string activeAudioMode: "none"
    property bool audioFallback: false

    // {ok, path, reason, endedAt, cancelled} of the last start attempt
    property var lastResult: ({ ok: false, path: "", reason: "", endedAt: 0, cancelled: false })

    readonly property int socketWarmupSec: 5
    // GSR silent this long after a stop with the file still there means the backend died
    readonly property int stopGraceSec: 15
    readonly property int resultFreshSec: 10
    readonly property int savedHoldMs: 2400
    readonly property int failedHoldMs: 4000

    property bool _filePaused: false
    property real _pausedAt: 0
    property real _pausedTotal: 0
    property bool _fileStopping: false
    // launch time of a capture the poll proved dead; its file stays until the next start
    property real _deadStartedAt: 0
    property real _startSentAt: 0
    property bool _stopPending: false
    property bool _stopSent: false
    property real _stoppingSince: 0
    property real _frozenAt: 0
    // optimistic pause flip, held until the file agrees or 2 s pass
    property bool _flipPaused: false
    property real _flipAt: 0
    property string _hold: ""
    property real _consumedEndedAt: 0
    property bool _resultUnshown: false
    // the result already on disk at start-up is never shown
    property bool _resultsReady: false
    property real _polledAt: 0

    // region capture: the box drawn as "WxH+X+Y" (global logical), so RegionOverlay
    // can draw the live boundary; "" means a full monitor. A remembered box is valid
    // only for the monitor layout it was drawn in, so it is stamped with a cheap
    // layout signature and dropped the moment the live layout stops matching -- a
    // stale box would land off-screen or on an output that no longer exists.
    property string regionGeom: ""
    property string regionLayoutSig: ""
    readonly property string layoutSig: {
        var out = Screens.uniqueByName(Quickshell.screens);
        var parts = [];
        for (var i = 0; i < out.length; i++) {
            var s = out[i];
            parts.push(s.name + "@" + s.x + "," + s.y + ":" + s.width + "x" + s.height);
        }
        parts.sort();
        return parts.join("|");
    }
    onLayoutSigChanged: {
        if (root.regionGeom !== "" && root.regionLayoutSig !== root.layoutSig)
            root.regionGeom = "";
    }
    onRegionGeomChanged: if (root.regionGeom === "") root.regionLayoutSig = "";

    // these fire mid-change, so they read base state
    onLiveChanged: root._refreshClock()
    onStoppingChanged: {
        const live = root._isLive();
        if (live && root._isStopping(live) && root._frozenAt === 0) {
            root._frozenAt = Date.now() / 1000;
            root._stoppingSince = Date.now();
        }
        root._refreshClock();
    }

    // its monitor, or the screen holding a region's centre, anything unknown covers every screen
    function showsOn(screen) {
        if (!screen)
            return true;
        var t = root.target;
        if (t === "" || t === "screen")
            return true;
        var covers = function (s) {
            var m = /^(\d+)x(\d+)\+(-?\d+)\+(-?\d+)$/.exec(t);
            if (!m)
                return s.name === t;
            var cx = Number(m[3]) + Number(m[1]) / 2;
            var cy = Number(m[4]) + Number(m[2]) / 2;
            return cx >= s.x && cx < s.x + s.width && cy >= s.y && cy < s.y + s.height;
        };
        if (covers(screen))
            return true;
        var outs = Screens.uniqueByName(Quickshell.screens);
        for (var i = 0; i < outs.length; i++)
            if (covers(outs[i]))
                return false;
        return true;
    }

    // Remembered capture options the record card reads, persisted to record.json:
    // the desktop-audio / mic toggles and the Discord quick-compress flag. Shape
    // mirrors Flags: watch for outside edits, write back on change, seed once. A
    // first recording captures the application being demonstrated (desktop audio),
    // not the user's voice -- recording a microphone by default is a privacy surprise.
    property alias optDesktopAudio: recPrefs.desktopAudio
    property alias optMic: recPrefs.mic
    // "discord" makes a Discord-sized copy of a finished clip once the backend
    // finalises it; persisted alongside the audio toggles.
    property alias discordMode: recPrefs.discord
    FileView {
        id: recPrefsFile
        path: root.statePath + "/record.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        atomicWrites: true
        onFileChanged: reload()
        onAdapterUpdated: writeAdapter()
        JsonAdapter {
            id: recPrefs
            property bool desktopAudio: true
            property bool mic: false
            property bool discord: false
        }
    }
    Component.onCompleted: {
        if (!recPrefsFile.text())
            recPrefsFile.writeAdapter();
        root._readResult(true);
        root._resultsReady = true;
        root.refreshStatus();
    }

    // Read-only mirror of the daemon-owned recording settings. The daemon is the
    // sole writer (ryoku-shell record settings); the capture popout shows these and
    // edits them through setSetting(), never by writing the file here. watchChanges
    // picks up the daemon's writes so the chips stay in sync. Defaults match the
    // daemon's so a missing file still reads sensibly.
    property int recFps: 60
    property string recQuality: "very_high"
    property string recBitrateMode: "quality"
    property string recCodec: "h264"
    property string recContainer: "mp4"
    property bool recCursor: true
    FileView {
        id: recSettingsFile
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/recording.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: {
            try {
                const s = JSON.parse(recSettingsFile.text() || "{}");
                root.recFps = Number(s.fps ?? 60);
                root.recQuality = String(s.quality ?? "very_high");
                root.recBitrateMode = String(s.bitrateMode ?? "quality");
                root.recCodec = String(s.codec ?? "h264");
                root.recContainer = String(s.container ?? "mp4");
                root.recCursor = s.cursor !== false;
            } catch (e) {
            }
        }
    }
    // Route one setting change through the daemon, which validates it and rewrites
    // the whole merged file; the mirror above then reloads from the watch.
    function setSetting(key, value) {
        Bus.run(["record", "settings", key, String(value)].join(" "));
    }

    // desktop-audio + mic flags -> record start args, shared by the card and
    // any other trigger so the argument list is never built two ways.
    function recordArgs() {
        var a = [];
        if (root.optDesktopAudio) a.push("--with-desktop-audio");
        if (root.optMic) a.push("--with-microphone-audio");
        return a;
    }

    // a new launch time is a new capture, so the last one's shell-side state goes
    function _applyStatus() {
        var p;
        try {
            p = JSON.parse(statusView.text());
        } catch (e) {
            return;
        }
        if (!p || typeof p !== "object")
            return;
        var launched = Number(p.startedAt ?? 0);
        if (!(launched > 0))
            launched = root.statusPresent && root.startedAt > 0 ? root.startedAt : Date.now() / 1000;
        if (launched !== root.startedAt)
            root._forgetCapture();
        root.startedAt = launched;
        root.recorderPid = Number(p.recorderPid ?? 0);
        root.target = String(p.target ?? "");
        root.activeAudioMode = String(p.activeAudioMode ?? "none");
        root.audioFallback = p.audioFallback === true;
        root._filePaused = p.paused === true;
        root._pausedAt = Number(p.pausedAt ?? 0);
        root._pausedTotal = Number(p.pausedTotal ?? 0);
        root._fileStopping = p.stopping === true;
        root.statusPresent = true;
        if (root._flipAt > 0 && root._filePaused === root._flipPaused) {
            root._flipAt = 0;
            flipHold.stop();
        }
        if (root._startSentAt > 0) {
            root._startSentAt = 0;
            startWatch.stop();
        }
        if (/^\d+x\d+\+-?\d+\+-?\d+$/.test(root.target) && root.regionGeom !== root.target) {
            root.regionGeom = root.target;
            root.regionLayoutSig = root.layoutSig;
        }
        // stop was pressed while it was still starting
        if (root._stopPending) {
            root._stopPending = false;
            if (root.live && !root.stopping)
                root._sendStop();
        }
        root._refreshClock();
    }
    // the result was written before the file went, so read it first and go from stopping straight to saved or failed
    function _statusGone() {
        if (!root.statusPresent)
            return;
        resultView.reload();
        root._readResult(false);
        root._takeResult();
        root.statusPresent = false;
        root.startedAt = 0;
        root._deadStartedAt = 0;
        root._forgetCapture();
    }
    function _forgetCapture() {
        root._stopSent = false;
        root._stoppingSince = 0;
        root._frozenAt = 0;
        root._flipAt = 0;
        flipHold.stop();
        root._filePaused = false;
        root._pausedAt = 0;
        root._pausedTotal = 0;
        root._fileStopping = false;
        root.recorderPid = 0;
        root.target = "";
        root.activeAudioMode = "none";
        root.audioFallback = false;
        root.regionGeom = "";
        if (root._hold === "")
            root.elapsedSec = 0;
    }
    FileView {
        id: statusView
        path: root.statePath + "/recorder-status.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: root._applyStatus()
        onLoadFailed: root._statusGone()
    }

    // only a result new to this instance and fresh is shown, a reload or an old file never replays one
    function _readResult(initial) {
        var r;
        try {
            r = JSON.parse(resultView.text());
        } catch (e) {
            return;
        }
        var ended = Number(r && r.endedAt);
        if (!(ended > root._consumedEndedAt))
            return;
        root._consumedEndedAt = ended;
        root.lastResult = {
            ok: r.ok === true,
            path: String(r.path ?? ""),
            reason: String(r.reason ?? ""),
            endedAt: ended,
            cancelled: r.cancelled === true
        };
        if (initial || Math.abs(Date.now() / 1000 - ended) > root.resultFreshSec)
            return;
        root._resultUnshown = true;
        if (!root.live)
            root._takeResult();
    }
    // the hold goes up before the start or capture is let go, so the phase never passes through idle
    function _takeResult() {
        if (!root._resultUnshown)
            return;
        root._resultUnshown = false;
        if (root.lastResult.cancelled) {
            root._hold = "";
            holdTimer.stop();
        } else {
            root._hold = root.lastResult.ok ? "saved" : "failed";
            holdTimer.interval = root.lastResult.ok ? root.savedHoldMs : root.failedHoldMs;
            holdTimer.restart();
        }
        root._endStart();
    }
    function _endStart() {
        startWatch.stop();
        if (root._startSentAt === 0)
            return;
        root._startSentAt = 0;
        root._stopPending = false;
        if (!root.live) {
            root.target = "";
            root.regionGeom = "";
        }
    }
    FileView {
        id: resultView
        path: root.statePath + "/recorder-result.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: if (root._resultsReady) root._readResult(false)
    }
    Timer {
        id: holdTimer
        onTriggered: {
            root._hold = "";
            if (!root.live)
                root.elapsedSec = 0;
        }
    }

    // pre-record countdown: the capture popout can arm a delay (Capture.delay,
    // 0/1/3/5/10s) so the desktop is framed before capture begins. startAfter ticks
    // that delay down -- the island renders it -- then calls start(), so the count
    // is honest. args are latched so a mid-count option change can't retarget it.
    property int countdownSec: 0
    property bool countingDown: false
    property var pendingArgs: []
    function startAfter(args, secs) {
        if (!root.available || root.active || root.countingDown)
            return;
        if (secs <= 0) {
            root.start(args);
            return;
        }
        root.pendingArgs = args || [];
        root.countdownSec = secs;
        root.countingDown = true;
        countdown.restart();
    }
    function cancelCountdown() {
        countdown.stop();
        root.countingDown = false;
        root.countdownSec = 0;
        root.pendingArgs = [];
    }
    Timer {
        id: countdown
        interval: 1000
        repeat: true
        onTriggered: {
            root.countdownSec--;
            if (root.countdownSec <= 0) {
                countdown.stop();
                var a = root.pendingArgs;
                root.pendingArgs = [];
                root.start(a);
                root.countingDown = false;
            }
        }
    }

    function start(extraArgs) {
        if (!root.available || root.active)
            return;
        var a = (extraArgs || []).slice();
        // Discord quick-compress rides the backend: pass --discord and ryoku-cmd-
        // record writes the <name>.discord.mp4 copy once the capture is finalised.
        if (root.discordMode)
            a.push("--discord");
        // surfaces can place themselves while it starts
        var isRegion = false, geom = "", monitor = "";
        for (var i = 0; i < a.length; i++) {
            if (a[i] === "--region")
                isRegion = true;
            else if (a[i] === "--geometry" && i + 1 < a.length)
                geom = a[i + 1];
            else if (a[i] === "--monitor" && i + 1 < a.length)
                monitor = a[i + 1];
        }
        if (isRegion && /^\d+x\d+\+-?\d+\+-?\d+$/.test(geom)) {
            root.regionGeom = geom;
            root.regionLayoutSig = root.layoutSig;
            root.target = geom;
        } else {
            root.regionGeom = "";
            root.target = isRegion ? "" : monitor;
        }
        Bus.run(["record", "start"].concat(a).join(" "));
        root._startSentAt = Date.now();
        root._stopPending = false;
        startWatch.restart();
        holdTimer.stop();
        root._hold = "";
        root.elapsedSec = 0;
    }
    // no status file in time and no fresh result either, back to idle
    Timer {
        id: startWatch
        interval: root.socketWarmupSec * 1000
        onTriggered: root._endStart()
    }

    // ignored while saving
    function togglePause() {
        if (!root.live || root.stopping)
            return;
        var want = !root.paused;
        Bus.run(want ? "record pause" : "record resume");
        root._flipPaused = want;
        root._flipAt = Date.now();
        flipHold.restart();
        root._refreshClock();
    }
    Timer {
        id: flipHold
        interval: 2000
        onTriggered: {
            root._flipAt = 0;
            root._refreshClock();
        }
    }

    // one stop per capture, it cancels a countdown and waits for a capture that is still starting
    function stop() {
        if (root.countingDown) {
            root.cancelCountdown();
            return;
        }
        if (root.live) {
            if (!root.stopping)
                root._sendStop();
            return;
        }
        if (root._startSentAt > 0)
            root._stopPending = true;
    }
    function _sendStop() {
        Bus.run("record stop");
        root._stopSent = true;
    }

    // ticks on the capture's own second boundary
    function _recordedSeconds(nowSec) {
        if (!root._isLive())
            return 0;
        var end = nowSec;
        var pausedTotal = root._pausedTotal;
        if (root._filePaused && root._pausedAt > 0) {
            if (root._flipAt > 0 && !root._flipPaused)
                pausedTotal += Math.max(0, root._flipAt / 1000 - root._pausedAt);
            else
                end = root._pausedAt;
        } else if (root._flipAt > 0 && root._flipPaused) {
            end = root._flipAt / 1000;
        }
        if (root._frozenAt > 0)
            end = Math.min(end, root._frozenAt);
        return Math.max(0, end - root.startedAt - pausedTotal);
    }
    function _refreshClock() {
        const live = root._isLive();
        if (!live) {
            tick.stop();
            return;
        }
        var rec = root._recordedSeconds(Date.now() / 1000);
        root.elapsedSec = Math.floor(rec + 0.0005);
        var moving = !root._isPaused(live) && !root._isStopping(live);
        var ms = moving ? Math.ceil((root.elapsedSec + 1 - rec) * 1000) + 8 : 1000;
        tick.interval = Math.max(16, Math.min(1000, ms));
        tick.restart();
    }
    // only while live, it also asks GSR once a second whether the capture still runs
    Timer {
        id: tick
        onTriggered: {
            var now = Date.now();
            if (now - root._polledAt >= 900) {
                root._polledAt = now;
                root.refreshStatus();
            }
            root._refreshClock();
        }
    }

    // GSR answers only for the capture this backend launched, so silence with the file still
    // there is a backend killed outright, unless the socket is still warming up or a save is finishing
    function refreshStatus() {
        if (root.available && !poll.running) {
            poll.launched = false;
            poll.running = true;
        }
    }
    Process {
        id: poll
        property bool launched: false
        command: ["gsr-cli", "-ipc",
            (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-gsr.sock",
            "status"]
        onStarted: poll.launched = true
        onRunningChanged: {
            if (!running && !poll.launched && root.available)
                root.available = false;
        }
        onExited: exitCode => {
            if (exitCode === 127) {
                root.available = false;
                return;
            }
            if (exitCode === 0 || !root.live)
                return;
            const now = Date.now();
            if (now / 1000 - root.startedAt < root.socketWarmupSec)
                return;
            if (root.stopping && now - root._stoppingSince < root.stopGraceSec * 1000)
                return;
            root._takeResult();
            root._deadStartedAt = root.startedAt;
            root._forgetCapture();
        }
    }

    function fmt(sec) {
        var s = Math.max(0, Math.round(sec));
        var h = Math.floor(s / 3600);
        var m = Math.floor((s % 3600) / 60);
        var r = s % 60;
        if (h > 0)
            return h + ":" + (m < 10 ? "0" : "") + m + ":" + (r < 10 ? "0" : "") + r;
        return m + ":" + (r < 10 ? "0" : "") + r;
    }
}
