pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Quickshell
import "../../../../components"
import Quickshell.Wayland
import Quickshell.Widgets
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../../shared/Singletons"
import "../../shared/lib/lifecycle.js" as Lifecycle
import "." as HeroVariant

PanelWindow {
    id: win

    required property var modelData
    required property Item providerSet
    required property var lifecycleState
    property var windowRail: null
    property var windowRailSurface: null

    signal lifecycleEvent(var event)
    signal requestClose(int generation, string monitor)
    signal becameActive(var surface, var launcher)

    property string surfaceMonitor: modelData ? String(modelData.name || "") : ""
    readonly property real s: Math.min(
        1.2, (modelData ? modelData.height / 1080 : 1))
        * Math.max(0.8, Math.min(1.4, Config.fontScale))
        * Tokens.uiScaleFor(surfaceMonitor)
    readonly property bool invocationSurface: Lifecycle.mapsMonitor(
        lifecycleState, surfaceMonitor)
    readonly property bool mapWanted: invocationSurface
        && !lifecycleState.unmapRequested
    readonly property bool inputEnabled: invocationSurface
        && !lifecycleState.visualTransparent
        && !lifecycleState.unmapRequested
    readonly property bool capturePending: invocationSurface
        && lifecycleState.capture === Lifecycle.CAPTURE.PENDING
    readonly property bool frostTreeNeeded: invocationSurface
        && (lifecycleState.capture === Lifecycle.CAPTURE.PENDING
            || lifecycleState.capture === Lifecycle.CAPTURE.READY)

    readonly property real shadowPadX: 52 * s
    readonly property real shadowPadTop: 54 * s
    readonly property real shadowPadBottom: 76 * s
    readonly property real bottomSafeMargin: 16 * s
    readonly property real longRadius: Math.max(0, LauncherConfig.radius) * s
    readonly property real shortRadius: Math.max(
        2, LauncherConfig.radius * 0.18) * s
    readonly property real cardWidth: launcher.implicitWidth
    readonly property real stableTopMargin: Math.round(Math.max(
        24 * s, (modelData.height - 250 * s) * 0.28))
    readonly property var surfaceBudget: Lifecycle.surfaceBudget({
        screenWidth: modelData.width,
        screenHeight: modelData.height,
        topMargin: stableTopMargin,
        shadowPadX: shadowPadX,
        shadowPadTop: shadowPadTop,
        shadowPadBottom: shadowPadBottom,
        bottomSafeMargin: bottomSafeMargin,
        compressedHeroHeight: 126 * s
    })
    readonly property real maxDrawerHeight: surfaceBudget.maxDrawerHeight
    readonly property real actualCardCapacity: Math.max(
        0, height - shadowPadTop - shadowPadBottom)
    readonly property bool frostActive: localFrost.frozenReady
        && lifecycleState.capture === Lifecycle.CAPTURE.READY
    readonly property var frostPlan: Lifecycle.frostPresentation({
        frostActive: frostActive,
        drawerHeight: launcher.drawerPresentedHeight,
        drawerOpacity: launcher.drawerPresentedOpacity,
        maxDrawerHeight: maxDrawerHeight,
        sourceHeight: localFrost.sampledHeight,
        bleedTop: localFrost.bleedTop,
        bleedBottom: localFrost.bleedBottom
    })

    property real visualOpacity: 0
    property real visualScale: 0.965
    property real visualOffsetY: -13 * s
    property real presentedHeight: 0
    property int observedGeneration: -1
    property string observedPhase: ""
    property string observedCloseStage: ""
    property real observedVisibleHeight: -1
    property bool syncQueued: false
    property int lossReportedGeneration: -1
    property int backingGeneration: -1

    readonly property alias launcherItem: launcher
    readonly property bool frameDriverActive: lifecycleFrame.running
    readonly property bool frostPresented: frostPresentation.active

    screen: modelData
    visible: mapWanted
    color: "transparent"
    surfaceFormat.opaque: false
    implicitWidth: Math.ceil(cardWidth + shadowPadX * 2)
    implicitHeight: Math.ceil(
        Math.max(0, lifecycleState.outerHeight)
            + shadowPadTop + shadowPadBottom)
    exclusionMode: ExclusionMode.Ignore
    WlrLayershell.namespace: "launcher"
    WlrLayershell.layer: WlrLayer.Overlay
    WlrLayershell.keyboardFocus: WlrKeyboardFocus.Exclusive
    anchors { top: true }
    margins.top: stableTopMargin
    mask: inputEnabled ? cardInputRegion : emptyInputRegion

    function emitLifecycle(type, details, generation) {
        var event = {};
        var source = details || {};
        for (var key in source)
            event[key] = source[key];
        event.type = type;
        event.generation = Number(generation);
        if (!isFinite(event.generation))
            return;
        lifecycleEvent(event);
    }

    function deferLifecycle(type, details, generation) {
        var capturedType = String(type || "");
        var capturedDetails = {};
        var source = details || {};
        for (var key in source)
            capturedDetails[key] = source[key];
        var capturedGeneration = Number(generation);
        Qt.callLater(function () {
            win.emitLifecycle(
                capturedType, capturedDetails, capturedGeneration);
        });
    }

    function scheduleSync() {
        if (syncQueued)
            return;
        syncQueued = true;
        var capturedGeneration = lifecycleState.generation;
        Qt.callLater(function () {
            syncQueued = false;
            if (capturedGeneration !== lifecycleState.generation) {
                scheduleSync();
                return;
            }
            syncLifecycle();
        });
    }

    function focusInput(generation) {
        if (Number(generation) !== lifecycleState.generation
                || !invocationSurface || !backingWindowVisible)
            return;
        launcher.focusInput();
    }

    function checkOuterConfigured(generation) {
        if (Number(generation) !== lifecycleState.generation
                || !invocationSurface
                || lifecycleState.geometryStage !== "grow-outer")
            return;
        if (actualCardCapacity + 0.5 >= lifecycleState.outerHeight) {
            emitLifecycle("outerConfigured", {
                height: lifecycleState.outerHeight
            }, generation);
        }
    }

    function observeActualGeometry(generation) {
        if (Number(generation) !== lifecycleState.generation
                || !invocationSurface)
            return;
        if (lifecycleState.geometryStage === "grow-outer") {
            checkOuterConfigured(generation);
            return;
        }
        if (lifecycleState.geometryStage === "stable"
                && lifecycleState.targetHeight
                    > lifecycleState.visibleHeight + 0.5
                && actualCardCapacity
                    > lifecycleState.visibleHeight + 0.5) {
            emitLifecycle("grow", {
                height: lifecycleState.targetHeight,
                duration: lifecycleState.geometryDuration
            }, generation);
        }
    }

    function animatePresentedHeight(target, generation) {
        var next = Math.max(0, Number(target) || 0);
        cardHeightAnimation.stop();
        if (Math.abs(presentedHeight - next) < 0.5) {
            presentedHeight = next;
            if (lifecycleState.geometryStage === "shrink-card") {
                deferLifecycle(
                    "shrinkVisualComplete", {}, generation);
            }
            return;
        }
        if (Motion.reduce || lifecycleState.geometryDuration <= 0) {
            presentedHeight = next;
            if (lifecycleState.geometryStage === "shrink-card") {
                deferLifecycle(
                    "shrinkVisualComplete", {}, generation);
            }
            return;
        }
        cardHeightAnimation.generation = generation;
        cardHeightAnimation.to = next;
        cardHeightAnimation.duration = lifecycleState.geometryDuration;
        cardHeightAnimation.restart();
    }

    function startOpening(generation) {
        closeVisual.stop();
        transparentDeadline.stop();
        if (Motion.open <= 0) {
            visualOpacity = 1;
            visualScale = 1;
            visualOffsetY = 0;
            deferLifecycle("openComplete", {}, generation);
            return;
        }
        openVisual.generation = generation;
        openVisual.restart();
    }

    function startClosing(generation) {
        openVisual.stop();
        if (Motion.close <= 0) {
            visualOpacity = 0;
            visualScale = 0.985;
            visualOffsetY = 8 * s;
            deferLifecycle("visualHidden", {}, generation);
            return;
        }
        closeVisual.generation = generation;
        closeVisual.restart();
    }

    function startTransparentFlush(generation) {
        openVisual.stop();
        closeVisual.stop();
        visualOpacity = 0;
        if (backingWindowVisible && !lifecycleState.unmapRequested) {
            transparentDeadline.generation = generation;
            transparentDeadline.restart();
        }
    }

    function reportSurfaceLost(generation) {
        var capturedGeneration = Number(generation);
        if (capturedGeneration !== lifecycleState.generation
                || !invocationSurface)
            return;
        if (lifecycleState.phase === Lifecycle.PHASES.CLOSING
                && lifecycleState.unmapRequested) {
            emitLifecycle("unmapped", {}, capturedGeneration);
            return;
        }
        if (lossReportedGeneration === capturedGeneration)
            return;
        lossReportedGeneration = capturedGeneration;
        emitLifecycle("surfaceLost", {
            monitor: surfaceMonitor
        }, capturedGeneration);
    }

    function syncLifecycle() {
        var state = lifecycleState;

        if (!invocationSurface) {
            captureDeadline.stop();
            transparentDeadline.stop();
            growDeadline.stop();
            return;
        }

        if (observedGeneration !== state.generation) {
            openVisual.stop();
            closeVisual.stop();
            cardHeightAnimation.stop();
            captureDeadline.stop();
            transparentDeadline.stop();
            growDeadline.stop();
            observedGeneration = state.generation;
            lossReportedGeneration = -1;
            observedPhase = "";
            observedCloseStage = "";
            observedVisibleHeight = state.visibleHeight;
            presentedHeight = state.visibleHeight;
            visualOpacity = 0;
            visualScale = 0.965;
            visualOffsetY = -13 * s;
            becameActive(win, launcher);
        } else if (observedVisibleHeight !== state.visibleHeight) {
            observedVisibleHeight = state.visibleHeight;
            animatePresentedHeight(
                state.visibleHeight, state.generation);
        }

        if (state.geometryStage === "grow-outer") {
            if (!growDeadline.running) {
                growDeadline.generation = state.generation;
                growDeadline.restart();
            }
            var growGeneration = state.generation;
            Qt.callLater(function () {
                win.checkOuterConfigured(growGeneration);
            });
        } else {
            growDeadline.stop();
        }

        if (observedPhase !== state.phase) {
            observedPhase = state.phase;
            if (state.phase === Lifecycle.PHASES.PRELUDE) {
                visualOpacity = 0;
                if (backingWindowVisible) {
                    var focusGeneration = state.generation;
                    Qt.callLater(function () {
                        win.focusInput(focusGeneration);
                    });
                }
            } else if (state.phase === Lifecycle.PHASES.OPENING) {
                startOpening(state.generation);
            } else if (state.phase === Lifecycle.PHASES.OPEN) {
                visualOpacity = 1;
                visualScale = 1;
                visualOffsetY = 0;
            } else if (state.phase === Lifecycle.PHASES.CLOSING
                    && state.closeStage === "visual") {
                startClosing(state.generation);
            }
        }

        if (observedCloseStage !== state.closeStage) {
            observedCloseStage = state.closeStage;
            if (state.phase === Lifecycle.PHASES.CLOSING
                    && state.closeStage === "transparent") {
                startTransparentFlush(state.generation);
            }
        }

        if (capturePending) {
            if (!captureDeadline.running) {
                captureDeadline.generation = state.generation;
                captureDeadline.restart();
            }
        } else {
            captureDeadline.stop();
        }

        if (state.unmapRequested) {
            transparentDeadline.stop();
            var unmapGeneration = state.generation;
            Qt.callLater(function () {
                if (!backingWindowVisible)
                    win.emitLifecycle(
                        "unmapped", {}, unmapGeneration);
            });
        }
    }

    onLifecycleStateChanged: scheduleSync()
    onHeightChanged: {
        var heightGeneration = lifecycleState.generation;
        Qt.callLater(function () {
            win.observeActualGeometry(heightGeneration);
        });
    }
    onInvocationSurfaceChanged: {
        if (invocationSurface)
            scheduleSync();
    }
    onBackingWindowVisibleChanged: {
        if (backingWindowVisible && invocationSurface) {
            backingGeneration = lifecycleState.generation;
            var visibleGeneration = lifecycleState.generation;
            Qt.callLater(function () {
                win.focusInput(visibleGeneration);
                win.observeActualGeometry(visibleGeneration);
            });
            scheduleSync();
        } else if (!backingWindowVisible && invocationSurface) {
            reportSurfaceLost(backingGeneration);
        }
    }
    onClosed: reportSurfaceLost(backingGeneration)
    onResourcesLost: reportSurfaceLost(backingGeneration)

    Region {
        id: emptyInputRegion
    }

    Region {
        id: cardInputRegion
        item: visibleCardBounds
        topLeftRadius: Math.round(win.longRadius * win.visualScale)
        topRightRadius: Math.round(win.shortRadius * win.visualScale)
        bottomLeftRadius: Math.round(win.shortRadius * win.visualScale)
        bottomRightRadius: Math.round(win.longRadius * win.visualScale)
    }

    Item {
        id: visibleCardBounds
        x: win.shadowPadX + win.cardWidth * (1 - win.visualScale) / 2
        y: win.shadowPadTop + win.visualOffsetY
        width: win.cardWidth * win.visualScale
        height: win.presentedHeight * win.visualScale
    }

    // Capture lives outside the zero-opacity visual subtree so the scheduled
    // texture update can finish during Prelude. It produces no scene pixels;
    // only FrostLayer presents the accepted texture inside the card clip.
    // The object is persistent for the surface's life. Recreating it per open
    // tears a ScreencopyView down while the compositor still delivers output
    // enter/leave, which segfaults the client where there is no focus-grab
    // protocol; open and close ride captureSource and a per-generation reset.
    HeroVariant.LocalFrost {
        id: localFrost
        captureScreen: win.modelData
        generation: win.lifecycleState.generation
        screenWidth: win.modelData.width
        screenHeight: win.modelData.height
        cropX: (win.modelData.width - win.cardWidth) / 2
        cropY: win.stableTopMargin + win.shadowPadTop
            + 126 * win.s
        cropWidth: win.cardWidth
        cropHeight: win.maxDrawerHeight
        blurRadius: Math.max(
            1, Math.min(64, LauncherConfig.bgBlur | 0))
        captureWanted: win.frostTreeNeeded
        captureEnabled: win.capturePending
            && win.backingWindowVisible
        onReady: generation => win.lifecycleEvent({
            type: "captureReady",
            generation: generation
        })
        onFailed: generation => win.lifecycleEvent({
            type: "captureFailed",
            generation: generation
        })
    }

    Item {
        id: visualGroup
        x: win.shadowPadX
        y: win.shadowPadTop + win.visualOffsetY
        width: win.cardWidth
        height: win.presentedHeight
        opacity: win.visualOpacity
        scale: win.visualScale
        transformOrigin: Item.Top

        RectangularShadow {
            width: visualGroup.width
            height: visualGroup.height
            color: Qt.rgba(0, 0, 0, 0.94)
            offset: Qt.vector2d(8 * win.s, 8 * win.s)
            blur: Math.max(0.5, win.s)
            spread: 0
            topLeftRadius: win.longRadius
            topRightRadius: win.shortRadius
            bottomLeftRadius: win.shortRadius
            bottomRightRadius: win.longRadius
        }

        ClippingRectangle {
            id: cardClip
            width: visualGroup.width
            height: visualGroup.height
            color: Theme.drawer
            contentUnderBorder: true
            topLeftRadius: win.longRadius
            topRightRadius: win.shortRadius
            bottomLeftRadius: win.shortRadius
            bottomRightRadius: win.longRadius
            border.width: 1
            border.color: Theme.frame

            Loader {
                id: frostPresentation
                x: 0
                y: launcher.heroPresentedHeight
                width: win.cardWidth
                height: win.frostPlan.drawerHeight
                opacity: win.frostPlan.active
                    ? win.frostPlan.opacity : 0
                active: win.frostPlan.active
                asynchronous: false

                sourceComponent: Component {
                    HeroVariant.FrostLayer {
                        sourceItem: localFrost.textureItem
                        sourceRect: localFrost.sampleRect
                        active: true
                        bleedLeft: localFrost.bleedLeft
                        bleedTop: localFrost.bleedTop
                        bleedRight: localFrost.bleedRight
                        bleedBottom: localFrost.bleedBottom
                        textureHeight: win.frostPlan.textureHeight
                        blurRadius: Math.max(
                            1, Math.min(64, LauncherConfig.bgBlur | 0))
                        topLeftRadius: 0
                        topRightRadius: 0
                        bottomLeftRadius: win.shortRadius
                        bottomRightRadius: win.longRadius
                    }
                }
            }

            HeroVariant.Launcher {
                id: launcher
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.top: parent.top
                s: win.s
                shown: win.invocationSurface
                providerSet: win.providerSet
                lifecyclePhase: String(win.lifecycleState.phase || "")
                lifecycleGeneration: win.lifecycleState.generation
                lifecycleOuterHeight: win.lifecycleState.outerHeight
                workAreaWidth: win.surfaceBudget.workAreaWidth
                workAreaHeight: win.surfaceBudget.workAreaHeight
                reportedSurfaceWidth: win.width
                reportedSurfaceHeight: win.height
                frostActive: win.frostActive
                windowRail: win.windowRail
                onRequestClose: win.requestClose(
                    win.lifecycleState.generation, win.surfaceMonitor)
                onOuterHeightRequested: (height, growing, duration) => {
                    win.lifecycleEvent({
                        type: growing ? "grow" : "shrink",
                        generation: win.lifecycleState.generation,
                        height: height,
                        duration: duration
                    });
                }
            }
        }
    }

    FocusGrab {
        id: focusGrab
        property int generation: -1
        active: win.invocationSurface
            && win.lifecycleState.focusHeld
            && win.backingWindowVisible
        windows: win.windowRailSurface
            ? [win, win.windowRailSurface] : [win]
        onActiveChanged: {
            if (active)
                generation = win.lifecycleState.generation;
        }
        onCleared: {
            if (!win.invocationSurface
                    || win.lifecycleState.phase === Lifecycle.PHASES.CLOSING)
                return;
            win.requestClose(generation, win.surfaceMonitor);
        }
    }

    // The dismiss scrim lives as a top-level sibling surface (DismissScrim,
    // driven from Main.qml): a layer surface nested inside this one is the
    // lifecycle quickshell faults on under a compositor with no focus-grab
    // protocol. This surface only exposes when it wants the scrim and the
    // generation an outside press must close.
    readonly property bool dismissWanted: !focusGrab.available && focusGrab.active
    readonly property int dismissGeneration: focusGrab.generation

    FrameAnimation {
        id: lifecycleFrame
        property int generation: 0
        running: win.backingWindowVisible
            && win.invocationSurface
            && ((win.lifecycleState.phase === Lifecycle.PHASES.PRELUDE
                    && win.lifecycleState.preludeTicks < 2)
                || (win.lifecycleState.phase === Lifecycle.PHASES.CLOSING
                    && win.lifecycleState.closeStage === "transparent"
                    && !win.lifecycleState.unmapRequested
                    && win.lifecycleState.closeTicks < 2)
                || win.lifecycleState.geometryStage === "grow-outer"
                || win.lifecycleState.geometryStage === "shrink-wait-tick")
        onRunningChanged: {
            if (running)
                generation = win.lifecycleState.generation;
        }
        onTriggered: win.emitLifecycle("frameTick", {}, generation)
    }

    Timer {
        id: captureDeadline
        property int generation: 0
        interval: 50
        repeat: false
        onTriggered: {
            win.lifecycleEvent({
                type: "captureTimeout",
                generation: generation
            });
        }
    }

    Timer {
        id: transparentDeadline
        property int generation: 0
        interval: 50
        repeat: false
        onTriggered: win.lifecycleEvent({
            type: "closeFallback",
            generation: generation
        })
    }

    Timer {
        id: growDeadline
        property int generation: 0
        interval: 50
        repeat: false
        onTriggered: win.lifecycleEvent({
            type: "growFallback",
            generation: generation,
            height: win.actualCardCapacity
        })
    }

    NumberAnimation {
        id: cardHeightAnimation
        property int generation: 0
        target: win
        property: "presentedHeight"
        easing.type: Easing.OutCubic
        onFinished: {
            if (win.lifecycleState.geometryStage === "shrink-card"
                    && Math.abs(win.presentedHeight
                        - win.lifecycleState.targetHeight) < 0.5) {
                win.emitLifecycle(
                    "shrinkVisualComplete", {}, generation);
            }
        }
    }

    ParallelAnimation {
        id: openVisual
        property int generation: 0
        NumberAnimation {
            target: win
            property: "visualOpacity"
            to: 1
            duration: Motion.open
            easing.type: Easing.OutCubic
        }
        NumberAnimation {
            target: win
            property: "visualScale"
            to: 1
            duration: Motion.open
            easing.type: Easing.OutCubic
        }
        NumberAnimation {
            target: win
            property: "visualOffsetY"
            to: 0
            duration: Motion.open
            easing.type: Easing.OutCubic
        }
        onFinished: win.emitLifecycle("openComplete", {}, generation)
    }

    ParallelAnimation {
        id: closeVisual
        property int generation: 0
        NumberAnimation {
            target: win
            property: "visualOpacity"
            to: 0
            duration: Motion.close
            easing.type: Easing.OutCubic
        }
        NumberAnimation {
            target: win
            property: "visualScale"
            to: 0.985
            duration: Motion.close
            easing.type: Easing.OutCubic
        }
        NumberAnimation {
            target: win
            property: "visualOffsetY"
            to: 8 * win.s
            duration: Motion.close
            easing.type: Easing.OutCubic
        }
        onFinished: win.emitLifecycle("visualHidden", {}, generation)
    }

    Component.onCompleted: {
        if (backingWindowVisible)
            backingGeneration = lifecycleState.generation;
        scheduleSync();
    }
    Component.onDestruction: reportSurfaceLost(observedGeneration)
}
