import QtQuick

Item {
    id: root

    property bool playing: false

    signal clicked()

    implicitWidth: 44
    implicitHeight: 44
    scale: playPress.pressScale

    Shape {
        anchors.fill: parent
        cutSize: Look.cutMd
        fill: Style.bone
        strokeWidth: Style.border
        strokeColor: Style.bone
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
