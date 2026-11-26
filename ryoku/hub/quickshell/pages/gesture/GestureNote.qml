import QtQuick
import Ryoku.Ui.Singletons

// One wrapped sentence. The caller passes the width, so the line breaks inside
// that width and the sentence itself does not widen the page.
Item {
    id: noteBox

    property string text: ""
    property real noteWidth: 0

    width: noteWidth
    implicitHeight: note.implicitHeight
    visible: text.length > 0

    Text {
        id: note
        width: noteBox.noteWidth
        wrapMode: Text.WordWrap
        text: noteBox.text
        color: Tokens.inkMuted
        font.family: Tokens.ui
        font.pixelSize: 11
    }
}
