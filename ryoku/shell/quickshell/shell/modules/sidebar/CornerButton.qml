pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls as QQC
import Lithic
import "../ask/look"

QQC.AbstractButton {
    id: root

    property real s: 1
    property string glyph: ""
    property bool emphasis: false
    property bool subtle: false
    property bool danger: false
    property real turn: 0
    readonly property bool accented: root.checked || root.emphasis
    readonly property real side: Math.round(QsLook.iconButton * root.s)
    readonly property bool labelled: root.text.length > 0
    readonly property real cutPx: Look.cutRow * root.s
    readonly property color rest: root.danger && root.checked ? Lithic.alpha(Lithic.jasper, QsLook.dark ? 0.18 : 0.14)
        : root.accented ? (QsLook.lithic ? Lithic.container : Lithic.sel)
        : root.subtle ? "transparent"
        : QsLook.lithic ? "transparent"
        : QsLook.material ? Lithic.field
        : QsLook.adwaita ? Lithic.alpha(Lithic.fg, QsLook.dark ? 0.1 : 0.08)
        : Lithic.alpha(Lithic.fg, QsLook.dark ? 0.06 : 0.045)
    readonly property color ink: root.danger ? Lithic.jasper
        : root.accented ? (QsLook.lithic ? Lithic.sel : Lithic.onSel)
        : Lithic.fg2
    readonly property color plane: root.down ? Lithic.mix(Qt.colorEqual(root.rest, "transparent") ? QsLook.press : root.rest, Lithic.fg, Qt.colorEqual(root.rest, "transparent") ? 0 : 0.12)
        : root.hovered ? (Qt.colorEqual(root.rest, "transparent") ? QsLook.hover : Lithic.mix(root.rest, Lithic.fg, 0.07))
        : root.rest

    hoverEnabled: true
    focusPolicy: Qt.StrongFocus
    implicitHeight: root.side
    implicitWidth: root.labelled ? Math.ceil(content.implicitWidth + Math.round(24 * root.s)) : root.side
    opacity: root.enabled ? 1 : Lithic.disabledOpacity
    Accessible.name: root.text

    background: Item {
        CutRect {
            visible: !QsLook.roundButtons || root.labelled
            anchors.fill: parent
            cut: root.cutPx + (QsLook.lithic && root.down ? Lithic.pressCutGrow * root.s : 0)
            fill: root.plane
            strokeColor: QsLook.lithic && !root.subtle && !root.accented ? Lithic.vein
                : QsLook.fluent && !root.subtle && !root.accented ? Lithic.alpha(Lithic.fg, QsLook.dark ? 0.09 : 0.12)
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
            visible: QsLook.roundButtons && !root.labelled
            anchors.fill: parent
            radius: width / 2
            color: root.plane
            Behavior on color {
                ColorAnimation { duration: QsLook.dur(QsLook.stateMs) }
            }
        }
    }

    contentItem: Item {
        Row {
            id: content
            anchors.centerIn: parent
            anchors.verticalCenterOffset: QsLook.lithic && root.down ? Lithic.pressSink * root.s : 0
            spacing: Math.round(6 * root.s)

            Symbol {
                visible: root.glyph.length > 0
                anchors.verticalCenter: parent.verticalCenter
                s: root.s
                name: root.glyph
                size: QsLook.material ? 22 : 19
                ink: root.ink
                chosen: root.accented
                rotation: root.turn
                scale: root.down ? 0.85 : 1
                Behavior on rotation {
                    NumberAnimation {
                        duration: QsLook.dur(QsLook.material ? 320 : 220)
                        easing.type: QsLook.material || QsLook.adwaita ? Easing.OutBack : Easing.OutCubic
                        easing.overshoot: 1.4
                    }
                }
                Behavior on scale {
                    NumberAnimation { duration: QsLook.dur(LithicTokens.motion.press); easing.type: Easing.OutCubic }
                }
                Behavior on ink {
                    ColorAnimation { duration: QsLook.dur(QsLook.stateMs) }
                }
            }
            Text {
                visible: root.labelled
                anchors.verticalCenter: parent.verticalCenter
                text: root.text
                color: root.danger ? Lithic.jasper : root.accented && !QsLook.lithic ? Lithic.onSel : Lithic.fg
                font.family: QsLook.sans
                font.pixelSize: Math.round(13 * root.s)
                font.weight: Font.Medium
                renderType: Text.NativeRendering
            }
        }
    }

    FocusFrame {
        s: root.s
        shown: root.visualFocus
        cut: root.cutPx
        pill: QsLook.roundButtons && !root.labelled
    }
    CornerTip {
        s: root.s
        visible: root.hovered && !root.down && !root.labelled && root.Accessible.name.length > 0
        text: root.Accessible.name
    }
    HoverHandler {
        enabled: root.enabled
        cursorShape: Qt.PointingHandCursor
    }
}
