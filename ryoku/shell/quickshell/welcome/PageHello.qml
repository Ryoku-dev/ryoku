import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons

Item {
    id: root

    Row {
        anchors.fill: parent
        spacing: Tokens.s6

        Rectangle {
            id: artPanel
            width: Math.min(parent.width * 0.29, 320)
            height: parent.height
            color: Tokens.paperLift
            border.width: Tokens.border
            border.color: Tokens.line
            radius: Tokens.radius
            clip: true

            Image {
                anchors.fill: parent
                anchors.margins: Tokens.s2
                source: Qt.resolvedUrl("art/hero.png")
                fillMode: Image.PreserveAspectFit
                asynchronous: true
                smooth: false
                opacity: 0.9
            }
            Grain { anchors.fill: parent }
            Ticks { anchors.margins: Tokens.s3 }

            Rectangle {
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.bottom: parent.bottom
                height: 112
                gradient: Gradient {
                    GradientStop { position: 0; color: "transparent" }
                    GradientStop { position: 1; color: Tokens.paper }
                }
            }

            Text {
                anchors.left: parent.left
                anchors.bottom: parent.bottom
                anchors.margins: Tokens.s5
                text: I18n.tr("FIRST LIGHT")
                color: Tokens.ink
                font.family: Tokens.mono
                font.pixelSize: Tokens.fMicro
                font.letterSpacing: Tokens.trackMark
            }
        }

        Item {
            width: parent.width - artPanel.width - parent.spacing
            height: parent.height

            Column {
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                spacing: Tokens.s5

                Image {
                    width: 64
                    height: 64
                    source: Qt.resolvedUrl("art/logo-mark.svg")
                    fillMode: Image.PreserveAspectFit
                    sourceSize: Qt.size(128, 128)
                }

                Text {
                    width: parent.width
                    text: I18n.tr("Welcome to Ryoku")
                    color: Tokens.ink
                    font.family: Tokens.display
                    font.pixelSize: Math.max(Tokens.fTitle, 44)
                    font.weight: Font.DemiBold
                    wrapMode: Text.WordWrap
                    lineHeight: 0.95
                }

                Text {
                    width: Math.min(parent.width, 520)
                    text: I18n.tr("Your desktop is already dressed. This short tour shows the few things worth knowing first.")
                    color: Tokens.inkDim
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fRow
                    wrapMode: Text.WordWrap
                    lineHeight: 1.35
                }

                Rectangle {
                    width: Math.min(parent.width, 480)
                    height: 1
                    color: Tokens.line
                }

                Text {
                    width: Math.min(parent.width, 520)
                    text: I18n.tr("Pick a bar, meet Ryostore, then learn the keys that open the desktop.")
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fBody
                    wrapMode: Text.WordWrap
                    lineHeight: 1.3
                }
            }
        }
    }
}
