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
    signal periodMenuRequested(var trigger)
    id: card
    property bool expanded: false
    radius: page.radius
    implicitHeight: body.implicitHeight + 2 * page.pad

    readonly property string caption: {
        const buckets = page.trendBuckets
        if (buckets.length === 0) return ""
        if (chart.hovered >= 0 && chart.hovered < buckets.length) {
            const bucket = buckets[chart.hovered]
            return Translation.tr("%1 · %2 tokens").arg(page.bucketLabel(bucket)).arg(usage.kmg(bucket.tokens))
        }
        const first = buckets.length > 0 ? page.bucketLabel(buckets[0]) : ""
        const last = buckets.length > 0 ? page.bucketLabel(buckets[buckets.length - 1]) : ""
        const range = first === last ? first : first + " – " + last
        if (page.trend.unit === "hours") {
            return Translation.tr("%1 · %2 tokens · peak %3/h")
                .arg(range).arg(usage.kmg(page.trend.total)).arg(usage.kmg(page.trend.peak))
        }
        const window = !page.hourly && page.periodDays === 0 && buckets.length === 45
            ? " · " + Translation.tr("last 45 days") : ""
        return Translation.tr("%1 · %2 tokens · peak %3")
            .arg(range).arg(usage.kmg(page.trend.total)).arg(usage.kmg(page.trend.peak)) + window
    }

    Column {
        id: body
        x: page.pad
        y: page.pad
        width: card.width - 2 * page.pad
        spacing: page.step * 2

        AiUsageFoldHeader {
            title: Translation.tr("Trend")
            expanded: card.expanded
            onToggled: card.expanded = !card.expanded
            // A control at every width: collapsed it opens the card, expanded
            // it opens the period menu. The hit area is padded out to a
            // comfortable target instead of the label's implicit size.
            Item {
                id: periodText
                width: periodLabelText.implicitWidth
                height: periodLabelText.implicitHeight
                IrisText {
                    id: periodLabelText
                    anchors.centerIn: parent
                    role: IrisText.Meta
                    color: card.expanded && periodArea.containsMouse
                        ? IrisStyle.accent : IrisStyle.muted
                    font.pixelSize: Math.round(10 * IrisStyle.density)
                    text: page.periodLabel
                }
                MouseArea {
                    id: periodArea
                    anchors.fill: parent
                    anchors.margins: -Math.round(7 * IrisStyle.density)
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onClicked: {
                        if (!card.expanded) {
                            card.expanded = true
                            return
                        }
                        card.periodMenuRequested(periodText)
                    }
                }
            }
        }

        Column {
            id: content
            width: parent.width
            visible: card.expanded
            spacing: page.step * 2

            IrisText {
                width: parent.width
                role: IrisText.Meta
                color: IrisStyle.muted
                font.pixelSize: Math.round(10 * IrisStyle.density)
                elide: Text.ElideRight
                maximumLineCount: 1
                text: card.caption
            }

            Item {
                id: chart
                width: parent.width
                height: Math.round(36 * IrisStyle.density)
                property int hovered: -1
                readonly property int count: page.trendBuckets.length
                readonly property int barGap: count > 30
                    ? Math.round(1 * IrisStyle.density) : Math.round(2 * IrisStyle.density)
                // Bars share the card width, so a wider page widens them and a
                // narrow page keeps every bucket visible.
                readonly property real slot: (chart.width - chart.barGap
                    * Math.max(0, count - 1)) / Math.max(1, count)
                readonly property real step: chart.slot + chart.barGap

                Row {
                    id: barRow
                    anchors.fill: parent
                    spacing: chart.barGap
                    Repeater {
                        model: page.trendBuckets
                        Item {
                            id: barSlot
                            required property var modelData
                            required property int index
                            width: chart.slot
                            height: barRow.height
                            readonly property bool future: {
                                if (!page.hourly) return false
                                const nowHour = new Date()
                                nowHour.setMinutes(0, 0, 0)
                                return Number(modelData?.start ?? 0) > nowHour.getTime()
                            }
                            readonly property bool isToday: {
                                if (page.hourly) {
                                    const nowHour = new Date()
                                    nowHour.setMinutes(0, 0, 0)
                                    return Number(modelData?.start ?? 0) === nowHour.getTime()
                                }
                                return String(modelData?.date ?? "")
                                    === Qt.formatDate(new Date(), "yyyy-MM-dd")
                            }
                            readonly property real tokens: Number(modelData?.tokens ?? 0)
                            Rectangle {
                                visible: !barSlot.future
                                anchors.bottom: parent.bottom
                                anchors.horizontalCenter: parent.horizontalCenter
                                width: Math.max(1, parent.width - (parent.width > 6
                                    ? Math.round(2 * IrisStyle.density) : 0))
                                height: barSlot.tokens > 0
                                    ? Math.max(2, (chart.height - 2) * barSlot.tokens / page.trend.peak)
                                    : Math.max(2, Math.round(2 * IrisStyle.density))
                                radius: Math.min(Math.round(2 * IrisStyle.density), width / 2)
                                color: barSlot.tokens <= 0
                                    ? ColorUtils.applyAlpha(IrisStyle.muted, 0.3)
                                    : barSlot.isToday
                                        ? IrisStyle.accent
                                        : chart.hovered === barSlot.index
                                            ? IrisStyle.accent
                                            : ColorUtils.applyAlpha(IrisStyle.accent, 0.42)
                            }
                        }
                    }
                }

                MouseArea {
                    id: hoverArea
                    anchors.fill: parent
                    hoverEnabled: true
                    onPositionChanged: function (mouse) {
                        // A page that opens under a still pointer gets a synthetic
                        // move; only a pointer actually inside the chart counts.
                        if (!hoverArea.containsMouse || chart.count === 0) return
                        const local = mouse.x - barRow.x
                        if (local < 0 || local > barRow.width) {
                            chart.hovered = -1
                            return
                        }
                        chart.hovered = Math.max(0, Math.min(chart.count - 1,
                            Math.floor(local / chart.step)))
                    }
                    onExited: chart.hovered = -1
                }
            }

        }
    }
}
