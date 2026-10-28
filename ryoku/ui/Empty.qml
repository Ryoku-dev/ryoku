import QtQuick
import "Singletons"

// The empty state as a plate: the reference sheet's ring ornament around 空,
// the kanji for emptiness, over the caption. An empty zone is dressed, never
// a bare line of mono floating in a void.
Column {
    id: empty

    property string caption: ""

    spacing: Tokens.s3

    Item {
        width: 116
        height: 116
        anchors.horizontalCenter: parent.horizontalCenter
        Motif { anchors.fill: parent; kind: "rings"; strength: 0.14 }
        Text {
            anchors.centerIn: parent
            text: UiPreferences.text("空", "")
            color: Tokens.inkDim
            font.family: Tokens.jp
            font.pixelSize: 42
        }
    }
    Text {
        anchors.horizontalCenter: parent.horizontalCenter
        text: "// EMPTY_"
        color: Tokens.inkDim
        font.family: Tokens.mono
        font.pixelSize: 10
        font.letterSpacing: 1.4
    }
    Text {
        anchors.horizontalCenter: parent.horizontalCenter
        width: Math.min(implicitWidth, 380)
        horizontalAlignment: Text.AlignHCenter
        text: empty.caption
        color: Tokens.inkMuted
        font.family: Tokens.ui
        font.pixelSize: Tokens.fSmall
        wrapMode: Text.WordWrap
    }
}
