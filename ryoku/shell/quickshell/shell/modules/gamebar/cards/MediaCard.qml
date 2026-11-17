pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import ".."
import "../../../components"
import Ryoku.Ui.Singletons
import Lithic
import "../look"

GameBarCard {
    id: card

    title: I18n.tr("Media")
    resizable: true

    property bool lyricsOpen: false

    readonly property var player: Media.player
    readonly property bool present: Media.present
    readonly property string art: {
        const direct = card.player ? String(card.player.trackArtUrl || "") : "";
        return direct.startsWith("file://") ? direct : Music.coverFor(card.player);
    }
    readonly property string artist: card.player
        ? Theme.joinArtists(card.player.trackArtists, card.player.trackArtist) : ""
    readonly property bool wantsClock: card.lyricsOpen
        && (!card.host || (card.host.live && !card.host.suppressed)) && !Presence.dark
    readonly property real zoom: Math.min(1.6, Math.max(1,
        Math.min(card.width / (358 * card.s), card.height / (235 * card.s))))

    onWantsClockChanged: Music.hold(card, card.wantsClock)
    Component.onCompleted: Music.hold(card, card.wantsClock)
    Component.onDestruction: Music.hold(card, false)

    Item {
        id: body
        anchors.fill: parent

        Item {
            id: nowRow
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            height: Math.max(0, parent.height - 48 * card.s)
            opacity: card.lyricsOpen ? 0 : 1

            Behavior on opacity {
                NumberAnimation { duration: Lithic.duration(180); easing.type: Easing.OutCubic }
            }

            Item {
                anchors.fill: parent

                GameBarPlate {
                    id: artPlate
                    anchors.verticalCenter: parent.verticalCenter
                    width: Math.min(96 * card.s, Math.max(0, body.height - 48 * card.s), body.width * 0.34)
                    height: width
                    cut: GameBarTone.controlCut
                    source: card.art
                    glyph: "music_note"
                    decodeSize: 192
                }

                Column {
                    anchors.left: artPlate.right
                    anchors.leftMargin: card.space3
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 2 * card.s

                    Label {
                        width: parent.width
                        text: card.present && card.player ? card.player.trackTitle : I18n.tr("Nothing playing")
                        typeRole: "title"
                        wrapMode: Text.WordWrap
                        maximumLineCount: 2
                    }
                    Label {
                        width: parent.width
                        text: card.present
                            ? (card.artist.length > 0 ? card.artist : I18n.tr("Unknown artist"))
                            : I18n.tr("Start a track in any player")
                        typeRole: "detail"
                        inkRole: "fg2"
                    }
                }
            }
        }

        LyricsSheet {
            id: lyrics
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.bottom: transport.top
            anchors.bottomMargin: card.space2
            visible: opacity > 0
            opacity: card.lyricsOpen ? 1 : 0
            s: card.s
            ink: GameBarTone.ink
            dim: GameBarTone.ink2
            accent: GameBarTone.lyric
            face: Lithic.font.sans
            base: Math.round(Lithic.type.detail.pixelSize * card.zoom)
            align: Text.AlignHCenter
            glow: false
            emptyText: card.present ? "" : I18n.tr("Nothing playing")

            Behavior on opacity {
                NumberAnimation { duration: Lithic.duration(200); easing.type: Easing.OutCubic }
            }
        }

        Row {
            id: transport
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.bottom: parent.bottom
            spacing: 6 * card.s

            IconPlaque {
                small: true
                kind: "quiet"
                glyph: "skip_previous"
                enabled: card.player !== null && card.player.canGoPrevious
                Accessible.name: I18n.tr("Previous")
                onClicked: if (card.player) card.player.previous()
            }

            IconPlaque {
                kind: "gold"
                glyph: Media.playing ? "pause" : "play_arrow"
                enabled: card.present
                Accessible.name: Media.playing ? I18n.tr("Pause") : I18n.tr("Play")
                onClicked: Media.toggle()
            }

            IconPlaque {
                small: true
                kind: "quiet"
                glyph: "skip_next"
                enabled: card.player !== null && card.player.canGoNext
                Accessible.name: I18n.tr("Next")
                onClicked: if (card.player) card.player.next()
            }

            IconPlaque {
                small: true
                kind: card.lyricsOpen ? "tonal" : "quiet"
                glyph: "lyrics"
                Accessible.name: I18n.tr("Lyrics")
                onClicked: card.lyricsOpen = !card.lyricsOpen
            }
        }
    }
}
