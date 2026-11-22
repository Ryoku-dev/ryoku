pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import "../../ask/look"
import ".."

Item {
    id: root

    property real s: 1
    property bool first: true
    property bool last: true
    property bool selected: false
    property bool checked: false
    property string glyph: ""
    property bool plated: false
    property bool plateOn: false
    property string title: ""
    property string meta: ""
    default property alias actions: tail.data

    readonly property bool washed: QsLook.material && root.selected
    readonly property real plateSide: Math.round(32 * root.s)

    implicitHeight: Math.max(Math.round((QsLook.material ? 64 : QsLook.lithic ? 52 : 56) * root.s),
        copy.implicitHeight + Math.round(20 * root.s))
    Accessible.role: Accessible.ListItem
    Accessible.name: root.title
    Accessible.description: root.meta

    HoverHandler { id: hover }

    RowPlane {
        id: plane
        anchors.fill: parent
        s: root.s
        first: root.first
        last: root.last
        selected: root.selected
        hovered: hover.hovered
    }

    Item {
        id: lead
        anchors.left: parent.left
        anchors.leftMargin: plane.inset
        anchors.verticalCenter: parent.verticalCenter
        width: root.plated ? root.plateSide : Math.round(22 * root.s)
        height: width

        CutRect {
            visible: root.plated && !QsLook.roundButtons
            anchors.fill: parent
            cut: Look.cutRow * root.s
            fill: root.plateOn ? Lithic.container : QsLook.lithic ? "transparent" : Look.plate
            strokeColor: QsLook.lithic && !root.plateOn ? Lithic.vein : QsLook.fluent ? Look.plateStroke : "transparent"
            strokeWidth: Math.max(1, Math.round(root.s))
            Behavior on fill {
                ColorAnimation { duration: QsLook.dur(QsLook.stateMs) }
            }
        }
        Rectangle {
            visible: root.plated && QsLook.roundButtons
            anchors.fill: parent
            radius: width / 2
            color: root.plateOn ? Lithic.sel
                : QsLook.material ? Lithic.container
                : Lithic.alpha(Lithic.fg, QsLook.dark ? 0.1 : 0.08)
            Behavior on color {
                ColorAnimation { duration: QsLook.dur(QsLook.stateMs) }
            }
        }
        Symbol {
            anchors.centerIn: parent
            s: root.s
            name: root.glyph
            size: root.plated ? 18 : 21
            chosen: root.plateOn || root.selected
            ink: root.washed ? Lithic.containerInk
                : !root.plated ? (root.selected && !QsLook.lithic ? Lithic.sel : Lithic.fg2)
                : root.plateOn ? (QsLook.lithic || QsLook.fluent ? Lithic.sel : Lithic.onSel)
                : QsLook.material ? Lithic.containerInk : Lithic.fg2
        }
    }

    Column {
        id: copy
        anchors.left: lead.right
        anchors.leftMargin: Math.round(12 * root.s)
        anchors.right: check.visible ? check.left : tail.left
        anchors.rightMargin: Math.round(10 * root.s)
        anchors.verticalCenter: parent.verticalCenter
        spacing: Math.round(2 * root.s)

        Text {
            width: parent.width
            text: root.title
            textFormat: Text.PlainText
            color: root.washed ? Lithic.containerInk : Lithic.fg
            font.family: QsLook.sans
            font.pixelSize: Math.round(14 * root.s)
            font.weight: Font.Medium
            elide: Text.ElideRight
            renderType: Text.NativeRendering
        }
        Text {
            width: parent.width
            visible: root.meta.length > 0
            text: root.meta
            textFormat: Text.PlainText
            color: root.washed ? Lithic.alpha(Lithic.containerInk, 0.8) : Lithic.fg2
            font.family: QsLook.sans
            font.pixelSize: Math.round(12 * root.s)
            font.features: ({ "tnum": 1 })
            elide: Text.ElideRight
            renderType: Text.NativeRendering
        }
    }

    Symbol {
        id: check
        visible: root.checked && !QsLook.lithic
        anchors.right: tail.left
        anchors.rightMargin: Math.round(8 * root.s)
        anchors.verticalCenter: parent.verticalCenter
        s: root.s
        name: "check"
        size: 20
        chosen: true
        ink: root.washed ? Lithic.containerInk : Lithic.sel
    }

    Row {
        id: tail
        anchors.right: parent.right
        anchors.rightMargin: plane.inset
        anchors.verticalCenter: parent.verticalCenter
        spacing: Math.round(4 * root.s)
    }
}
