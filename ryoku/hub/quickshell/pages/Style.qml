import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/style"
import "../schema/style.js" as Schema

// the design language the whole desktop draws in, chosen from a gallery of the designs
// each drawn as itself, and the colour scheme the round ones wear
Item {
    id: root

    required property var hub

    readonly property var designRow: Schema.rows.find(r => r.id === "design")
    readonly property string chosen: {
        Changes.revision;
        var v = Changes.value(root.designRow.bind);
        return Schema.designs.some(d => d.value === v) ? v : "lithic";
    }

    function focusRow(rowId) {
        page.focusRow(rowId);
    }

    function choose(id) {
        var d = Schema.designs.find(x => x.value === id);
        Changes.set(root.designRow.bind, id, root.designRow.label, "style",
                    I18n.tr("Design set to %1").replace("%1", I18n.tr(d ? d.label : id)));
    }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "style"
        schema: Schema
        bodies: ({ design: gallery, colours: palette })
    }

    Component {
        id: gallery
        Grid {
            id: grid
            readonly property int across: width >= 760 ? 4 : 2
            columns: across
            columnSpacing: 12
            rowSpacing: 12

            Repeater {
                model: Schema.designs
                delegate: StyleCard {
                    required property var modelData
                    width: Math.floor((grid.width - (grid.across - 1) * grid.columnSpacing) / grid.across)
                    design: modelData.value
                    text: I18n.tr(modelData.label)
                    detail: I18n.tr(modelData.detail)
                    chosen: root.chosen === modelData.value
                    onClicked: root.choose(modelData.value)
                }
            }
        }
    }

    Component {
        id: palette
        PaletteStrip {}
    }
}
