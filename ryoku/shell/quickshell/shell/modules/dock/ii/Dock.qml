import QtQuick
import QtQuick.Controls
import QtQuick.Effects
import QtQuick.Layouts
import Quickshell.Io
import Quickshell
import Quickshell.Widgets
import Quickshell.Wayland
import Ryoku.Ui.Singletons
import stage.modules.common.widgets
import stage as Stage
import shell.modules.stage.frame
import "."

pragma ComponentBehavior: Bound

Scope {
    id: dock

    property bool pinned: Config.options?.dock.pinnedOnStartup ?? true

    readonly property string dockEffectivePosition: {
        const pos = Config.options?.dock.position ?? "bottom"
        if (pos !== "auto") return pos
        return DockStore.sharedEdge()
    }

    readonly property bool isVertical: dockEffectivePosition === "left" || dockEffectivePosition === "right"

    function computeSizes(opts) {
        const isDynamic = opts.isDynamicIsland ?? false
        const isHug = opts.isHug ?? false
        const isFullWidth = opts.isFullWidth ?? false
        // The rounded full-width style draws two concave corners into the
        // screen beyond the tray's inner edge: the window keeps that room on
        // the cross axis, but the exclusive zone does not (windows reach the
        // tray; the corners round their bottom edge like a screen's).
        const concaveCrossPad = (opts.isFullWidthConcave ?? false) ? Math.max(0, opts.concaveCornerRadius || 0) : 0
        const isAttached = opts.isAttachedToEdge ?? false
        const gapsOut = opts.gapsOut
        const edgeGap = Math.max(0, Math.min(64, Number(opts.edgeGap || 0)))
        const shadowPad = Math.round(Appearance.sizes.elevationMargin * 1.2)

        const concaveReserve = isDynamic ? Math.max(0, opts.concaveCornerRadius || 0) : 0
        // A floating shadow is larger than gapsOut. Reserve its complete blur
        // envelope on both sides so the layer surface never becomes a clip wall.
        const floatingPad = isAttached ? 0 : shadowPad
        const mainPad = isDynamic ? (concaveReserve * 2) : (isHug ? (shadowPad * 2) : (floatingPad * 2))
        const crossPad = isAttached ? ((isHug || isFullWidth) ? shadowPad : 0) : (floatingPad * 2)

        // Full width is the screen's own edge: the window and the visible tray
        // both take the whole main axis, and the tray's body is the silhouette
        // with the concave corners. Nothing on the screen shortens that axis -
        // not the outer gaps, and not a bar on the other orientation, which the
        // panel simply draws over.
        const flushMainW = isFullWidth && !opts.isVertical
        const flushMainH = isFullWidth && opts.isVertical

        const barConflicts = opts.barActive && (opts.isVertical !== opts.barIsVertical)
        const barOffset = barConflicts ? (opts.isVertical ? opts.barThickness : 0) : 0
        const barOffsetH = barConflicts ? (!opts.isVertical ? opts.barThickness : 0) : 0

        const maxW = Math.max(1, opts.availableW - (isAttached ? 0 : gapsOut * 2) - (flushMainW ? 0 : barOffsetH))
        const maxH = Math.max(1, opts.availableH - (isAttached ? 0 : gapsOut * 2) - (flushMainH ? 0 : barOffset))

        const contentW = opts.contentVisualWidth + opts.dockPadding * 2
        const contentH = opts.contentVisualHeight + opts.dockPadding * 2
        const baseContentW = opts.baseVisualWidth + opts.dockPadding * 2
        const baseContentH = opts.baseVisualHeight + opts.dockPadding * 2
        const mainSafety = (opts.maxMainExtra || 0)
        const crossSafety = opts.maxCrossExtra || 0

        // The PanelWindow reserves a stable safe envelope. Only the visible
        // tray follows the actual animated content geometry.
        const trayCapW = maxW - (isDynamic ? 0 : (isHug ? shadowPad * 2 : floatingPad * 2))
        const trayCapH = maxH - (isDynamic ? 0 : (isHug ? shadowPad * 2 : floatingPad * 2))
        const bgW = Math.max(1, opts.isVertical ? baseContentW
            : flushMainW ? maxW
            : Math.min(contentW + (isDynamic ? concaveReserve * 2 : 0), trayCapW))
        const bgH = Math.max(1, opts.isVertical
            ? (flushMainH ? maxH : Math.min(contentH + (isDynamic ? concaveReserve * 2 : 0), trayCapH))
            : baseContentH)

        const baseDockW = opts.isVertical ? baseContentW + Math.max(crossSafety, concaveCrossPad) + crossPad + edgeGap : Math.min(baseContentW + mainSafety + mainPad, maxW)
        const baseDockH = opts.isVertical ? Math.min(baseContentH + mainSafety + mainPad, maxH) : Math.min(baseContentH + Math.max(crossSafety, concaveCrossPad) + crossPad + edgeGap, maxH)

        const fullDockW = Math.min(flushMainW ? maxW : baseDockW, maxW)
        const fullDockH = Math.min(flushMainH ? maxH : baseDockH, maxH)

        return {
            maxWidth: maxW,
            maxHeight: maxH,
            dockWidth: fullDockW,
            dockHeight: fullDockH,
            dockThickness: opts.isVertical ? fullDockW : fullDockH,
            unmagnifiedThickness: (opts.isVertical ? baseContentW + (isFullWidth ? 0 : crossPad) : baseContentH + (isFullWidth ? 0 : crossPad)) + edgeGap,
            surfaceMargin: floatingPad + edgeGap,
            // Main-axis room the tray keeps from the window edge for its shadow.
            mainEdgePad: isDynamic ? 0 : mainPad / 2,
            backgroundWidth: bgW,
            backgroundHeight: bgH
        }
    }

    // `dock.monitor` keeps the dock on one output; unset, or naming one that
    // is not connected, it is on every output.
    readonly property var dockScreens: {
        const wanted = String(Config.options?.dock?.monitor ?? "");
        const screens = Quickshell.screens;
        if (wanted.length === 0)
            return screens;
        const only = screens.filter(screen => screen.name === wanted);
        return only.length > 0 ? only : screens;
    }

    Variants {
        model: dock.dockScreens

        PanelWindow {
            id: dockRoot
            required property var modelData
            screen: modelData
            
            visible: !GlobalStates.lockLookActive && !positionChanging && !GlobalStates.oledSaverMonitors.includes(modelData.name) && !GlobalStates.isMediaModeActiveForScreen(modelData ? modelData.name : "")
            // using a flag for positionChanging is not really necessary, but it prevents some graphical issues caused by qml when the dock is moving

            // The main axis is anchored at both ends, so its length is the
            // compositor's, not the screen's: it already leaves out every
            // other exclusive zone (a bar on the other orientation, another
            // panel). Sizing it from the screen made the content longer than
            // its own window - centred, it slid down by half the excess, its
            // far end was cut, and the full-width styles' concave corners
            // fell outside the window at both ends.
            //
            // Read through handlers, not bindings: until the compositor
            // configures the surface its size is the implicit one, which is
            // computed from this - a binding loop on implicitWidth/sizing.
            property real _windowW: 0
            property real _windowH: 0
            property real _naturalW: 0
            property real _naturalH: 0
            // Deferred: before the configure the size follows implicitWidth
            // synchronously, inside the very evaluation that produced it.
            onWidthChanged: Qt.callLater(() => {
                dockRoot._windowW = dockRoot.width;
                // What the compositor gave before our own negative margin widened it.
                dockRoot._naturalW = dockRoot.width - Math.round(dockRoot.islandReclaim);
            })
            onHeightChanged: Qt.callLater(() => {
                dockRoot._windowH = dockRoot.height;
                dockRoot._naturalH = dockRoot.height - Math.round(dockRoot.islandReclaim);
            })
            readonly property bool mainAxisFromWindow: dock.isVertical ? dockRoot._windowH > 1 : dockRoot._windowW > 1
            readonly property real availableW: (!dock.isVertical && dockRoot._windowW > 1) ? dockRoot._windowW : (screen?.width ?? 1920)
            readonly property real availableH: (dock.isVertical && dockRoot._windowH > 1) ? dockRoot._windowH : (screen?.height ?? 1080)
            readonly property bool barActive: GlobalStates.barOpen
            readonly property bool barIsVertical: DockStore.sharedVertical()
            readonly property real barThickness: barActive? (barIsVertical ? (DockStore.sharedBarWidth()) : (DockStore.sharedBarHeight())) : 0

            readonly property bool enableMagnification: Config.options?.dock?.enableMagnification ?? false
            readonly property real magnificationScale: Config.options?.dock?.magnificationScale ?? 1.5
            // Safe interaction/render reserve; the visual background follows
            // dockContent.visualWidth/visualHeight independently.
            // The pointer-anchored lens can grow the whole extra on one side,
            // so reserve it on both.
            readonly property real magExtra: enableMagnification
                ? dockContent.maximumMagnificationExtra * (dockContent.magnificationDynamicSpacing ? 2 : 1)
                : 0
            readonly property real magCrossExtra: enableMagnification ? dockContent.maximumMagnificationCrossExtra : 0

            readonly property bool isVertical: dock.isVertical
            readonly property real dockThickness: dockRoot.sizing.dockThickness
            readonly property real unmagnifiedThickness: dockRoot.sizing.unmagnifiedThickness
            readonly property real surfaceMargin: dockRoot.sizing.surfaceMargin

            // The edge this dock takes, from the thickness it reveals at rest - configuration-
            // derived, unchanged by hover, magnification or a fullscreen window - for desktop
            // layouts that keep clear of it; published rather than re-derived, because the
            // padding and the style that make up this number live here.
            QtObject {
                id: editInsetPublisher
                readonly property string screenName: dockRoot.screen ? dockRoot.screen.name : ""
                readonly property string side: dock.dockEffectivePosition
                readonly property real thickness: dockRoot.unmagnifiedThickness
                readonly property real reserved: dockRoot.exclusiveZone

                function publish() {
                    GlobalStates.setDockInset(editInsetPublisher.screenName, editInsetPublisher.side, editInsetPublisher.thickness, editInsetPublisher.reserved);
                }

                onSideChanged: publish()
                onThicknessChanged: publish()
                onReservedChanged: publish()
                Component.onCompleted: publish()
                Component.onDestruction: GlobalStates.setDockInset(editInsetPublisher.screenName, "", 0)
            }

            readonly property bool anySidebarOpen: GlobalStates.effectiveLeftOpen || GlobalStates.effectiveRightOpen

            readonly property var activeOutputWorkspaces: {
                const screenName = String(dockRoot.screen?.name ?? "");
                return (Wm.workspaces ?? []).filter(workspace =>
                    workspace.active === true
                        && (String(workspace.output ?? "") === screenName
                            || (String(workspace.output ?? "") === ""
                                && screenName === String(Wm.focusedOutput ?? ""))));
            }
            readonly property bool isSpecialWorkspaceOpen:
                dockRoot.activeOutputWorkspaces.some(workspace => workspace.special === true)

            // ── Fullscreen & hover detection ─────────────────────────────────────
            readonly property bool hasFullscreenWindow: {
                if (dockRoot.activeOutputWorkspaces.some(workspace => workspace.fullscreen === true))
                    return true;
                return Wm.outputHasFullscreen(String(dockRoot.screen?.name ?? ""));
            }

            readonly property bool blockHoverInFullscreen: Config.options?.dock?.blockHoverInFullscreen ?? true
            readonly property bool hoverBlocked: blockHoverInFullscreen && hasFullscreenWindow
            readonly property bool effectiveHoverToReveal: (Config.options?.dock?.hoverToReveal ?? true) && !hoverBlocked

            // Edit Mode holds the dock revealed: the editor's card draws it in place and the
            // dock catalogue edits it there. A preset switch takes it off screen with the bar:
            // the preset may move it to another edge, and it would make that move in full view
            // while the shell is busy applying the rest.
            // Real Gnome's overview puts a workspace plane where the dock sits; on an
            // empty workspace the dock would cover its bottom, so it steps aside.
            readonly property var overviewController: GlobalStates.overviewBackgroundControllerFor(dockRoot.screen?.name ?? "")
            readonly property bool realGnomeOverviewOpen: GlobalStates.overviewOpen && !GlobalStates.editMode
                && !!dockRoot.overviewController && dockRoot.overviewController.isRealGnome
            property bool reveal: !GlobalStates.presetBarHidden
                && !dockRoot.realGnomeOverviewOpen
                && ((dock.pinned && !dockRoot.hoverBlocked)
                    || GlobalStates.editMode
                    || DockPresets.isSwitchingPreset
                    || (!anySidebarOpen
                        && ((dockRoot.effectiveHoverToReveal && dockMouseArea.containsMouse)
                            || dockContent.requestDockShow
                            || (workspaceEmpty && !isSpecialWorkspaceOpen
                                && (!(Config.options?.dock.showOnlyOnFocusedMonitor ?? false)
                                    || isFocusedMonitor)))))
            property bool positionChanging: false

            readonly property bool workspaceEmpty:
                !dockRoot.activeOutputWorkspaces.some(workspace =>
                    workspace.occupied === true || Number(workspace.windows ?? 0) > 0)

            readonly property bool isFocusedMonitor: {
                return (Wm.focusedOutput ?? "") === (dockRoot.screen ? dockRoot.screen.name : "")
            }

            readonly property bool isDynamicIsland: dockContent.isDynamicIsland
            readonly property bool isHug: dockContent.isHug
            readonly property bool isTransparent: dockContent.isTransparent
            readonly property bool isFullWidth: dockContent.isFullWidth
            readonly property bool isFullWidthConcave: dockContent.isFullWidthConcave
            readonly property bool isAttachedToEdge: dockContent.isAttachedToEdge
            // The radius is capped by the dock's thickness, and the thickness comes out
            // of computeSizes() — which also takes the radius. Thickness never depends
            // on the radius (it only pads the main axis), so the cap reads a
            // radius-free pass instead of `sizing`, breaking the binding loop.
            readonly property real radiusFreeThickness: dock.computeSizes(dockRoot.sizingInputs(0)).dockThickness
            readonly property real concaveCornerRadius: {
                if ((Config.options?.dock?.dockRadius ?? -1) >= 0) {
                    return Math.min(Config.options.dock.dockRadius, dockRoot.radiusFreeThickness * 0.8)
                }
                return Math.min(Appearance.rounding.large, dockRoot.radiusFreeThickness * 0.8)
            }
            readonly property var sizing: dock.computeSizes(dockRoot.sizingInputs(dockRoot.concaveCornerRadius))
            function sizingInputs(concaveCornerRadius) {
                return {
                    gapsOut: Appearance.sizes.windowGapsOut,
                    edgeGap: Config.options?.dock?.edgeGap ?? 0,
                    isDynamicIsland: dockRoot.isDynamicIsland,
                    isHug: dockRoot.isHug,
                    isFullWidth: dockRoot.isFullWidth,
                    isFullWidthConcave: dockRoot.isFullWidthConcave,
                    isAttachedToEdge: dockRoot.isAttachedToEdge,
                    concaveCornerRadius: concaveCornerRadius,
                    isVertical: dock.isVertical,
                    // The window's own length already excludes the bar.
                    barActive: dockRoot.barActive && !dockRoot.mainAxisFromWindow,
                    barIsVertical: dockRoot.barIsVertical,
                    barThickness: dockRoot.barThickness,
                    availableW: dockRoot.availableW,
                    availableH: dockRoot.availableH,
                    contentVisualWidth: dockContent.visualWidth,
                    contentVisualHeight: dockContent.visualHeight,
                    baseVisualWidth: dockContent.baseVisualWidth,
                    baseVisualHeight: dockContent.baseVisualHeight,
                    dockPadding: dockContent.dockPadding,
                    maxMainExtra: dockRoot.magExtra,
                    maxCrossExtra: dockRoot.magCrossExtra
                }
            }

            // ── Under a Dynamic Island bar ───────────────────────────────────────
            // That bar style reserves a whole strip but draws only a pill centred in
            // it, and the compositor shortens a full-width dock to what the strip
            // leaves - an empty column of wallpaper beside the dock, below the pill.
            // Where the pill (fillets included) ends clear of the dock's band, the
            // dock takes the strip back and runs on to the wrapped frame's ring, or
            // to the screen edge without one. Where they would meet it stays
            // beside the bar, as before.
            readonly property var barIsland: GlobalStates.barIslands[dockRoot.screen?.name ?? ""] ?? null
            readonly property real frameRing: Config.options?.appearance?.fakeScreenRounding === 3 ? (Config.options?.appearance?.wrappedFrameThickness ?? 0) : 0
            readonly property real islandReclaim: {
                const island = dockRoot.barIsland;
                if (!dockRoot.isFullWidth || !island || !(island.reserve > 0))
                    return 0;
                // Only a bar lying across the dock's axis leaves it a stretch to take.
                const barIsHorizontal = island.edge === "top" || island.edge === "bottom";
                if (barIsHorizontal !== dock.isVertical)
                    return 0;
                // The pill is centred across the dock's own cross axis.
                const crossLen = dock.isVertical ? (dockRoot.screen?.width ?? 1920) : (dockRoot.screen?.height ?? 1080);
                const mainLen = dock.isVertical ? (dockRoot.screen?.height ?? 1080) : (dockRoot.screen?.width ?? 1920);
                const atFar = dock.dockEffectivePosition === "bottom" || dock.dockEffectivePosition === "right";
                // A pinned dock shortens the bar's window from its own side, which
                // moves the pill's centre away from the dock by half the zone.
                const pinZone = (dock.pinned && dockRoot.reveal) ? dockRoot.unmagnifiedThickness : 0;
                const centre = crossLen / 2 + (atFar ? -pinZone : pinZone) / 2;
                // The band the dock fills across the screen: its tray plus the room its
                // concave corners keep, inside the frame's ring.
                const band = dockRoot.frameRing + dockRoot.radiusFreeThickness + dockRoot.concaveCornerRadius + 8;
                const clear = atFar
                    ? (centre + island.extent / 2 <= crossLen - band)
                    : (centre - island.extent / 2 >= band);
                if (!clear)
                    return 0;
                // Take back only what the compositor really withheld: a pinned dock
                // is arranged ahead of the bar's zone and can already span the screen,
                // where a negative margin would push it past the edge. Until the first
                // configure, assume the usual case - the strip is withheld.
                const natural = dock.isVertical ? dockRoot._naturalH : dockRoot._naturalW;
                if (!(natural > 1))
                    return island.reserve;
                const full = mainLen - dockRoot.frameRing * 2;
                return Math.max(0, Math.min(island.reserve, Math.round(full - natural)));
            }

            implicitWidth: dock.isVertical ? frameLayer.extent : Math.max(1, dockRoot.sizing.dockWidth)
            implicitHeight: dock.isVertical ? Math.max(1, dockRoot.sizing.dockHeight) : frameLayer.extent

            anchors {
                top: dock.dockEffectivePosition !== "bottom"
                bottom: dock.dockEffectivePosition !== "top"
                left: dock.dockEffectivePosition !== "right"
                right: dock.dockEffectivePosition !== "left"
            }

            // A negative margin gives back the strip the compositor took for the bar.
            margins {
                left: dockRoot.barIsland?.edge === "left" ? -Math.round(dockRoot.islandReclaim) : 0
                right: dockRoot.barIsland?.edge === "right" ? -Math.round(dockRoot.islandReclaim) : 0
                top: dockRoot.barIsland?.edge === "top" ? -Math.round(dockRoot.islandReclaim) : 0
                bottom: dockRoot.barIsland?.edge === "bottom" ? -Math.round(dockRoot.islandReclaim) : 0
            }

            // Expose the raw window geometry for global gesture tracking,
            // independent of any interactive child item bounds.
            readonly property real actualWindowWidth: width
            readonly property real actualWindowHeight: height

            exclusiveZone: (dock.pinned && reveal) ? unmagnifiedThickness : 0
            WlrLayershell.namespace: "quickshell:dock"
            // Pinned docks remain part of the desktop. An auto-hidden dock must
            // own its edge above app windows or neither its reveal strip nor the
            // revealed tray can receive the pointer.
            WlrLayershell.layer: (GlobalStates.editMode || GlobalStates.editProgress > 0)
                ? WlrLayer.Overlay : (dock.pinned ? WlrLayer.Bottom : WlrLayer.Top)
            color: "transparent"
            readonly property bool popupOnThisScreen: {
                const popup = Stage.GlobalStates.iiDockPopup;
                const popupScreen = popup?.anchorItem?.QsWindow?.window?.screen?.name ?? "";
                return popup !== null && popup.isClosing !== true
                    && String(popupScreen) === String(dockRoot.screen?.name ?? "");
            }
            // Exclusive is pointer-transparent only when the neutral capability says so.
            WlrLayershell.keyboardFocus: !dockRoot.popupOnThisScreen ? WlrKeyboardFocus.None
                : Wm.caps.keyboardGrabSharesPointer === true ? WlrKeyboardFocus.Exclusive
                : WlrKeyboardFocus.OnDemand
            Item {
                id: popupKeySink
                anchors.fill: parent
                z: -1000
                focus: dockRoot.popupOnThisScreen
                Keys.onEscapePressed: event => {
                    event.accepted = true;
                    Stage.GlobalStates.dismissIiDockPopup();
                }
            }
            onPopupOnThisScreenChanged: {
                if (dockRoot.popupOnThisScreen)
                    Qt.callLater(() => popupKeySink.forceActiveFocus(Qt.PopupFocusReason));
            }

            // The window keeps room beyond the tray (lens headroom, shadow,
            // the concave corners) that windows now reach under, since the
            // exclusive zone is the tray alone. Taking input over all of it
            // made a band above the dock eat clicks meant for those windows.
            // So input is the tray; the whole envelope only while it is in
            // use: hidden (the reveal strip lives there), dragging, or the
            // pointer on a magnifying dock, whose enlarged icons rise into it.
            //
            // A dock revealed by hover keeps the whole envelope while the
            // pointer is in it: shrinking to the tray as it appears put a
            // pointer resting on the screen edge (below a floating tray, in
            // its margin) outside the hover area - the dock hid, the reveal
            // strip took the pointer again, and it showed: a loop at that spot.
            readonly property bool fullInputMask: !dockRoot.reveal
                || dockContent.dragging
                || (dockMouseArea.containsMouse && (dockRoot.enableMagnification || !dock.pinned))
            readonly property bool inputActive: dockRoot.reveal || dockRoot.effectiveHoverToReveal
            readonly property Item inputItem: dockRoot.fullInputMask ? dockMouseArea : trayInputArea
            readonly property rect inputRect: frameLayer.maskRect(dockRoot.inputItem.x, dockRoot.inputItem.y,
                dockRoot.inputItem.width, dockRoot.inputItem.height)
            mask: Region {
                x: Math.round(dockRoot.inputRect.x)
                y: Math.round(dockRoot.inputRect.y)
                width: dockRoot.inputActive ? Math.round(dockRoot.inputRect.width) : 0
                height: dockRoot.inputActive ? Math.round(dockRoot.inputRect.height) : 0
            }

            // The dock is part of the desktop the Stage Editor shrinks into its
            // card: while it frames this monitor the window grows across the
            // screen and this layer draws the dock where the card puts it.
            StageFramedLayer {
                id: frameLayer
                screenName: dockRoot.screen?.name ?? ""
                edge: dock.dockEffectivePosition
                restThickness: Math.max(1, dock.isVertical ? dockRoot.sizing.dockWidth : dockRoot.sizing.dockHeight)
                screenWidth: dockRoot.screen?.width ?? 1920
                screenHeight: dockRoot.screen?.height ?? 1080
                originX: dockRoot.anchors.left ? dockRoot.margins.left : screenWidth - dockRoot.width - dockRoot.margins.right
                originY: dockRoot.anchors.top ? dockRoot.margins.top : screenHeight - dockRoot.height - dockRoot.margins.bottom

                // The tray in window coordinates (dockMouseArea › dockSurfaceHost,
                // which fills it, › dockVisualBackground), following its slides.
                Item {
                    id: trayInputArea
                    x: dockMouseArea.x + dockVisualBackground.x
                    y: dockMouseArea.y + dockVisualBackground.y
                    width: dockVisualBackground.width
                    height: dockVisualBackground.height
                }

                Timer {
                    id: positionChangeTimer
                    interval: 200
                    onTriggered: dockRoot.positionChanging = false
                }

                Connections {
                    target: Config.options.dock
                    function onPositionChanged() {
                        dockRoot.positionChanging = true
                        positionChangeTimer.restart()
                    }
                }

                FocusGrab {
                    id: dragFocusGrab
                    active: dockContent.dragging
                    windows: [dockRoot]
                    onCleared: {
                        dockContent.cancelDrag()
                    }
                }

                MouseArea {
                    id: dockMouseArea
                    hoverEnabled: dockRoot.reveal || dockRoot.effectiveHoverToReveal

                    property real hoverRegion: Config.options?.dock?.hoverRegionHeight ?? 2
                    property real hiddenOffset: dockRoot.dockThickness - hoverRegion
                    property real fullyHiddenOffset: dockRoot.dockThickness + 1
                    property real currentOffset: dockRoot.reveal ? 0 : (dockRoot.effectiveHoverToReveal ? hiddenOffset : fullyHiddenOffset)
                    property real dipOffset: (dockRoot.dockThickness + 30) * DockPresets.dipProgress

                    width: dock.isVertical ? dockRoot.dockThickness : dockRoot.sizing.dockWidth
                    height: dock.isVertical ? dockRoot.sizing.dockHeight : dockRoot.dockThickness

                    state: dock.dockEffectivePosition

                    states: [
                        State {
                            name: "top"
                            AnchorChanges { target: dockMouseArea; anchors.top: parent.top; anchors.horizontalCenter: parent.horizontalCenter }
                            PropertyChanges { target: dockMouseArea; anchors.topMargin: -(currentOffset + dockMouseArea.dipOffset) }
                        },
                        State {
                            name: "bottom"
                            AnchorChanges { target: dockMouseArea; anchors.bottom: parent.bottom; anchors.horizontalCenter: parent.horizontalCenter }
                            PropertyChanges { target: dockMouseArea; anchors.bottomMargin: -(currentOffset + dockMouseArea.dipOffset) }
                        },
                        State {
                            name: "left"
                            AnchorChanges { target: dockMouseArea; anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter }
                            PropertyChanges { target: dockMouseArea; anchors.leftMargin: -(currentOffset + dockMouseArea.dipOffset) }
                        },
                        State {
                            name: "right"
                            AnchorChanges { target: dockMouseArea; anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter }
                            PropertyChanges { target: dockMouseArea; anchors.rightMargin: -(currentOffset + dockMouseArea.dipOffset) }
                        }
                    ]

                    Behavior on anchors.topMargin {
                        enabled: !DockPresets.isSwitchingPreset
                        animation: Appearance.animation.elementMoveFast.numberAnimation.createObject(dockMouseArea)
                    }
                    Behavior on anchors.bottomMargin {
                        enabled: !DockPresets.isSwitchingPreset
                        animation: Appearance.animation.elementMoveFast.numberAnimation.createObject(dockMouseArea)
                    }
                    Behavior on anchors.leftMargin {
                        enabled: !DockPresets.isSwitchingPreset
                        animation: Appearance.animation.elementMoveFast.numberAnimation.createObject(dockMouseArea)
                    }
                    Behavior on anchors.rightMargin {
                        enabled: !DockPresets.isSwitchingPreset
                        animation: Appearance.animation.elementMoveFast.numberAnimation.createObject(dockMouseArea)
                    }

                    // Stable safe bounds feed one continuous magnification field,
                    // including the overflow area above/next to enlarged icons.
                    HoverHandler {
                        id: magnificationHover
                        blocking: false
                        acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
                        enabled: !DockPresets.magnificationSuspended

                        onPointChanged: {
                            const position = point.position
                            dockContent.updateMagnificationPointerFrom(dockMouseArea, position.x, position.y)
                        }
                        onHoveredChanged: dockContent.setMagnificationHovered(hovered)
                    }

                    Connections {
                        target: DockPresets
                        function onMagnificationSuspendedChanged() {
                            if (DockPresets.magnificationSuspended) {
                                dockContent.setMagnificationHovered(false);
                                dockContent.resetMagnificationImmediate();
                            } else if (magnificationHover.hovered) {
                                dockContent.updateMagnificationPointerFrom(dockMouseArea, magnificationHover.point.position.x, magnificationHover.point.position.y);
                                dockContent.setMagnificationHovered(true);
                            }
                        }
                    }

                    WheelHandler {
                        id: dockWheelPresetSwitcher
                        target: null
                        acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
                        onWheel: event => {
                            if (DockPresets.canSwitchPresets) {
                                if (DockPresets.handleWheelScroll(event.angleDelta.y, event.angleDelta.x)) {
                                    event.accepted = true;
                                }
                            }
                        }
                    }

                    // Neutral host: the classic surface, DropArea and content are
                    // siblings so hiding the global rectangle never hides items.
                    Item {
                        id: dockSurfaceHost
                        anchors.fill: parent
                        clip: false

                        // A layer shadow re-allocates its texture on every frame
                        // the lens resizes the tray; this one is drawn directly.
                        StyledRectangularShadow {
                            target: dockVisualBackground
                            cached: false
                            blur: Appearance.sizes.elevationMargin
                            spread: 0
                            color: Qt.rgba(0, 0, 0, 0.35)
                            offset: {
                                if (dock.dockEffectivePosition === "left") return Qt.vector2d(2, 0);
                                if (dock.dockEffectivePosition === "right") return Qt.vector2d(-2, 0);
                                if (dock.dockEffectivePosition === "top") return Qt.vector2d(0, 2);
                                return Qt.vector2d(0, -2);
                            }
                            visible: !dockContent.islandsStyle && !dockRoot.isDynamicIsland && !dockRoot.isTransparent
                                && opacity > 0.01
                                && !Config.options.appearance.transparency.popups
                                && !Config.options.appearance.transparency.enable
                        }

                        Rectangle {
                            id: dockVisualBackground
                            clip: false

                            width: Math.max(1, Math.min(
                                dockRoot.sizing.backgroundWidth,
                                dockRoot.sizing.dockWidth
                            ))
                            height: Math.max(1, Math.min(
                                dockRoot.sizing.backgroundHeight,
                                dockRoot.sizing.dockHeight
                            ))

                            // The tray's own silhouette: square when the panel is
                            // attached to a screen edge - the screen's edge is the
                            // corner there - or when the body is not this rectangle
                            // at all (dynamic island, full width).
                            readonly property real trayCornerRadius: (dockRoot.isDynamicIsland || dockRoot.isHug || dockRoot.isFullWidth) ? 0 : dockContent.dockCornerRadius

                            color: (dockRoot.isDynamicIsland || dockRoot.isTransparent) ? "transparent" : Appearance.colors.colLayer0
                            radius: trayCornerRadius
                            topLeftRadius: dockRoot.isHug ? ((dock.dockEffectivePosition === "bottom" || dock.dockEffectivePosition === "right") ? dockContent.dockCornerRadius : 0) : trayCornerRadius
                            topRightRadius: dockRoot.isHug ? ((dock.dockEffectivePosition === "bottom" || dock.dockEffectivePosition === "left") ? dockContent.dockCornerRadius : 0) : trayCornerRadius
                            bottomLeftRadius: dockRoot.isHug ? ((dock.dockEffectivePosition === "top" || dock.dockEffectivePosition === "right") ? dockContent.dockCornerRadius : 0) : trayCornerRadius
                            bottomRightRadius: dockRoot.isHug ? ((dock.dockEffectivePosition === "top" || dock.dockEffectivePosition === "left") ? dockContent.dockCornerRadius : 0) : trayCornerRadius

                            opacity: (dockContent.islandsStyle || dockRoot.isTransparent) ? 0.0 : 1.0

                            // Keeps the point under the cursor fixed while the lens
                            // grows; clamped to the room the window reserves.
                            readonly property real lensShift: {
                                if (dockRoot.isDynamicIsland)
                                    return 0;
                                const room = dock.isVertical
                                    ? (dockRoot.sizing.dockHeight - height) / 2
                                    : (dockRoot.sizing.dockWidth - width) / 2;
                                const limit = Math.max(0, room - dockRoot.sizing.mainEdgePad);
                                return Math.max(-limit, Math.min(limit, dockContent.magnificationCenterShift));
                            }
                            anchors.horizontalCenterOffset: dock.isVertical ? 0 : lensShift
                            anchors.verticalCenterOffset: dock.isVertical ? lensShift : 0

                            // Clear old anchors before installing the new edge. Conditional
                            // anchors can overlap during a preset change, stretch the tray
                            // and permanently remove its width/height bindings.
                            state: dock.dockEffectivePosition
                            states: [
                                State {
                                    name: "top"
                                    AnchorChanges { target: dockVisualBackground; anchors.top: parent.top; anchors.horizontalCenter: parent.horizontalCenter }
                                },
                                State {
                                    name: "bottom"
                                    AnchorChanges { target: dockVisualBackground; anchors.bottom: parent.bottom; anchors.horizontalCenter: parent.horizontalCenter }
                                },
                                State {
                                    name: "left"
                                    AnchorChanges { target: dockVisualBackground; anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter }
                                },
                                State {
                                    name: "right"
                                    AnchorChanges { target: dockVisualBackground; anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter }
                                }
                            ]

                            anchors.bottomMargin: dock.dockEffectivePosition === "bottom" ? (dockRoot.reveal ? dockRoot.surfaceMargin : -(dockMouseArea.hoverRegion + 4)) : 0

                            anchors.topMargin: dock.dockEffectivePosition === "top" ? (dockRoot.reveal ? dockRoot.surfaceMargin : -(dockMouseArea.hoverRegion + 4)) : 0

                            anchors.leftMargin: dock.dockEffectivePosition === "left" ? (dockRoot.reveal ? dockRoot.surfaceMargin : -(dockMouseArea.hoverRegion + 4)) : 0

                            anchors.rightMargin: dock.dockEffectivePosition === "right" ? (dockRoot.reveal ? dockRoot.surfaceMargin : -(dockMouseArea.hoverRegion + 4)) : 0

                            Behavior on opacity {
                                animation: Appearance.animation.elementMoveFast.numberAnimation.createObject(dockVisualBackground)
                            }
                            Behavior on anchors.bottomMargin { animation: Appearance.animation.elementMoveFast.numberAnimation.createObject(dockVisualBackground) }
                            Behavior on anchors.topMargin { animation: Appearance.animation.elementMoveFast.numberAnimation.createObject(dockVisualBackground) }
                            Behavior on anchors.leftMargin { animation: Appearance.animation.elementMoveFast.numberAnimation.createObject(dockVisualBackground) }
                            Behavior on anchors.rightMargin { animation: Appearance.animation.elementMoveFast.numberAnimation.createObject(dockVisualBackground) }

                            Notch {
                                id: dynamicIslandNotch
                                visible: dockRoot.isDynamicIsland && !dockContent.islandsStyle && !dock.isVertical
                                anchors.fill: parent
                                bodyWidth: parent.width
                                bodyHeight: parent.height
                                disableBehaviors: true
                                topRadius: dockRoot.concaveCornerRadius
                                bottomRadius: Math.min(dockContent.dockCornerRadius, parent.height)
                                fillColor: Appearance.colors.colLayer0

                                transform: Scale {
                                    xScale: 1
                                    yScale: dock.dockEffectivePosition === "bottom" ? -1 : 1
                                    origin.y: dynamicIslandNotch.height / 2
                                }
                            }

                            // Full width · rounded: the tray spans the screen and
                            // curves into it at both sides, the way the hug bar's
                            // screen corners do — the screen above looks rounded.
                            Repeater {
                                model: (dockRoot.isFullWidthConcave && !dockContent.islandsStyle) ? 2 : 0
                                delegate: RoundCorner {
                                    required property int index
                                    readonly property string pos: dock.dockEffectivePosition
                                    readonly property bool startEnd: index === 0
                                    visible: opacity > 0.01
                                    opacity: dockVisualBackground.opacity
                                    implicitSize: Math.max(1, dockRoot.concaveCornerRadius)
                                    color: dockVisualBackground.color
                                    // The filled side of each corner faces the tray
                                    // and the screen edge it meets.
                                    corner: pos === "top" ? (startEnd ? RoundCorner.CornerEnum.TopLeft : RoundCorner.CornerEnum.TopRight)
                                        : pos === "left" ? (startEnd ? RoundCorner.CornerEnum.TopLeft : RoundCorner.CornerEnum.BottomLeft)
                                        : pos === "right" ? (startEnd ? RoundCorner.CornerEnum.TopRight : RoundCorner.CornerEnum.BottomRight)
                                        : (startEnd ? RoundCorner.CornerEnum.BottomLeft : RoundCorner.CornerEnum.BottomRight)
                                    // A 1 px tuck hides an antialiasing gap on
                                    // an opaque tray; on a translucent one the
                                    // overlap paints twice and reads as a line.
                                    readonly property real tuck: dockVisualBackground.color.a < 1 ? 0 : 1
                                    x: pos === "left" ? parent.width - tuck
                                        : pos === "right" ? -width + tuck
                                        : (startEnd ? 0 : parent.width - width)
                                    y: pos === "bottom" ? -height + tuck
                                        : pos === "top" ? parent.height - tuck
                                        : (startEnd ? 0 : parent.height - height)
                                }
                            }

                            RoundCorner {
                                id: concaveCorner1
                                visible: dockRoot.isDynamicIsland && !dockContent.islandsStyle && dock.isVertical && opacity > 0.01
                                opacity: dockVisualBackground.opacity
                                implicitSize: Math.max(1, dockRoot.concaveCornerRadius)
                                color: dockVisualBackground.color
                                corner: {
                                    if (dock.dockEffectivePosition === "left") return RoundCorner.CornerEnum.BottomLeft;
                                    return RoundCorner.CornerEnum.BottomRight;
                                }
                                anchors {
                                    bottom: (dock.dockEffectivePosition === "left" || dock.dockEffectivePosition === "right") ? parent.top : undefined
                                    bottomMargin: (dock.dockEffectivePosition === "left" || dock.dockEffectivePosition === "right") ? -1 : 0
                                    right: (dock.dockEffectivePosition === "right") ? parent.right : undefined
                                    left: (dock.dockEffectivePosition === "left") ? parent.left : undefined
                                }
                            }

                            RoundCorner {
                                id: concaveCorner2
                                visible: dockRoot.isDynamicIsland && !dockContent.islandsStyle && dock.isVertical && opacity > 0.01
                                opacity: dockVisualBackground.opacity
                                implicitSize: Math.max(1, dockRoot.concaveCornerRadius)
                                color: dockVisualBackground.color
                                corner: {
                                    if (dock.dockEffectivePosition === "left") return RoundCorner.CornerEnum.TopLeft;
                                    return RoundCorner.CornerEnum.TopRight;
                                }
                                anchors {
                                    top: (dock.dockEffectivePosition === "left" || dock.dockEffectivePosition === "right") ? parent.bottom : undefined
                                    topMargin: (dock.dockEffectivePosition === "left" || dock.dockEffectivePosition === "right") ? -1 : 0
                                    left: (dock.dockEffectivePosition === "left") ? parent.left : undefined
                                    right: (dock.dockEffectivePosition === "right") ? parent.right : undefined
                                }
                            }
                        }

                        // Under the content (z 1): a drag is offered to the topmost
                        // DropArea that takes it, so widgets that take files (shelf,
                        // send) get them first, and everywhere else falls through
                        // to pinning here. Above the content it swallowed every drop.
                        DropArea {
                            id: fileDropArea
                            anchors.fill: parent
                            z: 0
                            keys: ["text/uri-list"]

                            // We delay the re-enablement slightly after an internal drag ends
                            // to prevent the "exited" event from firing for the internal drag.
                            property bool blockDueToInternal: dockContent.dragging
                            onBlockDueToInternalChanged: {
                                if (!blockDueToInternal) {
                                    reEnableTimer.restart()
                                } else {
                                    enabled = false
                                }
                            }

                            Timer {
                                id: reEnableTimer
                                interval: 50
                                onTriggered: fileDropArea.enabled = true
                            }

                            onEntered: (drag) => {
                                if (!drag.hasUrls) return
                                //console.log("[Dock] External drag entered")
                                const url = drag.urls[0]?.toString() ?? ""
                                dockContent.externalDragIcon = dockContent.mimeIconFromPath(url)
                                dockContent.externalDragOver = true
                            }
                            onExited: {
                                //console.log("[Dock] External drag exited")
                                dockContent.externalDragIcon = ""
                                dockContent.externalDragOver = false
                            }
                            onDropped: (drop) => {
                                if (!drop.hasUrls) return
                                //console.log("[Dock] External drag dropped")
                                for (let i = 0; i < drop.urls.length; i++)
                                    TaskbarApps.addPinnedFile(drop.urls[i])
                                drop.accept(Qt.CopyAction)
                                dockContent.externalDragIcon = ""
                                dockContent.externalDragOver = false
                            }
                        }

                        // A right-click on the dock's own body - between its icons,
                        // on its padding - offers the desktop's menu told it is on
                        // the dock. Under the content, so every icon's own menu
                        // still wins; the menu's surface is the whole screen, so
                        // the point is lifted from this window to it the way the
                        // bar lifts its own.
                        MouseArea {
                            anchors.fill: dockVisualBackground
                            z: 0
                            acceptedButtons: Qt.RightButton
                            onClicked: mouse => {
                                if (!dockRoot.screen)
                                    return;
                                const p = mapToItem(null, mouse.x, mouse.y);
                                const side = dock.dockEffectivePosition;
                                const offsetX = side === "right" ? dockRoot.screen.width - dockRoot.width : 0;
                                const offsetY = side === "bottom" ? dockRoot.screen.height - dockRoot.height : 0;
                                GlobalStates.openDesktopMenu(dockRoot.screen.name, p.x + offsetX, p.y + offsetY, "dock");
                            }
                        }

                        DockContent {
                            id: dockContent
                            anchors.fill: dockVisualBackground
                            anchors.leftMargin: (dockRoot.isDynamicIsland && !dock.isVertical) ? dockRoot.concaveCornerRadius : 0
                            anchors.rightMargin: (dockRoot.isDynamicIsland && !dock.isVertical) ? dockRoot.concaveCornerRadius : 0
                            anchors.topMargin: (dockRoot.isDynamicIsland && dock.isVertical) ? dockRoot.concaveCornerRadius : 0
                            anchors.bottomMargin: (dockRoot.isDynamicIsland && dock.isVertical) ? dockRoot.concaveCornerRadius : 0
                            z: 1
                            isPinned: dock.pinned
                            currentScreen: dockRoot.screen
                            dockRevealed: dockRoot.reveal
                            dockWindowVisible: dockRoot.visible
                            onTogglePinRequested:
                                Config.options.dock.pinnedOnStartup = !dock.pinned
                        }
                    }
                }
            }
        }
    }
}
