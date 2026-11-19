import QtQuick
import Lithic
import "look"

// A small direction picker for the drop-shadow angle. 0 deg points right, 90
// points down (screen coordinates), so the handle sits where the shadow falls.
// Click or drag anywhere on the face. Emits moved(degrees). Lithic's face is a
// cut octagon, the round designs' a disc. detailed rings Lithic's face with
// engraved ticks and points a blade, a compass on the spec sheet. With no label
// it is the face alone.
Item {
    id: dial

    property real u: 1
    property real angle: 90
    property string label: ""
    property real size: 52
    property bool detailed: false
    signal moved(real a)

    readonly property bool compass: Look.lithic && dial.detailed
    readonly property real labelZone: dial.label.length > 0 ? Math.round(18 * dial.u) : 0
    readonly property real rad: dial.angle * Math.PI / 180

    implicitWidth: Math.max(Math.round(dial.size * dial.u) + Math.round(4 * dial.u), dial.label.length > 0 ? Math.round(56 * dial.u) : 0)
    implicitHeight: dial.labelZone + Math.round(dial.size * dial.u)

    Text {
        visible: dial.label.length > 0
        anchors.top: parent.top
        anchors.horizontalCenter: parent.horizontalCenter
        text: Look.lithic ? dial.label.toUpperCase() : dial.label
        color: Look.lithic ? Look.fg3 : Look.fg2
        font.family: Look.lithic ? Look.mono : Look.sans
        font.pixelSize: Math.round((Look.lithic ? 9 : 12) * dial.u)
        font.weight: Look.lithic ? Font.Bold : Font.Medium
        font.letterSpacing: Look.lithic ? Look.tracking * dial.u : 0
        renderType: Text.CurveRendering
    }

    Item {
        id: face
        width: Math.round(dial.size * dial.u)
        height: width
        anchors.bottom: parent.bottom
        anchors.horizontalCenter: parent.horizontalCenter
        readonly property real r: width / 2

        CutRect {
            visible: Look.lithic
            anchors.fill: parent
            design: "lithic"
            cut: face.width * 0.29
            fill: Look.well
            strokeColor: dragArea.pressed ? Look.sel : Look.edge
            strokeWidth: Math.max(1, Math.round(dial.u))
        }
        Rectangle {
            visible: !Look.lithic
            anchors.fill: parent
            radius: width / 2
            color: Look.material ? Look.container : Lithic.alpha(Look.fg, 0.06)
            border.width: Look.material ? 0 : Math.max(1, Math.round(dial.u))
            border.color: Lithic.alpha(Look.fg, 0.14)
        }

        // the compass's tick ring: every 15 degrees, the quarters longer; the
        // tick under the blade is lit
        Repeater {
            model: dial.compass ? 24 : 0
            delegate: Rectangle {
                id: tick
                required property int index
                readonly property bool quarter: tick.index % 6 === 0
                readonly property real a: tick.index * 15
                readonly property real d: Math.abs(((dial.angle - tick.a) % 360 + 540) % 360 - 180)
                readonly property real len: Math.round((tick.quarter ? 6 : 3) * dial.u)
                readonly property real inner: face.r * 0.74 - tick.len
                width: Math.max(1, Math.round(dial.u))
                height: tick.len
                x: face.r - width / 2
                y: face.r - tick.inner - tick.len
                transformOrigin: Item.Top
                transform: Rotation {
                    origin.x: tick.width / 2
                    origin.y: tick.inner + tick.len
                    angle: tick.a + 90
                }
                color: tick.d < 7.5 ? Look.sel : tick.quarter ? Look.fg3 : Look.vein
                antialiasing: true
            }
        }
        // the compass's blade from the hub to the ring
        Rectangle {
            visible: dial.compass
            width: face.r * 0.62
            height: Math.max(1, Math.round(1.5 * dial.u))
            x: face.r
            y: face.r - height / 2
            transformOrigin: Item.Left
            rotation: dial.angle
            color: Look.sel
            antialiasing: true
        }

        Rectangle {
            anchors.centerIn: parent
            width: Math.round((dial.compass ? 5 : 4) * dial.u)
            height: width
            radius: Look.lithic ? 0 : width / 2
            rotation: dial.compass ? 45 : 0
            color: dial.compass ? Look.fg : Look.fg3
        }
        Rectangle {
            width: Math.round((dial.compass ? 7 : 9) * dial.u)
            height: width
            radius: Look.lithic ? 0 : width / 2
            rotation: Look.lithic ? 45 : 0
            color: Look.sel
            readonly property real reach: dial.compass ? face.r * 0.66 : face.r - 9 * dial.u
            x: face.r - width / 2 + Math.cos(dial.rad) * reach
            y: face.r - height / 2 + Math.sin(dial.rad) * reach
        }

        MouseArea {
            id: dragArea
            anchors.fill: parent
            preventStealing: true
            cursorShape: Qt.PointingHandCursor
            function apply(e) {
                var dx = e.x - face.width / 2;
                var dy = e.y - face.height / 2;
                dial.moved((Math.atan2(dy, dx) * 180 / Math.PI + 360) % 360);
            }
            onPressed: (e) => apply(e)
            onPositionChanged: (e) => { if (pressed) apply(e); }
        }
    }
}
