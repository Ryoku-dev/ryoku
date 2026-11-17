pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import shell.services

// Persistent region-capture boundary. While a region recording runs (Quick or
// Studio), everything outside the captured box is dimmed and the box stays clear,
// so you see exactly what is being recorded and can keep moving windows around it.
// Fully click-through and on the Top layer -- below the shell's own chrome (the
// recording control bar), above app windows. gsr crops to the same box, and the
// box interior has no overlay pixels, so the dim never lands in the recording.
PanelWindow {
    id: win

    required property var modelData

    // This surface's screen: layout position and logical size in the same space
    // slurp reports the box in, so we can map global -> screen-local. The window's
    // own width/height are 0 until it maps, so gating `visible` on them deadlocks
    // (never sized -> never visible -> never sized); read the screen instead.
    readonly property real monX: modelData ? modelData.x : 0
    readonly property real monY: modelData ? modelData.y : 0
    readonly property real screenW: modelData ? modelData.width : 0
    readonly property real screenH: modelData ? modelData.height : 0

    // Recorder.regionGeom is "WxH+X+Y" (global logical); parse to this screen's coords.
    readonly property var box: {
        var m = /^(\d+)x(\d+)\+(-?\d+)\+(-?\d+)$/.exec(Recorder.regionGeom);
        if (!m)
            return null;
        return {
            x: parseInt(m[3]) - win.monX,
            y: parseInt(m[4]) - win.monY,
            w: parseInt(m[1]),
            h: parseInt(m[2])
        };
    }
    readonly property bool onScreen: box && box.x < win.screenW && box.y < win.screenH && (box.x + box.w) > 0 && (box.y + box.h) > 0

    // the clear box clamped to this screen.
    readonly property real bx: box ? Math.max(0, box.x) : 0
    readonly property real by: box ? Math.max(0, box.y) : 0
    readonly property real bw: box ? Math.min(win.screenW, box.x + box.w) - bx : 0
    readonly property real bh: box ? Math.min(win.screenH, box.y + box.h) - by : 0
    readonly property color dim: Qt.rgba(0, 0, 0, 0.45)

    screen: modelData
    visible: Recorder.active && Recorder.regionGeom !== "" && onScreen
    color: "transparent"
    exclusionMode: ExclusionMode.Ignore
    WlrLayershell.layer: WlrLayer.Top
    WlrLayershell.keyboardFocus: WlrKeyboardFocus.None
    WlrLayershell.namespace: "ryoku-region"

    anchors { top: true; bottom: true; left: true; right: true }

    // click-through: purely a visual boundary, never eats a press.
    mask: Region {}

    // four dim bands framing the clear box (top / bottom / left / right).
    Rectangle { color: win.dim; x: 0; y: 0; width: win.screenW; height: win.by }
    Rectangle { color: win.dim; x: 0; y: win.by + win.bh; width: win.screenW; height: win.screenH - (win.by + win.bh) }
    Rectangle { color: win.dim; x: 0; y: win.by; width: win.bx; height: win.bh }
    Rectangle { color: win.dim; x: win.bx + win.bw; y: win.by; width: win.screenW - (win.bx + win.bw); height: win.bh }
}
