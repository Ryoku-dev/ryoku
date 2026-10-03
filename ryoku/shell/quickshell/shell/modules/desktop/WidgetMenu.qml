pragma ComponentBehavior: Bound
import QtQuick
import Quickshell
import "Singletons"
import Ryoku.Ui.Singletons
import "iris/IrisRoster.js" as IrisRoster
import "python/PythonRoster.js" as PythonRoster
import "../stage/Singletons" as StageCfg

// A desktop widget's right-click menu, built on the shared DesktopMenu chrome in
// the quick-settings sidebar idiom: a short card that names the widget, offers
// its quick knobs -- Style, Size, Lock -- then a primary Customize row that opens
// the widget inspector for everything else (colour, shape, snap and the widget's
// own options), Hide, and the shell globals (Settings, Reload). The full control
// set moved to WidgetInspector so this menu stays short; both write the same
// widgets Config the drag and Ryoku Settings do. The bare-wallpaper right-click
// is a separate surface, DesktopContextMenu.
Item {
    id: menu

    anchors.fill: parent

    // Exposed so the host menu surface maps only while this menu is on screen.
    readonly property bool showing: shell.showing

    // Raised when Customize is chosen; the desktop layer opens the inspector.
    signal customizeRequested(string widget)

    property string scope: "desktop"   // desktop | clock | ...
    // The wallpaper of the monitor whose right-click opened this menu; the
    // Depth row gates on it, so the lift is offered only where the scene has
    // cut-outs. Set by the owning desktop with openFor.
    property string wall: ""

    readonly property bool isWidget: menu.scope !== "desktop"
    readonly property bool isStats: menu.scope === "stats"
    readonly property bool isNotes: menu.scope === "notes"
    readonly property bool isCalendar: menu.scope === "calendar"
    readonly property bool isMusic: menu.scope === "music"
    readonly property bool isAio: menu.scope === "aio"
    readonly property bool isDayprogress: menu.scope === "dayprogress"
    readonly property bool isShape: menu.scope === "shape"
    readonly property bool locked: menu.isWidget ? Config[menu.scope + "Locked"] : false
    // The stage lift (docs/stage.md): Depth offers this widget a place above
    // every in-front cut-out; the row only shows while the wall cuts a subject.
    readonly property bool stageActive: menu.wall !== ""
        && StageCfg.StageBackend.isActiveFor(menu.wall)
    readonly property bool lifted: StageCfg.Config.isFront(menu.scope)

    // clock faces persist as <scope>Design; the calendar and the music sheet
    // persist their look as <scope>Style; shape as <scope>Kind.
    readonly property string designKey: menu.isCalendar || menu.isMusic || menu.isAio || menu.isDayprogress
        ? menu.scope + "Style"
        : menu.isShape ? menu.scope + "Kind"
        : menu.scope + "Design"
    readonly property string curDesign: menu.isWidget ? (Config[menu.designKey] ?? "") : ""

    // Vendored face libraries hosted in Ryoku's slot: roster metadata, their
    // upstream/Ryoku skin toggle, and the face-specific preset ladder.
    readonly property var irisFace: IrisRoster.byPrefix(menu.scope)
    readonly property bool isIris: menu.irisFace !== null
    readonly property var pythonFace: PythonRoster.byPrefix(menu.scope)
    readonly property bool isPython: menu.pythonFace !== null
    readonly property var hostedFace: menu.isIris ? menu.irisFace : menu.pythonFace
    readonly property bool isRyokuStyle: (menu.isIris || menu.isPython)
        && Config[menu.scope + "Style"] === "ryoku"
    readonly property string curIrisSize: menu.isIris ? (Config[menu.scope + "Size"] || menu.irisFace.sizes[0]) : ""
    readonly property string curPythonVariant: menu.isPython ? (Config[menu.scope + "Variant"] || menu.pythonFace.variants[0]) : ""
    readonly property var facePresets: menu.isIris ? menu.irisFace.sizes
        : menu.isPython ? menu.pythonFace.variants : []
    readonly property string facePresetKey: menu.scope + (menu.isPython ? "Variant" : "Size")
    readonly property string curFacePreset: menu.isPython ? menu.curPythonVariant : menu.curIrisSize

    // the kanji seal each scope carries in the menu masthead (docs/ui-ux.md).
    readonly property var glosses: ({
        "desktop": "卓上",
        "clock": "時計",
        "calendar": "暦",
        "music": "音楽",
        "aio": "一体",
        "stats": "計測",
        "weather": "天気",
        "notes": "筆記",
        "dayprogress": "経過",
        "shape": "図形"
    })

    // Human names for the built-in scopes; the masthead never shows a raw key.
    readonly property var builtinLabels: ({
        clock: "Clock", calendar: "Calendar", music: "Music", aio: "All-in-one",
        stats: "System stats", weather: "Weather", notes: "Notes",
        dayprogress: "Day Progress", shape: "Shape"
    })

    // the widgets that cycle through a set of looks from the quick Style row; the
    // rest (stats, notes) have a single look and skip the row.
    readonly property var designLists: ({
        clock: ["digital", "minimal", "grand", "column", "outline", "banner", "analog", "flip", "rings", "bighour", "metal", "goodnight"],
        calendar: ["glass", "paper"],
        music: ["cover", "glass"],
        aio: ["wide", "tall"],
        weather: ["compact", "full"],
        dayprogress: ["ring", "arc"],
        shape: ["dot", "ring", "diamond", "square"]
    })
    readonly property bool hasDesign: menu.isWidget && !menu.isIris && !menu.isPython
        && (menu.designLists[menu.scope] !== undefined)

    // Size quick-nudge: a small scale ladder every widget shares. Fine control
    // (and the iRiS size preset) lives on the inspector's Look tab.
    readonly property real curScale: menu.isWidget ? (Config[menu.scope + "Scale"] || 1) : 1

    function openFor(widget, x, y, wall) { menu.scope = widget; menu.wall = wall; shell.px = x; shell.py = y; shell.open = true; }
    function close() { shell.open = false; }
    function cap(s) { return s.length > 0 ? s.charAt(0).toUpperCase() + s.slice(1) : s; }

    function cycleDesign() {
        const d = menu.designLists[menu.scope];
        if (!d)
            return;
        Config.set(menu.designKey, d[(d.indexOf(Config[menu.designKey]) + 1) % d.length]);
    }
    function cycleFacePreset() {
        const presets = menu.facePresets;
        if (!presets || presets.length < 2)
            return;
        const current = presets.indexOf(menu.curFacePreset);
        Config.set(menu.facePresetKey, presets[(current + 1) % presets.length]);
    }
    function cycleScale() {
        const d = [0.75, 1.0, 1.25, 1.5, 2.0];
        var n = d.find(v => v > menu.curScale + 0.001);
        if (n === undefined)
            n = d[0];
        Config.set(menu.scope + "Scale", n);
    }
    function openSettings() {
        Spawn.run(["sh", "-c", "ryoku-hub config set section widgets; flock -n -o /tmp/ryoku-hub.lock qs -c hub"]);
        menu.close();
    }
    function refreshShell() {
        Quickshell.execDetached(["ryoku-shell", "reload"]);
        menu.close();
    }

    DesktopMenu {
        id: shell
        title: (menu.isIris || menu.isPython) ? I18n.tr(menu.hostedFace.label)
            : I18n.tr(menu.builtinLabels[menu.scope] || menu.cap(menu.scope))
        gloss: menu.glosses[menu.scope] || ((menu.isIris || menu.isPython) ? menu.hostedFace.gloss : "")

        // ── quick knobs ────────────────────────────────────────────────
        // Native widgets cycle their look; hosted faces flip between their
        // upstream skin and Ryoku's backing. Everything finer opens Customize.
        MenuRow {
            visible: menu.hasDesign
            label: I18n.tr("Style")
            value: menu.cap(menu.curDesign)
            closeOnTrigger: false
            onTriggered: menu.cycleDesign()
        }
        MenuRow {
            visible: menu.isIris || menu.isPython
            label: I18n.tr("Style")
            value: menu.isRyokuStyle ? "Ryoku" : "Original"
            on: menu.isRyokuStyle
            closeOnTrigger: false
            onTriggered: Config.set(menu.scope + "Style",
                menu.isRyokuStyle ? (menu.isPython ? "serp" : "inir") : "ryoku")
        }
        MenuRow {
            visible: (menu.isIris || menu.isPython) && menu.facePresets.length > 1
            label: menu.isPython ? I18n.tr("Variant") : I18n.tr("Face size")
            value: menu.cap(menu.curFacePreset)
            closeOnTrigger: false
            onTriggered: menu.cycleFacePreset()
        }
        MenuRow {
            visible: menu.isWidget
            label: I18n.tr("Size")
            value: Math.round(menu.curScale * 100) + "%"
            closeOnTrigger: false
            onTriggered: menu.cycleScale()
        }
        MenuRow {
            visible: menu.isWidget
            label: I18n.tr("Lock")
            value: menu.locked ? "On" : "Off"
            on: menu.locked
            closeOnTrigger: false
            onTriggered: Config.toggle(menu.scope + "Locked")
        }
        MenuRow {
            visible: menu.isWidget && menu.stageActive
            label: I18n.tr("Depth")
            value: menu.lifted ? I18n.tr("In front") : I18n.tr("Behind")
            on: menu.lifted
            closeOnTrigger: false
            onTriggered: StageCfg.Config.setFront(menu.scope, !menu.lifted)
        }

        // ── customize ──────────────────────────────────────────────────
        // The primary action: the widget inspector docks beside the widget with
        // its colour, shape, placement and own option groups on tabs.
        MenuRow {
            visible: menu.isWidget
            label: I18n.tr("Customize…")
            icon: "tune"
            accent: true
            onTriggered: menu.customizeRequested(menu.scope)
        }
        MenuRow {
            visible: menu.isWidget
            label: I18n.tr("Hide")
            onTriggered: Config.set(menu.scope + "Enabled", false)
        }

        // ── globals ────────────────────────────────────────────────────
        MenuSection {}
        MenuRow { label: I18n.tr("Settings"); closeOnTrigger: false; onTriggered: menu.openSettings() }
        MenuRow { label: I18n.tr("Reload shell"); closeOnTrigger: false; onTriggered: menu.refreshShell() }
    }
}
