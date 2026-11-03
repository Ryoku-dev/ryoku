// Standalone equivalent of MaterialShape for SDDM.
// Uses the exact same material-shapes.js polygon data as PasswordChars.qml in the shell.
// Import paths are relative to this file (dots/sddm/pixel/).
import QtQuick 2.15
import "shapes/material-shapes.js" as MaterialShapes
import "shapes/shapes/morph.js" as Morph

Canvas {
    id: root

    // 0=Clover4Leaf 1=Arrow 2=Pill 3=SoftBurst 4=Diamond 5=ClamShell 6=Pentagon
    // Cycles exactly as PasswordChars.qml does
    property int shapeIndex: 0
    property color shapeColor: "#cba6f7"

    // Animatable size — drives width/height together (matches PasswordChars implicitSize)
    property real implicitSize: 18
    width: implicitSize
    height: implicitSize

    onShapeColorChanged: requestPaint()
    onShapeIndexChanged: requestPaint()
    onImplicitSizeChanged: requestPaint()
    Component.onCompleted: requestPaint()

    onPaint: {
        var ctx = getContext("2d")
        ctx.clearRect(0, 0, width, height)
        if (width <= 0 || height <= 0) return

        var poly
        switch (root.shapeIndex % 7) {
            case 0: poly = MaterialShapes.getClover4Leaf(); break
            case 1: poly = MaterialShapes.getArrow();       break
            case 2: poly = MaterialShapes.getPill();        break
            case 3: poly = MaterialShapes.getSoftBurst();   break
            case 4: poly = MaterialShapes.getDiamond();     break
            case 5: poly = MaterialShapes.getClamShell();   break
            default: poly = MaterialShapes.getPentagon();   break
        }
        if (!poly) return

        var morph = new Morph.Morph(poly, poly)
        var cubics = morph.asCubics(1.0)
        if (!cubics || cubics.length === 0) return

        var size = Math.min(root.width, root.height)
        var ox = root.width  / 2 - size / 2
        var oy = root.height / 2 - size / 2

        ctx.save()
        ctx.fillStyle = root.shapeColor.toString()
        ctx.translate(ox, oy)
        ctx.scale(size, size)   // polygonIsNormalized = true
        ctx.beginPath()
        ctx.moveTo(cubics[0].anchor0X, cubics[0].anchor0Y)
        for (var i = 0; i < cubics.length; i++) {
            var c = cubics[i]
            ctx.bezierCurveTo(c.control0X, c.control0Y, c.control1X, c.control1Y, c.anchor1X, c.anchor1Y)
        }
        ctx.closePath()
        ctx.fill()
        ctx.restore()
    }
}
