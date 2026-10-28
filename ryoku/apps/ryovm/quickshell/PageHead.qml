import QtQuick
import Ryoku.Ui.Singletons

// A page head, sized to its own content: the eyebrow lead (a rule, the 力 seal,
// a tracked label), the Fraunces title, and an optional blurb. Because it reports
// its real implicitHeight, the toolbar or content anchored below it can never
// ride up into the title -- one head grammar, spaced the same on every plate.
Item {
    id: head

    property string eyebrow: ""
    property string title: ""
    property string blurb: ""
    property color titleColor: Tokens.ink

    implicitHeight: col.implicitHeight

    Column {
        id: col
        anchors { left: parent.left; right: parent.right; top: parent.top }
        spacing: Tokens.s2

        Row {
            visible: head.eyebrow.length > 0
            spacing: Tokens.s2
            Rectangle { width: 16; height: 1; color: Tokens.ink; anchors.verticalCenter: parent.verticalCenter }
            Text { text: UiPreferences.brandMark("力"); color: Tokens.ink; font.family: Tokens.jp; font.pixelSize: 11; anchors.verticalCenter: parent.verticalCenter }
            Text {
                text: head.eyebrow
                color: Tokens.inkMuted
                font.family: Tokens.ui; font.pixelSize: 9
                font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                anchors.verticalCenter: parent.verticalCenter
            }
        }
        Text {
            visible: head.title.length > 0
            text: head.title
            color: head.titleColor
            font.family: Tokens.display
            font.pixelSize: Tokens.fHero
        }
        Text {
            visible: head.blurb.length > 0
            width: parent.width
            text: head.blurb
            color: Tokens.inkMuted
            font.family: Tokens.ui; font.pixelSize: 13
            wrapMode: Text.WordWrap
        }
    }
}
