pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import shell.services

// The per-monitor popup column, built on the first popup and released once its
// surface has unmapped after the last card left (NotificationColumn).
Scope {
    id: popupHost

    required property var modelData

    readonly property string notificationUrl: Designs.notificationUrl
    property string failedNotificationUrl: ""
    onNotificationUrlChanged: popupHost.failedNotificationUrl = ""
    // Mirrors the column's mapped state, so the loader never reads its own item.
    property bool mapped: false

    LazyLoader {
        id: column
        active: Notifs.popups.length > 0 || popupHost.mapped

        NotificationColumn {
            modelData: popupHost.modelData
            host: popupHost
        }
    }
}
