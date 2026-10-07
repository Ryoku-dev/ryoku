import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "Singletons"

Item {
    id: root

    signal runCommand(var argv)

    Column {
        id: page
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        spacing: Tokens.s5

        PageTitle {
            id: heading
            width: parent.width
            title: I18n.tr("Make it yours")
            description: I18n.tr("Ryostore carries complete looks and the pieces to build your own. Wallpaper colours retune the desktop.")
        }

        Row {
            id: cards
            width: parent.width
            height: Math.min(390, Math.max(320,
                root.height - heading.implicitHeight - page.spacing))
            spacing: Tokens.s4

            Rectangle {
                id: storeCard
                width: Math.floor((parent.width - parent.spacing) * 0.66)
                height: parent.height
                radius: Tokens.radius
                color: Tokens.paper
                border.width: Tokens.border
                border.color: Tokens.lineStrong

                Rectangle {
                    anchors.left: parent.left
                    anchors.top: parent.top
                    anchors.bottom: parent.bottom
                    width: 5
                    radius: Tokens.radius
                    color: Tokens.bone
                }

                Column {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    anchors.leftMargin: Tokens.s5
                    anchors.rightMargin: Tokens.s5
                    spacing: Tokens.s4

                    Row {
                        width: parent.width
                        spacing: Tokens.s3

                        Text {
                            id: storeLabel
                            text: I18n.tr("RYOSTORE")
                            color: Tokens.ink
                            font.family: Tokens.mono
                            font.pixelSize: Tokens.fMicro
                            font.weight: Font.DemiBold
                            font.letterSpacing: Tokens.trackMark
                        }
                        Rectangle {
                            anchors.verticalCenter: parent.verticalCenter
                            width: parent.width - storeLabel.width - parent.spacing
                            height: 1
                            color: Tokens.line
                        }
                    }

                    Text {
                        width: parent.width
                        text: I18n.tr("Find the rest of the desktop")
                        color: Tokens.ink
                        font.family: Tokens.display
                        font.pixelSize: Tokens.fTitle
                        font.weight: Font.DemiBold
                        wrapMode: Text.WordWrap
                    }

                    Text {
                        width: parent.width
                        text: I18n.tr("Bar styles reshape the edge. Rices apply a complete look. Plugins add new parts. Widgets and lockscreens are here too.")
                        color: Tokens.inkDim
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fBody
                        wrapMode: Text.WordWrap
                        lineHeight: 1.3
                    }

                    Row {
                        spacing: Tokens.s2

                        Btn {
                            text: I18n.tr("Open Ryostore")
                            primary: true
                            onAct: root.runCommand(["ryostore", "open", "discover"])
                        }
                        Btn {
                            text: I18n.tr("Bar styles")
                            onAct: root.runCommand(["ryostore", "open", "barstyles"])
                        }
                        Btn {
                            text: I18n.tr("Rices")
                            onAct: root.runCommand(["ryostore", "open", "rices"])
                        }
                        Btn {
                            text: I18n.tr("Plugins")
                            onAct: root.runCommand(["ryostore", "open", "plugins"])
                        }
                    }
                }
            }

            Rectangle {
                width: parent.width - parent.spacing - storeCard.width
                height: parent.height
                radius: Tokens.radius
                color: Tokens.paperLift
                border.width: Tokens.border
                border.color: Tokens.line

                Ticks { anchors.margins: Tokens.s3 }

                Column {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    anchors.leftMargin: Tokens.s5
                    anchors.rightMargin: Tokens.s5
                    spacing: Tokens.s4

                    Text {
                        width: parent.width
                        text: I18n.tr("Wallpaper")
                        color: Tokens.ink
                        font.family: Tokens.display
                        font.pixelSize: Tokens.fValue
                        font.weight: Font.DemiBold
                        wrapMode: Text.WordWrap
                    }

                    Text {
                        width: parent.width
                        text: I18n.tr("Choose a picture or live wall. Ryoku follows its colours automatically.")
                        color: Tokens.inkDim
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                        wrapMode: Text.WordWrap
                        lineHeight: 1.25
                    }

                    Keycap {
                        id: wallpaperKey
                        text: I18n.tr("Super + W")
                        us: 0.72
                        width: implicitWidth
                        height: implicitHeight
                        layer.enabled: false
                        dark: !Tokens.light
                        motionEnabled: !Motion.reduce
                    }

                    Btn {
                        text: I18n.tr("Choose wallpaper")
                        primary: true
                        onAct: {
                            wallpaperKey.pulse++
                            root.runCommand(["ryogami", "wallpaper", "ui"])
                        }
                    }
                }
            }
        }
    }
}
