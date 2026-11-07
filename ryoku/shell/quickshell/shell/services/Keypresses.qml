pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import Ryoku.Ui

// Session-only activation and daemon-backed key events for the recording
// companion. Visual and placement settings persist in keypresses.json through
// the daemon's single-writer IPC seam; raw key codes and typed text never do.
Singleton {
    id: root

    readonly property string cfgPath: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/keypresses.json"

    property bool active: false
    property string theme: "dark"
    property string mode: "all"
    property string persistedTheme: "dark"
    property string persistedMode: "all"
    property real px: NaN
    property real py: NaN
    property string monitor: ""
    property string sessionMonitor: ""
    property bool loaded: false
    property string backendStatus: "disabled"
    property string backendError: ""
    property string lastEventSignature: ""
    property bool sawRecording: false
    property real previewRevision: 0
    property bool placementPreview: false

    readonly property bool recordingActive: Recorder.active

    signal chord(var keys, bool repeat, string state, real timestamp)

    function applySettings(text) {
        const settings = KeypressMath.parseSettings(text);
        root.persistedTheme = settings.theme;
        root.persistedMode = settings.mode;
        if (!root.active) {
            root.theme = settings.theme;
            root.mode = settings.mode;
        }
        root.px = settings.px === null ? NaN : settings.px;
        root.py = settings.py === null ? NaN : settings.py;
        root.monitor = settings.monitor;
        root.loaded = true;
    }

    function applyFrame(frame) {
        if (!frame || typeof frame !== "object")
            return;
        root.backendStatus = frame.status || "disabled";
        root.backendError = frame.error || "";
        if (!Array.isArray(frame.keys) || frame.keys.length === 0)
            return;
        const state = frame.state === "pressed" || frame.state === "released"
            ? frame.state : "tap";
        const signature = frame.serial ? String(frame.serial)
            : JSON.stringify([frame.time || 0, frame.repeat === true, state, frame.keys]);
        if (signature === root.lastEventSignature)
            return;
        root.lastEventSignature = signature;
        root.chord(frame.keys, frame.repeat === true, state, frame.time || Date.now());
    }

    function send(method, args) {
        Bus.call(method, args);
    }


    function monitorAvailable(name) {
        return Wm.outputByName(name) !== null;
    }

    function chooseMonitor() {
        if (root.monitor !== "" && root.monitorAvailable(root.monitor))
            return root.monitor;
        if (Wm.focusedOutput)
            return Wm.focusedOutput;
        return Wm.outputs.length > 0 ? Wm.outputs[0].name : "";
    }
    function sendConfigure() {
        root.send("keypress.configure", { enabled: root.active, mode: root.mode });
    }


    function nextPreviewRevision() {
        root.previewRevision = Math.max(root.previewRevision + 1, Date.now() * 1000);
    }

    function fencePreviewCommands() {
        root.previewRevision = Math.max(root.previewRevision + 1, Date.now() * 1000 + 999);
    }

    function toggle() {
        root.nextPreviewRevision();
        root.placementPreview = false;
        root.active = !root.active;
    }

    function previewVisual(nextTheme, nextMode) {
        root.theme = nextTheme === "light" ? "light" : "dark";
        root.mode = nextMode === "shortcuts" ? "shortcuts" : "all";
    }

    function acceptPreviewRevision(value) {
        const next = Number(value);
        if (!isFinite(next) || next <= root.previewRevision)
            return false;
        root.previewRevision = next;
        return true;
    }

    function activatePreview(nextTheme, nextMode, revision) {
        if (!root.acceptPreviewRevision(revision))
            return;
        root.previewVisual(nextTheme, nextMode);
        root.placementPreview = true;
        root.active = true;
    }

    function deactivatePreview(revision) {
        if (!root.acceptPreviewRevision(revision))
            return;
        root.placementPreview = false;
        root.active = false;
    }

    function restoreVisual() {
        root.theme = root.persistedTheme;
        root.mode = root.persistedMode;
    }

    function savePlacement(x, y, monitorName) {
        root.px = x;
        root.py = y;
        root.monitor = monitorName || "";
        root.send("keypress.settings", { px: x, py: y, monitor: root.monitor });
        root.sessionMonitor = root.monitor;
    }

    function resetPlacement() {
        root.px = NaN;
        root.py = NaN;
        root.monitor = "";
        root.send("keypress.settings", { resetPlacement: true });
    }

    onActiveChanged: {
        if (active) {
            root.sessionMonitor = root.chooseMonitor();
            root.lastEventSignature = "";
        } else {
            root.placementPreview = false;
            root.sessionMonitor = "";
            root.restoreVisual();
        }
        root.sendConfigure();
    }
    onModeChanged: if (loaded && active) sendConfigure()

    Connections {
        target: Recorder
        function onActiveChanged() {
            if (Recorder.active) {
                root.placementPreview = false;
                root.sawRecording = true;
            } else if (root.sawRecording) {
                root.sawRecording = false;
                root.fencePreviewCommands();
                root.active = false;
            }
        }
    }

    FileView {
        id: settingsFile
        path: root.cfgPath
        blockLoading: true
        watchChanges: true
        printErrors: false
        onLoaded: root.applySettings(text())
        onFileChanged: reload()
        onLoadFailed: if (!root.loaded) root.applySettings("")
    }

    BusTopic {
        id: topicFeed
        topic: "keypress"
        onUpdated: root.applyFrame(topicFeed.data)
    }

    // The capture runs in the daemon only while it is told to; a restarted
    // daemon hears the current activation again.
    Connections {
        target: Bus
        function onConnectedChanged() {
            if (Bus.connected)
                root.sendConfigure();
        }
    }

    Component.onCompleted: if (Bus.connected) root.sendConfigure()
}
