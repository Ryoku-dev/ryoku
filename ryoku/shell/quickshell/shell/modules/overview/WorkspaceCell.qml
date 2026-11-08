pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import Quickshell.Widgets
import Ryoku.Bus
import "Singletons"
import Ryoku.Ui.Singletons
import shell.services as Svc

/**
 * One workspace as a scaled mini-desktop. A large Fraunces workspace numeral is
 * the identity (vermillion when active); windows sit at their real fractional
 * positions, each a LIVE ScreencopyView over an app-icon fallback. Every cell
 * carries a hard solid-black offset shadow (Ryoku brutalist depth, no blur), and
 * Left-click empty space to switch here, or a window to focus it; RIGHT-CLICK
 * anywhere enters that workspace. Press-drag a window onto another cell to move
 * it. In the scrolling layout a workspace is an infinite horizontal tape, so the
 * whole tape is shown uniformly scaled with the on-screen slice framed. The
 * trailing cell (wsId -1) is the "+" that opens a fresh workspace. Tiles glide
 * via Behaviors so a move/drop/reflow animates instead of snapping.
 */
Item {
    id: cell

    property real s: 1
    property var ov: null
    property int wsId: -1
    property int idx: 0
    property bool selected: false
    property bool isAdd: false
    property bool full: true
    readonly property int targetWs: cell.isAdd ? (cell.ov ? cell.ov.newWsId : 0) : cell.wsId
    readonly property bool active: !!cell.ov && cell.wsId === cell.ov.activeWsId
    readonly property bool hot: !!cell.ov && cell.ov.dragging
        && cell.ov.dragTargetWs === cell.targetWs && cell.ov.dragTargetWs !== cell.ov.dragSrcWs
    property bool hovered: false
    readonly property bool lifted: cell.hovered || cell.hot || cell.selected || cell.active

    function clamp(v, lo, hi) { return Math.max(lo, Math.min(hi, v)); }

    // staggered entrance: each cell settles in a beat after the last.
    property bool appeared: false
    Timer {
        interval: 40 + cell.idx * 55
        running: !!cell.ov && cell.ov.active && cell.ov.dataReady && !cell.appeared
        repeat: false
        onTriggered: cell.appeared = true
    }
    // cells outlive the overview closing, so they settle back out and every open plays the entrance again
    Timer {
        interval: Motion.window
        running: !!cell.ov && !cell.ov.active && cell.appeared
        repeat: false
        onTriggered: cell.appeared = false
    }
    opacity: cell.appeared ? 1 : 0
    scale: cell.appeared ? 1 : 0.92
    Behavior on opacity { NumberAnimation { duration: Motion.standard; easing.type: Motion.easeExpo } }
    Behavior on scale { NumberAnimation { duration: Motion.standard; easing.type: Motion.easeExpo } }

    // Windows on this ws mapped into the cell. Positions normalise against the
    // CONTENT BOUNDS -- the window bounding box unioned with the monitor viewport
    // -- not the viewport alone. In a normal (dwindle/master) layout the windows
    // sit inside the viewport, so the bounds ARE the viewport and this is the
    // plain monitor mini-map (the transform reduces exactly to it). In the
    // SCROLLING layout the workspace is an infinite horizontal tape: off-screen
    // columns report x past the monitor, so the bounds widen and the whole tape
    // is shown, uniformly scaled (windows keep aspect, letterboxed into a band)
    // with the on-screen slice framed. One transform, both layouts.
    readonly property var geom: {
        var out = { cards: [], tape: false, vp: null };
        if (cell.isAdd || !cell.full || !cell.ov || !cell.ov.mon)
            return out;
        var ov = cell.ov;
        var vx0 = ov.monX, vy0 = ov.monY, vx1 = ov.monX + ov.monLW, vy1 = ov.monY + ov.monLH;
        var cx0 = vx0, cy0 = vy0, cx1 = vx1, cy1 = vy1;   // content bounds seeded with the viewport
        var raw = [];
        var wins = Wm.windows;
        for (var i = 0; i < wins.length; i++) {
            var w = wins[i];
            if (!w || w.workspace !== String(cell.wsId))
                continue;
            if (w.width <= 0 || w.height <= 0)
                continue;
            var ax = w.x, ay = w.y, aw = w.width, ah = w.height;
            raw.push({ addr: w.id, tl: w.toplevel, cls: (w.appId || "").toLowerCase(), ax: ax, ay: ay, aw: aw, ah: ah });
            cx0 = Math.min(cx0, ax); cy0 = Math.min(cy0, ay);
            cx1 = Math.max(cx1, ax + aw); cy1 = Math.max(cy1, ay + ah);
        }
        var cw = Math.max(1, cx1 - cx0), ch = Math.max(1, cy1 - cy0);
        var asp = cell.ov.aspect;                 // cell height / width
        var m = Math.min(1 / cw, asp / ch);       // uniform fit, independent of pixel size
        var ox = (1 - cw * m) / 2;                // x letterbox, as a fraction of cell width
        var oy = (asp - ch * m) / (2 * asp);      // y letterbox, as a fraction of cell height
        function mapRect(ax, ay, aw, ah) {
            return { fx: ox + (ax - cx0) * m, fy: oy + (ay - cy0) * m / asp,
                     fw: aw * m, fh: ah * m / asp };
        }
        for (var j = 0; j < raw.length; j++) {
            var r = raw[j], q = mapRect(r.ax, r.ay, r.aw, r.ah);
            out.cards.push({ addr: r.addr, tl: r.tl, cls: r.cls, fx: q.fx, fy: q.fy, fw: q.fw, fh: q.fh });
        }
        out.tape = cw > ov.monLW + 1;             // windows spilled past the viewport = a tape
        out.vp = out.tape ? mapRect(vx0, vy0, ov.monLW, ov.monLH) : null;
        return out;
    }
    readonly property var cards: cell.geom.cards
    readonly property bool tape: cell.geom.tape
    readonly property var viewportFrac: cell.geom.vp

    BusListModel {
        id: cardRows
        key: "addr"
        roles: ["addr", "tl", "cls", "fx", "fy", "fw", "fh"]
        rows: cell.cards
    }

    // distinct app icons open on this workspace (dedup by class, capped), for the
    // small roster under the number badge.
    readonly property var _liveAppIcons: {
        var seen = ({});
        var out = [];
        for (var i = 0; i < cell.cards.length && out.length < 6; i++) {
            var c = cell.cards[i].cls;
            if (!c || seen[c])
                continue;
            seen[c] = true;
            var e = DesktopEntries.heuristicLookup(c);
            var p = (e && e.icon) ? Svc.Icons.path(e.icon, true) : Svc.Icons.path(c, true);
            if (p)
                out.push(p);
        }
        return out;
    }
    property var appIcons: []
    function _adoptAppIcons() {
        var next = cell._liveAppIcons;
        if (next.join("\u0000") !== cell.appIcons.join("\u0000"))
            cell.appIcons = next;
    }
    on_LiveAppIconsChanged: cell._adoptAppIcons()
    Component.onCompleted: cell._adoptAppIcons()

    // hard offset shadow behind EVERY cell (solid black, no blur; the depth is
    // the offset, per docs/ui-ux.md). Deepens as the cell lifts.
    Rectangle {
        x: cell.lifted ? 10 * cell.s : 7 * cell.s
        y: cell.lifted ? 10 * cell.s : 7 * cell.s
        width: mini.width
        height: mini.height
        radius: Theme.radius
        color: Theme.shadow
        opacity: cell.active ? 0.9 : (cell.lifted ? 0.8 : 0.6)
        antialiasing: false
        Behavior on x { NumberAnimation { duration: Motion.fast; easing.type: Motion.easeStandard } }
        Behavior on y { NumberAnimation { duration: Motion.fast; easing.type: Motion.easeStandard } }
        Behavior on opacity { NumberAnimation { duration: Motion.fast } }
    }

    Rectangle {
        id: mini
        anchors.fill: parent
        radius: Theme.radius
        color: cell.active ? Qt.alpha(Theme.brand, 0.08) : Theme.cardBot
        border.width: (cell.active || cell.hot || cell.selected) ? 2 : 1
        border.color: cell.hot ? Theme.brand
            : cell.active ? Theme.brand
            : cell.selected ? Theme.vermLit
            : cell.hovered ? Theme.frameBorder
            : Theme.border
        clip: true

        Behavior on border.color { ColorAnimation { duration: Motion.highlight } }
        Behavior on color { ColorAnimation { duration: Motion.highlight } }

        // wallpaper backdrop: the real desktop background, cropped to the cell and
        // dimmed so the windows (and the number/icons) read on top. Backmost
        // child, above the flat fill. The add cell stays flat.
        Image {
            anchors.fill: parent
            visible: !cell.isAdd && Config.wallpaper.length > 0
            source: Config.wallpaper
            fillMode: Image.PreserveAspectCrop
            asynchronous: true
            cache: true
            smooth: true
            sourceSize.width: Math.round(parent.width * 1.2)
            opacity: cell.active ? 0.62 : 0.42
            Behavior on opacity { NumberAnimation { duration: Motion.highlight } }
            // a thin dim veil so bright wallpapers never wash out the previews.
            Rectangle { anchors.fill: parent; color: Qt.rgba(0, 0, 0, 0.28) }
        }

        // Ryoku-style workspace numeral: a bold zero-padded mono ordinal in the
        // top-left, vermillion when active/hovered.
        // Above the previews (z), on a faint chip so it reads over any window.
        Rectangle {
            id: numChip
            anchors.top: parent.top
            anchors.left: parent.left
            anchors.margins: 8 * cell.s
            visible: !cell.isAdd && cell.full
            width: numText.implicitWidth + 12 * cell.s
            height: numText.implicitHeight + 5 * cell.s
            radius: Theme.radius
            color: Qt.rgba(0, 0, 0, 0.5)
            border.width: 1
            border.color: cell.active ? Theme.brand : Theme.hair
            z: 3
            Behavior on border.color { ColorAnimation { duration: Motion.highlight } }
            Text {
                id: numText
                anchors.centerIn: parent
                // position within the desktop (ws11 on desktop 2 reads "01"), so
                // every desktop counts its own 01, 02, 03... Ryoku-style padded.
                readonly property int pos: cell.ov ? (((cell.wsId - 1) % cell.ov.perDesktop) + 1) : cell.wsId
                text: (numText.pos < 10 ? "0" : "") + numText.pos
                color: (cell.active || cell.hovered) ? Theme.brand : Theme.cream
                font.family: Theme.mono
                font.pixelSize: 22 * cell.s
                font.weight: Font.DemiBold
                Behavior on color { ColorAnimation { duration: Motion.highlight } }
            }
        }

        // app-icon roster: the distinct apps open on this workspace, drawn small
        // (below the number badge, smaller than it) as a quick glance of what
        // lives here beyond the live previews.
        Row {
            anchors.top: numChip.bottom
            anchors.left: numChip.left
            anchors.topMargin: 6 * cell.s
            spacing: 5 * cell.s
            visible: !cell.isAdd && cell.full && cell.cards.length > 0
            z: 3
            Repeater {
                model: cell.appIcons
                delegate: Rectangle {
                    id: appIco
                    required property var modelData
                    width: 22 * cell.s
                    height: 22 * cell.s
                    radius: Theme.radius
                    color: Qt.rgba(0, 0, 0, 0.5)
                    border.width: 1
                    border.color: Theme.hair
                    IconImage {
                        anchors.centerIn: parent
                        implicitSize: 15 * cell.s
                        source: appIco.modelData
                    }
                }
            }
        }

        // thin-slat identity: an empty/gap workspace shows just its number,
        // centered. The slat keeps the gap visible + reachable (click/drop
        // creates it) without a full preview cell's footprint.
        Text {
            anchors.centerIn: parent
            visible: !cell.isAdd && !cell.full
            readonly property int pos: cell.ov ? (((cell.wsId - 1) % cell.ov.perDesktop) + 1) : cell.wsId
            text: (pos < 10 ? "0" : "") + pos
            color: (cell.active || cell.hovered || cell.selected) ? Theme.brand : Theme.faint
            font.family: Theme.mono
            font.pixelSize: 19 * cell.s
            font.weight: Font.DemiBold
            Behavior on color { ColorAnimation { duration: Motion.highlight } }
        }

        // "EMPTY" note for a windowless workspace.
        Text {
            anchors.centerIn: parent
            visible: !cell.isAdd && cell.full && cell.cards.length === 0
            text: I18n.tr("EMPTY")
            color: Theme.faint
            font.family: Theme.mono
            font.pixelSize: 10 * cell.s
            font.letterSpacing: 2 * cell.s
        }

        // scrolling tape: the slice currently on screen, framed within the whole
        // tape so the current viewport is legible against the off-screen columns.
        // Above the wallpaper, below the window tiles and the number badge.
        Rectangle {
            visible: cell.tape && cell.viewportFrac !== null
            x: (cell.viewportFrac ? cell.viewportFrac.fx : 0) * mini.width
            y: (cell.viewportFrac ? cell.viewportFrac.fy : 0) * mini.height
            width: (cell.viewportFrac ? cell.viewportFrac.fw : 0) * mini.width
            height: (cell.viewportFrac ? cell.viewportFrac.fh : 0) * mini.height
            radius: Theme.radius
            color: Qt.alpha(Theme.brand, 0.10)
            border.width: 1
            border.color: Qt.alpha(Theme.brand, 0.55)
            antialiasing: true
        }

        // tape marker: a horizontal double-arrow so a scrolling workspace reads
        // as scrollable at a glance (bottom centre, faint).
        Text {
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.bottom: parent.bottom
            anchors.bottomMargin: 6 * cell.s
            visible: cell.tape
            text: "\u21C6"
            color: Theme.brand
            font.family: Theme.mono
            font.pixelSize: 15 * cell.s
            opacity: 0.85
            z: 3
        }

        // click empty space -> switch here (windows above capture their own).
        // left OR right on bare workspace space enters it. Right-click also enters
        // from on top of a window (handled in the tile's own MouseArea below).
        MouseArea {
            anchors.fill: parent
            visible: !cell.isAdd
            hoverEnabled: true
            acceptedButtons: Qt.LeftButton | Qt.RightButton
            cursorShape: Qt.PointingHandCursor
            onEntered: cell.hovered = true
            onExited: cell.hovered = false
            onClicked: if (cell.ov) cell.ov.switchWs(cell.wsId)
        }

        // ---- live window tiles ----
        Repeater {
            model: cardRows
            delegate: Rectangle {
                id: tile
                required property var modelData
                readonly property string addr: tile.modelData.addr

                x: tile.modelData.fx * mini.width
                y: tile.modelData.fy * mini.height
                width: Math.max(20 * cell.s, tile.modelData.fw * mini.width)
                height: Math.max(16 * cell.s, tile.modelData.fh * mini.height)
                radius: Theme.radius
                color: Theme.tileBg
                border.width: (tileMa.containsMouse || closeMa.containsMouse) ? 2 : 1
                border.color: (tileMa.containsMouse || closeMa.containsMouse) ? Theme.brand : Theme.hair
                antialiasing: true
                clip: true
                z: (tileMa.containsMouse || closeMa.containsMouse) ? 2 : 1
                // fade the carried window in place while it is being dragged.
                opacity: (cell.ov && cell.ov.dragging && tile.addr === cell.ov.dragAddr) ? 0.22 : 1

                // glide when the model reflows (a move/drop rearranges tiles).
                // enabled only after entrance, so the first layout doesn't slide
                // every tile in from the corner.
                Behavior on x { enabled: cell.appeared; NumberAnimation { duration: Motion.standard; easing.type: Motion.easeExpo } }
                Behavior on y { enabled: cell.appeared; NumberAnimation { duration: Motion.standard; easing.type: Motion.easeExpo } }
                Behavior on width { enabled: cell.appeared; NumberAnimation { duration: Motion.standard; easing.type: Motion.easeExpo } }
                Behavior on height { enabled: cell.appeared; NumberAnimation { duration: Motion.standard; easing.type: Motion.easeExpo } }
                Behavior on border.color { ColorAnimation { duration: Motion.highlight } }
                Behavior on opacity { NumberAnimation { duration: Motion.highlight } }

                // icon fallback, beneath the live capture.
                IconImage {
                    anchors.centerIn: parent
                    implicitSize: Math.max(16 * cell.s, Math.min(48 * cell.s, Math.min(tile.width, tile.height) * 0.5))
                    source: {
                        var c = tile.modelData.cls;
                        var e = c ? DesktopEntries.heuristicLookup(c) : null;
                        var p = (e && e.icon) ? Svc.Icons.path(e.icon, true) : "";
                        if (!p && c)
                            p = Svc.Icons.path(c, true);
                        return p;
                    }
                }

                // A one-shot capture (not live): the overview is a momentary
                // picker, so per-frame window capture is what dropped it to 30fps.
                ScreencopyView {
                    anchors.fill: parent
                    captureSource: (!!cell.ov && cell.ov.active && tile.modelData.tl) ? tile.modelData.tl : null
                    live: false
                    visible: tile.modelData.tl !== null
                }

                MouseArea {
                    id: tileMa
                    anchors.fill: parent
                    hoverEnabled: true
                    preventStealing: true
                    acceptedButtons: Qt.LeftButton | Qt.RightButton
                    cursorShape: Qt.PointingHandCursor
                    property real downX: 0
                    property real downY: 0
                    property bool armed: false
                    property int btn: Qt.LeftButton

                    // left arms a drag-or-focus; right never drags, it just enters
                    // this window's workspace on release.
                    onPressed: (m) => {
                        tileMa.btn = m.button;
                        tileMa.armed = (m.button === Qt.LeftButton);
                        tileMa.downX = m.x; tileMa.downY = m.y;
                    }
                    onPositionChanged: (m) => {
                        if (!tileMa.armed || !cell.ov)
                            return;
                        if (!cell.ov.dragging) {
                            if (Math.abs(m.x - tileMa.downX) + Math.abs(m.y - tileMa.downY) < 7 * cell.s)
                                return;
                            cell.ov.dragging = true;
                            cell.ov.dragAddr = tile.addr;
                            cell.ov.dragSrcWs = cell.wsId;
                            cell.ov.dragTl = tile.modelData.tl;
                        }
                        var rp = tileMa.mapToItem(cell.ov, m.x, m.y);
                        cell.ov.updateDrag(rp.x, rp.y);
                    }
                    onReleased: {
                        if (cell.ov && cell.ov.dragging) {
                            cell.ov.commitDrop();
                            cell.ov.endDrag();
                        } else if (tileMa.btn === Qt.RightButton && cell.ov) {
                            cell.ov.switchWs(cell.wsId);
                        } else if (tileMa.armed && cell.ov) {
                            cell.ov.focusWindow(tile.addr);
                        }
                        tileMa.armed = false;
                    }
                    onCanceled: { if (cell.ov) cell.ov.endDrag(); tileMa.armed = false; }
                }

                // hover ✕ close, top-right. Above tileMa, so it eats its own click.
                Rectangle {
                    id: closeBtn
                    anchors.top: parent.top
                    anchors.right: parent.right
                    anchors.margins: 3 * cell.s
                    width: 18 * cell.s
                    height: 18 * cell.s
                    radius: Theme.radius
                    visible: (tileMa.containsMouse || closeMa.containsMouse) && !(cell.ov && cell.ov.dragging)
                    color: closeMa.containsMouse ? Theme.brand : Qt.rgba(0, 0, 0, 0.72)
                    border.width: 1
                    border.color: closeMa.containsMouse ? Theme.brand : Theme.frameBorder
                    Text {
                        anchors.centerIn: parent
                        text: "\u2715"
                        color: closeMa.containsMouse ? Theme.bright : Theme.cream
                        font.family: Theme.mono
                        font.pixelSize: 11 * cell.s
                        font.weight: Font.Bold
                    }
                    MouseArea {
                        id: closeMa
                        anchors.fill: parent
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onClicked: if (cell.ov) cell.ov.closeWindow(tile.addr)
                    }
                }
            }
        }

        // ---- the "+" add cell ----
        Item {
            anchors.fill: parent
            visible: cell.isAdd
            Rectangle {
                anchors.centerIn: parent
                width: 26 * cell.s; height: 3 * cell.s; radius: height / 2
                color: cell.hot || cell.hovered ? Theme.brand : Theme.faint
                Behavior on color { ColorAnimation { duration: Motion.highlight } }
            }
            Rectangle {
                anchors.centerIn: parent
                width: 3 * cell.s; height: 26 * cell.s; radius: width / 2
                color: cell.hot || cell.hovered ? Theme.brand : Theme.faint
                Behavior on color { ColorAnimation { duration: Motion.highlight } }
            }
            MouseArea {
                anchors.fill: parent
                hoverEnabled: true
                acceptedButtons: Qt.LeftButton | Qt.RightButton
                cursorShape: Qt.PointingHandCursor
                onEntered: cell.hovered = true
                onExited: cell.hovered = false
                onClicked: if (cell.ov) cell.ov.createAndEnterWs(cell.ov.newWsId)
            }
        }
    }
}
