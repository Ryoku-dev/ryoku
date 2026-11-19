import QtQuick
import Lithic

// The small running label over a group: Lithic sets it in tracked mono capitals
// (a font setting, so the words stay whole for translation), the others plainly.
Text {
    id: root

    property real s: 1

    color: Lithic.fg3
    font.family: Look.monoMicro ? Look.mono : Look.sans
    font.pixelSize: Math.round((Look.monoMicro ? 9 : 12) * root.s)
    font.weight: Look.monoMicro ? Font.Normal : Font.Medium
    font.capitalization: Look.monoMicro ? Font.AllUppercase : Font.MixedCase
    font.letterSpacing: Look.tracking * root.s
    renderType: Text.NativeRendering
    elide: Text.ElideRight
}
