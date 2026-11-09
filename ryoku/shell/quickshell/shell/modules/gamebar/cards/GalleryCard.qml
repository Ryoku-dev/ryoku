pragma ComponentBehavior: Bound

import QtQuick
import Qt.labs.folderlistmodel
import shell.services
import ".."
import Ryoku.Ui.Singletons
import "../look"

// Gallery: the newest screenshots, live from the folder the capture path
// writes, plus one press to open either capture folder. Thumbnails are the
// files themselves, so there is no separate index to fall out of date.
GameBarCard {
    id: card

    title: I18n.tr("Gallery")

    readonly property string shotsDir: Capture.shotsDir
    readonly property string clipsDir: Paths.recordingsDir

    function newest(model, count) {
        const all = [];
        for (let i = 0; i < model.count; i++) {
            const url = model.get(i, "fileUrl");
            if (!url)
                continue;
            const stamp = model.get(i, "fileModified");
            const when = stamp ? (typeof stamp.getTime === "function"
                ? stamp.getTime() : Date.parse(stamp) || 0) : 0;
            all.push({
                url: "" + url,
                path: "" + model.get(i, "filePath"),
                t: when
            });
        }
        all.sort((a, b) => b.t - a.t);
        return all.slice(0, count);
    }

    readonly property var shots: card.newest(shotFiles, 6)

    FolderListModel {
        id: shotFiles
        folder: "file://" + card.shotsDir
        showDirs: false
        nameFilters: ["*.png", "*.jpg", "*.jpeg", "*.webp"]
    }

    Column {
        width: parent.width
        spacing: 8 * card.s

        Item {
            width: parent.width
            height: 24 * card.s

            Text {
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                text: card.shots.length === 1
                    ? I18n.tr("1 recent shot") : card.shots.length + " " + I18n.tr("recent shots")
                color: GameBarLook.subtext
                font.family: GameBarLook.face
                font.pixelSize: GameBarLook.smallie * card.s
            }

            GameBarTextButton {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                s: card.s
                compact: true
                label: I18n.tr("Folder")
                onClicked: Spawn.run(["xdg-open", card.shotsDir])
            }
        }

        Grid {
            id: grid
            width: parent.width
            columns: 3
            spacing: 6 * card.s

            Repeater {
                model: card.shots

                delegate: Item {
                    id: thumb
                    required property var modelData

                    width: (grid.width - (grid.columns - 1) * grid.spacing) / grid.columns
                    height: 62 * card.s

                    Rectangle {
                        anchors.fill: parent
                        radius: GameBarLook.radiusVerySmall * card.s
                        color: GameBarLook.raised
                        clip: true

                        Image {
                            anchors.fill: parent
                            source: thumb.modelData.url
                            asynchronous: true
                            fillMode: Image.PreserveAspectCrop
                        }
                        MouseArea {
                            anchors.fill: parent
                            hoverEnabled: true
                            cursorShape: Qt.PointingHandCursor
                            onClicked: Spawn.run(["xdg-open", thumb.modelData.path])
                        }
                    }
                }
            }
        }

        Text {
            visible: card.shots.length === 0
            width: parent.width
            text: I18n.tr("No screenshots yet")
            color: GameBarLook.subtext
            horizontalAlignment: Text.AlignHCenter
            font.family: GameBarLook.face
            font.pixelSize: GameBarLook.small * card.s
        }

        Row {
            width: parent.width
            spacing: 8 * card.s

            GameBarTextButton {
                s: card.s
                label: I18n.tr("Screenshots")
                onClicked: Spawn.run(["xdg-open", card.shotsDir])
            }
            GameBarTextButton {
                s: card.s
                label: I18n.tr("Recordings")
                onClicked: Spawn.run(["xdg-open", card.clipsDir])
            }
        }
    }
}
