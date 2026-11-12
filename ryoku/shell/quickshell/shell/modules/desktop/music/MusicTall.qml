pragma ComponentBehavior: Bound
import QtQuick
import Quickshell.Widgets
import shell.services
import "../Singletons"
import "../../../components"
import Ryoku.Ui.Singletons

// The 9:16 "canvas" now-playing sheet: a full-bleed backdrop -- the track's
// Spotify Canvas, a chosen video/gif, or the cover blown up -- with the line
// being sung floating over it and a compact transport bar seated at the foot,
// all under a legibility gradient. The portrait sibling of the wide sheet:
// same two services, same per-song colour, the art carrying the picture.
Item {
    id: tall

    property real s: 1
    property color accent: Theme.accent
    property color plate: Theme.surface
    property string videoSource: ""
    property string coverSource: ""
    property bool showLyrics: true
    // WidgetSlot pins a hex here to paint this face's ink; "" keeps the reference white.
    property string inkColorA: ""
    readonly property color ink: tall.inkColorA !== "" ? tall.inkColorA : "white"

    readonly property real pad: 16 * tall.s
    readonly property color dim: tall.inkColorA !== "" ? Qt.rgba(Qt.color(tall.inkColorA).r, Qt.color(tall.inkColorA).g, Qt.color(tall.inkColorA).b, 0.72) : Qt.rgba(1, 1, 1, 0.72)
    readonly property var player: Media.player
    readonly property bool seekable: Media.present && !Media.radio
        && tall.player !== null && tall.player.length > 0
    readonly property real length: tall.seekable ? tall.player.length : 0
    readonly property string curLine: (Music.synced && Music.index >= 0 && Music.lines[Music.index])
        ? Music.lines[Music.index].text : ""

    ClippingRectangle {
        anchors.fill: parent
        radius: Theme.radiusWidget * tall.s
        color: tall.plate
        border.width: 1
        border.color: Theme.line

        // the moving field: video/gif when set, else the cover blown up to fill.
        MusicBackdrop {
            anchors.fill: parent
            radius: 0
            source: tall.videoSource.length > 0 ? tall.videoSource : tall.coverSource
            active: tall.visible
        }

        // legibility: darken the foot (transport/title) and a touch of the head
        // (the corner button lives above), leaving the middle for the picture.
        Rectangle {
            anchors.fill: parent
            gradient: Gradient {
                GradientStop { position: 0.0; color: Qt.rgba(0, 0, 0, 0.38) }
                GradientStop { position: 0.28; color: Qt.rgba(0, 0, 0, 0.0) }
                GradientStop { position: 0.60; color: Qt.rgba(0, 0, 0, 0.0) }
                GradientStop { position: 1.0; color: Qt.rgba(0, 0, 0, 0.88) }
            }
        }

        // the line being sung, floating just above the foot, Canvas-style.
        Text {
            visible: tall.showLyrics && tall.curLine.length > 0
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.leftMargin: tall.pad
            anchors.rightMargin: tall.pad
            anchors.bottom: foot.top
            anchors.bottomMargin: 16 * tall.s
            textFormat: Text.PlainText
            text: tall.curLine
            color: tall.ink
            wrapMode: Text.WordWrap
            maximumLineCount: 3
            elide: Text.ElideRight
            font.family: Theme.display
            font.pixelSize: 22 * tall.s
            font.weight: Font.DemiBold
            style: Text.Raised
            styleColor: Qt.rgba(0, 0, 0, 0.55)
        }

        // foot: title / artist, the seek rail and the three transport moves.
        Column {
            id: foot
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            anchors.leftMargin: tall.pad
            anchors.rightMargin: tall.pad
            anchors.bottomMargin: tall.pad
            spacing: 8 * tall.s

            Text {
                width: parent.width
                text: Music.title
                color: tall.ink
                elide: Text.ElideRight
                font.family: Theme.display
                font.pixelSize: 19 * tall.s
                font.weight: Font.DemiBold
                style: Text.Raised
                styleColor: Qt.rgba(0, 0, 0, 0.5)
            }
            Text {
                width: parent.width
                visible: text.length > 0
                text: Music.artist
                color: tall.dim
                elide: Text.ElideRight
                font.family: Theme.font
                font.pixelSize: 12.5 * tall.s
            }

            Item {
                width: parent.width
                height: 34 * tall.s

                MusicSeek {
                    anchors.left: parent.left
                    anchors.right: moves.left
                    anchors.rightMargin: 12 * tall.s
                    anchors.verticalCenter: parent.verticalCenter
                    visible: tall.seekable
                    s: tall.s
                    frac: tall.length > 0 ? Music.elapsed / tall.length : 0
                    live: Media.playing
                    accent: tall.accent
                    track: Qt.rgba(tall.ink.r, tall.ink.g, tall.ink.b, 0.26)
                    seekable: tall.player !== null && tall.player.canSeek
                    onSeekRequested: (fraction) => Music.seek(fraction * tall.length)
                }

                Text {
                    anchors.left: parent.left
                    anchors.right: moves.left
                    anchors.rightMargin: 12 * tall.s
                    anchors.verticalCenter: parent.verticalCenter
                    visible: !tall.seekable
                    text: Media.radio ? I18n.tr("Live") : ""
                    color: tall.dim
                    elide: Text.ElideRight
                    font.family: Theme.mono
                    font.pixelSize: 11 * tall.s
                    font.letterSpacing: 1.1 * tall.s
                }

                MusicTransport {
                    id: moves
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    s: tall.s
                    accent: tall.accent
                    ink: tall.ink
                    inkOnAccent: Theme.surface
                    playing: Media.playing
                    canPrevious: tall.player !== null && tall.player.canGoPrevious
                    canNext: tall.player !== null && tall.player.canGoNext
                    canToggle: tall.player !== null && tall.player.canTogglePlaying
                    onPrevious: if (tall.player) tall.player.previous()
                    onNext: if (tall.player) tall.player.next()
                    onToggle: Media.toggle()
                }
            }
        }
    }
}
