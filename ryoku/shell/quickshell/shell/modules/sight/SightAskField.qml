pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons

Item {
    id: root
    property real s: 1
    property string placeholder: I18n.tr("Ask a follow-up")
    property bool busy: false
    property var keyTarget: null
    property alias text: editor.text
    readonly property bool typing: visible && editor.activeFocus
    readonly property string selectedText: editor.selectedText
    signal submitted(string text)
    implicitHeight: 40 * s
    function focusField() { editor.forceActiveFocus(); }
    function send() {
        if (busy || !editor.text.trim()) return;
        submitted(editor.text.trim());
        editor.text = "";
    }
    TextField {
        anchors.fill: parent
        text: editor.text
        placeholderText: ""
        readOnly: true; focusPolicy: Qt.NoFocus
        color: "transparent"
        leftPadding: 13 * root.s; rightPadding: 40 * root.s
        font.pixelSize: 14 * root.s
        Accessible.ignored: true
        background: Item {
            CutRect { anchors.fill: parent; cut: 7; fill: Lithic.field; strokeColor: editor.activeFocus ? Lithic.fg2 : Lithic.edge; strokeWidth: 1 }
            FocusRing { anchors.fill: parent; cut: 7; keyboardFocusVisible: editor.activeFocus }
        }
    }
    Label {
        x: 13 * root.s; width: Math.max(1, root.width - 56 * root.s)
        anchors.verticalCenter: parent.verticalCenter
        visible: editor.text.length === 0 && editor.preeditText.length === 0
        text: root.placeholder; typeRole: "body"; inkRole: "fg3"
        Accessible.ignored: true
    }
    Flickable {
        id: viewport
        x: 13 * root.s; y: 10 * root.s
        width: Math.max(1, parent.width - 56 * root.s); height: 20 * root.s
        contentWidth: width; contentHeight: editor.implicitHeight
        clip: true; interactive: false
        TextEdit {
            id: editor
            width: viewport.width
            color: Lithic.fg
            font.family: Lithic.type.body.family; font.pixelSize: 14 * root.s
            wrapMode: TextEdit.Wrap
            textFormat: TextEdit.PlainText
            selectByMouse: true
            selectionColor: Lithic.sel; selectedTextColor: Lithic.onSel
            Accessible.name: root.placeholder
            Keys.forwardTo: root.keyTarget ? [root.keyTarget] : []
            Keys.onPressed: event => {
                if ((event.key === Qt.Key_Return || event.key === Qt.Key_Enter) && !(event.modifiers & Qt.ShiftModifier)) {
                    root.send();
                    event.accepted = true;
                }
            }
            onCursorRectangleChanged: {
                if (cursorRectangle.y < viewport.contentY) viewport.contentY = cursorRectangle.y;
                else if (cursorRectangle.y + cursorRectangle.height > viewport.contentY + viewport.height)
                    viewport.contentY = cursorRectangle.y + cursorRectangle.height - viewport.height;
            }
        }
    }
    SightIconButton {
        anchors.right: parent.right; anchors.rightMargin: 6 * root.s
        anchors.verticalCenter: parent.verticalCenter
        width: 28 * root.s; height: 28 * root.s; s: root.s
        glyph: "arrow_upward"; accessibleName: I18n.tr("Send")
        enabled: !root.busy && editor.text.trim().length > 0
        onClicked: root.send()
    }
}
