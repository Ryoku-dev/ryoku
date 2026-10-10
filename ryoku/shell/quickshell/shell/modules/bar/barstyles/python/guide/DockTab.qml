import QtQuick
import QtQuick.Layouts
import "../"
import "../reusables"
import Ryoku.Ui.Singletons as Ui

Item {
    id: dockTabRoot
    required property var rootObj
    required property int tabIndex

    function text(key, fallback) {
        const translated = I18n.t(key)
        return translated === key ? fallback : translated
    }

    anchors.fill: parent
    visible: rootObj.currentTab === tabIndex
    opacity: visible ? 1 : 0

    ColumnLayout {
        anchors.centerIn: parent
        width: Math.min(parent.width - rootObj.s(48), rootObj.s(560))
        spacing: rootObj.s(16)

        Text {
            Layout.fillWidth: true
            text: dockTabRoot.text("guide.dock.stage_title", "Dock settings moved to Stage Editor")
            color: ThemeBackend.text
            font.family: ThemeBackend.fontFamily
            font.pixelSize: rootObj.s(22)
            font.bold: true
            horizontalAlignment: Text.AlignHCenter
            wrapMode: Text.WordWrap
        }

        Text {
            Layout.fillWidth: true
            text: dockTabRoot.text("guide.dock.stage_desc", "Choose the design, placement, behaviour and pinned apps in one shared dock section.")
            color: ThemeBackend.subtext0
            font.family: ThemeBackend.fontFamily
            font.pixelSize: rootObj.s(12)
            horizontalAlignment: Text.AlignHCenter
            wrapMode: Text.WordWrap
        }

        ClickButton {
            Layout.alignment: Qt.AlignHCenter
            implicitHeight: rootObj.s(40)
            horizontalPadding: rootObj.s(18)
            buttonText: dockTabRoot.text("guide.dock.open_stage", "Open Stage Editor")
            buttonIcon: "󰏘"
            accentColor: ThemeBackend.mauve
            textColor: ThemeBackend.crust
            cornerRadius: ThemeBackend.borderRadius
            onClicked: {
                rootObj.closePopup()
                Ui.Spawn.run(["qs", "-c", "shell", "ipc", "call", "desktop", "editSection", "dock", ""])
            }
        }
    }
}
