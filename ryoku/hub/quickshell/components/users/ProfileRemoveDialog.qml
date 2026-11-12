pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Templates as T
import Lithic
import Ryoku.Ui.Singletons
import "users.js" as Users

// takes a profile's tools away and archives, keeps or erases its account, by a reviewed plan
Dialog {
    id: dialog

    property var status: null
    property string profileName: ""
    property string mode: "archive"
    property var plan: null
    property var progress: []
    property bool finished: false
    property string failure: ""
    property bool removing: false
    property bool replan: false

    signal completed()

    readonly property string account: dialog.status ? String(dialog.status.account || "") : ""
    readonly property bool busy: planJob.running || removeJob.running
    readonly property var modes: ["archive", "keep", "erase"]
    readonly property real fraction: {
        var last = dialog.progress.length > 0 ? dialog.progress[dialog.progress.length - 1] : null;
        return last && Number(last.total || 0) > 0 ? Number(last.done || 0) / Number(last.total) : -1;
    }

    function list(v) { return Users.list(v); }

    title: dialog.finished ? I18n.tr("Profile removed") : I18n.tr("Remove the %1 profile from %2?").arg(dialog.profileName).arg(dialog.account)
    dialogWidth: 640
    closePolicy: removeJob.running ? T.Popup.NoAutoClose : (T.Popup.CloseOnEscape | T.Popup.CloseOnPressOutside)
    actions: dialog.finished
        ? [{ text: I18n.tr("Done"), kind: "gold" }]
        : [{ text: I18n.tr("Cancel"), kind: "quiet", enabled: !removeJob.running },
           { text: I18n.tr("Remove profile"), kind: "danger", closeOnTrigger: false,
             busy: removeJob.running && removeJob.slow,
             enabled: !dialog.busy && !!dialog.plan && (dialog.mode !== "erase" || confirmField.text.trim() === dialog.account) }]
    onActionTriggered: index => {
        if (!dialog.finished && index === 1)
            dialog.remove();
    }

    function openFor(status, name) {
        dialog.status = status;
        dialog.profileName = name;
        dialog.mode = "archive";
        modeChoice.currentIndex = 0;
        dialog.plan = null;
        dialog.progress = [];
        dialog.finished = false;
        dialog.failure = "";
        dialog.removing = false;
        confirmField.text = "";
        dialog.open();
        dialog.requestPlan();
    }

    function optionArgs() {
        return dialog.mode === "keep" ? ["--keep-account"] : dialog.mode === "erase" ? ["--erase-files"] : [];
    }

    // a new choice while a plan is being worked out asks again once that one stops
    function requestPlan() {
        if (removeJob.running)
            return;
        dialog.failure = "";
        dialog.plan = null;
        dialog.removing = false;
        if (planJob.running) {
            dialog.replan = true;
            planJob.cancel();
            return;
        }
        planJob.start(["ryoku-host", "profile", "plan-remove", "--account", dialog.account].concat(dialog.optionArgs()));
    }

    function remove() {
        if (!dialog.plan || dialog.busy)
            return;
        dialog.failure = "";
        dialog.progress = [];
        dialog.removing = true;
        removeJob.ok = false;
        removeJob.start(["pkexec", "ryoku-host", "profile", "remove", "--account", dialog.account]
            .concat(dialog.optionArgs()).concat(["--digest", String(dialog.plan.digest || "")]));
    }

    // the plan scrolls, so what the dialog reports at its foot is brought into view
    onFailureChanged: if (dialog.failure !== "") Qt.callLater(() => body.reveal(status))
    onProgressChanged: if (dialog.progress.length > 0) Qt.callLater(() => body.reveal(removal))
    Connections {
        target: removeJob
        function onSlowChanged() {
            if (removeJob.running && removeJob.slow)
                Qt.callLater(() => body.reveal(removal));
        }
        function onLengthyChanged() {
            if (removeJob.running && removeJob.lengthy)
                Qt.callLater(() => body.reveal(removal));
        }
    }

    Job {
        id: planJob
        onFinished: (ok, message) => {
            if (dialog.replan) {
                dialog.replan = false;
                dialog.requestPlan();
                return;
            }
            if (!ok) {
                dialog.failure = message;
                return;
            }
            try {
                var v = JSON.parse(planJob.out.trim().split("\n").pop() || "{}");
                if (v.error)
                    throw new Error(String(v.error));
                dialog.plan = v;
            } catch (e) {
                dialog.failure = String(e.message || e);
            }
        }
    }

    Job {
        id: removeJob
        property bool ok: false
        onLine: value => {
            if (value.error)
                dialog.failure = String(value.error);
            else if (value.ok === true)
                removeJob.ok = true;
            else if (value.stage)
                dialog.progress = dialog.progress.concat([value]);
        }
        onFinished: (ok, message) => {
            if (ok && removeJob.ok) {
                dialog.finished = true;
                dialog.completed();
                return;
            }
            if (dialog.failure === "")
                dialog.failure = ok ? I18n.tr("The removal finished without a result. Check the account list.") : message;
            dialog.completed();
        }
    }

    component PlanLine: Column {
        property string title: ""
        property string detail: ""
        property bool literal: false
        width: parent ? parent.width : 0
        spacing: LithicTokens.size.settingsDetailGap
        Label {
            width: parent.width
            text: parent.title
            typeRole: parent.literal ? "monoSm" : "body"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Label {
            visible: text.length > 0
            width: parent.width
            text: parent.detail
            typeRole: "detail"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
    }

    DialogBody {
        id: body
        width: parent ? parent.width : 0
        maxHeight: dialog.parent ? dialog.parent.height - 280 : 480

        Column {
            visible: !dialog.finished
            width: parent.width
            spacing: LithicTokens.space.s2

            Label {
                text: I18n.tr("The account and its files")
                typeRole: "label"
            }
            Segmented {
                id: modeChoice
                model: [I18n.tr("Archive the home"), I18n.tr("Keep the account"), I18n.tr("Erase the home")]
                currentIndex: dialog.modes.indexOf(dialog.mode)
                enabled: !removeJob.running
                accessibleName: I18n.tr("The account and its files")
                onActivated: index => {
                    dialog.mode = dialog.modes[index];
                    dialog.requestPlan();
                }
            }
            Label {
                width: parent.width
                text: dialog.mode === "keep" ? I18n.tr("The account stays as a plain standard account; only the profile's tools and managed files go.")
                    : dialog.mode === "erase" ? I18n.tr("The account and its home folder are deleted for good.")
                    : I18n.tr("The account is deleted and its home folder moves to the archive of removed accounts.")
                typeRole: "detail"
                ink: dialog.mode === "erase" ? Lithic.jasper : Lithic.fg2
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            TextField {
                id: confirmField
                visible: dialog.mode === "erase"
                width: Math.min(parent.width, LithicTokens.size.settingsControlMax)
                label: I18n.tr("Type %1 to confirm").arg(dialog.account)
                mono: true
                placeholderText: dialog.account
                enabled: !removeJob.running
            }
        }

        Column {
            visible: !!dialog.plan && !dialog.finished
            width: parent.width
            spacing: LithicTokens.space.s3

            PlanLine {
                readonly property var a: dialog.plan ? (dialog.plan.account || ({})) : ({})
                title: a.archive ? I18n.tr("Home folder archived to %1").arg(String(a.archive))
                    : dialog.mode === "keep" ? I18n.tr("Account kept: %1").arg(String(a.home || ""))
                    : I18n.tr("Account deleted: %1").arg(String(a.home || ""))
            }
            PlanLine {
                readonly property var f: dialog.plan ? (dialog.plan.files || ({})) : ({})
                title: I18n.tr("%1 files the profile manages").arg(Number(f.managed || 0))
                detail: I18n.tr("Only files the profile recorded are part of the removal.")
            }
            PlanLine {
                visible: dialog.plan && dialog.list(dialog.plan.userData).length > 0
                title: dialog.mode === "erase" ? I18n.tr("Data erased") : dialog.mode === "archive" ? I18n.tr("Data archived with the home") : I18n.tr("Data kept in the account")
                detail: dialog.plan ? dialog.list(dialog.plan.userData).join(", ") : ""
            }
            Repeater {
                model: dialog.plan ? dialog.list(dialog.plan.packages) : []
                delegate: PlanLine {
                    required property var modelData
                    title: String(modelData.name || "")
                    literal: true
                    detail: I18n.tr("Removed, frees %1").arg(Users.size(modelData.installed || 0))
                }
            }
            Repeater {
                model: dialog.plan ? dialog.list(dialog.plan.keptPackages) : []
                delegate: PlanLine {
                    required property var modelData
                    title: String(modelData.name || "")
                    literal: true
                    detail: String(modelData.reason || "") !== "" ? I18n.tr("Kept: %1").arg(String(modelData.reason)) : I18n.tr("Kept")
                }
            }
            Repeater {
                model: dialog.plan ? dialog.list(dialog.plan.repos) : []
                delegate: PlanLine {
                    required property var modelData
                    title: String(modelData.id || "")
                    literal: true
                    detail: [Users.planWord(modelData.action, I18n), String(modelData.reason || "")].filter(s => s !== "").join(": ")
                }
            }
            Repeater {
                model: dialog.plan ? dialog.list(dialog.plan.system) : []
                delegate: PlanLine {
                    required property var modelData
                    title: String(modelData.path || modelData.kind || "")
                    literal: true
                    detail: [Users.planWord(modelData.action, I18n), Users.planWord(modelData.kind, I18n)].filter(s => s !== "").join(": ")
                }
            }
        }

        Column {
            id: removal
            visible: dialog.removing && !dialog.finished
            width: parent.width
            spacing: LithicTokens.space.s2

            JobStatus {
                width: parent.width
                running: removeJob.running
                slow: removeJob.slow
                lengthy: removeJob.lengthy
                progress: dialog.fraction
                waitingText: dialog.progress.length === 0 ? I18n.tr("Waiting for an administrator's password") : I18n.tr("Working")
                // once a step runs as root it cannot be stopped part way
                canCancel: dialog.progress.length === 0
                onCancel: removeJob.cancel()
            }
            Repeater {
                model: dialog.progress
                delegate: PlanLine {
                    required property var modelData
                    title: I18n.tr(String(modelData.message || modelData.stage || ""))
                    detail: String(modelData.done || 0) + " / " + String(modelData.total || 0)
                }
            }
        }

        Label {
            visible: dialog.finished
            width: parent.width
            text: I18n.tr("%1 no longer has the %2 profile.").arg(dialog.account).arg(dialog.profileName)
            typeRole: "body"
            inkRole: "fg"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }

        JobStatus {
            id: status
            width: parent.width
            running: planJob.running
            slow: planJob.slow
            lengthy: planJob.lengthy
            waitingText: I18n.tr("Working out what goes")
            error: dialog.failure
            // after a failed removal Retry tries the removal again; after a failed read, the plan
            onRetry: dialog.removing && !!dialog.plan ? dialog.remove() : dialog.requestPlan()
            onCancel: planJob.cancel()
        }
    }
}
