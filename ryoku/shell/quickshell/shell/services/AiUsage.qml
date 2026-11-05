pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons

Singleton {
    id: root

    // The collectors write under ~/.cache whatever XDG_CACHE_HOME says, so the
    // reader follows them rather than the XDG variable.
    readonly property string cacheDir: Quickshell.env("HOME") + "/.cache"
    readonly property var toolIds: ["claude", "codex", "opencode"]
    readonly property var labels: ({
        claude: "Claude",
        codex: "Codex",
        opencode: "OpenCode"
    })

    property var tools: ({})
    property bool refreshing: false
    // Seconds since the epoch, advanced by `clock` so pace readings follow time.
    property real nowSec: Date.now() / 1000
    signal reloadRequested(var selected)

    readonly property var present: root.toolIds.filter(id => root.tool(id).has)
    readonly property bool available: root.present.length > 0

    function emptyTool(id) {
        return {
            id: id,
            has: false,
            fresh: false,
            has5h: false,
            has7d: false,
            pct5h: 0,
            pct7d: 0,
            reset5h: 0,
            reset7d: 0,
            blocked: false,
            status: "",
            plan: "",
            tokens: "",
            rate: "",
            today: 0,
            recent: [],
            model: "",
            models: [],
            buckets: [],
            windows: [],
            primaryPct: 0,
            primaryLabel: "",
            primaryReset: 0,
            quotaPct: 0,
            quotaLabel: "",
            limitReachedType: ""
        };
    }

    function tool(id) {
        return root.tools[id] || root.emptyTool(id);
    }

    function percent(value) {
        return Math.max(0, Math.min(100,
            Math.round((parseFloat(value) || 0) * 100)));
    }

    function windowLabel(minutes) {
        if (minutes === 300)
            return "5h";
        if (minutes === 10080)
            return I18n.tr("Weekly");
        if (minutes > 0 && minutes % 1440 === 0)
            return (minutes / 1440) + "d";
        if (minutes > 0 && minutes % 60 === 0)
            return (minutes / 60) + "h";
        return minutes > 0 ? minutes + "m" : I18n.tr("window");
    }

    function windowFromCache(value) {
        const window = value || {};
        const minutes = parseInt(window.minutes) || 0;
        return {
            kind: String(window.kind || ""),
            minutes: minutes,
            label: String(window.label || root.windowLabel(minutes)),
            pct: root.percent(window.utilization),
            resetTs: parseInt(window.reset) || 0
        };
    }

    function windowsFromCache(values) {
        if (!values || values.length === undefined)
            return [];
        const windows = [];
        for (let i = 0; i < values.length; ++i)
            windows.push(root.windowFromCache(values[i]));
        return windows;
    }

    function legacyWindows(data) {
        const windows = [];
        const has5h = data["5h-utilization"] !== undefined
            && String(data["5h-utilization"]) !== "";
        const has7d = data["7d-utilization"] !== undefined
            && String(data["7d-utilization"]) !== "";
        if (has5h || (parseInt(data["5h-reset"]) || 0) > 0) {
            windows.push({
                kind: "primary",
                minutes: 300,
                label: "5h",
                pct: root.percent(data["5h-utilization"]),
                resetTs: parseInt(data["5h-reset"]) || 0
            });
        }
        if (has7d || (parseInt(data["7d-reset"]) || 0) > 0) {
            windows.push({
                kind: "secondary",
                minutes: 10080,
                label: root.windowLabel(10080),
                pct: root.percent(data["7d-utilization"]),
                resetTs: parseInt(data["7d-reset"]) || 0
            });
        }
        return windows;
    }

    function codexBuckets(data) {
        const buckets = [];
        if (parseInt(data.schemaVersion) === 3
                && data.buckets && data.buckets.length !== undefined) {
            for (let i = 0; i < data.buckets.length; ++i) {
                const source = data.buckets[i] || {};
                const windows = root.windowsFromCache(source.windows);
                if (windows.length === 0)
                    continue;
                buckets.push({
                    id: String(source.id || ""),
                    label: String(source.label || source.id || "Codex"),
                    isGeneral: source.isGeneral === true,
                    windows: windows,
                    plan: String(source.plan || ""),
                    rateLimitReachedType: String(source.rateLimitReachedType || "")
                });
            }
        } else if (parseInt(data.schemaVersion) === 2
                && data.windows && data.windows.length !== undefined) {
            const windows = root.windowsFromCache(data.windows);
            if (windows.length > 0) {
                buckets.push({
                    id: "codex",
                    label: "Codex",
                    isGeneral: true,
                    windows: windows,
                    plan: String(data._plan || ""),
                    rateLimitReachedType: ""
                });
            }
        } else {
            const windows = root.legacyWindows(data);
            if (windows.length > 0) {
                buckets.push({
                    id: "codex",
                    label: "Codex",
                    isGeneral: true,
                    windows: windows,
                    plan: String(data._plan || ""),
                    rateLimitReachedType: ""
                });
            }
        }
        return buckets;
    }

    function generalBucket(buckets) {
        for (let i = 0; i < buckets.length; ++i) {
            if (buckets[i].isGeneral === true || buckets[i].id === "codex")
                return buckets[i];
        }
        return { label: "Codex", plan: "", windows: [], rateLimitReachedType: "" };
    }

    function usageText(data) {
        const used = Number(data._tokens_used) || 0;
        const limit = Number(data._window_limit) || 0;
        return used > 0
            ? (used / 1000000).toFixed(2) + "M / " + (limit / 1000000).toFixed(1) + "M"
            : "";
    }

    function rateText(data) {
        const hourly = Math.round((Number(data._rate_per_hour) || 0) / 1000);
        return hourly > 0 ? hourly + "k tok/h" : "";
    }

    function parseLegacy(id, data, fresh) {
        const result = root.emptyTool(id);
        result.has = true;
        result.fresh = fresh;
        result.has5h = data["5h-utilization"] !== undefined
            && String(data["5h-utilization"]) !== "";
        result.has7d = data["7d-utilization"] !== undefined
            && String(data["7d-utilization"]) !== "";
        result.pct5h = root.percent(data["5h-utilization"]);
        result.pct7d = root.percent(data["7d-utilization"]);
        result.reset5h = parseInt(data["5h-reset"]) || 0;
        result.reset7d = parseInt(data["7d-reset"]) || 0;
        result.status = String(data.status || "");
        result.blocked = result.status === "rejected" || result.status === "blocked";
        result.plan = String(data._plan || "");
        result.tokens = root.usageText(data);
        result.rate = root.rateText(data);
        result.today = parseInt(data._today_tokens) || 0;
        result.recent = data._recent_days instanceof Array ? data._recent_days : [];
        result.model = String(data._model || "");
        result.models = data._models instanceof Array ? data._models : [];
        return result;
    }

    function parseCodex(data, fresh) {
        const result = root.emptyTool("codex");
        const buckets = root.codexBuckets(data);
        const general = root.generalBucket(buckets);
        const windows = general.windows || [];
        result.has = windows.length > 0;
        result.fresh = fresh;
        result.buckets = buckets;
        result.windows = windows;
        result.plan = String(data._plan || general.plan || "");
        result.status = String(data.status || "");
        result.limitReachedType = String(data._limit_reached_type
            || general.rateLimitReachedType || "");
        result.blocked = result.status === "rejected" || result.status === "blocked"
            || result.limitReachedType.length > 0;
        result.rate = root.rateText(data);
        result.today = parseInt(data._today_tokens) || 0;
        result.recent = data._recent_days instanceof Array ? data._recent_days : [];
        for (let i = 0; i < windows.length; ++i) {
            const window = windows[i];
            if (i === 0) {
                result.primaryPct = window.pct;
                result.primaryLabel = window.label;
                result.primaryReset = window.resetTs;
            }
            if (window.minutes === 300) {
                result.has5h = true;
                result.pct5h = window.pct;
                result.reset5h = window.resetTs;
            } else if (window.minutes === 10080) {
                result.has7d = true;
                result.pct7d = window.pct;
                result.reset7d = window.resetTs;
            }
            if (window.pct > result.quotaPct) {
                result.quotaPct = window.pct;
                result.quotaLabel = String(general.label || "Codex")
                    + " " + String(window.label || root.windowLabel(window.minutes));
            }
        }
        return result;
    }

    function parse(id, text, modifiedAt) {
        try {
            const data = JSON.parse(String(text || "").trim());
            const fresh = modifiedAt > 0
                && (Date.now() / 1000 - modifiedAt) < 900
                && data._source !== "stale";
            return id === "codex"
                ? root.parseCodex(data, fresh)
                : root.parseLegacy(id, data, fresh);
        } catch (error) {
            return root.emptyTool(id);
        }
    }

    function store(id, text, modifiedAt) {
        const next = Object.assign({}, root.tools);
        const value = root.parse(id, text, modifiedAt);
        if (JSON.stringify(next[id] || {}) === JSON.stringify(value))
            return;
        next[id] = value;
        root.tools = next;
    }

    function reload(ids) {
        const selected = ids && ids.length !== undefined
            ? Array.from(ids) : root.toolIds;
        root.reloadRequested(selected);
    }

    // Where a quota window stands against an even burn rate. `expected` is the
    // share of the window already elapsed, `delta` is used minus expected in
    // points (positive: ahead of pace), `exhaustTs` is when the window fills at
    // the current rate, or 0 when it will not fill before it resets. Needs a
    // live reset time; a window that has not started or whose reset has passed
    // reports `available: false`. The first 5% of a window is too short to
    // extrapolate from, so there `exhaustTs` stays 0.
    function pace(id, weekly) {
        const data = root.tool(id);
        const seconds = weekly ? 604800 : 18000;
        const used = weekly ? data.pct7d : data.pct5h;
        const reset = weekly ? data.reset7d : data.reset5h;
        const none = { available: false, used: used, expected: 0, delta: 0,
            exhaustTs: 0 };
        if (!(weekly ? data.has7d : data.has5h) || !(reset > 0))
            return none;
        const left = reset - root.nowSec;
        if (left <= 0 || left > seconds + 60)
            return none;
        const elapsed = 1 - Math.min(left, seconds) / seconds;
        const expected = elapsed * 100;
        let exhaustTs = 0;
        if (elapsed >= 0.05 && used > 0 && used < 100) {
            const full = root.nowSec + (100 - used) / used * elapsed * seconds;
            exhaustTs = full < reset ? full : 0;
        }
        return { available: true, used: used, expected: Math.round(expected),
            delta: Math.round(used - expected), exhaustTs: exhaustTs };
    }

    function refresh() {
        if (root.refreshing)
            return;
        root.refreshing = true;
        refreshTimeout.restart();
        collector.running = false;
        collector.running = true;
    }

    Instantiator {
        id: readers
        model: root.toolIds

        delegate: Item {
            id: reader

            required property string modelData
            readonly property string toolId: modelData
            property string pendingText: ""
            property bool missing: true

            function reload() {
                cacheFile.reload();
            }

            width: 0
            height: 0
            visible: false

            FileView {
                id: cacheFile
                path: root.cacheDir + "/" + reader.toolId + "-usage.json"
                blockLoading: false
                watchChanges: true
                printErrors: false
                onFileChanged: reload()
                onLoaded: {
                    reader.missing = false;
                    reader.pendingText = text();
                    metadata.running = false;
                    metadata.running = true;
                }
                onLoadFailed: {
                    reader.missing = true;
                    metadata.running = false;
                    root.store(reader.toolId, "", 0);
                }
            }
            Connections {
                target: root
                function onReloadRequested(selected) {
                    if (selected.indexOf(reader.toolId) >= 0)
                        reader.reload();
                }
            }

            Process {
                id: metadata
                command: ["stat", "-c", "%Y", cacheFile.path]
                stdout: StdioCollector { id: metadataOutput }
                onExited: {
                    if (!reader.missing) {
                        root.store(reader.toolId, reader.pendingText,
                            parseInt(metadataOutput.text.trim()) || 0);
                    }
                }
            }

            Timer {
                interval: 5000
                repeat: true
                running: reader.missing
                onTriggered: reader.reload()
            }
        }
    }

    Process {
        id: collector
        command: ["ryoku-host", "svc", "--user", "start", "ryoku-ai-usage.service"]
        onExited: {
            refreshTimeout.stop();
            root.reload();
            root.refreshing = false;
        }
    }

    Timer {
        id: clock
        interval: 30000
        repeat: true
        running: root.available
        onTriggered: root.nowSec = Date.now() / 1000
    }

    Timer {
        id: refreshTimeout
        interval: 30000
        onTriggered: root.refreshing = false
    }
}
