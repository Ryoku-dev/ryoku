pragma Singleton
import QtQuick
import Quickshell
import shell.services as Svc

Singleton {
    id: root

    // Every duration runs through the shell's Motion.dur: Animations' Speed scales
    // it, and reduce motion, lowPowerMode or the Power Saver profile collapse it to
    // an instant cut so a weak GPU stops repainting through transitions.
    readonly property bool reduce: Svc.Motion.reduce

    readonly property int fast:       Svc.Motion.dur(140)
    readonly property int standard:   Svc.Motion.dur(300)
    readonly property int shapeshift: Svc.Motion.dur(820)
    readonly property int glide:      Svc.Motion.dur(260)
    readonly property int heat:       Svc.Motion.dur(1100)
    readonly property int easeStandard: Easing.OutCubic
    readonly property int easeMorph:    Easing.BezierSpline

    // liquid morph curve, cubic-bezier(0.16, 1, 0.3, 1). front-loaded like an
    // exponential chase but with a long visible settle tail. pair with easeMorph
    // (BezierSpline).
    readonly property var morphCurve: [0.16, 1, 0.3, 1, 1, 1]
    readonly property real rSmall: 7
    readonly property real rTile:  13

    // looping scan/pairing breath pulse.
    readonly property int pulse: Svc.Motion.dur(420)

    // Shutter motion explains a state change rather than decorating every
    // keystroke. Task 7 consumes open/close for the layer-surface lifecycle.
    readonly property int open:      Svc.Motion.dur(210)
    readonly property int close:     Svc.Motion.dur(160)
    readonly property int shutter:   Svc.Motion.dur(360)
    readonly property int drawer:    Svc.Motion.dur(280)
    readonly property int selection: Svc.Motion.dur(90)
    readonly property int shelf:     Svc.Motion.dur(180)

}
