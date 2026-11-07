import QtQuick

Item {
    id: root

    property bool playing: false

    signal clicked()

    implicitWidth: 44
    implicitHeight: 44
    scale: playPress.pressScale

    Rectangle {
        anchors.fill: parent
        radius: Style.plateRadius
        color: Style.bone
        border.width: Style.border
        border.color: Style.bone
    }

    Glyph {
        anchors.centerIn: parent
        name: root.playing ? "pause" : "play_arrow"
        size: 26
        tone: "onBone"
        filled: true
    }

    Press {
        id: playPress
        tint: false
        radius: Style.plateRadius
        onClicked: root.clicked()
    }
}
