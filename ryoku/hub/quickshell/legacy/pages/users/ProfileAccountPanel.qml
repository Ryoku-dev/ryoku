pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../../Singletons"

Column {
    id: panel

    required property var profileStatus
    property string profileName: ""
    property var resetPreview: null
    property string failure: ""
    property bool busy: false
    property string action: ""

    signal changed()

    width: parent ? parent.width : implicitWidth
    spacing: Tokens.s5

    function list(value) { return value instanceof Array ? value : []; }

    function parseError(stdoutText, stderrText, fallback) {
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

    function previewReset() {
        if (panel.busy)
            return;
        panel.failure = "";
        panel.resetPreview = null;
        panel.busy = true;
        panel.action = "preview";
        resetProc.command = ["ryoku", "profile", "reset", "--dry-run", "--json"];
        resetProc.running = true;
    }

    function applyReset() {
        if (panel.busy || !panel.resetPreview)
            return;
        panel.failure = "";
        panel.busy = true;
        panel.action = "reset";
        resetProc.command = ["ryoku", "profile", "reset", "--json"];
        resetProc.running = true;
    }

    function saveCheckpoint() {
        if (panel.busy)
            return;
        var command = ["pkexec", "ryoku-host", "profile", "checkpoint", "save", "--account", String(panel.profileStatus.account || "")];
        var label = checkpointLabel.text.trim();
        if (label !== "")
            command.push("--label", label);
        panel.failure = "";
        panel.busy = true;
        panel.action = "checkpoint";
        checkpointProc.command = command;
        checkpointProc.running = true;
    }

    Process {
        id: resetProc
        environment: Spawn.env
        stdout: StdioCollector { id: resetOut }
        stderr: StdioCollector { id: resetErr }
        onExited: function(code) {
            panel.busy = false;
            if (code !== 0) {
                panel.failure = panel.parseError(resetOut.text, resetErr.text, I18n.tr("The profile settings could not be reset."));
                return;
            }
            try {
                var value = JSON.parse(String(resetOut.text || "{}"));
                if (value.error)
                    throw new Error(String(value.error));
                if (panel.action === "preview")
                    panel.resetPreview = value;
                else {
                    panel.resetPreview = null;
                    panel.changed();
                }
            } catch (e) {
                panel.failure = I18n.tr(String(e.message || e));
            }
        }
    }

    Process {
        id: checkpointProc
        environment: Spawn.env
        stdout: StdioCollector { id: checkpointOut }
        stderr: StdioCollector { id: checkpointErr }
        onExited: function(code) {
            panel.busy = false;
            if (code !== 0) {
                panel.failure = panel.parseError(checkpointOut.text, checkpointErr.text, I18n.tr("The checkpoint could not be saved."));
                return;
            }
            checkpointLabel.text = "";
            panel.changed();
        }
    }

    SettingCard {
        width: parent.width
        title: I18n.tr("PROFILE ACCOUNT")
        kana: "型"
        collapsible: false

        Item {
            width: parent.width
            implicitHeight: 88
            Column {
                anchors { left: parent.left; right: parent.right; margins: Tokens.s4; verticalCenter: parent.verticalCenter }
                spacing: Tokens.s1
                Text {
                    width: parent.width
                    text: panel.profileName
                    color: Tokens.ink
                    font.family: Tokens.display
                    font.pixelSize: Tokens.fValue
                    elide: Text.ElideRight
                }
                Text {
                    width: parent.width
                    text: I18n.tr("Version %1 · account %2")
                        .arg(String(panel.profileStatus.version || ""))
                        .arg(String(panel.profileStatus.account || ""))
                    color: Tokens.inkMuted
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fMicro
                    elide: Text.ElideRight
                }
            }
        }

        Rectangle { width: parent.width; height: 1; color: Tokens.lineSoft }

        Item {
            width: parent.width
            implicitHeight: resetColumn.implicitHeight + Tokens.s4 * 2
            Column {
                id: resetColumn
                anchors { left: parent.left; right: parent.right; top: parent.top; margins: Tokens.s4 }
                spacing: Tokens.s3
                Text { text: I18n.tr("RESET SETTINGS"); color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fMicro; font.weight: Font.Medium; font.letterSpacing: Tokens.trackLabel }
                Text { width: parent.width; text: I18n.tr("Restore every file managed by this profile. Projects, notes, engagements and VPN files are not touched."); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap }
                Column {
                    visible: !!panel.resetPreview
                    width: parent.width
                    spacing: Tokens.s1
                    Text { width: parent.width; text: I18n.tr("%1 files restored · %2 unchanged")
                            .arg(panel.resetPreview ? panel.list(panel.resetPreview.restored).length : 0)
                            .arg(Number(panel.resetPreview ? panel.resetPreview.unchanged || 0 : 0)); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap }
                    Repeater {
                        model: panel.resetPreview ? panel.list(panel.resetPreview.restored) : []
                        delegate: Text { required property var modelData; width: resetColumn.width; text: String(modelData.path || modelData); color: Tokens.ink; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny; wrapMode: Text.WrapAnywhere }
                    }
                }
                Row {
                    spacing: Tokens.s2
                    Btn { text: panel.busy && panel.action === "preview" ? I18n.tr("PREPARING") : I18n.tr("PREVIEW RESET"); armed: !panel.busy; onAct: panel.previewReset() }
                    Btn { visible: !!panel.resetPreview; text: panel.busy && panel.action === "reset" ? I18n.tr("RESETTING") : I18n.tr("RESET SETTINGS"); primary: true; armed: !panel.busy; onAct: panel.applyReset() }
                }
            }
        }

        Rectangle { width: parent.width; height: 1; color: Tokens.lineSoft }

        Item {
            width: parent.width
            implicitHeight: checkpointColumn.implicitHeight + Tokens.s4 * 2
            Column {
                id: checkpointColumn
                anchors { left: parent.left; right: parent.right; top: parent.top; margins: Tokens.s4 }
                spacing: Tokens.s3
                Text { text: I18n.tr("SAVE CHECKPOINT"); color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fMicro; font.weight: Font.Medium; font.letterSpacing: Tokens.trackLabel }
                Text { width: parent.width; text: I18n.tr("Save the current home settings before a lab or a major configuration change."); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap }
                Row {
                    width: parent.width
                    spacing: Tokens.s2
                    Field { id: checkpointLabel; width: parent.width - saveCheckpointButton.width - Tokens.s2; placeholder: I18n.tr("Optional label"); onAccepted: panel.saveCheckpoint() }
                    Btn { id: saveCheckpointButton; text: panel.busy && panel.action === "checkpoint" ? I18n.tr("SAVING") : I18n.tr("SAVE"); armed: !panel.busy; onAct: panel.saveCheckpoint() }
                }
            }
        }

        Rectangle { width: parent.width; height: 1; color: Tokens.lineSoft }

        Item {
            width: parent.width
            implicitHeight: isolationColumn.implicitHeight + Tokens.s4 * 2
            Column {
                id: isolationColumn
                anchors { left: parent.left; right: parent.right; top: parent.top; margins: Tokens.s4 }
                spacing: Tokens.s1
                Text { width: parent.width; text: I18n.tr("A profile is a separate account, not a sandbox."); color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall; font.weight: Font.Medium; wrapMode: Text.WordWrap }
                Text { width: parent.width; text: I18n.tr("Its menus, dock and default apps show only its own tools, but programs installed for the whole computer can still be started from its terminal. Apps it installs also appear in the other accounts on this computer. Files in other accounts stay private: each home folder is readable only by its owner."); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap }
            }
        }
    }

    Text {
        visible: panel.failure !== ""
        width: parent.width
        text: panel.failure
        color: Tokens.alert
        font.family: Tokens.ui
        font.pixelSize: Tokens.fSmall
        font.weight: Font.Medium
        wrapMode: Text.WordWrap
    }
}
