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
    property string limitMode: "none"

    signal submitted(string name, double limitBytes)

    parent: Overlay.overlay
    anchors.centerIn: parent
    modal: true
    width: Math.min(440, parent ? parent.width - Tokens.s6 * 2 : 440)
    padding: Tokens.s5
    closePolicy: busy ? Popup.NoAutoClose : Popup.CloseOnEscape | Popup.CloseOnPressOutside

    background: Rectangle {
        color: Tokens.paper
        border.width: Tokens.border
        border.color: Tokens.lineStrong
        radius: Tokens.radius
    }
    Overlay.modal: Rectangle { color: Qt.rgba(0, 0, 0, 0.55) }

    function openFor(name, currentBytes) {
        dialog.userName = name;
        dialog.localError = "";
        if (Number(currentBytes || 0) > 0) {
            dialog.limitMode = "custom";
            limitField.text = (Number(currentBytes) / 1073741824).toFixed(2).replace(/\.00$/, "");
        } else {
            dialog.limitMode = "none";
            limitField.text = "";
        }
        open();
        if (dialog.limitMode === "custom")
            limitField.grabFocus();
    }

    function submit() {
        dialog.localError = "";
        var bytes = 0;
        if (dialog.limitMode === "custom") {
            const gib = Number(limitField.text.trim());
            if (!isFinite(gib) || gib <= 0) {
                dialog.localError = I18n.tr("Enter a storage limit greater than zero.");
                limitField.grabFocus();
                return;
            }
            bytes = Math.round(gib * 1073741824);
        }
        dialog.submitted(dialog.userName, bytes);
    }

    contentItem: Column {
        spacing: Tokens.s4

        Column {
            width: parent.width
            spacing: Tokens.s1
            Text {
                text: I18n.tr("HOME STORAGE")
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fMicro
                font.weight: Font.Medium
                font.letterSpacing: Tokens.trackMark
            }
            Text {
                width: parent.width
                text: I18n.tr("Storage limit for %1").arg(dialog.userName)
                color: Tokens.ink
                font.family: Tokens.display
                font.pixelSize: Tokens.fValue
                wrapMode: Text.WordWrap
            }
            Text {
                width: parent.width
                text: I18n.tr("A limit is enforced only when this home supports filesystem quotas.")
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                wrapMode: Text.WordWrap
            }
        }

        Chips {
            id: modeChoice
            options: ["none", "custom"]
            labels: ({ "none": I18n.tr("No limit"), "custom": I18n.tr("Custom") })
            current: dialog.limitMode
            onChose: value => {
                dialog.limitMode = value;
                if (value === "custom")
                    limitField.grabFocus();
            }
        }

        Column {
            visible: dialog.limitMode === "custom"
            width: parent.width
            spacing: Tokens.s1
            Text {
                text: I18n.tr("LIMIT IN GiB")
                color: Tokens.inkDim
                font.family: Tokens.ui
                font.pixelSize: Tokens.fMicro
                font.letterSpacing: Tokens.trackLabel
            }
            Field {
                id: limitField
                width: parent.width
                tabular: true
                placeholder: I18n.tr("For example, 128")
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
                text: dialog.busy ? I18n.tr("SAVING…") : I18n.tr("SAVE LIMIT")
                primary: true
                armed: !dialog.busy
                onAct: dialog.submit()
            }
        }
    }
}
