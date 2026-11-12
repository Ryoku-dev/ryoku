import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// a failure under a row: what went wrong and what to do, in jasper, with Retry
Row {
    id: note

    property string text: ""
    property bool canRetry: true

    signal retry()

    // shown and hidden here rather than bound: a Retry that succeeds clears the text, so its
    // plaque hands the keyboard to the control before it while the note is still shown, or
    // focus falls to the window and the next key lands in search
    visible: false
    Component.onCompleted: note.visible = note.text.length > 0
    onTextChanged: {
        var show = note.text.length > 0;
        if (!show && retryPlaque.activeFocus) {
            var back = retryPlaque.nextItemInFocusChain(false);
            if (back && back !== retryPlaque)
                back.forceActiveFocus(Qt.BacktabFocusReason);
        }
        note.visible = show;
    }
    spacing: LithicTokens.space.s3

    Label {
        anchors.verticalCenter: parent.verticalCenter
        width: Math.min(implicitWidth, note.width - (retryPlaque.visible ? retryPlaque.width + note.spacing : 0))
        text: note.text
        typeRole: "detail"
        ink: Lithic.jasper
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
    Plaque {
        id: retryPlaque
        visible: note.canRetry
        kind: "quiet"
        small: true
        glyph: "refresh"
        text: I18n.tr("Retry")
        onClicked: note.retry()
    }
}
