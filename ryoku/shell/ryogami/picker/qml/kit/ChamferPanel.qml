import QtQuick
import Lithic

CutRect {
    id: panel

    property real reveal: 1

    cut: LithicTokens.cut.large * Theme.scale
    fill: Theme.withAlpha(Lithic.stone4, 0.99 * panel.reveal)
    strokeWidth: 1
    strokeColor: Theme.withAlpha(Lithic.vein, panel.reveal)
}
