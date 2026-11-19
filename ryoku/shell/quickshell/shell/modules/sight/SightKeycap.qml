import QtQuick
import Lithic

Row {
    id: root
    property string key: ""
    property string label: ""
    property real s: 1
    spacing: 6 * s
    CutRect {
        anchors.verticalCenter: parent.verticalCenter
        width: Math.max(height, keyText.implicitWidth + 10 * root.s)
        height: 20 * root.s
        cut: 4
        fill: Lithic.field
        strokeColor: Lithic.vein
        strokeWidth: 1
        Label { id: keyText; anchors.centerIn: parent; typeRole: "mono"; inkRole: "fg2"; text: root.key }
    }
    Label { anchors.verticalCenter: parent.verticalCenter; visible: root.label.length > 0; typeRole: "detail"; inkRole: "fg2"; text: root.label }
}
