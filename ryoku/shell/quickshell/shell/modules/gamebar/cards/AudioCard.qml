pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import ".."
import "../../../components"
import Ryoku.Ui.Singletons
import "../look"

// Audio: the same faders and app rows the audio menu uses, so a level set here
// is the level Pipewire holds everywhere.
GameBarCard {
    id: card

    title: I18n.tr("Audio")

    readonly property var sinkAudio: Audio.sink && Audio.sink.audio ? Audio.sink.audio : null
    readonly property var sourceAudio: Audio.source && Audio.source.audio ? Audio.source.audio : null
    readonly property var streams: Audio.streams ? Audio.streams.filter(Audio.alive) : []

    Column {
        width: parent.width
        spacing: 8 * card.s

        HFader {
            width: parent.width
            s: card.s
            icon: Audio.sink ? Audio.nodeIcon(Audio.sink) : "speaker"
            lit: true
            value: card.sinkAudio ? card.sinkAudio.volume : 0
            muted: card.sinkAudio ? card.sinkAudio.muted : false
            valueLabel: card.sinkAudio
                ? (card.sinkAudio.muted ? I18n.tr("off") : Math.round(card.sinkAudio.volume * 100) + "%")
                : "n/a"
            peakNode: Audio.sink
            peakEnabled: !!Audio.sink
            onMoved: v => { if (card.sinkAudio) card.sinkAudio.volume = v; }
            onIconTapped: { if (card.sinkAudio) card.sinkAudio.muted = !card.sinkAudio.muted; }
        }

        HFader {
            width: parent.width
            s: card.s
            icon: "mic"
            lit: !!card.sourceAudio
            value: card.sourceAudio ? card.sourceAudio.volume : 0
            muted: card.sourceAudio ? card.sourceAudio.muted : false
            valueLabel: card.sourceAudio
                ? (card.sourceAudio.muted ? I18n.tr("off") : Math.round(card.sourceAudio.volume * 100) + "%")
                : "n/a"
            peakNode: Audio.source
            peakEnabled: !!Audio.source
            onMoved: v => { if (card.sourceAudio) card.sourceAudio.volume = v; }
            onIconTapped: { if (card.sourceAudio) card.sourceAudio.muted = !card.sourceAudio.muted; }
        }

        Rectangle {
            width: parent.width
            height: 1
            color: GameBarLook.border
        }

        Text {
            text: I18n.tr("Apps")
            color: GameBarLook.subtext
            font.family: GameBarLook.face
            font.pixelSize: GameBarLook.smallie * card.s
        }

        Repeater {
            model: card.streams.slice(0, 3)

            delegate: AudioAppRow {
                required property var modelData
                width: parent.width
                s: card.s
                stream: modelData
                open: true
            }
        }

        Text {
            visible: card.streams.length === 0
            width: parent.width
            text: I18n.tr("Nothing playing")
            color: GameBarLook.subtext
            horizontalAlignment: Text.AlignHCenter
            font.family: GameBarLook.face
            font.pixelSize: GameBarLook.small * card.s
        }
    }
}
