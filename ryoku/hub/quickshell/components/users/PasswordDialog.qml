import QtQuick
import QtQuick.Templates as T
import Lithic
import Ryoku.Ui.Singletons

// a new sign-in password for one person
Dialog {
    id: dialog

    required property var store
    property string userName: ""
    property string error: ""
    property string failure: ""
    property bool running: false
    property string lastSecret: ""

    signal changed(string name)

    title: I18n.tr("New password for %1").arg(dialog.userName)
    closePolicy: dialog.running ? T.Popup.NoAutoClose : (T.Popup.CloseOnEscape | T.Popup.CloseOnPressOutside)
    actions: [
        { text: I18n.tr("Cancel"), kind: "quiet", enabled: !dialog.running },
        { text: I18n.tr("Change password"), kind: "gold", busy: dialog.running && dialog.store.slow, enabled: !dialog.running, closeOnTrigger: false }
    ]
    onActionTriggered: index => {
        if (index === 1)
            dialog.submit();
    }

    function openFor(name) {
        dialog.userName = name;
        dialog.error = "";
        dialog.failure = "";
        dialog.lastSecret = "";
        passwordField.text = "";
        confirmField.text = "";
        dialog.open();
        passwordField.forceActiveFocus();
    }

    function submit() {
        if (dialog.running)
            return;
        dialog.error = "";
        if (passwordField.text === "") {
            dialog.error = I18n.tr("Enter a password.");
            passwordField.forceActiveFocus();
            return;
        }
        if (passwordField.text !== confirmField.text) {
            dialog.error = I18n.tr("The passwords do not match. Type the same password twice.");
            confirmField.forceActiveFocus();
            return;
        }
        dialog.lastSecret = passwordField.text;
        passwordField.text = "";
        confirmField.text = "";
        dialog.send();
    }

    function send() {
        dialog.running = true;
        dialog.failure = "";
        var name = dialog.userName;
        dialog.store.run(["pkexec", "ryoku-host", "users", "set-password", name], dialog.lastSecret, (ok, message) => {
            dialog.running = false;
            if (ok) {
                dialog.lastSecret = "";
                dialog.close();
                dialog.changed(name);
            } else {
                dialog.failure = message;
            }
        });
    }

    onClosed: dialog.lastSecret = ""

    Column {
        width: parent ? parent.width : 0
        spacing: LithicTokens.space.s4

        Label {
            width: parent.width
            text: dialog.userName === dialog.store.self
                ? I18n.tr("You sign in with this password from now on. Your keyring may ask for the old one once.")
                : I18n.tr("%1 signs in with this password from now on. Tell them the new one.").arg(dialog.userName)
            typeRole: "body"
            inkRole: "fg2"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        TextField {
            id: passwordField
            width: parent.width
            label: I18n.tr("New password")
            secret: true
            enabled: !dialog.running
            onAccepted: confirmField.forceActiveFocus()
        }
        TextField {
            id: confirmField
            width: parent.width
            label: I18n.tr("Type it again")
            secret: true
            enabled: !dialog.running
            onAccepted: dialog.submit()
        }
        Label {
            visible: dialog.error.length > 0
            width: parent.width
            text: dialog.error
            typeRole: "detail"
            ink: Lithic.jasper
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        JobStatus {
            width: parent.width
            running: dialog.running
            slow: dialog.store.slow
            lengthy: dialog.store.lengthy
            error: dialog.failure
            canRetry: dialog.lastSecret !== ""
            onRetry: dialog.send()
            onCancel: dialog.store.cancel()
        }
    }
}
