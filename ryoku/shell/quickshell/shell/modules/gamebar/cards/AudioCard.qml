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

    // Peak monitors are audio work, so run them only while the card is on
    // screen. Power Saver rests a card left pinned behind a closed bar; an open
    // bar is being looked at.
    readonly property bool saverIdle: Perf.saver && card.pinned && !GameBarStore.open
    readonly property bool polling: (!card.host || (card.host.live && !card.host.suppressed))
        && !Presence.dark && !card.saverIdle

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
                : I18n.tr("n/a")
            peakNode: Audio.sink
            peakEnabled: card.polling && !!Audio.sink
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
                : I18n.tr("n/a")
            peakNode: Audio.source
            peakEnabled: card.polling && !!Audio.source
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

        ListView {
            id: apps
            width: parent.width
            height: Math.min(contentHeight, 3 * 46 * card.s)
            clip: true
            spacing: 4 * card.s
            boundsBehavior: Flickable.StopAtBounds
            interactive: contentHeight > height
            model: card.streams

            delegate: AudioAppRow {
                required property var modelData
                width: apps.width
                s: card.s
                stream: modelData
                open: card.polling
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
