pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Dialogs
import QtQuick.Layouts
import Quickshell
import inir
import inir.modules.common
import inir.modules.common.functions
import inir.modules.common.widgets
import inir.modules.iris.components
import inir.modules.iris.style
import inir.services
import inir.modules.ii.overlay

StyledOverlayWidget {
    id: root

    title: Translation.tr("Floating image")
    showClickabilityButton: false
    minimumWidth: 320
    minimumHeight: 200
    property string imageSource: String(Config.options?.overlay?.floatingImage?.imageSource ?? "")

    contentItem: OverlayBackground {
        id: background
        radius: root.contentRadius

        Image {
            id: image
            anchors.fill: parent
            anchors.margins: Math.round(6 * IrisStyle.density)
            source: root.imageSource.length > 0 && !root.imageSource.startsWith("file:")
                ? "file://" + root.imageSource : root.imageSource
            fillMode: Image.PreserveAspectFit
            asynchronous: true
            cache: false
        }

        ColumnLayout {
            anchors.centerIn: parent
            visible: root.imageSource.length === 0 || image.status === Image.Error
            spacing: 8
            MaterialSymbol {
                Layout.alignment: Qt.AlignHCenter
                text: image.status === Image.Error ? "broken_image" : "add_photo_alternate"
                iconSize: 38
                color: OverlayLook.colSubtext
            }
            IrisText {
                text: image.status === Image.Error ? Translation.tr("Could not open this image") : Translation.tr("Choose an image")
                color: OverlayLook.colSubtext
            }
            IrisButton {
                Layout.alignment: Qt.AlignHCenter
                text: Translation.tr("Choose file")
                emphasized: true
                onClicked: chooser.open()
            }
        }

        IrisButton {
            anchors.top: parent.top
            anchors.right: parent.right
            anchors.margins: 8
            visible: root.imageSource.length > 0 && image.status !== Image.Error && GlobalStates.overlayOpen
            text: Translation.tr("Change")
            onClicked: chooser.open()
        }
    }

    FileDialog {
        id: chooser
        title: Translation.tr("Choose source image")
        fileMode: FileDialog.OpenFile
        currentFolder: Directories.pictures
        nameFilters: [Translation.tr("Images") + " (*.png *.jpg *.jpeg *.webp *.gif *.svg)"]
        onAccepted: Config.setNestedValue("overlay.floatingImage.imageSource", FileUtils.trimFileProtocol(String(selectedFile)))
        onVisibleChanged: OverlayContext.setNativeDialogVisible("floating-image", visible)
    }
    Component.onDestruction: OverlayContext.setNativeDialogVisible("floating-image", false)
}
