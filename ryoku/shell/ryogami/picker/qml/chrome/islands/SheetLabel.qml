import QtQuick
import Lithic

// A group's name in a sheet
Text {
    font.family: Theme.lithic ? Lithic.font.mono : Lithic.font.sans
    font.pixelSize: Theme.fs(Theme.lithic ? 10 : 12)
    font.weight: Theme.lithic ? Font.Normal : Font.Medium
    font.letterSpacing: Theme.lithic ? 1.6 : 0
    font.capitalization: Theme.lithic ? Font.AllUppercase : Font.MixedCase
    color: Lithic.fg3
    renderType: Text.NativeRendering
}
