pragma Singleton

import Quickshell
import stage
import stage.modules.common
import stage.modules.common.functions

/**
 * What Edit Mode may not draw on: the edges the bar and the dock occupy.
 *
 * Both stay where they are, at full size, while the mode is on (the dock's
 * catalogue edits the live surface in place), so the desktop shrinks inside
 * what is LEFT of the screen and the toolbar sits in a band opened inside
 * that. Three windows derive the viewport from these four numbers - the wallpaper surface, the
 * widgets surface and the chrome - and a second file working them out would be
 * a second answer to "where is the dock".
 *
 * The bar's inset is its BODY, derived from configuration: the bar's layer
 * surface carries a hover strip and shadow padding several times the bar's
 * height (194 px for a 40 px bar on the machine this was measured on), so the
 * surface is the wrong number. The dock's inset is published by the dock
 * window itself (`GlobalStates.dockInsets`) from the one thickness it reveals
 * at rest - the dock is the only place that knows its padding and style.
 *
 * The reservation is a function of configuration only, never of auto-hide,
 * hover reveal or `barOpen`: those move while the mode is on, and a viewport
 * that changes size mid-edit rescales every widget under the cursor. The
 * mode holds both panels visible for exactly that reason.
 */
Singleton {
    id: root

    readonly property bool barVertical: Config.options?.bar?.vertical ?? false
    // `bar.bottom` is the bar's far side on both axes: bottom for the
    // horizontal bar, right for the vertical one (VerticalBar.qml anchors on
    // the same flag).
    readonly property string barSide: root.barVertical
        ? ((Config.options?.bar?.bottom ?? false) ? "right" : "left")
        : ((Config.options?.bar?.bottom ?? false) ? "bottom" : "top")
    // The Hug style draws its screen corners below the body; the Float style
    // already carries its gaps inside barHeight.
    readonly property real barThickness: (root.barVertical
            ? Appearance.sizes.verticalBarWindowWidth
            : Appearance.sizes.barHeight)
        + ((Config.options?.bar?.cornerStyle ?? 0) === 0 ? Appearance.rounding.screenRounding : 0)

    // The bar's own screen rule, so a screen it is not drawn on gets none of
    // its inset. The dock runs on every screen and has no such list.
    function barShownOn(screenName) {
        return GlobalStates.isScreenAllowedForBar({ "name": screenName });
    }

    // The two panels can share an edge (a bottom bar over a bottom dock), so
    // the terms add rather than the larger winning.
    function insetsFor(screenName) {
        const insets = { "top": 0, "bottom": 0, "left": 0, "right": 0 };
        if (root.barShownOn(screenName))
            insets[root.barSide] += root.barThickness;
        const dock = GlobalStates.dockInsets[screenName];
        if (dock && (dock.side in insets) && dock.thickness > 0)
            insets[dock.side] += dock.thickness;
        return insets;
    }

    // The one derivation of the viewport every surface reads: the insets
    // above plus the Appearance tokens, through edit_mode.js's arithmetic.
    function viewportFor(screenName, screenWidth, screenHeight) {
        const insets = root.insetsFor(screenName);
        return EditModeLogic.viewportGeometry({
            "screenWidth": screenWidth,
            "screenHeight": screenHeight,
            "drawerWidth": Appearance.sizes.editModeDrawerWidth,
            "margin": Appearance.sizes.editModeMargin,
            "edgeMargin": Appearance.sizes.editModeEdgeMargin,
            "chromeThickness": Appearance.sizes.toolbarHeight,
            "insetTop": insets.top,
            "insetBottom": insets.bottom,
            "insetLeft": insets.left,
            "insetRight": insets.right
        });
    }

    // The desktop's shrink as one matrix, for a surface that hosts the real
    // desktop (Ryoku's wallpaper and widget layer) rather than the reference's
    // own background windows. Same arithmetic the chrome frames itself with:
    // the viewport above, the mode's progress, the drawer's sideways travel.
    function editMatrixFor(screenName, screenWidth, screenHeight, progress, drawerProgress) {
        const viewport = root.viewportFor(screenName, screenWidth, screenHeight);
        const shift = EditModeLogic.drawerTravel(viewport) * drawerProgress;
        const t = EditModeLogic.atProgress(viewport, progress, shift);
        return Qt.matrix4x4(
            t.scale, 0, 0, t.x,
            0, t.scale, 0, t.y,
            0, 0, 1, 0,
            0, 0, 0, 1);
    }

    // The card that matrix draws the desktop into, as a rect in screen space:
    // what EditModeCard cuts its blurred surround around (the reference's
    // BackgroundRoot.editCard, for a surface that is not the reference's).
    function cardRectFor(screenName, screenWidth, screenHeight, progress, drawerProgress) {
        const viewport = root.viewportFor(screenName, screenWidth, screenHeight);
        const shift = EditModeLogic.drawerTravel(viewport) * drawerProgress;
        const c = EditModeLogic.cardRect(viewport, progress, screenWidth, screenHeight, shift);
        return Qt.rect(c.x, c.y, c.width, c.height);
    }
}
