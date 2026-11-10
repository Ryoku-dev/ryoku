pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import shell.services
import Ryoku.Ui.Singletons
import "look"
import "cards"
import "recording.js" as Recording
import "discordVoice/layout.js" as VoiceLayout

// One passive overlay per output, above a fullscreen game. Only the strip and
// the widgets take the pointer; the rest falls through. No keyboard focus.
PanelWindow {
    id: win

    required property var modelData

    readonly property string mon: modelData ? modelData.name : ""
    readonly property var cfg: Config.gamebar
    readonly property real us: Tokens.uiScaleFor(win.mon) * win.cfg.scale
    readonly property int fadeMs: Motion.dur(win.cfg.fade)

    readonly property var catalogue: [
        { id: "capture", label: I18n.tr("Capture"), symbol: "screenshot_region" },
        { id: "performance", label: I18n.tr("Performance"), symbol: "speed" },
        { id: "resources", label: I18n.tr("Resources"), symbol: "monitoring" },
        { id: "tasks", label: I18n.tr("Tasks"), symbol: "checklist" },
        { id: "media", label: I18n.tr("Media"), symbol: "music_note" },
        { id: "voice", label: I18n.tr("Voice"), symbol: "headset_mic", glyph: "discord", minimal: true },
        { id: "audio", label: I18n.tr("Audio"), symbol: "volume_up" },
        { id: "gallery", label: I18n.tr("Gallery"), symbol: "photo_library" }
    ]

    readonly property var cards: GameBarStore.visibleCards
    // A saved monitor that is no longer connected (unplugged, or gone before a
    // restart) must not hide pinned cards: follow the focused output instead.
    // While the output list is still empty at startup, trust the saved name so
    // the cards do not flash on the wrong screen before it loads.
    readonly property bool mine: {
        const saved = GameBarStore.monitor;
        if (saved.length === 0)
            return Wm.focusedOutput === win.mon;
        if (Wm.outputs.length === 0 || Wm.outputByName(saved) !== null)
            return saved === win.mon;
        return Wm.focusedOutput === win.mon;
    }
    readonly property bool active: win.cfg.enabled && win.mine
        && (GameBarStore.open || GameBarStore.pinned.length > 0)

    // Placement uses the widest and tallest so a grid never overlaps.
    readonly property var cardSizes: ({
        capture: { w: 350, h: 240 },
        performance: { w: 340, h: 356 },
        resources: { w: 350, h: 200 },
        tasks: { w: 350, h: 270 },
        media: { w: 358, h: 235 },
        voice: { w: 358, h: 336 },
        audio: { w: 350, h: 282 },
        gallery: { w: 350, h: 290 }
    })
    readonly property real maxCardW: 358 * win.us
    readonly property real maxCardH: 356 * win.us
    readonly property real gap: 12 * win.us
    readonly property real gridStep: win.cfg.snapGrid * win.us
    readonly property real gridLine: win.cfg.gridStep * win.us
    readonly property real topPad: 56 * win.us
    readonly property real sidePad: 16 * win.us
    readonly property real stripH: 48 * win.us

    function symbolFor(id) {
        for (let i = 0; i < win.catalogue.length; i++) {
            if (win.catalogue[i].id === id)
                return win.catalogue[i].symbol;
        }
        return "";
    }

    // Only cards that can resize carry limits; the rest keep their drawn size.
    readonly property var cardLimits: ({
        voice: { minW: 300, minH: 280, maxW: 640, maxH: 640 }
    })

    function glyphFor(id) {
        for (let i = 0; i < win.catalogue.length; i++) {
            if (win.catalogue[i].id === id && win.catalogue[i].glyph)
                return win.catalogue[i].glyph;
        }
        return "";
    }

    function limitsFor(id) {
        return win.cardLimits[id] || { minW: 220, minH: 160, maxW: 800, maxH: 700 };
    }

    function minimalCard(id) {
        for (let i = 0; i < win.catalogue.length; i++) {
            if (win.catalogue[i].id === id)
                return win.catalogue[i].minimal === true;
        }
        return false;
    }

    // A pinned minimal card shrinks to what it draws, so it can meet a screen
    // edge instead of stopping where the full card would.
    function compactSizeFor(id) {
        if (!win.minimalCard(id))
            return null;
        const avatars = Math.max(1, Math.min(8, win.cfg.voiceAvatars || 8));
        const count = Math.min(avatars, DiscordVoice.participantCount);
        const avatar = VoiceLayout.avatar(win.cfg.voiceAvatarSize);
        const names = win.cfg.voiceNames !== false;
        return VoiceLayout.size(win.fullSizeFor(id).w / win.us, count, avatar, names);
    }

    function fullSizeFor(id) {
        const e = win.cardSizes[id] || { w: 320, h: 300 };
        const saved = GameBarStore.sizeFor(id);
        const lim = win.limitsFor(id);
        const w = Math.max(lim.minW, Math.min(lim.maxW,
            saved && typeof saved.w === "number" ? saved.w : e.w));
        const h = Math.max(lim.minH, Math.min(lim.maxH,
            saved && typeof saved.h === "number" ? saved.h : e.h));
        return { w: w * win.us, h: h * win.us };
    }

    function sizeFor(id) {
        if (id === win.resizeCard)
            return { w: win.liveW, h: win.liveH };
        if (!GameBarStore.open && GameBarStore.isPinned(id)) {
            const compact = win.compactSizeFor(id);
            if (compact)
                return { w: compact.w * win.us, h: compact.h * win.us };
        }
        return win.fullSizeFor(id);
    }

    property bool live: false
    property var shownCards: []
    property int count: win.shownCards.length

    property bool suppressed: false
    property bool sawSelecting: false
    // The visibility hold and the recording lifecycle are separate: reopening
    // the bar during a recording releases only the hold, so the end of the
    // recording can still restore the state it had before it started.
    property bool hidingForRecording: false
    property bool recordingCycle: false
    property bool reopenAfterRecording: false

    // A pinned card outlives the strip: the strip leaves, the cards stay. During
    // a full close the strip is held for the fade-out and then reset.
    property bool stripUp: GameBarStore.open

    // Free placement. liveX/liveY hold the dragged card's spot so only that
    // slot repaints; GameBarStore keeps the settled spot across reloads.
    property real liveX: 0
    property real liveY: 0
    property string dragCard: ""
    property string settlingCard: ""
    property real dragFromX: 0
    property real dragFromY: 0

    property real liveW: 0
    property real liveH: 0
    property string resizeCard: ""
    property real resizeFromW: 0
    property real resizeFromH: 0

    function shown(id) {
        return win.shownCards.indexOf(id) !== -1;
    }

    function clampTo(size, x, y) {
        const maxX = Math.max(0, win.width - size.w);
        const maxY = Math.max(0, win.height - size.h);
        return {
            x: Math.round(Math.max(0, Math.min(maxX, x))),
            y: Math.round(Math.max(0, Math.min(maxY, y)))
        };
    }

    function clampPos(id, x, y) {
        return win.clampTo(win.sizeFor(id), x, y);
    }

    // With snapping on, a released card settles onto the nearest edge or centre
    // line; otherwise it stays where it was let go.
    function snapPosition(id, x, y) {
        const size = win.sizeFor(id);
        if (!win.cfg.snap)
            return win.clampPos(id, x, y);

        const reach = win.cfg.snapReach * win.us;
        const xs = [
            0,
            Math.round((win.width - size.w) / 2),
            win.width - size.w
        ];
        const ys = [
            0,
            Math.round((win.height - size.h) / 2),
            win.topPad + win.stripH + win.gap,
            win.height - size.h
        ];

        for (let i = 0; i < win.shownCards.length; i++) {
            const other = win.shownCards[i];
            if (other === id)
                continue;
            const o = win.positionFor(other);
            const os = win.sizeFor(other);
            xs.push(o.x, o.x + os.w, o.x - size.w, o.x + os.w - size.w);
            ys.push(o.y, o.y + os.h, o.y - size.h, o.y + os.h - size.h);
        }

        let sx = x;
        let sy = y;
        let dx = reach;
        let dy = reach;
        let foundX = false;
        let foundY = false;
        for (let i = 0; i < xs.length; i++) {
            const d = Math.abs(x - xs[i]);
            if (d <= dx) {
                dx = d;
                sx = xs[i];
                foundX = true;
            }
        }
        for (let i = 0; i < ys.length; i++) {
            const d = Math.abs(y - ys[i]);
            if (d <= dy) {
                dy = d;
                sy = ys[i];
                foundY = true;
            }
        }

        if (!foundX && win.gridStep > 0)
            sx = Math.round(x / win.gridStep) * win.gridStep;
        if (!foundY && win.gridStep > 0)
            sy = Math.round(y / win.gridStep) * win.gridStep;

        return win.clampPos(id, sx, sy);
    }

    function defaultPosFor(id, size) {
        const list = win.shownCards;
        const i = Math.max(0, list.indexOf(id));
        const n = Math.max(1, list.length);
        const cols = Math.min(4, Math.max(1, Math.floor(
            (win.width - win.sidePad * 2 + win.gap) / (win.maxCardW + win.gap))));
        const used = Math.min(n, cols);
        const gridW = used * win.maxCardW + Math.max(0, used - 1) * win.gap;
        const col = i % cols;
        const row = Math.floor(i / cols);
        const x = (win.width - gridW) / 2 + col * (win.maxCardW + win.gap);
        const y = win.topPad + win.stripH + win.gap + row * (win.maxCardH + win.gap);
        return win.clampTo(size, x, y);
    }

    function defaultPos(id) {
        return win.defaultPosFor(id, win.sizeFor(id));
    }

    // The pinned card is placed against its full frame. When it shrinks, anchor
    // the compact panel to the screen edge the full card sits against, or centre
    // it on the full card when that is floating clear of every edge.
    function compactPositionFor(id) {
        const saved = GameBarStore.positionFor(id);
        if (!saved || typeof saved.x !== "number" || typeof saved.y !== "number")
            return win.defaultPosFor(id, win.sizeFor(id));
        const full = win.fullSizeFor(id);
        const compact = win.sizeFor(id);
        // A saved spot can belong to a monitor that is gone or resized; clamp
        // the full frame it was placed on before anchoring the compact panel.
        const frame = win.clampTo(full, saved.x, saved.y);
        const rightEdge = frame.x + full.w;
        const bottomEdge = frame.y + full.h;
        const reach = 120 * win.us;
        const right = win.width - rightEdge <= reach;
        const left = frame.x <= reach;
        const bottom = win.height - bottomEdge <= reach;
        const top = frame.y <= reach;
        const x = right ? rightEdge - compact.w
            : (left ? frame.x : frame.x + (full.w - compact.w) / 2);
        const y = bottom ? bottomEdge - compact.h
            : (top ? frame.y : frame.y + (full.h - compact.h) / 2);
        return win.clampPos(id, x, y);
    }

    function positionFor(id) {
        if (id === win.dragCard)
            return { x: win.liveX, y: win.liveY };
        if (!GameBarStore.open && GameBarStore.isPinned(id)
                && win.minimalCard(id))
            return win.compactPositionFor(id);
        const saved = GameBarStore.positionFor(id);
        if (saved && typeof saved.x === "number" && typeof saved.y === "number")
            return win.clampPos(id, saved.x, saved.y);
        return win.defaultPos(id);
    }

    function beginDrag(id) {
        const p = win.positionFor(id);
        // A recent drop still has its spring armed; grabbing again must not
        // drag through the settle animation.
        win.settlingCard = "";
        settleTimer.stop();
        win.liveX = p.x;
        win.liveY = p.y;
        win.dragFromX = p.x;
        win.dragFromY = p.y;
        win.dragCard = id;
    }

    function dragBy(id, dx, dy) {
        if (win.dragCard !== id)
            return;
        const p = win.clampPos(id, win.dragFromX + dx, win.dragFromY + dy);
        win.liveX = p.x;
        win.liveY = p.y;
    }

    function endDrag(id) {
        if (win.dragCard !== id)
            return;
        const settled = win.snapPosition(id, win.liveX, win.liveY);
        // Store the settled spot while liveX/liveY still pin the card, then
        // clear the drag so the slot's spring animates into place.
        GameBarStore.place(id, settled.x, settled.y);
        win.settlingCard = id;
        settleTimer.restart();
        win.dragCard = "";
    }

    function beginResize(id) {
        const size = win.sizeFor(id);
        win.settlingCard = "";
        settleTimer.stop();
        win.liveW = size.w;
        win.liveH = size.h;
        win.resizeFromW = size.w;
        win.resizeFromH = size.h;
        win.resizeCard = id;
    }

    function resizeBy(id, dx, dy) {
        if (win.resizeCard !== id)
            return;
        const lim = win.limitsFor(id);
        win.liveW = Math.round(Math.max(lim.minW * win.us,
            Math.min(lim.maxW * win.us, win.resizeFromW + dx)));
        win.liveH = Math.round(Math.max(lim.minH * win.us,
            Math.min(lim.maxH * win.us, win.resizeFromH + dy)));
    }

    function endResize(id) {
        if (win.resizeCard !== id)
            return;
        GameBarStore.resize(id, win.liveW / win.us, win.liveH / win.us);
        win.resizeCard = "";
        // positionFor() clamps, so compare the clamped spot with the stored one
        // to persist the nudge a larger card needs at a screen edge.
        const saved = GameBarStore.positionFor(id);
        if (saved && typeof saved.x === "number" && typeof saved.y === "number") {
            const clamped = win.clampPos(id, saved.x, saved.y);
            if (clamped.x !== saved.x || clamped.y !== saved.y)
                GameBarStore.place(id, clamped.x, clamped.y);
        }
    }

    // A pinned card is a picture once the bar closes: no mask rect, so the
    // whole surface falls through and it takes no pointer or keyboard focus.
    function maskRect(id) {
        // While dragging, one full-surface region covers every card, so the per
        // card rects stay at zero and the mask never changes between moves.
        if (win.dragCard !== "" || !win.shown(id))
            return { x: 0, y: 0, w: 0, h: 0 };
        if (!win.takingInput)
            return { x: 0, y: 0, w: 0, h: 0 };
        const p = win.positionFor(id);
        const size = win.sizeFor(id);
        return { x: Math.round(p.x), y: Math.round(p.y), w: Math.round(size.w), h: Math.round(size.h) };
    }

    Connections {
        target: GameBarStore
        function onOpenChanged() {
            if (GameBarStore.open)
                win.stripUp = true;
            else if (GameBarStore.pinned.length > 0)
                win.stripUp = false;
        }
    }

    // Turning it off in the Hub closes it at once, leaving nothing the shortcut
    // and the strip can no longer reach.
    Connections {
        target: Config
        function onGamebarChanged() {
            if (!win.cfg.enabled)
                GameBarStore.closeBar();
        }
    }

    onActiveChanged: {
        if (win.active) {
            exitTimer.stop();
            win.live = true;
            win.shownCards = win.cards;
        } else {
            exitTimer.restart();
        }
    }
    // A transient deactivation during startup must not latch the surface off.
    onWantedChanged: {
        if (!win.wanted)
            return;
        exitTimer.stop();
        win.live = true;
        if (win.shownCards.length === 0)
            win.shownCards = win.cards;
    }
    onCardsChanged: if (win.active) win.shownCards = win.cards
    Component.onCompleted: {
        win.live = win.active;
        if (win.active)
            win.shownCards = win.cards;
        if (!win.cfg.enabled)
            GameBarStore.closeBar();
    }

    readonly property bool wanted: win.active && !win.suppressed
    // Input follows the open state, not the fade: a closing bar hands the
    // pointer back at once while the strip may still be fading out. A pinned
    // card left after close takes nothing.
    readonly property bool takingInput: win.wanted && GameBarStore.open

    screen: modelData
    visible: win.live && !win.suppressed
    color: "transparent"
    // A negative exclusive zone makes the surface span the whole output, over
    // the bar and the dock.
    exclusiveZone: -1
    exclusionMode: ExclusionMode.Normal
    anchors { top: true; left: true; right: true; bottom: true }
    WlrLayershell.layer: WlrLayer.Overlay
    WlrLayershell.namespace: "ryoku-gamebar"
    WlrLayershell.keyboardFocus: WlrKeyboardFocus.None

    mask: Region {
        Region {
            x: strip.x; y: strip.y
            width: win.takingInput ? strip.width : 0
            height: win.takingInput ? strip.height : 0
            intersection: Intersection.Combine
        }
        Region {
            readonly property var rect: win.maskRect("capture")
            x: rect.x; y: rect.y; width: rect.w; height: rect.h
            intersection: Intersection.Combine
        }
        Region {
            readonly property var rect: win.maskRect("performance")
            x: rect.x; y: rect.y; width: rect.w; height: rect.h
            intersection: Intersection.Combine
        }
        Region {
            readonly property var rect: win.maskRect("resources")
            x: rect.x; y: rect.y; width: rect.w; height: rect.h
            intersection: Intersection.Combine
        }
        Region {
            readonly property var rect: win.maskRect("tasks")
            x: rect.x; y: rect.y; width: rect.w; height: rect.h
            intersection: Intersection.Combine
        }
        Region {
            readonly property var rect: win.maskRect("media")
            x: rect.x; y: rect.y; width: rect.w; height: rect.h
            intersection: Intersection.Combine
        }
        Region {
            readonly property var rect: win.maskRect("voice")
            x: rect.x; y: rect.y; width: rect.w; height: rect.h
            intersection: Intersection.Combine
        }
        Region {
            readonly property var rect: win.maskRect("audio")
            x: rect.x; y: rect.y; width: rect.w; height: rect.h
            intersection: Intersection.Combine
        }
        Region {
            readonly property var rect: win.maskRect("gallery")
            x: rect.x; y: rect.y; width: rect.w; height: rect.h
            intersection: Intersection.Combine
        }
        // A dragged or resized card must not let the pointer fall through to
        // the bar, the dock or the game, so the surface takes the whole screen.
        Region {
            x: 0; y: 0
            width: (win.dragCard !== "" || win.resizeCard !== "") ? win.width : 0
            height: (win.dragCard !== "" || win.resizeCard !== "") ? win.height : 0
            intersection: Intersection.Combine
        }
    }

    // Opening fades in once the surface is mapped instead of popping on.
    property bool faded: false
    property real reveal: win.faded && win.wanted ? 1 : 0
    Behavior on reveal {
        NumberAnimation {
            duration: win.fadeMs
            easing.type: Motion.rowRevealCurve
        }
    }

    onBackingWindowVisibleChanged: {
        if (win.backingWindowVisible) {
            win.faded = false;
            fadeIn.restart();
        } else {
            win.faded = false;
        }
    }

    Timer {
        id: fadeIn
        interval: 40
        onTriggered: win.faded = true
    }

    Timer {
        id: exitTimer
        interval: Math.max(1, win.fadeMs)
        onTriggered: {
            if (win.wanted) {
                win.live = true;
                return;
            }
            win.live = false;
            win.stripUp = GameBarStore.open;
            if (!GameBarStore.open)
                win.shownCards = [];
        }
    }

    Rectangle {
        id: scrim
        anchors.fill: parent
        color: "#000000"
        opacity: (win.stripUp ? win.cfg.dim : 0) * win.reveal

        Behavior on opacity {
            NumberAnimation {
                duration: win.fadeMs
                easing.type: Motion.rowRevealCurve
            }
        }
    }

    // The guide grid appears only while a dragged card can snap.
    Item {
        id: grid
        anchors.fill: parent
        visible: win.dragCard !== "" && win.cfg.snap && win.cfg.guideGrid
        opacity: win.reveal

        readonly property color line: Qt.rgba(GameBarLook.accent.r, GameBarLook.accent.g, GameBarLook.accent.b, win.cfg.gridAlpha)

        Repeater {
            model: grid.visible ? Math.ceil(win.width / win.gridLine) + 1 : 0

            delegate: Rectangle {
                required property int index
                x: index * win.gridLine
                width: 1
                height: grid.height
                color: grid.line
            }
        }

        Repeater {
            model: grid.visible ? Math.ceil(win.height / win.gridLine) + 1 : 0

            delegate: Rectangle {
                required property int index
                y: index * win.gridLine
                width: grid.width
                height: 1
                color: grid.line
            }
        }
    }

    // Cards paint under the strip, and each keeps one slot for the surface's
    // life so switching widgets on never re-creates the ones already up.
    Repeater {
        model: win.catalogue

        delegate: Item {
            id: slot
            required property var modelData

            readonly property string cardId: slot.modelData.id
            readonly property bool on: win.shown(slot.cardId)
            readonly property var size: win.sizeFor(slot.cardId)
            readonly property var pos: win.positionFor(slot.cardId)

            visible: slot.on
            width: size.w
            height: size.h
            x: slot.pos.x
            y: slot.pos.y
            opacity: win.reveal
            // Closed with pins up, a card is a picture: no hover, no drag, no
            // cursor change, and the empty mask hands the pointer to the game.
            enabled: win.stripUp

            // The spring belongs to a drop only; a reflow places cards at once.
            Behavior on x {
                enabled: !Motion.reduce && win.settlingCard === slot.cardId
                SpringAnimation {
                    spring: win.cfg.spring
                    damping: win.cfg.damping
                    mass: win.cfg.mass
                    epsilon: 0.4 * win.us
                }
            }
            Behavior on y {
                enabled: !Motion.reduce && win.settlingCard === slot.cardId
                SpringAnimation {
                    spring: win.cfg.spring
                    damping: win.cfg.damping
                    mass: win.cfg.mass
                    epsilon: 0.4 * win.us
                }
            }

            Loader {
                id: cardLoader
                anchors.fill: parent
                active: slot.on && win.live
                sourceComponent: win.componentFor(slot.cardId)
            }
        }
    }

    Timer {
        id: settleTimer
        interval: 900
        onTriggered: win.settlingCard = ""
    }

    GameBarStrip {
        id: strip
        s: win.us
        visible: win.live
        enabled: win.stripUp
        x: Math.round((win.width - width) / 2)
        y: win.topPad
        opacity: win.stripUp ? win.reveal : 0
        cards: win.catalogue
        activeIds: win.shownCards
        snapping: win.cfg.snap
        onPicked: id => GameBarStore.toggleCard(id)
        onSnapToggled: Config.setGameBar("snap", !win.cfg.snap)

        Behavior on opacity {
            NumberAnimation {
                duration: win.fadeMs
                easing.type: Motion.rowRevealCurve
            }
        }
    }

    function componentFor(id) {
        switch (id) {
        case "capture": return captureComp;
        case "performance": return performanceComp;
        case "resources": return resourcesComp;
        case "tasks": return tasksComp;
        case "media": return mediaComp;
        case "voice": return voiceComp;
        case "audio": return audioComp;
        case "gallery": return galleryComp;
        }
        return null;
    }

    // The bar must not be in its own capture: unmap the surface, run the
    // existing shot path, and restore after the selection overlay finishes.
    function shoot(mode) {
        if (Capture.selecting !== "")
            return;
        restoreDelay.stop();
        // The recording watcher keeps running: a shot taken during the countdown
        // must still end the cycle when that countdown is cancelled. It stops
        // itself as soon as the recorder is actually active.
        win.sawSelecting = false;
        win.suppressed = true;
        // A shot taken during a take must not put the bar back into the rest of
        // the recording. Hold it until the take ends; opening the bar again is
        // the explicit request to show it.
        if (win.recordingCycle) {
            win.hidingForRecording = true;
            if (GameBarStore.open)
                GameBarStore.closeBar();
        }
        Capture.shoot(mode);
        if (mode === "all" || Capture.selecting === "")
            win.restoreAfter(Capture.delay * 1000 + 800);
    }

    function startRecording() {
        if (Recorder.anyActive || Recorder.countingDown) {
            Recorder.stop();
            return;
        }
        restoreDelay.stop();
        recordStartFallback.stop();
        const plan = Recording.begin(GameBarStore.open);
        win.reopenAfterRecording = plan.reopen;
        win.recordingCycle = true;
        win.hidingForRecording = true;
        win.suppressed = true;
        if (plan.closeBar)
            GameBarStore.closeBar();
        Recorder.startAfter(Recorder.recordArgs(), 1);
        recordStartFallback.restart();
    }

    function endRecordingSuppression() {
        if (!win.recordingCycle)
            return;
        const plan = Recording.finish(win.recordingCycle, win.reopenAfterRecording,
            GameBarStore.open);
        win.recordingCycle = plan.cycle;
        win.hidingForRecording = plan.hold;
        recordStartFallback.stop();
        // Restore the pre-recording state, even when the bar was reopened while
        // the recording ran.
        if (plan.openBar)
            GameBarStore.openBar(GameBarStore.monitor);
        else if (plan.closeBar)
            GameBarStore.closeBar();
        win.reopenAfterRecording = false;
        win.restoreAfter(900);
    }

    // The shortcut must still reach the Capture card while a recording runs:
    // opening the bar is an explicit choice to show it in the capture. Only the
    // hold is released; the fallback keeps watching so a cancelled countdown
    // still ends the cycle.
    function releaseRecordingHold() {
        if (!Recording.release(win.hidingForRecording, win.recordingCycle))
            return;
        win.hidingForRecording = false;
        restoreDelay.stop();
        win.suppressed = false;
    }

    function restoreAfter(ms) {
        restoreDelay.interval = Math.max(900, ms);
        restoreDelay.restart();
    }

    // Follow the selection overlay itself instead of polling for it: a fast
    // click can open and close the overlay between two timer ticks, and a
    // selection that never reaches the source must not leave the bar hidden.
    Connections {
        target: Capture
        function onSelectingChanged() {
            if (!win.suppressed)
                return;
            if (Capture.selecting !== "") {
                win.sawSelecting = true;
                return;
            }
            if (!win.sawSelecting)
                return;
            win.sawSelecting = false;
            win.restoreAfter(Capture.delay * 1000 + 700);
        }
    }

    // Opening the bar during a recording is an explicit request for the Capture
    // card, so let go of the hold instead of keeping the surface unreachable.
    Connections {
        target: GameBarStore
        function onOpenChanged() {
            if (GameBarStore.open)
                win.releaseRecordingHold();
        }
    }

    // A running capture would otherwise record the bar: hold suppression until
    // Recorder reports the capture ended. The fallback keeps checking so a
    // cancelled countdown cannot leave the bar hidden.
    Connections {
        target: Recorder
        function onAnyActiveChanged() {
            if (!win.recordingCycle)
                return;
            if (Recorder.anyActive) {
                recordStartFallback.stop();
                return;
            }
            win.endRecordingSuppression();
        }
    }

    Timer {
        id: recordStartFallback
        interval: 1500
        repeat: true
        onTriggered: {
            if (!win.recordingCycle || Recorder.anyActive) {
                stop();
                return;
            }
            if (!Recorder.countingDown)
                win.endRecordingSuppression();
        }
    }

    Timer {
        id: restoreDelay
        // A shot hides the surface briefly. Put it back the way the recording
        // cycle had it: hidden while the recording hold is up, visible when the
        // shortcut reopened the bar during the take.
        onTriggered: win.suppressed = win.hidingForRecording
    }

    Component {
        id: captureComp
        CaptureCard { s: win.us; host: win; cardId: "capture" }
    }
    Component {
        id: performanceComp
        PerformanceCard { s: win.us; host: win; cardId: "performance"; monitorName: win.mon }
    }
    Component {
        id: resourcesComp
        ResourcesCard { s: win.us; host: win; cardId: "resources" }
    }
    Component {
        id: tasksComp
        TasksCard { s: win.us; host: win; cardId: "tasks" }
    }
    Component {
        id: mediaComp
        MediaCard { s: win.us; host: win; cardId: "media" }
    }
    Component {
        id: voiceComp
        VoiceCard { s: win.us; host: win; cardId: "voice"; minimalWhenPinned: win.minimalCard("voice") }
    }
    Component {
        id: audioComp
        AudioCard { s: win.us; host: win; cardId: "audio" }
    }
    Component {
        id: galleryComp
        GalleryCard { s: win.us; host: win; cardId: "gallery" }
    }
}
