import QtQuick
import Lithic
import ".."

// An icon toggle in the design's checked, hover and press states: Lithic's cut
// sel plate, Material's state layer and primary squircle (a circle that morphs
// to a rounded square when checked, squishing as it is pressed), Adwaita's flat
// button with its shade, Fluent's subtle button with the accent when checked.
// The tooltip waits a beat and names the key that does the same.
Item {
    id: root

    property real u: 1
    property string icon: ""
    property real iconSize: 18
    property real cellWidth: 32
    property real cellHeight: 32
    property bool checked: false
    // close and other destructive buttons take the alert colour on hover; Fluent's
    // close fills with it, as a window's close button does
    property bool danger: false
    property string tip: ""
    property string keyHint: ""
    property bool tipBelow: false
    property color checkedFill: Look.checkedFill
    property color checkedInk: Look.checkedInk
    property color ink: Look.lithic || Look.material ? Look.fg2 : Look.fg
    // round: Material's circle and squircle; false: the design's small plate
    property bool round: Look.material

    signal clicked()

    readonly property bool hovered: area.containsMouse
    readonly property bool pressed: area.pressed

    implicitWidth: Math.round(root.cellWidth * root.u)
    implicitHeight: Math.round(root.cellHeight * root.u)
    width: implicitWidth
    height: implicitHeight
    opacity: root.enabled ? 1 : Lithic.disabledOpacity

    readonly property color fillNow: root.checked ? (root.hovered ? Lithic.mix(root.checkedFill, Look.fg, 0.08) : root.checkedFill)
        : root.pressed ? Look.press
        : root.hovered ? (root.danger ? (Look.fluent ? Look.alert : Lithic.alpha(Look.alert, 0.16)) : Look.hover)
        : Lithic.alpha(Look.hover, 0)
    readonly property color inkNow: root.checked ? root.checkedInk
        : root.danger && root.hovered ? (Look.fluent ? Look.onAlert : Look.alert)
        : root.hovered ? Look.fg
        : root.ink

    // the plate; Material morphs its corner, the others keep the design's own
    Item {
        id: plate
        anchors.centerIn: parent
        width: parent.width
        height: parent.height
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
            color: root.fillNow
            radius: root.round
                ? (root.checked ? Math.min(width, height) * 0.3 : Math.min(width, height) / 2)
                : (Look.adwaita ? 6 : 4) * root.u
            antialiasing: true
            Behavior on color {
                ColorAnimation { duration: Look.dur(Look.stateMs) }
            }
            Behavior on radius {
                NumberAnimation {
                    duration: Look.dur(300)
                    easing.type: Easing.BezierSpline
                    easing.bezierCurve: Look.springDefault
                }
            }
        }

        Icon {
            anchors.centerIn: parent
            anchors.verticalCenterOffset: Look.lithic && root.pressed ? Lithic.pressSink * root.u : 0
            name: root.icon
            size: root.iconSize * root.u
            tint: root.inkNow
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
        if (root.hovered && root.tip.length > 0) {
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
