import QtQuick
import "Singletons"

// 5-8 exclusive named options. wraps; the selected chip inverts.
Flow {
    id: chips
    property var options: []
    property string current: ""
    // optional value -> display map; a value not present falls back to I18n.tr(value)
    property var labels: ({})
    signal chose(string key)

    activeFocusOnTab: true
    function activate(i) { if (i >= 0 && i < options.length) chose(options[i]) }
    Keys.onPressed: event => {
        var i = options.indexOf(current);
        if (event.key === Qt.Key_Right || event.key === Qt.Key_Down) {
            activate(Math.min(options.length - 1, i + 1)); event.accepted = true;
        } else if (event.key === Qt.Key_Left || event.key === Qt.Key_Up) {
            activate(Math.max(0, i - 1)); event.accepted = true;
        }
    }

    spacing: 5
    height: childrenRect.height

    Repeater {
        model: chips.options
        Rectangle {
            required property string modelData
            readonly property bool on: chips.current === modelData
            width: cl.width + 18
            height: 24
            radius: Tokens.radius
            color: on ? Tokens.bone : (tap.pressed ? Tokens.tint16 : (ch.hovered ? Tokens.tint10 : "transparent"))
            border.width: Tokens.border
            border.color: ch.hovered && !on ? Tokens.lineStrong : Tokens.line
            Behavior on color { ColorAnimation { duration: Tokens.snap } }
            Text {
                id: cl
                anchors.centerIn: parent
                text: chips.labels[parent.modelData] !== undefined ? chips.labels[parent.modelData] : I18n.tr(parent.modelData)
                color: parent.on ? Tokens.inkOnBone : Tokens.inkDim
                font.family: Tokens.ui
                font.pixelSize: 10
                font.weight: Font.Medium
            }
            HoverHandler { id: ch; cursorShape: Qt.PointingHandCursor }
            TapHandler { id: tap; onTapped: chips.chose(parent.modelData) }
        }
    }
}
