pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls as QQC
import Lithic
import "../ask/look"

QQC.ToolTip {
    id: root

    property real s: 1
    readonly property bool inverse: QsLook.material || QsLook.adwaita

    delay: QsLook.still ? 0 : LithicTokens.motion.tooltipDelay
    timeout: 6000
    padding: Math.round(7 * root.s)
    leftPadding: Math.round(10 * root.s)
    rightPadding: Math.round(10 * root.s)
    margins: Math.round(4 * root.s)
    y: -implicitHeight - Math.round(8 * root.s)
    x: (parent ? parent.width - implicitWidth : 0) / 2

    enter: Transition {
        NumberAnimation { property: "opacity"; from: 0; to: 1; duration: QsLook.dur(140); easing.type: Easing.OutCubic }
    }
    exit: Transition {
        NumberAnimation { property: "opacity"; from: 1; to: 0; duration: QsLook.dur(100); easing.type: Easing.OutCubic }
    }

    background: CutRect {
        cut: LithicTokens.cut.tooltip * root.s
        fill: root.inverse ? Lithic.fg : Lithic.stone4
        strokeColor: root.inverse ? "transparent" : Lithic.vein
        strokeWidth: Math.max(1, Math.round(root.s))
    }
    contentItem: Text {
        text: root.text
        color: root.inverse ? Lithic.ground : Lithic.fg
        font.family: QsLook.sans
        font.pixelSize: Math.round(12 * root.s)
        font.weight: Font.Medium
        wrapMode: Text.NoWrap
        renderType: Text.NativeRendering
    }
}
