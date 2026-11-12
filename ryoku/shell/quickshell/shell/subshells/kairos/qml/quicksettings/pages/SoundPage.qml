pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Widgets
import shell.services
import Ryoku.Ui.Singletons
import "../../kit"
import "../Segments.js" as Segments

Column {
    id: page

    property var host

    spacing: 8

    readonly property var sink: Audio.sink
    readonly property var source: Audio.source
    readonly property var outputs: Audio.outputs.filter(Audio.alive)
    readonly property var inputs: Audio.inputs.filter(Audio.alive)
    readonly property var apps: Audio.streams.filter(Audio.alive)

    function glyphFor(n) {
        var g = Audio.nodeIcon(n);
        return g === "monitor" ? "tv" : g;
    }
    function setVolume(audio, v) {
        if (!audio) return;
        if (audio.muted && v > 0) audio.muted = false;
        audio.volume = Math.max(0, Math.min(1, v));
    }
    function toggleMute(audio) {
        if (audio) audio.muted = !audio.muted;
    }
    function deviceNote(n) {
        var pct = Audio.batteryOf(n);
        return pct >= 0 ? I18n.tr("Battery %1%").replace("%1", pct) : "";
    }

    component Level: Segment {
        id: level
        property var audio: null
        property string iconOn: ""
        property string iconOff: ""
        readonly property bool muted: !!level.audio && level.audio.muted
        height: 72

        Slider {
            id: levelSlider
            anchors {
                left: parent.left; leftMargin: 14
                right: levelText.left; rightMargin: 12
                verticalCenter: parent.verticalCenter
            }
            enabled: level.audio !== null
            value: level.audio ? level.audio.volume : 0
            muted: level.muted
            icon: level.muted || levelSlider.value <= 0 ? level.iconOff : level.iconOn
            iconClickable: true
            onMoved: v => page.setVolume(level.audio, v)
            onCommitted: v => page.setVolume(level.audio, v)
            onIconClicked: page.toggleMute(level.audio)
        }
        Label {
            id: levelText
            anchors { right: muteButton.left; rightMargin: 6; verticalCenter: parent.verticalCenter }
            width: 40
            horizontalAlignment: Text.AlignRight
            role: "label"
            font.family: Md.mono
            color: Md.inkMuted
            text: level.audio ? Math.round((levelSlider.dragging ? levelSlider.live : levelSlider.value) * 100) + "%" : ""
        }
        IconButton {
            id: muteButton
            anchors { right: parent.right; rightMargin: 12; verticalCenter: parent.verticalCenter }
            enabled: level.audio !== null
            variant: "tonal"
            checked: level.muted
            icon: level.muted ? level.iconOff : level.iconOn
            tip: level.muted ? I18n.tr("Unmute") : I18n.tr("Mute")
            onClicked: page.toggleMute(level.audio)
        }
    }

    component DeviceRow: ListItem {
        id: device
        required property var modelData
        required property int index
        property int count: 1
        property int offset: 0
        property var current: null
        signal picked()
        width: parent ? parent.width : 0
        position: Segments.position(device.index + device.offset, device.count + device.offset)
        selected: device.modelData === device.current
        icon: page.glyphFor(device.modelData)
        headline: Audio.nodeLabel(device.modelData)
        supporting: page.deviceNote(device.modelData)
        onClicked: device.picked()

        Icon {
            anchors.verticalCenter: parent.verticalCenter
            visible: device.selected
            name: "check"
            size: 20
            color: device.text
        }
    }

    TopBar {
        width: parent.width
        title: I18n.tr("Sound")
        onBack: page.host.back()
    }

    SectionLabel { text: I18n.tr("Output") }
    Column {
        width: parent.width
        spacing: Md.seam

        Level {
            position: page.outputs.length > 0 ? "first" : "only"
            audio: page.sink ? page.sink.audio : null
            iconOn: "volume_up"
            iconOff: "volume_off"
        }
        Repeater {
            model: page.outputs
            delegate: DeviceRow {
                count: page.outputs.length
                offset: 1
                current: page.sink
                onPicked: Audio.setOutput(modelData)
            }
        }
    }

    SectionLabel { text: I18n.tr("Microphone") }
    Column {
        width: parent.width
        spacing: Md.seam

        Level {
            position: page.inputs.length > 0 ? "first" : "only"
            audio: page.source ? page.source.audio : null
            iconOn: "mic"
            iconOff: "mic_off"
        }
        Repeater {
            model: page.inputs
            delegate: DeviceRow {
                count: page.inputs.length
                offset: 1
                current: page.source
                onPicked: Audio.setInput(modelData)
            }
        }
    }

    SectionLabel {
        visible: page.apps.length > 0
        text: I18n.tr("Apps")
    }
    Column {
        width: parent.width
        visible: page.apps.length > 0
        spacing: Md.seam

        Repeater {
            model: page.apps
            delegate: Segment {
                id: app
                required property var modelData
                required property int index
                readonly property var audio: app.modelData ? app.modelData.audio : null
                readonly property bool muted: !!app.audio && app.audio.muted
                position: Segments.position(app.index, page.apps.length)
                height: 100

                Rectangle {
                    id: appDisc
                    anchors { left: parent.left; leftMargin: 14; top: parent.top; topMargin: 12 }
                    width: 28
                    height: 28
                    radius: 14
                    color: Md.containerHighest

                    IconImage {
                        anchors.centerIn: parent
                        width: 20
                        height: 20
                        source: Audio.streamIcon(app.modelData)
                        opacity: app.muted ? 0.4 : 1
                    }
                }
                Label {
                    anchors {
                        left: appDisc.right; leftMargin: 10
                        right: appText.left; rightMargin: 8
                        verticalCenter: appDisc.verticalCenter
                    }
                    role: "title"
                    font.pixelSize: 15
                    color: app.muted ? Md.inkMuted : Md.ink
                    text: Audio.streamName(app.modelData)
                }
                Label {
                    id: appText
                    anchors { right: parent.right; rightMargin: 18; verticalCenter: appDisc.verticalCenter }
                    role: "label"
                    font.family: Md.mono
                    color: Md.inkMuted
                    text: app.audio ? Math.round((appLevel.dragging ? appLevel.live : appLevel.value) * 100) + "%" : ""
                }
                Slider {
                    id: appLevel
                    anchors {
                        left: parent.left; leftMargin: 14
                        right: parent.right; rightMargin: 14
                        bottom: parent.bottom; bottomMargin: 12
                    }
                    height: 44
                    enabled: app.audio !== null
                    value: app.audio ? app.audio.volume : 0
                    muted: app.muted
                    icon: app.muted || appLevel.value <= 0 ? "volume_off" : "volume_up"
                    iconClickable: true
                    onMoved: v => page.setVolume(app.audio, v)
                    onCommitted: v => page.setVolume(app.audio, v)
                    onIconClicked: page.toggleMute(app.audio)
                }
            }
        }
    }

    Item { width: 1; height: 4 }
}
