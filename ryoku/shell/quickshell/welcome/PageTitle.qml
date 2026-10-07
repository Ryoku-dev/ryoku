import QtQuick
import Ryoku.Ui.Singletons

Column {
    id: root

    property string title: ""
    property string description: ""

    spacing: Tokens.s2

    Text {
        width: root.width
        text: root.title
        color: Tokens.ink
        font.family: Tokens.display
        font.pixelSize: Tokens.fTitle
        font.weight: Font.DemiBold
        wrapMode: Text.WordWrap
    }

    Text {
        width: root.width
        text: root.description
        color: Tokens.inkDim
        font.family: Tokens.ui
        font.pixelSize: Tokens.fBody
        wrapMode: Text.WordWrap
        lineHeight: 1.25
    }
}
