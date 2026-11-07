import QtQuick
import shell.services
import shell.barkit as Pill

// The modal back control.
Item {
    id: back

    signal clicked()

    readonly property color fill: Theme.shadow
    readonly property color ink: Theme.ink(fill, 7)
    function dim(a) { return Qt.rgba(back.ink.r, back.ink.g, back.ink.b, a) }

    implicitWidth: 36
    implicitHeight: 36

    Rectangle {
        anchors.fill: parent
        radius: width / 2
        color: hover.hovered ? back.dim(0.12) : "transparent"
        Behavior on color {
            enabled: !Motion.reduce
            ColorAnimation { duration: Motion.fast; easing.type: Motion.easeStandard }
        }
    }

    Pill.MaterialIcon {
        anchors.centerIn: parent
        text: "arrow_back"
        color: back.ink
        font.pixelSize: 20
    }

    HoverHandler { id: hover }
    MouseArea {
        anchors.fill: parent
        cursorShape: Qt.PointingHandCursor
        onClicked: back.clicked()
    }
}
