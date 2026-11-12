import QtQuick

Row {
    id: dots

    property int count: 1
    property int current: 0
    signal picked(int index)

    spacing: 6
    visible: dots.count > 1

    Repeater {
        model: dots.count
        delegate: Rectangle {
            id: dot
            required property int index
            readonly property bool on: dot.index === dots.current
            width: dot.on ? 18 : 6
            height: 6
            radius: 3
            color: dot.on ? Md.accent : Md.inkFaint
            Behavior on width {
                enabled: !Md.still
                Spring { speed: "fast" }
            }
            MouseArea {
                anchors.fill: parent
                anchors.margins: -6
                cursorShape: Qt.PointingHandCursor
                onClicked: dots.picked(dot.index)
            }
        }
    }
}
