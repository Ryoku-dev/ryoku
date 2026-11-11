pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import ".."
import "../../../components"
import Ryoku.Ui.Singletons
import "../look"

// Media: the one now-playing pick the whole shell shares, with transport. The
// controls are 40px borderless buttons and the art sits on a 12px tile, as in
// the overlay's media pane.
GameBarCard {
    id: card

    title: I18n.tr("Media")

    readonly property var player: Media.player
    readonly property bool present: Media.present
    readonly property string art: card.player && card.player.trackArtUrl ? String(card.player.trackArtUrl) : ""
    readonly property string artist: card.player
        ? Theme.joinArtists(card.player.trackArtists, card.player.trackArtist) : ""

    Column {
        width: parent.width
        spacing: 12 * card.s

        Row {
            width: parent.width
            spacing: 14 * card.s

            Rectangle {
                width: 104 * card.s
                height: 104 * card.s
                radius: GameBarLook.radiusSmall * card.s
                color: GameBarLook.raised
                clip: true

                Image {
                    anchors.fill: parent
                    source: card.art
                    sourceSize.width: Math.ceil(104 * card.s)
                    sourceSize.height: Math.ceil(104 * card.s)
                    asynchronous: true
                    fillMode: Image.PreserveAspectCrop
                    visible: card.art.length > 0
                }
                GlyphIcon {
                    anchors.centerIn: parent
                    width: 32 * card.s
                    height: 32 * card.s
                    name: "music"
                    color: GameBarLook.onLayer
                    visible: card.art.length === 0
                    stroke: 1.6
                }
            }

            Column {
                width: parent.width - 104 * card.s - 14 * card.s
                spacing: 4 * card.s
                anchors.verticalCenter: parent.verticalCenter

                Text {
                    width: parent.width
                    textFormat: Text.PlainText
                    text: card.present ? card.player.trackTitle : I18n.tr("Nothing playing")
                    color: GameBarLook.fg
                    font.family: GameBarLook.face
                    font.pixelSize: GameBarLook.small * card.s
                    elide: Text.ElideRight
                    maximumLineCount: 2
                    wrapMode: Text.WordWrap
                }
                Text {
                    width: parent.width
                    textFormat: Text.PlainText
                    text: card.present
                        ? (card.artist.length > 0 ? card.artist : I18n.tr("Unknown artist"))
                        : I18n.tr("Start a track in any player")
                    color: GameBarLook.subtext
                    font.family: GameBarLook.face
                    font.pixelSize: GameBarLook.smallie * card.s
                    elide: Text.ElideRight
                }
            }
        }

        Row {
            anchors.horizontalCenter: parent.horizontalCenter
            spacing: 8 * card.s

            GameBarTransport {
                s: card.s
                glyph: "prev"
                available: card.present && card.player.canGoPrevious
                onTapped: if (card.player) card.player.previous()
            }
            GameBarTransport {
                s: card.s
                glyph: Media.playing ? "pause" : "play"
                available: card.present
                onTapped: Media.toggle()
            }
            GameBarTransport {
                s: card.s
                glyph: "next"
                available: card.present && card.player.canGoNext
                onTapped: if (card.player) card.player.next()
            }
        }
    }
}
