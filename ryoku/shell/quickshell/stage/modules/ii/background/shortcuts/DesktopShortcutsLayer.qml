pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Widgets
import Ryoku.Ui.Singletons
import stage.modules.common
import stage.modules.common.widgets

Item {
    id: root

    required property string screenName
    property Item overlayParent: null
    property bool wallpaperLight: false
    // The grid is the store's (DesktopShortcuts.cellWidth/cellHeight): the
    // spacing preset scaled by the icon size, so a bigger icon takes a
    // bigger cell. Positions keep the 10px snap; every hit-test reads the
    // same two numbers.
    readonly property real cellWidth: DesktopShortcuts.cellWidth
    readonly property real cellHeight: DesktopShortcuts.cellHeight
    readonly property real iconScale: DesktopShortcuts.iconScale
    readonly property real iconSize: DesktopShortcuts.iconSize
    readonly property var options: Config.options.background.desktopIcons
    readonly property bool autoArrange: root.options.autoArrange ?? false
    // A clean desktop: the icons fade out and stop taking input; the store
    // is untouched. Coming back replays the entrance wave.
    readonly property bool iconsHidden: DesktopShortcuts.hidden
    opacity: root.iconsHidden ? 0 : 1
    visible: root.opacity > 0.01
    enabled: !root.iconsHidden
    readonly property bool pointerOverShortcut: {
        for (let i = 0; i < iconRepeater.count; ++i) {
            const tile = iconRepeater.itemAt(i);
            if (tile && tile.hovered)
                return true;
        }
        return false;
    }
    Behavior on opacity {
        NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease }
    }
    onIconsHiddenChanged: {
        if (root.iconsHidden)
            root.clearSelection();
        else {
            root.introWave = true;
            ++root.introEpoch;
        }
    }
    // Bumped to replay the entrance: every tile restarts its own delayed
    // rise, the delay growing with its distance from the top-left, so the
    // desktop fills in as one diagonal wave - the widgets' stagger, per icon.
    property int introEpoch: 0
    // The wave is for arrivals of the whole desktop (the layer's first
    // frame, an unhide); one icon added later rises without waiting its turn.
    property bool introWave: true
    onIntroEpochChanged: {
        root.introWave = true;
        introWaveTimer.restart();
    }
    Timer {
        id: introWaveTimer
        interval: 1000
        running: true
        onTriggered: root.introWave = false
    }
    readonly property var items: DesktopShortcuts.itemsFor(root.screenName)
    readonly property bool dialogOpen: contextDialog.active || groupPopup.active
    property string dropTargetId: ""
    property string contextId: ""
    property point contextPosition: Qt.point(0, 0)
    readonly property var contextEntry: root.items.find(item => item.id === root.contextId) ?? null
    onContextEntryChanged: {
        if (!contextEntry)
            closeContext();
    }
    // ── Multi-selection ────────────────────────────────────────────────────
    // Mirrors the widget canvas's system: the same marquee band (the canvas
    // owns the press on empty desktop and announces the settled rect — a
    // MouseArea of this layer's own would swallow the widgets' marquee),
    // Shift/Ctrl extends a band or toggles a clicked icon, a rigid group drag,
    // and Delete takes the lot off. Session state only, never persisted, and
    // pruned with the items themselves so a removed icon cannot haunt a halo.
    property var canvas: null
    property var selectedIds: []
    readonly property bool hasSelection: root.selectedIds.length > 0
    // Edit Mode shrinks this whole layer with the desktop, and the
    // background overview scales it too (camera-push holds 1.09 permanently
    // when the overview is always-on). The modal cards — context menu and
    // folder popup — undo the window's NET content scale (the
    // align bar's counter-scale pattern): they are laid out and rasterized
    // in SCREEN pixels, so their text, symbols and icons stay on their
    // native grid and the menu stays readable at any combined scale.
    // Cancelling only the edit shrink left the menu off-grid by the
    // overview's factor — the residual jaggies of the second screenshot.
    // Handed in by the host window; 1 when nothing is applied.
    property real surfaceScale: 1
    readonly property real counterScale: 1 / Math.max(0.2, Math.min(5, root.surfaceScale))
    // ── Click-to-open ──────────────────────────────────────────────────────
    // Single-click mode opens immediately. Double-click mode selects and arms
    // on the first click, then opens a second click on that icon. One timer for
    // the whole layer keeps the pair keyed to one id; Qt.styleHints is absent
    // in this runtime, so the window is the conventional 400 ms.
    property string armedId: ""
    readonly property bool iconsLocked: Config.options.background.desktopIconsLocked ?? false
    readonly property bool singleClickOpen: root.options.openOn === "single"
    Timer {
        id: clickTimer
        interval: 400
        onTriggered: root.armedId = ""
    }
    // Which page the next context menu opens on ("" = the action list).
    // F2 sets "rename"; the dialog is built with it, so no page motion
    // plays over what is already the menu's first frame.
    property string contextInitialPage: ""

    function isSelected(id) {
        return root.selectedIds.indexOf(id) !== -1;
    }
    function toggleSelected(id) {
        const next = root.selectedIds.slice();
        const at = next.indexOf(id);
        if (at === -1)
            next.push(id);
        else
            next.splice(at, 1);
        root.selectedIds = next;
    }
    function clearSelection() {
        if (root.selectedIds.length > 0)
            root.selectedIds = [];
    }
    function removeSelection() {
        if (root.selectedIds.length === 0)
            return;
        const ids = root.selectedIds;
        root.clearSelection();
        Qt.callLater(() => DesktopShortcuts.removeMany(root.screenName, ids));
    }

    // Rigid cluster travel: the selection stops where its tightest member
    // would leave the desktop, the widget canvas's own rule. The inputs are
    // settled (grid-snapped) positions, so every bound is a multiple of 10
    // and the clamped delta lands on the lattice without a second snap.
    function groupDelta(ids, dx, dy) {
        let minX = -Infinity, maxX = Infinity, minY = -Infinity, maxY = Infinity;
        for (let i = 0; i < iconModel.count; ++i) {
            const entry = iconModel.get(i).entry;
            if (ids.indexOf(entry.id) === -1)
                continue;
            const s = root.positionAt(entry.x, entry.y);
            minX = Math.max(minX, -s.x);
            maxX = Math.min(maxX, root.width - root.cellWidth - s.x);
            minY = Math.max(minY, -s.y);
            maxY = Math.min(maxY, root.height - root.cellHeight - s.y);
        }
        return Qt.point(Math.max(minX, Math.min(maxX, dx)), Math.max(minY, Math.min(maxY, dy)));
    }

    // Notify-backed so followers re-render from one property write per frame
    // instead of a fresh object per pointer event.
    QtObject {
        id: groupDrag
        property string leaderId: ""
        property var ids: []
        property real dx: 0
        property real dy: 0
        readonly property bool active: leaderId !== ""
    }

    Connections {
        target: root.canvas
        enabled: root.canvas !== null
        function onMarqueeFinished(band) {
            const picked = root.canvas.marqueeExtending
                ? root.selectedIds.slice() : [];
            for (let i = 0; i < iconModel.count; ++i) {
                const entry = iconModel.get(i).entry;
                const s = root.positionAt(entry.x, entry.y);
                if (s.x < band.x + band.width && s.x + root.cellWidth > band.x
                    && s.y < band.y + band.height && s.y + root.cellHeight > band.y
                    && picked.indexOf(entry.id) < 0)
                    picked.push(entry.id);
            }
            root.selectedIds = picked;
        }
    }

    // The host holds the keyboard while a selection exists and the pointer is
    // over a shortcut; moving away releases it before another surface is
    // clicked. When a marquee picked widgets too, the canvas owns Delete — the
    // context menu's Remove still takes the whole icon selection.
    focus: root.hasSelection
    onHasSelectionChanged: {
        if (root.hasSelection)
            root.forceActiveFocus();
    }
    Keys.onEscapePressed: event => {
        if (!root.hasSelection)
            return;
        event.accepted = true;
        root.clearSelection();
    }
    Keys.onPressed: event => {
        // Keys has no Backspace signal, so both delete keys ride the generic
        // one; auto-repeat dropped — a held key is one removal, not a queue.
        if ((event.key === Qt.Key_Delete || event.key === Qt.Key_Backspace)
            && !event.isAutoRepeat) {
            event.accepted = true;
            root.removeSelection();
        } else if (event.key === Qt.Key_F2 && !event.isAutoRepeat
            && root.selectedIds.length === 1) {
            // Rename the selection: the menu opens straight on its page,
            // at the tile's own centre.
            event.accepted = true;
            const entry = root.items.find(item => item.id === root.selectedIds[0]);
            if (entry) {
                const s = root.positionAt(entry.x, entry.y);
                root.openContext(entry.id, s.x + root.cellWidth / 2,
                    s.y + root.cellHeight / 2, "rename");
            }
        } else if (event.key === Qt.Key_A && (event.modifiers & Qt.ControlModifier)) {
            event.accepted = true;
            root.selectedIds = root.items.map(item => item.id);
        } else if (event.key === Qt.Key_Z && (event.modifiers & Qt.ControlModifier)) {
            event.accepted = true;
            if (!event.isAutoRepeat)
                DesktopShortcuts.undo();
        } else if (event.key === Qt.Key_Left || event.key === Qt.Key_Right
            || event.key === Qt.Key_Up || event.key === Qt.Key_Down) {
            // The arrows walk the desktop; Shift keeps what was picked.
            event.accepted = true;
            const dx = event.key === Qt.Key_Left ? -1 : event.key === Qt.Key_Right ? 1 : 0;
            const dy = event.key === Qt.Key_Up ? -1 : event.key === Qt.Key_Down ? 1 : 0;
            const next = root.neighbour(root.focusId, dx, dy);
            if (next === "")
                return;
            if (event.modifiers & Qt.ShiftModifier) {
                if (!root.isSelected(next))
                    root.selectedIds = root.selectedIds.concat([next]);
            } else {
                root.selectedIds = [next];
            }
            root.focusId = next;
        } else if ((event.key === Qt.Key_Return || event.key === Qt.Key_Enter) && !event.isAutoRepeat) {
            event.accepted = true;
            const entry = root.items.find(item => item.id === root.focusId);
            if (entry)
                root.openEntry(entry);
        } else if (event.key === Qt.Key_Menu) {
            event.accepted = true;
            const tile = root.tileFor(root.focusId);
            if (tile)
                root.openContext(root.focusId, tile.x + tile.width / 2, tile.y + tile.height / 2);
        } else if (event.text.length === 1 && event.text.trim() !== ""
            && !(event.modifiers & (Qt.ControlModifier | Qt.AltModifier | Qt.MetaModifier))) {
            event.accepted = true;
            root.typeAhead += event.text.toLocaleLowerCase();
            typeAheadTimer.restart();
            const found = root.findByPrefix(root.typeAhead);
            if (found !== "") {
                root.selectedIds = [found];
                root.focusId = found;
            }
        }
    }

    function openContext(itemId, x, y, initialPage = "") {
        root.closePopup();
        root.contextId = itemId;
        root.contextPosition = Qt.point(x, y);
        root.contextInitialPage = initialPage;
        contextDialog.active = root.contextEntry !== null;
    }

    function closeContext() {
        contextDialog.active = false;
        root.contextId = "";
        root.contextInitialPage = "";
    }

    // ── Group popup ────────────────────────────────────────────────────────
    // A folder's contents on left-click (DesktopShortcutGroupPopup); the
    // context menu keeps right-click. Mutually exclusive with the menu,
    // and closed by the layer itself when the group is gone.
    property string popupId: ""
    property rect popupRect: Qt.rect(0, 0, 0, 0)
    property bool popupShowPicker: false
    readonly property var popupEntry: root.items.find(item => item.id === root.popupId && item.type === "group") ?? null
    onPopupEntryChanged: {
        if (!popupEntry)
            closePopup();
    }
    function openPopup(itemId, x, y, w, h) {
        root.closeContext();
        root.popupId = itemId;
        root.popupRect = Qt.rect(x, y, w, h);
        root.popupShowPicker = false;
        groupPopup.active = root.popupEntry !== null;
    }
    function openFolder(folderId, showPicker = false) {
        const entry = root.items.find(item => item.id === folderId && item.type === "group");
        if (!entry)
            return false;
        root.closeContext();
        root.popupId = folderId;
        root.popupShowPicker = showPicker;
        const tile = root.tileFor(folderId);
        root.popupRect = tile
            ? Qt.rect(tile.x, tile.y, tile.width, tile.height)
            : Qt.rect(entry.x, entry.y, root.cellWidth, root.cellHeight);
        groupPopup.active = true;
        return true;
    }
    function closePopup() {
        groupPopup.active = false;
        root.popupId = "";
        root.popupShowPicker = false;
    }

    function positionAt(x, y) {
        return Qt.point(Math.max(0, Math.min(width - cellWidth, Math.round(x / 10) * 10)),
                        Math.max(0, Math.min(height - cellHeight, Math.round(y / 10) * 10)));
    }

    // The merge target under (x, y). With autoArrange only the middle of a
    // tile merges: its edges are where a drop pushes the icon aside instead.
    function targetAt(x, y, exceptId) {
        const insetX = root.autoArrange ? root.cellWidth * 0.22 : 0;
        const insetY = root.autoArrange ? root.cellHeight * 0.22 : 0;
        for (let i = 0; i < iconModel.count; ++i) {
            const item = iconModel.get(i).entry;
            const s = root.positionAt(item.x, item.y);
            if (item.id !== exceptId && DesktopShortcuts.isGroupable(item)
                && x >= s.x + insetX && x < s.x + root.cellWidth - insetX
                && y >= s.y + insetY && y < s.y + root.cellHeight - insetY)
                return item.id;
        }
        return "";
    }

    // ── Drag preview ───────────────────────────────────────────────────────
    // With autoArrange a single drag shows where it will land - a cell
    // outline - and the icon holding that cell steps into the one it will
    // be pushed to. Both come from DesktopShortcuts.planDrop, the same
    // function the release commits through.
    property var dragPlan: null
    property string dragId: ""
    onDragPlanChanged: {
        if (!root.dragPlan)
            return;
        dropGhost.snapping = dropGhost.opacity < 0.05;
        dropGhost.x = root.dragPlan.x;
        dropGhost.y = root.dragPlan.y;
        dropGhost.snapping = false;
    }

    // ── Keyboard ───────────────────────────────────────────────────────────
    // The icon the arrows walk from: the last one clicked or reached.
    property string focusId: ""
    onSelectedIdsChanged: {
        if (root.selectedIds.length === 0)
            root.focusId = "";
        else if (root.selectedIds.indexOf(root.focusId) === -1)
            root.focusId = root.selectedIds[root.selectedIds.length - 1];
    }
    // The nearest icon in a direction, weighting the sideways distance
    // double so the walk keeps to its row or column.
    function neighbour(fromId, dx, dy) {
        const from = root.items.find(item => item.id === fromId);
        if (!from)
            return root.items.length > 0 ? root.items[0].id : "";
        const a = root.positionAt(from.x, from.y);
        let best = "", bestScore = Infinity;
        for (const item of root.items) {
            if (item.id === fromId)
                continue;
            const b = root.positionAt(item.x, item.y);
            const along = dx !== 0 ? (b.x - a.x) * dx : (b.y - a.y) * dy;
            const across = dx !== 0 ? Math.abs(b.y - a.y) : Math.abs(b.x - a.x);
            if (along <= 0)
                continue;
            const score = along + across * 2;
            if (score < bestScore) {
                bestScore = score;
                best = item.id;
            }
        }
        return best;
    }
    function openEntry(entry) {
        const tile = root.tileFor(entry.id);
        if (entry.type === "group" && tile) {
            root.openPopup(entry.id, tile.x, tile.y, tile.width, tile.height);
        } else {
            root.clearSelection();
            DesktopShortcuts.launch(entry);
        }
    }
    function tileFor(id) {
        for (let i = 0; i < iconRepeater.count; ++i) {
            const tile = iconRepeater.itemAt(i);
            if (tile && tile.entry.id === id)
                return tile;
        }
        return null;
    }
    // The wallpaper's full-surface right-click catcher sits above this layer
    // outside Edit Mode. Let it route a hit back to the exact tile while still
    // retaining empty-space clicks for the desktop menu.
    function openContextAt(x, y) {
        if (root.iconsHidden)
            return false;
        for (let i = iconRepeater.count - 1; i >= 0; --i) {
            const tile = iconRepeater.itemAt(i);
            if (!tile || !tile.visible || tile.merging)
                continue;
            if (x < tile.x || y < tile.y
                || x >= tile.x + tile.width || y >= tile.y + tile.height)
                continue;
            root.focusId = tile.entry.id;
            if (!root.isSelected(tile.entry.id))
                root.clearSelection();
            root.openContext(tile.entry.id, x, y);
            return true;
        }
        return false;
    }
    // Type to find: letters typed within a beat of each other build one
    // prefix; a repeated single letter cycles through the icons it starts.
    property string typeAhead: ""
    Timer {
        id: typeAheadTimer
        interval: 900
        onTriggered: root.typeAhead = ""
    }
    function findByPrefix(text) {
        const label = item => String(item.name || item.id).toLocaleLowerCase();
        const ordered = root.items.slice().sort((a, b) => (a.y - b.y) || (a.x - b.x));
        const matches = ordered.filter(item => label(item).startsWith(text));
        if (matches.length === 0)
            return "";
        const repeat = text.length > 1 && text.split("").every(c => c === text[0]);
        if (repeat) {
            const single = ordered.filter(item => label(item).startsWith(text[0]));
            const at = single.findIndex(item => item.id === root.focusId);
            return single[(at + 1) % single.length].id;
        }
        return matches[0].id;
    }

    // ── Quick size ─────────────────────────────────────────────────────────
    // Ctrl + wheel anywhere on the desktop steps the icon size, one notch
    // per step; a touchpad's small deltas are summed into notches.
    property real wheelAccum: 0
    WheelHandler {
        acceptedModifiers: Qt.ControlModifier
        acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
        onWheel: event => {
            root.wheelAccum += event.angleDelta.y;
            while (Math.abs(root.wheelAccum) >= 120) {
                DesktopShortcuts.stepIconScale(root.wheelAccum > 0 ? 1 : -1);
                root.wheelAccum -= root.wheelAccum > 0 ? 120 : -120;
            }
        }
    }

    // Preserve delegates on rename/move/member edits; never rebuild the whole desktop.
    function sync() {
        const wanted = new Set(root.items.map(item => item.id));
        if (root.selectedIds.some(id => !wanted.has(id)))
            root.selectedIds = root.selectedIds.filter(id => wanted.has(id));
        for (let i = iconModel.count - 1; i >= 0; --i) {
            if (!wanted.has(iconModel.get(i).entry.id))
                iconModel.remove(i);
        }
        for (let i = 0; i < root.items.length; ++i) {
            const entry = root.items[i];
            let found = -1;
            for (let j = i; j < iconModel.count; ++j) {
                if (iconModel.get(j).entry.id === entry.id) {
                    found = j;
                    break;
                }
            }
            const encoded = JSON.stringify(entry);
            if (found < 0)
                iconModel.insert(i, { entry: entry, encoded: encoded, mergePulse: 0 });
            else {
                if (found !== i)
                    iconModel.move(found, i, 1);
                if (iconModel.get(i).encoded !== encoded) {
                    const prev = iconModel.get(i).entry;
                    iconModel.setProperty(i, "entry", entry);
                    iconModel.setProperty(i, "encoded", encoded);
                    // The swallow landed here: the group grew. One pulse
                    // drives the target's pop — a role write, not a signal
                    // hunt, so the delegate animates exactly once per merge.
                    if (entry.type === "group"
                        && entry.apps.length > (prev.type === "group" ? prev.apps.length : 1))
                        iconModel.setProperty(i, "mergePulse", iconModel.get(i).mergePulse + 1);
                }
            }
        }
    }
    onItemsChanged: sync()
    Component.onCompleted: sync()
    ListModel { id: iconModel; dynamicRoles: true }

    onVisibleChanged: {
        if (!visible) {
            closeContext();
            closePopup();
            root.clearSelection();
        }
    }

    // The landing spot of a single autoArrange drag.
    Rectangle {
        id: dropGhost
        readonly property bool shown: root.dragPlan !== null
        // Placed by onDragPlanChanged: it keeps its last cell while fading
        // out, and jumps (rather than slides) to the first one of a drag.
        property bool snapping: false
        width: root.cellWidth
        height: root.cellHeight
        radius: Tokens.radius
        color: Tokens.tint10
        border.width: Tokens.border
        border.color: Tokens.lineStrong
        opacity: dropGhost.shown ? 1 : 0
        visible: opacity > 0.001
        scale: dropGhost.shown ? 1 : 0.9
        Behavior on x {
            enabled: !dropGhost.snapping && !Tokens.reduceMotion
            NumberAnimation { duration: Tokens.move; easing.type: Tokens.ease }
        }
        Behavior on y {
            enabled: !dropGhost.snapping && !Tokens.reduceMotion
            NumberAnimation { duration: Tokens.move; easing.type: Tokens.ease }
        }
        Behavior on opacity {
            NumberAnimation { duration: Tokens.snap; easing.type: Tokens.ease }
        }
        Behavior on scale {
            NumberAnimation { duration: Tokens.snap; easing.type: Tokens.ease }
        }
    }

    Repeater {
        id: iconRepeater
        model: iconModel
        delegate: Item {
            id: tile
            required property var entry
            required property int mergePulse
            readonly property point settled: root.positionAt(entry.x, entry.y)
            property bool dragging: false
            property bool suppressClick: false
            property point pressPoint: Qt.point(0, 0)
            property point origin: Qt.point(0, 0)
            property point pending: Qt.point(0, 0)
            // The swallow: while the absorbed tile animates into its target
            // the store write is still pending — merging gates the gesture
            // and mergeData carries the commit.
            property bool merging: false
            property var mergeData: null
            readonly property bool selected: root.isSelected(entry.id)
            readonly property bool groupMember: groupDrag.active && !tile.dragging
                && entry.id !== groupDrag.leaderId && groupDrag.ids.indexOf(entry.id) !== -1
            readonly property bool hovered: tileHover.hovered

            // ── Motion ─────────────────────────────────────────────────────
            // The tile is drawn at its stored cell plus an offset that glides
            // to rest. Whenever the cell changes - a sort, an align, a drop,
            // a reflow - the offset is re-based so the tile starts from where
            // it was on screen and travels to the new cell instead of
            // teleporting. The rest is 0, or the bump preview's step aside.
            property real offX: 0
            property real offY: 0
            property point prevSettled: Qt.point(0, 0)
            property bool placed: false
            readonly property bool bumped: root.dragPlan !== null && root.dragPlan.bumpId === tile.entry.id
            readonly property real restX: tile.bumped ? root.dragPlan.bumpX - tile.settled.x : 0
            readonly property real restY: tile.bumped ? root.dragPlan.bumpY - tile.settled.y : 0
            function glide() {
                glideMotion.stop();
                if (Tokens.reduceMotion || (tile.offX === tile.restX && tile.offY === tile.restY)) {
                    tile.offX = tile.restX;
                    tile.offY = tile.restY;
                    return;
                }
                glideX.to = tile.restX;
                glideY.to = tile.restY;
                glideMotion.start();
            }
            // Hold the tile where it is now (screen position), then glide.
            function holdAt(screenX, screenY) {
                glideMotion.stop();
                tile.offX = screenX - tile.settled.x;
                tile.offY = screenY - tile.settled.y;
                tile.glide();
            }
            onSettledChanged: {
                if (!tile.placed)
                    return;
                const fromX = tile.prevSettled.x + tile.offX;
                const fromY = tile.prevSettled.y + tile.offY;
                tile.prevSettled = tile.settled;
                tile.holdAt(fromX, fromY);
            }
            onRestXChanged: tile.glide()
            onRestYChanged: tile.glide()
            ParallelAnimation {
                id: glideMotion
                NumberAnimation {
                    id: glideX
                    target: tile; property: "offX"
                    duration: Tokens.move
                    easing.type: Tokens.ease
                }
                NumberAnimation {
                    id: glideY
                    target: tile; property: "offY"
                    duration: Tokens.move
                    easing.type: Tokens.ease
                }
            }

            x: settled.x + offX
            y: settled.y + offY
            width: root.cellWidth
            height: root.cellHeight
            z: dragging || merging ? 2 : (glideMotion.running ? 1 : 0)

            // ── Entrance ───────────────────────────────────────────────────
            // A rise and fade, delayed by the tile's distance from the
            // top-left in cells: the whole desktop arrives as one wave.
            property real intro: Tokens.reduceMotion ? 1 : 0
            function replayIntro() {
                if (Tokens.reduceMotion) {
                    tile.intro = 1;
                    return;
                }
                introMotion.stop();
                tile.intro = 0;
                introDelay.duration = !root.introWave ? 0 : Math.min(700, 30 * (tile.settled.x / root.cellWidth + tile.settled.y / root.cellHeight));
                introMotion.start();
            }
            Component.onCompleted: {
                tile.prevSettled = tile.settled;
                tile.placed = true;
                tile.replayIntro();
            }
            Connections {
                target: root
                function onIntroEpochChanged() {
                    tile.replayIntro();
                }
            }
            SequentialAnimation {
                id: introMotion
                PauseAnimation { id: introDelay; duration: 0 }
                NumberAnimation {
                    target: tile; property: "intro"; to: 1
                    duration: Tokens.move
                    easing.type: Tokens.ease
                }
            }

            // The merge physics, one-shot: the leader pops as it swallows
            // (scale 1.1→1 on the eased tail), the absorbed tile shrinks and
            // fades into it (mergeMotion). Both animate tileContent's scale
            // imperatively — the press-dip binding is shadowed while they
            // run and resumes the instant they stop, so hover/press and the
            // merge cannot fight over the property.
            onMergePulseChanged: {
                if (mergePulse > 0)
                    targetPop.restart();
            }
            SequentialAnimation {
                id: targetPop
                NumberAnimation {
                    target: tileContent; property: "scale"; to: 1.1
                    duration: Tokens.snap; easing.type: Tokens.ease
                }
                NumberAnimation {
                    target: tileContent; property: "scale"; to: 1.0
                    duration: Tokens.move; easing.type: Tokens.ease
                }
            }
            ParallelAnimation {
                id: mergeMotion
                NumberAnimation {
                    target: tileContent; property: "opacity"; to: 0
                    duration: Tokens.move
                    easing.type: Tokens.ease
                }
                NumberAnimation {
                    target: tileContent; property: "scale"; to: 0.3
                    duration: Tokens.move
                    easing.type: Tokens.ease
                }
                onFinished: {
                    const m = tile.mergeData;
                    tile.merging = false;
                    tile.mergeData = null;
                    if (m)
                        DesktopShortcuts.move(root.screenName, m.id, m.x, m.y, m.target);
                }
            }

            HoverHandler {
                id: tileHover
            }

            Item {
                id: tileContent
                anchors.fill: parent
                opacity: tile.intro
                // Keep press feedback noticeable without moving a tile enough
                // to look like the start of a drag.
                scale: (gesture.pressedButtons & Qt.LeftButton) !== 0
                    && !tile.dragging ? 0.9 : 1.0
                Behavior on scale {
                    NumberAnimation {
                        duration: Tokens.move
                        easing.type: Tokens.ease
                    }
                }
                transform: [
                    Translate {
                        x: tile.dragging ? tile.pending.x - tile.x : (tile.groupMember ? groupDrag.dx : 0)
                        y: tile.dragging ? tile.pending.y - tile.y : (tile.groupMember ? groupDrag.dy : 0)
                    },
                    // The entrance's rise.
                    Translate {
                        y: (1 - tile.intro) * 14
                    }
                ]
                // Hover and merge-target plates.
                Rectangle {
                    anchors.fill: parent
                    radius: Tokens.radius
                    color: root.dropTargetId === tile.entry.id ? Tokens.tint16 : Tokens.tint5
                    opacity: root.dropTargetId === tile.entry.id || (tile.hovered && !tile.selected && !tile.dragging) ? 1 : 0
                    visible: opacity > 0.001
                    Behavior on opacity {
                        NumberAnimation { duration: Tokens.snap; easing.type: Tokens.ease }
                    }
                }
                ColumnLayout {
                    anchors.fill: parent
                    anchors.margins: 4
                    spacing: 2
                    Item {
                        id: plate
                        Layout.alignment: Qt.AlignHCenter
                        Layout.preferredWidth: root.iconSize
                        Layout.preferredHeight: root.iconSize
                        readonly property string style: root.options.iconBackground ?? "none"
                        readonly property bool shaped: plate.style === "circle" || plate.style === "squircle"
                        // The icon backdrop: a translucent card, or a
                        // palette-tinted circle or squircle the glyph sits in.
                        Rectangle {
                            anchors.fill: parent
                            visible: plate.style !== "none" && tile.entry.type !== "group"
                            radius: plate.style === "circle" ? width / 2 : Tokens.radius
                            color: plate.shaped ? Tokens.bone : Tokens.paperLift
                            border.width: Tokens.border
                            border.color: plate.shaped ? Tokens.bone : Tokens.line
                        }
                        Loader {
                            anchors.centerIn: parent
                            readonly property real glyph: tile.entry.type === "group" ? root.iconSize
                                : plate.shaped ? root.iconSize * 0.64
                                : plate.style === "translucent" ? root.iconSize * 0.78 : root.iconSize
                            width: glyph
                            height: glyph
                            sourceComponent: tile.entry.type === "group" ? groupIcon : singleIcon
                        }
                        // Running: the dock's own dot, under the icon.
                        Rectangle {
                            anchors.horizontalCenter: parent.horizontalCenter
                            anchors.top: parent.bottom
                            anchors.topMargin: -Tokens.border * 2
                            readonly property bool shown: (root.options.runningBadges ?? true) && DesktopShortcuts.isRunning(tile.entry)
                            width: shown ? Tokens.s3 * Math.max(0.75, root.iconScale) : Tokens.s1
                            height: Tokens.s1
                            radius: height / 2
                            color: Tokens.ink
                            opacity: shown ? 1 : 0
                            visible: opacity > 0.001
                            Behavior on opacity {
                                NumberAnimation { duration: Tokens.snap; easing.type: Tokens.ease }
                            }
                            Behavior on width {
                                NumberAnimation { duration: Tokens.move; easing.type: Tokens.ease }
                            }
                        }
                        // Unread notifications from the app.
                        Rectangle {
                            id: unreadBadge
                            readonly property int count: (root.options.notificationBadges ?? true) ? DesktopShortcuts.unreadCount(tile.entry) : 0
                            anchors.right: parent.right
                            anchors.top: parent.top
                            anchors.rightMargin: -4
                            anchors.topMargin: -4
                            height: Tokens.s4 + Tokens.s1
                            width: Math.max(height, unreadText.implicitWidth + Tokens.s3)
                            radius: height / 2
                            color: Tokens.alert
                            scale: unreadBadge.count > 0 ? 1 : 0
                            visible: scale > 0.01
                            Behavior on scale {
                                NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease }
                            }
                            Text {
                                id: unreadText
                                anchors.centerIn: parent
                                text: unreadBadge.count > 99 ? "99+" : String(unreadBadge.count)
                                font.family: Tokens.ui
                                font.pixelSize: Tokens.fTiny
                                font.weight: Font.DemiBold
                                color: Tokens.keycapOnDark
                            }
                        }
                    }
                    Item {
                        id: labelBox
                        Layout.fillWidth: true
                        Layout.fillHeight: true
                        readonly property string mode: root.options.labels ?? "always"
                        readonly property string style: root.options.labelStyle ?? "auto"
                        readonly property bool pill: labelBox.style === "pill"
                            || (labelBox.style === "auto" && root.wallpaperLight)
                        visible: labelBox.mode !== "never"
                        opacity: labelBox.mode === "hover" ? (tile.hovered || tile.selected ? 1 : 0) : 1
                        Behavior on opacity {
                            NumberAnimation { duration: Tokens.snap; easing.type: Tokens.ease }
                        }
                        Rectangle {
                            anchors.horizontalCenter: parent.horizontalCenter
                            anchors.top: parent.top
                            width: Math.min(parent.width, tileLabel.contentWidth + Tokens.s3)
                            height: tileLabel.contentHeight + Tokens.s1
                            radius: Tokens.radius
                            color: Tokens.bone
                            border.width: Tokens.border
                            border.color: Tokens.bone
                            visible: labelBox.pill
                        }
                        Text {
                            id: tileLabel
                            anchors.left: parent.left
                            anchors.right: parent.right
                            anchors.top: parent.top
                            anchors.topMargin: Tokens.border * 2
                            text: tile.entry.name || tile.entry.id
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall
                            color: labelBox.pill ? Tokens.inkOnBone : Tokens.ink
                            elide: Text.ElideRight
                            wrapMode: (root.options.labelLines ?? 1) === 2 ? Text.Wrap : Text.NoWrap
                            horizontalAlignment: Text.AlignHCenter
                            maximumLineCount: (root.options.labelLines ?? 1) === 2 ? 2 : 1
                            style: labelBox.pill ? Text.Normal : Text.Raised
                            styleColor: Tokens.paper
                        }
                    }
                }
                // Selection halo: the widget canvas's own, scaled to a tile —
                // same colour, fill and border, so one language says "picked"
                // whether it is a clock or a shortcut.
                Rectangle {
                    visible: opacity > 0.001
                    opacity: tile.selected ? 1 : 0
                    anchors.fill: parent
                    anchors.margins: -Tokens.s2
                    radius: Tokens.radius
                    color: Tokens.tint16
                    border.color: Tokens.bone
                    border.width: root.focusId === tile.entry.id && root.selectedIds.length > 1
                        ? Tokens.border * 3 : Tokens.border * 2
                    Behavior on opacity {
                        NumberAnimation { duration: Tokens.snap; easing.type: Tokens.ease }
                    }
                }
                Component {
                    id: singleIcon
                    IconImage {
                        implicitSize: root.iconSize
                        // The tiles ARE minified in Edit Mode (the mode's
                        // shrink has no counter for them — they are the
                        // desktop). Mipmapping keeps their edges clean.
                        mipmap: true
                        source: Quickshell.iconPath(tile.entry.icon || (tile.entry.type === "directory"
                            ? "folder" : tile.entry.type === "url" ? "internet-web-browser" : "text-x-generic"), "image-missing")
                    }
                }
                Component {
                    id: groupIcon
                    Rectangle {
                        id: groupPlate
                        readonly property string style: root.options.iconBackground ?? "none"
                        radius: style === "circle" ? width / 2 : Tokens.radius
                        color: style === "circle" || style === "squircle" ? Tokens.bone : Tokens.paperLift
                        border.width: Tokens.border
                        border.color: style === "circle" || style === "squircle" ? Tokens.bone : Tokens.line
                        Grid {
                            anchors.centerIn: parent
                            columns: 2
                            spacing: 4 * root.iconScale
                            Repeater {
                                model: tile.entry.apps.slice(0, 4)
                                delegate: IconImage {
                                    required property var modelData
                                    implicitSize: (groupPlate.style === "circle" ? 18 : 22) * root.iconScale
                                    mipmap: true
                                    source: Quickshell.iconPath(modelData.icon, "image-missing")
                                }
                            }
                        }
                        MaterialSymbol {
                            anchors.centerIn: parent
                            visible: (tile.entry.apps ?? []).length === 0
                            text: "folder"
                            iconSize: root.iconSize * 0.62
                            color: Tokens.inkDim
                        }
                    }
                }
            }
            MouseArea {
                id: gesture
                anchors.fill: parent
                acceptedButtons: Qt.LeftButton | Qt.RightButton
                preventStealing: true
                cursorShape: Qt.PointingHandCursor
                enabled: !tile.merging
                onPressed: mouse => {
                    tile.suppressClick = false;
                    tile.pressPoint = root.mapFromItem(gesture, mouse.x, mouse.y);
                    tile.origin = Qt.point(tile.settled.x, tile.settled.y);
                }
                onPositionChanged: mouse => {
                    if (!(pressedButtons & Qt.LeftButton))
                        return;
                    // The lock gates only the drag START — press feedback,
                    // selection and the menu keep working.
                    if (!tile.dragging && root.iconsLocked)
                        return;
                    const p = root.mapFromItem(gesture, mouse.x, mouse.y);
                    const dx = p.x - tile.pressPoint.x;
                    const dy = p.y - tile.pressPoint.y;
                    if (!tile.dragging && dx * dx + dy * dy < 100)
                        return;
                    if (!tile.dragging) {
                        tile.dragging = true;
                        root.dragId = tile.entry.id;
                        if (root.selectedIds.length > 1 && root.isSelected(tile.entry.id)) {
                            // Grabbing a selected tile drags the whole set.
                            groupDrag.leaderId = tile.entry.id;
                            groupDrag.ids = root.selectedIds;
                            groupDrag.dx = 0;
                            groupDrag.dy = 0;
                        } else if (root.hasSelection) {
                            // Grabbing outside the selection is a click-away
                            // (the widget canvas's rule): single drag.
                            root.clearSelection();
                        }
                    }
                    tile.suppressClick = true;
                    tile.pending = root.positionAt(tile.origin.x + dx, tile.origin.y + dy);
                    if (groupDrag.leaderId === tile.entry.id) {
                        // Re-clamp the leader's own snapped position to what
                        // the cluster's tightest member allows, so the whole
                        // selection stops at the first wall.
                        const travel = root.groupDelta(groupDrag.ids,
                            tile.pending.x - tile.origin.x, tile.pending.y - tile.origin.y);
                        tile.pending = Qt.point(tile.origin.x + travel.x, tile.origin.y + travel.y);
                        groupDrag.dx = travel.x;
                        groupDrag.dy = travel.y;
                    } else {
                        root.dropTargetId = DesktopShortcuts.isGroupable(tile.entry) ? root.targetAt(p.x, p.y, tile.entry.id) : "";
                        root.dragPlan = root.autoArrange && root.dropTargetId === ""
                            ? DesktopShortcuts.planDrop(root.screenName, tile.entry.id, tile.pending.x, tile.pending.y)
                            : null;
                    }
                }
                onReleased: {
                    if (!tile.dragging)
                        return;
                    const itemId = tile.entry.id;
                    const targetId = root.dropTargetId;
                    const p = tile.pending;
                    tile.dragging = false;
                    root.dragId = "";
                    root.dropTargetId = "";
                    root.dragPlan = null;
                    if (groupDrag.leaderId === itemId) {
                        // One write for the cluster; merging is a single-drag
                        // gesture, so the group just travels. Every member is
                        // held where it was dropped and glides from there.
                        const ids = groupDrag.ids;
                        const ddx = groupDrag.dx;
                        const ddy = groupDrag.dy;
                        for (const id of ids) {
                            const member = root.tileFor(id);
                            if (member)
                                member.holdAt(member.settled.x + ddx, member.settled.y + ddy);
                        }
                        groupDrag.leaderId = "";
                        groupDrag.ids = [];
                        groupDrag.dx = 0;
                        groupDrag.dy = 0;
                        if (ddx !== 0 || ddy !== 0) {
                            const moves = [];
                            for (const entry of root.items) {
                                if (ids.indexOf(entry.id) === -1)
                                    continue;
                                const s = root.positionAt(entry.x, entry.y);
                                moves.push({ id: entry.id, x: s.x + ddx, y: s.y + ddy });
                            }
                            Qt.callLater(() => DesktopShortcuts.moveMany(root.screenName, moves));
                        }
                        return;
                    }
                    if (targetId !== "" && DesktopShortcuts.isGroupable(tile.entry)) {
                        // Play the swallow first, commit on its last frame:
                        // the store write destroying an invisible delegate
                        // is what made the old merge read as a teleport.
                        tile.holdAt(p.x, p.y);
                        tile.merging = true;
                        tile.mergeData = { id: itemId, x: p.x, y: p.y, target: targetId };
                        mergeMotion.start();
                        return;
                    }
                    // Held at the drop point; the store write moves the cell
                    // and the tile glides into it.
                    tile.holdAt(p.x, p.y);
                    Qt.callLater(() => DesktopShortcuts.move(root.screenName, itemId, p.x, p.y, targetId));
                }
                onCanceled: {
                    if (groupDrag.leaderId === tile.entry.id) {
                        groupDrag.leaderId = "";
                        groupDrag.ids = [];
                        groupDrag.dx = 0;
                        groupDrag.dy = 0;
                    }
                    tile.dragging = false;
                    tile.suppressClick = true;
                    root.dragId = "";
                    root.dropTargetId = "";
                    root.dragPlan = null;
                }
                onClicked: mouse => {
                    if (tile.suppressClick)
                        return;
                    root.focusId = tile.entry.id;
                    if (mouse.button === Qt.LeftButton
                        && (mouse.modifiers & (Qt.ShiftModifier | Qt.ControlModifier))) {
                        root.toggleSelected(tile.entry.id);
                        return;
                    }
                    if (mouse.button === Qt.RightButton) {
                        // Right-clicking outside the selection retargets it,
                        // like everywhere else: the menu then speaks of the
                        // clicked icon alone.
                        if (!root.isSelected(tile.entry.id))
                            root.clearSelection();
                        const p = root.mapFromItem(gesture, mouse.x, mouse.y);
                        root.openContext(tile.entry.id, p.x, p.y);
                    } else if (root.singleClickOpen) {
                        root.selectedIds = [tile.entry.id];
                        root.armedId = "";
                        clickTimer.stop();
                        root.openEntry(tile.entry);
                    } else if (root.armedId !== tile.entry.id) {
                        root.selectedIds = [tile.entry.id];
                        root.armedId = tile.entry.id;
                        clickTimer.restart();
                    } else {
                        root.armedId = "";
                        clickTimer.stop();
                        root.openEntry(tile.entry);
                    }
                }
            }
        }
    }

    // The two modal cards counter-scale the mode's shrink (see
    // `counterScale`): the dialog undoes it inside its own cards, the
    // popup on the popup itself.
    Loader {
        id: contextDialog
        parent: root.overlayParent ?? root
        anchors.fill: parent
        z: 3
        active: false
        sourceComponent: DesktopShortcutContextDialog {
            entry: root.contextEntry ?? ({})
            anchorPoint: root.contextPosition
            screenName: root.screenName
            selectionCount: root.isSelected(root.contextId) ? root.selectedIds.length : 1
            selectedIds: root.isSelected(root.contextId) ? root.selectedIds : [root.contextId]
            counterScale: root.counterScale
            page: root.contextInitialPage
            onSelectAllRequested: root.selectedIds = root.items.map(item => item.id)
            onCloseRequested: {
                const remove = pendingAction === "remove";
                const itemId = root.contextId;
                const screenName = root.screenName;
                const bulk = remove && root.isSelected(itemId) && root.selectedIds.length > 1;
                const ids = root.selectedIds.slice();
                root.closeContext();
                if (bulk) {
                    root.clearSelection();
                    Qt.callLater(() => DesktopShortcuts.removeMany(screenName, ids));
                } else if (remove)
                    Qt.callLater(() => DesktopShortcuts.remove(screenName, itemId));
            }
        }
    }

    Loader {
        id: groupPopup
        parent: root.overlayParent ?? root
        anchors.fill: parent
        z: 4
        active: false
        sourceComponent: DesktopShortcutGroupPopup {
            entry: root.popupEntry ?? ({ apps: [] })
            tileRect: root.popupRect
            screenName: root.screenName
            showPicker: root.popupShowPicker
            counterScale: root.counterScale
            onCloseRequested: root.closePopup()
            onRenameRequested: name => DesktopShortcuts.rename(root.screenName, root.popupId, name)
            onUngroupRequested: DesktopShortcuts.ungroup(root.screenName, root.popupId)
            onTakeOutRequested: appId => DesktopShortcuts.takeOut(
                root.screenName, root.popupId, appId)
            onMoveRequested: (appId, folderId) => DesktopShortcuts.moveToFolder(
                root.screenName, root.popupId, appId, folderId)
            onRemoveRequested: appId => DesktopShortcuts.removeApp(root.screenName, appId)
            onDisableStacksRequested: DesktopShortcuts.setStacks(false)
        }
    }
}
