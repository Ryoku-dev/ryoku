import QtQuick
import Lithic
import "../../ask/look"

// One line of the view's text. figures sets numerals the design's way: Martian
// Mono in Lithic, tabular Geologica in the others.
Text {
    id: root

    property real s: 1
    property real size: 13
    property bool figures: false
    property color ink: Lithic.fg

    color: root.ink
    font.family: root.figures ? Look.figures : Look.sans
    font.pixelSize: Math.round(root.size * (root.figures ? Look.figureScale : 1) * root.s)
    font.features: ({ "tnum": 1 })
    textFormat: Text.PlainText
    renderType: Text.NativeRendering
    elide: Text.ElideRight
}
