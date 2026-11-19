pragma ComponentBehavior: Bound
import QtQuick
import Lithic
import "../../../ask/look"
Flickable {
    id: root
    required property real s
    required property var engine
    required property var store
    readonly property real rowHeight: Math.max(1, words.cursorRectangle.height)
    implicitHeight: rowHeight * Math.max(2, store.visibleLines)
    contentWidth: width; contentHeight: words.height
    clip: true; interactive: false
    function htmlEscape(text) { return String(text).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;"); }
    function markup() {
        const typed = Array.from(engine.inputText), target = engine.targetChars;
        const parts = [];
        let word = 0;
        for (let i = 0; i < target.length; i++) {
            let color = i < typed.length ? (store.blindMode || typed[i] === target[i] ? Lithic.fg : Lithic.jasper) : Lithic.fg3;
            if (store.highlightCurrentWord && word !== engine.currentWordIndex)
                color = Lithic.fg3;
            const caret = i === typed.length && store.caretStyle !== "off";
            const decoration = caret && store.caretStyle !== "line" ? (store.caretStyle === "block" ? 'background-color:' + Lithic.sel + ';' : 'text-decoration:underline;') : '';
            parts.push('<span style="color:' + color + ';' + decoration + '">' + htmlEscape(target[i]) + '</span>');
            if (target[i] === " ") word++;
        }
        return parts.join("");
    }
    TextEdit {
        id: words
        width: root.width
        height: Math.max(root.height, contentHeight)
        text: root.markup()
        textFormat: TextEdit.RichText; readOnly: true; selectByMouse: false
        activeFocusOnPress: false
        color: Lithic.fg3
        font.family: root.engine.languagePack?.preferredFont || Look.mono
        font.pixelSize: root.store.fontSize * root.s
        wrapMode: TextEdit.Wrap
        cursorVisible: false
        onTextChanged: Qt.callLater(() => {
            cursorPosition = Math.min(length, root.engine.inputText.length);
            root.contentY = Math.max(0, Math.min(root.contentHeight - root.height, cursorRectangle.y - root.rowHeight));
        })
    }
    Rectangle {
        x: words.cursorRectangle.x
        y: words.cursorRectangle.y
        width: 2 * root.s
        height: words.cursorRectangle.height
        visible: root.store.caretStyle === "line"
        color: Lithic.sel
        Behavior on x { enabled: root.store.smoothCaret; NumberAnimation { duration: Look.dur(90); easing.type: Easing.OutCubic } }
    }
    Behavior on contentY { enabled: root.store.smoothCaret; NumberAnimation { duration: Look.dur(90); easing.type: Easing.OutCubic } }
}
