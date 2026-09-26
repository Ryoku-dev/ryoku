pragma ComponentBehavior: Bound
import QtQuick
import Quickshell
import "Singletons"
import Ryoku.Ui.Singletons
// namespaced: shell.services also has a Config, and an unqualified import of it
// shadowed this menu's own, leaving every widget toggle reading undefined.
import shell.services as Services
import "../visualizer/Singletons" as VizCfg
import "../stage/Singletons" as StageCfg

// The desktop right-click menu, built on the shared DesktopMenu chrome in the
// quick-settings sidebar idiom. Two scopes:
//   right-click bare desktop = desktop menu (show/hide the clock, settings,
//     reload)
//   right-click a widget     = its menu (cycle design, toggle date, lock, snap
//     to a compass zone, hide) + the same globals
// Every action writes the same widgets Config the drag and Ryoku Settings do.
Item {
    id: menu

    anchors.fill: parent

    property string scope: "desktop"   // desktop | clock
    // The owning Desktop surface. The menu's actions target the screen it was
    // opened on; the compositor's focused output can be another monitor (or
    // resolve to no slice at all), which made the editors a silent no-op or
    // opened them on the wrong screen.
    property var desktop: null

    readonly property bool isWidget: menu.scope !== "desktop"
    readonly property bool isClock: menu.scope === "clock"
    readonly property bool isCalendar: menu.scope === "calendar"
    readonly property bool isMusic: menu.scope === "music"
    readonly property bool isAio: menu.scope === "aio"
    readonly property bool isStats: menu.scope === "stats"
    readonly property bool isWeather: menu.scope === "weather"
    readonly property bool isNotes: menu.scope === "notes"
    // only offer to place the spectrum when it is actually running
    readonly property bool vizOn: VizCfg.Config.enabled
    readonly property bool locked: menu.isWidget ? Config[menu.scope + "Locked"] : false
    readonly property string curAnchor: menu.isWidget ? Config[menu.scope + "Anchor"] : ""
    // clock faces persist as <scope>Design; the calendar and the music sheet
    // persist their look as <scope>Style.
    readonly property string designKey: menu.isCalendar || menu.isMusic || menu.isAio
        ? menu.scope + "Style" : menu.scope + "Design"
    // A widget with one look (notes, stats) has no design key at all, so the
    // lookup must resolve to a string rather than undefined.
    readonly property string curDesign: menu.isWidget ? (Config[menu.designKey] ?? "") : ""
    // ink colour of the open widget: "" follows the wallpaper (Auto), a hex is a
    // solid, and Gradient blends <scope>Color -> <scope>Color2 across the glyphs.
    readonly property string curColor: menu.isWidget ? (Config[menu.scope + "Color"] || "") : ""
    readonly property bool curGradient: menu.isWidget ? (Config[menu.scope + "Gradient"] === true) : false
    readonly property string colorMode: menu.curColor === "" ? "auto" : (menu.curGradient ? "gradient" : "solid")

    readonly property var zones: [
        { "zone": "top-left", "glyph": "\u2196" }, { "zone": "top", "glyph": "\u2191" }, { "zone": "top-right", "glyph": "\u2197" },
        { "zone": "left", "glyph": "\u2190" }, { "zone": "center", "glyph": "\u2299" }, { "zone": "right", "glyph": "\u2192" },
        { "zone": "bottom-left", "glyph": "\u2199" }, { "zone": "bottom", "glyph": "\u2193" }, { "zone": "bottom-right", "glyph": "\u2198" }
    ]

    // the kanji seal each scope carries in the menu masthead (docs/ui-ux.md).
    readonly property var glosses: ({
        "desktop": "卓上",
        "clock": "時計",
        "calendar": "暦",
        "music": "音楽",
        "aio": "一体",
        "stats": "計測",
        "weather": "天気",
        "notes": "筆記"
    })

    function openFor(widget, x, y) { menu.scope = widget; shell.px = x; shell.py = y; shell.open = true; }
    function openDesktop(x, y) { menu.scope = "desktop"; shell.px = x; shell.py = y; shell.open = true; }
    function close() { shell.open = false; }
    function cap(s) { return s.length > 0 ? s.charAt(0).toUpperCase() + s.slice(1) : s; }

    function cycleDesign() {
        const lists = {
            clock: ["digital", "minimal", "grand", "column", "outline", "banner", "analog", "flip", "rings", "bighour", "metal", "goodnight"],
            calendar: ["glass", "paper"],
            music: ["cover", "glass"],
            aio: ["wide", "tall"],
            weather: ["compact", "full"]
        };
        const d = lists[menu.scope];
        if (!d)
            return;
        Config.set(menu.designKey, d[(d.indexOf(Config[menu.designKey]) + 1) % d.length]);
    }
    // "#RRGGBB" for a colour, the syntax widgets.json stores.
    function hexOf(c) {
        return "#" + [c.r, c.g, c.b].map(function (x) {
            const s = Math.round(x * 255).toString(16);
            return s.length === 1 ? "0" + s : s;
        }).join("").toUpperCase();
    }
    // Auto clears the pin; Solid/Gradient need a base, so seed the wallpaper
    // accent (and a lighter twin for the second stop) when coming from Auto.
    function setColorMode(m) {
        if (m === "auto") {
            Config.set(menu.scope + "Color", "");
            Config.set(menu.scope + "Gradient", false);
            return;
        }
        if (menu.curColor === "")
            Config.set(menu.scope + "Color", menu.hexOf(Scheme.accent));
        if (m === "gradient") {
            if ((Config[menu.scope + "Color2"] || "") === "")
                Config.set(menu.scope + "Color2", menu.hexOf(Qt.lighter(Scheme.accent, 1.5)));
            Config.set(menu.scope + "Gradient", true);
        } else {
            Config.set(menu.scope + "Gradient", false);
        }
    }
    function openSettings() {
        Spawn.run(["sh", "-c", "ryoku-hub config set section widgets; flock -n -o /tmp/ryoku-hub.lock qs -c hub"]);
        menu.close();
    }
    function refreshShell() {
        Quickshell.execDetached(["ryoku-shell", "reload"]);
        menu.close();
    }
    function videoLabel(v) { return v === "canvas" ? "Spotify Canvas" : v === "custom" ? "Custom" : "Off"; }
    function videoName(p) {
        if (!p || p.length === 0)
            return "None";
        const s = ("" + p).replace(/\/+$/, "");
        return decodeURIComponent(s.slice(s.lastIndexOf("/") + 1));
    }
    function cycleVideo() {
        const d = ["off", "canvas", "custom"];
        Config.set("musicVideo", d[(d.indexOf(Config.musicVideo) + 1) % d.length]);
    }
    // The three editors and the visualizer's own editor (docs/stage.md, "The
    // desktop right-click menu"). Sessions open on the monitor the menu is on:
    // the owning desktop's screen, falling back to the focused output only if
    // the menu was built without one.
    function targetState() {
        return menu.desktop ? menu.desktop.stageState : Services.ShellState.forActive();
    }
    function activeMonitor() {
        const st = menu.targetState();
        return (st && st.modelData) ? st.modelData.name : "";
    }
    function editWidgets() {
        StageCfg.StageSession.enterWidgets(menu.activeMonitor());
        menu.close();
    }
    // Every Depth and Parallax setting lives on the Stage tab of Super+Esc;
    // "#stage" deep-links the panel there (FrameMenuManager.openSurface).
    function depthSettings() {
        Services.ShellState.requestSurfaceActive("quick-settings#stage", undefined);
        menu.close();
    }
    function customizeVisualizer() {
        const st = menu.targetState();
        if (!st)
            return;
        if (!VizCfg.Config.enabled)
            VizCfg.Config.setEnabled(true);
        st.visualizerPlacing = true;
        menu.close();
    }
    // Depth off drops Parallax with it; Parallax on turns Depth on with it.
    readonly property string stageEffect: StageCfg.StageBackend.effect
    readonly property bool stageBusy: StageCfg.StageBackend.busy
    readonly property int stagePct: StageCfg.StageBackend.percent
    function toggleDepth() {
        StageCfg.StageBackend.setEffect(menu.stageEffect === "off" ? "depth" : "off");
    }
    function toggleParallax() {
        StageCfg.StageBackend.setEffect(menu.stageEffect === "parallax" ? "depth" : "parallax");
    }
    function changeWallpaper() {
        Services.ShellState.requestSurfaceActive("wallpaper", null);
        menu.close();
    }

    DesktopMenu {
        id: shell
        title: menu.scope
        gloss: menu.glosses[menu.scope] || ""

        // ── desktop scope ──────────────────────────────────────────────
        MenuRow {
            visible: !menu.isWidget
            label: I18n.tr("Edit widgets")
            onTriggered: menu.editWidgets()
        }
        MenuRow {
            visible: !menu.isWidget
            label: I18n.tr("Customize visualizer")
            onTriggered: menu.customizeVisualizer()
        }
        MenuRow {
            visible: !menu.isWidget
            label: I18n.tr("Change wallpaper")
            closeOnTrigger: false
            onTriggered: menu.changeWallpaper()
        }

        // ── widget scope ───────────────────────────────────────────────
        MenuRow {
            visible: menu.isWidget && !menu.isStats && !menu.isNotes
            label: I18n.tr("Design")
            value: menu.cap(menu.curDesign)
            closeOnTrigger: false
            onTriggered: menu.cycleDesign()
        }
        MenuRow {
            visible: menu.isClock
            label: I18n.tr("Date")
            value: Config.dateShow ? "On" : "Off"
            on: Config.dateShow
            closeOnTrigger: false
            onTriggered: Config.toggle("dateShow")
        }
        MenuRow {
            visible: menu.isMusic
            label: I18n.tr("Lyrics")
            value: Config.musicLyrics ? "On" : "Off"
            on: Config.musicLyrics
            closeOnTrigger: false
            onTriggered: Config.toggle("musicLyrics")
        }
        MenuRow {
            visible: menu.isMusic
            label: I18n.tr("Visualiser")
            value: Config.musicViz === "wave" ? "Wave" : "Bars"
            on: Config.musicViz === "wave"
            closeOnTrigger: false
            onTriggered: Config.set("musicViz", Config.musicViz === "wave" ? "bars" : "wave")
        }
        MenuRow {
            visible: menu.isMusic
            label: I18n.tr("Canvas")
            value: Config.musicShape === "tall" ? "9:16" : "Wide"
            on: Config.musicShape === "tall"
            closeOnTrigger: false
            onTriggered: Config.set("musicShape", Config.musicShape === "tall" ? "wide" : "tall")
        }
        MenuRow {
            visible: menu.isMusic
            label: I18n.tr("Backdrop")
            value: menu.videoLabel(Config.musicVideo)
            on: Config.musicVideo !== "off"
            closeOnTrigger: false
            onTriggered: menu.cycleVideo()
        }
        MenuRow {
            visible: menu.isMusic
            label: I18n.tr("Video / GIF…")
            value: menu.videoName(Config.musicVideoFile)
            onTriggered: videoPicker.open = true
        }
        MenuRow {
            visible: menu.isWidget
            label: I18n.tr("Lock")
            value: menu.locked ? "On" : "Off"
            on: menu.locked
            closeOnTrigger: false
            onTriggered: Config.toggle(menu.scope + "Locked")
        }

        // Size + Opacity: discoverable equivalents of the corner-drag resize and
        // Ryoku Settings' opacity, targeting whichever widget the menu is open
        // for. Both scrub live and persist on release.
        MenuSection { visible: menu.isWidget; label: I18n.tr("Adjust"); gloss: "調整" }
        MenuSlider {
            id: sizeSlider
            visible: menu.isWidget
            label: I18n.tr("Size")
            from: 0.5
            to: 2.5
            step: 0.02
            value: menu.isWidget ? Config[menu.scope + "Scale"] : 1
            valueText: Math.round(sizeSlider.value * 100) + "%"
            onMoved: (v) => Config.setLive(menu.scope + "Scale", v)
            onReleased: (v) => Config.set(menu.scope + "Scale", v)
        }
        MenuSlider {
            id: opacitySlider
            visible: menu.isWidget
            label: I18n.tr("Opacity")
            from: 0.2
            to: 1.0
            step: 0.01
            value: menu.isWidget ? Config[menu.scope + "Opacity"] : 1
            valueText: Math.round(opacitySlider.value * 100) + "%"
            onMoved: (v) => Config.setLive(menu.scope + "Opacity", v)
            onReleased: (v) => Config.set(menu.scope + "Opacity", v)
        }

        // Colour: the widget's ink follows the wallpaper (Auto), a solid pin, or
        // an A->B gradient -- the same model the visualiser wears. Solid/Gradient
        // reveal a compact picker; edits scrub live and persist on release.
        MenuSection { visible: menu.isWidget; label: I18n.tr("Colour"); gloss: "彩色" }
        Item {
            visible: menu.isWidget
            width: parent.width
            implicitHeight: menu.isWidget ? colorCol.implicitHeight : 0
            Column {
                id: colorCol
                width: parent.width
                spacing: Theme.s1
                Row {
                    id: modeRow
                    width: parent.width
                    spacing: Theme.s1
                    readonly property real cw: (width - 2 * Theme.s1) / 3
                    MenuChip {
                        width: modeRow.cw; height: Theme.ctlH
                        label: I18n.tr("Auto"); selected: menu.colorMode === "auto"
                        onClicked: menu.setColorMode("auto")
                    }
                    MenuChip {
                        width: modeRow.cw; height: Theme.ctlH
                        label: I18n.tr("Solid"); selected: menu.colorMode === "solid"
                        onClicked: menu.setColorMode("solid")
                    }
                    MenuChip {
                        width: modeRow.cw; height: Theme.ctlH
                        label: I18n.tr("Gradient"); selected: menu.colorMode === "gradient"
                        onClicked: menu.setColorMode("gradient")
                    }
                }
                MenuColorPicker {
                    width: parent.width
                    visible: menu.colorMode !== "auto"
                    scope: menu.scope
                    gradient: menu.colorMode === "gradient"
                }
            }
        }

        MenuSection { visible: menu.isWidget; label: I18n.tr("Snap"); gloss: "位置" }

        // One placement control: an Auto lane above a square 3x3 compass whose
        // centre cell is the centre zone. Auto lands the widget on the
        // wallpaper's calmest region and re-follows every wallpaper change; Free
        // is the implicit dragged state, so it has no cell of its own.
        Item {
            visible: menu.isWidget
            width: parent.width
            implicitHeight: menu.isWidget ? placer.implicitHeight : 0
            Column {
                id: placer
                anchors.horizontalCenter: parent.horizontalCenter
                spacing: Theme.s1
                readonly property real cellSize: Theme.s6
                readonly property real span: placer.cellSize * 3 + Theme.s1 * 2
                MenuChip {
                    label: I18n.tr("Auto")
                    width: placer.span
                    height: Theme.ctlH
                    selected: menu.curAnchor === "auto"
                    onClicked: { Config.setAnchor(menu.scope, "auto"); menu.close(); }
                }
                Grid {
                    columns: 3
                    spacing: Theme.s1
                    Repeater {
                        model: menu.zones
                        MenuChip {
                            id: zoneCell
                            required property var modelData
                            width: placer.cellSize
                            height: placer.cellSize
                            selected: menu.curAnchor === zoneCell.modelData.zone
                            onClicked: { Config.setAnchor(menu.scope, zoneCell.modelData.zone); menu.close(); }
                            Text {
                                anchors.centerIn: parent
                                text: zoneCell.modelData.glyph
                                color: zoneCell.contentColor
                                font.family: Theme.font
                                font.pixelSize: Theme.fSmall
                            }
                        }
                    }
                }
            }
        }

        MenuRow {
            visible: menu.isWidget
            label: I18n.tr("Hide")
            onTriggered: Config.set(menu.scope + "Enabled", false)
        }

        // ── the two Stage switches, side by side (docs/stage.md) ────────
        // The menu's own choice chips: a bone plate when the effect is on, a
        // quiet tile when off, the state spelled out in the label.
        MenuSection {}
        Row {
            id: stageRow
            visible: !menu.isWidget
            width: parent.width
            spacing: Theme.s1
            readonly property real cw: (width - Theme.s1) / 2
            MenuChip {
                width: stageRow.cw
                height: Theme.ctlH + 6
                selected: menu.stageEffect !== "off"
                label: I18n.tr("Depth") + " \u00b7 " + (menu.stageBusy ? (menu.stagePct + "%")
                    : menu.stageEffect !== "off" ? I18n.tr("On") : I18n.tr("Off"))
                onClicked: menu.toggleDepth()
            }
            MenuChip {
                width: stageRow.cw
                height: Theme.ctlH + 6
                selected: menu.stageEffect === "parallax"
                label: I18n.tr("Parallax") + " \u00b7 " + (menu.stageEffect === "parallax" ? I18n.tr("On") : I18n.tr("Off"))
                onClicked: menu.toggleParallax()
            }
        }

        MenuRow {
            visible: !menu.isWidget
            label: I18n.tr("Depth settings…")
            onTriggered: menu.depthSettings()
        }

        // ── globals ────────────────────────────────────────────────────
        MenuSection {}
        MenuRow { label: I18n.tr("Settings"); accent: true; closeOnTrigger: false; onTriggered: menu.openSettings() }
        MenuRow { label: I18n.tr("Reload shell"); closeOnTrigger: false; onTriggered: menu.refreshShell() }
    }

    MusicVideoPicker {
        id: videoPicker
        onChose: (url) => {
            Config.set("musicVideoFile", url);
            Config.set("musicVideo", "custom");
        }
    }
}
