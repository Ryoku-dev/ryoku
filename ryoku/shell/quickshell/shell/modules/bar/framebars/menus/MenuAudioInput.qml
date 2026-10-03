pragma ComponentBehavior: Bound

import QtQuick
import "../.." as Pill
import shell.services
import "../../../../components"
import Ryoku.Ui.Singletons

// Audio input control (contract 06 sec 2.8): the default source's volume + mute
// on a fader, and a device switcher listing the input devices with a check on
// the current default. Volume/mute read live from the default Pipewire source;
// the device list from Audio.inputs.
Item {
    id: root

    property real s: 1
    property bool open: false

    implicitHeight: col.implicitHeight

    readonly property var source: Audio.source
    readonly property bool haveSource: !!(root.source && root.source.audio)

    property bool devicesOpen: false

    Column {
        id: col
        width: root.width
        spacing: 8 * root.s

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

        AudioDevicePicker {
            width: parent.width
            s: root.s
            current: root.source
            devices: Audio.inputs
            listOpen: root.devicesOpen
            fallbackIcon: "mic"
            emptyLabel: I18n.tr("No input device")
            onToggled: root.devicesOpen = !root.devicesOpen
            onPicked: node => Audio.setInput(node)
        }
    }
}
