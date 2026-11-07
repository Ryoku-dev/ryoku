pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import Ryoku.Bus
import "Singletons"
import Ryoku.Ui.Singletons

/**
 * The overview body for one monitor. Two levels, macOS-style:
 *
 *   ┌──────────────────────────────────────────────────────────┐
 *   │       [ DESKTOP 01 ] [ 02 ] [ + ]        the desktop strip  │
 *   ├──────────────────────────────────────────────────────────┤
 *   │  ┌─────┐ ┃ ┌─────┐ ┌─────┐ ┃  the viewed desktop's spaces  │
 *   │  │ 01  │ ┃2┃ 03  │ │ 04  │ +┃  as a FILMSTRIP: windowed     │
 *   │  └─────┘ ┃ └─────┘ └─────┘ ┃  spaces are full preview cells,│
 *   │          empty ones are thin numbered slats (the gaps stay  │
 *   │          visible + reachable without eating a full cell).   │
 *   └──────────────────────────────────────────────────────────┘
 *
 * The window manager has no native "desktop" and may create a workspace only on
 * first visit, so gaps (e.g. ws3 when you hold 1,2,4) can be absent. The
 * filmstrip renders every SLOT 1..N in order regardless: a slot with windows is a
 * full live-preview cell, an empty slot is a thin number slat (click/drop creates
 * and enters it). A desktop is a block of workspace ids (desktop d owns ids
 * [d*perDesktop+1 .. d*perDesktop+perDesktop]); the strip switches which block
 * shows. Drag a window onto another slot to move it, or onto a desktop card to
 * move it to that desktop.
 *
 * Motion model: scroll / Tab move a SELECTION highlight (no switch, no flicker);
 * Enter or a click commits and closes.
 */
