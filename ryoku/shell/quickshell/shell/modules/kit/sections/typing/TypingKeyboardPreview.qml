pragma ComponentBehavior: Bound
import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../../ask/look"
import "TypingFingers.js" as Fingers
Item {
    id: root
    required property real s
    required property var store
    property string nextChar: ""
    property string pressedChar: ""
    property bool editing: false
    readonly property bool guiding: editing || store.fingerGuide
    signal requestInputFocus()
    TypingKeyboardLayouts { id: layouts }
    readonly property var board: Fingers.classicBoard(layouts.rowsFor(store.keyboardLayout), store.keyboardLayout)
    readonly property string boardId: "classic:" + store.keyboardLayout
    readonly property var assignments: Fingers.assignments(board.fingers, board.keys, boardId, store.fingerAssignments)
    readonly property var targets: store.highlightNextKey ? Fingers.targets(board.entries, nextChar) : []
    readonly property var pressed: Fingers.targets(board.entries, pressedChar)
    readonly property real unit: Math.min(25 * s, (width - 4 * s * board.width) / board.width, Math.max(8 * s, (height - 26 * s) / board.height - 4 * s))
    property int selected: -1
    Item {
        width: root.board.width * (root.unit + 4 * root.s); height: root.board.height * (root.unit + 4 * root.s)
        anchors.horizontalCenter: parent.horizontalCenter
        Repeater {
            model: root.board.keys
            delegate: Item {
                id: key
                required property var modelData
                required property int index
                x: modelData.x * (root.unit + 4 * root.s); y: modelData.y * (root.unit + 4 * root.s)
                width: modelData.w * root.unit + (modelData.w - 1) * 4 * root.s
                height: modelData.h * root.unit + (modelData.h - 1) * 4 * root.s
                Shape { anchors.fill: parent; cut: Look.cutToken * root.s; fill: root.pressed.includes(key.index) ? Lithic.sel : root.targets.includes(key.index) ? Look.plate : root.guiding ? Lithic.alpha(Lithic.fg, 0.03 + Math.abs(root.assignments[key.index] || 0) * 0.025) : Look.well; strokeColor: root.targets.includes(key.index) || root.selected === key.index ? Lithic.fg2 : Look.cardStroke; strokeWidth: 1 }
                Text { anchors.centerIn: parent; text: root.board.entries[key.index].label; color: root.pressed.includes(key.index) ? Lithic.onSel : Lithic.fg2; font.family: Look.mono; font.pixelSize: 9 * root.s }
                TapHandler { enabled: root.guiding; onTapped: root.selected = key.index }
            }
        }
    }
    Row {
        anchors.bottom: parent.bottom; anchors.horizontalCenter: parent.horizontalCenter
        visible: root.selected >= 0 && root.guiding
        spacing: 3 * root.s
        Repeater {
            model: [-5, -4, -3, -2, -1, 6, 1, 2, 3, 4, 5]
            delegate: TypingButton { required property int modelData; s: root.s; text: String(modelData); chosen: root.selected >= 0 && root.assignments[root.selected] === modelData; onClicked: { root.store.fingerAssignments = Fingers.saveAssignment(root.store.fingerAssignments, root.boardId, root.board.keys[root.selected], modelData); if (!root.editing) root.selected = -1; root.requestInputFocus(); } }
        }
    }
}
