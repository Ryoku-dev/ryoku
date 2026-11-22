pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Lithic
import shell.services

Item {
    id: root

    property real s: 1
    readonly property real cut: QsLook.frameCut * root.s
    readonly property real quietCut: QsLook.frameQuietCut * root.s
    readonly property real hairline: Math.max(1, Math.round(root.s))

    RectangularShadow {
        visible: !Perf.shadowsDisabled
        anchors.fill: parent
        offset.y: QsLook.shadowY * root.s
        blur: QsLook.shadowBlur * root.s
        spread: -Math.round(4 * root.s)
        radius: QsLook.lithic ? root.cut / 2 : Lithic.cornerRadius(root.cut, width, height)
        color: QsLook.shadow
    }
    CutRect {
        id: plane
        anchors.fill: parent
        cutTopLeft: root.cut
        cutBottomRight: root.cut
        cutTopRight: QsLook.lithic ? root.quietCut : root.cut
        cutBottomLeft: QsLook.lithic ? root.quietCut : root.cut
        fill: QsLook.frame
        strokeColor: QsLook.frameStroke
        strokeWidth: Qt.colorEqual(QsLook.frameStroke, "transparent") ? 0 : root.hairline
    }
    CutRect {
        visible: QsLook.fluent
        anchors.fill: parent
        anchors.margins: root.hairline
        cut: Math.max(0, root.cut - root.hairline)
        fill: "transparent"
        strokeColor: QsLook.frameInner
        strokeWidth: root.hairline
    }
}
