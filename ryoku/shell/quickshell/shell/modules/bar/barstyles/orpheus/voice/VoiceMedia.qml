pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import "../kit" as K
import "../settings" as S

Item {
    id: root

    required property var scene

    readonly property var player: Media.player
    readonly property string title: player ? String(player.trackTitle || "") : ""
    readonly property string artist: player
        ? Theme.joinArtists(player.trackArtists, player.trackArtist) : ""
    readonly property url art: Music.coverFor(player)
    // Same contract as VoiceNotice: this face feeds the voice slot width, which
    // laneNeeded reads live, so it must not depend on scene.textRoom.
    readonly property real titleWidth: Math.max(1, Math.min(
        S.Prefs.mediaWidth * K.Style.scale,
        Math.max(titleMeasure.implicitWidth, artistMeasure.implicitWidth)))

    implicitWidth: content.implicitWidth
    implicitHeight: K.Style.barH

    K.Label {
        id: titleMeasure
        visible: false
        text: root.title
        role: "bar"
    }

    K.Label {
        id: artistMeasure
        visible: false
        text: root.artist
        role: "barSub"
    }

    Row {
        id: content
        anchors.centerIn: parent
        spacing: K.Style.s2

        K.CoverArt {
            anchors.verticalCenter: parent.verticalCenter
            source: root.art
            size: 22 * K.Style.scale
            round: Media.playing && root.visible && S.Prefs.coverSpin && !K.Style.still
            spinning: round
        }

        Column {
            anchors.verticalCenter: parent.verticalCenter
            width: root.titleWidth
            spacing: 0

            K.Label {
                width: parent.width
                text: root.title
                role: "bar"
            }

            K.Label {
                width: parent.width
                text: root.artist
                role: "barSub"
                tone: "dim"
            }
        }

        Row {
            anchors.verticalCenter: parent.verticalCenter
            spacing: 2 * K.Style.scale

            Repeater {
                model: 6

                delegate: Rectangle {
                    id: spectrumBar
                    required property int index

                    readonly property var samples: AudioBars.levels || []
                    readonly property real level: Number(samples.length > 0
                        ? samples[Math.min(samples.length - 1,
                            Math.floor(index * samples.length / 6))] : 0) || 0

                    anchors.verticalCenter: parent.verticalCenter
                    width: 3 * K.Style.scale
                    height: Math.max(2 * K.Style.scale,
                        Math.round(14 * K.Style.scale * level))
                    radius: width / 2
                    color: K.Style.ink

                }
            }
        }
    }
}
