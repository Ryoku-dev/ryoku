import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Wayland
import "lib/coords.js" as Coords
import "lib/capture-backend.js" as CaptureBackend
import Ryoku.Ui.Singletons
import "look"

Item {
    id: overlay
    anchors.fill: parent

    required property var screenData
    property var globalSel: null
    property bool capturing: false
    property bool ready: false
    property bool resizable: false
    property bool eyedropArmed: false
    // The region-selector front draws its crosshair and reads the pointer here;
    // pickMode arms the compositor-neutral colour sample (from the frozen plate).
    property bool frontActive: false
    property bool pickMode: false

    property var model: null
    property var draft: null
    property int annRevision: 0
    property bool textEditing: false
    property var selectedIndex: null
    property var moveOffset: null
    property var hoverWindow: null
    // the hover rect is a whole output (the Screen target), not a window
    property bool hoverScreen: false
    // where the live drag started, so the chrome knows which corner is moving
    property var pressPoint: null
    property bool recordIntent: false
    // Beautify is up over this frame
    property bool chromeAway: false

    signal pressedAt(real gx, real gy, int mods)
    signal movedTo(real gx, real gy, int mods)
    signal hovered(real gx, real gy)
    signal released()
    signal frozen()
    signal textChanged(string t)
    signal textCommitted()
    signal wheeled(int dir)
    signal resizeStarted(string role, real gx, real gy)
    signal resizeMoved(real gx, real gy)
    signal resizeEnded()
    signal sampled(color c)

    readonly property int sx: screenData.x
    readonly property int sy: screenData.y
    readonly property string scrName: screenData.name
    readonly property real u: Look.scaleFor(scrName)
    readonly property real dpr: overlay.QsWindow.window ? overlay.QsWindow.window.devicePixelRatio : 1

    readonly property var localSel: globalSel
        ? Coords.intersectRect(globalSel, { x: sx, y: sy, width: width, height: height })
        : null

    // grim renders the same output the compositor would hand screencopy, so the
    // fallback frame lines up pixel for pixel with the layer surface above it.
    readonly property bool forceFallback: CaptureBackend.shouldUseGrim(
        Quickshell.env("RYOSHOT_FORCE_FALLBACK"),
        Quickshell.env("XDG_CURRENT_DESKTOP"),
        Quickshell.env("NIRI_SOCKET"))
    property bool fallbackActive: false
    readonly property string fallbackPath: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoshot-fallback-" + scrName + ".png"

    function selectionBox() {
        if (selectedIndex === null || !model
            || selectedIndex < 0 || selectedIndex >= model.items.length) return null;
        var a = model.items[selectedIndex];
        var off = moveOffset || { x: 0, y: 0 };
        var xs = a.points.map(function (p) { return p.x; });
        var ys = a.points.map(function (p) { return p.y; });
        var x0 = Math.min.apply(null, xs), x1 = Math.max.apply(null, xs);
        var y0 = Math.min.apply(null, ys), y1 = Math.max.apply(null, ys);
        var pad = Math.max((a.width || 4), 6);
        if (a.type === "text") {
            var size = a.size || 16;
            x1 = x0 + Math.max((a.text ? a.text.length : 1) * size * 0.6, size);
            y1 = y0 + size * 1.4;
            pad = 4;
        }
        return {
            x: x0 - sx + off.x - pad,
            y: y0 - sy + off.y - pad,
            w: (x1 - x0) + pad * 2,
            h: (y1 - y0) + pad * 2
        };
    }

    readonly property var selBox: { annRevision; return selectionBox(); }

    Item {
        id: scene
        anchors.fill: parent

        // The captured pixels with no annotation over them. The effect layer
        // samples this, the export samples the whole scene.
        Item {
            id: plate
            anchors.fill: parent

            Loader {
                id: shotLoader
                anchors.fill: parent
                active: !overlay.forceFallback
                sourceComponent: Component {
                    ScreencopyView {
                        anchors.fill: parent
                        captureSource: overlay.screenData
                        live: false
                        paintCursor: false
                        visible: !overlay.fallbackActive
                    }
                }
            }

            Image {
                id: fallbackImage
                anchors.fill: parent
                visible: overlay.fallbackActive
                cache: false
                fillMode: Image.Stretch
                onStatusChanged: {
                    if (status === Image.Ready && !overlay.ready) {
                        overlay.fallbackActive = true;
                        overlay.ready = true;
                        overlay.frozen();
                    } else if (status === Image.Error) {
                        console.error("ryoshot: grim fallback failed to load for " + overlay.scrName);
                    }
                }
            }
        }

        EffectLayer {
            anchors.fill: parent
            frame: plate
            sx: overlay.sx
            sy: overlay.sy
            model: overlay.model
            draft: overlay.draft
            revision: overlay.annRevision
        }

        AnnLayer {
            anchors.fill: parent
            sx: overlay.sx
            sy: overlay.sy
            model: overlay.model
            draft: overlay.draft
            revision: overlay.annRevision
            selectedIndex: overlay.selectedIndex
            moveOffset: overlay.moveOffset
        }
    }

    Process {
        id: grimProc
        command: ["ryoku-cmd-grim", "-o", overlay.scrName, overlay.fallbackPath]
        onExited: (code) => {
            if (code !== 0) {
                console.error("ryoshot: grim fallback exited " + code + " for " + overlay.scrName);
                return;
            }
            fallbackImage.source = "file://" + overlay.fallbackPath + "?t=" + Date.now();
        }
    }

    function startFallback() {
        if (overlay.fallbackActive || grimProc.running) return;
        console.log(overlay.forceFallback
            ? "ryoshot: using grim capture for " + overlay.scrName
            : "ryoshot: no screencopy frame for " + overlay.scrName + ", falling back to grim");
        grimProc.running = true;
    }

    Timer {
        id: capTimer
        interval: 50
        repeat: true
        running: !overlay.forceFallback
        property int tries: 0
        onTriggered: {
            tries += 1;
            var shot = shotLoader.item;
            if (shot && shot.hasContent) {
                running = false;
                overlay.ready = true;
                overlay.frozen();
            } else if (!shot || tries > 60) {
                running = false;
                overlay.startFallback();
            } else {
                shot.captureFrame();
            }
        }
    }

    Component.onCompleted: if (overlay.forceFallback) overlay.startFallback()

    SelectionChrome {
        id: chrome
        anchors.fill: parent
        u: overlay.u
        dpr: overlay.dpr
        ready: overlay.ready
        plate: plate
        sx: overlay.sx
        sy: overlay.sy
        screenName: overlay.scrName
        globalSel: overlay.globalSel
        pressPoint: overlay.pressPoint
        front: overlay.frontActive
        capturing: overlay.capturing
        editing: overlay.resizable
        record: overlay.recordIntent && overlay.frontActive
        picking: overlay.pickMode || overlay.eyedropArmed
        pickHex: probe.hex
        annBox: overlay.selBox
        gripRole: grips.downRole
        hotGrip: grips.hotRole
        pointerIn: input.containsMouse
        px: input.mouseX
        py: input.mouseY
        hoverRect: overlay.hoverWindow
        hoverScreen: overlay.hoverScreen
        away: overlay.chromeAway
    }

    PixelProbe {
        id: probe
        source: plate
        dpr: overlay.dpr
        tracking: chrome.picking && input.containsMouse && overlay.ready
        cx: input.mouseX
        cy: input.mouseY
    }

    ResizeHandles {
        id: grips
        anchors.fill: parent
        // above the drawing MouseArea, or a grip press would start a stroke
        z: 20
        visible: overlay.ready && overlay.resizable && overlay.globalSel !== null
        u: overlay.u
        globalRect: overlay.globalSel
        sx: overlay.sx
        sy: overlay.sy
        onStarted: (role, gx, gy) => overlay.resizeStarted(role, gx, gy)
        onMoved: (gx, gy) => overlay.resizeMoved(gx, gy)
        onEnded: overlay.resizeEnded()
    }

    ClipGrab {
        id: exportClip
        source: scene
        ready: overlay.ready
        rect: overlay.localSel
    }

    ClipGrab {
        id: regionClip
        source: scene
        ready: overlay.ready
        rect: null
    }

    ClipGrab {
        id: plateClip
        source: plate
        ready: overlay.ready
        rect: null
    }

    /** Saves the selected region of this monitor, annotations included. */
    function grabExport(path, cb, targetSize) { exportClip.grab(path, cb, targetSize || null); }

    /** Saves an arbitrary local rect of the annotated scene, for OCR crops. */
    function grabRegion(rect, path, cb) {
        regionClip.rect = rect;
        regionClip.grab(path, cb);
    }

    /** Saves an arbitrary local rect of the raw capture, for palette sampling. */
    function grabPlate(rect, path, cb, targetSize) {
        plateClip.rect = rect;
        plateClip.grab(path, cb, targetSize || null);
    }

    function sampleAt(lx, ly) {
        probe.sample(lx, ly, function (hex) {
            if (hex) overlay.sampled(hex);
            else console.log("ryoshot: colour sample failed at " + lx + "," + ly);
        });
    }

    MouseArea {
        id: input
        anchors.fill: parent
        enabled: overlay.ready
        hoverEnabled: true
        acceptedButtons: Qt.LeftButton
        cursorShape: overlay.eyedropArmed ? Qt.PointingHandCursor : Qt.CrossCursor
        onPressed: (m) => {
            if (overlay.eyedropArmed || overlay.pickMode) { overlay.sampleAt(m.x, m.y); return; }
            overlay.pressedAt(m.x + overlay.sx, m.y + overlay.sy, m.modifiers);
        }
        onPositionChanged: (m) => {
            if (overlay.capturing) overlay.movedTo(m.x + overlay.sx, m.y + overlay.sy, m.modifiers);
            else overlay.hovered(m.x + overlay.sx, m.y + overlay.sy);
        }
        onReleased: { if (!overlay.eyedropArmed && !overlay.pickMode) overlay.released(); }
        onWheel: (w) => {
            if (w.angleDelta.y === 0) return;
            overlay.wheeled(w.angleDelta.y > 0 ? 1 : -1);
            w.accepted = true;
        }
    }

    TextInput {
        id: textEdit
        readonly property bool mine: overlay.textEditing && overlay.draft
            && overlay.draft.type === "text" && overlay.localSel !== null
            && (overlay.draft.points[0].x >= overlay.sx) && (overlay.draft.points[0].x < overlay.sx + overlay.width)
            && (overlay.draft.points[0].y >= overlay.sy) && (overlay.draft.points[0].y < overlay.sy + overlay.height)
        visible: mine
        enabled: mine
        x: mine ? overlay.draft.points[0].x - overlay.sx : 0
        y: mine ? overlay.draft.points[0].y - overlay.sy : 0
        color: mine ? overlay.draft.color : "transparent"
        font.family: Look.ui
        font.pixelSize: mine ? overlay.draft.size : 16
        renderType: Text.NativeRendering
        cursorVisible: mine
        autoScroll: false
        onTextEdited: overlay.textChanged(text)
        onMineChanged: if (mine) { text = overlay.draft.text || ""; forceActiveFocus(); }
        Keys.onPressed: (e) => {
            if (e.key === Qt.Key_Return || e.key === Qt.Key_Enter) { overlay.textCommitted(); e.accepted = true; }
            else if (e.key === Qt.Key_Escape) { e.accepted = false; }
        }
    }
}
