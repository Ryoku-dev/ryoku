pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Shapes
import Ryoku.Ui.Singletons
import "../../kit"
import "../edit" as E
import "../../settings" as S
import "../Catalogue.js" as Catalogue

// android 17's edit mode: hold and drag to move, the side handle to resize, drag up to remove
Item {
    id: page

    property var host

    implicitHeight: sections.implicitHeight
    focus: true

    S.IslandSettings { id: cfg }

    readonly property int columns: 4
    readonly property int gap: 8
    readonly property int tileHeight: 72
    readonly property real unit: (page.width - (page.columns - 1) * page.gap) / page.columns
    readonly property real mid: page.unit + (page.unit + page.gap) / 2
    readonly property int chipSize: 48
    readonly property int chipGap: 8
    readonly property int glanceMax: cfg.glanceMax

    property bool ready: false
    Timer {
        interval: 80
        running: true
        onTriggered: page.ready = true
    }

    // quick clicks land before the daemon echoes the last save, so edits build on what was sent
    property var pending: null
    readonly property var saved: cfg.tiles.filter(function (t) { return Catalogue.ids.indexOf(t.id) >= 0; })
    onSavedChanged: if (page.pending && JSON.stringify(page.pending) === JSON.stringify(page.saved)) page.pending = null
    readonly property var tiles: page.pending || page.saved

    property var pendingGlance: null
    readonly property var savedGlance: cfg.glanceTiles.filter(function (id) { return Catalogue.ids.indexOf(id) >= 0; })
    onSavedGlanceChanged: if (page.pendingGlance && JSON.stringify(page.pendingGlance) === JSON.stringify(page.savedGlance)) page.pendingGlance = null
    readonly property var glance: page.pendingGlance || page.savedGlance

    property string picked: ""
    property string pickedKind: ""

    // the lifted tile or chip: gx/gy is its corner, px/py the pointer, both in page coordinates
    property string dragKind: ""
    property string dragId: ""
    property string dragFrom: ""
    property bool tracking: false
    property real gx: 0
    property real gy: 0
    property real px: 0
    property real py: 0
    property real offX: 0
    property real offY: 0
    property string target: ""
    property int gridIndex: -1
    property int chipIndex: -1
    property bool moved: false

    property bool sizing: false
    property bool sizeTracking: false
    property string sizeId: ""
    property real sizeFrom: 0
    property real sizeStart: 0
    property real sizeW: 0

    function indexIn(list, id) {
        for (var i = 0; i < list.length; i++)
            if (list[i].id === id) return i;
        return -1;
    }
    function same(a, b) { return JSON.stringify(a) === JSON.stringify(b) }

    // a large tile that no longer fits its row starts the next one, as on the home grid
    function pack(list) {
        var map = {}, row = 0, col = 0;
        for (var i = 0; i < list.length; i++) {
            var span = list[i].large ? 2 : 1;
            if (col + span > page.columns) {
                row++;
                col = 0;
            }
            map[list[i].id] = {
                x: col * (page.unit + page.gap),
                y: row * (page.tileHeight + page.gap),
                w: span === 2 ? 2 * page.unit + page.gap : page.unit,
                large: span === 2
            };
            col += span;
        }
        return { map: map, rows: list.length > 0 ? row + 1 : 0 };
    }

    // what the grid shows right now: the committed order with the lifted tile at its insertion point
    readonly property var gridView: {
        var list = page.tiles.slice();
        if (page.dragKind !== "tile" || page.target !== "grid" || page.gridIndex < 0)
            return list;
        var i = page.indexIn(list, page.dragId);
        var item = i >= 0 ? list[i] : { id: page.dragId, large: false };
        if (i >= 0)
            list.splice(i, 1);
        list.splice(Math.min(page.gridIndex, list.length), 0, item);
        return list;
    }
    readonly property var gridPack: page.pack(page.gridView)
    readonly property var spareView: Catalogue.ids.filter(function (id) { return page.indexIn(page.gridView, id) < 0; })
    // packed from gridView in one pass: reading gridPack and spareView here could mix an old and a
    // new list for a moment, the lifted tile would have no place and lose the pointer
    readonly property var places: {
        var out = {};
        var view = page.gridView;
        var map = page.pack(view).map;
        for (var id in map)
            out[id] = { x: gridBox.x + map[id].x, y: gridBox.y + map[id].y, w: map[id].w, large: map[id].large, where: "grid" };
        var k = 0;
        for (var i = 0; i < Catalogue.ids.length; i++) {
            var spare = Catalogue.ids[i];
            if (map[spare] !== undefined)
                continue;
            out[spare] = {
                x: spareBox.x + (k % page.columns) * (page.unit + page.gap),
                y: spareBox.y + Math.floor(k / page.columns) * (page.tileHeight + page.gap),
                w: page.unit, large: false, where: "spare"
            };
            k++;
        }
        return out;
    }

    readonly property var chipView: {
        var list = page.glance.slice();
        if (page.target !== "chips" || page.chipIndex < 0)
            return list;
        var i = list.indexOf(page.dragId);
        if (i >= 0)
            list.splice(i, 1);
        list.splice(Math.min(page.chipIndex, list.length), 0, page.dragId);
        return list;
    }
    readonly property real chipRowX: chipSeg.x + (chipSeg.width - (page.glanceMax * page.chipSize + (page.glanceMax - 1) * page.chipGap)) / 2
    readonly property real chipY: chipGroup.y + (chipSeg.height - page.chipSize) / 2
    function chipX(k) { return page.chipRowX + k * (page.chipSize + page.chipGap) }

    // the scroller around the page, so the remove target can sit at the top of what is visible
    readonly property Flickable scroller: {
        var p = page.parent;
        while (p && (p.contentY === undefined || p.flicking === undefined))
            p = p.parent;
        return p || null;
    }
    readonly property real viewTop: page.scroller ? page.scroller.contentY : 0
    readonly property real viewHeight: page.scroller ? page.scroller.height : page.height
    readonly property bool removable: page.dragKind === "chip" || (page.dragKind === "tile" && page.dragFrom === "grid")
    readonly property bool removeBlocked: page.dragKind === "tile" && page.tiles.length <= 1

    function commit(list) {
        page.pending = list;
        cfg.setTiles(list);
    }
    function commitGlance(list) {
        var next = list.slice(0, page.glanceMax);
        page.pendingGlance = next;
        cfg.setGlanceTiles(next);
    }
    function pick(kind, id) {
        page.pickedKind = id.length > 0 ? kind : "";
        page.picked = id;
        page.forceActiveFocus();
    }

    function addTile(id) {
        if (page.indexIn(page.tiles, id) >= 0) return;
        var list = page.tiles.slice();
        list.push({ id: id, large: false });
        page.commit(list);
        page.pick("tile", id);
    }
    function removeTile(id) {
        var i = page.indexIn(page.tiles, id);
        if (i < 0 || page.tiles.length <= 1) return;
        var list = page.tiles.slice();
        list.splice(i, 1);
        if (page.picked === id) page.pick("", "");
        page.commit(list);
    }
    function moveTile(id, d) {
        var i = page.indexIn(page.tiles, id), j = i + d;
        if (i < 0 || j < 0 || j >= page.tiles.length) return;
        var list = page.tiles.slice();
        var t = list[i];
        list[i] = list[j];
        list[j] = t;
        page.commit(list);
    }
    // up and down go to the slot nearest the one a row away
    function moveTileRow(id, d) {
        var i = page.indexIn(page.tiles, id);
        if (i < 0) return;
        var here = page.pack(page.tiles).map[id];
        var base = page.tiles.slice();
        var item = base.splice(i, 1)[0];
        var j = page.nearest(base, item, here.x + here.w / 2, here.y + page.tileHeight / 2 + d * (page.tileHeight + page.gap), -1);
        if (j === i) j = Math.max(0, Math.min(base.length, i + d));
        base.splice(j, 0, item);
        page.commit(base);
    }
    function setSize(id, large) {
        var i = page.indexIn(page.tiles, id);
        if (i < 0 || page.tiles[i].large === large) return;
        var list = page.tiles.slice();
        list[i] = { id: id, large: large };
        page.commit(list);
    }

    function addChip(id, at) {
        var list = page.glance.slice();
        var i = list.indexOf(id);
        if (i >= 0) list.splice(i, 1);
        list.splice(Math.max(0, Math.min(at, list.length)), 0, id);
        page.commitGlance(list);
    }
    function removeChip(id) {
        var list = page.glance.filter(function (x) { return x !== id; });
        if (page.picked === id && page.pickedKind === "chip") page.pick("", "");
        page.commitGlance(list);
    }
    function moveChip(id, d) {
        var i = page.glance.indexOf(id), j = i + d;
        if (i < 0 || j < 0 || j >= page.glance.length) return;
        var list = page.glance.slice();
        list[i] = list[j];
        list[j] = id;
        page.commitGlance(list);
    }

    function tapTile(id) {
        var p = page.places[id];
        if (!p) return;
        if (p.where === "spare") {
            page.addTile(id);
            return;
        }
        page.pick("tile", page.pickedKind === "tile" && page.picked === id ? "" : id);
    }
    function tapChip(id) {
        page.pick("chip", page.pickedKind === "chip" && page.picked === id ? "" : id);
    }

    // insertion index whose slot lands closest to (cx, cy); keeps `current` unless another is clearly closer
    function nearest(base, item, cx, cy, current) {
        var best = -1, bestD = Infinity, curD = Infinity;
        for (var i = 0; i <= base.length; i++) {
            var list = base.slice();
            list.splice(i, 0, item);
            var r = page.pack(list).map[item.id];
            var d = Math.hypot(r.x + r.w / 2 - cx, r.y + page.tileHeight / 2 - cy);
            if (d < bestD) {
                bestD = d;
                best = i;
            }
            if (i === current) curD = d;
        }
        return curD - bestD < 12 ? current : best;
    }
    function inside(item, x, y, pad) {
        return x >= item.x - pad && x <= item.x + item.width + pad && y >= item.y - pad && y <= item.y + item.height + pad;
    }

    function begin(kind, id, from, at, x, y) {
        page.pick("", "");
        page.moved = false;
        page.tracking = true;
        page.offX = at.x - x;
        page.offY = at.y - y;
        page.gx = x;
        page.gy = y;
        page.px = at.x;
        page.py = at.y;
        page.dragFrom = from;
        page.target = from;
        page.gridIndex = kind === "tile" && from === "grid" ? page.indexIn(page.tiles, id) : -1;
        page.chipIndex = kind === "chip" ? page.glance.indexOf(id) : -1;
        page.dragKind = kind;
        page.dragId = id;
        page.scrollAt = page.viewTop;
    }
    function liftTile(id, at, x, y) {
        var p = page.places[id];
        if (p) page.begin("tile", id, p.where, at, x, y);
    }
    function liftChip(id, at, x, y) {
        page.begin("chip", id, "chips", at, x, y);
    }
    function dragTo(at) {
        if (page.dragId === "") return;
        if (Math.abs(at.x - page.px) + Math.abs(at.y - page.py) > 0) page.moved = true;
        page.px = at.x;
        page.py = at.y;
        page.gx = at.x - page.offX;
        page.gy = at.y - page.offY;
        page.retarget();
    }
    function retarget() {
        var t = "";
        if (page.removable && page.inside(removeTarget, page.px, page.py, 4))
            t = "remove";
        else if (page.inside(chipGroup, page.px, page.py, 4) && page.py <= chipGroup.y + chipSeg.height + 8)
            t = "chips";
        else if (page.dragKind === "tile" && page.inside(gridBox, page.px, page.py, 16))
            t = "grid";
        else if (page.dragKind === "tile" && page.dragFrom === "grid" && page.py > gridBox.y + gridBox.height + 16)
            t = "spare";

        if (t === "grid") {
            var i = page.indexIn(page.tiles, page.dragId);
            var base = page.tiles.slice();
            var item = i >= 0 ? base.splice(i, 1)[0] : { id: page.dragId, large: false };
            var w = item.large ? 2 * page.unit + page.gap : page.unit;
            page.gridIndex = page.nearest(base, item, page.gx + w / 2 - gridBox.x, page.gy + page.tileHeight / 2 - gridBox.y,
                page.target === "grid" ? page.gridIndex : -1);
        } else if (t === "chips") {
            var count = page.glance.filter(function (x) { return x !== page.dragId; }).length;
            var k = Math.round((page.px - page.chipRowX - page.chipSize / 2) / (page.chipSize + page.chipGap));
            page.chipIndex = Math.max(0, Math.min(k, count, page.glanceMax - 1));
        }
        page.target = t;
    }
    function drop(at) {
        if (page.dragId === "") return;
        page.dragTo(at);
        page.tracking = false;
        var id = page.dragId, t = page.target;
        if (page.dragKind === "tile") {
            if (t === "grid" && !page.same(page.gridView, page.tiles))
                page.commit(page.gridView);
            else if ((t === "remove" || t === "spare") && page.dragFrom === "grid")
                page.removeTile(id);
            else if (t === "chips")
                page.addChip(id, page.chipIndex);
        } else if (t === "chips" && !page.same(page.chipView.slice(0, page.glanceMax), page.glance)) {
            page.commitGlance(page.chipView);
        } else if (t === "remove") {
            page.removeChip(id);
        }
        var kind = page.dragKind, from = page.dragFrom, still = !page.moved;
        page.end();
        // a long press without a move picks it, like a tap
        if (still && kind === "tile" && from === "grid") page.pick("tile", id);
        else if (still && kind === "chip") page.pick("chip", id);
    }
    function cancelDrag() {
        page.tracking = false;
        page.end();
    }
    function end() {
        page.dragId = "";
        page.dragKind = "";
        page.dragFrom = "";
        page.target = "";
        page.gridIndex = -1;
        page.chipIndex = -1;
    }

    function beginSize(id, x) {
        var p = page.places[id];
        if (!p) return;
        page.sizeId = id;
        page.sizeFrom = x;
        page.sizeStart = p.w;
        page.sizeW = p.w;
        page.moved = false;
        page.sizeTracking = true;
        page.sizing = true;
    }
    function sizeTo(x) {
        if (!page.sizing) return;
        if (Math.abs(x - page.sizeFrom) > 4) page.moved = true;
        page.sizeW = Math.max(page.unit, Math.min(2 * page.unit + page.gap, page.sizeStart + x - page.sizeFrom));
    }
    function endSize(commit) {
        if (!page.sizing) return;
        page.sizeTracking = false;
        if (commit) {
            var i = page.indexIn(page.tiles, page.sizeId);
            if (i >= 0)
                page.setSize(page.sizeId, page.moved ? page.sizeW > page.mid : !page.tiles[i].large);
        }
        page.sizing = false;
    }

    // the lifted thing stays under a still pointer while the page scrolls under it
    property real scrollAt: 0
    onViewTopChanged: {
        var d = page.viewTop - page.scrollAt;
        page.scrollAt = page.viewTop;
        if (!page.tracking || d === 0) return;
        page.py += d;
        page.gy += d;
        // a grid that grows under the drag scrolls the page mid-layout, so aim once it settles
        Qt.callLater(page.retarget);
    }
    Timer {
        interval: 16
        repeat: true
        running: page.tracking && page.moved && page.scroller !== null
        onTriggered: {
            var s = page.scroller;
            var top = page.viewTop + (page.removable ? removeTarget.height + 8 : 0);
            var bottom = page.viewTop + page.viewHeight;
            var d = page.py < top + 40 ? -Math.min(12, (top + 40 - page.py) / 3)
                : page.py > bottom - 40 ? Math.min(12, (page.py - bottom + 40) / 3) : 0;
            if (d === 0 || (d < 0 && page.py < top)) return;
            s.contentY = Math.max(0, Math.min(s.contentHeight - s.height, s.contentY + d));
        }
    }

    Keys.onPressed: function (event) {
        if (page.picked === "" || page.dragId !== "") return;
        var id = page.picked, tile = page.pickedKind === "tile";
        var k = event.key;
        if (k === Qt.Key_Escape) {
            page.pick("", "");
        } else if (k === Qt.Key_Delete || k === Qt.Key_Backspace) {
            if (tile) page.removeTile(id);
            else page.removeChip(id);
        } else if (k === Qt.Key_Left || k === Qt.Key_Right) {
            var d = k === Qt.Key_Left ? -1 : 1;
            if (tile) page.moveTile(id, d);
            else page.moveChip(id, d);
        } else if ((k === Qt.Key_Up || k === Qt.Key_Down) && tile) {
            page.moveTileRow(id, k === Qt.Key_Up ? -1 : 1);
        } else if ((k === Qt.Key_Return || k === Qt.Key_Enter || k === Qt.Key_Space) && tile) {
            var i = page.indexIn(page.tiles, id);
            if (i >= 0) page.setSize(id, !page.tiles[i].large);
        } else {
            return;
        }
        event.accepted = true;
    }

    MouseArea {
        anchors.fill: parent
        z: -1
        onClicked: page.pick("", "")
    }

    Column {
        id: sections
        width: parent.width
        spacing: 8

        TopBar {
            width: parent.width
            title: I18n.tr("Edit")
            onBack: page.host.back()

            Button {
                anchors.verticalCenter: parent.verticalCenter
                variant: "text"
                text: I18n.tr("Reset")
                onClicked: {
                    page.pick("", "");
                    page.commit(cfg.defaultTiles);
                    page.commitGlance(cfg.defaultGlanceTiles);
                }
            }
        }

        SectionLabel { text: I18n.tr("On hover") }

        Label {
            width: parent.width
            leftPadding: 16
            rightPadding: 16
            role: "caption"
            color: Md.inkMuted
            wrapMode: Text.Wrap
            text: page.pickedKind === "chip"
                ? I18n.tr("Drag it to move it, or tap the badge to remove it.")
                : I18n.tr("Drag tiles onto the row to show them when you hover the island.")
        }

        Column {
            id: chipGroup
            width: parent.width
            spacing: Md.seam

            Segment {
                id: chipSeg
                position: "first"
                height: 80

                Repeater {
                    model: page.glanceMax
                    delegate: Shape {
                        id: slot
                        required property int index
                        x: page.chipRowX - chipSeg.x + slot.index * (page.chipSize + page.chipGap)
                        y: (chipSeg.height - page.chipSize) / 2
                        width: page.chipSize
                        height: page.chipSize
                        preferredRendererType: Shape.CurveRenderer
                        // empty slots, and the one a lifted chip left behind
                        opacity: slot.index >= page.chipView.length
                            || (page.dragKind === "chip" && page.chipView[slot.index] === page.dragId) ? 1 : 0
                        visible: opacity > 0.01
                        Behavior on opacity {
                            enabled: !Md.still
                            NumberAnimation { duration: Md.fast }
                        }

                        ShapePath {
                            strokeColor: Md.outline
                            strokeWidth: 2
                            strokeStyle: ShapePath.DashLine
                            dashPattern: [3, 3]
                            fillColor: "transparent"
                            PathAngleArc {
                                centerX: page.chipSize / 2
                                centerY: page.chipSize / 2
                                radiusX: page.chipSize / 2 - 1
                                radiusY: page.chipSize / 2 - 1
                                startAngle: 0
                                sweepAngle: 360
                            }
                        }
                    }
                }
            }
            ListItem {
                width: parent.width
                position: "middle"
                icon: "partly_cloudy_day"
                headline: I18n.tr("Show weather")
                onClicked: cfg.set("glanceWeather", !cfg.glanceWeather)

                Switch {
                    checked: cfg.glanceWeather
                    onToggled: (v) => cfg.set("glanceWeather", v)
                }
            }
            ListItem {
                width: parent.width
                position: "last"
                icon: "battery_full"
                headline: I18n.tr("Show battery")
                onClicked: cfg.set("glanceBattery", !cfg.glanceBattery)

                Switch {
                    checked: cfg.glanceBattery
                    onToggled: (v) => cfg.set("glanceBattery", v)
                }
            }
        }

        SectionLabel { text: I18n.tr("Tiles") }

        Label {
            width: parent.width
            leftPadding: 16
            rightPadding: 16
            role: "caption"
            color: Md.inkMuted
            wrapMode: Text.Wrap
            text: page.pickedKind === "tile"
                ? I18n.tr("Pull the handle to resize it. Delete removes it, arrow keys move it.")
                : I18n.tr("Hold a tile to move it, tap it to resize it.")
        }

        Item {
            id: gridBox
            width: parent.width
            height: Math.max(page.tileHeight, page.gridPack.rows * (page.tileHeight + page.gap) - page.gap)
            Behavior on height {
                enabled: page.ready && !Md.still
                Spring { speed: "fast" }
            }
        }

        SectionLabel {
            visible: page.spareView.length > 0
            text: I18n.tr("Available tiles")
        }

        Item {
            id: spareBox
            width: parent.width
            visible: page.spareView.length > 0
            height: Math.ceil(page.spareView.length / page.columns) * (page.tileHeight + page.gap) - page.gap
        }

        Item { width: 1; height: 4 }
    }

    Rectangle {
        id: hole
        readonly property var at: page.dragKind === "tile" ? page.places[page.dragId] : undefined
        // it fades out where it last stood instead of sliding home
        property var spot: ({ x: 0, y: 0, w: 0 })
        onAtChanged: if (hole.at) hole.spot = hole.at
        x: hole.spot.x
        y: hole.spot.y
        width: hole.spot.w
        height: page.tileHeight
        radius: Md.rXl
        color: Md.alpha(Md.ink, 0.04)
        border.width: 2
        border.color: Md.outlineSoft
        opacity: hole.at ? 1 : 0
        visible: opacity > 0.01
        Behavior on x {
            enabled: !Md.still && hole.visible
            Spring {}
        }
        Behavior on y {
            enabled: !Md.still && hole.visible
            Spring {}
        }
        Behavior on opacity {
            enabled: !Md.still
            NumberAnimation { duration: Md.fast }
        }
    }

    Repeater {
        model: Catalogue.ids
        delegate: E.EditTile {
            required property string modelData
            editor: page
            tileId: modelData
        }
    }

    Repeater {
        model: Catalogue.ids
        delegate: E.EditChip {
            required property string modelData
            editor: page
            tileId: modelData
        }
    }

    Rectangle {
        id: removeTarget
        readonly property bool hot: page.target === "remove" && !page.removeBlocked
        x: 0
        property real slide: page.removable ? 0 : -12
        y: page.viewTop + 4 + removeTarget.slide
        z: 8
        width: page.width
        height: 56
        radius: height / 2
        color: removeTarget.hot ? Md.error : Md.containerHighest
        opacity: page.removable ? 1 : 0
        visible: opacity > 0.01
        Behavior on slide {
            enabled: !Md.still
            Spring { speed: "fast" }
        }
        Behavior on opacity {
            enabled: !Md.still
            NumberAnimation { duration: Md.fast }
        }
        Behavior on color {
            enabled: !Md.still
            ColorAnimation { duration: Md.fast }
        }

        Row {
            anchors.centerIn: parent
            spacing: 10

            Icon {
                anchors.verticalCenter: parent.verticalCenter
                name: page.removeBlocked ? "block" : "delete"
                size: 22
                filled: removeTarget.hot
                color: removeTarget.hot ? Md.onError : Md.ink
            }
            Label {
                anchors.verticalCenter: parent.verticalCenter
                role: "title"
                font.pixelSize: 15
                color: removeTarget.hot ? Md.onError : Md.ink
                text: page.removeBlocked ? I18n.tr("Keep at least one tile") : I18n.tr("Drag here to remove")
            }
        }
    }
}
