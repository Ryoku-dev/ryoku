pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls as QQC
import Lithic
import "../ask/look"

QQC.AbstractButton {
    id: root

    property real s: 1
    property bool on: false
    property string detail: ""
    property string glyph: ""
    property bool small: false
    signal toggleRequested(bool value)

    readonly property real k: root.s * (root.small ? 0.84 : 1)
    readonly property real trackW: Math.round((QsLook.lithic ? 32 : QsLook.material ? 46 : 40) * root.k)
    readonly property real trackH: Math.round((QsLook.lithic ? 18 : QsLook.material ? 28 : QsLook.adwaita ? 24 : 20) * root.k)
    readonly property real knob: Math.round((QsLook.lithic ? 12
        : QsLook.material ? (root.on ? 20 : root.down ? 24 : 14)
        : QsLook.adwaita ? 20
        : root.hovered || root.down ? 14 : 12) * root.k)
    readonly property real inset: (root.trackH - root.knob) / 2
    readonly property bool labelled: root.text.length > 0
    readonly property color trackFill: root.on ? Lithic.sel
        : QsLook.lithic ? Lithic.field
        : QsLook.material ? Lithic.mix(Lithic.field, Lithic.fg, 0.04)
        : QsLook.adwaita ? Lithic.alpha(Lithic.fg, QsLook.dark ? 0.16 : 0.13)
        : "transparent"
    readonly property color knobFill: QsLook.adwaita ? (QsLook.dark ? Lithic.fg : Lithic.stone4)
        : root.on ? Lithic.onSel
        : QsLook.lithic ? Lithic.fg3 : Lithic.fg2

    hoverEnabled: true
    focusPolicy: Qt.StrongFocus
    implicitHeight: root.labelled ? Math.max(Math.round(44 * root.s), copy.implicitHeight + Math.round(12 * root.s))
        : root.trackH
    implicitWidth: root.labelled ? Math.round(240 * root.s) : root.trackW
    opacity: root.enabled ? 1 : Lithic.disabledOpacity
    Accessible.role: Accessible.CheckBox
    Accessible.name: root.text
    Accessible.description: root.detail
    Accessible.checkable: true
    Accessible.checked: root.on
    onClicked: root.toggleRequested(!root.on)

    background: Item {}

    contentItem: Item {
        Symbol {
            id: icon
            visible: root.labelled && root.glyph.length > 0
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            s: root.s
            name: root.glyph
            size: 20
            ink: Lithic.fg2
        }

        Column {
            id: copy
            visible: root.labelled
            anchors.left: icon.visible ? icon.right : parent.left
            anchors.leftMargin: icon.visible ? Math.round(12 * root.s) : 0
            anchors.right: track.left
            anchors.rightMargin: Math.round(12 * root.s)
            anchors.verticalCenter: parent.verticalCenter
            spacing: Math.round(2 * root.s)

            Text {
                width: parent.width
                text: root.text
                color: Lithic.fg
                font.family: QsLook.sans
                font.pixelSize: Math.round(14 * root.s)
                font.weight: Font.Medium
                elide: Text.ElideRight
                renderType: Text.NativeRendering
            }
            Text {
                width: parent.width
                visible: root.detail.length > 0
                text: root.detail
                color: Lithic.fg2
                font.family: QsLook.sans
                font.pixelSize: Math.round(12 * root.s)
                elide: Text.ElideRight
                renderType: Text.NativeRendering
            }
        }

        Item {
            id: track
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            width: root.trackW
            height: root.trackH

            CutRect {
                anchors.fill: parent
                visible: QsLook.lithic
                design: "lithic"
                cut: Math.round(4 * root.k)
                fill: root.trackFill
                strokeColor: root.on ? "transparent" : Lithic.edge
                strokeWidth: Math.max(1, Math.round(root.s))
                Behavior on fill {
                    ColorAnimation { duration: QsLook.dur(QsLook.stateMs) }
                }
            }
            Rectangle {
                anchors.fill: parent
                visible: !QsLook.lithic
                radius: height / 2
                color: root.trackFill
                border.width: root.on || QsLook.adwaita ? 0 : Math.max(1, Math.round((QsLook.material ? 2 : 1) * root.s))
                border.color: QsLook.material ? Lithic.fg3 : Lithic.fg2
                Behavior on color {
                    ColorAnimation { duration: QsLook.dur(QsLook.stateMs) }
                }
            }

            Item {
                id: knob
                width: root.knob
                height: root.knob
                x: root.on ? track.width - width - root.inset : root.inset
                anchors.verticalCenter: parent.verticalCenter
                Behavior on x {
                    NumberAnimation {
                        duration: QsLook.dur(QsLook.material ? 300 : 160)
                        easing.type: Easing.BezierSpline
                        easing.bezierCurve: QsLook.material ? Look.springSoft : Look.outCubic
                    }
                }
                Behavior on width {
                    NumberAnimation { duration: QsLook.dur(160); easing.type: Easing.OutCubic }
                }

                CutRect {
                    anchors.fill: parent
                    visible: QsLook.lithic
                    design: "lithic"
                    cut: Math.round(2 * root.k)
                    fill: root.knobFill
                }
                Rectangle {
                    anchors.fill: parent
                    visible: !QsLook.lithic
                    radius: width / 2
                    color: root.knobFill
                    border.width: QsLook.adwaita ? Math.max(1, Math.round(root.s)) : 0
                    border.color: Lithic.alpha(Lithic.scrim, QsLook.dark ? 0.5 : 0.18)
                }
                Symbol {
                    visible: QsLook.material && root.on
                    anchors.centerIn: parent
                    s: root.k
                    name: "check"
                    size: 14
                    ink: Lithic.sel
                }
            }

            FocusFrame {
                s: root.s
                shown: root.visualFocus
                cut: QsLook.lithic ? Math.round(4 * root.k) : 0
                pill: !QsLook.lithic
            }
        }
    }

    HoverHandler {
        enabled: root.enabled
        cursorShape: Qt.PointingHandCursor
    }
}
