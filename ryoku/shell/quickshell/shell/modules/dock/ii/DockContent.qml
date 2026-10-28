import QtQuick
import QtQuick.Effects
import QtQuick.Layouts
import Quickshell
import Quickshell.Widgets
import Quickshell.Wayland
import QtQuick.Controls
import Quickshell.Services.Mpris
import Ryoku.Ui.Singletons as RyokuUi

import stage.modules.common
import stage.modules.common.functions
import stage.modules.common.widgets
import shell.modules.phone
import stage.services
import "./widgets"
import "DockReorder.js" as DockReorder
import "DockMagnification.js" as DockMagnification
import "utilities/DockUtilityCatalog.js" as DockUtilityCatalog
import "."

Item {
    id: root

    signal togglePinRequested

    property var currentScreen: null
    property bool isPinned: false
    property bool dockRevealed: false
    property bool dockWindowVisible: true

    readonly property real dockPadding: 0
    readonly property bool isVertical: dock.isVertical
    readonly property real dotMargin: ((Config.options && Config.options.dock) ? Config.options.dock.height : 60) * 0.2 - 2
    readonly property real dotMarginV: dotMargin
    readonly property real sepThickness: Math.max(3, Math.round(Appearance.sizes.dockButtonSize * 0.06))
    readonly property real buttonSlotSize: Appearance.sizes.dockButtonSize + dotMargin * 2
    readonly property real buttonSlotHeight: Appearance.sizes.dockButtonSize + dotMarginV * 2
    readonly property real sportsWidgetSlots: 4
    readonly property real livePreviewWidgetSlots: {
        const slots = (Config.options && Config.options.dock) ? Config.options.dock.livePreviewSlots : 2
        return Math.max(2, Math.min(6, slots !== undefined ? slots : 2))
    }
    readonly property string dockPos: dock.dockEffectivePosition
    readonly property string effectiveDockStyle: {
        const st = (Config.options && Config.options.dock) ? Config.options.dock.dockStyle : ""
        if (st === "islands" || st === "dynamic_island" || st === "hug" || st === "floating" || st === "transparent" || st === "full_width" || st === "full_width_concave")
            return st
        return (Config.options && Config.options.dock && Config.options.dock.islandsStyle) ? "islands" : "floating"
    }
    readonly property bool isDynamicIsland: effectiveDockStyle === "dynamic_island"
    readonly property bool isHug: effectiveDockStyle === "hug"
    readonly property bool isTransparent: effectiveDockStyle === "transparent"
    // Both full-width styles share the flush layout; the concave one adds
    // the two inward corners at the screen's sides (Dock.qml).
    readonly property bool isFullWidthConcave: effectiveDockStyle === "full_width_concave"
    readonly property bool isFullWidth: effectiveDockStyle === "full_width" || isFullWidthConcave
    readonly property bool isAttachedToEdge: isDynamicIsland || isHug || isFullWidth
    readonly property bool islandsStyle: effectiveDockStyle === "islands"
    readonly property real islandSpacing: Math.max(0, (Config.options && Config.options.dock && Config.options.dock.islandSpacing !== undefined) ? Config.options.dock.islandSpacing : 8)
    readonly property real islandExtraGap: islandsStyle
        ? Math.max(0, islandSpacing - ((Config.options && Config.options.dock && Config.options.dock.iconSpacing !== undefined) ? Config.options.dock.iconSpacing : 0))
        : 0
    readonly property bool effectiveShowDividers: ((Config.options && Config.options.dock && Config.options.dock.showDividers !== undefined) ? Config.options.dock.showDividers : true) && !islandsStyle
    readonly property real dockCornerRadius: {
        const rad = (Config.options && Config.options.dock && Config.options.dock.dockRadius !== undefined) ? Config.options.dock.dockRadius : -1
        if (rad >= 0) return rad
        return isAttachedToEdge ? Appearance.rounding.windowRounding : Appearance.rounding.windowRounding + 12
    }

    readonly property real layoutVisualMainExtent: isVertical ? unifiedColumn.height : unifiedRow.width
    readonly property real animatedVisualMainExtent: layoutVisualMainExtent
    readonly property real visualWidth: isVertical ? buttonSlotSize : animatedVisualMainExtent
    readonly property real visualHeight: isVertical ? animatedVisualMainExtent : buttonSlotHeight
    readonly property real baseVisualWidth: isVertical ? buttonSlotSize : baseMetrics.totalMainExtent
    readonly property real baseVisualHeight: isVertical ? baseMetrics.totalMainExtent : buttonSlotHeight

    readonly property bool requestDockShow: (previewPopupLoader.item && previewPopupLoader.item.active) || anyContextMenuOpen

    // PanelWindow.visible stays true while the auto-hide surface is moved off
    // screen. Expensive visual widgets must follow reveal, not only the
    // lifetime of the panel window.
    readonly property bool dockWidgetsActive: root.dockRevealed && root.dockWindowVisible

    readonly property real maxWindowPreviewHeight: 200
    readonly property real maxWindowPreviewWidth: 300
    readonly property real windowControlsHeight: 30

    property int _contextMenuOpenCount: 0
    function registerContextMenuOpen() {
        _contextMenuOpenCount++;
        _contextMenuSafetyTimer.restart();
    }
    function registerContextMenuClose() {
        _contextMenuOpenCount = Math.max(0, _contextMenuOpenCount - 1);
        _contextMenuSafetyTimer.restart();
    }
    readonly property bool anyContextMenuOpen: _contextMenuOpenCount > 0

    // Safety: if the counter stays > 0 for too long (e.g. a menu was destroyed
    // while open without properly decrementing), reset it to unstick the dock.
    Timer {
        id: _contextMenuSafetyTimer
        interval: 8000
        onTriggered: {
            if (_contextMenuOpenCount > 0) {
                _contextMenuOpenCount = 0;
            }
        }
    }
    property bool popupIsResizing: false
    property Item lastHoveredButton: null
    property Item hoveredSlot: null
    property bool buttonHovered: false
    property bool suppressHover: false
    property point hoveredButtonCenter: Qt.point(0, 0)
    property string externalDragIcon: ""
    property bool externalDragOver: false

    readonly property bool enableMagnification: (Config.options && Config.options.dock && Config.options.dock.enableMagnification !== undefined) ? Config.options.dock.enableMagnification : false
    readonly property real magnificationScale: (Config.options && Config.options.dock && Config.options.dock.magnificationScale !== undefined) ? Config.options.dock.magnificationScale : 1.5
    readonly property real magnificationInfluenceRadiusSlots: (Config.options && Config.options.dock && Config.options.dock.magnificationInfluenceRadius !== undefined) ? Config.options.dock.magnificationInfluenceRadius : 2.35
    readonly property real magnificationInfluenceRadiusPx: Math.max(
        Appearance.sizes.dockButtonSize,
        buttonSlotSize * magnificationInfluenceRadiusSlots
    )
    readonly property string magnificationCurve: (Config.options && Config.options.dock && Config.options.dock.magnificationCurve !== undefined) ? Config.options.dock.magnificationCurve : "cosine"
    readonly property string magnificationMotion: (Config.options && Config.options.dock && Config.options.dock.magnificationMotion !== undefined) ? Config.options.dock.magnificationMotion : "balanced"
    readonly property bool magnificationDynamicSpacing: (Config.options && Config.options.dock && Config.options.dock.magnificationDynamicSpacing !== undefined) ? Config.options.dock.magnificationDynamicSpacing : true
    readonly property var magnificationMotionProfile: {
        switch (magnificationMotion) {
        case "fast":
            return Appearance.animation.dockMagnificationScale.fast;
        case "smooth":
            return Appearance.animation.dockMagnificationScale.smooth;
        default:
            return Appearance.animation.dockMagnificationScale.balanced;
        }
    }
    // One lens, not one spring per icon: every icon reads the same smoothed
    // pointer and the same enter/exit strength, so neighbours never drift out
    // of phase. The pointer is measured against the unmagnified layout, and
    // the lens itself is a critically damped spring: a fast sweep glides
    // through the icons instead of snapping from one to the next.
    property real magnificationPointerTarget: 0
    property real magnificationPointerMain: 0
    property real magnificationStrength: 0
    property bool magnificationHovered: false
    // How far the pointer is into the band between the window edge and the
    // icons, 0..1. The lens follows it both ways, so entering or leaving
    // through that edge grows and shrinks it with the cursor, and resting
    // inside the band holds an in-between magnification.
    property real magnificationCrossReach: 0
    // The pointer sample the lens is fed from. A mouse can report hundreds of
    // positions per second and each one used to run the cross-reach mapping
    // and two mapToItem() calls for a value only the next frame could show;
    // a move now records the point and wakes the frame loop instead.
    property var _pointerItem: null
    property point _pointerRaw: Qt.point(0, 0)
    property bool _pointerPending: false
    // Spring velocities of the two lens channels: main-axis units per second
    // for the pointer, strength units per second for the grow-in.
    property real _pointerVelocity: 0
    property real _strengthVelocity: 0
    readonly property bool magnificationInteractionActive: root.dockWidgetsActive
        && enableMagnification
        && !DockPresets.magnificationSuspended
        && magnificationHovered
        && !dragging
        && !islandDragging
        && !anyContextMenuOpen
        && !suppressHover
        && !externalDragOver
    readonly property real magnificationPointerContentMain: magnificationPointerMain
        + (isVertical ? scrollArea.contentY : scrollArea.contentX)
    readonly property bool magnificationOverflowing: isVertical
        ? scrollArea.contentHeight > scrollArea.height + 1
        : scrollArea.contentWidth > scrollArea.width + 1
    // The lens measures distance with island gaps squeezed out, so crossing
    // a gap costs nothing whatever the island spacing: a pointer inside a gap
    // sits on the seam between both islands and the lens holds still.
    readonly property real magnificationLensPointer: _lensCoordinateFor(magnificationPointerContentMain)
    // The same pointer in slot space, where every item spans the room its
    // growth needs: what the lens field is measured in.
    readonly property real magnificationSlotPointer: DockMagnification.toSlotSpace(magnificationLensPointer, baseMetrics.items)

    function _lensCoordinateFor(p) {
        let removed = 0;
        for (const m of baseMetrics.items) {
            const gap = m.bodyStart - m.baseStart;
            if (gap <= 0)
                continue;
            if (p <= m.baseStart)
                break;
            removed += Math.min(gap, p - m.baseStart);
        }
        return p - removed;
    }

    // macOS keeps the point under the cursor fixed: the dock grows by the
    // magnified extra left of the pointer on the left and the rest on the
    // right. The dock centre moves by half the difference.
    readonly property real magnificationCenterShift: {
        if (!enableMagnification || !magnificationDynamicSpacing || magnificationStrength <= 0
                || magnificationOverflowing)
            return 0;
        const items = baseMetrics.items;
        const p = magnificationPointerContentMain;
        let before = 0;
        let after = 0;
        for (let i = 0; i < items.length; i++) {
            const extra = _magnificationExtraForIndex(i);
            if (extra <= 0)
                continue;
            const m = items[i];
            const f = Math.max(0, Math.min(1, (p - m.bodyStart) / Math.max(1, m.bodyExtent)));
            before += extra * f;
            after += extra * (1 - f);
        }
        return (after - before) / 2;
    }

    // A context menu freezes the lens where it is, so the icon under the
    // menu keeps its size instead of shrinking away from it.
    property real _lensFrozenStrength: 0
    onAnyContextMenuOpenChanged: {
        if (!anyContextMenuOpen)
            return;
        _lensFrozenStrength = magnificationStrength;
        magnificationPointerTarget = magnificationPointerMain;
    }
    readonly property real _lensStrengthTarget: anyContextMenuOpen ? _lensFrozenStrength
        : magnificationInteractionActive ? magnificationCrossReach : 0
    property bool _lensSettled: true
    // Exit run: strength it started from and progress 0..1; -1 when idle.
    property real _lensExitFrom: 0
    property real _lensExitProgress: -1
    // Drag targets use baseMetrics, so the visual lens can finish its exit.
    on_LensStrengthTargetChanged: _lensSettled = false
    onDockWidgetsActiveChanged: {
        if (!root.dockWidgetsActive)
            root.resetMagnificationImmediate();
    }

    // Every pointer sample and every layout change the lens reads goes through
    // here; the frame loop itself stops as soon as both channels are at rest.
    function _wakeLens() {
        root._lensSettled = false;
    }

    function resetMagnificationImmediate() {
        magnificationHovered = false;
        magnificationCrossReach = 0;
        magnificationStrength = 0;
        magnificationPointerMain = 0;
        magnificationPointerTarget = 0;
        _pointerVelocity = 0;
        _strengthVelocity = 0;
        _pointerPending = false;
        _lensFrozenStrength = 0;
        _lensExitProgress = -1;
        _lensSettled = true;
    }

    FrameAnimation {
        running: root.dockWidgetsActive && root.enableMagnification && !root._lensSettled
        onTriggered: root._stepMagnification(frameTime)
    }

    // Resolves the pointer sample the events left behind. This is the only
    // place that maps between the hover area and the tray, and it runs once per
    // frame rather than once per mouse event.
    function _resolvePointerTarget() {
        _pointerPending = false;
        const item = root._pointerItem;
        if (!item)
            return;
        const point = root._pointerRaw;
        root._updateMagnificationCrossReach(item, point.x, point.y);
        if (root.magnificationOverflowing) {
            const targetContainer = root.isVertical ? unifiedColumn : unifiedRow;
            const mapped = item.mapToItem(targetContainer, point.x, point.y);
            const visualExtra = root.isVertical
                ? Math.max(0, root.visualHeight - root.baseVisualHeight)
                : Math.max(0, root.visualWidth - root.baseVisualWidth);
            const pointerMain = root.isVertical ? mapped.y : mapped.x;
            root.magnificationPointerTarget = pointerMain - visualExtra / 2;
            return;
        }
        // `item` never moves and the unmagnified dock is centred in it, so
        // this stays independent of the lens it drives.
        root.magnificationPointerTarget = root.isVertical
            ? point.y - item.height / 2 + root.baseVisualHeight / 2
            : point.x - item.width / 2 + root.baseVisualWidth / 2;
    }

    function _stepMagnification(dt) {
        const profile = root.magnificationMotionProfile;
        const step = Math.min(Math.max(dt, 0), 0.05);
        const reduced = Appearance.reducedMotion;

        if (root._pointerPending)
            root._resolvePointerTarget();

        const pointerTarget = root.magnificationPointerTarget;
        const strengthTarget = root._lensStrengthTarget;

        // Entering: start the lens where the pointer is, not where it left.
        let pointer = root.magnificationStrength <= 0.001 ? pointerTarget : root.magnificationPointerMain;
        let pointerVelocity = root.magnificationStrength <= 0.001 ? 0 : root._pointerVelocity;
        if (reduced) {
            pointer = pointerTarget;
            pointerVelocity = 0;
        } else {
            const tracked = DockMagnification.springStep(pointer, pointerVelocity, pointerTarget,
                DockMagnification.trackingOmega(profile.pointerLag), step);
            pointer = tracked.position;
            pointerVelocity = tracked.velocity;
        }

        let strength = root.magnificationStrength;
        let strengthVelocity = root._strengthVelocity;
        if (strengthTarget <= 0 && strength > 0 && !reduced && profile.exitDuration > 0) {
            // Past the window edge there are no pointer samples. Usually the
            // band has already brought this near zero; a flick that skipped it
            // settles on an ease-in-out rather than a decay that reads as a snap.
            if (root._lensExitProgress < 0) {
                root._lensExitFrom = strength;
                root._lensExitProgress = 0;
            }
            const progress = Math.min(1, root._lensExitProgress + step / (profile.exitDuration / 1000));
            root._lensExitProgress = progress;
            strength = root._lensExitFrom * 0.5 * (1 + Math.cos(Math.PI * progress));
            strengthVelocity = 0;
            if (progress >= 1)
                strength = 0;
        } else {
            root._lensExitProgress = -1;
            if (reduced) {
                strength = strengthTarget;
                strengthVelocity = 0;
            } else {
                const grown = DockMagnification.springStep(strength, strengthVelocity, strengthTarget,
                    DockMagnification.settleOmega(profile.strengthDuration), step);
                strength = grown.position;
                strengthVelocity = grown.velocity;
            }
        }

        // Snap the tail. A spring only approaches its target, and a lens
        // creeping for a hundredth of a pixel would hold the frame loop — and
        // with it the whole magnified layout — alive long after the motion is
        // over. Below a tenth of a pixel per frame the snap is invisible.
        let pointerAtRest = false;
        if (Math.abs(pointerTarget - pointer) < 0.05 && Math.abs(pointerVelocity) < 8) {
            pointer = pointerTarget;
            pointerVelocity = 0;
            pointerAtRest = true;
        }
        let strengthAtRest = false;
        if (Math.abs(strengthTarget - strength) < 0.002 && Math.abs(strengthVelocity) < 0.02) {
            strength = strengthTarget;
            strengthVelocity = 0;
            strengthAtRest = true;
        }

        root.magnificationPointerMain = pointer;
        root._pointerVelocity = pointerVelocity;
        root.magnificationStrength = strength;
        root._strengthVelocity = strengthVelocity;
        if (pointerAtRest && strengthAtRest && !root._pointerPending) {
            root._lensExitProgress = -1;
            root._lensSettled = true;
        }
    }

    // Stable metrics are based only on the unscaled layout. Animated wrapper
    // widths never feed back into the proximity field.
    readonly property var baseMetrics: {
        const items = [];
        const spacing = Config.options?.dock?.iconSpacing ?? 0;
        let cursor = 0;
        let removedGap = 0;
        const itemCount = root.flattenedItems.length;
        for (let i = 0; i < itemCount; i++) {
            const leadingGap = root._leadingIslandGapForIndex(i);
            const bodyExtent = root._baseItemMainExtentForIndex(i);
            const mainExtent = leadingGap + bodyExtent;
            const profile = root._magnificationProfileForItem(root.flattenedItems[i]);
            removedGap += leadingGap;
            items.push({
                baseStart: cursor,
                bodyStart: cursor + leadingGap,
                baseCenter: cursor + leadingGap + bodyExtent / 2,
                lensCenter: cursor + leadingGap + bodyExtent / 2 - removedGap,
                baseExtent: mainExtent,
                bodyExtent: bodyExtent,
                islandId: root._islandIdForIndex(i),
                magnifiable: profile !== null,
                magFactor: profile ? profile.factor : 0,
                fromBody: profile ? profile.fromBody === true : false,
                // The extent the slot grows by has to be the extent the
                // component scales, or the neighbours would move either too
                // little (overlap) or too much (a gap).
                magExtent: profile ? (profile.fromBody ? bodyExtent : Appearance.sizes.dockButtonSize) : 0
            });
            cursor += mainExtent;
            if (i < itemCount - 1)
                cursor += spacing;
        }
        // Slot space (DockMagnification.slotCell): each item's cell on the
        // lens axis, in pixels and in growth units, tiling both.
        const iconPitch = Math.max(1, root.buttonSlotSize + spacing);
        let slotCursor = 0;
        for (const m of items) {
            m.lensCell = Math.max(1, m.bodyExtent + spacing);
            m.lensStart = m.lensCenter - m.lensCell / 2;
            m.slotCell = DockMagnification.slotCell(m.lensCell, m.magExtent, m.magFactor, m.fromBody,
                Appearance.sizes.dockButtonSize, iconPitch);
            m.slotStart = slotCursor;
            m.slotCenter = slotCursor + m.slotCell / 2;
            slotCursor += m.slotCell;
        }
        return {
            items: items,
            totalMainExtent: Math.max(0, cursor)
        };
    }

    // The main-axis reserve keeps the PanelWindow stable while the visible
    // tray follows the animated layout width/height.
    readonly property real maximumMagnificationExtra: {
        if (!enableMagnification || baseMetrics.items.length === 0)
            return 0;

        let maximum = 0;
        const candidates = baseMetrics.items.filter(metric => metric.magnifiable);
        for (const candidate of candidates) {
            let total = 0;
            for (const metric of baseMetrics.items) {
                if (!metric.magnifiable)
                    continue;
                total += root._magnificationLayoutExtraFor(metric, root.magnificationFactorForDistance(Math.abs(candidate.slotCenter - metric.slotCenter)), true);
            }
            maximum = Math.max(maximum, total);
        }
        return Math.ceil(maximum + Appearance.sizes.elevationMargin);
    }

    readonly property real magnificationRenderScale: enableMagnification ? Math.max(1, magnificationScale) : 1
    readonly property real maximumMagnificationCrossExtra: enableMagnification
        ? Math.ceil(Appearance.sizes.dockButtonSize * Math.max(0, magnificationScale - 1.0))
        : 0
    // Magnified content grows away from the screen edge the dock sits on, the
    // way the icons do. Shared so a dock widget cannot pick a different one.
    readonly property int magnificationTransformOrigin: {
        if (dockPos === "top")
            return Item.Top;
        if (dockPos === "left")
            return Item.Left;
        if (dockPos === "right")
            return Item.Right;
        return Item.Bottom;
    }
    readonly property real effectiveIconSpacing: Config.options?.dock?.iconSpacing ?? 0
    readonly property bool enableAppGroups: Config.options?.dock?.enableAppGroups ?? true
    readonly property int maxGroupApps: 6
    readonly property int groupAnimationDuration: Appearance.animation.elementMoveFast.duration

    // Group changes replace Repeater delegates. Keep the old surface alive for
    // one animation cycle before committing the persistent model so apps can
    // visibly leave a group instead of disappearing synchronously.
    property bool groupMutationPending: false
    property int groupMutationRevision: 0
    property string groupMutationKind: ""
    property string groupMutationGroupId: ""
    property var groupMutationAppIds: []
    property var pendingGroupMutationGroups: []
    property var pendingGroupOrderRequest: null

    // This second state describes the delegates created by the committed
    // mutation. It is intentionally transient and never persisted.
    property int groupTransitionRevision: 0
    property string groupTransitionKind: ""
    property string groupTransitionGroupId: ""
    property var groupTransitionAppIds: []

    Timer {
        id: groupMutationTimer
        interval: root.groupAnimationDuration
        repeat: false
        onTriggered: root._commitGroupMutation()
    }

    Timer {
        id: groupTransitionClearTimer
        interval: root.groupAnimationDuration * 2
        repeat: false
        onTriggered: {
            root.groupTransitionKind = ""
            root.groupTransitionGroupId = ""
            root.groupTransitionAppIds = []
            root.groupTransitionRevision++
        }
    }

    // Islands are derived from the final flattened order. Keeping this model
    // independent from the visual surfaces preserves global drag indices.
    readonly property var islandSegments: root._buildIslandSegments(root.flattenedItems)
    readonly property var islandByItemIndex: {
        const lookup = [];
        for (const segment of root.islandSegments) {
            for (let i = segment.startIndex; i <= segment.endIndex; i++)
                lookup[i] = segment;
        }
        return lookup;
    }

    // Repeater.count can settle before itemAt() exposes the delegates. Island
    // surfaces read this revision so their endpoint bindings retry as each
    // wrapper becomes available instead of waiting for an app-list change.
    property int _itemDelegateRevision: 0

    function _islandKind(item) {
        if (!item)
            return "single";
        switch (item.type) {
        case "app": return "apps";
        case "action": return "actions";
        case "appGroup": return "apps";
        case "runningAppsGroup": return "apps";
        case "file": return "file";
        case "media": return "media";
        case "weather": return "weather";
        case "sports": return "sports";
        case "livePreview": return "livePreview";
        case "phone": return "phone";
        case "tasks": return "tasks";
        case "widgetStack": return "widgetStack";
        case "utility": return "utility";
        default: return "single";
        }
    }

    function _isLauncherItem(item) {
        if (!item)
            return false;
        return item.type === "app"
            || item.type === "appGroup"
            || item.type === "runningAppsGroup";
    }

    function _canMergeIslandItems(previous, current) {
        if (!previous || !current)
            return false;
        return (root._isLauncherItem(previous) && root._isLauncherItem(current))
            || (previous.type === "action" && current.type === "action");
    }

    function _islandIdForItem(item, index) {
        if (!item)
            return "single:" + String(index);
        switch (item.type) {
        case "app":
            return "apps:" + String(item.orderKey ?? item.appId ?? index);
        case "action":
            return "actions:" + String(item.actionId ?? index);
        case "appGroup":
            return "group:" + String(item.groupId ?? index);
        case "runningAppsGroup":
            return "apps:" + String(item.orderKey ?? index);
        case "file":
            return "file:" + String(item.path ?? index);
        case "media":
            return "widget:media";
        case "weather":
            return "widget:weather";
        case "sports":
            return "widget:sports";
        case "livePreview":
            return "widget:livePreview";
        case "phone":
            return "widget:phone";
        case "tasks":
            return "widget:tasks";
        case "utility":
            return "widget:" + String(item.orderKey ?? index);
        case "widgetStack":
            return "widget:stack";
        default:
            return "single:" + String(item.orderKey ?? index);
        }
    }

    function _buildIslandSegments(items) {
        const segments = [];
        let current = null;
        for (let i = 0; i < items.length; i++) {
            const item = items[i];
            if (current && root._canMergeIslandItems(items[i - 1], item)) {
                current.endIndex = i;
                continue;
            }

            current = {
                id: root._islandIdForItem(item, i),
                kind: root._islandKind(item),
                startIndex: i,
                endIndex: i
            };
            segments.push(current);
        }
        return segments;
    }

    function _islandMetaForIndex(index) {
        return root.islandByItemIndex[index] ?? null;
    }

    function _islandIdForIndex(index) {
        return root._islandMetaForIndex(index)?.id ?? "";
    }

    function _islandStartsAt(index) {
        const meta = root._islandMetaForIndex(index);
        return !!meta && meta.startIndex === index;
    }

    function _leadingIslandGapForIndex(index) {
        return index > 0 && root._islandStartsAt(index) ? root.islandExtraGap : 0;
    }

    function _islandSegmentIndexForId(islandId) {
        for (let i = 0; i < root.islandSegments.length; i++) {
            if (String(root.islandSegments[i].id) === String(islandId))
                return i;
        }
        return -1;
    }

    function _islandBoundsForSegment(segment) {
        if (!segment)
            return null;

        const first = root.getItemWrapper(segment.startIndex);
        const last = root.getItemWrapper(segment.endIndex);
        if (!first || !last || !first.parent || !last.parent)
            return null;

        const firstMain = first.bodyMainStart;
        const lastMain = last.bodyMainEnd;
        const firstPoint = first.parent.mapToItem(
            root,
            root.isVertical ? first.x : firstMain,
            root.isVertical ? firstMain : first.y
        );
        const lastPoint = last.parent.mapToItem(
            root,
            root.isVertical ? last.x : lastMain,
            root.isVertical ? lastMain : last.y
        );
        return {
            start: root.isVertical ? firstPoint.y : firstPoint.x,
            end: root.isVertical ? lastPoint.y : lastPoint.x
        };
    }

    function _islandMainCoordinateFromScene(scenePosition) {
        if (!scenePosition)
            return 0;
        const point = root.mapFromItem(null, scenePosition.x, scenePosition.y);
        return root.isVertical ? point.y : point.x;
    }

    function _orderKeysForIslandItem(item, currentOrder) {
        if (!item)
            return [];

        if (item.type === "appGroup") {
            return (item.appIds ?? []).map(appId => root._orderKeyForAppId(currentOrder, appId));
        }

        if (item.type === "runningAppsGroup") {
            return (item.apps ?? []).map(app => root._orderKeyForAppId(currentOrder, app?.appId ?? app));
        }

        const key = String(item.orderKey ?? "");
        return key ? [key] : [];
    }

    function _orderKeysForIsland(segment, currentOrder) {
        const keys = [];
        const seen = {};
        for (let i = segment.startIndex; i <= segment.endIndex; i++) {
            const itemKeys = root._orderKeysForIslandItem(root.flattenedItems[i], currentOrder);
            for (const key of itemKeys) {
                if (!key || seen[key])
                    continue;
                seen[key] = true;
                keys.push(key);
            }
        }
        return keys;
    }

    function _resetIslandDrag() {
        root.islandDragging = false;
        root.islandDragId = "";
        root.islandDragSourceIndex = -1;
        root.islandDragTargetIndex = -1;
        root.islandDragCursorMain = 0;
    }

    // Settings → Dock → "Lock arrangement": nothing starts a drag, so nothing
    // moves, groups or leaves the dock by accident.
    readonly property bool reorderLocked: Config.options?.dock?.lockReorder ?? false

    // ── Edit Mode, on the dock ─────────────────────────────────────────────
    // Apps wear their own pin/unpin badges (DockAppButton). Everything else
    // gets an edit layer from the delegate: a remove badge, and a click that
    // opens its menu (size, stack, options). The (+) tile at the end adds.
    readonly property bool editLayerActive: GlobalStates.editMode && (GlobalStates.editProgress > 0.85 || Appearance.reducedMotion)
    readonly property string editAddKey: "__editAdd"
    readonly property alias dockEdit: dockEditController
    DockEditController {
        id: dockEditController
        dockContent: root
    }
    DockEditMenu {
        id: dockEditMenu
        dockContent: root
        controller: dockEditController
    }
    readonly property bool editMenuOpen: dockEditMenu.active && !dockEditMenu.isClosing
    readonly property var editMenuTarget: dockEditMenu.target
    function openEditMenu(anchor, item) {
        dockEditMenu.openFor(anchor, item, item?.type === "editAdd" ? "add" : "item");
    }
    onEditLayerActiveChanged: {
        if (!root.editLayerActive)
            dockEditMenu.close();
    }

    function startIslandDrag(islandId, scenePosition) {
        if (root.reorderLocked || !root.islandsStyle || root.dragging || root.anyContextMenuOpen)
            return false;

        const sourceIndex = root._islandSegmentIndexForId(islandId);
        if (sourceIndex < 0)
            return false;

        root.islandDragging = true;
        root.islandDragId = String(islandId);
        root.islandDragSourceIndex = sourceIndex;
        root.islandDragTargetIndex = sourceIndex;
        root.islandDragCursorMain = root._islandMainCoordinateFromScene(scenePosition);
        root._suppressTranslateAnim = true;
        root.buttonHovered = false;
        root.lastHoveredButton = null;
        return true;
    }

    function moveIslandDrag(scenePosition) {
        if (!root.islandDragging)
            return;

        root.islandDragCursorMain = root._islandMainCoordinateFromScene(scenePosition);
        let targetIndex = root.islandDragSourceIndex;
        for (let i = 0; i < root.islandSegments.length; i++) {
            if (i === root.islandDragSourceIndex)
                continue;
            const bounds = root._islandBoundsForSegment(root.islandSegments[i]);
            if (!bounds)
                continue;
            if (root.islandDragCursorMain < (bounds.start + bounds.end) / 2) {
                targetIndex = i;
                break;
            }
            targetIndex = i;
        }
        root.islandDragTargetIndex = targetIndex;
    }

    function _moveIslandBlock(sourceIndex, targetIndex) {
        if (sourceIndex < 0 || targetIndex < 0
                || sourceIndex >= root.islandSegments.length
                || targetIndex >= root.islandSegments.length
                || sourceIndex === targetIndex)
            return false;

        const currentOrder = Array.from(Config.options?.dock?.order ?? []);
        const sourceKeys = root._orderKeysForIsland(root.islandSegments[sourceIndex], currentOrder);
        const targetKeys = root._orderKeysForIsland(root.islandSegments[targetIndex], currentOrder);
        if (sourceKeys.length === 0 || targetKeys.length === 0)
            return false;

        const sourceSet = {};
        for (const key of sourceKeys)
            sourceSet[key] = true;
        const nextOrder = currentOrder.filter(key => !sourceSet[key]);

        let insertionIndex = -1;
        if (sourceIndex < targetIndex) {
            for (const key of targetKeys) {
                const index = nextOrder.indexOf(key);
                if (index >= 0)
                    insertionIndex = Math.max(insertionIndex, index + 1);
            }
        } else {
            for (const key of targetKeys) {
                const index = nextOrder.indexOf(key);
                if (index >= 0) {
                    insertionIndex = index;
                    break;
                }
            }
        }

        if (insertionIndex < 0)
            insertionIndex = nextOrder.length;
        nextOrder.splice(insertionIndex, 0, ...sourceKeys);

        if (nextOrder.length === currentOrder.length
                && nextOrder.every((entry, index) => entry === currentOrder[index]))
            return false;

        root._writeDockOrder(nextOrder);
        return true;
    }

    function endIslandDrag() {
        if (!root.islandDragging)
            return;

        const sourceIndex = root.islandDragSourceIndex;
        const targetIndex = root.islandDragTargetIndex;
        if (sourceIndex >= 0 && targetIndex >= 0 && sourceIndex !== targetIndex) {
            root._reordering = true;
            root._moveIslandBlock(sourceIndex, targetIndex);
        }

        root._resetIslandDrag();
        root.buttonHovered = false;
        root.lastHoveredButton = null;
        root.suppressHover = true;
        suppressHoverTimer.restart();
        Qt.callLater(function () {
            root._reordering = false;
            root._suppressTranslateAnim = false;
        });
    }

    // Per-item share of the lens. A single icon takes all of it; a widget
    // drawn as one wide card takes a muted share, because the same linear
    // factor applied to a 3-4 slot body would sweep its neighbours several
    // times as far as an icon does. `fromBody` says which extent the slot
    // grows by: the icon square, or the item's own body (always the body the
    // component actually draws, so the slot can never grow less than the card).
    function _magnificationProfileForType(type) {
        switch (type) {
        case "app":
        case "appGroup":
        case "file":
        case "action":
        case "phone":
            return { factor: 1.0, fromBody: false };
        case "media":
        case "weather":
            return { factor: 0.5, fromBody: true };
        case "sports":
            return { factor: 0.4, fromBody: true };
        case "livePreview":
            return { factor: 0.45, fromBody: true };
        case "tasks":
        case "widgetStack":
            return { factor: 0.5, fromBody: true };
        default:
            return null;
        }
    }

    // A square utility tile is an icon-sized body and takes the icon's share;
    // a wide one is a card like the other widgets.
    function _magnificationProfileForItem(item) {
        if (item?.type === "utility")
            return root._utilitySlots(item) > 1 ? { factor: 0.5, fromBody: true } : { factor: 1.0, fromBody: false };
        return root._magnificationProfileForType(item?.type);
    }

    function _isMagnifiableItem(item) {
        return root._magnificationProfileForItem(item) !== null;
    }

    function _rawItemMainExtent(item) {
        if (isVertical || !item)
            return buttonSlotSize;
        switch (item.type) {
        case "media":
        case "weather":
            return buttonSlotSize * 3;
        case "sports":
            return buttonSlotSize * sportsWidgetSlots;
        case "livePreview":
            return buttonSlotSize * livePreviewWidgetSlots;
        case "tasks":
            return buttonSlotSize * 3;
        case "widgetStack":
            return root.widgetStackExtent;
        case "utility":
            return buttonSlotSize * root._utilitySlots(item);
        default:
            return buttonSlotSize;
        }
    }

    function _separatorBeforeSpaceFor(index) {
        if (!root.effectiveShowDividers || index <= 0)
            return 0;
        const item = root.flattenedItems[index];
        const previous = root.flattenedItems[index - 1];
        if (Config.options?.dock?.smartGrouping)
            return root.getItemCategory(item) !== root.getItemCategory(previous) ? Math.max(root.dotMargin, root.sepThickness * 2) : 0;
        return root.isSpecialItem(item) ? Math.max(root.dotMargin, root.sepThickness * 2) : 0;
    }

    function _separatorAfterSpaceFor(index) {
        if (!root.effectiveShowDividers || index >= root.flattenedItems.length - 1)
            return 0;
        if (Config.options?.dock?.smartGrouping)
            return 0;
        const item = root.flattenedItems[index];
        const next = root.flattenedItems[index + 1];
        return root.isSpecialItem(item) && !root.isSpecialItem(next)
            ? Math.max(root.dotMargin, root.sepThickness * 2)
            : 0;
    }

    function _baseMainExtentForIndex(index) {
        return root._leadingIslandGapForIndex(index) + root._baseItemMainExtentForIndex(index);
    }

    function _baseItemMainExtentForIndex(index) {
        const item = root.flattenedItems[index];
        return root._rawItemMainExtent(item)
            + root._separatorBeforeSpaceFor(index)
            + root._separatorAfterSpaceFor(index);
    }

    function magnificationFactorForDistance(distancePx) {
        return DockMagnification.factorForDistance(distancePx, magnificationInfluenceRadiusPx, magnificationCurve);
    }

    // The main-axis room one item needs at `factor` of the lens. The slot grows
    // by exactly the amount the item's content grows, so magnifying an item
    // slides its neighbours instead of drawing over them.
    function magnificationLayoutExtraForFactor(factor, extent, contentFactor) {
        return DockMagnification.layoutExtra(factor, magnificationScale, extent, contentFactor, magnificationDynamicSpacing);
    }

    // The same room, without the spacing switch: what the window has to reserve
    // whether or not the layout actually takes it.
    function magnificationSafetyExtraForFactor(factor, extent, contentFactor) {
        return DockMagnification.layoutExtra(factor, magnificationScale, extent, contentFactor, true);
    }

    function _magnificationContentScaleForWeight(weight, contentFactor) {
        return DockMagnification.contentScale(weight, magnificationScale, contentFactor);
    }


    // 0..1 lens weight of one item: distance falloff times enter/exit strength,
    // measured in slot space. An icon samples the field at its centre; a card
    // spans several slot units and averages the field over them, so it grows
    // and shrinks with the lens as one smooth piece.
    function _magnificationWeightForIndex(index) {
        const metric = baseMetrics.items[index];
        if (!enableMagnification || !metric || metric.magFactor <= 0 || magnificationStrength <= 0)
            return 0;
        if (metric.fromBody)
            return DockMagnification.cellWeight(metric.slotCenter, Math.max(1, metric.slotCell / 2),
                magnificationSlotPointer, magnificationInfluenceRadiusPx, magnificationCurve, magnificationStrength, 1);
        const distance = Math.abs(magnificationSlotPointer - metric.slotCenter);
        return DockMagnification.weightForDistance(distance, magnificationInfluenceRadiusPx, magnificationCurve, magnificationStrength);
    }

    // The main-axis room an item takes at `weight`: exactly what its content
    // grows (slot space already evens out the growth along the dock).
    function _magnificationLayoutExtraFor(metric, weight, dynamicSpacing) {
        if (!metric)
            return 0;
        return DockMagnification.layoutExtra(weight, magnificationScale, metric.magExtent, metric.magFactor, dynamicSpacing);
    }

    // 0 when the item never magnifies, 1 for a single icon, less for a widget.
    function _magnificationContentFactorForIndex(index) {
        const metric = baseMetrics.items[index];
        return metric ? metric.magFactor : 0;
    }

    function _magnificationExtraForIndex(index) {
        const metric = baseMetrics.items[index];
        if (!metric)
            return 0;
        return root._magnificationLayoutExtraFor(metric, _magnificationWeightForIndex(index), magnificationDynamicSpacing);
    }

    // Compatibility helper for tooltip/preview code. Main button scale is
    // owned by the delegate wrapper, so this no longer scans repeater items.
    function _getSlotMagScale(targetSlot) {
        let item = targetSlot;
        for (let depth = 0; item && depth < 6; depth++) {
            if (typeof item.dockMagnificationScale !== "undefined")
                return item.dockMagnificationScale;
            if (typeof item._magnificationScale !== "undefined")
                return item._magnificationScale;
            item = item.parent;
        }
        return 1.0;
    }

    function onButtonEntered(slotItem) {
        if (suppressHover || dragging)
            return;
        hoverGraceTimer.stop();
        hoveredSlot = slotItem;
        lastHoveredButton = slotItem;
        buttonHovered = true;
    }

    function onButtonExited(slotItem) {
        if (lastHoveredButton === slotItem || hoveredSlot === slotItem) {
            hoverGraceTimer.restart();
        }
    }

    Timer {
        id: hoverGraceTimer
        interval: 150
        onTriggered: {
            root.buttonHovered = false;
            root.hoveredSlot = null;
        }
    }

    Timer {
        id: magnificationExitTimer
        interval: Appearance.animation.dockMagnificationScale.hoverExitGrace
        onTriggered: {
            root.magnificationHovered = false;
            root.magnificationCrossReach = 0;
        }
    }

    function setMagnificationHovered(value) {
        if (value) {
            magnificationExitTimer.stop();
            root.magnificationHovered = true;
        } else {
            magnificationExitTimer.restart();
        }
    }

    // Called from the hover handler on every mouse move. It only records the
    // sample: the mapping to the lens and the cross-reach pass run once per
    // frame in _resolvePointerTarget(), because a mouse reporting hundreds of
    // positions a second would otherwise run both for every one of them.
    function updateMagnificationPointerFrom(item, x, y) {
        if (!item || !root.enableMagnification || DockPresets.magnificationSuspended || root.anyContextMenuOpen)
            return;
        root._pointerItem = item;
        root._pointerRaw = Qt.point(x, y);
        root._pointerPending = true;
        root._wakeLens();
    }

    function _updateMagnificationCrossReach(item, x, y) {
        const local = item.mapToItem(root, x, y);
        const origin = root.mapToItem(item, 0, 0);
        let distance = 0;
        let band = 0;
        switch (root.dockPos) {
        case "top":
            distance = local.y - root.height;
            band = item.height - (origin.y + root.height);
            break;
        case "left":
            distance = local.x - root.width;
            band = item.width - (origin.x + root.width);
            break;
        case "right":
            distance = -local.x;
            band = origin.x;
            break;
        default:
            distance = -local.y;
            band = origin.y;
            break;
        }
        const t = band > 1 ? Math.max(0, Math.min(1, 1 - distance / band)) : 1;
        root.magnificationCrossReach = t * t * (3 - 2 * t);
    }

    readonly property var activePlayer: MprisController.activePlayer
    readonly property string rawTitle: StringUtils.cleanMusicTitle(activePlayer?.trackTitle) || ""
    readonly property bool hasRealData: activePlayer !== null && rawTitle !== ""
    property bool showMusicPlayer: hasRealData

    onHasRealDataChanged: {
        if (hasRealData) {
            switchHoldTimer.stop();
            showMusicPlayer = true;
        } else
            switchHoldTimer.restart();
    }

    Timer {
        id: suppressHoverTimer
        interval: 250
        onTriggered: root.suppressHover = false
    }
    Timer {
        id: switchHoldTimer
        interval: 2000
        onTriggered: if (!root.hasRealData)
            root.showMusicPlayer = false
    }

    function updateHoveredButtonCenter() {
        const btn = root.lastHoveredButton;
        if (!btn)
            return;
        // Mapping the live visual center includes wrapper reflow and the
        // delegate's current animated scale without duplicating transform math.
        root.hoveredButtonCenter = btn.mapToItem(null, btn.width / 2, btn.height / 2);
    }

    // The center only feeds the open preview popup and the group popup: both
    // exist only while the pointer dwells on one item, which is also the only
    // time the value matters. Trailing a lens in motion was a mapToItem() per
    // geometry change per frame for a consumer that was not on screen.
    readonly property bool hoveredCenterTrackingActive:
        root.dockWindowVisible && !root.dragging && !root.islandDragging
        && (root.requestDockShow || root.lastHoveredButton?.containsMouse === true)

    onLastHoveredButtonChanged: root.updateHoveredButtonCenter()
    onVisualWidthChanged: root.updateHoveredButtonCenter()
    onVisualHeightChanged: root.updateHoveredButtonCenter()

    Connections {
        target: root.lastHoveredButton
        enabled: root.hoveredCenterTrackingActive
        function onXChanged() { root.updateHoveredButtonCenter(); }
        function onYChanged() { root.updateHoveredButtonCenter(); }
        function onWidthChanged() { root.updateHoveredButtonCenter(); }
        function onHeightChanged() { root.updateHoveredButtonCenter(); }
        function onScaleChanged() { root.updateHoveredButtonCenter(); }
    }

    readonly property bool showPin: Config.options?.dock?.showPinButton ?? true
    readonly property bool showOverview: Config.options?.dock?.showOverviewButton ?? true
    readonly property bool showTrash: Config.options?.dock?.showTrashButton ?? true
    // ── Widget stack ───────────────────────────────────────────────────────
    // Widgets placed in the stack share one slot and are not drawn on their
    // own. A member that has nothing to show right now (no player, no game)
    // drops out of the stack the way it would drop out of the dock.
    readonly property bool widgetStackEnabled: Config.options?.dock?.enableWidgetStack ?? false
    readonly property var widgetStackMembers: {
        if (!root.widgetStackEnabled)
            return [];
        const wanted = Config.options?.dock?.widgetStackItems ?? [];
        const members = [];
        for (const type of wanted) {
            if (members.indexOf(type) < 0 && root._widgetAvailable(type))
                members.push(type);
        }
        return members;
    }
    readonly property real widgetStackExtent: {
        if (root.isVertical)
            return root.buttonSlotSize;
        let widest = root.buttonSlotSize;
        for (const type of root.widgetStackMembers)
            widest = Math.max(widest, root._rawItemMainExtent({ type: type }));
        return widest;
    }
    // The page on show, by type: kept here so rebuilding the model (an app
    // opening or closing) does not turn the stack back to its first page.
    property string widgetStackCurrentType: ""

    function _widgetAvailable(type) {
        switch (type) {
        case "media": return root.showMusicPlayer;
        case "weather": return true;
        case "sports": return !root.isVertical && SportsService.allGames.length > 0;
        case "livePreview": return true;
        case "tasks": return true;
        default: return false;
        }
    }

    function _inWidgetStack(type) {
        return root.widgetStackMembers.indexOf(type) >= 0;
    }

    readonly property bool showMedia: (Config.options?.dock?.enableMediaWidget ?? false) && root.showMusicPlayer
        && !root._inWidgetStack("media")
    readonly property bool showWeather: (Config.options?.dock?.enableWeatherWidget ?? false) && !root._inWidgetStack("weather")
    readonly property bool showSports: (Config.options?.dock?.enableSportsWidget ?? true) && !root.isVertical
        && SportsService.allGames.length > 0 && !root._inWidgetStack("sports")
    readonly property bool showLivePreview: (Config.options?.dock?.enableLivePreviewWidget ?? false) && !root._inWidgetStack("livePreview")
    readonly property bool showTasks: (Config.options?.dock?.enableTasksWidget ?? false) && !root._inWidgetStack("tasks")
    // Utility widgets, as saved ({ kind, wide }), and the ones the dock shows.
    readonly property var utilityEntries: DockUtilityCatalog.normalize(Config.options?.dock?.utilityWidgets ?? [])
    readonly property var utilityEntryByKey: {
        const map = {};
        for (const entry of root.utilityEntries)
            map[DockUtilityCatalog.orderKey(entry.kind)] = entry;
        // A renamed kind keeps the place its old key holds in dock.order.
        for (const oldKind in DockUtilityCatalog.aliases) {
            const entry = map[DockUtilityCatalog.orderKey(DockUtilityCatalog.aliases[oldKind])];
            if (entry)
                map[DockUtilityCatalog.orderKey(oldKind)] = entry;
        }
        return map;
    }
    function _utilityItem(entry) {
        return {
            type: "utility",
            kind: entry.kind,
            wide: entry.wide,
            orderKey: DockUtilityCatalog.orderKey(entry.kind)
        };
    }
    function _utilitySlots(item) {
        return DockUtilityCatalog.slotsFor(item, root.isVertical);
    }
    // Utility hosts by kind, for the drag-off export above.
    property var _utilityHosts: ({})
    function registerUtilityHost(kind, host) {
        root._utilityHosts[kind] = host;
    }
    function unregisterUtilityHost(kind, host) {
        if (root._utilityHosts[kind] === host)
            delete root._utilityHosts[kind];
    }
    function _utilityExport(kind) {
        const host = root._utilityHosts[kind];
        return host ? host.exportFiles() : null;
    }

    // Renamed utility kinds: rewrite their old dock.order keys once, so a
    // stale key can never hold the place of the current one.
    function _migrateUtilityKeys() {
        if (!Config.ready)
            return;
        const order = Array.from(Config.options.dock.order ?? []);
        let changed = false;
        const next = [];
        for (const key of order) {
            const kind = DockUtilityCatalog.kindFromOrderKey(key);
            const current = kind.length > 0 ? DockUtilityCatalog.orderKey(kind) : key;
            if (current !== key)
                changed = true;
            if (kind.length > 0 && next.indexOf(current) >= 0) {
                changed = true;
                continue;
            }
            next.push(current);
        }
        if (changed)
            root._writeDockOrder(next);
        const saved = Array.from(Config.options.dock.utilityWidgets ?? []);
        if (saved.some(entry => DockUtilityCatalog.canonical(entry?.kind ?? entry) !== (entry?.kind ?? entry)))
            Config.options.dock.utilityWidgets = DockUtilityCatalog.normalize(saved);
    }

    function setUtilityWide(kind, wide) {
        Config.options.dock.utilityWidgets = DockUtilityCatalog.withKind(Config.options.dock.utilityWidgets ?? [], kind, wide);
    }
    function removeUtility(kind) {
        Config.options.dock.utilityWidgets = DockUtilityCatalog.withoutKind(Config.options.dock.utilityWidgets ?? [], kind);
        const key = DockUtilityCatalog.orderKey(kind);
        const order = Array.from(Config.options.dock.order ?? []);
        if (order.indexOf(key) >= 0)
            root._writeDockOrder(order.filter(entry => entry !== key));
    }

    readonly property bool showPhone: (Config.options?.dock?.showPhoneButton ?? true)
        && KdeConnectService.activeReachable

    // ── Drag-to-reorder state ────────────────────────────────────────────
    // The gesture resolves against a snapshot of the layout taken when the
    // drag starts (`_dragSlots`), never against live delegate geometry: the
    // preview translations, magnification and apps launching in the background
    // all move delegates around, and the old cumulative walk from the press
    // point drifted away from what the user could see.
    property bool dragging: false
    property bool _reordering: false
    property bool _suppressTranslateAnim: false
    property bool reorderMotionActive: false
    Timer {
        id: reorderMotionTimer
        interval: Appearance.animation.elementMoveFast.duration
        onTriggered: root.reorderMotionActive = false
    }
    property string dragSourceKey: ""
    property int dragSourceIndex: -1
    property real dragCursorX: 0
    property real dragStartCursorX: 0
    property int _dragTargetIndex: -1
    property int _groupDropTargetIndex: -1
    property bool _groupDropWillCreate: false
    property bool _groupDropBlocked: false
    property var _dragSlots: []
    property var _dragGroupable: []
    property var _dragState: DockReorder.createDragState()
    property real _groupDropProgress: 0
    property bool exportingShortcut: false
    Item {
        id: shortcutDrag
        Drag.dragType: Drag.None
        Drag.supportedActions: Qt.CopyAction
        Drag.proposedAction: Qt.CopyAction
        Drag.imageSourceSize: Qt.size(48, 48)
    }

    // The pointer must penetrate a neighbour by this fraction of that
    // neighbour's own extent before it takes over as the drop target. Without
    // it a pointer resting on a boundary re-decides on every mouse sample.
    readonly property real reorderHysteresis: 0.25
    // Grouping is a deliberate hold over the middle of an icon. The old test
    // armed on the whole slot with no dwell, so sliding an app across the dock
    // kept latching onto grouping and reordering could not be finished.
    readonly property real groupDropCenterZone: 0.5
    readonly property int groupDropDwellMs: 220

    // The gesture owns settling; the stable delegate retains its visual position
    // while the committed order catches up with the preview.
    property string _dropSettleKey: ""
    property real _dropSettleOffset: 0
    NumberAnimation {
        id: dropSettleAnimation
        target: root
        property: "_dropSettleOffset"
        to: 0
        duration: Appearance.animation.elementMoveFast.duration
        easing.type: Appearance.animation.elementMoveFast.type
        easing.bezierCurve: Appearance.animation.elementMoveFast.bezierCurve
        onFinished: root._dropSettleKey = ""
    }

    function _settleDropAt(orderKey, offset) {
        dropSettleAnimation.stop();
        const key = String(orderKey ?? "");
        if (key === "" || !isFinite(offset) || Math.abs(offset) < 0.5) {
            root._dropSettleKey = "";
            root._dropSettleOffset = 0;
            return;
        }
        root._dropSettleKey = key;
        root._dropSettleOffset = offset;
        dropSettleAnimation.restart();
    }

    // Islands are reordered as contiguous blocks of the existing dock.order.
    // This keeps item-level ordering and grouping data in one persistent model.
    property bool islandDragging: false
    property string islandDragId: ""
    property int islandDragSourceIndex: -1
    property int islandDragTargetIndex: -1
    property real islandDragCursorMain: 0

    // ── Helper: get the active Repeater instance ─────────────────────────
    function _getActiveRepeater() {
        return root.isVertical ? columnItemRepeater : itemRepeater;
    }
    function getItemWrapper(index) {
        void root._itemDelegateRevision;
        var repeater = _getActiveRepeater();
        return repeater ? repeater.itemAt(index) : null;
    }

    // Snapshot every item's body box in root coordinates, once, when the drag
    // begins.
    //
    // The geometry comes from `baseMetrics` — the model's own layout — and not
    // from live delegates. Delegates retain magnification during its animated
    // exit and pick up preview translations, so reading them back would
    // snapshot a transient layout rather than the resting drop targets.
    // Where the Row/Column starts in root coordinates when the slots were
    // taken: slots are in root space, delegates position in the container's.
    property real _dragSlotOrigin: 0
    function _buildDragSlots() {
        const metrics = root.baseMetrics.items;
        const container = root.isVertical ? unifiedColumn : unifiedRow;
        const origin = container ? container.mapToItem(root, 0, 0) : Qt.point(0, 0);
        const originMain = root.isVertical ? origin.y : origin.x;
        root._dragSlotOrigin = originMain;

        var slots = [];
        for (var i = 0; i < metrics.length; i++) {
            const metric = metrics[i];
            if (!metric) {
                slots.push({ start: originMain, end: originMain });
                continue;
            }
            const start = originMain + metric.bodyStart;
            slots.push({ start: start, end: start + metric.bodyExtent });
        }
        return slots;
    }

    // Which items the dragged one could be dropped onto to form or join a
    // group. Types cannot change mid-gesture, so this is computed once too.
    function _canGroupWithIndex(source, index) {
        if (!root.enableAppGroups || !source || source.type !== "app")
            return false;
        if (index === root.dragSourceIndex)
            return false;
        var target = root.flattenedItems[index];
        if (!target || target.__exiting === true)
            return false;
        if (target.type === "app")
            return target.appId !== source.appId;
        return target.type === "appGroup";
    }

    function _buildGroupableFlags() {
        var source = root.flattenedItems[root.dragSourceIndex];
        var flags = [];
        for (var i = 0; i < root.flattenedItems.length; i++)
            flags.push(root._canGroupWithIndex(source, i));
        return flags;
    }

    function _orderEntryAppId(entry) {
        const key = String(entry ?? "");
        if (key.startsWith("app:"))
            return key.substring(4);
        if (key.startsWith("runningApp:"))
            return key.substring(11);
        return "";
    }

    function _appDataForId(appId) {
        return root.pinnedAppMap[appId] || root.runningAppMap[appId] || {
            appId: appId,
            pinned: true,
            toplevels: []
        };
    }

    function _makeAppGroupItem(group) {
        return {
            type: "appGroup",
            groupId: group.id,
            appIds: Array.from(group.apps),
            apps: group.apps.map(appId => root._appDataForId(appId)),
            orderKey: "group:" + group.id
        };
    }

    function isGroupDropTarget(index) {
        return root.enableAppGroups && root.dragging && root._groupDropTargetIndex === index;
    }

    function groupDropIsBlocked(index) {
        return root.isGroupDropTarget(index) && root._groupDropBlocked;
    }

    function groupDropWillCreate(index) {
        return root.isGroupDropTarget(index) && root._groupDropWillCreate && !root._groupDropBlocked;
    }

    // Resolve the grouping gesture from the pointer itself: hovering the middle
    // of a groupable icon for `groupDropDwellMs` arms it, anything else leaves
    // the drag as a plain reorder. Returns the armed index, or -1.
    //
    // A dwell cannot be driven by mouse events alone — holding still produces
    // none — so entering a candidate starts a timer that re-runs this once the
    // hold is complete, and an animation that shows the hold building.
    property int _groupDropCandidateIndex: -1

    NumberAnimation {
        id: groupDwellAnimation
        target: root
        property: "_groupDropProgress"
        to: 1
        duration: root.groupDropDwellMs
        easing.type: Easing.Linear
    }

    Timer {
        id: groupArmTimer
        interval: root.groupDropDwellMs + 20
        onTriggered: root._refreshGroupDwell()
    }

    function _refreshGroupDwell() {
        if (!root.dragging)
            return;
        root.updateGroupDropTarget();
        if (root._groupDropTargetIndex >= 0)
            root._dragTargetIndex = root.dragSourceIndex;
    }

    function _resolveGroupIntent() {
        const intent = DockReorder.resolveGroupIntent(
            root._dragSlots,
            root.dragCursorX,
            root.dragSourceIndex,
            root._dragGroupable,
            root._dragState,
            {
                enabled: root.enableAppGroups && root.dragging,
                centerZone: root.groupDropCenterZone,
                dwellMs: root.groupDropDwellMs,
                now: Date.now()
            }
        );

        if (intent.candidate !== root._groupDropCandidateIndex) {
            root._groupDropCandidateIndex = intent.candidate;
            groupDwellAnimation.stop();
            groupArmTimer.stop();
            if (intent.candidate >= 0 && !intent.armed) {
                root._groupDropProgress = 0;
                groupDwellAnimation.restart();
                groupArmTimer.restart();
            } else {
                root._groupDropProgress = intent.armed ? 1 : 0;
            }
        } else if (intent.armed) {
            groupDwellAnimation.stop();
            groupArmTimer.stop();
            root._groupDropProgress = 1;
        }
        return intent.index;
    }

    // True while the pointer is holding over a groupable item but the hold is
    // not complete yet. The dock shows the intent building instead of springing
    // the whole affordance on at once.
    function isGroupDropCandidate(index) {
        return root.enableAppGroups && root.dragging
            && root._groupDropCandidateIndex === index
            && index >= 0;
    }

    function _clearGroupDropTarget() {
        root._groupDropTargetIndex = -1;
        root._groupDropWillCreate = false;
        root._groupDropBlocked = false;
    }

    function _clearGroupDwell() {
        groupDwellAnimation.stop();
        groupArmTimer.stop();
        root._groupDropCandidateIndex = -1;
        root._groupDropProgress = 0;
    }

    function updateGroupDropTarget() {
        root._clearGroupDropTarget();

        if (!root.enableAppGroups || !root.dragging) {
            root._clearGroupDwell();
            return;
        }

        const armedIndex = root._resolveGroupIntent();
        if (armedIndex < 0)
            return;

        const source = root.flattenedItems[root.dragSourceIndex];
        const target = root.flattenedItems[armedIndex];
        if (!source || source.type !== "app" || !target)
            return;

        if (target.type === "app") {
            root._groupDropTargetIndex = armedIndex;
            root._groupDropWillCreate = true;
            return;
        }

        if (target.type === "appGroup") {
            root._groupDropTargetIndex = armedIndex;
            root._groupDropBlocked = target.appIds.length >= root.maxGroupApps
                || target.appIds.includes(source.appId);
        }
    }

    function _newAppGroupId() {
        const prefix = "group-" + Date.now().toString(36);
        let candidate = prefix;
        let suffix = 1;
        while (root.appGroupById[candidate])
            candidate = prefix + "-" + String(suffix++);
        return candidate;
    }

    function _reorderAppGroupInOrder(appIds, anchorAppId, preferredKeys) {
        const currentOrder = Array.from(Config.options?.dock?.order ?? []);
        const memberIds = {};
        for (const appId of appIds)
            memberIds[appId] = true;

        let anchorIndex = -1;
        for (let i = 0; i < currentOrder.length; i++) {
            if (root._orderEntryAppId(currentOrder[i]) === anchorAppId) {
                anchorIndex = i;
                break;
            }
        }

        let insertionIndex = currentOrder.length;
        if (anchorIndex >= 0) {
            insertionIndex = 0;
            for (let i = 0; i < anchorIndex; i++) {
                if (!memberIds[root._orderEntryAppId(currentOrder[i])])
                    insertionIndex++;
            }
        }

        const groupOrderKeys = appIds.map(appId => {
            for (const entry of currentOrder) {
                if (root._orderEntryAppId(entry) === appId)
                    return entry;
            }
            const preferred = preferredKeys?.[appId] ?? "";
            if (preferred.startsWith("app:") || preferred.startsWith("runningApp:"))
                return preferred;
            return root.runningAppMap[appId] ? "runningApp:" + appId : "app:" + appId;
        });

        const nextOrder = currentOrder.filter(entry => !memberIds[root._orderEntryAppId(entry)]);
        nextOrder.splice(insertionIndex, 0, ...groupOrderKeys);
        root._writeDockOrder(nextOrder);
    }

    function isGroupAppExiting(appId) {
        return root.groupMutationPending
            && (root.groupMutationKind === "create" || root.groupMutationKind === "add")
            && root.groupMutationAppIds.includes(String(appId ?? ""));
    }

    function isGroupExiting(groupId) {
        return root.groupMutationPending
            && root.groupMutationKind === "dissolve"
            && root.groupMutationGroupId === String(groupId ?? "");
    }

    function isGroupEntryTransition(groupId) {
        return root.groupTransitionGroupId === String(groupId ?? "")
            && (root.groupTransitionKind === "create" || root.groupTransitionKind === "add");
    }

    function shouldAnimateGroupAppReturn(appId) {
        return root.groupTransitionKind === "dissolve"
            && root.groupTransitionAppIds.includes(String(appId ?? ""));
    }

    function _startGroupTransition(kind, groupId, appIds) {
        root.groupTransitionKind = kind;
        root.groupTransitionGroupId = String(groupId ?? "");
        root.groupTransitionAppIds = Array.from(appIds ?? []).map(value => String(value ?? ""));
        root.groupTransitionRevision++;
        groupTransitionClearTimer.restart();
    }

    function _queueGroupMutation(nextGroups, kind, groupId, appIds, orderRequest) {
        if (root.groupMutationPending)
            return false;

        root.groupMutationPending = true;
        root.groupMutationKind = kind;
        root.groupMutationGroupId = String(groupId ?? "");
        root.groupMutationAppIds = Array.from(appIds ?? []).map(value => String(value ?? ""));
        root.pendingGroupMutationGroups = nextGroups;
        root.pendingGroupOrderRequest = orderRequest;
        root.groupMutationRevision++;
        groupMutationTimer.restart();
        return true;
    }

    function _commitGroupMutation() {
        if (!root.groupMutationPending)
            return;

        const nextGroups = root.pendingGroupMutationGroups;
        const orderRequest = root.pendingGroupOrderRequest;
        const mutationKind = root.groupMutationKind;
        const mutationGroupId = root.groupMutationGroupId;
        const mutationAppIds = root.groupMutationAppIds;

        // Set the post-commit state before changing Config so newly created
        // delegates can read it during their first evaluation.
        const transitionKind = mutationKind === "dissolve"
            ? "dissolve"
            : (mutationKind === "create" ? "create" : "add");
        root._startGroupTransition(transitionKind, mutationGroupId, mutationAppIds);

        Config.options.dock.appGroups = nextGroups;
        if (orderRequest)
            root._reorderAppGroupInOrder(orderRequest.appIds, orderRequest.anchorAppId, orderRequest.preferredKeys);
        else
            TaskbarApps.syncPinnedFileOrder();

        root.groupMutationPending = false;
        root.groupMutationKind = "";
        root.groupMutationGroupId = "";
        root.groupMutationAppIds = [];
        root.pendingGroupMutationGroups = [];
        root.pendingGroupOrderRequest = null;
        root.groupMutationRevision++;
    }

    function completeGroupDrop() {
        if (!root.enableAppGroups || root._groupDropTargetIndex < 0)
            return false;

        const source = root.flattenedItems[root.dragSourceIndex];
        const target = root.flattenedItems[root._groupDropTargetIndex];
        if (!source || source.type !== "app" || !target || root._groupDropBlocked)
            return false;

        const sourceId = source.appId;
        let groupApps = [];
        let anchorAppId = "";
        const nextGroups = root.appGroups.map(group => ({
            id: group.id,
            apps: Array.from(group.apps)
        }));

        if (target.type === "appGroup") {
            const existing = nextGroups.find(group => group.id === target.groupId);
            if (!existing || existing.apps.length >= root.maxGroupApps || existing.apps.includes(sourceId))
                return false;
            existing.apps.push(sourceId);
            groupApps = existing.apps;
            anchorAppId = existing.apps[0];
        } else if (target.type === "app") {
            const group = {
                id: root._newAppGroupId(),
                apps: [target.appId, sourceId]
            };
            nextGroups.push(group);
            groupApps = group.apps;
            anchorAppId = target.appId;
        } else {
            return false;
        }

        const preferredKeys = {};
        preferredKeys[sourceId] = source.orderKey;
        if (target.type === "app")
            preferredKeys[target.appId] = target.orderKey;
        const transitionAppIds = target.type === "appGroup"
            ? [sourceId]
            : [target.appId, sourceId];
        return root._queueGroupMutation(
            nextGroups,
            target.type === "appGroup" ? "add" : "create",
            target.type === "appGroup" ? target.groupId : nextGroups[nextGroups.length - 1].id,
            transitionAppIds,
            {
                appIds: groupApps,
                anchorAppId: anchorAppId,
                preferredKeys: preferredKeys
            }
        );
    }

    function removeAppFromGroup(groupId, appId) {
        const targetId = String(groupId ?? "").trim();
        const targetAppId = String(appId ?? "").trim();
        if (!targetId || !targetAppId)
            return false;

        const currentGroups = Config.options?.dock?.appGroups ?? [];
        const nextGroups = [];
        let removedGroup = null;
        let removed = false;

        for (const rawGroup of currentGroups) {
            if (!rawGroup)
                continue;

            const currentId = String(rawGroup.id ?? "").trim();
            const apps = Array.from(rawGroup.apps ?? []).map(value => String(value ?? "").trim()).filter(value => value !== "");
            if (currentId !== targetId) {
                nextGroups.push({ id: currentId, apps: apps });
                continue;
            }

            removedGroup = { id: currentId, apps: apps };
            const nextApps = apps.filter(value => value !== targetAppId);
            removed = nextApps.length !== apps.length;
            // Keep a one-member group after detaching an app. This makes a
            // right-click on a popup app remove only that app; the group is
            // removed explicitly by its own right-click or when its last
            // member is detached.
            if (nextApps.length >= 1)
                nextGroups.push({ id: currentId, apps: nextApps });
        }

        if (!removed)
            return false;

        if (removedGroup && removedGroup.apps.length === 1) {
            return root._queueGroupMutation(nextGroups, "dissolve", targetId, removedGroup.apps, null);
        }

        root._startGroupTransition("remove", targetId, [targetAppId]);
        Config.options.dock.appGroups = nextGroups;
        TaskbarApps.syncPinnedFileOrder();
        return true;
    }

    function removeAppGroup(groupId) {
        const targetId = String(groupId ?? "").trim();
        if (!targetId)
            return false;

        const currentGroups = Config.options?.dock?.appGroups ?? [];
        const groupToRemove = currentGroups.find(group => group && String(group.id ?? "").trim() === targetId);
        if (!groupToRemove)
            return false;

        const nextGroups = currentGroups
            .filter(group => group && String(group.id ?? "").trim() !== targetId)
            .map(group => ({
                id: String(group.id ?? "").trim(),
                apps: Array.from(group.apps ?? []).map(value => String(value ?? "").trim()).filter(value => value !== "")
            }));

        return root._queueGroupMutation(
            nextGroups,
            "dissolve",
            targetId,
            Array.from(groupToRemove.apps ?? []),
            null
        );
    }

    function _orderKeyForAppId(order, appId) {
        for (const entry of order) {
            if (root._orderEntryAppId(entry) === appId)
                return entry;
        }
        return root.runningAppMap[appId] ? "runningApp:" + appId : "app:" + appId;
    }

    // Every reorder lands here: one write, and in Edit Mode one history entry
    // closed over copies of the order (a no-op outside the mode).
    function _writeDockOrder(nextOrder) {
        const before = Array.from(Config.options?.dock?.order ?? []);
        const after = Array.from(nextOrder).filter(entry => entry !== root.editAddKey);
        Config.options.dock.order = after;
        TaskbarApps.syncPinnedFileOrder();
        GlobalStates.editHistoryPush({
            "undo": () => { Config.options.dock.order = before; TaskbarApps.syncPinnedFileOrder(); },
            "redo": () => { Config.options.dock.order = after; TaskbarApps.syncPinnedFileOrder(); }
        });
    }

    // Commits a drop exactly as the preview showed it: the dragged item ends
    // at the target's index of the visible list, and the order is rebuilt
    // from that list (DockReorder.orderFromVisual) — dock.order alone does not
    // name every item the dock shows, so a move resolved inside it landed
    // wherever that list happened to end.
    function moveDockItem(sourceIndex, targetIndex) {
        const items = root.flattenedItems;
        const sourceItem = items[sourceIndex];
        if (items[targetIndex]?.type === "editAdd")
            targetIndex = Math.max(0, targetIndex - 1);
        const targetItem = items[targetIndex];
        if (sourceItem?.type === "editAdd")
            return false;
        if (!sourceItem || !targetItem || sourceItem.orderKey === targetItem.orderKey)
            return false;
        if (sourceItem.type === "appGroup" && targetItem.type === "appGroup"
                && sourceItem.groupId === targetItem.groupId)
            return false;

        const currentOrder = Array.from(Config.options?.dock?.order ?? []);
        const keysOf = function (item) {
            return item.type === "appGroup"
                ? item.appIds.map(appId => root._orderKeyForAppId(currentOrder, appId))
                : [String(item.orderKey)];
        };

        const sequence = items.slice();
        sequence.splice(sourceIndex, 1);
        sequence.splice(targetIndex, 0, sourceItem);
        const visualKeys = sequence
            .filter(item => item && item.__exiting !== true)
            .map(keysOf);

        const nextOrder = DockReorder.orderFromVisual(currentOrder, visualKeys, keysOf(sourceItem));
        if (nextOrder.length === currentOrder.length
                && nextOrder.every((entry, index) => entry === currentOrder[index]))
            return false;

        root._writeDockOrder(nextOrder);
        root._rememberManualPlacement(sourceItem);
        return true;
    }

    // Smart grouping keeps arranging whatever the user has not touched, and
    // leaves every hand-placed item exactly where it was dropped. Without this
    // the category sort silently reverted each drag — a widget is alone in its
    // category, so it always snapped straight back.
    function _rememberManualPlacement(item) {
        if (!Config.options?.dock?.smartGrouping || !item)
            return;
        const key = String(item.orderKey ?? "");
        if (key === "")
            return;
        const next = DockReorder.withManualKey(Config.options.dock.manualOrder ?? [], key);
        if (next.length !== (Config.options.dock.manualOrder ?? []).length)
            Config.options.dock.manualOrder = next;
    }

    // Anchors for items that are no longer in the dock would keep holding slots
    // that nothing can fill, so they are dropped as the model settles.
    function _pruneManualPlacements() {
        if (!Config.options?.dock?.smartGrouping || !Config.ready)
            return;
        const current = Config.options.dock.manualOrder ?? [];
        // An empty model means the dock has not been built yet, never that the
        // user's anchors are all stale — pruning against it would wipe them.
        if (current.length === 0 || root.modelItems.length === 0)
            return;
        const live = root.modelItems.map(function (item) { return String(item?.orderKey ?? ""); });
        const next = DockReorder.pruneManualKeys(current, live);
        if (next.length !== current.length)
            Config.options.dock.manualOrder = next;
    }

    // ── Resolve the drop target from the pointer against the drag snapshot ──
    function recomputeDragTarget() {
        if (!dragging) {
            _dragTargetIndex = dragSourceIndex;
            root.updateGroupDropTarget();
            return;
        }

        const src = dragSourceIndex;
        if (root._dragSlots.length <= 1) {
            _dragTargetIndex = src;
            root.updateGroupDropTarget();
            return;
        }

        const drop = DockReorder.resolveDropIndex(
            root._dragSlots,
            root.dragCursorX,
            src,
            root._dragState,
            { hysteresis: root.reorderHysteresis }
        );
        _dragTargetIndex = drop.index;

        root.updateGroupDropTarget();
        // Once grouping is actually armed the dock returns to its resting
        // order, which is the signal that releasing now groups instead of
        // reordering. Until then the reorder preview keeps following the
        // pointer, so merely passing over an icon costs nothing.
        if (root._groupDropTargetIndex >= 0)
            _dragTargetIndex = src;
    }

    function _resetDragState() {
        dragging = false;
        dragSourceIndex = -1;
        dragSourceKey = "";
        _dragTargetIndex = -1;
        _groupDropTargetIndex = -1;
        _groupDropWillCreate = false;
        _groupDropBlocked = false;
        dragCursorX = 0;
        dragStartCursorX = 0;
        _dragSlots = [];
        _dragGroupable = [];
        root._clearGroupDwell();
        DockReorder.resetDragState(root._dragState);
        root._refreshFlattenedItems();
    }

    // Where the dragged item's body will start once `target` has been
    // committed, derived from the same snapshot the drag was resolved against.
    // Reinserting the item pushes the run between the two positions by exactly
    // one footprint, so the landing point is exact rather than measured after
    // the fact from a delegate that no longer exists.
    function _settledStartFor(source, target) {
        const slots = root._dragSlots;
        if (!slots[source] || !slots[target])
            return NaN;
        if (target <= source)
            return slots[target].start;
        const sourceExtent = slots[source].end - slots[source].start;
        const targetExtent = slots[target].end - slots[target].start;
        return slots[target].start + targetExtent - sourceExtent;
    }

    // Hand the drop to the settle animation: the item keeps the position the
    // pointer released it at and slides into its committed slot.
    function _beginDropSettle(orderKey, source, target) {
        const slots = root._dragSlots;
        if (!slots[source]) {
            root._settleDropAt("", 0);
            return;
        }
        const releasedStart = slots[source].start + (root.dragCursorX - root.dragStartCursorX);
        const settledStart = root._settledStartFor(source, Math.max(0, target));
        if (!isFinite(settledStart)) {
            root._settleDropAt("", 0);
            return;
        }
        root._settleDropAt(orderKey, releasedStart - settledStart);
    }

    function finishDrag() {
        reorderMotionTimer.restart();
        var src = dragSourceIndex;
        var tgt = _dragTargetIndex;
        var srcEntry = (src >= 0 && src < flattenedItems.length) ? flattenedItems[src] : null;
        var droppedKey = srcEntry ? String(srcEntry.orderKey ?? "") : "";

        // The preview translation of every other item must snap the instant the
        // committed model moves them, or they would animate the same distance
        // twice. Only the dropped item keeps a visible transition, owned by the
        // settle animation.
        _suppressTranslateAnim = true;

        if (dragging && _groupDropTargetIndex >= 0 && !_groupDropBlocked) {
            _reordering = true;
            if (root.completeGroupDrop()) {
                root._settleDropAt("", 0);
                root._resetDragState();
                buttonHovered = false;
                lastHoveredButton = null;
                suppressHover = true;
                suppressHoverTimer.restart();
                Qt.callLater(function () {
                    _reordering = false;
                    _suppressTranslateAnim = false;
                });
                return;
            }
        }

        // The settle has to aim at where the item really lands, so the move is
        // committed first and a refused move settles the item back home.
        var landedIndex = src;
        if (dragging && src !== tgt) {
            _reordering = true;
            if (src >= 0 && src < flattenedItems.length && tgt >= 0 && tgt < flattenedItems.length) {
                var tgtEntry = flattenedItems[tgt];
                // An entry that is mid-exit no longer has a place in the order.
                if (srcEntry && tgtEntry && tgtEntry.__exiting !== true && srcEntry.__exiting !== true
                        && root.moveDockItem(src, tgt))
                    landedIndex = tgt;
            }
        }

        if (dragging && srcEntry)
            root._beginDropSettle(droppedKey, src, landedIndex);

        root._resetDragState();
        buttonHovered = false;
        lastHoveredButton = null;
        suppressHover = true;
        suppressHoverTimer.restart();
        Qt.callLater(function () {
            _reordering = false;
            _suppressTranslateAnim = false;
        });
    }

    function cancelDrag() {
        if (root.exportingShortcut)
            return;
        reorderMotionTimer.restart();
        // A cancelled drag is still a drag that ends somewhere: send the item
        // home with the same settle instead of teleporting it.
        if (dragging && dragSourceIndex >= 0 && dragSourceIndex < flattenedItems.length) {
            const entry = flattenedItems[dragSourceIndex];
            if (entry)
                root._beginDropSettle(String(entry.orderKey ?? ""), dragSourceIndex, dragSourceIndex);
        }
        _suppressTranslateAnim = true;
        root._resetDragState();
        Qt.callLater(function () {
            _suppressTranslateAnim = false;
        });
    }

    function startItemDrag(delegateIndex, child, eventX, eventY) {
        if (root.reorderLocked)
            return;
        const wrapper = root.getItemWrapper(delegateIndex);
        const visibleStart = wrapper ? wrapper.mapToItem(root,
            root.isVertical ? 0 : wrapper.leadingIslandGap,
            root.isVertical ? wrapper.leadingIslandGap : 0) : null;
        reorderMotionTimer.stop();
        root.reorderMotionActive = true;
        _suppressTranslateAnim = true;
        dropSettleAnimation.stop();
        root._dropSettleKey = "";
        root._dropSettleOffset = 0;
        dragSourceIndex = delegateIndex;
        dragSourceKey = String(root.flattenedItems[delegateIndex]?.orderKey ?? "");
        _dragTargetIndex = delegateIndex;
        var mapped = child.mapToItem(root, eventX, eventY);
        var mappedCoord = isVertical ? mapped.y : mapped.x;
        dragStartCursorX = mappedCoord;
        dragCursorX = mappedCoord;
        _groupDropTargetIndex = -1;
        _groupDropWillCreate = false;
        _groupDropBlocked = false;
        root._clearGroupDwell();
        dragging = true;
        DockReorder.resetDragState(root._dragState);
        root._dragState.targetIndex = delegateIndex;
        root._dragSlots = root._buildDragSlots();
        if (visibleStart && root._dragSlots[delegateIndex]) {
            const visibleMain = root.isVertical ? visibleStart.y : visibleStart.x;
            dragStartCursorX -= visibleMain - root._dragSlots[delegateIndex].start;
        }
        root._dragGroupable = root._buildGroupableFlags();
        buttonHovered = false;
        if (previewPopupLoader.item)
            previewPopupLoader.item.show = false;
        Qt.callLater(function () {
            _suppressTranslateAnim = false;
        });
    }

    function moveItemDrag(child, eventX, eventY) {
        if (!dragging)
            return;
        var mapped = child.mapToItem(root, eventX, eventY);
        const outside = root.isVertical ? (mapped.x < -16 || mapped.x > root.width + 16)
            : (mapped.y < -16 || mapped.y > root.height + 16);
        const entry = root.flattenedItems[root.dragSourceIndex];
        // A utility widget that holds files (shelf, screenshots…) hands them
        // over when it is dragged off the dock, like a pinned file does.
        const utilityExport = outside && entry?.type === "utility" ? root._utilityExport(entry.kind) : null;
        if (PanelFamily.isIi && outside && entry
            && (entry.type === "app" || entry.type === "appGroup" || entry.type === "file" || utilityExport)) {
            const appIds = entry.type === "appGroup" ? Array.from(entry.appIds) : [entry.appId];
            shortcutDrag.Drag.mimeData = utilityExport
                ? { "text/uri-list": utilityExport.uriList }
                : entry.type === "file"
                ? { "text/uri-list": "file://" + encodeURI(entry.path).replace(/#/g, "%23").replace(/\?/g, "%3F") }
                : { "application/x-ii-desktop-shortcut": JSON.stringify({
                    type: entry.type === "appGroup" ? "group" : "app", apps: appIds }) };
            shortcutDrag.Drag.imageSource = Quickshell.iconPath(utilityExport ? utilityExport.icon
                : entry.type === "file" ? "folder"
                : TaskbarApps.getCachedIcon(appIds[0]), "image-missing");
            root.exportingShortcut = true;
            root._clearGroupDwell();
            // Native QDrag owns the pointer until release/cancel. Keep the model
            // frozen until it returns; exporting never changes pins or dock order.
            shortcutDrag.Drag.active = true;
            shortcutDrag.Drag.startDrag(Qt.CopyAction);
            shortcutDrag.Drag.active = false;
            root.exportingShortcut = false;
            shortcutDrag.Drag.mimeData = {};
            shortcutDrag.Drag.imageSource = "";
            root.cancelDrag();
            return;
        }
        dragCursorX = isVertical ? mapped.y : mapped.x;
        recomputeDragTarget();
    }

    function endItemDrag() {
        if (root.exportingShortcut || !root.dragging)
            return;
        finishDrag();
    }

    function mapDragToRoot(item, x, y) {
        return item.mapToItem(root, x, y);
    }

    // ── Flattened items model ─────────────────────────────────────────────
    readonly property string dockOutputName: String(root.currentScreen?.name ?? "")
    readonly property bool isolateMonitors: Config.options?.dock?.isolateMonitors ?? false
    readonly property var scopedApps: {
        const allApps = TaskbarApps.apps ?? [];
        if (!root.isolateMonitors || !root.dockOutputName)
            return allApps;

        const output = RyokuUi.Wm.outputByName(root.dockOutputName);
        if (!output)
            return allApps;

        const targetName = String(output.name ?? root.dockOutputName);
        const windowOutputs = new Map();
        for (const window of (RyokuUi.Wm.windows ?? [])) {
            if (window?.toplevel)
                windowOutputs.set(window.toplevel, String(window.output ?? ""));
        }

        const result = [];
        for (const app of allApps) {
            const toplevels = Array.from(app.toplevels ?? []).filter(toplevel => {
                const name = windowOutputs.get(toplevel) ?? toplevel.output;
                // Keep an unattributed window visible until Wm can join it.
                return name === undefined || name === "" || name === targetName;
            });
            if (app.pinned || toplevels.length > 0) {
                result.push({
                    appId: app.appId,
                    toplevels: toplevels,
                    pinned: app.pinned
                });
            }
        }
        return result;
    }

    readonly property var pinnedAppMap: {
        var m = {};
        var allApps = root.scopedApps;
        for (var i = 0; i < allApps.length; i++) {
            var a = allApps[i];
            if (a.pinned)
                m[a.appId] = a;
        }
        return m;
    }

    readonly property var runningAppMap: {
        var m = {};
        var allApps = root.scopedApps;
        for (var i = 0; i < allApps.length; i++) {
            var a = allApps[i];
            if (!a.pinned && a.toplevels && a.toplevels.length > 0)
                m[a.appId] = a;
        }
        return m;
    }

    // Groups are persisted independently from dock.order. The order keeps
    // every app key, which means disabling groups can safely reveal members
    // individually without destroying the user's group definitions.
    readonly property var appGroups: {
        const rawGroups = Config.options?.dock?.appGroups ?? [];
        const groups = [];
        const seenApps = {};

        for (const rawGroup of rawGroups) {
            if (!rawGroup)
                continue;
            const groupId = String(rawGroup.id ?? "").trim();
            if (!groupId)
                continue;

            const apps = [];
            for (const rawAppId of Array.from(rawGroup.apps ?? [])) {
                const appId = String(rawAppId ?? "").trim();
                if (!appId || seenApps[appId] || apps.includes(appId))
                    continue;
                apps.push(appId);
                seenApps[appId] = true;
                if (apps.length >= root.maxGroupApps)
                    break;
            }

            if (apps.length >= 1)
                groups.push({
                    id: groupId,
                    apps: apps
                });
        }
        return groups;
    }

    readonly property var appGroupByAppId: {
        const result = {};
        for (const group of root.appGroups) {
            for (const appId of group.apps)
                result[appId] = group;
        }
        return result;
    }

    readonly property var appGroupById: {
        const result = {};
        for (const group of root.appGroups)
            result[group.id] = group;
        return result;
    }

    readonly property var pinnedFileMap: {
        var m = {};
        var files = Config.options?.dock?.pinnedFiles ?? [];
        for (var i = 0; i < files.length; i++)
            m[files[i]] = {
                path: files[i]
            };
        return m;
    }

    // Islands mode presents launchers as one continuous app surface. The
    // persisted order may contain widgets or actions between launcher keys,
    // so coalesce the derived visual list without changing dock.order. This
    // also keeps a group whose keys were appended by an older config version
    // with the other pinned/running launchers.
    function _coalesceLauncherItems(items) {
        if (!root.islandsStyle || !items || items.length < 2)
            return items;

        const launchers = [];
        let firstLauncherIndex = -1;
        for (let i = 0; i < items.length; i++) {
            if (!root._isLauncherItem(items[i]))
                continue;
            if (firstLauncherIndex < 0)
                firstLauncherIndex = i;
            launchers.push(items[i]);
        }

        if (launchers.length < 2)
            return items;

        const result = items.filter(item => !root._isLauncherItem(item));
        result.splice(firstLauncherIndex, 0, ...launchers);
        return result;
    }

    readonly property var modelItems: {
        var result = [];
        var order = Config.options.dock.order ?? [];
        var allApps = root.scopedApps;
        var allAppIds = [];
        for (var i = 0; i < allApps.length; i++)
            allAppIds.push(allApps[i].appId);

        // Track seen orderKeys to avoid duplicates
        var seenOrderKeys = {};
        var seenGroupIds = {};

        // Pre-scan explicit running apps and apps to avoid them being swallowed by "runningApps" marker
        var explicitKeys = {};
        for (var e_i = 0; e_i < order.length; e_i++) {
            if (order[e_i].startsWith("runningApp:") || order[e_i].startsWith("app:")) {
                explicitKeys[order[e_i]] = true;
            }
        }

        const orderHasWidgetStack = order.indexOf("widgetStack") >= 0;
        for (var oi = 0; oi < order.length; oi++) {
            var entry = order[oi];

            if (root.enableAppGroups && (entry.startsWith("app:") || entry.startsWith("runningApp:"))) {
                const groupedAppId = root._orderEntryAppId(entry);
                const groupedApp = root.appGroupByAppId[groupedAppId];
                if (groupedApp) {
                    if (!seenGroupIds[groupedApp.id]) {
                        result.push(root._makeAppGroupItem(groupedApp));
                        seenGroupIds[groupedApp.id] = true;
                    }
                    for (const memberId of groupedApp.apps) {
                        seenOrderKeys["app:" + memberId] = true;
                        seenOrderKeys["runningApp:" + memberId] = true;
                    }
                    continue;
                }
            }

            // The stack sits where its own key was dropped; before it was
            // ever dragged, where its first member used to be.
            if (entry === "widgetStack" || root._inWidgetStack(entry)) {
                if (!seenOrderKeys["widgetStack"] && root.widgetStackMembers.length > 0
                        && (entry === "widgetStack" || !orderHasWidgetStack)) {
                    result.push({
                        type: "widgetStack",
                        orderKey: "widgetStack"
                    });
                    seenOrderKeys["widgetStack"] = true;
                }
                continue;
            }

            if (entry === "pin" && root.showPin) {
                result.push({
                    type: "action",
                    actionId: "pin",
                    orderKey: "pin"
                });
                seenOrderKeys["pin"] = true;
            } else if (entry === "trash" && root.showTrash) {
                result.push({
                    type: "action",
                    actionId: "trash",
                    orderKey: "trash"
                });
                seenOrderKeys["trash"] = true;
            } else if (entry === "overview" && root.showOverview) {
                result.push({
                    type: "action",
                    actionId: "overview",
                    orderKey: "overview"
                });
                seenOrderKeys["overview"] = true;
            } else if (entry === "media" && root.showMedia) {
                result.push({
                    type: "media",
                    orderKey: "media"
                });
                seenOrderKeys["media"] = true;
            } else if (entry === "weather" && root.showWeather) {
                result.push({
                    type: "weather",
                    orderKey: "weather"
                });
                seenOrderKeys["weather"] = true;
            } else if (entry === "sports" && root.showSports) {
                result.push({
                    type: "sports",
                    orderKey: "sports"
                });
                seenOrderKeys["sports"] = true;
            } else if (entry === "livePreview" && root.showLivePreview) {
                result.push({
                    type: "livePreview",
                    orderKey: "livePreview"
                });
                seenOrderKeys["livePreview"] = true;
            } else if (root.utilityEntryByKey[entry]) {
                const utility = root._utilityItem(root.utilityEntryByKey[entry]);
                if (!seenOrderKeys[utility.orderKey]) {
                    result.push(utility);
                    seenOrderKeys[utility.orderKey] = true;
                }
            } else if (entry === "tasks" && root.showTasks) {
                result.push({
                    type: "tasks",
                    orderKey: "tasks"
                });
                seenOrderKeys["tasks"] = true;
            } else if (entry === "phone" && root.showPhone) {
                result.push({
                    type: "phone",
                    orderKey: "phone"
                });
                seenOrderKeys["phone"] = true;
            } else if (entry === "runningApps") {
                // The legacy runningApps marker is ignored.
                // Unpinned apps will be handled by the smart append logic at the end,
                // grouping them correctly after the last explicit app icon.
            } else if (entry.startsWith("runningApp:")) {
                // Individual running app that was previously reordered
                var runningAppId = entry.substring(11);
                if (!seenOrderKeys[entry]) {
                    var runningAppData = runningAppMap[runningAppId];
                    if (runningAppData) {
                        result.push({
                            type: "app",
                            appId: runningAppId,
                            appData: runningAppData,
                            orderKey: entry
                        });
                        seenOrderKeys[entry] = true;
                        // Also mark the app: variant to prevent duplicates
                        seenOrderKeys["app:" + runningAppId] = true;
                    }
                    // If app is not running anymore, skip (cleanup)
                }
            } else if (entry.startsWith("app:")) {
                var appId = entry.substring(4);
                var appKey = "app:" + appId;
                if (!seenOrderKeys[appKey]) {
                    var appData = pinnedAppMap[appId] || runningAppMap[appId];
                    if (appData || allAppIds.indexOf(appId) !== -1) {
                        result.push({
                            type: "app",
                            appId: appId,
                            appData: appData || {
                                appId: appId,
                                pinned: true,
                                toplevels: []
                            },
                            orderKey: appKey
                        });
                        seenOrderKeys[appKey] = true;
                        // Also mark the runningApp variant to prevent duplicates
                        // (e.g. when an app goes from pinned→unpinned but stays running)
                        seenOrderKeys["runningApp:" + appId] = true;
                    }
                }
            } else if (entry.startsWith("file:")) {
                var path = entry.substring(5);
                var fileKey = "file:" + path;
                if (!seenOrderKeys[fileKey] && pinnedFileMap[path]) {
                    result.push({
                        type: "file",
                        path: path,
                        orderKey: fileKey
                    });
                    seenOrderKeys[fileKey] = true;
                }
            }
        }

        // Keep a group visible even if all of its app keys were removed from
        // dock.order by an older version or a manual config edit.
        if (root.enableAppGroups) {
            for (const group of root.appGroups) {
                if (seenGroupIds[group.id])
                    continue;
                result.push(root._makeAppGroupItem(group));
                seenGroupIds[group.id] = true;
                for (const memberId of group.apps) {
                    seenOrderKeys["app:" + memberId] = true;
                    seenOrderKeys["runningApp:" + memberId] = true;
                }
            }
        }

        // Append any running apps and pinned apps that weren't in the order at all
        var remainingApps = [];
        var remainingRas = Object.values(runningAppMap);
        for (var rj = 0; rj < remainingRas.length; rj++) {
            var rKey = "runningApp:" + remainingRas[rj].appId;
            if (!seenOrderKeys[rKey]) {
                remainingApps.push({
                    type: "app",
                    appId: remainingRas[rj].appId,
                    appData: remainingRas[rj],
                    orderKey: rKey
                });
                seenOrderKeys[rKey] = true;
                seenOrderKeys["app:" + remainingRas[rj].appId] = true;
            }
        }

        var remainingPinned = Object.values(pinnedAppMap);
        for (var pk = 0; pk < remainingPinned.length; pk++) {
            var pKey = "app:" + remainingPinned[pk].appId;
            if (!seenOrderKeys[pKey]) {
                remainingApps.push({
                    type: "app",
                    appId: remainingPinned[pk].appId,
                    appData: remainingPinned[pk],
                    orderKey: pKey
                });
                seenOrderKeys[pKey] = true;
                seenOrderKeys["runningApp:" + remainingPinned[pk].appId] = true;
            }
        }

        if (remainingApps.length > 0) {
            var targetIndex = -1;
            for (var idx = result.length - 1; idx >= 0; idx--) {
                if (result[idx].type === "app") {
                    targetIndex = idx + 1;
                    break;
                }
            }
            if (targetIndex === -1) {
                targetIndex = result.length;
                while (targetIndex > 0) {
                    var item = result[targetIndex - 1];
                    if (item.type === "action" && (item.actionId === "trash" || item.actionId === "overview" || item.actionId === "pin")) {
                        targetIndex--;
                    } else {
                        break;
                    }
                }
            }
            for (var m = 0; m < remainingApps.length; m++) {
                result.splice(targetIndex + m, 0, remainingApps[m]);
            }
        }

        // Append pinned files that aren't in the order at all (e.g. newly pinned folders)
        var remainingFiles = [];
        var pinnedFiles = Object.values(pinnedFileMap);
        for (var fk = 0; fk < pinnedFiles.length; fk++) {
            var fKey = "file:" + pinnedFiles[fk].path;
            if (!seenOrderKeys[fKey]) {
                remainingFiles.push({
                    type: "file",
                    path: pinnedFiles[fk].path,
                    orderKey: fKey
                });
                seenOrderKeys[fKey] = true;
            }
        }

        if (remainingFiles.length > 0) {
            var fileTargetIndex = -1;
            for (var fidx = result.length - 1; fidx >= 0; fidx--) {
                if (result[fidx].type === "file") {
                    fileTargetIndex = fidx + 1;
                    break;
                }
            }
            if (fileTargetIndex === -1) {
                fileTargetIndex = result.length;
                while (fileTargetIndex > 0) {
                    var fitem = result[fileTargetIndex - 1];
                    if (fitem.type === "action" && (fitem.actionId === "trash" || fitem.actionId === "overview" || fitem.actionId === "pin")) {
                        fileTargetIndex--;
                    } else {
                        break;
                    }
                }
            }
            for (var n = 0; n < remainingFiles.length; n++) {
                result.splice(fileTargetIndex + n, 0, remainingFiles[n]);
            }
        }

        // Existing users may have a dock.order saved before the sports item
        // existed. Keep the new shortcut discoverable without rewriting their
        // entire order list; a later drag persists its explicit position.
        if (root.showSports && !seenOrderKeys["sports"]) {
            const sportsItem = {
                type: "sports",
                orderKey: "sports"
            };
            let sportsTargetIndex = result.length;
            while (sportsTargetIndex > 0) {
                const item = result[sportsTargetIndex - 1];
                if (item.type === "action"
                        && (item.actionId === "trash"
                            || item.actionId === "overview"
                            || item.actionId === "pin")) {
                    sportsTargetIndex--;
                } else {
                    break;
                }
            }
            result.splice(sportsTargetIndex, 0, sportsItem);
            seenOrderKeys["sports"] = true;
        }

        // Existing users may have a dock.order saved before the phone item
        // existed. Keep the new shortcut discoverable without rewriting their
        // entire order list; a later drag persists its explicit position.
        if (root.showPhone && !seenOrderKeys["phone"]) {
            const phoneItem = {
                type: "phone",
                orderKey: "phone"
            };
            let phoneTargetIndex = result.length;
            while (phoneTargetIndex > 0) {
                const item = result[phoneTargetIndex - 1];
                if (item.type === "action"
                        && (item.actionId === "trash"
                            || item.actionId === "overview"
                            || item.actionId === "pin")) {
                    phoneTargetIndex--;
                } else {
                    break;
                }
            }
            result.splice(phoneTargetIndex, 0, phoneItem);
            seenOrderKeys["phone"] = true;
        }

        // Existing users may have a dock.order saved before the live preview
        // item existed. Add it to the derived model near sports/phone without
        // rewriting the persisted order list.
        if (root.showLivePreview && !seenOrderKeys["livePreview"]) {
            const livePreviewItem = {
                type: "livePreview",
                orderKey: "livePreview"
            };
            let livePreviewTargetIndex = result.length;
            const phoneIndex = result.findIndex(item => item.type === "phone");
            if (phoneIndex >= 0) {
                livePreviewTargetIndex = phoneIndex;
            } else {
                while (livePreviewTargetIndex > 0) {
                    const item = result[livePreviewTargetIndex - 1];
                    if (item.type === "action"
                            && (item.actionId === "trash"
                                || item.actionId === "overview"
                                || item.actionId === "pin")) {
                        livePreviewTargetIndex--;
                    } else {
                        break;
                    }
                }
            }
            result.splice(livePreviewTargetIndex, 0, livePreviewItem);
            seenOrderKeys["livePreview"] = true;
        }

        // Widgets new to an order saved before they existed (or never placed)
        // land before the trailing actions, without rewriting the order.
        const trailingInsertIndex = function () {
            let index = result.length;
            while (index > 0) {
                const item = result[index - 1];
                if (item.type === "action"
                        && (item.actionId === "trash" || item.actionId === "overview" || item.actionId === "pin"))
                    index--;
                else
                    break;
            }
            return index;
        };
        if (root.showTasks && !seenOrderKeys["tasks"]) {
            result.splice(trailingInsertIndex(), 0, { type: "tasks", orderKey: "tasks" });
            seenOrderKeys["tasks"] = true;
        }
        for (const entry of root.utilityEntries) {
            const key = DockUtilityCatalog.orderKey(entry.kind);
            if (!seenOrderKeys[key]) {
                result.splice(trailingInsertIndex(), 0, root._utilityItem(entry));
                seenOrderKeys[key] = true;
            }
        }
        if (root.widgetStackMembers.length > 0 && !seenOrderKeys["widgetStack"]) {
            result.splice(trailingInsertIndex(), 0, { type: "widgetStack", orderKey: "widgetStack" });
            seenOrderKeys["widgetStack"] = true;
        }

        if (Config.options?.dock?.smartGrouping) {
            result = DockReorder.applySmartGrouping(
                result,
                result.map(function (el) { return root.getItemCategory(el); }),
                Config.options?.dock?.manualOrder ?? []
            );
        }

        const items = root._coalesceLauncherItems(result);
        // Edit Mode's (+): the way to put a widget on the dock from the dock.
        // Never part of dock.order (_writeDockOrder drops its key).
        if (root.editLayerActive)
            items.push({ type: "editAdd", orderKey: root.editAddKey });
        return items;
    }

    // ── Presence transitions ───────────────────────────────────────────────
    // A Repeater destroys a delegate the moment its entry leaves the model, so
    // an app closing used to blink out of existence. Removed entries are handed
    // back here for one animation cycle, flagged and pinned to the index they
    // held, which gives their delegate time to shrink out of the row before the
    // model really loses them.
    readonly property int itemTransitionDuration: Appearance.animation.elementMoveFast.duration
    property bool _itemTransitionsReady: false
    property var _modelSnapshot: []
    property var _exitingRecords: []
    property var _enteringKeys: ({})

    // Only genuinely new identities enter; payload and order updates preserve
    // the existing delegates, pointer grabs and open menus.
    function _isEnteringKey(orderKey) {
        const at = root._enteringKeys[String(orderKey ?? "")];
        return at !== undefined && (Date.now() - at) < root.itemTransitionDuration;
    }

    // Deliberately assigned, not bound. A binding on `modelItems` would be
    // re-evaluated before the handler below records what just disappeared, so
    // the Repeater would rebuild once without the removed item and again with
    // it — the icon would blink out and only then start animating away.
    property var flattenedItems: []

    StableDockModel {
        id: dockItemModel
        sourceValues: root.flattenedItems
        keyFunction: value => DockReorder.identity(value)
    }

    function _refreshFlattenedItems() {
        // Preserve the gesture snapshot when apps/windows change in the background.
        if (root.dragging && !root._reordering)
            return;
        root.flattenedItems = root._exitingRecords.length === 0
            ? root.modelItems
            : DockReorder.mergeExitingItems(
                root.modelItems,
                root._exitingRecords,
                Date.now(),
                root.itemTransitionDuration + 200
            );
    }

    onModelItemsChanged: {
        const previous = root._modelSnapshot;
        root._modelSnapshot = root.modelItems.slice();
        if (!root._itemTransitionsReady) {
            root._enteringKeys = ({});
            root._refreshFlattenedItems();
            return;
        }

        const now = Date.now();
        root._enteringKeys = DockReorder.collectAddedKeys(previous, root.modelItems, now);

        const removed = DockReorder.collectRemovedItems(previous, root.modelItems, now);
        if (removed.length > 0) {
            root._exitingRecords = root._exitingRecords.concat(removed);
            exitPurgeTimer.restart();
        }
        root._refreshFlattenedItems();
    }

    Component.onCompleted: {
        root._refreshFlattenedItems();
        GlobalStates.registerDockContent(root);
    }
    Component.onDestruction: GlobalStates.unregisterDockContent(root)

    Timer {
        id: exitPurgeTimer
        interval: root.itemTransitionDuration + 80
        onTriggered: {
            root._exitingRecords = [];
            root._refreshFlattenedItems();
            root._pruneManualPlacements();
        }
    }

    // Nothing animates in during the first build: the dock arriving one icon at
    // a time on startup is noise, not feedback.
    Timer {
        id: itemTransitionsReadyTimer
        interval: 700
        running: true
        onTriggered: {
            root._itemTransitionsReady = true;
            root._migrateUtilityKeys();
            root._pruneManualPlacements();
        }
    }

    // ── Separator helpers ──────────────────────────────────────────────────
    function isSpecialItem(item) {
        if (!item)
            return false;
        var t = item.type;
        return t === "media" || t === "weather" || t === "sports" || t === "livePreview" || t === "phone" || t === "action"
            || t === "tasks" || t === "widgetStack" || t === "utility";
    }

    function getItemCategory(item) {
        if (!item)
            return 99;
        var t = item.type;

        if (t === "action" && item.actionId === "overview")
            return 1;
        if (t === "action" && item.actionId === "pin")
            return 2;
        if (t === "weather")
            return 3;
        if (t === "sports")
            return 4;
        if (t === "livePreview")
            return 5;
        if (t === "tasks")
            return 6;
        if (t === "utility")
            return 7;
        if (t === "widgetStack")
            return 3;
        if (t === "phone")
            return 25;
        if (t === "appGroup" && item.appIds?.length > 0)
            return root.getItemCategory({
                type: "app",
                appId: item.appIds[0]
            });

        var id = "";
        if (t === "app" && item.appId)
            id = item.appId.toLowerCase();

        if (id.match(/(firefox|chrome|chromium|edge|brave|librewolf|vivaldi|opera|waterfox|tor|safari|thorium|zen)/))
            return 10;
        if (t === "file" || id.match(/(dolphin|nautilus|thunar|pcmanfm|nemo|caja|kitty|alacritty|konsole|wezterm|foot|terminal|files)/))
            return 20;
        if (id.match(/(code|vscode|vscodium|idea|intellij|pycharm|webstorm|neovim|nvim|vim|emacs|sublime|notepadqq|kate|kwrite|gedit|geany|zed)/))
            return 30;
        if (id.match(/(discord|vesktop|slack|telegram|whatsapp|signal|teams|element|skype|mattermost)/))
            return 40;
        if (id.match(/(gimp|inkscape|krita|kdenlive|davinci|obs|blender|audacity|lmms|figma)/))
            return 50;
        if (t === "media" || id.match(/(spotify|youtube-music|vlc|mpv|spotify-launcher|amberol|elisa|lollypop|rhythmbox|audacious|cider|mpd)/))
            return 60;
        if (id.match(/(steam|heroic|lutris|epic|minigalaxy|prismlauncher|bottles)/))
            return 70;

        if (t === "action" && item.actionId === "trash")
            return 100;

        if (t === "app")
            return 80;
        return 90;
    }

    function mimeIconFromPath(path) {
        const p = (path ?? "").toString().toLowerCase();
        if (/\.(png|jpe?g|webp|gif|svg|bmp|ico)$/.test(p))
            return "image";
        if (/\.(mp3|flac|ogg|wav|aac|m4a)$/.test(p))
            return "music_note";
        if (/\.(mp4|mkv|webm|avi|mov)$/.test(p))
            return "movie";
        if (p.endsWith(".pdf"))
            return "picture_as_pdf";
        if (/\.(txt|md|rst|log)$/.test(p))
            return "description";
        if (/\.(zip|tar|gz|zst|rar|7z)$/.test(p))
            return "folder_zip";
        const last = p.split("/").filter(s => s).pop() || "";
        return last.includes(".") ? "insert_drive_file" : "folder";
    }

    // ── Layout ────────────────────────────────────────────────────────────
    Flickable {
        id: scrollArea
        anchors.fill: parent
        clip: false
        // Full width lays the content out on a canvas as wide as the tray so the
        // row can be centred on it; the content is narrower than the tray there,
        // so nothing scrolls and contentX stays where the lens expects it.
        contentWidth: root.isVertical ? parent.width : Math.max(unifiedRow.width, parent.width)
        contentHeight: root.isVertical ? Math.max(unifiedColumn.height, parent.height) : parent.height
        interactive: root.isVertical ? contentHeight > height : contentWidth > width
        flickableDirection: root.isVertical ? Flickable.VerticalFlick : Flickable.HorizontalFlick

        WheelHandler {
            onWheel: event => {
                if (DockPresets.canSwitchPresets) {
                    if (DockPresets.handleWheelScroll(event.angleDelta.y, event.angleDelta.x)) {
                        event.accepted = true;
                        return;
                    }
                }
                const vertical = event.angleDelta.y !== 0;
                const d = ScrollWheel.step(vertical ? event.angleDelta.y : event.angleDelta.x,
                    vertical ? event.pixelDelta.y : event.pixelDelta.x, Config.options?.interactions?.scrolling, event.phase);
                if (root.isVertical)
                    scrollArea.contentY = Math.max(0, Math.min(scrollArea.contentHeight - scrollArea.height, scrollArea.contentY - d));
                else
                    scrollArea.contentX = Math.max(0, Math.min(scrollArea.contentWidth - scrollArea.width, scrollArea.contentX - d));
                event.accepted = true;
            }
        }

        // Island surfaces live in the same content coordinates as the flat
        // Row/Column, so they scroll together with their delegates.
        Item {
            id: islandLayer
            z: -1
            width: root.isVertical ? parent.width : unifiedRow.width
            height: root.isVertical ? unifiedColumn.height : parent.height

            Repeater {
                // Island surfaces are an alternate style. Do not instantiate
                // one hidden surface per segment for floating/transparent
                // docks, where neither the surface nor its shadow is used.
                model: root.islandsStyle ? root.islandSegments : null

                delegate: DockIslandSurface {
                    required property var modelData

                    readonly property var activeRepeater: root._getActiveRepeater()
                    readonly property var firstWrapper: {
                        const repeater = activeRepeater;
                        if (!repeater)
                            return null;
                        repeater.count;
                        return root.getItemWrapper(modelData.startIndex);
                    }
                    readonly property var lastWrapper: {
                        const repeater = activeRepeater;
                        if (!repeater)
                            return null;
                        repeater.count;
                        return root.getItemWrapper(modelData.endIndex);
                    }
                    readonly property bool endpointGeometryReady: !!firstWrapper
                        && !!lastWrapper
                        && firstWrapper.bodyMainEnd > firstWrapper.bodyMainStart
                        && lastWrapper.bodyMainEnd > lastWrapper.bodyMainStart

                    active: root.islandsStyle && endpointGeometryReady
                    islandKind: modelData.kind
                    dockPosition: root.dockPos
                    cornerRadius: root.dockCornerRadius
                    // A live lens resizes this surface every frame; the shadow
                    // then has to be drawn rather than cached into a texture
                    // that would be thrown away on the next frame.
                    resizing: root.enableMagnification && root.magnificationStrength > 0.001
                    x: root.isVertical ? 0 : (firstWrapper?.bodyMainStart ?? 0)
                    y: root.isVertical ? (firstWrapper?.bodyMainStart ?? 0) : 0
                    width: root.isVertical
                        ? root.buttonSlotSize
                        : Math.max(0, (lastWrapper?.bodyMainEnd ?? 0) - (firstWrapper?.bodyMainStart ?? 0))
                    height: root.isVertical
                        ? Math.max(0, (lastWrapper?.bodyMainEnd ?? 0) - (firstWrapper?.bodyMainStart ?? 0))
                        : root.buttonSlotHeight
                }
            }
        }

        // Transparent drag handles sit on the outer padding of each island.
        // Keeping the handle out of the icon hitboxes preserves app clicks,
        // previews and app-group creation while making the island itself movable.
        Item {
            id: islandInteractionLayer
            z: 2
            width: root.isVertical ? parent.width : unifiedRow.width
            height: root.isVertical ? unifiedColumn.height : parent.height

            Repeater {
                // The interaction layer is just as expensive as the visual
                // layer when it creates a DragHandler per island. Keep it
                // empty until islands are the selected dock style.
                model: root.islandsStyle ? root.islandSegments : null

                delegate: Item {
                    required property var modelData

                    readonly property var activeRepeater: root._getActiveRepeater()
                    readonly property var firstWrapper: {
                        const repeater = activeRepeater;
                        if (!repeater)
                            return null;
                        repeater.count;
                        return root.getItemWrapper(modelData.startIndex);
                    }
                    readonly property var lastWrapper: {
                        const repeater = activeRepeater;
                        if (!repeater)
                            return null;
                        repeater.count;
                        return root.getItemWrapper(modelData.endIndex);
                    }
                    readonly property bool endpointGeometryReady: !!firstWrapper
                        && !!lastWrapper
                        && firstWrapper.bodyMainEnd > firstWrapper.bodyMainStart
                        && lastWrapper.bodyMainEnd > lastWrapper.bodyMainStart

                    enabled: root.islandsStyle && endpointGeometryReady
                    x: root.isVertical ? 0 : (firstWrapper?.bodyMainStart ?? 0)
                    y: root.isVertical ? (firstWrapper?.bodyMainStart ?? 0) : 0
                    width: root.isVertical
                        ? root.buttonSlotSize
                        : Math.max(0, (lastWrapper?.bodyMainEnd ?? 0) - (firstWrapper?.bodyMainStart ?? 0))
                    height: root.isVertical
                        ? Math.max(0, (lastWrapper?.bodyMainEnd ?? 0) - (firstWrapper?.bodyMainStart ?? 0))
                        : root.buttonSlotHeight

                    Item {
                        id: islandDragStrip
                        width: root.isVertical
                            ? Math.max(root.dotMargin, root.sepThickness * 2)
                            : parent.width
                        height: root.isVertical
                            ? parent.height
                            : Math.max(root.dotMarginV, root.sepThickness * 2)

                        // On the island's edge away from the screen. Positions, not
                        // anchors: an anchor set back to undefined keeps the old
                        // coordinate, so a strip moved with the dock stayed put.
                        x: root.dockPos === "left" ? parent.width - width : 0
                        y: root.dockPos === "top" ? parent.height - height : 0

                        HoverHandler {
                            cursorShape: islandDragHandler.active ? Qt.ClosedHandCursor : Qt.OpenHandCursor
                        }

                        DragHandler {
                            id: islandDragHandler
                            enabled: parent.parent.enabled
                                && !root.dragging
                                && !root.anyContextMenuOpen
                            target: null
                            acceptedButtons: Qt.LeftButton
                            grabPermissions: PointerHandler.CanTakeOverFromAnything
                            xAxis.enabled: !root.isVertical
                            yAxis.enabled: root.isVertical

                            onActiveChanged: {
                                if (active)
                                    root.startIslandDrag(modelData.id, centroid.scenePosition);
                                else
                                    root.endIslandDrag();
                            }

                            onCentroidChanged: {
                                if (active)
                                    root.moveIslandDrag(centroid.scenePosition);
                            }
                        }
                    }
                }
            }
        }

        Row {
            id: unifiedRow
            visible: !root.isVertical
            spacing: (Config.options && Config.options.dock && Config.options.dock.iconSpacing !== undefined) ? Config.options.dock.iconSpacing : 0
            // The flat styles size the tray to the content, so the row sits at
            // its edge; a full width tray spans the screen and the row is
            // centred on it - which is also the centre the magnification
            // pointer mapping reads.
            // A position, not an anchor: setting an anchor back to undefined
            // leaves the item where it was, so leaving full width kept the
            // centring offset until a restart - the items sat that far into
            // a tray sized to them, the islands drawn without them.
            x: root.isFullWidth ? Math.max(0, (parent.width - width) / 2) : 0

            Repeater {
                id: itemRepeater
                // `visible: false` on the Row does not stop a Repeater from
                // creating delegates. Keep the inactive orientation empty so
                // each dock item (and its nested Loader tree) exists once.
                model: root.isVertical ? null : dockItemModel
                delegate: unifiedItemDelegate
                onItemAdded: root._itemDelegateRevision++
                onItemRemoved: root._itemDelegateRevision++
            }
        }

        Column {
            id: unifiedColumn
            visible: root.isVertical
            spacing: (Config.options && Config.options.dock && Config.options.dock.iconSpacing !== undefined) ? Config.options.dock.iconSpacing : 0
            // A position, not an anchor (see unifiedRow).
            y: root.isFullWidth ? Math.max(0, (parent.height - height) / 2) : 0

            Repeater {
                id: columnItemRepeater
                // The vertical and horizontal layouts are alternatives, not
                // two render targets. A null model avoids duplicating every
                // delegate; only a deliberate orientation change recreates
                // the delegates, while reveal/reorder animations stay on the
                // same active repeater.
                model: root.isVertical ? dockItemModel : null
                delegate: unifiedItemDelegate
                onItemAdded: root._itemDelegateRevision++
                onItemRemoved: root._itemDelegateRevision++
            }
        }
    }

    // ── Unified item delegate ──────────────────────────────────────────────
    Component {
        id: unifiedItemDelegate

        Item {
            id: delegateWrapper
            required property string entryKey
            required property string entryType
            required property int index
            readonly property int delegateIndex: index
            readonly property var itemData: dockItemModel.itemsByKey[entryKey] ?? ({ type: entryType })
            readonly property real itemWidth: {
                if (root.isVertical)
                    return root.buttonSlotSize;
                switch (itemData.type) {
                case "media":
                case "weather":
                    return root.buttonSlotSize * 3;
                case "sports":
                    return root.buttonSlotSize * root.sportsWidgetSlots;
                case "livePreview":
                    return root.buttonSlotSize * root.livePreviewWidgetSlots;
                case "tasks":
                    return root.buttonSlotSize * 3;
                case "widgetStack":
                    return root.widgetStackExtent;
                case "utility":
                    return root.buttonSlotSize * root._utilitySlots(itemData);
                default:
                    return root.buttonSlotSize;
                }
            }
            readonly property real itemHeight: root.isVertical ? root.buttonSlotSize : root.buttonSlotHeight
            readonly property bool magnifiable: root._isMagnifiableItem(delegateWrapper.itemData)
            readonly property var islandMeta: root._islandMetaForIndex(delegateIndex)
            readonly property string islandId: islandMeta?.id ?? ""
            readonly property bool isIslandStart: islandMeta?.startIndex === delegateIndex
            readonly property real targetLeadingIslandGap: root._leadingIslandGapForIndex(delegateIndex)
            property real animatedLeadingIslandGap: targetLeadingIslandGap
            readonly property real leadingIslandGap: animatedLeadingIslandGap
            readonly property real baseBodyMainExtent: root._baseItemMainExtentForIndex(delegateIndex)
            readonly property real baseMainExtent: root.baseMetrics.items[delegateIndex]?.baseExtent ?? (root.isVertical ? root.buttonSlotSize : itemWidth)
            readonly property real magWeight: root._magnificationWeightForIndex(delegateIndex)
            // The lens share this item's content takes: 1 for an icon, less for
            // a widget drawn as one wide card, 0 when it never magnifies.
            readonly property real magContentFactor: root._magnificationContentFactorForIndex(delegateIndex)
            readonly property real magExtent: root.baseMetrics.items[delegateIndex]?.magExtent ?? 0
            readonly property real animatedMagScale: root._magnificationContentScaleForWeight(magWeight, magContentFactor)
            readonly property real layoutExtra: root._magnificationLayoutExtraFor(root.baseMetrics.items[delegateIndex], magWeight, root.magnificationDynamicSpacing)

            // ── Presence transition ─────────────────────────────────────────
            // An item joining or leaving the dock grows and collapses its own
            // slot, so its neighbours slide over instead of jumping. The scale
            // is not decoration: it is the item's presence, and without it a
            // full-size icon would overlap the row while its slot shrinks.
            readonly property bool isExiting: delegateWrapper.itemData?.__exiting === true
            property real revealProgress: 1

            NumberAnimation {
                id: revealAnimation
                target: delegateWrapper
                property: "revealProgress"
                duration: root.itemTransitionDuration
                easing.type: Appearance.animation.elementMoveFast.type
                easing.bezierCurve: Appearance.animation.elementMoveFast.bezierCurve
            }

            function playReveal(to) {
                revealAnimation.stop();
                revealAnimation.from = delegateWrapper.revealProgress;
                revealAnimation.to = to;
                revealAnimation.start();
            }

            onIsExitingChanged: {
                delegateWrapper.playReveal(isExiting ? 0 : 1);
            }

            Component.onCompleted: {
                // Arm the width glide after the first layout: a new delegate
                // takes its size at once (its presence has its own reveal).
                Qt.callLater(() => delegateWrapper._extentAnimationReady = true);
                if (delegateWrapper.isExiting) {
                    delegateWrapper.revealProgress = 1;
                    delegateWrapper.playReveal(0);
                    return;
                }
                if (!root._itemTransitionsReady || root.dragging || root._reordering)
                    return;
                if (!root._isEnteringKey(delegateWrapper.itemData?.orderKey))
                    return;
                delegateWrapper.revealProgress = 0;
                delegateWrapper.playReveal(1);
            }

            // ── Width changes ───────────────────────────────────────────────
            // The item's own extent (a widget made wide or square, a stack or
            // live preview resized, a separator appearing) glides instead of
            // jumping; the Row slides the neighbours and the dock's surface
            // follows the laid-out width. Only the BASE extent animates — the
            // lens's `layoutExtra` stays on its own springs, so this is never
            // a second animation queue on the wrapper's width.
            property real animatedBaseBodyExtent: baseBodyMainExtent
            property bool _extentAnimationReady: false
            Behavior on animatedBaseBodyExtent {
                enabled: delegateWrapper._extentAnimationReady && root._itemTransitionsReady
                    && !root.dragging && !root._reordering && !Appearance.reducedMotion
                NumberAnimation {
                    duration: Appearance.animation.elementMove.duration
                    easing.type: Appearance.animation.elementMove.type
                    easing.bezierCurve: Appearance.animation.elementMove.bezierCurve
                }
            }
            // What the item's content may occupy right now: the animated
            // extent minus its separator room.
            readonly property real animatedContentExtent: animatedBaseBodyExtent - _separatorBeforeSpace - _separatorAfterSpace

            readonly property real bodyMainExtent: (animatedBaseBodyExtent + layoutExtra) * revealProgress
            readonly property real bodyMainStart: (root.isVertical ? y : x) + leadingIslandGap
            readonly property real bodyMainEnd: bodyMainStart + bodyMainExtent
            readonly property real _magnificationScale: animatedMagScale

            Behavior on animatedLeadingIslandGap {
                enabled: !root.dragging && !root.islandDragging
                animation: Appearance.animation.elementMoveFast.numberAnimation.createObject(this)
            }

            // Drag translation. Displaced items move by the dragged item's real
            // footprint read from the drag snapshot, so a 4-slot widget pushes
            // its neighbours exactly as far as it will actually occupy.
            readonly property bool isDragged: root.dragging && String(itemData?.orderKey ?? "") === root.dragSourceKey
            readonly property bool isSettling: !root.dragging
                && root._dropSettleKey !== ""
                && root._dropSettleKey === String(delegateWrapper.itemData?.orderKey ?? "")
            readonly property real dragTranslate: {
                if (!root.dragging)
                    return isSettling ? root._dropSettleOffset : 0;
                if (isDragged)
                    return root.dragCursorX - root.dragStartCursorX;
                return DockReorder.previewShift(
                    root._dragSlots,
                    root.dragSourceIndex,
                    root._dragTargetIndex,
                    delegateIndex,
                    root.effectiveIconSpacing
                );
            }
            readonly property real itemMagScale: animatedMagScale
            // Quantized: a distinct z per fraction of magnification re-orders
            // the render stack every frame the lens moves, for a difference
            // nobody can see between two neighbours at 1.03 and 1.05. Three
            // tiers are all the eye reads — rest, magnified, most magnified.
            z: (isDragged || isSettling) ? 100
                : (itemMagScale > 1.01 ? (itemMagScale > 1.25 ? 2 : 1) : 0)
            opacity: isDragged ? 0.85 : 1
            scale: isDragged ? 1.05 : 1

            Behavior on opacity {
                NumberAnimation {
                    duration: Appearance.animation.elementMoveFast.duration
                    easing.type: Appearance.animation.elementMoveFast.type
                    easing.bezierCurve: Appearance.animation.elementMoveFast.bezierCurve
                }
            }
            Behavior on scale {
                NumberAnimation {
                    duration: Appearance.animation.elementMoveFast.duration
                    easing.type: Appearance.animation.elementMoveFast.type
                    easing.bezierCurve: Appearance.animation.elementMoveFast.bezierCurve
                }
            }

            // Animate the final position, not a preview offset in a moving slot.
            // On commit Row/Column moves the slot while previewShift disappears;
            // a single target preserves the intermediate frame of every neighbour.
            DockItemPosition {
                id: itemPosition
                // The drag offset already includes the initial magnified displacement.
                layoutPosition: delegateWrapper.isDragged
                    // Slots are in root space; this position is the container's.
                    // A full width row starts far into the tray, and the dragged
                    // item jumped right by that much.
                    ? (root._dragSlots[delegateWrapper.delegateIndex]
                        ? root._dragSlots[delegateWrapper.delegateIndex].start - root._dragSlotOrigin
                        : delegateWrapper.bodyMainStart) - delegateWrapper.leadingIslandGap
                    : (root.isVertical ? delegateWrapper.y : delegateWrapper.x)
                offset: delegateWrapper.dragTranslate
                animate: root.reorderMotionActive
                tracking: delegateWrapper.isDragged
                settling: delegateWrapper.isSettling
            }
            transform: Translate {
                x: root.isVertical ? 0 : itemPosition.position - delegateWrapper.x
                y: root.isVertical ? itemPosition.position - delegateWrapper.y : 0
            }

            // ── Intelligent separators ──────────────────────────────────────
            readonly property bool _sepIsSpecial: root.isSpecialItem(delegateWrapper.itemData)
            readonly property bool _sepNextIsSpecial: {
                if (delegateIndex >= root.flattenedItems.length - 1)
                    return false;
                return root.isSpecialItem(root.flattenedItems[delegateIndex + 1]);
            }
            readonly property bool _sepDividersOn: root.effectiveShowDividers
            readonly property bool _sepShowBefore: {
                if (!_sepDividersOn || delegateIndex === 0)
                    return false;
                if (Config.options?.dock?.smartGrouping) {
                    return root.getItemCategory(root.flattenedItems[delegateIndex]) !== root.getItemCategory(root.flattenedItems[delegateIndex - 1]);
                }
                return _sepIsSpecial && delegateIndex > 0;
            }
            readonly property bool _sepShowAfter: {
                if (!_sepDividersOn || delegateIndex >= root.flattenedItems.length - 1)
                    return false;
                if (Config.options?.dock?.smartGrouping) {
                    return false; // we only draw before to avoid double lines
                }
                return _sepIsSpecial && !_sepNextIsSpecial;
            }
            // Reserve a real layout slot for each divider. This keeps the line
            // away from both the current icon and the preceding widget even
            // when iconSpacing is compact or negative.
            readonly property real _separatorSlot: Math.max(root.dotMargin, root.sepThickness * 2)
            readonly property real _separatorBeforeSpace: _sepShowBefore ? _separatorSlot : 0
            readonly property real _separatorAfterSpace: _sepShowAfter ? _separatorSlot : 0
            readonly property real _separatorLineMargin: (_separatorSlot - root.sepThickness) / 2

            width: root.isVertical ? itemHeight : leadingIslandGap + bodyMainExtent
            height: root.isVertical ? leadingIslandGap + bodyMainExtent : itemHeight

            // Horizontal mode: left vertical line
            Rectangle {
                visible: delegateWrapper._sepShowBefore && !root.isVertical
                anchors.left: parent.left
                anchors.leftMargin: delegateWrapper._separatorLineMargin
                anchors.verticalCenter: parent.verticalCenter
                width: root.sepThickness
                height: parent.height - root.dotMarginV * 2
                radius: Appearance.rounding.full
                color: Appearance.colors.colOutlineVariant
            }
            // Horizontal mode: right vertical line
            Rectangle {
                visible: delegateWrapper._sepShowAfter && !root.isVertical
                anchors.right: parent.right
                anchors.rightMargin: delegateWrapper._separatorLineMargin
                anchors.verticalCenter: parent.verticalCenter
                width: root.sepThickness
                height: parent.height - root.dotMarginV * 2
                radius: Appearance.rounding.full
                color: Appearance.colors.colOutlineVariant
            }
            // Vertical mode: top horizontal line
            Rectangle {
                visible: delegateWrapper._sepShowBefore && root.isVertical
                anchors.top: parent.top
                anchors.topMargin: delegateWrapper._separatorLineMargin
                anchors.horizontalCenter: parent.horizontalCenter
                width: parent.width - root.dotMargin * 2
                height: root.sepThickness
                radius: Appearance.rounding.full
                color: Appearance.colors.colOutlineVariant
            }
            // Vertical mode: bottom horizontal line
            Rectangle {
                visible: delegateWrapper._sepShowAfter && root.isVertical
                anchors.bottom: parent.bottom
                anchors.bottomMargin: delegateWrapper._separatorLineMargin
                anchors.horizontalCenter: parent.horizontalCenter
                width: parent.width - root.dotMargin * 2
                height: root.sepThickness
                radius: Appearance.rounding.full
                color: Appearance.colors.colOutlineVariant
            }

            Loader {
                id: contentLoader
                anchors.centerIn: parent
                anchors.horizontalCenterOffset: root.isVertical ? 0 : delegateWrapper.leadingIslandGap / 2 + (delegateWrapper._separatorBeforeSpace - delegateWrapper._separatorAfterSpace) / 2
                anchors.verticalCenterOffset: root.isVertical ? delegateWrapper.leadingIslandGap / 2 + (delegateWrapper._separatorBeforeSpace - delegateWrapper._separatorAfterSpace) / 2 : 0
                // The slot collapses to nothing; the content has to come with
                // it or it would spill over the neighbours on the way out.
                opacity: Math.max(0, Math.min(1, delegateWrapper.revealProgress))
                scale: 0.55 + 0.45 * Math.max(0, Math.min(1, delegateWrapper.revealProgress))
                visible: delegateWrapper.revealProgress > 0.01
                enabled: !delegateWrapper.isExiting

                // Expose delegate data so loaded components can access it via parent
                readonly property var _itemData: delegateWrapper.itemData
                readonly property real _animatedContentExtent: delegateWrapper.animatedContentExtent
                readonly property int _index: delegateWrapper.index
                readonly property real _magnificationScale: delegateWrapper._magnificationScale
                readonly property string _islandId: delegateWrapper.islandId

                sourceComponent: {
                    switch (itemData.type) {
                    case "action":
                        return actionItemComponent;
                    case "appGroup":
                        return appGroupItemComponent;
                    case "app":
                        return appItemComponent;
                    case "file":
                        return fileItemComponent;
                    case "media":
                        return mediaItemComponent;
                    case "weather":
                        return weatherItemComponent;
                    case "sports":
                        return sportsItemComponent;
                    case "livePreview":
                        return livePreviewItemComponent;
                    case "phone":
                        return phoneItemComponent;
                    case "tasks":
                        return tasksItemComponent;
                    case "utility":
                        return utilityItemComponent;
                    case "widgetStack":
                        return widgetStackItemComponent;
                    case "runningAppsGroup":
                        return runningAppsGroupComponent;
                    case "editAdd":
                        return editAddItemComponent;
                    default:
                        return null;
                    }
                }
            }

            Loader {
                anchors.fill: contentLoader
                z: 7
                active: root.editLayerActive && !delegateWrapper.isExiting
                    && root.dockEdit.editable(delegateWrapper.itemData)
                sourceComponent: DockEditItemLayer {
                    dockContent: root
                    wrapper: delegateWrapper
                    itemData: delegateWrapper.itemData
                    delegateIndex: delegateWrapper.delegateIndex
                }
            }

            Rectangle {
                readonly property bool _dropArmed: root.isGroupDropTarget(delegateWrapper.delegateIndex)
                readonly property bool _dropBuilding: root.isGroupDropCandidate(delegateWrapper.delegateIndex)
                visible: _dropArmed || _dropBuilding
                anchors.fill: parent
                z: 5
                radius: Appearance.rounding.normal
                color: root.groupDropIsBlocked(delegateWrapper.delegateIndex)
                    ? Appearance.colors.colErrorContainer
                    : Appearance.colors.colPrimaryContainer
                // Fills in as the hold builds, so the gesture reads as "keep
                // holding to group" rather than appearing out of nowhere.
                opacity: _dropArmed ? 0.28 : (_dropBuilding ? 0.28 * root._groupDropProgress : 0.0)
            }

            Rectangle {
                visible: root.isGroupDropTarget(delegateWrapper.delegateIndex)
                width: Math.max(Appearance.font.pixelSize.normal, root.buttonSlotSize * 0.3)
                height: width
                anchors.top: parent.top
                anchors.right: parent.right
                anchors.topMargin: -height * 0.18
                anchors.rightMargin: -width * 0.18
                z: 6
                radius: Appearance.rounding.full
                color: root.groupDropIsBlocked(delegateWrapper.delegateIndex)
                    ? Appearance.colors.colError
                    : Appearance.colors.colPrimary

                MaterialSymbol {
                    anchors.centerIn: parent
                    text: root.groupDropIsBlocked(delegateWrapper.delegateIndex)
                        ? "block"
                        : (root.groupDropWillCreate(delegateWrapper.delegateIndex) ? "create_new_folder" : "group_add")
                    iconSize: Appearance.font.pixelSize.small
                    color: root.groupDropIsBlocked(delegateWrapper.delegateIndex)
                        ? Appearance.colors.colOnError
                        : Appearance.colors.colOnPrimary
                }
            }
        }
    }

    // ── Item type components ───────────────────────────────────────────────

    Component {
        id: appGroupItemComponent
        Item {
            id: appGroupItemRoot
            width: root.buttonSlotSize
            height: root.isVertical ? root.buttonSlotSize : root.buttonSlotHeight
            readonly property var _itemData: parent._itemData
            readonly property int _index: parent._index

            DockAppGroupButton {
                anchors.centerIn: parent
                apps: appGroupItemRoot._itemData.apps ?? []
                dockContent: root
                groupId: appGroupItemRoot._itemData.groupId ?? ""
                delegateIndex: appGroupItemRoot._index
            }
        }
    }

    Component {
        id: actionItemComponent
        Item {
            id: actionItemRoot
            width: root.buttonSlotSize
            height: root.isVertical ? root.buttonSlotSize : root.buttonSlotHeight
            readonly property var _itemData: parent._itemData
            readonly property int _index: parent._index
            DockActionButton {
                anchors.centerIn: parent
                property int _delegateIndex: actionItemRoot._index
                actionId: actionItemRoot._itemData.actionId ?? ""
                symbolName: {
                    switch (actionItemRoot._itemData.actionId) {
                    case "pin":
                        return "keep";
                    case "trash":
                        return "delete";
                    case "overview": {
                        const symbol = String(Config.options?.dock?.overviewButton?.symbol ?? "").trim();
                        return symbol.length > 0 ? symbol : "apps";
                    }
                    default:
                        return "drag_indicator";
                    }
                }
                toggledSymbolName: actionItemRoot._itemData.actionId === "pin" ? "bookmark" : ""
                toggled: actionItemRoot._itemData.actionId === "pin" ? root.isPinned
                    : actionItemRoot._itemData.actionId === "overview" && GlobalStates.overviewSurfaceOpen
                normalShape: actionItemRoot._itemData.actionId === "overview" ? MaterialShape.Shape.SoftBurst : MaterialShape.Shape.Pill
                activeShape: actionItemRoot._itemData.actionId === "overview" ? MaterialShape.Shape.SoftBurst : MaterialShape.Shape.Cookie9Sided
                symbolSize: Math.round(Appearance.sizes.dockButtonSize * 0.5)
                dockContent: root
                delegateIndex: actionItemRoot._index
                onClicked: {
                    if (actionItemRoot._itemData.actionId === "pin")
                        root.togglePinRequested();
                    else if (actionItemRoot._itemData.actionId === "trash")
                        RyokuUi.Spawn.runApp(["xdg-open", "trash:///"], { label: "trash" });
                    else if (actionItemRoot._itemData.actionId === "overview")
                        GlobalStates.toggleOverview();
                }
                customImageSource: actionItemRoot._itemData.actionId === "trash" ? ("file://" + Directories.assetsPath + "/icons/" + (Appearance.m3colors.darkmode ? "macos-trash-dark.png" : "macos-trash.png")) : ""
                // The overview button's look (Settings → Dock → Overview button).
                readonly property var overviewCfg: Config.options?.dock?.overviewButton ?? null
                readonly property bool isOverview: actionItemRoot._itemData.actionId === "overview"
                customIconSource: isOverview && String(overviewCfg?.iconFile ?? "").length > 0
                    ? ("file://" + Directories.assetsPath + "/icons/" + overviewCfg.iconFile) : ""
                tintCustomIcon: overviewCfg?.iconTint ?? true
                shapeName: isOverview ? String(overviewCfg?.shape ?? "SoftBurst") : ""
                alwaysShowShape: isOverview && (overviewCfg?.alwaysShowShape ?? false)
                dragActive: false
                dragOver: false
                dragSymbol: ""
            }
        }
    }

    Component {
        id: appItemComponent
        Item {
            id: appItemRoot
            width: root.buttonSlotSize
            height: root.isVertical ? root.buttonSlotSize : root.buttonSlotHeight
            readonly property var _itemData: parent._itemData
            readonly property int _index: parent._index
            readonly property string _appId: _itemData?.appId ?? ""
            readonly property bool _groupReturnRequested: root.shouldAnimateGroupAppReturn(_appId)
            readonly property bool _groupExitRequested: root.isGroupAppExiting(_appId)
            property real _groupTransitionOpacity: 1.0
            property real _groupTransitionScale: 1.0

            function playGroupReturnAnimation() {
                _groupTransitionOpacity = 0.0;
                _groupTransitionScale = 0.72;
                Qt.callLater(function () {
                    if (!appItemRoot)
                        return;
                    appItemRoot._groupTransitionOpacity = 1.0;
                    appItemRoot._groupTransitionScale = 1.0;
                });
            }

            function playGroupExitAnimation() {
                _groupTransitionOpacity = 0.0;
                _groupTransitionScale = 0.72;
            }

            Component.onCompleted: {
                if (_groupReturnRequested)
                    playGroupReturnAnimation();
                else if (_groupExitRequested)
                    playGroupExitAnimation();
            }

            Connections {
                target: root
                function onGroupMutationRevisionChanged() {
                    if (appItemRoot._groupExitRequested)
                        appItemRoot.playGroupExitAnimation();
                }
                function onGroupTransitionRevisionChanged() {
                    if (appItemRoot._groupReturnRequested)
                        appItemRoot.playGroupReturnAnimation();
                }
            }

            opacity: _groupTransitionOpacity
            scale: _groupTransitionScale

            Behavior on opacity {
                NumberAnimation {
                    duration: Appearance.animation.elementMoveFast.duration
                    easing.type: Appearance.animation.elementMoveFast.type
                    easing.bezierCurve: Appearance.animation.elementMoveFast.bezierCurve
                }
            }

            Behavior on scale {
                NumberAnimation {
                    duration: Appearance.animation.elementMoveFast.duration
                    easing.type: Appearance.animation.elementMoveFast.type
                    easing.bezierCurve: Appearance.animation.elementMoveFast.bezierCurve
                }
            }

            layer.enabled: opacity > 0 && opacity < 1
            layer.effect: MultiEffect {
                blurEnabled: true
                blurMax: 8
                blur: 1 - appItemRoot.opacity
            }

            DockAppButton {
                anchors.centerIn: parent
                appToplevel: appItemRoot._itemData.appData
                dockContent: root
                delegateIndex: appItemRoot._index
            }
        }
    }

    Component {
        id: fileItemComponent
        Item {
            id: fileItemRoot
            width: root.buttonSlotSize
            height: root.isVertical ? root.buttonSlotSize : root.buttonSlotHeight
            readonly property var _itemData: parent._itemData
            readonly property int _index: parent._index
            DockFileButton {
                anchors.centerIn: parent
                filePath: fileItemRoot._itemData.path
                dockContent: root
                delegateIndex: fileItemRoot._index
            }
        }
    }

    Component {
        id: mediaItemComponent
        Item {
            id: mediaItemRoot
            width: root.isVertical ? root.buttonSlotSize : root.buttonSlotSize * 3
            height: root.isVertical ? root.buttonSlotSize : root.buttonSlotHeight
            readonly property int _index: parent._index
            Loader {
                anchors.fill: parent
                active: root.dockWidgetsActive
                sourceComponent: DockMediaWidget {
                    anchors.centerIn: parent
                    isVertical: root.isVertical
                    dockContent: root
                    delegateIndex: mediaItemRoot._index
                }
            }
        }
    }

    Component {
        id: weatherItemComponent
        Item {
            id: weatherItemRoot
            width: root.isVertical ? root.buttonSlotSize : root.buttonSlotSize * 3
            height: root.isVertical ? root.buttonSlotSize : root.buttonSlotHeight
            readonly property int _index: parent._index
            Loader {
                anchors.fill: parent
                active: root.dockWidgetsActive
                sourceComponent: DockWeatherWidget {
                    anchors.centerIn: parent
                    isVertical: root.isVertical
                    dockContent: root
                    delegateIndex: weatherItemRoot._index
                }
            }
        }
    }

    Component {
        id: sportsItemComponent
        Item {
            id: sportsItemRoot
            width: root.isVertical ? root.buttonSlotSize : root.buttonSlotSize * 3
            height: root.isVertical ? root.buttonSlotSize : root.buttonSlotHeight
            readonly property int _index: parent._index
            Loader {
                anchors.fill: parent
                active: root.dockWidgetsActive
                sourceComponent: DockSportsWidget {
                    anchors.centerIn: parent
                    isVertical: root.isVertical
                    dockContent: root
                    delegateIndex: sportsItemRoot._index
                }
            }
        }
    }

    Component {
        id: livePreviewItemComponent
        Item {
            id: livePreviewItemRoot
            width: root.isVertical ? root.buttonSlotSize : root.buttonSlotSize * root.livePreviewWidgetSlots
            height: root.isVertical ? root.buttonSlotSize : root.buttonSlotHeight
            readonly property int _index: parent._index
            Loader {
                anchors.fill: parent
                active: root.dockWidgetsActive
                sourceComponent: DockLivePreviewWidget {
                    anchors.centerIn: parent
                    isVertical: root.isVertical
                    dockContent: root
                    dockRevealed: root.dockRevealed
                    dockWindowVisible: root.dockWindowVisible
                    delegateIndex: livePreviewItemRoot._index
                }
            }
        }
    }

    Component {
        id: tasksItemComponent
        Item {
            id: tasksItemRoot
            width: root.isVertical ? root.buttonSlotSize : root.buttonSlotSize * 3
            height: root.isVertical ? root.buttonSlotSize : root.buttonSlotHeight
            readonly property int _index: parent._index
            DockTasksWidget {
                anchors.centerIn: parent
                isVertical: root.isVertical
                dockContent: root
                delegateIndex: tasksItemRoot._index
            }
        }
    }


    Component {
        id: utilityItemComponent
        Item {
            id: utilityItemRoot
            readonly property int _index: parent._index
            readonly property var _itemData: parent._itemData
            // Follows the slot while it glides between square and wide.
            width: root.isVertical ? root.buttonSlotSize : Math.max(root.buttonSlotSize, parent._animatedContentExtent)
            height: root.isVertical ? root.buttonSlotSize : root.buttonSlotHeight
            DockUtilityWidget {
                anchors.fill: parent
                isVertical: root.isVertical
                dockContent: root
                delegateIndex: utilityItemRoot._index
                kind: utilityItemRoot._itemData?.kind ?? ""
                wide: root._utilitySlots(utilityItemRoot._itemData) > 1
            }
        }
    }

    Component {
        id: widgetStackItemComponent
        Item {
            id: widgetStackItemRoot
            width: root.widgetStackExtent
            height: root.isVertical ? root.buttonSlotSize : root.buttonSlotHeight
            readonly property int _index: parent._index
            DockWidgetStack {
                anchors.fill: parent
                isVertical: root.isVertical
                dockContent: root
                delegateIndex: widgetStackItemRoot._index
                members: root.widgetStackMembers
                currentType: root.widgetStackCurrentType
                onCurrentTypeRequested: type => root.widgetStackCurrentType = type
            }
        }
    }

    Component {
        id: phoneItemComponent
        Item {
            id: phoneItemRoot
            width: root.buttonSlotSize
            height: root.isVertical ? root.buttonSlotSize : root.buttonSlotHeight
            readonly property int _index: parent._index
            DockPhoneWidget {
                anchors.centerIn: parent
                isVertical: root.isVertical
                dockContent: root
                delegateIndex: phoneItemRoot._index
            }
        }
    }

    Component {
        id: runningAppsGroupComponent
        Item {
            id: runningAppsRoot
            width: runningAppsRow.implicitWidth + root.dotMargin * 2
            height: root.isVertical ? root.buttonSlotSize : root.buttonSlotHeight
            readonly property var _itemData: parent._itemData
            readonly property int _index: parent._index
            Row {
                id: runningAppsRow
                spacing: Config.options.dock.iconSpacing
                anchors.centerIn: parent
                Repeater {
                    model: runningAppsRoot._itemData.apps ?? []
                    delegate: DockAppButton {
                        required property var modelData
                        appToplevel: modelData
                        dockContent: root
                        delegateIndex: -1
                    }
                }
            }
            MouseArea {
                anchors.fill: parent
                acceptedButtons: Qt.LeftButton
                preventStealing: true
                cursorShape: Qt.PointingHandCursor
                propagateComposedEvents: true
                property real pressCoord: 0
                property bool dragActive: false
                onPressed: event => {
                    pressCoord = root.isVertical ? event.y : event.x;
                    event.accepted = false;
                }
                onPositionChanged: event => {
                    if (!pressed)
                        return;
                    var cur = root.isVertical ? event.y : event.x;
                    var dist = Math.abs(cur - pressCoord);
                    if (!dragActive && dist > 5) {
                        dragActive = true;
                        root.startItemDrag(parent._index, this, event.x, event.y);
                    }
                    if (dragActive) {
                        root.moveItemDrag(this, event.x, event.y);
                        event.accepted = true;
                    } else {
                        event.accepted = false;
                    }
                }
                onReleased: event => {
                    if (dragActive) {
                        dragActive = false;
                        root.endItemDrag();
                        event.accepted = true;
                    } else {
                        event.accepted = false;
                    }
                }
                onCanceled: event => {
                    if (dragActive) {
                        dragActive = false;
                        root.cancelDrag();
                        event.accepted = true;
                    } else {
                        event.accepted = false;
                    }
                }
            }
        }
    }

    Component {
        id: editAddItemComponent
        Item {
            id: editAddRoot
            width: root.buttonSlotSize
            height: root.isVertical ? root.buttonSlotSize : root.buttonSlotHeight

            DockEditAddButton {
                anchors.centerIn: parent
                dockContent: root
                anchorTarget: editAddRoot
            }
        }
    }

    // ── Preview Popup ──────────────────────────────────────────────────────
    Loader {
        id: previewPopupLoader
        // Not while Edit Mode is on: a window preview popping up over the dock
        // you are rearranging is in the way, and there is nothing to switch to.
        active: (Config.options.dock.enablePreview ?? true) && !GlobalStates.editMode
        sourceComponent: DockPreviewPopup {
            dockRoot: root
            dockWindow: root.QsWindow.window
            appTopLevel: root.lastHoveredButton?.appToplevel
        }
    }

    // The dock's one tooltip: every item's DockTooltip asks this host, which
    // slides a single bubble between items inside one resident surface.
    readonly property var dockTooltipHost: tooltipHost
    DockTooltipHost {
        id: tooltipHost
        dockContent: root
    }

}
