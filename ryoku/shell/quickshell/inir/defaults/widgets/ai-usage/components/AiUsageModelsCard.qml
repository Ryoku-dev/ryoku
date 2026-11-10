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
    property bool expanded: false
    readonly property int modelCount: Number(page.activePeriod?.modelCount ?? page.periodModels.length)
    radius: page.radius
    implicitHeight: body.implicitHeight + 2 * page.pad

    Column {
        id: body
        x: page.pad
        y: page.pad
        width: card.width - 2 * page.pad
        spacing: page.step * 3

        AiUsageFoldHeader {
            title: "Models"
            expanded: card.expanded
            onToggled: card.expanded = !card.expanded
            IrisText {
                id: modelsCount
                role: IrisText.Meta
                color: IrisStyle.muted
                font.pixelSize: Math.round(10 * IrisStyle.density)
                text: card.modelCount > page.topModels
                    ? "top " + page.topModels + " of " + card.modelCount
                    : card.modelCount + " model"
                        + (card.modelCount === 1 ? "" : "s")
            }
        }

        Column {
            id: content
            width: parent.width
            visible: card.expanded
            spacing: page.step * 3

            Repeater {
                model: page.periodModels.slice(0, page.topModels)
                AiUsageModelRow {
                page: card.page
                    width: content.width
                }
            }

            IrisText {
                width: parent.width
                visible: card.modelCount > page.topModels
                role: IrisText.Meta
                color: IrisStyle.muted
                font.pixelSize: Math.round(9 * IrisStyle.density)
                text: "+" + (card.modelCount - page.topModels) + " more model"
                    + (card.modelCount - page.topModels === 1 ? "" : "s")
            }
        }
    }
}
