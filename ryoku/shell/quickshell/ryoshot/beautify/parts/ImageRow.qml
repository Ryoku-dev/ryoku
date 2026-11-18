import QtQuick
import Ryoku.Ui.Singletons
import "../../look"
import "../.."

// Choosing the background picture: Lithic's quiet plaque and Material's tonal
// button over the file's name, Adwaita's activatable row with a folder at its
// end, Fluent's clickable SettingsCard with its chevron.
Field {
    id: ir

    property string path: ""
    signal choose()

    readonly property bool asRow: Look.adwaita || Look.fluent
    readonly property string fileName: ir.path.length > 0 ? ir.path.slice(ir.path.lastIndexOf("/") + 1) : ""

    title: ir.asRow ? I18n.tr("Choose an image") : ""
    subtitle: ir.asRow ? ir.fileName : ""
    glyph: "image"
    activatable: ir.asRow
    onClicked: ir.choose()

    Icon {
        visible: ir.asRow
        name: Look.adwaita ? "folder" : "chevronRight"
        size: Math.round((Look.adwaita ? 16 : 14) * ir.u)
        tint: Look.fg
    }

    block: Column {
        visible: !ir.asRow
        width: parent ? parent.width : 0
        spacing: Math.round(8 * ir.u)
        TextButton {
            u: ir.u
            icon: "image"
            text: I18n.tr("Choose an image")
            cellHeight: Look.lithic ? 28 : 40
            onClicked: ir.choose()
        }
        Text {
            visible: ir.fileName.length > 0
            width: parent.width
            elide: Text.ElideMiddle
            text: Look.lithic ? ir.fileName.toUpperCase() : ir.fileName
            color: Look.fg3
            font.family: Look.lithic ? Look.mono : Look.sans
            font.pixelSize: Math.round((Look.lithic ? 9 : 12) * ir.u)
            renderType: Text.CurveRendering
        }
    }
}
