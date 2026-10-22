pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Quickshell.Services.Mpris
import Quickshell.Services.Pipewire
import Caelestia.Config
import Caelestia.Services
import caelestia.components
import caelestia.services

// Compact now-playing entry: truncated title plus the cava visualiser.
StyledRect {
    id: root

    required property var popouts

    readonly property bool isHorizontal: Config.bar.position === "top" || Config.bar.position === "bottom"
    readonly property MprisPlayer player: Players.active

    readonly property int maxLen: Config.bar.media.maxTitleLength
    readonly property string rawTitle: player?.trackTitle || qsTr("Nothing playing")
    readonly property string trackTitle: rawTitle.length > maxLen ? rawTitle.substring(0, maxLen) + "…" : rawTitle
    readonly property bool isPlaying: player?.isPlaying ?? false
    readonly property bool available: player !== null
    readonly property bool showTitle: Config.bar.media.showTitle
    readonly property real contentWidth: root.showTitle ? contentLayout.implicitWidth : modeIcon.implicitWidth
    readonly property real contentHeight: root.showTitle ? contentLayout.implicitHeight : modeIcon.implicitHeight

    // Stream-aware volume: MPRIS app volume first, then the matching
    // PipeWire stream, then the global sink. Driven by mouse scroll.
    readonly property PwNode playerStream: Audio.streams.find(s => {
        const identity = root.player?.identity?.toLowerCase() ?? "";
        const entry = (root.player?.entry ?? "").toString().toLowerCase();
        if (!identity && !entry)
            return false;
        const streamName = Audio.getStreamName(s).toLowerCase();
        const binary = (s.properties["application.process.binary"] ?? "").toString().toLowerCase();
        const appName = (s.properties["app.name"] ?? "").toString().toLowerCase();
        const playerNames = [identity, entry].filter(n => n);
        const streamNames = [streamName, binary, appName].filter(n => n);
        return streamNames.some(sn => playerNames.some(pn => sn.includes(pn) || pn.includes(sn)));
    }) || null
    readonly property real currentVolume: (Players.supportsAppVolume(root.player) && root.player?.volume !== undefined && root.player?.volume !== null) ? root.player.volume : (root.playerStream ? Audio.getStreamVolume(root.playerStream) : Audio.volume)

    function setVolumeLevel(v: real): void {
        const clamped = Math.max(0, Math.min(1, v));
        if (Players.supportsAppVolume(root.player) && root.player && root.player.volume !== undefined) {
            root.player.volume = clamped;
        } else if (root.playerStream) {
            Audio.setStreamVolume(root.playerStream, clamped);
        } else {
            Audio.setVolume(clamped);
        }
    }

    implicitWidth: isHorizontal ? contentWidth + Tokens.padding.medium * 2 : Tokens.sizes.bar.innerWidth
    implicitHeight: isHorizontal ? Tokens.sizes.bar.innerWidth : contentHeight + Tokens.padding.medium * 2

    color: Config.bar.media.background ? Colours.tPalette.m3surfaceContainer : Qt.alpha(Colours.tPalette.m3surfaceContainer, 0)
    radius: Tokens.rounding.full

    // A bar entry with auto-hide collapses instead of leaving an empty pill.
    visible: enabled && (!Config.bar.media.autoHide || available)

    ServiceRef {
        service: Config.bar.media.showVisualiser ? Audio.cava : null
    }

    GridLayout {
        id: contentLayout

        anchors.centerIn: parent
        columns: root.isHorizontal ? 2 : 1
        rows: root.isHorizontal ? 1 : 2
        rowSpacing: Tokens.spacing.small
        columnSpacing: Tokens.spacing.small

        Item {
            id: textContainer

            visible: root.showTitle

            Layout.alignment: Qt.AlignCenter
            Layout.row: root.isHorizontal ? 0 : (Config.bar.media.inverted ? 1 : 0)
            Layout.column: 0

            implicitWidth: root.isHorizontal ? titleText.implicitWidth : titleText.implicitHeight
            implicitHeight: root.isHorizontal ? titleText.implicitHeight : titleText.implicitWidth

            StyledText {
                id: titleText

                anchors.centerIn: parent
                text: root.trackTitle
                font: Tokens.font.body.medium
                color: Colours.palette.m3onSurface
                elide: Text.ElideRight
                animate: true

                rotation: root.isHorizontal ? 0 : (Config.bar.media.inverted ? 270 : 90)
            }
        }

        Item {
            id: equalizerContainer

            Layout.alignment: Qt.AlignCenter
            Layout.row: root.isHorizontal ? 0 : (Config.bar.media.inverted ? 0 : 1)
            Layout.column: root.isHorizontal ? 1 : 0

            visible: Config.bar.media.showVisualiser && root.showTitle
            implicitWidth: root.isHorizontal ? (visible ? 23 : 0) : 20
            implicitHeight: root.isHorizontal ? 20 : (visible ? 23 : 0)

            Item {
                anchors.centerIn: parent
                width: 23
                height: 20

                rotation: root.isHorizontal ? 0 : (Config.bar.media.inverted ? 270 : 90)

                Repeater {
                    model: 5

                    Rectangle {
                        id: barItem

                        required property int index

                        readonly property real cavaVal: (Audio.cava && Audio.cava.values && Audio.cava.values.length > index * 2) ? Audio.cava.values[index * 2] : 0
                        readonly property real rawHeight: root.isPlaying ? Math.max(3, Math.min(18, 3 + cavaVal * 15)) : 3

                        x: index * 5
                        width: 3
                        height: Math.round(rawHeight)
                        y: Math.round((20 - height) / 2)
                        radius: 1.5
                        color: Colours.palette.m3primary

                        Behavior on height {
                            NumberAnimation {
                                duration: 50
                                easing.type: Easing.OutQuad
                            }
                        }
                    }
                }
            }
        }
    }

    MaterialIcon {
        id: modeIcon

        anchors.centerIn: parent
        visible: !root.showTitle
        text: "graphic_eq"
        color: Colours.palette.m3onSurface
        fontStyle: Tokens.font.icon.builders.medium.build()
    }

    WheelHandler {
        acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
        onWheel: event => {
            if (event.angleDelta.y > 0)
                root.setVolumeLevel(root.currentVolume + 0.05);
            else if (event.angleDelta.y < 0)
                root.setVolumeLevel(root.currentVolume - 0.05);
        }
    }
}
