pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import shell.services

Scope {
    id: popupHost

    required property var modelData

    readonly property string notificationUrl: Designs.notificationUrl
    property string failedNotificationUrl: ""
    onNotificationUrlChanged: popupHost.failedNotificationUrl = ""
    // mirrored here so the loader never reads its own item
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
