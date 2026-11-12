pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../../Singletons"

Popup {
    id: dialog

    property string account: ""
    property bool loggedIn: false
    property var checkpoints: []
    property string failure: ""
    property bool busy: false
    property string action: ""

    signal changed()

    parent: Overlay.overlay
    anchors.centerIn: parent
    modal: true
    width: Math.min(620, parent ? parent.width - Tokens.s6 * 2 : 620)
    height: Math.min(650, parent ? parent.height - Tokens.s6 * 2 : 650)
    padding: 0
    closePolicy: dialog.busy ? Popup.NoAutoClose : Popup.CloseOnEscape | Popup.CloseOnPressOutside

    background: Rectangle {
        color: Tokens.paper
        border.width: Tokens.border
        border.color: Tokens.lineStrong
        radius: Tokens.radius
    }
    Overlay.modal: Rectangle { color: Qt.rgba(0, 0, 0, 0.6) }

    function openFor(name, active) {
        dialog.account = name;
        dialog.loggedIn = active;
        dialog.checkpoints = [];
        dialog.failure = "";
        labelField.text = "";
        open();
        refresh();
    }

    function refresh() {
        if (dialog.busy || dialog.account === "")
            return;
        dialog.failure = "";
        dialog.busy = true;
        dialog.action = "list";
        actionProc.command = ["ryoku-host", "profile", "checkpoint", "list", "--account", dialog.account];
        actionProc.running = true;
    }

    function saveCheckpoint() {
        if (dialog.busy)
            return;
        var command = ["pkexec", "ryoku-host", "profile", "checkpoint", "save", "--account", dialog.account];
        var label = labelField.text.trim();
        if (label !== "")
            command.push("--label", label);
        dialog.failure = "";
        dialog.busy = true;
        dialog.action = "save";
        actionProc.command = command;
        actionProc.running = true;
    }

    function revertCheckpoint(id) {
        if (dialog.busy || dialog.loggedIn)
            return;
        dialog.failure = "";
        dialog.busy = true;
        dialog.action = "revert";
        actionProc.command = ["pkexec", "ryoku-host", "profile", "checkpoint", "revert", "--account", dialog.account, "--id", id];
        actionProc.running = true;
    }

    function deleteCheckpoint(id) {
        if (dialog.busy)
            return;
        dialog.failure = "";
        dialog.busy = true;
        dialog.action = "delete";
        actionProc.command = ["pkexec", "ryoku-host", "profile", "checkpoint", "delete", "--account", dialog.account, "--id", id];
        actionProc.running = true;
    }

    function parseError(stdoutText, stderrText) {
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
        return text !== "" ? I18n.tr(text) : I18n.tr("The checkpoint action could not be completed.");
    }

    Process {
        id: actionProc
        environment: Spawn.env
        stdout: StdioCollector { id: actionOut }
        stderr: StdioCollector { id: actionErr }
        onExited: function(code) {
            dialog.busy = false;
            if (code !== 0) {
                dialog.failure = dialog.parseError(actionOut.text, actionErr.text);
                return;
            }
            var value = ({});
            try {
                value = JSON.parse(String(actionOut.text || "{}"));
                if (value.error)
                    throw new Error(String(value.error));
            } catch (e) {
                dialog.failure = I18n.tr(String(e.message || e));
                return;
            }
            if (dialog.action === "list") {
                dialog.checkpoints = value.checkpoints instanceof Array ? value.checkpoints : [];
                return;
            }
            labelField.text = "";
            dialog.changed();
            dialog.refresh();
        }
    }

    Item {
        id: header
        anchors { left: parent.left; right: parent.right; top: parent.top }
        height: 88
        Column {
            anchors { left: parent.left; right: closeButton.left; leftMargin: Tokens.s5; rightMargin: Tokens.s4; verticalCenter: parent.verticalCenter }
            spacing: Tokens.s1
            Text { text: I18n.tr("CHECKPOINTS"); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fMicro; font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark }
            Text { width: parent.width; text: I18n.tr("Settings for %1").arg(dialog.account); color: Tokens.ink; font.family: Tokens.display; font.pixelSize: Tokens.fValue; elide: Text.ElideRight }
        }
        Btn {
            id: closeButton
            anchors {
                right: parent.right
                rightMargin: Tokens.s4
                verticalCenter: parent.verticalCenter
            }
            compact: true
            text: I18n.tr("CLOSE")
            armed: !dialog.busy
            onAct: dialog.close()
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

    Column {
        id: saveArea
        anchors { left: parent.left; right: parent.right; top: header.bottom; leftMargin: Tokens.s5; rightMargin: Tokens.s5; topMargin: Tokens.s4 }
        spacing: Tokens.s2
        Text { text: I18n.tr("SAVE CHECKPOINT"); color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fMicro; font.weight: Font.Medium; font.letterSpacing: Tokens.trackLabel }
        Row {
            width: parent.width
            spacing: Tokens.s2
            Field { id: labelField; width: parent.width - saveButton.width - Tokens.s2; placeholder: I18n.tr("Optional label"); onAccepted: dialog.saveCheckpoint() }
            Btn { id: saveButton; text: dialog.busy && dialog.action === "save" ? I18n.tr("SAVING") : I18n.tr("SAVE"); armed: !dialog.busy; onAct: dialog.saveCheckpoint() }
        }
        Text {
            width: parent.width
            text: I18n.tr("Reverting restores profile-managed settings. User data folders such as notes, engagements and VPN files are kept.")
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            wrapMode: Text.WordWrap
        }
        Text {
            width: parent.width
            text: dialog.loggedIn
                ? I18n.tr("Sign out of this account before reverting a checkpoint.")
                : I18n.tr("The account must stay signed out while a checkpoint is reverted.")
            color: dialog.loggedIn ? Tokens.alert : Tokens.inkFaint
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            wrapMode: Text.WordWrap
        }
    }

    Flickable {
        id: list
        anchors { left: parent.left; right: parent.right; top: saveArea.bottom; bottom: footer.top; leftMargin: Tokens.s5; rightMargin: Tokens.s4; topMargin: Tokens.s4; bottomMargin: Tokens.s3 }
        contentWidth: width
        contentHeight: rows.implicitHeight
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
        WheelScroll { }

        Column {
            id: rows
            width: list.width - Tokens.s3
            spacing: Tokens.s1
            Text {
                visible: !dialog.busy && dialog.checkpoints.length === 0
                width: parent.width
                text: I18n.tr("No checkpoints have been saved.")
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
            }
            Repeater {
                model: dialog.checkpoints
                delegate: Item {
                    id: row
                    required property var modelData
                    width: rows.width
                    implicitHeight: 58
                    Column {
                        anchors { left: parent.left; right: actions.left; rightMargin: Tokens.s3; verticalCenter: parent.verticalCenter }
                        spacing: Tokens.s1
                        Text { width: parent.width; text: String(row.modelData.label || row.modelData.id || ""); color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall; font.weight: Font.Medium; elide: Text.ElideRight }
                        Text { width: parent.width; text: String(row.modelData.createdAt || row.modelData.id || ""); color: Tokens.inkMuted; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny; elide: Text.ElideRight }
                    }
                    Row {
                        id: actions
                        anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                        spacing: Tokens.s2
                        Btn { compact: true; text: I18n.tr("REVERT"); armed: !dialog.busy && !dialog.loggedIn; onAct: dialog.revertCheckpoint(String(row.modelData.id || "")) }
                        Btn { compact: true; text: I18n.tr("DELETE"); armed: !dialog.busy; onAct: dialog.deleteCheckpoint(String(row.modelData.id || "")) }
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
            Text {
                visible: dialog.failure !== ""
                width: parent.width
                text: dialog.failure
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
        height: 54
        Rectangle {
            anchors {
                left: parent.left
                right: parent.right
                top: parent.top
            }
            height: 1
            color: Tokens.lineSoft
        }
        Text {
            anchors { left: parent.left; leftMargin: Tokens.s5; verticalCenter: parent.verticalCenter }
            text: dialog.busy ? I18n.tr("WORKING") : I18n.tr("%1 CHECKPOINTS").arg(dialog.checkpoints.length)
            color: Tokens.inkFaint
            font.family: Tokens.mono
            font.pixelSize: Tokens.fTiny
        }
    }
}
