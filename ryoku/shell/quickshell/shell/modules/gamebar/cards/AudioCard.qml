pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import ".."
import Ryoku.Ui.Singletons
import Lithic
import "../look"

GameBarCard {
    id: card

    title: I18n.tr("Audio")
    resizable: true

    readonly property var sinkAudio: Audio.sink && Audio.sink.audio ? Audio.sink.audio : null
    readonly property var sourceAudio: Audio.source && Audio.source.audio ? Audio.source.audio : null
    readonly property var streams: Audio.streams ? Audio.streams.filter(Audio.alive) : []

    readonly property bool saverIdle: Perf.saver && card.pinned && !GameBarStore.open
    readonly property bool polling: (!card.host || (card.host.live && !card.host.suppressed))
        && !Presence.dark && !card.saverIdle

    function glyphFor(node) {
        if (!node)
            return "volume_up";
        switch (Audio.nodeIcon(node)) {
        case "mic": return "mic";
        case "headphones": return "headphones";
        case "monitor": return "desktop_windows";
        default: return "volume_up";
        }
    }

    readonly property real gap: card.space2
    readonly property real rowSpacing: 2 * card.s
    readonly property real rowHeight: 40 * card.s
    readonly property real chromeHeight: card.grab * 2 + card.pad * 2 + card.head + 6 * card.s
        + output.implicitHeight + card.gap + input.implicitHeight + card.gap
        + LithicTokens.size.hairline + card.gap + appsLabel.implicitHeight + card.gap + card.gap
    readonly property real allRowsHeight: card.streams.length > 0
        ? card.streams.length * card.rowHeight + (card.streams.length - 1) * card.rowSpacing
        : card.rowHeight
    readonly property real threeRowsHeight: Math.min(card.allRowsHeight,
        3 * card.rowHeight + 2 * card.rowSpacing)
    readonly property real roomHeight: card.host ? card.host.height * 0.8 - card.chromeHeight : 0
    readonly property real listHeight: Math.max(card.threeRowsHeight,
        Math.min(card.allRowsHeight, card.roomHeight))

    preferredHeight: card.chromeHeight + card.listHeight

    Item {
        anchors.fill: parent

        GameBarFader {
            id: output
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            glyph: card.glyphFor(Audio.sink)
            label: I18n.tr("Output")
            value: card.sinkAudio ? card.sinkAudio.volume : 0
            valueLabel: card.sinkAudio
                ? (card.sinkAudio.muted ? I18n.tr("off") : Math.round(card.sinkAudio.volume * 100) + "%")
                : I18n.tr("n/a")
            muted: card.sinkAudio ? card.sinkAudio.muted : false
            available: !!card.sinkAudio
            peakNode: Audio.sink
            peakEnabled: card.polling && !!Audio.sink
            onMoved: v => { if (card.sinkAudio) card.sinkAudio.volume = v; }
            onIconTapped: { if (card.sinkAudio) card.sinkAudio.muted = !card.sinkAudio.muted; }
        }

        GameBarFader {
            id: input
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: output.bottom
            anchors.topMargin: card.gap
            glyph: "mic"
            label: I18n.tr("Input")
            value: card.sourceAudio ? card.sourceAudio.volume : 0
            valueLabel: card.sourceAudio
                ? (card.sourceAudio.muted ? I18n.tr("off") : Math.round(card.sourceAudio.volume * 100) + "%")
                : I18n.tr("n/a")
            muted: card.sourceAudio ? card.sourceAudio.muted : false
            available: !!card.sourceAudio
            peakNode: Audio.source
            peakEnabled: card.polling && !!Audio.source
            onMoved: v => { if (card.sourceAudio) card.sourceAudio.volume = v; }
            onIconTapped: { if (card.sourceAudio) card.sourceAudio.muted = !card.sourceAudio.muted; }
        }

        Rectangle {
            id: divider
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: input.bottom
            anchors.topMargin: card.gap
            height: LithicTokens.size.hairline
            color: GameBarTone.line
        }

        Label {
            id: appsLabel
            anchors.left: parent.left
            anchors.top: divider.bottom
            anchors.topMargin: card.gap
            text: I18n.tr("Applications")
            typeRole: "group"
            inkRole: "fg3"
        }

        ListView {
            id: apps
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: appsLabel.bottom
            anchors.topMargin: card.gap
            anchors.bottom: parent.bottom
            anchors.bottomMargin: card.gap
            clip: true
            spacing: card.rowSpacing
            boundsBehavior: Flickable.StopAtBounds
            interactive: contentHeight > height
            model: card.streams

            delegate: GameBarAppRow {
                required property var modelData
                width: apps.width
                stream: modelData
                open: card.polling
            }
        }

        Label {
            anchors.top: appsLabel.bottom
            anchors.topMargin: card.gap
            anchors.left: parent.left
            anchors.right: parent.right
            visible: card.streams.length === 0
            text: I18n.tr("Nothing playing")
            typeRole: "detail"
            inkRole: "fg3"
            horizontalAlignment: Text.AlignHCenter
        }
    }
}
