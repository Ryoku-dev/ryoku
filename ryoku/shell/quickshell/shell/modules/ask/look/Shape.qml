import QtQuick
import Lithic

// A plane of the bar: a cut plaque in Lithic, the design's rounded corner in the
// others. The cut is in reference px and scales with the bar.
CutRect {
    property real s: 1
    property real cutSize: Look.cutRow

    cut: cutSize * s
    fill: "transparent"
    strokeColor: "transparent"
    strokeWidth: 0
}
