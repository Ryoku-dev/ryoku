import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import "../desktop-dock"
import "../../schema/quick-settings.js" as Schema

// the panel's blocks in their order: show or hide each, move it, and choose
// the parts it shows; every change goes through the quick settings store
Slab {
    id: root

    required property Item host
    property string pageId: "quick-settings"
    // block id -> parts open, kept across the rebuild a move causes
    property var openParts: ({})

    readonly property var orderBind: ({ store: "quicksettings", key: "order" })
    readonly property var layoutBind: ({ store: "quicksettings", key: "layout" })
    readonly property var order: {
        var v = Changes.value(root.orderBind);
        return String(v || Schema.defaultOrder()).split(",").filter(id => !!Schema.block(id));
    }
    readonly property var restoreRow: Schema.row("restore-panel")
    // the same marks the rows show, so Restore is live exactly when something differs
    readonly property bool layoutChanged: {
        if (root.order.join(",") !== Schema.defaultOrder())
            return true;
        return Schema.rows.some(r => (r.block !== undefined) && Changes.isChanged(r));
    }

    function move(id, by) {
        var ids = root.order.slice();
        var from = ids.indexOf(id), to = from + by;
        if (from < 0 || to < 0 || to >= ids.length)
            return;
        ids.splice(from, 1);
        ids.splice(to, 0, id);
        var block = Schema.block(id);
        Changes.set(root.orderBind, ids.join(","), I18n.tr("Block order"), root.pageId,
                    by < 0 ? I18n.tr("Moved %1 up").arg(I18n.tr(block.label)) : I18n.tr("Moved %1 down").arg(I18n.tr(block.label)));
        Qt.callLater(root.refocus, "block-" + id);
    }

    function refocus(rowId) {
        var item = root.host.rowItems[rowId];
        if (item)
            item.forceActiveFocus(Qt.TabFocusReason);
    }

    // the row that last held the keyboard here: an undo of a move rebuilds every block,
    // which drops focus to the window, so the row gets it back once the list settles
    property string keptRow: ""
    function ownerRow(item) {
        for (var it = item; it && it !== root; it = it.parent)
            if (it.settingRow === it && it.rowId)
                return it.rowId;
        return "";
    }
    function giveBack() {
        var f = root.Window.activeFocusItem;
        if (root.keptRow !== "" && !(f && f.activeFocusOnTab))
            root.refocus(root.keptRow);
    }
    onOrderChanged: Qt.callLater(root.giveBack)
    Connections {
        target: root.Window
        function onActiveFocusItemChanged() {
            var f = root.Window.activeFocusItem;
            // focus passing through the window mid-rebuild keeps the row
            if (f && f.activeFocusOnTab)
                root.keptRow = root.ownerRow(f);
        }
    }

    function setOpen(id, open) {
        var m = Object.assign({}, root.openParts);
        m[id] = open;
        root.openParts = m;
    }

    // search landed on a part: open its block first
    function reveal(blockId) {
        if (root.openParts[blockId] === true)
            return false;
        root.setOpen(blockId, true);
        var d = blockRepeater.itemAt(root.order.indexOf(blockId));
        if (d)
            d.openFold();
        return true;
    }

    function restore() {
        Changes.set(root.layoutBind, Schema.defaultLayout(), I18n.tr("Quick settings blocks"), root.pageId,
                    I18n.tr("Quick settings blocks restored"));
    }

    function shownParts(block) {
        var n = 0;
        for (var i = 0; i < block.parts.length; i++)
            if (Changes.value({ store: "quicksettings", key: "part:" + block.parts[i].id }) !== false)
                n++;
        return n;
    }

    // one block or part on or off, said in the panel's words
    function show(row, on) {
        var name = I18n.tr(row.label);
        Changes.set(row.bind, on, row.label, root.pageId,
                    on ? I18n.tr("%1 shows in Quick settings").arg(name) : I18n.tr("%1 is hidden in Quick settings").arg(name));
    }

    Column {
        width: parent.width

        Repeater {
            id: blockRepeater
            model: root.order

            delegate: Column {
                id: entry
                required property string modelData
                required property int index
                readonly property var block: Schema.block(entry.modelData)
                readonly property var row: Schema.row("block-" + entry.modelData)
                readonly property bool shown: Changes.value(entry.row.bind) !== false
                readonly property int partsShown: root.shownParts(entry.block)

                function openFold() { fold.open = true; }

                width: parent.width

                OwnRow {
                    id: blockRow
                    host: root.host
                    rowId: entry.row.id
                    first: entry.index === 0
                    title: I18n.tr(entry.block.label)
                    detail: {
                        var lead = I18n.tr(entry.block.detail);
                        var total = entry.block.parts.length;
                        if (total === 0 || entry.partsShown === total)
                            return lead;
                        return I18n.tr("%1. Shows %2 of its %3 parts.").arg(lead).arg(entry.partsShown).arg(total);
                    }
                    activatable: true
                    changed: Changes.isChanged(entry.row)
                    onActivated: root.show(entry.row, !entry.shown)
                    onResetRequested: root.show(entry.row, Changes.defaultOf(entry.row) === true)
                    Keys.onPressed: event => {
                        if (!(event.modifiers & Qt.AltModifier))
                            return;
                        if (event.key === Qt.Key_Up) {
                            root.move(entry.modelData, -1);
                            event.accepted = true;
                        } else if (event.key === Qt.Key_Down) {
                            root.move(entry.modelData, 1);
                            event.accepted = true;
                        }
                    }

                    IconPlaque {
                        anchors.verticalCenter: parent.verticalCenter
                        kind: "quiet"
                        small: true
                        glyph: "arrow_upward"
                        enabled: entry.index > 0
                        accessibleName: I18n.tr("Move %1 up").arg(I18n.tr(entry.block.label))
                        onClicked: root.move(entry.modelData, -1)
                    }
                    IconPlaque {
                        anchors.verticalCenter: parent.verticalCenter
                        kind: "quiet"
                        small: true
                        glyph: "arrow_downward"
                        enabled: entry.index < root.order.length - 1
                        accessibleName: I18n.tr("Move %1 down").arg(I18n.tr(entry.block.label))
                        onClicked: root.move(entry.modelData, 1)
                    }
                    Toggle {
                        anchors.verticalCenter: parent.verticalCenter
                        checked: entry.shown
                        Accessible.name: I18n.tr("Show %1").arg(I18n.tr(entry.block.label))
                        onToggled: root.show(entry.row, checked)
                    }
                }

                RowNote {
                    width: parent.width
                    text: Changes.error(entry.row.bind)
                    onRetry: Changes.retry(entry.row.bind)
                }

                Disclosure {
                    id: fold
                    visible: entry.block.parts.length > 0
                    width: parent.width
                    topPadding: LithicTokens.space.s1
                    text: I18n.tr("What it shows")
                    accessibleName: I18n.tr("What %1 shows").arg(I18n.tr(entry.block.label))
                    open: false
                    Component.onCompleted: fold.open = root.openParts[entry.modelData] === true
                    onToggled: open => root.setOpen(entry.modelData, open)

                    Column {
                        x: LithicTokens.space.s5
                        width: fold.width - LithicTokens.space.s5

                        Repeater {
                            model: entry.block.parts
                            delegate: Column {
                                id: partEntry
                                required property var modelData
                                required property int index
                                readonly property var row: Schema.row("part-" + partEntry.modelData.id)
                                readonly property bool shown: Changes.value(partEntry.row.bind) !== false
                                width: parent ? parent.width : 0

                                OwnRow {
                                    host: root.host
                                    rowId: partEntry.row.id
                                    first: partEntry.index === 0
                                    title: I18n.tr(partEntry.modelData.label)
                                    activatable: true
                                    changed: Changes.isChanged(partEntry.row)
                                    onActivated: root.show(partEntry.row, !partEntry.shown)
                                    onResetRequested: root.show(partEntry.row, true)

                                    Toggle {
                                        anchors.verticalCenter: parent.verticalCenter
                                        checked: partEntry.shown
                                        Accessible.name: I18n.tr(partEntry.modelData.label)
                                        onToggled: root.show(partEntry.row, checked)
                                    }
                                }

                                RowNote {
                                    width: parent.width
                                    text: Changes.error(partEntry.row.bind)
                                    onRetry: Changes.retry(partEntry.row.bind)
                                }
                            }
                        }
                    }
                }
            }
        }

        RowNote {
            width: parent.width
            text: Changes.error(root.orderBind)
            onRetry: Changes.retry(root.orderBind)
        }

        OwnRow {
            host: root.host
            rowId: "restore-panel"
            title: I18n.tr(root.restoreRow.label)
            detail: I18n.tr(root.restoreRow.detail)

            Plaque {
                id: restorePlaque
                anchors.verticalCenter: parent.verticalCenter
                kind: "line"
                small: true
                glyph: "restart_alt"
                text: I18n.tr("Restore")
                accessibleName: I18n.tr("Restore the panel")
                enabled: root.layoutChanged
                // the plaque turns off once the panel is restored, and every block is rebuilt in
                // its first order: the keyboard goes to the block just above, the last one
                onClicked: {
                    var keyboard = restorePlaque.visualFocus;
                    root.restore();
                    if (keyboard)
                        Qt.callLater(root.refocus, "block-" + Schema.blocks[Schema.blocks.length - 1].id);
                }
            }
        }

        RowNote {
            width: parent.width
            text: Changes.error(root.layoutBind)
            onRetry: Changes.retry(root.layoutBind)
        }
    }
}
