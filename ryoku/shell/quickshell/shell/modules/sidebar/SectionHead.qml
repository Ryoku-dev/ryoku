import QtQuick
import Lithic
import "../ask/look"

Item {
    id: root

    property real s: 1
    property string title: ""
    property string ordinal: ""

    implicitHeight: Math.round((QsLook.lithic ? 14 : 18) * root.s)

    Text {
        id: name
        anchors.left: parent.left
        anchors.verticalCenter: parent.verticalCenter
        text: root.title
        color: QsLook.lithic ? Lithic.fg3 : QsLook.headInk
        font.family: QsLook.lithic ? QsLook.mono : QsLook.sans
        font.pixelSize: Math.round((QsLook.lithic ? 9 : 12) * root.s)
        font.weight: QsLook.lithic ? Font.Normal : Font.DemiBold
        font.capitalization: QsLook.lithic ? Font.AllUppercase : Font.MixedCase
        font.letterSpacing: Look.tracking * root.s
        renderType: Text.NativeRendering
    }
    Rectangle {
        visible: QsLook.lithic
        anchors.left: name.right
        anchors.leftMargin: Math.round(8 * root.s)
        anchors.right: mark.left
        anchors.rightMargin: Math.round(8 * root.s)
        anchors.verticalCenter: parent.verticalCenter
        height: Math.max(1, Math.round(root.s))
        color: Lithic.vein
    }
    Text {
        id: mark
        visible: QsLook.lithic && root.ordinal.length > 0
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        text: root.ordinal
        color: Lithic.fg3
        font.family: QsLook.mono
        font.pixelSize: Math.round(9 * root.s)
        renderType: Text.NativeRendering
    }
}
