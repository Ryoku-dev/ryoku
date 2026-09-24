//@ pragma UseQApplication

import QtQuick
import Quickshell
import Ryoku.Ui.Singletons
import Quickshell.Io
import "../../shared/Singletons"
import "../../shared/providers" as SharedProviders
import "../../shared/lib/lifecycle.js" as Lifecycle
import "." as HeroVariant
import "../../../../services/lib/screens.js" as Screens
import Ryoku.Ui.Singletons as Ui

Scope {
    id: root

    property var lifecycleState: Lifecycle.initialState()
    property bool openRequested: false
    property string requestedMonitor: ""
    property var activeLauncher: null
    property var activeSurface: null
    property var railSurfaces: ({})
    property int railRevision: 0
    property string recoveryTarget: ""
    property bool pointerFocusForced: false
    property string savedFollowMouse: ""
    property bool restorePending: false
    property var lifetime: ({ alive: true })

    readonly property bool open: openRequested
    readonly property bool shown:
        lifecycleState.phase !== Lifecycle.PHASES.CLOSED
    readonly property string openMon: String(lifecycleState.monitor || "")

    onOpenChanged: {
        // Phase 10: the launcher-open daemon report (execDetached ryoku-shell state
        // launcher) is dropped; ShellState.launcherOpen owns open state now.
        if (open) {
            freezePointerFocus();
        } else {
            restorePointerFocus();
        }
    }

    Binding {
        target: Weather
        property: "unitOverride"
        value: LauncherConfig.weatherUnit === "auto"
            ? "" : LauncherConfig.weatherUnit
    }

    function focusedMonitor() {
        if (Wm.focusedOutput !== "")
            return Wm.focusedOutput;
        return Quickshell.screens.length > 0
            ? Quickshell.screens[0].name : "";
    }

    function screenForName(name) {
        var target = String(name || "");
        for (var index = 0; index < Quickshell.screens.length; index++) {
            var screen = Quickshell.screens[index];
            if (screen && screen.name === target)
                return screen;
        }
        return null;
    }

    function scaleForScreen(screen) {
        return Math.min(1.2, (screen ? screen.height / 1080 : 1))
            * Math.max(0.8, Math.min(1.4, Config.fontScale))
            * Ui.Tokens.uiScaleFor(screen && screen.name ? String(screen.name) : "");
    }

    function budgetForScreen(screen) {
        var scale = scaleForScreen(screen);
        var topMargin = Math.round(Math.max(
            24 * scale, (screen.height - 250 * scale) * 0.28));
        return Lifecycle.surfaceBudget({
            screenWidth: screen.width,
            screenHeight: screen.height,
            topMargin: topMargin,
            shadowPadX: 52 * scale,
            shadowPadTop: 54 * scale,
            shadowPadBottom: 76 * scale,
            bottomSafeMargin: 16 * scale,
            compressedHeroHeight: 126 * scale
        });
    }

    function availableMonitorNames() {
        var names = [];
        for (var index = 0; index < Quickshell.screens.length; index++) {
            var screen = Quickshell.screens[index];
            if (screen && screen.name)
                names.push(String(screen.name));
        }
        return names;
    }

    function restHeightForScreen(screen) {
        if (!screen)
            return 0;
        return Math.min(
            250 * scaleForScreen(screen),
            budgetForScreen(screen).maxCardHeight);
    }

    function frostEligible() {
        return (LauncherConfig.bgBlur | 0) > 0
            && !Motion.reduce
            && !Performance.blurDisabled;
    }

    function dispatchLifecycle(event) {
        lifecycleState = Lifecycle.reduce(lifecycleState, event);
        if (lifecycleState.phase === Lifecycle.PHASES.CLOSED) {
            activeSurface = null;
            activeLauncher = null;
        }
    }

    function recoveryMonitor() {
        return Lifecycle.recoveryMonitor({
            availableMonitors: availableMonitorNames(),
            pendingMonitor: lifecycleState.pendingMonitor,
            requestedMonitor: requestedMonitor,
            focusedMonitor: focusedMonitor(),
            currentMonitor: lifecycleState.monitor
        });
    }

    function handleLifecycleEvent(event) {
        var captured = event || {};
        var replacement = "";
        if (captured.type === "surfaceLost"
                && Number(captured.generation) === lifecycleState.generation
                && String(captured.monitor || "") === lifecycleState.monitor) {
            replacement = openRequested ? recoveryMonitor() : "";
            if (replacement)
                requestedMonitor = replacement;
        }
        dispatchLifecycle(captured);
        if (replacement && openRequested
                && lifecycleState.phase === Lifecycle.PHASES.CLOSED) {
            recoveryTarget = replacement;
            recoveryDelay.generation = lifecycleState.generation;
            recoveryDelay.restart();
        }
    }

    function queueLifecycleEvent(event) {
        var captured = {};
        var source = event || {};
        for (var key in source)
            captured[key] = source[key];
        var lifetime = root.lifetime;
        Qt.callLater(function () {
            if (lifetime.alive)
                root.handleLifecycleEvent(captured);
        });
    }

    function requestClose(generation, monitor) {
        var capturedGeneration = Number(generation);
        var capturedMonitor = String(monitor || "");
        var lifetime = root.lifetime;
        Qt.callLater(function () {
            if (!lifetime.alive)
                return;
            if (capturedGeneration !== root.lifecycleState.generation
                    || capturedMonitor !== root.lifecycleState.monitor)
                return;
            root.hide();
        });
    }

    function recoverScreens() {
        var state = lifecycleState;
        var currentValid = screenForName(state.monitor) !== null;

        if (state.phase !== Lifecycle.PHASES.CLOSED && !currentValid) {
            handleLifecycleEvent({
                type: "surfaceLost",
                generation: state.generation,
                monitor: state.monitor
            });
            return;
        }

        if (state.pendingMonitor
                && screenForName(state.pendingMonitor) === null) {
            var fallback = recoveryMonitor();
            if (!fallback) {
                handleLifecycleEvent({
                    type: "surfaceLost",
                    generation: state.generation,
                    monitor: state.monitor
                });
                return;
            }
            requestedMonitor = fallback;
            dispatchLifecycle({
                type: "show",
                monitor: fallback,
                height: restHeightForScreen(screenForName(fallback)),
                frostEligible: frostEligible()
            });
            return;
        }

        if (openRequested
                && state.phase === Lifecycle.PHASES.CLOSED
                && !recoveryDelay.running) {
            var target = recoveryMonitor();
            if (target)
                show(target);
        }
    }

    function show(mon) {
        recoveryDelay.stop();
        recoveryTarget = "";
        var target = String(mon || "");
        if (!screenForName(target))
            target = focusedMonitor();
        var screen = screenForName(target);
        if (!screen)
            return;

        requestedMonitor = target;
        openRequested = true;
        dispatchLifecycle({
            type: "show",
            monitor: target,
            height: restHeightForScreen(screen),
            frostEligible: frostEligible()
        });
    }

    function hide() {
        recoveryDelay.stop();
        recoveryTarget = "";
        openRequested = false;
        requestedMonitor = "";
        restorePointerFocus();
        dispatchLifecycle({ type: "hide" });
    }

    function toggle(mon) {
        if (openRequested)
            hide();
        else
            show(mon);
    }

    function registerActive(surface, launcher) {
        activeSurface = surface;
        activeLauncher = launcher;
        if (surface) {
            surface.windowRail = railForMonitor(surface.surfaceMonitor);
            surface.windowRailSurface = railSurfaceForMonitor(
                surface.surfaceMonitor);
        }
    }

    function registerRail(surface) {
        if (!surface || !surface.surfaceMonitor)
            return;
        var next = {};
        for (var name in railSurfaces)
            next[name] = railSurfaces[name];
        next[String(surface.surfaceMonitor)] = surface;
        railSurfaces = next;
        railRevision++;
        if (activeSurface
                && String(activeSurface.surfaceMonitor || "")
                    === String(surface.surfaceMonitor)) {
            activeSurface.windowRail = surface.rail;
            activeSurface.windowRailSurface = surface;
        }
    }

    function railForMonitor(monitor) {
        var surface = railSurfaceForMonitor(monitor);
        return surface ? surface.rail : null;
    }

    function railSurfaceForMonitor(monitor) {
        return railSurfaces[String(monitor || "")] || null;
    }

    // The launcher holds exclusive keyboard focus; suppress pointer-driven
    // refocus while open so a cursor outside its small surface does not hand keys
    // back, then restore the user's setting on close.
    function freezePointerFocus() {
        if (pointerFocusForced || !Wm.caps.liveConfigEval)
            return;
        pointerFocusForced = true;
        root.restorePending = false;
        root.savedFollowMouse = "";
        Wm.setFocusFollowsMouse("0", function (prev) {
            root.savedFollowMouse = prev || "";
            if (root.restorePending)
                root._restoreFollowMouse();
        });
    }

    function restorePointerFocus() {
        if (!pointerFocusForced)
            return;
        pointerFocusForced = false;
        if (root.savedFollowMouse !== "")
            root._restoreFollowMouse();
        else
            root.restorePending = true;
    }

    function _restoreFollowMouse() {
        root.restorePending = false;
        if (root.savedFollowMouse !== "")
            Wm.setFocusFollowsMouse(root.savedFollowMouse);
    }

    function stateDump() {
        var dump = activeLauncher ? activeLauncher.stateDump() : {};
        var mapped = activeSurface
            && activeSurface.invocationSurface
            && activeSurface.backingWindowVisible;

        dump.open = openRequested;
        dump.phase = String(lifecycleState.phase || Lifecycle.PHASES.CLOSED);
        dump.monitor = String(lifecycleState.monitor || "");
        dump.generation = Number(lifecycleState.generation || 0);
        dump.capture = String(lifecycleState.capture || Lifecycle.CAPTURE.IDLE);
        dump.preludeTicks = Number(lifecycleState.preludeTicks || 0);
        dump.closeTicks = Number(lifecycleState.closeTicks || 0);
        dump.mapped = Boolean(mapped);
        dump.focusHeld = Boolean(lifecycleState.focusHeld);
        dump.surfaceWidth = mapped ? Math.round(activeSurface.width) : 0;
        dump.surfaceHeight = mapped ? Math.round(activeSurface.height) : 0;
        dump.screenWidth = mapped
            ? Math.round(activeSurface.modelData.width) : 0;
        dump.screenHeight = mapped
            ? Math.round(activeSurface.modelData.height) : 0;
        dump.cardHeight = mapped
            ? Math.round(activeSurface.presentedHeight) : 0;
        dump.inputEnabled = Boolean(
            mapped && activeSurface.inputEnabled);
        dump.maskWidth = activeSurface && activeSurface.inputEnabled
            ? Math.round(activeSurface.cardWidth
                * activeSurface.visualScale) : 0;
        dump.maskHeight = activeSurface && activeSurface.inputEnabled
            ? Math.round(activeSurface.presentedHeight
                * activeSurface.visualScale) : 0;
        dump.visualOpacity = activeSurface
            ? Number(activeSurface.visualOpacity) : 0;
        dump.visualScale = activeSurface
            ? Number(activeSurface.visualScale) : 0;
        dump.visualOffsetY = activeSurface
            ? Number(activeSurface.visualOffsetY) : 0;
        dump.frostActive = Boolean(
            activeSurface && activeSurface.frostActive);
        dump.frostPresented = Boolean(
            activeSurface && activeSurface.frostPresented);
        dump.frostPresentationHeight = activeSurface
            ? Math.round(activeSurface.frostPlan.drawerHeight || 0) : 0;
        dump.frostPresentationOpacity = activeSurface
            ? Number(activeSurface.frostPlan.opacity || 0) : 0;
        dump.frameDriverActive = Boolean(
            activeSurface && activeSurface.frameDriverActive);
        dump.outerHeight = Math.round(lifecycleState.outerHeight || 0);
        dump.btConnected = 0;
        return dump;
    }


    Component.onDestruction: root.lifetime.alive = false

    SharedProviders.Providers {
        id: sharedProviders
    }

    Timer {
        id: recoveryDelay
        property int generation: 0
        interval: 50
        repeat: false
        onTriggered: {
            var target = root.recoveryTarget;
            root.recoveryTarget = "";
            if (!root.openRequested
                    || root.lifecycleState.phase
                        !== Lifecycle.PHASES.CLOSED
                    || root.lifecycleState.generation !== generation
                    || root.screenForName(target) === null)
                return;
            root.show(target);
        }
    }

    Connections {
        target: Quickshell
        function onScreensChanged() {
            root.recoverScreens();
        }
    }

    Variants {
        model: Screens.uniqueByName(Quickshell.screens)

        HeroVariant.LauncherSurface {
            providerSet: sharedProviders
            lifecycleState: root.lifecycleState
            windowRail: {
                void root.railRevision;
                return root.railForMonitor(modelData.name);
            }
            windowRailSurface: {
                void root.railRevision;
                return root.railSurfaceForMonitor(modelData.name);
            }
            onLifecycleEvent: event => root.queueLifecycleEvent(event)
            onRequestClose: (generation, monitor) =>
                root.requestClose(generation, monitor)
            onBecameActive: (surface, launcher) =>
                root.registerActive(surface, launcher)
        }
    }

    Variants {
        model: Screens.uniqueByName(Quickshell.screens)

        HeroVariant.WindowRailSurface {
            launcher: root.activeLauncher
            launcherSurface: root.activeSurface
            onReady: surface => root.registerRail(surface)
        }
    }

    // One dismiss scrim per screen, mapped only for the screen holding the
    // active surface: the scrim exists where the launcher holds focus.
    Variants {
        model: Screens.uniqueByName(Quickshell.screens)

        HeroVariant.DismissScrim {
            screenData: modelData
            surface: root.activeSurface
                && String(root.activeSurface.surfaceMonitor || "")
                    === String(modelData.name)
                ? root.activeSurface : null
        }
    }
}
