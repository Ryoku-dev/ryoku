import QtQuick
import QtQuick.Effects
import Lithic
import "../../look"

// A padding grip on the midpoint of one background edge, in the design's
// handle: Lithic's tick grip, Material's pill, Adwaita's round knob, Fluent's
// circle with its light ring. Its travel is reported along the edge's outward
// normal, in window px, so the studio turns it into padding at its own scale.
Item {
    id: handle

    // t | r | b | l
    property string side: "r"
    property real u: 1

    signal started()
    signal moved(real travel)
    signal ended()

    readonly property bool across: handle.side === "t" || handle.side === "b"
    readonly property bool hot: area.containsMouse || area.pressed
    readonly property bool held: area.pressed

    // the hit square, centred on the edge point the parent places it at
    width: Math.round(28 * handle.u)
    height: width

    readonly property real nx: handle.side === "r" ? 1 : handle.side === "l" ? -1 : 0
    readonly property real ny: handle.side === "b" ? 1 : handle.side === "t" ? -1 : 0

    // Lithic: a bar laid along the edge and a tick standing out from it
    Item {
        visible: Look.lithic
        anchors.centerIn: parent
        readonly property real t: Math.max(1, Math.round(2 * handle.u))
        property real len: (handle.held ? 20 : handle.hot ? 18 : 14) * handle.u
        Behavior on len { NumberAnimation { duration: Look.dur(120); easing.type: Easing.OutCubic } }
        property real reach: (handle.held ? 12 : handle.hot ? 10 : 7) * handle.u
        Behavior on reach { NumberAnimation { duration: Look.dur(120); easing.type: Easing.OutCubic } }
        readonly property color ink: handle.hot ? Look.fg : Look.sel
        Rectangle {
            width: handle.across ? parent.len : parent.t
            height: handle.across ? parent.t : parent.len
            x: -width / 2
            y: -height / 2
            color: parent.ink
            antialiasing: false
            Behavior on color { ColorAnimation { duration: Look.dur(120) } }
        }
        Rectangle {
            width: handle.across ? parent.t : parent.reach
            height: handle.across ? parent.reach : parent.t
            x: handle.nx > 0 ? 0 : handle.nx < 0 ? -width : -width / 2
            y: handle.ny > 0 ? 0 : handle.ny < 0 ? -height : -height / 2
            color: parent.ink
            antialiasing: false
            Behavior on color { ColorAnimation { duration: Look.dur(120) } }
        }
    }

    // Material: a primary pill along the edge that thickens under the pointer
    // and narrows while held, as the M3 Expressive slider handle does
    Rectangle {
        visible: Look.material
        anchors.centerIn: parent
        property real thick: (handle.held ? 4 : handle.hot ? 8 : 6) * handle.u
        Behavior on thick {
            NumberAnimation { duration: Look.dur(handle.held ? 90 : 300); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.springFast }
        }
        property real span: (handle.hot ? 36 : 30) * handle.u
        Behavior on span {
            NumberAnimation { duration: Look.dur(300); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.springFast }
        }
        width: handle.across ? span : thick
        height: handle.across ? thick : span
        radius: Math.min(width, height) / 2
        color: Look.sel
        border.width: Math.max(1, Math.round(2 * handle.u))
        border.color: Look.onSel
        antialiasing: true
    }

    // Adwaita: the scale's round knob, light, with its shade border and shadow
    Item {
        visible: Look.adwaita
        anchors.centerIn: parent
        width: (handle.held ? 18 : 16) * handle.u
        height: width
        Behavior on width { NumberAnimation { duration: Look.dur(100); easing.type: Easing.OutCubic } }
        RectangularShadow {
            anchors.fill: knob
            radius: knob.radius
            offset.y: 1 * handle.u
            blur: 3 * handle.u
            color: Qt.rgba(0, 0, 0, 0.3)
        }
        Rectangle {
            id: knob
            anchors.fill: parent
            radius: width / 2
            color: handle.hot ? Lithic.mix(handle.light, Look.fg, 0.04) : handle.light
            border.width: Math.max(1, Math.round(handle.u))
            border.color: Qt.rgba(0, 0, 0, 0.18)
            antialiasing: true
        }
    }
    function luma(c) { return 0.2126 * c.r + 0.7152 * c.g + 0.0722 * c.b; }
    readonly property color light: handle.luma(Look.fg) > handle.luma(Look.ground) ? Look.fg : Look.ground

    // Fluent: the slider thumb, an accent dot in a light ring that grows under
    // the pointer and shrinks while held
    Rectangle {
        visible: Look.fluent
        anchors.centerIn: parent
        width: 20 * handle.u
        height: width
        radius: width / 2
        color: Look.planes.card
        border.width: Math.max(1, Math.round(handle.u))
        border.color: Look.planes.cardStroke
        antialiasing: true
        Rectangle {
            anchors.centerIn: parent
            property real d: (handle.held ? 9 : handle.hot ? 14 : 12) * handle.u
            Behavior on d { NumberAnimation { duration: Look.dur(167); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.fluentIn } }
            width: d
            height: d
            radius: d / 2
            color: Look.sel
            antialiasing: true
        }
    }

    MouseArea {
        id: area
        anchors.fill: parent
        hoverEnabled: true
        preventStealing: true
        cursorShape: handle.across ? Qt.SizeVerCursor : Qt.SizeHorCursor
        property real ox: 0
        property real oy: 0
        onPressed: (m) => {
            var p = mapToItem(null, m.x, m.y);
            ox = p.x;
            oy = p.y;
            handle.started();
        }
        onPositionChanged: (m) => {
            if (!pressed)
                return;
            var p = mapToItem(null, m.x, m.y);
            handle.moved((p.x - ox) * handle.nx + (p.y - oy) * handle.ny);
        }
        onReleased: handle.ended()
        onCanceled: handle.ended()
    }
}
