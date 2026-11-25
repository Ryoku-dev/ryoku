pragma Singleton

import QtQuick
import Quickshell

// Shared hover state for the dock's window-preview strip. RailDock writes the
// hovered app class and the hovered icon's screen-centre on hover; the
// DockPreviewPopout (hosted by FrameMenuManager, one per monitor) reads them to
// grow a live preview welded to that icon. Empty `hoveredClass` = nothing
// hovered, which lets the popout's own hover-grace melt it shut. Only the dock
// writes here, so a second dock instance on another rail simply retargets it.
Singleton {
    id: root

    // The app class currently hovered on the dock ("" = none). The popout shows
    // one live tile per open window of this class.
    property string hoveredClass: ""
    // Hovered icon centre, in global (screen) coordinates. The popout maps these
    // into its own space to weld the strip to the icon along the rail.
    property real gx: -1
    property real gy: -1
    // The rail edge the dock sits on ("left" | "right" | "top" | "bottom"), so
    // the strip grows off the correct side.
    property string edge: "left"
    // Depth (px, from the screen edge) a hovering dock reserves, so the preview
    // popout can clear it instead of overlapping. 0 for the thin frame rails.
    property real margin: 0

    // The output whose dock owns the current hover ("" = none). The strip is
    // hosted once per output, so only the popout on this output may open;
    // without it every output opens a copy, welded to the wrong icon.
    property string monitorName: ""

    // Per-output hold: the strip on that output is open, or the pointer is on
    // it. PreviewHold owns the map, so the logic lives in one place while this
    // preview keeps its own state.
    PreviewHold { id: holdMap }
    readonly property var holds: holdMap

    // Clear the hovered class, but only when that output owns the hover: a
    // dock on another output has already published its own hover by now, and
    // a stale leave must not erase it.
    function clearHover(monitor) {
        if (root.monitorName === String(monitor ?? ""))
            root.hoveredClass = "";
    }
}
