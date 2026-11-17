import QtQuick

Text {
    id: root

    property alias name: root.text
    property color ink: Lithic.fg
    property int size: LithicTokens.size.iconPlaqueIcon

    color: ink
    font.family: Lithic.font.icons
    font.pixelSize: Math.round(root.size * Lithic.scaleOf(root))
    font.weight: Font.Normal
    renderType: Text.NativeRendering
    horizontalAlignment: Text.AlignHCenter
    verticalAlignment: Text.AlignVCenter
}
