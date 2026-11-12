import QtQuick
import Ryoku.Ui.Singletons

Grid {
    id: grid
    readonly property int columnCount: Math.max(2,
        Math.min(3, Math.floor((width + Tokens.s4) / 360)))
    readonly property real cardWidth: Math.floor(
        (width - (columnCount - 1) * columnSpacing) / columnCount)

    columns: columnCount
    columnSpacing: Tokens.s4
    rowSpacing: Tokens.s4
}
