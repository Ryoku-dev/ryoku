import QtQuick

// The transparency checker behind a composition with no background. The
// pattern is painted once at the output's size and only the window onto it
// follows the composition, so a lift or a padding drag never repaints it.
Item {
    id: checker

    // the composition on screen, in the parent's px
    property rect area: Qt.rect(0, 0, 0, 0)
    property real u: 1

    x: checker.area.x
    y: checker.area.y
    width: checker.area.width
    height: checker.area.height
    clip: true

    Rectangle {
        x: -checker.x
        y: -checker.y
        width: checker.parent ? checker.parent.width : 0
        height: checker.parent ? checker.parent.height : 0
        color: "#ffffff"

        Canvas {
            anchors.fill: parent
            renderStrategy: Canvas.Cooperative
            readonly property real cell: Math.round(10 * checker.u)
            onCellChanged: requestPaint()
            onWidthChanged: requestPaint()
            onHeightChanged: requestPaint()
            onPaint: {
                var ctx = getContext("2d");
                ctx.clearRect(0, 0, width, height);
                ctx.fillStyle = "#d4d4d4";
                var s = cell;
                for (var y = 0, r = 0; y < height; y += s, r++)
                    for (var x = (r % 2) * s; x < width; x += 2 * s)
                        ctx.fillRect(x, y, s, s);
            }
        }
    }
}
