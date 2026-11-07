import QtQuick

Item {
    id: root

    property string label: ""
    property bool selected: false
    default property alias preview: previewArea.data

    signal clicked()

    implicitWidth: 128
    implicitHeight: 84
    scale: tilePress.pressScale

    Rectangle {
        anchors.fill: parent
        radius: Style.radius
        color: tilePress.pressed ? Style.tint16
            : root.selected ? Style.tint10
            : tilePress.hovered ? Style.tint5 : "transparent"
        border.width: Style.border
        border.color: root.selected ? Style.ink
            : tilePress.hovered ? Style.lineStrong : Style.line

        Behavior on color {
            enabled: !Style.still
            ColorAnimation { duration: Style.snap }
        }
    }

    Item {
        id: previewArea
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.margins: Style.s2
        height: 44
        clip: true
    }

    Label {
        anchors.left: parent.left
        anchors.leftMargin: Style.s2
        anchors.right: selectedDot.left
        anchors.rightMargin: Style.s2
        anchors.bottom: parent.bottom
        anchors.bottomMargin: Style.s2
        text: root.label
        role: "label"
        tone: root.selected ? "ink" : "dim"
        strong: root.selected
    }

    Rectangle {
        id: selectedDot
        visible: root.selected
        anchors.right: parent.right
        anchors.rightMargin: Style.s2
        anchors.bottom: parent.bottom
        anchors.bottomMargin: Style.s2
        width: 6
        height: 6
        radius: 3
        color: Style.ink
    }

    Press {
        id: tilePress
        enabled: root.enabled
        tint: false
        onClicked: root.clicked()
    }
}
