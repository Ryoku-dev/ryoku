import QtQuick
import "../../kit" as K
import Ryoku.Ui.Singletons

Column {
    id: head

    property string title: ""
    property string description: ""

    spacing: K.Style.s1

    Row {
        spacing: K.Style.s2

        Rectangle {
            anchors.verticalCenter: parent.verticalCenter
            width: 14
            height: K.Style.border
            color: K.Style.lineStrong
        }

        K.Label {
            text: UiPreferences.brandMark("力")
            role: "micro"
            tone: "accent"
            font.family: K.Style.jp
        }

        K.Label {
            text: "ORPHEUS"
            role: "micro"
            tone: "muted"
        }
    }

    K.Label {
        width: parent.width
        text: head.title
        color: K.Style.ink
        font.family: K.Style.display
        font.pixelSize: 28
        font.weight: Font.Light
    }

    K.Label {
        width: parent.width
        text: head.description
        role: "body"
        tone: "muted"
        wrapMode: Text.WordWrap
    }
}
