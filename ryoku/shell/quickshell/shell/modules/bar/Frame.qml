pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import Ryoku.Blobs
import Ryoku.Ui.Singletons
import shell.services
import shell.modules.stage.frame
import "framebars/RailGeometry.js" as RailGeometry
import "../../services/lib/screens.js" as Screens

// One monitor's frame bar. While Sumi is active it maps four exclusive-zone
// background surfaces that reserve the revealed bar's thickness, plus a
// full-screen transparent overlay that hosts the painted frame chrome, rail
// widgets, and this monitor's frame menu/popout host. Reveal binds to this
// monitor's ShellState: the bar toggle shortcut flips ShellState.barRevealed and
// each edge then follows its Config reveal flag, so the resting desktop shows
// exactly the configured edges. A rail widget's menu or surface intent opens on
// this monitor's FrameMenuManager in-process; daemon-driven prompts arrive on
// the ShellState surface bus and open on the owning monitor.
Scope {
    id: root

    // The screen this frame bar draws on, injected by the per-screen Variants in
    // shell.qml; null only for the instant before that binding lands.
    property var modelData: null

    // This monitor's dock lane, fed from the sibling DockSurface in shell.qml: the
    // edge it sits on, and its band's along-edge size and centre. The record island
    // parks flush beside the band on a shared edge instead of drifting to a corner.
    property string dockLaneEdge: ""
    property real dockLaneSize: 0
    property real dockLaneCenter: 0

    readonly property real frameBorderPx: Config.frameThickness

    // This monitor's per-monitor UI scale (shell.json displays.ui_scale, default
    // 1.0). The bar content (railScale) and the reserved band (edgeReserve) both
    // multiply by this one factor, so a scaled bar stays matched to its reserve
    // without touching the compositor scale apps depend on.
    readonly property real uiScale: Tokens.uiScaleFor(root.modelData ? root.modelData.name : "")
    readonly property bool barEnabled: Tokens.barEnabledFor(
        root.modelData ? root.modelData.name : "")
    // The screen that hosts whole-desktop surfaces: the primary output. QS Bar
    // needs it even when its bar is hidden (its popups/notifications live
    // there), and so does any frame-family style (iRiS): the scene is one
    // island, dock and popup system for the desktop, not a per-screen bar, so
    // hiding a monitor's bar must not blank the whole shell.
    readonly property bool primaryHost: root.modelData
        && ShellState.screens.length > 0
        && ShellState.screens[0].name === root.modelData.name
        && (Config.barStyle === "qsbar" || BarProducts.isFrameFamily(Config.barStyle))

    // The built-in Sumi frame scene draws while the active bar style is the
    // built-in one, or while a broken style has degraded to it; a healthy
    // receipt-owned style loads its own scene instead.
    readonly property bool sumiActive: root.degraded
        || BarProducts.sceneUrl(Config.barStyle) === ""

    // A transient Loader.Error on a builtin style must not stick us on sumi;
    // count retries so a hiccup self-heals. Once a builtin is genuinely broken
    // (retries spent), BarProducts records it and every monitor degrades to the
    // painted sumi rail instead of an empty frame, until the style changes.
    property int barStyleRetries: 0
    readonly property bool degraded: BarProducts.brokenStyle === Config.barStyle

    readonly property var state: root.modelData ? ShellState.forScreen(root.modelData) : null
    readonly property bool revealed: root.state ? root.state.barRevealed : true

    // The frame bar's rail-action handler. External tools (lock, screenshot,
    // colour picker) still run through their standalone scripts; the launcher and
    // wallpaper switcher are in-process surfaces now, so their buttons flip this
    // monitor's ShellState flag instead of spawning a ryoku-shell client. Session
    // actions (logout/reboot/shutdown) raise the confirmation dialog on this
    // monitor, which runs the action only on the positive press.
    function runBarAction(id) {
        switch (id) {
        case "lock": Quickshell.execDetached(["ryoku-shell", "lock"]); break;
        case "logout":
        case "reboot":
        case "shutdown": ShellState.askSessionAction(id, root.modelData ? root.modelData.name : ""); break;
        case "screenshot": Spawn.run(["ryoku-cmd-ryoshot"]); break;
        case "wallpaper": Quickshell.execDetached(["ryogami", "wallpaper", "ui"]); break;
        case "color-picker": Quickshell.execDetached(["ryoku-cmd-color-picker"]); break;
        case "app-launcher": if (root.state) root.state.launcherOpen = !root.state.launcherOpen; break;
        default: return;
        }
    }

    // Whether an edge's bar is revealed: this monitor's master reveal gated by
    // that edge's Config reveal flag. A hidden bar still reveals on hover, which
    // FrameRail handles from its own 1px strip.
    function edgeRevealed(edge) {
        if (!root.revealed)
            return false;
        const rail = Config.normalizedFrameBars.rails[edge];
        return !!(rail && rail.reveal);
    }
    // Framing reveals the painted rail without changing the reservation window.
    // That keeps client geometry still while the complete desktop is in the card.
    function edgeDrawRevealed(edge) {
        const screenName = root.modelData ? root.modelData.name : "";
        if (!StageFrame.framing(screenName))
            return root.edgeRevealed(edge);
        const rail = Config.normalizedFrameBars.rails[edge];
        return !!(rail && rail.reveal);
    }

    function edgeDrawReserve(edge) {
        if (!root.barEnabled)
            return 0;
        const rail = Config.normalizedFrameBars.rails[edge];
        if (!rail)
            return 0;
        if (!root.edgeDrawRevealed(edge))
            return 1;
        return ((rail.enabled && root.railHasWidgets(rail, edge)) ? rail.size * root.uiScale : 1) + root.frameBorderPx;
    }

    function railHasWidgets(rail, edge) {
        if (!rail)
            return false;
        const zs = (edge === "top" || edge === "bottom") ? ["start", "center", "end"] : ["top", "center", "bottom"];
        for (let i = 0; i < zs.length; ++i)
            if (Array.isArray(rail[zs[i]]) && rail[zs[i]].length > 0)
                return true;
        return false;
    }

    // The exclusive zone an edge reserves: the bar band plus the frame border
    // when the edge is revealed and holds widgets, else a 1px lip, so a hidden or
    // empty edge releases its screen space.
    function edgeReserve(edge) {
        if (!root.barEnabled)
            return 0;
        const rail = Config.normalizedFrameBars.rails[edge];
        if (!rail)
            return 0;
        if (!root.edgeRevealed(edge))
            return 1;
        return ((rail.enabled && root.railHasWidgets(rail, edge)) ? rail.size * root.uiScale : 1) + root.frameBorderPx;
    }

    readonly property var sidebarRailClearances: !root.barEnabled ? ({
        top: 0, left: 0, bottom: 0, right: 0
    }) : root.sumiActive ? ({
        top: overlay.railClearance("top"),
        left: overlay.railClearance("left"),
        bottom: overlay.railClearance("bottom"),
        right: overlay.railClearance("right")
    }) : (overlay.qsBarEdge === "bottom"
        ? ({ top: 0, left: 0, bottom: 52, right: 0 })
        : ({ top: 52, left: 0, bottom: 0, right: 0 }))

    function syncSidebarRailClearances() {
        if (root.modelData)
            SidebarState.setRailClearances(root.modelData, root.sidebarRailClearances);
    }

    onSidebarRailClearancesChanged: root.syncSidebarRailClearances()
    Component.onCompleted: Qt.callLater(root.syncSidebarRailClearances)

    // The reservation stack owns shared corners by map order. Keep all four in
    // one lifetime so activating Sumi always maps top, bottom, left, right.
    LazyLoader {
        active: root.sumiActive
        component: Scope {
            FrameEdge { edge: "top";    screen: root.modelData; reserve: root.edgeReserve("top") }
            FrameEdge { edge: "bottom"; screen: root.modelData; reserve: root.edgeReserve("bottom") }
            FrameEdge { edge: "left";   screen: root.modelData; reserve: root.edgeReserve("left") }
            FrameEdge { edge: "right";  screen: root.modelData; reserve: root.edgeReserve("right") }
        }
    }

    PanelWindow {
        id: overlay

        // Physical panel height (Quickshell's is logical: a logical-only term
        // shrinks the popouts and the needle panel as the display scale rises),
        // the accessibility font scale, and this monitor's interface scale, so
        // the popout chrome matches the rails it grows out of.
        readonly property real s: Screens.monitorScale(root.modelData,
            Wm.outputScale(root.modelData ? root.modelData.name : ""))
            * Math.max(0.7, Math.min(1.6, Config.fontScale)) * root.uiScale
        readonly property var frameBars: Config.normalizedFrameBars
        // Read through `overlay.` and keep the rail host's id distinct: an id
        // shadows a same-named property in this scope, which once left every rail
        // rect at zero and swallowed all bar input.
        readonly property var rails: overlay.frameBars.rails
        readonly property real frameLip: root.frameBorderPx

        // fold qsbar's shared menus to whichever edge its bar sits on
        readonly property string qsBarEdge: (!root.sumiActive && Config.qsbar && Config.qsbar.barPosition === "bottom") ? "bottom" : "top"
        readonly property var edgeReveal: ({
            top: root.edgeDrawRevealed("top"),
            bottom: root.edgeDrawRevealed("bottom"),
            left: root.edgeDrawRevealed("left"),
            right: root.edgeDrawRevealed("right")
        })
        readonly property var topRailRect: RailGeometry.edgeRect("top", overlay.railThickness("top"), width, height)
        readonly property var leftRailRect: RailGeometry.edgeRect("left", overlay.railThickness("left"), width, height)
        readonly property var bottomRailRect: RailGeometry.edgeRect("bottom", overlay.railThickness("bottom"), width, height)
        readonly property var rightRailRect: RailGeometry.edgeRect("right", overlay.railThickness("right"), width, height)
        // The frame chrome is a full-output surface, so its framed layer keeps
        // the same extent and only maps the painted edges into the Stage card.
        function framedRect(rect) {
            return frameLayer.maskRect(rect.x, rect.y, rect.width, rect.height);
        }
        readonly property rect framedTopRailRect: overlay.framedRect(overlay.topRailRect)
        readonly property rect framedLeftRailRect: overlay.framedRect(overlay.leftRailRect)
        readonly property rect framedBottomRailRect: overlay.framedRect(overlay.bottomRailRect)
        readonly property rect framedRightRailRect: overlay.framedRect(overlay.rightRailRect)
        readonly property rect framedTopHoverRect: frameLayer.maskRect(0, 0, overlay.width, 1)
        readonly property rect framedBottomHoverRect: frameLayer.maskRect(0, overlay.height - 1, overlay.width, 1)
        readonly property rect framedLeftHoverRect: frameLayer.maskRect(0, 0, 1, overlay.height)
        readonly property rect framedRightHoverRect: frameLayer.maskRect(overlay.width - 1, 0, 1, overlay.height)
        function railRecord(edge) { return overlay.rails[edge] || ({ size: 0, enabled: false }); }
        function railThickness(edge) { return Math.max(0, root.edgeDrawReserve(edge) - root.frameBorderPx); }
        function railEnabled(edge) {
            return root.barEnabled && overlay.railRecord(edge).enabled === true;
        }
        // Clearance from the screen edge to the inside of each rail, per edge: the
        // reserve when the edge carries a bar, else the frame lip. Feeds the menu
        // manager (bodies clear their own rail) and the record island's dock.
        function railClearance(edge) {
            const reserve = root.edgeReserve(edge);
            return reserve > 0 ? reserve : overlay.frameLip;
        }

        // Right-edge input reserve: a 2px sliver (the hidden-bar trick) so the
        // frame reveals on hover there without the region eating clicks; off
        // while a fullscreen window owns the monitor.
        readonly property bool rightDropOn: !overlay.monFullscreen
        readonly property real rightDropW: Math.max(root.edgeReserve("right"), 2)

        // True when this monitor's active workspace holds a fullscreen window;
        // the frame then unmaps its input and hides so the window is unobstructed.
        readonly property bool monFullscreen: Wm.outputHasFullscreen(root.modelData ? root.modelData.name : "")

        onMonFullscreenChanged: if (monFullscreen) frameMenus.closeAll()

        screen: root.modelData
        // Sumi paints its rails here; other styles only host menus and popouts,
        // so the full-output surface maps only while one of them is showing.
        visible: root.sumiActive || frameMenus.anyShowing
        color: "transparent"
        exclusionMode: ExclusionMode.Ignore
        WlrLayershell.layer: WlrLayer.Top
        WlrLayershell.keyboardFocus: frameMenus.keyboardMode === "exclusive" ? WlrKeyboardFocus.Exclusive
            : frameMenus.keyboardMode === "ondemand" ? WlrKeyboardFocus.OnDemand
            : WlrKeyboardFocus.None
        WlrLayershell.namespace: "ryoku-frame"
        anchors { top: true; left: true; right: true; bottom: true }

        // Input catches the drawn rails and their 1px hover strips, plus every
        // open menu's trigger and body, the record island, and the right-edge
        // stash drop strip; everything else clicks through to the desktop. A
        // dragging island or any visible menu widens the mask to the whole
        // surface so the pointer never slips off its rect mid-interaction.
        mask: overlay.monFullscreen ? hiddenRegion
            : frameMenus.anyVisible ? fullRegion
            : root.sumiActive ? railRegion
            : dragRegion

        Region { id: hiddenRegion }
        Region {
            id: fullRegion
            width: overlay.width
            height: overlay.height
        }
        Region {
            id: railRegion
            Region { x: overlay.framedTopRailRect.x; y: overlay.framedTopRailRect.y; width: overlay.railEnabled("top") ? overlay.framedTopRailRect.width : 0; height: overlay.railEnabled("top") ? overlay.framedTopRailRect.height : 0 }
            Region { x: overlay.framedLeftRailRect.x; y: overlay.framedLeftRailRect.y; width: overlay.railEnabled("left") ? overlay.framedLeftRailRect.width : 0; height: overlay.railEnabled("left") ? overlay.framedLeftRailRect.height : 0 }
            Region { x: overlay.framedBottomRailRect.x; y: overlay.framedBottomRailRect.y; width: overlay.railEnabled("bottom") ? overlay.framedBottomRailRect.width : 0; height: overlay.railEnabled("bottom") ? overlay.framedBottomRailRect.height : 0 }
            Region { x: overlay.framedRightRailRect.x; y: overlay.framedRightRailRect.y; width: overlay.railEnabled("right") ? overlay.framedRightRailRect.width : 0; height: overlay.railEnabled("right") ? overlay.framedRightRailRect.height : 0 }
            // The reveal lips follow the card too; leaving them at the physical
            // edge makes a hidden rail impossible to reach in the editor.
            Region { x: overlay.framedTopHoverRect.x; y: overlay.framedTopHoverRect.y; width: overlay.railEnabled("top") ? overlay.framedTopHoverRect.width : 0; height: overlay.railEnabled("top") ? overlay.framedTopHoverRect.height : 0 }
            Region { x: overlay.framedBottomHoverRect.x; y: overlay.framedBottomHoverRect.y; width: overlay.railEnabled("bottom") ? overlay.framedBottomHoverRect.width : 0; height: overlay.railEnabled("bottom") ? overlay.framedBottomHoverRect.height : 0 }
            Region { x: overlay.framedLeftHoverRect.x; y: overlay.framedLeftHoverRect.y; width: overlay.railEnabled("left") ? overlay.framedLeftHoverRect.width : 0; height: overlay.railEnabled("left") ? overlay.framedLeftHoverRect.height : 0 }
            Region { x: overlay.framedRightHoverRect.x; y: overlay.framedRightHoverRect.y; width: overlay.railEnabled("right") ? overlay.framedRightHoverRect.width : 0; height: overlay.railEnabled("right") ? overlay.framedRightHoverRect.height : 0 }
            // frame menus: each open menu unions its trigger (owner) and body
            // rects so the click target and the open body keep catching input;
            // idle anchors stay zero-size and click through.
            Region { x: frameMenus.masks["top"].tx; y: frameMenus.masks["top"].ty; width: frameMenus.masks["top"].tw; height: frameMenus.masks["top"].th }
            Region { x: frameMenus.masks["top"].bx; y: frameMenus.masks["top"].by; width: frameMenus.masks["top"].bw; height: frameMenus.masks["top"].bh }
            Region { x: frameMenus.masks["top-left"].tx; y: frameMenus.masks["top-left"].ty; width: frameMenus.masks["top-left"].tw; height: frameMenus.masks["top-left"].th }
            Region { x: frameMenus.masks["top-left"].bx; y: frameMenus.masks["top-left"].by; width: frameMenus.masks["top-left"].bw; height: frameMenus.masks["top-left"].bh }
            Region { x: frameMenus.masks["top-right"].tx; y: frameMenus.masks["top-right"].ty; width: frameMenus.masks["top-right"].tw; height: frameMenus.masks["top-right"].th }
            Region { x: frameMenus.masks["top-right"].bx; y: frameMenus.masks["top-right"].by; width: frameMenus.masks["top-right"].bw; height: frameMenus.masks["top-right"].bh }
            Region { x: frameMenus.masks["left"].tx; y: frameMenus.masks["left"].ty; width: frameMenus.masks["left"].tw; height: frameMenus.masks["left"].th }
            Region { x: frameMenus.masks["left"].bx; y: frameMenus.masks["left"].by; width: frameMenus.masks["left"].bw; height: frameMenus.masks["left"].bh }
            Region { x: frameMenus.masks["right"].tx; y: frameMenus.masks["right"].ty; width: frameMenus.masks["right"].tw; height: frameMenus.masks["right"].th }
            Region { x: frameMenus.masks["right"].bx; y: frameMenus.masks["right"].by; width: frameMenus.masks["right"].bw; height: frameMenus.masks["right"].bh }
            Region { x: frameMenus.masks["bottom"].tx; y: frameMenus.masks["bottom"].ty; width: frameMenus.masks["bottom"].tw; height: frameMenus.masks["bottom"].th }
            Region { x: frameMenus.masks["bottom"].bx; y: frameMenus.masks["bottom"].by; width: frameMenus.masks["bottom"].bw; height: frameMenus.masks["bottom"].bh }
            Region { x: frameMenus.masks["bottom-left"].tx; y: frameMenus.masks["bottom-left"].ty; width: frameMenus.masks["bottom-left"].tw; height: frameMenus.masks["bottom-left"].th }
            Region { x: frameMenus.masks["bottom-left"].bx; y: frameMenus.masks["bottom-left"].by; width: frameMenus.masks["bottom-left"].bw; height: frameMenus.masks["bottom-left"].bh }
            Region { x: frameMenus.masks["bottom-right"].tx; y: frameMenus.masks["bottom-right"].ty; width: frameMenus.masks["bottom-right"].tw; height: frameMenus.masks["bottom-right"].th }
            Region { x: frameMenus.masks["bottom-right"].bx; y: frameMenus.masks["bottom-right"].by; width: frameMenus.masks["bottom-right"].bw; height: frameMenus.masks["bottom-right"].bh }
            // right edge stays masked so a file drag lands on the DropArea below.
            Region { x: overlay.width - overlay.rightDropW; y: 0; width: overlay.rightDropOn ? overlay.rightDropW : 0; height: overlay.rightDropOn ? overlay.height : 0 }
            // centred plugin popout: no edge anchor, so its body rides here.
            Region { x: frameMenus.pluginMask.x; y: frameMenus.pluginMask.y; width: frameMenus.pluginMask.w; height: frameMenus.pluginMask.h }
        }

        // folder / islands bar styles have no rail mask, so carry the stash drop
        // strip and the dock preview body on their own region for the idle-desktop
        // case -- without the dock body here an islands (qsbar) dock preview cannot
        // catch hover/click, so it melts the instant the pointer reaches a tile.
        Region {
            id: dragRegion
            Region { x: overlay.width - overlay.rightDropW; y: 0; width: overlay.rightDropOn ? overlay.rightDropW : 0; height: overlay.rightDropOn ? overlay.height : 0 }
            Region { x: frameMenus.dockMask.x; y: frameMenus.dockMask.y; width: frameMenus.dockMask.w; height: frameMenus.dockMask.h }
            Region { x: frameMenus.musicMask.x; y: frameMenus.musicMask.y; width: frameMenus.musicMask.w; height: frameMenus.musicMask.h }
            Region { x: frameMenus.pluginMask.x; y: frameMenus.pluginMask.y; width: frameMenus.pluginMask.w; height: frameMenus.pluginMask.h }
        }

        // Non-visual: authentication-island close bookkeeping and the keyboard
        // hand-back pulse. Dismissing a keyring/polkit island answers the blocked
        // caller, then asks ShellState to pulse the root kbBounce helper.
        FrameSurfaceLifecycle {
            id: surfaceLifecycle
            keyring: Keyring
            polkit: Polkit
            onFocusRestored: ShellState.restoreFocus()
        }

        // Outside-click closes whatever is open: a press that reaches the
        // backdrop, outside the bars and the open panel, dismisses. Presses on
        // rails and content never reach here; they hit the items above.
        MouseArea {
            anchors.fill: parent
            enabled: frameMenus.anyOpen
            acceptedButtons: Qt.AllButtons
            onPressed: mouse => {
                for (const a in frameMenus.masks) {
                    const m = frameMenus.masks[a];
                    if (!m) continue;
                    if (m.tw > 0 && mouse.x >= m.tx && mouse.x < m.tx + m.tw
                        && mouse.y >= m.ty && mouse.y < m.ty + m.th) return;
                    if (m.bw > 0 && mouse.x >= m.bx && mouse.x < m.bx + m.bw
                        && mouse.y >= m.by && mouse.y < m.by + m.bh) return;
                }
                if (mouse.x < root.edgeReserve("left") || mouse.y < root.edgeReserve("top")
                    || mouse.x >= overlay.width - root.edgeReserve("right")
                    || mouse.y >= overlay.height - root.edgeReserve("bottom"))
                    return;
                frameMenus.closeAll();
            }
        }

        FocusScope {
            id: focusScope
            anchors.fill: parent
            focus: frameMenus.anyOpen
            visible: !overlay.monFullscreen
            Keys.onEscapePressed: if (frameMenus.keyboardMode === "exclusive") frameMenus.closeAll()


            // Shared blob field for retained Ryoku-owned credential, voice,
            // stash, capture, and rail-card popouts.
            BlobGroup {
                id: blobGroup
                color: Theme.surface
                borderWidth: 0
                smoothing: 0
                shadowStrength: 0
                shadowSize: 0
            }

            // The animated menu extension is a simple scene-graph primitive;
            // changing its rect stays smooth while the frame path remains static.
            // Loaded only under Sumi: hidden but alive, the rails still rebuild
            // on every window change.
            Loader {
                active: root.sumiActive && root.barEnabled
                sourceComponent: Rectangle {
                    x: frameMenus.chromePanel.x
                    y: frameMenus.chromePanel.y
                    width: frameMenus.chromePanel.w
                    height: frameMenus.chromePanel.h
                    radius: Math.min(Config.frameCorner, width / 2, height / 2)
                    color: Theme.surface
                    border.width: Theme.borderWidth
                    border.color: Theme.outline
                    opacity: Theme.windowOpacity * (frameMenus.chromeSide ? frameMenus.chromeOpacity : 1)
                    visible: !overlay.monFullscreen && Config.frameEnabled
                        && width > 0 && height > 0
                }
            }

            StageFramedLayer {
                id: frameLayer
                interactive: false
                screenName: overlay.screen?.name ?? ""
                edge: "top"
                restThickness: overlay.screen?.height ?? 1080
                screenWidth: overlay.screen?.width ?? 1920
                screenHeight: overlay.screen?.height ?? 1080
            }

            Loader {
                parent: frameLayer.rest
                anchors.fill: parent
                active: root.sumiActive && root.barEnabled
                sourceComponent: Item {
                    FrameChrome {
                        // The painted frame band: per-edge thickness = that edge's reserve
                        // (bar band + border), the 2px outline stroked just inside the hole
                        // edge, inner corners arced at radiusWindow.
                        anchors.fill: parent
                        // No sumiActive arm: the Loader above requires it, so the
                        // band is never painted without it and a fallback thickness
                        // would describe a frame nobody sees. The reserves at the top of
                        // the file are the opposite case and keep theirs, because an
                        // exclusive zone is claimed whether or not anything is drawn.
                        reserveTop: root.edgeDrawReserve("top")
                        reserveBottom: root.edgeDrawReserve("bottom")
                        reserveLeft: root.edgeDrawReserve("left")
                        reserveRight: root.edgeDrawReserve("right")
                        holeRadius: Config.frameCorner
                        surface: Theme.surface
                        outline: Theme.outline
                        strokeWidth: Theme.borderWidth
                        opacity: Theme.windowOpacity
                        visible: !overlay.monFullscreen && Config.frameEnabled
                    }

                    Bar {
                        anchors.fill: parent
                        z: 1
                        visible: !overlay.monFullscreen
                        // The bar content scales by this monitor's uiScale and the reserve
                        // (edgeReserve) scales its band by the same factor, so the drawn
                        // bar and its reserved exclusive-zone thickness stay matched.
                        railScale: root.uiScale
                        revealState: overlay.edgeReveal
                        frameBars: overlay.frameBars
                        style: ({ group: blobGroup })
                        monitorName: root.modelData ? root.modelData.name : ""
                        onMenuRequested: (id, ownerRect) => frameMenus.openSurface(id, ownerRect, "")
                        onSurfaceRequested: (id, ownerRect) => frameMenus.openSurface(id, ownerRect, "")
                        onActionRequested: id => root.runBarAction(id)
                    }
                }
            }

            // per-monitor frame menu manager: a bar widget asks for a menu via
            // Bar.onMenuRequested; this monitor's manager opens it on the shared
            // Popout scene. The backdrop press above dismisses a click outside,
            // and Escape closes through the FocusScope. Daemon-driven prompts
            // arrive on the ShellState surface bus below.
            FrameMenuManager {
                id: frameMenus
                // Above the rails: a surface grows out of a rail and must cover
                // it, or the rail chrome clips the body's first rows.
                z: 2
                monitorName: root.modelData ? root.modelData.name : ""
                scale: overlay.s
                group: blobGroup
                topBar: !root.sumiActive || !root.barEnabled
                barEdge: overlay.qsBarEdge
                railClearances: root.sidebarRailClearances
                active: !overlay.monFullscreen
                onSurfaceClosed: (id, context) => surfaceLifecycle.handleClosed(id, context)

                // Daemon- and root-driven surface prompts land on the ShellState
                // bus; open only when this request targets this monitor (mon ""
                // broadcasts to every monitor, matching the reference pill).
                Connections {
                    target: ShellState
                    function onSurfaceRequested(id, mon, ctx) {
                        if (mon !== "" && mon !== (root.modelData ? root.modelData.name : ""))
                            return;
                        // The wallpaper + theme picker is the ryogami overlay now
                        // (Super+W); route its surface request straight there.
                        if (id === "wallpaper") {
                            Quickshell.execDetached(["ryogami", "wallpaper", "ui"]);
                            return;
                        }
                        // A request that carries a trigger opens welded to it,
                        // the same as a rail widget's own menuRequested above.
                        // Without one openSurface falls back to the record's
                        // config anchor, which a non-sumi bar folds into a
                        // screen corner.
                        const anchorRect = ctx && ctx.ownerRect ? ctx.ownerRect : null;
                        frameMenus.openSurface(id, anchorRect, mon, ctx);
                    }
                    function onSurfaceClosed(id, mon) { frameMenus.closeSurface(id, mon); }
                    function onKeyringPromptChanged(pid) { frameMenus.retireKeyringPrompt(pid); }
                }
            }

            // No compositor focus grab here. A modal surface already takes the
            // layer's exclusive keyboard focus, and the compositor clears a grab
            // the moment that focus moves to the grabbing layer: the two together
            // closed every surface a few milliseconds after it opened. The mask
            // plus the backdrop press above dismisses a click outside, and Escape
            // closes through the FocusScope.

        }
    }

    // Receipt-owned bar styles load from their durable content-addressed
    // views. Sumi is the built-in frame scene (the overlay above); any other
    // style loads its own scene here, without rails. The overlay and its menu
    // host still map (topBar mode) so menus and surfaces stay style-agnostic.
    Loader {
        id: barStyleLoader
        active: !root.sumiActive && (root.barEnabled || root.primaryHost)
        source: BarProducts.sceneUrl(Config.barStyle)
        onLoaded: {
            root.barStyleRetries = 0
            if (item) item.modelData = root.modelData
        }
        // A builtin style cannot be legitimately broken: a load error is a
        // transient hiccup (an update's config/plugin swap, a cold-start
        // import race), so retry it. Once retries are spent the style is
        // genuinely broken on this box: degrade to the painted sumi rail so
        // the user keeps a working bar instead of an empty frame, and record
        // it so the Hub can say so. A store style that errors is failed as
        // before (sceneUrl then returns "" and sumi draws). Both marks flip
        // this Loader's own `active`, so they land after the status change
        // settles instead of inside it.
        onStatusChanged: {
            if (status !== Loader.Error)
                return;
            const style = Config.barStyle;
            if (BarProducts.isBuiltin(style)) {
                if (root.barStyleRetries < 10) {
                    root.barStyleRetries++;
                    barStyleReload.restart();
                } else {
                    Qt.callLater(BarProducts.markBroken, style);
                }
            } else {
                Qt.callLater(BarProducts.fail, style);
            }
        }
    }
    Timer {
        id: barStyleReload
        interval: 800
        onTriggered: {
            barStyleLoader.active = false;
            barStyleLoader.active = Qt.binding(
                () => !root.sumiActive && (root.barEnabled || root.primaryHost));
        }
    }
    Connections {
        target: root
        function onModelDataChanged() {
            root.syncSidebarRailClearances();
            if (barStyleLoader.item)
                barStyleLoader.item.modelData = root.modelData;
        }
    }
    // A new pick gets a fresh set of retries; a style that broke earlier is
    // tried again once the user picks it again.
    Connections {
        target: Config
        function onBarStyleChanged() {
            root.barStyleRetries = 0;
            if (BarProducts.brokenStyle !== Config.barStyle)
                BarProducts.brokenStyle = "";
        }
    }
}
