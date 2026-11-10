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

Item {
    required property var page
    id: modelRow
    required property var modelData
    readonly property int rowGap: Math.round(4 * IrisStyle.density)
    readonly property int meterHeight: Math.round(5 * IrisStyle.density)
    readonly property int nameHeight: Math.max(nameText.implicitHeight,
        Math.round(14 * IrisStyle.density))
    height: nameHeight + rowGap + meterHeight + rowGap + detailText.implicitHeight
    implicitHeight: height

    IrisText {
        id: nameText
        anchors.left: parent.left
        anchors.top: parent.top
        width: parent.width * 0.72
        height: modelRow.nameHeight
        text: modelRow.modelData?.nameShort ?? ""
        textFormat: Text.PlainText
        elide: Text.ElideRight
        font.pixelSize: Math.round(12 * IrisStyle.density)
        font.weight: IrisStyle.weight(Font.Medium)
    }
    IrisText {
        anchors.right: parent.right
        anchors.top: parent.top
        text: modelRow.modelData?.totalWithCacheReadLabel
            ?? modelRow.modelData?.totalLabel ?? ""
        role: IrisText.Metric
        color: IrisStyle.accent
        font.family: IrisStyle.fontNumbers
        font.pixelSize: Math.round(13 * IrisStyle.density)
    }
    Rectangle {
        id: modelMeter
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: nameText.bottom
        anchors.topMargin: modelRow.rowGap
        height: modelRow.meterHeight
        radius: height / 2
        color: ColorUtils.applyAlpha(IrisStyle.muted, 0.18)
        Rectangle {
            width: parent.width * Math.max(0, Math.min(100,
                Number(modelRow.modelData?.pct ?? 0))) / 100
            height: parent.height
            radius: parent.radius
            color: IrisStyle.accent
            opacity: 0.85
        }
    }
    IrisText {
        id: detailText
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: modelMeter.bottom
        anchors.topMargin: modelRow.rowGap
        text: page.modelDetail(modelRow.modelData)
        elide: Text.ElideRight
        role: IrisText.Eyebrow
        font.pixelSize: Math.round(11 * IrisStyle.density)
        color: IrisStyle.subtext
    }
}
