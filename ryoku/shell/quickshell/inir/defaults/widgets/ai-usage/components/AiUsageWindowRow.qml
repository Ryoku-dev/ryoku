pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Quickshell
import Quickshell.Widgets
import inir
import inir.services
import inir.modules.common
import inir.modules.common.widgets
import inir.modules.common.functions
import inir.modules.iris.style
import inir.modules.iris.components

Column {
    id: windowRow
    property string label: ""
    property string caption: ""
    property string share: ""
    property real proportion: 0
    property string resetLabel: ""
    width: parent ? parent.width : 0
    spacing: Math.round(3 * IrisStyle.density)

    readonly property color ink: windowRow.proportion >= 0.95 ? IrisStyle.danger
        : windowRow.proportion >= 0.8 ? IrisStyle.warning
        : IrisStyle.accent

    Item {
        width: parent.width
        height: Math.max(windowLabel.implicitHeight, windowShare.implicitHeight,
            windowCaption.implicitHeight)
        IrisText {
            id: windowLabel
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            role: IrisText.Eyebrow
            font.pixelSize: Math.round(10 * IrisStyle.density)
            color: IrisStyle.muted
            text: windowRow.label
        }
        IrisText {
            id: windowCaption
            anchors.left: windowLabel.right
            anchors.leftMargin: Math.round(5 * IrisStyle.density)
            anchors.right: windowShare.left
            anchors.rightMargin: Math.round(6 * IrisStyle.density)
            anchors.verticalCenter: parent.verticalCenter
            role: IrisText.Eyebrow
            font.pixelSize: Math.round(9 * IrisStyle.density)
            color: IrisStyle.muted
            elide: Text.ElideRight
            text: windowRow.caption
        }
        IrisText {
            id: windowShare
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            role: IrisText.Meta
            font.pixelSize: Math.round(11 * IrisStyle.density)
            font.weight: IrisStyle.weight(Font.DemiBold)
            font.family: IrisStyle.fontNumbers
            color: windowRow.proportion >= 0.8 ? windowRow.ink : IrisStyle.text
            text: windowRow.share
        }
    }
    Rectangle {
        width: parent.width
        height: Math.round(5 * IrisStyle.density)
        radius: height / 2
        color: ColorUtils.applyAlpha(IrisStyle.muted, 0.22)
        Rectangle {
            height: parent.height
            radius: parent.radius
            width: Math.max(0, Math.min(1, windowRow.proportion)) * parent.width
            color: windowRow.ink
        }
    }
    IrisText {
        visible: windowRow.resetLabel.length > 0
        width: parent.width
        role: IrisText.Eyebrow
        font.pixelSize: Math.round(9 * IrisStyle.density)
        color: IrisStyle.muted
        text: windowRow.resetLabel
    }
}
