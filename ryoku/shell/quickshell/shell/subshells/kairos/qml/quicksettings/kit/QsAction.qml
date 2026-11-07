import QtQuick
import shell.services

// A compact pill action (Connect, Disconnect, Cancel), or with `destructive` the
// hairline-outlined word the rest of the shell uses for Forget: no fill, so it
// reads as the quieter of two actions standing side by side.
Rectangle {
    id: action

    property string text: ""
    property bool destructive: false
    signal clicked()

    readonly property color fill: Theme.shadow
    readonly property color ink: Theme.ink(fill, 7)
    function dim(a) { return Qt.rgba(action.ink.r, action.ink.g, action.ink.b, a) }

    implicitWidth: label.implicitWidth + 26
    implicitHeight: 30
    radius: 15
    opacity: action.enabled ? 1 : 0.45
    color: action.destructive ? "transparent"
        : hover.hovered ? action.dim(0.20) : action.dim(0.13)
    border.width: action.destructive ? Theme.borderWidth : 0
    border.color: action.dim(hover.hovered ? 0.30 : 0.18)
    Behavior on color {
        enabled: !Motion.reduce
        ColorAnimation { duration: Motion.fast; easing.type: Motion.easeStandard }
    }

    Text {
        id: label
        anchors.centerIn: parent
        text: action.text
        color: action.ink
        font.family: Theme.fontPrimary
        font.pixelSize: 12
        font.weight: Font.DemiBold
    }

    HoverHandler { id: hover }
    MouseArea {
        anchors.fill: parent
        cursorShape: Qt.PointingHandCursor
        onClicked: action.clicked()
    }
}
