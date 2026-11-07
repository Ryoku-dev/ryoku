import QtQuick
import Ryoku.Ui.Singletons

// The block a page puts a live preview in. Shell, Widgets, Appearance and
// ryowalls each grew their own, with their own gradient and their own badge, so
// the previews look like four products. The frame is here; what goes inside is
// the page's, because only the page knows what it is previewing.
//
// It is a block, not a cell: a preview is not a setting and does not belong in
// one. It spans the sheet's grid and the rows flow around it.
Item {
    id: prev

    property string label: I18n.tr("LIVE PREVIEW")
    property string tag: ""            // the corner readout: an output, a size
    property bool live: true           // false draws the off state instead
    property string offText: I18n.tr("OFF")
    default property alias content: slot.data

    implicitHeight: 200

    Rectangle {
        anchors.fill: parent
        color: "transparent"
        radius: Tokens.radius
        border.width: Tokens.border
        border.color: Tokens.line
    }
    Ticks { }

    Row {
        id: lab
        anchors { left: parent.left; top: parent.top; margins: Tokens.s4 }
        spacing: Tokens.s2
        Text {
            text: "//"
            color: Tokens.inkFaint
            font.family: Tokens.mono
            font.pixelSize: 10
        }
        Text {
            text: I18n.tr(prev.label)
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: 10
            font.weight: Font.Medium
            font.letterSpacing: Tokens.trackLabel
        }
    }
    Text {
        anchors { right: parent.right; top: parent.top; margins: Tokens.s4 }
        visible: prev.tag !== ""
        text: prev.tag
        color: Tokens.inkFaint
        font.family: Tokens.mono
        font.pixelSize: 9
    }

    Item {
        id: slot
        anchors {
            left: parent.left; right: parent.right
            top: lab.bottom; bottom: parent.bottom
            leftMargin: Tokens.s4; rightMargin: Tokens.s4
            topMargin: Tokens.s3; bottomMargin: Tokens.s4
        }
        visible: prev.live
        clip: true
    }

    // the off state says which thing is off, in words. there is no red here and
    // does not need to be: the hazard sheet this language comes from is black
    // and white and reads as more serious for it.
    Text {
        anchors.centerIn: parent
        visible: !prev.live
        text: prev.offText
        color: Tokens.inkMuted
        font.family: Tokens.ui
        font.pixelSize: 12
        font.letterSpacing: 2
    }
}
