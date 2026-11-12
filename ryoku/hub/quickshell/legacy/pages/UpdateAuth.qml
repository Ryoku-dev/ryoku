import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons

// The password a Hub-driven update needs for sudo, asked for in the Hub
// rather than in a terminal. The page hands it to the waiting run over its
// FIFO (`ryoku update --auth`, the secret on stdin); nothing here keeps it.
Row {
    id: auth

    required property var run

    spacing: Tokens.s4

    function submit() {
        if (field.text.length === 0 || auth.run.authBusy)
            return;
        auth.run.submitPassword(field.text);
        field.text = "";
    }

    // the field takes focus as soon as the question appears
    onVisibleChanged: if (visible) field.forceActiveFocus()

    Rectangle { width: 2; height: col.implicitHeight; color: Tokens.ink; antialiasing: false }

    Column {
        id: col
        width: auth.width - 2 - Tokens.s4
        spacing: Tokens.s3

        Text {
            text: I18n.tr("PASSWORD")
            color: Tokens.inkMuted; font.family: Tokens.ui
            font.pixelSize: Tokens.fMicro; font.weight: Font.Medium
            font.letterSpacing: Tokens.trackMark
        }
        Text {
            width: parent.width
            text: auth.run.promptTitle
            color: Tokens.ink; font.family: Tokens.display; font.pixelSize: Tokens.fValue
            wrapMode: Text.WordWrap
        }
        Text {
            width: parent.width
            text: auth.run.promptDetail
            color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
            lineHeight: 1.35; wrapMode: Text.WordWrap
        }

        Item { width: 1; height: Tokens.s1 }

        Row {
            spacing: Tokens.s3

            Rectangle {
                width: Math.min(320, col.width - authBtn.width - cancelBtn.width - Tokens.s3 * 2)
                height: Tokens.ctlH + 8
                radius: Tokens.radius
                color: "transparent"
                border.width: Tokens.border
                border.color: auth.run.promptError !== "" ? Tokens.ink : (field.activeFocus ? Tokens.ink : Tokens.line)
                Behavior on border.color { ColorAnimation { duration: Tokens.snap } }

                TextInput {
                    id: field
                    anchors.fill: parent
                    anchors.leftMargin: Tokens.s3; anchors.rightMargin: Tokens.s3
                    verticalAlignment: TextInput.AlignVCenter
                    color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fBody
                    echoMode: TextInput.Password
                    passwordCharacter: "\u2022"
                    selectByMouse: true
                    selectionColor: Tokens.ink; selectedTextColor: Tokens.inkOnBone
                    enabled: !auth.run.authBusy
                    onAccepted: auth.submit()

                    Text {
                        anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                        visible: field.text.length === 0
                        text: auth.run.authBusy ? I18n.tr("checking\u2026") : I18n.tr("Your password")
                        color: Tokens.inkFaint; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                    }
                }
            }
            Btn {
                id: authBtn
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("AUTHORIZE")
                primary: true
                armed: field.text.length > 0 && !auth.run.authBusy
                onAct: auth.submit()
            }
            Btn {
                id: cancelBtn
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("CANCEL")
                armed: !auth.run.authBusy
                onAct: auth.run.cancelPassword()
            }
        }

        Text {
            width: parent.width
            visible: auth.run.promptError !== "" && !auth.run.authBusy
            text: auth.run.promptError
            color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
            font.weight: Font.Medium; wrapMode: Text.WordWrap
        }
    }
}
