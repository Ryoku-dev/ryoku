import QtQuick
import Lithic

// Orpheus's design-aware chrome shape: a cut plaque in Lithic, a rounded rect in
// the round styles, drawn by the Lithic library so the corner is exactly the one
// the desktop design prescribes (Lithic.cornerRadius). Colour is passed in from
// the shell palette (Style), never chosen here, so the design changes geometry
// only. Drop it in where a chrome Rectangle used to be; children land on top.
CutRect {
    id: root

    // The cut size in reference px; the design turns it into a chamfer or radius.
    property real cutSize: Look.cutSm
    // Per-corner overrides default to the uniform cut; a welded edge zeroes its
    // corner so the shape melts into the bar instead of boxing it.
    cut: cutSize
    fill: "transparent"
    strokeColor: "transparent"
    strokeWidth: 0
}