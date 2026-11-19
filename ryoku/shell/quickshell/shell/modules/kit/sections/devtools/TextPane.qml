pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Lithic
import "../../../ask/look"

Item {
    id: root
    required property real s
    property string label: ""
    property string text: ""
    property string placeholder: ""
    property bool readOnly: false
    property bool error: false
    property int maximumLength: 32768
    signal limitExceeded()
    readonly property bool focused: editor.activeFocus
    signal edited(string value)
    signal cycle(bool backwards)
    function grabFocus() { editor.forceActiveFocus(); }

    Text {
        id: heading
        width: parent.width
        text: root.label
        color: root.error ? Lithic.jasper : Lithic.fg2
        font { family: Look.sans; pixelSize: 11 * root.s; weight: Font.Medium }
    }
    Shape {
        anchors { top: heading.bottom; topMargin: 9 * root.s; bottom: parent.bottom; left: parent.left; right: parent.right }
        fill: Lithic.ground
        strokeColor: root.error ? Lithic.jasper : root.focused ? Lithic.fg2 : Look.cardStroke
        strokeWidth: root.s
        cut: Look.cutCard * root.s
        Behavior on strokeColor { ColorAnimation { duration: Look.dur(Look.stateMs) } }
        Flickable {
            id: scroll
            anchors { fill: parent; margins: 13 * root.s }
            clip: true
            contentWidth: width
            contentHeight: Math.max(height, editor.contentHeight)
            boundsBehavior: Flickable.StopAtBounds
            ScrollBar.vertical: ScrollBar { policy: ScrollBar.AsNeeded }
            TextEdit {
                id: editor
                width: scroll.width
                height: Math.max(scroll.height, contentHeight)
                text: root.text
                color: root.error ? Lithic.jasper : Lithic.fg
                selectionColor: Lithic.sel
                selectedTextColor: Lithic.onSel
                font { family: Look.mono; pixelSize: 11 * root.s }
                textFormat: TextEdit.PlainText
                wrapMode: TextEdit.Wrap
                readOnly: root.readOnly
                selectByMouse: true
                activeFocusOnTab: true
                onTextChanged: {
                    if (readOnly || text === root.text)
                        return;
                    if (text.length > root.maximumLength) {
                        text = Qt.binding(() => root.text);
                        root.limitExceeded();
                    } else {
                        root.edited(text);
                    }
                }
                onCursorRectangleChanged: {
                    if (cursorRectangle.y < scroll.contentY)
                        scroll.contentY = cursorRectangle.y;
                    else if (cursorRectangle.y + cursorRectangle.height > scroll.contentY + scroll.height)
                        scroll.contentY = cursorRectangle.y + cursorRectangle.height - scroll.height;
                }
                Keys.onPressed: event => {
                    if (event.key === Qt.Key_Tab || event.key === Qt.Key_Backtab) {
                        root.cycle(event.key === Qt.Key_Backtab || Boolean(event.modifiers & Qt.ShiftModifier));
                        event.accepted = true;
                    }
                }
                Text {
                    visible: !editor.text.length
                    width: parent.width
                    text: root.placeholder
                    color: Lithic.fg3
                    font: editor.font
                    wrapMode: Text.Wrap
                }
            }
        }
    }
}
