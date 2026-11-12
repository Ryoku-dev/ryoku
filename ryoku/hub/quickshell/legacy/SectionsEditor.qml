pragma ComponentBehavior: Bound
import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons

// Orders a launcher's section chips and shows or hides each one. A hidden
// section loses its chip only; its apps stay under All and in search.
// Emits the whole new order and hidden list for the caller's draft.
Item {
    id: root

    property var order: []
    property var hidden: []
    signal edited(var order, var hidden)

    // { id, label } rows the launcher files its apps under, from its manifest.
    property var known: []
    readonly property var rows: {
        var labels = {};
        for (var i = 0; i < root.known.length; i++)
            labels[root.known[i].id] = root.known[i].label;
        var seq = [];
        var want = root.order || [];
        for (var j = 0; j < want.length; j++)
            if (labels[want[j]] !== undefined && seq.indexOf(want[j]) < 0)
                seq.push(want[j]);
        for (var k = 0; k < root.known.length; k++)
            if (seq.indexOf(root.known[k].id) < 0)
                seq.push(root.known[k].id);
        return seq.map(function (id) { return { id: id, label: labels[id] }; });
    }

    readonly property int rowH: 38
    implicitHeight: root.rows.length * root.rowH + Tokens.s2
    height: implicitHeight

    function ids() { return root.rows.map(function (r) { return r.id; }); }
    function move(index, by) {
        var seq = root.ids();
        var to = index + by;
        if (to < 0 || to >= seq.length)
            return;
        var tmp = seq[index];
        seq[index] = seq[to];
        seq[to] = tmp;
        root.edited(seq, root.hidden || []);
    }
    function show(id, on) {
        var next = [];
        var h = root.hidden || [];
        for (var i = 0; i < h.length; i++)
            if (h[i] !== id)
                next.push(h[i]);
        if (!on)
            next.push(id);
        root.edited(root.ids(), next);
    }

    Column {
        anchors { left: parent.left; right: parent.right; top: parent.top; topMargin: Tokens.s1 }
        Repeater {
            model: root.rows
            delegate: Item {
                id: row
                required property var modelData
                required property int index
                readonly property bool shown: (root.hidden || []).indexOf(row.modelData.id) < 0
                width: root.width
                height: root.rowH

                Rectangle {
                    visible: row.index > 0
                    anchors { left: parent.left; right: parent.right; top: parent.top }
                    anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s4
                    height: 1
                    color: Tokens.lineSoft
                }
                Text {
                    anchors { left: parent.left; leftMargin: Tokens.s4; verticalCenter: parent.verticalCenter }
                    text: I18n.tr(row.modelData.label)
                    color: row.shown ? Tokens.ink : Tokens.inkFaint
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fBody
                }
                Row {
                    anchors { right: parent.right; rightMargin: Tokens.s4; verticalCenter: parent.verticalCenter }
                    spacing: Tokens.s2
                    IconBtn {
                        glyph: "\u2191"
                        armed: row.index > 0
                        onAct: root.move(row.index, -1)
                    }
                    IconBtn {
                        glyph: "\u2193"
                        armed: row.index < root.rows.length - 1
                        onAct: root.move(row.index, 1)
                    }
                    Sw {
                        anchors.verticalCenter: parent.verticalCenter
                        on: row.shown
                        onToggled: (v) => root.show(row.modelData.id, v)
                    }
                }
            }
        }
    }
}
