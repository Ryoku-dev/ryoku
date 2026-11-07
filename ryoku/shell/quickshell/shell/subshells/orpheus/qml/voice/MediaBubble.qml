import QtQuick
import shell.services
import "../kit" as K
import "../settings" as S

Item {
    id: root

    required property var scene

    readonly property var player: Media.player

    implicitWidth: K.Style.barH
    implicitHeight: K.Style.barH

    K.CoverArt {
        anchors.centerIn: parent
        source: root.player ? (root.player.trackArtUrl || "") : ""
        size: 22 * K.Style.scale
        round: Media.playing && root.visible && S.Prefs.coverSpin && !K.Style.still
        spinning: round
    }

    K.Press {
        id: press
        radius: K.Style.radius
        onClicked: root.scene.toggleCard("media", root)
        onMiddleClicked: Media.toggle()
    }

    K.Hint {
        target: root
        text: root.player ? String(root.player.trackTitle || "Now playing") : "Now playing"
        shown: press.hovered
    }
}
