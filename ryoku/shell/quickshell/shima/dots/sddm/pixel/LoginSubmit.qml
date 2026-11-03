// The one filled control: an accent disc that arrives with the first character and turns while signing in.
import QtQuick 2.15
import QtQuick.Shapes
import "."

Item {
    id: submit
    property real d: 1
    property bool shown: false
    property bool busy: false
    property color fill: "#a8c7fa"
    property color ink: "#101012"
    property string symbolFont: ""
    signal activated()

    readonly property var expressive: [0.16, 1, 0.3, 1, 1, 1]
    opacity: submit.shown || submit.busy ? 1 : 0
    scale: submit.shown || submit.busy ? 1 : 0.5
    visible: opacity > 0
    Behavior on opacity { NumberAnimation { duration: 140; easing.type: Easing.OutCubic } }
    Behavior on scale { NumberAnimation { duration: 280; easing.type: Easing.BezierSpline; easing.bezierCurve: submit.expressive } }

    Rectangle {
        anchors.fill: parent
        radius: width / 2
        color: area.pressed ? Qt.darker(submit.fill, 1.12) : area.containsMouse ? Qt.lighter(submit.fill, 1.08) : submit.fill
    }
    MSymbol {
        anchors.centerIn: parent
        visible: !submit.busy
        text: "arrow_forward"
        symFont: submit.symbolFont
        iconSize: Math.round(submit.height * 0.46)
        iconColor: submit.ink
    }
    Shape {
        anchors.centerIn: parent
        width: Math.round(submit.height * 0.46)
        height: width
        visible: submit.busy
        preferredRendererType: Shape.CurveRenderer
        RotationAnimation on rotation { running: submit.busy; from: 0; to: 360; duration: 900; loops: Animation.Infinite }
        ShapePath {
            fillColor: "transparent"
            strokeColor: submit.ink
            strokeWidth: Math.max(2, Math.round(2.5 * submit.d))
            capStyle: ShapePath.RoundCap
            PathAngleArc {
                centerX: submit.height * 0.23; centerY: centerX
                radiusX: submit.height * 0.23 - Math.max(1, Math.round(1.25 * submit.d)); radiusY: radiusX
                startAngle: -90; sweepAngle: 250
            }
        }
    }
    MouseArea {
        id: area
        anchors.fill: parent
        enabled: !submit.busy
        hoverEnabled: true
        cursorShape: Qt.PointingHandCursor
        onClicked: submit.activated()
    }
}
