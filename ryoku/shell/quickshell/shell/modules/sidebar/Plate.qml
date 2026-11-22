import QtQuick
import Lithic

CutRect {
    property real s: 1

    cut: QsLook.cardCut * s
    fill: QsLook.card
    strokeColor: QsLook.cardStroke
    strokeWidth: Qt.colorEqual(strokeColor, "transparent") ? 0 : Math.max(1, Math.round(s))
}
