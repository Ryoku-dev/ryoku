// The person: their face in a circle, or their initial when there is no face.
import QtQuick 2.15
import QtQuick.Effects

Item {
    id: root
    property string icon: ""
    property string name: ""
    property string fontFamily: "Inter"
    property color fill: Qt.rgba(1, 1, 1, 0.2)
    property color ink: "#ffffff"

    property int sourceIndex: 0
    readonly property var sources: [root.icon, String(Qt.resolvedUrl("assets/user-face.png"))].filter(s => s.length > 0)
    onIconChanged: root.sourceIndex = 0

    Item {
        id: face
        anchors.fill: parent
        layer.enabled: true
        layer.effect: MultiEffect {
            maskEnabled: true
            maskSource: mask
            maskThresholdMin: 0.5
            maskSpreadAtMin: 1
        }
        Rectangle { anchors.fill: parent; color: root.fill }
        Image {
            id: image
            anchors.fill: parent
            source: root.sources[root.sourceIndex] || ""
            sourceSize: Qt.size(root.width * 2, root.height * 2)
            fillMode: Image.PreserveAspectCrop
            mipmap: true
            asynchronous: true
            onStatusChanged: if (status === Image.Error && root.sourceIndex + 1 < root.sources.length) root.sourceIndex++
        }
    }
    LoginText {
        anchors.centerIn: parent
        visible: image.status !== Image.Ready
        text: (root.name || "?").charAt(0).toUpperCase()
        color: root.ink
        font.family: root.fontFamily
        font.pixelSize: Math.round(root.height * 0.42)
        font.weight: Font.DemiBold
    }
    Item {
        id: mask
        anchors.fill: parent
        layer.enabled: true
        visible: false
        Rectangle { anchors.fill: parent; radius: width / 2 }
    }
}
