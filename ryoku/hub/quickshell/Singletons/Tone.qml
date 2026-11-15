pragma Singleton

import QtQuick
import Quickshell
import Lithic

// Settings' surfaces and motion: the sidebar lies on the ground, the page pane is lifted a
// step off it, and the groups of rows a step more, so the eye reads depth instead of a flat
// sheet drawn in hairlines. Every tone is taken from the palette in use, so Dark, Light and
// Wallpaper each keep their own colours.
Singleton {
    readonly property color side: Lithic.ground
    readonly property color pane: Lithic.dark ? Lithic.mix(Lithic.ground, Lithic.stone3, 0.55) : Lithic.mix(Lithic.ground, Lithic.stone4, 0.6)
    readonly property color card: Lithic.dark ? Lithic.mix(Lithic.stone3, Lithic.stone4, 0.7) : Lithic.stone4
    readonly property color line: Lithic.alpha(Lithic.fg, Lithic.dark ? 0.08 : 0.1)
    readonly property color guide: Lithic.alpha(Lithic.fg, Lithic.dark ? 0.16 : 0.2)
    readonly property color guideLit: Lithic.alpha(Lithic.fg, Lithic.dark ? 0.36 : 0.42)
    readonly property color hover: Lithic.alpha(Lithic.fg, Lithic.dark ? 0.06 : 0.07)
    readonly property color press: Lithic.alpha(Lithic.fg, Lithic.dark ? 0.1 : 0.12)
    readonly property color accent: Lithic.sel
    readonly property color onAccent: Lithic.onSel
    readonly property color accentSoft: Lithic.alpha(Lithic.sel, Lithic.dark ? 0.16 : 0.14)

    readonly property int radius: 12
    readonly property int radiusSmall: 8

    // quick, interruptible feedback; a new target cuts into a running move
    readonly property int quick: Lithic.duration(120)
    readonly property int move: Lithic.duration(240)
    readonly property int unfold: Lithic.duration(260)
    readonly property int stagger: Lithic.reduceMotion ? 0 : 18
}
