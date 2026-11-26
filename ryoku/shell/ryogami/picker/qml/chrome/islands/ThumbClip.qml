import QtQuick
import QtQuick.Effects
import Lithic

// A thumbnail cut to the style's corner
Item {
    id: clip

    property url source
    property real cutSize: 4
    property int decode: 192
    property alias status: img.status

    Image {
        id: img
        anchors.fill: parent
        source: clip.source
        sourceSize.width: clip.decode
        fillMode: Image.PreserveAspectCrop
        asynchronous: true
        smooth: true
        visible: false
    }
    CutRect {
        id: mask
        anchors.fill: parent
        cut: clip.cutSize
        fill: "white"
        layer.enabled: true
        visible: false
    }
    CutRect {
        anchors.fill: parent
        cut: clip.cutSize
        fill: Lithic.field
        visible: img.status !== Image.Ready
    }
    MultiEffect {
        anchors.fill: parent
        source: img
        visible: img.status === Image.Ready
        maskEnabled: true
        maskSource: mask
        maskThresholdMin: 0.5
        maskSpreadAtMin: 1
    }
}
