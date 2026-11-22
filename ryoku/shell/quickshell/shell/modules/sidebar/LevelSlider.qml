pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls as QQC
import Lithic
import Ryoku.Ui.Singletons
import shell.services
import "../ask/look"
import "../osd/designs" as Osd

Item {
    id: root

    required property real s
    required property string label
    required property string glyph
    property real value: 0
    property real from: 0
    property bool available: true
    property bool muted: false
    property bool muteEnabled: false
    property bool expandable: true
    property bool expanded: false
    readonly property int displayPercent: Math.round(Math.max(root.from, Math.min(1, root.value)) * 100)
    readonly property color fillInk: root.muted || !root.available ? Lithic.fg3 : Lithic.sel
    readonly property real rowHeight: Math.round(QsLook.levelHeight * root.s)
    signal adjusted(real value)
    signal muteRequested()
    signal expandedRequested(bool expanded)

    implicitHeight: root.rowHeight
    opacity: root.available ? 1 : Lithic.disabledOpacity

    function clamp(value): real {
        return Math.max(root.from, Math.min(1, value));
    }

    function beginEditing(): void {
        if (!root.available)
            return;
        exactInput.text = String(root.displayPercent);
        exact.editing = true;
        exactInput.forceActiveFocus();
        exactInput.selectAll();
    }

    function finishEditing(): void {
        if (!exact.editing)
            return;
        const parsed = Number(exactInput.text);
        exact.editing = false;
        if (!isNaN(parsed))
            root.adjusted(root.clamp(parsed / 100));
    }

    CornerButton {
        id: mute
        visible: !QsLook.material
        anchors.left: parent.left
        anchors.verticalCenter: QsLook.lithic ? undefined : parent.verticalCenter
        anchors.bottom: QsLook.lithic ? parent.bottom : undefined
        s: root.s
        glyph: root.glyph
        subtle: !QsLook.lithic
        checked: root.muted
        danger: root.muted
        enabled: root.available && root.muteEnabled
        Accessible.name: root.muted ? I18n.tr("Unmute %1").arg(root.label) : I18n.tr("Mute %1").arg(root.label)
        onClicked: root.muteRequested()
    }

    CornerButton {
        id: chevron
        visible: root.expandable
        anchors.right: parent.right
        anchors.verticalCenter: QsLook.lithic ? undefined : parent.verticalCenter
        anchors.bottom: QsLook.lithic ? parent.bottom : undefined
        s: root.s
        glyph: "expand_more"
        subtle: !QsLook.material
        turn: root.expanded ? 180 : 0
        checked: root.expanded
        Accessible.name: root.expanded ? I18n.tr("Collapse %1").arg(root.label) : I18n.tr("Expand %1").arg(root.label)
        onClicked: root.expandedRequested(!root.expanded)
    }

    QQC.AbstractButton {
        id: exact
        property bool editing: false
        visible: !QsLook.material
        anchors.right: chevron.visible ? chevron.left : parent.right
        anchors.rightMargin: Math.round((QsLook.lithic ? 6 : 4) * root.s)
        anchors.top: QsLook.lithic ? parent.top : undefined
        anchors.verticalCenter: QsLook.lithic ? undefined : parent.verticalCenter
        width: Math.round((QsLook.lithic ? 52 : 44) * root.s)
        height: Math.round((QsLook.lithic ? 18 : 28) * root.s)
        hoverEnabled: true
        enabled: root.available
        focusPolicy: Qt.StrongFocus
        Accessible.name: I18n.tr("Set %1 exactly").arg(root.label)
        onClicked: root.beginEditing()
        background: Item {
            Shape {
                anchors.fill: parent
                s: root.s
                cutSize: Look.cutToken
                fill: exact.editing ? Lithic.field : exact.hovered ? QsLook.hover : "transparent"
                strokeColor: exact.editing ? Lithic.edge : "transparent"
                strokeWidth: exact.editing ? Math.max(1, Math.round(root.s)) : 0
            }
        }
        contentItem: Item {
            Row {
                visible: QsLook.lithic && !exact.editing
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                spacing: Math.round(1 * root.s)

                Osd.OdometerFigure {
                    anchors.verticalCenter: parent.verticalCenter
                    value: root.available ? root.displayPercent : 0
                    size: Math.round(13 * root.s)
                    duration: QsLook.dur(260)
                    ink: root.muted ? Lithic.fg3 : Lithic.fg
                    quietInk: Lithic.alpha(Lithic.fg3, 0.55)
                }
                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: "%"
                    color: Lithic.fg3
                    font.family: QsLook.mono
                    font.pixelSize: Math.round(8 * root.s)
                    renderType: Text.NativeRendering
                }
            }
            Text {
                visible: !QsLook.lithic && !exact.editing
                anchors.fill: parent
                text: !root.available ? "—" : root.muted ? I18n.tr("Muted") : I18n.tr("%1%").arg(root.displayPercent)
                color: root.muted || !root.available ? Lithic.fg3 : Lithic.fg2
                font.family: QsLook.sans
                font.pixelSize: Math.round(12 * root.s)
                font.features: ({ "tnum": 1 })
                horizontalAlignment: Text.AlignRight
                verticalAlignment: Text.AlignVCenter
                elide: Text.ElideRight
                renderType: Text.NativeRendering
            }
            TextInput {
                id: exactInput
                anchors.fill: parent
                anchors.rightMargin: Math.round(4 * root.s)
                visible: exact.editing
                color: Lithic.fg
                selectionColor: Lithic.sel
                selectedTextColor: Lithic.onSel
                font.family: QsLook.figures
                font.pixelSize: Math.round(12 * root.s)
                horizontalAlignment: TextInput.AlignRight
                verticalAlignment: TextInput.AlignVCenter
                inputMethodHints: Qt.ImhDigitsOnly
                validator: IntValidator { bottom: Math.round(root.from * 100); top: 100 }
                onAccepted: root.finishEditing()
                onActiveFocusChanged: if (!activeFocus) root.finishEditing()
                Keys.onEscapePressed: event => {
                    exact.editing = false;
                    event.accepted = true;
                }
            }
        }
        HoverHandler {
            enabled: exact.enabled
            cursorShape: Qt.IBeamCursor
        }
    }

    Text {
        visible: QsLook.lithic
        anchors.left: mute.right
        anchors.leftMargin: Math.round(10 * root.s)
        anchors.right: exact.left
        anchors.top: parent.top
        height: exact.height
        text: root.label
        color: Lithic.fg
        font.family: QsLook.sans
        font.pixelSize: Math.round(13 * root.s)
        font.weight: Font.Medium
        verticalAlignment: Text.AlignVCenter
        elide: Text.ElideRight
        renderType: Text.NativeRendering
    }

    QQC.Slider {
        id: slider
        anchors.left: QsLook.material ? parent.left : mute.right
        anchors.leftMargin: QsLook.material ? 0 : Math.round(10 * root.s)
        anchors.right: QsLook.lithic ? chevron.visible ? chevron.left : parent.right
            : QsLook.material ? chevron.visible ? chevron.left : parent.right
            : exact.left
        anchors.rightMargin: Math.round((QsLook.material ? 8 : QsLook.lithic ? 6 : 6) * root.s)
        anchors.verticalCenter: QsLook.lithic ? undefined : parent.verticalCenter
        anchors.bottom: QsLook.lithic ? parent.bottom : undefined
        height: QsLook.material ? Math.round(QsLook.levelTrack * root.s) : Math.round((QsLook.lithic ? 22 : 28) * root.s)
        padding: 0
        from: root.from
        to: 1
        stepSize: 0.01
        snapMode: QQC.Slider.SnapAlways
        value: root.clamp(root.value)
        enabled: root.available
        focusPolicy: Qt.StrongFocus
        Accessible.name: root.label
        onMoved: root.adjusted(value)

        readonly property real pos: slider.visualPosition
        readonly property real lift: slider.pressed ? 1 : slider.hovered || slider.visualFocus ? 0.5 : 0
        property real liftShown: slider.lift
        Behavior on liftShown {
            NumberAnimation { duration: QsLook.dur(120); easing.type: Easing.OutCubic }
        }
        property real swell: 0
        onValueChanged: if (QsLook.fluent && !QsLook.still) swellRun.restart()
        SequentialAnimation {
            id: swellRun
            NumberAnimation { target: slider; property: "swell"; to: 1; duration: 90; easing.type: Easing.OutQuad }
            PauseAnimation { duration: 160 }
            NumberAnimation { target: slider; property: "swell"; to: 0; duration: 260; easing.type: Easing.OutCubic }
        }

        background: Item {
            Loader {
                anchors.fill: parent
                sourceComponent: QsLook.material ? materialTrack : QsLook.lithic ? lithicTrack : thinTrack
            }
        }
        handle: Item {
            x: slider.leftPadding + slider.pos * (slider.availableWidth - width)
            y: slider.topPadding + (slider.availableHeight - height) / 2
            width: QsLook.material ? Math.round(16 * root.s) : QsLook.lithic ? Math.round(6 * root.s) : Math.round((QsLook.adwaita ? 20 : 20) * root.s)
            height: QsLook.material ? slider.height + Math.round(8 * root.s) : QsLook.lithic ? Math.round(18 * root.s) : width

            Loader {
                anchors.fill: parent
                sourceComponent: QsLook.material ? materialHandle : QsLook.lithic ? lithicHandle
                    : QsLook.adwaita ? adwaitaHandle : fluentHandle
            }
        }

        FocusFrame {
            s: root.s
            shown: slider.visualFocus
            cut: QsLook.lithic ? 0 : Look.corner(Look.cutRow, slider.width / root.s, slider.height / root.s) * root.s
            pill: QsLook.material
        }
        HoverHandler {
            enabled: slider.enabled
            cursorShape: Qt.PointingHandCursor
        }
        WheelHandler {
            enabled: root.available
            onWheel: event => {
                const direction = event.angleDelta.y >= 0 ? 1 : -1;
                root.adjusted(root.clamp(root.value + direction * 0.02));
                event.accepted = true;
            }
        }
    }

    QQC.AbstractButton {
        id: inner
        visible: QsLook.material
        anchors.left: slider.left
        anchors.verticalCenter: slider.verticalCenter
        width: slider.height
        height: slider.height
        enabled: root.available && root.muteEnabled
        hoverEnabled: true
        focusPolicy: Qt.StrongFocus
        Accessible.name: mute.Accessible.name
        onClicked: root.muteRequested()
        background: Item {}
        contentItem: Symbol {
            s: root.s
            name: root.glyph
            size: 22
            chosen: !root.muted
            ink: slider.pos * slider.width > inner.width * 0.62 && !root.muted ? Lithic.onSel : Lithic.fg2
        }
        HoverHandler {
            enabled: inner.enabled
            cursorShape: Qt.PointingHandCursor
        }
    }
    Text {
        id: materialValue
        visible: QsLook.material
        anchors.left: inner.right
        anchors.verticalCenter: slider.verticalCenter
        text: !root.available ? "—" : root.muted ? I18n.tr("Muted") : I18n.tr("%1%").arg(root.displayPercent)
        color: !root.muted && slider.pos * (slider.width - Math.round(16 * root.s)) + Math.round(2 * root.s)
            > materialValue.x + materialValue.width - slider.x ? Lithic.onSel : Lithic.fg2
        font.family: QsLook.sans
        font.pixelSize: Math.round(13 * root.s)
        font.weight: Font.Medium
        font.features: ({ "tnum": 1 })
        renderType: Text.NativeRendering
    }

    Component {
        id: lithicTrack

        Item {
            id: ruler
            readonly property real step: Math.max(4, Math.round(6 * root.s))
            readonly property int count: Math.max(2, Math.floor(width / ruler.step) + 1)

            Rectangle {
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.bottom: parent.bottom
                height: Math.max(1, Math.round(root.s))
                color: Lithic.vein
            }
            Repeater {
                model: ruler.count
                delegate: Rectangle {
                    required property int index
                    readonly property bool lit: index * ruler.step <= slider.pos * ruler.width + 0.5
                    x: Math.round(index * ruler.step)
                    anchors.bottom: parent.bottom
                    width: Math.max(1, Math.round(root.s))
                    height: Math.round((index % 5 === 0 ? 11 : 6) * root.s)
                    color: lit ? (root.muted ? Lithic.fg3 : Lithic.fg) : Lithic.alpha(Lithic.fg3, 0.5)
                    Behavior on color {
                        ColorAnimation { duration: QsLook.dur(90) }
                    }
                }
            }
        }
    }
    Component {
        id: lithicHandle

        CutRect {
            design: "lithic"
            cut: Math.round(2 * root.s)
            fill: root.fillInk
            scale: 1 + slider.liftShown * 0.12
            transformOrigin: Item.Bottom
        }
    }

    Component {
        id: materialTrack

        Item {
            readonly property real gap: Math.round(6 * root.s)
            readonly property real at: slider.pos * (width - Math.round(16 * root.s)) + Math.round(8 * root.s)

            CutRect {
                x: 0
                width: Math.max(height, parent.at - parent.gap)
                height: parent.height
                cut: 12 * root.s
                fill: root.fillInk
                Behavior on fill {
                    ColorAnimation { duration: QsLook.dur(QsLook.stateMs) }
                }
            }
            CutRect {
                x: parent.at + parent.gap
                width: Math.max(0, parent.width - x)
                visible: width > Math.round(2 * root.s)
                height: parent.height
                cut: 12 * root.s
                fill: Lithic.mix(Lithic.field, Lithic.fg, 0.03)
            }
        }
    }
    Component {
        id: materialHandle

        Item {
            Rectangle {
                anchors.centerIn: parent
                width: Math.round((4 - slider.liftShown * 1.5) * root.s)
                height: parent.height
                radius: width / 2
                color: root.fillInk
            }
        }
    }

    Component {
        id: thinTrack

        Item {
            Rectangle {
                anchors.verticalCenter: parent.verticalCenter
                width: parent.width
                height: Math.max(2, Math.round(QsLook.levelTrack * root.s))
                radius: height / 2
                color: QsLook.adwaita ? Lithic.alpha(Lithic.fg, QsLook.dark ? 0.16 : 0.13)
                    : Lithic.alpha(Lithic.fg, QsLook.dark ? 0.45 : 0.4)
            }
            Rectangle {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.max(height, slider.pos * parent.width)
                height: Math.max(2, Math.round(QsLook.levelTrack * root.s))
                radius: height / 2
                color: root.fillInk
            }
        }
    }
    Component {
        id: adwaitaHandle

        Rectangle {
            radius: width / 2
            color: QsLook.dark ? Lithic.fg : Lithic.stone4
            border.width: Math.max(1, Math.round(root.s))
            border.color: Lithic.alpha(Lithic.scrim, QsLook.dark ? 0.5 : 0.2)
            scale: 1 + slider.liftShown * 0.1
        }
    }
    Component {
        id: fluentHandle

        Rectangle {
            radius: width / 2
            color: Lithic.stone4
            border.width: Math.max(1, Math.round(root.s))
            border.color: Lithic.vein

            Rectangle {
                anchors.centerIn: parent
                width: Math.round((10 + 3 * slider.liftShown * (slider.pressed ? -0.6 : 1) + 2 * slider.swell) * root.s)
                height: width
                radius: width / 2
                color: root.fillInk
            }
        }
    }
}
