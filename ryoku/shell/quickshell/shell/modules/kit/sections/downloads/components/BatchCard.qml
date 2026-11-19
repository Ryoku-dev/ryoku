pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../../../ask/bar"
import "../../../../ask/look"
import "../../../parts"
import "../../../../ask/lib/downloads.js" as Logic
import "../../../../ask/lib/format.js" as Format
import "../../../../ask/lib/root.js" as RootLogic

// Several links at once. The engine cleans the paste (`links`), then each link
// is read four at a time and its row fills in. One format is chosen for all and
// lands on each link's nearest stream; any row can take its own. Enter adds the
// ticked rows in one `addMany`. The view feeds it keys and shows its actions.
Item {
    id: root

    required property real s
    property var downloads: null
    // the paste, or the root view's cleaned links one per line
    property string text: ""
    property bool live: false
    property var settings: ({})
    // this batch's own folder; empty keeps the settings' one
    property string folder: ""
    property string home: ""
    property real maximumHeight: 480 * s

    // [{raw, url, host, kind, notes, state, resolved, error, ticked, own, only}]
    property var rows: []
    property var tally: null
    property var failure: null
    property bool parsed: false
    property bool adding: false
    property int all: 0
    property int generation: 0

    readonly property string wanted: Logic.BATCH[root.all]
    readonly property int limit: parseInt(String(root.settings.videoQuality || ""), 10) || 0
    readonly property int count: root.rows.length
    readonly property int ticked: root.rows.filter(row => row.ticked && row.state !== "failed").length
    readonly property int unread: root.rows.filter(row => row.state === "waiting" || row.state === "resolving").length
    readonly property int at: list.current ? Number(list.current.id) : -1
    readonly property var row: root.at >= 0 && root.at < root.rows.length ? root.rows[root.at] : null
    readonly property var rowChoices: root.row ? root.choicesOf(root.row) : []
    readonly property bool rowListed: root.row !== null && root.listed(root.row)
    readonly property string dir: root.folder.length > 0 ? root.folder : String(root.settings.dir || "")

    readonly property var allNames: [I18n.tr("Best"), "1080p", "720p", I18n.tr("Audio")]
    readonly property string summary: {
        const t = root.tally;
        if (!t)
            return "";
        const parts = [t.count === 1 ? I18n.tr("1 link") : I18n.tr("%1 links").arg(t.count)];
        if (t.cleaned > 0)
            parts.push(I18n.tr("%1 cleaned").arg(t.cleaned));
        if (t.duplicates > 0)
            parts.push(t.duplicates === 1 ? I18n.tr("1 duplicate dropped") : I18n.tr("%1 duplicates dropped").arg(t.duplicates));
        return parts.join(", ");
    }
    readonly property string progress: root.unread > 0 ? I18n.tr("%1 of %2 read").arg(root.count - root.unread).arg(root.count)
        : root.count > 0 ? I18n.tr("%1 of %2 ticked").arg(root.ticked).arg(root.count) : ""

    signal added(var tasks, bool paused)
    signal hud(string text)
    signal alert(string text)

    implicitHeight: head.height + (list.count > 0 ? list.implicitHeight : blank.implicitHeight) + strip.height

    function tilde(path) {
        const value = String(path || "");
        return root.home.length > 0 && value.indexOf(root.home) === 0 ? "~" + value.slice(root.home.length) : value;
    }
    function join(parts) {
        return parts.filter(part => String(part || "").length > 0).join(" \u00b7 ");
    }
    function choicesOf(row) {
        return row.state === "ready" ? Logic.batchChoices(row.resolved, root.limit) : [];
    }
    function pickOf(row, choices) {
        return row.own >= 0 && row.own < choices.length ? row.own : Logic.batchPick(choices, root.wanted);
    }
    function listed(row) {
        return row.state === "ready" && ((row.resolved.items || []).length > 1);
    }
    function noteWord(note) {
        return note === "https-added" ? I18n.tr("https added")
            : note === "trimmed" ? I18n.tr("trimmed")
            : note === "unwrapped" ? I18n.tr("redirect unwrapped")
            : note === "tracking-removed" ? I18n.tr("tracking removed")
            : note === "canonical" ? I18n.tr("site's own address")
            : String(note);
    }

    function sizeOf(row, choices) {
        const r = row.resolved;
        if (root.listed(row) && row.only < 0)
            return -1;
        if (choices.length > 0)
            return choices[root.pickOf(row, choices)].size;
        if ((r.files || []).length > 0)
            return Logic.selectedSize(r.files, Logic.everything(r.files.length));
        return Number(r.size) > 0 ? Number(r.size) : -1;
    }

    function shownRow(row, index) {
        const out = { id: String(index), size: "two", state: row.state, ticked: row.ticked };
        const bare = String(row.url).replace(/^[a-z][a-z0-9+.-]*:\/\//i, "");
        if (row.state === "failed") {
            out.title = bare;
            out.glyph = "link_off";
            out.reason = root.downloads ? String(root.downloads.reason(row.error)) : "";
            return out;
        }
        if (row.state !== "ready") {
            out.title = row.kind === "magnet" ? (RootLogic.magnetName(row.url) || bare) : bare;
            out.glyph = row.kind === "url" ? "link" : "hub";
            out.sub = I18n.tr("Waiting to be read");
            return out;
        }
        const r = row.resolved;
        const choices = root.choicesOf(row);
        const size = root.sizeOf(row, choices);
        const items = (r.items || []).length;
        out.title = String(r.title || r.name || bare);
        out.thumb = String(r.thumb || "");
        out.glyph = Logic.glyph({ kind: r.kind, name: r.name, mime: r.mime });
        out.sub = root.join([
            r.source ? String(r.source.name || "") : "",
            r.kind === "torrent" && (r.files || []).length > 1 ? I18n.tr("%1 files").arg(r.files.length) : "",
            size > 0 ? (choices.length > 0 ? I18n.tr("about %1").arg(Format.bytes(size)) : Format.bytes(size)) : ""
        ]);
        if (items > 1)
            out.spread = row.only < 0 ? I18n.tr("All %1").arg(items) : row.only === 0 ? I18n.tr("First of %1").arg(items) : I18n.tr("This one of %1").arg(items);
        if (choices.length > 0) {
            const pick = root.pickOf(row, choices);
            const choice = choices[pick];
            out.format = choice.kind === "best" ? I18n.tr("Best") : choice.kind === "audio" ? I18n.tr("Audio") : choice.label;
            out.formatNote = choice.kind === "video" ? "" : choice.detail;
            out.own = pick !== Logic.batchPick(choices, root.wanted);
            out.canLess = pick > 0;
            out.canMore = pick < choices.length - 1;
        } else {
            out.format = r.kind === "torrent" ? I18n.tr("Torrent") : I18n.tr("File");
            out.canLess = false;
            out.canMore = false;
        }
        return out;
    }

    readonly property var sections: [{ id: "links", title: "", note: "", rows: root.rows.map(root.shownRow) }]

    function change(index, patch) {
        if (index < 0 || index >= root.rows.length)
            return;
        const next = root.rows.slice();
        next[index] = Object.assign({}, next[index], patch);
        root.rows = next;
    }

    // the engine reads four links at a time; each answer starts the next
    function pump() {
        if (!root.downloads || !root.live)
            return;
        const generation = root.generation;
        const start = Logic.batchNext(root.rows.map(row => row.state), 4);
        if (start.length === 0)
            return;
        const next = root.rows.slice();
        for (var i = 0; i < start.length; i++)
            next[start[i]] = Object.assign({}, next[start[i]], { state: "resolving" });
        root.rows = next;
        start.forEach(index => {
            root.downloads.resolve(root.rows[index].url, function(result, error) {
                if (generation !== root.generation)
                    return;
                if (error)
                    root.change(index, { state: "failed", error: error, ticked: false });
                else
                    // a video pasted from inside a list starts as that video alone
                    {
                    const named = Logic.named(root.rows[index].url, result.items);
                    root.change(index, { state: "ready", resolved: result, only: named, named: named });
                }
                root.pump();
            });
        });
    }

    function read() {
        if (!root.downloads || !root.live)
            return;
        const text = root.text;
        const generation = ++root.generation;
        root.parsed = false;
        root.failure = null;
        root.rows = [];
        root.tally = null;
        if (text.trim().length === 0)
            return;
        root.downloads.links(text, function(result, error) {
            if (generation !== root.generation)
                return;
            root.parsed = true;
            if (error) {
                root.failure = error;
                return;
            }
            root.tally = RootLogic.batch(result);
            root.rows = (result.links || []).filter(link => link && link.duplicate !== true && String(link.url || "").length > 0).map(link => ({
                raw: String(link.raw || link.url), url: String(link.url), host: String(link.host || ""), kind: String(link.kind || "url"),
                notes: link.notes || [], state: "waiting", resolved: null, error: null, ticked: true, own: -1, only: -1, named: -1
            }));
            list.selectFirst();
            root.pump();
        });
    }
    onTextChanged: settle.restart()
    onLiveChanged: if (root.live) settle.restart()
    onDownloadsChanged: settle.restart()
    Timer { id: settle; interval: 140; onTriggered: root.read() }

    function move(delta) { list.move(delta); }
    function jumpSection(delta) { list.select(delta > 0 ? list.count - 1 : 0); }
    function page(delta) { list.page(delta); }

    function tick() {
        const row = root.row;
        if (!row)
            return;
        if (row.state === "failed") {
            root.alert(root.downloads ? String(root.downloads.reason(row.error)) : "");
            return;
        }
        root.change(root.at, { ticked: !row.ticked });
    }
    function tickAll(on) {
        root.rows = root.rows.map(row => row.state === "failed" ? row : Object.assign({}, row, { ticked: on }));
    }
    function stepRow(delta) {
        const row = root.row;
        const choices = root.rowChoices;
        if (!row || choices.length < 2)
            return;
        const next = Logic.clamp(root.pickOf(row, choices) + delta, 0, choices.length - 1);
        // back on the choice for all, the row follows it again
        root.change(root.at, { own: next === Logic.batchPick(choices, root.wanted) ? -1 : next });
    }
    function stepAll(delta) {
        root.all = Logic.clamp(root.all + delta, 0, Logic.BATCH.length - 1);
    }
    function spread() {
        // all of the list, or the one entry the link named (its first when it named none)
        if (root.rowListed)
            root.change(root.at, { only: root.row.only >= 0 ? -1 : Math.max(0, root.row.named) });
    }

    function add(paused) {
        if (!root.downloads || root.adding)
            return;
        const items = [];
        const from = [];
        for (var i = 0; i < root.rows.length; i++) {
            const row = root.rows[i];
            if (!row.ticked || row.state === "failed")
                continue;
            const choices = root.choicesOf(row);
            const only = root.listed(row) ? row.only : -1;
            const read = row.state === "ready" ? row.resolved : null;
            const title = !read ? "" : only >= 0 ? String(read.items[only].title || "") : String(read.title || read.name || "");
            items.push(Logic.batchItem(row.url, choices.length > 0 ? choices[root.pickOf(row, choices)] : null, root.wanted, only, title));
            from.push(i);
        }
        if (items.length === 0) {
            root.alert(I18n.tr("Nothing is ticked"));
            return;
        }
        root.adding = true;
        root.downloads.addMany(items, { dir: root.folder, paused: paused === true }, function(result, error) {
            root.adding = false;
            if (error) {
                root.alert(String(root.downloads.reason(error)));
                return;
            }
            const errors = result.errors || [];
            const tasks = result.tasks || [];
            if (errors.length === 0) {
                root.added(tasks, paused === true);
                return;
            }
            // what the engine refused stays, with its reason; the rest has gone to the queue
            root.generation++;
            root.rows = errors.filter(entry => entry.index >= 0 && entry.index < from.length).map(entry => Object.assign({}, root.rows[from[entry.index]], {
                state: "failed", ticked: false, error: { code: entry.code, message: entry.message }
            }));
            root.alert(I18n.tr("%1 added, %2 could not be").arg(tasks.length).arg(errors.length));
        });
    }

    // the summary and the one format for all
    Item {
        id: head
        width: parent.width
        height: Math.round(50 * root.s)

        Line {
            x: Math.round(20 * root.s)
            anchors.verticalCenter: parent.verticalCenter
            width: Math.max(0, forAll.x - x - Math.round(16 * root.s))
            s: root.s
            size: 14.5
            font.weight: Font.Medium
            text: root.failure ? (root.downloads ? String(root.downloads.reason(root.failure)) : "")
                : !root.parsed ? I18n.tr("Reading what you pasted")
                : root.summary
        }
        Row {
            id: forAll
            x: parent.width - width - Math.round(20 * root.s)
            anchors.verticalCenter: parent.verticalCenter
            visible: root.count > 0
            spacing: Math.round(6 * root.s)

            Micro { anchors.verticalCenter: parent.verticalCenter; s: root.s; text: I18n.tr("Format for all") }
            Item { width: Math.round(4 * root.s); height: 1 }
            Repeater {
                model: root.allNames

                delegate: Chip {
                    required property string modelData
                    required property int index

                    anchors.verticalCenter: parent.verticalCenter
                    s: root.s
                    text: modelData
                    tier: index === root.all ? "accent" : "line"
                    onClicked: root.all = index
                }
            }
            Item { width: Math.round(4 * root.s); height: 1 }
            KeyHint { anchors.verticalCenter: parent.verticalCenter; s: root.s; hint: "Ctrl+Right" }
        }
    }

    ResultList {
        id: list
        anchors.top: head.bottom
        width: parent.width
        height: implicitHeight
        visible: count > 0
        s: root.s
        sections: root.sections
        maximumHeight: Math.max(0, root.maximumHeight - head.height - strip.height)
        rowDelegate: BatchRow {}
        onActivated: root.tick()
    }

    Item {
        id: blank
        anchors.top: head.bottom
        width: parent.width
        visible: list.count === 0
        implicitHeight: Math.round(46 * root.s)
        height: implicitHeight

        Busy {
            x: Math.round(20 * root.s)
            y: Math.round(4 * root.s)
            visible: !root.parsed && root.text.trim().length > 0
            s: root.s
        }
        Line {
            x: Math.round(20 * root.s)
            width: parent.width - x * 2
            visible: root.parsed && !root.failure
            s: root.s
            size: 12.5
            text: I18n.tr("No link was found in it.")
            ink: Lithic.fg2
        }
    }

    // the selected row's link as it was pasted, and where the batch lands
    Item {
        id: strip
        anchors.top: list.visible ? list.bottom : blank.bottom
        width: parent.width
        height: Math.round(36 * root.s)

        Rectangle {
            x: Math.round(8 * root.s)
            width: parent.width - x * 2
            height: 1
            color: Look.lithic ? Lithic.alpha(Lithic.vein, 0.7) : Lithic.alpha(Lithic.fg, Look.dark ? 0.08 : 0.1)
        }
        Symbol {
            id: originMark
            x: Math.round(20 * root.s)
            anchors.verticalCenter: parent.verticalCenter
            visible: root.row !== null
            s: root.s
            name: "link"
            size: 15
            ink: Lithic.fg3
        }
        Line {
            x: originMark.x + originMark.width + Math.round(8 * root.s)
            anchors.verticalCenter: parent.verticalCenter
            width: Math.max(0, where.x - x - Math.round(20 * root.s))
            visible: root.row !== null
            s: root.s
            size: 12
            text: !root.row ? ""
                : root.row.notes.length > 0 ? I18n.tr("Pasted as %1 (%2)").arg(root.row.raw).arg(root.row.notes.map(root.noteWord).join(", "))
                : root.row.raw
            ink: Lithic.fg2
            elide: Text.ElideMiddle
        }
        Row {
            id: where
            x: parent.width - width - Math.round(20 * root.s)
            anchors.verticalCenter: parent.verticalCenter
            spacing: Math.round(6 * root.s)

            Symbol { anchors.verticalCenter: parent.verticalCenter; s: root.s; name: "folder"; size: 15; ink: Lithic.fg3 }
            Line { anchors.verticalCenter: parent.verticalCenter; s: root.s; size: 12; text: root.tilde(root.dir); ink: Lithic.fg2 }
        }
    }
}
