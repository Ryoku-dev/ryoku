import QtQuick

Item {
    id: chip

    property string text: ""
    property string icon: ""
    property bool selected: false
    signal clicked()

    readonly property bool lead: chip.icon.length > 0 || chip.selected

    implicitWidth: row.implicitWidth + (chip.lead ? 28 : 32)
    implicitHeight: 36
    opacity: chip.enabled ? 1 : 0.38

    Accessible.role: Accessible.CheckBox
    Accessible.name: chip.text
    Accessible.checked: chip.selected

    Rectangle {
        anchors.fill: parent
        radius: Md.rMd
        color: chip.selected ? Md.accentContainer : "transparent"
        border.width: chip.selected ? 0 : 1
        border.color: Md.outline
        Behavior on color {
            enabled: !Md.still
            ColorAnimation { duration: Md.fast }
        }
    }

    Row {
        id: row
        anchors.centerIn: parent
        spacing: 6

        Icon {
            anchors.verticalCenter: parent.verticalCenter
            visible: chip.lead
            name: chip.icon.length > 0 ? chip.icon : "check"
            size: 18
            filled: chip.selected
            color: chip.selected ? Md.onAccentContainer : Md.inkMuted
        }
        Label {
            anchors.verticalCenter: parent.verticalCenter
            role: "label"
            text: chip.text
            color: chip.selected ? Md.onAccentContainer : Md.ink
        }
    }

    StateLayer {
        radius: Md.rMd
        onActivated: chip.clicked()
    }
}
