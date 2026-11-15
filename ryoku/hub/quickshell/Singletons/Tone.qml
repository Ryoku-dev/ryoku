pragma Singleton

import QtQuick
import Quickshell
import Lithic

// Settings' surfaces and motion, in the design language the desktop draws in (Lithic.design).
// Lithic lays everything on one paper and draws each plane by its cut hairline; the round
// designs lift the page and its groups of rows off the ground in their palette's surface
// containers. Every shape is a CutRect, so its corners follow the design too.
Singleton {
    readonly property string design: Lithic.shape.id
    readonly property bool flat: design === "lithic"

    // the planes a design lays out, from a role set (Lithic itself, or Lithic.rolesFor(id)
    // for a preview of another design)
    function planes(id, r) {
        var flat = id === "lithic";
        return {
            side: id === "material" || id === "adwaita" ? r.stone1 : r.ground,
            pane: id === "fluent" ? r.stone3 : r.stone2,
            paneStroke: flat || id === "fluent" ? r.vein : "transparent",
            // a plain slab (r.slab) is drawn by its hairline alone; a filled one stands on its fill
            card: flat ? r.slab : id === "material" ? r.stone1 : id === "adwaita" ? r.stone3 : r.stone4,
            cardStroke: flat ? r.vein : "transparent"
        };
    }
    readonly property var _planes: planes(design, Lithic)

    readonly property color side: _planes.side
    readonly property color pane: _planes.pane
    readonly property color paneStroke: _planes.paneStroke
    readonly property color card: _planes.card
    readonly property color cardStroke: _planes.cardStroke
    readonly property color line: Lithic.alpha(Lithic.fg, Lithic.dark ? 0.08 : 0.1)
    readonly property color guide: Lithic.alpha(Lithic.fg, Lithic.dark ? 0.16 : 0.2)
    readonly property color guideLit: Lithic.alpha(Lithic.fg, Lithic.dark ? 0.36 : 0.42)
    readonly property color hover: Lithic.alpha(Lithic.fg, Lithic.dark ? 0.06 : 0.07)
    readonly property color press: Lithic.alpha(Lithic.fg, Lithic.dark ? 0.1 : 0.12)
    readonly property color accent: Lithic.sel
    readonly property color accentInk: Lithic.onSel
    // the open page's wash and the ink on it
    readonly property color select: Lithic.container
    readonly property color selectInk: Lithic.containerInk

    // cuts, which each design turns into its own corner
    readonly property int paneCut: LithicTokens.cut.large
    readonly property int cardCut: LithicTokens.cut.mid
    readonly property int rowCut: LithicTokens.cut.small

    // quick, interruptible feedback; a new target cuts into a running move
    readonly property int quick: Lithic.duration(120)
    readonly property int move: Lithic.duration(240)
    readonly property int unfold: Lithic.duration(260)
    readonly property int stagger: Lithic.reduceMotion ? 0 : 18
}
