pragma ComponentBehavior: Bound

import QtQuick
import inir.services
import inir.modules.iris.stage
import inir.modules.ii.overlay

StyledOverlayWidget {
    id: root
    title: Translation.tr("Notifications")
    minimumWidth: 420
    minimumHeight: 500
    contentItem: OverlayBackground {
        radius: root.contentRadius
        IrisCardContent {
            anchors.fill: parent
            kind: "notifications"
            contentActive: root.visible
        }
    }
}
