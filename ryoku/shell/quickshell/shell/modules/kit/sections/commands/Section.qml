pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Quickshell
import Lithic
import Ryoku.Ui.Singletons
import "../../../ask/look"
import "commands.js" as Logic

Item {
    id: root
    required property real s
    property bool active: false
    property string query: ""
    property var args: ({})
    property var memory: null
    property var source: null
    property real maximumHeight: height
    property string pageId: "ryoku"
    property string mode: ""
    property bool runPending: false
    property string formError: ""
    property var blanks: []
    property var values: ({})
    property int selectedIndex: 0
    property string openedCommand: ""
    readonly property var catalogue: source ? source.catalogue : null
    readonly property var store: source ? source.store : null
    readonly property var entries: source ? source.entries : []
    readonly property int revision: source ? source.revision : 0
    readonly property var rows: {
        const observed = root.revision;
        return Logic.list(root.entries, root.catalogue ? root.catalogue.libraries : [], root.store ? root.store.favouriteIds : [], root.pageId, root.query, function(text) { return I18n.tr(text); });
    }
    readonly property var entry: root.rows.length ? root.rows[Math.min(root.selectedIndex, root.rows.length - 1)].entry : null
    readonly property bool mine: root.entry && root.entry.source === "user"
    readonly property bool ownsText: root.mode !== ""
    readonly property string title: I18n.tr("Commands")
    readonly property string placeholder: I18n.tr("Filter commands")
    readonly property string status: root.formError || (root.store && root.store.error) || (root.store && root.store.notice) || (!root.catalogue || !root.catalogue.ready ? I18n.tr("Reading the command library") : I18n.tr("Fills the blank first; the terminal stays open"))
    readonly property var pages: [
        { id: "ryoku", label: I18n.tr("Ryoku"), count: root.entries.filter(entry => entry.library === "ryoku").length },
        { id: "linux", label: I18n.tr("Linux"), count: root.entries.filter(entry => entry.library === "linux").length },
        { id: "saved", label: I18n.tr("Saved"), count: root.source ? root.source.saved : 0 }
    ]
    readonly property var primaryAction: root.mode ? ({ label: root.mode === "blanks" ? root.runPending ? I18n.tr("Run in a terminal") : I18n.tr("Copy") : root.mode === "delete" ? I18n.tr("Delete command") : root.mode === "import" ? I18n.tr("Import commands") : I18n.tr("Save command"), hint: "Enter" }) : root.entry ? ({ label: I18n.tr("Copy"), glyph: "content_copy" }) : null
    readonly property var secondaryAction: !root.mode && Logic.runnable(root.entry) ? ({ label: I18n.tr("Run in a terminal"), glyph: "terminal", hint: "Ctrl+Enter" }) : null
    readonly property string actionNote: root.entry && !Logic.runnable(root.entry) ? I18n.tr("Copy this command and review it yourself. Commands marked Careful or containing usage syntax cannot run here.") : ""
    readonly property var actions: root.mode || !root.store || !root.store.ready ? [] : (root.entry ? [{ id: "star", label: root.store.isFavourite(root.entry.id) ? I18n.tr("Remove from favourites") : I18n.tr("Add to favourites"), glyph: "star", hint: "Ctrl+S", section: "" }] : []).concat(root.mine ? [
        { id: "edit", label: I18n.tr("Edit command"), glyph: "edit", hint: "Ctrl+E", section: "" },
        { id: "delete", label: I18n.tr("Delete command"), glyph: "delete", hint: "Ctrl+D", section: "", destructive: true }
    ] : []).concat([
        { id: "new", label: I18n.tr("New command"), glyph: "add", hint: "Ctrl+N", section: I18n.tr("My commands") },
        { id: "import", label: I18n.tr("Import commands"), glyph: "upload_file", hint: "Ctrl+I", section: I18n.tr("My commands") }
    ])
    signal closeRequested()
    signal hud(string text)
    signal alert(string text)
    signal queryRequested(string text)
    signal menuRequested()

    function translated(value) { return root.mine ? value : I18n.tr(value); }
    function takeFocus() { Qt.callLater(function() { const field = fields.itemAt(0); if (field) field.takeFocus(); else root.forceActiveFocus(); }); }
    function move(delta) { if (!root.mode) { root.selectedIndex = Math.max(0, Math.min(root.rows.length - 1, root.selectedIndex + delta)); list.positionViewAtIndex(root.selectedIndex, ListView.Contain); } }
    function page(delta) { root.move(delta * Math.max(1, Math.floor(list.height / (34 * root.s)) - 1)); }
    function jumpSection(delta) {
        const here = root.rows[root.selectedIndex];
        let next = root.selectedIndex + delta;
        while (here && next >= 0 && next < root.rows.length && root.rows[next].group === here.group) next += delta;
        root.move(next - root.selectedIndex);
    }
    function handleEscape() { if (!root.mode) return false; root.mode = ""; root.formError = ""; return true; }
    function handleKey(event) {
        if (root.mode && (event.key === Qt.Key_Return || event.key === Qt.Key_Enter)) { root.submit(); return true; }
        if (!root.mode && (event.key === Qt.Key_Home || event.key === Qt.Key_End)) { root.move(event.key === Qt.Key_Home ? -root.rows.length : root.rows.length); return true; }
        return false;
    }
    function activate() { if (root.mode) root.submit(); else root.prepare(false); }
    function activateSecondary() { if (!root.mode && Logic.runnable(root.entry)) root.prepare(true); }
    function prepare(run) {
        if (!root.entry) return;
        root.runPending = run;
        root.blanks = Logic.placeholders(root.entry.command);
        root.values = ({});
        if (root.blanks.length) { root.mode = "blanks"; root.takeFocus(); } else root.deliver(root.entry.command, run);
    }
    function deliver(command, run) {
        if (run) {
            if (!Logic.runnable(root.entry)) return;
            Spawn.runApp(["ryoku-app", "terminal", "--", "/usr/bin/bash", "-c", Logic.terminalLine(command, I18n.tr("Press Enter to close this window."))]);
            root.hud(I18n.tr("Opened in a terminal"));
        } else { Quickshell.clipboardText = command; root.hud(I18n.tr("Copied")); }
        root.mode = "";
        if (run) root.closeRequested();
    }
    function submit() {
        const data = ({});
        for (let i = 0; i < fields.count; i++) data[fields.itemAt(i).label] = fields.itemAt(i).text;
        if (root.mode === "blanks") {
            if (root.blanks.some(name => !String(data[name] || "").trim())) { root.formError = I18n.tr("Fill every blank first"); return; }
            root.deliver(Logic.fill(root.entry.command, data), root.runPending);
        } else if (root.mode === "delete") { root.store.deleteCommand(root.entry.id); root.mode = ""; }
        else if (root.mode === "import") { root.store.importCommands(data[I18n.tr("JSON file path")]); root.mode = ""; root.pageId = "saved"; }
        else {
            const name = data[I18n.tr("Name")]; const command = data[I18n.tr("Command")];
            if (!name.trim() || !command.trim()) { root.formError = I18n.tr("Give the command a name and command line"); return; }
            const desc = data[I18n.tr("Description")]; const tags = data[I18n.tr("Tags")].split(",");
            if (root.mode === "edit") root.store.updateCommand(root.entry.id, name, command, desc, tags);
            else root.store.addCommand(name, command, desc, tags);
            root.mode = ""; root.pageId = "saved"; root.queryRequested("");
        }
        root.formError = "";
    }
    function act(id) {
        if (!root.active || !root.store || root.mode) return;
        root.formError = "";
        if (id === "star" && root.entry) root.store.toggleFavourite(root.entry.id);
        else if (id === "new" || id === "edit" || id === "import" || id === "delete") {
            root.values = id === "edit" && root.entry ? ({ name: root.entry.title, command: root.entry.command, description: root.entry.description, tags: root.entry.tags.join(", ") }) : ({});
            root.mode = id;
            root.takeFocus();
        }
    }
    function restore() {
        if (root.memory) root.pageId = Logic.pageId(root.memory.get("commands.rail", "library:ryoku"));
    }
    function selectArgs() {
        if (!root.args.command || root.openedCommand === root.args.command) return;
        const index = root.rows.findIndex(row => row.entry.id === root.args.command);
        if (index >= 0) { root.openedCommand = root.args.command; root.selectedIndex = index; list.positionViewAtIndex(index, ListView.Contain); }
    }
    onMemoryChanged: restore()
    onArgsChanged: { root.openedCommand = ""; root.selectArgs(); }
    onRowsChanged: { root.selectedIndex = Math.min(root.selectedIndex, Math.max(0, root.rows.length - 1)); root.selectArgs(); }
    onQueryChanged: { root.selectedIndex = 0; if (root.mode) root.handleEscape(); }
    onPageIdChanged: {
        const clean = Logic.pageId(root.pageId);
        if (clean !== root.pageId) { root.pageId = clean; return; }
        root.selectedIndex = 0; root.mode = "";
        if (root.memory) root.memory.set("commands.rail", "library:" + root.pageId);
    }
    Component.onCompleted: restore()

    Shape { anchors.fill: list; visible: Look.boxed; s: root.s; fill: Look.card; cutSize: Look.cutCard }
    ListView {
        id: list
        x: 10 * root.s; y: 10 * root.s
        width: Math.floor((parent.width - 30 * root.s) * 0.51); height: parent.height - 20 * root.s
        clip: true; model: root.rows; currentIndex: root.selectedIndex
        enabled: !root.ownsText
        boundsBehavior: Flickable.StopAtBounds
        section.property: "group"; section.criteria: ViewSection.FullString; section.labelPositioning: ViewSection.InlineLabels | ViewSection.CurrentLabelAtStart
        section.delegate: Rectangle {
            required property string section
            width: list.width; height: 30 * root.s; color: Look.frame
            Text { anchors.left: parent.left; anchors.leftMargin: 12 * root.s; anchors.verticalCenter: parent.verticalCenter; text: section; color: Lithic.fg2; font.family: Look.sans; font.pixelSize: Math.round(12 * root.s); font.weight: Font.Medium }
        }
        delegate: Item {
            id: row
            required property var modelData
            required property int index
            width: list.width; height: 34 * root.s
            Shape { anchors.fill: parent; s: root.s; fill: row.index === root.selectedIndex ? Look.plate : hover.hovered ? Look.hover : "transparent"; cutSize: Look.cutRow; Behavior on fill { ColorAnimation { duration: Look.dur(Look.plateMs) } } }
            Text { x: 12 * root.s; width: parent.width * 0.60 - 16 * root.s; anchors.verticalCenter: parent.verticalCenter; text: row.modelData.entry.source === "user" ? row.modelData.entry.title : I18n.tr(row.modelData.entry.title); elide: Text.ElideRight; color: Lithic.fg; font.family: Look.sans; font.pixelSize: Math.round(13.5 * root.s) }
            Symbol { x: parent.width * 0.60; anchors.verticalCenter: parent.verticalCenter; s: root.s; size: 13; name: row.modelData.entry.danger ? "warning" : row.modelData.entry.privileged ? "lock" : ""; ink: row.modelData.entry.danger ? Lithic.amber : Lithic.fg3 }
            Text { x: parent.width * 0.64; width: parent.width * 0.36 - 12 * root.s; anchors.verticalCenter: parent.verticalCenter; text: row.modelData.entry.command; elide: Text.ElideRight; horizontalAlignment: Text.AlignRight; color: Lithic.fg3; font.family: Look.mono; font.pixelSize: Math.round(10.5 * root.s) }
            HoverHandler { id: hover }
            TapHandler { onTapped: root.selectedIndex = row.index; onDoubleTapped: root.activate() }
        }
        ScrollBar.vertical: ScrollBar { policy: ScrollBar.AsNeeded }
    }
    Text {
        visible: root.rows.length === 0; x: list.x + 16 * root.s; y: list.y + 30 * root.s; width: list.width - 32 * root.s
        text: root.pageId === "saved" && !root.query ? I18n.tr("Star a command or add your own with Ctrl+N.") : I18n.tr("No commands match"); wrapMode: Text.Wrap; color: Lithic.fg2; font.family: Look.sans; font.pixelSize: Math.round(14 * root.s)
    }
    Flickable {
        id: detail
        x: list.x + list.width + 10 * root.s; y: 10 * root.s
        width: parent.width - x - 10 * root.s; height: parent.height - 20 * root.s
        contentHeight: pane.height; clip: true; boundsBehavior: Flickable.StopAtBounds
        Shape {
            id: pane
            width: detail.width; height: content.height + 44 * root.s; visible: root.entry !== null || root.mode !== ""
            s: root.s; cutSize: Look.cutCard; fill: Look.card; strokeColor: Look.cardStroke; strokeWidth: 1
            Column {
                id: content
                x: 22 * root.s; y: 22 * root.s; width: parent.width - 44 * root.s; spacing: 16 * root.s
                Text { width: parent.width; text: root.mode === "new" ? I18n.tr("New command") : root.mode === "import" ? I18n.tr("Import commands") : root.mode === "delete" ? I18n.tr("Delete this command?") : root.entry ? root.translated(root.entry.title) : ""; wrapMode: Text.Wrap; color: Lithic.fg; font.family: Look.sans; font.pixelSize: Math.round(19 * root.s); font.weight: Font.DemiBold }
                Text { width: parent.width; visible: root.entry !== null && root.mode !== "new" && root.mode !== "import"; text: root.entry ? root.rows[root.selectedIndex].group + " · " + root.entry.tool : ""; color: Lithic.fg2; font.family: Look.sans; font.pixelSize: Math.round(12.5 * root.s); wrapMode: Text.Wrap }
                Shape {
                    width: parent.width; height: commandText.implicitHeight + 28 * root.s; s: root.s; fill: Lithic.ground; strokeColor: Lithic.vein; strokeWidth: 1
                    visible: root.entry !== null && ["new", "edit", "import"].indexOf(root.mode) < 0
                    Text { id: commandText; x: 14 * root.s; y: 14 * root.s; width: parent.width - 28 * root.s; text: root.entry ? root.mode === "blanks" ? Logic.fill(root.entry.command, root.values) : root.entry.command : ""; wrapMode: Text.WrapAnywhere; color: Lithic.fg; font.family: Look.mono; font.pixelSize: Math.round(13 * root.s) }
                }
                Repeater {
                    id: fields
                    model: root.mode === "blanks" ? root.blanks : root.mode === "import" ? [I18n.tr("JSON file path")] : root.mode === "new" || root.mode === "edit" ? [I18n.tr("Name"), I18n.tr("Command"), I18n.tr("Description"), I18n.tr("Tags")] : []
                    EntryField {
                        required property var modelData
                        required property int index
                        width: root.mode === "blanks" ? Math.min(content.width, 280 * root.s) : content.width; s: root.s; label: modelData
                        text: root.mode === "edit" ? String(root.values[["name", "command", "description", "tags"][index]] || "") : ""
                        onTextChanged: {
                            if (root.mode !== "blanks") return;
                            const next = Object.assign({}, root.values);
                            next[modelData] = text;
                            root.values = next;
                        }
                        onNavigate: delta => fields.itemAt((index + delta + fields.count) % fields.count).takeFocus()
                        onAccepted: { if (index + 1 < fields.count) fields.itemAt(index + 1).takeFocus(); else root.submit(); }
                    }
                }
                Text { width: parent.width; visible: root.entry !== null && ["new", "edit", "import"].indexOf(root.mode) < 0; text: root.entry ? root.translated(root.entry.description) : ""; wrapMode: Text.Wrap; color: Lithic.fg; font.family: Look.sans; font.pixelSize: Math.round(14 * root.s); lineHeight: 1.4 }
                Flow {
                    width: parent.width; spacing: 8 * root.s; visible: root.entry !== null && !root.mode
                    Repeater {
                        model: root.entry ? (root.entry.privileged ? [I18n.tr("Needs password")] : []).concat(root.entry.danger ? [I18n.tr("Careful")] : []).concat(root.entry.level === "advanced" ? [I18n.tr("Advanced")] : []).concat(root.entry.onlyOn ? [I18n.tr("Only on %1").arg(root.entry.onlyOn)] : []).concat(root.entry.requires ? [I18n.tr("Needs %1").arg(root.entry.requires)] : []) : []
                        Chip { required property var modelData; s: root.s; text: modelData }
                    }
                }
                Text { width: parent.width; visible: root.entry !== null && !root.mode; text: root.entry ? I18n.tr("Tool: %1\nEditions: %2\nCompositors: %3").arg(root.entry.tool).arg(root.entry.editions.length ? root.entry.editions.join(", ") : I18n.tr("All")).arg(root.entry.compositors.length ? root.entry.compositors.join(", ") : I18n.tr("All")) : ""; color: Lithic.fg2; font.family: Look.sans; font.pixelSize: Math.round(12 * root.s); lineHeight: 1.6; wrapMode: Text.Wrap }
                Text { width: parent.width; visible: root.mode === "import"; text: I18n.tr("Import a JSON array or an object with a commands array. Each entry needs a command; name, description and tags are optional."); wrapMode: Text.Wrap; color: Lithic.fg2; font.family: Look.sans; font.pixelSize: Math.round(12 * root.s) }
                Shape {
                    visible: root.mode !== ""; width: Math.min(content.width, 220 * root.s); height: 34 * root.s; s: root.s; fill: Look.plate; cutSize: Look.cutRow
                    Text { anchors.centerIn: parent; text: root.primaryAction ? root.primaryAction.label : ""; color: Lithic.fg; font.family: Look.sans; font.pixelSize: Math.round(13 * root.s) }
                    TapHandler { onTapped: root.submit() }
                }
            }
        }
    }
}
