import QtQuick
import QtQuick.Effects
import Quickshell.Services.Mpris
import shell.services
import Ryoku.Ui.Singletons
import "../kit"
import "../IslandMetrics.js" as Island

Rectangle {
    id: surface

    required property var player
    // 0 is the bubble, 1 the panel; a spring may carry it a little past either end
    required property real full

    property color ink: Md.ink
    property color accent: Md.accent

    // the island keeps the panel open while a drag strays off it
    readonly property bool scrubbing: seekBar.scrubbing
    readonly property bool clockHeld: surface.full > 0.5
    readonly property bool seekable: surface.player !== null
        && surface.player.canSeek && surface.length > 0

    readonly property real shown: Math.max(0, Math.min(1, surface.full))
    // content arrives late on open and is the first thing to leave on close
    readonly property real fullVis: Math.max(0, Math.min(1, (surface.full - 0.4) / 0.45))
    readonly property string artUrl: Music.coverFor(surface.player)
    readonly property bool hasArt: surface.artUrl.length > 0
    readonly property bool playing: surface.player !== null && surface.player.isPlaying
    readonly property real length: Music.duration
    readonly property real position: Music.position
    readonly property real fraction: surface.length > 0
        ? Math.max(0, Math.min(1, surface.position / surface.length)) : 0
    readonly property bool backdropActive: surface.full > 0.5
        && surface.hasArt && !Perf.blurDisabled
    readonly property int coverDecodeSize: Math.max(1,
        Math.ceil(Island.coverOpen * Screen.devicePixelRatio))
    readonly property size backdropDecodeSize: Qt.size(
        Math.max(1, Math.ceil((Island.musicWidth - 2 * Island.framePad)
            * Screen.devicePixelRatio)),
        Math.max(1, Math.ceil((Island.musicHeight - 2 * Island.framePad)
            * Screen.devicePixelRatio)))
    readonly property string source: {
        if (!surface.player)
            return "";
        return String(surface.player.identity
            || surface.player.dbusName || "").replace(/^org\.mpris\.MediaPlayer2\./, "");
    }
    readonly property string outputName: Audio.sink ? Audio.nodeLabel(Audio.sink) : ""
    readonly property bool canShuffle: surface.player !== null && surface.player.shuffleSupported
    readonly property bool canLoop: surface.player !== null && surface.player.loopSupported

    readonly property int controlSize: 40
    readonly property int playFull: 56
    readonly property int controlGap: 12
    readonly property real fullPlayX: (Island.musicWidth - surface.playFull) / 2
    readonly property real fullNextX: surface.fullPlayX + surface.playFull + surface.controlGap
    readonly property real fullPrevX: surface.fullPlayX - surface.controlGap - surface.controlSize
    readonly property real fullControlY: Island.musicTransportY + (surface.playFull - surface.controlSize) / 2

    // past the bubble end the spring shrinks the bubble by its own size, so the
    // settle is a small pulse rather than a fraction of the whole panel
    function span(bubble, open) {
        return surface.full >= 0 ? bubble + (open - bubble) * surface.full : bubble * (1 + surface.full);
    }

    // off, queue, track, off, as Android cycles it
    function cycleLoop() {
        if (surface.player.loopState === MprisLoopState.None)
            surface.player.loopState = MprisLoopState.Playlist;
        else if (surface.player.loopState === MprisLoopState.Playlist)
            surface.player.loopState = MprisLoopState.Track;
        else
            surface.player.loopState = MprisLoopState.None;
    }

    onClockHeldChanged: Music.holdClock(surface, surface.clockHeld)
    Component.onCompleted: Music.holdClock(surface, surface.clockHeld)
    Component.onDestruction: Music.holdClock(surface, false)

    width: surface.span(Island.bubbleSize, Island.musicWidth)
    height: surface.span(Island.bubbleSize, Island.musicHeight)
    radius: surface.span(Island.bubbleSize / 2, Island.musicRadius)
    // the island draws the body, one liquid with the clock
    color: "transparent"
    clip: true

    // tapping the media control opens its app, as on Android
    MouseArea {
        anchors.fill: parent
        enabled: surface.player !== null && surface.player.canRaise
        cursorShape: Qt.PointingHandCursor
        onClicked: surface.player.raise()
    }

    // one layer so the blur and the rounded mask apply to the whole wash at once
    Item {
        id: glass
        anchors.fill: parent
        anchors.margins: Island.framePad
        visible: surface.shown > 0.001
        opacity: surface.shown
        readonly property real radius: Math.max(0, surface.radius - Island.framePad)
        layer.enabled: surface.backdropActive
        layer.smooth: surface.backdropActive
        layer.effect: MultiEffect {
            maskEnabled: surface.backdropActive
            maskSource: glassMask
            autoPaddingEnabled: false
            blurEnabled: surface.backdropActive
            blur: 1.0
            blurMax: 64
            saturation: 0.4
            brightness: -0.05
        }

        Rectangle {
            anchors.fill: parent
            radius: glass.radius
            color: Qt.darker(surface.accent, 1.4)
        }
        Image {
            anchors.fill: parent
            source: surface.artUrl
            sourceSize: surface.backdropDecodeSize
            visible: surface.backdropActive
            fillMode: Image.PreserveAspectCrop
            asynchronous: true
            cache: true
        }
        Rectangle {
            anchors.fill: parent
            radius: glass.radius
            color: surface.accent
            opacity: surface.hasArt ? 0.1 : 0.18
        }
        Rectangle {
            anchors.fill: parent
            radius: glass.radius
            gradient: Gradient {
                GradientStop { position: 0.0; color: Qt.rgba(0, 0, 0, 0.34) }
                GradientStop { position: 0.55; color: Qt.rgba(0, 0, 0, 0.46) }
                GradientStop { position: 1.0; color: Qt.rgba(0, 0, 0, 0.62) }
            }
        }
    }

    // a sibling so the group's layer can sample it without masking the mask
    Rectangle {
        id: glassMask
        x: glass.x
        y: glass.y
        width: glass.width
        height: glass.height
        radius: glass.radius
        visible: false
        layer.enabled: surface.backdropActive
    }

    RectangularShadow {
        anchors.fill: cover
        radius: cover.radius
        blur: 16
        spread: 0
        offset: Qt.vector2d(0, 4)
        color: Qt.rgba(0, 0, 0, 0.5)
        opacity: surface.shown
    }

    Item {
        id: cover
        x: Island.coverX * surface.shown
        y: Island.coverY * surface.shown
        width: surface.span(Island.bubbleSize, Island.coverOpen)
        height: width
        readonly property real radius: Math.max(0, surface.span(Island.bubbleSize / 2, Island.coverRadius))

        Rectangle {
            anchors.fill: parent
            radius: cover.radius
            color: Md.accentContainer
            visible: !surface.hasArt
        }

        // a mask layer, not ShapePath.fillItem: the invisible Image source never repainted once art arrived
        Rectangle {
            id: coverMask
            anchors.fill: parent
            radius: cover.radius
            visible: false
            layer.enabled: surface.hasArt
        }

        Image {
            anchors.fill: parent
            source: surface.artUrl
            sourceSize: Qt.size(surface.coverDecodeSize, surface.coverDecodeSize)
            visible: surface.hasArt
            fillMode: Image.PreserveAspectCrop
            asynchronous: true
            cache: true
            layer.enabled: surface.hasArt
            layer.smooth: true
            layer.effect: MultiEffect { maskEnabled: true; maskSource: coverMask }
        }

        Icon {
            anchors.centerIn: parent
            visible: !surface.hasArt
            name: "music_note"
            filled: true
            size: Math.round(cover.height * 0.42)
            color: Md.onAccentContainer
        }

        Rectangle {
            anchors.fill: parent
            radius: cover.radius
            color: "transparent"
            border.width: 2
            border.color: Md.base
        }
    }

    Item {
        id: meta
        x: Island.coverX + Island.coverOpen + 16
        y: Island.coverY
        width: Island.musicWidth - x - Island.coverX
        height: Island.coverOpen
        opacity: surface.fullVis
        visible: opacity > 0.01

        Label {
            id: sourceLabel
            anchors.left: parent.left
            anchors.right: output.visible ? output.left : parent.right
            anchors.rightMargin: output.visible ? 12 : 0
            anchors.top: parent.top
            height: 20
            role: "label"
            color: Md.alpha(Md.ink, 0.82)
            text: surface.source
        }

        Row {
            id: output
            anchors.right: parent.right
            anchors.verticalCenter: sourceLabel.verticalCenter
            spacing: 4
            visible: surface.outputName.length > 0

            Icon {
                id: outputIcon
                anchors.verticalCenter: parent.verticalCenter
                name: Audio.sinkIsBluez ? "headphones" : "speaker"
                size: 16
                color: Md.alpha(Md.ink, 0.82)
            }
            Label {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.min(implicitWidth, meta.width * 0.5 - outputIcon.width - output.spacing)
                role: "caption"
                color: Md.alpha(Md.ink, 0.82)
                text: surface.outputName
            }
        }

        Column {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            spacing: 2

            Label {
                width: parent.width
                role: "title"
                font.pixelSize: 19
                text: Music.title
            }
            Label {
                width: parent.width
                role: "body"
                color: Md.alpha(Md.ink, 0.82)
                text: Music.artist
            }
            Label {
                width: parent.width
                visible: Music.album.length > 0
                role: "caption"
                color: Md.alpha(Md.ink, 0.6)
                text: Music.album
            }
        }
    }

    Row {
        x: Island.coverX
        y: Island.musicSeekY
        width: Island.musicWidth - 2 * Island.coverX
        height: 24
        spacing: 8
        opacity: surface.fullVis
        visible: opacity > 0.01

        Label {
            id: elapsed
            anchors.verticalCenter: parent.verticalCenter
            width: 40
            horizontalAlignment: Text.AlignRight
            role: "caption"
            font.family: Md.mono
            font.pixelSize: 11
            color: Md.alpha(Md.ink, 0.82)
            text: Music.stamp(seekBar.scrubbing ? seekBar.scrub * surface.length : surface.position)
        }
        Wavy {
            id: seekBar
            anchors.verticalCenter: parent.verticalCenter
            width: parent.width - elapsed.width - total.width - 2 * parent.spacing
            height: 24
            progress: surface.fraction
            playing: surface.playing
            seekable: surface.seekable
            trackColor: Md.alpha(Md.ink, 0.26)
            onSeek: function (fraction) { Music.seekPosition(fraction * surface.length); }
        }
        Label {
            id: total
            anchors.verticalCenter: parent.verticalCenter
            width: 40
            role: "caption"
            font.family: Md.mono
            font.pixelSize: 11
            color: Md.alpha(Md.ink, 0.82)
            text: surface.length > 0 ? Music.stamp(surface.length) : ""
        }
    }

    IconButton {
        x: surface.fullPrevX - surface.controlGap - surface.controlSize
        y: surface.fullControlY
        size: surface.controlSize
        icon: "shuffle"
        tip: I18n.tr("Shuffle")
        checked: surface.canShuffle && surface.player.shuffle
        opacity: surface.fullVis
        visible: surface.canShuffle && surface.fullVis > 0.01
        onClicked: surface.player.shuffle = !surface.player.shuffle
    }

    IconButton {
        x: surface.fullPrevX
        y: surface.fullControlY
        size: surface.controlSize
        icon: "skip_previous"
        tip: I18n.tr("Previous")
        enabled: surface.player !== null && surface.player.canGoPrevious
        opacity: surface.fullVis * (enabled ? 1 : 0.38)
        visible: surface.fullVis > 0.01
        onClicked: surface.player.previous()
    }

    Rectangle {
        id: play
        readonly property real shapeT: playHit.pressed ? 1.25 : surface.playing ? 1 : 0
        property real squareness: play.shapeT
        Behavior on squareness {
            enabled: !Md.still
            Spring { speed: "fast" }
        }

        x: surface.fullPlayX
        y: Island.musicTransportY
        width: surface.playFull
        height: width
        radius: width * (0.5 - 0.25 * play.squareness)
        color: Md.accent
        opacity: surface.fullVis
        visible: surface.fullVis > 0.01

        Accessible.role: Accessible.Button
        Accessible.name: surface.playing ? I18n.tr("Pause") : I18n.tr("Play")

        Icon {
            anchors.centerIn: parent
            name: surface.playing ? "pause" : "play_arrow"
            size: 28
            filled: true
            color: Md.onAccent
        }
        StateLayer {
            id: playHit
            radius: play.radius
            tint: Md.onAccent
            enabled: surface.player !== null && surface.player.canTogglePlaying
            onActivated: surface.player.togglePlaying()
        }
    }

    IconButton {
        x: surface.fullNextX
        y: surface.fullControlY
        size: surface.controlSize
        icon: "skip_next"
        tip: I18n.tr("Next")
        enabled: surface.player !== null && surface.player.canGoNext
        opacity: surface.fullVis * (enabled ? 1 : 0.38)
        visible: surface.fullVis > 0.01
        onClicked: surface.player.next()
    }

    IconButton {
        x: surface.fullNextX + surface.controlSize + surface.controlGap
        y: surface.fullControlY
        size: surface.controlSize
        icon: surface.canLoop && surface.player.loopState === MprisLoopState.Track ? "repeat_one" : "repeat"
        tip: I18n.tr("Repeat")
        checked: surface.canLoop && surface.player.loopState !== MprisLoopState.None
        opacity: surface.fullVis
        visible: surface.canLoop && surface.fullVis > 0.01
        onClicked: surface.cycleLoop()
    }
}
