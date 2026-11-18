import QtQuick
import "../../modules"
import Ryoku.Ui
import Ryoku.Ui.Singletons

// One pickable Material Symbol on a hairline tile, or `fallback` text when
// `icon` is empty. Selection is the frame, as in the Identity mark grid.
Rectangle {
    id: tile
    property string icon: ""
    property string fallback: ""
    property bool selected: false
    property color ink: Tokens.ink
    signal picked()

    radius: Tokens.radius
    clip: true
    color: tileMa.containsMouse ? Tokens.tint5 : "transparent"
    border.width: tile.selected ? 2 : Tokens.border
    border.color: tile.selected ? Tokens.bone : (tileMa.containsMouse ? Tokens.ink : Tokens.line)
    Behavior on color { ColorAnimation { duration: Tokens.move } }
    Behavior on border.color { ColorAnimation { duration: Tokens.move } }

    IconText {
        visible: tile.icon !== ""
        anchors.centerIn: parent
        text: tile.icon
        color: tile.ink
        font.pixelSize: Tokens.fRow
    }
    UiText {
        visible: tile.icon === ""
        anchors.centerIn: parent
        text: tile.fallback
        color: tile.ink
        font.family: Tokens.mono
        font.pixelSize: Tokens.fBody
    }
    MouseArea {
        id: tileMa
        anchors.fill: parent
        hoverEnabled: true
        cursorShape: Qt.PointingHandCursor
        onClicked: tile.picked()
    }
}
