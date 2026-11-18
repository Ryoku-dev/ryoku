import QtQuick
import Lithic
import "look"

// A labelled value slider: name on the left, live numeric readout on the right,
// the track below in the design's own form: Lithic's blade with a cut handle,
// Material's thick track and bar handle, Adwaita's thin track and round knob,
// Fluent's track with a ringed thumb. Click or drag anywhere on the row; emits
// moved(value) continuously.
//
// detailed is the Beautify panel's fuller form: Lithic's tick ruler with a short
// blade, Material 3 Expressive's 16 px track and 44 tall bar with a value bubble,
// Fluent's value tooltip while dragging. labels off leaves the track alone, for a
// row that names the value itself.
Item {
    id: sl

    property real u: 1
    property string label: ""
    property real from: 0
    property real to: 100
    property real value: 0
    property int decimals: 0
    property string suffix: ""
    property bool bipolar: false
    property bool detailed: false
    property bool labels: true
    signal moved(real v)
    signal released()

    readonly property bool ruler: Look.lithic && sl.detailed
    readonly property bool expressive: Look.material && sl.detailed
    // the height the handle and its track take
    readonly property real handleZone: Math.round((sl.expressive ? 44 : sl.ruler ? 18 : Look.material ? 28 : 20) * sl.u)
    readonly property real labelZone: sl.labels ? Math.round((sl.expressive ? 22 : sl.ruler ? 16 : 20) * sl.u) : 0

    implicitWidth: Math.round(240 * sl.u)
    implicitHeight: sl.detailed || !sl.labels ? sl.labelZone + sl.handleZone : Math.round(44 * sl.u)

    readonly property real frac: sl.to > sl.from ? Math.max(0, Math.min(1, (sl.value - sl.from) / (sl.to - sl.from))) : 0
    readonly property bool active: ma.pressed
    readonly property bool hovered: ma.containsMouse

    function fmt(v) { return (sl.decimals > 0 ? v.toFixed(sl.decimals) : String(Math.round(v))) + sl.suffix; }
    function setFromX(x) {
        var t = track.width > 0 ? Math.max(0, Math.min(1, x / track.width)) : 0;
        sl.moved(sl.from + t * (sl.to - sl.from));
    }

    Text {
        visible: sl.labels
        anchors.left: parent.left
        anchors.top: parent.top
        text: Look.lithic ? sl.label.toUpperCase() : sl.label
        color: Look.lithic ? Look.fg3 : sl.expressive ? Look.fg : Look.fg2
        font.family: Look.lithic ? Look.mono : Look.sans
        font.pixelSize: Math.round((Look.lithic ? 9 : sl.expressive ? 14 : 12) * sl.u)
        font.weight: Look.lithic ? Font.Bold : Font.Medium
        font.letterSpacing: Look.lithic ? Look.tracking * sl.u : 0
        renderType: Text.CurveRendering
    }
    Text {
        visible: sl.labels
        anchors.right: parent.right
        anchors.top: parent.top
        text: sl.fmt(sl.value)
        color: sl.active ? Look.sel : sl.expressive ? Look.fg2 : Look.fg
        font.family: Look.lithic ? Look.mono : Look.sans
        font.pixelSize: Math.round((Look.lithic ? 10.5 : sl.expressive ? 14 : 12) * sl.u)
        font.weight: Font.Medium
        font.features: ({ "tnum": 1 })
        renderType: Text.CurveRendering
        Behavior on color {
            ColorAnimation { duration: Look.dur(Look.stateMs) }
        }
    }

    readonly property real trackH: Math.round((sl.expressive ? 16 : Look.material ? 12 : sl.ruler ? 1 : 4) * sl.u)
    readonly property real fillX: sl.bipolar ? Math.min(0.5, sl.frac) * track.width : 0
    readonly property real fillW: sl.bipolar ? Math.abs(sl.frac - 0.5) * track.width : track.width * sl.frac
    readonly property real thumbX: track.width * sl.frac
    // the gap Material leaves on each side of its bar handle
    readonly property real gapM: 6 * sl.u

    Item {
        id: track
        anchors.left: parent.left
        anchors.right: parent.right
        y: sl.ruler ? sl.labelZone + sl.handleZone - height - Math.round(2 * sl.u)
            : sl.detailed || !sl.labels ? Math.round(sl.labelZone + (sl.handleZone - height) / 2)
            : parent.height - Math.round(8 * sl.u) - height
        height: sl.trackH

        // Lithic: an unfilled blade in the well with an edge hairline, the run in sel
        CutRect {
            visible: Look.lithic && !sl.ruler
            anchors.fill: parent
            design: "lithic"
            fill: Look.well
            strokeColor: Look.edge
            strokeWidth: Math.max(1, Math.round(sl.u))
        }
        Rectangle {
            visible: Look.lithic && !sl.ruler
            x: sl.fillX
            width: sl.fillW
            height: parent.height
            color: Look.sel
        }

        // Lithic's ruler: a baseline under fine ticks, every fifth taller, the run lit
        Rectangle {
            visible: sl.ruler
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            height: Math.max(1, Math.round(sl.u))
            color: Look.edge
        }
        Repeater {
            model: sl.ruler ? Math.max(2, Math.floor(track.width / (6 * sl.u)) + 1) : 0
            delegate: Rectangle {
                id: tick
                required property int index
                readonly property real at: tick.index * (track.width - width) / (Math.max(2, Math.floor(track.width / (6 * sl.u)) + 1) - 1)
                readonly property bool lit: tick.at + width / 2 >= sl.fillX && tick.at <= sl.fillX + sl.fillW
                x: Math.round(tick.at)
                y: track.height - height
                width: Math.max(1, Math.round(sl.u))
                height: Math.round((tick.index % 5 === 0 ? 9 : 5) * sl.u)
                color: tick.lit ? Look.sel : Look.vein
                Behavior on color {
                    ColorAnimation { duration: Look.dur(90) }
                }
            }
        }

        // Material: the inactive track and the active run, parted around the handle
        Rectangle {
            visible: Look.material
            x: Math.min(parent.width, sl.thumbX + sl.gapM)
            width: Math.max(0, parent.width - x)
            height: parent.height
            radius: sl.expressive ? Math.round(2 * sl.u) : height / 2
            topRightRadius: height / 2
            bottomRightRadius: height / 2
            color: Look.container
        }
        Rectangle {
            visible: Look.material && !sl.bipolar
            width: Math.max(0, sl.thumbX - sl.gapM)
            height: parent.height
            radius: sl.expressive ? Math.round(2 * sl.u) : height / 2
            topLeftRadius: height / 2
            bottomLeftRadius: height / 2
            color: Look.sel
        }
        // a centred slider keeps its inactive track on both sides, the run grows from the middle
        Rectangle {
            visible: Look.material && sl.bipolar
            width: Math.max(0, sl.thumbX - sl.gapM)
            height: parent.height
            radius: sl.expressive ? Math.round(2 * sl.u) : height / 2
            topLeftRadius: height / 2
            bottomLeftRadius: height / 2
            color: Look.container
        }
        Rectangle {
            visible: Look.material && sl.bipolar
            readonly property real mid: parent.width / 2
            x: sl.frac >= 0.5 ? mid : Math.min(mid, sl.thumbX + sl.gapM)
            width: sl.frac >= 0.5 ? Math.max(0, sl.thumbX - sl.gapM - mid) : Math.max(0, mid - x)
            height: parent.height
            radius: sl.expressive ? Math.round(2 * sl.u) : height / 2
            color: Look.sel
        }
        // M3 Expressive's stop indicator at the far end of the track
        Rectangle {
            visible: sl.expressive && !sl.bipolar && sl.thumbX < track.width - 16 * sl.u
            width: Math.round(4 * sl.u)
            height: width
            radius: width / 2
            x: track.width - width - Math.round(6 * sl.u)
            anchors.verticalCenter: parent.verticalCenter
            color: Look.sel
        }

        // Adwaita and Fluent: a thin rounded track and its accent run
        Rectangle {
            visible: Look.adwaita || Look.fluent
            anchors.fill: parent
            radius: height / 2
            color: Look.adwaita ? Lithic.alpha(Look.fg, 0.15) : Look.fg3
        }
        Rectangle {
            visible: Look.adwaita || Look.fluent
            x: sl.fillX
            width: sl.fillW
            height: parent.height
            radius: height / 2
            color: Look.sel
        }

        // the handle
        CutRect {
            visible: Look.lithic && !sl.ruler
            width: Math.round(8 * sl.u)
            height: Math.round(16 * sl.u)
            x: Math.max(0, Math.min(parent.width - width, sl.thumbX - width / 2))
            anchors.verticalCenter: parent.verticalCenter
            design: "lithic"
            cut: (2 + (sl.active ? 1 : 0)) * sl.u
            fill: sl.active || sl.hovered ? Look.sel : Look.fg
        }
        // the ruler's blade: a short cut sliver standing on the baseline
        CutRect {
            visible: sl.ruler
            width: Math.round(3 * sl.u)
            height: Math.round((sl.active ? 16 : 14) * sl.u)
            x: Math.max(0, Math.min(parent.width - width, sl.thumbX - width / 2))
            y: track.height - height + Math.round(2 * sl.u)
            design: "lithic"
            cutTopLeft: width / 2
            cutTopRight: width / 2
            fill: sl.active || sl.hovered ? Look.sel : Look.fg
            Behavior on height {
                NumberAnimation { duration: Look.dur(90); easing.type: Easing.OutCubic }
            }
        }
        Rectangle {
            visible: Look.material
            width: Math.round((sl.active ? 2 : 4) * sl.u)
            height: Math.round((sl.expressive ? 44 : 28) * sl.u)
            x: sl.thumbX - width / 2
            anchors.verticalCenter: parent.verticalCenter
            radius: width / 2
            color: Look.sel
            Behavior on width {
                NumberAnimation { duration: Look.dur(120) }
            }
        }
        Rectangle {
            visible: Look.adwaita
            width: Math.round(18 * sl.u)
            height: width
            radius: width / 2
            x: Math.max(0, Math.min(parent.width - width, sl.thumbX - width / 2))
            anchors.verticalCenter: parent.verticalCenter
            color: Look.dark ? Look.fg : Look.ground
            border.width: Math.max(1, Math.round(sl.u))
            border.color: Look.dark ? Lithic.alpha(Look.ground, 0.4) : Lithic.alpha(Look.fg, 0.2)
            scale: sl.active ? 1.08 : 1
        }
        Rectangle {
            id: fluentThumb
            visible: Look.fluent
            width: Math.round(20 * sl.u)
            height: width
            radius: width / 2
            x: Math.max(0, Math.min(parent.width - width, sl.thumbX - width / 2))
            anchors.verticalCenter: parent.verticalCenter
            color: Look.dark ? Lithic.mix(Look.plane, Look.fg, 0.2) : Look.stone4
            border.width: Math.max(1, Math.round(sl.u))
            border.color: Look.planes.cardStroke
            Rectangle {
                anchors.centerIn: parent
                width: Math.round((sl.active ? 10 : sl.hovered ? 14 : 12) * sl.u)
                height: width
                radius: width / 2
                color: Look.sel
                Behavior on width {
                    NumberAnimation { duration: Look.dur(Look.stateMs) }
                }
            }
        }

        // the value while dragging: Material's bubble, Fluent's tooltip card
        Item {
            id: bubble
            readonly property bool on: sl.detailed && sl.active && (Look.material || Look.fluent)
            width: Math.max(Math.round((Look.material ? 48 : 36) * sl.u), Math.ceil(bubbleText.implicitWidth + 20 * sl.u))
            height: Math.round((Look.material ? 44 : 30) * sl.u)
            x: Math.max(0, Math.min(track.width - width, sl.thumbX - width / 2))
            y: (track.height - sl.handleZone) / 2 - height - Math.round((Look.material ? 4 : 6) * sl.u)
            z: 10
            visible: opacity > 0
            opacity: bubble.on ? 1 : 0
            scale: bubble.on || Look.fluent ? 1 : 0.6
            transformOrigin: Item.Bottom
            Behavior on opacity {
                NumberAnimation { duration: Look.dur(bubble.on ? 120 : 90); easing.type: Easing.OutCubic }
            }
            Behavior on scale {
                NumberAnimation { duration: Look.dur(bubble.on ? 220 : 90); easing.type: Easing.BezierSpline; easing.bezierCurve: bubble.on ? Look.springFast : Look.accelerate }
            }

            Surface {
                anchors.fill: parent
                u: sl.u
                radius: Look.material ? parent.height / 2 : 4 * sl.u
                fill: Look.material ? Look.fg : Look.plane
                stroke: Look.material ? "transparent" : Look.planeStroke
                elevation: 1
                acrylic: false
            }
            Text {
                id: bubbleText
                anchors.centerIn: parent
                text: sl.fmt(sl.value)
                color: Look.material ? Look.ground : Look.fg
                font.family: Look.sans
                font.pixelSize: Math.round((Look.material ? 14 : 12) * sl.u)
                font.weight: Look.material ? Font.Medium : Font.Normal
                font.features: ({ "tnum": 1 })
                renderType: Text.CurveRendering
            }
        }
    }

    MouseArea {
        id: ma
        anchors.fill: parent
        anchors.topMargin: sl.detailed && sl.labels ? sl.labelZone - Math.round(4 * sl.u) : 0
        hoverEnabled: true
        preventStealing: true
        cursorShape: Qt.PointingHandCursor
        function apply(e) { var p = mapToItem(track, e.x, e.y); sl.setFromX(p.x); }
        onPressed: (e) => apply(e)
        onPositionChanged: (e) => { if (pressed) apply(e); }
        onReleased: sl.released()
    }
}
