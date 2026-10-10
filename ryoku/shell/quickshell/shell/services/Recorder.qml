pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import "lib/screens.js" as Screens

// Screen recording state + control for GPU Screen Recorder. The backend
// (ryoku-cmd-record) writes recorder-status.json the moment it launches the
// recorder and removes it on every exit, so that file is the capture's lifetime:
// watching it shows the island at once for a capture started anywhere (the
// capture card, ryoshot, the CLI) and carries the launch time, so the clock is
// right from its first frame and survives a shell reload. GSR's IPC socket is
// polled as the backstop for a stale file. Control goes through the one command
// API -- `ryoku-shell record start|stop|pause|resume` and `record settings` --
// so nothing here builds a recorder invocation of its own.
// The capture popout and the floating record island share this single source of truth.
Singleton {
    id: root

    property bool active: false
    property bool available: true
    property int recorderPid: 0
    property int startedAt: 0
    property int elapsedSec: 0
    readonly property string elapsedText: fmt(elapsedSec)

    // The record island and capture popout watch anyActive and its changed signal;
    // with the standalone editor gone it tracks the one live capture directly.
    readonly property bool anyActive: root.active

    // Pause toggle the daemon relays to GSR's IPC socket. The status file carries
    // no paused flag, so the poll cannot reconcile it: the flip is optimistic and
    // reset the moment a capture ends.
    property bool paused: false

    // live audio mode from the backend's status file: none|system|microphone|both,
    // and whether a system+mic mix fell back to a single source.
    property string activeAudioMode: "none"
    property bool audioFallback: false

    // region capture: the box drawn as "WxH+X+Y" (global logical), set when a
    // region recording starts so RegionOverlay can draw the live boundary; ""
    // means a full monitor. A remembered box is valid only for the monitor layout
    // it was drawn in, so it is stamped with a cheap layout signature and dropped
    // the moment the live layout stops matching -- a stale box would land off-screen
    // or on an output that no longer exists.
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
        path: (Quickshell.env("RYOKU_STATE_PATH") || (Quickshell.env("HOME") + "/.local/state/ryoku")) + "/record.json"
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
        Qt.callLater(root.refreshStatus);
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
        Quickshell.execDetached(["ryoku-shell", "record", "settings", key, String(value)]);
    }

    // desktop-audio + mic flags -> ryoku-shell record args, shared by the card and
    // any other trigger so the argument list is never built two ways.
    function recordArgs() {
        var a = [];
        if (root.optDesktopAudio) a.push("--with-desktop-audio");
        if (root.optMic) a.push("--with-microphone-audio");
        return a;
    }

    // The backend's status file: present exactly while a capture runs. A capture
    // seen here that the shell did not start adopts its launch time, so the clock
    // never counts from when the shell first noticed.
    property bool statusPresent: false
    function applyStatus() {
        if (!root.available) {
            root.statusPresent = false;
            root.clearStatus();
            return;
        }
        var p;
        try {
            p = JSON.parse(statusView.text() || "{}");
        } catch (e) {
            p = {};
        }
        root.statusPresent = true;
        root.recorderPid = Number(p.recorderPid ?? 0);
        root.activeAudioMode = String(p.activeAudioMode ?? "none");
        root.audioFallback = p.audioFallback === true;
        const launched = Number(p.startedAt ?? 0);
        if (launched > 0)
            root.startedAt = launched;
        else if (root.startedAt <= 0)
            root.startedAt = Math.floor(Date.now() / 1000);
        root.elapsedSec = Math.max(0, Math.floor(Date.now() / 1000) - root.startedAt);
        if (!root.active) {
            root.active = true;
            root.paused = false;
        }
        // The launch stamp just arrived: tick from it, so each second rolls on
        // the capture's own boundary instead of up to a second late.
        tick.restart();
    }
    function clearStatus() {
        root.active = false;
        root.startedAt = 0;
        root.elapsedSec = 0;
        root.regionGeom = "";
        root.activeAudioMode = "none";
        root.audioFallback = false;
        root.paused = false;
        root.recorderPid = 0;
    }
    FileView {
        id: statusView
        path: (Quickshell.env("RYOKU_STATE_PATH") || (Quickshell.env("HOME") + "/.local/state/ryoku")) + "/recorder-status.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: root.applyStatus()
        onLoadFailed: {
            // The backend removed its file: the capture is over, whatever the
            // optimistic state says; the poll confirms.
            root.statusPresent = false;
            if (root.active)
                root.clearStatus();
            root.refreshStatus();
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
        if (!root.available)
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
                root.countingDown = false;
                var a = root.pendingArgs;
                root.pendingArgs = [];
                root.start(a);
            }
        }
    }

    function start(extraArgs) {
        if (!root.available)
            return;
        var a = (extraArgs || []).slice();
        // Discord quick-compress rides the backend: pass --discord and ryoku-cmd-
        // record writes the <name>.discord.mp4 copy once the capture is finalised.
        if (root.discordMode)
            a.push("--discord");
        // A region capture carries "--region --geometry WxH+X+Y"; stamp the box so
        // RegionOverlay draws the live boundary and it drops on a layout change.
        var isRegion = false, geom = "";
        for (var i = 0; i < a.length; i++) {
            if (a[i] === "--region")
                isRegion = true;
            else if (a[i] === "--geometry" && i + 1 < a.length)
                geom = a[i + 1];
        }
        if (isRegion && /^\d+x\d+\+\d+\+\d+$/.test(geom)) {
            root.regionGeom = geom;
            root.regionLayoutSig = root.layoutSig;
        } else {
            root.regionGeom = "";
        }
        Quickshell.execDetached(["ryoku-shell", "record", "start"].concat(a));
        // Optimistic, so the island shows at once; the backend's status file then
        // supplies the launch time and the poll confirms the recorder came up.
        root.active = true;
        root.paused = false;
        root.startedAt = Math.floor(Date.now() / 1000);
        root.elapsedSec = 0;
    }

    // Pause/resume the live capture through the daemon (which relays to GSR's IPC
    // socket). Optimistic: flipped here, and the poll resets it when a capture ends.
    function togglePause() {
        Quickshell.execDetached(["ryoku-shell", "record", root.paused ? "resume" : "pause"]);
        root.paused = !root.paused;
    }

    function stop() {
        // a stop during the pre-record countdown just cancels it: nothing launched,
        // so there is nothing to stop.
        if (root.countingDown) {
            root.cancelCountdown();
            return;
        }
        Quickshell.execDetached(["ryoku-shell", "record", "stop"]);
        root.active = false;
        root.paused = false;
        root.regionGeom = "";
    }

    // ── reconcile against the live process. The status file says a capture runs;
    // GSR's IPC status backs it up so a stale file (the backend killed outright)
    // can never strand the island. GSR brings its socket up a moment after launch,
    // so a negative answer inside the first seconds of a capture is the socket
    // warming up, not a stop; a launch that never produced a file clears once that
    // window passes.
    readonly property int socketWarmupSec: 5
    function refreshStatus() {
        if (root.available && !poll.running)
            poll.running = true;
    }

    Process {
        id: poll
        command: ["/bin/sh", "-c",
            "command -v gsr-cli >/dev/null 2>&1 || exit 127; "
            + "exec gsr-cli -ipc \"$1\" status",
            "ryoku-gsr-status",
            (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-gsr.sock"]
        onExited: (exitCode) => {
            if (exitCode === 127) {
                root.available = false;
                if (root.active)
                    root.clearStatus();
                return;
            }
            const answering = exitCode === 0;
            const warming = root.active && root.startedAt > 0
                && Math.floor(Date.now() / 1000) - root.startedAt < root.socketWarmupSec;
            if (!answering && !warming) {
                if (root.active)
                    root.clearStatus();
                return;
            }
            if (!root.active) {
                root.active = true;
                root.paused = false;
            }
            // A capture the shell has not read the file for yet (a reload mid-capture
            // before the watch delivered): adopt its launch time now.
            if (!root.statusPresent)
                statusView.reload();
        }
    }

    // Tick the clock and back the status file up with GSR's IPC: every second
    // while a capture is live, every five idle, in case a backend died without
    // removing its file. The file watch, not this poll, is what notices a start.
    Timer {
        id: tick
        interval: root.anyActive ? 1000 : 5000
        running: root.available
        repeat: true
        triggeredOnStart: true
        onTriggered: {
            if (root.anyActive && root.startedAt > 0)
                root.elapsedSec = Math.max(0, Math.floor(Date.now() / 1000) - root.startedAt);
            root.refreshStatus();
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
