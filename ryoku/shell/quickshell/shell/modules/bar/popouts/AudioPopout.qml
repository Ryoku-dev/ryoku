pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import "../../../components"
import ".."
import "../framebars/menus" as Menus
import Ryoku.Ui.Singletons

// Audio popout: the mixer card grown off the speaker/mic rail widget (shared
// PopoutCard, so it opens and melts like the music card). Output and input each
// lead with a live VU fader -- tap the glyph to mute, drag or wheel to set --
// over an expandable device switcher; a Bluetooth sink also shows its codec and
// a Hi-Fi <-> Headset profile flip. Per-app rows give every playback stream its
// own fader, and the recording section does the same for apps capturing the mic,
// the control surface a streamer reaches for. All live off Pipewire via Audio.
Item {
    id: root

    property real s: 1
    property bool open: false

    readonly property real pad: 11 * root.s
    readonly property color ink: Theme.ink(Theme.effectiveSurface)
    readonly property color inkDim: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
    readonly property color line: Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.14)

    readonly property var sink: Audio.sink
    readonly property var source: Audio.source
    readonly property bool haveSink: !!(root.sink && root.sink.audio)
    readonly property bool haveSource: !!(root.source && root.source.audio)

    // device switchers collapse again whenever the card closes.
    property bool outDevicesOpen: false
    property bool inDevicesOpen: false
    onOpenChanged: if (!root.open) { root.outDevicesOpen = false; root.inDevicesOpen = false; }

    implicitWidth: 300 * root.s
    implicitHeight: content.implicitHeight + root.pad * 2

    PopoutCard { anchors.fill: parent }

    Column {
        id: content
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.margins: root.pad
        spacing: 11 * root.s

        // head.
        Text {
            text: I18n.tr("AUDIO")
            color: root.inkDim
            font.family: Theme.mono
            font.pixelSize: 9 * root.s
            font.letterSpacing: 1.6
            font.weight: Font.Medium
        }

        // ── output ───────────────────────────────────────────────────────────
        Column {
            width: parent.width
            spacing: 6 * root.s

            Text {
                width: parent.width
                text: I18n.tr("OUTPUT")
                color: root.inkDim
                font.family: Theme.mono
                font.pixelSize: 8.5 * root.s
                font.letterSpacing: 1.5
            }
            HFader {
                width: parent.width
                s: root.s
                icon: root.sink ? Audio.nodeIcon(root.sink) : "speaker"
                lit: root.open
                value: root.haveSink ? root.sink.audio.volume : 0
                muted: root.haveSink ? root.sink.audio.muted : false
                valueLabel: !root.haveSink ? "" : (root.sink.audio.muted ? I18n.tr("off") : Math.round(root.sink.audio.volume * 100) + "%")
                peakNode: root.sink
                peakEnabled: root.open && !!root.sink
                onMoved: v => { if (root.haveSink) root.sink.audio.volume = v; }
                onIconTapped: { if (root.haveSink) root.sink.audio.muted = !root.sink.audio.muted; }
            }
            Menus.AudioDevicePicker {
                width: parent.width
                s: root.s
                current: root.sink
                devices: Audio.outputs.filter(Audio.alive)
                listOpen: root.outDevicesOpen
                fallbackIcon: "speaker"
                emptyLabel: I18n.tr("No output device")
                onToggled: root.outDevicesOpen = !root.outDevicesOpen
                onPicked: node => Audio.setOutput(node)
            }
            // a Bluetooth sink: its live codec, and a tap to flip the profile.
            Row {
                width: parent.width
                spacing: 6 * root.s
                visible: Audio.sinkIsBluez
                PopoutChip {
                    s: root.s
                    glyph: "bluetooth"
                    label: Audio.btCodec.length ? Audio.btCodec : I18n.tr("Codec")
                }
                PopoutChip {
                    s: root.s
                    label: Audio.profileLabel().length ? Audio.profileLabel() : I18n.tr("Profile")
                    act: true
                    onClicked: Audio.toggleProfile()
                }
            }
        }

        // ── input ────────────────────────────────────────────────────────────
        Column {
            width: parent.width
            spacing: 6 * root.s

            Text {
                width: parent.width
                text: I18n.tr("INPUT")
                color: root.inkDim
                font.family: Theme.mono
                font.pixelSize: 8.5 * root.s
                font.letterSpacing: 1.5
            }
            HFader {
                width: parent.width
                s: root.s
                icon: "mic"
                lit: root.open
                value: root.haveSource ? root.source.audio.volume : 0
                muted: root.haveSource ? root.source.audio.muted : false
                valueLabel: !root.haveSource ? "" : (root.source.audio.muted ? I18n.tr("off") : Math.round(root.source.audio.volume * 100) + "%")
                peakNode: root.source
                peakEnabled: root.open && !!root.source
                onMoved: v => { if (root.haveSource) root.source.audio.volume = v; }
                onIconTapped: { if (root.haveSource) root.source.audio.muted = !root.source.audio.muted; }
            }
            Menus.AudioDevicePicker {
                width: parent.width
                s: root.s
                current: root.source
                devices: Audio.inputs.filter(Audio.alive)
                listOpen: root.inDevicesOpen
                fallbackIcon: "mic"
                emptyLabel: I18n.tr("No input device")
                onToggled: root.inDevicesOpen = !root.inDevicesOpen
                onPicked: node => Audio.setInput(node)
            }
        }

        // ── per-app playback ──────────────────────────────────────────────────
        Column {
            id: appsCol
            width: parent.width
            spacing: 6 * root.s

            Text {
                width: parent.width
                text: I18n.tr("APPS")
                color: root.inkDim
                font.family: Theme.mono
                font.pixelSize: 8.5 * root.s
                font.letterSpacing: 1.5
            }
            Repeater {
                model: root.open ? Audio.streams.filter(Audio.alive) : []
                delegate: AudioAppRow {
                    required property var modelData
                    width: appsCol.width
                    s: root.s
                    open: root.open
                    stream: modelData
                }
            }
            Text {
                visible: Audio.streams.length === 0
                width: parent.width
                topPadding: 2 * root.s
                text: I18n.tr("Nothing playing")
                horizontalAlignment: Text.AlignHCenter
                color: root.inkDim
                font.family: Theme.fontPrimary
                font.pixelSize: 10 * root.s
            }
        }

        // ── per-app capture (recording) ───────────────────────────────────────
        Column {
            id: capCol
            width: parent.width
            spacing: 6 * root.s
            visible: Audio.captureStreams.length > 0

            Text {
                width: parent.width
                text: I18n.tr("RECORDING")
                color: root.inkDim
                font.family: Theme.mono
                font.pixelSize: 8.5 * root.s
                font.letterSpacing: 1.5
            }
            Repeater {
                model: root.open ? Audio.captureStreams.filter(Audio.alive) : []
                delegate: AudioAppRow {
                    required property var modelData
                    width: capCol.width
                    s: root.s
                    open: root.open
                    capture: true
                    stream: modelData
                }
            }
        }
    }
}
