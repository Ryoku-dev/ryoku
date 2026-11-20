import QtQuick
import Lithic
import ".."

Item {
    id: seal

    property string badgeId: ""
    property string title: "RYOKU"
    property string sigil: "力"
    property string rarity: "common"
    property string category: "Ryoku"
    property bool locked: false
    property bool animateFoil: false

    readonly property bool moving: animateFoil && visible &&
        rarity === "holographic" && !locked && !Lithic.reduceMotion

    implicitWidth: 150
    implicitHeight: 150
    clip: false

    Image {
        id: emblem
        anchors.fill: parent
        source: seal.visible && seal.badgeId !== "" ?
            Qt.resolvedUrl("../../assets/badges/" + seal.badgeId + ".svg") : ""
        sourceSize.width: Math.min(1024, Math.max(1, Math.round(width * 2)))
        sourceSize.height: Math.min(1024, Math.max(1, Math.round(height * 2)))
        fillMode: Image.PreserveAspectFit
        smooth: true
        asynchronous: true
        opacity: seal.locked ? 0.29 : 1
    }

    Rectangle {
        visible: emblem.status !== Image.Ready
        anchors.centerIn: parent
        width: parent.width * 0.7
        height: width
        rotation: 45
        color: "transparent"
        border.color: Lithic.fg3
        border.width: 1
        Label {
            anchors.centerIn: parent
            text: "力"
            typeRole: "section"
        }
    }

    Rectangle {
        id: glint
        visible: seal.moving
        width: Math.max(4, seal.width * 0.13)
        height: seal.height * 1.45
        y: -seal.height * 0.22
        opacity: 0.28
        rotation: 25
        x: -width * 2
        gradient: Gradient {
            GradientStop { position: 0; color: "transparent" }
            GradientStop { position: 0.45; color: Lithic.mix(Lithic.lapis, Lithic.gold, 0.20) }
            GradientStop { position: 0.6; color: Lithic.mix(Lithic.gold, Lithic.fg, 0.38) }
            GradientStop { position: 1; color: "transparent" }
        }

        NumberAnimation on x {
            from: -glint.width * 2
            to: seal.width + glint.width * 2
            duration: 4900
            loops: Animation.Infinite
            running: seal.moving
        }
    }
}
