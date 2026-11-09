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
    readonly property string art: card.player && card.player.artUrl ? String(card.player.artUrl) : ""
    readonly property string artist: card.player
        ? Theme.joinArtists(card.player.trackArtists, card.player.trackArtist) : ""

    component Transport: Item {
        id: button
        property string glyph: ""
        property bool available: true

        width: 40 * card.s
        height: 40 * card.s
        opacity: button.available ? 1 : 0.35

        signal tapped()

        Rectangle {
            anchors.fill: parent
            radius: height / 2
            color: transportHover.containsMouse ? GameBarLook.hover : "transparent"
        }
        GlyphIcon {
            anchors.centerIn: parent
            width: 18 * card.s
            height: 18 * card.s
            name: button.glyph
            color: GameBarLook.fg
            stroke: 1.7
        }
        MouseArea {
            id: transportHover
            anchors.fill: parent
            hoverEnabled: true
            enabled: button.available
            cursorShape: Qt.PointingHandCursor
            onClicked: button.tapped()
        }
    }

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

            Transport {
                glyph: "prev"
                available: card.present && card.player.canGoPrevious
                onTapped: if (card.player) card.player.previous()
            }
            Transport {
                glyph: Media.playing ? "pause" : "play"
                available: card.present
                onTapped: Media.toggle()
            }
            Transport {
                glyph: "next"
                available: card.present && card.player.canGoNext
                onTapped: if (card.player) card.player.next()
            }
        }
    }
}
