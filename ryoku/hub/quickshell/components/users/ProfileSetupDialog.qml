pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Templates as T
import Lithic
import Ryoku.Ui.Singletons
import "users.js" as Users

// creates a profile account, or changes its tools: account, tools, review, apply, done
Dialog {
    id: dialog

    required property var store
    property var profile: null
    property var existing: null
    property int step: 0
    property var selected: []
    property var extras: []
    property var plan: null
    property var planArgs: []
    property var progress: []
    property var outcome: null
    property string error: ""
    property string failure: ""
    property string password: ""

    signal completed(string account)

    readonly property var manifest: dialog.profile ? (dialog.profile.manifest || ({})) : ({})
    readonly property var components: Users.list(dialog.manifest.components)
    readonly property bool editing: dialog.existing !== null
    readonly property bool busy: planJob.running || applyJob.running
    readonly property bool blocked: !!dialog.plan && Users.list(dialog.plan.blockers).length > 0
    readonly property string profileName: String(dialog.manifest.name || (dialog.profile ? dialog.profile.name : "") || "")
    readonly property var stepNames: [I18n.tr("Account"), I18n.tr("Tools"), I18n.tr("Review"), I18n.tr("Apply"), I18n.tr("Done")]
    readonly property real stepFraction: {
        var last = dialog.progress.length > 0 ? dialog.progress[dialog.progress.length - 1] : null;
        return last && Number(last.total || 0) > 0 ? Number(last.done || 0) / Number(last.total) : -1;
    }

    title: dialog.editing ? I18n.tr("Tools for %1").arg(dialog.existing ? String(dialog.existing.account || "") : "")
                          : I18n.tr("Create a %1 account").arg(dialog.profileName)
    dialogWidth: LithicTokens.size.catalogueDialogWidth
    closePolicy: dialog.busy ? T.Popup.NoAutoClose : (T.Popup.CloseOnEscape | T.Popup.CloseOnPressOutside)
    actions: {
        switch (dialog.step) {
        case 0:
            return [{ text: I18n.tr("Cancel"), kind: "quiet" },
                    { text: I18n.tr("Choose tools"), kind: "gold", closeOnTrigger: false }];
        case 1:
            return [{ text: dialog.editing ? I18n.tr("Cancel") : I18n.tr("Back"), kind: "quiet", enabled: !dialog.busy, closeOnTrigger: false },
                    { text: I18n.tr("Review changes"), kind: "gold", busy: planJob.running && planJob.slow, enabled: !dialog.busy, closeOnTrigger: false }];
        case 2:
            return [{ text: I18n.tr("Back"), kind: "quiet", enabled: !dialog.busy, closeOnTrigger: false },
                    { text: dialog.editing ? I18n.tr("Apply changes") : I18n.tr("Create account"), kind: "gold",
                      enabled: !dialog.busy && !dialog.blocked, closeOnTrigger: false }];
        case 3:
            return dialog.busy ? [] : [{ text: I18n.tr("Back to review"), kind: "quiet", closeOnTrigger: false }];
        }
        return [{ text: I18n.tr("Done"), kind: "gold" }];
    }
    onActionTriggered: index => {
        if (dialog.step === 0 && index === 1)
            dialog.toTools();
        else if ((dialog.step === 1 || dialog.step === 2) && index === 0)
            dialog.back();
        else if (dialog.step === 1 && index === 1)
            dialog.requestPlan();
        else if (dialog.step === 2 && index === 1)
            dialog.apply();
        else if (dialog.step === 3 && index === 0)
            dialog.step = 2;
    }

    function openFor(definition, status) {
        dialog.profile = definition;
        dialog.existing = status || null;
        dialog.step = status ? 1 : 0;
        dialog.plan = null;
        dialog.outcome = null;
        dialog.progress = [];
        dialog.error = "";
        dialog.failure = "";
        dialog.password = "";
        var spec = (definition && definition.manifest && definition.manifest.account) || ({});
        accountField.text = status ? String(status.account || "") : String(spec.suggestedName || "");
        passwordField.text = "";
        confirmField.text = "";
        var before = status ? Users.list(status.packages) : [];
        var comps = definition && definition.manifest ? Users.list(definition.manifest.components) : [];
        dialog.selected = comps.filter(c => status ? before.indexOf(c.name) >= 0 : String(c.tier || "") === "core").map(c => String(c.name || ""));
        dialog.extras = status ? before.filter(p => !comps.some(c => String(c.name || "") === String(p))).map(p => String(p)) : [];
        dialog.open();
        if (!status)
            accountField.forceActiveFocus();
    }

    function groups() {
        var out = [], by = {};
        dialog.components.forEach(c => {
            var name = String(c.group || "Other");
            if (!by[name]) {
                by[name] = { name: name, components: [] };
                out.push(by[name]);
            }
            by[name].components.push(c);
        });
        return out;
    }

    function accountReady() {
        var name = accountField.text.trim();
        var problem = Users.nameProblem(name);
        if (problem !== "") {
            dialog.error = problem === "empty" ? I18n.tr("Enter a user name.")
                : I18n.tr("Start the user name with a lowercase letter, then use up to 32 lowercase letters, numbers, hyphens or underscores.");
            accountField.forceActiveFocus();
            return false;
        }
        if (!dialog.editing && passwordField.text === "") {
            dialog.error = I18n.tr("Enter a password.");
            passwordField.forceActiveFocus();
            return false;
        }
        if (!dialog.editing && passwordField.text !== confirmField.text) {
            dialog.error = I18n.tr("The passwords do not match. Type the same password twice.");
            confirmField.forceActiveFocus();
            return false;
        }
        dialog.error = "";
        return true;
    }

    function toTools() {
        if (!dialog.accountReady())
            return;
        dialog.password = passwordField.text;
        dialog.step = 1;
    }

    // editing starts at the tools, so going back from them closes
    function back() {
        dialog.error = "";
        dialog.failure = "";
        if (dialog.step === 1 && dialog.editing)
            dialog.close();
        else
            dialog.step--;
    }

    function toggle(name) {
        var next = dialog.selected.filter(n => n !== name);
        if (next.length === dialog.selected.length)
            next.push(name);
        dialog.selected = next;
        dialog.plan = null;
    }

    function argsForPlan() {
        var args = ["--source", String(dialog.profile.source || ""), "--account", accountField.text.trim()];
        dialog.components.forEach(c => {
            var name = String(c.name || ""), on = dialog.selected.indexOf(name) >= 0;
            if (String(c.tier || "") === "optional" && on)
                args.push("--component", name);
            else if (String(c.tier || "") === "core" && !on)
                args.push("--no-component", name);
        });
        dialog.extras.forEach(p => args.push("--extra", String(p)));
        return args;
    }

    function requestPlan() {
        if (dialog.busy)
            return;
        dialog.failure = "";
        dialog.plan = null;
        dialog.planArgs = dialog.argsForPlan();
        planJob.start(["ryoku-host", "profile", "plan"].concat(dialog.planArgs));
    }

    function apply() {
        if (!dialog.plan || dialog.blocked || dialog.busy)
            return;
        dialog.failure = "";
        dialog.progress = [];
        dialog.outcome = null;
        dialog.step = 3;
        var needsPassword = !(dialog.plan.account && dialog.plan.account.exists === true);
        applyJob.start(["pkexec", "ryoku-host", "profile", "apply"].concat(dialog.planArgs).concat(["--digest", String(dialog.plan.digest || "")]),
                       needsPassword ? dialog.password : undefined);
    }

    function checkRows() {
        var check = dialog.outcome && dialog.outcome.check ? dialog.outcome.check : null;
        if (!check)
            return [];
        var rows = [];
        Users.list(check.failures).forEach(f => rows.push({ warning: true, text: String(f.detail || f.kind || f) }));
        Users.list(check.warnings).forEach(w => rows.push({ warning: false, text: String(w.detail || w.kind || w) }));
        return rows;
    }

    onClosed: {
        dialog.password = "";
        passwordField.text = "";
        confirmField.text = "";
    }

    Job {
        id: planJob
        onFinished: (ok, message) => {
            if (!ok) {
                dialog.failure = message;
                return;
            }
            try {
                var v = JSON.parse(planJob.out.trim().split("\n").pop() || "{}");
                if (v.error)
                    throw new Error(String(v.error));
                dialog.plan = v;
                dialog.step = 2;
            } catch (e) {
                dialog.failure = I18n.tr("The plan could not be read: %1").arg(String(e.message || e));
            }
        }
    }

    Job {
        id: applyJob
        onLine: value => {
            if (value.error)
                dialog.failure = String(value.error);
            else if (value.ok === true)
                dialog.outcome = value;
            else if (value.stage)
                dialog.progress = dialog.progress.concat([value]);
        }
        onFinished: (ok, message) => {
            if (ok && dialog.outcome) {
                dialog.password = "";
                dialog.step = 4;
                dialog.completed(accountField.text.trim());
                return;
            }
            if (dialog.failure === "")
                dialog.failure = ok ? I18n.tr("The profile finished without a result. Look at the account before using it.") : message;
        }
    }

    DialogBody {
        id: body
        width: parent ? parent.width : 0
        maxHeight: dialog.parent ? dialog.parent.height - 280 : 480

        Label {
            width: parent.width
            // editing has no account step, so it counts four
            text: I18n.tr("Step %1 of %2: %3").arg(dialog.editing ? dialog.step : dialog.step + 1).arg(dialog.editing ? 4 : 5).arg(dialog.stepNames[dialog.step])
            typeRole: "label"
            inkRole: "fg2"
        }

        // account
        Column {
            visible: dialog.step === 0
            width: parent.width
            spacing: LithicTokens.space.s4

            Label {
                width: parent.width
                text: I18n.tr("The profile goes into a separate standard account. It never gets administrator rights.")
                typeRole: "body"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            TextField {
                id: accountField
                width: Math.min(parent.width, LithicTokens.size.settingsControlMax)
                label: I18n.tr("User name")
                mono: true
                placeholderText: "hacker"
                onAccepted: passwordField.forceActiveFocus()
            }
            Row {
                width: parent.width
                spacing: LithicTokens.space.s3
                TextField {
                    id: passwordField
                    width: (parent.width - parent.spacing) / 2
                    label: I18n.tr("Password")
                    secret: true
                    onAccepted: confirmField.forceActiveFocus()
                }
                TextField {
                    id: confirmField
                    width: (parent.width - parent.spacing) / 2
                    label: I18n.tr("Type it again")
                    secret: true
                    onAccepted: dialog.toTools()
                }
            }
        }

        // tools
        Column {
            visible: dialog.step === 1
            width: parent.width
            spacing: LithicTokens.space.s4

            Repeater {
                model: dialog.step === 1 ? dialog.groups() : []
                delegate: Column {
                    id: toolGroup
                    required property var modelData
                    width: parent ? parent.width : 0
                    spacing: 0

                    Label {
                        width: parent.width
                        bottomPadding: LithicTokens.space.s1
                        text: I18n.tr(String(toolGroup.modelData.name || ""))
                        typeRole: "label"
                    }
                    Repeater {
                        model: toolGroup.modelData.components || []
                        delegate: SettingRow {
                            id: toolRow
                            required property var modelData
                            required property int index
                            readonly property string componentName: String(modelData.name || "")
                            width: parent ? parent.width : 0
                            first: index === 0
                            plane: Lithic.stone4
                            activatable: true
                            enabled: !dialog.busy
                            title: componentName
                            detail: I18n.tr(String(modelData.summary || ""))
                            onActivated: dialog.toggle(componentName)

                            Toggle {
                                checked: dialog.selected.indexOf(toolRow.componentName) >= 0
                                Accessible.name: toolRow.componentName
                                onToggled: dialog.toggle(toolRow.componentName)
                            }
                        }
                    }
                }
            }

            Column {
                width: parent.width
                spacing: LithicTokens.space.s1

                Label {
                    width: parent.width
                    text: I18n.tr("More packages")
                    typeRole: "label"
                }
                ListEditor {
                    width: parent.width
                    plane: Lithic.stone4
                    model: dialog.extras.map(p => ({ title: p }))
                    addText: I18n.tr("Add a package")
                    doneText: I18n.tr("Add")
                    emptyText: I18n.tr("Only the profile's own tools. Add any other package by its exact name.")
                    accessibleName: I18n.tr("More packages")
                    editor: Component {
                        TextField {
                            property var item
                            readonly property bool valid: /^[a-z0-9@._+-]+$/.test(text.trim())
                            function value() { return { title: text.trim() }; }
                            width: parent ? parent.width : 0
                            label: I18n.tr("Package name")
                            mono: true
                            text: item && item.title ? item.title : ""
                            errorText: text.length > 0 && !valid ? I18n.tr("Use the package's exact name, in lowercase") : ""
                        }
                    }
                    onAdded: item => {
                        if (dialog.extras.indexOf(item.title) < 0)
                            dialog.extras = dialog.extras.concat([item.title]);
                        dialog.plan = null;
                    }
                    onEdited: (index, item) => {
                        var next = dialog.extras.slice();
                        next[index] = item.title;
                        dialog.extras = next.filter((p, i) => next.indexOf(p) === i);
                        dialog.plan = null;
                    }
                    onRemoved: index => {
                        dialog.extras = dialog.extras.filter((p, i) => i !== index);
                        dialog.plan = null;
                    }
                }
            }
        }

        // review
        ProfilePlanView {
            visible: dialog.step === 2 && dialog.plan !== null
            width: parent.width
            plan: dialog.plan || ({})
        }

        Column {
            visible: dialog.step === 2
            width: parent.width
            spacing: LithicTokens.space.s1
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

        // apply
        Column {
            visible: dialog.step === 3
            width: parent.width
            spacing: LithicTokens.space.s3

            Label {
                width: parent.width
                text: applyJob.running ? (dialog.editing ? I18n.tr("Changing the tools") : I18n.tr("Creating the account"))
                    : I18n.tr("The profile did not finish")
                typeRole: "body"
                inkRole: "fg"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            JobStatus {
                width: parent.width
                running: applyJob.running
                slow: applyJob.slow
                lengthy: applyJob.lengthy
                progress: dialog.stepFraction
                waitingText: dialog.progress.length === 0 ? I18n.tr("Waiting for an administrator's password") : I18n.tr("Working")
                // once a step runs as root it cannot be stopped part way
                canCancel: dialog.progress.length === 0
                onCancel: applyJob.cancel()
            }
            Label {
                visible: applyJob.running && dialog.progress.length > 0
                width: parent.width
                text: I18n.tr("Installing cannot be stopped part way. Keep this window open until it finishes.")
                typeRole: "detail"
                inkRole: "fg3"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Repeater {
                model: dialog.progress
                delegate: Row {
                    required property var modelData
                    width: parent ? parent.width : 0
                    spacing: LithicTokens.space.s3
                    Label {
                        width: parent.width - count.implicitWidth - LithicTokens.space.s3
                        text: I18n.tr(String(modelData.message || modelData.stage || ""))
                        typeRole: "detail"
                        inkRole: "fg"
                        wrapMode: Text.Wrap
                        elide: Text.ElideNone
                    }
                    Label {
                        id: count
                        text: String(modelData.done || 0) + " / " + String(modelData.total || 0)
                        typeRole: "detail"
                        numeric: true
                    }
                }
            }
        }

        // done
        Column {
            visible: dialog.step === 4
            width: parent.width
            spacing: LithicTokens.space.s3

            Label {
                width: parent.width
                text: dialog.editing ? I18n.tr("%1 now has the tools you chose.").arg(accountField.text.trim())
                    : I18n.tr("Sign out, then choose %1 at the sign-in screen.").arg(accountField.text.trim())
                typeRole: "body"
                inkRole: "fg"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Label {
                width: parent.width
                text: dialog.checkRows().length === 0 ? I18n.tr("The profile check passed.") : I18n.tr("The profile check found:")
                typeRole: "body"
                inkRole: "fg2"
            }
            Repeater {
                model: dialog.checkRows()
                delegate: Label {
                    required property var modelData
                    width: parent ? parent.width : 0
                    text: String(modelData.text || "")
                    typeRole: "detail"
                    ink: modelData.warning ? Lithic.jasper : Lithic.fg2
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }
            }
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
            running: planJob.running
            slow: planJob.slow
            lengthy: planJob.lengthy
            waitingText: I18n.tr("Working out what changes")
            error: dialog.failure
            onRetry: dialog.step === 3 ? dialog.apply() : dialog.requestPlan()
            onCancel: planJob.cancel()
        }
    }
}
