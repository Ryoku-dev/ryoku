pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls as QQC
import QtQuick.Shapes as QS
import Lithic
import "../ask/look"

QQC.AbstractButton {
    id: root

    required property real s
    property string glyph: ""
    property int holdDuration: 650
    property bool subtle: false
    property bool labelled: true
    property bool danger: false
    // a whole sentence per action, so a language can word and case it its own way
    property string tip: ""
    signal held()

    readonly property bool round: QsLook.roundButtons && !root.showsLabel
    readonly property bool showsLabel: root.labelled && root.text.length > 0
    readonly property color accent: root.danger ? Lithic.jasper : Lithic.sel
    readonly property color accentInk: root.danger ? Lithic.inkOn(Lithic.jasper) : Lithic.onSel
    readonly property color ink: root.danger && (QsLook.lithic || QsLook.fluent) ? Lithic.jasper : Lithic.fg
    readonly property color rest: root.subtle && !QsLook.roundButtons ? "transparent"
        : QsLook.lithic ? "transparent"
        : QsLook.material ? Lithic.field
        : QsLook.adwaita ? Lithic.alpha(Lithic.fg, QsLook.dark ? 0.1 : 0.08)
        : Lithic.alpha(Lithic.fg, QsLook.dark ? 0.06 : 0.045)
    readonly property color plane: root.down ? Lithic.mix(Qt.colorEqual(root.rest, "transparent") ? QsLook.press : root.rest, Lithic.fg, Qt.colorEqual(root.rest, "transparent") ? 0 : 0.1)
        : root.hovered ? (Qt.colorEqual(root.rest, "transparent") ? QsLook.hover : Lithic.mix(root.rest, Lithic.fg, 0.06))
        : root.rest

    QtObject {
        id: hold
        property real progress: 0
        property real flash: 0
        property bool active: false
        property bool completed: false
    }

    hoverEnabled: true
    focusPolicy: Qt.StrongFocus
    implicitWidth: root.round ? Math.round(QsLook.holdHeight * root.s)
        : Math.ceil(label.implicitWidth + (root.glyph.length > 0 ? Math.round(26 * root.s) : 0) + Math.round(26 * root.s))
    implicitHeight: Math.round(QsLook.holdHeight * root.s)
    Accessible.name: root.text
    Accessible.description: root.tip

    function beginHold(): void {
        if (!root.enabled || hold.active)
            return;
        fillRun.stop();
        drainRun.stop();
        flashRun.stop();
        reducedHold.stop();
        hold.progress = 0;
        hold.completed = false;
        hold.active = true;
        if (QsLook.still)
            reducedHold.start();
        else
            fillRun.start();
    }

    function endHold(): void {
        if (!hold.active)
            return;
        hold.active = false;
        fillRun.stop();
        reducedHold.stop();
        if (hold.completed)
            return;
        if (QsLook.still)
            hold.progress = 0;
        else
            drainRun.start();
    }

    function completeHold(): void {
        if (!hold.active || hold.completed)
            return;
        hold.completed = true;
        hold.active = false;
        hold.progress = 1;
        root.held();
        flashRun.restart();
    }

    onPressed: root.beginHold()
    onReleased: root.endHold()
    onCanceled: root.endHold()
    onEnabledChanged: if (!enabled) root.endHold()
    Keys.onPressed: event => {
        if ((event.key === Qt.Key_Space || event.key === Qt.Key_Return || event.key === Qt.Key_Enter) && !event.isAutoRepeat) {
            root.beginHold();
            event.accepted = true;
        }
    }
    Keys.onReleased: event => {
        if ((event.key === Qt.Key_Space || event.key === Qt.Key_Return || event.key === Qt.Key_Enter) && !event.isAutoRepeat) {
            root.endHold();
            event.accepted = true;
        }
    }

    NumberAnimation {
        id: fillRun
        target: hold
        property: "progress"
        to: 1
        duration: root.holdDuration
        easing.type: Easing.Linear
        onFinished: root.completeHold()
    }
    NumberAnimation {
        id: drainRun
        target: hold
        property: "progress"
        to: 0
        duration: QsLook.dur(240)
        easing.type: Easing.OutCubic
    }
    Timer {
        id: reducedHold
        interval: root.holdDuration
        onTriggered: {
            hold.progress = 1;
            root.completeHold();
        }
    }
    SequentialAnimation {
        id: flashRun
        NumberAnimation { target: hold; property: "flash"; from: 0; to: 1; duration: QsLook.dur(90); easing.type: Easing.OutQuad }
        NumberAnimation { target: hold; property: "flash"; to: 0; duration: QsLook.dur(260); easing.type: Easing.OutCubic }
        ScriptAction { script: hold.progress = 0 }
    }

    scale: QsLook.material ? 1 + hold.flash * 0.04 : 1

    background: Item {
        id: plate

        readonly property real cut: Look.cutRow * root.s + (QsLook.lithic && (root.down || hold.flash > 0) ? Lithic.pressCutGrow * root.s : 0)

        CutRect {
            id: body
            visible: !root.round
            anchors.fill: parent
            cut: plate.cut
            fill: root.plane
            strokeColor: QsLook.lithic ? (root.danger ? Lithic.alpha(Lithic.jasper, 0.7) : Lithic.vein)
                : QsLook.fluent && !root.subtle ? Lithic.alpha(Lithic.fg, QsLook.dark ? 0.09 : 0.12)
                : "transparent"
            strokeWidth: Math.max(1, Math.round(root.s))
            Behavior on fill {
                ColorAnimation { duration: QsLook.dur(QsLook.stateMs) }
            }
            Behavior on cut {
                NumberAnimation { duration: QsLook.dur(LithicTokens.motion.press) }
            }
        }
        Rectangle {
            visible: root.round
            anchors.fill: parent
            radius: width / 2
            color: root.plane
        }

        // clipping a whole accent copy keeps the shape's left corners and a straight leading edge
        Item {
            visible: !QsLook.fluent && !root.round && hold.progress > 0
            width: parent.width * hold.progress
            height: parent.height
            clip: true

            CutRect {
                width: plate.width
                height: plate.height
                cut: plate.cut
                fill: root.accent
                opacity: 1 - hold.flash * 0.45
            }
        }
        Rectangle {
            visible: QsLook.fluent && hold.progress > 0
            anchors.left: parent.left
            anchors.bottom: parent.bottom
            anchors.leftMargin: Math.round(3 * root.s)
            width: (parent.width - Math.round(6 * root.s)) * hold.progress
            height: Math.max(2, Math.round(3 * root.s))
            radius: height / 2
            color: root.accent
        }
        Rectangle {
            visible: QsLook.fluent && hold.flash > 0
            anchors.fill: parent
            radius: Look.corner(Look.cutRow, width / root.s, height / root.s) * root.s
            color: Lithic.alpha(root.accent, 0.3 * hold.flash)
        }
        QS.Shape {
            visible: root.round && hold.progress > 0
            anchors.fill: parent
            preferredRendererType: QS.Shape.CurveRenderer

            QS.ShapePath {
                strokeColor: root.accent
                strokeWidth: Math.max(2, Math.round(3 * root.s))
                fillColor: "transparent"
                capStyle: QS.ShapePath.RoundCap
                PathAngleArc {
                    centerX: plate.width / 2
                    centerY: plate.height / 2
                    radiusX: plate.width / 2 - Math.max(2, Math.round(3 * root.s)) / 2
                    radiusY: radiusX
                    startAngle: -90
                    sweepAngle: 360 * hold.progress
                }
            }
        }
    }

    contentItem: Item {
        id: face
        implicitWidth: label.implicitWidth + (root.glyph.length > 0 ? Math.round(26 * root.s) : 0)

        Row {
            id: restFace
            anchors.centerIn: parent
            anchors.verticalCenterOffset: QsLook.lithic && root.down ? Lithic.pressSink * root.s : 0
            spacing: Math.round(7 * root.s)

            Symbol {
                visible: root.glyph.length > 0
                anchors.verticalCenter: parent.verticalCenter
                s: root.s
                name: root.glyph
                size: root.round ? 20 : 17
                ink: root.ink
                scale: root.down ? 0.88 : 1
                Behavior on scale {
                    NumberAnimation { duration: QsLook.dur(LithicTokens.motion.press); easing.type: Easing.OutCubic }
                }
            }
            Text {
                id: label
                visible: root.showsLabel
                anchors.verticalCenter: parent.verticalCenter
                text: root.text
                color: root.ink
                font.family: QsLook.sans
                font.pixelSize: Math.round(12 * root.s)
                font.weight: Font.Medium
                renderType: Text.NativeRendering
            }
        }

        Item {
            visible: !QsLook.fluent && !root.round && hold.progress > 0
            x: 0
            width: face.width * hold.progress
            height: face.height
            clip: true

            Row {
                x: restFace.x
                y: restFace.y
                spacing: restFace.spacing

                Symbol {
                    visible: root.glyph.length > 0
                    anchors.verticalCenter: parent.verticalCenter
                    s: root.s
                    name: root.glyph
                    size: 17
                    ink: root.accentInk
                    scale: root.down ? 0.88 : 1
                }
                Text {
                    visible: root.showsLabel
                    anchors.verticalCenter: parent.verticalCenter
                    text: root.text
                    color: root.accentInk
                    font.family: QsLook.sans
                    font.pixelSize: Math.round(12 * root.s)
                    font.weight: Font.Medium
                    renderType: Text.NativeRendering
                }
            }
        }
    }

    FocusFrame {
        s: root.s
        shown: root.visualFocus
        cut: Look.cutRow * root.s
        pill: root.round
    }
    CornerTip {
        s: root.s
        visible: root.tip.length > 0 && root.hovered && !root.down
        text: root.tip
    }
    HoverHandler {
        enabled: root.enabled
        cursorShape: Qt.PointingHandCursor
    }
}
