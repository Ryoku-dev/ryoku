pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Services.Mpris
import shell.services
import "../kit" as K
import "../settings" as S

Item {
    id: root

    required property var scene
    property bool shown: false
    property var args: ({})

    readonly property var players: Media.playerSet
        ? Media.playerSet.filter(candidate => candidate && !Media.isWallpaper(candidate)) : []
    readonly property var player: Media.player
    readonly property bool playing: player !== null && player.isPlaying === true
    readonly property bool hasTrack: player !== null
        && String(player.trackTitle || "").length > 0
    readonly property string titleText: player ? String(player.trackTitle || "") : ""
    readonly property string artistText: player
        ? Theme.joinArtists(player.trackArtists, player.trackArtist) : ""
    readonly property string albumText: player ? String(player.trackAlbum || "") : ""
    readonly property real trackLength: player
        ? Math.max(0, Number(Music.duration) || 0) : 0
    readonly property real trackPosition: player
        ? Math.max(0, Number(Music.position) || 0) : 0

    readonly property real progress: trackLength > 0
        ? Math.max(0, Math.min(1, trackPosition / trackLength)) : 0
    readonly property bool canSeek: player !== null && player.canSeek === true
        && trackLength > 0
    readonly property bool volumeAvailable: player !== null
        && player.volumeSupported === true && player.canControl === true
    readonly property bool canRaise: player !== null && player.canRaise === true

    implicitWidth: sheet.implicitWidth
    implicitHeight: sheet.implicitHeight

    function playerName(candidate) {
        if (!candidate)
            return "Player";
        const identity = String(candidate.identity || "");
        return identity.length > 0 ? identity : String(candidate.dbusName || "Player");
    }

    function formatTime(seconds) {
        const whole = Math.max(0, Math.floor(Number(seconds) || 0));
        const hours = Math.floor(whole / 3600);
        const minutes = Math.floor((whole % 3600) / 60);
        const remainder = whole % 60;
        const tail = (remainder < 10 ? "0" : "") + remainder;
        if (hours > 0)
            return hours + ":" + (minutes < 10 ? "0" : "") + minutes + ":" + tail;
        return minutes + ":" + tail;
    }

    function cycleLoop() {
        if (!player || player.loopSupported !== true)
            return;
        if (player.loopState === MprisLoopState.None)
            player.loopState = MprisLoopState.Playlist;
        else if (player.loopState === MprisLoopState.Playlist)
            player.loopState = MprisLoopState.Track;
        else
            player.loopState = MprisLoopState.None;
    }

    function updateSpectrumClaim() {
        AudioBars.setActive(root, root.shown && root.hasTrack
            && S.Prefs.spectrum === true);
    }
    function updateClockClaim() {
        Music.holdClock(root, root.shown && root.hasTrack);
    }


    onShownChanged: {
        updateSpectrumClaim();
        updateClockClaim();
    }
    onHasTrackChanged: {
        updateSpectrumClaim();
        updateClockClaim();
    }

    Connections {
        target: S.Prefs
        function onSpectrumChanged() { root.updateSpectrumClaim(); }
    }
    Component.onCompleted: {
        updateSpectrumClaim();
        updateClockClaim();
    }
    Component.onDestruction: {
        AudioBars.setActive(root, false);
        Music.holdClock(root, false);
    }


    K.Sheet {
        id: sheet
        pageWidth: 380

        Item {
            visible: root.hasTrack
            width: parent.width
            height: 112

            K.CoverArt {
                id: cover
                source: root.player ? (root.player.trackArtUrl || "") : ""
                size: 112
                round: S.Prefs.coverSpin === true
                spinning: root.shown && root.playing && S.Prefs.coverSpin === true
            }

            Column {
                x: cover.width + K.Style.s4
                width: Math.max(0, parent.width - x - (root.canRaise ? openPlayer.width + K.Style.s1 : 0))
                anchors.verticalCenter: parent.verticalCenter
                spacing: K.Style.s1

                K.Label {
                    width: parent.width
                    text: root.titleText
                    role: "title"
                    wrapMode: Text.Wrap
                    maximumLineCount: 2
                }
                K.Label {
                    visible: text.length > 0
                    width: parent.width
                    text: root.artistText
                    role: "body"
                    tone: "dim"
                }
                K.Label {
                    visible: text.length > 0
                    width: parent.width
                    text: root.albumText
                    role: "caption"
                    tone: "faint"
                }
            }

            K.IconButton {
                id: openPlayer
                anchors.right: parent.right
                anchors.top: parent.top
                visible: root.canRaise
                icon: "open_in_new"
                size: 36
                onClicked: if (root.canRaise) root.player.raise()
            }
        }

        Column {
            visible: root.hasTrack
            width: parent.width
            spacing: K.Style.s1

            K.Squiggle {
                width: parent.width
                progress: root.progress
                playing: root.shown && root.playing
                amplitude: S.Prefs.wavy === true ? 2 : 0
                interactive: root.canSeek
                onSeek: fraction => {
                    if (root.player && root.canSeek)
                        Music.seekPosition(fraction * root.trackLength);
                }

            }

            Item {
                width: parent.width
                height: Math.max(elapsedLabel.implicitHeight, remainingLabel.implicitHeight)

                K.Label {
                    id: elapsedLabel
                    anchors.left: parent.left
                    text: root.formatTime(root.trackPosition)
                    role: "caption"
                    tone: "variant"
                    numeric: true
                }
                K.Label {
                    id: remainingLabel
                    anchors.right: parent.right
                    text: "−" + root.formatTime(Math.max(0,
                        root.trackLength - root.trackPosition))
                    role: "caption"
                    tone: "variant"
                    numeric: true
                }
            }
        }

        Item {
            visible: root.hasTrack
            width: parent.width
            height: 64

            K.IconButton {
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                visible: root.player !== null
                    && root.player.shuffleSupported === true
                icon: "shuffle"
                size: 36
                toggled: root.player !== null && root.player.shuffle === true
                variant: "plain"
                onClicked: if (root.player) root.player.shuffle = !root.player.shuffle
            }

            Row {
                anchors.centerIn: parent
                spacing: K.Style.s3

                K.IconButton {
                    enabled: root.player !== null && root.player.canGoPrevious === true
                    icon: "skip_previous"
                    size: 44
                    variant: "plain"
                    onClicked: if (enabled) root.player.previous()
                }
                K.PlayButton {
                    enabled: root.player !== null
                        && root.player.canTogglePlaying === true
                    playing: root.playing
                    onClicked: if (enabled) Media.toggle()
                }
                K.IconButton {
                    enabled: root.player !== null && root.player.canGoNext === true
                    icon: "skip_next"
                    size: 44
                    variant: "plain"
                    onClicked: if (enabled) root.player.next()
                }
            }

            K.IconButton {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                visible: root.player !== null && root.player.loopSupported === true
                icon: root.player && root.player.loopState === MprisLoopState.Track
                    ? "repeat_one" : "repeat"
                size: 36
                toggled: root.player !== null
                    && root.player.loopState !== MprisLoopState.None
                variant: "plain"
                onClicked: root.cycleLoop()
            }
        }

        K.Slider {
            visible: root.hasTrack && root.volumeAvailable
            width: parent.width
            from: 0
            to: 1
            value: root.player
                ? Math.max(0, Math.min(1, Number(root.player.volume) || 0)) : 0
            stepSize: 0.05
            icon: "volume_up"
            valueText: Math.round(value * 100) + "%"
            onCommitted: value => {
                if (root.player && root.volumeAvailable)
                    root.player.volume = value;
            }
        }

        Flow {
            visible: root.players.length > 1
            width: parent.width
            spacing: K.Style.s2

            Repeater {
                model: root.players

                delegate: K.Chip {
                    required property var modelData
                    text: root.playerName(modelData)
                    icon: "music_note"
                    selected: modelData === root.player
                    onClicked: Media.player = modelData
                }
            }
        }

        Item {
            visible: root.hasTrack && S.Prefs.spectrum === true
            width: parent.width
            height: 24

            Repeater {
                model: 40

                delegate: Item {
                    id: spectrumBand
                    required property int index
                    readonly property real level: root.shown
                        && AudioBars.levels && index < AudioBars.levels.length
                        ? Math.max(0, Math.min(1,
                            Number(AudioBars.levels[index]) || 0)) : 0

                    x: index * (parent.width - width) / 39
                    width: 3
                    height: parent.height

                    Rectangle {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.bottom: parent.bottom
                        height: spectrumBand.level > 0
                            ? Math.max(1, spectrumBand.level * parent.height) : 1
                        radius: 1
                        color: K.Style.alpha(K.Style.ink, 0.32)

                    }
                }
            }
        }

        K.Label {
            visible: !root.hasTrack
            width: parent.width
            text: "Nothing playing"
            role: "body"
            tone: "variant"
            horizontalAlignment: Text.AlignHCenter
        }

        Flow {
            visible: !root.hasTrack && root.players.length > 0
            width: parent.width
            spacing: K.Style.s2

            Repeater {
                model: root.players

                delegate: K.Chip {
                    required property var modelData
                    text: root.playerName(modelData)
                    icon: "music_note"
                    selected: modelData === root.player
                    onClicked: Media.player = modelData
                }
            }
        }
    }
}
