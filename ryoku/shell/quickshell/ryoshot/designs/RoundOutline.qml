import QtQuick
import "../look"

// The round designs' window hover: a rounded outline with a faint fill of its
// own colour around the window or output a click would take. It fades in and
// glides from one window to the next; tags read its rx, ry, rw and rh.
Item {
    id: outline

    required property var chrome
    property real radius: 12
    property real strokeWidth: 2
    property color stroke: Look.sel
    property real fillAlpha: 0.1

    readonly property var r: outline.chrome.hover
    property real rx: 0
    property real ry: 0
    property real rw: 0
    property real rh: 0
    property bool gliding: false

    opacity: outline.r ? 1 : 0
    visible: opacity > 0
    Behavior on opacity { NumberAnimation { duration: Look.dur(outline.r ? 140 : 90); easing.type: Easing.OutCubic } }
    Behavior on rx { enabled: outline.gliding; NumberAnimation { duration: Look.dur(Look.material ? 260 : 160); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.material ? Look.springDefault : Look.decelerate } }
    Behavior on ry { enabled: outline.gliding; NumberAnimation { duration: Look.dur(Look.material ? 260 : 160); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.material ? Look.springDefault : Look.decelerate } }
    Behavior on rw { enabled: outline.gliding; NumberAnimation { duration: Look.dur(Look.material ? 260 : 160); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.material ? Look.springDefault : Look.decelerate } }
    Behavior on rh { enabled: outline.gliding; NumberAnimation { duration: Look.dur(Look.material ? 260 : 160); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.material ? Look.springDefault : Look.decelerate } }
    onRChanged: {
        if (!outline.r) return;
        outline.gliding = outline.opacity > 0.5;
        outline.rx = outline.r.x;
        outline.ry = outline.r.y;
        outline.rw = outline.r.w;
        outline.rh = outline.r.h;
    }

    Rectangle {
        readonly property real sw: outline.chrome.px2(outline.strokeWidth)
        // the outline sits just outside the window so it never hides the window's own edge
        x: outline.chrome.snap(outline.rx) - sw
        y: outline.chrome.snap(outline.ry) - sw
        width: outline.chrome.snap(outline.rw) + sw * 2
        height: outline.chrome.snap(outline.rh) + sw * 2
        radius: outline.radius * outline.chrome.u + sw
        color: Qt.rgba(outline.stroke.r, outline.stroke.g, outline.stroke.b, outline.fillAlpha)
        border.width: sw
        border.color: outline.stroke
        antialiasing: true
    }
}
