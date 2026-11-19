pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Lithic
import Ryoku.Ui.Singletons
import "../look"
import "../lib/root.js" as RootLogic

// Ctrl+K: every action for what is selected, in one filterable list that rises
// over the foot of the bar. Typing filters, Enter runs, Esc or Ctrl+K puts it away.
FocusScope {
    id: root

    required property real s
    // [{id, label, glyph, hint, section, destructive}]
    property var actions: []
    // a line or two about what is selected, over the actions (a failure's detail)
    property string note: ""
    property bool open: false
    property bool animate: true
    property real maximumHeight: 320 * s
    readonly property real noteHeight: root.note.length > 0 ? Math.ceil(noteText.implicitHeight + 20 * root.s) : 0

    readonly property var sections: {
        const out = [];
        const index = {};
        const list = root.actions || [];
        const text = filter.text.trim();
        for (var i = 0; i < list.length; i++) {
            const action = list[i];
            if (!action || (text.length > 0 && RootLogic.matchScore(text, action.label) === 0))
                continue;
            const name = String(action.section || "");
            if (index[name] === undefined) {
                index[name] = out.length;
                out.push({ id: "s" + out.length, title: name, rows: [] });
            }
            out[index[name]].rows.push({
                id: String(action.id), size: "menu", glyph: String(action.glyph || ""),
                glyphTone: action.destructive === true ? "error" : "",
                title: String(action.label || ""), hint: String(action.hint || ""),
                tone: action.destructive === true ? "error" : ""
            });
        }
        return out;
    }

    signal picked(string id)
    signal dismissed()

    width: Math.round(330 * root.s)
    height: Math.min(root.maximumHeight, root.noteHeight + list.implicitHeight + entry.height + (list.count === 0 ? empty.height : 0))
    visible: root.rise > 0.001
    opacity: Math.min(1, root.rise * 1.8)
    scale: Look.fluent || Look.material ? 0.96 + 0.04 * root.rise : 1
    transformOrigin: Item.BottomRight

    property real rise: root.open ? 1 : 0
    Behavior on rise {
        enabled: root.animate
        NumberAnimation {
            duration: Look.dur(root.open ? Math.round(Look.viewMs * 0.8) : 110)
            easing.type: Easing.BezierSpline
            easing.bezierCurve: root.open ? Look.arrive : Look.leave
        }
    }
    transform: Translate { y: Math.round(8 * root.s) * (1 - Math.min(1, root.rise)) }

    onOpenChanged: {
        if (!root.open)
            return;
        filter.text = "";
        list.selectFirst();
        filter.forceActiveFocus();
    }

    function runCurrent() {
        if (list.current)
            root.picked(String(list.current.id));
    }

    RectangularShadow {
        anchors.fill: plane
        offset.y: Math.round(6 * root.s)
        blur: Math.round(22 * root.s)
        radius: Look.cornerAt(Look.cutMenu, plane.width, plane.height, root.s)
        color: Look.shadow
        opacity: root.opacity * 0.8
    }

    Shape {
        id: plane
        anchors.fill: parent
        s: root.s
        cutSize: Look.cutMenu
        fill: Look.popover
        strokeColor: Look.popoverStroke
        strokeWidth: Qt.colorEqual(Look.popoverStroke, "transparent") ? 0 : 1
    }

    MouseArea {
        anchors.fill: parent
        acceptedButtons: Qt.AllButtons
        onPressed: mouse => mouse.accepted = true
    }

    Text {
        id: noteText
        x: Math.round(14 * root.s)
        y: Math.round(11 * root.s)
        width: parent.width - x * 2
        visible: root.note.length > 0
        text: root.note
        textFormat: Text.PlainText
        color: Lithic.fg2
        font.family: Look.sans
        font.pixelSize: Math.round(12 * root.s)
        wrapMode: Text.Wrap
        maximumLineCount: 4
        elide: Text.ElideRight
        renderType: Text.NativeRendering
    }

    ResultList {
        id: list
        anchors.left: parent.left
        anchors.right: parent.right
        y: root.noteHeight
        height: implicitHeight
        s: root.s
        sections: root.sections
        maximumHeight: Math.max(0, root.maximumHeight - entry.height - root.noteHeight)
        onActivated: row => root.picked(String(row.id))
    }

    Text {
        id: empty
        anchors.left: parent.left
        anchors.right: parent.right
        y: root.noteHeight
        height: Math.round(44 * root.s)
        visible: list.count === 0
        horizontalAlignment: Text.AlignHCenter
        verticalAlignment: Text.AlignVCenter
        text: I18n.tr("No action matches")
        color: Lithic.fg3
        font.family: Look.sans
        font.pixelSize: Math.round(13 * root.s)
        renderType: Text.NativeRendering
    }

    Item {
        id: entry
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        height: Math.round(38 * root.s)

        Rectangle {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            height: 1
            color: Qt.colorEqual(Look.rule, "transparent") ? Lithic.alpha(Lithic.fg, 0.1) : Look.rule
        }
        TextInput {
            id: filter
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.leftMargin: Math.round(14 * root.s)
            anchors.rightMargin: Math.round(14 * root.s)
            anchors.verticalCenter: parent.verticalCenter
            clip: true
            color: Lithic.fg
            selectionColor: Lithic.sel
            selectedTextColor: Lithic.onSel
            font.family: Look.sans
            font.pixelSize: Math.round(13 * root.s)
            renderType: Text.NativeRendering
            cursorDelegate: Rectangle {
                width: Math.max(1, Math.round(1.5 * root.s))
                color: Lithic.sel
                visible: filter.cursorVisible
            }
            onTextEdited: list.selectFirst()
            Keys.onPressed: event => {
                const ctrl = (event.modifiers & Qt.ControlModifier) !== 0;
                if (event.key === Qt.Key_Escape || (ctrl && event.key === Qt.Key_K)) {
                    root.dismissed();
                } else if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter) {
                    root.runCurrent();
                } else if (event.key === Qt.Key_Down || (event.key === Qt.Key_Tab && !ctrl)) {
                    list.move(1);
                } else if (event.key === Qt.Key_Up || event.key === Qt.Key_Backtab) {
                    list.move(-1);
                } else if (event.key === Qt.Key_PageDown) {
                    list.page(1);
                } else if (event.key === Qt.Key_PageUp) {
                    list.page(-1);
                } else {
                    return;
                }
                event.accepted = true;
            }

            Text {
                anchors.fill: parent
                verticalAlignment: Text.AlignVCenter
                visible: filter.text.length === 0
                text: I18n.tr("Filter actions")
                color: Lithic.fg3
                font: filter.font
                renderType: Text.NativeRendering
            }
        }
    }
}
