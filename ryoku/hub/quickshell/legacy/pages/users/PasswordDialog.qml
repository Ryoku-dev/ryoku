import QtQuick
import QtQuick.Controls
import Ryoku.Ui
import Ryoku.Ui.Singletons

Popup {
    id: dialog

    property string userName: ""
    property bool busy: false
    property string error: ""
    property string localError: ""

    signal submitted(string name, string password)

    parent: Overlay.overlay
    anchors.centerIn: parent
    modal: true
    width: Math.min(440, parent ? parent.width - Tokens.s6 * 2 : 440)
    padding: Tokens.s5
    closePolicy: busy ? Popup.NoAutoClose : Popup.CloseOnEscape | Popup.CloseOnPressOutside

    background: Rectangle {
        color: Tokens.paper
        border.width: Tokens.border
        border.color: Tokens.lineStrong
        radius: Tokens.radius
    }
    Overlay.modal: Rectangle { color: Qt.rgba(0, 0, 0, 0.55) }

    function openFor(name) {
        dialog.userName = name;
        dialog.localError = "";
        passwordField.text = "";
        confirmField.text = "";
        open();
        passwordField.grabFocus();
    }

    function submit() {
        dialog.localError = "";
        if (passwordField.text === "") {
            dialog.localError = I18n.tr("Enter a password.");
            passwordField.grabFocus();
            return;
        }
        if (passwordField.text !== confirmField.text) {
            dialog.localError = I18n.tr("Passwords do not match.");
            confirmField.grabFocus();
            return;
        }
        dialog.submitted(dialog.userName, passwordField.text);
        passwordField.text = "";
        confirmField.text = "";
    }

    contentItem: Column {
        spacing: Tokens.s4

        Column {
            width: parent.width
            spacing: Tokens.s1
            Text {
                text: I18n.tr("RESET PASSWORD")
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fMicro
                font.weight: Font.Medium
                font.letterSpacing: Tokens.trackMark
            }
            Text {
                width: parent.width
                text: I18n.tr("New password for %1").arg(dialog.userName)
                color: Tokens.ink
                font.family: Tokens.display
                font.pixelSize: Tokens.fValue
                wrapMode: Text.WordWrap
            }
        }

        Column {
            width: parent.width
            spacing: Tokens.s1
            Text {
                text: I18n.tr("NEW PASSWORD")
                color: Tokens.inkDim
                font.family: Tokens.ui
                font.pixelSize: Tokens.fMicro
                font.letterSpacing: Tokens.trackLabel
            }
            Field { id: passwordField; width: parent.width; secret: true; placeholder: I18n.tr("Password") }
        }

        Column {
            width: parent.width
            spacing: Tokens.s1
            Text {
                text: I18n.tr("CONFIRM")
                color: Tokens.inkDim
                font.family: Tokens.ui
                font.pixelSize: Tokens.fMicro
                font.letterSpacing: Tokens.trackLabel
            }
            Field {
                id: confirmField
                width: parent.width
                secret: true
                placeholder: I18n.tr("Repeat password")
                onAccepted: dialog.submit()
            }
        }

        Text {
            visible: dialog.localError !== "" || dialog.error !== ""
            width: parent.width
            text: dialog.localError !== "" ? dialog.localError : dialog.error
            color: Tokens.alert
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            font.weight: Font.Medium
            wrapMode: Text.WordWrap
        }

        Row {
            anchors.right: parent.right
            spacing: Tokens.s2
            Btn { text: I18n.tr("CANCEL"); armed: !dialog.busy; onAct: dialog.close() }
            Btn {
                text: dialog.busy ? I18n.tr("SAVING…") : I18n.tr("SAVE PASSWORD")
                primary: true
                armed: !dialog.busy
                onAct: dialog.submit()
            }
        }
    }
}
