pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Templates as T
import Lithic
import Ryoku.Ui.Singletons

// a profile account's saved settings: save one, go back to one, delete one
Dialog {
    id: dialog

    required property var hub
    property string account: ""
    property bool signedIn: false
    property var checkpoints: []
    property bool listed: false
    property string action: ""
    property string failure: ""
    property var lastCommand: []
    property string outcome: ""

    signal changed()

    readonly property bool busy: job.running

    title: I18n.tr("Checkpoints of %1").arg(dialog.account)
    dialogWidth: 620
    closePolicy: dialog.busy && dialog.action !== "list" ? T.Popup.NoAutoClose : (T.Popup.CloseOnEscape | T.Popup.CloseOnPressOutside)
    actions: [{ text: I18n.tr("Close"), kind: "quiet", enabled: !dialog.busy || dialog.action === "list" }]

    function openFor(name, loggedIn) {
        dialog.account = name;
        dialog.signedIn = loggedIn === true;
        dialog.checkpoints = [];
        dialog.listed = false;
        dialog.failure = "";
        dialog.outcome = "";
        labelField.text = "";
        dialog.open();
        dialog.refresh();
    }

    function run(action, command) {
        if (dialog.busy)
            return;
        dialog.action = action;
        dialog.failure = "";
        dialog.lastCommand = command;
        job.start(command);
    }

    function refresh() {
        dialog.run("list", ["ryoku-host", "profile", "checkpoint", "list", "--account", dialog.account]);
    }

    function save() {
        var command = ["pkexec", "ryoku-host", "profile", "checkpoint", "save", "--account", dialog.account];
        var label = labelField.text.trim();
        if (label !== "")
            command.push("--label", label);
        dialog.outcome = "";
        dialog.run("save", command);
    }

    // ryoku-host sends RFC 3339 times; shown in the person's own date and clock format
    function when(item) {
        var raw = String(item.createdAt || "");
        var d = new Date(raw);
        if (raw === "" || isNaN(d.getTime()))
            return raw;
        return Qt.formatDate(d, TimeFormat.locale.dateFormat(Locale.ShortFormat)) + " " + TimeFormat.format(d);
    }

    function nameOf(item) {
        return String(item.label || "") !== "" ? String(item.label) : I18n.tr("the checkpoint from %1").arg(dialog.when(item));
    }

    function revert(item) {
        dialog.hub.confirm({
            title: I18n.tr("Go back to %1?").arg(dialog.nameOf(item)),
            text: I18n.tr("The settings %1's profile manages go back to how they were then. Notes, projects and VPN files stay as they are now.").arg(dialog.account),
            actionText: I18n.tr("Go back"),
            destructive: true
        }, () => {
            dialog.outcome = "";
            dialog.run("revert", ["pkexec", "ryoku-host", "profile", "checkpoint", "revert", "--account", dialog.account, "--id", String(item.id)]);
        });
    }

    function remove(item) {
        dialog.hub.confirm({
            title: I18n.tr("Delete %1?").arg(dialog.nameOf(item)),
            text: I18n.tr("This checkpoint goes for good. The account's settings stay as they are."),
            actionText: I18n.tr("Delete"),
            destructive: true
        }, () => {
            dialog.outcome = "";
            dialog.run("delete", ["pkexec", "ryoku-host", "profile", "checkpoint", "delete", "--account", dialog.account, "--id", String(item.id)]);
        });
    }

    // a long list scrolls, so what the dialog reports at its foot is brought into view
    onFailureChanged: if (dialog.failure !== "") Qt.callLater(() => body.reveal(status))
    Connections {
        target: job
        function onSlowChanged() {
            if (job.running && job.slow && dialog.action !== "list")
                Qt.callLater(() => body.reveal(status));
        }
    }

    Job {
        id: job
        onFinished: (ok, message) => {
            if (!ok) {
                dialog.failure = message;
                return;
            }
            var value = {};
            try {
                value = JSON.parse(job.out.trim().split("\n").pop() || "{}");
            } catch (e) {
            }
            if (dialog.action === "list") {
                dialog.checkpoints = Array.isArray(value.checkpoints) ? value.checkpoints : [];
                dialog.listed = true;
                if (dialog.outcome !== "")
                    Qt.callLater(() => body.reveal(outcomeLabel));
                return;
            }
            if (dialog.action === "save") {
                labelField.text = "";
                dialog.outcome = I18n.tr("Checkpoint saved.");
            } else if (dialog.action === "revert") {
                dialog.outcome = I18n.tr("The settings are back to that checkpoint.");
            } else {
                dialog.outcome = I18n.tr("Checkpoint deleted.");
            }
            dialog.changed();
            dialog.refresh();
        }
    }

    DialogBody {
        id: body
        width: parent ? parent.width : 0
        maxHeight: dialog.parent ? dialog.parent.height - 280 : 480

        Label {
            width: parent.width
            text: I18n.tr("A checkpoint keeps the settings the profile manages, so you can go back to them after a lab or a big change.")
            typeRole: "body"
            inkRole: "fg2"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }

        Row {
            width: parent.width
            spacing: LithicTokens.space.s2

            TextField {
                id: labelField
                width: parent.width - saveButton.width - LithicTokens.space.s2
                label: I18n.tr("Name for a new checkpoint")
                placeholderText: I18n.tr("Optional, such as Before the lab")
                enabled: !dialog.busy
                onAccepted: dialog.save()
            }
            Plaque {
                id: saveButton
                anchors.bottom: labelField.bottom
                kind: "line"
                glyph: "bookmark_add"
                text: I18n.tr("Save")
                busy: dialog.busy && dialog.action === "save" && job.slow
                enabled: !dialog.busy
                onClicked: dialog.save()
            }
        }

        Label {
            width: parent.width
            text: dialog.signedIn
                ? I18n.tr("%1 is signed in. Go back to a checkpoint once they sign out.").arg(dialog.account)
                : I18n.tr("Keep %1 signed out while going back to a checkpoint.").arg(dialog.account)
            typeRole: "detail"
            ink: dialog.signedIn ? Lithic.amber : Lithic.fg2
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }

        Column {
            width: parent.width
            spacing: 0

            Label {
                visible: dialog.listed && dialog.checkpoints.length === 0
                width: parent.width
                text: I18n.tr("No checkpoints yet. Save one above.")
                typeRole: "body"
                inkRole: "fg2"
            }
            Repeater {
                model: dialog.checkpoints
                delegate: SettingRow {
                    id: cp
                    required property var modelData
                    required property int index
                    width: parent.width
                    first: index === 0
                    plane: Lithic.stone4
                    title: String(modelData.label || "") !== "" ? String(modelData.label) : I18n.tr("Unnamed checkpoint")
                    detail: dialog.when(modelData)

                    Plaque {
                        kind: "quiet"
                        small: true
                        glyph: "history"
                        text: I18n.tr("Go back")
                        enabled: !dialog.busy && !dialog.signedIn
                        accessibleName: I18n.tr("Go back to %1").arg(cp.title)
                        onClicked: dialog.revert(cp.modelData)
                    }
                    Plaque {
                        kind: "quiet"
                        small: true
                        glyph: "delete"
                        text: I18n.tr("Delete")
                        enabled: !dialog.busy
                        accessibleName: I18n.tr("Delete %1").arg(cp.title)
                        onClicked: dialog.remove(cp.modelData)
                    }
                }
            }
        }

        Label {
            id: outcomeLabel
            visible: dialog.outcome.length > 0 && !dialog.busy
            width: parent.width
            text: dialog.outcome
            typeRole: "detail"
            inkRole: "fg"
        }

        JobStatus {
            id: status
            width: parent.width
            running: dialog.busy
            slow: job.slow
            lengthy: job.lengthy
            waitingText: dialog.action === "list" ? I18n.tr("Reading the checkpoints") : I18n.tr("Waiting for an administrator's password")
            error: dialog.failure
            onRetry: dialog.run(dialog.action, dialog.lastCommand)
            onCancel: job.cancel()
        }
    }
}
