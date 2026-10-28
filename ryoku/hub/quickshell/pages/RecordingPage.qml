pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons

// Recording (DESIGN.md section 11, SYSTEM). The quality knobs behind the bar's
// one-tap screen recorder. The recorder daemon (ryoku-shell record) reads these
// at capture time; environment variables still override any of them per run.
// Constant framerate is the default because variable-framerate files often
// import or play back as ~30fps and look choppy.
//
// Self-contained full-bleed page: it owns its whole content region, so it draws
// its own head, the setting cells regrouped by meaning, the live UNDER THE HOOD
// backend readout, and -- because the shell hides its global action bar -- its
// own dirty status + Reset/Revert/Save bar.
//
// The daemon owns recording.json and is its sole writer: this page reads the
// merged settings over the shell socket (`call record.settings`) on open and
// writes the whole draft back through `ryoku-shell record settings` on Save, so
// nothing lands on disk until Save and no two writers can race. Types matter --
// fps/bitrate/audioBitrate/keyint ride the wire as ints and cursor as a bool, or
// the daemon rejects the write; the coerce() helper keeps the draft wire-correct.
// When the socket is unreachable the page says so rather than showing factory
// defaults as if they were live. These defaults mirror the daemon's own
// fallbacks (ryoku-cmd-record) and must not drift.
Item {
    id: pg

    property var hub
    readonly property bool fullBleed: true

    // factory defaults: the single source RESET walks back to; it mirrors the
    // JsonAdapter defaults below and the shell consumer's fallbacks.
    readonly property var factory: ({
        "fps": 60,
        "framerateMode": "cfr",
        "quality": "very_high",
        "bitrateMode": "quality",
        "bitrate": 20000,
        "codec": "h264",
        "encoder": "gpu",
        "cursor": true,
        "container": "mp4",
        "audioCodec": "opus",
        "audioBitrate": 0,
        "colorRange": "limited",
        "keyint": 2,
        "maxResolution": "native",
        "directory": ""
    })
    readonly property var keyFactory: ({
        "theme": "dark",
        "mode": "all"
    })

    // What an empty `directory` resolves to, so the field can show the real path
    // rather than an empty box. Mirrors Paths.recordingsDir and the recorder's
    // recordings_dir().
    readonly property string defaultDir: {
        const xdg = Quickshell.env("XDG_VIDEOS_DIR");
        const vids = (xdg && xdg.length > 0) ? xdg : (Quickshell.env("HOME") || "") + "/Videos";
        return vids + "/Recordings";
    }

    // committed = what is on disk; draft = the live, previewed edit; dirty = they
    // differ. Both are plain maps, reassigned wholesale so the cells re-render.
    property var committed: null
    property var draft: null
    // recorder daemon reachability, so the page never shows fabricated defaults
    // as if they were the live settings: only a good read flips this to "ready".
    property string recordStatus: "loading"   // loading | ready | unavailable
    property string recordError: ""
    property int pendingReadId: 0
    property int pendingSaveId: 0
    property var keyCommitted: null
    property var keyDraft: null
    property bool keyActive: false
    property string keyBackendStatus: "disabled"
    property string keyBackendError: ""
    readonly property string shellSockPath: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-shell.sock"
    property real keyPreviewRevision: 0
    property int callSerial: 0
    property int pendingKeySaveId: 0
    property string keySettingsError: ""
    property bool reduceMotion: false

    // JSON types the daemon validates against: these keys must ride the wire as
    // ints and bools, not strings, or the write is rejected. The rest are strings.
    readonly property var intKeys: ["fps", "bitrate", "audioBitrate", "keyint"]
    readonly property var boolKeys: ["cursor"]

    readonly property int recordingDirtyCount: {
        if (!pg.draft || !pg.committed)
            return 0;
        var n = 0;
        for (var k in pg.factory)
            if (pg.draft[k] !== pg.committed[k])
                n++;
        return n;
    }
    readonly property int keyDirtyCount: {
        if (!pg.keyDraft || !pg.keyCommitted)
            return 0;
        var n = 0;
        for (var k in pg.keyFactory)
            if (pg.keyDraft[k] !== pg.keyCommitted[k])
                n++;
        return n;
    }
    readonly property int dirtyCount: recordingDirtyCount + keyDirtyCount
    // RESET is a no-op when the draft already equals stock, so gate it on that.
    readonly property bool offDefaults: {
        if (pg.draft) {
            for (var k in pg.factory)
                if (pg.draft[k] !== pg.factory[k])
                    return true;
        }
        if (pg.keyDraft) {
            for (var key in pg.keyFactory)
                if (pg.keyDraft[key] !== pg.keyFactory[key])
                    return true;
        }
        return false;
    }

    function clone(o) {
        var r = {};
        for (var k in o)
            r[k] = o[k];
        return r;
    }
    // Merge a daemon reply's settings map onto the factory keys, coercing the
    // int/bool keys so the draft carries wire-correct types (an int key that
    // arrived as a string would be rejected on the next write).
    function coerce(k, v) {
        if (pg.intKeys.indexOf(k) >= 0) {
            var n = Math.round(Number(v));
            return isNaN(n) ? pg.factory[k] : n;
        }
        if (pg.boolKeys.indexOf(k) >= 0)
            return v === true || v === "true";
        return String(v);
    }
    function mergeFactory(map) {
        var r = {};
        for (var k in pg.factory) {
            var v = (map && map[k] !== undefined && map[k] !== null) ? map[k] : pg.factory[k];
            r[k] = pg.coerce(k, v);
        }
        return r;
    }
    function buildSettings(d) {
        var r = {};
        for (var k in pg.factory)
            r[k] = pg.coerce(k, d[k]);
        return r;
    }

    // ask the daemon for the merged settings; the reply seeds committed (and, on
    // a clean view, the draft too), so the page follows the file's real state.
    function requestRecording() {
        pg.pendingReadId = pg.sendCall("record.settings", {});
    }

    // adopt the daemon's state. First load seeds the draft too; a later refresh
    // rebases committed but keeps an in-flight draft (DESIGN.md section 8), while
    // a clean view simply follows the daemon.
    function adoptRecording(map) {
        var wasClean = pg.draft === null || pg.recordingDirtyCount === 0;
        pg.committed = pg.mergeFactory(map);
        if (wasClean)
            pg.draft = pg.clone(pg.committed);
    }

    function adoptKey(text) {
        var wasClean = pg.keyDraft === null || pg.keyDirtyCount === 0;
        var parsed = KeypressMath.parseSettings(text);
        pg.keyCommitted = { "theme": parsed.theme, "mode": parsed.mode };
        if (wasClean)
            pg.keyDraft = pg.clone(pg.keyCommitted);
    }

    function edit(k, v) {
        if (!pg.draft)
            return;
        var d = pg.clone(pg.draft);
        d[k] = v;
        pg.draft = d;
    }
    // a typed numeric entry commits a string; parse and clamp before it lands, so
    // a bounded key only ever holds an in-range int.
    function editNum(k, text, lo, hi) {
        var n = parseInt(text, 10);
        if (isNaN(n))
            return;
        pg.edit(k, Math.max(lo, Math.min(hi, n)));
    }
    // segmented controls whose stored key differs from the label the user reads:
    // the store keeps cbr/2160p, the control shows Constant/4K.
    function bitrateModeLabel(k) { return k === "cbr" ? "Constant" : "Quality"; }
    function bitrateModeKey(label) { return label === "Constant" ? "cbr" : "quality"; }
    function maxResLabel(k) {
        return k === "1080p" ? "1080p" : k === "1440p" ? "1440p" : k === "2160p" ? "4K" : "Native";
    }
    function maxResKey(label) {
        return label === "1080p" ? "1080p" : label === "1440p" ? "1440p" : label === "4K" ? "2160p" : "native";
    }
    function editKey(k, v) {
        if (!pg.keyDraft)
            return;
        var d = pg.clone(pg.keyDraft);
        d[k] = v;
        pg.keyDraft = d;
        pg.keySettingsError = "";
        if (pg.keyActive)
            pg.showKeyOverlay(true);
    }
    function revert() {
        if (pg.committed)
            pg.draft = pg.clone(pg.committed);
        if (pg.keyCommitted)
            pg.keyDraft = pg.clone(pg.keyCommitted);
        if (pg.keyActive)
            pg.showKeyOverlay(true);
    }
    function reset() {
        if (pg.recordStatus === "ready")
            pg.draft = pg.clone(pg.factory);
        pg.keyDraft = pg.clone(pg.keyFactory);
        if (pg.keyActive)
            pg.showKeyOverlay(true);
    }
    // the daemon is the sole writer: save sends the whole draft in one call and
    // commits only when the reply confirms it, so a rejected key never looks
    // saved. Recording and key-press settings ride the same socket independently.
    function save() {
        if (pg.recordStatus === "ready" && pg.draft && pg.committed && pg.recordingDirtyCount > 0)
            pg.pendingSaveId = pg.sendCall("record.settings", { settings: pg.buildSettings(pg.draft) });
        if (pg.keyDraft && pg.keyCommitted && pg.keyDirtyCount > 0) {
            pg.pendingKeySaveId = pg.sendCall("keypress.settings", {
                theme: pg.keyDraft.theme,
                mode: pg.keyDraft.mode
            });
        }
    }

    function adoptMotion(text) {
        try {
            const settings = JSON.parse(text);
            pg.reduceMotion = settings.reduceMotion === true || settings.lowPowerMode === true;
        } catch (e) {
            pg.reduceMotion = false;
        }
    }

    Component.onCompleted: {
        if (pg.keyCommitted === null)
            pg.adoptKey("");
    }

    FileView {
        id: keyCfg
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/keypresses.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: pg.adoptKey(text())
        onLoadFailed: if (pg.keyCommitted === null) pg.adoptKey("")
    }

    FileView {
        id: performanceCfg
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/performance.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: pg.adoptMotion(text())
        onLoadFailed: pg.reduceMotion = false
    }

    function sendCall(method, args) {
        var payload = {};
        for (var k in args)
            payload[k] = args[k];
        payload.id = ++pg.callSerial;
        ctl.queued += "call " + method + " " + JSON.stringify(payload) + "\n";
        if (ctl.connected)
            ctl.flushQueued();
        else
            ctl.connected = true;
        return payload.id;
    }

    function applyKeyFrame(line) {
        try {
            const frame = JSON.parse(line);
            pg.keyBackendStatus = frame.status || "disabled";
            pg.keyBackendError = frame.error || "";
            pg.keyActive = pg.keyBackendStatus !== "disabled";
        } catch (e) {}
    }

    // one reply parser for the shared control socket: match the echoed id to the
    // in-flight read, recording save, or key-press save and route it. An id that
    // matches nothing (a fire-and-forget call like reset-placement) is dropped.
    function applyReply(line) {
        try {
            const reply = JSON.parse(line);
            var id = reply.id;
            if (id === pg.pendingReadId) {
                pg.pendingReadId = 0;
                pg.applyRecordRead(reply);
            } else if (id === pg.pendingSaveId) {
                pg.pendingSaveId = 0;
                pg.applyRecordSave(reply);
            } else if (id === pg.pendingKeySaveId) {
                pg.pendingKeySaveId = 0;
                pg.applyKeySave(reply);
            }
        } catch (e) {}
    }

    function applyRecordRead(reply) {
        if (!reply.ok) {
            pg.recordStatus = "unavailable";
            pg.recordError = reply.error || I18n.tr("Recording settings are unavailable.");
            return;
        }
        pg.adoptRecording(reply.result || {});
        pg.recordStatus = "ready";
        pg.recordError = "";
    }

    function applyRecordSave(reply) {
        if (!reply.ok) {
            pg.recordError = reply.error || I18n.tr("Could not save recording settings.");
            return;
        }
        pg.committed = pg.clone(pg.draft);
        pg.recordError = "";
    }

    function applyKeySave(reply) {
        if (!reply.ok) {
            pg.keySettingsError = reply.error || I18n.tr("Could not save key press settings.");
            return;
        }
        const saved = reply.result || {};
        pg.keyCommitted = {
            "theme": saved.theme === "light" ? "light" : "dark",
            "mode": saved.mode === "shortcuts" ? "shortcuts" : "all"
        };
        pg.keySettingsError = "";
    }

    function showKeyOverlay(show) {
        pg.keyActive = show;
        pg.keyPreviewRevision = Math.max(pg.keyPreviewRevision + 1, Date.now() * 1000);
        const revision = String(pg.keyPreviewRevision);
        if (show) {
            Spawn.run([
                "qs", "-c", "shell", "ipc", "call", "keypresses", "activate",
                pg.keyDraft ? pg.keyDraft.theme : "dark",
                pg.keyDraft ? pg.keyDraft.mode : "all",
                revision
            ]);
        } else {
            Spawn.run([
                "qs", "-c", "shell", "ipc", "call", "keypresses", "deactivate",
                revision
            ]);
        }
    }

    Socket {
        id: keySub
        path: pg.shellSockPath
        parser: SplitParser { onRead: line => pg.applyKeyFrame(line) }
        Component.onCompleted: connected = true
        onConnectionStateChanged: {
            if (connected) {
                write("subscribe keypress\n");
                flush();
            } else {
                keySubRetry.restart();
            }
        }
    }

    Timer {
        id: keySubRetry
        interval: 2000
        onTriggered: if (!keySub.connected) keySub.connected = true
    }

    Socket {
        id: ctl
        path: pg.shellSockPath
        property string queued: ""
        parser: SplitParser { onRead: line => pg.applyReply(line) }

        function flushQueued() {
            if (queued.length === 0)
                return;
            write(queued);
            flush();
            queued = "";
        }

        Component.onCompleted: connected = true
        onConnectionStateChanged: {
            if (connected) {
                pg.requestRecording();
                flushQueued();
            } else if (pg.recordStatus !== "ready") {
                pg.recordStatus = "unavailable";
            }
        }
    }

    // while the daemon is out of reach, keep trying: reconnect when the socket is
    // down, re-read when it is up but the page never got a good reply. Stops once
    // a read succeeds (recordStatus "ready").
    Timer {
        id: ctlRetry
        interval: 2000
        repeat: true
        running: pg.recordStatus !== "ready"
        onTriggered: {
            if (ctl.connected)
                pg.requestRecording();
            else {
                pg.recordStatus = "unavailable";
                ctl.connected = true;
            }
        }
    }

    // live readout: which backend + hardware encoder the recorder resolves for
    // this machine right now (the gsr probe is time-boxed, so first open can take
    // a moment). Parse failures leave the readout on "Detecting...".
    property string infoBackend: ""
    property string infoEncoder: ""
    property string infoVersion: ""
    property bool recorderProbeDone: false
    property bool recorderAvailable: false
    property string recorderUnavailableReason: ""
    property var supportedCodecs: ["h264", "hevc", "av1"]
    property var supportedEncoders: ["gpu", "cpu"]

    // Whether apps can actually be offered a source to pick. Screen sharing can
    // be entirely dead with nothing on screen to show for it, so ask the portal
    // and say so; the repair lives in ryoku doctor.
    property bool shareKnown: false
    property bool shareReady: false
    property string shareDetail: ""
    property bool repairing: false
    Process {
        id: shareInfo
        command: ["ryoku-hub", "share", "status"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var j = JSON.parse(this.text);
                    pg.shareKnown = j.probeable === true;
                    pg.shareReady = j.available === true;
                    pg.shareDetail = j.detail || "";
                } catch (e) {
                    pg.shareKnown = false;
                }
            }
        }
    }
    Process {
        id: shareRepair
        command: ["ryoku", "doctor"]
        stdout: StdioCollector { onStreamFinished: shareInfo.running = true }
        stderr: StdioCollector { onStreamFinished: shareInfo.running = true }
        onExited: pg.repairing = false
    }
    Process {
        id: info
        command: ["ryoku-cmd-record", "--info"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var j = JSON.parse(this.text);
                    pg.infoBackend = j.backend || "";
                    pg.infoEncoder = j.encoder || "";
                    pg.infoVersion = j.version || "";
                } catch (e) {}
            }
        }
    }
    Process {
        id: recorderProbe
        command: ["sh", "-c", "command -v gpu-screen-recorder >/dev/null && exec gpu-screen-recorder --info"]
        running: true
        stdout: StdioCollector { id: recorderProbeOut }
        stderr: StdioCollector {}
        onExited: code => {
            var codecs = [];
            var encoders = [];
            var lines = recorderProbeOut.text.split("\n").map(line => line.trim());
            function add(list, value) {
                if (list.indexOf(value) < 0)
                    list.push(value);
            }
            for (var i = 0; i < lines.length; ++i) {
                var value = lines[i];
                if (value === "h264" || value === "h264_software")
                    add(codecs, "h264");
                else if (value === "hevc")
                    add(codecs, "hevc");
                else if (value === "av1")
                    add(codecs, "av1");
                if (value === "h264_software")
                    add(encoders, "cpu");
                else if (value === "h264" || value === "hevc" || value === "av1"
                        || value === "vp8" || value === "vp9")
                    add(encoders, "gpu");
            }
            pg.supportedCodecs = codecs.length > 0 ? codecs : ["h264", "hevc", "av1"];
            pg.supportedEncoders = encoders.length > 0 ? encoders : ["gpu", "cpu"];
            pg.recorderAvailable = code === 0 && codecs.length > 0 && encoders.length > 0;
            pg.recorderUnavailableReason = pg.recorderAvailable ? ""
                : I18n.tr("GPU Screen Recorder is missing or no supported video encoder was detected.");
            pg.recorderProbeDone = true;
        }
    }

    // ── head: eyebrow, Fraunces title, intro blurb (matches every settings page) ──
    Column {
        id: head
        anchors.left: parent.left
        anchors.leftMargin: Tokens.s6
        anchors.top: parent.top
        anchors.topMargin: Tokens.s6
        // the head starts at the body's left inset and spans its width
        width: parent.width - Tokens.s6 * 2
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
                text: UiPreferences.brandMark("力"); color: Tokens.ink; font.family: Tokens.jp
                font.pixelSize: 11; anchors.verticalCenter: parent.verticalCenter
            }
            Text {
                text: I18n.tr("TOOLS"); color: Tokens.inkMuted; font.family: Tokens.ui
                font.pixelSize: 9; font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                anchors.verticalCenter: parent.verticalCenter
            }
        }
        Text {
            text: I18n.tr("Recording"); color: Tokens.ink
            font.family: Tokens.display; font.pixelSize: Tokens.fTitle
        }
        Text {
            width: Math.min(parent.width, 720)
            text: I18n.tr("How Ryoku records: quality, encoder, and where files land.")
            color: Tokens.inkMuted; font.family: Tokens.ui
            font.pixelSize: Tokens.fBody; wrapMode: Text.WordWrap
        }
    }

    // ── the scroll region: three sections, grouped by meaning ──
    Flickable {
        id: flick
        anchors {
            left: parent.left; right: parent.right
            top: head.bottom; bottom: bar.top
            leftMargin: Tokens.s6; rightMargin: Tokens.s6
            topMargin: Tokens.s5; bottomMargin: Tokens.s4
        }
        contentWidth: width
        contentHeight: Math.max(col.height, height)
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
        WheelScroll { }

        CardColumns {
            id: col
            // a body of cards fills the measure and splits into balanced columns
            width: flick.width - Tokens.s3
            spacing: Tokens.s5
            fillTo: flick.height

            SettingCard {
                width: col.colWidth
                title: I18n.tr("KEY PRESSES")
                kana: "鍵"

                Text {
                    width: parent.width
                    leftPadding: Tokens.s4; rightPadding: Tokens.s4
                    topPadding: Tokens.s3; bottomPadding: Tokens.s2
                    text: I18n.tr("Polished keycaps for tutorials and demos; turning this on never records.")
                    color: Tokens.inkMuted; font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap
                }

                Rectangle {
                    width: parent.width
                    height: 184
                    color: pg.keyDraft && pg.keyDraft.theme === "light" ? Tokens.keycapDark : Tokens.keycapLight
                    border.width: Tokens.border
                    border.color: Tokens.line
                    clip: true

                    Grain { anchors.fill: parent; opacity: 0.22 }

                    KeypressStack {
                        id: keyPreview
                        anchors.centerIn: parent
                        theme: pg.keyDraft ? pg.keyDraft.theme : "dark"
                        previewChords: pg.keyDraft && pg.keyDraft.mode === "shortcuts"
                            ? [["Super", "Shift", "R"], ["Ctrl", "K"], ["Alt", "Tab"]]
                            : [["A"], ["B"], ["C"]]
                        preview: true
                        motionEnabled: !pg.reduceMotion
                    }

                    Text {
                        anchors.left: parent.left; anchors.bottom: parent.bottom
                        anchors.leftMargin: Tokens.s4; anchors.bottomMargin: Tokens.s3
                        text: pg.keyDraft && pg.keyDraft.mode === "shortcuts"
                            ? I18n.tr("SHORTCUTS ONLY · SAFER FOR PASSWORDS")
                            : I18n.tr("ALL KEYS · BEST FOR TUTORIALS")
                        color: pg.keyDraft && pg.keyDraft.theme === "light" ? Tokens.keycapOnDark : Tokens.keycapOnLight
                        font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                        font.weight: Font.Medium; font.letterSpacing: Tokens.trackLabel
                    }
                }

                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    label: I18n.tr("Keycap style")
                    desc: I18n.tr("Dark keycaps, or light.")
                    source: "keypresses.json"
                    def: pg.keyCommitted ? pg.keyCommitted.theme : ""
                    changed: pg.keyDraft && pg.keyCommitted ? pg.keyDraft.theme !== pg.keyCommitted.theme : false
                    controlWidth: 132
                    Seg {
                        anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                        options: ["dark", "light"]
                        current: pg.keyDraft ? pg.keyDraft.theme : "dark"
                        onChose: k => pg.editKey("theme", k)
                    }
                }

                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    label: I18n.tr("Visible keys")
                    desc: I18n.tr("Every key, or only shortcuts.")
                    source: "keypresses.json"
                    def: pg.keyCommitted ? pg.keyCommitted.mode : ""
                    changed: pg.keyDraft && pg.keyCommitted ? pg.keyDraft.mode !== pg.keyCommitted.mode : false
                    controlWidth: 156
                    Seg {
                        anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                        options: ["all", "shortcuts"]
                        current: pg.keyDraft ? pg.keyDraft.mode : "all"
                        onChose: k => pg.editKey("mode", k)
                    }
                }

                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    label: I18n.tr("Desktop placement")
                    desc: pg.keySettingsError !== "" ? pg.keySettingsError
                        : (pg.keyBackendStatus === "error" ? pg.keyBackendError
                        : I18n.tr("Show the sample, drag the keycaps"))
                    controlWidth: 282
                    Row {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: Tokens.s2

                        Btn {
                            text: pg.keyActive ? I18n.tr("HIDE") : I18n.tr("SHOW ON DESKTOP")
                            primary: !pg.keyActive
                            armed: true
                            onAct: pg.showKeyOverlay(!pg.keyActive)
                        }
                        Btn {
                            text: I18n.tr("RESET POSITION")
                            armed: true
                            onAct: pg.sendCall("keypress.settings", { resetPlacement: true })
                        }
                    }
                }
            }

            // A recorder that can't be reached shows this instead of the setting
            // cards, so the page never presents factory defaults as the live
            // values or lets a save vanish into a dead socket.
            SettingCard {
                visible: pg.recordStatus !== "ready"
                width: col.colWidth
                title: I18n.tr("RECORDING")
                Text {
                    width: parent.width
                    leftPadding: Tokens.s4; rightPadding: Tokens.s4
                    topPadding: Tokens.s3; bottomPadding: Tokens.s3
                    wrapMode: Text.WordWrap
                    text: pg.recordStatus === "loading"
                        ? I18n.tr("Reading your recording settings\u2026")
                        : (pg.recordError !== "" ? pg.recordError
                           : I18n.tr("The shell isn't running, so recording settings can't be read or changed right now."))
                    color: Tokens.inkMuted; font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                }
            }

            SettingCard {
                visible: pg.recordStatus === "ready" && pg.recorderProbeDone && !pg.recorderAvailable
                width: col.colWidth
                title: I18n.tr("RECORDING UNAVAILABLE")
                Text {
                    width: parent.width
                    leftPadding: Tokens.s4; rightPadding: Tokens.s4
                    topPadding: Tokens.s3; bottomPadding: Tokens.s3
                    wrapMode: Text.WordWrap
                    text: pg.recorderUnavailableReason
                    color: Tokens.inkMuted; font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                }
            }

            // ── QUALITY ──────────────────────────────────────────────────────
            SettingCard {
                visible: pg.recordStatus === "ready"
                enabled: pg.recorderAvailable
                opacity: enabled ? 1 : 0.45
                width: col.colWidth
                title: I18n.tr("QUALITY")
                Text {
                    width: parent.width
                    leftPadding: Tokens.s4; rightPadding: Tokens.s4
                    topPadding: Tokens.s3; bottomPadding: Tokens.s1
                    text: I18n.tr("Higher framerate and quality look better but make larger files.")
                    color: Tokens.inkMuted; font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap
                }
                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    label: I18n.tr("Framerate")
                    desc: I18n.tr("Frames per second; higher is smoother but larger.")
                    unit: "fps"
                    source: "recording.json"
                    value: pg.draft ? String(pg.draft.fps) : ""
                    def: pg.committed ? String(pg.committed.fps) : ""
                    changed: pg.draft && pg.committed ? pg.draft.fps !== pg.committed.fps : false
                    editableValue: true
                    onValueCommitted: (t) => pg.editNum("fps", t, 1, 360)
                    controlWidth: 58
                    Step {
                        anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                        value: pg.draft ? (Number(pg.draft.fps) || 60) : 60
                        from: 1; to: 360; stepBy: 5
                        onModified: (v) => pg.edit("fps", v)
                    }
                }
                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    label: I18n.tr("Framerate mode")
                    desc: I18n.tr("Constant plays anywhere; variable is smaller.")
                    source: "recording.json"
                    def: pg.committed ? String(pg.committed.framerateMode) : ""
                    changed: pg.draft && pg.committed ? pg.draft.framerateMode !== pg.committed.framerateMode : false
                    controlWidth: 124
                    Seg {
                        anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                        options: ["cfr", "vfr"]
                        current: pg.draft ? String(pg.draft.framerateMode) : ""
                        onChose: (k) => pg.edit("framerateMode", k)
                    }
                }
                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    block: true
                    label: I18n.tr("Quality")
                    desc: I18n.tr("Higher settings look crisper but make larger files.")
                    source: "recording.json"
                    def: pg.committed ? String(pg.committed.quality) : ""
                    changed: pg.draft && pg.committed ? pg.draft.quality !== pg.committed.quality : false
                    Seg {
                        anchors.left: parent.left; anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        options: ["medium", "high", "very_high", "ultra"]
                        current: pg.draft ? String(pg.draft.quality) : ""
                        onChose: (k) => pg.edit("quality", k)
                    }
                }
                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    label: I18n.tr("Rate control")
                    desc: I18n.tr("Quality targets a look; Constant pins a fixed bitrate.")
                    source: "recording.json"
                    def: pg.committed ? pg.bitrateModeLabel(pg.committed.bitrateMode) : ""
                    changed: pg.draft && pg.committed ? pg.draft.bitrateMode !== pg.committed.bitrateMode : false
                    controlWidth: 168
                    Seg {
                        anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                        options: ["Quality", "Constant"]
                        current: pg.draft ? pg.bitrateModeLabel(pg.draft.bitrateMode) : ""
                        onChose: (label) => pg.edit("bitrateMode", pg.bitrateModeKey(label))
                    }
                }
                SettingRow {
                    visible: pg.draft ? pg.draft.bitrateMode === "cbr" : false
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    label: I18n.tr("Bitrate")
                    desc: I18n.tr("The fixed data rate used in Constant mode.")
                    unit: "kbps"
                    source: "recording.json"
                    value: pg.draft ? String(pg.draft.bitrate) : ""
                    def: pg.committed ? String(pg.committed.bitrate) : ""
                    changed: pg.draft && pg.committed ? pg.draft.bitrate !== pg.committed.bitrate : false
                    editableValue: true
                    onValueCommitted: (t) => pg.editNum("bitrate", t, 500, 200000)
                    controlWidth: 58
                    Step {
                        anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                        value: pg.draft ? (Number(pg.draft.bitrate) || 20000) : 20000
                        from: 500; to: 200000; stepBy: 1000
                        onModified: (v) => pg.edit("bitrate", v)
                    }
                }
                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    block: true
                    label: I18n.tr("Maximum resolution")
                    desc: I18n.tr("Scale the capture down to save space; Native keeps full size.")
                    source: "recording.json"
                    def: pg.committed ? pg.maxResLabel(pg.committed.maxResolution) : ""
                    changed: pg.draft && pg.committed ? pg.draft.maxResolution !== pg.committed.maxResolution : false
                    Seg {
                        anchors.left: parent.left; anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        options: ["Native", "1080p", "1440p", "4K"]
                        current: pg.draft ? pg.maxResLabel(pg.draft.maxResolution) : ""
                        onChose: (label) => pg.edit("maxResolution", pg.maxResKey(label))
                    }
                }
            }

            // ── FILE ─────────────────────────────────────────────────────────
            SettingCard {
                visible: pg.recordStatus === "ready"
                enabled: pg.recorderAvailable
                opacity: enabled ? 1 : 0.45
                width: col.colWidth
                title: I18n.tr("FILE")
                Text {
                    width: parent.width
                    leftPadding: Tokens.s4; rightPadding: Tokens.s4
                    topPadding: Tokens.s3; bottomPadding: Tokens.s1
                    text: I18n.tr("The wrapper format and where finished recordings land.")
                    color: Tokens.inkMuted; font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap
                }
                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    label: I18n.tr("Container")
                    desc: I18n.tr("MP4 with H.264 plays in browsers and Discord; MKV and WebM are pickier.")
                    source: "recording.json"
                    def: pg.committed ? String(pg.committed.container) : ""
                    changed: pg.draft && pg.committed ? pg.draft.container !== pg.committed.container : false
                    controlWidth: 168
                    Seg {
                        anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                        options: ["mp4", "mkv", "webm"]
                        current: pg.draft ? String(pg.draft.container) : ""
                        onChose: (k) => pg.edit("container", k)
                    }
                }
                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    footH: 32
                    label: I18n.tr("Save recordings to")
                    desc: I18n.tr("Leave empty to follow your Videos folder.")
                    source: "recording.json"
                    changed: pg.draft && pg.committed ? pg.draft.directory !== pg.committed.directory : false
                    Field {
                        anchors.left: parent.left; anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        tabular: true
                        placeholder: pg.defaultDir
                        text: pg.draft ? String(pg.draft.directory) : ""
                        onCommitted: (v) => pg.edit("directory", v.trim())
                    }
                }
            }

            // ── ENCODER ──────────────────────────────────────────────────────
            SettingCard {
                visible: pg.recordStatus === "ready"
                enabled: pg.recorderAvailable
                opacity: enabled ? 1 : 0.45
                width: col.colWidth
                title: I18n.tr("ENCODER")
                Text {
                    width: parent.width
                    leftPadding: Tokens.s4; rightPadding: Tokens.s4
                    topPadding: Tokens.s3; bottomPadding: Tokens.s1
                    text: I18n.tr("GPU encoding is fast; CPU is the fallback if it misbehaves.")
                    color: Tokens.inkMuted; font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap
                }
                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    block: true
                    label: I18n.tr("Codec")
                    desc: I18n.tr("H.264 plays anywhere; HEVC and AV1 are smaller but need a newer GPU.")
                    source: "recording.json"
                    def: pg.committed ? String(pg.committed.codec) : ""
                    changed: pg.draft && pg.committed ? pg.draft.codec !== pg.committed.codec : false
                    Seg {
                        anchors.left: parent.left; anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        options: pg.supportedCodecs
                        current: pg.draft ? String(pg.draft.codec) : ""
                        onChose: (k) => pg.edit("codec", k)
                    }
                }
                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    label: I18n.tr("Encoder")
                    desc: I18n.tr("GPU offloads the CPU; CPU if GPU fails.")
                    source: "recording.json"
                    def: pg.committed ? String(pg.committed.encoder) : ""
                    changed: pg.draft && pg.committed ? pg.draft.encoder !== pg.committed.encoder : false
                    controlWidth: 124
                    Seg {
                        anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                        options: pg.supportedEncoders
                        current: pg.draft ? String(pg.draft.encoder) : ""
                        onChose: (k) => pg.edit("encoder", k)
                    }
                }
                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    label: I18n.tr("Show the cursor")
                    desc: I18n.tr("Draws the mouse pointer into the video.")
                    source: "recording.json"
                    changed: pg.draft && pg.committed ? pg.draft.cursor !== pg.committed.cursor : false
                    controlWidth: 54
                    Sw {
                        anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                        on: pg.draft ? !!pg.draft.cursor : false
                        onToggled: (v) => pg.edit("cursor", v)
                    }
                }
                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    label: I18n.tr("Color range")
                    desc: I18n.tr("Limited matches most players; Full is richer but can look washed out.")
                    source: "recording.json"
                    def: pg.committed ? String(pg.committed.colorRange) : ""
                    changed: pg.draft && pg.committed ? pg.draft.colorRange !== pg.committed.colorRange : false
                    controlWidth: 124
                    Seg {
                        anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                        options: ["limited", "full"]
                        current: pg.draft ? String(pg.draft.colorRange) : ""
                        onChose: (k) => pg.edit("colorRange", k)
                    }
                }
                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    label: I18n.tr("Keyframe interval")
                    desc: I18n.tr("Seconds between keyframes; lower seeks smoother but grows the file.")
                    unit: "s"
                    source: "recording.json"
                    value: pg.draft ? String(pg.draft.keyint) : ""
                    def: pg.committed ? String(pg.committed.keyint) : ""
                    changed: pg.draft && pg.committed ? pg.draft.keyint !== pg.committed.keyint : false
                    editableValue: true
                    onValueCommitted: (t) => pg.editNum("keyint", t, 1, 600)
                    controlWidth: 58
                    Step {
                        anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                        value: pg.draft ? (Number(pg.draft.keyint) || 2) : 2
                        from: 1; to: 600; stepBy: 1
                        onModified: (v) => pg.edit("keyint", v)
                    }
                }
            }

            // ── AUDIO ────────────────────────────────────────────────────────
            SettingCard {
                visible: pg.recordStatus === "ready"
                enabled: pg.recorderAvailable
                opacity: enabled ? 1 : 0.45
                width: col.colWidth
                title: I18n.tr("AUDIO")
                Text {
                    width: parent.width
                    leftPadding: Tokens.s4; rightPadding: Tokens.s4
                    topPadding: Tokens.s3; bottomPadding: Tokens.s1
                    text: I18n.tr("The codec and bitrate for captured sound.")
                    color: Tokens.inkMuted; font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap
                }
                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    label: I18n.tr("Audio codec")
                    desc: I18n.tr("Opus sounds better at low bitrates; AAC plays in more editors.")
                    source: "recording.json"
                    def: pg.committed ? String(pg.committed.audioCodec) : ""
                    changed: pg.draft && pg.committed ? pg.draft.audioCodec !== pg.committed.audioCodec : false
                    controlWidth: 124
                    Seg {
                        anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                        options: ["opus", "aac"]
                        current: pg.draft ? String(pg.draft.audioCodec) : ""
                        onChose: (k) => pg.edit("audioCodec", k)
                    }
                }
                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    label: I18n.tr("Audio bitrate")
                    desc: I18n.tr("Leave at Auto to let the codec choose.")
                    unit: pg.draft && pg.draft.audioBitrate !== 0 ? "kbps" : ""
                    source: "recording.json"
                    value: pg.draft ? (pg.draft.audioBitrate === 0 ? I18n.tr("Auto") : String(pg.draft.audioBitrate)) : ""
                    def: pg.committed ? (pg.committed.audioBitrate === 0 ? I18n.tr("Auto") : String(pg.committed.audioBitrate)) : ""
                    changed: pg.draft && pg.committed ? pg.draft.audioBitrate !== pg.committed.audioBitrate : false
                    editableValue: true
                    onValueCommitted: (t) => pg.editNum("audioBitrate", t, 0, 512)
                    controlWidth: 58
                    Step {
                        anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                        value: pg.draft ? (Number(pg.draft.audioBitrate) || 0) : 0
                        from: 0; to: 512; stepBy: 16
                        onModified: (v) => pg.edit("audioBitrate", v)
                    }
                }
            }

            // Screen sharing can fail with nothing on screen to show for it: an
            // app simply never gets a picker. Say so here, and offer the one
            // command that can fix it. Hidden when there is no session bus to ask.
            SettingCard {
                visible: pg.shareKnown
                width: col.colWidth
                title: I18n.tr("SCREEN PICKER")
                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    label: pg.shareReady ? I18n.tr("Picker ready") : I18n.tr("No source picker")
                    desc: pg.shareDetail
                    controlWidth: 96
                    Btn {
                        anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                        visible: !pg.shareReady
                        text: pg.repairing ? I18n.tr("REPAIRING") : I18n.tr("REPAIR")
                        armed: true
                        enabled: !pg.repairing
                        onAct: {
                            pg.repairing = true;
                            shareRepair.running = true;
                        }
                    }
                }
            }

            SettingCard {
                width: col.colWidth
                title: I18n.tr("UNDER THE HOOD")
                expanded: false
                summary: I18n.tr("AUTO-DETECTED")
                Text {
                    width: parent.width
                    leftPadding: Tokens.s4; rightPadding: Tokens.s4
                    topPadding: Tokens.s3; bottomPadding: Tokens.s2
                    wrapMode: Text.WordWrap
                    text: pg.infoBackend === ""
                        ? I18n.tr("Detecting\u2026")
                        : (I18n.tr("Backend    ") + (pg.infoBackend === "gsr" ? I18n.tr("GPU Screen Recorder") : pg.infoBackend)
                           + (pg.infoVersion !== "" ? "  " + pg.infoVersion : "")
                           + I18n.tr("\nEncoder    ") + pg.infoEncoder
                           + (pg.draft
                              ? (I18n.tr("\nContainer  ") + String(pg.draft.container).toUpperCase()
                                 + "  \u00b7  " + pg.draft.fps + "fps " + String(pg.draft.framerateMode).toUpperCase()
                                 + "  \u00b7  " + pg.draft.codec + "  \u00b7  " + pg.draft.quality)
                              : ""))
                    color: Tokens.inkMuted
                    font.family: Tokens.mono
                    font.pixelSize: 12
                    lineHeight: 1.5
                }
            }
        }
    }

    // ── action bar: dirty status left, Reset / Revert / Save right ──
    // full-bleed hides the shell's global bar, so this is the only way to persist.
    // RESET walks every key to stock (creating dirt), REVERT drops the unsaved
    // draft, SAVE writes it to recording.json (env vars still override at record
    // time). Nothing here reaches hardware.
    Rectangle {
        id: bar
        anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
        height: 60
        color: "transparent"

        // hairline lid, like the shell's action bar (DESIGN.md section 8).
        Rectangle {
            anchors { left: parent.left; right: parent.right; top: parent.top }
            height: 1; color: Tokens.line
        }

        Row {
            anchors.left: parent.left
            anchors.leftMargin: Tokens.s6
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s3

            // filled ink while dirty, with a 600/600 heartbeat -- the one
            // perpetual animation allowed on an app surface; a hairline dot clean.
            Rectangle {
                id: dot
                anchors.verticalCenter: parent.verticalCenter
                width: 6; height: 6; radius: 3
                antialiasing: false
                readonly property bool lit: pg.dirtyCount > 0
                color: lit ? Tokens.ink : "transparent"
                border.width: lit ? 0 : Tokens.border
                border.color: Tokens.inkFaint

                SequentialAnimation on opacity {
                    running: pg.dirtyCount > 0
                    loops: Animation.Infinite
                    NumberAnimation { to: 0.3; duration: 600; easing.type: Easing.InOutSine }
                    NumberAnimation { to: 1.0; duration: 600; easing.type: Easing.InOutSine }
                    onStopped: dot.opacity = 1
                }
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: pg.dirtyCount > 0
                    ? (pg.dirtyCount === 1
                        ? I18n.tr("%1 CHANGE \u00b7 PREVIEWING \u00b7 NOT SAVED").arg(pg.dirtyCount)
                        : I18n.tr("%1 CHANGES \u00b7 PREVIEWING \u00b7 NOT SAVED").arg(pg.dirtyCount))
                    : I18n.tr("SAVED \u00b7 LIVE ON YOUR DESKTOP")
                color: pg.dirtyCount > 0 ? Tokens.ink : Tokens.inkMuted
                font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                font.weight: Font.Medium; font.letterSpacing: Tokens.trackLabel
                font.capitalization: Font.AllUppercase
            }
        }

        Row {
            anchors.right: parent.right
            anchors.rightMargin: Tokens.s6
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s3

            Btn {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("RESET TO DEFAULTS")
                armed: pg.offDefaults
                onAct: pg.reset()
            }
            Rectangle {
                anchors.verticalCenter: parent.verticalCenter
                width: 1; height: 20; color: Tokens.line
            }
            Btn {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("REVERT")
                armed: pg.dirtyCount > 0
                onAct: pg.revert()
            }
            Btn {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("SAVE")
                primary: true
                armed: pg.dirtyCount > 0
                onAct: pg.save()
            }
        }
    }
}
