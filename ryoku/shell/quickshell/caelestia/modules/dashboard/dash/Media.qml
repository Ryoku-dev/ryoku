pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Quickshell.Services.Mpris
import Caelestia.Components
import Caelestia.Config
import Caelestia.Services
import caelestia.components
import caelestia.components.controls
import caelestia.components.widgets
import caelestia.services
import caelestia.utils

Item {
    id: root

    property real playerProgress: {
        const active = Players.active;
        return active?.length ? (active.position % active.length) / active.length : 0;
    }

    readonly property real arcCoverGap: Tokens.spacing.extraSmall

    // Lets embedders (e.g. the bar popout) show extra controls in the
    // built-in row. Defaults to false, so the dashboard renders exactly
    // as before.
    property bool extraButtons: false

    // Lets embedders hide the gif block to compact the player.
    // Defaults to true, so the dashboard renders exactly as before.
    property bool showGif: true

    anchors.top: parent.top
    anchors.bottom: parent.bottom
    implicitWidth: Tokens.sizes.dashboard.mediaWidth

    Behavior on playerProgress {
        Anim {
            type: Anim.StandardLarge
        }
    }

    Timer {
        running: Players.active?.isPlaying ?? false
        interval: GlobalConfig.dashboard.mediaUpdateInterval
        triggeredOnStart: true
        repeat: true
        onTriggered: Players.active?.positionChanged()
    }

    Loader {
        active: root.visible && (Players.active?.isPlaying ?? false)
        sourceComponent: ServiceRef {
            service: Audio.beatTracker
        }
    }

    CircularProgress {
        id: prog

        anchors.centerIn: cover
        implicitSize: cover.width + root.arcCoverGap + thickness * 2

        fgColour: Colours.palette.m3primary
        strokeWidth: Tokens.sizes.dashboard.mediaProgressThickness
        startAngle: -90 - sweepAngle / 2
        sweepAngle: Tokens.sizes.dashboard.mediaProgressSweep
        value: root.playerProgress

        wavy: true
        waveFrequency: 8
        waveDuration: 2000
        wavePaused: !Players.active?.isPlaying
    }

    CoverArt {
        id: cover

        visible: HostState.element("mediaArtwork")
        anchors.top: parent.top
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.margins: Tokens.padding.medium + root.arcCoverGap + prog.thickness
        implicitHeight: width
    }

    Column {
        id: trackInfo

        visible: HostState.element("mediaTrack")
        anchors.top: cover.bottom
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.topMargin: Tokens.spacing.medium
        spacing: Tokens.spacing.small

        StyledText {
            anchors.horizontalCenter: parent.horizontalCenter

            animate: true
            horizontalAlignment: Text.AlignHCenter
            text: (Players.active?.trackTitle ?? qsTr("No media")) || qsTr("Unknown title")
            color: Colours.palette.m3primary
            font: Tokens.font.title.small

            width: root.implicitWidth - Tokens.padding.extraLargeIncreased
            elide: Text.ElideRight
        }

        StyledText {
            visible: !!Players.active && text !== ""
            anchors.horizontalCenter: parent.horizontalCenter

            animate: true
            horizontalAlignment: Text.AlignHCenter
            text: Players.active?.trackAlbum || qsTr("Unknown album")
            color: Colours.palette.m3outline
            font: Tokens.font.body.small

            width: root.implicitWidth - Tokens.padding.extraLargeIncreased
            elide: Text.ElideRight
        }

        StyledText {
            visible: !!Players.active && text !== ""
            anchors.horizontalCenter: parent.horizontalCenter

            animate: true
            horizontalAlignment: Text.AlignHCenter
            text: Players.active?.trackArtist || qsTr("Unknown artist")
            color: Colours.palette.m3secondary

            width: root.implicitWidth - Tokens.padding.extraLargeIncreased
            elide: Text.ElideRight
        }
    }

    ButtonRow {
        id: controls

        visible: HostState.element("mediaTransport")
        anchors.top: trackInfo.bottom
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.topMargin: Tokens.spacing.medium
        anchors.margins: Tokens.padding.large
        anchors.leftMargin: root.extraButtons ? Tokens.padding.small : Tokens.padding.large
        anchors.rightMargin: root.extraButtons ? Tokens.padding.small : Tokens.padding.large

        spacing: Tokens.spacing.extraSmall

        IconButton {
            visible: root.extraButtons
            type: IconButton.Tonal
            icon: "shuffle"
            isRound: true
            shapeMorph: true
            checked: Players.active?.shuffle ?? false
            font: Tokens.font.icon.small
            disabled: !Players.active?.shuffleSupported
            onClicked: Players.active.shuffle = !Players.active?.shuffle
            implicitWidth: Math.round(implicitHeight * 0.9)
        }

        IconButton {
            type: IconButton.Tonal
            icon: "skip_previous"
            isRound: true
            shapeMorph: true
            disabled: !Players.active?.canGoPrevious
            onClicked: Players.active?.previous()
        }

        IconButton {
            fillWidth: true
            icon: Players.active?.isPlaying ? "pause" : "play_arrow"
            isRound: true
            shapeMorph: true
            checked: Players.active?.isPlaying ?? false
            disabled: !Players.active?.canTogglePlaying
            onClicked: Players.active?.togglePlaying()
        }

        IconButton {
            type: IconButton.Tonal
            icon: "skip_next"
            isRound: true
            shapeMorph: true
            disabled: !Players.active?.canGoNext
            onClicked: Players.active?.next()
        }

        IconButton {
            visible: root.extraButtons
            type: IconButton.Tonal
            icon: Players.active?.loopState === MprisLoopState.Track ? "repeat_one" : "repeat"
            isRound: true
            shapeMorph: true
            checked: Players.active?.loopState === MprisLoopState.Track || Players.active?.loopState === MprisLoopState.Playlist
            font: Tokens.font.icon.small
            disabled: !Players.active?.loopSupported
            onClicked: {
                const state = Players.active.loopState;
                if (state === MprisLoopState.None)
                    Players.active.loopState = MprisLoopState.Track;
                else if (state === MprisLoopState.Track)
                    Players.active.loopState = MprisLoopState.Playlist;
                else
                    Players.active.loopState = MprisLoopState.None;
            }
            implicitWidth: Math.round(implicitHeight * 0.9)
        }
    }

    Item {
        id: bongocat

        visible: root.showGif
        anchors.top: controls.bottom
        anchors.bottom: parent.bottom
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.topMargin: Tokens.spacing.small
        anchors.bottomMargin: Tokens.padding.large
        anchors.margins: Tokens.padding.extraLargeIncreased

        AnimatedImage {
            id: gif

            anchors.fill: parent

            playing: Players.active?.isPlaying ?? false
            speed: Audio.beatTracker.bpm / Config.general.mediaGifSpeedAdjustment // qmllint disable unresolved-type
            source: Paths.absolutePath(Config.paths.mediaGif)
            asynchronous: true
            fillMode: AnimatedImage.PreserveAspectFit
            visible: !Config.dashboard.useMediaShapes
        }

        MultiEffect {
            anchors.fill: gif
            source: gif

            visible: Config.dashboard.colorizeMediaGif && !Config.dashboard.useMediaShapes
            colorization: 1
            colorizationColor: Colours.palette.m3primary
        }

        MediaShapes {
            anchors.fill: parent
            visible: Config.dashboard.useMediaShapes
            active: Config.dashboard.useMediaShapes
        }
    }
}
