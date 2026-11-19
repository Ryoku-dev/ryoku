pragma ComponentBehavior: Bound

import QtQuick
import Lithic

// Fluent's slider, drawn as a readout: a thin rail, the accent up to the value,
// the overdrive past 100% in the error colour, and the ringed thumb whose inner
// dot swells for a moment each time the value moves, as it does under a press.
Item {
    id: root

    property real value: 0
    property real maxValue: 1
    property color ink: Lithic.sel
    property color overInk: Lithic.jasper
    property real u: 1
    property color thumbPlane: Lithic.stone4
    property color thumbStroke: Lithic.vein

    readonly property real thumb: Math.round(14 * root.u)
    readonly property real rail: Math.max(2, Math.round(4 * root.u))
    readonly property real span: root.width - root.thumb
    readonly property real thumbX: root.thumb / 2 + root.span * root.level / root.maxValue
    readonly property real unityX: root.thumb / 2 + root.span / root.maxValue

    implicitHeight: root.thumb

    property real level: 0
    Behavior on level {
        NumberAnimation {
            duration: Lithic.duration(240)
            easing.type: Easing.BezierSpline
            easing.bezierCurve: [0.1, 0.9, 0.2, 1, 1, 1]
        }
    }
    Component.onCompleted: root.level = Qt.binding(() => Math.max(0, Math.min(root.maxValue, root.value)))

    property real swell: 0
    onValueChanged: if (!Lithic.reduceMotion) pulse.restart()
    SequentialAnimation {
        id: pulse
        NumberAnimation { target: root; property: "swell"; to: 1; duration: 90; easing.type: Easing.OutQuad }
        PauseAnimation { duration: 160 }
        NumberAnimation { target: root; property: "swell"; to: 0; duration: 260; easing.type: Easing.OutCubic }
    }

    Rectangle {
        x: root.thumb / 2
        anchors.verticalCenter: parent.verticalCenter
        width: root.span
        height: root.rail
        radius: height / 2
        color: Lithic.alpha(Lithic.fg, Lithic.dark ? 0.45 : 0.4)
    }
    Rectangle {
        visible: root.level > 1.005
        x: root.unityX - root.rail / 2
        anchors.verticalCenter: parent.verticalCenter
        width: root.thumbX - x
        height: root.rail
        radius: height / 2
        color: root.overInk
    }
    Rectangle {
        x: root.thumb / 2
        anchors.verticalCenter: parent.verticalCenter
        width: Math.max(0, Math.min(root.thumbX, root.unityX) - x)
        height: root.rail
        radius: height / 2
        color: root.ink
    }
    Rectangle {
        visible: root.maxValue > 1.005
        x: Math.round(root.unityX - width / 2)
        anchors.verticalCenter: parent.verticalCenter
        width: Math.max(1, Math.round(2 * root.u))
        height: Math.round(10 * root.u)
        radius: width / 2
        color: Lithic.fg2
    }

    Rectangle {
        x: root.thumbX - width / 2
        anchors.verticalCenter: parent.verticalCenter
        width: root.thumb
        height: width
        radius: width / 2
        color: root.thumbPlane
        border.width: Math.max(1, Math.round(root.u))
        border.color: root.thumbStroke

        Rectangle {
            anchors.centerIn: parent
            width: Math.round((7 + 2 * root.swell) * root.u)
            height: width
            radius: width / 2
            color: root.level > 1.005 ? root.overInk : root.ink
        }
    }
}
