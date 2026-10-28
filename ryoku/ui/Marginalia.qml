import QtQuick
import "Singletons"
import Ryoku.Ui.Singletons

// Printed-page marginalia for a chrome dead zone: the reference's masthead row
// distilled to a thin strip -- a pixel dingbat, a katakana gloss, a numbered
// index plate, a second dingbat and a chevron run. It dresses an empty margin
// (a head's right edge, a bar's centre, the rail foot) without crowding a
// control. Ink only: the acid accent stays on state, never on ornament.
Row {
    id: mg

    property string kana: ""
    property string index: ""
    property string label: ""
    property string glyph: "meander"   // leading pixel dingbat (Greek key)
    property string glyph2: "torii"    // trailing pixel dingbat ("" hides it)
    property bool chevrons: true

    spacing: Tokens.s3
    height: 20

    Pixel {
        anchors.verticalCenter: parent.verticalCenter
        width: 16; height: 16
        kind: mg.glyph
    }

    Text {
        visible: UiPreferences.showJapaneseGloss && mg.kana !== ""
        anchors.verticalCenter: parent.verticalCenter
        text: mg.kana; color: Tokens.inkFaint
        font.family: Tokens.jp; font.pixelSize: 12
    }

    Rectangle {
        visible: mg.index !== ""
        anchors.verticalCenter: parent.verticalCenter
        width: idx.implicitWidth + Tokens.s2 * 2
        height: 18
        color: "transparent"
        border.width: Tokens.border; border.color: Tokens.line
        Text {
            id: idx
            anchors.centerIn: parent
            text: mg.index + (mg.label !== "" ? " \u002f\u002f " + I18n.tr(mg.label) : "")
            color: Tokens.inkFaint
            font.family: Tokens.mono; font.pixelSize: 10; font.letterSpacing: 1.2
        }
    }

    Pixel {
        visible: mg.glyph2 !== ""
        anchors.verticalCenter: parent.verticalCenter
        width: 14; height: 14
        kind: mg.glyph2
    }

    Text {
        visible: mg.chevrons
        anchors.verticalCenter: parent.verticalCenter
        text: "\u276f\u276f\u276f\u276f\u276f\u276f"
        color: Tokens.inkFaint
        font.family: Tokens.mono; font.pixelSize: 11; font.letterSpacing: 2
        opacity: 0.55
    }
}
