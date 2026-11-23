import QtQuick
import "../kit" as K
import "lib/geometry.js" as Geometry

// The selection: one liquid bead behind the rows. It glides to a row the user
// picks, stretching along its travel and squashing as it lands, and stays glued
// to a selected row that slides. Lithic sets it as the print language's bone
// plate, Material a tonal pill, Adwaita a rounded tint, Fluent a tint with a hairline.
Item {
    id: root

    property real rowHeight: K.Style.rowH
    property real targetY: 0
    property real travelY: 0
    property real squash: 0
    readonly property bool animated: K.Look.microMotion && !K.Style.still

    height: Geometry.beadHeight(root.rowHeight, root.travelY - root.targetY) * (1 - 0.08 * root.squash)
    y: root.travelY + root.rowHeight / 2 - root.height / 2

    function moveTo(y, glide) {
        root.targetY = y;
        if (glide && root.animated && Math.abs(y - root.travelY) > 0.5) {
            travel.from = root.travelY;
            travel.to = y;
            travel.restart();
            return;
        }
        travel.stop();
        root.travelY = y;
    }

    function follow(y) {
        travel.stop();
        root.targetY = y;
        root.travelY = y;
    }

    NumberAnimation {
        id: travel
        target: root
        property: "travelY"
        duration: K.Style.ms(K.Look.stateMs + 60)
        easing.type: Easing.BezierSpline
        easing.bezierCurve: K.Look.enterCurve
        onFinished: {
            if (root.animated)
                land.restart();
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
