import QtQuick
import QtQuick.Shapes

Item {
    id: root

    property real fraction: 0
    property real explicitWidth: -1
    property alias progress: root.fraction
    property color fill: Lithic.fg2
    property alias bladeColor: root.fill
    property color track: Lithic.field
    property alias trackColor: root.track
    readonly property real bladeWidth: Math.max(0, Math.min(width,
        explicitWidth >= 0 ? explicitWidth : fraction * width))
    implicitHeight: LithicTokens.size.bladeTrack

    Shape {
        anchors.fill: parent
        antialiasing: true
        ShapePath {
            fillColor: root.track
            strokeColor: "transparent"
            startX: 0
            startY: 0
            PathLine { x: root.width; y: 0 }
            PathLine { x: root.width; y: root.height }
            PathLine { x: 0; y: root.height }
            PathLine { x: 0; y: 0 }
        }
    }

    Shape {
        width: root.bladeWidth
        height: root.height
        visible: width > 0
        antialiasing: true
        ShapePath {
            fillColor: root.fill
            strokeColor: "transparent"
            startX: 0
            startY: 0
            PathLine { x: Math.max(0, root.bladeWidth - root.height); y: 0 }
            PathLine { x: root.bladeWidth; y: Math.min(root.height, root.bladeWidth) }
            PathLine { x: root.bladeWidth; y: root.height }
            PathLine { x: 0; y: root.height }
            PathLine { x: 0; y: 0 }
        }
    }
}
