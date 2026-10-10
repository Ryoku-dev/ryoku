pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import shell.services

// The island row itself: pinned launchers in user order first, then the
// running-unpinned apps in pid order, one separator between the two runs when
// both exist, and an optional media chip at the end. This is exactly
// Dock.resolve(pinned, clients) split at the pin boundary, so a pinned app that
// starts running keeps its place instead of jumping into the running run -- the
// flaw in the retired DockRow this rewrite fixes.
//
// Two diffed ListModels (one per run) drive the Repeaters, so only a genuinely
// new island pops in and every other island slides to its new spot, rather than
// the whole row being rebuilt on each toplevel event. Layout is fixed at rest;
// magnify overflows the band as a pure transform, and the drag ghost + live swap
// ride on top. Every animation is gated on reduce-motion.
Item {
    id: band

    // ── surface-provided context ─────────────────────────────────────────────
    required property string edge
    readonly property bool horizontal: band.edge === "top" || band.edge === "bottom"
    // Depth (band + edge gap) a hovering dock reserves, so the preview strip can
    // clear it; the surface knows the gap, so it feeds this in.
    property real reservedDepth: 0

    // ── sizing + look knobs ──────────────────────────────────────────────────
    readonly property real baseSize: Math.max(28, Math.min(72, Number(Dock.cfg("size", 44)))) + 2
    readonly property real iconSize: Math.max(18, baseSize - 16)
    readonly property real gap: Math.max(10, Math.round(baseSize * 0.35))
    readonly property real sepWidth: 15
    readonly property real radius: Theme.radiusWidget + 4
    readonly property real maxScale: 1.4
    readonly property real reach: (baseSize + gap) * 1.9
    readonly property real step: baseSize + gap

    readonly property bool magnify: Dock.cfg("magnify", true) && !Perf.reduceMotion
    readonly property bool animate: !Perf.reduceMotion
    readonly property bool frost: Dock.cfg("frost", true)
    readonly property bool shadow: Dock.cfg("shadow", true)
    readonly property bool labels: Dock.cfg("labels", true)

    // Persisted look. A style only changes what is DRAWN: the geometry below is
    // fixed, so autohide, the peek strip, the drag-reorder, the hover label and
    // the window preview hold in every form. DockItem/chrome read it via band.
    readonly property string style: Dock.cfg("style", "islands")

    // Hairline vocabulary hoisted so every style role reads a token, not a raw
    // alpha: `hairline` is the plate/cell border and `rule` the stronger divider
    // (the two weights already in this file), `inkFaint` the faintest ink for a
    // mono index or abbreviation, `bone`/`inkOnBone` the Material inversion pair
    // that stands in for the forbidden accent tint.
    readonly property color hairline: Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.12)
    readonly property color rule: Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.22)
    readonly property color inkFaint: Qt.rgba(Theme.onSurfaceVariant.r, Theme.onSurfaceVariant.g, Theme.onSurfaceVariant.b, 0.55)
    readonly property color bone: Theme.inverseSurface
    readonly property color inkOnBone: Theme.inverseOnSurface

    // ── the two runs ─────────────────────────────────────────────────────────
    // basePins is the persisted (or starter) order; workOrder is the live order a
    // drag edits before it commits. While dragging, the row follows workOrder.
    readonly property var basePins: Dock.pinnedOrStarter()
    property var workOrder: []
    readonly property var pins: band.dragging ? band.workOrder : band.basePins
    onBasePinsChanged: if (!band.dragging) band.workOrder = band.basePins.slice()

    readonly property var running: {
        const out = [], seen = {};
        const p = band.pins, cs = Dock.clients;
        for (let i = 0; i < cs.length; ++i) {
            const cn = cs[i].className;
            if (cn && !seen[cn] && p.indexOf(cn) === -1) { seen[cn] = true; out.push(cn); }
        }
        return out;
    }
    readonly property bool sepShown: band.pins.length > 0 && band.running.length > 0
    readonly property bool mediaShown: Dock.cfg("media", false) && Media.present

    // ── geometry (along the band; cross axis is always baseSize) ──────────────
    function pinStart(i) { return i * band.step; }
    readonly property real afterPins: band.pins.length > 0 ? band.pins.length * band.step : 0
    readonly property real runBase: band.afterPins + (band.sepShown ? band.sepWidth + band.gap : 0)
    function runStart(j) { return band.runBase + j * band.step; }
    readonly property real afterRun: band.running.length > 0 ? band.runBase + band.running.length * band.step : band.runBase
    readonly property real mediaStart: band.afterRun
    readonly property real mediaLen: band.mediaShown ? (band.horizontal ? mediaChip.implicitWidth : mediaChip.implicitHeight) : 0
    readonly property real totalSpan: {
        let e = 0;
        if (band.pins.length) e = Math.max(e, band.pins.length * band.step - band.gap);
        if (band.sepShown) e = Math.max(e, band.afterPins + band.sepWidth);
        if (band.running.length) e = Math.max(e, band.runBase + band.running.length * band.step - band.gap);
        if (band.mediaShown) e = Math.max(e, band.mediaStart + band.mediaLen);
        return Math.max(1, e);
    }

    // The app run alone (pins · separator · running), no media chip: the extent
    // the rail/ledger backdrop plate spans. The media chip keeps its own tile.
    readonly property real runSpan: {
        let e = 0;
        if (band.pins.length) e = Math.max(e, band.pins.length * band.step - band.gap);
        if (band.sepShown) e = Math.max(e, band.afterPins + band.sepWidth);
        if (band.running.length) e = Math.max(e, band.runBase + band.running.length * band.step - band.gap);
        return Math.max(1, e);
    }
    // Perp gap between the band's outer edge and the screen edge, so a tanzaku
    // strip can reach the edge. Derived (reservedDepth is depth+edgeGap and depth
    // is baseSize), so no surface change is needed to know the edge gap here.
    readonly property real edgeReach: Math.max(0, band.reservedDepth - band.baseSize)

    implicitWidth: band.horizontal ? band.totalSpan : band.baseSize
    implicitHeight: band.horizontal ? band.baseSize : band.totalSpan

    // ── cursor tracking (icons AND gaps, no dropout) ──────────────────────────
    property real cursorAlong: 0
    // The surface reads this to keep an autohiding dock revealed while hovered.
    readonly property bool hovered: bandHover.hovered
    HoverHandler {
        id: bandHover
        onPointChanged: band.cursorAlong = band.horizontal ? point.position.x : point.position.y
    }

    // ── drag-to-reorder (pinned islands only) ─────────────────────────────────
    property int dragIndex: -1
    property string dragClass: ""
    property point dragScene: Qt.point(0, 0)
    readonly property bool dragging: band.dragIndex >= 0

    function beginDrag(pinIndex, className) {
        if (pinIndex < 0)
            return;
        band.workOrder = band.basePins.slice();
        band.dragClass = className;
        band.dragIndex = pinIndex;
    }
    function dragMove(scene) {
        band.dragScene = scene;
        if (band.dragIndex < 0)
            return;
        const lp = band.mapFromItem(null, scene.x, scene.y);
        const along = band.horizontal ? lp.x : lp.y;
        const cur = band.dragIndex;
        let best = cur, bd = Infinity;
        for (let i = 0; i < band.workOrder.length; ++i) {
            if (i === cur)
                continue;
            const d = Math.abs(along - (band.pinStart(i) + band.baseSize / 2));
            if (d < bd) { bd = d; best = i; }
        }
        if (best === cur)
            return;
        // Swap only once the drag centroid has passed the neighbour's centre, so
        // an island trades places instead of flickering across the boundary.
        const nc = band.pinStart(best) + band.baseSize / 2;
        const crossed = best > cur ? (along >= nc) : (along <= nc);
        if (crossed) {
            const arr = band.workOrder.slice();
            const tmp = arr[cur]; arr[cur] = arr[best]; arr[best] = tmp;
            band.workOrder = arr;
            band.dragIndex = best;
        }
    }
    function endDrag() {
        if (band.dragIndex < 0)
            return;
        Dock.setPinned(band.workOrder.slice());
        band.dragIndex = -1;
        band.dragClass = "";
    }

    // ── context menu (right-click) ─────────────────────────────────────────────
    // The menu state and actions live on the Dock singleton (shared with the
    // per-monitor DockMenuOverlay). The band only forwards the owning screen so a
    // right-click on this monitor's dock opens the menu on this monitor.
    required property string screenName

    // ── diffed models: create ONLY genuinely new islands, slide the rest ───────
    ListModel { id: pinsModel }
    ListModel { id: runningModel }
    function syncModel(model, desired) {
        for (let i = model.count - 1; i >= 0; --i)
            if (desired.indexOf(model.get(i).className) === -1)
                model.remove(i);
        for (let j = 0; j < desired.length; ++j) {
            const cn = desired[j];
            let cur = -1;
            for (let k = 0; k < model.count; ++k)
                if (model.get(k).className === cn) { cur = k; break; }
            if (cur === -1)
                model.insert(j, { className: cn });
            else if (cur !== j)
                model.move(cur, j, 1);
        }
    }
    onPinsChanged: band.syncModel(pinsModel, band.pins)
    onRunningChanged: band.syncModel(runningModel, band.running)
    Component.onCompleted: {
        band.workOrder = band.basePins.slice();
        band.syncModel(pinsModel, band.pins);
        band.syncModel(runningModel, band.running);
    }

    // ── the shared hover label (one instance, moved between islands) ──────────
    // Nearest app island to the cursor, for the preview strip and the name label.
    readonly property var hoveredApp: {
        if (!bandHover.hovered || band.dragging)
            return null;
        // The chip drives its own card; keep it out of the nearest-app search.
        if (band.mediaShown && (mediaChip.chipHovered || band.cursorAlong >= band.mediaStart))
            return null;
        let best = null, bd = 1e9;
        for (let i = 0; i < band.pins.length; ++i) {
            const c = band.pinStart(i) + band.baseSize / 2, d = Math.abs(c - band.cursorAlong);
            if (d < bd) { bd = d; best = { cn: band.pins[i], center: c }; }
        }
        for (let j = 0; j < band.running.length; ++j) {
            const c = band.runStart(j) + band.baseSize / 2, d = Math.abs(c - band.cursorAlong);
            if (d < bd) { bd = d; best = { cn: band.running[j], center: c }; }
        }
        return best;
    }
    // A change handler on the object itself would fire every pointer move (a
    // fresh object each evaluation); key the label + preview off the class name,
    // which only changes when the pointer actually crosses to another island.
    readonly property string hoveredClass: band.hoveredApp ? band.hoveredApp.cn : ""
    property bool labelReady: false
    Timer { id: labelTimer; interval: 420; onTriggered: band.labelReady = true }
    onHoveredClassChanged: {
        band.syncPreview();
        if (band.labels && band.hoveredClass !== "")
            labelTimer.restart();
        else {
            labelTimer.stop();
            band.labelReady = false;
        }
    }

    // Keep feeding the existing DockPreview singleton so its live window-preview
    // strip still grows off the hovered icon, exactly as the retired DockRow did.
    function syncPreview() {
        if (!band.hoveredApp) {
            DockPreview.hoveredClass = "";
            return;
        }
        const cn = band.hoveredApp.cn;
        const cx = band.horizontal ? band.hoveredApp.center : band.baseSize / 2;
        const cy = band.horizontal ? band.baseSize / 2 : band.hoveredApp.center;
        const g = band.mapToGlobal(cx, cy);
        DockPreview.gx = g.x;
        DockPreview.gy = g.y;
        DockPreview.edge = band.edge;
        DockPreview.margin = band.reservedDepth + 14;
        DockPreview.hoveredClass = Dock.countFor(cn) > 0 ? cn : "";
    }

    // rail / ledger draw ONE plate behind the whole app run; the per-item marks
    // (underline, cell divider, index) stay with the item. Empty for the others.
    DockBackdrop {
        band: band
        z: -1
    }

    // ── pinned run ─────────────────────────────────────────────────────────
    Repeater {
        model: pinsModel
        delegate: Item {
            id: pw
            required property int index
            required property string className
            readonly property bool beingDragged: band.dragging && band.dragIndex === pw.index

            property bool ready: false
            Component.onCompleted: pw.ready = true
            width: band.baseSize
            height: band.baseSize
            x: band.horizontal ? band.pinStart(pw.index) : 0
            y: band.horizontal ? 0 : band.pinStart(pw.index)
            Behavior on x { enabled: band.animate && pw.ready && !pw.beingDragged; NumberAnimation { duration: Motion.rowReveal; easing.type: Motion.rowRevealCurve } }
            Behavior on y { enabled: band.animate && pw.ready && !pw.beingDragged; NumberAnimation { duration: Motion.rowReveal; easing.type: Motion.rowRevealCurve } }
            // The dragged island hides in place; the ghost carries it instead.
            opacity: pw.beingDragged ? 0 : 1
            Behavior on opacity { enabled: band.animate; NumberAnimation { duration: Motion.thumbHover } }

            DockItem {
                anchors.fill: parent
                band: band
                alongCenter: band.pinStart(pw.index) + band.baseSize / 2
                pinIndex: pw.index
                className: pw.className
                ordinal: pw.index
            }
        }
    }

    // A gap mark between the two runs, or a full-depth hairline on the continuous
    // rail/ledger sheets so the boundary reads as a ruled division, not a gap.
    Rectangle {
        readonly property bool full: band.style === "rail" || band.style === "ledger"
        visible: band.sepShown
        width: band.horizontal ? (full ? 1 : 2) : (full ? band.baseSize : Math.round(band.baseSize * 0.5))
        height: band.horizontal ? (full ? band.baseSize : Math.round(band.baseSize * 0.5)) : (full ? 1 : 2)
        radius: full ? 0 : 1
        color: band.rule
        x: band.horizontal ? (band.afterPins + (band.sepWidth - width) / 2) : (band.baseSize - width) / 2
        y: band.horizontal ? (band.baseSize - height) / 2 : (band.afterPins + (band.sepWidth - height) / 2)
    }

    // ── running-unpinned run ──────────────────────────────────────────────────
    Repeater {
        model: runningModel
        delegate: Item {
            id: rw
            required property int index
            required property string className

            property bool ready: false
            Component.onCompleted: rw.ready = true
            width: band.baseSize
            height: band.baseSize
            x: band.horizontal ? band.runStart(rw.index) : 0
            y: band.horizontal ? 0 : band.runStart(rw.index)
            Behavior on x { enabled: band.animate && rw.ready; NumberAnimation { duration: Motion.rowReveal; easing.type: Motion.rowRevealCurve } }
            Behavior on y { enabled: band.animate && rw.ready; NumberAnimation { duration: Motion.rowReveal; easing.type: Motion.rowRevealCurve } }

            DockItem {
                anchors.fill: parent
                band: band
                alongCenter: band.runStart(rw.index) + band.baseSize / 2
                pinIndex: -1
                className: rw.className
                ordinal: band.pins.length + rw.index
            }
        }
    }

    // ── media chip ────────────────────────────────────────────────────────────
    DockMedia {
        id: mediaChip
        band: band
        visible: band.mediaShown
        x: band.horizontal ? band.mediaStart : 0
        y: band.horizontal ? 0 : band.mediaStart
        Behavior on x { enabled: band.animate; NumberAnimation { duration: Motion.rowReveal; easing.type: Motion.rowRevealCurve } }
        Behavior on y { enabled: band.animate; NumberAnimation { duration: Motion.rowReveal; easing.type: Motion.rowRevealCurve } }
    }

    // ── the hover name label (owned here, positioned over the hovered island) ─
    Rectangle {
        id: label
        visible: band.labels && band.hoveredApp !== null && band.labelReady
        readonly property string appName: {
            if (!band.hoveredApp)
                return "";
            const e = DesktopEntries.heuristicLookup(band.hoveredApp.cn);
            return (e && e.name) ? e.name : band.hoveredApp.cn;
        }
        readonly property real along: band.hoveredApp ? band.hoveredApp.center : 0
        readonly property real labelGap: 8
        width: labelText.implicitWidth + 16
        height: 22
        radius: 6
        color: Qt.rgba(Theme.surfaceContainerHigh.r, Theme.surfaceContainerHigh.g, Theme.surfaceContainerHigh.b, 0.96)
        border.width: 1
        border.color: Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.12)
        z: 200
        // Float off the band's INNER side (away from the screen edge), centred on
        // the hovered island along the band.
        x: band.horizontal
            ? (label.along - width / 2)
            : (band.edge === "left" ? band.baseSize + label.labelGap : -(width + label.labelGap))
        y: band.horizontal
            ? (band.edge === "bottom" ? -(height + label.labelGap) : band.baseSize + label.labelGap)
            : (label.along - height / 2)
        Text {
            id: labelText
            anchors.centerIn: parent
            text: label.appName
            color: Theme.onSurface
            font.family: Theme.fontPrimary
            font.pixelSize: 11
            font.weight: Font.DemiBold
        }
    }

    // ── drag ghost (follows the cursor while a pin is dragged) ────────────────
    Item {
        id: ghost
        visible: band.dragging
        z: 300
        width: band.baseSize
        height: band.baseSize
        readonly property point p: band.mapFromItem(null, band.dragScene.x, band.dragScene.y)
        x: ghost.p.x - width / 2
        y: ghost.p.y - height / 2
        scale: band.dragging ? 1.15 : 0.9
        Behavior on scale { enabled: band.animate; NumberAnimation { duration: Motion.standard; easing.type: Easing.OutBack } }
        Image {
            anchors.centerIn: parent
            width: band.iconSize
            height: band.iconSize
            source: {
                const i = Dock.iconFor(band.dragClass);
                return i !== "" ? i : Icons.path("application-x-executable", true);
            }
            sourceSize.width: Math.round(band.iconSize * 1.3)
            sourceSize.height: Math.round(band.iconSize * 1.3)
            smooth: true
            opacity: 0.9
        }
    }
}
