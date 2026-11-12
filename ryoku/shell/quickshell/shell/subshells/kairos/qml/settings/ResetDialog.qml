import QtQuick
import Ryoku.Ui.Singletons
import "../kit"

Item {
    id: dialog

    property bool open: false
    signal confirmed()
    signal cancelled()

    visible: dialog.opacity > 0.01
    opacity: dialog.open ? 1 : 0
    Behavior on opacity {
        enabled: !Md.still
        NumberAnimation { duration: Md.fast }
    }

    Rectangle {
        anchors.fill: parent
        radius: Md.rXxl
        color: Md.alpha(Md.base, 0.72)
    }
    MouseArea {
        anchors.fill: parent
        enabled: dialog.open
        onClicked: dialog.cancelled()
    }

    Rectangle {
        anchors.centerIn: parent
        width: Math.min(360, parent.width - 48)
        height: body.implicitHeight + 48
        radius: 28
        color: Md.containerHigh
        scale: dialog.open || Md.still ? 1 : 0.9
        Behavior on scale {
            enabled: !Md.still
            Spring {}
        }

        MouseArea { anchors.fill: parent }

        Column {
            id: body
            anchors { left: parent.left; right: parent.right; top: parent.top; margins: 24 }
            spacing: 16

            Label {
                width: parent.width
                role: "headline"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
                text: I18n.tr("Reset island settings?")
            }
            Label {
                width: parent.width
                role: "body"
                color: Md.inkMuted
                wrapMode: Text.Wrap
                elide: Text.ElideNone
                text: I18n.tr("The island's size, position and the options on these pages go back to their defaults. Your tiles, time format, temperature unit and weather location are kept.")
            }
            Row {
                anchors.right: parent.right
                topPadding: 8
                spacing: 8

                Button {
                    variant: "text"
                    text: I18n.tr("Cancel")
                    onClicked: dialog.cancelled()
                }
                Button {
                    variant: "text"
                    danger: true
                    text: I18n.tr("Reset")
                    onClicked: dialog.confirmed()
                }
            }
        }
    }
}
