import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../look"
import "../.."

// The panel's foot, where the design keeps its actions there. Lithic: the
// "◇ EVERY TIME" check row with a quiet "SAVE ⌃S" plaque, over a full-width
// "COPY ⌃C" sel plaque. Material: a tonal Save beside the extended FAB, Copy.
// Adwaita and Fluent keep their actions in the header and have no foot.
Item {
    id: ft

    required property var studio
    property real u: 1

    readonly property bool busy: ft.studio ? ft.studio.busy : false
    readonly property string copyText: ft.busy ? I18n.tr("Working\u2026") : I18n.tr("Copy")

    implicitHeight: Look.lithic ? Math.round(88 * ft.u) : Look.material ? Math.round(88 * ft.u) : 0
    height: implicitHeight
    visible: ft.height > 0

    // ---- Lithic
    Rectangle {
        visible: Look.lithic
        x: Math.round(16 * ft.u)
        width: parent.width - 2 * x
        height: Math.max(1, Math.round(ft.u))
        color: Look.vein
    }
    Item {
        id: checkRow
        visible: Look.lithic
        x: Math.round(16 * ft.u)
        y: Math.round(10 * ft.u)
        width: everyRow.width + Math.round(12 * ft.u)
        height: Math.round(24 * ft.u)
        readonly property bool on: ft.studio ? ft.studio.always : false

        CutRect {
            anchors.fill: parent
            anchors.leftMargin: -Math.round(6 * ft.u)
            design: "lithic"
            cut: Look.cutSm * ft.u
            fill: checkTap.pressed ? Look.press : checkHover.hovered ? Look.hover : Lithic.alpha(Look.hover, 0)
        }
        Row {
            id: everyRow
            anchors.verticalCenter: parent.verticalCenter
            spacing: Math.round(8 * ft.u)
            // the check: an outlined diamond, filled in sel when on
            Item {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.round(11 * ft.u)
                height: width
                CutRect {
                    anchors.fill: parent
                    design: "lithic"
                    cut: width / 2
                    fill: checkRow.on ? Look.sel : "transparent"
                    strokeColor: checkRow.on ? Look.sel : Look.fg2
                    strokeWidth: Math.max(1, Math.round(ft.u))
                    Behavior on fill {
                        ColorAnimation { duration: Look.dur(Look.stateMs) }
                    }
                }
                CutRect {
                    anchors.centerIn: parent
                    width: Math.round(3 * ft.u)
                    height: width
                    design: "lithic"
                    cut: width / 2
                    fill: Look.onSel
                    visible: checkRow.on
                }
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("Every time").toUpperCase()
                color: checkRow.on ? Look.fg : Look.fg2
                font.family: Look.mono
                font.pixelSize: Math.round(9.5 * ft.u)
                font.weight: Font.Bold
                font.letterSpacing: Look.tracking * ft.u
                renderType: Text.CurveRendering
            }
        }
        HoverHandler {
            id: checkHover
            cursorShape: Qt.PointingHandCursor
        }
        TapHandler {
            id: checkTap
            onTapped: ft.studio.setAlways(!checkRow.on)
        }
    }
    TextButton {
        visible: Look.lithic
        anchors.right: parent.right
        anchors.rightMargin: Math.round(16 * ft.u)
        anchors.verticalCenter: checkRow.verticalCenter
        u: ft.u
        cellHeight: 24
        text: I18n.tr("Save")
        keyHint: "\u2303S"
        enabled: !ft.busy
        onClicked: ft.studio.save()
    }
    TextButton {
        visible: Look.lithic
        x: Math.round(16 * ft.u)
        y: Math.round(44 * ft.u)
        width: parent.width - 2 * x
        u: ft.u
        primary: true
        cellHeight: 32
        text: ft.copyText
        keyHint: "\u2303C"
        enabled: !ft.busy
        onClicked: ft.studio.copy()
    }

    // ---- Material: the side sheet's bottom action area, its own band under a divider
    Rectangle {
        visible: Look.material
        width: parent.width
        height: Math.max(1, Math.round(ft.u))
        color: Look.divider
    }
    Row {
        visible: Look.material
        anchors.right: parent.right
        anchors.rightMargin: Math.round(24 * ft.u)
        anchors.verticalCenter: parent.verticalCenter
        spacing: Math.round(12 * ft.u)
        layoutDirection: Qt.LeftToRight

        TextButton {
            anchors.verticalCenter: parent.verticalCenter
            u: ft.u
            icon: "save"
            text: I18n.tr("Save")
            keyHint: "Ctrl+S"
            enabled: !ft.busy
            onClicked: ft.studio.save()
        }

        // the extended FAB: primaryContainer, the icon and the label
        Item {
            id: fab
            width: Math.ceil(fabRow.implicitWidth + 2 * Math.round(20 * ft.u))
            height: Math.round(56 * ft.u)
            enabled: !ft.busy
            opacity: enabled ? 1 : Lithic.disabledOpacity
            readonly property color container: Lithic.mix(Look.sel, Look.stone1, Look.dark ? 0.62 : 0.72)
            readonly property color ink: Lithic.mix(Look.sel, Look.fg, Look.dark ? 0.35 : 0.6)
            scale: fabTap.pressed ? 0.95 : 1
            Behavior on scale {
                NumberAnimation {
                    duration: Look.dur(fabTap.pressed ? 90 : 260)
                    easing.type: Easing.BezierSpline
                    easing.bezierCurve: fabTap.pressed ? Look.decelerate : Look.springFast
                }
            }

            Surface {
                anchors.fill: parent
                u: ft.u
                radius: 16 * ft.u
                fill: fabTap.pressed ? Lithic.mix(fab.container, fab.ink, 0.12)
                    : fabHover.hovered ? Lithic.mix(fab.container, fab.ink, 0.08) : fab.container
                stroke: "transparent"
                elevation: fabHover.hovered ? 2 : 1
                acrylic: false
            }
            Row {
                id: fabRow
                anchors.centerIn: parent
                spacing: Math.round(12 * ft.u)
                Icon {
                    anchors.verticalCenter: parent.verticalCenter
                    name: "copy"
                    size: Math.round(24 * ft.u)
                    tint: fab.ink
                }
                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: ft.copyText
                    color: fab.ink
                    font.family: Look.sans
                    font.pixelSize: Math.round(16 * ft.u)
                    font.weight: Font.Medium
                    renderType: Text.CurveRendering
                }
            }
            HoverHandler {
                id: fabHover
                cursorShape: Qt.PointingHandCursor
            }
            TapHandler {
                id: fabTap
                onTapped: ft.studio.copy()
            }
        }
    }
}
