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
    property bool hovered: false
    readonly property real inset: Math.round((QsLook.material ? 16 : QsLook.lithic ? 8 : 12) * root.s)
    readonly property real hairline: Math.max(1, Math.round(root.s))
    readonly property real outerCut: (QsLook.material ? 9 : QsLook.adwaita ? QsLook.cardCut : Look.cutRow) * root.s
    readonly property real innerCut: QsLook.material ? 1.6 * root.s : QsLook.adwaita ? 0 : root.outerCut
    readonly property color rest: QsLook.material ? (root.selected ? Lithic.container : Lithic.field)
        : QsLook.adwaita ? Look.card
        : QsLook.fluent && root.selected ? Look.plate
        : "transparent"
    readonly property color plane: Qt.colorEqual(root.rest, "transparent")
        ? (root.hovered ? QsLook.hover : "transparent")
        : Lithic.mix(root.rest, QsLook.material && root.selected ? Lithic.containerInk : Lithic.fg,
            root.hovered ? (QsLook.adwaita ? 0.04 : 0.07) : 0)

    CutRect {
        anchors.fill: parent
        cutTopLeft: root.first ? root.outerCut : root.innerCut
        cutTopRight: root.first ? root.outerCut : root.innerCut
        cutBottomLeft: root.last ? root.outerCut : root.innerCut
        cutBottomRight: root.last ? root.outerCut : root.innerCut
        cut: 0
        fill: root.plane
        strokeColor: QsLook.fluent && root.selected ? Look.plateStroke : "transparent"
        strokeWidth: QsLook.fluent && root.selected ? root.hairline : 0
        Behavior on fill {
            ColorAnimation { duration: QsLook.dur(QsLook.stateMs) }
        }
    }

    Rectangle {
        visible: QsLook.lithic
        width: parent.width
        height: root.hairline
        color: Lithic.vein
    }
    Rectangle {
        visible: QsLook.lithic && root.last
        anchors.bottom: parent.bottom
        width: parent.width
        height: root.hairline
        color: Lithic.vein
    }
    Rectangle {
        visible: QsLook.adwaita && !root.first
        x: root.inset
        width: parent.width - root.inset * 2
        height: root.hairline
        color: QsLook.divider
    }
}
