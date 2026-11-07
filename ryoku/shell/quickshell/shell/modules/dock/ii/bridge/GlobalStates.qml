pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import stage as Stage
import shell.services
import shell.modules.phone
import "../../../stage/Singletons" as StageCfg
import Ryoku.Ui.Singletons
import ".."

/**
 * The ii dock's window to the shell's global state.
 *
 * The port reads one flat `GlobalStates` object; Ryoku splits it between the
 * Stage Editor's states (edit mode, sidebars, overview, the clock app's open
 * flag) and the dock's own session facts (what is dragging, ringing, or
 * mirrored right now). This singleton forwards everything the stage owns and
 * keeps the rest, so the dock never learns that the split exists.
 */
Singleton {
    id: root
    readonly property var clockAppHost: ClockApp
    readonly property ReminderAlertPopup reminderAlertHost: ReminderAlertPopup {}
    readonly property LocalSendPopup localSendPopupHost: LocalSendPopup {}


    // ── forwarded from the Stage Editor ──────────────────────────────────────
    readonly property bool editMode: Stage.GlobalStates.editMode
    readonly property real editProgress: Stage.GlobalStates.editProgress
    readonly property bool screenLocked: Stage.GlobalStates.screenLocked
    readonly property bool lockLookActive: Stage.GlobalStates.lockLookActive
    readonly property bool presetBarHidden: Stage.GlobalStates.presetBarHidden
    readonly property bool barOpen: Stage.GlobalStates.barOpen
    readonly property bool overviewOpen: Stage.GlobalStates.overviewOpen
    property bool mediaControlsOpen: Stage.GlobalStates.mediaControlsOpen
    onMediaControlsOpenChanged: {
        if (Stage.GlobalStates.mediaControlsOpen !== root.mediaControlsOpen)
            Stage.GlobalStates.mediaControlsOpen = root.mediaControlsOpen;
    }
    Connections {
        target: Stage.GlobalStates
        function onMediaControlsOpenChanged() {
            if (root.mediaControlsOpen !== Stage.GlobalStates.mediaControlsOpen)
                root.mediaControlsOpen = Stage.GlobalStates.mediaControlsOpen;
        }
    }
    function openMediaControlsFromDock(anchorRect, dockSide, screenName) {
        Stage.GlobalStates.mediaControlsDockAnchor = {
            "x": Number(anchorRect?.x ?? 0),
            "y": Number(anchorRect?.y ?? 0),
            "width": Number(anchorRect?.width ?? 0),
            "height": Number(anchorRect?.height ?? 0),
            "screen": String(screenName ?? ""),
            "side": String(dockSide ?? "bottom"),
            "requestedAt": Date.now()
        };
        root.mediaControlsOpen = true;
    }
    property bool localSendPopupOpen: Stage.GlobalStates.localSendPopupOpen
    onLocalSendPopupOpenChanged: if (Stage.GlobalStates.localSendPopupOpen !== root.localSendPopupOpen)
        Stage.GlobalStates.localSendPopupOpen = root.localSendPopupOpen
    property var localSendPopupTransfer: Stage.GlobalStates.localSendPopupTransfer
    onLocalSendPopupTransferChanged: if (Stage.GlobalStates.localSendPopupTransfer !== root.localSendPopupTransfer)
        Stage.GlobalStates.localSendPopupTransfer = root.localSendPopupTransfer
    Connections {
        target: Stage.GlobalStates
        function onLocalSendPopupOpenChanged() {
            if (root.localSendPopupOpen !== Stage.GlobalStates.localSendPopupOpen)
                root.localSendPopupOpen = Stage.GlobalStates.localSendPopupOpen;
        }
        function onLocalSendPopupTransferChanged() {
            if (root.localSendPopupTransfer !== Stage.GlobalStates.localSendPopupTransfer)
                root.localSendPopupTransfer = Stage.GlobalStates.localSendPopupTransfer;
        }
    }
    // The wallpaper tile flips this flag to ask for Ryoku's picker
    // (ryogami's full-screen browser); rising launches it.
    property bool wallpaperSelectorOpen: false
    onWallpaperSelectorOpenChanged: if (root.wallpaperSelectorOpen)
        Spawn.run(["ryogami", "wallpaper", "ui"]);
    readonly property bool editBarMenuOpen: Stage.GlobalStates.editBarMenuOpen
    readonly property bool editWidgetMenuOpen: Stage.GlobalStates.editWidgetMenuOpen
    property bool modesOpen: Stage.GlobalStates.modesOpen
    onModesOpenChanged: if (Stage.GlobalStates.modesOpen !== root.modesOpen)
        Stage.GlobalStates.modesOpen = root.modesOpen
    readonly property bool phoneCameraRunning: Stage.GlobalStates.phoneCameraRunning
    readonly property bool phoneMicRunning: Stage.GlobalStates.phoneMicRunning
    readonly property bool phoneMirrorRunning: PhoneScrcpyService.mirrorRunning || KdeConnectService.scrcpyRunning
    property bool clockAppOpen: Stage.GlobalStates.clockAppOpen
    onClockAppOpenChanged: if (Stage.GlobalStates.clockAppOpen !== root.clockAppOpen)
        Stage.GlobalStates.clockAppOpen = root.clockAppOpen
    property bool oskOpen: Stage.GlobalStates.oskOpen
    onOskOpenChanged: if (Stage.GlobalStates.oskOpen !== root.oskOpen)
        Stage.GlobalStates.oskOpen = root.oskOpen
    // Read-only overlay state the dock's tooltip gate and badges watch: the
    // stage owns every surface behind these, the dock never raises them.
    readonly property bool sidebarLeftOpen: Stage.GlobalStates.sidebarLeftOpen
    readonly property bool sidebarRightOpen: Stage.GlobalStates.sidebarRightOpen
    readonly property bool osdVolumeOpen: Stage.GlobalStates.osdVolumeOpen
    readonly property bool cheatsheetOpen: Stage.GlobalStates.cheatsheetOpen
    readonly property bool notesAppOpen: Stage.GlobalStates.notesAppOpen
    readonly property bool sessionOpen: Stage.GlobalStates.sessionOpen
    readonly property bool usageOpen: Stage.GlobalStates.usageOpen
    readonly property bool islandDashboardOpen: Stage.GlobalStates.islandDashboardOpen

    readonly property bool effectiveLeftOpen: Stage.GlobalStates.sidebarLeftOpen
    readonly property bool effectiveRightOpen: Stage.GlobalStates.sidebarRightOpen
    readonly property bool overviewSurfaceOpen: Stage.GlobalStates.overviewOpen

    function closeEditBarMenu() { Stage.GlobalStates.closeEditBarMenu(); }
    function closeEditWidgetMenu() { Stage.GlobalStates.closeEditWidgetMenu(); }
    function closeDesktopMenu() { Stage.GlobalStates.closeDesktopMenu(); }
    function openDesktopMenu(screenName, x, y, origin) { Stage.GlobalStates.openDesktopMenu(screenName, x, y, origin ?? "desktop"); }
    function openEditCatalogue(section, screenName, page) { Stage.GlobalStates.openEditCatalogue(section, screenName ?? "", page ?? ""); }
    function editHistoryPush(entry) { Stage.GlobalStates.editHistoryPush(entry); }
    function toggleOverview(monitorName) { Stage.GlobalStates.toggleOverview(monitorName); }
    function launchColorPicker() { Spawn.run(["ryoku-cmd-color-picker"]); }
    // The dock's settings live in the Stage Editor's dock pages: the design
    // page for the dock as a whole, its widgets page for any one item on it.
    // Opened through the session, exactly as the editor's IPC opens a
    // section, so the desktop's edit frames come up with it.
    readonly property bool settingsOpen: Stage.GlobalStates.settingsOpen
    function openSettingsPage(pageId, subPageId, sectionId) {
        const section = "dock/" + (String(subPageId ?? "") !== "" ? "iiWidgets" : "appearance");
        if (section === "dock/iiWidgets") {
            Stage.GlobalStates.iiDockWidgetSettingsTarget = "";
            Stage.GlobalStates.iiDockWidgetSettingsTarget = String(sectionId ?? "");
        }
        const active = ShellState.forActive();
        let monitor = active?.modelData?.name ?? "";
        if (monitor === "")
            monitor = ShellState.screens.length > 0 ? ShellState.screens[0].name : "";
        if (StageCfg.StageSession.onMonitor(monitor))
            StageCfg.StageSession.openSection(section);
        else
            StageCfg.StageSession.enterWidgets(monitor, section);
    }
    function setDockInset(screenName, side, thickness, reserved) {
        Stage.GlobalStates.setDockInset(screenName, side, thickness);
        const next = Object.assign({}, root.dockInsets);
        next[screenName] = { side: side, thickness: thickness, reserved: reserved ?? false };
        root.dockInsets = next;
    }


    // Stage owns this pointer so relative and module imports share one popup.
    readonly property var dockPopup: Stage.GlobalStates.iiDockPopup
    function claimDockPopup(popup) {
        Stage.GlobalStates.claimIiDockPopup(popup);
    }
    function releaseDockPopup(popup) {
        Stage.GlobalStates.releaseIiDockPopup(popup);
    }
    function dismissDockPopup() {
        Stage.GlobalStates.dismissIiDockPopup();
    }
    // ── the dock's own session state ─────────────────────────────────────────
    property var dockInsets: ({})
    property var barIslands: ({})
    property bool fileDragActive: false
    property bool alarmRinging: false
    property bool islandOwnsAlarm: false
    property bool islandOwnsReminder: false
    property var oledSaverMonitors: []
    property string clockAppPendingTab: ""
    property string reminderToOpen: ""
    property color colorPickerPopupColor: "#00000000"
    property var dockContents: []

    function registerDockContent(content) {
        const list = root.dockContents.slice();
        if (list.indexOf(content) < 0)
            list.push(content);
        root.dockContents = list;
    }

    function unregisterDockContent(content) {
        root.dockContents = root.dockContents.filter(c => c !== content);
    }

    // The dock's clock tile: open the clock app, optionally landing on a tab.
    function openClockApp(tab) {
        root.clockAppHost.requestOpen(String(tab ?? ""));
    }

    // The search tile opens Ryoku's launcher.
    function toggleSearchOnly(screenName) {
        ShellState.requestSurfaceActive("launcher", screenName ?? "");
    }

    // The dock has no media-mode screen concept; media is a widget, not a mode.
    function isMediaModeActiveForScreen(screenName): bool {
        return false;
    }

    // The overview backdrop controller belongs to the stage's overview; the
    // dock only asks for it to keep its own reveal quiet while it is open.
    function overviewBackgroundControllerFor(screenName) {
        return null;
    }

    // The clock app's timetable deep link: open the shell's left sidebar.
    function openTimetableAt(day) {
        ShellState.requestSurfaceActive("sidebar-left", "");
    }
}
