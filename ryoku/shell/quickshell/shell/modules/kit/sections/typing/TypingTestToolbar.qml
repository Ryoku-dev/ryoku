pragma ComponentBehavior: Bound
import QtQuick
import Ryoku.Ui.Singletons
import "../../../ask/look"
Flow {
    id: root
    required property real s
    required property var store
    property bool locked: false
    signal refocus()
    spacing: 7 * s
    Repeater {
        model: [{id: "time", label: I18n.tr("Time")}, {id: "words", label: I18n.tr("Words")}, {id: "zen", label: I18n.tr("Zen")}]
        delegate: TypingButton { required property var modelData; s: root.s; text: modelData.label; chosen: root.store.mode === modelData.id; enabled: !root.locked; opacity: enabled ? 1 : 0.55; onClicked: { root.store.mode = modelData.id; root.refocus(); } }
    }
    Item { width: 10 * root.s; height: 28 * root.s }
    Repeater {
        model: root.store.mode === "time" ? [15, 30, 60, 120] : root.store.mode === "words" ? [10, 25, 50, 100] : []
        delegate: TypingButton { required property var modelData; s: root.s; text: String(modelData); chosen: Number(modelData) === (root.store.mode === "time" ? root.store.timeLimit : root.store.wordLimit); enabled: !root.locked; onClicked: { if (root.store.mode === "time") root.store.timeLimit = modelData; else root.store.wordLimit = modelData; root.refocus(); } }
    }
    Item { width: 10 * root.s; height: 28 * root.s }
    TypingButton { s: root.s; text: I18n.tr("Punctuation"); chosen: root.store.punctuation; enabled: !root.locked; onClicked: { root.store.punctuation = !root.store.punctuation; root.refocus(); } }
    TypingButton { s: root.s; text: I18n.tr("Numbers"); chosen: root.store.numbers; enabled: !root.locked; onClicked: { root.store.numbers = !root.store.numbers; root.refocus(); } }
}
