import QtQuick
import Ryoku.Ui.Singletons

// bounded integer. the value itself lives in the cell's numeral; this is only
// the pair of buttons. holding repeats, and an exhausted bound disables.
Row {
    id: step
    property int value: 0
    property int from: 0
    property int to: 100
    property int stepBy: (to - from) > 60 ? 4 : 1
    signal modified(int v)

    activeFocusOnTab: true
    function activate(up) { bump(up) }
    Keys.onPressed: event => {
        if (event.key === Qt.Key_Right || event.key === Qt.Key_Up || event.key === Qt.Key_Plus) {
            bump(true); event.accepted = true;
        } else if (event.key === Qt.Key_Left || event.key === Qt.Key_Down || event.key === Qt.Key_Minus) {
            bump(false); event.accepted = true;
        }
    }

    spacing: 0

    Repeater {
        model: ["−", "+"]
        Rectangle {
            required property string modelData
            required property int index
            readonly property bool up: index === 1
            readonly property bool spent: up ? step.value >= step.to : step.value <= step.from

            width: 29
            height: 24
            radius: Tokens.radius
            opacity: spent ? 0.3 : 1
            color: !spent && tap.pressed ? Tokens.tint16 : (bh.hovered && !spent ? Tokens.tint10 : "transparent")
            border.width: Tokens.border
            border.color: bh.hovered && !spent ? Tokens.lineStrong : Tokens.line
            Behavior on color { ColorAnimation { duration: Tokens.snap } }

            Text {
                anchors.centerIn: parent
                text: parent.modelData
                color: Tokens.inkDim
                font.family: Tokens.ui
                font.pixelSize: 13
            }
            HoverHandler { id: bh; enabled: !parent.spent; cursorShape: Qt.PointingHandCursor }
            TapHandler {
                id: tap
                enabled: !parent.spent
                longPressThreshold: 0.4
                onTapped: step.bump(parent.up)
                onLongPressed: repeat.start()
                onPressedChanged: if (!pressed) repeat.stop()
                onCanceled: repeat.stop()
            }
            Timer {
                id: repeat
                interval: 125
                repeat: true
                onTriggered: parent.spent ? stop() : step.bump(parent.up)
            }
        }
    }
    function bump(up) {
        var v = value + (up ? stepBy : -stepBy);
        modified(Math.max(from, Math.min(to, v)));
    }
}
