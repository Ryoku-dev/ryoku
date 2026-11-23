import QtQuick
import "../kit" as K
import "lib/geometry.js" as Geometry

// The selection: one liquid bead behind the rows. It stretches along its travel
// and squashes as it lands. Lithic sets it as the print language's bone plate,
// Material a tonal pill, Adwaita a rounded tint, Fluent a tint with a hairline.
Item {
    id: root

    property real targetY: 0
    property real rowHeight: K.Style.rowH
    readonly property bool animated: K.Look.microMotion && !K.Style.still

    property real travelY: root.targetY
    property real squash: 0

    height: Geometry.beadHeight(root.rowHeight, root.travelY - root.targetY) * (1 - 0.08 * root.squash)
    y: root.travelY + root.rowHeight / 2 - root.height / 2

    Behavior on travelY {
        enabled: root.animated
        NumberAnimation {
            duration: K.Style.ms(K.Look.stateMs + 60)
            easing.type: Easing.BezierSpline
            easing.bezierCurve: K.Look.enterCurve
            onFinished: {
                if (root.animated)
                    land.restart();
            }
        }
    }

    SequentialAnimation {
        id: land
        NumberAnimation { target: root; property: "squash"; to: 1; duration: 30; easing.type: Easing.OutQuad }
        NumberAnimation { target: root; property: "squash"; to: 0; duration: 60; easing.type: Easing.OutQuad }
    }

    K.Shape {
        anchors.fill: parent
        anchors.leftMargin: -2 * root.squash
        anchors.rightMargin: -2 * root.squash
        cutSize: K.Look.cutSm
        fill: K.Look.lithic ? K.Style.bone : K.Look.material ? K.Style.tint16 : K.Style.tint10
        strokeWidth: K.Look.fluent ? K.Style.border : 0
        strokeColor: K.Look.fluent ? K.Style.lineSoft : "transparent"
    }
}
