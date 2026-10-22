pragma Singleton

import QtQuick
import Quickshell
import Caelestia.Config
import caelestia.components
import caelestia.services

Singleton {
    property var screens: new Map()
    property var bars: new Map()
    property var docks: new Map()
    property string launcherInitialSearch: ""
    property string initialSidebarTab: ""
    property string lastSidebarTab: "notifications"
    property int openDialogs: 0
    readonly property bool sidebarPinned: GlobalConfig.sidebar.pinned
    // Live width while the pinned sidebar's edge is dragged; otherwise follows
    // sidebar.width, where 0 means the default width.
    property int sidebarWidth
    property bool sidebarResizing: false
    property string preOverviewActiveWindowAddress: ""
    property string dragAddress: ""
    property string dragOriginScreen: ""
    property real dragX: 0
    property real dragY: 0
    property real dragWidth: 0
    property real dragHeight: 0
    property string streamClaim: ""

    signal cycleOverview(bool backwards)

    function sidebarWidthFor(defaultWidth: real): real {
        return sidebarPinned && sidebarWidth > 0 ? sidebarWidth : defaultWidth;
    }

    function setSidebarWidth(width: int): void {
        // Also set the live width: the binding is off mid-drag, and a
        // double-click's second press starts one.
        sidebarWidth = Math.max(0, width);
        GlobalConfig.sidebar.width = sidebarWidth;
    }

    function sidebarOpenTab(): string {
        return GlobalConfig.sidebar.defaultTab === "last" ? lastSidebarTab : GlobalConfig.sidebar.defaultTab;
    }

    function load(screen: ShellScreen, visibilities: DrawerVisibilities): void {
        screens.set(Kwin.monitorFor(screen), visibilities);
        screens = new Map(screens);
        visibilities.launcherChanged.connect(() => {
            if (!visibilities.launcher) {
                Kwin.clearHighlight();
                return;
            }
            for (const other of screens.values()) {
                if (other !== visibilities)
                    other.launcher = false;
            }
        });
        visibilities.overviewChanged.connect(() => {
            if (visibilities.overview)
                Kwin.clearHighlight();
        });
        visibilities.sessionChanged.connect(() => {
            if (visibilities.session)
                Kwin.clearHighlight();
        });
    }

    function registerBar(screen: ShellScreen, barWrapper: var): void {
        bars.set(screen.name, barWrapper);
        bars = new Map(bars);
    }
    function registerDock(screen: ShellScreen, dock: var): void {
        docks.set(screen.name, dock);
        docks = new Map(docks);
    }
    function unregisterDock(screen: ShellScreen): void {
        docks.delete(screen.name);
        docks = new Map(docks);
    }
    function getForActive(): DrawerVisibilities {
        const monitor = Kwin.monitors.find(m => m.name === Kwin.cursorOutputName()) || Kwin.focusedMonitor;
        return screens.get(monitor) || screens.values().next().value;
    }

    function setDrag(address: string, x: real, y: real, w: real, h: real, originScreen: string): void {
        dragAddress = address;
        dragX = x;
        dragY = y;
        dragWidth = w;
        dragHeight = h;
        dragOriginScreen = originScreen;
    }

    function clearDrag(): void {
        dragAddress = "";
        dragOriginScreen = "";
    }

    function setOverview(visible: bool): void {
        for (const visibilities of screens.values())
            visibilities.overview = visible;
    }

    Binding on sidebarWidth {
        value: GlobalConfig.sidebar.width
        when: !sidebarResizing
        restoreMode: Binding.RestoreNone
    }

    Timer {
        id: pinRestore

        interval: 1500
        running: GlobalConfig.sidebar.pinned
        onTriggered: {
            const v = getForActive();
            if (v && GlobalConfig.sidebar.pinned)
                v.sidebar = true;
        }
    }
}
