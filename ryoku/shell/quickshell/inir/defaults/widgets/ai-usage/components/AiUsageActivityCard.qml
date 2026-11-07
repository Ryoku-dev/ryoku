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
    radius: page.radius
    implicitHeight: body.implicitHeight + 2 * page.pad

    readonly property string todayKey: Qt.formatDate(new Date(), "yyyy-MM-dd")
    readonly property int cellSize: Math.max(7, Math.round(11 * IrisStyle.density))
    readonly property int cellGap: Math.max(2, Math.round(3 * IrisStyle.density))
    readonly property int cellRadius: Math.max(2, Math.round(3 * IrisStyle.density))

    // Everything a day spent, cache reads included.
    function dayWeight(day) {
        return Number(day?.tokens ?? 0) + Number(day?.cacheRead ?? 0)
    }

    // Sixty-three calendar days ending today in nine full seven-day columns.
    // No partial columns; missing days remain dim squares.
    readonly property var map: {
        const days = usage.days
        if (!days || days.length === 0)
            return ({ weeks: [], total: 0, active: 0, busiest: null, span: 0, thresholds: [] })
        const byDate = ({})
        for (const day of days)
            byDate[String(day?.date ?? "")] = card.dayWeight(day)
        const today = new Date()
        today.setHours(12, 0, 0, 0)
        const start = new Date(today.getTime())
        start.setDate(start.getDate() - 62)
        const cursor = new Date(start.getTime())
        const weekCount = 9
        let active = 0
        let busiest = null
        let span = 0
        const values = []
        const weeks = []
        for (let w = 0; w < weekCount; w++) {
            const column = []
            for (let r = 0; r < 7; r++) {
                const key = Qt.formatDate(cursor, "yyyy-MM-dd")
                if (cursor.getTime() < start.getTime() || cursor.getTime() > today.getTime()) {
                    // Dates outside the 63-day window are padding, not zero.
                    column.push(null)
                } else {
                    const recorded = byDate[key] !== undefined
                    const value = Number(byDate[key] ?? 0) || 0
                    column.push({ date: key, value: value })
                    if (value > 0) {
                        active += 1
                        values.push(value)
                    }
                    if (!busiest || value > busiest.value)
                        busiest = { date: key, value: value }
                    if (recorded && span === 0)
                        span = Math.round((today.getTime() - cursor.getTime()) / 86400000) + 1
                }
                cursor.setDate(cursor.getDate() + 1)
            }
            weeks.push(column)
        }
        values.sort((a, b) => a - b)
        const thresholds = values.length > 0
            ? [0.25, 0.5, 0.75].map(q => values[Math.min(values.length - 1, Math.floor(values.length * q))])
            : []
        return ({ weeks: weeks, active: active, busiest: busiest,
                  span: span, thresholds: thresholds })
    }

    // Consecutive active days ending today; a day still in progress counts
    // back from yesterday rather than breaking the run.
    readonly property int streak: {
        const days = usage.days
        if (!days || days.length === 0) return 0
        const byDate = ({})
        for (const day of days)
            byDate[String(day?.date ?? "")] = card.dayWeight(day)
        const cursor = new Date()
        cursor.setHours(12, 0, 0, 0)
        if (!(Number(byDate[Qt.formatDate(cursor, "yyyy-MM-dd")] ?? 0) > 0))
            cursor.setDate(cursor.getDate() - 1)
        let count = 0
        while (Number(byDate[Qt.formatDate(cursor, "yyyy-MM-dd")] ?? 0) > 0) {
            count += 1
            cursor.setDate(cursor.getDate() - 1)
        }
        return count
    }

    function spanLabel(days: int): string {
        if (days <= 0) return ""
        if (days >= 7 && days % 7 === 0)
            return (days / 7) + (days === 7 ? " week" : " weeks")
        return days + (days === 1 ? " day" : " days")
    }

    // The card's headline is the whole history, not just the mapped weeks.
    readonly property double allTimeTotal: {
        let total = 0
        for (const day of usage.days) total += card.dayWeight(day)
        return total
    }

    function cellColor(cell) {
        if (!cell) return "transparent"
        const value = Number(cell.value ?? 0)
        if (value <= 0) return ColorUtils.applyAlpha(IrisStyle.text, 0.07)
        const level = card.map.thresholds.filter(limit => value > limit).length
        return ColorUtils.applyAlpha(IrisStyle.accent, [0.28, 0.45, 0.65, 0.9][Math.min(3, level)])
    }

    Column {
        id: body
        x: page.pad
        y: page.pad
        width: card.width - 2 * page.pad
        spacing: page.step * 2

        AiUsageFoldHeader {
            title: "Activity"
            expanded: card.expanded
            onToggled: card.expanded = !card.expanded
            // The run of active days, with its own explanation on hover.
            Item {
                id: streakBadge
                visible: card.streak > 1
                width: streakIcon.width + Math.round(2 * IrisStyle.density) + streakText.width
                height: Math.max(streakIcon.height, streakText.height)
                MaterialSymbol {
                    id: streakIcon
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    text: "local_fire_department"
                    iconSize: Math.round(15 * IrisStyle.density)
                    color: IrisStyle.warning
                }
                IrisText {
                    id: streakText
                    anchors.left: streakIcon.right
                    anchors.leftMargin: Math.round(2 * IrisStyle.density)
                    anchors.verticalCenter: parent.verticalCenter
                    text: String(card.streak)
                    color: IrisStyle.warning
                    font.pixelSize: Math.round(10 * IrisStyle.density)
                    font.weight: IrisStyle.weight(Font.DemiBold)
                }
                MouseArea {
                    id: badgeArea
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onClicked: card.expanded = !card.expanded
                }
                IrisSurface {
                    id: streakTip
                    visible: badgeArea.containsMouse
                    z: 10
                    raised: true
                    outlined: true
                    radius: IrisStyle.radiusSmall
                    anchors.horizontalCenter: parent.horizontalCenter
                    anchors.bottom: parent.top
                    anchors.bottomMargin: Math.round(4 * IrisStyle.density)
                    width: streakTipText.implicitWidth + 2 * Math.round(6 * IrisStyle.density)
                    height: streakTipText.implicitHeight + 2 * Math.round(3 * IrisStyle.density)
                    IrisText {
                        id: streakTipText
                        anchors.centerIn: parent
                        role: IrisText.Meta
                        color: IrisStyle.muted
                        font.pixelSize: Math.round(10 * IrisStyle.density)
                        text: card.streak + "-day streak"
                    }
                }
            }
        }

        Item {
            id: content
            width: parent.width
            height: Math.max(heat.implicitHeight, Math.round(80 * IrisStyle.density))
            visible: card.expanded

            Row {
                id: heat
                anchors.left: parent.left
                anchors.top: parent.top
                spacing: card.cellGap
                Repeater {
                    model: card.map.weeks
                    Column {
                        required property var modelData
                        spacing: card.cellGap
                        Repeater {
                            model: modelData
                            Rectangle {
                                id: heatCell
                                required property var modelData
                                width: card.cellSize
                                height: width
                                radius: card.cellRadius
                                color: card.cellColor(heatCell.modelData)
                                border.width: heatCell.modelData !== null
                                    && String(heatCell.modelData.date) === card.todayKey ? 1 : 0
                                border.color: IrisStyle.accent
                            }
                        }
                    }
                }
            }

            // The figures take the rest of the row and sit on the card's
            // inner right edge, as the reference does.
            Item {
                id: figures
                anchors.left: heat.right
                anchors.leftMargin: Math.round(16 * IrisStyle.density)
                anchors.right: parent.right
                anchors.top: parent.top
                anchors.bottom: parent.bottom

                IrisText {
                    id: totalValue
                    anchors.top: parent.top
                    anchors.left: parent.left
                    role: IrisText.Metric
                    text: usage.kmg(card.allTimeTotal)
                    font.pixelSize: Math.round(21 * IrisStyle.density)
                }
                IrisText {
                    id: spanValue
                    anchors.top: totalValue.bottom
                    anchors.topMargin: Math.round(1 * IrisStyle.density)
                    anchors.left: parent.left
                    role: IrisText.Eyebrow
                    color: IrisStyle.muted
                    font.pixelSize: Math.round(10 * IrisStyle.density)
                    text: "all time"
                }
                Item {
                    id: activeRow
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.bottom: busiestRow.top
                    anchors.bottomMargin: Math.round(3 * IrisStyle.density)
                    height: Math.max(activeLabel.implicitHeight, activeValue.implicitHeight)
                    IrisText {
                        id: activeLabel
                        anchors.left: parent.left
                        anchors.verticalCenter: parent.verticalCenter
                        role: IrisText.Eyebrow
                        color: IrisStyle.muted
                        font.pixelSize: Math.round(10 * IrisStyle.density)
                        text: "Active days"
                    }
                    IrisText {
                        id: activeValue
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        role: IrisText.Meta
                        font.pixelSize: Math.round(10 * IrisStyle.density)
                        text: String(card.map.active)
                    }
                }
                Item {
                    id: busiestRow
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.bottom: parent.bottom
                    height: Math.max(busiestLabel.implicitHeight, busiestValue.implicitHeight)
                    IrisText {
                        id: busiestLabel
                        anchors.left: parent.left
                        anchors.verticalCenter: parent.verticalCenter
                        role: IrisText.Eyebrow
                        color: IrisStyle.muted
                        font.pixelSize: Math.round(10 * IrisStyle.density)
                        text: "Busiest day"
                    }
                    IrisText {
                        id: busiestValue
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        role: IrisText.Meta
                        font.pixelSize: Math.round(10 * IrisStyle.density)
                        text: card.map.busiest && card.map.busiest.value > 0
                            ? page.dayLabel(card.map.busiest.date) + " · "
                                + usage.kmg(card.map.busiest.value)
                            : "—"
                    }
                }
            }
        }
    }
}
