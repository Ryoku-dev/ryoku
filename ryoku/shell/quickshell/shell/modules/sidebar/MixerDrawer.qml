pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls as QQC
import Lithic
import Ryoku.Ui.Singletons
import shell.services
import "../ask/look"

Item {
    id: root

    required property real s
    required property bool active
    readonly property real groupGap: Math.round((QsLook.lithic ? 12 : QsLook.material ? 16 : 14) * root.s)
    readonly property real labelGap: Math.round((QsLook.lithic ? 6 : 8) * root.s)
    readonly property real rowGap: Math.round((QsLook.material ? 4 : QsLook.adwaita ? 0 : 2) * root.s)
    readonly property real travel: Math.round(6 * root.s)
    readonly property int listMs: QsLook.dur(180)

    implicitHeight: body.implicitHeight

    component DeviceChoice: QQC.AbstractButton {
        id: choice

        property real s: 1
        property var node: null
        property bool selected: false
        readonly property string deviceLabel: Audio.nodeLabel(choice.node)
        readonly property bool pill: QsLook.material
        readonly property real cutPx: Look.cutRow * choice.s + (QsLook.lithic && choice.down ? Lithic.pressCutGrow * choice.s : 0)
        readonly property color rest: !choice.selected ? "transparent"
            : QsLook.lithic || QsLook.material ? Lithic.container
            : QsLook.fluent ? Look.plate : "transparent"
        readonly property color plane: choice.down ? Lithic.mix(Qt.colorEqual(choice.rest, "transparent") ? QsLook.press : choice.rest, Lithic.fg, Qt.colorEqual(choice.rest, "transparent") ? 0 : 0.1)
            : choice.hovered ? (Qt.colorEqual(choice.rest, "transparent") ? QsLook.hover : Lithic.mix(choice.rest, Lithic.fg, 0.06))
            : choice.rest
        readonly property color ink: choice.selected && QsLook.material ? Lithic.containerInk : Lithic.fg
        readonly property color glyphInk: !choice.selected ? Lithic.fg2
            : QsLook.lithic ? Lithic.sel
            : QsLook.material ? Lithic.containerInk : Lithic.fg

        hoverEnabled: true
        focusPolicy: Qt.StrongFocus
        padding: 0
        implicitHeight: Math.round((QsLook.material ? 44 : QsLook.adwaita ? 40 : QsLook.fluent ? 36 : 34) * choice.s)
        Accessible.role: Accessible.RadioButton
        Accessible.checkable: true
        Accessible.checked: choice.selected
        scale: QsLook.fluent && choice.down ? 0.985 : 1
        Behavior on scale {
            NumberAnimation { duration: QsLook.dur(LithicTokens.motion.press); easing.type: Easing.OutCubic }
        }

        background: Item {
            CutRect {
                visible: !choice.pill
                anchors.fill: parent
                cut: choice.cutPx
                fill: choice.plane
                strokeColor: QsLook.lithic && choice.selected ? Lithic.alpha(Lithic.sel, 0.6) : "transparent"
                strokeWidth: Math.max(1, Math.round(choice.s))
                Behavior on fill {
                    ColorAnimation { duration: QsLook.dur(choice.hovered ? LithicTokens.motion.hoverIn : QsLook.stateMs) }
                }
                Behavior on cut {
                    NumberAnimation { duration: QsLook.dur(LithicTokens.motion.press); easing.type: Easing.OutCubic }
                }
            }
            Rectangle {
                visible: choice.pill
                anchors.fill: parent
                radius: height / 2
                color: choice.plane
                Behavior on color {
                    ColorAnimation { duration: QsLook.dur(QsLook.stateMs) }
                }
            }
            Notch {
                visible: QsLook.lithic
                size: Math.round(7 * choice.s)
                fill: Lithic.sel
                corner: "topLeft"
                scale: choice.selected ? 1 : 0
                transformOrigin: Item.TopLeft
                Behavior on scale {
                    NumberAnimation { duration: QsLook.dur(220); easing.type: Easing.OutBack; easing.overshoot: 1.6 }
                }
            }
        }

        contentItem: Item {
            transform: Translate { y: QsLook.lithic && choice.down ? Lithic.pressSink * choice.s : 0 }

            Symbol {
                id: deviceGlyph
                x: Math.round((QsLook.material ? 14 : 10) * choice.s)
                anchors.verticalCenter: parent.verticalCenter
                s: choice.s
                name: Audio.nodeIcon(choice.node)
                size: QsLook.material ? 20 : 18
                ink: choice.glyphInk
                chosen: choice.selected
                Accessible.ignored: true
                Behavior on ink {
                    ColorAnimation { duration: QsLook.dur(QsLook.stateMs) }
                }
            }
            Text {
                anchors.left: deviceGlyph.right
                anchors.leftMargin: Math.round((QsLook.material ? 12 : 10) * choice.s)
                anchors.right: check.left
                anchors.rightMargin: Math.round(8 * choice.s)
                anchors.verticalCenter: parent.verticalCenter
                text: choice.deviceLabel
                color: choice.ink
                font.family: QsLook.sans
                font.pixelSize: Math.round((QsLook.material ? 14 : 13) * choice.s)
                font.weight: choice.selected && !QsLook.lithic ? Font.DemiBold : Font.Medium
                elide: Text.ElideRight
                renderType: Text.NativeRendering
                Behavior on color {
                    ColorAnimation { duration: QsLook.dur(QsLook.stateMs) }
                }
            }
            Symbol {
                id: check
                anchors.right: parent.right
                anchors.rightMargin: Math.round((QsLook.material ? 14 : 10) * choice.s)
                anchors.verticalCenter: parent.verticalCenter
                width: QsLook.lithic ? 0 : implicitWidth
                visible: !QsLook.lithic
                s: choice.s
                name: "check"
                size: 18
                ink: QsLook.material ? Lithic.containerInk : Lithic.sel
                opacity: choice.selected ? 1 : 0
                scale: choice.selected ? 1 : 0.6
                Accessible.ignored: true
                Behavior on opacity {
                    NumberAnimation { duration: QsLook.dur(QsLook.stateMs) }
                }
                Behavior on scale {
                    NumberAnimation {
                        duration: QsLook.dur(QsLook.material ? 280 : 180)
                        easing.type: QsLook.material ? Easing.OutBack : Easing.OutCubic
                        easing.overshoot: 1.8
                    }
                }
            }
        }

        FocusFrame {
            s: choice.s
            shown: choice.visualFocus
            cut: choice.cutPx
            pill: choice.pill
        }
        HoverHandler {
            enabled: choice.enabled
            cursorShape: Qt.PointingHandCursor
        }
    }

    component Quiet: Text {
        id: quiet
        property real s: 1
        color: Lithic.fg3
        font.family: QsLook.sans
        font.pixelSize: Math.round(12 * quiet.s)
        elide: Text.ElideRight
        renderType: Text.NativeRendering
    }

    component Streams: ListView {
        id: list

        property real s: 1
        property int ms: 0
        property real travel: 0
        property bool capture: false

        height: contentHeight
        interactive: false
        clip: false
        spacing: Math.round((QsLook.material ? 6 : 4) * list.s)
        delegate: MixerRow {
            required property var modelData
            width: list.width
            s: list.s
            node: modelData
            capture: list.capture
        }
        add: Transition {
            ParallelAnimation {
                NumberAnimation { property: "opacity"; from: 0; to: 1; duration: list.ms; easing.type: Easing.OutCubic }
                NumberAnimation { property: "y"; from: list.travel; duration: list.ms; easing.type: Easing.OutCubic }
            }
        }
        remove: Transition {
            ParallelAnimation {
                NumberAnimation { property: "opacity"; to: 0; duration: list.ms; easing.type: Easing.OutCubic }
                NumberAnimation { property: "y"; to: -list.travel; duration: list.ms; easing.type: Easing.OutCubic }
            }
        }
        displaced: Transition {
            NumberAnimation { properties: "y"; duration: list.ms; easing.type: Easing.OutCubic }
        }
    }

    Column {
        id: body
        width: parent.width
        spacing: root.groupGap

        Column {
            width: parent.width
            spacing: root.labelGap

            Micro {
                width: parent.width
                s: root.s
                text: I18n.tr("Output")
            }
            Column {
                width: parent.width
                spacing: root.rowGap

                Repeater {
                    model: root.active ? Audio.outputs.filter(Audio.alive) : []
                    delegate: DeviceChoice {
                        required property var modelData
                        width: parent.width
                        s: root.s
                        node: modelData
                        selected: !!(Audio.sink && modelData && Audio.sink.name === modelData.name)
                        Accessible.name: selected ? I18n.tr("%1 is the default output").arg(deviceLabel)
                            : I18n.tr("Use %1 as the default output").arg(deviceLabel)
                        onClicked: if (!selected) Audio.setOutput(modelData)
                    }
                }
                Quiet {
                    width: parent.width
                    visible: root.active && Audio.outputs.length === 0
                    s: root.s
                    text: I18n.tr("No audio output")
                }
            }
        }

        Column {
            width: parent.width
            spacing: root.labelGap

            Micro {
                width: parent.width
                s: root.s
                text: I18n.tr("Playing")
            }
            Streams {
                width: parent.width
                s: root.s
                ms: root.listMs
                travel: root.travel
                model: root.active ? Audio.streams.filter(Audio.alive) : []
            }
            Quiet {
                width: parent.width
                visible: root.active && Audio.streams.length === 0
                s: root.s
                text: I18n.tr("No audio playing")
            }
        }

        Column {
            width: parent.width
            spacing: root.labelGap
            visible: root.active && Audio.captureStreams.length > 0

            Micro {
                width: parent.width
                s: root.s
                text: I18n.tr("Recording")
            }
            Streams {
                width: parent.width
                s: root.s
                ms: root.listMs
                travel: root.travel
                capture: true
                model: root.active ? Audio.captureStreams.filter(Audio.alive) : []
            }
        }

        Column {
            width: parent.width
            spacing: root.labelGap
            visible: Audio.inputs.length > 0 || !!(Audio.source && Audio.source.audio)

            Micro {
                width: parent.width
                s: root.s
                text: I18n.tr("Microphone")
            }
            Column {
                width: parent.width
                spacing: root.rowGap

                Repeater {
                    model: root.active ? Audio.inputs.filter(Audio.alive) : []
                    delegate: DeviceChoice {
                        required property var modelData
                        width: parent.width
                        s: root.s
                        node: modelData
                        selected: !!(Audio.source && modelData && Audio.source.name === modelData.name)
                        Accessible.name: selected ? I18n.tr("%1 is the default microphone").arg(deviceLabel)
                            : I18n.tr("Use %1 as the default microphone").arg(deviceLabel)
                        onClicked: if (!selected) Audio.setInput(modelData)
                    }
                }
            }
            MixerRow {
                width: parent.width
                visible: !!(Audio.source && Audio.source.audio)
                s: root.s
                node: Audio.source
                device: true
                capture: true
            }
        }
    }
}
