pragma ComponentBehavior: Bound

import QtQuick
import "framebars/menus"
import shell.services

// One reference frame menu (contract 05 sec 7). It is NOT a window, overlay
// card or blob: the shell paints its background as a scene-graph extension of
// the frame band. Full-height sidebars ride one fixed-size translated panel;
// compact menus clip the same fixed-size body into a scaling edge/corner rect.
// Width is EXACTLY minimumWidth (no natural-width growth), rendered at
// reference pixels (scale 1) to match the unscaled frame band.
//
// Openness is owned by FrameMenuManager (menuOpen), never a private boolean, so
// a busy anchor's content is replaced by flipping which record is active. The
// Ryoku-owned surfaces (power/voice/keyring/system) are NOT menus; they are
// hosted by FrameSurface (still a Popout blob this pass), delegated below.
Item {
    id: root

    anchors.fill: parent

    // Set by the FrameMenuManager delegate.
    property var record: null
    // The live menuState record carries dynamic fields the static record lacks,
    // such as `off` for voice opened in its inactive state.
    property var openRecord: null
    readonly property bool recordOff: !!(root.openRecord && root.openRecord.off)
    property string anchor: "left"
    property bool menuOpen: false
    property var manager: null
    property var clearances: null
    property real triggerAlong: -1
    property real s: 1
    property bool active: true
    // Forwarded to FrameSurface for the Ryoku-own surface popouts.
    property var group: null
    property real frameThickness: 0
    property real radius: 8
    property real smoothing: 0

    signal requestClose()

    readonly property var widgetIds: record && record.widgets ? record.widgets : []
    readonly property real minWidth: record && record.minWidth ? record.minWidth : 200
    readonly property string kind: record && record.kind ? record.kind : "menu"
    readonly property bool isMenu: root.kind === "menu"

    // The reveal fades the body content in and out; on a same-region swap the
    // two bodies crossfade (200 ms) while the chrome band stays put. A retained
    // body starts that fade only after its nested content is ready. The reveal
    // also holds a non-retained body mounted through close until it is flush.
    property real bodyReveal: root.menuOpen && (!root.retainBody || root.bodyReady) ? 1 : 0
    Behavior on bodyReveal { NumberAnimation { duration: Motion.crossfade; easing.type: Motion.crossfadeCurve } }
    readonly property bool effectiveOpen: root.menuOpen || root.bodyReveal > 0.004
    readonly property bool retainBody: root.isMenu && root.fillsBand
    readonly property bool bodyReady: menuBody.status === Loader.Ready
        && !!(menuBody.item && menuBody.item.ready)
    readonly property bool stableSidePanel: root.sideMenu && root.manager
        && !!root.manager.activeMenu && !root.manager.chromeSwitchPending
        && root.manager.waitingChromeId === ""
        && root.manager.chromeReveal > 0.996 && root.manager.chromeOpacity > 0.996

    Timer {
        interval: 2000
        running: root.menuOpen && root.retainBody && !root.bodyReady
        onTriggered: root.requestClose()
    }
    // Per-edge clearance (bar band + border, or the bare frame lip) so the
    // formulas reference all four bar thicknesses, not just this anchor's.
    readonly property real clL: root.clearances && typeof root.clearances.left === "number" ? root.clearances.left : 0
    readonly property real clR: root.clearances && typeof root.clearances.right === "number" ? root.clearances.right : 0
    readonly property real clT: root.clearances && typeof root.clearances.top === "number" ? root.clearances.top : 0
    readonly property real clB: root.clearances && typeof root.clearances.bottom === "number" ? root.clearances.bottom : 0
    // Band height available to a side menu = monitor height minus the top and
    // bottom bar thicknesses (contract 05 sec 7, H = MH - T - B).
    readonly property real bandH: Math.max(0, root.height - root.clT - root.clB)

    readonly property string edge: root.anchor.indexOf("top") === 0 ? "top"
        : root.anchor.indexOf("bottom") === 0 ? "bottom"
        : root.anchor
    readonly property bool sideMenu: root.edge === "left" || root.edge === "right"
    readonly property bool atLeft: root.anchor === "left" || root.anchor.indexOf("-left") >= 0
    readonly property bool atRight: root.anchor === "right" || root.anchor.indexOf("-right") >= 0
    readonly property bool atBottom: root.anchor.indexOf("bottom") === 0

    // Natural body height; side menus fill the band (top-pinned content), edge
    // and corner menus size to content, capped at the band with the panel's own
    // vertical scroll taking any overflow.
    readonly property real contentH: menuBody.item ? menuBody.item.implicitHeight : 0

    // A sole "always" widget fills the band at the fixed side-menu height.
    // Content-sized menus instead derive their resting height from the column.
    readonly property string expansion: root.record && root.record.expansion ? root.record.expansion : "never"
    readonly property bool fillsBand: root.widgetIds.length === 1 && root.expansion === "always"

    // Resting panel rect in window coords (contract 05 sec 7). Width is exactly
    // minimumWidth; the manager animates the panel to and from this target.
    readonly property real restW: root.minWidth
    readonly property real restH: (root.sideMenu || root.fillsBand) ? root.bandH : Math.min(root.contentH, root.bandH)
    // A top/bottom menu opens over the widget that summoned it, clamped inside
    // the side rails, instead of always jumping to the centre of the screen --
    // the same "grows out of its trigger" reading the side rails already give.
    function alongX(w) {
        if (root.triggerAlong < 0) return (root.clL + root.width - root.clR) / 2 - w / 2;
        return Math.max(root.clL, Math.min(root.width - root.clR - w, root.triggerAlong - w / 2));
    }
    readonly property real restX: root.atLeft ? root.clL
        : root.atRight ? (root.width - root.clR - root.restW)
        : root.alongX(root.restW)
    readonly property real restY: root.atBottom ? (root.height - root.clB - root.restH) : root.clT

    // The animated panel is shared by one menu at a time. A cross-anchor swap
    // keeps the incoming body hidden until the manager has retracted the
    // previous edge and transferred chrome ownership.
    readonly property bool sharesChrome: !!(root.manager && root.record
        && !root.manager.chromeSwitchPending && root.manager.chromeAnchor === root.anchor)
    readonly property var panel: root.manager && root.record
        && (root.manager.chromeOwner === root.record.id || root.sharesChrome)
        ? root.manager.chromePanel : null

    // Feed the manager the resting rect + anchor while open. On close the
    // manager holds the last rect and retracts it; on a same-region swap the new
    // menu pushes the same rect, so the panel never blinks.
    function pushChrome() {
        if (!root.isMenu || !root.menuOpen || !root.manager
                || (root.retainBody && !root.bodyReady))
            return;
        // Compute the rect atomically from width/height so a top-anchored y
        // never lags a just-changed height.
        const w = root.restW;
        const h = root.restH;
        const x = root.atLeft ? root.clL
            : root.atRight ? (root.width - root.clR - w)
            : root.alongX(w);
        const y = root.atBottom ? (root.height - root.clB - h) : root.clT;
        root.manager.setChromeSource(root.record.id, root.anchor, x, y, w, h,
            root.sideMenu || root.fillsBand);
    }
    onMenuOpenChanged: {
        if (root.menuOpen && root.retainBody && !root.bodyReady && root.manager)
            root.manager.prepareChromeSource(root.record.id, root.anchor);
        else
            root.pushChrome();
    }
    onBodyReadyChanged: if (root.bodyReady) root.pushChrome()
    // -1 is the manager clearing the trigger as the record closes, not a move to
    // the centre of the bar; pushing that would re-open the band on its way out.
    onTriggerAlongChanged: if (root.triggerAlong >= 0) root.pushChrome()
    onRestXChanged: root.pushChrome()
    onRestYChanged: root.pushChrome()
    onRestWChanged: root.pushChrome()
    onRestHChanged: root.pushChrome()
    Component.onCompleted: root.pushChrome()

    // Input mask: the resting panel rect while open. The manager's `anyVisible`
    // keeps the overlay's full input region through the closing animation even
    // after this per-menu rect drops away.
    readonly property real maskX: root.isMenu ? root.restX : (surfaceHost.item ? surfaceHost.item.maskX : 0)
    readonly property real maskY: root.isMenu ? root.restY : (surfaceHost.item ? surfaceHost.item.maskY : 0)
    readonly property real maskW: root.isMenu ? (root.menuOpen ? root.restW : 0) : (surfaceHost.item ? surfaceHost.item.maskW : 0)
    readonly property real maskH: root.isMenu ? (root.menuOpen ? root.restH : 0) : (surfaceHost.item ? surfaceHost.item.maskH : 0)

    // ---- Reference-menu path: a plain clipped body riding the panel rect.
    Item {
        id: clipHost
        clip: true
        visible: root.isMenu && root.effectiveOpen && root.bodyReady
        enabled: root.menuOpen
        x: root.panel ? root.panel.x : root.restX
        y: root.panel ? root.panel.y : root.restY
        width: root.panel ? root.panel.w : 0
        height: root.panel ? root.panel.h : 0

        // A folder bar style hides the frame band, so a menu that would ride it
        // paints its own card here instead, in the same rect the band would fill.
        Rectangle {
            visible: !!(root.manager && root.manager.topBar)
            x: menuBody.x
            y: menuBody.y
            width: root.restW
            height: root.restH
            radius: root.radius
            color: Theme.surface
            border.width: Theme.borderWidth
            border.color: Theme.outline
            opacity: Theme.windowOpacity
        }

        Loader {
            id: menuBody
            active: root.isMenu && (root.effectiveOpen || root.retainBody)
            // Band-filling side menus are retained between opens and incubated
            // before first use. Content-sized menus still load synchronously
            // because their measured height defines the band.
            asynchronous: root.retainBody
            width: root.restW
            height: root.restH
            // A moving sidebar fades as one surface with its background. Once
            // settled, same-anchor replacements use each body's local crossfade.
            x: root.atLeft ? 0
                : root.atRight ? (clipHost.width - root.restW)
                : ((clipHost.width - root.restW) / 2)
            y: root.atBottom ? (clipHost.height - root.restH) : 0
            opacity: menuBody.status === Loader.Ready
                ? ((root.sideMenu && root.manager)
                    ? root.manager.chromeOpacity * (root.stableSidePanel ? root.bodyReveal : 1)
                    : root.bodyReveal)
                : 0
            sourceComponent: menuColumnBody
        }
    }

    Component {
        id: menuColumnBody
        MenuColumn {
            width: root.restW
            height: root.restH
            scale: 1
            open: root.effectiveOpen
            mounted: root.effectiveOpen
            retain: root.retainBody
            incubate: root.retainBody
            widgets: root.widgetIds
            onRequestClose: root.requestClose()
        }
    }

    // ---- Ryoku-own surface path: still a Popout blob this pass (FrameSurface).
    Loader {
        id: surfaceHost
        anchors.fill: parent
        active: !root.isMenu
        sourceComponent: surfaceComponent
    }
    Component {
        id: surfaceComponent
        FrameSurface {
            group: root.group
            frameThickness: root.frameThickness
            radius: root.radius
            smoothing: root.smoothing
            s: root.s
            active: root.active
            record: root.record
            off: root.recordOff
            anchor: root.anchor
            menuOpen: root.menuOpen
            triggerAlong: root.triggerAlong
            onRequestClose: root.requestClose()
        }
    }
}
