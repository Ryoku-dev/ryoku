pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Lithic
import "../../../components"

// Adwaita's pop-up, after GNOME's on-screen display since 42: a full-round pill at
// the bottom centre, 56 above the edge, holding a symbolic icon, a thin level bar
// and a small dim value at its end. It fades in while it grows from 94%, and
// fades out. The icon nods when it changes. The keyboard layout shows the
// icon and the layout's name.
Item {
    id: root

    property string kind: "volume"
    property real u: 1
    property bool shown: false
    property real value: 0
    property real maxValue: 1
    property bool muted: false
    property string iconName: ""
    property string label: ""

    readonly property string placement: "bottom"
    readonly property real gap: Math.max(0, 56 * root.u - root.pad)
    readonly property real reveal: root.appear

    readonly property bool keyboard: root.kind === "keyboard"
    readonly property bool over: root.value > 1.005
    readonly property real pad: 16 * root.u
    readonly property real pillWidth: root.keyboard
        ? Math.round(Math.max(160 * root.u, Math.min(320 * root.u,
            layoutName.implicitWidth + 64 * root.u)))
        : Math.round(260 * root.u)
    readonly property real pillHeight: Math.round(44 * root.u)
    readonly property color levelInk: root.muted ? Lithic.fg3 : Lithic.sel

    implicitWidth: Math.ceil(root.pillWidth + root.pad * 2)
    implicitHeight: Math.ceil(root.pillHeight + root.pad * 2)

    property real appear: 0
    Behavior on appear {
        id: appearMotion
        NumberAnimation {
            duration: Lithic.duration(appearMotion.targetValue > 0.5 ? 180 : 150)
            easing.type: Easing.OutQuad
        }
    }
    Component.onCompleted: root.appear = Qt.binding(() => root.shown ? 1 : 0)

    Item {
        id: pill

        anchors.centerIn: parent
        width: root.pillWidth
        height: root.pillHeight
        opacity: root.appear
        scale: 0.94 + 0.06 * root.appear

        RectangularShadow {
            anchors.fill: parent
            offset.y: 4 * root.u
            blur: 14 * root.u
            radius: plate.radius
            color: Lithic.alpha(Lithic.scrim, Lithic.dark ? 0.7 : 0.3)
        }

        Rectangle {
            id: plate
            anchors.fill: parent
            radius: height / 2
            color: Lithic.planes("adwaita").card
        }

        SymbolIcon {
            id: icon
            x: Math.round(14 * root.u)
            anchors.verticalCenter: parent.verticalCenter
            name: root.iconName
            size: Math.round(20 * root.u)
            color: root.muted ? Lithic.fg3 : Lithic.fg
        }

        Item {
            id: bar
            visible: !root.keyboard
            anchors.left: icon.right
            anchors.leftMargin: 12 * root.u
            anchors.right: figure.left
            anchors.rightMargin: 10 * root.u
            anchors.verticalCenter: parent.verticalCenter
            height: Math.round(6 * root.u)

            readonly property real unityX: bar.width / root.maxValue
            property real level: 0
            Behavior on level {
                NumberAnimation {
                    duration: Lithic.duration(200)
                    easing.type: Easing.OutCubic
                }
            }
            Component.onCompleted: bar.level = Qt.binding(() => Math.max(0, Math.min(root.maxValue, root.value)))

            Rectangle {
                anchors.fill: parent
                radius: height / 2
                color: Lithic.alpha(Lithic.fg, 0.14)
            }
            Rectangle {
                width: Math.max(bar.level > 0 ? bar.height : 0, bar.width * bar.level / root.maxValue)
                height: bar.height
                radius: height / 2
                color: bar.level > 1.005 && !root.muted ? Lithic.jasper : root.levelInk
            }
            // the part below 100% keeps the accent; only the overdrive is red
            Rectangle {
                visible: bar.level > 1.005
                width: bar.unityX
                height: bar.height
                radius: height / 2
                color: root.levelInk
            }
            Rectangle {
                visible: root.maxValue > 1.005
                x: Math.round(bar.unityX - width / 2)
                anchors.verticalCenter: parent.verticalCenter
                width: Math.max(1, Math.round(2 * root.u))
                height: bar.height + 6 * root.u
                radius: width / 2
                color: Lithic.fg2
            }
        }

        Text {
            id: figure
            visible: !root.keyboard
            anchors.right: parent.right
            anchors.rightMargin: 16 * root.u
            anchors.verticalCenter: parent.verticalCenter
            width: Math.ceil(26 * root.u)
            horizontalAlignment: Text.AlignRight
            text: Math.round(root.value * 100)
            color: root.over && !root.muted ? Lithic.jasper : Lithic.fg3
            font.family: Lithic.font.sans
            font.pixelSize: Math.round(12 * root.u)
            font.features: ({ "tnum": 1 })
            renderType: Text.CurveRendering
        }

        Text {
            id: layoutName
            visible: root.keyboard
            anchors.left: icon.right
            anchors.leftMargin: 12 * root.u
            anchors.verticalCenter: parent.verticalCenter
            width: Math.min(implicitWidth, root.pillWidth - 64 * root.u)
            elide: Text.ElideRight
            text: root.label
            color: Lithic.fg
            font.family: Lithic.font.sans
            font.weight: Font.Bold
            font.pixelSize: Math.round(14 * root.u)
            renderType: Text.CurveRendering
        }
    }

    onIconNameChanged: if (root.shown && !Lithic.reduceMotion) nod.restart()
    SequentialAnimation {
        id: nod
        NumberAnimation { target: icon; property: "scale"; to: 1.12; duration: 110; easing.type: Easing.OutQuad }
        NumberAnimation { target: icon; property: "scale"; to: 1; duration: 220; easing.type: Easing.OutBack }
    }
}
