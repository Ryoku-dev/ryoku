import QtQuick
import "../look"

// A short value on the round designs' own chip: Material's tonal chip, the
// Adwaita osd pill, a Fluent tooltip card. It arrives in the design's motion
// (Material springs up from small, the others fade and settle) whenever shown.
Item {
    id: chip

    property real u: 1
    property string text: ""
    // a mark before the value, such as the record dot
    property string lead: ""
    property color leadInk: Look.alert
    property color fill: Look.plane
    property color ink: Look.fg
    property color stroke: "transparent"
    property real chipHeight: 28
    property real padX: 12
    // a corner radius in logical px; a pill when negative
    property real radius: -1
    property int elevation: 1
    property bool shadow: true
    property real fontSize: 12.5
    property int fontWeight: Font.DemiBold
    property string fontFamily: Look.sans
    property bool shown: true

    implicitWidth: Math.ceil(row.implicitWidth + chip.padX * 2 * chip.u)
    implicitHeight: Math.round(chip.chipHeight * chip.u)
    width: implicitWidth
    height: implicitHeight

    visible: opacity > 0
    opacity: chip.shown ? 1 : 0
    scale: chip.shown ? 1 : (Look.material ? 0.6 : 0.96)
    Behavior on opacity {
        enabled: chip.shown
        NumberAnimation { duration: Look.dur(Look.material ? 160 : 140); easing.type: Easing.OutCubic }
    }
    Behavior on scale {
        enabled: chip.shown
        NumberAnimation {
            duration: Look.dur(Look.material ? 380 : 160)
            easing.type: Easing.BezierSpline
            easing.bezierCurve: Look.material ? Look.springFast : Look.decelerate
        }
    }

    Surface {
        anchors.fill: parent
        u: chip.u
        radius: chip.radius < 0 ? chip.height / 2 : chip.radius * chip.u
        fill: chip.fill
        stroke: chip.stroke
        shadow: chip.shadow
        elevation: chip.elevation
        acrylic: false
    }
    Row {
        id: row
        anchors.centerIn: parent
        spacing: 6 * chip.u
        Text {
            visible: chip.lead !== ""
            anchors.verticalCenter: parent.verticalCenter
            text: chip.lead
            color: chip.leadInk
            font.family: chip.fontFamily
            font.pixelSize: Math.round(chip.fontSize * 0.8 * chip.u)
            renderType: Text.NativeRendering
        }
        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: chip.text
            color: chip.ink
            font.family: chip.fontFamily
            font.pixelSize: Math.round(chip.fontSize * chip.u)
            font.weight: chip.fontWeight
            font.features: ({ "tnum": 1 })
            renderType: Text.NativeRendering
        }
    }
}
