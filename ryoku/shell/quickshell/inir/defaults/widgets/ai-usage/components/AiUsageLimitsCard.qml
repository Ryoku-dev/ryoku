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
    // Last-known Claude percentages are useful, but must carry their capture
    // age rather than masquerade as a fresh account response.
    readonly property var displayedWindows: usage.windows.length > 0 ? usage.windows
        : page.activeTool === "claude" && usage.quota?.stale === true
            ? usage.normalizeWindows({ _windows: usage.quota.last_known_windows ?? [] }) : []

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
                width: parent.width
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                spacing: Math.round(7 * IrisStyle.density)
                IrisText {
                    id: todayMetric
                    role: IrisText.Metric
                    text: usage.kmg(usage.today)
                    font.pixelSize: Math.round(26 * IrisStyle.density)
                }
                Column {
                    width: Math.max(0, todayRow.width - todayMetric.width - todayRow.spacing)
                    anchors.verticalCenter: todayMetric.verticalCenter
                    spacing: 0
                    IrisText {
                        width: parent.width
                        role: IrisText.Meta
                        color: IrisStyle.muted
                        font.pixelSize: Math.round(10 * IrisStyle.density)
                        text: Translation.tr("tokens today")
                        elide: Text.ElideRight
                        maximumLineCount: 1
                    }
                    IrisText {
                        width: parent.width
                        role: IrisText.Meta
                        color: IrisStyle.muted
                        font.pixelSize: Math.round(10 * IrisStyle.density)
                        text: Translation.tr("(incl. cached)")
                        elide: Text.ElideRight
                        maximumLineCount: 1
                    }
                }
            }
        }

        Repeater {
            model: card.displayedWindows
            AiUsageWindowRow {
                required property var modelData
                width: parent.width
                label: modelData.label
                caption: modelData.caption
                share: modelData.pct + "% used"
                proportion: modelData.pct / 100
                resetLabel: modelData.resetTs > 0 ? card.usage.resetAt(modelData.resetTs, page.translateTime)
                    : page.activeTool === "claude" ? Translation.tr("reset time unavailable") : ""
            }
        }

        IrisText {
            id: quotaNote
            visible: page.activeTool === "claude"
            width: parent.width
            role: IrisText.Meta
            color: IrisStyle.muted
            font.pixelSize: Math.round(9 * IrisStyle.density)
            wrapMode: Text.Wrap
            text: card.displayedWindows.length === 0 ? Translation.tr("Usage limits unavailable")
                : (usage.quota?.stale === true ? Translation.tr("Last known · %1") : Translation.tr("Account · %1"))
                    .arg(usage.relative(Number(usage.quota?.observed_at ?? 0) * 1000, page.translateTime))
            MouseArea {
                id: quotaHelp
                anchors.fill: parent
                hoverEnabled: true
                acceptedButtons: Qt.NoButton
                StyledToolTip {
                    extraVisibleCondition: quotaHelp.containsMouse
                    text: (Translation.tr(String(card.usage.quota?.message ?? "")) || Translation.tr("Account usage unavailable"))
                        + "\n" + Translation.tr("Desktop and CLI subscription logins are supported.")
                        + "\n" + Translation.tr("Limits include use on other devices.")
                        + "\n" + Translation.tr("Reset times use your local time zone.")
                        + (card.usage.quota?.stale === true
                            ? "\n" + Translation.tr("Saved usage may be out of date.") : "")
                }
            }
        }
    }
}
