pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../services/lib/markdown.js" as Md

Column {
    id: root
    property real s: 1
    property string text: ""
    property var codes: []
    property bool done: false
    property bool busy: false
    property var keyTarget: null
    readonly property bool typing: visible && editor.activeFocus
    signal edited(string text)
    signal copyRequested(string text)
    signal openRequested(string url)
    signal readRequested()
    spacing: 13 * s
    Item {
        width: parent.width
        height: Math.max(60 * root.s, editor.implicitHeight + 20 * root.s)
        visible: root.text.length > 0
        CutRect { anchors.fill: parent; cut: 7; fill: Lithic.field; strokeColor: editor.activeFocus ? Lithic.fg2 : "transparent"; strokeWidth: editor.activeFocus ? 1 : 0 }
        TextEdit {
            id: editor
            x: 13 * root.s; y: 10 * root.s; width: parent.width - 26 * root.s
            text: root.text; color: Lithic.fg
            font.family: Lithic.type.body.family; font.pixelSize: 14 * root.s
            wrapMode: TextEdit.Wrap; textFormat: TextEdit.PlainText
            readOnly: root.busy
            selectByMouse: true; selectionColor: Lithic.sel; selectedTextColor: Lithic.onSel
            Keys.forwardTo: root.keyTarget ? [root.keyTarget] : []
            onTextChanged: if (activeFocus && text !== root.text) root.edited(text)
        }
    }
    Column {
        width: parent.width
        visible: root.done && !root.text.trim()
        spacing: 8 * root.s
        Label { width: parent.width; typeRole: "body"; inkRole: "fg2"; text: I18n.tr("No text was found."); wrapMode: Text.Wrap; elide: Text.ElideNone }
        SightActionButton { s: root.s; text: I18n.tr("Read with Rashin"); enabled: !root.busy; onClicked: root.readRequested() }
    }
    Repeater {
        model: root.codes
        delegate: Column {
            required property var modelData
            width: root.width
            spacing: 6 * root.s
            Label { text: String(modelData.format || ""); typeRole: "caption"; inkRole: "fg3" }
            TextEdit {
                width: parent.width; text: String(modelData.text || "")
                color: Lithic.fg; font.family: Lithic.type.body.family; font.pixelSize: 14 * root.s
                wrapMode: TextEdit.Wrap; textFormat: TextEdit.PlainText; readOnly: true; selectByMouse: true
                selectionColor: Lithic.sel; selectedTextColor: Lithic.onSel
                Keys.forwardTo: root.keyTarget ? [root.keyTarget] : []
            }
            Row {
                spacing: 8 * root.s
                SightActionButton { s: root.s; text: I18n.tr("Copy"); glyph: "content_copy"; onClicked: root.copyRequested(String(modelData.text || "")) }
                SightActionButton { s: root.s; text: I18n.tr("Open"); glyph: "open_in_new"; enabled: Md.safeUrl(String(modelData.text || "")).length > 0; onClicked: root.openRequested(Md.safeUrl(String(modelData.text || ""))) }
            }
        }
    }
}
