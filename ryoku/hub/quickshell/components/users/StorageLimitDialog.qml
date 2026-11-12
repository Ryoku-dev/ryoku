import QtQuick
import QtQuick.Templates as T
import Lithic
import Ryoku.Ui.Singletons
import "users.js" as Users

// how much one person's home folder may hold
Dialog {
    id: dialog

    required property var store
    property string userName: ""
    property bool limited: false
    property double usedBytes: 0
    property string error: ""
    property string failure: ""
    property bool running: false
    property double lastBytes: 0

    signal changed(string name)

    title: I18n.tr("Storage limit for %1").arg(dialog.userName)
    closePolicy: dialog.running ? T.Popup.NoAutoClose : (T.Popup.CloseOnEscape | T.Popup.CloseOnPressOutside)
    actions: [
        { text: I18n.tr("Cancel"), kind: "quiet", enabled: !dialog.running },
        { text: I18n.tr("Save limit"), kind: "gold", busy: dialog.running && dialog.store.slow, enabled: !dialog.running, closeOnTrigger: false }
    ]
    onActionTriggered: index => {
        if (index === 1)
            dialog.submit();
    }

    function openFor(name, limitBytes, usedBytes) {
        dialog.userName = name;
        dialog.limited = Number(limitBytes || 0) > 0;
        limitChoice.currentIndex = dialog.limited ? 1 : 0;
        dialog.usedBytes = Number(usedBytes || 0);
        limitField.text = Users.bytesToGib(limitBytes);
        dialog.error = "";
        dialog.failure = "";
        dialog.open();
        if (dialog.limited)
            limitField.forceActiveFocus();
    }

    function submit() {
        if (dialog.running)
            return;
        dialog.error = "";
        var bytes = 0;
        if (dialog.limited) {
            bytes = Users.gibToBytes(limitField.text);
            if (bytes <= 0) {
                dialog.error = I18n.tr("Enter the limit as a number of gigabytes, such as 128.");
                limitField.forceActiveFocus();
                return;
            }
        }
        dialog.lastBytes = bytes;
        dialog.send();
    }

    function send() {
        dialog.running = true;
        dialog.failure = "";
        var name = dialog.userName;
        dialog.store.run(["pkexec", "ryoku-host", "users", "set-limit", name, String(Math.round(dialog.lastBytes))], undefined, (ok, message) => {
            dialog.running = false;
            if (ok) {
                dialog.close();
                dialog.changed(name);
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
            text: dialog.usedBytes > 0
                ? I18n.tr("The home folder holds %1 now. With a limit, saving more than it allows fails until something is deleted.").arg(Users.size(dialog.usedBytes))
                : I18n.tr("With a limit, saving more than it allows fails until something is deleted.")
            typeRole: "body"
            inkRole: "fg2"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Segmented {
            id: limitChoice
            model: [I18n.tr("No limit"), I18n.tr("Set a limit")]
            currentIndex: dialog.limited ? 1 : 0
            enabled: !dialog.running
            accessibleName: I18n.tr("Storage limit")
            onActivated: index => {
                dialog.limited = index === 1;
                dialog.error = "";
                if (dialog.limited)
                    limitField.forceActiveFocus();
            }
        }
        TextField {
            id: limitField
            visible: dialog.limited
            width: Math.min(parent.width, LithicTokens.size.settingsControlMax)
            label: I18n.tr("Limit in GB")
            placeholderText: "128"
            enabled: !dialog.running
            validator: DoubleValidator { bottom: 0 }
            onAccepted: dialog.submit()
            // an empty or partly typed number never sends accepted
            Keys.onReturnPressed: dialog.submit()
            Keys.onEnterPressed: dialog.submit()
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
