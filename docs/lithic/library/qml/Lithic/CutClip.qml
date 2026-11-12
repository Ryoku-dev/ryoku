import QtQuick

// clips content to a cut outline: the outline is the intersection of this
// rectangle and a 45 degree square, so the stencil clip needs no mask layer
Item {
    id: root

    property real cut: 0
    property bool squareBottom: false
    default property alias content: inner.data

    readonly property real _clipHeight: squareBottom ? height + width : height
    readonly property real _side: Math.max(0, (width + _clipHeight - cut * 2) / Math.SQRT2)
    // the content centre sits above the square's centre when the bottom is square
    readonly property real _shift: (height - _clipHeight) / 2 / Math.SQRT2

    clip: true

    Item {
        id: diamond
        width: root._side
        height: root._side
        x: (root.width - width) / 2
        y: (root._clipHeight - height) / 2
        rotation: 45
        clip: true

        Item {
            id: inner
            width: root.width
            height: root.height
            x: (diamond.width - root.width) / 2 + root._shift
            y: (diamond.height - root.height) / 2 + root._shift
            rotation: -45
        }
    }
}
