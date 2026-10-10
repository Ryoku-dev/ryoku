pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import inir.modules.common

Singleton {
    id: root

    property bool isRecording: false
    property bool available: true
    property int recorderPid: 0
    property string requestedAudioMode: "none"
    property string activeAudioMode: "none"
    property bool audioFallback: false
    property bool hasAudioMetadata: false
    property bool hasStoredAudioMode: false
    property bool hasStoredSystemAudioSource: false
    property bool hasStoredMicrophoneSource: false
    property string legacyAudioSource: ""
    readonly property string configuredAudioMode: root.hasStoredAudioMode
        ? root.normalizeAudioMode(Config.options?.screenRecord?.audioMode ?? "system")
        : root.audioModeFromLegacySource(root.legacyAudioSource)
    readonly property string configuredSystemAudioSource: {
        const configured = String(Config.options?.screenRecord?.systemAudioSource ?? "")
        if (root.hasStoredSystemAudioSource)
            return configured
        return root.legacyAudioSource.endsWith(".monitor") ? root.legacyAudioSource : configured
    }
    readonly property string configuredMicrophoneSource: {
        const configured = String(Config.options?.screenRecord?.microphoneSource ?? "")
        if (root.hasStoredMicrophoneSource)
            return configured
        return root.legacyAudioSource.length > 0 && !root.legacyAudioSource.endsWith(".monitor")
            ? root.legacyAudioSource : configured
    }
    readonly property string effectiveAudioMode: isRecording && hasAudioMetadata ? activeAudioMode : configuredAudioMode
    readonly property string recorderStatusPath: (Quickshell.env("XDG_STATE_HOME") || (Quickshell.env("HOME") + "/.local/state")) + "/ryoku/recorder-status.json"
    // Timestamp (ms since epoch) when recording started, 0 when not recording
    property real recordingStartTime: 0
    // Elapsed seconds since recording started, updated every second
    property int elapsedSeconds: 0

    function normalizeAudioMode(mode: string): string {
        switch (mode) {
        case "none":
        case "system":
        case "microphone":
        case "both":
            return mode
        default:
            return "system"
        }
    }

    function audioModeFromLegacySource(source: string): string {
        if (source.length > 0 && !source.endsWith(".monitor"))
            return "microphone"
        return "system"
    }

    function setConfiguredAudioMode(mode: string): void {
        Config.setNestedValue("screenRecord.audioMode", root.normalizeAudioMode(mode))
        root.hasStoredAudioMode = true
    }

    function setConfiguredSystemAudioSource(source): void {
        Config.setNestedValue("screenRecord.systemAudioSource", String(source ?? ""))
        root.hasStoredSystemAudioSource = true
    }

    function setConfiguredMicrophoneSource(source): void {
        Config.setNestedValue("screenRecord.microphoneSource", String(source ?? ""))
        root.hasStoredMicrophoneSource = true
    }

    function resetStoredAudioConfig(): void {
        root.hasStoredAudioMode = false
        root.hasStoredSystemAudioSource = false
        root.hasStoredMicrophoneSource = false
        root.legacyAudioSource = ""
    }

    function parseStoredAudioConfig(payloadText: string): void {
        try {
            const payload = JSON.parse(payloadText.trim() || "{}")
            const screenRecord = payload?.screenRecord
            if (screenRecord === null || typeof screenRecord !== "object" || Array.isArray(screenRecord)) {
                root.resetStoredAudioConfig()
                return
            }
            root.hasStoredAudioMode = Object.prototype.hasOwnProperty.call(screenRecord, "audioMode")
            root.hasStoredSystemAudioSource = Object.prototype.hasOwnProperty.call(screenRecord, "systemAudioSource")
            root.hasStoredMicrophoneSource = Object.prototype.hasOwnProperty.call(screenRecord, "microphoneSource")
            root.legacyAudioSource = typeof screenRecord.audioSource === "string"
                ? screenRecord.audioSource : ""
        } catch (error) {
            root.resetStoredAudioConfig()
        }
    }

    function refreshStoredAudioConfig(): void {
        if (Config.ready && !storedConfigProcess.running)
            storedConfigProcess.running = true
    }

    function resetAudioMetadata(): void {
        requestedAudioMode = "none"
        activeAudioMode = "none"
        audioFallback = false
        hasAudioMetadata = false
    }

    // The backend's status file exists exactly while a capture runs and carries
    // its launch time: reading it on change shows a start at once, from any entry
    // point, with the clock counting from the real launch rather than first sight.
    function applyStatusFile(): void {
        if (!root.available) {
            root.isRecording = false
            root.resetAudioMetadata()
            return
        }
        let payload
        try {
            payload = JSON.parse(statusFile.text() || "{}")
        } catch (error) {
            payload = {}
        }
        const payloadPid = Number(payload.recorderPid ?? 0)
        if (payloadPid <= 0) {
            root.resetAudioMetadata()
            return
        }
        const launched = Number(payload.startedAt ?? 0) * 1000
        if (launched > 0)
            root.recordingStartTime = launched
        if (!root.isRecording)
            root.isRecording = true
        if (root.recordingStartTime > 0)
            root.elapsedSeconds = Math.floor((Date.now() - root.recordingStartTime) / 1000)
        root.recorderPid = payloadPid
        root.requestedAudioMode = root.normalizeAudioMode(String(payload.requestedAudioMode ?? "system"))
        root.activeAudioMode = root.normalizeAudioMode(String(payload.activeAudioMode ?? "none"))
        root.audioFallback = payload.audioFallback === true
        root.hasAudioMetadata = true
    }

    FileView {
        id: statusFile
        path: root.recorderStatusPath
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: root.applyStatusFile()
        onLoadFailed: {
            if (root.isRecording)
                root.isRecording = false
            root.refreshStatus()
        }
    }

    onIsRecordingChanged: {
        if (isRecording) {
            if (recordingStartTime <= 0)
                recordingStartTime = Date.now()
            elapsedSeconds = Math.floor((Date.now() - recordingStartTime) / 1000)
        } else {
            recordingStartTime = 0
            elapsedSeconds = 0
            recorderPid = 0
            resetAudioMetadata()
        }
    }

    function refreshStatus() {
        if (root.available && !checkProcess.running)
            checkProcess.running = true
    }

    // Idle poll: a backstop for a stale status file; the file watch notices a start.
    Timer {
        id: idlePollTimer
        interval: 5000
        running: root.available && Config.ready && !root.isRecording
        repeat: true
        onTriggered: root.refreshStatus()
    }

    // Active poll: 1s tick while recording (elapsed counter + stop detection)
    Timer {
        id: activePollTimer
        interval: 1000
        running: root.available && root.isRecording
        repeat: true
        onTriggered: {
            if (root.recordingStartTime > 0)
                root.elapsedSeconds = Math.floor((Date.now() - root.recordingStartTime) / 1000)
            root.refreshStatus()
        }
    }

    // Quick recheck after a recording action (start/stop) to catch state change fast
    function scheduleQuickCheck(): void {
        if (!root.available)
            return
        quickCheckTimer.attemptsRemaining = 6
        quickCheckTimer.restart()
    }
    Timer {
        id: quickCheckTimer
        property int attemptsRemaining: 0
        interval: 350
        repeat: true
        onTriggered: {
            root.refreshStatus()
            attemptsRemaining = Math.max(0, attemptsRemaining - 1)
            if (root.isRecording || attemptsRemaining <= 0)
                stop()
        }
    }

    Connections {
        target: Config
        function onReadyChanged(): void {
            if (Config.ready)
                Qt.callLater(root.refreshStoredAudioConfig)
        }
    }

    Component.onCompleted: {
        Qt.callLater(root.refreshStatus)
        Qt.callLater(root.refreshStoredAudioConfig)
    }

    Process {
        id: storedConfigProcess
        command: ["/usr/bin/cat", Config.filePath]
        stdout: StdioCollector {
            id: storedConfigCollector
        }
        stderr: StdioCollector {}
        onExited: (exitCode, exitStatus) => {
            if (exitCode === 0)
                root.parseStoredAudioConfig(storedConfigCollector.text)
            else
                root.resetStoredAudioConfig()
        }
    }

    // GSR's IPC socket backs the status file up (a backend killed outright leaves
    // a stale file). The socket comes up a moment after launch, so a negative
    // answer inside the first seconds of a capture is the socket warming up.
    readonly property int socketWarmupMs: 5000
    Process {
        id: checkProcess
        command: ["/bin/sh", "-c",
            "command -v gsr-cli >/dev/null 2>&1 || exit 127; "
            + "exec gsr-cli -ipc \"$1\" status",
            "ryoku-gsr-status",
            (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-gsr.sock"]
        onExited: (exitCode, exitStatus) => {
            if (exitCode === 127) {
                root.available = false
                root.isRecording = false
                return
            }
            const warming = root.isRecording && root.recordingStartTime > 0
                && Date.now() - root.recordingStartTime < root.socketWarmupMs
            root.isRecording = exitCode === 0 || warming
            if (root.isRecording && !root.hasAudioMetadata)
                statusFile.reload()
        }
    }
}
