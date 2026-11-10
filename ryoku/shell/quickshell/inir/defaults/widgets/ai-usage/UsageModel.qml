pragma ComponentBehavior: Bound

// Reads one tool's private cache from the bundled collectors:
// ~/.cache/ai-usage/<tool>-usage.json. Mirrors the
// legacy parser of the shell's AiUsage service, kept local so a module failure
// stays local (the shell service is not imported across namespaces).
import QtQuick
import Quickshell
import Quickshell.Io
import "refresh" as Background

Item {
    id: root

    property string tool: "opencode"
    property bool refreshing: false
    property bool collectionReady: false
    // True while a tool switch is waiting for the new cache. The old card stays
    // on screen instead of flashing the empty state and its refresh button.
    property bool switching: false
    signal collected

    // -- parsed state --
    property bool has: false
    property bool fresh: false
    property int pct5h: 0
    property int percent7d: 0
    property double reset5hTs: 0
    property double reset7dTs: 0
    // Relative times recompute when this advances (see relative()).
    property int tick: 0
    property string plan: ""
    property string windowTokens: ""   // "_tokens_used / _window_limit"
    property string weekTokens: ""     // "_7d_tokens / _7d_limit"
    property double today: 0              // tokens today
    property var recent: []            // [{date, tokens}] fallback when the cache has no _days
    property var days: []              // [{date, tokens}] full history from the collector
    property var hours: []             // [{start, tokens}] the last 48 clock hours
    property var sessions: []          // [{id, title, directory, started, updated, tokens, cost, running}]
    property var periods: ({})         // "7"/"30"/"all" → {tokens, cacheRate, cost, sessions, models}
    property var windows: []           // [{label, pct, caption, resetTs}] server-reported quota windows
    property var spending: ({})        // Local API-equivalent estimate and coverage; not account charges.
    property var quota: ({})           // Claude account source, capture time and freshness.
    readonly property var runningSessions: root.sessions.filter(s => s && s.running)
    // A provider plan is what makes a 5h/7d window meaningful. The local
    // OpenCode collector says "local messages" and invents its budgets, so
    // those percentages would be fiction and the card stays away.
    readonly property bool hasPlan: {
        if (!root.has) return false
        const name = String(root.plan || "").trim().toLowerCase()
        if (name.length === 0)
            return root.windows.length > 0 || root.reset5hTs > 0 || root.reset7dTs > 0
        return name !== "local messages" && name !== "local" && name !== "unknown"
    }
    function period(key: string): var {
        return root.periods?.[key] ?? null
    }

    function normalizeWindows(data: var): var {
        const rows = []
        if (data._windows instanceof Array) {
            for (const window of data._windows) {
                const minutes = Number(window?.minutes) || 0
                if (minutes <= 0) continue
                const label = minutes % 1440 === 0 ? (minutes / 1440) + "d"
                    : minutes % 60 === 0 ? (minutes / 60) + "h" : minutes + "m"
                rows.push({
                    label: label,
                    caption: minutes === 300 ? root.windowTokens
                        : minutes === 10080 ? root.weekTokens : "",
                    pct: root.pct(String(Number(window?.utilization) || 0)),
                    resetTs: parseInt(window?.reset) || 0
                })
            }
        } else {
            // Older caches have only named windows. An explicit empty array
            // means the account reports no windows; never invent quotas then.
            if (String(data["5h-utilization"] ?? "").length > 0)
                rows.push({ label: "5h", caption: root.windowTokens,
                    pct: root.pct5h, resetTs: root.reset5hTs })
            if (String(data["7d-utilization"] ?? "").length > 0)
                rows.push({ label: "7d", caption: root.weekTokens,
                    pct: root.percent7d, resetTs: root.reset7dTs })
        }
        return rows
    }

    // Period rows come from the collector with a raw model name; give them the
    // same short label and numeric fields the rest of the card expects.
    function shortName(name: string): string {
        return String(name || "").replace(
            /\s*\((?:opencode|openrouter|openai|claude|codex|deepseek)\)$/i, "")
    }
    function normalizePeriods(source: var): var {
        const out = ({})
        for (const key in source) {
            const period = source[key] ?? ({})
            const rows = []
            for (const model of (period.models ?? [])) {
                rows.push(Object.assign({}, model, {
                    name: String(model.name || ""),
                    nameShort: root.shortName(model.name),
                    totalLabel: String(model.totalLabel || root.kmg(Number(model.total) || 0)),
                    cacheRate: Number(model.cacheRate) || 0,
                    pct: Math.max(0, Math.min(100, Number(model.pct) || 0))
                }))
            }
            out[key] = Object.assign({}, period, {
                tokens: Number(period.tokens) || 0,
                cacheRate: Number(period.cacheRate) || 0,
                cost: Number(period.cost) || 0,
                sessions: Number(period.sessions) || 0,
                models: rows
            })
        }
        return out
    }
    // Newest write any session made, in ms: what "Now" counts from.
    readonly property double lastActivityMs: {
        let latest = 0
        for (const session of root.sessions)
            latest = Math.max(latest, Number(session?.updated ?? 0))
        return latest
    }
    // "18 min. ago", and nothing at all before the first message.
    function relative(ms: double, translate: var): string {
        root.tick
        if (!(ms > 0)) return ""
        const tr = typeof translate === "function" ? translate : value => value
        const mins = Math.max(0, Math.round((Date.now() - ms) / 60000))
        if (mins < 2) return tr("just now")
        if (mins < 60) return tr("%1 min. ago").replace("%1", String(mins))
        const h = Math.floor(mins / 60)
        if (h < 24) return tr(h === 1 ? "%1 hr. ago" : "%1 hrs. ago").replace("%1", String(h))
        const d = Math.floor(h / 24)
        return tr(d === 1 ? "%1 day ago" : "%1 days ago").replace("%1", String(d))
    }

    function resetAt(ts: double, translate: var): string {
        root.tick
        if (!(ts > 0) || !Number.isFinite(ts)) return ""
        const deadline = new Date(ts * 1000)
        if (!Number.isFinite(deadline.getTime())) return ""
        const label = Qt.formatDateTime(deadline, "d MMM, h:mm AP t")
        const tr = typeof translate === "function" ? translate : value => value
        return tr(ts <= Date.now() / 1000 ? "reset was %1" : "resets %1").replace("%1", label)
    }

    Component.onCompleted: {
        root.collectionReady = Background.UsageRefresh.collectionReady
        cacheFile.reload()
        root.refresh()
    }
    onVisibleChanged: if (root.visible) {
        cacheFile.reload()
        root.refresh()
    }
    onToolChanged: {
        // Keep the current numbers visible while the other cache loads. A
        // switch whose cache is missing or unreadable ends in empty(), either
        // from onLoadFailed or from switchTimer, so nothing stays stale.
        root.switching = true
        switchTimer.restart()
        cacheFile.reload()
    }

    function empty() {
        root.has = false
        root.fresh = false
        root.pct5h = 0
        root.percent7d = 0
        root.reset5hTs = 0
        root.reset7dTs = 0
        root.plan = ""
        root.windowTokens = ""
        root.weekTokens = ""
        root.today = 0
        root.recent = []
        root.days = []
        root.hours = []
        root.sessions = []
        root.periods = ({})
        root.windows = []
        root.spending = ({})
        root.quota = ({})
    }

    function pct(value: string): int {
        return Math.max(0, Math.min(100, Math.round((parseFloat(value) || 0) * 100)))
    }

    function kmg(value: double): string {
        if (value >= 1000000) {
            const m = value / 1000000
            return m.toFixed(2).replace(/\.?0+$/, "") + "M"
        }
        if (value >= 1000) {
            const k = value / 1000
            return k >= 100 ? Math.round(k) + "K" : k.toFixed(1).replace(/\.0$/, "") + "K"
        }
        return String(value)
    }

    function refresh(): void {
        Background.UsageRefresh.refresh()
        root.refreshing = Background.UsageRefresh.refreshing
        staleTimer.restart()
    }

    function parse(text: string, modifiedAt: double): void {
        root.empty()
        try {
            const data = JSON.parse(String(text || "").trim())
            if (!data || typeof data !== "object" || Array.isArray(data) || Object.keys(data).length === 0) return
            root.has = true
            const age = Date.now() / 1000 - modifiedAt
            root.fresh = modifiedAt > 0 && age >= -120 && age < 900
                && data._source !== "stale" && data._stale !== true
            root.pct5h = root.pct(data["5h-utilization"])
            root.percent7d = root.pct(data["7d-utilization"])
            root.reset5hTs = parseInt(data["5h-reset"]) || 0
            root.reset7dTs = parseInt(data["7d-reset"]) || 0
            root.plan = String(data._plan || "")
            const used = Number(data._tokens_used) || 0
            const limit = Number(data._window_limit) || 0
            root.windowTokens = used > 0
                ? root.kmg(used) + " / " + root.kmg(limit)
                : ""
            const weekUsed = Number(data._7d_tokens) || 0
            const weekLimit = Number(data._7d_limit) || 0
            root.weekTokens = weekLimit > 0
                ? root.kmg(weekUsed) + " / " + root.kmg(weekLimit)
                : ""
            root.windows = root.normalizeWindows(data)
            const today = Number(data._today_tokens_with_cache_read ?? data._today_tokens ?? 0)
            root.today = Number.isFinite(today) && today >= 0 ? today : 0
            root.spending = data._spending && typeof data._spending === "object"
                ? data._spending : ({})
            root.quota = data._quota && typeof data._quota === "object"
                ? data._quota : ({})
            if (data._recent_days instanceof Array) {
                const days = data._recent_days
                root.recent = days.slice(Math.max(0, days.length - 7))
            }
            if (data._hours instanceof Array && data._hours.length > 0) {
                const hours = []
                for (const hour of data._hours) {
                    // Older Codex caches stamped hours in seconds; the widget
                    // builds slots in milliseconds, so normalise at the door.
                    const start = Number(hour?.start) || 0
                    hours.push({
                        start: start > 0 && start < 100000000000 ? start * 1000 : start,
                        tokens: Number(hour?.tokens) || 0,
                        cacheRead: Number(hour?.cacheRead) || 0
                    })
                }
                root.hours = hours
            }
            if (data._days instanceof Array && data._days.length > 0) {
                const history = []
                for (const day of data._days) {
                    history.push({
                        date: String(day?.date ?? ""),
                        tokens: Number(day?.tokens) || 0,
                        cacheRead: Number(day?.cacheRead) || 0,
                        cost: Number(day?.cost) || 0
                    })
                }
                root.days = history
            } else {
                // Older collectors carry only the seven-day window.
                root.days = root.recent.map(day => ({ date: String(day?.date ?? ""), tokens: Number(day?.tokens) || 0, cacheRead: 0, cost: 0 }))
            }
            const sessionRows = []
            for (const s of (data._sessions instanceof Array ? data._sessions : [])) {
                sessionRows.push({
                    id: String(s?.id ?? ""),
                    title: String(s?.title ?? ""),
                    directory: String(s?.directory ?? ""),
                    started: Number(s?.started) || 0,
                    updated: Number(s?.updated) || 0,
                    tokens: Number(s?.tokens) || 0,
                    cost: Number(s?.cost) || 0,
                    running: s?.running === true,
                    activityState: String(s?.activityState ?? ""),
                    activityEvidence: String(s?.activityEvidence ?? "")
                })
            }
            root.sessions = sessionRows
            root.periods = (data._periods && typeof data._periods === "object")
                ? root.normalizePeriods(data._periods) : ({})
        } catch (e) {
            console.warn("[ai-usage] parse failed:", e)
        } finally {
            // The load has landed, as data or as a genuine empty cache.
            switchTimer.stop()
            root.switching = false
        }
    }

    // Private widget caches cannot be replaced by an older system collector.
    readonly property string cacheDir: Quickshell.env("HOME") + "/.cache/ai-usage"
    readonly property string cachePath: root.cacheDir + "/" + root.tool + "-usage.json"

    property string _pendingText: ""
    property bool _missing: true

    FileView {
        id: cacheFile
        path: root.cachePath
        blockLoading: false
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: {
            root._missing = false
            root._pendingText = cacheFile.text()
            stat.running = false
            stat.running = true
        }
        onLoadFailed: {
            root._missing = true
            root._pendingText = ""
            switchTimer.stop()
            root.switching = false
            root.empty()
        }
    }

    Process {
        id: stat
        command: ["stat", "-c", "%Y", root.cachePath]
        stdout: StdioCollector { id: statOutput }
        onExited: {
            if (!root._missing)
                root.parse(root._pendingText, parseInt(statOutput.text.trim()) || 0)
        }
    }
    // A cache that appeared after first scan is a plain reload away.
    Timer {
        interval: 6000
        repeat: true
        running: root._missing && root.visible
        onTriggered: cacheFile.reload()
    }

    Connections {
        target: Background.UsageRefresh
        function onCollected(exitCode: int): void {
            if (exitCode === 0) root.collectionReady = true
            _pendingReloadTimer.restart()
        }
        function onRefreshingChanged(): void {
            root.refreshing = Background.UsageRefresh.refreshing
        }
    }
    Timer {
        id: _pendingReloadTimer
        interval: 600
        onTriggered: {
            // The collector has exited by now: release the refresh button even
            // when the run wrote nothing new, instead of waiting the full
            // stale timeout.
            root.refreshing = false
            cacheFile.reload()
            root.collected()
        }
    }
    Timer {
        id: staleTimer
        interval: 10000
        onTriggered: root.refreshing = false
    }
    // Last resort for a switch that never lands (an unreadable cache that is
    // neither loaded nor failed): drop the old numbers after a beat instead of
    // leaving them on the card for good.
    Timer {
        id: switchTimer
        interval: 1200
        onTriggered: {
            if (!root.switching)
                return
            root.switching = false
            root.empty()
        }
    }
    // Relative times ("18 min. ago", a session's elapsed) have to move while
    // the card sits on the page; one ticker keeps them honest.
    Timer {
        interval: 30000
        repeat: true
        triggeredOnStart: true
        running: root.has && root.visible
        onTriggered: root.tick++
    }
    Connections {
        target: cacheFile
        function onFileChanged() {
            root.refreshing = false
        }
    }
}
