pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Lithic
import shell.services
import "../ask/look"

Singleton {
    id: qs

    readonly property bool lithic: Look.lithic
    readonly property bool material: Look.material
    readonly property bool adwaita: Look.adwaita
    readonly property bool fluent: Look.fluent
    readonly property bool dark: Look.dark
    readonly property bool still: Look.still

    readonly property real width: 440
    readonly property real pad: qs.material ? 18 : qs.adwaita ? 14 : 16
    readonly property real gap: qs.lithic ? 13 : 12
    readonly property color frame: Look.frame
    readonly property color frameStroke: Look.frameStroke
    readonly property color frameInner: Look.frameInner
    readonly property real frameCut: Look.cutFrame
    readonly property real frameQuietCut: Look.cutFrameQuiet
    readonly property color shadow: Look.shadow
    readonly property real shadowY: qs.lithic ? 5 : 10
    readonly property real shadowBlur: qs.lithic ? 20 : 36
    readonly property color card: qs.lithic ? "transparent" : Look.card
    readonly property color cardStroke: qs.lithic ? Lithic.vein : Look.cardStroke
    readonly property real cardCut: Look.cutCard
    readonly property color divider: qs.lithic ? Lithic.vein : Look.divider
    readonly property color hover: Look.hover
    readonly property color press: Look.press

    // the others head only the blocks that would not read without a name
    function headed(block) {
        return qs.lithic || block === "vitals" || block === "powerProfile";
    }
    readonly property color headInk: qs.material ? Lithic.sel : Lithic.fg2

    readonly property real tileGap: qs.lithic ? 6 : 8
    function tileColumns(size) {
        return qs.fluent ? 3 : qs.material && size === "small" ? 4 : 2;
    }
    function tileHeight(size) {
        if (qs.material)
            return size === "small" ? 64 : 72;
        if (qs.fluent)
            return 48;
        if (qs.adwaita)
            return size === "small" ? 44 : 52;
        return size === "small" ? 40 : 52;
    }
    readonly property real tileLabelRow: qs.fluent ? 32 : 0
    function tileCut(size) {
        return qs.material ? 10 : qs.fluent ? LithicTokens.cut.small
            : size === "small" ? LithicTokens.cut.small : LithicTokens.cut.mid;
    }
    readonly property real tilePressedCut: 5
    readonly property bool tilePill: qs.adwaita
    function tileRest(on) {
        if (on)
            return qs.lithic ? Lithic.container : Lithic.sel;
        return qs.lithic ? "transparent"
            : qs.material ? Lithic.field
            : qs.adwaita ? Lithic.alpha(Lithic.fg, qs.dark ? 0.1 : 0.08)
            : Lithic.alpha(Lithic.fg, qs.dark ? 0.06 : 0.045);
    }
    function tileFill(on, hovered, pressed) {
        const rest = qs.tileRest(on);
        if (qs.lithic && !on)
            return pressed ? qs.press : hovered ? qs.hover : rest;
        const ink = on && !qs.lithic ? Lithic.onSel : Lithic.fg;
        return Lithic.mix(rest, ink, pressed ? 0.12 : hovered ? 0.07 : 0);
    }
    function tileStroke(on) {
        return qs.lithic ? (on ? Lithic.alpha(Lithic.sel, 0.6) : Lithic.vein)
            : qs.fluent && !on ? Lithic.alpha(Lithic.fg, qs.dark ? 0.09 : 0.12)
            : "transparent";
    }
    function tileInk(on) {
        return on && !qs.lithic ? Lithic.onSel : Lithic.fg;
    }
    function tileDetailInk(on) {
        return on && !qs.lithic ? Lithic.onSel2 : Lithic.fg2;
    }
    function tileGlyphInk(on) {
        return qs.lithic ? (on ? Lithic.sel : Lithic.fg2) : on ? Lithic.onSel : Lithic.fg;
    }

    readonly property real levelHeight: qs.material ? 48 : qs.lithic ? 46 : 40
    readonly property real levelTrack: qs.material ? 44 : 4

    readonly property real holdHeight: qs.material ? 44 : 40

    readonly property real segmentHeight: qs.material ? 40 : qs.adwaita ? 34 : 36

    readonly property real iconButton: qs.material ? 40 : 32
    readonly property bool roundButtons: qs.material || qs.adwaita

    readonly property string sans: Look.sans
    readonly property string mono: Look.mono
    readonly property string figures: Look.figures
    readonly property real figureScale: Look.figureScale

    // opening, closing and the rising blocks follow the Quick settings speed, the rest keeps the library pace
    function panelMs(ms) {
        return qs.still ? 0 : Math.round(ms * SidebarState.speedScale);
    }
    function dur(ms) {
        return Lithic.duration(ms);
    }
    readonly property int openMs: Look.openMs
    readonly property int closeMs: Look.closeMs
    readonly property var arrive: Look.arrive
    readonly property var leave: Look.leave
    readonly property int stateMs: Look.stateMs
    readonly property int unfoldMs: 240
    readonly property var unfold: Look.outCubic
    readonly property int revealMs: 200
    readonly property int revealStagger: 30
    readonly property real revealTravel: 8
    readonly property real nod: qs.lithic ? 0 : qs.material ? -14 : -9

    function temperatureInk(celsius) {
        return !isFinite(celsius) ? Lithic.fg : celsius >= 90 ? Lithic.jasper
            : celsius >= 75 ? Lithic.amber : Lithic.fg;
    }
    function batteryInk(percent, charging) {
        return charging || !isFinite(percent) ? Lithic.fg
            : percent < 10 ? Lithic.jasper : percent < 20 ? Lithic.amber : Lithic.fg;
    }
}
