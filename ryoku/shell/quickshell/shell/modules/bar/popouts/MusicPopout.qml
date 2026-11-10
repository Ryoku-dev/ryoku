pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Quickshell.Widgets
import Quickshell
import ".."
import shell.services
import "../../../components"
import "../../../utils/artcolor.js" as ArtColor
import Ryoku.Ui.Singletons

// Music card popout: grown from the frame edge off the rail's spectrum widget,
// framed like a bar blob (a crisp surface tile with the frame's own outline).
// The sleeve art leads, casting a soft glow in its own dominant colour; the
// title, artist, a wide spectrum sweep (the shared AudioBars feed the rail draws
// from) and the three transport moves follow, all tuned to the art's palette so
// the card wears the record's colour. The card never grabs the keyboard and
// dismisses on an outside click like every Ryoku surface. cava is claimed while
// open, so the strip keeps dancing even when the rail is out of view.
Item {
    id: root

    property real s: 1
    property bool open: false

    // Overridable so the card can be driven from a stub; defaults to the shared
    // media pick.
    property var player: Media.player
    property bool present: Media.present
    property bool playing: Media.playing
    // a live radio stream has no seekable length; the progress row hides.
    readonly property bool seekable: root.present && !Media.radio
        && root.player !== null && root.player.length > 0

    readonly property real pad: 14 * root.s
    readonly property real artSize: 160 * root.s

    implicitWidth: 264 * root.s
    implicitHeight: (root.present ? cardCol.implicitHeight : emptyCol.implicitHeight) + root.pad * 2

    function syncAudioClaim() {
        AudioBars.setActive(root, root.open && root.playing);
    }
    onOpenChanged: root.syncAudioClaim()
    onPlayingChanged: root.syncAudioClaim()
    Component.onCompleted: root.syncAudioClaim()
    Component.onDestruction: AudioBars.setActive(root, false)

    // ── album-art colour ────────────────────────────────────────────────────
    // Quantise the sleeve and lift its most vibrant tone (saturation x value) as
    // the card accent; the transport, progress and spectrum ramp all wear it, so
    // the card retunes to whatever is playing. Falls back to the palette accent
    // for art-less tracks or a muddy sleeve.
    ColorQuantizer {
        id: quant
        source: art.source
        depth: 3
        rescaleSize: 48
    }
    readonly property color artAccent: ArtColor.accentOf(quant.colors, Theme.primary)
    // a readable accent for ink-on-accent buttons, and the two ends of the ramp.
    readonly property color accentLo: Qt.darker(root.artAccent, 1.35)
    readonly property color accentHi: Qt.lighter(root.artAccent, 1.25)

    // the framed card skin + click-swallow, shared with every frame-edge card.
    PopoutCard { anchors.fill: parent }

    // "{h}:{mm}:{ss}" past an hour, else "{m}:{ss}".
    function fmtTime(sec) {
        sec = Math.max(0, Math.floor(sec));
        const h = Math.floor(sec / 3600);
        const m = Math.floor((sec % 3600) / 60);
        const r = sec % 60;
        const ss = (r < 10 ? "0" : "") + r;
        if (h >= 1)
            return h + ":" + (m < 10 ? "0" : "") + m + ":" + ss;
        return m + ":" + ss;
    }

    // MPRIS never pushes position, so re-read the stream on a tick while playing.
    readonly property real posn: root.player ? root.player.position : 0
    readonly property real len: root.seekable ? root.player.length : 0
    readonly property real frac: root.len > 0 ? Math.max(0, Math.min(1, root.posn / root.len)) : 0
    Timer {
        interval: 500
        repeat: true
        running: root.open && root.player !== null && root.player.isPlaying
        onTriggered: root.player.positionChanged()
    }

    // one transport move: a round tile, the play/pause move wears the art accent.
    component TButton: Item {
        id: tb
        property string glyph: ""
        property bool accent: false
        signal clicked()

        width: 42 * root.s
        height: 42 * root.s

        Rectangle {
            anchors.fill: parent
            radius: width / 2
            color: tb.accent ? root.artAccent
                : (tap.pressed ? Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.16)
                : (hover.hovered ? Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.08) : "transparent"))
            Behavior on color { ColorAnimation { duration: Motion.fast } }
        }
        GlyphIcon {
            anchors.centerIn: parent
            width: (tb.accent ? 20 : 18) * root.s
            height: width
            name: tb.glyph
            color: tb.accent ? Theme.ink(root.artAccent) : Theme.ink(Theme.effectiveSurface)
        }
        HoverHandler { id: hover; cursorShape: Qt.PointingHandCursor }
        MouseArea { id: tap; anchors.fill: parent; onClicked: tb.clicked() }
    }

    // ── the card ───────────────────────────────────────────────────────────
    Column {
        id: cardCol
        visible: root.present
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.margins: root.pad
        spacing: 10 * root.s

        // sleeve art: a square plate centred in the card, glowing in its own
        // dominant colour, seated with a hairline dark rim like a record sleeve.
        Item {
            width: root.artSize
            height: root.artSize
            anchors.horizontalCenter: parent.horizontalCenter

            // a soft colour seat in the sleeve's own tone, peeking around it.
            Rectangle {
                anchors.fill: artClip
                anchors.margins: -6 * root.s
                radius: 14 * root.s
                antialiasing: true
                color: Qt.rgba(root.artAccent.r, root.artAccent.g, root.artAccent.b, 0.13)
            }
            Rectangle {
                anchors.fill: artClip
                anchors.margins: -3 * root.s
                radius: 11 * root.s
                antialiasing: true
                color: Qt.rgba(root.artAccent.r, root.artAccent.g, root.artAccent.b, 0.22)
            }

            ClippingRectangle {
                id: artClip
                anchors.fill: parent
                radius: 8 * root.s
                color: Theme.surfaceContainer

                GlyphIcon {
                    visible: art.status !== Image.Ready
                    anchors.centerIn: parent
                    width: 46 * root.s
                    height: width
                    name: "music"
                    color: Theme.onSurfaceVariant
                    stroke: 1.4
                    opacity: 0.7
                }
                Image {
                    id: art
                    anchors.fill: parent
                    source: Music.coverFor(root.player)
                    asynchronous: true
                    cache: true
                    smooth: true
                    mipmap: true
                    sourceSize.width: Math.round(root.artSize * 2)
                    sourceSize.height: Math.round(root.artSize * 2)
                    fillMode: Image.PreserveAspectCrop
                }
            }
            Rectangle {
                anchors.fill: parent
                radius: 8 * root.s
                color: "transparent"
                border.width: 1.5 * root.s
                border.color: Qt.rgba(0, 0, 0, 0.35)
            }
            MouseArea {
                anchors.fill: parent
                cursorShape: Qt.PointingHandCursor
                enabled: root.player !== null && root.player.canTogglePlaying
                onClicked: Media.toggle()
            }
        }

        // title in the serif display face; wraps to two lines for a long title
        // instead of clipping at one, then elides. artist under it.
        Text {
            width: parent.width
            horizontalAlignment: Text.AlignHCenter
            text: root.player ? (root.player.trackTitle || "") : ""
            color: Theme.ink(Theme.effectiveSurface)
            font.family: Theme.display
            font.pixelSize: 18 * root.s
            font.weight: Font.DemiBold
            lineHeight: 0.95
            wrapMode: Text.WordWrap
            maximumLineCount: 2
            elide: Text.ElideRight
        }
        Text {
            width: parent.width
            visible: text.length > 0
            horizontalAlignment: Text.AlignHCenter
            text: root.player ? Theme.joinArtists(root.player.trackArtists, root.player.trackArtist) : ""
            color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
            font.family: Theme.fontPrimary
            font.pixelSize: Theme.fontSm * root.s
            elide: Text.ElideRight
        }

        // the wide sweep: the shared feed at card resolution, tinted to the art.
        MusicBars {
            width: parent.width
            height: 40 * root.s
            bands: 24
            s: root.s
            orient: "vertical"
            running: root.open && root.playing
            lowColor: root.accentLo
            highColor: root.accentHi
        }

        // progress hairline with mono timestamps; hidden on radio and streams
        // with no length.
        Item {
            width: parent.width
            visible: root.seekable
            height: visible ? Math.max(el.implicitHeight, tot.implicitHeight) : 0

            Text {
                id: el
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                text: root.fmtTime(root.posn)
                color: Theme.onSurfaceVariant
                font.family: Theme.mono
                font.pixelSize: 9.5 * root.s
            }
            Text {
                id: tot
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                text: root.fmtTime(root.len)
                color: Theme.onSurfaceVariant
                font.family: Theme.mono
                font.pixelSize: 9.5 * root.s
            }
            Rectangle {
                anchors.left: el.right
                anchors.right: tot.left
                anchors.leftMargin: 10 * root.s
                anchors.rightMargin: 10 * root.s
                anchors.verticalCenter: parent.verticalCenter
                height: 2 * root.s
                radius: height / 2
                color: Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.14)
                Rectangle {
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    width: parent.width * root.frac
                    height: parent.height
                    radius: height / 2
                    color: root.artAccent
                }
            }
        }

        // transport: previous, play/pause on the art accent, next.
        Row {
            anchors.horizontalCenter: parent.horizontalCenter
            spacing: 10 * root.s

            TButton {
                glyph: "prev"
                enabled: root.player !== null && root.player.canGoPrevious
                opacity: enabled ? 1 : 0.4
                onClicked: if (root.player) root.player.previous()
            }
            TButton {
                accent: true
                glyph: root.playing ? "pause" : "play"
                enabled: root.player !== null && root.player.canTogglePlaying
                opacity: enabled ? 1 : 0.4
                onClicked: Media.toggle()
            }
            TButton {
                glyph: "next"
                enabled: root.player !== null && root.player.canGoNext
                opacity: enabled ? 1 : 0.4
                onClicked: if (root.player) root.player.next()
            }
        }
    }

    // ── empty state ──────────────────────────────────────────────────────────
    Column {
        id: emptyCol
        visible: !root.present
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        spacing: 10 * root.s

        GlyphIcon {
            anchors.horizontalCenter: parent.horizontalCenter
            width: 40 * root.s
            height: width
            name: "music"
            color: Theme.onSurfaceVariant
            stroke: 1.4
            opacity: 0.6
        }
        Text {
            anchors.horizontalCenter: parent.horizontalCenter
            text: I18n.tr("Nothing playing")
            color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
            font.family: Theme.fontPrimary
            font.pixelSize: Theme.fontSm * root.s
        }
    }
}
