import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons

Item {
    id: root

    signal runCommand(var argv)
    signal finish()

    Rectangle {
        anchors.centerIn: parent
        width: Math.min(parent.width, 720)
        height: Math.min(parent.height, 360)
        radius: Tokens.radius
        color: Tokens.paperLift
        border.width: Tokens.border
        border.color: Tokens.lineStrong

        Ticks { anchors.margins: Tokens.s3 }

        Column {
            anchors.centerIn: parent
            width: Math.min(parent.width - Tokens.s7 * 2, 600)
            spacing: Tokens.s5

            Image {
                anchors.horizontalCenter: parent.horizontalCenter
                width: 58
                height: 58
                source: Qt.resolvedUrl("art/logo-mark.svg")
                fillMode: Image.PreserveAspectFit
                sourceSize: Qt.size(116, 116)
            }

            Text {
                width: parent.width
                text: I18n.tr("The desktop is yours")
                color: Tokens.ink
                font.family: Tokens.display
                font.pixelSize: Math.max(Tokens.fTitle, 40)
                font.weight: Font.DemiBold
                horizontalAlignment: Text.AlignHCenter
                wrapMode: Text.WordWrap
            }

            Text {
                width: parent.width
                text: I18n.tr("Change anything later in Ryoku Hub, or tune QS Bar from its own settings.")
                color: Tokens.inkDim
                font.family: Tokens.ui
                font.pixelSize: Tokens.fBody
                horizontalAlignment: Text.AlignHCenter
                wrapMode: Text.WordWrap
                lineHeight: 1.3
            }

            Row {
                anchors.horizontalCenter: parent.horizontalCenter
                spacing: Tokens.s3

                Btn {
                    text: I18n.tr("Open Ryoku Hub")
                    onAct: root.runCommand(["ryoku-shell", "hub", "open"])
                }
                Btn {
                    text: I18n.tr("QS Bar settings")
                    onAct: root.runCommand(["ryoku-shell", "bar", "settings"])
                }
                Btn {
                    text: I18n.tr("Finish")
                    primary: true
                    onAct: root.finish()
                }
            }
        }
    }
}
