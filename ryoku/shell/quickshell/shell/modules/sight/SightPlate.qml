pragma ComponentBehavior: Bound

import QtQuick
import Lithic

Item {
    id: root
    property string source: ""
    property var frame: null
    property rect rect: Qt.rect(0, 0, 0, 0)
    property bool committed: false
    property bool imageVisible: true
    property bool highlighted: false
    property real s: 1
    property real gap: (committed ? 3 : 6) * s
    readonly property bool rounded: Lithic.design === "material" || Lithic.design === "adwaita"
    readonly property color outline: Lithic.design === "adwaita" ? Lithic.fg : Lithic.sel
    readonly property real tickLength: (Lithic.design === "fluent" ? 5 : 9) * s
    x: rect.x
    y: rect.y
    width: rect.width
    height: rect.height
    visible: width > 0 && height > 0
    Behavior on gap { NumberAnimation { duration: Lithic.duration(160); easing.type: Easing.OutCubic } }

    Item {
        anchors.fill: parent
        clip: true
        Image {
            visible: root.imageVisible && root.source.length > 0
            source: root.source
            x: root.frame ? -root.rect.x : 0
            y: root.frame ? -root.rect.y : 0
            width: root.frame ? Number(root.frame.width) : root.width
            height: root.frame ? Number(root.frame.height) : root.height
            sourceSize.width: root.frame ? Math.ceil(Number(root.frame.width) * Number(root.frame.scale || 1)) : Math.ceil(root.width * 2)
            sourceSize.height: root.frame ? Math.ceil(Number(root.frame.height) * Number(root.frame.scale || 1)) : Math.ceil(root.height * 2)
            cache: false
            asynchronous: false
            fillMode: Image.Stretch
        }
    }
    CutRect {
        anchors.fill: parent
        cut: root.rounded ? 4 : 0
        fill: root.highlighted ? Lithic.alpha(Lithic.sel, 0.08) : "transparent"
        strokeColor: root.outline
        strokeWidth: Lithic.design === "material" ? 2 : 1
    }
    Repeater {
        model: 4
        delegate: Item {
            id: tick
            required property int index
            readonly property bool rightCorner: index % 2 === 1
            readonly property bool bottomCorner: index >= 2
            x: rightCorner ? root.width : 0
            y: bottomCorner ? root.height : 0
            visible: !root.rounded
            transform: Translate {
                x: tick.rightCorner ? root.gap : -root.gap
                y: tick.bottomCorner ? root.gap : -root.gap
            }
            CutRect {
                x: parent.rightCorner ? -root.tickLength : 0
                y: parent.bottomCorner ? -1 : 0
                width: root.tickLength; height: 1
                fill: Lithic.sel
            }
            CutRect {
                x: parent.rightCorner ? -1 : 0
                y: parent.bottomCorner ? -root.tickLength : 0
                width: 1; height: root.tickLength
                fill: Lithic.sel
            }
        }
    }
    Repeater {
        model: Lithic.design === "adwaita" ? 4 : 0
        delegate: CutRect {
            required property int index
            x: (index % 2 ? root.width : 0) - width / 2
            y: (index >= 2 ? root.height : 0) - height / 2
            width: 6 * root.s; height: width
            cut: 3
            fill: root.outline
        }
    }
}
