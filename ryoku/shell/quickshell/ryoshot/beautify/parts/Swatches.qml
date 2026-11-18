import QtQuick
import "../../look"
import "../.."

// The literal colours to pick from, set out as the design sets a palette:
// Lithic's one tight row of cut chips, two rows of discs in Material and
// Adwaita, Fluent's grid of small rounded squares.
Grid {
    id: sw

    property real u: 1
    property var colors: []
    property color current: "white"
    signal picked(color c)

    readonly property int cols: Look.lithic ? 12 : 6
    readonly property real cell: Look.lithic ? 20 : Look.fluent ? 36 : 32

    columns: sw.cols
    columnSpacing: Math.max(0, (sw.width - sw.cols * Math.round(sw.cell * sw.u)) / (sw.cols - 1))
    rowSpacing: Math.round((Look.lithic ? 6 : Look.fluent ? 6 : 10) * sw.u)

    Repeater {
        model: sw.colors
        delegate: Chip {
            required property var modelData
            u: sw.u
            size: sw.cell
            ink: modelData
            selected: Qt.colorEqual(sw.current, modelData)
            onTapped: sw.picked(modelData)
        }
    }
}
