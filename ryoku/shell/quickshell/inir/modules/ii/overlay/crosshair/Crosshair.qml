import QtQuick
import QtQuick.Layouts
import Quickshell
import inir.modules.common
import inir.modules.ii.overlay

StyledOverlayWidget {
    id: root
    fancyBorders: false // Crosshair should be see-through
    showCenterButton: true
    opacity: 1 // The crosshair itself already has transparency if configured
    showClickabilityButton: false
    resizable: false

    contentItem: CrosshairContent {
        anchors.centerIn: parent
    }
}
