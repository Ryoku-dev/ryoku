pragma Singleton

import QtQuick
import Quickshell
import Caelestia.Config
import caelestia.services

Singleton {
    id: root

    property var items: []
    property int selectedIndex: 0
    property bool isSwitching: false

    function triggerCycleNext(): void {
        if (items.length === 0) return;
        selectedIndex = (selectedIndex + 1) % items.length;
    }

    function triggerCyclePrev(): void {
        if (items.length === 0) return;
        selectedIndex = (selectedIndex - 1 + items.length) % items.length;
    }

    function focusSelectedWindow(): void {
        if (selectedIndex >= 0 && selectedIndex < items.length) {
            focusWindow(items[selectedIndex].address);
        }
    }

    function reload(): void {
        updateItems();
    }

    function refreshHighlight(): void {
        if (!root.isSwitching || !GlobalConfig.tabSwitch?.previewOnDesktop) {
            Kwin.clearHighlight();
            return;
        }
        if (selectedIndex >= 0 && selectedIndex < items.length) {
            Kwin.highlightWindow(items[selectedIndex].address);
        } else {
            Kwin.clearHighlight();
        }
    }

    function getDesktopName(client: var): string {
        if (!client || !client.workspace) return "";
        const wsId = client.workspace.id;
        const wsUuid = client.workspace.uuid;
        if (Kwin.workspaces) {
            for (let i = 0; i < Kwin.workspaces.length; ++i) {
                const ws = Kwin.workspaces[i];
                if ((wsUuid && ws.id === wsUuid) || (wsId !== undefined && wsId !== -1 && ws.index === wsId)) {
                    return ws.name || ("Desktop " + ws.index);
                }
            }
        }
        if (typeof wsId === "number" && wsId > 0) return "Desktop " + wsId;
        return wsUuid ? String(wsUuid) : "";
    }

    function updateItems(): void {
        const activeAddress = Kwin.activeWindow ? Kwin.activeWindow.address : "";
        const winList = (Kwin.windowList || []).filter(w => !(w.class && w.class.toLowerCase().includes("xwaylandvideobridge")));
        
        let currentItems = root.items.slice();
        
        currentItems = currentItems.filter(item => {
            for (let i = 0; i < winList.length; i++) {
                if (winList[i].address === item.address) return true;
            }
            return false;
        });
        
        const formatClient = (client) => {
            return {
                address: client.address,
                title: client.title || "",
                class: client.class || "",
                iconName: client.iconName || client.class || "",
                workspace: client.workspace?.id ?? "",
                workspaceUuid: client.workspace?.uuid ?? "",
                desktopName: getDesktopName(client),
                minimized: !!client.minimized,
                closeable: true,
                pid: client.pid || 0,
                monitor: client.output || "",
                wayland: true,
                size: [client.width || 0, client.height || 0],
                at: [client.x || 0, client.y || 0]
            };
        };

        for (let i = 0; i < winList.length; ++i) {
            const client = winList[i];
            let found = false;
            for (let j = 0; j < currentItems.length; ++j) {
                if (currentItems[j].address === client.address) {
                    currentItems[j] = formatClient(client);
                    found = true;
                    break;
                }
            }
            if (!found) {
                currentItems.push(formatClient(client));
            }
        }
        
        if (activeAddress) {
            for (let i = 0; i < currentItems.length; i++) {
                if (currentItems[i].address === activeAddress) {
                    const activeWin = currentItems.splice(i, 1)[0];
                    currentItems.unshift(activeWin);
                    break;
                }
            }
        }

        if (GlobalConfig.tabSwitch?.currentDesktopOnly) {
            currentItems = Kwin.filterWindows(currentItems, Kwin.activeWsId, "", true);
        }

        if (GlobalConfig.tabSwitch && !GlobalConfig.tabSwitch.allScreens) {
            const activeOut = Kwin.activeOutputName || Kwin.cursorOutputName();
            if (activeOut) {
                currentItems = Kwin.filterWindows(currentItems, null, activeOut, true);
            }
        }

        if (GlobalConfig.tabSwitch && !GlobalConfig.tabSwitch.showMinimized) {
            currentItems = currentItems.filter(item => !item.minimized);
        }
        
        items = currentItems;

        if (root.selectedIndex >= currentItems.length)
            root.selectedIndex = Math.max(0, currentItems.length - 1);
    }

    function query(search: string): var {
        if (!search)
            return items;
        const lower = search.toLowerCase();
        return items.filter(w => (w.title && w.title.toLowerCase().includes(lower)) || (w.class && w.class.toLowerCase().includes(lower)) || (w.desktopName && w.desktopName.toLowerCase().includes(lower)));
    }

    function focusWindow(address: string): void {
        root.isSwitching = false;
        Kwin.clearHighlight();
        Kwin.focusWindow(address);
    }

    function closeWindow(address: string): void {
        Kwin.closeWindow(address);
    }

    onSelectedIndexChanged: {
        if (root.isSwitching)
            refreshHighlight();
    }

    onIsSwitchingChanged: {
        if (root.isSwitching)
            refreshHighlight();
        else
            Kwin.clearHighlight();
    }

    Component.onCompleted: {
        updateItems();
    }

    Connections {
        function onWindowListChanged(): void {
            root.updateItems();
        }

        function onActiveWindowChanged(): void {
            root.updateItems();
        }

        target: Kwin
    }

    Connections {
        function onCurrentDesktopOnlyChanged(): void {
            root.updateItems();
        }

        function onAllScreensChanged(): void {
            root.updateItems();
        }

        function onShowMinimizedChanged(): void {
            root.updateItems();
        }

        function onPreviewOnDesktopChanged(): void {
            if (!GlobalConfig.tabSwitch.previewOnDesktop || !root.isSwitching) {
                Kwin.clearHighlight();
            } else {
                root.refreshHighlight();
            }
        }

        target: GlobalConfig.tabSwitch
    }
}

