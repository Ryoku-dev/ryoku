pragma ComponentBehavior: Bound

// AI usage — the Shima Island's AI page, laid out like Vorssaint's Dynamic
// Island page: a grid of small cards.
//
//   [ Limits | Now ]  ← Limits only when the cache reports a real plan
//   [    Trend     ]
//   [    Models    ]
//
// Every card is an IrisSurface on Shima tokens; the data is the shared usage
// cache (~/.cache/ai-usage/<tool>-usage.json) written by its bundled collectors.
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
import "components"

Item {
    id: root

    property string irisSlot: ""
    property var targetScreen

    // ── config (written by Shima Settings; read null-guarded here) ──
    readonly property string tool: CustomWidgets.getConfigValue("ai-usage", "tool", "opencode")
    readonly property var labels: ({ opencode: "OpenCode", claude: "Claude", codex: "Codex" })

    // ── agents this machine actually uses ──
    // A usage cache exists only after that agent ran here, so the menu offers
    // just the agents with a cache. The stored tool stays valid: if its cache
    // disappears the page falls back to the first agent that still has one
    // (OpenCode first) and writes that once.
    // Claude is enabled as a local-history preview for the requested desktop trial.
    readonly property var agentCatalog: [
        { value: "opencode", label: "OpenCode", available: availability.opencode },
        { value: "codex", label: "Codex", available: availability.codex },
        { value: "claude", label: "Claude", available: availability.claude }
    ]
    readonly property var agentOptions: root.agentCatalog.filter(option => option.available)
    readonly property bool hasAnyAgent: root.agentOptions.length > 0
    readonly property bool toolAvailable: root.agentCatalog.some(
        option => option.value === root.tool && option.available)
    // One agent means nothing to switch to: the chip stays, the chevron goes.
    readonly property bool agentSwitchable: availability.ready && root.agentOptions.length > 1
    // No cache at all: keep the card and name OpenCode, like a fresh install,
    // instead of an empty menu and a chip with nothing behind it. A stored
    // tool this build does not offer falls to
    // the first agent that has a cache, without waiting for the write back.
    readonly property string activeTool: {
        if (!availability.ready) return root.tool
        if (!root.hasAnyAgent) return "opencode"
        if (root.toolAvailable) return root.tool
        return String(root.agentOptions[0]?.value ?? "opencode")
    }
    readonly property string toolLabel: root.labels[root.activeTool] ?? root.activeTool
    property bool _fallbackWritten: false

    // The Island's Desktop page body is iris.bar.pageWidth minus its 20px
    // padding each side; a wider card would overflow the chassis and clip.
    readonly property int pageBody: Math.max(280,
        Math.max(360, Math.min(600, Number(Config.options?.iris?.bar?.pageWidth ?? 440)))
        - 2 * Math.round(20 * IrisStyle.density))
    readonly property int cardWidth: Math.max(280, Math.min(root.pageBody,
        Number(CustomWidgets.getConfigValue("ai-usage", "moduleWidth", 480))))
    readonly property int topModels: Math.max(1, Math.min(3,
        Number(CustomWidgets.getConfigValue("ai-usage", "topModels", 3))))

    readonly property var periodOptions: [
        { value: "today", label: "Today" },
        { value: "24h", label: "Last 24 hours" },
        { value: "7", label: "Last 7 days" },
        { value: "30", label: "Last 30 days" },
        { value: "all", label: "All time" }
    ]
    readonly property string periodKey: {
        const stored = String(CustomWidgets.getConfigValue("ai-usage", "period", "30"))
        return ["today", "24h", "7", "30", "all"].includes(stored) ? stored : "30"
    }
    readonly property bool hourly: root.periodKey === "today" || root.periodKey === "24h"
    readonly property int periodDays: root.periodKey === "all" ? 0 : Number(root.periodKey)
    readonly property string periodLabel: {
        const option = root.periodOptions.find(entry => entry.value === root.periodKey)
        return option ? option.label : "Last 30 days"
    }

    readonly property int pad: Math.round(10 * IrisStyle.density)
    readonly property int gap: Math.round(10 * IrisStyle.density)
    readonly property int radius: IrisStyle.radiusTile
    readonly property int step: Math.round(2 * IrisStyle.density)

    implicitWidth: root.cardWidth
    implicitHeight: grid.implicitHeight

    AgentAvailability {
        id: availability
    }

    UsageModel {
        id: usageModel
        tool: root.activeTool
        onCollected: availability.reloadAll()
    }

    // Everything on the page follows this one period.
    readonly property var activePeriod: usageModel.period(root.periodKey)
    readonly property var periodModels: root.activePeriod ? root.activePeriod.models : []
    // This month's spend from the day buckets (cost is per assistant turn).
    readonly property var monthly: {
        const key = Qt.formatDate(new Date(), "yyyy-MM")
        if (root.activeTool === "codex" || root.activeTool === "claude") {
            const estimate = usageModel.spending
            const available = estimate?.kind === "api-equivalent" && estimate?.month === key
                && estimate?.amount !== null && estimate?.amount !== undefined
                && Number.isFinite(Number(estimate.amount)) && Number(estimate.amount) >= 0
            return ({ current: available ? Number(estimate.amount) : 0,
                label: root.monthLabel(key), available: available,
                partial: estimate?.partial === true, stale: estimate?.stale_prices === true })
        }
        let total = 0
        for (const day of usageModel.days) {
            if (String(day?.date ?? "").slice(0, 7) === key)
                total += Number(day?.cost ?? 0)
        }
        return ({ current: total, label: root.monthLabel(key) })
    }
    readonly property string spendHelp: {
        if (root.activeTool === "opencode") return "This month's recorded usage cost."
        const estimate = usageModel.spending
        let text = "Estimated cost at published API rates.\nNot a bill or wallet balance."
        if (usageModel.plan.toLowerCase() === "plus")
            text += "\nYour Plus usage is covered by your plan."
        if (root.activeTool === "claude")
            text += "\nLocal Claude Code history only. Shared-account use on other devices is not included."
        if (root.monthly.partial) text += "\nSome usage lacks rates or complete pricing data."
        if (estimate?.price_date) text += "\nLatest saved prices: " + estimate.price_date + "."
        if (root.monthly.stale) text += "\nPrice refresh pending; saved rates are in use."
        if (!root.monthly.available) text += "\nNo priced usage is available yet."
        return text
    }
    readonly property bool hasCostHistory: usageModel.days.some(day => Number(day?.cost ?? 0) > 0)
    readonly property real periodCost: root.activePeriod
        ? Number(root.activePeriod.cost) : 0
    readonly property bool hasSpend: root.periodCost > 0.0005 || root.hasCostHistory
    // Claude and Codex are plan agents: Spending shares their Now card. OpenCode
    // keeps the standalone Spend card beside Now.
    readonly property bool planAgent: root.activeTool !== "opencode"
    readonly property bool integratedSpend: root.planAgent
    readonly property bool hasPlan: usageModel.hasPlan
    readonly property bool showLimits: root.hasPlan || root.activeTool === "claude"

    // Keep the stored agent pointing at a cache that exists. One write per
    // stale value; changing the tool re-arms it, so a later loss falls back
    // again. Nothing to fall back to leaves the config alone.
    function reconcileAgent(): void {
        if (!usageModel?.collectionReady) return
        if (!availability.ready)
            return
        if (!root.agentSwitchable)
            agentMenu.open = false
        if (root.toolAvailable || !root.hasAnyAgent || root._fallbackWritten)
            return
        root._fallbackWritten = true
        CustomWidgets.setConfigValue("ai-usage", "tool",
            String(root.agentOptions[0]?.value ?? "opencode"))
    }
    onToolChanged: root._fallbackWritten = false
    Connections {
        target: availability
        function onProbed() { Qt.callLater(root.reconcileAgent) }
    }

    // Layout 1 was the single long card, layout 2 the fixed 60/90/180 control.
    // Run once, then the stored values are the person's own.
    Component.onCompleted: {
        const version = Number(CustomWidgets.getConfigValue("ai-usage", "layoutVersion", 1))
        if (version < 2) {
            CustomWidgets.setConfigValue("ai-usage", "moduleWidth", 480)
            CustomWidgets.setConfigValue("ai-usage", "topModels", 3)
        }
        if (version < 3) {
            const stored = String(CustomWidgets.getConfigValue("ai-usage", "period", "30"))
            if (!["7", "30", "all"].includes(stored))
                CustomWidgets.setConfigValue("ai-usage", "period", "30")
            CustomWidgets.setConfigValue("ai-usage", "layoutVersion", 3)
        }
        root.reconcileAgent()
    }

    // ── trend buckets: hourly for Today / 24 hours, daily for the rest ──
    readonly property var trend: {
        if (root.hourly) {
            const allHours = usageModel.hours
            if (!allHours || allHours.length === 0)
                return ({ buckets: [], total: 0, peak: 1, span: 0, unit: "hours" })
            const hourBuckets = []
            let hourTotal = 0
            let hourPeak = 1
            if (root.periodKey === "today") {
                // The whole day as 24 slots, so the shape of the day is the
                // same at 02:00 and at 22:00; hours still to come stay empty.
                const midnight = new Date()
                midnight.setHours(0, 0, 0, 0)
                const dayKey = Qt.formatDate(midnight, "yyyy-MM-dd")
                const byHour = ({})
                for (const hour of allHours) {
                    const instant = new Date(Number(hour?.start ?? 0))
                    if (Qt.formatDate(instant, "yyyy-MM-dd") !== dayKey) continue
                    const clockHour = instant.getHours()
                    byHour[clockHour] = Number(byHour[clockHour] ?? 0) + root.historyWeight(hour)
                }
                for (let i = 0; i < 24; i++) {
                    const slot = new Date(midnight.getTime())
                    slot.setHours(i, 0, 0, 0)
                    const tokens = Number(byHour[i] ?? 0)
                    hourBuckets.push({ start: slot.getTime(), clockHour: i, tokens: tokens })
                    hourTotal += tokens
                    hourPeak = Math.max(hourPeak, tokens)
                }
                return ({ buckets: hourBuckets, total: hourTotal, peak: hourPeak,
                         span: 24, unit: "hours" })
            }
            const list = allHours.slice(Math.max(0, allHours.length - 24))
            for (const hour of list) {
                const tokens = root.historyWeight(hour)
                hourBuckets.push({ start: Number(hour?.start ?? 0), tokens: tokens })
                hourTotal += tokens
                hourPeak = Math.max(hourPeak, tokens)
            }
            return ({ buckets: hourBuckets, total: hourTotal, peak: hourPeak,
                     span: list.length, unit: "hours" })
        }
        const all = usageModel.days
        if (!all || all.length === 0)
            return ({ buckets: [], total: 0, peak: 1, span: 0, unit: "days" })
        // Fixed calendar slots ending today: a period shows its whole window
        // even when the cache starts a day or two ago, like the hourly view.
        const byDate = ({})
        for (const day of all) {
            const key = String(day?.date ?? "")
            if (isNaN(new Date(key + "T12:00:00").getTime())) continue
            byDate[key] = root.historyWeight(day)
        }
        const today = new Date()
        today.setHours(12, 0, 0, 0)
        // Periods end today. All time is always the same 45-day window: a
        // short cache draws dim empty days instead of two giant bars, and a
        // long one never turns the chart into hairlines.
        const count = root.periodDays > 0 ? root.periodDays : 45
        const list = []
        for (let step = count - 1; step >= 0; step--) {
            const slot = new Date(today.getTime())
            slot.setDate(slot.getDate() - step)
            const key = Qt.formatDate(slot, "yyyy-MM-dd")
            list.push({ date: key, tokens: Number(byDate[key] ?? 0) })
        }
        const buckets = []
        let total = 0
        let peak = 1
        for (const day of list) {
            const tokens = root.historyWeight(day)
            buckets.push({ date: String(day?.date ?? ""), tokens: tokens })
            total += tokens
            peak = Math.max(peak, tokens)
        }
        return ({ buckets: buckets, total: total, peak: peak, span: list.length, unit: "days" })
    }
    readonly property var trendBuckets: root.trend.buckets

    // Every chart weighs tokens plus cache reads, like the Activity card.
    function historyWeight(entry) {
        return Number(entry?.tokens ?? 0) + Number(entry?.cacheRead ?? 0)
    }

    function dayLabel(date: string): string {
        const parsed = new Date(String(date) + "T12:00:00")
        if (isNaN(parsed.getTime())) return ""
        return Qt.formatDate(parsed, "d MMM")
    }
    function bucketLabel(bucket: var): string {
        if (root.trend.unit === "hours") {
            if (Number.isInteger(bucket?.clockHour))
                return String(bucket.clockHour).padStart(2, "0") + ":00"
            const moment = new Date(Number(bucket?.start ?? 0))
            return isNaN(moment.getTime()) ? "" : Qt.formatTime(moment, "HH:mm")
        }
        return root.dayLabel(String(bucket?.date ?? ""))
    }
    function money(value: real): string {
        return "$" + Number(value || 0).toFixed(2)
    }
    function monthLabel(key: string): string {
        const parsed = new Date(String(key) + "-01T12:00:00")
        return isNaN(parsed.getTime()) ? String(key) : Qt.formatDate(parsed, "MMM")
    }
    function elapsedLabel(startMs: double): string {
        usageModel.tick   // a binding that calls this re-evaluates when the ticker moves
        const mins = Math.max(0, Math.floor((Date.now() - Number(startMs || 0)) / 60000))
        if (mins < 60) return mins + "m"
        const m = mins % 60
        return Math.floor(mins / 60) + "h " + (m < 10 ? "0" : "") + m + "m"
    }
    function modelDetail(model: var): string {
        if (!model) return ""
        const parts = ["I " + (model.inputLabel || "0"), "O " + (model.outputLabel || "0")]
        if (String(model.reasoningLabel ?? "0") !== "0") parts.push("R " + model.reasoningLabel)
        if (String(model.cacheReadLabel ?? "0") !== "0") parts.push("CR " + model.cacheReadLabel)
        if (String(model.cacheWriteLabel ?? "0") !== "0") parts.push("CW " + model.cacheWriteLabel)
        let text = parts.join("  ")
        if (model.cacheRate !== undefined && model.cacheRate !== null)
            text += " · " + Math.round(Number(model.cacheRate) * 100) + "% cached"
        return text
    }

    // The period menu sits above the grid so no card clips it.
    function openPeriodMenu(trigger: Item): void {
        agentMenu.open = false
        const point = trigger.mapToItem(root, 0, trigger.height)
        periodMenu.x = Math.max(0, Math.min(root.cardWidth - periodMenu.width,
            Math.round(point.x + (trigger.width - periodMenu.width) / 2)))
        periodMenu.y = Math.max(0, Math.min(Math.round(point.y + 4 * IrisStyle.density),
            Math.round(root.implicitHeight - periodMenu.height - 2)))
        periodMenu.open = true
    }

    // The agent list drops straight out of its chip, near the card's top.
    function openAgentMenu(): void {
        if (!root.agentSwitchable)
            return
        periodMenu.open = false
        const point = agentChip.mapToItem(root, 0, agentChip.height)
        agentMenu.x = Math.max(0, Math.min(root.cardWidth - agentMenu.width, Math.round(point.x)))
        agentMenu.y = Math.round(point.y + 4 * IrisStyle.density)
        agentMenu.open = true
    }

    Column {
        id: grid
        width: root.cardWidth
        spacing: root.gap

        // The agent list opens from this chip: one cache is read at a time,
        // and a pick writes the same `tool` key Settings writes. With a single
        // agent there is nothing to switch to, so the chip is just a label.
        Item {
            id: agentBar
            width: parent.width
            height: Math.round(28 * IrisStyle.density)

            Rectangle {
                id: agentChip
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                width: agentChipRow.implicitWidth + Math.round(20 * IrisStyle.density)
                height: Math.round(28 * IrisStyle.density)
                radius: height / 2
                color: agentMenu.open ? IrisStyle.fillHover : IrisStyle.fillQuiet
                border.width: 1
                border.color: ColorUtils.applyAlpha(IrisStyle.text, 0.10)

                Row {
                    id: agentChipRow
                    anchors.centerIn: parent
                    spacing: Math.round(5 * IrisStyle.density)
                    IrisText {
                        anchors.verticalCenter: parent.verticalCenter
                        text: root.toolLabel
                        font.pixelSize: Math.round(12 * IrisStyle.density)
                        font.weight: IrisStyle.weight(Font.DemiBold)
                    }
                    MaterialSymbol {
                        anchors.verticalCenter: parent.verticalCenter
                        visible: root.agentSwitchable
                        text: "expand_more"
                        iconSize: Math.round(16 * IrisStyle.density)
                        color: IrisStyle.muted
                        rotation: agentMenu.open ? 180 : 0
                        Behavior on rotation {
                            NumberAnimation {
                                duration: IrisStyle.duration(140)
                                easing.type: Easing.BezierSpline
                            }
                        }
                    }
                }

                MouseArea {
                    anchors.fill: parent
                    enabled: root.agentSwitchable
                    hoverEnabled: true
                    cursorShape: root.agentSwitchable ? Qt.PointingHandCursor : Qt.ArrowCursor
                    onClicked: agentMenu.open ? agentMenu.open = false : root.openAgentMenu()
                }
            }

            IrisText {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                role: IrisText.Eyebrow
                color: IrisStyle.muted
                text: "AI USAGE"
            }
        }

        Row {
            id: primaryRow
            width: parent.width
            spacing: root.gap
            visible: usageModel.has
            readonly property bool spendHere: !root.planAgent && root.hasSpend
            readonly property int count: (root.showLimits ? 1 : 0) + 1 + (spendHere ? 1 : 0)
            readonly property real half: Math.round((width - spacing) / 2)
            readonly property real rowHeight: Math.max(
                root.showLimits ? limitsCard.implicitHeight : 0,
                nowCard.implicitHeight,
                primaryRow.spendHere ? spendCard.implicitHeight : 0)

            AiUsageLimitsCard {
                page: root
                usage: usageModel
                id: limitsCard
                visible: root.showLimits
                width: primaryRow.count > 1 ? primaryRow.half : primaryRow.width
                height: primaryRow.rowHeight
            }
            AiUsageNowCard {
                page: root
                usage: usageModel
                id: nowCard
                width: primaryRow.count > 1 ? primaryRow.half : primaryRow.width
                height: primaryRow.rowHeight
            }
            AiUsageSpendCard {
                page: root
                usage: usageModel
                id: spendCard
                visible: primaryRow.spendHere
                width: primaryRow.count > 1 ? primaryRow.half : primaryRow.width
                height: primaryRow.rowHeight
            }
        }

        AiUsageTrendCard {
                page: root
                usage: usageModel
            width: parent.width
            visible: usageModel.has && root.trendBuckets.length > 0
            onPeriodMenuRequested: trigger => root.openPeriodMenu(trigger)
        }

        AiUsageModelsCard {
                page: root
                usage: usageModel
            width: parent.width
            visible: usageModel.has && root.periodModels.length > 0
        }

        AiUsageActivityCard {
                page: root
                usage: usageModel
            width: parent.width
            visible: usageModel.has && usageModel.days.length > 0
        }

        AiUsageEmptyCard {
                page: root
                usage: usageModel
            width: parent.width
            height: implicitHeight
            // Not while an agent switch is still loading: the old card holds the
            // frame, and a no-cache agent shows this only once the load failed.
            visible: !usageModel.has && !usageModel.switching
        }
    }

    // One dropdown implementation serves both pickers.
    MouseArea {
        anchors.fill: parent
        visible: agentMenu.open || periodMenu.open
        z: 88
        onClicked: {
            agentMenu.open = false
            periodMenu.open = false
        }
    }
    AiUsageMenu {
        id: agentMenu
        z: 92
        options: root.agentOptions
        currentValue: root.tool
        onPicked: value => CustomWidgets.setConfigValue("ai-usage", "tool", value)
    }
    AiUsageMenu {
        id: periodMenu
        z: 91
        menuWidth: 158
        rowHeight: Math.round(28 * IrisStyle.density)
        boldCurrent: false
        options: root.periodOptions
        currentValue: root.periodKey
        onPicked: value => CustomWidgets.setConfigValue("ai-usage", "period", value)
    }
}
