pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui
import shell.barkit as Pill
import "../../kit" as K
import "." as P

Item {
    id: root

    property var scene: null
    property bool shown: false
    property var args: ({})

    implicitHeight: content.implicitHeight
    enabled: shown

    Column {
        id: content
        width: root.width
        spacing: K.Style.s5

        P.PageHead {
            width: parent.width
            title: "About"
            description: "The identity and place of Ryoku's default bar."
        }

        Column {
            width: parent.width
            spacing: K.Style.s2

            K.Label {
                width: parent.width
                text: "Orpheus"
                color: K.Style.ink
                font.family: K.Style.display
                font.pixelSize: 28
                font.weight: Font.Light
            }

            K.Label {
                width: parent.width
                text: "Ryoku's liquid bar grows pages from the frame and moves with what is playing."
                role: "body"
                tone: "muted"
                wrapMode: Text.WordWrap
            }
        }

        SettingCard {
            width: parent.width
            title: "DETAILS"
            collapsible: false

            SettingRow {
                width: parent.width
                label: "Version"
                value: "5"
                controlWidth: 0
            }

            SettingRow {
                width: parent.width
                label: "Part of Ryoku"
                controlWidth: 32
                divider: true

                Item {
                    anchors.fill: parent
                    Pill.BrandMark {
                        anchors.centerIn: parent
                        size: 20
                        color: K.Style.accent
                        weight: Font.Black
                    }
                }
            }
        }

        K.Label {
            width: parent.width
            text: "Made by neur0map"
            role: "body"
            tone: "faint"
            horizontalAlignment: Text.AlignHCenter
        }
    }
}
