pragma ComponentBehavior: Bound
import QtQuick
import Quickshell
import Ryoku.Ui.Singletons
import shell.services as Services
import "../visualizer/Singletons" as VizCfg
import "../stage/Singletons" as StageCfg
import inir

// The system desktop right-click menu: right-clicking bare wallpaper on any bar
// style opens this. It carries the iRiS desktop menu's structure -- a quick row
// of icon tiles (wallpaper thumbnail, widgets, visualiser, search) over the
// desktop's own actions -- in Ryoku's paper-and-ink language, on the shared
// DesktopMenu chrome. Every action targets the screen the menu opened on (the
// owning Desktop's slice), never the focused output, which can be a different
// monitor. The per-widget menu (WidgetMenu) and plugin menu are separate.
Item {
    id: menu

    anchors.fill: parent

    // Exposed so the host menu surface maps only while this menu is on screen.
    readonly property alias showing: shell.showing

    // The owning Desktop surface: its screen slice is where the editors and the
    // launcher open, and its wallpaper feeds the Wallpaper tile's thumbnail.
    property var desktop: null

    function openAt(x, y) { shell.px = x; shell.py = y; shell.open = true; }
    function close() { shell.open = false; }

    // The three editors and the launcher open on the monitor the menu is on: the
    // owning desktop's slice, falling back to the focused output only if the menu
    // was built without one.
    function targetState() {
        return menu.desktop ? menu.desktop.stageState : Services.ShellState.forActive();
    }
    function activeMonitor() {
        const st = menu.targetState();
        return (st && st.modelData) ? st.modelData.name : "";
    }

    // Widgets owns the edit-mode state machine (StageSession); this only enters it.
    function editWidgets() {
        StageCfg.StageSession.enterWidgets(menu.activeMonitor());
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
    function changeWallpaper() {
        Services.ShellState.requestSurfaceActive("wallpaper", null);
        menu.close();
    }
    // Search opens this monitor's app launcher (the Super+Space surface).
    function openSearch() {
        const st = menu.targetState();
        if (st)
            st.launcherOpen = true;
        menu.close();
    }
    // Quick controls opens the left sidebar; Depth settings deep-links Stage.
    function quickControls() {
        Services.ShellState.requestSurfaceActive("sidebar-left", undefined);
        menu.close();
    }
    function depthSettings() {
        Services.ShellState.requestSurfaceActive("sidebar-left#stage", undefined);
        menu.close();
    }
    function openSettings() {
        Spawn.run(["sh", "-c", "ryoku-hub config set section widgets; flock -n -o /tmp/ryoku-hub.lock qs -c hub"]);
        menu.close();
    }
    function refreshShell() {
        Quickshell.execDetached(["ryoku-shell", "reload"]);
        menu.close();
    }

    // iRiS-only conveniences, reachable from the desktop menu on the iris bar
    // style. Studio is the live appearance editor; Edit iRiS arranges the bar in
    // place. Both drive inir's GlobalStates in-process, the way the iris bar
    // scene and the launcher already reach it.
    function openIrisStudio() {
        GlobalStates.irisStudioOpen = true;
        menu.close();
    }
    function editIris() {
        GlobalStates.irisEdit = true;
        menu.close();
    }

    // The Stage depth/parallax effect, named for the Depth row (which opens the
    // full Stage controls rather than juggling two chips inside the menu).
    readonly property string stageEffect: StageCfg.StageBackend.effect
    readonly property bool stageBusy: StageCfg.StageBackend.busy
    readonly property int stagePct: StageCfg.StageBackend.percent
    readonly property string depthLabel: menu.stageBusy ? (menu.stagePct + "%")
        : menu.stageEffect === "parallax" ? I18n.tr("Parallax")
        : menu.stageEffect === "off" ? I18n.tr("Off") : I18n.tr("On")

    // The wallpaper thumbnail for the Wallpaper tile: the still the desktop paints
    // (a poster for a video wall), so the tile shows what is on screen.
    readonly property string wallpaperThumb: menu.desktop ? (menu.desktop.wallpaperPath || "") : ""

    // The iRiS conveniences below show only while its bar style is on screen.
    readonly property bool iris: Services.Config.barStyle === "iris"

    DesktopMenu {
        id: shell
        title: I18n.tr("Desktop")
        gloss: "卓上"

        // ── quick row: the iRiS menu's four tiles, mapped to Ryoku actions ──
        MenuQuick {
            items: [
                { icon: "wallpaper", label: I18n.tr("Wallpaper"), image: menu.wallpaperThumb, action: () => menu.changeWallpaper() },
                { icon: "widgets", label: I18n.tr("Widgets"), action: () => menu.editWidgets() },
                { icon: "graphic_eq", label: I18n.tr("Visualiser"), action: () => menu.customizeVisualizer() },
                { icon: "search", label: I18n.tr("Search"), action: () => menu.openSearch() }
            ]
        }

        MenuSection {}

        // Depth folds the old Depth/Parallax chips and their settings row into
        // one quiet row: it names the current mode and opens the Stage controls.
        MenuRow {
            icon: "landscape"
            label: I18n.tr("Depth")
            value: menu.depthLabel
            on: menu.stageEffect !== "off"
            onTriggered: menu.depthSettings()
        }
        MenuRow { icon: "tune"; label: I18n.tr("Quick controls"); onTriggered: menu.quickControls() }
        // iRiS conveniences: Studio and Edit iRiS on the iris bar style only. On
        // every other style they hide, and the Column skips them with no gap.
        MenuRow { visible: menu.iris; icon: "palette"; label: I18n.tr("Studio"); onTriggered: menu.openIrisStudio() }
        MenuRow { visible: menu.iris; icon: "edit"; label: I18n.tr("Edit Shima"); onTriggered: menu.editIris() }

        MenuSection {}
        MenuRow { icon: "settings"; label: I18n.tr("Settings"); accent: true; closeOnTrigger: false; onTriggered: menu.openSettings() }
        MenuRow { icon: "refresh"; label: I18n.tr("Reload shell"); closeOnTrigger: false; onTriggered: menu.refreshShell() }
    }
}
