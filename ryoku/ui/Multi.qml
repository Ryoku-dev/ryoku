import QtQuick
import Ryoku.Ui.Singletons

// set membership -- not a choice. members toggle independently, so a member
// carries its own in/out mark rather than borrowing the exclusive grammar.
Flow {
    id: multi
    property var options: []
    property var chosen: []
    signal toggled(string key)

    spacing: 5

    Repeater {
        model: multi.options
        Rectangle {
            required property string modelData
            readonly property bool on: multi.chosen.indexOf(modelData) >= 0
            width: row.width + 20
            height: 24
            radius: Tokens.radius
            color: on ? Tokens.bone : (tap.pressed ? Tokens.tint16 : (mh.hovered ? Tokens.tint10 : "transparent"))
            border.width: Tokens.border
            border.color: mh.hovered && !on ? Tokens.lineStrong : Tokens.line
            Behavior on color { ColorAnimation { duration: Tokens.snap } }
            Row {
                id: row
                anchors.centerIn: parent
                spacing: 5
                Text {
                    text: parent.parent.on ? "✓" : "+"
                    color: parent.parent.on ? Tokens.inkOnBone : Tokens.inkFaint
                    font.family: Tokens.ui
                    font.pixelSize: 9
                    anchors.verticalCenter: parent.verticalCenter
                }
                Text {
                    text: I18n.tr(parent.parent.modelData)
                    color: parent.parent.on ? Tokens.inkOnBone : Tokens.inkDim
                    font.family: Tokens.ui
                    font.pixelSize: 10
                    font.weight: Font.Medium
                }
            }
            HoverHandler { id: mh; cursorShape: Qt.PointingHandCursor }
            TapHandler { id: tap; onTapped: multi.toggled(parent.modelData) }
        }
    }
}
