import QtQuick
import QtQuick.Controls
import Ryoku.Ui
import Ryoku.Ui.Singletons

Popup {
    id: dialog

    property string userName: ""
    property bool busy: false
    property string error: ""
    property string localError: ""
    property string fileChoice: "keep"

    signal submitted(string name, bool eraseFiles)

    parent: Overlay.overlay
    anchors.centerIn: parent
    modal: true
    width: Math.min(480, parent ? parent.width - Tokens.s6 * 2 : 480)
    padding: Tokens.s5
    closePolicy: busy ? Popup.NoAutoClose : Popup.CloseOnEscape | Popup.CloseOnPressOutside

    background: Rectangle {
        color: Tokens.paper
        border.width: Tokens.border
        border.color: Tokens.lineStrong
        radius: Tokens.radius
    }
    Overlay.modal: Rectangle { color: Qt.rgba(0, 0, 0, 0.6) }

    function openFor(name) {
        dialog.userName = name;
        dialog.fileChoice = "keep";
        dialog.localError = "";
        confirmField.text = "";
        open();
    }

    function submit() {
        dialog.localError = "";
        if (dialog.fileChoice === "erase" && confirmField.text.trim() !== dialog.userName) {
            dialog.localError = I18n.tr("Type %1 to erase this person's files.").arg(dialog.userName);
            confirmField.grabFocus();
            return;
        }
        dialog.submitted(dialog.userName, dialog.fileChoice === "erase");
    }

    contentItem: Column {
        spacing: Tokens.s4

        Column {
            width: parent.width
            spacing: Tokens.s1
            Text {
                text: I18n.tr("REMOVE PERSON")
                color: Tokens.alert
                font.family: Tokens.ui
                font.pixelSize: Tokens.fMicro
                font.weight: Font.Medium
                font.letterSpacing: Tokens.trackMark
            }
            Text {
                width: parent.width
                text: I18n.tr("Remove %1?").arg(dialog.userName)
                color: Tokens.ink
                font.family: Tokens.display
                font.pixelSize: Tokens.fValue
                wrapMode: Text.WordWrap
            }
            Text {
                width: parent.width
                text: I18n.tr("Signed-in people cannot be removed. The last administrator and your own account are protected.")
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                wrapMode: Text.WordWrap
            }
        }

        Chips {
            id: fileChoice
            options: ["keep", "erase"]
            labels: ({ "keep": I18n.tr("Keep files"), "erase": I18n.tr("Erase files") })
            current: dialog.fileChoice
            onChose: value => {
                dialog.fileChoice = value;
                dialog.localError = "";
                if (value === "erase")
                    confirmField.grabFocus();
            }
        }

        Text {
            width: parent.width
            text: dialog.fileChoice === "keep"
                ? I18n.tr("The home folder is moved to the removed-accounts archive.")
                : I18n.tr("The home subvolume or folder is deleted. Snapshots may still retain old data.")
            color: dialog.fileChoice === "erase" ? Tokens.alert : Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            wrapMode: Text.WordWrap
        }

        Column {
            visible: dialog.fileChoice === "erase"
            width: parent.width
            spacing: Tokens.s1
            Text {
                text: I18n.tr("TYPE %1 TO ERASE").arg(dialog.userName)
                color: Tokens.inkDim
                font.family: Tokens.ui
                font.pixelSize: Tokens.fMicro
                font.letterSpacing: Tokens.trackLabel
            }
            Field {
                id: confirmField
                width: parent.width
                tabular: true
                placeholder: dialog.userName
                onAccepted: dialog.submit()
            }
        }

        Text {
            visible: dialog.localError !== "" || dialog.error !== ""
            width: parent.width
            text: dialog.localError !== "" ? dialog.localError : dialog.error
            color: Tokens.alert
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            font.weight: Font.Medium
            wrapMode: Text.WordWrap
        }

        Row {
            anchors.right: parent.right
            spacing: Tokens.s2
            Btn { text: I18n.tr("CANCEL"); armed: !dialog.busy; onAct: dialog.close() }
            Btn {
                text: dialog.busy ? I18n.tr("REMOVING…") : I18n.tr("REMOVE")
                primary: true
                armed: !dialog.busy
                onAct: dialog.submit()
            }
        }
    }
}
