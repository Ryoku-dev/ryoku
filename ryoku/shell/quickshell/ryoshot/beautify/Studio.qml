import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import Lithic
import "../look"
import "../lib/beautify.js" as B
import "stage"

// Beautify, floating over the frozen screen. The selection lifts in place into
// a card: its background blooms out, the corners round, the shadow drops and
// the screen dims, while the panel of edits slides in beside it. The card can
// be framed by dragging the background's edges; Back folds it into the
// selection again. Copy and Save render a composition bound to the exact
// style, made on demand, never the animated stage.
Item {
    id: studio

    // ---- the shell's inputs ----
    property string srcPath: ""
    property string bgImagePath: ""
    property bool composeOnly: false
    property string composeMode: ""
    // the selection in this output's px, or null (a file, or a selection that
    // spans outputs), and the dock button it was opened from, or null
    property var originRect: null
    property var entryRect: null
    // the capture scrim's clear hole on this output, which the studio dim fills
    property var holeRect: null
    property real u: 1
    property real dpr: 1
    property bool fromFile: false
    property bool open: false

    signal copyRequested(string path)
    signal saveRequested(string path)
    signal pickImageRequested()
    signal closeRequested()

    // ---- the style ----
    property string bgKind: B.defaults.bgKind
    property int bgPreset: B.defaults.bgPreset
    property color bgSolid: B.defaults.bgSolid
    property color bgGradA: B.defaults.bgGradA
    property color bgGradB: B.defaults.bgGradB
    property real bgGradAngle: B.defaults.bgGradAngle
    property real padding: B.defaults.padding
    property real roundness: B.defaults.roundness
    property real borderW: B.defaults.borderW
    property color borderColor: B.defaults.borderColor
    property real shadowStrength: B.defaults.shadowStrength
    property real shadowBlur: B.defaults.shadowBlur
    property real shadowDist: B.defaults.shadowDist
    property real shadowAngle: B.defaults.shadowAngle
    property real adjBright: B.defaults.adjBright
    property real adjContrast: B.defaults.adjContrast
    property real adjSat: B.defaults.adjSat
    property string ratioKey: B.defaults.ratioKey
    property bool watermark: B.defaults.watermark
    property bool hd: false

    readonly property var style: ({
        "bgKind": studio.bgKind, "bgPreset": studio.bgPreset, "bgSolid": studio.bgSolid.toString(),
        "bgGradA": studio.bgGradA.toString(), "bgGradB": studio.bgGradB.toString(), "bgGradAngle": studio.bgGradAngle,
        "bgImage": studio.bgImagePath, "padding": studio.padding, "roundness": studio.roundness,
        "borderW": studio.borderW, "borderColor": studio.borderColor.toString(),
        "shadowStrength": studio.shadowStrength, "shadowBlur": studio.shadowBlur, "shadowDist": studio.shadowDist,
        "shadowAngle": studio.shadowAngle, "adjBright": studio.adjBright, "adjContrast": studio.adjContrast,
        "adjSat": studio.adjSat, "ratioKey": studio.ratioKey, "watermark": studio.watermark
    })

    // ---- what the panel reads ----
    readonly property var presets: B.presets
    readonly property var palette: B.palette
    readonly property var ratioRow: [
        { "k": "auto", "l": I18n.tr("Auto") }, { "k": "1:1", "l": "1:1" }, { "k": "4:3", "l": "4:3" },
        { "k": "3:2", "l": "3:2" }, { "k": "16:9", "l": "16:9" }, { "k": "9:16", "l": "9:16" }
    ]
    readonly property var socialRow: [
        { "k": "x", "l": "X" }, { "k": "instagram", "l": "Instagram" }, { "k": "story", "l": I18n.tr("Story") },
        { "k": "linkedin", "l": "LinkedIn" }, { "k": "youtube", "l": "YouTube" }, { "k": "pinterest", "l": "Pinterest" }
    ]
    readonly property var looks: {
        var names = { "ember": I18n.tr("Ember"), "ocean": I18n.tr("Ocean"), "sunset": I18n.tr("Sunset"),
            "mono": I18n.tr("Mono"), "paper": I18n.tr("Paper"), "bare": I18n.tr("Bare") };
        var out = [];
        if (cfg.hasDefault)
            out.push({ "id": "yours", "name": I18n.tr("Yours"), "cfg": studio.savedStyle() });
        for (var i = 0; i < B.looks.length; i++)
            out.push({ "id": B.looks[i].id, "name": names[B.looks[i].id], "cfg": B.looks[i].cfg });
        return out;
    }
    readonly property bool always: cfg.hasDefault
    readonly property bool hasDefault: cfg.hasDefault
    property bool busy: false
    // the panel sets this while one of its popovers is open; Esc then closes it
    property bool popupOpen: false
    signal closePopupRequested()

    property int sourceSerial: 0
    onSrcPathChanged: studio.sourceSerial++
    readonly property url source: studio.srcPath.length > 0 ? "file://" + studio.srcPath + "?v=" + studio.sourceSerial : ""
    readonly property Item probe: stageLoader.item ? stageLoader.item : exporter.item
    readonly property real natW: studio.probe ? studio.probe.srcW : 0
    readonly property real natH: studio.probe ? studio.probe.srcH : 0
    readonly property var frameSize: B.frame(studio.natW, studio.natH, studio.padding, studio.ratioKey)
    readonly property real fullW: studio.frameSize.w
    readonly property real fullH: studio.frameSize.h

    readonly property string userName: Quickshell.env("USER") || "user"
    readonly property string mark: UiPreferences.brandMark(brandAdapter.markText)

    // ---- the stage's state ----
    // from the first frame of the lift until Back
    property bool shown: false
    // the lift: 0 the selection, 1 the card at rest; a spring rides past 1
    property real lift: 0
    readonly property real level: Math.max(0, Math.min(1, studio.lift))
    // the whole studio's fade, which is all the motion reduced motion keeps
    property real veil: 1
    property bool settled: false
    property bool folding: false
    property string dragSide: ""
    readonly property bool paddingDragging: studio.dragSide !== ""

    // the studio shows; a styled export from the dock never does
    readonly property bool live: studio.open && !studio.composeOnly

    // ---- functions ----
    function applyLook(c) {
        for (var k in c)
            studio[k] = c[k];
    }
    // Yours carries hd too, so it is matched with it
    readonly property var matchStyle: Object.assign({ "hd": studio.hd }, studio.style)
    function lookMatches(c) {
        return B.matches(studio.matchStyle, c);
    }
    function nudgePadding(delta) {
        studio.padding = B.clamp(Math.round(studio.padding + delta), 0, 240);
    }
    function pickImage() {
        studio.pickImageRequested();
    }
    onBgImagePathChanged: if (studio.bgImagePath.length > 0) studio.bgKind = "image"

    function copy() {
        studio.exportTo(studio.exportTmp, function (ok) { if (ok) studio.copyRequested(studio.exportTmp); });
    }
    function save() {
        studio.exportTo(studio.exportTmp, function (ok) { if (ok) studio.saveRequested(studio.exportTmp); });
    }

    function back() {
        if (studio.popupOpen) {
            studio.closePopupRequested();
            return;
        }
        if (studio.fromFile) {
            studio.closeRequested();
            return;
        }
        if (studio.folding)
            return;
        studio.folding = true;
        studio.settled = false;
        studio.shown = false;
        spring.running = false;
        liftAnim.stop();
        if (Look.reduceMotion || !studio.originRect) {
            veilOut.restart();
        } else {
            foldAnim.from = studio.lift;
            foldAnim.restart();
        }
    }

    // ---- the persisted default ----
    function savedStyle() {
        var c = {};
        for (var i = 0; i < B.styleKeys.length; i++) {
            var k = B.styleKeys[i];
            if (k !== "bgImage") c[k] = cfg[k];
        }
        c.hd = cfg.hd;
        return c;
    }
    function saveDefault() {
        cfg.hasDefault = true;
        for (var i = 0; i < B.styleKeys.length; i++) {
            var k = B.styleKeys[i];
            if (k === "bgImage") continue;
            var v = studio[k];
            cfg[k] = typeof v === "object" && v !== null ? v.toString() : v;
        }
        cfg.hd = studio.hd;
        cfgFile.writeAdapter();
    }
    function loadDefault() {
        studio.applyLook(studio.savedStyle());
    }
    function setAlways(on) {
        if (on) {
            studio.saveDefault();
        } else {
            cfg.hasDefault = false;
            cfgFile.writeAdapter();
        }
    }
    // while it is on, the default follows every edit
    onStyleChanged: if (studio.always && studio.open && !studio.composeOnly) alwaysSave.restart()
    onHdChanged: if (studio.always && studio.open && !studio.composeOnly) alwaysSave.restart()
    Timer {
        id: alwaysSave
        interval: 400
        onTriggered: if (studio.always) studio.saveDefault()
    }

    FileView {
        id: cfgFile
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/ryoshot-beautify.json"
        blockLoading: true
        printErrors: false
        JsonAdapter {
            id: cfg
            property bool hasDefault: false
            property string bgKind: "preset"
            property int bgPreset: 6
            property string bgSolid: "#2b3350"
            property string bgGradA: "#4facfe"
            property string bgGradB: "#00c6a7"
            property real bgGradAngle: 135
            property real padding: 90
            property real roundness: 24
            property real borderW: 0
            property string borderColor: "#ffffff"
            property real shadowStrength: 45
            property real shadowBlur: 55
            property real shadowDist: 20
            property real shadowAngle: 90
            property real adjBright: 0
            property real adjContrast: 0
            property real adjSat: 0
            property string ratioKey: "auto"
            property bool watermark: false
            property bool hd: false
        }
    }

    // the desktop's brand seal (Ryoku Settings, Shell, Global); empty falls back to 力
    FileView {
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/brand.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        JsonAdapter { id: brandAdapter; property string markText: "力" }
    }

    // ---- export ----
    readonly property string exportTmp: "/tmp/ryoshot-beautified.png"
    readonly property string rawTmp: "/tmp/ryoshot-beautified-raw.png"
    property var exportStyle: ({})
    property var exportDone: null
    property string exportPath: ""
    property bool exportHd: false
    property int exportTries: 0

    // Renders the style as it is now, at its exact values, to path. HD (opt-in)
    // doubles it through waifu2x with no denoise, so text edges stay crisp; a
    // missing tool or an already-large shot keeps the grab.
    function exportTo(path, cb) {
        if (studio.busy || studio.srcPath.length === 0) return;
        studio.busy = true;
        studio.exportStyle = studio.style;
        studio.exportHd = studio.hd;
        studio.exportPath = path;
        studio.exportDone = cb;
        studio.exportTries = 0;
        exporter.active = true;
        exportTick.restart();
    }
    function exportFinished(ok) {
        exporter.active = false;
        studio.busy = false;
        var cb = studio.exportDone;
        studio.exportDone = null;
        if (cb) cb(ok);
    }
    function exportAttempt() {
        var c = exporter.item;
        if (!c || !c.ready) {
            if (studio.exportTries++ < 40) exportTick.restart();
            else studio.exportFinished(false);
            return;
        }
        var target = studio.exportHd ? studio.rawTmp : studio.exportPath;
        var scheduled = c.grabToImage(function (r) {
            var ok = false;
            try { ok = r ? r.saveToFile(target) : false; }
            catch (e) { console.log("ryoshot: beautify grab failed: " + e); }
            if (!ok) {
                if (studio.exportTries++ < 40) exportTick.restart();
                else studio.exportFinished(false);
                return;
            }
            if (studio.exportHd) hdProc.upscale(target, studio.exportPath, function () { studio.exportFinished(true); });
            else studio.exportFinished(true);
        }, Qt.size(Math.round(c.fullW / studio.dpr), Math.round(c.fullH / studio.dpr)));
        if (!scheduled) {
            if (studio.exportTries++ < 40) exportTick.restart();
            else studio.exportFinished(false);
        }
    }
    Timer { id: exportTick; interval: 30; onTriggered: studio.exportAttempt() }
    // The export renders where nothing can see it: past the output's left
    // edge, outside the studio's visibility, which would keep its layers from
    // ever drawing.
    Loader {
        id: exporter
        active: false
        x: -width - 4096
        sourceComponent: Composition {
            source: studio.source
            style: studio.exportStyle
            mark: studio.mark
        }
    }
    Process {
        id: hdProc
        property var cb: null
        function upscale(src, dst, cb_) {
            hdProc.cb = cb_;
            command = ["sh", "-c",
                'src="$1"; dst="$2"; m=/usr/share/waifu2x-ncnn-vulkan/models-cunet; ' +
                'h=$(identify -format "%h" "$src" 2>/dev/null | head -1); ' +
                'if command -v waifu2x-ncnn-vulkan >/dev/null 2>&1 && [ "${h:-0}" -gt 0 ] && [ "${h:-0}" -lt 2000 ]; then ' +
                // nvidia hands back garbage tiles for a shot with alpha at the default tile size, 128 is clean everywhere
                'waifu2x-ncnn-vulkan -i "$src" -o "$dst" -s 2 -n 0 -t 128 -m "$m" >/dev/null 2>&1 && [ -s "$dst" ] || cp -f "$src" "$dst"; ' +
                'else cp -f "$src" "$dst"; fi',
                "sh", src, dst];
            running = true;
        }
        onExited: (code, status) => { var f = hdProc.cb; hdProc.cb = null; if (f) f(); }
    }

    // A styled Copy or Save from the dock bakes the default and exports it
    // without the studio ever showing.
    readonly property bool composeArmed: studio.composeOnly && studio.srcPath.length > 0
    onComposeArmedChanged: if (studio.composeArmed) Qt.callLater(studio.compose)
    Component.onCompleted: {
        if (studio.composeArmed) Qt.callLater(studio.compose);
        else if (studio.open) studio.enter();
        if (studio.live) panelLoader.load();
    }
    property bool composeFired: false
    function compose() {
        if (!studio.composeArmed || studio.composeFired) return;
        studio.composeFired = true;
        if (cfg.hasDefault) studio.loadDefault();
        studio.exportTo(studio.exportTmp, function (ok) {
            if (!ok) {
                studio.composeFired = false;
                studio.closeRequested();
                return;
            }
            if (studio.composeMode === "save") studio.saveRequested(studio.exportTmp);
            else studio.copyRequested(studio.exportTmp);
        });
    }

    // ---- entering ----
    onOpenChanged: {
        if (studio.open) studio.enter();
        else studio.reset();
    }
    function reset() {
        liftAnim.stop();
        foldAnim.stop();
        veilIn.stop();
        veilOut.stop();
        spring.running = false;
        studio.lift = 0;
        studio.veil = 1;
        studio.shown = false;
        studio.settled = false;
        studio.folding = false;
        studio.dragSide = "";
        studio.popupOpen = false;
        studio.waiting = false;
    }
    property bool waiting: false
    function enter() {
        if (studio.composeOnly) return;
        studio.reset();
        if (cfg.hasDefault) studio.loadDefault();
        studio.waiting = true;
        readyWatch.restart();
        studio.tryLift();
    }
    // the card waits for its image, so nothing flashes before it can lift
    readonly property bool stageReady: stageLoader.item !== null && stageLoader.item.ready
    onStageReadyChanged: studio.tryLift()
    function tryLift() {
        if (!studio.waiting || !studio.stageReady) return;
        studio.waiting = false;
        readyWatch.stop();
        studio.beginLift();
    }
    Timer {
        id: readyWatch
        interval: 1500
        onTriggered: if (studio.waiting) { studio.waiting = false; studio.beginLift(); }
    }
    function beginLift() {
        studio.shown = true;
        if (Look.reduceMotion) {
            studio.lift = 1;
            studio.veil = 0;
            veilIn.restart();
            studio.settled = true;
            return;
        }
        if (Look.material && studio.originRect) {
            spring.x = 0;
            spring.v = 0;
            studio.lift = 0;
            spring.running = true;
            return;
        }
        liftAnim.duration = studio.fromFile || !studio.originRect ? 220 : Look.material ? 350 : Look.fluent ? 300 : Look.adwaita ? 250 : 260;
        liftAnim.easing.bezierCurve = Look.fluent ? Look.fluentIn : Look.adwaita ? [0.33, 1, 0.68, 1, 1, 1] : Look.decelerate;
        liftAnim.restart();
    }
    NumberAnimation {
        id: liftAnim
        target: studio
        property: "lift"
        from: 0
        to: 1
        easing.type: Easing.BezierSpline
        onFinished: studio.settled = true
    }
    NumberAnimation {
        id: foldAnim
        target: studio
        property: "lift"
        to: 0
        duration: 200
        easing.type: Easing.BezierSpline
        easing.bezierCurve: [0.4, 0, 0.6, 1, 1, 1]
        onFinished: studio.closeRequested()
    }
    // reduced motion keeps a plain crossfade
    NumberAnimation { id: veilIn; target: studio; property: "veil"; from: 0; to: 1; duration: 150 }
    NumberAnimation {
        id: veilOut
        target: studio
        property: "veil"
        to: 0
        duration: 150
        onFinished: studio.closeRequested()
    }
    // Material's lift is the M3 Expressive default spatial spring (damping
    // ratio 0.8, stiffness 380), stepped per frame so it keeps the display's rate
    FrameAnimation {
        id: spring
        property real x: 0
        property real v: 0
        onTriggered: {
            var t = Math.min(frameTime, 0.05);
            var n = Math.max(1, Math.ceil(t / 0.004));
            var dt = t / n;
            var w = Math.sqrt(380);
            for (var i = 0; i < n; i++) {
                var a = -380 * (spring.x - 1) - 2 * 0.8 * w * spring.v;
                spring.v += a * dt;
                spring.x += spring.v * dt;
            }
            if (Math.abs(spring.x - 1) < 5e-4 && Math.abs(spring.v) < 5e-3) {
                spring.x = 1;
                spring.running = false;
                studio.settled = true;
            }
            studio.lift = spring.x;
        }
    }

    // ---- geometry ----
    readonly property real panelW: panelLoader.item ? panelLoader.item.width : (Look.lithic ? 320 : 340) * studio.u
    readonly property real panelEdge: panelLoader.item && panelLoader.item.edgeMargin !== undefined
        ? panelLoader.item.edgeMargin : (Look.lithic ? 0 : Look.material ? 16 : 12) * studio.u
    readonly property var freeArea: B.freeArea({ w: studio.width, h: studio.height },
        { w: studio.panelW, edge: studio.panelEdge }, 32 * studio.u,
        Look.lithic ? { top: 26 * studio.u, left: 26 * studio.u } : { bottom: 40 * studio.u })
    readonly property real scale0: studio.originRect && studio.natW > 0 ? studio.originRect.w / studio.natW : 1 / studio.dpr
    readonly property var centre0: studio.originRect
        ? { x: studio.originRect.x + studio.originRect.w / 2, y: studio.originRect.y + studio.originRect.h / 2 }
        : { x: studio.freeArea.x + studio.freeArea.w / 2, y: studio.freeArea.y + studio.freeArea.h / 2 }
    readonly property var target: B.rest({ w: Math.max(1, studio.fullW), h: Math.max(1, studio.fullH) },
        studio.scale0, studio.centre0, studio.freeArea)

    // where the card rests; a drag holds it still, then it eases to its new fit
    property real heldScale: 1
    property real heldX: 0
    property real heldY: 0
    property real restScale: studio.paddingDragging ? studio.heldScale : studio.target.scale
    property real restX: studio.paddingDragging ? studio.heldX : studio.target.x
    property real restY: studio.paddingDragging ? studio.heldY : studio.target.y
    Behavior on restScale { enabled: studio.settled && !studio.paddingDragging; NumberAnimation { duration: Look.dur(220); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.decelerate } }
    Behavior on restX { enabled: studio.settled && !studio.paddingDragging; NumberAnimation { duration: Look.dur(220); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.decelerate } }
    Behavior on restY { enabled: studio.settled && !studio.paddingDragging; NumberAnimation { duration: Look.dur(220); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.decelerate } }

    // the values the stage shows ease toward the exact ones
    property real shownPadding: studio.padding
    property real shownRoundness: studio.roundness
    property real shownBorder: studio.borderW
    property real shownStrength: studio.shadowStrength
    property real shownBlur: studio.shadowBlur
    property real shownDist: studio.shadowDist
    property real shownAngle: studio.shadowAngle
    Behavior on shownPadding { enabled: studio.settled && !studio.paddingDragging; NumberAnimation { duration: Look.dur(200); easing.type: Easing.OutCubic } }
    Behavior on shownRoundness { enabled: studio.settled; NumberAnimation { duration: Look.dur(200); easing.type: Easing.OutCubic } }
    Behavior on shownBorder { enabled: studio.settled; NumberAnimation { duration: Look.dur(200); easing.type: Easing.OutCubic } }
    Behavior on shownStrength { enabled: studio.settled; NumberAnimation { duration: Look.dur(200); easing.type: Easing.OutCubic } }
    Behavior on shownBlur { enabled: studio.settled; NumberAnimation { duration: Look.dur(200); easing.type: Easing.OutCubic } }
    Behavior on shownDist { enabled: studio.settled; NumberAnimation { duration: Look.dur(200); easing.type: Easing.OutCubic } }
    Behavior on shownAngle { enabled: studio.settled; NumberAnimation { duration: Look.dur(200); easing.type: Easing.OutCubic } }

    readonly property real bloomed: studio.fromFile || !studio.originRect ? 1 : studio.level
    readonly property var stageStyle: {
        var f = studio.bloomed;
        var s = Object.assign({}, studio.style);
        s.padding = studio.shownPadding * f;
        s.roundness = studio.shownRoundness * f;
        s.borderW = studio.shownBorder * f;
        s.shadowStrength = studio.shownStrength * f;
        s.shadowBlur = studio.shownBlur;
        s.shadowDist = studio.shownDist * f;
        s.shadowAngle = studio.shownAngle;
        s.adjBright = studio.adjBright * f;
        s.adjContrast = studio.adjContrast * f;
        s.adjSat = studio.adjSat * f;
        return s;
    }
    readonly property bool lifted: studio.originRect !== null && !studio.fromFile
    readonly property real shownScale: studio.lifted ? B.lerp(studio.scale0, studio.restScale, studio.lift)
        : studio.restScale * B.lerp(0.94, 1, studio.level)
    readonly property real shownX: studio.lifted ? B.lerp(studio.centre0.x, studio.restX, studio.lift) : studio.restX
    readonly property real shownY: studio.lifted ? B.lerp(studio.centre0.y, studio.restY, studio.lift) : studio.restY
    readonly property real compW: stageLoader.item ? stageLoader.item.fullW : 0
    readonly property real compH: stageLoader.item ? stageLoader.item.fullH : 0
    readonly property rect shownRect: Qt.rect(studio.shownX - studio.compW * studio.shownScale / 2,
        studio.shownY - studio.compH * studio.shownScale / 2, studio.compW * studio.shownScale, studio.compH * studio.shownScale)
    readonly property rect cardRect: Qt.rect(studio.shownX - studio.natW * studio.shownScale / 2,
        studio.shownY - studio.natH * studio.shownScale / 2, studio.natW * studio.shownScale, studio.natH * studio.shownScale)

    // ---- the stage ----
    Item {
        id: deck
        anchors.fill: parent
        visible: studio.live
        opacity: studio.veil

        // the frozen screen stays put under the studio: nothing reaches it
        MouseArea {
            anchors.fill: parent
            hoverEnabled: true
            acceptedButtons: Qt.AllButtons
            onWheel: (w) => w.accepted = true
        }

        Scrim {
            anchors.fill: parent
            hole: studio.holeRect
            level: studio.level
        }

        Checker {
            visible: studio.bgKind === "none" && stageLoader.item !== null
            u: studio.u
            area: studio.shownRect
            opacity: studio.level
        }

        Loader {
            id: stageLoader
            active: studio.live
            x: studio.shownX - width / 2
            y: studio.shownY - height / 2
            width: item ? item.fullW : 0
            height: item ? item.fullH : 0
            scale: studio.shownScale
            transformOrigin: Item.Center
            opacity: studio.lifted ? 1 : studio.level
            sourceComponent: Composition {
                source: studio.source
                style: studio.stageStyle
                mark: studio.mark
                animated: studio.settled
                bloom: studio.bloomed
            }
        }

        Readout {
            anchors.fill: parent
            chrome: studio.kit
            area: studio.shownRect
            card: studio.cardRect
            outW: studio.fullW
            outH: studio.fullH
            hd: studio.hd
            showSize: studio.settled && !studio.folding
            side: studio.dragSide
            padding: Math.round(studio.padding)
        }

        Repeater {
            model: ["t", "r", "b", "l"]
            Handle {
                id: grip
                required property string modelData
                side: grip.modelData
                u: studio.u
                visible: opacity > 0
                opacity: studio.settled && !studio.folding ? 1 : 0
                Behavior on opacity { NumberAnimation { duration: Look.dur(160); easing.type: Easing.OutCubic } }
                readonly property rect r: studio.shownRect
                x: (grip.side === "l" ? r.x : grip.side === "r" ? r.x + r.width : r.x + r.width / 2) - width / 2
                y: (grip.side === "t" ? r.y : grip.side === "b" ? r.y + r.height : r.y + r.height / 2) - height / 2
                property real startPad: 0
                onStarted: {
                    grip.startPad = studio.padding;
                    studio.heldScale = studio.restScale;
                    studio.heldX = studio.restX;
                    studio.heldY = studio.restY;
                    studio.dragSide = grip.side;
                }
                onMoved: (travel) => studio.padding = B.dragPadding(grip.startPad, travel, studio.heldScale)
                onEnded: studio.dragSide = ""
            }
        }

        // ---- the panel ----
        Loader {
            id: panelLoader
            anchors.right: parent.right
            anchors.rightMargin: studio.panelEdge
            anchors.verticalCenter: parent.verticalCenter
            function load() {
                panelLoader.setSource(Qt.resolvedUrl("Panel.qml"), {
                    "studio": studio, "u": studio.u, "maxHeight": studio.panelMax,
                    "entryRect": studio.entryRect, "shown": studio.shown
                });
            }
            onLoaded: {
                item.u = Qt.binding(() => studio.u);
                item.maxHeight = Qt.binding(() => studio.panelMax);
                item.entryRect = Qt.binding(() => studio.entryRect);
                item.shown = Qt.binding(() => studio.shown);
            }
        }
    }

    readonly property QtObject kit: QtObject {
        readonly property real u: studio.u
        readonly property real hair: 1 / studio.dpr
        function snap(v) { return Math.round(v * studio.dpr) / studio.dpr; }
        function px2(n) { return Math.max(1, Math.round(n * studio.u * studio.dpr)) / studio.dpr; }
    }
    readonly property real panelMax: studio.height - 2 * Math.max(studio.panelEdge, 12 * studio.u)
    onLiveChanged: {
        if (studio.live) panelLoader.load();
        else panelLoader.source = "";
    }
}
