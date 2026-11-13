import QtQuick
import "Singletons"
import Ryoku.Ui.Singletons

// A small circular direction picker for the drop-shadow angle. 0 deg points
// right, 90 points down (screen coordinates), so the handle sits where the
// shadow falls. Click or drag anywhere in the circle. Emits moved(degrees).
Item {
    id: dial

    property real angle: 90
    property string label: ""
    signal moved(real a)

    readonly property color vermilion: Theme.accent
    readonly property color idle: Theme.inkDim

    implicitWidth: 56
    implicitHeight: 70

    Text {
        anchors.top: parent.top
        anchors.horizontalCenter: parent.horizontalCenter
        text: dial.label
        color: dial.idle
        font.family: "Geologica"
        font.pixelSize: 12
        font.weight: Font.Medium
    }

    Rectangle {
        id: face
        width: 52
        height: 52
        radius: 26
        anchors.bottom: parent.bottom
        anchors.horizontalCenter: parent.horizontalCenter
        color: Qt.rgba(1, 1, 1, 0.05)
        border.width: 1
        border.color: Qt.rgba(1, 1, 1, 0.14)

        Rectangle {
            anchors.centerIn: parent
            width: 4
            height: 4
            radius: 2
            color: Qt.rgba(1, 1, 1, 0.3)
        }
        Rectangle {
            width: 9
            height: 9
            radius: 4.5
            color: dial.vermilion
            x: face.width / 2 - width / 2 + Math.cos(dial.angle * Math.PI / 180) * (face.width / 2 - 9)
            y: face.height / 2 - height / 2 + Math.sin(dial.angle * Math.PI / 180) * (face.height / 2 - 9)
        }

        MouseArea {
            anchors.fill: parent
            preventStealing: true
            function apply(e) {
                var dx = e.x - face.width / 2;
                var dy = e.y - face.height / 2;
                dial.moved((Math.atan2(dy, dx) * 180 / Math.PI + 360) % 360);
            }
            onPressed: (e) => apply(e)
            onPositionChanged: (e) => { if (pressed) apply(e); }
        }
    }
}
