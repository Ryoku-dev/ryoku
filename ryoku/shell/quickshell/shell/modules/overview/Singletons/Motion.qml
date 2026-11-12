pragma Singleton
import QtQuick
import Quickshell
import shell.services as Svc

// Overview motion budget, matched to the shell's tokens so the open/close and
// the window glides read like the rest of the desktop (docs/ui-ux.md: keep
// durations and easing consistent, OutExpo/OutCubic, no bespoke curves).
Singleton {
    id: root

    // Every duration runs through the shell's Motion.dur: Animations' Speed scales
    // it, and reduce motion, lowPowerMode or the Power Saver profile collapse it to
    // an instant cut so a weak GPU stops repainting through the expo reveal.
    readonly property bool reduce: Svc.Motion.reduce

    readonly property int fast:     Svc.Motion.dur(140)
    readonly property int standard: Svc.Motion.dur(300)
    readonly property int window:   Svc.Motion.dur(240)
    readonly property int easeStandard: Easing.OutCubic
    readonly property int easeExpo:     Easing.OutExpo
    // window show/close spends its budget on a low-bounce spring, like the
    // launcher, so the reveal is the felt moment.
    readonly property real windowSpring:  reduce ? 12.0 : 3.0
    readonly property real windowDamping: reduce ? 1.0 : 0.85
    // tile drag snap-back and the active-cell ring track quickly.
    readonly property int highlight: Svc.Motion.dur(90)
}
