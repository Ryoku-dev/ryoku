import QtQuick
import shell.services

// Apple-style toggle: accent track when on, dark knob riding the accent.
Rectangle {
    id: sw

    property bool value: false
    signal toggled(bool v)

    readonly property color fill: Theme.shadow
    readonly property color ink: Theme.ink(fill, 7)
    readonly property color accent: Theme.primary
    function dim(a) { return Qt.rgba(sw.ink.r, sw.ink.g, sw.ink.b, a) }

    implicitWidth: 44
    implicitHeight: 26
    radius: height / 2
    color: sw.value ? sw.accent : sw.dim(0.16)
    Behavior on color {
        enabled: !Motion.reduce
        ColorAnimation { duration: Motion.fast; easing.type: Motion.easeStandard }
    }

    Rectangle {
        width: 20
        height: 20
        radius: 10
        y: (parent.height - height) / 2
        x: sw.value ? parent.width - width - 3 : 3
        color: sw.value ? sw.fill : sw.dim(0.85)
        Behavior on x {
            enabled: !Motion.reduce
            NumberAnimation { duration: Motion.fast; easing.type: Motion.easeStandard }
        }
    }

    MouseArea {
        anchors.fill: parent
        cursorShape: Qt.PointingHandCursor
        onClicked: sw.toggled(!sw.value)
    }
}
