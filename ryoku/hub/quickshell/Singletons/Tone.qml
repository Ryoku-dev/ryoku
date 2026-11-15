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
    // for a preview of another design). Lithic: one paper, every plane a hairline. Material:
    // the page a step up from the ground, its cards a step more, no lines. Adwaita: a sidebar
    // set apart from the page, cards on it. Fluent: layered planes, each with a thin outline
    function planes(id, r) {
        if (id === "material")
            return { side: r.ground, pane: r.stone1, paneStroke: "transparent", card: r.stone3, cardStroke: "transparent" };
        if (id === "adwaita")
            return { side: r.stone3, pane: r.ground, paneStroke: "transparent", card: r.stone4, cardStroke: "transparent" };
        if (id === "fluent")
            return { side: r.ground, pane: r.stone3, paneStroke: r.vein, card: r.stone4, cardStroke: Lithic.alpha(r.vein, 0.7) };
        // a plain slab (r.slab) is drawn by its hairline alone
        return { side: r.ground, pane: r.stone2, paneStroke: r.vein, card: r.slab, cardStroke: r.vein };
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
