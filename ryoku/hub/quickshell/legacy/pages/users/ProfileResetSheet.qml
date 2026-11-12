pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../../Singletons"

Popup {
    id: sheet

    property string account: ""
    property var preview: null
    property string failure: ""
    property bool busy: false
    property bool done: false

    signal completed()

    parent: Overlay.overlay
    anchors.centerIn: parent
    modal: true
    width: Math.min(560, parent ? parent.width - Tokens.s6 * 2 : 560)
    height: Math.min(600, parent ? parent.height - Tokens.s6 * 2 : 600)
    padding: 0
    closePolicy: sheet.busy ? Popup.NoAutoClose : Popup.CloseOnEscape | Popup.CloseOnPressOutside

    background: Rectangle {
        color: Tokens.paper
        border.width: Tokens.border
        border.color: Tokens.lineStrong
        radius: Tokens.radius
    }
    Overlay.modal: Rectangle { color: Qt.rgba(0, 0, 0, 0.6) }

    function list(value) { return value instanceof Array ? value : []; }

    function openFor(name) {
        sheet.account = name;
        sheet.preview = null;
        sheet.failure = "";
        sheet.done = false;
        open();
        runReset(true);
    }

    function runReset(dryRun) {
        if (sheet.busy)
            return;
        sheet.failure = "";
        sheet.busy = true;
        resetProc.dryRun = dryRun;
        resetProc.command = ["pkexec", "ryoku-host", "profile", "reset", "--account", sheet.account]
            .concat(dryRun ? ["--dry-run"] : []);
        resetProc.running = true;
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
        return text !== "" ? I18n.tr(text) : I18n.tr("The profile settings could not be reset.");
    }

    Process {
        id: resetProc
        property bool dryRun: true
        environment: Spawn.env
        stdout: StdioCollector { id: resetOut }
        stderr: StdioCollector { id: resetErr }
        onExited: function(code) {
            sheet.busy = false;
            if (code !== 0) {
                sheet.failure = sheet.parseError(resetOut.text, resetErr.text);
                return;
            }
            try {
                var value = JSON.parse(String(resetOut.text || "{}"));
                if (value.error)
                    throw new Error(String(value.error));
                if (resetProc.dryRun)
                    sheet.preview = value;
                else {
                    sheet.done = true;
                    sheet.completed();
                }
            } catch (e) {
                sheet.failure = I18n.tr(String(e.message || e));
            }
        }
    }

    Item {
        id: header
        anchors { left: parent.left; right: parent.right; top: parent.top }
        height: 88
        Column {
            anchors { left: parent.left; right: closeButton.left; leftMargin: Tokens.s5; rightMargin: Tokens.s4; verticalCenter: parent.verticalCenter }
            spacing: Tokens.s1
            Text { text: I18n.tr("RESET PROFILE SETTINGS"); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fMicro; font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark }
            Text { width: parent.width; text: I18n.tr("Restore %1's managed files").arg(sheet.account); color: Tokens.ink; font.family: Tokens.display; font.pixelSize: Tokens.fValue; elide: Text.ElideRight }
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

    Flickable {
        id: body
        anchors { left: parent.left; right: parent.right; top: header.bottom; bottom: footer.top; margins: Tokens.s5; rightMargin: Tokens.s4 }
        contentWidth: width
        contentHeight: content.implicitHeight
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
        WheelScroll { }

        Column {
            id: content
            width: body.width - Tokens.s3
            spacing: Tokens.s4
            Text {
                width: parent.width
                text: I18n.tr("Every profile-managed setting is restored. Projects, notes, engagements, VPN files and other user data are not touched.")
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                wrapMode: Text.WordWrap
            }
            Text { visible: sheet.busy && !sheet.preview; text: I18n.tr("Preparing a dry-run preview…"); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall }
            Column {
                visible: !!sheet.preview
                width: parent.width
                spacing: Tokens.s3
                Text { text: I18n.tr("RESTORED"); color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fMicro; font.weight: Font.Medium; font.letterSpacing: Tokens.trackLabel }
                Text {
                    width: parent.width
                    text: I18n.tr("%1 files restored · %2 unchanged")
                        .arg(sheet.preview ? sheet.list(sheet.preview.restored).length : 0)
                        .arg(Number(sheet.preview ? sheet.preview.unchanged || 0 : 0))
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                    wrapMode: Text.WordWrap
                }
                Repeater {
                    model: sheet.preview ? sheet.list(sheet.preview.restored) : []
                    delegate: Text { required property var modelData; width: content.width; text: String(modelData.path || modelData); color: Tokens.ink; font.family: Tokens.mono; font.pixelSize: Tokens.fMicro; wrapMode: Text.WrapAnywhere }
                }
                Text { visible: sheet.preview && sheet.list(sheet.preview.restored).length === 0; text: I18n.tr("No managed files need restoring."); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall }
            }
            Text { visible: sheet.done; width: parent.width; text: I18n.tr("Profile settings were reset. User data was not touched."); color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap }
            Text { visible: sheet.failure !== ""; width: parent.width; text: sheet.failure; color: Tokens.alert; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall; font.weight: Font.Medium; wrapMode: Text.WordWrap }
        }
    }

    Item {
        id: footer
        anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
        height: 62
        Rectangle {
            anchors {
                left: parent.left
                right: parent.right
                top: parent.top
            }
            height: 1
            color: Tokens.lineSoft
        }
        Btn {
            anchors {
                left: parent.left
                leftMargin: Tokens.s5
                verticalCenter: parent.verticalCenter
            }
            text: sheet.done ? I18n.tr("DONE") : I18n.tr("CANCEL")
            armed: !sheet.busy
            onAct: sheet.close()
        }
        Btn {
            visible: !sheet.done
            anchors {
                right: parent.right
                rightMargin: Tokens.s5
                verticalCenter: parent.verticalCenter
            }
            text: sheet.busy ? I18n.tr("WORKING") : I18n.tr("RESET SETTINGS")
            primary: true
            armed: !sheet.busy && !!sheet.preview
            onAct: sheet.runReset(false)
        }
    }
}
