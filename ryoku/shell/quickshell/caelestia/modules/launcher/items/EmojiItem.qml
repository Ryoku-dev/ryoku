import QtQuick
import Quickshell
import Caelestia
import Caelestia.Config
import caelestia.components
import caelestia.services
import caelestia.modules.launcher.services

Item {
    id: root

    required property var modelData
    required property var list

    function clicked() {
        if (!root.modelData)
            return;
        root.list.visibilities.launcher = false;
        Quickshell.execDetached(["wl-copy", root.modelData.ch]);
        Emojis.recordUsage(root.modelData.ch);
        if (GlobalConfig.utilities.toasts.clipboardChanged)
            Toaster.toast(qsTr("Copied to clipboard"), root.modelData.ch + " " + root.modelData.name, "emoji_emotions");
    }

    implicitHeight: Tokens.sizes.launcher.itemHeight

    anchors.left: parent?.left
    anchors.right: parent?.right

    StateLayer {
        radius: Tokens.rounding.large
        onClicked: root.clicked()
    }

    Item {
        anchors.fill: parent
        anchors.leftMargin: Tokens.padding.medium
        anchors.rightMargin: Tokens.padding.medium
        anchors.margins: Tokens.padding.small

        StyledText {
            id: emojiChar

            text: root.modelData?.ch ?? ""
            font.pixelSize: Tokens.font.icon.builders.large.scale(1.3).build().pixelSize

            anchors.verticalCenter: parent.verticalCenter
            anchors.left: parent.left
        }

        StyledText {
            id: name

            anchors.left: emojiChar.right
            anchors.leftMargin: Tokens.spacing.medium
            anchors.right: favIcon.left
            anchors.rightMargin: 80
            anchors.verticalCenter: parent.verticalCenter

            text: root.modelData?.name ?? ""
            font: Tokens.font.body.medium
            elide: Text.ElideRight
        }

        StateLayer {
            id: favIcon

            readonly property bool isFav: GlobalConfig.launcher.favouriteEmojis && GlobalConfig.launcher.favouriteEmojis.includes(root.modelData?.ch)

            anchors.fill: undefined
            anchors.verticalCenter: parent.verticalCenter
            anchors.right: parent.right
            width: 32
            height: 32
            radius: Tokens.rounding.full

            onClicked: {
                const emojiChar = root.modelData?.ch;
                if (!emojiChar)
                    return;
                const favEmojis = GlobalConfig.launcher.favouriteEmojis ? [...GlobalConfig.launcher.favouriteEmojis] : [];
                if (favEmojis.includes(emojiChar)) {
                    const idx = favEmojis.indexOf(emojiChar);
                    if (idx !== -1)
                        favEmojis.splice(idx, 1);
                } else {
                    favEmojis.push(emojiChar);
                }
                GlobalConfig.launcher.favouriteEmojis = favEmojis;
            }

            MaterialIcon {
                anchors.centerIn: parent
                text: favIcon.isFav ? "favorite" : "favorite_border"
                fill: favIcon.isFav ? 1 : 0
                color: favIcon.containsMouse ? Colours.palette.m3primary : Colours.palette.m3onSurfaceVariant

                Behavior on color {
                    CAnim {}
                }
            }
        }
    }
}
