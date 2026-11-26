import QtQuick
import Lithic

Item {
    id: label

    property string text: ""

    implicitHeight: Math.max(caption.implicitHeight, 14 * Theme.scale)

    Text {
        id: caption
        anchors.left: parent.left
        anchors.verticalCenter: parent.verticalCenter
        text: label.text
        font.family: Theme.lithic ? Lithic.font.mono : Lithic.font.sans
        font.weight: Theme.lithic ? Font.Normal : Font.DemiBold
        font.pixelSize: Theme.fs(Theme.lithic ? 9.5 : 12)
        font.letterSpacing: Theme.lithic ? 1.6 : 0
        font.capitalization: Theme.lithic ? Font.AllUppercase : Font.MixedCase
        color: Lithic.fg3
        renderType: Text.NativeRendering
    }
    Rectangle {
        anchors.left: caption.right
        anchors.leftMargin: 12 * Theme.scale
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        height: 1
        color: Lithic.vein
    }
}
