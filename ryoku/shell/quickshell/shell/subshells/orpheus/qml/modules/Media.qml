pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import Ryoku.Ui.Singletons
import "../kit" as K
import "../settings" as S

Item {
    id: root

    required property var scene
    required property string lane
    property bool present: Media.present

    readonly property var player: Media.player
    readonly property string title: player ? String(player.trackTitle || "") : ""
    readonly property string artist: player
        ? Theme.joinArtists(player.trackArtists, player.trackArtist) : ""
    readonly property url art: player ? (player.trackArtUrl || "") : ""
    readonly property bool hasText: title.length > 0 || artist.length > 0
    readonly property real coverSize: Math.round(22 * K.Style.scale)
    readonly property real contentGap: hasText ? Math.round(6 * K.Style.scale) : 0
    readonly property real naturalTextWidth: Math.max(
        titleMeasure.implicitWidth, artistMeasure.implicitWidth)
    readonly property real layoutPreferredTextWidth: hasText
        ? Math.min(Math.max(0, Number(S.Prefs.mediaWidth) || 220), naturalTextWidth)
        : 0
    readonly property real layoutFixedWidth: 2 * K.Style.hitPad
        + coverSize + contentGap
    readonly property real textWidth: Math.max(0,
        Math.min(layoutPreferredTextWidth, scene.textRoom))
    readonly property string hintText: artist.length > 0
        ? title + ", " + artist : title

    implicitWidth: present ? layoutFixedWidth + textWidth : 0
    implicitHeight: K.Style.barH
    height: K.Style.barH

    Component.onCompleted: scene.registerTextModule("media", root)
    Component.onDestruction: scene.unregisterTextModule("media", root)

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
        spacing: root.contentGap
        scale: press.pressScale

        K.CoverArt {
            anchors.verticalCenter: parent.verticalCenter
            source: root.art
            size: root.coverSize
            round: Media.playing && root.visible && root.present
                && S.Prefs.coverSpin && !K.Style.still
            spinning: round
        }

        Column {
            visible: root.hasText
            anchors.verticalCenter: parent.verticalCenter
            width: root.textWidth
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
    }

    K.Press {
        id: press
        radius: K.Style.radius
        onClicked: root.scene.toggleCard("media", root)
        onRightClicked: Media.toggle()
        onMiddleClicked: Media.toggle()
        onHoveredChanged: root.scene.hoverCard("media", root, hovered)
    }

    K.Hint {
        target: root
        text: root.hintText
        shown: root.present && press.hovered && root.hintText.length > 0
    }
}
