import QtQuick
import Quickshell
import Quickshell.Io
import Caelestia
import Caelestia.Config
import Caelestia.Services
import caelestia.components.misc
import caelestia.services
import caelestia.utils
import caelestia.modules.launcher.services

Scope {
    id: root

    property bool launcherInterrupted
    property string lastAction: ""
    readonly property bool hasFullscreen: Kwin.hasFullscreen()

    // `action` ids are the krohnkite
    // kwinscript's own registrations (verified against its contents/ui/shortcuts.qml):
    // keep their exact casing (e.g. "KrohnkitegrowWidth") or KWin will invoke actions
    // it never registered. `key` is the default binding ("" = unbound), overridden by
    // the user's keybinds.json.
    readonly property var krohnkiteShortcuts: [
        { name: "krohnkiteFocusUp", description: qsTr("Focus the window above"), action: "KrohnkiteFocusUp", key: "Meta+Up" },
        { name: "krohnkiteFocusDown", description: qsTr("Focus the window below"), action: "KrohnkiteFocusDown", key: "Meta+Down" },
        { name: "krohnkiteFocusLeft", description: qsTr("Focus the window to the left"), action: "KrohnkiteFocusLeft", key: "Meta+Left" },
        { name: "krohnkiteFocusRight", description: qsTr("Focus the window to the right"), action: "KrohnkiteFocusRight", key: "Meta+Right" },
        { name: "krohnkiteShiftUp", description: qsTr("Move window up"), action: "KrohnkiteShiftUp", key: "Meta+Shift+Up" },
        { name: "krohnkiteShiftDown", description: qsTr("Move window down"), action: "KrohnkiteShiftDown", key: "Meta+Shift+Down" },
        { name: "krohnkiteShiftLeft", description: qsTr("Move window left"), action: "KrohnkiteShiftLeft", key: "Meta+Shift+Left" },
        { name: "krohnkiteShiftRight", description: qsTr("Move window right"), action: "KrohnkiteShiftRight", key: "Meta+Shift+Right" },
        { name: "krohnkiteCloseWindow", description: qsTr("Close current window"), action: "Window Close", key: "Meta+Q" },
        { name: "krohnkiteFocusNext", description: qsTr("Focus next window"), action: "KrohnkiteFocusNext", key: "" },
        { name: "krohnkiteFocusPrev", description: qsTr("Focus previous window"), action: "KrohnkiteFocusPrev", key: "" },
        { name: "krohnkiteSetMaster", description: qsTr("Set active window as Master"), action: "KrohnkiteSetMaster", key: "" },
        { name: "krohnkiteNextLayout", description: qsTr("Switch to next layout"), action: "KrohnkiteNextLayout", key: "" },
        { name: "krohnkitePreviousLayout", description: qsTr("Switch to previous layout"), action: "KrohnkitePreviousLayout", key: "" },
        { name: "krohnkiteBTreeLayout", description: qsTr("Switch to BTree layout"), action: "KrohnkiteBTreeLayout", key: "" },
        { name: "krohnkiteMonocleLayout", description: qsTr("Switch to Monocle layout"), action: "KrohnkiteMonocleLayout", key: "" },
        { name: "krohnkiteFloatingLayout", description: qsTr("Switch to Floating layout"), action: "KrohnkiteFloatingLayout", key: "" },
        { name: "krohnkiteQuarterLayout", description: qsTr("Switch to Quarter layout"), action: "KrohnkiteQuarterLayout", key: "" },
        { name: "krohnkiteSpreadLayout", description: qsTr("Switch to Spread layout"), action: "KrohnkiteSpreadLayout", key: "" },
        { name: "krohnkiteStackedLayout", description: qsTr("Switch to Stacked layout"), action: "KrohnkiteStackedLayout", key: "" },
        { name: "krohnkiteStairLayout", description: qsTr("Switch to Stair layout"), action: "KrohnkiteStairLayout", key: "" },
        { name: "krohnkiteColumnsLayout", description: qsTr("Switch to Columns layout"), action: "KrohnkiteColumnsLayout", key: "" },
        { name: "krohnkiteTreeColumnLayout", description: qsTr("Switch to Three Column layout"), action: "KrohnkiteThreeColumnLayout", key: "" },
        { name: "krohnkiteSpiralLayout", description: qsTr("Switch to Spiral layout"), action: "KrohnkiteSpiralLayout", key: "" },
        { name: "krohnkiteTileLayout", description: qsTr("Switch to Tile layout"), action: "KrohnkiteTileLayout", key: "" },
        { name: "krohnkiteGrowHeight", description: qsTr("Increase window height"), action: "KrohnkiteGrowHeight", key: "" },
        { name: "krohnkiteShrinkHeight", description: qsTr("Decrease window height"), action: "KrohnkiteShrinkHeight", key: "" },
        { name: "krohnkiteGrowWidth", description: qsTr("Increase window width"), action: "KrohnkitegrowWidth", key: "" },
        { name: "krohnkiteShrinkWidth", description: qsTr("Decrease window width"), action: "KrohnkiteShrinkWidth", key: "" },
        { name: "krohnkiteIncreaseMaster", description: qsTr("Increase master area size"), action: "KrohnkiteIncrease", key: "" },
        { name: "krohnkiteDecreaseMaster", description: qsTr("Decrease master area size"), action: "KrohnkiteDecrease", key: "" },
        { name: "krohnkiteToggleFloat", description: qsTr("Toggle floating state"), action: "KrohnkiteToggleFloat", key: "" },
        { name: "krohnkiteFloatAll", description: qsTr("Toggle floating state for all"), action: "KrohnkiteFloatAll", key: "" },
        { name: "krohnkiteRotate", description: qsTr("Rotate the window layout"), action: "KrohnkiteRotate", key: "" },
        { name: "krohnkiteRotatePart", description: qsTr("Rotate windows within a part"), action: "KrohnkiteRotatePart", key: "" },
        { name: "krohnkiteToggleDock", description: qsTr("Toggle dock support"), action: "KrohnkitetoggleDock", key: "" },
    ]

    function activateDockEntry(idx: int): void {
        let output = "";
        if (Kwin.cursorOutputName)
            output = Kwin.cursorOutputName();
        let screenName = "";
        const screens = [...Quickshell.screens];
        for (const s of screens) {
            if (s.name === output) {
                screenName = s.name;
                break;
            }
        }
        if (!screenName && screens.length > 0)
            screenName = screens[0].name;
        const dock = Visibilities.docks.get(screenName);
        if (dock)
            dock.activateIndex(idx);
    }

    function activateDockEntryNew(idx: int): void {
        let output = "";
        if (Kwin.cursorOutputName)
            output = Kwin.cursorOutputName();
        let screenName = "";
        const screens = [...Quickshell.screens];
        for (const s of screens) {
            if (s.name === output) {
                screenName = s.name;
                break;
            }
        }
        if (!screenName && screens.length > 0)
            screenName = screens[0].name;
        const dock = Visibilities.docks.get(screenName);
        if (dock)
            dock.activateNewIndex(idx);
    }

    Component.onCompleted: {
        Logger.mark("shortcuts-ready");
    }

    // qmllint disable unresolved-type

    CustomShortcut {
        // qmllint enable unresolved-type
        name: "settings"
        description: qsTr("Open settings")
        onPressed: Quickshell.execDetached(["ryoku-shell", "hub", "open"])
    }

    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "showall"
        description: qsTr("Toggle launcher, dashboard and osd")
        onPressed: {
            const v = Visibilities.getForActive();
            v.launcher = v.dashboard = v.osd = v.utilities = !(v.launcher || v.dashboard || v.osd || v.utilities);
        }
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "dashboard"
        description: qsTr("Toggle dashboard")
        onPressed: {
            const visibilities = Visibilities.getForActive();
            visibilities.dashboard = !visibilities.dashboard;
        }
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "overview"
        description: qsTr("Toggle overview")
        onPressed: {
            const visibilities = Visibilities.getForActive();
            if (visibilities.overview) {
                Visibilities.setOverview(false);
            } else {
                if (Kwin.activeWindow && Kwin.activeWindow.address) {
                    Visibilities.preOverviewActiveWindowAddress = Kwin.activeWindow.address;
                } else {
                    Visibilities.preOverviewActiveWindowAddress = "";
                }
                Visibilities.setOverview(true);
            }
        }
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "screenRecording"
        description: qsTr("Toggle screen recording")
        onPressed: {
            if (Recorder.running) {
                if (Recorder.paused) {
                    Recorder.togglePause();
                } else {
                    Recorder.stop();
                }
            } else {
                Recorder.start();
            }
        }
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "session"

        description: qsTr("Toggle session menu")
        onPressed: {
            const visibilities = Visibilities.getForActive();
            visibilities.session = !visibilities.session;
        }
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "launcher"
        description: qsTr("Toggle launcher")
        onPressed: root.launcherInterrupted = false
        onReleased: {
            if (!root.launcherInterrupted) {
                root.lastAction = "launcher";
                const visibilities = Visibilities.getForActive();
                visibilities.launcher = !visibilities.launcher;
            }
            root.launcherInterrupted = false;
        }
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "launcherInterrupt"
        description: qsTr("Interrupt launcher keybind")
        onPressed: root.launcherInterrupted = true
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "sidebar"
        description: qsTr("Toggle sidebar")
        onPressed: {
            const visibilities = Visibilities.getForActive();
            Visibilities.initialSidebarTab = "";
            visibilities.sidebar = !visibilities.sidebar;
        }
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "aiAssistant"
        description: qsTr("Toggle AI Assistant")
        onPressed: {
            const visibilities = Visibilities.getForActive();
            Visibilities.initialSidebarTab = "ai";
            visibilities.sidebar = true;
        }
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "utilities"
        description: qsTr("Toggle utilities")
        onPressed: {
            const visibilities = Visibilities.getForActive();
            visibilities.utilities = !visibilities.utilities;
        }
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "emoji"
        description: qsTr("Open emoji picker")
        onPressed: {
            Visibilities.launcherInitialSearch = `${GlobalConfig.launcher.actionPrefix}emoji `;
            const visibilities = Visibilities.getForActive();
            visibilities.launcher = true;
        }
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "clipboard"
        description: qsTr("Open clipboard history")
        onPressed: {
            Visibilities.launcherInitialSearch = `${GlobalConfig.launcher.actionPrefix}clipboard `;
            const visibilities = Visibilities.getForActive();
            visibilities.launcher = true;
        }
    }

    Connections {
        function onModifierReleased(): void {
            const visibilities = Visibilities.getForActive();
            if (visibilities.launcher && root.lastAction === "windows") {
                const switcherKey = "Alt+Tab";
                if (!CUtils.isShortcutModifierPressed(switcherKey)) {
                    Windows.focusSelectedWindow();
                    visibilities.launcher = false;
                    root.lastAction = "";
                }
            }
        }

        target: CUtils
    }


    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "windowSwitcher"
        description: qsTr("Open window switcher")
        enabled: Config.tabSwitch.enabled
        onPressed: {
            const visibilities = Visibilities.getForActive();
            if (visibilities.launcher && root.lastAction === "windows") {
                Windows.triggerCycleNext();
            } else {
                root.lastAction = "windows";
                Windows.isSwitching = true;
                Windows.updateItems();
                Windows.selectedIndex = (Windows.items.length > 1) ? 1 : 0;
                Windows.refreshHighlight();
                Visibilities.launcherInitialSearch = `${GlobalConfig.launcher.actionPrefix}windows `;
                visibilities.launcher = true;
            }
        }
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "windowSwitcherReverse"
        description: qsTr("Open window switcher (reverse)")
        enabled: Config.tabSwitch.enabled
        onPressed: {
            const visibilities = Visibilities.getForActive();
            if (visibilities.launcher && root.lastAction === "windows") {
                Windows.triggerCyclePrev();
            } else {
                root.lastAction = "windows";
                Windows.isSwitching = true;
                Windows.updateItems();
                Windows.selectedIndex = (Windows.items.length > 1) ? Windows.items.length - 1 : 0;
                Windows.refreshHighlight();
                Visibilities.launcherInitialSearch = `${GlobalConfig.launcher.actionPrefix}windows `;
                visibilities.launcher = true;
            }
        }
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "wallpaper"
        description: qsTr("Open wallpaper picker")
        onPressed: {
            Visibilities.launcherInitialSearch = `${GlobalConfig.launcher.actionPrefix}wallpaper `;
            const visibilities = Visibilities.getForActive();
            visibilities.launcher = true;
        }
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "keybinds"
        description: qsTr("Open keybinds list")
        onPressed: {
            Visibilities.launcherInitialSearch = `${GlobalConfig.launcher.actionPrefix}keybinds `;
            const visibilities = Visibilities.getForActive();
            visibilities.launcher = true;
        }
    }
    CustomShortcut {
        name: "foot"
        description: qsTr("Launch Terminal")
        onPressed: Launch.exec([...GlobalConfig.general.apps.terminal])
    }
    CustomShortcut {
        name: "firefox"
        description: qsTr("Launch Browser")
        onPressed: Launch.exec(["firefox"])
    }
    CustomShortcut {
        name: "code"
        description: qsTr("Launch Editor")
        onPressed: Launch.exec(["code"])
    }
    CustomShortcut {
        name: "github-desktop"
        description: qsTr("Launch GitHub Desktop")
        onPressed: Launch.exec(["github-desktop"])
    }
    CustomShortcut {
        name: "nemo"
        description: qsTr("Launch File Manager")
        onPressed: Launch.exec(["nemo"])
    }
    CustomShortcut {
        name: "kcolorpicker"
        description: qsTr("Color Picker")
        onPressed: ColorPicker.pickColor()
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "workspace1"
        description: qsTr("Switch to workspace 1")
        onPressed: Kwin.setDesktop(1)
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "workspace2"
        description: qsTr("Switch to workspace 2")
        onPressed: Kwin.setDesktop(2)
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "workspace3"
        description: qsTr("Switch to workspace 3")
        onPressed: Kwin.setDesktop(3)
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "workspace4"
        description: qsTr("Switch to workspace 4")
        onPressed: Kwin.setDesktop(4)
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "workspace5"
        description: qsTr("Switch to workspace 5")
        onPressed: Kwin.setDesktop(5)
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "workspace6"
        description: qsTr("Switch to workspace 6")
        onPressed: Kwin.setDesktop(6)
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "workspace7"
        description: qsTr("Switch to workspace 7")
        onPressed: Kwin.setDesktop(7)
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "workspace8"
        description: qsTr("Switch to workspace 8")
        onPressed: Kwin.setDesktop(8)
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "workspace9"
        description: qsTr("Switch to workspace 9")
        onPressed: Kwin.setDesktop(9)
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "workspace10"
        description: qsTr("Switch to workspace 10")
        onPressed: Kwin.setDesktop(10)
    }
    IpcHandler {
        function toggle(drawer: string): void {
            if (list().split("\n").includes(drawer)) {
                if (root.hasFullscreen && ["launcher", "session", "dashboard"].includes(drawer))
                    return;
                const visibilities = Visibilities.getForActive();
                if (drawer === "overview")
                    Visibilities.setOverview(!visibilities.overview);
                else
                    visibilities[drawer] = !visibilities[drawer];
                Logger.mark(`drawer=${drawer} toggled`);
            } else {
                console.warn(lc, `Drawer "${drawer}" does not exist`);
            }
        }
        function toggleTab(drawer: string, tab: string): void {
            if (list().split("\n").includes(drawer)) {
                if (root.hasFullscreen && ["launcher", "session", "dashboard"].includes(drawer))
                    return;
                if (drawer === "sidebar" && tab !== "") {
                    Visibilities.initialSidebarTab = tab;
                    const visibilities = Visibilities.getForActive();
                    visibilities.sidebar = true;
                    return;
                }
                const visibilities = Visibilities.getForActive();
                if (drawer === "overview")
                    Visibilities.setOverview(!visibilities.overview);
                else
                    visibilities[drawer] = !visibilities[drawer];
            } else {
                console.warn(lc, `Drawer "${drawer}" does not exist`);
            }
        }
        function list(): string {
            const visibilities = Visibilities.getForActive();
            return Object.keys(visibilities).filter(k => typeof visibilities[k] === "boolean").join("\n");
        }

        target: "drawers"
    }
    IpcHandler {
        function open(): void {
            Quickshell.execDetached(["ryoku-shell", "hub", "open"]);
        }

        target: "settings"
    }
    IpcHandler {
        function info(title: string, message: string, icon: string): void {
            Toaster.toast(title, message, icon, Toast.Info);
        }
        function success(title: string, message: string, icon: string): void {
            Toaster.toast(title, message, icon, Toast.Success);
        }
        function warn(title: string, message: string, icon: string): void {
            Toaster.toast(title, message, icon, Toast.Warning);
        }
        function error(title: string, message: string, icon: string): void {
            Toaster.toast(title, message, icon, Toast.Error);
        }

        target: "toaster"
    }
    IpcHandler {
        function action(name: string): void {
            root.lastAction = name;
            Visibilities.launcherInitialSearch =
            `${GlobalConfig.launcher.actionPrefix}${name} `;

            const visibilities = Visibilities.getForActive();
            visibilities.launcher = true;
        }
        target: "launcher"
    }
    Instantiator {
        model: root.krohnkiteShortcuts

        // qmllint disable unresolved-type
        delegate: CustomShortcut {
            // qmllint enable unresolved-type
            required property var modelData

            name: modelData.name
            description: modelData.description
            key: Config.general.krohnkiteEnabled ? modelData.key : ""
            onPressed: {
                if (Config.general.krohnkiteEnabled)
                    Quickshell.execDetached(["qdbus6", "org.kde.kglobalaccel", "/component/kwin", "org.kde.kglobalaccel.Component.invokeShortcut", modelData.action]);
            }
        }
    }
    LoggingCategory {
        id: lc

        name: "caelestia.qml.shortcuts"
        defaultLogLevel: LoggingCategory.Info
    }

    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "dockActivate1"
        description: qsTr("Open dock entry 1")
        key: ""
        onPressed: root.activateDockEntry(0)
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "dockActivate2"
        description: qsTr("Open dock entry 2")
        key: ""
        onPressed: root.activateDockEntry(1)
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "dockActivate3"
        description: qsTr("Open dock entry 3")
        key: ""
        onPressed: root.activateDockEntry(2)
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "dockActivate4"
        description: qsTr("Open dock entry 4")
        key: ""
        onPressed: root.activateDockEntry(3)
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "dockActivate5"
        description: qsTr("Open dock entry 5")
        key: ""
        onPressed: root.activateDockEntry(4)
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "dockActivate6"
        description: qsTr("Open dock entry 6")
        key: ""
        onPressed: root.activateDockEntry(5)
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "dockActivate7"
        description: qsTr("Open dock entry 7")
        key: ""
        onPressed: root.activateDockEntry(6)
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "dockActivate8"
        description: qsTr("Open dock entry 8")
        key: ""
        onPressed: root.activateDockEntry(7)
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "dockActivate9"
        description: qsTr("Open dock entry 9")
        key: ""
        onPressed: root.activateDockEntry(8)
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "dockNewWindow1"
        description: qsTr("Open new window of dock entry 1")
        key: "Meta+Ctrl+1"
        onPressed: root.activateDockEntryNew(0)
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "dockNewWindow2"
        description: qsTr("Open new window of dock entry 2")
        key: "Meta+Ctrl+2"
        onPressed: root.activateDockEntryNew(1)
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "dockNewWindow3"
        description: qsTr("Open new window of dock entry 3")
        key: "Meta+Ctrl+3"
        onPressed: root.activateDockEntryNew(2)
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "dockNewWindow4"
        description: qsTr("Open new window of dock entry 4")
        key: "Meta+Ctrl+4"
        onPressed: root.activateDockEntryNew(3)
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "dockNewWindow5"
        description: qsTr("Open new window of dock entry 5")
        key: "Meta+Ctrl+5"
        onPressed: root.activateDockEntryNew(4)
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "dockNewWindow6"
        description: qsTr("Open new window of dock entry 6")
        key: "Meta+Ctrl+6"
        onPressed: root.activateDockEntryNew(5)
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "dockNewWindow7"
        description: qsTr("Open new window of dock entry 7")
        key: "Meta+Ctrl+7"
        onPressed: root.activateDockEntryNew(6)
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "dockNewWindow8"
        description: qsTr("Open new window of dock entry 8")
        key: "Meta+Ctrl+8"
        onPressed: root.activateDockEntryNew(7)
    }
    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "dockNewWindow9"
        description: qsTr("Open new window of dock entry 9")
        key: "Meta+Ctrl+9"
        onPressed: root.activateDockEntryNew(8)
    }
    }
}
