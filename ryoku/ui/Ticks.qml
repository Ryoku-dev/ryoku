import QtQuick
import Ryoku.Ui.Singletons

// HUD corner ticks: four L-brackets registering a block's corners, the line
// vocabulary of the reference sheet. Ornament with a job: it marks a surface
// as an instrument panel, so it goes on framed blocks (a preview, a state
// plate), never on every cell.
Item {
    id: t

    property int arm: 7
    property color color: Tokens.lineStrong

    anchors.fill: parent

    // top-left
    Rectangle { x: 0; y: 0; width: t.arm; height: 1; color: t.color }
    Rectangle { x: 0; y: 0; width: 1; height: t.arm; color: t.color }
    // top-right
    Rectangle { x: parent.width - t.arm; y: 0; width: t.arm; height: 1; color: t.color }
    Rectangle { x: parent.width - 1; y: 0; width: 1; height: t.arm; color: t.color }
    // bottom-left
    Rectangle { x: 0; y: parent.height - 1; width: t.arm; height: 1; color: t.color }
    Rectangle { x: 0; y: parent.height - t.arm; width: 1; height: t.arm; color: t.color }
    // bottom-right
    Rectangle { x: parent.width - t.arm; y: parent.height - 1; width: t.arm; height: 1; color: t.color }
    Rectangle { x: parent.width - 1; y: parent.height - t.arm; width: 1; height: t.arm; color: t.color }
}
