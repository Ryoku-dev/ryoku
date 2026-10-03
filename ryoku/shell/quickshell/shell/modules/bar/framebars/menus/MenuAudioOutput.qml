pragma ComponentBehavior: Bound

import QtQuick
import "../.." as Pill
import shell.services
import "../../../../components"
import Ryoku.Ui.Singletons

// Audio output mixer (contract 06 sec 2.8): the default sink's volume + mute on
// a fader, a device switcher that lists the output devices with a check on the
// current default, and a per-app row for every playback stream so each app's
// volume and mute ride their own fader. Volume/mute read live from Pipewire; the
// device list from Audio.outputs and the app streams from Audio.streams. cava-
// style VU shimmer rides each fader from its node's live peak while open.
Item {
    id: root

    property real s: 1
    property bool open: false

    implicitHeight: col.implicitHeight

    readonly property var sink: Audio.sink
    readonly property bool haveSink: !!(root.sink && root.sink.audio)

    property bool devicesOpen: false

    Column {
        id: col
        width: root.width
        spacing: 8 * root.s

        // ── output device fader ──────────────────────────────────────────────
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

        // ── device switcher ──────────────────────────────────────────────────
        AudioDevicePicker {
            width: parent.width
            s: root.s
            current: root.sink
            devices: Audio.outputs
            listOpen: root.devicesOpen
            fallbackIcon: "speaker"
            emptyLabel: I18n.tr("No output device")
            onToggled: root.devicesOpen = !root.devicesOpen
            onPicked: node => Audio.setOutput(node)
        }

        // ── per-app mixer ─────────────────────────────────────────────────────
        MicroLabel {
            label: I18n.tr("Apps")
            s: root.s
            visible: root.open && Audio.streams.length > 0
        }
        Repeater {
            model: root.open ? Audio.streams : []
            delegate: AudioAppRow {
                required property var modelData
                s: root.s
                open: root.open
                stream: modelData
                width: col.width
            }
        }
        Text {
            visible: root.open && Audio.streams.length === 0
            width: parent.width
            topPadding: 2 * root.s
            text: I18n.tr("Nothing playing")
            horizontalAlignment: Text.AlignHCenter
            color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
            font.family: Theme.fontPrimary
            font.pixelSize: 10 * root.s
        }
    }
}
