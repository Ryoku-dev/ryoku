pragma Singleton

import QtQuick
import Quickshell

// Dock -> music-card hover state, mirroring DockPreview: DockMedia writes the
// chip's screen-centre and the band edge on hover; MusicHoverPopout reads them.
Singleton {
    id: root

    property bool hovered: false
    property string monitorName: ""
    property real gx: -1
    property real gy: -1
    property string edge: "bottom"
    property real margin: 0

    // Per-output hold: the card on that output is open, or the pointer is on
    // it. PreviewHold owns the map, so the logic lives in one place while this
    // preview keeps its own state.
    PreviewHold { id: holdMap }
    readonly property var holds: holdMap
}
