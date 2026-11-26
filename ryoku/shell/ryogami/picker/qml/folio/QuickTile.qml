import QtQuick
import Lithic

// A quick switch
CutRect {
    id: tile

    property string glyph: ""
    property string title: ""
    property string line: ""
    property bool on: false
    signal triggered()

    readonly property bool hovered: area.containsMouse
    readonly property real s: Theme.scale
    height: 84 * s
    cut: (LithicTokens.cut.mid + (area.pressed ? Lithic.pressCutGrow : 0)) * s
    fill: tile.on ? Lithic.sel : Lithic.alpha(Lithic.fg, tile.hovered ? 0.09 : 0.05)
    Behavior on fill { ColorAnimation { duration: Theme.standard } }
    Behavior on cut { NumberAnimation { duration: 90 } }

    CutRect {
        id: plate
        x: 16 * tile.s
        anchors.verticalCenter: parent.verticalCenter
        anchors.verticalCenterOffset: area.pressed ? 1 : 0
        width: 46 * tile.s
        height: width
        cut: LithicTokens.cut.small * tile.s
        fill: tile.on ? Lithic.alpha(Lithic.onSel, 0.14) : Lithic.alpha(Lithic.fg, 0.07)
        Glyph {
            anchors.centerIn: parent
            name: tile.glyph
            size: 22
            ink: tile.on ? Lithic.onSel : Lithic.fg2
            scale: area.pressed ? 0.9 : 1
            Behavior on scale { NumberAnimation { duration: Theme.fast; easing.type: Easing.OutBack } }
        }
    }
    Column {
        anchors.left: plate.right
        anchors.leftMargin: 14 * tile.s
        anchors.right: parent.right
        anchors.rightMargin: 14 * tile.s
        anchors.verticalCenter: parent.verticalCenter
        anchors.verticalCenterOffset: area.pressed ? 1 : 0
        spacing: 3 * tile.s
        Text {
            width: parent.width
            elide: Text.ElideRight
            text: tile.title
            font.family: Lithic.font.sans
            font.weight: Font.Medium
            font.pixelSize: Theme.fs(15)
            color: tile.on ? Lithic.onSel : Lithic.fg
            renderType: Text.NativeRendering
        }
        Text {
            width: parent.width
            elide: Text.ElideRight
            text: tile.line
            font.family: Lithic.font.sans
            font.pixelSize: Theme.fs(12)
            color: tile.on ? Lithic.alpha(Lithic.onSel, 0.75) : Lithic.fg3
            renderType: Text.NativeRendering
        }
    }
    MouseArea {
        id: area
        anchors.fill: parent
        hoverEnabled: true
        cursorShape: Qt.PointingHandCursor
        onClicked: tile.triggered()
    }
}
