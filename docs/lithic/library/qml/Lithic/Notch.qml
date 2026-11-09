import QtQuick
import QtQuick.Shapes

Item {
    id: root

    property real size: LithicTokens.cut.notch
    property color fill: Lithic.vein
    property alias toneColor: root.fill
    property string corner: "topLeft"
    implicitWidth: size
    implicitHeight: size

    readonly property bool _right: corner === "topRight" || corner === "bottomRight"
    readonly property bool _bottom: corner === "bottomLeft" || corner === "bottomRight"

    Shape {
        anchors.fill: parent
        antialiasing: true
        ShapePath {
            fillColor: root.fill
            strokeColor: "transparent"
            startX: root._right ? root.width : 0
            startY: root._bottom ? root.height : 0
            PathLine {
                x: root._right ? root.width - root.size : root.size
                y: root._bottom ? root.height : 0
            }
            PathLine {
                x: root._right ? root.width : 0
                y: root._bottom ? root.height - root.size : root.size
            }
            PathLine { x: root._right ? root.width : 0; y: root._bottom ? root.height : 0 }
        }
    }
}
