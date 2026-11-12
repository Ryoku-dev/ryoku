import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"

// the launcher's sections in order, each shown or hidden; a hidden section keeps its apps under All and in search
Column {
    id: root

    // { id, label } as the launcher's own settings name them
    property var known: []
    readonly property var orderBind: ({ store: "launcher", key: "sectionOrder" })
    readonly property var hiddenBind: ({ store: "launcher", key: "hiddenSections" })

    function list(bind) {
        var v = Changes.value(bind);
        return v && v.length !== undefined ? Array.prototype.slice.call(v).map(String) : [];
    }

    readonly property var hidden: root.list(root.hiddenBind)
    readonly property var rows: {
        var labels = {};
        root.known.forEach(k => labels[k.id] = k.label);
        var seq = root.list(root.orderBind).filter((id, i, all) => labels[id] !== undefined && all.indexOf(id) === i);
        root.known.forEach(k => {
            if (seq.indexOf(k.id) < 0)
                seq.push(k.id);
        });
        return seq.map(id => ({ id: id, label: labels[id] }));
    }

    // which section's control holds keyboard focus: a move, a show or hide, or Undo redraws the rows,
    // and the control of that section takes focus back, an arrow that no longer applies handing it to the other
    property string focusId: ""
    property string focusPart: ""
    property bool focusMarked: false
    function noteFocus(id, part, item) {
        root.focusId = id;
        root.focusPart = part;
        root.focusMarked = item.visualFocus;
    }
    // focus that left for another control lets go; focus that fell out with a redrawn row does not
    function checkFocus() {
        var now = root.Window.activeFocusItem;
        for (var p = now; p; p = p.parent)
            if (p === root)
                return;
        if (now && now !== root.Window.contentItem && !root.holds(now))
            root.focusId = "";
    }
    function holds(item) {
        for (var p = root.parent; p; p = p.parent)
            if (p === item)
                return true;
        return false;
    }

    // the page holding the list; a redraw can leave it scrolled with the control that took focus
    // back out of view, so that control comes into view once the page has stopped moving
    property Item host: null
    Timer {
        id: inView
        property Item item: null
        interval: Lithic.duration(LithicTokens.motion.navigate) + 50
        onTriggered: {
            if (root.host && inView.item && inView.item.activeFocus)
                root.host.keepInView(inView.item);
        }
    }

    function move(index, by) {
        var seq = root.rows.map(r => r.id);
        var to = index + by;
        if (to < 0 || to >= seq.length)
            return;
        var moved = seq.splice(index, 1)[0];
        seq.splice(to, 0, moved);
        Changes.set(root.orderBind, seq, I18n.tr("Section order"), "launcher");
    }

    function show(id, on) {
        var next = root.hidden.filter(h => h !== id);
        if (!on)
            next.push(id);
        Changes.set(root.hiddenBind, next, I18n.tr("Hidden sections"), "launcher");
    }

    Repeater {
        id: rowsRepeater
        model: root.rows
        delegate: SettingRow {
            id: row
            required property var modelData
            required property int index
            readonly property bool shown: root.hidden.indexOf(modelData.id) < 0
            width: root.width
            first: index === 0
            title: modelData.label
            activatable: true
            // the part that held focus before the rows were redrawn; the same arrow when it still applies, else the other
            function focusPart(part, reason) {
                var target = row;
                if (part === "up" || part === "down") {
                    var same = part === "up" ? up : down, other = part === "up" ? down : up;
                    target = same.enabled ? same : other;
                } else if (part === "show") {
                    target = toggle;
                }
                target.forceActiveFocus(reason);
                inView.item = target;
                inView.restart();
            }
            function watch(item, part) {
                // a row holds focus for its controls too; only focus on the row itself counts as the row's
                if (item.activeFocus && (part !== "row" || row.Window.activeFocusItem === row))
                    root.noteFocus(row.modelData.id, part, item);
                else
                    Qt.callLater(root.checkFocus);
            }
            onActiveFocusChanged: row.watch(row, "row")
            // the row joins the scene after it completes, so focus waits a turn
            Component.onCompleted: {
                if (root.focusId !== row.modelData.id)
                    return;
                var part = root.focusPart, reason = root.focusMarked ? Qt.TabFocusReason : Qt.OtherFocusReason;
                Qt.callLater(() => row.focusPart(part, reason));
            }

            IconPlaque {
                id: up
                anchors.verticalCenter: parent.verticalCenter
                kind: "quiet"
                small: true
                glyph: "arrow_upward"
                enabled: row.index > 0
                accessibleName: I18n.tr("Move %1 up").arg(row.modelData.label)
                onClicked: root.move(row.index, -1)
                onActiveFocusChanged: row.watch(up, "up")
            }
            IconPlaque {
                id: down
                anchors.verticalCenter: parent.verticalCenter
                kind: "quiet"
                small: true
                glyph: "arrow_downward"
                enabled: row.index < root.rows.length - 1
                accessibleName: I18n.tr("Move %1 down").arg(row.modelData.label)
                onClicked: root.move(row.index, 1)
                onActiveFocusChanged: row.watch(down, "down")
            }
            Toggle {
                id: toggle
                anchors.verticalCenter: parent.verticalCenter
                checked: row.shown
                Accessible.name: I18n.tr("Show %1").arg(row.modelData.label)
                onToggled: root.show(row.modelData.id, checked)
                onActiveFocusChanged: row.watch(toggle, "show")
            }
        }
    }
}
