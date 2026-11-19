import QtQuick
import Lithic

// A Material Symbols glyph in the design's own cut of the family: Sharp for
// Lithic, Rounded for Material and Adwaita, a thin Outlined for Fluent.
Text {
    id: root

    property real s: 1
    property alias name: root.text
    property real size: 18
    property color ink: Lithic.fg2
    // a chosen glyph fills in Material
    property bool chosen: false

    color: root.ink
    font.family: Look.icons
    font.pixelSize: Math.round(root.size * root.s)
    font.variableAxes: ({ "FILL": root.chosen && Look.fillChosen ? 1 : 0, "wght": Look.iconWeight })
    renderType: Text.NativeRendering
    horizontalAlignment: Text.AlignHCenter
    verticalAlignment: Text.AlignVCenter
}
