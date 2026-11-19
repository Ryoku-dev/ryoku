pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Lithic
import Ryoku.Ui.Singletons
import "../../../ask/look"
import "shortcuts.js" as Shortcuts

Item {
    id: root

    required property real s
    property bool active: false
    property string query: ""
    property var args: ({})
    property real maximumHeight: 0
    property var memory: null
    property var source: null
    property string pageId: ""
    readonly property var pages: []
    readonly property string title: I18n.tr("Keys")
    readonly property string placeholder: I18n.tr("Filter by name or chord, like super+k")
    property bool capturing: false
    readonly property bool ownsText: root.capturing
    property string selectedId: ""
    property string captureNote: ""
    readonly property var categories: root.source ? root.source.categories : []
    readonly property var groups: Shortcuts.groups(root.categories, root.query, text => I18n.tr(text))
    readonly property var rows: root.groups.reduce((out, group) => out.concat(group.rows), [])
    readonly property int selectedIndex: root.rows.findIndex(row => row.id === root.selectedId)
    readonly property var selected: root.selectedIndex >= 0 ? root.rows[root.selectedIndex] : null
    readonly property string status: root.capturing ? (root.captureNote || I18n.tr("Press a shortcut to find it. Esc leaves this mode.")) : root.selected ? [I18n.tr(root.selected.bind.unhonored || ""), I18n.tr(root.selected.bind.hint || root.selected.bind.desc || "")].filter(Boolean).join(" · ") : root.source && !root.source.loaded ? I18n.tr("Reading your shortcuts") : I18n.tr("No matching shortcuts")
    readonly property var primaryAction: ({ label: I18n.tr("Rebind in Settings"), glyph: "tune", hint: "Enter" })
    readonly property var secondaryAction: ({ label: root.capturing ? I18n.tr("Stop finding keys") : I18n.tr("Find by pressing keys"), hint: "Ctrl+Enter" })
    readonly property var actions: []
    readonly property string actionNote: ""
    readonly property int columns: root.width / root.s >= 1178 ? 3 : 2
    readonly property real pad: Math.round(16 * root.s)
    readonly property real gutter: Math.round(22 * root.s)
    readonly property real columnWidth: Math.floor((root.width - 2 * root.pad - (root.columns - 1) * root.gutter) / root.columns)
    readonly property real rowHeight: Math.round(29 * root.s)
    readonly property real headHeight: Math.round(32 * root.s)
    readonly property var blocks: Shortcuts.layout(root.groups, root.columns, root.rowHeight, root.headHeight, Math.round(10 * root.s))

    signal closeRequested()
    signal queryRequested(string text)
    signal hud(string text)
    signal alert(string text)
    signal menuRequested()

    function activate() {
        Spawn.run(["ryoku-shell", "hub", "open", "shortcuts"]);
        root.closeRequested();
    }
    function activateSecondary() {
        root.capturing = !root.capturing;
        root.captureNote = "";
        if (root.capturing)
            root.takeFocus();
    }
    function takeFocus() { capture.forceActiveFocus(); }
    function handleEscape() {
        if (!root.capturing)
            return false;
        root.capturing = false;
        root.captureNote = "";
        return true;
    }
    function ensureVisible() {
        for (var i = 0; i < root.blocks.length; i++) {
            var block = root.blocks[i];
            if (root.selectedIndex < block.start || root.selectedIndex >= block.start + block.group.rows.length)
                continue;
            var y = block.y + root.headHeight + (root.selectedIndex - block.start) * root.rowHeight;
            if (y < sheet.contentY + root.headHeight)
                sheet.contentY = Math.max(0, y - root.headHeight);
            else if (y + root.rowHeight > sheet.contentY + sheet.height)
                sheet.contentY = Math.min(Math.max(0, sheet.contentHeight - sheet.height), y + root.rowHeight - sheet.height);
            return;
        }
    }
    function select(index) {
        if (!root.rows.length)
            return;
        root.selectedId = root.rows[Math.max(0, Math.min(root.rows.length - 1, index))].id;
        root.ensureVisible();
    }
    function move(delta) { root.select(root.selectedIndex + delta); }
    function page(delta) { root.move(delta * Math.max(1, Math.floor(sheet.height / root.rowHeight) - 1)); }
    function jumpSection(delta) {
        if (!root.groups.length)
            return;
        var index = root.selected ? root.groups.findIndex(group => group.category === root.selected.category) : 0;
        var group = root.groups[(index + delta + root.groups.length) % root.groups.length];
        root.selectedId = group.rows[0].id;
        root.ensureVisible();
    }
    function handleKey(event) {
        if (!root.active)
            return false;
        if (event.key === Qt.Key_Tab && !(event.modifiers & (Qt.ControlModifier | Qt.AltModifier | Qt.MetaModifier))) {
            root.activateSecondary();
            return true;
        }
        if (root.capturing)
            return root.captureKey(event);
        if (event.modifiers !== Qt.NoModifier)
            return false;
        if (event.key === Qt.Key_Home || event.key === Qt.Key_End) {
            root.select(event.key === Qt.Key_Home ? 0 : root.rows.length - 1);
            return true;
        }
        return false;
    }
    function captureKey(event) {
        if (event.key === Qt.Key_Escape)
            return false;
        if (event.isAutoRepeat)
            return true;
        var pressed = Shortcuts.eventChord(event.key, event.modifiers, event.text);
        if (!pressed)
            return true;
        var all = Shortcuts.groups(root.categories, "", text => I18n.tr(text)).reduce((out, group) => out.concat(group.rows), []);
        var found = Shortcuts.find(all, pressed);
        if (!found) {
            root.captureNote = I18n.tr("No shortcut uses %1. Try another, or Esc to leave.").arg(pressed);
            return true;
        }
        root.queryRequested("");
        root.selectedId = found;
        root.captureNote = I18n.tr("Found %1. Press another shortcut, or Esc to leave.").arg(pressed);
        Qt.callLater(root.ensureVisible);
        return true;
    }
    onRowsChanged: {
        if (!root.rows.some(row => row.id === root.selectedId) && root.rows.length)
            root.selectedId = root.rows[0].id;
        sheet.contentY = 0;
    }
    onPageIdChanged: {
        var c = Number(root.pageId.replace("category-", ""));
        var group = root.groups.find(group => group.category === c);
        if (group) {
            root.selectedId = group.rows[0].id;
            Qt.callLater(root.ensureVisible);
        }
    }
    onActiveChanged: if (!root.active) root.capturing = false
    onArgsChanged: if (root.args && root.args.shortcut) { root.selectedId = String(root.args.shortcut); Qt.callLater(root.ensureVisible); }

    Item {
        id: capture
        focus: root.capturing
    }
    Flickable {
        id: sheet
        anchors.fill: parent
        anchors.margins: Math.round(10 * root.s)
        clip: true
        contentWidth: width
        contentHeight: Math.max(height, root.blocks.reduce((h, block) => Math.max(h, block.y + block.height + root.pad), 0))
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollBar { policy: ScrollBar.AsNeeded }
        Repeater {
            model: root.blocks
            delegate: Item {
                id: block
                required property var modelData
                x: root.pad - Math.round(10 * root.s) + block.modelData.column * (root.columnWidth + root.gutter)
                y: block.modelData.y
                width: root.columnWidth
                height: block.modelData.height
                Shape {
                    visible: Look.adwaita
                    y: root.headHeight
                    width: parent.width
                    height: parent.height - y
                    s: root.s
                    cutSize: Look.cutCard
                    fill: Look.card
                    strokeColor: Look.cardStroke
                    strokeWidth: 1
                }
                Repeater {
                    model: block.modelData.group.rows
                    delegate: Item {
                        id: row
                        required property var modelData
                        required property int index
                        readonly property var bind: row.modelData.bind
                        readonly property bool chosen: root.selectedId === row.modelData.id
                        y: root.headHeight + row.index * root.rowHeight
                        width: block.width
                        height: root.rowHeight
                        visible: block.y + row.y >= sheet.contentY && block.y + row.y + row.height <= sheet.contentY + sheet.height
                        Shape {
                            anchors.fill: parent
                            s: root.s
                            cutSize: Look.cutRow
                            fill: row.chosen ? Look.plate : hover.hovered ? Look.hover : "transparent"
                            strokeColor: row.chosen ? Look.plateStroke : "transparent"
                            strokeWidth: row.chosen ? 1 : 0
                            Behavior on fill { ColorAnimation { duration: Look.dur(Look.plateMs) } }
                        }
                        Text {
                            x: Math.round(10 * root.s)
                            anchors.verticalCenter: parent.verticalCenter
                            width: Math.max(0, caps.x - x - Math.round(8 * root.s))
                            text: I18n.tr(row.bind.desc || "")
                            color: row.bind.unhonored ? Lithic.fg3 : row.chosen ? Look.plateInk : Lithic.fg
                            font.family: Look.sans
                            font.pixelSize: Math.round(13 * root.s)
                            elide: Text.ElideRight
                            renderType: Text.NativeRendering
                        }
                        Row {
                            id: caps
                            x: parent.width - width - Math.round(8 * root.s)
                            anchors.verticalCenter: parent.verticalCenter
                            spacing: Math.round(3 * root.s)
                            visible: !row.bind.unhonored
                            Repeater {
                                model: row.bind.keys || []
                                delegate: Shape {
                                    id: cap
                                    required property string modelData
                                    s: root.s
                                    cutSize: Look.cutToken
                                    width: capText.implicitWidth + Math.round(8 * root.s)
                                    height: Math.round(19 * root.s)
                                    fill: Look.cap
                                    strokeColor: Look.capStroke
                                    strokeWidth: 1
                                    Text {
                                        id: capText
                                        anchors.centerIn: parent
                                        text: I18n.tr(cap.modelData)
                                        color: row.chosen ? Look.plateInk : Lithic.fg2
                                        font.family: Look.monoMicro ? Look.mono : Look.sans
                                        font.pixelSize: Math.round(9 * root.s)
                                        renderType: Text.NativeRendering
                                    }
                                }
                            }
                            Text {
                                visible: (row.bind.taps || 0) > 1 || row.bind.whileHighlighting === true
                                text: row.bind.whileHighlighting ? I18n.tr("hold") : I18n.tr("×%1").arg(row.bind.taps || 1)
                                color: Lithic.fg3
                                font.family: Look.mono
                                font.pixelSize: Math.round(9 * root.s)
                            }
                        }
                        Text {
                            visible: !!row.bind.unhonored
                            anchors.right: parent.right
                            anchors.rightMargin: Math.round(10 * root.s)
                            anchors.verticalCenter: parent.verticalCenter
                            text: I18n.tr("Unavailable")
                            color: Lithic.fg3
                            font.family: Look.sans
                            font.pixelSize: Math.round(10.5 * root.s)
                        }
                        HoverHandler { id: hover; onHoveredChanged: if (hovered) root.selectedId = row.modelData.id }
                        TapHandler { onTapped: root.selectedId = row.modelData.id }
                    }
                }
                Rectangle {
                    z: 2
                    y: Math.max(0, Math.min(sheet.contentY - block.y, block.height - root.headHeight))
                    width: parent.width
                    height: root.headHeight
                    color: Look.frame
                    Micro {
                        id: name
                        x: Math.round(10 * root.s)
                        anchors.verticalCenter: parent.verticalCenter
                        width: Math.min(implicitWidth, parent.width - count.width - Math.round(38 * root.s))
                        s: root.s
                        text: I18n.tr(block.modelData.group.name)
                        color: Lithic.fg2
                        elide: Text.ElideRight
                    }
                    Rectangle {
                        visible: Look.lithic
                        x: name.x + name.width + Math.round(10 * root.s)
                        anchors.verticalCenter: parent.verticalCenter
                        width: Math.max(0, count.x - x - Math.round(10 * root.s))
                        height: 1
                        color: Lithic.vein
                    }
                    Micro {
                        id: count
                        anchors.right: parent.right
                        anchors.rightMargin: Math.round(10 * root.s)
                        anchors.verticalCenter: parent.verticalCenter
                        s: root.s
                        text: String(block.modelData.group.rows.length)
                    }
                }
            }
        }
    }
    Text {
        anchors.centerIn: parent
        visible: !root.rows.length
        text: root.source && !root.source.loaded ? I18n.tr("Reading your shortcuts") : I18n.tr("No matching shortcuts")
        color: Lithic.fg2
        font.family: Look.sans
        font.pixelSize: Math.round(14 * root.s)
    }
}
