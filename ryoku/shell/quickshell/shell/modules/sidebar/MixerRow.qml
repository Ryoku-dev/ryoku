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
    required property var node
    property bool capture: false
    property bool device: false
    readonly property var audio: node ? node.audio : null
    readonly property bool muted: !!(audio && audio.muted)
    readonly property real level: audio ? Math.max(0, Math.min(1, audio.volume)) : 0
    readonly property string title: device ? Audio.nodeLabel(node) : Audio.streamName(node)
    readonly property string iconSource: device ? "" : Audio.streamIcon(node)
    readonly property string initial: title.length > 0 ? title.charAt(0).toUpperCase() : "•"
    readonly property color fillInk: root.muted ? Lithic.fg3 : QsLook.lithic ? Lithic.fg2 : Lithic.sel
    readonly property color knobInk: root.muted ? Lithic.fg3 : Lithic.sel
    readonly property bool roundPlate: QsLook.material || QsLook.adwaita

    implicitHeight: Math.round((QsLook.material ? 44 : QsLook.lithic ? 32 : 36) * s)
    height: implicitHeight
    enabled: audio !== null
    opacity: enabled ? 1 : Lithic.disabledOpacity

    function setLevel(value): void {
        if (root.audio)
            root.audio.volume = Math.max(0, Math.min(1, value));
    }

    Item {
        id: plate
        anchors.left: parent.left
        anchors.verticalCenter: parent.verticalCenter
        width: Math.round((QsLook.material ? 30 : 26) * root.s)
        height: width

        CutRect {
            visible: !root.roundPlate
            anchors.fill: parent
            cut: (QsLook.lithic ? Look.cutToken : Look.cutRow) * root.s
            fill: QsLook.lithic ? "transparent" : Look.plate
            strokeColor: QsLook.lithic ? Lithic.vein : Look.plateStroke
            strokeWidth: Math.max(1, Math.round(root.s))
        }
        Rectangle {
            visible: root.roundPlate
            anchors.fill: parent
            radius: width / 2
            color: QsLook.material ? Lithic.container : Lithic.alpha(Lithic.fg, QsLook.dark ? 0.1 : 0.08)
        }
        Image {
            id: appIcon
            anchors.centerIn: parent
            width: Math.round((QsLook.material ? 18 : 16) * root.s)
            height: width
            source: root.iconSource
            sourceSize.width: width * 2
            sourceSize.height: height * 2
            fillMode: Image.PreserveAspectFit
            smooth: true
            asynchronous: true
            visible: !root.device && status === Image.Ready
        }
        Symbol {
            anchors.centerIn: parent
            visible: root.device
            s: root.s
            name: Audio.nodeIcon(root.node)
            size: 16
            ink: QsLook.material ? Lithic.containerInk : Lithic.fg2
            Accessible.ignored: true
        }
        Text {
            anchors.centerIn: parent
            visible: !root.device && !appIcon.visible
            text: root.initial
            color: QsLook.material ? Lithic.containerInk : Lithic.fg2
            font.family: QsLook.lithic ? QsLook.mono : QsLook.sans
            font.pixelSize: Math.round(12 * root.s)
            font.weight: Font.DemiBold
            renderType: Text.NativeRendering
            Accessible.ignored: true
        }
    }

    Text {
        id: nameLabel
        anchors.left: plate.right
        anchors.leftMargin: Math.round(10 * root.s)
        anchors.verticalCenter: parent.verticalCenter
        width: Math.max(0, (mute.x - plate.width - Math.round(10 * root.s)) * 0.36)
        text: root.title
        color: Lithic.fg
        font.family: QsLook.sans
        font.pixelSize: Math.round(13 * root.s)
        font.weight: Font.Medium
        elide: Text.ElideRight
        renderType: Text.NativeRendering
    }

    QQC.Slider {
        id: slider
        anchors.left: nameLabel.right
        anchors.leftMargin: Math.round(10 * root.s)
        anchors.right: percent.left
        anchors.rightMargin: Math.round(8 * root.s)
        anchors.verticalCenter: parent.verticalCenter
        height: Math.round((QsLook.material ? 24 : QsLook.lithic ? 16 : 20) * root.s)
        padding: 0
        from: 0
        to: 1
        stepSize: 0.01
        snapMode: QQC.Slider.SnapAlways
        value: root.level
        focusPolicy: Qt.StrongFocus
        Accessible.name: I18n.tr("%1 volume").arg(root.title)
        onMoved: root.setLevel(value)

        // glide to levels set elsewhere but follow the hand exactly
        property real shown: slider.visualPosition
        Behavior on shown {
            enabled: !slider.pressed
            NumberAnimation { duration: QsLook.dur(140); easing.type: Easing.OutCubic }
        }
        readonly property real lift: slider.pressed ? 1 : slider.hovered || slider.visualFocus ? 0.5 : 0
        property real liftShown: slider.lift
        Behavior on liftShown {
            NumberAnimation { duration: QsLook.dur(120); easing.type: Easing.OutCubic }
        }
        background: Item {
            Loader {
                anchors.fill: parent
                sourceComponent: QsLook.lithic ? bladeTrack : QsLook.material ? pillTrack : thinTrack
            }
        }
        handle: Item {
            x: slider.leftPadding + slider.shown * (slider.availableWidth - width)
            y: slider.topPadding + (slider.availableHeight - height) / 2
            width: Math.round((QsLook.lithic ? 6 : QsLook.material ? 4 : 16) * root.s)
            height: QsLook.lithic ? Math.round(14 * root.s) : QsLook.material ? slider.height : width

            Loader {
                anchors.fill: parent
                sourceComponent: QsLook.lithic ? cutHandle : QsLook.material ? barHandle
                    : QsLook.adwaita ? knobHandle : ringHandle
            }
        }

        FocusFrame {
            s: root.s
            shown: slider.visualFocus
            pill: !QsLook.lithic
        }
        HoverHandler {
            enabled: slider.enabled
            cursorShape: Qt.PointingHandCursor
        }
        WheelHandler {
            onWheel: event => {
                const direction = event.angleDelta.y >= 0 ? 1 : -1;
                root.setLevel(slider.value + direction * 0.02);
                event.accepted = true;
            }
        }
    }

    Text {
        id: percent
        anchors.right: mute.left
        anchors.rightMargin: Math.round(4 * root.s)
        anchors.verticalCenter: parent.verticalCenter
        width: Math.round(36 * root.s)
        text: {
            const value = Math.round(root.level * 100);
            return I18n.tr("%1%").arg(QsLook.lithic ? String(value).padStart(3, "0") : value);
        }
        color: root.muted ? Lithic.fg3 : Lithic.fg2
        font.family: QsLook.figures
        font.pixelSize: Math.round((QsLook.lithic ? 11 : 12) * root.s)
        font.features: ({ "tnum": 1 })
        horizontalAlignment: Text.AlignRight
        renderType: Text.NativeRendering
    }

    CornerButton {
        id: mute
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        s: root.s
        glyph: root.capture ? (root.muted ? "mic_off" : "mic") : (root.muted ? "volume_off" : "volume_up")
        subtle: true
        checked: root.muted
        danger: root.muted
        Accessible.name: root.muted ? I18n.tr("Unmute %1").arg(root.title) : I18n.tr("Mute %1").arg(root.title)
        onClicked: if (root.audio) root.audio.muted = !root.audio.muted
    }

    Component {
        id: bladeTrack

        Item {
            Blade {
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                height: Math.max(2, Math.round(4 * root.s))
                fraction: slider.shown
                fill: root.fillInk
                track: Lithic.field
            }
        }
    }
    Component {
        id: cutHandle

        CutRect {
            design: "lithic"
            cut: Math.round(2 * root.s)
            fill: root.knobInk
            scale: 1 + slider.liftShown * 0.15
        }
    }

    Component {
        id: pillTrack

        Item {
            readonly property real track: Math.round(16 * root.s)
            readonly property real gap: Math.round(4 * root.s)
            readonly property real handle: Math.round(4 * root.s)
            readonly property real at: slider.shown * (width - handle) + handle / 2

            Rectangle {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.max(height, parent.at - parent.gap)
                height: parent.track
                radius: height / 2
                color: root.fillInk
                Behavior on color {
                    ColorAnimation { duration: QsLook.dur(QsLook.stateMs) }
                }
            }
            Rectangle {
                anchors.verticalCenter: parent.verticalCenter
                x: Math.min(parent.width - height, parent.at + parent.gap)
                width: Math.max(height, parent.width - x)
                height: parent.track
                radius: height / 2
                color: Lithic.mix(Lithic.field, Lithic.fg, 0.08)
            }
        }
    }
    Component {
        id: barHandle

        Item {
            Rectangle {
                anchors.centerIn: parent
                width: Math.round((4 - slider.liftShown * 1.5) * root.s)
                height: parent.height
                radius: width / 2
                color: root.knobInk
            }
        }
    }

    Component {
        id: thinTrack

        Item {
            Rectangle {
                anchors.verticalCenter: parent.verticalCenter
                width: parent.width
                height: Math.max(2, Math.round(4 * root.s))
                radius: height / 2
                color: QsLook.adwaita ? Lithic.alpha(Lithic.fg, QsLook.dark ? 0.16 : 0.13)
                    : Lithic.alpha(Lithic.fg, QsLook.dark ? 0.45 : 0.4)
            }
            Rectangle {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.max(height, slider.shown * parent.width)
                height: Math.max(2, Math.round(4 * root.s))
                radius: height / 2
                color: root.fillInk
            }
        }
    }
    Component {
        id: knobHandle

        Rectangle {
            radius: width / 2
            color: QsLook.dark ? Lithic.fg : Lithic.stone4
            border.width: Math.max(1, Math.round(root.s))
            border.color: Lithic.alpha(Lithic.scrim, QsLook.dark ? 0.5 : 0.2)
            scale: 1 + slider.liftShown * 0.12
        }
    }
    Component {
        id: ringHandle

        Rectangle {
            radius: width / 2
            color: Lithic.stone4
            border.width: Math.max(1, Math.round(root.s))
            border.color: Lithic.vein

            Rectangle {
                anchors.centerIn: parent
                width: Math.round((8 + 3 * slider.liftShown * (slider.pressed ? -0.5 : 1)) * root.s)
                height: width
                radius: width / 2
                color: root.knobInk
            }
        }
    }
}
