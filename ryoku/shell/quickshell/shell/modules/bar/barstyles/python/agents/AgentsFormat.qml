pragma Singleton
import QtQuick
import "../"
import shell.services as Ryoku

// Presentation helpers shared by the bar pill and the popup. The usage data
// itself is parsed once, by the shell's AiUsage service.
QtObject {
    id: root

    // The marks are the QS Bar's assets; only Claude keeps its brand colour.
    readonly property var marks: ({
        claude: { file: "claude.svg", tint: false },
        codex: { file: "codex.svg", tint: true },
        opencode: { file: "opencode-mark.svg", tint: true }
    })

    function markSource(id) {
        const mark = marks[id];
        return mark ? Qt.resolvedUrl("../../qsbar/assets/" + mark.file) : "";
    }

    function markTinted(id) {
        return !!marks[id] && marks[id].tint;
    }

    function agentName(id) {
        return Ryoku.AiUsage.labels[id] || id;
    }

    // A key this install's catalog lacks falls back to the English text.
    function tr(key, fallback, args) {
        let text = I18n.t("agents." + key, args);
        if (!text || text === "agents." + key) {
            text = fallback;
            for (let k in (args || {})) text = text.replace("{" + k + "}", args[k]);
        }
        return text;
    }

    function levelColor(pct, blocked) {
        if (blocked || pct >= 90) return ThemeBackend.red;
        if (pct >= 70) return ThemeBackend.yellow;
        return ThemeBackend.green;
    }

    function countdown(ts, nowSec) {
        const left = Math.round(ts - nowSec);
        if (!(ts > 0) || left <= 0) return "";
        const d = Math.floor(left / 86400);
        const h = Math.floor(left % 86400 / 3600);
        const m = Math.floor(left % 3600 / 60);
        if (d > 0) return d + "d " + h + "h";
        if (h > 0) return h + "h " + m + "m";
        return m + "m";
    }

    function clock(ts, nowSec) {
        if (!(ts > 0)) return "";
        const when = new Date(ts * 1000);
        const sameDay = when.toDateString() === new Date(nowSec * 1000).toDateString();
        return Qt.formatDateTime(when, sameDay ? "HH:mm" : "ddd HH:mm");
    }

    function tokens(n) {
        n = Number(n) || 0;
        if (n >= 1e9) return (n / 1e9).toFixed(1) + "B";
        if (n >= 1e6) return (n / 1e6).toFixed(1) + "M";
        if (n >= 1e3) return Math.round(n / 1e3) + "k";
        return String(Math.round(n));
    }

    // AiUsage hands the hourly rate over as "<thousands>k tok/h".
    function rate(text) {
        const k = parseFloat(text);
        return k > 0 ? tr("rate", "{n} tokens/h", { n: tokens(k * 1000) }) : "";
    }

    function plan(value) {
        const p = String(value || "");
        if (p.toLowerCase() === "prolite") return "Pro Lite";
        return p.length ? p.charAt(0).toUpperCase() + p.slice(1) : "";
    }

    function windowLabel(w) {
        const minutes = Number(w.minutes) || 0;
        if (minutes === 300) return tr("window5h", "5-hour");
        if (minutes === 10080) return tr("window7d", "Weekly");
        if (minutes > 0 && minutes % 1440 === 0) return tr("windowDays", "{n}-day", { n: minutes / 1440 });
        if (minutes > 0 && minutes % 60 === 0) return tr("windowHours", "{n}-hour", { n: minutes / 60 });
        return String(w.label || "");
    }

    // Codex reports buckets of windows; the other collectors report the
    // 5-hour and weekly pair. Both become a list of labelled window groups.
    function groups(tool) {
        const buckets = tool.buckets || [];
        if (buckets.length) {
            return buckets.map(b => ({
                label: buckets.length > 1 ? String(b.label || "") : "",
                general: b.isGeneral === true || b.id === "codex",
                blocked: String(b.rateLimitReachedType || "") !== "",
                windows: b.windows || []
            }));
        }
        let windows = [];
        if (tool.has5h) windows.push({ minutes: 300, pct: tool.pct5h, resetTs: tool.reset5h });
        if (tool.has7d) windows.push({ minutes: 10080, pct: tool.pct7d, resetTs: tool.reset7d });
        return windows.length ? [{ label: "", general: true, blocked: false, windows: windows }] : [];
    }

    // The bar shows the general group's percentages, at most two of them.
    function headline(tool) {
        const all = groups(tool);
        let general = all.find(g => g.general) || all[0];
        return general ? general.windows.slice(0, 2).map(w => w.pct) : [];
    }

    function pace(id, w, general, nowSec) {
        if (!general || typeof Ryoku.AiUsage.pace !== "function") return null;
        if (w.minutes !== 300 && w.minutes !== 10080) return null;
        const p = Ryoku.AiUsage.pace(id, w.minutes === 10080, nowSec);
        return p && p.available ? p : null;
    }

    // Monday-first calendar week (ISO 8601), filled from the collectors'
    // last-seven-days history, which always covers Monday through today.
    function week(recent, nowSec) {
        let byDate = {};
        for (let i = 0; i < (recent || []).length; i++)
            byDate[String(recent[i].date)] = Number(recent[i].tokens) || 0;
        const today = new Date(nowSec * 1000);
        today.setHours(12, 0, 0, 0);
        const monday = new Date(today);
        monday.setDate(today.getDate() - (today.getDay() + 6) % 7);
        let days = [];
        for (let i = 0; i < 7; i++) {
            const d = new Date(monday);
            d.setDate(monday.getDate() + i);
            days.push({
                label: Qt.locale().dayName(d.getDay(), Locale.NarrowFormat),
                tokens: byDate[Qt.formatDate(d, "yyyy-MM-dd")] || 0,
                isToday: d.getTime() === today.getTime(),
                future: d.getTime() > today.getTime()
            });
        }
        return days;
    }
}
