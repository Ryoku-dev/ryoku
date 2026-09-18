pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import Ryoku.Blobs
import Ryoku.Ui.Singletons
import shell.services
import "framebars/RailGeometry.js" as RailGeometry

// One monitor's frame bar. It maps four exclusive-zone background surfaces that
// reserve the revealed bar's thickness (so tiled windows clear the rails) and a
// full-screen transparent overlay that hosts the painted frame chrome, the rail
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
    readonly property bool qsbarPrimaryHost: Config.barStyle === "qsbar"
        && root.modelData
        && ShellState.screens.length > 0
        && ShellState.screens[0].name === root.modelData.name

    // The built-in Sumi frame scene draws only while the active bar style is the
    // built-in one; a receipt-owned style would load its own scene without rails.
    readonly property bool sumiActive: BarProducts.sceneUrl(Config.barStyle) === ""

    // A transient Loader.Error on a builtin style must not stick us on sumi; count
    // retries so a genuinely broken style still degrades gracefully after ~8s.
    property int barStyleRetries: 0

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
        case "screenshot": Spawn.run(["sh", "-c", "flock -n -o /tmp/ryoshot.lock qs -c ryoshot"]); break;
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

    // Four background reservation surfaces, mapped top, bottom, left, right in
    // that fixed order so the horizontal edges own the shared corners.
    FrameEdge { edge: "top";    screen: root.modelData; reserve: root.sumiActive ? root.edgeReserve("top") : 0 }
    FrameEdge { edge: "bottom"; screen: root.modelData; reserve: root.sumiActive ? root.edgeReserve("bottom") : 0 }
    FrameEdge { edge: "left";   screen: root.modelData; reserve: root.sumiActive ? root.edgeReserve("left") : 0 }
    FrameEdge { edge: "right";  screen: root.modelData; reserve: root.sumiActive ? root.edgeReserve("right") : 0 }

    PanelWindow {
        id: overlay

        readonly property real s: (root.modelData ? root.modelData.height / 1080 : 1) * Math.max(0.7, Math.min(1.6, Config.fontScale))
        readonly property var frameBars: Config.normalizedFrameBars
        // Read through `overlay.` and keep the rail host's id distinct: an id
        // shadows a same-named property in this scope, which once left every rail
        // rect at zero and swallowed all bar input.
        readonly property var rails: overlay.frameBars.rails
        readonly property real frameLip: root.frameBorderPx

        // fold qsbar's shared menus to whichever edge its bar sits on
        readonly property string qsBarEdge: (!root.sumiActive && Config.qsbar && Config.qsbar.barPosition === "bottom") ? "bottom" : "top"
        readonly property var edgeReveal: ({
            top: root.edgeRevealed("top"),
            bottom: root.edgeRevealed("bottom"),
            left: root.edgeRevealed("left"),
            right: root.edgeRevealed("right")
        })
        readonly property var topRailRect: RailGeometry.edgeRect("top", overlay.railThickness("top"), width, height)
        readonly property var leftRailRect: RailGeometry.edgeRect("left", overlay.railThickness("left"), width, height)
        readonly property var bottomRailRect: RailGeometry.edgeRect("bottom", overlay.railThickness("bottom"), width, height)
        readonly property var rightRailRect: RailGeometry.edgeRect("right", overlay.railThickness("right"), width, height)
        function railRecord(edge) { return overlay.rails[edge] || ({ size: 0, enabled: false }); }
        function railThickness(edge) { return Math.max(0, root.edgeReserve(edge) - root.frameBorderPx); }
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
            : (frameMenus.anyVisible || recHud.dragging) ? fullRegion
            : root.sumiActive ? railRegion
            : (Recorder.anyActive || Recorder.chooserOpen) ? recRegion
            : dragRegion

        Region { id: hiddenRegion }
        Region {
            id: fullRegion
            width: overlay.width
            height: overlay.height
        }
        Region {
            id: railRegion
            Region { x: overlay.topRailRect.x; y: overlay.topRailRect.y; width: overlay.railEnabled("top") ? overlay.topRailRect.width : 0; height: overlay.railEnabled("top") ? overlay.topRailRect.height : 0 }
            Region { x: overlay.leftRailRect.x; y: overlay.leftRailRect.y; width: overlay.railEnabled("left") ? overlay.leftRailRect.width : 0; height: overlay.railEnabled("left") ? overlay.leftRailRect.height : 0 }
            Region { x: overlay.bottomRailRect.x; y: overlay.bottomRailRect.y; width: overlay.railEnabled("bottom") ? overlay.bottomRailRect.width : 0; height: overlay.railEnabled("bottom") ? overlay.bottomRailRect.height : 0 }
            Region { x: overlay.rightRailRect.x; y: overlay.rightRailRect.y; width: overlay.railEnabled("right") ? overlay.rightRailRect.width : 0; height: overlay.railEnabled("right") ? overlay.rightRailRect.height : 0 }
            // 1px hover strips at each screen edge, always exposed so a hidden bar
            // (reserve released, band mask gone) can still be revealed by pointer
            // proximity.
            Region { x: 0; y: 0; width: overlay.railEnabled("top") ? overlay.width : 0; height: overlay.railEnabled("top") ? 1 : 0 }
            Region { x: 0; y: overlay.height - 1; width: overlay.railEnabled("bottom") ? overlay.width : 0; height: overlay.railEnabled("bottom") ? 1 : 0 }
            Region { x: 0; y: 0; width: overlay.railEnabled("left") ? 1 : 0; height: overlay.railEnabled("left") ? overlay.height : 0 }
            Region { x: overlay.width - 1; y: 0; width: overlay.railEnabled("right") ? 1 : 0; height: overlay.railEnabled("right") ? overlay.height : 0 }
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
            // record island: its resting card and the tucked-nub reveal strip.
            Region { x: recHud.hudX; y: recHud.hudY; width: ((Recorder.anyActive || Recorder.chooserOpen) && recHud.prog > 0.25) ? recHud.hudW : 0; height: ((Recorder.anyActive || Recorder.chooserOpen) && recHud.prog > 0.25) ? recHud.hudH : 0 }
            Region { x: recHud.trigX; y: recHud.trigY; width: Recorder.anyActive ? recHud.trigW : 0; height: Recorder.anyActive ? recHud.trigH : 0 }
            // right edge stays masked so a file drag lands on the DropArea below.
            Region { x: overlay.width - overlay.rightDropW; y: 0; width: overlay.rightDropOn ? overlay.rightDropW : 0; height: overlay.rightDropOn ? overlay.height : 0 }
            // centred plugin popout: no edge anchor, so its body rides here.
            Region { x: frameMenus.pluginMask.x; y: frameMenus.pluginMask.y; width: frameMenus.pluginMask.w; height: frameMenus.pluginMask.h }
        }

        // Record island only, for a folder bar style (sumi frame + rails off). The
        // dock preview body rides along so its live tiles stay hoverable while a
        // recording HUD owns the mask.
        Region {
            id: recRegion
            Region { x: recHud.hudX; y: recHud.hudY; width: ((Recorder.anyActive || Recorder.chooserOpen) && recHud.prog > 0.25) ? recHud.hudW : 0; height: ((Recorder.anyActive || Recorder.chooserOpen) && recHud.prog > 0.25) ? recHud.hudH : 0 }
            Region { x: recHud.trigX; y: recHud.trigY; width: Recorder.anyActive ? recHud.trigW : 0; height: Recorder.anyActive ? recHud.trigH : 0 }
            Region { x: frameMenus.dockMask.x; y: frameMenus.dockMask.y; width: frameMenus.dockMask.w; height: frameMenus.dockMask.h }
            Region { x: frameMenus.musicMask.x; y: frameMenus.musicMask.y; width: frameMenus.musicMask.w; height: frameMenus.musicMask.h }
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
            // stash, capture, and rail-card popouts plus RecordHud.
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
            Rectangle {
                x: frameMenus.chromePanel.x
                y: frameMenus.chromePanel.y
                width: frameMenus.chromePanel.w
                height: frameMenus.chromePanel.h
                radius: Math.min(Config.frameCorner, width / 2, height / 2)
                color: Theme.surface
                border.width: Theme.borderWidth
                border.color: Theme.outline
                opacity: Theme.windowOpacity * (frameMenus.chromeSide ? frameMenus.chromeOpacity : 1)
                visible: !overlay.monFullscreen
                    && root.barEnabled && Config.frameEnabled && root.sumiActive
                    && width > 0 && height > 0
            }

            FrameChrome {
                // The painted frame band: per-edge thickness = that edge's reserve
                // (bar band + border), the 2px outline stroked just inside the hole
                // edge, inner corners arced at radiusWindow.
                anchors.fill: parent
                reserveTop: root.sumiActive ? root.edgeReserve("top") : root.frameBorderPx
                reserveBottom: root.sumiActive ? root.edgeReserve("bottom") : root.frameBorderPx
                reserveLeft: root.sumiActive ? root.edgeReserve("left") : root.frameBorderPx
                reserveRight: root.sumiActive ? root.edgeReserve("right") : root.frameBorderPx
                holeRadius: Config.frameCorner
                surface: Theme.surface
                outline: Theme.outline
                strokeWidth: Theme.borderWidth
                opacity: Theme.windowOpacity
                visible: !overlay.monFullscreen
                    && root.barEnabled && Config.frameEnabled && root.sumiActive
            }

            Bar {
                id: frameRails
                anchors.fill: parent
                z: 1
                visible: !overlay.monFullscreen && root.barEnabled && root.sumiActive
                // The bar content scales by this monitor's uiScale and the reserve
                // (edgeReserve) scales its band by the same factor, so the drawn
                // bar and its reserved exclusive-zone thickness stay matched.
                railScale: root.uiScale
                revealState: overlay.edgeReveal
                frameBars: overlay.frameBars
                style: ({ group: blobGroup })
                onMenuRequested: (id, ownerRect) => frameMenus.openSurface(id, ownerRect, "")
                onSurfaceRequested: (id, ownerRect) => frameMenus.openSurface(id, ownerRect, "")
                onActionRequested: id => root.runBarAction(id)
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
                railClearances: !root.barEnabled ? ({
                    top: 0, left: 0, bottom: 0, right: 0
                }) : root.sumiActive ? ({
                    top: overlay.railClearance("top"),
                    left: overlay.railClearance("left"),
                    bottom: overlay.railClearance("bottom"),
                    right: overlay.railClearance("right")
                }) : (overlay.qsBarEdge === "bottom"
                    ? ({ top: 0, left: 0, bottom: 52, right: 0 })
                    : ({ top: 52, left: 0, bottom: 0, right: 0 }))
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
                        frameMenus.openSurface(id, null, mon, ctx);
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

            RecordHud {
                id: recHud
                s: overlay.s
                clearanceTop: overlay.railClearance("top")
                clearanceBottom: overlay.railClearance("bottom")
                clearanceLeft: overlay.railClearance("left")
                clearanceRight: overlay.railClearance("right")
                laneDockEdge: root.dockLaneEdge
                laneDockSize: root.dockLaneSize
                laneDockCenter: root.dockLaneCenter
            }
        }
    }

    // Receipt-owned bar styles load from their durable content-addressed
    // views. Sumi is the built-in frame scene (the overlay above); any other
    // style loads its own scene here, without rails. The overlay and its menu
    // host still map (topBar mode) so menus and surfaces stay style-agnostic.
    Loader {
        id: barStyleLoader
        active: !root.sumiActive && (root.barEnabled || root.qsbarPrimaryHost)
        source: BarProducts.sceneUrl(Config.barStyle)
        onLoaded: {
            root.barStyleRetries = 0
            if (item) item.modelData = root.modelData
        }
        // A builtin style (qsbar) cannot be legitimately broken: a load error is a
        // transient hiccup (an update's config/plugin swap, a cold-start race), so
        // retry instead of permanently dropping to the sumi rail. A store-installed
        // style that errors is genuinely broken → fail() it as before.
        onStatusChanged: {
            if (status !== Loader.Error)
                return;
            if (BarProducts.isBuiltin(Config.barStyle) && root.barStyleRetries < 10) {
                root.barStyleRetries++;
                barStyleReload.restart();
            } else {
                BarProducts.fail(Config.barStyle);
            }
        }
    }
    Timer {
        id: barStyleReload
        interval: 800
        onTriggered: {
            barStyleLoader.active = false;
            barStyleLoader.active = Qt.binding(
                () => !root.sumiActive && (root.barEnabled || root.qsbarPrimaryHost));
        }
    }
    Connections {
        target: root
        function onModelDataChanged() {
            if (barStyleLoader.item)
                barStyleLoader.item.modelData = root.modelData;
        }
    }
}
