import QtQuick
import Lithic

Item {
    id: root

    property real s: 1
    property bool shown: false
    // the control's own cut, already in px
    property real cut: 0
    property bool pill: false
    readonly property real ring: Math.max(1, Math.round(2 * root.s))
    readonly property real outset: root.ring * 2

    anchors.fill: parent
    anchors.margins: -root.outset
    visible: root.shown
    z: 10

    CutRect {
        visible: !root.pill
        anchors.fill: parent
        cut: root.cut > 0 ? root.cut + root.outset : 0
        fill: "transparent"
        strokeColor: Lithic.focusColor
        strokeWidth: root.ring
    }
    Rectangle {
        visible: root.pill
        anchors.fill: parent
        radius: height / 2
        color: "transparent"
        border.width: root.ring
        border.color: Lithic.focusColor
    }
}
