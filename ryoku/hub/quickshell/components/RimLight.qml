import QtQuick
import QtQuick.Shapes
import Lithic

// a light that runs round a control's rim: a short streak, bright in the middle and
// fading at both ends, over a faint gloss on the whole edge, in the control's own
// shape (cut in Lithic, rounded in the other designs). It lies on the control's edge,
// above it, and takes no input. It holds still under reduced motion.
//
// The rings are built once, as shapes; the streak is a conical gradient on its ring,
// and only the gradient's angle moves, so a running light costs the GPU a uniform per
// frame and no script or repaint.
Shape {
    id: rim

    // the control whose rim it lights
    property Item target: null
    // the control's own cut, so the light follows its outline
    property real cut: LithicTokens.cut.small
    // room for the streak's soft glow outside the edge
    readonly property real glow: 3

    // the streak's place, in degrees as a conical gradient counts them (counter-clockwise
    // from three o'clock): the page's one clock (SettingsPage.rimTurn) drives every rim
    property real turn: 135

    x: target ? target.x - glow : 0
    y: target ? target.y - glow : 0
    width: target ? target.width + glow * 2 : 0
    height: target ? target.height + glow * 2 : 0
    visible: !!target && target.visible && target.width > 0
    preferredRendererType: Shape.CurveRenderer

    readonly property bool _round: Lithic.shape.corner === "round"
    // light on a dark design, the ink on a light one
    readonly property color _light: Lithic.dark ? "#ffffff" : Lithic.fg

    // one outline as SVG path data, `inset` in from the shape's edge
    function outline(inset, c) {
        var x0 = inset, y0 = inset, x1 = rim.width - inset, y1 = rim.height - inset;
        c = Math.max(0, Math.min(c, (x1 - x0) / 2, (y1 - y0) / 2));
        if (rim._round)
            return "M " + (x0 + c) + " " + y0 + " H " + (x1 - c) + " A " + c + " " + c + " 0 0 1 " + x1 + " " + (y0 + c)
                + " V " + (y1 - c) + " A " + c + " " + c + " 0 0 1 " + (x1 - c) + " " + y1
                + " H " + (x0 + c) + " A " + c + " " + c + " 0 0 1 " + x0 + " " + (y1 - c)
                + " V " + (y0 + c) + " A " + c + " " + c + " 0 0 1 " + (x0 + c) + " " + y0 + " Z ";
        return "M " + (x0 + c) + " " + y0 + " L " + (x1 - c) + " " + y0 + " L " + x1 + " " + (y0 + c)
            + " L " + x1 + " " + (y1 - c) + " L " + (x1 - c) + " " + y1 + " L " + (x0 + c) + " " + y1
            + " L " + x0 + " " + (y1 - c) + " L " + x0 + " " + (y0 + c) + " Z ";
    }

    // a ring `thick` wide whose middle lies `out` outside the control's edge. The corner
    // is the control's own (a pill stays a pill), grown or shrunk with the offset.
    function ring(out, thick) {
        var w = rim.width - rim.glow * 2, h = rim.height - rim.glow * 2;
        var c = Lithic.cornerRadius(rim.cut * Lithic.scaleOf(rim), w, h);
        // a cut corner's diagonal sits closer in than a round one's arc
        var k = rim._round ? 1 : 2 - Math.SQRT2;
        var mid = rim.glow - out, a = mid - thick / 2, b = mid + thick / 2;
        return rim.outline(a, Math.max(0, c + (out + thick / 2) * k)) + rim.outline(b, Math.max(0, c + (out - thick / 2) * k));
    }

    component Streak: ConicalGradient {
        property real strength: 1
        property real reach: 0.07
        readonly property color light: rim._light
        centerX: rim.width / 2
        centerY: rim.height / 2
        angle: rim.turn
        GradientStop { position: 0; color: Qt.rgba(light.r, light.g, light.b, strength) }
        GradientStop { position: reach * 0.45; color: Qt.rgba(light.r, light.g, light.b, strength * 0.35) }
        GradientStop { position: reach; color: Qt.rgba(light.r, light.g, light.b, 0) }
        GradientStop { position: 1 - reach; color: Qt.rgba(light.r, light.g, light.b, 0) }
        GradientStop { position: 1 - reach * 0.45; color: Qt.rgba(light.r, light.g, light.b, strength * 0.35) }
        GradientStop { position: 1; color: Qt.rgba(light.r, light.g, light.b, strength) }
    }

    // the gloss: the whole edge, faint
    ShapePath {
        strokeColor: "transparent"
        fillRule: ShapePath.OddEvenFill
        fillColor: Qt.rgba(rim._light.r, rim._light.g, rim._light.b, 0.10)
        PathSvg { path: rim.width > 0 ? rim.ring(0.5, 1) : "" }
    }
    // the streak's glow
    ShapePath {
        strokeColor: "transparent"
        fillRule: ShapePath.OddEvenFill
        fillGradient: Streak { strength: 0.22; reach: 0.09 }
        PathSvg { path: rim.width > 0 ? rim.ring(0.5, 4) : "" }
    }
    // the streak
    ShapePath {
        strokeColor: "transparent"
        fillRule: ShapePath.OddEvenFill
        fillGradient: Streak { strength: 0.95; reach: 0.07 }
        PathSvg { path: rim.width > 0 ? rim.ring(0.5, 1.5) : "" }
    }
}
