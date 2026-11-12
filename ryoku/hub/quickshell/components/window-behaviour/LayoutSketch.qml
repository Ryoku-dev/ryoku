import QtQuick
import Lithic

// a still drawing of a tiling layout: the screen and its windows, the focused one in gold
CutRect {
    id: root

    property string layout: "dwindle"
    // [x, y, w, h] in screen fractions; the last one holds focus
    readonly property var frames: {
        switch (root.layout) {
        case "master":
            return [[0.6, 0, 0.4, 0.333], [0.6, 0.333, 0.4, 0.334], [0.6, 0.667, 0.4, 0.333], [0, 0, 0.6, 1]];
        case "scrolling":
            return [[-0.18, 0, 0.34, 1], [0.84, 0, 0.42, 0.5], [0.84, 0.5, 0.42, 0.5], [0.16, 0, 0.68, 1]];
        default:
            return [[0, 0, 0.5, 1], [0.5, 0, 0.5, 0.5], [0.75, 0.5, 0.25, 0.5], [0.5, 0.5, 0.25, 0.5]];
        }
    }
    readonly property real pad: LithicTokens.space.s2
    readonly property real gap: LithicTokens.space.s1

    implicitWidth: 240
    implicitHeight: Math.round(root.width * 0.625)
    fill: Lithic.field
    strokeColor: Lithic.edge
    strokeWidth: LithicTokens.size.hairline
    cut: LithicTokens.cut.small
    clip: true

    Item {
        id: screen
        x: root.pad
        y: root.pad
        width: root.width - root.pad * 2
        height: root.height - root.pad * 2
        clip: true

        Repeater {
            model: root.frames
            delegate: CutRect {
                required property var modelData
                required property int index
                readonly property bool focused: index === root.frames.length - 1
                x: modelData[0] * screen.width + root.gap / 2
                y: modelData[1] * screen.height + root.gap / 2
                width: modelData[2] * screen.width - root.gap
                height: modelData[3] * screen.height - root.gap
                fill: focused ? Lithic.stone4 : Lithic.stone2
                strokeColor: focused ? Lithic.gold : Lithic.vein
                strokeWidth: focused ? 2 : LithicTokens.size.hairline
                cut: LithicTokens.cut.token
            }
        }
    }
}
