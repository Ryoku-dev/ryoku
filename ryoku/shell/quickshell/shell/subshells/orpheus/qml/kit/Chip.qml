import QtQuick

Item {
    id: root

    property string text: ""
    property string icon: ""
    property bool selected: false
    property string variant: "filter"

    signal clicked()

    readonly property bool showIcon: icon.length > 0

    implicitWidth: chipContent.implicitWidth + Style.s4
    implicitHeight: 28
    opacity: enabled ? 1 : 0.38
    scale: chipPress.pressScale

    Rectangle {
        anchors.fill: parent
        radius: Style.radius
        color: root.selected ? Style.bone
            : chipPress.pressed ? Style.tint16
            : chipPress.hovered ? Style.tint10 : "transparent"
        border.width: Style.border
        border.color: root.selected ? Style.bone
            : chipPress.hovered ? Style.lineStrong : Style.line

        Behavior on color {
            enabled: !Style.still
            ColorAnimation { duration: Style.snap }
        }
    }

    Row {
        id: chipContent
        anchors.centerIn: parent
        spacing: Style.s1

        Glyph {
            visible: root.showIcon
            anchors.verticalCenter: parent.verticalCenter
            name: root.icon
            size: 16
            tone: root.selected ? "onBone" : "dim"
        }

        Label {
            anchors.verticalCenter: parent.verticalCenter
            text: root.text
            role: "label"
            tone: root.selected ? "onBone" : "ink"
        }
    }

    Press {
        id: chipPress
        enabled: root.enabled
        tint: false
        onClicked: root.clicked()
    }
}
