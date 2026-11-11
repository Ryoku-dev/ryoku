pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
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

    property bool lyricsOpen: false

    readonly property var player: Media.player
    readonly property bool present: Media.present
    readonly property string art: {
        const direct = card.player ? String(card.player.trackArtUrl || "") : "";
        return direct.startsWith("file://") ? direct : Music.coverFor(card.player);
    }
    readonly property string artist: card.player
        ? Theme.joinArtists(card.player.trackArtists, card.player.trackArtist) : ""
    readonly property bool wantsClock: card.lyricsOpen && card.visible

    onWantsClockChanged: Music.hold(card, card.wantsClock)
    Component.onCompleted: Music.hold(card, card.wantsClock)
    Component.onDestruction: Music.hold(card, false)

    Item {
        anchors.fill: parent

        Item {
            id: glass
            anchors.fill: parent
            anchors.margins: -8 * card.s
            visible: opacity > 0
            opacity: card.lyricsOpen ? 1 : 0
            layer.enabled: glass.visible && card.art.length > 0
            layer.effect: MultiEffect {
                maskEnabled: true
                maskSource: glassMask
                autoPaddingEnabled: false
                blurEnabled: true
                blur: 0.9
                blurMax: 64
                saturation: 0.6
                brightness: -0.02
            }

            Behavior on opacity {
                NumberAnimation { duration: Motion.dur(220); easing.type: Easing.OutCubic }
            }

            Rectangle {
                anchors.fill: parent
                radius: GameBarLook.radiusSmall * card.s
                color: GameBarLook.content
            }

            Image {
                anchors.fill: parent
                source: card.art
                sourceSize.width: 512
                sourceSize.height: 512
                asynchronous: true
                cache: true
                fillMode: Image.PreserveAspectCrop
                visible: card.art.length > 0
            }

            Rectangle {
                anchors.fill: parent
                radius: GameBarLook.radiusSmall * card.s
                gradient: Gradient {
                    GradientStop { position: 0.0; color: GameBarLook.alpha(GameBarLook.content, 0.10) }
                    GradientStop { position: 1.0; color: GameBarLook.alpha(GameBarLook.content, 0.38) }
                }
            }
        }

        Rectangle {
            id: glassMask
            x: glass.x
            y: glass.y
            width: glass.width
            height: glass.height
            radius: GameBarLook.radiusSmall * card.s
            visible: false
            layer.enabled: glass.visible
        }

        Row {
            id: nowRow
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            height: 104 * card.s
            spacing: 14 * card.s
            visible: opacity > 0
            opacity: card.lyricsOpen ? 0 : 1

            Behavior on opacity {
                NumberAnimation { duration: Motion.dur(180); easing.type: Easing.OutCubic }
            }

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

        LyricsSheet {
            id: lyrics
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.bottom: transportRow.top
            anchors.leftMargin: 12 * card.s
            anchors.rightMargin: 12 * card.s
            anchors.topMargin: 4 * card.s
            anchors.bottomMargin: 6 * card.s
            visible: opacity > 0
            opacity: card.lyricsOpen ? 1 : 0
            s: card.s
            ink: GameBarLook.fg
            dim: GameBarLook.onLayer
            accent: GameBarLook.accent
            face: GameBarLook.face
            base: GameBarLook.smallie
            align: Text.AlignHCenter
            glow: true
            emptyText: card.present ? "" : I18n.tr("Nothing playing")

            Behavior on opacity {
                NumberAnimation { duration: Motion.dur(220); easing.type: Easing.OutCubic }
            }
        }

        Rectangle {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.leftMargin: -8 * card.s
            anchors.rightMargin: -8 * card.s
            anchors.topMargin: -8 * card.s
            height: 28 * card.s
            visible: card.lyricsOpen
            gradient: Gradient {
                GradientStop { position: 0.0; color: GameBarLook.alpha(GameBarLook.content, 0.55) }
                GradientStop { position: 1.0; color: "transparent" }
            }
        }

        Rectangle {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            anchors.leftMargin: -8 * card.s
            anchors.rightMargin: -8 * card.s
            anchors.bottomMargin: -8 * card.s
            height: 46 * card.s
            visible: card.lyricsOpen
            gradient: Gradient {
                GradientStop { position: 0.0; color: "transparent" }
                GradientStop { position: 1.0; color: GameBarLook.alpha(GameBarLook.content, 0.92) }
            }
        }

        Row {
            id: transportRow
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.bottom: parent.bottom
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
            Item {
                width: 30 * card.s
                height: 40 * card.s

                GameBarIconButton {
                    anchors.centerIn: parent
                    s: card.s
                    glyph: "text"
                    toggled: card.lyricsOpen
                    onClicked: card.lyricsOpen = !card.lyricsOpen
                }
            }
        }
    }
}
