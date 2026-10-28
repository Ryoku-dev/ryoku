import QtQuick
import QtQuick.Layouts
import "Singletons"
import Ryoku.Ui.Singletons

Item {
    id: sheet

    property var tools: []

    signal closeRequested()

    readonly property var actions: [
        { key: "Ctrl+C", label: I18n.tr("Copy") },
        { key: "Ctrl+S", label: I18n.tr("Save") },
        { key: "Ctrl+P", label: I18n.tr("Pin") },
        { key: "Ctrl+U", label: I18n.tr("Upload") },
        { key: "Ctrl+K", label: I18n.tr("Send to phone") },
        { key: "Ctrl+B", label: I18n.tr("Beautify") },
        { key: "Enter", label: I18n.tr("Copy and save") },
        { key: "Ctrl+Z", label: I18n.tr("Undo") },
        { key: "Ctrl+Shift+Z", label: I18n.tr("Redo") },
        { key: "Ctrl+A", label: I18n.tr("Whole monitor") },
        { key: "Space", label: I18n.tr("Cycle target") },
        { key: "f", label: I18n.tr("Fill") },
        { key: "k", label: I18n.tr("Sketch") },
        { key: "[ ]", label: I18n.tr("Width") },
        { key: "1..8", label: I18n.tr("Colour") },
        { key: "Del", label: I18n.tr("Delete") },
        { key: "Esc", label: I18n.tr("Back") }
    ]

    focus: visible

    component ShortcutRow: RowLayout {
        id: row
        property string label: ""
        property string key: ""
        spacing: 16
        Text {
            Layout.fillWidth: true
            text: row.label
            color: Theme.ink
            font.family: Theme.ui
            font.pixelSize: 13
            elide: Text.ElideRight
        }
        Rectangle {
            radius: 4
            color: Theme.hover
            border.color: Theme.hair
            border.width: 1
            implicitWidth: cap.implicitWidth + 12
            implicitHeight: cap.implicitHeight + 6
            Text {
                id: cap
                anchors.centerIn: parent
                text: row.key
                color: Theme.inkDim
                font.family: Theme.mono
                font.pixelSize: 12
            }
        }
    }

    MouseArea {
        anchors.fill: parent
        onClicked: sheet.closeRequested()
    }

    Rectangle {
        id: card
        anchors.centerIn: parent
        radius: Theme.radius
        color: Theme.panel
        border.color: Theme.hair
        border.width: 1
        implicitWidth: body.implicitWidth + 40
        implicitHeight: body.implicitHeight + 36

        opacity: sheet.visible ? 1 : 0
        scale: sheet.visible ? 1.0 : 0.97
        Behavior on opacity { NumberAnimation { duration: Theme.snap } }
        Behavior on scale { NumberAnimation { duration: Theme.snap } }

        MouseArea { anchors.fill: parent }

        ColumnLayout {
            id: body
            anchors.centerIn: parent
            spacing: 14

            Text {
                text: I18n.tr("Shortcuts")
                color: Theme.ink
                font.family: Theme.ui
                font.pixelSize: 18
                font.weight: Font.DemiBold
            }

            GridLayout {
                columns: 2
                columnSpacing: 40
                rowSpacing: 8
                Repeater {
                    model: sheet.tools
                    ShortcutRow {
                        Layout.preferredWidth: 200
                        label: modelData.label
                        key: modelData.key
                    }
                }
            }

            Rectangle {
                Layout.fillWidth: true
                height: 1
                color: Theme.hair
            }

            GridLayout {
                columns: 2
                columnSpacing: 40
                rowSpacing: 8
                Repeater {
                    model: sheet.actions
                    ShortcutRow {
                        Layout.preferredWidth: 200
                        label: modelData.label
                        key: modelData.key
                    }
                }
            }
        }
    }

    Keys.onPressed: (e) => {
        if (e.key === Qt.Key_Escape) {
            e.accepted = true;
            sheet.closeRequested();
        }
    }
}
