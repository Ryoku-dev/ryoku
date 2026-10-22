import QtQuick
import Quickshell
import Quickshell.Io
import Caelestia
import Caelestia.Config
import caelestia.components
import caelestia.components.images
import caelestia.services
import caelestia.modules.launcher.services

Item {
    id: root

    required property var modelData
    required property var list

    readonly property bool isPinned: root.modelData?.isPinned ?? false

    function clicked() {
        if (!root.modelData)
            return;
        root.list.visibilities.launcher = false;
        const preview = root.modelData.preview.length > 30 ? root.modelData.preview.slice(0, 30) + "..." : root.modelData.preview;

        if (root.isPinned)
            Clipboard.copyPinned(root.modelData.pinId);
        else
            Quickshell.execDetached(["sh", "-c", "cliphist decode " + root.modelData.id + " | wl-copy"]);

        if (GlobalConfig.utilities.toasts.clipboardChanged)
            Toaster.toast(qsTr("Copied to clipboard"), preview, "content_paste");
    }

    function updateImage(): void {
        if (!root.modelData?.isImage) {
            imagePreview.imagePath = "";
            return;
        }

        if (root.isPinned) {
            imagePreview.imagePath = root.modelData.imagePath ?? "";
            return;
        }

        // If already cached on disk, display immediately; otherwise wait for imageReady
        if (Clipboard.isImageCached(root.modelData.id)) {
            imagePreview.imagePath = Clipboard.getImagePath(root.modelData.id);
        } else {
            imagePreview.imagePath = "";
        }
    }
    implicitHeight: (root.modelData?.isImage ?? false) ? Tokens.sizes.launcher.itemHeight * 2 : Tokens.sizes.launcher.itemHeight
    anchors.left: parent?.left
    anchors.right: parent?.right

    onModelDataChanged: updateImage()
    Component.onCompleted: updateImage()

    Connections {
        function onImageReady(id: int, path: string): void {
            if (root.modelData?.isImage && id === root.modelData.id)
                imagePreview.imagePath = path;
        }

        target: Clipboard
    }

    StateLayer {
        radius: Tokens.rounding.large
        onClicked: root.clicked()
    }

    Item {
        anchors.fill: parent
        anchors.leftMargin: Tokens.padding.medium
        anchors.rightMargin: Tokens.padding.medium
        anchors.margins: Tokens.padding.small

        MaterialIcon {
            id: icon

            text: (root.modelData?.isImage ?? false) ? "image" : "content_paste"
            fontStyle: Tokens.font.icon.builders.large.scale(1.3).build()

            anchors.verticalCenter: parent.verticalCenter
            anchors.left: parent.left
        }

        Item {
            id: imagePreview

            property string imagePath: ""

            width: (root.modelData?.isImage ?? false) ? 120 : 0
            height: (root.modelData?.isImage ?? false) ? 80 : 0
            anchors.verticalCenter: parent.verticalCenter
            anchors.left: icon.right
            anchors.leftMargin: (root.modelData?.isImage ?? false) ? Tokens.spacing.medium : 0
            visible: root.modelData?.isImage ?? false

            CachingImage {
                anchors.fill: parent
                path: imagePreview.imagePath
            }
        }

        StyledText {
            anchors.left: icon.right
            anchors.leftMargin: Tokens.spacing.medium
            anchors.right: pinIcon.left
            anchors.rightMargin: Tokens.spacing.small
            anchors.verticalCenter: parent.verticalCenter

            text: root.modelData?.preview ?? ""
            font: Tokens.font.body.medium
            elide: Text.ElideRight
            visible: !(root.modelData?.isImage ?? false)
        }

        StateLayer {
            id: pinIcon

            anchors.fill: undefined
            anchors.verticalCenter: parent.verticalCenter
            anchors.right: parent.right
            width: 32
            height: 32
            radius: Tokens.rounding.full

            onClicked: {
                if (!root.modelData)
                    return;
                if (root.isPinned)
                    Clipboard.unpin(root.modelData.pinId);
                else
                    Clipboard.pin(root.modelData.id);
            }

            MaterialIcon {
                anchors.centerIn: parent
                text: root.isPinned ? "keep" : "keep_off"
                fill: root.isPinned ? 1 : 0
                color: pinIcon.containsMouse ? Colours.palette.m3primary : Colours.palette.m3onSurfaceVariant

                Behavior on color {
                    CAnim {}
                }
            }
        }
    }
}
