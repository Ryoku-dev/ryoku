import QtQuick
import Quickshell
import Caelestia.Config

Region {
    required property real bX
    required property real bY
    required property real bW
    required property real bH
    required property real inLeft
    required property real inRight
    required property real inTop
    required property real inBottom

    Region {
        x: bX
        y: inTop
        width: Math.max(0, bW)
        height: Math.max(0, inBottom - inTop)
    }
    Region {
        x: inLeft
        y: bY
        width: Math.max(0, inRight - inLeft)
        height: Math.max(0, bH)
    }
}
