pragma ComponentBehavior: Bound

import QtQuick
import Lithic

Item {
    id: root
    required property var frame
    property point pointer: Qt.point(0, 0)
    property real s: 1
    property real outputWidth: 1
    property real outputHeight: 1
    readonly property real inset: 18 * s
    width: 112 * s
    height: width
    x: Math.max(0, Math.min(outputWidth - width, pointer.x + inset + width <= outputWidth ? pointer.x + inset : pointer.x - width - inset))
    y: Math.max(0, Math.min(outputHeight - height, pointer.y + inset + height <= outputHeight ? pointer.y + inset : pointer.y - height - inset))
    CutRect { anchors.fill: parent; fill: Lithic.field; strokeColor: Lithic.vein; strokeWidth: 1; cut: 0 }
    Item {
        anchors.fill: parent
        anchors.margins: 1
        clip: true
        Image {
            source: root.frame ? "file://" + root.frame.path : ""
            width: root.frame ? Number(root.frame.width) * 6 : 0
            height: root.frame ? Number(root.frame.height) * 6 : 0
            x: parent.width / 2 - root.pointer.x * 6
            y: parent.height / 2 - root.pointer.y * 6
            sourceSize.width: root.frame ? Math.ceil(Number(root.frame.width) * Number(root.frame.scale || 1)) : 1
            sourceSize.height: root.frame ? Math.ceil(Number(root.frame.height) * Number(root.frame.scale || 1)) : 1
            smooth: false
            mipmap: false
            cache: false
            asynchronous: false
        }
        CutRect { x: parent.width / 2 - 0.5; width: 1; height: parent.height; fill: Lithic.alpha(Lithic.sel, 0.7) }
        CutRect { y: parent.height / 2 - 0.5; width: parent.width; height: 1; fill: Lithic.alpha(Lithic.sel, 0.7) }
    }
}
