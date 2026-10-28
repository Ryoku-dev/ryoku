import QtQuick
import QtQuick.Effects
import QtQuick.Controls as QQC
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "Singletons"

// Beautify: a sharing-first image editor for a capture. Frosted, textured chrome
// (the launcher's grainy look: a warm translucent surface over a square-grid
// weave), a live canvas, and a right panel of the right control per job -- a
// background thumbnail grid (preset gradients, solid, custom gradient, image,
// transparent), colour swatches, ratio + social-format pills, a shadow-direction
// dial -- with sliders only for the continuous frame values. The card layers are
// kept separate (a blurred directional shadow, a mask-rounded image, a coloured
// border) so rounding, border colour and shadow direction each work on their own.
// Exports at full resolution through the same wl-copy / save path as the shot.
Item {
    id: beautify

    property string srcPath: ""
    property string bgImagePath: ""

    signal copyRequested(string path)
    signal saveRequested(string path)
    signal pickImageRequested()
    signal closeRequested()

    readonly property string exportTmp: "/tmp/ryoshot-beautified.png"

    readonly property color vermilion: Theme.accent
    readonly property color bright: Theme.ink
    readonly property color idle: Theme.inkDim
    readonly property color dim: Theme.inkFaint
    readonly property color panelBg: Theme.panel
    readonly property color fieldBg: Theme.field
    readonly property color hair: Theme.hair

    // ---- state ----
    property string bgKind: "preset"     // preset | solid | gradient | image | none
    property int bgPreset: 6
    property color bgSolid: "#2b3350"
    property color bgGradA: "#4facfe"
    property color bgGradB: "#00c6a7"
    property real bgGradAngle: 135
    property real padding: 90
    property real roundness: 24
    property real borderW: 0
    property color borderColor: "#ffffff"
    property real shadowStrength: 45
    property real shadowBlur: 55
    property real shadowDist: 20
    property real shadowAngle: 90
    property real adjBright: 0
    property real adjContrast: 0
    property real adjSat: 0
    property string ratioKey: "auto"
    property bool watermark: false
    readonly property string userName: Quickshell.env("USER") || "user"
    // the desktop brand seal, user-overridable via ~/.config/ryoku/brand.json
    // (Ryoku Settings -> Shell -> Global). the stamped watermark and the editor
    // seals follow it; the user@host handle beside the watermark stays as-is.
    // empty/absent markText falls back to the 力 default, so this never seeds.
    readonly property string mark: UiPreferences.brandMark(brandAdapter.markText)
    property bool composeOnly: false
    property string composeMode: ""
    property bool hd: false
    property bool busy: false

    readonly property var presets: [
        { "a": "#4facfe", "b": "#00c6a7", "ang": 135 },
        { "a": "#f6543e", "b": "#f7b733", "ang": 135 },
        { "a": "#7b4397", "b": "#dc2430", "ang": 135 },
        { "a": "#11998e", "b": "#38ef7d", "ang": 135 },
        { "a": "#ee9ca7", "b": "#ffdde1", "ang": 135 },
        { "a": "#334155", "b": "#0f172a", "ang": 135 },
        { "a": "#e2342a", "b": "#14120f", "ang": 135 },
        { "a": "#00c3ff", "b": "#ffff1c", "ang": 135 },
        { "a": "#ffecd2", "b": "#fcb69f", "ang": 135 },
        { "a": "#3e6868", "b": "#0f1514", "ang": 135 },
        { "a": "#ff6a88", "b": "#ff99ac", "ang": 135 },
        { "a": "#141e30", "b": "#243b55", "ang": 135 }
    ]
    readonly property var palette: [
        "#ffffff", "#0e0d0b", "#e2342a", "#f3701e", "#f7b733", "#38ef7d",
        "#4facfe", "#2b6cb0", "#7b4397", "#ee9ca7", "#3e6868", "#c7bfae"
    ]
    readonly property var ratioRow: [
        { "k": "auto", "l": I18n.tr("Auto") }, { "k": "1:1", "l": "1:1" }, { "k": "4:3", "l": "4:3" },
        { "k": "3:2", "l": "3:2" }, { "k": "16:9", "l": "16:9" }, { "k": "9:16", "l": "9:16" }
    ]
    readonly property var socialRow: [
        { "k": "x", "l": "X" }, { "k": "instagram", "l": "Instagram" }, { "k": "story", "l": I18n.tr("Story") },
        { "k": "linkedin", "l": "LinkedIn" }, { "k": "youtube", "l": "YouTube" }, { "k": "pinterest", "l": "Pinterest" }
    ]
    readonly property var ratioMap: ({ "auto": 0, "1:1": 1, "4:3": 1.3333, "3:2": 1.5, "16:9": 1.7778, "9:16": 0.5625, "x": 1.7778, "instagram": 1, "story": 0.5625, "linkedin": 1.91, "youtube": 1.7778, "pinterest": 0.6667 })

    readonly property bool hasDefault: cfg.hasDefault
    readonly property var looks: [
        { "name": "Ember",  "cfg": { "bgKind": "preset", "bgPreset": 6, "padding": 90, "roundness": 24, "borderW": 0, "shadowStrength": 45, "shadowBlur": 55, "shadowDist": 20, "shadowAngle": 90 } },
        { "name": "Ocean",  "cfg": { "bgKind": "preset", "bgPreset": 0, "padding": 96, "roundness": 26, "borderW": 0, "shadowStrength": 42, "shadowBlur": 60, "shadowDist": 22, "shadowAngle": 100 } },
        { "name": "Sunset", "cfg": { "bgKind": "preset", "bgPreset": 1, "padding": 88, "roundness": 22, "borderW": 0, "shadowStrength": 50, "shadowBlur": 50, "shadowDist": 24, "shadowAngle": 110 } },
        { "name": "Mono",   "cfg": { "bgKind": "solid", "bgSolid": "#14120f", "padding": 72, "roundness": 16, "borderW": 0, "shadowStrength": 32, "shadowBlur": 42, "shadowDist": 16, "shadowAngle": 90 } },
        { "name": "Paper",  "cfg": { "bgKind": "solid", "bgSolid": "#f5efe4", "padding": 64, "roundness": 12, "borderW": 1, "borderColor": "#dcd3c2", "shadowStrength": 18, "shadowBlur": 30, "shadowDist": 10, "shadowAngle": 90 } },
        { "name": "Bare",   "cfg": { "bgKind": "none", "padding": 0, "roundness": 0, "borderW": 0, "shadowStrength": 0 } }
    ]

    // ---- resolved background ----
    readonly property bool bgIsGradient: bgKind === "preset" || bgKind === "gradient"
    readonly property color bgA: bgKind === "preset" ? presets[bgPreset].a : bgGradA
    readonly property color bgB: bgKind === "preset" ? presets[bgPreset].b : bgGradB
    readonly property real bgAngle: bgKind === "preset" ? presets[bgPreset].ang : bgGradAngle

    // ---- geometry (full resolution) ----
    readonly property real natW: img.sourceSize.width > 0 ? img.sourceSize.width : 800
    readonly property real natH: img.sourceSize.height > 0 ? img.sourceSize.height : 500
    readonly property real minW: natW + 2 * padding
    readonly property real minH: natH + 2 * padding
    readonly property real ratioV: ratioMap[ratioKey] !== undefined ? ratioMap[ratioKey] : 0
    readonly property real fullW: ratioV <= 0 ? minW : (ratioV >= minW / minH ? minH * ratioV : minW)
    readonly property real fullH: ratioV <= 0 ? minH : (ratioV >= minW / minH ? minH : minW / ratioV)

    // grab the composition to `path`. HD (opt-in) reroutes through the GPU
    // upscaler first: waifu2x doubles the resolution with no denoise so text edges
    // stay crisp. A missing tool or an already-large shot falls back to the grab.
    readonly property string rawTmp: "/tmp/ryoshot-beautified-raw.png"
    function exportStage(path, cb) {
        beautify.busy = true;
        function done(ok) { beautify.busy = false; if (cb) cb(ok); }
        var target = beautify.hd ? beautify.rawTmp : path;
        var scheduled = stage.grabToImage(function (r) {
            var ok = false;
            try { ok = r ? r.saveToFile(target) : false; }
            catch (e) { console.log("ryoshot: beautify grab failed: " + e); }
            if (!ok) { done(false); return; }
            if (beautify.hd) hdProc.upscale(target, path, function () { done(true); });
            else done(true);
        }, Qt.size(Math.round(beautify.fullW), Math.round(beautify.fullH)));
        if (!scheduled) done(false);
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
                'waifu2x-ncnn-vulkan -i "$src" -o "$dst" -s 2 -n 0 -m "$m" >/dev/null 2>&1 && [ -s "$dst" ] || cp -f "$src" "$dst"; ' +
                'else cp -f "$src" "$dst"; fi',
                "sh", src, dst];
            running = true;
        }
        onExited: (code, status) => { var f = hdProc.cb; hdProc.cb = null; if (f) f(); }
    }

    // ---- named looks + persisted default ----
    // applyLook only writes the keys a look defines, so partial looks (e.g. Bare)
    // leave everything else alone.
    function applyLook(c) {
        for (var k in c)
            beautify[k] = c[k];
    }
    function saveDefault() {
        cfg.hasDefault = true;
        cfg.bgKind = beautify.bgKind; cfg.bgPreset = beautify.bgPreset;
        cfg.bgSolid = beautify.bgSolid.toString(); cfg.bgGradA = beautify.bgGradA.toString(); cfg.bgGradB = beautify.bgGradB.toString(); cfg.bgGradAngle = beautify.bgGradAngle;
        cfg.padding = beautify.padding; cfg.roundness = beautify.roundness; cfg.borderW = beautify.borderW; cfg.borderColor = beautify.borderColor.toString();
        cfg.shadowStrength = beautify.shadowStrength; cfg.shadowBlur = beautify.shadowBlur; cfg.shadowDist = beautify.shadowDist; cfg.shadowAngle = beautify.shadowAngle;
        cfg.adjBright = beautify.adjBright; cfg.adjContrast = beautify.adjContrast; cfg.adjSat = beautify.adjSat;
        cfg.ratioKey = beautify.ratioKey; cfg.watermark = beautify.watermark; cfg.hd = beautify.hd;
        cfgFile.writeAdapter();
    }
    function loadDefault() {
        beautify.bgKind = cfg.bgKind; beautify.bgPreset = cfg.bgPreset;
        beautify.bgSolid = cfg.bgSolid; beautify.bgGradA = cfg.bgGradA; beautify.bgGradB = cfg.bgGradB; beautify.bgGradAngle = cfg.bgGradAngle;
        beautify.padding = cfg.padding; beautify.roundness = cfg.roundness; beautify.borderW = cfg.borderW; beautify.borderColor = cfg.borderColor;
        beautify.shadowStrength = cfg.shadowStrength; beautify.shadowBlur = cfg.shadowBlur; beautify.shadowDist = cfg.shadowDist; beautify.shadowAngle = cfg.shadowAngle;
        beautify.adjBright = cfg.adjBright; beautify.adjContrast = cfg.adjContrast; beautify.adjSat = cfg.adjSat;
        beautify.ratioKey = cfg.ratioKey; beautify.watermark = cfg.watermark; beautify.hd = cfg.hd;
    }
    // reset strips all styling back to the raw capture.
    function resetDefault() {
        applyLook({ "bgKind": "none", "padding": 0, "roundness": 0, "borderW": 0, "shadowStrength": 0 });
    }
    // each new beautify session starts from the saved default, so shots come out
    // consistent without re-tuning.
    onVisibleChanged: {
        if (visible && cfg.hasDefault) beautify.loadDefault();
        if (visible && beautify.composeOnly) { beautify.composeFired = false; composeTimer.restart(); }
    }
    // headless "bake the default and export" path: used when Copy/Save is pressed
    // in the toolbar with a saved default, so the shot exports styled without the
    // editor ever opening.
    property int composeTries: 0
    // Fire exactly once per compose session: without this the timer could grab
    // twice (a half-rendered frame and the settled one), copying two frames and
    // racing two wl-copy owners so the live selection ends up cleared.
    property bool composeFired: false
    function composeExport() {
        if (beautify.composeFired) return;
        if (img.status !== Image.Ready) {
            if (beautify.composeTries++ < 20) composeTimer.restart();
            else beautify.closeRequested();
            return;
        }
        beautify.composeFired = true;
        beautify.exportStage(beautify.exportTmp, function (ok) {
            if (ok) {
                beautify.composeTries = 0;
                if (beautify.composeMode === "save") beautify.saveRequested(beautify.exportTmp);
                else beautify.copyRequested(beautify.exportTmp);
                return;
            }
            // Transient failure: the styled stage was not rendered yet. Release
            // the one-shot claim and retry a few frames on, so a click never ends
            // up copying nothing.
            beautify.composeFired = false;
            if (beautify.composeTries++ < 20) composeTimer.restart();
            else beautify.closeRequested();
        });
    }
    Timer { id: composeTimer; interval: 220; onTriggered: beautify.composeExport() }

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

    // brand identity master (shared with the shell and the Hub's
    // Shell -> Global editor). watched so a Settings change retunes the live
    // editor. only markText is read here; the shell owns seeding and the other
    // keys, so this reads gracefully with a 力 default and never writes back.
    FileView {
        id: brandFile
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/brand.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        JsonAdapter { id: brandAdapter; property string markText: "力" }
    }

    // ============================ frosted backdrop ============================
    Rectangle {
        anchors.fill: parent
        gradient: Gradient {
            GradientStop { position: 0.0; color: Theme.panelSolid }
            GradientStop { position: 1.0; color: Qt.darker(Theme.panelSolid, 1.4) }
        }
    }
    Canvas {
        anchors.fill: parent
        opacity: 0.5
        onPaint: {
            var ctx = getContext("2d");
            ctx.clearRect(0, 0, width, height);
            ctx.strokeStyle = "rgba(245, 239, 228, 0.04)";
            ctx.lineWidth = 1;
            var step = 34;
            for (var x = 0; x <= width; x += step) { ctx.beginPath(); ctx.moveTo(x, 0); ctx.lineTo(x, height); ctx.stroke(); }
            for (var y = 0; y <= height; y += step) { ctx.beginPath(); ctx.moveTo(0, y); ctx.lineTo(width, y); ctx.stroke(); }
        }
        Component.onCompleted: requestPaint()
    }

    // ============================ top bar ============================
    Rectangle {
        id: topbar
        visible: !beautify.composeOnly
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        height: 56
        color: beautify.panelBg
        Rectangle { anchors.left: parent.left; anchors.right: parent.right; anchors.bottom: parent.bottom; height: 1; color: beautify.hair }

        Row {
            anchors.left: parent.left
            anchors.leftMargin: 24
            anchors.verticalCenter: parent.verticalCenter
            spacing: 11
            Text { anchors.verticalCenter: parent.verticalCenter; text: beautify.mark; color: beautify.vermilion; font.family: "Noto Sans CJK JP"; font.pixelSize: 21; font.weight: Font.DemiBold }
            Text { anchors.verticalCenter: parent.verticalCenter; text: I18n.tr("Beautify"); color: beautify.bright; font.family: Theme.ui; font.pixelSize: 16; font.weight: Font.DemiBold }
            Text { anchors.verticalCenter: parent.verticalCenter; text: I18n.tr("ready to share"); color: beautify.dim; font.family: Theme.ui; font.pixelSize: 12 }
        }
        Row {
            anchors.right: parent.right
            anchors.rightMargin: 18
            anchors.verticalCenter: parent.verticalCenter
            spacing: 10
            TopBtn { label: I18n.tr("Back"); onTapped: beautify.closeRequested() }
            TopBtn { label: beautify.busy ? I18n.tr("Working\u2026") : I18n.tr("Copy"); onTapped: { if (!beautify.busy) beautify.exportStage(beautify.exportTmp, function (ok) { if (ok) beautify.copyRequested(beautify.exportTmp); }); } }
            TopBtn { label: beautify.busy ? I18n.tr("Working\u2026") : I18n.tr("Save image"); accent: true; onTapped: { if (!beautify.busy) beautify.exportStage(beautify.exportTmp, function (ok) { if (ok) beautify.saveRequested(beautify.exportTmp); }); } }
        }
    }

    // ============================ right panel ============================
    Rectangle {
        id: panel
        visible: !beautify.composeOnly
        anchors.right: parent.right
        anchors.top: topbar.bottom
        anchors.bottom: parent.bottom
        width: 360
        color: beautify.panelBg
        Rectangle { anchors.left: parent.left; anchors.top: parent.top; anchors.bottom: parent.bottom; width: 1; color: beautify.hair }

        Flickable {
            id: flick
            anchors.fill: parent
            anchors.leftMargin: 22
            anchors.rightMargin: 18
            anchors.topMargin: 20
            anchors.bottomMargin: 20
            contentHeight: pcol.height
            clip: true
            boundsBehavior: Flickable.StopAtBounds
            // native flick handles the mouse wheel and touchpad scroll; the panel
            // controls set preventStealing so a drag on a slider adjusts it instead
            // of scrolling.
            QQC.ScrollBar.vertical: QQC.ScrollBar {
                id: sb
                policy: QQC.ScrollBar.AsNeeded
                width: 7
                contentItem: Rectangle {
                    implicitWidth: 4
                    radius: 2
                    color: beautify.idle
                    opacity: sb.pressed ? 0.9 : (sb.hovered ? 0.7 : 0.4)
                    Behavior on opacity { NumberAnimation { duration: 120 } }
                }
            }

            Column {
                id: pcol
                width: flick.width - 14
                spacing: 24

                // ---------- PRESETS ----------
                Group {
                    title: I18n.tr("PRESETS")
                    Flow {
                        width: parent.width
                        spacing: 7
                        Repeater {
                            model: beautify.looks
                            Pill {
                                required property var modelData
                                label: modelData.name
                                onTapped: beautify.applyLook(modelData.cfg)
                            }
                        }
                    }
                    Row {
                        width: parent.width
                        spacing: 8
                        TopBtn {
                            label: beautify.hasDefault ? I18n.tr("\u2605 Update default") : I18n.tr("\u2605 Set as default")
                            onTapped: beautify.saveDefault()
                        }
                        TopBtn {
                            label: I18n.tr("Reset")
                            onTapped: beautify.resetDefault()
                        }
                    }
                }

                // ---------- BACKGROUND ----------
                Group {
                    title: I18n.tr("BACKGROUND")
                    Grid {
                        width: parent.width
                        columns: 6
                        columnSpacing: 8
                        rowSpacing: 8
                        Repeater {
                            model: beautify.presets
                            Rectangle {
                                id: cell
                                required property int index
                                required property var modelData
                                width: (parent.width - 5 * 8) / 6
                                height: 34
                                radius: 8
                                readonly property bool sel: beautify.bgKind === "preset" && beautify.bgPreset === cell.index
                                border.color: cell.sel ? Theme.ink : beautify.hair
                                border.width: cell.sel ? 2 : 1
                                gradient: Gradient {
                                    GradientStop { position: 0.0; color: cell.modelData.a }
                                    GradientStop { position: 1.0; color: cell.modelData.b }
                                }
                                MouseArea { anchors.fill: parent; onClicked: { beautify.bgKind = "preset"; beautify.bgPreset = cell.index; } }
                            }
                        }
                    }
                    Row {
                        width: parent.width
                        spacing: 8
                        BgType { label: I18n.tr("Solid"); on: beautify.bgKind === "solid"; onTapped: beautify.bgKind = "solid" }
                        BgType { label: I18n.tr("Gradient"); on: beautify.bgKind === "gradient"; onTapped: beautify.bgKind = "gradient" }
                        BgType { label: I18n.tr("Image"); on: beautify.bgKind === "image"; onTapped: { beautify.bgKind = "image"; beautify.pickImageRequested(); } }
                        BgType { label: I18n.tr("None"); on: beautify.bgKind === "none"; onTapped: beautify.bgKind = "none" }
                    }
                    ColorRow { visible: beautify.bgKind === "solid"; width: parent.width; current: beautify.bgSolid; onPicked: (c) => beautify.bgSolid = c }
                    Column {
                        visible: beautify.bgKind === "gradient"
                        width: parent.width
                        spacing: 10
                        ColorRow { width: parent.width; current: beautify.bgGradA; onPicked: (c) => beautify.bgGradA = c }
                        ColorRow { width: parent.width; current: beautify.bgGradB; onPicked: (c) => beautify.bgGradB = c }
                        Slider { width: parent.width; label: I18n.tr("Angle"); from: 0; to: 360; suffix: "\u00b0"; value: beautify.bgGradAngle; onMoved: (v) => beautify.bgGradAngle = v }
                    }
                    Text {
                        visible: beautify.bgKind === "image"
                        width: parent.width
                        wrapMode: Text.WordWrap
                        text: beautify.bgImagePath ? beautify.bgImagePath : I18n.tr("Click Image again to choose a file\u2026")
                        color: beautify.dim
                        font.family: Theme.ui
                        font.pixelSize: 11
                        elide: Text.ElideMiddle
                    }
                }

                // ---------- FRAME ----------
                Group {
                    title: I18n.tr("FRAME")
                    Slider { width: parent.width; label: I18n.tr("Padding"); from: 0; to: 240; value: beautify.padding; onMoved: (v) => beautify.padding = v }
                    Slider { width: parent.width; label: I18n.tr("Roundness"); from: 0; to: 80; value: beautify.roundness; onMoved: (v) => beautify.roundness = v }
                    Slider { width: parent.width; label: I18n.tr("Border"); from: 0; to: 16; value: beautify.borderW; onMoved: (v) => beautify.borderW = v }
                    ColorRow { visible: beautify.borderW > 0; width: parent.width; current: beautify.borderColor; onPicked: (c) => beautify.borderColor = c }
                }

                // ---------- SHADOW ----------
                Group {
                    title: I18n.tr("SHADOW")
                    Row {
                        width: parent.width
                        spacing: 16
                        Column {
                            width: parent.width - 56 - 16
                            spacing: 16
                            Slider { width: parent.width; label: I18n.tr("Strength"); from: 0; to: 100; suffix: "%"; value: beautify.shadowStrength; onMoved: (v) => beautify.shadowStrength = v }
                            Slider { width: parent.width; label: I18n.tr("Blur"); from: 0; to: 100; value: beautify.shadowBlur; onMoved: (v) => beautify.shadowBlur = v }
                            Slider { width: parent.width; label: I18n.tr("Distance"); from: 0; to: 80; value: beautify.shadowDist; onMoved: (v) => beautify.shadowDist = v }
                        }
                        Dial {
                            anchors.verticalCenter: parent.verticalCenter
                            label: I18n.tr("Angle")
                            angle: beautify.shadowAngle
                            onMoved: (a) => beautify.shadowAngle = a
                        }
                    }
                }

                // ---------- ADJUST ----------
                Group {
                    title: I18n.tr("ADJUST")
                    Slider { width: parent.width; label: I18n.tr("Brightness"); bipolar: true; from: -100; to: 100; value: beautify.adjBright; onMoved: (v) => beautify.adjBright = v }
                    Slider { width: parent.width; label: I18n.tr("Contrast"); bipolar: true; from: -100; to: 100; value: beautify.adjContrast; onMoved: (v) => beautify.adjContrast = v }
                    Slider { width: parent.width; label: I18n.tr("Saturation"); bipolar: true; from: -100; to: 100; value: beautify.adjSat; onMoved: (v) => beautify.adjSat = v }
                }

                // ---------- RATIO / SIZE ----------
                Group {
                    title: I18n.tr("RATIO / SIZE")
                    Flow {
                        width: parent.width
                        spacing: 6
                        Repeater {
                            model: beautify.ratioRow
                            Pill { label: modelData.l; on: beautify.ratioKey === modelData.k; onTapped: beautify.ratioKey = modelData.k }
                        }
                    }
                    Flow {
                        width: parent.width
                        spacing: 6
                        Repeater {
                            model: beautify.socialRow
                            Pill { label: modelData.l; on: beautify.ratioKey === modelData.k; onTapped: beautify.ratioKey = modelData.k }
                        }
                    }
                }

                // ---------- SHARE ----------
                Group {
                    title: I18n.tr("SHARE")
                    ToggleRow { width: parent.width; label: I18n.tr("Watermark (%1 handle)").arg(beautify.mark); on: beautify.watermark; onToggled: (v) => beautify.watermark = v }
                    ToggleRow { width: parent.width; label: I18n.tr("HD \u00d72 (AI upscale)"); on: beautify.hd; onToggled: (v) => beautify.hd = v }
                }
            }
        }
    }

    // ============================ canvas ============================
    Item {
        id: canvasArea
        anchors.left: parent.left
        anchors.right: panel.left
        anchors.top: topbar.bottom
        anchors.bottom: parent.bottom

        // transparency checker, shown behind the stage only when bg is None
        Rectangle {
            visible: beautify.bgKind === "none"
            width: stage.width * stage.scale
            height: stage.height * stage.scale
            anchors.centerIn: stage
            color: "#ffffff"
            Canvas {
                anchors.fill: parent
                onPaint: {
                    var ctx = getContext("2d");
                    var s = 12;
                    ctx.fillStyle = "#c8c8c8";
                    for (var y = 0; y < height; y += s)
                        for (var x = 0; x < width; x += s)
                            if (((x / s) + (y / s)) % 2 === 0) ctx.fillRect(x, y, s, s);
                }
                Component.onCompleted: requestPaint()
            }
        }

        Item {
            id: stage
            width: beautify.fullW
            height: beautify.fullH
            anchors.centerIn: parent
            anchors.verticalCenterOffset: -12
            scale: Math.min((canvasArea.width - 96) / beautify.fullW, (canvasArea.height - 120) / beautify.fullH, 1)
            transformOrigin: Item.Center

            // background
            Item {
                anchors.fill: parent
                visible: beautify.bgKind !== "none"
                clip: true
                Rectangle {
                    visible: beautify.bgIsGradient
                    anchors.centerIn: parent
                    width: Math.max(parent.width, parent.height) * 1.5
                    height: width
                    rotation: beautify.bgAngle - 90
                    gradient: Gradient {
                        GradientStop { position: 0.0; color: beautify.bgA }
                        GradientStop { position: 1.0; color: beautify.bgB }
                    }
                }
                Rectangle { visible: beautify.bgKind === "solid"; anchors.fill: parent; color: beautify.bgSolid }
                Image { visible: beautify.bgKind === "image" && beautify.bgImagePath !== ""; anchors.fill: parent; source: beautify.bgImagePath ? ("file://" + beautify.bgImagePath) : ""; fillMode: Image.PreserveAspectCrop; cache: false }
            }

            // the framed capture
            Item {
                id: card
                anchors.centerIn: parent
                width: beautify.natW
                height: beautify.natH

                // directional blurred shadow, cast from the rounded card shape
                Rectangle {
                    width: parent.width; height: parent.height
                    x: Math.cos(beautify.shadowAngle * Math.PI / 180) * beautify.shadowDist
                    y: Math.sin(beautify.shadowAngle * Math.PI / 180) * beautify.shadowDist
                    radius: beautify.roundness
                    color: "#000000"
                    visible: beautify.shadowStrength > 0
                    opacity: beautify.shadowStrength / 100
                    layer.enabled: true
                    layer.effect: MultiEffect {
                        blurEnabled: true
                        blur: 1.0
                        blurMax: Math.round(beautify.shadowBlur * 0.64)
                        autoPaddingEnabled: true
                    }
                }

                // the screenshot, rounded via a mask (no shadow/padding here, so
                // the mask stays aligned), plus the colour adjust
                Image {
                    id: img
                    anchors.fill: parent
                    source: beautify.srcPath ? ("file://" + beautify.srcPath) : ""
                    cache: false
                    fillMode: Image.Stretch
                    layer.enabled: true
                    layer.effect: MultiEffect {
                        maskEnabled: beautify.roundness > 0
                        maskSource: maskRect
                        brightness: beautify.adjBright / 100
                        contrast: beautify.adjContrast / 100
                        saturation: beautify.adjSat / 100
                    }
                }
                Rectangle {
                    id: maskRect
                    anchors.fill: parent
                    radius: beautify.roundness
                    visible: false
                    layer.enabled: true
                }
                // coloured border, tracing the same rounded shape
                Rectangle {
                    anchors.fill: parent
                    visible: beautify.borderW > 0
                    color: "transparent"
                    radius: beautify.roundness
                    border.width: beautify.borderW
                    border.color: beautify.borderColor
                }
            }

            // watermark
            Row {
                id: wmark
                visible: beautify.watermark
                anchors.right: parent.right
                anchors.bottom: parent.bottom
                anchors.margins: Math.max(16, beautify.padding * 0.32)
                spacing: wmark.fs * 0.55
                readonly property real fs: Math.max(15, beautify.fullW * 0.016)

                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: beautify.mark
                    color: beautify.vermilion
                    font.family: "Noto Sans CJK JP"
                    font.pixelSize: wmark.fs * 1.4
                    font.weight: Font.Bold
                    style: Text.Raised
                    styleColor: Qt.rgba(0, 0, 0, 0.5)
                }
                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: "<b>" + beautify.userName + "</b><font color=\"#f5b53f\">@RyokuArch</font>"
                    textFormat: Text.StyledText
                    color: "#ffffff"
                    font.family: Theme.ui
                    font.pixelSize: wmark.fs
                    font.weight: Font.DemiBold
                    style: Text.Raised
                    styleColor: Qt.rgba(0, 0, 0, 0.45)
                }
            }
        }

        Text {
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.bottom: parent.bottom
            anchors.bottomMargin: 16
            text: Math.round(beautify.fullW) + " \u00d7 " + Math.round(beautify.fullH) + "  \u00b7  " + Math.round(stage.scale * 100) + "%"
            color: beautify.dim
            font.family: "JetBrainsMono Nerd Font"
            font.pixelSize: 12
        }
    }

    // ============================ inline widgets ============================
    component Group: Column {
        id: grp
        width: parent ? parent.width : 0
        property string title: ""
        default property alias content: body.data
        spacing: 14
        Item {
            width: grp.width
            height: 14
            Text {
                id: gh
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                text: grp.title
                color: beautify.dim
                font.family: Theme.ui
                font.pixelSize: 11
                font.weight: Font.DemiBold
                font.letterSpacing: 2
            }
            Rectangle { anchors.left: gh.right; anchors.leftMargin: 12; anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter; height: 1; color: beautify.hair }
        }
        Column { id: body; width: grp.width; spacing: 14 }
    }

    component TopBtn: Rectangle {
        id: tbn
        property string label: ""
        property bool accent: false
        signal tapped()
        implicitWidth: tl.implicitWidth + 32
        implicitHeight: 36
        radius: 9
        color: tbn.accent ? (tbnMa.containsMouse ? Qt.lighter(beautify.vermilion, 1.12) : beautify.vermilion) : (tbnMa.containsMouse ? Qt.rgba(1, 1, 1, 0.08) : "transparent")
        border.width: tbn.accent ? 0 : 1
        border.color: beautify.hair
        Text { id: tl; anchors.centerIn: parent; text: tbn.label; color: tbn.accent ? Theme.accentInk : beautify.idle; font.family: Theme.ui; font.pixelSize: 13; font.weight: Font.DemiBold }
        MouseArea { id: tbnMa; anchors.fill: parent; hoverEnabled: true; onClicked: tbn.tapped() }
    }

    component BgType: Rectangle {
        id: bgt
        property string label: ""
        property bool on: false
        signal tapped()
        width: (parent.width - 3 * 8) / 4
        height: 30
        radius: 8
        color: bgt.on ? beautify.vermilion : (btMa.containsMouse ? Qt.rgba(1, 1, 1, 0.07) : beautify.fieldBg)
        Text { anchors.centerIn: parent; text: bgt.label; color: bgt.on ? Theme.accentInk : beautify.idle; font.family: Theme.ui; font.pixelSize: 12; font.weight: bgt.on ? Font.DemiBold : Font.Medium }
        MouseArea { id: btMa; anchors.fill: parent; hoverEnabled: true; onClicked: bgt.tapped() }
    }

    component Pill: Rectangle {
        id: pill
        property string label: ""
        property bool on: false
        signal tapped()
        implicitWidth: pl.implicitWidth + 22
        height: 28
        radius: 8
        color: pill.on ? beautify.vermilion : (plMa.containsMouse ? Qt.rgba(1, 1, 1, 0.07) : beautify.fieldBg)
        Text { id: pl; anchors.centerIn: parent; text: pill.label; color: pill.on ? Theme.accentInk : beautify.idle; font.family: Theme.ui; font.pixelSize: 12; font.weight: pill.on ? Font.DemiBold : Font.Medium }
        MouseArea { id: plMa; anchors.fill: parent; hoverEnabled: true; onClicked: pill.tapped() }
    }

    component ColorRow: Flow {
        id: cr
        property color current: "#ffffff"
        signal picked(color c)
        spacing: 7
        Repeater {
            model: beautify.palette
            Rectangle {
                id: sw
                required property var modelData
                width: 22
                height: 22
                radius: 6
                color: sw.modelData
                readonly property bool sel: Qt.colorEqual(cr.current, sw.modelData)
                border.color: sw.sel ? Theme.ink : beautify.hair
                border.width: sw.sel ? 2 : 1
                scale: crMa.containsMouse ? 1.12 : 1
                Behavior on scale { NumberAnimation { duration: 80 } }
                MouseArea { id: crMa; anchors.fill: parent; hoverEnabled: true; onClicked: cr.picked(sw.modelData) }
            }
        }
    }

    component ToggleRow: Item {
        id: tr
        property string label: ""
        property bool on: false
        signal toggled(bool v)
        width: parent.width
        height: 28
        Text { anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter; text: tr.label; color: beautify.idle; font.family: Theme.ui; font.pixelSize: 13; font.weight: Font.Medium }
        Rectangle {
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            width: 42
            height: 24
            radius: 12
            color: tr.on ? beautify.vermilion : beautify.fieldBg
            border.width: 1
            border.color: tr.on ? beautify.vermilion : beautify.hair
            Rectangle {
                width: 18
                height: 18
                radius: 9
                y: 3
                x: tr.on ? parent.width - width - 3 : 3
                color: Theme.ink
                Behavior on x { NumberAnimation { duration: 120; easing.type: Easing.OutCubic } }
            }
            MouseArea { anchors.fill: parent; onClicked: tr.toggled(!tr.on) }
        }
    }
}
