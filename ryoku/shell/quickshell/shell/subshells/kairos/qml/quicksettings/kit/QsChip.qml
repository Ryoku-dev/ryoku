import QtQuick
import shell.services
import shell.barkit as Pill

// A quick-settings choice chip: filled with the accent when active.
Rectangle {
    id: chip

    property string text: ""
    property bool active: false
    signal clicked()

    readonly property color fill: Theme.shadow
    readonly property color ink: Theme.ink(fill, 7)
    readonly property color accent: Theme.primary
    function dim(a) { return Qt.rgba(chip.ink.r, chip.ink.g, chip.ink.b, a) }

    implicitWidth: label.implicitWidth + 26
    implicitHeight: 32
    radius: 16
    color: chip.active ? chip.accent : (hover.hovered ? chip.dim(0.18) : chip.dim(0.12))
    Behavior on color {
        enabled: !Motion.reduce
        ColorAnimation { duration: Motion.fast; easing.type: Motion.easeStandard }
    }

    Text {
        id: label
        anchors.centerIn: parent
        text: chip.text
        color: chip.active ? chip.fill : chip.ink
        font.family: Theme.fontPrimary
        font.pixelSize: 12
        font.weight: Font.DemiBold
    }

    HoverHandler { id: hover }
    MouseArea {
        anchors.fill: parent
        cursorShape: Qt.PointingHandCursor
        onClicked: chip.clicked()
    }
}
