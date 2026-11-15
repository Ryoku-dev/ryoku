import QtQuick
import Lithic
import "../../Singletons"

// a gloss set vertically, top to bottom, as a Japanese poster sets its title beside a plate:
// a short rule, the page's kanji large, a dot, then its category's kanji small
Column {
    id: root

    property string gloss: ""
    property string category: ""

    spacing: 4
    width: 22

    Rectangle {
        anchors.horizontalCenter: parent.horizontalCenter
        width: 1
        height: 18
        color: Tone.accent
    }
    Repeater {
        model: Array.from(root.gloss)
        delegate: Label {
            required property string modelData
            anchors.horizontalCenter: parent.horizontalCenter
            text: modelData
            typeRole: "title"
            font.pixelSize: 20
            lineHeight: 22
            inkRole: "fg2"
        }
    }
    Rectangle {
        visible: root.category !== "" && root.category !== root.gloss
        anchors.horizontalCenter: parent.horizontalCenter
        width: 3
        height: 3
        color: Lithic.fg3
    }
    Repeater {
        model: root.category !== root.gloss ? Array.from(root.category) : []
        delegate: Label {
            required property string modelData
            anchors.horizontalCenter: parent.horizontalCenter
            text: modelData
            typeRole: "caption"
            font.pixelSize: 11
            inkRole: "fg3"
        }
    }
}
