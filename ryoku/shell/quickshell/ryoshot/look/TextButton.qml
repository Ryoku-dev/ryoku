import QtQuick
import Lithic
import ".."

// A labelled button. primary is the one action a surface leads with: Lithic's
// sel plaque in mono caps with its key hint, Material's filled primary pill,
// Adwaita's suggested action, Fluent's accent button. A secondary one is the
// design's flat or tonal button.
Item {
    id: root

    property real u: 1
    property string text: ""
    property string icon: ""
    property string keyHint: ""
    property bool primary: false
    property real cellHeight: Look.material ? 40 : Look.adwaita ? 34 : 32
    property string tip: ""
    property bool tipBelow: false

    signal clicked()

    readonly property bool hovered: area.containsMouse
    readonly property bool pressed: area.pressed

    readonly property real padX: (Look.material ? 16 : 12) * root.u
    implicitWidth: Math.ceil(row.implicitWidth + root.padX * 2)
    implicitHeight: Math.round(root.cellHeight * root.u)
    width: implicitWidth
    height: implicitHeight
    opacity: root.enabled ? 1 : Lithic.disabledOpacity

    readonly property color baseFill: root.primary ? Look.sel
        : Look.material ? Look.container
        : Look.fluent ? (Look.dark ? Lithic.alpha(Look.fg, 0.06) : Lithic.alpha(Look.ground, 0.7))
        : "transparent"
    readonly property color fillNow: root.primary
        ? (root.pressed ? Lithic.mix(Look.sel, Look.ground, 0.16) : root.hovered ? Lithic.hoverFill(Look.sel) : Look.sel)
        : root.pressed ? Look.press
        : root.hovered ? (root.baseFill.a > 0 ? Lithic.mix(root.baseFill, Look.fg, 0.08) : Look.hover)
        : root.baseFill
    readonly property color inkNow: root.primary ? Look.onSel
        : Look.material ? Look.containerInk
        : Look.fg

    Item {
        id: body
        anchors.fill: parent
        scale: Look.material && root.pressed ? 0.94 : 1
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
            strokeColor: root.primary ? "transparent" : Look.vein
            strokeWidth: root.primary ? 0 : Math.max(1, Math.round(root.u))
            Behavior on fill {
                ColorAnimation { duration: Look.dur(Look.stateMs) }
            }
        }
        Rectangle {
            visible: !Look.lithic
            anchors.fill: parent
            color: root.fillNow
            radius: Look.material ? height / 2 : (Look.adwaita ? 6 : 4) * root.u
            border.width: Look.fluent && !root.primary ? Math.max(1, Math.round(root.u)) : 0
            border.color: Look.planes.cardStroke
            antialiasing: true
            Behavior on color {
                ColorAnimation { duration: Look.dur(Look.stateMs) }
            }
        }

        Row {
            id: row
            anchors.centerIn: parent
            anchors.verticalCenterOffset: Look.lithic && root.pressed ? Lithic.pressSink * root.u : 0
            spacing: (Look.lithic ? 7 : 6) * root.u

            Icon {
                visible: root.icon.length > 0
                anchors.verticalCenter: parent.verticalCenter
                width: (Look.lithic ? 14 : 18) * root.u
                height: width
                name: root.icon
                size: width
                tint: root.inkNow
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: Look.lithic ? root.text.toUpperCase() : root.text
                color: root.inkNow
                font.family: Look.lithic ? Look.mono : Look.sans
                font.pixelSize: Math.round((Look.lithic ? 10.5 : Look.material ? 14 : 13) * root.u)
                font.weight: Look.lithic ? Font.Bold : Look.fluent ? Font.Normal : Font.DemiBold
                font.letterSpacing: Look.lithic ? Look.tracking * root.u : 0
                renderType: Text.CurveRendering
            }
            Text {
                visible: Look.lithic && root.keyHint.length > 0
                anchors.verticalCenter: parent.verticalCenter
                text: root.keyHint
                color: Lithic.alpha(root.inkNow, 0.62)
                font.family: Look.mono
                font.pixelSize: Math.round(9 * root.u)
                renderType: Text.CurveRendering
            }
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
        if (root.hovered && (root.tip.length > 0 || (!Look.lithic && root.keyHint.length > 0))) {
            tipDelay.restart();
        } else {
            tipDelay.stop();
            tipItem.shown = false;
        }
    }

    Tip {
        id: tipItem
        z: 100
        u: root.u
        text: root.tip.length > 0 ? root.tip : root.text
        keyHint: Look.lithic ? "" : root.keyHint
        anchors.horizontalCenter: parent.horizontalCenter
        y: root.tipBelow ? root.height + 6 * root.u : -height - 6 * root.u
    }
}
