import QtQuick
import Lithic
import "look"

Item {
    id: root

    property string label: ""
    property string value: ""

    readonly property real _scale: Lithic.scaleOf(root)

    implicitHeight: 20 * root._scale

    Label {
        anchors.left: parent.left
        anchors.verticalCenter: parent.verticalCenter
        width: Math.max(0, root.width - valueLabel.width - 8 * root._scale)
        text: root.label
        typeRole: "detail"
        inkRole: "fg3"
    }

    Label {
        id: valueLabel
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        width: Math.min(implicitWidth, Math.max(0, root.width * 0.6))
        text: root.value
        typeRole: "detail"
        ink: GameBarTone.ink
        numeric: true
    }
}
