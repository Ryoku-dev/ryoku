import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// the profile account you are signed in to: reset it, save a checkpoint, and what it shares
Slab {
    id: root

    required property var store
    property var page: null
    property bool spot: true
    property string saved: ""
    property string failure: ""

    readonly property var status: root.store.selfProfile || ({})
    readonly property string account: String(root.status.account || root.store.self)

    signal resetRequested()

    function saveCheckpoint() {
        if (job.running)
            return;
        var command = ["pkexec", "ryoku-host", "profile", "checkpoint", "save", "--account", root.account];
        if (labelField.text.trim() !== "")
            command.push("--label", labelField.text.trim());
        root.saved = "";
        root.failure = "";
        job.start(command);
    }

    Job {
        id: job
        onFinished: (ok, message) => {
            if (ok) {
                labelField.text = "";
                root.saved = I18n.tr("Checkpoint saved.");
                root.store.refreshProfiles();
            } else {
                root.failure = message;
            }
        }
    }

    Column {
        width: parent.width

        Column {
            width: parent.width
            spacing: LithicTokens.size.settingsDetailGap
            Label {
                width: parent.width
                text: I18n.tr("%1 profile").arg(root.store.profileName(root.status.profile))
                typeRole: "row"
            }
            Row {
                spacing: LithicTokens.space.s3
                Label {
                    text: root.account
                    typeRole: "monoSm"
                    inkRole: "fg2"
                }
                Label {
                    visible: String(root.status.version || "") !== ""
                    text: I18n.tr("Version %1").arg(String(root.status.version || ""))
                    typeRole: "detail"
                }
            }
        }

        PersonRow {
            owner: root
            rowId: "this-reset"
            title: I18n.tr("Reset settings")
            detail: I18n.tr("Put back every file the profile manages. Projects, notes, engagements and VPN files stay.")

            Plaque {
                kind: "quiet"
                small: true
                glyph: "restart_alt"
                text: I18n.tr("Reset")
                onClicked: root.resetRequested()
            }
        }

        PersonRow {
            id: saveRow
            owner: root
            rowId: "this-checkpoint"
            title: I18n.tr("Save a checkpoint")
            detail: I18n.tr("Keep this account's settings as they are now, before a lab or a big change. An administrator goes back to it from their account.")
            note: root.saved

            TextField {
                id: labelField
                width: 200
                placeholderText: I18n.tr("Name, optional")
                enabled: !job.running
                Accessible.name: I18n.tr("Checkpoint name")
                onAccepted: root.saveCheckpoint()
            }
            Plaque {
                anchors.verticalCenter: parent.verticalCenter
                kind: "quiet"
                small: true
                glyph: "bookmark_add"
                text: I18n.tr("Save")
                busy: job.running && job.slow
                enabled: !job.running
                onClicked: root.saveCheckpoint()
            }
        }

        JobStatus {
            width: parent.width
            topPadding: LithicTokens.space.s2
            running: job.running
            slow: job.slow
            lengthy: job.lengthy
            error: root.failure
            onRetry: root.saveCheckpoint()
            onCancel: job.cancel()
        }

        Item { width: 1; height: LithicTokens.size.settingsRowGap }

        Label {
            width: parent.width
            text: I18n.tr("A profile is a separate account, not a sandbox.")
            typeRole: "body"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Label {
            width: parent.width
            text: I18n.tr("Its menus, dock and default apps show only its own tools, but programs installed for the whole computer still start from its terminal, and apps it installs appear in the other accounts too. Each home folder stays readable only by its owner.")
            typeRole: "detail"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
    }
}
