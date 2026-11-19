import QtQuick
import "../look"

// Print crop marks around a rect: an L at each corner, held off the rect, with
// arms running back along its edges. As editing grips they light under the
// pointer, and short ticks at the edge midpoints join them.
Item {
    id: marks

    required property var chrome
    // the rect in this item's coordinates
    property real rx: 0
    property real ry: 0
    property real rw: 0
    property real rh: 0
    property real length: 16
    property real thickness: 2
    property real offset: 4
    property color ink: Look.sel
    property color hotInk: Look.fg
    property bool grips: false
    property string hotRole: ""
    property string downRole: ""

    readonly property real t: marks.chrome.px2(marks.thickness)
    readonly property real len: marks.chrome.snap(marks.length * marks.chrome.u)
    readonly property real off: marks.chrome.snap(marks.offset * marks.chrome.u)
    readonly property real x0: marks.chrome.snap(marks.rx) - marks.off
    readonly property real y0: marks.chrome.snap(marks.ry) - marks.off
    readonly property real x1: marks.chrome.snap(marks.rx + marks.rw) + marks.off
    readonly property real y1: marks.chrome.snap(marks.ry + marks.rh) + marks.off

    function inkFor(role) {
        return marks.grips && (marks.downRole === role || marks.hotRole === role) ? marks.hotInk : marks.ink;
    }

    Repeater {
        model: [
            { role: "tl", sx: 1, sy: 1 },
            { role: "tr", sx: -1, sy: 1 },
            { role: "br", sx: -1, sy: -1 },
            { role: "bl", sx: 1, sy: -1 }
        ]
        Item {
            id: corner
            required property var modelData
            readonly property real cx: corner.modelData.sx > 0 ? marks.x0 : marks.x1
            readonly property real cy: corner.modelData.sy > 0 ? marks.y0 : marks.y1
            readonly property color ink: marks.inkFor(corner.modelData.role)
            readonly property bool held: marks.grips && marks.downRole === corner.modelData.role
            // a pressed grip reaches a little further along its edges
            property real reach: marks.len + (corner.held ? 4 * marks.chrome.u : 0)
            Behavior on reach { NumberAnimation { duration: Look.dur(120); easing.type: Easing.OutCubic } }

            Rectangle {
                x: corner.modelData.sx > 0 ? corner.cx : corner.cx - corner.reach
                y: corner.modelData.sy > 0 ? corner.cy : corner.cy - marks.t
                width: corner.reach
                height: marks.t
                color: corner.ink
                antialiasing: false
                Behavior on color { ColorAnimation { duration: Look.dur(120) } }
            }
            Rectangle {
                x: corner.modelData.sx > 0 ? corner.cx : corner.cx - marks.t
                y: corner.modelData.sy > 0 ? corner.cy : corner.cy - corner.reach
                width: marks.t
                height: corner.reach
                color: corner.ink
                antialiasing: false
                Behavior on color { ColorAnimation { duration: Look.dur(120) } }
            }
        }
    }

    Repeater {
        model: marks.grips ? ["t", "r", "b", "l"] : []
        Rectangle {
            id: tick
            required property string modelData
            readonly property bool across: tick.modelData === "t" || tick.modelData === "b"
            property real tickLen: marks.chrome.snap(8 * marks.chrome.u)
                + (marks.downRole === tick.modelData ? 4 * marks.chrome.u : 0)
            Behavior on tickLen { NumberAnimation { duration: Look.dur(120); easing.type: Easing.OutCubic } }
            x: tick.across
                ? marks.chrome.snap((marks.x0 + marks.x1) / 2 - marks.t / 2)
                : (tick.modelData === "l" ? marks.x0 + marks.t - tick.tickLen : marks.x1 - marks.t)
            y: tick.across
                ? (tick.modelData === "t" ? marks.y0 + marks.t - tick.tickLen : marks.y1 - marks.t)
                : marks.chrome.snap((marks.y0 + marks.y1) / 2 - marks.t / 2)
            width: tick.across ? marks.t : tick.tickLen
            height: tick.across ? tick.tickLen : marks.t
            color: marks.inkFor(tick.modelData)
            antialiasing: false
            Behavior on color { ColorAnimation { duration: Look.dur(120) } }
        }
    }
}
