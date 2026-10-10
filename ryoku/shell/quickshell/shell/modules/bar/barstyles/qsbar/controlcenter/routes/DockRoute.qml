import QtQuick
import "../kit"
import "../../modules"
import Ryoku.Ui
import Ryoku.Ui.Singletons

Item {
    id: page
    property var root
    property var cc
    readonly property var tk: page.cc ? page.cc.tokens : null
    readonly property real colW: Math.min(page.width, page.tk ? page.tk.contentW : 640)
    implicitHeight: col.implicitHeight

    function openStageEditor() {
        if (page.cc)
            page.cc.close()
        Spawn.run(["qs", "-c", "shell", "ipc", "call", "desktop", "editSection", "dock", ""])
    }

    Flickable {
        anchors.fill: parent
        contentWidth: width
        contentHeight: col.implicitHeight
        clip: true
        boundsBehavior: Flickable.StopAtBounds

        Column {
            id: col
            width: page.colW
            spacing: page.tk ? page.tk.sectionGap : 16

            Entrance {
                width: page.colW
                index: 0
                SettingCard {
                    width: page.colW
                    title: I18n.tr("DOCK")
                    kana: "\u53f0"

                    SettingRow {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        block: true
                        label: I18n.tr("Dock settings moved to Stage Editor")
                        desc: I18n.tr("Choose the design, placement, behaviour and pinned apps in one place.")
                        source: I18n.tr("Stage Editor")
                        Btn {
                            anchors.left: parent.left
                            anchors.verticalCenter: parent.verticalCenter
                            text: I18n.tr("OPEN STAGE EDITOR")
                            primary: true
                            onAct: page.openStageEditor()
                        }
                    }
                }
            }
        }
    }
}
