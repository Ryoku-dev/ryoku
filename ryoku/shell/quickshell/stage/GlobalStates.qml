pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import stage.services
import stage.modules.common
import stage.modules.common.functions
import Ryoku.Ui.Singletons

// The Stage Editor's shared state: the reference's Edit Mode machine verbatim
// (the switch, the drawer, the per-widget menu, the undo/redo history, the one
// animated shrink scalar, the Escape ladder's owner), with the parts that spoke
// to the reference's own shell routed to Ryoku: the focused monitor comes from
// the Wm seam, the desktop no longer parks on a temp workspace (Ryoku's chrome
// is an Overlay surface over the live desktop), and the bar/dock/lock/tablet
// members the copied chrome still reads are held inert so they compile and do
// nothing. See docs/stage.md.
Singleton {
    id: root

    // ── Edit Mode ────────────────────────────────────────────────────────────
    property bool editMode: false
    property string editModeMonitor: ""

    // The per-widget menu, one at a time shell-wide, drawn by the chrome of the
    // screen it was summoned on. Screen coordinates.
    property bool editWidgetMenuOpen: false
    property string editWidgetMenuInstanceId: ""
    property string editWidgetMenuScreenName: ""
    property real editWidgetMenuX: 0
    property real editWidgetMenuY: 0
    property var editWidgetMenuCanvas: null

    // The drawer (the mode's catalogue) and its own animated scalar.
    property bool editDrawerOpen: false
    property string editDrawerSection: "widgets"
    property string editDrawerPage: ""
    property bool editSearchFocused: false
    signal editSearchFocusRequested()
    signal editSearchReleaseRequested()
    property real editDrawerProgress: root.editMode && root.editDrawerOpen ? 1 : 0
    Behavior on editDrawerProgress {
        enabled: !Appearance.reducedMotion
        animation: Appearance.animation.elementMove.numberAnimation.createObject(root)
    }
    property var editDrawerReveals: ({})
    property string editDrawerDropScreen: ""
    signal editWidgetDroppedOnDrawer(string instanceId)

    // The desktop's right-click menu (in and out of the mode).
    property bool desktopMenuOpen: false
    property bool desktopMenuClosing: false
    property string desktopMenuScreenName: ""
    property real desktopMenuX: 0
    property real desktopMenuY: 0
    property string desktopMenuOrigin: "desktop"
    readonly property bool desktopMenuOnBar: root.desktopMenuOrigin === "bar"

    function openDesktopMenu(screenName, x, y, origin = "desktop") {
        root.closeEditWidgetMenu();
        root.closeEditBarMenu();
        if (origin === true)
            origin = "bar";
        root.desktopMenuOrigin = (origin === "bar" || origin === "dock") ? origin : "desktop";
        root.desktopMenuScreenName = screenName;
        root.desktopMenuX = x;
        root.desktopMenuY = y;
        root.desktopMenuClosing = false;
        root.desktopMenuOpen = true;
    }

    function closeDesktopMenu() {
        if (!root.desktopMenuOpen || root.desktopMenuClosing)
            return;
        root.desktopMenuClosing = true;
    }

    function finishDesktopMenuClose() {
        root.desktopMenuClosing = false;
        root.desktopMenuOpen = false;
    }

    // The bar's in-place edit is not part of the Stage Editor (no bar editing):
    // the members the copied chrome reads are held inert so it compiles.
    property bool editBarDragActive: false
    signal editBarDragCancel()
    property var editBarControllers: ({})
    function registerBarEditController(screenName, controller) {}
    function unregisterBarEditController(controller) {}
    function barEditControllerFor(screenName) { return null; }
    property bool editBarMenuOpen: false
    property string editBarMenuScreenName: ""
    property var editBarMenuController: null
    property int editBarMenuBucket: -1
    property int editBarMenuIndex: -1
    property bool editBarMenuCentered: false
    property real editBarMenuX: 0
    property real editBarMenuY: 0
    property real editBarMenuWindowWidth: 0
    property real editBarMenuWindowHeight: 0

    function openEditBarMenu(screenName, controller, bucket, index, centered, x, y, windowWidth, windowHeight) {
    }

    function closeEditBarMenu() {
        root.editBarMenuOpen = false;
        root.editBarMenuController = null;
    }

    property var editBarHoverSlot: null
    property string editBarHoverName: ""
    property string editBarHoverScreenName: ""
    property real editBarHoverX: 0
    property real editBarHoverY: 0
    property real editBarHoverWindowWidth: 0
    property real editBarHoverWindowHeight: 0
    readonly property bool editBarHoverShown: false

    function showEditBarHover(slot, screenName, name, x, y, windowWidth, windowHeight) {
    }

    function clearEditBarHover(slot) {
    }

    function openEditWidgetMenu(canvas, instanceId, screenName, x, y) {
        if (!root.editMode)
            return;
        root.closeDesktopMenu();
        root.closeEditBarMenu();
        root.editWidgetMenuCanvas = canvas;
        root.editWidgetMenuInstanceId = instanceId;
        root.editWidgetMenuScreenName = screenName;
        root.editWidgetMenuX = x;
        root.editWidgetMenuY = y;
        root.editWidgetMenuOpen = true;
    }

    function closeEditWidgetMenu() {
        root.editWidgetMenuOpen = false;
        root.editWidgetMenuCanvas = null;
        root.editWidgetMenuInstanceId = "";
    }

    // The entry and the exit as ONE animated scalar: the wallpaper surface, the
    // widget canvas and the chrome all derive their geometry from this number.
    property real editProgress: root.editMode ? 1 : 0
    Behavior on editProgress {
        enabled: !Appearance.reducedMotion
        animation: Appearance.animation.elementMove.numberAnimation.createObject(root)
    }

    // Which face the viewport shows: a filter, never a mode of its own. The
    // Stage Editor has no Lockscreen tab, so this only ever holds the desktop.
    property string editTab: EditModeLogic.desktopTab
    readonly property bool editLockPreview: root.editMode && root.editTab === EditModeLogic.lockscreenTab
    property real editTabProgress: root.editLockPreview ? 1 : 0
    Behavior on editTabProgress {
        enabled: !Appearance.reducedMotion
        animation: Appearance.animation.elementMoveFast.numberAnimation.createObject(root)
    }
    readonly property bool lockLookActive: root.screenLocked || root.editLockPreview

    // The edge each dock occupies (screen name -> { side, thickness }), so the
    // viewport clears a dock that reserves nothing. A dock publishes its own
    // resting depth while the Stage Editor frames its screen; with no editor
    // open (or no dock on that edge) the entry is absent and the viewport keeps it.
    property var dockInsets: ({})
    function setDockInset(screenName, side, thickness) {
        if (!screenName)
            return;
        const next = Object.assign({}, root.dockInsets);
        if (!side || !(thickness > 0))
            delete next[screenName];
        else
            next[screenName] = { "side": side, "thickness": thickness };
        root.dockInsets = next;
    }

    // The guided-session flags the chrome reads; Ryoku hosts no tour, so the
    // guide stays off and the toolbar's Done always leaves the mode.
    property bool editGuideActive: false
    property bool welcomeCollapsed: false
    signal editGuideDoneRequested()
    property rect editToolbarRect: Qt.rect(0, 0, 0, 0)

    // ── Undo / redo ──────────────────────────────────────────────────────────
    property var editUndoStack: []
    property var editRedoStack: []
    readonly property bool editCanUndo: editUndoStack.length > 0
    readonly property bool editCanRedo: editRedoStack.length > 0
    property var _editHistoryBatch: null
    property bool _editHistoryReplaying: false
    signal editHistoryWillReplay()

    function editHistoryBeginBatch() {
        if (root._editHistoryBatch === null)
            root._editHistoryBatch = [];
    }

    function editHistoryEndBatch() {
        const entries = root._editHistoryBatch;
        root._editHistoryBatch = null;
        if (entries === null || entries.length === 0)
            return;
        if (entries.length === 1) {
            root._editHistoryCommit(entries[0]);
            return;
        }
        root._editHistoryCommit({
            "undo": () => {
                for (let i = entries.length - 1; i >= 0; i--)
                    entries[i].undo();
            },
            "redo": () => {
                for (let i = 0; i < entries.length; i++)
                    entries[i].redo();
            }
        });
    }

    function editHistoryPush(entry) {
        if (!root.editMode || root._editHistoryReplaying)
            return;
        if (!entry || typeof entry.undo !== "function" || typeof entry.redo !== "function")
            return;
        if (root._editHistoryBatch !== null) {
            root._editHistoryBatch.push(entry);
            return;
        }
        root._editHistoryCommit(entry);
    }

    function _editHistoryCommit(entry) {
        root.editUndoStack = EditModeLogic.undoPush(root.editUndoStack, entry);
        root.editRedoStack = [];
    }

    function editUndo() {
        root.editHistoryWillReplay();
        if (root._editHistoryBatch !== null)
            root.editHistoryEndBatch();
        const popped = EditModeLogic.undoPop(root.editUndoStack);
        root.editUndoStack = popped.stack;
        if (popped.entry === null)
            return;
        root._editHistoryReplay(popped.entry.undo);
        root.editRedoStack = EditModeLogic.undoPush(root.editRedoStack, popped.entry);
    }

    function editRedo() {
        root.editHistoryWillReplay();
        const popped = EditModeLogic.undoPop(root.editRedoStack);
        root.editRedoStack = popped.stack;
        if (popped.entry === null)
            return;
        root._editHistoryReplay(popped.entry.redo);
        root.editUndoStack = EditModeLogic.undoPush(root.editUndoStack, popped.entry);
    }

    function _editHistoryReplay(fn) {
        root._editHistoryReplaying = true;
        try {
            fn();
        } finally {
            root._editHistoryReplaying = false;
        }
    }

    function editHistoryClear() {
        root._editHistoryBatch = null;
        root.editUndoStack = [];
        root.editRedoStack = [];
    }

    // ── Entry and exit ───────────────────────────────────────────────────────
    property string _editRequestedMonitor: ""
    property bool _editKeepWorkspace: false

    function openEditMode(monitor = "", keepWorkspace = false) {
        if (root.editMode)
            return;
        if (root.screenLocked || root.mediaModeActive)
            return;
        root.overviewOpen = false;
        root.sessionOpen = false;
        root._editRequestedMonitor = monitor;
        root.editModeMonitor = monitor !== "" ? monitor
            : (Wm.focusedOutput !== "" ? Wm.focusedOutput : (Quickshell.primaryScreen ? Quickshell.primaryScreen.name : ""));
        root._editKeepWorkspace = keepWorkspace;
        root.editMode = true;
    }

    function openEditCatalogue(section, screenName = "", page = "", keepWorkspace = false) {
        root.editDrawerSection = section;
        root.editDrawerPage = page;
        root.openEditMode(screenName, keepWorkspace);
        if (!root.editMode)
            return;
        root.editDrawerOpen = true;
    }

    function closeEditMode() {
        root.editMode = false;
    }

    function openSettingsFromEditMode(pageId, subPageId, sectionId) {
        if (root.editMode)
            root.closeEditMode();
        root.openSettingsPage(pageId, subPageId, sectionId);
    }

    function openWallpaperSelectorFromEditMode(target = "desktop") {
        root.wallpaperSelectorTarget = target;
        root.wallpaperSelectorOpen = true;
    }

    function toggleEditMode() {
        if (root.editMode)
            root.closeEditMode();
        else
            root.openEditMode();
    }

    property string _editReopenMonitor: ""
    property string _editReopenPage: ""
    property bool _editReopenWallpaper: false
    function switchEditMonitor(monitorName) {
        if (!root.editMode || !monitorName || monitorName === root.editModeMonitor)
            return;
        root._editReopenMonitor = monitorName;
        root._editReopenWallpaper = root.editDrawerOpen && root.editDrawerSection === "wallpaper"
            && !root.editLockPreview;
        root._editReopenPage = root._editReopenWallpaper ? root.editDrawerPage : "";
        root.closeEditMode();
        editReopenTimer.restart();
    }
    Timer {
        id: editReopenTimer
        interval: Appearance.reducedMotion ? 50 : Appearance.animation.elementMove.duration + 80
        repeat: false
        onTriggered: {
            const monitor = root._editReopenMonitor;
            const wallpaper = root._editReopenWallpaper;
            const page = root._editReopenPage;
            root._editReopenMonitor = "";
            root._editReopenWallpaper = false;
            root._editReopenPage = "";
            if (monitor === "")
                return;
            if (wallpaper)
                root.openEditCatalogue("wallpaper", monitor, page);
            else
                root.openEditMode(monitor);
        }
    }

    function _enterEditMode() {
        root.editTab = EditModeLogic.desktopTab;
        root.editModeMonitor = root._editRequestedMonitor !== "" ? root._editRequestedMonitor
            : (Wm.focusedOutput !== "" ? Wm.focusedOutput : (Quickshell.primaryScreen ? Quickshell.primaryScreen.name : ""));
        root._editRequestedMonitor = "";
        root.dashboardPanelOpen = false;
        root.mediaControlsOpen = false;
    }

    function _leaveEditMode() {
        root._editKeepWorkspace = false;
        root.closeEditWidgetMenu();
        root.closeEditBarMenu();
        root.closeDesktopMenu();
        root.clearEditBarHover(null);
        root.editBarDragActive = false;
        root.editDrawerOpen = false;
        root.editDrawerSection = "widgets";
        root.editDrawerPage = "";
        root.editDrawerDropScreen = "";
    }

    onEditModeChanged: {
        if (root.editMode) {
            root._enterEditMode();
            return;
        }
        root._leaveEditMode();
        root.editHistoryClear();
    }

    // The bar runs on every screen under Ryoku.
    function isScreenAllowedForBar(screen) {
        return screen !== null && screen !== undefined;
    }

    // ── Members the copied chrome reads but the Stage Editor does not drive ───
    // The bar/dock/lock/tablet surfaces keep their own Ryoku editors; these hold
    // the shared flags at rest so the copied Edit Mode code compiles unchanged.
    property bool screenLocked: false
    property bool sessionOpen: false
    property bool overviewOpen: false
    property bool mediaModeActive: false
    property bool mediaModeActivatedKeepAwake: false
    property bool connectModeActive: false
    property bool barOpen: true
    property bool settingsOpen: false
    property bool policiesPanelOpen: false
    property bool dashboardPanelOpen: false
    property alias sidebarLeftOpen: root.policiesPanelOpen
    property alias sidebarRightOpen: root.dashboardPanelOpen
    property string activeLeftSidebarMonitor: ""
    property string activeRightSidebarMonitor: ""
    property bool mediaControlsOpen: false
    property bool lockScreenCentered: false
    property bool lockAnimationActive: false
    property bool workspaceRestoreInProgress: false
    property bool wallpaperSelectorOpen: false
    property string wallpaperSelectorTarget: "desktop"
    property bool usageOpen: false
    property bool cheatsheetOpen: false
    property bool clockAppOpen: false
    property bool notesAppOpen: false
    property bool appDrawerOpen: false
    property bool recentsOpen: false
    property bool shellSwitcherOpen: false
    property bool hubModePreview: false
    property bool islandDashboardOpen: false
    property bool oledSaverOpen: false
    property bool screenTranslatorOpen: false
    property bool osdVolumeOpen: false
    property bool oskOpen: false
    property bool overlayOpen: false
    property bool modesOpen: false
    property bool modeFlashActive: false
    property var modeFlashPayload: null
    property bool phoneCameraRunning: false
    property bool phoneMicRunning: false
    property bool presetBarHidden: false
    property bool presetHoldMotion: false
    property bool presetRecoloring: false
    property bool presetWorkDeferred: false
    property int homeScreenAppsRevision: 0
    property var liveDrawHandler: null
    property var navigateBackHandler: null
    property var navigateHomeHandler: null
    property var addAppToHomeScreenHandler: null
    property var removeAppFromHomeScreenHandler: null
    property var isAppOnHomeScreenHandler: null
    property var addAppPairToHomeScreenHandler: null
    property var addFolderToHomeScreenHandler: null
    property var clearHomeScreenAppsHandler: null
    signal osdNoticeRequested(string icon, string caption, string label)

    function openLeftSidebar(monitorName) {}
    function openRightSidebar(monitorName) {}
    function openModesApp(tab = "") {}
    function openSearchPanel(panelId, monitorName, initialQuery) {}
    function toggleAppDrawer(monitorName) {}
    function toggleCheatsheet() {}
    function toggleHubModePreview() {}
    function toggleOverview(monitorName) {}
    function toggleRecents(monitorName) {}
    function toggleSettings() {}
    function toggleTabletApp(appId) {}
    function toggleWelcome() {}
    function requestBarPlacement(bottom, vertical) {}
    function launchColorPicker() {}
    function openSettingsPage(pageId, subPageId, sectionId) {}
}
