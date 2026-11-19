import QtQuick
import Lithic

Item {
    id: root

    property string artwork: ""
    property string fallbackGlyph: "auto_awesome"
    property color tint: Lithic.gold

    implicitWidth: 112
    implicitHeight: 112

    Image {
        id: illustration
        anchors.fill: parent
        source: root.artwork ? Qt.resolvedUrl("../../assets/" +
            (root.artwork.indexOf(".") >= 0 ? root.artwork : "stickers/" + root.artwork + ".svg")) : ""
        fillMode: Image.PreserveAspectFit
        sourceSize.width: 512
        sourceSize.height: 512
        asynchronous: true
        mipmap: true
    }

    Glyph {
        anchors.centerIn: parent
        visible: illustration.status === Image.Error
        name: root.fallbackGlyph
        size: Math.round(Math.min(root.width, root.height) * 0.6)
        ink: root.tint
    }
}
