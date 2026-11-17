import QtQuick
import Lithic
import "../../components"

Item {
    id: root

    property string symbol: ""
    property string brand: ""
    property int size: LithicTokens.size.iconPlaqueIcon
    property color ink: Lithic.fg

    readonly property real _scale: Lithic.scaleOf(root)

    implicitWidth: root.size * root._scale
    implicitHeight: root.size * root._scale

    Glyph {
        anchors.centerIn: parent
        visible: root.brand.length === 0
        name: root.symbol
        size: root.size
        ink: root.ink
    }

    GlyphIcon {
        anchors.centerIn: parent
        visible: root.brand.length > 0
        width: root.size * root._scale
        height: root.size * root._scale
        name: root.brand
        color: root.ink
        stroke: 1.7
    }
}
