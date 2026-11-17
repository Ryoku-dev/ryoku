pragma ComponentBehavior: Bound
import QtQuick
import Quickshell
import Quickshell.Widgets
import shell.services
import ".."
import "../Singletons"
import "../../../components"
import "../../../utils/artcolor.js" as ArtColor
import Ryoku.Ui.Singletons

// The wallpaper's now-playing sheet: the sleeve leads, this song's lyrics run
// beside it under the line being sung, and the track, its clock, a wavy seek rail
// and the three transport moves close it out. Everything but placement comes from
// two services: Media picks the player, Music carries the daemon-resolved cover
// and lyrics plus the shared playback clock.
//
// The card wears the sleeve's colour, not the theme's: album art is data
// (docs/ui-ux.md), so the accent, the lit part of the rail and the play tile all
// retune per song.
Item {
    id: root

    property string style: "cover"          // cover | glass
    property bool showLyrics: true
    property bool active: true
    property string musicApp: ""            // corner-button launch command (from Config)
    property real s: 1
    property string viz: "bars"             // bars | wave (the no-lyrics visualiser look)
    property real underL: Scheme.wallLstar
    // WidgetSlot pins a hex here to paint this widget's ink; "" keeps the palette ink.
    // Only the theme inks follow it; the per-song accent + plate stay album data.
    property string inkColorA: ""
    property Item wallpaperSource: null
    property rect wallpaperRect: Qt.rect(0, 0, 0, 0)
    property string shape: "wide"           // wide | tall (9:16 canvas)
    property string videoMode: "canvas"     // off | canvas | custom (default: the track's Spotify Canvas, falls back to the cover when none)
    property string videoFile: ""           // custom backdrop file (video/gif)
    readonly property bool tall: root.shape === "tall"
    // the backdrop the surfaces play: the track's Spotify Canvas, a custom
    // file, or nothing (then the cover carries the picture).
    readonly property string videoSource: root.videoMode === "canvas" ? Music.canvas
        : (root.videoMode === "custom" ? root.videoFile : "")

    readonly property var player: Media.player
    readonly property bool present: Media.present
    // a live broadcast has no position to seek along.
    readonly property bool seekable: root.present && !Media.radio
        && root.player !== null && root.player.length > 0
    readonly property real length: root.seekable ? root.player.length : 0

    readonly property real pad: 16 * root.s
    readonly property real gap: 14 * root.s
    readonly property real coverSize: 168 * root.s
    readonly property real infoH: 42 * root.s
    readonly property real seekH: 38 * root.s

    implicitWidth: root.tall ? 320 * root.s : 560 * root.s
    implicitHeight: root.tall ? Math.round(320 * 16 / 9) * root.s
        : root.pad * 2 + root.coverSize + root.gap + root.infoH + 8 * root.s + root.seekH

    // The playback clock and the lyric tracker run only while a surface wants
    // them, so a disabled or covered widget costs nothing.
    readonly property bool wanted: root.active && root.visible
    onWantedChanged: Music.hold(root, root.wanted)
    Component.onCompleted: Music.hold(root, root.wanted)
    Component.onDestruction: Music.hold(root, false)

    readonly property string artSource: Music.coverFor(root.player)

    ColorQuantizer {
        id: quant
        source: root.artSource
        depth: 3
        rescaleSize: 48
    }
    readonly property color accent: ArtColor.accentOf(quant.colors, Scheme.accent)
    readonly property color ink: root.inkColorA !== "" ? root.inkColorA : Theme.ink
    readonly property color dim: root.inkColorA !== "" ? Qt.rgba(Qt.color(root.inkColorA).r, Qt.color(root.inkColorA).g, Qt.color(root.inkColorA).b, 0.7) : Theme.inkDim
    readonly property color plate: ArtColor.plateOf(root.accent, Theme.surface)
    readonly property bool hasLyrics: Music.synced || Music.unsynced
    // Hysteresis for the side area so lyrics and the visualizer never strobe: a
    // re-lookup or a brief empty frame keeps the current surface, and the
    // visualizer only takes over once a track has settled with no lyrics. Lyrics
    // reclaim the area the instant they arrive.
    readonly property bool lyricsPending: Music.searching || root.hasLyrics
    property bool noLyricsSettled: false
    onLyricsPendingChanged: if (root.lyricsPending) root.noLyricsSettled = false
    readonly property bool vizShown: root.present && (!root.showLyrics || root.noLyricsSettled)
    Timer {
        interval: 700
        running: root.present && root.showLyrics && !root.lyricsPending && !root.noLyricsSettled
        onTriggered: root.noLyricsSettled = true
    }

    HoverHandler { id: hover }

    Loader {
        anchors.fill: parent
        active: !root.tall || !root.present
        sourceComponent: root.style === "glass" ? glassPlate : coverPlate
    }
    Component {
        id: coverPlate
        MusicCard {
            art: root.artSource
            accent: root.accent
            s: root.s
            hovered: hover.hovered
            plate: root.plate
            video: root.videoSource
            playing: Media.playing
        }
    }
    Component {
        id: glassPlate
        WidgetGlass {
            hovered: hover.hovered
            s: root.s
            sourceItem: root.wallpaperSource
            sourceRect: root.wallpaperRect
        }
    }

    // ── nothing playing ─────────────────────────────────────────────────────
    Column {
        anchors.centerIn: parent
        visible: !root.present
        spacing: 10 * root.s

        GlyphIcon {
            anchors.horizontalCenter: parent.horizontalCenter
            width: 40 * root.s
            height: width
            name: "music"
            color: root.dim
            stroke: 1.4
            opacity: 0.7
        }
        Text {
            anchors.horizontalCenter: parent.horizontalCenter
            text: I18n.tr("Nothing playing")
            color: root.dim
            font.family: Theme.font
            font.pixelSize: 13 * root.s
        }
    }

    // ── the sheet ───────────────────────────────────────────────────────────
    Item {
        id: body
        anchors.fill: parent
        anchors.margins: root.pad
        visible: root.present && !root.tall

        MusicCover {
            id: cover
            x: 0
            y: 0
            width: root.coverSize
            height: root.coverSize
            radius: 10 * root.s
            source: root.artSource
            plate: Theme.tile
            ink: root.dim
            s: root.s
        }
        // now-playing pulse on the sleeve, only while sound is actually moving.
        Rectangle {
            visible: root.present && Media.playing
            x: 8 * root.s
            y: cover.height - height - 8 * root.s
            width: pulse.implicitWidth + 14 * root.s
            height: 20 * root.s
            radius: height / 2
            color: Qt.rgba(0, 0, 0, 0.42)
            MusicPulse {
                id: pulse
                anchors.centerIn: parent
                s: root.s
                live: Media.playing
                color: "white"
            }
        }

        LyricsSheet {
            x: cover.width + root.gap
            y: 0
            width: body.width - cover.width - root.gap
            height: root.coverSize
            visible: root.showLyrics && root.hasLyrics
            s: root.s
            ink: root.ink
            dim: root.dim
            accent: root.accent
            face: Theme.font
        }

        // no lyric sheet up (lyrics switched off, or none found for this track):
        // a live cava spectrum takes the area instead, in the sleeve's colour.
        MusicViz {
            x: cover.width + root.gap
            y: 0
            width: body.width - cover.width - root.gap
            height: root.coverSize
            visible: root.vizShown
            s: root.s
            accent: root.accent
            live: Media.playing
            look: root.viz
        }


        Item {
            id: info
            x: 0
            y: root.coverSize + root.gap
            width: body.width
            height: root.infoH

            Column {
                anchors.left: parent.left
                anchors.right: stamp.left
                anchors.rightMargin: 12 * root.s
                anchors.verticalCenter: parent.verticalCenter
                spacing: 2 * root.s

                Text {
                    width: parent.width
                    text: Music.title
                    color: root.ink
                    elide: Text.ElideRight
                    font.family: Theme.display
                    font.pixelSize: 20 * root.s
                    font.weight: Font.DemiBold
                }
                Text {
                    width: parent.width
                    visible: text.length > 0
                    text: Music.artist
                    color: root.dim
                    elide: Text.ElideRight
                    font.family: Theme.font
                    font.pixelSize: 12.5 * root.s
                }
            }

            Text {
                id: stamp
                anchors.right: parent.right
                anchors.bottom: parent.bottom
                anchors.bottomMargin: 2 * root.s
                visible: root.seekable
                text: Music.stamp(Music.elapsed) + " / " + Music.stamp(root.length)
                color: root.dim
                font.family: Theme.mono
                font.pixelSize: 11 * root.s
            }
        }

        Item {
            id: controls
            x: 0
            y: info.y + info.height + 8 * root.s
            width: body.width
            height: root.seekH

            MusicSeek {
                anchors.left: parent.left
                anchors.right: moves.left
                anchors.rightMargin: 14 * root.s
                anchors.verticalCenter: parent.verticalCenter
                visible: root.seekable
                s: root.s
                frac: root.length > 0 ? Music.elapsed / root.length : 0
                live: Media.playing
                accent: root.accent
                track: Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.16)
                seekable: root.player !== null && root.player.canSeek
                onSeekRequested: (fraction) => Music.seek(fraction * root.length)
            }

            // a broadcast has no rail, so its name takes the room instead.
            Text {
                anchors.left: parent.left
                anchors.right: moves.left
                anchors.rightMargin: 14 * root.s
                anchors.verticalCenter: parent.verticalCenter
                visible: !root.seekable
                text: Media.radio ? I18n.tr("Live") : ""
                color: root.dim
                elide: Text.ElideRight
                font.family: Theme.mono
                font.pixelSize: 11 * root.s
                font.letterSpacing: 1.1 * root.s
            }

            MusicTransport {
                id: moves
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                s: root.s
                accent: root.accent
                ink: root.ink
                inkOnAccent: Theme.surface
                playing: Media.playing
                canPrevious: root.player !== null && root.player.canGoPrevious
                canNext: root.player !== null && root.player.canGoNext
                canToggle: root.player !== null && root.player.canTogglePlaying
                onPrevious: if (root.player) root.player.previous()
                onNext: if (root.player) root.player.next()
                onToggle: Media.toggle()
            }
        }
    }

    // the 9:16 canvas sheet: a full-bleed backdrop with the now-playing overlaid.
    Loader {
        anchors.fill: parent
        active: root.tall
        visible: root.present && root.tall
        sourceComponent: tallComp
    }
    Component {
        id: tallComp
        MusicTall {
            s: root.s
            inkColorA: root.inkColorA
            accent: root.accent
            plate: root.plate
            videoSource: root.videoSource
            coverSource: root.artSource
            showLyrics: root.showLyrics
        }
    }

    // corner button: opens the music app (default ryotunes / YouTube Music, or
    // whatever the user picked in Desktop Widgets). Above the sheet, so it works
    // whether or not a track is playing.
    Item {
        id: openBtn
        width: 26 * root.s
        height: width
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.margins: root.pad
        opacity: hover.hovered || tapOpen.pressed ? 1 : 0.5
        Behavior on opacity { NumberAnimation { duration: Theme.quick } }

        Rectangle {
            anchors.fill: parent
            radius: width / 2
            color: openHover.hovered ? Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.12) : "transparent"
            Behavior on color { ColorAnimation { duration: Theme.quick } }
        }
        GlyphIcon {
            anchors.centerIn: parent
            width: 14 * root.s
            height: width
            name: "music"
            color: root.ink
            stroke: 1.5
        }
        scale: tapOpen.pressed ? 0.9 : 1
        Behavior on scale { NumberAnimation { duration: Theme.quick; easing.type: Theme.ease } }

        HoverHandler { id: openHover; cursorShape: Qt.PointingHandCursor }
        TapHandler {
            id: tapOpen
            onTapped: {
                const cmd = (root.musicApp && root.musicApp.length > 0) ? root.musicApp : "ryotunes";
                Spawn.runApp(["sh", "-c", cmd], { label: cmd.split(/\s+/)[0] || "music" });
            }
        }
    }
}
