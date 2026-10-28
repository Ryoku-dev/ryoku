pragma ComponentBehavior: Bound
import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons as Ui
import "Singletons"
// The Stage Editor's placement grip for the visualiser. The shared frame owns
// movement while the corner grip and turn dot keep their direct gestures.
// Ctrl+wheel scales over the look. The frame boxes the turned footprint exposed
// through `boxItem`.
Item {
    id: win

    // The visualizer instance this grip owns. Its roster index may change when
    // another instance is deleted; identity and interaction routing do not.
    required property string instanceId
    readonly property int instanceIndex: Config.indexOfId(win.instanceId)
    required property bool composing
    property var stageController: null

    signal activated()
    // Direct size and turn gestures keep their own walk-back.
    signal gestureStarted()
    signal gestureFinished()
    // Right-click on the look asks the desktop for its menu, like a slot does.
    signal menuRequested(real x, real y)
    // Keep the shared spectrum running while a look is being aimed, even under
    // Power Saver or silence, so it stays visible to place; released when the
    // editor leaves. The hold is the placer's, moved here with the gestures.
    readonly property bool holding: win.composing && Config.enabled
        && win.instanceIndex < Config.count
    onHoldingChanged: Spectrum.placementHolds += holding ? 1 : -1
    Component.onDestruction: if (holding) Spectrum.placementHolds -= 1

    anchors.fill: parent
    visible: win.composing && Config.enabled && win.instanceIndex < Config.count

    readonly property real handle: Ui.Tokens.s4
    readonly property var instance: Config.dataById(win.instanceId)
    readonly property var preview: Config.previewAt(win.instanceId)
    function value(key, fallback) {
        const v = win.instance[key];
        return v === undefined || v === null ? fallback : v;
    }
    readonly property bool aura: String(win.value("style", "bars")) === "aura"
    readonly property real vx: win.preview
        ? Number(win.preview.x) : Number(win.value("x", 0))
    readonly property real vy: win.preview
        ? Number(win.preview.y) : Number(win.value("y", 0.58))
    readonly property real vw: Number(win.value("w", 1))
    readonly property real vh: Number(win.value("h", 0.42))
    readonly property real va: Number(win.value("angle", 0))
    // The look's box, in desktop px; the field's box is the screen.
    readonly property rect box: win.aura
        ? Qt.rect(0, 0, win.width, win.height)
        : Qt.rect(win.vx * win.width, win.vy * win.height,
                  win.vw * win.width, win.vh * win.height)
    readonly property real cx: win.box.x + win.box.width / 2
    readonly property real cy: win.box.y + win.box.height / 2
    readonly property real aspect: win.height > 0 ? win.width / win.height : 1

    property bool stageMoveActive: false
    property bool dragging: false
    property real stageMoveGrabX: 0
    property real stageMoveGrabY: 0
    property real stageMoveStartX: 0
    property real stageMoveStartY: 0
    property real groupDragMinX: -Infinity
    property real groupDragMaxX: Infinity
    property real groupDragMinY: -Infinity
    property real groupDragMaxY: Infinity

    function stageBeginMove(point, modifiers) {
        if (win.aura || !win.composing || !win.stageController)
            return false;
        const bounds = win.stageController.widgetDragStarted(
            "visualizer:" + win.instanceId);
        if (bounds.active === false)
            return false;
        win.stageMoveActive = true;
        win.dragging = false;
        win.stageMoveStartX = footprint.x;
        win.stageMoveStartY = footprint.y;
        win.stageMoveGrabX = point.x - footprint.x;
        win.stageMoveGrabY = point.y - footprint.y;
        win.groupDragMinX = bounds.minX;
        win.groupDragMaxX = bounds.maxX;
        win.groupDragMinY = bounds.minY;
        win.groupDragMaxY = bounds.maxY;
        return true;
    }
    function stageUpdateMove(point, modifiers) {
        if (!win.stageMoveActive)
            return;
        const nx = point.x - win.stageMoveGrabX;
        const ny = point.y - win.stageMoveGrabY;
        if (!win.dragging) {
            if (Math.abs(nx - win.stageMoveStartX) < 6
                    && Math.abs(ny - win.stageMoveStartY) < 6)
                return;
            win.dragging = true;
        }
        let px = Math.max(win.groupDragMinX,
            Math.min(win.groupDragMaxX, footprint.clampX(nx)));
        let py = Math.max(win.groupDragMinY,
            Math.min(win.groupDragMaxY, footprint.clampY(ny)));
        const bounded = win.stageController.widgetDragMoved(
            "visualizer:" + win.instanceId, px, py, modifiers);
        footprint.stagePreviewPosition(bounded.x, bounded.y);
    }
    function stageEndMove(modifiers) {
        if (!win.stageMoveActive)
            return;
        if (win.dragging) {
            const px = Math.round(footprint.x);
            const py = Math.round(footprint.y);
            const handled = win.stageController.widgetDragEnded(
                "visualizer:" + win.instanceId, px, py, modifiers);
            if (!handled) {
                footprint.stageCommitPosition(px, py);
                footprint.stageEndPreview();
            }
        } else {
            win.stageController.widgetDragCancelled(
                "visualizer:" + win.instanceId);
        }
        win.stageMoveActive = false;
        win.dragging = false;
        win.groupDragMinX = -Infinity;
        win.groupDragMaxX = Infinity;
        win.groupDragMinY = -Infinity;
        win.groupDragMaxY = Infinity;
    }
    function stageCancelMove() {
        if (!win.stageMoveActive)
            return;
        win.stageController.widgetDragCancelled(
            "visualizer:" + win.instanceId);
        Config.clearPreview(win.instanceId);
        win.stageMoveActive = false;
        win.dragging = false;
    }
    function stageHandleAt(point) {
        if (win.aura)
            return "";
        const turnPoint = spinner.mapToItem(win,
            spinner.width / 2, spinner.height / 2);
        if (Math.abs(point.x - turnPoint.x) < win.handle
                && Math.abs(point.y - turnPoint.y) < win.handle)
            return "turn";
        const sizePoint = grip.mapToItem(win, grip.width / 2, grip.height / 2);
        if (Math.abs(point.x - sizePoint.x) < win.handle
                && Math.abs(point.y - sizePoint.y) < win.handle)
            return "size";
        return "";
    }

    readonly property string customInk: String(win.value("color", ""))
    readonly property color guide: /^#[0-9a-fA-F]{6}$/.test(win.customInk)
        ? win.customInk : Ui.Tokens.sun

    property string gesture: ""
    property real tx: 0
    property real ty: 0
    property real tw: 0
    property real th: 0
    property real tAngle: 0

    // StageOutline's four corner handles scale from the centre. This remains
    // stable for turned boxes, unlike axis-aligned corner maths that jumps as
    // soon as a rotated footprint is grabbed.
    readonly property bool locked: false
    property bool resizing: false
    property real resizeStartDistance: 1
    property real resizeStartW: 1
    property real resizeStartH: 1
    property real resizeCentreX: 0.5
    property real resizeCentreY: 0.5
    function stageBeginResize(corner, point, modifiers) {
        if (win.aura || !win.composing)
            return;
        Config.setActive(win.instanceId);
        win.activated();
        win.resizeStartW = win.vw;
        win.resizeStartH = win.vh;
        win.resizeCentreX = win.vx + win.vw / 2;
        win.resizeCentreY = win.vy + win.vh / 2;
        win.resizeStartDistance = Math.max(1,
            Math.hypot(point.x - win.cx, point.y - win.cy));
        win.resizing = true;
        win.gestureStarted();
    }
    function stageUpdateResize(point, modifiers) {
        if (!win.resizing)
            return;
        const distance = Math.max(1,
            Math.hypot(point.x - win.cx, point.y - win.cy));
        const factor = distance / win.resizeStartDistance;
        const nw = win.resizeStartW * factor;
        const nh = win.resizeStartH * factor;
        Config.setBox(win.resizeCentreX - nw / 2,
            win.resizeCentreY - nh / 2, nw, nh, win.aspect);
    }
    function stageEndResize() {
        if (!win.resizing)
            return;
        win.resizing = false;
        win.gestureFinished();
    }
    function stageCancelResize() {
        if (!win.resizing)
            return;
        Config.setBox(win.resizeCentreX - win.resizeStartW / 2,
            win.resizeCentreY - win.resizeStartH / 2,
            win.resizeStartW, win.resizeStartH, win.aspect);
        win.resizing = false;
        win.gestureFinished();
    }

    Timer {
        id: ease
        interval: 16
        repeat: true
        running: win.gesture !== ""
        onTriggered: {
            var k = 0.32;
            var eps = 0.0006;
            var done = false;
            if (win.gesture === "turn") {
                var d = PlaceMath.shortestTurn(win.va, win.tAngle);
                done = Math.abs(d) < 0.05;
                Config.rotate(done ? win.tAngle : win.va + d * k);
            } else {
                done = Math.abs(win.tx - win.vx) < eps && Math.abs(win.ty - win.vy) < eps
                    && Math.abs(win.tw - win.vw) < eps && Math.abs(win.th - win.vh) < eps;
                if (done)
                    Config.setBox(win.tx, win.ty, win.tw, win.th, win.aspect);
                else
                    Config.setBox(win.vx + (win.tx - win.vx) * k,
                                  win.vy + (win.ty - win.vy) * k,
                                  win.vw + (win.tw - win.vw) * k,
                                  win.vh + (win.th - win.vh) * k,
                                  win.aspect);
            }
            // Over only once the hand is off and the box has caught up; the
            // desktop turns that into one undo entry.
            if (done && grab.mode === "") {
                win.gesture = "";
                win.gestureFinished();
            }
        }
    }

    Connections {
        target: Config
        function onActiveChanged() {
            if (Config.activeId === win.instanceId || win.gesture === "")
                return;
            win.gesture = "";
            grab.mode = "";
            win.gestureFinished();
        }
    }

    // The look's turned footprint, axis-aligned: the edit frame (StageOutline)
    // boxes this the way it boxes every other widget, and the inspector docks
    // beside it. Same bounding-box maths the spectrum field uses for its cover
    // rect, so the frame hugs the look at any angle rather than its unturned w/h.
    readonly property rect outer: {
        var a = win.va * Math.PI / 180;
        var c = Math.abs(Math.cos(a)), s = Math.abs(Math.sin(a));
        var w = win.box.width * c + win.box.height * s;
        var h = win.box.width * s + win.box.height * c;
        return Qt.rect(win.cx - w / 2, win.cy - h / 2, w, h);
    }
    Item {
        id: footprint
        x: win.outer.x
        y: win.outer.y
        width: win.outer.width
        height: win.outer.height
        readonly property bool locked: win.aura

        function clampX(value) {
            const overhang = Math.max(0, Math.min(0.5, Config.overhang));
            return Math.max(-footprint.width * overhang,
                Math.min(win.width - footprint.width * (1 - overhang), value));
        }
        function clampY(value) {
            const overhang = Math.max(0, Math.min(0.5, Config.overhang));
            return Math.max(-footprint.height * overhang,
                Math.min(win.height - footprint.height * (1 - overhang), value));
        }
        function boxPosition(px, py) {
            return Qt.point(
                (px + (footprint.width - win.box.width) / 2) / Math.max(1, win.width),
                (py + (footprint.height - win.box.height) / 2) / Math.max(1, win.height));
        }
        function stagePreviewPosition(px, py) {
            if (win.aura)
                return;
            const point = footprint.boxPosition(
                footprint.clampX(px), footprint.clampY(py));
            Config.setPreview(win.instanceId, point.x, point.y);
        }
        function stageEndPreview() {
            Config.clearPreview(win.instanceId);
        }
        function stageCommitPosition(px, py) {
            if (win.aura)
                return;
            const point = footprint.boxPosition(
                footprint.clampX(px), footprint.clampY(py));
            Config.moveBoxAt(win.instanceId, point.x, point.y, win.aspect);
        }
    }
    // The turned footprint, exposed for the desktop: the edit frame boxes this
    // the way it boxes a slot, and the inspector docks beside it.
    readonly property Item boxItem: footprint

    // The handles ride a turned frame, so they sit where the look's own corner
    // and top edge actually are. The outline itself is the edit frame's job.
    Item {
        id: frame

        // The field owns the whole screen and has no box to aim: the edge
        // handles would ring the display with controls that edit nothing (the
        // placer treated it the same way), so the field rides handleless.
        visible: !win.aura
        x: win.box.x
        y: win.box.y
        width: win.box.width
        height: win.box.height
        rotation: win.va
        transformOrigin: Item.Center

        // the grip sits on the box's own corner, so it is always beside what it sizes
        Rectangle {
            id: grip
            width: win.handle
            height: win.handle
            radius: Ui.Tokens.radius
            color: (grab.mode === "size" || grab.over === "size") ? win.guide : Qt.alpha(win.guide, 0.45)
            border.width: Ui.Tokens.border
            border.color: win.guide
            x: parent.width - width / 2
            y: parent.height - height / 2
        }

        // the turn handle stands off the top edge on a stem, so it reads as a lever
        // rather than another corner
        Rectangle {
            width: Ui.Tokens.border
            height: win.handle * 1.6
            color: Qt.alpha(win.guide, 0.55)
            x: parent.width / 2
            y: -height
        }
        Rectangle {
            id: spinner
            width: win.handle
            height: win.handle
            radius: width / 2
            color: (grab.mode === "turn" || grab.over === "turn") ? win.guide : Qt.alpha(win.guide, 0.45)
            border.width: Ui.Tokens.border
            border.color: win.guide
            x: parent.width / 2 - width / 2
            y: -win.handle * 1.6 - height / 2
        }
    }

    // Ctrl+wheel scales, the same chord every other widget in the editor uses.
    // A notch that repeats is one gesture: the walk-back opens on the first and
    // closes when the wheel settles, the way the slot's does.
    property bool wheelHold: false
    Timer {
        id: wheelSettle
        interval: Ui.Tokens.move * 2
        onTriggered: {
            win.wheelHold = false;
            win.gestureFinished();
        }
    }

    // The direct press area only owns the two handles. The shared frame owns
    // movement and forwards handle presses here.
    MouseArea {
        id: grab

        enabled: !win.aura
        x: Math.max(0, footprint.x - win.handle * 3)
        y: Math.max(0, footprint.y - win.handle * 3)
        width: Math.min(win.width - x, footprint.width + win.handle * 6)
        height: Math.min(win.height - y, footprint.height + win.handle * 6)
        acceptedButtons: Qt.LeftButton | Qt.RightButton
        hoverEnabled: true
        cursorShape: grab.over === "size" ? Qt.SizeFDiagCursor
            : (grab.over === "turn" ? Qt.CrossCursor : Qt.ArrowCursor)

        readonly property string over: {
            const point = grab.mapToItem(win, grab.mouseX, grab.mouseY);
            return win.stageHandleAt(point);
        }

        property string mode: ""
        property real pressX: 0
        property real pressY: 0
        property real baseX: 0
        property real baseY: 0
        property real baseW: 0
        property real baseH: 0
        property real baseAngle: 0
        property real pressAngle: 0

        onPressed: (m) => {
            Config.setActive(win.instanceId);
            win.activated();
            if (m.button === Qt.RightButton) {
                win.menuRequested(win.box.x, win.box.y);
                return;
            }
            if (win.aura || grab.over === "") {
                m.accepted = false;
                return;
            }
            grab.mode = grab.over;
            win.gesture = grab.over;
            win.gestureStarted();
            grab.pressX = m.x;
            grab.pressY = m.y;
            grab.baseX = win.vx;
            grab.baseY = win.vy;
            grab.baseW = win.vw;
            grab.baseH = win.vh;
            grab.baseAngle = win.va;
            const pressPoint = grab.mapToItem(win, m.x, m.y);
            grab.pressAngle = PlaceMath.angleAt(win.cx, win.cy,
                pressPoint.x, pressPoint.y);
            win.tx = win.vx;
            win.ty = win.vy;
            win.tw = win.vw;
            win.th = win.vh;
            win.tAngle = win.va;
        }
        onReleased: grab.mode = ""
        onDoubleClicked: win.menuRequested(win.box.x, win.box.y)
        // Deltas from the press, never absolute positions, so nothing jumps.
        onPositionChanged: (m) => {
            if (!grab.pressed || grab.mode === "" || win.aura)
                return;
            if (grab.mode === "turn") {
                const point = grab.mapToItem(win, m.x, m.y);
                // Near the centre a pixel of travel is a wild swing.
                if (Math.hypot(point.x - win.cx, point.y - win.cy) < win.handle * 1.5)
                    return;
                var want = grab.baseAngle
                    + PlaceMath.angleAt(win.cx, win.cy, point.x, point.y) - grab.pressAngle;
                win.tAngle = PlaceMath.magnet(want, 15, 2.5);
                return;
            }
            var dx = m.x - grab.pressX;
            var dy = m.y - grab.pressY;
            var out = PlaceMath.resize({ x: grab.baseX, y: grab.baseY, w: grab.baseW, h: grab.baseH },
                                       grab.baseAngle, dx, dy,
                                       { w: win.width, h: win.height }, { w: 0.04, h: 0.03 });
            win.tx = out.x;
            win.ty = out.y;
            win.tw = out.w;
            win.th = out.h;
        }
        // Ctrl+wheel scales over the same footprint as the press: a wheel
        // anywhere else still scrolls what is under it. The handler takes no
        // geometry of its own; nesting it in the area borrows the area's.
        WheelHandler {
            acceptedModifiers: Qt.ControlModifier
            onWheel: (w) => {
                Config.setActive(win.instanceId);
                win.activated();
                if (!win.wheelHold) {
                    win.wheelHold = true;
                    win.gestureStarted();
                }
                var k = w.angleDelta.y > 0 ? 1.06 : 0.94;
                Config.sizeBox(win.vw * k, win.vh * k, win.aspect);
                wheelSettle.restart();
            }
        }
    }
}
