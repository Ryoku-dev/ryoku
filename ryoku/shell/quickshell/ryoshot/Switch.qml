import QtQuick
import Lithic
import "look"

// An on/off switch in the design's own form: Lithic's cut track and square
// knob, Material's switch whose knob grows and takes a check, Adwaita's pill,
// Fluent's outlined toggle.
Item {
    id: root

    property real u: 1
    property bool checked: false
    signal toggled(bool v)

    readonly property real trackW: (Look.lithic ? 34 : Look.material ? 44 : Look.adwaita ? 44 : 40) * root.u
    readonly property real trackH: (Look.lithic ? 18 : Look.material ? 26 : Look.adwaita ? 24 : 20) * root.u
    readonly property real knob: (Look.lithic ? 12
        : Look.material ? (root.checked ? 20 : area.pressed ? 22 : 14)
        : Look.adwaita ? 18
        : (area.containsMouse ? 14 : 12)) * root.u
    readonly property real inset: (root.trackH - root.knob) / 2

    implicitWidth: Math.round(root.trackW)
    implicitHeight: Math.round(root.trackH)

    readonly property color trackFill: root.checked ? Look.sel
        : Look.lithic ? Look.well
        : Look.material ? Look.container
        : Look.adwaita ? Lithic.alpha(Look.fg, 0.15)
        : "transparent"
    readonly property color knobFill: root.checked ? (Look.adwaita ? (Look.dark ? Look.fg : Look.ground) : Look.onSel)
        : Look.adwaita ? (Look.dark ? Look.fg : Look.ground)
        : Look.lithic ? Look.fg3
        : Look.fg2

    CutRect {
        visible: Look.lithic
        anchors.fill: parent
        design: "lithic"
        cut: 4 * root.u
        fill: root.trackFill
        strokeColor: root.checked ? "transparent" : Look.edge
        strokeWidth: Math.max(1, Math.round(root.u))
        Behavior on fill {
            ColorAnimation { duration: Look.dur(Look.stateMs) }
        }
    }
    Rectangle {
        visible: !Look.lithic
        anchors.fill: parent
        radius: height / 2
        color: root.trackFill
        border.width: root.checked || Look.adwaita ? 0 : Math.max(1, Math.round((Look.material ? 2 : 1) * root.u))
        border.color: Look.material ? Look.fg3 : Look.fg2
        Behavior on color {
            ColorAnimation { duration: Look.dur(Look.stateMs) }
        }
    }

    Item {
        id: knob
        width: root.knob
        height: root.knob
        x: root.checked ? root.width - width - root.inset : root.inset
        anchors.verticalCenter: parent.verticalCenter
        Behavior on x {
            NumberAnimation {
                duration: Look.dur(Look.material ? 300 : 160)
                easing.type: Easing.BezierSpline
                easing.bezierCurve: Look.material ? Look.springDefault : Look.decelerate
            }
        }
        Behavior on width {
            NumberAnimation { duration: Look.dur(160); easing.type: Easing.OutCubic }
        }

        CutRect {
            visible: Look.lithic
            anchors.fill: parent
            design: "lithic"
            cut: 2 * root.u
            fill: root.knobFill
        }
        Rectangle {
            visible: !Look.lithic
            anchors.fill: parent
            radius: width / 2
            color: root.knobFill
        }
        Icon {
            visible: Look.material && root.checked
            anchors.centerIn: parent
            name: "check"
            size: 12 * root.u
            tint: Look.sel
        }
    }

    MouseArea {
        id: area
        anchors.fill: parent
        hoverEnabled: true
        cursorShape: Qt.PointingHandCursor
        onClicked: root.toggled(!root.checked)
    }
}
