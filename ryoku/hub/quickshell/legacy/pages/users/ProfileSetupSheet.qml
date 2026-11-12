pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../../Singletons"

Popup {
    id: sheet

    property var profile: null
    property var existingStatus: null
    property int step: 0
    property var selectedComponents: []
    property var extras: []
    property var plan: null
    property var planArgs: []
    property var progress: []
    property var result: null
    property string failure: ""
    property bool busy: false

    signal completed()

    parent: Overlay.overlay
    anchors.centerIn: parent
    modal: true
    width: Math.min(780, parent ? parent.width - Tokens.s6 * 2 : 780)
    height: Math.min(760, parent ? parent.height - Tokens.s6 * 2 : 760)
    padding: 0
    closePolicy: sheet.busy ? Popup.NoAutoClose : Popup.CloseOnEscape | Popup.CloseOnPressOutside

    background: Rectangle {
        color: Tokens.paper
        border.width: Tokens.border
        border.color: Tokens.lineStrong
        radius: Tokens.radius
    }
    Overlay.modal: Rectangle { color: Qt.rgba(0, 0, 0, 0.6) }

    readonly property var manifest: sheet.profile ? (sheet.profile.manifest || ({})) : ({})
    readonly property var components: manifest.components instanceof Array ? manifest.components : []
    readonly property bool editing: sheet.existingStatus !== null
    readonly property bool blocked: !!sheet.plan && sheet.plan.blockers instanceof Array && sheet.plan.blockers.length > 0
    readonly property string profileName: String(manifest.name || (sheet.profile ? sheet.profile.name : "") || "")

    function componentGroups() {
        var groups = [];
        var byName = ({});
        for (var i = 0; i < sheet.components.length; i++) {
            var component = sheet.components[i];
            var groupName = String(component.group || I18n.tr("Other"));
            if (!byName[groupName]) {
                byName[groupName] = { name: groupName, components: [] };
                groups.push(byName[groupName]);
            }
            byName[groupName].components.push(component);
        }
        return groups;
    }

    function openFor(definition, status) {
        sheet.profile = definition;
        sheet.existingStatus = status || null;
        sheet.step = status ? 1 : 0;
        sheet.plan = null;
        sheet.result = null;
        sheet.progress = [];
        sheet.failure = "";
        sheet.busy = false;
        var definitionManifest = ((definition || {}).manifest || ({}));
        var accountSpec = definitionManifest.account || ({});
        accountField.text = status ? String(status.account || "")
            : String(accountSpec.suggestedName || "");
        passwordField.text = "";
        confirmField.text = "";
        extraField.text = "";

        var previous = status && status.packages instanceof Array ? status.packages : [];
        var selected = [];
        var extraPackages = [];
        for (var i = 0; i < sheet.components.length; i++) {
            var component = sheet.components[i];
            if (status ? previous.indexOf(component.name) >= 0 : String(component.tier || "") === "core")
                selected.push(String(component.name || ""));
        }
        if (status) {
            for (var j = 0; j < previous.length; j++) {
                var known = false;
                for (var k = 0; k < sheet.components.length; k++)
                    if (String(sheet.components[k].name || "") === String(previous[j])) {
                        known = true;
                        break;
                    }
                if (!known)
                    extraPackages.push(String(previous[j]));
            }
        }
        sheet.selectedComponents = selected;
        sheet.extras = extraPackages;
        open();
    }

    function validAccountName(name) {
        return /^[a-z_][a-z0-9_-]{0,31}$/.test(name);
    }

    function accountReady() {
        var name = accountField.text.trim();
        if (!sheet.validAccountName(name)) {
            sheet.failure = I18n.tr("Usernames must start with a lowercase letter or underscore and use at most 32 lowercase letters, numbers, underscores, or hyphens.");
            accountField.forceActiveFocus();
            return false;
        }
        if (!sheet.editing && passwordField.text === "") {
            sheet.failure = I18n.tr("Enter a password.");
            passwordField.forceActiveFocus();
            return false;
        }
        if (!sheet.editing && passwordField.text !== confirmField.text) {
            sheet.failure = I18n.tr("Passwords do not match.");
            confirmField.forceActiveFocus();
            return false;
        }
        sheet.failure = "";
        return true;
    }

    function toggleComponent(name) {
        var next = sheet.selectedComponents.slice();
        var index = next.indexOf(name);
        if (index >= 0)
            next.splice(index, 1);
        else
            next.push(name);
        sheet.selectedComponents = next;
        sheet.plan = null;
    }

    function addExtra() {
        var name = extraField.text.trim();
        if (!/^[a-z0-9@._+-]+$/.test(name)) {
            sheet.failure = I18n.tr("Enter a valid package name.");
            extraField.forceActiveFocus();
            return;
        }
        if (sheet.extras.indexOf(name) < 0)
            sheet.extras = sheet.extras.concat([name]);
        extraField.text = "";
        sheet.failure = "";
        sheet.plan = null;
    }

    function removeExtra(name) {
        var next = sheet.extras.slice();
        var index = next.indexOf(name);
        if (index >= 0)
            next.splice(index, 1);
        sheet.extras = next;
        sheet.plan = null;
    }

    function argsForPlan() {
        var args = ["--source", String(sheet.profile.source || ""), "--account", accountField.text.trim()];
        for (var i = 0; i < sheet.components.length; i++) {
            var component = sheet.components[i];
            var name = String(component.name || "");
            var selected = sheet.selectedComponents.indexOf(name) >= 0;
            if (String(component.tier || "") === "optional" && selected)
                args.push("--component", name);
            else if (String(component.tier || "") === "core" && !selected)
                args.push("--no-component", name);
        }
        for (var j = 0; j < sheet.extras.length; j++)
            args.push("--extra", String(sheet.extras[j]));
        return args;
    }

    function requestPlan() {
        if (!sheet.accountReady() || sheet.busy)
            return;
        sheet.failure = "";
        sheet.plan = null;
        sheet.planArgs = sheet.argsForPlan();
        planProc.command = ["ryoku-host", "profile", "plan"].concat(sheet.planArgs);
        sheet.busy = true;
        planProc.running = true;
    }

    function applyPlan() {
        if (!sheet.plan || sheet.blocked || sheet.busy)
            return;
        sheet.failure = "";
        sheet.progress = [];
        sheet.result = null;
        sheet.step = 3;
        sheet.busy = true;
        applyProc.command = ["pkexec", "ryoku-host", "profile", "apply"]
            .concat(sheet.planArgs).concat(["--digest", String(sheet.plan.digest || "")]);
        var accountPlan = sheet.plan.account || ({});
        applyProc.stdinEnabled = accountPlan.exists !== true;
        applyProc.running = true;
    }

    function parseApplyLine(raw) {
        var line = String(raw || "").trim();
        if (line === "")
            return;
        try {
            var value = JSON.parse(line);
            if (value.error) {
                sheet.failure = I18n.tr(String(value.error));
                return;
            }
            if (value.ok === true) {
                sheet.result = value;
                return;
            }
            if (value.stage)
                sheet.progress = sheet.progress.concat([value]);
        } catch (e) {
            sheet.failure = I18n.tr(line);
        }
    }

    function checkRows() {
        var check = sheet.result && sheet.result.check ? sheet.result.check : null;
        if (!check)
            return [];
        var rows = [];
        var failures = check.failures instanceof Array ? check.failures : [];
        var warnings = check.warnings instanceof Array ? check.warnings : [];
        for (var i = 0; i < failures.length; i++)
            rows.push({ warning: true, text: String(failures[i].detail || failures[i].kind || failures[i]) });
        for (var j = 0; j < warnings.length; j++)
            rows.push({ warning: false, text: String(warnings[j].detail || warnings[j].kind || warnings[j]) });
        if (rows.length === 0)
            rows.push({ warning: false, text: I18n.tr("The profile check passed.") });
        return rows;
    }

    Process {
        id: planProc
        environment: Spawn.env
        stdout: StdioCollector { id: planOut }
        stderr: StdioCollector { id: planErr }
        onExited: function(code) {
            sheet.busy = false;
            if (code !== 0) {
                sheet.failure = sheet.parseProcessError(planOut.text, planErr.text, I18n.tr("The profile plan could not be prepared."));
                return;
            }
            try {
                var value = JSON.parse(String(planOut.text || "{}"));
                if (value.error)
                    throw new Error(String(value.error));
                sheet.plan = value;
                sheet.step = 2;
            } catch (e) {
                sheet.failure = I18n.tr(String(e.message || e));
            }
        }
    }

    Process {
        id: applyProc
        environment: Spawn.env
        stdinEnabled: false
        stdout: SplitParser {
            splitMarker: "\n"
            onRead: line => sheet.parseApplyLine(line)
        }
        stderr: StdioCollector { id: applyErr }
        onStarted: {
            if (applyProc.stdinEnabled) {
                write(passwordField.text + "\n");
                passwordField.text = "";
                confirmField.text = "";
            }
        }
        onExited: function(code) {
            sheet.busy = false;
            applyProc.stdinEnabled = false;
            if (code === 0 && sheet.result) {
                sheet.step = 4;
                sheet.completed();
                return;
            }
            if (sheet.failure === "")
                sheet.failure = sheet.parseProcessError("", applyErr.text, I18n.tr("The profile could not be applied."));
        }
    }

    function parseProcessError(stdoutText, stderrText, fallback) {
        var text = String(stdoutText || "").trim();
        if (text !== "") {
            try {
                var value = JSON.parse(text);
                if (value && value.error)
                    return I18n.tr(String(value.error));
            } catch (e) {
            }
        }
        text = String(stderrText || "").trim();
        return text !== "" ? I18n.tr(text) : fallback;
    }

    Item {
        id: header
        anchors { left: parent.left; right: parent.right; top: parent.top }
        height: 88

        Column {
            anchors { left: parent.left; right: closeButton.left; leftMargin: Tokens.s5; rightMargin: Tokens.s4; verticalCenter: parent.verticalCenter }
            spacing: Tokens.s1
            Text {
                text: sheet.editing ? I18n.tr("EDIT PROFILE TOOLS") : I18n.tr("CREATE PROFILE ACCOUNT")
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fMicro
                font.weight: Font.Medium
                font.letterSpacing: Tokens.trackMark
            }
            Text {
                width: parent.width
                text: sheet.profileName
                color: Tokens.ink
                font.family: Tokens.display
                font.pixelSize: Tokens.fValue
                elide: Text.ElideRight
            }
        }
        Btn {
            id: closeButton
            anchors { right: parent.right; rightMargin: Tokens.s4; verticalCenter: parent.verticalCenter }
            compact: true
            text: I18n.tr("CLOSE")
            armed: !sheet.busy
            onAct: sheet.close()
        }
        Rectangle {
            anchors {
                left: parent.left
                right: parent.right
                bottom: parent.bottom
            }
            height: 1
            color: Tokens.lineSoft
        }
    }

    Row {
        id: stepRail
        anchors { left: parent.left; right: parent.right; top: header.bottom; leftMargin: Tokens.s5; rightMargin: Tokens.s5; topMargin: Tokens.s3 }
        spacing: Tokens.s2
        Repeater {
            model: [I18n.tr("ACCOUNT"), I18n.tr("TOOLS"), I18n.tr("REVIEW"), I18n.tr("APPLY"), I18n.tr("DONE")]
            delegate: Rectangle {
                required property string modelData
                required property int index
                width: (stepRail.width - stepRail.spacing * 4) / 5
                height: 26
                radius: Tokens.radius
                color: index === sheet.step ? Tokens.bone : "transparent"
                border.width: Tokens.border
                border.color: index === sheet.step ? Tokens.bone : Tokens.lineSoft
                Text {
                    anchors.centerIn: parent
                    text: parent.modelData
                    color: parent.index === sheet.step ? Tokens.inkOnBone : Tokens.inkFaint
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fTiny
                }
            }
        }
    }

    Flickable {
        id: body
        anchors {
            left: parent.left; right: parent.right
            top: stepRail.bottom; bottom: footer.top
            leftMargin: Tokens.s5; rightMargin: Tokens.s4
            topMargin: Tokens.s4; bottomMargin: Tokens.s3
        }
        contentWidth: width
        contentHeight: content.implicitHeight
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
        WheelScroll { }

        Column {
            id: content
            width: body.width - Tokens.s3
            spacing: Tokens.s5

            Column {
                visible: sheet.step === 0
                width: parent.width
                spacing: Tokens.s4
                Text {
                    width: parent.width
                    text: I18n.tr("Choose the separate standard account that will carry this profile.")
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                    wrapMode: Text.WordWrap
                }
                Column {
                    width: parent.width
                    spacing: Tokens.s1
                    Text {
                        text: I18n.tr("USER NAME")
                        color: Tokens.inkDim
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fMicro
                        font.letterSpacing: Tokens.trackLabel
                    }
                    Field { id: accountField; width: parent.width; tabular: true; placeholder: I18n.tr("hacker") }
                }
                Row {
                    visible: !sheet.editing
                    width: parent.width
                    spacing: Tokens.s3
                    Column {
                        width: (parent.width - Tokens.s3) / 2
                        spacing: Tokens.s1
                        Text { text: I18n.tr("PASSWORD"); color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fMicro; font.letterSpacing: Tokens.trackLabel }
                        Field { id: passwordField; width: parent.width; secret: true; placeholder: I18n.tr("Password") }
                    }
                    Column {
                        width: (parent.width - Tokens.s3) / 2
                        spacing: Tokens.s1
                        Text { text: I18n.tr("CONFIRM"); color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fMicro; font.letterSpacing: Tokens.trackLabel }
                        Field { id: confirmField; width: parent.width; secret: true; placeholder: I18n.tr("Repeat password"); onAccepted: if (sheet.accountReady()) sheet.step = 1 }
                    }
                }
                Text {
                    width: parent.width
                    text: I18n.tr("This is a standard account. It does not receive administrator rights.")
                    color: Tokens.inkFaint
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                    wrapMode: Text.WordWrap
                }
            }

            Column {
                visible: sheet.step === 1
                width: parent.width
                spacing: Tokens.s5
                Repeater {
                    model: sheet.componentGroups()
                    delegate: Column {
                        id: toolGroup
                        required property var modelData
                        width: content.width
                        spacing: Tokens.s2
                        Text {
                            text: I18n.tr(String(toolGroup.modelData.name || "").toUpperCase())
                            color: Tokens.inkDim
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fMicro
                            font.weight: Font.Medium
                            font.letterSpacing: Tokens.trackLabel
                        }
                        Repeater {
                            model: toolGroup.modelData.components || []
                            delegate: Item {
                                id: toolRow
                                required property var modelData
                                width: toolGroup.width
                                implicitHeight: 52
                                readonly property string componentName: String(modelData.name || "")
                                readonly property bool selected: sheet.selectedComponents.indexOf(componentName) >= 0
                                Column {
                                    anchors { left: parent.left; right: toolSwitch.left; rightMargin: Tokens.s3; verticalCenter: parent.verticalCenter }
                                    spacing: Tokens.s1
                                    Text { width: parent.width; text: toolRow.componentName; color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall; font.weight: Font.Medium; elide: Text.ElideRight }
                                    Text { width: parent.width; text: I18n.tr(String(toolRow.modelData.summary || "")); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fMicro; elide: Text.ElideRight }
                                }
                                Sw {
                                    id: toolSwitch
                                    anchors {
                                        right: parent.right
                                        verticalCenter: parent.verticalCenter
                                    }
                                    on: toolRow.selected
                                    onToggled: sheet.toggleComponent(toolRow.componentName)
                                }
                                Rectangle {
                                    anchors {
                                        left: parent.left
                                        right: parent.right
                                        bottom: parent.bottom
                                    }
                                    height: 1
                                    color: Tokens.lineSoft
                                }
                            }
                        }
                    }
                }
                Column {
                    width: parent.width
                    spacing: Tokens.s2
                    Text { text: I18n.tr("ADD A PACKAGE"); color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fMicro; font.letterSpacing: Tokens.trackLabel }
                    Row {
                        width: parent.width
                        spacing: Tokens.s2
                        Field { id: extraField; width: parent.width - addExtraButton.width - Tokens.s2; tabular: true; placeholder: I18n.tr("Package name"); onAccepted: sheet.addExtra() }
                        Btn { id: addExtraButton; text: I18n.tr("ADD"); onAct: sheet.addExtra() }
                    }
                    Flow {
                        width: parent.width
                        spacing: Tokens.s2
                        Repeater {
                            model: sheet.extras
                            delegate: Btn {
                                required property string modelData
                                compact: true
                                text: I18n.tr("REMOVE %1").arg(modelData.toUpperCase())
                                onAct: sheet.removeExtra(modelData)
                            }
                        }
                    }
                }
            }

            ProfilePlanView {
                visible: sheet.step === 2 && sheet.plan !== null
                width: parent.width
                plan: sheet.plan || ({})
            }

            Column {
                visible: sheet.step === 3
                width: parent.width
                spacing: Tokens.s3
                Text { text: sheet.busy ? I18n.tr("APPLYING PROFILE") : I18n.tr("PROFILE DID NOT FINISH"); color: Tokens.ink; font.family: Tokens.display; font.pixelSize: Tokens.fValue }
                Repeater {
                    model: sheet.progress
                    delegate: Item {
                        required property var modelData
                        width: content.width
                        implicitHeight: 46
                        Column {
                            anchors { left: parent.left; right: progressCount.left; rightMargin: Tokens.s3; verticalCenter: parent.verticalCenter }
                            spacing: Tokens.s1
                            Text { width: parent.width; text: I18n.tr(String(parent.parent.modelData.message || parent.parent.modelData.stage || "")); color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall; elide: Text.ElideRight }
                            Text { text: I18n.tr(String(parent.parent.modelData.stage || "").toUpperCase()); color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny }
                        }
                        Text {
                            id: progressCount
                            anchors {
                                right: parent.right
                                verticalCenter: parent.verticalCenter
                            }
                            text: String(modelData.done || 0) + " / " + String(modelData.total || 0)
                            color: Tokens.inkMuted
                            font.family: Tokens.mono
                            font.pixelSize: Tokens.fMicro
                        }
                        Rectangle {
                            anchors {
                                left: parent.left
                                right: parent.right
                                bottom: parent.bottom
                            }
                            height: 1
                            color: Tokens.lineSoft
                        }
                    }
                }
            }

            Column {
                visible: sheet.step === 4
                width: parent.width
                spacing: Tokens.s4
                Text { width: parent.width; text: I18n.tr("Log out, then choose %1 at the login screen.").arg(accountField.text.trim()); color: Tokens.ink; font.family: Tokens.display; font.pixelSize: Tokens.fValue; wrapMode: Text.WordWrap }
                Text { text: I18n.tr("PROFILE CHECK"); color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fMicro; font.weight: Font.Medium; font.letterSpacing: Tokens.trackLabel }
                Repeater {
                    model: sheet.checkRows()
                    delegate: Text {
                        required property var modelData
                        width: content.width
                        text: I18n.tr(String(modelData.text || ""))
                        color: modelData.warning ? Tokens.alert : Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                        wrapMode: Text.WordWrap
                    }
                }
            }

            Text {
                visible: sheet.failure !== ""
                width: parent.width
                text: sheet.failure
                color: Tokens.alert
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                font.weight: Font.Medium
                wrapMode: Text.WordWrap
            }
        }
    }

    Item {
        id: footer
        anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
        height: sheet.step === 2 ? isolation.implicitHeight + 66 : 62
        Rectangle {
            anchors {
                left: parent.left
                right: parent.right
                top: parent.top
            }
            height: 1
            color: Tokens.lineSoft
        }

        Column {
            id: isolation
            visible: sheet.step === 2
            anchors { left: parent.left; right: parent.right; top: parent.top; leftMargin: Tokens.s5; rightMargin: Tokens.s5; topMargin: Tokens.s3 }
            spacing: Tokens.s1
            Text { width: parent.width; text: I18n.tr("A profile is a separate account, not a sandbox."); color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall; font.weight: Font.Medium; wrapMode: Text.WordWrap }
            Text { width: parent.width; text: I18n.tr("Its menus, dock and default apps show only its own tools, but programs installed for the whole computer can still be started from its terminal. Apps it installs also appear in the other accounts on this computer. Files in other accounts stay private: each home folder is readable only by its owner."); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fMicro; wrapMode: Text.WordWrap }
        }

        Row {
            anchors { right: parent.right; rightMargin: Tokens.s5; bottom: parent.bottom; bottomMargin: Tokens.s3 }
            spacing: Tokens.s2
            Btn {
                visible: sheet.step > 0 && sheet.step < 3
                text: I18n.tr("BACK")
                armed: !sheet.busy
                onAct: {
                    sheet.failure = "";
                    sheet.step--;
                }
            }
            Btn {
                visible: sheet.step === 0
                text: I18n.tr("TOOLS")
                primary: true
                armed: !sheet.busy
                onAct: if (sheet.accountReady()) sheet.step = 1
            }
            Btn {
                visible: sheet.step === 1
                text: sheet.busy ? I18n.tr("PLANNING") : I18n.tr("REVIEW PLAN")
                primary: true
                armed: !sheet.busy
                onAct: sheet.requestPlan()
            }
            Btn {
                visible: sheet.step === 2
                text: sheet.editing ? I18n.tr("APPLY CHANGES") : I18n.tr("CREATE")
                primary: true
                armed: !sheet.busy && !sheet.blocked
                onAct: sheet.applyPlan()
            }
            Btn {
                visible: sheet.step === 3 && !sheet.busy
                text: I18n.tr("BACK TO REVIEW")
                onAct: sheet.step = 2
            }
            Btn {
                visible: sheet.step === 4
                text: I18n.tr("DONE")
                primary: true
                onAct: sheet.close()
            }
        }
    }
}
