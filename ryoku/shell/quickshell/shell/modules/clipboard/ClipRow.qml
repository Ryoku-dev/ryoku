pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import "../ask/look"

Item {
    id: root

    property real s: 1
    property var row: ({})
    property bool selected: false
    property bool numbers: false
    property bool copied: false

    readonly property real pad: Math.round(12 * root.s)
    readonly property real leadWidth: Math.round(40 * root.s)
    readonly property real textX: root.pad + root.leadWidth + Math.round(12 * root.s)
    readonly property color titleInk: root.selected ? Look.plateInk : Lithic.fg
    readonly property color subInk: root.selected && Look.material ? Lithic.alpha(Look.plateInk, 0.78) : Lithic.fg2
    readonly property color faintInk: root.selected && Look.material ? Lithic.alpha(Look.plateInk, 0.7) : Lithic.fg3
    readonly property int number: Number(root.row.number) || 0
    readonly property bool numbered: root.numbers && root.number > 0

    readonly property string marked: String(root.row.marked || "")
    readonly property string excerpt: String(root.row.excerpt || "")

    Item {
        id: lead
        x: root.pad
        anchors.verticalCenter: parent.verticalCenter
        width: root.leadWidth
        height: Math.round(30 * root.s)

        Shape {
            anchors.fill: parent
            visible: root.row.kind === "image"
            s: root.s
            cutSize: Look.cutToken
            fill: Look.well
            strokeColor: Look.lithic ? Lithic.vein : "transparent"
            strokeWidth: Look.lithic ? 1 : 0
            clip: true

            Image {
                anchors.fill: parent
                anchors.margins: Look.lithic ? 1 : 0
                source: root.row.kind === "image" && String(root.row.thumb || "").length > 0 ? "file://" + root.row.thumb : ""
                sourceSize: Qt.size(Math.ceil(width * 2), Math.ceil(height * 2))
                fillMode: Image.PreserveAspectCrop
                asynchronous: true
                cache: true
                opacity: root.copied ? 0.35 : 1
            }
        }

        Shape {
            anchors.centerIn: parent
            width: Math.round(22 * root.s)
            height: width
            visible: root.row.kind === "colour"
            s: root.s
            cutSize: Look.cutToken
            fill: String(root.row.swatch || "transparent")
            strokeColor: Lithic.alpha(Lithic.fg, 0.18)
            strokeWidth: 1
        }

        Symbol {
            id: mark
            anchors.centerIn: parent
            visible: root.copied || (root.row.kind !== "image" && root.row.kind !== "colour")
            s: root.s
            name: root.copied ? "check" : String(root.row.glyph || "")
            size: 18
            ink: root.copied ? Lithic.jade : root.selected ? Look.plateMark : Lithic.fg2
            chosen: root.selected || root.copied
            scale: root.copied ? 1 : 0.999
            Behavior on scale {
                NumberAnimation { duration: Look.dur(Look.stateMs * 2); easing.type: Easing.OutBack; easing.overshoot: 3 }
            }
            Behavior on color {
                ColorAnimation { duration: Look.dur(Look.stateMs) }
            }
        }
    }

    Column {
        x: root.textX
        y: Math.round(8 * root.s)
        width: Math.max(0, trailing.x - root.textX - Math.round(10 * root.s))
        spacing: Math.round(1 * root.s)

        Text {
            width: parent.width
            height: Math.round(19 * root.s)
            verticalAlignment: Text.AlignVCenter
            text: root.marked.length > 0 ? root.marked : String(root.row.title || "")
            textFormat: root.marked.length > 0 ? Text.StyledText : Text.PlainText
            color: root.titleInk
            font.family: root.row.mono === true ? Look.mono : Look.sans
            font.pixelSize: Math.round((root.row.mono === true ? 12.5 : 14) * root.s)
            elide: Text.ElideRight
            maximumLineCount: 1
            renderType: Text.NativeRendering
            Behavior on color {
                ColorAnimation { duration: Look.dur(Look.stateMs) }
            }
        }
        Text {
            width: parent.width
            text: root.excerpt.length > 0 ? root.excerpt : String(root.row.meta || "")
            textFormat: root.excerpt.length > 0 ? Text.StyledText : Text.PlainText
            color: root.subInk
            font.family: Look.sans
            font.pixelSize: Math.round(12 * root.s)
            font.features: ({ "tnum": 1 })
            elide: Text.ElideRight
            maximumLineCount: 1
            renderType: Text.NativeRendering
        }
    }

    Column {
        id: trailing
        anchors.right: parent.right
        anchors.rightMargin: root.pad
        y: Math.round(9 * root.s)
        spacing: Math.round(4 * root.s)

        Item {
            anchors.right: parent.right
            width: Math.max(stamp.implicitWidth, cap.width)
            height: Math.round(16 * root.s)

            Text {
                id: stamp
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                text: String(root.row.time || "")
                color: root.faintInk
                font.family: Look.figures
                font.pixelSize: Math.round(11 * Look.figureScale * root.s)
                font.features: ({ "tnum": 1 })
                opacity: root.numbered ? 0 : 1
                renderType: Text.NativeRendering
                Behavior on opacity {
                    NumberAnimation { duration: Look.dur(Look.stateMs) }
                }
            }
            KeyHint {
                id: cap
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                s: root.s
                hint: root.number > 0 ? String(root.number) : ""
                ink: root.faintInk
                opacity: root.numbered ? 1 : 0
                visible: opacity > 0.01
                Behavior on opacity {
                    NumberAnimation { duration: Look.dur(Look.stateMs) }
                }
            }
        }
        Symbol {
            id: star
            anchors.right: parent.right
            visible: root.row.starred === true
            s: root.s
            name: "star"
            size: 14
            ink: root.selected && Look.material ? Look.plateMark : Lithic.goldInk
            chosen: true
            font.variableAxes: ({ "FILL": 1, "wght": Look.iconWeight })
        }
    }
}
