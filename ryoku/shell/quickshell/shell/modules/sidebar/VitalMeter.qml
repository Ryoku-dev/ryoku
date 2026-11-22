pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import "../ask/look"

Item {
    id: root

    required property real s
    required property string label
    required property string valueText
    property color valueInk: Lithic.fg
    property string detailText: ""
    property color detailInk: Lithic.fg2
    property string tooltipText: ""
    property bool showProgress: false
    property real progressValue: 0
    property real progressMaximum: 100
    property int settle: 0
    property color accent: QsLook.lithic ? Lithic.fg2 : Lithic.sel
    property Component meterContent: null

    readonly property bool tiled: QsLook.material
    readonly property real pad: root.tiled ? Math.round(12 * root.s) : 0
    readonly property real head: QsLook.lithic ? Math.round(8 * root.s) : root.pad
    readonly property real line: Math.round(18 * root.s)
    readonly property real slot: Math.round(14 * root.s)
    readonly property real between: Math.round(8 * root.s)
    readonly property real progressRatio: root.showProgress && isFinite(root.progressValue) && root.progressMaximum > 0
        ? Math.max(0, Math.min(1, root.progressValue / root.progressMaximum)) : 0

    implicitHeight: root.head + root.line + Math.round(6 * root.s) + root.slot + root.pad
    Accessible.role: Accessible.StaticText
    Accessible.name: root.label + ", " + root.valueText + root.detailText

    Plate {
        visible: root.tiled
        anchors.fill: parent
        s: root.s
    }
    Rectangle {
        visible: QsLook.lithic
        anchors.left: parent.left
        anchors.right: parent.right
        height: Math.max(1, Math.round(root.s))
        color: Lithic.vein
    }

    Item {
        id: content
        anchors.fill: parent
        anchors.leftMargin: root.pad
        anchors.rightMargin: root.pad
        anchors.topMargin: root.head
        anchors.bottomMargin: root.pad

        Micro {
            id: name
            anchors.left: parent.left
            anchors.verticalCenter: reading.verticalCenter
            width: Math.max(Math.min(implicitWidth, parent.width / 3),
                Math.min(implicitWidth, parent.width - reading.implicitWidth - root.between))
            s: root.s
            text: root.label
        }

        Row {
            id: reading
            anchors.right: parent.right
            height: root.line
            width: Math.min(implicitWidth, parent.width - name.width - root.between)
            clip: true
            layoutDirection: Qt.RightToLeft

            Text {
                visible: root.detailText.length > 0
                anchors.verticalCenter: parent.verticalCenter
                text: root.detailText
                color: root.detailInk
                font.family: QsLook.lithic ? QsLook.figures : QsLook.sans
                font.pixelSize: Math.round((QsLook.lithic ? 13 * QsLook.figureScale : 13) * root.s)
                font.features: ({ "tnum": 1 })
                renderType: Text.NativeRendering
                Behavior on color {
                    ColorAnimation { duration: QsLook.dur(QsLook.stateMs * 2) }
                }
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: root.valueText
                color: root.valueInk
                font.family: QsLook.lithic ? QsLook.figures : QsLook.sans
                font.pixelSize: Math.round((QsLook.lithic ? 13 * QsLook.figureScale : 13) * root.s)
                font.weight: QsLook.lithic ? Font.Normal : Font.Medium
                font.features: ({ "tnum": 1 })
                renderType: Text.NativeRendering
                Behavior on color {
                    ColorAnimation { duration: QsLook.dur(QsLook.stateMs * 2) }
                }
            }
        }

        Item {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            height: root.slot

            Meter {
                visible: root.showProgress && root.meterContent === null
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.top: parent.top
                s: root.s
                value: root.progressRatio
                ink: root.accent
                settle: root.settle
            }
            Loader {
                anchors.fill: parent
                active: root.visible && root.meterContent !== null
                sourceComponent: root.meterContent
            }
        }
    }

    HoverHandler {
        id: hover
    }
    CornerTip {
        s: root.s
        visible: hover.hovered && root.tooltipText.length > 0
        text: root.tooltipText
    }
}
