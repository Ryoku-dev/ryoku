pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Io
import shell.services
import shell.barkit as Pill

Item {
    id: root

    required property var colors
    required property real s

    readonly property var player: Media.player
    readonly property bool hasMedia: Media.present && player !== null
    readonly property color accent: hasMedia ? colors.accent(6) : colors.accent(4)

    // CAVA must analyse the playback device only. `auto` can fall back to an
    // input node on some PipeWire graphs, so target the active sink monitor by
    // its actual node name and never fall back to a microphone.
    readonly property string playbackMonitor: Audio.sink && Audio.sink.name
        ? String(Audio.sink.name) + ".monitor"
        : ""
    readonly property bool analyserWanted: root.hasMedia
        && Media.playing
        && root.playbackMonitor.length > 0
        && !Perf.pillFrozen

    property var spectrumLevels: flatSpectrum()

    readonly property string activeWindowTitle: {
        const top = Wm.focusedWindow
        const title = String(top && top.title ? top.title : "")
        return title.length > 0 ? title : "DESKTOP // IDLE"
    }

    readonly property string title: hasMedia
        ? String(player.trackTitle || player.identity || "UNTITLED SIGNAL")
        : activeWindowTitle

    readonly property string subtitle: {
        if (!hasMedia)
            return "ACTIVE WINDOW CHANNEL"
        const artist = Theme.joinArtists(player.trackArtists, player.trackArtist)
        return String(artist || player.trackAlbum || player.identity || "UNKNOWN ARTIST")
    }

    readonly property real progress: hasMedia && Number(player.length) > 0
        ? Math.max(0, Math.min(1, Number(player.position) / Number(player.length)))
        : 0

    function flatSpectrum() {
        const out = []
        for (let i = 0; i < 40; ++i)
            out.push(0)
        return out
    }

    function readSpectrum(line) {
        const text = String(line || "").trim()
        if (!text)
            return
        const parts = text.split(/[;\s]+/)
        if (parts.length < 40)
            return
        const out = []
        for (let i = 0; i < 40; ++i) {
            const value = parseInt(parts[i])
            out.push(isNaN(value) ? 0 : Math.max(0, Math.min(1, value / 100)))
        }
        root.spectrumLevels = out
    }

    Process {
        id: cavaProc
        property bool backoff: false

        // Pass the sink monitor as argv rather than interpolating it into the
        // shell program. An unresolved sink means no analyser, never `auto`.
        command: [
            "sh", "-c",
            "command -v cava >/dev/null 2>&1 || exit 0; src=\"$1\"; [ -n \"$src\" ] || exit 0; cfg=\"${XDG_RUNTIME_DIR:-/tmp}/ryoku-cava-chroma.conf\"; printf '%s\\n' '[general]' 'framerate = 30' 'bars = 40' '' '[input]' 'method = pipewire' \"source = $src\" 'active = 1' 'virtual = 1' '' '[output]' 'method = raw' 'raw_target = /dev/stdout' 'data_format = ascii' 'ascii_max_range = 100' 'channels = mono' 'mono_option = average' '' '[smoothing]' 'noise_reduction = 45' > \"$cfg\"; exec cava -p \"$cfg\"",
            "sh", root.playbackMonitor
        ]
        running: root.analyserWanted && !cavaProc.backoff

        stdout: SplitParser {
            splitMarker: "\n"
            onRead: line => root.readSpectrum(line)
        }

        onExited: if (root.analyserWanted) {
            cavaProc.backoff = true
            cavaRestart.restart()
        }
    }

    Timer {
        id: cavaRestart
        interval: 1200
        onTriggered: cavaProc.backoff = false
    }

    onPlaybackMonitorChanged: {
        root.spectrumLevels = root.flatSpectrum()
        if (root.analyserWanted) {
            cavaProc.backoff = true
            cavaRestart.restart()
        }
    }

    onAnalyserWantedChanged: if (!root.analyserWanted)
        root.spectrumLevels = root.flatSpectrum()

    Timer {
        interval: 1000
        running: root.hasMedia && Media.playing
        repeat: true
        onTriggered: {
            if (root.player && root.player.positionSupported)
                root.player.positionChanged()
        }
    }

    Rectangle {
        id: canvas
        anchors.fill: parent
        radius: Theme.radiusWidget * root.s
        color: hover.containsMouse ? root.colors.surface : root.colors.backgroundAlt
        border.width: 0
        clip: true

        Behavior on color {
            enabled: !Motion.reduce
            ColorAnimation { duration: Motion.fast; easing.type: Motion.easeStandard }
        }

        Rectangle {
            id: progressTrack
            visible: root.hasMedia
            anchors {
                left: parent.left
                right: parent.right
                bottom: parent.bottom
                leftMargin: Theme.paddingMd * root.s
                rightMargin: Theme.paddingMd * root.s
                bottomMargin: Theme.paddingSm * root.s
            }
            height: Math.max(1, Theme.borderWidth * root.s)
            radius: Math.max(0.5, Theme.borderWidth * root.s / 2)
            color: root.colors.surfaceHover

            Rectangle {
                anchors {
                    left: parent.left
                    top: parent.top
                    bottom: parent.bottom
                }
                width: parent.width * root.progress
                radius: Math.max(0.5, Theme.borderWidth * root.s / 2)
                color: root.accent

                Behavior on width {
                    enabled: !Motion.reduce
                    NumberAnimation { duration: Motion.fast; easing.type: Easing.OutCubic }
                }
            }
        }

        RowLayout {
            anchors {
                fill: parent
                leftMargin: Theme.paddingMd * root.s
                rightMargin: Theme.paddingMd * root.s
                bottomMargin: Theme.borderWidth * root.s
            }
            spacing: Theme.paddingMd * root.s

            Rectangle {
                Layout.preferredWidth: Theme.iconLg * root.s
                Layout.preferredHeight: Theme.iconLg * root.s
                Layout.alignment: Qt.AlignVCenter
                radius: Theme.radiusWidget * root.s
                color: root.colors.surface
                border.width: 0
                clip: true

                Image {
                    id: album
                    anchors.fill: parent
                    source: root.hasMedia ? String(root.player.trackArtUrl || "") : ""
                    fillMode: Image.PreserveAspectCrop
                    asynchronous: true
                    cache: false
                    visible: root.hasMedia && status === Image.Ready
                }

                Pill.MaterialIcon {
                    anchors.centerIn: parent
                    visible: !album.visible
                    text: root.hasMedia ? "music_note" : "desktop_windows"
                    color: root.accent
                    font.pixelSize: Theme.iconMd * root.s
                    fill: 1
                }
            }

            ColumnLayout {
                Layout.fillWidth: true
                Layout.alignment: Qt.AlignVCenter
                spacing: 0

                Text {
                    Layout.fillWidth: true
                    text: root.title
                    color: root.colors.text
                    font.family: Theme.mono
                    font.pixelSize: Theme.fontSm * root.s
                    font.weight: Font.Black
                    elide: Text.ElideRight
                }

                Text {
                    Layout.fillWidth: true
                    text: root.subtitle.toUpperCase()
                    color: root.colors.muted
                    font.family: Theme.mono
                    font.pixelSize: Math.max(Theme.paddingMd, Theme.fontSm - Theme.paddingSm - Theme.borderWidth) * root.s
                    font.weight: Font.Bold
                    font.letterSpacing: 1.2
                    elide: Text.ElideRight
                }
            }

            Item {
                visible: root.hasMedia
                Layout.preferredWidth: visible ? Theme.iconLg * 2 * root.s : 0
                Layout.preferredHeight: Theme.iconMd * root.s
                Layout.alignment: Qt.AlignVCenter

                Row {
                    anchors {
                        right: parent.right
                        bottom: parent.bottom
                    }
                    spacing: Theme.borderWidth * root.s

                    Repeater {
                        model: 12

                        Item {
                            required property int index
                            width: Math.max(2, 3 * root.s)
                            height: Theme.iconMd * root.s

                            Rectangle {
                                anchors {
                                    horizontalCenter: parent.horizontalCenter
                                    bottom: parent.bottom
                                }
                                width: Math.max(2, 3 * root.s)
                                height: {
                                    const levels = root.spectrumLevels || []
                                    if (levels.length === 0)
                                        return Theme.borderWidth
                                    const sourceIndex = Math.min(
                                        levels.length - 1,
                                        Math.floor(index * levels.length / 12)
                                    )
                                    return Theme.borderWidth * root.s
                                        + Math.max(0, Math.min(1, Number(levels[sourceIndex]) || 0))
                                            * (Theme.iconMd - Theme.borderWidth) * root.s
                                }
                                radius: width / 2
                                color: root.colors.accent(index)

                                Behavior on height {
                                    enabled: !Motion.reduce
                                    NumberAnimation { duration: 70 }
                                }
                            }
                        }
                    }
                }
            }
        }

        MouseArea {
            id: hover
            anchors.fill: parent
            hoverEnabled: true
            cursorShape: root.hasMedia ? Qt.PointingHandCursor : Qt.ArrowCursor
            onClicked: {
                if (root.hasMedia && root.player && root.player.canTogglePlaying)
                    Media.toggle()
            }
        }
    }
}
