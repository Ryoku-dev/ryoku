pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import inir.modules.common
import inir.modules.iris.components
import inir.modules.iris.style
import inir.services

Item {
    id: root

    required property var keybindData
    readonly property var keys: KeybindCatalogue.keysOf(keybindData)
    implicitHeight: Math.round(42 * IrisStyle.density)

    HoverHandler { id: hover }
    Rectangle {
        anchors.fill: parent
        radius: IrisStyle.radiusRow
        color: hover.hovered ? IrisStyle.fillHover : "transparent"
        Behavior on color { ColorAnimation { duration: IrisStyle.duration(110); easing.type: IrisStyle.feedbackEasing } }
    }
    IrisText {
        anchors.left: parent.left
        anchors.leftMargin: Math.round(10 * IrisStyle.density)
        anchors.right: caps.left
        anchors.rightMargin: Math.round(10 * IrisStyle.density)
        anchors.verticalCenter: parent.verticalCenter
        text: String(root.keybindData?.label ?? root.keybindData?.desc ?? "")
        color: IrisStyle.text
        font.pixelSize: IrisStyle.typeLabel
        elide: Text.ElideRight
    }
    Row {
        id: caps
        anchors.right: parent.right
        anchors.rightMargin: Math.round(8 * IrisStyle.density)
        anchors.verticalCenter: parent.verticalCenter
        spacing: Math.round(3 * IrisStyle.density)
        Repeater {
            model: root.keys
            Rectangle {
                required property var modelData
                implicitWidth: Math.max(label.implicitWidth + Math.round(12 * IrisStyle.density), Math.round(26 * IrisStyle.density))
                implicitHeight: Math.round(24 * IrisStyle.density)
                radius: IrisStyle.radiusMicro
                color: IrisStyle.fill
                border.width: 1
                border.color: IrisStyle.hairlineStrong
                IrisText {
                    id: label
                    anchors.centerIn: parent
                    text: String(parent.modelData)
                    font.pixelSize: IrisStyle.typeMeta
                    font.weight: IrisStyle.weight(Font.Medium)
                }
            }
        }
    }
}
