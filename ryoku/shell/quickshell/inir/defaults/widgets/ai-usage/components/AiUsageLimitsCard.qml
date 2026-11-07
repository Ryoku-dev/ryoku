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

AiUsageGlassCard {
    required property var usage
    id: card
    radius: page.radius
    implicitHeight: body.implicitHeight + 2 * page.pad
    // The primary row stretches every card to the tallest one, so this card can
    // be taller than its content. Centre the content in that spare height so
    // the blank space splits above and below instead of collecting under the
    // text. At the content-sized height the offset is exactly page.pad, so the
    // natural card does not move.
    readonly property real spareHeight: card.height - card.implicitHeight

    Column {
        id: body
        x: page.pad
        y: page.pad + Math.round(Math.max(0, card.spareHeight) / 2)
        width: card.width - 2 * page.pad
        spacing: page.step * 3

        Item {
            width: parent.width
            height: Math.max(agentName.implicitHeight, planText.implicitHeight)
            IrisText {
                id: agentName
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                text: page.toolLabel
                font.pixelSize: Math.round(12 * IrisStyle.density)
                font.weight: IrisStyle.weight(Font.DemiBold)
            }
            IrisText {
                id: planText
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                role: IrisText.Meta
                color: IrisStyle.muted
                font.pixelSize: Math.round(10 * IrisStyle.density)
                text: usage.plan.length > 0 ? usage.plan : ""
            }
        }

        Item {
            width: parent.width
            height: todayRow.implicitHeight
            Row {
                id: todayRow
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                spacing: Math.round(7 * IrisStyle.density)
                IrisText {
                    id: todayMetric
                    role: IrisText.Metric
                    text: usage.kmg(usage.today)
                    font.pixelSize: Math.round(26 * IrisStyle.density)
                }
                IrisText {
                    anchors.baseline: todayMetric.baseline
                    role: IrisText.Meta
                    color: IrisStyle.muted
                    font.pixelSize: Math.round(10 * IrisStyle.density)
                    text: "tokens today · incl. cached"
                }
            }
        }

        Repeater {
            model: card.usage.windows
            AiUsageWindowRow {
                required property var modelData
                width: parent.width
                label: modelData.label
                caption: modelData.caption
                share: modelData.pct + "% used"
                proportion: modelData.pct / 100
                resetLabel: card.usage.resetIn(modelData.resetTs)
            }
        }
    }
}
