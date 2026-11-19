import QtQuick
import QtQuick.Effects
import "../look"
import "../lib/hittest.js" as Hit

// The round designs' selection handles: a filled circle with a ring at each
// corner and, where the design has them, pills or circles at the edge
// midpoints. A handle under the pointer grows; a held one grows a little less
// and takes the ring's colour, the way a pressed control darkens.
Item {
    id: handles

    required property var chrome
    property var rect: null
    property real cornerSize: 12
    // edge handles: a pill edgeLong by edgeShort along the edge, a circle when
    // edgeLong is 0, none when edges is false
    property bool edges: true
    property real edgeLong: 24
    property real edgeShort: 6
    property color fill: Look.sel
    property color ring: Look.ground
    property real ringWidth: 2
    property bool shadow: false
    property string hotRole: ""
    property string downRole: ""

    readonly property real u: handles.chrome.u

    Repeater {
        model: handles.edges ? Hit.handleRoles() : ["tl", "tr", "br", "bl"]

        Item {
            id: handle
            required property string modelData
            readonly property bool corner: handle.modelData.length === 2
            readonly property bool across: handle.modelData === "t" || handle.modelData === "b"
            readonly property bool pill: !handle.corner && handles.edgeLong > 0
            readonly property var c: handles.rect ? Hit.handleCenter(handles.rect, handle.modelData) : null
            readonly property real w: (handle.pill ? (handle.across ? handles.edgeLong : handles.edgeShort) : handles.cornerSize) * handles.u
            readonly property real h: (handle.pill ? (handle.across ? handles.edgeShort : handles.edgeLong) : handles.cornerSize) * handles.u
            readonly property bool hot: handles.hotRole === handle.modelData
            readonly property bool held: handles.downRole === handle.modelData

            visible: handle.c !== null
            x: handle.c ? handles.chrome.snap(handle.c.x - handle.w / 2) : 0
            y: handle.c ? handles.chrome.snap(handle.c.y - handle.h / 2) : 0
            width: handle.w
            height: handle.h
            scale: handle.held ? 1.12 : handle.hot ? 1.25 : 1
            Behavior on scale {
                NumberAnimation {
                    duration: Look.dur(Look.material ? 260 : 120)
                    easing.type: Easing.BezierSpline
                    easing.bezierCurve: Look.material ? Look.springFast : Look.decelerate
                }
            }

            RectangularShadow {
                visible: handles.shadow
                anchors.fill: parent
                offset.y: 1 * handles.u
                blur: 6 * handles.u
                radius: Math.min(width, height) / 2
                color: Look.shadow
            }
            Rectangle {
                anchors.fill: parent
                radius: Math.min(width, height) / 2
                color: handle.held ? handles.ring : handles.fill
                // a slim pill keeps most of its width for the fill
                border.width: (handle.pill ? Math.min(handles.ringWidth, handles.edgeShort / 6) : handles.ringWidth) * handles.u
                border.color: handle.held ? handles.fill : handles.ring
                antialiasing: true
                Behavior on color { ColorAnimation { duration: Look.dur(Look.stateMs) } }
            }
        }
    }
}
