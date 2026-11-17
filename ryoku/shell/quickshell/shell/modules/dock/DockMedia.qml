pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Quickshell
import Quickshell.Widgets
import shell.services
import "../../components"
import "../../utils/artcolor.js" as ArtColor

// The dock's optional media chip: a compact frosted tile at the end of the band
// carrying the album art, the track line and the transport moves. It reads the
// shared Media pick every other Ryoku surface uses (MusicPopout is the fuller
// card) and tints its transport from the sleeve, so the chip wears the record's
// colour. Horizontal bands get art + text + buttons in a row; a vertical band is
// too narrow for a title, so it stacks the art over the two moves.
Item {
    id: media

    required property var band
    readonly property bool horizontal: media.band.horizontal
    readonly property real depth: media.band.baseSize
    // Exposed so the band suppresses its app preview while the chip is hovered.
    readonly property bool chipHovered: chipHover.hovered

    readonly property var player: Media.player
    readonly property bool playing: Media.playing
    readonly property string artUrl: Music.coverFor(media.player)
    readonly property real art: media.depth - 10

    // Album-art accent (MusicPopout's approach): quantise the sleeve and lift its
    // most vibrant tone, falling back to the palette accent for art-less tracks.
    ColorQuantizer {
        id: quant
        source: media.artUrl
        depth: 3
        rescaleSize: 48
    }
    readonly property color accent: ArtColor.accentOf(quant.colors, Theme.primary)

    implicitWidth: media.horizontal ? row.implicitWidth + 12 : media.depth
    implicitHeight: media.horizontal ? media.depth : col.implicitHeight + 12

    Rectangle {
        id: chip
        anchors.fill: parent
        radius: media.band.radius
        color: Qt.rgba(Theme.surfaceContainer.r, Theme.surfaceContainer.g, Theme.surfaceContainer.b, media.band.frost ? 0.6 : 0.94)
        border.width: 1
        border.color: Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.12)

        RectangularShadow {
            anchors.fill: parent
            radius: parent.radius
            blur: 12
            spread: 0
            offset: media.horizontal
                ? Qt.vector2d(0, media.band.edge === "bottom" ? -2 : 2)
                : Qt.vector2d(media.band.edge === "right" ? -2 : 2, 0)
            color: Qt.rgba(Theme.shadow.r, Theme.shadow.g, Theme.shadow.b, 0.5)
            visible: media.band.shadow && !Perf.shadowsDisabled
            z: -1
        }
    }

    // A rounded sleeve tile with an art image and a placeholder glyph.
    component Cover: ClippingRectangle {
        radius: 5
        color: Theme.surfaceContainerHigh
        Image {
            id: cover
            anchors.fill: parent
            source: media.artUrl
            fillMode: Image.PreserveAspectCrop
            sourceSize.width: Math.round(media.art * 2)
            sourceSize.height: Math.round(media.art * 2)
            smooth: true
            cache: true
            asynchronous: true
        }
        GlyphIcon {
            visible: cover.status !== Image.Ready
            anchors.centerIn: parent
            width: parent.width * 0.5
            height: width
            name: "music"
            color: Theme.onSurfaceVariant
            stroke: 1.4
        }
    }

    // Transport move: a round tap target; the play/pause move wears the accent.
    component TBtn: Item {
        id: tb
        property string glyph: ""
        property bool accentFill: false
        signal act()
        width: 22
        height: 22
        Rectangle {
            anchors.fill: parent
            radius: width / 2
            color: tb.accentFill ? media.accent
                : (tbHover.hovered ? Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.1) : "transparent")
        }
        GlyphIcon {
            anchors.centerIn: parent
            width: tb.accentFill ? 13 : 12
            height: width
            name: tb.glyph
            color: tb.accentFill ? Theme.ink(media.accent) : Theme.onSurface
        }
        HoverHandler { id: tbHover; cursorShape: Qt.PointingHandCursor }
        TapHandler { onSingleTapped: tb.act() }
    }

    // ── horizontal band: art · title/artist · play-pause + next ──────────────
    Row {
        id: row
        visible: media.horizontal
        anchors.verticalCenter: parent.verticalCenter
        anchors.left: parent.left
        anchors.leftMargin: 6
        spacing: 6

        Cover {
            anchors.verticalCenter: parent.verticalCenter
            width: media.art
            height: media.art
        }

        Column {
            anchors.verticalCenter: parent.verticalCenter
            width: 118
            spacing: 0
            Text {
                width: parent.width
                text: media.player ? (media.player.trackTitle || "") : ""
                color: Theme.onSurface
                font.family: Theme.fontPrimary
                font.pixelSize: 11
                font.weight: Font.DemiBold
                elide: Text.ElideRight
            }
            Text {
                width: parent.width
                visible: text.length > 0
                text: media.player ? Theme.joinArtists(media.player.trackArtists, media.player.trackArtist) : ""
                color: Theme.onSurfaceVariant
                font.family: Theme.fontPrimary
                font.pixelSize: 10
                elide: Text.ElideRight
            }
        }

        Row {
            anchors.verticalCenter: parent.verticalCenter
            spacing: 2
            TBtn {
                accentFill: true
                glyph: media.playing ? "pause" : "play"
                onAct: Media.toggle()
            }
            TBtn {
                glyph: "next"
                enabled: media.player !== null && media.player.canGoNext
                opacity: enabled ? 1 : 0.4
                onAct: if (media.player) media.player.next()
            }
        }
    }

    // ── vertical band: art over the two moves (no room for a title) ──────────
    Column {
        id: col
        visible: !media.horizontal
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.top: parent.top
        anchors.topMargin: 6
        spacing: 5

        Cover {
            anchors.horizontalCenter: parent.horizontalCenter
            width: media.art
            height: media.art
        }
        Row {
            anchors.horizontalCenter: parent.horizontalCenter
            spacing: 2
            TBtn {
                accentFill: true
                glyph: media.playing ? "pause" : "play"
                onAct: Media.toggle()
            }
            TBtn {
                glyph: "next"
                enabled: media.player !== null && media.player.canGoNext
                opacity: enabled ? 1 : 0.4
                onAct: if (media.player) media.player.next()
            }
        }
    }

    // Hovering the chip opens the music card via MusicPreview; DockBand suppresses
    // the app window-preview over the chip's span.
    function syncMusicPreview() {
        if (chipHover.hovered) {
            const c = media.mapToGlobal(media.width / 2, media.height / 2);
            MusicPreview.gx = c.x;
            MusicPreview.gy = c.y;
            MusicPreview.edge = media.band.edge;
            MusicPreview.margin = media.band.reservedDepth + 14;
            MusicPreview.hovered = true;
        } else if (MusicPreview.hovered) {
            MusicPreview.hovered = false;
        }
    }
    HoverHandler {
        id: chipHover
        onHoveredChanged: media.syncMusicPreview()
    }
}
