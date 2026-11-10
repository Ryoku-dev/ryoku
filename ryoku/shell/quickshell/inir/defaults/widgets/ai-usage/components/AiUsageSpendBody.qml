pragma ComponentBehavior: Bound

// The Spending figures without a card around them. OpenCode shows them in the
// standalone Spend card; Claude and Codex show them inside the Now card, and
// both must read the same, so this lives in one place.
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
    id: body
    // Vertical breathing room the figures want around them. The standalone
    // Spend card pads its box with it, and the block inside Now keeps its
    // bottom edge off the card with it, so the figures read the same in both
    // places instead of the amount sitting on a card edge.
    readonly property int padY: Math.round(14 * IrisStyle.density)
    implicitHeight: column.implicitHeight

    Column {
        id: column
        width: body.width
        spacing: body.page.step * 3

        Item {
            id: titleRow
            width: parent.width
            height: spendTitle.implicitHeight
            IrisText {
                id: spendTitle
                anchors.left: parent.left
                anchors.right: spendPeriod.left
                anchors.rightMargin: Math.round(6 * IrisStyle.density)
                anchors.verticalCenter: parent.verticalCenter
                text: "API Value"
                elide: Text.ElideRight
                maximumLineCount: 1
                font.pixelSize: Math.round(12 * IrisStyle.density)
                font.weight: IrisStyle.weight(Font.DemiBold)
            }
            IrisText {
                id: spendPeriod
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                role: IrisText.Meta
                color: IrisStyle.muted
                font.pixelSize: Math.round(10 * IrisStyle.density)
                text: "This month" + (body.page.planAgent
                    && (body.page.monthly.partial || body.page.monthly.stale) ? "*" : "")
            }
            MouseArea {
                id: helpArea
                anchors.fill: parent
                hoverEnabled: true
                acceptedButtons: Qt.NoButton
                StyledToolTip {
                    extraVisibleCondition: helpArea.containsMouse
                    text: body.page.spendHelp
                }
            }
        }

        Item {
            width: parent.width
            height: spendMetric.implicitHeight
            Row {
                id: spendRow
                width: parent.width
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                spacing: Math.round(6 * IrisStyle.density)
                IrisText {
                    id: spendMetric
                    role: IrisText.Metric
                    text: body.page.monthly.available === false ? "—"
                        : body.page.money(body.page.monthly.current)
                    width: Math.min(implicitWidth,
                        Math.max(0, spendRow.width - spendMonth.implicitWidth - spendRow.spacing))
                    fontSizeMode: Text.Fit
                    minimumPixelSize: Math.round(14 * IrisStyle.density)
                    font.pixelSize: Math.round(24 * IrisStyle.density)
                }
                IrisText {
                    id: spendMonth
                    anchors.baseline: spendMetric.baseline
                    role: IrisText.Meta
                    color: IrisStyle.muted
                    font.pixelSize: Math.round(10 * IrisStyle.density)
                    text: body.page.monthly.label
                }
            }
        }
    }
}
