pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import "../kit" as K
import "../recording" as Rec
import "../recording/Clock.js" as Clock
import "../recording/Audio.js" as Audio

Item {
    id: root

    required property var scene
    property bool shown: false
    property var args: ({})

    readonly property string phase: String(Recorder.phase || "idle")
    readonly property bool arming: phase === "arming"
    readonly property bool saving: phase === "stopping"
    readonly property bool open: ["arming", "starting", "recording", "paused", "stopping"]
        .indexOf(phase) >= 0
    readonly property var audio: Audio.sources(phase, Recorder.activeAudioMode,
        Recorder.audioFallback === true, Recorder.optDesktopAudio, Recorder.optMic)

    implicitWidth: sheet.implicitWidth
    implicitHeight: sheet.implicitHeight

    function stateWord() {
        switch (root.phase) {
        case "arming": return "Recording in";
        case "starting": return "Starting";
        case "paused": return "Paused";
        case "stopping": return "Saving";
        default: return "Rec";
        }
    }

    function targetText() {
        const target = String(Recorder.target || "");
        if (target === "")
            return "";
        if (target === "screen")
            return "All displays";
        const region = /^(\d+)x(\d+)\+-?\d+\+-?\d+$/.exec(target);
        if (region)
            return "Region " + region[1] + "\u00d7" + region[2];
        return "Display " + target;
    }

    function headerText() {
        const target = root.arming ? "" : root.targetText();
        return target.length > 0 ? root.stateWord() + " \u00b7 " + target : root.stateWord();
    }

    // the card goes with the capture, the bubble then says how it ended
    onOpenChanged: if (!open && shown && scene)
        scene.closeCard()

    K.Sheet {
        id: sheet
        pageWidth: 300

        Row {
            width: parent.width
            spacing: K.Style.s2

            Rec.PhaseMark {
                id: headerMark
                anchors.verticalCenter: parent.verticalCenter
                phase: root.phase
            }

            K.Label {
                anchors.verticalCenter: parent.verticalCenter
                width: parent.width - headerMark.width - parent.spacing
                text: root.headerText()
                role: "micro"
                tone: root.phase === "recording" || root.arming ? "alert" : "muted"
                strong: true
            }
        }

        K.Label {
            width: parent.width
            text: root.arming ? String(Recorder.countdownSec) : Clock.format(Recorder.elapsedSec)
            role: "display"
            tone: root.phase === "paused" || root.saving ? "dim" : "ink"
            numeric: true
        }

        Column {
            visible: !root.arming
            width: parent.width

            AudioRow {
                label: "Desktop audio"
                source: root.audio.system
            }

            AudioRow {
                label: "Microphone"
                source: root.audio.mic
            }

            K.Label {
                visible: Recorder.audioFallback
                width: parent.width
                topPadding: K.Style.s2
                text: "The desktop and microphone mix failed, so one source is recorded."
                role: "caption"
                tone: "alert"
                wrapMode: Text.WordWrap
            }
        }

        Row {
            visible: !root.arming
            width: parent.width
            spacing: K.Style.s2

            ActionButton {
                width: (parent.width - parent.spacing) / 2
                text: Recorder.paused ? "Resume" : "Pause"
                icon: Recorder.paused ? "play_arrow" : "pause"
                enabled: root.phase === "recording" || root.phase === "paused"
                onClicked: Recorder.togglePause()
            }

            K.HoldButton {
                width: (parent.width - parent.spacing) / 2
                height: K.Style.rowH
                text: "Stop"
                icon: "stop"
                danger: true
                enabled: !root.saving
                onActivated: Recorder.stop()
            }
        }

        ActionButton {
            visible: root.arming
            width: parent.width
            text: "Cancel"
            icon: "close"
            onClicked: Recorder.cancelCountdown()
        }
    }

    // drawn like the held Stop next to it so the two read as a pair
    component ActionButton: Item {
        id: action

        property string text: ""
        property string icon: ""
        signal clicked()

        height: K.Style.rowH
        opacity: enabled ? 1 : 0.38
        scale: actionPress.pressScale

        Rectangle {
            anchors.fill: parent
            radius: K.Style.radius
            color: actionPress.pressed ? K.Style.tint16
                : actionPress.hovered ? K.Style.tint10 : "transparent"
            border.width: K.Style.border
            border.color: actionPress.hovered ? K.Style.lineStrong : K.Style.line
        }

        Row {
            anchors.centerIn: parent
            spacing: K.Style.s2

            K.Glyph {
                anchors.verticalCenter: parent.verticalCenter
                name: action.icon
                size: K.Style.icon
                tone: "dim"
            }

            K.Label {
                anchors.verticalCenter: parent.verticalCenter
                text: action.text
                role: "bodyStrong"
            }
        }

        K.Press {
            id: actionPress
            tint: false
            enabled: action.enabled
            onClicked: action.clicked()
        }
    }

    component AudioRow: Item {
        id: audioRow

        property string label: ""
        property string source: "off"

        width: parent.width
        height: K.Style.rowH

        Rectangle {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            height: K.Style.border
            color: K.Style.lineSoft
        }

        K.Label {
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            text: audioRow.label
            role: "body"
        }

        K.Label {
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            text: audioRow.source === "on" ? "On" : audioRow.source === "lost" ? "Dropped" : "Off"
            role: "micro"
            tone: audioRow.source === "on" ? "ink" : audioRow.source === "lost" ? "alert" : "faint"
        }
    }
}
