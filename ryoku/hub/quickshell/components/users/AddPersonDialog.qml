pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Templates as T
import Lithic
import Ryoku.Ui.Singletons
import "users.js" as Users

// a new person: their own sign-in, home folder and desktop
Dialog {
    id: dialog

    required property var store
    property string role: "standard"
    property var chosen: []
    property string error: ""
    property string failure: ""
    property bool running: false
    property var lastArgs: []
    property string lastSecret: ""

    signal profileRequested(var definition)
    signal added(string name)

    title: I18n.tr("Add a person")
    dialogWidth: 560
    closePolicy: dialog.running ? T.Popup.NoAutoClose : (T.Popup.CloseOnEscape | T.Popup.CloseOnPressOutside)
    actions: [
        { text: I18n.tr("Cancel"), kind: "quiet", enabled: !dialog.running },
        { text: I18n.tr("Add person"), kind: "gold", busy: dialog.running && dialog.store.slow, enabled: !dialog.running, closeOnTrigger: false }
    ]
    onActionTriggered: index => {
        if (index === 1)
            dialog.submit();
    }

    function openForCreate() {
        nameField.text = "";
        fullNameField.text = "";
        passwordField.text = "";
        confirmField.text = "";
        limitField.text = "";
        dialog.role = "standard";
        roleChoice.currentIndex = 0;
        dialog.chosen = dialog.store.permissionDefs.some(d => d.id === "devices") ? ["devices"] : [];
        dialog.error = "";
        dialog.failure = "";
        dialog.open();
        nameField.forceActiveFocus();
    }

    function toggle(id) {
        var next = dialog.chosen.filter(p => p !== id);
        if (next.length === dialog.chosen.length)
            next.push(id);
        dialog.chosen = next;
    }

    function submit() {
        if (dialog.running)
            return;
        dialog.error = "";
        dialog.failure = "";
        var name = nameField.text.trim();
        var problem = Users.nameProblem(name);
        if (problem !== "") {
            dialog.error = problem === "empty" ? I18n.tr("Enter a user name.")
                : I18n.tr("Start the user name with a lowercase letter, then use up to 32 lowercase letters, numbers, hyphens or underscores.");
            nameField.forceActiveFocus();
            return;
        }
        if (dialog.store.user(name)) {
            dialog.error = I18n.tr("Someone on this computer already uses %1. Choose another user name.").arg(name);
            nameField.forceActiveFocus();
            return;
        }
        if (/[:\n\r,]/.test(fullNameField.text)) {
            dialog.error = I18n.tr("Leave colons, commas and line breaks out of the full name.");
            fullNameField.forceActiveFocus();
            return;
        }
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
        var limit = 0;
        if (limitField.visible && limitField.text.trim() !== "") {
            limit = Users.gibToBytes(limitField.text);
            if (limit <= 0) {
                dialog.error = I18n.tr("Enter the limit as a number of gigabytes, such as 128, or leave it empty for no limit.");
                limitField.forceActiveFocus();
                return;
            }
        }
        var args = ["pkexec", "ryoku-host", "users", "add", name, "--full-name", fullNameField.text.trim(), "--role", dialog.role];
        dialog.chosen.forEach(p => args.push("--perm", p));
        if (limit > 0)
            args.push("--limit", String(limit));
        dialog.lastArgs = args;
        dialog.lastSecret = passwordField.text;
        passwordField.text = "";
        confirmField.text = "";
        dialog.send();
    }

    function send() {
        dialog.running = true;
        dialog.failure = "";
        var name = dialog.lastArgs[4];
        dialog.store.run(dialog.lastArgs, dialog.lastSecret, (ok, message) => {
            dialog.running = false;
            if (ok) {
                dialog.lastSecret = "";
                dialog.close();
                dialog.added(name);
            } else {
                dialog.failure = message;
            }
        });
    }

    onClosed: dialog.lastSecret = ""
    // the form scrolls, so what it reports at its foot is brought into view
    onErrorChanged: if (dialog.error !== "") Qt.callLater(() => body.reveal(errorLabel))
    onFailureChanged: if (dialog.failure !== "") Qt.callLater(() => body.reveal(status))
    Connections {
        target: dialog.store
        function onSlowChanged() {
            if (dialog.running && dialog.store.slow)
                Qt.callLater(() => body.reveal(status));
        }
        function onLengthyChanged() {
            if (dialog.running && dialog.store.lengthy)
                Qt.callLater(() => body.reveal(status));
        }
    }

    DialogBody {
        id: body
        width: parent ? parent.width : 0
        maxHeight: dialog.parent ? dialog.parent.height - 280 : 480

        Label {
            width: parent.width
            text: I18n.tr("They get a private home folder and their own desktop the first time they sign in.")
            typeRole: "body"
            inkRole: "fg2"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }

        Column {
            visible: dialog.store.installedProfiles.length > 0
            width: parent.width
            spacing: LithicTokens.space.s2

            Label {
                width: parent.width
                text: I18n.tr("Start from a profile")
                typeRole: "label"
            }
            Label {
                width: parent.width
                text: I18n.tr("A profile sets up a separate standard account with its own tools. Or fill in the form below for a plain account.")
                typeRole: "detail"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Flow {
                width: parent.width
                spacing: LithicTokens.space.s2
                Repeater {
                    model: dialog.store.installedProfiles
                    delegate: Plaque {
                        required property var modelData
                        kind: "line"
                        small: true
                        glyph: "person_add"
                        enabled: !dialog.running
                        text: I18n.tr("Create a %1 account").arg(String(modelData.name || modelData.id))
                        onClicked: {
                            dialog.close();
                            dialog.profileRequested(modelData);
                        }
                    }
                }
            }
        }

        Row {
            width: parent.width
            spacing: LithicTokens.space.s3

            TextField {
                id: nameField
                width: (parent.width - parent.spacing) / 2
                label: I18n.tr("User name")
                placeholderText: "sam"
                mono: true
                enabled: !dialog.running
                helpText: I18n.tr("Lowercase, used to sign in")
                onActiveFocusChanged: if (activeFocus) body.reveal(nameField)
                onAccepted: fullNameField.forceActiveFocus()
            }
            TextField {
                id: fullNameField
                width: (parent.width - parent.spacing) / 2
                label: I18n.tr("Full name")
                placeholderText: I18n.tr("Sam Rivera")
                enabled: !dialog.running
                helpText: I18n.tr("Shown at sign-in")
                onActiveFocusChanged: if (activeFocus) body.reveal(fullNameField)
                onAccepted: passwordField.forceActiveFocus()
            }
        }

        Column {
            width: parent.width
            spacing: LithicTokens.space.s2

            Label {
                text: I18n.tr("Role")
                typeRole: "label"
            }
            Segmented {
                id: roleChoice
                model: [I18n.tr("Standard"), I18n.tr("Administrator")]
                currentIndex: dialog.role === "admin" ? 1 : 0
                enabled: !dialog.running
                accessibleName: I18n.tr("Role")
                onActivated: index => dialog.role = index === 1 ? "admin" : "standard"
            }
            Label {
                width: parent.width
                text: dialog.role === "admin"
                    ? I18n.tr("Administrators can install apps, change the system and manage other accounts.")
                    : I18n.tr("Standard accounts use the computer and change only their own settings.")
                typeRole: "detail"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
        }

        Row {
            width: parent.width
            spacing: LithicTokens.space.s3

            TextField {
                id: passwordField
                width: (parent.width - parent.spacing) / 2
                label: I18n.tr("Password")
                secret: true
                enabled: !dialog.running
                onActiveFocusChanged: if (activeFocus) body.reveal(passwordField)
                onAccepted: confirmField.forceActiveFocus()
            }
            TextField {
                id: confirmField
                width: (parent.width - parent.spacing) / 2
                label: I18n.tr("Type it again")
                secret: true
                enabled: !dialog.running
                onActiveFocusChanged: if (activeFocus) body.reveal(confirmField)
                onAccepted: dialog.submit()
            }
        }

        Column {
            visible: dialog.store.permissionDefs.length > 0
            width: parent.width
            spacing: 0

            Label {
                width: parent.width
                bottomPadding: LithicTokens.space.s1
                text: I18n.tr("Permissions")
                typeRole: "label"
            }
            Repeater {
                model: dialog.store.permissionDefs
                delegate: SettingRow {
                    id: permRow
                    required property var modelData
                    required property int index
                    readonly property bool on: dialog.chosen.indexOf(String(modelData.id)) >= 0
                    width: parent.width
                    first: index === 0
                    plane: Lithic.stone4
                    enabled: !dialog.running
                    activatable: true
                    title: Users.permissionTitle(modelData, I18n)
                    detail: Users.permissionDetail(modelData, I18n)
                    onActivated: dialog.toggle(String(modelData.id))

                    Toggle {
                        checked: permRow.on
                        Accessible.name: permRow.title
                        onToggled: dialog.toggle(String(permRow.modelData.id))
                    }
                }
            }
        }

        TextField {
            id: limitField
            visible: dialog.store.storageCaps.quota === true
            width: Math.min(parent.width, LithicTokens.size.settingsControlMax)
            label: I18n.tr("Storage limit in GB")
            placeholderText: I18n.tr("No limit")
            helpText: I18n.tr("Leave it empty for no limit.")
            enabled: !dialog.running
            validator: DoubleValidator { bottom: 0 }
            onActiveFocusChanged: if (activeFocus) body.reveal(limitField)
            Keys.onReturnPressed: dialog.submit()
            Keys.onEnterPressed: dialog.submit()
        }

        Label {
            id: errorLabel
            visible: dialog.error.length > 0
            width: parent.width
            text: dialog.error
            typeRole: "detail"
            ink: Lithic.jasper
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }

        JobStatus {
            id: status
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
