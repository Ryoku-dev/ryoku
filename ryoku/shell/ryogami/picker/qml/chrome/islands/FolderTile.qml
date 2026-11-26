import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// A folder shown by its own newest wallpapers, fanned like a hand of cards.
CutRect {
    id: tile

    property string collection: ""
    property string label: ""
    property string value: "*"
    property bool on: false
    signal triggered()

    readonly property bool hovered: area.containsMouse
    readonly property real s: Theme.scale
    height: 62 * s
    cut: (LithicTokens.cut.mid + (area.pressed ? Lithic.pressCutGrow : 0)) * s
    fill: tile.on ? Lithic.container : Lithic.alpha(Lithic.fg, tile.hovered ? 0.09 : 0.05)
    Behavior on fill { ColorAnimation { duration: Theme.fast } }

    LibraryView {
        id: inFolder
        collection: tile.collection
        folder: tile.value
        sort: "date"
    }

    Item {
        id: fan
        x: 8 * tile.s
        y: 8 * tile.s
        width: 46 * tile.s + 2 * (tile.hovered ? 22 : 16) * tile.s
        height: 46 * tile.s
        Repeater {
            model: Math.min(3, inFolder.count)
            ThumbClip {
                required property int index
                x: index * (tile.hovered ? 22 : 16) * tile.s
                z: 3 - index
                width: 46 * tile.s
                height: 46 * tile.s
                cutSize: 5 * tile.s
                decode: 128
                source: { inFolder.generation; inFolder.count; var e = inFolder.get(index); return e && e.thumb ? Library.fileUrl(e.thumb) : "" }
                Behavior on x { NumberAnimation { duration: Theme.standard; easing.type: Easing.OutCubic } }
            }
        }
    }
    Column {
        anchors.left: fan.right
        anchors.leftMargin: 14 * tile.s
        anchors.right: parent.right
        anchors.rightMargin: 12 * tile.s
        anchors.verticalCenter: parent.verticalCenter
        spacing: 3 * tile.s
        Text {
            width: parent.width
            elide: Text.ElideMiddle
            text: tile.label
            font.family: Lithic.font.sans
            font.weight: Font.Medium
            font.pixelSize: Theme.fs(14)
            color: tile.on ? Lithic.containerInk : Lithic.fg
            renderType: Text.NativeRendering
        }
        Text {
            text: I18n.tr("%1 walls").arg(inFolder.count)
            font.family: Lithic.font.mono
            font.pixelSize: Theme.fs(10)
            color: tile.on ? Lithic.sel : Lithic.fg3
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
