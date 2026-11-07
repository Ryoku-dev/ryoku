pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import "../kit" as K

Item {
    id: root

    required property var scene
    required property string lane

    readonly property var captureStreams: Audio.captureStreams || []
    readonly property var captureNames: {
        const names = [];
        const seen = ({});
        for (let i = 0; i < root.captureStreams.length; ++i) {
            const name = String(Audio.streamName(root.captureStreams[i]) || "Application");
            if (seen[name] !== true) {
                seen[name] = true;
                names.push(name);
            }
        }
        return names;
    }
    readonly property bool microphoneActive: captureNames.length > 0

    property bool present: Recorder.active || microphoneActive

    implicitWidth: present ? activities.implicitWidth + 2 * K.Style.hitPad : 0
    implicitHeight: K.Style.barH
    height: K.Style.barH

    Row {
        id: activities
        anchors.centerIn: parent
        spacing: Math.round(6 * K.Style.scale)

        Item {
            id: recording
            visible: Recorder.active
            width: Recorder.active ? recordingBody.implicitWidth : 0
            height: K.Style.barH

            Row {
                id: recordingBody
                anchors.centerIn: parent
                spacing: Math.round(5 * K.Style.scale)
                scale: recordingPress.pressScale

                Rectangle {
                    id: recordingDot
                    property real pulse: 1
                    anchors.verticalCenter: parent.verticalCenter
                    width: Math.round(8 * K.Style.scale)
                    height: width
                    radius: width / 2
                    color: K.Style.alert
                    opacity: pulse

                    SequentialAnimation on pulse {
                        running: Recorder.active && recording.visible
                            && root.visible && !K.Style.still
                        loops: Animation.Infinite
                        onStopped: recordingDot.pulse = 1

                        NumberAnimation {
                            from: 1
                            to: 0.35
                            duration: 600
                            easing.type: Easing.InOutSine
                        }
                        NumberAnimation {
                            from: 0.35
                            to: 1
                            duration: 600
                            easing.type: Easing.InOutSine
                        }
                    }
                }

                K.Label {
                    anchors.verticalCenter: parent.verticalCenter
                    text: Recorder.elapsedText
                    role: "barSub"
                    tone: "alert"
                    numeric: true
                    strong: true
                }
            }

            K.Press {
                id: recordingPress
                radius: K.Style.radius
                onClicked: Recorder.stop()
                onRightClicked: Recorder.stop()
            }

            K.Hint {
                target: recording
                text: "Recording, click to stop"
                shown: recordingPress.hovered
            }
        }

        Item {
            id: microphone
            visible: root.microphoneActive
            width: root.microphoneActive ? K.Style.barIcon : 0
            height: K.Style.barH

            K.Glyph {
                anchors.centerIn: parent
                scale: microphonePress.pressScale
                name: "mic"
                size: K.Style.barIcon
                tone: "alert"
                filled: true
            }

            K.Press {
                id: microphonePress
                radius: K.Style.radius
                onClicked: root.scene.toggleCard("controls", root, { page: "sound" })
                onRightClicked: Toggles.toggleMic()
                onHoveredChanged: root.scene.hoverCard("controls", root, hovered)
            }

            K.Hint {
                target: microphone
                text: "Microphone in use, " + root.captureNames.join(", ")
                shown: microphonePress.hovered
            }
        }
    }
}