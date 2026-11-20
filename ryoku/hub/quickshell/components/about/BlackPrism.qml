import QtQuick
import Lithic

Item {
    id: prism
    property bool live: true
    readonly property bool moving: live && visible && !Lithic.reduceMotion

    clip: true
    implicitWidth: 640
    implicitHeight: 400

    Rectangle {
        anchors.fill: parent
        color: "#060812"
    }

    AnimatedImage {
        id: matter
        anchors.fill: parent
        source: Qt.resolvedUrl("../../assets/rewards/black-prism.gif")
        sourceSize.width: Math.min(1024, Math.max(1, Math.round(width * 2)))
        sourceSize.height: Math.min(640, Math.max(1, Math.round(height * 2)))
        asynchronous: true
        fillMode: Image.PreserveAspectCrop
        smooth: true
        playing: prism.moving
        visible: status === AnimatedImage.Ready
    }

    Rectangle {
        anchors.fill: parent
        color: "#050611"
        opacity: 0.22
    }
}
