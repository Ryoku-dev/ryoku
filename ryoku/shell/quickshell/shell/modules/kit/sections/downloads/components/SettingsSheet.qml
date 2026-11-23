pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Lithic
import Ryoku.Ui.Singletons
import "../../../../ask/bar"
import "../../../../ask/look"
import "../../../parts"
import "../../../../ask/lib/downloads.js" as Logic
import "../../../../ask/lib/format.js" as Format

// Download settings, inside the view: the daemon's Settings as one list. Left
// and Right step a value, Enter flips a switch or starts typing one in the
// bar's field; every change is a `settings.set` patch the view sends.
Item {
    id: root

    required property real s
    property var settings: ({})
    // what is typed: a filter, or the value being edited
    property string query: ""
    // the row whose value the field is editing
    property string editing: ""
    property real maximumHeight: 480 * s

    readonly property var current: list.current
    readonly property string home: String(Quickshell.env("HOME") || "")

    signal patched(var patch)
    signal editRequested(string id, string text)
    signal refused(string text)

    implicitHeight: list.count > 0 ? list.implicitHeight : blank.implicitHeight

    readonly property var qualities: ["360", "480", "720", "1080", "1440", "2160", "best"]
    readonly property var audios: ["best", "m4a", "mp3", "opus"]
    readonly property var actives: [1, 2, 3, 4, 5, 6, 8, 10]
    readonly property var conns: [1, 2, 4, 6, 8, 12, 16]
    readonly property var ratios: [0.5, 1, 1.5, 2, 3, 5]
    readonly property var minutes: [5, 15, 30, 60, 120, 360, 1440]
    // how long a finished, failed or stopped download stays listed, 0 for ever
    readonly property var keeps: [1, 7, 30, 0]

    function tilde(path) {
        const value = String(path || "");
        return root.home.length > 0 && value.indexOf(root.home) === 0 ? "~" + value.slice(root.home.length) : value;
    }
    function limitText(value) {
        return Number(value) > 0 ? Format.speed(Number(value) * 1024) : I18n.tr("Off");
    }
    function minutesText(value) {
        const n = Number(value) || 0;
        return n >= 1440 ? I18n.tr("%1 d").arg(Math.round(n / 1440)) : n >= 60 ? I18n.tr("%1 h").arg(Math.round(n / 60 * 10) / 10) : I18n.tr("%1 min").arg(n);
    }
    function qualityText(value) {
        return value === "best" ? I18n.tr("Best there is") : value + "p";
    }
    function audioText(value) {
        return value === "best" ? I18n.tr("As it comes") : String(value).toUpperCase();
    }
    // a day count the sheet does not offer lands on the nearest one it does
    function keepOf(value) {
        const n = value === undefined || value === null ? 7 : Number(value) || 0;
        return n <= 0 ? 0 : n < 4 ? 1 : n < 19 ? 7 : 30;
    }
    function keepText(value) {
        return value === 1 ? I18n.tr("A day") : value === 7 ? I18n.tr("A week") : value === 30 ? I18n.tr("A month") : I18n.tr("Never");
    }

    function stepRow(id, title, list, value, text, detail) {
        const at = list.indexOf(value);
        return { id: id, size: "one", kind: "step", title: title, list: list, raw: value, value: text, detail: detail || "",
            first: at === 0, last: at === list.length - 1 };
    }
    function toggleRow(id, title, on, detail) {
        return { id: id, size: "one", kind: "toggle", title: title, on: on === true, detail: detail || "" };
    }
    function textRow(id, title, value, detail) {
        const live = root.editing === id;
        return { id: id, size: "one", kind: "text", title: title, value: live ? I18n.tr("Enter sets it, Esc keeps it") : value, detail: detail || "", editing: live };
    }

    readonly property var groups: {
        const v = root.settings || ({});
        const seed = v.seed === true;
        return [
            { id: "general", title: I18n.tr("General"), rows: [
                root.textRow("dir", I18n.tr("Folder"), root.tilde(v.dir), ""),
                root.toggleRow("sortByType", I18n.tr("Sort new files into folders by type"), v.sortByType, I18n.tr("Videos, Music, Pictures and so on, inside the folder")),
                root.stepRow("maxActive", I18n.tr("Downloads at once"), root.actives, Number(v.maxActive), String(v.maxActive || ""), ""),
                root.stepRow("connections", I18n.tr("Connections per download"), root.conns, Number(v.connections), String(v.connections || ""), ""),
                root.stepRow("downLimit", I18n.tr("Download limit"), Logic.LIMITS, Number(v.downLimit), root.limitText(v.downLimit), ""),
                root.stepRow("upLimit", I18n.tr("Upload limit"), Logic.LIMITS, Number(v.upLimit), root.limitText(v.upLimit), "")
            ] },
            { id: "media", title: I18n.tr("Video and audio"), rows: [
                root.stepRow("videoQuality", I18n.tr("Video quality"), root.qualities, String(v.videoQuality), root.qualityText(String(v.videoQuality || "best")), I18n.tr("the most a video is fetched at")),
                root.stepRow("audioFormat", I18n.tr("Audio format"), root.audios, String(v.audioFormat), root.audioText(String(v.audioFormat || "best")), I18n.tr("for audio-only downloads")),
                root.toggleRow("ytdlpFallback", I18n.tr("Fall back to yt-dlp"), v.ytdlpFallback, I18n.tr("for sites the engine cannot read itself")),
                root.toggleRow("coverArt", I18n.tr("Add cover art to music"), v.coverArt !== false, I18n.tr("the picture the site shows, inside the audio file"))
            ] },
            { id: "torrents", title: I18n.tr("Torrents"), rows: [
                root.toggleRow("seed", I18n.tr("Seed when a torrent finishes"), v.seed, "")
            ].concat(seed ? [
                root.stepRow("seedRatio", I18n.tr("Stop at ratio"), root.ratios, Number(v.seedRatio), String(v.seedRatio || ""), ""),
                root.stepRow("seedMinutes", I18n.tr("Stop after"), root.minutes, Number(v.seedMinutes), root.minutesText(v.seedMinutes), I18n.tr("whichever comes first"))
            ] : []) },
            { id: "behaviour", title: I18n.tr("Behaviour"), rows: [
                root.toggleRow("notify", I18n.tr("Notify when a download finishes"), v.notify, ""),
                root.toggleRow("resumeOnStart", I18n.tr("Resume unfinished downloads"), v.resumeOnStart, I18n.tr("when the engine starts")),
                root.stepRow("keepDays", I18n.tr("Forget finished downloads after"), root.keeps, root.keepOf(v.keepDays), root.keepText(root.keepOf(v.keepDays)), I18n.tr("failed and stopped ones too; finished files stay on disk"))
            ] },
            { id: "advanced", title: I18n.tr("Advanced"), rows: [
                root.textRow("port", I18n.tr("Torrent port"), Number(v.port) > 0 ? String(v.port) : I18n.tr("Random"), I18n.tr("0 picks a free one")),
                root.toggleRow("dht", I18n.tr("Find peers without a tracker"), v.dht, I18n.tr("DHT"))
            ] }
        ];
    }
    readonly property var sections: {
        const text = root.editing.length > 0 ? "" : root.query.trim();
        const out = [];
        for (var i = 0; i < root.groups.length; i++) {
            const group = root.groups[i];
            const rows = group.rows.filter(row => Logic.matches({ title: row.title + " " + row.detail + " " + group.title }, text));
            if (rows.length > 0)
                out.push({ id: group.id, title: group.title, note: "", rows: rows });
        }
        return out;
    }

    function move(delta) { if (root.editing.length === 0) list.move(delta); }
    function jumpSection(delta) { if (root.editing.length === 0) list.jumpSection(delta); }
    function page(delta) { if (root.editing.length === 0) list.page(delta); }

    function set(id, value) {
        const patch = {};
        patch[id] = value;
        root.patched(patch);
    }

    // step moves the selected value one place; a switch takes a side
    function step(delta) {
        const row = root.current;
        if (!row || root.editing.length > 0)
            return;
        if (row.kind === "toggle")
            root.set(row.id, delta > 0);
        else if (row.kind === "step")
            root.set(row.id, Logic.step(row.list, row.raw, delta));
    }

    // Enter: flip, step on (round to the start), or type the value
    function activate() {
        const row = root.current;
        if (!row)
            return;
        if (root.editing.length > 0) {
            root.commit();
        } else if (row.kind === "toggle") {
            root.set(row.id, !row.on);
        } else if (row.kind === "step") {
            const at = row.list.indexOf(row.raw);
            root.set(row.id, row.list[(at + 1) % row.list.length]);
        } else if (row.id === "dir") {
            root.editRequested("dir", root.tilde(root.settings.dir));
        } else if (row.id === "port") {
            root.editRequested("port", String(Number(root.settings.port) || 0));
        }
    }

    function commit() {
        const text = root.query.trim();
        if (root.editing === "port") {
            const port = Logic.number(text, 0, 65535, true);
            if (port === null || (port > 0 && port < 1024)) {
                root.refused(I18n.tr("A port is 0, or 1024 to 65535"));
                return;
            }
            root.set("port", port);
        } else if (root.editing === "dir") {
            var path = text;
            if (path === "~" || path.indexOf("~/") === 0)
                path = root.home + path.slice(1);
            if (path.charAt(0) !== "/") {
                root.refused(I18n.tr("Type the folder's whole path"));
                return;
            }
            root.set("dir", path);
        }
        root.editRequested("", "");
    }

    ResultList {
        id: list
        width: parent.width
        height: implicitHeight
        visible: count > 0
        s: root.s
        sections: root.sections
        maximumHeight: root.maximumHeight
        rowDelegate: OptionRow {}
        onActivated: root.activate()
    }

    Item {
        id: blank
        width: parent.width
        visible: list.count === 0
        implicitHeight: Math.round(64 * root.s)
        height: implicitHeight

        Line {
            x: Math.round(18 * root.s)
            y: Math.round(14 * root.s)
            width: parent.width - x * 2
            s: root.s
            size: 14
            text: I18n.tr("No setting matches \u201c%1\u201d").arg(root.query.trim())
        }
    }
}
