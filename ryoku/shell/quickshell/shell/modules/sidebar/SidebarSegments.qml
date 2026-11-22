pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls as QQC
import Lithic
import Ryoku.Ui.Singletons
import shell.services
import "../ask/look"

Item {
    id: root

    property real s: 1
    property var options: []
    property var labels: ({})
    property var glyphs: ({})
    property string current: ""
    property string accessibleName: ""
    signal chose(string key)

    readonly property int count: root.options.length
    readonly property int index: root.options.indexOf(root.current)
    readonly property real h: Math.round(QsLook.segmentHeight * root.s)
    readonly property real gap: QsLook.material ? Math.round(2 * root.s) : QsLook.adwaita ? Math.max(1, Math.round(root.s)) : 0
    readonly property real inset: QsLook.lithic || QsLook.fluent ? Math.round(3 * root.s) : 0
    readonly property real cell: root.count > 0
        ? (root.width - root.inset * 2 - root.gap * (root.count - 1)) / root.count : 0
    readonly property bool glided: QsLook.lithic || QsLook.fluent

    implicitHeight: root.h
    implicitWidth: Math.round(320 * root.s)
    Accessible.role: Accessible.List
    Accessible.name: root.accessibleName

    function label(key) {
        return I18n.tr(root.labels[key] !== undefined ? root.labels[key] : key);
    }
    function step(delta) {
        const next = Math.max(0, Math.min(root.count - 1, root.index + delta));
        if (next !== root.index && next >= 0)
            root.chose(root.options[next]);
    }

    CutRect {
        visible: root.glided
        anchors.fill: parent
        cut: (QsLook.lithic ? LithicTokens.cut.mid : LithicTokens.cut.small) * root.s
        fill: QsLook.lithic ? Lithic.field : Lithic.alpha(Lithic.fg, QsLook.dark ? 0.05 : 0.04)
        strokeColor: QsLook.lithic ? Lithic.vein : Lithic.alpha(Lithic.fg, QsLook.dark ? 0.09 : 0.12)
        strokeWidth: Math.max(1, Math.round(root.s))
    }
    CutRect {
        id: plate
        visible: root.glided && root.index >= 0
        x: root.inset + root.index * (root.cell + root.gap)
        y: root.inset
        width: root.cell
        height: root.h - root.inset * 2
        cut: (QsLook.lithic ? LithicTokens.cut.segment : 4) * root.s
        fill: QsLook.lithic ? Lithic.sel : Lithic.mix(Lithic.stone4, Lithic.fg, QsLook.dark ? 0.06 : 0)
        strokeColor: QsLook.fluent ? Lithic.alpha(Lithic.fg, QsLook.dark ? 0.08 : 0.1) : "transparent"
        strokeWidth: Math.max(1, Math.round(root.s))
        Behavior on x {
            NumberAnimation { duration: QsLook.dur(LithicTokens.motion.select); easing.type: Easing.BezierSpline; easing.bezierCurve: QsLook.fluent ? Look.fluentIn : Lithic.curve }
        }

        Rectangle {
            visible: QsLook.fluent
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.bottom: parent.bottom
            anchors.bottomMargin: Math.round(3 * root.s)
            width: Math.round(16 * root.s)
            height: Math.max(2, Math.round(3 * root.s))
            radius: height / 2
            color: Lithic.sel
        }
    }

    Row {
        x: root.inset
        y: root.inset
        spacing: root.gap

        Repeater {
            model: root.options

            delegate: QQC.AbstractButton {
                id: segment

                required property string modelData
                required property int index
                readonly property bool chosen: segment.modelData === root.current
                readonly property bool first: segment.index === 0
                readonly property bool last: segment.index === root.count - 1
                readonly property color ink: segment.chosen && !QsLook.fluent ? Lithic.onSel : segment.chosen ? Lithic.fg : Lithic.fg2
                readonly property real outer: height / 2
                readonly property real inner: QsLook.material
                    ? (segment.chosen ? height / 2 : Math.round(8 * root.s))
                    : 0

                width: root.cell
                height: root.h - root.inset * 2
                hoverEnabled: true
                focusPolicy: segment.chosen ? Qt.StrongFocus : Qt.NoFocus
                Accessible.role: Accessible.RadioButton
                Accessible.name: root.label(segment.modelData)
                Accessible.checkable: true
                Accessible.checked: segment.chosen
                onClicked: root.chose(segment.modelData)
                Keys.onLeftPressed: root.step(-1)
                Keys.onRightPressed: root.step(1)

                background: Item {
                    Rectangle {
                        visible: QsLook.material
                        anchors.fill: parent
                        topLeftRadius: segment.first ? segment.outer : segment.innerShown
                        bottomLeftRadius: segment.first ? segment.outer : segment.innerShown
                        topRightRadius: segment.last ? segment.outer : segment.innerShown
                        bottomRightRadius: segment.last ? segment.outer : segment.innerShown
                        color: Lithic.mix(segment.chosen ? Lithic.sel : Lithic.field, segment.ink,
                            segment.down ? 0.12 : segment.hovered ? 0.07 : 0)
                        Behavior on color {
                            ColorAnimation { duration: QsLook.dur(QsLook.stateMs * 2) }
                        }
                    }
                    Rectangle {
                        visible: QsLook.adwaita
                        anchors.fill: parent
                        readonly property real r: Look.corner(LithicTokens.cut.small, root.cell / root.s, height / root.s) * root.s
                        topLeftRadius: segment.first ? r : 0
                        bottomLeftRadius: segment.first ? r : 0
                        topRightRadius: segment.last ? r : 0
                        bottomRightRadius: segment.last ? r : 0
                        color: Lithic.mix(segment.chosen ? Lithic.sel : Lithic.alpha(Lithic.fg, QsLook.dark ? 0.1 : 0.08),
                            segment.ink, segment.down ? 0.12 : segment.hovered ? 0.07 : 0)
                        Behavior on color {
                            ColorAnimation { duration: QsLook.dur(QsLook.stateMs) }
                        }
                    }
                    CutRect {
                        visible: root.glided && !segment.chosen
                        anchors.fill: parent
                        cut: plate.cut
                        fill: segment.down ? QsLook.press : segment.hovered ? QsLook.hover : "transparent"
                        Behavior on fill {
                            ColorAnimation { duration: QsLook.dur(LithicTokens.motion.hoverIn) }
                        }
                    }
                }
                property real innerShown: segment.inner
                Behavior on innerShown {
                    NumberAnimation { duration: QsLook.dur(360); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.springSoft }
                }

                contentItem: Item {
                    Row {
                        anchors.centerIn: parent
                        anchors.verticalCenterOffset: QsLook.fluent ? -Math.round(2 * root.s) : 0
                        spacing: Math.round(6 * root.s)

                        Symbol {
                            readonly property string name0: QsLook.material && segment.chosen ? "check"
                                : root.glyphs[segment.modelData] !== undefined ? root.glyphs[segment.modelData] : ""
                            visible: name0.length > 0 && root.cell > Math.round(96 * root.s)
                            anchors.verticalCenter: parent.verticalCenter
                            s: root.s
                            name: name0
                            size: 16
                            ink: segment.ink
                            chosen: segment.chosen
                        }
                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            text: root.label(segment.modelData)
                            color: segment.ink
                            font.family: QsLook.sans
                            font.pixelSize: Math.round(13 * root.s)
                            font.weight: segment.chosen ? Font.DemiBold : Font.Medium
                            elide: Text.ElideRight
                            renderType: Text.NativeRendering
                            Behavior on color {
                                ColorAnimation { duration: QsLook.dur(QsLook.stateMs) }
                            }
                        }
                    }
                }

                FocusFrame {
                    s: root.s
                    shown: segment.visualFocus
                    cut: plate.cut
                    pill: QsLook.material
                }
                HoverHandler {
                    cursorShape: Qt.PointingHandCursor
                }
            }
        }
    }
}
