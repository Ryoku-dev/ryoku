pragma ComponentBehavior: Bound
import QtQuick
import Quickshell
import Ryoku.Ui.Singletons
import shell.services as Services
import "../stage/Singletons" as StageCfg
import inir
import "Singletons"
import stage.modules.ii.background.shortcuts as StageShortcuts

// The system desktop right-click menu: right-clicking bare wallpaper on any bar
// style opens this. It carries the iRiS desktop menu's structure -- a quick row
// of icon tiles (wallpaper thumbnail, search) over the desktop's own actions,
// led by the one way into the Stage Editor -- in Ryoku's paper-and-ink language, on the shared
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

    property var pendingFolderPoint: null
    property var pendingApplicationPoint: null
    property string page: ""
    function openAt(x, y) {
        menu.pendingFolderPoint = null;
        menu.pendingApplicationPoint = null;
        menu.page = "";
        shell.px = x; shell.py = y; shell.open = true;
    }
    function close() { shell.open = false; }

    // The launcher and the Stage Editor open on the monitor the menu is on: the
    // owning desktop's slice, falling back to the focused output only if the menu
    // was built without one.
    function targetState() {
        return menu.desktop ? menu.desktop.stageState : Services.ShellState.forActive();
    }
    function activeMonitor() {
        const st = menu.targetState();
        return (st && st.modelData) ? st.modelData.name : "";
    }

    // The one way into the Stage Editor: widgets, the visualiser, the depth
    // stage and the wallpaper's framing are all its catalogues now, so the
    // menu enters the mode on this monitor and the toolbar does the rest.
    function editDesktop() {
        StageCfg.StageSession.enterWidgets(menu.activeMonitor());
        menu.close();
    }
    function newFolder() {
        menu.pendingFolderPoint = Qt.point(shell.px, shell.py);
        menu.close();
    }
    function finishNewFolder() {
        const point = menu.pendingFolderPoint;
        menu.pendingFolderPoint = null;
        if (point && menu.desktop)
            menu.desktop.newDesktopFolder(point.x, point.y);
    }
    function newApplication() {
        menu.pendingApplicationPoint = Qt.point(shell.px, shell.py);
        menu.page = "application";
    }
    function leaveApplicationPage() {
        menu.page = "";
        shell.forceActiveFocus();
    }
    function finishNewApplication() {
        menu.pendingApplicationPoint = null;
        menu.leaveApplicationPage();
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
    // Quick controls opens the left sidebar; Settings opens Ryoku Settings on
    // its normal page, while Edit desktop above owns the Stage Editor entry.
    function quickControls() {
        Services.ShellState.requestSurfaceActive("sidebar-left", undefined);
        menu.close();
    }
    function openSettings() {
        Spawn.run(["ryoku-shell", "hub", "open"]);
        menu.close();
    }
    function refreshShell() {
        Quickshell.execDetached(["ryoku-shell", "reload"]);
        menu.close();
    }
    function confirmSessionAction(action) {
        Services.ShellState.askSessionAction(action, menu.activeMonitor());
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

    // The wallpaper thumbnail for the Wallpaper tile: the still the desktop paints
    // (a poster for a video wall), so the tile shows what is on screen.
    readonly property string wallpaperThumb: menu.desktop ? (menu.desktop.wallpaperPath || "") : ""

    // The iRiS conveniences below show only while its bar style is on screen.
    readonly property bool iris: Services.Config.barStyle === "iris"

    DesktopMenu {
        id: shell
        cardWidth: menu.page === "application" ? 400 : powerActions.implicitWidth + Theme.s4 * 2
        onShowingChanged: {
            if (!showing && menu.pendingFolderPoint)
                menu.finishNewFolder();
            if (!showing)
                menu.pendingApplicationPoint = null;
        }
        title: menu.page === "application" ? I18n.tr("New application") : I18n.tr("Desktop")
        gloss: "卓上"
        // ── quick row: the wallpaper picker and the launcher ──
        MenuQuick {
            visible: menu.page === ""
            items: [
                { icon: "wallpaper", label: I18n.tr("Wallpaper"), image: menu.wallpaperThumb, action: () => menu.changeWallpaper() },
                { icon: "search", label: I18n.tr("Search"), action: () => menu.openSearch() }
            ]
        }

        MenuSection { visible: menu.page === "" }

        MenuRow { visible: menu.page === ""; icon: "dashboard_customize"; label: I18n.tr("Edit desktop"); accent: true; onTriggered: menu.editDesktop() }
        MenuRow { visible: menu.page === ""; icon: "create_new_folder"; label: I18n.tr("New folder"); onTriggered: menu.newFolder() }
        MenuRow {
            visible: menu.page === ""
            icon: "add_to_home_screen"
            label: I18n.tr("New application")
            closeOnTrigger: false
            onTriggered: menu.newApplication()
        }
        MenuRow { visible: menu.page === ""; icon: "tune"; label: I18n.tr("Quick controls"); onTriggered: menu.quickControls() }
        // iRiS conveniences: Studio and Edit iRiS on the iris bar style only. On
        // every other style they hide, and the Column skips them with no gap.
        MenuRow { visible: menu.page === "" && menu.iris; icon: "palette"; label: I18n.tr("Shima Studio"); onTriggered: menu.openIrisStudio() }
        MenuRow { visible: menu.page === "" && menu.iris; icon: "edit"; label: I18n.tr("Edit Shima"); onTriggered: menu.editIris() }

        MenuSection { visible: menu.page === "" }
        MenuRow { visible: menu.page === ""; icon: "settings"; label: I18n.tr("General settings"); accent: true; closeOnTrigger: false; onTriggered: menu.openSettings() }
        MenuRow { visible: menu.page === ""; icon: "refresh"; label: I18n.tr("Reload desktop"); closeOnTrigger: false; onTriggered: menu.refreshShell() }

        MenuSection { visible: menu.page === "" }
        MenuQuick {
            visible: menu.page === ""
            id: powerActions
            columns: 3
            items: [
                { icon: "logout", label: I18n.tr("Log out"), action: () => menu.confirmSessionAction("logout") },
                { icon: "restart_alt", label: I18n.tr("Reboot"), action: () => menu.confirmSessionAction("reboot") },
                { icon: "power_settings_new", label: I18n.tr("Power off"), action: () => menu.confirmSessionAction("shutdown") }
            ]
        }

        StageShortcuts.DesktopShortcutAppPicker {
            visible: menu.page === "application"
            width: 368
            screenName: menu.activeMonitor()
            folderId: ""
            targetX: menu.pendingApplicationPoint ? menu.pendingApplicationPoint.x : NaN
            targetY: menu.pendingApplicationPoint ? menu.pendingApplicationPoint.y : NaN
            onApplicationAdded: menu.finishNewApplication()
            onDone: menu.leaveApplicationPage()
        }
    }
}
