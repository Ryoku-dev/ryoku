import QtQuick
import QtQuick.Layouts
import Quickshell
import inir.services
import inir.modules.common
import inir.modules.ii.overlay

StyledOverlayWidget {
    id: root
    title: Translation.tr("Notes")
    showCenterButton: true

    contentItem: NotesContent {
        radius: root.contentRadius
    }
}
