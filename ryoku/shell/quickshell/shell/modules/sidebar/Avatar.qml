import QtQuick
import Quickshell
import Ryoku.Ui
import Lithic

Item {
    id: root

    property real s: 1
    property real size: 36
    property string userName: ""
    readonly property string initial: root.userName.length > 0 ? root.userName.charAt(0).toUpperCase() : "·"

    implicitWidth: Math.round(root.size * root.s)
    implicitHeight: implicitWidth

    CutRect {
        visible: QsLook.lithic
        anchors.fill: parent
        cut: LithicTokens.cut.small * root.s
        fill: Lithic.field
        strokeColor: Lithic.vein
        strokeWidth: Math.max(1, Math.round(root.s))
    }
    Rectangle {
        visible: !QsLook.lithic
        anchors.fill: parent
        radius: width / 2
        color: QsLook.material ? Lithic.container : Lithic.mix(Lithic.field, Lithic.fg, 0.06)
    }
    Text {
        anchors.centerIn: parent
        visible: QsLook.lithic || !picture.item || !picture.item.hasImage
        text: root.initial
        color: QsLook.lithic ? Lithic.sel : QsLook.material ? Lithic.containerInk : Lithic.fg
        font.family: QsLook.lithic ? QsLook.mono : QsLook.sans
        font.pixelSize: Math.round(root.size * 0.42 * root.s)
        font.weight: QsLook.lithic ? Font.Normal : Font.DemiBold
        renderType: Text.NativeRendering
    }
    // only the round designs show a picture, so only they look for one
    Loader {
        id: picture
        anchors.fill: parent
        active: !QsLook.lithic
        sourceComponent: AvatarImage {
            userName: root.userName
            home: Quickshell.env("HOME") || ""
        }
    }
}
