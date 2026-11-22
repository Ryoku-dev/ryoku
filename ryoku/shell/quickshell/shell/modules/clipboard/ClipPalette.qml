pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Text
import Ryoku.Ui.Singletons
import shell.services
import "../ask/bar"
import "../ask/look"
import "lib/clip.js" as Clip

FocusScope {
    id: root

    required property real s
    property bool active: false
    property real maximumWidth: 880 * s
    property real maximumHeight: 520 * s

    signal requestClose()

    readonly property bool roomy: root.maximumWidth >= Math.round(640 * root.s)
    readonly property real stageHeight: Math.max(Math.round(160 * root.s),
        Math.min(Math.round(404 * root.s), root.maximumHeight - field.height - foot.height - 2))

    implicitWidth: Math.min(Math.round((root.roomy ? 880 : 520) * root.s), root.maximumWidth)
    implicitHeight: field.height + root.stageHeight + foot.height + 2
    focus: true

    property string view: "recent"
    readonly property string query: field.text.trim()
    property real now: Date.now()
    property bool ctrlHeld: false
    // the list as it stood is held still while the panel leaves, so a copy's promotion never jumps it
    property string copiedId: ""
    property var frozen: null
    // a delete waits here for its undo
    property var pending: null

    readonly property var entries: Clipboard.entries || []
    readonly property var infos: {
        const out = {};
        for (const e of root.entries) {
            if (e)
                out[String(e.id)] = Clip.read(e);
        }
        return out;
    }
    readonly property var pool: root.entries.filter(e => e && !root.isPending(e)
        && (root.view !== "starred" || e.starred === true))
    readonly property int total: root.entries.filter(e => e && !root.isPending(e)).length
    readonly property int starredCount: root.entries.filter(e => e && e.starred === true && !root.isPending(e)).length
    readonly property var ranked: {
        if (root.query.length === 0)
            return null;
        const q = root.query.toLowerCase();
        const hits = [];
        for (let i = 0; i < root.pool.length; i++) {
            const e = root.pool[i];
            const score = FuzzyRanker.score(Clip.searchable(e, root.infos[String(e.id)] || Clip.read(e)), q);
            if (score < 99)
                hits.push({ entry: e, score: score, at: i });
        }
        hits.sort((a, b) => a.score !== b.score ? a.score - b.score : a.at - b.at);
        return hits.map(hit => hit.entry);
    }
    readonly property var built: {
        let n = 0;
        const toRow = e => root.rowOf(e, ++n <= 9 ? n : 0);
        if (root.ranked !== null)
            return root.ranked.length > 0 ? [{ id: "hits", title: "", rows: root.ranked.map(toRow) }] : [];
        if (root.view === "starred")
            return root.pool.length > 0 ? [{ id: "starred", title: "", rows: root.pool.map(toRow) }] : [];
        return Clip.groups(root.pool, root.now).map(group => ({
            id: group.id,
            title: root.dayTitle(group.id),
            note: group.id === "all" ? "" : String(group.rows.length),
            rows: group.rows.map(toRow)
        }));
    }
    readonly property var sections: root.frozen !== null ? root.frozen : root.built
    readonly property var current: list.current
    readonly property var currentEntry: root.current ? root.current.entry : null
    readonly property var detail: root.current ? root.detailOf(root.current) : null

    readonly property string status: {
        if (root.query.length > 0) {
            const hits = root.ranked ? root.ranked.length : 0;
            return hits === 1 ? I18n.tr("1 match") : I18n.tr("%1 matches").arg(hits);
        }
        if (root.view === "starred")
            return root.starredCount === 1 ? I18n.tr("1 starred") : I18n.tr("%1 starred").arg(root.starredCount);
        return root.total === 1 ? I18n.tr("1 item") : I18n.tr("%1 items").arg(root.total);
    }

    readonly property var actions: {
        const e = root.currentEntry;
        const out = [];
        if (e) {
            out.push({ id: "copy", label: I18n.tr("Copy"), glyph: "content_copy", hint: "Enter", section: "" });
            if (root.current.info.kind === "link")
                out.push({ id: "open", label: I18n.tr("Open in the browser"), glyph: "open_in_new", hint: "Ctrl+Enter", section: "" });
            out.push({ id: "star", label: e.starred === true ? I18n.tr("Remove the star") : I18n.tr("Star"), glyph: "star", hint: "Ctrl+S", section: "" });
            out.push({ id: "delete", label: I18n.tr("Delete"), glyph: "delete", hint: "Delete", section: "", destructive: true });
        }
        out.push({ id: "manage", label: I18n.tr("Clear or manage the history"), glyph: "settings", hint: "", section: I18n.tr("History") });
        return out;
    }

    function isPending(entry) {
        return root.pending !== null && String(entry.id) === root.pending.id;
    }

    function bytes(n) {
        n = Number(n) || 0;
        if (n < 1024)
            return I18n.tr("%1 B").arg(n);
        if (n < 1048576)
            return I18n.tr("%1 KB").arg((n / 1024).toFixed(1));
        return I18n.tr("%1 MB").arg((n / 1048576).toFixed(1));
    }

    function homeShort(path) {
        return String(path || "").replace(/^\/home\/[^\/]+/, "~");
    }

    function dayTitle(id) {
        return id === "today" ? I18n.tr("Today")
            : id === "yesterday" ? I18n.tr("Yesterday")
            : id === "earlier" ? I18n.tr("Earlier") : "";
    }

    function shortAgo(at) {
        const a = Clip.ago(at, root.now);
        if (!a)
            return "";
        return a.unit === "now" ? I18n.tr("now")
            : a.unit === "min" ? I18n.tr("%1m").arg(a.n)
            : a.unit === "hour" ? I18n.tr("%1h").arg(a.n)
            : a.unit === "day" ? I18n.tr("%1d").arg(a.n)
            : new Date(Number(at)).toLocaleDateString(Qt.locale(), I18n.tr("d MMM"));
    }

    function longAgo(at) {
        const a = Clip.ago(at, root.now);
        if (!a)
            return "";
        return a.unit === "now" ? I18n.tr("Just now")
            : a.unit === "min" ? (a.n === 1 ? I18n.tr("A minute ago") : I18n.tr("%1 minutes ago").arg(a.n))
            : a.unit === "hour" ? (a.n === 1 ? I18n.tr("An hour ago") : I18n.tr("%1 hours ago").arg(a.n))
            : a.unit === "day" ? (a.n === 1 ? I18n.tr("A day ago") : I18n.tr("%1 days ago").arg(a.n))
            : new Date(Number(at)).toLocaleDateString(Qt.locale(), I18n.tr("d MMMM yyyy"));
    }

    function glyphOf(info) {
        return info.kind === "link" ? "link"
            : info.kind === "command" ? "terminal"
            : info.kind === "code" ? "code"
            : info.kind === "files" ? (info.files.paths.length > 1 ? "folder_copy" : "draft")
            : info.kind === "binary" ? "draft"
            : "notes";
    }

    function typeName(info) {
        return info.kind === "link" ? I18n.tr("Link")
            : info.kind === "colour" ? I18n.tr("Colour")
            : info.kind === "command" ? I18n.tr("Command")
            : info.kind === "code" ? I18n.tr("Code")
            : info.kind === "files" ? (info.files.paths.length > 1 ? I18n.tr("Files") : I18n.tr("File"))
            : info.kind === "image" ? I18n.tr("%1 image").arg(String(info.mime).replace(/^image\//, "").toUpperCase())
            : info.kind === "binary" ? info.mime
            : I18n.tr("Text");
    }

    function lengthOf(info) {
        if (info.cut)
            return root.bytes(info.size);
        const chars = info.chars === 1 ? I18n.tr("1 character") : I18n.tr("%1 characters").arg(info.chars);
        return info.lines > 1 ? [chars, I18n.tr("%1 lines").arg(info.lines)].join(" \u00b7 ") : chars;
    }

    function metaOf(info) {
        if (info.kind === "image")
            return [I18n.tr("%1 \u00d7 %2").arg(info.w).arg(info.h), root.bytes(info.size)].join(" \u00b7 ");
        if (info.kind === "binary")
            return root.bytes(info.size);
        if (info.kind === "link")
            return I18n.tr("Link");
        if (info.kind === "colour")
            return info.colour.hex.toLowerCase() !== info.colour.value.toLowerCase()
                ? info.colour.hex.toUpperCase() : I18n.tr("RGB %1, %2, %3").arg(info.colour.r).arg(info.colour.g).arg(info.colour.b);
        if (info.kind === "files") {
            const where = root.homeShort(info.files.dir);
            return info.files.paths.length > 1 ? [I18n.tr("%1 files").arg(info.files.paths.length), where].join(" \u00b7 ") : where;
        }
        if (info.kind === "command")
            return info.lines > 1 ? I18n.tr("%1 commands").arg(info.lines) : I18n.tr("Command");
        if (info.kind === "code")
            return [I18n.tr("Code"), root.lengthOf(info)].join(" \u00b7 ");
        return root.lengthOf(info);
    }

    function rowOf(e, number) {
        const info = root.infos[String(e.id)] || Clip.read(e);
        return {
            id: String(e.id), size: "two", entry: e, info: info,
            kind: info.kind, glyph: root.glyphOf(info), thumb: String(e.thumb || ""),
            swatch: info.kind === "colour" ? info.colour.hex : "",
            title: info.kind === "image" ? I18n.tr("Image") : info.title,
            mono: info.mono === true, meta: root.metaOf(info), time: root.shortAgo(e.at),
            starred: e.starred === true, number: number
        };
    }

    function detailOf(row) {
        const e = row.entry, info = row.info;
        const facts = [{ label: I18n.tr("Type"), value: root.typeName(info) }];
        if (info.kind === "image") {
            facts.push({ label: I18n.tr("Pixels"), value: I18n.tr("%1 \u00d7 %2").arg(info.w).arg(info.h) });
            facts.push({ label: I18n.tr("Size"), value: root.bytes(info.size) });
        } else if (info.kind === "colour") {
            if (info.colour.hex.toLowerCase() !== info.colour.value.toLowerCase())
                facts.push({ label: I18n.tr("Hex"), value: info.colour.hex.toUpperCase(), mono: true });
            facts.push({ label: I18n.tr("RGB"), value: [info.colour.r, info.colour.g, info.colour.b].join(", "), mono: true });
            facts.push({ label: I18n.tr("HSL"), value: I18n.tr("%1\u00b0, %2%, %3%").arg(info.colour.h).arg(info.colour.s).arg(info.colour.l), mono: true });
        } else if (info.kind === "files") {
            facts.push({ label: I18n.tr("Folder"), value: root.homeShort(info.files.dir) });
        } else if (info.kind === "binary") {
            facts.push({ label: I18n.tr("Size"), value: root.bytes(info.size) });
        } else if (info.kind !== "link") {
            facts.push({ label: I18n.tr("Length"), value: root.lengthOf(info) });
        }
        const when = root.longAgo(e.at);
        if (when.length > 0)
            facts.push({ label: I18n.tr("Copied"), value: when });
        const names = info.kind === "files" ? info.files.names : [];
        return {
            id: row.id, kind: info.kind, title: row.title, mono: info.mono === true, text: info.text || "",
            note: info.cut ? I18n.tr("Shows the first %1 characters of %2").arg(info.chars).arg(root.bytes(info.size)) : "",
            thumb: String(e.thumb || ""), thumbW: Number(e.thumbW) || 0, thumbH: Number(e.thumbH) || 0,
            swatch: row.swatch, link: info.link || null, mime: info.mime,
            files: names.slice(0, 8), more: names.length > 8 ? I18n.tr("%1 more").arg(names.length - 8) : "",
            facts: facts, starred: e.starred === true
        };
    }

    function focusField() {
        if (!panel.open)
            field.focusField();
    }

    function copy(entry) {
        if (!entry || root.copiedId.length > 0)
            return;
        root.commitPending();
        root.frozen = root.built;
        Clipboard.copy(Number(entry.id));
        root.copiedId = String(entry.id);
        foot.flash(I18n.tr("Copied"), false);
        leave.restart();
    }

    function copyNumber(n) {
        const rows = list.layout.rows;
        if (n >= 1 && n <= rows.length)
            root.copy(rows[n - 1].row.entry);
    }

    function open(entry) {
        const info = entry ? root.infos[String(entry.id)] : null;
        if (!info || info.kind !== "link")
            return;
        Spawn.runApp(["xdg-open", info.link.url], { label: "browser" });
        root.requestClose();
    }

    function keepNeighbour(entry) {
        const rows = list.layout.rows;
        let at = -1;
        for (let i = 0; i < rows.length; i++) {
            if (rows[i].row.id === String(entry.id)) {
                at = i;
                break;
            }
        }
        const next = at < 0 ? null : at + 1 < rows.length ? rows[at + 1] : at > 0 ? rows[at - 1] : null;
        if (next) {
            list.selectedId = next.row.id;
            list.held = true;
        }
    }

    function star(entry) {
        if (!entry)
            return;
        const on = entry.starred !== true;
        if (!on && root.view === "starred")
            root.keepNeighbour(entry);
        Clipboard.star(Number(entry.id), on);
        foot.flash(on ? I18n.tr("Starred") : I18n.tr("Star removed"), false);
    }

    function remove(entry) {
        if (!entry)
            return;
        root.commitPending();
        root.keepNeighbour(entry);
        root.pending = { id: String(entry.id) };
        undoWindow.restart();
    }

    function undo() {
        if (root.pending === null)
            return;
        undoWindow.stop();
        const id = root.pending.id;
        root.pending = null;
        Qt.callLater(() => {
            list.selectedId = id;
            list.held = true;
            list.reveal(list.selectedIndex);
        });
    }

    function commitPending() {
        if (root.pending === null)
            return;
        undoWindow.stop();
        Clipboard.del(Number(root.pending.id));
        root.pending = null;
    }

    function switchView() {
        root.view = root.view === "recent" ? "starred" : "recent";
        list.selectFirst();
    }

    function openPanel() {
        panel.open = true;
    }

    function closePanel() {
        panel.open = false;
        field.focusField();
    }

    function run(id) {
        root.closePanel();
        const e = root.currentEntry;
        if (id === "copy")
            root.copy(e);
        else if (id === "open")
            root.open(e);
        else if (id === "star")
            root.star(e);
        else if (id === "delete")
            root.remove(e);
        else if (id === "manage") {
            Bus.run("hub open clipboard");
            root.requestClose();
        }
    }

    function escapeLadder() {
        if (panel.open)
            root.closePanel();
        else if (field.text.length > 0)
            field.clear();
        else if (root.view !== "recent")
            root.switchView();
        else
            root.requestClose();
    }

    function handleKey(event) {
        const mods = event.modifiers & (Qt.ControlModifier | Qt.ShiftModifier | Qt.AltModifier);
        const ctrl = (mods & Qt.ControlModifier) !== 0;
        const key = event.key;
        root.ctrlHeld = ctrl || key === Qt.Key_Control;
        if (key === Qt.Key_Control)
            return;
        if (key === Qt.Key_Escape) {
            root.escapeLadder();
        } else if (ctrl && key === Qt.Key_K) {
            panel.open ? root.closePanel() : root.openPanel();
        } else if (key === Qt.Key_Return || key === Qt.Key_Enter) {
            if (ctrl)
                root.open(root.currentEntry);
            else
                root.copy(root.currentEntry);
        } else if (key === Qt.Key_Down || key === Qt.Key_Up) {
            if (ctrl)
                list.jumpSection(key === Qt.Key_Down ? 1 : -1);
            else
                list.move(key === Qt.Key_Down ? 1 : -1);
        } else if (mods === Qt.ControlModifier && (key === Qt.Key_N || key === Qt.Key_P)) {
            list.move(key === Qt.Key_N ? 1 : -1);
        } else if (key === Qt.Key_PageDown || key === Qt.Key_PageUp) {
            list.page(key === Qt.Key_PageDown ? 1 : -1);
        } else if ((key === Qt.Key_Tab || key === Qt.Key_Backtab) && !ctrl) {
            root.switchView();
        } else if (mods === Qt.ControlModifier && key === Qt.Key_S) {
            root.star(root.currentEntry);
        } else if (key === Qt.Key_Delete && mods === 0 && field.cursorPosition >= field.text.length) {
            root.remove(root.currentEntry);
        } else if (mods === Qt.ControlModifier && key === Qt.Key_Z && root.pending !== null) {
            root.undo();
        } else if (mods === Qt.ControlModifier && key >= Qt.Key_1 && key <= Qt.Key_9) {
            root.copyNumber(key - Qt.Key_0);
        } else if ((key === Qt.Key_Home || key === Qt.Key_End) && mods === 0 && field.text.length === 0 && list.count > 0) {
            list.select(key === Qt.Key_Home ? 0 : list.count - 1);
        } else {
            return;
        }
        event.accepted = true;
    }

    Keys.onReleased: event => {
        if (event.key === Qt.Key_Control)
            root.ctrlHeld = false;
    }

    onActiveChanged: {
        if (root.active) {
            root.now = Date.now();
            Qt.callLater(root.focusField);
            return;
        }
        leave.stop();
        root.commitPending();
        panel.open = false;
        root.ctrlHeld = false;
        root.copiedId = "";
        root.frozen = null;
        root.view = "recent";
        field.clear();
        list.selectFirst();
    }

    Timer {
        id: leave
        interval: Look.still ? 1 : 200
        onTriggered: root.requestClose()
    }
    Timer {
        id: undoWindow
        interval: 5000
        onTriggered: root.commitPending()
    }
    Timer {
        interval: 30000
        repeat: true
        running: root.active
        onTriggered: root.now = Date.now()
    }

    SearchField {
        id: field
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        s: root.s
        placeholder: root.view === "starred" ? I18n.tr("Search what you starred") : I18n.tr("Search the clipboard")
        accessory: Component {
            ClipTabs {
                s: root.s
                current: root.view
                tabs: [
                    { id: "recent", label: I18n.tr("Recent"), count: 0 },
                    { id: "starred", label: I18n.tr("Starred"), count: root.starredCount }
                ]
                onPicked: id => {
                    if (id !== root.view)
                        root.switchView();
                    root.focusField();
                }
            }
        }
        onKeyPressed: event => root.handleKey(event)
        onEdited: list.selectFirst()
    }

    Rectangle {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: field.bottom
        height: 1
        color: Look.rule
        visible: !Look.fluent
        z: 3
    }

    Item {
        id: stage
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: field.bottom
        anchors.topMargin: 1
        height: root.stageHeight
        clip: true

        ResultList {
            id: list
            width: root.roomy ? Math.round(parent.width * 0.44) : parent.width
            height: parent.height
            visible: list.count > 0
            s: root.s
            sections: root.sections
            maximumHeight: parent.height
            rowDelegate: Component {
                ClipRow {
                    id: clipRow
                    query: root.query
                    numbers: root.ctrlHeld
                    copied: root.copiedId.length > 0 && clipRow.row.id === root.copiedId
                }
            }
            onActivated: row => root.copy(row.entry)
            onMenuRequested: root.openPanel()
        }

        ClipDetail {
            anchors.left: list.right
            anchors.right: parent.right
            height: parent.height
            visible: root.roomy && list.count > 0
            s: root.s
            item: root.detail
            onCopyRequested: root.copy(root.currentEntry)
            onOpenRequested: root.open(root.currentEntry)
            onStarRequested: root.star(root.currentEntry)
            onDeleteRequested: root.remove(root.currentEntry)
        }

        Column {
            anchors.centerIn: parent
            width: Math.min(parent.width - Math.round(48 * root.s), Math.round(420 * root.s))
            visible: list.count === 0
            spacing: Math.round(8 * root.s)

            readonly property bool searching: root.query.length > 0

            Symbol {
                anchors.horizontalCenter: parent.horizontalCenter
                s: root.s
                name: !Bus.connected ? "content_paste_off" : parent.searching ? "search_off" : root.view === "starred" ? "star" : "content_paste"
                size: 34
                ink: Lithic.fg3
            }
            Text {
                width: parent.width
                horizontalAlignment: Text.AlignHCenter
                text: !Bus.connected ? I18n.tr("The clipboard history is not answering")
                    : parent.searching ? I18n.tr("Nothing matches \u201c%1\u201d").arg(root.query)
                    : root.view === "starred" ? I18n.tr("Nothing starred yet")
                    : I18n.tr("Nothing copied yet")
                color: Lithic.fg
                font.family: Look.sans
                font.pixelSize: Math.round(15 * root.s)
                font.weight: Font.Medium
                wrapMode: Text.WordWrap
                renderType: Text.NativeRendering
            }
            Text {
                width: parent.width
                horizontalAlignment: Text.AlignHCenter
                text: !Bus.connected ? I18n.tr("It comes back with the desktop.")
                    : parent.searching ? (root.view === "starred" ? I18n.tr("Only what you starred is searched here. Tab searches everything.")
                        : I18n.tr("Search reads the text, links, file names, and kinds like image or link."))
                    : root.view === "starred" ? I18n.tr("Ctrl+S stars what is selected. Starred items stay when the history is cleared.")
                    : I18n.tr("Copy text, a link or a picture and it shows up here.")
                color: Lithic.fg2
                font.family: Look.sans
                font.pixelSize: Math.round(12.5 * root.s)
                lineHeight: Math.round(18 * root.s)
                lineHeightMode: Text.FixedHeight
                wrapMode: Text.WordWrap
                renderType: Text.NativeRendering
            }
        }
    }

    Rectangle {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: foot.top
        height: 1
        color: Look.rule
        z: 3
    }

    ActionBar {
        id: foot
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        s: root.s
        glyph: "content_paste"
        context: (Look.lithic ? "\u5199\u3057 \u00b7 " : "") + root.status
        contextItem: root.pending !== null ? undoLine : null
        primary: root.currentEntry ? { label: I18n.tr("Copy"), hint: "Enter" } : null
        hasActions: root.actions.length > 0
        panelOpen: panel.open
        onPrimaryClicked: root.copy(root.currentEntry)
        onActionsClicked: panel.open ? root.closePanel() : root.openPanel()
    }

    Component {
        id: undoLine
        Row {
            spacing: Math.round(8 * root.s)

            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("Deleted")
                color: Lithic.fg
                font.family: Look.sans
                font.pixelSize: Math.round(12 * root.s)
                font.weight: Font.Medium
                renderType: Text.NativeRendering
            }
            Chip {
                anchors.verticalCenter: parent.verticalCenter
                s: root.s
                tier: "line"
                glyph: "undo"
                text: I18n.tr("Undo")
                onClicked: {
                    root.undo();
                    root.focusField();
                }
            }
            KeyHint {
                anchors.verticalCenter: parent.verticalCenter
                s: root.s
                hint: "Ctrl+Z"
            }
        }
    }

    MouseArea {
        anchors.fill: parent
        z: 4
        visible: panel.open
        acceptedButtons: Qt.AllButtons
        onPressed: root.closePanel()
    }

    ActionPanel {
        id: panel
        z: 5
        anchors.right: parent.right
        anchors.rightMargin: Math.round(8 * root.s)
        anchors.bottom: foot.top
        anchors.bottomMargin: Math.round(4 * root.s)
        s: root.s
        actions: root.actions
        maximumHeight: Math.max(120 * root.s, root.stageHeight - 12 * root.s)
        onPicked: id => root.run(id)
        onDismissed: root.closePanel()
    }
}
