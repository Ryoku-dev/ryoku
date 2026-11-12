import QtQuick
import Ryoku.Ui.Singletons
import "../kit"

Column {
    id: page

    required property var cfg

    readonly property bool stock: JSON.stringify(page.cfg.tiles) === JSON.stringify(page.cfg.defaultTiles)

    spacing: Md.seam

    SectionLabel { topPadding: 4; text: I18n.tr("Tiles") }
    ListItem {
        objectName: "editTiles"
        width: parent.width
        position: "first"
        clickable: false
        icon: "edit"
        headline: I18n.tr("Edit tiles from quick settings")
        supporting: I18n.tr("Open quick settings and press the pencil to add, remove, resize or move tiles")
        wrapSupporting: true
    }
    ListItem {
        objectName: "resetTiles"
        width: parent.width
        position: "last"
        clickable: false
        icon: "grid_view"
        headline: I18n.tr("Tile layout")
        supporting: page.stock ? I18n.tr("Using the default tiles")
            : I18n.tr("%1 tiles, arranged by you").arg(page.cfg.tiles.length)

        Button {
            anchors.verticalCenter: parent.verticalCenter
            variant: "tonal"
            icon: "restart_alt"
            text: I18n.tr("Reset tiles")
            enabled: !page.stock
            onClicked: page.cfg.setTiles(page.cfg.defaultTiles)
        }
    }
}
