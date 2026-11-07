import QtQuick
import Ryoku.Ui.Singletons

// The matte. The black is #000000; the grain is what stops it reading as a
// void. One layer, topmost, never per-surface.
Image {
    source: Qt.resolvedUrl("grain.png")
    fillMode: Image.Tile
    opacity: Tokens.grainOpacity
    z: 999
}
