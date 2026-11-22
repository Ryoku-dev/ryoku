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

}
