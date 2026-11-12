import QtQuick
import QtQuick.Templates as T
import Lithic
import Ryoku.Ui.Singletons

// deletes an account; its home folder is archived, or erased once its name is typed
Dialog {
    id: dialog

    required property var store
    property string userName: ""
    property bool erase: false
    property string error: ""
    property string failure: ""
    property bool running: false

    signal removed(string name)

    title: I18n.tr("Remove %1?").arg(dialog.userName)
    closePolicy: dialog.running ? T.Popup.NoAutoClose : (T.Popup.CloseOnEscape | T.Popup.CloseOnPressOutside)
    actions: [
        { text: I18n.tr("Cancel"), kind: "quiet", enabled: !dialog.running },
        { text: dialog.erase ? I18n.tr("Remove and erase") : I18n.tr("Remove"), kind: "danger",
          busy: dialog.running && dialog.store.slow, closeOnTrigger: false,
          enabled: !dialog.running && (!dialog.erase || confirmField.text.trim() === dialog.userName) }
    ]
    onActionTriggered: index => {
        if (index === 1)
            dialog.submit();
    }

    function openFor(name) {
        dialog.userName = name;
        dialog.erase = false;
        // the control keeps the index of its last click, so set it back too
        fileChoice.currentIndex = 0;
        dialog.error = "";
        dialog.failure = "";
        confirmField.text = "";
        dialog.open();
    }

    function submit() {
        if (dialog.running)
            return;
        dialog.error = "";
        if (dialog.erase && confirmField.text.trim() !== dialog.userName) {
            dialog.error = I18n.tr("Type %1 to erase this person's files.").arg(dialog.userName);
            confirmField.forceActiveFocus();
            return;
        }
        dialog.send();
    }

    function send() {
        dialog.running = true;
        dialog.failure = "";
        var name = dialog.userName;
        dialog.store.run(["pkexec", "ryoku-host", "users", "delete", name, dialog.erase ? "--erase-files" : "--keep-files"], undefined, (ok, message) => {
            dialog.running = false;
            if (ok) {
                dialog.close();
                dialog.removed(name);
            } else {
                dialog.failure = message;
            }
        });
    }

    Column {
        width: parent ? parent.width : 0
        spacing: LithicTokens.space.s4

        Label {
            width: parent.width
            text: I18n.tr("%1 can no longer sign in. Sign them out first if they are signed in.").arg(dialog.userName)
            typeRole: "body"
            inkRole: "fg2"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Segmented {
            id: fileChoice
            model: [I18n.tr("Keep their files"), I18n.tr("Erase their files")]
            currentIndex: dialog.erase ? 1 : 0
            enabled: !dialog.running
            accessibleName: I18n.tr("Their files")
            onActivated: index => {
                dialog.erase = index === 1;
                dialog.error = "";
                if (dialog.erase)
                    confirmField.forceActiveFocus();
            }
        }
        Label {
            width: parent.width
            text: dialog.erase
                ? I18n.tr("The home folder is deleted and cannot be brought back from here. Older system snapshots may still hold copies.")
                : I18n.tr("An administrator can still reach the home folder in the archive of removed accounts, /home/.removed.")
            typeRole: "detail"
            ink: dialog.erase ? Lithic.jasper : Lithic.fg2
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        TextField {
            id: confirmField
            visible: dialog.erase
            width: Math.min(parent.width, LithicTokens.size.settingsControlMax)
            label: I18n.tr("Type %1 to confirm").arg(dialog.userName)
            mono: true
            placeholderText: dialog.userName
            enabled: !dialog.running
            onAccepted: dialog.submit()
            onTextChanged: dialog.error = ""
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
            onRetry: dialog.send()
            onCancel: dialog.store.cancel()
        }
    }
}
