pragma Singleton

import QtQuick
import Quickshell
import Lithic

Singleton {
    readonly property var _planes: Lithic.planes()

    readonly property color pane: _planes.pane
    readonly property color paneStroke: _planes.paneStroke
    readonly property color card: _planes.card
    readonly property color cardStroke: _planes.cardStroke

    readonly property color ink: Lithic.fg
    readonly property color ink2: Lithic.fg2
    readonly property color ink3: Lithic.fg3
    readonly property color line: Lithic.vein
    readonly property color danger: Lithic.jasper
    readonly property color dangerInk: Lithic.inkOn(Lithic.jasper)
    readonly property color select: Lithic.sel
    readonly property color selectInk: Lithic.onSel
    readonly property color lyric: Lithic.gold
    readonly property color scrim: Lithic.scrim

    readonly property int paneCut: LithicTokens.cut.large
    readonly property int cardCut: LithicTokens.cut.mid
    readonly property int controlCut: LithicTokens.cut.small
}
