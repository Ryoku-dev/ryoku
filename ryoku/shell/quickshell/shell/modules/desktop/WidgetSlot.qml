pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Effects
import Qt5Compat.GraphicalEffects
import "Singletons"
import "../stage/Singletons" as StageCfg

// placement / shape / interaction frame for one desktop widget. measures its
// single child's natural size, pads it, positions it either at a compass
// zone (a fixed margin in from that edge) or, once dragged, at a free
// monitor pixel. draws the chosen backing (none|card|glass). carries the
// desktop interaction: drag to move (grid-snapped, with a press bump),
// right-click for the menu, and a per-widget lock that freezes everything.
// dragging persists a "free" position to the widgets Config; the menu and
// Ryoku Settings write the same file. the grip takes the press over the
// desktop catcher beneath it, so a right-click on the widget opens its
// menu, not the desktop one.
Item {
    id: slot
    enabled: !slot.stageInputBlocked

    property string widget: "clock"            // config prefix, for persistence
    property string monitor: ""                 // output owning this slot
    property string anchor: "top-left"         // auto | 9 zones | free
    property real freeX: 72
    property real freeY: 64
    property bool locked: false
    property real pad: 0
    property string bg: "none"                 // none | card | glass
    property real radiusOverride: -1           // -1 keeps Theme.radius
    property real radius: slot.radiusOverride >= 0 ? slot.radiusOverride : Theme.radius
    property real gridSize: 32
    property bool snapEnabled: true
    property real zoneMargin: 64
    property real scaleCfg: 1
    property real backingOpacity: -1
    property real borderWidth: -1
    property real borderOpacity: -1
    property bool composing: false
    property var stageController: null
    property real depthV: 0.5
    property bool spatial: false             // desktop's stageSpatial gate
    property real mouseNX: 0                 // shared smoothed cursor, -1..1
    property real mouseNY: 0
    readonly property bool stageInputBlocked: !!(slot.stageController
        && slot.stageController.inputBlocked)
    readonly property real stageFramingDim: slot.stageController
        ? slot.stageController.framingDim : 0

    signal menuRequested(real x, real y, string widget)
    signal dropped(rect box)
    signal resized()

    // The placement a gesture (drag, corner resize, Ctrl+wheel scale) started
    // from, captured on press / first wheel. The desktop reads it when the
    // gesture commits, pushes it into the Stage Editor's undo stack, and
    // clears it, so a drag or resize walks back exactly like the reference's
    // own canvas does.
    property var gestureBefore: null
    property bool groupDragging: false
    property real groupX: 0
    property real groupY: 0
    property real groupDragMinX: -Infinity
    property real groupDragMaxX: Infinity
    property real groupDragMinY: -Infinity
    property real groupDragMaxY: Infinity
    property bool stageMoveActive: false
    property real stageMoveGrabX: 0
    property real stageMoveGrabY: 0
    readonly property string storeMonitor: (slot.composing || Config.isForked(slot.monitor))
        ? slot.monitor : ""
    function _captureGesture() {
        if (slot.gestureBefore !== null)
            return;
        slot.gestureBefore = {
            Anchor: Config.get(slot.widget + "Anchor", slot.monitor),
            X: Config.get(slot.widget + "X", slot.monitor),
            Y: Config.get(slot.widget + "Y", slot.monitor),
            Scale: Config.get(slot.widget + "Scale", slot.monitor),
            Locked: Config.get(slot.widget + "Locked", slot.monitor)
        };
    }
    function _setFree(x, y) {
        const patch = {};
        patch[slot.widget + "Anchor"] = "free";
        patch[slot.widget + "X"] = x;
        patch[slot.widget + "Y"] = y;
        Config.setManyFor(slot.storeMonitor, patch);
    }
    function _quantiseScale(value, modifiers) {
        const bounded = Math.max(0.5, Math.min(2.5, value));
        if (Boolean(modifiers & Qt.ShiftModifier))
            return Math.round(bounded * 100) / 100;
        if (Math.abs(bounded - 1) < 0.03)
            return 1;
        return Math.round(bounded / 0.05) * 0.05;
    }
    function _commitScale(value, x, y) {
        const next = Math.max(0.5, Math.min(2.5, value));
        const patch = {};
        patch[slot.widget + "Anchor"] = "free";
        patch[slot.widget + "X"] = Math.round(x);
        patch[slot.widget + "Y"] = Math.round(y);
        patch[slot.widget + "Scale"] = next;
        Config.setManyFor(slot.storeMonitor, patch);
        slot.liveScale = next;
        slot.resized();
    }
    function stageSetScale(value) {
        if (slot.locked)
            return;
        slot._captureGesture();
        slot._commitScale(slot._quantiseScale(value, Qt.NoModifier), slot.x, slot.y);
        guard.restart();
    }
    function stageToggleLock() {
        slot._captureGesture();
        Config.setFor(slot.storeMonitor, slot.widget + "Locked", !slot.locked);
        slot.resized();
    }
    function stageBeginMove(point, modifiers) {
        if (slot.locked || slot.stageInputBlocked)
            return false;
        slot.stageMoveActive = true;
        slot._captureGesture();
        slot.stageMoveGrabX = point.x - slot.x;
        slot.stageMoveGrabY = point.y - slot.y;
        slot.dragX = slot.x;
        slot.dragY = slot.y;
        if (slot.composing && slot.stageController) {
            const bounds = slot.stageController.widgetDragStarted(slot.widget);
            if (bounds.active === false) {
                slot.stageMoveActive = false;
                slot.gestureBefore = null;
                return false;
            }
            slot.groupDragMinX = bounds.minX;
            slot.groupDragMaxX = bounds.maxX;
            slot.groupDragMinY = bounds.minY;
            slot.groupDragMaxY = bounds.maxY;
        }
        return true;
    }
    function stageUpdateMove(point, modifiers) {
        if (!slot.stageMoveActive || slot.locked)
            return;
        const nx = point.x - slot.stageMoveGrabX;
        const ny = point.y - slot.stageMoveGrabY;
        if (!slot.dragging) {
            if (Math.abs(nx - slot.x) < 6 && Math.abs(ny - slot.y) < 6)
                return;
            slot.dragging = true;
        }
        slot.dragX = Math.max(slot.groupDragMinX,
            Math.min(slot.groupDragMaxX, slot.clampX(nx)));
        slot.dragY = Math.max(slot.groupDragMinY,
            Math.min(slot.groupDragMaxY, slot.clampY(ny)));
        if (slot.composing && slot.stageController) {
            const bounded = slot.stageController.widgetDragMoved(
                slot.widget, slot.dragX, slot.dragY, modifiers);
            slot.dragX = bounded.x;
            slot.dragY = bounded.y;
        }
    }
    function stageEndMove(modifiers) {
        if (!slot.stageMoveActive)
            return;
        if (slot.dragging) {
            const fx = Math.round(Math.max(slot.groupDragMinX,
                Math.min(slot.groupDragMaxX, slot.composing
                    ? slot.dragX : slot.snap(slot.dragX))));
            const fy = Math.round(Math.max(slot.groupDragMinY,
                Math.min(slot.groupDragMaxY, slot.composing
                    ? slot.dragY : slot.snap(slot.dragY))));
            const handled = slot.composing && slot.stageController
                && slot.stageController.widgetDragEnded(
                    slot.widget, fx, fy, modifiers);
            if (!handled) {
                slot._setFree(fx, fy);
                slot.dropped(Qt.rect(fx, fy, slot.width, slot.height));
            } else {
                slot.gestureBefore = null;
            }
            slot.dragging = false;
            guard.restart();
        } else {
            if (slot.composing && slot.stageController)
                slot.stageController.widgetDragCancelled(slot.widget);
            slot.gestureBefore = null;
        }
        slot.stageMoveActive = false;
        slot.groupDragMinX = -Infinity;
        slot.groupDragMaxX = Infinity;
        slot.groupDragMinY = -Infinity;
        slot.groupDragMaxY = Infinity;
    }
    function stageCancelMove() {
        if (!slot.stageMoveActive)
            return;
        if (slot.composing && slot.stageController)
            slot.stageController.widgetDragCancelled(slot.widget);
        slot.dragging = false;
        slot.stageMoveActive = false;
        slot.gestureBefore = null;
        slot.groupDragMinX = -Infinity;
        slot.groupDragMaxX = Infinity;
        slot.groupDragMinY = -Infinity;
        slot.groupDragMaxY = Infinity;
    }
    function stageBeginResize(corner, point, modifiers) {
        if (slot.locked || slot.stageInputBlocked)
            return;
        slot._captureGesture();
        slot.resizeCorner = corner;
        slot.resizeStartScale = slot.effectiveScale;
        slot.resizeStartWidth = slot.width;
        slot.resizeStartHeight = slot.height;
        slot.resizeUnderL = slot.underL;
        slot.resizeStartX = slot.x;
        slot.resizeStartY = slot.y;
        slot.resizeOppX = corner.indexOf("l") >= 0 ? slot.x + slot.width : slot.x;
        slot.resizeOppY = corner.indexOf("t") >= 0 ? slot.y + slot.height : slot.y;
        slot.resizeStartDiag = Math.max(1,
            Math.hypot(point.x - slot.resizeOppX, point.y - slot.resizeOppY));
        slot.dragX = slot.x;
        slot.dragY = slot.y;
        slot.liveScale = slot.scaleCfg;
        slot.resizeMoved = false;
        slot.resizing = true;
    }
    function stageUpdateResize(point, modifiers) {
        if (!slot.resizing)
            return;
        slot.resizeMoved = true;
        const distance = Math.max(1,
            Math.hypot(point.x - slot.resizeOppX, point.y - slot.resizeOppY));
        slot.liveScale = slot._quantiseScale(
            slot.resizeStartScale * distance / slot.resizeStartDiag, modifiers);
        const factor = slot.liveScale / Math.max(0.001, slot.resizeStartScale);
        const nextW = slot.resizeStartWidth * factor;
        const nextH = slot.resizeStartHeight * factor;
        slot.dragX = slot.resizeCorner.indexOf("l") >= 0
            ? slot.resizeOppX - nextW : slot.resizeOppX;
        slot.dragY = slot.resizeCorner.indexOf("t") >= 0
            ? slot.resizeOppY - nextH : slot.resizeOppY;
    }
    function stageEndResize() {
        if (!slot.resizing)
            return;
        if (!slot.resizeMoved) {
            slot.resizing = false;
            slot.gestureBefore = null;
            return;
        }
        const x = slot.clampX(slot.dragX);
        const y = slot.clampY(slot.dragY);
        slot._commitScale(slot.liveScale, x, y);
        slot.resizing = false;
        guard.restart();
    }
    function stageCancelResize() {
        slot.resizing = false;
        slot.liveScale = slot.scaleCfg;
        slot.gestureBefore = null;
    }
    function stageResetScale() {
        if (!slot.locked)
            slot.stageSetScale(1);
    }
    function stagePreviewPosition(x, y) {
        if (!slot.groupDragging) {
            slot.groupX = slot.x;
            slot.groupY = slot.y;
            slot.groupDragging = true;
        }
        slot.groupX = x;
        slot.groupY = y;
    }
    function stageEndPreview() {
        slot.groupDragging = false;
    }
    default property alias content: holder.data

    readonly property var item: holder.children.length > 0 ? holder.children[0] : null
    readonly property real cw: slot.item ? slot.item.implicitWidth : 0
    readonly property real ch: slot.item ? slot.item.implicitHeight : 0
    // true while the hosted widget wants the keyboard (its own `editing` flag).
    // the host raises the layer's keyboard grab off this, the same way plugin
    // tiles do; the clock never exposes it, so it stays input-passive.
    readonly property bool editing: slot.visible && !!(slot.item && slot.item.editing)
    readonly property bool contentManagesOwnEffects: slot.item?.managesOwnEffects === true

    // custom ink: a bg:none widget can wear a pinned colour or an A->B sweep in
    // place of its adaptive ink. The solid colour is pushed into the widget so it
    // paints only the ink (text/marks), never a card; a gradient is layered on top
    // via the mask, but only for card-less widgets (calendar/music/aio keep their
    // card, so they take the solid colour, never the mask). Empty = adaptive.
    readonly property string inkColorA: slot.bg === "none" ? (Config.get(slot.widget + "Color") || "") : ""
    readonly property string inkColorB: Config.get(slot.widget + "Color2") || ""
    readonly property bool cardWidget: slot.widget === "calendar" || slot.widget === "music" || slot.widget === "aio"
    readonly property bool inkGradient: slot.inkColorA !== ""
        && (Config.get(slot.widget + "Gradient") === true)
        && slot.inkColorB !== ""
    readonly property bool inkMaskOn: slot.inkGradient && !slot.cardWidget

    // drag state. while holding (dragging, or briefly after release until
    // the config write lands) the rendered position follows the drag so it
    // doesn't flicker back to the old anchor for a frame.
    property bool dragging: false
    property real dragX: 0
    property real dragY: 0
    property bool resizing: false
    property bool resizeMoved: false
    property real liveScale: scaleCfg
    property string resizeCorner: "br"
    property real resizeStartScale: 1
    property real resizeStartDiag: 1
    property real resizeStartWidth: 1
    property real resizeStartHeight: 1
    property real resizeUnderL: 50
    property real resizeStartX: 0
    property real resizeStartY: 0
    property real resizeOppX: 0
    property real resizeOppY: 0
    readonly property real effectiveScale: (slot.resizing
        || scalePersist.running || guard.running) ? slot.liveScale : slot.scaleCfg
    readonly property real previewFactor: slot.effectiveScale
        / Math.max(0.001, slot.scaleCfg)
    readonly property bool holding: slot.dragging || slot.resizing
        || slot.groupDragging || guard.running

    onScaleCfgChanged: if (!slot.resizing && !scalePersist.running)
        slot.liveScale = slot.scaleCfg

    width: Math.max(1, (slot.cw + slot.pad * 2) * slot.previewFactor)
    height: Math.max(1, (slot.ch + slot.pad * 2) * slot.previewFactor)

    // A slot's parent is the Loader that hosts it; before the desktop window
    // has been laid out that parent momentarily reports width/height 0. Clamping
    // a saved position against a zero parent collapses every widget to (0,0) and
    // they stack in the corner (#251), so treat a non-positive parent as "not
    // laid out yet" and pass the requested value through untouched; the binding
    // re-resolves to the clamped position once the parent has a real size.
    function clampX(v) {
        const w = slot.parent ? slot.parent.width : 0;
        if (w <= 0)
            return v;
        return Math.max(0, Math.min(v, w - slot.width));
    }
    function clampY(v) {
        const h = slot.parent ? slot.parent.height : 0;
        if (h <= 0)
            return v;
        return Math.max(0, Math.min(v, h - slot.height));
    }
    function snap(v) { return slot.snapEnabled ? Math.round(v / slot.gridSize) * slot.gridSize : v; }
    function zoneX() {
        const w = slot.parent ? slot.parent.width : 0;
        if (w <= 0)
            return slot.anchor.indexOf("left") >= 0 ? slot.zoneMargin : 0;
        if (slot.anchor.indexOf("left") >= 0) return slot.zoneMargin;
        if (slot.anchor.indexOf("right") >= 0) return w - slot.width - slot.zoneMargin;
        return (w - slot.width) / 2;
    }
    function zoneY() {
        const h = slot.parent ? slot.parent.height : 0;
        if (h <= 0)
            return slot.anchor.indexOf("top") >= 0 ? slot.zoneMargin : 0;
        if (slot.anchor.indexOf("top") >= 0) return slot.zoneMargin;
        if (slot.anchor.indexOf("bottom") >= 0) return h - slot.height - slot.zoneMargin;
        return (h - slot.height) / 2;
    }

    // anchor:"auto" -> the wallpaper's calmest patch. calmSpot returns a
    // screen-normalised top-left for a box of the slot's normalised size; scale
    // it back to monitor pixels. calmSpot reads the daemon's tone map internally,
    // so this binding tracks it and re-resolves -- gliding via the x/y Behaviors
    // below -- whenever a new wallpaper publishes a fresh map. marginN is the zone
    // pixel inset against the shorter screen axis, so an auto widget stays at
    // least as far off every edge as a zoned one.
    readonly property point autoPoint: {
        // only auto slots pay for calmSpot (and subscribe to the tone map);
        // a zoned or free slot must not re-lay-out on a wallpaper change.
        if (slot.anchor !== "auto")
            return Qt.point(0, 0);
        const pw = slot.parent ? slot.parent.width : 0;
        const ph = slot.parent ? slot.parent.height : 0;
        if (pw <= 0 || ph <= 0)
            return Qt.point(slot.freeX, slot.freeY);
        const s = Scheme.calmSpot(slot.width / pw, slot.height / ph,
            slot.zoneMargin / Math.min(pw, ph));
        return Qt.point(slot.clampX(s.x * pw), slot.clampY(s.y * ph));
    }

    x: slot.groupDragging ? slot.groupX
        : slot.holding ? slot.dragX
        : slot.anchor === "free" ? slot.clampX(slot.freeX)
        : slot.anchor === "auto" ? slot.autoPoint.x : slot.zoneX()
    y: slot.groupDragging ? slot.groupY
        : slot.holding ? slot.dragY
        : slot.anchor === "free" ? slot.clampY(slot.freeY)
        : slot.anchor === "auto" ? slot.autoPoint.y : slot.zoneY()

    Behavior on x { enabled: !slot.holding; NumberAnimation { duration: 240; easing.type: Easing.OutCubic } }
    Behavior on y { enabled: !slot.holding; NumberAnimation { duration: 240; easing.type: Easing.OutCubic } }


    opacity: Math.max(0.2, Math.min(1, Config.get(slot.widget + "Opacity")))
        * (1 - 0.75 * slot.stageFramingDim) * (slot._hovered ? 0.85 : 1)
    Behavior on opacity { NumberAnimation { duration: 240; easing.type: Easing.OutCubic } }

    Timer { id: guard; interval: 90 }

    // the tone the hosted widget's ink actually sits on: the wallpaper under
    // this slot, or the backing plate composited over it. pushed into the
    // widget, which has no way to find out where on the screen it landed.
    readonly property real underL: {
        if (slot.resizing)
            return slot.resizeUnderL;
        const pw = slot.parent ? slot.parent.width : 0;
        const ph = slot.parent ? slot.parent.height : 0;
        if (pw <= 0 || ph <= 0)
            return Scheme.wallLstar;
        const l = Scheme.lstarAt(slot.x / pw, slot.y / ph, slot.width / pw, slot.height / ph);
        return slot.bg === "none" ? l : Scheme.overLstar(l, backing.color);
    }
    Binding {
        target: slot.item
        property: "underL"
        value: slot.underL
        when: slot.item !== null && slot.item.underL !== undefined
    }
    // push the pinned solid colour into the widget so it paints its own ink (never
    // a card). "" leaves the widget on its adaptive inkOn(underL).
    Binding {
        target: slot.item
        property: "inkColorA"
        value: slot.inkColorA
        when: slot.item !== null && slot.item.inkColorA !== undefined
    }

    readonly property bool _sceneOn: slot.spatial && !slot.composing
    readonly property real _gainRaw: slot.spatial
            && StageCfg.Config.followMouse("spatial")
        ? StageCfg.Config.amountFactor("spatial")
            * StageCfg.Config.sensitivity("spatial")
            * StageCfg.Config.range("spatial") : 0
    property real _gain: slot._gainRaw
    Behavior on _gain { SmoothedAnimation { velocity: 2.5; duration: 200 } }
    readonly property real _pivot: StageCfg.StageBackend.pivotDistanceFor(slot.monitor)
    readonly property real _sceneH: slot.parent ? slot.parent.height : 0
    readonly property bool _hovered: slot._sceneOn && slotHover.hovered
        && !slot.holding && !slot.editing
    property var _scene: slot._sceneOn
        ? StageCfg.StageBackend.widgetSceneAt(
            slot.depthV, slot._pivot, slot.mouseNX, slot.mouseNY, slot._gain,
            slot.x + slot.width / 2, slot.y + slot.height / 2,
            slot.width, slot.height, slot.parent.width, slot._sceneH)
        : null
    readonly property real _sceneDx: slot._scene ? slot._scene.dx : 0
    readonly property real _sceneDy: slot._scene ? slot._scene.dy : 0
    readonly property real _sceneScale: slot._scene ? slot._scene.scale : 1
    readonly property real _nearness: Math.max(0, Math.min(1, slot.depthV))
    readonly property real _shadowAngle: StageCfg.Config.shadowAngle("spatial") * Math.PI / 180
    readonly property real _shadowSpan: slot._sceneOn ? 4 + 16 * slot._nearness : 6
    transform: [
        Scale {
            origin.x: slot.width / 2
            origin.y: slot.height / 2
            xScale: slot._sceneScale
            yScale: slot._sceneScale
        },
        Translate {
            x: slot._sceneDx
            y: slot._sceneDy
        }
    ]

    MultiEffect {
        source: backing
        anchors.fill: backing
        visible: !slot.resizing && !Performance.shadowsDisabled && slot.bg !== "none"
        shadowEnabled: true
        shadowColor: slot._sceneOn
            ? Qt.rgba(0, 0, 0, 0.30 + 0.40 * slot._nearness)
            : Theme.shadow
        shadowBlur: slot._sceneOn ? 0.80 + 0.20 * slot._nearness : 1.0
        shadowScale: slot._sceneOn ? 0.96 + 0.14 * slot._nearness : 1.0
        shadowHorizontalOffset: Math.round(slot._sceneOn
            ? Math.cos(slot._shadowAngle) * slot._shadowSpan : 0)
        shadowVerticalOffset: Math.round(slot._sceneOn
            ? Math.sin(slot._shadowAngle) * slot._shadowSpan : 6)
        blurMax: 32
        autoPaddingEnabled: true
    }

    Rectangle {
        id: backing
        anchors.fill: parent
        visible: slot.bg !== "none"
        radius: slot.radius
        // A floating widget plate is pure-black paper with a bone hairline
        // (docs/ui-ux.md): black reads on any wallpaper, and the hairline follows
        // the palette ink. Glass is the same plate, thinner, under the sheen.
        color: Qt.rgba(0, 0, 0, slot.backingOpacity >= 0 ? slot.backingOpacity
            : slot.bg === "card" ? 0.5 : 0.32)
        border.width: slot.borderWidth >= 0 ? slot.borderWidth : 1
        border.color: slot.borderOpacity >= 0
            ? Qt.rgba(Theme.ink.r, Theme.ink.g, Theme.ink.b, slot.borderOpacity)
            : slot.bg === "card" ? Theme.line
            : Qt.rgba(Theme.ink.r, Theme.ink.g, Theme.ink.b, 0.12)

        // glass sheen: faint top-down highlight so the panel reads as a pane
        // of glass rather than a flat fill.
        Rectangle {
            visible: slot.bg === "glass"
            anchors.fill: parent
            radius: parent.radius
            gradient: Gradient {
                GradientStop { position: 0.0; color: Qt.rgba(Theme.ink.r, Theme.ink.g, Theme.ink.b, 0.07) }
                GradientStop { position: 0.5; color: "transparent" }
                GradientStop { position: 1.0; color: Qt.rgba(0, 0, 0, 0.05) }
            }
        }
    }

    // Outside compose mode the grip stays under the content, preserving every
    // widget control. The compose frame calls the same move API from above.
    MouseArea {
        id: grip
        anchors.fill: parent
        acceptedButtons: Qt.LeftButton | Qt.RightButton
        enabled: !slot.composing && !slot.stageInputBlocked
        hoverEnabled: true
        cursorShape: slot.locked ? Qt.ArrowCursor
            : (slot.dragging ? Qt.ClosedHandCursor : Qt.OpenHandCursor)

        onPressed: mouse => {
            if (mouse.button === Qt.RightButton) {
                Config.selectMonitor(slot.monitor, false);
                const position = grip.mapToItem(slot.parent, mouse.x, mouse.y);
                slot.menuRequested(position.x, position.y, slot.widget);
                return;
            }
            const point = grip.mapToItem(slot.parent, mouse.x, mouse.y);
            slot.stageBeginMove(point, mouse.modifiers);
        }
        onPositionChanged: mouse => {
            const point = grip.mapToItem(slot.parent, mouse.x, mouse.y);
            slot.stageUpdateMove(point, mouse.modifiers);
        }
        onReleased: mouse => slot.stageEndMove(mouse.modifiers)
        onCanceled: slot.stageCancelMove()
    }

    // lift a bare widget off the wallpaper for legibility on any backdrop. a
    // card/glass panel already gives contrast, so the shadow only applies
    // when the widget sits directly on the wallpaper.
    Item {
        id: holder
        x: slot.pad * slot.previewFactor
        y: slot.pad * slot.previewFactor
        width: slot.cw
        height: slot.ch
        transform: Scale {
            origin.x: 0
            origin.y: 0
            xScale: slot.previewFactor
            yScale: slot.previewFactor
        }
        layer.enabled: !slot.contentManagesOwnEffects
            && !slot.resizing
            && (slot.inkMaskOn || (!Performance.shadowsDisabled && slot.bg === "none"))
        layer.smooth: true
        layer.effect: slot.inkMaskOn ? recolorFx : shadowFx
    }

    // the bare-widget shadow, and the ink recolour: a gradient (or a solid, both
    // stops equal) masked by the live content's alpha via LinearGradient's source,
    // so only the glyph shapes wear the pinned colour, never the box. Auto uses the
    // plain shadow and the widget's own adaptive inkOn(underL).
    Component {
        id: shadowFx
        MultiEffect {
            shadowEnabled: true
            shadowColor: slot._sceneOn
                ? Qt.rgba(0, 0, 0, 0.32 + 0.36 * slot._nearness)
                : Qt.rgba(0, 0, 0, 0.5)
            shadowBlur: slot._sceneOn ? 0.70 + 0.30 * slot._nearness : 0.8
            shadowScale: slot._sceneOn ? 0.97 + 0.10 * slot._nearness : 1.0
            shadowHorizontalOffset: Math.round(slot._sceneOn
                ? Math.cos(slot._shadowAngle) * (2 + 8 * slot._nearness) : 0)
            shadowVerticalOffset: Math.round(slot._sceneOn
                ? Math.sin(slot._shadowAngle) * (2 + 8 * slot._nearness) : 2)
            blurMax: 28
            autoPaddingEnabled: true
        }
    }
    Component {
        id: recolorFx
        LinearGradient {
            start: Qt.point(0, 0)
            end: Qt.point(0, height)
            gradient: Gradient {
                GradientStop { position: 0; color: slot.inkColorA }
                GradientStop { position: 1; color: slot.inkGradient ? slot.inkColorB : slot.inkColorA }
            }
        }
    }

    // hover state for the slot and its children, so the resize handle stays
    // lit while you reach across to it.
    HoverHandler {
        id: slotHover

        onPointChanged: {
            if (!slot._sceneOn)
                return;
            const w = slot.parent ? slot.parent.width : 0;
            const h = slot.parent ? slot.parent.height : 0;
            if (w <= 0 || h <= 0)
                return;
            StageCfg.StageBackend.setCursor(slot.monitor,
                point.scenePosition.x / w * 2 - 1,
                point.scenePosition.y / h * 2 - 1);
        }
    }

    // Wheel scaling previews locally and commits once after the gesture settles.
    WheelHandler {
        enabled: !slot.locked && !slot.stageInputBlocked
        acceptedModifiers: Qt.ControlModifier
        onWheel: event => {
            if (slot.gestureBefore === null)
                slot._captureGesture();
            const step = event.angleDelta.y > 0 ? 0.05 : -0.05;
            slot.liveScale = slot._quantiseScale(slot.effectiveScale + step,
                event.modifiers);
            slot.dragX = slot.x;
            slot.dragY = slot.y;
            scalePersist.restart();
        }
    }
    Timer {
        id: scalePersist
        interval: 350
        onTriggered: {
            slot._commitScale(slot.liveScale, slot.x, slot.y);
            guard.restart();
        }
    }

    // Outside the editor the familiar bottom-right grip remains available.
    // Edit mode supplies four counter-scaled grips in StageOutline.
    Item {
        id: handle
        width: 22
        height: 22
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        opacity: (((slotHover.hovered && !slot.composing) && !slot.locked
            && !slot.dragging) || (slot.resizing && !slot.composing)) ? 1 : 0
        visible: opacity > 0
        Behavior on opacity { NumberAnimation { duration: 120 } }

        Rectangle {
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            width: 13
            height: 2
            radius: Theme.radius
            color: (hgrip.containsMouse || slot.resizing) ? Theme.accent : Theme.faint
        }
        Rectangle {
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            width: 2
            height: 13
            radius: Theme.radius
            color: (hgrip.containsMouse || slot.resizing) ? Theme.accent : Theme.faint
        }

        MouseArea {
            id: hgrip
            anchors.fill: parent
            enabled: !slot.locked && !slot.stageInputBlocked
            acceptedButtons: Qt.LeftButton
            hoverEnabled: true
            cursorShape: Qt.SizeFDiagCursor
            onPressed: mouse => {
                const p = hgrip.mapToItem(slot.parent, mouse.x, mouse.y);
                slot.stageBeginResize("br", p, mouse.modifiers);
            }
            onPositionChanged: mouse => {
                const p = hgrip.mapToItem(slot.parent, mouse.x, mouse.y);
                slot.stageUpdateResize(p, mouse.modifiers);
            }
            onReleased: slot.stageEndResize()
            onCanceled: slot.stageCancelResize()
            onDoubleClicked: slot.stageResetScale()
        }
    }

    Rectangle {
        visible: !slot.composing && (slot.resizing || scalePersist.running)
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        anchors.rightMargin: 26
        anchors.bottomMargin: 26
        width: roText.implicitWidth + 16
        height: 20
        radius: Theme.radius
        color: Qt.rgba(0, 0, 0, 0.62)
        Text {
            id: roText
            anchors.centerIn: parent
            text: Math.round(slot.effectiveScale * 100) + "%"
            color: Theme.ink
            font.family: Theme.mono
            font.pixelSize: 11
            font.weight: Font.DemiBold
        }
    }
}

