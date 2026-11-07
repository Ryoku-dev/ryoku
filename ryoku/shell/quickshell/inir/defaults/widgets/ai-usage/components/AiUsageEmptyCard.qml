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

    Column {
        id: body
        x: page.pad
        y: page.pad
        width: card.width - 2 * page.pad
        spacing: page.step * 3

        Item {
            width: parent.width
            height: Math.max(emptyTitle.implicitHeight, refreshBtn.implicitHeight)
            IrisText {
                id: emptyTitle
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                text: "AI usage · " + page.toolLabel
                font.pixelSize: Math.round(12 * IrisStyle.density)
                font.weight: IrisStyle.weight(Font.DemiBold)
            }
            IrisIconButton {
                id: refreshBtn
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                materialIcon: "refresh"
                iconSize: Math.round(15 * IrisStyle.density)
                onClicked: usage.refresh()
            }
        }
        IrisText {
            width: parent.width
            role: IrisText.Meta
            color: IrisStyle.muted
            text: "No " + page.tool + " usage yet. Run " + page.tool
                + ", then tap ↻ — the collector writes ~/.cache/" + page.tool + "-usage.json."
            wrapMode: Text.WordWrap
        }
    }
}

// ─── pieces ──────────────────────────────────────────────────────────

