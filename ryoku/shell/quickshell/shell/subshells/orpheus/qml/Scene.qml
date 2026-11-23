pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import Ryoku.Blobs
import Ryoku.Ui.Singletons
import shell.services
import shell.modules.stage.frame
import "Metrics.js" as M
import "components" as C
import "kit" as K
import "launcher/vessel" as V
import "recording" as Rec
import "settings" as S

Item {
    id: scene

    property var modelData: null
    readonly property var prefs: S.Prefs
    readonly property var screen: modelData
    readonly property string screenName: screen ? screen.name : ""
    property string cardKey: ""
    property var cardArgs: ({})
    property string cardOwner: "voice"
    property bool settingsOpen: false
    property string settingsPage: "bar"
    readonly property bool hintsSuppressed: cardKey !== ""
    readonly property real cardMaxHeight: Math.max(0, Math.min(M.cardMaxH,
        (screen ? screen.height : M.cardMaxH + K.Style.barH + 48)
            - K.Style.barH - 48))
    readonly property real launcherMaxHeight: Math.max(0, Math.min(
        M.launcherMaxH * K.Style.scale,
        (screen ? screen.height : M.launcherMaxH * K.Style.scale + K.Style.barH + 48)
            - K.Style.barH - 48))
    // The launcher card: open from the request until it has fully melted, so
    // the launcher host keeps it alive through the close.
    property bool launcherOpening: false
    readonly property bool launcherOpen: launcherOpening || cardKey === "launcher"
    readonly property bool launcherShown: launcherOpen
        || (cardKey === "" && cards.retainedKey === "launcher")
    readonly property var launcherSheet: cards.launcherSheet
    readonly property real textRoom: Math.max(64,
        Math.max(Number(prefs.mediaWidth) || 220,
            Number(prefs.windowWidth) || 240) - win.overflow)

    property real fallbackOriginX: 0
    property real fallbackOriginW: 20
    property string hoverCandidate: ""
    property var hoverCandidateAnchor: null
    property string hoverSwitchedKey: ""
    property double hoverSwitchedAt: 0
    property var hoverStates: ({})
    property var textModules: ({})
    property bool autoHidden: false
    property var pendingSurfaceActions: []
    property bool settingsSurfaceHeld: false
    property bool cardSurfaceHeld: false
    property var pendingCardAction: null
    // the screen name this scene is registered under with the launcher vessels
    property string vesselName: ""
    readonly property bool surfaceOpening: pendingSurfaceActions.length > 0
    readonly property real restingSurfaceHeight: Math.ceil(Math.max(
        K.Style.barH + K.Style.s2 + Math.ceil(K.Style.label * 1.5)
            + K.Style.s3 + K.Style.s1,
        K.Style.barH + Math.max(0, Number(prefs.depth) || 0)
            + (prefs.form === "square" ? M.squareSmoothing : M.smoothing)
            + K.Style.border,
        // a hidden bar keeps the recording bubble docked a whole lift lower in the rail
        prefs.autoHide !== "off" ? hideLift + K.Style.barH + M.smoothing : 0))
    // the hidden body clears the top by the melt distance too, or its edge and
    // the docked bubble's shoulder blend into a sliver along the top
    readonly property real hideLift: K.Style.barH + 2 + M.smoothing

    readonly property string recPhase: String(Recorder.phase || "idle")
    readonly property bool recShown: prefs.recordingInBar && recPhase !== "idle"
        && Recorder.showsOn(scene.screen)
    // the bubble rides down as far as the rail slides up, so it stays docked
    readonly property real recDock: -rail.y
    readonly property real recDockShare: Math.max(0, Math.min(1, recDock / hideLift))
    readonly property var recordDrop: ({
        x: recMark.x - (K.Style.barPad - K.Style.hitPad),
        w: recShown ? win.recBubbleW : 0
    })
    readonly property var recordBubble: win.recInline ? ({
        x: recMark.x - (K.Style.barPad - K.Style.hitPad),
        w: win.recBubbleW * recInlineGeom.grown * recDockShare,
        y: recDock
    }) : ({
        x: recBubbleGeom.x,
        w: recBubbleGeom.width,
        y: recDock
    })

    readonly property var leftDrop: ({ x: leftGeom.x, w: leftGeom.width })
    readonly property var voiceDrop: ({ x: voiceGeom.x, w: voiceGeom.width })
    readonly property var rightDrop: ({ x: rightGeom.x, w: rightGeom.width })
    readonly property real bandX: win.full ? 0 : Math.min(leftGeom.x, voiceGeom.x, rightGeom.x)
    readonly property real bandW: win.full ? win.width : Math.max(0,
        Math.max(leftGeom.x + leftGeom.width, voiceGeom.x + voiceGeom.width,
            rightGeom.x + rightGeom.width) - bandX)
    readonly property real waveX: win.morphed(win.morphFrom.wx, win.islands ? voiceGeom.x : bandX)
    readonly property real waveW: Math.max(0,
        win.morphed(win.morphFrom.ww, win.islands ? voiceGeom.width : bandW))
    readonly property var bubbleLeft: ({
        x: leftBubbleGeom.x,
        w: leftBubbleGeom.width,
        visible: win.leftBubbleVisible,
        key: win.leftBubbleKey
    })
    // Side content sits against the drop's inner edge, so the extra paper an
    // edge-hugging drop carries stays on the screen side.
    readonly property real leftContentX: leftGeom.x
        + Math.max(K.Style.barPad, leftGeom.width - K.Style.barPad - win.leftW)
    readonly property real centerContentX: voiceGeom.x + K.Style.barPad
    readonly property real rightContentX: rightGeom.x + K.Style.barPad + win.recInlineExtra
    readonly property var currentOwnerDrop: cardOwner === "left" ? leftDrop
        : cardOwner === "right" ? rightDrop
        : cardOwner === "record" ? recordDrop : voiceDrop
    readonly property real anchoredOriginX: currentOwnerDrop.w > 0
        ? currentOwnerDrop.x : fallbackOriginX
    readonly property real anchoredOriginW: currentOwnerDrop.w > 0
        ? currentOwnerDrop.w : fallbackOriginW

    readonly property bool barHovered: fullBandHover.hovered
        || leftBarHover.hovered || voiceBarHover.hovered || rightBarHover.hovered
        || leftBubbleHover.hovered || recHover.hovered
    readonly property bool bandHovered: bandHover.hovered
    readonly property var activeWorkspace: workspaceForOutput()
    readonly property bool activeWorkspaceOccupied: workspaceOccupied(activeWorkspace)
    readonly property bool hideRequested: prefs.autoHide === "always"
        || (prefs.autoHide === "smart" && activeWorkspaceOccupied)
    readonly property bool holdVisible: frameLayer.framed || bandHovered || cards.hovered
        || cardKey !== "" || settingsOpen
    readonly property point settingsOrigin: {
        const logo = leftLane.slotFor("logo");
        if (logo && logo.width > 0)
            return logo.mapToGlobal(logo.width / 2, logo.height / 2);
        return focusSurface.mapToGlobal(leftGeom.x + K.Style.barPad,
            K.Style.barH / 2);
    }

    width: 0
    height: 0

    function normalKey(key) {
        return key === "calendar" ? "today" : key;
    }

    function validCardKey(key) {
        return ["today", "media", "weather", "controls", "notifications",
            "tray", "power", "system", "session", "recording", "vpn", "launcher"]
            .indexOf(normalKey(key)) >= 0;
    }

    function cardMaxHeightFor(key) {
        return normalKey(key) === "launcher" ? scene.launcherMaxHeight : scene.cardMaxHeight;
    }
    function surfaceIsFull() {
        const fullHeight = Math.max(1, scene.screen ? scene.screen.height : 1);
        return frameLayer.framed || win.height >= fullHeight - 0.5;
    }

    function hasPendingSurfaceRequest(kind) {
        for (let index = 0; index < scene.pendingSurfaceActions.length; ++index) {
            if (scene.pendingSurfaceActions[index].kind === kind)
                return true;
        }
        return false;
    }

    function requestFullSurface(kind, action) {
        if (scene.surfaceIsFull()) {
            action();
            return;
        }
        const pending = [];
        for (let index = 0; index < scene.pendingSurfaceActions.length; ++index) {
            if (scene.pendingSurfaceActions[index].kind !== kind)
                pending.push(scene.pendingSurfaceActions[index]);
        }
        pending.push({ kind: kind, action: action });
        scene.pendingSurfaceActions = pending;
        surfaceReadyTimer.restart();
    }

    function cancelFullSurfaceRequest(kind) {
        const pending = [];
        for (let index = 0; index < scene.pendingSurfaceActions.length; ++index) {
            if (scene.pendingSurfaceActions[index].kind !== kind)
                pending.push(scene.pendingSurfaceActions[index]);
        }
        scene.pendingSurfaceActions = pending;
        if (pending.length === 0)
            surfaceReadyTimer.stop();
    }

    function finishPendingSurfaceActions() {
        if (!scene.surfaceIsFull()) {
            surfaceReadyTimer.restart();
            return;
        }
        const pending = scene.pendingSurfaceActions;
        for (let index = 0; index < pending.length; ++index)
            pending[index].action();
        scene.pendingSurfaceActions = [];
    }

    // launcherOpening survives its own request: the host closes the launcher the
    // moment it reads as shut, so it may only drop once the card key is set.
    function requestCardSurface(action, forLauncher) {
        if (forLauncher !== true)
            scene.launcherOpening = false;
        if (scene.cardSurfaceHeld) {
            action();
            return;
        }
        scene.cardSurfaceHeld = true;
        scene.pendingCardAction = action;
        cardSurfaceReadyTimer.restart();
    }

    function cancelCardSurfaceRequest() {
        scene.launcherOpening = false;
        cardSurfaceReadyTimer.stop();
        scene.pendingCardAction = null;
        if (liquidBody.cardProgress <= 0.001)
            scene.cardSurfaceHeld = false;
    }

    function finishPendingCardAction() {
        const action = scene.pendingCardAction;
        scene.pendingCardAction = null;
        if (action)
            action();
    }


    function defaultOwnerForKey(key) {
        const value = normalKey(key);
        if (value === "recording" && scene.recShown)
            return "record";
        return ["today", "media", "notifications", "recording", "launcher"]
            .indexOf(value) >= 0 ? "voice" : "right";
    }

    function workspaceForOutput() {
        const list = Wm.workspaces || [];
        for (let i = 0; i < list.length; ++i) {
            if (list[i].active === true && list[i].output === scene.screenName)
                return list[i];
        }
        if (Wm.focusedOutput === scene.screenName)
            return Wm.focusedWorkspace;
        return null;
    }

    function workspaceOccupied(workspace) {
        if (!workspace)
            return false;
        if (workspace.occupied === true || Number(workspace.windows) > 0)
            return true;
        const key = workspace.id !== undefined ? workspace.id : workspace.name;
        return Wm.windowsOnWorkspace(key).length > 0;
    }

    function registerTextModule(key, item) {
        const modules = Object.assign({}, scene.textModules);
        modules[key] = item;
        scene.textModules = modules;
    }

    function unregisterTextModule(key, item) {
        if (scene.textModules[key] !== item)
            return;
        const modules = Object.assign({}, scene.textModules);
        delete modules[key];
        scene.textModules = modules;
    }

    function laneNeeded(laneItem) {
        const ids = laneItem.moduleIds || [];
        let total = 0;
        let count = 0;
        for (let i = 0; i < ids.length; ++i) {
            const key = String(ids[i] || "");
            const slot = laneItem.slotFor(key);
            if (!slot || !slot.present)
                continue;
            const textModule = scene.textModules[key];
            const moduleWidth = textModule
                ? Math.max(0, Number(textModule.layoutFixedWidth) || 0)
                    + Math.max(0, Number(textModule.layoutPreferredTextWidth) || 0)
                : Math.max(0, Number(slot.targetWidth) || 0);
            if (moduleWidth <= 0.001)
                continue;
            if (count > 0)
                total += K.Style.barGap;
            total += moduleWidth;
            ++count;
        }
        return total;
    }

    function ownerNameFor(item) {
        let cursor = item;
        while (cursor) {
            if (cursor === recMark)
                return "record";
            if (cursor === leftLane)
                return "left";
            if (cursor === centerLane || cursor === leftBubbleHost)
                return "voice";
            if (cursor === rightLane)
                return "right";
            cursor = cursor.parent;
        }
        if (item && item.mapToItem) {
            const point = item.mapToItem(focusSurface,
                Number(item.width) / 2 || 0, Number(item.height) / 2 || 0);
            const centres = [
                { name: "left", value: leftGeom.x + leftGeom.width / 2 },
                { name: "voice", value: voiceGeom.x + voiceGeom.width / 2 },
                { name: "right", value: rightGeom.x + rightGeom.width / 2 }
            ];
            let best = centres[0];
            for (let i = 1; i < centres.length; ++i) {
                if (Math.abs(point.x - centres[i].value)
                        < Math.abs(point.x - best.value))
                    best = centres[i];
            }
            return best.name;
        }
        return "voice";
    }

    function ownerDropFor(item) {
        const name = ownerNameFor(item);
        return name === "left" ? scene.leftDrop
            : name === "right" ? scene.rightDrop
            : name === "record" ? scene.recordDrop : scene.voiceDrop;
    }

    function isVoiceAnchor(item) {
        return ownerNameFor(item) === "voice";
    }

    function reconcileAutoHide() {
        if (!scene.hideRequested || scene.holdVisible) {
            hideTimer.stop();
            scene.autoHidden = false;
            return;
        }
        if (!scene.autoHidden)
            hideTimer.restart();
    }

    function captureOrigin(anchorItem) {
        if (!anchorItem) {
            scene.cardOwner = "voice";
            scene.fallbackOriginX = voiceGeom.x;
            scene.fallbackOriginW = Math.max(12, voiceGeom.width);
            return;
        }
        scene.cardOwner = ownerNameFor(anchorItem);
        const drop = ownerDropFor(anchorItem);
        scene.fallbackOriginX = drop.x;
        scene.fallbackOriginW = Math.max(12, drop.w);
    }

    function showCard(key, args) {
        const value = normalKey(key);
        if (!scene.validCardKey(value))
            return;
        if (value === "launcher") {
            scene.requestLauncher(true);
            return;
        }
        const nextArgs = args && typeof args === "object" ? args : ({});
        scene.requestCardSurface(function() {
            hoverOpenTimer.stop();
            scene.hoverCandidate = "";
            scene.hoverCandidateAnchor = null;
            scene.settingsOpen = false;
            scene.cardOwner = scene.defaultOwnerForKey(value);
            scene.cardArgs = nextArgs;
            scene.cardKey = value;
        });
    }

    function openCard(key, anchorItem, args) {
        const value = normalKey(key);
        if (!scene.validCardKey(value))
            return;
        if (value === "launcher") {
            scene.requestLauncher(true);
            return;
        }
        scene.captureOrigin(anchorItem);
        const nextArgs = args && typeof args === "object" ? args : ({});
        scene.requestCardSurface(function() {
            hoverOpenTimer.stop();
            scene.hoverCandidate = "";
            scene.hoverCandidateAnchor = null;
            scene.settingsOpen = false;
            scene.cardArgs = nextArgs;
            scene.cardKey = value;
        });
    }


    function markHoverSwitch(key) {
        scene.hoverSwitchedKey = key;
        scene.hoverSwitchedAt = Date.now();
    }

    function openFromHover(key, anchorItem) {
        const value = normalKey(key);
        if (!scene.validCardKey(value) || value === "tray" || value === "launcher"
                || scene.launcherOpen)
            return;
        scene.markHoverSwitch(value);
        scene.captureOrigin(anchorItem);
        scene.requestCardSurface(function() {
            if (scene.hoverStates[value] !== true || scene.launcherOpen) {
                scene.hoverSwitchedKey = "";
                scene.hoverSwitchedAt = 0;
                return;
            }
            scene.settingsOpen = false;
            scene.cardArgs = ({});
            scene.cardKey = value;
        });
    }


    function toggleCard(key, anchorItem, args) {
        const value = normalKey(key);
        if (!scene.validCardKey(value))
            return;
        if (value === "launcher") {
            scene.toggleLauncher();
            return;
        }
        hoverOpenTimer.stop();
        scene.hoverCandidate = "";
        scene.hoverCandidateAnchor = null;
        const now = Date.now();
        const recentHoverSwitch = scene.cardKey === value
            && scene.hoverSwitchedKey === value
            && now - scene.hoverSwitchedAt >= 0
            && now - scene.hoverSwitchedAt < 400;
        scene.hoverSwitchedKey = "";
        scene.hoverSwitchedAt = 0;
        if (recentHoverSwitch)
            return;
        if (scene.cardKey === value)
            scene.closeCard();
        else
            scene.openCard(value, anchorItem, args);
    }

    function closeCard() {
        scene.cancelCardSurfaceRequest();
        hoverOpenTimer.stop();
        hoverCloseTimer.stop();
        scene.hoverCandidate = "";
        scene.hoverCandidateAnchor = null;
        scene.hoverSwitchedKey = "";
        scene.hoverSwitchedAt = 0;
        scene.cardKey = "";
    }

    // The launcher pours out of the voice drop like any card, but only a key,
    // a click away, Esc or a ran result closes it, never the hover timers.
    function openLauncher() {
        if (scene.launcherOpen)
            return;
        scene.launcherOpening = true;
        scene.requestCardSurface(function() {
            hoverOpenTimer.stop();
            hoverCloseTimer.stop();
            scene.hoverCandidate = "";
            scene.hoverCandidateAnchor = null;
            scene.settingsOpen = false;
            scene.cardOwner = scene.defaultOwnerForKey("launcher");
            scene.cardArgs = ({});
            scene.cardKey = "launcher";
            scene.launcherOpening = false;
        }, true);
    }

    function closeLauncher() {
        if (scene.launcherOpen)
            scene.closeCard();
    }

    function hoverCard(key, anchorItem, hovered) {
        const value = normalKey(key);
        if (!scene.validCardKey(value) || value === "tray" || value === "launcher")
            return;
        const states = Object.assign({}, scene.hoverStates);
        states[value] = hovered === true;
        scene.hoverStates = states;
        if (hovered) {
            hoverCloseTimer.stop();
            if (scene.launcherOpen)
                return;
            if (scene.cardKey !== "") {
                if (scene.prefs.hoverSwitch && scene.cardKey !== value)
                    scene.openFromHover(value, anchorItem);
            } else if (scene.prefs.hoverOpen && scene.isVoiceAnchor(anchorItem)) {
                scene.hoverCandidate = value;
                scene.hoverCandidateAnchor = anchorItem;
                hoverOpenTimer.restart();
            }
        } else {
            if (scene.hoverCandidate === value) {
                scene.hoverCandidate = "";
                scene.hoverCandidateAnchor = null;
                hoverOpenTimer.stop();
            }
            scene.scheduleHoverClose();
        }
    }

    function scheduleHoverClose() {
        if (!scene.prefs.hoverOpen || scene.cardKey === "" || scene.launcherOpen
                || scene.barHovered || cards.hovered) {
            hoverCloseTimer.stop();
            return;
        }
        hoverCloseTimer.restart();
    }

    function openSettings(page) {
        const pendingOpen = scene.hasPendingSurfaceRequest("settings");
        if (scene.settingsOpen || pendingOpen) {
            scene.cancelFullSurfaceRequest("settings");
            if (pendingOpen)
                scene.settingsSurfaceHeld = false;
            scene.settingsOpen = false;
            return;
        }
        let value = typeof page === "string" && page !== "" ? page : "bar";
        if (value === "general") value = "bar";
        if (value === "wave") value = "sound";
        scene.settingsPage = value;
        scene.closeCard();
        scene.settingsSurfaceHeld = true;
        scene.requestFullSurface("settings", function() {
            scene.settingsOpen = true;
        });
    }


    function closeSettings() {
        scene.cancelFullSurfaceRequest("settings");
        scene.settingsOpen = false;
    }


    // Opens settings on a page without toggling them shut (the IPC path).
    function showSettings(page) {
        if (scene.settingsOpen && typeof page === "string" && page !== "") {
            scene.settingsPage = page === "general" ? "bar"
                : page === "wave" ? "sound" : page;
            return;
        }
        if (!scene.settingsOpen && !scene.hasPendingSurfaceRequest("settings"))
            scene.openSettings(page);
    }

    // The output an IPC request lands on: the focused one, or the first screen
    // while the compositor has not reported focus.
    function requestOutput() {
        const screens = Quickshell.screens;
        for (let i = 0; i < screens.length; ++i) {
            if (screens[i] && screens[i].name === Wm.focusedOutput)
                return Wm.focusedOutput;
        }
        return screens.length > 0 && screens[0] ? screens[0].name : "";
    }

    Connections {
        target: S.Prefs
        function onSettingsRequested(page) {
            if (scene.screenName === "" || scene.screenName !== scene.requestOutput())
                return;
            if (page === "close")
                scene.closeSettings();
            else
                scene.showSettings(page);
        }
    }


    function toggleLauncher() {
        const state = scene.screen ? ShellState.forScreen(scene.screen) : null;
        if (state)
            state.launcherOpen = !state.launcherOpen;
    }

    // A launcher card asked for like any other card goes through the launcher
    // host, which owns the providers the sheet searches.
    function requestLauncher(open) {
        const state = scene.screen ? ShellState.forScreen(scene.screen) : null;
        if (state)
            state.launcherOpen = open === true;
    }

    function toggleOverview() {
        Wm.toggleOverview();
    }

    function toggleClipboard() {
        const state = scene.screen ? ShellState.forScreen(scene.screen) : null;
        if (state)
            state.clipboardOpen = !state.clipboardOpen;
    }

    function openAsk() {
        ShellState.requestSurface("ask", scene.screenName, undefined);
    }

    function openHub(section) {
        Bus.run(["hub", "open", section].join(" "));
    }

    // The launcher card is created before its field can take focus, and the
    // scope restore below lands on the card root, so hand focus on to the field.
    onCardKeyChanged: if (cardKey !== "") {
        settingsOpen = false;
        Qt.callLater(function() {
            focusSurface.forceActiveFocus();
            if (scene.cardKey === "launcher" && scene.launcherSheet)
                scene.launcherSheet.focusSearch();
        });
    }
    function syncVessel() {
        if (scene.vesselName === scene.screenName)
            return;
        V.Vessels.unregister(scene.vesselName, scene);
        scene.vesselName = scene.screenName;
        V.Vessels.register(scene.vesselName, scene);
    }
    onScreenNameChanged: syncVessel()
    onBarHoveredChanged: {
        if (barHovered)
            hoverCloseTimer.stop();
        else
            scheduleHoverClose();
    }
    onHoldVisibleChanged: reconcileAutoHide()
    onHideRequestedChanged: reconcileAutoHide()
    Component.onCompleted: {
        win.layoutSpan = scene.prefs.span;
        reconcileAutoHide();
        syncVessel();
    }
    Component.onDestruction: V.Vessels.unregister(scene.vesselName, scene)

    Timer {
        id: hoverOpenTimer
        interval: 350
        onTriggered: {
            const key = scene.hoverCandidate;
            const anchor = scene.hoverCandidateAnchor;
            scene.hoverCandidate = "";
            scene.hoverCandidateAnchor = null;
            if (key !== "" && scene.hoverStates[key] === true)
                scene.openFromHover(key, anchor);
        }
    }

    Timer {
        id: hoverCloseTimer
        interval: 450
        onTriggered: if (!scene.barHovered && !cards.hovered && !scene.launcherOpen)
            scene.closeCard()
    }

    Timer {
        id: surfaceReadyTimer
        interval: 16
        repeat: false
        onTriggered: scene.finishPendingSurfaceActions()
    }

    Timer {
        id: cardSurfaceReadyTimer
        interval: 16
        repeat: false
        onTriggered: scene.finishPendingCardAction()
    }

    C.Morph {
        id: morphAnimation
        target: win
        property: "morph"
        from: 0
        to: 1
    }

    Timer {
        id: hideTimer
        interval: 600
        onTriggered: if (scene.hideRequested && !scene.holdVisible)
            scene.autoHidden = true
    }

    Connections {
        target: cards
        function onHoveredChanged() {
            if (cards.hovered)
                hoverCloseTimer.stop();
            else
                scene.scheduleHoverClose();
        }
    }

    Connections {
        target: scene.prefs
        function onHoverOpenChanged() {
            if (!scene.prefs.hoverOpen) {
                hoverOpenTimer.stop();
                hoverCloseTimer.stop();
                scene.hoverCandidate = "";
            }
        }
        // The drops travel from where they are to the new span's live layout;
        // blending a snapshot keeps them moving while slot springs reshape it.
        function onSpanChanged() {
            const nextSpan = scene.prefs.span;
            scene.requestFullSurface("span", function() {
                win.morphFrom = {
                    lx: leftGeom.x, lw: leftGeom.width,
                    vx: voiceGeom.x, vw: voiceGeom.width,
                    rx: rightGeom.x, rw: rightGeom.width,
                    wx: scene.waveX, ww: scene.waveW
                };
                win.morph = 0;
                win.layoutSpan = nextSpan;
                morphAnimation.restart();
            });
        }

    }

    PanelWindow {
        id: win

        screen: scene.screen
        color: "transparent"
        exclusionMode: ExclusionMode.Normal
        exclusiveZone: scene.prefs.autoHide === "off" ? K.Style.barH : 0
        anchors { top: true; left: true; right: true }
        implicitHeight: frameLayer.extent
        WlrLayershell.layer: WlrLayer.Top
        WlrLayershell.keyboardFocus: scene.cardKey !== ""
            ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None
        WlrLayershell.namespace: "ryoku-bar"

        property string layoutSpan: "islands"
        property real morph: 1
        property var morphFrom: ({ lx: 0, lw: 0, vx: 0, vw: 0, rx: 0, rw: 0, wx: 0, ww: 0 })
        function morphed(from, to) {
            return from + (to - from) * morph;
        }
        readonly property bool islands: layoutSpan === "islands"
        readonly property bool fused: layoutSpan === "fused"
        readonly property bool full: layoutSpan === "full"
        readonly property bool needsFullHeight: scene.surfaceOpening
            || scene.settingsSurfaceHeld || scene.cardSurfaceHeld
            || morphAnimation.running

        readonly property real inset: 8 * K.Style.scale
        readonly property real clearance: 10 * K.Style.scale
        readonly property real pad: K.Style.barPad
        readonly property real leftW: leftLane.contentWidth
        readonly property real centerW: centerLane.contentWidth
        readonly property real rightW: rightLane.contentWidth + recInlineExtra
        readonly property real leftDropW: leftW > 0.001 ? leftW + pad * 2 : 0
        readonly property real voiceDropW: centerW > 0.001 ? centerW + pad * 2 : 0
        readonly property real rightDropW: rightW > 0.001 ? rightW + pad * 2 : 0
        // Islands and full reach the screen edges; the side drops carry `inset`
        // of extra paper there so their content keeps its distance from the edge.
        readonly property real edgeLeftW: leftDropW > 0 ? leftDropW + inset : 0
        readonly property real edgeRightW: rightDropW > 0 ? rightDropW + inset : 0

        // room is judged on the lanes alone, measuring with the inline mark in loops
        readonly property real recMarkW: recMark.implicitWidth
        readonly property real recBubbleW: recMarkW + 2 * (pad - K.Style.hitPad)
        readonly property real recInlineExtra: (recMarkW + K.Style.barGap) * recInlineGeom.grown
        readonly property real rightLaneDropW: rightLane.contentWidth > 0.001
            ? rightLane.contentWidth + pad * 2 : 0
        readonly property real recRoomEnd: fused ? width - inset
            : width - (rightLaneDropW > 0 ? rightLaneDropW + inset + bubbleGap : inset)
        readonly property real recAnchorEnd: fused
            ? (width + leftDropW + voiceDropW + rightLaneDropW) / 2
            : Math.min((width + voiceDropW) / 2, recRoomEnd)
        readonly property bool recInline: full
            || recAnchorEnd + bubbleGap + recBubbleW > recRoomEnd

        readonly property real leftNeededW: scene.laneNeeded(leftLane)
        readonly property real centerNeededW: scene.laneNeeded(centerLane)
        readonly property real rightNeededW: scene.laneNeeded(rightLane)
        readonly property real neededDrops: (leftNeededW > 0 ? leftNeededW + pad * 2 : 0)
            + (centerNeededW > 0 ? centerNeededW + pad * 2 : 0)
            + (rightNeededW > 0 ? rightNeededW + pad * 2 : 0)
        readonly property int visibleDropCount: (leftNeededW > 0 ? 1 : 0)
            + (centerNeededW > 0 ? 1 : 0) + (rightNeededW > 0 ? 1 : 0)
        readonly property real spanAllowance: islands
            ? Math.max(0, width - inset * 2
                - clearance * Math.max(0, visibleDropCount - 1))
            : full ? Math.max(0, width - (pad + inset) * 2) : width
        readonly property real overflow: Math.max(0, neededDrops
            - (fused ? Math.max(0, visibleDropCount - 1) : 0) - spanAllowance)

        readonly property real fusedW: leftDropW + voiceDropW + rightDropW
            - Math.max(0, (leftDropW > 0 ? 1 : 0)
                + (voiceDropW > 0 ? 1 : 0) + (rightDropW > 0 ? 1 : 0) - 1)
        readonly property real fusedStart: Math.max(0, (width - fusedW) / 2)
        readonly property real fusedVoiceX: fusedStart
            + (leftDropW > 0 ? leftDropW - 1 : 0)
        readonly property real fusedRightX: fusedVoiceX
            + (voiceDropW > 0 && rightDropW > 0 ? voiceDropW - 1 : voiceDropW)

        readonly property real islandsLeftEdge: leftDropW > 0 ? edgeLeftW + clearance : inset
        readonly property real islandsRightEdge: width
            - (rightDropW > 0 ? edgeRightW + clearance : inset)
        readonly property real islandsVoiceRoom: Math.max(0,
            islandsRightEdge - islandsLeftEdge)
        readonly property real islandsVoiceW: Math.min(voiceDropW, islandsVoiceRoom)
        readonly property real islandsVoiceX: {
            const ideal = (width - islandsVoiceW) / 2;
            return Math.max(islandsLeftEdge,
                Math.min(islandsRightEdge - islandsVoiceW, ideal));
        }

        readonly property real fullLeftContentX: pad + inset
        readonly property real fullRightContentX: width - pad - inset - rightW
        readonly property real fullCenterMin: fullLeftContentX + leftW
            + (leftW > 0 && centerW > 0 ? pad * 2 : 0)
        readonly property real fullCenterMax: fullRightContentX - centerW
            - (rightW > 0 && centerW > 0 ? pad * 2 : 0)
        readonly property real fullCenterContentX: Math.max(fullCenterMin,
            Math.min(fullCenterMax, (width - centerW) / 2))

        readonly property real leftTargetX: islands || full ? 0 : fusedStart
        readonly property real leftTargetW: islands || full ? edgeLeftW : leftDropW
        readonly property real voiceTargetX: full
            ? fullCenterContentX - pad : islands ? islandsVoiceX : fusedVoiceX
        readonly property real voiceTargetW: islands ? islandsVoiceW : voiceDropW
        readonly property real rightTargetX: islands || full
            ? width - edgeRightW : fusedRightX
        readonly property real rightTargetW: islands || full ? edgeRightW : rightDropW

        readonly property var voiceItem: {
            const slot = centerLane.slotFor("voice");
            return slot ? slot.moduleItem : null;
        }
        readonly property var voiceBubbles: voiceItem && voiceItem.bubbles
            ? voiceItem.bubbles : ({ left: [] })
        readonly property string leftBubbleKey: voiceBubbles.left
            && voiceBubbles.left.length > 0 ? String(voiceBubbles.left[0]) : ""
        readonly property real bubbleGap: 8 * K.Style.scale
        readonly property bool leftBubbleVisible: islands && leftBubbleKey !== ""
            && (leftTargetW <= 0.001 || voiceTargetX - bubbleGap - leftBubbleHost.width
                >= leftTargetX + leftTargetW + bubbleGap)

        readonly property rect framedFullBandRect: frameLayer.maskRect(0, 0,
            win.width, K.Style.barH)
        readonly property rect framedLeftRect: frameLayer.maskRect(leftGeom.x, 0,
            leftGeom.width, K.Style.barH)
        readonly property rect framedVoiceRect: frameLayer.maskRect(voiceGeom.x, 0,
            voiceGeom.width, K.Style.barH)
        readonly property rect framedRightRect: frameLayer.maskRect(rightGeom.x, 0,
            rightGeom.width, K.Style.barH)
        readonly property rect framedLeftBubbleRect: frameLayer.maskRect(
            leftBubbleGeom.x, 0, leftBubbleHost.width, K.Style.barH)
        readonly property rect framedRecRect: frameLayer.maskRect(
            scene.recordBubble.x, 0, scene.recordBubble.w, K.Style.barH)
        readonly property rect framedRevealRect: frameLayer.maskRect(0, 0,
            win.width, 4)

        mask: Region {
            Region {
                intersection: Intersection.Combine
                x: Math.round(win.framedFullBandRect.x)
                y: Math.round(win.framedFullBandRect.y)
                width: !scene.autoHidden && win.full
                    ? Math.round(win.framedFullBandRect.width) : 0
                height: Math.round(win.framedFullBandRect.height)
            }
            Region {
                intersection: Intersection.Combine
                x: Math.round(win.framedLeftRect.x)
                y: Math.round(win.framedLeftRect.y)
                width: !scene.autoHidden
                    ? Math.round(win.framedLeftRect.width) : 0
                height: Math.round(win.framedLeftRect.height)
            }
            Region {
                intersection: Intersection.Combine
                x: Math.round(win.framedVoiceRect.x)
                y: Math.round(win.framedVoiceRect.y)
                width: !scene.autoHidden
                    ? Math.round(win.framedVoiceRect.width) : 0
                height: Math.round(win.framedVoiceRect.height)
            }
            Region {
                intersection: Intersection.Combine
                x: Math.round(win.framedRightRect.x)
                y: Math.round(win.framedRightRect.y)
                width: !scene.autoHidden
                    ? Math.round(win.framedRightRect.width) : 0
                height: Math.round(win.framedRightRect.height)
            }
            Region {
                intersection: Intersection.Combine
                x: Math.round(win.framedLeftBubbleRect.x)
                y: Math.round(win.framedLeftBubbleRect.y)
                width: !scene.autoHidden && win.leftBubbleVisible
                    ? Math.round(win.framedLeftBubbleRect.width) : 0
                height: Math.round(win.framedLeftBubbleRect.height)
            }
            // the docked bubble stays clickable while the bar is hidden
            Region {
                intersection: Intersection.Combine
                x: Math.round(win.framedRecRect.x)
                y: Math.round(win.framedRecRect.y)
                width: scene.recordBubble.w > 0.5
                    ? Math.round(win.framedRecRect.width) : 0
                height: Math.round(win.framedRecRect.height)
            }
            Region {
                intersection: Intersection.Combine
                x: Math.round(win.framedRevealRect.x)
                y: Math.round(win.framedRevealRect.y)
                width: scene.prefs.autoHide !== "off"
                    ? Math.round(win.framedRevealRect.width) : 0
                height: scene.prefs.autoHide !== "off"
                    ? Math.round(win.framedRevealRect.height) : 0
            }
            // While a card is open the whole output below the bar is a dismiss
            // surface: a click anywhere off the card closes it. The card itself
            // sits above this region and swallows its own clicks.
            Region {
                intersection: Intersection.Combine
                x: 0
                y: K.Style.barH
                width: scene.cardKey !== "" ? win.width : 0
                height: scene.cardKey !== ""
                    ? Math.max(0, win.height - K.Style.barH) : 0
            }
        }


        FocusScope {
            id: focusSurface
            anchors.fill: parent
            focus: scene.cardKey !== ""


            StageFramedLayer {
                id: frameLayer
                interactive: false
                screenName: win.screen?.name ?? ""
                edge: "top"
                restThickness: Math.max(1, Math.min(win.screen?.height ?? 1,
                    win.needsFullHeight ? (win.screen?.height ?? 1)
                        : scene.restingSurfaceHeight))
                screenWidth: win.screen?.width ?? 1
                screenHeight: win.screen?.height ?? 1
                // An open card belongs to the real screen: it closes as the
                // bar steps into the card.
                onFramedChanged: if (framed) scene.closeCard()
            }

            // The card's dismiss backdrop: a click anywhere off the card closes
            // it. It sits under the rail and the card content, over the grown
            // window's full output below the bar.
            MouseArea {
                id: cardDismiss
                z: 0
                anchors.fill: parent
                enabled: scene.cardKey !== ""
                acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
                onPressed: mouse => {
                    mouse.accepted = true;
                    Qt.callLater(scene.closeCard);
                }
            }

            Shortcut {
                sequences: [StandardKey.Cancel]
                enabled: scene.cardKey !== ""
                onActivated: scene.closeCard()
            }

            // The card content rides the bar's own surface so its blob welds
            // into the drop it grows from (one window, one blob field). The
            // liquid paper is drawn by Body's cardShape; this is the content.
            C.CardHost {
                id: cards
                z: 2
                scene: scene
                body: liquidBody
                cardKey: scene.cardKey
                cardArgs: scene.cardArgs
            }

            Item {
                id: rail
                parent: frameLayer.rest
                z: 1
                x: 0
                y: scene.autoHidden ? -scene.hideLift : 0
                width: parent.width
                height: parent.height
                clip: true

                Behavior on y {
                    enabled: !K.Style.still
                    NumberAnimation {
                        duration: K.Style.spatialFast
                        easing.type: Easing.BezierSpline
                        easing.bezierCurve: K.Style.spatialFastCurve
                    }
                }

                Item {
                    id: leftGeom
                    x: win.morphed(win.morphFrom.lx, win.leftTargetX)
                    width: Math.max(0, win.morphed(win.morphFrom.lw, win.leftTargetW))
                    height: K.Style.barH
                    visible: width > 0.001
                }

                Item {
                    id: voiceGeom
                    x: win.morphed(win.morphFrom.vx, win.voiceTargetX)
                    width: Math.max(0, win.morphed(win.morphFrom.vw, win.voiceTargetW))
                    height: K.Style.barH
                    visible: width > 0.001
                }

                Item {
                    id: rightGeom
                    x: win.morphed(win.morphFrom.rx, win.rightTargetX)
                    width: Math.max(0, win.morphed(win.morphFrom.rw, win.rightTargetW))
                    height: K.Style.barH
                    visible: width > 0.001
                }

                Item {
                    id: leftBubbleGeom
                    property real bud: win.leftBubbleVisible ? 1 : 0
                    readonly property real grown: Math.max(0, bud)
                    x: voiceGeom.x - (win.bubbleGap + leftBubbleHost.width) * grown
                    width: leftBubbleHost.width * grown
                    height: K.Style.barH
                    Behavior on bud { enabled: !K.Style.still; C.Morph { duration: K.Style.spatial } }
                }

                Item {
                    id: recBubbleGeom
                    property real bud: scene.recShown && !win.recInline ? 1 : 0
                    readonly property real grown: Math.max(0, bud)
                    readonly property real anchorX: win.fused
                        ? scene.bandX + scene.bandW : voiceGeom.x + voiceGeom.width
                    x: anchorX + win.bubbleGap * grown
                    width: win.recBubbleW * grown
                    height: K.Style.barH
                    Behavior on bud { enabled: !K.Style.still; C.Morph { duration: K.Style.spatial } }
                }

                Item {
                    id: recInlineGeom
                    property real bud: scene.recShown && win.recInline ? 1 : 0
                    readonly property real grown: Math.max(0, bud)
                    Behavior on bud { enabled: !K.Style.still; C.Morph { duration: K.Style.spatial } }
                }

                C.Body {
                    id: liquidBody
                    anchors.fill: parent
                    prefs: scene.prefs
                    span: win.layoutSpan
                    leftDrop: scene.leftDrop
                    voiceDrop: scene.voiceDrop
                    rightDrop: scene.rightDrop
                    waveX: scene.waveX
                    waveW: scene.waveW
                    bubbleLeft: scene.bubbleLeft
                    recordBubble: scene.recordBubble
                    cardKey: scene.cardKey
                    originX: scene.anchoredOriginX
                    originW: scene.anchoredOriginW
                    cardW: cards.cardW
                    cardH: cards.cardH
                    cardMaxW: cards.maxW
                    launcherCard: cards.retainedKey === "launcher"
                    musicEdge: scene.cardKey === "launcher" && scene.launcherSheet !== null
                        && scene.launcherSheet.wantsMusicEdge === true
                    waveActive: !scene.autoHidden
                }

                Item {
                    id: fullBandHit
                    z: 0.5
                    width: win.width
                    height: K.Style.barH
                    visible: win.full
                    HoverHandler { id: fullBandHover }
                    MouseArea {
                        anchors.fill: parent
                        acceptedButtons: Qt.RightButton
                        onPressed: mouse => scene.openCard("controls", fullBandHit, ({}))
                    }
                }

                Item {
                    id: leftBarHit
                    z: 1
                    x: leftGeom.x
                    width: leftGeom.width
                    height: K.Style.barH
                    HoverHandler { id: leftBarHover }
                    MouseArea {
                        anchors.fill: parent
                        acceptedButtons: Qt.RightButton
                        onPressed: mouse => scene.openCard("controls", leftBarHit, ({}))
                    }
                }

                Item {
                    id: voiceBarHit
                    z: 1
                    x: voiceGeom.x
                    width: voiceGeom.width
                    height: K.Style.barH
                    HoverHandler { id: voiceBarHover }
                    MouseArea {
                        anchors.fill: parent
                        acceptedButtons: Qt.RightButton
                        onPressed: mouse => scene.openCard("controls", voiceBarHit, ({}))
                    }
                }

                Item {
                    id: rightBarHit
                    z: 1
                    x: rightGeom.x
                    width: rightGeom.width
                    height: K.Style.barH
                    HoverHandler { id: rightBarHover }
                    MouseArea {
                        anchors.fill: parent
                        acceptedButtons: Qt.RightButton
                        onPressed: mouse => scene.openCard("controls", rightBarHit, ({}))
                    }
                }

                C.Lane {
                    id: leftLane
                    z: 2
                    scene: scene
                    name: "left"
                    moduleIds: scene.prefs.lane("left")
                    x: scene.leftContentX
                    y: 0
                    width: Math.max(0, Math.min(win.leftW, leftGeom.width - K.Style.barPad * 2))
                    clip: contentWidth > width
                }

                C.Lane {
                    id: centerLane
                    z: 2
                    scene: scene
                    name: "center"
                    moduleIds: scene.prefs.lane("center")
                    x: scene.centerContentX
                    y: 0
                    width: Math.max(0, voiceGeom.width - K.Style.barPad * 2)
                    clip: contentWidth > width
                }

                C.Lane {
                    id: rightLane
                    z: 2
                    scene: scene
                    name: "right"
                    moduleIds: scene.prefs.lane("right")
                    x: scene.rightContentX
                    y: 0
                    width: Math.max(0, Math.min(contentWidth,
                        rightGeom.width - K.Style.barPad * 2 - win.recInlineExtra))
                    clip: contentWidth > width
                }

                C.BubbleHost {
                    id: leftBubbleHost
                    z: 2
                    scene: scene
                    bubbleKey: win.leftBubbleKey
                    shown: win.leftBubbleVisible
                    x: leftBubbleGeom.x
                    opacity: Math.max(0, Math.min(1, (leftBubbleGeom.width
                        / Math.max(1, leftBubbleHost.width) - 0.6) / 0.4))
                    y: 0
                    HoverHandler { id: leftBubbleHover }
                }

                Rec.RecordMark {
                    id: recMark
                    readonly property real grown: win.recInline
                        ? recInlineGeom.grown : recBubbleGeom.grown
                    z: 3
                    scene: scene
                    x: win.recInline ? rightGeom.x + K.Style.barPad
                        : recBubbleGeom.x + (K.Style.barPad - K.Style.hitPad)
                    y: scene.recDock
                    opacity: Math.max(0, Math.min(1, (grown - 0.6) / 0.4))
                    visible: grown > 0.001
                    HoverHandler { id: recHover }
                }

            }

            Item {
                id: hoverBand
                parent: frameLayer.rest
                z: 4
                anchors { top: parent.top; left: parent.left; right: parent.right }
                height: K.Style.barH
                visible: scene.prefs.autoHide !== "off"
                containmentMask: QtObject {
                    function contains(point: point): bool {
                        if (point.y < 0 || point.y > hoverBand.height)
                            return false;
                        if (point.y <= 4)
                            return true;
                        if (scene.autoHidden)
                            return false;
                        if (win.full)
                            return true;
                        const inLeft = point.x >= leftGeom.x
                            && point.x <= leftGeom.x + leftGeom.width;
                        const inVoice = point.x >= voiceGeom.x
                            && point.x <= voiceGeom.x + voiceGeom.width;
                        const inRight = point.x >= rightGeom.x
                            && point.x <= rightGeom.x + rightGeom.width;
                        const inRecord = scene.recordBubble.w > 0.5
                            && point.x >= scene.recordBubble.x
                            && point.x <= scene.recordBubble.x + scene.recordBubble.w;
                        return inLeft || inVoice || inRight || inRecord;
                    }
                }
                HoverHandler { id: bandHover }
            }
        }

        // The card surface releases once the melt finishes and no new card is
        // pending, so the bar window shrinks back to its resting strip.
        Connections {
            target: liquidBody
            function onCardProgressChanged() {
                if (liquidBody.cardProgress <= 0.001 && scene.cardKey === ""
                        && scene.pendingCardAction === null)
                    scene.cardSurfaceHeld = false;
            }
        }
    }

    S.SettingsWindow {
        scene: scene
        targetScreen: scene.screen
        open: scene.settingsOpen
        origin: scene.settingsOrigin
        page: scene.settingsPage
        onCloseRequested: scene.closeSettings()
        onClosed: scene.settingsSurfaceHeld = false
    }
}