Item {
    id: root

    property real s: 1
    property string screenName: ""
    property bool active: false
    property bool dataReady: false
    property bool focusHere: false
    signal requestClose()

    // ---- monitor geometry (logical layout coordinates) ------------------------
    readonly property var mon: Wm.outputByName(root.screenName)
    readonly property var monObj: root.mon
    readonly property real monScale: (root.monObj && root.monObj.scale > 0) ? root.monObj.scale : 1
    readonly property real monLW: (root.monObj && root.monObj.width  > 0) ? root.monObj.width  / root.monScale : 2560
    readonly property real monLH: (root.monObj && root.monObj.height > 0) ? root.monObj.height / root.monScale : 1600
    // Window geometry is global layout coordinates, so the mini-map origin is the
    // output's own top-left, taken from its screen.
    readonly property var screen: {
        var list = Quickshell.screens;
        for (var i = 0; i < list.length; i++)
            if (list[i] && list[i].name === root.screenName)
                return list[i];
        return null;
    }
    readonly property real monX: root.screen ? root.screen.x : 0
    readonly property real monY: root.screen ? root.screen.y : 0
    readonly property real aspect: root.monLW > 0 ? root.monLH / root.monLW : 0.625
    // Active workspace shown on this output; the fixed model keys it numerically.
    readonly property int activeWsId: {
        var n = (root.mon && root.mon.activeWorkspace) ? Number(root.mon.activeWorkspace) : NaN;
        return isNaN(n) ? -1 : n;
    }

    // ---- desktops = blocks of workspace ids -----------------------------------
    readonly property int perDesktop: 10
    function deskOf(id) { return id > 0 ? Math.floor((id - 1) / root.perDesktop) : 0; }
    readonly property int activeDesktop: root.deskOf(root.activeWsId)

    readonly property var allWsIds: {
        var out = [];
        var all = Wm.workspaces;
        for (var i = 0; i < all.length; i++) {
            var w = all[i];
            if (!w || w.special || w.output !== root.screenName)
                continue;
            var id = Number(w.name);
            if (id > 0)
                out.push(id);
        }
        out.sort(function (a, b) { return a - b; });
        return out;
    }
    readonly property var globalWsIds: {
        var out = [];
        var all = Wm.workspaces;
        for (var i = 0; i < all.length; i++) {
            var w = all[i];
            if (!w || w.special)
                continue;
            var id = Number(w.name);
            if (id > 0)
                out.push(id);
        }
        out.sort(function (a, b) { return a - b; });
        return out;
    }
    readonly property int maxOccDesk: {
        var m = 0;
        for (var i = 0; i < root.allWsIds.length; i++)
            m = Math.max(m, root.deskOf(root.allWsIds[i]));
        return Math.max(m, root.activeDesktop);
    }
    readonly property int maxOccDeskGlobal: {
        var m = 0;
        for (var i = 0; i < root.globalWsIds.length; i++)
            m = Math.max(m, root.deskOf(root.globalWsIds[i]));
        return Math.max(m, root.activeDesktop);
    }
    readonly property int newDesktopIdx: root.maxOccDeskGlobal + 1
    // Only desktops that actually hold workspaces, then ONE trailing "add
    // desktop" card. Empty desktops in between are never shown, so the strip
    // stays short (no wall of blank "NEW" cards).
    readonly property var deskList: {
        var out = [];
        var seen = ({});
        for (var i = 0; i < root.allWsIds.length; i++)
            seen[root.deskOf(root.allWsIds[i])] = true;
        seen[root.activeDesktop] = true;
        for (var d = 0; d <= root.maxOccDesk; d++)
            if (seen[d]) out.push(d);
        out.push(root.newDesktopIdx);
        return out;
    }
    property int viewedDesktop: 0
    function wsHasWindows(id) {
        var ws = Wm.workspaceByName(String(id));
        return !!ws && ws.occupied;
    }
    function deskDots(d) {
        var out = [];
        for (var i = 0; i < root.allWsIds.length; i++)
            if (root.deskOf(root.allWsIds[i]) === d)
                out.push(root.wsHasWindows(root.allWsIds[i]));
        return out;
    }
    function switchToDesktop(d) {
        root.viewedDesktop = d;
        root.selected = 0;
    }
    // "Add desktop": create AND move to a brand-new desktop's first workspace,
    // then close. Unlike switchToDesktop (which only previews in the grid), this
    // actually takes you to the new desktop.
    function createDesktop() {
        root.createAndEnterWs(root.newDesktopIdx * root.perDesktop + 1);
    }

    // ---- the viewed desktop's block + its occupancy ---------------------------
    readonly property int blockBase: root.viewedDesktop * root.perDesktop
    // ids of THIS desktop present in the workspace model.
    readonly property var wsList: {
        var out = [];
        for (var i = 0; i < root.allWsIds.length; i++)
            if (root.deskOf(root.allWsIds[i]) === root.viewedDesktop)
                out.push(root.allWsIds[i]);
        return out;
    }
    // first globally free id in this block, for the "+" add slot. If this
    // desktop is full, fall through to the next globally empty desktop.
    readonly property int newWsId: {
        var used = ({});
        for (var i = 0; i < root.globalWsIds.length; i++)
            used[root.globalWsIds[i]] = true;
        var lo = root.blockBase + 1;
        var hi = root.blockBase + root.perDesktop;
        for (var id = lo; id <= hi; id++)
            if (!used[id])
                return id;
        return root.newDesktopIdx * root.perDesktop + 1;
    }
    // The viewed desktop's occupied workspaces as full live cells, then ONE
    // full-size "+" add cell. Empty / gap positions are not shown, and the add
    // cell is always a full-size target (never a thin slat), so it is easy to
    // hit and clicking it opens the next free workspace.
    readonly property var slotModel: {
        var out = [];
        for (var i = 0; i < root.wsList.length; i++) {
            var id = root.wsList[i];
            if (root.wsHasWindows(id) || id === root.activeWsId)
                out.push({ pos: id - root.blockBase, wsId: id, add: false, full: true });
        }
        out.push({ pos: out.length + 1, wsId: -1, add: true, full: true });
        return out;
    }
    readonly property int slotCount: root.slotModel.length
    // The cells follow a keyed copy of slotModel, so a workspace frame keeps
    // each cell, its captures and its glide state while the slot stays.
    BusListModel {
        id: slotRows
        key: "wsId"
        roles: ["pos", "wsId", "add", "full"]
        rows: root.slotModel
    }

    // ---- desktop card geometry (shared with DesktopStrip + the drag hit-test) -
    readonly property real deskCardW: 122 * root.s
    readonly property real deskCardH: 58 * root.s
    readonly property real deskGap: 14 * root.s
    readonly property real stripTopMargin: 40 * root.s

    // ---- layout: cells wrap into as many rows as needed, shrinking so every
    // row fits the width AND all rows fit the height (no overflow, no scroll) ---
    readonly property real regionTop: root.stripTopMargin + root.deskCardH + 46 * root.s
    readonly property real regionBottom: root.height - 78 * root.s
    readonly property real availW: root.width - 2 * (72 * root.s)
    readonly property real availH: Math.max(150 * root.s, root.regionBottom - root.regionTop)
    readonly property real rowGap: 18 * root.s
    readonly property real thinRatio: 0.42  // a thin slat's width = full cell * this

    // Pack the slots into rows for a given full-cell height h, greedily wrapping
    // when the next cell would overflow availW. Returns the rows (each a list of
    // slot indices) and the total block height, or null if a single cell can't
    // even fit the width. Full cells are h/aspect wide; thin slats are narrower.
    function packRows(h) {
        var fw = h / root.aspect;
        var tw = fw * root.thinRatio;
        var rows = [], row = [], rowW = 0;
        for (var i = 0; i < root.slotModel.length; i++) {
            var w = root.slotModel[i].full ? fw : tw;
            if (w > root.availW)
                return null;
            var add = (row.length > 0 ? root.rowGap : 0) + w;
            if (rowW + add > root.availW && row.length > 0) {
                rows.push(row); row = []; rowW = 0; add = w;
            }
            row.push(i); rowW += add;
        }
        if (row.length > 0) rows.push(row);
        var blockH = rows.length * h + Math.max(0, rows.length - 1) * root.rowGap;
        return { rows: rows, blockH: blockH };
    }
    // Largest full-cell height (capped) whose packing fits the height budget.
    // Search downward in fixed steps; monotone, so the first fit is the biggest.
    readonly property var lay: {
        var hi = Math.min(root.availH, 460 * root.s);
        var lo = 132 * root.s;
        var step = 6 * root.s;
        var chosen = null, chosenH = lo;
        for (var h = hi; h >= lo; h -= step) {
            var p = root.packRows(h);
            if (p && p.blockH <= root.availH) { chosen = p; chosenH = h; break; }
        }
        if (!chosen) { chosen = root.packRows(lo) || { rows: [[]], blockH: lo }; chosenH = lo; }
        // place each slot: rows centred horizontally, block centred vertically.
        var fw = chosenH / root.aspect;
        var tw = fw * root.thinRatio;
        var items = [];
        var y0 = Math.max(0, (root.availH - chosen.blockH) / 2);
        for (var r = 0; r < chosen.rows.length; r++) {
            var ids = chosen.rows[r];
            var rw = 0;
            for (var a = 0; a < ids.length; a++)
                rw += (root.slotModel[ids[a]].full ? fw : tw) + (a > 0 ? root.rowGap : 0);
            var x = (root.availW - rw) / 2;
            var yy = y0 + r * (chosenH + root.rowGap);
            for (var c = 0; c < ids.length; c++) {
                var w = root.slotModel[ids[c]].full ? fw : tw;
                items[ids[c]] = { x: x, y: yy, w: w, h: chosenH };
                x += w + root.rowGap;
            }
        }
        return { items: items, cellH: chosenH, width: root.availW, height: chosen.blockH };
    }

    // ---- selection: scroll / Tab move it, Enter or click commits -------------
    property int selected: 0
    property bool seeded: false
    function trySeed() {
        if (root.seeded || !root.dataReady || root.activeWsId <= 0)
            return;
        root.viewedDesktop = root.activeDesktop;
        root.selected = Math.max(0, root.slotIndexOfWs(root.activeWsId));
        root.seeded = true;
    }
    onDataReadyChanged: root.trySeed()
    onActiveWsIdChanged: root.trySeed()
    // Re-seed each time the expo opens so the selection lands on the CURRENT
    // active workspace, not the one left over from the previous open. A fresh
    // instance got this for free; a resident one must reset it per show.
    onActiveChanged: {
        if (root.active) {
            root.seeded = false;
            root.trySeed();
        }
    }
    function slotIndexOfWs(id) {
        for (var i = 0; i < root.slotModel.length; i++)
            if (root.slotModel[i].wsId === id)
                return i;
        return -1;
    }
    function cycle(d) {
        var n = root.slotCount;
        if (n === 0) return;
        root.selected = ((root.selected + d) % n + n) % n;
    }
    function cycleDesktop(d) {
        var list = root.deskList;
        if (list.length === 0) return;
        var cur = list.indexOf(root.viewedDesktop);
        if (cur < 0) cur = 0;
        var nx = ((cur + d) % list.length + list.length) % list.length;
        root.switchToDesktop(list[nx]);
    }
    function activateSelected() {
        if (root.selected < 0 || root.selected >= root.slotModel.length)
            return;
        var slot = root.slotModel[root.selected];
        if (slot.add)
            root.createAndEnterWs(root.newWsId);
        else
            root.switchWs(slot.wsId);
    }

    // ---- actions --------------------------------------------------------------
    // Actions that dismiss the overview are DEFERRED until after it closes. The
    // overlay holds an exclusive keyboard grab; releasing it on close refocuses
    // the previously active window, which would undo a workspace/window switch
    // issued while the overlay was still up (you would land back where you
    // started). So: close first, then run the switch once the grab is released.
    property var pendingCommit: null
    Timer {
        id: commitTimer
        interval: 150
        repeat: false
        onTriggered: {
            var fn = root.pendingCommit;
            root.pendingCommit = null;
            if (fn)
                fn();
        }
    }
    function commitOnClose(fn) {
        root.pendingCommit = fn;
        root.requestClose();
        commitTimer.restart();
    }
    function switchWs(id) {
        root.commitOnClose(function () {
            Wm.focusWorkspace(String(id));
        });
    }
    function createAndEnterWs(id) {
        // No neutral focus-output action, so entering the workspace also moves
        // the shown output to the one that owns it.
        root.commitOnClose(function () {
            Wm.focusWorkspace(String(id));
        });
    }
    function focusWindow(id) {
        root.commitOnClose(function () {
            Wm.focusWindow(id);
        });
    }
    function moveWindow(id, wsId) {
        if (!id) return;
        Wm.moveWindowToWorkspace(id, String(wsId));
    }
    function closeWindow(id) {
        if (id)
            Wm.closeWindow(id);
    }
    function landingWsForDesk(d) {
        var lo = -1;
        for (var i = 0; i < root.allWsIds.length; i++)
            if (root.deskOf(root.allWsIds[i]) === d && (lo < 0 || root.allWsIds[i] < lo))
                lo = root.allWsIds[i];
        return lo > 0 ? lo : d * root.perDesktop + 1;
    }

    // ---- hand-tracked drag state ----------------------------------------------
    readonly property int noWs: -999
    property bool dragging: false
    property string dragAddr: ""
    property int dragSrcWs: noWs
    property int dragTargetWs: noWs
    property int dragTargetDesk: noWs
    property real dragX: 0
    property real dragY: 0
    property var dragTl: null
    function endDrag() {
        root.dragging = false;
        root.dragTargetWs = root.noWs;
        root.dragTargetDesk = root.noWs;
        root.dragSrcWs = root.noWs;
        root.dragAddr = "";
        root.dragTl = null;
    }
    function updateDrag(rx, ry) {
        root.dragX = rx;
        root.dragY = ry;
        var d = root.deskAtRoot(rx, ry);
        if (d !== root.noWs) {
            root.dragTargetDesk = d;
            root.dragTargetWs = root.noWs;
        } else {
            root.dragTargetDesk = root.noWs;
            root.dragTargetWs = root.targetAtRoot(rx, ry);
        }
    }
    function commitDrop() {
        if (root.dragTargetDesk !== root.noWs && root.dragTargetDesk !== root.deskOf(root.dragSrcWs))
            root.moveWindow(root.dragAddr, root.landingWsForDesk(root.dragTargetDesk));
        else if (root.dragTargetWs !== root.noWs && root.dragTargetWs !== root.dragSrcWs)
            root.moveWindow(root.dragAddr, root.dragTargetWs);
    }
    // resolve the slot under a point (in gridWrap coords) against the wrapped
    // layout boxes. Empty slots map to their real id (creates on drop).
    function cellAt(px, py) {
        var it = root.lay.items;
        for (var i = 0; i < root.slotModel.length; i++) {
            var b = it[i];
            if (b && px >= b.x && px <= b.x + b.w && py >= b.y && py <= b.y + b.h) {
                var slot = root.slotModel[i];
                return slot.add ? root.newWsId : slot.wsId;
            }
        }
        return root.noWs;
    }
    function targetAtRoot(px, py) {
        return root.cellAt(px - gridWrap.x, py - gridWrap.y);
    }
    function deskAtRoot(px, py) {
        var lx = px - stripWrap.x;
        var ly = py - stripWrap.y;
        if (ly < 0 || ly > root.deskCardH || lx < 0)
            return root.noWs;
        var unit = root.deskCardW + root.deskGap;
        var i = Math.floor(lx / unit);
        if (lx - i * unit > root.deskCardW)
            return root.noWs;
        if (i < 0 || i >= root.deskList.length)
            return root.noWs;
        return root.deskList[i];
    }

    // ---- poster mark, top-left ------------------------------------------------
    Row {
        id: mark
        anchors.left: parent.left
        anchors.top: parent.top
        anchors.leftMargin: 52 * root.s
        anchors.topMargin: 46 * root.s
        spacing: 11 * root.s

        BrandMark {
            anchors.verticalCenter: parent.verticalCenter
            size: 20 * root.s
        }
        Column {
            anchors.verticalCenter: parent.verticalCenter
            spacing: 1 * root.s
            Text {
                text: I18n.tr("OVERVIEW")
                color: Theme.cream
                font.family: Theme.mono
                font.pixelSize: 12 * root.s
                font.letterSpacing: 3 * root.s
                font.weight: Font.DemiBold
            }
            Text {
                text: I18n.tr("DESKTOP %1").arg(("0" + (root.viewedDesktop + 1)).slice(-2))
                color: Theme.faint
                font.family: Theme.mono
                font.pixelSize: 9 * root.s
                font.letterSpacing: 2 * root.s
            }
        }
    }

    // ---- desktop strip (top centre) -------------------------------------------
    Item {
        id: stripWrap
        x: (root.width - stripInner.width) / 2
        y: root.stripTopMargin
        width: stripInner.width
        height: root.deskCardH
        DesktopStrip {
            id: stripInner
            s: root.s
            ov: root
        }
    }

    // ---- the wrapped grid -----------------------------------------------------
    // gridWrap spans the whole available region; each cell is placed absolutely
    // from lay.items (rows centred, block centred), so hit-test == render.
    Item {
        id: gridWrap
        x: 72 * root.s
        y: root.regionTop
        width: root.availW
        height: root.availH

        Repeater {
            model: slotRows
            delegate: WorkspaceCell {
                id: cell
                required property var modelData
                required property int index
                readonly property var box: root.lay.items[cell.index] || ({ x: 0, y: 0, w: 10, h: 10 })
                x: cell.box.x
                y: cell.box.y
                width: cell.box.w
                height: cell.box.h
                s: root.s
                ov: root
                idx: cell.index
                wsId: cell.modelData.wsId
                isAdd: cell.modelData.add === true
                full: cell.modelData.full === true
                selected: root.selected === cell.index
                Behavior on x { enabled: cell.appeared; NumberAnimation { duration: Motion.standard; easing.type: Motion.easeExpo } }
                Behavior on y { enabled: cell.appeared; NumberAnimation { duration: Motion.standard; easing.type: Motion.easeExpo } }
            }
        }
    }

    // ---- footer hint ----------------------------------------------------------
    Text {
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.bottom: parent.bottom
        anchors.bottomMargin: 38 * root.s
        text: I18n.tr("SCROLL / TAB  CYCLE      ALT+TAB  DESKTOP      DRAG  MOVE      \u2715  CLOSE      R-CLICK / ENTER  GO      ESC  DISMISS")
        color: Theme.faint
        font.family: Theme.mono
        font.pixelSize: 10 * root.s
        font.letterSpacing: 2 * root.s
        opacity: 0.85
    }

    WheelHandler {
        acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
        onWheel: (e) => {
            if (e.angleDelta.y === 0) return;
            root.cycle(e.angleDelta.y < 0 ? 1 : -1);
        }
    }

    // ---- carried-window ghost -------------------------------------------------
    Item {
        anchors.fill: parent
        z: 100

        Rectangle {
            visible: ghost.visible
            width: ghost.width
            height: ghost.height
            x: ghost.x + 8 * root.s
            y: ghost.y + 8 * root.s
            radius: Theme.radius
            color: Theme.shadow
            opacity: 0.8
            antialiasing: false
        }

        Rectangle {
            id: ghost
            visible: root.dragging
            width: 150 * root.s
            height: Math.round(150 * root.s * root.aspect)
            x: root.dragX - width / 2
            y: root.dragY - height / 2
            radius: Theme.radius
            color: Theme.tileBg
            border.width: 2
            border.color: Theme.brand
            antialiasing: true
            scale: root.dragging ? 1 : 0.9
            Behavior on scale { NumberAnimation { duration: Motion.fast; easing.type: Motion.easeExpo } }

            ScreencopyView {
                anchors.fill: parent
                anchors.margins: 2
                captureSource: root.dragTl
                live: false
                visible: root.dragTl !== null
            }
        }
    }
}
