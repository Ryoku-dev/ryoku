import QtQuick
import Lithic
import "../look"

// A dock cell that shows a value rather than a glyph (the ink, the stroke, the
// seal): the same hover, press and open plate as a GlyphButton, in the design's
// shape, with the tooltip naming its key.
Item {
    id: root

    property real u: 1
    property real cellWidth: 32
    property real cellHeight: 32
    // its popover is open
    property bool active: false
    property string tip: ""
    property string keyHint: ""
    property bool tipBelow: false
    property bool round: Look.material

    default property alias content: body.data
    signal clicked()

    readonly property bool hovered: area.containsMouse
    readonly property bool pressed: area.pressed

    implicitWidth: Math.round(root.cellWidth * root.u)
    implicitHeight: Math.round(root.cellHeight * root.u)
    width: implicitWidth
    height: implicitHeight

    readonly property color fillNow: root.pressed ? Look.press
        : root.active ? Look.press
        : root.hovered ? Look.hover
        : Lithic.alpha(Look.hover, 0)

    Item {
        id: plate
        anchors.fill: parent
        scale: Look.material && root.pressed ? 0.92 : 1
        Behavior on scale {
            NumberAnimation {
                duration: Look.dur(root.pressed ? 90 : 260)
                easing.type: Easing.BezierSpline
                easing.bezierCurve: root.pressed ? Look.decelerate : Look.springFast
            }
        }

        CutRect {
            visible: Look.lithic
            anchors.fill: parent
            design: "lithic"
            cut: (Look.cutSm + (root.pressed ? Lithic.pressCutGrow : 0)) * root.u
            fill: root.fillNow
            Behavior on fill {
                ColorAnimation { duration: Look.dur(Look.stateMs) }
            }
        }
        Rectangle {
            visible: !Look.lithic
            anchors.fill: parent
            radius: root.round ? Math.min(width, height) / 2 : (Look.adwaita ? 6 : 4) * root.u
            color: root.fillNow
            antialiasing: true
            Behavior on color {
                ColorAnimation { duration: Look.dur(Look.stateMs) }
            }
        }

        Item {
            id: body
            width: parent.width
            height: parent.height
            y: Look.lithic && root.pressed ? Lithic.pressSink * root.u : 0
        }
    }

    MouseArea {
        id: area
        anchors.fill: parent
        hoverEnabled: true
        cursorShape: Qt.PointingHandCursor
        onClicked: root.clicked()
    }

    Timer {
        id: tipDelay
        interval: 450
        onTriggered: tipItem.shown = true
    }
    onHoveredChanged: {
        if (root.hovered && root.tip.length > 0 && !root.active) {
            tipDelay.restart();
        } else {
            tipDelay.stop();
            tipItem.shown = false;
        }
    }
    onPressedChanged: if (root.pressed) { tipDelay.stop(); tipItem.shown = false; }

    Tip {
        id: tipItem
        z: 100
        u: root.u
        text: root.tip
        keyHint: root.keyHint
        anchors.horizontalCenter: parent.horizontalCenter
        y: root.tipBelow ? root.height + 6 * root.u : -height - 6 * root.u
    }
}
