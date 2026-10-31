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
    readonly property bool surfaceOpening: pendingSurfaceActions.length > 0
    readonly property real restingSurfaceHeight: Math.ceil(Math.max(
        K.Style.barH + K.Style.s2 + Math.ceil(K.Style.label * 1.5)
            + K.Style.s3 + K.Style.s1,
        K.Style.barH + Math.max(0, Number(prefs.depth) || 0)
            + (prefs.form === "square" ? M.squareSmoothing : M.smoothing)
            + K.Style.border))


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
    readonly property var bubbleRight: ({
        x: rightBubbleGeom.x,
        w: rightBubbleGeom.width,
        visible: win.rightBubbleVisible,
        key: win.rightBubbleKey
    })
    // Side content sits against the drop's inner edge, so the extra paper an
    // edge-hugging drop carries stays on the screen side.
    readonly property real leftContentX: leftGeom.x
        + Math.max(K.Style.barPad, leftGeom.width - K.Style.barPad - win.leftW)
    readonly property real centerContentX: voiceGeom.x + K.Style.barPad
    readonly property real rightContentX: rightGeom.x + K.Style.barPad
    readonly property var currentOwnerDrop: cardOwner === "left" ? leftDrop
        : cardOwner === "right" ? rightDrop : voiceDrop
    readonly property real anchoredOriginX: currentOwnerDrop.w > 0
        ? currentOwnerDrop.x : fallbackOriginX
    readonly property real anchoredOriginW: currentOwnerDrop.w > 0
        ? currentOwnerDrop.w : fallbackOriginW

    readonly property bool barHovered: fullBandHover.hovered
        || leftBarHover.hovered || voiceBarHover.hovered || rightBarHover.hovered
        || leftBubbleHover.hovered || rightBubbleHover.hovered
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
            "tray", "power", "system", "session", "recording", "vpn"]
            .indexOf(normalKey(key)) >= 0;
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

    function requestCardSurface(action) {
        if (scene.cardSurfaceHeld) {
            action();
            return;
        }
        scene.cardSurfaceHeld = true;
        scene.pendingCardAction = action;
        cardSurfaceReadyTimer.restart();
    }

    function cancelCardSurfaceRequest() {
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
        return ["today", "media", "notifications", "recording"]
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
            if (cursor === leftLane)
                return "left";
            if (cursor === centerLane || cursor === leftBubbleHost
                    || cursor === rightBubbleHost)
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
            : name === "right" ? scene.rightDrop : scene.voiceDrop;
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
        if (!scene.validCardKey(value) || value === "tray")
            return;
        scene.markHoverSwitch(value);
        scene.captureOrigin(anchorItem);
        scene.requestCardSurface(function() {
            if (scene.hoverStates[value] !== true) {
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

    function hoverCard(key, anchorItem, hovered) {
        const value = normalKey(key);
        if (!scene.validCardKey(value) || value === "tray")
            return;
        const states = Object.assign({}, scene.hoverStates);
        states[value] = hovered === true;
        scene.hoverStates = states;
        if (hovered) {
            hoverCloseTimer.stop();
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
        if (!scene.prefs.hoverOpen || scene.cardKey === ""
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


    function toggleLauncher() {
        const state = scene.screen ? ShellState.forScreen(scene.screen) : null;
        if (state)
            state.launcherOpen = !state.launcherOpen;
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
        Spawn.run(["ryoku-shell", "hub", "open", section]);
    }

    onCardKeyChanged: if (cardKey !== "") {
        settingsOpen = false;
        Qt.callLater(function() { focusSurface.forceActiveFocus(); });
    }
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
    }

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
        onTriggered: if (!scene.barHovered && !cards.hovered) scene.closeCard()
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
        WlrLayershell.keyboardFocus: WlrKeyboardFocus.None
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
            || scene.settingsSurfaceHeld || morphAnimation.running

        readonly property real inset: 8 * K.Style.scale
        readonly property real clearance: 10 * K.Style.scale
        readonly property real pad: K.Style.barPad
        readonly property real leftW: leftLane.contentWidth
        readonly property real centerW: centerLane.contentWidth
        readonly property real rightW: rightLane.contentWidth
        readonly property real leftDropW: leftW > 0.001 ? leftW + pad * 2 : 0
        readonly property real voiceDropW: centerW > 0.001 ? centerW + pad * 2 : 0
        readonly property real rightDropW: rightW > 0.001 ? rightW + pad * 2 : 0
        // Islands and full reach the screen edges; the side drops carry `inset`
        // of extra paper there so their content keeps its distance from the edge.
        readonly property real edgeLeftW: leftDropW > 0 ? leftDropW + inset : 0
        readonly property real edgeRightW: rightDropW > 0 ? rightDropW + inset : 0

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
            ? voiceItem.bubbles : ({ left: [], right: [] })
        readonly property string leftBubbleKey: voiceBubbles.left
            && voiceBubbles.left.length > 0 ? String(voiceBubbles.left[0]) : ""
        readonly property string rightBubbleKey: voiceBubbles.right
            && voiceBubbles.right.length > 0 ? String(voiceBubbles.right[0]) : ""
        readonly property real bubbleGap: 8 * K.Style.scale
        readonly property bool leftBubbleVisible: islands && leftBubbleKey !== ""
            && (leftTargetW <= 0.001 || voiceTargetX - bubbleGap - leftBubbleHost.width
                >= leftTargetX + leftTargetW + bubbleGap)
        readonly property bool rightBubbleVisible: islands && rightBubbleKey !== ""
            && (rightTargetW <= 0.001 || voiceTargetX + voiceTargetW + bubbleGap
                + rightBubbleHost.width <= rightTargetX - bubbleGap)

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
        readonly property rect framedRightBubbleRect: frameLayer.maskRect(
            rightBubbleGeom.x, 0, rightBubbleHost.width, K.Style.barH)
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
            Region {
                intersection: Intersection.Combine
                x: Math.round(win.framedRightBubbleRect.x)
                y: Math.round(win.framedRightBubbleRect.y)
                width: !scene.autoHidden && win.rightBubbleVisible
                    ? Math.round(win.framedRightBubbleRect.width) : 0
                height: Math.round(win.framedRightBubbleRect.height)
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

            Item {
                id: rail
                parent: frameLayer.rest
                z: 1
                x: 0
                y: scene.autoHidden ? -(K.Style.barH + 2) : 0
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
                    id: rightBubbleGeom
                    property real bud: win.rightBubbleVisible ? 1 : 0
                    readonly property real grown: Math.max(0, bud)
                    x: voiceGeom.x + voiceGeom.width + win.bubbleGap * grown
                    width: rightBubbleHost.width * grown
                    height: K.Style.barH
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
                    bubbleRight: scene.bubbleRight
                    cardKey: scene.cardKey
                    originX: scene.anchoredOriginX
                    originW: scene.anchoredOriginW
                    cardW: cards.cardW
                    cardH: cards.cardH
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
                    width: Math.max(0, Math.min(win.rightW, rightGeom.width - K.Style.barPad * 2))
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

                C.BubbleHost {
                    id: rightBubbleHost
                    z: 2
                    scene: scene
                    bubbleKey: win.rightBubbleKey
                    shown: win.rightBubbleVisible
                    x: rightBubbleGeom.x
                    opacity: Math.max(0, Math.min(1, (rightBubbleGeom.width
                        / Math.max(1, rightBubbleHost.width) - 0.6) / 0.4))
                    y: 0
                    HoverHandler { id: rightBubbleHover }
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
                        return inLeft || inVoice || inRight;
                    }
                }
                HoverHandler { id: bandHover }
            }
        }
    }

    // Cards use their own full-output dismiss surface so opening one never
    // resizes the bar's layer surface and invalidates its visible buffer.
    PanelWindow {
        id: cardWindow

        visible: scene.cardSurfaceHeld
        screen: scene.screen
        color: "transparent"
        exclusionMode: ExclusionMode.Ignore
        exclusiveZone: 0
        anchors { top: true; bottom: true; left: true; right: true }
        WlrLayershell.layer: WlrLayer.Top
        WlrLayershell.keyboardFocus: scene.cardKey !== ""
            ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None
        WlrLayershell.namespace: "ryoku-bar-card"

        mask: Region {
            Region {
                width: scene.cardKey !== "" ? cardWindow.width : 0
                height: scene.cardKey !== "" ? cardWindow.height : 0
            }
            Region {
                intersection: Intersection.Subtract
                width: cardWindow.width
                height: K.Style.barH
            }
        }

        FocusScope {
            anchors.fill: parent
            focus: scene.cardKey !== ""

            Shortcut {
                sequences: [StandardKey.Cancel]
                enabled: scene.cardKey !== ""
                onActivated: scene.closeCard()
            }

            MouseArea {
                z: 0
                anchors.fill: parent
                enabled: scene.cardKey !== ""
                acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
                onPressed: mouse => {
                    mouse.accepted = true;
                    Qt.callLater(scene.closeCard);
                }
            }

            Item {
                id: cardPaper

                z: 1
                x: liquidBody.cardX
                y: liquidBody.cardY
                width: liquidBody.cardCurW
                height: liquidBody.cardCurH
                visible: liquidBody.cardActive
                opacity: liquidBody.opacity

                BlobGroup {
                    id: cardLiquid
                    color: K.Style.base
                    smoothing: liquidBody.smoothing
                    borderWidth: scene.prefs.rim ? 1 : 0
                    borderColor: K.Style.rim
                    shadowStrength: 0
                    shadowSize: 0
                }

                BlobRect {
                    group: cardLiquid
                    width: cardPaper.width
                    height: cardPaper.height
                    topLeftRadius: liquidBody.leftOverhang ? liquidBody.radius : 0
                    topRightRadius: liquidBody.rightOverhang ? liquidBody.radius : 0
                    bottomLeftRadius: liquidBody.edgeRadius(
                        liquidBody.cardX, liquidBody.cardCurW, "left")
                    bottomRightRadius: liquidBody.edgeRadius(
                        liquidBody.cardX, liquidBody.cardCurW, "right")
                    stiffness: 180
                    damping: 16
                    deformScale: 0
                    sinks: false
                }
            }

            C.CardHost {
                id: cards
                z: 2
                scene: scene
                body: liquidBody
                cardKey: scene.cardKey
                cardArgs: scene.cardArgs
            }
        }

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
