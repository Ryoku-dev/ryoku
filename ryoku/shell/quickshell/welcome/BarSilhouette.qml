import QtQuick
import Ryoku.Ui.Singletons
import "Singletons"

Item {
    id: root

    property string styleId: "sumi"
    property bool selected: false
    readonly property color ink: selected ? Tokens.inkOnBone : Tokens.ink
    readonly property color dim: selected ? Tokens.inkOnBoneDim : Tokens.inkFaint

    function pill(context, x, y, width, height, radius) {
        context.beginPath()
        context.moveTo(x + radius, y)
        context.lineTo(x + width - radius, y)
        context.quadraticCurveTo(x + width, y, x + width, y + radius)
        context.lineTo(x + width, y + height - radius)
        context.quadraticCurveTo(x + width, y + height, x + width - radius, y + height)
        context.lineTo(x + radius, y + height)
        context.quadraticCurveTo(x, y + height, x, y + height - radius)
        context.lineTo(x, y + radius)
        context.quadraticCurveTo(x, y, x + radius, y)
        context.closePath()
    }

    scale: selected ? 1.04 : 1
    Behavior on scale {
        NumberAnimation { duration: Motion.move; easing.type: Tokens.ease }
    }

    Canvas {
        id: canvas
        anchors.fill: parent

        onPaint: {
            const c = getContext("2d")
            const w = width
            const h = height
            c.reset()
            c.fillStyle = root.ink
            c.strokeStyle = root.dim
            c.lineWidth = 1

            if (root.styleId === "sumi") {
                c.fillRect(6, 5, 7, h - 10)
                c.fillRect(17, 8, 3, 15)
                c.fillRect(17, h - 23, 3, 15)
                c.strokeRect(26, 7, w - 32, h - 14)
            } else if (root.styleId === "qsbar") {
                c.fillRect(7, 6, w - 14, 8)
                for (let i = 0; i < 7; i++)
                    c.fillRect(13 + i * ((w - 34) / 7), 20, Math.max(4, (w - 52) / 7), 5)
                c.strokeRect(7, 31, w - 14, h - 37)
            } else if (root.styleId === "kairos") {
                c.beginPath()
                root.pill(c, w * 0.31, 5, w * 0.38, 15, 7)
                c.fill()
                c.beginPath()
                c.arc(w * 0.5, 12.5, 3, 0, Math.PI * 2)
                c.fillStyle = root.selected ? Tokens.bone : Tokens.paper
                c.fill()
                c.strokeStyle = root.dim
                c.strokeRect(7, 28, w - 14, h - 34)
            } else if (root.styleId === "nomarchy") {
                c.strokeRect(7, 5, w - 14, 10)
                for (let i = 0; i < 4; i++) {
                    c.beginPath()
                    c.arc(14 + i * 7, 10, 1.5, 0, Math.PI * 2)
                    c.fill()
                }
                c.beginPath()
                root.pill(c, w * 0.45, 8, w * 0.1, 4, 2)
                c.fill()
                for (let i = 0; i < 3; i++)
                    c.fillRect(w - 31 + i * 7, 8, 3, 4)
                c.strokeStyle = root.dim
                c.strokeRect(7, 22, w - 14, h - 28)
            } else if (root.styleId === "iris") {
                c.beginPath()
                root.pill(c, w * 0.18, 5, w * 0.64, 13, 7)
                c.fill()
                c.beginPath()
                root.pill(c, w * 0.35, h - 15, w * 0.3, 9, 5)
                c.fill()
                c.strokeStyle = root.dim
                c.strokeRect(7, 24, w - 14, h - 45)
            } else {
                for (let i = 0; i < 5; i++) {
                    c.beginPath()
                    root.pill(c, 7 + i * ((w - 12) / 5), 5, Math.max(8, (w - 30) / 5), 13, 7)
                    c.fill()
                }
                c.beginPath()
                root.pill(c, w * 0.28, 26, w * 0.44, h - 32, 9)
                c.stroke()
            }
        }

        Component.onCompleted: requestPaint()
    }

    onStyleIdChanged: canvas.requestPaint()
    onInkChanged: canvas.requestPaint()
    onDimChanged: canvas.requestPaint()
}
