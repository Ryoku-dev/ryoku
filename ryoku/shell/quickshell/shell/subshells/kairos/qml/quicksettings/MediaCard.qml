pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import shell.services
import Ryoku.Ui.Singletons
import "../kit"

Item {
    id: card

    readonly property var player: Media.player
    readonly property bool playing: Media.playing
    readonly property string art: Music.coverFor(card.player)
    readonly property real length: Music.duration
    readonly property string source: card.player
        ? String(card.player.identity || card.player.dbusName || "").replace(/^org\.mpris\.MediaPlayer2\./, "") : ""

    function stamp(seconds) {
        var s = Math.max(0, Math.floor(seconds || 0));
        var m = Math.floor(s / 60);
        var r = s % 60;
        return m + ":" + (r < 10 ? "0" : "") + r;
    }

    implicitHeight: 152

    onVisibleChanged: Music.holdClock(card, card.visible)
    Component.onCompleted: Music.holdClock(card, card.visible)
    Component.onDestruction: Music.holdClock(card, false)

    Item {
        id: backdrop
        anchors.fill: parent
        layer.enabled: true
        layer.effect: MultiEffect {
            maskEnabled: true
            maskSource: mask
            autoPaddingEnabled: false
        }

        Rectangle {
            anchors.fill: parent
            color: Md.accentContainer
        }
        Image {
            anchors.fill: parent
            source: card.art
            visible: card.art.length > 0 && status === Image.Ready
            // a fixed size, the card's width follows the shade's open animation and would re-decode every frame
            sourceSize: Qt.size(Math.ceil(440 * Screen.devicePixelRatio), Math.ceil(152 * Screen.devicePixelRatio))
            fillMode: Image.PreserveAspectCrop
            asynchronous: true
        }
        Rectangle {
            anchors.fill: parent
            gradient: Gradient {
                orientation: Gradient.Horizontal
                GradientStop { position: 0.0; color: Qt.rgba(0, 0, 0, 0.78) }
                GradientStop { position: 1.0; color: Qt.rgba(0, 0, 0, 0.42) }
            }
        }
    }
    Rectangle {
        id: mask
        anchors.fill: parent
        radius: Md.rXxl
        visible: false
        layer.enabled: true
    }

    Label {
        id: sourceLabel
        anchors { left: parent.left; leftMargin: 20; top: parent.top; topMargin: 16; right: play.left; rightMargin: 16 }
        role: "caption"
        color: Qt.rgba(1, 1, 1, 0.78)
        text: card.source
    }
    Column {
        anchors { left: parent.left; leftMargin: 20; top: sourceLabel.bottom; topMargin: 4; right: play.left; rightMargin: 16 }
        spacing: 2

        Label {
            width: parent.width
            role: "title"
            font.pixelSize: 17
            color: "white"
            text: card.player ? (card.player.trackTitle || "") : ""
        }
        Label {
            width: parent.width
            role: "body"
            color: Qt.rgba(1, 1, 1, 0.78)
            text: card.player ? Theme.joinArtists(card.player.trackArtists, card.player.trackArtist) : ""
        }
    }

    Rectangle {
        id: play
        anchors { right: parent.right; rightMargin: 16; top: parent.top; topMargin: 16 }
        width: 56
        height: 56
        radius: playHit.pressed ? Md.rMd : (card.playing ? Md.rLg : 28)
        color: Md.accent
        Behavior on radius {
            enabled: !Md.still
            Spring { speed: "fast" }
        }

        Icon {
            anchors.centerIn: parent
            name: card.playing ? "pause" : "play_arrow"
            size: 28
            filled: true
            color: Md.onAccent
        }
        StateLayer {
            id: playHit
            radius: play.radius
            tint: Md.onAccent
            enabled: !!card.player && card.player.canTogglePlaying
            onActivated: card.player.togglePlaying()
        }
    }

    Row {
        anchors { left: parent.left; leftMargin: 8; right: parent.right; rightMargin: 8; bottom: parent.bottom; bottomMargin: 10 }
        spacing: 4

        IconButton {
            id: prev
            anchors.verticalCenter: parent.verticalCenter
            icon: "skip_previous"
            tip: I18n.tr("Previous")
            enabled: !!card.player && card.player.canGoPrevious
            onClicked: card.player.previous()
        }
        Label {
            id: elapsed
            anchors.verticalCenter: parent.verticalCenter
            width: 38
            horizontalAlignment: Text.AlignRight
            role: "caption"
            font.family: Md.mono
            font.pixelSize: 11
            color: Qt.rgba(1, 1, 1, 0.78)
            text: card.stamp(seek.scrubbing ? seek.scrub * card.length : Music.position)
        }
        Wavy {
            id: seek
            anchors.verticalCenter: parent.verticalCenter
            width: parent.width - prev.width - next.width - elapsed.width - total.width - 4 * parent.spacing - 8
            progress: card.length > 0 ? Music.position / card.length : 0
            playing: card.playing
            seekable: !!card.player && card.player.canSeek && card.length > 0
            trackColor: Qt.rgba(1, 1, 1, 0.28)
            onSeek: function (fraction) { Music.seekPosition(fraction * card.length); }
        }
        Label {
            id: total
            anchors.verticalCenter: parent.verticalCenter
            width: 38
            role: "caption"
            font.family: Md.mono
            font.pixelSize: 11
            color: Qt.rgba(1, 1, 1, 0.78)
            text: card.length > 0 ? card.stamp(card.length) : ""
        }
        IconButton {
            id: next
            anchors.verticalCenter: parent.verticalCenter
            icon: "skip_next"
            tip: I18n.tr("Next")
            enabled: !!card.player && card.player.canGoNext
            onClicked: card.player.next()
        }
    }
}
