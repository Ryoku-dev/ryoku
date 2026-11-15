import QtQuick
import Lithic
import "../../Singletons"

// Fonts & icons' preview: the interface font large and at the chosen size, and a line in the
// monospace font, each in the face picked on the page
Rectangle {
    id: root

    function v(key, fallback) {
        Changes.revision;
        var said = Changes.value({ store: "settings", key: key, fallback: fallback });
        return said === undefined || said === null || said === "" ? fallback : said;
    }
    readonly property string face: root.v("fontFamily", "Geologica")
    readonly property string mono: root.v("terminalFontFamily", "JetBrainsMono Nerd Font")
    readonly property real size: root.v("fontSize", 11)

    radius: Tone.radiusSmall
    color: Tone.card
    border.width: 1
    border.color: Tone.line

    Text {
        id: big
        x: 16
        y: 2
        text: "Aa"
        font.family: root.face
        font.pixelSize: 46
        font.weight: Font.DemiBold
        color: Tone.accent
    }
    Column {
        anchors.left: big.right
        anchors.leftMargin: 14
        anchors.right: parent.right
        anchors.rightMargin: 12
        anchors.verticalCenter: big.verticalCenter
        spacing: 2
        Text {
            width: parent.width
            text: root.face
            font.family: root.face
            font.pixelSize: 15
            font.weight: Font.DemiBold
            color: Lithic.fg
            elide: Text.ElideRight
        }
        Text {
            width: parent.width
            text: root.size + " pt"
            font.family: Lithic.font.mono
            font.pixelSize: 10
            color: Lithic.fg3
        }
    }
    Text {
        x: 16
        y: big.y + big.height - 4
        width: parent.width - 32
        text: "The quick brown fox jumps over the lazy dog."
        font.family: root.face
        font.pointSize: Math.max(6, root.size)
        color: Lithic.fg2
        elide: Text.ElideRight
    }
    Rectangle {
        x: 12
        anchors.bottom: parent.bottom
        anchors.bottomMargin: 8
        width: parent.width - 24
        height: 24
        radius: 5
        color: Lithic.alpha(Lithic.fg, 0.06)
        Text {
            x: 8
            anchors.verticalCenter: parent.verticalCenter
            width: parent.width - 16
            text: "$ ryoku update --check  ->  0 {}"
            font.family: root.mono
            font.pixelSize: 11
            color: Lithic.jade
            elide: Text.ElideRight
        }
    }
}
