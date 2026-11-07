pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import "../../kit" as K

Item {
    id: page

    required property var scene
    property var card: null
    property bool shown: false

    implicitWidth: 320
    implicitHeight: content.implicitHeight

    readonly property var sink: Audio.sink
    readonly property var source: Audio.source
    readonly property var sinkAudio: sink && sink.audio ? sink.audio : null
    readonly property var sourceAudio: source && source.audio ? source.audio : null
    readonly property var outputs: (Audio.outputs || []).filter(Audio.alive)
    readonly property var inputs: (Audio.inputs || []).filter(Audio.alive)
    readonly property var streams: (Audio.streams || []).filter(Audio.alive)

    function setVolume(audio, value) {
        if (!audio)
            return;
        audio.volume = Math.max(0, Math.min(1, value));
        if (audio.muted && value > 0)
            audio.muted = false;
    }

    function toggleMute(audio) {
        if (audio)
            audio.muted = !audio.muted;
    }

    component DeviceEntry: K.ListRow {
        id: deviceRow

        required property var node
        required property bool input
        readonly property var activeNode: input ? page.source : page.sink
        readonly property bool activeDevice: !!activeNode && !!node
            && activeNode.name === node.name

        icon: Audio.nodeIcon(node)
        title: Audio.nodeLabel(node)
        subtitle: activeDevice ? (input ? "Current input" : "Current output") : ""
        selected: activeDevice
        onClicked: {
            if (input)
                Audio.setInput(node);
            else
                Audio.setOutput(node);
        }

        K.Glyph {
            visible: deviceRow.activeDevice
            name: "check"
            size: K.Style.icon
            tone: "onBone"
        }
    }

    component VolumeControl: Row {
        id: control

        required property var audio
        required property string iconOn
        required property string iconOff

        spacing: K.Style.s2

        K.Slider {
            id: volumeSlider
            anchors.verticalCenter: parent.verticalCenter
            width: Math.max(0, control.width - muteButton.width - control.spacing)
            value: control.audio ? control.audio.volume : 0
            icon: !control.audio || control.audio.muted || control.audio.volume <= 0
                ? control.iconOff : control.iconOn
            valueText: Math.round(value * 100) + "%"
            enabled: control.audio !== null
            onMoved: value => page.setVolume(control.audio, value)
            onCommitted: value => page.setVolume(control.audio, value)
        }

        K.IconButton {
            id: muteButton
            anchors.verticalCenter: parent.verticalCenter
            icon: control.audio && control.audio.muted ? control.iconOff : control.iconOn
            toggled: !!control.audio && control.audio.muted
            enabled: control.audio !== null
            onClicked: page.toggleMute(control.audio)
        }
    }

    component StreamEntry: Item {
        id: streamRow

        required property var stream
        readonly property var audio: stream && stream.audio ? stream.audio : null
        property bool divider: false

        implicitHeight: streamLabel.implicitHeight + streamControl.implicitHeight + K.Style.s1

        K.ListRow {
            id: streamLabel
            width: parent.width
            image: Audio.streamIcon(streamRow.stream)
            title: Audio.streamName(streamRow.stream)
            subtitle: streamRow.audio && streamRow.audio.muted ? "Muted" : "Application"
            divider: streamRow.divider
        }

        VolumeControl {
            id: streamControl
            anchors.top: streamLabel.bottom
            anchors.topMargin: K.Style.s1
            width: parent.width
            audio: streamRow.audio
            iconOn: "volume_up"
            iconOff: "volume_off"
        }
    }

    Column {
        id: content

        width: parent.width
        spacing: K.Style.s4

        Column {
            width: parent.width
            spacing: K.Style.s2

            K.Label {
                width: parent.width
                text: "Output"
                role: "micro"
            }

            VolumeControl {
                width: parent.width
                audio: page.sinkAudio
                iconOn: "volume_up"
                iconOff: "volume_off"
            }

            Repeater {
                model: page.outputs
                delegate: DeviceEntry {
                    required property var modelData
                    required property int index
                    width: parent.width
                    node: modelData
                    input: false
                    divider: index > 0
                }
            }

            K.Label {
                visible: page.outputs.length === 0
                width: parent.width
                text: "No output devices"
                role: "body"
                tone: "variant"
            }
        }

        Column {
            width: parent.width
            spacing: K.Style.s2

            K.Label {
                width: parent.width
                text: "Input"
                role: "micro"
            }

            VolumeControl {
                width: parent.width
                audio: page.sourceAudio
                iconOn: "mic"
                iconOff: "mic_off"
            }

            Repeater {
                model: page.inputs
                delegate: DeviceEntry {
                    required property var modelData
                    required property int index
                    width: parent.width
                    node: modelData
                    input: true
                    divider: index > 0
                }
            }

            K.Label {
                visible: page.inputs.length === 0
                width: parent.width
                text: "No input devices"
                role: "body"
                tone: "variant"
            }
        }

        Column {
            visible: page.streams.length > 0
            width: parent.width
            spacing: K.Style.s2

            K.Label {
                width: parent.width
                text: "Applications"
                role: "micro"
            }

            Repeater {
                model: page.streams
                delegate: StreamEntry {
                    required property var modelData
                    required property int index
                    width: parent.width
                    stream: modelData
                    divider: index > 0
                }
            }
        }
    }
}
