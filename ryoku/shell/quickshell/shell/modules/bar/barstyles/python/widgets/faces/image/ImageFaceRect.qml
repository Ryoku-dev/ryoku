import QtQuick
import QtQuick.Layouts
import "../../../reusables"
import "../../../"

Item {
    id: root
    anchors.fill: parent
    clip: true

    property real minWidth: 30
    property real minHeight: 30
    property real maxWidth: 4000
    property real maxHeight: 4000
    property real minAspect: 0.02
    property real maxAspect: 50.0
    property bool isRound: false
    // The Stage slot can own the backing without covering this face.
    property bool ryokuBare: false


    property string imagePath: ""
    property bool fitCrop: false

    Rectangle {
        anchors.fill: parent
        color: (root.ryokuBare || root.imagePath !== "") ? "transparent" : ThemeBackend.surface0
        clip: true

        BoundedImage {
            id: srcImage
            anchors.fill: parent
            source: root.imagePath !== "" ? (root.imagePath.startsWith("file://") ? root.imagePath : "file://" + root.imagePath) : ""
            fillMode: root.fitCrop ? Image.PreserveAspectCrop : Image.PreserveAspectFit
            asynchronous: true
            cache: true
            smooth: true
            antialiasing: true
            playing: root.visible
            visible: status === Image.Ready
        }

        Item {
            anchors.fill: parent
            visible: root.imagePath === "" || srcImage.status !== Image.Ready

            Text {
                anchors.centerIn: parent
                text: "󰋩"
                font.family: ThemeBackend.iconFont
                font.pixelSize: Math.max(16, Math.min(root.width, root.height) * 0.35)
                color: ThemeBackend.subtext0
            }
        }
    }
}
