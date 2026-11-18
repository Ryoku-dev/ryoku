import QtQuick
import QtQuick.Effects
import "../look"
import ".."

// Material's and Adwaita's loupe: the magnified pixels in a circle with a ring,
// a soft shadow under it, and the value on the design's chip below.
Item {
    id: loupe

    required property var chrome
    property real px: 0
    property real py: 0
    property real diameter: 104
    property real ringWidth: 3
    property color ring: Look.sel
    property color mark: Look.sel
    property string value: ""
    property color chipFill: Look.container
    property color chipInk: Look.containerInk
    property real chipHeight: 28
    property real chipRadius: -1
    property bool shown: false

    readonly property real u: loupe.chrome.u
    readonly property real d: loupe.chrome.snap(loupe.diameter * loupe.u)
    readonly property real gap: loupe.chrome.snap(8 * loupe.u)

    implicitWidth: Math.max(loupe.d, valueChip.implicitWidth)
    implicitHeight: loupe.d + loupe.gap + valueChip.implicitHeight

    visible: opacity > 0
    opacity: loupe.shown ? 1 : 0
    scale: loupe.shown ? 1 : (Look.material ? 0.7 : 0.94)
    Behavior on opacity { enabled: loupe.shown; NumberAnimation { duration: Look.dur(150); easing.type: Easing.OutCubic } }
    Behavior on scale {
        enabled: loupe.shown
        NumberAnimation {
            duration: Look.dur(Look.material ? 350 : 180)
            easing.type: Easing.BezierSpline
            easing.bezierCurve: Look.material ? Look.springFast : Look.enterCurve
        }
    }

    Item {
        id: disc
        anchors.horizontalCenter: parent.horizontalCenter
        width: loupe.d
        height: loupe.d

        RectangularShadow {
            anchors.fill: parent
            offset.y: 3 * loupe.u
            blur: 14 * loupe.u
            radius: width / 2
            color: Look.shadow
        }

        Loupe {
            id: lens
            anchors.fill: parent
            source: loupe.chrome.plate
            cx: loupe.px
            cy: loupe.py
            dpr: loupe.chrome.dpr
            u: loupe.u
            markColor: loupe.mark
            layer.enabled: loupe.visible
            layer.effect: MultiEffect {
                maskEnabled: true
                maskSource: mask
                maskThresholdMin: 0.5
                maskSpreadAtMin: 1
            }
        }
        Rectangle {
            id: mask
            anchors.fill: parent
            radius: width / 2
            visible: false
            layer.enabled: loupe.visible
        }
        Rectangle {
            anchors.fill: parent
            radius: width / 2
            color: "transparent"
            border.width: loupe.chrome.px2(loupe.ringWidth)
            border.color: loupe.ring
            antialiasing: true
        }
    }

    ValueChip {
        id: valueChip
        anchors.horizontalCenter: parent.horizontalCenter
        y: loupe.d + loupe.gap
        u: loupe.u
        text: loupe.value
        fill: loupe.chipFill
        ink: loupe.chipInk
        chipHeight: loupe.chipHeight
        radius: loupe.chipRadius
        fontSize: 12
        fontWeight: Font.Medium
    }
}
