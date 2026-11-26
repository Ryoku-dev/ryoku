import QtQuick
import Lithic

CutRect {
    id: chip

    property string label: ""
    property color tint: Theme.withAlpha(Theme.surfaceText, 0.7)
    property bool strong: false

    implicitWidth: chipText.implicitWidth + 16 * Theme.scale
    height: 22 * Theme.scale
    cut: 3 * Theme.scale
    fill: chip.strong ? Theme.withAlpha(chip.tint, 0.16) : "transparent"
    strokeWidth: 1
    strokeColor: Theme.withAlpha(chip.tint, chip.strong ? 0.55 : 0.3)

    Text {
        id: chipText
        anchors.centerIn: parent
        text: chip.label
        font.family: Theme.sans
        font.weight: Font.Medium
        font.pixelSize: Theme.fontFine
        font.letterSpacing: 0.8
        color: chip.strong ? chip.tint : Theme.withAlpha(Theme.surfaceText, 0.7)
        renderType: Text.NativeRendering
    }
}
