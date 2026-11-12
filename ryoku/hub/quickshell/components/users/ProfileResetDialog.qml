pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Templates as T
import Lithic
import Ryoku.Ui.Singletons

// puts back the files a profile manages, after showing which ones change
Dialog {
    id: dialog

    // the account to reset, or "" for your own profile account (no administrator needed)
    property string account: ""
    property var preview: null
    property bool dryRun: true
    property bool finished: false
    property string failure: ""
    // a preview still stopping after a close starts again when it ends
    property bool rerun: false

    signal completed()

    readonly property bool busy: job.running
    readonly property var restored: dialog.preview && Array.isArray(dialog.preview.restored) ? dialog.preview.restored : []

    title: dialog.finished ? I18n.tr("Settings reset")
        : dialog.account !== "" ? I18n.tr("Reset the settings of %1?").arg(dialog.account) : I18n.tr("Reset this account's settings?")
    dialogWidth: 580
    closePolicy: dialog.busy && !dialog.dryRun ? T.Popup.NoAutoClose : (T.Popup.CloseOnEscape | T.Popup.CloseOnPressOutside)
    actions: dialog.finished
        ? [{ text: I18n.tr("Done"), kind: "gold" }]
        : [{ text: I18n.tr("Cancel"), kind: "quiet", enabled: !dialog.busy || dialog.dryRun },
           { text: I18n.tr("Reset settings"), kind: "danger", closeOnTrigger: false,
             busy: dialog.busy && !dialog.dryRun && job.slow,
             enabled: !dialog.busy && !!dialog.preview && dialog.restored.length > 0 }]
    onActionTriggered: index => {
        if (!dialog.finished && index === 1)
            dialog.run(false);
    }
    // a closed dialog stops its preview, so a password prompt it asked for goes away too
    onClosed: {
        if (job.running && dialog.dryRun)
            job.cancel();
    }
    // a long file list scrolls, so what the dialog reports at its foot is brought into view
    onFailureChanged: if (dialog.failure !== "") Qt.callLater(() => body.reveal(status))
    Connections {
        target: job
        function onSlowChanged() {
            if (job.running && job.slow)
                Qt.callLater(() => body.reveal(status));
        }
    }

    function openFor(name) {
        dialog.account = String(name || "");
        dialog.preview = null;
        dialog.finished = false;
        dialog.failure = "";
        dialog.open();
        dialog.run(true);
    }

    function command(dry) {
        var c = dialog.account !== ""
            ? ["pkexec", "ryoku-host", "profile", "reset", "--account", dialog.account]
            : ["ryoku", "profile", "reset", "--json"];
        return dry ? c.concat(["--dry-run"]) : c;
    }

    function run(dry) {
        if (dialog.busy) {
            dialog.rerun = dry && dialog.dryRun;
            return;
        }
        dialog.dryRun = dry;
        dialog.failure = "";
        job.start(dialog.command(dry));
    }

    Job {
        id: job
        onFinished: (ok, message) => {
            if (dialog.rerun) {
                dialog.rerun = false;
                dialog.run(true);
                return;
            }
            if (!dialog.visible)
                return;
            if (!ok) {
                dialog.failure = message;
                return;
            }
            try {
                var v = JSON.parse(job.out.trim().split("\n").pop() || "{}");
                if (v.error)
                    throw new Error(String(v.error));
                if (dialog.dryRun) {
                    dialog.preview = v;
                } else {
                    dialog.finished = true;
                    dialog.completed();
                }
            } catch (e) {
                dialog.failure = String(e.message || e);
            }
        }
    }

    DialogBody {
        id: body
        width: parent ? parent.width : 0
        maxHeight: dialog.parent ? dialog.parent.height - 280 : 480

        Label {
            width: parent.width
            text: dialog.finished
                ? I18n.tr("The settings are reset. Projects, notes, engagements and VPN files were not touched.")
                : I18n.tr("Every file the profile manages goes back to how the profile ships it. Projects, notes, engagements, VPN files and other data stay.")
            typeRole: "body"
            inkRole: dialog.finished ? "fg" : "fg2"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }

        Label {
            visible: !!dialog.preview && !dialog.finished
            width: parent.width
            text: dialog.restored.length === 0 ? I18n.tr("Nothing differs from the profile, so there is nothing to reset.")
                : dialog.restored.length === 1 ? I18n.tr("One file changes; %1 already match.").arg(Number(dialog.preview.unchanged || 0))
                : I18n.tr("%1 files change; %2 already match.").arg(dialog.restored.length).arg(Number(dialog.preview.unchanged || 0))
            typeRole: "label"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }

        Column {
            visible: !!dialog.preview && !dialog.finished && dialog.restored.length > 0
            width: parent.width
            spacing: LithicTokens.space.s1
            Repeater {
                model: dialog.restored
                delegate: Label {
                    required property var modelData
                    width: parent ? parent.width : 0
                    text: String(modelData.path || modelData)
                    typeRole: "monoSm"
                    inkRole: "fg2"
                    wrapMode: Text.WrapAnywhere
                    elide: Text.ElideNone
                }
            }
        }

        JobStatus {
            id: status
            width: parent.width
            running: dialog.busy
            slow: job.slow
            lengthy: job.lengthy
            waitingText: dialog.dryRun
                ? (dialog.account !== "" ? I18n.tr("Checking which files differ. This may ask for an administrator's password.") : I18n.tr("Checking which files differ"))
                : (dialog.account !== "" ? I18n.tr("Resetting. This may ask for an administrator's password.") : I18n.tr("Resetting"))
            error: dialog.failure
            canCancel: dialog.dryRun
            onRetry: dialog.run(dialog.dryRun)
            onCancel: job.cancel()
        }
    }
}
