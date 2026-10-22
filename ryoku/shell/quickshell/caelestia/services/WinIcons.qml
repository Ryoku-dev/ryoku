pragma Singleton

import QtQuick
import Quickshell
import caelestia.utils

Singleton {
    id: root

    // shortcut: the KDE per-window icon extraction is gone, so a window shows the
    // theme icon for its app id, add an extractor here if apps without one matter.
    function request(appClass: string, title: string, pid: int, address: string): void {}

    function sourceFor(entry: var, appClass: string, iconName: string, pid: int): string {
        if (entry && entry.icon)
            return Quickshell.iconPath(entry.icon, "application-x-executable");
        return Quickshell.iconPath(iconName || "application-x-executable", "application-x-executable");
    }

    function sourceForClient(client: var): string {
        if (!client)
            return "";
        return client.iconName ? Icons.getAppIcon(client.iconName, "application-x-executable")
                               : (client.class ? Icons.getAppIcon(client.class, "application-x-executable") : "");
    }
}
