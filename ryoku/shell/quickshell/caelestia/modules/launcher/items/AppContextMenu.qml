pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import Caelestia.Config
import caelestia.components
import caelestia.components.controls as Controls
import caelestia.services
import caelestia.utils

Controls.Menu {
    id: root

    property DesktopEntry app: null
    property DrawerVisibilities visibilities: null

    property real _menuW: root.backgroundItem && root.backgroundItem.implicitWidth > 0 ? root.backgroundItem.implicitWidth : 220
    property real _menuH: root.backgroundItem && root.backgroundItem.implicitHeight > 0 ? root.backgroundItem.implicitHeight : 200
    property bool _flipX: {
        if (!attachTo || !parent)
            return false;
        const mapped = attachTo.mapToItem(parent, 0, 0);
        return mapped.x + attachTo.width / 2 > parent.width / 2;
    }
    property bool _flipY: {
        if (!attachTo || !parent)
            return false;
        const mapped = attachTo.mapToItem(parent, 0, attachTo.height);
        return mapped.y + _menuH > parent.height;
    }

    readonly property bool isPinnedToDock: app ? Strings.testRegexList(GlobalConfig.bar.dock.pinnedApps, app.id) : false
    readonly property bool isHidden: app ? Strings.testRegexList(GlobalConfig.launcher.hiddenApps, app.id) : false
    property bool isPinnedToDesktop: false

    function openFor(targetApp: DesktopEntry, targetItem: Item): void {
        app = targetApp;
        attachTo = targetItem;
        expanded = true;
    }

    function checkDesktopPinned(): void {
        if (!app?.id)
            return;
        desktopCheckProc.running = true;
    }

    attachSideX: _flipX ? Controls.Menu.Right : Controls.Menu.Left
    attachSideY: _flipY ? Controls.Menu.Top : Controls.Menu.Bottom
    thisSideX: _flipX ? Controls.Menu.Right : Controls.Menu.Left
    thisSideY: _flipY ? Controls.Menu.Bottom : Controls.Menu.Top

    items: [
        Controls.MenuItem {
            text: root.isPinnedToDock ? qsTr("Unpin from dock") : qsTr("Pin to dock")
            icon: "push_pin"
            onClicked: {
                if (!root.app?.id)
                    return;
                const current = GlobalConfig.bar.dock.pinnedApps ? [...GlobalConfig.bar.dock.pinnedApps] : [];
                const id = root.app.id;
                if (Strings.testRegexList(current, id)) {
                    const idx = current.indexOf(id);
                    if (idx !== -1)
                        current.splice(idx, 1);
                } else {
                    current.push(id);
                }
                GlobalConfig.bar.dock.pinnedApps = current;
            }
        },
        Controls.MenuItem {
            text: root.isPinnedToDesktop ? qsTr("Remove from desktop") : qsTr("Add to desktop")
            icon: "desktop_windows"
            onClicked: {
                const appId = root.app?.id;
                if (!appId)
                    return;

                if (root.isPinnedToDesktop) {
                    Quickshell.execDetached([
                        "sh", "-c",
                        `rm -f ~/Desktop/"$1" ~/Desktop/"$1.desktop"`,
                        "--", appId
                    ]);
                    root.isPinnedToDesktop = false;
                } else {
                    Quickshell.execDetached([
                        "sh", "-c",
                        `FILE=$(find /usr/share/applications ~/.local/share/applications /var/lib/flatpak/exports/share/applications -name "$1" -o -name "$1.desktop" 2>/dev/null | head -n 1); if [ -n "$FILE" ]; then cp "$FILE" ~/Desktop/; BASENAME=$(basename "$FILE"); chmod +x ~/Desktop/"$BASENAME"; fi`,
                        "--", appId
                    ]);
                    root.isPinnedToDesktop = true;
                }
            }
        },
        Controls.MenuItem {
            text: root.isHidden ? qsTr("Show in launcher") : qsTr("Hide app")
            icon: root.isHidden ? "visibility" : "visibility_off"
            onClicked: {
                const appId = root.app?.id;
                if (!appId)
                    return;
                const hiddenApps = GlobalConfig.launcher.hiddenApps ? [...GlobalConfig.launcher.hiddenApps] : [];
                if (Strings.testRegexList(hiddenApps, appId)) {
                    const idx = hiddenApps.indexOf(appId);
                    if (idx !== -1)
                        hiddenApps.splice(idx, 1);
                } else {
                    hiddenApps.push(appId);
                }
                GlobalConfig.launcher.hiddenApps = hiddenApps;
            }
        },
        Controls.MenuItem {
            text: qsTr("App info")
            icon: "info"
            onClicked: {
                if (!root.app?.id)
                    return;
                const desktopFile = root.app.id.endsWith(".desktop") ? root.app.id : (root.app.id + ".desktop");
                Launch.exec(["kmenuedit", desktopFile]);
                if (root.visibilities)
                    root.visibilities.launcher = false;
            }
        }
    ]

    onExpandedChanged: {
        if (expanded)
            checkDesktopPinned();
    }

    onAppChanged: {
        if (expanded)
            checkDesktopPinned();
    }

    Process {
        id: desktopCheckProc

        command: ["sh", "-c", "test -f ~/Desktop/\"$1\" || test -f ~/Desktop/\"$1.desktop\"", "--", root.app?.id ?? ""]
        onExited: code => {
            root.isPinnedToDesktop = (code === 0);
        }
    }
}
