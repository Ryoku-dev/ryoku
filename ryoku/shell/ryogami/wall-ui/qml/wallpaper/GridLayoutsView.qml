import QtQuick
import QtQuick.Effects
import ".."
import "../services"

// Custom-grid carousel for the wallpaper picker. One component drives all six
// v2 arrangements (uniform, brick, masonry, justified, editorial, cylinder):
// each layout is a pure geometry pass into a shared content array, then the
// visible slice is windowed into a Repeater so a many-thousand catalogue never
// materialises more cards than the viewport can show at once.
Item {
    id: root

    // ── shared picker contract ───────────────────────────────────────────
    property var model: null
    property var colors: null
    property bool active: false
    property int selectedIndex: 0
    signal selectionChanged(int index)
    signal itemActivated(var item)

    // ── layout controls (bound by the parent) ────────────────────────────
    property string layoutKind: "uniform"
    property int columns: 4
    property int rows: 3
    property real thumbWidth: 320
    property real thumbHeight: 200
    property real stagger: 0.5
    property real selectedScale: 1.16
    property real flowWave: 46
    property real flowFrequency: 0.7
    property real scatter: 0.0
    property real scaleVariance: 0.28
    property real cylinderBend: 0.6
    property real cylinderRadius: 900

    clip: true
    focus: active

    readonly property int _gap: Style.spacingLarge
    readonly property int _maxCells: 2000
    readonly property int _maxVisible: 96
    readonly property real _aspect: thumbHeight / Math.max(1, thumbWidth)
    readonly property bool _isCylinder: layoutKind === "cylinder"

    property var _cells: []
    property var _visible: []
    property real _contentW: width
    property real _contentH: height

    property int _count: model ? Math.min(model.count, _maxCells) : 0

    // Any control that reshapes the arrangement funnels through one signature so
    // a single handler schedules the rebuild instead of a dozen onXChanged hooks.
    readonly property var _signature: [layoutKind, columns, rows, thumbWidth, thumbHeight,
        stagger, flowWave, flowFrequency, scatter, scaleVariance,
        cylinderRadius, width, height, _count]
    on_SignatureChanged: _scheduleRebuild()

    onActiveChanged: {
        if (active) {
            _clampSelection()
            _scheduleRebuild()
            forceActiveFocus()
        }
    }
    onModelChanged: { _clampSelection(); _scheduleRebuild() }

    Connections {
        target: root.model
        function onCountChanged() {
            root._clampSelection()
            root._scheduleRebuild()
        }
    }

    Timer {
        id: _rebuildTimer
        interval: 0
        repeat: false
        onTriggered: root._rebuild()
    }
    function _scheduleRebuild() { if (root.active) _rebuildTimer.restart() }

    // Deterministic hash → [0,1). Same index always yields the same jitter so a
    // rebuild (resize, filter) never reshuffles scatter or size variance.
    function _rand(i, salt) {
        var x = (Math.imul(i + 1, 374761393) + Math.imul(salt + 1, 668265263)) | 0
        x = Math.imul(x ^ (x >>> 13), 1274126177)
        x = (x ^ (x >>> 16)) >>> 0
        return x / 4294967296
    }
    function _signed(i, salt) { return _rand(i, salt) * 2 - 1 }

    function _clampSelection() {
        if (!model) return
        var n = model.count
        if (n <= 0) { selectedIndex = 0; return }
        if (selectedIndex >= n) selectedIndex = n - 1
        if (selectedIndex < 0) selectedIndex = 0
    }

    function _thumbFor(it) {
        if (!it) return ""
        var p = it.thumbnail || it.thumb || it.path || ""
        return p.length > 0 ? ImageService.fileUrl(p) : ""
    }

    // ── geometry ─────────────────────────────────────────────────────────
    function _rebuild() {
        if (!active || width <= 0 || height <= 0 || _count <= 0) {
            _cells = []
            _visible = []
            return
        }
        switch (layoutKind) {
        case "brick":     _buildBrick(); break
        case "masonry":   _buildMasonry(); break
        case "justified": _buildJustified(); break
        case "editorial": _buildEditorial(); break
        case "cylinder":  _buildCylinder(); break
        default:          _buildUniform(); break
        }
        _recomputeWindow()
    }

    function _buildUniform() {
        var W = width, gap = _gap, cols = Math.max(1, columns)
        var colW = (W - gap * (cols + 1)) / cols
        var rowH = colW * _aspect
        var cells = []
        for (var i = 0; i < _count; i++) {
            var c = i % cols, r = Math.floor(i / cols)
            var shrink = 1 - _rand(i, 1) * scaleVariance * 0.5
            var w = colW * shrink, h = rowH * shrink
            var slotX = gap + c * (colW + gap)
            var slotY = gap + r * (rowH + gap)
            var jx = _signed(i, 2) * scatter * colW * 0.22
            var jy = _signed(i, 3) * scatter * rowH * 0.22
            var x = slotX + (colW - w) / 2 + jx
            var y = slotY + (rowH - h) / 2 + jy
            cells.push({ index: i, x: x, y: y, w: w, h: h, cx: x + w / 2, cy: y + h / 2 })
        }
        _cells = cells
        _contentW = W
        _contentH = gap + Math.ceil(_count / cols) * (rowH + gap)
    }

    // Running-bond wall: fixed-height courses of near-full-width bricks whose
    // odd rows lead with a fractional brick so the vertical seams never line up.
    function _buildBrick() {
        var W = width, gap = _gap, cols = Math.max(1, columns)
        var unit = (W - gap * (cols + 1)) / cols
        var rowH = unit * _aspect
        var cells = []
        var i = 0, rowIdx = 0, y = gap
        while (i < _count) {
            var x = gap, first = true
            while (x < W - gap * 0.5 && i < _count) {
                var bw = unit * (1 + _signed(i, 7) * scaleVariance * 0.3)
                if (first && (rowIdx % 2) === 1) bw = unit * Math.max(0.25, stagger)
                if (x + bw + gap > W) bw = W - gap - x
                if (bw < unit * 0.2) { bw = W - gap - x }
                cells.push({ index: i, x: x, y: y, w: bw, h: rowH, cx: x + bw / 2, cy: y + rowH / 2 })
                x += bw + gap
                i++; first = false
            }
            y += rowH + gap; rowIdx++
        }
        _cells = cells
        _contentW = W
        _contentH = y
    }

    // Column skyline: every card keeps the column width but takes a varied height,
    // dropping into whichever column is currently shortest.
    function _buildMasonry() {
        var W = width, gap = _gap, cols = Math.max(1, columns)
        var colW = (W - gap * (cols + 1)) / cols
        var baseH = colW * _aspect
        var top = []
        for (var c = 0; c < cols; c++) top.push(gap + _rand(c, 11) * stagger * baseH)
        var cells = []
        for (var i = 0; i < _count; i++) {
            var pick = 0
            for (var k = 1; k < cols; k++) if (top[k] < top[pick] - 0.5) pick = k
            var h = baseH * (1 + _signed(i, 13) * scaleVariance)
            h = Math.max(baseH * 0.55, h)
            var x = gap + pick * (colW + gap)
            var y = top[pick]
            cells.push({ index: i, x: x, y: y, w: colW, h: h, cx: x + colW / 2, cy: y + h / 2 })
            top[pick] = y + h + gap
        }
        var maxTop = gap
        for (var m = 0; m < cols; m++) maxTop = Math.max(maxTop, top[m])
        _cells = cells
        _contentW = W
        _contentH = maxTop
    }

    // Flickr-style justified rows: greedily gather cards with varied intrinsic
    // aspect ratios, then solve the row height that makes the run span the width.
    function _buildJustified() {
        var W = width, gap = _gap
        var targetH = ((W - gap * (Math.max(1, columns) + 1)) / Math.max(1, columns)) * _aspect
        var baseAr = thumbWidth / Math.max(1, thumbHeight)
        var cells = []
        var i = 0, y = gap
        while (i < _count) {
            var run = [], sumAr = 0
            while (i < _count) {
                var ar = baseAr * (1 + _signed(i, 17) * scaleVariance)
                ar = Math.max(0.5, ar)
                run.push({ index: i, ar: ar })
                sumAr += ar
                i++
                if (sumAr * targetH + gap * (run.length + 1) >= W) break
            }
            var h = (W - gap * (run.length + 1)) / sumAr
            var lastRow = i >= _count
            if (lastRow && h > targetH * 1.35) h = targetH
            var x = gap
            for (var r = 0; r < run.length; r++) {
                var w = h * run[r].ar
                cells.push({ index: run[r].index, x: x, y: y, w: w, h: h, cx: x + w / 2, cy: y + h / 2 })
                x += w + gap
            }
            y += h + gap
        }
        _cells = cells
        _contentW = W
        _contentH = y
    }

    // Magazine grid: a skyline packer that promotes occasional two-column hero
    // tiles, then rides the whole thing on a sine flow so rows breathe.
    function _buildEditorial() {
        var W = width, gap = _gap, cols = Math.max(2, columns)
        var colW = (W - gap * (cols + 1)) / cols
        var baseH = colW * _aspect
        var top = []
        for (var c = 0; c < cols; c++) top.push(gap)
        var cells = []
        var freq = flowFrequency / 340
        for (var i = 0; i < _count; i++) {
            var pick = 0
            for (var k = 1; k < cols; k++) if (top[k] < top[pick] - 0.5) pick = k
            var span = 1
            if (pick <= cols - 2 && Math.abs(top[pick] - top[pick + 1]) < 1 && _rand(i, 19) < 0.22)
                span = 2
            var w = span * colW + (span - 1) * gap
            var h = baseH * (span > 1 ? 1.4 : 1) * (1 + _signed(i, 23) * scaleVariance * 0.4)
            var baseX = gap + pick * (colW + gap)
            var baseY = top[pick]
            var wave = flowWave * Math.sin((baseX + baseY) * freq)
            var jx = _signed(i, 29) * scatter * colW * 0.24
            var x = baseX + jx
            var y = baseY + wave
            cells.push({ index: i, x: x, y: y, w: w, h: h, cx: x + w / 2, cy: y + h / 2 })
            for (var s = pick; s < pick + span; s++) top[s] = baseY + h + gap
        }
        var maxTop = gap
        for (var m = 0; m < cols; m++) maxTop = Math.max(maxTop, top[m])
        _cells = cells
        _contentW = W
        _contentH = maxTop + Math.abs(flowWave) + gap
    }

    // Curved carousel: straight base strip whose per-card curvature (scale,
    // rotation, downward bow) is resolved live against the viewport centre so
    // the arc tracks the scroll instead of freezing at build time.
    function _buildCylinder() {
        var W = width, H = height
        var shelves = Math.max(1, rows)
        var perShelf = Math.ceil(_count / shelves)
        var slot = W / Math.max(2, columns)
        var bandH = H / shelves
        var tileW = slot * 0.82
        var tileH = Math.min(bandH * 0.78, tileW * _aspect)
        var cells = []
        for (var i = 0; i < _count; i++) {
            var s = Math.floor(i / perShelf)
            var k = i - s * perShelf
            var cx = slot * (k + 1)
            var cy = (s + 0.5) * bandH
            var x = cx - tileW / 2
            var y = cy - tileH / 2
            cells.push({ index: i, x: x, y: y, w: tileW, h: tileH, cx: cx, cy: cy })
        }
        _cells = cells
        _contentW = slot * (perShelf + 1)
        _contentH = H
    }

    function _cylTransform(baseCx) {
        var viewCx = flick.contentX + flick.width / 2
        var t = (baseCx - viewCx) / Math.max(1, cylinderRadius)
        t = Math.max(-1.3, Math.min(1.3, t))
        var depth = 1 - Math.cos(t)
        var scale = Math.max(0.4, 1 - depth * 0.62)
        var rot = -t * 40 * cylinderBend
        var bow = cylinderBend * 150 * depth
        var xShift = Math.sin(t) * cylinderRadius - (baseCx - viewCx)
        return { dx: xShift, dy: bow, scale: scale, rot: rot, depth: depth }
    }

    // ── windowing ────────────────────────────────────────────────────────
    function _recomputeWindow() {
        var cells = _cells
        if (!cells || cells.length === 0) { _visible = []; return }
        var out = []
        if (_isCylinder) {
            var lo = flick.contentX - flick.width
            var hi = flick.contentX + flick.width * 2
            for (var i = 0; i < cells.length; i++) {
                var c = cells[i]
                if (c.x + c.w >= lo && c.x <= hi) out.push(c)
                if (out.length >= _maxVisible) break
            }
        } else {
            var loy = flick.contentY - flick.height * 0.5
            var hiy = flick.contentY + flick.height * 1.5
            for (var j = 0; j < cells.length; j++) {
                var d = cells[j]
                if (d.y + d.h >= loy && d.y <= hiy) out.push(d)
                if (out.length >= _maxVisible) break
            }
        }
        _visible = out
    }

    function _cellByIndex(idx) {
        for (var i = 0; i < _cells.length; i++) if (_cells[i].index === idx) return _cells[i]
        return null
    }

    // ── interaction ──────────────────────────────────────────────────────
    function _select(i) {
        if (!model) return
        var n = model.count
        if (n <= 0) return
        i = Math.max(0, Math.min(n - 1, i))
        if (i !== selectedIndex) {
            selectedIndex = i
            selectionChanged(i)
        }
        _ensureVisible(i)
        forceActiveFocus()
    }

    function _activate(i) {
        if (!model) return
        var it = model.get(i)
        if (it) itemActivated(it)
    }

    function _ensureVisible(i) {
        var c = _cellByIndex(i)
        if (!c) return
        if (_isCylinder) {
            var target = c.cx - flick.width / 2
            flick.contentX = Math.max(0, Math.min(Math.max(0, _contentW - flick.width), target))
        } else {
            if (c.y < flick.contentY)
                flick.contentY = Math.max(0, c.y - _gap)
            else if (c.y + c.h > flick.contentY + flick.height)
                flick.contentY = Math.min(Math.max(0, _contentH - flick.height), c.y + c.h - flick.height + _gap)
        }
    }

    // Spatial navigation works uniformly across every layout: pick the nearest
    // card in the requested direction, weighting cross-axis drift so arrows feel
    // natural even on masonry and the cylinder.
    function _navigate(dir) {
        if (_cells.length === 0) return
        var cur = _cellByIndex(selectedIndex)
        if (!cur) { _select(_cells[0].index); return }
        var best = -1, bestScore = Infinity
        for (var i = 0; i < _cells.length; i++) {
            var c = _cells[i]
            if (c.index === selectedIndex) continue
            var dx = c.cx - cur.cx, dy = c.cy - cur.cy
            var primary, cross
            if (dir === "r") { if (dx <= 1) continue; primary = dx; cross = Math.abs(dy) }
            else if (dir === "l") { if (dx >= -1) continue; primary = -dx; cross = Math.abs(dy) }
            else if (dir === "d") { if (dy <= 1) continue; primary = dy; cross = Math.abs(dx) }
            else { if (dy >= -1) continue; primary = -dy; cross = Math.abs(dx) }
            var score = primary + cross * 2.4
            if (score < bestScore) { bestScore = score; best = c.index }
        }
        if (best >= 0) _select(best)
    }

    Keys.onPressed: function (e) {
        if (!active || _count === 0) return
        switch (e.key) {
        case Qt.Key_Left:  _navigate("l"); e.accepted = true; break
        case Qt.Key_Right: _navigate("r"); e.accepted = true; break
        case Qt.Key_Up:    _navigate("u"); e.accepted = true; break
        case Qt.Key_Down:  _navigate("d"); e.accepted = true; break
        case Qt.Key_Home:  _select(0); e.accepted = true; break
        case Qt.Key_End:   _select(model ? model.count - 1 : 0); e.accepted = true; break
        case Qt.Key_Return:
        case Qt.Key_Enter:
        case Qt.Key_Space: _activate(selectedIndex); e.accepted = true; break
        }
    }

    Flickable {
        id: flick
        anchors.fill: parent
        clip: true
        contentWidth: root._contentW
        contentHeight: root._contentH
        interactive: true
        boundsBehavior: Flickable.StopAtBounds
        flickableDirection: root._isCylinder ? Flickable.HorizontalFlick : Flickable.VerticalFlick
        maximumFlickVelocity: 4000

        onContentYChanged: root._recomputeWindow()
        onContentXChanged: root._recomputeWindow()

        WheelHandler {
            acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
            onWheel: function (ev) {
                var step = (ev.angleDelta.y !== 0 ? ev.angleDelta.y : ev.angleDelta.x)
                if (root._isCylinder) {
                    var nx = flick.contentX - step
                    flick.contentX = Math.max(0, Math.min(Math.max(0, root._contentW - flick.width), nx))
                } else {
                    var ny = flick.contentY - step
                    flick.contentY = Math.max(0, Math.min(Math.max(0, root._contentH - flick.height), ny))
                }
            }
        }

        Repeater {
            model: root._visible

            delegate: Item {
                id: card
                required property var modelData
                required property int index

                readonly property var cell: modelData
                readonly property int itemIndex: cell.index
                readonly property var itemData: root.model ? root.model.get(itemIndex) : null
                readonly property bool selected: itemIndex === root.selectedIndex
                readonly property bool hovered: hover.containsMouse
                readonly property var cyl: root._isCylinder ? root._cylTransform(cell.cx) : null

                x: cell.x + (cyl ? cyl.dx : 0)
                y: cell.y + (cyl ? cyl.dy : 0)
                width: cell.w
                height: cell.h
                transformOrigin: Item.Center
                rotation: cyl ? cyl.rot : 0
                scale: (selected ? root.selectedScale : (hovered ? 1.04 : 1)) * (cyl ? cyl.scale : 1)
                z: selected ? 1000 : (cyl ? Math.round(200 - cyl.depth * 180) : (hovered ? 20 : 1))
                opacity: cyl ? Math.max(0.35, 1 - cyl.depth * 0.5) : 1

                Behavior on scale { NumberAnimation { duration: Style.animFast; easing.type: Easing.OutCubic } }

                Item {
                    id: roundMask
                    anchors.fill: parent
                    visible: false
                    layer.enabled: true
                    Rectangle {
                        anchors.fill: parent
                        radius: Style.radiusLarge
                        color: "white"
                        antialiasing: true
                    }
                }

                Item {
                    id: frame
                    anchors.fill: parent
                    layer.enabled: true
                    layer.smooth: true
                    layer.effect: MultiEffect {
                        maskEnabled: true
                        maskSource: roundMask
                        maskThresholdMin: 0.5
                    }

                    Rectangle {
                        anchors.fill: parent
                        color: root.colors ? Qt.rgba(root.colors.surfaceContainer.r, root.colors.surfaceContainer.g,
                                                      root.colors.surfaceContainer.b, 1)
                                           : Qt.rgba(0.09, 0.10, 0.13, 1)
                    }

                    Image {
                        id: thumb
                        anchors.fill: parent
                        fillMode: Image.PreserveAspectCrop
                        cache: true
                        asynchronous: true
                        smooth: true
                        source: root._thumbFor(card.itemData)
                        sourceSize.width: ImageService.thumbWidth
                        sourceSize.height: ImageService.thumbHeight
                        opacity: status === Image.Ready ? 1 : 0
                        Behavior on opacity { NumberAnimation { duration: Style.animNormal; easing.type: Easing.OutCubic } }
                    }
                }

                Rectangle {
                    anchors.fill: parent
                    radius: Style.radiusLarge
                    color: "transparent"
                    antialiasing: true
                    border.width: card.selected ? Style.borderThick : (card.hovered ? Style.borderMedium : Style.borderThin)
                    border.color: card.selected
                        ? (root.colors ? root.colors.primary : Style.fallbackAccent)
                        : (card.hovered
                            ? (root.colors ? Qt.rgba(root.colors.primary.r, root.colors.primary.g, root.colors.primary.b, 0.65)
                                           : Qt.rgba(1, 1, 1, 0.5))
                            : Qt.rgba(0, 0, 0, 0.45))
                    Behavior on border.color { ColorAnimation { duration: Style.animFast } }
                }

                Rectangle {
                    anchors.right: parent.right
                    anchors.bottom: parent.bottom
                    anchors.margins: 8
                    width: typeLabel.implicitWidth + 12
                    height: 18
                    radius: 9
                    color: Qt.rgba(0, 0, 0, 0.75)
                    border.width: 1
                    border.color: root.colors ? Qt.rgba(root.colors.primary.r, root.colors.primary.g, root.colors.primary.b, 0.45)
                                              : Qt.rgba(1, 1, 1, 0.25)
                    visible: card.itemData !== null

                    Text {
                        id: typeLabel
                        anchors.centerIn: parent
                        text: {
                            var it = card.itemData
                            if (!it) return ""
                            if (it.type === "static") return "PIC"
                            if (it.type === "video" || it.videoFile) return "VID"
                            return "WE"
                        }
                        font.family: Style.fontFamily
                        font.pixelSize: Style.fontTiny
                        font.weight: Font.Bold
                        font.letterSpacing: 0.5
                        color: root.colors ? root.colors.tertiary : "#8bceff"
                    }
                }

                Text {
                    anchors.left: parent.left
                    anchors.bottom: parent.bottom
                    anchors.margins: 7
                    text: "\u2605"
                    font.pixelSize: 14
                    style: Text.Outline
                    styleColor: Qt.rgba(0, 0, 0, 0.7)
                    color: root.colors ? root.colors.primary : Style.fallbackAccent
                    visible: card.itemData && card.itemData.favourite === true
                }

                MouseArea {
                    id: hover
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    acceptedButtons: Qt.LeftButton
                    onClicked: root._select(card.itemIndex)
                    onDoubleClicked: {
                        root._select(card.itemIndex)
                        root._activate(card.itemIndex)
                    }
                }
            }
        }
    }

    Text {
        anchors.centerIn: parent
        visible: root.active && root._count === 0
        text: "No wallpapers to show"
        font.family: Style.fontFamily
        font.pixelSize: Style.fontSubtitle
        color: root.colors ? Qt.rgba(root.colors.surfaceText.r, root.colors.surfaceText.g, root.colors.surfaceText.b, 0.6)
                           : Qt.rgba(1, 1, 1, 0.5)
    }

    Component.onCompleted: if (active) _rebuild()
}
