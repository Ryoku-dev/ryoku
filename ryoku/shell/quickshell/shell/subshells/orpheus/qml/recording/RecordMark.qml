pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import "../kit" as K
import "Clock.js" as Clock
import "Audio.js" as Audio

// keeps the last phase while the bubble melts away, so a capture never ends on a reset face
Item {
    id: root

    required property var scene
    readonly property bool hintsSuppressed: scene.cardKey !== "" || scene.settingsOpen

    readonly property string livePhase: String(Recorder.phase || "idle")
    readonly property bool live: livePhase !== "idle"
    readonly property bool capturing: ["starting", "recording", "paused", "stopping"]
        .indexOf(livePhase) >= 0

    property string phase: "idle"
    property int seconds: 0
    property int count: 0
    property var audio: ({ system: "off", mic: "off" })
    property string resultText: ""
    property real countdownLeft: 1

    readonly property bool hasSystem: audio.system !== "off"
    readonly property bool hasMic: audio.mic !== "off"
    readonly property string timeText: phase === "arming" ? String(count)
        : phase === "failed" && seconds <= 0 ? "--:--" : Clock.format(seconds)
    readonly property bool settled: phase === "saved" || phase === "failed"
    readonly property string hintText: {
        switch (phase) {
        case "arming": return "Recording in " + count + ", right-click to cancel";
        case "starting": return "Starting the recording";
        case "recording": return "Recording, right-click to stop";
        case "paused": return "Paused, right-click to stop";
        case "stopping": return "Saving the recording";
        case "saved": return resultText.length > 0 ? "Saved " + resultText : "Recording saved";
        case "failed": return resultText.length > 0 ? "Recording failed: " + resultText : "Recording failed";
        default: return "";
        }
    }

    implicitWidth: content.implicitWidth + 2 * K.Style.hitPad
    implicitHeight: K.Style.barH
    width: implicitWidth
    height: K.Style.barH

    function fileName(path) {
        const value = String(path || "");
        return value.substring(value.lastIndexOf("/") + 1);
    }

    function restartCountdown() {
        countdownSweep.stop();
        root.countdownLeft = 1;
        if (root.livePhase === "arming" && !K.Style.still)
            countdownSweep.start();
    }

    // these follow the recorder only while a capture is live and hold their last value after
    Binding {
        target: root
        property: "phase"
        value: root.livePhase
        when: root.live
        restoreMode: Binding.RestoreNone
    }
    Binding {
        target: root
        property: "seconds"
        value: Recorder.elapsedSec
        when: root.capturing || (root.live && Recorder.elapsedSec > 0)
        restoreMode: Binding.RestoreNone
    }
    Binding {
        target: root
        property: "count"
        value: Recorder.countdownSec
        when: root.livePhase === "arming"
        restoreMode: Binding.RestoreNone
    }
    // held through saved and failed too, so the bubble keeps one width to the end
    Binding {
        target: root
        property: "audio"
        value: Audio.sources(root.livePhase, Recorder.activeAudioMode,
            Recorder.audioFallback === true, Recorder.optDesktopAudio, Recorder.optMic)
        when: root.capturing || root.livePhase === "arming"
        restoreMode: Binding.RestoreNone
    }
    Binding {
        target: root
        property: "resultText"
        value: root.livePhase === "failed"
            ? String(Recorder.lastResult?.reason || "")
            : root.fileName(Recorder.lastResult?.path)
        when: root.livePhase === "saved" || root.livePhase === "failed"
        restoreMode: Binding.RestoreNone
    }

    onLivePhaseChanged: if (livePhase === "arming") restartCountdown()
    Component.onCompleted: restartCountdown()

    Connections {
        target: Recorder
        function onCountdownSecChanged() { root.restartCountdown(); }
    }

    NumberAnimation {
        id: countdownSweep
        target: root
        property: "countdownLeft"
        from: 1
        to: 0
        duration: 1000
    }

    Row {
        id: content
        anchors.centerIn: parent
        spacing: Math.round(6 * K.Style.scale)

        PhaseMark {
            anchors.verticalCenter: parent.verticalCenter
            phase: root.phase
            countdown: root.countdownLeft
        }

        RollTime {
            anchors.verticalCenter: parent.verticalCenter
            text: root.timeText
            tone: root.phase === "paused" ? "muted" : root.settled ? "dim" : "ink"
        }

        Row {
            anchors.verticalCenter: parent.verticalCenter
            visible: root.hasSystem || root.hasMic
            spacing: Math.round(2 * K.Style.scale)

            K.Glyph {
                visible: root.hasSystem
                name: "volume_up"
                size: K.Style.barSub
                tone: root.audio.system === "lost" ? "faint" : "dim"
            }

            K.Glyph {
                visible: root.hasMic
                name: "mic"
                size: K.Style.barSub
                tone: root.audio.mic === "lost" ? "faint" : "dim"
            }
        }
    }

    K.Press {
        id: press
        radius: K.Style.radius
        enabled: !root.settled
        onClicked: root.scene.toggleCard("recording", root)
        onRightClicked: Recorder.stop()
    }

    K.Hint {
        target: root
        text: root.hintText
        shown: press.hovered || (root.settled && settledHover.hovered)
    }

    HoverHandler { id: settledHover }
}
