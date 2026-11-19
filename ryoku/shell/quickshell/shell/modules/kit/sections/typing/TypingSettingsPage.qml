pragma ComponentBehavior: Bound
import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../../ask/bar"
import "../../../ask/look"
import "../../parts"
import "TypingFingers.js" as Fingers

FocusScope {
    id: root
    required property real s
    required property var store
    required property var languages
    property string query: ""
    property bool confirmClear: false
    property bool editorOpen: false
    readonly property var current: list.current
    readonly property real maximumHeight: height

    function option(id, title, choices, labels) {
        const value = root.store[id];
        const at = choices ? choices.indexOf(value) : -1;
        return {id: id, size: "one", kind: choices ? "step" : "toggle", title: title,
            list: choices || [], raw: value, on: value === true,
            value: labels?.[at] || String(value), first: at === 0, last: choices ? at === choices.length - 1 : false};
    }
    readonly property var groups: [
        {id: "general", title: I18n.tr("General"), rows: [
            option("language", I18n.tr("Language"), languages.catalogue.filter(v => !v.id.endsWith("_extended")).map(v => v.id), languages.catalogue.filter(v => !v.id.endsWith("_extended")).map(v => I18n.tr(v.label || v.id))),
            option("wordlistSize", I18n.tr("Word list"), ["standard", "extended"], [I18n.tr("Standard"), I18n.tr("Extended")]),
            option("zenGuided", I18n.tr("Guided zen")),
            option("finishOnLastWord", I18n.tr("Finish on the last word")),
            option("fontSize", I18n.tr("Font size"), [18, 22, 26, 30, 34]),
            option("visibleLines", I18n.tr("Visible lines"), [2, 3, 4, 5]),
            option("caretStyle", I18n.tr("Caret"), ["line", "block", "underline", "off"], [I18n.tr("Line"), I18n.tr("Block"), I18n.tr("Underline"), I18n.tr("Off")]),
            option("smoothCaret", I18n.tr("Smooth caret")),
            option("highlightCurrentWord", I18n.tr("Highlight current word")),
            option("blindMode", I18n.tr("Blind mode")),
            option("showLiveWpm", I18n.tr("Live speed")),
            option("showLiveAccuracy", I18n.tr("Live accuracy")),
            option("quickRestart", I18n.tr("Quick restart"))
        ]},
        {id: "keyboard", title: I18n.tr("Keyboard"), rows: [
            option("keyboardEnabled", I18n.tr("Show keyboard")),
            option("keyboardLayout", I18n.tr("Layout"), ["qwerty", "qwertz", "azerty", "dvorak", "colemak"]),
            option("highlightNextKey", I18n.tr("Highlight next key")),
            option("fingerGuide", I18n.tr("Finger guide")),
            {id: "fingers", size: "one", kind: "text", title: I18n.tr("Finger assignments"), value: I18n.tr("Edit")}
        ]},
        {id: "sounds", title: I18n.tr("Sounds"), rows: [
            option("soundsEnabled", I18n.tr("Typing sounds")),
            option("soundTheme", I18n.tr("Key sound"), ["click1", "click2", "click3", "click4", "click5", "click6", "click7"]),
            option("errorSound", I18n.tr("Sound on mistakes")),
            option("errorTheme", I18n.tr("Error sound"), ["error1", "error2", "error3", "error4"]),
            option("soundVolume", I18n.tr("Volume"), [0, 10, 25, 40, 55, 70, 85, 100])
        ]},
        {id: "history", title: I18n.tr("History"), rows: [
            option("historyEnabled", I18n.tr("Save results locally")),
            option("maxEntries", I18n.tr("Saved tests"), [25, 50, 100, 250, 500]),
            {id: "clear", size: "one", kind: "action", title: root.confirmClear ? I18n.tr("Delete all saved tests?") : I18n.tr("Clear history"), value: root.confirmClear ? I18n.tr("Enter confirms, Esc cancels") : I18n.tr("History stays on this computer")}
        ]}
    ]
    readonly property var sections: {
        const terms = root.query.trim().toLowerCase().split(/\s+/).filter(Boolean);
        return root.groups.map(group => ({id: group.id, title: group.title, rows: group.rows.filter(row => row.id === "clear" && root.confirmClear || terms.every(term => (group.title + " " + row.title + " " + row.value).toLowerCase().includes(term)))})).filter(group => group.rows.length);
    }
    function move(delta) { if (!editorOpen) list.move(delta); }
    function jumpSection(delta) { if (!editorOpen) list.jumpSection(delta); }
    function page(delta) { if (!editorOpen) list.page(delta); }
    function takeFocus() { if (editorOpen) forceActiveFocus(); }
    function requestClear() {
        confirmClear = true;
        const at = list.layout.rows.findIndex(entry => entry.row.id === "clear");
        if (at >= 0) list.select(at);
    }
    function step(delta) {
        const row = current;
        if (!row || editorOpen) return;
        if (row.kind === "toggle") store[row.id] = delta > 0;
        else if (row.kind === "step" && row.list.length) store[row.id] = row.list[Math.max(0, Math.min(row.list.length - 1, row.list.indexOf(row.raw) + delta))];
    }
    function activate() {
        const row = current;
        if (!row) return;
        if (row.kind === "toggle") store[row.id] = !row.on;
        else if (row.kind === "step" && row.list.length) store[row.id] = row.list[(row.list.indexOf(row.raw) + 1) % row.list.length];
        else if (row.id === "fingers") { editorOpen = true; keyboard.selected = 0; takeFocus(); }
        else if (row.id === "clear") {
            if (confirmClear) { store.clearHistory(); confirmClear = false; }
            else confirmClear = true;
        }
    }
    function handleEscape() {
        if (editorOpen) { editorOpen = false; return true; }
        if (confirmClear) { confirmClear = false; return true; }
        return false;
    }
    function handleKey(event) {
        if (!editorOpen) return false;
        let delta = event.key === Qt.Key_Left ? -1 : event.key === Qt.Key_Right || event.key === Qt.Key_Tab ? 1 : event.key === Qt.Key_Up ? -14 : event.key === Qt.Key_Down ? 14 : 0;
        if (delta) keyboard.selected = (keyboard.selected + delta + keyboard.board.keys.length) % keyboard.board.keys.length;
        else if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter || event.key === Qt.Key_Space) {
            const fingers = [-5, -4, -3, -2, -1, 6, 1, 2, 3, 4, 5];
            const current = keyboard.assignments[keyboard.selected];
            store.fingerAssignments = Fingers.saveAssignment(store.fingerAssignments, keyboard.boardId, keyboard.board.keys[keyboard.selected], fingers[(fingers.indexOf(current) + 1) % fingers.length]);
        } else return false;
        return true;
    }
    ResultList {
        id: list
        anchors.fill: parent
        s: root.s
        sections: root.sections
        maximumHeight: root.maximumHeight
        rowDelegate: OptionRow {}
        onActivated: root.activate()
    }
    Line { anchors.centerIn: parent; s: root.s; visible: !list.count; text: I18n.tr("No matching settings"); ink: Lithic.fg2 }
    Shape {
        anchors.left: parent.left; anchors.right: parent.right; anchors.bottom: parent.bottom
        anchors.margins: 10 * root.s
        height: Math.min(parent.height, 240 * root.s)
        visible: root.editorOpen
        s: root.s; cutSize: Look.cutCard
        fill: Look.popover; strokeColor: Look.popoverStroke; strokeWidth: 1
        Column {
            anchors.fill: parent; anchors.margins: 18 * root.s; spacing: 12 * root.s
            Line { width: parent.width; s: root.s; size: 15; text: I18n.tr("Finger assignments") }
            Line { width: parent.width; s: root.s; size: 12; ink: Lithic.fg2; text: I18n.tr("Arrows choose a key, Enter changes its finger, Esc returns") }
            TypingKeyboardPreview { id: keyboard; width: parent.width; height: Math.max(80 * root.s, parent.height - 72 * root.s); s: root.s; store: root.store; editing: true }
        }
    }
}
