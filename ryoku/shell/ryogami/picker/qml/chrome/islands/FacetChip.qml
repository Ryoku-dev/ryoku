import QtQuick
import Lithic

// One choice in the filter sheet
CutRect {
    id: chip

    property string glyph: ""
    property string text: ""
    property string count: ""
    property bool on: false
    signal triggered()

    readonly property bool hovered: area.containsMouse
    height: 36 * Theme.scale
    width: row.implicitWidth + 28 * Theme.scale
    cut: (LithicTokens.cut.small + (area.pressed ? Lithic.pressCutGrow : 0)) * Theme.scale
    fill: chip.on ? Lithic.container : Lithic.alpha(Lithic.fg, chip.hovered ? 0.1 : 0.05)
    Behavior on fill { ColorAnimation { duration: Theme.fast } }
    Behavior on cut { NumberAnimation { duration: 90 } }

    Row {
        id: row
        anchors.centerIn: parent
        anchors.verticalCenterOffset: area.pressed ? 1 : 0
        spacing: 8 * Theme.scale
        Glyph {
            visible: chip.glyph.length > 0
            anchors.verticalCenter: parent.verticalCenter
            name: chip.glyph; size: 17
            ink: chip.on ? Lithic.containerInk : Lithic.fg2
        }
        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: chip.text
            font.family: Lithic.font.sans
            font.weight: Font.Medium
            font.pixelSize: Theme.fs(13)
            color: chip.on ? Lithic.containerInk : chip.hovered ? Lithic.fg : Lithic.fg2
            renderType: Text.NativeRendering
        }
        Text {
            visible: chip.count.length > 0
            anchors.verticalCenter: parent.verticalCenter
            text: chip.count
            font.family: Lithic.font.mono
            font.pixelSize: Theme.fs(10)
            color: chip.on ? Lithic.sel : Lithic.fg3
            renderType: Text.NativeRendering
        }
    }
    MouseArea {
        id: area
        anchors.fill: parent
        hoverEnabled: true
        cursorShape: Qt.PointingHandCursor
        onClicked: chip.triggered()
    }
}
